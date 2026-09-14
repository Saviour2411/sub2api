package admin

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserCustomizationStrictWriteFields(t *testing.T) {
	for _, body := range []string{
		`{"auto_credit_enabled":true,"credit_threshold":1000,"credit_amount":50,"credit_generation":2}`,
		`{"credit_used_at":null}`, `{"link_enabled":true}`, `{"user_id":2}`, `{} {}`,
		strings.Repeat(" ", 65537) + `{}`,
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(body))
		var input service.UserCustomizationInput
		require.False(t, bindCustomizationJSON(c, &input))
		require.Equal(t, 400, recorder.Code)
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(`{"auto_credit_enabled":true,"credit_threshold":1000,"credit_amount":50}`))
	var input service.UserCustomizationInput
	require.True(t, bindCustomizationJSON(c, &input))
	require.Equal(t, float64(50), input.CreditAmount)
}
