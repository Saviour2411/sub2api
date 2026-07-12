//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImageGroupSuccessRateRepositoryRecordAndRecordOnceIntegration(t *testing.T) {
	ctx := context.Background()
	group := mustCreateGroup(t, testEntClient(t), &service.Group{
		Name:           fmt.Sprintf("Image rate integration %d", time.Now().UnixNano()),
		Status:         service.StatusActive,
		RateMultiplier: 1,
	})
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), `DELETE FROM groups WHERE id = $1`, group.ID)
		require.NoError(t, err)
	})

	var generation int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT generation
FROM image_group_success_rate_state
WHERE id = 1`).Scan(&generation))

	repo := NewImageGroupSuccessRateRepository(integrationDB)
	successAt := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	failureAt := successAt.Add(time.Minute)
	batchAt := successAt.Add(2 * time.Minute)
	eventKey := fmt.Sprintf("integration:image-rate:%d", group.ID)

	require.NoError(t, repo.Record(ctx, group.ID, 1, 0, successAt))
	require.NoError(t, repo.Record(ctx, group.ID, 0, 1, failureAt))
	assertImageGroupSuccessRateStats(t, ctx, generation, group.ID, 2, 1, successAt)

	require.NoError(t, repo.RecordOnce(ctx, eventKey, group.ID, 2, 1, batchAt))
	require.NoError(t, repo.RecordOnce(ctx, eventKey, group.ID, 9, 9, batchAt.Add(time.Hour)))
	assertImageGroupSuccessRateStats(t, ctx, generation, group.ID, 5, 2, batchAt)

	var eventCount int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM image_group_success_rate_events
WHERE event_key = $1`, eventKey).Scan(&eventCount))
	require.Equal(t, int64(1), eventCount)
}

func assertImageGroupSuccessRateStats(
	t *testing.T,
	ctx context.Context,
	generation, groupID, requestCount, failureCount int64,
	lastSuccessAt time.Time,
) {
	t.Helper()

	var actualRequestCount int64
	var actualFailureCount int64
	var actualLastSuccessAt sql.NullTime
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT request_count, failure_count, last_success_at
FROM image_group_success_rate_stats
WHERE generation = $1 AND group_id = $2`, generation, groupID).
		Scan(&actualRequestCount, &actualFailureCount, &actualLastSuccessAt))
	require.Equal(t, requestCount, actualRequestCount)
	require.Equal(t, failureCount, actualFailureCount)
	require.True(t, actualLastSuccessAt.Valid)
	require.True(t, actualLastSuccessAt.Time.Equal(lastSuccessAt))
}
