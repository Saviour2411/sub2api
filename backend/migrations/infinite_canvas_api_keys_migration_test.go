package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfiniteCanvasAPIKeysMigration(t *testing.T) {
	content, err := FS.ReadFile("222_infinite_canvas_api_keys.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS purpose VARCHAR(32) NOT NULL DEFAULT 'general'")
	require.Contains(t, sql, "CHECK (purpose IN ('general', 'infinite_canvas'))")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_infinite_canvas_user_group")
	require.Contains(t, sql, "ON api_keys(user_id, group_id)")
	require.Contains(t, sql, "WHERE deleted_at IS NULL AND purpose = 'infinite_canvas'")
}
