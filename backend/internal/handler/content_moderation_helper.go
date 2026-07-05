package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type successfulConversationAuditOptions struct {
	SessionID          string
	ClientSessionID    string
	SessionSource      string
	UserAgent          string
	Originator         string
	ResponseID         string
	PreviousResponseID string
	RawResponse        []byte
}

const successfulConversationAuditCaptureEnabledKey = "successful_conversation_audit_capture_enabled"

func (h *GatewayHandler) beginSuccessfulConversationAuditCapture(c *gin.Context) (*auditResponseCaptureWriter, func()) {
	if h == nil {
		return nil, func() {}
	}
	return beginSuccessfulConversationAuditCapture(c, h.contentModerationService)
}

//nolint:unused
func (h *OpenAIGatewayHandler) beginSuccessfulConversationAuditCapture(c *gin.Context) (*auditResponseCaptureWriter, func()) {
	if h == nil {
		return nil, func() {}
	}
	return beginSuccessfulConversationAuditCapture(c, h.contentModerationService)
}

func beginSuccessfulConversationAuditCapture(c *gin.Context, svc *service.ContentModerationService) (*auditResponseCaptureWriter, func()) {
	if svc == nil || c == nil || c.Request == nil {
		return nil, func() {}
	}
	release, ok := svc.TryBeginLocalAuditCapture(c.Request.Context())
	if !ok {
		return nil, func() {}
	}
	auditCapture, restoreAuditCapture := attachAuditResponseCapture(c)
	if auditCapture == nil {
		release()
		return nil, func() {}
	}
	c.Set(successfulConversationAuditCaptureEnabledKey, true)
	return auditCapture, func() {
		c.Set(successfulConversationAuditCaptureEnabledKey, false)
		restoreAuditCapture()
		release()
	}
}

func (h *GatewayHandler) checkContentModeration(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *service.ContentModerationDecision {
	if h == nil || h.contentModerationService == nil {
		return nil
	}
	return runContentModeration(c, reqLog, h.contentModerationService, apiKey, subject, protocol, model, body)
}

func (h *GatewayHandler) recordSuccessfulConversationAudit(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, upstreamModel string, stream bool, body []byte, usage any, opts ...successfulConversationAuditOptions) {
	if h == nil || h.contentModerationService == nil {
		return
	}
	recordSuccessfulConversationAudit(c, h.contentModerationService, apiKey, subject, protocol, model, upstreamModel, stream, body, usage, opts...)
}

func contentModerationStatus(decision *service.ContentModerationDecision) int {
	if decision == nil || decision.StatusCode < 400 || decision.StatusCode > 599 {
		return http.StatusForbidden
	}
	return decision.StatusCode
}

func contentModerationErrorCode(decision *service.ContentModerationDecision) string {
	return "content_policy_violation"
}

func (h *OpenAIGatewayHandler) checkContentModeration(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *service.ContentModerationDecision {
	if h == nil || h.contentModerationService == nil {
		return nil
	}
	return runContentModeration(c, reqLog, h.contentModerationService, apiKey, subject, protocol, model, body)
}

//nolint:unused
func (h *OpenAIGatewayHandler) recordSuccessfulConversationAudit(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, upstreamModel string, stream bool, body []byte, usage any, opts ...successfulConversationAuditOptions) {
	if h == nil || h.contentModerationService == nil {
		return
	}
	recordSuccessfulConversationAudit(c, h.contentModerationService, apiKey, subject, protocol, model, upstreamModel, stream, body, usage, opts...)
}

func runContentModeration(c *gin.Context, reqLog *zap.Logger, svc *service.ContentModerationService, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) *service.ContentModerationDecision {
	if svc == nil || c == nil || c.Request == nil {
		return nil
	}
	input := buildContentModerationInput(c, apiKey, subject, protocol, model, body)
	if reqLog != nil {
		reqLog.Info("content_moderation.gateway_check_start",
			zap.String("request_id", input.RequestID),
			zap.Int64("user_id", input.UserID),
			zap.Int64("api_key_id", input.APIKeyID),
			zap.String("api_key_name", input.APIKeyName),
			zap.Int64p("group_id", input.GroupID),
			zap.String("group_name", input.GroupName),
			zap.String("endpoint", input.Endpoint),
			zap.String("provider", input.Provider),
			zap.String("protocol", input.Protocol),
			zap.String("model", input.Model),
			zap.Int("body_bytes", len(body)),
		)
	}
	decision, err := svc.Check(c.Request.Context(), input)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("content_moderation.check_failed", zap.Error(err))
		}
		return nil
	}
	if reqLog != nil && decision != nil {
		reqLog.Info("content_moderation.gateway_check_done",
			zap.String("request_id", input.RequestID),
			zap.Bool("allowed", decision.Allowed),
			zap.Bool("blocked", decision.Blocked),
			zap.Bool("flagged", decision.Flagged),
			zap.String("action", decision.Action),
			zap.Int("status_code", decision.StatusCode),
			zap.String("highest_category", decision.HighestCategory),
			zap.Float64("highest_score", decision.HighestScore),
		)
	}
	return decision
}

func buildContentModerationInput(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, body []byte) service.ContentModerationCheckInput {
	input := service.ContentModerationCheckInput{
		RequestID: contentModerationRequestID(c.Request.Context()),
		UserID:    subject.UserID,
		Endpoint:  GetInboundEndpoint(c),
		Provider:  contentModerationProvider(apiKey),
		Model:     strings.TrimSpace(model),
		Protocol:  protocol,
		Body:      body,
	}
	if forcedPlatform, ok := middleware2.GetForcePlatformFromContext(c); ok {
		input.Provider = strings.TrimSpace(forcedPlatform)
	}
	if apiKey != nil {
		input.APIKeyID = apiKey.ID
		input.APIKeyName = apiKey.Name
		if apiKey.User != nil {
			input.UserEmail = apiKey.User.Email
		}
		if apiKey.GroupID != nil {
			groupID := *apiKey.GroupID
			input.GroupID = &groupID
		}
		if apiKey.Group != nil {
			input.GroupName = apiKey.Group.Name
		}
	}
	if input.Endpoint == "" && c.Request != nil && c.Request.URL != nil {
		input.Endpoint = c.Request.URL.Path
	}
	return input
}

func recordSuccessfulConversationAudit(c *gin.Context, svc *service.ContentModerationService, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol string, model string, upstreamModel string, stream bool, body []byte, usage any, opts ...successfulConversationAuditOptions) {
	if svc == nil || c == nil || c.Request == nil || len(body) == 0 {
		return
	}
	if enabled, ok := c.Get(successfulConversationAuditCaptureEnabledKey); !ok || enabled != true {
		return
	}
	var opt successfulConversationAuditOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	input := buildContentModerationInput(c, apiKey, subject, protocol, model, body)
	svc.RecordSuccessfulConversation(c.Request.Context(), service.ContentModerationLocalAuditInput{
		RequestID:          input.RequestID,
		UserID:             input.UserID,
		UserEmail:          input.UserEmail,
		APIKeyID:           input.APIKeyID,
		APIKeyName:         input.APIKeyName,
		GroupID:            input.GroupID,
		GroupName:          input.GroupName,
		Endpoint:           input.Endpoint,
		Provider:           input.Provider,
		Model:              input.Model,
		UpstreamModel:      strings.TrimSpace(upstreamModel),
		Protocol:           input.Protocol,
		SessionID:          strings.TrimSpace(opt.SessionID),
		ClientSessionID:    strings.TrimSpace(opt.ClientSessionID),
		SessionSource:      strings.TrimSpace(opt.SessionSource),
		UserAgent:          strings.TrimSpace(firstHeaderValue(opt.UserAgent, c.GetHeader("User-Agent"))),
		Originator:         strings.TrimSpace(firstHeaderValue(opt.Originator, c.GetHeader("Originator"))),
		ResponseID:         strings.TrimSpace(opt.ResponseID),
		PreviousResponseID: strings.TrimSpace(opt.PreviousResponseID),
		Stream:             stream,
		Body:               body,
		RawResponse:        append([]byte(nil), opt.RawResponse...),
		Usage:              usage,
	})
}

//nolint:unused
func resolveOpenAISessionAuditFields(explicitSessionID, sessionHash string) (string, string, string) {
	explicitSessionID = strings.TrimSpace(explicitSessionID)
	sessionHash = strings.TrimSpace(sessionHash)
	if explicitSessionID != "" {
		return explicitSessionID, explicitSessionID, "explicit"
	}
	if sessionHash != "" {
		return sessionHash, "", "hash_fallback"
	}
	return "", "", ""
}

func resolveParsedSessionAuditFields(parsedReq *service.ParsedRequest, sessionHash string) (string, string, string) {
	if parsedReq != nil {
		if parsed := service.ParseMetadataUserID(parsedReq.MetadataUserID); parsed != nil && strings.TrimSpace(parsed.SessionID) != "" {
			sessionID := strings.TrimSpace(parsed.SessionID)
			return sessionID, sessionID, "metadata_user_id"
		}
	}
	sessionHash = strings.TrimSpace(sessionHash)
	if sessionHash != "" {
		return sessionHash, "", "hash_fallback"
	}
	return "", "", ""
}

func firstHeaderValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func contentModerationProvider(apiKey *service.APIKey) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return strings.TrimSpace(apiKey.Group.Platform)
}

func contentModerationRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(ctxkey.RequestID).(string); ok {
		return strings.TrimSpace(requestID)
	}
	return ""
}
