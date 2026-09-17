//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func kimiCompatTestService(upstream HTTPUpstream) *OpenAIGatewayService {
	cfg := rawChatCompletionsTestConfig()
	settings := NewSettingService(&anthropicSamplingFilterSettingsRepo{&customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyGatewayKimiSamplingParameterRetryEnabled:   "true",
		SettingKeyGatewayKimiReasoningEffortRetryEnabled:     "true",
		SettingKeyGatewayKimiToolChoiceRetryEnabled:          "true",
		SettingKeyGatewayKimiMaxCompletionTokensRetryEnabled: "true",
	}}}, cfg)
	return &OpenAIGatewayService{cfg: cfg, settingService: settings, httpUpstream: upstream}
}

func kimiCompatTestContext(body []byte, path string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	ctx.Set("api_key", &APIKey{Group: &Group{ID: 53, Platform: PlatformKimi}})
	return ctx, recorder
}

func kimiCompatTestAccount() *Account {
	account := rawChatCompletionsTestAccount()
	account.Platform = PlatformKimi
	account.Credentials["api_protocol"] = "chat_completions"
	return account
}

func kimiCompatTestEvents(ctx *gin.Context) []*OpsUpstreamErrorEvent {
	value, _ := ctx.Get(OpsUpstreamErrorsKey)
	events, _ := value.([]*OpsUpstreamErrorEvent)
	return events
}

func kimiCompatTestResponse(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}, "X-Request-Id": {"kimi-test-attempt"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func kimiCompatTestSuccess(stream bool) *http.Response {
	if !stream {
		return kimiCompatTestResponse(http.StatusOK, []byte(`{"id":"chatcmpl_kimi","object":"chat.completion","model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`))
	}
	response := kimiCompatTestResponse(http.StatusOK, []byte("data: {\"id\":\"chatcmpl_kimi\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k3\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n"+
		"data: {\"id\":\"chatcmpl_kimi\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k3\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"id\":\"chatcmpl_kimi\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k3\",\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n"+
		"data: [DONE]\n\n"))
	response.Header.Set("Content-Type", "text/event-stream")
	return response
}

func TestKimiCompatibilityAllCCPaths(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/messages", "/v1/responses"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", path, stream), func(t *testing.T) {
				body := fmt.Sprintf(`{"model":"kimi-k3","messages":[{"role":"user","content":"hello"}],"temperature":0.7,"max_tokens":128,"stream":%t}`, stream)
				if path == "/v1/responses" {
					body = fmt.Sprintf(`{"model":"kimi-k3","input":"hello","temperature":0.7,"stream":%t}`, stream)
				}
				ctx, recorder := kimiCompatTestContext([]byte(body), path)
				failure := kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model"))
				tracked := &passthroughCloseTrackingReadCloser{Reader: failure.Body}
				failure.Body = tracked
				upstream := &httpUpstreamRecorder{responses: []*http.Response{failure, kimiCompatTestSuccess(stream)}}
				service := kimiCompatTestService(upstream)
				account := kimiCompatTestAccount()
				var result *OpenAIForwardResult
				var err error
				switch path {
				case "/v1/chat/completions":
					result, err = service.ForwardAsChatCompletions(ctx.Request.Context(), ctx, account, []byte(body), "", "")
				case "/v1/messages":
					result, err = service.ForwardAsAnthropic(ctx.Request.Context(), ctx, account, []byte(body), "", "")
				case "/v1/responses":
					result, err = service.Forward(ctx.Request.Context(), ctx, account, []byte(body))
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, upstream.bodies, 2)
				require.True(t, gjson.GetBytes(upstream.bodies[0], "temperature").Exists())
				require.False(t, gjson.GetBytes(upstream.bodies[1], "temperature").Exists())
				require.True(t, tracked.closed)
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.Equal(t, 200, recorder.Code)
				require.NotContains(t, recorder.Body.String(), "invalid")
				require.Contains(t, recorder.Body.String(), "ok")
				if path != "/v1/messages" {
					require.False(t, gjson.GetBytes(upstream.bodies[1], "reasoning_effort").Exists())
				}
				events := kimiCompatTestEvents(ctx)
				require.Len(t, events, 1)
				require.Equal(t, "parameter_compat_retry", events[0].Kind)
				require.Equal(t, kimiCompatSampling.String(), events[0].Reason)
			})
		}
	}
}

func TestKimiCompatibilityFourRepairsAndOuterRetry(t *testing.T) {
	body := []byte(`{"model":"kimi-k3","messages":[],"temperature":0.7,"top_p":0.5,"reasoning_effort":"invalid-audit-value","tool_choice":"bogus","tools":[{"type":"function","function":{"name":"keep"}}],"max_completion_tokens":128}`)
	ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")),
		kimiCompatTestResponse(400, kimiTestError(`level "invalid-audit-value" not supported, valid levels: low, high, max`)),
		kimiCompatTestResponse(400, kimiTestError("Invalid value for `tool_choice`: bogus! Only named tools, \"none\", \"auto\" or \"required\" are supported.")),
		kimiCompatTestResponse(400, kimiTestError("max_completion_tokens [128] must be greater than thinking_budget [32768]")),
		kimiCompatTestResponse(503, []byte(`{"error":{"message":"busy"}}`)),
		kimiCompatTestSuccess(false),
	}}
	service := kimiCompatTestService(upstream)
	account := kimiCompatTestAccount()
	response, _, err := service.sendCCUpstreamRequest(ctx.Request.Context(), ctx, account, "http://upstream.example/v1/chat/completions", body, false, "kimi-k3", "test", "", "")
	require.NoError(t, err)
	require.Equal(t, 503, response.StatusCode)
	require.NoError(t, response.Body.Close())
	require.Len(t, upstream.bodies, 5)
	require.Len(t, kimiCompatTestEvents(ctx), 4)
	require.Equal(t, "invalid-audit-value", gjson.Get(kimiCompatTestEvents(ctx)[1].Detail, "rejected_effort").String())
	require.Equal(t, "low", gjson.Get(kimiCompatTestEvents(ctx)[1].Detail, "effective_effort").String())
	account.ID++
	account.Credentials["model_mapping"] = map[string]any{"kimi-k3": "kimi-k2.6"}
	result, err := service.forwardAsRawChatCompletions(ctx.Request.Context(), ctx, account, body, "")
	require.NoError(t, err)
	require.Len(t, upstream.bodies, 6)
	require.Equal(t, 4, service.kimiParameterCompat(ctx.Request.Context(), ctx).retries)
	require.Equal(t, "low", *result.ReasoningEffort)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, "kimi-k2.6", gjson.GetBytes(upstream.lastBody, "model").String())
	for _, finalBody := range upstream.bodies[4:] {
		for _, removed := range []string{"temperature", "top_p", "tool_choice", "max_completion_tokens"} {
			require.False(t, gjson.GetBytes(finalBody, removed).Exists())
		}
		require.Equal(t, "low", gjson.GetBytes(finalBody, "reasoning_effort").String())
		require.Equal(t, gjson.GetBytes(body, "tools").Raw, gjson.GetBytes(finalBody, "tools").Raw)
	}
}

func TestKimiCompatibilityEachRepairRecovers(t *testing.T) {
	for _, test := range []struct {
		rule    kimiCompatRule
		message string
	}{
		{kimiCompatSampling, "field Temperature invalid, only 1 is allowed for this model"},
		{kimiCompatReasoning, `level "invalid-audit-value" not supported, valid levels: low, high, max`},
		{kimiCompatToolChoice, "Invalid value for `tool_choice`: bogus! Only named tools, \"none\", \"auto\" or \"required\" are supported."},
		{kimiCompatBudget, "max_completion_tokens [128] must be greater than thinking_budget [32768]"},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", test.rule, stream), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":"kimi-k3","messages":[{"role":"user","content":"hello"}],"stream":%t,"temperature":0.7,"reasoning_effort":"invalid-audit-value","tool_choice":"bogus","tools":[{"type":"function","function":{"name":"keep"}}],"max_completion_tokens":128}`, stream))
				ctx, recorder := kimiCompatTestContext(body, "/v1/chat/completions")
				upstream := &httpUpstreamRecorder{responses: []*http.Response{
					kimiCompatTestResponse(400, kimiTestError(test.message)), kimiCompatTestSuccess(stream),
				}}
				service := kimiCompatTestService(upstream)
				state := service.kimiParameterCompat(ctx.Request.Context(), ctx)
				state.settings = DefaultGatewaySettings()
				fields := []*bool{&state.settings.KimiSamplingParameterRetryEnabled, &state.settings.KimiReasoningEffortRetryEnabled, &state.settings.KimiToolChoiceRetryEnabled, &state.settings.KimiMaxCompletionTokensRetryEnabled}
				*fields[test.rule] = true
				result, err := service.ForwardAsChatCompletions(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "", "")
				require.NoError(t, err)
				require.Len(t, upstream.requests, 2)
				expected, _, err := applyKimiParameterRepair(upstream.bodies[0], test.rule)
				require.NoError(t, err)
				require.JSONEq(t, string(expected), string(upstream.bodies[1]))
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.NotContains(t, recorder.Body.String(), test.message)
				require.Len(t, kimiCompatTestEvents(ctx), 1)
				require.Equal(t, test.rule.String(), kimiCompatTestEvents(ctx)[0].Reason)
				if test.rule == kimiCompatReasoning {
					require.Equal(t, "low", *result.ReasoningEffort)
				}
			})
		}
	}
}

func TestKimiCompatibilityScopeAndLimits(t *testing.T) {
	for _, test := range []struct {
		name      string
		platform  string
		status    int
		disabled  bool
		committed bool
		cancelled bool
	}{
		{"关闭", PlatformKimi, 400, true, false, false},
		{"非Kimi", PlatformOpenAI, 400, false, false, false},
		{"综合分组", PlatformComposite, 400, false, false, false},
		{"额度错误", PlatformKimi, 403, false, false, false},
		{"上游500", PlatformKimi, 500, false, false, false},
		{"已提交", PlatformKimi, 400, false, true, false},
		{"取消", PlatformKimi, 400, false, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(`{"model":"kimi-k3","temperature":0.7}`)
			ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
			ctx.Set("api_key", &APIKey{Group: &Group{Platform: test.platform}})
			if test.committed {
				_, err := ctx.Writer.WriteString("data: output\n\n")
				require.NoError(t, err)
			}
			if test.cancelled {
				cancelCtx, cancel := context.WithCancel(ctx.Request.Context())
				cancel()
				ctx.Request = ctx.Request.WithContext(cancelCtx)
			}
			failureBody := kimiTestError("field Temperature invalid, only 1 is allowed for this model")
			upstream := &httpUpstreamRecorder{resp: kimiCompatTestResponse(test.status, failureBody)}
			service := kimiCompatTestService(upstream)
			if test.disabled {
				service.settingService = nil
			}
			response, _, err := service.sendCCUpstreamRequest(ctx.Request.Context(), ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", body, false, "kimi-k3", "test", "", "")
			if test.cancelled {
				require.ErrorIs(t, err, context.Canceled)
				require.Empty(t, upstream.requests)
				return
			}
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			got, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, failureBody, got)
			require.Empty(t, kimiCompatTestEvents(ctx))
		})
	}
	ctx, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")),
		kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")),
	}}
	service := kimiCompatTestService(upstream)
	response, _, err := service.sendCCUpstreamRequest(ctx.Request.Context(), ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", []byte(`{"temperature":0.7}`), false, "kimi-k3", "test", "", "")
	require.NoError(t, err)
	require.Equal(t, 400, response.StatusCode)
	require.NoError(t, response.Body.Close())
	require.Len(t, upstream.requests, 2)
}

func TestKimiCompatibilityMessagesEffortAndBudget(t *testing.T) {
	for _, test := range []struct {
		name     string
		model    string
		effort   string
		disabled bool
		want     string
	}{
		{"默认low", "kimi-k3", "", false, "low"},
		{"保留max", "kimi-k3", "max", false, "max"},
		{"保留high", "kimi-k3", "high", false, "high"},
		{"显式medium不预改", "kimi-k3", "medium", false, "medium"},
		{"其他模型", "kimi-k2.6", "", false, "medium"},
		{"关闭默认不改", "kimi-k3", "", true, "medium"},
		{"关闭max不改", "kimi-k3", "max", true, "max"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"model":"alias","messages":[{"role":"user","content":"hello"}],"max_tokens":128,"output_config":{"effort":%q}}`, test.effort)
			ctx, _ := kimiCompatTestContext([]byte(body), "/v1/messages")
			upstream := &httpUpstreamRecorder{resp: kimiCompatTestSuccess(false)}
			service := kimiCompatTestService(upstream)
			if test.disabled {
				service.settingService = nil
			}
			account := kimiCompatTestAccount()
			account.Credentials["model_mapping"] = map[string]any{"alias": test.model}
			result, err := service.ForwardAsAnthropic(ctx.Request.Context(), ctx, account, []byte(body), "", "")
			require.NoError(t, err)
			require.Equal(t, test.want, gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
			require.Equal(t, test.want, *result.ReasoningEffort)
		})
	}
	for _, path := range []string{"/v1/messages", "/v1/responses"} {
		t.Run("预算/"+path, func(t *testing.T) {
			body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
			if path == "/v1/responses" {
				body = []byte(`{"model":"kimi-k3","input":"hello","max_output_tokens":128}`)
			}
			ctx, _ := kimiCompatTestContext(body, path)
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				kimiCompatTestResponse(400, kimiTestError("max_completion_tokens [128] must be greater than thinking_budget [32768]")),
				kimiCompatTestSuccess(false),
			}}
			service := kimiCompatTestService(upstream)
			if path == "/v1/messages" {
				_, err := service.ForwardAsAnthropic(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "", "")
				require.NoError(t, err)
			} else {
				_, err := service.Forward(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body)
				require.NoError(t, err)
			}
			require.Len(t, upstream.requests, 2)
			require.True(t, gjson.GetBytes(upstream.bodies[0], "max_completion_tokens").Exists())
			require.False(t, gjson.GetBytes(upstream.bodies[1], "max_completion_tokens").Exists())
		})
	}
}

func TestKimiCompatibilityFrozenSettings(t *testing.T) {
	service := kimiCompatTestService(nil)
	ctx, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
	state := service.kimiParameterCompat(ctx.Request.Context(), ctx)
	require.True(t, state.settings.KimiReasoningEffortRetryEnabled)
	service.settingService.storeGatewaySettingsCache(DefaultGatewaySettings(), gatewaySettingsCacheTTL)
	require.True(t, service.kimiParameterCompat(ctx.Request.Context(), ctx).settings.KimiReasoningEffortRetryEnabled)
	next, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
	require.False(t, service.kimiParameterCompat(next.Request.Context(), next).settings.KimiReasoningEffortRetryEnabled)
}

func TestKimiCompatibilityStopsAfterStreamOutput(t *testing.T) {
	body := []byte(`{"model":"kimi-k3","stream":true,"messages":[],"temperature":0.7}`)
	ctx, recorder := kimiCompatTestContext(body, "/v1/chat/completions")
	upstream := &httpUpstreamRecorder{resp: kimiCompatTestResponse(200, []byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))}
	upstream.resp.Header.Set("Content-Type", "text/event-stream")
	service := kimiCompatTestService(upstream)
	_, err := service.forwardAsRawChatCompletions(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "")
	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
	require.Contains(t, recorder.Body.String(), "partial")
	require.False(t, strings.Contains(recorder.Body.String(), "compat"))
}

type kimiCompatHookUpstream struct {
	*httpUpstreamRecorder
	afterResponse func(*http.Request, *http.Response)
}

func (upstream *kimiCompatHookUpstream) Do(request *http.Request, proxy string, accountID int64, concurrency int) (*http.Response, error) {
	StartFirstTokenAttemptFromContext(request.Context())
	response, err := upstream.httpUpstreamRecorder.Do(request, proxy, accountID, concurrency)
	if upstream.afterResponse != nil {
		upstream.afterResponse(request, response)
	}
	return response, err
}

func TestKimiCompatibilityFirstTokenCleanup(t *testing.T) {
	body := []byte(`{"model":"kimi-k3","stream":true,"messages":[],"temperature":0.7}`)
	ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
	upstream := &kimiCompatHookUpstream{httpUpstreamRecorder: &httpUpstreamRecorder{responses: []*http.Response{
		kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")),
		kimiCompatTestSuccess(true),
	}}}
	var attempts []*firstTokenAttempt
	upstream.afterResponse = func(request *http.Request, _ *http.Response) {
		attempt, ok := request.Context().Value(firstTokenAttemptContextKey{}).(*firstTokenAttempt)
		require.True(t, ok)
		attempts = append(attempts, attempt)
		if len(attempts) == 2 {
			require.Equal(t, firstTokenAttemptStopped, attempts[0].currentState())
			attempts[0].mu.Lock()
			require.True(t, attempts[0].closed)
			attempts[0].mu.Unlock()
		}
	}
	service := kimiCompatTestService(upstream)
	result, err := service.forwardAsRawChatCompletions(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "")
	require.NoError(t, err)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Len(t, attempts, 2)
	for _, attempt := range attempts {
		attempt.mu.Lock()
		require.True(t, attempt.closed)
		attempt.mu.Unlock()
	}
}

func TestKimiCompatibilityCancellationAndTimeoutWin(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprintf("timeout=%t", timeout), func(t *testing.T) {
			body := []byte(`{"model":"kimi-k3","stream":true,"messages":[],"temperature":0.7}`)
			ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
			requestCtx, cancel := context.WithCancel(ctx.Request.Context())
			defer cancel()
			ctx.Request = ctx.Request.WithContext(requestCtx)
			upstream := &kimiCompatHookUpstream{httpUpstreamRecorder: &httpUpstreamRecorder{resp: kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model"))}}
			upstream.afterResponse = func(request *http.Request, _ *http.Response) {
				attempt, ok := request.Context().Value(firstTokenAttemptContextKey{}).(*firstTokenAttempt)
				require.True(t, ok)
				if timeout {
					require.True(t, attempt.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptTimedOut))
					attempt.stopTimer()
				} else {
					cancel()
				}
			}
			service := kimiCompatTestService(upstream)
			response, _, err := service.sendCCUpstreamRequest(requestCtx, ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", body, true, "kimi-k3", "test", "", "")
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Len(t, upstream.requests, 1)
			require.Empty(t, kimiCompatTestEvents(ctx))
		})
	}
	ctx, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
	expired, cancel := context.WithDeadline(ctx.Request.Context(), time.Now().Add(-time.Second))
	defer cancel()
	ctx.Request = ctx.Request.WithContext(expired)
	service := kimiCompatTestService(&httpUpstreamRecorder{})
	_, _, err := service.sendCCUpstreamRequest(expired, ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", nil, false, "kimi-k3", "test", "", "")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestKimiCompatibilitySwitchesIndependentAndSafeBodyRead(t *testing.T) {
	for _, test := range []struct {
		name string
		body io.ReadCloser
	}{
		{"读取失败", passthroughErrReadCloser{err: io.ErrUnexpectedEOF}},
		{"截断JSON", io.NopCloser(strings.NewReader(`{"error":{"message":"field Temperature invalid, only 1 is allowed for this model"}`))},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{}, Body: test.body}}
			service := kimiCompatTestService(upstream)
			response, _, err := service.sendCCUpstreamRequest(ctx.Request.Context(), ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", []byte(`{"temperature":0.7}`), false, "kimi-k3", "test", "", "")
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Len(t, upstream.requests, 1)
		})
	}
	for enabled := kimiCompatRule(0); enabled < kimiCompatRuleCount; enabled++ {
		t.Run(enabled.String(), func(t *testing.T) {
			ctx, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")),
				kimiCompatTestSuccess(false),
			}}
			service := kimiCompatTestService(upstream)
			state := service.kimiParameterCompat(ctx.Request.Context(), ctx)
			state.settings = DefaultGatewaySettings()
			fields := []*bool{&state.settings.KimiSamplingParameterRetryEnabled, &state.settings.KimiReasoningEffortRetryEnabled, &state.settings.KimiToolChoiceRetryEnabled, &state.settings.KimiMaxCompletionTokensRetryEnabled}
			*fields[enabled] = true
			response, _, err := service.sendCCUpstreamRequest(ctx.Request.Context(), ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", []byte(`{"temperature":0.7}`), false, "kimi-k3", "test", "", "")
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			if enabled == kimiCompatSampling {
				require.Len(t, upstream.requests, 2)
			} else {
				require.Len(t, upstream.requests, 1)
			}
		})
	}
}

func TestKimiCompatibilityDoesNotAffectNativeAnthropic(t *testing.T) {
	body := []byte(`{"model":"kimi-k3","max_tokens":128,"messages":[],"temperature":0.7}`)
	ctx, _ := kimiCompatTestContext(body, "/v1/messages")
	upstream := &httpUpstreamRecorder{resp: nativeAnthropicBufferedResponse()}
	service := kimiCompatTestService(upstream)
	_, err := service.ForwardAsAnthropic(ctx.Request.Context(), ctx, nativeAnthropicTestAccount(), body, "", "")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, 0.7, gjson.GetBytes(upstream.lastBody, "temperature").Float())
	require.False(t, gjson.GetBytes(upstream.lastBody, "reasoning_effort").Exists())
	_, exists := ctx.Get(kimiParameterCompatContextKey)
	require.False(t, exists)
}

func TestKimiCompatibilityDoesNotAffectNativeResponses(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","input":"hello","temperature":0.7}`)
	ctx, _ := kimiCompatTestContext(body, "/v1/responses")
	upstream := &httpUpstreamRecorder{resp: kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model"))}
	service := kimiCompatTestService(upstream)
	account := rawChatCompletionsTestAccount()
	account.Credentials["api_protocol"] = "responses"
	account.Extra = map[string]any{"openai_passthrough": true}
	_, err := service.Forward(ctx.Request.Context(), ctx, account, body)
	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
	require.Equal(t, 0.7, gjson.GetBytes(upstream.lastBody, "temperature").Float())
	_, exists := ctx.Get(kimiParameterCompatContextKey)
	require.False(t, exists)
}

type kimiCompatBlockingErrorBody struct {
	*io.PipeReader
	started chan struct{}
}

func (body *kimiCompatBlockingErrorBody) Read(buffer []byte) (int, error) {
	select {
	case <-body.started:
	default:
		close(body.started)
	}
	return body.PipeReader.Read(buffer)
}

func TestKimiCompatibilityCancellationDuringErrorRead(t *testing.T) {
	ctx, _ := kimiCompatTestContext(nil, "/v1/chat/completions")
	requestCtx, cancel := context.WithCancel(ctx.Request.Context())
	defer cancel()
	ctx.Request = ctx.Request.WithContext(requestCtx)
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()
	errorBody := &kimiCompatBlockingErrorBody{PipeReader: reader, started: make(chan struct{})}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 400, Header: http.Header{}, Body: errorBody}}
	service := kimiCompatTestService(upstream)
	finished := make(chan error, 1)
	go func() {
		response, _, err := service.sendCCUpstreamRequest(requestCtx, ctx, kimiCompatTestAccount(), "http://upstream.example/v1/chat/completions", []byte(`{"temperature":0.7}`), false, "kimi-k3", "test", "", "")
		if response != nil {
			_ = response.Body.Close()
		}
		finished <- err
	}()
	<-errorBody.started
	cancel()
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("客户端取消后错误体读取未退出")
	}
	require.Len(t, upstream.requests, 1)
	require.Empty(t, kimiCompatTestEvents(ctx))
}
