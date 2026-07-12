package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIManualEndpointsReturnTransportFailoverError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	transportErr := errors.New("upstream dial timeout")
	account := &Account{
		ID:          701,
		Name:        "manual-failover",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                      "sk-test",
			"base_url":                     "https://api.openai.com",
			"pool_mode":                    true,
			"pool_mode_retry_count":        1,
			"pool_mode_retry_status_codes": []any{float64(http.StatusBadGateway)},
		},
	}

	t.Run("Images", func(t *testing.T) {
		body := []byte(`{"model":"gpt-image-1","prompt":"test"}`)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{err: transportErr}}
		parsed, err := svc.ParseOpenAIImagesRequest(c, body)
		require.NoError(t, err)

		result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
		require.Nil(t, result)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
		require.True(t, failoverErr.RetryableOnSameAccount)
		require.False(t, c.Writer.Written())
	})

	t.Run("Embedding", func(t *testing.T) {
		body := []byte(`{"model":"text-embedding-3-small","input":"test"}`)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{err: transportErr}}

		result, err := svc.ForwardEmbeddings(context.Background(), c, account, body, "")
		require.Nil(t, result)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
		require.True(t, failoverErr.RetryableOnSameAccount)
		require.False(t, c.Writer.Written())
	})
}
