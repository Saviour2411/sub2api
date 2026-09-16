//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const safeStart = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"content\":[],\"usage\":{\"input_tokens\":7}}}\n\n"
const safeText = "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"测试正文\"}}\n\n"
const safeStop = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

func safeRetryAccount(id int64) *Account {
	a := newAnthropicAPIKeyAccountForTest()
	a.ID = id
	a.Credentials["pool_mode"] = true
	a.Credentials["pool_mode_retry_count"] = 10
	return a
}
func newSafeRetryTest(t *testing.T) (*GatewayService, *gin.Context, *httptest.ResponseRecorder, *AnthropicStreamRetryState) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	settings := DefaultGatewaySettings()
	settings.AnthropicStreamSafeRetryEnabled = true
	r := newAnthropicStreamRetryState(c.Request.Context(), settings)
	c.Set(anthropicStreamRetryContextKey, r)
	t.Cleanup(r.Close)
	s := &GatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize, StreamDataIntervalTimeout: 180, StreamKeepaliveInterval: 10}}}
	return s, c, w, r
}
func runSafeTestStream(s *GatewayService, c *gin.Context, r *AnthropicStreamRetryState, body string, first *firstTokenAttempt) (*streamingResult, error) {
	return s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": {"upstream-test"}}, Body: io.NopCloser(strings.NewReader(body))}, c, safeRetryAccount(740), time.Now(), "claude-test", r, first)
}
func requireSafeKind(t *testing.T, err error, kind string, replayable bool) {
	t.Helper()
	var failure *AnthropicStreamFailure
	require.ErrorAs(t, err, &failure)
	require.Equal(t, kind, failure.Kind)
	require.Equal(t, replayable, failure.Replayable)
}
func TestAnthropicStreamSafeRetryFrameClassification(t *testing.T) {
	cases := []struct {
		name, frame string
		kind        anthropicFrameKind
		bad         bool
	}{
		{"空起始", safeStart, anthropicFramePrelude, false}, {"注释", ": ping\n\n", anthropicFramePrelude, false},
		{"心跳", "event: ping\ndata: {\"type\":\"ping\"}\n\n", anthropicFramePrelude, false},
		{"空文本块", "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n", anthropicFramePrelude, false},
		{"正文", safeText, anthropicFrameContent, false},
		{"思考", "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"想法\"}}\n\n", anthropicFrameContent, false},
		{"工具起始", "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_1\"}}\n\n", anthropicFrameContent, false},
		{"未知事件", "data: {\"type\":\"future_event\"}\n\n", anthropicFrameContent, false},
		{"终止", safeStop, anthropicFrameTerminal, false}, {"DONE", "data: [DONE]\n\n", anthropicFrameTerminal, false},
		{"只有名称", "event: message_stop\n\n", 0, true}, {"无效JSON", "data: {bad}\n\n", 0, true},
		{"类型矛盾", "event: message_stop\ndata: {\"type\":\"ping\"}\n\n", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, _, _, err := inspectAnthropicFrame([]byte(tc.frame))
			if tc.bad {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.kind, k)
			}
		})
	}
}
func TestAnthropicStreamSafeRetryPreludeAndContentBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body, kind      string
		replayable, committed bool
	}{
		{"空流", "", "empty_stream", true, false}, {"前导EOF", safeStart, "missing_terminal", true, false},
		{"正文后EOF", safeStart + safeText, "missing_terminal", false, true},
		{"半个终止", safeStart + strings.TrimSuffix(safeStop, "\n"), "truncated_event", false, false},
		{"只有终止名称", safeStart + "event: message_stop\n", "truncated_event", false, false},
		{"错误事件", safeStart + "data: {\"type\":\"error\",\"error\":{\"message\":\"私密错误\"}}\n\n", "upstream_error_event", false, false},
		{"超限", strings.Repeat(": x\n\n", maxAnthropicStreamPreludeBytes/5+1), "prelude_overflow", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c, w, r := newSafeRetryTest(t)
			require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
			result, err := runSafeTestStream(s, c, r, tc.body, nil)
			requireSafeKind(t, err, tc.kind, tc.replayable)
			require.Equal(t, tc.committed, r.Committed())
			require.NotNil(t, result)
			if !tc.committed {
				require.Empty(t, w.Body.String())
				require.False(t, c.Writer.Written())
				require.Empty(t, c.Writer.Header().Get("X-Request-Id"))
			}
			require.NotContains(t, w.Body.String(), "message_stop")
		})
	}
}
func TestAnthropicStreamSafeRetryDoesNotWaitForEOF(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, w, r := newSafeRetryTest(t)
		defer r.Close()
		require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
		pr, pw := io.Pipe()
		defer func() { _ = pw.Close() }()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, c, safeRetryAccount(740), time.Now(), "claude", r, nil)
			done <- err
		}()
		_, err := io.WriteString(pw, safeStart+safeStop)
		require.NoError(t, err)
		synctest.Wait()
		select {
		case err := <-done:
			require.NoError(t, err)
		default:
			t.Fatal("完整终止事件后仍在等待EOF")
		}
		require.Equal(t, safeStart+safeStop, w.Body.String())
	})
}
func TestAnthropicStreamSafeRetryBudgetAndDispatchLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, _, _, r := newSafeRetryTest(t)
		defer r.Close()
		a := safeRetryAccount(740)
		require.NoError(t, r.beforeDispatch(a))
		time.Sleep(time.Hour)
		require.NoError(t, r.Check(), "首次响应头前不启动预算")
		r.startResponse()
		time.Sleep(180 * time.Second)
		fault := &AnthropicStreamFailure{Kind: "idle_timeout", Replayable: true}
		require.NoError(t, r.PrepareRetry(a, fault))
		require.NoError(t, r.beforeDispatch(a))
		require.Equal(t, 1, r.replays)
		require.NoError(t, r.PrepareRetry(a, fault))
		require.NoError(t, r.beforeDispatch(safeRetryAccount(741)))
		require.Equal(t, 2, r.replays)
		requireSafeKind(t, r.beforeDispatch(safeRetryAccount(742)), "retries_exhausted", false)
		time.Sleep(120 * time.Second)
		synctest.Wait()
		requireSafeKind(t, r.Check(), "budget_exhausted", false)
	})
}
func TestAnthropicStreamSafeRetryPreciseIdleAndTotalBudget(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		heartbeat bool
		want      string
		seconds   int
	}{
		{"空闲180秒", false, "idle_timeout", 180}, {"心跳不续总预算", true, "budget_exhausted", 300},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, c, w, r := newSafeRetryTest(t)
				defer r.Close()
				r.settings.AnthropicStreamSafeRetryFirstContentTimeoutSeconds = 0
				require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
				pr, pw := io.Pipe()
				defer func() { _ = pw.Close() }()
				completed := make(chan error, 1)
				go func() {
					_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, c, safeRetryAccount(740), time.Now(), "claude", r, nil)
					completed <- err
				}()
				_, err := io.WriteString(pw, safeStart)
				require.NoError(t, err)
				synctest.Wait()
				if scenario.heartbeat {
					for range 2 {
						time.Sleep(100 * time.Second)
						_, err = io.WriteString(pw, ": 保活\n\n")
						require.NoError(t, err)
						synctest.Wait()
					}
					time.Sleep(100 * time.Second)
				} else {
					time.Sleep(180 * time.Second)
				}
				synctest.Wait()
				select {
				case err := <-completed:
					requireSafeKind(t, err, scenario.want, !scenario.heartbeat)
				default:
					t.Fatal("到期未退出")
				}
				require.Empty(t, w.Body.String())
			})
		})
	}
}
func TestAnthropicStreamSafeRetryContentReleasesBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, w, r := newSafeRetryTest(t)
		defer r.Close()
		require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
		pr, pw := io.Pipe()
		defer func() { _ = pw.Close() }()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, c, safeRetryAccount(740), time.Now(), "claude", r, nil)
			done <- err
		}()
		_, err := io.WriteString(pw, safeStart+safeText)
		require.NoError(t, err)
		synctest.Wait()
		for range 4 {
			time.Sleep(100 * time.Second)
			_, err = io.WriteString(pw, ": 保活\n\n")
			require.NoError(t, err)
			synctest.Wait()
		}
		_, err = io.WriteString(pw, safeStop)
		require.NoError(t, err)
		synctest.Wait()
		require.NoError(t, <-done)
		require.Contains(t, w.Body.String(), "测试正文")
	})
}
func TestAnthropicStreamSafeRetryCancelAndNoReplay(t *testing.T) {
	s, c, _, r := newSafeRetryTest(t)
	a := safeRetryAccount(740)
	require.NoError(t, r.beforeDispatch(a))
	_, err := runSafeTestStream(s, c, r, safeStart, nil)
	require.NoError(t, r.PrepareRetry(a, err))
	a.Credentials["pool_mode_retry_count"] = 0
	requireSafeKind(t, r.beforeDispatch(a), "replay_forbidden", false)
	require.Error(t, r.PrepareRetry(a, &PluginTransportError{RequestSent: true, Message: "不可重放"}))
	ctx, cancel := context.WithCancel(context.Background())
	rr := newAnthropicStreamRetryState(ctx, DefaultGatewaySettings())
	defer rr.Close()
	cancel()
	requireSafeKind(t, rr.Check(), "client_canceled", false)
}
func TestAnthropicStreamSafeRetryFirstTokenGuard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, _, r := newSafeRetryTest(t)
		defer r.Close()
		a := safeRetryAccount(740)
		require.NoError(t, r.beforeDispatch(a))
		first := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, a, "claude", 45*time.Second)
		first.useExternalStreamGuard()
		first.start()
		require.Nil(t, first.bufferedWriter, "只能有一份前导缓存")
		pr, pw := io.Pipe()
		defer func() { _ = pw.Close() }()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, c, a, time.Now(), "claude", r, first)
			done <- first.finish(err)
		}()
		_, err := io.WriteString(pw, safeStart)
		require.NoError(t, err)
		synctest.Wait()
		time.Sleep(45 * time.Second)
		synctest.Wait()
		select {
		case err := <-done:
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.True(t, failover.FirstTokenTimeout)
		default:
			t.Fatal("首Token超时未退出")
		}
	})
}
func TestAnthropicStreamSafeRetrySettings(t *testing.T) {
	defaults := DefaultGatewaySettings()
	require.False(t, defaults.AnthropicStreamSafeRetryEnabled)
	require.Equal(t, 2, defaults.AnthropicStreamSafeRetryMaxRetries)
	require.Equal(t, 300, defaults.AnthropicStreamSafeRetryTotalWaitSeconds)
	require.Equal(t, 180, defaults.AnthropicStreamSafeRetryFirstContentTimeoutSeconds)
	for _, n := range []int{-1, 6} {
		bad := defaults
		bad.AnthropicStreamSafeRetryMaxRetries = n
		require.Error(t, validateGatewaySettings(&bad))
	}
	for _, n := range []int{0, 3601} {
		bad := defaults
		bad.AnthropicStreamSafeRetryTotalWaitSeconds = n
		require.Error(t, validateGatewaySettings(&bad))
	}
	for _, seconds := range []int{1, 600, 601, 3600} {
		settings := defaults
		settings.AnthropicStreamSafeRetryTotalWaitSeconds = seconds
		require.NoError(t, validateGatewaySettings(&settings))
		parsed := parseGatewaySettings(map[string]string{SettingKeyGatewayAnthropicStreamSafeRetryTotalWaitSeconds: fmt.Sprint(seconds)})
		require.Equal(t, seconds, parsed.AnthropicStreamSafeRetryTotalWaitSeconds)
	}
	for _, seconds := range []int{0, 1, 180, 3600} {
		settings := defaults
		settings.AnthropicStreamSafeRetryFirstContentTimeoutSeconds = seconds
		require.NoError(t, validateGatewaySettings(&settings))
		parsed := parseGatewaySettings(map[string]string{SettingKeyGatewayAnthropicStreamSafeRetryFirstContentTimeoutSeconds: fmt.Sprint(seconds)})
		require.Equal(t, seconds, parsed.AnthropicStreamSafeRetryFirstContentTimeoutSeconds)
	}
	for _, seconds := range []int{-1, 3601} {
		settings := defaults
		settings.AnthropicStreamSafeRetryFirstContentTimeoutSeconds = seconds
		require.Error(t, validateGatewaySettings(&settings))
	}
	settings := parseGatewaySettings(map[string]string{SettingKeyGatewayAnthropicStreamSafeRetryEnabled: "true", SettingKeyGatewayAnthropicStreamSafeRetryMaxRetries: "1", SettingKeyGatewayAnthropicStreamSafeRetryTotalWaitSeconds: "240"})
	require.True(t, settings.AnthropicStreamSafeRetryEnabled)
	require.Equal(t, 1, settings.AnthropicStreamSafeRetryMaxRetries)
	require.Equal(t, 240, settings.AnthropicStreamSafeRetryTotalWaitSeconds)
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	updated, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	loaded, err := svc.GetGatewaySettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, updated.AnthropicStreamSafeRetryMaxRetries, loaded.AnthropicStreamSafeRetryMaxRetries)
	require.Equal(t, updated.AnthropicStreamSafeRetryFirstContentTimeoutSeconds, loaded.AnthropicStreamSafeRetryFirstContentTimeoutSeconds)
	require.True(t, svc.GetGatewayRuntime(context.Background()).AnthropicStreamSafeRetryEnabled)
	require.True(t, errors.Is((&AnthropicStreamFailure{Cause: io.ErrUnexpectedEOF}).Unwrap(), io.ErrUnexpectedEOF))
}

func TestAnthropicStreamSafeRetryAccountOrderAndDisabledAccounts(t *testing.T) {
	for _, onlyOne := range []bool{false, true} {
		t.Run(map[bool]string{false: "先原号后换号", true: "单账号使用剩余次数"}[onlyOne], func(t *testing.T) {
			f := newLoadAwareRestrictionFixture(t, false, nil, nil)
			a := safeRetryAccount(740)
			b := safeRetryAccount(741)
			disabled := safeRetryAccount(742)
			disabled.Schedulable = false
			accounts := []Account{*a, *disabled}
			if !onlyOne {
				accounts = append(accounts, *b)
			}
			repo := &mockAccountRepoForPlatform{accounts: accounts, accountsByID: map[int64]*Account{a.ID: a, b.ID: b, disabled.ID: disabled}}
			f.svc.accountRepo = repo
			f.svc.channelService = nil
			settings := DefaultGatewaySettings()
			settings.AnthropicStreamSafeRetryEnabled = true
			r := newAnthropicStreamRetryState(f.ctx, settings)
			defer r.Close()
			require.NoError(t, r.beforeDispatch(a))
			r.startResponse()
			fault := &AnthropicStreamFailure{Kind: "missing_terminal", Replayable: true}
			require.NoError(t, r.PrepareRetry(a, fault))
			selected, err := f.svc.SelectAnthropicStreamRetryAccount(f.ctx, r, &f.groupID, "", "claude-sonnet-4-6", nil, "", 0, true)
			require.NoError(t, err)
			require.Equal(t, a.ID, selected.Account.ID)
			if selected.ReleaseFunc != nil {
				selected.ReleaseFunc()
			}
			require.NoError(t, r.beforeDispatch(selected.Account))
			require.NoError(t, r.PrepareRetry(selected.Account, fault))
			selected, err = f.svc.SelectAnthropicStreamRetryAccount(f.ctx, r, &f.groupID, "", "claude-sonnet-4-6", nil, "", 0, true)
			require.NoError(t, err)
			if onlyOne {
				require.Equal(t, a.ID, selected.Account.ID)
			} else {
				require.Equal(t, b.ID, selected.Account.ID)
			}
			if selected.ReleaseFunc != nil {
				selected.ReleaseFunc()
			}
			require.False(t, disabled.Schedulable)
		})
	}
}

func TestAnthropicStreamSafeRetryWriteFailureIsNotReplayable(t *testing.T) {
	s, c, _, r := newSafeRetryTest(t)
	require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
	c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
	result, err := runSafeTestStream(s, c, r, safeStart+safeText, nil)
	require.True(t, result.clientDisconnect)
	require.True(t, r.Committed())
	requireSafeKind(t, err, "missing_terminal", false)
	require.Error(t, r.PrepareRetry(safeRetryAccount(740), err))
}

func TestAnthropicStreamSafeRetryDiscardsFailedPrelude(t *testing.T) {
	s, c, w, r := newSafeRetryTest(t)
	a := safeRetryAccount(740)
	require.NoError(t, r.beforeDispatch(a))
	result, err := runSafeTestStream(s, c, r, safeStart, nil)
	require.Equal(t, 7, result.usage.InputTokens)
	require.Empty(t, w.Body.String())
	require.NoError(t, r.PrepareRetry(a, err))
	require.NoError(t, r.beforeDispatch(a))
	_, err = runSafeTestStream(s, c, r, safeStart+safeText+safeStop, nil)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(w.Body.String(), "event: message_start"))
	require.Equal(t, safeStart+safeText+safeStop, w.Body.String())
}

// safeBudgetUpstream 只使用内存管道与请求取消，不发起真实网络连接。
type safeBudgetUpstream struct {
	HTTPUpstream
	call func(*http.Request) (*http.Response, error)
}

func (u *safeBudgetUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.call(req)
}

func TestAnthropicStreamSafeRetryBudgetCancelsRetryResponseHeaders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, _, r := newSafeRetryTest(t)
		defer r.Close()
		r.settings.AnthropicStreamSafeRetryFirstContentTimeoutSeconds = 0
		settings := NewSettingService(&safeRetrySettingsRepo{customFeatureSettingsRepoStub{values: map[string]string{SettingKeyGatewayFirstTokenTimeoutSeconds: "0"}}}, s.cfg)
		s.rateLimitService = &RateLimitService{settingService: settings}
		pr, pw := io.Pipe()
		defer func() { _ = pw.Close() }()
		calls := 0
		s.httpUpstream = &safeBudgetUpstream{call: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				go func() { _, _ = io.WriteString(pw, safeStart) }()
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		}}
		start := time.Now()
		account := safeRetryAccount(740)
		body := []byte(`{"model":"claude-sonnet-4-5","stream":true,"messages":[{"role":"user","content":"测试"}]}`)
		partial, err := s.forwardAnthropicAPIKeyPassthrough(c.Request.Context(), c, account, body, "claude-sonnet-4-5", "claude-sonnet-4-5", true, start)
		requireSafeKind(t, err, "idle_timeout", true)
		require.NotNil(t, partial)
		require.Equal(t, 7, partial.Usage.InputTokens)
		require.Equal(t, 180*time.Second, time.Since(start))
		require.NoError(t, r.PrepareRetry(account, err))
		time.Sleep(500 * time.Millisecond)
		_, err = s.forwardAnthropicAPIKeyPassthrough(c.Request.Context(), c, account, body, "claude-sonnet-4-5", "claude-sonnet-4-5", true, start)
		requireSafeKind(t, err, "budget_exhausted", false)
		require.Equal(t, 300*time.Second, time.Since(start))
		require.Equal(t, 2, calls)
	})
}
func TestAnthropicStreamSafeRetrySnapshotAndZeroRetries(t *testing.T) {
	s, c, _, r := newSafeRetryTest(t)
	r.settings.AnthropicStreamSafeRetryMaxRetries = 0
	account := safeRetryAccount(740)
	require.NoError(t, r.beforeDispatch(account))
	_, err := runSafeTestStream(s, c, r, safeStart, nil)
	requireSafeKind(t, r.PrepareRetry(account, err), "retries_exhausted", false)
	require.Same(t, r, s.anthropicStreamRetry(c, context.Background()), "在途请求不能因设置修改而获得新预算")
}

type safeRetrySettingsRepo struct{ customFeatureSettingsRepoStub }

func (r *safeRetrySettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func TestAnthropicStreamSafeRetryScopedWhitespaceDoesNotDisableFirstTokenGuard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, _, r := newSafeRetryTest(t)
		defer r.Close()
		a := safeRetryAccount(740)
		require.NoError(t, r.beforeDispatch(a))
		first := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, a, "claude", 45*time.Second)
		first.strictOutput = true
		first.useExternalStreamGuard()
		first.start()
		pr, pw := io.Pipe()
		defer func() { _ = pw.Close() }()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, c, a, time.Now(), "claude", r, first)
			done <- first.finish(err)
		}()
		whitespace := strings.Replace(safeText, "测试正文", " ", 1)
		_, err := io.WriteString(pw, safeStart+whitespace)
		require.NoError(t, err)
		synctest.Wait()
		require.False(t, r.Committed(), "纯空白应保留在安全前导缓存中")
		require.Equal(t, firstTokenAttemptWaiting, first.currentState(), "指定分组纯空白不能冒充首有效输出")
		time.Sleep(45 * time.Second)
		synctest.Wait()
		select {
		case err := <-done:
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.True(t, failover.FirstTokenTimeout)
			require.NoError(t, r.PrepareRetry(a, err))
		default:
			t.Fatal("首Token超时未结束前导等待")
		}
	})
}
func TestAnthropicStreamSafeRetryClientCancellationClosesWaitingBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, w, old := newSafeRetryTest(t)
		old.Close()
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		settings := DefaultGatewaySettings()
		r := newAnthropicStreamRetryState(ctx, settings)
		defer r.Close()
		require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
		pr, pw := io.Pipe()
		defer func() { _ = pw.Close() }()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(ctx, &http.Response{StatusCode: 200, Header: make(http.Header), Body: pr}, c, safeRetryAccount(740), time.Now(), "claude", r, nil)
			done <- err
		}()
		_, err := io.WriteString(pw, safeStart)
		require.NoError(t, err)
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case err := <-done:
			requireSafeKind(t, err, "client_canceled", false)
		default:
			t.Fatal("客户端取消未关闭等待中的上游")
		}
		require.Empty(t, w.Body.String())
		require.Error(t, r.beforeDispatch(safeRetryAccount(740)))
	})
}
func TestAnthropicStreamSafeRetryUsageSnapshotCannotChangeAccount(t *testing.T) {
	_, _, _, r := newSafeRetryTest(t)
	require.NoError(t, r.beforeDispatch(safeRetryAccount(740)))
	r.observeAttemptUsage(&ForwardResult{Usage: ClaudeUsage{InputTokens: 7}})
	require.NotNil(t, r.AttemptUsageSnapshot(740))
	require.Nil(t, r.AttemptUsageSnapshot(741))
}
