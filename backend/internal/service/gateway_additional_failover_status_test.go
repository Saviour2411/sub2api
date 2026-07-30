//go:build unit

package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newAdditionalFailoverSettingService(enabled bool, codes ...int) *SettingService {
	settings := DefaultGatewaySettings()
	settings.AdditionalFailoverStatusCodesEnabled = enabled
	settings.AdditionalFailoverStatusCodes = append([]int(nil), codes...)
	settingService := &SettingService{}
	settingService.storeGatewaySettingsCache(settings, time.Hour)
	return settingService
}

func TestAdditionalFailoverStatus_AppliesToAllGatewayServices(t *testing.T) {
	disabled := newAdditionalFailoverSettingService(false, http.StatusUnavailableForLegalReasons)
	require.False(t, (&GatewayService{settingService: disabled}).shouldFailoverUpstreamError(http.StatusUnavailableForLegalReasons))
	require.False(t, (&OpenAIGatewayService{settingService: disabled}).shouldFailoverUpstreamError(http.StatusUnavailableForLegalReasons))
	require.False(t, (&AntigravityGatewayService{settingService: disabled}).shouldFailoverUpstreamError(http.StatusUnavailableForLegalReasons))
	require.False(t, (&GeminiMessagesCompatService{settingService: disabled}).shouldFailoverGeminiUpstreamError(http.StatusUnavailableForLegalReasons))

	enabled := newAdditionalFailoverSettingService(true, http.StatusConflict, http.StatusUnavailableForLegalReasons)
	require.True(t, (&GatewayService{settingService: enabled}).shouldFailoverUpstreamError(http.StatusUnavailableForLegalReasons))
	require.True(t, (&OpenAIGatewayService{settingService: enabled}).shouldFailoverUpstreamError(http.StatusUnavailableForLegalReasons))
	require.True(t, (&AntigravityGatewayService{settingService: enabled}).shouldFailoverUpstreamError(http.StatusUnavailableForLegalReasons))
	require.True(t, (&GeminiMessagesCompatService{settingService: enabled}).shouldFailoverGeminiUpstreamError(http.StatusUnavailableForLegalReasons))
	require.False(t, (&GatewayService{settingService: enabled}).shouldFailoverUpstreamError(http.StatusUnprocessableEntity))
}

func TestAdditionalFailoverStatus_DoesNotChangeBuiltInRules(t *testing.T) {
	settingService := newAdditionalFailoverSettingService(false, http.StatusUnavailableForLegalReasons)
	require.True(t, (&GatewayService{settingService: settingService}).shouldFailoverUpstreamError(http.StatusTooManyRequests))
	require.True(t, (&GatewayService{settingService: settingService}).shouldFailoverUpstreamError(http.StatusBadGateway))
	require.True(t, (&OpenAIGatewayService{settingService: settingService}).shouldFailoverUpstreamError(http.StatusPaymentRequired))
	require.True(t, (&AntigravityGatewayService{settingService: settingService}).shouldFailoverUpstreamError(http.StatusServiceUnavailable))
	require.True(t, (&GeminiMessagesCompatService{settingService: settingService}).shouldFailoverGeminiUpstreamError(http.StatusBadGateway))
}

func TestAdditionalFailoverStatus_PreservesSemanticTerminalRules(t *testing.T) {
	settingService := newAdditionalFailoverSettingService(true, http.StatusBadRequest, http.StatusForbidden)
	openAIService := &OpenAIGatewayService{settingService: settingService}

	contextBody := []byte(`{"error":{"message":"maximum context length exceeded"}}`)
	require.False(t, openAIService.shouldFailoverOpenAIUpstreamResponse(
		http.StatusBadRequest,
		"maximum context length exceeded",
		contextBody,
	))

	cyberBody := []byte(`{"error":{"code":"cyber_policy","message":"request blocked"}}`)
	require.False(t, openAIService.shouldFailoverOpenAIUpstreamResponse(
		http.StatusBadRequest,
		"request blocked",
		cyberBody,
	))

	grokBody := []byte(`{"response":{"error":{"code":"content_policy_violation"}}}`)
	require.False(t, openAIService.shouldFailoverGrokUpstreamError(http.StatusForbidden, grokBody))

	openAIAPIKeyAccount := &Account{Type: AccountTypeAPIKey}
	require.False(t, openAIService.shouldFailoverOpenAIPassthroughResponse(
		openAIAPIKeyAccount,
		http.StatusBadRequest,
		contextBody,
	))
	require.False(t, openAIService.shouldFailoverOpenAIPassthroughResponse(
		openAIAPIKeyAccount,
		http.StatusBadRequest,
		cyberBody,
	))
	require.False(t, openAIService.shouldFailoverOpenAIPassthroughResponse(
		&Account{Platform: PlatformGrok, Type: AccountTypeAPIKey},
		http.StatusForbidden,
		grokBody,
	))
}

func TestAdditionalFailoverStatus_AppliesToOpenAIPassthrough(t *testing.T) {
	account := &Account{Type: AccountTypeOAuth}
	disabled := &OpenAIGatewayService{
		settingService: newAdditionalFailoverSettingService(false, http.StatusUnavailableForLegalReasons),
	}
	require.False(t, disabled.shouldFailoverOpenAIPassthroughResponse(
		account,
		http.StatusUnavailableForLegalReasons,
		nil,
	))

	enabled := &OpenAIGatewayService{
		settingService: newAdditionalFailoverSettingService(true, http.StatusUnavailableForLegalReasons),
	}
	require.True(t, enabled.shouldFailoverOpenAIPassthroughResponse(
		account,
		http.StatusUnavailableForLegalReasons,
		nil,
	))
}
