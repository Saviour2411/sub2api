package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateBatchImageJobWithSQLBindsAllColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	expectedErr := errors.New("停止于批量图片任务插入")
	args := make([]driver.Value, 38)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	mock.ExpectQuery(`INSERT INTO batch_image_jobs \(.*group_id.*session_id, output_expires_at.*\$37, \$38.*RETURNING`).
		WithArgs(args...).
		WillReturnError(expectedErr)

	repo := &batchImageRepository{db: db, sql: db}
	groupID := int64(11)
	sessionID := "session-1"
	_, err = repo.CreateBatchImageJob(context.Background(), service.CreateBatchImageJobParams{
		BatchID:   "batch-1",
		UserID:    7,
		GroupID:   &groupID,
		Provider:  service.BatchImageProviderGeminiAPI,
		Model:     "gemini-2.5-flash-image",
		SessionID: &sessionID,
		ItemCount: 1,
	})
	require.ErrorIs(t, err, expectedErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
