//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newRequestedPricingTestResolver(model string, pricing *ChannelModelPricing) *ModelPricingResolver {
	return newRequestedPricingTestResolverWithEntries(map[string]*ChannelModelPricing{model: pricing})
}

func newRequestedPricingTestResolverWithEntries(entries map[string]*ChannelModelPricing) *ModelPricingResolver {
	const groupID int64 = 42
	cache := newEmptyChannelCache()
	for model, pricing := range entries {
		cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}] = pricing
	}
	cache.channelByGroupID[groupID] = &Channel{ID: groupID, Status: StatusActive}
	cache.groupPlatform[groupID] = ""
	cache.loadedAt = time.Now()
	channelService := &ChannelService{}
	channelService.cache.Store(cache)
	billingService := NewBillingService(&config.Config{}, nil)
	return NewModelPricingResolver(channelService, billingService)
}

func TestValidateRequestedModelPricing_UsesRequestedAliasOnly(t *testing.T) {
	cfg := &config.Config{}
	billingService := NewBillingService(cfg, nil)
	resolver := NewModelPricingResolver(nil, billingService)
	svc := &OpenAIGatewayService{cfg: cfg, billingService: billingService, resolver: resolver}

	require.NoError(t, svc.ValidateRequestedModelPricing(
		context.Background(), nil, "openai/gpt-5.6", PricingUsageToken, "",
	))
	require.ErrorIs(t, svc.ValidateRequestedModelPricing(
		context.Background(), nil, "unpriced-requested-model", PricingUsageToken, "",
	), ErrModelPricingUnavailable)
}

func TestValidateRequestedModelPricing_ExplicitZeroAndEmptyChannelEntry(t *testing.T) {
	zero := 0.0
	groupID := int64(42)
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: 42}}

	t.Run("explicit zero is configured", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("free-model", &ChannelModelPricing{
			BillingMode: BillingModeToken,
			InputPrice:  &zero,
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.NoError(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "free-model", PricingUsageToken, "",
		))
	})

	t.Run("empty entry is unavailable", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("empty-model", &ChannelModelPricing{BillingMode: BillingModeToken})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.ErrorIs(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "empty-model", PricingUsageToken, "",
		), ErrModelPricingUnavailable)
	})

	t.Run("image output price alone is not text pricing", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("image-token-only", &ChannelModelPricing{
			BillingMode:      BillingModeToken,
			ImageOutputPrice: &zero,
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.ErrorIs(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "image-token-only", PricingUsageToken, "",
		), ErrModelPricingUnavailable)
	})
}

func TestResolveRequestedModelPricing_EmptyLiteralEntryDoesNotBeatCanonicalChannelPrice(t *testing.T) {
	zero := 0.0
	groupID := int64(42)
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID}}
	resolver := newRequestedPricingTestResolverWithEntries(map[string]*ChannelModelPricing{
		"gpt-5.6":     {BillingMode: BillingModeToken},
		"gpt-5.6-sol": {BillingMode: BillingModeToken, InputPrice: &zero},
	})

	model, err := resolveRequestedModelPricing(
		context.Background(), &config.Config{}, resolver, resolver.billingService,
		apiKey, "gpt-5.6", PricingUsageToken, "",
	)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.6-sol", model)
}

func TestResolveRequestedModelPricing_OfficialVariantPrefersCanonicalChannelPrice(t *testing.T) {
	channelInputPrice := 0.000321
	groupID := int64(42)
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID}}
	resolver := newRequestedPricingTestResolver("gpt-5.4-pro", &ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  &channelInputPrice,
	})

	model, err := resolveRequestedModelPricing(
		context.Background(), &config.Config{}, resolver, resolver.billingService,
		apiKey, "gpt-5.4-pro-2026-03-05", PricingUsageToken, "",
	)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.4-pro", model)
}

func TestValidateRequestedModelPricing_PerRequestCoverage(t *testing.T) {
	zero := 0.0
	max := 100
	groupID := int64(42)
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID}}

	t.Run("partial context interval is rejected", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("partial-per-request", &ChannelModelPricing{
			BillingMode: BillingModePerRequest,
			Intervals:   []PricingInterval{{MinTokens: 0, MaxTokens: &max, PerRequestPrice: &zero}},
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.ErrorIs(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "partial-per-request", PricingUsageToken, "",
		), ErrModelPricingUnavailable)
	})

	t.Run("full context interval is accepted", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("full-per-request", &ChannelModelPricing{
			BillingMode: BillingModePerRequest,
			Intervals:   []PricingInterval{{MinTokens: 0, PerRequestPrice: &zero}},
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.NoError(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "full-per-request", PricingUsageToken, "",
		))
	})

	t.Run("explicit zero default is accepted", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("free-per-request", &ChannelModelPricing{
			BillingMode: BillingModePerRequest, PerRequestPrice: &zero,
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.NoError(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "free-per-request", PricingUsageToken, "",
		))
	})

	t.Run("token-only interval is not a per request price", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("wrong-mode-interval", &ChannelModelPricing{
			BillingMode: BillingModePerRequest,
			Intervals:   []PricingInterval{{MinTokens: 0, InputPrice: &zero}},
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.ErrorIs(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "wrong-mode-interval", PricingUsageToken, "",
		), ErrModelPricingUnavailable)
	})
}

func TestValidateRequestedModelPricing_MediaTierPresence(t *testing.T) {
	zero := 0.0
	max := 100
	groupID := int64(42)
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID}}

	t.Run("explicit zero current tier is accepted", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("tiered-image", &ChannelModelPricing{
			BillingMode: BillingModeImage,
			Intervals:   []PricingInterval{{TierLabel: "2K", PerRequestPrice: &zero}},
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.NoError(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "tiered-image", PricingUsageImage, "2K",
		))
	})

	t.Run("missing channel tier keeps model default fallback", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("tiered-image", &ChannelModelPricing{
			BillingMode: BillingModeImage,
			Intervals:   []PricingInterval{{TierLabel: "1K", PerRequestPrice: &zero}},
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.NoError(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "tiered-image", PricingUsageImage, "4K",
		))
	})

	t.Run("incomplete channel token mode cannot fall back to media default", func(t *testing.T) {
		resolver := newRequestedPricingTestResolver("token-priced-image", &ChannelModelPricing{
			BillingMode: BillingModeToken,
			Intervals:   []PricingInterval{{MinTokens: 0, MaxTokens: &max, InputPrice: &zero}},
		})
		svc := &GatewayService{cfg: &config.Config{}, billingService: resolver.billingService, resolver: resolver}
		require.ErrorIs(t, svc.ValidateRequestedModelPricing(
			context.Background(), apiKey, "token-priced-image", PricingUsageImage, "2K",
		), ErrModelPricingUnavailable)
	})
}

func TestValidateRequestedModelPricing_SimpleModeSkipsValidation(t *testing.T) {
	cfg := &config.Config{RunMode: config.RunModeSimple}
	svc := &GatewayService{cfg: cfg}
	require.NoError(t, svc.ValidateRequestedModelPricing(
		context.Background(), nil, "unpriced-model", PricingUsageToken, "",
	))
}

func TestCalculateCostUnified_PerRequestPricingPresence(t *testing.T) {
	billingService := NewBillingService(&config.Config{}, nil)
	resolver := NewModelPricingResolver(nil, billingService)

	t.Run("missing price fails closed", func(t *testing.T) {
		_, err := billingService.CalculateCostUnified(CostInput{
			Model: "per-request-model", RateMultiplier: 1, Resolver: resolver,
			Resolved: &ResolvedPricing{Mode: BillingModePerRequest},
		})
		require.ErrorIs(t, err, ErrModelPricingUnavailable)
	})

	t.Run("explicit zero default is free", func(t *testing.T) {
		cost, err := billingService.CalculateCostUnified(CostInput{
			Model: "free-per-request-model", RequestCount: 1, RateMultiplier: 1, Resolver: resolver,
			Resolved: &ResolvedPricing{
				Mode: BillingModePerRequest, DefaultPerRequestPriceConfigured: true,
			},
		})
		require.NoError(t, err)
		require.Zero(t, cost.ActualCost)
	})

	t.Run("explicit zero tier does not fall back", func(t *testing.T) {
		zero := 0.0
		fallback := 0.25
		cost, err := billingService.CalculateCostUnified(CostInput{
			Model: "free-tier-model", SizeTier: "1K", RequestCount: 1, RateMultiplier: 1, Resolver: resolver,
			Resolved: &ResolvedPricing{
				Mode:                             BillingModeImage,
				DefaultPerRequestPrice:           fallback,
				DefaultPerRequestPriceConfigured: true,
				RequestTiers:                     []PricingInterval{{TierLabel: "1K", PerRequestPrice: &zero}},
			},
		})
		require.NoError(t, err)
		require.Zero(t, cost.ActualCost)
	})
}
