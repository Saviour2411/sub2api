package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const (
	FirstTokenTimeoutScopeAll            = "all"
	FirstTokenTimeoutScopeSelectedGroups = "selected_groups"
)

type firstTokenScopedRequestKey struct{}

func marshalFirstTokenTimeoutGroupIDs(ids []int64) string {
	if ids == nil {
		ids = []int64{}
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	data, _ := json.Marshal(ids)
	return string(data)
}

func firstTokenTimeoutApplies(ctx context.Context, settings GatewaySettings) bool {
	if settings.FirstTokenTimeoutScope != FirstTokenTimeoutScopeSelectedGroups {
		return true
	}
	if ctx == nil {
		return false
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	return group != nil && group.ID > 0 && slices.Contains(settings.FirstTokenTimeoutGroupIDs, group.ID)
}

func markFirstTokenScopedRequest(ctx context.Context, c *gin.Context) context.Context {
	ctx = context.WithValue(ctx, firstTokenScopedRequestKey{}, true)
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), firstTokenScopedRequestKey{}, true))
	}
	return ctx
}

func isFirstTokenScopedRequest(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(firstTokenScopedRequestKey{}).(bool)
	return value
}

func isStrictFirstTokenRequest(c *gin.Context) bool {
	return c != nil && c.Request != nil && (isFirstTokenScopedRequest(c.Request.Context()) || firstTokenRecoveryProbeFromContext(c.Request.Context()) != nil)
}

func firstTokenTextMeaningful(text string, strict ...bool) bool {
	if len(strict) > 0 && strict[0] {
		text = strings.TrimSpace(text)
	}
	return text != ""
}

func (s *RateLimitService) resetFirstTokenTimeoutStreakForRequest(ctx context.Context, accountID int64, event AccountFailureStreakEvent) {
	if firstTokenRecoveryProbeFromContext(ctx) != nil {
		return
	}
	settings := s.gatewayFailureSettings(ctx)
	if settings.FirstTokenTimeoutScope == FirstTokenTimeoutScopeSelectedGroups &&
		(!isFirstTokenScopedRequest(ctx) || !firstTokenTimeoutApplies(ctx, settings)) {
		return
	}
	s.resetFailureStreakEvent(ctx, accountID, AccountFailureStreakSourceFirstTokenTimeout, 0, event)
}
