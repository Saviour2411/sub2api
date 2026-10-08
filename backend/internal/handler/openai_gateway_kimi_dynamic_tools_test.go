//go:build unit

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestKimiDynamicToolsHandlerRejectsBeforeQueue(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, message := range []string{
			`{"role":"system","content":"非空正文","tools":[{"type":"function","function":{"name":"get_weather"}}]}`,
			`{"role":"user","content":"用户声明","tools":[]}`,
			`{"role":"assistant","content":"助手声明","tools":[]}`,
			`{"role":"developer","tools":[]}`,
			`{"role":"tool","content":"缺少调用编号"}`,
		} {
			t.Run(fmt.Sprintf("%s/流式=%t", gjson.Get(message, "role").String(), stream), func(t *testing.T) {
				cfg := &config.Config{RunMode: config.RunModeSimple}
				settings := service.NewSettingService(&safeHandlerSettingsRepo{values: map[string]string{
					service.SettingKeyGatewayKimiDynamicToolsEnabled: "true",
				}}, cfg)
				billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billing.Stop)
				gateway := service.NewOpenAIGatewayService(
					nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
					service.NewBillingService(cfg, nil), nil, billing, nil,
					&service.DeferredService{}, nil, nil, nil, nil, nil, settings, nil,
				)
				cache := &concurrencyCacheMock{
					acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
						t.Fatal("非法动态工具请求不能进入用户并发队列")
						return false, nil
					},
					acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) {
						t.Fatal("非法动态工具请求不能占用上游并发")
						return false, nil
					},
				}
				h := &OpenAIGatewayHandler{
					gatewayService: gateway, billingCacheService: billing, cfg: cfg,
					apiKeyService:     &service.APIKeyService{},
					concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
				}
				router := gin.New()
				router.POST("/v1/chat/completions", func(c *gin.Context) {
					groupID := int64(53)
					c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
						ID: 1, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformKimi},
						User: &service.User{ID: 1},
					})
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
					h.ChatCompletions(c)
				})
				body := fmt.Sprintf(`{"model":"kimi-k3","stream":%t,"messages":[%s]}`, stream, message)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))
				require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
				require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
				require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
				require.NotContains(t, recorder.Body.String(), "data:")
			})
		}
	}
}
