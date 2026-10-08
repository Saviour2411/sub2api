//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const kimiDynamicWeatherTool = `{"type":"function","function":{"name":"get_weather","description":"查询城市天气","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}`

func kimiDynamicTestBody(messages, global string) []byte {
	body := `{"model":"kimi-k3","messages":` + messages + `,"tool_choice":"required","max_tokens":512,"extra":{"number":9007199254740993}}`
	if global != "" {
		body = strings.TrimSuffix(body, "}") + `,"tools":` + global + `}`
	}
	return []byte(body)
}

func TestKimiDynamicToolsNormalization(t *testing.T) {
	declaration := `{"role":"system","content":"","tools":[` + kimiDynamicWeatherTool + `]}`
	user := `{"role":"user","content":"查询北京天气"}`
	second := strings.ReplaceAll(declaration, "get_weather", "get_time")
	for _, test := range []struct {
		name, messages, global  string
		messageCount, toolCount int
	}{
		{"空正文", `[` + declaration + `,` + user + `]`, "", 1, 1},
		{"无正文", `[` + strings.Replace(declaration, `"content":"",`, "", 1) + `,` + user + `]`, "", 1, 1},
		{"空值正文", `[` + strings.Replace(declaration, `"content":""`, `"content":null`, 1) + `,` + user + `]`, "", 1, 1},
		{"中途声明", `[{"role":"system","content":"保留指令"},` + user + `,{"role":"assistant","content":"你好","reasoning_content":"保留思考"},` + declaration + `,` + user + `]`, "", 4, 1},
		{"末尾声明", `[` + user + `,` + declaration + `]`, "", 1, 1},
		{"多条声明", `[` + declaration + `,` + second + `,` + user + `]`, "", 1, 2},
		{"全局共存", `[` + declaration + `,` + user + `]`, `[` + strings.ReplaceAll(kimiDynamicWeatherTool, "get_weather", "get_stock_price") + `]`, 1, 2},
		{"严格模式关闭", `[` + strings.Replace(declaration, `"name":"get_weather"`, `"strict":false,"name":"get_weather"`, 1) + `,` + user + `]`, "", 1, 1},
		{"无参数函数", `[{"role":"system","tools":[{"type":"function","function":{"name":"get_weather"}}]},` + user + `]`, "", 1, 1},
		{"空列表", `[{"role":"system","tools":[]},` + user + `]`, "", 1, 0},
		{"扩展元数据保留", `[` + strings.Replace(declaration, `"role":"system"`, `"role":"system","metadata":{"number":9007199254740993}`, 1) + `,` + user + `]`, "", 2, 1},
		{"无正文扩展元数据", `[{"role":"system","tools":[` + kimiDynamicWeatherTool + `],"name":"声明"},` + user + `]`, "", 2, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := kimiDynamicTestBody(test.messages, test.global)
			original := string(body)
			updated, err := normalizeKimiDynamicTools(body)
			require.NoError(t, err)
			require.Len(t, gjson.GetBytes(updated, "messages").Array(), test.messageCount)
			require.Len(t, gjson.GetBytes(updated, "tools").Array(), test.toolCount)
			require.Equal(t, "required", gjson.GetBytes(updated, "tool_choice").String())
			require.Equal(t, "9007199254740993", gjson.GetBytes(updated, "extra.number").Raw)
			for _, message := range gjson.GetBytes(updated, "messages").Array() {
				require.False(t, message.Get("tools").Exists())
			}
			if test.global != "" {
				require.Equal(t, "get_stock_price", gjson.GetBytes(updated, "tools.0.function.name").String())
			}
			repeated, err := normalizeKimiDynamicTools(updated)
			require.NoError(t, err)
			require.Equal(t, string(updated), string(repeated))
			require.Equal(t, original, string(body))
		})
	}
}

func TestKimiDynamicToolsRejectInvalidContracts(t *testing.T) {
	valid := kimiDynamicTestBody(`[{"role":"system","content":"","tools":[`+kimiDynamicWeatherTool+`]},{"role":"user","content":"你好"}]`, "")
	for _, test := range []struct {
		name, path, raw string
	}{
		{"用户声明", "messages.0.role", `"user"`},
		{"助手声明", "messages.0.role", `"assistant"`},
		{"开发者声明不能提升权限", "messages.0.role", `"developer"`},
		{"正文共存", "messages.0.content", `"not empty"`},
		{"空白也是正文", "messages.0.content", `" "`},
		{"正文数组", "messages.0.content", `[{"type":"text","text":"正文"}]`},
		{"非法正文类型", "messages.0.content", `0`},
		{"非数组工具", "messages.0.tools", `{}`},
		{"空值工具", "messages.0.tools", `null`},
		{"非对象工具项", "messages.0.tools", `[null]`},
		{"无工具类型", "messages.0.tools.0", `{"function":{"name":"valid"}}`},
		{"非法工具类型", "messages.0.tools.0.type", `"bogus"`},
		{"无函数", "messages.0.tools.0", `{"type":"function"}`},
		{"函数非对象", "messages.0.tools.0.function", `[]`},
		{"无函数名", "messages.0.tools.0.function", `{}`},
		{"空函数名", "messages.0.tools.0.function.name", `""`},
		{"数字开头", "messages.0.tools.0.function.name", `"1bad_name"`},
		{"特殊字符", "messages.0.tools.0.function.name", `"bad@name"`},
		{"超长函数名", "messages.0.tools.0.function.name", `"` + strings.Repeat("a", 257) + `"`},
		{"参数非对象", "messages.0.tools.0.function.parameters", `[]`},
		{"严格标记非布尔", "messages.0.tools.0.function.strict", `"false"`},
		{"说明非字符串", "messages.0.tools.0.function.description", `7`},
		{"声明内重名", "messages.0.tools", `[` + kimiDynamicWeatherTool + `,` + kimiDynamicWeatherTool + `]`},
		{"跨消息重名", "messages.1", `{"role":"system","tools":[` + kimiDynamicWeatherTool + `]}`},
		{"与顶层重名", "tools", `[` + kimiDynamicWeatherTool + `]`},
		{"顶层格式非法", "tools", `{}`},
		{"无调用编号", "messages.1", `{"role":"tool","content":"结果"}`},
		{"空调用编号", "messages.1", `{"role":"tool","tool_call_id":" ","content":"结果"}`},
		{"消息非对象", "messages.0", `null`},
		{"消息角色缺失", "messages.0", `{"tools":[]}`},
		{"消息重复字段", "messages.0", `{"role":"system","role":"user","tools":[]}`},
		{"大小写歧义", "messages.0", `{"role":"system","Tools":[],"tools":[]}`},
		{"函数重复字段", "messages.0.tools.0.function", `{"name":"valid","name":"other"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := sjson.SetRawBytes(valid, test.path, []byte(test.raw))
			require.NoError(t, err)
			updated, err := normalizeKimiDynamicTools(body)
			require.Error(t, err)
			require.Nil(t, updated)
		})
	}
}

func TestKimiDynamicToolsPreserveOrdinaryMessages(t *testing.T) {
	for _, body := range []string{
		`{"model":"kimi-k3","messages":[{"role":"assistant","content":"你好"}]}`,
		`{"model":"kimi-k3","messages":[{"role":"tool","content":"天气结果","tool_call_id":"call_1"}]}`,
		`{"model":"kimi-k3","messages":[{"role":"system","content":"指令"},{"role":"user","content":"问题"}],"tools":[{"type":"builtin_function","function":{"name":"$web_search"}}]}`,
		`{"model":"kimi-k3","input":"保持 Responses 形状原样"}`,
	} {
		updated, err := normalizeKimiDynamicTools([]byte(body))
		require.NoError(t, err)
		require.Equal(t, body, string(updated))
	}
}

func TestKimiDynamicToolsSchemaAndHistoryUnchanged(t *testing.T) {
	body := []byte(`{"model":"kimi-k3","messages":[{"role":"system","tools":[{"type":"function","function":{"name":"nested","strict":false,"parameters":{"type":"object","properties":{"location":{"type":"object","properties":{"lat":{"type":"number","maximum":9007199254740993}}}}}}}]},{"role":"assistant","content":null,"reasoning_content":"保留思考","tool_calls":[{"id":"call_1","type":"function","function":{"name":"nested","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"结果"},{"role":"user","content":"继续"}],"tool_choice":"none"}`)
	updated, err := normalizeKimiDynamicTools(body)
	require.NoError(t, err)
	require.JSONEq(t, gjson.GetBytes(body, "messages.0.tools.0").Raw, gjson.GetBytes(updated, "tools.0").Raw)
	for i := 0; i < 3; i++ {
		require.JSONEq(t, gjson.GetBytes(body, fmt.Sprintf("messages.%d", i+1)).Raw, gjson.GetBytes(updated, fmt.Sprintf("messages.%d", i)).Raw)
	}
	require.Equal(t, "none", gjson.GetBytes(updated, "tool_choice").String())
}

func TestKimiDynamicToolsScopeAndSnapshot(t *testing.T) {
	body := kimiDynamicTestBody(`[{"role":"user","content":"非法声明","tools":[`+kimiDynamicWeatherTool+`]}]`, "")
	for _, platform := range []string{PlatformKimi, PlatformOpenAI, PlatformComposite, PlatformDeepseek} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", platform, enabled), func(t *testing.T) {
				ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
				ctx.Set("api_key", &APIKey{Group: &Group{Platform: platform}})
				svc := kimiCompatTestService(&httpUpstreamRecorder{})
				settings := DefaultGatewaySettings()
				settings.KimiDynamicToolsEnabled = enabled
				svc.settingService.storeGatewaySettingsCache(settings, gatewaySettingsCacheTTL)
				updated, err := svc.PrepareKimiDynamicTools(context.Background(), ctx, body)
				if platform == PlatformKimi && enabled {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, string(body), string(updated))
				}
				settings.KimiDynamicToolsEnabled = !enabled
				svc.settingService.storeGatewaySettingsCache(settings, gatewaySettingsCacheTTL)
				_, again := svc.PrepareKimiDynamicTools(context.Background(), ctx, body)
				require.Equal(t, err != nil, again != nil, "同一请求冻结开关")
			})
		}
	}
}

func kimiDynamicToolSuccess(stream bool) *http.Response {
	response := `{"id":"chatcmpl_dynamic","object":"chat.completion","model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Beijing\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":25,"completion_tokens":7,"total_tokens":32}}`
	if stream {
		response = "data: {\"id\":\"chatcmpl_dynamic\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k3\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"Beijing\\\"}\"}}]},\"finish_reason\":null}]}\n\n" +
			"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":25,\"completion_tokens\":7,\"total_tokens\":32}}\n\ndata: [DONE]\n\n"
	}
	result := kimiCompatTestResponse(200, []byte(response))
	if stream {
		result.Header.Set("Content-Type", "text/event-stream")
	}
	return result
}

func TestKimiDynamicToolsForwardAndRetry(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("流式=%t", stream), func(t *testing.T) {
			body := kimiDynamicTestBody(`[{"role":"system","content":"","tools":[`+kimiDynamicWeatherTool+`]},{"role":"user","content":"查询北京天气"}]`, "")
			body, err := sjson.SetBytes(body, "stream", stream)
			require.NoError(t, err)
			body, err = sjson.SetBytes(body, "temperature", 0.5)
			require.NoError(t, err)
			ctx, recorder := kimiCompatTestContext(body, "/v1/chat/completions")
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")), kimiDynamicToolSuccess(stream),
			}}
			svc := kimiCompatTestService(upstream)
			svc.kimiParameterCompat(ctx.Request.Context(), ctx).settings.KimiDynamicToolsEnabled = true
			result, err := svc.ForwardAsChatCompletions(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "", "")
			require.NoError(t, err)
			require.Equal(t, 200, recorder.Code)
			require.Len(t, upstream.requests, 2)
			require.Equal(t, 25, result.Usage.InputTokens, "不得修正 Token 基准")
			require.Equal(t, 7, result.Usage.OutputTokens)
			require.Contains(t, recorder.Body.String(), `"finish_reason":"tool_calls"`)
			require.Contains(t, recorder.Body.String(), `"name":"get_weather"`)
			if stream {
				require.Contains(t, recorder.Body.String(), "data: [DONE]")
			}
			for _, outgoing := range upstream.bodies {
				require.Len(t, gjson.GetBytes(outgoing, "tools").Array(), 1)
				require.Len(t, gjson.GetBytes(outgoing, "messages").Array(), 1)
				require.Equal(t, "required", gjson.GetBytes(outgoing, "tool_choice").String())
				require.JSONEq(t, kimiDynamicWeatherTool, gjson.GetBytes(outgoing, "tools.0").Raw)
			}
			require.True(t, gjson.GetBytes(body, "messages.0.tools").Exists())
		})
	}
}

func TestKimiDynamicToolsProtocolBridges(t *testing.T) {
	for _, protocol := range []string{APIProtocolResponses, APIProtocolAnthropic, APIProtocolAdaptive} {
		t.Run(protocol, func(t *testing.T) {
			body := kimiDynamicTestBody(`[{"role":"system","content":"","tools":[`+kimiDynamicWeatherTool+`]},{"role":"user","content":"查询北京天气"}]`, "")
			ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				kimiCompatTestResponse(400, kimiTestError("测试在上游边界终止")),
			}}
			svc := kimiCompatTestService(upstream)
			svc.kimiParameterCompat(ctx.Request.Context(), ctx).settings.KimiDynamicToolsEnabled = true
			account := kimiCompatTestAccount()
			account.Credentials["api_protocol"] = protocol
			_, err := svc.ForwardAsChatCompletions(ctx.Request.Context(), ctx, account, body, "", "")
			require.Error(t, err)
			require.Len(t, upstream.requests, 1)
			outgoing := upstream.bodies[0]
			require.Len(t, gjson.GetBytes(outgoing, "tools").Array(), 1)
			switch protocol {
			case APIProtocolResponses:
				require.Equal(t, "get_weather", gjson.GetBytes(outgoing, "tools.0.name").String())
				require.Equal(t, "required", gjson.GetBytes(outgoing, "tool_choice").String())
			case APIProtocolAnthropic:
				require.Equal(t, "get_weather", gjson.GetBytes(outgoing, "tools.0.name").String())
				require.Equal(t, "any", gjson.GetBytes(outgoing, "tool_choice.type").String())
			default:
				require.Equal(t, "get_weather", gjson.GetBytes(outgoing, "tools.0.function.name").String())
			}
		})
	}
}

func TestKimiDynamicToolsDisabledPassthrough(t *testing.T) {
	body := kimiDynamicTestBody(`[{"role":"system","content":"非空正文","tools":[`+kimiDynamicWeatherTool+`]},{"role":"user","content":"问题"}]`, "")
	ctx, _ := kimiCompatTestContext(body, "/v1/chat/completions")
	upstream := &httpUpstreamRecorder{responses: []*http.Response{kimiDynamicToolSuccess(false)}}
	svc := kimiCompatTestService(upstream)
	_, err := svc.ForwardAsChatCompletions(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "", "")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.JSONEq(t, string(body), string(upstream.bodies[0]))
}

func TestKimiDynamicToolsFailoverKeepsMappingAndOneDeclaration(t *testing.T) {
	body := kimiDynamicTestBody(`[{"role":"system","tools":[`+kimiDynamicWeatherTool+`]},{"role":"user","content":"查询北京天气"}]`, "")
	ctx, recorder := kimiCompatTestContext(body, "/v1/chat/completions")
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		kimiCompatTestResponse(503, kimiTestError("上游暂不可用")), kimiDynamicToolSuccess(false),
	}}
	svc := kimiCompatTestService(upstream)
	svc.kimiParameterCompat(ctx.Request.Context(), ctx).settings.KimiDynamicToolsEnabled = true
	first := kimiCompatTestAccount()
	first.Credentials["model_mapping"] = map[string]any{"kimi-k3": "channel-first"}
	_, err := svc.ForwardAsChatCompletions(ctx.Request.Context(), ctx, first, body, "", "")
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.False(t, ctx.Writer.Written())
	second := kimiCompatTestAccount()
	second.ID = first.ID + 1
	second.Credentials["model_mapping"] = map[string]any{"kimi-k3": "channel-second"}
	result, err := svc.ForwardAsChatCompletions(ctx.Request.Context(), ctx, second, body, "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 25, result.Usage.InputTokens)
	require.Len(t, upstream.requests, 2)
	for index, model := range []string{"channel-first", "channel-second"} {
		require.Equal(t, model, gjson.GetBytes(upstream.bodies[index], "model").String())
		require.Len(t, gjson.GetBytes(upstream.bodies[index], "tools").Array(), 1)
	}
}

func TestKimiDynamicToolsRejectBeforeUpstream(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, role := range []string{"user", "assistant", "developer"} {
			t.Run(fmt.Sprintf("%s/流式=%t", role, stream), func(t *testing.T) {
				message, _ := json.Marshal(map[string]any{"role": role, "content": "正文", "tools": []any{}})
				body := kimiDynamicTestBody(`[`+string(message)+`]`, "")
				body, err := sjson.SetBytes(body, "stream", stream)
				require.NoError(t, err)
				ctx, recorder := kimiCompatTestContext(body, "/v1/chat/completions")
				upstream := &httpUpstreamRecorder{}
				svc := kimiCompatTestService(upstream)
				svc.kimiParameterCompat(ctx.Request.Context(), ctx).settings.KimiDynamicToolsEnabled = true
				result, err := svc.ForwardAsChatCompletions(ctx.Request.Context(), ctx, kimiCompatTestAccount(), body, "", "")
				require.Error(t, err)
				require.Nil(t, result)
				require.Empty(t, upstream.requests)
				require.Equal(t, 400, recorder.Code)
				require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
				require.NotContains(t, recorder.Body.String(), "data:")
			})
		}
	}
}
