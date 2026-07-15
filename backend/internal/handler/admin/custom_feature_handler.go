package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CustomFeatureHandler 管理二开功能的独立配置接口。
type CustomFeatureHandler struct {
	settingService     *service.SettingService
	successRateService *service.ImageGroupSuccessRateService
	upstreamService    *service.UpstreamService
}

func NewCustomFeatureHandler(settingService *service.SettingService, successRateService *service.ImageGroupSuccessRateService) *CustomFeatureHandler {
	return &CustomFeatureHandler{settingService: settingService, successRateService: successRateService}
}

func (h *CustomFeatureHandler) SetUpstreamService(upstreamService *service.UpstreamService) {
	h.upstreamService = upstreamService
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

func (h *CustomFeatureHandler) ListUpstreams(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	params := service.UpstreamListParams{
		Page: page, PageSize: pageSize, Search: c.Query("search"), Platform: strings.ToLower(strings.TrimSpace(c.Query("platform"))),
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			response.BadRequest(c, "enabled 参数无效")
			return
		}
		params.Enabled = &enabled
	}
	items, total, err := h.upstreamService.List(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *CustomFeatureHandler) CreateUpstream(c *gin.Context) {
	var input service.UpstreamCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		input.CreatedBy = subject.UserID
	}
	item, err := h.upstreamService.Create(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *CustomFeatureHandler) UpdateUpstream(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	var input service.UpstreamUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	item, err := h.upstreamService.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *CustomFeatureHandler) SetUpstreamEnabled(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Enabled == nil {
		response.BadRequest(c, "enabled 字段必填")
		return
	}
	item, err := h.upstreamService.SetEnabled(c.Request.Context(), id, *input.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *CustomFeatureHandler) DeleteUpstream(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	if err := h.upstreamService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *CustomFeatureHandler) SyncUpstream(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	if err := h.upstreamService.QueueSync(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, gin.H{"queued": true})
}

func (h *CustomFeatureHandler) SyncAllUpstreams(c *gin.Context) {
	count, err := h.upstreamService.QueueAll(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, gin.H{"queued": count})
}

func (h *CustomFeatureHandler) ListUpstreamGroups(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	items, err := h.upstreamService.ListGroups(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *CustomFeatureHandler) SetUpstreamGroupDisplayed(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	var input service.UpstreamGroupDisplayInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求格式无效")
		return
	}
	result, err := h.upstreamService.SetGroupDisplayed(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *CustomFeatureHandler) ListUpstreamHistory(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	from, through, ok := parseUpstreamDateRange(c)
	if !ok {
		return
	}
	items, err := h.upstreamService.ListHistory(c.Request.Context(), id, from, through)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *CustomFeatureHandler) ListUpstreamMultiplierHistory(c *gin.Context) {
	id, ok := parseUpstreamID(c)
	if !ok {
		return
	}
	from, through, ok := parseUpstreamDateRange(c)
	if !ok {
		return
	}
	items, err := h.upstreamService.ListMultiplierHistory(c.Request.Context(), id, from, through)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func parseUpstreamDateRange(c *gin.Context) (time.Time, time.Time, bool) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	today := time.Now().In(loc)
	through := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	from := through.AddDate(0, 0, -29)
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		from, err = time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			response.BadRequest(c, "from 日期格式无效")
			return time.Time{}, time.Time{}, false
		}
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		through, err = time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			response.BadRequest(c, "to 日期格式无效")
			return time.Time{}, time.Time{}, false
		}
	}
	return from, through, true
}

func parseUpstreamID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "站点 ID 无效")
		return 0, false
	}
	return id, true
}
