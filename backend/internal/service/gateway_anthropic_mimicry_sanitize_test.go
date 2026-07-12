package service

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeClaudeCodeMimicryBody_MigratesOutputFormatAndOnlyDropsKnownOpenAIFields(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"output_format":{"type":"json_schema","schema":{"type":"object"}},
		"output_config":{"effort":"high"},
		"store":false,
		"user":"openai-user",
		"truncation":"auto",
		"previous_response_id":"resp_1",
		"metadata":{"user_id":"anthropic-metadata"},
		"service_tier":"auto",
		"inference_geo":"us",
		"speed":"fast",
		"temperature":0.4,
		"top_p":0.8,
		"top_k":32,
		"max_tokens":4096,
		"stop_sequences":["END"],
		"future_anthropic_field":{"enabled":true},
		"messages":[]
	}`)

	out := sanitizeClaudeCodeMimicryBody(
		body,
		"claude-sonnet-4-6",
		strings.Join([]string{claude.BetaEffort, claude.BetaFastMode}, ","),
		anthropicMimicEndpointMessages,
	)

	require.False(t, gjson.GetBytes(out, "output_format").Exists())
	require.Equal(t, "json_schema", gjson.GetBytes(out, "output_config.format.type").String())
	require.Equal(t, "object", gjson.GetBytes(out, "output_config.format.schema.type").String())
	require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
	require.False(t, gjson.GetBytes(out, "store").Exists())
	require.False(t, gjson.GetBytes(out, "user").Exists())
	require.False(t, gjson.GetBytes(out, "truncation").Exists())
	require.False(t, gjson.GetBytes(out, "previous_response_id").Exists())
	require.Equal(t, "anthropic-metadata", gjson.GetBytes(out, "metadata.user_id").String())
	require.Equal(t, "auto", gjson.GetBytes(out, "service_tier").String())
	require.Equal(t, "us", gjson.GetBytes(out, "inference_geo").String())
	require.Equal(t, "fast", gjson.GetBytes(out, "speed").String())
	require.Equal(t, 0.4, gjson.GetBytes(out, "temperature").Float())
	require.Equal(t, 0.8, gjson.GetBytes(out, "top_p").Float())
	require.Equal(t, int64(32), gjson.GetBytes(out, "top_k").Int())
	require.Equal(t, int64(4096), gjson.GetBytes(out, "max_tokens").Int())
	require.Equal(t, "END", gjson.GetBytes(out, "stop_sequences.0").String())
	require.True(t, gjson.GetBytes(out, "future_anthropic_field.enabled").Bool())
}

func TestSanitizeClaudeCodeMimicryBody_StripsCapabilitiesMissingFromFinalBeta(t *testing.T) {
	body := []byte(`{
		"context_management":{"edits":[{"type":"clear_thinking_20251015"}]},
		"output_config":{"effort":"max","format":{"type":"json_schema"}},
		"cache_control":{"type":"ephemeral","scope":"global","ttl":"1h"},
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","scope":"global","ttl":"5m"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","scope":"global","ttl":"1h"}}]}],
		"tools":[{"name":"keep","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","scope":"global","ttl":"5m"}}]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.False(t, gjson.GetBytes(out, "context_management").Exists())
	require.False(t, gjson.GetBytes(out, "output_config.effort").Exists())
	require.Equal(t, "json_schema", gjson.GetBytes(out, "output_config.format.type").String())
	for _, path := range []string{
		"cache_control.scope",
		"system.0.cache_control.scope",
		"messages.0.content.0.cache_control.scope",
		"tools.0.cache_control.scope",
	} {
		require.Falsef(t, gjson.GetBytes(out, path).Exists(), "应删除 %s", path)
	}
	require.False(t, gjson.GetBytes(out, "cache_control.ttl").Exists())
	require.False(t, gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").Exists())
	require.Equal(t, "5m", gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
}

func TestSanitizeClaudeCodeMimicryBody_PreservesCapabilitiesPresentInFinalBeta(t *testing.T) {
	finalBeta := strings.Join([]string{
		claude.BetaContextManagement,
		claude.BetaEffort,
		claude.BetaPromptCachingScope,
		claude.BetaExtendedCacheTTL,
		anthropicBetaMCPClient,
		anthropicBetaAdvancedToolUse,
		claude.BetaFineGrainedToolStreaming,
	}, ",")
	body := []byte(`{
		"context_management":{"edits":[]},
		"output_config":{"effort":"high"},
		"cache_control":{"type":"ephemeral","scope":"global","ttl":"1h"},
		"mcp_servers":[{"type":"url","url":"https://mcp.example"}],
		"messages":[],
		"tools":[
			{"type":"tool_search_tool_regex_20251119","name":"tool_search","allowed_callers":["direct"],"input_examples":[{}],"defer_loading":true,"eager_input_streaming":true},
			{"type":"mcp_toolset","name":"mcp"}
		]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", finalBeta, anthropicMimicEndpointMessages)

	require.JSONEq(t, string(body), string(out))
}

func TestSanitizeClaudeCodeMimicryBody_ValidatesCacheControlShape(t *testing.T) {
	body := []byte(`{
		"cache_control":true,
		"system":[
			{"type":"text","text":"bad type","cache_control":{"type":"persistent","ttl":"5m"}},
			{"type":"text","text":"bad ttl","cache_control":{"type":"ephemeral","ttl":"2h"}},
			{"type":"text","text":"valid default","cache_control":{"type":"ephemeral"}}
		],
		"messages":[{"role":"user","content":[
			{"type":"text","text":"valid 5m","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"valid 1h","cache_control":{"type":"ephemeral","ttl":"1h"}}
		]}]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.False(t, gjson.GetBytes(out, "cache_control").Exists())
	require.False(t, gjson.GetBytes(out, "system.0.cache_control").Exists())
	require.False(t, gjson.GetBytes(out, "system.1.cache_control").Exists())
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "system.2.cache_control.type").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "messages.0.content.1.cache_control.type").String())
	require.False(t, gjson.GetBytes(out, "messages.0.content.1.cache_control.ttl").Exists(), "缺 extended beta 时仅删除合法 1h 的 ttl")
}

func TestSanitizeClaudeCodeMimicryBody_Opus47And48UseTargetedSamplingRules(t *testing.T) {
	for _, model := range []string{"claude-opus-4-7", "claude-opus-4.8-20260601"} {
		t.Run(model, func(t *testing.T) {
			body := []byte(`{"messages":[],"temperature":0.7,"top_p":0.9,"top_k":40}`)
			out := sanitizeClaudeCodeMimicryBody(body, model, "", anthropicMimicEndpointMessages)
			require.False(t, gjson.GetBytes(out, "temperature").Exists())
			require.False(t, gjson.GetBytes(out, "top_p").Exists())
			require.False(t, gjson.GetBytes(out, "top_k").Exists())
		})
	}
}

func TestSanitizeClaudeCodeMimicryBody_Opus47KeepsSupportedSamplingValues(t *testing.T) {
	body := []byte(`{"messages":[],"temperature":1,"top_p":0.99,"top_k":40}`)
	out := sanitizeClaudeCodeMimicryBody(body, "claude-opus-4-7-20260417", "", anthropicMimicEndpointMessages)
	require.Equal(t, float64(1), gjson.GetBytes(out, "temperature").Float())
	require.Equal(t, 0.99, gjson.GetBytes(out, "top_p").Float())
	require.False(t, gjson.GetBytes(out, "top_k").Exists())
}

func TestSanitizeClaudeCodeMimicryBody_OtherModelsKeepSamplingParameters(t *testing.T) {
	body := []byte(`{"messages":[],"temperature":0.3,"top_p":0.8,"top_k":20}`)
	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)
	require.JSONEq(t, string(body), string(out))
}

func TestSanitizeClaudeCodeMimicryBody_SpeedFollowsFastModeBeta(t *testing.T) {
	body := []byte(`{"messages":[],"speed":"fast"}`)
	withoutBeta := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)
	withBeta := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", claude.BetaFastMode, anthropicMimicEndpointMessages)
	countTokens := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", claude.BetaFastMode, anthropicMimicEndpointCountTokens)
	require.False(t, gjson.GetBytes(withoutBeta, "speed").Exists())
	require.Equal(t, "fast", gjson.GetBytes(withBeta, "speed").String())
	require.False(t, gjson.GetBytes(countTokens, "speed").Exists())
}

func TestSanitizeClaudeCodeMimicryBody_RemovesUnsupportedToolsAndLinkedHistory(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"assistant","content":[
				{"type":"text","text":"before"},
				{"type":"tool_use","id":"toolu_computer","name":"computer","input":{}},
				{"type":"tool_use","id":"toolu_keep","name":"keep","input":{}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_computer","content":"removed"},
				{"type":"tool_result","tool_use_id":"toolu_keep","content":"kept"},
				{"type":"text","text":"after"}
			]}
		],
		"tools":[
			{"type":"custom","name":"keep","input_schema":{"type":"object"}},
			{"type":"function","name":"also_keep","input_schema":{"type":"object"}},
			{"type":"computer_20251124","name":"computer"},
			{"type":"tool_search_tool_regex_20251119","name":"search"},
			{"type":"mcp_toolset","name":"mcp"},
			{"type":"web_search","name":"web"},
			{"type":"image_generation","name":"image"}
		],
		"tool_choice":{"type":"tool","name":"computer"}
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	tools := gjson.GetBytes(out, "tools").Array()
	require.Len(t, tools, 2)
	require.Equal(t, "keep", tools[0].Get("name").String())
	require.Equal(t, "also_keep", tools[1].Get("name").String())
	require.False(t, gjson.GetBytes(out, "tool_choice").Exists())

	assistantContent := gjson.GetBytes(out, "messages.0.content").Array()
	require.Len(t, assistantContent, 2)
	require.Equal(t, "text", assistantContent[0].Get("type").String())
	require.Equal(t, "toolu_keep", assistantContent[1].Get("id").String())
	userContent := gjson.GetBytes(out, "messages.1.content").Array()
	require.Len(t, userContent, 2)
	require.Equal(t, "toolu_keep", userContent[0].Get("tool_use_id").String())
	require.Equal(t, "text", userContent[1].Get("type").String())
	require.NotContains(t, string(out), "removed")
	require.NotContains(t, string(out), "(tool_use)")
}

func TestSanitizeClaudeCodeMimicryBody_DropsMessagesLeftEmptyByToolRemoval(t *testing.T) {
	body := []byte(`{
		"tools":[{"type":"computer_20251124","name":"computer"}],
		"messages":[
			{"role":"assistant","content":[{"type":"server_tool_use","id":"srv_1","name":"computer","input":{}}]},
			{"role":"user","content":[{"type":"computer_tool_result","tool_use_id":"srv_1","content":"x"}]},
			{"role":"user","content":[{"type":"text","text":"keep"}]}
		]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Len(t, gjson.GetBytes(out, "messages").Array(), 1)
	require.Equal(t, "keep", gjson.GetBytes(out, "messages.0.content.0.text").String())
}

func TestSanitizeClaudeCodeMimicryBody_AdvancedToolFieldsFollowBeta(t *testing.T) {
	body := []byte(`{
		"messages":[],
		"tools":[{
			"type":"custom",
			"name":"keep",
			"description":"desc",
			"input_schema":{"type":"object"},
			"strict":true,
			"allowed_callers":["direct"],
			"input_examples":[{"q":"x"}],
			"defer_loading":true,
			"eager_input_streaming":true,
			"custom":{"defer_loading":true},
			"cache_control":{"type":"ephemeral","ttl":"5m"}
		}]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.True(t, gjson.GetBytes(out, "tools.0.strict").Bool())
	require.Equal(t, "desc", gjson.GetBytes(out, "tools.0.description").String())
	require.Equal(t, "object", gjson.GetBytes(out, "tools.0.input_schema.type").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
	for _, path := range []string{
		"tools.0.allowed_callers",
		"tools.0.input_examples",
		"tools.0.defer_loading",
		"tools.0.eager_input_streaming",
		"tools.0.custom",
	} {
		require.Falsef(t, gjson.GetBytes(out, path).Exists(), "应删除 %s", path)
	}
}

func TestSanitizeClaudeCodeMimicryBody_RemovesMalformedOrdinaryToolsAndHistory(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_missing_schema","name":"missing_schema","input":{}},
				{"type":"tool_use","id":"toolu_bad_schema","name":"bad_schema","input":{}},
				{"type":"tool_use","id":"toolu_keep","name":"keep","input":{}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_missing_schema","content":"remove 1"},
				{"type":"tool_result","tool_use_id":"toolu_bad_schema","content":"remove 2"},
				{"type":"tool_result","tool_use_id":"toolu_keep","content":"keep"}
			]}
		],
		"tools":[
			{"name":"missing_schema"},
			{"type":"custom","name":"bad_schema","input_schema":[]},
			{"type":"custom","input_schema":{"type":"object"}},
			{"type":"function","name":"keep","input_schema":{"type":"object"}}
		],
		"tool_choice":{"type":"tool","name":"missing_schema"}
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Len(t, gjson.GetBytes(out, "tools").Array(), 1)
	require.Equal(t, "keep", gjson.GetBytes(out, "tools.0.name").String())
	require.False(t, gjson.GetBytes(out, "tool_choice").Exists())
	require.Len(t, gjson.GetBytes(out, "messages.0.content").Array(), 1)
	require.Equal(t, "toolu_keep", gjson.GetBytes(out, "messages.0.content.0.id").String())
	require.Len(t, gjson.GetBytes(out, "messages.1.content").Array(), 1)
	require.Equal(t, "toolu_keep", gjson.GetBytes(out, "messages.1.content.0.tool_use_id").String())
}

func TestSanitizeClaudeCodeMimicryBody_CountTokensUsesDedicatedFieldsAndTools(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[],
		"system":[{"type":"text","text":"sys"}],
		"tools":[
			{"type":"custom","name":"keep","input_schema":{"type":"object"}},
			{"type":"computer_20251124","name":"computer"},
			{"type":"tool_search_tool_regex_20251119","name":"tool_search"},
			{"type":"mcp_toolset","name":"mcp"}
		],
		"tool_choice":{"type":"tool","name":"computer"},
		"thinking":{"type":"adaptive"},
		"output_config":{"effort":"high"},
		"cache_control":{"type":"ephemeral","ttl":"5m"},
		"metadata":{"user_id":"x"},
		"max_tokens":4096,
		"temperature":0.5,
		"top_p":0.9,
		"top_k":40,
		"stream":true,
		"stop_sequences":["END"],
		"context_management":{"edits":[]},
		"container":{"id":"container_1"},
		"service_tier":"auto",
		"speed":"fast",
		"mcp_servers":[]
	}`)
	finalBeta := strings.Join([]string{
		claude.BetaEffort,
		claude.BetaContextManagement,
		"computer-use-2025-11-24",
		anthropicBetaMCPClient,
		anthropicBetaAdvancedToolUse,
	}, ",")

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", finalBeta, anthropicMimicEndpointCountTokens)

	for _, path := range anthropicCountTokensUnsupportedFields {
		require.Falsef(t, gjson.GetBytes(out, path).Exists(), "Count Tokens 应删除 %s", path)
	}
	require.False(t, gjson.GetBytes(out, "speed").Exists())
	require.True(t, gjson.GetBytes(out, "mcp_servers").Exists())
	require.True(t, gjson.GetBytes(out, "context_management").Exists())
	require.Equal(t, "sys", gjson.GetBytes(out, "system.0.text").String())
	require.Equal(t, "adaptive", gjson.GetBytes(out, "thinking.type").String())
	require.Equal(t, "high", gjson.GetBytes(out, "output_config.effort").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "cache_control.ttl").String())
	require.Len(t, gjson.GetBytes(out, "tools").Array(), 4)
	require.Equal(t, "keep", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "computer", gjson.GetBytes(out, "tool_choice.name").String())
}

func TestSanitizeClaudeCodeMimicryBody_FlattensNestedOpenAIFunctionTool(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_lookup","name":"lookup","input":{"q":"x"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_lookup","content":"ok"}]}
		],
		"tools":[
			{
				"type":"function",
				"function":{
					"name":"lookup",
					"description":"Lookup a value",
					"parameters":{"required":["q"],"properties":{"q":{"type":"string"}}},
					"strict":true,
					"allowed_callers":["direct"]
				},
				"cache_control":{"type":"ephemeral","ttl":"5m"}
			},
			{"type":"custom","name":"keep","input_schema":{"type":"object","properties":{}}}
		],
		"tool_choice":{"type":"tool","name":"lookup"}
	}`)
	finalBeta := anthropicBetaAdvancedToolUse

	out1 := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", finalBeta, anthropicMimicEndpointMessages)
	out2 := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", finalBeta, anthropicMimicEndpointMessages)

	require.Equal(t, out1, out2, "扁平化结果必须确定性稳定")
	require.Len(t, gjson.GetBytes(out1, "tools").Array(), 2)
	require.False(t, gjson.GetBytes(out1, "tools.0.type").Exists())
	require.False(t, gjson.GetBytes(out1, "tools.0.function").Exists())
	require.Equal(t, "lookup", gjson.GetBytes(out1, "tools.0.name").String())
	require.Equal(t, "Lookup a value", gjson.GetBytes(out1, "tools.0.description").String())
	require.Equal(t, "object", gjson.GetBytes(out1, "tools.0.input_schema.type").String())
	require.Equal(t, "string", gjson.GetBytes(out1, "tools.0.input_schema.properties.q.type").String())
	require.True(t, gjson.GetBytes(out1, "tools.0.strict").Bool())
	require.Equal(t, "direct", gjson.GetBytes(out1, "tools.0.allowed_callers.0").String())
	require.Equal(t, "5m", gjson.GetBytes(out1, "tools.0.cache_control.ttl").String())
	require.Equal(t, "lookup", gjson.GetBytes(out1, "tool_choice.name").String())
	require.Equal(t, "toolu_lookup", gjson.GetBytes(out1, "messages.0.content.0.id").String())
}

func TestSanitizeClaudeCodeMimicryBody_ConvertsNestedOpenAIFunctionToolChoice(t *testing.T) {
	body := []byte(`{
		"messages":[],
		"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],
		"tool_choice":{"type":"function","function":{"name":"lookup"},"disable_parallel_tool_use":true}
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Equal(t, "lookup", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "tool", gjson.GetBytes(out, "tool_choice.type").String())
	require.Equal(t, "lookup", gjson.GetBytes(out, "tool_choice.name").String())
	require.True(t, gjson.GetBytes(out, "tool_choice.disable_parallel_tool_use").Bool())
	require.False(t, gjson.GetBytes(out, "tool_choice.function").Exists())
}

func TestSanitizeClaudeCodeMimicryBody_ConvertsOpenAIStringToolChoice(t *testing.T) {
	tests := []struct {
		name         string
		choice       string
		expectedType string
	}{
		{name: "auto", choice: "auto", expectedType: "auto"},
		{name: "required", choice: "required", expectedType: "any"},
		{name: "none", choice: "none", expectedType: "none"},
		{name: "unknown", choice: "legacy", expectedType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{
				"messages":[],
				"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
				"tool_choice":"` + tt.choice + `"
			}`)

			out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)
			choice := gjson.GetBytes(out, "tool_choice")
			if tt.expectedType == "" {
				require.False(t, choice.Exists())
				return
			}
			require.True(t, choice.IsObject())
			require.Equal(t, tt.expectedType, choice.Get("type").String())
		})
	}

	withoutTools := sanitizeClaudeCodeMimicryBody(
		[]byte(`{"messages":[],"tool_choice":"auto"}`),
		"claude-sonnet-4-6",
		"",
		anthropicMimicEndpointMessages,
	)
	require.False(t, gjson.GetBytes(withoutTools, "tool_choice").Exists())
}

func TestSanitizeClaudeCodeMimicryBody_RemovesServerHistoryWhenCustomToolHasSameName(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"assistant","content":[
				{"type":"server_tool_use","id":"srv_shared","name":"shared","input":{}},
				{"type":"tool_use","id":"toolu_shared","name":"shared","input":{}},
				{"type":"text","text":"keep"}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"srv_shared","content":"remove server result"},
				{"type":"tool_result","tool_use_id":"toolu_shared","content":"keep custom result"}
			]}
		],
		"tools":[
			{"type":"computer_20251124","name":"shared"},
			{"type":"custom","name":"shared","input_schema":{"type":"object"}}
		],
		"tool_choice":{"type":"tool","name":"shared"}
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Len(t, gjson.GetBytes(out, "tools").Array(), 1)
	require.Equal(t, "custom", gjson.GetBytes(out, "tools.0.type").String())
	require.Equal(t, "shared", gjson.GetBytes(out, "tool_choice.name").String())
	require.Len(t, gjson.GetBytes(out, "messages.0.content").Array(), 2)
	require.Equal(t, "toolu_shared", gjson.GetBytes(out, "messages.0.content.0.id").String())
	require.Equal(t, "keep", gjson.GetBytes(out, "messages.0.content.1.text").String())
	require.Len(t, gjson.GetBytes(out, "messages.1.content").Array(), 1)
	require.Equal(t, "toolu_shared", gjson.GetBytes(out, "messages.1.content.0.tool_use_id").String())
	require.NotContains(t, string(out), "srv_shared")
	require.NotContains(t, string(out), "remove server result")
}

func TestSanitizeClaudeCodeMimicryBody_RevalidatesFlattenedFunctionCacheControl(t *testing.T) {
	body := []byte(`{
		"messages":[],
		"tools":[{"type":"function","function":{
			"name":"lookup",
			"parameters":{"type":"object"},
			"cache_control":{"type":"ephemeral","scope":"global","ttl":"1h"}
		}}]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Equal(t, "lookup", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "tools.0.cache_control.type").String())
	require.False(t, gjson.GetBytes(out, "tools.0.cache_control.scope").Exists())
	require.False(t, gjson.GetBytes(out, "tools.0.cache_control.ttl").Exists())
}

func TestSanitizeClaudeCodeMimicryBody_RemovesInvalidNestedFunctionAndHistory(t *testing.T) {
	body := []byte(`{
		"messages":[
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_bad","name":"bad","input":{}},
				{"type":"tool_use","id":"toolu_keep","name":"keep","input":{}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_bad","content":"bad result"},
				{"type":"tool_result","tool_use_id":"toolu_keep","content":"keep result"}
			]}
		],
		"tools":[
			{"type":"function","function":{"name":"bad","description":"missing parameters"}},
			{"type":"custom","name":"keep","input_schema":{"type":"object"}}
		],
		"tool_choice":{"type":"tool","name":"bad"}
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Len(t, gjson.GetBytes(out, "tools").Array(), 1)
	require.Equal(t, "keep", gjson.GetBytes(out, "tools.0.name").String())
	require.False(t, gjson.GetBytes(out, "tool_choice").Exists())
	require.Len(t, gjson.GetBytes(out, "messages.0.content").Array(), 1)
	require.Equal(t, "toolu_keep", gjson.GetBytes(out, "messages.0.content.0.id").String())
	require.Len(t, gjson.GetBytes(out, "messages.1.content").Array(), 1)
	require.Equal(t, "toolu_keep", gjson.GetBytes(out, "messages.1.content.0.tool_use_id").String())
	require.NotContains(t, string(out), "bad result")
}

func TestSanitizeClaudeCodeMimicryBody_InvalidJSONIsNoop(t *testing.T) {
	body := []byte(`{"messages":[`)
	require.Equal(t, body, sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages))
}

func TestSanitizeClaudeCodeMimicryBody_NamelessNestedFunctionRemovesToolChoice(t *testing.T) {
	body := []byte(`{
		"messages":[],
		"tools":[{"type":"function","function":{}}],
		"tool_choice":{"type":"tool","name":"missing"}
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.Empty(t, gjson.GetBytes(out, "tools").Array())
	require.False(t, gjson.GetBytes(out, "tool_choice").Exists())
}

func TestSanitizeClaudeCodeMimicryBody_RemovesMCPHistoryWithoutBeta(t *testing.T) {
	body := []byte(`{
		"mcp_servers":[{"type":"url","url":"https://mcp.example"}],
		"messages":[
			{"role":"assistant","content":[{"type":"text","text":"keep"},{"type":"mcp_tool_use","id":"mcp_1","name":"remote_lookup","input":{}}]},
			{"role":"user","content":[{"type":"mcp_tool_result","tool_use_id":"mcp_1","content":"remove"}]}
		]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", "", anthropicMimicEndpointMessages)

	require.False(t, gjson.GetBytes(out, "mcp_servers").Exists())
	require.Len(t, gjson.GetBytes(out, "messages").Array(), 1)
	require.Equal(t, "keep", gjson.GetBytes(out, "messages.0.content.0.text").String())
	require.NotContains(t, string(out), "mcp_tool_use")
	require.NotContains(t, string(out), "mcp_tool_result")
}

func TestSanitizeClaudeCodeMimicryBody_RemovesOpenAIMCPShapeEvenWithBeta(t *testing.T) {
	body := []byte(`{
		"tools":[{"type":"mcp","server_label":"remote"}],
		"messages":[
			{"role":"assistant","content":[{"type":"mcp_tool_use","id":"mcp_1","name":"lookup","input":{}}]},
			{"role":"user","content":[{"type":"mcp_tool_result","tool_use_id":"mcp_1","content":"remove"}]},
			{"role":"user","content":[{"type":"text","text":"keep"}]}
		]
	}`)

	out := sanitizeClaudeCodeMimicryBody(body, "claude-sonnet-4-6", anthropicBetaMCPClient, anthropicMimicEndpointMessages)

	require.Empty(t, gjson.GetBytes(out, "tools").Array())
	require.Len(t, gjson.GetBytes(out, "messages").Array(), 1)
	require.Equal(t, "keep", gjson.GetBytes(out, "messages.0.content.0.text").String())
}

func TestEnforceCacheControlLimit_CountsTopLevelAndPreservesPriority(t *testing.T) {
	body := []byte(`{
		"cache_control":{"type":"ephemeral","ttl":"1h"},
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"messages":[{"role":"user","content":[
			{"type":"text","text":"old","cache_control":{"type":"ephemeral","ttl":"1h"}},
			{"type":"text","text":"middle","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"new","cache_control":{"type":"ephemeral","ttl":"1h"}}
		]}],
		"tools":[{"name":"tool","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"5m"}}]
	}`)

	out := enforceCacheControlLimit(body)

	require.Equal(t, maxCacheControlBlocks, strings.Count(string(out), `"cache_control"`))
	require.Equal(t, "1h", gjson.GetBytes(out, "cache_control.ttl").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.False(t, gjson.GetBytes(out, "messages.0.content.0.cache_control").Exists())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.1.cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.2.cache_control.ttl").String())
	require.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists())
}

func TestAddMessageCacheBreakpointsPreservingClient_DoesNotRewriteExplicitTTL(t *testing.T) {
	body := []byte(`{
		"cache_control":{"type":"ephemeral","ttl":"5m"},
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"first","cache_control":{"type":"ephemeral","ttl":"1h"}}]},
			{"role":"assistant","content":[{"type":"text","text":"answer"}]},
			{"role":"user","content":[{"type":"text","text":"follow-up"}]},
			{"role":"assistant","content":[{"type":"text","text":"latest"}]}
		]
	}`)

	out := addMessageCacheBreakpointsPreservingClient(body)

	require.Equal(t, maxCacheControlBlocks, cacheControlBlockCount(out))
	require.Equal(t, "5m", gjson.GetBytes(out, "cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.3.content.0.cache_control.ttl").String())
	require.False(t, gjson.GetBytes(out, "messages.2.content.0.cache_control").Exists(), "配额用尽后不得挤掉客户端断点")
}
