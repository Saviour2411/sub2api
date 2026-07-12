package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOAuthMetadataUserID_FallbackWithoutAccountUUID(t *testing.T) {
	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
	}

	account := &Account{
		ID:    123,
		Type:  AccountTypeOAuth,
		Extra: map[string]any{}, // intentionally missing account_uuid / claude_user_id
	}

	fp := &Fingerprint{ClientID: "deadbeef"}

	got := svc.buildOAuthMetadataUserID(nil, parsed, account, fp)
	require.NotEmpty(t, got)
	metadata := ParseMetadataUserID(got)
	require.NotNil(t, metadata)
	require.True(t, metadata.IsNewFormat)
	require.Len(t, metadata.DeviceID, 64)
	require.True(t, IsValidClaudeCodeMetadataUserID(got))
}

func TestBuildOAuthMetadataUserID_UsesAccountUUIDWhenPresent(t *testing.T) {
	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
	}

	account := &Account{
		ID:   123,
		Type: AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid":      "acc-uuid",
			"claude_user_id":    "clientid123",
			"anthropic_user_id": "",
		},
	}

	got := svc.buildOAuthMetadataUserID(nil, parsed, account, nil)
	require.NotEmpty(t, got)
	metadata := ParseMetadataUserID(got)
	require.NotNil(t, metadata)
	require.True(t, metadata.IsNewFormat)
	require.Len(t, metadata.DeviceID, 64)
	require.Equal(t, "acc-uuid", metadata.AccountUUID)
	require.True(t, IsValidClaudeCodeMetadataUserID(got))
}

// TestBuildOAuthMetadataUserID_SessionIDStableAcrossTurns 验证伪装路径合成的
// metadata.user_id 在同一会话多轮请求间保持不变（session_id 稳定），贴近真实 Claude Code
// 进程级稳定的 session。账号 / 指纹 / UA 版本均相同，唯一可能变化的就是 session_id，
// 因此直接比较完整 user_id 字符串即可判定 session_id 是否稳定。
func TestBuildOAuthMetadataUserID_SessionIDStableAcrossTurns(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{ID: 777, Type: AccountTypeOAuth, Extra: map[string]any{"account_uuid": "acc-uuid"}}
	fp := &Fingerprint{ClientID: "clientid777", UserAgent: "claude-cli/2.1.161 (external, cli)"}

	mustParse := func(body string) *ParsedRequest {
		parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(body)), PlatformAnthropic)
		require.NoError(t, err)
		return parsed
	}

	round1 := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"first question"}]}`)
	round2 := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"first question"},` +
		`{"role":"assistant","content":"answer 1"},` +
		`{"role":"user","content":"second question"}]}`)
	round3 := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"first question"},` +
		`{"role":"assistant","content":"answer 1"},` +
		`{"role":"user","content":"second question"},` +
		`{"role":"assistant","content":"answer 2"},` +
		`{"role":"user","content":"third question"}]}`)

	id1 := svc.buildOAuthMetadataUserID(nil, round1, account, fp)
	id2 := svc.buildOAuthMetadataUserID(nil, round2, account, fp)
	id3 := svc.buildOAuthMetadataUserID(nil, round3, account, fp)

	require.NotEmpty(t, id1)
	require.Equal(t, id1, id2, "session_id 应随对话增长保持不变")
	require.Equal(t, id2, id3, "session_id 应跨所有轮次保持不变")

	// 不同的首条 user 消息应派生出不同的 session_id（不同会话）。
	other := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"a completely different opener"}]}`)
	idOther := svc.buildOAuthMetadataUserID(nil, other, account, fp)
	require.NotEqual(t, id1, idOther, "不同首条消息应派生不同 session_id")
}

func TestBuildOAuthMetadataUserID_ReusesOnlyValidSessionAndReplacesMetadata(t *testing.T) {
	svc := &GatewayService{}
	account := &Account{ID: 778, Type: AccountTypeOAuth}
	sessionID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	existing := FormatMetadataUserID(strings.Repeat("b", 64), "old-account", sessionID, claude.CLICurrentVersion)
	body := []byte(`{
		"model":"claude-sonnet-4-6",
		"metadata":{"user_id":` + mustOAuthMetadataJSONString(t, existing) + `,"trace_id":"remove-me"},
		"messages":[{"role":"user","content":"hello"}]
	}`)
	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)

	userID := svc.buildOAuthMetadataUserID(nil, parsed, account, &Fingerprint{ClientID: "client"})
	metadata := ParseMetadataUserID(userID)
	require.NotNil(t, metadata)
	require.Equal(t, sessionID, metadata.SessionID)
	require.True(t, metadata.IsNewFormat)
	require.True(t, IsValidClaudeCodeMetadataUserID(userID))

	out, changed := ensureClaudeOAuthMetadataUserID(body, userID)
	require.True(t, changed)
	require.Equal(t, userID, gjson.GetBytes(out, "metadata.user_id").String())
	require.False(t, gjson.GetBytes(out, "metadata.trace_id").Exists())
}

func mustOAuthMetadataJSONString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
