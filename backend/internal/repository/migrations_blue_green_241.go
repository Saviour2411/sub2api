package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	blueGreenBonus241Profile     = "bonus-existing-241-v1"
	blueGreenBonus241File        = "241_add_payment_order_bonus_amount.sql"
	blueGreenBonus241Checksum    = "18b6a524a9873d3e4a45e6f9bd03b6e69f4384b701825c659817e017f2e8244f"
	blueGreenPlatform241Profile  = "platform-guarded-241-v1"
	blueGreenPlatform241File     = "241_add_typesafe_platform.sql"
	blueGreenPlatform241Checksum = "b6559525bf8d0b5d7c617e7415944f0d1fae8c408e9131ace84139795f66dcd2"
	blueGreenBonus149Checksum    = "7c8a322e0c7296e5226229b355a92274e7ab6165dea560d4cc3d0adcb0303278"
	blueGreenPlatform238Checksum = "6f987e251519bd3759e60da44620a5d777494cceb333b6ce394aa0ea536ef5a2"
)

var blueGreen241Platforms = []struct{ table, column, constraint, guard string }{
	{"user_platform_quotas", "platform", "user_platform_quotas_platform_check", "sub2api_bg241_quota"},
	{"composite_model_routes", "target_platform", "composite_model_routes_target_platform_check", "sub2api_bg241_route"},
}

func blueGreen241PlatformDefinition(column string, expanded bool) string {
	values := []string{"anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go"}
	if expanded {
		values = append(values, "typesafe")
	}
	for i, value := range values {
		values[i] = "'" + value + "'::character varying"
	}
	return fmt.Sprintf("((%s)::text = ANY ((ARRAY[%s])::text[]))", column, strings.Join(values, ", "))
}

func blueGreen241Ledger(ctx context.Context, tx *sql.Tx, filename, checksum string) error {
	var valid bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename=$1 AND checksum=$2)", filename, checksum).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿241历史迁移账本不符：%s", filename)
	}
	return nil
}

func blueGreen241BonusColumn(ctx context.Context, tx *sql.Tx) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT a.atttypid='numeric'::regtype AND format_type(a.atttypid,a.atttypmod)='numeric(20,2)'
        AND a.attnotnull AND a.attidentity='' AND a.attgenerated='' AND pg_get_expr(d.adbin,d.adrelid)='0'
        FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
        WHERE a.attrelid='public.payment_orders'::regclass AND a.attname='bonus_amount' AND NOT a.attisdropped`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿241赠金列缺失或定义漂移，禁止新增、回填或修正生产列")
	}
	return nil
}

func blueGreen241Constraint(ctx context.Context, tx *sql.Tx, table, name, definition string) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT count(*)=1 AND bool_and(contype='c' AND convalidated AND conislocal
        AND coninhcount=0 AND NOT connoinherit AND pg_get_expr(conbin,conrelid)=$3)
        FROM pg_constraint WHERE conrelid=to_regclass('public.' || $1) AND conname=$2`, table, name, definition).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿241约束缺失或定义漂移：%s", name)
	}
	return nil
}

func applyBlueGreen241(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{"SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'", "SET LOCAL search_path=pg_catalog,public"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version/10000 != 18 {
		return fmt.Errorf("蓝绿241专项迁移只批准 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	tables := []string{"payment_orders"}
	if entry.Profile == blueGreenPlatform241Profile {
		tables = []string{"user_platform_quotas", "composite_model_routes"}
	}
	// 在任何DDL之前一次性获得全部表锁；不能持锁等待业务查询结束。
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`LOCK TABLE ONLY public."%s" IN ACCESS EXCLUSIVE MODE NOWAIT`, table)); err != nil {
			return fmt.Errorf("蓝绿241无法立即获锁，保留旧实例：%w", err)
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.reloftype=0 AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
            AND pg_total_relation_size(c.oid)<=$2
            AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhparent=c.oid OR i.inhrelid=c.oid)
            AND NOT EXISTS (SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal)
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname='public' AND c.relname=$1`, table, blueGreenSpecialTableLimit).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿241只批准无自定义触发器及RLS、至多16MiB的普通表：%s", table)
		}
	}
	if entry.Profile == blueGreenBonus241Profile {
		if err := blueGreen241Ledger(ctx, tx, "149_payment_balance_bonus_rules.sql", blueGreenBonus149Checksum); err != nil {
			return err
		}
		if err := blueGreen241BonusColumn(ctx, tx); err != nil {
			return err
		}
	} else {
		if err := blueGreen241Ledger(ctx, tx, "238_opencode_go_platform.sql", blueGreenPlatform238Checksum); err != nil {
			return err
		}
		for _, target := range blueGreen241Platforms {
			if err := blueGreen241Constraint(ctx, tx, target.table, target.constraint, blueGreen241PlatformDefinition(target.column, false)); err != nil {
				return err
			}
			var exists bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('public.' || $1) AND conname=$2)", target.table, target.guard).Scan(&exists); err != nil || exists {
				return fmt.Errorf("蓝绿241发现未登记的同名保护约束：%s", target.guard)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("执行蓝绿241原始SQL失败：%w", err)
	}
	if entry.Profile == blueGreenBonus241Profile {
		if err := blueGreen241BonusColumn(ctx, tx); err != nil {
			return err
		}
	} else {
		for _, target := range blueGreen241Platforms {
			if err := blueGreen241Constraint(ctx, tx, target.table, target.constraint, blueGreen241PlatformDefinition(target.column, true)); err != nil {
				return err
			}
			statement := fmt.Sprintf(`ALTER TABLE public."%s" ADD CONSTRAINT "%s" CHECK (%s <> 'typesafe')`, target.table, target.guard, target.column)
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("安装蓝绿241旧版兼容保护失败：%w", err)
			}
			if err := blueGreen241Constraint(ctx, tx, target.table, target.guard, fmt.Sprintf("((%s)::text <> 'typesafe'::text)", target.column)); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename,checksum) VALUES ($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}
