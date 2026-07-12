package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type imageGroupSuccessRateRepository struct {
	db *sql.DB
}

func NewImageGroupSuccessRateRepository(db *sql.DB) service.ImageGroupSuccessRateRepository {
	return &imageGroupSuccessRateRepository{db: db}
}

func (r *imageGroupSuccessRateRepository) Record(ctx context.Context, groupID, successCount, failureCount int64, occurredAt time.Time) error {
	return upsertImageGroupSuccessRate(ctx, r.db, 0, groupID, successCount, failureCount, occurredAt)
}

func (r *imageGroupSuccessRateRepository) RecordOnce(ctx context.Context, eventKey string, groupID, successCount, failureCount int64, occurredAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var generation int64
	err = tx.QueryRowContext(ctx, `
SELECT state.generation
FROM image_group_success_rate_state AS state
JOIN groups AS groups ON groups.id = $1
WHERE state.id = 1
  AND groups.status = 'active'
  AND groups.deleted_at IS NULL
  AND groups.name ILIKE '%image%'`, groupID).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO image_group_success_rate_events (generation, event_key, group_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (event_key) DO NOTHING`, generation, eventKey, groupID, occurredAt)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return tx.Commit()
	}
	if err := upsertImageGroupSuccessRate(ctx, tx, generation, groupID, successCount, failureCount, occurredAt); err != nil {
		return err
	}
	return tx.Commit()
}

type imageGroupSuccessRateExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func upsertImageGroupSuccessRate(
	ctx context.Context,
	executor imageGroupSuccessRateExecutor,
	generation, groupID, successCount, failureCount int64,
	occurredAt time.Time,
) error {
	var lastSuccessAt any
	if successCount > 0 {
		lastSuccessAt = occurredAt
	}
	_, err := executor.ExecContext(ctx, `
INSERT INTO image_group_success_rate_stats (
    generation, group_id, request_count, failure_count, last_success_at, created_at, updated_at
)
SELECT
    CASE WHEN $1 > 0 THEN $1 ELSE state.generation END,
    groups.id,
    $3 + $4,
    $4,
    $5,
    $6,
    $6
FROM image_group_success_rate_state AS state
JOIN groups AS groups ON groups.id = $2
WHERE state.id = 1
  AND groups.status = 'active'
  AND groups.deleted_at IS NULL
  AND groups.name ILIKE '%image%'
ON CONFLICT (generation, group_id) DO UPDATE SET
    request_count = image_group_success_rate_stats.request_count + EXCLUDED.request_count,
    failure_count = image_group_success_rate_stats.failure_count + EXCLUDED.failure_count,
    last_success_at = CASE
        WHEN EXCLUDED.last_success_at IS NULL THEN image_group_success_rate_stats.last_success_at
        WHEN image_group_success_rate_stats.last_success_at IS NULL THEN EXCLUDED.last_success_at
        ELSE GREATEST(image_group_success_rate_stats.last_success_at, EXCLUDED.last_success_at)
    END,
    updated_at = GREATEST(image_group_success_rate_stats.updated_at, EXCLUDED.updated_at)`, generation, groupID, successCount, failureCount, lastSuccessAt, occurredAt)
	return err
}

func (r *imageGroupSuccessRateRepository) ListCurrent(ctx context.Context) ([]service.ImageGroupSuccessRateAggregate, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    groups.id,
    groups.name,
    COALESCE(stats.request_count, 0),
    COALESCE(stats.failure_count, 0),
    stats.last_success_at
FROM groups
CROSS JOIN image_group_success_rate_state AS state
LEFT JOIN image_group_success_rate_stats AS stats
  ON stats.generation = state.generation
 AND stats.group_id = groups.id
WHERE state.id = 1
  AND groups.status = 'active'
  AND groups.deleted_at IS NULL
  AND groups.name ILIKE '%image%'
ORDER BY groups.sort_order ASC, groups.id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.ImageGroupSuccessRateAggregate, 0)
	for rows.Next() {
		var item service.ImageGroupSuccessRateAggregate
		var lastSuccessAt sql.NullTime
		if err := rows.Scan(&item.GroupID, &item.GroupName, &item.RequestCount, &item.FailureCount, &lastSuccessAt); err != nil {
			return nil, err
		}
		if lastSuccessAt.Valid {
			value := lastSuccessAt.Time
			item.LastSuccessAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *imageGroupSuccessRateRepository) Reset(ctx context.Context) (time.Time, error) {
	var resetAt time.Time
	err := r.db.QueryRowContext(ctx, `
UPDATE image_group_success_rate_state
SET generation = generation + 1,
    reset_at = NOW()
WHERE id = 1
RETURNING reset_at`).Scan(&resetAt)
	return resetAt, err
}
