//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnthropicCompatTerminalFramePreservesFirstTokenDecision(t *testing.T) {
	for _, test := range []struct {
		name  string
		state firstTokenAttemptState
		write bool
	}{
		{"等待首输出时提交协议终止帧", firstTokenAttemptWaiting, true},
		{"首Token超时继续换号", firstTokenAttemptTimedOut, false},
		{"客户端取消不写入", firstTokenAttemptClientCanceled, false},
		{"前奏溢出继续换号", firstTokenAttemptPreludeOverflow, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			attempt := &firstTokenAttempt{}
			attempt.state.Store(int32(test.state))
			attempt.wrapWriter(c)
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			failure := &anthropicCompatStreamFailure{err: errAnthropicNativeStreamIdle, errType: "stream_timeout", message: "上游超时"}
			writeAnthropicCompatCCStreamFailure(c, failure)
			require.Equal(t, test.write, IsResponseCommitted(c))
			if test.write {
				require.Equal(t, firstTokenAttemptDecidedWithoutToken, attempt.currentState())
				require.ErrorIs(t, attempt.finish(failure.err), errAnthropicNativeStreamIdle)
				require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
				require.Contains(t, recorder.Body.String(), "stream_timeout")
			} else {
				require.Equal(t, test.state, attempt.currentState())
				require.Empty(t, recorder.Body.String())
			}
		})
	}
}
