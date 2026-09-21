//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func expansionFixture(t *testing.T, table string) (blueGreenMigration, string, *sql.Conn) {
	t.Helper()
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, "CREATE TABLE "+table+" (id integer PRIMARY KEY, old_value text)")
	require.NoError(t, err)
	entry, content := expansionEntry("9999_"+table+".sql", table, ""), "ALTER TABLE "+table+" ADD COLUMN IF NOT EXISTS engine_meta JSONB;"
	entry = expansionEntry(entry.Filename, table, content)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", entry.Filename)
	})
	conn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return entry, content, conn
}

func TestBlueGreenExpansionLegacyReadWriteAndIdempotency(t *testing.T) {
	entry, content, conn := expansionFixture(t, "bg_expand_compat")
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, "INSERT INTO bg_expand_compat (id,old_value) VALUES (1,'before')")
	require.NoError(t, err)
	var lockBefore, statementBefore string
	require.NoError(t, conn.QueryRowContext(ctx, "SHOW lock_timeout").Scan(&lockBefore))
	require.NoError(t, conn.QueryRowContext(ctx, "SHOW statement_timeout").Scan(&statementBefore))
	require.NoError(t, applyBlueGreenExpansion(ctx, conn, entry, content))
	var empty bool
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT engine_meta IS NULL FROM bg_expand_compat WHERE id=1").Scan(&empty))
	require.True(t, empty)
	_, err = integrationDB.ExecContext(ctx, "INSERT INTO bg_expand_compat (id,old_value) VALUES (2,'old-version')")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO bg_expand_compat (id,old_value,engine_meta) VALUES (3,'new-version','{"engine":"test"}')`)
	require.NoError(t, err)
	var old string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT old_value FROM bg_expand_compat WHERE id=3").Scan(&old))
	require.Equal(t, "new-version", old)
	fsys := fstest.MapFS{entry.Filename: &fstest.MapFile{Data: []byte(content)}}
	require.NoError(t, applyMigrationsFSWithPolicy(ctx, integrationDB, fsys, &blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}}))
	var lockAfter, statementAfter string
	require.NoError(t, conn.QueryRowContext(ctx, "SHOW lock_timeout").Scan(&lockAfter))
	require.NoError(t, conn.QueryRowContext(ctx, "SHOW statement_timeout").Scan(&statementAfter))
	require.Equal(t, lockBefore, lockAfter)
	require.Equal(t, statementBefore, statementAfter)
}

func TestBlueGreenExpansionLockConflictLeavesOldQueriesAvailable(t *testing.T) {
	entry, content, conn := expansionFixture(t, "bg_expand_busy")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	old, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = old.Rollback() }()
	_, err = old.ExecContext(ctx, "INSERT INTO bg_expand_busy (id,old_value) VALUES (1,'in-flight')")
	require.NoError(t, err)
	started := time.Now()
	err = applyBlueGreenExpansion(ctx, conn, entry, content)
	require.Error(t, err)
	require.Less(t, time.Since(started), 2*time.Second)
	require.False(t, isTransientDatabaseInitializationError(err), "锁冲突不能自动重试")
	probe, stop := context.WithTimeout(ctx, time.Second)
	defer stop()
	_, err = integrationDB.ExecContext(probe, "INSERT INTO bg_expand_busy (id,old_value) VALUES (2,'old-still-writes')")
	require.NoError(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(probe, "SELECT count(*) FROM bg_expand_busy").Scan(&count))
	require.NoError(t, integrationDB.QueryRowContext(probe, "SELECT count(*) FROM schema_migrations WHERE filename=$1", entry.Filename).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, old.Commit())
	// 仅测试显式重试成功；生产执行器不会自动重启失败候选。
	require.NoError(t, applyBlueGreenExpansion(ctx, conn, entry, content))
}

func TestBlueGreenExpansionRejectsSchemaDriftAndRollsBack(t *testing.T) {
	for index, definition := range []string{"text", "jsonb NOT NULL", "jsonb DEFAULT '{}'", "jsonb CHECK (engine_meta IS NOT NULL)"} {
		t.Run(definition, func(t *testing.T) {
			table := fmt.Sprintf("bg_expand_drift_%d", index)
			entry, content, conn := expansionFixture(t, table)
			_, err := integrationDB.ExecContext(context.Background(), "ALTER TABLE "+table+" ADD COLUMN engine_meta "+definition)
			require.NoError(t, err)
			require.Error(t, applyBlueGreenExpansion(context.Background(), conn, entry, content))
			var count int
			require.NoError(t, integrationDB.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", entry.Filename).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestBlueGreenExpansionContentModerationOldContract(t *testing.T) {
	ctx := context.Background()
	const table = "bg_expand_moderation"
	_, err := integrationDB.ExecContext(ctx, "CREATE TABLE "+table+" (LIKE content_moderation_logs INCLUDING DEFAULTS INCLUDING CONSTRAINTS)")
	require.NoError(t, err)
	name := "9999_bg_expand_moderation.sql"
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP TABLE "+table)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename=$1", name)
	})
	_, err = integrationDB.ExecContext(ctx, "ALTER TABLE "+table+" DROP COLUMN engine_meta")
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("238b_content_moderation_engine_meta.sql")
	require.NoError(t, err)
	content := strings.ReplaceAll(string(raw), "content_moderation_logs", table)
	entry := expansionEntry(name, table, content)
	conn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	// 列清单对应 v0.1.242 的 CreateLog；既有行和旧版写入不提供新字段。
	oldInsert := `INSERT INTO bg_expand_moderation (
        request_id,user_id,user_email,api_key_id,api_key_name,group_id,group_name,
        endpoint,provider,model,mode,action,flagged,highest_category,highest_score,
        category_scores,threshold_snapshot,input_excerpt,upstream_latency_ms,error,
        violation_count,auto_banned,email_sent,queue_delay_ms,matched_keyword
        ) VALUES ($1,NULL,'',NULL,'',NULL,'','/v1/messages','test','test','observe','allow',false,'',0,
        '{}','{}','',NULL,'',0,false,false,NULL,'') RETURNING id,created_at`
	insert := func(request string) {
		var id int64
		var created time.Time
		require.NoError(t, integrationDB.QueryRowContext(ctx, oldInsert, request).Scan(&id, &created))
	}
	insert("before")
	require.NoError(t, applyBlueGreenExpansion(ctx, conn, entry, content))
	insert("after-old")
	_, err = integrationDB.ExecContext(ctx, `UPDATE bg_expand_moderation SET engine_meta='{"engine":"test"}' WHERE request_id='before'`)
	require.NoError(t, err)
	var request string
	var flagged bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT request_id,flagged FROM bg_expand_moderation WHERE request_id='before'").Scan(&request, &flagged))
	require.Equal(t, "before", request)
	var empty bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT engine_meta IS NULL FROM bg_expand_moderation WHERE request_id='after-old'").Scan(&empty))
	require.True(t, empty)
}
