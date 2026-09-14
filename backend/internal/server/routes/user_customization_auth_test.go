package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserCustomizationAdminRoutesRequireAuthentication(t *testing.T) {
	router := gin.New()
	group := router.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{CustomFeature: adminhandler.NewCustomFeatureHandler(nil, nil)}}
	registerCustomFeatureRoutes(group, handlers)
	base := "/api/v1/admin/custom-features/user-customizations"
	for _, endpoint := range []struct{ method, path string }{
		{http.MethodGet, ""}, {http.MethodGet, "/payment-methods"}, {http.MethodPut, "/payment-methods"},
		{http.MethodPut, "/7"}, {http.MethodGet, "/7/link"}, {http.MethodPost, "/7/link"},
		{http.MethodPatch, "/7/link"}, {http.MethodPost, "/7/restore-credit"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(endpoint.method, base+endpoint.path, strings.NewReader(`{}`))
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, endpoint.method+endpoint.path)
	}
}
