//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBalanceQueryPageNeverUsesCachedSettings(t *testing.T) {
	provider := &mockSettingsProvider{}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	server.cache.Set([]byte("不可复用的注入页面"), []byte(`{}`))
	router := gin.New()
	router.Use(server.Middleware())
	for _, path := range []string{"/balance-query", "/balance-query/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("If-None-Match", server.cache.Get().ETag)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "no-store, private", recorder.Header().Get("Cache-Control"))
		require.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
		require.Contains(t, recorder.Header().Get("X-Robots-Tag"), "noindex")
		require.Empty(t, recorder.Header().Get("ETag"))
		require.NotContains(t, recorder.Body.String(), "不可复用")
		require.Zero(t, provider.called)
	}
}
