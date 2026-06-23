-- 155: Speed up auto-managed scheduled recovery probes.
-- Auto-managed plans are disabled while healthy, so the shorter base cadence
-- only affects accounts already in recoverable unschedulable states.

UPDATE scheduled_test_plans
SET cron_expression = '*/5 * * * *',
    updated_at = NOW()
WHERE auto_managed = true
  AND cron_expression = '0 * * * *';

UPDATE scheduled_test_plans p
SET next_run_at = NOW(),
    updated_at = NOW()
FROM accounts a
WHERE p.account_id = a.id
  AND a.deleted_at IS NULL
  AND p.auto_managed = true
  AND p.enabled = true
  AND (p.next_run_at IS NULL OR p.next_run_at > NOW())
  AND (
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
