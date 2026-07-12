package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeClaudeCodeOAuthMimicryBody_RemovesKnownIncompatibilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	account := &Account{ID: 601, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"frequency_penalty":0.5,
		"tools":[
			{"type":"web_search_preview","name":"search"},
			{"name":"lookup","input_schema":{"type":"object"}}
		]
	}`)

	out := (&GatewayService{cfg: &config.Config{}}).sanitizeClaudeCodeOAuthMimicryBody(
		context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointMessages,
	)

	require.False(t, gjson.GetBytes(out, "frequency_penalty").Exists())
	require.Len(t, gjson.GetBytes(out, "tools").Array(), 1)
	require.Equal(t, "lookup", gjson.GetBytes(out, "tools.0.name").String())
}

func TestOAuthCountTokensMimicRestoresSessionHeaderAfterMetadataRemoval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	const sessionID = "3d0cf5c8-d737-4e43-8a0f-7a2739f66d33"
	c.Request.Header.Set("X-Claude-Code-Session-Id", sessionID)
	account := &Account{
		ID:          602,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-token"},
	}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"metadata":{"user_id":"invalid"},
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"max_tokens":1024,
		"temperature":1,
		"stream":false
	}`)
	svc := &GatewayService{cfg: &config.Config{}}

	rememberClaudeCodeMimicSessionID(c, account, body, claudeCodeMimicClientDiscriminator(c))
	sanitized := svc.sanitizeClaudeCodeOAuthMimicryBody(
		context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointCountTokens,
	)
	req, wireBody, err := svc.buildCountTokensRequest(
		context.Background(), c, account, sanitized,
		"oauth-token", "oauth", "claude-sonnet-4-6", true,
	)
	require.NoError(t, err)

	require.False(t, gjson.GetBytes(wireBody, "metadata").Exists())
	require.False(t, gjson.GetBytes(wireBody, "max_tokens").Exists())
	require.Equal(t, sessionID, getHeaderRaw(req.Header, "X-Claude-Code-Session-Id"))
	require.True(t, anthropicBetaTokensContains(getHeaderRaw(req.Header, "anthropic-beta"), claude.BetaTokenCounting))
}

func TestClaudeCodeOAuthMimicry_ReplacesInvalidMetadataEvenWhenPassthroughEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	account := &Account{ID: 603, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"metadata":{"user_id":"invalid","trace_id":"remove-me"},
		"messages":[{"role":"user","content":"hello"}]
	}`)
	repo := &gatewayTTLSettingRepo{data: map[string]string{SettingKeyEnableMetadataPassthrough: "true"}}
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	t.Cleanup(func() { gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{}) })
	svc := &GatewayService{cfg: &config.Config{}, settingService: NewSettingService(repo, &config.Config{})}

	mimicBody := svc.applyClaudeCodeOAuthMimicryToBody(
		context.Background(), c, account, body, nil, "claude-sonnet-4-6", "client",
	)
	req, wireBody, err := svc.buildUpstreamRequest(
		context.Background(), c, account, mimicBody, "oauth-token", "oauth", "claude-sonnet-4-6", false, true,
	)
	require.NoError(t, err)

	metadataUserID := gjson.GetBytes(wireBody, "metadata.user_id").String()
	require.True(t, IsValidClaudeCodeMetadataUserID(metadataUserID))
	require.True(t, ParseMetadataUserID(metadataUserID).IsNewFormat)
	require.False(t, gjson.GetBytes(wireBody, "metadata.trace_id").Exists())
	require.Equal(t, ParseMetadataUserID(metadataUserID).SessionID, getHeaderRaw(req.Header, "X-Claude-Code-Session-Id"))
}

func TestClaudeCodeOAuthMimicry_UsesSameSessionDiscriminatorAcrossEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"same conversation"}]}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)
	parsed.SessionContext = &SessionContext{ClientIP: "203.0.113.5", UserAgent: "client/1.0", APIKeyID: 99}
	account := &Account{ID: 604, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	discriminator := claudeCodeMimicRequestDiscriminator(nil, parsed)

	messagesUserID := (&GatewayService{}).buildOAuthMetadataUserID(nil, parsed, account, nil)
	messagesSession := ParseMetadataUserID(messagesUserID).SessionID
	countContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	countContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	rememberClaudeCodeMimicSessionID(countContext, account, body, discriminator)
	countSession := claudeCodeMimicSessionID(countContext, account, nil)
	convertedUserID := (&GatewayService{}).buildOAuthMetadataUserIDFromBody(
		context.Background(), nil, account, nil, body, discriminator,
	)

	require.Equal(t, messagesSession, countSession)
	require.Equal(t, messagesSession, ParseMetadataUserID(convertedUserID).SessionID)

	parsed.SessionContext.APIKeyID = 100
	otherContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	otherContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	rememberClaudeCodeMimicSessionID(otherContext, account, body, claudeCodeMimicRequestDiscriminator(nil, parsed))
	require.NotEqual(t, countSession, claudeCodeMimicSessionID(otherContext, account, nil))
}
