//go:build unit && embed

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/web"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildVersionInjectedIntoFirstHTML(t *testing.T) {
	const version = "0.1.244-custom.build"
	settings, _, _ := versionTestHandlers(t, version)
	frontend, err := web.NewFrontendServer(settings)
	require.NoError(t, err)
	router := gin.New()
	router.Use(frontend.Middleware())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	match := regexp.MustCompile(`window\.__APP_CONFIG__=(.*?);</script>`).FindStringSubmatch(w.Body.String())
	require.Len(t, match, 2)
	var injected map[string]any
	require.NoError(t, json.Unmarshal([]byte(match[1]), &injected))
	require.Equal(t, version, injected["version"])
	require.NotContains(t, injected, "upstream_version")
}
