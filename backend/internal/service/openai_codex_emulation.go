package service

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

const openAICodexCLIEmulationExtraKey = "openai_codex_cli_emulation_enabled"

func resolveOpenAICodexUserAgent(ctx context.Context, settingService *SettingService) string {
	if settingService != nil {
		if value := strings.TrimSpace(settingService.GetOpenAICodexUserAgent(ctx)); value != "" {
			return value
		}
	}
	return DefaultOpenAICodexUserAgent
}

func applyOpenAICodexCLIEmulationHeaders(
	ctx context.Context,
	headers http.Header,
	account *Account,
	settingService *SettingService,
	useResponsesAPI bool,
) {
	if headers == nil || account == nil || !account.IsOpenAICodexCLIEmulationEnabled() {
		return
	}
	headers.Set("user-agent", resolveOpenAICodexUserAgent(ctx, settingService))
	headers.Set("originator", "codex_cli_rs")
	headers.Set("version", codexCLIVersion)
	if useResponsesAPI {
		headers.Set("OpenAI-Beta", "responses=experimental")
	}
}

func buildOpenAICodexProbeSessionID(accountID int64) string {
	if accountID <= 0 {
		return "probe_codex_cli_emulation"
	}
	return "probe_codex_cli_emulation_" + strconv.FormatInt(accountID, 10)
}
