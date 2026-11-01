package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type leaseInterleaveHook struct {
	redis.Hook
	pipeline func(context.Context, []redis.Cmder)
	process  func(context.Context, redis.Cmder)
}

func (h leaseInterleaveHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h leaseInterleaveHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.process != nil {
			h.process(ctx, cmd)
		}
		return next(ctx, cmd)
	}
}
func (h leaseInterleaveHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if h.pipeline != nil {
			h.pipeline(ctx, cmds)
		}
		return next(ctx, cmds)
	}
}

func TestConcurrencySlowWaitDoesNotHoldGlobalLock(t *testing.T) {
	r := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: r.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c, cacheOK := NewConcurrencyCache(rdb, 15, 900).(*concurrencyCache)
	require.True(t, cacheOK)
	entered, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	rdb.AddHook(leaseInterleaveHook{process: func(ctx context.Context, cmd redis.Cmder) {
		if cmd.Name() == "evalsha" || cmd.Name() == "eval" {
			args := cmd.Args()
			if len(args) > 3 && args[3] == waitQueueKey(1) {
				once.Do(func() { close(entered) })
				select {
				case <-unblock:
				case <-ctx.Done():
				}
			}
		}
	}})
	done := make(chan error, 1)
	go func() { _, err := c.IncrementWaitCount(context.Background(), 1, 10); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	ok, err := c.AcquireUserSlot(ctx, 2, 1, "other")
	require.NoError(t, err)
	require.True(t, ok)
	_, err = c.IncrementWaitCount(ctx, 2, 10)
	require.NoError(t, err)
	close(unblock)
	require.NoError(t, <-done)
}

func TestConcurrencyRenewalSnapshotCannotRestoreReleasedWork(t *testing.T) {
	r := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: r.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c, cacheOK := NewConcurrencyCache(rdb, 15, 900).(*concurrencyCache)
	require.True(t, cacheOK)
	ctx := context.Background()
	for i := int64(1); i <= 260; i++ {
		ok, err := c.AcquireUserSlot(ctx, i, 1, "live")
		require.NoError(t, err)
		require.True(t, ok)
	}
	_, err := c.IncrementWaitCount(ctx, 1, 10)
	require.NoError(t, err)
	entered, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var largest atomic.Int64
	rdb.AddHook(leaseInterleaveHook{pipeline: func(_ context.Context, cmds []redis.Cmder) {
		largest.Store(max(largest.Load(), int64(len(cmds))))
		if len(cmds) > 0 && cmds[0].Name() == "eval" && cmds[0].Args()[1] == renewOwnedSlotLua {
			once.Do(func() { close(entered); <-unblock })
		}
	}})
	done := make(chan error, 1)
	go func() { done <- c.RefreshOwnedLeases(ctx) }()
	<-entered
	require.NoError(t, c.ReleaseUserSlot(ctx, 1, "live"))
	require.NoError(t, c.DecrementWaitCount(ctx, 1))
	close(unblock)
	require.NoError(t, <-done)
	require.LessOrEqual(t, largest.Load(), int64(128))
	require.Zero(t, rdb.ZCard(ctx, userSlotKey(1)).Val())
	require.False(t, rdb.HExists(ctx, waitOwnersKey(waitQueueKey(1)), c.owner).Val())
	ok, err := c.AcquireUserSlot(ctx, 1, 1, "live")
	require.NoError(t, err)
	require.False(t, ok, "完成标记阻止迟到获取复活")
}
