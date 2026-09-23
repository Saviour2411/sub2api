//go:build unit

package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestUsageExportQueryKeepsFiltersWithoutCountOrOffset(t *testing.T) {
	start, end := time.Now().Add(-time.Hour), time.Now()
	requestType, billingType, enabled := int16(2), int8(1), true
	filters := UsageLogFilters{
		UserID: 42, APIKeyID: 12, AccountID: 8, GroupID: 3, Model: "requested-model",
		ModelFilterSource: usagestats.ModelSourceRequested, RequestID: "request-1",
		RequestType: &requestType, BillingType: &billingType, NativeCompactionV2: &enabled,
		BillingMode: "token", UpstreamModelMismatch: &enabled, StartTime: &start, EndTime: &end,
	}
	where, args := usageLogFilterWhere(filters)
	query, exportArgs := usageExportQuery(usagestats.ExportPageOptions{BeforeID: 900, PageSize: 1000}, filters)
	require.Contains(t, query, where+" AND id <")
	require.Equal(t, args, exportArgs[:len(args)])
	require.Equal(t, []any{int64(900), 1001}, exportArgs[len(args):])
	require.Contains(t, query, "ORDER BY id DESC LIMIT")
	require.NotContains(t, query, "COUNT(")
	require.NotContains(t, query, "OFFSET")
	query, _ = usageExportQuery(usagestats.ExportPageOptions{BeforeID: 900, PageSize: 10}, UsageLogFilters{})
	require.Contains(t, query, "WHERE id < $1")
}
