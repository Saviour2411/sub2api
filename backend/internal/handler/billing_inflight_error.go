package handler

import "github.com/gin-gonic/gin"

func normalizeBillingError(c *gin.Context, errType, code string) (string, string) {
	if errType == "balance_inflight_limit" || code == "balance_inflight_limit" {
		c.Header("Retry-After", "1")
		return "rate_limit_error", "balance_inflight_limit"
	}
	return errType, code
}

func billingErrorObject(c *gin.Context, errType, message string) gin.H {
	errType, code := normalizeBillingError(c, errType, "")
	result := gin.H{"type": errType, "message": message}
	if code != "" {
		result["code"] = code
	}
	return result
}
