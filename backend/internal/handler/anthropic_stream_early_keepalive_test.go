//go:build unit

package handler

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const earlyHandlerPing = "event: ping\ndata: {\"type\":\"ping\"}\n\n"

func TestAnthropicStreamSafeRetryHandlerEarlyKeepaliveOutcomes(t *testing.T) {
	for _, scenario := range []struct {
		name                              string
		bodies                            []string
		statuses                          []int
		wantCalls, wantErrors, wantTokens int
	}{
		{"空流恢复", []string{"", safeHandlerPrelude + safeHandlerOutput + safeHandlerStop}, nil, 2, 0, 3},
		{"前导重试耗尽", []string{safeHandlerPrelude, safeHandlerPrelude, ""}, nil, 3, 1, 0},
		{"正文后中断", []string{safeHandlerPrelude + safeHandlerOutput}, nil, 1, 1, 3},
		{"重试遇到400", []string{safeHandlerPrelude, `{"error":{"type":"invalid_request_error","message":"私密上游错误"}}`}, []int{200, 400}, 2, 1, 0},
		{"重试遇到503再恢复", []string{safeHandlerPrelude, `{"error":{"message":"unavailable"}}`, safeHandlerPrelude + safeHandlerOutput + safeHandlerStop}, []int{200, 503, 200}, 3, 0, 3},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			upstream := &safeHandlerUpstream{earlyKeepalive: true, bodies: scenario.bodies, statuses: scenario.statuses}
			writer, usage := runSafeHandlerFixture(t, upstream, false, func(c *gin.Context) {
				if scenario.wantErrors > 0 {
					require.NotEmpty(t, service.GetOpsStreamErrors(c), "HTTP 200 的流内失败仍须进入错误记录")
				}
			})
			require.Len(t, upstream.accounts, scenario.wantCalls)
			require.Equal(t, http.StatusOK, writer.Code)
			require.Equal(t, "text/event-stream", writer.Result().Header.Get("Content-Type"))
			require.Empty(t, writer.Result().Header.Get("X-Request-Id"))
			require.True(t, strings.HasPrefix(writer.Body.String(), earlyHandlerPing))
			require.Equal(t, scenario.wantErrors, strings.Count(writer.Body.String(), `"type":"error"`))
			require.NotContains(t, writer.Body.String(), "私密上游错误")
			require.Len(t, usage.logs, 1)
			require.Equal(t, scenario.wantTokens, usage.logs[0].OutputTokens)
			if scenario.wantTokens > 0 {
				require.Equal(t, 1, strings.Count(writer.Body.String(), "event: message_start"))
			} else {
				require.NotContains(t, writer.Body.String(), "message_start")
			}
		})
	}
}

func TestAnthropicStreamSafeRetryHandlerEarlyKeepaliveRealHTTP(t *testing.T) {
	for _, protocol := range []string{"HTTP/1.1", "HTTP/2.0"} {
		for _, encoding := range []string{"identity", "gzip", "zstd"} {
			t.Run(protocol+"/"+encoding, func(t *testing.T) {
				payload := safeHandlerPrelude + safeHandlerOutput + safeHandlerStop
				encoded := terminalStreamPayload(t, encoding, payload)
				releaseFirst := make(chan struct{})
				releaseFinal := make(chan struct{})
				var calls atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					_, _ = io.Copy(io.Discard, request.Body)
					attempt := calls.Add(1)
					writer.Header().Set("Content-Type", "text/event-stream")
					writer.Header().Set("X-Request-Id", "不得提前泄露尝试ID")
					if attempt == 1 {
						writer.WriteHeader(http.StatusOK)
						_ = http.NewResponseController(writer).Flush()
						select {
						case <-releaseFirst:
						case <-request.Context().Done():
						}
						return
					}
					writer.Header().Set("Content-Encoding", encoding)
					_, _ = writer.Write(encoded)
					_ = http.NewResponseController(writer).Flush()
					select {
					case <-releaseFinal:
					case <-request.Context().Done():
					}
				}))
				handler, usage := terminalStreamHandler(t, upstream.URL, true, 0, true)
				done := make(chan struct{})
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					defer close(done)
					handler.ServeHTTP(writer, request)
				}))
				server.EnableHTTP2 = protocol == "HTTP/2.0"
				server.StartTLS()
				t.Cleanup(func() {
					close(releaseFinal)
					server.Close()
					upstream.Close()
				})
				client := server.Client()
				client.Timeout = 5 * time.Second
				response, err := client.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(`{"model":"claude-sonnet-4-5","stream":true,"max_tokens":50,"messages":[{"role":"user","content":"测试"}]}`))
				require.NoError(t, err)
				defer response.Body.Close()
				require.Equal(t, protocol, response.Proto)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.Empty(t, response.Header.Get("X-Request-Id"))
				reader := bufio.NewReader(response.Body)
				first := make([]byte, len(earlyHandlerPing))
				_, err = io.ReadFull(reader, first)
				require.NoError(t, err)
				require.Equal(t, earlyHandlerPing, string(first), "上游还未发送任何事件，下游必须已收到完整JSON保活")
				require.EqualValues(t, 1, calls.Load())
				close(releaseFirst)
				remainder, err := io.ReadAll(reader)
				require.NoError(t, err)
				require.Equal(t, payload, strings.ReplaceAll(string(remainder), earlyHandlerPing, ""))
				<-done
				require.EqualValues(t, 2, calls.Load())
				require.Len(t, usage.logs, 1)
				require.Equal(t, 7, usage.logs[0].InputTokens)
				require.Equal(t, 3, usage.logs[0].OutputTokens)
			})
		}
	}
}
