//go:build integration

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9" //nolint:depguard // 集成测试直接控制 Redis 故障，生产处理器仅注入限流器。
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestBalanceQueryRedisLimitsAndFailure(t *testing.T) {
	ctx := context.Background()
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("集成测试需要本机 Docker", err)
		}
		t.Skip("本机 Docker 不可用")
	}
	container, err := tcredis.Run(ctx, "redis:8.4-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: endpoint, MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { _ = client.Close() })
	repo := &queryRepoStub{}
	h := NewBalanceQueryHandler(service.NewUserCustomizationService(repo, &querySettingsStub{}, nil), middleware.NewRateLimiter(client))
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/api/v1/public/balance-query", h.Query)
	token := strings.Repeat("A", 43)
	request := func(ip, secret string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/public/balance-query", nil)
		req.RemoteAddr = ip + ":1234"
		req.Header.Set(BalanceQueryTokenHeader, secret)
		req.Header.Set("X-Forwarded-For", "192.0.2.100")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	for i := 0; i < 30; i++ {
		require.Equal(t, http.StatusOK, request("192.0.2.1", token).Code)
	}
	limited := request("192.0.2.1", token)
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.NotEmpty(t, limited.Header().Get("Retry-After"))
	for i := 0; i < 30; i++ {
		require.Equal(t, http.StatusOK, request(fmt.Sprintf("192.0.2.%d", i+2), token).Code)
	}
	require.Equal(t, http.StatusTooManyRequests, request("198.51.100.1", token).Code, "有效链接跨 IP 共用 60 次限制")
	require.Equal(t, 60, repo.histories)
	require.NoError(t, client.FlushDB(ctx).Err())
	for i := 0; i < 30; i++ {
		require.Equal(t, http.StatusNotFound, request("192.0.2.1", "无效").Code)
	}
	require.Equal(t, http.StatusTooManyRequests, request("192.0.2.1", "无效").Code)
	require.Equal(t, int64(0), client.Exists(ctx, "rate_limit:balance-query:link:"+service.BalanceQueryTokenHash(token)).Val())
	require.NoError(t, client.Close())
	unavailable := request("203.0.113.1", token)
	require.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
	require.Equal(t, 60, repo.histories, "Redis 故障不能无保护查询")
}
