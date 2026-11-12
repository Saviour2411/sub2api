#!/usr/bin/env python3
"""249/250固定发布的定价保护；旧槽退出前只允许原有通用缓存价。"""
import argparse

TABLES = ("channel_model_pricing", "channel_pricing_intervals",
          "channel_account_stats_model_pricing", "channel_account_stats_pricing_intervals")


def check_sql():
    tables = ",".join("('"+table+"')" for table in TABLES)
    return """SELECT json_build_object('present',count(c.oid),'valid',count(c.oid)=4 AND bool_and(
        c.contype='c' AND c.convalidated AND pg_get_expr(c.conbin,c.conrelid)='(cache_write_12h_price IS NULL)'),
        'migrations',(SELECT count(*)=2 FROM schema_migrations WHERE
            (filename='249_add_usage_logs_api_key_cost_index_notx.sql' AND checksum='94f124c3e50405f7b75746885111a4f662c331985a697de46fe946a17ff1f7d9') OR
            (filename='250_openai_cache_write_ttl.sql' AND checksum='532551ce7aee86b2ee0a78d397d422424ceed4051e31d5f542277c80c21b8b98')))
        FROM (VALUES """+tables+""") e(table_name) LEFT JOIN pg_constraint c
        ON c.conrelid=to_regclass('public.'||e.table_name) AND c.conname='sub2api_bg250_price'"""


def release_sql():
    locks = "\n".join("LOCK TABLE ONLY public."+table+" IN ACCESS EXCLUSIVE MODE NOWAIT;" for table in TABLES)
    drops = "\n".join("ALTER TABLE public."+table+" DROP CONSTRAINT IF EXISTS sub2api_bg250_price;" for table in TABLES)
    return """BEGIN;
SET LOCAL lock_timeout='1s';
SET LOCAL statement_timeout='5s';
SET LOCAL transaction_timeout='10s';
SET LOCAL search_path=pg_catalog,public;
DO $$ BEGIN
    IF NOT pg_try_advisory_xact_lock(694208311321144027) THEN
        RAISE EXCEPTION '迁移或备份正在执行，保留250定价保护';
    END IF;
END $$;
"""+locks+"""
DO $$ DECLARE report json; BEGIN
    SELECT result INTO report FROM ("""+check_sql()+""") q(result);
    IF NOT (report->>'migrations')::boolean OR
        ((report->>'present')::int<>0 AND NOT (report->>'valid')::boolean) THEN
        RAISE EXCEPTION '250账本或定价保护漂移，拒绝解除';
    END IF;
END $$;
"""+drops+"\nCOMMIT;"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sql", choices=("check", "release"), required=True)
    args = parser.parse_args()
    print(check_sql() if args.sql == "check" else release_sql())
