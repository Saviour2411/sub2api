package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

const blueGreen250Profile = "openai-cache-249-250-v1"

var blueGreen250Migrations = []blueGreenMigration{
	{Filename: "249_add_usage_logs_api_key_cost_index_notx.sql", Checksum: "94f124c3e50405f7b75746885111a4f662c331985a697de46fe946a17ff1f7d9", Profile: blueGreen250Profile},
	{Filename: "250_openai_cache_write_ttl.sql", Checksum: "532551ce7aee86b2ee0a78d397d422424ceed4051e31d5f542277c80c21b8b98", Profile: blueGreen250Profile},
}

var blueGreen250PricingTables = []string{
	"channel_model_pricing", "channel_pricing_intervals",
	"channel_account_stats_model_pricing", "channel_account_stats_pricing_intervals",
}

const blueGreen250IndexDefinition = "CREATE INDEX idx_usage_logs_api_key_created_cost ON public.usage_logs USING btree (api_key_id, created_at) INCLUDE (actual_cost)"

func validateBlueGreen250Policy(policy *blueGreenMigrationPolicy) error {
	for _, entry := range policy.Migrations {
		if entry.Profile != blueGreen250Profile {
			continue
		}
		if len(policy.Migrations) != len(blueGreen250Migrations) {
			return fmt.Errorf("蓝绿249/250必须同时包含两条固定迁移")
		}
		for _, expected := range blueGreen250Migrations {
			if actual, ok := policy.find(expected.Filename); !ok || actual != expected {
				return fmt.Errorf("蓝绿249/250迁移策略不完整或发生漂移")
			}
		}
	}
	return nil
}

func blueGreen250Transaction(ctx context.Context, conn *sql.Conn) (*sql.Tx, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, statement := range []string{"SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'", "SET LOCAL transaction_timeout='10s'", "SET LOCAL search_path=public"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	return tx, nil
}

func validateBlueGreen250Tables(ctx context.Context, tx *sql.Tx, limit bool) error {
	var safe bool
	if err := tx.QueryRowContext(ctx, `SELECT current_setting('server_version_num')::int/10000=18
        AND NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')`).Scan(&safe); err != nil || !safe {
		return fmt.Errorf("蓝绿249/250仅允许PostgreSQL18且不能存在DDL触发器")
	}
	for _, table := range append([]string{"usage_logs", "schema_migrations"}, blueGreen250PricingTables...) {
		// 仅固定用量表允许大表元数据扩展；其他既有表继续使用16MiB预算。
		budget := int64(blueGreenSpecialTableLimit)
		if table == "usage_logs" {
			budget = 64 * 1024 * 1024 * 1024
		}
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
            AND (NOT $3 OR pg_total_relation_size(c.oid)<=$2)
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE tgrelid=c.oid AND NOT tgisinternal AND NOT (
                c.relname='usage_logs' AND tgenabled='O' AND (
                    (pg_get_triggerdef(t.oid)='CREATE TRIGGER usage_logs_group_rollup_invalidate_insert AFTER INSERT ON public.usage_logs REFERENCING NEW TABLE AS inserted_usage_logs FOR EACH STATEMENT EXECUTE FUNCTION invalidate_group_usage_rollup_state_after_insert()'
                     AND encode(sha256(convert_to(pg_get_functiondef(tgfoid),'UTF8')),'hex')='b5ddf5ad94d92c0b29bb358e5316b992ce16231e80104663b0887ae44e83ab0a') OR
                    (pg_get_triggerdef(t.oid) IN (
                     'CREATE TRIGGER usage_logs_group_rollup_invalidate_delete AFTER DELETE ON public.usage_logs FOR EACH ROW WHEN ((old.group_id IS NOT NULL)) EXECUTE FUNCTION invalidate_group_usage_rollup_state()',
                     'CREATE TRIGGER usage_logs_group_rollup_invalidate_update AFTER UPDATE OF created_at, group_id, actual_cost ON public.usage_logs FOR EACH ROW WHEN ((((old.created_at IS DISTINCT FROM new.created_at) OR (old.group_id IS DISTINCT FROM new.group_id) OR (old.actual_cost IS DISTINCT FROM new.actual_cost)) AND ((old.group_id IS NOT NULL) OR (new.group_id IS NOT NULL)))) EXECUTE FUNCTION invalidate_group_usage_rollup_state()')
                     AND encode(sha256(convert_to(pg_get_functiondef(tgfoid),'UTF8')),'hex')='1d93592c296bd1811acf0e76e13558c0a30ef7d521343cf510b82e6eed5cd30d'))))
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, budget, limit).Scan(&safe)
		if err != nil || !safe {
			return fmt.Errorf("蓝绿249/250既有表大小、类型、RLS、规则或触发器不符：%s", table)
		}
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(a.attnum)=3 AND bool_and(a.attnotnull AND format_type(a.atttypid,a.atttypmod)=e.typ)
        FROM (VALUES ('api_key_id','bigint'),('created_at','timestamp with time zone'),('actual_cost','numeric(20,10)')) e(name,typ)
        LEFT JOIN pg_attribute a ON a.attrelid='public.usage_logs'::regclass AND a.attname=e.name AND NOT a.attisdropped`).Scan(&safe); err != nil || !safe {
		return fmt.Errorf("蓝绿249用量索引的既有字段定义漂移")
	}
	return nil
}

func blueGreen250IndexState(ctx context.Context, conn *sql.Conn) (bool, bool, error) {
	var definition string
	var valid, safe bool
	err := conn.QueryRowContext(ctx, `SELECT pg_get_indexdef(i.indexrelid),i.indisvalid AND i.indisready,
        i.indislive AND NOT i.indisunique AND NOT i.indisprimary AND NOT i.indisexclusion
        AND i.indrelid='public.usage_logs'::regclass AND c.relkind='i' AND c.relpersistence='p'
        AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conindid=i.indexrelid)
        FROM pg_class c LEFT JOIN pg_index i ON i.indexrelid=c.oid
        WHERE c.oid=to_regclass('public.idx_usage_logs_api_key_created_cost')`).Scan(&definition, &valid, &safe)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil || !safe || definition != blueGreen250IndexDefinition {
		return false, false, fmt.Errorf("蓝绿249并发索引存在同名对象或定义漂移")
	}
	return true, valid, nil
}

func applyBlueGreen250Index(ctx context.Context, conn *sql.Conn, content string) error {
	ctx, cancel := context.WithTimeout(ctx, 140*time.Second)
	defer cancel()
	tx, err := blueGreen250Transaction(ctx, conn)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := validateBlueGreen250Tables(ctx, tx, true); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "LOCK TABLE ONLY public.usage_logs IN SHARE UPDATE EXCLUSIVE MODE NOWAIT"); err != nil {
		return fmt.Errorf("蓝绿249无法立即获锁：%w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// CONCURRENTLY不能放在事务内；保存会话设置，并在所有退出路径恢复。
	settings := map[string]string{}
	for _, name := range []string{"lock_timeout", "statement_timeout", "search_path"} {
		var value string
		if err := conn.QueryRowContext(ctx, "SELECT current_setting($1)", name).Scan(&value); err != nil {
			return err
		}
		settings[name] = value
	}
	defer func() {
		restore, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		for name, value := range settings {
			if _, err := conn.ExecContext(restore, "SELECT set_config($1,$2,false)", name, value); err != nil {
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
				return
			}
		}
	}()
	for name, value := range map[string]string{"lock_timeout": "1s", "statement_timeout": "120s", "search_path": "public"} {
		if _, err := conn.ExecContext(ctx, "SELECT set_config($1,$2,false)", name, value); err != nil {
			return err
		}
	}
	exists, valid, err := blueGreen250IndexState(ctx, conn)
	if err != nil {
		return err
	}
	if exists && !valid {
		// 只清理与固定定义完全一致的失败构建，不触碰其他索引。
		if _, err := conn.ExecContext(ctx, "DROP INDEX CONCURRENTLY IF EXISTS public.idx_usage_logs_api_key_created_cost"); err != nil {
			return fmt.Errorf("蓝绿249无效索引清理失败：%w", err)
		}
	}
	if !exists || !valid {
		if _, err := conn.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("蓝绿249并发索引构建失败，旧实例继续服务：%w", err)
		}
	}
	if exists, valid, err = blueGreen250IndexState(ctx, conn); err != nil || !exists || !valid {
		return fmt.Errorf("蓝绿249并发索引未通过最终验证：%v", err)
	}
	_, err = conn.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", blueGreen250Migrations[0].Filename, blueGreen250Migrations[0].Checksum)
	return err
}

func verifyBlueGreen250Columns(ctx context.Context, tx *sql.Tx) error {
	for _, table := range append([]string{"usage_logs"}, blueGreen250PricingTables...) {
		columns := []string{"cache_write_12h_price"}
		if table == "usage_logs" {
			columns = []string{"cache_creation_30m_tokens", "cache_creation_12h_tokens"}
		}
		for _, column := range columns {
			var valid bool
			err := tx.QueryRowContext(ctx, `SELECT a.attidentity='' AND a.attgenerated=''
                AND NOT EXISTS(SELECT 1 FROM pg_constraint k WHERE k.conrelid=a.attrelid AND k.contype<>'n'
                    AND (k.conkey IS NULL OR a.attnum=ANY(k.conkey)) AND NOT (
                        k.conname='sub2api_bg250_price' AND k.contype='c' AND k.convalidated
                        AND pg_get_expr(k.conbin,k.conrelid)='(cache_write_12h_price IS NULL)')) AND (
                ($1='usage_logs' AND format_type(a.atttypid,a.atttypmod)='integer' AND a.attnotnull AND pg_get_expr(d.adbin,d.adrelid)='0') OR
                ($1<>'usage_logs' AND format_type(a.atttypid,a.atttypmod)='numeric(20,12)' AND NOT a.attnotnull AND NOT a.atthasdef))
                FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
                WHERE a.attrelid=to_regclass('public.'||$1) AND a.attname=$2 AND NOT a.attisdropped`, table, column).Scan(&valid)
			if err != nil || !valid {
				return fmt.Errorf("蓝绿250字段类型、默认值或可空性漂移：%s.%s", table, column)
			}
		}
	}
	var count, valid int
	err := tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE contype='c' AND convalidated
        AND pg_get_expr(conbin,conrelid)='(cache_write_12h_price IS NULL)') FROM pg_constraint
        WHERE conname='sub2api_bg250_price' AND conrelid IN ('public.channel_model_pricing'::regclass,
        'public.channel_pricing_intervals'::regclass,'public.channel_account_stats_model_pricing'::regclass,
        'public.channel_account_stats_pricing_intervals'::regclass)`).Scan(&count, &valid)
	if err != nil || (count != 0 && (count != 4 || valid != 4)) {
		return fmt.Errorf("蓝绿250定价兼容保护发生部分解除或定义漂移")
	}
	return nil
}

func applyBlueGreen250Columns(ctx context.Context, conn *sql.Conn, content string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := blueGreen250Transaction(ctx, conn)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range append([]string{"usage_logs", "schema_migrations"}, blueGreen250PricingTables...) {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("LOCK TABLE ONLY public.%s IN ACCESS EXCLUSIVE MODE NOWAIT", table)); err != nil {
			return fmt.Errorf("蓝绿250无法立即获锁：%w", err)
		}
	}
	if err := validateBlueGreen250Tables(ctx, tx, true); err != nil {
		return err
	}
	var clean bool
	err = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_attribute WHERE NOT attisdropped AND (
        (attrelid='public.usage_logs'::regclass AND attname IN ('cache_creation_30m_tokens','cache_creation_12h_tokens')) OR
        (attrelid IN ('public.channel_model_pricing'::regclass,'public.channel_pricing_intervals'::regclass,
            'public.channel_account_stats_model_pricing'::regclass,'public.channel_account_stats_pricing_intervals'::regclass)
         AND attname='cache_write_12h_price')))
        AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='sub2api_bg250_price')
        AND EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1 AND checksum=$2)`, blueGreen250Migrations[0].Filename, blueGreen250Migrations[0].Checksum).Scan(&clean)
	if err != nil || !clean {
		return fmt.Errorf("蓝绿250存在同名字段、保护或缺少固定249账本")
	}
	var before, after int64
	if err := tx.QueryRowContext(ctx, "SELECT relfilenode FROM pg_class WHERE oid='public.usage_logs'::regclass").Scan(&before); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("蓝绿250原始迁移失败，整体回滚：%w", err)
	}
	// 旧管理端整表替换定价会丢失12h新字段，双版本并存期间禁止启用该覆盖价。
	for _, table := range blueGreen250PricingTables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE public.%s ADD CONSTRAINT sub2api_bg250_price CHECK(cache_write_12h_price IS NULL)", table)); err != nil {
			return err
		}
	}
	if err := tx.QueryRowContext(ctx, "SELECT relfilenode FROM pg_class WHERE oid='public.usage_logs'::regclass").Scan(&after); err != nil || before != after {
		return fmt.Errorf("蓝绿250用量表发生重写，拒绝提交")
	}
	if err := verifyBlueGreen250Columns(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", blueGreen250Migrations[1].Filename, blueGreen250Migrations[1].Checksum); err != nil {
		return err
	}
	return tx.Commit()
}

func validateBlueGreen250Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen250Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen250Migrations[0].Filename); !ok {
		return nil
	}
	index, columns := applied[blueGreen250Migrations[0].Filename], applied[blueGreen250Migrations[1].Filename]
	if index == "" && columns == "" {
		return nil
	}
	if index != blueGreen250Migrations[0].Checksum || (columns != "" && columns != blueGreen250Migrations[1].Checksum) {
		return fmt.Errorf("蓝绿249/250账本顺序或摘要不符")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if exists, valid, err := blueGreen250IndexState(ctx, conn); err != nil || !exists || !valid {
		return fmt.Errorf("蓝绿249已登记索引缺失或漂移：%v", err)
	}
	if columns == "" {
		return nil
	}
	tx, err := blueGreen250Transaction(ctx, conn)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := validateBlueGreen250Tables(ctx, tx, false); err != nil {
		return err
	}
	if err := verifyBlueGreen250Columns(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
