package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageExportModelRepo struct {
	service.UsageLogRepository
	filters   usagestats.UsageLogFilters
	channelID int64
}

func (r *usageExportModelRepo) ListExport(_ context.Context, _ usagestats.ExportPageOptions, filters usagestats.UsageLogFilters) ([]service.UsageLog, string, error) {
	r.filters = filters
	intermediate, final := "private-intermediate", "visible-final"
	return []service.UsageLog{{ID: 7, UserID: 42, AccountID: 3, Model: "requested-model", ChannelID: &r.channelID, UpstreamModel: &intermediate, UpstreamResponseModel: &final}}, "7", nil
}

type usageExportChannelRepo struct {
	service.ChannelRepository
	enabled bool
	err     error
}

func (r *usageExportChannelRepo) ListAll(context.Context) ([]service.Channel, error) {
	return []service.Channel{{ID: 1, FeaturesConfig: map[string]any{"expose_response_model": r.enabled}}}, r.err
}

func TestUserUsageExportResponseModelVisibility(t *testing.T) {
	for _, tt := range []struct {
		name            string
		enabled         bool
		channelID       int64
		unavailable     bool
		repositoryError bool
		exposed         bool
	}{
		{name: "默认关闭", channelID: 1},
		{name: "管理员显式开启", channelID: 1, enabled: true, exposed: true},
		{name: "其他渠道不继承开关", channelID: 2, enabled: true},
		{name: "渠道服务缺失时隐藏", channelID: 1, enabled: true, unavailable: true},
		{name: "渠道缓存加载失败时隐藏", channelID: 1, enabled: true, repositoryError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &usageExportModelRepo{channelID: tt.channelID}
			channelRepo := &usageExportChannelRepo{enabled: tt.enabled}
			if tt.repositoryError {
				channelRepo.err = errors.New("测试缓存不可用")
			}
			var channels *service.ChannelService
			if !tt.unavailable {
				channels = service.NewChannelService(channelRepo, nil, nil, nil, nil)
			}
			h := NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil, channels)
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
			router.GET("/usage/export", h.Export)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/usage/export?user_id=99&account_id=3", nil))
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, int64(42), repo.filters.UserID)
			require.Zero(t, repo.filters.AccountID)
			body := recorder.Body.String()
			require.Contains(t, body, "requested-model")
			require.Contains(t, body, "\"next_cursor\":\"7\"")
			require.NotContains(t, body, "private-intermediate")
			require.NotContains(t, body, "upstream_response_model")
			require.NotContains(t, body, "account_stats_cost")
			if tt.exposed {
				require.Contains(t, body, "\"response_model\":\"visible-final\"")
			} else {
				require.NotContains(t, body, "visible-final")
				require.NotContains(t, body, "response_model")
			}
		})
	}
}
