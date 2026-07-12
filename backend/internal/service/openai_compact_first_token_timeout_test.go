package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type compactFirstTokenTestUpstream struct {
	do       func(*http.Request) (*http.Response, error)
	lastBody []byte
}

func (u *compactFirstTokenTestUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.lastBody = append([]byte(nil), body...)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return u.do(req)
}

func (u *compactFirstTokenTestUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func newCompactFirstTokenTestService(upstream HTTPUpstream) *OpenAIGatewayService {
	settingService := &SettingService{}
	settings := DefaultGatewaySettings()
	settings.FirstTokenTimeoutSeconds = 1
	settingService.storeGatewaySettingsCache(settings, time.Hour)

	rateLimitService := NewRateLimitService(nil, nil, nil, nil, nil)
	rateLimitService.settingService = settingService
	return &OpenAIGatewayService{
		httpUpstream:     upstream,
		rateLimitService: rateLimitService,
	}
}

func newCompactFirstTokenTestContext(t *testing.T, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	MarkOpenAICompactClientStream(c)
	stopKeepalive := StartOpenAICompactSSEKeepalivePaused(c, 10*time.Millisecond)
	t.Cleanup(stopKeepalive)
	return c, recorder
}

func compactFirstTokenTestAccount() *Account {
	return &Account{
		ID:          901,
		Name:        "compact-first-token",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-account",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func TestOpenAIGatewayForwardBodySignalCompactTimesOutBeforeResponseHeaders(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","instructions":"compact-test","input":[{"type":"compaction_trigger"}]}`)
	upstream := &compactFirstTokenTestUpstream{
		do: func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		},
	}
	svc := newCompactFirstTokenTestService(upstream)
	c, recorder := newCompactFirstTokenTestContext(t, body)

	startedAt := time.Now()
	result, err := svc.Forward(c.Request.Context(), c, compactFirstTokenTestAccount(), body)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
	require.True(t, failoverErr.FirstTokenTimeout)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Less(t, time.Since(startedAt), 2500*time.Millisecond)
	require.False(t, c.Writer.Written(), "首 Token 超时前不能由 compact keepalive 提交 200")
	require.Empty(t, recorder.Body.String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Exists(), "compact 上游仍应使用 unary body")
}

func TestOpenAIGatewayForwardBodySignalCompactStopsWatchdogOnMeaningfulDelta(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","instructions":"compact-test","input":[{"type":"compaction_trigger"}]}`)
	upstream := &compactFirstTokenTestUpstream{
		do: func(req *http.Request) (*http.Response, error) {
			reader, writer := io.Pipe()
			go func() {
				defer writer.Close()
				select {
				case <-req.Context().Done():
					_ = writer.CloseWithError(req.Context().Err())
					return
				case <-time.After(50 * time.Millisecond):
				}
				_, _ = io.WriteString(writer, strings.Join([]string{
					`event: response.created`,
					`data: {"type":"response.created","response":{"id":"resp_compact_ft","status":"in_progress"}}`,
					``,
					`event: response.output_text.delta`,
					`data: {"type":"response.output_text.delta","delta":"ok"}`,
					``,
				}, "\n")+"\n")
				select {
				case <-req.Context().Done():
					_ = writer.CloseWithError(req.Context().Err())
					return
				case <-time.After(1100 * time.Millisecond):
				}
				_, _ = io.WriteString(writer, strings.Join([]string{
					`event: response.output_item.done`,
					`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"cmp_ft","type":"compaction","status":"completed","encrypted_content":"compact-payload"}}`,
					``,
					`event: response.completed`,
					`data: {"type":"response.completed","response":{"id":"resp_compact_ft","object":"response","status":"completed","output":[{"id":"cmp_ft","type":"compaction","status":"completed","encrypted_content":"compact-payload"}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`,
					``,
				}, "\n")+"\n")
			}()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       reader,
			}, nil
		},
	}
	svc := newCompactFirstTokenTestService(upstream)
	c, recorder := newCompactFirstTokenTestContext(t, body)

	startedAt := time.Now()
	result, err := svc.Forward(c.Request.Context(), c, compactFirstTokenTestAccount(), body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream, "usage 与 Ops 应保留客户端 SSE 语义")
	require.NotNil(t, result.FirstTokenMs)
	require.Less(t, *result.FirstTokenMs, 1000)
	require.GreaterOrEqual(t, time.Since(startedAt), time.Second, "终态晚于超时阈值，用于证明有效增量已停止 watchdog")
	require.NotContains(t, recorder.Body.String(), ": keepalive", "首 Token 前 compact keepalive 必须保持暂停")
	require.Contains(t, recorder.Body.String(), "event: response.completed")
	require.Contains(t, recorder.Body.String(), "compact-payload")
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Exists(), "compact 上游仍应使用 unary body")
}
