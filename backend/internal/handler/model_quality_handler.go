package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	auth "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelQualityHandler struct{ service *service.ModelQualityService }

func NewModelQualityHandler(s *service.ModelQualityService) *ModelQualityHandler {
	return &ModelQualityHandler{s}
}
func (h *ModelQualityHandler) GetConfig(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	item, err := h.service.Config(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
func (h *ModelQualityHandler) SaveConfig(c *gin.Context) {
	subject, ok := auth.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请使用管理员账号登录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var input service.ModelQualityConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "检测配置格式无效")
		return
	}
	if err := h.service.SaveConfig(c.Request.Context(), subject.UserID, &input); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.GetConfig(c)
}
func (h *ModelQualityHandler) Keys(c *gin.Context) {
	subject, ok := auth.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请使用管理员账号登录")
		return
	}
	groupID, err := strconv.ParseInt(c.Query("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "分组 ID 无效")
		return
	}
	items, err := h.service.KeyOptions(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, items)
}
func (h *ModelQualityHandler) RunNow(c *gin.Context) {
	id, ok := modelQualityID(c)
	if !ok {
		return
	}
	if err := h.service.RunNow(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, gin.H{"queued": true})
}
func (h *ModelQualityHandler) Groups(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	items, err := h.service.Groups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *ModelQualityHandler) Runs(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Query("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "请选择分组")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		response.BadRequest(c, "页码无效")
		return
	}
	items, total, err := h.service.Runs(c.Request.Context(), service.ModelQualityFilter{GroupID: groupID, Page: page, Model: c.Query("model"), Method: c.Query("method"), Outcome: c.Query("outcome")})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Paginated(c, items, total, page, 12)
}
func (h *ModelQualityHandler) Run(c *gin.Context) {
	id, ok := modelQualityID(c)
	if !ok {
		return
	}
	item, err := h.service.Run(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, item)
}
func modelQualityID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "ID 无效")
		return 0, false
	}
	return id, true
}
