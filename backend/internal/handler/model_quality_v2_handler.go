package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	auth "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *ModelQualityHandler) LegacyWriteDisabled(c *gin.Context) {
	response.Error(c, http.StatusGone, "旧版第三方检测已停用，请刷新页面使用新版题库")
}

func (h *ModelQualityHandler) GetConfigV2(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	value, err := h.service.ConfigV2(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, value)
}
func (h *ModelQualityHandler) SaveConfigV2(c *gin.Context) {
	subject, ok := auth.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请使用管理员账号登录")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var input service.ModelQualityV2Config
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "检测配置必须是有效 JSON 且不超过 1 MiB")
		return
	}
	if err := h.service.SaveConfigV2(c.Request.Context(), subject.UserID, &input); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.GetConfigV2(c)
}
func (h *ModelQualityHandler) KeysV2(c *gin.Context) {
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
	items, err := h.service.KeyOptionsV2(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, items)
}
func (h *ModelQualityHandler) RunNowV2(c *gin.Context) {
	id, ok := modelQualityID(c)
	if !ok {
		return
	}
	if err := h.service.RunNowV2(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, gin.H{"queued": true})
}
func (h *ModelQualityHandler) GroupsV2(c *gin.Context) {
	items, err := h.service.GroupsV2(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, items)
}
func (h *ModelQualityHandler) RunsV2(c *gin.Context) {
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
	items, total, err := h.service.RunsV2(c.Request.Context(), service.ModelQualityFilter{GroupID: groupID, Model: c.Query("model"), Method: c.Query("method"), Outcome: c.Query("outcome"), Page: page})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	private, err := h.privateQualityRuns(c, items)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var result any = items
	if private != nil {
		result = private
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"items": result, "total": total, "page": page, "page_size": 12, "pages": (total + 11) / 12})
}
func (h *ModelQualityHandler) RunV2(c *gin.Context) {
	item, err := h.service.RunV2(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	private, err := h.privateQualityRuns(c, []service.ModelQualityV2Run{*item})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if len(private) == 1 {
		response.Success(c, private[0])
		return
	}
	response.Success(c, item)
}

func (h *ModelQualityHandler) privateQualityRuns(c *gin.Context, runs []service.ModelQualityV2Run) ([]service.ModelQualityPrivateRun, error) {
	subject, ok := auth.GetAuthSubjectFromContext(c)
	role, _ := auth.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		return nil, nil
	}
	return h.service.PrivateQualityV2Runs(c.Request.Context(), subject.UserID, runs)
}

func (h *ModelQualityHandler) RejudgeV2(c *gin.Context) {
	subject, ok := auth.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请使用管理员账号登录")
		return
	}
	if err := h.service.RejudgeV2(c.Request.Context(), c.Param("id"), subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, gin.H{"queued": true})
}
