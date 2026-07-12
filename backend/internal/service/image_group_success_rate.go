package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// ImageGroupSuccessRateAggregate 是仓储层返回的内部聚合数据，不可直接暴露给前端。
type ImageGroupSuccessRateAggregate struct {
	GroupID       int64
	GroupName     string
	RequestCount  int64
	FailureCount  int64
	LastSuccessAt *time.Time
}

// ImageGroupSuccessRateItem 是渠道状态接口允许返回的最小字段集合。
type ImageGroupSuccessRateItem struct {
	GroupID       int64
	GroupName     string
	SuccessRate   float64
	LastSuccessAt *time.Time
}

type ImageGroupSuccessRateRepository interface {
	Record(ctx context.Context, groupID, successCount, failureCount int64, occurredAt time.Time) error
	RecordOnce(ctx context.Context, eventKey string, groupID, successCount, failureCount int64, occurredAt time.Time) error
	ListCurrent(ctx context.Context) ([]ImageGroupSuccessRateAggregate, error)
	Reset(ctx context.Context) (time.Time, error)
}

type ImageGroupSuccessRateService struct {
	repo ImageGroupSuccessRateRepository
}

func NewImageGroupSuccessRateService(repo ImageGroupSuccessRateRepository) *ImageGroupSuccessRateService {
	return &ImageGroupSuccessRateService{repo: repo}
}

// RecordRequestResult 在整次用户请求得出最终结果后记录一次。
func (s *ImageGroupSuccessRateService) RecordRequestResult(ctx context.Context, groupID int64, succeeded bool) error {
	if s == nil || s.repo == nil || groupID <= 0 {
		return nil
	}
	successCount, failureCount := int64(0), int64(1)
	if succeeded {
		successCount, failureCount = 1, 0
	}
	if err := s.repo.Record(ctx, groupID, successCount, failureCount, time.Now()); err != nil {
		return fmt.Errorf("记录 Image 分组请求结果: %w", err)
	}
	return nil
}

// RecordBatchResult 使用批次 ID 做幂等键，Worker 重试不会重复累计。
func (s *ImageGroupSuccessRateService) RecordBatchResult(ctx context.Context, groupID int64, batchID string, successCount, failureCount int) error {
	if s == nil || s.repo == nil || groupID <= 0 {
		return nil
	}
	batchID = strings.TrimSpace(batchID)
	if batchID == "" || successCount < 0 || failureCount < 0 || successCount+failureCount == 0 {
		return nil
	}
	if err := s.repo.RecordOnce(
		ctx,
		"batch:"+batchID,
		groupID,
		int64(successCount),
		int64(failureCount),
		time.Now(),
	); err != nil {
		return fmt.Errorf("记录批量生图分组结果: %w", err)
	}
	return nil
}

func (s *ImageGroupSuccessRateService) List(ctx context.Context) ([]ImageGroupSuccessRateItem, error) {
	if s == nil || s.repo == nil {
		return []ImageGroupSuccessRateItem{}, nil
	}
	aggregates, err := s.repo.ListCurrent(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询 Image 分组成功率: %w", err)
	}
	items := make([]ImageGroupSuccessRateItem, 0, len(aggregates))
	for _, aggregate := range aggregates {
		items = append(items, ImageGroupSuccessRateItem{
			GroupID:       aggregate.GroupID,
			GroupName:     aggregate.GroupName,
			SuccessRate:   calculateImageGroupSuccessRate(aggregate.RequestCount, aggregate.FailureCount),
			LastSuccessAt: aggregate.LastSuccessAt,
		})
	}
	return items, nil
}

func (s *ImageGroupSuccessRateService) Reset(ctx context.Context) (time.Time, error) {
	if s == nil || s.repo == nil {
		return time.Time{}, fmt.Errorf("Image 分组成功率服务未配置")
	}
	resetAt, err := s.repo.Reset(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("清零 Image 分组成功率: %w", err)
	}
	return resetAt, nil
}

func calculateImageGroupSuccessRate(requestCount, failureCount int64) float64 {
	if requestCount <= 0 {
		return 100
	}
	if failureCount < 0 {
		failureCount = 0
	}
	if failureCount > requestCount {
		failureCount = requestCount
	}
	rate := float64(requestCount-failureCount) * 100 / float64(requestCount)
	rate = math.Round(rate*100) / 100
	return math.Max(0, math.Min(100, rate))
}
