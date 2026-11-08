package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const blueGreen249Profile = "mcp-oauth-additive-249-v1"

var blueGreen249Migration = blueGreenMigration{
	Filename: "249_mcp_oauth.sql",
	Checksum: "751a1dcee6108afcecbd580b6e3140cca3480853fecd2174861b02f3a71e54a1",
	Profile:  blueGreen249Profile,
}

// 固定原始SQL在PostgreSQL18中的结构摘要，不含OAuth数据或数据库对象OID。
const blueGreen249SchemaChecksum = "7801b4680044d8f4c1cff3af1f81ad8a0c36683facbf6d7d1f9a059390d876e8"
const blueGreenUserTriggerHash = "8acfd67bab6e277289b904f0768c91e54249730adc272778e77061387780ca54"

const blueGreen249SchemaSQL = `WITH tables AS (
    SELECT c.* FROM pg_class c WHERE c.relnamespace='public'::regnamespace
    AND c.relname ~ '^mcp_oauth_' AND c.relkind IN ('r','p','v','m','f')
), relations AS (
    SELECT c.relname,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,c.reloftype<>0 AS typed,
        EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid) AS inherits,
        (SELECT count(*) FROM pg_rewrite WHERE ev_class=c.oid) AS rules,
        (SELECT count(*) FROM pg_policy WHERE polrelid=c.oid) AS policies,
        (SELECT coalesce(jsonb_agg(pg_get_triggerdef(t.oid) ORDER BY t.tgname COLLATE "C"),'[]'::jsonb)
            FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal) AS triggers,
        (SELECT coalesce(jsonb_agg(jsonb_build_object('type',t.tgtype,'enabled',t.tgenabled,'function',t.tgfoid::regproc::text)
            ORDER BY t.tgtype,t.tgfoid::regproc::text COLLATE "C"),'[]'::jsonb)
            FROM pg_trigger t WHERE t.tgrelid=c.oid AND t.tgisinternal) AS internal_triggers
    FROM tables c
), columns AS (
    SELECT c.relname,a.attnum,a.attname,format_type(a.atttypid,a.atttypmod) AS type,a.attnotnull,
        a.attidentity,a.attgenerated,coalesce(pg_get_expr(d.adbin,d.adrelid),'') AS default_value
    FROM tables c JOIN pg_attribute a ON a.attrelid=c.oid
    LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
    WHERE a.attnum>0 AND NOT a.attisdropped
), indexes AS (
    SELECT c.relname,ic.relname AS name,pg_get_indexdef(i.indexrelid) AS definition,i.indisvalid,i.indisready
    FROM tables c JOIN pg_index i ON i.indrelid=c.oid JOIN pg_class ic ON ic.oid=i.indexrelid
), constraints AS (
    SELECT c.relname,k.conname,k.contype,k.convalidated,k.condeferrable,k.condeferred,pg_get_constraintdef(k.oid) AS definition
    FROM tables c JOIN pg_constraint k ON k.conrelid=c.oid WHERE k.contype<>'n'
), objects AS (
    SELECT relname,relkind FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^(idx_)?mcp_oauth_'
), types AS (
    SELECT typname,typtype FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?(idx_)?mcp_oauth_'
)
SELECT jsonb_build_object('relations',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM relations r),
    'columns',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.attnum) FROM columns r),
    'indexes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.name COLLATE "C") FROM indexes r),
    'constraints',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.conname COLLATE "C") FROM constraints r),
    'objects',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM objects r),
    'types',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.typname COLLATE "C") FROM types r))::text`

func validateBlueGreen249Policy(policy *blueGreenMigrationPolicy) error {
	if entry, ok := policy.find(blueGreen249Migration.Filename); ok && (entry != blueGreen249Migration || len(policy.Migrations) != 1) {
		return fmt.Errorf("蓝绿249仅允许唯一的固定MCP OAuth迁移")
	}
	return nil
}

func verifyBlueGreen249Schema(ctx context.Context, tx *sql.Tx) error {
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version/10000 != 18 {
		return fmt.Errorf("蓝绿249固定迁移仅支持PostgreSQL18")
	}
	for _, statement := range []string{"SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'", "SET LOCAL transaction_timeout='10s'", "SET LOCAL search_path=public"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var definition string
	if err := tx.QueryRowContext(ctx, blueGreen249SchemaSQL).Scan(&definition); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(definition))
	if hex.EncodeToString(digest[:]) != blueGreen249SchemaChecksum {
		return fmt.Errorf("蓝绿249表、字段、索引、约束、类型或触发器定义漂移")
	}
	return nil
}

func validateBlueGreen249Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen249Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen249Migration.Filename); !ok {
		return nil
	}
	digest, exists := applied[blueGreen249Migration.Filename]
	if !exists {
		return nil
	}
	if digest != blueGreen249Migration.Checksum {
		return fmt.Errorf("蓝绿249迁移账本摘要不符")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := verifyBlueGreen249Schema(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func applyBlueGreen249(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	if entry != blueGreen249Migration {
		return fmt.Errorf("蓝绿249迁移条目不符")
	}
	if err := validateBlueGreenSQL(entry, content); err != nil {
		return err
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
		return fmt.Errorf("蓝绿249固定迁移仅支持PostgreSQL18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	// 外键需要短暂锁定users；NOWAIT避免排队阻塞旧版写入，普通读取可继续。
	for _, table := range []string{"users", "schema_migrations"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("LOCK TABLE ONLY public.%s IN SHARE ROW EXCLUSIVE MODE NOWAIT", table)); err != nil {
			return fmt.Errorf("蓝绿249无法立即获锁，保留旧实例：%w", err)
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND pg_total_relation_size(c.oid)<=$2
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal AND NOT (
                c.relname='users' AND t.tgenabled='O'
                AND pg_get_triggerdef(t.oid)='CREATE TRIGGER trg_users_auth_cache_invalidation AFTER DELETE OR UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION enqueue_user_auth_cache_invalidation()'
                AND encode(sha256(convert_to(pg_get_functiondef(t.tgfoid),'UTF8')),'hex')=$3))
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, blueGreenSpecialTableLimit, blueGreenUserTriggerHash).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿249既有表超出16MiB或类型、RLS、规则、触发器不符：%s", table)
		}
	}
	var clean bool
	err = tx.QueryRowContext(ctx, `SELECT
        NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
        AND NOT EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^(idx_)?mcp_oauth_')
        AND NOT EXISTS(SELECT 1 FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?(idx_)?mcp_oauth_')
        AND NOT EXISTS(SELECT 1 FROM public.schema_migrations WHERE filename=$1)`, entry.Filename).Scan(&clean)
	if err != nil || !clean {
		return fmt.Errorf("蓝绿249发现同名对象、已有账本或DDL触发器，拒绝覆盖")
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("蓝绿249原始迁移失败，整体回滚：%w", err)
	}
	if err := verifyBlueGreen249Schema(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}
