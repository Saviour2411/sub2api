package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var anthropicDeprecatedSamplingParameters = [...]string{"temperature", "top_k", "top_p"}

type anthropicSamplingPolicyKey struct{}

type anthropicSamplingPolicy struct {
	enabled    bool
	validation apicompat.AnthropicModelValidationOptions
}

func resolveAnthropicSamplingPolicy(ctx context.Context, settings GatewaySettings, model string) anthropicSamplingPolicy {
	policy := anthropicSamplingPolicy{}
	if !settings.AnthropicSamplingParameterFilterEnabled || !matchModelWhitelist(model, settings.AnthropicSamplingParameterFilterModels) {
		return policy
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	if group != nil && (group.Platform != PlatformAnthropic || slices.Contains(settings.AnthropicSamplingParameterFilterExcludedGroupIDs, group.ID)) {
		return policy
	}
	policy.enabled = true
	// 无分组的账号测试维持原有采样过滤行为，不自动放宽校验。
	if group != nil && group.ID > 0 {
		policy.validation.SkipThinkingValidation = settings.AnthropicSamplingParameterFilterSkipThinkingValidation
		policy.validation.SkipToolChoiceValidation = settings.AnthropicSamplingParameterFilterSkipToolChoiceValidation
	}
	return policy
}

func resolveAnthropicValidationModel(account *Account, model string) string {
	if account == nil {
		return model
	}
	if account.IsBedrock() {
		if resolved, ok := ResolveBedrockModelID(account, model); ok {
			return resolved
		}
		return model
	}
	switch account.Type {
	case AccountTypeServiceAccount:
		if mapped, matched := account.ResolveMappedModel(model); matched {
			return mapped
		}
	case AccountTypeAPIKey:
		if mapped := account.GetMappedModel(model); mapped != model {
			return mapped
		}
	}
	if account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
		model = claude.NormalizeModelID(model)
		if account.Type == AccountTypeServiceAccount {
			model = normalizeVertexAnthropicModelID(model)
		}
	}
	return model
}

// prepareAnthropicSamplingRequest 为每次账号尝试独立计算策略，后续出站复用同一决定。
func (s *GatewayService) prepareAnthropicSamplingRequest(ctx context.Context, account *Account, body []byte, model string) (context.Context, []byte, apicompat.AnthropicModelValidationOptions, error) {
	policy := anthropicSamplingPolicy{}
	if account != nil && account.Platform == PlatformAnthropic && s.settingService != nil {
		policy = resolveAnthropicSamplingPolicy(ctx, s.settingService.GetGatewayRuntime(ctx), model)
	}
	ctx = context.WithValue(ctx, anthropicSamplingPolicyKey{}, policy)
	filtered, _, err := filterAnthropicSamplingParametersWithSettings(ctx, s.settingService, body, model)
	return ctx, filtered, policy.validation, err
}

func filterAnthropicSamplingParameters(body []byte, model string, settings GatewaySettings) ([]byte, bool, error) {
	if !settings.AnthropicSamplingParameterFilterEnabled ||
		!matchModelWhitelist(model, settings.AnthropicSamplingParameterFilterModels) {
		return body, false, nil
	}
	if !gjson.ValidBytes(body) {
		return nil, false, fmt.Errorf("过滤 Anthropic 采样参数: 请求体不是有效 JSON")
	}

	filtered := body
	changed := false
	for _, parameter := range anthropicDeprecatedSamplingParameters {
		if !gjson.GetBytes(filtered, parameter).Exists() {
			continue
		}
		next, err := sjson.DeleteBytes(filtered, parameter)
		if err != nil {
			return nil, false, fmt.Errorf("过滤 Anthropic 采样参数 %s: %w", parameter, err)
		}
		filtered = next
		changed = true
	}
	return filtered, changed, nil
}

func filterAnthropicSamplingParametersWithSettings(
	ctx context.Context,
	settingService *SettingService,
	body []byte,
	model string,
) ([]byte, bool, error) {
	if policy, ok := ctx.Value(anthropicSamplingPolicyKey{}).(anthropicSamplingPolicy); ok {
		if !policy.enabled {
			return body, false, nil
		}
		return filterAnthropicSamplingParameters(body, model, GatewaySettings{
			AnthropicSamplingParameterFilterEnabled: true,
			AnthropicSamplingParameterFilterModels:  []string{model},
		})
	}
	if settingService == nil {
		return body, false, nil
	}
	settings := settingService.GetGatewayRuntime(ctx)
	if !resolveAnthropicSamplingPolicy(ctx, settings, model).enabled {
		return body, false, nil
	}
	return filterAnthropicSamplingParameters(body, model, settings)
}
