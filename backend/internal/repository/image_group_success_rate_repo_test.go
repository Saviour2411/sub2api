package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestImageGroupSuccessRateRepositoryRecordUsesAtomicUpsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	occurredAt := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	mock.ExpectExec("INSERT INTO image_group_success_rate_stats").
		WithArgs(int64(0), int64(7), int64(1), int64(0), occurredAt, occurredAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = NewImageGroupSuccessRateRepository(db).Record(context.Background(), 7, 1, 0, occurredAt)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGroupSuccessRateRepositoryRecordOnceSkipsDuplicateBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	occurredAt := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT state.generation").
		WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"generation"}).AddRow(int64(4)))
	mock.ExpectExec("INSERT INTO image_group_success_rate_events").
		WithArgs(int64(4), "batch:abc", int64(8), occurredAt).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err = NewImageGroupSuccessRateRepository(db).RecordOnce(context.Background(), "batch:abc", 8, 3, 1, occurredAt)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGroupSuccessRateRepositoryListIncludesUnsampledImageGroups(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	lastSuccessAt := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	mock.ExpectQuery("SELECT[[:space:]]+groups.id").WillReturnRows(sqlmock.NewRows([]string{
		"id", "name", "request_count", "failure_count", "last_success_at",
	}).AddRow(int64(2), "Image A", int64(0), int64(0), nil).
		AddRow(int64(3), "image B", int64(5), int64(1), lastSuccessAt))

	items, err := NewImageGroupSuccessRateRepository(db).ListCurrent(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, int64(0), items[0].RequestCount)
	require.Nil(t, items[0].LastSuccessAt)
	require.Equal(t, &lastSuccessAt, items[1].LastSuccessAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGroupSuccessRateRepositoryResetSwitchesGeneration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	resetAt := time.Date(2026, 7, 12, 1, 2, 3, 0, time.UTC)
	mock.ExpectQuery("UPDATE image_group_success_rate_state").
		WillReturnRows(sqlmock.NewRows([]string{"reset_at"}).AddRow(resetAt))

	got, err := NewImageGroupSuccessRateRepository(db).Reset(context.Background())

	require.NoError(t, err)
	require.Equal(t, resetAt, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
