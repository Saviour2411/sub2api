package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/cleanupqueue"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const slotAcquireTimeout = 2 * time.Second

type SlotOperationError struct {
	Phase, SlotType, SlotID string
	Err                     error
}

func (e *SlotOperationError) Error() string {
	return fmt.Sprintf("并发槽位 %s/%s 失败：%v", e.SlotType, e.Phase, e.Err)
}
func (e *SlotOperationError) Unwrap() error { return e.Err }

func (s *ConcurrencyService) acquireSlot(ctx context.Context, kind string, id int64, limit int) (*AcquireResult, error) {
	if limit <= 0 {
		return &AcquireResult{Acquired: true, ReleaseFunc: noopRelease}, nil
	}
	if s == nil || s.cache == nil {
		return nil, &SlotOperationError{Phase: "acquire", SlotType: kind, Err: fmt.Errorf("并发缓存不可用")}
	}
	requestID := generateRequestID()
	acquire, release := s.cache.AcquireUserSlot, s.cache.ReleaseUserSlot
	if kind == "account" {
		acquire, release = s.cache.AcquireAccountSlot, s.cache.ReleaseAccountSlot
	}
	cleanup := func(ctx context.Context) error { return release(ctx, id, requestID) }
	ttl := 15 * time.Minute
	if cache, ok := s.cache.(interface{ SlotTTL() time.Duration }); ok {
		ttl = cache.SlotTTL()
	}
	schedule := func() { s.scheduleSlotCleanup(kind+":"+requestID, time.Now().Add(ttl), cleanup) }
	started := time.Now()
	call, cancel := context.WithTimeout(ctx, slotAcquireTimeout)
	acquired, err := acquire(call, id, limit, requestID)
	cancel()
	if err != nil {
		// 响应丢失时 Redis 可能已占槽，异步释放复用原 ID，完成标记挡住迟到写入。
		schedule()
		logger.FromContext(ctx).Warn("gateway.slot_operation_failed", zap.String("phase", "acquire"), zap.String("slot_type", kind), zap.Int64("subject_id", id), zap.String("slot_id", requestID), zap.Duration("elapsed", time.Since(started)), zap.Error(err))
		return nil, &SlotOperationError{Phase: "acquire", SlotType: kind, SlotID: requestID, Err: err}
	}
	if !acquired {
		return &AcquireResult{}, nil
	}
	var once sync.Once
	return &AcquireResult{Acquired: true, ReleaseFunc: func() {
		once.Do(func() {
			call, cancel := context.WithTimeout(context.Background(), cleanupqueue.AttemptTimeout)
			err := cleanup(call)
			cancel()
			if err != nil {
				schedule()
			}
		})
	}}, nil
}

func (s *ConcurrencyService) scheduleSlotCleanup(id string, expires time.Time, run func(context.Context) error) {
	s.cleanupOnce.Do(func() { s.cleanup = cleanupqueue.New("concurrency_slot") })
	if s.cleanup == nil {
		cleanupqueue.RecordFallback("concurrency_slot", id, "服务已停止")
		return
	}
	if s.stopped.Load() {
		s.cleanup.Stop()
	}
	s.cleanup.Enqueue(id, expires, run)
}

func (s *ConcurrencyService) Stop() {
	if s == nil {
		return
	}
	s.stopped.Store(true)
	s.cleanupOnce.Do(func() {})
	s.cleanup.Stop()
}
