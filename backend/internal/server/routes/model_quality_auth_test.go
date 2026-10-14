package routes

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelQualityAdminAuthentication(t *testing.T) {
	r := gin.New()
	group := r.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	registerCustomFeatureRoutes(group, &handler.Handlers{ModelQuality: handler.NewModelQualityHandler(nil), Admin: &handler.AdminHandlers{CustomFeature: adminhandler.NewCustomFeatureHandler(nil, nil)}})
	for _, tc := range []struct{ method, path string }{{"GET", "/custom-features/model-quality"}, {"PUT", "/custom-features/model-quality"}, {"GET", "/custom-features/model-quality/keys?group_id=7"}, {"POST", "/model-quality/groups/7/run"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/v1/admin"+tc.path, nil))
		require.Equal(t, 401, w.Code)
	}
}
