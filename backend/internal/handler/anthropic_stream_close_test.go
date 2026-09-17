//go:build unit

package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamSafeRetryHandlerTerminalReturnsHTTPEOF(tester *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := safeHandlerPrelude + safeHandlerOutput + safeHandlerStop
	for _, protocol := range []string{"HTTP/1.1", "HTTP/2.0"} {
		for _, encoding := range []string{"identity", "gzip", "zstd"} {
			for _, safeRetry := range []bool{false, true} {
				for _, firstToken := range []int{0, 45} {
					tester.Run(fmt.Sprintf("%s/%s/safe=%v/first=%d", protocol, encoding, safeRetry, firstToken), func(tester *testing.T) {
						encoded := terminalStreamPayload(tester, encoding, payload)
						release := make(chan struct{})
						cancelled := make(chan struct{}, 2)
						var dispatches atomic.Int32
						upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
							dispatches.Add(1)
							_, _ = io.Copy(io.Discard, request.Body)
							writer.Header().Set("Content-Type", "text/event-stream")
							writer.Header().Set("Content-Encoding", encoding)
							_, _ = writer.Write(encoded)
							_ = http.NewResponseController(writer).Flush()
							select {
							case <-request.Context().Done():
								cancelled <- struct{}{}
							case <-release:
							}
						}))
						handler, usage := terminalStreamHandler(tester, upstream.URL, safeRetry, firstToken)
						handlerDone := make(chan struct{})
						server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
							defer close(handlerDone)
							handler.ServeHTTP(writer, request)
						}))
						server.EnableHTTP2 = protocol == "HTTP/2.0"
						server.StartTLS()
						tester.Cleanup(func() {
							close(release)
							server.Close()
							upstream.Close()
						})
						client := server.Client()
						client.Timeout = 3 * time.Second
						requestBody := `{"model":"claude-sonnet-4-5","stream":true,"max_tokens":50,"messages":[{"role":"user","content":"测试"}]}`
						response, err := client.Post(server.URL+"/v1/messages", "application/json", strings.NewReader(requestBody))
						require.NoError(tester, err)
						defer response.Body.Close()
						actual, err := io.ReadAll(response.Body)
						require.NoError(tester, err, "上游不结束时，下游仍须在截止时间内正常EOF，而非客户端超时关闭")
						require.Equal(tester, http.StatusOK, response.StatusCode, string(actual))
						require.Equal(tester, protocol, response.Proto)
						require.Equal(tester, payload, string(actual))
						select {
						case <-cancelled:
						case <-time.After(time.Second):
							tester.Fatal("完整终止后必须取消仍在等待的上游请求")
						}
						<-handlerDone
						require.EqualValues(tester, 1, dispatches.Load())
						require.Len(tester, usage.logs, 1)
						require.Equal(tester, int64(740), usage.logs[0].AccountID)
						require.Equal(tester, 7, usage.logs[0].InputTokens)
						require.Equal(tester, 3, usage.logs[0].OutputTokens)
					})
				}
			}
		}
	}
}

func terminalStreamPayload(tester *testing.T, encoding, payload string) []byte {
	tester.Helper()
	var buffer bytes.Buffer
	var encoder interface {
		io.WriteCloser
		Flush() error
	}
	switch encoding {
	case "gzip":
		encoder = gzip.NewWriter(&buffer)
	case "zstd":
		var err error
		encoder, err = zstd.NewWriter(&buffer)
		require.NoError(tester, err)
	default:
		return []byte(payload)
	}
	_, err := io.WriteString(encoder, payload)
	require.NoError(tester, err)
	require.NoError(tester, encoder.Flush())
	encoded := bytes.Clone(buffer.Bytes())
	require.NoError(tester, encoder.Close())
	return encoded
}

func terminalStreamHandler(tester *testing.T, baseURL string, safeRetry bool, firstToken int) (http.Handler, *safeHandlerUsageRepo) {
	tester.Helper()
	groupID := int64(41)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
	account := &service.Account{
		ID: 740, Name: "流终止测试", Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		AccountGroups: []service.AccountGroup{{AccountID: 740, GroupID: groupID}},
		Credentials:   map[string]any{"api_key": "test-key", "base_url": baseURL, "pool_mode": true, "pool_mode_retry_count": 10},
		Extra:         map[string]any{"anthropic_passthrough": true},
	}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.MaxLineSize = 8 << 20
	cfg.Gateway.StreamDataIntervalTimeout = 180
	settings := service.NewSettingService(&safeHandlerSettingsRepo{values: map[string]string{
		service.SettingKeyGatewayAnthropicStreamSafeRetryEnabled:          strconv.FormatBool(safeRetry),
		service.SettingKeyGatewayAnthropicStreamSafeRetryMaxRetries:       "2",
		service.SettingKeyGatewayAnthropicStreamSafeRetryTotalWaitSeconds: "300",
		service.SettingKeyGatewayFirstTokenTimeoutSeconds:                 strconv.Itoa(firstToken),
	}}, cfg)
	rate := service.NewRateLimitService(nil, nil, cfg, nil, nil)
	rate.SetSettingService(settings)
	usage := &safeHandlerUsageRepo{}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: []*service.Account{account}}, nil, nil, nil, nil)
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	tester.Cleanup(billingCache.Stop)
	gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, usage, nil, nil, nil, nil, nil, cfg, snapshot, nil, service.NewBillingService(cfg, nil), rate, billingCache, nil, repository.NewHTTPUpstream(nil), &service.DeferredService{}, nil, nil, nil, nil, settings, nil, nil, nil, nil, nil, nil)
	handler := &GatewayHandler{gatewayService: gateway, billingCacheService: billingCache, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0), cfg: cfg, maxAccountSwitches: 2}
	key := &service.APIKey{ID: 1, UserID: 2, GroupID: &groupID, Status: service.StatusActive, User: &service.User{ID: 2, Concurrency: 10, Balance: 100}, Group: group}
	router := gin.New()
	router.POST("/v1/messages", func(ginContext *gin.Context) {
		ctx := context.WithValue(ginContext.Request.Context(), ctxkey.Group, group)
		ginContext.Request = ginContext.Request.WithContext(ctx)
		ginContext.Set(string(middleware.ContextKeyAPIKey), key)
		ginContext.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 10})
		handler.Messages(ginContext)
	})
	return router, usage
}
