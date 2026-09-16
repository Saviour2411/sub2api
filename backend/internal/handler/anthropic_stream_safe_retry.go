package handler

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// finishAnthropicStreamRetry 在未提交响应时返回真实 HTTP 错误，已提交时仅补一帧 SSE 错误。
func (h *GatewayHandler) finishAnthropicStreamRetry(c *gin.Context, retry *service.AnthropicStreamRetryState, err error) {
	defer func() { retry.RecordFinalWireStatus(c.Writer.Status()) }()
	if retry.ClientContext().Err() != nil {
		retry.RecordDecision(c.Request.Context(), "stop", "client_canceled")
		failoverClientGone(c)
		return
	}
	reason := "replay_forbidden"
	var failure *service.AnthropicStreamFailure
	if errors.As(err, &failure) {
		reason = failure.Kind
	}
	if retry.Committed() {
		retry.RecordDecision(c.Request.Context(), "stop", "output_committed")
	} else {
		retry.RecordDecision(c.Request.Context(), "stop", reason)
	}
	if reason == "client_canceled" || (c.Request.Context().Err() != nil && reason != "budget_exhausted") {
		failoverClientGone(c)
		return
	}
	status := http.StatusBadGateway
	if failure != nil {
		status = failure.ClientStatus()
	}
	var failover *service.UpstreamFailoverError
	if errors.As(err, &failover) {
		h.handleFailoverExhausted(c, failover, service.PlatformAnthropic, c.Writer.Written())
		return
	}
	message := "上游流未正常完成，请重新发起请求"
	if reason == "budget_exhausted" {
		message = "等待上游有效内容的总预算已耗尽"
	}
	if reason == "retries_exhausted" {
		message = "上游流安全重试次数已耗尽"
	}
	if reason == "idle_timeout" {
		message = "等待上游流数据超时"
	}
	if reason == "first_content_timeout" {
		message = "等待上游有效内容超时"
	}
	// HTTP 200 与逻辑错误分开记录，禁止把本地流故障作为账号 HTTP 状态故障。
	h.handleStreamingAwareErrorWithCode(c, status, "upstream_error", "anthropic_stream_"+reason, message, c.Writer.Written())
}
