// Package cleanupqueue 为结果未知或释放失败的 Redis 租约提供有界补偿。
package cleanupqueue

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	Workers        = 4
	Capacity       = 16384
	AttemptTimeout = 2 * time.Second
)

type counters struct{ queued, confirmed, retries, full, fallback atomic.Int64 }
type Stats struct {
	Queued    int64 `json:"queued_total"`
	Confirmed int64 `json:"confirmed_total"`
	Retries   int64 `json:"retry_total"`
	Full      int64 `json:"queue_full_total"`
	Fallback  int64 `json:"ttl_fallback_total"`
}

var metrics sync.Map

func countersFor(name string) *counters {
	value, _ := metrics.LoadOrStore(name, &counters{})
	c, ok := value.(*counters)
	if !ok {
		panic("补偿指标类型不匹配")
	}
	return c
}

// RecordFallback 也覆盖服务已停止、尚未创建补偿队列时的迟到清理。
func RecordFallback(name, id, reason string) {
	n := countersFor(name).fallback.Add(1)
	if n <= 4 || n%100 == 0 {
		logger.L().Warn("redis.cleanup_ttl_fallback", zap.String("queue", name), zap.String("lease_id", id), zap.String("reason", reason), zap.Int64("total", n))
	}
}

func Metrics() map[string]Stats {
	out := make(map[string]Stats)
	metrics.Range(func(key, value any) bool {
		name, nameOK := key.(string)
		c, countersOK := value.(*counters)
		if nameOK && countersOK {
			out[name] = Stats{c.queued.Load(), c.confirmed.Load(), c.retries.Load(), c.full.Load(), c.fallback.Load()}
		}
		return true
	})
	return out
}

type job struct {
	id           string
	run          func(context.Context) error
	due, expires time.Time
	delay        time.Duration
}
type jobs []*job

func (h jobs) Len() int           { return len(h) }
func (h jobs) Less(i, j int) bool { return h[i].due.Before(h[j].due) }
func (h jobs) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *jobs) Push(value any) {
	entry, ok := value.(*job)
	if !ok {
		panic("补偿堆只允许租约任务")
	}
	*h = append(*h, entry)
}
func (h *jobs) Pop() any {
	old := *h
	n := len(old) - 1
	value := old[n]
	old[n] = nil
	*h = old[:n]
	return value
}

type Queue struct {
	mu       sync.Mutex
	name     string
	capacity int
	pending  map[string]*job
	ready    jobs
	wake     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	stopping bool
	stopOnce sync.Once
	wg       sync.WaitGroup
	stats    *counters
}

func New(name string) *Queue { return newQueue(name, Workers, Capacity) }

func newQueue(name string, workers, capacity int) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	stats := countersFor(name)
	q := &Queue{name: name, capacity: capacity, pending: make(map[string]*job), wake: make(chan struct{}, workers), ctx: ctx, cancel: cancel, stats: stats}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker()
	}
	return q
}

func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Enqueue 按租约 ID 去重，容量包含正在执行的项，退避项不占用工作协程。
func (q *Queue) Enqueue(id string, expires time.Time, run func(context.Context) error) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.pending[id]; ok {
		return true
	}
	if q.stopping || len(q.pending) >= q.capacity {
		if !q.stopping {
			q.stats.full.Add(1)
		}
		q.fallback(id, "补偿队列已满或停止")
		return false
	}
	j := &job{id: id, run: run, expires: expires, due: time.Now(), delay: 100 * time.Millisecond}
	q.pending[id] = j
	heap.Push(&q.ready, j)
	q.stats.queued.Add(1)
	q.signal()
	return true
}

func (q *Queue) fallback(id, reason string) {
	RecordFallback(q.name, id, reason)
}

func (q *Queue) worker() {
	defer q.wg.Done()
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		q.mu.Lock()
		if q.ctx.Err() != nil {
			q.mu.Unlock()
			return
		}
		if q.stopping && len(q.pending) == 0 {
			q.mu.Unlock()
			return
		}
		wait := time.Hour
		var next *job
		if len(q.ready) > 0 {
			wait = time.Until(q.ready[0].due)
			if wait <= 0 || q.stopping {
				next = q.ready[0]
				heap.Pop(&q.ready)
			}
		}
		q.mu.Unlock()
		if next == nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(max(wait, time.Millisecond))
			select {
			case <-q.ctx.Done():
				return
			case <-q.wake:
			case <-timer.C:
			}
			continue
		}
		if time.Now().After(next.expires) {
			q.mu.Lock()
			delete(q.pending, next.id)
			q.mu.Unlock()
			q.fallback(next.id, "超过租约 TTL，交由到期清理")
			q.signal()
			continue
		}
		ctx, cancel := context.WithTimeout(q.ctx, AttemptTimeout)
		err := next.run(ctx)
		cancel()
		q.mu.Lock()
		if err == nil {
			delete(q.pending, next.id)
			q.stats.confirmed.Add(1)
		} else if q.stopping {
			delete(q.pending, next.id)
			q.fallback(next.id, "退出时释放未获确认")
		} else {
			n := q.stats.retries.Add(1)
			if n <= 4 || n%100 == 0 {
				logger.L().Warn("redis.cleanup_retry", zap.String("queue", q.name), zap.String("lease_id", next.id), zap.Int64("total", n), zap.Error(err))
			}
			next.due = time.Now().Add(next.delay)
			next.delay = min(next.delay*2, 5*time.Second)
			heap.Push(&q.ready, next)
		}
		q.mu.Unlock()
		q.signal()
	}
}

// Stop 最多再给两秒排空预算；剩余项显式记录 TTL 兜底，不阻塞进程退出。
func (q *Queue) Stop() {
	if q == nil {
		return
	}
	q.stopOnce.Do(func() {
		q.mu.Lock()
		q.stopping = true
		q.mu.Unlock()
		for i := 0; i < cap(q.wake); i++ {
			q.signal()
		}
		timer := time.AfterFunc(AttemptTimeout, q.cancel)
		q.wg.Wait()
		timer.Stop()
		q.cancel()
		q.mu.Lock()
		for id := range q.pending {
			q.fallback(id, "退出排空预算耗尽")
		}
		q.pending = make(map[string]*job)
		q.ready = nil
		q.mu.Unlock()
	})
}
