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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardKimiRequestNonceScope(t *testing.T) {
	for _, platform := range []string{PlatformKimi, PlatformOpenAI, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Request.Header.Set("X-Msh-Request-Nonce", "caller-nonce")
			dst := http.Header{}
			forwardKimiRequestNonce(c, &Account{Platform: platform}, dst)
			if platform == PlatformKimi {
				require.Equal(t, "caller-nonce", dst.Get("X-Msh-Request-Nonce"))
			} else {
				require.Empty(t, dst)
			}
		})
	}
	dst := http.Header{}
	forwardKimiRequestNonce(nil, nil, dst)
	require.Empty(t, dst)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	forwardKimiRequestNonce(c, &Account{Platform: PlatformKimi}, dst)
	require.Empty(t, dst)
}

func TestKimiRawChatRequestSignaturePassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		for _, signed := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/signed=%t", stream, signed), func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":"kimi-k3","messages":[{"role":"user","content":"OK"}],"stream":%t}`, stream))
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
				c.Request.Header.Set("X-Msh-Request-Nonce", "caller-nonce")
				c.Request.Header.Set("Authorization", "Bearer caller-secret")
				c.Request.Header.Set("X-Msh-Private-Key", "must-not-forward")
				response := `{"id":"chatcmpl_signature","model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
				headers := http.Header{"Content-Type": []string{"application/json"}}
				if stream {
					response = "data: {\"id\":\"chatcmpl_signature\",\"model\":\"kimi-k3\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n"
					headers.Set("Content-Type", "text/event-stream")
				}
				if signed {
					headers.Set("Msh-Request-Timestamp", "1790232056000")
					headers.Set("Msh-Request-Signature", "reqsigv1_upstream-proof")
				}
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(response))}}
				cfg := rawChatCompletionsTestConfig()
				svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, responseHeaderFilter: compileResponseHeaderFilter(cfg)}
				account := rawChatCompletionsTestAccount()
				account.Platform = PlatformKimi
				result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "caller-nonce", upstream.lastReq.Header.Get("X-Msh-Request-Nonce"))
				require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
				require.Empty(t, upstream.lastReq.Header.Get("X-Msh-Private-Key"))
				require.Equal(t, headers.Get("Msh-Request-Signature"), recorder.Header().Get("Msh-Request-Signature"))
				require.Equal(t, headers.Get("Msh-Request-Timestamp"), recorder.Header().Get("Msh-Request-Timestamp"))
				require.Equal(t, http.StatusOK, recorder.Code)
			})
		}
	}
}

func TestKimiNativeAnthropicRequestNonce(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("X-Msh-Request-Nonce", "messages-nonce")
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	body := []byte(`{"model":"kimi-k3","max_tokens":16,"messages":[{"role":"user","content":"OK"}]}`)
	req, _, err := svc.buildNativeAnthropicUpstreamRequest(context.Background(), c, nativeAnthropicTestAccount(), body, "upstream-test-key", "http://upstream.example/v1/messages")
	require.NoError(t, err)
	require.Equal(t, "messages-nonce", req.Header.Get("X-Msh-Request-Nonce"))
}

func TestKimiNativeResponsesRequestNonce(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(fmt.Sprintf("passthrough=%t", passthrough), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			c.Request.Header.Set("X-Msh-Request-Nonce", "responses-nonce")
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
			account := rawChatCompletionsTestAccount()
			account.Platform = PlatformKimi
			body := []byte(`{"model":"kimi-k3","input":"OK"}`)
			var req *http.Request
			var err error
			if passthrough {
				req, err = svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "upstream-key")
			} else {
				req, err = svc.buildUpstreamRequest(context.Background(), c, account, body, "upstream-key", false, "", false)
			}
			require.NoError(t, err)
			require.Equal(t, "responses-nonce", req.Header.Get("X-Msh-Request-Nonce"))
			require.Equal(t, "Bearer upstream-key", req.Header.Get("Authorization"))
		})
	}
}

func TestKimiRawChatPreservesContractFieldsAndUsage(t *testing.T) {
	body := []byte(`{"model":"kimi-k3","messages":[{"role":"system","tools":[{"type":"function","function":{"name":"weather","description":"天气","parameters":{"type":"object","properties":{}}}}]},{"role":"assistant","content":"OK","reasoning_content":"保留历史思考"},{"role":"user","content":"继续"}],"reasoning_effort":"max","response_format":{"type":"json_object"},"tool_choice":"required"}`)
	original := bytes.Clone(body)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	response := `{"id":"contract","model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","content":"{}","reasoning_content":"原始思考"},"finish_reason":"stop"}],"usage":{"prompt_tokens":104,"completion_tokens":3,"total_tokens":107}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()
	account.Platform = PlatformKimi
	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, original, body)
	for _, field := range []string{"messages", "reasoning_effort", "response_format", "tool_choice"} {
		require.JSONEq(t, gjson.GetBytes(body, field).Raw, gjson.GetBytes(upstream.lastBody, field).Raw, field)
	}
	require.Equal(t, int64(104), gjson.Get(recorder.Body.String(), "usage.prompt_tokens").Int())
	require.Equal(t, "原始思考", gjson.Get(recorder.Body.String(), "choices.0.message.reasoning_content").String())
}
