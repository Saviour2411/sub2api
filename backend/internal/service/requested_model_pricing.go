package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// PricingUsageKind 表示请求前校验所针对的计费路径。
type PricingUsageKind string

const (
	PricingUsageToken PricingUsageKind = "token"
	PricingUsageImage PricingUsageKind = "image"
	PricingUsageVideo PricingUsageKind = "video"
)

// ValidateRequestedModelPricing 确保用户请求模型在转发前具备可用价格。
func (s *GatewayService) ValidateRequestedModelPricing(
	ctx context.Context,
	apiKey *APIKey,
	model string,
	kind PricingUsageKind,
	sizeTier string,
) error {
	if s == nil {
		return requestedModelPricingUnavailable(model)
	}
	_, err := resolveRequestedModelPricing(ctx, s.cfg, s.resolver, s.billingService, apiKey, model, kind, sizeTier)
	return err
}

// ValidateRequestedModelPricing 确保用户请求模型在转发前具备可用价格。
func (s *OpenAIGatewayService) ValidateRequestedModelPricing(
	ctx context.Context,
	apiKey *APIKey,
	model string,
	kind PricingUsageKind,
	sizeTier string,
) error {
	if s == nil {
		return requestedModelPricingUnavailable(model)
	}
	_, err := resolveRequestedModelPricing(ctx, s.cfg, s.resolver, s.billingService, apiKey, model, kind, sizeTier)
	return err
}

// resolveRequestedModelPricing 返回请求模型自身可计费的规范名称。候选只由请求模型
// 自身生成，绝不包含账号映射、渠道映射或最终上游模型。
func resolveRequestedModelPricing(
	ctx context.Context,
	cfg *config.Config,
	resolver *ModelPricingResolver,
	billingService *BillingService,
	apiKey *APIKey,
	model string,
	kind PricingUsageKind,
	sizeTier string,
) (string, error) {
	requestedModel := strings.TrimSpace(model)
	if cfg != nil && cfg.RunMode == config.RunModeSimple {
		return requestedModel, nil
	}
	if requestedModel == "" || billingService == nil {
		return "", requestedModelPricingUnavailable(requestedModel)
	}

	switch kind {
	case PricingUsageImage, PricingUsageVideo:
		return resolveRequestedMediaPricing(ctx, resolver, billingService, apiKey, requestedModel, kind, sizeTier)
	case PricingUsageToken:
		// 继续执行 token 定价解析。
	default:
		return "", fmt.Errorf("unsupported pricing usage kind %q: %w", kind, ErrModelPricingUnavailable)
	}

	candidates := usageBillingModelCandidates(requestedModel)
	groupID := pricingGroupID(apiKey)
	if resolver != nil {
		type candidatePricing struct {
			model    string
			resolved *ResolvedPricing
		}
		resolvedCandidates := make([]candidatePricing, 0, len(candidates))
		for _, candidate := range candidates {
			resolvedCandidates = append(resolvedCandidates, candidatePricing{
				model:    candidate,
				resolved: resolver.Resolve(ctx, PricingInput{Model: candidate, GroupID: groupID}),
			})
		}
		// 请求字面名及其规范别名都属于用户请求模型。渠道显式销售价格优先于
		// 任一候选的全局价格，避免裸别名先命中全局价后跳过 canonical 渠道价。
		for _, candidate := range resolvedCandidates {
			if candidate.resolved != nil && candidate.resolved.ChannelPricingConfigured && hasResolvedTokenPricing(candidate.resolved) {
				return candidate.model, nil
			}
		}
		for _, candidate := range resolvedCandidates {
			if hasResolvedTokenPricing(candidate.resolved) {
				return candidate.model, nil
			}
		}
		return "", requestedModelPricingUnavailable(requestedModel)
	}

	for _, candidate := range candidates {
		if _, err := billingService.GetModelPricing(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", requestedModelPricingUnavailable(requestedModel)
}

func resolveRequestedMediaPricing(
	ctx context.Context,
	resolver *ModelPricingResolver,
	billingService *BillingService,
	apiKey *APIKey,
	model string,
	kind PricingUsageKind,
	sizeTier string,
) (string, error) {
	if kind == PricingUsageImage {
		sizeTier = NormalizeImageBillingTierOrDefault(sizeTier)
		if apiKeyHasConfiguredImagePrice(apiKey, sizeTier) {
			return model, nil
		}
	} else {
		sizeTier = NormalizeVideoBillingResolutionOrDefault(sizeTier)
		if apiKeyHasConfiguredVideoPrice(apiKey, sizeTier) {
			return model, nil
		}
	}

	// 媒体后扣只会对当前实际媒体模型查询渠道价。渠道 token 模式会接管
	// 媒体计费，因此必须具备完整 token 价格；按次/图片模式则校验本次档位。
	if resolver != nil {
		resolved := resolver.Resolve(ctx, PricingInput{Model: model, GroupID: pricingGroupID(apiKey)})
		if resolved != nil && resolved.ChannelPricingConfigured {
			switch resolved.Mode {
			case BillingModePerRequest, BillingModeImage:
				if hasResolvedMediaTierPricing(resolved, sizeTier) {
					return model, nil
				}
				// 当前档位未配置时，媒体后扣会回退到模型默认价格。
			default:
				if hasResolvedTokenPricing(resolved) {
					return model, nil
				}
				return "", requestedModelPricingUnavailable(model)
			}
		}
	}

	// 图片和视频保留现有模型默认价格兜底；显式渠道/分组零价已在上面识别。
	if kind == PricingUsageImage {
		if billingService.getDefaultImagePrice(model, sizeTier) > 0 {
			return model, nil
		}
	} else if billingService.getDefaultVideoPrice(model, sizeTier) > 0 {
		return model, nil
	}
	return "", requestedModelPricingUnavailable(model)
}

func pricingGroupID(apiKey *APIKey) *int64 {
	if apiKey == nil {
		return nil
	}
	if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		return apiKey.GroupID
	}
	if apiKey.Group != nil && apiKey.Group.ID > 0 {
		groupID := apiKey.Group.ID
		return &groupID
	}
	return nil
}

func hasResolvedTokenPricing(resolved *ResolvedPricing) bool {
	if resolved == nil {
		return false
	}
	switch resolved.Mode {
	case BillingModePerRequest, BillingModeImage:
		if resolved.DefaultPerRequestPriceConfigured || resolved.DefaultPerRequestPrice != 0 {
			return true
		}
		return requestIntervalsCoverAllContexts(resolved.RequestTiers)
	default:
		if !resolved.TokenPricingConfigured {
			return false
		}
		return resolved.BasePricing != nil || tokenIntervalsCoverAllContexts(resolved.Intervals)
	}
}

func hasResolvedMediaTierPricing(resolved *ResolvedPricing, sizeTier string) bool {
	if resolved == nil {
		return false
	}
	for i := range resolved.RequestTiers {
		tier := &resolved.RequestTiers[i]
		if tier.TierLabel == sizeTier && tier.PerRequestPrice != nil {
			return true
		}
	}
	return resolved.DefaultPerRequestPriceConfigured || resolved.DefaultPerRequestPrice != 0
}

func requestIntervalsCoverAllContexts(intervals []PricingInterval) bool {
	priced := make([]PricingInterval, 0, len(intervals))
	for _, interval := range intervals {
		if interval.PerRequestPrice != nil {
			priced = append(priced, interval)
		}
	}
	return tokenIntervalsCoverAllContexts(priced)
}

func tokenIntervalsCoverAllContexts(intervals []PricingInterval) bool {
	if len(intervals) == 0 {
		return false
	}
	sorted := append([]PricingInterval(nil), intervals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].MinTokens < sorted[j].MinTokens })
	if sorted[0].MinTokens != 0 {
		return false
	}
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].MaxTokens == nil || *sorted[i-1].MaxTokens != sorted[i].MinTokens {
			return false
		}
	}
	return sorted[len(sorted)-1].MaxTokens == nil
}

func requestedModelPricingUnavailable(model string) error {
	return fmt.Errorf("%w for requested model: %s", ErrModelPricingUnavailable, strings.TrimSpace(model))
}
