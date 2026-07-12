package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration177UsesGenerationIsolationForImageGroupSuccessRates(t *testing.T) {
	content, err := FS.ReadFile("177_image_group_success_rates.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "create table if not exists image_group_success_rate_state")
	require.Contains(t, sql, "create table if not exists image_group_success_rate_stats")
	require.Contains(t, sql, "primary key (generation, group_id)")
	require.Contains(t, sql, "create table if not exists image_group_success_rate_events")
	require.Contains(t, sql, "event_key varchar(160) primary key")
	require.Contains(t, sql, "add column if not exists group_id")
	require.NotContains(t, sql, "insert into image_group_success_rate_stats (")
}
