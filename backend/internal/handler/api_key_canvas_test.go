package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestResolveCanvasCredentialAlwaysDisablesCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/canvas/credentials/resolve", nil)

	NewAPIKeyHandler(nil).ResolveCanvasCredential(c)

	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
