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

		capture, _ := c.Writer.(*opsCaptureWriter)
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
		succeeded, excluded := imageGroupRequestResult(c, capture)
		if excluded {
			return
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

func imageGroupRequestResult(c *gin.Context, capture *opsCaptureWriter) (succeeded, excluded bool) {
	status := c.Writer.Status()
	var result parsedOpsError
	if capture != nil {
		capture.finalizeCapture()
		result = parseOpsErrorResponse(capture.capturedBytes())
		if terminal, ok := capture.capturedTerminalError(); ok {
			result = terminal
		}
	}
	streamError, streamFailed := service.GetOpsStreamError(c)
	if status >= http.StatusOK && status < http.StatusMultipleChoices && !streamFailed && !result.StreamFailure {
		return true, false
	}
	if streamFailed {
		result = parsedOpsError{Code: streamError.Code, ErrorType: streamError.ErrType, Message: streamError.Message}
		if streamError.IntendedStatus > 0 {
			status = streamError.IntendedStatus
		}
	} else if result.StatusCode > 0 {
		status = result.StatusCode
	}
	if service.IsImageGroupExcludedError(status, result.Code, result.ErrorType, result.Message) {
		return false, true
	}
	// 请求级带内拒绝不能继承此前重试的上游错误。
	if streamFailed && streamError.RequestScoped {
		return false, false
	}

	upstreamStatus, upstream := imageGroupFinalUpstreamError(c)
	if streamFailed {
		upstreamStatus, upstream = imageGroupUpstreamError(streamError.UpstreamStatus, streamError.UpstreamMessage, streamError.UpstreamDetail)
		if event := imageGroupLastUpstreamEvent(streamError.UpstreamErrors); event != nil {
			upstreamStatus, upstream = imageGroupUpstreamEventError(event)
		}
	}
	if upstreamStatus > 0 {
		status = upstreamStatus
	}
	message := result.Message + "\n" + upstream.Message
	return false, service.IsImageGroupExcludedError(status, upstream.Code, upstream.ErrorType, message)
}

func imageGroupFinalUpstreamError(c *gin.Context) (int, parsedOpsError) {
	if value, ok := c.Get(service.OpsUpstreamErrorsKey); ok {
		if events, ok := value.([]*service.OpsUpstreamErrorEvent); ok {
			if event := imageGroupLastUpstreamEvent(events); event != nil {
				return imageGroupUpstreamEventError(event)
			}
		}
	}
	status := opsJSONInt(c.Value(service.OpsUpstreamStatusCodeKey))
	return imageGroupUpstreamError(status, c.GetString(service.OpsUpstreamErrorMessageKey), c.GetString(service.OpsUpstreamErrorDetailKey))
}

func imageGroupLastUpstreamEvent(events []*service.OpsUpstreamErrorEvent) *service.OpsUpstreamErrorEvent {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i] != nil {
			return events[i]
		}
	}
	return nil
}

func imageGroupUpstreamEventError(event *service.OpsUpstreamErrorEvent) (int, parsedOpsError) {
	detail := event.UpstreamResponseBody
	if detail == "" {
		detail = event.Detail
	}
	return imageGroupUpstreamError(event.UpstreamStatusCode, event.Message, detail)
}

func imageGroupUpstreamError(status int, message, detail string) (int, parsedOpsError) {
	result := parseOpsErrorResponse([]byte(detail))
	if result.Message == "" {
		result.Message = message
	} else if message != "" {
		result.Message = message + "\n" + result.Message
	}
	return status, result
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
		"/v1/videos/edits",
		"/v1/videos/extensions",
		"/responses",
		"/responses/compact",
		"/chat/completions",
		"/images/generations",
		"/images/edits",
		"/videos/generations",
		"/videos/edits",
		"/videos/extensions",
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
