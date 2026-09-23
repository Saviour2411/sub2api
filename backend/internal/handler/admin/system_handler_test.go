package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/buildmeta"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemVersionReturnsBuildVersion(t *testing.T) {
	for _, version := range []string{"0.1.241", "dev", "0.1.242-custom"} {
		t.Run(version, func(t *testing.T) {
			upstream, err := buildmeta.EmbeddedUpstreamSync()
			require.NoError(t, err)
			h := NewSystemHandler(version, upstream, nil)
			router := gin.New()
			router.GET("/version", h.GetVersion)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/version", nil))
			require.Equal(t, http.StatusOK, w.Code)
			var payload struct {
				Data map[string]string `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
			require.Equal(t, version, payload.Data["version"])
			require.Equal(t, upstream.Version, payload.Data["upstream_version"])
			require.Equal(t, upstream.Commit, payload.Data["upstream_commit"])
		})
	}
}
