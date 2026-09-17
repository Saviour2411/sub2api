//go:build unit

package service

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestKimiCompatibilityStrictRolesSurviveParameterRetry(t *testing.T) {
	for _, path := range []string{"/v1/chat/completions", "/v1/responses"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", path, stream), func(t *testing.T) {
				payload := fmt.Sprintf(`{"model":"kimi-k3","messages":[{"role":"developer","content":"保留规则"},{"role":"user","content":"hello"}],"temperature":0.7,"stream":%t}`, stream)
				if path == "/v1/responses" {
					payload = fmt.Sprintf(`{"model":"kimi-k3","input":[{"role":"developer","content":"保留规则"},{"role":"user","content":"hello"}],"temperature":0.7,"stream":%t}`, stream)
				}
				body := []byte(payload)
				original := bytes.Clone(body)
				ctx, recorder := kimiCompatTestContext(body, path)
				upstream := &httpUpstreamRecorder{responses: []*http.Response{
					kimiCompatTestResponse(400, kimiTestError("field Temperature invalid, only 1 is allowed for this model")),
					kimiCompatTestSuccess(stream),
				}}
				gateway := kimiCompatTestService(upstream)
				account := kimiCompatTestAccount()
				var result *OpenAIForwardResult
				var err error
				if path == "/v1/chat/completions" {
					result, err = gateway.ForwardAsChatCompletions(ctx.Request.Context(), ctx, account, body, "", "")
				} else {
					result, err = gateway.Forward(ctx.Request.Context(), ctx, account, body)
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Len(t, upstream.bodies, 2)
				for _, sent := range upstream.bodies {
					require.Equal(t, "system", gjson.GetBytes(sent, "messages.0.role").String())
					require.Equal(t, "保留规则", gjson.GetBytes(sent, "messages.0.content").String())
					require.Equal(t, "user", gjson.GetBytes(sent, "messages.1.role").String())
				}
				require.True(t, gjson.GetBytes(upstream.bodies[0], "temperature").Exists())
				require.False(t, gjson.GetBytes(upstream.bodies[1], "temperature").Exists())
				require.Equal(t, original, body, "账号兼容与参数重试不能修改入口请求体")
				require.Equal(t, 3, result.Usage.InputTokens)
				require.Equal(t, 2, result.Usage.OutputTokens)
				require.Equal(t, http.StatusOK, recorder.Code)
				require.NotContains(t, recorder.Body.String(), "Temperature invalid")
				events := kimiCompatTestEvents(ctx)
				require.Len(t, events, 1)
				require.Equal(t, kimiCompatSampling.String(), events[0].Reason)
			})
		}
	}
}
