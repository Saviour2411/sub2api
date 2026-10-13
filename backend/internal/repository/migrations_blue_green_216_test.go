//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen216FixedBundlePolicy(t *testing.T) {
	policy := blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen216Migrations}
	raw, err := json.Marshal(policy)
	require.NoError(t, err)
	_, err = parseBlueGreenMigrationPolicy(string(raw))
	require.NoError(t, err)
	for _, entry := range blueGreen216Migrations {
		content, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		require.NoError(t, validateBlueGreenSQL(entry, string(content)))
		require.Error(t, validateBlueGreenSQL(entry, string(content)+"\nDELETE FROM users;"))
		changed := entry
		changed.Checksum = strings.Repeat("0", 64)
		require.False(t, approvedBlueGreenSpecial(changed))
		changed = entry
		changed.Table = "groups"
		require.False(t, approvedBlueGreenSpecial(changed))
	}
	for _, entries := range [][]blueGreenMigration{
		blueGreen216Migrations[:1], blueGreen216Migrations[1:],
		append(append([]blueGreenMigration{}, blueGreen216Migrations...), blueGreen216Migrations[0]),
	} {
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: entries})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		require.Error(t, err)
	}
}
