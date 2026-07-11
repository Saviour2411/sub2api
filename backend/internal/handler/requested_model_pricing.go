package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const modelPricingNotFoundCode = "MODEL_PRICING_NOT_FOUND"

type invalidRequestedPricingSpecError struct {
	err error
}

func (e *invalidRequestedPricingSpecError) Error() string {
	if e == nil || e.err == nil {
		return "Invalid media pricing parameters"
	}
	return e.err.Error()
}

func (e *invalidRequestedPricingSpecError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type pricingErrorProtocol uint8

const (
	pricingErrorProtocolOpenAI pricingErrorProtocol = iota
	pricingErrorProtocolAnthropic
	pricingErrorProtocolGemini
)

type requestedPricingSpec struct {
	model    string
	kind     service.PricingUsageKind
	sizeTier string
}

func requestedPricingValidationSkipped(cfg *config.Config) bool {
	return cfg != nil && cfg.RunMode == config.RunModeSimple
}

// requestedImagePricingSpec 让请求前校验使用后续用量结算相同的媒体模型和尺寸。
func requestedImagePricingSpec(endpoint, requestedModel string, body []byte) (model, sizeTier string, image bool, err error) {
	image = service.IsImageGenerationIntent(endpoint, requestedModel, body) || service.IsGeneratedImageModel(requestedModel)
	if !image {
		return strings.TrimSpace(requestedModel), "", false, nil
	}

	if strings.Contains(strings.ToLower(strings.TrimSpace(endpoint)), "/responses") {
		cfg, resolveErr := service.ResolveOpenAIResponsesImageBillingConfigDetailedFromBody(body, requestedModel)
		if resolveErr != nil {
			return "", "", true, resolveErr
		}
		return strings.TrimSpace(cfg.Model), cfg.SizeTier, true, nil
	}

	size := firstNonEmptyPricingSize(
		gjson.GetBytes(body, "generationConfig.imageConfig.imageSize").String(),
		gjson.GetBytes(body, "generation_config.image_config.image_size").String(),
		gjson.GetBytes(body, "imageConfig.imageSize").String(),
		gjson.GetBytes(body, "size").String(),
	)
	return strings.TrimSpace(requestedModel), service.NormalizeImageBillingTierOrDefault(size), true, nil
}

// requestedPricingSpecs 覆盖请求最终可能进入的全部用户计费路径。
// Responses 可选图片工具可能只返回文本，因此需要同时校验文本模型和媒体模型。
func requestedPricingSpecs(endpoint, requestedModel string, body []byte) ([]requestedPricingSpec, error) {
	model, sizeTier, image, err := requestedImagePricingSpec(endpoint, requestedModel, body)
	if err != nil {
		return nil, &invalidRequestedPricingSpecError{err: err}
	}
	if !image {
		return []requestedPricingSpec{{
			model: strings.TrimSpace(requestedModel),
			kind:  service.PricingUsageToken,
		}}, nil
	}

	imageSpec := requestedPricingSpec{
		model:    model,
		kind:     service.PricingUsageImage,
		sizeTier: sizeTier,
	}
	isResponses := strings.Contains(strings.ToLower(strings.TrimSpace(endpoint)), "/responses")
	imageOnly := service.IsGeneratedImageModel(requestedModel) || service.IsImageGenerationToolForced(body)
	if !isResponses || imageOnly {
		return []requestedPricingSpec{imageSpec}, nil
	}

	return []requestedPricingSpec{
		{model: strings.TrimSpace(requestedModel), kind: service.PricingUsageToken},
		imageSpec,
	}, nil
}

func (h *GatewayHandler) gatewayRequestPricingValidationError(
	ctx context.Context,
	apiKey *service.APIKey,
	endpoint, requestedModel string,
	body []byte,
) (string, error) {
	if h != nil && requestedPricingValidationSkipped(h.cfg) {
		return "", nil
	}
	specs, err := requestedPricingSpecs(endpoint, requestedModel, body)
	if err != nil {
		return strings.TrimSpace(requestedModel), err
	}
	for _, spec := range specs {
		if err := h.gatewayService.ValidateRequestedModelPricing(ctx, apiKey, spec.model, spec.kind, spec.sizeTier); err != nil {
			return spec.model, err
		}
	}
	return "", nil
}

func (h *OpenAIGatewayHandler) openAIRequestPricingValidationError(
	ctx context.Context,
	apiKey *service.APIKey,
	endpoint, requestedModel string,
	body []byte,
) (string, error) {
	if h != nil && requestedPricingValidationSkipped(h.cfg) {
		return "", nil
	}
	specs, err := requestedPricingSpecs(endpoint, requestedModel, body)
	if err != nil {
		return strings.TrimSpace(requestedModel), err
	}
	for _, spec := range specs {
		if err := h.gatewayService.ValidateRequestedModelPricing(ctx, apiKey, spec.model, spec.kind, spec.sizeTier); err != nil {
			return spec.model, err
		}
	}
	return "", nil
}

func (h *GatewayHandler) validateGatewayRequestPricing(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	protocol pricingErrorProtocol,
	endpoint, requestedModel string,
	body []byte,
) bool {
	model, err := h.gatewayRequestPricingValidationError(c.Request.Context(), apiKey, endpoint, requestedModel, body)
	return validateRequestedPricingAndWrite(c, reqLog, protocol, model, func() error { return err })
}

func (h *OpenAIGatewayHandler) validateOpenAIRequestPricing(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	protocol pricingErrorProtocol,
	endpoint, requestedModel string,
	body []byte,
) bool {
	model, err := h.openAIRequestPricingValidationError(c.Request.Context(), apiKey, endpoint, requestedModel, body)
	return validateRequestedPricingAndWrite(c, reqLog, protocol, model, func() error { return err })
}

func (h *OpenAIGatewayHandler) validateOpenAIPricingSpec(
	c *gin.Context,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	protocol pricingErrorProtocol,
	model string,
	kind service.PricingUsageKind,
	sizeTier string,
	resolveErr error,
) bool {
	if h != nil && requestedPricingValidationSkipped(h.cfg) {
		return true
	}
	return validateRequestedPricingAndWrite(c, reqLog, protocol, model, func() error {
		if resolveErr != nil {
			return resolveErr
		}
		return h.gatewayService.ValidateRequestedModelPricing(c.Request.Context(), apiKey, model, kind, sizeTier)
	})
}

func (h *OpenAIGatewayHandler) validateOpenAIRequestPricingForWS(
	ctx context.Context,
	c *gin.Context,
	conn *coderws.Conn,
	reqLog *zap.Logger,
	apiKey *service.APIKey,
	endpoint, requestedModel string,
	body []byte,
) error {
	model, err := h.openAIRequestPricingValidationError(ctx, apiKey, endpoint, requestedModel, body)
	return validateRequestedPricingForWS(ctx, c, conn, reqLog, model, func() error { return err })
}

func firstNonEmptyPricingSize(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func validateRequestedPricingAndWrite(
	c *gin.Context,
	reqLog *zap.Logger,
	protocol pricingErrorProtocol,
	model string,
	validate func() error,
) bool {
	if validate == nil {
		return true
	}
	err := validate()
	if err == nil {
		return true
	}

	status, errType, code, message := requestedPricingErrorDetails(model, err)
	markRequestedPricingOpsContext(c, code)
	logRequestedPricingValidationFailure(reqLog, model, code, err)
	writeRequestedPricingHTTPError(c, protocol, status, errType, code, message)
	return false
}

func writeRequestedPricingHTTPError(
	c *gin.Context,
	protocol pricingErrorProtocol,
	status int,
	errType, code, message string,
) {
	switch protocol {
	case pricingErrorProtocolAnthropic:
		errorBody := gin.H{
			"type":    errType,
			"message": message,
		}
		if code != "" {
			errorBody["code"] = code
		}
		c.JSON(status, gin.H{
			"type":  "error",
			"error": errorBody,
		})
	case pricingErrorProtocolGemini:
		googleErrorWithReason(c, status, message, code)
	default:
		errorBody := gin.H{
			"type":    errType,
			"message": message,
		}
		if code != "" {
			errorBody["code"] = code
		}
		c.JSON(status, gin.H{"error": errorBody})
	}
}

func writeAnthropicRequestedPricingStreamError(c *gin.Context, status int, errType, code, message string) {
	if c == nil || c.Writer == nil {
		return
	}
	service.MarkOpsStreamError(c, errType, message, status)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	errorBody := gin.H{"type": errType, "message": message}
	if code != "" {
		errorBody["code"] = code
	}
	payload, err := json.Marshal(gin.H{"type": "error", "error": errorBody})
	if err != nil {
		_ = c.Error(err)
		return
	}
	if _, err := fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", payload); err != nil {
		_ = c.Error(err)
		return
	}
	flusher.Flush()
}

func validateRequestedPricingForWS(
	ctx context.Context,
	c *gin.Context,
	conn *coderws.Conn,
	reqLog *zap.Logger,
	model string,
	validate func() error,
) error {
	if validate == nil {
		return nil
	}
	err := validate()
	if err == nil {
		return nil
	}

	_, errType, code, message := requestedPricingErrorDetails(model, err)
	markRequestedPricingOpsContext(c, code)
	logRequestedPricingValidationFailure(reqLog, model, code, err)
	writeRequestedPricingWSError(ctx, conn, errType, code, message)
	return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, message, err)
}

func markRequestedPricingOpsContext(c *gin.Context, code string) {
	if code != modelPricingNotFoundCode {
		return
	}
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonModelPricingNotFound)
}

func requestedPricingErrorDetails(model string, err error) (status int, errType, code, message string) {
	model = strings.TrimSpace(model)
	var specErr *invalidRequestedPricingSpecError
	if errors.As(err, &specErr) {
		return http.StatusBadRequest, "invalid_request_error", "", specErr.Error()
	}
	if errors.Is(err, service.ErrModelPricingUnavailable) {
		message = "Pricing is not configured for requested model"
		if model != "" {
			message = fmt.Sprintf("Pricing is not configured for requested model %q", model)
		}
		return http.StatusBadRequest, "invalid_request_error", modelPricingNotFoundCode, message
	}
	return http.StatusInternalServerError, "api_error", "", "Unable to validate pricing for requested model"
}

func logRequestedPricingValidationFailure(reqLog *zap.Logger, model, code string, err error) {
	if reqLog == nil {
		return
	}
	fields := []zap.Field{
		zap.String("requested_pricing_model", strings.TrimSpace(model)),
		zap.Error(err),
	}
	if code != "" {
		fields = append(fields, zap.String("error_code", code))
		reqLog.Info("gateway.requested_model_pricing_rejected", fields...)
		return
	}
	reqLog.Error("gateway.requested_model_pricing_validation_failed", fields...)
}

func writeRequestedPricingWSError(ctx context.Context, conn *coderws.Conn, errType, code, message string) {
	if conn == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	payload := gin.H{
		"event_id": "evt_model_pricing_not_found",
		"type":     "error",
		"error": gin.H{
			"type":    errType,
			"code":    code,
			"message": message,
		},
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, coderws.MessageText, marshalRequestedPricingWSError(payload))
}

func marshalRequestedPricingWSError(value any) []byte {
	payload, err := json.Marshal(value)
	if err == nil {
		return payload
	}
	return []byte(`{"event_id":"evt_model_pricing_not_found","type":"error","error":{"type":"invalid_request_error","code":"MODEL_PRICING_NOT_FOUND","message":"Pricing is not configured for requested model"}}`)
}
