package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *CustomFeatureHandler) SetUserCustomizationService(s *service.UserCustomizationService) {
	h.userCustomization = s
}

func customizationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "用户 ID 无效")
		return 0, false
	}
	return id, true
}

// 禁止将只读授信状态混入普通保存请求，也不允许尾随多个 JSON 对象。
func bindCustomizationJSON(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		response.BadRequest(c, "请求字段或格式无效")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		response.BadRequest(c, "请求格式无效")
		return false
	}
	return true
}

func (h *CustomFeatureHandler) ListUserCustomizations(c *gin.Context) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	if page > 1000000 || len(c.Query("search")) > 200 {
		response.BadRequest(c, "查询范围无效")
		return
	}
	items, total, err := h.userCustomization.List(c.Request.Context(), c.Query("search"), page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *CustomFeatureHandler) UpdateUserCustomization(c *gin.Context) {
	id, ok := customizationID(c)
	if !ok {
		return
	}
	var in service.UserCustomizationInput
	if !bindCustomizationJSON(c, &in) {
		return
	}
	executeAdminIdempotentJSON(c, "admin.user-customizations.update", struct {
		ID    int64                          `json:"id"`
		Input service.UserCustomizationInput `json:"input"`
	}{id, in}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) { return h.userCustomization.Save(ctx, id, in) })
}

func (h *CustomFeatureHandler) RotateUserCustomizationLink(c *gin.Context) {
	id, ok := customizationID(c)
	if !ok {
		return
	}
	var in struct {
		Version *int64 `json:"version"`
	}
	if !bindCustomizationJSON(c, &in) {
		return
	}
	if in.Version == nil || *in.Version < 0 {
		response.BadRequest(c, "链接版本无效")
		return
	}
	executeAdminIdempotentJSON(c, "admin.user-customizations.link.rotate", struct {
		ID      int64 `json:"id"`
		Version int64 `json:"version"`
	}{id, *in.Version}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) { return h.userCustomization.RotateLink(ctx, id, *in.Version) })
}

func (h *CustomFeatureHandler) SetUserCustomizationLinkEnabled(c *gin.Context) {
	id, ok := customizationID(c)
	if !ok {
		return
	}
	var in struct {
		Version *int64 `json:"version"`
		Enabled *bool  `json:"enabled"`
	}
	if !bindCustomizationJSON(c, &in) {
		return
	}
	if in.Version == nil || *in.Version < 0 || in.Enabled == nil {
		response.BadRequest(c, "链接配置无效")
		return
	}
	executeAdminIdempotentJSON(c, "admin.user-customizations.link.enabled", struct {
		ID      int64 `json:"id"`
		Version int64 `json:"version"`
		Enabled bool  `json:"enabled"`
	}{id, *in.Version, *in.Enabled}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.userCustomization.SetLinkEnabled(ctx, id, *in.Version, *in.Enabled)
	})
}

func (h *CustomFeatureHandler) GetUserCustomizationLink(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	id, ok := customizationID(c)
	if !ok {
		return
	}
	token, err := h.userCustomization.LinkToken(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 原值仅在显式敏感读取中返回，不进入写请求幂等响应缓存。
	response.Success(c, gin.H{"token": token})
}

func (h *CustomFeatureHandler) RestoreUserCustomizationCredit(c *gin.Context) {
	id, ok := customizationID(c)
	if !ok {
		return
	}
	var in struct {
		Generation int64 `json:"generation"`
	}
	if !bindCustomizationJSON(c, &in) {
		return
	}
	if in.Generation <= 0 {
		response.BadRequest(c, "授信代次无效")
		return
	}
	executeAdminIdempotentJSON(c, "admin.user-customizations.credit.restore", struct {
		ID         int64 `json:"id"`
		Generation int64 `json:"generation"`
	}{id, in.Generation}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.userCustomization.RestoreCredit(ctx, id, in.Generation)
	})
}

func (h *CustomFeatureHandler) GetUserPaymentMethods(c *gin.Context) {
	methods, err := h.userCustomization.PaymentMethods(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"payment_methods": methods})
}

func (h *CustomFeatureHandler) UpdateUserPaymentMethods(c *gin.Context) {
	var in struct {
		Methods []service.UserPaymentMethod `json:"payment_methods"`
	}
	if !bindCustomizationJSON(c, &in) {
		return
	}
	executeAdminIdempotentJSON(c, "admin.user-customizations.payment-methods.update", in, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		methods, err := h.userCustomization.SavePaymentMethods(ctx, in.Methods)
		return gin.H{"payment_methods": methods}, err
	})
}
