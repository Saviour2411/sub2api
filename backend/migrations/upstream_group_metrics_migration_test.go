package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration179BackfillsActualCostAndMultiplierHistory(t *testing.T) {
	content, err := FS.ReadFile("179_upstream_group_metrics.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(strings.ToLower(string(content))), " ")
	require.Contains(t, sql, "update upstream_daily_stats as stats set cost_basis_version = 2 from upstream_sites as sites")
	require.Contains(t, sql, "sites.id = stats.site_id and sites.platform = 'newapi' and stats.cost_basis_version < 2")
	require.Contains(t, sql, "update upstream_sites set today_cost_usd = 0, total_cost_usd = 0")
	require.Contains(t, sql, "where platform = 'sub2api'")
	require.Contains(t, sql, "insert into upstream_group_multiplier_history")
	require.Contains(t, sql, "where not exists ( select 1 from upstream_group_multiplier_history as history")
	require.Contains(t, sql, "history.site_id = groups.site_id and history.remote_id = groups.remote_id")
	require.Contains(t, sql, "set status = 'pending', error_message = null, next_sync_at = now()")
	require.Contains(t, sql, "where platform = 'sub2api' and enabled = true")
}

func TestMigration180AddsUpstreamGroupDisplayState(t *testing.T) {
	content, err := FS.ReadFile("180_upstream_group_display.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(strings.ToLower(string(content))), " ")
	require.Contains(t, sql, "add column if not exists displayed boolean not null default false")
	require.Contains(t, sql, "add column if not exists available boolean not null default true")
	require.Contains(t, sql, "idx_upstream_groups_site_displayed")
	require.Contains(t, sql, "idx_upstream_groups_site_available_name")
}
