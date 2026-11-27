package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const blueGreenQualityV2Profile = "model-quality-v2-guarded-253-v1"

var blueGreenQualityV2Migration = blueGreenMigration{Filename: "253_model_quality_v2.sql", Checksum: "6c21f152ee5f10f0a846f5957f749df19d8af64fe0760a831640a317604bb200", Profile: blueGreenQualityV2Profile}

const blueGreenQualityV2SchemaChecksum = "eba04534198cc40ae850e219774a62e12fc3d1d0ff56d2bdd3cdab970d1fdc48"
const blueGreenQualityV2SchemaSQL = `WITH tables AS (
    SELECT c.* FROM pg_class c WHERE c.relnamespace='public'::regnamespace
    AND c.relname ~ '^model_quality_v2_' AND c.relkind IN ('r','p','v','m','f')
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
    SELECT relname,relkind FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^(idx_)?model_quality_v2_'
), types AS (
    SELECT typname,typtype FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?(idx_)?model_quality_v2_'
)
SELECT jsonb_build_object('relations',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM relations r),
    'columns',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.attnum) FROM columns r),
    'indexes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.name COLLATE "C") FROM indexes r),
    'constraints',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.conname COLLATE "C") FROM constraints r),
    'objects',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM objects r),
    'types',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.typname COLLATE "C") FROM types r))::text`

func validateBlueGreenQualityV2Policy(policy *blueGreenMigrationPolicy) error {
	if entry, ok := policy.find(blueGreenQualityV2Migration.Filename); ok && (entry != blueGreenQualityV2Migration || len(policy.Migrations) != 1) {
		return fmt.Errorf("蓝绿253仅允许唯一的固定新版检测迁移")
	}
	return nil
}
func verifyBlueGreenQualityV2Schema(ctx context.Context, tx *sql.Tx) error {
	var definition string
	if err := tx.QueryRowContext(ctx, blueGreenQualityV2SchemaSQL).Scan(&definition); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(definition))
	if hex.EncodeToString(digest[:]) != blueGreenQualityV2SchemaChecksum {
		return fmt.Errorf("蓝绿253新版检测结构发生漂移")
	}
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.model_quality_config'::regclass AND conname='model_quality_v2_legacy_disabled' AND contype='c' AND convalidated AND pg_get_constraintdef(oid)='CHECK ((NOT enabled))') AND NOT EXISTS(SELECT 1 FROM public.model_quality_config WHERE enabled)`).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿253旧检测停用保护不存在或不一致")
	}
	return nil
}
func verifyBlueGreenQualityV2Applied(ctx context.Context, conn *sql.Conn) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, "SET LOCAL search_path=public"); err != nil {
		return err
	}
	if err = verifyBlueGreenQualityV2Schema(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func applyBlueGreenQualityV2(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	if entry != blueGreenQualityV2Migration {
		return fmt.Errorf("蓝绿253迁移未经批准")
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var version int
	if err = tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version/10000 != 18 {
		return fmt.Errorf("蓝绿253固定迁移仅支持PostgreSQL18")
	}
	for _, sql := range []string{"SET LOCAL search_path=public", "SET LOCAL lock_timeout='1s'", "SET LOCAL statement_timeout='5s'", "SET LOCAL transaction_timeout='10s'", "LOCK TABLE ONLY public.model_quality_config IN ACCESS EXCLUSIVE MODE NOWAIT", "LOCK TABLE ONLY public.settings, public.schema_migrations IN SHARE ROW EXCLUSIVE MODE NOWAIT"} {
		if _, err = tx.ExecContext(ctx, sql); err != nil {
			return fmt.Errorf("蓝绿253无法立即获得短事务锁，保留旧实例：%w", err)
		}
	}
	var valid bool
	err = tx.QueryRowContext(ctx, `SELECT
 NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
 AND NOT EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^model_quality_v2_')
 AND NOT EXISTS(SELECT 1 FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?model_quality_v2_')
 AND NOT EXISTS(SELECT 1 FROM public.schema_migrations WHERE filename=$1)
 AND EXISTS(SELECT 1 FROM public.model_quality_config WHERE id=1 AND NOT enabled)
 AND NOT EXISTS(SELECT 1 FROM public.model_quality_tasks WHERE status IN ('submitting','running','pending','uncertain'))
 AND (SELECT count(*)=3 FROM pg_class c WHERE c.oid IN ('public.settings'::regclass,'public.schema_migrations'::regclass,'public.model_quality_config'::regclass)
 AND c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0 AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
 AND pg_total_relation_size(c.oid)<=16777216 AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=c.oid AND NOT tgisinternal)
 AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid) AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid))`, entry.Filename).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿253基线不符，旧检测未暂停或仍有未完成任务：%v", err)
	}
	if _, err = tx.ExecContext(ctx, content); err != nil {
		return err
	}
	if err = verifyBlueGreenQualityV2Schema(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}
