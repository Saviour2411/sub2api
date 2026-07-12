package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const accountFailureStreakPrefix = "account_failure_streak:"

var accountFailureStreakApplyScript = redis.NewScript(`
	local key = KEYS[1]
	local seen_key = KEYS[2]
	local policy_revision = tonumber(ARGV[1])
	local policy_fingerprint = ARGV[2]
	local outcome = ARGV[3]
	local occurred_at_sec = tonumber(ARGV[4])
	local occurred_at_nano = tonumber(ARGV[5])
	local event_id = ARGV[6]

	local stored_revision = tonumber(redis.call('HGET', key, 'policy_revision')) or 0
	local stored_count = tonumber(redis.call('HGET', key, 'count')) or 0
	if redis.call('ZSCORE', seen_key, event_id) then
		return {stored_count, 0, stored_revision}
	end
	if policy_revision < stored_revision then
		return {stored_count, 0, stored_revision}
	end

	if policy_revision > stored_revision then
		stored_count = 0
	else
		local stored_fingerprint = redis.call('HGET', key, 'policy_fingerprint')
		if stored_fingerprint and stored_fingerprint ~= policy_fingerprint then
			return redis.error_reply('account failure streak policy fingerprint mismatch')
		end
		local stored_sec = tonumber(redis.call('HGET', key, 'occurred_at_sec'))
		local stored_nano = tonumber(redis.call('HGET', key, 'occurred_at_nano'))
		if stored_sec then
			local older = occurred_at_sec < stored_sec
				or (occurred_at_sec == stored_sec and occurred_at_nano < stored_nano)
			if older then
				return {stored_count, 0, stored_revision}
			end
		end
	end

	if outcome == 'increment' then
		stored_count = stored_count + 1
	elseif outcome == 'reset' then
		stored_count = 0
	else
		return redis.error_reply('invalid account failure streak outcome')
	end

	redis.call(
		'HSET', key,
		'count', stored_count,
		'policy_revision', ARGV[1],
		'policy_fingerprint', policy_fingerprint,
		'occurred_at_sec', ARGV[4],
		'occurred_at_nano', ARGV[5],
		'event_id', event_id
	)
	local event_sequence = redis.call('HINCRBY', key, 'event_sequence', 1)
	redis.call('ZADD', seen_key, event_sequence, event_id)
	redis.call('ZREMRANGEBYRANK', seen_key, 0, -257)
	return {stored_count, 1, policy_revision}
`)

type accountFailureStreakCache struct {
	rdb           *redis.Client
	keyLocks      sync.Map
	pendingResets sync.Map
}

type accountFailurePendingReset struct {
	policy service.AccountFailureStreakPolicy
	event  service.AccountFailureStreakEvent
}

// NewAccountFailureStreakCache 创建账号连续失败缓存。
func NewAccountFailureStreakCache(rdb *redis.Client) service.AccountFailureStreakCache {
	return &accountFailureStreakCache{rdb: rdb}
}

func (c *accountFailureStreakCache) ApplyOutcome(
	ctx context.Context,
	accountID int64,
	source service.AccountFailureStreakSource,
	policy service.AccountFailureStreakPolicy,
	outcome service.AccountFailureStreakOutcome,
	event service.AccountFailureStreakEvent,
) (service.AccountFailureStreakState, error) {
	if c == nil || c.rdb == nil {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败缓存未初始化")
	}
	if accountID <= 0 {
		return service.AccountFailureStreakState{}, errors.New("账号 ID 无效")
	}
	if source != service.AccountFailureStreakSourceFirstTokenTimeout && source != service.AccountFailureStreakSourceUpstreamError {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败来源无效")
	}
	if outcome != service.AccountFailureStreakOutcomeIncrement && outcome != service.AccountFailureStreakOutcomeReset {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败结果无效")
	}
	if policy.Revision <= 0 || strings.TrimSpace(policy.Fingerprint) == "" {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败策略无效")
	}
	if event.OccurredAt.IsZero() || strings.TrimSpace(event.ID) == "" {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败发生时间无效")
	}

	key := fmt.Sprintf("%s%s:account:%d", accountFailureStreakPrefix, source, accountID)
	lock := c.failureStreakKeyLock(key)
	lock.Lock()
	defer lock.Unlock()

	if outcome == service.AccountFailureStreakOutcomeIncrement {
		if pendingRaw, ok := c.pendingResets.Load(key); ok {
			pending, valid := pendingRaw.(accountFailurePendingReset)
			if !valid {
				c.pendingResets.Delete(key)
				return service.AccountFailureStreakState{}, errors.New("账号连续失败待补偿重置类型无效")
			}
			if pending.policy.Revision < policy.Revision {
				c.pendingResets.Delete(key)
			} else {
				state, err := c.applyOutcome(ctx, key, pending.policy, service.AccountFailureStreakOutcomeReset, pending.event)
				if err != nil {
					return service.AccountFailureStreakState{}, fmt.Errorf("补偿账号连续失败重置: %w", err)
				}
				if !state.Applied && state.PolicyRevision == pending.policy.Revision {
					return service.AccountFailureStreakState{}, errors.New("补偿账号连续失败重置事件已落后，拒绝使用旧计数")
				}
				c.pendingResets.Delete(key)
			}
		}
	}

	state, err := c.applyOutcome(ctx, key, policy, outcome, event)
	if outcome == service.AccountFailureStreakOutcomeReset {
		if err != nil {
			c.pendingResets.Store(key, accountFailurePendingReset{policy: policy, event: event})
		} else {
			c.pendingResets.Delete(key)
		}
	}
	return state, err
}

func (c *accountFailureStreakCache) failureStreakKeyLock(key string) *sync.Mutex {
	lock, _ := c.keyLocks.LoadOrStore(key, &sync.Mutex{})
	mutex, ok := lock.(*sync.Mutex)
	if !ok {
		panic("账号连续失败锁类型无效")
	}
	return mutex
}

func (c *accountFailureStreakCache) applyOutcome(
	ctx context.Context,
	key string,
	policy service.AccountFailureStreakPolicy,
	outcome service.AccountFailureStreakOutcome,
	event service.AccountFailureStreakEvent,
) (service.AccountFailureStreakState, error) {
	occurredAt := event.OccurredAt.UTC()
	result, err := accountFailureStreakApplyScript.Run(
		ctx,
		c.rdb,
		[]string{key, key + ":events"},
		policy.Revision,
		policy.Fingerprint,
		string(outcome),
		occurredAt.Unix(),
		occurredAt.Nanosecond(),
		strings.TrimSpace(event.ID),
	).Slice()
	if err != nil {
		return service.AccountFailureStreakState{}, fmt.Errorf("更新账号连续失败次数: %w", err)
	}
	if len(result) != 3 {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败缓存返回值无效")
	}
	count, ok := result[0].(int64)
	if !ok {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败计数返回值无效")
	}
	applied, ok := result[1].(int64)
	if !ok {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败应用状态返回值无效")
	}
	revision, ok := result[2].(int64)
	if !ok {
		return service.AccountFailureStreakState{}, errors.New("账号连续失败策略代次返回值无效")
	}
	return service.AccountFailureStreakState{Count: count, Applied: applied == 1, PolicyRevision: revision}, nil
}
