#!/usr/bin/env python3
"""252渠道Ultrafast共存保护；仅在旧槽退役后解除NULL限制。"""
import argparse


def check_sql():
    return """SELECT json_build_object(
        'present',(SELECT count(*) FROM pg_constraint WHERE conrelid='public.channel_model_pricing'::regclass
            AND conname='sub2api_bg252_price'),
        'valid',EXISTS(SELECT 1 FROM pg_constraint c JOIN pg_attribute a
            ON a.attrelid=c.conrelid AND a.attname='ultrafast_multiplier' AND NOT a.attisdropped
            WHERE c.conrelid='public.channel_model_pricing'::regclass AND c.conname='sub2api_bg252_price'
            AND c.contype='c' AND c.convalidated AND c.conenforced AND NOT c.connoinherit AND c.conislocal AND c.coninhcount=0
            AND c.conkey=ARRAY[a.attnum] AND pg_get_expr(c.conbin,c.conrelid)='(ultrafast_multiplier IS NULL)'),
        'migrations',EXISTS(SELECT 1 FROM public.schema_migrations
            WHERE filename='252_channel_pricing_ultrafast_multiplier.sql'
            AND checksum='458752b3523254ee60ab4a821261ab64df9216a6f208c1960e7b816f8c7cfd1d'),
        'schema',current_setting('server_version_num')::int/10000=18
            AND NOT EXISTS(SELECT 1 FROM pg_event_trigger WHERE evtenabled<>'D')
            AND (SELECT count(*)=2 AND bool_and(c.relkind='r' AND c.relpersistence='p' AND c.reloftype=0
                AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity AND pg_total_relation_size(c.oid)<=16777216
                AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=c.oid OR inhrelid=c.oid)
                AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
                AND NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=c.oid AND NOT tgisinternal))
                FROM pg_class c WHERE c.oid IN ('public.channel_model_pricing'::regclass,'public.schema_migrations'::regclass))
            AND EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid='public.channel_model_pricing'::regclass
                AND a.attname='ultrafast_multiplier' AND NOT a.attisdropped
                AND a.atttypid='numeric'::regtype AND format_type(a.atttypid,a.atttypmod)='numeric(12,6)'
                AND NOT a.attnotnull AND NOT a.atthasdef AND a.attidentity='' AND a.attgenerated=''
                AND col_description(a.attrelid,a.attnum)='Optional channel-specific price multiplier for OpenAI ultrafast service tier; NULL uses the model default'
                AND (SELECT count(*)=1 FROM pg_constraint WHERE conrelid=a.attrelid
                    AND conname='channel_model_pricing_ultrafast_multiplier_positive' AND contype='c' AND convalidated AND conenforced
                    AND NOT connoinherit AND conislocal AND coninhcount=0 AND conkey=ARRAY[a.attnum]
                    AND pg_get_expr(conbin,conrelid)='((ultrafast_multiplier IS NULL) OR (ultrafast_multiplier > (0)::numeric))')
                AND NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=a.attrelid AND a.attnum=ANY(conkey)
                    AND conname NOT IN ('channel_model_pricing_ultrafast_multiplier_positive','sub2api_bg252_price'))))"""


def release_sql():
    return """BEGIN;
SET LOCAL lock_timeout='1s';
SET LOCAL statement_timeout='5s';
SET LOCAL transaction_timeout='10s';
SET LOCAL search_path=pg_catalog,public;
DO $$ BEGIN
    IF NOT pg_try_advisory_xact_lock(694208311321144027) THEN
        RAISE EXCEPTION '迁移或备份正在执行，保留252定价保护';
    END IF;
END $$;
LOCK TABLE ONLY public.channel_model_pricing IN ACCESS EXCLUSIVE MODE NOWAIT;
LOCK TABLE ONLY public.schema_migrations IN SHARE ROW EXCLUSIVE MODE NOWAIT;
DO $$ DECLARE report json; BEGIN
    SELECT result INTO report FROM ("""+check_sql()+""") q(result);
    IF NOT (report->>'migrations')::boolean OR NOT (report->>'schema')::boolean OR
        ((report->>'present')::int<>0 AND NOT (report->>'valid')::boolean) THEN
        RAISE EXCEPTION '252账本、结构或共存保护漂移，拒绝解除';
    END IF;
END $$;
ALTER TABLE public.channel_model_pricing DROP CONSTRAINT IF EXISTS sub2api_bg252_price;
COMMIT;"""


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sql", choices=("check", "release"), required=True)
    args = parser.parse_args()
    print(check_sql() if args.sql == "check" else release_sql())
