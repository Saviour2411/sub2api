package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

type PreResponseKeepaliveContextKey struct{}

type preResponseKeepaliveStopper interface {
	Stop() bool
}

func StopPreResponseKeepaliveFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	stopper, _ := ctx.Value(PreResponseKeepaliveContextKey{}).(preResponseKeepaliveStopper)
	if stopper == nil {
		return false
	}
	return stopper.Stop()
}

type preResponseKeepaliveBeforeResponseStopper interface {
	StopBeforeResponse() bool
}

func StopPreResponseKeepaliveBeforeResponseFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if stopper, _ := ctx.Value(PreResponseKeepaliveContextKey{}).(preResponseKeepaliveBeforeResponseStopper); stopper != nil {
		return stopper.StopBeforeResponse()
	}
	return StopPreResponseKeepaliveFromContext(ctx)
}

func (s *GatewayService) applyClaudeCodeAPIKeyMimicryToBody(ctx context.Context, c *gin.Context, account *Account, body []byte, systemRaw any, model string) ([]byte, string) {
	if account == nil || len(body) == 0 {
		return body, model
	}
	systemPromptInjectionEnabled, systemPrompt, systemPromptBlocks := s.claudeOAuthSystemPromptInjectionSettings(ctx)
	systemRewritten := false
	if systemPromptInjectionEnabled && !strings.Contains(strings.ToLower(model), "haiku") {
		body = rewriteSystemForNonClaudeCodeWithPromptBlocks(body, normalizeSystemParam(systemRaw), systemPrompt, systemPromptBlocks)
		systemRewritten = true
	}
	normalizeOpts := claudeOAuthNormalizeOptions{stripSystemCacheControl: !systemRewritten}
	if uid := s.buildOAuthMetadataUserIDFromBody(ctx, account, nil, body); uid != "" {
		normalizeOpts.injectMetadata = true
		normalizeOpts.metadataUserID = uid
	}
	body, model = normalizeClaudeOAuthRequestBody(body, model, normalizeOpts)
	body = s.rewriteMessageCacheControlIfEnabled(ctx, body)
	if rw := buildToolNameRewriteFromBody(body); rw != nil {
		body = applyToolNameRewriteToBody(body, rw)
		if c != nil {
			c.Set(toolNameRewriteKey, rw)
		}
	} else {
		body = applyToolsLastCacheBreakpoint(body)
	}
	return body, model
}

func groupFromContextAny(ctx context.Context) *Group {
	if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && IsGroupContextValid(group) {
		return group
	}
	return nil
}

func (s *GatewayService) shouldMimicClaudeCodeUpstream(ctx context.Context, account *Account, groupID *int64, isClaudeCode bool) bool {
	if account == nil || isClaudeCode {
		return false
	}
	if account.IsOAuth() {
		return true
	}
	if account.Platform != PlatformAnthropic {
		return false
	}
	var group *Group
	if groupID != nil && *groupID > 0 {
		group = s.groupFromContext(ctx, *groupID)
	}
	if group == nil {
		group = groupFromContextAny(ctx)
	}
	return group != nil && group.Platform == PlatformAnthropic && group.ClaudeCodeUpstreamMimicry
}
