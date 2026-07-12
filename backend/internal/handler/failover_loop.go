package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// TempUnscheduler 用于 HandleFailoverError 中同账号重试耗尽后的临时封禁。
// GatewayService 隐式实现此接口。
type TempUnscheduler interface {
	TempUnscheduleRetryableError(ctx context.Context, accountID int64, failoverErr *service.UpstreamFailoverError)
}

type FailoverStrictScheduler interface {
	HandleUpstreamFailoverError(ctx context.Context, accountID int64, failoverErr *service.UpstreamFailoverError) bool
}

// FailoverOutcomeRecorder 在同账号重试耗尽后记录一次账号最终结果。
// managed 表示该状态码由全局连续错误策略接管；blocked 表示已经达到阈值并持久阻断账号。
type FailoverOutcomeRecorder interface {
	RecordUpstreamFailureOutcome(ctx context.Context, accountID int64, failoverErr *service.UpstreamFailoverError) (managed bool, blocked bool)
}

type FailoverOutcomeEventRecorder interface {
	RecordUpstreamFailureOutcomeAt(
		ctx context.Context,
		accountID int64,
		failoverErr *service.UpstreamFailoverError,
		event service.AccountFailureStreakEvent,
	) (managed bool, blocked bool)
}

type FailoverOutcomeSnapshotCapturer interface {
	CaptureUpstreamFailureOutcome(
		ctx context.Context,
		event service.AccountFailureStreakEvent,
	) service.AccountFailureOutcomeSnapshot
}

type FailoverOutcomeSnapshotRecorder interface {
	RecordUpstreamFailureOutcomeSnapshot(
		ctx context.Context,
		accountID int64,
		failoverErr *service.UpstreamFailoverError,
		snapshot service.AccountFailureOutcomeSnapshot,
	) (managed bool, blocked bool)
}

type FailoverOutcomePolicy interface {
	IsUpstreamFailureOutcomeManaged(ctx context.Context, accountID int64, failoverErr *service.UpstreamFailoverError) bool
}

type FailoverSuccessOutcomeRecorder interface {
	RecordUpstreamSuccessOutcome(ctx context.Context, accountID int64)
}

type FailoverSuccessOutcomeEventRecorder interface {
	RecordUpstreamSuccessOutcomeAt(ctx context.Context, accountID int64, event service.AccountFailureStreakEvent)
}

// FailoverAction 表示 failover 错误处理后的下一步动作
type FailoverAction int

const (
	// FailoverContinue 继续循环（同账号重试或切换账号，调用方统一 continue）
	FailoverContinue FailoverAction = iota
	// FailoverExhausted 切换次数耗尽（调用方应返回错误响应）
	FailoverExhausted
	// FailoverCanceled context 已取消（调用方应直接 return）
	FailoverCanceled
)

const (
	// sameAccountRetryDelay 同账号重试间隔
	sameAccountRetryDelay = 500 * time.Millisecond
	// singleAccountBackoffDelay 单账号分组 503 退避重试固定延时。
	// Service 层在 SingleAccountRetry 模式下已做充分原地重试（最多 3 次、总等待 30s），
	// Handler 层只需短暂间隔后重新进入 Service 层即可。
	singleAccountBackoffDelay = 2 * time.Second
)

// FailoverState 跨循环迭代共享的 failover 状态
type FailoverState struct {
	SwitchCount           int
	MaxSwitches           int
	FailedAccountIDs      map[int64]struct{}
	SameAccountRetryCount map[int64]int
	RecordedOutcomes      map[int64]failoverRecordedOutcome
	PendingOutcomes       map[int64]failoverPendingOutcome
	LastFailoverErr       *service.UpstreamFailoverError
	ForceCacheBilling     bool
	hasBoundSession       bool
}

type failoverRecordedOutcome struct {
	managed bool
	blocked bool
}

type failoverPendingOutcome struct {
	failoverErr *service.UpstreamFailoverError
	snapshot    service.AccountFailureOutcomeSnapshot
}

// NewFailoverState 创建 failover 状态
func NewFailoverState(maxSwitches int, hasBoundSession bool) *FailoverState {
	return &FailoverState{
		MaxSwitches:           maxSwitches,
		FailedAccountIDs:      make(map[int64]struct{}),
		SameAccountRetryCount: make(map[int64]int),
		RecordedOutcomes:      make(map[int64]failoverRecordedOutcome),
		PendingOutcomes:       make(map[int64]failoverPendingOutcome),
		hasBoundSession:       hasBoundSession,
	}
}

// HandleFailoverError 处理 UpstreamFailoverError，返回下一步动作。
// 包含：缓存计费判断、同账号重试、临时封禁、切换计数、Antigravity 延时。
func (s *FailoverState) HandleFailoverError(
	ctx context.Context,
	gatewayService TempUnscheduler,
	accountID int64,
	platform string,
	failoverErr *service.UpstreamFailoverError,
	retryLimit int,
) FailoverAction {
	if ctx != nil && ctx.Err() != nil {
		return FailoverCanceled
	}
	s.LastFailoverErr = failoverErr
	if retryLimit < 0 {
		retryLimit = 0
	}

	// 缓存计费判断
	if needForceCacheBilling(s.hasBoundSession, failoverErr) {
		s.ForceCacheBilling = true
	}

	strictFailureUnscheduled := false
	if strictScheduler, ok := gatewayService.(FailoverStrictScheduler); ok {
		strictFailureUnscheduled = strictScheduler.HandleUpstreamFailoverError(ctx, accountID, failoverErr)
	}
	if strictFailureUnscheduled {
		delete(s.PendingOutcomes, accountID)
	}

	// 同账号重试：对 RetryableOnSameAccount 的临时性错误，先在同一账号上重试
	if failoverErr.RetryableOnSameAccount && !strictFailureUnscheduled && s.SameAccountRetryCount[accountID] < retryLimit {
		s.SameAccountRetryCount[accountID]++
		logger.FromContext(ctx).Warn("gateway.failover_same_account_retry",
			zap.Int64("account_id", accountID),
			zap.Int("upstream_status", failoverErr.StatusCode),
			zap.Int("same_account_retry_count", s.SameAccountRetryCount[accountID]),
			zap.Int("same_account_retry_max", retryLimit),
		)
		if !sleepWithContext(ctx, sameAccountRetryDelay) {
			return FailoverCanceled
		}
		return FailoverContinue
	}

	streakManaged := false
	if !strictFailureUnscheduled {
		// 账号可能因 503 选号退避在同一用户请求内再次被选中；先暂存最后结果，
		// 直到请求成功、终止或真正耗尽时再提交一次。
		event := service.NewAccountFailureStreakEvent(time.Now().UTC())
		snapshot := service.AccountFailureOutcomeSnapshot{Event: event}
		if capturer, ok := gatewayService.(FailoverOutcomeSnapshotCapturer); ok {
			snapshot = capturer.CaptureUpstreamFailureOutcome(ctx, event)
		}
		s.PendingOutcomes[accountID] = failoverPendingOutcome{
			failoverErr: failoverErr,
			snapshot:    snapshot,
		}
		if policy, ok := gatewayService.(FailoverOutcomePolicy); ok {
			streakManaged = policy.IsUpstreamFailureOutcomeManaged(ctx, accountID, failoverErr)
		}
	}

	// 同账号重试用尽，执行临时封禁。由连续错误策略管理的状态码在达到
	// 阈值前保持可调度，达到阈值后已经由持久阻断替代此兼容封禁。
	if failoverErr.RetryableOnSameAccount && !strictFailureUnscheduled && !streakManaged {
		gatewayService.TempUnscheduleRetryableError(ctx, accountID, failoverErr)
	}

	// 加入失败列表
	s.FailedAccountIDs[accountID] = struct{}{}

	// 检查是否耗尽
	if s.SwitchCount >= s.MaxSwitches {
		s.finalizeAllPendingOutcomes(ctx, gatewayService)
		return FailoverExhausted
	}

	// 递增切换计数
	s.SwitchCount++
	logger.FromContext(ctx).Warn("gateway.failover_switch_account",
		zap.Int64("account_id", accountID),
		zap.Int("upstream_status", failoverErr.StatusCode),
		zap.Int("switch_count", s.SwitchCount),
		zap.Int("max_switches", s.MaxSwitches),
	)

	// Antigravity 平台换号线性递增延时
	if platform == service.PlatformAntigravity {
		delay := time.Duration(s.SwitchCount-1) * time.Second
		if !sleepWithContext(ctx, delay) {
			return FailoverCanceled
		}
	}

	return FailoverContinue
}

// HandleSelectionExhausted 处理选号失败（所有候选账号都在排除列表中）时的退避重试决策。
// 针对 Antigravity 单账号分组的 503 (MODEL_CAPACITY_EXHAUSTED) 场景：
// 清除排除列表、等待退避后重新选号。
//
// 返回 FailoverContinue 时，调用方应设置 SingleAccountRetry context 并 continue。
// 返回 FailoverExhausted 时，调用方应返回错误响应。
// 返回 FailoverCanceled 时，调用方应直接 return。
func (s *FailoverState) HandleSelectionExhausted(ctx context.Context, gatewayService any) FailoverAction {
	if s.LastFailoverErr != nil &&
		s.LastFailoverErr.StatusCode == http.StatusServiceUnavailable &&
		s.SwitchCount <= s.MaxSwitches {

		logger.FromContext(ctx).Warn("gateway.failover_single_account_backoff",
			zap.Duration("backoff_delay", singleAccountBackoffDelay),
			zap.Int("switch_count", s.SwitchCount),
			zap.Int("max_switches", s.MaxSwitches),
		)
		if !sleepWithContext(ctx, singleAccountBackoffDelay) {
			return FailoverCanceled
		}
		logger.FromContext(ctx).Warn("gateway.failover_single_account_retry",
			zap.Int("switch_count", s.SwitchCount),
			zap.Int("max_switches", s.MaxSwitches),
		)
		s.FailedAccountIDs = make(map[int64]struct{})
		return FailoverContinue
	}
	s.finalizeAllPendingOutcomes(ctx, gatewayService)
	return FailoverExhausted
}

// RecordSuccessOutcome 将该账号在本次用户请求中的最终结果结算为成功。
// 即使此前因单账号退避记录过失败，最终成功也必须清零连续错误次数。
func (s *FailoverState) RecordSuccessOutcome(ctx context.Context, gatewayService any, accountID int64) {
	if s == nil || accountID <= 0 || ctx == nil || ctx.Err() != nil {
		return
	}
	s.finalizePendingOutcomesExcept(ctx, gatewayService, accountID)
	delete(s.PendingOutcomes, accountID)
	recordFailoverSuccessOutcome(
		ctx,
		gatewayService,
		accountID,
		service.NewAccountFailureStreakEvent(time.Now().UTC()),
	)
	delete(s.RecordedOutcomes, accountID)
}

// RecordTerminalFailureOutcome 记录无法继续切号时已经形成的账号最终失败，例如流内错误。
func (s *FailoverState) RecordTerminalFailureOutcome(ctx context.Context, gatewayService any, accountID int64, failoverErr *service.UpstreamFailoverError) {
	if s == nil || accountID <= 0 || failoverErr == nil || ctx == nil || ctx.Err() != nil {
		return
	}
	s.LastFailoverErr = failoverErr
	s.finalizePendingOutcomesExcept(ctx, gatewayService, accountID)
	delete(s.PendingOutcomes, accountID)
	if _, ok := s.RecordedOutcomes[accountID]; ok {
		return
	}
	if strictScheduler, ok := gatewayService.(FailoverStrictScheduler); ok && strictScheduler.HandleUpstreamFailoverError(ctx, accountID, failoverErr) {
		return
	}
	if managed, blocked, recorded := recordFailoverFailureOutcome(
		ctx,
		gatewayService,
		accountID,
		failoverErr,
		service.NewAccountFailureStreakEvent(time.Now().UTC()),
	); recorded {
		s.RecordedOutcomes[accountID] = failoverRecordedOutcome{managed: managed, blocked: blocked}
	}
}

// RecordOutcomeError 结算明确的非 failover 上游结果。
func (s *FailoverState) RecordOutcomeError(ctx context.Context, gatewayService any, accountID int64, err error, clientDisconnected bool) bool {
	var outcomeErr *service.UpstreamOutcomeError
	if !errors.As(err, &outcomeErr) {
		return false
	}
	if s == nil || outcomeErr == nil || outcomeErr.ClientDisconnect || clientDisconnected || accountID <= 0 || ctx == nil || ctx.Err() != nil {
		return true
	}
	s.finalizePendingOutcomesExcept(ctx, gatewayService, accountID)
	delete(s.PendingOutcomes, accountID)
	if _, recorded := s.RecordedOutcomes[accountID]; recorded {
		return true
	}
	failoverErr := &service.UpstreamFailoverError{
		StatusCode:   outcomeErr.StatusCode,
		ResponseBody: outcomeErr.ResponseBody,
	}
	if strictScheduler, ok := gatewayService.(FailoverStrictScheduler); ok && strictScheduler.HandleUpstreamFailoverError(ctx, accountID, failoverErr) {
		return true
	}
	if managed, blocked, recorded := recordFailoverFailureOutcome(
		ctx,
		gatewayService,
		accountID,
		failoverErr,
		service.NewAccountFailureStreakEvent(time.Now().UTC()),
	); recorded {
		s.RecordedOutcomes[accountID] = failoverRecordedOutcome{managed: managed, blocked: blocked}
	}
	return true
}

func (s *FailoverState) finalizePendingOutcomesExcept(ctx context.Context, gatewayService any, currentAccountID int64) {
	if s == nil || len(s.PendingOutcomes) == 0 || ctx == nil || ctx.Err() != nil {
		return
	}
	for accountID := range s.PendingOutcomes {
		if accountID != currentAccountID {
			s.finalizePendingOutcome(ctx, gatewayService, accountID)
		}
	}
}

func (s *FailoverState) finalizeAllPendingOutcomes(ctx context.Context, gatewayService any) {
	if s == nil || len(s.PendingOutcomes) == 0 || ctx == nil || ctx.Err() != nil {
		return
	}
	accountIDs := make([]int64, 0, len(s.PendingOutcomes))
	for accountID := range s.PendingOutcomes {
		accountIDs = append(accountIDs, accountID)
	}
	for _, accountID := range accountIDs {
		s.finalizePendingOutcome(ctx, gatewayService, accountID)
	}
}

// FinalizePendingOutcomes 在请求任意本地早退时提交尚未结算的账号最终结果。
// 客户端取消时 context 已失效，方法会保持 streak 不变。
func (s *FailoverState) FinalizePendingOutcomes(ctx context.Context, gatewayService any) {
	s.finalizeAllPendingOutcomes(ctx, gatewayService)
}

func (s *FailoverState) finalizePendingOutcome(ctx context.Context, gatewayService any, accountID int64) {
	if s == nil || accountID <= 0 || ctx == nil || ctx.Err() != nil {
		return
	}
	pending, ok := s.PendingOutcomes[accountID]
	if !ok || pending.failoverErr == nil {
		return
	}
	delete(s.PendingOutcomes, accountID)
	if _, recorded := s.RecordedOutcomes[accountID]; recorded {
		return
	}
	if managed, blocked, recorded := recordFailoverFailureOutcome(
		ctx,
		gatewayService,
		accountID,
		pending.failoverErr,
		pending.snapshot.Event,
		pending.snapshot,
	); recorded {
		s.RecordedOutcomes[accountID] = failoverRecordedOutcome{managed: managed, blocked: blocked}
	}
}

func recordFailoverFailureOutcome(
	ctx context.Context,
	gatewayService any,
	accountID int64,
	failoverErr *service.UpstreamFailoverError,
	event service.AccountFailureStreakEvent,
	snapshots ...service.AccountFailureOutcomeSnapshot,
) (managed bool, blocked bool, recorded bool) {
	if len(snapshots) > 0 && snapshots[0].Settings.FailurePolicyRevision > 0 {
		if recorder, ok := gatewayService.(FailoverOutcomeSnapshotRecorder); ok {
			managed, blocked = recorder.RecordUpstreamFailureOutcomeSnapshot(ctx, accountID, failoverErr, snapshots[0])
			return managed, blocked, true
		}
	}
	if recorder, ok := gatewayService.(FailoverOutcomeEventRecorder); ok {
		managed, blocked = recorder.RecordUpstreamFailureOutcomeAt(ctx, accountID, failoverErr, event)
		return managed, blocked, true
	}
	if recorder, ok := gatewayService.(FailoverOutcomeRecorder); ok {
		managed, blocked = recorder.RecordUpstreamFailureOutcome(ctx, accountID, failoverErr)
		return managed, blocked, true
	}
	return false, false, false
}

func recordFailoverSuccessOutcome(
	ctx context.Context,
	gatewayService any,
	accountID int64,
	event service.AccountFailureStreakEvent,
) {
	if recorder, ok := gatewayService.(FailoverSuccessOutcomeEventRecorder); ok {
		recorder.RecordUpstreamSuccessOutcomeAt(ctx, accountID, event)
		return
	}
	if recorder, ok := gatewayService.(FailoverSuccessOutcomeRecorder); ok {
		recorder.RecordUpstreamSuccessOutcome(ctx, accountID)
	}
}

// needForceCacheBilling 判断 failover 时是否需要强制缓存计费。
// 粘性会话切换账号、或上游明确标记时，将 input_tokens 转为 cache_read 计费。
func needForceCacheBilling(hasBoundSession bool, failoverErr *service.UpstreamFailoverError) bool {
	return hasBoundSession || (failoverErr != nil && failoverErr.ForceCacheBilling)
}

// sleepWithContext 等待指定时长，返回 false 表示 context 已取消。
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
