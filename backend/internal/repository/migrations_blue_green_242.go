package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	blueGreenPlatform242Profile  = "platform-guarded-242-v1"
	blueGreenPlatform242File     = "242_drop_platform_check_constraints.sql"
	blueGreenPlatform242Checksum = "a7725f31f637208d200e0f37d5270e9f263743d99fa9e4f1bb9a7387e8be5a61"
	blueGreenPlatform242Values   = "'anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe'"
)

var blueGreen242Platforms = []struct {
	table, column, constraint, guard, previousGuard string
	length                                          int
}{
	{"user_platform_quotas", "platform", "user_platform_quotas_platform_check", "sub2api_bg242_quota", "sub2api_bg241_quota", 32},
	{"composite_model_routes", "target_platform", "composite_model_routes_target_platform_check", "sub2api_bg242_route", "sub2api_bg241_route", 50},
}

func applyBlueGreen242(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
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
		return fmt.Errorf("蓝绿242专项迁移只批准 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	// 固定两表均立即获锁后才执行DDL，锁冲突必须释放已取得的锁并保留旧实例。
	for _, target := range blueGreen242Platforms {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`LOCK TABLE ONLY public."%s" IN ACCESS EXCLUSIVE MODE NOWAIT`, target.table)); err != nil {
			return fmt.Errorf("蓝绿242无法立即获锁，保留旧实例：%w", err)
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.reloftype=0 AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
            AND pg_total_relation_size(c.oid)<=$2
            AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhparent=c.oid OR i.inhrelid=c.oid)
            AND NOT EXISTS (SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal)
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname='public' AND c.relname=$1`, target.table, blueGreenSpecialTableLimit).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿242只批准无自定义触发器及RLS、至多16MiB的普通表：%s", target.table)
		}
		err = tx.QueryRowContext(ctx, `SELECT a.atttypid='varchar'::regtype AND format_type(a.atttypid,a.atttypmod)=$3
            AND a.attnotnull AND a.attidentity='' AND a.attgenerated=''
            FROM pg_attribute a WHERE a.attrelid=to_regclass('public.' || $1) AND a.attname=$2 AND NOT a.attisdropped`,
			target.table, target.column, fmt.Sprintf("character varying(%d)", target.length)).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿242平台列定义漂移：%s", target.table)
		}
		if err := blueGreen241Constraint(ctx, tx, target.table, target.constraint, blueGreen241PlatformDefinition(target.column, true)); err != nil {
			return err
		}
		var exists bool
		err = tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('public.' || $1) AND conname IN ($2,$3))", target.table, target.guard, target.previousGuard).Scan(&exists)
		if err != nil || exists {
			return fmt.Errorf("蓝绿242发现未解除的旧保护或同名约束：%s", target.table)
		}
	}
	for _, ledger := range []struct{ filename, checksum string }{
		{"238_opencode_go_platform.sql", blueGreenPlatform238Checksum},
		{blueGreenPlatform241File, blueGreenPlatform241Checksum},
	} {
		if err := blueGreen241Ledger(ctx, tx, ledger.filename, ledger.checksum); err != nil {
			return err
		}
	}
	// 先复制旧版十一种平台的精确定义，再执行原始242；同事务内不出现无保护的提交状态。
	for _, target := range blueGreen242Platforms {
		// 沿用原SQL的IN语法，不重解析pg_get_expr输出以免改变规范表达式。
		statement := fmt.Sprintf(`ALTER TABLE public."%s" ADD CONSTRAINT "%s" CHECK (%s IN (%s))`, target.table, target.guard, target.column, blueGreenPlatform242Values)
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("安装蓝绿242兼容保护失败：%w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("执行蓝绿242原始SQL失败：%w", err)
	}
	for _, target := range blueGreen242Platforms {
		if err := blueGreen241Constraint(ctx, tx, target.table, target.guard, blueGreen241PlatformDefinition(target.column, true)); err != nil {
			return err
		}
		var exists bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('public.' || $1) AND conname=$2)", target.table, target.constraint).Scan(&exists)
		if err != nil || exists {
			return fmt.Errorf("蓝绿242原约束未移除：%s", target.constraint)
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename,checksum) VALUES ($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}
