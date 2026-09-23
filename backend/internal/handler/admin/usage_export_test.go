package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminUsageExportCapture struct {
	service.UsageLogRepository
	filters usagestats.UsageLogFilters
}

func (r *adminUsageExportCapture) ListExport(_ context.Context, _ usagestats.ExportPageOptions, filters usagestats.UsageLogFilters) ([]service.UsageLog, string, error) {
	r.filters = filters
	return []service.UsageLog{}, "", nil
}

func TestAdminUsageExportKeepsAllFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &adminUsageExportCapture{}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	router := gin.New()
	router.GET("/admin/usage/export", h.Export)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/usage/export?user_id=42&account_id=9&api_key_id=8&group_id=7&request_id=req&model=model&billing_type=1&billing_mode=token&request_type=ws_v2&native_compaction_v2=true&upstream_model_mismatch=true&start_date=2026-09-01&end_date=2026-09-23", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.filters.UserID)
	require.Equal(t, int64(9), repo.filters.AccountID)
	require.Equal(t, int64(8), repo.filters.APIKeyID)
	require.Equal(t, int64(7), repo.filters.GroupID)
	require.Equal(t, "req", repo.filters.RequestID)
	require.Equal(t, "model", repo.filters.Model)
	require.Equal(t, "token", repo.filters.BillingMode)
	require.NotNil(t, repo.filters.RequestType)
	require.NotNil(t, repo.filters.BillingType)
	require.NotNil(t, repo.filters.StartTime)
	require.NotNil(t, repo.filters.EndTime)
	require.True(t, *repo.filters.NativeCompactionV2)
	require.True(t, *repo.filters.UpstreamModelMismatch)
	require.Contains(t, rec.Body.String(), `"next_cursor":""`)
}
