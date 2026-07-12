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
