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

		upstreams := customFeatures.Group("/upstreams")
		{
			upstreams.GET("", h.Admin.CustomFeature.ListUpstreams)
			upstreams.POST("", h.Admin.CustomFeature.CreateUpstream)
			upstreams.POST("/sync-all", h.Admin.CustomFeature.SyncAllUpstreams)
			upstreams.PUT("/:id", h.Admin.CustomFeature.UpdateUpstream)
			upstreams.PATCH("/:id/enabled", h.Admin.CustomFeature.SetUpstreamEnabled)
			upstreams.DELETE("/:id", h.Admin.CustomFeature.DeleteUpstream)
			upstreams.POST("/:id/sync", h.Admin.CustomFeature.SyncUpstream)
			upstreams.GET("/:id/groups", h.Admin.CustomFeature.ListUpstreamGroups)
			upstreams.PATCH("/:id/groups/display", h.Admin.CustomFeature.SetUpstreamGroupDisplayed)
			upstreams.GET("/:id/history", h.Admin.CustomFeature.ListUpstreamHistory)
			upstreams.GET("/:id/multiplier-history", h.Admin.CustomFeature.ListUpstreamMultiplierHistory)
		}
	}
}
