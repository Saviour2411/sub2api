//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os/exec"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func migration252Files(t *testing.T) fstest.MapFS {
	t.Helper()
	raw, err := migrations.FS.ReadFile(blueGreen252Migration.Filename)
	require.NoError(t, err)
	return fstest.MapFS{blueGreen252Migration.Filename: &fstest.MapFile{Data: raw}}
}

func runMigration252(t *testing.T, db *sql.DB) error {
	t.Helper()
	return applyMigrationsFSWithPolicy(context.Background(), db, migration252Files(t),
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreen252Migration}})
}

func ultrafast252GuardSQL(t *testing.T, mode string) string {
	t.Helper()
	output, err := exec.Command("python3", "-B", "../../../deploy/blue-green/ultrafast252-guard.py", "--sql", mode).CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}

func release252Guard(t *testing.T, db *sql.DB) error {
	t.Helper()
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = conn.ExecContext(context.Background(), ultrafast252GuardSQL(t, "release"))
	if err != nil {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
	}
	return err
}

func assertMigration252Unchanged(t *testing.T, db *sql.DB) {
	t.Helper()
	var clean bool
	require.NoError(t, db.QueryRow(`SELECT
        NOT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)
        AND NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='channel_model_pricing'::regclass
            AND attname='ultrafast_multiplier' AND NOT attisdropped)`, blueGreen252Migration.Filename).Scan(&clean))
	require.True(t, clean)
}

func TestBlueGreen252UpgradeOldWritesGuardReleaseAndReplay(t *testing.T) {
	db, conn, _ := specialMigrationFixture(t)
	var before, after int64
	var oldTimeout, newTimeout string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW lock_timeout").Scan(&oldTimeout))
	require.NoError(t, db.QueryRow("SELECT pg_relation_filenode('channel_model_pricing')").Scan(&before))
	require.NoError(t, runMigration252(t, db))
	require.NoError(t, db.QueryRow("SELECT pg_relation_filenode('channel_model_pricing')").Scan(&after))
	require.Equal(t, before, after)
	_, err := db.Exec("UPDATE channel_model_pricing SET input_price=3 WHERE id=1; DELETE FROM channel_model_pricing WHERE id=2; INSERT INTO channel_model_pricing(id,input_price) VALUES(2,4)")
	require.NoError(t, err)
	_, err = db.Exec("UPDATE channel_model_pricing SET ultrafast_multiplier=2 WHERE id=1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "sub2api_bg252_price")
	require.NoError(t, runMigration252(t, db))
	var raw []byte
	require.NoError(t, db.QueryRow(ultrafast252GuardSQL(t, "check")).Scan(&raw))
	var report map[string]any
	require.NoError(t, json.Unmarshal(raw, &report))
	for _, key := range []string{"valid", "migrations", "schema"} {
		require.Equal(t, true, report[key], key)
	}
	require.NoError(t, release252Guard(t, db))
	require.NoError(t, release252Guard(t, db))
	_, err = db.Exec("UPDATE channel_model_pricing SET ultrafast_multiplier=2.5 WHERE id=1")
	require.NoError(t, err)
	for _, value := range []string{"0", "-1"} {
		_, err = db.Exec("UPDATE channel_model_pricing SET ultrafast_multiplier=" + value + " WHERE id=1")
		require.Error(t, err)
	}
	require.NoError(t, runMigration252(t, db))
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT
        (SELECT input_price=3 AND ultrafast_multiplier=2.5 FROM channel_model_pricing WHERE id=1)
        AND (SELECT input_price=4 AND ultrafast_multiplier IS NULL FROM channel_model_pricing WHERE id=2)
        AND (SELECT count(*)=1 FROM schema_migrations)`).Scan(&valid))
	require.True(t, valid)
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW lock_timeout").Scan(&newTimeout))
	require.Equal(t, oldTimeout, newTimeout)
}

func TestBlueGreen252CompleteOldSchema(t *testing.T) {
	db, _, _ := specialMigrationFixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name == blueGreen252Migration.Filename {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	_, err = db.Exec(`INSERT INTO channels(name) VALUES('旧版渠道');
        INSERT INTO channel_model_pricing(channel_id,models,input_price,fast_multiplier,flex_multiplier)
        SELECT id,'["gpt-5.5"]',0.01,2,0.5 FROM channels WHERE name='旧版渠道'`)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreen252Migration}}))
	// 对应旧版明确列名读取与删后重建，新增字段保持NULL。
	var models string
	require.NoError(t, db.QueryRow("SELECT models::text FROM channel_model_pricing").Scan(&models))
	require.Contains(t, models, "gpt-5.5")
	_, err = db.Exec(`DELETE FROM channel_model_pricing;
        INSERT INTO channel_model_pricing(channel_id,models,input_price,fast_multiplier,flex_multiplier)
        SELECT id,'["gpt-5.5"]',0.02,2,0.5 FROM channels WHERE name='旧版渠道'`)
	require.NoError(t, err)
	require.NoError(t, release252Guard(t, db))
	_, err = db.Exec("UPDATE channel_model_pricing SET ultrafast_multiplier=3")
	require.NoError(t, err)
	require.NoError(t, runMigration252(t, db))
}

func TestBlueGreen252LockConflictAndAtomicFailure(t *testing.T) {
	for _, table := range []string{"channel_model_pricing", "schema_migrations"} {
		t.Run(table, func(t *testing.T) {
			db, conn, _ := specialMigrationFixture(t)
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec(fmt.Sprintf("LOCK TABLE %s IN ROW EXCLUSIVE MODE", table))
			require.NoError(t, err)
			started := time.Now()
			err = runSpecialMigration(t, conn, blueGreen252Migration)
			require.ErrorContains(t, err, "立即获锁")
			require.Less(t, time.Since(started), time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			assertMigration252Unchanged(t, db)
		})
	}
	t.Run("账本失败整体回滚", func(t *testing.T) {
		db, _, _ := specialMigrationFixture(t)
		_, err := db.Exec("ALTER TABLE schema_migrations ADD CONSTRAINT reject_252 CHECK(filename<>'252_channel_pricing_ultrafast_multiplier.sql')")
		require.NoError(t, err)
		require.Error(t, runMigration252(t, db))
		assertMigration252Unchanged(t, db)
	})
}

func TestBlueGreen252RejectsUnexpectedBaseline(t *testing.T) {
	for name, statement := range map[string]string{
		"同名字段":   "ALTER TABLE channel_model_pricing ADD COLUMN ultrafast_multiplier numeric(12,6)",
		"同名约束":   "ALTER TABLE channel_model_pricing ADD CONSTRAINT sub2api_bg252_price CHECK(true)",
		"RLS":    "ALTER TABLE channel_model_pricing ENABLE ROW LEVEL SECURITY",
		"账本RLS":  "ALTER TABLE schema_migrations ENABLE ROW LEVEL SECURITY",
		"继承":     "CREATE TABLE inherited_pricing() INHERITS(channel_model_pricing)",
		"规则":     "CREATE RULE reject_update AS ON UPDATE TO channel_model_pricing DO INSTEAD NOTHING",
		"触发器":    "CREATE FUNCTION unexpected() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END'; CREATE TRIGGER unexpected BEFORE UPDATE ON channel_model_pricing FOR EACH ROW EXECUTE FUNCTION unexpected()",
		"DDL触发器": "CREATE FUNCTION unexpected_ddl() RETURNS event_trigger LANGUAGE plpgsql AS 'BEGIN END'; CREATE EVENT TRIGGER unexpected_ddl ON ddl_command_start EXECUTE FUNCTION unexpected_ddl()",
		"表过大":    "ALTER TABLE channel_model_pricing ADD COLUMN oversized text STORAGE PLAIN; INSERT INTO channel_model_pricing(id,oversized) SELECT n,repeat('x',7000) FROM generate_series(100,2600) n",
	} {
		t.Run(name, func(t *testing.T) {
			db, _, _ := specialMigrationFixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration252(t, db))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", blueGreen252Migration.Filename).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestBlueGreen252RejectsAppliedSchemaAndReleaseDrift(t *testing.T) {
	for name, statement := range map[string]string{
		"正值约束未执行": "ALTER TABLE channel_model_pricing DROP CONSTRAINT channel_model_pricing_ultrafast_multiplier_positive; ALTER TABLE channel_model_pricing ADD CONSTRAINT channel_model_pricing_ultrafast_multiplier_positive CHECK(ultrafast_multiplier IS NULL OR ultrafast_multiplier>0) NOT ENFORCED",
		"共存保护未执行": "ALTER TABLE channel_model_pricing DROP CONSTRAINT sub2api_bg252_price; ALTER TABLE channel_model_pricing ADD CONSTRAINT sub2api_bg252_price CHECK(ultrafast_multiplier IS NULL) NOT ENFORCED",
		"默认值":     "ALTER TABLE channel_model_pricing ALTER COLUMN ultrafast_multiplier SET DEFAULT 2",
		"类型":      "ALTER TABLE channel_model_pricing ALTER COLUMN ultrafast_multiplier TYPE numeric(12,5)",
		"正值约束":    "ALTER TABLE channel_model_pricing DROP CONSTRAINT channel_model_pricing_ultrafast_multiplier_positive",
		"保护漂移":    "ALTER TABLE channel_model_pricing DROP CONSTRAINT sub2api_bg252_price; ALTER TABLE channel_model_pricing ADD CONSTRAINT sub2api_bg252_price CHECK(true)",
		"注释":      "COMMENT ON COLUMN channel_model_pricing.ultrafast_multiplier IS 'changed'",
		"额外约束":    "ALTER TABLE channel_model_pricing ADD CONSTRAINT extra_limit CHECK(ultrafast_multiplier<100)",
		"账本摘要":    "UPDATE schema_migrations SET checksum='wrong' WHERE filename='252_channel_pricing_ultrafast_multiplier.sql'",
		"RLS":     "ALTER TABLE channel_model_pricing ENABLE ROW LEVEL SECURITY",
	} {
		t.Run(name, func(t *testing.T) {
			db, _, _ := specialMigrationFixture(t)
			require.NoError(t, runMigration252(t, db))
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration252(t, db))
			require.Error(t, release252Guard(t, db))
			var retained bool
			require.NoError(t, db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='sub2api_bg252_price')").Scan(&retained))
			require.True(t, retained)
		})
	}
}

func TestBlueGreen252ReleaseLockConflictRetainsGuard(t *testing.T) {
	for _, table := range []string{"channel_model_pricing", "schema_migrations", "迁移锁"} {
		t.Run(table, func(t *testing.T) {
			db, _, _ := specialMigrationFixture(t)
			require.NoError(t, runMigration252(t, db))
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			statement := "SELECT pg_advisory_xact_lock(694208311321144027)"
			if table != "迁移锁" {
				statement = fmt.Sprintf("LOCK TABLE %s IN ROW EXCLUSIVE MODE", table)
			}
			_, err = old.Exec(statement)
			require.NoError(t, err)
			started := time.Now()
			require.Error(t, release252Guard(t, db))
			require.Less(t, time.Since(started), time.Second)
			var present int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM pg_constraint WHERE conname='sub2api_bg252_price'").Scan(&present))
			require.Equal(t, 1, present)
		})
	}
}
