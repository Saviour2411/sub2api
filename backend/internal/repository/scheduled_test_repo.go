package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// --- Plan Repository ---

type scheduledTestPlanRepository struct {
	db *sql.DB
}

func NewScheduledTestPlanRepository(db *sql.DB) service.ScheduledTestPlanRepository {
	return &scheduledTestPlanRepository{db: db}
}

func (r *scheduledTestPlanRepository) Create(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_plans (account_id, model_id, prompt, cron_expression, enabled, max_results, auto_recover, auto_managed, next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, account_id, model_id, prompt, cron_expression, enabled, max_results, auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
	`, plan.AccountID, plan.ModelID, plan.Prompt, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.AutoManaged, plan.NextRunAt)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) GetByID(ctx context.Context, id int64) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, model_id, prompt, cron_expression, enabled, max_results, auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE id = $1
	`, id)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) ListByAccountID(ctx context.Context, accountID int64) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, prompt, cron_expression, enabled, max_results, auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListDue(ctx context.Context, now time.Time) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, prompt, cron_expression, enabled, max_results, auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans
		WHERE enabled = true AND next_run_at <= $1
		ORDER BY next_run_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListAutoManagedActivationCandidates(ctx context.Context, now time.Time) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.account_id, p.model_id, p.prompt, p.cron_expression, p.enabled, p.max_results, p.auto_recover, p.auto_managed, p.last_run_at, p.next_run_at, p.created_at, p.updated_at
		FROM scheduled_test_plans p
		JOIN accounts a ON a.id = p.account_id AND a.deleted_at IS NULL
		WHERE p.auto_managed = true
		  AND p.enabled = false
		  AND (
		      a.status = 'error'
		      OR (
		          a.extra ? 'failure_strategy_unscheduled'
		          AND a.extra->'failure_strategy_unscheduled' IS NOT NULL
		          AND a.extra->'failure_strategy_unscheduled' <> 'null'::jsonb
		          AND a.extra->'failure_strategy_unscheduled' <> '{}'::jsonb
		      )
		      OR a.rate_limit_reset_at > $1
		      OR a.overload_until > $1
		      OR a.temp_unschedulable_until > $1
		  )
		ORDER BY p.id ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) Update(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET model_id = $2, prompt = $3, cron_expression = $4, enabled = $5, max_results = $6, auto_recover = $7, next_run_at = $8, updated_at = NOW()
		WHERE id = $1
		RETURNING id, account_id, model_id, prompt, cron_expression, enabled, max_results, auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
	`, plan.ID, plan.ModelID, plan.Prompt, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.NextRunAt)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id = $1`, id)
	return err
}

func (r *scheduledTestPlanRepository) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1
	`, id, lastRunAt, nextRunAt)
	return err
}

func (r *scheduledTestPlanRepository) EnableAutoManaged(ctx context.Context, id int64, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans
		SET enabled = true, next_run_at = $2, updated_at = NOW()
		WHERE id = $1 AND auto_managed = true
	`, id, nextRunAt)
	return err
}

func (r *scheduledTestPlanRepository) DisableAutoManaged(ctx context.Context, id int64, lastRunAt *time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans
		SET enabled = false,
			last_run_at = COALESCE($2, last_run_at),
			next_run_at = NULL,
			updated_at = NOW()
		WHERE id = $1 AND auto_managed = true
	`, id, lastRunAt)
	return err
}

// --- Result Repository ---

type scheduledTestResultRepository struct {
	db *sql.DB
}

func NewScheduledTestResultRepository(db *sql.DB) service.ScheduledTestResultRepository {
	return &scheduledTestResultRepository{db: db}
}

func (r *scheduledTestResultRepository) Create(ctx context.Context, result *service.ScheduledTestResult) (*service.ScheduledTestResult, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_results (plan_id, status, response_text, error_message, latency_ms, started_at, finished_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, plan_id, status, response_text, error_message, latency_ms, started_at, finished_at, created_at
	`, result.PlanID, result.Status, result.ResponseText, result.ErrorMessage, result.LatencyMs, result.StartedAt, result.FinishedAt)

	out := &service.ScheduledTestResult{}
	if err := row.Scan(
		&out.ID, &out.PlanID, &out.Status, &out.ResponseText, &out.ErrorMessage,
		&out.LatencyMs, &out.StartedAt, &out.FinishedAt, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *scheduledTestResultRepository) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*service.ScheduledTestResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_id, status, response_text, error_message, latency_ms, started_at, finished_at, created_at
		FROM scheduled_test_results
		WHERE plan_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, planID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*service.ScheduledTestResult
	for rows.Next() {
		r := &service.ScheduledTestResult{}
		if err := rows.Scan(
			&r.ID, &r.PlanID, &r.Status, &r.ResponseText, &r.ErrorMessage,
			&r.LatencyMs, &r.StartedAt, &r.FinishedAt, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *scheduledTestResultRepository) ListLatestFailuresByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*service.ScheduledTestLatestFailure, error) {
	result := make(map[int64]*service.ScheduledTestLatestFailure)
	if len(accountIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (p.account_id)
			p.account_id,
			p.id,
			p.model_id,
			r.id,
			r.error_message,
			r.started_at,
			r.finished_at,
			r.created_at
		FROM scheduled_test_results r
		JOIN scheduled_test_plans p ON p.id = r.plan_id
		WHERE p.account_id = ANY($1) AND r.status = 'failed'
		ORDER BY p.account_id, r.created_at DESC, r.id DESC
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		failure := &service.ScheduledTestLatestFailure{}
		if err := rows.Scan(
			&failure.AccountID, &failure.PlanID, &failure.ModelID, &failure.ResultID,
			&failure.ErrorMessage, &failure.StartedAt, &failure.FinishedAt, &failure.CreatedAt,
		); err != nil {
			return nil, err
		}
		result[failure.AccountID] = failure
	}
	return result, rows.Err()
}

func (r *scheduledTestResultRepository) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM scheduled_test_results
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY plan_id ORDER BY created_at DESC) AS rn
				FROM scheduled_test_results
				WHERE plan_id = $1
			) ranked
			WHERE rn > $2
		)
	`, planID, keepCount)
	return err
}

// --- scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanPlan(row scannable) (*service.ScheduledTestPlan, error) {
	p := &service.ScheduledTestPlan{}
	if err := row.Scan(
		&p.ID, &p.AccountID, &p.ModelID, &p.Prompt, &p.CronExpression, &p.Enabled, &p.MaxResults, &p.AutoRecover,
		&p.AutoManaged, &p.LastRunAt, &p.NextRunAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return p, nil
}

func scanPlans(rows *sql.Rows) ([]*service.ScheduledTestPlan, error) {
	var plans []*service.ScheduledTestPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
