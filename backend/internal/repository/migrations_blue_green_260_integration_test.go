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

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func migration260Fixture(t *testing.T) (*sql.DB, *sql.Conn) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	_, err := db.Exec(`ALTER TABLE groups ADD COLUMN platform text NOT NULL DEFAULT 'composite';
        ALTER TABLE groups ADD COLUMN allow_image_generation boolean NOT NULL DEFAULT false;
        ALTER TABLE groups ADD COLUMN name text NOT NULL DEFAULT '';
        UPDATE groups SET platform='gemini' WHERE id=1;
        UPDATE groups SET platform='antigravity' WHERE id=2;
        CREATE TABLE users(id bigint PRIMARY KEY,email text NOT NULL,balance numeric NOT NULL DEFAULT 0,deleted_at timestamptz);
        INSERT INTO users(id,email,balance) VALUES(1,'legacy@example.invalid',19.5)`)
	require.NoError(t, err)
	return db, conn
}

func migration260FS(t *testing.T) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	for _, entry := range blueGreen260Migrations {
		raw, err := migrations.FS.ReadFile(entry.Filename)
		require.NoError(t, err)
		files[entry.Filename] = &fstest.MapFile{Data: raw}
	}
	return files
}

func runMigration260(t *testing.T, db *sql.DB) error {
	t.Helper()
	return applyMigrationsFSWithPolicy(context.Background(), db, migration260FS(t),
		&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen260Migrations})
}

func assertMigration260Unchanged(t *testing.T, db *sql.DB) {
	t.Helper()
	var clean bool
	require.NoError(t, db.QueryRow(`SELECT NOT EXISTS(SELECT 1 FROM groups WHERE allow_image_generation)
        AND NOT EXISTS(SELECT 1 FROM schema_migrations WHERE filename IN ($1,$2))`,
		blueGreen260Migrations[0].Filename, blueGreen260Migrations[1].Filename).Scan(&clean))
	require.True(t, clean)
}

func TestBlueGreen260AtomicUpgradeOldWritesAndReplay(t *testing.T) {
	db, conn := migration260Fixture(t)
	var before, after string
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&before))
	require.NoError(t, runMigration260(t, db))
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT count(*)=3 AND bool_and(allow_image_generation=(platform IN ('gemini','antigravity'))) FROM groups`).Scan(&valid))
	require.True(t, valid)
	_, err := db.Exec(`INSERT INTO scoped_admin_api_keys(id,name,key_prefix,key_hash,created_by) VALUES('key','new-key','prefix','hash',1);
        UPDATE groups SET name='old-client',allow_image_generation=false WHERE id=1;
        UPDATE users SET email='updated@example.invalid',deleted_at=now() WHERE id=1`)
	require.NoError(t, err)
	// 旧版显式列更新与软删除保持有效；重新启动不再次启用已经关闭的生图开关。
	require.NoError(t, runMigration260(t, db))
	require.NoError(t, db.QueryRow(`SELECT (SELECT NOT allow_image_generation AND name='old-client'
        AND model_pricing->0->>'input_price'='2' FROM groups WHERE id=1)
        AND (SELECT email='updated@example.invalid' AND balance=19.5 AND deleted_at IS NOT NULL FROM users WHERE id=1)
        AND (SELECT count(*)=1 FROM scoped_admin_api_keys)`).Scan(&valid))
	require.True(t, valid)
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&after))
	require.Equal(t, before, after)
}

func TestBlueGreen260CompleteOldSchemaUpgrade(t *testing.T) {
	db, _ := migration260Fixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if _, added := migration260FS(t)[name]; added {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	_, err = db.Exec(`INSERT INTO groups(name,platform,allow_image_generation,rate_multiplier)
        VALUES('legacy-gemini','gemini',false,1.25),('legacy-composite','composite',false,1.5)`)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: blueGreen260Migrations}))
	var userID int64
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('legacy-delete@example.invalid','test-hash') RETURNING id`).Scan(&userID))
	_, err = db.Exec(`INSERT INTO scoped_admin_api_keys(id,name,key_prefix,key_hash,created_by) VALUES('legacy-delete','new-key','prefix','hash',$1)`, userID)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, NewUserRepository(client, db).Delete(context.Background(), userID))
	var softDeleted bool
	require.NoError(t, db.QueryRow(`SELECT (SELECT deleted_at IS NOT NULL FROM users WHERE id=$1)
        AND EXISTS(SELECT 1 FROM scoped_admin_api_keys WHERE created_by=$1)`, userID).Scan(&softDeleted))
	require.True(t, softDeleted)
	require.NoError(t, runMigration260(t, db))
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT
        (SELECT allow_image_generation AND rate_multiplier=1.25 FROM groups WHERE name='legacy-gemini')
        AND (SELECT NOT allow_image_generation AND rate_multiplier=1.5 FROM groups WHERE name='legacy-composite')`).Scan(&valid))
	require.True(t, valid)
}

func TestBlueGreen260LockConflictAndAtomicFailure(t *testing.T) {
	for _, table := range []string{"groups", "users", "schema_migrations"} {
		t.Run(table, func(t *testing.T) {
			db, conn := migration260Fixture(t)
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec(fmt.Sprintf("LOCK TABLE %s IN ROW EXCLUSIVE MODE", table))
			require.NoError(t, err)
			started := time.Now()
			err = runSpecialMigration(t, conn, blueGreen260Migrations[0])
			require.ErrorContains(t, err, "立即获锁")
			require.Less(t, time.Since(started), time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			assertMigration260Unchanged(t, db)
		})
	}
	t.Run("末尾账本失败整体回滚", func(t *testing.T) {
		db, _ := migration260Fixture(t)
		_, err := db.Exec("ALTER TABLE schema_migrations ADD CONSTRAINT reject_247 CHECK(filename<>'247_scoped_admin_api_keys.sql')")
		require.NoError(t, err)
		require.Error(t, runMigration260(t, db))
		assertMigration260Unchanged(t, db)
		var absent bool
		require.NoError(t, db.QueryRow("SELECT to_regclass('public.scoped_admin_api_keys') IS NULL").Scan(&absent))
		require.True(t, absent)
	})
}

func TestBlueGreen260RejectsUnexpectedBaseline(t *testing.T) {
	for name, statement := range map[string]string{
		"同名表":    "CREATE TABLE scoped_admin_api_keys(id text)",
		"同名类型":   "CREATE TYPE scoped_admin_api_keys AS (value text)",
		"同名索引对象": "CREATE SEQUENCE scoped_admin_api_keys_pkey",
		"RLS":    "ALTER TABLE groups ENABLE ROW LEVEL SECURITY",
		"继承":     "CREATE TABLE inherited_groups() INHERITS(groups)",
		"部分账本":   "INSERT INTO schema_migrations(filename,checksum) VALUES('246_enable_gemini_image_generation_groups.sql','87267be7ab657173768989e2b0b105c5569c5930a7f1ed124f4cf81e5198297c')",
		"超出表大小":  "ALTER TABLE groups ADD COLUMN ballast text; ALTER TABLE groups ALTER COLUMN ballast SET STORAGE EXTERNAL; UPDATE groups SET ballast=repeat('x',17*1024*1024) WHERE id=1",
		"未知触发器": `CREATE FUNCTION unexpected_260() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
            CREATE TRIGGER unexpected BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION unexpected_260()`,
		"DDL触发器": `CREATE FUNCTION unexpected_260_ddl() RETURNS event_trigger LANGUAGE plpgsql AS $$ BEGIN RETURN; END $$;
            CREATE EVENT TRIGGER unexpected ON ddl_command_start EXECUTE FUNCTION unexpected_260_ddl()`,
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := migration260Fixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration260(t, db))
			var untouched bool
			require.NoError(t, db.QueryRow("SELECT NOT EXISTS(SELECT 1 FROM groups WHERE allow_image_generation)").Scan(&untouched))
			require.True(t, untouched)
		})
	}
}

func TestBlueGreen260ReplayRejectsSchemaDrift(t *testing.T) {
	for name, statement := range map[string]string{
		"字段":  "ALTER TABLE scoped_admin_api_keys ALTER COLUMN key_hash DROP NOT NULL",
		"默认值": "ALTER TABLE scoped_admin_api_keys ALTER COLUMN enabled SET DEFAULT false",
		"新增列": "ALTER TABLE scoped_admin_api_keys ADD COLUMN extra text",
		"索引":  "CREATE INDEX unexpected ON scoped_admin_api_keys(name)",
		"外键":  "ALTER TABLE scoped_admin_api_keys DROP CONSTRAINT scoped_admin_api_keys_created_by_fkey",
		"权限":  "ALTER TABLE scoped_admin_api_keys ENABLE ROW LEVEL SECURITY",
		"账本":  "DELETE FROM schema_migrations WHERE filename='247_scoped_admin_api_keys.sql'",
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := migration260Fixture(t)
			require.NoError(t, runMigration260(t, db))
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration260(t, db))
		})
	}
}
