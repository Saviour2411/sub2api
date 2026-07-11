package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRequestedImagePricingSpec(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		requestedModel string
		body           string
		wantModel      string
		wantSize       string
		wantImage      bool
	}{
		{
			name:           "responses 图片工具使用显式媒体模型和尺寸",
			endpoint:       "/v1/responses",
			requestedModel: "gpt-5.6-sol",
			body:           `{"model":"gpt-5.6-sol","tools":[{"type":"image_generation","model":"gpt-image-2","size":"2048x2048"}]}`,
			wantModel:      "gpt-image-2",
			wantSize:       service.ImageBillingSize2K,
			wantImage:      true,
		},
		{
			name:           "responses 图片工具使用现有默认媒体模型",
			endpoint:       "/v1/responses",
			requestedModel: "gpt-5.6-sol",
			body:           `{"model":"gpt-5.6-sol","tools":[{"type":"image_generation"}]}`,
			wantModel:      "gpt-image-2",
			wantSize:       service.ImageBillingSize2K,
			wantImage:      true,
		},
		{
			name:           "Gemini 图片模型读取原生尺寸",
			endpoint:       "/v1beta/models/gemini-3-pro-image:generateContent",
			requestedModel: "gemini-3-pro-image",
			body:           `{"generationConfig":{"imageConfig":{"imageSize":"4K"}}}`,
			wantModel:      "gemini-3-pro-image",
			wantSize:       service.ImageBillingSize4K,
			wantImage:      true,
		},
		{
			name:           "文本请求保持请求模型",
			endpoint:       "/v1/chat/completions",
			requestedModel: "gpt-5.6-sol",
			body:           `{"model":"gpt-5.6-sol"}`,
			wantModel:      "gpt-5.6-sol",
			wantImage:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, size, image, err := requestedImagePricingSpec(tt.endpoint, tt.requestedModel, []byte(tt.body))
			require.NoError(t, err)
			require.Equal(t, tt.wantModel, model)
			require.Equal(t, tt.wantSize, size)
			require.Equal(t, tt.wantImage, image)
		})
	}
}

func TestRequestedPricingSpecs_ResponsesOptionalImageAlsoValidatesText(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantKinds []service.PricingUsageKind
		wantModel []string
	}{
		{
			name:      "可选图片工具覆盖文本和图片路径",
			body:      `{"model":"gpt-5.6-sol","tools":[{"type":"image_generation","model":"gpt-image-2"}],"tool_choice":"auto"}`,
			wantKinds: []service.PricingUsageKind{service.PricingUsageToken, service.PricingUsageImage},
			wantModel: []string{"gpt-5.6-sol", "gpt-image-2"},
		},
		{
			name:      "明确强制图片只校验图片路径",
			body:      `{"model":"gpt-5.6-sol","tools":[{"type":"image_generation","model":"gpt-image-2"}],"tool_choice":{"type":"image_generation"}}`,
			wantKinds: []service.PricingUsageKind{service.PricingUsageImage},
			wantModel: []string{"gpt-image-2"},
		},
		{
			name:      "required 且唯一图片工具只校验图片路径",
			body:      `{"model":"gpt-5.6-sol","tools":[{"type":"image_generation","model":"gpt-image-2"}],"tool_choice":"required"}`,
			wantKinds: []service.PricingUsageKind{service.PricingUsageImage},
			wantModel: []string{"gpt-image-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs, err := requestedPricingSpecs("/v1/responses", "gpt-5.6-sol", []byte(tt.body))
			require.NoError(t, err)
			kinds := make([]service.PricingUsageKind, 0, len(specs))
			models := make([]string, 0, len(specs))
			for _, spec := range specs {
				kinds = append(kinds, spec.kind)
				models = append(models, spec.model)
			}
			require.Equal(t, tt.wantKinds, kinds)
			require.Equal(t, tt.wantModel, models)
		})
	}
}

func TestValidateRequestedPricingAndWrite_MissingPriceProtocols(t *testing.T) {
	gin.SetMode(gin.TestMode)
	missingErr := fmt.Errorf("wrapped: %w", service.ErrModelPricingUnavailable)

	tests := []struct {
		name         string
		protocol     pricingErrorProtocol
		wantType     string
		wantHTTPCode any
		wantReason   string
	}{
		{
			name:         "OpenAI 返回稳定业务码",
			protocol:     pricingErrorProtocolOpenAI,
			wantType:     "invalid_request_error",
			wantHTTPCode: "MODEL_PRICING_NOT_FOUND",
		},
		{
			name:         "Anthropic 在协议错误对象中返回业务码",
			protocol:     pricingErrorProtocolAnthropic,
			wantType:     "invalid_request_error",
			wantHTTPCode: "MODEL_PRICING_NOT_FOUND",
		},
		{
			name:         "Gemini 保持数字状态码并通过 ErrorInfo 返回业务码",
			protocol:     pricingErrorProtocolGemini,
			wantType:     "",
			wantHTTPCode: float64(http.StatusBadRequest),
			wantReason:   "MODEL_PRICING_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", nil)

			ok := validateRequestedPricingAndWrite(c, nil, tt.protocol, "gpt-5.6-sol", func() error {
				return missingErr
			})

			require.False(t, ok)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.True(t, service.HasOpsClientBusinessLimited(c))
			reason, exists := c.Get(service.OpsClientBusinessLimitedReasonKey)
			require.True(t, exists)
			require.Equal(t, service.OpsClientBusinessLimitedReasonModelPricingNotFound, reason)
			_, hasUpstreamStatus := c.Get(service.OpsUpstreamStatusCodeKey)
			_, hasUpstreamMessage := c.Get(service.OpsUpstreamErrorMessageKey)
			require.False(t, hasUpstreamStatus)
			require.False(t, hasUpstreamMessage)
			require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String(), "gpt-5.6-sol")
			if tt.wantType != "" {
				require.Equal(t, tt.wantType, gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
			}
			code := gjson.GetBytes(recorder.Body.Bytes(), "error.code")
			if tt.wantHTTPCode == nil {
				require.False(t, code.Exists())
			} else if wantString, ok := tt.wantHTTPCode.(string); ok {
				require.Equal(t, wantString, code.String())
			} else {
				require.Equal(t, int64(http.StatusBadRequest), code.Int())
			}
			if tt.wantReason != "" {
				require.Equal(t, tt.wantReason, gjson.GetBytes(recorder.Body.Bytes(), "error.details.0.reason").String())
			}
		})
	}
}

func TestRequestedPricingValidation_SimpleModeSkipsBeforeMediaResolution(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	h := &OpenAIGatewayHandler{cfg: &config.Config{RunMode: config.RunModeSimple}}

	model, err := h.openAIRequestPricingValidationError(
		c.Request.Context(), nil, "/v1/responses", "unpriced-model", []byte(`{"model":"unpriced-model"}`),
	)
	require.NoError(t, err)
	require.Empty(t, model)

	require.True(t, h.validateOpenAIPricingSpec(
		c, nil, nil, pricingErrorProtocolOpenAI, "gpt-image-2",
		service.PricingUsageImage, service.ImageBillingSize2K, errors.New("invalid image pricing parameters"),
	))
	require.Empty(t, recorder.Body.String())
}

func TestValidateRequestedPricingAndWrite_InvalidMediaParametersAreNotMissingPricing(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	ok := validateRequestedPricingAndWrite(c, nil, pricingErrorProtocolOpenAI, "gpt-image-2", func() error {
		return &invalidRequestedPricingSpecError{err: errors.New("invalid image size")}
	})

	require.False(t, ok)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
	require.False(t, gjson.GetBytes(recorder.Body.Bytes(), "error.code").Exists())
	require.False(t, service.HasOpsClientBusinessLimited(c))
}

func TestWriteAnthropicRequestedPricingStreamError_IncludesBusinessCode(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	writeAnthropicRequestedPricingStreamError(
		c, http.StatusBadRequest, "invalid_request_error", modelPricingNotFoundCode,
		"Pricing is not configured for requested model",
	)

	require.Contains(t, recorder.Body.String(), "event: error")
	require.Contains(t, recorder.Body.String(), `"code":"MODEL_PRICING_NOT_FOUND"`)
	_, marked := service.GetOpsStreamError(c)
	require.True(t, marked)
}

func TestValidateRequestedPricingForWS_MarksLocalBusinessRejection(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)

	err := validateRequestedPricingForWS(c.Request.Context(), c, nil, nil, "gpt-5.6-sol", func() error {
		return service.ErrModelPricingUnavailable
	})

	require.Error(t, err)
	var closeErr *service.OpenAIWSClientCloseError
	require.ErrorAs(t, err, &closeErr)
	require.Equal(t, coderws.StatusPolicyViolation, closeErr.StatusCode())
	require.True(t, service.HasOpsClientBusinessLimited(c))
	reason, exists := c.Get(service.OpsClientBusinessLimitedReasonKey)
	require.True(t, exists)
	require.Equal(t, service.OpsClientBusinessLimitedReasonModelPricingNotFound, reason)
	_, hasUpstreamStatus := c.Get(service.OpsUpstreamStatusCodeKey)
	_, hasUpstreamMessage := c.Get(service.OpsUpstreamErrorMessageKey)
	require.False(t, hasUpstreamStatus)
	require.False(t, hasUpstreamMessage)

	payload := marshalRequestedPricingWSError(gin.H{
		"event_id": "evt_model_pricing_not_found",
		"type":     "error",
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    "MODEL_PRICING_NOT_FOUND",
			"message": "Pricing is not configured for requested model",
		},
	})
	require.Equal(t, "error", gjson.GetBytes(payload, "type").String())
	require.Equal(t, "MODEL_PRICING_NOT_FOUND", gjson.GetBytes(payload, "error.code").String())
}

func TestOpenAIChatValidateRequestedModelPricingBeforeConcurrencyAndRPM(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var moderationCalls atomic.Int32
	moderationServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		moderationCalls.Add(1)
		_, _ = w.Write([]byte(`{"results":[{"category_scores":{}}]}`))
	}))
	defer moderationServer.Close()
	moderationConfig, err := json.Marshal(&service.ContentModerationConfig{
		Enabled: true, Mode: service.ContentModerationModePreBlock, BaseURL: moderationServer.URL,
		Model: "omni-moderation-latest", APIKeys: []string{"sk-test"}, SampleRate: 100, AllGroups: true,
	})
	require.NoError(t, err)
	moderationService := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled:      "true",
			service.SettingKeyContentModerationConfig: string(moderationConfig),
		}}, nil, nil, nil, nil, nil, nil,
	)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"unpriced-request-model","messages":[{"role":"user","content":"hello"}]}`,
	))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(7)
	apiKey := &service.APIKey{
		ID:      11,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
		User:    &service.User{ID: 13},
	}
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 13, Concurrency: 1})

	// 底层缓存故意保持 nil：价格校验如果晚于并发或 RPM 检查，此处会进入无效依赖。
	h := &OpenAIGatewayHandler{
		gatewayService:           &service.OpenAIGatewayService{},
		billingCacheService:      &service.BillingCacheService{},
		apiKeyService:            &service.APIKeyService{},
		contentModerationService: moderationService,
		concurrencyHelper:        NewConcurrencyHelper(service.NewConcurrencyService(nil), SSEPingFormatNone, 0),
	}
	require.NotPanics(t, func() { h.ChatCompletions(c) })

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "MODEL_PRICING_NOT_FOUND", gjson.GetBytes(recorder.Body.Bytes(), "error.code").String())
	require.True(t, service.HasOpsClientBusinessLimited(c))
	require.Zero(t, moderationCalls.Load())
	_, selectedAccount := c.Get(opsAccountIDKey)
	require.False(t, selectedAccount)
}

func TestValidateRequestedPricingAndWrite_SuccessDoesNotWriteResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", nil)

	require.True(t, validateRequestedPricingAndWrite(c, nil, pricingErrorProtocolOpenAI, "gpt-5.6-sol", func() error { return nil }))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Body.String())
}
