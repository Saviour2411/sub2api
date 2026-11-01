package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBalanceInflightLimitIsRetryableAcrossProtocols(t *testing.T) {
	status, code, message, retry := billingErrorDetails(service.ErrBalanceInflightLimit)
	require.Equal(t, 429, status)
	require.Equal(t, "balance_inflight_limit", code)
	require.Equal(t, 1, retry)
	for _, protocol := range []string{"openai", "anthropic", "chat", "responses", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			switch protocol {
			case "openai":
				(&OpenAIGatewayHandler{}).handleStreamingAwareError(c, status, code, message, false)
			case "anthropic":
				(&GatewayHandler{}).handleStreamingAwareError(c, status, code, message, false)
			case "chat":
				(&GatewayHandler{}).chatCompletionsErrorResponse(c, status, code, message)
			case "responses":
				(&GatewayHandler{}).responsesErrorResponse(c, status, code, message)
			case "gemini":
				googleErrorWithType(c, status, code, "", message)
			}
			require.Equal(t, 429, w.Code)
			require.Equal(t, "1", w.Header().Get("Retry-After"))
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			err, ok := body["error"].(map[string]any)
			require.True(t, ok)
			if protocol == "gemini" {
				require.Equal(t, "RESOURCE_EXHAUSTED", err["status"])
				require.Contains(t, w.Body.String(), "balance_inflight_limit")
			} else {
				require.Equal(t, "rate_limit_error", err["type"])
				require.Equal(t, "balance_inflight_limit", err["code"])
			}
		})
	}
	status, _, _, retry = billingErrorDetails(service.ErrInsufficientBalance)
	require.Equal(t, 403, status)
	require.Zero(t, retry)
}

func TestSlotBackendFailureHasDiagnosticCode(t *testing.T) {
	status, typ, code, _ := concurrencyErrorResponse(&service.SlotOperationError{Phase: "acquire", SlotType: "user", Err: http.ErrHandlerTimeout}, "user")
	require.Equal(t, 503, status)
	require.Equal(t, "api_error", typ)
	require.Equal(t, "gateway_concurrency_backend_unavailable", code)
}
