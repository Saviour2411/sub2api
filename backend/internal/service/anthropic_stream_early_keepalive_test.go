//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamSafeRetryEarlyKeepalivePreservesFirstTokenTiming(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			s, c, w, retry := newSafeRetryTest(t)
			defer retry.Close()
			retry.settings.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = enabled
			account := safeRetryAccount(740)
			require.NoError(t, retry.beforeDispatch(account))
			reader, writer := io.Pipe()
			defer writer.Close()
			started := time.Now()
			var result *streamingResult
			var streamErr error
			done := make(chan struct{})
			go func() {
				defer close(done)
				result, streamErr = s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": {"upstream-private"}}, Body: reader}, c, account, started, "claude", retry, nil)
			}()
			synctest.Wait()
			require.Equal(t, enabled, c.Writer.Written())
			require.False(t, retry.Committed())
			if enabled {
				require.Equal(t, anthropicStreamKeepaliveFrame, w.Body.String())
				require.Empty(t, w.Result().Header.Get("X-Request-Id"))
			}
			time.Sleep(3 * time.Second)
			_, err := io.WriteString(writer, safeStart)
			require.NoError(t, err)
			synctest.Wait()
			time.Sleep(56 * time.Second)
			synctest.Wait()
			require.NotContains(t, w.Body.String(), "message_start")
			require.False(t, retry.Committed())
			_, err = io.WriteString(writer, safeText+safeStop)
			require.NoError(t, err)
			<-done
			require.NoError(t, streamErr)
			require.NotNil(t, result.firstTokenMs)
			require.Equal(t, 3000, *result.firstTokenMs, "保持原有上游首事件计时，不能改成59秒正文计时或本地保活时间")
			require.Equal(t, safeStart+safeText+safeStop, strings.ReplaceAll(w.Body.String(), anthropicStreamKeepaliveFrame, ""))
			require.True(t, retry.Committed())
		})
	}
}

func TestAnthropicStreamSafeRetryEarlyKeepaliveDiscardsFailedPrelude(t *testing.T) {
	s, c, w, retry := newSafeRetryTest(t)
	retry.settings.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = true
	account := safeRetryAccount(740)
	require.NoError(t, retry.beforeDispatch(account))
	result, err := runSafeTestStream(s, c, retry, strings.Replace(safeStart, "7", "99", 1), nil)
	requireSafeKind(t, err, "missing_terminal", true)
	require.Equal(t, 99, result.usage.InputTokens)
	require.Equal(t, anthropicStreamKeepaliveFrame, w.Body.String())
	require.True(t, retry.EarlyKeepaliveSent())
	require.False(t, retry.Committed())
	require.NoError(t, retry.PrepareRetry(account, err))
	require.NoError(t, retry.beforeDispatch(safeRetryAccount(741)))
	_, err = runSafeTestStream(s, c, retry, safeStart+safeText+safeStop, nil)
	require.NoError(t, err)
	require.Equal(t, anthropicStreamKeepaliveFrame+safeStart+safeText+safeStop, w.Body.String())
	require.NotContains(t, w.Body.String(), "99")
	require.Equal(t, "text/event-stream", w.Result().Header.Get("Content-Type"))
}

func TestAnthropicStreamSafeRetryEarlyKeepaliveDoesNotExtendTimeouts(t *testing.T) {
	for _, scenario := range []struct {
		name                         string
		content, idle, budget, first int
		seconds                      int
		kind                         string
	}{
		{"内容超时", 25, 180, 300, 0, 25, "first_content_timeout"},
		{"空闲超时", 0, 25, 300, 0, 25, "idle_timeout"},
		{"总预算", 0, 180, 25, 0, 25, "budget_exhausted"},
		{"首Token守卫", 180, 180, 300, 25, 25, "first_token_timeout"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, c, w, retry := newSafeRetryTest(t)
				defer retry.Close()
				retry.settings.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = true
				retry.settings.AnthropicStreamSafeRetryFirstContentTimeoutSeconds = scenario.content
				retry.settings.AnthropicStreamSafeRetryTotalWaitSeconds = scenario.budget
				s.cfg.Gateway.StreamDataIntervalTimeout = scenario.idle
				s.cfg.Gateway.StreamKeepaliveInterval = 0
				account := safeRetryAccount(740)
				require.NoError(t, retry.beforeDispatch(account))
				var first *firstTokenAttempt
				if scenario.first > 0 {
					first = newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, account, "claude", time.Duration(scenario.first)*time.Second)
					first.useExternalStreamGuard()
					first.start()
				}
				reader, writer := io.Pipe()
				defer writer.Close()
				started := time.Now()
				_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, c, account, started, "claude", retry, first)
				if first != nil {
					var failover *UpstreamFailoverError
					require.ErrorAs(t, first.finish(err), &failover)
					require.True(t, failover.FirstTokenTimeout)
				} else {
					requireSafeKind(t, err, scenario.kind, scenario.kind != "budget_exhausted")
				}
				require.Equal(t, time.Duration(scenario.seconds)*time.Second, time.Since(started))
				require.GreaterOrEqual(t, strings.Count(w.Body.String(), anthropicStreamKeepaliveFrame), 3)
				require.NotContains(t, w.Body.String(), "message_start")
				require.False(t, retry.Committed())
			})
		})
	}
}

func TestAnthropicStreamSafeRetryEarlyKeepaliveWriteFailureNeverRetries(t *testing.T) {
	s, c, _, retry := newSafeRetryTest(t)
	retry.settings.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = true
	account := safeRetryAccount(740)
	require.NoError(t, retry.beforeDispatch(account))
	c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
	result, err := runSafeTestStream(s, c, retry, safeStart, nil)
	requireSafeKind(t, err, "client_write_error", false)
	require.True(t, result.clientDisconnect)
	require.Error(t, retry.PrepareRetry(account, err))
}

func TestAnthropicStreamSafeRetryEarlyKeepaliveCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, w, initial := newSafeRetryTest(t)
		initial.Close()
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		settings := DefaultGatewaySettings()
		settings.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = true
		retry := newAnthropicStreamRetryState(ctx, settings)
		defer retry.Close()
		account := safeRetryAccount(740)
		require.NoError(t, retry.beforeDispatch(account))
		reader, writer := io.Pipe()
		defer writer.Close()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(ctx, &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, c, account, time.Now(), "claude", retry, nil)
			done <- err
		}()
		synctest.Wait()
		require.Equal(t, anthropicStreamKeepaliveFrame, w.Body.String())
		cancel()
		synctest.Wait()
		err := <-done
		requireSafeKind(t, err, "client_canceled", false)
		require.Error(t, retry.PrepareRetry(account, err))
		require.False(t, retry.Committed())
	})
}

func TestAnthropicStreamSafeRetryEarlyKeepalivePartialFrameAndContentBoundary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, c, w, retry := newSafeRetryTest(t)
		defer retry.Close()
		retry.settings.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = true
		account := safeRetryAccount(740)
		require.NoError(t, retry.beforeDispatch(account))
		reader, writer := io.Pipe()
		defer writer.Close()
		done := make(chan error, 1)
		go func() {
			_, err := s.handleGuardedAnthropicStream(c.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, c, account, time.Now(), "claude", retry, nil)
			done <- err
		}()
		_, err := io.WriteString(writer, "event: message_start\n")
		require.NoError(t, err)
		time.Sleep(11 * time.Second)
		synctest.Wait()
		require.Equal(t, strings.Repeat(anthropicStreamKeepaliveFrame, 2), w.Body.String())
		_, err = io.WriteString(writer, strings.TrimPrefix(safeStart, "event: message_start\n")+safeText)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		synctest.Wait()
		err = <-done
		requireSafeKind(t, err, "missing_terminal", false)
		require.True(t, retry.Committed())
		require.Error(t, retry.PrepareRetry(account, err))
		require.Equal(t, safeStart+safeText, strings.ReplaceAll(w.Body.String(), anthropicStreamKeepaliveFrame, ""))
	})
}

func TestAnthropicStreamSafeRetryEarlyKeepaliveSettings(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	settings := NewSettingService(repo, &config.Config{})
	ctx := context.Background()
	require.False(t, settings.GetGatewayRuntime(ctx).AnthropicStreamSafeRetryEarlyKeepaliveEnabled)
	input := DefaultGatewaySettings()
	input.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = true
	_, err := settings.UpdateGatewaySettings(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyGatewayAnthropicStreamSafeRetryEarlyKeepaliveEnabled])
	require.True(t, settings.GetGatewayRuntime(ctx).AnthropicStreamSafeRetryEarlyKeepaliveEnabled)
	all, err := settings.GetCustomFeatureSettings(ctx)
	require.NoError(t, err)
	require.True(t, all.Gateway.AnthropicStreamSafeRetryEarlyKeepaliveEnabled)
	input.AnthropicStreamSafeRetryEarlyKeepaliveEnabled = false
	_, err = settings.UpdateGatewaySettings(ctx, input)
	require.NoError(t, err)
	require.False(t, settings.GetGatewayRuntime(ctx).AnthropicStreamSafeRetryEarlyKeepaliveEnabled)
}
