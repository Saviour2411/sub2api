package repository

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// 保留旧明细键；全局模式由蓝绿部署在旧实例退出后切换，缺失时采用兼容统计。
const billingInflightAccountingKey = "billing:inflight:accounting"

func billingInflightKeys(userID int64) (string, string) {
	return fmt.Sprintf("billing:inflight:{%d}", userID), fmt.Sprintf("billing:inflight_amt:{%d}", userID)
}

func billingInflightScopeKeys(scope string) (string, string) {
	return "billing:inflight_scope:{" + scope + "}", "billing:inflight_scope_amt:{" + scope + "}"
}

//go:embed billing_inflight.lua
var billingInflightLua string

var billingInflightScript = redis.NewScript(billingInflightLua)

func (c *billingCache) inflightOperation(ctx context.Context, zkey, hkey, op, id string, amount, balance float64, ttl time.Duration) (int64, float64, error) {
	keys := []string{zkey, hkey, zkey + ":total", billingInflightAccountingKey, zkey + ":done:" + id, zkey + ":prior"}
	for {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		result, err := billingInflightScript.Run(ctx, c.rdb, keys, op, id,
			strconv.FormatFloat(amount, 'f', -1, 64), strconv.FormatFloat(balance, 'f', -1, 64),
			max(ttl.Milliseconds(), 1)).Slice()
		if err != nil {
			return 0, 0, err
		}
		if len(result) != 2 {
			return 0, 0, fmt.Errorf("预留操作 %s 返回格式异常", op)
		}
		status, ok := result[0].(int64)
		if !ok {
			return 0, 0, fmt.Errorf("预留操作 %s 返回状态异常", op)
		}
		if status == 2 {
			// 每个脚本最多清理 128 条，沿用原预算并让出 Redis 执行机会。
			continue
		}
		text, ok := result[1].(string)
		if !ok {
			return 0, 0, fmt.Errorf("预留操作 %s 返回金额异常", op)
		}
		sum, err := strconv.ParseFloat(text, 64)
		return status, sum, err
	}
}

func (c *billingCache) ReserveInflightBalance(ctx context.Context, userID int64, id string, amount, balance float64, ttl time.Duration) (bool, float64, error) {
	zkey, hkey := billingInflightKeys(userID)
	status, sum, err := c.inflightOperation(ctx, zkey, hkey, "balance", id, amount, balance, ttl)
	return status == 1, sum, err
}

func (c *billingCache) ReleaseInflightBalance(ctx context.Context, userID int64, id string) error {
	zkey, hkey := billingInflightKeys(userID)
	_, _, err := c.inflightOperation(ctx, zkey, hkey, "release", id, 0, 0, 0)
	return err
}

func (c *billingCache) RenewInflightBalance(ctx context.Context, userID int64, id string, ttl time.Duration) (bool, error) {
	zkey, hkey := billingInflightKeys(userID)
	status, _, err := c.inflightOperation(ctx, zkey, hkey, "renew", id, 0, 0, ttl)
	return status == 1, err
}

func (c *billingCache) ReserveInflightScope(ctx context.Context, scope, id string, amount float64, ttl time.Duration) (float64, error) {
	zkey, hkey := billingInflightScopeKeys(scope)
	_, sum, err := c.inflightOperation(ctx, zkey, hkey, "scope", id, amount, 0, ttl)
	return sum, err
}

func (c *billingCache) ReleaseInflightScope(ctx context.Context, scope, id string) error {
	zkey, hkey := billingInflightScopeKeys(scope)
	_, _, err := c.inflightOperation(ctx, zkey, hkey, "release", id, 0, 0, 0)
	return err
}

var (
	_ service.InflightBalanceReservationCache   = (*billingCache)(nil)
	_ service.InflightBalanceReservationRenewer = (*billingCache)(nil)
	_ service.InflightScopeReservationCache     = (*billingCache)(nil)
)
