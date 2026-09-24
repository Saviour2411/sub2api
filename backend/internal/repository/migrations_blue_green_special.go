package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	blueGreenReasoningProfile  = "reasoning-empty-239-v1"
	blueGreenAffiliateProfile  = "affiliate-operation-240-v1"
	blueGreenReasoningFile     = "239_channel_reasoning_effort_multipliers.sql"
	blueGreenAffiliateFile     = "240_affiliate_ledger_operation_id.sql"
	blueGreenReasoningChecksum = "66feb546785dfa385d8efd268a40276cd8ffdb0cead7026cd79e339a3b69edf1"
	blueGreenAffiliateChecksum = "3823bfea5f64feb58f5fcebc341c6877eaefc97834b4cd652c8e83ad08ed78da"
	blueGreenSpecialTableLimit = 16 * 1024 * 1024
)

// 旧版把 nil 倍率序列化为 JSON null；允许它继续保存其他价格，但不能新增倍率。
const blueGreenGroupPricingEmpty = `(model_pricing IS NULL OR model_pricing = 'null'::jsonb OR
    (jsonb_typeof(model_pricing) = 'array'
     AND jsonb_path_query_array(model_pricing, '$[*].max_reasoning_effort_multiplier') <@ '[null]'::jsonb
     AND jsonb_path_query_array(model_pricing, '$[*].reasoning_effort_multipliers') <@ '[{}]'::jsonb))`

func approvedBlueGreenSpecial(entry blueGreenMigration) bool {
	if entry.Table != "" || entry.Column != "" || entry.DataType != "" {
		return false
	}
	switch entry.Profile {
	case blueGreenReasoningProfile:
		return entry.Filename == blueGreenReasoningFile && entry.Checksum == blueGreenReasoningChecksum
	case blueGreenAffiliateProfile:
		return entry.Filename == blueGreenAffiliateFile && entry.Checksum == blueGreenAffiliateChecksum
	default:
		return false
	}
}

func applyBlueGreenSpecial(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{"SET LOCAL lock_timeout = '1s'", "SET LOCAL statement_timeout = '5s'", "SET LOCAL search_path = pg_catalog, public"} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version/10000 != 18 {
		return fmt.Errorf("蓝绿239/240专项迁移只批准已验证的 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout = '10s'"); err != nil {
		return err
	}
	tables := []string{"user_affiliate_ledger"}
	if entry.Profile == blueGreenReasoningProfile {
		tables = []string{"channel_model_pricing", "channel_account_stats_model_pricing", "groups"}
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`LOCK TABLE ONLY public."%s" IN ACCESS EXCLUSIVE MODE NOWAIT`, table)); err != nil {
			return fmt.Errorf("蓝绿专项迁移无法立即获锁，保留旧实例：%w", err)
		}
		var allowed bool
		if err := tx.QueryRowContext(ctx, `SELECT c.relkind = 'r' AND c.reloftype = 0
            AND pg_total_relation_size(c.oid) <= $2
            AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhparent = c.oid OR i.inhrelid = c.oid)
            AND NOT EXISTS (SELECT 1 FROM pg_trigger t WHERE t.tgrelid = c.oid AND NOT t.tgisinternal)
            FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = 'public' AND c.relname = $1`, table, blueGreenSpecialTableLimit).Scan(&allowed); err != nil || !allowed {
			return fmt.Errorf("蓝绿专项迁移只允许无用户触发器、至多16MiB的普通表：%s", table)
		}
	}
	if err := precheckBlueGreenSpecial(ctx, tx, entry.Profile); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("执行蓝绿专项迁移失败：%w", err)
	}
	if entry.Profile == blueGreenReasoningProfile {
		for _, statement := range []string{
			`ALTER TABLE channel_model_pricing ADD CONSTRAINT sub2api_bg239_channel CHECK (max_reasoning_effort_multiplier IS NULL AND reasoning_effort_multipliers = '{}'::jsonb)`,
			`ALTER TABLE channel_account_stats_model_pricing ADD CONSTRAINT sub2api_bg239_stats CHECK (reasoning_effort_multipliers = '{}'::jsonb)`,
			`ALTER TABLE groups ADD CONSTRAINT sub2api_bg239_groups CHECK ` + blueGreenGroupPricingEmpty,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("安装新旧计费兼容保护失败：%w", err)
			}
		}
	}
	if err := verifyBlueGreenSpecialSchema(ctx, tx, entry.Profile); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}

func precheckBlueGreenSpecial(ctx context.Context, tx *sql.Tx, profile string) error {
	query := `SELECT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'public.user_affiliate_ledger'::regclass AND attname = 'operation_id' AND NOT attisdropped)
        OR to_regclass('public.idx_user_affiliate_ledger_operation_id') IS NOT NULL`
	if profile == blueGreenReasoningProfile {
		query = `SELECT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid IN ('public.channel_model_pricing'::regclass, 'public.channel_account_stats_model_pricing'::regclass)
            AND attname = 'reasoning_effort_multipliers' AND NOT attisdropped)
            OR EXISTS (SELECT 1 FROM channel_model_pricing WHERE max_reasoning_effort_multiplier IS NOT NULL)
            OR EXISTS (SELECT 1 FROM groups WHERE NOT ` + blueGreenGroupPricingEmpty + `)`
	}
	var conflict bool
	if err := tx.QueryRowContext(ctx, query).Scan(&conflict); err != nil {
		return fmt.Errorf("蓝绿专项迁移前置检查失败：%w", err)
	}
	if conflict {
		return fmt.Errorf("蓝绿专项迁移拒绝已配置倍率或未登记的同名字段/索引；不得绕过检查")
	}
	return nil
}

func verifyBlueGreenSpecialSchema(ctx context.Context, tx *sql.Tx, profile string) error {
	query := `SELECT a.atttypid = 'varchar'::regtype AND a.atttypmod = 68 AND NOT a.attnotnull AND NOT a.atthasdef
        AND a.attidentity = '' AND a.attgenerated = ''
        AND EXISTS (SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
            WHERE c.relname = 'idx_user_affiliate_ledger_operation_id' AND i.indrelid = a.attrelid
              AND i.indisunique AND i.indisvalid AND i.indisready AND i.indnatts = 1 AND i.indnkeyatts = 1
              AND i.indkey[0] = a.attnum AND i.indexprs IS NULL
              AND pg_get_expr(i.indpred, i.indrelid) = '(operation_id IS NOT NULL)')
        FROM pg_attribute a WHERE a.attrelid = 'public.user_affiliate_ledger'::regclass AND a.attname = 'operation_id' AND NOT a.attisdropped`
	if profile == blueGreenReasoningProfile {
		query = `SELECT count(*) = 2 AND bool_and(a.atttypid = 'jsonb'::regtype AND a.attnotnull
            AND a.attidentity = '' AND a.attgenerated = '' AND pg_get_expr(d.adbin, d.adrelid) = '''{}''::jsonb')
            FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
            WHERE a.attrelid IN ('public.channel_model_pricing'::regclass, 'public.channel_account_stats_model_pricing'::regclass)
              AND a.attname = 'reasoning_effort_multipliers' AND NOT a.attisdropped`
	}
	var valid bool
	if err := tx.QueryRowContext(ctx, query).Scan(&valid); err != nil || !valid {
		return fmt.Errorf("蓝绿专项迁移字段默认值、可空性或索引定义核验失败")
	}
	return nil
}
