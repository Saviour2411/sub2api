//go:build unit

package repository

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestBlueGreen251FixedPolicy(t *testing.T) {
	for _, entry := range blueGreen251Migrations {
		raw, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		require.NoError(t, validateBlueGreenSQL(entry, string(raw)))
		require.Error(t, validateBlueGreenSQL(entry, string(raw)+"\nSELECT 1;"))
		bad := entry
		bad.Table = "groups"
		require.False(t, approvedBlueGreenSpecial(bad))
		bad = entry
		bad.Checksum = strings.Repeat("0", 64)
		require.False(t, approvedBlueGreenSpecial(bad))
	}
	cases := [][]blueGreenMigration{
		blueGreen251Migrations,
		blueGreen251Migrations[:1], blueGreen251Migrations[1:],
		{blueGreen251Migrations[0], blueGreen251Migrations[1], blueGreen251Migrations[1]},
		{blueGreen251Migrations[0], blueGreen251Migrations[1], blueGreen248Migration},
		append(append([]blueGreenMigration{}, blueGreen251Migrations...), blueGreen248Migration),
	}
	for i, entries := range cases {
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
