//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamSafeRetryTerminalCancelsUpstream(tester *testing.T) {
	for _, safeRetry := range []bool{false, true} {
		for _, firstToken := range []int{0, 45} {
			tester.Run(fmt.Sprintf("safe=%v/first=%d", safeRetry, firstToken), func(tester *testing.T) {
				synctest.Test(tester, func(tester *testing.T) {
					gateway, ginContext, writer, retry := newSafeRetryTest(tester)
					defer retry.Close()
					if !safeRetry {
						delete(ginContext.Keys, anthropicStreamRetryContextKey)
					}
					settings := NewSettingService(&safeRetrySettingsRepo{customFeatureSettingsRepoStub{values: map[string]string{
						SettingKeyGatewayFirstTokenTimeoutSeconds:        strconv.Itoa(firstToken),
						SettingKeyGatewayAnthropicStreamSafeRetryEnabled: strconv.FormatBool(safeRetry),
					}}}, gateway.cfg)
					gateway.rateLimitService = &RateLimitService{settingService: settings}
					payload := safeStart + safeText + "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":3}}\n\n" + safeStop
					var upstreamBody *cleanupContextBody
					calls := 0
					release := make(chan struct{})
					defer close(release)
					gateway.httpUpstream = &safeBudgetUpstream{call: func(request *http.Request) (*http.Response, error) {
						calls++
						bodyCtx, cancel := context.WithCancel(request.Context())
						go func() {
							select {
							case <-release:
							case <-bodyCtx.Done():
							}
							cancel()
						}()
						upstreamBody = &cleanupContextBody{ctx: bodyCtx, reader: strings.NewReader(payload), waitAfterPayload: true}
						return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: upstreamBody}, nil
					}}
					done := make(chan struct{})
					var result *ForwardResult
					var forwardErr error
					go func() {
						defer close(done)
						result, forwardErr = gateway.forwardAnthropicAPIKeyPassthrough(ginContext.Request.Context(), ginContext, safeRetryAccount(740), []byte(`{"model":"claude-sonnet-4-5","stream":true}`), "claude-sonnet-4-5", "claude-sonnet-4-5", true, time.Now())
					}()
					synctest.Wait()
					select {
					case <-done:
					default:
						tester.Fatal("完整终止事件后，转发收尾仍然等待上游关闭")
					}
					require.NoError(tester, forwardErr)
					require.NotNil(tester, result)
					require.Equal(tester, 7, result.Usage.InputTokens)
					require.Equal(tester, 3, result.Usage.OutputTokens)
					require.Equal(tester, payload, writer.Body.String())
					require.Equal(tester, 1, calls)
					require.EqualValues(tester, 1, upstreamBody.closeCalls.Load())
					require.ErrorIs(tester, upstreamBody.ctx.Err(), context.Canceled)
					require.NoError(tester, ginContext.Request.Context().Err())
					if safeRetry {
						require.NoError(tester, retry.WaitContext().Err(), "本轮关闭不能取消共享预算")
					}
				})
			})
		}
	}
}
