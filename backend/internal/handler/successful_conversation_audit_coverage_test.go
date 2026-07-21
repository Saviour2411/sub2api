package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSuccessfulConversationAuditCoversAllRESTHandlers(t *testing.T) {
	tests := []struct {
		file     string
		function string
	}{
		{file: "gateway_handler.go", function: "Messages"},
		{file: "gateway_handler_responses.go", function: "Responses"},
		{file: "gateway_handler_chat_completions.go", function: "ChatCompletions"},
		{file: "gemini_v1beta_handler.go", function: "GeminiV1BetaModels"},
		{file: "openai_gateway_handler.go", function: "Responses"},
		{file: "openai_gateway_handler.go", function: "Messages"},
		{file: "openai_chat_completions.go", function: "ChatCompletions"},
		{file: "openai_images.go", function: "Images"},
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.function, func(t *testing.T) {
			source := stripGoComments(goFunctionSource(t, tt.file, tt.function))
			beginIndex := strings.Index(source, "beginSuccessfulConversationAuditCapture")
			recordIndex := strings.Index(source, "recordSuccessfulConversationAudit")
			require.NotEqual(t, -1, beginIndex, "REST handler must reserve response capture")
			require.NotEqual(t, -1, recordIndex, "REST handler must record a successful conversation")
			require.Less(t, beginIndex, recordIndex, "response capture must start before the success record")
			require.Contains(t, source, "!result.ClientDisconnect", "client-disconnected responses must not be recorded")
		})
	}

	webSocketSource := stripGoComments(goFunctionSource(t, "openai_gateway_handler.go", "ResponsesWebSocket"))
	require.NotContains(t, webSocketSource, "beginSuccessfulConversationAuditCapture")
	require.NotContains(t, webSocketSource, "recordSuccessfulConversationAudit")
}

func TestAuditResponseCaptureWriterLimitsJSONAndSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name  string
		write func(*gin.Context, []byte) error
	}{
		{
			name: "json",
			write: func(c *gin.Context, payload []byte) error {
				_, err := c.Writer.Write(payload)
				return err
			},
		},
		{
			name: "sse",
			write: func(c *gin.Context, payload []byte) error {
				_, err := c.Writer.WriteString("data: " + string(payload) + "\n\n")
				return err
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			original := c.Writer
			capture, restore := attachAuditResponseCapture(c)
			require.NotNil(t, capture)

			payload := bytes.Repeat([]byte("x"), service.ContentModerationLocalAuditResponseCaptureLimitBytes+4096)
			require.NoError(t, tt.write(c, payload))
			require.Len(t, capture.Bytes(), service.ContentModerationLocalAuditResponseCaptureLimitBytes)
			require.Greater(t, recorder.Body.Len(), len(capture.Bytes()))

			restore()
			require.Same(t, original, c.Writer)
		})
	}
}
