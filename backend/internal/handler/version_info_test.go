//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/buildmeta"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func versionTestHandlers(t *testing.T, version string) (*service.SettingService, *SettingHandler, *admin.SystemHandler) {
	t.Helper()
	upstream, err := buildmeta.EmbeddedUpstreamSync()
	require.NoError(t, err)
	info := BuildInfo{Version: version, Upstream: upstream}
	settings := service.NewSettingService(&settingHandlerPublicRepoStub{}, &config.Config{})
	return settings, ProvideSettingHandler(settings, info, nil), ProvideSystemHandler(info, nil)
}

func TestBuildVersionConsistentAcrossPublicAndAdmin(t *testing.T) {
	for _, version := range []string{"0.1.244", "dev", "0.1.244-custom.abcdef"} {
		t.Run(version, func(t *testing.T) {
			settings, public, system := versionTestHandlers(t, version)
			injected, err := settings.GetPublicSettingsForInjection(context.Background())
			require.NoError(t, err)
			encoded, err := json.Marshal(injected)
			require.NoError(t, err)
			var config map[string]any
			require.NoError(t, json.Unmarshal(encoded, &config))
			require.Equal(t, version, config["version"])

			router := gin.New()
			router.GET("/settings/public", public.GetPublicSettings)
			router.GET("/admin/system/version", system.GetVersion)
			for _, path := range []string{"/settings/public", "/admin/system/version"} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusOK, w.Code)
				var payload struct {
					Data map[string]any `json:"data"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
				require.Equal(t, config["version"], payload.Data["version"])
				if path == "/settings/public" {
					require.NotContains(t, payload.Data, "upstream_version")
				}
			}
		})
	}
}
