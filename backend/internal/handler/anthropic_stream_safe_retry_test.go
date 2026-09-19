//go:build unit

package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const safeHandlerPrelude = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"content\":[],\"usage\":{\"input_tokens\":7}}}\n\n"
const safeHandlerOutput = "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"有效正文\"}}\n\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\n"
const safeHandlerStop = "data: {\"type\":\"message_stop\"}\n\n"

type safeHandlerSettingsRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *safeHandlerSettingsRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return r.values, nil
}
func (r *safeHandlerSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}
func (r *safeHandlerSettingsRepo) Get(_ context.Context, key string) (*service.Setting, error) {
	return &service.Setting{Key: key, Value: r.values[key]}, nil
}

type safeHandlerUsageRepo struct {
	service.UsageLogRepository
	logs []*service.UsageLog
}

func (r *safeHandlerUsageRepo) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	copyLog := *log
	r.logs = append(r.logs, &copyLog)
	return true, nil
}

type safeHandlerUpstream struct {
	service.HTTPUpstream
	earlyKeepalive bool
	statuses       []int
	bodies         []string
	accounts       []int64
	requests       [][]byte
	responseBody   func(*http.Request, int) io.ReadCloser
	runRequest     func(func())
}

func (u *safeHandlerUpstream) DoWithTLS(req *http.Request, _ string, accountID int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	u.requests = append(u.requests, body)
	u.accounts = append(u.accounts, accountID)
	i := len(u.accounts) - 1
	if i >= len(u.bodies) {
		return nil, fmt.Errorf("超出预期上游调用次数")
	}
	responseBody := io.NopCloser(strings.NewReader(u.bodies[i]))
	if u.responseBody != nil {
		responseBody = u.responseBody(req, i)
	}
	status := http.StatusOK
	if i < len(u.statuses) {
		status = u.statuses[i]
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/event-stream"}, "X-Request-Id": {fmt.Sprintf("upstream-%d", i)}}, Body: responseBody}, nil
}

func runSafeHandlerFixture(t *testing.T, upstream *safeHandlerUpstream, oneAccount bool, inspect ...func(*gin.Context)) (*httptest.ResponseRecorder, *safeHandlerUsageRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupID := int64(41)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, RateMultiplier: 1}
	makeAccount := func(id int64, priority int) *service.Account {
		return &service.Account{ID: id, Name: fmt.Sprint(id), Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: priority, Concurrency: 1, AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: groupID}}, Credentials: map[string]any{"api_key": "test-key", "base_url": "https://example.test", "pool_mode": true, "pool_mode_retry_count": 10}, Extra: map[string]any{"anthropic_passthrough": true}}
	}
	accounts := []*service.Account{makeAccount(740, 1)}
	if !oneAccount {
		accounts = append(accounts, makeAccount(741, 2))
	}
	disabled := makeAccount(742, 0)
	disabled.Schedulable = false
	accounts = append(accounts, disabled)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Gateway.MaxLineSize = 8 << 20
	cfg.Gateway.StreamDataIntervalTimeout = 180
	settings := service.NewSettingService(&safeHandlerSettingsRepo{values: map[string]string{
		service.SettingKeyGatewayAnthropicStreamSafeRetryEarlyKeepaliveEnabled: fmt.Sprint(upstream.earlyKeepalive),
		service.SettingKeyGatewayAnthropicStreamSafeRetryEnabled:               "true", service.SettingKeyGatewayAnthropicStreamSafeRetryMaxRetries: "2", service.SettingKeyGatewayAnthropicStreamSafeRetryTotalWaitSeconds: "300", service.SettingKeyGatewayFirstTokenTimeoutSeconds: "0",
	}}, cfg)
	rate := service.NewRateLimitService(nil, nil, cfg, nil, nil)
	rate.SetSettingService(settings)
	usages := &safeHandlerUsageRepo{}
	snapshot := service.NewSchedulerSnapshotService(&fakeSchedulerCache{accounts: accounts}, nil, nil, nil, nil)
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	defer billingCache.Stop()
	gateway := service.NewGatewayService(nil, &fakeGroupRepo{group: group}, usages, nil, nil, nil, nil, nil, cfg, snapshot, nil, service.NewBillingService(cfg, nil), rate, billingCache, nil, upstream, &service.DeferredService{}, nil, nil, nil, nil, settings, nil, nil, nil, nil, nil, nil)
	h := &GatewayHandler{gatewayService: gateway, billingCacheService: billingCache, concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(&fakeConcurrencyCache{}), SSEPingFormatClaude, 0), cfg: cfg, maxAccountSwitches: 2}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := []byte(`{"model":"claude-sonnet-4-5","stream":true,"max_tokens":50,"messages":[{"role":"user","content":"测试请求"}]}`)
	c.Request = httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(c.Request.Context(), ctxkey.Group, group)
	ctx = context.WithValue(ctx, ctxkey.ClientRequestID, "client-stable-id")
	c.Request = c.Request.WithContext(ctx)
	key := &service.APIKey{ID: 1, UserID: 2, GroupID: &groupID, Status: service.StatusActive, User: &service.User{ID: 2, Concurrency: 10, Balance: 100}, Group: group}
	c.Set(string(middleware.ContextKeyAPIKey), key)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 10})
	run := func() {
		h.Messages(c)
		for _, observe := range inspect {
			observe(c)
		}
	}
	if upstream.runRequest != nil {
		upstream.runRequest(run)
	} else {
		run()
	}
	require.NoError(t, c.Request.Context().Err(), "内部预算清理不能冒充客户端取消")
	require.False(t, disabled.Schedulable)
	return w, usages
}

func TestAnthropicStreamSafeRetryHandlerRecoveryAndUsage(t *testing.T) {
	for _, one := range []bool{false, true} {
		t.Run(fmt.Sprint(one), func(t *testing.T) {
			func(t *testing.T) {
				upstream := &safeHandlerUpstream{bodies: []string{safeHandlerPrelude, safeHandlerPrelude, safeHandlerPrelude + safeHandlerOutput + safeHandlerStop}}
				w, usage := runSafeHandlerFixture(t, upstream, one)
				expected := []int64{740, 740, 741}
				if one {
					expected[2] = 740
				}
				require.Equal(t, expected, upstream.accounts)
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, 1, strings.Count(w.Body.String(), "event: message_start"))
				require.Contains(t, w.Body.String(), "有效正文")
				require.NotContains(t, w.Body.String(), "upstream_error")
				require.Len(t, usage.logs, 1)
				require.Equal(t, expected[2], usage.logs[0].AccountID)
				require.Equal(t, 3, usage.logs[0].OutputTokens)
				require.Equal(t, upstream.requests[0], upstream.requests[1])
				require.Equal(t, upstream.requests[1], upstream.requests[2])
			}(t)
		})
	}
}
func TestAnthropicStreamSafeRetryHandlerExhaustionRetainsLatestUsage(t *testing.T) {
	func(t *testing.T) {
		upstream := &safeHandlerUpstream{bodies: []string{safeHandlerPrelude, safeHandlerPrelude, ""}}
		w, usage := runSafeHandlerFixture(t, upstream, false)
		require.Equal(t, []int64{740, 740, 741}, upstream.accounts)
		require.Equal(t, http.StatusBadGateway, w.Code)
		require.NotContains(t, w.Body.String(), "message_start")
		require.NotContains(t, w.Body.String(), "message_stop")
		require.Len(t, usage.logs, 1)
		require.Equal(t, int64(740), usage.logs[0].AccountID)
		require.Equal(t, 7, usage.logs[0].InputTokens)
	}(t)
}

func TestAnthropicStreamSafeRetryHandlerFirstContentRecovery(t *testing.T) {
	upstream := &safeHandlerUpstream{bodies: []string{"", safeHandlerPrelude + safeHandlerOutput + safeHandlerStop}}
	upstream.responseBody = func(request *http.Request, attempt int) io.ReadCloser {
		if attempt != 0 {
			return io.NopCloser(strings.NewReader(upstream.bodies[attempt]))
		}
		reader, writer := io.Pipe()
		go func() {
			defer writer.Close()
			if _, err := io.WriteString(writer, strings.Replace(safeHandlerPrelude, "7", "99", 1)); err != nil {
				return
			}
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-request.Context().Done():
					return
				case <-ticker.C:
					if _, err := io.WriteString(writer, ": heartbeat\n\n"); err != nil {
						return
					}
				}
			}
		}()
		return reader
	}
	upstream.runRequest = func(run func()) {
		synctest.Test(t, func(t *testing.T) {
			started := time.Now()
			run()
			require.Equal(t, 180*time.Second+sameAccountRetryDelay, time.Since(started))
		})
	}
	writer, usage := runSafeHandlerFixture(t, upstream, true, func(ginContext *gin.Context) {
		events := ginContext.MustGet(service.OpsUpstreamErrorsKey).([]*service.OpsUpstreamErrorEvent)
		require.Len(t, events, 1)
		require.Equal(t, "first_content_timeout", events[0].StreamDiagnostic.FailureKind)
		require.True(t, events[0].StreamDiagnostic.Recovered)
		var final service.AnthropicStreamDiagnostic
		service.AnthropicStreamRetryFromGin(ginContext).Diagnostic(&final)
		require.Equal(t, 2, final.Attempt)
	})
	require.Equal(t, []int64{740, 740}, upstream.accounts)
	require.Equal(t, http.StatusOK, writer.Code)
	require.Equal(t, 1, strings.Count(writer.Body.String(), "event: message_start"))
	require.NotContains(t, writer.Body.String(), "99")
	require.NotContains(t, writer.Body.String(), "heartbeat")
	require.Len(t, usage.logs, 1)
	require.Equal(t, 7, usage.logs[0].InputTokens)
	require.Equal(t, 3, usage.logs[0].OutputTokens)
}
func TestAnthropicStreamSafeRetryHandlerPartialContentNeverRegenerates(t *testing.T) {
	func(t *testing.T) {
		upstream := &safeHandlerUpstream{bodies: []string{safeHandlerPrelude + safeHandlerOutput}}
		w, usage := runSafeHandlerFixture(t, upstream, false)
		require.Equal(t, []int64{740}, upstream.accounts)
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), "有效正文")
		require.Contains(t, w.Body.String(), `"type":"error"`)
		require.NotContains(t, w.Body.String(), "message_stop")
		require.True(t, strings.HasSuffix(w.Body.String(), "\n\n"))
		require.Len(t, usage.logs, 1)
	}(t)
}

func TestAnthropicStreamSafeRetryHTTPOutcomeIsRecordedOnce(t *testing.T) {
	ctx := context.Background()
	state := NewFailoverState(10, false)
	recorder := &mockTempUnscheduler{}
	fault := &service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable, RetryableOnSameAccount: true}
	for range 3 {
		state.RecordSafeStreamHTTPFailure(ctx, recorder, 740, fault)
	}
	require.Empty(t, recorder.outcomeCalls)
	require.Empty(t, state.SameAccountRetryCount)
	require.Zero(t, state.SwitchCount)
	state.FinalizePendingOutcomes(ctx, recorder)
	require.Len(t, recorder.outcomeCalls, 1)
	state = NewFailoverState(10, false)
	recorder = &mockTempUnscheduler{}
	state.RecordSafeStreamHTTPFailure(ctx, recorder, 740, fault)
	state.RecordSuccessOutcome(ctx, recorder, 740)
	state.FinalizePendingOutcomes(ctx, recorder)
	require.Empty(t, recorder.outcomeCalls)
	require.Equal(t, []int64{740}, recorder.successOutcomeIDs)
	recorder.strictResult = true
	state.RecordSafeStreamHTTPFailure(ctx, recorder, 741, fault)
	require.Contains(t, state.FailedAccountIDs, int64(741))
}
