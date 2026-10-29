//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func runMigration248(t *testing.T, db *sql.DB) error {
	t.Helper()
	raw, err := migrations.FS.ReadFile(blueGreen248Migration.Filename)
	require.NoError(t, err)
	return applyMigrationsFSWithPolicy(context.Background(), db,
		fstest.MapFS{blueGreen248Migration.Filename: &fstest.MapFile{Data: raw}},
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreen248Migration}})
}

func assertMigration248Unchanged(t *testing.T, db *sql.DB) {
	t.Helper()
	var clean bool
	require.NoError(t, db.QueryRow(`SELECT
        NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='groups'::regclass AND attname='image_count_billing' AND NOT attisdropped)
        AND NOT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, blueGreen248Migration.Filename).Scan(&clean))
	require.True(t, clean)
}

func TestBlueGreen248DefaultOldWritesAndReplay(t *testing.T) {
	db, conn, _ := specialMigrationFixture(t)
	var before, after, timeoutBefore, timeoutAfter string
	require.NoError(t, db.QueryRow("SELECT jsonb_agg(to_jsonb(g) ORDER BY id)::text FROM groups g").Scan(&before))
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&timeoutBefore))
	require.NoError(t, runMigration248(t, db))
	require.NoError(t, db.QueryRow("SELECT jsonb_agg(to_jsonb(g)-'image_count_billing' ORDER BY id)::text FROM groups g").Scan(&after))
	require.JSONEq(t, before, after)
	var valid bool
	require.NoError(t, db.QueryRow("SELECT bool_and(NOT image_count_billing) FROM groups").Scan(&valid))
	require.True(t, valid)
	// 复现旧仓库明确列名的新增和更新；旧写入不能清除新版本保存的开关。
	_, err := db.Exec(`UPDATE groups SET image_count_billing=true WHERE id=1;
        INSERT INTO groups(id,model_pricing) VALUES(4,'[{"input_price":3}]');
        UPDATE groups SET model_pricing='[{"input_price":4}]' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, runMigration248(t, db))
	require.NoError(t, db.QueryRow(`SELECT (SELECT image_count_billing AND model_pricing->0->>'input_price'='4' FROM groups WHERE id=1)
        AND (SELECT NOT image_count_billing FROM groups WHERE id=4)
        AND (SELECT count(*)=1 FROM schema_migrations WHERE filename=$1)`, blueGreen248Migration.Filename).Scan(&valid))
	require.True(t, valid)
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&timeoutAfter))
	require.Equal(t, timeoutBefore, timeoutAfter)
}

func TestBlueGreen248CompleteOldSchemaUpgrade(t *testing.T) {
	db, _, _ := specialMigrationFixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name == blueGreen248Migration.Filename {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	var initialCount int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM groups").Scan(&initialCount))
	_, err = db.Exec("INSERT INTO groups(name,platform,rate_multiplier) VALUES('legacy-openai','openai',1.25),('legacy-composite','composite',1.5)")
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreen248Migration}}))
	_, err = db.Exec("UPDATE groups SET name='old-updated' WHERE name='legacy-openai'; INSERT INTO groups(name,platform) VALUES('old-created','openai')")
	require.NoError(t, err)
	require.NoError(t, runMigration248(t, db))
	var valid bool
	require.NoError(t, db.QueryRow("SELECT count(*)=$1 AND bool_and(NOT image_count_billing) AND bool_or(name='old-updated' AND rate_multiplier=1.25) FROM groups", initialCount+3).Scan(&valid))
	require.True(t, valid)
}

func TestBlueGreen248LockConflictAndAtomicFailure(t *testing.T) {
	for _, table := range []string{"groups", "schema_migrations"} {
		t.Run(table, func(t *testing.T) {
			db, conn, _ := specialMigrationFixture(t)
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec(fmt.Sprintf("LOCK TABLE %s IN ROW EXCLUSIVE MODE", table))
			require.NoError(t, err)
			started := time.Now()
			err = runSpecialMigration(t, conn, blueGreen248Migration)
			require.ErrorContains(t, err, "立即获锁")
			require.Less(t, time.Since(started), time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			assertMigration248Unchanged(t, db)
		})
	}
	t.Run("账本失败整体回滚", func(t *testing.T) {
		db, _, _ := specialMigrationFixture(t)
		_, err := db.Exec("ALTER TABLE schema_migrations ADD CONSTRAINT reject_248 CHECK(filename<>'248_group_image_count_billing.sql')")
		require.NoError(t, err)
		require.Error(t, runMigration248(t, db))
		assertMigration248Unchanged(t, db)
	})
}

func TestBlueGreen248RejectsUnexpectedBaseline(t *testing.T) {
	for name, statement := range map[string]string{
		"同名字段":   "ALTER TABLE groups ADD COLUMN image_count_billing boolean NOT NULL DEFAULT false",
		"RLS":    "ALTER TABLE groups ENABLE ROW LEVEL SECURITY",
		"继承":     "CREATE TABLE inherited_groups() INHERITS(groups)",
		"账本RLS":  "ALTER TABLE schema_migrations ENABLE ROW LEVEL SECURITY",
		"规则":     "CREATE RULE reject_update AS ON UPDATE TO groups DO INSTEAD NOTHING",
		"触发器":    "CREATE FUNCTION unexpected() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END'; CREATE TRIGGER unexpected BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION unexpected()",
		"DDL触发器": "CREATE FUNCTION unexpected_ddl() RETURNS event_trigger LANGUAGE plpgsql AS 'BEGIN END'; CREATE EVENT TRIGGER unexpected_ddl ON ddl_command_start EXECUTE FUNCTION unexpected_ddl()",
		"表过大":    "ALTER TABLE groups ADD COLUMN oversized text STORAGE PLAIN; INSERT INTO groups(id,oversized) SELECT n,repeat('x',7000) FROM generate_series(100,2600) n",
	} {
		t.Run(name, func(t *testing.T) {
			db, _, _ := specialMigrationFixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration248(t, db))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", blueGreen248Migration.Filename).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestBlueGreen248RejectsAppliedSchemaDrift(t *testing.T) {
	for name, statement := range map[string]string{
		"默认值":   "ALTER TABLE groups ALTER COLUMN image_count_billing SET DEFAULT true",
		"可空性":   "ALTER TABLE groups ALTER COLUMN image_count_billing DROP NOT NULL",
		"字段缺失":  "ALTER TABLE groups DROP COLUMN image_count_billing",
		"注释":    "COMMENT ON COLUMN groups.image_count_billing IS 'changed'",
		"额外约束":  "ALTER TABLE groups ADD CONSTRAINT forbidden_image CHECK(NOT image_count_billing)",
		"账本摘要":  "UPDATE schema_migrations SET checksum='wrong' WHERE filename='248_group_image_count_billing.sql'",
		"无账本字段": "DELETE FROM schema_migrations WHERE filename='248_group_image_count_billing.sql'",
	} {
		t.Run(name, func(t *testing.T) {
			db, _, _ := specialMigrationFixture(t)
			require.NoError(t, runMigration248(t, db))
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration248(t, db))
		})
	}
}
