package cleanupqueue

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestCleanupQueueDeduplicatesAndRetries(t *testing.T) {
	q := newQueue(t.Name(), 1, 2)
	t.Cleanup(q.Stop)
	var calls atomic.Int32
	started, unblock, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	run := func(context.Context) error {
		if calls.Add(1) == 1 {
			close(started)
			<-unblock
			return errors.New("模拟断连")
		}
		close(done)
		return nil
	}
	require.True(t, q.Enqueue("lease", time.Now().Add(time.Minute), run))
	<-started
	for i := 0; i < 10; i++ {
		require.True(t, q.Enqueue("lease", time.Now().Add(time.Minute), run))
	}
	close(unblock)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("补偿未完成")
	}
	q.Stop()
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, int64(1), q.stats.queued.Load())
	require.Equal(t, int64(1), q.stats.confirmed.Load())
}

func TestCleanupQueueFullAndShutdownAreBounded(t *testing.T) {
	q := newQueue(t.Name(), 1, 1)
	started := make(chan struct{})
	require.True(t, q.Enqueue("one", time.Now().Add(time.Minute), func(ctx context.Context) error { close(started); <-ctx.Done(); return ctx.Err() }))
	<-started
	require.False(t, q.Enqueue("two", time.Now().Add(time.Minute), func(context.Context) error { return nil }))
	start := time.Now()
	q.Stop()
	require.Less(t, time.Since(start), 3*time.Second)
	require.Equal(t, int64(1), q.stats.full.Load())
	require.GreaterOrEqual(t, q.stats.fallback.Load(), int64(2))
	require.False(t, q.Enqueue("after-stop", time.Now().Add(time.Minute), func(context.Context) error { return nil }))
}

func TestCleanupQueueExpiredTaskFallsBackWithoutWrite(t *testing.T) {
	q := newQueue(t.Name(), 1, 1)
	t.Cleanup(q.Stop)
	var calls atomic.Int32
	require.True(t, q.Enqueue("expired", time.Now().Add(-time.Second), func(context.Context) error { calls.Add(1); return nil }))
	require.Eventually(t, func() bool { return q.stats.fallback.Load() == 1 }, time.Second, time.Millisecond)
	require.Zero(t, calls.Load())
}
