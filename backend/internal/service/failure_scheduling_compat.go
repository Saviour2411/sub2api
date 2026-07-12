package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type failureSchedulingStatePersister interface {
	PersistFailureSchedulingState(ctx context.Context, accountID int64, marker map[string]any, nextRunAt time.Time) (bool, error)
}

type legacyFirstTokenTimeoutStatePersister interface {
	PersistFirstTokenTimeoutState(ctx context.Context, accountID int64, marker map[string]any, nextRunAt time.Time) error
}

type failureSchedulingStateRecoverer interface {
	RecoverFailureSchedulingState(ctx context.Context, accountID int64, incidentID string) (bool, error)
}

const (
	AccountFailureSchedulingStrategyDefault              = "default"
	AccountFailureSchedulingStrategyDisableUntilTestPass = "disable_until_test_pass"
	accountFailureSchedulingStrategyKey                  = "failure_scheduling_strategy"
	accountFailureStrategyUnscheduledKey                 = "failure_strategy_unscheduled"
	accountFailureStrategyUnscheduledAtKey               = "at"
	accountFailureStrategyUnscheduledStatusCodeKey       = "status_code"
	accountFailureStrategyUnscheduledReasonKey           = "reason"
	accountFailureStrategyUnscheduledSourceKey           = "source"
	accountFailureStrategyUnscheduledModelKey            = "model"
	accountFailureStrategyUnscheduledTimeoutSecondsKey   = "timeout_seconds"
	accountFailureStrategyUnscheduledConsecutiveCountKey = "consecutive_count"
	accountFailureStrategyUnscheduledThresholdKey        = "threshold"
	accountFailureStrategyUnscheduledIncidentIDKey       = "incident_id"
	accountFailureStrategyUnscheduledStrictSource        = "strict_failure_strategy"
)

func (a *Account) FailureSchedulingStrategy() string {
	if a == nil || len(a.Extra) == 0 {
		return AccountFailureSchedulingStrategyDefault
	}
	strategy, _ := a.Extra[accountFailureSchedulingStrategyKey].(string)
	strategy = strings.TrimSpace(strategy)
	if strategy == AccountFailureSchedulingStrategyDisableUntilTestPass {
		return strategy
	}
	return AccountFailureSchedulingStrategyDefault
}

// HandleFirstTokenTimeout 记录一次首 Token 超时；达到连续阈值后才持久停调度。
func (s *RateLimitService) HandleFirstTokenTimeout(ctx context.Context, account *Account, model string, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultFirstTokenTimeoutSeconds
	}
	settings := s.gatewayFailureSettings(ctx)
	settings.FirstTokenTimeoutSeconds = timeoutSeconds
	threshold := settings.FirstTokenTimeoutConsecutiveThreshold
	if threshold < 1 || threshold > MaxGatewayFailureConsecutiveThreshold {
		threshold = DefaultGatewaySettings().FirstTokenTimeoutConsecutiveThreshold
		settings.FirstTokenTimeoutConsecutiveThreshold = threshold
	}
	return s.handleFirstTokenTimeoutOutcome(
		ctx,
		account,
		model,
		timeoutSeconds,
		BuildAccountFailureStreakPolicy(AccountFailureStreakSourceFirstTokenTimeout, settings),
		threshold,
		NewAccountFailureStreakEvent(time.Now().UTC()),
		false,
	)
}

func (s *RateLimitService) handleFirstTokenTimeoutOutcome(
	ctx context.Context,
	account *Account,
	model string,
	timeoutSeconds int,
	policy AccountFailureStreakPolicy,
	threshold int,
	event AccountFailureStreakEvent,
	checkCurrentPolicy bool,
) error {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 {
		return nil
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultFirstTokenTimeoutSeconds
	}
	if threshold < 1 || threshold > MaxGatewayFailureConsecutiveThreshold {
		threshold = DefaultGatewaySettings().FirstTokenTimeoutConsecutiveThreshold
	}
	if policy.Revision <= 0 || strings.TrimSpace(policy.Fingerprint) == "" {
		settings := s.gatewayFailureSettings(ctx)
		settings.FirstTokenTimeoutSeconds = timeoutSeconds
		settings.FirstTokenTimeoutConsecutiveThreshold = threshold
		policy = BuildAccountFailureStreakPolicy(AccountFailureStreakSourceFirstTokenTimeout, settings)
	}
	if checkCurrentPolicy {
		current := BuildAccountFailureStreakPolicy(
			AccountFailureStreakSourceFirstTokenTimeout,
			s.gatewayFailureSettings(ctx),
		)
		if policy.Revision != current.Revision || policy.Fingerprint != current.Fingerprint {
			logger.FromContext(ctx).Info("忽略旧策略下形成的首 Token 超时结果",
				zap.Int64("account_id", account.ID),
				zap.Int64("event_policy_revision", policy.Revision),
				zap.Int64("current_policy_revision", current.Revision),
			)
			return nil
		}
	}
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.ID) == "" {
		event = NewAccountFailureStreakEvent(time.Now().UTC())
	}
	now := event.OccurredAt.UTC()
	if s.accountFailureStreakCache == nil {
		logger.FromContext(ctx).Warn("首 Token 连续超时缓存未配置，跳过账号停调度",
			zap.Int64("account_id", account.ID),
		)
		return nil
	}
	opCtx, cancel := failureSchedulingOperationContext(ctx)
	streak, err := s.accountFailureStreakCache.ApplyOutcome(
		opCtx,
		account.ID,
		AccountFailureStreakSourceFirstTokenTimeout,
		policy,
		AccountFailureStreakOutcomeIncrement,
		event,
	)
	cancel()
	if err != nil {
		logger.FromContext(ctx).Warn("记录首 Token 连续超时失败，保持账号可调度",
			zap.Int64("account_id", account.ID),
			zap.Error(err),
		)
		return err
	}
	if !streak.Applied || streak.PolicyRevision != policy.Revision || streak.Count < int64(threshold) {
		return nil
	}

	model = strings.TrimSpace(model)
	incidentID := uuid.NewString()
	reason := fmt.Sprintf(
		"source=first_token_timeout model=%s timeout_seconds=%d status_code=%d consecutive_count=%d threshold=%d at=%s incident_id=%s",
		model,
		timeoutSeconds,
		http.StatusGatewayTimeout,
		streak.Count,
		threshold,
		now.Format(time.RFC3339),
		incidentID,
	)
	marker := BuildFailureStrategyUnscheduledMarker(http.StatusGatewayTimeout, reason, now)
	marker[accountFailureStrategyUnscheduledSourceKey] = "first_token_timeout"
	marker[accountFailureStrategyUnscheduledModelKey] = model
	marker[accountFailureStrategyUnscheduledTimeoutSecondsKey] = timeoutSeconds
	marker[accountFailureStrategyUnscheduledConsecutiveCountKey] = streak.Count
	marker[accountFailureStrategyUnscheduledThresholdKey] = threshold
	marker[accountFailureStrategyUnscheduledIncidentIDKey] = incidentID

	if ctx == nil {
		ctx = context.Background()
	}

	created, persistErr := s.persistFailureSchedulingIncident(ctx, account, marker, "first_token_timeout")
	if persistErr != nil {
		logger.FromContext(ctx).With(
			zap.String("component", "service.first_token_timeout"),
			zap.Int64("account_id", account.ID),
			zap.Error(persistErr),
		).Error("首 Token 超时状态原子持久化失败，账号保持原调度状态")
		return persistErr
	}
	if !created {
		return nil
	}
	return nil
}

// persistFailureSchedulingIncident 持久创建一个不会覆盖现有事故的停调度状态。
// 普通上游错误和首 Token 超时共用此入口，确保数据库、自动测活和运行时缓存语义一致。
func (s *RateLimitService) persistFailureSchedulingIncident(
	ctx context.Context,
	account *Account,
	marker map[string]any,
	source string,
) (bool, error) {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 || len(marker) == 0 {
		return false, nil
	}
	now := time.Now().UTC()
	created := false
	var persistErr error
	if persister, ok := s.accountRepo.(failureSchedulingStatePersister); ok {
		opCtx, cancel := failureSchedulingOperationContext(ctx)
		created, persistErr = persister.PersistFailureSchedulingState(opCtx, account.ID, marker, now)
		cancel()
	} else if source == string(AccountFailureStreakSourceFirstTokenTimeout) {
		if persister, ok := s.accountRepo.(legacyFirstTokenTimeoutStatePersister); ok {
			opCtx, cancel := failureSchedulingOperationContext(ctx)
			persistErr = persister.PersistFirstTokenTimeoutState(opCtx, account.ID, marker, now)
			cancel()
			created = persistErr == nil
		} else {
			created, persistErr = s.persistFailureSchedulingStateCompat(ctx, account.ID, marker)
		}
	} else if source == accountFailureStrategyUnscheduledStrictSource && s.autoManagedProbe == nil {
		// 仅兼容尚未实现条件事故事务的旧测试桩。生产仓储实现
		// failureSchedulingStatePersister，不会进入此非原子分支。
		created, persistErr = s.persistStrictFailureSchedulingStateCompat(ctx, account.ID, marker)
	} else {
		created, persistErr = s.persistFailureSchedulingStateCompat(ctx, account.ID, marker)
	}
	if persistErr != nil || !created {
		return created, persistErr
	}

	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	account.Extra[accountFailureStrategyUnscheduledKey] = marker
	account.Schedulable = false
	source = strings.TrimSpace(source)
	if source == "" {
		source = "upstream_error"
	}
	// OpenAI 高速调度缓存需要同步阻断；测活成功路径会清除此状态。
	s.notifyAccountSchedulingBlocked(account, now.AddDate(100, 0, 0), source)
	return true, nil
}

func (s *RateLimitService) persistStrictFailureSchedulingStateCompat(
	ctx context.Context,
	accountID int64,
	marker map[string]any,
) (bool, error) {
	run := func(fn func(context.Context) error) error {
		opCtx, cancel := failureSchedulingOperationContext(ctx)
		defer cancel()
		return fn(opCtx)
	}
	if err := run(func(opCtx context.Context) error {
		return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: marker})
	}); err != nil {
		return false, fmt.Errorf("写入严格策略不可调度标记: %w", err)
	}
	if err := run(func(opCtx context.Context) error {
		return s.accountRepo.SetSchedulable(opCtx, accountID, false)
	}); err != nil {
		rollbackErr := run(func(opCtx context.Context) error {
			return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: nil})
		})
		return false, errors.Join(fmt.Errorf("设置严格策略账号不可调度: %w", err), rollbackErr)
	}
	return true, nil
}

// persistFailureSchedulingStateCompat 只用于不具备事务扩展的测试桩。
// 任一步失败都会短路，并尽力撤销此前写入，生产仓储不会进入此路径。
func (s *RateLimitService) persistFailureSchedulingStateCompat(ctx context.Context, accountID int64, marker map[string]any) (bool, error) {
	if s.autoManagedProbe == nil {
		return false, errors.New("auto managed probe scheduler is not configured")
	}
	run := func(fn func(context.Context) error) error {
		opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return fn(opCtx)
	}
	if err := run(func(opCtx context.Context) error {
		return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: marker})
	}); err != nil {
		return false, fmt.Errorf("写入不可调度标记: %w", err)
	}
	if err := run(func(opCtx context.Context) error {
		return s.accountRepo.SetSchedulable(opCtx, accountID, false)
	}); err != nil {
		rollbackErr := run(func(opCtx context.Context) error {
			return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: nil})
		})
		return false, errors.Join(fmt.Errorf("设置账号不可调度: %w", err), rollbackErr)
	}
	if err := run(func(opCtx context.Context) error {
		return s.autoManagedProbe.EnsureAutoManagedProbe(opCtx, accountID)
	}); err != nil {
		restoreErr := run(func(opCtx context.Context) error {
			return s.accountRepo.SetSchedulable(opCtx, accountID, true)
		})
		clearMarkerErr := run(func(opCtx context.Context) error {
			return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: nil})
		})
		return false, errors.Join(fmt.Errorf("补建自动测活计划: %w", err), restoreErr, clearMarkerErr)
	}
	return true, nil
}

func (s *RateLimitService) gatewayFailureSettings(ctx context.Context) GatewaySettings {
	if s != nil && s.settingService != nil {
		opCtx, cancel := failureSchedulingOperationContext(ctx)
		defer cancel()
		return s.settingService.GetGatewayRuntime(opCtx)
	}
	return DefaultGatewaySettings()
}

func failureSchedulingOperationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (s *RateLimitService) resetFailureStreak(ctx context.Context, accountID int64, source AccountFailureStreakSource, timeoutSeconds int) {
	s.resetFailureStreakAt(ctx, accountID, source, timeoutSeconds, time.Now().UTC())
}

func (s *RateLimitService) resetFailureStreakAt(
	ctx context.Context,
	accountID int64,
	source AccountFailureStreakSource,
	timeoutSeconds int,
	occurredAt time.Time,
) {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	} else {
		occurredAt = occurredAt.UTC()
	}
	s.resetFailureStreakEvent(ctx, accountID, source, timeoutSeconds, NewAccountFailureStreakEvent(occurredAt))
}

func (s *RateLimitService) resetFailureStreakEvent(
	ctx context.Context,
	accountID int64,
	source AccountFailureStreakSource,
	timeoutSeconds int,
	event AccountFailureStreakEvent,
) {
	if s == nil || s.accountFailureStreakCache == nil || accountID <= 0 {
		return
	}
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.ID) == "" {
		event = NewAccountFailureStreakEvent(time.Now().UTC())
	}
	settings := s.gatewayFailureSettings(ctx)
	if source == AccountFailureStreakSourceFirstTokenTimeout && timeoutSeconds > 0 {
		settings.FirstTokenTimeoutSeconds = timeoutSeconds
	}
	opCtx, cancel := failureSchedulingOperationContext(ctx)
	defer cancel()
	if _, err := s.accountFailureStreakCache.ApplyOutcome(
		opCtx,
		accountID,
		source,
		BuildAccountFailureStreakPolicy(source, settings),
		AccountFailureStreakOutcomeReset,
		event,
	); err != nil {
		logger.FromContext(ctx).Warn("清理账号连续失败次数失败",
			zap.Int64("account_id", accountID),
			zap.String("source", string(source)),
			zap.Error(err),
		)
	}
}

func (s *RateLimitService) resetFirstTokenTimeoutStreak(ctx context.Context, accountID int64, timeoutSeconds int) {
	s.resetFailureStreak(ctx, accountID, AccountFailureStreakSourceFirstTokenTimeout, timeoutSeconds)
}

// FailureStrategyUnscheduledIncident 返回账号当前事故的 ID 和来源。
func (a *Account) FailureStrategyUnscheduledIncident() (string, AccountFailureStreakSource) {
	marker := failureStrategyUnscheduledMarker(a)
	if len(marker) == 0 {
		return "", ""
	}
	incidentID := strings.TrimSpace(failureMarkerString(marker, accountFailureStrategyUnscheduledIncidentIDKey))
	source := AccountFailureStreakSource(strings.TrimSpace(failureMarkerString(marker, accountFailureStrategyUnscheduledSourceKey)))
	return incidentID, source
}

func failureStrategyUnscheduledMarker(account *Account) map[string]any {
	if account == nil || len(account.Extra) == 0 {
		return nil
	}
	raw, ok := account.Extra[accountFailureStrategyUnscheduledKey]
	if !ok || raw == nil {
		return nil
	}
	switch marker := raw.(type) {
	case map[string]any:
		return marker
	case map[string]string:
		out := make(map[string]any, len(marker))
		for key, value := range marker {
			out[key] = value
		}
		return out
	default:
		return nil
	}
}

func failureMarkerString(marker map[string]any, key string) string {
	if len(marker) == 0 {
		return ""
	}
	value, ok := marker[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func failureMarkerInt(marker map[string]any, key string) int {
	if len(marker) == 0 {
		return 0
	}
	switch value := marker[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscan(strings.TrimSpace(value), &parsed); err == nil {
			return parsed
		}
	}
	return 0
}

// RecoverAccountAfterSuccessfulTestIncident 仅恢复测活开始前捕获的事故。
// incidentID 不匹配表示测活期间出现了新事故，此时保持账号和自动计划不变。
func (s *RateLimitService) RecoverAccountAfterSuccessfulTestIncident(
	ctx context.Context,
	accountID int64,
	incidentID string,
	recoveryStartedAt ...time.Time,
) (*SuccessfulTestRecoveryResult, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return &SuccessfulTestRecoveryResult{}, nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	incidentID = strings.TrimSpace(incidentID)
	currentIncidentID, _ := account.FailureStrategyUnscheduledIncident()
	if incidentID == "" {
		if currentIncidentID != "" {
			return &SuccessfulTestRecoveryResult{}, nil
		}
		return s.RecoverAccountAfterSuccessfulTest(ctx, accountID)
	}
	if currentIncidentID != incidentID {
		return &SuccessfulTestRecoveryResult{}, nil
	}

	recovered, err := s.recoverFailureSchedulingIncident(ctx, account, incidentID)
	if err != nil {
		return nil, err
	}
	if !recovered {
		return &SuccessfulTestRecoveryResult{}, nil
	}

	marker := failureStrategyUnscheduledMarker(account)
	timeoutSeconds := failureMarkerInt(marker, accountFailureStrategyUnscheduledTimeoutSecondsKey)
	resetOccurredAt := time.Now().UTC()
	if len(recoveryStartedAt) > 0 && !recoveryStartedAt[0].IsZero() {
		resetOccurredAt = recoveryStartedAt[0].UTC()
	}
	// 所有失败事故测活成功后都必须清零两类 streak，包括账号显式配置触发的
	// strict_failure_strategy；否则旧计数会跨事故残留，恢复后过早再次停调度。
	// 使用测活开始时间作为清零事件。测活开始后形成的新失败时间更晚，
	// Redis Lua 会拒绝这条旧清零，避免旧测活结果清除新 streak。
	s.resetFailureStreakAt(ctx, accountID, AccountFailureStreakSourceFirstTokenTimeout, timeoutSeconds, resetOccurredAt)
	s.resetFailureStreakAt(ctx, accountID, AccountFailureStreakSourceUpstreamError, 0, resetOccurredAt)
	s.notifyAccountSchedulingBlockCleared(accountID)
	s.restoreConcurrentFailureSchedulingBlock(ctx, accountID)
	return &SuccessfulTestRecoveryResult{ClearedRateLimit: true}, nil
}

func (s *RateLimitService) restoreConcurrentFailureSchedulingBlock(ctx context.Context, accountID int64) {
	fresh, err := s.loadAccountForFailureScheduling(ctx, accountID)
	if err != nil || fresh == nil || !fresh.HasFailureStrategyUnscheduledMarker() {
		if err != nil {
			logger.FromContext(ctx).Warn("复核并发失败事故失败",
				zap.Int64("account_id", accountID),
				zap.Error(err),
			)
		}
		return
	}
	_, source := fresh.FailureStrategyUnscheduledIncident()
	reason := strings.TrimSpace(string(source))
	if reason == "" {
		reason = "failure_scheduling_incident"
	}
	// 新事故可能在旧事故 CAS 提交后、运行时缓存清理前创建。复核数据库
	// 并重新施加阻断，保证旧测活最终不会把新事故暴露给高速调度缓存。
	s.notifyAccountSchedulingBlocked(fresh, time.Now().UTC().AddDate(100, 0, 0), reason)
}

func (s *RateLimitService) recoverFailureSchedulingIncident(ctx context.Context, account *Account, incidentID string) (bool, error) {
	if recoverer, ok := s.accountRepo.(failureSchedulingStateRecoverer); ok {
		opCtx, cancel := failureSchedulingOperationContext(ctx)
		recovered, err := recoverer.RecoverFailureSchedulingState(opCtx, account.ID, incidentID)
		cancel()
		return recovered, err
	}

	// 仅供旧测试桩使用；生产仓储使用上面的单条条件更新事务。
	if err := s.accountRepo.SetSchedulable(ctx, account.ID, true); err != nil {
		return false, err
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{accountFailureStrategyUnscheduledKey: nil}); err != nil {
		rollbackErr := s.accountRepo.SetSchedulable(ctx, account.ID, false)
		return false, errors.Join(err, rollbackErr)
	}
	return true, nil
}

func (a *Account) ShouldDisableSchedulingOnUpstreamError() bool {
	return a.FailureSchedulingStrategy() == AccountFailureSchedulingStrategyDisableUntilTestPass
}

func (a *Account) HasFailureStrategyUnscheduledMarker() bool {
	if a == nil || len(a.Extra) == 0 {
		return false
	}
	marker, ok := a.Extra[accountFailureStrategyUnscheduledKey]
	if !ok || marker == nil {
		return false
	}
	switch v := marker.(type) {
	case map[string]any:
		return len(v) > 0
	case map[string]string:
		return len(v) > 0
	default:
		return true
	}
}

func BuildFailureStrategyUnscheduledMarker(statusCode int, reason string, now time.Time) map[string]any {
	if now.IsZero() {
		now = time.Now()
	}
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "upstream error"
	}
	return map[string]any{
		accountFailureStrategyUnscheduledAtKey:         now.UTC().Format(time.RFC3339),
		accountFailureStrategyUnscheduledStatusCodeKey: statusCode,
		accountFailureStrategyUnscheduledReasonKey:     reason,
	}
}

func ClearFailureStrategyUnscheduledMarker(extra map[string]any) map[string]any {
	if extra == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(extra))
	for key, value := range extra {
		if key == accountFailureStrategyUnscheduledKey {
			continue
		}
		out[key] = value
	}
	return out
}

func (s *RateLimitService) ShouldDisableSchedulingOnUpstreamError(ctx context.Context, account *Account) bool {
	if account == nil {
		return false
	}
	localStrict := account.ShouldDisableSchedulingOnUpstreamError()
	if s == nil || s.accountRepo == nil || account.ID <= 0 {
		return localStrict
	}
	readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var fresh *Account
	var err error
	func() {
		defer func() {
			if recover() != nil {
				fresh = nil
				err = context.Canceled
			}
		}()
		fresh, err = s.accountRepo.GetByID(readCtx, account.ID)
	}()
	if err != nil || fresh == nil {
		return localStrict
	}
	if marker := failureStrategyUnscheduledMarker(fresh); len(marker) > 0 {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		account.Extra[accountFailureStrategyUnscheduledKey] = marker
		account.Schedulable = fresh.Schedulable
	}
	if fresh.ShouldDisableSchedulingOnUpstreamError() {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		account.Extra[accountFailureSchedulingStrategyKey] = AccountFailureSchedulingStrategyDisableUntilTestPass
		return true
	}
	return localStrict
}

func (s *RateLimitService) HandleStrictFailureScheduling(ctx context.Context, account *Account, statusCode int, reason string) bool {
	if s == nil || account == nil || !s.ShouldDisableSchedulingOnUpstreamError(ctx, account) {
		return false
	}
	if account.HasFailureStrategyUnscheduledMarker() {
		return true
	}
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	now := time.Now().UTC()
	marker := BuildFailureStrategyUnscheduledMarker(statusCode, reason, now)
	marker[accountFailureStrategyUnscheduledSourceKey] = accountFailureStrategyUnscheduledStrictSource
	marker[accountFailureStrategyUnscheduledIncidentIDKey] = uuid.NewString()

	created, err := s.persistFailureSchedulingIncident(ctx, account, marker, accountFailureStrategyUnscheduledStrictSource)
	if err != nil {
		logger.FromContext(ctx).Error("持久化严格失败停调度事故失败",
			zap.Int64("account_id", account.ID),
			zap.Int("status_code", statusCode),
			zap.Error(err),
		)
		return false
	}
	if created {
		return true
	}

	// 条件写未命中表示并发路径已经创建事故。重新读取并复制真实 marker，
	// 不得把本次生成的 incident_id 写回覆盖先到事故。
	fresh, loadErr := s.loadAccountForFailureScheduling(ctx, account.ID)
	if loadErr != nil {
		logger.FromContext(ctx).Warn("读取并发严格失败事故失败",
			zap.Int64("account_id", account.ID),
			zap.Error(loadErr),
		)
		return true
	}
	if existingMarker := failureStrategyUnscheduledMarker(fresh); len(existingMarker) > 0 {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		account.Extra[accountFailureStrategyUnscheduledKey] = existingMarker
		account.Schedulable = fresh.Schedulable
	}
	return true
}

func (s *RateLimitService) HandleUpstreamFailoverError(ctx context.Context, account *Account, failoverErr *UpstreamFailoverError) bool {
	if failoverErr == nil {
		return false
	}
	statusCode := failoverErr.StatusCode
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	return s.HandleStrictFailureScheduling(ctx, account, statusCode, "upstream failover error")
}
