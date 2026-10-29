package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const blueGreen248Profile = "image-count-additive-248-v1"

var blueGreen248Migration = blueGreenMigration{
	Filename: "248_group_image_count_billing.sql",
	Checksum: "a872dde339b19f311feb5fa6fd85c8ff7b11d69794c35a18e2086d9846b3499c",
	Profile:  blueGreen248Profile,
}

func validateBlueGreen248Policy(policy *blueGreenMigrationPolicy) error {
	for _, entry := range policy.Migrations {
		if entry.Profile == blueGreen248Profile && (len(policy.Migrations) != 1 || entry != blueGreen248Migration) {
			return fmt.Errorf("蓝绿248仅允许唯一的固定图片计费迁移")
		}
	}
	return nil
}

func validateBlueGreen248Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen248Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen248Migration.Filename); !ok {
		return nil
	}
	digest, ok := applied[blueGreen248Migration.Filename]
	if !ok {
		return nil
	}
	if digest != blueGreen248Migration.Checksum {
		return fmt.Errorf("蓝绿248迁移账本摘要不符")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := verifyBlueGreen248Schema(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func applyBlueGreen248(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	if entry != blueGreen248Migration {
		return fmt.Errorf("蓝绿248迁移清单不符")
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
		return fmt.Errorf("蓝绿248固定迁移仅支持 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	// 字段与账本同事务提交；拿不到锁时立即退出，避免在长请求后排队。
	for _, table := range []string{"groups", "schema_migrations"} {
		mode := "SHARE ROW EXCLUSIVE"
		if table == "groups" {
			mode = "ACCESS EXCLUSIVE"
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("LOCK TABLE ONLY public.%s IN %s MODE NOWAIT", table, mode)); err != nil {
			return fmt.Errorf("蓝绿248无法立即获锁，保留旧实例：%w", err)
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND pg_total_relation_size(c.oid)<=$2
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal AND NOT (
                c.relname='groups' AND t.tgenabled='O'
                AND pg_get_triggerdef(t.oid)='CREATE TRIGGER trg_groups_auth_cache_invalidation AFTER DELETE OR UPDATE ON public.groups FOR EACH ROW EXECUTE FUNCTION enqueue_group_auth_cache_invalidation()'
                AND encode(sha256(convert_to(pg_get_functiondef(t.tgfoid),'UTF8')),'hex')=$3))
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, blueGreenSpecialTableLimit, blueGreenGroupTriggerHash).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿248既有表超出16MiB或类型、RLS、规则、触发器不符：%s", table)
		}
	}
	var clean bool
	err = tx.QueryRowContext(ctx, `SELECT
        NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
        AND NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='public.groups'::regclass AND attname='image_count_billing' AND NOT attisdropped)
        AND NOT EXISTS(SELECT 1 FROM public.schema_migrations WHERE filename=$1)`, entry.Filename).Scan(&clean)
	if err != nil || !clean {
		return fmt.Errorf("蓝绿248发现同名字段、已有账本或DDL触发器，拒绝覆盖")
	}
	var before, after int64
	if err := tx.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.groups')").Scan(&before); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("蓝绿248原始迁移失败，整体回滚：%w", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.groups')").Scan(&after); err != nil || before != after {
		return fmt.Errorf("蓝绿248分组表发生重写，拒绝提交")
	}
	if err := verifyBlueGreen248Schema(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}

func verifyBlueGreen248Schema(ctx context.Context, tx *sql.Tx) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
        AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
        AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
        AND a.atttypid='boolean'::regtype AND a.attnotnull AND a.attidentity='' AND a.attgenerated=''
        AND pg_get_expr(d.adbin,d.adrelid)='false'
        AND col_description(c.oid,a.attnum)='图片生成是否按实际出图张数计费'
        AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=c.oid AND a.attnum=ANY(conkey) AND contype<>'n')
        FROM pg_class c JOIN pg_attribute a ON a.attrelid=c.oid
        LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
        WHERE c.oid='public.groups'::regclass AND a.attname='image_count_billing' AND NOT a.attisdropped`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿248图片计费字段定义缺失或漂移")
	}
	return nil
}
