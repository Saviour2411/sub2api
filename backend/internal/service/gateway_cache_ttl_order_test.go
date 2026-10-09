package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// requireCacheTTLOrderAccepted fails when a ttl=1h breakpoint follows a 5m one
// in Anthropic's processing order (tools, system, messages). Anthropic rejects
// such requests with HTTP 400.
func requireCacheTTLOrderAccepted(t *testing.T, body []byte) {
	t.Helper()
	_, messagePaths, toolPaths, systemPaths := collectCacheControlPaths(body)
	var paths []string
	paths = append(paths, toolPaths...)
	paths = append(paths, systemPaths...)
	paths = append(paths, messagePaths...)
	shortTTLPath := ""
	for _, path := range paths {
		if gjson.GetBytes(body, path+".ttl").String() == cacheTTLTarget1h {
			require.Empty(t, shortTTLPath, "1h breakpoint %s follows 5m breakpoint %s in %s", path, shortTTLPath, body)
			continue
		}
		if shortTTLPath == "" {
			shortTTLPath = path
		}
	}
}

func TestEnforceCacheControlLimit_PreservesClientMixedTTL(t *testing.T) {
	body := []byte(`{"tools":[{"name":"t","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral"}}],` +
		`"system":[{"type":"text","text":"s","cache_control":{"type":"ephemeral","ttl":"5m"}}],` +
		`"messages":[{"role":"user","content":[` +
		`{"type":"text","text":"a","cache_control":{"type":"ephemeral","ttl":"1h"}},` +
		`{"type":"text","text":"b","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]}`)

	out := enforceCacheControlLimit(body)

	require.False(t, gjson.GetBytes(out, "tools.0.cache_control.ttl").Exists(), "客户端省略 TTL 的断点保持原样")
	require.Equal(t, "5m", gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.1.cache_control.ttl").String(),
		"a 5m breakpoint after the last 1h one is valid and must keep its ttl")
	require.JSONEq(t, string(body), string(out), "客户端自身的无效顺序不能通过覆盖显式 TTL 修复")
}

func TestEnforceCacheControlLimit_PreservesExplicitTTLBeforeTopLevel1h(t *testing.T) {
	body := []byte(`{"cache_control":{"type":"ephemeral","ttl":"1h"},` +
		`"system":[{"type":"text","text":"s","cache_control":{"type":"ephemeral","ttl":"5m"}}],` +
		`"messages":[{"role":"user","content":"hi"}]}`)

	out := enforceCacheControlLimit(body)

	require.Equal(t, "5m", gjson.GetBytes(out, "system.0.cache_control.ttl").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "cache_control.ttl").String())
}

func TestEnforceCacheControlLimit_KeepsValidRequestsUnchanged(t *testing.T) {
	cases := map[string]string{
		"no breakpoints": `{"system":"s","messages":[{"role":"user","content":"hi"}]}`,
		"all 5m": `{"tools":[{"name":"t","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral"}}],` +
			`"system":[{"type":"text","text":"s","cache_control":{"type":"ephemeral","ttl":"5m"}}],` +
			`"messages":[{"role":"user","content":[{"type":"text","text":"a","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]}`,
		"1h before 5m": `{"system":[{"type":"text","text":"s","cache_control":{"type":"ephemeral","ttl":"1h"}}],` +
			`"messages":[{"role":"user","content":[{"type":"text","text":"a","cache_control":{"type":"ephemeral"}}]}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, body, string(enforceCacheControlLimit([]byte(body))))
		})
	}
}

// 非 Claude Code 客户端声明 1h 时，拟态链路的新建自动断点选择 1h，
// 避免在客户端断点前插入 5m；不能靠出口覆盖客户端 TTL 修复顺序。
func TestEnforceCacheControlLimit_MimicPathKeepsClient1hBreakpointValid(t *testing.T) {
	cases := map[string]string{
		"message 1h": `{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":[` +
			`{"type":"text","text":"Reply OK.","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`,
		"system 1h moved into messages": `{"model":"claude-sonnet-4-6",` +
			`"system":[{"type":"text","text":"Be terse.","cache_control":{"type":"ephemeral","ttl":"1h"}}],` +
			`"messages":[{"role":"user","content":"Reply OK."}]}`,
		"tools and message 1h": `{"model":"claude-sonnet-4-6","tools":[{"name":"probe","input_schema":{"type":"object"}}],` +
			`"messages":[{"role":"user","content":[` +
			`{"type":"text","text":"Reply OK.","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			body := []byte(raw)
			var system any
			if sys := gjson.GetBytes(body, "system"); sys.Exists() {
				require.NoError(t, json.Unmarshal([]byte(sys.Raw), &system))
			}
			body = rewriteSystemForNonClaudeCode(body, system)
			body = applyToolsLastCacheBreakpoint(body)
			requireCacheTTLOrderAccepted(t, body)

			out := enforceCacheControlLimit(body)

			requireCacheTTLOrderAccepted(t, out)
		})
	}
}

func TestForwardCountTokens_MimicPathKeepsClient1hBreakpointValid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	c.Request.Header.Set("User-Agent", "third-party-client/1.0")

	// 最后一个工具的新建自动断点应匹配后续客户端的 1h。
	body := []byte(`{"model":"claude-sonnet-4-6",` +
		`"tools":[{"name":"probe","description":"d","input_schema":{"type":"object"}}],` +
		`"messages":[{"role":"user","content":[{"type":"text","text":"Reply OK.","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-6"}

	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"input_tokens":42}`)),
		},
	}
	svc := &GatewayService{
		cfg:              &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		httpUpstream:     upstream,
		rateLimitService: &RateLimitService{},
	}
	account := &Account{
		ID:          402,
		Name:        "count-tokens-ttl-order-test",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeSetupToken,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-oauth-token"},
		Status:      StatusActive,
		Schedulable: true,
	}

	require.NoError(t, svc.ForwardCountTokens(context.Background(), c, account, parsed))

	require.True(t, gjson.GetBytes(upstream.lastBody, "tools.0.cache_control").Exists(),
		"the mimic tools breakpoint should still be present")
	requireCacheTTLOrderAccepted(t, upstream.lastBody)
}

func Test自动缓存断点按后续客户端TTL选择(t *testing.T) {
	for _, tc := range []struct {
		name, fields, want string
	}{
		{"默认五分钟", `"messages":[{"role":"user","content":"hi"}]`, "5m"},
		{"显式五分钟", `"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"5m"}}]}]`, "5m"},
		{"消息一小时", `"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral","ttl":"1h"}}]}]`, "1h"},
		{"系统一小时", `"system":[{"type":"text","text":"s","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":"hi"}]`, "1h"},
		{"顶层一小时", `"cache_control":{"type":"ephemeral","ttl":"1h"},"messages":[{"role":"user","content":"hi"}]`, "1h"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"tools":[{"name":"probe","input_schema":{"type":"object"}}],` + tc.fields + `}`)
			require.Equal(t, tc.want, automaticAnthropicCacheTTL(body))
			toolsBody := applyToolsLastCacheBreakpoint(body)
			require.Equal(t, tc.want, gjson.GetBytes(toolsBody, "tools.0.cache_control.ttl").String())
			var system any
			if raw := gjson.GetBytes(body, "system"); raw.Exists() {
				require.NoError(t, json.Unmarshal([]byte(raw.Raw), &system))
			}
			injected := injectClaudeCodePrompt(body, system)
			require.Equal(t, tc.want, gjson.GetBytes(injected, "system.0.cache_control.ttl").String())
			for _, blocksConfig := range []string{"", `[{"type":"text","text":"测试自动断点","cache_control":true}]`} {
				blocks, err := buildClaudeOAuthSystemPromptBlocksJSON(body, "测试扩展提示", blocksConfig)
				require.NoError(t, err)
				count := 0
				for _, block := range blocks {
					if cc := gjson.GetBytes(block, "cache_control"); cc.Exists() {
						require.Equal(t, tc.want, cc.Get("ttl").String())
						count++
					}
				}
				require.Positive(t, count)
			}
		})
	}
}

func Test自动缓存适配不覆盖已有工具或系统配置TTL(t *testing.T) {
	for _, cc := range []string{`{"type":"ephemeral","ttl":"5m"}`, `{"type":"ephemeral","ttl":"1h"}`, `{"type":"ephemeral"}`} {
		body := []byte(`{"tools":[{"name":"probe","cache_control":` + cc + `}],"cache_control":{"type":"ephemeral","ttl":"1h"}}`)
		out := applyToolsLastCacheBreakpoint(body)
		want := gjson.Get(cc, "ttl").String()
		if want == "" {
			want = "5m"
		}
		require.Equal(t, want, gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
		blocks, err := buildClaudeOAuthSystemPromptBlocksJSON(body, "", `[{"type":"text","text":"显式配置","cache_control":`+cc+`}]`)
		require.NoError(t, err)
		require.Len(t, blocks, 1)
		require.JSONEq(t, cc, gjson.GetBytes(blocks[0], "cache_control").Raw)
	}
}
