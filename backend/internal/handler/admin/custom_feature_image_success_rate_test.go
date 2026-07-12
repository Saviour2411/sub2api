package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type customFeatureImageSuccessRateRepoStub struct {
	resetAt time.Time
}

func (r *customFeatureImageSuccessRateRepoStub) Record(context.Context, int64, int64, int64, time.Time) error {
	return nil
}

func (r *customFeatureImageSuccessRateRepoStub) RecordOnce(context.Context, string, int64, int64, int64, time.Time) error {
	return nil
}

func (r *customFeatureImageSuccessRateRepoStub) ListCurrent(context.Context) ([]service.ImageGroupSuccessRateAggregate, error) {
	return nil, nil
}

func (r *customFeatureImageSuccessRateRepoStub) Reset(context.Context) (time.Time, error) {
	return r.resetAt, nil
}

func TestCustomFeatureHandlerResetImageGroupSuccessRates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetAt := time.Date(2026, 7, 12, 15, 4, 5, 123000000, time.UTC)
	stats := service.NewImageGroupSuccessRateService(&customFeatureImageSuccessRateRepoStub{resetAt: resetAt})
	handler := NewCustomFeatureHandler(nil, stats)
	router := gin.New()
	router.POST("/api/v1/admin/custom-features/gateway/image-group-success-rates/reset", handler.ResetImageGroupSuccessRates)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/custom-features/gateway/image-group-success-rates/reset", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"code": 0,
		"message": "success",
		"data": {"reset_at": "2026-07-12T15:04:05.123Z"}
	}`, recorder.Body.String())
}
