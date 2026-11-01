package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func setInflightMode(t *testing.T, c *billingCache, mode, generation string) {
	t.Helper()
	require.NoError(t, c.rdb.HSet(context.Background(), billingInflightAccountingKey, "mode", mode, "generation", generation).Err())
}

func TestInflightAccountingIdempotencyAndCompletion(t *testing.T) {
	for _, mode := range []string{"compat", "aggregate"} {
		t.Run(mode, func(t *testing.T) {
			_, cache, _ := newInflightTestEnv(t, true, 60)
			setInflightMode(t, cache, mode, "a")
			ctx := context.Background()
			allowed, _, err := cache.ReserveInflightBalance(ctx, 1, "one", 0.6, 1, time.Minute)
			require.NoError(t, err)
			require.True(t, allowed)
			allowed, prior, err := cache.ReserveInflightBalance(ctx, 1, "one", 10, 0, time.Minute)
			require.NoError(t, err)
			require.True(t, allowed)
			require.Zero(t, prior)
			for _, scope := range []string{"sub:1", "apikey:1"} {
				first, err := cache.ReserveInflightScope(ctx, scope, "one", 0.6, time.Minute)
				require.NoError(t, err)
				again, err := cache.ReserveInflightScope(ctx, scope, "one", 10, time.Minute)
				require.NoError(t, err)
				require.Equal(t, first, again)
				z, h := billingInflightScopeKeys(scope)
				require.Equal(t, "0.6", cache.rdb.HGet(ctx, h, "one").Val())
				require.NoError(t, cache.ReleaseInflightScope(ctx, scope, "one"))
				require.NoError(t, cache.ReleaseInflightScope(ctx, scope, "one"))
				_, err = cache.ReserveInflightScope(ctx, scope, "one", 0.6, time.Minute)
				require.NoError(t, err)
				require.Zero(t, cache.rdb.ZCard(ctx, z).Val())
			}
			require.NoError(t, cache.ReleaseInflightBalance(ctx, 1, "one"))
			require.NoError(t, cache.ReleaseInflightBalance(ctx, 1, "one"))
			allowed, _, err = cache.ReserveInflightBalance(ctx, 1, "one", 0.6, 1, time.Minute)
			require.NoError(t, err)
			require.True(t, allowed)
			require.Zero(t, inflightCount(t, cache, 1))
			alive, err := cache.RenewInflightBalance(ctx, 1, "one", time.Minute)
			require.NoError(t, err)
			require.False(t, alive)
		})
	}
}

func TestInflightAccountingMixedVersionsAndGeneration(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	peer := &billingCache{rdb: cache.rdb}
	ctx := context.Background()
	z, h := billingInflightKeys(2)
	reserve := func(id string, want float64) {
		ok, sum, err := peer.ReserveInflightBalance(ctx, 2, id, 1, 100, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		require.InDelta(t, want, sum, 1e-9)
	}
	reserve("first", 0)
	// 模拟旧版只更新原有两个明细键。
	require.NoError(t, cache.rdb.ZAdd(ctx, z, redis.Z{Score: float64(time.Now().Add(time.Minute).UnixMilli()), Member: "old"}).Err())
	require.NoError(t, cache.rdb.HSet(ctx, h, "old", 3).Err())
	reserve("second", 4)
	setInflightMode(t, cache, "aggregate", "a")
	reserve("third", 5)
	require.NoError(t, cache.ReleaseInflightBalance(ctx, 2, "old"))
	reserve("fourth", 3)
	setInflightMode(t, cache, "compat", "b")
	require.NoError(t, cache.rdb.ZRem(ctx, z, "first").Err())
	require.NoError(t, cache.rdb.HDel(ctx, h, "first").Err())
	reserve("fifth", 3)
	setInflightMode(t, cache, "aggregate", "c")
	reserve("sixth", 4)
	require.NoError(t, cache.rdb.HDel(ctx, z+":total", "sum").Err())
	reserve("seventh", 5)
	require.Equal(t, "c", cache.rdb.HGet(ctx, z+":total", "generation").Val())
}

func TestInflightAccountingBoundedExpiryAndConcurrentRelease(t *testing.T) {
	_, cache, _ := newInflightTestEnv(t, true, 60)
	setInflightMode(t, cache, "aggregate", "a")
	ctx := context.Background()
	z, h := billingInflightKeys(3)
	pipe := cache.rdb.Pipeline()
	for i := 0; i < 400; i++ {
		id := fmt.Sprint(i)
		pipe.ZAdd(ctx, z, redis.Z{Score: float64(time.Now().Add(-time.Second).UnixMilli()), Member: id})
		pipe.HSet(ctx, h, id, 0.25)
	}
	_, err := pipe.Exec(ctx)
	require.NoError(t, err)
	ok, prior, err := cache.ReserveInflightBalance(ctx, 3, "live", 0.5, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Zero(t, prior)
	require.Equal(t, int64(1), inflightCount(t, cache, 3))
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Go(func() {
			require.NoError(t, cache.ReleaseInflightBalance(ctx, 3, "live"))
			_, err := cache.RenewInflightBalance(ctx, 3, "live", time.Minute)
			require.NoError(t, err)
		})
	}
	wg.Wait()
	require.Zero(t, inflightCount(t, cache, 3))
	require.Equal(t, "0", cache.rdb.HGet(ctx, z+":total", "sum").Val())
}
