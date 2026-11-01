package repository

// 保留 v0.1.263 原脚本，用于真实 Redis 性能和混跑基线。
const legacyInflightLua = `
		local now = tonumber(ARGV[1])
		local expired = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', now)
		if #expired > 0 then
			redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
			redis.call('HDEL', KEYS[2], unpack(expired))
		end
		local vals = redis.call('HVALS', KEYS[2])
		local sum = 0
		for i = 1, #vals do
			sum = sum + (tonumber(vals[i]) or 0)
		end
		local count = redis.call('ZCARD', KEYS[1])
		local amount = tonumber(ARGV[4])
		local balance = tonumber(ARGV[5])
		if count > 0 and (balance - sum) < amount then
			return {0, tostring(sum), count}
		end
		redis.call('ZADD', KEYS[1], tonumber(ARGV[2]), ARGV[3])
		redis.call('HSET', KEYS[2], ARGV[3], ARGV[4])
		redis.call('PEXPIRE', KEYS[1], ARGV[6])
		redis.call('PEXPIRE', KEYS[2], ARGV[6])
		return {1, tostring(sum), count}
	`
