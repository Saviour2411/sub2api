#!/usr/bin/env python3
"""239专项计费保护的只读核验与退役后解除SQL；不自行连接数据库。"""
import argparse

PROFILE = "reasoning-empty-239-v1"
GUARDS = (
    ("channel_model_pricing", "sub2api_bg239_channel", "((max_reasoning_effort_multiplier IS NULL) AND (reasoning_effort_multipliers = '{}'::jsonb))"),
    ("channel_account_stats_model_pricing", "sub2api_bg239_stats", "(reasoning_effort_multipliers = '{}'::jsonb)"),
    ("groups", "sub2api_bg239_groups", "((model_pricing IS NULL) OR (model_pricing = 'null'::jsonb) OR ((jsonb_typeof(model_pricing) = 'array'::text) AND (jsonb_path_query_array(model_pricing, '$[*].\"max_reasoning_effort_multiplier\"'::jsonpath) <@ '[null]'::jsonb) AND (jsonb_path_query_array(model_pricing, '$[*].\"reasoning_effort_multipliers\"'::jsonpath) <@ '[{}]'::jsonb)))"),
)


def literal(value):
    return "'" + value.replace("'", "''") + "'"


def check_sql():
    values = ",".join("(" + ",".join(map(literal, entry)) + ")" for entry in GUARDS)
    return """SELECT json_build_object('present', count(c.oid), 'valid', count(c.oid) = 3 AND bool_and(
        c.contype = 'c' AND c.convalidated AND pg_get_expr(c.conbin,c.conrelid) = e.definition),
        'migrations', (SELECT count(*) = 2 FROM schema_migrations WHERE
            (filename = '239_channel_reasoning_effort_multipliers.sql' AND checksum = '66feb546785dfa385d8efd268a40276cd8ffdb0cead7026cd79e339a3b69edf1') OR
            (filename = '240_affiliate_ledger_operation_id.sql' AND checksum = '3823bfea5f64feb58f5fcebc341c6877eaefc97834b4cd652c8e83ad08ed78da')))
        FROM (VALUES """ + values + """) e(table_name, constraint_name, definition)
        LEFT JOIN pg_constraint c ON c.conrelid = to_regclass('public.' || e.table_name) AND c.conname = e.constraint_name"""


def release_sql():
    locks = "\n".join('LOCK TABLE ONLY public."' + table + '" IN ACCESS EXCLUSIVE MODE NOWAIT;' for table, _, _ in GUARDS)
    drops = "\n".join('ALTER TABLE public."' + table + '" DROP CONSTRAINT IF EXISTS "' + name + '";' for table, name, _ in GUARDS)
    return """BEGIN;
SET LOCAL lock_timeout = '1s';
SET LOCAL statement_timeout = '5s';
SET LOCAL transaction_timeout = '10s';
SET LOCAL search_path = pg_catalog, public;
DO $$ BEGIN
    IF NOT pg_try_advisory_xact_lock(694208311321144027) THEN RAISE EXCEPTION '迁移或备份正在执行，保留计费保护'; END IF;
END $$;
""" + locks + """
DO $$ DECLARE report json; BEGIN
    SELECT result INTO report FROM (""" + check_sql() + """) AS q(result);
    IF NOT (report->>'migrations')::boolean OR
       ((report->>'present')::int <> 0 AND NOT (report->>'valid')::boolean) THEN
        RAISE EXCEPTION '迁移账本或计费保护发生漂移，拒绝解除';
    END IF;
END $$;
""" + drops + "\nCOMMIT;"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sql", choices=("check", "release"), required=True)
    args = parser.parse_args()
    print(check_sql() if args.sql == "check" else release_sql())
