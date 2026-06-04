//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_ClaudeSemanticErrorFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Remember to join the new community!\"}}\n\n" +
			"data: {\"type\":\"message_stop\"}\n\n",
	))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream:   upstream,
		settingService: testSemanticSettingService(t, PlatformAnthropic, "join the new community", "semantic blocked"),
	}
	account := &Account{
		ID:          201,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
	}

	err := svc.testClaudeAccountConnection(ctx, account, "claude-sonnet-4-5", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "semantic blocked")
	require.Contains(t, recorder.Body.String(), "\"type\":\"error\"")
	require.NotContains(t, recorder.Body.String(), "\"success\":true")
}

func TestAccountTestService_AntigravityAPIKeySemanticErrorFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"Remember to join the new community!\"}}\n\n" +
			"data: {\"type\":\"message_stop\"}\n\n",
	))

	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream:   upstream,
		cfg:            testAccountURLConfig(),
		settingService: testSemanticSettingService(t, PlatformAntigravity, "join the new community", "semantic blocked"),
	}
	account := &Account{
		ID:          202,
		Platform:    PlatformAntigravity,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://api.anthropic.com",
		},
	}

	err := svc.routeAntigravityTest(ctx, account, "claude-sonnet-4-5", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "semantic blocked")
	require.Contains(t, recorder.Body.String(), "\"type\":\"error\"")
	require.NotContains(t, recorder.Body.String(), "\"success\":true")
}
