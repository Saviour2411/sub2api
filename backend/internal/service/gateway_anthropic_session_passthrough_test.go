//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicSessionIDPassthrough_AllEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens", "/v1/chat/completions", "/v1/responses"} {
		t.Run(path, func(t *testing.T) {
			account := newAnthropicAPIKeyAccountForTest()
			upstream := &anthropicHTTPUpstreamRecorder{}
			svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			cases := []struct {
				name        string
				passthrough bool
				header      string
				values      []string
			}{
				{name: "关闭透传", header: "X-Session-ID", values: []string{"session-A"}},
				{name: "开启透传", passthrough: true, header: "X-Session-ID", values: []string{"session-A"}},
				{name: "下一轮保持会话", passthrough: true, header: "X-Session-Id", values: []string{"session-A"}},
				{name: "小写头与独立会话", passthrough: true, header: "x-session-id", values: []string{"session-B"}},
				{name: "混合大小写与多值", passthrough: true, header: "x-SeSsIoN-iD", values: []string{"session-C", "session-D"}},
				{name: "缺失时不生成", passthrough: true},
				{name: "再次关闭透传", header: "x-session-id", values: []string{"session-E"}},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					account.Extra["anthropic_passthrough"] = tc.passthrough
					body := []byte(`{"model":"claude-sonnet-4-6","max_tokens":64,"messages":[{"role":"user","content":"hello"}]}`)
					if path == "/v1/responses" {
						body = []byte(`{"model":"claude-sonnet-4-6","input":"hello"}`)
					}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
					if tc.header != "" {
						c.Request.Header[tc.header] = tc.values
					}
					c.Request.Header.Set("X-Claude-Code-Session-Id", "claude-code-session")
					c.Request.Header.Set("Authorization", "Bearer client-key")
					c.Request.Header.Set("X-Api-Key", "client-key")
					c.Request.Header.Set("Cookie", "client-cookie")
					c.Request.Header.Set("X-Unlisted-Header", "unlisted")
					originalHeaders := c.Request.Header.Clone()
					// 在上游边界截获真实出站请求，返回模拟响应，不调用外部模型。
					responseBody := `{"id":"msg_session","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`
					contentType := "application/json"
					if path == "/v1/messages/count_tokens" {
						responseBody = `{"input_tokens":1}`
					} else if path == "/v1/chat/completions" || path == "/v1/responses" {
						responseBody = namespaceToolAnthropicStream()
						contentType = "text/event-stream"
					}
					upstream.lastReq = nil
					upstream.resp = &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{contentType}},
						Body:       io.NopCloser(strings.NewReader(responseBody)),
					}
					var err error
					switch path {
					case "/v1/chat/completions":
						_, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
					case "/v1/responses":
						_, err = svc.ForwardAsResponses(context.Background(), c, account, body, nil)
					default:
						parsed, parseErr := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
						require.NoError(t, parseErr)
						if path == "/v1/messages/count_tokens" {
							err = svc.ForwardCountTokens(context.Background(), c, account, parsed)
						} else {
							_, err = svc.Forward(context.Background(), c, account, parsed)
						}
					}
					require.NoError(t, err)
					require.NotNil(t, upstream.lastReq, "必须到达上游请求边界")
					// 按 HTTP 的大小写不敏感规则检查实际发出的所有同名头。
					wireHeaders := make(http.Header)
					for key, values := range upstream.lastReq.Header {
						for _, value := range values {
							wireHeaders.Add(key, value)
						}
					}
					if tc.passthrough {
						require.Equal(t, tc.values, wireHeaders.Values("X-Session-ID"))
					} else {
						require.Empty(t, wireHeaders.Values("X-Session-ID"))
					}
					require.Equal(t, "claude-code-session", wireHeaders.Get("X-Claude-Code-Session-Id"), "两个会话头必须相互独立")
					require.Equal(t, "upstream-anthropic-key", wireHeaders.Get("X-Api-Key"))
					for _, key := range []string{"Authorization", "Cookie", "X-Unlisted-Header"} {
						require.Empty(t, wireHeaders.Values(key), "不得放开其他客户端头：%s", key)
					}
					require.Equal(t, originalHeaders, c.Request.Header, "转发不能修改客户端请求头")
				})
			}
		})
	}
}
