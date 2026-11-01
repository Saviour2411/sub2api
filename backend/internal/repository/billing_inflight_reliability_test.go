package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

type billingAuditLostReplyCache struct {
	*billingCache
	loseNext atomic.Bool
}

func (c *billingAuditLostReplyCache) ReserveInflightBalance(ctx context.Context, userID int64, id string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	allowed, sum, err := c.billingCache.ReserveInflightBalance(ctx, userID, id, amount, balance, ttl)
	if err == nil && allowed && c.loseNext.Swap(false) {
		return false, 0, context.DeadlineExceeded
	}
	return allowed, sum, err
}

type billingAuditReleaseFailureCache struct {
	*billingCache
	releaseCalls atomic.Int32
}

type lostScopeReplyCache struct{ *billingCache }

func (c *lostScopeReplyCache) ReserveInflightScope(ctx context.Context, scope, id string, amount float64, ttl time.Duration) (float64, error) {
	_, err := c.billingCache.ReserveInflightScope(ctx, scope, id, amount, ttl)
	if err != nil {
		return 0, err
	}
	return 0, context.DeadlineExceeded
}

func TestInflightReliability_SubscriptionAndAPIKeyUnknownResultsAreReleased(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	svc := billingAuditService(t, &lostScopeReplyCache{cache})
	limit := 10.0
	group := &service.Group{ID: 2, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: &limit}
	sub := &service.UserSubscription{ID: 3}
	user := &service.User{ID: 1}
	key := &service.APIKey{ID: 4, Quota: 10}
	require.NoError(t, cache.SetSubscriptionCache(ctx, 1, 2, &service.SubscriptionCacheData{Status: "active", ExpiresAt: time.Now().Add(time.Hour)}))
	res, err := svc.ReserveFixedCharge(ctx, user, key, group, sub, 1)
	require.NoError(t, err)
	require.NotNil(t, res)
	done := res.Acquire()
	res.HandlerDone()
	for _, scope := range []string{"sub:3", "apikey:4"} {
		z, _ := billingInflightScopeKeys(scope)
		require.Equal(t, int64(1), cache.rdb.ZCard(ctx, z).Val())
	}
	done()
	for _, scope := range []string{"sub:3", "apikey:4"} {
		z, _ := billingInflightScopeKeys(scope)
		require.Zero(t, cache.rdb.ZCard(ctx, z).Val())
	}
}

func (c *billingAuditReleaseFailureCache) ReleaseInflightBalance(ctx context.Context, userID int64, id string) error {
	if c.releaseCalls.Add(1) == 1 {
		return context.DeadlineExceeded
	}
	return c.billingCache.ReleaseInflightBalance(ctx, userID, id)
}

func billingAuditService(t *testing.T, cache service.BillingCache) *service.BillingCacheService {
	t.Helper()
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60}
	svc := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	return svc
}

func TestInflightReliability_LostReplyKeepsCleanupHandle(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 1054}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1))
	lost := &billingAuditLostReplyCache{billingCache: cache}
	lost.loseNext.Store(true)
	svc := billingAuditService(t, lost)
	reservation, err := svc.ReserveInflight(ctx, user, nil, nil, 0.6)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	taskDone := reservation.Acquire()
	reservation.HandlerDone()
	require.Equal(t, int64(1), inflightCount(t, cache, user.ID))
	taskDone()
	require.Zero(t, inflightCount(t, cache, user.ID))
	next, err := svc.ReserveInflight(ctx, user, nil, nil, 0.6)
	require.NoError(t, err)
	next.HandlerDone()
	balance, err := cache.GetUserBalance(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 1.0, balance)
}

func TestInflightReliability_ReleaseFailureIsRetried(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	user := &service.User{ID: 1054}
	require.NoError(t, cache.SetUserBalance(ctx, user.ID, 1))
	failing := &billingAuditReleaseFailureCache{billingCache: cache}
	svc := billingAuditService(t, failing)
	reservation, err := svc.ReserveInflight(ctx, user, nil, nil, 0.6)
	require.NoError(t, err)
	require.NotNil(t, reservation)
	reservation.HandlerDone()
	require.Eventually(t, func() bool { return inflightCount(t, cache, user.ID) == 0 }, time.Second, 10*time.Millisecond)
	reservation.Release()
	require.GreaterOrEqual(t, failing.releaseCalls.Load(), int32(2))
}

func TestInflightReliability_RetryDoesNotRejectItself(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	ctx := context.Background()
	const userID int64 = 1054
	allowed, _, err := cache.ReserveInflightBalance(ctx, userID, "same-request", 0.6, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, sum, err := cache.ReserveInflightBalance(ctx, userID, "same-request", 0.6, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Zero(t, sum)
	require.Equal(t, int64(1), inflightCount(t, cache, userID))
}
