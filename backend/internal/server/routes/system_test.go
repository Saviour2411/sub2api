package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemRoutesOnlyExposeReadOnlyVersion(t *testing.T) {
	router := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{System: adminhandler.NewSystemHandler("0.1.241")}}
	registerSystemRoutes(router.Group("/api/v1/admin"), h)
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, "check-updates"},
		{http.MethodGet, "rollback-versions"},
		{http.MethodPost, "update"},
		{http.MethodPost, "rollback"},
		{http.MethodPost, "restart"},
	} {
		t.Run(endpoint.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(endpoint.method, "/api/v1/admin/system/"+endpoint.path, nil))
			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}
	require.Len(t, router.Routes(), 1)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/version", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"version":"0.1.241"`)
}

func TestSystemVersionRequiresAdminAuthentication(t *testing.T) {
	router := gin.New()
	group := router.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	h := &handler.Handlers{Admin: &handler.AdminHandlers{System: adminhandler.NewSystemHandler("0.1.241")}}
	registerSystemRoutes(group, h)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/version", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
