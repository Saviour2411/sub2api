package service

import (
	"context"
	"time"
)

// LegacyLongContextRule 平台级“超出阈值部分按倍率计费”的旧规则。
//
// 语义为边际计费：仅 input/cache_read 中超过 Threshold 的部分乘 Multiplier，
// output 与 cache_write 不受影响。仅 Gemini 原生 /v1beta 入口使用。
type LegacyLongContextRule struct {
	Threshold  int
	Multiplier float64
}

const (
	geminiLegacyLongContextThreshold  = 200000
	geminiLegacyLongContextMultiplier = 2.0
)

// LegacyLongContextRule 返回平台边际长上下文规则；无规则的平台返回 nil。
func (s *BillingService) LegacyLongContextRule(platform string) *LegacyLongContextRule {
	if platform == PlatformGemini {
		return &LegacyLongContextRule{
			Threshold:  geminiLegacyLongContextThreshold,
			Multiplier: geminiLegacyLongContextMultiplier,
		}
	}
	return nil
}

// TokenCostRequest 通用网关 token 计费请求。
type TokenCostRequest struct {
	Ctx            context.Context
	Model          string
	Group          *Group
	Tokens         UsageTokens
	RateMultiplier float64
	PricingAt      time.Time
	ServiceTier    string
	Resolver       *ModelPricingResolver
	// Resolved 为调用方预先解析的定价（Resolver.Resolve 的结果），nil 表示未解析。
	Resolved *ResolvedPricing
	// LegacyLongContext 由 Gemini 原生入口显式携带；nil 表示使用目录整单阶梯。
	LegacyLongContext *LegacyLongContextRule
}

// legacyLongContextApplies 判定是否启用 Gemini 边际计费：分组/渠道显式定价优先；
// 否则规则存在且分组长上下文开关开启时生效。该分支位于目录整单阶梯之前，避免双重计费。
func legacyLongContextApplies(resolved *ResolvedPricing, group *Group, rule *LegacyLongContextRule) bool {
	if rule == nil || rule.Threshold <= 0 {
		return false
	}
	if resolved != nil && (resolved.Source == PricingSourceGroup || resolved.Source == PricingSourceChannel) {
		return false
	}
	return group == nil || group.LongContextPricingEnabled
}

// CalculateTokenCostForRequest 按通用网关的路径选择计算 token 费用：
//  1. 分组/渠道显式定价 → 统一计费；
//  2. Gemini 原生入口携带边际规则且分组开关开启 → 旧边际计费；
//  3. 否则有解析器与分组 → 目录驱动的统一计费；
//  4. 否则按模型目录直接计费。
//
// 模型广场的阶梯表查询与网关使用同一入口，保证展示与扣费同源。
func (s *BillingService) CalculateTokenCostForRequest(req TokenCostRequest) (*CostBreakdown, error) {
	resolved := req.Resolved
	if resolved != nil && (resolved.Source == PricingSourceGroup || resolved.Source == PricingSourceChannel) {
		return s.CalculateCostUnified(s.tokenCostInput(req, resolved))
	}
	if legacyLongContextApplies(resolved, req.Group, req.LegacyLongContext) {
		return s.calculateCostWithMarginalLongContext(req, req.LegacyLongContext)
	}
	if req.Resolver != nil && req.Group != nil {
		return s.CalculateCostUnified(s.tokenCostInput(req, resolved))
	}
	return s.CalculateCost(req.Model, req.Tokens, req.RateMultiplier)
}

// calculateCostWithMarginalLongContext 仅对超过阈值的 input/cache_read 乘倍率。
// 两段成本都显式关闭目录整单阶梯，避免 Gemini 原生入口同时应用两套规则。
func (s *BillingService) calculateCostWithMarginalLongContext(req TokenCostRequest, rule *LegacyLongContextRule) (*CostBreakdown, error) {
	if rule == nil || rule.Threshold <= 0 || rule.Multiplier <= 1 {
		withoutLegacy := req
		withoutLegacy.LegacyLongContext = nil
		return s.calculateTokenCostWithoutLongContext(withoutLegacy)
	}

	total := req.Tokens.CacheReadTokens + req.Tokens.InputTokens
	if total <= rule.Threshold {
		withoutLegacy := req
		withoutLegacy.LegacyLongContext = nil
		return s.calculateTokenCostWithoutLongContext(withoutLegacy)
	}

	var inRangeCacheTokens, inRangeInputTokens int
	var outRangeCacheTokens, outRangeInputTokens int
	if req.Tokens.CacheReadTokens >= rule.Threshold {
		inRangeCacheTokens = rule.Threshold
		outRangeCacheTokens = req.Tokens.CacheReadTokens - rule.Threshold
		outRangeInputTokens = req.Tokens.InputTokens
	} else {
		inRangeCacheTokens = req.Tokens.CacheReadTokens
		inRangeInputTokens = rule.Threshold - req.Tokens.CacheReadTokens
		outRangeInputTokens = req.Tokens.InputTokens - inRangeInputTokens
	}

	inRangeReq := req
	inRangeReq.LegacyLongContext = nil
	inRangeReq.Tokens = UsageTokens{
		InputTokens:           inRangeInputTokens,
		ImageInputTokens:      req.Tokens.ImageInputTokens,
		OutputTokens:          req.Tokens.OutputTokens,
		CacheCreationTokens:   req.Tokens.CacheCreationTokens,
		CacheReadTokens:       inRangeCacheTokens,
		CacheCreation5mTokens: req.Tokens.CacheCreation5mTokens,
		CacheCreation1hTokens: req.Tokens.CacheCreation1hTokens,
		ImageOutputTokens:     req.Tokens.ImageOutputTokens,
	}
	inRangeCost, err := s.calculateTokenCostWithoutLongContext(inRangeReq)
	if err != nil {
		return nil, err
	}

	outRangeReq := req
	outRangeReq.LegacyLongContext = nil
	outRangeReq.RateMultiplier *= rule.Multiplier
	outRangeReq.Tokens = UsageTokens{InputTokens: outRangeInputTokens, CacheReadTokens: outRangeCacheTokens}
	outRangeCost, err := s.calculateTokenCostWithoutLongContext(outRangeReq)
	if err != nil {
		return inRangeCost, err
	}

	return &CostBreakdown{
		InputCost:                 inRangeCost.InputCost + outRangeCost.InputCost,
		ImageInputCost:            inRangeCost.ImageInputCost + outRangeCost.ImageInputCost,
		OutputCost:                inRangeCost.OutputCost,
		ImageOutputCost:           inRangeCost.ImageOutputCost,
		CacheCreationCost:         inRangeCost.CacheCreationCost,
		CacheReadCost:             inRangeCost.CacheReadCost + outRangeCost.CacheReadCost,
		TotalCost:                 inRangeCost.TotalCost + outRangeCost.TotalCost,
		ActualCost:                inRangeCost.ActualCost + outRangeCost.ActualCost,
		BillingMode:               inRangeCost.BillingMode,
		LongContextBillingApplied: outRangeCost.ActualCost > 0,
	}, nil
}

func (s *BillingService) calculateTokenCostWithoutLongContext(req TokenCostRequest) (*CostBreakdown, error) {
	disabled := false
	input := s.tokenCostInput(req, req.Resolved)
	input.LongContextBillingEnabled = &disabled
	return s.CalculateCostUnified(input)
}

func (s *BillingService) tokenCostInput(req TokenCostRequest, resolved *ResolvedPricing) CostInput {
	input := CostInput{
		Ctx:            req.Ctx,
		Model:          req.Model,
		Group:          req.Group,
		Tokens:         req.Tokens,
		RequestCount:   1,
		RateMultiplier: req.RateMultiplier,
		PricingAt:      req.PricingAt,
		ServiceTier:    req.ServiceTier,
		Resolver:       req.Resolver,
		Resolved:       resolved,
	}
	if req.Group != nil {
		gid := req.Group.ID
		input.GroupID = &gid
	}
	return input
}
