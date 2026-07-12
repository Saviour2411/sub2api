package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newAccountFailureStreakCacheForTest(t *testing.T) *accountFailureStreakCache {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })
	return &accountFailureStreakCache{rdb: rdb}
}

func failurePolicy(revision int64, fingerprint string) service.AccountFailureStreakPolicy {
	return service.AccountFailureStreakPolicy{Revision: revision, Fingerprint: fingerprint}
}

func failureEvent(at time.Time, id string) service.AccountFailureStreakEvent {
	return service.AccountFailureStreakEvent{OccurredAt: at, ID: id}
}

func TestAccountFailureStreakCache_ApplyOutcome(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0).UTC()
	source := service.AccountFailureStreakSourceFirstTokenTimeout
	policyV1 := failurePolicy(1, "policy-a")

	state, err := cache.ApplyOutcome(ctx, 42, source, policyV1, service.AccountFailureStreakOutcomeIncrement, failureEvent(base, "event-1"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 1, Applied: true, PolicyRevision: 1}, state)

	state, err = cache.ApplyOutcome(ctx, 42, source, policyV1, service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(time.Second), "event-2"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 2, Applied: true, PolicyRevision: 1}, state)

	policyV2 := failurePolicy(2, "policy-b")
	state, err = cache.ApplyOutcome(ctx, 42, source, policyV2, service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(2*time.Second), "event-3"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 1, Applied: true, PolicyRevision: 2}, state)

	state, err = cache.ApplyOutcome(ctx, 42, source, policyV2, service.AccountFailureStreakOutcomeReset, failureEvent(base.Add(3*time.Second), "event-4"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 0, Applied: true, PolicyRevision: 2}, state)
}

func TestAccountFailureStreakCache_SameTimestampEventsAndEventIDIdempotency(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	at := time.Unix(1_700_000_000, 123_456_000).UTC()
	policy := failurePolicy(1, "policy")
	source := service.AccountFailureStreakSourceUpstreamError

	first, err := cache.ApplyOutcome(ctx, 7, source, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(at, "event-a"))
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Count)
	second, err := cache.ApplyOutcome(ctx, 7, source, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(at, "event-b"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 2, Applied: true, PolicyRevision: 1}, second)

	duplicate, err := cache.ApplyOutcome(ctx, 7, source, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(at.Add(time.Second), "event-a"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 2, Applied: false, PolicyRevision: 1}, duplicate)
}

func TestAccountFailureStreakCache_RejectsOldEventAndOldPolicyRevision(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0).UTC()
	source := service.AccountFailureStreakSourceUpstreamError

	state, err := cache.ApplyOutcome(ctx, 7, source, failurePolicy(2, "policy-new"), service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(2*time.Second), "new"))
	require.NoError(t, err)
	require.EqualValues(t, 1, state.Count)

	state, err = cache.ApplyOutcome(ctx, 7, source, failurePolicy(2, "policy-new"), service.AccountFailureStreakOutcomeReset, failureEvent(base, "old-event"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 1, Applied: false, PolicyRevision: 2}, state)

	state, err = cache.ApplyOutcome(ctx, 7, source, failurePolicy(1, "policy-old"), service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(10*time.Second), "old-policy"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 1, Applied: false, PolicyRevision: 2}, state)

	_, err = cache.ApplyOutcome(ctx, 7, source, failurePolicy(2, "mismatched-fingerprint"), service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(11*time.Second), "bad-fingerprint"))
	require.Error(t, err)
}

func TestAccountFailureStreakCache_ResetFailureIsCompensatedBeforeIncrement(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0).UTC()
	policy := failurePolicy(1, "policy")
	source := service.AccountFailureStreakSourceUpstreamError

	_, err := cache.ApplyOutcome(ctx, 8, source, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(base, "failure-old"))
	require.NoError(t, err)
	workingClient := cache.rdb
	badClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 10 * time.Millisecond, MaxRetries: -1})
	cache.rdb = badClient
	_, err = cache.ApplyOutcome(ctx, 8, source, policy, service.AccountFailureStreakOutcomeReset, failureEvent(base.Add(time.Second), "reset-failed"))
	require.Error(t, err)
	cache.rdb = workingClient
	require.NoError(t, badClient.Close())

	state, err := cache.ApplyOutcome(ctx, 8, source, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(2*time.Second), "failure-new"))
	require.NoError(t, err)
	require.Equal(t, service.AccountFailureStreakState{Count: 1, Applied: true, PolicyRevision: 1}, state)
}

func TestAccountFailureStreakCache_StalePendingResetSuppressesThreshold(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0).UTC()
	policy := failurePolicy(1, "policy")
	source := service.AccountFailureStreakSourceUpstreamError
	key := fmt.Sprintf("%s%s:account:%d", accountFailureStreakPrefix, source, 9)

	_, err := cache.applyOutcome(ctx, key, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(2*time.Second), "already-newer"))
	require.NoError(t, err)
	cache.pendingResets.Store(key, accountFailurePendingReset{
		policy: policy,
		event:  failureEvent(base.Add(time.Second), "stale-reset"),
	})

	_, err = cache.ApplyOutcome(ctx, 9, source, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(base.Add(3*time.Second), "must-not-apply"))
	require.ErrorContains(t, err, "拒绝使用旧计数")
}

func TestAccountFailureStreakCache_IsolatesSources(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	occurredAt := time.Unix(1_700_000_000, 0).UTC()
	policy := failurePolicy(1, "policy")

	firstToken, err := cache.ApplyOutcome(ctx, 9, service.AccountFailureStreakSourceFirstTokenTimeout, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(occurredAt, "first-token"))
	require.NoError(t, err)
	upstream, err := cache.ApplyOutcome(ctx, 9, service.AccountFailureStreakSourceUpstreamError, policy, service.AccountFailureStreakOutcomeIncrement, failureEvent(occurredAt, "upstream"))
	require.NoError(t, err)
	require.EqualValues(t, 1, firstToken.Count)
	require.EqualValues(t, 1, upstream.Count)
}

func TestAccountFailureStreakCache_RejectsInvalidInput(t *testing.T) {
	cache := newAccountFailureStreakCacheForTest(t)
	ctx := context.Background()
	now := time.Now()
	policy := failurePolicy(1, "policy")
	event := failureEvent(now, "event")

	_, err := cache.ApplyOutcome(ctx, 0, service.AccountFailureStreakSourceFirstTokenTimeout, policy, service.AccountFailureStreakOutcomeIncrement, event)
	require.Error(t, err)
	_, err = cache.ApplyOutcome(ctx, 1, "unknown", policy, service.AccountFailureStreakOutcomeIncrement, event)
	require.Error(t, err)
	_, err = cache.ApplyOutcome(ctx, 1, service.AccountFailureStreakSourceFirstTokenTimeout, policy, "unknown", event)
	require.Error(t, err)
	_, err = cache.ApplyOutcome(ctx, 1, service.AccountFailureStreakSourceFirstTokenTimeout, service.AccountFailureStreakPolicy{}, service.AccountFailureStreakOutcomeIncrement, event)
	require.Error(t, err)
	_, err = cache.ApplyOutcome(ctx, 1, service.AccountFailureStreakSourceFirstTokenTimeout, policy, service.AccountFailureStreakOutcomeIncrement, service.AccountFailureStreakEvent{})
	require.Error(t, err)
}
