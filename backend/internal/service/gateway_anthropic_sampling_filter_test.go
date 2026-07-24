//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type anthropicSamplingFilterSettingsRepo struct {
	*customFeatureSettingsRepoStub
}

func (r *anthropicSamplingFilterSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func TestFilterAnthropicSamplingParameters(t *testing.T) {
	body := []byte(`{
		"model":"claude-opus-4-8",
		"temperature":0.7,
		"top_k":40,
		"top_p":0.9,
		"metadata":{"temperature":0.2},
		"messages":[{"role":"user","content":"hello"}]
	}`)
	settings := DefaultGatewaySettings()
	settings.AnthropicSamplingParameterFilterEnabled = true
	settings.AnthropicSamplingParameterFilterModels = []string{"claude-opus-*"}

	filtered, changed, err := filterAnthropicSamplingParameters(body, "claude-opus-4-8", settings)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(filtered, "temperature").Exists())
	require.False(t, gjson.GetBytes(filtered, "top_k").Exists())
	require.False(t, gjson.GetBytes(filtered, "top_p").Exists())
	require.Equal(t, 0.2, gjson.GetBytes(filtered, "metadata.temperature").Float())
	require.Equal(t, "hello", gjson.GetBytes(filtered, "messages.0.content").String())
}

func TestFilterAnthropicSamplingParameters_精确匹配且仅删除已有根级字段(t *testing.T) {
	body := []byte(`{"model":"claude-opus-4-8","top_p":0.9,"metadata":{"top_p":0.2}}`)
	settings := GatewaySettings{
		AnthropicSamplingParameterFilterEnabled: true,
		AnthropicSamplingParameterFilterModels:  []string{"claude-opus-4-8"},
	}

	filtered, changed, err := filterAnthropicSamplingParameters(body, "claude-opus-4-8", settings)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(filtered, "temperature").Exists())
	require.False(t, gjson.GetBytes(filtered, "top_k").Exists())
	require.False(t, gjson.GetBytes(filtered, "top_p").Exists())
	require.Equal(t, 0.2, gjson.GetBytes(filtered, "metadata.top_p").Float())
}

func TestFilterAnthropicSamplingParameters_DisabledOrModelMismatchLeavesBodyUnchanged(t *testing.T) {
	body := []byte(`{"temperature":1,"top_k":20,"top_p":0.95}`)
	tests := []struct {
		name     string
		model    string
		settings GatewaySettings
	}{
		{name: "开关关闭", model: "claude-opus-4-8", settings: DefaultGatewaySettings()},
		{
			name:  "模型不匹配",
			model: "claude-sonnet-4-6",
			settings: GatewaySettings{
				AnthropicSamplingParameterFilterEnabled: true,
				AnthropicSamplingParameterFilterModels:  []string{"claude-opus-4-8"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, changed, err := filterAnthropicSamplingParameters(body, tt.model, tt.settings)
			require.NoError(t, err)
			require.False(t, changed)
			require.Equal(t, body, filtered)
		})
	}
}

func TestFilterAnthropicSamplingParameters_MatchedInvalidJSONReturnsError(t *testing.T) {
	settings := GatewaySettings{
		AnthropicSamplingParameterFilterEnabled: true,
		AnthropicSamplingParameterFilterModels:  []string{"claude-opus-4-8"},
	}
	_, _, err := filterAnthropicSamplingParameters([]byte(`{"temperature":`), "claude-opus-4-8", settings)
	require.Error(t, err)
}

func TestGatewayMessagesBuilders_FilterSamplingParameters(t *testing.T) {
	settingService := NewSettingService(&anthropicSamplingFilterSettingsRepo{
		customFeatureSettingsRepoStub: &customFeatureSettingsRepoStub{values: map[string]string{
			SettingKeyGatewayAnthropicSamplingParameterFilterEnabled: "true",
			SettingKeyGatewayAnthropicSamplingParameterFilterModels:  `["vendor-opus-*"]`,
		}},
	}, &config.Config{})
	svc := &GatewayService{cfg: &config.Config{}, settingService: settingService}
	account := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
	body := []byte(`{"model":"vendor-opus-4-8","messages":[],"temperature":1,"top_k":10,"top_p":0.9}`)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(betaPolicyFilterSetKey, map[string]struct{}{})

	_, standardBody, err := svc.buildUpstreamRequest(
		context.Background(), c, account, body, "key", "apikey", "vendor-opus-4-8", false, false,
	)
	require.NoError(t, err)
	assertAnthropicSamplingParametersRemoved(t, standardBody)

	_, passthroughBody, err := svc.buildUpstreamRequestAnthropicAPIKeyPassthrough(
		context.Background(), c, account, body, "key",
	)
	require.NoError(t, err)
	assertAnthropicSamplingParametersRemoved(t, passthroughBody)

	oauthAccount := &Account{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	_, oauthMimicBody, err := svc.buildUpstreamRequest(
		context.Background(), c, oauthAccount, body, "oauth-token", "oauth", "vendor-opus-4-8", true, true,
	)
	require.NoError(t, err)
	assertAnthropicSamplingParametersRemoved(t, oauthMimicBody)
}

func assertAnthropicSamplingParametersRemoved(t *testing.T, body []byte) {
	t.Helper()
	for _, parameter := range anthropicDeprecatedSamplingParameters {
		require.Falsef(t, gjson.GetBytes(body, parameter).Exists(), "%s 应被过滤", parameter)
	}
}
