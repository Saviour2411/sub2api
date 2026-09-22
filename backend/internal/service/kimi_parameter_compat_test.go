//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func kimiTestError(message string) []byte {
	body, err := json.Marshal(map[string]any{"error": kimiParameterError{Message: message, Type: "invalid_request_error"}})
	if err != nil {
		panic(err)
	}
	return body
}

func TestKimiParameterRejectionMatching(t *testing.T) {
	tests := []struct {
		name    string
		request string
		message string
		rule    kimiCompatRule
		matched bool
	}{
		{"百炼温度", `{"temperature":0.7}`, "<400> ***.***.InvalidParameter: Parameter 'temperature'=0.7 is not supported for kimi-k3 model.", kimiCompatSampling, true},
		{"旧渠道温度", `{"temperature":0.7}`, "field Temperature invalid, only 1 is allowed for this model", kimiCompatSampling, true},
		{"旧渠道核采样", `{"top_p":0.7}`, "field TopP invalid, only 0.95 is allowed for this model", kimiCompatSampling, true},
		{"旧渠道存在惩罚", `{"presence_penalty":0.5}`, "field PresencePenalty invalid, only 0 is allowed for this model", kimiCompatSampling, true},
		{"旧渠道频率惩罚", `{"frequency_penalty":0.5}`, "field FrequencyPenalty invalid, only 0 is allowed for this model", kimiCompatSampling, true},
		{"数值类型", `{"temperature":"hot"}`, "'temperature' must be Float", kimiCompatSampling, true},
		{"数值范围", `{"temperature":3}`, "<400> ***.***.InvalidParameter: Temperature should be in [0.0, 2.0)", kimiCompatSampling, true},
		{"top_k不支持", `{"top_k":50}`, "Unsupported parameter: 'top_k'.", kimiCompatSampling, true},
		{"字段不存在", `{}`, "'temperature' must be Float", 0, false},
		{"仅嵌套字段", `{"metadata":{"temperature":"hot"}}`, "'temperature' must be Float", 0, false},
		{"值不一致", `{"temperature":1}`, "Parameter 'temperature'=0.7 is not supported for kimi-k3 model.", 0, false},
		{"泛化错误", `{"temperature":0.7}`, "<400> ***.Algo: An error occurred in model serving, error message is: [Invalid request parameters.]", 0, false},
		{"消息引用", `{"temperature":0.7}`, "Invalid message content: field Temperature invalid, only 1 is allowed for this model", 0, false},
		{"多义尾随错误", `{"temperature":0.7,"tool_choice":"bogus"}`, "field Temperature invalid, only 1 is allowed for this model; tool_choice invalid", 0, false},
		{"相似字段", `{"temperature_extra":0.7}`, "Unsupported parameter: 'temperature_extra'.", 0, false},
		{"相似字段不得视为别名", `{"temperature":0.7,"temperature_":0.5}`, "Unsupported parameter: 'temperature_'.", 0, false},
		{"多余下划线不得视为别名", `{"top_p":0.7}`, "field top__p invalid, only 0.95 is allowed for this model", 0, false},
		{"n不处理", `{"n":2}`, "field n invalid, only 1 is allowed for this model", 0, false},
		{"stop不处理", `{"stop":"end"}`, "Unsupported parameter: 'stop'.", 0, false},
		{"非法JSON请求", `{"temperature":`, "'temperature' must be Float", 0, false},
		{"重复请求字段", `{"temperature":0.7,"temperature":1}`, "field Temperature invalid, only 1 is allowed for this model", 0, false},
		{"大小写歧义请求字段", `{"temperature":0.7,"Temperature":1}`, "field Temperature invalid, only 1 is allowed for this model", 0, false},
		{"已满足固定值", `{"temperature":1}`, "field Temperature invalid, only 1 is allowed for this model", 0, false},
		{"已满足范围", `{"temperature":0.7}`, "Temperature should be in [0.0, 2.0)", 0, false},
		{"已满足类型", `{"temperature":0.7}`, "'temperature' must be Float", 0, false},
		{"非法level", `{"reasoning_effort":"invalid-audit-value"}`, `level "invalid-audit-value" not supported, valid levels: minimal, low, medium, high, xhigh, max, none, auto`, kimiCompatReasoning, true},
		{"非法强度", `{"reasoning_effort":"medium"}`, `Invalid value for 'reasoning_effort': 'medium'. Supported values are: 'low', 'high', 'max'.`, kimiCompatReasoning, true},
		{"指定模型拒绝强度", `{"reasoning_effort":"medium"}`, `Parameter 'reasoning_effort'='medium' is not supported for kimi-k3 model.`, kimiCompatReasoning, true},
		{"强度允许列表不含low", `{"reasoning_effort":"medium"}`, `level "medium" not supported, valid levels: high, max`, 0, false},
		{"不误匹配low子串", `{"reasoning_effort":"medium"}`, `level "medium" not supported, valid levels: lower, high`, 0, false},
		{"已是low", `{"reasoning_effort":"low"}`, `level "low" not supported, valid levels: high, max`, 0, false},
		{"level值不一致", `{"reasoning_effort":"high"}`, `level "invalid-audit-value" not supported, valid levels: low, high, max`, 0, false},
		{"level无强度字段", `{}`, `level "invalid-audit-value" not supported, valid levels: low, high, max`, 0, false},
		{"整个强度字段不支持", `{"reasoning_effort":"high"}`, `Unsupported parameter: 'reasoning_effort'.`, 0, false},
		{"非法工具选择", `{"tool_choice":"bogus","tools":[{"type":"function","function":{"name":"keep"}}]}`, "Invalid value for `tool_choice`: bogus! Only named tools, \"none\", \"auto\" or \"required\" are supported.", kimiCompatToolChoice, true},
		{"缺少function", `{"tool_choice":{"type":"function","name":"keep"}}`, "Expected field `function` in `tool_choice`. Correct usage: `{\"type\": \"function\", \"function\": {\"name\": \"my_function\"}}`", kimiCompatToolChoice, true},
		{"缺少function无示例", `{"tool_choice":{"type":"function"}}`, "Expected field `function` in `tool_choice`.", kimiCompatToolChoice, true},
		{"工具实际值不同", `{"tool_choice":"auto"}`, "Invalid value for `tool_choice`: bogus! Only named tools, \"none\", \"auto\" or \"required\" are supported.", 0, false},
		{"工具存在function", `{"tool_choice":{"type":"function","function":{"name":"keep"}}}`, "Expected field `function` in `tool_choice`.", 0, false},
		{"工具schema不处理", `{"tool_choice":"bogus"}`, "Invalid tools function parameters schema", 0, false},
		{"预算冲突128", `{"max_completion_tokens":128}`, "<400> ***.***.InvalidParameter: max_completion_tokens [128] must be greater than thinking_budget [32768]", kimiCompatBudget, true},
		{"预算冲突16384", `{"max_completion_tokens":16384,"thinking_budget":32768}`, "max_completion_tokens [16384] must be greater than thinking_budget [32768]", kimiCompatBudget, true},
		{"预算冲突无值", `{"max_completion_tokens":128}`, "max_completion_tokens must be greater than thinking_budget", kimiCompatBudget, true},
		{"预算关系实际合法", `{"max_completion_tokens":512,"thinking_budget":128}`, "max_completion_tokens must be greater than thinking_budget", 0, false},
		{"预算值不符", `{"max_completion_tokens":512}`, "max_completion_tokens [128] must be greater than thinking_budget [32768]", 0, false},
		{"思考预算值不符", `{"max_completion_tokens":128,"thinking_budget":1}`, "max_completion_tokens [128] must be greater than thinking_budget [32768]", 0, false},
		{"预算字段不存在", `{"max_tokens":128}`, "max_completion_tokens [128] must be greater than thinking_budget [32768]", 0, false},
		{"上下文超长不处理", `{"max_completion_tokens":128}`, "max_completion_tokens exceeds context length", 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule, matched := matchKimiParameterRejection([]byte(test.request), kimiTestError(test.message))
			require.Equal(t, test.matched, matched)
			if matched {
				require.Equal(t, test.rule, rule)
			}
		})
	}
}

func TestKimiParameterErrorEnvelope(t *testing.T) {
	for _, test := range []struct {
		body    string
		matched bool
	}{
		{`{"code":"InvalidParameter","message":"'temperature' must be Float"}`, true},
		{`{"error":{"code":"invalid_parameter_error","type":"invalid_request_error","message":"'temperature' must be Float","param":"temperature"}}`, true},
		{`{"error":{"type":"invalid_request_error","message":"'temperature' must be Float","param":"top_p"}}`, false},
		{`{"error":{"type":"data_inspection_failed","message":"'temperature' must be Float"}}`, false},
		{`{"error":{"code":"insufficient_quota","message":"'temperature' must be Float"}}`, false},
		{`{"error":{"message":"'temperature' must be Float","message":"Invalid request parameters"}}`, false},
		{`{"error":{"message":"Invalid request parameters","Message":"'temperature' must be Float"}}`, false},
		{`{"error":{"message":"'temperature' must be Float","param":"temperature_"}}`, false},
		{`{"error":{"message":"Invalid request parameters","detail":"'temperature' must be Float"}}`, false},
		{`{"error":null,"message":"'temperature' must be Float","code":"InvalidParameter"}`, false},
		{`{"message":"'temperature' must be Float"}`, false},
		{`{"error":{"message":"'temperature' must be Float"}`, false},
		{`<html>'temperature' must be Float</html>`, false},
	} {
		t.Run(test.body, func(t *testing.T) {
			_, matched := matchKimiParameterRejection([]byte(`{"temperature":"hot","top_p":0.5}`), []byte(test.body))
			require.Equal(t, test.matched, matched)
		})
	}
}

func TestKimiThinkingTypeParameterRejectionMatching(t *testing.T) {
	message := `'type' must be in ["enabled", "disabled", "auto"]`
	for _, test := range []struct {
		name    string
		request string
		matched bool
	}{
		{"非法adaptive", `{"thinking":{"type":"adaptive"}}`, true},
		{"其他非法字符串", `{"thinking":{"type":"high"}}`, true},
		{"空字符串", `{"thinking":{"type":""}}`, true},
		{"非法布尔值", `{"thinking":{"type":true}}`, true},
		{"非法数字", `{"thinking":{"type":1}}`, true},
		{"显式null", `{"thinking":{"type":null}}`, true},
		{"合法enabled", `{"thinking":{"type":"enabled"}}`, false},
		{"合法disabled", `{"thinking":{"type":"disabled"}}`, false},
		{"合法auto", `{"thinking":{"type":"auto"}}`, false},
		{"缺少thinking", `{}`, false},
		{"缺少type", `{"thinking":{"budget_tokens":128}}`, false},
		{"非对象thinking", `{"thinking":"adaptive"}`, false},
		{"数组thinking", `{"thinking":[{"type":"adaptive"}]}`, false},
		{"重复type", `{"thinking":{"type":"adaptive","type":"enabled"}}`, false},
		{"大小写歧义type", `{"thinking":{"type":"adaptive","Type":"enabled"}}`, false},
		{"重复thinking", `{"thinking":{"type":"adaptive"},"thinking":{"type":"enabled"}}`, false},
		{"仅extra_body嵌套", `{"extra_body":{"thinking":{"type":"adaptive"}}}`, false},
		{"仅工具type", `{"tools":[{"type":"adaptive"}]}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rule, matched := matchKimiParameterRejection([]byte(test.request), kimiTestError(message))
			require.Equal(t, test.matched, matched)
			if matched {
				require.Equal(t, kimiCompatThinkingType, rule)
			}
		})
	}
	for _, test := range []struct {
		name    string
		error   kimiParameterError
		matched bool
	}{
		{"完整生产错误", kimiParameterError{Message: message, Code: "invalid_parameter_error", Type: "invalid_request_error"}, true},
		{"明确thinking.type", kimiParameterError{Message: message, Param: "thinking.type"}, true},
		{"明确type", kimiParameterError{Message: message, Param: "type"}, true},
		{"其他字段", kimiParameterError{Message: message, Param: "tool_choice.type"}, false},
		{"引用错误", kimiParameterError{Message: "Invalid message content: " + message}, false},
		{"歧义尾随错误", kimiParameterError{Message: message + "; tool_choice invalid"}, false},
		{"不同枚举", kimiParameterError{Message: `'type' must be in ["function", "custom"]`}, false},
		{"泛化400", kimiParameterError{Message: "Invalid request parameters"}, false},
		{"审核错误", kimiParameterError{Message: message, Type: "content_filter"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"error": test.error})
			require.NoError(t, err)
			_, matched := matchKimiParameterRejection([]byte(`{"thinking":{"type":"adaptive"}}`), body)
			require.Equal(t, test.matched, matched)
		})
	}
}

func TestKimiParameterRepairPreservesUnrelatedFields(t *testing.T) {
	body := []byte(`{"temperature":0.7,"top_p":0.8,"top_k":5,"presence_penalty":0.5,"frequency_penalty":0.5,"n":1,"stop":"end","reasoning_effort":"medium","tool_choice":"bogus","tools":[{"type":"function","function":{"name":"keep"}}],"messages":[{"role":"assistant","tool_calls":[{"id":"call_keep"}]}],"max_completion_tokens":128,"max_tokens":512,"thinking_budget":32768,"thinking":{"type":"adaptive","budget_tokens":1024,"keep":"all"}}`)
	for rule := kimiCompatRule(0); rule < kimiCompatRuleCount; rule++ {
		t.Run(rule.String(), func(t *testing.T) {
			modified, fields, err := applyKimiParameterRepair(body, rule)
			require.NoError(t, err)
			require.NotEmpty(t, fields)
			for _, field := range []string{"tools", "messages", "n", "stop", "max_tokens", "thinking_budget"} {
				require.Equal(t, gjson.GetBytes(body, field).Raw, gjson.GetBytes(modified, field).Raw)
			}
			if rule == kimiCompatSampling {
				require.Len(t, fields, 5)
			}
			if rule == kimiCompatReasoning {
				require.Equal(t, "low", gjson.GetBytes(modified, "reasoning_effort").String())
			} else {
				for _, field := range fields {
					require.False(t, gjson.GetBytes(modified, field).Exists())
				}
			}
			if rule == kimiCompatThinkingType {
				require.Equal(t, []string{"thinking"}, fields)
				for _, field := range []string{"reasoning_effort", "max_completion_tokens"} {
					require.Equal(t, gjson.GetBytes(body, field).Raw, gjson.GetBytes(modified, field).Raw)
				}
			} else {
				require.Equal(t, gjson.GetBytes(body, "thinking").Raw, gjson.GetBytes(modified, "thinking").Raw)
			}
			_, changed, err := applyKimiParameterRepair(modified, rule)
			require.NoError(t, err)
			require.Empty(t, changed)
		})
	}
}
