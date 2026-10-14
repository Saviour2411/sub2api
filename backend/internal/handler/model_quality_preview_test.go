package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelQualityPreviewIsolation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		handler    gin.HandlerFunc
		directives []string
	}{{"preview", ModelQualityPreview, []string{"frame-src 'self'", "sandbox allow-scripts", "script-src 'sha256-"}}, {"artwork", ModelQualityArtwork, []string{"default-src 'none'", "connect-src 'none'", "worker-src 'none'", "frame-src 'none'", "sandbox allow-scripts"}}} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", tc.handler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, 200, w.Code)
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.Empty(t, w.Header().Get("X-Frame-Options"))
			for _, directive := range tc.directives {
				require.Contains(t, w.Header().Get("Content-Security-Policy"), directive)
			}
			require.NotContains(t, w.Header().Get("Content-Security-Policy"), "allow-same-origin")
			require.Contains(t, w.Body.String(), "e.source!==parent")
			// 浏览器验收使用真实处理器的响应，避免模拟壳与正式 CSP 发生偏差。
			if out := os.Getenv("MODEL_QUALITY_PREVIEW_FIXTURES"); out != "" {
				require.NoError(t, os.MkdirAll(out, 0755))
				require.NoError(t, os.WriteFile(filepath.Join(out, tc.name+".html"), w.Body.Bytes(), 0644))
				raw, e := json.Marshal(w.Header())
				require.NoError(t, e)
				require.NoError(t, os.WriteFile(filepath.Join(out, tc.name+"-headers.json"), raw, 0644))
			}
		})
	}
}
func TestModelQualityPublicResultHasNoCredentials(t *testing.T) {
	raw, err := json.Marshal(service.ModelQualityRun{Tests: []service.ModelQualityTest{{Benchmark: "pelican", Status: "completed", Verdict: "normal", HTML: "<canvas></canvas>"}}})
	require.NoError(t, err)
	for _, private := range []string{"key_id", "api_key", "owner_id", "endpoint", "remote_id", "idempotency_key"} {
		require.False(t, strings.Contains(string(raw), private))
	}
}
