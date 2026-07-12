package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newFirstTokenTimeoutHandlerContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, recorder
}

func firstTokenTimeoutHandlerError() *service.UpstreamFailoverError {
	return &service.UpstreamFailoverError{
		StatusCode:             http.StatusGatewayTimeout,
		RetryableOnSameAccount: false,
		FirstTokenTimeout:      true,
	}
}

func TestFirstTokenTimeoutExhaustedReturnsProtocol504BeforeStreamCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("通用网关", func(t *testing.T) {
		c, recorder := newFirstTokenTimeoutHandlerContext()
		h := &GatewayHandler{}
		h.handleFailoverExhausted(c, firstTokenTimeoutHandlerError(), service.PlatformOpenAI, true)
		require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
		require.Contains(t, recorder.Body.String(), "upstream_timeout")
	})

	t.Run("Chat Completions", func(t *testing.T) {
		c, recorder := newFirstTokenTimeoutHandlerContext()
		h := &GatewayHandler{}
		h.handleCCFailoverExhausted(c, firstTokenTimeoutHandlerError(), true)
		require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
		require.Contains(t, recorder.Body.String(), "upstream_timeout")
	})

	t.Run("Responses", func(t *testing.T) {
		c, recorder := newFirstTokenTimeoutHandlerContext()
		h := &GatewayHandler{}
		h.handleResponsesFailoverExhausted(c, firstTokenTimeoutHandlerError(), true)
		require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
		require.Contains(t, recorder.Body.String(), "upstream_timeout")
	})

	t.Run("Anthropic Messages", func(t *testing.T) {
		c, recorder := newFirstTokenTimeoutHandlerContext()
		h := &OpenAIGatewayHandler{}
		h.handleAnthropicFailoverExhausted(c, firstTokenTimeoutHandlerError(), true)
		require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
		require.Contains(t, recorder.Body.String(), "upstream_timeout")
	})

	t.Run("Gemini", func(t *testing.T) {
		c, recorder := newFirstTokenTimeoutHandlerContext()
		h := &GatewayHandler{}
		h.handleGeminiFailoverExhausted(c, firstTokenTimeoutHandlerError())
		require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
		require.Contains(t, recorder.Body.String(), "Upstream response timed out")
	})
}
