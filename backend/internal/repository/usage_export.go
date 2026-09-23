package repository

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.UsageExportRepository = (*usageLogRepository)(nil)

// ListExport 固定按不可变 ID 倒序读取，避免新增记录导致 OFFSET 翻页重复或遗漏。
func (r *usageLogRepository) ListExport(ctx context.Context, options usagestats.ExportPageOptions, filters UsageLogFilters) ([]service.UsageLog, string, error) {
	query, args := usageExportQuery(options, filters)
	logs, err := r.queryUsageLogs(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(logs) > options.PageSize {
		logs = logs[:options.PageSize]
		next = strconv.FormatInt(logs[len(logs)-1].ID, 10)
	}
	if err := r.hydrateUsageLogAssociations(ctx, logs); err != nil {
		return nil, "", err
	}
	return logs, next, nil
}

func usageExportQuery(options usagestats.ExportPageOptions, filters UsageLogFilters) (string, []any) {
	where, args := usageLogFilterWhere(filters)
	if options.BeforeID > 0 {
		if where == "" {
			where = "WHERE "
		} else {
			where += " AND "
		}
		args = append(args, options.BeforeID)
		where += fmt.Sprintf("id < $%d", len(args))
	}
	args = append(args, options.PageSize+1)
	query := fmt.Sprintf("SELECT %s FROM usage_logs %s ORDER BY id DESC LIMIT $%d", usageLogSelectColumns, where, len(args))
	return query, args
}
