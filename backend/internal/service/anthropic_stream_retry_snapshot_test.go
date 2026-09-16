//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type streamRetrySnapshotCache struct {
	snapshotHydrationCache
	reads    map[int64]int
	failedID int64
}

func (cache *streamRetrySnapshotCache) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	cache.reads[accountID]++
	if accountID == cache.failedID {
		return nil, errors.New("完整账号读取失败")
	}
	return cache.snapshotHydrationCache.GetAccount(ctx, accountID)
}

func TestAnthropicStreamSafeRetrySnapshotEligibility(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, scenario := range []struct {
			name               string
			disablePassthrough bool
			disableSame        bool
			unschedulable      bool
			readFailure        bool
			wantID             int64
		}{
			{name: "原号优先", wantID: 740},
			{name: "透传已关闭", disablePassthrough: true, wantID: 741},
			{name: "同号次数为零", disableSame: true, wantID: 741},
			{name: "原号已不可调度", unschedulable: true, wantID: 741},
			{name: "旧缓存补全失败", readFailure: true, wantID: 741},
		} {
			t.Run(scenario.name+map[bool]string{true: "/旧缓存", false: "/新缓存"}[legacy], func(t *testing.T) {
				fixture := newLoadAwareRestrictionFixture(t, false, nil, nil)
				fixture.svc.channelService = nil
				original := safeRetryAccount(740)
				original.Priority = 0
				alternative := safeRetryAccount(741)
				alternative.Priority = 1
				retry := newAnthropicStreamRetryState(fixture.ctx, DefaultGatewaySettings())
				defer retry.Close()
				require.NoError(t, retry.beforeDispatch(original))
				retry.startResponse()
				require.NoError(t, retry.PrepareRetry(original, &AnthropicStreamFailure{Kind: "missing_terminal", Replayable: true}))
				if scenario.disablePassthrough {
					original.Extra["anthropic_passthrough"] = false
				}
				if scenario.disableSame {
					original.Credentials["pool_mode_retry_count"] = 0
				}
				original.Schedulable = !scenario.unschedulable
				cache := &streamRetrySnapshotCache{
					snapshotHydrationCache: snapshotHydrationCache{accounts: map[int64]*Account{740: original, 741: alternative}},
					reads:                  make(map[int64]int),
				}
				for _, account := range []*Account{original, alternative} {
					projection := *account
					if legacy || scenario.readFailure {
						projection.Credentials = nil
						projection.Extra = nil
					}
					cache.snapshot = append(cache.snapshot, &projection)
				}
				if scenario.readFailure {
					cache.failedID = original.ID
				}
				repository := &mockAccountRepoForPlatform{}
				fixture.svc.schedulerSnapshot = NewSchedulerSnapshotService(cache, nil, repository, nil, nil)
				selection, err := fixture.svc.SelectAnthropicStreamRetryAccount(fixture.ctx, retry, &fixture.groupID, "", "claude-sonnet-4-6", nil, "", 0, true)
				require.NoError(t, err)
				require.Equal(t, scenario.wantID, selection.Account.ID)
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				if scenario.readFailure {
					require.Equal(t, 1, cache.reads[original.ID])
				}
			})
		}
	}
}
