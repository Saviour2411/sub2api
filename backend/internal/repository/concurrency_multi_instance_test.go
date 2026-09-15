package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestNewInstancePreservesPeerSlotsAndWaits(t *testing.T) {
	r := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: r.Addr()})
	defer func() { _ = client.Close() }()
	a, ok := NewConcurrencyCache(client, 15, 900).(*concurrencyCache)
	require.True(t, ok)
	a.owner = "a"
	b, ok := NewConcurrencyCache(client, 15, 900).(*concurrencyCache)
	require.True(t, ok)
	b.owner = "b"
	ctx := context.Background()
	ok, err := a.AcquireAccountSlot(ctx, 1, 3, "a-live")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = a.IncrementAccountWaitCount(ctx, 1, 2)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, b.CleanupStaleProcessSlots(ctx, "b"))
	n, err := b.GetAccountConcurrency(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	n, err = b.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.NoError(t, b.DecrementAccountWaitCount(ctx, 1))
	n, err = b.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	ok, err = b.IncrementAccountWaitCount(ctx, 1, 2)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = a.IncrementAccountWaitCount(ctx, 1, 2)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, a.DecrementAccountWaitCount(ctx, 1))
	n, err = b.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.NoError(t, b.DecrementAccountWaitCount(ctx, 1))
	n, err = b.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, n)
}
func TestLegacyWaitsArePreservedAndCounted(t *testing.T) {
	r := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: r.Addr()})
	defer func() { _ = client.Close() }()
	c, ok := NewConcurrencyCache(client, 15, 900).(*concurrencyCache)
	require.True(t, ok)
	ctx := context.Background()
	require.NoError(t, client.Set(ctx, accountWaitKey(1), 2, time.Minute).Err())
	require.NoError(t, c.CleanupStaleProcessSlots(ctx, "new"))
	ok, err := c.IncrementAccountWaitCount(ctx, 1, 2)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, c.DecrementAccountWaitCount(ctx, 1))
	count, err := c.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
func TestRenewalNeverResurrectsReleasedSlot(t *testing.T) {
	r := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: r.Addr()})
	defer func() { _ = client.Close() }()
	c, ok := NewConcurrencyCache(client, 15, 900).(*concurrencyCache)
	require.True(t, ok)
	ctx := context.Background()
	ok, err := c.AcquireUserSlot(ctx, 1, 2, "a-live")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, c.RefreshOwnedLeases(ctx))
	require.NoError(t, c.ReleaseUserSlot(ctx, 1, "a-live"))
	require.NoError(t, c.RefreshOwnedLeases(ctx))
	count, err := c.GetUserConcurrency(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, count)
}
