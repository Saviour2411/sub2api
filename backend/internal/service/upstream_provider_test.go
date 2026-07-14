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

func writeUpstreamJSON(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data}))
}

func TestSub2APIUpstreamProviderPasswordAndUsage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		writeUpstreamJSON(t, w, map[string]any{"access_token": "access-1", "refresh_token": "refresh-1"})
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access-1", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"balance": 12.5})
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, []any{map[string]any{"id": "g1", "name": "默认组", "platform": "openai"}})
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"rates": map[string]any{"g1": 1.5}})
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.URL.Query().Get("start_date"))
		writeUpstreamJSON(t, w, map[string]any{
			"total_tokens": 1234,
			"total_cost":   0.42,
			"groups":       []any{map[string]any{"group_id": "g1", "tokens": 1234, "cost": 0.42}},
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
	require.Equal(t, int64(1234), result.Groups[0].TodayTokens)
	require.Equal(t, "refresh-1", result.Credential.RefreshToken)
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
		writeUpstreamJSON(t, w, map[string]any{"vip": map[string]any{"ratio": 1.2}})
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
	require.Equal(t, int64(1007), result.Groups[0].TodayTokens)
	require.Equal(t, "session=cookie-1", result.Credential.Cookie)
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
	require.Nil(t, result)
	require.Contains(t, err.Error(), fmt.Sprintf("第 %d 页", 2))
}
