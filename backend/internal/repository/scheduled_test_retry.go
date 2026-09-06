package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type autoManagedProbeState struct {
	PlanID    int64     `json:"plan_id"`
	Failures  int       `json:"failures"`
	LastRunAt time.Time `json:"last_run_at"`
	NextRunAt time.Time `json:"next_run_at"`
}

// ScheduleAutoManagedRetry 将退避计数与下次执行时间原子保存，结果保留数不再限制退避档位。
func (r *scheduledTestPlanRepository) ScheduleAutoManagedRetry(ctx context.Context, plan *service.ScheduledTestPlan, finishedAt time.Time, steps []time.Duration) error {
	if plan == nil || len(steps) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 与事故恢复、人工暂停保持相同锁顺序：先账号，再计划。
	var raw []byte
	var paused bool
	if err := tx.QueryRowContext(ctx, `SELECT extra->'auto_managed_probe_state',
		COALESCE(extra->>'manual_scheduling_paused', 'false') = 'true'
		FROM accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, plan.AccountID).Scan(&raw, &paused); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if paused {
		return nil
	}
	var enabled bool
	var last, next sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT enabled, last_run_at, next_run_at
		FROM scheduled_test_plans WHERE id = $1 AND account_id = $2 AND auto_managed = true FOR UPDATE`,
		plan.ID, plan.AccountID).Scan(&enabled, &last, &next); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	// 旧执行不能覆盖管理员修改或并发新事故重新安排的计划。
	if !enabled || !samePlanRunTime(last, plan.LastRunAt) || !samePlanRunTime(next, plan.NextRunAt) {
		return nil
	}
	var state autoManagedProbeState
	_ = json.Unmarshal(raw, &state)
	failures := 1
	if state.PlanID == plan.ID && state.Failures > 0 && last.Valid && next.Valid &&
		state.LastRunAt.Equal(last.Time) && state.NextRunAt.Equal(next.Time) {
		failures = state.Failures + 1
	} else {
		// 兼容升级前已有的计划；当前失败结果已经保存。
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM scheduled_test_results r
			WHERE r.plan_id = $1 AND r.status = 'failed' AND r.id > COALESCE((
				SELECT MAX(id) FROM scheduled_test_results WHERE plan_id = $1 AND status <> 'failed'
			), 0)`, plan.ID).Scan(&failures); err != nil {
			return err
		}
	}
	// 配置最多十档，保留足够计数并限制持久化值大小。
	failures = max(1, min(failures, 10))
	finishedAt = finishedAt.UTC().Truncate(time.Microsecond)
	nextRunAt := finishedAt.Add(steps[min(failures, len(steps))-1])
	payload, err := json.Marshal(autoManagedProbeState{plan.ID, failures, finishedAt, nextRunAt})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts
		SET extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object('auto_managed_probe_state', $2::jsonb)
		WHERE id = $1`, plan.AccountID, string(payload)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans
		SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1`, plan.ID, finishedAt, nextRunAt); err != nil {
		return err
	}
	return tx.Commit()
}

func samePlanRunTime(stored sql.NullTime, snapshot *time.Time) bool {
	if snapshot == nil {
		return !stored.Valid
	}
	return stored.Valid && stored.Time.Equal(snapshot.UTC().Truncate(time.Microsecond))
}
