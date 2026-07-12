package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerCustomFeatureRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	customFeatures := admin.Group("/custom-features")
	{
		customFeatures.GET("", h.Admin.CustomFeature.GetSettings)
		customFeatures.PUT("/model-marketplace", h.Admin.CustomFeature.UpdateModelMarketplace)
		customFeatures.PUT("/daily-checkin", h.Admin.CustomFeature.UpdateDailyCheckin)
		customFeatures.PUT("/gateway", h.Admin.CustomFeature.UpdateGateway)
		customFeatures.POST("/gateway/image-group-success-rates/reset", h.Admin.CustomFeature.ResetImageGroupSuccessRates)
	}
}
