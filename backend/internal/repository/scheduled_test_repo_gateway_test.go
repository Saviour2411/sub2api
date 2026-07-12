package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestRescheduleEnabledAutoManagedKeepsPendingPlanWithoutFailures(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	pendingAt := now.Add(30 * time.Second)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT p\\.id").WillReturnRows(
		sqlmock.NewRows([]string{"id", "last_run_at", "next_run_at", "consecutive_failures"}).
			AddRow(int64(7), nil, pendingAt, 0),
	)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE scheduled_test_plans")).
		WithArgs(int64(7), pendingAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &scheduledTestPlanRepository{db: db}
	err = repo.RescheduleEnabledAutoManaged(context.Background(), []time.Duration{time.Minute, 5 * time.Minute}, now)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRescheduleEnabledAutoManagedUsesFailureStepFromLastRun(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	lastRunAt := now.Add(-2 * time.Minute)
	wantNextRunAt := lastRunAt.Add(5 * time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT p\\.id").WillReturnRows(
		sqlmock.NewRows([]string{"id", "last_run_at", "next_run_at", "consecutive_failures"}).
			AddRow(int64(8), lastRunAt, now.Add(time.Minute), 2),
	)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE scheduled_test_plans")).
		WithArgs(int64(8), wantNextRunAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &scheduledTestPlanRepository{db: db}
	err = repo.RescheduleEnabledAutoManaged(context.Background(), []time.Duration{time.Minute, 5 * time.Minute}, now)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
