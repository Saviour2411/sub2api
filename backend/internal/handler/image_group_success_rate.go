package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const imageGroupSuccessRateWriteTimeout = 2 * time.Second

type imageGroupSuccessRatesResponse struct {
	Visible bool                                `json:"visible"`
	Items   []imageGroupSuccessRateItemResponse `json:"items"`
}

type imageGroupSuccessRateItemResponse struct {
	GroupID       int64      `json:"group_id"`
	GroupName     string     `json:"group_name"`
	SuccessRate   float64    `json:"success_rate"`
	LastSuccessAt *time.Time `json:"last_success_at"`
}

func (h *ChannelMonitorUserHandler) imageGroupSuccessRates(ctx context.Context) (imageGroupSuccessRatesResponse, error) {
	visible := true
	if h != nil && h.settingService != nil {
		visible = h.settingService.GetGatewayRuntime(ctx).ImageGroupSuccessRateVisible
	}
	result := imageGroupSuccessRatesResponse{
		Visible: visible,
		Items:   []imageGroupSuccessRateItemResponse{},
	}
	if !visible || h == nil || h.successRateService == nil {
		return result, nil
	}
	items, err := h.successRateService.List(ctx)
	if err != nil {
		return result, err
	}
	result.Items = make([]imageGroupSuccessRateItemResponse, 0, len(items))
	for _, item := range items {
		result.Items = append(result.Items, imageGroupSuccessRateItemResponse{
			GroupID:       item.GroupID,
			GroupName:     item.GroupName,
			SuccessRate:   item.SuccessRate,
			LastSuccessAt: item.LastSuccessAt,
		})
	}
	return result, nil
}

// TrackImageGroupRequestResult 在一次完整网关请求结束后按最终结果统计一次。
func (h *ChannelMonitorUserHandler) TrackImageGroupRequestResult() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.successRateService == nil || !isImageGroupSuccessRateEndpoint(c) {
			c.Next()
			return
		}

		c.Next()
		if c.Request == nil || c.Request.Context().Err() != nil || c.Writer.Status() == statusClientClosedRequest {
			return
		}
		if _, selected := c.Get(opsAccountIDKey); !selected {
			return
		}
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.GroupID == nil || *apiKey.GroupID <= 0 {
			return
		}

		status := c.Writer.Status()
		succeeded := status >= http.StatusOK && status < http.StatusMultipleChoices
		if _, streamFailed := service.GetOpsStreamError(c); streamFailed {
			succeeded = false
		}
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), imageGroupSuccessRateWriteTimeout)
		defer cancel()
		if err := h.successRateService.RecordRequestResult(writeCtx, *apiKey.GroupID, succeeded); err != nil {
			logger.L().Warn("image_group_success_rate.record_failed",
				zap.Int64("group_id", *apiKey.GroupID),
				zap.Int("status", status),
				zap.Error(err),
			)
		}
	}
}

func isImageGroupSuccessRateEndpoint(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.Method != http.MethodPost {
		return false
	}
	path := strings.TrimRight(c.Request.URL.Path, "/")
	switch path {
	case "/v1/messages",
		"/v1/responses",
		"/v1/responses/compact",
		"/v1/chat/completions",
		"/v1/images/generations",
		"/v1/images/edits",
		"/v1/videos/generations",
		"/responses",
		"/responses/compact",
		"/chat/completions",
		"/images/generations",
		"/images/edits",
		"/videos/generations",
		"/backend-api/codex/responses",
		"/backend-api/codex/responses/compact",
		"/antigravity/v1/messages":
		return true
	}
	if !strings.Contains(path, "/models/") {
		return false
	}
	actionIndex := strings.LastIndex(path, ":")
	if actionIndex < 0 || actionIndex == len(path)-1 {
		return false
	}
	action := path[actionIndex+1:]
	return action == "generateContent" || action == "streamGenerateContent"
}
