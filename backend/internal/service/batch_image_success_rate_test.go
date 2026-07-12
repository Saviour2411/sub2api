package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

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
