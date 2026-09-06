package repository

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// SetAdminSchedulable 将人工开关和自动测活状态置于账号行锁保护下。
func (r *accountRepository) SetAdminSchedulable(ctx context.Context, id int64, schedulable bool) error {
	beginner, ok := r.sql.(sqlTxBeginner)
	if !ok {
		return errors.New("账号仓储不支持事务")
	}
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		UPDATE accounts SET schedulable = $2,
			extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object('manual_scheduling_paused', NOT $2::boolean),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, schedulable)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrAccountNotFound
	}
	if !schedulable {
		if _, err := tx.ExecContext(ctx, `UPDATE scheduled_test_plans
			SET enabled = false, next_run_at = NULL, updated_at = NOW()
			WHERE account_id = $1 AND auto_managed = true`, id); err != nil {
			return err
		}
	}
	if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.syncSchedulerAccountSnapshot(ctx, id)
	return nil
}
