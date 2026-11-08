//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen249FixedPolicy(t *testing.T) {
	entry := blueGreen249Migration
	raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}})
	require.NoError(t, err)
	_, err = parseBlueGreenMigrationPolicy(string(raw))
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile(entry.Filename)
	require.NoError(t, err)
	require.NoError(t, validateBlueGreenSQL(entry, string(content)))
	require.Error(t, validateBlueGreenSQL(entry, string(content)+"\nSELECT 1;"))
	for _, field := range []string{"filename", "checksum", "profile", "table"} {
		changed := entry
		switch field {
		case "filename":
			changed.Filename = "999_unknown.sql"
		case "checksum":
			changed.Checksum = strings.Repeat("0", 64)
		case "profile":
			changed.Profile = "unknown"
		case "table":
			changed.Table = "users"
		}
		require.False(t, approvedBlueGreenSpecial(changed))
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{changed}})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		require.Error(t, err)
	}
	for _, extra := range []blueGreenMigration{entry, blueGreen248Migration,
		{Filename: "999_other.sql", Checksum: strings.Repeat("a", 64), Table: "users", Column: "extra", DataType: "text"}} {
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry, extra}})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		require.Error(t, err)
	}
}
