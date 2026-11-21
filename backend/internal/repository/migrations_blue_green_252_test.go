//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen252FixedPolicy(t *testing.T) {
	entry := blueGreen252Migration
	raw, err := migrations.FS.ReadFile(entry.Filename)
	require.NoError(t, err)
	require.NoError(t, validateBlueGreenSQL(entry, string(raw)))
	require.Error(t, validateBlueGreenSQL(entry, string(raw)+"\nSELECT 1;"))
	bad := entry
	bad.Checksum = strings.Repeat("0", 64)
	require.False(t, approvedBlueGreenSpecial(bad))
	bad = entry
	bad.Table = "channel_model_pricing"
	require.False(t, approvedBlueGreenSpecial(bad))
	for i, entries := range [][]blueGreenMigration{{entry}, {entry, entry}, {entry, blueGreen248Migration}, {bad}} {
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: entries})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		if i == 0 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}
