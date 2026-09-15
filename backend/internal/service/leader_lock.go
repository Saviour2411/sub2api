package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
)

// LeaderLockCache provides cross-instance mutual exclusion for periodic background
// jobs. It is implemented in the repository layer (Redis-backed) so the service
// layer never depends on Redis directly. Release is a compare-and-delete keyed by
// owner so a stale holder can never delete a peer's lock.
type LeaderLockCache interface {
	// TryAcquireLeaderLock sets key=owner with the given TTL iff key is absent.
	// It returns true when the caller becomes the owner.
	TryAcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	// ReleaseLeaderLock deletes key iff it is still owned by owner.
	ReleaseLeaderLock(ctx context.Context, key, owner string) error
}

// tryAcquireSingletonLeaderLock 对有数据库的实例始终使用PostgreSQL互斥。
// Redis-only适配器仅供不带数据库的旧调用/测试使用；故障时不放行任务。
// 同一共享任务不得在不同实例使用Redis锁与数据库锁两个独立互斥域。
func tryAcquireSingletonLeaderLock(ctx context.Context, cache LeaderLockCache, db *sql.DB, key, owner string, ttl time.Duration) (func(), bool) {
	release, ok, _ := tryAcquireSingletonLeaderLockWithError(ctx, cache, db, key, owner, ttl)
	return release, ok
}

func tryAcquireSingletonLeaderLockWithError(ctx context.Context, cache LeaderLockCache, db *sql.DB, key, owner string, ttl time.Duration) (func(), bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	done, accepting := lifecycle.Process.BeginBackground()
	if !accepting {
		return nil, false, nil
	}
	success := false
	defer func() {
		if !success {
			done()
		}
	}()
	if db == nil {
		db = lifecycle.Process.SharedDB()
	}
	if db != nil {
		release, ok, err := tryAcquireDBAdvisoryLockWithError(ctx, db, hashAdvisoryLockID(key))
		if !ok {
			return nil, false, err
		}
		success = true
		return func() { defer done(); release() }, true, nil
	}
	if cache != nil {
		ok, err := cache.TryAcquireLeaderLock(ctx, key, owner, ttl)
		if err == nil {
			if !ok {
				return nil, false, nil
			}
			release := func() {
				defer done()
				ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = cache.ReleaseLeaderLock(ctx2, key, owner)
			}
			success = true
			return release, true, nil
		}
		// 未配置数据库的旧接口不在Redis故障时绕过互斥。
		return nil, false, err
	}

	// No coordination backend available: run without gating.
	success = true
	return done, true, nil
}

// 所有共享调度入口使用同一个 PostgreSQL 锁域；请求侧服务不经过此入口。
func trySharedBackgroundJob(ctx context.Context, key string) (func(), bool) {
	return tryAcquireSingletonLeaderLock(ctx, nil, lifecycle.Process.SharedDB(), "shared:"+key, lifecycle.Process.ID(), time.Minute)
}
