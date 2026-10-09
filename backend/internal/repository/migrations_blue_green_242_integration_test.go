//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func migration242Fixture(t *testing.T) (*sql.DB, *sql.Conn, blueGreenMigration) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	// 直接建立生产列定义，避免ALTER TYPE重写旧CHECK的表达式。
	_, err := db.Exec(`CREATE TABLE user_platform_quotas (id integer PRIMARY KEY, platform varchar(32) NOT NULL, daily_limit_usd numeric);
        CREATE TABLE composite_model_routes (id integer PRIMARY KEY, target_platform varchar(50) NOT NULL, upstream_model text);
        INSERT INTO user_platform_quotas VALUES (1,'openai',10),(2,'typesafe',5);
        INSERT INTO composite_model_routes VALUES (1,'openai','existing'),(2,'typesafe','legacy-model');`)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile(blueGreenPlatform241File)
	require.NoError(t, err)
	_, err = db.Exec(string(content))
	require.NoError(t, err)
	for _, ledger := range []struct{ filename, checksum string }{
		{"238_opencode_go_platform.sql", blueGreenPlatform238Checksum},
		{blueGreenPlatform241File, blueGreenPlatform241Checksum},
	} {
		_, err = db.Exec("INSERT INTO schema_migrations (filename,checksum) VALUES ($1,$2)", ledger.filename, ledger.checksum)
		require.NoError(t, err)
	}
	return db, conn, blueGreenMigration{Filename: blueGreenPlatform242File, Checksum: blueGreenPlatform242Checksum, Profile: blueGreenPlatform242Profile}
}

func platform242SQL(t *testing.T, mode string) string {
	t.Helper()
	out, err := exec.Command("python3", "-B", "../../../deploy/blue-green/platform242-guard.py", "--sql", mode).CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func platform242Report(t *testing.T, db *sql.DB) map[string]any {
	t.Helper()
	var raw string
	require.NoError(t, db.QueryRow(platform242SQL(t, "check")).Scan(&raw))
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &report))
	return report
}

func TestBlueGreen242LegacyReadWriteAndRetirement(t *testing.T) {
	db, conn, entry := migration242Fixture(t)
	var before, after string
	query := "SELECT current_setting('lock_timeout')||','||current_setting('statement_timeout')||','||current_setting('transaction_timeout')"
	require.NoError(t, conn.QueryRowContext(context.Background(), query).Scan(&before))
	require.NoError(t, runSpecialMigration(t, conn, entry))
	require.NoError(t, conn.QueryRowContext(context.Background(), query).Scan(&after))
	require.Equal(t, before, after)
	require.Equal(t, map[string]any{"present": float64(2), "valid": true, "migrations": true, "schema": true}, platform242Report(t, db))
	// 旧版十一种平台仍可明确列名读写，包含typesafe；新平台与未知平台均不能提前落库。
	for _, platform := range []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go", "typesafe"} {
		_, err := db.Exec("UPDATE user_platform_quotas SET platform=$1,daily_limit_usd=20 WHERE id=1", platform)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE composite_model_routes SET target_platform=$1,upstream_model='compatible' WHERE id=1", platform)
		require.NoError(t, err)
	}
	for _, platform := range []string{"command_code", "cline", "unknown"} {
		for _, target := range blueGreen242Platforms {
			_, err := db.Exec(fmt.Sprintf("UPDATE %s SET %s=$1 WHERE id=1", target.table, target.column), platform)
			require.ErrorContains(t, err, target.guard)
		}
	}
	content, err := migrations.FS.ReadFile(entry.Filename)
	require.NoError(t, err)
	policy := &blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}}
	require.NoError(t, validateBlueGreenPendingMigrations(context.Background(), conn, fstest.MapFS{entry.Filename: &fstest.MapFile{Data: content}}, policy))
	for range 2 {
		_, err = conn.ExecContext(context.Background(), platform242SQL(t, "release"))
		require.NoError(t, err, "已解除后的重试必须幂等")
	}
	report := platform242Report(t, db)
	require.Equal(t, float64(0), report["present"])
	require.Equal(t, true, report["schema"])
	require.Equal(t, true, report["migrations"])
	_, err = db.Exec("UPDATE user_platform_quotas SET platform='command_code' WHERE id=1; UPDATE composite_model_routes SET target_platform='cline' WHERE id=1")
	require.NoError(t, err)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1 AND checksum=$2", entry.Filename, entry.Checksum).Scan(&count))
	require.Equal(t, 1, count)
}

func TestBlueGreen242LedgerFailureRollsBackDDL(t *testing.T) {
	db, conn, entry := migration242Fixture(t)
	// 注入最后记账步骤的冲突，证明前面的ADD/DROP不会部分提交。
	_, err := db.Exec("INSERT INTO schema_migrations (filename,checksum) VALUES ($1,$2)", entry.Filename, entry.Checksum)
	require.NoError(t, err)
	require.Error(t, runSpecialMigration(t, conn, entry))
	for _, target := range blueGreen242Platforms {
		var count int
		require.NoError(t, db.QueryRow("SELECT count(*) FROM pg_constraint WHERE conrelid=to_regclass('public.' || $1) AND conname=$2", target.table, target.guard).Scan(&count))
		require.Zero(t, count)
		_, err = db.Exec(fmt.Sprintf("UPDATE %s SET %s='command_code' WHERE id=1", target.table, target.column))
		require.ErrorContains(t, err, target.constraint)
	}
}

func TestBlueGreen242LockConflictRollsBackAllTables(t *testing.T) {
	for _, table := range []string{"user_platform_quotas", "composite_model_routes"} {
		t.Run(table, func(t *testing.T) {
			db, conn, entry := migration242Fixture(t)
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec("LOCK TABLE " + table + " IN ROW EXCLUSIVE MODE")
			require.NoError(t, err)
			start := time.Now()
			require.ErrorContains(t, runSpecialMigration(t, conn, entry), "无法立即获锁")
			require.Less(t, time.Since(start), 2*time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err = db.ExecContext(ctx, "UPDATE user_platform_quotas SET daily_limit_usd=11 WHERE id=1")
			require.NoError(t, err, "失败必须释放第一张表的锁")
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", entry.Filename).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, old.Rollback())
			require.NoError(t, runSpecialMigration(t, conn, entry))
		})
	}
}

func TestBlueGreen242RejectsDriftWithoutChangingOriginalConstraints(t *testing.T) {
	scenarios := map[string]string{
		"ledger":      "UPDATE schema_migrations SET checksum='drift' WHERE filename='241_add_typesafe_platform.sql'",
		"missing":     "ALTER TABLE composite_model_routes DROP CONSTRAINT composite_model_routes_target_platform_check",
		"constraint":  "ALTER TABLE composite_model_routes DROP CONSTRAINT composite_model_routes_target_platform_check; ALTER TABLE composite_model_routes ADD CONSTRAINT composite_model_routes_target_platform_check CHECK (true)",
		"guard":       "ALTER TABLE user_platform_quotas ADD CONSTRAINT sub2api_bg242_quota CHECK (true)",
		"old_guard":   "ALTER TABLE user_platform_quotas ADD CONSTRAINT sub2api_bg241_quota CHECK (true)",
		"nullable":    "ALTER TABLE user_platform_quotas ALTER COLUMN platform DROP NOT NULL",
		"length":      "ALTER TABLE composite_model_routes ALTER COLUMN target_platform TYPE varchar(60)",
		"inheritance": "CREATE TABLE quota_child () INHERITS (user_platform_quotas)",
		"rls":         "ALTER TABLE user_platform_quotas ENABLE ROW LEVEL SECURITY",
		"trigger":     "CREATE FUNCTION bg242_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$; CREATE TRIGGER bg242_test BEFORE UPDATE ON user_platform_quotas FOR EACH ROW EXECUTE FUNCTION bg242_trigger()",
		"large":       "ALTER TABLE user_platform_quotas ADD COLUMN payload text; UPDATE user_platform_quotas SET payload=(SELECT string_agg(md5(i::text),'') FROM generate_series(1,600000) AS i) WHERE id=1",
	}
	for name, statement := range scenarios {
		t.Run(name, func(t *testing.T) {
			db, conn, entry := migration242Fixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runSpecialMigration(t, conn, entry))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", entry.Filename).Scan(&count))
			require.Zero(t, count)
			_, err = db.Exec("UPDATE user_platform_quotas SET platform='command_code' WHERE id=1")
			require.ErrorContains(t, err, "user_platform_quotas_platform_check")
		})
	}
}

func TestBlueGreen242ReleaseRejectsLocksAndDrift(t *testing.T) {
	for _, scenario := range []string{"lock", "advisory", "guard", "missing_guard", "ledger", "original", "schema", "inheritance", "rls", "trigger", "large"} {
		t.Run(scenario, func(t *testing.T) {
			db, conn, entry := migration242Fixture(t)
			require.NoError(t, runSpecialMigration(t, conn, entry))
			var old *sql.Tx
			var err error
			if scenario == "lock" || scenario == "advisory" {
				old, err = db.Begin()
				require.NoError(t, err)
				defer func() { _ = old.Rollback() }()
				statement := "LOCK TABLE composite_model_routes IN ROW EXCLUSIVE MODE"
				if scenario == "advisory" {
					statement = "SELECT pg_advisory_xact_lock(694208311321144027)"
				}
				_, err = old.Exec(statement)
			} else {
				statement := map[string]string{
					"guard":         "ALTER TABLE user_platform_quotas DROP CONSTRAINT sub2api_bg242_quota; ALTER TABLE user_platform_quotas ADD CONSTRAINT sub2api_bg242_quota CHECK (true)",
					"missing_guard": "ALTER TABLE user_platform_quotas DROP CONSTRAINT sub2api_bg242_quota",
					"ledger":        "DELETE FROM schema_migrations WHERE filename='242_drop_platform_check_constraints.sql'",
					"original":      "ALTER TABLE user_platform_quotas ADD CONSTRAINT user_platform_quotas_platform_check CHECK (true)",
					"schema":        "ALTER TABLE user_platform_quotas ALTER COLUMN platform DROP NOT NULL",
					"inheritance":   "CREATE TABLE quota_child () INHERITS (user_platform_quotas)",
					"rls":           "ALTER TABLE user_platform_quotas ENABLE ROW LEVEL SECURITY",
					"trigger":       "CREATE FUNCTION bg242_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$; CREATE TRIGGER bg242_test BEFORE UPDATE ON user_platform_quotas FOR EACH ROW EXECUTE FUNCTION bg242_trigger()",
					"large":         "ALTER TABLE user_platform_quotas ADD COLUMN payload text; UPDATE user_platform_quotas SET payload=(SELECT string_agg(md5(i::text),'') FROM generate_series(1,600000) AS i) WHERE id=1",
				}[scenario]
				_, err = db.Exec(statement)
			}
			require.NoError(t, err)
			start := time.Now()
			_, err = conn.ExecContext(context.Background(), platform242SQL(t, "release"))
			require.Error(t, err)
			require.Less(t, time.Since(start), 2*time.Second)
			_, err = conn.ExecContext(context.Background(), "ROLLBACK")
			require.NoError(t, err)
			if old != nil {
				require.NoError(t, old.Rollback())
			}
			_, err = db.Exec("UPDATE composite_model_routes SET target_platform='cline' WHERE id=1")
			require.ErrorContains(t, err, "sub2api_bg242_route")
		})
	}
}
