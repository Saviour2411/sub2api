package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	defaultSemanticErrorMatchMaxChars = 4096
	minSemanticErrorMatchMaxChars     = 128
	maxSemanticErrorMatchMaxChars     = 65536
)

type SemanticErrorConfig struct {
	Enabled       bool
	MatchMaxChars int
	Rules         []CompiledSemanticErrorRule
}

type CompiledSemanticErrorRule struct {
	Enabled       bool
	Name          string
	Platforms     []string
	MatchType     string
	Pattern       string
	CustomMessage string
	Priority      int
	regex         *regexp.Regexp
}

type SemanticErrorMatch struct {
	RuleName      string
	CustomMessage string
}

func defaultSemanticErrorConfig() SemanticErrorConfig {
	return SemanticErrorConfig{
		Enabled:       false,
		MatchMaxChars: defaultSemanticErrorMatchMaxChars,
		Rules:         nil,
	}
}

func normalizeSemanticErrorMatchMaxChars(raw any) int {
	value := defaultSemanticErrorMatchMaxChars
	switch v := raw.(type) {
	case int:
		value = v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			if parsed, err := strconv.Atoi(trimmed); err == nil {
				value = parsed
			}
		}
	}
	if value < minSemanticErrorMatchMaxChars {
		return minSemanticErrorMatchMaxChars
	}
	if value > maxSemanticErrorMatchMaxChars {
		return maxSemanticErrorMatchMaxChars
	}
	return value
}

func normalizeSemanticErrorRules(rules []SemanticErrorRule) []SemanticErrorRule {
	out := make([]SemanticErrorRule, 0, len(rules))
	for _, rule := range rules {
		name := strings.TrimSpace(rule.Name)
		pattern := strings.TrimSpace(rule.Pattern)
		message := strings.TrimSpace(rule.CustomMessage)
		if name == "" || pattern == "" || message == "" {
			continue
		}
		matchType := strings.ToLower(strings.TrimSpace(rule.MatchType))
		if matchType != "regex" {
			matchType = "contains"
		}
		out = append(out, SemanticErrorRule{
			Enabled:       rule.Enabled,
			Name:          name,
			Platforms:     normalizeSemanticErrorPlatforms(rule.Platforms),
			MatchType:     matchType,
			Pattern:       pattern,
			CustomMessage: message,
			Priority:      rule.Priority,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].Name < out[j].Name
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func ValidateSemanticErrorRules(rules []SemanticErrorRule) error {
	for _, rule := range rules {
		name := strings.TrimSpace(rule.Name)
		pattern := strings.TrimSpace(rule.Pattern)
		message := strings.TrimSpace(rule.CustomMessage)
		if name == "" && pattern == "" && message == "" {
			continue
		}
		if name == "" {
			return infraerrors.BadRequest("INVALID_SEMANTIC_ERROR_RULE", "semantic error rule name is required")
		}
		if pattern == "" {
			return infraerrors.BadRequest("INVALID_SEMANTIC_ERROR_RULE", "semantic error rule pattern is required")
		}
		if message == "" {
			return infraerrors.BadRequest("INVALID_SEMANTIC_ERROR_RULE", "semantic error rule custom_message is required")
		}
		matchType := strings.ToLower(strings.TrimSpace(rule.MatchType))
		if matchType != "" && matchType != "contains" && matchType != "regex" {
			return infraerrors.BadRequest("INVALID_SEMANTIC_ERROR_RULE", "semantic error rule match_type must be contains or regex")
		}
		if matchType == "regex" {
			if _, err := regexp.Compile(pattern); err != nil {
				return infraerrors.BadRequest("INVALID_SEMANTIC_ERROR_RULE", fmt.Sprintf("semantic error rule %q regex invalid: %v", name, err))
			}
		}
	}
	return nil
}

func normalizeSemanticErrorPlatforms(values []string) []string {
	allowed := map[string]struct{}{
		PlatformAnthropic:   {},
		PlatformOpenAI:      {},
		PlatformGemini:      {},
		PlatformAntigravity: {},
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		platform := strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[platform]; !ok {
			continue
		}
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		out = append(out, platform)
	}
	sort.Strings(out)
	return out
}

func compileSemanticErrorRules(rules []SemanticErrorRule) []CompiledSemanticErrorRule {
	normalized := normalizeSemanticErrorRules(rules)
	out := make([]CompiledSemanticErrorRule, 0, len(normalized))
	for _, rule := range normalized {
		compiled := CompiledSemanticErrorRule{
			Enabled:       rule.Enabled,
			Name:          rule.Name,
			Platforms:     rule.Platforms,
			MatchType:     rule.MatchType,
			Pattern:       rule.Pattern,
			CustomMessage: rule.CustomMessage,
			Priority:      rule.Priority,
		}
		if compiled.MatchType == "regex" {
			re, err := regexp.Compile(compiled.Pattern)
			if err != nil {
				slog.Warn("semantic_error_rule_regex_invalid", "rule", compiled.Name, "error", err)
				continue
			}
			compiled.regex = re
		}
		out = append(out, compiled)
	}
	return out
}

func parseSemanticErrorRules(raw string) []SemanticErrorRule {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var rules []SemanticErrorRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		slog.Warn("semantic_error_rules_parse_failed", "error", err)
		return nil
	}
	return normalizeSemanticErrorRules(rules)
}

// GetSemanticErrorConfig returns the cached 2xx semantic error detection config.
func (s *SettingService) GetSemanticErrorConfig(ctx context.Context) SemanticErrorConfig {
	cfg := s.getGatewayForwardingSettingsCached(ctx).semanticErrorConfig
	if cfg.MatchMaxChars <= 0 {
		cfg.MatchMaxChars = defaultSemanticErrorMatchMaxChars
	}
	return cfg
}

// MatchSemanticError matches 2xx semantic error rules when the body is within the configured threshold.
func (s *SettingService) MatchSemanticError(ctx context.Context, platform string, body []byte) *SemanticErrorMatch {
	return matchSemanticError(s.GetSemanticErrorConfig(ctx), platform, body)
}
