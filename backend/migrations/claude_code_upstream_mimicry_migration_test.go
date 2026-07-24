package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration186幂等删除无效ClaudeCode分组字段(t *testing.T) {
	content, err := FS.ReadFile("186_drop_group_claude_code_upstream_mimicry.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(strings.ToLower(string(content))), " ")
	require.Contains(t, sql, "drop index if exists idx_groups_claude_code_upstream_mimicry")
	require.Contains(t, sql, "drop column if exists claude_code_upstream_mimicry")
	require.Less(t,
		strings.Index(sql, "drop index if exists idx_groups_claude_code_upstream_mimicry"),
		strings.Index(sql, "drop column if exists claude_code_upstream_mimicry"),
	)
}
