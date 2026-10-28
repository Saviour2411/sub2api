package apicompat

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRequest_独立模型校验选项(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		for _, tool := range []bool{false, true} {
			t.Run(fmt.Sprintf("thinking=%v/tool=%v", thinking, tool), func(t *testing.T) {
				options := AnthropicModelValidationOptions{SkipThinkingValidation: thinking, SkipToolChoiceValidation: tool}
				var req ResponsesRequest
				require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-opus-5-5","input":"hello","reasoning":{"effort":"none"}}`), &req))
				converted, err := ResponsesToAnthropicRequestWithOptions(&req, options)
				if thinking {
					require.NoError(t, err)
					require.Equal(t, "none", converted.OutputConfig.Effort)
				} else {
					require.ErrorContains(t, err, "reasoning effort")
				}
				req.Reasoning = nil
				req.ToolChoice = json.RawMessage(`"required"`)
				converted, err = ResponsesToAnthropicRequestWithOptions(&req, options)
				if tool {
					require.NoError(t, err)
					require.JSONEq(t, `{"type":"any"}`, string(converted.ToolChoice))
				} else {
					require.ErrorContains(t, err, "forced tool_choice")
				}
				_, err = ResponsesToAnthropicRequest(&req)
				require.ErrorContains(t, err, "forced tool_choice")
				req.ToolChoice = json.RawMessage(`[1]`)
				_, err = ResponsesToAnthropicRequestWithOptions(&req, options)
				require.Error(t, err, "数组类型仍是结构错误")
			})
		}
	}
}
