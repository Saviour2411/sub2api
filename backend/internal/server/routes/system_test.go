package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/buildmeta"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemRoutesOnlyExposeReadOnlyVersion(t *testing.T) {
	router := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{System: adminhandler.NewSystemHandler("0.1.241", buildmeta.UpstreamSync{}, nil)}}
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
	require.Len(t, router.Routes(), 2)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/version", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"version":"0.1.241"`)
}

func TestSystemVersionRequiresAdminAuthentication(t *testing.T) {
	router := gin.New()
	group := router.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	h := &handler.Handlers{Admin: &handler.AdminHandlers{System: adminhandler.NewSystemHandler("0.1.241", buildmeta.UpstreamSync{}, nil)}}
	registerSystemRoutes(group, h)
	for _, endpoint := range []string{"version", "upstream-version"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/"+endpoint, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

type systemVersionUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *systemVersionUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	return r.user, nil
}

func (r *systemVersionUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func TestSystemVersionRejectsOrdinaryUser(t *testing.T) {
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "version-test-secret", ExpireHour: 1}}
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{ID: 42, Role: service.RoleUser, Status: service.StatusActive}
	users := service.NewUserService(&systemVersionUserRepo{user: user}, nil, nil, nil)
	token, err := auth.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	router := gin.New()
	group := router.Group("/api/v1/admin")
	group.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(auth, users, nil, nil)))
	h := &handler.Handlers{Admin: &handler.AdminHandlers{System: adminhandler.NewSystemHandler("0.1.244", buildmeta.UpstreamSync{}, nil)}}
	registerSystemRoutes(group, h)
	for _, endpoint := range []string{"version", "upstream-version"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/"+endpoint, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		require.Equal(t, http.StatusForbidden, w.Code)
		require.NotContains(t, w.Body.String(), "0.1.244")
	}
}
