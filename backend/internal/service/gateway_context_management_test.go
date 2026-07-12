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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildAnthropicMimicRequests_BillingMatchesForcedCurrentUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 420, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"account_uuid": "account-420"}}
	initialUserID := FormatMetadataUserID(strings.Repeat("a", 64), "account-420", "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", claude.CLICurrentVersion)
	encodedUserID, err := json.Marshal(initialUserID)
	require.NoError(t, err)
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=1.0.0.old; cc_entrypoint=cli;"}],
		"metadata":{"user_id":` + string(encodedUserID) + `},
		"messages":[{"role":"user","content":"hello"}]
	}`)
	svc := &GatewayService{cfg: &config.Config{}, identityService: NewIdentityService(&identityCacheStub{})}

	tests := []struct {
		name          string
		checkMetadata bool
		build         func(*gin.Context) (*http.Request, []byte, error)
	}{
		{
			name:          "Messages",
			checkMetadata: true,
			build: func(c *gin.Context) (*http.Request, []byte, error) {
				return svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, true)
			},
		},
		{
			name: "Count Tokens",
			build: func(c *gin.Context) (*http.Request, []byte, error) {
				return svc.buildCountTokensRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			c.Request.Header.Set("User-Agent", "claude-cli/1.0.0 (external, cli)")
			req, wireBody, err := tt.build(c)
			require.NoError(t, err)
			require.Equal(t, claude.DefaultHeaders["User-Agent"], getHeaderRaw(req.Header, "User-Agent"))
			require.Contains(t, gjson.GetBytes(wireBody, "system.0.text").String(), "cc_version="+claude.CLICurrentVersion+".")
			require.NotContains(t, gjson.GetBytes(wireBody, "system.0.text").String(), "cc_version=1.0.0.")
			if tt.checkMetadata {
				metadataUserID := gjson.GetBytes(wireBody, "metadata.user_id").String()
				require.True(t, IsValidClaudeCodeMetadataUserID(metadataUserID))
				require.True(t, ParseMetadataUserID(metadataUserID).IsNewFormat)
			}
		})
	}
}

func TestBuildUpstreamRequest_TTL1hInjectionUsesFinalBetaAndPreservesClientTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 421, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[
			{"type":"text","text":"client","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"proxy","cache_control":{"type":"ephemeral"}}
		],
		"messages":[{"role":"user","content":"hello"}]
	}`)

	t.Run("最终 beta 支持 1h 时仅补缺失 TTL", func(t *testing.T) {
		repo := &gatewayTTLSettingRepo{data: map[string]string{SettingKeyEnableAnthropicCacheTTL1hInjection: "true"}}
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
		svc := &GatewayService{cfg: &config.Config{}, settingService: NewSettingService(repo, &config.Config{})}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

		req, wireBody, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", "oauth", "claude-sonnet-4-6", false, true)
		require.NoError(t, err)
		require.True(t, anthropicBetaTokensContains(getHeaderRaw(req.Header, "anthropic-beta"), claude.BetaExtendedCacheTTL))
		require.Equal(t, "5m", gjson.GetBytes(wireBody, "system.0.cache_control.ttl").String())
		require.Equal(t, "1h", gjson.GetBytes(wireBody, "system.1.cache_control.ttl").String())
	})

	t.Run("最终 beta 过滤 1h 时不得重新注入", func(t *testing.T) {
		policyJSON, err := json.Marshal(BetaPolicySettings{Rules: []BetaPolicyRule{{
			BetaToken: claude.BetaExtendedCacheTTL,
			Action:    BetaPolicyActionFilter,
			Scope:     BetaPolicyScopeOAuth,
		}}})
		require.NoError(t, err)
		repo := &gatewayTTLSettingRepo{data: map[string]string{
			SettingKeyEnableAnthropicCacheTTL1hInjection: "true",
			SettingKeyBetaPolicySettings:                 string(policyJSON),
		}}
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
		svc := &GatewayService{cfg: &config.Config{}, settingService: NewSettingService(repo, &config.Config{})}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
		require.NoError(t, svc.cacheBetaPolicyForRequest(context.Background(), c, account, "claude-sonnet-4-6"))
		sanitized := svc.sanitizeClaudeCodeOAuthMimicryBody(context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointMessages)

		req, wireBody, err := svc.buildUpstreamRequest(context.Background(), c, account, sanitized, "oauth-token", "oauth", "claude-sonnet-4-6", false, true)
		require.NoError(t, err)
		require.False(t, anthropicBetaTokensContains(getHeaderRaw(req.Header, "anthropic-beta"), claude.BetaExtendedCacheTTL))
		require.Equal(t, "5m", gjson.GetBytes(wireBody, "system.0.cache_control.ttl").String())
		require.False(t, gjson.GetBytes(wireBody, "system.1.cache_control.ttl").Exists())
	})
}

func TestBuildAnthropicMimicRequests_FinalBetaHonorsBlockPolicy(t *testing.T) {
	policyJSON, err := json.Marshal(BetaPolicySettings{Rules: []BetaPolicyRule{{
		BetaToken: claude.BetaClaudeCode,
		Action:    BetaPolicyActionBlock,
		Scope:     BetaPolicyScopeAPIKey,
	}}})
	require.NoError(t, err)
	repo := &gatewayTTLSettingRepo{data: map[string]string{SettingKeyBetaPolicySettings: string(policyJSON)}}
	svc := &GatewayService{cfg: &config.Config{}, settingService: NewSettingService(repo, &config.Config{})}
	account := &Account{ID: 422, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hello"}]}`)

	tests := []struct {
		name  string
		build func(*gin.Context) error
	}{
		{
			name: "Messages",
			build: func(c *gin.Context) error {
				_, _, buildErr := svc.buildUpstreamRequest(context.Background(), c, account, body, "api-key", "apikey", "claude-sonnet-4-6", false, true)
				return buildErr
			},
		},
		{
			name: "Count Tokens",
			build: func(c *gin.Context) error {
				_, _, buildErr := svc.buildCountTokensRequest(context.Background(), c, account, body, "api-key", "apikey", "claude-sonnet-4-6", true)
				return buildErr
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			var blocked *BetaBlockedError
			require.ErrorAs(t, tt.build(c), &blocked)
		})
	}
}

// ============================================================================
// 背景
// ============================================================================
//
// Anthropic 上游对 body.context_management 字段实施 Pydantic schema 校验：
// 当且仅当 anthropic-beta header 含 context-management-2025-06-27 时接受。
// 否则报：
//   "context_management: Extra inputs are not permitted"
//
// 本仓采用能力维度对称约束（与 Bedrock 路径的 sanitizeBedrockFieldsForBetaTokens
// 对称）：在所有 Anthropic 直连出口，按最终 anthropic-beta header 是否含上述 token
// 决定 body 是否保留同名字段。
//
// 本文件覆盖：
//   1) sanitizeAnthropicBodyForBetaTokens 纯函数
//   2) anthropicBetaTokensContains 解析辅助函数
//   3) computeFinalAnthropicBeta / computeFinalCountTokensAnthropicBeta 各路径
//   4) normalizeClaudeOAuthRequestBody 的 context_management 补齐行为（不再按 model 短路）

// ============================================================================
// anthropicBetaTokensContains
// ============================================================================

func TestAnthropicBetaTokensContains_EmptyInputs(t *testing.T) {
	require.False(t, anthropicBetaTokensContains("", "context-management-2025-06-27"))
	require.False(t, anthropicBetaTokensContains("oauth-2025-04-20", ""))
}

func TestAnthropicBetaTokensContains_SingleToken(t *testing.T) {
	require.True(t, anthropicBetaTokensContains("context-management-2025-06-27", "context-management-2025-06-27"))
}

func TestAnthropicBetaTokensContains_MultiTokenComma(t *testing.T) {
	header := "oauth-2025-04-20,context-management-2025-06-27,interleaved-thinking-2025-05-14"
	require.True(t, anthropicBetaTokensContains(header, "context-management-2025-06-27"))
	require.True(t, anthropicBetaTokensContains(header, "oauth-2025-04-20"))
	require.False(t, anthropicBetaTokensContains(header, "fast-mode-2026-02-01"))
}

func TestAnthropicBetaTokensContains_ToleratesWhitespace(t *testing.T) {
	header := "oauth-2025-04-20 , context-management-2025-06-27 ,  interleaved-thinking-2025-05-14"
	require.True(t, anthropicBetaTokensContains(header, "context-management-2025-06-27"))
}

func TestAnthropicBetaTokensContains_SubstringNotMatched(t *testing.T) {
	// 严格 token 比较，不应被子串误匹配
	require.False(t, anthropicBetaTokensContains("context-management-2025-06-27-rev2", "context-management-2025-06-27"),
		"必须按 token 边界匹配，不允许 prefix 子串误命中")
}

// ============================================================================
// sanitizeAnthropicBodyForBetaTokens
// ============================================================================

func TestSanitizeAnthropicBodyForBetaTokens_NoFieldNoChange(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-4-5","messages":[]}`)
	out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20")
	require.False(t, changed)
	require.Equal(t, string(body), string(out))
}

func TestSanitizeAnthropicBodyForBetaTokens_FieldKeptWhenBetaPresent(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-7","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	out, changed := sanitizeAnthropicBodyForBetaTokens(body,
		"oauth-2025-04-20,context-management-2025-06-27,interleaved-thinking-2025-05-14")
	require.False(t, changed)
	require.True(t, gjson.GetBytes(out, "context_management").Exists())
	require.Equal(t, "clear_thinking_20251015",
		gjson.GetBytes(out, "context_management.edits.0.type").String())
}

func TestSanitizeAnthropicBodyForBetaTokens_FieldStrippedWhenBetaMissing(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	out, changed := sanitizeAnthropicBodyForBetaTokens(body, "oauth-2025-04-20,interleaved-thinking-2025-05-14")
	require.True(t, changed)
	require.False(t, gjson.GetBytes(out, "context_management").Exists(),
		"header 不含 context-management beta 时必须 strip 同名字段")
}

func TestSanitizeAnthropicBodyForBetaTokens_FieldStrippedWhenBetaEmpty(t *testing.T) {
	body := []byte(`{"context_management":{"edits":[]},"messages":[]}`)
	out, changed := sanitizeAnthropicBodyForBetaTokens(body, "")
	require.True(t, changed)
	require.False(t, gjson.GetBytes(out, "context_management").Exists())
}

func TestSanitizeAnthropicBodyForBetaTokens_EmptyBody(t *testing.T) {
	out, changed := sanitizeAnthropicBodyForBetaTokens([]byte{}, "")
	require.False(t, changed)
	require.Empty(t, out)

	out, changed = sanitizeAnthropicBodyForBetaTokens(nil, "")
	require.False(t, changed)
	require.Empty(t, out)
}

// ★ 关键回归断言：能力维度 sanitize 解决了 "真 CC + haiku" 路径的过度删除问题。
// 真实 Claude Code CLI 2.1.87+ 客户端 header 含 context-management beta；
// 即使 model 是 haiku，sanitize 也不应剥离功能字段。
func TestSanitizeAnthropicBodyForBetaTokens_HaikuRealCCClientPreservesField(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015","keep":"all"}]},"messages":[]}`)
	// 真 Claude Code CLI 2.1.87+ 客户端 header 含 context-management beta
	clientBeta := "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,context-management-2025-06-27"
	out, changed := sanitizeAnthropicBodyForBetaTokens(body, clientBeta)
	require.False(t, changed,
		"真 CC 客户端 header 含 context-management beta 时，haiku body 字段必须保留（功能不丢）")
	require.True(t, gjson.GetBytes(out, "context_management").Exists())
}

// ============================================================================
// computeFinalAnthropicBeta — 关键路径
// ============================================================================

func newTestGatewayServiceForBeta(injectBetaForAPIKey bool) *GatewayService {
	cfg := &config.Config{}
	cfg.Gateway.InjectBetaForAPIKey = injectBetaForAPIKey
	return &GatewayService{cfg: cfg}
}

func TestComputeFinalAnthropicBeta_OAuthMimic_NonHaiku_IncludesContextManagement(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	final, ok := s.computeFinalAnthropicBeta("oauth", true, "claude-sonnet-4-6", http.Header{}, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, anthropicBetaTokensContains(final, claude.BetaContextManagement),
		"OAuth mimic non-haiku 必须注入完整 CC mimicry beta，含 context-management-2025-06-27")
	require.True(t, anthropicBetaTokensContains(final, claude.BetaOAuth))
	require.True(t, anthropicBetaTokensContains(final, claude.BetaClaudeCode))
}

func TestComputeFinalAnthropicBeta_OAuthMimic_Haiku_ExcludesContextManagement(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	final, ok := s.computeFinalAnthropicBeta("oauth", true, "claude-haiku-4-5", http.Header{}, []byte(`{}`), nil)
	require.True(t, ok)
	require.False(t, anthropicBetaTokensContains(final, claude.BetaContextManagement),
		"OAuth mimic haiku 仅注入 oauth + interleaved-thinking，不含 context-management")
	require.True(t, anthropicBetaTokensContains(final, claude.BetaOAuth))
	require.True(t, anthropicBetaTokensContains(final, claude.BetaInterleavedThinking))
}

func TestComputeFinalAnthropicBeta_OAuthMimic_IgnoresClientBeta(t *testing.T) {
	// mimic 路径下原代码白名单透传被跳过，client beta 应被忽略
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "custom-experimental-beta")
	final, ok := s.computeFinalAnthropicBeta("oauth", true, "claude-sonnet-4-6", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.False(t, strings.Contains(final, "custom-experimental-beta"),
		"mimic 路径必须忽略客户端 anthropic-beta header")
}

func TestComputeFinalAnthropicBeta_OAuthTransparent_NonHaiku_PreservesClientContextManagement(t *testing.T) {
	// 真 CC 客户端透传：客户端 header 中的 context-management beta 必须保留
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "claude-code-20250219,oauth-2025-04-20,context-management-2025-06-27")
	final, ok := s.computeFinalAnthropicBeta("oauth", false, "claude-sonnet-4-6", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, anthropicBetaTokensContains(final, claude.BetaContextManagement))
}

func TestComputeFinalAnthropicBeta_OAuthTransparent_Haiku_RealCCPreservesContextManagement(t *testing.T) {
	// haiku 透传 + 客户端带 context-management beta → 必须保留
	// （能力维度核心场景：避免 model-name 误删客户端透传的功能 beta）
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "claude-code-20250219,oauth-2025-04-20,context-management-2025-06-27,interleaved-thinking-2025-05-14")
	final, ok := s.computeFinalAnthropicBeta("oauth", false, "claude-haiku-4-5", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, anthropicBetaTokensContains(final, claude.BetaContextManagement),
		"真 CC + haiku + 客户端带 context-management beta → 透传必须保留")
}

func TestComputeFinalAnthropicBeta_APIKey_PassesClientBetaThroughDropSet(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "oauth-2025-04-20,custom-beta")
	final, ok := s.computeFinalAnthropicBeta("apikey", false, "claude-sonnet-4-6", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, anthropicBetaTokensContains(final, "oauth-2025-04-20"))
	require.True(t, anthropicBetaTokensContains(final, "custom-beta"))
}

func TestComputeFinalAnthropicBeta_APIKey_NoClientBetaInjectOff_ShouldNotSet(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	final, ok := s.computeFinalAnthropicBeta("apikey", false, "claude-sonnet-4-6", http.Header{}, []byte(`{}`), nil)
	require.False(t, ok, "API-key + 客户端未传 + InjectBetaForAPIKey 关 → 不应主动设置 anthropic-beta")
	require.Equal(t, "", final)
}

// ============================================================================
// computeFinalCountTokensAnthropicBeta
// ============================================================================

func TestComputeFinalCountTokensAnthropicBeta_OAuthMimic_AlwaysIncludesContextManagement(t *testing.T) {
	// count_tokens 路径下 mimic 不按 haiku 排除：始终注入完整 mimicry beta
	s := newTestGatewayServiceForBeta(false)
	final, ok := s.computeFinalCountTokensAnthropicBeta("oauth", true, "claude-haiku-4-5", http.Header{}, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, anthropicBetaTokensContains(final, claude.BetaContextManagement),
		"count_tokens + mimic 即使 haiku 也注入 context-management beta（与 messages 不同）")
	require.True(t, anthropicBetaTokensContains(final, claude.BetaTokenCounting),
		"count_tokens 路径必须含 token-counting beta")
}

// 重构等价性回归：
// 原 main buildCountTokensRequest 在 count_tokens mimic 分支上不跳过白名单透传
// （与 messages mimic 不同），incomingBeta 取自客户端透传。重构后必须从 clientHeaders
// 拿同一个值并 merge，否则会丢失客户端 beta。
func TestComputeFinalCountTokensAnthropicBeta_OAuthMimic_FiltersUnknownClientBeta(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "custom-experimental-beta,context-1m-2025-08-07")
	final, ok := s.computeFinalCountTokensAnthropicBeta("oauth", true, "claude-haiku-4-5", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.False(t, anthropicBetaTokensContains(final, "custom-experimental-beta"),
		"count_tokens mimic 不得透传未知 beta")
	require.True(t, anthropicBetaTokensContains(final, "context-1m-2025-08-07"),
		"客户端透传的已知能力 beta 仍需保留")
	require.True(t, anthropicBetaTokensContains(final, claude.BetaContextManagement),
		"同时 FullClaudeCodeMimicryBetas 不打折扣")
	require.True(t, anthropicBetaTokensContains(final, claude.BetaTokenCounting),
		"同时补齐 token-counting beta")
}

// messages mimic 路径反向验证：原代码会跳过白名单透传，
// 客户端 beta 不会进入 mimic 计算。重构后 messages computeFinalAnthropicBeta
// mimic 分支依然不该使用 clientBeta。
func TestComputeFinalAnthropicBeta_OAuthMimic_IgnoresClientBetaExplicit(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "custom-experimental-beta")
	final, ok := s.computeFinalAnthropicBeta("oauth", true, "claude-sonnet-4-6", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.False(t, anthropicBetaTokensContains(final, "custom-experimental-beta"),
		"messages mimic 原代码跳过白名单透传 → 客户端 beta 不进入计算。"+
			"与 count_tokens mimic 是不同的设计，不能合并为同一函数。")
}

func TestComputeFinalCountTokensAnthropicBeta_OAuthTransparent_NoClientBetaInjectsDefault(t *testing.T) {
	// 真 CC 客户端透传 + 客户端未传 anthropic-beta → 用 CountTokensBetaHeader 兜底
	s := newTestGatewayServiceForBeta(false)
	final, ok := s.computeFinalCountTokensAnthropicBeta("oauth", false, "claude-haiku-4-5", http.Header{}, []byte(`{}`), nil)
	require.True(t, ok)
	require.Equal(t, claude.CountTokensBetaHeader, final)
	// CountTokensBetaHeader 不含 context-management beta
	require.False(t, anthropicBetaTokensContains(final, claude.BetaContextManagement))
}

func TestComputeFinalCountTokensAnthropicBeta_OAuthTransparent_AppendsBetaTokenCounting(t *testing.T) {
	s := newTestGatewayServiceForBeta(false)
	hdr := http.Header{}
	hdr.Set("anthropic-beta", "oauth-2025-04-20,context-management-2025-06-27")
	final, ok := s.computeFinalCountTokensAnthropicBeta("oauth", false, "claude-sonnet-4-6", hdr, []byte(`{}`), nil)
	require.True(t, ok)
	require.True(t, anthropicBetaTokensContains(final, claude.BetaTokenCounting),
		"客户端未带 token-counting beta 时必须补齐")
	require.True(t, anthropicBetaTokensContains(final, claude.BetaContextManagement),
		"客户端带的 context-management beta 必须保留")
}

// ============================================================================
// normalizeClaudeOAuthRequestBody — 回归：context_management 补齐恢复原行为
// ============================================================================
//
// 重构后该函数不再按 model 名短路：thinking=enabled/adaptive 时补齐 context_management，
// 与 model 无关。strip 责任移交 sanitizeAnthropicBodyForBetaTokens（在
// buildUpstreamRequest 层按最终 beta header 执行）。

func TestNormalizeClaudeOAuthRequestBody_InjectsContextManagement_ThinkingEnabled(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","thinking":{"type":"enabled","budget_tokens":1000},"messages":[]}`)
	out, _ := normalizeClaudeOAuthRequestBody(body, "claude-sonnet-4-6", claudeOAuthNormalizeOptions{})
	require.True(t, gjson.GetBytes(out, "context_management").Exists())
	require.Equal(t, "clear_thinking_20251015",
		gjson.GetBytes(out, "context_management.edits.0.type").String())
}

func TestNormalizeClaudeOAuthRequestBody_InjectsContextManagement_ThinkingAdaptive(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-7","thinking":{"type":"adaptive"},"messages":[]}`)
	out, _ := normalizeClaudeOAuthRequestBody(body, "claude-opus-4-7", claudeOAuthNormalizeOptions{})
	require.True(t, gjson.GetBytes(out, "context_management").Exists())
}

func TestNormalizeClaudeOAuthRequestBody_HaikuStillInjects_StripDeferredToSanitize(t *testing.T) {
	// haiku + thinking=enabled：normalize 阶段仍按 CLI mimicry 行为补齐字段；
	// strip 由 buildUpstreamRequest 层的 sanitize 兜底（如果 final beta 不含 token）。
	body := []byte(`{"model":"claude-haiku-4-5","thinking":{"type":"enabled","budget_tokens":1000},"messages":[]}`)
	out, _ := normalizeClaudeOAuthRequestBody(body, "claude-haiku-4-5", claudeOAuthNormalizeOptions{})
	require.True(t, gjson.GetBytes(out, "context_management").Exists(),
		"normalize 不再按 model 名短路；strip 责任移交 sanitize 层")
}

func TestNormalizeClaudeOAuthRequestBody_PreservesClientContextManagement(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-7","context_management":{"edits":[{"type":"custom_strategy"}]},"thinking":{"type":"enabled","budget_tokens":1000},"messages":[]}`)
	out, _ := normalizeClaudeOAuthRequestBody(body, "claude-opus-4-7", claudeOAuthNormalizeOptions{})
	require.Equal(t, "custom_strategy",
		gjson.GetBytes(out, "context_management.edits.0.type").String(),
		"客户端透传的 context_management 内容必须原样保留")
}

func TestNormalizeClaudeOAuthRequestBody_NoThinking_NoInject(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[]}`)
	out, _ := normalizeClaudeOAuthRequestBody(body, "claude-sonnet-4-6", claudeOAuthNormalizeOptions{})
	require.False(t, gjson.GetBytes(out, "context_management").Exists())
}

// ============================================================================
// passthrough 集成测试：buildUpstreamRequest-
// AnthropicAPIKeyPassthrough 与 buildCountTokensRequestAnthropicAPIKeyPassthrough
// 路径上 sanitize 是否生效。
// ============================================================================

// passthrough 集成测试不设 base_url，避开 validateUpstreamBaseURL 对 cfg.Security 的依赖。
// targetURL 会走默认 claudeAPIURL，sanitize 逻辑与 baseURL 是否存在无关。
func newAnthropicAPIKeyPassthroughAccountForBetaTest() *Account {
	return &Account{
		ID:       501,
		Name:     "anthropic-apikey-passthrough-ctxmgmt-test",
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "upstream-key",
		},
		Extra:       map[string]any{"anthropic_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func readUpstreamBodyForTest(t *testing.T, req *http.Request) []byte {
	t.Helper()
	require.NotNil(t, req.Body)
	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	return b
}

func TestBuildUpstreamRequestAnthropicAPIKeyPassthrough_StripsContextManagementWhenClientHeaderMissingBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	// 客户端仅带 oauth beta，不带 context-management-2025-06-27
	c.Request.Header.Set("Anthropic-Beta", "oauth-2025-04-20")

	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(
		context.Background(), c, newAnthropicAPIKeyPassthroughAccountForBetaTest(), body, "token",
	)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(readUpstreamBodyForTest(t, req), "context_management").Exists(),
		"API-key passthrough + 客户端未带 context-management beta → strip body 字段")
}

func TestBuildUpstreamRequestAnthropicAPIKeyPassthrough_PreservesContextManagementWhenClientHeaderHasBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Anthropic-Beta", "oauth-2025-04-20,context-management-2025-06-27")

	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(
		context.Background(), c, newAnthropicAPIKeyPassthroughAccountForBetaTest(), body, "token",
	)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(readUpstreamBodyForTest(t, req), "context_management").Exists(),
		"API-key passthrough + 客户端带 context-management beta → 字段保留（不过度删除）")
}

func TestBuildCountTokensRequestAnthropicAPIKeyPassthrough_StripsContextManagementWhenClientHeaderMissingBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	c.Request.Header.Set("Anthropic-Beta", "oauth-2025-04-20,token-counting-2024-11-01")

	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, err := svc.buildCountTokensRequestAnthropicAPIKeyPassthrough(
		context.Background(), c, newAnthropicAPIKeyPassthroughAccountForBetaTest(), body, "token",
	)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(readUpstreamBodyForTest(t, req), "context_management").Exists(),
		"count_tokens passthrough + 客户端未带 context-management beta → strip")
}

// ============================================================================
// 集成测试：buildUpstreamRequest
// 全路径验证上游 outgoing body 与 anthropic-beta header 严格对称。
// 这个测试能挡住未来某人忘调 sanitize / 将 sanitize 挪到 CCH 之后 等 regression。
// ============================================================================

func TestBuildUpstreamRequest_OAuthMimicHaiku_StripsContextManagementEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	account := &Account{ID: 401, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-tok"},
		Status:      StatusActive,
		Schedulable: true,
	}
	// haiku + mimic CC → final beta = HaikuBetaHeader（不含 context-management）→
	// body 必须 strip。
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequest(
		context.Background(), c, account, body,
		"oauth-tok", "oauth", "claude-haiku-4-5", false, true, // mimicClaudeCode=true
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	outBeta := getHeaderRaw(req.Header, "anthropic-beta")

	require.False(t, gjson.GetBytes(outBody, "context_management").Exists(),
		"OAuth mimic + haiku 端到端：outgoing body 不应含 context_management")
	require.False(t, anthropicBetaTokensContains(outBeta, claude.BetaContextManagement),
		"对称约束：outgoing anthropic-beta header 也不带 context-management beta")
}

func TestBuildUpstreamRequest_OAuthMimicNonHaiku_PreservesContextManagementEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	account := &Account{ID: 402, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-tok"},
		Status:      StatusActive,
		Schedulable: true,
	}
	// sonnet + mimic CC → final beta = FullClaudeCodeMimicryBetas（含 context-management）→
	// body 保留。
	body := []byte(`{"model":"claude-sonnet-4-6","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequest(
		context.Background(), c, account, body,
		"oauth-tok", "oauth", "claude-sonnet-4-6", false, true,
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	outBeta := getHeaderRaw(req.Header, "anthropic-beta")

	require.True(t, gjson.GetBytes(outBody, "context_management").Exists(),
		"OAuth mimic + non-haiku：outgoing body 必须保留 context_management。")
	require.True(t, anthropicBetaTokensContains(outBeta, claude.BetaContextManagement),
		"对称约束：outgoing anthropic-beta header 同时含 context-management beta")
}

func TestBuildUpstreamRequest_OAuthTransparentHaikuWithRealCCBeta_PreservesField(t *testing.T) {
	// 端到端验证：真 CC 客户端 + haiku + 客户端 header 带 context-management beta
	// → final beta 透传 → 不应该过度删除 body 字段
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Anthropic-Beta",
		"claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,context-management-2025-06-27")

	account := &Account{ID: 403, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-tok"},
		Status:      StatusActive, Schedulable: true,
	}
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015","keep":"all"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequest(
		context.Background(), c, account, body,
		"oauth-tok", "oauth", "claude-haiku-4-5", false, false, // mimicClaudeCode=false（真 CC）
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	outBeta := getHeaderRaw(req.Header, "anthropic-beta")

	require.True(t, anthropicBetaTokensContains(outBeta, claude.BetaContextManagement),
		"真 CC 透传路径：客户端 header 中的 context-management beta 必须保留")
	require.True(t, gjson.GetBytes(outBody, "context_management").Exists(),
		"回归保护：真 CC + haiku + 客户端带 beta token 时，clear_thinking_20251015 功能不能静默失效")
}

// count_tokens 主路径 E2E 集成测试
func TestBuildCountTokensRequest_OAuthMimicHaiku_PreservesContextManagementEndToEnd(t *testing.T) {
	// count_tokens 路径下 mimic 不按 haiku 排除，始终注入 BetaContextManagement
	// → sanitize 看到最终 beta header 含 context-management beta → 字段保留。
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)

	account := &Account{ID: 411, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-tok"},
		Status:      StatusActive, Schedulable: true,
	}
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildCountTokensRequest(
		context.Background(), c, account, body,
		"oauth-tok", "oauth", "claude-haiku-4-5", true, // mimicClaudeCode=true
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	outBeta := getHeaderRaw(req.Header, "anthropic-beta")

	require.True(t, anthropicBetaTokensContains(outBeta, claude.BetaContextManagement),
		"count_tokens mimic 始终注入 context-management beta")
	require.True(t, gjson.GetBytes(outBody, "context_management").Exists(),
		"对称约束：final beta 含 token 时 body 字段保留")
	require.True(t, anthropicBetaTokensContains(outBeta, claude.BetaTokenCounting),
		"count_tokens 路径必须含 token-counting beta")
}

func TestBuildCountTokensRequest_APIKeyHaiku_StripsContextManagementEndToEnd(t *testing.T) {
	// API-key + haiku + 客户端 header 不带 context-management beta → final beta 不含 → strip
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	c.Request.Header.Set("Anthropic-Beta", "interleaved-thinking-2025-05-14")

	account := &Account{ID: 412, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-ant-xxx"},
		Status:      StatusActive, Schedulable: true,
	}
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildCountTokensRequest(
		context.Background(), c, account, body,
		"sk-ant-xxx", "apikey", "claude-haiku-4-5", false,
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	require.False(t, gjson.GetBytes(outBody, "context_management").Exists(),
		"count_tokens API-key + 客户端未带 beta token → body strip")
}

// count_tokens passthrough preserve 测试
func TestBuildCountTokensRequestAnthropicAPIKeyPassthrough_PreservesContextManagementWhenClientHeaderHasBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	c.Request.Header.Set("Anthropic-Beta", "oauth-2025-04-20,context-management-2025-06-27,token-counting-2024-11-01")

	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[{"type":"clear_thinking_20251015"}]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, err := svc.buildCountTokensRequestAnthropicAPIKeyPassthrough(
		context.Background(), c, newAnthropicAPIKeyPassthroughAccountForBetaTest(), body, "token",
	)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(readUpstreamBodyForTest(t, req), "context_management").Exists(),
		"count_tokens passthrough + 客户端带 context-management beta → 字段保留")
}

func TestBuildUpstreamRequest_APIKeyHaikuWithContextManagement_StripsField(t *testing.T) {
	// API-key + haiku + body 带 context_management + 客户端 header 未带 context-management beta
	// → final beta 不含 → body 字段被 strip
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("Anthropic-Beta", "interleaved-thinking-2025-05-14")

	account := &Account{ID: 404, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-ant-xxx"},
		Status:      StatusActive, Schedulable: true,
	}
	body := []byte(`{"model":"claude-haiku-4-5","context_management":{"edits":[]},"messages":[]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	req, _, err := svc.buildUpstreamRequest(
		context.Background(), c, account, body,
		"sk-ant-xxx", "apikey", "claude-haiku-4-5", false, false,
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	require.False(t, gjson.GetBytes(outBody, "context_management").Exists(),
		"API-key + haiku + 客户端未带 beta token → body 字段必须被 strip")
}

func TestShouldMimicClaudeCodeUpstream_GlobalSetting(t *testing.T) {
	settings := DefaultGatewaySettings()
	settings.AnthropicClaudeCodeMimicryEnabled = true
	settingService := &SettingService{}
	settingService.storeGatewaySettingsCache(settings, time.Hour)
	svc := &GatewayService{settingService: settingService}
	account := &Account{ID: 501, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}

	require.True(t, svc.shouldMimicClaudeCodeUpstream(context.Background(), account, false))
	require.False(t, svc.shouldMimicClaudeCodeUpstream(context.Background(), account, true))
	account.Extra = map[string]any{"anthropic_passthrough": true}
	require.False(t, svc.shouldMimicClaudeCodeUpstream(context.Background(), account, false))
}

func TestBuildUpstreamRequest_APIKeyMimicUsesClaudeCodeHeadersAndAPIKeyBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "non-claude-client/1.0")
	c.Request.Header.Set("Anthropic-Beta", "client-beta-should-not-pass")

	account := &Account{ID: 502, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":                 "sk-ant-xxx",
			"header_override_enabled": true,
			"header_overrides": map[string]any{
				"user-agent":                "overridden-client/1.0",
				"anthropic-version":         "invalid-version",
				"x-stainless-helper-method": "invalid-helper",
				"x-custom-account-header":   "kept",
			},
		},
		Status: StatusActive, Schedulable: true,
	}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	svc := &GatewayService{cfg: &config.Config{}}
	mimicBody, mappedModel := svc.applyClaudeCodeAPIKeyMimicryToBody(context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointMessages)
	req, _, err := svc.buildUpstreamRequest(
		context.Background(), c, account, mimicBody,
		"sk-ant-xxx", "apikey", mappedModel, false, true,
	)
	require.NoError(t, err)

	outBody := readUpstreamBodyForTest(t, req)
	outBeta := getHeaderRaw(req.Header, "anthropic-beta")

	require.Equal(t, "sk-ant-xxx", getHeaderRaw(req.Header, "x-api-key"))
	require.Empty(t, getHeaderRaw(req.Header, "authorization"))
	require.Contains(t, getHeaderRaw(req.Header, "User-Agent"), "claude-cli/")
	require.Equal(t, "cli", getHeaderRaw(req.Header, "X-App"))
	require.Equal(t, "2023-06-01", getHeaderRaw(req.Header, "anthropic-version"))
	require.Empty(t, getHeaderRaw(req.Header, "x-stainless-helper-method"))
	require.Equal(t, "kept", getHeaderRaw(req.Header, "x-custom-account-header"))
	require.NotEmpty(t, getHeaderRaw(req.Header, "X-Claude-Code-Session-Id"))
	require.True(t, anthropicBetaTokensContains(outBeta, claude.BetaClaudeCode))
	require.False(t, anthropicBetaTokensContains(outBeta, claude.BetaOAuth))
	require.NotContains(t, outBeta, "client-beta-should-not-pass")
	require.NotEmpty(t, gjson.GetBytes(outBody, "metadata.user_id").String())
	require.True(t, gjson.GetBytes(outBody, "tools").IsArray())
	require.Equal(t, float64(1), gjson.GetBytes(outBody, "temperature").Float())
}

func TestBuildCountTokensRequest_APIKeyMimicUsesDedicatedBodyAndBeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	c.Request.Header.Set("User-Agent", "non-claude-client/1.0")
	c.Request.Header.Set("Anthropic-Beta", strings.Join([]string{
		claude.BetaOAuth,
		claude.BetaContextManagement,
		claude.BetaExtendedCacheTTL,
	}, ","))

	account := &Account{
		ID:          503,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-ant-xxx"},
		Status:      StatusActive,
		Schedulable: true,
	}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"system":[{"type":"text","text":"original","cache_control":{"type":"ephemeral","ttl":"1h"}}],
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
		"thinking":{"type":"adaptive"},
		"context_management":{"edits":[]},
		"max_tokens":4096,
		"temperature":0.5,
		"stream":true
	}`)
	svc := &GatewayService{cfg: &config.Config{}}
	mimicBody, mappedModel := svc.applyClaudeCodeAPIKeyMimicryToBody(
		context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointCountTokens,
	)
	req, wireBody, err := svc.buildCountTokensRequest(
		context.Background(), c, account, mimicBody,
		"sk-ant-xxx", "apikey", mappedModel, true,
	)
	require.NoError(t, err)

	finalBeta := getHeaderRaw(req.Header, "anthropic-beta")
	require.Contains(t, getHeaderRaw(req.Header, "User-Agent"), "claude-cli/")
	require.Equal(t, "cli", getHeaderRaw(req.Header, "X-App"))
	require.NotEmpty(t, getHeaderRaw(req.Header, "X-Claude-Code-Session-Id"))
	require.True(t, anthropicBetaTokensContains(finalBeta, claude.BetaClaudeCode))
	require.True(t, anthropicBetaTokensContains(finalBeta, claude.BetaTokenCounting))
	require.True(t, anthropicBetaTokensContains(finalBeta, claude.BetaContextManagement))
	require.False(t, anthropicBetaTokensContains(finalBeta, claude.BetaOAuth))
	require.False(t, gjson.GetBytes(wireBody, "metadata").Exists())
	require.False(t, gjson.GetBytes(wireBody, "max_tokens").Exists())
	require.False(t, gjson.GetBytes(wireBody, "temperature").Exists())
	require.False(t, gjson.GetBytes(wireBody, "stream").Exists())
	require.Equal(t, "original", gjson.GetBytes(wireBody, "system.2.text").String())
	require.Equal(t, "1h", gjson.GetBytes(wireBody, "system.2.cache_control.ttl").String())
	require.Equal(t, "lookup", gjson.GetBytes(wireBody, "tools.0.name").String())
	require.True(t, gjson.GetBytes(wireBody, "context_management").Exists())
}

func TestApplyClaudeCodeAPIKeyMimicryToBody_CountTokensEnforcesFinalCacheLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	account := &Account{ID: 504, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"cache_control":{"type":"ephemeral","ttl":"5m"},
		"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"messages":[{"role":"user","content":[
			{"type":"text","text":"one","cache_control":{"type":"ephemeral","ttl":"5m"}},
			{"type":"text","text":"two","cache_control":{"type":"ephemeral","ttl":"5m"}}
		]}],
		"tools":[{"name":"lookup","input_schema":{"type":"object"}}]
	}`)

	out, _ := (&GatewayService{}).applyClaudeCodeAPIKeyMimicryToBody(
		context.Background(), c, account, body, "claude-sonnet-4-6", anthropicMimicEndpointCountTokens,
	)

	require.Equal(t, maxCacheControlBlocks, cacheControlBlockCount(out))
	require.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists(), "代理新增的工具断点应最先移除")
}

func TestComputeFinalCountTokensAnthropicBeta_APIKeyHaikuMimicIncludesStrictFingerprint(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-4-5","messages":[]}`)
	beta, shouldSet := (&GatewayService{}).computeFinalCountTokensAnthropicBeta(
		"apikey", true, "claude-haiku-4-5", nil, body, nil,
	)

	require.True(t, shouldSet)
	require.True(t, anthropicBetaTokensContains(beta, claude.BetaClaudeCode))
	require.True(t, anthropicBetaTokensContains(beta, claude.BetaInterleavedThinking))
	require.True(t, anthropicBetaTokensContains(beta, claude.BetaTokenCounting))
	require.False(t, anthropicBetaTokensContains(beta, claude.BetaOAuth))
}
