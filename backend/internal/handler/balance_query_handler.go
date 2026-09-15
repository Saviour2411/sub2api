package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const BalanceQueryTokenHeader = "X-Balance-Query-Token"

type balanceQueryLimiter interface {
	Allow(context.Context, string, int, time.Duration) (middleware.AllowResult, error)
}

type BalanceQueryHandler struct {
	service *service.UserCustomizationService
	limiter balanceQueryLimiter
}

func NewBalanceQueryHandler(s *service.UserCustomizationService, limiter *middleware.RateLimiter) *BalanceQueryHandler {
	return &BalanceQueryHandler{service: s, limiter: limiter}
}

func (h *BalanceQueryHandler) allow(c *gin.Context, key string, limit int) bool {
	result, err := h.limiter.Allow(c.Request.Context(), key, limit, time.Minute)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "查询暂时不可用，请稍后重试")
		return false
	}
	if !result.Allowed {
		seconds := int64((result.RetryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 60
		}
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
		response.Error(c, http.StatusTooManyRequests, "查询过于频繁，请稍后重试")
		return false
	}
	return true
}

func (h *BalanceQueryHandler) Query(c *gin.Context) {
	// 尽早移除敏感头，避免异常恢复或后置日志中间件转储请求时泄漏随机码。
	token := c.GetHeader(BalanceQueryTokenHeader)
	c.Request.Header.Del(BalanceQueryTokenHeader)
	c.Header("Cache-Control", "no-store, private")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex, nofollow, noarchive")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	ip := ippkg.GetSecurityClientIP(c, false)
	if ip == "" {
		ip = c.ClientIP()
	}
	if !h.allow(c, "balance-query:ip:"+ip, 30) {
		return
	}
	for key := range c.Request.URL.Query() {
		if key != "page" && key != "page_size" {
			response.BadRequest(c, "查询参数无效")
			return
		}
	}
	if len(token) != 43 {
		response.ErrorFrom(c, service.ErrBalanceQueryUnavailable)
		return
	}
	user, err := h.service.ResolvePublic(ctx, token)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !h.allow(c, "balance-query:link:"+service.BalanceQueryTokenHash(token), 60) {
		return
	}
	page, size := 1, 20
	if raw := c.Query("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 || page > 1000000 {
			response.BadRequest(c, "页码无效")
			return
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		size, err = strconv.Atoi(raw)
		if err != nil || size < 1 {
			response.BadRequest(c, "分页大小无效")
			return
		}
		if size > 50 {
			size = 50
		}
	}
	data, err := h.service.Query(ctx, *user, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
