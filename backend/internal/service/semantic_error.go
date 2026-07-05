package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const semanticErrorResponseStatus = http.StatusBadGateway

type semanticErrorPrefixDetector struct {
	config   SemanticErrorConfig
	platform string
	buffer   strings.Builder
	released bool
}

func newSemanticErrorPrefixDetector(config SemanticErrorConfig, platform string) *semanticErrorPrefixDetector {
	if config.MatchMaxChars <= 0 {
		config.MatchMaxChars = defaultSemanticErrorMatchMaxChars
	}
	return &semanticErrorPrefixDetector{
		config:   config,
		platform: strings.ToLower(strings.TrimSpace(platform)),
	}
}

func (d *semanticErrorPrefixDetector) Enabled() bool {
	return d != nil && d.config.Enabled && d.config.MatchMaxChars > 0 && len(d.config.Rules) > 0
}

func (d *semanticErrorPrefixDetector) Released() bool {
	return d == nil || d.released
}

func (d *semanticErrorPrefixDetector) Observe(text string) bool {
	if !d.Enabled() || d.released || text == "" {
		return d == nil || d.released
	}
	if d.buffer.Len()+len(text) > d.config.MatchMaxChars*4 {
		d.released = true
		return true
	}
	_, _ = d.buffer.WriteString(text)
	if utf8.RuneCountInString(d.buffer.String()) > d.config.MatchMaxChars {
		d.released = true
		return true
	}
	return false
}

func (d *semanticErrorPrefixDetector) MatchIfComplete() *SemanticErrorMatch {
	if !d.Enabled() || d.released {
		return nil
	}
	return matchSemanticError(d.config, d.platform, []byte(d.buffer.String()))
}

func matchSemanticError(config SemanticErrorConfig, platform string, body []byte) *SemanticErrorMatch {
	if !config.Enabled || len(config.Rules) == 0 || len(body) == 0 {
		return nil
	}
	maxChars := normalizeSemanticErrorMatchMaxChars(config.MatchMaxChars)
	if len(body) > maxChars*4 {
		return nil
	}
	text := string(body)
	if utf8.RuneCountInString(text) > maxChars {
		return nil
	}
	platform = strings.ToLower(strings.TrimSpace(platform))
	lowerText := strings.ToLower(text)
	for _, rule := range config.Rules {
		if !rule.Enabled || !semanticErrorRuleAppliesToPlatform(rule, platform) {
			continue
		}
		matched := false
		if rule.MatchType == "regex" && rule.regex != nil {
			matched = rule.regex.MatchString(text)
		} else {
			matched = strings.Contains(lowerText, strings.ToLower(rule.Pattern))
		}
		if matched {
			return &SemanticErrorMatch{
				RuleName:      rule.Name,
				CustomMessage: rule.CustomMessage,
			}
		}
	}
	return nil
}

func semanticErrorRuleAppliesToPlatform(rule CompiledSemanticErrorRule, platform string) bool {
	if len(rule.Platforms) == 0 {
		return true
	}
	for _, item := range rule.Platforms {
		if strings.EqualFold(item, platform) {
			return true
		}
	}
	return false
}

//nolint:unused
func writeAnthropicSemanticErrorJSON(c *gin.Context, message string) {
	c.JSON(semanticErrorResponseStatus, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    "upstream_error",
			"message": message,
		},
	})
}

func newSemanticErrorFailoverError(match *SemanticErrorMatch) *UpstreamFailoverError {
	message := "Upstream semantic error"
	ruleName := ""
	if match != nil {
		ruleName = match.RuleName
		if strings.TrimSpace(match.CustomMessage) != "" {
			message = match.CustomMessage
		}
	}
	body, _ := json.Marshal(map[string]any{
		"error": map[string]string{
			"type":    "upstream_error",
			"message": message,
		},
	})
	return &UpstreamFailoverError{
		StatusCode:            semanticErrorResponseStatus,
		ResponseBody:          body,
		SemanticError:         true,
		SemanticErrorRuleName: ruleName,
		SemanticErrorMessage:  message,
	}
}

func (s *RateLimitService) HandleSemanticFailureScheduling(ctx context.Context, account *Account, reason string) bool {
	return s.HandleStrictFailureScheduling(ctx, account, semanticErrorResponseStatus, reason)
}

func handleSemanticErrorScheduling(rateLimitService *RateLimitService, ctx context.Context, account *Account, match *SemanticErrorMatch) bool {
	if rateLimitService == nil || match == nil {
		return false
	}
	reason := "semantic error"
	if match.RuleName != "" {
		reason = fmt.Sprintf("semantic error: %s", match.RuleName)
	}
	return rateLimitService.HandleSemanticFailureScheduling(ctx, account, reason)
}

func recordSemanticErrorOps(c *gin.Context, account *Account, match *SemanticErrorMatch, body []byte, upstreamRequestID string) {
	if c == nil || match == nil {
		return
	}
	detail := ""
	if len(bytes.TrimSpace(body)) > 0 {
		detail = truncateString(string(body), 2048)
	}
	message := match.CustomMessage
	if message == "" {
		message = "Upstream semantic error"
	}
	setOpsUpstreamError(c, http.StatusOK, message, detail)
	ev := OpsUpstreamErrorEvent{
		UpstreamStatusCode: http.StatusOK,
		UpstreamRequestID:  upstreamRequestID,
		Kind:               "semantic_error",
		Message:            message,
		Detail:             detail,
	}
	if account != nil {
		ev.Platform = account.Platform
		ev.AccountID = account.ID
		ev.AccountName = account.Name
	}
	appendOpsUpstreamError(c, ev)
}
