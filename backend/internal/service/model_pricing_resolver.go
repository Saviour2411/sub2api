package service

import (
	"context"
	"log/slog"
	"strings"
)

// PricingSource 定价来源标识
const (
	PricingSourceGroup    = "group"
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

	longContextPricingEnabled    bool
	longContextPricingConfigured bool
}

// ModelPricingResolver 统一模型定价解析器。
// 解析链：Group → Channel → LiteLLM → Fallback。
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
	Group   *Group
}

// Resolve 解析模型定价。
// 1. 获取基础定价（LiteLLM → Fallback）
// 2. 如果指定了 GroupID，查找渠道定价并覆盖
func (r *ModelPricingResolver) Resolve(ctx context.Context, input PricingInput) *ResolvedPricing {
	longContextPricingEnabled := input.Group == nil || input.Group.LongContextPricingEnabled
	longContextPricingConfigured := input.Group != nil
	if groupPricing := matchGroupModelPricing(input.Group, input.Model); groupPricing != nil {
		// Group token cards only override the first-tier / flat rates.
		// Long-context ladders come from official presets, gated by the checkbox.
		if groupPricing.BillingMode == "" || groupPricing.BillingMode == BillingModeToken {
			stripped := groupPricing.Clone()
			stripped.Intervals = nil
			groupPricing = &stripped
		}
		resolved := r.resolveConfiguredPricing(groupPricing, input.Model, PricingSourceGroup)
		resolved.longContextPricingEnabled = longContextPricingEnabled
		resolved.longContextPricingConfigured = longContextPricingConfigured
		return resolved
	}

	var chPricing *ChannelModelPricing
	if input.GroupID != nil && r.channelService != nil {
		chPricing = r.channelService.GetChannelModelPricing(ctx, *input.GroupID, input.Model)
		if chPricing != nil {
			mode := chPricing.BillingMode
			if mode == "" {
				mode = BillingModeToken
			}
			if mode == BillingModePerRequest || mode == BillingModeImage || mode == BillingModeVideo {
				resolved := &ResolvedPricing{
					Mode:           mode,
					Source:         PricingSourceChannel,
					channelPricing: chPricing,
				}
				resolved.longContextPricingEnabled = longContextPricingEnabled
				resolved.longContextPricingConfigured = longContextPricingConfigured
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
	resolved.longContextPricingEnabled = longContextPricingEnabled
	resolved.longContextPricingConfigured = longContextPricingConfigured

	// 2. 如果有 GroupID，尝试渠道覆盖
	if chPricing != nil {
		resolved.channelPricing = chPricing
		r.applyTokenOverrides(chPricing, resolved)
	} else if input.GroupID != nil && r.channelService != nil {
		r.applyChannelOverrides(ctx, *input.GroupID, input.Model, resolved)
	}

	return resolved
}

func (r *ModelPricingResolver) resolveConfiguredPricing(config *ChannelModelPricing, model, source string) *ResolvedPricing {
	mode := config.BillingMode
	if mode == "" {
		mode = BillingModeToken
	}
	resolved := &ResolvedPricing{Mode: mode, Source: source, channelPricing: config}
	if mode == BillingModePerRequest || mode == BillingModeImage || mode == BillingModeVideo {
		r.applyRequestTierOverrides(config, resolved)
		return resolved
	}
	resolved.BasePricing, _ = r.resolveBasePricing(model)
	resolved.SupportsCacheBreakdown = resolved.BasePricing != nil && resolved.BasePricing.SupportsCacheBreakdown
	r.applyTokenOverrides(config, resolved)
	return resolved
}

func matchGroupModelPricing(group *Group, model string) *ChannelModelPricing {
	if group == nil {
		return nil
	}
	model = normalizeChannelPricingModelName(model)
	var wildcard *ChannelModelPricing
	for i := range group.ModelPricing {
		entry := &group.ModelPricing[i]
		for _, pattern := range entry.Models {
			normalized := normalizeChannelPricingModelName(pattern)
			if normalized == model {
				cp := entry.Clone()
				return &cp
			}
			if strings.HasSuffix(normalized, "*") && strings.HasPrefix(model, strings.TrimSuffix(normalized, "*")) && wildcard == nil {
				cp := entry.Clone()
				wildcard = &cp
			}
		}
	}
	return wildcard
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
	case BillingModePerRequest, BillingModeImage, BillingModeVideo:
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
	isGroupPricing := resolved.Source == PricingSourceGroup
	validIntervals := filterValidTokenIntervals(chPricing.Intervals)
	hasFlatTokenPricing := chPricing.InputPrice != nil || chPricing.OutputPrice != nil ||
		chPricing.CacheWritePrice != nil || chPricing.CacheReadPrice != nil
	hasExplicitIntervalPricing := false
	for i := range validIntervals {
		iv := &validIntervals[i]
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil {
			hasExplicitIntervalPricing = true
			break
		}
	}
	hasMetadataOverride := chPricing.FastMultiplier != nil || chPricing.FlexMultiplier != nil ||
		chPricing.ImageInputPrice != nil || chPricing.ImageOutputPrice != nil ||
		chPricing.TimePricing != nil
	hasAnyOverride := hasFlatTokenPricing || len(validIntervals) > 0 || hasMetadataOverride

	// 空渠道条目不应把未知模型伪装成一个全零价格模型。已有全局价格时也无需覆盖。
	if !hasAnyOverride {
		return
	}

	// 倍率、Fast/Flex、图片价或分时倍率都必须建立在已知基础价上；
	// 只有显式 token 价才能为未知模型建立可计费定价。
	if resolved.BasePricing == nil && !hasFlatTokenPricing && !hasExplicitIntervalPricing {
		return
	}
	if !isGroupPricing {
		resolved.Source = PricingSourceChannel
		resolved.ChannelPricingConfigured = true
	}
	if resolved.BasePricing != nil || hasFlatTokenPricing || hasExplicitIntervalPricing {
		resolved.TokenPricingConfigured = true
	}

	if resolved.BasePricing == nil && hasFlatTokenPricing {
		resolved.BasePricing = &ModelPricing{}
	} else if resolved.BasePricing != nil {
		// 防止修改 fallbackPrices 中的共享指针
		cloned := *resolved.BasePricing
		resolved.BasePricing = &cloned
	}

	if resolved.BasePricing != nil {
		// flat 价是区间未命中时的渠道基价；区间倍率也以它为基数。
		applyChannelTokenPriceOverrides(resolved.BasePricing, chPricing)
		resolved.BasePricing.FastMultiplier = chPricing.FastMultiplier
		resolved.BasePricing.FlexMultiplier = chPricing.FlexMultiplier
		// 渠道定价存在时，图片输出价显式覆盖；未配置即按 0 处理。
		if chPricing.ImageOutputPrice != nil {
			resolved.BasePricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
		} else {
			resolved.BasePricing.ImageOutputPricePerToken = 0
		}
		resolved.BasePricing.ImageOutputPriceExplicit = true
		applyChannelImageInputPrice(chPricing, resolved.BasePricing)
	}

	resolved.Intervals = validIntervals
}

// applyChannelImageInputPrice 应用渠道图片输入价：显式配置则用配置值；
// 未配置时归零，使 computeTokenBreakdown 回退到文本输入价（向后兼容，
// 避免 commit 引入的 LiteLLM 图片输入价泄漏进渠道自定义定价）。
// 与 image_output 不同，此处不设 Explicit 标志——图片输入未配置应回退文本价，
// 而非硬置 0。
func applyChannelImageInputPrice(chPricing *ChannelModelPricing, pricing *ModelPricing) {
	if pricing == nil {
		return
	}
	if chPricing != nil && chPricing.ImageInputPrice != nil {
		pricing.ImageInputPricePerToken = *chPricing.ImageInputPrice
	} else {
		pricing.ImageInputPricePerToken = 0
	}
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
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil ||
			iv.InputMultiplier != nil || iv.OutputMultiplier != nil ||
			iv.CacheWriteMultiplier != nil || iv.CacheReadMultiplier != nil {
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

	pricing := intervalToModelPricing(iv, resolved.BasePricing, resolved.channelPricing)
	// BasePricing 为 nil（仅配置区间）时拷贝不到该标志，从 resolved 回填，
	// 保证 computeCacheCreationCost 的 5m/1h 分档判断不被区间路径吞掉。
	pricing.SupportsCacheBreakdown = resolved.SupportsCacheBreakdown
	return pricing
}

// intervalToModelPricing 将区间定价转换为 ModelPricing
func intervalToModelPricing(iv *PricingInterval, base *ModelPricing, chPricing *ChannelModelPricing) *ModelPricing {
	pricing := &ModelPricing{}
	if base != nil {
		*pricing = *base
	}
	applyMultiplier := func(value float64, multiplier *float64) float64 {
		if multiplier == nil {
			return value
		}
		return value * *multiplier
	}
	if iv.InputPrice != nil {
		pricing.InputPricePerTokenPriority = channelTierOverridePrice(pricing.InputPricePerToken, pricing.InputPricePerTokenPriority, *iv.InputPrice)
		pricing.InputPricePerToken = *iv.InputPrice
	} else if iv.InputMultiplier != nil {
		pricing.InputPricePerToken = applyMultiplier(pricing.InputPricePerToken, iv.InputMultiplier)
		pricing.InputPricePerTokenPriority = applyMultiplier(pricing.InputPricePerTokenPriority, iv.InputMultiplier)
	}
	if iv.OutputPrice != nil {
		pricing.OutputPricePerTokenPriority = channelTierOverridePrice(pricing.OutputPricePerToken, pricing.OutputPricePerTokenPriority, *iv.OutputPrice)
		pricing.OutputPricePerToken = *iv.OutputPrice
	} else if iv.OutputMultiplier != nil {
		pricing.OutputPricePerToken = applyMultiplier(pricing.OutputPricePerToken, iv.OutputMultiplier)
		pricing.OutputPricePerTokenPriority = applyMultiplier(pricing.OutputPricePerTokenPriority, iv.OutputMultiplier)
	}
	if iv.CacheWritePrice != nil {
		pricing.CacheCreationPricePerTokenPriority = channelTierOverridePrice(pricing.CacheCreationPricePerToken, pricing.CacheCreationPricePerTokenPriority, *iv.CacheWritePrice)
		pricing.CacheCreationPricePerToken = *iv.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *iv.CacheWritePrice
		pricing.CacheCreation1hPrice = *iv.CacheWritePrice
	} else if iv.CacheWriteMultiplier != nil {
		pricing.CacheCreationPricePerToken = applyMultiplier(pricing.CacheCreationPricePerToken, iv.CacheWriteMultiplier)
		pricing.CacheCreationPricePerTokenPriority = applyMultiplier(pricing.CacheCreationPricePerTokenPriority, iv.CacheWriteMultiplier)
		pricing.CacheCreation5mPrice = applyMultiplier(pricing.CacheCreation5mPrice, iv.CacheWriteMultiplier)
		pricing.CacheCreation1hPrice = applyMultiplier(pricing.CacheCreation1hPrice, iv.CacheWriteMultiplier)
	}
	if iv.CacheReadPrice != nil {
		pricing.CacheReadPricePerTokenPriority = channelTierOverridePrice(pricing.CacheReadPricePerToken, pricing.CacheReadPricePerTokenPriority, *iv.CacheReadPrice)
		pricing.CacheReadPricePerToken = *iv.CacheReadPrice
	} else if iv.CacheReadMultiplier != nil {
		pricing.CacheReadPricePerToken = applyMultiplier(pricing.CacheReadPricePerToken, iv.CacheReadMultiplier)
		pricing.CacheReadPricePerTokenPriority = applyMultiplier(pricing.CacheReadPricePerTokenPriority, iv.CacheReadMultiplier)
	}
	// 渠道定价存在时，ImageOutputPrice 显式覆盖；图片输入价用渠道级配置
	// （区间不携带图片输入价，与 image_output 一致）。
	if chPricing != nil {
		pricing.FastMultiplier = chPricing.FastMultiplier
		pricing.FlexMultiplier = chPricing.FlexMultiplier
		pricing.ImageOutputPriceExplicit = true
		if chPricing.ImageOutputPrice != nil {
			pricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
		}
		applyChannelImageInputPrice(chPricing, pricing)
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
