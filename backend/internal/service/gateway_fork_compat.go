package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type PreResponseKeepaliveContextKey struct{}

const claudeCodeMimicSessionIDKey = "claude_code_mimic_session_id"

type claudeCodeMimicSession struct {
	accountID int64
	sessionID string
}

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

func (s *GatewayService) applyClaudeCodeAPIKeyMimicryToBody(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	model string,
	endpoint anthropicMimicEndpoint,
) ([]byte, string) {
	if account == nil || len(body) == 0 {
		return body, model
	}
	// API Key 上游模拟只补齐 CC 身份特征，原始 system blocks 原样保留。
	// OAuth 的 system-to-messages 重写会改变 prompt cache 前缀，不适用于这里。
	body = prependClaudeCodeMimicSystemBlocks(body)
	metadataUserID := buildAPIKeyMimicMetadataUserID(c, account, body)
	storeClaudeCodeMimicSessionID(c, account, metadataUserID)
	body = forceClaudeCodeMetadataUserID(body, metadataUserID)
	normalizeOpts := claudeOAuthNormalizeOptions{
		stripSystemCacheControl: false,
		preserveSystemText:      true,
		preserveModel:           true,
	}
	body, model = normalizeClaudeOAuthRequestBody(body, model, normalizeOpts)
	finalBeta := s.resolveAPIKeyMimicFinalBeta(ctx, c, account, body, model, endpoint)
	body = sanitizeClaudeCodeMimicryBody(body, model, finalBeta, endpoint)
	if s.isRewriteMessageCacheControlEnabled(ctx) {
		body = addMessageCacheBreakpointsPreservingClient(body)
	}
	if rw := buildToolNameRewriteFromBody(body); rw != nil {
		body = applyToolNameRewriteToBody(body, rw)
		if c != nil {
			c.Set(toolNameRewriteKey, rw)
		}
	} else {
		body = applyToolsLastCacheBreakpoint(body)
	}
	return enforceCacheControlLimit(body), model
}

func (s *GatewayService) resolveAPIKeyMimicFinalBeta(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	model string,
	endpoint anthropicMimicEndpoint,
) string {
	clientHeaders := http.Header{}
	if c != nil && c.Request != nil {
		clientHeaders = c.Request.Header
	}
	dropSet := mergeDropSets(s.getBetaPolicyFilterSet(ctx, c, account, model))
	betaBody := body
	if bodyModel := gjson.GetBytes(body, "model").String(); model != "" && bodyModel != model {
		betaBody = ReplaceModelInBody(body, model)
	}
	var finalBeta string
	if endpoint == anthropicMimicEndpointCountTokens {
		finalBeta, _ = s.computeFinalCountTokensAnthropicBeta("apikey", true, model, clientHeaders, betaBody, dropSet)
	} else {
		finalBeta, _ = s.computeFinalAnthropicBeta("apikey", true, model, clientHeaders, betaBody, dropSet)
	}
	return mergeAPIKeyMimicAccountBeta(finalBeta, account, dropSet)
}

func prependClaudeCodeMimicSystemBlocks(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	billingText, err := buildBillingAttributionText(body, claude.CLICurrentVersion)
	if err != nil {
		return body
	}
	billingBlock, err := marshalAnthropicSystemTextBlock(billingText, false)
	if err != nil {
		return body
	}
	identityBlock, err := marshalAnthropicSystemTextBlock(strings.TrimSpace(claudeCodeSystemPrompt), false)
	if err != nil {
		return body
	}

	system := gjson.GetBytes(body, "system")
	originalBlocks := make([][]byte, 0, 4)
	switch {
	case system.Type == gjson.String:
		text := system.String()
		if raw, marshalErr := marshalAnthropicSystemTextBlock(text, false); marshalErr == nil {
			originalBlocks = append(originalBlocks, raw)
		}
	case system.IsArray():
		system.ForEach(func(_, block gjson.Result) bool {
			originalBlocks = append(originalBlocks, []byte(block.Raw))
			return true
		})
	}

	blocks := make([][]byte, 0, len(originalBlocks)+2)
	originalIndex := 0
	if len(originalBlocks) > 0 && gjson.GetBytes(originalBlocks[0], "text").String() == billingText {
		blocks = append(blocks, originalBlocks[0])
		originalIndex++
	} else {
		blocks = append(blocks, billingBlock)
	}
	if originalIndex < len(originalBlocks) && hasClaudeCodePrefix(gjson.GetBytes(originalBlocks[originalIndex], "text").String()) {
		blocks = append(blocks, originalBlocks[originalIndex])
		originalIndex++
	} else {
		blocks = append(blocks, identityBlock)
	}
	blocks = append(blocks, originalBlocks[originalIndex:]...)
	if len(blocks) == 0 {
		return body
	}
	if next, ok := setJSONRawBytes(body, "system", buildJSONArrayRaw(blocks)); ok {
		return next
	}
	return body
}

func isCompleteClaudeCodeBillingBlock(text string) bool {
	return strings.HasPrefix(text, claudeCodeBillingHeaderPrefix) &&
		strings.Contains(text, claudeCodeEntrypointMarker)
}

func claudeCodeMimicClientDiscriminator(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.ClientIP() + ":" + NormalizeSessionUserAgent(c.Request.UserAgent())
}

func claudeCodeMimicRequestDiscriminator(c *gin.Context, parsed *ParsedRequest) string {
	if parsed != nil {
		if discriminator := sessionContextDiscriminator(parsed.SessionContext); discriminator != "" {
			return discriminator
		}
	}
	return claudeCodeMimicClientDiscriminator(c)
}

func buildAPIKeyMimicMetadataUserID(c *gin.Context, account *Account, body []byte) string {
	if account == nil {
		return ""
	}

	clientDiscriminator := claudeCodeMimicClientDiscriminator(c)
	deviceHash := sha256.Sum256([]byte("apikey-mimic-user:" + strconv.FormatInt(account.ID, 10) + ":" + clientDiscriminator))
	deviceID := hex.EncodeToString(deviceHash[:])
	accountUUID := strings.TrimSpace(account.GetExtraString("account_uuid"))
	rawUserID := gjson.GetBytes(body, "metadata.user_id").String()
	sessionID := preferredClaudeCodeMimicSessionID(c, rawUserID)
	if sessionID == "" {
		sessionID = generateSessionUUID(buildStableSessionSeed(account.ID, clientDiscriminator, extractFirstUserText(body)))
	}
	return FormatMetadataUserID(deviceID, accountUUID, sessionID, claude.CLICurrentVersion)
}

func forceClaudeCodeMetadataUserID(body []byte, userID string) []byte {
	if len(body) == 0 || strings.TrimSpace(userID) == "" {
		return body
	}
	raw, err := marshalAnthropicMetadata(userID)
	if err != nil {
		return body
	}
	if next, ok := setJSONRawBytes(body, "metadata", raw); ok {
		return next
	}
	return body
}

func storeClaudeCodeMimicSessionID(c *gin.Context, account *Account, metadataUserID string) {
	if c == nil || account == nil {
		return
	}
	parsed := ParseMetadataUserID(metadataUserID)
	if parsed == nil || parsed.SessionID == "" {
		return
	}
	storeClaudeCodeMimicSession(c, account, parsed.SessionID)
}

func claudeCodeMimicSessionID(c *gin.Context, account *Account, body []byte) string {
	if uid := gjson.GetBytes(body, "metadata.user_id").String(); IsValidClaudeCodeMetadataUserID(uid) {
		if parsed := ParseMetadataUserID(uid); parsed != nil {
			return parsed.SessionID
		}
	}
	if c == nil || account == nil {
		return ""
	}
	value, ok := c.Get(claudeCodeMimicSessionIDKey)
	if !ok {
		return ""
	}
	session, ok := value.(claudeCodeMimicSession)
	if !ok || session.accountID != account.ID {
		return ""
	}
	return strings.TrimSpace(session.sessionID)
}

func shouldMimicClaudeCodeForAccount(account *Account, isClaudeCode, anthropicAPIKeyMimicryEnabled bool) bool {
	if account == nil || isClaudeCode {
		return false
	}
	if account.IsAnthropicOAuthOrSetupToken() {
		return true
	}
	return anthropicAPIKeyMimicryEnabled &&
		account.Platform == PlatformAnthropic &&
		account.Type == AccountTypeAPIKey &&
		!account.IsAnthropicAPIKeyPassthroughEnabled()
}

func (s *GatewayService) shouldMimicClaudeCodeUpstream(ctx context.Context, account *Account, isClaudeCode bool) bool {
	enabled := false
	if s != nil && s.settingService != nil {
		enabled = s.settingService.GetGatewayRuntime(ctx).AnthropicClaudeCodeMimicryEnabled
	}
	return shouldMimicClaudeCodeForAccount(account, isClaudeCode, enabled)
}
