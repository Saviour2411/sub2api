package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration175ForcesRequestedBillingModelSource(t *testing.T) {
	entries, err := FS.ReadDir(".")
	require.NoError(t, err)

	previousIndex := -1
	currentIndex := -1
	for i, entry := range entries {
		switch entry.Name() {
		case "174_allow_cyber_blocked_usage_request_type.sql":
			previousIndex = i
		case "175_force_requested_billing_model_source.sql":
			currentIndex = i
		}
	}
	require.NotEqual(t, -1, previousIndex)
	require.NotEqual(t, -1, currentIndex)
	require.Less(t, previousIndex, currentIndex)

	content, err := FS.ReadFile("175_force_requested_billing_model_source.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "set billing_model_source = 'requested'")
	require.Contains(t, sql, "billing_model_source is distinct from 'requested'")
	require.Contains(t, sql, "alter column billing_model_source set default 'requested'")
	require.NotContains(t, sql, "drop column")
}
