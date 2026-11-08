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

func migration249Fixture(t *testing.T) (*sql.DB, *sql.Conn) {
	t.Helper()
	db, conn, _ := specialMigrationFixture(t)
	_, err := db.Exec(`CREATE TABLE users(id bigint PRIMARY KEY,email text NOT NULL,balance numeric NOT NULL DEFAULT 0,deleted_at timestamptz);
        INSERT INTO users(id,email,balance) VALUES(1,'legacy@example.invalid',19.5)`)
	require.NoError(t, err)
	return db, conn
}

func runMigration249(t *testing.T, db *sql.DB) error {
	t.Helper()
	raw, err := migrations.FS.ReadFile(blueGreen249Migration.Filename)
	require.NoError(t, err)
	return applyMigrationsFSWithPolicy(context.Background(), db,
		fstest.MapFS{blueGreen249Migration.Filename: &fstest.MapFile{Data: raw}},
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreen249Migration}})
}

func assertMigration249Absent(t *testing.T, db *sql.DB) {
	t.Helper()
	var absent bool
	require.NoError(t, db.QueryRow(`SELECT
        NOT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)
        AND NOT EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^(idx_)?mcp_oauth_')
        AND NOT EXISTS(SELECT 1 FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?mcp_oauth_')`, blueGreen249Migration.Filename).Scan(&absent))
	require.True(t, absent)
}

func TestBlueGreen249AtomicUpgradeOldWritesAndReplay(t *testing.T) {
	db, conn := migration249Fixture(t)
	var before, after, timeoutBefore, timeoutAfter string
	var nodeBefore, nodeAfter int64
	require.NoError(t, db.QueryRow("SELECT jsonb_agg(to_jsonb(u) ORDER BY id)::text FROM users u").Scan(&before))
	require.NoError(t, db.QueryRow("SELECT relfilenode FROM pg_class WHERE oid='users'::regclass").Scan(&nodeBefore))
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&timeoutBefore))
	require.NoError(t, runMigration249(t, db))
	require.NoError(t, db.QueryRow("SELECT jsonb_agg(to_jsonb(u) ORDER BY id)::text FROM users u").Scan(&after))
	require.JSONEq(t, before, after)
	require.NoError(t, db.QueryRow("SELECT relfilenode FROM pg_class WHERE oid='users'::regclass").Scan(&nodeAfter))
	require.Equal(t, nodeBefore, nodeAfter)
	_, err := db.Exec(`INSERT INTO mcp_oauth_grants(id,client_id,client_name,user_id,token_version,resource,scope,created_at,expires_at)
        VALUES('grant','client','测试客户端',1,1,'resource','user',now(),now()+interval '1 hour');
        INSERT INTO mcp_oauth_codes(code_hash,grant_id,client_id,redirect_uri,resource,code_challenge,expires_at)
        VALUES('code-digest','grant','client','https://example.invalid','resource','challenge',now()+interval '5 minutes');
        INSERT INTO mcp_oauth_credentials(key_hash,grant_id,kind,expires_at) VALUES('token-digest','grant','refresh',now()+interval '1 hour');
        UPDATE users SET email='updated@example.invalid',balance=20,deleted_at=now() WHERE id=1;
        INSERT INTO users(id,email,balance) VALUES(2,'created@example.invalid',10)`)
	require.NoError(t, err)
	// 旧版软删除和明确列更新不删除授权；重复启动仅重验结构，不重放或清空新数据。
	require.NoError(t, runMigration249(t, db))
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT
        (SELECT email='updated@example.invalid' AND balance=20 AND deleted_at IS NOT NULL FROM users WHERE id=1)
        AND (SELECT read_only FROM mcp_oauth_grants WHERE id='grant')
        AND (SELECT count(*)=1 FROM mcp_oauth_codes)
        AND (SELECT count(*)=1 FROM mcp_oauth_credentials)
        AND (SELECT count(*)=1 FROM schema_migrations WHERE filename=$1)`, blueGreen249Migration.Filename).Scan(&valid))
	require.True(t, valid)
	_, err = db.Exec("DELETE FROM users WHERE id=1")
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT
        NOT EXISTS(SELECT 1 FROM mcp_oauth_grants) AND NOT EXISTS(SELECT 1 FROM mcp_oauth_codes)
        AND NOT EXISTS(SELECT 1 FROM mcp_oauth_credentials)`).Scan(&valid))
	require.True(t, valid)
	require.NoError(t, conn.QueryRowContext(context.Background(), "SHOW statement_timeout").Scan(&timeoutAfter))
	require.Equal(t, timeoutBefore, timeoutAfter)
}

func TestBlueGreen249CompleteOldSchemaUpgrade(t *testing.T) {
	db, _ := migration249Fixture(t)
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name == blueGreen249Migration.Filename {
			continue
		}
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(context.Background(), db, old))
	require.NoError(t, applyMigrationsFSWithPolicy(context.Background(), db, migrations.FS,
		&blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreen249Migration}}))
	var userID int64
	require.NoError(t, db.QueryRow("INSERT INTO users(email,password_hash,balance) VALUES('legacy-delete@example.invalid','test-hash',19.5) RETURNING id").Scan(&userID))
	_, err = db.Exec(`INSERT INTO mcp_oauth_grants(id,client_id,client_name,user_id,token_version,resource,scope,created_at,expires_at)
        VALUES('legacy-grant','client','测试客户端',$1,1,'resource','user',now(),now()+interval '1 hour')`, userID)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	legacy := NewUserRepository(client, db)
	user, err := legacy.GetByID(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, 19.5, user.Balance)
	require.NoError(t, client.User.UpdateOneID(userID).SetBalance(21).Exec(context.Background()))
	require.NoError(t, legacy.Delete(context.Background(), userID))
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT (SELECT deleted_at IS NOT NULL AND balance=21 FROM users WHERE id=$1)
        AND EXISTS(SELECT 1 FROM mcp_oauth_grants WHERE user_id=$1)`, userID).Scan(&valid))
	require.True(t, valid)
	require.NoError(t, runMigration249(t, db))
}

func TestBlueGreen249LockConflictAndAtomicFailure(t *testing.T) {
	for _, table := range []string{"users", "schema_migrations"} {
		t.Run(table, func(t *testing.T) {
			db, conn := migration249Fixture(t)
			old, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = old.Rollback() }()
			_, err = old.Exec(fmt.Sprintf("LOCK TABLE %s IN ROW EXCLUSIVE MODE", table))
			require.NoError(t, err)
			started := time.Now()
			err = runSpecialMigration(t, conn, blueGreen249Migration)
			require.ErrorContains(t, err, "立即获锁")
			require.Less(t, time.Since(started), time.Second)
			require.False(t, isTransientDatabaseInitializationError(err))
			assertMigration249Absent(t, db)
		})
	}
	t.Run("末尾账本失败整体回滚", func(t *testing.T) {
		db, _ := migration249Fixture(t)
		_, err := db.Exec("ALTER TABLE schema_migrations ADD CONSTRAINT reject_249 CHECK(filename<>'249_mcp_oauth.sql')")
		require.NoError(t, err)
		require.Error(t, runMigration249(t, db))
		assertMigration249Absent(t, db)
	})
}

func TestBlueGreen249RejectsUnexpectedBaseline(t *testing.T) {
	for name, statement := range map[string]string{
		"同名表":    "CREATE TABLE mcp_oauth_grants(id text)",
		"同名类型":   "CREATE TYPE mcp_oauth_codes AS (value text)",
		"同名索引对象": "CREATE SEQUENCE idx_mcp_oauth_credentials_grant",
		"RLS":    "ALTER TABLE users ENABLE ROW LEVEL SECURITY",
		"继承":     "CREATE TABLE inherited_users() INHERITS(users)",
		"伪造账本":   "INSERT INTO schema_migrations(filename,checksum) VALUES('249_mcp_oauth.sql','751a1dcee6108afcecbd580b6e3140cca3480853fecd2174861b02f3a71e54a1')",
		"超出表大小":  "ALTER TABLE users ADD COLUMN ballast text; ALTER TABLE users ALTER COLUMN ballast SET STORAGE EXTERNAL; UPDATE users SET ballast=repeat('x',17*1024*1024)",
		"未知触发器": `CREATE FUNCTION unexpected_249() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
            CREATE TRIGGER unexpected BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION unexpected_249()`,
		"DDL触发器": `CREATE FUNCTION unexpected_249_ddl() RETURNS event_trigger LANGUAGE plpgsql AS $$ BEGIN RETURN; END $$;
            CREATE EVENT TRIGGER unexpected ON ddl_command_start EXECUTE FUNCTION unexpected_249_ddl()`,
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := migration249Fixture(t)
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration249(t, db))
			var unchanged bool
			require.NoError(t, db.QueryRow("SELECT email='legacy@example.invalid' AND balance=19.5 FROM users WHERE id=1").Scan(&unchanged))
			require.True(t, unchanged)
		})
	}
}

func TestBlueGreen249ReplayRejectsSchemaDrift(t *testing.T) {
	for name, statement := range map[string]string{
		"字段":      "ALTER TABLE mcp_oauth_grants ALTER COLUMN client_id DROP NOT NULL",
		"默认值":     "ALTER TABLE mcp_oauth_grants ALTER COLUMN read_only SET DEFAULT false",
		"新增列":     "ALTER TABLE mcp_oauth_codes ADD COLUMN extra text",
		"索引":      "DROP INDEX idx_mcp_oauth_grants_user_created",
		"外键":      "ALTER TABLE mcp_oauth_grants DROP CONSTRAINT mcp_oauth_grants_user_id_fkey",
		"检查约束":    "ALTER TABLE mcp_oauth_credentials DROP CONSTRAINT mcp_oauth_credentials_kind_check",
		"权限":      "ALTER TABLE mcp_oauth_credentials ENABLE ROW LEVEL SECURITY",
		"额外类型":    "CREATE TYPE mcp_oauth_unknown AS (value text)",
		"禁用内部触发器": "ALTER TABLE mcp_oauth_codes DISABLE TRIGGER ALL",
		"账本缺失":    "DELETE FROM schema_migrations WHERE filename='249_mcp_oauth.sql'",
		"账本漂移":    "UPDATE schema_migrations SET checksum=repeat('0',64) WHERE filename='249_mcp_oauth.sql'",
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := migration249Fixture(t)
			require.NoError(t, runMigration249(t, db))
			_, err := db.Exec(statement)
			require.NoError(t, err)
			require.Error(t, runMigration249(t, db))
		})
	}
}
