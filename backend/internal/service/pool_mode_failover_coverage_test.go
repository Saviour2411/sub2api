package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type poolModeFailoverHTTPUpstream struct {
	statusCode int
	body       []byte
}

func (u *poolModeFailoverHTTPUpstream) response() *http.Response {
	return &http.Response{
		StatusCode: u.statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(u.body)),
	}
}

func (u *poolModeFailoverHTTPUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return u.response(), nil
}

func (u *poolModeFailoverHTTPUpstream) DoWithTLS(*http.Request, string, int64, int, *tlsfingerprint.Profile) (*http.Response, error) {
	return u.response(), nil
}

func newPoolModeFailoverAccount(statusCodes ...int) *Account {
	rawCodes := make([]any, 0, len(statusCodes))
	for _, statusCode := range statusCodes {
		rawCodes = append(rawCodes, statusCode)
	}
	return &Account{
		ID:          701,
		Name:        "pool-mode-failover",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                      "sk-upstream",
			"base_url":                     "https://api.anthropic.com",
			"pool_mode":                    true,
			"pool_mode_retry_count":        1,
			"pool_mode_retry_status_codes": rawCodes,
		},
	}
}

func newPoolModeFailoverGinContext(path string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c
}

func TestGatewayProtocolConversionsRespectAccountRetryStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamBody := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)

	tests := []struct {
		name string
		path string
		body []byte
		run  func(*GatewayService, *gin.Context, *Account, []byte) error
	}{
		{
			name: "Chat Completions 转 Anthropic",
			path: "/v1/chat/completions",
			body: []byte(`{"model":"claude-test","stream":false,"messages":[{"role":"user","content":"hello"}]}`),
			run: func(s *GatewayService, c *gin.Context, account *Account, body []byte) error {
				_, err := s.ForwardAsChatCompletions(context.Background(), c, account, body, nil)
				return err
			},
		},
		{
			name: "Responses 转 Anthropic",
			path: "/v1/responses",
			body: []byte(`{"model":"claude-test","stream":false,"input":[{"role":"user","content":"hello"}]}`),
			run: func(s *GatewayService, c *gin.Context, account *Account, body []byte) error {
				_, err := s.ForwardAsResponses(context.Background(), c, account, body, nil)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &poolModeFailoverHTTPUpstream{statusCode: http.StatusServiceUnavailable, body: upstreamBody}
			svc := &GatewayService{
				cfg:          &config.Config{},
				httpUpstream: upstream,
			}
			account := newPoolModeFailoverAccount(http.StatusServiceUnavailable)
			err := tt.run(svc, newPoolModeFailoverGinContext(tt.path), account, tt.body)

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
			require.True(t, failoverErr.RetryableOnSameAccount)
		})
	}
}

func TestOpenAIFailoverConstructorsRespectAccountRetryStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := newPoolModeFailoverAccount(http.StatusTooManyRequests, http.StatusBadGateway)
	account.Platform = PlatformOpenAI
	svc := &OpenAIGatewayService{}

	t.Run("HTTP 透传状态码", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"busy"}}`))),
		}
		err := svc.handleFailoverErrorResponsePassthrough(context.Background(), resp, newPoolModeFailoverGinContext("/v1/responses"), account, []byte(`{"model":"gpt-test"}`))
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.True(t, failoverErr.RetryableOnSameAccount)
	})

	t.Run("流内错误", func(t *testing.T) {
		err := svc.newOpenAIStreamFailoverError(newPoolModeFailoverGinContext("/v1/responses"), account, false, "", nil, "stream failed")
		require.True(t, err.RetryableOnSameAccount)
	})

	t.Run("瞬时传输错误", func(t *testing.T) {
		err := svc.handleOpenAIUpstreamTransportError(context.Background(), newPoolModeFailoverGinContext("/v1/responses"), account, context.DeadlineExceeded, false)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.True(t, failoverErr.RetryableOnSameAccount)
	})

	t.Run("持久传输错误强制切号", func(t *testing.T) {
		err := svc.handleOpenAIUpstreamTransportError(context.Background(), newPoolModeFailoverGinContext("/v1/responses"), account, errors.New("proxy username/password authentication failed"), false)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.False(t, failoverErr.RetryableOnSameAccount)
	})
}
