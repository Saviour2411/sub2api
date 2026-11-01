package repository

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

func (c *concurrencyCache) SlotTTL() time.Duration {
	return time.Duration(c.slotTTLSeconds) * time.Second
}

// 完成标记防止超时获取在补偿之后迟到执行；槽位与索引同一次原子更新。
var releaseIndexedSlotScript = redis.NewScript(`
redis.replicate_commands()
local now=tonumber(redis.call('TIME')[1])
redis.call('SET',KEYS[6],'1','EX',math.max(120,tonumber(ARGV[3])))
redis.call('ZREM',KEYS[1],ARGV[1])
local slots=redis.call('ZCARD',KEYS[1])+redis.call('ZCARD',KEYS[2])
local waits=tonumber(redis.call('GET',KEYS[3]) or '0')
local owners=redis.call('HVALS',KEYS[4])
for i=1,#owners do
 local count,expiry=string.match(owners[i],'^(%d+):(%d+)$')
 if count and tonumber(expiry)>now then waits=waits+tonumber(count) end
end
if slots==0 and waits==0 then redis.call('ZREM',KEYS[5],ARGV[2])
else
 local ttl=0
 if slots>0 then ttl=tonumber(ARGV[3]) end
 if waits>0 then ttl=math.max(ttl,tonumber(ARGV[4])) end
 redis.call('ZADD',KEYS[5],now+ttl,ARGV[2])
end
return 1
`)

func (c *concurrencyCache) releaseIndexedSlot(ctx context.Context, key, live, wait, index string, id int64, requestID string) error {
	return releaseIndexedSlotScript.Run(ctx, c.rdb, []string{key, live, wait, waitOwnersKey(wait), index, key + ":done:" + requestID}, requestID, id, c.slotTTLSeconds, c.waitQueueTTLSeconds).Err()
}

// 旧快照仅刷新尚未到期的现存成员，不复活已释放或已过期的槽位。
const renewOwnedSlotLua = `
redis.replicate_commands()
local now=tonumber(redis.call('TIME')[1])
local score=redis.call('ZSCORE',KEYS[1],ARGV[1])
if not score or tonumber(score)<=now-tonumber(ARGV[2]) then return 0 end
redis.call('ZADD',KEYS[1],'XX',now,ARGV[1])
redis.call('EXPIRE',KEYS[1],ARGV[2])
redis.call('ZADD',KEYS[2],now+tonumber(ARGV[2]),ARGV[3])
return 1
`
