package service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const kimiParameterCompatContextKey = "kimi_parameter_compat"

type kimiParameterCompatState struct {
	settings GatewaySettings
	applied  [kimiCompatRuleCount]bool
	retries  int
}

func (s *OpenAIGatewayService) kimiParameterCompat(ctx context.Context, c *gin.Context) *kimiParameterCompatState {
	if c == nil {
		return nil
	}
	value, _ := c.Get("api_key")
	apiKey, _ := value.(*APIKey)
	if apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != PlatformKimi {
		return nil
	}
	if existing, ok := c.Get(kimiParameterCompatContextKey); ok {
		state, _ := existing.(*kimiParameterCompatState)
		return state
	}
	state := &kimiParameterCompatState{settings: s.settingService.GetGatewayRuntime(ctx)}
	c.Set(kimiParameterCompatContextKey, state)
	return state
}

func (s *OpenAIGatewayService) kimiMessagesReasoningEffort(ctx context.Context, c *gin.Context, request *apicompat.AnthropicRequest, model, converted string) string {
	state := s.kimiParameterCompat(ctx, c)
	if state == nil || !state.settings.KimiReasoningEffortRetryEnabled || model != "kimi-k3" {
		return converted
	}
	if request.OutputConfig == nil || request.OutputConfig.Effort == "" {
		return "low"
	}
	return request.OutputConfig.Effort
}

func kimiEffectiveReasoningEffort(c *gin.Context, original *string) *string {
	if c != nil {
		if value, ok := c.Get(kimiParameterCompatContextKey); ok {
			if state, ok := value.(*kimiParameterCompatState); ok && state.applied[kimiCompatReasoning] {
				effort := "low"
				return &effort
			}
		}
	}
	return original
}

func kimiCompatCanRetry(ctx context.Context, c *gin.Context, attempt *firstTokenAttempt) bool {
	if ctx.Err() != nil || c == nil || c.Writer == nil || c.Writer.Written() || IsResponseCommitted(c) {
		return false
	}
	if c.Request != nil && c.Request.Context().Err() != nil {
		return false
	}
	return attempt == nil || attempt.currentState() == firstTokenAttemptStopped
}

func (s *OpenAIGatewayService) sendCCUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	targetURL string,
	body []byte,
	stream bool,
	model string,
	bearerToken string,
	userAgent string,
	grokCacheIdentity string,
) (*http.Response, *firstTokenAttempt, error) {
	state := s.kimiParameterCompat(ctx, c)
	if state == nil {
		return s.sendCCUpstreamRequestOnce(ctx, c, account, targetURL, body, stream, model, bearerToken, userAgent, grokCacheIdentity)
	}
	for rule, applied := range state.applied {
		if applied {
			modified, _, err := applyKimiParameterRepair(body, kimiCompatRule(rule))
			if err != nil {
				return nil, nil, err
			}
			body = modified
		}
	}
	compatDispatch := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if c.Request != nil && c.Request.Context().Err() != nil {
			return nil, nil, c.Request.Context().Err()
		}
		if compatDispatch {
			logger.FromContext(ctx).Info("kimi.parameter_compat_dispatch",
				zap.Int64("account_id", account.ID), zap.Int("compat_retry", state.retries))
		}
		resp, attempt, err := s.sendCCUpstreamRequestOnce(ctx, c, account, targetURL, body, stream, model, bearerToken, userAgent, grokCacheIdentity)
		if compatDispatch && err == nil {
			logger.FromContext(ctx).Info("kimi.parameter_compat_response",
				zap.Int64("account_id", account.ID), zap.Int("compat_retry", state.retries),
				zap.Int("upstream_status", resp.StatusCode))
		}
		if err != nil || resp.StatusCode != http.StatusBadRequest || state.retries >= int(kimiCompatRuleCount) || !kimiCompatCanRetry(ctx, c, attempt) {
			return resp, attempt, err
		}
		eligible := false
		for rule := kimiCompatRule(0); rule < kimiCompatRuleCount; rule++ {
			eligible = eligible || (!state.applied[rule] && rule.enabled(state.settings))
		}
		if !eligible {
			return resp, attempt, nil
		}
		upstreamBody := resp.Body
		stopContextClose := context.AfterFunc(ctx, func() { _ = upstreamBody.Close() })
		stopClientClose := func() bool { return true }
		if c.Request != nil {
			stopClientClose = context.AfterFunc(c.Request.Context(), func() { _ = upstreamBody.Close() })
		}
		errorBody, _, readErr := s.readOpenAIUpstreamError(resp)
		stopContextClose()
		stopClientClose()
		if isOpenAIRequestSentPluginError(readErr) {
			_ = resp.Body.Close()
			return nil, nil, readErr
		}
		if readErr != nil || int64(len(errorBody)) >= openAIUpstreamErrorBodyReadLimitForConfig(s.cfg) || !kimiCompatCanRetry(ctx, c, attempt) {
			return resp, attempt, nil
		}
		rule, matched := matchKimiParameterRejection(body, errorBody)
		if !matched || state.applied[rule] || !rule.enabled(state.settings) {
			return resp, attempt, nil
		}
		modified, fields, repairErr := applyKimiParameterRepair(body, rule)
		if repairErr != nil || len(fields) == 0 {
			return resp, attempt, nil
		}
		_ = resp.Body.Close()
		state.applied[rule] = true
		state.retries++
		compatDispatch = true
		diagnostic := struct {
			Rule            string   `json:"rule"`
			ChangedFields   []string `json:"changed_fields"`
			CompatRetry     int      `json:"compat_retry"`
			RejectedEffort  string   `json:"rejected_effort,omitempty"`
			EffectiveEffort string   `json:"effective_effort,omitempty"`
		}{Rule: rule.String(), ChangedFields: fields, CompatRetry: state.retries}
		if rule == kimiCompatReasoning {
			diagnostic.RejectedEffort = sanitizeUpstreamErrorMessage(truncateString(gjson.GetBytes(body, "reasoning_effort").String(), 64))
			diagnostic.EffectiveEffort = "low"
		}
		detail, _ := json.Marshal(diagnostic)
		body = modified
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
			ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account),
			UpstreamStatusCode: http.StatusBadRequest, UpstreamRequestID: resp.Header.Get("x-request-id"),
			Kind: "parameter_compat_retry", Scope: "request", Reason: rule.String(),
			Message: "Kimi 参数拒绝兼容重试: " + rule.String(), Detail: string(detail),
		})
		logger.FromContext(ctx).Info("kimi.parameter_compat_repair",
			zap.Int64("account_id", account.ID), zap.String("rule", rule.String()),
			zap.Strings("changed_fields", fields), zap.Int("compat_retry", state.retries),
			zap.Int("upstream_status", resp.StatusCode))
	}
}
