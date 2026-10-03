#!/usr/bin/env python3
"""241平台扩展的兼容保护核验与退役后解除SQL，不自行连接数据库。"""
import argparse

PROFILE = "platform-guarded-241-v1"
PLATFORMS = ("anthropic", "openai", "gemini", "antigravity", "grok", "kimi", "zhipu", "deepseek", "minimax", "opencode_go", "typesafe")
TARGETS = (
    ("user_platform_quotas", "platform", "user_platform_quotas_platform_check", "sub2api_bg241_quota"),
    ("composite_model_routes", "target_platform", "composite_model_routes_target_platform_check", "sub2api_bg241_route"),
)
MIGRATIONS = (
    ("149_payment_balance_bonus_rules.sql", "7c8a322e0c7296e5226229b355a92274e7ab6165dea560d4cc3d0adcb0303278"),
    ("238_opencode_go_platform.sql", "6f987e251519bd3759e60da44620a5d777494cceb333b6ce394aa0ea536ef5a2"),
    ("241_add_payment_order_bonus_amount.sql", "18b6a524a9873d3e4a45e6f9bd03b6e69f4384b701825c659817e017f2e8244f"),
    ("241_add_typesafe_platform.sql", "b6559525bf8d0b5d7c617e7415944f0d1fae8c408e9131ace84139795f66dcd2"),
)


def literal(value):
    return "'" + value.replace("'", "''") + "'"


def check_sql():
    values = ",".join("(" + ",".join(map(literal, (table, guard, f"(({column})::text <> 'typesafe'::text)"))) + ")"
                      for table, column, _, guard in TARGETS)
    platforms = ", ".join(literal(value) + "::character varying" for value in PLATFORMS)
    schema = ",".join("(" + ",".join(map(literal, (table, name, f"(({column})::text = ANY ((ARRAY[{platforms}])::text[]))"))) + ")"
                      for table, column, name, _ in TARGETS)
    migrations = ",".join("(" + ",".join(map(literal, item)) + ")" for item in MIGRATIONS)
    return """SELECT json_build_object(
        'present', count(c.oid),
        'valid', count(c.oid)=2 AND bool_and(c.contype='c' AND c.convalidated AND c.conislocal
            AND c.coninhcount=0 AND NOT c.connoinherit AND pg_get_expr(c.conbin,c.conrelid)=e.definition),
        'migrations', (SELECT count(m.filename)=4 FROM (VALUES """ + migrations + """) expected(filename,checksum)
            LEFT JOIN schema_migrations m USING (filename,checksum)),
        'schema', (SELECT count(k.oid)=2 AND bool_and(k.contype='c' AND k.convalidated AND k.conislocal
            AND k.coninhcount=0 AND NOT k.connoinherit AND pg_get_expr(k.conbin,k.conrelid)=s.definition)
            FROM (VALUES """ + schema + """) s(table_name,constraint_name,definition)
            LEFT JOIN pg_constraint k ON k.conrelid=to_regclass('public.' || s.table_name) AND k.conname=s.constraint_name)
            AND coalesce((SELECT a.atttypid='numeric'::regtype AND format_type(a.atttypid,a.atttypmod)='numeric(20,2)'
                AND a.attnotnull AND a.attidentity='' AND a.attgenerated='' AND pg_get_expr(d.adbin,d.adrelid)='0'
                FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
                WHERE a.attrelid='public.payment_orders'::regclass AND a.attname='bonus_amount' AND NOT a.attisdropped),false))
        FROM (VALUES """ + values + """) e(table_name,constraint_name,definition)
        LEFT JOIN pg_constraint c ON c.conrelid=to_regclass('public.' || e.table_name) AND c.conname=e.constraint_name"""


def release_sql():
    locks = "\n".join('LOCK TABLE ONLY public."' + table + '" IN ACCESS EXCLUSIVE MODE NOWAIT;'
                      for table in ("payment_orders", "user_platform_quotas", "composite_model_routes"))
    drops = "\n".join('ALTER TABLE public."' + table + '" DROP CONSTRAINT IF EXISTS "' + guard + '";'
                      for table, _, _, guard in TARGETS)
    return """BEGIN;
SET LOCAL lock_timeout='1s';
SET LOCAL statement_timeout='5s';
SET LOCAL transaction_timeout='10s';
SET LOCAL search_path=pg_catalog,public;
DO $$ BEGIN
    IF current_setting('server_version_num')::int / 10000 <> 18 THEN RAISE EXCEPTION '241只批准PostgreSQL18'; END IF;
    IF NOT pg_try_advisory_xact_lock(694208311321144027) THEN RAISE EXCEPTION '迁移或备份正在执行，保留241平台保护'; END IF;
END $$;
""" + locks + """
DO $$ DECLARE report json; valid_tables boolean; BEGIN
    SELECT count(*)=3 AND bool_and(c.relkind='r' AND c.reloftype=0 AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
        AND pg_total_relation_size(c.oid)<=16777216
        AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhparent=c.oid OR i.inhrelid=c.oid)
        AND NOT EXISTS (SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal)) INTO valid_tables
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relname IN ('payment_orders','user_platform_quotas','composite_model_routes');
    IF NOT coalesce(valid_tables,false) THEN RAISE EXCEPTION '241表结构或大小变化，保留平台保护'; END IF;
    SELECT result INTO report FROM (""" + check_sql() + """) AS q(result);
    IF NOT coalesce((report->>'migrations')::boolean,false)
       OR NOT coalesce((report->>'schema')::boolean,false)
       OR ((report->>'present')::int <> 0 AND NOT coalesce((report->>'valid')::boolean,false)) THEN
        RAISE EXCEPTION '241迁移账本、结构或保护发生漂移，拒绝解除';
    END IF;
END $$;
""" + drops + "\nCOMMIT;"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sql", choices=("check", "release"), required=True)
    args = parser.parse_args()
    print(check_sql() if args.sql == "check" else release_sql())
