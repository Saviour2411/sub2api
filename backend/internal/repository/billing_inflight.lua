-- KEYS：到期集合、金额明细、累计元数据、共享模式、当前 ID 完成标记、原始结果。
-- ARGV：操作、预留 ID、金额、余额、TTL 毫秒。所有操作在同一 Redis 实例原子执行。
redis.replicate_commands()
local op, id = ARGV[1], ARGV[2]
local amount, balance, ttl = tonumber(ARGV[3]), tonumber(ARGV[4]), tonumber(ARGV[5])
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
local mode = redis.call('HMGET', KEYS[4], 'mode', 'generation')
local aggregate = mode[1] == 'aggregate' and mode[2] and mode[2] ~= ''
local count = redis.call('ZCARD', KEYS[1])
local sum = nil
if aggregate then
    local meta = redis.call('HMGET', KEYS[3], 'generation', 'sum', 'count')
    if meta[1] == mode[2] and tonumber(meta[3]) == count and redis.call('HLEN', KEYS[2]) == count then
        sum = tonumber(meta[2])
    end
else
    redis.call('DEL', KEYS[3])
end

local function extend(key, milliseconds)
    if redis.call('PTTL', key) < milliseconds then redis.call('PEXPIRE', key, milliseconds) end
end

local function total()
    if not sum then
        sum = 0
        local values = redis.call('HVALS', KEYS[2])
        for i = 1, #values do sum = sum + tonumber(values[i]) end
    end
    return sum
end

local function persist()
    if aggregate then
        total()
        if count == 0 then sum = 0 end
        sum = math.max(0, sum)
        redis.call('HSET', KEYS[3], 'generation', mode[2], 'sum', string.format('%.17g', sum), 'count', count)
        extend(KEYS[3], math.max(ttl, redis.call('PTTL', KEYS[1]), 1))
    end
end

local function remove(member)
    local value = redis.call('HGET', KEYS[2], member)
    if sum and value then sum = sum - tonumber(value) end
    count = count - redis.call('ZREM', KEYS[1], member)
    redis.call('HDEL', KEYS[2], member)
    redis.call('HDEL', KEYS[6], member)
end

if op == 'release' then
    -- 完成标记必须先于任何迟到写入生效；重复释放刷新标记，不重复扣累计。
    local prior = redis.call('HGET', KEYS[6], id) or redis.call('GET', KEYS[5]) or '0'
    redis.call('SET', KEYS[5], prior, 'PX', math.max(1800000, redis.call('PTTL', KEYS[1])))
    remove(id)
    persist()
    return {1, '0'}
end

local completed = redis.call('GET', KEYS[5])
if completed then
    if op == 'renew' then return {0, '0'} end
    return {1, completed}
end

if op == 'renew' then
    local score = redis.call('ZSCORE', KEYS[1], id)
    if not score then return {0, '0'} end
    if tonumber(score) <= now then
        remove(id)
        persist()
        return {0, '0'}
    end
    redis.call('ZADD', KEYS[1], 'XX', now + ttl, id)
    for _, key in ipairs({KEYS[1], KEYS[2], KEYS[6]}) do extend(key, ttl) end
    persist()
    return {1, '0'}
end

local expired = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', now, 'LIMIT', 0, 128)
for i = 1, #expired do remove(expired[i]) end
if #expired == 128 and redis.call('ZCOUNT', KEYS[1], '-inf', now) > 0 then
    -- 元数据失效时最后一批才重建，避免过期堆积触发重复扫描。
    if aggregate and sum then persist() end
    return {2, '0'}
end
total()
local existing = redis.call('HGET', KEYS[2], id)
if existing and redis.call('ZSCORE', KEYS[1], id) then
    persist()
    return {1, redis.call('HGET', KEYS[6], id) or string.format('%.17g', math.max(0, sum - tonumber(existing)))}
end
local prior = string.format('%.17g', sum)
if op == 'balance' and count > 0 and balance - sum < amount then
    persist()
    return {0, prior}
end
redis.call('ZADD', KEYS[1], now + ttl, id)
redis.call('HSET', KEYS[2], id, ARGV[3])
redis.call('HSET', KEYS[6], id, prior)
count = count + 1
sum = sum + amount
for _, key in ipairs({KEYS[1], KEYS[2], KEYS[6]}) do extend(key, ttl) end
persist()
return {1, prior}
