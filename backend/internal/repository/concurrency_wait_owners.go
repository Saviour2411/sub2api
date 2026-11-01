package repository

import (
	"context"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

// 花括号内容与旧key完全相同，使两键在Redis Cluster落入相同slot。
func waitOwnersKey(key string) string { return "concurrency:wait_owners:{" + key + "}" }

// 旧版本的字符串计数只读兼容；新版本只增减自己的字段，不重置同伴或旧进程计数。
const readWaitOwnersLua = `
redis.replicate_commands()
local now=tonumber(redis.call('TIME')[1])
local total=tonumber(redis.call('GET',KEYS[1]) or '0')
local entries=redis.call('HGETALL',KEYS[2])
for i=1,#entries,2 do
 local count, expiry=string.match(entries[i+1],'^(%d+):(%d+)$')
 if count and tonumber(expiry)>now then total=total+tonumber(count)
 else redis.call('HDEL',KEYS[2],entries[i]) end
end
`

const mutateWaitOwnersLua = `
local owner=ARGV[1]
local delta=tonumber(ARGV[2])
local limit=tonumber(ARGV[3])
local ttl=tonumber(ARGV[4])
local count=tonumber((string.match(redis.call('HGET',KEYS[2],owner) or '', '^(%d+):'))) or 0
if delta>0 and total>=limit then return {0,now} end
if delta<0 and count==0 then return {0,now} end
count=math.max(0,count+delta)
if count==0 then redis.call('HDEL',KEYS[2],owner)
else
 redis.call('HSET',KEYS[2],owner,tostring(count)..':'..tostring(now+ttl))
 local remaining=redis.call('TTL',KEYS[2])
 if remaining<ttl then redis.call('EXPIRE',KEYS[2],ttl) end
end
return {1,now}
`

var mutateWaitOwnersScript = redis.NewScript(readWaitOwnersLua + mutateWaitOwnersLua)

func (c *concurrencyCache) readWait(ctx context.Context, cmd redis.Scripter, key string) *redis.Cmd {
	return cmd.Eval(ctx, readWaitOwnersLua+`return total`, []string{key, waitOwnersKey(key)})
}
func (c *concurrencyCache) mutateWait(ctx context.Context, key string, delta, maxWait int) (int64, int64, error) {
	unlock, err := c.lockWaitKey(ctx, key)
	if err != nil {
		return 0, 0, err
	}
	defer unlock()
	result, now, err := runScriptInt64Pair(ctx, c.rdb, mutateWaitOwnersScript, []string{key, waitOwnersKey(key)}, c.owner, delta, maxWait, c.waitQueueTTLSeconds)
	if err == nil {
		c.ownedMu.Lock()
		defer c.ownedMu.Unlock()
		if result == 1 && delta > 0 {
			c.waitKeys[key]++
		}
		if delta < 0 {
			c.waitKeys[key]--
			if c.waitKeys[key] <= 0 {
				delete(c.waitKeys, key)
			}
		}
	}
	return result, now, err
}

// RefreshOwnedLeases只刷新本进程仍持有的槽和等待字段；不复活已释放的槽。
func (c *concurrencyCache) RefreshOwnedLeases(ctx context.Context) error {
	type lease struct{ key, member string }
	var slots []lease
	var waits []string
	c.ownedMu.Lock()
	for key, members := range c.ownedSlots {
		for member := range members {
			slots = append(slots, lease{key, member})
		}
	}
	for key := range c.waitKeys {
		waits = append(waits, key)
	}
	c.ownedMu.Unlock()
	const batchSize = 128
	for len(slots) > 0 || len(waits) > 0 {
		pipe := c.rdb.Pipeline()
		n := 0
		for len(slots) > 0 && n < batchSize {
			slot := slots[0]
			slots = slots[1:]
			n++
			index, prefix := userActiveIndexKey, userSlotKeyPrefix
			if strings.HasPrefix(slot.key, accountSlotKeyPrefix) {
				index, prefix = accountActiveIndexKey, accountSlotKeyPrefix
			}
			pipe.Eval(ctx, renewOwnedSlotLua, []string{slot.key, index}, slot.member, c.slotTTLSeconds, strings.TrimPrefix(slot.key, prefix))
		}
		for len(waits) > 0 && n < batchSize {
			key := waits[0]
			waits = waits[1:]
			n++
			pipe.Eval(ctx, readWaitOwnersLua+mutateWaitOwnersLua, []string{key, waitOwnersKey(key)}, c.owner, 0, 0, c.waitQueueTTLSeconds)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

type waitKeyLock struct {
	token chan struct{}
	refs  int
}

// 按键协调本地计数与 Redis 操作；等待锁同样服从调用方截止时间。
func (c *concurrencyCache) lockWaitKey(ctx context.Context, key string) (func(), error) {
	c.ownedMu.Lock()
	if c.waitLocks == nil {
		c.waitLocks = make(map[string]*waitKeyLock)
	}
	lock := c.waitLocks[key]
	if lock == nil {
		lock = &waitKeyLock{token: make(chan struct{}, 1)}
		c.waitLocks[key] = lock
	}
	lock.refs++
	c.ownedMu.Unlock()
	drop := func() {
		c.ownedMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(c.waitLocks, key)
		}
		c.ownedMu.Unlock()
	}
	select {
	case lock.token <- struct{}{}:
		return func() { <-lock.token; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	}
}

func (c *concurrencyCache) rememberSlot(key, requestID string, held bool) {
	c.ownedMu.Lock()
	defer c.ownedMu.Unlock()
	if held {
		if c.ownedSlots[key] == nil {
			c.ownedSlots[key] = make(map[string]bool)
		}
		c.ownedSlots[key][requestID] = true
	} else {
		delete(c.ownedSlots[key], requestID)
		if len(c.ownedSlots[key]) == 0 {
			delete(c.ownedSlots, key)
		}
	}
}

// 启动恢复只裁剪到期成员，不把非当前进程或未知旧前缀当作死进程。
func (c *concurrencyCache) cleanupExpiredIndex(ctx context.Context, spec slotIndexSpec, now int64) error {
	members, err := c.allIndexMembers(ctx, spec.indexKey)
	if err != nil {
		return err
	}
	for _, member := range members {
		id, err := strconv.ParseInt(member, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if err = c.rdb.ZRemRangeByScore(ctx, spec.slotKey(id), "-inf", strconv.FormatInt(now-int64(c.slotTTLSeconds), 10)).Err(); err != nil {
			return err
		}
		c.refreshActiveIndex(ctx, spec.indexKey, id, spec.slotKey(id), spec.waitKey(id))
	}
	return nil
}
