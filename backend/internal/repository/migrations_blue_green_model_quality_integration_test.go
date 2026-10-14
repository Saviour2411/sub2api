//go:build integration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func modelQualityMigrationFixture(t *testing.T) (*sql.DB, *sql.Conn) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	_, err := db.Exec(`CREATE TABLE settings(id BIGSERIAL PRIMARY KEY,key VARCHAR(100) NOT NULL UNIQUE,
        value TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
        INSERT INTO settings(key,value) VALUES('legacy_setting','before')`)
	require.NoError(t, err)
	return db, conn
}

func runModelQualityMigration(t *testing.T, db *sql.DB) error {
	t.Helper()
	entry := blueGreenModelQualityMigration
	raw, err := migrations.FS.ReadFile(entry.Filename)
	require.NoError(t, err)
	return applyMigrationsFSWithPolicy(context.Background(), db,
		fstest.MapFS{entry.Filename: &fstest.MapFile{Data: raw}},
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}})
}

func TestBlueGreenModelQualityCompleteUpgradeAndReplay(t *testing.T) {
	db, conn := modelQualityMigrationFixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name == blueGreenModelQualityMigration.Filename {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	_, err = db.Exec("INSERT INTO settings(key,value) VALUES('legacy_setting','before')")
	require.NoError(t, err)
	var before string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&before))
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreenModelQualityMigration}}))
	config, err := NewModelQualityRepository(db).Config(context.Background())
	require.NoError(t, err)
	require.False(t, config.Enabled)
	// 旧版按键增改其他设置，不删除新功能的开关和独立表。
	_, err = db.Exec(`INSERT INTO settings(key,value) VALUES('legacy_setting','after') ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value;
        INSERT INTO model_quality_rounds(id,group_id,group_name,parameters,endpoint) VALUES('round',1,'分组','{}','https://example.invalid/v1');
        INSERT INTO model_quality_runs(round_id,model,reasoning_effort) VALUES('round','model','low');
        INSERT INTO model_quality_tasks(run_id,benchmark,idempotency_key) VALUES(1,'candy','test-idempotency')`)
	require.NoError(t, err)
	require.NoError(t, runModelQualityMigration(t, db))
	var value, after string
	require.NoError(t, db.QueryRow("SELECT value FROM settings WHERE key='legacy_setting'").Scan(&value))
	require.Equal(t, "after", value)
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&after))
	require.Equal(t, before, after)
	_, err = db.Exec("DELETE FROM model_quality_rounds WHERE id='round'")
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM model_quality_tasks").Scan(&count))
	require.Zero(t, count)
}

func TestBlueGreenModelQualityRejectsConflictsAndRollsBack(t *testing.T) {
	for name, statement := range map[string]string{
		"同名表":    "CREATE TABLE model_quality_config(id int)",
		"同名序列":   "CREATE SEQUENCE model_quality_runs_id_seq",
		"同名类型":   "CREATE TYPE model_quality_config AS (value text)",
		"已有开关":   "INSERT INTO settings(key,value) VALUES('model_quality_enabled','true')",
		"行级权限":   "ALTER TABLE settings ENABLE ROW LEVEL SECURITY",
		"结构漂移":   "ALTER TABLE settings ADD COLUMN unexpected text",
		"账本写入失败": "ALTER TABLE schema_migrations ADD CONSTRAINT reject_entry CHECK(filename<>'246_model_quality.sql')",
		"用户触发器": `CREATE FUNCTION unexpected_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
            CREATE TRIGGER unexpected BEFORE INSERT ON settings FOR EACH ROW EXECUTE FUNCTION unexpected_trigger()`,
		"DDL触发器": `CREATE FUNCTION unexpected_ddl() RETURNS event_trigger LANGUAGE plpgsql AS $$ BEGIN RETURN; END $$;
            CREATE EVENT TRIGGER unexpected ON ddl_command_start EXECUTE FUNCTION unexpected_ddl()`,
	} {
		t.Run(name, func(t *testing.T) {
			db, conn := modelQualityMigrationFixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runSpecialMigration(t, conn, blueGreenModelQualityMigration))
			var clean bool
			require.NoError(t, db.QueryRow(`SELECT to_regclass('public.model_quality_tasks') IS NULL
                AND NOT EXISTS(SELECT 1 FROM schema_migrations WHERE filename='246_model_quality.sql')
                AND (SELECT value='before' FROM settings WHERE key='legacy_setting')`).Scan(&clean))
			require.True(t, clean)
		})
	}
}

func TestBlueGreenModelQualityLockConflictFailsImmediately(t *testing.T) {
	db, conn := modelQualityMigrationFixture(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec("UPDATE settings SET value='busy' WHERE key='legacy_setting'")
	require.NoError(t, err)
	start := time.Now()
	require.ErrorContains(t, runSpecialMigration(t, conn, blueGreenModelQualityMigration), "立即获锁")
	require.Less(t, time.Since(start), time.Second)
}

func TestBlueGreenModelQualityReplayRejectsSchemaDrift(t *testing.T) {
	for name, statement := range map[string]string{
		"字段": "ALTER TABLE model_quality_tasks ALTER COLUMN status SET DEFAULT 'changed'",
		"索引": "DROP INDEX model_quality_round_active",
		"外键": "ALTER TABLE model_quality_tasks DROP CONSTRAINT model_quality_tasks_run_id_fkey",
		"序列": "ALTER SEQUENCE model_quality_runs_id_seq INCREMENT BY 2",
		"权限": "ALTER TABLE model_quality_rounds ENABLE ROW LEVEL SECURITY",
		"账本": "UPDATE schema_migrations SET checksum='wrong' WHERE filename='246_model_quality.sql'",
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := modelQualityMigrationFixture(t)
			require.NoError(t, runModelQualityMigration(t, db))
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runModelQualityMigration(t, db))
		})
	}
}
