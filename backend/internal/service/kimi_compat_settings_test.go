//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestKimiCompatibilitySettingsRoundTrip(t *testing.T) {
	keys := []string{
		SettingKeyGatewayKimiSamplingParameterRetryEnabled,
		SettingKeyGatewayKimiReasoningEffortRetryEnabled,
		SettingKeyGatewayKimiToolChoiceRetryEnabled,
		SettingKeyGatewayKimiMaxCompletionTokensRetryEnabled,
		SettingKeyGatewayKimiThinkingTypeRetryEnabled,
	}
	for index, key := range keys {
		t.Run(key, func(t *testing.T) {
			repository := &customFeatureSettingsRepoStub{}
			service := NewSettingService(repository, &config.Config{})
			settings, err := service.GetGatewaySettings(context.Background())
			require.NoError(t, err)
			for rule := kimiCompatRule(0); rule < kimiCompatRuleCount; rule++ {
				require.False(t, rule.enabled(*settings))
			}
			fields := []*bool{&settings.KimiSamplingParameterRetryEnabled, &settings.KimiReasoningEffortRetryEnabled, &settings.KimiToolChoiceRetryEnabled, &settings.KimiMaxCompletionTokensRetryEnabled, &settings.KimiThinkingTypeRetryEnabled}
			*fields[index] = true
			_, err = service.UpdateGatewaySettings(context.Background(), *settings)
			require.NoError(t, err)
			require.Equal(t, "true", repository.values[key])
			reloaded := NewSettingService(repository, &config.Config{})
			stored, err := reloaded.GetGatewaySettings(context.Background())
			require.NoError(t, err)
			runtime := reloaded.GetGatewayRuntime(context.Background())
			features, err := reloaded.GetCustomFeatureSettings(context.Background())
			require.NoError(t, err)
			for rule := kimiCompatRule(0); rule < kimiCompatRuleCount; rule++ {
				require.Equal(t, int(rule) == index, rule.enabled(*stored))
				require.Equal(t, int(rule) == index, rule.enabled(runtime))
				require.Equal(t, int(rule) == index, rule.enabled(features.Gateway))
			}
		})
	}
}
