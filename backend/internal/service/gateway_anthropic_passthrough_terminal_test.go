package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type anthropicTerminalResponseBody struct {
	*io.PipeReader
	closed atomic.Bool
}

func (body *anthropicTerminalResponseBody) Close() error {
	body.closed.Store(true)
	return body.PipeReader.Close()
}

func TestGatewayService_AnthropicAPIKeyPassthrough_StreamingTerminalEventDoesNotWaitForEOF(tester *testing.T) {
	gin.SetMode(gin.TestMode)
	scenarios := []struct {
		name               string
		terminalPrefix     string
		terminalSuffix     string
		clientDisconnected bool
	}{
		{
			name:           "终止事件名后仍等待完整事件",
			terminalPrefix: "event: message_stop\n",
			terminalSuffix: "data: {\"type\":\"message_stop\"}\n\n",
		},
		{
			name:           "终止数据行后仍等待事件分隔符",
			terminalPrefix: "data: {\"type\":\"message_stop\"}\n",
			terminalSuffix: "\n",
		},
		{
			name:           "保留DONE兼容",
			terminalPrefix: "data: [DONE]\n",
			terminalSuffix: "\n",
		},
		{
			name:               "客户端断开后仍收集最终用量并结束",
			terminalPrefix:     "event: message_stop\n",
			terminalSuffix:     "data: {\"type\":\"message_stop\"}\n\n",
			clientDisconnected: true,
		},
	}
	for _, scenario := range scenarios {
		tester.Run(scenario.name, func(tester *testing.T) {
			recorder := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(recorder)
			requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			if scenario.clientDisconnected {
				requestContext.Writer = &failWriteResponseWriter{ResponseWriter: requestContext.Writer}
			}
			reader, writer := io.Pipe()
			tester.Cleanup(func() {
				_ = reader.Close()
				_ = writer.Close()
			})
			responseBody := &anthropicTerminalResponseBody{PipeReader: reader}
			gateway := &GatewayService{
				cfg: &config.Config{Gateway: config.GatewayConfig{
					MaxLineSize:               defaultMaxLineSize,
					StreamDataIntervalTimeout: 1,
				}},
				httpUpstream: &anthropicHTTPUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"text/event-stream"}},
					Body:       responseBody,
				}},
			}
			type forwardOutcome struct {
				result *ForwardResult
				err    error
			}
			completed := make(chan forwardOutcome, 1)
			go func() {
				result, forwardError := gateway.forwardAnthropicAPIKeyPassthrough(
					context.Background(), requestContext, newAnthropicAPIKeyAccountForTest(),
					[]byte(`{"model":"claude-3-7-sonnet-20250219","stream":true,"messages":[{"role":"user","content":"本地回归"}]}`),
					"claude-3-7-sonnet-20250219", "claude-3-7-sonnet-20250219", true, time.Now(),
				)
				completed <- forwardOutcome{result: result, err: forwardError}
			}()
			payload := "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11,\"cache_read_input_tokens\":7}}}\n\n" +
				"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"完整回答\"}}\n\n" +
				"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n"
			_, writeError := io.WriteString(writer, payload+scenario.terminalPrefix)
			require.NoError(tester, writeError)
			select {
			case <-completed:
				tester.Fatal("终止事件尚未完整转发，不应结束请求")
			case <-time.After(50 * time.Millisecond):
			}
			_, writeError = io.WriteString(writer, scenario.terminalSuffix)
			require.NoError(tester, writeError)
			select {
			case outcome := <-completed:
				require.NoError(tester, outcome.err)
				require.NotNil(tester, outcome.result)
				require.Equal(tester, 11, outcome.result.Usage.InputTokens)
				require.Equal(tester, 7, outcome.result.Usage.CacheReadInputTokens)
				require.Equal(tester, 5, outcome.result.Usage.OutputTokens)
				require.Equal(tester, scenario.clientDisconnected, outcome.result.ClientDisconnect)
				require.True(tester, responseBody.closed.Load(), "转发结束时应关闭响应体，释放读取协程")
				if !scenario.clientDisconnected {
					require.Equal(tester, payload+scenario.terminalPrefix+scenario.terminalSuffix, recorder.Body.String())
					require.True(tester, recorder.Flushed)
				}
			case <-time.After(3 * time.Second):
				tester.Fatal("收到完整终止事件后不应继续等待上游EOF")
			}
		})
	}
}
