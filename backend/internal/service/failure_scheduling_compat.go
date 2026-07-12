package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

type firstTokenTimeoutStatePersister interface {
	PersistFirstTokenTimeoutState(ctx context.Context, accountID int64, marker map[string]any, nextRunAt time.Time) error
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

// HandleFirstTokenTimeout 将账号持久停调度，直到 auto_managed 测活成功后恢复。
// 该策略不依赖账号原有 failure_scheduling_strategy，是首 Token 超时的强制保护。
func (s *RateLimitService) HandleFirstTokenTimeout(ctx context.Context, account *Account, model string, timeoutSeconds int) error {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 {
		return nil
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultFirstTokenTimeoutSeconds
	}
	now := time.Now().UTC()
	model = strings.TrimSpace(model)
	reason := fmt.Sprintf(
		"source=first_token_timeout model=%s timeout_seconds=%d status_code=%d at=%s",
		model,
		timeoutSeconds,
		http.StatusGatewayTimeout,
		now.Format(time.RFC3339),
	)
	marker := BuildFailureStrategyUnscheduledMarker(http.StatusGatewayTimeout, reason, now)
	marker[accountFailureStrategyUnscheduledSourceKey] = "first_token_timeout"
	marker[accountFailureStrategyUnscheduledModelKey] = model
	marker[accountFailureStrategyUnscheduledTimeoutSecondsKey] = timeoutSeconds

	if ctx == nil {
		ctx = context.Background()
	}

	var persistErr error
	if persister, ok := s.accountRepo.(firstTokenTimeoutStatePersister); ok {
		opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		persistErr = persister.PersistFirstTokenTimeoutState(opCtx, account.ID, marker, now)
		cancel()
	} else {
		persistErr = s.persistFirstTokenTimeoutStateCompat(ctx, account.ID, marker)
	}
	if persistErr != nil {
		logger.FromContext(ctx).With(
			zap.String("component", "service.first_token_timeout"),
			zap.Int64("account_id", account.ID),
			zap.Error(persistErr),
		).Error("首 Token 超时状态原子持久化失败，账号保持原调度状态")
		return persistErr
	}

	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	account.Extra[accountFailureStrategyUnscheduledKey] = marker
	account.Schedulable = false
	// OpenAI 高速调度缓存需要同步阻断；测活成功路径会调用 ClearAccountSchedulingBlock。
	s.notifyAccountSchedulingBlocked(account, now.AddDate(100, 0, 0), "first_token_timeout")
	return nil
}

// persistFirstTokenTimeoutStateCompat 只用于不具备事务扩展的测试桩。
// 任一步失败都会短路，并尽力撤销此前写入，生产仓储不会进入此路径。
func (s *RateLimitService) persistFirstTokenTimeoutStateCompat(ctx context.Context, accountID int64, marker map[string]any) error {
	if s.autoManagedProbe == nil {
		return errors.New("auto managed probe scheduler is not configured")
	}
	run := func(fn func(context.Context) error) error {
		opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return fn(opCtx)
	}
	if err := run(func(opCtx context.Context) error {
		return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: marker})
	}); err != nil {
		return fmt.Errorf("写入不可调度标记: %w", err)
	}
	if err := run(func(opCtx context.Context) error {
		return s.accountRepo.SetSchedulable(opCtx, accountID, false)
	}); err != nil {
		rollbackErr := run(func(opCtx context.Context) error {
			return s.accountRepo.UpdateExtra(opCtx, accountID, map[string]any{accountFailureStrategyUnscheduledKey: nil})
		})
		return errors.Join(fmt.Errorf("设置账号不可调度: %w", err), rollbackErr)
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
		return errors.Join(fmt.Errorf("补建自动测活计划: %w", err), restoreErr, clearMarkerErr)
	}
	return nil
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
	if account.ShouldDisableSchedulingOnUpstreamError() {
		return true
	}
	if s == nil || s.accountRepo == nil || account.ID <= 0 {
		return false
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
		return false
	}
	if fresh.ShouldDisableSchedulingOnUpstreamError() {
		if account.Extra == nil {
			account.Extra = map[string]any{}
		}
		account.Extra[accountFailureSchedulingStrategyKey] = AccountFailureSchedulingStrategyDisableUntilTestPass
		return true
	}
	return false
}

func (s *RateLimitService) HandleStrictFailureScheduling(ctx context.Context, account *Account, statusCode int, reason string) bool {
	if s == nil || account == nil || !s.ShouldDisableSchedulingOnUpstreamError(ctx, account) {
		return false
	}
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	marker := BuildFailureStrategyUnscheduledMarker(statusCode, reason, time.Now())
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	account.Extra[accountFailureStrategyUnscheduledKey] = marker
	account.Schedulable = false
	opCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.accountRepo.SetSchedulable(opCtx, account.ID, false)
	_ = s.accountRepo.UpdateExtra(opCtx, account.ID, map[string]any{accountFailureStrategyUnscheduledKey: marker})
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
