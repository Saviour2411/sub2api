//go:build unit

package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIStreamTransportEventRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	codes := []string{OpenAIUpstreamHTTP2StreamErrorCode, OpenAIUpstreamStreamReadErrorCode, OpenAIUpstreamStreamTruncatedCode}
	for _, code := range codes {
		for _, nested := range []bool{false, true} {
			for _, mode := range []string{"responses", "passthrough", "chat", "messages"} {
				for _, afterOutput := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/nested=%t/%s/after_output=%t", code, nested, mode, afterOutput), func(t *testing.T) {
						errFields := fmt.Sprintf(`"code":%q,"message":"Upstream HTTP/2 stream failed"`, code)
						payload := `{"type":"error",` + errFields + `}`
						if nested {
							payload = `{"type":"error","error":{` + errFields + `}}`
						}
						stream := "event: error\ndata: " + payload + "\n\n"
						if afterOutput {
							stream = "data: " + `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"partial"}` + "\n\n" + stream
						}
						recorder := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(recorder)
						c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
						resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}
						svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
						account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
							"pool_mode": true, "pool_mode_retry_status_codes": []any{http.StatusBadGateway},
						}}
						var err error
						switch mode {
						case "responses":
							_, err = svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
						case "passthrough":
							_, err = svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
						case "chat":
							_, err = svc.handleChatStreamingResponse(resp, c, account, "gpt-6.1-sol", "gpt-6.1-sol", "gpt-6.1-sol", time.Now(), 0)
						case "messages":
							_, err = svc.handleAnthropicStreamingResponse(resp, c, account, "gpt-6.1-sol", "gpt-6.1-sol", "gpt-6.1-sol", time.Now())
						}
						require.Error(t, err)
						var failover *UpstreamFailoverError
						if afterOutput {
							require.False(t, errors.As(err, &failover), "已输出内容的请求不能重放")
							require.Contains(t, recorder.Body.String(), "partial")
							require.Contains(t, recorder.Body.String(), "Upstream HTTP/2 stream failed")
							if mode == "responses" || mode == "passthrough" {
								require.Contains(t, recorder.Body.String(), code)
							}
							return
						}
						require.ErrorAs(t, err, &failover)
						require.Equal(t, http.StatusBadGateway, failover.StatusCode)
						require.True(t, failover.RetryableOnSameAccount)
						require.Equal(t, code, gjson.GetBytes(failover.ResponseBody, "error.code").String())
						require.False(t, c.Writer.Written())
						require.Empty(t, recorder.Body.String(), "中间失败不能提前泄露到客户端")
					})
				}
			}
		}
	}
}

func TestOpenAIStreamTransportEventHonorsPoolRetryPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry_502=%t", enabled), func(t *testing.T) {
			statuses := []any{}
			if enabled {
				statuses = append(statuses, http.StatusBadGateway)
			}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"pool_mode": true, "pool_mode_retry_status_codes": statuses, "pool_mode_retry_count": 2,
			}}
			payload := []byte(`{"type":"error","code":"upstream_http2_stream_error","message":"Upstream HTTP/2 stream failed"}`)
			failover := (&OpenAIGatewayService{}).newOpenAIStreamFailoverError(nil, account, false, "", payload, extractOpenAISSEErrorMessage(payload))
			require.Equal(t, enabled, failover.RetryableOnSameAccount)
			require.Equal(t, 2, account.GetPoolModeRetryCount())
		})
	}
}

func TestOpenAIStreamTransportEventRejectsUnclassifiedErrors(t *testing.T) {
	for _, payload := range []string{
		`{"type":"error","message":"Upstream HTTP/2 stream failed"}`,
		`{"type":"error","code":"unknown_error","message":"Upstream HTTP/2 stream failed"}`,
		`{"type":"error","error":{"code":"content_policy","message":"upstream_http2_stream_error"}}`,
		`{"type":"error","code":"upstream_http2_stream_error","error":{"type":"invalid_request_error","message":"invalid request"}}`,
		`{"type":"error","code":"upstream_http2_stream_error","error":{"code":"context_length_exceeded","message":"input exceeds the context window"}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			require.False(t, openAIStreamErrorEventShouldFailover([]byte(payload), extractOpenAISSEErrorMessage([]byte(payload))))
		})
	}
}

func TestOpenAIStreamTransportEventBufferedRecovery(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(fmt.Sprintf("passthrough=%t", passthrough), func(t *testing.T) {
			c, recorder := newNonStreamingFailoverContext(t)
			body := sseTerminalBody("error", `{"type":"error","code":"upstream_http2_stream_error","message":"Upstream HTTP/2 stream failed"}`)
			svc := newNonStreamingFailoverService()
			var err error
			if passthrough {
				_, err = svc.handlePassthroughSSEToJSON(newNonStreamingSSEResponse(), c, newNonStreamingFailoverAccount(), body, "gpt-6.1-sol", "gpt-6.1-sol")
			} else {
				_, err = svc.handleSSEToJSON(newNonStreamingSSEResponse(), c, newNonStreamingFailoverAccount(), body, "gpt-6.1-sol", "gpt-6.1-sol")
			}
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.Equal(t, OpenAIUpstreamHTTP2StreamErrorCode, gjson.GetBytes(failover.ResponseBody, "error.code").String())
			require.False(t, c.Writer.Written())
			require.Empty(t, recorder.Body.String())
		})
	}
}
