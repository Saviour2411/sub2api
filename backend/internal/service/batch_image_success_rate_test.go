package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type successRateBatchItemRepo struct {
	BatchImageRepository
	items []*BatchImageItem
	err   error
	calls int
}

func (r *successRateBatchItemRepo) ListBatchImageItems(_ context.Context, _ string, filter BatchImageItemFilter) ([]*BatchImageItem, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if filter.Offset >= len(r.items) {
		return nil, nil
	}
	return r.items[filter.Offset:min(filter.Offset+filter.Limit, len(r.items))], nil
}

func TestBatchImagePipelineProcessorRecordSuccessRate排除逐项业务拒绝(t *testing.T) {
	for _, tt := range []struct {
		name      string
		allErrors bool
		listError bool
	}{
		{name: "混合结果"},
		{name: "全部为业务拒绝", allErrors: true},
		{name: "读取失败允许重试", listError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &imageGroupSuccessRateRepoStub{}
			items := &successRateBatchItemRepo{items: []*BatchImageItem{
				{ErrorCode: batchImageStringPtr("SAFETY_BLOCKED")},
				{ErrorCode: batchImageStringPtr("INVALID_ARGUMENT")},
				{ErrorCode: batchImageStringPtr("INSUFFICIENT_BALANCE")},
			}}
			groupID := int64(21)
			job := &BatchImageJob{BatchID: "imgbatch_filtered", GroupID: &groupID, Status: BatchImageJobStatusSettling, FailCount: 3}
			if !tt.allErrors {
				job.SuccessCount, job.FailCount = 2, 4
				items.items = append(items.items, &BatchImageItem{ErrorCode: batchImageStringPtr("INVALID_ARGUMENT"), ErrorMessage: batchImageStringPtr("生图任务超时")})
			}
			if tt.listError {
				items.err = errors.New("数据库读取失败")
			}
			processor := &BatchImagePipelineProcessor{
				SuccessRates: NewImageGroupSuccessRateService(repo), ProviderProcessor: &BatchImageProviderProcessor{Repo: items},
			}
			err := processor.recordSuccessRate(context.Background(), job)
			if tt.listError {
				require.ErrorIs(t, err, items.err)
				require.Empty(t, repo.records)
				return
			}
			require.NoError(t, err)
			if tt.allErrors {
				require.Empty(t, repo.records)
			} else {
				require.Equal(t, []imageGroupSuccessRateRecord{{eventKey: "batch:imgbatch_filtered", groupID: 21, successes: 2, failures: 1, idempotent: true}}, repo.records)
			}
		})
	}
}

func TestBatchImagePipelineProcessorRecordSuccessRate遍历失败分页(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	items := &successRateBatchItemRepo{}
	for range 500 {
		items.items = append(items.items, &BatchImageItem{ErrorCode: batchImageStringPtr("SAFETY_BLOCKED")})
	}
	items.items = append(items.items, &BatchImageItem{ErrorCode: batchImageStringPtr("PROVIDER_ITEM_FAILED")})
	groupID := int64(22)
	processor := &BatchImagePipelineProcessor{SuccessRates: NewImageGroupSuccessRateService(repo), ProviderProcessor: &BatchImageProviderProcessor{Repo: items}}
	job := &BatchImageJob{BatchID: "imgbatch_pages", GroupID: &groupID, Status: BatchImageJobStatusSettling, FailCount: 501}
	require.NoError(t, processor.recordSuccessRate(context.Background(), job))
	require.Equal(t, 2, items.calls)
	require.Equal(t, []imageGroupSuccessRateRecord{{eventKey: "batch:imgbatch_pages", groupID: 22, failures: 1, idempotent: true}}, repo.records)
}

func TestBatchImagePipelineProcessorRecordSuccessRate整批业务拒绝不统计(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	groupID := int64(23)
	processor := &BatchImagePipelineProcessor{SuccessRates: NewImageGroupSuccessRateService(repo)}
	job := &BatchImageJob{BatchID: "imgbatch_balance", GroupID: &groupID, Status: BatchImageJobStatusFailed, LastErrorCode: batchImageStringPtr("INSUFFICIENT_BALANCE")}
	require.NoError(t, processor.recordSuccessRate(context.Background(), job))
	require.Empty(t, repo.records)
}

func TestBatchImagePublicServiceRecordFailedBatchResult排除业务拒绝(t *testing.T) {
	for _, tt := range []struct {
		name     string
		cause    error
		excluded bool
	}{
		{name: "余额预扣失败", cause: ErrBatchImageInsufficientBalance, excluded: true},
		{name: "包装后的上游403", cause: fmt.Errorf("提交失败: %w", &GeminiAPIError{StatusCode: 403}), excluded: true},
		{name: "Vertex业务拒绝", cause: &VertexAPIError{StatusCode: 400, Code: "INVALID_ARGUMENT"}, excluded: true},
		{name: "上游审核拒绝", cause: &GeminiAPIError{StatusCode: 400, Message: "blocked by content policy"}, excluded: true},
		{name: "上游400超时", cause: &GeminiAPIError{StatusCode: 400, Message: "生图任务超时"}},
		{name: "上游429限流", cause: &VertexAPIError{StatusCode: 429, Code: "RESOURCE_EXHAUSTED"}},
		{name: "数据库失败", cause: errors.New("database connection reset")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &imageGroupSuccessRateRepoStub{}
			groupID := int64(24)
			svc := &BatchImagePublicService{SuccessRates: NewImageGroupSuccessRateService(repo)}
			svc.recordFailedBatchResult(context.Background(), &BatchImageJob{BatchID: "imgbatch_submit", GroupID: &groupID}, tt.cause)
			if tt.excluded {
				require.Empty(t, repo.records)
			} else {
				require.Equal(t, []imageGroupSuccessRateRecord{{eventKey: "batch:imgbatch_submit", groupID: 24, failures: 1, idempotent: true}}, repo.records)
			}
		})
	}
}

func TestBatchImagePipelineProcessorRecordSuccessRate按逐项结果统计(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	groupID := int64(17)
	processor := &BatchImagePipelineProcessor{SuccessRates: NewImageGroupSuccessRateService(repo)}
	job := &BatchImageJob{
		BatchID:      "imgbatch_items",
		GroupID:      &groupID,
		Status:       BatchImageJobStatusSettling,
		SuccessCount: 3,
		FailCount:    2,
	}

	err := processor.recordSuccessRate(context.Background(), job)

	require.NoError(t, err)
	require.Equal(t, []imageGroupSuccessRateRecord{{
		eventKey: "batch:imgbatch_items", groupID: 17, successes: 3, failures: 2, idempotent: true,
	}}, repo.records)
}

func TestBatchImagePipelineProcessorRecordSuccessRate整批失败只统计一次(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	groupID := int64(18)
	processor := &BatchImagePipelineProcessor{SuccessRates: NewImageGroupSuccessRateService(repo)}
	job := &BatchImageJob{BatchID: "imgbatch_failed", GroupID: &groupID, Status: BatchImageJobStatusFailed}

	err := processor.recordSuccessRate(context.Background(), job)

	require.NoError(t, err)
	require.Equal(t, []imageGroupSuccessRateRecord{{
		eventKey: "batch:imgbatch_failed", groupID: 18, failures: 1, idempotent: true,
	}}, repo.records)
}

func TestBatchImagePipelineProcessorRecordSuccessRate取消不统计(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	groupID := int64(19)
	processor := &BatchImagePipelineProcessor{SuccessRates: NewImageGroupSuccessRateService(repo)}
	job := &BatchImageJob{BatchID: "imgbatch_cancelled", GroupID: &groupID, Status: BatchImageJobStatusCancelled}

	err := processor.recordSuccessRate(context.Background(), job)

	require.NoError(t, err)
	require.Empty(t, repo.records)
}

func TestBatchImagePipelineProcessorRecordSuccessRate取消批次保留明确结果(t *testing.T) {
	repo := &imageGroupSuccessRateRepoStub{}
	groupID := int64(20)
	processor := &BatchImagePipelineProcessor{SuccessRates: NewImageGroupSuccessRateService(repo)}
	job := &BatchImageJob{
		BatchID:        "imgbatch_partial_cancelled",
		GroupID:        &groupID,
		Status:         BatchImageJobStatusCancelled,
		SuccessCount:   2,
		FailCount:      1,
		CancelledCount: 4,
	}

	err := processor.recordSuccessRate(context.Background(), job)

	require.NoError(t, err)
	require.Equal(t, []imageGroupSuccessRateRecord{{
		eventKey: "batch:imgbatch_partial_cancelled", groupID: 20, successes: 2, failures: 1, idempotent: true,
	}}, repo.records)
}
