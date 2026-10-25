package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
)

const blueGreen260Profile = "gemini-scoped-246-247-v1"

var blueGreen260Migrations = []blueGreenMigration{
	{Filename: "246_enable_gemini_image_generation_groups.sql", Checksum: "87267be7ab657173768989e2b0b105c5569c5930a7f1ed124f4cf81e5198297c", Profile: blueGreen260Profile},
	{Filename: "247_scoped_admin_api_keys.sql", Checksum: "03445774e2dfad32da9e23279193d1bca02953896bc3d3b8a87ba0b36b6ed577", Profile: blueGreen260Profile},
}

func validateBlueGreen260Policy(policy *blueGreenMigrationPolicy) error {
	count := 0
	for _, entry := range policy.Migrations {
		if entry.Profile == blueGreen260Profile {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != len(blueGreen260Migrations) || count != len(policy.Migrations) {
		return fmt.Errorf("蓝绿246/247必须包含完整且唯一的两条固定迁移")
	}
	for _, expected := range blueGreen260Migrations {
		if entry, ok := policy.find(expected.Filename); !ok || entry != expected {
			return fmt.Errorf("蓝绿246/247迁移清单不符")
		}
	}
	return nil
}

func validateBlueGreen260Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen260Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen260Migrations[0].Filename); !ok {
		return nil
	}
	count := 0
	for _, entry := range blueGreen260Migrations {
		if digest, ok := applied[entry.Filename]; ok {
			if digest != entry.Checksum {
				return fmt.Errorf("蓝绿246/247迁移账本摘要不符")
			}
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != len(blueGreen260Migrations) {
		return fmt.Errorf("蓝绿246/247迁移账本不完整，禁止自动补写")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := verifyBlueGreen260Schema(ctx, tx); err != nil {
		return err
	}
	// 已登记的回填不重放，保留管理员后来显式关闭的生图开关。
	return tx.Commit()
}

func applyBlueGreen260(ctx context.Context, conn *sql.Conn, first blueGreenMigration) error {
	if first != blueGreen260Migrations[0] {
		return fmt.Errorf("蓝绿246/247只能从固定246迁移开始完整事务")
	}
	contents := make([]string, len(blueGreen260Migrations))
	for i, entry := range blueGreen260Migrations {
		raw, err := migrations.FS.ReadFile(entry.Filename)
		if err != nil {
			return err
		}
		contents[i] = string(raw)
		if err := validateBlueGreenSQL(entry, contents[i]); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{"SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'", "SET LOCAL search_path=public"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version/10000 != 18 {
		return fmt.Errorf("蓝绿246/247固定迁移仅支持 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	// 回填、外键及账本使用同一短事务；普通查询可继续，写锁冲突立即退出。
	for _, table := range []string{"groups", "users", "schema_migrations"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("LOCK TABLE ONLY public.%s IN SHARE ROW EXCLUSIVE MODE NOWAIT", table)); err != nil {
			return fmt.Errorf("蓝绿246/247无法立即获锁，保留旧实例：%w", err)
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND pg_total_relation_size(c.oid)<=$2
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND (c.relname='users' OR NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal AND NOT (
                c.relname='groups' AND t.tgenabled='O'
                AND pg_get_triggerdef(t.oid)='CREATE TRIGGER trg_groups_auth_cache_invalidation AFTER DELETE OR UPDATE ON public.groups FOR EACH ROW EXECUTE FUNCTION enqueue_group_auth_cache_invalidation()'
                AND encode(sha256(convert_to(pg_get_functiondef(t.tgfoid),'UTF8')),'hex')=$3)))
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, blueGreenSpecialTableLimit, blueGreenGroupTriggerHash).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿246/247既有表超出16MiB或类型、RLS、规则、触发器不符：%s", table)
		}
	}
	var clean bool
	err = tx.QueryRowContext(ctx, `SELECT
        NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
        AND NOT EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^scoped_admin_api_keys($|_)')
        AND NOT EXISTS(SELECT 1 FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?scoped_admin_api_keys($|_)')
        AND NOT EXISTS(SELECT 1 FROM public.schema_migrations WHERE filename IN ($1,$2))`,
		blueGreen260Migrations[0].Filename, blueGreen260Migrations[1].Filename).Scan(&clean)
	if err != nil || !clean {
		return fmt.Errorf("蓝绿246/247发现同名对象、已有账本或DDL触发器，拒绝覆盖")
	}
	for _, content := range contents {
		if _, err := tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("蓝绿246/247原始迁移失败，整体回滚：%w", err)
		}
	}
	if err := verifyBlueGreen260Schema(ctx, tx); err != nil {
		return err
	}
	for _, entry := range blueGreen260Migrations {
		if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func verifyBlueGreen260Schema(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path=public"); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
        AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
        AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
        AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
        AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=c.oid AND NOT tgisinternal)
        FROM pg_class c WHERE c.oid=to_regclass('public.scoped_admin_api_keys')`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿246/247管理密钥表类型、RLS、规则或触发器不符")
	}
	type column struct {
		name, dataType string
		notNull        bool
		defaultValue   string
	}
	expected := []column{
		{"id", "character varying(24)", true, ""}, {"name", "character varying(100)", true, ""},
		{"description", "text", true, "''::text"}, {"key_prefix", "character varying(64)", true, ""},
		{"key_hash", "character varying(64)", true, ""}, {"enabled", "boolean", true, "true"},
		{"expires_at", "timestamp with time zone", false, ""}, {"allowed_ips", "jsonb", true, "'[]'::jsonb"},
		{"policy", "jsonb", true, "'{}'::jsonb"}, {"created_by", "bigint", true, ""},
		{"created_at", "timestamp with time zone", true, "now()"}, {"updated_at", "timestamp with time zone", true, "now()"},
		{"last_used_at", "timestamp with time zone", false, ""}, {"last_used_ip", "character varying(64)", true, "''::character varying"},
		{"revoked_at", "timestamp with time zone", false, ""}, {"deleted_at", "timestamp with time zone", false, ""},
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,
        coalesce(pg_get_expr(d.adbin,d.adrelid),''),a.attidentity<>'' OR a.attgenerated<>''
        FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
        WHERE a.attrelid='public.scoped_admin_api_keys'::regclass AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`)
	if err != nil {
		return err
	}
	index := 0
	for rows.Next() {
		var actual column
		var generated bool
		err = rows.Scan(&actual.name, &actual.dataType, &actual.notNull, &actual.defaultValue, &generated)
		if err != nil || index >= len(expected) || actual != expected[index] || generated {
			_ = rows.Close()
			return fmt.Errorf("蓝绿246/247管理密钥字段定义漂移")
		}
		index++
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil || index != len(expected) {
		return fmt.Errorf("蓝绿246/247管理密钥字段清单不完整")
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*)=1 AND bool_and(i.indisvalid AND i.indisready
        AND pg_get_indexdef(i.indexrelid)='CREATE UNIQUE INDEX scoped_admin_api_keys_pkey ON public.scoped_admin_api_keys USING btree (id)')
        FROM pg_index i WHERE i.indrelid='public.scoped_admin_api_keys'::regclass`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿246/247管理密钥索引定义漂移")
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*)=2 AND bool_and(convalidated AND NOT condeferrable AND NOT condeferred AND (
        (conname='scoped_admin_api_keys_pkey' AND contype='p' AND pg_get_constraintdef(oid)='PRIMARY KEY (id)') OR
        (conname='scoped_admin_api_keys_created_by_fkey' AND contype='f' AND pg_get_constraintdef(oid)='FOREIGN KEY (created_by) REFERENCES users(id)')))
        FROM pg_constraint WHERE conrelid='public.scoped_admin_api_keys'::regclass AND contype<>'n'`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿246/247管理密钥主键或外键定义漂移")
	}
	return nil
}
