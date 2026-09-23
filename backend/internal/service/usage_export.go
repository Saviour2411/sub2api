package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// UsageExportRepository 单独声明游标导出能力，普通列表及其既有分页契约不变。
type UsageExportRepository interface {
	ListExport(context.Context, usagestats.ExportPageOptions, usagestats.UsageLogFilters) ([]UsageLog, string, error)
}

func (s *UsageService) ListExport(ctx context.Context, options usagestats.ExportPageOptions, filters usagestats.UsageLogFilters) ([]UsageLog, string, error) {
	if options.BeforeID < 0 || options.PageSize <= 0 || options.PageSize > usagestats.MaxExportPageSize {
		return nil, "", fmt.Errorf("导出分页参数无效")
	}
	repo, ok := s.usageRepo.(UsageExportRepository)
	if !ok {
		return nil, "", fmt.Errorf("使用记录导出暂不可用")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return repo.ListExport(ctx, options, filters)
}
