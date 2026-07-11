package service

import (
	"context"
	"log/slog"
)

// PricingSource 定价来源标识
const (
	PricingSourceChannel  = "channel"
	PricingSourceLiteLLM  = "litellm"
	PricingSourceFallback = "fallback"
)

// ResolvedPricing 统一定价解析结果
type ResolvedPricing struct {
	// Mode 计费模式
	Mode BillingMode

	// Token 模式：基础定价（来自 LiteLLM 或 fallback）
	BasePricing *ModelPricing
	// TokenPricingConfigured 表示至少存在一项真实 token 价格配置；显式 0 元也算配置。
	TokenPricingConfigured bool

	// Token 模式：区间定价列表（如有，覆盖 BasePricing 中的对应字段）
	Intervals []PricingInterval

	// 按次/图片模式：分层定价
	RequestTiers []PricingInterval

	// 按次/图片模式：默认价格（未命中层级时使用）
	DefaultPerRequestPrice float64
	// DefaultPerRequestPriceConfigured 区分显式 0 元与未配置。
	DefaultPerRequestPriceConfigured bool

	// ChannelPricingConfigured 表示渠道确实提供了当前计费模式可用的价格。
	// 仅查询到空渠道条目时为 false；显式 0 元和渠道默认价格均为 true。
	ChannelPricingConfigured bool

	// 来源标识
	Source string // "channel", "litellm", "fallback"

	// 是否支持缓存细分
	SupportsCacheBreakdown bool

	// 渠道定价原始配置（用于区间模式下获取 ImageOutputPrice）
	channelPricing *ChannelModelPricing
}

// ModelPricingResolver 统一模型定价解析器。
// 解析链：Channel → LiteLLM → Fallback。
type ModelPricingResolver struct {
	channelService *ChannelService
	billingService *BillingService
}

// NewModelPricingResolver 创建定价解析器实例
func NewModelPricingResolver(channelService *ChannelService, billingService *BillingService) *ModelPricingResolver {
	return &ModelPricingResolver{
		channelService: channelService,
		billingService: billingService,
	}
}

// PricingInput 定价解析输入
type PricingInput struct {
	Model   string
	GroupID *int64 // nil 表示不检查渠道
}

// Resolve 解析模型定价。
// 1. 获取基础定价（LiteLLM → Fallback）
// 2. 如果指定了 GroupID，查找渠道定价并覆盖
func (r *ModelPricingResolver) Resolve(ctx context.Context, input PricingInput) *ResolvedPricing {
	var chPricing *ChannelModelPricing
	if input.GroupID != nil && r.channelService != nil {
		chPricing = r.channelService.GetChannelModelPricing(ctx, *input.GroupID, input.Model)
		if chPricing != nil {
			mode := chPricing.BillingMode
			if mode == "" {
				mode = BillingModeToken
			}
			if mode == BillingModePerRequest || mode == BillingModeImage {
				resolved := &ResolvedPricing{
					Mode:           mode,
					Source:         PricingSourceChannel,
					channelPricing: chPricing,
				}
				r.applyRequestTierOverrides(chPricing, resolved)
				if resolved.ChannelPricingConfigured {
					return resolved
				}
				// 空的按次/图片条目不应覆盖模型已有的 token 定价。
				chPricing = nil
			}
		}
	}

	// 1. 获取基础定价
	basePricing, source := r.resolveBasePricing(input.Model)
	if basePricing == nil && input.GroupID != nil && r.channelService != nil {
		if defaultPricing := r.channelService.GetChannelDefaultPricing(ctx, *input.GroupID); defaultPricing != nil {
			if modelPricing := defaultPricing.ToModelPricing(); modelPricing != nil {
				basePricing = modelPricing
				source = PricingSourceChannel
			}
		}
	}

	resolved := &ResolvedPricing{
		Mode:                     BillingModeToken,
		BasePricing:              basePricing,
		TokenPricingConfigured:   basePricing != nil,
		Source:                   source,
		SupportsCacheBreakdown:   basePricing != nil && basePricing.SupportsCacheBreakdown,
		ChannelPricingConfigured: source == PricingSourceChannel,
	}

	// 2. 如果有 GroupID，尝试渠道覆盖
	if chPricing != nil {
		resolved.channelPricing = chPricing
		r.applyTokenOverrides(chPricing, resolved)
	} else if input.GroupID != nil {
		r.applyChannelOverrides(ctx, *input.GroupID, input.Model, resolved)
	}

	return resolved
}

// resolveBasePricing 从 LiteLLM 或 Fallback 获取基础定价
func (r *ModelPricingResolver) resolveBasePricing(model string) (*ModelPricing, string) {
	pricing, err := r.billingService.GetModelPricing(model)
	if err != nil {
		slog.Debug("failed to get model pricing from LiteLLM, using fallback",
			"model", model, "error", err)
		return nil, PricingSourceFallback
	}
	return pricing, PricingSourceLiteLLM
}

// applyChannelOverrides 应用渠道定价覆盖
func (r *ModelPricingResolver) applyChannelOverrides(ctx context.Context, groupID int64, model string, resolved *ResolvedPricing) {
	chPricing := r.channelService.GetChannelModelPricing(ctx, groupID, model)
	if chPricing == nil {
		return
	}

	mode := chPricing.BillingMode
	if mode == "" {
		mode = BillingModeToken
	}

	switch mode {
	case BillingModeToken:
		resolved.channelPricing = chPricing
		r.applyTokenOverrides(chPricing, resolved)
	case BillingModePerRequest, BillingModeImage:
		channelResolved := &ResolvedPricing{
			Mode:           mode,
			Source:         PricingSourceChannel,
			channelPricing: chPricing,
		}
		r.applyRequestTierOverrides(chPricing, channelResolved)
		if channelResolved.ChannelPricingConfigured {
			*resolved = *channelResolved
		}
	}
}

// applyTokenOverrides 应用 token 模式的渠道覆盖
func (r *ModelPricingResolver) applyTokenOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	validIntervals := filterValidTokenIntervals(chPricing.Intervals)
	hasFlatTokenPricing := chPricing.InputPrice != nil || chPricing.OutputPrice != nil ||
		chPricing.CacheWritePrice != nil || chPricing.CacheReadPrice != nil
	hasAnyOverride := hasFlatTokenPricing || chPricing.ImageOutputPrice != nil

	// 空渠道条目不应把未知模型伪装成一个全零价格模型。已有全局价格时也无需覆盖。
	if len(validIntervals) == 0 && !hasAnyOverride {
		return
	}

	// 如果有有效的区间定价，使用区间
	if len(validIntervals) > 0 {
		resolved.Source = PricingSourceChannel
		resolved.ChannelPricingConfigured = true
		resolved.TokenPricingConfigured = true
		resolved.Intervals = validIntervals
		// 区间不匹配时回退到 BasePricing，也需要覆盖图片价格
		if resolved.BasePricing != nil {
			// 防止修改 fallbackPrices 中的共享指针
			cloned := *resolved.BasePricing
			resolved.BasePricing = &cloned
			if chPricing.ImageOutputPrice != nil {
				resolved.BasePricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
			} else {
				resolved.BasePricing.ImageOutputPricePerToken = 0
			}
			resolved.BasePricing.ImageOutputPriceExplicit = true
		}
		return
	}
	// 仅配置图片输出 token 价格不能让原本未知的文本模型变成可计费模型。
	if resolved.BasePricing == nil && !hasFlatTokenPricing {
		return
	}
	resolved.Source = PricingSourceChannel
	resolved.ChannelPricingConfigured = true
	if hasFlatTokenPricing {
		resolved.TokenPricingConfigured = true
	}

	// 否则用 flat 字段覆盖 BasePricing
	if resolved.BasePricing == nil {
		resolved.BasePricing = &ModelPricing{}
	} else {
		// 防止修改 fallbackPrices 中的共享指针
		cloned := *resolved.BasePricing
		resolved.BasePricing = &cloned
	}

	if chPricing.InputPrice != nil {
		resolved.BasePricing.InputPricePerToken = *chPricing.InputPrice
		resolved.BasePricing.InputPricePerTokenPriority = *chPricing.InputPrice
	}
	if chPricing.OutputPrice != nil {
		resolved.BasePricing.OutputPricePerToken = *chPricing.OutputPrice
		resolved.BasePricing.OutputPricePerTokenPriority = *chPricing.OutputPrice
	}
	if chPricing.CacheWritePrice != nil {
		resolved.BasePricing.CacheCreationPricePerToken = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreationPricePerTokenPriority = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreationPriceExplicit = true
		resolved.BasePricing.CacheCreation5mPrice = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreation1hPrice = *chPricing.CacheWritePrice
	}
	if chPricing.CacheReadPrice != nil {
		resolved.BasePricing.CacheReadPricePerToken = *chPricing.CacheReadPrice
		resolved.BasePricing.CacheReadPricePerTokenPriority = *chPricing.CacheReadPrice
	}
	// 渠道定价覆盖一切：显式配置则用配置值，未配置则归零（不回退到 LiteLLM）
	if chPricing.ImageOutputPrice != nil {
		resolved.BasePricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
	} else {
		resolved.BasePricing.ImageOutputPricePerToken = 0
	}
	resolved.BasePricing.ImageOutputPriceExplicit = true
}

// applyRequestTierOverrides 应用按次/图片模式的渠道覆盖
func (r *ModelPricingResolver) applyRequestTierOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	resolved.RequestTiers = filterValidRequestIntervals(chPricing.Intervals)
	if chPricing.PerRequestPrice != nil {
		resolved.DefaultPerRequestPrice = *chPricing.PerRequestPrice
		resolved.DefaultPerRequestPriceConfigured = true
	}
	if resolved.DefaultPerRequestPriceConfigured || len(resolved.RequestTiers) > 0 {
		resolved.Source = PricingSourceChannel
		resolved.ChannelPricingConfigured = true
	}
}

func filterValidTokenIntervals(intervals []PricingInterval) []PricingInterval {
	var valid []PricingInterval
	for _, iv := range intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil {
			valid = append(valid, iv)
		}
	}
	return valid
}

func filterValidRequestIntervals(intervals []PricingInterval) []PricingInterval {
	var valid []PricingInterval
	for _, iv := range intervals {
		if iv.PerRequestPrice != nil {
			valid = append(valid, iv)
		}
	}
	return valid
}

// GetIntervalPricing 根据 context token 数获取区间定价。
// 如果有区间列表，找到匹配区间并构造 ModelPricing；否则直接返回 BasePricing。
func (r *ModelPricingResolver) GetIntervalPricing(resolved *ResolvedPricing, totalContextTokens int) *ModelPricing {
	if len(resolved.Intervals) == 0 {
		return resolved.BasePricing
	}

	iv := FindMatchingInterval(resolved.Intervals, totalContextTokens)
	if iv == nil {
		return resolved.BasePricing
	}

	return intervalToModelPricing(iv, resolved.SupportsCacheBreakdown, resolved.channelPricing)
}

// intervalToModelPricing 将区间定价转换为 ModelPricing
func intervalToModelPricing(iv *PricingInterval, supportsCacheBreakdown bool, chPricing *ChannelModelPricing) *ModelPricing {
	pricing := &ModelPricing{
		SupportsCacheBreakdown: supportsCacheBreakdown,
	}
	if iv.InputPrice != nil {
		pricing.InputPricePerToken = *iv.InputPrice
		pricing.InputPricePerTokenPriority = *iv.InputPrice
	}
	if iv.OutputPrice != nil {
		pricing.OutputPricePerToken = *iv.OutputPrice
		pricing.OutputPricePerTokenPriority = *iv.OutputPrice
	}
	if iv.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *iv.CacheWritePrice
		pricing.CacheCreationPricePerTokenPriority = *iv.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *iv.CacheWritePrice
		pricing.CacheCreation1hPrice = *iv.CacheWritePrice
	}
	if iv.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *iv.CacheReadPrice
		pricing.CacheReadPricePerTokenPriority = *iv.CacheReadPrice
	}
	// 渠道定价存在时，ImageOutputPrice 显式覆盖
	if chPricing != nil {
		pricing.ImageOutputPriceExplicit = true
		if chPricing.ImageOutputPrice != nil {
			pricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
		}
	}
	return pricing
}

// GetRequestTierPrice 根据层级标签获取按次价格
func (r *ModelPricingResolver) GetRequestTierPrice(resolved *ResolvedPricing, tierLabel string) float64 {
	price, _ := r.GetRequestTierPriceWithPresence(resolved, tierLabel)
	return price
}

// GetRequestTierPriceWithPresence 返回价格及是否显式配置，用于区分免费与缺价。
func (r *ModelPricingResolver) GetRequestTierPriceWithPresence(resolved *ResolvedPricing, tierLabel string) (float64, bool) {
	for _, tier := range resolved.RequestTiers {
		if tier.TierLabel == tierLabel && tier.PerRequestPrice != nil {
			return *tier.PerRequestPrice, true
		}
	}
	return 0, false
}

// GetRequestTierPriceByContext 根据 context token 数获取按次价格
func (r *ModelPricingResolver) GetRequestTierPriceByContext(resolved *ResolvedPricing, totalContextTokens int) float64 {
	price, _ := r.GetRequestTierPriceByContextWithPresence(resolved, totalContextTokens)
	return price
}

// GetRequestTierPriceByContextWithPresence 返回上下文区间价格及是否显式配置。
func (r *ModelPricingResolver) GetRequestTierPriceByContextWithPresence(resolved *ResolvedPricing, totalContextTokens int) (float64, bool) {
	iv := FindMatchingInterval(resolved.RequestTiers, totalContextTokens)
	if iv != nil && iv.PerRequestPrice != nil {
		return *iv.PerRequestPrice, true
	}
	return 0, false
}
