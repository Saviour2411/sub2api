package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeKnownOpenAICodexModel_BareGPT56RoutesToSol(t *testing.T) {
	tests := map[string]string{
		"gpt-5.6":               "gpt-5.6-sol",
		"openai/gpt-5.6":        "gpt-5.6-sol",
		"gpt5.6":                "gpt-5.6-sol",
		"gpt-5.6-high":          "gpt-5.6-sol",
		"gpt-5.6-max":           "gpt-5.6-sol",
		"gpt-5.6-2026-07-09":    "gpt-5.6-sol",
		"gpt-5.6-20260709":      "gpt-5.6-sol",
		"gpt-5.6-sol-max":       "gpt-5.6-sol",
		"gpt-5.6-sol-preview":   "gpt-5.6-sol",
		"gpt-5.6-terra-preview": "gpt-5.6-terra",
		"gpt-5.6-luna-preview":  "gpt-5.6-luna",
		"openai/gpt-5.6-max":    "gpt-5.6-sol",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, expected, normalizeKnownOpenAICodexModel(input))
		})
	}
}

func TestNormalizeKnownOpenAICodexModel_RejectsUnknownGPT5Variants(t *testing.T) {
	for _, model := range []string{
		"gpt-5.999",
		"gpt-5.555",
		"x-gpt-5.4-y",
		"gpt-5.999-gpt-5.4",
		"gpt-5.6-sol-weird",
		"gpt-5.6-preview",
		"gpt-5.6-sol-preview-weird",
		"gpt-5.4-custom",
	} {
		t.Run(model, func(t *testing.T) {
			require.Empty(t, normalizeKnownOpenAICodexModel(model))
			require.False(t, isOpenAIGPT56Model(model))
		})
	}
}

func TestNormalizeKnownOpenAIPricingModel_OfficialFamilies(t *testing.T) {
	tests := map[string]string{
		"gpt-5.1-2025-11-13":      "gpt-5.1",
		"gpt-5.1-chat-latest":     "gpt-5.1",
		"gpt-5.2-chat-latest":     "gpt-5.2",
		"gpt-5.2-codex":           "gpt-5.2",
		"gpt-5.2-pro-2025-12-11":  "gpt-5.2-pro",
		"gpt-5.3-chat-latest":     "gpt-5.3-codex",
		"gpt-5.4-2026-03-05":      "gpt-5.4",
		"gpt-5.4-mini-2026-03-17": "gpt-5.4-mini",
		"gpt-5.4-nano-2026-03-17": "gpt-5.4-nano",
		"gpt-5.4-pro-2026-03-05":  "gpt-5.4-pro",
		"openai/gpt-5.4-pro":      "gpt-5.4-pro",
		"gpt-5.6-terra-preview":   "gpt-5.6-terra",
	}
	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, expected, normalizeKnownOpenAIPricingModel(input))
		})
	}
	for _, input := range []string{"gpt-5.999", "gpt-5.4-custom", "gpt-5.2-pro-custom"} {
		t.Run("reject/"+input, func(t *testing.T) {
			require.Empty(t, normalizeKnownOpenAIPricingModel(input))
		})
	}
}

func TestUsageBillingModelCandidates_BareGPT56IncludesSol(t *testing.T) {
	require.Equal(t,
		[]string{"gpt-5.6", "gpt-5.6-sol"},
		usageBillingModelCandidates("gpt-5.6"),
	)
	require.Equal(t,
		[]string{"openai/gpt-5.6", "gpt-5.6", "gpt-5.6-sol"},
		usageBillingModelCandidates("openai/gpt-5.6"),
	)
}

func TestUsageBillingModelCandidates_OfficialPricingVariantsIncludeFamilyBase(t *testing.T) {
	require.Equal(t,
		[]string{"gpt-5.2-pro-2025-12-11", "gpt-5.2-pro"},
		usageBillingModelCandidates("gpt-5.2-pro-2025-12-11"),
	)
	require.Equal(t,
		[]string{"openai/gpt-5.4-pro-2026-03-05", "gpt-5.4-pro-2026-03-05", "gpt-5.4-pro"},
		usageBillingModelCandidates("openai/gpt-5.4-pro-2026-03-05"),
	)
	require.Equal(t,
		[]string{"gpt-5.4-custom"},
		usageBillingModelCandidates("gpt-5.4-custom"),
	)
}
