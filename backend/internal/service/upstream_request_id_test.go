package service

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamRequestIDFromHeaders_UnconfiguredAccountRecordsNothing(t *testing.T) {
	h := http.Header{}
	h.Set("X-Client-Request-ID", "sub2api-client")
	h.Set("X-Request-ID", "sub2api-local")
	h.Set("X-Oneapi-Request-Id", "oneapi-1")
	h.Set("Request-Id", "req_official")
	h.Set("xai-request-id", "xai-1")
	h.Set("x-goog-request-id", "goog-1")

	require.Equal(t, "", UpstreamRequestIDFromHeaders(nil, h))
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok} {
		require.Equal(t, "", UpstreamRequestIDFromHeaders(&Account{Platform: platform}, h), platform)
	}
	blank := &Account{Platform: PlatformOpenAI, Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "   "}}
	require.Equal(t, "", UpstreamRequestIDFromHeaders(blank, h))
}

func TestUpstreamRequestIDFromHeaders_ReadsOnlyConfiguredHeader(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra:    map[string]any{AccountExtraUpstreamRequestIDHeader: " x-oneapi-request-id "},
	}
	h := http.Header{}
	h.Set("X-Request-ID", "passthrough-from-real-upstream")
	require.Equal(t, "", UpstreamRequestIDFromHeaders(account, h))

	h.Set("X-Oneapi-Request-Id", " oneapi-2 ")
	require.Equal(t, "oneapi-2", UpstreamRequestIDFromHeaders(account, h))
	require.Equal(t, "", UpstreamRequestIDFromHeaders(account, nil))

	official := &Account{Platform: PlatformAnthropic, Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "request-id"}}
	only := http.Header{}
	only.Set("Request-Id", "req_official")
	require.Equal(t, "req_official", UpstreamRequestIDFromHeaders(official, only))
}

func TestUsageUpstreamRequestIDPtr(t *testing.T) {
	account := &Account{Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: "X-Request-ID"}}
	h := http.Header{}
	h.Set("X-Request-ID", strings.Repeat("a", 200))
	require.Nil(t, usageUpstreamRequestIDPtr(account, h, true))
	require.Nil(t, usageUpstreamRequestIDPtr(account, http.Header{}, false))
	require.Nil(t, usageUpstreamRequestIDPtr(nil, h, false))
	require.Nil(t, usageUpstreamRequestIDPtr(&Account{}, h, false))

	got := usageUpstreamRequestIDPtr(account, h, false)
	require.NotNil(t, got)
	require.Len(t, *got, maxUsageUpstreamRequestIDLen)
}

func TestValidateUpstreamRequestIDHeaderExtra(t *testing.T) {
	require.NoError(t, ValidateUpstreamRequestIDHeaderExtra(nil))
	require.NoError(t, ValidateUpstreamRequestIDHeaderExtra(map[string]any{}))

	blank := map[string]any{AccountExtraUpstreamRequestIDHeader: "   "}
	require.NoError(t, ValidateUpstreamRequestIDHeaderExtra(blank))
	_, present := blank[AccountExtraUpstreamRequestIDHeader]
	require.False(t, present, "blank header name must be removed")

	valid := map[string]any{AccountExtraUpstreamRequestIDHeader: " X-Oneapi-Request-Id "}
	require.NoError(t, ValidateUpstreamRequestIDHeaderExtra(valid))
	require.Equal(t, "X-Oneapi-Request-Id", valid[AccountExtraUpstreamRequestIDHeader])

	require.Error(t, ValidateUpstreamRequestIDHeaderExtra(map[string]any{AccountExtraUpstreamRequestIDHeader: 1}))
	require.Error(t, ValidateUpstreamRequestIDHeaderExtra(map[string]any{AccountExtraUpstreamRequestIDHeader: "X Request Id"}))
	require.Error(t, ValidateUpstreamRequestIDHeaderExtra(map[string]any{AccountExtraUpstreamRequestIDHeader: "X-Request-Id:"}))
	require.Error(t, ValidateUpstreamRequestIDHeaderExtra(map[string]any{AccountExtraUpstreamRequestIDHeader: strings.Repeat("x", 65)}))
}

func TestUpstreamRequestIDHeader_SensitiveHeadersAreRejectedAndNeverRecorded(t *testing.T) {
	for _, name := range []string{
		"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie",
		"X-API-Key", "Api-Key", "X-Auth-Token", "X-Access-Token", "  sEt-CoOkIe  ",
	} {
		t.Run(name, func(t *testing.T) {
			extra := map[string]any{AccountExtraUpstreamRequestIDHeader: name}
			require.Error(t, ValidateUpstreamRequestIDHeaderExtra(extra), "敏感头不能作为请求标识配置")

			// 旧配置同样必须在读取处拦截，不能只依赖管理员保存时的校验。
			account := &Account{Extra: map[string]any{AccountExtraUpstreamRequestIDHeader: name}}
			headers := http.Header{}
			headers.Set(strings.TrimSpace(name), "private-fixture-value")
			require.Empty(t, UpstreamRequestIDFromHeaders(account, headers))
			require.Nil(t, usageUpstreamRequestIDPtr(account, headers, false))
		})
	}
}

func TestUpstreamRequestIDHeader_CustomIdentifiersRemainAvailable(t *testing.T) {
	for _, name := range []string{"X-Request-ID", "X-Oneapi-Request-Id", "X-Correlation-ID", "Traceparent"} {
		t.Run(name, func(t *testing.T) {
			extra := map[string]any{AccountExtraUpstreamRequestIDHeader: " " + name + " "}
			require.NoError(t, ValidateUpstreamRequestIDHeaderExtra(extra))
			require.Equal(t, name, extra[AccountExtraUpstreamRequestIDHeader])
			account := &Account{Extra: extra}
			headers := http.Header{}
			headers.Set(name, "trace-fixture-123")
			require.Equal(t, "trace-fixture-123", UpstreamRequestIDFromHeaders(account, headers))
			require.NotNil(t, usageUpstreamRequestIDPtr(account, headers, false))
			require.Nil(t, usageUpstreamRequestIDPtr(account, headers, true), "WS 轮次仍不记录 HTTP 请求标识")
		})
	}
}
