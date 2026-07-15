package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSub2APIUpstreamProviderSnapshotUsesActualCostAndEffectiveRate(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	today := time.Now().In(loc)
	yesterday := today.AddDate(0, 0, -1)
	var snapshotCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"access_token": "access", "refresh_token": "refresh"})
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer access", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, map[string]any{"balance": 28.5})
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, []any{map[string]any{
			"id": 11, "name": "高优先级", "platform": "openai",
			"description": "优先线路", "rate_multiplier": 1.2,
		}})
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"11": 1.75})
	})
	mux.HandleFunc("/api/v1/usage/dashboard/snapshot-v2", func(w http.ResponseWriter, r *http.Request) {
		snapshotCalls.Add(1)
		require.Equal(t, "Asia/Shanghai", r.URL.Query().Get("timezone"))
		require.Equal(t, "false", r.URL.Query().Get("include_model_stats"))
		if r.URL.Query().Get("include_group_stats") == "true" {
			require.Equal(t, "false", r.URL.Query().Get("include_trend"))
			writeUpstreamJSON(t, w, map[string]any{"groups": []any{map[string]any{
				"group_id": 11, "group_name": "高优先级", "total_tokens": 3456,
				"cost": 88.8, "actual_cost": 2.25,
			}}})
			return
		}
		require.Equal(t, "true", r.URL.Query().Get("include_trend"))
		writeUpstreamJSON(t, w, map[string]any{"trend": []any{
			map[string]any{"date": yesterday.Format("2006-01-02"), "total_tokens": 1234, "cost": 40.0, "actual_cost": 1.5},
			map[string]any{"date": today.Format("2006-01-02"), "total_tokens": 3456, "cost": 88.8, "actual_cost": 2.25},
		}})
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("支持 snapshot-v2 时不应回退到 usage/stats")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site: &UpstreamSite{
			BaseURL: server.URL, Platform: UpstreamPlatformSub2API,
			AuthMode: UpstreamAuthPassword, Account: "admin@example.com",
		},
		Credential: UpstreamCredential{Password: "secret"},
		Dates:      []time.Time{yesterday, today},
		Location:   loc,
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), snapshotCalls.Load())
	require.Len(t, result.Daily, 2)
	require.Equal(t, int64(1234), result.Daily[0].Tokens)
	require.InDelta(t, 1.5, result.Daily[0].CostUSD, 1e-9)
	require.Equal(t, int64(3456), result.Daily[1].Tokens)
	require.InDelta(t, 2.25, result.Daily[1].CostUSD, 1e-9)
	require.Len(t, result.Groups, 1)
	require.Equal(t, "优先线路", result.Groups[0].Description)
	require.Equal(t, "openai", result.Groups[0].Platform)
	require.InDelta(t, 1.75, *result.Groups[0].Multiplier, 1e-9)
	require.Equal(t, int64(3456), result.Groups[0].TodayTokens)
	require.InDelta(t, 2.25, result.Groups[0].TodayCostUSD, 1e-9)
}

func TestSub2APIUsageRejectsStandardCostAsActualCost(t *testing.T) {
	_, err := parseSub2APIUsage(map[string]any{
		"data": map[string]any{"total_tokens": 10, "total_cost": 99.9},
	}, time.Now())
	require.Error(t, err)
	require.Contains(t, err.Error(), "实际扣费")

	_, err = parseSub2APISnapshotTrend(map[string]any{
		"data": map[string]any{"trend": []any{map[string]any{
			"date": "2026-07-15", "total_tokens": 10, "cost": 99.9,
		}}},
	}, []time.Time{time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)}, time.UTC)
	require.Error(t, err)
	require.Contains(t, err.Error(), "实际扣费")
}

func TestSub2APISnapshotTrendFillsMissingRequestedDateWithZero(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	first := time.Date(2026, 7, 14, 0, 0, 0, 0, loc)
	second := first.AddDate(0, 0, 1)

	result, err := parseSub2APISnapshotTrend(map[string]any{
		"data": map[string]any{"trend": []any{map[string]any{
			"date": first.Format("2006-01-02"), "total_tokens": 10, "actual_cost": 1.2,
		}}},
	}, []time.Time{first, second}, loc)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, first, result[0].Date)
	require.Equal(t, int64(10), result[0].Tokens)
	require.InDelta(t, 1.2, result[0].CostUSD, 1e-9)
	require.Equal(t, second, result[1].Date)
	require.Zero(t, result[1].Tokens)
	require.Zero(t, result[1].CostUSD)
}

func TestSub2APISnapshotTrendRejectsInvalidDate(t *testing.T) {
	requested := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	result, err := parseSub2APISnapshotTrend(map[string]any{
		"data": map[string]any{"trend": []any{map[string]any{
			"date": "2026-99-99", "total_tokens": 10, "actual_cost": 1.2,
		}}},
	}, []time.Time{requested}, time.UTC)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "日期")
}

func TestSub2APIUpstreamProviderOnlyFallsBackOnSnapshotNotFound(t *testing.T) {
	var legacyCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/usage/dashboard/snapshot-v2", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})
	mux.HandleFunc("/api/v1/usage/stats", func(w http.ResponseWriter, _ *http.Request) {
		legacyCalls.Add(1)
		writeUpstreamJSON(t, w, map[string]any{"total_tokens": 1, "total_actual_cost": 0.1})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := &sub2APIUpstreamProvider{http: newTestUpstreamHTTPClient(t)}
	daily, groups, err := provider.fetchUsage(context.Background(), UpstreamSyncRequest{
		Site:     &UpstreamSite{BaseURL: server.URL},
		Dates:    []time.Time{time.Now()},
		Location: time.Local,
	}, nil, nil)
	require.Error(t, err)
	require.Nil(t, daily)
	require.Nil(t, groups)
	require.Equal(t, int32(0), legacyCalls.Load())
}

func TestSub2APIUpstreamProviderRefreshesOnceAndRestartsSync(t *testing.T) {
	var meCalls atomic.Int32
	var groupCalls atomic.Int32
	var refreshCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, _ *http.Request) {
		meCalls.Add(1)
		writeUpstreamJSON(t, w, map[string]any{"balance": 1})
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, r *http.Request) {
		groupCalls.Add(1)
		if r.Header.Get("Authorization") == "Bearer expired" {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer refreshed", r.Header.Get("Authorization"))
		writeUpstreamJSON(t, w, []any{})
	})
	mux.HandleFunc("/api/v1/groups/rates", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{})
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, _ *http.Request) {
		refreshCalls.Add(1)
		writeUpstreamJSON(t, w, map[string]any{"access_token": "refreshed", "refresh_token": "refresh-2"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site: &UpstreamSite{
			BaseURL: server.URL, Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthToken,
		},
		Credential: UpstreamCredential{AccessToken: "expired", RefreshToken: "refresh-1"},
		Location:   time.Local,
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), meCalls.Load(), "恢复认证后应从账号信息重新开始完整同步")
	require.Equal(t, int32(2), groupCalls.Load())
	require.Equal(t, int32(1), refreshCalls.Load())
	require.Equal(t, "refreshed", result.Credential.AccessToken)
	require.Equal(t, "refresh-2", result.Credential.RefreshToken)
}

func TestSub2APIUpstreamProviderStopsAfterSecondUnauthorized(t *testing.T) {
	var meCalls atomic.Int32
	var groupCalls atomic.Int32
	var refreshCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, _ *http.Request) {
		meCalls.Add(1)
		writeUpstreamJSON(t, w, map[string]any{"balance": 1})
	})
	mux.HandleFunc("/api/v1/groups/available", func(w http.ResponseWriter, _ *http.Request) {
		groupCalls.Add(1)
		http.Error(w, "expired", http.StatusForbidden)
	})
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, _ *http.Request) {
		refreshCalls.Add(1)
		writeUpstreamJSON(t, w, map[string]any{"access_token": "refreshed", "refresh_token": "refresh-2"})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := newSub2APIUpstreamProvider(newTestUpstreamHTTPClient(t))
	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site: &UpstreamSite{
			BaseURL: server.URL, Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthToken,
		},
		Credential: UpstreamCredential{AccessToken: "expired", RefreshToken: "refresh"},
	})
	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, "refreshed", result.Credential.AccessToken)
	require.Equal(t, "refresh-2", result.Credential.RefreshToken)
	require.Equal(t, int32(2), meCalls.Load())
	require.Equal(t, int32(2), groupCalls.Load())
	require.Equal(t, int32(1), refreshCalls.Load(), "认证恢复不得循环")
}

func TestNewAPIUpstreamProviderReloginsOnceAndRestartsSync(t *testing.T) {
	var loginCalls atomic.Int32
	var selfCalls atomic.Int32
	var groupCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		attempt := loginCalls.Add(1)
		cookie := "cookie-2"
		if attempt == 1 {
			cookie = "cookie-1"
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: cookie})
		writeUpstreamJSON(t, w, map[string]any{"id": 7})
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, _ *http.Request) {
		selfCalls.Add(1)
		writeUpstreamJSON(t, w, map[string]any{"quota": 100})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 100})
	})
	mux.HandleFunc("/api/user/self/groups", func(w http.ResponseWriter, r *http.Request) {
		groupCalls.Add(1)
		if r.Header.Get("Cookie") == "session=cookie-1" {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		require.Equal(t, "session=cookie-2", r.Header.Get("Cookie"))
		writeUpstreamJSON(t, w, map[string]any{})
	})
	mux.HandleFunc("/api/pricing", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{})
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
	require.NoError(t, err)
	require.Equal(t, int32(2), loginCalls.Load())
	require.Equal(t, int32(2), selfCalls.Load(), "重新登录后应完整重读账号信息")
	require.Equal(t, int32(2), groupCalls.Load())
	require.Equal(t, "session=cookie-2", result.Credential.Cookie)
}

func TestNewAPIUpstreamProviderStopsAfterSecondUnauthorized(t *testing.T) {
	var loginCalls atomic.Int32
	var groupCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/user/login", func(w http.ResponseWriter, _ *http.Request) {
		loginCalls.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie"})
		writeUpstreamJSON(t, w, map[string]any{"id": 7})
	})
	mux.HandleFunc("/api/user/self", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota": 100})
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 100})
	})
	mux.HandleFunc("/api/user/self/groups", func(w http.ResponseWriter, _ *http.Request) {
		groupCalls.Add(1)
		http.Error(w, "expired", http.StatusUnauthorized)
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
	require.Nil(t, result)
	require.Equal(t, int32(2), loginCalls.Load(), "重新认证不得超过一次")
	require.Equal(t, int32(2), groupCalls.Load())
}
