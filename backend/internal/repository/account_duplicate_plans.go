package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func copyAccountScheduledTestPlans(ctx context.Context, client *dbent.Client, sourceID, targetID int64) error {
	// 不复制执行时间和历史结果；按源创建顺序分配新 ID，保留恢复模板的稳定优先级。
	if _, err := client.ExecContext(ctx, `
		INSERT INTO scheduled_test_plans (
			account_id, model_id, prompt, cron_expression, enabled, max_results,
			auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
		)
		SELECT $2, model_id, prompt, cron_expression, false, max_results,
			CASE WHEN auto_managed THEN true ELSE auto_recover END,
			auto_managed, NULL, NULL, NOW(), NOW()
		FROM scheduled_test_plans
		WHERE account_id = $1
		ORDER BY created_at, id
	`, sourceID, targetID); err != nil {
		return err
	}

	// 已复制的系统计划优先；没有系统计划时，使用最早的普通恢复计划，否则使用默认值。
	_, err := client.ExecContext(ctx, `
		WITH template AS (
			SELECT model_id, prompt, cron_expression, max_results
			FROM scheduled_test_plans
			WHERE account_id = $1 AND auto_recover = true AND auto_managed = false
			ORDER BY id
			LIMIT 1
		)
		INSERT INTO scheduled_test_plans (
			account_id, model_id, prompt, cron_expression, enabled, max_results,
			auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at
		)
		SELECT $1, COALESCE(t.model_id, ''), COALESCE(t.prompt, ''),
			COALESCE(t.cron_expression, '*/5 * * * *'), false, COALESCE(t.max_results, 20),
			true, true, NULL, NULL, NOW(), NOW()
		FROM (VALUES (1)) AS seed(n)
		LEFT JOIN template t ON true
		WHERE NOT EXISTS (
			SELECT 1 FROM scheduled_test_plans WHERE account_id = $1 AND auto_managed = true
		)
	`, targetID)
	return err
}
