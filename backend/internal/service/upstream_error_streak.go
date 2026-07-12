package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RecordUpstreamFailureOutcome 记录同一用户请求中该账号的最终上游结果。
// 返回的 managed 表示状态码由全局连续错误策略接管，调用方应跳过旧的单次临时停调度逻辑。
func (s *RateLimitService) RecordUpstreamFailureOutcome(
	ctx context.Context,
	accountID int64,
	failoverErr *UpstreamFailoverError,
) (managed bool, blocked bool) {
	return s.RecordUpstreamFailureOutcomeAt(ctx, accountID, failoverErr, NewAccountFailureStreakEvent(time.Now().UTC()))
}

// RecordUpstreamFailureOutcomeAt 使用账号结果形成时创建的事件结算，避免延迟提交改变并发顺序。
func (s *RateLimitService) RecordUpstreamFailureOutcomeAt(
	ctx context.Context,
	accountID int64,
	failoverErr *UpstreamFailoverError,
	event AccountFailureStreakEvent,
) (managed bool, blocked bool) {
	return s.recordUpstreamFailureOutcomeSnapshot(
		ctx,
		accountID,
		failoverErr,
		s.captureAccountFailureOutcome(ctx, event),
		false,
	)
}

// RecordUpstreamFailureOutcomeSnapshot 只提交结果形成时捕获的策略；代次已变化时忽略旧结果。
func (s *RateLimitService) RecordUpstreamFailureOutcomeSnapshot(
	ctx context.Context,
	accountID int64,
	failoverErr *UpstreamFailoverError,
	snapshot AccountFailureOutcomeSnapshot,
) (managed bool, blocked bool) {
	return s.recordUpstreamFailureOutcomeSnapshot(ctx, accountID, failoverErr, snapshot, true)
}

func (s *RateLimitService) captureAccountFailureOutcome(
	ctx context.Context,
	event AccountFailureStreakEvent,
) AccountFailureOutcomeSnapshot {
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.ID) == "" {
		event = NewAccountFailureStreakEvent(time.Now().UTC())
	}
	return AccountFailureOutcomeSnapshot{
		Event:    event,
		Settings: cloneGatewaySettings(s.gatewayFailureSettings(ctx)),
	}
}

func (s *RateLimitService) recordUpstreamFailureOutcomeSnapshot(
	ctx context.Context,
	accountID int64,
	failoverErr *UpstreamFailoverError,
	snapshot AccountFailureOutcomeSnapshot,
	checkCurrentPolicy bool,
) (managed bool, blocked bool) {
	if s == nil || accountID <= 0 || failoverErr == nil {
		return false, false
	}
	event := snapshot.Event
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.ID) == "" {
		event = NewAccountFailureStreakEvent(time.Now().UTC())
	}
	if failoverErr.FirstTokenTimeout {
		return false, false
	}
	settings := snapshot.Settings
	if settings.FailurePolicyRevision <= 0 {
		settings = s.gatewayFailureSettings(ctx)
	}
	statusCode := failoverErr.StatusCode
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	managed = containsHTTPStatus(settings.UpstreamErrorStatusCodes, statusCode)
	if checkCurrentPolicy {
		current := s.gatewayFailureSettings(ctx)
		if settings.FailurePolicyRevision != current.FailurePolicyRevision ||
			BuildGatewayFailurePolicyFingerprint(settings) != BuildGatewayFailurePolicyFingerprint(current) {
			logger.FromContext(ctx).Info("忽略旧策略下形成的账号上游结果",
				zap.Int64("account_id", accountID),
				zap.Int64("event_policy_revision", settings.FailurePolicyRevision),
				zap.Int64("current_policy_revision", current.FailurePolicyRevision),
			)
			return managed, false
		}
	}
	// 首 Token 连续超时按一次用户请求中的账号最终结果结算；任何明确的
	// 非超时结果都在这里清零，避免内部协议重试提前改写 streak。
	s.resetFailureStreakEvent(ctx, accountID, AccountFailureStreakSourceFirstTokenTimeout, 0, event)

	if !managed {
		s.resetFailureStreakEvent(ctx, accountID, AccountFailureStreakSourceUpstreamError, 0, event)
		return false, false
	}

	// 认证、配额和限流错误继续由已有即时策略处理，不能被全局连续阈值延后。
	if preservesExistingImmediateFailurePolicy(statusCode) {
		return false, false
	}
	account, err := s.loadAccountForFailureScheduling(ctx, accountID)
	if err != nil {
		logger.FromContext(ctx).Warn("读取连续上游错误账号失败，保持账号可调度",
			zap.Int64("account_id", accountID),
			zap.Int("status_code", statusCode),
			zap.Error(err),
		)
		return true, false
	}
	if account == nil {
		return true, false
	}
	if account.HasFailureStrategyUnscheduledMarker() {
		return true, true
	}
	if account.IsCustomErrorCodesEnabled() || account.ShouldDisableSchedulingOnUpstreamError() {
		return false, false
	}
	if s.accountFailureStreakCache == nil {
		logger.FromContext(ctx).Warn("账号连续上游错误缓存未配置，保持账号可调度",
			zap.Int64("account_id", accountID),
			zap.Int("status_code", statusCode),
		)
		return true, false
	}

	now := event.OccurredAt.UTC()
	opCtx, cancel := failureSchedulingOperationContext(ctx)
	policy := BuildAccountFailureStreakPolicy(AccountFailureStreakSourceUpstreamError, settings)
	streak, err := s.accountFailureStreakCache.ApplyOutcome(
		opCtx,
		accountID,
		AccountFailureStreakSourceUpstreamError,
		policy,
		AccountFailureStreakOutcomeIncrement,
		event,
	)
	cancel()
	if err != nil {
		logger.FromContext(ctx).Warn("记录账号连续上游错误失败，保持账号可调度",
			zap.Int64("account_id", accountID),
			zap.Int("status_code", statusCode),
			zap.Error(err),
		)
		return true, false
	}
	threshold := settings.UpstreamErrorConsecutiveThreshold
	if threshold < 1 || threshold > MaxGatewayFailureConsecutiveThreshold {
		threshold = DefaultGatewaySettings().UpstreamErrorConsecutiveThreshold
	}
	if !streak.Applied || streak.PolicyRevision != policy.Revision || streak.Count < int64(threshold) {
		return true, false
	}

	incidentID := uuid.NewString()
	reason := upstreamFailureSchedulingReason(failoverErr, statusCode)
	marker := BuildFailureStrategyUnscheduledMarker(statusCode, reason, now)
	marker[accountFailureStrategyUnscheduledSourceKey] = string(AccountFailureStreakSourceUpstreamError)
	marker[accountFailureStrategyUnscheduledConsecutiveCountKey] = streak.Count
	marker[accountFailureStrategyUnscheduledThresholdKey] = threshold
	marker[accountFailureStrategyUnscheduledIncidentIDKey] = incidentID

	_, err = s.persistFailureSchedulingIncident(ctx, account, marker, string(AccountFailureStreakSourceUpstreamError))
	if err != nil {
		logger.FromContext(ctx).Error("持久化连续上游错误停调度事故失败，账号保持原调度状态",
			zap.Int64("account_id", accountID),
			zap.Int("status_code", statusCode),
			zap.Int64("consecutive_count", streak.Count),
			zap.Int("threshold", threshold),
			zap.Error(err),
		)
		return true, false
	}
	// 阈值后的条件写成功表示本请求创建了事故，或并发请求已经抢先创建事故。
	// 两种情况都不能让长连接继续使用该账号；winner 负责持久状态和缓存同步。
	return true, true
}

// IsUpstreamFailureOutcomeManaged 仅判断全局连续错误策略是否接管该账号结果，
// 不修改 streak；统一 FailoverState 会在整个用户请求结束时再提交最终结果。
func (s *RateLimitService) IsUpstreamFailureOutcomeManaged(
	ctx context.Context,
	accountID int64,
	failoverErr *UpstreamFailoverError,
) bool {
	if s == nil || accountID <= 0 || failoverErr == nil || failoverErr.FirstTokenTimeout {
		return false
	}
	statusCode := failoverErr.StatusCode
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	settings := s.gatewayFailureSettings(ctx)
	if !containsHTTPStatus(settings.UpstreamErrorStatusCodes, statusCode) || preservesExistingImmediateFailurePolicy(statusCode) {
		return false
	}
	account, err := s.loadAccountForFailureScheduling(ctx, accountID)
	if err != nil || account == nil {
		return true
	}
	if account.IsCustomErrorCodesEnabled() || account.ShouldDisableSchedulingOnUpstreamError() {
		return false
	}
	return true
}

// RecordUpstreamSuccessOutcome 清零该账号的连续普通上游错误次数。
func (s *RateLimitService) RecordUpstreamSuccessOutcome(ctx context.Context, accountID int64) {
	s.RecordUpstreamSuccessOutcomeAt(ctx, accountID, NewAccountFailureStreakEvent(time.Now().UTC()))
}

func (s *RateLimitService) RecordUpstreamSuccessOutcomeAt(ctx context.Context, accountID int64, event AccountFailureStreakEvent) {
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.ID) == "" {
		event = NewAccountFailureStreakEvent(time.Now().UTC())
	}
	s.resetFailureStreakEvent(ctx, accountID, AccountFailureStreakSourceFirstTokenTimeout, 0, event)
	s.resetFailureStreakEvent(ctx, accountID, AccountFailureStreakSourceUpstreamError, 0, event)
}

func (s *RateLimitService) loadAccountForFailureScheduling(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return nil, nil
	}
	opCtx, cancel := failureSchedulingOperationContext(ctx)
	defer cancel()
	return s.accountRepo.GetByID(opCtx, accountID)
}

func containsHTTPStatus(codes []int, statusCode int) bool {
	for _, code := range codes {
		if code == statusCode {
			return true
		}
	}
	return false
}

func preservesExistingImmediateFailurePolicy(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		529:
		return true
	default:
		return false
	}
}

func upstreamFailureSchedulingReason(failoverErr *UpstreamFailoverError, statusCode int) string {
	message := ""
	if failoverErr != nil {
		message = strings.TrimSpace(extractUpstreamErrorMessage(failoverErr.ResponseBody))
		message = sanitizeUpstreamErrorMessage(message)
	}
	if message == "" {
		message = fmt.Sprintf("上游返回 HTTP %d", statusCode)
	}
	return truncateForLog([]byte(message), 512)
}

// RecordUpstreamFailureOutcome 让统一 FailoverState 使用 GatewayService 记录最终账号结果。
func (s *GatewayService) RecordUpstreamFailureOutcome(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) (bool, bool) {
	if s == nil || s.rateLimitService == nil {
		return false, false
	}
	return s.rateLimitService.RecordUpstreamFailureOutcome(ctx, accountID, failoverErr)
}

func (s *GatewayService) RecordUpstreamFailureOutcomeAt(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError, event AccountFailureStreakEvent) (bool, bool) {
	if s == nil || s.rateLimitService == nil {
		return false, false
	}
	return s.rateLimitService.RecordUpstreamFailureOutcomeAt(ctx, accountID, failoverErr, event)
}

func (s *GatewayService) CaptureUpstreamFailureOutcome(ctx context.Context, event AccountFailureStreakEvent) AccountFailureOutcomeSnapshot {
	if s == nil || s.rateLimitService == nil {
		return AccountFailureOutcomeSnapshot{Event: event, Settings: DefaultGatewaySettings()}
	}
	return s.rateLimitService.captureAccountFailureOutcome(ctx, event)
}

func (s *GatewayService) RecordUpstreamFailureOutcomeSnapshot(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError, snapshot AccountFailureOutcomeSnapshot) (bool, bool) {
	if s == nil || s.rateLimitService == nil {
		return false, false
	}
	return s.rateLimitService.RecordUpstreamFailureOutcomeSnapshot(ctx, accountID, failoverErr, snapshot)
}

func (s *GatewayService) RecordUpstreamSuccessOutcome(ctx context.Context, accountID int64) {
	if s != nil && s.rateLimitService != nil {
		s.rateLimitService.RecordUpstreamSuccessOutcome(ctx, accountID)
	}
}

func (s *GatewayService) RecordUpstreamSuccessOutcomeAt(ctx context.Context, accountID int64, event AccountFailureStreakEvent) {
	if s != nil && s.rateLimitService != nil {
		s.rateLimitService.RecordUpstreamSuccessOutcomeAt(ctx, accountID, event)
	}
}

func (s *GatewayService) IsUpstreamFailureOutcomeManaged(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) bool {
	return s != nil && s.rateLimitService != nil && s.rateLimitService.IsUpstreamFailureOutcomeManaged(ctx, accountID, failoverErr)
}

// HandleUpstreamFailoverError 保留账号级严格停调度策略在全局连续错误策略之前的优先级。
func (s *GatewayService) HandleUpstreamFailoverError(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) bool {
	if s == nil || s.rateLimitService == nil || failoverErr == nil {
		return false
	}
	account, err := s.rateLimitService.loadAccountForFailureScheduling(ctx, accountID)
	if err != nil || account == nil {
		return false
	}
	blocked := s.rateLimitService.HandleUpstreamFailoverError(ctx, account, failoverErr)
	if blocked && !failoverErr.FirstTokenTimeout {
		s.rateLimitService.resetFirstTokenTimeoutStreak(ctx, accountID, 0)
	}
	return blocked
}

func (s *OpenAIGatewayService) RecordUpstreamFailureOutcome(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) (bool, bool) {
	if s == nil || s.rateLimitService == nil {
		return false, false
	}
	return s.rateLimitService.RecordUpstreamFailureOutcome(ctx, accountID, failoverErr)
}

func (s *OpenAIGatewayService) RecordUpstreamFailureOutcomeAt(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError, event AccountFailureStreakEvent) (bool, bool) {
	if s == nil || s.rateLimitService == nil {
		return false, false
	}
	return s.rateLimitService.RecordUpstreamFailureOutcomeAt(ctx, accountID, failoverErr, event)
}

func (s *OpenAIGatewayService) CaptureUpstreamFailureOutcome(ctx context.Context, event AccountFailureStreakEvent) AccountFailureOutcomeSnapshot {
	if s == nil || s.rateLimitService == nil {
		return AccountFailureOutcomeSnapshot{Event: event, Settings: DefaultGatewaySettings()}
	}
	return s.rateLimitService.captureAccountFailureOutcome(ctx, event)
}

func (s *OpenAIGatewayService) RecordUpstreamFailureOutcomeSnapshot(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError, snapshot AccountFailureOutcomeSnapshot) (bool, bool) {
	if s == nil || s.rateLimitService == nil {
		return false, false
	}
	return s.rateLimitService.RecordUpstreamFailureOutcomeSnapshot(ctx, accountID, failoverErr, snapshot)
}

func (s *OpenAIGatewayService) RecordUpstreamSuccessOutcome(ctx context.Context, accountID int64) {
	if s != nil && s.rateLimitService != nil {
		s.rateLimitService.RecordUpstreamSuccessOutcome(ctx, accountID)
	}
}

func (s *OpenAIGatewayService) RecordUpstreamSuccessOutcomeAt(ctx context.Context, accountID int64, event AccountFailureStreakEvent) {
	if s != nil && s.rateLimitService != nil {
		s.rateLimitService.RecordUpstreamSuccessOutcomeAt(ctx, accountID, event)
	}
}

func (s *OpenAIGatewayService) IsUpstreamFailureOutcomeManaged(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) bool {
	return s != nil && s.rateLimitService != nil && s.rateLimitService.IsUpstreamFailureOutcomeManaged(ctx, accountID, failoverErr)
}

func (s *OpenAIGatewayService) HandleUpstreamFailoverError(ctx context.Context, accountID int64, failoverErr *UpstreamFailoverError) bool {
	if s == nil || s.rateLimitService == nil || failoverErr == nil {
		return false
	}
	account, err := s.rateLimitService.loadAccountForFailureScheduling(ctx, accountID)
	if err != nil || account == nil {
		return false
	}
	blocked := s.rateLimitService.HandleUpstreamFailoverError(ctx, account, failoverErr)
	if blocked && !failoverErr.FirstTokenTimeout {
		s.rateLimitService.resetFirstTokenTimeoutStreak(ctx, accountID, 0)
	}
	return blocked
}
