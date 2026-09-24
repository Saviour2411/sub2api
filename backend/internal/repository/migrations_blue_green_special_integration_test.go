//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func specialMigrationFixture(t *testing.T) (*sql.DB, *sql.Conn, []blueGreenMigration) {
	t.Helper()
	name := fmt.Sprintf("bg_special_%d", time.Now().UnixNano())
	_, err := integrationDB.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	dsn, err := url.Parse(integrationDSN)
	require.NoError(t, err)
	dsn.Path = "/" + name
	db, err := sql.Open("postgres", dsn.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_, err := integrationDB.Exec("DROP DATABASE " + name)
		require.NoError(t, err)
	})
	_, err = db.Exec(`CREATE TABLE schema_migrations (filename text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz DEFAULT now());
        CREATE TABLE channel_model_pricing (id integer PRIMARY KEY, input_price numeric, max_reasoning_effort_multiplier double precision);
        CREATE TABLE channel_account_stats_model_pricing (id integer PRIMARY KEY, input_price numeric);
        CREATE TABLE groups (id integer PRIMARY KEY, model_pricing jsonb);
        CREATE TABLE user_affiliate_ledger (id integer PRIMARY KEY, user_id bigint, amount numeric);
        INSERT INTO channel_model_pricing (id,input_price) VALUES (1,2);
        INSERT INTO channel_account_stats_model_pricing (id,input_price) VALUES (1,2);
        INSERT INTO groups VALUES (1,'[{"models":["custom"],"input_price":2,"max_reasoning_effort_multiplier":null}]'),(2,'null'),(3,NULL);
        INSERT INTO user_affiliate_ledger VALUES (1,1,10);`)
	require.NoError(t, err)
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return db, conn, []blueGreenMigration{
		{Filename: blueGreenReasoningFile, Checksum: blueGreenReasoningChecksum, Profile: blueGreenReasoningProfile},
		{Filename: blueGreenAffiliateFile, Checksum: blueGreenAffiliateChecksum, Profile: blueGreenAffiliateProfile},
	}
}

func runSpecialMigration(t *testing.T, conn *sql.Conn, entry blueGreenMigration) error {
	t.Helper()
	content, err := migrations.FS.ReadFile(entry.Filename)
	require.NoError(t, err)
	return applyBlueGreenExpansion(context.Background(), conn, entry, string(content))
}

func pricingGuardSQL(t *testing.T, mode string) string {
	t.Helper()
	output, err := exec.Command("python3", "-B", "../../../deploy/blue-green/pricing-guard.py", "--sql", mode).CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}

func TestBlueGreenSpecialOldNewReadWriteBillingAndRetirement(t *testing.T) {
	db, conn, entries := specialMigrationFixture(t)
	var timeoutBefore string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&timeoutBefore))
	for _, entry := range entries {
		require.NoError(t, runSpecialMigration(t, conn, entry))
	}
	var report string
	require.NoError(t, db.QueryRow(pricingGuardSQL(t, "check")).Scan(&report))
	require.JSONEq(t, `{"present":3,"valid":true,"migrations":true}`, report)
	// 精确列名复现旧版读写；旧版null与新版空映射均表示不额外加价。
	_, err := db.Exec(`INSERT INTO channel_model_pricing (id,input_price,max_reasoning_effort_multiplier) VALUES (2,3,NULL);
        UPDATE channel_model_pricing SET input_price=4,max_reasoning_effort_multiplier=NULL WHERE id=1;
        INSERT INTO channel_account_stats_model_pricing (id,input_price) VALUES (2,3);
        UPDATE groups SET model_pricing='[{"models":["custom"],"input_price":4,"max_reasoning_effort_multiplier":null}]' WHERE id=1;
        INSERT INTO user_affiliate_ledger (id,user_id,amount) VALUES (2,1,20),(3,1,30);`)
	require.NoError(t, err)
	var old *float64
	var generic, group string
	require.NoError(t, db.QueryRow("SELECT max_reasoning_effort_multiplier,reasoning_effort_multipliers::text FROM channel_model_pricing WHERE id=1").Scan(&old, &generic))
	require.Nil(t, old)
	require.JSONEq(t, "{}", generic)
	require.NoError(t, db.QueryRow("SELECT model_pricing::text FROM groups WHERE id=1").Scan(&group))
	var current []service.ChannelModelPricing
	var legacy []struct {
		Max *float64 `json:"max_reasoning_effort_multiplier"`
	}
	require.NoError(t, json.Unmarshal([]byte(group), &current))
	require.NoError(t, json.Unmarshal([]byte(group), &legacy))
	require.Empty(t, current[0].ReasoningEffortMultipliers)
	require.Nil(t, legacy[0].Max)
	for _, statement := range []string{
		`UPDATE channel_model_pricing SET max_reasoning_effort_multiplier=2 WHERE id=1`,
		`UPDATE channel_model_pricing SET reasoning_effort_multipliers='{"max":2}' WHERE id=1`,
		`UPDATE channel_account_stats_model_pricing SET reasoning_effort_multipliers='{"high":2}' WHERE id=1`,
		`UPDATE groups SET model_pricing='[{"max_reasoning_effort_multiplier":2}]' WHERE id=1`,
		`UPDATE groups SET model_pricing='[{"reasoning_effort_multipliers":{"max":2}}]' WHERE id=1`,
		`UPDATE groups SET model_pricing='[{"reasoning_effort_multipliers":null}]' WHERE id=1`,
	} {
		_, err := db.Exec(statement)
		require.ErrorContains(t, err, "sub2api_bg239", statement)
	}
	_, err = db.Exec(`INSERT INTO user_affiliate_ledger (id,user_id,amount,operation_id) VALUES (4,1,40,'operation')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_affiliate_ledger (id,user_id,amount,operation_id) VALUES (5,1,50,'operation')`)
	require.ErrorContains(t, err, "idx_user_affiliate_ledger_operation_id")
	fsys := fstest.MapFS{}
	for _, entry := range entries {
		content, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		fsys[entry.Filename] = &fstest.MapFile{Data: content}
	}
	policy := &blueGreenMigrationPolicy{Version: 1, Migrations: entries}
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, fsys, policy))
	_, err = db.Exec(pricingGuardSQL(t, "release"))
	require.NoError(t, err)
	// 解除后恢复完整新功能；重启不重新安装已经解除的保护，也不重放239。
	_, err = db.Exec(`UPDATE channel_model_pricing SET reasoning_effort_multipliers='{"high":2}' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, fsys, policy))
	_, err = db.Exec(pricingGuardSQL(t, "release"))
	require.NoError(t, err)
	require.NoError(t, db.QueryRow("SELECT reasoning_effort_multipliers::text FROM channel_model_pricing WHERE id=1").Scan(&generic))
	require.JSONEq(t, `{"high":2}`, generic)
	var timeoutAfter string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&timeoutAfter))
	require.Equal(t, timeoutBefore, timeoutAfter)
}

func TestBlueGreenSpecialPreconditionsRollback(t *testing.T) {
	for _, statement := range []string{
		`UPDATE channel_model_pricing SET max_reasoning_effort_multiplier=2`,
		`UPDATE groups SET model_pricing='[{"max_reasoning_effort_multiplier":2}]' WHERE id=1`,
		`UPDATE groups SET model_pricing='[{"reasoning_effort_multipliers":{"high":2}}]' WHERE id=1`,
		`ALTER TABLE channel_model_pricing ADD COLUMN reasoning_effort_multipliers text`,
		`ALTER TABLE channel_account_stats_model_pricing ADD CONSTRAINT sub2api_bg239_stats CHECK (true)`,
	} {
		t.Run(statement, func(t *testing.T) {
			db, conn, entries := specialMigrationFixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runSpecialMigration(t, conn, entries[0]))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count))
			require.Zero(t, count)
			require.NoError(t, db.QueryRow("SELECT count(*) FROM information_schema.columns WHERE table_name='channel_account_stats_model_pricing' AND column_name='reasoning_effort_multipliers'").Scan(&count))
			require.Zero(t, count)
			if strings.Contains(statement, `max_reasoning_effort_multiplier":2`) {
				var pricing string
				require.NoError(t, db.QueryRow("SELECT model_pricing::text FROM groups WHERE id=1").Scan(&pricing))
				require.Contains(t, pricing, "max_reasoning_effort_multiplier")
			}
		})
	}
}

func TestBlueGreenSpecialLockConflictAndSchemaDrift(t *testing.T) {
	for i, table := range []string{"channel_model_pricing", "user_affiliate_ledger"} {
		t.Run(table, func(t *testing.T) {
			db, conn, entries := specialMigrationFixture(t)
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec("LOCK TABLE " + table + " IN ROW EXCLUSIVE MODE")
			require.NoError(t, err)
			start := time.Now()
			err = runSpecialMigration(t, conn, entries[i])
			require.Error(t, err)
			require.Less(t, time.Since(start), 2*time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			require.NoError(t, old.Commit())
			require.NoError(t, runSpecialMigration(t, conn, entries[i]))
		})
	}
	t.Run("existing_index", func(t *testing.T) {
		db, conn, entries := specialMigrationFixture(t)
		_, err := db.Exec("CREATE INDEX idx_user_affiliate_ledger_operation_id ON user_affiliate_ledger (user_id)")
		require.NoError(t, err)
		require.Error(t, runSpecialMigration(t, conn, entries[1]))
		var count int
		require.NoError(t, db.QueryRow("SELECT count(*) FROM information_schema.columns WHERE table_name='user_affiliate_ledger' AND column_name='operation_id'").Scan(&count))
		require.Zero(t, count)
	})
}

func TestBlueGreenSpecialReleaseRejectsLockAndConstraintDrift(t *testing.T) {
	db, conn, entries := specialMigrationFixture(t)
	for _, entry := range entries {
		require.NoError(t, runSpecialMigration(t, conn, entry))
	}
	old, err := db.Begin()
	require.NoError(t, err)
	_, err = old.Exec("LOCK TABLE groups IN ROW EXCLUSIVE MODE")
	require.NoError(t, err)
	_, err = conn.ExecContext(context.Background(), pricingGuardSQL(t, "release"))
	require.Error(t, err)
	_, err = conn.ExecContext(context.Background(), "ROLLBACK")
	require.NoError(t, err)
	require.NoError(t, old.Rollback())
	var report string
	require.NoError(t, db.QueryRow(pricingGuardSQL(t, "check")).Scan(&report))
	require.JSONEq(t, `{"present":3,"valid":true,"migrations":true}`, report)
	_, err = db.Exec("ALTER TABLE groups DROP CONSTRAINT sub2api_bg239_groups; ALTER TABLE groups ADD CONSTRAINT sub2api_bg239_groups CHECK (true)")
	require.NoError(t, err)
	_, err = conn.ExecContext(context.Background(), pricingGuardSQL(t, "release"))
	require.ErrorContains(t, err, "漂移")
	_, err = conn.ExecContext(context.Background(), "ROLLBACK")
	require.NoError(t, err)
}

func TestBlueGreenSpecialRejectsLargeAndInheritedTables(t *testing.T) {
	for _, scenario := range []string{"large", "inherited"} {
		t.Run(scenario, func(t *testing.T) {
			db, conn, entries := specialMigrationFixture(t)
			statement := "CREATE TABLE ledger_child () INHERITS (user_affiliate_ledger)"
			if scenario == "large" {
				statement = `ALTER TABLE user_affiliate_ledger ADD COLUMN payload text;
                    UPDATE user_affiliate_ledger SET payload=(SELECT string_agg(md5(i::text),'') FROM generate_series(1,600000) AS i)`
			}
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.ErrorContains(t, runSpecialMigration(t, conn, entries[1]), "16MiB")
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestBlueGreenSpecialSecondMigrationFailureRetainsPricingGuard(t *testing.T) {
	db, conn, entries := specialMigrationFixture(t)
	require.NoError(t, runSpecialMigration(t, conn, entries[0]))
	_, err := db.Exec("CREATE INDEX idx_user_affiliate_ledger_operation_id ON user_affiliate_ledger (user_id)")
	require.NoError(t, err)
	require.Error(t, runSpecialMigration(t, conn, entries[1]))
	var report string
	require.NoError(t, db.QueryRow(pricingGuardSQL(t, "check")).Scan(&report))
	require.JSONEq(t, `{"present":3,"valid":true,"migrations":false}`, report)
	_, err = db.Exec("UPDATE channel_model_pricing SET input_price=5 WHERE id=1")
	require.NoError(t, err)
	_, err = db.Exec("UPDATE channel_model_pricing SET max_reasoning_effort_multiplier=2 WHERE id=1")
	require.ErrorContains(t, err, "sub2api_bg239_channel")
	_, err = conn.ExecContext(context.Background(), pricingGuardSQL(t, "release"))
	require.ErrorContains(t, err, "漂移")
	_, err = conn.ExecContext(context.Background(), "ROLLBACK")
	require.NoError(t, err)
}
