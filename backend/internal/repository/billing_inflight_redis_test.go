package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// 此入口只用于显式指定的隔离 Redis，不复用生产连接配置。
func inflightRealRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("INFLIGHT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("未指定隔离 Redis")
	}
	host, _, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	require.True(t, host == "127.0.0.1" || host == "localhost" || host == "::1", "性能测试仅允许本机隔离 Redis")
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15, PoolSize: 64, ContextTimeoutEnabled: true})
	ctx := context.Background()
	require.NoError(t, rdb.Ping(ctx).Err())
	previous, err := rdb.HGetAll(ctx, billingInflightAccountingKey).Result()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rdb.Del(ctx, billingInflightAccountingKey).Err()
		if len(previous) > 0 {
			_ = rdb.HSet(ctx, billingInflightAccountingKey, previous).Err()
		}
		_ = rdb.Close()
	})
	return rdb
}

func TestInflightRedisHotspotPerformance(t *testing.T) {
	rdb := inflightRealRedis(t)
	ctx := context.Background()
	require.NoError(t, rdb.ConfigSet(ctx, "slowlog-log-slower-than", "0").Err())
	require.NoError(t, rdb.ConfigSet(ctx, "slowlog-max-len", "131072").Err())
	legacy := redis.NewScript(legacyInflightLua)
	for _, size := range []int{6500, 20000} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			uid := time.Now().UnixMilli()
			z, h := billingInflightKeys(uid)
			cache := &billingCache{rdb: rdb}
			pipe := rdb.Pipeline()
			for i := 0; i < size; i++ {
				id := fmt.Sprintf("seed-%d", i)
				pipe.ZAdd(ctx, z, redis.Z{Score: float64(time.Now().Add(time.Hour).UnixMilli()), Member: id})
				pipe.HSet(ctx, h, id, "0.25")
			}
			_, err := pipe.Exec(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { _ = rdb.Del(ctx, z, h, z+":total", z+":prior").Err() })
			measure := func(modern bool) (time.Duration, time.Duration) {
				mode := "compat"
				if modern {
					mode = "aggregate"
				}
				setInflightMode(t, cache, mode, uuid.NewString())
				slots := service.NewConcurrencyService(NewConcurrencyCache(rdb, 15, 900))
				defer slots.Stop()
				reserve := func(user int64, id string) error {
					if modern {
						ok, _, err := cache.ReserveInflightBalance(ctx, user, id, 0.001, 1e9, time.Hour)
						if err == nil && !ok {
							return fmt.Errorf("意外拒绝")
						}
						return err
					}
					zz, hh := billingInflightKeys(user)
					now := time.Now().UnixMilli()
					return legacy.Run(ctx, rdb, []string{zz, hh}, now, now+3600000, id, "0.001", "1000000000", 3600000).Err()
				}
				release := func(user int64, id string) error {
					if modern {
						return cache.ReleaseInflightBalance(ctx, user, id)
					}
					zz, hh := billingInflightKeys(user)
					return rdb.Eval(ctx, "redis.call('ZREM',KEYS[1],ARGV[1]);redis.call('HDEL',KEYS[2],ARGV[1]);return 1", []string{zz, hh}, id).Err()
				}
				warm := uuid.NewString()
				require.NoError(t, reserve(uid, warm))
				require.NoError(t, release(uid, warm))
				require.NoError(t, rdb.Do(ctx, "SLOWLOG", "RESET").Err())
				const requests = 2048
				errors := make(chan error, requests*2)
				latencies := make(chan time.Duration, requests)
				var wg sync.WaitGroup
				for worker := 0; worker < 16; worker++ {
					wg.Go(func() {
						for i := worker; i < requests; i += 16 {
							user := uid
							if i%5 == 0 {
								user += int64(1 + i%31)
							}
							id := uuid.NewString()
							if err := reserve(user, id); err != nil {
								errors <- err
								continue
							}
							started := time.Now()
							slot, err := slots.AcquireUserSlot(ctx, user, 32)
							latencies <- time.Since(started)
							if err != nil {
								errors <- err
							} else if !slot.Acquired {
								errors <- fmt.Errorf("未预期的并发拒绝")
							} else {
								slot.ReleaseFunc()
							}
							if err := release(user, id); err != nil {
								errors <- err
							}
						}
					})
				}
				wg.Wait()
				close(errors)
				close(latencies)
				for err := range errors {
					require.NoError(t, err)
				}
				var slotTimes []time.Duration
				for latency := range latencies {
					slotTimes = append(slotTimes, latency)
				}
				slices.Sort(slotTimes)
				require.Len(t, slotTimes, requests)
				slow, err := rdb.SlowLogGet(ctx, 131072).Result()
				require.NoError(t, err)
				sha := legacy.Hash()
				if modern {
					sha = billingInflightScript.Hash()
				}
				var scriptTimes []time.Duration
				for _, entry := range slow {
					args := entry.Args
					if len(args) > 3 && args[0] == "evalsha" && args[1] == sha && args[3] == z {
						if modern && (len(args) <= 9 || args[9] != "balance") {
							continue
						}
						scriptTimes = append(scriptTimes, entry.Duration)
					}
				}
				require.GreaterOrEqual(t, len(scriptTimes), 128, "慢日志必须保留足够的预留脚本样本")
				slices.Sort(scriptTimes)
				require.Equal(t, int64(size), rdb.ZCard(ctx, z).Val())
				amounts, err := rdb.HVals(ctx, h).Result()
				require.NoError(t, err)
				total := 0.0
				for _, amount := range amounts {
					n, err := strconv.ParseFloat(amount, 64)
					require.NoError(t, err)
					total += n
				}
				require.InDelta(t, float64(size)*0.25, total, 1e-6)
				if modern {
					stored, err := rdb.HGet(ctx, z+":total", "sum").Float64()
					require.NoError(t, err)
					require.InDelta(t, total, stored, 1e-6)
				}
				return scriptTimes[(len(scriptTimes)-1)*99/100], slotTimes[(len(slotTimes)-1)*99/100]
			}
			old, oldSlot := measure(false)
			current, currentSlot := measure(true)
			improvement := 1 - float64(current)/float64(old)
			t.Logf("预留=%d，旧脚本p99=%s，新脚本p99=%s，下降=%.2f%%；旧/新槽位p99=%s/%s；各2048混合请求，无重复占用", size, old, current, improvement*100, oldSlot, currentSlot)
			require.GreaterOrEqual(t, improvement, 0.80)
			require.Less(t, currentSlot, 2*time.Second)
		})
	}
}

func TestInflightRedisDeadlineAndUnknownSlotCleanup(t *testing.T) {
	rdb := inflightRealRedis(t)
	ctx := context.Background()
	cache, ok := NewConcurrencyCache(rdb, 15, 900).(*concurrencyCache)
	require.True(t, ok)
	svc := service.NewConcurrencyService(cache)
	t.Cleanup(svc.Stop)
	require.NoError(t, rdb.Do(ctx, "CLIENT", "PAUSE", 2400, "ALL").Err())
	started := time.Now()
	_, err := svc.AcquireUserSlot(ctx, 990001, 1)
	require.Error(t, err)
	require.Less(t, time.Since(started), 2500*time.Millisecond)
	var operation *service.SlotOperationError
	require.ErrorAs(t, err, &operation)
	require.Eventually(t, func() bool { return rdb.Exists(ctx, userSlotKey(990001)+":done:"+operation.SlotID).Val() == 1 }, 5*time.Second, 20*time.Millisecond)
	require.Zero(t, rdb.ZCard(ctx, userSlotKey(990001)).Val())
}

type loseScriptReplyConn struct {
	net.Conn
	armed *atomic.Bool
	lose  atomic.Bool
}

func (c *loseScriptReplyConn) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("\r\nevalsha\r\n")) && c.armed.Swap(false) {
		c.lose.Store(true)
	}
	return c.Conn.Write(data)
}
func (c *loseScriptReplyConn) Read(data []byte) (int, error) {
	n, err := c.Conn.Read(data)
	if n > 0 && c.lose.Swap(false) {
		_ = c.Close()
		return 0, io.ErrUnexpectedEOF
	}
	return n, err
}

func TestInflightRedisWrittenThenDisconnected(t *testing.T) {
	rdb := inflightRealRedis(t)
	ctx := context.Background()
	_, err := billingInflightScript.Load(ctx, rdb).Result()
	require.NoError(t, err)
	var armed atomic.Bool
	opts := *rdb.Options()
	opts.MaxRetries = -1
	opts.Dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &loseScriptReplyConn{Conn: conn, armed: &armed}, nil
	}
	lossy := redis.NewClient(&opts)
	t.Cleanup(func() { _ = lossy.Close() })
	cache := &billingCache{rdb: lossy}
	uid := time.Now().UnixMilli()
	require.NoError(t, cache.SetUserBalance(ctx, uid, 1))
	svc := billingAuditService(t, cache)
	armed.Store(true)
	res, err := svc.ReserveInflight(ctx, &service.User{ID: uid}, nil, nil, 0.6)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, armed.Load())
	z, _ := billingInflightKeys(uid)
	require.Equal(t, int64(1), rdb.ZCard(ctx, z).Val(), "服务返回结果未知，但真实 Redis 已写入")
	taskDone := res.Acquire()
	res.HandlerDone()
	require.Equal(t, int64(1), rdb.ZCard(ctx, z).Val())
	armed.Store(true)
	taskDone()
	require.False(t, armed.Load(), "释放响应也被断连丢弃")
	require.Eventually(t, func() bool { return rdb.ZCard(ctx, z).Val() == 0 }, time.Second, time.Millisecond)
	res.Release()
	require.Zero(t, rdb.ZCard(ctx, z).Val())
}

func TestInflightRedisLegacyAndTwoInstancesSwitchModes(t *testing.T) {
	rdb := inflightRealRedis(t)
	other := redis.NewClient(rdb.Options())
	t.Cleanup(func() { _ = other.Close() })
	a, b := &billingCache{rdb: rdb}, &billingCache{rdb: other}
	ctx := context.Background()
	uid := time.Now().UnixMilli()
	z, h := billingInflightKeys(uid)
	setInflightMode(t, a, "compat", uuid.NewString())
	now := time.Now().UnixMilli()
	require.NoError(t, redis.NewScript(legacyInflightLua).Run(ctx, rdb, []string{z, h}, now, now+60000, "legacy", 0.5, 1, 60000).Err())
	ok, sum, err := a.ReserveInflightBalance(ctx, uid, "modern", 0.25, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 0.5, sum, 1e-9)
	setInflightMode(t, a, "aggregate", uuid.NewString())
	ok, sum, err = b.ReserveInflightBalance(ctx, uid, "peer", 0.1, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 0.75, sum, 1e-9)
	setInflightMode(t, a, "compat", uuid.NewString())
	require.NoError(t, rdb.Eval(ctx, "redis.call('ZREM',KEYS[1],ARGV[1]);redis.call('HDEL',KEYS[2],ARGV[1]);return 1", []string{z, h}, "legacy").Err())
	setInflightMode(t, a, "aggregate", uuid.NewString())
	ok, sum, err = b.ReserveInflightBalance(ctx, uid, "again", 0.1, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.InDelta(t, 0.35, sum, 1e-9)
}
