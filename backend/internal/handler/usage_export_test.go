package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageExportCapture struct {
	service.UsageLogRepository
	options usagestats.ExportPageOptions
	filters usagestats.UsageLogFilters
	calls   int
}

func (r *usageExportCapture) ListExport(_ context.Context, options usagestats.ExportPageOptions, filters usagestats.UsageLogFilters) ([]service.UsageLog, string, error) {
	r.options, r.filters = options, filters
	r.calls++
	upstreamModel := "private-model"
	return []service.UsageLog{{ID: 7, UserID: 42, AccountID: 3, UpstreamModel: &upstreamModel}}, "7", nil
}

func TestUserUsageExportEnforcesOwnerAndProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &usageExportCapture{}
	h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
	router.GET("/usage/export", h.Export)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/export?user_id=99&account_id=3&cursor=9&page_size=1000&model=test&group_id=4&request_type=ws_v2&billing_mode=token&native_compaction_v2=true", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.filters.UserID)
	require.Zero(t, repo.filters.AccountID)
	require.Equal(t, int64(4), repo.filters.GroupID)
	require.Equal(t, "test", repo.filters.Model)
	require.True(t, *repo.filters.NativeCompactionV2)
	require.Equal(t, usagestats.ExportPageOptions{BeforeID: 9, PageSize: 1000}, repo.options)
	require.NotContains(t, rec.Body.String(), "private-model")
	require.NotContains(t, rec.Body.String(), `"account":`)
	require.NotContains(t, rec.Body.String(), "account_stats_cost")
	var payload struct {
		Data struct {
			Next string `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "7", payload.Data.Next)
}

func TestUserUsageExportRejectsInvalidOrUnauthenticatedRequests(t *testing.T) {
	for _, query := range []string{"?cursor=-1", "?page_size=1001", "?request_type=invalid", "?start_date=invalid"} {
		repo := &usageExportCapture{}
		h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
		router.GET("/usage/export", h.Export)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/export"+query, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Zero(t, repo.calls)
	}
	router := gin.New()
	router.GET("/usage/export", NewUsageHandler(nil, nil, nil, nil).Export)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/export", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
