package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
)

const blueGreen251Profile = "group-rollup-251-252-v1"

var blueGreen251Migrations = []blueGreenMigration{
	{Filename: "251_group_force_openai_ultrafast.sql", Checksum: "d78e605d4fc2038c57f620dff398022823a850ea8994b129b50774ac49e20af1", Profile: blueGreen251Profile},
	{Filename: "251_group_usage_rollup_nonblocking_build.sql", Checksum: "5b6e9b57f9b354a96c6870185c034db8adc7def944efcddf654e39cd97770718", Profile: blueGreen251Profile},
	{Filename: "252_group_usage_rollup_current_insert_nonblocking.sql", Checksum: "b9c27f1710131a16937b7c5a42fea3be51650b1412dd963c8938aca4af6d0e58", Profile: blueGreen251Profile},
}

var blueGreen251FunctionsBefore = map[string]string{
	"invalidate_group_usage_rollup_state":              "1d93592c296bd1811acf0e76e13558c0a30ef7d521343cf510b82e6eed5cd30d",
	"invalidate_group_usage_rollup_state_after_insert": "b5ddf5ad94d92c0b29bb358e5316b992ce16231e80104663b0887ae44e83ab0a",
}

var blueGreen251FunctionsAfter = map[string]string{
	"apply_group_usage_rollup_invalidation":            "4bb10d7ed3deee10136431aa31546d7461549b8764bfb96ec0b4859dd103dc91",
	"invalidate_group_usage_rollups_at":                "232c2cc8d2bbc26dc954a989b42dcd854c9ee971e77d7b1d11f2e7e79ba07385",
	"invalidate_group_usage_rollup_state":              "ef49292071e8cbfb8dd079c0dfc6e2504edc16cc63418fcdbf2719353c18e345",
	"invalidate_group_usage_rollup_state_after_insert": "35cbd219ac6e55cbc8a16388faf5caf3d71702ff210e52090d37194490339b54",
}

func validateBlueGreen251Policy(policy *blueGreenMigrationPolicy) error {
	for _, entry := range policy.Migrations {
		if entry.Profile != blueGreen251Profile {
			continue
		}
		if len(policy.Migrations) != len(blueGreen251Migrations) {
			return fmt.Errorf("蓝绿251/252必须完整包含三条固定迁移")
		}
		for _, expected := range blueGreen251Migrations {
			if actual, ok := policy.find(expected.Filename); !ok || actual != expected {
				return fmt.Errorf("蓝绿251/252迁移策略不完整或发生漂移")
			}
		}
	}
	return nil
}

func validateBlueGreen251Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen251Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen251Migrations[0].Filename); !ok {
		return nil
	}
	count := 0
	for _, entry := range blueGreen251Migrations {
		if digest, ok := applied[entry.Filename]; ok {
			if digest != entry.Checksum {
				return fmt.Errorf("蓝绿251/252迁移账本摘要不符")
			}
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != len(blueGreen251Migrations) {
		return fmt.Errorf("蓝绿251/252账本不完整，禁止部分补写")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := blueGreen251Settings(ctx, tx); err != nil {
		return err
	}
	if err := verifyBlueGreen251Schema(ctx, tx, true, false); err != nil {
		return err
	}
	return tx.Commit()
}

func blueGreen251Settings(ctx context.Context, tx *sql.Tx) error {
	var safe bool
	if err := tx.QueryRowContext(ctx, `SELECT current_setting('server_version_num')::int/10000=18
        AND NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')`).Scan(&safe); err != nil || !safe {
		return fmt.Errorf("蓝绿251/252仅支持PostgreSQL18且不能存在DDL触发器")
	}
	for _, statement := range []string{"SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'",
		"SET LOCAL transaction_timeout='10s'", "SET LOCAL search_path=public", "SET LOCAL timezone='UTC'"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func verifyBlueGreen251Schema(ctx context.Context, tx *sql.Tx, upgraded, limit bool) error {
	functions := blueGreen251FunctionsBefore
	if upgraded {
		functions = blueGreen251FunctionsAfter
	}
	for _, table := range []string{"groups", "usage_logs", "usage_group_rollup_state", "usage_group_daily_rollups", "schema_migrations"} {
		budget := int64(blueGreenSpecialTableLimit)
		if table == "usage_logs" {
			budget = 64 * 1024 * 1024 * 1024
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
            AND (NOT $3 OR pg_total_relation_size(c.oid)<=$2)
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal AND NOT (
                t.tgenabled='O' AND (
                (c.relname='groups' AND pg_get_triggerdef(t.oid)='CREATE TRIGGER trg_groups_auth_cache_invalidation AFTER DELETE OR UPDATE ON public.groups FOR EACH ROW EXECUTE FUNCTION enqueue_group_auth_cache_invalidation()'
                    AND encode(sha256(convert_to(pg_get_functiondef(t.tgfoid),'UTF8')),'hex')=$4) OR
                (c.relname='usage_logs' AND pg_get_triggerdef(t.oid) IN (
                    'CREATE TRIGGER usage_logs_group_rollup_invalidate_insert AFTER INSERT ON public.usage_logs REFERENCING NEW TABLE AS inserted_usage_logs FOR EACH STATEMENT EXECUTE FUNCTION invalidate_group_usage_rollup_state_after_insert()',
                    'CREATE TRIGGER usage_logs_group_rollup_invalidate_delete AFTER DELETE ON public.usage_logs FOR EACH ROW WHEN ((old.group_id IS NOT NULL)) EXECUTE FUNCTION invalidate_group_usage_rollup_state()',
                    'CREATE TRIGGER usage_logs_group_rollup_invalidate_update AFTER UPDATE OF created_at, group_id, actual_cost ON public.usage_logs FOR EACH ROW WHEN ((((old.created_at IS DISTINCT FROM new.created_at) OR (old.group_id IS DISTINCT FROM new.group_id) OR (old.actual_cost IS DISTINCT FROM new.actual_cost)) AND ((old.group_id IS NOT NULL) OR (new.group_id IS NOT NULL)))) EXECUTE FUNCTION invalidate_group_usage_rollup_state()')))))
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, budget, limit, blueGreenGroupTriggerHash).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿251/252表大小、类型、RLS、规则或触发器不符：%s", table)
		}
	}
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT
        (SELECT count(*)=3 FROM pg_trigger WHERE tgrelid='public.usage_logs'::regclass AND NOT tgisinternal)
        AND (SELECT count(*)=1 AND bool_and(id=1) FROM public.usage_group_rollup_state)
        AND (SELECT count(*)=$1 FROM pg_proc WHERE pronamespace='public'::regnamespace AND proname IN
          ('apply_group_usage_rollup_invalidation','invalidate_group_usage_rollups_at',
           'invalidate_group_usage_rollup_state','invalidate_group_usage_rollup_state_after_insert'))`, len(functions)).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿251/252状态行、触发器或函数数量不符")
	}
	for name, hash := range functions {
		if err := tx.QueryRowContext(ctx, `SELECT count(*)=1 AND bool_and(
            encode(sha256(convert_to(pg_get_functiondef(oid),'UTF8')),'hex')=$2)
            FROM pg_proc WHERE pronamespace='public'::regnamespace AND proname=$1`, name, hash).Scan(&valid); err != nil || !valid {
			return fmt.Errorf("蓝绿251/252汇总函数定义漂移：%s", name)
		}
	}
	err = tx.QueryRowContext(ctx, `WITH expected(name,typ,nn,def) AS (VALUES
        ('id','smallint',true,NULL),('closed_before','date',true,'''1970-01-01''::date'),
        ('retained_from','timestamp with time zone',true,'''1970-01-01 00:00:00+00''::timestamp with time zone'),
        ('updated_at','timestamp with time zone',true,'now()'),('timezone_name','text',true,'''Asia/Shanghai''::text'),
        ('expired_before','timestamp with time zone',true,'''1970-01-01 00:00:00+00''::timestamp with time zone'),
        ('building_until','date',false,NULL),('build_dirty_from','date',false,NULL)),
        actual AS (SELECT a.attname,a.atttypid,a.atttypmod,a.attnotnull,a.attidentity,a.attgenerated,a.attnum,
            pg_get_expr(d.adbin,d.adrelid) def FROM pg_attribute a
            LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
            WHERE a.attrelid='public.usage_group_rollup_state'::regclass AND a.attnum>0 AND NOT a.attisdropped)
        SELECT count(*)=CASE WHEN $1 THEN 8 ELSE 5 END AND bool_and(e.name IS NOT NULL
            AND format_type(a.atttypid,a.atttypmod)=e.typ AND a.attnotnull=e.nn
            AND a.attidentity='' AND a.attgenerated='' AND a.def IS NOT DISTINCT FROM e.def)
        FROM actual a LEFT JOIN expected e ON a.attname=e.name
            AND ($1 OR e.name NOT IN ('expired_before','building_until','build_dirty_from'))`, upgraded).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿251/252汇总状态字段定义漂移")
	}
	if !upgraded {
		err = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_attribute
            WHERE attrelid='public.groups'::regclass AND attname='force_openai_ultrafast' AND NOT attisdropped)`).Scan(&valid)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT a.atttypid='boolean'::regtype AND a.attnotnull
            AND a.attidentity='' AND a.attgenerated='' AND pg_get_expr(d.adbin,d.adrelid)='false'
            AND NOT EXISTS(SELECT 1 FROM pg_constraint k WHERE k.conrelid=a.attrelid
                AND k.contype<>'n' AND (k.conkey IS NULL OR a.attnum=ANY(k.conkey)))
            FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
            WHERE a.attrelid='public.groups'::regclass AND a.attname='force_openai_ultrafast' AND NOT a.attisdropped`).Scan(&valid)
	}
	if err != nil || !valid {
		return fmt.Errorf("蓝绿251/252分组Ultrafast字段存在或定义漂移")
	}
	return nil
}

func applyBlueGreen251(ctx context.Context, conn *sql.Conn, first blueGreenMigration) error {
	if first != blueGreen251Migrations[0] {
		return fmt.Errorf("蓝绿251/252必须从第一条迁移开始完整事务")
	}
	contents := make([]string, len(blueGreen251Migrations))
	for i, entry := range blueGreen251Migrations {
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
	if err := blueGreen251Settings(ctx, tx); err != nil {
		return err
	}
	// 先排除正在执行旧触发器的写事务；各表锁均不排队，三条原SQL原子生效。
	for _, lock := range []struct{ table, mode string }{
		{"usage_logs", "SHARE ROW EXCLUSIVE"}, {"groups", "ACCESS EXCLUSIVE"},
		{"usage_group_rollup_state", "ACCESS EXCLUSIVE"}, {"usage_group_daily_rollups", "SHARE ROW EXCLUSIVE"},
		{"schema_migrations", "SHARE ROW EXCLUSIVE"},
	} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("LOCK TABLE ONLY public.%s IN %s MODE NOWAIT", lock.table, lock.mode)); err != nil {
			return fmt.Errorf("蓝绿251/252无法立即获锁，保留旧实例：%w", err)
		}
	}
	if err := verifyBlueGreen251Schema(ctx, tx, false, true); err != nil {
		return err
	}
	for _, name := range []string{"222_group_usage_daily_rollups.sql", "223_group_usage_rollup_timezone.sql", "226_group_usage_rollup_current_insert_nonblocking.sql"} {
		raw, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		var digest string
		if err := tx.QueryRowContext(ctx, "SELECT checksum FROM public.schema_migrations WHERE filename=$1", name).Scan(&digest); err != nil || digest != fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(string(raw))))) {
			return fmt.Errorf("蓝绿251/252缺少固定历史汇总账本：%s", name)
		}
	}
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM public.schema_migrations WHERE filename IN ($1,$2,$3)",
		blueGreen251Migrations[0].Filename, blueGreen251Migrations[1].Filename, blueGreen251Migrations[2].Filename).Scan(&count); err != nil || count != 0 {
		return fmt.Errorf("蓝绿251/252发现已有账本，拒绝重放")
	}
	var before, after string
	const filenodes = "SELECT jsonb_object_agg(relname,relfilenode)::text FROM pg_class WHERE oid IN ('public.groups'::regclass,'public.usage_group_rollup_state'::regclass,'public.usage_logs'::regclass)"
	if err := tx.QueryRowContext(ctx, filenodes).Scan(&before); err != nil {
		return err
	}
	for i, content := range contents {
		statements := []string{content}
		if i == 1 {
			// 固定摘要中的唯一超时语句单独执行；函数体和字符串中的分号不能拆开。
			const setting = "SET LOCAL lock_timeout = '5s';"
			prefix, remainder, found := strings.Cut(content, setting)
			if !found {
				return fmt.Errorf("蓝绿251汇总迁移缺少固定超时语句")
			}
			statements = []string{prefix + setting, remainder}
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("蓝绿251/252原始迁移失败，整体回滚：%w", err)
			}
			// 原251设置5秒lock_timeout；在下一条DDL前恢复专项的1秒预算。
			if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout='1s'"); err != nil {
				return err
			}
		}
	}
	if err := tx.QueryRowContext(ctx, filenodes).Scan(&after); err != nil || before != after {
		return fmt.Errorf("蓝绿251/252检测到表重写，拒绝提交")
	}
	if err := verifyBlueGreen251Schema(ctx, tx, true, true); err != nil {
		return err
	}
	for _, entry := range blueGreen251Migrations {
		if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}
