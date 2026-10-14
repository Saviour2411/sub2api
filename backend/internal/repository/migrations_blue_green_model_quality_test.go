//go:build unit

package repository

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreenModelQualityFixedPolicy(t *testing.T) {
	entry := blueGreenModelQualityMigration
	raw, err := migrations.FS.ReadFile(entry.Filename)
	require.NoError(t, err)
	require.NoError(t, validateBlueGreenSQL(entry, string(raw)))
	require.Error(t, validateBlueGreenSQL(entry, string(raw)+"\nSELECT 1;"))
	for _, entries := range [][]blueGreenMigration{
		{entry},
		{entry, {Filename: "999_unknown.sql", Checksum: entry.Checksum, Table: "settings", Column: "extra", DataType: "text"}},
		{{Filename: entry.Filename, Checksum: entry.Checksum, Profile: "unknown"}},
	} {
		encoded, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: entries})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(encoded))
		if len(entries) == 1 && entries[0] == entry {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}
