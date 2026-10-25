//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen260FixedBundlePolicy(t *testing.T) {
	raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen260Migrations})
	require.NoError(t, err)
	_, err = parseBlueGreenMigrationPolicy(string(raw))
	require.NoError(t, err)
	for _, entry := range blueGreen260Migrations {
		content, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		require.NoError(t, validateBlueGreenSQL(entry, string(content)))
		require.Error(t, validateBlueGreenSQL(entry, string(content)+"\nSELECT 1;"))
		changed := entry
		changed.Checksum = strings.Repeat("0", 64)
		require.False(t, approvedBlueGreenSpecial(changed))
		changed = entry
		changed.Table = "groups"
		require.False(t, approvedBlueGreenSpecial(changed))
	}
	for _, entries := range [][]blueGreenMigration{
		blueGreen260Migrations[:1], blueGreen260Migrations[1:],
		append(append([]blueGreenMigration{}, blueGreen260Migrations...), blueGreen260Migrations[0]),
		append(append([]blueGreenMigration{}, blueGreen260Migrations...), blueGreenModelQualityMigration),
	} {
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: entries})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		require.Error(t, err)
	}
}
