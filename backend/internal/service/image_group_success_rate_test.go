package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageGroupSuccessRateRepoStub struct {
	aggregates []ImageGroupSuccessRateAggregate
	records    []imageGroupSuccessRateRecord
	resetAt    time.Time
}

type imageGroupSuccessRateRecord struct {
	eventKey   string
	groupID    int64
	successes  int64
	failures   int64
	idempotent bool
}

func (r *imageGroupSuccessRateRepoStub) Record(_ context.Context, groupID, successCount, failureCount int64, _ time.Time) error {
	r.records = append(r.records, imageGroupSuccessRateRecord{groupID: groupID, successes: successCount, failures: failureCount})
	return nil
}

func (r *imageGroupSuccessRateRepoStub) RecordOnce(_ context.Context, eventKey string, groupID, successCount, failureCount int64, _ time.Time) error {
	r.records = append(r.records, imageGroupSuccessRateRecord{eventKey: eventKey, groupID: groupID, successes: successCount, failures: failureCount, idempotent: true})
	return nil
}

func (r *imageGroupSuccessRateRepoStub) ListCurrent(context.Context) ([]ImageGroupSuccessRateAggregate, error) {
	return r.aggregates, nil
}

func (r *imageGroupSuccessRateRepoStub) Reset(context.Context) (time.Time, error) {
	return r.resetAt, nil
}

func TestImageGroupSuccessRateService_List计算成功率(t *testing.T) {
	now := time.Now()
	repo := &imageGroupSuccessRateRepoStub{aggregates: []ImageGroupSuccessRateAggregate{
		{GroupID: 1, GroupName: "Image A"},
		{GroupID: 2, GroupName: "image B", RequestCount: 3, FailureCount: 1, LastSuccessAt: &now},
		{GroupID: 3, GroupName: "IMAGE C", RequestCount: 2, FailureCount: 5},
	}}

	items, err := NewImageGroupSuccessRateService(repo).List(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, float64(100), items[0].SuccessRate)
	require.Equal(t, 66.67, items[1].SuccessRate)
	require.Equal(t, float64(0), items[2].SuccessRate)
	require.Equal(t, &now, items[1].LastSuccessAt)
}

func TestImageGroupSuccessRateService_RecordRequestResult(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	service := NewImageGroupSuccessRateService(repo)

	require.NoError(t, service.RecordRequestResult(context.Background(), 11, true))
	require.NoError(t, service.RecordRequestResult(context.Background(), 11, false))

	require.Equal(t, []imageGroupSuccessRateRecord{
		{groupID: 11, successes: 1},
		{groupID: 11, failures: 1},
	}, repo.records)
}

func TestImageGroupSuccessRateService_RecordBatchResult使用稳定幂等键(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	service := NewImageGroupSuccessRateService(repo)

	require.NoError(t, service.RecordBatchResult(context.Background(), 12, " batch_abc ", 4, 2))
	require.Equal(t, []imageGroupSuccessRateRecord{{
		eventKey: "batch:batch_abc", groupID: 12, successes: 4, failures: 2, idempotent: true,
	}}, repo.records)
}

func TestImageGroupSuccessRateService_Reset返回数据库时间(t *testing.T) {
	resetAt := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
	repo := &imageGroupSuccessRateRepoStub{resetAt: resetAt}

	got, err := NewImageGroupSuccessRateService(repo).Reset(context.Background())

	require.NoError(t, err)
	require.Equal(t, resetAt, got)
}
