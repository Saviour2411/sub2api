package service

import (
	"context"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var anthropicDeprecatedSamplingParameters = [...]string{"temperature", "top_k", "top_p"}

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
	if settingService == nil {
		return body, false, nil
	}
	return filterAnthropicSamplingParameters(body, model, settingService.GetGatewayRuntime(ctx))
}
