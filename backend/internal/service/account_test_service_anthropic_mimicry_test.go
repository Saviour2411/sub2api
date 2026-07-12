//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestService_AnthropicAPIKeyMimicryFollowsGlobalFlagAndPassthrough(t *testing.T) {
	tests := []struct {
		name         string
		enabled      bool
		passthrough  bool
		betaOverride string
		wantMimic    bool
	}{
		{name: "开关关闭时发送普通请求", enabled: false, wantMimic: false},
		{name: "开关开启时发送CC模拟请求", enabled: true, wantMimic: true},
		{name: "账号beta覆写与模拟必需beta合并", enabled: true, betaOverride: claude.BetaOAuth + "," + claude.BetaExtendedCacheTTL, wantMimic: true},
		{name: "透传账号不受全局开关影响", enabled: true, passthrough: true, wantMimic: false},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newAnthropicMimicAccountTestAccount(int64(800+index), tt.passthrough)
			if tt.betaOverride != "" {
				account.Credentials[credKeyHeaderOverrideEnabled] = true
				account.Credentials[credKeyHeaderOverrides] = map[string]any{"anthropic-beta": tt.betaOverride}
			}
			repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newAnthropicAccountTestSuccessResponse("连接成功")}}
			svc := &AccountTestService{
				accountRepo:    repo,
				httpUpstream:   upstream,
				cfg:            testAccountURLConfig(),
				settingService: newAnthropicMimicAccountTestSettingService(tt.enabled),
			}
			ginCtx, _ := newTestContext()

			err := svc.TestAccountConnection(ginCtx, account.ID, "claude-sonnet-4-6", "手工测试", AccountTestModeDefault)
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			req := upstream.requests[0]
			payload := readQueuedRequestJSON(t, req)
			assertAnthropicAccountTestMimicRequest(t, req, payload, tt.wantMimic)
			if tt.betaOverride != "" {
				require.Contains(t, anthropicAccountTestHeaderValue(req.Header, "anthropic-beta"), claude.BetaExtendedCacheTTL)
				require.NotContains(t, anthropicAccountTestHeaderValue(req.Header, "anthropic-beta"), claude.BetaOAuth)
			}
			require.Equal(t, "手工测试", gjson.Get(toJSONString(t, payload), "messages.0.content.0.text").String())
		})
	}
}

func TestAccountTestService_AnthropicOAuthUsesFullClaudeCodeMimicry(t *testing.T) {
	account := &Account{
		ID:          850,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "test-oauth-token",
		},
	}
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newAnthropicAccountTestSuccessResponse("oauth success")}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          testAccountURLConfig(),
	}
	ginCtx, _ := newTestContext()

	err := svc.TestAccountConnection(ginCtx, account.ID, "claude-sonnet-4-6", "oauth test", AccountTestModeDefault)
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	payload := readQueuedRequestJSON(t, req)
	payloadJSON := toJSONString(t, payload)

	system := gjson.Get(payloadJSON, "system").Array()
	require.GreaterOrEqual(t, len(system), 2)
	require.True(t, strings.HasPrefix(system[0].Get("text").String(), claudeCodeBillingHeaderPrefix))
	require.True(t, hasClaudeCodePrefix(system[1].Get("text").String()))
	metadata := ParseMetadataUserID(gjson.Get(payloadJSON, "metadata.user_id").String())
	require.NotNil(t, metadata)
	require.True(t, metadata.IsNewFormat)
	require.Equal(t, metadata.SessionID, anthropicAccountTestHeaderValue(req.Header, "X-Claude-Code-Session-Id"))
	require.Equal(t, "Bearer test-oauth-token", anthropicAccountTestHeaderValue(req.Header, "Authorization"))

	beta := anthropicAccountTestHeaderValue(req.Header, "anthropic-beta")
	require.Contains(t, beta, claude.BetaClaudeCode)
	require.Contains(t, beta, claude.BetaOAuth)
	require.Contains(t, beta, claude.BetaContextManagement)
	require.Contains(t, beta, claude.BetaExtendedCacheTTL)
}

func TestAccountTestService_RunTestBackgroundReusesAnthropicMimicryPolicy(t *testing.T) {
	account := newAnthropicMimicAccountTestAccount(899, false)
	repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newAnthropicAccountTestSuccessResponse("后台成功")}}
	svc := &AccountTestService{
		accountRepo:    repo,
		httpUpstream:   upstream,
		cfg:            testAccountURLConfig(),
		settingService: newAnthropicMimicAccountTestSettingService(true),
	}

	result, err := svc.RunTestBackground(context.Background(), account.ID, "claude-sonnet-4-6", "定时测试")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "后台成功", result.ResponseText)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	payload := readQueuedRequestJSON(t, req)
	assertAnthropicAccountTestMimicRequest(t, req, payload, true)
	require.Equal(t, "定时测试", gjson.Get(toJSONString(t, payload), "messages.0.content.0.text").String())
}

func newAnthropicMimicAccountTestAccount(id int64, passthrough bool) *Account {
	extra := map[string]any{}
	if passthrough {
		extra["anthropic_passthrough"] = true
	}
	return &Account{
		ID:          id,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://api.anthropic.com",
		},
		Extra: extra,
	}
}

func newAnthropicMimicAccountTestSettingService(enabled bool) *SettingService {
	repo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyGatewayAnthropicClaudeCodeMimicryEnabled: strconv.FormatBool(enabled),
	}}
	return NewSettingService(repo, &config.Config{})
}

func newAnthropicAccountTestSuccessResponse(text string) *http.Response {
	resp := newJSONResponse(http.StatusOK, "")
	resp.Body = io.NopCloser(strings.NewReader(
		`data: {"type":"content_block_delta","delta":{"text":` + strconv.Quote(text) + `}}` + "\n\n" +
			`data: {"type":"message_stop"}` + "\n\n",
	))
	return resp
}

func assertAnthropicAccountTestMimicRequest(t *testing.T, req *http.Request, payload map[string]any, wantMimic bool) {
	t.Helper()
	payloadJSON := toJSONString(t, payload)
	require.Equal(t, "2023-06-01", anthropicAccountTestHeaderValue(req.Header, "anthropic-version"))
	require.Equal(t, "test-api-key", anthropicAccountTestHeaderValue(req.Header, "x-api-key"))
	require.NotContains(t, anthropicAccountTestHeaderValue(req.Header, "anthropic-beta"), claude.BetaOAuth)

	if !wantMimic {
		require.False(t, gjson.Get(payloadJSON, "system").Exists())
		require.False(t, gjson.Get(payloadJSON, "metadata").Exists())
		require.Empty(t, anthropicAccountTestHeaderValue(req.Header, "User-Agent"))
		require.Empty(t, anthropicAccountTestHeaderValue(req.Header, "X-App"))
		require.Empty(t, anthropicAccountTestHeaderValue(req.Header, "X-Claude-Code-Session-Id"))
		return
	}

	require.GreaterOrEqual(t, len(gjson.Get(payloadJSON, "system").Array()), 2)
	require.True(t, strings.HasPrefix(gjson.Get(payloadJSON, "system.0.text").String(), claudeCodeBillingHeaderPrefix))
	require.True(t, hasClaudeCodePrefix(gjson.Get(payloadJSON, "system.1.text").String()))
	metadata := ParseMetadataUserID(gjson.Get(payloadJSON, "metadata.user_id").String())
	require.NotNil(t, metadata)
	require.True(t, metadata.IsNewFormat)
	require.Equal(t, metadata.SessionID, anthropicAccountTestHeaderValue(req.Header, "X-Claude-Code-Session-Id"))
	require.Equal(t, claude.DefaultHeaders["User-Agent"], anthropicAccountTestHeaderValue(req.Header, "User-Agent"))
	require.Equal(t, claude.DefaultHeaders["X-App"], anthropicAccountTestHeaderValue(req.Header, "X-App"))
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Lang"], anthropicAccountTestHeaderValue(req.Header, "X-Stainless-Lang"))
	require.Equal(t, "stream", anthropicAccountTestHeaderValue(req.Header, "X-Stainless-Helper-Method"))
	require.Equal(t, "application/json", anthropicAccountTestHeaderValue(req.Header, "Accept"))
	require.Contains(t, anthropicAccountTestHeaderValue(req.Header, "anthropic-beta"), claude.BetaClaudeCode)
}

func anthropicAccountTestHeaderValue(header http.Header, name string) string {
	for key, values := range header {
		if strings.EqualFold(key, name) {
			return strings.Join(values, ",")
		}
	}
	return ""
}

func toJSONString(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return string(b)
}
