package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const LeaseInterval = 20 * time.Second
const LeaseTTL = 60 * time.Second

type Instance struct {
	ID      string `json:"instance_id"`
	Socket  string `json:"socket"`
	Version string `json:"version"`
}

// Registry 将“见过实例”与“实例仍存活”分开；未知旧实例不判死。
type Registry struct{ Redis *redis.Client }

func (r Registry) Publish(ctx context.Context, instance Instance) error {
	raw, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	pipe := r.Redis.TxPipeline()
	pipe.Set(ctx, "lifecycle:known:"+instance.ID, raw, 0)
	pipe.Set(ctx, "lifecycle:live:"+instance.ID, instance.ID, LeaseTTL)
	_, err = pipe.Exec(ctx)
	return err
}
func (r Registry) Dead(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	known, err := r.Redis.Exists(ctx, "lifecycle:known:"+id).Result()
	if err != nil || known == 0 {
		return false, err
	}
	live, err := r.Redis.Exists(ctx, "lifecycle:live:"+id).Result()
	return err == nil && live == 0, err
}
func (r Registry) Lookup(ctx context.Context, id string) (Instance, error) {
	var result Instance
	raw, err := r.Redis.Get(ctx, "lifecycle:known:"+id).Bytes()
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	if result.ID != id {
		return result, errors.New("实例登记不匹配")
	}
	live, err := r.Redis.Exists(ctx, "lifecycle:live:"+id).Result()
	if err != nil {
		return result, err
	}
	if live != 1 {
		return result, errors.New("实例租约不可确认")
	}
	return result, nil
}
func (r Registry) Heartbeat(ctx context.Context, instance Instance, failed func(error)) {
	ticker := time.NewTicker(LeaseInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			call, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := r.Publish(call, instance)
			cancel()
			failed(err)
		}
	}
}
