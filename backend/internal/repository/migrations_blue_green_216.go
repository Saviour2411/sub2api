package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
)

const blueGreen216Profile = "upstream216-additive-v1"

var blueGreen216Migrations = []blueGreenMigration{
	{Filename: "242_group_i18n.sql", Checksum: "e81c6818cc806bc54365c993f4cec1634f16a311451faf5cb1a875a1358774cd", Profile: blueGreen216Profile},
	{Filename: "243_gemini_cached_contents.sql", Checksum: "2d345cff5c64b01e705f04ec972e5d5ec3d618fe71dc7d45bc80fdd38c6a89d5", Profile: blueGreen216Profile},
	{Filename: "244_gemini_cached_contents_channel_usage.sql", Checksum: "9072cb37752a23c3a29cc0330a69837f09c5102375ee053d7bccf14634d3da29", Profile: blueGreen216Profile},
	{Filename: "245_gemini_cached_contents_mutation_lease.sql", Checksum: "75434a7beeb8aad09b3339708ed520604d153e99c7fe680a376e8c958aadb1e8", Profile: blueGreen216Profile},
}

func validateBlueGreen216Policy(policy *blueGreenMigrationPolicy) error {
	count := 0
	for _, entry := range policy.Migrations {
		if entry.Profile == blueGreen216Profile {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != len(blueGreen216Migrations) || count != len(policy.Migrations) {
		return fmt.Errorf("蓝绿0.2.16必须包含完整且唯一的四条迁移")
	}
	for _, expected := range blueGreen216Migrations {
		entry, ok := policy.find(expected.Filename)
		if !ok || entry != expected {
			return fmt.Errorf("蓝绿0.2.16迁移清单不符")
		}
	}
	return nil
}

func validateBlueGreen216Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen216Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen216Migrations[0].Filename); !ok {
		return nil
	}
	count := 0
	for _, entry := range blueGreen216Migrations {
		if digest, ok := applied[entry.Filename]; ok {
			if digest != entry.Checksum {
				return fmt.Errorf("蓝绿0.2.16迁移账本摘要不符")
			}
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != len(blueGreen216Migrations) {
		return fmt.Errorf("蓝绿0.2.16迁移账本不完整，禁止自动补写")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := verifyBlueGreen216Schema(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func applyBlueGreen216(ctx context.Context, conn *sql.Conn, first blueGreenMigration) error {
	if first != blueGreen216Migrations[0] {
		return fmt.Errorf("蓝绿0.2.16只能从分组迁移开始完整事务")
	}
	contents := make([]string, len(blueGreen216Migrations))
	for i, entry := range blueGreen216Migrations {
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
	// public 是新表创建目标；内置 pg_catalog 仍由 PostgreSQL 隐式优先查找。
	for _, statement := range []string{"SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'", "SET LOCAL search_path=public"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version/10000 != 18 {
		return fmt.Errorf("蓝绿0.2.16固定迁移仅支持 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "LOCK TABLE ONLY public.groups IN ACCESS EXCLUSIVE MODE NOWAIT"); err != nil {
		return fmt.Errorf("蓝绿0.2.16无法立即锁定分组表，保留旧实例：%w", err)
	}
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
        AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND pg_total_relation_size(c.oid)<=$1
        AND NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
        AND NOT EXISTS (SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
        AND NOT EXISTS (SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
        AND NOT EXISTS (SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal AND NOT (
            t.tgenabled='O' AND pg_get_triggerdef(t.oid)='CREATE TRIGGER trg_groups_auth_cache_invalidation AFTER DELETE OR UPDATE ON public.groups FOR EACH ROW EXECUTE FUNCTION enqueue_group_auth_cache_invalidation()'
            AND encode(sha256(convert_to(pg_get_functiondef(t.tgfoid),'UTF8')),'hex')=$2))
        FROM pg_class c WHERE c.oid='public.groups'::regclass`, blueGreenSpecialTableLimit, blueGreenGroupTriggerHash).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿0.2.16分组表超出16MiB或表类型、触发器、RLS、规则不符")
	}
	err = tx.QueryRowContext(ctx, `SELECT
        NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='public.groups'::regclass AND attname='i18n' AND NOT attisdropped)
        AND NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname='public' AND c.relname IN ('gemini_cached_contents','gemini_cached_contents_id_seq',
            'gemini_cached_contents_pkey','idx_gemini_cached_contents_public_id','idx_gemini_cached_contents_api_key_active',
            'idx_gemini_cached_contents_account','idx_gemini_cached_contents_pending_mutation'))
        AND to_regtype('public.gemini_cached_contents') IS NULL
        AND NOT EXISTS (SELECT 1 FROM public.schema_migrations WHERE filename IN ($1,$2,$3,$4))`,
		blueGreen216Migrations[0].Filename, blueGreen216Migrations[1].Filename,
		blueGreen216Migrations[2].Filename, blueGreen216Migrations[3].Filename).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿0.2.16发现同名结构或已有账本，禁止通过IF NOT EXISTS掩盖差异")
	}
	var before, after int64
	if err := tx.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.groups')").Scan(&before); err != nil {
		return err
	}
	// 四条上游原文与四条账本同一事务提交，失败不留下未完成的新缓存表。
	for _, content := range contents {
		if _, err := tx.ExecContext(ctx, content); err != nil {
			return fmt.Errorf("蓝绿0.2.16原始迁移失败，整体回滚：%w", err)
		}
	}
	if err := tx.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.groups')").Scan(&after); err != nil || before != after {
		return fmt.Errorf("蓝绿0.2.16分组表发生重写，拒绝提交")
	}
	if err := verifyBlueGreen216Schema(ctx, tx); err != nil {
		return err
	}
	for _, entry := range blueGreen216Migrations {
		if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func verifyBlueGreen216Schema(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path=public"); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT a.atttypid='jsonb'::regtype AND a.attnotnull
        AND a.attidentity='' AND a.attgenerated='' AND pg_get_expr(d.adbin,d.adrelid)='''{}''::jsonb'
        FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
        WHERE a.attrelid='public.groups'::regclass AND a.attname='i18n' AND NOT a.attisdropped`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿0.2.16分组i18n定义缺失或漂移")
	}
	err = tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
        AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
        AND NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
        AND NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid=c.oid)
        AND NOT EXISTS (SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
        AND pg_get_serial_sequence('public.gemini_cached_contents','id')='public.gemini_cached_contents_id_seq'
        FROM pg_class c WHERE c.oid=to_regclass('public.gemini_cached_contents')`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿0.2.16缓存表、序列或附加行为不符")
	}
	type column struct {
		name, dataType string
		notNull        bool
		defaultValue   string
	}
	expected := []column{
		{"id", "bigint", true, "nextval('gemini_cached_contents_id_seq'::regclass)"},
		{"public_id", "character varying(64)", true, ""},
		{"user_id", "bigint", true, ""}, {"api_key_id", "bigint", true, ""},
		{"group_id", "bigint", true, ""}, {"account_id", "bigint", true, ""},
		{"upstream_name", "character varying(512)", true, ""},
		{"model", "character varying(255)", true, ""}, {"upstream_model", "character varying(512)", true, ""},
		{"display_name", "character varying(512)", true, "''::character varying"},
		{"total_token_count", "bigint", true, "0"}, {"expire_time", "timestamp with time zone", true, ""},
		{"created_at", "timestamp with time zone", true, "now()"}, {"updated_at", "timestamp with time zone", true, "now()"},
		{"deleted_at", "timestamp with time zone", false, ""},
		{"channel_usage", "jsonb", true, "'{}'::jsonb"},
		{"mutation_token", "character varying(64)", false, ""}, {"mutation_lease_until", "timestamp with time zone", false, ""},
	}
	rows, err := tx.QueryContext(ctx, `SELECT a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,
        coalesce(pg_get_expr(d.adbin,d.adrelid),''),a.attidentity<>'' OR a.attgenerated<>''
        FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
        WHERE a.attrelid='public.gemini_cached_contents'::regclass AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`)
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
			return fmt.Errorf("蓝绿0.2.16缓存字段定义漂移")
		}
		index++
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil || index != len(expected) {
		return fmt.Errorf("蓝绿0.2.16缓存字段清单不完整")
	}
	indexes := map[string]string{
		"gemini_cached_contents_pkey":                 "CREATE UNIQUE INDEX gemini_cached_contents_pkey ON public.gemini_cached_contents USING btree (id)",
		"idx_gemini_cached_contents_public_id":        "CREATE UNIQUE INDEX idx_gemini_cached_contents_public_id ON public.gemini_cached_contents USING btree (public_id)",
		"idx_gemini_cached_contents_api_key_active":   "CREATE INDEX idx_gemini_cached_contents_api_key_active ON public.gemini_cached_contents USING btree (api_key_id, id DESC) WHERE (deleted_at IS NULL)",
		"idx_gemini_cached_contents_account":          "CREATE INDEX idx_gemini_cached_contents_account ON public.gemini_cached_contents USING btree (account_id)",
		"idx_gemini_cached_contents_pending_mutation": "CREATE INDEX idx_gemini_cached_contents_pending_mutation ON public.gemini_cached_contents USING btree (mutation_lease_until) WHERE ((mutation_token IS NOT NULL) AND (deleted_at IS NULL))",
	}
	rows, err = tx.QueryContext(ctx, `SELECT c.relname,pg_get_indexdef(i.indexrelid),i.indisvalid AND i.indisready
        FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='public.gemini_cached_contents'::regclass`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, definition string
		err = rows.Scan(&name, &definition, &valid)
		if err != nil || !valid || indexes[name] != definition {
			_ = rows.Close()
			return fmt.Errorf("蓝绿0.2.16缓存索引定义漂移")
		}
		delete(indexes, name)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil || len(indexes) != 0 {
		return fmt.Errorf("蓝绿0.2.16缓存索引清单不完整")
	}
	err = tx.QueryRowContext(ctx, `SELECT count(*)=1 AND bool_and(contype='p' AND convalidated
        AND conname='gemini_cached_contents_pkey' AND pg_get_constraintdef(oid)='PRIMARY KEY (id)')
        FROM pg_constraint WHERE conrelid='public.gemini_cached_contents'::regclass AND contype<>'n'`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿0.2.16缓存约束定义漂移")
	}
	return nil
}
