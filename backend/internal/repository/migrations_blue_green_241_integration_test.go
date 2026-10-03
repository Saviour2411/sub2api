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

func migration241Fixture(t *testing.T) (*sql.DB, *sql.Conn, []blueGreenMigration) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	_, err := db.Exec(`CREATE TABLE payment_orders (id integer PRIMARY KEY, amount numeric(20,2), bonus_amount numeric(20,2) NOT NULL DEFAULT 0);
        CREATE TABLE user_platform_quotas (id integer PRIMARY KEY, platform varchar(32), daily_limit_usd numeric);
        CREATE TABLE composite_model_routes (id integer PRIMARY KEY, target_platform varchar(32), upstream_model text);
        CREATE TABLE channel_monitors (provider varchar(32));
        CREATE TABLE channel_monitor_request_templates (provider varchar(32));
        INSERT INTO payment_orders VALUES (1,12,2);
        INSERT INTO user_platform_quotas VALUES (1,'openai',10);
        INSERT INTO composite_model_routes VALUES (1,'openai','existing');`)
	require.NoError(t, err)
	for _, item := range []struct{ name, checksum string }{
		{"149_payment_balance_bonus_rules.sql", blueGreenBonus149Checksum},
		{"238_opencode_go_platform.sql", blueGreenPlatform238Checksum},
	} {
		_, err = db.Exec("INSERT INTO schema_migrations (filename,checksum) VALUES ($1,$2)", item.name, item.checksum)
		require.NoError(t, err)
	}
	// 用真实238迁移重建旧约束，不能重新解析pg_get_expr输出替代原始SQL。
	oldSQL, err := migrations.FS.ReadFile("238_opencode_go_platform.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(oldSQL))
	require.NoError(t, err)
	return db, conn, []blueGreenMigration{
		{Filename: blueGreenBonus241File, Checksum: blueGreenBonus241Checksum, Profile: blueGreenBonus241Profile},
		{Filename: blueGreenPlatform241File, Checksum: blueGreenPlatform241Checksum, Profile: blueGreenPlatform241Profile},
	}
}

func platformGuardSQL(t *testing.T, mode string) string {
	t.Helper()
	output, err := exec.Command("python3", "-B", "../../../deploy/blue-green/platform-guard.py", "--sql", mode).CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}

func TestBlueGreen241ConstraintRepresentations(t *testing.T) {
	db, _, _ := migration241Fixture(t)
	for _, expanded := range []bool{false, true} {
		if expanded {
			content, err := migrations.FS.ReadFile(blueGreenPlatform241File)
			require.NoError(t, err)
			_, err = db.Exec(string(content))
			require.NoError(t, err)
		}
		for _, target := range blueGreen241Platforms {
			var expression string
			require.NoError(t, db.QueryRow("SELECT pg_get_expr(conbin,conrelid) FROM pg_constraint WHERE conrelid=to_regclass('public.' || $1) AND conname=$2", target.table, target.constraint).Scan(&expression))
			require.Equal(t, blueGreen241PlatformDefinition(target.column, expanded), expression, "expanded=%v", expanded)
		}
	}
}

func platformGuardReport(t *testing.T, db *sql.DB) map[string]any {
	t.Helper()
	var raw string
	require.NoError(t, db.QueryRow(platformGuardSQL(t, "check")).Scan(&raw))
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &report))
	return report
}

func TestBlueGreen241LegacyReadWriteAndRetirement(t *testing.T) {
	db, conn, entries := migration241Fixture(t)
	var before, after string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SELECT current_setting('lock_timeout')||','||current_setting('statement_timeout')||','||current_setting('transaction_timeout')").Scan(&before))
	fsys := fstest.MapFS{}
	for _, entry := range entries {
		require.NoError(t, runSpecialMigration(t, conn, entry))
		content, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		fsys[entry.Filename] = &fstest.MapFile{Data: content}
	}
	require.NoError(t, conn.QueryRowContext(context.Background(), "SELECT current_setting('lock_timeout')||','||current_setting('statement_timeout')||','||current_setting('transaction_timeout')").Scan(&after))
	require.Equal(t, before, after, "事务保护参数不能泄漏到业务连接")
	require.Equal(t, map[string]any{"present": float64(2), "valid": true, "migrations": true, "schema": true}, platformGuardReport(t, db))
	// 按旧代码明确列名读写；赠金既有值及省略字段时的默认值不变。
	_, err := db.Exec(`INSERT INTO payment_orders (id,amount) VALUES (2,20);
        INSERT INTO payment_orders (id,amount,bonus_amount) VALUES (3,33,3);
        UPDATE payment_orders SET amount=15,bonus_amount=5 WHERE id=1;
        UPDATE user_platform_quotas SET daily_limit_usd=20 WHERE id=1;
        INSERT INTO user_platform_quotas (id,platform,daily_limit_usd) VALUES (2,'anthropic',10);
        UPDATE composite_model_routes SET upstream_model='new-model' WHERE id=1;`)
	require.NoError(t, err)
	var bonus float64
	require.NoError(t, db.QueryRow("SELECT bonus_amount FROM payment_orders WHERE id=2").Scan(&bonus))
	require.Zero(t, bonus)
	require.NoError(t, db.QueryRow("SELECT bonus_amount FROM payment_orders WHERE id=1").Scan(&bonus))
	require.Equal(t, float64(5), bonus)
	for _, target := range blueGreen241Platforms {
		_, err = db.Exec(fmt.Sprintf("UPDATE %s SET %s='typesafe' WHERE id=1", target.table, target.column))
		require.ErrorContains(t, err, target.guard)
	}
	policy := &blueGreenMigrationPolicy{Version: 1, Migrations: entries}
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, fsys, policy))
	_, err = db.Exec(platformGuardSQL(t, "release"))
	require.NoError(t, err)
	for _, target := range blueGreen241Platforms {
		_, err = db.Exec(fmt.Sprintf("UPDATE %s SET %s='typesafe' WHERE id=1", target.table, target.column))
		require.NoError(t, err)
		_, err = db.Exec(fmt.Sprintf("UPDATE %s SET %s='unapproved' WHERE id=1", target.table, target.column))
		require.ErrorContains(t, err, target.constraint)
	}
	_, err = db.Exec(platformGuardSQL(t, "release"))
	require.NoError(t, err, "解除保护支持中断后幂等恢复")
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, fsys, policy))
}

func TestBlueGreen241LockConflictLeavesOldWritesAvailable(t *testing.T) {
	for _, index := range []int{0, 1} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			db, conn, entries := migration241Fixture(t)
			table := "payment_orders"
			if index == 1 {
				table = "composite_model_routes"
			}
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec("LOCK TABLE " + table + " IN ROW EXCLUSIVE MODE")
			require.NoError(t, err)
			start := time.Now()
			err = runSpecialMigration(t, conn, entries[index])
			require.ErrorContains(t, err, "立即获锁")
			require.Less(t, time.Since(start), 2*time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err = db.ExecContext(ctx, "UPDATE user_platform_quotas SET daily_limit_usd=11 WHERE id=1")
			require.NoError(t, err, "第二张表获锁失败必须释放第一张表锁")
			var count int
			require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE filename=$1", entries[index].Filename).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, old.Rollback())
			require.NoError(t, runSpecialMigration(t, conn, entries[index]))
		})
	}
}

func TestBlueGreen241RejectsBonusDrift(t *testing.T) {
	for _, definition := range []string{"", "text NOT NULL DEFAULT '0'", "numeric(20,2) DEFAULT 0", "numeric(20,2) NOT NULL DEFAULT 1", "numeric(21,2) NOT NULL DEFAULT 0"} {
		t.Run(definition, func(t *testing.T) {
			db, conn, entries := migration241Fixture(t)
			_, err := db.Exec("ALTER TABLE payment_orders DROP COLUMN bonus_amount")
			require.NoError(t, err)
			if definition != "" {
				_, err = db.Exec("ALTER TABLE payment_orders ADD COLUMN bonus_amount " + definition)
				require.NoError(t, err)
			}
			require.ErrorContains(t, runSpecialMigration(t, conn, entries[0]), "赠金列")
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", entries[0].Filename).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestBlueGreen241RejectsPlatformDriftAndUnsafeTables(t *testing.T) {
	scenarios := map[string]string{
		"ledger":      "UPDATE schema_migrations SET checksum='drift' WHERE filename='238_opencode_go_platform.sql'",
		"missing":     "ALTER TABLE composite_model_routes DROP CONSTRAINT composite_model_routes_target_platform_check",
		"constraint":  "ALTER TABLE composite_model_routes DROP CONSTRAINT composite_model_routes_target_platform_check; ALTER TABLE composite_model_routes ADD CONSTRAINT composite_model_routes_target_platform_check CHECK (true)",
		"guard":       "ALTER TABLE user_platform_quotas ADD CONSTRAINT sub2api_bg241_quota CHECK (true)",
		"inheritance": "CREATE TABLE quota_child () INHERITS (user_platform_quotas)",
		"rls":         "ALTER TABLE user_platform_quotas ENABLE ROW LEVEL SECURITY",
		"trigger":     "CREATE FUNCTION bg241_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$; CREATE TRIGGER bg241_test BEFORE UPDATE ON user_platform_quotas FOR EACH ROW EXECUTE FUNCTION bg241_trigger()",
		"large":       "ALTER TABLE user_platform_quotas ADD COLUMN payload text; UPDATE user_platform_quotas SET payload=(SELECT string_agg(md5(i::text),'') FROM generate_series(1,600000) AS i)",
	}
	for name, statement := range scenarios {
		t.Run(name, func(t *testing.T) {
			db, conn, entries := migration241Fixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runSpecialMigration(t, conn, entries[1]))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", entries[1].Filename).Scan(&count))
			require.Zero(t, count)
			_, err = db.Exec("UPDATE user_platform_quotas SET platform='typesafe' WHERE id=1")
			require.Error(t, err, "拒绝后不能留下扩展过的约束")
		})
	}
}

func TestBlueGreen241ReleaseRejectsLocksAndDrift(t *testing.T) {
	for _, scenario := range []string{"lock", "guard", "schema", "ledger", "missing_guard", "inheritance"} {
		t.Run(scenario, func(t *testing.T) {
			db, conn, entries := migration241Fixture(t)
			for _, entry := range entries {
				require.NoError(t, runSpecialMigration(t, conn, entry))
			}
			var old *sql.Tx
			var err error
			if scenario == "lock" {
				old, err = db.Begin()
				require.NoError(t, err)
				defer func() { _ = old.Rollback() }()
				_, err = old.Exec("LOCK TABLE composite_model_routes IN ROW EXCLUSIVE MODE")
			} else {
				statement := map[string]string{
					"guard":         "ALTER TABLE user_platform_quotas DROP CONSTRAINT sub2api_bg241_quota; ALTER TABLE user_platform_quotas ADD CONSTRAINT sub2api_bg241_quota CHECK (true)",
					"schema":        "ALTER TABLE payment_orders ALTER COLUMN bonus_amount SET DEFAULT 7",
					"ledger":        "DELETE FROM schema_migrations WHERE filename='241_add_payment_order_bonus_amount.sql'",
					"missing_guard": "ALTER TABLE user_platform_quotas DROP CONSTRAINT sub2api_bg241_quota",
					"inheritance":   "CREATE TABLE route_child () INHERITS (composite_model_routes)",
				}[scenario]
				_, err = db.Exec(statement)
			}
			require.NoError(t, err)
			start := time.Now()
			_, err = conn.ExecContext(context.Background(), platformGuardSQL(t, "release"))
			require.Error(t, err)
			require.Less(t, time.Since(start), 2*time.Second)
			_, err = conn.ExecContext(context.Background(), "ROLLBACK")
			require.NoError(t, err)
			if old != nil {
				require.NoError(t, old.Rollback())
			}
			_, err = db.Exec("UPDATE composite_model_routes SET target_platform='typesafe' WHERE id=1")
			require.ErrorContains(t, err, "sub2api_bg241_route")
		})
	}
}
