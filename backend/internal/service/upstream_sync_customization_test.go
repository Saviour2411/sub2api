//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type syncDeadlineBillingRepo struct {
	UsageBillingBatchRepository
	deadline time.Time
}

func (r *syncDeadlineBillingRepo) Apply(ctx context.Context, _ *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	r.deadline, _ = ctx.Deadline()
	return nil, ctx.Err()
}

func TestUsageBillingProductionProviderKeepsExpiredWorkerDeadline(t *testing.T) {
	repo := &syncDeadlineBillingRepo{}
	provider := ProvideUsageBillingRepository(repo)
	deadline := time.Now().Add(-time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	_, err := provider.Apply(ctx, &UsageBillingCommand{UserID: 1, RequestID: "local-deadline-test"})
	require.ErrorIs(t, err, context.DeadlineExceeded, "生产注入不能让已过期任务重新获得结算时间")
	require.Equal(t, deadline, repo.deadline)
	require.Same(t, repo, provider)
}

func TestSchedulerFallbackIsolatesConcurrentMutableFields(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
		cache := &fallbackCoalesceCache{}
		repo := &fallbackCoalesceRepo{
			release: make(chan struct{}),
			accounts: []Account{{
				ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"public": "upstream"}},
				Extra:       map[string]any{"nested": []any{map[string]any{"value": "original"}}},
				GroupIDs:    []int64{71}, AccountGroups: []AccountGroup{{AccountID: 1, GroupID: 71, Priority: 1}},
				ExpiresAt: &expires,
			}},
		}
		svc := newFallbackCoalesceService(cache, repo)
		groupID := int64(71)
		const callers = 8
		results := make([][]Account, callers)
		errs := make([]error, callers)
		var workers sync.WaitGroup
		for i := range callers {
			workers.Go(func() {
				results[i], _, errs[i] = svc.ListSchedulableAccounts(context.Background(), &groupID, PlatformOpenAI, false)
			})
		}
		synctest.Wait()
		require.Equal(t, int32(1), repo.calls.Load())
		close(repo.release)
		workers.Wait()
		for i := range callers {
			require.NoError(t, errs[i])
			require.Len(t, results[i], 1)
			workers.Go(func() {
				account := &results[i][0]
				account.Credentials["model_mapping"].(map[string]any)["public"] = i
				account.Extra["nested"].([]any)[0].(map[string]any)["value"] = i
				account.GroupIDs[0] = int64(i)
				account.AccountGroups[0].Priority = i
				*account.ExpiresAt = expires.Add(time.Duration(i+1) * time.Hour)
			})
		}
		workers.Wait()
		for i := range callers {
			require.Equal(t, i, results[i][0].Credentials["model_mapping"].(map[string]any)["public"])
			require.Equal(t, i, results[i][0].Extra["nested"].([]any)[0].(map[string]any)["value"])
			require.Equal(t, int64(i), results[i][0].GroupIDs[0])
			require.Equal(t, i, results[i][0].AccountGroups[0].Priority)
			require.Equal(t, expires.Add(time.Duration(i+1)*time.Hour), *results[i][0].ExpiresAt)
		}
		require.Equal(t, "upstream", repo.accounts[0].Credentials["model_mapping"].(map[string]any)["public"])
		require.Equal(t, "original", cache.snapshot[0].Extra["nested"].([]any)[0].(map[string]any)["value"])
		require.Equal(t, []int64{71}, cache.snapshot[0].GroupIDs)
		require.Equal(t, 1, cache.snapshot[0].AccountGroups[0].Priority)
		require.Equal(t, expires, *cache.snapshot[0].ExpiresAt)
	})
}

func TestSchedulerSnapshotPointerReaderIsolatesMutableFields(t *testing.T) {
	original := &Account{
		ID: 1, Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"public": "upstream"}},
		GroupIDs:    []int64{71},
	}
	first := derefSchedulableAccounts([]*Account{nil, original})
	second := derefSchedulableAccounts([]*Account{original})
	require.Len(t, first, 1)
	first[0].Credentials["model_mapping"].(map[string]any)["public"] = "changed"
	first[0].GroupIDs[0] = 99
	require.Equal(t, "upstream", second[0].Credentials["model_mapping"].(map[string]any)["public"])
	require.Equal(t, []int64{71}, original.GroupIDs)
}
