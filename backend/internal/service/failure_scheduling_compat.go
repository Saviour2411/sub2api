package service

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	AccountFailureSchedulingStrategyDefault              = "default"
	AccountFailureSchedulingStrategyDisableUntilTestPass = "disable_until_test_pass"
	accountFailureSchedulingStrategyKey                  = "failure_scheduling_strategy"
	accountFailureStrategyUnscheduledKey                 = "failure_strategy_unscheduled"
	accountFailureStrategyUnscheduledAtKey               = "at"
	accountFailureStrategyUnscheduledStatusCodeKey       = "status_code"
	accountFailureStrategyUnscheduledReasonKey           = "reason"
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
	fresh, err := s.accountRepo.GetByID(readCtx, account.ID)
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
