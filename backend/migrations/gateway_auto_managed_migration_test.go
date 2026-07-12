package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration176MergesAutoManagedPlansBeforeUniqueIndex(t *testing.T) {
	content, err := FS.ReadFile("176_gateway_auto_managed_plan_uniqueness.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	updateResults := strings.Index(sql, "update scheduled_test_results")
	deletePlans := strings.Index(sql, "delete from scheduled_test_plans")
	createIndex := strings.Index(sql, "create unique index")
	require.Greater(t, updateResults, -1)
	require.Greater(t, deletePlans, updateResults)
	require.Greater(t, createIndex, deletePlans)
	require.Contains(t, sql, "where auto_managed = true")
	require.Contains(t, sql, "coalesce(summary.next_run_at, now())")
}
