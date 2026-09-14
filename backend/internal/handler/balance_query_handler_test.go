package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type queryRepoStub struct {
	service.UserCustomizationRepository
	resolves  int
	histories int
	pageSize  int
}

func (r *queryRepoStub) ResolvePublic(_ context.Context, hash string) (*service.PublicBalanceUser, error) {
	r.resolves++
	if hash != service.BalanceQueryTokenHash(strings.Repeat("A", 43)) {
		return nil, service.ErrBalanceQueryUnavailable
	}
	return &service.PublicBalanceUser{ID: 7, Username: "公开用户名", Balance: 123}, nil
}
func (r *queryRepoStub) PublicHistory(_ context.Context, id int64, _, size int) ([]service.PublicRechargeRecord, int64, error) {
	if id != 7 {
		panic("跨用户查询")
	}
	r.histories++
	r.pageSize = size
	return []service.PublicRechargeRecord{{Type: "temporary_credit", Amount: 500}}, 1, nil
}

type querySettingsStub struct{ service.SettingRepository }

func (*querySettingsStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

type queryLimiterStub struct {
	calls    int
	failAt   int
	rejectAt int
	keys     []string
}

func (l *queryLimiterStub) Allow(_ context.Context, key string, _ int, _ time.Duration) (middleware.AllowResult, error) {
	l.calls++
	l.keys = append(l.keys, key)
	if l.calls == l.failAt {
		return middleware.AllowResult{}, errors.New("Redis 断开")
	}
	return middleware.AllowResult{Allowed: l.calls != l.rejectAt, RetryAfter: 12 * time.Second}, nil
}

func runBalanceQuery(t *testing.T, query, token string, limiter *queryLimiterStub) (*httptest.ResponseRecorder, *queryRepoStub) {
	t.Helper()
	repo := &queryRepoStub{}
	h := NewBalanceQueryHandler(service.NewUserCustomizationService(repo, &querySettingsStub{}, nil), nil)
	h.limiter = limiter
	engine := gin.New()
	engine.GET("/api/v1/public/balance-query", h.Query)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/balance-query"+query, nil)
	request.Header.Set(BalanceQueryTokenHeader, token)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder, repo
}

func TestBalanceQueryReadOnlyAndSafeFields(t *testing.T) {
	limiter := &queryLimiterStub{}
	recorder, repo := runBalanceQuery(t, "?page_size=100", strings.Repeat("A", 43), limiter)
	require.Equal(t, 200, recorder.Code)
	require.Equal(t, 50, repo.pageSize)
	require.Contains(t, recorder.Body.String(), "临时授信500")
	require.NotContains(t, recorder.Body.String(), "email")
	require.NotContains(t, recorder.Body.String(), "credit_generation")
	require.Equal(t, "no-store, private", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
	require.Contains(t, recorder.Header().Get("X-Robots-Tag"), "noindex")
	require.Len(t, limiter.keys, 2)
	require.NotContains(t, limiter.keys[1], strings.Repeat("A", 43))
}

func TestBalanceQueryRejectsInvalidLinksAndUserOverride(t *testing.T) {
	for _, query := range []string{"?user_id=8", "?token=secret", "?page=0", "?page_size=-1"} {
		recorder, repo := runBalanceQuery(t, query, strings.Repeat("A", 43), &queryLimiterStub{})
		require.Equal(t, 400, recorder.Code)
		require.Zero(t, repo.histories)
	}
	for _, token := range []string{"", "invalid", strings.Repeat("B", 43)} {
		recorder, repo := runBalanceQuery(t, "", token, &queryLimiterStub{})
		require.Equal(t, 404, recorder.Code)
		require.Zero(t, repo.histories)
	}
}

func TestBalanceQueryRateLimitsFailClosed(t *testing.T) {
	for _, stage := range []int{1, 2} {
		recorder, repo := runBalanceQuery(t, "", strings.Repeat("A", 43), &queryLimiterStub{rejectAt: stage})
		require.Equal(t, 429, recorder.Code)
		require.Equal(t, "12", recorder.Header().Get("Retry-After"))
		require.Zero(t, repo.histories)
		recorder, repo = runBalanceQuery(t, "", strings.Repeat("A", 43), &queryLimiterStub{failAt: stage})
		require.Equal(t, 503, recorder.Code)
		require.Zero(t, repo.histories)
	}
}
