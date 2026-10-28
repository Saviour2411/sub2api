//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func samplingPolicyTestService() *SettingService {
	return NewSettingService(&anthropicSamplingFilterSettingsRepo{
		customFeatureSettingsRepoStub: &customFeatureSettingsRepoStub{values: map[string]string{
			SettingKeyGatewayAnthropicSamplingParameterFilterEnabled:                  "true",
			SettingKeyGatewayAnthropicSamplingParameterFilterModels:                   `["claude-sonnet-5-5","claude-haiku-5-5"]`,
			SettingKeyGatewayAnthropicSamplingParameterFilterExcludedGroupIDs:         `[45]`,
			SettingKeyGatewayAnthropicSamplingParameterFilterSkipThinkingValidation:   "true",
			SettingKeyGatewayAnthropicSamplingParameterFilterSkipToolChoiceValidation: "true",
		}},
	}, &config.Config{})
}

func TestAnthropicSamplingPolicy_分组白名单开关隔离(t *testing.T) {
	settings := samplingPolicyTestService().GetGatewayRuntime(context.Background())
	for _, tc := range []struct {
		name                    string
		group                   *Group
		model                   string
		enabled, filtered, skip bool
	}{
		{"命中", &Group{ID: 44, Platform: PlatformAnthropic}, "claude-sonnet-5-5", true, true, true},
		{"停用组", &Group{ID: 45, Platform: PlatformAnthropic}, "claude-sonnet-5-5", true, false, false},
		{"其他平台", &Group{ID: 44, Platform: PlatformOpenAI}, "claude-sonnet-5-5", true, false, false},
		{"模型不命中", &Group{ID: 44, Platform: PlatformAnthropic}, "claude-sonnet-4-6", true, false, false},
		{"总开关关闭", &Group{ID: 44, Platform: PlatformAnthropic}, "claude-sonnet-5-5", false, false, false},
		{"无分组测试", nil, "claude-sonnet-5-5", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := cloneGatewaySettings(settings)
			input.AnthropicSamplingParameterFilterEnabled = tc.enabled
			ctx := context.WithValue(context.Background(), ctxkey.Group, tc.group)
			policy := resolveAnthropicSamplingPolicy(ctx, input, tc.model)
			require.Equal(t, tc.filtered, policy.enabled)
			require.Equal(t, tc.skip, policy.validation.SkipThinkingValidation)
			require.Equal(t, tc.skip, policy.validation.SkipToolChoiceValidation)
		})
	}
}

func TestAnthropicSamplingPolicy_独立校验开关(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		for _, tool := range []bool{false, true} {
			t.Run(fmt.Sprintf("thinking=%v/tool=%v", thinking, tool), func(t *testing.T) {
				options := apicompat.AnthropicModelValidationOptions{SkipThinkingValidation: thinking, SkipToolChoiceValidation: tool}
				require.Equal(t, thinking, validateClaude55RequestWithOptions([]byte(`{"thinking":{"type":"enabled","budget_tokens":1000}}`), "claude-sonnet-5-5", options) == nil)
				require.Equal(t, tool, validateClaude55RequestWithOptions([]byte(`{"tool_choice":{"type":"any"}}`), "claude-sonnet-5-5", options) == nil)
				// 跳过另两类限制不会绕过采样校验。
				require.Error(t, validateClaude55RequestWithOptions([]byte(`{"temperature":0.7}`), "claude-sonnet-5-5", options))
			})
		}
	}
}

func TestAnthropicSamplingPolicy_重试不沿用旧策略(t *testing.T) {
	svc := &GatewayService{settingService: samplingPolicyTestService()}
	account := newAnthropicAPIKeyAccountForTest()
	original := []byte(`{"model":"claude-sonnet-5-5","messages":[],"temperature":0.7,"tool_choice":{"type":"any"}}`)
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{ID: 44, Platform: PlatformAnthropic})
	firstCtx, first, options, err := svc.prepareAnthropicSamplingRequest(ctx, account, original, "claude-sonnet-5-5")
	require.NoError(t, err)
	require.True(t, options.SkipToolChoiceValidation)
	require.False(t, gjson.GetBytes(first, "temperature").Exists())
	require.True(t, gjson.GetBytes(original, "temperature").Exists())
	for _, tc := range []struct {
		group int64
		model string
	}{{45, "claude-sonnet-5-5"}, {44, "claude-sonnet-4-6"}} {
		nextCtx := context.WithValue(firstCtx, ctxkey.Group, &Group{ID: tc.group, Platform: PlatformAnthropic})
		nextCtx, next, options, err := svc.prepareAnthropicSamplingRequest(nextCtx, account, original, tc.model)
		require.NoError(t, err)
		require.False(t, options.SkipToolChoiceValidation)
		require.Equal(t, original, next)
		out, changed, err := filterAnthropicSamplingParametersWithSettings(nextCtx, svc.settingService, next, tc.model)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, original, out)
	}
}

func TestAnthropicSamplingPolicy_入口实际转发(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"messages", "passthrough", "count_tokens", "responses", "chat"} {
		for _, model := range []string{"claude-sonnet-5-5", "claude-haiku-5-5"} {
			for _, excluded := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/停用=%v", endpoint, model, excluded), func(t *testing.T) {
					cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
					responseBody := `{"id":"msg_test","type":"message","role":"assistant","model":"` + model + `","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`
					if endpoint == "count_tokens" {
						responseBody = `{"input_tokens":1}`
					}
					if endpoint == "responses" || endpoint == "chat" {
						responseBody = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"" + model + "\",\"content\":[],\"usage\":{\"input_tokens\":1}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
					}
					upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody))}}
					svc := &GatewayService{cfg: cfg, settingService: samplingPolicyTestService(), httpUpstream: upstream, responseHeaderFilter: compileResponseHeaderFilter(cfg), rateLimitService: &RateLimitService{}, deferredService: &DeferredService{}}
					account := newAnthropicAPIKeyAccountForTest()
					account.Extra["anthropic_passthrough"] = endpoint == "passthrough"
					account.Credentials["model_mapping"] = map[string]any{"client-alias": model}
					groupID := int64(44)
					if excluded {
						groupID = 45
					}
					ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{ID: groupID, Platform: PlatformAnthropic})
					recorder := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(recorder)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, nil).WithContext(ctx)
					body := []byte(`{"model":"client-alias","max_tokens":100,"messages":[{"role":"user","content":"hello"}],"input":"hello","temperature":0.7,"top_p":0.8,"top_k":20}`)
					parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "client-alias"}
					var err error
					switch endpoint {
					case "responses":
						_, err = svc.ForwardAsResponses(ctx, c, account, body, parsed)
					case "chat":
						_, err = svc.ForwardAsChatCompletions(ctx, c, account, body, parsed)
					case "count_tokens":
						err = svc.ForwardCountTokens(ctx, c, account, parsed)
					default:
						_, err = svc.Forward(ctx, c, account, parsed)
					}
					if excluded {
						require.ErrorContains(t, err, "non-default temperature")
						require.Equal(t, http.StatusBadRequest, recorder.Code)
						require.Nil(t, upstream.lastReq)
					} else {
						require.NoError(t, err)
						require.NotNil(t, upstream.lastReq)
						assertAnthropicSamplingParametersRemoved(t, upstream.lastBody)
						require.Equal(t, model, gjson.GetBytes(upstream.lastBody, "model").String())
					}
				})
			}
		}
	}
}

func TestAnthropicSamplingPolicy_设置保存缓存与分组校验(t *testing.T) {
	svc := samplingPolicyTestService()
	svc.SetDefaultSubscriptionGroupReader(&customFeatureGroupReaderStub{groups: map[int64]*Group{44: {ID: 44, Platform: PlatformAnthropic, Status: "disabled"}, 46: {ID: 46, Platform: PlatformOpenAI}}})
	input := svc.GetGatewayRuntime(context.Background())
	input.AnthropicSamplingParameterFilterExcludedGroupIDs = []int64{44, 44, 45}
	updated, err := svc.UpdateGatewaySettings(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []int64{44, 45}, updated.AnthropicSamplingParameterFilterExcludedGroupIDs)
	updated.AnthropicSamplingParameterFilterExcludedGroupIDs[0] = 999
	runtime := svc.GetGatewayRuntime(context.Background())
	require.Equal(t, []int64{44, 45}, runtime.AnthropicSamplingParameterFilterExcludedGroupIDs)
	require.True(t, runtime.AnthropicSamplingParameterFilterSkipThinkingValidation)
	require.True(t, runtime.AnthropicSamplingParameterFilterSkipToolChoiceValidation)
	for _, id := range []int64{46, 99} {
		input.AnthropicSamplingParameterFilterExcludedGroupIDs = []int64{id}
		_, err = svc.UpdateGatewaySettings(context.Background(), input)
		require.ErrorIs(t, err, ErrGatewaySettingsInvalid)
	}
	data, err := json.Marshal(DefaultGatewaySettings())
	require.NoError(t, err)
	require.JSONEq(t, `[]`, gjson.GetBytes(data, "anthropic_sampling_parameter_filter_excluded_group_ids").Raw)
}

func TestAnthropicSamplingPolicy_原生跳过校验仍保留字段(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, tc := range []struct{ thinking, tool, excluded bool }{{false, false, false}, {true, false, false}, {false, true, false}, {true, true, false}, {true, true, true}} {
			t.Run(fmt.Sprintf("透传=%v/thinking=%v/tool=%v/停用=%v", passthrough, tc.thinking, tc.tool, tc.excluded), func(t *testing.T) {
				settingsService := samplingPolicyTestService()
				settings := settingsService.GetGatewayRuntime(context.Background())
				settings.AnthropicSamplingParameterFilterSkipThinkingValidation = tc.thinking
				settings.AnthropicSamplingParameterFilterSkipToolChoiceValidation = tc.tool
				settingsService.storeGatewaySettingsCache(settings, gatewaySettingsCacheTTL)
				cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
				upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))}}
				svc := &GatewayService{cfg: cfg, settingService: settingsService, httpUpstream: upstream, responseHeaderFilter: compileResponseHeaderFilter(cfg), rateLimitService: &RateLimitService{}, deferredService: &DeferredService{}}
				account := newAnthropicAPIKeyAccountForTest()
				account.Extra["anthropic_passthrough"] = passthrough
				groupID := int64(44)
				if tc.excluded {
					groupID = 45
				}
				ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{ID: groupID, Platform: PlatformAnthropic})
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
				body := []byte(`{"model":"claude-sonnet-5-5","max_tokens":4096,"temperature":0.7,"thinking":{"type":"enabled","budget_tokens":1024},"tool_choice":{"type":"any"},"tools":[{"name":"test","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"hello"}]}`)
				_, err := svc.Forward(ctx, c, account, &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-5-5"})
				if !tc.thinking || !tc.tool || tc.excluded {
					require.Error(t, err)
					require.Nil(t, upstream.lastReq)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, upstream.lastReq)
				assertAnthropicSamplingParametersRemoved(t, upstream.lastBody)
				require.JSONEq(t, `{"type":"enabled","budget_tokens":1024}`, gjson.GetBytes(upstream.lastBody, "thinking").Raw)
				require.JSONEq(t, `{"type":"any"}`, gjson.GetBytes(upstream.lastBody, "tool_choice").Raw)
			})
		}
	}
}
