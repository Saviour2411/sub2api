package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const blueGreen252Profile = "channel-ultrafast-guarded-252-v1"

var blueGreen252Migration = blueGreenMigration{
	Filename: "252_channel_pricing_ultrafast_multiplier.sql",
	Checksum: "458752b3523254ee60ab4a821261ab64df9216a6f208c1960e7b816f8c7cfd1d",
	Profile:  blueGreen252Profile,
}

func validateBlueGreen252Policy(policy *blueGreenMigrationPolicy) error {
	for _, entry := range policy.Migrations {
		if entry.Profile == blueGreen252Profile && (len(policy.Migrations) != 1 || entry != blueGreen252Migration) {
			return fmt.Errorf("蓝绿252仅允许唯一的固定渠道Ultrafast迁移")
		}
	}
	return nil
}

func blueGreen252Transaction(ctx context.Context, conn *sql.Conn, readOnly bool) (*sql.Tx, error) {
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
	if err != nil {
		return nil, err
	}
	if err := blueGreen251Settings(ctx, tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func validateBlueGreen252Tables(ctx context.Context, tx *sql.Tx, limit bool) error {
	for _, table := range []string{"channel_model_pricing", "schema_migrations"} {
		var valid bool
		err := tx.QueryRowContext(ctx, `SELECT c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
            AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
            AND (NOT $2 OR pg_total_relation_size(c.oid)<=$3)
            AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
            AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=c.oid AND NOT tgisinternal)
            FROM pg_class c WHERE c.oid=to_regclass('public.'||$1)`, table, limit, blueGreenSpecialTableLimit).Scan(&valid)
		if err != nil || !valid {
			return fmt.Errorf("蓝绿252表类型、大小、RLS、规则或触发器不符：%s", table)
		}
	}
	return nil
}

func verifyBlueGreen252Schema(ctx context.Context, tx *sql.Tx, requireGuard bool) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT
        a.atttypid='numeric'::regtype AND format_type(a.atttypid,a.atttypmod)='numeric(12,6)'
        AND NOT a.attnotnull AND NOT a.atthasdef AND a.attidentity='' AND a.attgenerated=''
        AND col_description(a.attrelid,a.attnum)='Optional channel-specific price multiplier for OpenAI ultrafast service tier; NULL uses the model default'
        AND (SELECT count(*)=1 FROM pg_constraint WHERE conrelid=a.attrelid
            AND conname='channel_model_pricing_ultrafast_multiplier_positive' AND contype='c' AND convalidated AND conenforced
            AND NOT connoinherit AND conislocal AND coninhcount=0 AND conkey=ARRAY[a.attnum]
            AND pg_get_expr(conbin,conrelid)='((ultrafast_multiplier IS NULL) OR (ultrafast_multiplier > (0)::numeric))')
        AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=a.attrelid AND a.attnum=ANY(conkey)
            AND conname NOT IN ('channel_model_pricing_ultrafast_multiplier_positive','sub2api_bg252_price'))
        AND (SELECT CASE WHEN count(*)=0 THEN NOT $1 ELSE count(*)=1 AND bool_and(
            contype='c' AND convalidated AND conenforced AND NOT connoinherit AND conislocal AND coninhcount=0
            AND conkey=ARRAY[a.attnum] AND pg_get_expr(conbin,conrelid)='(ultrafast_multiplier IS NULL)') END
            FROM pg_constraint WHERE conrelid=a.attrelid AND conname='sub2api_bg252_price')
        FROM pg_attribute a WHERE a.attrelid='public.channel_model_pricing'::regclass
        AND a.attname='ultrafast_multiplier' AND NOT a.attisdropped`, requireGuard).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("蓝绿252渠道倍率字段、正值约束或共存保护缺失或漂移")
	}
	return nil
}

func applyBlueGreen252(ctx context.Context, conn *sql.Conn, entry blueGreenMigration, content string) error {
	if entry != blueGreen252Migration {
		return fmt.Errorf("蓝绿252迁移清单不符")
	}
	if err := validateBlueGreenSQL(entry, content); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := blueGreen252Transaction(ctx, conn, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 旧版批量保存会删后重建；同事务安装NULL保护，直到旧实例完全退役。
	for _, statement := range []string{
		"LOCK TABLE ONLY public.channel_model_pricing IN ACCESS EXCLUSIVE MODE NOWAIT",
		"LOCK TABLE ONLY public.schema_migrations IN SHARE ROW EXCLUSIVE MODE NOWAIT",
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("蓝绿252无法立即获锁，保留旧实例：%w", err)
		}
	}
	if err := validateBlueGreen252Tables(ctx, tx, true); err != nil {
		return err
	}
	var clean bool
	err = tx.QueryRowContext(ctx, `SELECT
        NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='public.channel_model_pricing'::regclass AND attname='ultrafast_multiplier' AND NOT attisdropped)
        AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='public.channel_model_pricing'::regclass
            AND conname IN ('channel_model_pricing_ultrafast_multiplier_positive','sub2api_bg252_price'))
        AND NOT EXISTS(SELECT 1 FROM public.schema_migrations WHERE filename=$1)`, entry.Filename).Scan(&clean)
	if err != nil || !clean {
		return fmt.Errorf("蓝绿252发现同名字段、约束或账本，拒绝覆盖")
	}
	var before, after int64
	if err := tx.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.channel_model_pricing')").Scan(&before); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("蓝绿252原始迁移失败，整体回滚：%w", err)
	}
	if _, err := tx.ExecContext(ctx, "ALTER TABLE public.channel_model_pricing ADD CONSTRAINT sub2api_bg252_price CHECK(ultrafast_multiplier IS NULL)"); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, "SELECT pg_relation_filenode('public.channel_model_pricing')").Scan(&after); err != nil || before != after {
		return fmt.Errorf("蓝绿252定价表发生物理重写，拒绝提交")
	}
	if err := verifyBlueGreen252Schema(ctx, tx, true); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO public.schema_migrations(filename,checksum) VALUES($1,$2)", entry.Filename, entry.Checksum); err != nil {
		return err
	}
	return tx.Commit()
}

func validateBlueGreen252Applied(ctx context.Context, conn *sql.Conn, policy *blueGreenMigrationPolicy, applied map[string]string) error {
	if err := validateBlueGreen252Policy(policy); err != nil {
		return err
	}
	if _, ok := policy.find(blueGreen252Migration.Filename); !ok {
		return nil
	}
	digest, ok := applied[blueGreen252Migration.Filename]
	if !ok {
		return nil
	}
	if digest != blueGreen252Migration.Checksum {
		return fmt.Errorf("蓝绿252迁移账本摘要不符")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := blueGreen252Transaction(ctx, conn, true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := validateBlueGreen252Tables(ctx, tx, false); err != nil {
		return err
	}
	// 稳态重启允许已解除保护；切流和回滚仍由部署器强制要求保护存在。
	if err := verifyBlueGreen252Schema(ctx, tx, false); err != nil {
		return err
	}
	return tx.Commit()
}
