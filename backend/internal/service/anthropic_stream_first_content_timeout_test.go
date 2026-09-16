//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamSafeRetryFirstContentTimeout(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		frame string
	}{
		{"心跳", ": ping\n\n"},
		{"协议心跳", "data: {\"type\":\"ping\"}\n\n"},
		{"空前导", "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"},
		{"空白正文", "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" \\n\\t\"}}\n\n"},
		{"空白思考", "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\" \"}}\n\n"},
		{"空白块", "data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"thinking\",\"thinking\":\" \"}}\n\ndata: {\"type\":\"content_block_stop\"}\n\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				gateway, ginContext, writer, retry := newSafeRetryTest(t)
				require.NoError(t, retry.beforeDispatch(safeRetryAccount(740)))
				reader, upstream := io.Pipe()
				defer upstream.Close()
				completed := make(chan error, 1)
				go func() {
					_, err := gateway.handleGuardedAnthropicStream(ginContext.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, ginContext, safeRetryAccount(740), time.Now(), "claude", retry, nil)
					completed <- err
				}()
				_, err := io.WriteString(upstream, safeStart)
				require.NoError(t, err)
				for range 3 {
					time.Sleep(50 * time.Second)
					_, err = io.WriteString(upstream, scenario.frame)
					require.NoError(t, err)
					synctest.Wait()
				}
				time.Sleep(30 * time.Second)
				synctest.Wait()
				requireSafeKind(t, <-completed, "first_content_timeout", true)
				require.Empty(t, writer.Body.String())
				require.False(t, retry.Committed())
				require.Equal(t, 120*time.Second, retry.remainingLocked())
				require.Equal(t, http.StatusGatewayTimeout, retry.diagnostics[0].LogicalStatus)
				_, err = upstream.Write([]byte("closed"))
				require.Error(t, err)
			})
		})
	}
}

func TestAnthropicStreamSafeRetryMeaningfulContentStopsTimeout(t *testing.T) {
	for _, content := range []string{
		safeText,
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"思考\"}}\n\n",
		"data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_1\",\"name\":\"lookup\",\"input\":{}}}\n\n",
		"data: {\"type\":\"content_block_start\",\"content_block\":{\"type\":\"text\",\"text\":\"正文\"}}\n\n",
		"data: {\"type\":\"message_start\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"正文\"}]}}\n\n",
	} {
		synctest.Test(t, func(t *testing.T) {
			gateway, ginContext, writer, retry := newSafeRetryTest(t)
			require.NoError(t, retry.beforeDispatch(safeRetryAccount(740)))
			reader, upstream := io.Pipe()
			defer upstream.Close()
			completed := make(chan error, 1)
			go func() {
				_, err := gateway.handleGuardedAnthropicStream(ginContext.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, ginContext, safeRetryAccount(740), time.Now(), "claude", retry, nil)
				completed <- err
			}()
			whitespace := "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" \"}}\n\n"
			_, err := io.WriteString(upstream, safeStart+whitespace)
			require.NoError(t, err)
			synctest.Wait()
			require.Empty(t, writer.Body.String())
			time.Sleep(100 * time.Second)
			_, err = io.WriteString(upstream, content)
			require.NoError(t, err)
			synctest.Wait()
			for range 3 {
				time.Sleep(100 * time.Second)
				_, err = io.WriteString(upstream, ": ping\n\n")
				require.NoError(t, err)
			}
			_, err = io.WriteString(upstream, safeStop)
			require.NoError(t, err)
			synctest.Wait()
			require.NoError(t, <-completed)
			require.Contains(t, writer.Body.String(), safeStart+whitespace+content)
			require.Equal(t, 1, strings.Count(writer.Body.String(), "event: message_start"))
		})
	}
}

func TestAnthropicStreamSafeRetryFirstContentBudgetAndUnknownFrame(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		budget  int
		unknown bool
		want    string
	}{
		{"总预算优先", 180, false, "budget_exhausted"},
		{"已提交未知帧不能重放", 300, true, "first_content_timeout"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				gateway, ginContext, writer, retry := newSafeRetryTest(t)
				retry.settings.AnthropicStreamSafeRetryTotalWaitSeconds = scenario.budget
				require.NoError(t, retry.beforeDispatch(safeRetryAccount(740)))
				reader, upstream := io.Pipe()
				defer upstream.Close()
				completed := make(chan error, 1)
				go func() {
					_, err := gateway.handleGuardedAnthropicStream(ginContext.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, ginContext, safeRetryAccount(740), time.Now(), "claude", retry, nil)
					completed <- err
				}()
				prelude := safeStart
				if scenario.unknown {
					prelude += "data: {\"type\":\"future_event\"}\n\n"
				}
				_, err := io.WriteString(upstream, prelude)
				require.NoError(t, err)
				synctest.Wait()
				time.Sleep(100 * time.Second)
				_, err = io.WriteString(upstream, ": ping\n\n")
				require.NoError(t, err)
				time.Sleep(80 * time.Second)
				synctest.Wait()
				requireSafeKind(t, <-completed, scenario.want, false)
				require.Equal(t, scenario.unknown, retry.Committed())
				if !scenario.unknown {
					require.Empty(t, writer.Body.String())
				}
			})
		})
	}
}

func TestAnthropicStreamSafeRetryFirstContentResetsButBudgetDoesNot(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		budget     int
		secondWait time.Duration
		want       string
	}{
		{"共享300秒预算", 300, 120 * time.Second, "budget_exhausted"},
		{"每轮重新等待180秒", 3600, 180 * time.Second, "first_content_timeout"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				gateway, ginContext, writer, retry := newSafeRetryTest(t)
				retry.settings.AnthropicStreamSafeRetryTotalWaitSeconds = scenario.budget
				account := safeRetryAccount(740)
				waitForAttempt := func(duration time.Duration, headerDelay time.Duration) error {
					require.NoError(t, retry.beforeDispatch(account))
					time.Sleep(headerDelay)
					reader, upstream := io.Pipe()
					defer upstream.Close()
					completed := make(chan error, 1)
					go func() {
						_, err := gateway.handleGuardedAnthropicStream(ginContext.Request.Context(), &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, ginContext, account, time.Now(), "claude", retry, nil)
						completed <- err
					}()
					_, err := io.WriteString(upstream, safeStart)
					require.NoError(t, err)
					synctest.Wait()
					for elapsed := 30 * time.Second; elapsed < duration; elapsed += 30 * time.Second {
						time.Sleep(30 * time.Second)
						_, err = io.WriteString(upstream, ": heartbeat\n\n")
						require.NoError(t, err)
						synctest.Wait()
					}
					time.Sleep(30 * time.Second)
					synctest.Wait()
					return <-completed
				}
				started := time.Now()
				firstErr := waitForAttempt(180*time.Second, 90*time.Second)
				require.Equal(t, 270*time.Second, time.Since(started))
				requireSafeKind(t, firstErr, "first_content_timeout", true)
				require.NoError(t, retry.PrepareRetry(account, firstErr))
				secondErr := waitForAttempt(scenario.secondWait, 0)
				requireSafeKind(t, secondErr, scenario.want, scenario.want == "first_content_timeout")
				require.Equal(t, 2, retry.diagnostics[1].Attempt)
				require.Equal(t, 1, retry.diagnostics[1].RetriesRemaining)
				require.Empty(t, writer.Body.String())
			})
		})
	}
}
