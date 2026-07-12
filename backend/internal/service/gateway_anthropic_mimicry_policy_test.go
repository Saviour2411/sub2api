//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func isCompleteClaudeCodeBillingBlock(text string) bool {
	return strings.HasPrefix(text, claudeCodeBillingHeaderPrefix) &&
		strings.Contains(text, claudeCodeEntrypointMarker)
}

func TestShouldMimicClaudeCodeForAccount_StrategyMatrix(t *testing.T) {
	tests := []struct {
		name       string
		account    *Account
		isCC       bool
		globalFlag bool
		want       bool
	}{
		{name: "空账号", account: nil, globalFlag: true, want: false},
		{name: "OAuth始终模拟", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, want: true},
		{name: "SetupToken始终模拟", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}, want: true},
		{name: "真实CC的OAuth不重复模拟", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, isCC: true, globalFlag: true, want: false},
		{name: "APIKey开关关闭", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, want: false},
		{name: "APIKey开关开启", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, globalFlag: true, want: true},
		{name: "APIKey透传优先", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: map[string]any{"anthropic_passthrough": true}}, globalFlag: true, want: false},
		{name: "真实CC的APIKey不重复模拟", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, isCC: true, globalFlag: true, want: false},
		{name: "Bedrock不模拟", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}, globalFlag: true, want: false},
		{name: "Vertex服务账号不模拟", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeServiceAccount}, globalFlag: true, want: false},
		{name: "Antigravity不模拟", account: &Account{Platform: PlatformAntigravity, Type: AccountTypeAPIKey}, globalFlag: true, want: false},
		{name: "Gemini不模拟", account: &Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}, globalFlag: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldMimicClaudeCodeForAccount(tt.account, tt.isCC, tt.globalFlag))
		})
	}
}

func TestPrependClaudeCodeMimicSystemBlocks_PreservesOriginalBlocksAndIsIdempotent(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
			{"type":"text","text":"原始系统块","cache_control":{"type":"ephemeral","ttl":"1h"}},
			{"type":"text","text":"第二个系统块","cache_control":{"type":"ephemeral","ttl":"5m"}}
		],
		"messages":[{"role":"user","content":[{"type":"text","text":"你好"}]}]
	}`)

	once := prependClaudeCodeMimicSystemBlocks(body)
	blocks := gjson.GetBytes(once, "system").Array()
	require.Len(t, blocks, 4)
	require.True(t, strings.HasPrefix(blocks[0].Get("text").String(), claudeCodeBillingHeaderPrefix))
	require.True(t, hasClaudeCodePrefix(blocks[1].Get("text").String()))
	require.JSONEq(t, `{"type":"text","text":"原始系统块","cache_control":{"type":"ephemeral","ttl":"1h"}}`, blocks[2].Raw)
	require.JSONEq(t, `{"type":"text","text":"第二个系统块","cache_control":{"type":"ephemeral","ttl":"5m"}}`, blocks[3].Raw)
	require.Equal(t, "1h", blocks[2].Get("cache_control.ttl").String())
	require.Equal(t, "5m", blocks[3].Get("cache_control.ttl").String())

	twice := prependClaudeCodeMimicSystemBlocks(once)
	require.JSONEq(t, string(once), string(twice))
	require.Len(t, gjson.GetBytes(twice, "system").Array(), 4)
}

func TestApplyClaudeCodeAPIKeyMimicryToBody_PreservesSystemTextOrderAndCacheControl(t *testing.T) {
	account := &Account{ID: 705, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	c := newAPIKeyMimicTestContext("opencode/1.0", "")
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
			{"type":"text","text":"You are OpenCode, the best coding agent on the planet.","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"Keep this system block unchanged.","cache_control":{"type":"ephemeral"}}
		],
		"messages":[{"role":"user","content":"hello"}]
	}`)

	out, _ := (&GatewayService{}).applyClaudeCodeAPIKeyMimicryToBody(
		context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointMessages,
	)
	system := gjson.GetBytes(out, "system").Array()

	require.Len(t, system, 4)
	require.True(t, strings.HasPrefix(system[0].Get("text").String(), claudeCodeBillingHeaderPrefix))
	require.True(t, hasClaudeCodePrefix(system[1].Get("text").String()))
	require.Equal(t, "You are OpenCode, the best coding agent on the planet.", system[2].Get("text").String())
	require.Equal(t, "Keep this system block unchanged.", system[3].Get("text").String())
	require.JSONEq(t, `{"type":"ephemeral","ttl":"5m"}`, system[2].Get("cache_control").Raw)
	require.JSONEq(t, `{"type":"ephemeral"}`, system[3].Get("cache_control").Raw)
}

func TestPrependClaudeCodeMimicSystemBlocks_ReplacesIncompleteBillingSignal(t *testing.T) {
	body := []byte(`{
		"system":[{"type":"text","text":"x-anthropic-billing-header: malformed"}],
		"messages":[{"role":"user","content":"hello"}]
	}`)

	out := prependClaudeCodeMimicSystemBlocks(body)

	require.True(t, isCompleteClaudeCodeBillingBlock(gjson.GetBytes(out, "system.0.text").String()))
	require.Equal(t, "x-anthropic-billing-header: malformed", gjson.GetBytes(out, "system.2.text").String())
}

func TestPrependClaudeCodeMimicSystemBlocks_OnlyReusesLeadingIdentitySignals(t *testing.T) {
	body := []byte(`{
		"system":[
			{"type":"text","text":"original"},
			{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.195.abc; cc_entrypoint=cli;"},
			{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}
		],
		"messages":[{"role":"user","content":"hello"}]
	}`)

	out := prependClaudeCodeMimicSystemBlocks(body)

	require.True(t, isCompleteClaudeCodeBillingBlock(gjson.GetBytes(out, "system.0.text").String()))
	require.True(t, hasClaudeCodePrefix(gjson.GetBytes(out, "system.1.text").String()))
	require.Equal(t, "original", gjson.GetBytes(out, "system.2.text").String())
	require.Contains(t, gjson.GetBytes(out, "system.3.text").String(), "cc_entrypoint=cli")
	require.True(t, hasClaudeCodePrefix(gjson.GetBytes(out, "system.4.text").String()))
}

func TestPrependClaudeCodeMimicSystemBlocks_DoesNotReuseMismatchedBillingVersion(t *testing.T) {
	body := []byte(`{
		"system":[
			{"type":"text","text":"x-anthropic-billing-header: cc_version=1.0.0.abc; cc_entrypoint=claude-vscode;"},
			{"type":"text","text":"You are Claude Code, Anthropic's official CLI for Claude."}
		],
		"messages":[{"role":"user","content":"hello"}]
	}`)

	out := prependClaudeCodeMimicSystemBlocks(body)

	require.Contains(t, gjson.GetBytes(out, "system.0.text").String(), "cc_version="+claude.CLICurrentVersion+".")
	require.Contains(t, gjson.GetBytes(out, "system.0.text").String(), "cc_entrypoint=cli")
	require.True(t, hasClaudeCodePrefix(gjson.GetBytes(out, "system.1.text").String()))
	require.Contains(t, gjson.GetBytes(out, "system.2.text").String(), "cc_version=1.0.0.abc")
}

func TestBuildAPIKeyMimicMetadataUserID_UsesModernJSONAndReplacesInvalidMetadata(t *testing.T) {
	account := &Account{
		ID:       701,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"account_uuid": "account-701"},
	}
	ctx := newAPIKeyMimicTestContext("curl/8.7.1", "")
	body := []byte(`{
		"metadata":{"user_id":"not-a-valid-claude-code-user","trace_id":"trace-1"},
		"messages":[{"role":"user","content":[{"type":"text","text":"第一轮问题"}]}]
	}`)

	userID := buildAPIKeyMimicMetadataUserID(ctx, account, body)
	parsed := ParseMetadataUserID(userID)
	require.NotNil(t, parsed)
	require.True(t, parsed.IsNewFormat)
	require.Len(t, parsed.DeviceID, 64)
	require.Equal(t, "account-701", parsed.AccountUUID)
	require.NotEmpty(t, parsed.SessionID)
	require.True(t, gjson.Valid(userID), "现代 metadata.user_id 应为 JSON 字符串")

	out := forceClaudeCodeMetadataUserID(body, userID)
	require.Equal(t, userID, gjson.GetBytes(out, "metadata.user_id").String())
	require.False(t, gjson.GetBytes(out, "metadata.trace_id").Exists())
	require.NotEqual(t, "not-a-valid-claude-code-user", gjson.GetBytes(out, "metadata.user_id").String())
}

func TestBuildAPIKeyMimicMetadataUserID_SessionIsStableWhenConversationAppends(t *testing.T) {
	account := &Account{ID: 702, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	ctx := newAPIKeyMimicTestContext("my-client/1.0", "")
	firstRound := []byte(`{
		"messages":[{"role":"user","content":[{"type":"text","text":"保持这一会话"}]}]
	}`)
	appendedRound := []byte(`{
		"messages":[
			{"role":"user","content":[{"type":"text","text":"保持这一会话"}]},
			{"role":"assistant","content":[{"type":"text","text":"上一轮回答"}]},
			{"role":"user","content":[{"type":"text","text":"追加问题"}]}
		]
	}`)

	first := ParseMetadataUserID(buildAPIKeyMimicMetadataUserID(ctx, account, firstRound))
	appended := ParseMetadataUserID(buildAPIKeyMimicMetadataUserID(ctx, account, appendedRound))
	require.NotNil(t, first)
	require.NotNil(t, appended)
	require.Equal(t, first.DeviceID, appended.DeviceID)
	require.Equal(t, first.SessionID, appended.SessionID)
}

func TestBuildAPIKeyMimicMetadataUserID_PrefersExistingMetadataThenSessionHeader(t *testing.T) {
	account := &Account{ID: 703, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	headerSession := "11111111-2222-4333-8444-555555555555"
	ctx := newAPIKeyMimicTestContext("my-client/1.0", headerSession)

	fromHeader := ParseMetadataUserID(buildAPIKeyMimicMetadataUserID(ctx, account, []byte(`{
		"metadata":{"user_id":"invalid"},
		"messages":[{"role":"user","content":"问题"}]
	}`)))
	require.NotNil(t, fromHeader)
	require.Equal(t, headerSession, fromHeader.SessionID)

	existingSession := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	existingUserID := FormatMetadataUserID(strings.Repeat("a", 64), "existing-account", existingSession, claude.CLICurrentVersion)
	bodyWithExisting := []byte(`{
		"metadata":{"user_id":` + mustMarshalJSONString(t, existingUserID) + `},
		"messages":[{"role":"user","content":"问题"}]
	}`)
	fromExisting := ParseMetadataUserID(buildAPIKeyMimicMetadataUserID(ctx, account, bodyWithExisting))
	require.NotNil(t, fromExisting)
	require.Equal(t, existingSession, fromExisting.SessionID)
}

func TestBuildAPIKeyMimicMetadataUserID_RejectsNonCanonicalSessionInputs(t *testing.T) {
	account := &Account{ID: 706, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	ctx := newAPIKeyMimicTestContext("my-client/1.0", "{11111111-2222-4333-8444-555555555555}")
	nonCanonicalUserID := `{"device_id":"` + strings.Repeat("a", 64) + `","account_uuid":"","session_id":"{aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee}"}`
	body := []byte(`{
		"metadata":{"user_id":` + mustMarshalJSONString(t, nonCanonicalUserID) + `},
		"messages":[{"role":"user","content":"问题"}]
	}`)

	parsed := ParseMetadataUserID(buildAPIKeyMimicMetadataUserID(ctx, account, body))
	require.NotNil(t, parsed)
	require.NotEqual(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", parsed.SessionID)
	require.NotEqual(t, "11111111-2222-4333-8444-555555555555", parsed.SessionID)
	require.True(t, IsValidClaudeCodeMetadataUserID(FormatMetadataUserID(parsed.DeviceID, parsed.AccountUUID, parsed.SessionID, claude.CLICurrentVersion)))
}

func TestApplyClaudeCodeAPIKeyMimicryToBody_DoesNotApplySecondModelMapping(t *testing.T) {
	account := &Account{
		ID:       704,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "claude-opus-4-7",
				"*":                 "second-hop-model",
			},
		},
	}
	c := newAPIKeyMimicTestContext("my-client/1.0", "")
	mappedModel := account.GetMappedModel("claude-sonnet-4-5")
	body := []byte(`{"model":"claude-opus-4-7","temperature":0.7,"messages":[{"role":"user","content":"hello"}]}`)
	svc := &GatewayService{}

	out, model := svc.applyClaudeCodeAPIKeyMimicryToBody(
		context.Background(), c, account, body, mappedModel, anthropicMimicEndpointMessages,
	)

	require.Equal(t, "claude-opus-4-7", model)
	require.Equal(t, "claude-opus-4-7", gjson.GetBytes(out, "model").String())
	require.False(t, gjson.GetBytes(out, "temperature").Exists(), "能力清洗必须按第一次映射后的 Opus 模型执行")
}

type anthropicMimicCaptureUpstream struct {
	header http.Header
	body   []byte
}

type anthropicMimicSettingsRepo struct {
	*customFeatureSettingsRepoStub
}

func (r *anthropicMimicSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (u *anthropicMimicCaptureUpstream) capture(req *http.Request) (*http.Response, error) {
	u.header = req.Header.Clone()
	u.body, _ = io.ReadAll(req.Body)
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"type":"error","error":{"type":"invalid_request_error","message":"test stop"}}`,
		)),
	}, nil
}

func (u *anthropicMimicCaptureUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.capture(req)
}

func (u *anthropicMimicCaptureUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.capture(req)
}

func TestAnthropicAPIKeyMimicry_AllRequestEntrypointsSharePolicyAndMappedModel(t *testing.T) {
	settings := DefaultGatewaySettings()
	settings.AnthropicClaudeCodeMimicryEnabled = true
	betaPolicy, err := json.Marshal(BetaPolicySettings{Rules: []BetaPolicyRule{{
		BetaToken:      claude.BetaExtendedCacheTTL,
		Action:         BetaPolicyActionFilter,
		Scope:          BetaPolicyScopeAPIKey,
		ModelWhitelist: []string{"vendor-sonnet"},
		FallbackAction: BetaPolicyActionPass,
	}}})
	require.NoError(t, err)
	settingService := NewSettingService(&anthropicMimicSettingsRepo{
		customFeatureSettingsRepoStub: &customFeatureSettingsRepoStub{values: map[string]string{
			SettingKeyBetaPolicySettings: string(betaPolicy),
		}},
	}, &config.Config{})
	settingService.storeGatewaySettingsCache(settings, time.Hour)

	tests := []struct {
		name        string
		path        string
		body        []byte
		countTokens bool
		run         func(*testing.T, *GatewayService, *gin.Context, *Account, []byte)
	}{
		{
			name: "原生 Messages",
			path: "/v1/messages",
			body: []byte(`{"model":"claude-sonnet-4-5","max_tokens":64,"messages":[{"role":"user","content":"hello"}]}`),
			run: func(t *testing.T, s *GatewayService, c *gin.Context, account *Account, body []byte) {
				parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, err)
				_, _ = s.Forward(c.Request.Context(), c, account, parsed)
			},
		},
		{
			name:        "Count Tokens",
			path:        "/v1/messages/count_tokens",
			body:        []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}]}`),
			countTokens: true,
			run: func(t *testing.T, s *GatewayService, c *gin.Context, account *Account, body []byte) {
				parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
				require.NoError(t, err)
				_ = s.ForwardCountTokens(c.Request.Context(), c, account, parsed)
			},
		},
		{
			name: "Chat Completions 转 Anthropic",
			path: "/v1/chat/completions",
			body: []byte(`{"model":"claude-sonnet-4-5","stream":false,"messages":[{"role":"user","content":"hello"}]}`),
			run: func(_ *testing.T, s *GatewayService, c *gin.Context, account *Account, body []byte) {
				_, _ = s.ForwardAsChatCompletions(c.Request.Context(), c, account, body, nil)
			},
		},
		{
			name: "Responses 转 Anthropic",
			path: "/v1/responses",
			body: []byte(`{"model":"claude-sonnet-4-5","stream":false,"input":[{"role":"user","content":"hello"}]}`),
			run: func(_ *testing.T, s *GatewayService, c *gin.Context, account *Account, body []byte) {
				_, _ = s.ForwardAsResponses(c.Request.Context(), c, account, body, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &anthropicMimicCaptureUpstream{}
			svc := &GatewayService{
				cfg:            &config.Config{},
				httpUpstream:   upstream,
				settingService: settingService,
			}
			account := &Account{
				ID:          705,
				Name:        "mimic-entrypoint",
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "sk-test",
					"base_url": "https://api.anthropic.com",
					"model_mapping": map[string]any{
						"claude-sonnet-4-5": "vendor-sonnet",
						"*":                 "second-hop-model",
					},
				},
			}
			c := newAPIKeyMimicTestContext("third-party-client/1.0", "")
			c.Request.URL.Path = tt.path
			c.Request.Header.Set("Anthropic-Beta", claude.BetaExtendedCacheTTL)

			tt.run(t, svc, c, account, tt.body)

			require.NotEmpty(t, upstream.body)
			require.Equal(t, "vendor-sonnet", gjson.GetBytes(upstream.body, "model").String())
			require.True(t, strings.HasPrefix(gjson.GetBytes(upstream.body, "system.0.text").String(), claudeCodeBillingHeaderPrefix))
			require.Equal(t, "cli", anthropicAccountTestHeaderValue(upstream.header, "X-App"))
			require.Contains(t, anthropicAccountTestHeaderValue(upstream.header, "User-Agent"), "claude-cli/")
			require.NotEmpty(t, anthropicAccountTestHeaderValue(upstream.header, "X-Claude-Code-Session-Id"))
			require.NotContains(t, anthropicAccountTestHeaderValue(upstream.header, "anthropic-beta"), claude.BetaOAuth)
			require.NotContains(t, anthropicAccountTestHeaderValue(upstream.header, "anthropic-beta"), claude.BetaExtendedCacheTTL)
			if tt.countTokens {
				require.False(t, gjson.GetBytes(upstream.body, "metadata").Exists())
				require.Contains(t, anthropicAccountTestHeaderValue(upstream.header, "anthropic-beta"), claude.BetaTokenCounting)
			} else {
				require.True(t, gjson.GetBytes(upstream.body, "metadata.user_id").Exists())
			}
		})
	}
}

func newAPIKeyMimicTestContext(userAgent, sessionID string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.RemoteAddr = "203.0.113.10:43210"
	c.Request.Header.Set("User-Agent", userAgent)
	if sessionID != "" {
		c.Request.Header.Set("X-Claude-Code-Session-Id", sessionID)
	}
	return c
}

func mustMarshalJSONString(t *testing.T, value string) string {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return string(b)
}
