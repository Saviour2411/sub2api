package handler

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelMarketplaceHandler 处理公开模型广场查询。
//
// 该接口无需登录，只返回分组、模型和请求格式的白名单字段，不暴露账号、
// 渠道、价格、内部路由和配额等管理信息。
type ModelMarketplaceHandler struct {
	settingService *service.SettingService
	groupRepo      service.GroupRepository
	gatewayService *service.GatewayService
}

// NewModelMarketplaceHandler 创建公开模型广场 handler。
func NewModelMarketplaceHandler(
	settingService *service.SettingService,
	groupRepo service.GroupRepository,
	gatewayService *service.GatewayService,
) *ModelMarketplaceHandler {
	return &ModelMarketplaceHandler{
		settingService: settingService,
		groupRepo:      groupRepo,
		gatewayService: gatewayService,
	}
}

type modelMarketplaceResponse struct {
	Enabled     bool                    `json:"enabled"`
	Intro       string                  `json:"intro"`
	GeneratedAt string                  `json:"generated_at"`
	Groups      []modelMarketplaceGroup `json:"groups"`
}

type modelMarketplaceGroup struct {
	ID               int64                           `json:"id"`
	Name             string                          `json:"name"`
	Description      string                          `json:"description"`
	Platform         string                          `json:"platform"`
	SubscriptionType string                          `json:"subscription_type"`
	IsExclusive      bool                            `json:"is_exclusive"`
	Models           []string                        `json:"models"`
	RequestFormats   []modelMarketplaceRequestFormat `json:"request_formats"`
}

type modelMarketplaceRequestFormat struct {
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	ContentType string `json:"content_type,omitempty"`
	Body        string `json:"body,omitempty"`
}

// List 返回公开模型广场数据。
// GET /api/v1/model-marketplace
func (h *ModelMarketplaceHandler) List(c *gin.Context) {
	runtime := h.settingService.GetModelMarketplaceRuntime(c.Request.Context())
	out := modelMarketplaceResponse{
		Enabled:     runtime.Enabled,
		Intro:       runtime.Intro,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Groups:      []modelMarketplaceGroup{},
	}
	if !runtime.Enabled {
		response.Success(c, out)
		return
	}

	groups, err := h.groupRepo.ListActive(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	configured := make(map[int64]struct{}, len(runtime.GroupIDs))
	for _, id := range runtime.GroupIDs {
		configured[id] = struct{}{}
	}
	useConfiguredGroups := len(configured) > 0

	for i := range groups {
		group := groups[i]
		if !modelMarketplaceGroupVisible(group, configured, useConfiguredGroups) {
			continue
		}
		models := h.modelsForGroup(c, group)
		out.Groups = append(out.Groups, modelMarketplaceGroup{
			ID:               group.ID,
			Name:             group.Name,
			Description:      group.Description,
			Platform:         group.Platform,
			SubscriptionType: group.SubscriptionType,
			IsExclusive:      group.IsExclusive,
			Models:           models,
			RequestFormats:   requestFormatsForGroup(group),
		})
	}

	response.Success(c, out)
}

func modelMarketplaceGroupVisible(group service.Group, configured map[int64]struct{}, useConfiguredGroups bool) bool {
	if group.Status != service.StatusActive {
		return false
	}
	if useConfiguredGroups {
		_, ok := configured[group.ID]
		return ok
	}
	return !group.IsExclusive && group.SubscriptionType != service.SubscriptionTypeSubscription
}

func (h *ModelMarketplaceHandler) modelsForGroup(c *gin.Context, group service.Group) []string {
	groupID := group.ID
	models := h.gatewayService.GetAvailableModels(c.Request.Context(), &groupID, group.Platform)
	if group.CustomModelsListEnabled() {
		return filterModelsByCustomList(models, defaultModelIDsForPlatform(group.Platform), group.ModelsListConfig.Models)
	}
	if len(models) > 0 {
		return cloneModelMarketplaceModels(models)
	}
	return cloneModelMarketplaceModels(defaultModelIDsForPlatform(group.Platform))
}

func cloneModelMarketplaceModels(models []string) []string {
	out := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func requestFormatsForGroup(group service.Group) []modelMarketplaceRequestFormat {
	switch group.Platform {
	case service.PlatformOpenAI:
		formats := []modelMarketplaceRequestFormat{
			{
				Name:        "OpenAI Chat Completions",
				Method:      "POST",
				Path:        "/v1/chat/completions",
				ContentType: "application/json",
				Body:        "{\n  \"model\": \"{model}\",\n  \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],\n  \"stream\": true\n}",
			},
			{
				Name:        "OpenAI Responses",
				Method:      "POST",
				Path:        "/v1/responses",
				ContentType: "application/json",
				Body:        "{\n  \"model\": \"{model}\",\n  \"input\": \"Hello\",\n  \"stream\": true\n}",
			},
		}
		if group.AllowMessagesDispatch {
			formats = append(formats, modelMarketplaceRequestFormat{
				Name:        "Anthropic Messages",
				Method:      "POST",
				Path:        "/v1/messages",
				ContentType: "application/json",
				Body:        "{\n  \"model\": \"{model}\",\n  \"max_tokens\": 1024,\n  \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],\n  \"stream\": true\n}",
			})
		}
		return formats
	case service.PlatformGemini:
		return []modelMarketplaceRequestFormat{
			{
				Name:        "Gemini Generate Content",
				Method:      "POST",
				Path:        "/v1beta/models/{model}:generateContent",
				ContentType: "application/json",
				Body:        "{\n  \"contents\": [{\"parts\": [{\"text\": \"Hello\"}]}]\n}",
			},
		}
	case service.PlatformAntigravity:
		return []modelMarketplaceRequestFormat{
			{
				Name:        "Antigravity Messages",
				Method:      "POST",
				Path:        "/antigravity/v1/messages",
				ContentType: "application/json",
				Body:        "{\n  \"model\": \"{model}\",\n  \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],\n  \"stream\": true\n}",
			},
			{
				Name:        "Antigravity Gemini Generate Content",
				Method:      "POST",
				Path:        "/antigravity/v1beta/models/{model}:generateContent",
				ContentType: "application/json",
				Body:        "{\n  \"contents\": [{\"parts\": [{\"text\": \"Hello\"}]}]\n}",
			},
		}
	default:
		return []modelMarketplaceRequestFormat{
			{
				Name:        "Anthropic Messages",
				Method:      "POST",
				Path:        "/v1/messages",
				ContentType: "application/json",
				Body:        "{\n  \"model\": \"{model}\",\n  \"max_tokens\": 1024,\n  \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}],\n  \"stream\": true\n}",
			},
		}
	}
}
