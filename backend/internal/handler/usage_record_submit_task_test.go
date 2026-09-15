package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newUsageRecordTestPool(t *testing.T) *service.UsageRecordWorkerPool {
	t.Helper()
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             8,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	return pool
}

func TestGatewayHandlerSubmitUsageRecordTask_WithPool(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &GatewayHandler{usageRecordWorkerPool: pool}

	done := make(chan struct{})
	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestGatewayHandlerSubmitUsageRecordTask_WithoutPoolSyncFallback(t *testing.T) {
	h := &GatewayHandler{}
	var called atomic.Bool

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected deadline in fallback context")
		}
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestGatewayHandlerSubmitUsageRecordTask_NilTask(t *testing.T) {
	h := &GatewayHandler{}
	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), nil)
	})
}

func TestGatewayHandlerSubmitUsageRecordTask_WithoutPool_TaskPanicRecovered(t *testing.T) {
	h := &GatewayHandler{}
	var called atomic.Bool

	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
			panic("usage task panic")
		})
	})

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	require.True(t, called.Load(), "panic 后后续任务应仍可执行")
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithPool(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	done := make(chan struct{})
	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithoutPoolSyncFallback(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	var called atomic.Bool

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected deadline in fallback context")
		}
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_NilTask(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), nil)
	})
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithoutPool_TaskPanicRecovered(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	var called atomic.Bool

	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
			panic("usage task panic")
		})
	})

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	require.True(t, called.Load(), "panic 后后续任务应仍可执行")
}

func TestOpenAIGatewayHandlerSubmitMandatoryUsageRecordTask_DroppedTaskSyncFallback(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	block := make(chan struct{})
	release := make(chan struct{})
	pool.Submit(func(ctx context.Context) {
		close(block)
		<-release
	})
	<-block
	pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	h.submitMandatoryUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	close(release)

	require.True(t, called.Load(), "mandatory usage task must run synchronously when async submit is dropped")
}

func TestOpenAIGatewayHandlerSubmitOpenAIUsageRecordTask_ImageResultUsesMandatoryFallback(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	block := make(chan struct{})
	release := make(chan struct{})
	pool.Submit(func(ctx context.Context) {
		close(block)
		<-release
	})
	<-block
	pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	h.submitOpenAIUsageRecordTask(context.Background(), &service.OpenAIForwardResult{ImageCount: 1, ImageDelivered: true}, func(ctx context.Context) {
		called.Store(true)
	})
	close(release)

	require.True(t, called.Load(), "image usage task must be mandatory when async submit is dropped")
}

func TestOpenAIGatewayHandlerSubmitOpenAIUsageRecordTask_SearchCountUsesMandatoryFallback(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	block := make(chan struct{})
	release := make(chan struct{})
	pool.Submit(func(ctx context.Context) {
		close(block)
		<-release
	})
	<-block
	pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	h.submitOpenAIUsageRecordTask(context.Background(), &service.OpenAIForwardResult{SearchCount: 3}, func(ctx context.Context) {
		called.Store(true)
	})
	close(release)

	require.True(t, called.Load(), "search surcharge usage task must be mandatory when async submit is dropped")
}

// 正常鉴权请求携带UserID，必须覆盖SubmitKeyed而不仅是无用户上下文的Submit。
func TestStoppedKeyedUsageRunsFallbackExactlyOnce(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Second})
	pool.Stop()
	parent := context.WithValue(context.Background(), ctxkey.UserID, int64(42))
	for _, submit := range []func(context.Context, service.UsageRecordTask){
		(&GatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask,
		(&OpenAIGatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask,
		(&OpenAIGatewayHandler{usageRecordWorkerPool: pool}).submitMandatoryUsageRecordTask,
	} {
		var calls atomic.Int32
		submit(parent, func(context.Context) { calls.Add(1) })
		require.Equal(t, int32(1), calls.Load())
	}
}
