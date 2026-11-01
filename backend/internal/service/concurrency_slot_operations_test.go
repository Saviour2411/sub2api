package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type unknownSlotCache struct {
	ConcurrencyCache
	acquired chan string
	released chan string
	block    bool
	count    atomic.Int64
}

func (c *unknownSlotCache) AcquireUserSlot(ctx context.Context, _ int64, _ int, id string) (bool, error) {
	c.acquired <- id
	if c.block {
		<-ctx.Done()
	} else {
		c.count.Add(1)
	}
	return false, context.DeadlineExceeded
}
func (c *unknownSlotCache) ReleaseUserSlot(_ context.Context, _ int64, id string) error {
	c.count.Store(0)
	c.released <- id
	return nil
}
func (c *unknownSlotCache) AcquireAccountSlot(ctx context.Context, id int64, n int, key string) (bool, error) {
	return c.AcquireUserSlot(ctx, id, n, key)
}
func (c *unknownSlotCache) ReleaseAccountSlot(ctx context.Context, id int64, key string) error {
	return c.ReleaseUserSlot(ctx, id, key)
}

func TestSlotAcquireUnknownResultSchedulesSameIDCleanup(t *testing.T) {
	for _, kind := range []string{"user", "account"} {
		t.Run(kind, func(t *testing.T) {
			cache := &unknownSlotCache{acquired: make(chan string, 1), released: make(chan string, 1)}
			svc := NewConcurrencyService(cache)
			t.Cleanup(svc.Stop)
			_, err := svc.acquireSlot(context.Background(), kind, 1, 1)
			var operation *SlotOperationError
			require.ErrorAs(t, err, &operation)
			select {
			case id := <-cache.released:
				require.Equal(t, <-cache.acquired, id)
				require.Equal(t, operation.SlotID, id)
			case <-time.After(time.Second):
				t.Fatal("结果未知的槽位没有补偿")
			}
			require.Zero(t, cache.count.Load())
		})
	}
}

func TestSlotAcquireUsesOneTotalDeadline(t *testing.T) {
	cache := &unknownSlotCache{acquired: make(chan string, 1), released: make(chan string, 1), block: true}
	svc := NewConcurrencyService(cache)
	t.Cleanup(svc.Stop)
	started := time.Now()
	_, err := svc.AcquireUserSlot(context.Background(), 1, 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.GreaterOrEqual(t, time.Since(started), 1900*time.Millisecond)
	require.Less(t, time.Since(started), 2500*time.Millisecond)
	require.Len(t, cache.acquired, 1)
}
