package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type anthropicIdleTestOutcome struct {
	result      *streamingResult
	err         error
	completedAt time.Time
}

// 测试仅使用内存管道和虚拟时钟，避免真实等待数分钟或访问上游。
func startAnthropicIdleTestStream(t *testing.T, timeoutSeconds, keepaliveSeconds int, disconnected bool) (*httptest.ResponseRecorder, *io.PipeWriter, <-chan anthropicIdleTestOutcome, func()) {
	t.Helper()
	recorder := httptest.NewRecorder()
	requestContext, _ := gin.CreateTestContext(recorder)
	requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	if disconnected {
		requestContext.Writer = &failWriteResponseWriter{ResponseWriter: requestContext.Writer}
	}
	reader, writer := io.Pipe()
	gateway := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		StreamDataIntervalTimeout: timeoutSeconds,
		StreamKeepaliveInterval:   keepaliveSeconds,
		MaxLineSize:               defaultMaxLineSize,
	}}}
	completed := make(chan anthropicIdleTestOutcome, 1)
	go func() {
		result, err := gateway.handleStreamingResponseAnthropicAPIKeyPassthrough(
			context.Background(), &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body:       reader,
			}, requestContext, &Account{ID: 740}, time.Now(), "claude-opus-4-6",
		)
		completed <- anthropicIdleTestOutcome{result: result, err: err, completedAt: time.Now()}
	}()
	synctest.Wait()
	return recorder, writer, completed, func() {
		_ = writer.Close()
		_ = reader.Close()
	}
}

func writeAnthropicIdleTestEvent(t *testing.T, writer *io.PipeWriter, event string) {
	t.Helper()
	_, err := io.WriteString(writer, event)
	require.NoError(t, err)
	synctest.Wait()
}

func requireAnthropicIdleTestPending(t *testing.T, completed <-chan anthropicIdleTestOutcome) {
	t.Helper()
	select {
	case outcome := <-completed:
		t.Fatalf("尚未达到空闲截止时间，不应结束请求：%v", outcome.err)
	default:
	}
}

func requireAnthropicIdleTestCompleted(t *testing.T, completed <-chan anthropicIdleTestOutcome) anthropicIdleTestOutcome {
	t.Helper()
	select {
	case outcome := <-completed:
		return outcome
	default:
		t.Fatal("已达到空闲截止时间或收到完整终止事件，不应继续等待下一轮检查")
		return anthropicIdleTestOutcome{}
	}
}

const anthropicIdleTestStart = "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":7}}}\n\n"
const anthropicIdleTestStop = "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":2}}\n\ndata: {\"type\":\"message_stop\"}\n\n"

func TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeoutUsesExactDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []struct {
		name          string
		keepalive     int
		disconnected  bool
		expectedError string
	}{
		{name: "最后一行之后准确等待180秒", expectedError: "stream data interval timeout"},
		{name: "下游心跳不能延长上游空闲期限", keepalive: 10, expectedError: "stream data interval timeout"},
		{name: "客户端断开后仍按相同期限排空", keepalive: 10, disconnected: true, expectedError: "stream usage incomplete after timeout"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				recorder, writer, completed, cleanup := startAnthropicIdleTestStream(t, 180, scenario.keepalive, scenario.disconnected)
				defer cleanup()
				// 首帧稍晚于计时器启动，复现旧实现把180秒空闲拖到下一轮检查的问题。
				time.Sleep(time.Second)
				writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStart)
				lastDataAt := time.Now()
				time.Sleep(180*time.Second - time.Nanosecond)
				synctest.Wait()
				requireAnthropicIdleTestPending(t, completed)
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				outcome := requireAnthropicIdleTestCompleted(t, completed)
				require.EqualError(t, outcome.err, scenario.expectedError)
				require.Equal(t, 180*time.Second, outcome.completedAt.Sub(lastDataAt))
				require.NotNil(t, outcome.result)
				require.Equal(t, 7, outcome.result.usage.InputTokens)
				require.Equal(t, scenario.disconnected, outcome.result.clientDisconnect)
				require.NotContains(t, recorder.Body.String(), "message_stop", "真实超时不能补造成功终止事件")
				if scenario.keepalive > 0 && !scenario.disconnected {
					require.Contains(t, recorder.Body.String(), "event: ping")
				}
			})
		})
	}
}

func TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeoutExtendsOnUpstreamLines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []struct{ name, event string }{
		{name: "内容数据", event: "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"持续输出\"}}\n\n"},
		{name: "上游心跳", event: "event: ping\ndata: {\"type\":\"ping\"}\n\n"},
		{name: "注释行", event: ": 上游保活\n\n"},
		{name: "空行", event: "\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				_, writer, completed, cleanup := startAnthropicIdleTestStream(t, 180, 0, false)
				defer cleanup()
				writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStart)
				for range 3 {
					time.Sleep(179 * time.Second)
					synctest.Wait()
					requireAnthropicIdleTestPending(t, completed)
					writeAnthropicIdleTestEvent(t, writer, scenario.event)
				}
				lastDataAt := time.Now()
				time.Sleep(180*time.Second - time.Nanosecond)
				synctest.Wait()
				requireAnthropicIdleTestPending(t, completed)
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				outcome := requireAnthropicIdleTestCompleted(t, completed)
				require.EqualError(t, outcome.err, "stream data interval timeout")
				require.Equal(t, 180*time.Second, outcome.completedAt.Sub(lastDataAt))
			})
		})
	}
}

func TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeoutDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	synctest.Test(t, func(t *testing.T) {
		_, writer, completed, cleanup := startAnthropicIdleTestStream(t, 0, 0, false)
		defer cleanup()
		writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStart)
		time.Sleep(30 * time.Minute)
		synctest.Wait()
		requireAnthropicIdleTestPending(t, completed)
		writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStop)
		outcome := requireAnthropicIdleTestCompleted(t, completed)
		require.NoError(t, outcome.err)
		require.Equal(t, 2, outcome.result.usage.OutputTokens)
	})
}

func TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeoutAcceptsTerminalBeforeDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	synctest.Test(t, func(t *testing.T) {
		_, writer, completed, cleanup := startAnthropicIdleTestStream(t, 180, 0, false)
		defer cleanup()
		writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStart)
		time.Sleep(180*time.Second - time.Nanosecond)
		synctest.Wait()
		requireAnthropicIdleTestPending(t, completed)
		writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStop)
		outcome := requireAnthropicIdleTestCompleted(t, completed)
		require.NoError(t, outcome.err)
		require.Equal(t, 2, outcome.result.usage.OutputTokens)
	})
}

func TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeoutDoesNotHideMissingTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	synctest.Test(t, func(t *testing.T) {
		_, writer, completed, cleanup := startAnthropicIdleTestStream(t, 180, 0, false)
		defer cleanup()
		writeAnthropicIdleTestEvent(t, writer, anthropicIdleTestStart)
		time.Sleep(179 * time.Second)
		require.NoError(t, writer.Close())
		synctest.Wait()
		outcome := requireAnthropicIdleTestCompleted(t, completed)
		require.EqualError(t, outcome.err, "stream usage incomplete: missing terminal event")
		require.Equal(t, 7, outcome.result.usage.InputTokens)
	})
}
