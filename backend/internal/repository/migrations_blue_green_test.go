package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func expansionEntry(name, table, content string) blueGreenMigration {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return blueGreenMigration{Filename: name, Checksum: hex.EncodeToString(sum[:]), Table: table, Column: "engine_meta", DataType: "jsonb"}
}

func TestBlueGreenMigrationPolicy(t *testing.T) {
	policy, err := parseBlueGreenMigrationPolicy("")
	require.NoError(t, err)
	require.Nil(t, policy)
	policy, err = parseBlueGreenMigrationPolicy(`{"version":1,"migrations":[]}`)
	require.NoError(t, err)
	require.NotNil(t, policy)
	sql := "ALTER TABLE logs ADD COLUMN IF NOT EXISTS engine_meta JSONB;"
	entry := expansionEntry("999_test.sql", "logs", sql)
	raw, err := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}})
	require.NoError(t, err)
	_, err = parseBlueGreenMigrationPolicy(string(raw))
	require.NoError(t, err)
	for _, invalid := range []string{
		`null`, `{}`, `{"version":1,"migrations":null}`, `{"version":2,"migrations":[]}`,
		`{"version":1,"migrations":[],"unknown":true}`, string(raw) + `{}`,
		strings.ReplaceAll(string(raw), "999_test.sql", "../999_test.sql"),
		strings.ReplaceAll(string(raw), "jsonb", "custom_domain"),
		strings.ReplaceAll(string(raw), `"logs"`, `"logs;DROP TABLE users"`),
	} {
		_, err := parseBlueGreenMigrationPolicy(invalid)
		require.Error(t, err, invalid)
	}
}

func TestBlueGreenMigrationPublicEntryRejectsInvalidPolicyBeforeQueries(t *testing.T) {
	t.Setenv("SUB2API_BLUE_GREEN_MIGRATIONS", "invalid")
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.ErrorContains(t, ApplyMigrations(context.Background(), db), "策略格式无效")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBlueGreenSQLRejectsUnsafeChanges(t *testing.T) {
	base := "ALTER TABLE logs ADD COLUMN IF NOT EXISTS engine_meta JSONB;"
	valid := "-- 审核来源\n" + base
	require.NoError(t, validateBlueGreenSQL(expansionEntry("999_test.sql", "logs", valid), valid))
	for _, unsafe := range []string{
		strings.Replace(base, "JSONB", "JSONB NOT NULL", 1),
		strings.Replace(base, "JSONB", "JSONB DEFAULT NULL", 1),
		strings.Replace(base, "JSONB", "JSONB CHECK (true)", 1),
		strings.Replace(base, "JSONB", "custom_domain", 1),
		base + " DELETE FROM logs;", "ALTER TABLE logs DROP COLUMN engine_meta;",
		strings.Replace(base, "logs", "public.logs", 1),
	} {
		require.Error(t, validateBlueGreenSQL(expansionEntry("999_test.sql", "logs", unsafe), unsafe))
	}
	entry := expansionEntry("999_test.sql", "logs", base)
	require.Error(t, validateBlueGreenSQL(entry, base+"\n-- 摘要变化"))
}

func TestBlueGreenPendingListFailsBeforeAnyDDL(t *testing.T) {
	content := "ALTER TABLE logs ADD COLUMN IF NOT EXISTS engine_meta JSONB;"
	entry := expansionEntry("999_test.sql", "logs", content)
	for _, scenario := range []string{"missing_approval", "checksum_mismatch", "unknown_pending", "approved", "already_applied"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			conn, err := db.Conn(context.Background())
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()
			rows := sqlmock.NewRows([]string{"filename", "checksum"})
			fsys := fstest.MapFS{entry.Filename: &fstest.MapFile{Data: []byte(content)}}
			policy := &blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}}
			switch scenario {
			case "missing_approval":
				policy.Migrations = nil
			case "checksum_mismatch":
				rows.AddRow(entry.Filename, "wrong")
			case "unknown_pending":
				fsys["998_unapproved.sql"] = &fstest.MapFile{Data: []byte("DELETE FROM logs;")}
			case "already_applied":
				rows.AddRow(entry.Filename, entry.Checksum)
			}
			mock.ExpectQuery("SELECT filename, checksum FROM schema_migrations").WillReturnRows(rows)
			err = validateBlueGreenPendingMigrations(context.Background(), conn, fsys, policy)
			if scenario == "approved" || scenario == "already_applied" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
