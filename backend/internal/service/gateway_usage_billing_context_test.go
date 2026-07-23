package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type billingContextTestKey struct{}

func TestDetachedBillingContext_PreservesShorterDeadlineAndValue(t *testing.T) {
	wantValue := "worker"
	parent := context.WithValue(context.Background(), billingContextTestKey{}, wantValue)
	parent, parentCancel := context.WithTimeout(parent, 200*time.Millisecond)
	defer parentCancel()

	wantDeadline, ok := parent.Deadline()
	require.True(t, ok)
	ctx, cancel := detachedBillingContext(parent)
	defer cancel()

	gotDeadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, wantDeadline, gotDeadline, time.Millisecond)
	require.Equal(t, wantValue, ctx.Value(billingContextTestKey{}))
}

func TestDetachedBillingContext_IgnoresCancellationWithoutDeadline(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	ctx, cancel := detachedBillingContext(parent)
	defer cancel()

	parentCancel()
	select {
	case <-ctx.Done():
		t.Fatalf("后台扣费 context 不应继承请求取消: %v", ctx.Err())
	case <-time.After(30 * time.Millisecond):
	}
}

func TestDetachedBillingContext_CapsLongerDeadline(t *testing.T) {
	parent, parentCancel := context.WithTimeout(context.Background(), postUsageBillingTimeout*2)
	defer parentCancel()
	started := time.Now()

	ctx, cancel := detachedBillingContext(parent)
	defer cancel()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, started.Add(postUsageBillingTimeout), deadline, 100*time.Millisecond)
}

func TestUsageRecordWorkerPool_DetachedBillingContextKeepsWorkerTimeout(t *testing.T) {
	pool := &UsageRecordWorkerPool{taskTimeout: 40 * time.Millisecond}
	started := time.Now()
	pool.execute(func(workerCtx context.Context) {
		billingCtx, cancel := detachedBillingContext(workerCtx)
		defer cancel()
		<-billingCtx.Done()
		require.ErrorIs(t, billingCtx.Err(), context.DeadlineExceeded)
	})
	require.Less(t, time.Since(started), time.Second)
}
