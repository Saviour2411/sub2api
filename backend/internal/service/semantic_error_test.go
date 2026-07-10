package service

import (
	"context"
	"testing"
)

func TestMatchSemanticErrorRules(t *testing.T) {
	rules := []SemanticErrorRule{
		{
			Enabled:       true,
			Name:          "高优先级",
			MatchType:     "contains",
			Pattern:       "quota exceeded",
			CustomMessage: "额度异常",
			Priority:      10,
		},
		{
			Enabled:       true,
			Name:          "低优先级",
			MatchType:     "contains",
			Pattern:       "quota",
			CustomMessage: "通用额度异常",
			Priority:      20,
		},
	}
	cfg := SemanticErrorConfig{
		Enabled:       true,
		MatchMaxChars: 4096,
		Rules:         compileSemanticErrorRules(rules),
	}

	match := matchSemanticError(cfg, PlatformOpenAI, []byte(`{"message":"Success response quota exceeded"}`))
	if match == nil {
		t.Fatal("expected semantic error match")
		return
	}
	if match.RuleName != "高优先级" || match.CustomMessage != "额度异常" {
		t.Fatalf("unexpected match: %#v", match)
	}
}

func TestMatchSemanticErrorRespectsPlatformAndThreshold(t *testing.T) {
	cfg := SemanticErrorConfig{
		Enabled:       true,
		MatchMaxChars: 128,
		Rules: compileSemanticErrorRules([]SemanticErrorRule{{
			Enabled:       true,
			Name:          "Claude only",
			Platforms:     []string{PlatformAnthropic},
			MatchType:     "regex",
			Pattern:       `(?i)access forbidden`,
			CustomMessage: "访问被拒绝",
			Priority:      1,
		}}),
	}

	if match := matchSemanticError(cfg, PlatformOpenAI, []byte("access forbidden")); match != nil {
		t.Fatalf("expected platform mismatch, got %#v", match)
	}
	if match := matchSemanticError(cfg, PlatformAnthropic, []byte("access forbidden")); match == nil {
		t.Fatal("expected platform match")
	}
	if match := matchSemanticError(cfg, PlatformAnthropic, []byte(string(make([]byte, 129))+"access forbidden")); match != nil {
		t.Fatalf("expected over-threshold body to skip matching, got %#v", match)
	}
}

func TestValidateSemanticErrorRulesRejectsInvalidRegex(t *testing.T) {
	err := ValidateSemanticErrorRules([]SemanticErrorRule{{
		Enabled:       true,
		Name:          "bad regex",
		MatchType:     "regex",
		Pattern:       "(",
		CustomMessage: "自定义错误",
	}})
	if err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestSettingServiceMatchSemanticErrorDisabledByDefault(t *testing.T) {
	svc := &SettingService{}
	match := svc.MatchSemanticError(context.Background(), PlatformOpenAI, []byte("quota exceeded"))
	if match != nil {
		t.Fatalf("expected nil match, got %#v", match)
	}
}
