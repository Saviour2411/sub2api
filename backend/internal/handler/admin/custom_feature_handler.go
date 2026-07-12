package admin

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CustomFeatureHandler 管理二开功能的独立配置接口。
type CustomFeatureHandler struct {
	settingService     *service.SettingService
	successRateService *service.ImageGroupSuccessRateService
}

func NewCustomFeatureHandler(settingService *service.SettingService, successRateService *service.ImageGroupSuccessRateService) *CustomFeatureHandler {
	return &CustomFeatureHandler{settingService: settingService, successRateService: successRateService}
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

// UpdateGateway 更新网关配置。
func (h *CustomFeatureHandler) UpdateGateway(c *gin.Context) {
	current, err := h.settingService.GetGatewaySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	req := *current
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	settings, err := h.settingService.UpdateGatewaySettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// ResetImageGroupSuccessRates 切换统计代次，不回算历史请求。
func (h *CustomFeatureHandler) ResetImageGroupSuccessRates(c *gin.Context) {
	resetAt, err := h.successRateService.Reset(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"reset_at": resetAt.UTC().Format(time.RFC3339Nano)})
}
