package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicPassthroughThinkingBindingPreserved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{
		"model":"claude-sonnet-5-5",
		"thinking":{"type":"adaptive","display":"summarized","block_binding":{"prefix_mismatch_behavior":"error","vendor_extension":true}},
		"system":"system B",
		"messages":[{"role":"user","content":"question"},{"role":"assistant","content":[{"type":"thinking","thinking":"reasoning","signature":"signed-under-system-A"},{"type":"text","text":"answer"}]},{"role":"user","content":"Reply OK."}]
	}`)
	tests := []struct {
		name         string
		clientBeta   string
		overrideBeta string
	}{
		{name: "缺少beta头"},
		{name: "仅有无关beta", clientBeta: claude.BetaClaudeCode},
		{name: "已有绑定beta", clientBeta: claude.BetaThinkingBindingControls},
		{name: "账号覆写移除绑定beta", clientBeta: claude.BetaThinkingBindingControls, overrideBeta: claude.BetaClaudeCode},
		{name: "账号覆写添加绑定beta", overrideBeta: claude.BetaThinkingBindingControls},
	}
	for _, endpoint := range []string{"/v1/messages", "/v1/messages/count_tokens"} {
		t.Run(endpoint, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, endpoint, nil)
					if tt.clientBeta != "" {
						c.Request.Header.Set("Anthropic-Beta", tt.clientBeta)
					}
					account := newAnthropicAPIKeyAccountForTest()
					expectedBeta := tt.clientBeta
					if tt.overrideBeta != "" {
						account.Credentials["header_override_enabled"] = true
						account.Credentials["header_overrides"] = map[string]any{"anthropic-beta": tt.overrideBeta}
						expectedBeta = tt.overrideBeta
					}
					svc := &GatewayService{cfg: &config.Config{}}
					var req *http.Request
					var err error
					if endpoint == "/v1/messages/count_tokens" {
						req, err = svc.buildCountTokensRequestAnthropicAPIKeyPassthrough(context.Background(), c, account, body, "upstream-key")
					} else {
						var wireBody []byte
						req, wireBody, err = svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(context.Background(), c, account, body, "upstream-key")
						require.NoError(t, err)
						require.Equal(t, string(body), string(wireBody), "透传不能按beta删除绑定控制及其扩展字段")
					}
					require.NoError(t, err)
					defer req.Body.Close()
					actual, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					require.Equal(t, string(body), string(actual), "出站请求必须保留签名和前缀绑定要求")
					require.Equal(t, expectedBeta, getHeaderRaw(req.Header, "anthropic-beta"), "保留字段不应自动添加beta头")
					require.Equal(t, "upstream-key", getHeaderRaw(req.Header, "x-api-key"))
				})
			}
		})
	}
}

func TestAnthropicPassthroughThinkingBindingUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []string{"false", "true"} {
		t.Run("stream="+stream, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			body := []byte(`{"model":"claude-sonnet-5-5","max_tokens":1024,"stream":` + stream + `,"thinking":{"type":"adaptive","display":"summarized","block_binding":{"prefix_mismatch_behavior":"error"}},"system":"system B","messages":[{"role":"user","content":"question"},{"role":"assistant","content":[{"type":"thinking","thinking":"reasoning","signature":"signed-under-system-A"},{"type":"text","text":"answer"}]},{"role":"user","content":"Reply OK."}]}`)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
			upstreamBody := `{"type":"error","error":{"type":"invalid_request_error","message":"thinking block signature does not match the request prefix (prefix_mismatch_behavior is 'error')"}}`
			upstream := &anthropicHTTPUpstreamRecorder{
				resp: &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(upstreamBody)),
				},
			}
			svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			parsed := &ParsedRequest{Model: "claude-sonnet-5-5", Stream: stream == "true", Body: NewRequestBodyRef(body)}
			result, err := svc.Forward(context.Background(), c, newAnthropicAPIKeyAccountForTest(), parsed)
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, string(body), string(upstream.lastBody), "完整转发链路不能丢失绑定字段")
			require.Equal(t, string(body), string(parsed.Body.Bytes()), "请求记录必须保留实际转发的绑定要求")
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, upstreamBody, rec.Body.String(), "签名校验错误应交给上游并原样返回")
		})
	}
}
