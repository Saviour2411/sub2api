-- 154: Mark automatically created scheduled test plans.
-- Auto-managed plans are enabled only while an account is in a recoverable
-- unschedulable state, then disabled after a successful recovery probe.

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS auto_managed BOOLEAN NOT NULL DEFAULT false;

UPDATE scheduled_test_plans
SET auto_managed = true,
    updated_at = NOW()
WHERE auto_managed = false
  AND auto_recover = true
  AND cron_expression = '0 * * * *'
  AND max_results = 50;

UPDATE scheduled_test_plans p
SET enabled = false,
    next_run_at = NULL,
    updated_at = NOW()
FROM accounts a
WHERE p.account_id = a.id
  AND p.auto_managed = true
  AND NOT (
      a.status = 'error'
      OR (
          a.extra ? 'failure_strategy_unscheduled'
          AND a.extra->'failure_strategy_unscheduled' IS NOT NULL
          AND a.extra->'failure_strategy_unscheduled' <> 'null'::jsonb
          AND a.extra->'failure_strategy_unscheduled' <> '{}'::jsonb
      )
      OR a.rate_limit_reset_at > NOW()
      OR a.overload_until > NOW()
      OR a.temp_unschedulable_until > NOW()
  );
