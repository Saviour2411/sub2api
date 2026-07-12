package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamErrorStreakCacheCall struct {
	accountID         int64
	source            AccountFailureStreakSource
	policyFingerprint string
	policyRevision    int64
	outcome           AccountFailureStreakOutcome
	occurredAt        time.Time
	eventID           string
}

type upstreamErrorStreakCacheStub struct {
	count              int64
	policyFingerprint  string
	policyRevision     int64
	counts             map[AccountFailureStreakSource]int64
	policyFingerprints map[AccountFailureStreakSource]string
	policyRevisions    map[AccountFailureStreakSource]int64
	lastEvents         map[AccountFailureStreakSource]AccountFailureStreakEvent
	err                error
	calls              []upstreamErrorStreakCacheCall
}

func (s *upstreamErrorStreakCacheStub) ApplyOutcome(
	_ context.Context,
	accountID int64,
	source AccountFailureStreakSource,
	policy AccountFailureStreakPolicy,
	outcome AccountFailureStreakOutcome,
	event AccountFailureStreakEvent,
) (AccountFailureStreakState, error) {
	s.calls = append(s.calls, upstreamErrorStreakCacheCall{
		accountID:         accountID,
		source:            source,
		policyFingerprint: policy.Fingerprint,
		policyRevision:    policy.Revision,
		outcome:           outcome,
		occurredAt:        event.OccurredAt,
		eventID:           event.ID,
	})
	if s.err != nil {
		return AccountFailureStreakState{}, s.err
	}
	if s.counts == nil {
		if s.policyRevision <= 0 {
			s.policyRevision = DefaultGatewayFailurePolicyRevision
		}
		s.counts = make(map[AccountFailureStreakSource]int64)
		s.policyFingerprints = make(map[AccountFailureStreakSource]string)
		s.policyRevisions = make(map[AccountFailureStreakSource]int64)
		s.lastEvents = make(map[AccountFailureStreakSource]AccountFailureStreakEvent)
		s.counts[AccountFailureStreakSourceUpstreamError] = s.count
		s.policyFingerprints[AccountFailureStreakSourceUpstreamError] = s.policyFingerprint
		s.policyRevisions[AccountFailureStreakSourceUpstreamError] = s.policyRevision
	}
	count := s.counts[source]
	if last := s.lastEvents[source]; !last.OccurredAt.IsZero() {
		if event.OccurredAt.Before(last.OccurredAt) || (event.OccurredAt.Equal(last.OccurredAt) && event.ID == last.ID) {
			return AccountFailureStreakState{Count: count, Applied: false, PolicyRevision: s.policyRevisions[source]}, nil
		}
	}
	if current := s.policyRevisions[source]; current > policy.Revision {
		return AccountFailureStreakState{Count: count, Applied: false, PolicyRevision: current}, nil
	}
	if s.policyRevisions[source] != policy.Revision || s.policyFingerprints[source] != policy.Fingerprint {
		s.policyFingerprints[source] = policy.Fingerprint
		s.policyRevisions[source] = policy.Revision
		count = 0
	}
	switch outcome {
	case AccountFailureStreakOutcomeIncrement:
		count++
	case AccountFailureStreakOutcomeReset:
		count = 0
	}
	s.counts[source] = count
	s.lastEvents[source] = event
	if source == AccountFailureStreakSourceUpstreamError {
		s.count = count
		s.policyFingerprint = policy.Fingerprint
		s.policyRevision = policy.Revision
	}
	return AccountFailureStreakState{Count: count, Applied: true, PolicyRevision: policy.Revision}, nil
}

func upstreamErrorCalls(calls []upstreamErrorStreakCacheCall) []upstreamErrorStreakCacheCall {
	filtered := make([]upstreamErrorStreakCacheCall, 0, len(calls))
	for _, call := range calls {
		if call.source == AccountFailureStreakSourceUpstreamError {
			filtered = append(filtered, call)
		}
	}
	return filtered
}

type upstreamErrorStreakRepoStub struct {
	AccountRepository
	account        *Account
	getErr         error
	persistErr     error
	persistCreated *bool
	persistCalls   int
	persistAccount int64
	persistMarker  map[string]any
	persistNextRun time.Time
}

func (r *upstreamErrorStreakRepoStub) GetByID(_ context.Context, _ int64) (*Account, error) {
	return r.account, r.getErr
}

func (r *upstreamErrorStreakRepoStub) PersistFailureSchedulingState(
	_ context.Context,
	accountID int64,
	marker map[string]any,
	nextRunAt time.Time,
) (bool, error) {
	r.persistCalls++
	r.persistAccount = accountID
	r.persistMarker = marker
	r.persistNextRun = nextRunAt
	if r.persistCreated != nil {
		return *r.persistCreated, r.persistErr
	}
	return r.persistErr == nil, r.persistErr
}

func newUpstreamErrorStreakAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{},
	}
}

func newUpstreamErrorStreakSettingService(settings GatewaySettings) *SettingService {
	settingService := &SettingService{}
	settingService.storeGatewaySettingsCache(settings, time.Hour)
	return settingService
}

func TestRecordUpstreamFailureOutcome_DefaultStatusesShareOneStreak(t *testing.T) {
	account := newUpstreamErrorStreakAccount(41)
	repo := &upstreamErrorStreakRepoStub{account: account}
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{accountRepo: repo, accountFailureStreakCache: cache}

	for _, statusCode := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{StatusCode: statusCode})
		require.True(t, managed)
		require.False(t, blocked)
	}

	require.EqualValues(t, 3, cache.count)
	calls := upstreamErrorCalls(cache.calls)
	require.Len(t, calls, 3)
	for _, call := range calls {
		require.Equal(t, account.ID, call.accountID)
		require.Equal(t, AccountFailureStreakSourceUpstreamError, call.source)
		require.Equal(t, AccountFailureStreakOutcomeIncrement, call.outcome)
		require.Equal(t, "status_codes=502,503,504;threshold=10", call.policyFingerprint)
	}
	require.Zero(t, repo.persistCalls)
	require.True(t, account.Schedulable)
}

func TestRecordUpstreamFailureOutcome_NonConfiguredStatusResetsStreak(t *testing.T) {
	account := newUpstreamErrorStreakAccount(42)
	repo := &upstreamErrorStreakRepoStub{account: account}
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{accountRepo: repo, accountFailureStreakCache: cache}

	managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{StatusCode: http.StatusBadGateway})
	require.True(t, managed)
	require.False(t, blocked)
	managed, blocked = svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{StatusCode: http.StatusInternalServerError})
	require.False(t, managed)
	require.False(t, blocked)

	require.Zero(t, cache.count)
	calls := upstreamErrorCalls(cache.calls)
	require.Len(t, calls, 2)
	require.Equal(t, AccountFailureStreakOutcomeIncrement, calls[0].outcome)
	require.Equal(t, AccountFailureStreakOutcomeReset, calls[1].outcome)
}

func TestRecordUpstreamFailureOutcome_FirstToken504IsExcluded(t *testing.T) {
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{accountFailureStreakCache: cache}

	managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), 43, &UpstreamFailoverError{
		StatusCode:        http.StatusGatewayTimeout,
		FirstTokenTimeout: true,
	})

	require.False(t, managed)
	require.False(t, blocked)
	require.Empty(t, cache.calls)
}

func TestRecordUpstreamFailureOutcome_ThresholdPersistsMarker(t *testing.T) {
	settings := DefaultGatewaySettings()
	settings.UpstreamErrorConsecutiveThreshold = 2
	fingerprint := BuildAccountFailureStreakPolicyFingerprint(AccountFailureStreakSourceUpstreamError, settings)
	account := newUpstreamErrorStreakAccount(44)
	repo := &upstreamErrorStreakRepoStub{account: account}
	cache := &upstreamErrorStreakCacheStub{count: 1, policyFingerprint: fingerprint}
	svc := &RateLimitService{
		accountRepo:               repo,
		accountFailureStreakCache: cache,
		settingService:            newUpstreamErrorStreakSettingService(settings),
	}

	managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{
		StatusCode:   http.StatusGatewayTimeout,
		ResponseBody: []byte(`{"error":{"message":"gateway unavailable"}}`),
	})

	require.True(t, managed)
	require.True(t, blocked)
	require.Equal(t, 1, repo.persistCalls)
	require.Equal(t, account.ID, repo.persistAccount)
	require.False(t, repo.persistNextRun.IsZero())
	require.False(t, account.Schedulable)
	require.Equal(t, string(AccountFailureStreakSourceUpstreamError), repo.persistMarker[accountFailureStrategyUnscheduledSourceKey])
	require.Equal(t, http.StatusGatewayTimeout, repo.persistMarker[accountFailureStrategyUnscheduledStatusCodeKey])
	require.Equal(t, int64(2), repo.persistMarker[accountFailureStrategyUnscheduledConsecutiveCountKey])
	require.Equal(t, 2, repo.persistMarker[accountFailureStrategyUnscheduledThresholdKey])
	require.Equal(t, "gateway unavailable", repo.persistMarker[accountFailureStrategyUnscheduledReasonKey])
	require.NotEmpty(t, repo.persistMarker[accountFailureStrategyUnscheduledIncidentIDKey])
}

func TestRecordUpstreamFailureOutcome_ConcurrentIncidentLoserReportsBlocked(t *testing.T) {
	settings := DefaultGatewaySettings()
	settings.UpstreamErrorConsecutiveThreshold = 1
	created := false
	account := newUpstreamErrorStreakAccount(441)
	repo := &upstreamErrorStreakRepoStub{account: account, persistCreated: &created}
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{
		accountRepo:               repo,
		accountFailureStreakCache: cache,
		settingService:            newUpstreamErrorStreakSettingService(settings),
	}

	managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{
		StatusCode: http.StatusBadGateway,
	})

	require.True(t, managed)
	require.True(t, blocked)
	require.Equal(t, 1, repo.persistCalls)
}

func TestRecordUpstreamFailureOutcome_RedisFailureFailsOpen(t *testing.T) {
	account := newUpstreamErrorStreakAccount(45)
	repo := &upstreamErrorStreakRepoStub{account: account}
	cache := &upstreamErrorStreakCacheStub{err: errors.New("redis unavailable")}
	svc := &RateLimitService{accountRepo: repo, accountFailureStreakCache: cache}

	managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{StatusCode: http.StatusBadGateway})

	require.True(t, managed)
	require.False(t, blocked)
	require.True(t, account.Schedulable)
	require.Zero(t, repo.persistCalls)
	require.Len(t, upstreamErrorCalls(cache.calls), 1)
}

func TestRecordUpstreamSuccessOutcome_ResetsStreak(t *testing.T) {
	settings := DefaultGatewaySettings()
	fingerprint := BuildAccountFailureStreakPolicyFingerprint(AccountFailureStreakSourceUpstreamError, settings)
	cache := &upstreamErrorStreakCacheStub{count: 4, policyFingerprint: fingerprint}
	svc := &RateLimitService{accountFailureStreakCache: cache}

	svc.RecordUpstreamSuccessOutcome(context.Background(), 46)

	require.Zero(t, cache.count)
	require.Len(t, cache.calls, 2)
	require.Equal(t, AccountFailureStreakSourceFirstTokenTimeout, cache.calls[0].source)
	require.Equal(t, int64(46), cache.calls[1].accountID)
	require.Equal(t, AccountFailureStreakSourceUpstreamError, cache.calls[1].source)
	require.Equal(t, AccountFailureStreakOutcomeReset, cache.calls[1].outcome)
}

func TestRecordUpstreamFailureOutcome_PolicyFingerprintChangeRestartsStreak(t *testing.T) {
	settings := DefaultGatewaySettings()
	settingService := newUpstreamErrorStreakSettingService(settings)
	account := newUpstreamErrorStreakAccount(47)
	repo := &upstreamErrorStreakRepoStub{account: account}
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{
		accountRepo:               repo,
		accountFailureStreakCache: cache,
		settingService:            settingService,
	}

	managed, blocked := svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{StatusCode: http.StatusBadGateway})
	require.True(t, managed)
	require.False(t, blocked)
	require.EqualValues(t, 1, cache.count)

	settings.UpstreamErrorConsecutiveThreshold = 4
	settings.FailurePolicyRevision++
	settingService.storeGatewaySettingsCache(settings, time.Hour)
	managed, blocked = svc.RecordUpstreamFailureOutcome(context.Background(), account.ID, &UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable})
	require.True(t, managed)
	require.False(t, blocked)

	require.EqualValues(t, 1, cache.count)
	calls := upstreamErrorCalls(cache.calls)
	require.Len(t, calls, 2)
	require.Equal(t, "status_codes=502,503,504;threshold=10", calls[0].policyFingerprint)
	require.Equal(t, "status_codes=502,503,504;threshold=4", calls[1].policyFingerprint)
}

func TestRecordUpstreamFailureOutcomeAt_OldFailureCannotOverrideNewSuccess(t *testing.T) {
	settings := DefaultGatewaySettings()
	settings.UpstreamErrorConsecutiveThreshold = 1
	account := newUpstreamErrorStreakAccount(48)
	repo := &upstreamErrorStreakRepoStub{account: account}
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{
		accountRepo:               repo,
		accountFailureStreakCache: cache,
		settingService:            newUpstreamErrorStreakSettingService(settings),
	}
	base := time.Unix(1_700_000_000, 0).UTC()
	svc.RecordUpstreamSuccessOutcomeAt(context.Background(), account.ID, AccountFailureStreakEvent{
		OccurredAt: base.Add(time.Second),
		ID:         "success-new",
	})

	managed, blocked := svc.RecordUpstreamFailureOutcomeAt(
		context.Background(),
		account.ID,
		&UpstreamFailoverError{StatusCode: http.StatusBadGateway},
		AccountFailureStreakEvent{OccurredAt: base, ID: "failure-old"},
	)
	require.True(t, managed)
	require.False(t, blocked)
	require.Zero(t, cache.count)
	require.Zero(t, repo.persistCalls)
}

func TestRecordUpstreamFailureOutcomeSnapshot_IgnoresChangedPolicy(t *testing.T) {
	current := DefaultGatewaySettings()
	current.UpstreamErrorConsecutiveThreshold = 2
	current.FailurePolicyRevision = 2
	account := newUpstreamErrorStreakAccount(49)
	cache := &upstreamErrorStreakCacheStub{}
	svc := &RateLimitService{
		accountRepo:               &upstreamErrorStreakRepoStub{account: account},
		accountFailureStreakCache: cache,
		settingService:            newUpstreamErrorStreakSettingService(current),
	}
	old := current
	old.UpstreamErrorConsecutiveThreshold = 10
	old.FailurePolicyRevision = 1

	managed, blocked := svc.RecordUpstreamFailureOutcomeSnapshot(
		context.Background(),
		account.ID,
		&UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable},
		AccountFailureOutcomeSnapshot{
			Event:    NewAccountFailureStreakEvent(time.Now().Add(-time.Second)),
			Settings: old,
		},
	)

	require.True(t, managed)
	require.False(t, blocked)
	require.Empty(t, cache.calls)
	require.True(t, account.Schedulable)
}
