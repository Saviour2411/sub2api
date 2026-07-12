package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPersistFirstTokenTimeoutStateAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	nextRunAt := time.Date(2026, 7, 12, 8, 30, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduled_test_plans")).
		WithArgs(int64(42), nextRunAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repo.PersistFirstTokenTimeoutState(context.Background(), 42, map[string]any{
		"source": "first_token_timeout",
	}, nextRunAt)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPersistFirstTokenTimeoutStateRollsBackTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduled_test_plans")).
		WithArgs(int64(42), sqlmock.AnyArg()).
		WillReturnError(context.DeadlineExceeded)
	mock.ExpectRollback()

	err = repo.PersistFirstTokenTimeoutState(context.Background(), 42, map[string]any{
		"source": "first_token_timeout",
	}, time.Now())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPersistFailureSchedulingStateDoesNotOverwriteExistingIncident(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	created, err := repo.PersistFailureSchedulingState(context.Background(), 42, map[string]any{
		"incident_id": "incident-new",
	}, time.Now())
	require.NoError(t, err)
	require.False(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecoverFailureSchedulingStateUsesIncidentCAS(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(int64(42), "incident-current").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE scheduled_test_plans")).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	recovered, err := repo.RecoverFailureSchedulingState(context.Background(), 42, "incident-current")
	require.NoError(t, err)
	require.True(t, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecoverFailureSchedulingStateRejectsStaleIncident(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(int64(42), "incident-old").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	recovered, err := repo.RecoverFailureSchedulingState(context.Background(), 42, "incident-old")
	require.NoError(t, err)
	require.False(t, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClearFailureSchedulingStateDisablesAutoManagedPlanAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(int64(42), "incident-clear").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE scheduled_test_plans")).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	cleared, err := repo.ClearFailureSchedulingState(context.Background(), 42, "incident-clear")

	require.NoError(t, err)
	require.True(t, cleared)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClearFailureSchedulingStateRejectsConcurrentIncident(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(int64(42), "incident-old").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	cleared, err := repo.ClearFailureSchedulingState(context.Background(), 42, "incident-old")

	require.NoError(t, err)
	require.False(t, cleared)
	require.NoError(t, mock.ExpectationsWereMet())
}
