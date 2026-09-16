package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const anthropicStreamRetryContextKey = "anthropic_stream_safe_retry"
const maxAnthropicStreamPreludeBytes = 256 << 10

// AnthropicStreamFailure 保留真实流故障分类，不将 HTTP 200 包装成账号的 HTTP 502 故障。
type AnthropicStreamFailure struct {
	Kind       string
	Cause      error
	Replayable bool
}

func (e *AnthropicStreamFailure) Error() string {
	switch e.Kind {
	case "empty_stream", "missing_terminal":
		return "stream usage incomplete: missing terminal event"
	case "idle_timeout":
		return "stream data interval timeout"
	case "first_content_timeout":
		return "等待上游有效内容超时"
	case "budget_exhausted":
		return "流安全重试总等待预算已耗尽"
	case "retries_exhausted":
		return "流安全重试次数已耗尽"
	case "client_canceled":
		return "客户端已取消请求"
	case "prelude_overflow":
		return "流前导帧超过256 KiB缓存上限"
	case "no_available_account":
		return "没有可用于安全重试的账号"
	case "replay_forbidden":
		return "当前请求禁止重放"
	default:
		return "上游流异常：" + e.Kind
	}
}
func (e *AnthropicStreamFailure) Unwrap() error { return e.Cause }
func (e *AnthropicStreamFailure) ClientStatus() int {
	if e.Kind == "idle_timeout" || e.Kind == "budget_exhausted" || e.Kind == "first_content_timeout" {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

// AnthropicStreamDiagnostic 只含结构与计量摘要，禁止加入生成正文、工具参数或凭据。
type AnthropicStreamDiagnostic struct {
	PendingFrameBytes     int         `json:"pending_frame_bytes"`
	TerminalCandidateSeen bool        `json:"terminal_candidate_seen"`
	RequestID             string      `json:"request_id,omitempty"`
	ClientRequestID       string      `json:"client_request_id,omitempty"`
	GroupID               int64       `json:"group_id,omitempty"`
	Attempt               int         `json:"attempt"`
	FailureKind           string      `json:"failure_kind,omitempty"`
	LastEventType         string      `json:"last_event_type,omitempty"`
	TerminalComplete      bool        `json:"terminal_complete"`
	OutputCommitted       bool        `json:"output_committed"`
	PreludeBytes          int         `json:"prelude_bytes"`
	UpstreamBytes         int64       `json:"upstream_bytes"`
	Heartbeats            int         `json:"heartbeats"`
	LastReadAgeMs         int64       `json:"last_read_age_ms"`
	FirstSemanticMs       *int64      `json:"first_semantic_ms,omitempty"`
	ElapsedMs             int64       `json:"elapsed_ms"`
	BudgetRemainingMs     int64       `json:"budget_remaining_ms"`
	RetriesRemaining      int         `json:"retries_remaining"`
	Decision              string      `json:"decision,omitempty"`
	StopReason            string      `json:"stop_reason,omitempty"`
	WireStatus            int         `json:"wire_status"`
	LogicalStatus         int         `json:"logical_status,omitempty"`
	Recovered             bool        `json:"recovered"`
	Usage                 ClaudeUsage `json:"usage"`
}

// AnthropicStreamRetryState 跨账号持有同一份预算；所有计时与取消回调只访问受锁状态。
// waitCtx 没有固定 deadline，首个内容提交后停止计时，不会误杀后续长流。
type AnthropicStreamRetryState struct {
	mu            sync.Mutex
	settings      GatewaySettings
	clientCtx     context.Context
	waitCtx       context.Context
	cancel        context.CancelCauseFunc
	timer         *time.Timer
	clientStop    func() bool
	started       time.Time
	committed     bool
	closed        bool
	replays       int
	dispatches    int
	lastAccountID int64
	pending       bool
	preferSame    bool
	allowSame     bool
	sameTried     map[int64]bool
	failed        map[int64]struct{}
	diagnostics   []*AnthropicStreamDiagnostic
	attemptUsage  *ForwardResult
}

func newAnthropicStreamRetryState(ctx context.Context, settings GatewaySettings) *AnthropicStreamRetryState {
	waitCtx, cancel := context.WithCancelCause(ctx)
	r := &AnthropicStreamRetryState{settings: settings, clientCtx: ctx, waitCtx: waitCtx, cancel: cancel, sameTried: make(map[int64]bool), failed: make(map[int64]struct{})}
	r.clientStop = context.AfterFunc(ctx, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.timer != nil {
			r.timer.Stop()
		}
	})
	return r
}
func AnthropicStreamRetryFromGin(c *gin.Context) *AnthropicStreamRetryState {
	if c == nil {
		return nil
	}
	value, _ := c.Get(anthropicStreamRetryContextKey)
	r, _ := value.(*AnthropicStreamRetryState)
	return r
}
func (s *GatewayService) anthropicStreamRetry(c *gin.Context, ctx context.Context) *AnthropicStreamRetryState {
	if r := AnthropicStreamRetryFromGin(c); r != nil {
		return r
	}
	if c == nil {
		return nil
	}
	// 请求首次进入该路径时冻结配置，后续尝试不重新读取开关或预算。
	const snapshotKey = "anthropic_stream_safe_retry_snapshot"
	value, exists := c.Get(snapshotKey)
	settings, _ := value.(GatewaySettings)
	if !exists {
		settings = resolveFirstTokenTimeoutSettings(ctx, s.rateLimitService)
		c.Set(snapshotKey, settings)
	}
	if !settings.AnthropicStreamSafeRetryEnabled {
		return nil
	}
	r := newAnthropicStreamRetryState(ctx, settings)
	c.Set(anthropicStreamRetryContextKey, r)
	return r
}
func (r *AnthropicStreamRetryState) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	if r.timer != nil {
		r.timer.Stop()
	}
	r.mu.Unlock()
	if r.clientStop != nil {
		r.clientStop()
	}
	r.cancel(context.Canceled)
}
func (r *AnthropicStreamRetryState) WaitContext() context.Context   { return r.waitCtx }
func (r *AnthropicStreamRetryState) ClientContext() context.Context { return r.clientCtx }
func (r *AnthropicStreamRetryState) Started() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.started.IsZero()
}
func (r *AnthropicStreamRetryState) Pending() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending
}
func (r *AnthropicStreamRetryState) Committed() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.committed
}
func (r *AnthropicStreamRetryState) LastAccountID() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastAccountID
}
func (r *AnthropicStreamRetryState) remainingLocked() time.Duration {
	if r.started.IsZero() {
		return time.Duration(r.settings.AnthropicStreamSafeRetryTotalWaitSeconds) * time.Second
	}
	return time.Duration(r.settings.AnthropicStreamSafeRetryTotalWaitSeconds)*time.Second - time.Since(r.started)
}
func (r *AnthropicStreamRetryState) checkLocked() error {
	if r.clientCtx.Err() != nil {
		return &AnthropicStreamFailure{Kind: "client_canceled", Cause: r.clientCtx.Err()}
	}
	if r.closed {
		return &AnthropicStreamFailure{Kind: "replay_forbidden"}
	}
	if !r.committed && (!r.started.IsZero() && r.remainingLocked() <= 0 || errors.As(context.Cause(r.waitCtx), new(*AnthropicStreamFailure))) {
		return &AnthropicStreamFailure{Kind: "budget_exhausted"}
	}
	return nil
}
func (r *AnthropicStreamRetryState) Check() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.checkLocked()
}
func (r *AnthropicStreamRetryState) startResponse() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.started.IsZero() || r.closed {
		return
	}
	r.started = time.Now()
	r.timer = time.AfterFunc(r.remainingLocked(), func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.committed && !r.closed {
			r.cancel(&AnthropicStreamFailure{Kind: "budget_exhausted"})
		}
	})
}

// beforeDispatch 是独立次数的唯一扣减点，内层 HTTP 重试也必须经过这里。
func (r *AnthropicStreamRetryState) beforeDispatch(account *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkLocked(); err != nil {
		return err
	}
	if r.committed {
		return &AnthropicStreamFailure{Kind: "replay_forbidden"}
	}
	if !r.started.IsZero() {
		if r.replays >= r.settings.AnthropicStreamSafeRetryMaxRetries {
			return &AnthropicStreamFailure{Kind: "retries_exhausted"}
		}
		if account.ID == r.lastAccountID && account.GetPoolModeRetryCount() == 0 {
			return &AnthropicStreamFailure{Kind: "replay_forbidden"}
		}
		if account.ID == r.lastAccountID {
			r.sameTried[account.ID] = true
		}
		r.replays++
	}
	r.attemptUsage = nil
	r.dispatches++
	r.lastAccountID = account.ID
	r.pending = false
	return nil
}

// bindRequest 显式接回前导阶段的取消，避免 WithoutCancel 吞掉预算；内容提交后保留既有排空语义。
func (r *AnthropicStreamRetryState) bindRequest(req *http.Request) (*http.Request, func()) {
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(r.waitCtx, func() {
		if !r.Committed() {
			cancel()
		}
	})
	return req.WithContext(ctx), func() { stop(); cancel() }
}
func (r *AnthropicStreamRetryState) commit() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkLocked(); err != nil {
		return err
	}
	r.committed = true
	r.pending = false
	if r.timer != nil {
		r.timer.Stop()
	}
	return nil
}

// PrepareRetry 不借用账号重试次数，也不触发账号 HTTP 状态码封禁。
func (r *AnthropicStreamRetryState) PrepareRetry(account *Account, err error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stop := r.checkLocked(); stop != nil {
		return stop
	}
	if r.committed {
		return &AnthropicStreamFailure{Kind: "output_committed"}
	}
	var streamErr *AnthropicStreamFailure
	var failover *UpstreamFailoverError
	replayable := errors.As(err, &streamErr) && streamErr.Replayable
	if errors.As(err, &failover) {
		replayable = failover.ShouldRetryNextAccount()
	}
	if isOpenAIRequestSentPluginError(err) {
		replayable = false
	}
	if !replayable {
		return err
	}
	if r.replays >= r.settings.AnthropicStreamSafeRetryMaxRetries {
		return &AnthropicStreamFailure{Kind: "retries_exhausted", Cause: err}
	}
	r.pending = true
	r.allowSame = account.GetPoolModeRetryCount() > 0 && (failover == nil || (failover.RetryableOnSameAccount && !failover.FirstTokenTimeout))
	r.preferSame = !r.sameTried[account.ID] && r.allowSame
	r.failed[account.ID] = struct{}{}
	return nil
}
func (r *AnthropicStreamRetryState) Diagnostic(d *AnthropicStreamDiagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d.RequestID, _ = r.clientCtx.Value(ctxkey.RequestID).(string)
	d.ClientRequestID, _ = r.clientCtx.Value(ctxkey.ClientRequestID).(string)
	d.RequestID = truncateString(d.RequestID, 128)
	d.ClientRequestID = truncateString(d.ClientRequestID, 128)
	if group, _ := r.clientCtx.Value(ctxkey.Group).(*Group); group != nil {
		d.GroupID = group.ID
	}
	d.Attempt = r.dispatches
	d.OutputCommitted = r.committed
	if !r.started.IsZero() {
		d.ElapsedMs = time.Since(r.started).Milliseconds()
	}
	d.BudgetRemainingMs = nonnegativeStreamMillis(r.remainingLocked().Milliseconds())
	d.RetriesRemaining = max(0, r.settings.AnthropicStreamSafeRetryMaxRetries-r.replays)
	r.diagnostics = append(r.diagnostics, d)
}
func (r *AnthropicStreamRetryState) RecordDecision(ctx context.Context, decision, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.diagnostics) == 0 {
		return
	}
	d := r.diagnostics[len(r.diagnostics)-1]
	d.Decision = decision
	d.StopReason = reason
	d.BudgetRemainingMs = nonnegativeStreamMillis(r.remainingLocked().Milliseconds())
	logger.FromContext(ctx).Info("gateway.anthropic_stream_retry_decision", zap.Any("stream_diagnostic", *d), zap.Int64("account_id", r.lastAccountID))
}
func (r *AnthropicStreamRetryState) MarkRecovered(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.replays == 0 || r.clientCtx.Err() != nil {
		return
	}
	for _, d := range r.diagnostics {
		d.Recovered = true
	}
	logger.FromContext(ctx).Info("gateway.anthropic_stream_recovered", zap.Int("retries", r.replays), zap.Int64("account_id", r.lastAccountID))
}

// SelectAnthropicStreamRetryAccount 通过既有完整调度链重选，过滤并不绕过分组、利润、限流、会话和并发资格。
func (s *GatewayService) SelectAnthropicStreamRetryAccount(ctx context.Context, r *AnthropicStreamRetryState, groupID *int64, session, model string, excluded map[int64]struct{}, metadata string, userID int64, allowSwitch bool) (*AccountSelectionResult, error) {
	if err := r.Check(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	lastID, preferSame, allowSame := r.lastAccountID, r.preferSame, r.allowSame
	failed := make(map[int64]struct{}, len(r.failed))
	for id := range r.failed {
		failed[id] = struct{}{}
	}
	r.mu.Unlock()
	accounts, _, err := s.listSchedulableAccounts(ctx, groupID, PlatformAnthropic, false)
	if err != nil {
		return nil, err
	}
	ineligible := make(map[int64]struct{})
	for index := range accounts {
		candidate := &accounts[index]
		if candidate.Platform != PlatformAnthropic || candidate.Type != AccountTypeAPIKey {
			continue
		}
		_, hasPassthrough := candidate.Extra["anthropic_passthrough"]
		_, hasPoolMode := candidate.Credentials["pool_mode"]
		_, hasRetryCount := candidate.Credentials["pool_mode_retry_count"]
		if hasPassthrough && hasPoolMode && hasRetryCount {
			continue
		}
		hydrated, hydrateErr := s.hydrateSelectedAccount(ctx, candidate)
		if hydrateErr != nil || hydrated == nil {
			ineligible[candidate.ID] = struct{}{}
			continue
		}
		accounts[index] = *hydrated
	}
	selectCandidate := func(same bool) (*AccountSelectionResult, error) {
		blocked := make(map[int64]struct{}, len(excluded)+len(accounts))
		for id := range excluded {
			blocked[id] = struct{}{}
		}
		for id := range ineligible {
			blocked[id] = struct{}{}
		}
		for _, a := range accounts {
			_, failedBefore := failed[a.ID]
			if !a.IsAnthropicAPIKeyPassthroughEnabled() || (same && (a.ID != lastID || a.GetPoolModeRetryCount() == 0)) || (!same && (a.ID == lastID || failedBefore)) {
				blocked[a.ID] = struct{}{}
			}
		}
		if _, found := blocked[lastID]; !same && !found {
			blocked[lastID] = struct{}{}
		}
		result, selectErr := s.SelectAccountWithLoadAwareness(ctx, groupID, session, model, blocked, metadata, userID)
		if selectErr != nil {
			return nil, selectErr
		}
		a := result.Account
		if !a.IsAnthropicAPIKeyPassthroughEnabled() || (same && (a.ID != lastID || a.GetPoolModeRetryCount() == 0)) || (!same && a.ID == lastID) {
			if result.ReleaseFunc != nil {
				result.ReleaseFunc()
			}
			s.ReleaseAccountSession(context.WithoutCancel(ctx), a, session)
			return nil, fmt.Errorf("%w: 重试账号资格已变化", ErrNoAvailableAccounts)
		}
		return result, nil
	}
	if preferSame {
		if result, err := selectCandidate(true); err == nil {
			return result, nil
		} else if ctx.Err() != nil {
			return nil, r.Check()
		}
	}
	if allowSwitch {
		if result, err := selectCandidate(false); err == nil {
			return result, nil
		} else if ctx.Err() != nil {
			return nil, r.Check()
		}
	}
	if !preferSame && allowSame {
		if result, err := selectCandidate(true); err == nil {
			return result, nil
		}
	}
	if err := r.Check(); err != nil {
		return nil, err
	}
	return nil, &AnthropicStreamFailure{Kind: "no_available_account"}
}

func nonnegativeStreamMillis(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}

// AttemptUsageSnapshot 独立保存当前尝试的计量，不改变 UpstreamFailoverError 的 result=nil 约定。
func (r *AnthropicStreamRetryState) AttemptUsageSnapshot(accountID int64) *ForwardResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID != r.lastAccountID {
		return nil
	}
	return r.attemptUsage
}
func (r *AnthropicStreamRetryState) observeAttemptUsage(result *ForwardResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attemptUsage = result
}

// RecordAttemptFailure 补齐重试响应头等待失败等尚未进入 SSE 解析器的诊断。
func (r *AnthropicStreamRetryState) RecordAttemptFailure(c *gin.Context, account *Account, err error) {
	if err == nil || !r.Started() {
		return
	}
	r.mu.Lock()
	exists := len(r.diagnostics) > 0 && r.diagnostics[len(r.diagnostics)-1].Attempt == r.dispatches
	r.mu.Unlock()
	if exists {
		return
	}
	d := &AnthropicStreamDiagnostic{FailureKind: "stream_read_error"}
	var failure *AnthropicStreamFailure
	if errors.As(err, &failure) {
		d.FailureKind = failure.Kind
		d.LogicalStatus = failure.ClientStatus()
	}
	if c.Writer.Written() {
		d.WireStatus = c.Writer.Status()
	}
	r.Diagnostic(d)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{Platform: account.Platform, AccountID: account.ID, AccountName: account.Name, ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account), Passthrough: true, Kind: "stream_failure", Stage: "stream", Reason: d.FailureKind, Message: "Claude安全重试等待未正常完成", StreamDiagnostic: d})
}
func (r *AnthropicStreamRetryState) RecordFinalWireStatus(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.diagnostics) > 0 {
		r.diagnostics[len(r.diagnostics)-1].WireStatus = status
	}
}
