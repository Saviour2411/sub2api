package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// sanitizeClaudeCodeOAuthMimicryBody 在工具名映射和代理缓存断点之前，按最终
// anthropic-beta 清理 OAuth 模拟请求。调用方应先缓存当前账号和最终模型的 beta policy。
func (s *GatewayService) sanitizeClaudeCodeOAuthMimicryBody(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	model string,
	endpoint anthropicMimicEndpoint,
) []byte {
	clientHeaders := http.Header{}
	if c != nil && c.Request != nil {
		clientHeaders = c.Request.Header
	}
	dropSet := mergeDropSets(s.getBetaPolicyFilterSet(ctx, c, account, model))
	finalBeta := ""
	if endpoint == anthropicMimicEndpointCountTokens {
		finalBeta, _ = s.computeFinalCountTokensAnthropicBeta("oauth", true, model, clientHeaders, body, dropSet)
	} else {
		finalBeta, _ = s.computeFinalAnthropicBeta("oauth", true, model, clientHeaders, body, dropSet)
	}
	return sanitizeClaudeCodeMimicryBody(body, model, finalBeta, endpoint)
}

// rememberClaudeCodeMimicSessionID 在 Count Tokens 删除 metadata 前保存 session。
// 优先使用严格合法的 metadata，其次使用合法入站头，最后按稳定会话锚点生成。
func rememberClaudeCodeMimicSessionID(c *gin.Context, account *Account, body []byte, clientDiscriminator string) {
	if c == nil || account == nil {
		return
	}
	rawUserID := gjson.GetBytes(body, "metadata.user_id").String()
	if sessionID := preferredClaudeCodeMimicSessionID(c, rawUserID); sessionID != "" {
		storeClaudeCodeMimicSession(c, account, sessionID)
		return
	}
	sessionID := generateSessionUUID(buildStableSessionSeed(
		account.ID,
		clientDiscriminator,
		extractFirstUserText(body),
	))
	storeClaudeCodeMimicSession(c, account, sessionID)
}

func preferredClaudeCodeMimicSessionID(c *gin.Context, rawUserID string) string {
	if IsValidClaudeCodeMetadataUserID(rawUserID) {
		if parsed := ParseMetadataUserID(rawUserID); parsed != nil {
			return parsed.SessionID
		}
	}
	if c == nil || c.Request == nil {
		return ""
	}
	candidate := strings.TrimSpace(c.GetHeader("X-Claude-Code-Session-Id"))
	if parsed, err := uuid.Parse(candidate); err == nil && strings.EqualFold(candidate, parsed.String()) {
		return parsed.String()
	}
	return ""
}

func storeClaudeCodeMimicSession(c *gin.Context, account *Account, sessionID string) {
	if c == nil || account == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	c.Set(claudeCodeMimicSessionIDKey, claudeCodeMimicSession{
		accountID: account.ID,
		sessionID: strings.TrimSpace(sessionID),
	})
}
