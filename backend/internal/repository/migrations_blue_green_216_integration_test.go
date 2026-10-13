//go:build integration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func migration216Fixture(t *testing.T) (*sql.DB, *sql.Conn) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	_, err := db.Exec("ALTER TABLE groups ADD COLUMN name text NOT NULL DEFAULT 'old'; ALTER TABLE groups ADD COLUMN description text")
	require.NoError(t, err)
	return db, conn
}

func migration216FS(t *testing.T) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	for _, entry := range blueGreen216Migrations {
		content, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		files[entry.Filename] = &fstest.MapFile{Data: content}
	}
	return files
}

func runMigration216(t *testing.T, db *sql.DB) error {
	t.Helper()
	return applyMigrationsFSWithPolicy(context.Background(), db, migration216FS(t),
		&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen216Migrations})
}

func assertMigration216Unchanged(t *testing.T, db *sql.DB) {
	t.Helper()
	var unchanged bool
	err := db.QueryRow(`SELECT to_regclass('public.gemini_cached_contents') IS NULL
        AND NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='public.groups'::regclass AND attname='i18n' AND NOT attisdropped)
        AND NOT EXISTS (SELECT 1 FROM schema_migrations WHERE filename IN ($1,$2,$3,$4))`,
		blueGreen216Migrations[0].Filename, blueGreen216Migrations[1].Filename,
		blueGreen216Migrations[2].Filename, blueGreen216Migrations[3].Filename).Scan(&unchanged)
	require.NoError(t, err)
	require.True(t, unchanged)
}

func TestBlueGreen216LegacyReadWriteAndReplay(t *testing.T) {
	db, conn := migration216Fixture(t)
	var pricing, lockBefore, statementBefore, transactionBefore string
	require.NoError(t, db.QueryRow("SELECT model_pricing::text FROM groups WHERE id=1").Scan(&pricing))
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW lock_timeout").Scan(&lockBefore))
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&statementBefore))
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW transaction_timeout").Scan(&transactionBefore))
	require.NoError(t, runSpecialMigration(t, conn, blueGreen216Migrations[0]))
	var after string
	require.NoError(t, db.QueryRow("SELECT model_pricing::text FROM groups WHERE id=1").Scan(&after))
	require.Equal(t, pricing, after)
	// 旧版使用显式列名，未提供i18n的写入获得空对象，更新不覆盖新版译文。
	_, err := db.Exec(`INSERT INTO groups(id,name,description) VALUES(4,'old-client','old-description');
        UPDATE groups SET i18n='{"en":{"name":"new-name"}}' WHERE id=4;
        UPDATE groups SET name='old-update',description='old-update' WHERE id=4`)
	require.NoError(t, err)
	var localized string
	require.NoError(t, db.QueryRow("SELECT i18n::text FROM groups WHERE id=4").Scan(&localized))
	require.JSONEq(t, `{"en":{"name":"new-name"}}`, localized)
	cache := &service.GeminiCachedContent{PublicID: "new-cache", UserID: 1, APIKeyID: 2, GroupID: 4, AccountID: 3,
		UpstreamName: "cachedContents/example", Model: "model", UpstreamModel: "model", ExpireTime: time.Now().Add(time.Hour)}
	repo := NewGeminiCachedContentRepository(db)
	require.NoError(t, repo.Create(context.Background(), cache))
	require.ErrorIs(t, repo.Create(context.Background(), cache), service.ErrGeminiCachedContentExists)
	require.NoError(t, runMigration216(t, db), "完整账本可以带真实缓存行只读重验")
	for _, item := range []struct{ setting, expected string }{
		{"lock_timeout", lockBefore}, {"statement_timeout", statementBefore}, {"transaction_timeout", transactionBefore},
	} {
		require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW "+item.setting).Scan(&after))
		require.Equal(t, item.expected, after)
	}
}

func TestBlueGreen216CompleteOldSchemaUpgrade(t *testing.T) {
	db, conn := migration216Fixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if _, isNew := migration216FS(t)[name]; isNew {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	_, err = db.Exec("INSERT INTO groups(name,description,rate_multiplier) VALUES('old-upgrade','before',1.25)")
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen216Migrations}))
	_, err = db.Exec("UPDATE groups SET description='after' WHERE name='old-upgrade'; INSERT INTO groups(name,rate_multiplier) VALUES('old-after',1.5)")
	require.NoError(t, err)
	var valid bool
	require.NoError(t, conn.QueryRowContext(context.Background(), `SELECT bool_and(i18n='{}'::jsonb) AND count(*)=2 FROM groups WHERE name IN ('old-upgrade','old-after')`).Scan(&valid))
	require.True(t, valid)
}

func TestBlueGreen216LockConflictAndAtomicFailure(t *testing.T) {
	t.Run("旧查询持锁", func(t *testing.T) {
		db, conn := migration216Fixture(t)
		old, err := db.Begin()
		require.NoError(t, err)
		defer func() { _ = old.Rollback() }()
		_, err = old.Exec("UPDATE groups SET name='in-flight' WHERE id=1")
		require.NoError(t, err)
		started := time.Now()
		err = runSpecialMigration(t, conn, blueGreen216Migrations[0])
		require.ErrorContains(t, err, "立即锁定")
		require.Less(t, time.Since(started), 2*time.Second)
		require.False(t, isTransientDatabaseInitializationError(err))
		assertMigration216Unchanged(t, db)
	})
	t.Run("末尾账本写入失败", func(t *testing.T) {
		db, _ := migration216Fixture(t)
		_, err := db.Exec(`CREATE FUNCTION fail_last_216() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
            IF NEW.filename='245_gemini_cached_contents_mutation_lease.sql' THEN RAISE EXCEPTION '测试末尾故障'; END IF;
            RETURN NEW; END $$;
            CREATE TRIGGER fail_last_216 BEFORE INSERT ON schema_migrations FOR EACH ROW EXECUTE FUNCTION fail_last_216()`)
		require.NoError(t, err)
		require.ErrorContains(t, runMigration216(t, db), "测试末尾故障")
		assertMigration216Unchanged(t, db)
	})
}

func TestBlueGreen216RejectsUnexpectedBaseline(t *testing.T) {
	cases := map[string]string{
		"同名缓存表":  "CREATE TABLE gemini_cached_contents(id int)",
		"同名序列":   "CREATE SEQUENCE gemini_cached_contents_id_seq",
		"已有译文字段": "ALTER TABLE groups ADD COLUMN i18n jsonb",
		"部分账本":   "INSERT INTO schema_migrations(filename,checksum) VALUES('243_gemini_cached_contents.sql','2d345cff5c64b01e705f04ec972e5d5ec3d618fe71dc7d45bc80fdd38c6a89d5')",
		"RLS":    "ALTER TABLE groups ENABLE ROW LEVEL SECURITY",
		"继承":     "CREATE TABLE child_groups() INHERITS(groups)",
		"未知触发器":  "CREATE FUNCTION unknown_216() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$; CREATE TRIGGER unknown_216 BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION unknown_216()",
		"超出表大小":  "ALTER TABLE groups ADD COLUMN ballast text; ALTER TABLE groups ALTER COLUMN ballast SET STORAGE EXTERNAL; UPDATE groups SET ballast=repeat('x',17*1024*1024) WHERE id=1",
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			db, _ := migration216Fixture(t)
			_, err := db.Exec(sql)
			require.NoError(t, err)
			require.Error(t, runMigration216(t, db))
			var count int
			require.NoError(t, db.QueryRow("SELECT count(*) FROM schema_migrations WHERE filename=$1", blueGreen216Migrations[0].Filename).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestBlueGreen216ReplayRejectsSchemaDrift(t *testing.T) {
	for _, sql := range []string{
		"ALTER TABLE groups ALTER COLUMN i18n DROP DEFAULT",
		"ALTER TABLE gemini_cached_contents ALTER COLUMN channel_usage DROP NOT NULL",
		"DROP INDEX idx_gemini_cached_contents_pending_mutation",
		"ALTER TABLE gemini_cached_contents ADD CONSTRAINT unexpected CHECK(user_id>0)",
	} {
		t.Run(sql, func(t *testing.T) {
			db, _ := migration216Fixture(t)
			require.NoError(t, runMigration216(t, db))
			_, err := db.Exec(sql)
			require.NoError(t, err)
			require.Error(t, runMigration216(t, db))
		})
	}
}
