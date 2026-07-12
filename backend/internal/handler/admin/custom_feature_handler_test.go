//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type customFeatureHandlerRepoStub struct {
	values map[string]string
}

func (s *customFeatureHandlerRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *customFeatureHandlerRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *customFeatureHandlerRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *customFeatureHandlerRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *customFeatureHandlerRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	for key, value := range values {
		s.values[key] = value
	}
	return nil
}

func (s *customFeatureHandlerRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *customFeatureHandlerRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func newCustomFeatureHandlerRouter(repo service.SettingRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewCustomFeatureHandler(service.NewSettingService(repo, &config.Config{}), nil)
	router := gin.New()
	router.GET("/api/v1/admin/custom-features", handler.GetSettings)
	router.PUT("/api/v1/admin/custom-features/model-marketplace", handler.UpdateModelMarketplace)
	router.PUT("/api/v1/admin/custom-features/daily-checkin", handler.UpdateDailyCheckin)
	router.PUT("/api/v1/admin/custom-features/gateway", handler.UpdateGateway)
	return router
}

func TestCustomFeatureHandler_GetSettings_返回独立契约(t *testing.T) {
	repo := &customFeatureHandlerRepoStub{values: map[string]string{
		service.SettingKeyDailyCheckinEnabled:     "true",
		service.SettingKeyModelMarketplaceEnabled: "false",
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/custom-features", nil)
	newCustomFeatureHandlerRouter(repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	require.Contains(t, data, "model_marketplace")
	require.Contains(t, data, "daily_checkin")
	require.Contains(t, data, "gateway")
}

func TestCustomFeatureHandler_UpdateModelMarketplace_规范化响应(t *testing.T) {
	repo := &customFeatureHandlerRepoStub{}
	body := bytes.NewBufferString(`{"enabled":true,"intro":"  hello  ","group_ids":[2,2,0]}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/custom-features/model-marketplace", body)
	request.Header.Set("Content-Type", "application/json")
	newCustomFeatureHandlerRouter(repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "hello", repo.values[service.SettingKeyModelMarketplaceIntro])
	require.Equal(t, "[2]", repo.values[service.SettingKeyModelMarketplaceGroupIDs])
}

func TestCustomFeatureHandler_UpdateDailyCheckin_拒绝概率不完整(t *testing.T) {
	repo := &customFeatureHandlerRepoStub{}
	body := bytes.NewBufferString(`{"enabled":true,"prizes":[{"id":"none","name":"谢谢参与","type":"none","probability_bps":9999,"enabled":true}],"unpaid_full_days":7,"unpaid_decay_rules":[],"linuxdo_exempt_enabled":false}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/custom-features/daily-checkin", body)
	request.Header.Set("Content-Type", "application/json")
	newCustomFeatureHandlerRouter(repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCustomFeatureHandler_UpdateGateway_ReturnsNormalizedSettings(t *testing.T) {
	repo := &customFeatureHandlerRepoStub{}
	body := bytes.NewBufferString(`{
		"default_pool_mode_retry_count":1,
		"default_pool_mode_retry_status_codes":[503,429,503],
		"auto_managed_probe_backoff_minutes":[5,10,30],
		"first_token_timeout_seconds":60,
		"image_group_success_rate_visible":true
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/custom-features/gateway", body)
	request.Header.Set("Content-Type", "application/json")
	newCustomFeatureHandlerRouter(repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, `[429,503]`, repo.values[service.SettingKeyGatewayDefaultPoolModeRetryStatusCodes])
	require.Equal(t, `[5,10,30]`, repo.values[service.SettingKeyGatewayAutoManagedProbeBackoffMinutes])
}

func TestCustomFeatureHandler_UpdateGateway_RejectsDecreasingBackoff(t *testing.T) {
	repo := &customFeatureHandlerRepoStub{}
	body := bytes.NewBufferString(`{
		"default_pool_mode_retry_count":1,
		"default_pool_mode_retry_status_codes":[],
		"auto_managed_probe_backoff_minutes":[10,5],
		"first_token_timeout_seconds":60,
		"image_group_success_rate_visible":true
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/custom-features/gateway", body)
	request.Header.Set("Content-Type", "application/json")
	newCustomFeatureHandlerRouter(repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, repo.values)
}

func TestCustomFeatureHandler_UpdateGateway_EmptyRetryStatusCodesReturnsArray(t *testing.T) {
	repo := &customFeatureHandlerRepoStub{}
	body := bytes.NewBufferString(`{
		"default_pool_mode_retry_count":1,
		"default_pool_mode_retry_status_codes":[],
		"auto_managed_probe_backoff_minutes":[5,10],
		"first_token_timeout_seconds":60,
		"image_group_success_rate_visible":true
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/custom-features/gateway", body)
	request.Header.Set("Content-Type", "application/json")
	newCustomFeatureHandlerRouter(repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var responseBody map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	data, ok := responseBody["data"].(map[string]any)
	require.True(t, ok)
	retryCodes, ok := data["default_pool_mode_retry_status_codes"].([]any)
	require.True(t, ok)
	require.Empty(t, retryCodes)
}
