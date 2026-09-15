package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserCustomizationLinkReadAuditDoesNotCaptureSecret(t *testing.T) {
	repo := &auditCaptureRepository{}
	audit := service.NewAuditLogService(repo, nil)
	audit.Start()
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(string(ContextKeyUser), AuthSubject{UserID: 1}); c.Next() })
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
	router.GET("/api/v1/admin/custom-features/user-customizations/:id/link", func(c *gin.Context) { c.JSON(200, gin.H{"token": "不可进入审计的随机码"}) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/custom-features/user-customizations/9/link", nil))
	audit.Stop()
	require.Len(t, repo.logs, 1)
	require.Equal(t, "admin.user-customizations.link.read", repo.logs[0].Action)
	require.Empty(t, repo.logs[0].RequestBody)
	require.NotContains(t, repo.logs[0].Extra, "token")
}
