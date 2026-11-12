//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen250FixedPolicy(t *testing.T) {
	check := func(entries []blueGreenMigration) error {
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: entries})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		return err
	}
	require.NoError(t, check(blueGreen250Migrations))
	for _, entry := range blueGreen250Migrations {
		content, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		require.NoError(t, validateBlueGreenSQL(entry, string(content)))
		require.Error(t, validateBlueGreenSQL(entry, string(content)+"\nSELECT 1;"))
		require.Error(t, check([]blueGreenMigration{entry}))
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
				changed.Table = "usage_logs"
			}
			require.False(t, approvedBlueGreenSpecial(changed))
			require.Error(t, check([]blueGreenMigration{changed, blueGreen250Migrations[1]}))
		}
	}
	entries := append([]blueGreenMigration{}, blueGreen250Migrations...)
	require.Error(t, check(append(entries, blueGreen249Migration)))
}
