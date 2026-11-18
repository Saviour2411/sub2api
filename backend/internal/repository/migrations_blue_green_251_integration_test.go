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

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func migration251Fixture(t *testing.T) (*sql.DB, *sql.Conn) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	_, err := db.Exec(`CREATE TABLE usage_logs(id bigserial PRIMARY KEY,group_id bigint,created_at timestamptz NOT NULL DEFAULT now(),actual_cost numeric NOT NULL DEFAULT 0);`)
	require.NoError(t, err)
	old := fstest.MapFS{}
	for _, name := range []string{"222_group_usage_daily_rollups.sql", "223_group_usage_rollup_timezone.sql", "226_group_usage_rollup_current_insert_nonblocking.sql"} {
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	return db, conn
}

func migration251Files(t *testing.T) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	for _, entry := range blueGreen251Migrations {
		raw, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		files[entry.Filename] = &fstest.MapFile{Data: raw}
	}
	return files
}

func runMigration251(t *testing.T, db *sql.DB) error {
	t.Helper()
	return applyMigrationsFSWithPolicy(context.Background(), db, migration251Files(t),
		&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen251Migrations})
}

func assertMigration251Unchanged(t *testing.T, db *sql.DB) {
	t.Helper()
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT NOT EXISTS(SELECT 1 FROM schema_migrations WHERE filename IN ($1,$2,$3))
        AND NOT EXISTS(SELECT 1 FROM pg_attribute WHERE NOT attisdropped AND
          ((attrelid='groups'::regclass AND attname='force_openai_ultrafast') OR
           (attrelid='usage_group_rollup_state'::regclass AND attname IN ('expired_before','building_until','build_dirty_from'))))
        AND NOT EXISTS(SELECT 1 FROM pg_proc WHERE pronamespace='public'::regnamespace
            AND proname IN ('apply_group_usage_rollup_invalidation','invalidate_group_usage_rollups_at'))`,
		blueGreen251Migrations[0].Filename, blueGreen251Migrations[1].Filename, blueGreen251Migrations[2].Filename).Scan(&valid))
	require.True(t, valid)
}

func TestBlueGreen251UpgradeOldWritesReplayAndNoRewrite(t *testing.T) {
	db, conn := migration251Fixture(t)
	var before, after, timeoutBefore, timeoutAfter string
	const nodes = "SELECT jsonb_object_agg(relname,relfilenode)::text FROM pg_class WHERE oid IN ('groups'::regclass,'usage_logs'::regclass,'usage_group_rollup_state'::regclass)"
	require.NoError(t, db.QueryRow(nodes).Scan(&before))
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW lock_timeout").Scan(&timeoutBefore))
	require.NoError(t, runMigration251(t, db))
	require.NoError(t, db.QueryRow(nodes).Scan(&after))
	require.Equal(t, before, after)
	_, err := db.Exec(`UPDATE groups SET force_openai_ultrafast=true WHERE id=1;
        INSERT INTO groups(id,model_pricing) VALUES(4,'[]'); UPDATE groups SET model_pricing='[]' WHERE id=1;
        INSERT INTO usage_logs(group_id,actual_cost) VALUES(1,2.5);
        UPDATE usage_group_rollup_state SET closed_before=CURRENT_DATE,retained_from=now(),timezone_name='UTC',updated_at=now() WHERE id=1;`)
	require.NoError(t, err)
	require.NoError(t, runMigration251(t, db))
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT (SELECT force_openai_ultrafast FROM groups WHERE id=1)
        AND (SELECT NOT force_openai_ultrafast FROM groups WHERE id=4)
        AND (SELECT sum(actual_cost)=2.5 FROM usage_logs)
        AND (SELECT expired_before=TIMESTAMPTZ '1970-01-01 00:00:00+00' AND building_until IS NULL AND build_dirty_from IS NULL FROM usage_group_rollup_state)`).Scan(&valid))
	require.True(t, valid)
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW lock_timeout").Scan(&timeoutAfter))
	require.Equal(t, timeoutBefore, timeoutAfter)
}

func TestBlueGreen251CompleteOldSchemaUpgrade(t *testing.T) {
	db, _, _ := specialMigrationFixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name >= blueGreen251Migrations[0].Filename {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	_, err = db.Exec("INSERT INTO groups(name,platform,rate_multiplier) VALUES('legacy','openai',1.25)")
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen251Migrations}))
	require.NoError(t, runMigration251(t, db))
	_, err = db.Exec("UPDATE groups SET name='legacy-updated' WHERE name='legacy'")
	require.NoError(t, err)
	var valid bool
	require.NoError(t, db.QueryRow("SELECT NOT force_openai_ultrafast AND rate_multiplier=1.25 FROM groups WHERE name='legacy-updated'").Scan(&valid))
	require.True(t, valid)
}

func TestBlueGreen251LockConflictAndAtomicFailure(t *testing.T) {
	for _, table := range []string{"usage_logs", "groups", "usage_group_rollup_state", "usage_group_daily_rollups", "schema_migrations"} {
		t.Run(table, func(t *testing.T) {
			db, conn := migration251Fixture(t)
			blocker, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback() }()
			_, err = blocker.Exec(fmt.Sprintf("LOCK TABLE %s IN ROW EXCLUSIVE MODE", table))
			require.NoError(t, err)
			started := time.Now()
			err = runSpecialMigration(t, conn, blueGreen251Migrations[0])
			require.ErrorContains(t, err, "立即获锁")
			require.Less(t, time.Since(started), time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			assertMigration251Unchanged(t, db)
		})
	}
	t.Run("末条账本失败整体回滚", func(t *testing.T) {
		db, _ := migration251Fixture(t)
		_, err := db.Exec("ALTER TABLE schema_migrations ADD CONSTRAINT reject_252 CHECK(filename<>'252_group_usage_rollup_current_insert_nonblocking.sql')")
		require.NoError(t, err)
		require.Error(t, runMigration251(t, db))
		assertMigration251Unchanged(t, db)
		var hash string
		require.NoError(t, db.QueryRow("SELECT encode(sha256(convert_to(pg_get_functiondef('invalidate_group_usage_rollup_state_after_insert()'::regprocedure),'UTF8')),'hex')").Scan(&hash))
		require.Equal(t, blueGreen251FunctionsBefore["invalidate_group_usage_rollup_state_after_insert"], hash)
	})
}

func TestBlueGreen251RejectsDriftAndExtraMigration(t *testing.T) {
	for name, statement := range map[string]string{
		"同名字段":   "ALTER TABLE groups ADD COLUMN force_openai_ultrafast boolean",
		"状态列漂移":  "ALTER TABLE usage_group_rollup_state ALTER COLUMN closed_before DROP DEFAULT",
		"函数漂移":   "ALTER FUNCTION invalidate_group_usage_rollup_state_after_insert() SECURITY DEFINER",
		"触发器缺失":  "DROP TRIGGER usage_logs_group_rollup_invalidate_insert ON usage_logs",
		"启用RLS":  "ALTER TABLE groups ENABLE ROW LEVEL SECURITY",
		"状态缺失":   "DELETE FROM usage_group_rollup_state",
		"错误历史账本": "UPDATE schema_migrations SET checksum='bad' WHERE filename='223_group_usage_rollup_timezone.sql'",
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := migration251Fixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration251(t, db))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename>='251'").Scan(&count))
			require.Zero(t, count)
		})
	}
	t.Run("额外迁移先拒绝", func(t *testing.T) {
		db, _ := migration251Fixture(t)
		files := migration251Files(t)
		files["253_extra.sql"] = &fstest.MapFile{Data: []byte("SELECT 1;")}
		require.ErrorContains(t, applyMigrationsFSWithPolicy(context.Background(), db, files,
			&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen251Migrations}), "未获批准")
		assertMigration251Unchanged(t, db)
	})
	t.Run("部分账本拒绝补写", func(t *testing.T) {
		db, _ := migration251Fixture(t)
		require.NoError(t, runMigration251(t, db))
		_, err := db.Exec("DELETE FROM schema_migrations WHERE filename=$1", blueGreen251Migrations[2].Filename)
		require.NoError(t, err)
		require.ErrorContains(t, runMigration251(t, db), "账本不完整")
	})
	t.Run("重放检查函数漂移", func(t *testing.T) {
		db, _ := migration251Fixture(t)
		require.NoError(t, runMigration251(t, db))
		_, err := db.Exec("ALTER FUNCTION apply_group_usage_rollup_invalidation(date) SECURITY DEFINER")
		require.NoError(t, err)
		require.ErrorContains(t, runMigration251(t, db), "函数定义漂移")
	})
}

func TestBlueGreen251CurrentInsertAndHistoricalInvalidation(t *testing.T) {
	db, _ := migration251Fixture(t)
	require.NoError(t, runMigration251(t, db))
	_, err := db.Exec("UPDATE usage_group_rollup_state SET closed_before=CURRENT_DATE,timezone_name='UTC',building_until=CURRENT_DATE WHERE id=1")
	require.NoError(t, err)
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec("SELECT id FROM usage_group_rollup_state WHERE id=1 FOR UPDATE")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_, err = db.ExecContext(ctx, "INSERT INTO usage_logs(group_id,actual_cost,created_at) VALUES(1,2,clock_timestamp())")
	require.NoError(t, err)
	historical := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := db.ExecContext(ctx, "INSERT INTO usage_logs(group_id,actual_cost,created_at) VALUES(1,3,CURRENT_DATE-INTERVAL '1 day')")
		historical <- err
	}()
	select {
	case err := <-historical:
		t.Fatalf("历史写入提前结束：%v", err)
	case <-time.After(150 * time.Millisecond):
	}
	require.NoError(t, blocker.Commit())
	require.NoError(t, <-historical)
	var valid bool
	require.NoError(t, db.QueryRow("SELECT closed_before=CURRENT_DATE-1 AND build_dirty_from=CURRENT_DATE-1 FROM usage_group_rollup_state").Scan(&valid))
	require.True(t, valid)
}

func TestBlueGreen251LegacyAndNewRollupBuildInterleave(t *testing.T) {
	db, _ := migration251Fixture(t)
	require.NoError(t, runMigration251(t, db))
	today := service.GroupUsageTodayStart(time.Now())
	_, err := db.Exec("INSERT INTO usage_logs(group_id,actual_cost,created_at) VALUES(1,2,$1),(1,3,$1)", today.AddDate(0, 0, -1))
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	plan, err := beginGroupUsageRollupBuild(context.Background(), tx, today)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NoError(t, tx.Commit())
	// 固定旧版在单个FOR UPDATE事务中重建并发布；它不读取或覆盖新增字段。
	legacy, err := db.Begin()
	require.NoError(t, err)
	_, err = legacy.Exec("SELECT id FROM usage_group_rollup_state WHERE id=1 FOR UPDATE")
	require.NoError(t, err)
	_, err = legacy.Exec(`DELETE FROM usage_group_daily_rollups;
        INSERT INTO usage_group_daily_rollups(bucket_date,group_id,actual_cost)
        SELECT (created_at AT TIME ZONE 'Asia/Shanghai')::date,group_id,sum(actual_cost) FROM usage_logs GROUP BY 1,2;`)
	require.NoError(t, err)
	_, err = legacy.Exec("UPDATE usage_group_rollup_state SET closed_before=$1::date WHERE id=1", plan.targetDate)
	require.NoError(t, err)
	require.NoError(t, legacy.Commit())
	tx, err = db.Begin()
	require.NoError(t, err)
	require.NoError(t, buildGroupUsageRollups(context.Background(), tx, plan))
	require.NoError(t, tx.Commit())
	_, err = db.Exec("UPDATE usage_logs SET actual_cost=7 WHERE actual_cost=2")
	require.NoError(t, err)
	tx, err = db.Begin()
	require.NoError(t, err)
	dirty, err := publishGroupUsageRollups(context.Background(), tx, plan)
	require.NoError(t, err)
	require.True(t, dirty)
	require.NoError(t, tx.Commit())
	repo := newDashboardAggregationRepositoryWithSQL(db)
	require.NoError(t, repo.SyncGroupUsageRollups(context.Background(), today))
	var total float64
	require.NoError(t, db.QueryRow("SELECT sum(actual_cost) FROM usage_group_daily_rollups").Scan(&total))
	require.Equal(t, float64(10), total)
}
