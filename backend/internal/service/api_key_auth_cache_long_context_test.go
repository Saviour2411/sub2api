//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotRoundTripPreservesGrokLongContextPricing(t *testing.T) {
	const (
		baseTotal   = 0.140710
		tieredTotal = 0.281420
		rate        = 0.15
	)

	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "关闭", true: "开启"}[enabled], func(t *testing.T) {
			groupID := int64(39)
			apiKey := &APIKey{
				ID:      1356,
				UserID:  877,
				GroupID: &groupID,
				Key:     "k-grok-long-context",
				Status:  StatusActive,
				User: &User{
					ID:      877,
					Status:  StatusActive,
					Role:    RoleUser,
					Balance: 10,
				},
				Group: &Group{
					ID:                        groupID,
					Name:                      "Grok-稳定通道【非free】",
					Platform:                  PlatformGrok,
					Status:                    StatusActive,
					SubscriptionType:          SubscriptionTypeStandard,
					RateMultiplier:            rate,
					LongContextPricingEnabled: enabled,
				},
			}

			apiKeyService := NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
			snapshot := apiKeyService.snapshotFromAPIKey(context.Background(), apiKey)
			restored := apiKeyService.snapshotToAPIKey(apiKey.Key, snapshot)

			require.Equal(t, apiKeyAuthSnapshotVersion, snapshot.Version)
			require.NotNil(t, restored)
			require.NotNil(t, restored.Group)
			require.Equal(t, enabled, restored.Group.LongContextPricingEnabled)

			billingService := NewBillingService(&config.Config{}, nil)
			resolver := NewModelPricingResolver(nil, billingService)
			cost, err := billingService.CalculateCostUnified(CostInput{
				Ctx:            context.Background(),
				Model:          "grok-4.6",
				GroupID:        restored.GroupID,
				Group:          restored.Group,
				Tokens:         UsageTokens{InputTokens: 1567, OutputTokens: 252, CacheReadTokens: 272128},
				RateMultiplier: rate,
				Resolver:       resolver,
			})
			require.NoError(t, err)

			if enabled {
				require.True(t, cost.LongContextBillingApplied)
				require.InDelta(t, tieredTotal, cost.TotalCost, 1e-12)
				require.InDelta(t, tieredTotal*rate, cost.ActualCost, 1e-12)
				return
			}
			require.False(t, cost.LongContextBillingApplied)
			require.InDelta(t, baseTotal, cost.TotalCost, 1e-12)
			require.InDelta(t, baseTotal*rate, cost.ActualCost, 1e-12)
		})
	}
}
