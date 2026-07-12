//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountFailureStreakCallForAdminReset struct {
	accountID  int64
	source     AccountFailureStreakSource
	policy     AccountFailureStreakPolicy
	outcome    AccountFailureStreakOutcome
	occurredAt time.Time
	eventID    string
}

type accountFailureStreakCacheStubForAdminReset struct {
	calls       []accountFailureStreakCallForAdminReset
	errorSource AccountFailureStreakSource
}

func (s *accountFailureStreakCacheStubForAdminReset) ApplyOutcome(
	_ context.Context,
	accountID int64,
	source AccountFailureStreakSource,
	policy AccountFailureStreakPolicy,
	outcome AccountFailureStreakOutcome,
	event AccountFailureStreakEvent,
) (AccountFailureStreakState, error) {
	s.calls = append(s.calls, accountFailureStreakCallForAdminReset{
		accountID:  accountID,
		source:     source,
		policy:     policy,
		outcome:    outcome,
		occurredAt: event.OccurredAt,
		eventID:    event.ID,
	})
	if source == s.errorSource {
		return AccountFailureStreakState{}, errors.New("redis unavailable")
	}
	return AccountFailureStreakState{Applied: true}, nil
}

type accountRepoStubForClearAccountError struct {
	mockAccountRepoForGemini
	account                  *Account
	clearErrorCalls          int
	clearRateLimitCalls      int
	clearAntigravityCalls    int
	clearModelRateLimitCalls int
	clearTempUnschedCalls    int
	clearFailureStateCalls   int
	clearFailureIncidentIDs  []string
	accountAfterFailureClear *Account
}

func (r *accountRepoStubForClearAccountError) GetByID(ctx context.Context, id int64) (*Account, error) {
	return r.account, nil
}

func (r *accountRepoStubForClearAccountError) ClearError(ctx context.Context, id int64) error {
	r.clearErrorCalls++
	r.account.Status = StatusActive
	r.account.ErrorMessage = ""
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearRateLimit(ctx context.Context, id int64) error {
	r.clearRateLimitCalls++
	r.account.RateLimitedAt = nil
	r.account.RateLimitResetAt = nil
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	r.clearAntigravityCalls++
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearModelRateLimits(ctx context.Context, id int64) error {
	r.clearModelRateLimitCalls++
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearTempUnschedulable(ctx context.Context, id int64) error {
	r.clearTempUnschedCalls++
	r.account.TempUnschedulableUntil = nil
	r.account.TempUnschedulableReason = ""
	return nil
}

func (r *accountRepoStubForClearAccountError) ClearFailureSchedulingState(_ context.Context, _ int64, incidentID string) (bool, error) {
	r.clearFailureStateCalls++
	r.clearFailureIncidentIDs = append(r.clearFailureIncidentIDs, incidentID)
	if r.account == nil || !r.account.HasFailureStrategyUnscheduledMarker() {
		return false, nil
	}
	currentIncidentID, _ := r.account.FailureStrategyUnscheduledIncident()
	if currentIncidentID != incidentID {
		return false, nil
	}
	delete(r.account.Extra, accountFailureStrategyUnscheduledKey)
	r.account.Schedulable = true
	if r.accountAfterFailureClear != nil {
		r.account = r.accountAfterFailureClear
	}
	return true, nil
}

func TestAdminService_ClearAccountError_AlsoClearsRecoverableRuntimeState(t *testing.T) {
	until := time.Now().Add(10 * time.Minute)
	resetAt := time.Now().Add(5 * time.Minute)
	repo := &accountRepoStubForClearAccountError{
		account: &Account{
			ID:                      31,
			Platform:                PlatformOpenAI,
			Type:                    AccountTypeOAuth,
			Status:                  StatusError,
			ErrorMessage:            "refresh failed",
			RateLimitResetAt:        &resetAt,
			TempUnschedulableUntil:  &until,
			TempUnschedulableReason: "missing refresh token",
		},
	}
	blocker := &runtimeBlockRecorder{}
	streakCache := &accountFailureStreakCacheStubForAdminReset{}
	svc := &adminServiceImpl{
		accountRepo:               repo,
		runtimeBlocker:            blocker,
		accountFailureStreakCache: streakCache,
	}

	updated, err := svc.ClearAccountError(context.Background(), 31)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, repo.clearErrorCalls)
	require.Equal(t, 1, repo.clearRateLimitCalls)
	require.Equal(t, 1, repo.clearAntigravityCalls)
	require.Equal(t, 1, repo.clearModelRateLimitCalls)
	require.Equal(t, 1, repo.clearTempUnschedCalls)
	require.Nil(t, updated.RateLimitResetAt)
	require.Nil(t, updated.TempUnschedulableUntil)
	require.Empty(t, updated.TempUnschedulableReason)
	require.Equal(t, []int64{31}, blocker.clearedIDs)
	require.Len(t, streakCache.calls, 2)
	require.Equal(t, AccountFailureStreakSourceFirstTokenTimeout, streakCache.calls[0].source)
	require.Equal(t, AccountFailureStreakSourceUpstreamError, streakCache.calls[1].source)
	require.Equal(t, AccountFailureStreakOutcomeReset, streakCache.calls[0].outcome)
	require.Equal(t, AccountFailureStreakOutcomeReset, streakCache.calls[1].outcome)
	require.Equal(t, int64(31), streakCache.calls[0].accountID)
	require.Equal(t, streakCache.calls[0].occurredAt, streakCache.calls[1].occurredAt)
	require.Equal(t, "timeout_seconds=60;threshold=3", streakCache.calls[0].policy.Fingerprint)
	require.Equal(t, "status_codes=502,503,504;threshold=10", streakCache.calls[1].policy.Fingerprint)
	require.NotEmpty(t, streakCache.calls[0].eventID)
	require.NotEmpty(t, streakCache.calls[1].eventID)
}

func TestAdminService_ClearAccountError_ReturnsStreakResetError(t *testing.T) {
	repo := &accountRepoStubForClearAccountError{
		account: &Account{ID: 32, Status: StatusError, ErrorMessage: "upstream failed"},
	}
	streakCache := &accountFailureStreakCacheStubForAdminReset{
		errorSource: AccountFailureStreakSourceUpstreamError,
	}
	svc := &adminServiceImpl{
		accountRepo:               repo,
		accountFailureStreakCache: streakCache,
	}

	updated, err := svc.ClearAccountError(context.Background(), 32)
	require.Nil(t, updated)
	require.ErrorContains(t, err, "upstream_error")
	require.Len(t, streakCache.calls, 2)
}

func TestAdminService_ClearAccountError_ClearsFailureIncidentWithoutUnpausingManualAccount(t *testing.T) {
	t.Run("事故停调度恢复并清除 marker", func(t *testing.T) {
		repo := &accountRepoStubForClearAccountError{account: &Account{
			ID:          33,
			Status:      StatusActive,
			Schedulable: false,
			Extra: map[string]any{
				accountFailureStrategyUnscheduledKey: map[string]any{
					accountFailureStrategyUnscheduledSourceKey:     "upstream_error",
					accountFailureStrategyUnscheduledIncidentIDKey: "incident-33",
				},
			},
		}}
		svc := &adminServiceImpl{accountRepo: repo}

		updated, err := svc.ClearAccountError(context.Background(), 33)

		require.NoError(t, err)
		require.True(t, updated.Schedulable)
		require.False(t, updated.HasFailureStrategyUnscheduledMarker())
		require.Equal(t, 1, repo.clearFailureStateCalls)
		require.Equal(t, []string{"incident-33"}, repo.clearFailureIncidentIDs)
	})

	t.Run("无事故 marker 时保留手动暂停", func(t *testing.T) {
		repo := &accountRepoStubForClearAccountError{account: &Account{
			ID:          34,
			Status:      StatusActive,
			Schedulable: false,
			Extra:       map[string]any{},
		}}
		svc := &adminServiceImpl{accountRepo: repo}

		updated, err := svc.ClearAccountError(context.Background(), 34)

		require.NoError(t, err)
		require.False(t, updated.Schedulable)
		require.Zero(t, repo.clearFailureStateCalls)
	})
}

func TestAdminService_ClearAccountError_PreservesConcurrentIncident(t *testing.T) {
	newIncident := &Account{
		ID:          35,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: false,
		Extra: map[string]any{
			accountFailureStrategyUnscheduledKey: map[string]any{
				accountFailureStrategyUnscheduledSourceKey:     "upstream_error",
				accountFailureStrategyUnscheduledIncidentIDKey: "incident-new",
			},
		},
	}
	repo := &accountRepoStubForClearAccountError{
		account: &Account{
			ID:          35,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: false,
			Extra: map[string]any{
				accountFailureStrategyUnscheduledKey: map[string]any{
					accountFailureStrategyUnscheduledSourceKey:     "first_token_timeout",
					accountFailureStrategyUnscheduledIncidentIDKey: "incident-old",
				},
			},
		},
		accountAfterFailureClear: newIncident,
	}
	blocker := &runtimeBlockRecorder{}
	streak := &accountFailureStreakCacheStubForAdminReset{}
	svc := &adminServiceImpl{
		accountRepo:               repo,
		runtimeBlocker:            blocker,
		accountFailureStreakCache: streak,
	}

	updated, err := svc.ClearAccountError(context.Background(), 35)

	require.NoError(t, err)
	require.Equal(t, "incident-new", func() string {
		incidentID, _ := updated.FailureStrategyUnscheduledIncident()
		return incidentID
	}())
	require.Equal(t, []string{"incident-old"}, repo.clearFailureIncidentIDs)
	require.Equal(t, []int64{35}, blocker.clearedIDs)
	require.Equal(t, []*Account{newIncident}, blocker.accounts)
	require.Equal(t, []string{"upstream_error"}, blocker.reasons)
	require.Len(t, streak.calls, 2)
}
