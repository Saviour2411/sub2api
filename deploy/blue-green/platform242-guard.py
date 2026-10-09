#!/usr/bin/env python3
"""242专用旧平台保护；仅在固定发布的旧实例全部退役后解除。"""
import argparse


# 冻结为v0.1.251的数据库白名单，不能随新版应用平台目录扩展。
PLATFORMS = ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi',
             'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe')
TARGETS = (
    ('user_platform_quotas', 'platform', 32, 'user_platform_quotas_platform_check', 'sub2api_bg242_quota', 'sub2api_bg241_quota'),
    ('composite_model_routes', 'target_platform', 50, 'composite_model_routes_target_platform_check', 'sub2api_bg242_route', 'sub2api_bg241_route'),
)
MIGRATIONS = (
    ('238_opencode_go_platform.sql', '6f987e251519bd3759e60da44620a5d777494cceb333b6ce394aa0ea536ef5a2'),
    ('241_add_typesafe_platform.sql', 'b6559525bf8d0b5d7c617e7415944f0d1fae8c408e9131ace84139795f66dcd2'),
    ('242_drop_platform_check_constraints.sql', 'a7725f31f637208d200e0f37d5270e9f263743d99fa9e4f1bb9a7387e8be5a61'),
)


def literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def check_sql():
    platforms = ', '.join(literal(value) + '::character varying' for value in PLATFORMS)
    rows = []
    for table, column, length, original, guard, previous in TARGETS:
        definition = f"(({column})::text = ANY ((ARRAY[{platforms}])::text[]))"
        rows.append('(' + ','.join(map(literal, (table, column, f'character varying({length})', original, guard, previous, definition))) + ')')
    migrations = ','.join('(' + ','.join(map(literal, item)) + ')' for item in MIGRATIONS)
    return """WITH expected(table_name,column_name,data_type,original_name,guard_name,previous_name,definition) AS (VALUES """ + ','.join(rows) + """)
    SELECT json_build_object(
        'present', count(c.oid),
        'valid', count(c.oid)=2 AND bool_and(c.contype='c' AND c.convalidated AND c.conislocal
            AND c.coninhcount=0 AND NOT c.connoinherit AND pg_get_expr(c.conbin,c.conrelid)=e.definition),
        'migrations', (SELECT count(m.filename)=3 FROM (VALUES """ + migrations + """) required(filename,checksum)
            LEFT JOIN schema_migrations m USING (filename,checksum)),
        'schema', (SELECT count(a.attnum)=2 AND bool_and(a.atttypid='varchar'::regtype
            AND format_type(a.atttypid,a.atttypmod)=s.data_type AND a.attnotnull
            AND a.attidentity='' AND a.attgenerated=''
            AND NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid=a.attrelid
                AND k.conname IN (s.original_name,s.previous_name)))
            FROM expected s LEFT JOIN pg_attribute a ON a.attrelid=to_regclass('public.' || s.table_name)
                AND a.attname=s.column_name AND NOT a.attisdropped))
    FROM expected e LEFT JOIN pg_constraint c ON c.conrelid=to_regclass('public.' || e.table_name) AND c.conname=e.guard_name"""


def release_sql():
    locks = '\n'.join('LOCK TABLE ONLY public."' + table + '" IN ACCESS EXCLUSIVE MODE NOWAIT;' for table, *_ in TARGETS)
    drops = '\n'.join('ALTER TABLE public."' + table + '" DROP CONSTRAINT IF EXISTS "' + guard + '";'
                      for table, _, _, _, guard, _ in TARGETS)
    return """BEGIN;
SET LOCAL lock_timeout='1s';
SET LOCAL statement_timeout='5s';
SET LOCAL transaction_timeout='10s';
SET LOCAL search_path=pg_catalog,public;
DO $$ BEGIN
    IF current_setting('server_version_num')::int / 10000 <> 18 THEN RAISE EXCEPTION '242只批准PostgreSQL18'; END IF;
    IF NOT pg_try_advisory_xact_lock(694208311321144027) THEN RAISE EXCEPTION '迁移或备份正在执行，保留242保护'; END IF;
END $$;
""" + locks + """
DO $$ DECLARE report json; valid_tables boolean; BEGIN
    SELECT count(*)=2 AND bool_and(c.relkind='r' AND c.reloftype=0 AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
        AND pg_total_relation_size(c.oid)<=16777216
        AND NOT EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhparent=c.oid OR i.inhrelid=c.oid)
        AND NOT EXISTS (SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal)) INTO valid_tables
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relname IN ('user_platform_quotas','composite_model_routes');
    IF NOT coalesce(valid_tables,false) THEN RAISE EXCEPTION '242表结构或大小变化，保留平台保护'; END IF;
    SELECT result INTO report FROM (""" + check_sql() + """) AS q(result);
    IF NOT coalesce((report->>'migrations')::boolean,false)
       OR NOT coalesce((report->>'schema')::boolean,false)
       OR ((report->>'present')::int <> 0 AND NOT coalesce((report->>'valid')::boolean,false)) THEN
        RAISE EXCEPTION '242账本、结构或保护发生漂移，拒绝解除';
    END IF;
END $$;
""" + drops + "\nCOMMIT;"


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sql', choices=('check', 'release'), required=True)
    args = parser.parse_args()
    print(check_sql() if args.sql == 'check' else release_sql())
