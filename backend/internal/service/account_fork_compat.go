package service

import openai_compat "github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"

func (a *Account) IsOpenAICodexCLIEmulationEnabled() bool {
	if a == nil || !a.IsOpenAI() || a.Extra == nil {
		return false
	}
	enabled, ok := a.Extra[openAICodexCLIEmulationExtraKey].(bool)
	return ok && enabled
}

func (a *Account) ShouldUseOpenAIResponsesAPI() bool {
	if a == nil || !a.IsOpenAI() || a.Type != AccountTypeAPIKey {
		return true
	}
	if !a.IsOpenAICodexCLIEmulationEnabled() {
		return openai_compat.ShouldUseResponsesAPI(a.Extra)
	}
	mode, _ := a.Extra[openai_compat.ExtraKeyResponsesMode].(string)
	return openai_compat.NormalizeResponsesSupportMode(mode) != openai_compat.ResponsesSupportModeForceChatCompletions
}
