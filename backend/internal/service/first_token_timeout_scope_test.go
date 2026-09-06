//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func scopedFirstTokenSettings() GatewaySettings {
	settings := DefaultGatewaySettings()
	settings.FirstTokenTimeoutScope = FirstTokenTimeoutScopeSelectedGroups
	settings.FirstTokenTimeoutGroupIDs = []int64{46}
	settings.FirstTokenTimeoutSeconds = 20
	return settings
}

func scopedFirstTokenContext(groupID int64) context.Context {
	return context.WithValue(context.Background(), ctxkey.Group, &Group{ID: groupID})
}

func TestFirstToken分组范围与协议边界(t *testing.T) {
	settings := scopedFirstTokenSettings()
	settingSvc := &SettingService{}
	settingSvc.storeGatewaySettingsCache(settings, time.Hour)
	rateLimit := &RateLimitService{settingService: settingSvc}
	for _, tc := range []struct {
		name    string
		groupID int64
		stream  bool
		ws      bool
		want    bool
	}{
		{"选中分组", 46, true, false, true},
		{"其它分组", 26, true, false, false},
		{"缺少分组", 0, true, false, false},
		{"非流式", 46, false, false, false},
		{"不扩展WebSocket", 46, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(scopedFirstTokenContext(tc.groupID))
			if tc.ws {
				SetOpenAIClientTransport(c, OpenAIClientTransportWS)
			}
			a := newFirstTokenAttempt(c.Request.Context(), c, rateLimit, &Account{ID: 1}, "gpt-test", tc.stream)
			if !tc.want {
				require.Nil(t, a)
				return
			}
			require.NotNil(t, a)
			require.True(t, a.strictOutput)
			require.True(t, isFirstTokenScopedRequest(c.Request.Context()))
			require.Equal(t, 20*time.Second, a.timeout)
			_ = a.finish(nil)
		})
	}
}

func TestFirstToken严格模式不接受空白但保留全局语义(t *testing.T) {
	for _, payload := range []string{
		`{"type":"response.output_text.delta","delta":" "}`,
		`{"type":"response.reasoning_text.delta","delta":"\n"}`,
		`{"choices":[{"delta":{"content":"\t"}}]}`,
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":" "}}`,
		`{"candidates":[{"content":{"parts":[{"text":" "}]}}]}`,
	} {
		require.True(t, isMeaningfulFirstTokenJSON([]byte(payload)))
		require.False(t, isMeaningfulFirstTokenJSON([]byte(payload), true))
	}
	require.True(t, isMeaningfulFirstTokenJSON([]byte(`{"type":"response.function_call_arguments.delta","delta":"{"}`), true))
	require.False(t, openAIStreamDataStartsClientOutput(`{"type":"response.output_text.delta","delta":" "}`, "response.output_text.delta", true))
	require.False(t, openAIStreamDataStartsClientOutput(`{"type":"response.content_part.done","part":{"text":" "}}`, "response.content_part.done", true))
}

func TestFirstToken严格模式超时丢弃空白并保持可以换号(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	a := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "gpt-test", 30*time.Millisecond)
	a.strictOutput = true
	resp := &http.Response{Body: io.NopCloser(io.MultiReader(
		strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\" \"}\n\n"),
		&contextReadCloser{ctx: a.requestCtx},
	))}
	a.wrapResponse(resp, c, firstTokenProtocolSSE)
	_, readErr := io.ReadAll(resp.Body)
	require.ErrorIs(t, readErr, errFirstTokenAttemptTimedOut)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, a.finish(readErr), &failover)
	require.True(t, failover.FirstTokenTimeout)
	require.False(t, failover.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
}

func TestFirstToken严格模式保留缩进和完整原始响应(t *testing.T) {
	a := newFirstTokenAttemptWithTimeout(context.Background(), nil, nil, &Account{ID: 1}, "gpt-test", time.Second)
	a.strictOutput = true
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"  \"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	a.wrapResponse(resp, nil, firstTokenProtocolSSE)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
	require.Equal(t, firstTokenAttemptReceived, a.currentState())
	require.NoError(t, a.finish(nil))
}

func TestFirstToken指定分组以外不清零超时计数(t *testing.T) {
	settings := scopedFirstTokenSettings()
	settingSvc := &SettingService{}
	settingSvc.storeGatewaySettingsCache(settings, time.Hour)
	streak := &firstTokenStreakCacheStub{}
	svc := &RateLimitService{settingService: settingSvc, accountFailureStreakCache: streak}
	event := NewAccountFailureStreakEvent(time.Now())
	svc.RecordUpstreamSuccessOutcomeAt(scopedFirstTokenContext(26), 1, event)
	require.Equal(t, []AccountFailureStreakSource{AccountFailureStreakSourceUpstreamError}, streak.sources)
	streak.sources = nil
	svc.RecordUpstreamSuccessOutcomeAt(scopedFirstTokenContext(46), 1, event)
	require.Equal(t, []AccountFailureStreakSource{AccountFailureStreakSourceUpstreamError}, streak.sources)
	streak.sources = nil
	svc.RecordUpstreamSuccessOutcomeAt(markFirstTokenScopedRequest(scopedFirstTokenContext(46), nil), 1, event)
	require.Equal(t, []AccountFailureStreakSource{AccountFailureStreakSourceFirstTokenTimeout, AccountFailureStreakSourceUpstreamError}, streak.sources)
}

func TestFirstToken指定分组达到阈值保存达速恢复快照(t *testing.T) {
	settings := scopedFirstTokenSettings()
	settingSvc := &SettingService{}
	settingSvc.storeGatewaySettingsCache(settings, time.Hour)
	repo := &firstTokenAtomicRepoStub{}
	svc := &RateLimitService{settingService: settingSvc, accountRepo: repo, accountFailureStreakCache: &firstTokenStreakCacheStub{
		state: AccountFailureStreakState{Count: 3, Applied: true},
	}}
	ctx := markFirstTokenScopedRequest(scopedFirstTokenContext(46), nil)
	account := &Account{ID: 1, Schedulable: true}
	require.NoError(t, svc.handleFirstTokenTimeoutOutcome(ctx, account, "gpt-test", 20,
		BuildAccountFailureStreakPolicy(AccountFailureStreakSourceFirstTokenTimeout, settings), 3, NewAccountFailureStreakEvent(time.Now()), true))
	require.False(t, account.Schedulable)
	require.Equal(t, true, repo.marker[accountFailureStrategyUnscheduledStrictOutputKey])
	require.Equal(t, "gpt-test", repo.marker[accountFailureStrategyUnscheduledModelKey])
}

func TestFirstToken严格模式空白后流内错误仍可换号(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, event := range []string{
			`{"type":"response.failed","error":{"code":"server_error","message":"upstream failed"}}`,
			`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"Too many requests"}}`,
		} {
			t.Run(event+map[bool]string{false: "常规转发", true: "透传"}[passthrough], func(t *testing.T) {
				c, recorder := newTestContext()
				c.Request = c.Request.WithContext(scopedFirstTokenContext(46))
				settings := &SettingService{}
				settings.storeGatewaySettingsCache(scopedFirstTokenSettings(), time.Hour)
				rateLimit := &RateLimitService{settingService: settings}
				account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
				a := newFirstTokenAttempt(c.Request.Context(), c, rateLimit, account, "gpt-test", true)
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}}
				resp.Body = io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\" \"}\n\ndata: " + event + "\n\n"))
				a.wrapResponse(resp, c, firstTokenProtocolSSE)
				defer resp.Body.Close()
				svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
				var err error
				if passthrough {
					_, err = svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-test", "gpt-test")
				} else {
					_, err = svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-test", "gpt-test")
				}
				var failover *UpstreamFailoverError
				require.ErrorAs(t, a.finish(err), &failover)
				require.False(t, c.Writer.Written())
				require.Empty(t, recorder.Body.String())
			})
		}
	}
}
