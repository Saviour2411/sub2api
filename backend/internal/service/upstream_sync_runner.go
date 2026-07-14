package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

const (
	upstreamWorkerCount = 4
	upstreamSyncTimeout = 2 * time.Minute
	upstreamLockTTL     = 3 * time.Minute
)

// UpstreamSyncRunner 每分钟扫描到期站点，并以固定 4 路并发执行同步。
type UpstreamSyncRunner struct {
	service *UpstreamService
	lock    LeaderLockCache
	db      *sql.DB
	ctx     context.Context
	cancel  context.CancelFunc
	queue   chan int64
	wg      sync.WaitGroup
	mu      sync.Mutex
	queued  map[int64]struct{}
	started bool
	stopped bool
}

func NewUpstreamSyncRunner(service *UpstreamService, lock LeaderLockCache, db *sql.DB) *UpstreamSyncRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &UpstreamSyncRunner{
		service: service, lock: lock, db: db, ctx: ctx, cancel: cancel,
		queue: make(chan int64, 256), queued: make(map[int64]struct{}),
	}
}

func (r *UpstreamSyncRunner) Start() {
	if r == nil || r.service == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.mu.Unlock()
	for index := 0; index < upstreamWorkerCount; index++ {
		r.wg.Add(1)
		go r.worker()
	}
	r.wg.Add(1)
	go r.scanLoop()
}

func (r *UpstreamSyncRunner) Enqueue(id int64) {
	if r == nil || id <= 0 {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	if _, exists := r.queued[id]; exists {
		r.mu.Unlock()
		return
	}
	r.queued[id] = struct{}{}
	r.mu.Unlock()
	select {
	case r.queue <- id:
	case <-r.ctx.Done():
		r.finish(id)
	}
}

func (r *UpstreamSyncRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.mu.Unlock()
	r.cancel()
	r.wg.Wait()
}

func (r *UpstreamSyncRunner) scanLoop() {
	defer r.wg.Done()
	r.scan()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.scan()
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *UpstreamSyncRunner) scan() {
	ctx, cancel := context.WithTimeout(r.ctx, 15*time.Second)
	defer cancel()
	ids, err := r.service.ListDue(ctx, time.Now(), 200)
	if err != nil {
		if r.ctx.Err() == nil {
			slog.Error("上游管理：扫描待同步站点失败", "error", err)
		}
		return
	}
	for _, id := range ids {
		r.Enqueue(id)
	}
}

func (r *UpstreamSyncRunner) worker() {
	defer r.wg.Done()
	for {
		select {
		case id := <-r.queue:
			r.runOne(id)
			r.finish(id)
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *UpstreamSyncRunner) runOne(id int64) {
	ctx, cancel := context.WithTimeout(r.ctx, upstreamSyncTimeout)
	defer cancel()
	owner := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	release, acquired := tryAcquireSingletonLeaderLock(ctx, r.lock, r.db, fmt.Sprintf("upstream-site-%d", id), owner, upstreamLockTTL)
	if !acquired {
		return
	}
	defer release()
	if err := r.service.RunSync(ctx, id); err != nil && ctx.Err() == nil {
		slog.Warn("上游管理：站点同步失败", "site_id", id, "error", err)
	}
}

func (r *UpstreamSyncRunner) finish(id int64) {
	r.mu.Lock()
	delete(r.queued, id)
	r.mu.Unlock()
}

func ProvideUpstreamSyncRunner(service *UpstreamService, lock LeaderLockCache, db *sql.DB) *UpstreamSyncRunner {
	runner := NewUpstreamSyncRunner(service, lock, db)
	service.SetScheduler(runner)
	runner.Start()
	return runner
}
