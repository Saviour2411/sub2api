package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemVersionReturnsBuildVersion(t *testing.T) {
	for _, version := range []string{"0.1.241", "dev", "0.1.242-custom"} {
		t.Run(version, func(t *testing.T) {
			h := NewSystemHandler(version)
			router := gin.New()
			router.GET("/version", h.GetVersion)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/version", nil))
			require.Equal(t, http.StatusOK, w.Code)
			require.JSONEq(t, `{"code":0,"message":"success","data":{"version":"`+version+`"}}`, w.Body.String())
		})
	}
}
