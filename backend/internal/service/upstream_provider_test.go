package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newTestUpstreamHTTPClient(t *testing.T) *upstreamHTTPClient {
	t.Helper()
	client, err := newUpstreamHTTPClient(&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
		AllowInsecureHTTP: true,
		AllowPrivateHosts: true,
	}}})
	require.NoError(t, err)
	return client
}

func TestUpstreamHTTPClientNormalizeBaseURLWhenAllowlistDisabled(t *testing.T) {
	tests := []struct {
		name          string
		upstreamHosts []string
		raw           string
		want          string
	}{
		{
			name: "空白名单允许任意公网 HTTPS 地址",
			raw:  "https://upstream.example.com/api///",
			want: "https://upstream.example.com/api",
		},
		{
			name:          "非空默认白名单不限制公网 HTTPS 地址",
			upstreamHosts: []string{"api.openai.com", "api.anthropic.com"},
			raw:           "https://www.xiaobaishu.org///",
			want:          "https://www.xiaobaishu.org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newUpstreamHTTPClient(&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
				Enabled:       false,
				UpstreamHosts: tt.upstreamHosts,
			}}})
			require.NoError(t, err)

			normalized, err := client.normalizeBaseURL(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, normalized)
		})
	}
}

func TestUpstreamHTTPClientNormalizeBaseURLWhenAllowlistEnabled(t *testing.T) {
	client, err := newUpstreamHTTPClient(&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
		Enabled:           true,
		UpstreamHosts:     []string{"allowed.example.com"},
		AllowPrivateHosts: true,
		AllowInsecureHTTP: true,
	}}})
	require.NoError(t, err)

	t.Run("拒绝未列入白名单的地址", func(t *testing.T) {
		_, err := client.normalizeBaseURL("https://denied.example.com")
		require.Error(t, err)
	})

	t.Run("允许列入白名单的地址并移除尾斜杠", func(t *testing.T) {
		normalized, err := client.normalizeBaseURL("https://allowed.example.com/api///")
		require.NoError(t, err)
		require.Equal(t, "https://allowed.example.com/api", normalized)
	})

	t.Run("启用白名单时始终拒绝 HTTP 地址", func(t *testing.T) {
		_, err := client.normalizeBaseURL("http://allowed.example.com")
		require.Error(t, err)
	})
}

func TestUpstreamHTTPClientAllowsLocalRequestWhenAllowlistDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"status": "ok"})
	}))
	defer server.Close()

	client, err := newUpstreamHTTPClient(&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
		Enabled:           false,
		UpstreamHosts:     []string{"api.openai.com"},
		AllowPrivateHosts: false,
		AllowInsecureHTTP: true,
	}}})
	require.NoError(t, err)

	normalized, err := client.normalizeBaseURL(server.URL + "/")
	require.NoError(t, err)
	require.Equal(t, server.URL, normalized)

	payload, _, err := client.doJSON(context.Background(), http.MethodGet, normalized, "/health", nil, "", nil)
	require.NoError(t, err)
	require.Equal(t, "ok", stringValue(valueByKeys(payload, "status")))
}

func writeUpstreamJSON(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data}))
}

func TestSub2APIUpstreamProviderPasswordAndUsage(t *testing.T) {
	mux := http.NewServeMux()
	statsGroupIDs := make([]string, 0, 2)
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		writeUpstreamJSON(t, w, map[string]any{"access_token": "access-1", "refresh_token": "refresh-1"})
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access-1", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"balance": 12.5})
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, []any{map[string]any{
			"id": "g1", "name": "默认组", "platform": "openai",
			"description": "默认分组", "rate_multiplier": 1.2,
		}})
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"rates": map[string]any{"g1": 1.5}})
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.URL.Query().Get("start_date"))
		statsGroupIDs = append(statsGroupIDs, r.URL.Query().Get("group_id"))
		writeUpstreamJSON(t, w, map[string]any{
			"total_tokens":      1234,
			"total_actual_cost": 0.42,
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site:       &UpstreamSite{BaseURL: server.URL, Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthPassword, Account: "admin@example.com"},
		Credential: UpstreamCredential{Password: "secret"},
		Dates:      []time.Time{time.Now().In(loc)},
		Location:   loc,
	})
	require.NoError(t, err)
	require.Equal(t, 12.5, *result.BalanceUSD)
	require.Equal(t, int64(1234), result.Daily[0].Tokens)
	require.InDelta(t, 0.42, result.Daily[0].CostUSD, 1e-9)
	require.Len(t, result.Groups, 1)
	require.InDelta(t, 1.5, *result.Groups[0].Multiplier, 1e-9)
	require.Equal(t, "默认分组", result.Groups[0].Description)
	require.Equal(t, int64(1234), result.Groups[0].TodayTokens)
	require.Equal(t, "refresh-1", result.Credential.RefreshToken)
	require.Equal(t, []string{"", "g1"}, statsGroupIDs, "旧接口回退必须按 group_id 单独查询每个分组")
}

func TestSub2APIUpstreamProviderRefreshToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"access_token": "refreshed"})
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer refreshed", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"id": 1})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	credential, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthToken,
	}, UpstreamCredential{RefreshToken: "refresh"})
	require.NoError(t, err)
	require.Equal(t, "refreshed", credential.AccessToken)
}

func TestSub2APIUpstreamProviderPasswordModeReusesCachedAccessToken(t *testing.T) {
	var loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.Error(w, "不应登录", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer cached-access", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"id": 1})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	credential, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformSub2API,
		AuthMode: UpstreamAuthPassword, Account: "admin@example.com",
	}, UpstreamCredential{Password: "secret", AccessToken: "cached-access", RefreshToken: "cached-refresh"})
	require.NoError(t, err)
	require.Zero(t, loginCalls)
	require.Equal(t, "cached-access", credential.AccessToken)
}

func TestSub2APIUpstreamProviderPasswordModeRefreshesBeforeLogin(t *testing.T) {
	var loginCalls, refreshCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.Error(w, "不应登录", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, _ *http.Request) {
		refreshCalls++
		writeUpstreamJSON(t, w, map[string]any{"access_token": "refreshed-access", "refresh_token": "refreshed-refresh"})
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer cached-access" {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer refreshed-access", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"id": 1})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	credential, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformSub2API,
		AuthMode: UpstreamAuthPassword, Account: "admin@example.com",
	}, UpstreamCredential{Password: "secret", AccessToken: "cached-access", RefreshToken: "cached-refresh"})
	require.NoError(t, err)
	require.Zero(t, loginCalls)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, "refreshed-access", credential.AccessToken)
	require.Equal(t, "refreshed-refresh", credential.RefreshToken)
}

func TestSub2APIUpstreamProviderPasswordModeFallsBackAfterRefreshRejected(t *testing.T) {
	var loginCalls, refreshCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		writeUpstreamJSON(t, w, map[string]any{"access_token": "login-access", "refresh_token": "login-refresh"})
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, _ *http.Request) {
		refreshCalls++
		http.Error(w, "refresh rejected", http.StatusUnauthorized)
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer cached-access" {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer login-access", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"id": 1})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	credential, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformSub2API,
		AuthMode: UpstreamAuthPassword, Account: "admin@example.com",
	}, UpstreamCredential{Password: "secret", AccessToken: "cached-access", RefreshToken: "cached-refresh"})
	require.NoError(t, err)
	require.Equal(t, 1, refreshCalls)
	require.Equal(t, 1, loginCalls)
	require.Equal(t, "login-access", credential.AccessToken)
	require.Equal(t, "login-refresh", credential.RefreshToken)
}

func TestSub2APIUpstreamProviderDoesNotLoginOnNonAuthFailure(t *testing.T) {
	var loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.Error(w, "不应登录", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site: &UpstreamSite{
			BaseURL: server.URL, Platform: UpstreamPlatformSub2API,
			AuthMode: UpstreamAuthPassword, Account: "admin@example.com",
		},
		Credential: UpstreamCredential{Password: "secret", AccessToken: "cached-access"},
	})
	require.Error(t, err)
	require.Zero(t, loginCalls)
	require.NotNil(t, result)
	require.Equal(t, "cached-access", result.Credential.AccessToken)
}

func TestSub2APIUpstreamProviderMapsTurnstileLoginFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"turnstile verification failed","reason":"TURNSTILE_VERIFICATION_FAILED"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	_, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthPassword, Account: "admin@example.com",
	}, UpstreamCredential{Password: "secret"})
	require.ErrorIs(t, err, ErrUpstreamTurnstileRequired)
	require.ErrorIs(t, upstreamValidationError(err), ErrUpstreamTurnstileRequired)
}

func TestNewAPIUpstreamProviderPaginationTokenFallbackAndUSD(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	today := time.Now().In(loc)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie-1"})
		writeUpstreamJSON(t, w, map[string]any{"id": 9})
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Cookie"), "session=cookie-1")
		require.Equal(t, "9", r.Header.Get("New-Api-User"))
		writeUpstreamJSON(t, w, map[string]any{"quota": 1_000_000})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 500_000})
	})
	mux.HandleFunc("/api/user/self/groups", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"vip": map[string]any{"ratio": 1.2, "desc": "高级分组"}})
	})
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"group_ratio": map[string]any{"vip": 2}})
	})
	mux.HandleFunc("/api/log/self", func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("p"))
		items := make([]any, 0)
		switch page {
		case 1:
			for index := 0; index < newAPILogPageSize; index++ {
				items = append(items, map[string]any{"total_tokens": 10, "quota": 500, "group": "vip"})
			}
		case 2:
			items = append(items, map[string]any{"prompt_tokens": 3, "completion_tokens": 4, "quota": 1000, "group": "vip"})
		}
		writeUpstreamJSON(t, w, map[string]any{"items": items, "total": 101})
	})
	mux.HandleFunc("/api/log/self/stat", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota": 51_000})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site:       &UpstreamSite{BaseURL: server.URL, Platform: UpstreamPlatformNewAPI, AuthMode: UpstreamAuthPassword, Account: "admin"},
		Credential: UpstreamCredential{Password: "secret"}, Dates: []time.Time{today}, Location: loc,
	})
	require.NoError(t, err)
	require.Equal(t, 2.0, *result.BalanceUSD)
	require.Equal(t, int64(1007), result.Daily[0].Tokens)
	require.InDelta(t, 0.102, result.Daily[0].CostUSD, 1e-9)
	require.Len(t, result.Groups, 1)
	require.InDelta(t, 2, *result.Groups[0].Multiplier, 1e-9)
	require.Equal(t, "高级分组", result.Groups[0].Description)
	require.Equal(t, "New API", result.Groups[0].Platform)
	require.Equal(t, int64(1007), result.Groups[0].TodayTokens)
	require.Equal(t, "session=cookie-1", result.Credential.Cookie)
	require.Equal(t, "9", result.Credential.NewAPIUserID)
}

func TestNewAPIUpstreamProviderReusesCachedCookie(t *testing.T) {
	var loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.Error(w, "不应登录", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "session=cached", r.Header.Get("Cookie"))
		require.Empty(t, r.Header.Get("New-Api-User"), "旧凭证允许从 self 响应补全用户 ID")
		writeUpstreamJSON(t, w, map[string]any{"id": 9, "quota": 100})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 100})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
	credential, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformNewAPI,
		AuthMode: UpstreamAuthPassword, Account: "admin",
	}, UpstreamCredential{Password: "secret", Cookie: "session=cached"})
	require.NoError(t, err)
	require.Zero(t, loginCalls)
	require.Equal(t, "session=cached", credential.Cookie)
	require.Equal(t, "9", credential.NewAPIUserID)
}

func TestNewAPIUpstreamProviderReloginsAfterCookieRejected(t *testing.T) {
	var loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "renewed"})
		writeUpstreamJSON(t, w, map[string]any{"id": 9})
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") == "session=expired" {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		require.Equal(t, "session=renewed", r.Header.Get("Cookie"))
		writeUpstreamJSON(t, w, map[string]any{"id": 9, "quota": 100})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 100})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
	credential, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformNewAPI,
		AuthMode: UpstreamAuthPassword, Account: "admin",
	}, UpstreamCredential{Password: "secret", Cookie: "session=expired", NewAPIUserID: "9"})
	require.NoError(t, err)
	require.Equal(t, 1, loginCalls)
	require.Equal(t, "session=renewed", credential.Cookie)
}

func TestNewAPIUpstreamProviderDoesNotLoginOnNonAuthFailure(t *testing.T) {
	var loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.Error(w, "不应登录", http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
	_, err := provider.Validate(context.Background(), &UpstreamSite{
		BaseURL: server.URL, Platform: UpstreamPlatformNewAPI,
		AuthMode: UpstreamAuthPassword, Account: "admin",
	}, UpstreamCredential{Password: "secret", Cookie: "session=cached", NewAPIUserID: "9"})
	require.Error(t, err)
	require.Zero(t, loginCalls)
}

func TestNewAPIUpstreamProviderPreservesLoginCookieWhenPostLoginLoadFails(t *testing.T) {
	var loginCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls++
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "new-cookie"})
		writeUpstreamJSON(t, w, map[string]any{"id": 9})
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site: &UpstreamSite{
			BaseURL: server.URL, Platform: UpstreamPlatformNewAPI,
			AuthMode: UpstreamAuthPassword, Account: "admin",
		},
		Credential: UpstreamCredential{Password: "secret"},
	})
	require.Error(t, err)
	require.Equal(t, 1, loginCalls)
	require.NotNil(t, result)
	require.NotNil(t, result.Credential)
	require.Equal(t, "session=new-cookie", result.Credential.Cookie)
	require.Equal(t, "9", result.Credential.NewAPIUserID)
}

func TestNewAPIUpstreamProviderDoesNotReturnPartialPagination(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie"})
		writeUpstreamJSON(t, w, map[string]any{"id": 1})
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, _ *http.Request) { writeUpstreamJSON(t, w, map[string]any{"quota": 1}) })
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 1})
	})
	mux.HandleFunc("/api/user/self/groups", func(w http.ResponseWriter, _ *http.Request) { writeUpstreamJSON(t, w, map[string]any{}) })
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) { writeUpstreamJSON(t, w, map[string]any{}) })
	mux.HandleFunc("/api/log/self", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("p") == "2" {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		items := make([]any, newAPILogPageSize)
		for index := range items {
			items[index] = map[string]any{"total_tokens": 1, "quota": 1}
		}
		writeUpstreamJSON(t, w, map[string]any{"items": items, "total": 101})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site:       &UpstreamSite{BaseURL: server.URL, Platform: UpstreamPlatformNewAPI, AuthMode: UpstreamAuthPassword, Account: "admin"},
		Credential: UpstreamCredential{Password: "secret"}, Dates: []time.Time{time.Now()}, Location: time.Local,
	})
	require.Error(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Credential)
	require.Empty(t, result.Groups)
	require.Empty(t, result.Daily)
	require.Contains(t, err.Error(), fmt.Sprintf("第 %d 页", 2))
}

func TestParseNewAPIGroupsResolvesPlatform(t *testing.T) {
	groups := parseNewAPIGroups(map[string]any{
		"claude-aws":  map[string]any{"ratio": 0.3, "desc": "AWS 渠道 99% 高缓存"},
		"cheap-gpt":   map[string]any{"ratio": 0.02, "desc": "稳定低价 GPT 分组"},
		"gemini":      map[string]any{"ratio": 0.1, "provider": "google"},
		"explicit":    map[string]any{"ratio": 1, "provider_type": "anthropic"},
		"unknown":     map[string]any{"ratio": 1, "platform": "New API"},
		"kiro-night":  map[string]any{"ratio": 0.04, "platform": "New API"},
		"antigravity": map[string]any{"ratio": 0.2, "platform": "google antigravity"},
	})

	platforms := make(map[string]string, len(groups))
	for _, group := range groups {
		platforms[group.RemoteID] = group.Platform
	}
	require.Equal(t, "Anthropic", platforms["claude-aws"])
	require.Equal(t, "OpenAI", platforms["cheap-gpt"])
	require.Equal(t, "Gemini", platforms["gemini"])
	require.Equal(t, "Anthropic", platforms["explicit"])
	require.Equal(t, "New API", platforms["unknown"])
	require.Equal(t, "Anthropic", platforms["kiro-night"])
	require.Equal(t, "Antigravity", platforms["antigravity"])
}
