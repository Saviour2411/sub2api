package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CustomFeatureHandler 管理二开功能的独立配置接口。
type CustomFeatureHandler struct {
	settingService *service.SettingService
}

func NewCustomFeatureHandler(settingService *service.SettingService) *CustomFeatureHandler {
	return &CustomFeatureHandler{settingService: settingService}
}

// GetSettings 返回模型广场和每日签到配置。
func (h *CustomFeatureHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetCustomFeatureSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateModelMarketplace 更新模型广场配置。
func (h *CustomFeatureHandler) UpdateModelMarketplace(c *gin.Context) {
	var req service.ModelMarketplaceSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	settings, err := h.settingService.UpdateModelMarketplaceSettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateDailyCheckin 更新每日签到配置。
func (h *CustomFeatureHandler) UpdateDailyCheckin(c *gin.Context) {
	var req service.DailyCheckinSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	settings, err := h.settingService.UpdateDailyCheckinSettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
