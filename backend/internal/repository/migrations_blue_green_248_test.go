//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen248FixedPolicy(t *testing.T) {
	content, err := migrations.FS.ReadFile(blueGreen248Migration.Filename)
	require.NoError(t, err)
	require.NoError(t, validateBlueGreenSQL(blueGreen248Migration, string(content)))
	require.Error(t, validateBlueGreenSQL(blueGreen248Migration, string(content)+"\nSELECT 1;"))
	badDigest := blueGreen248Migration
	badDigest.Checksum = strings.Repeat("0", 64)
	badColumn := blueGreen248Migration
	badColumn.Table = "groups"
	for _, entry := range []blueGreenMigration{badDigest, badColumn} {
		require.False(t, approvedBlueGreenSpecial(entry))
	}
	for _, entries := range [][]blueGreenMigration{
		{blueGreen248Migration}, {badDigest}, {badColumn},
		{blueGreen248Migration, blueGreen248Migration}, {blueGreen248Migration, blueGreenModelQualityMigration},
	} {
		raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: entries})
		require.NoError(t, err)
		_, err = parseBlueGreenMigrationPolicy(string(raw))
		if len(entries) == 1 && entries[0] == blueGreen248Migration {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}
