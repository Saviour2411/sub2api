package service

import "strings"

func lastOpenAIModelSegment(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		model = parts[len(parts)-1]
	}
	return strings.TrimSpace(model)
}

func canonicalizeOpenAIModelAliasSpelling(model string) string {
	model = strings.ToLower(lastOpenAIModelSegment(model))
	if model == "" {
		return ""
	}

	normalized := strings.ReplaceAll(model, "_", "-")
	normalized = strings.Join(strings.Fields(normalized), "-")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}

	if strings.HasPrefix(normalized, "gpt5") {
		normalized = "gpt-5" + strings.TrimPrefix(normalized, "gpt5")
	}
	if !strings.HasPrefix(normalized, "gpt-") && !strings.Contains(normalized, "codex") {
		return ""
	}

	replacements := []struct {
		from string
		to   string
	}{
		{"gpt-5.4mini", "gpt-5.4-mini"},
		{"gpt-5.4nano", "gpt-5.4-nano"},
		{"gpt-5.3-codexspark", "gpt-5.3-codex-spark"},
		{"gpt-5.3codexspark", "gpt-5.3-codex-spark"},
		{"gpt-5.3codex", "gpt-5.3-codex"},
	}
	for _, replacement := range replacements {
		normalized = strings.ReplaceAll(normalized, replacement.from, replacement.to)
	}
	return normalized
}

func normalizeKnownOpenAICodexModel(model string) string {
	normalized := canonicalizeOpenAIModelAliasSpelling(model)
	if normalized == "" {
		return ""
	}

	if mapped := getNormalizedCodexModel(normalized); mapped != "" {
		return mapped
	}
	if strings.HasSuffix(normalized, "-openai-compact") {
		if mapped := getNormalizedCodexModel(strings.TrimSuffix(normalized, "-openai-compact")); mapped != "" {
			return mapped
		}
	}

	switch {
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.6-sol"):
		return "gpt-5.6-sol"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.6-terra"):
		return "gpt-5.6-terra"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.6-luna"):
		return "gpt-5.6-luna"
	case normalized == "gpt-5.6":
		return "gpt-5.6-sol"
	case strings.HasPrefix(normalized, "gpt-5.6-"):
		suffix := strings.TrimPrefix(normalized, "gpt-5.6-")
		if suffix == "max" || isKnownCodexModelSuffix(suffix) {
			return "gpt-5.6-sol"
		}
		return ""
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.5-pro"):
		return "gpt-5.5-pro"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.5"):
		return "gpt-5.5"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.4-mini"):
		return "gpt-5.4-mini"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.4-nano"):
		return "gpt-5.4-nano"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.4"):
		return "gpt-5.4"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.2"):
		return "gpt-5.2"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.3-codex-spark"):
		return "gpt-5.3-codex-spark"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.3-codex"):
		return "gpt-5.3-codex"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.3"):
		return "gpt-5.3-codex"
	default:
		return ""
	}
}

// normalizeKnownOpenAIPricingModel 只归并官方已知的计费同价型号。它与 Codex
// 上游路由规范化分开，避免把 Pro 等独立价格型号错误映射到基础型号。
func normalizeKnownOpenAIPricingModel(model string) string {
	normalized := canonicalizeOpenAIModelAliasSpelling(model)
	if normalized == "" {
		return ""
	}
	normalized = strings.TrimSuffix(normalized, "-openai-compact")

	switch {
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.6-sol"):
		return "gpt-5.6-sol"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.6-terra"):
		return "gpt-5.6-terra"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.6-luna"):
		return "gpt-5.6-luna"
	case normalized == "gpt-5.6":
		return "gpt-5.6-sol"
	case strings.HasPrefix(normalized, "gpt-5.6-"):
		suffix := strings.TrimPrefix(normalized, "gpt-5.6-")
		if suffix == "max" || isKnownCodexModelSuffix(suffix) {
			return "gpt-5.6-sol"
		}
		return ""
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.5-pro"):
		return "gpt-5.5-pro"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.5"):
		return "gpt-5.5"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.4-pro"):
		return "gpt-5.4-pro"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.4-mini"):
		return "gpt-5.4-mini"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.4-nano"):
		return "gpt-5.4-nano"
	case normalized == "gpt-5.4-chat-latest" || matchesKnownOpenAIModelVariant(normalized, "gpt-5.4"):
		return "gpt-5.4"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.3-codex-spark"):
		return "gpt-5.3-codex-spark"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.3-codex"):
		return "gpt-5.3-codex"
	case normalized == "gpt-5.3-chat-latest" || matchesKnownOpenAIModelVariant(normalized, "gpt-5.3"):
		return "gpt-5.3-codex"
	case matchesKnownOpenAIModelVariant(normalized, "gpt-5.2-pro"):
		return "gpt-5.2-pro"
	case normalized == "gpt-5.2-chat-latest" || normalized == "gpt-5.2-codex" ||
		matchesKnownOpenAIModelVariant(normalized, "gpt-5.2"):
		return "gpt-5.2"
	case normalized == "gpt-5.1-chat-latest" || matchesKnownOpenAIModelVariant(normalized, "gpt-5.1"):
		return "gpt-5.1"
	case normalized == "gpt-5.1-codex" || normalized == "gpt-5.1-codex-max" || normalized == "gpt-5.1-codex-mini":
		return "gpt-5.3-codex"
	case normalized == "codex-mini-latest" || normalized == "gpt-5-codex":
		return "gpt-5.3-codex"
	case normalized == "gpt-5" || normalized == "gpt-5-mini" || normalized == "gpt-5-nano":
		return "gpt-5.4"
	default:
		return ""
	}
}

func matchesKnownOpenAIModelVariant(model, base string) bool {
	if model == base {
		return true
	}
	suffix, ok := strings.CutPrefix(model, base+"-")
	return ok && (isKnownCodexModelSuffix(suffix) ||
		(strings.HasPrefix(base, "gpt-5.6-") && (suffix == "max" || suffix == "preview")))
}

// isOpenAIGPT56Model 判断是否 GPT-5.6 系列模型；入参可为原始模型名
// （含大小写/路径/后缀变体）或已归一化的基名，两者均能正确识别。
func isOpenAIGPT56Model(model string) bool {
	switch normalizeKnownOpenAICodexModel(model) {
	case "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna":
		return true
	default:
		return false
	}
}

func appendUsageBillingModelCandidate(candidates []string, seen map[string]struct{}, model string) []string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return candidates
	}
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		candidates = append(candidates, candidate)
	}

	add(trimmed)
	if canonical := canonicalizeOpenAIModelAliasSpelling(trimmed); canonical != "" {
		add(canonical)
	}
	if normalized := normalizeKnownOpenAIPricingModel(trimmed); normalized != "" {
		add(normalized)
	}
	return candidates
}

func usageBillingModelCandidates(primary string, alternates ...string) []string {
	seen := make(map[string]struct{}, 1+len(alternates))
	candidates := appendUsageBillingModelCandidate(nil, seen, primary)
	for _, alternate := range alternates {
		candidates = appendUsageBillingModelCandidate(candidates, seen, alternate)
	}
	return candidates
}

func firstUsageBillingModel(candidates []string) string {
	for _, candidate := range candidates {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
