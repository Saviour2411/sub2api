-- 合并每个账号重复的自动测活计划，并为后续幂等创建增加唯一约束。

WITH plan_summary AS (
    SELECT
        account_id,
        MIN(id) AS keeper_id,
        BOOL_OR(enabled) AS enabled,
        BOOL_OR(auto_recover) AS auto_recover,
        MAX(max_results) AS max_results,
        MAX(last_run_at) AS last_run_at,
        MIN(next_run_at) FILTER (WHERE enabled = true) AS next_run_at
    FROM scheduled_test_plans
    WHERE auto_managed = true
    GROUP BY account_id
    HAVING COUNT(*) > 1
)
UPDATE scheduled_test_plans keeper
SET enabled = summary.enabled,
    auto_recover = summary.auto_recover,
    max_results = summary.max_results,
    last_run_at = summary.last_run_at,
    next_run_at = CASE
        WHEN summary.enabled THEN COALESCE(summary.next_run_at, NOW())
        ELSE NULL
    END,
    updated_at = NOW()
FROM plan_summary summary
WHERE keeper.id = summary.keeper_id;

WITH duplicate_mapping AS (
    SELECT
        plan.id AS duplicate_id,
        MIN(plan.id) OVER (PARTITION BY plan.account_id) AS keeper_id
    FROM scheduled_test_plans plan
    WHERE plan.auto_managed = true
)
UPDATE scheduled_test_results result
SET plan_id = mapping.keeper_id
FROM duplicate_mapping mapping
WHERE result.plan_id = mapping.duplicate_id
  AND mapping.duplicate_id <> mapping.keeper_id;

WITH duplicate_mapping AS (
    SELECT
        plan.id,
        MIN(plan.id) OVER (PARTITION BY plan.account_id) AS keeper_id
    FROM scheduled_test_plans plan
    WHERE plan.auto_managed = true
)
DELETE FROM scheduled_test_plans plan
USING duplicate_mapping mapping
WHERE plan.id = mapping.id
  AND mapping.id <> mapping.keeper_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_scheduled_test_plans_auto_managed_account
    ON scheduled_test_plans(account_id)
    WHERE auto_managed = true;
