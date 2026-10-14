package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const blueGreenModelQualityProfile = "model-quality-additive-246-v1"

var blueGreenModelQualityMigration = blueGreenMigration{
	Filename: "246_model_quality.sql",
	Checksum: "1bb3d0dd89a9e48f4c524f306b3c248873581b6a33aa1f37f1e24fc3caab545e",
	Profile:  blueGreenModelQualityProfile,
}

// 固定原始SQL在PostgreSQL18中生成的结构摘要，不包含业务行或序列当前值。
const blueGreenModelQualitySchemaChecksum = "944a2a7bc61a8a8c610635623cf25b37c843ad1822ac659c1f1f62472efd14bd"

const blueGreenModelQualitySchemaSQL = `WITH tables AS (
    SELECT c.* FROM pg_class c WHERE c.relnamespace='public'::regnamespace
    AND (c.relname='settings' OR c.relname ~ '^model_quality_') AND c.relkind IN ('r','p','v','m','f')
), relations AS (
    SELECT c.relname,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,c.reloftype<>0 AS typed,
        EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid) AS inherits,
        (SELECT count(*) FROM pg_rewrite WHERE ev_class=c.oid) AS rules,
        (SELECT coalesce(jsonb_agg(pg_get_triggerdef(t.oid) ORDER BY t.tgname COLLATE "C"),'[]'::jsonb)
            FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal) AS triggers
    FROM tables c ORDER BY c.relname
), columns AS (
    SELECT c.relname,a.attnum,a.attname,format_type(a.atttypid,a.atttypmod) AS type,a.attnotnull,
        a.attidentity,a.attgenerated,coalesce(pg_get_expr(d.adbin,d.adrelid),'') AS default_value
    FROM tables c JOIN pg_attribute a ON a.attrelid=c.oid
    LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
    WHERE a.attnum>0 AND NOT a.attisdropped ORDER BY c.relname,a.attnum
), indexes AS (
    SELECT c.relname,ic.relname AS name,pg_get_indexdef(i.indexrelid) AS definition,i.indisvalid,i.indisready
    FROM tables c JOIN pg_index i ON i.indrelid=c.oid JOIN pg_class ic ON ic.oid=i.indexrelid
    ORDER BY c.relname,ic.relname
), constraints AS (
    SELECT c.relname,k.conname,k.contype,k.convalidated,k.condeferrable,k.condeferred,pg_get_constraintdef(k.oid) AS definition
    FROM tables c JOIN pg_constraint k ON k.conrelid=c.oid WHERE k.contype<>'n' ORDER BY c.relname,k.conname
), sequences AS (
    SELECT c.relname,format_type(s.seqtypid,NULL) AS type,s.seqstart,s.seqincrement,s.seqmax,s.seqmin,s.seqcache,s.seqcycle,
        (SELECT coalesce(jsonb_agg(jsonb_build_object('table',r.relname,'column',a.attname,'dependency',d.deptype)
            ORDER BY r.relname COLLATE "C",a.attname COLLATE "C"),'[]'::jsonb)
         FROM pg_depend d JOIN pg_class r ON r.oid=d.refobjid
         JOIN pg_attribute a ON a.attrelid=r.oid AND a.attnum=d.refobjsubid
         WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.refclassid='pg_class'::regclass AND d.deptype IN ('a','i')) AS owners
    FROM pg_class c JOIN pg_sequence s ON s.seqrelid=c.oid
    WHERE c.relnamespace='public'::regnamespace AND (c.relname='settings_id_seq' OR c.relname ~ '^model_quality_') ORDER BY c.relname
), objects AS (
    SELECT relname,relkind FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^model_quality_' ORDER BY relname
)
SELECT jsonb_build_object('relations',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM relations r),
    'columns',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.attnum) FROM columns r),
    'indexes',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.name COLLATE "C") FROM indexes r),
    'constraints',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C",r.conname COLLATE "C") FROM constraints r),
    'sequences',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM sequences r),
    'objects',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.relname COLLATE "C") FROM objects r))::text`

func verifyBlueGreenModelQualitySchema(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path=public"); err != nil {
		return err
	}
	var definition string
	if err := tx.QueryRowContext(ctx, blueGreenModelQualitySchemaSQL).Scan(&definition); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(definition))
	if hex.EncodeToString(digest[:]) != blueGreenModelQualitySchemaChecksum {
		return fmt.Errorf("蓝绿降智检测表、字段、索引、约束或序列定义漂移")
	}
	return nil
}

func verifyBlueGreenModelQualityApplied(ctx context.Context, conn *sql.Conn) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := verifyBlueGreenModelQualitySchema(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func applyBlueGreenModelQuality(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
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
		return fmt.Errorf("蓝绿降智检测固定迁移仅支持 PostgreSQL 18")
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL transaction_timeout='10s'"); err != nil {
		return err
	}
	// 仅短暂封闭设置写入，普通读取仍可进行；冲突立即退出，不等待旧实例事务。
	for _, table := range []string{"settings", "schema_migrations"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("LOCK TABLE ONLY public.%s IN SHARE ROW EXCLUSIVE MODE NOWAIT", table)); err != nil {
			return fmt.Errorf("蓝绿降智检测无法立即获锁，保留旧实例：%w", err)
		}
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND pg_total_relation_size(c.oid)<=$2
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=c.oid AND NOT tgisinternal)
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, blueGreenSpecialTableLimit).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿降智检测既有表超出16MiB或结构、触发器、RLS、规则不符：%s", table)
		}
	}
	var clean bool
	err = tx.QueryRowContext(ctx, `SELECT
        NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
        AND NOT EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='public'::regnamespace AND relname ~ '^model_quality_')
        AND NOT EXISTS(SELECT 1 FROM pg_type WHERE typnamespace='public'::regnamespace AND typname ~ '^_?model_quality_')
        AND NOT EXISTS(SELECT 1 FROM public.settings WHERE key='model_quality_enabled')
        AND NOT EXISTS(SELECT 1 FROM public.schema_migrations WHERE filename=$1)`, entry.Filename).Scan(&clean)
	if err != nil || !clean {
		return fmt.Errorf("蓝绿降智检测发现已有对象、开关、账本或DDL触发器，拒绝覆盖")
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("蓝绿降智检测原始迁移失败，事务回滚：%w", err)
	}
	if err := verifyBlueGreenModelQualitySchema(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}
