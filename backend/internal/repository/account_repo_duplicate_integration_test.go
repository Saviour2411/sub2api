//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateDuplicateWithPlansPersistsPausedCopyAtomically(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	suffix := time.Now().UnixNano()
	source, err := client.Account.Create().SetName(fmt.Sprintf("duplicate-source-%d", suffix)).
		SetPlatform(service.PlatformAnthropic).SetType(service.AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "source-secret"}).Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName(fmt.Sprintf("duplicate-atomic-%d", suffix)).
		SetPlatform(service.PlatformAnthropic).
		Save(ctx)
	require.NoError(t, err)

	success := &service.Account{
		Name:        fmt.Sprintf("duplicate-success-%d", suffix),
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: false,
		Credentials: map[string]any{"api_key": "secret"},
		Extra:       map[string]any{},
	}
	require.NoError(t, repo.CreateDuplicateWithPlans(ctx, source.ID, success, []service.AccountGroup{{GroupID: group.ID, Priority: 37}}))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id = $1", success.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM account_groups WHERE account_id = $1", success.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", success.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", source.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})

	var schedulable bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT schedulable FROM accounts WHERE id = $1", success.ID).Scan(&schedulable))
	require.False(t, schedulable)
	var priority int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT priority FROM account_groups WHERE account_id = $1 AND group_id = $2", success.ID, group.ID).Scan(&priority))
	require.Equal(t, 37, priority)
	var outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id = $1", success.ID).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount)

	failure := &service.Account{
		Name:        fmt.Sprintf("duplicate-failure-%d", suffix),
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: false,
		Credentials: map[string]any{"api_key": "secret"},
		Extra:       map[string]any{},
	}
	err = repo.CreateDuplicateWithPlans(ctx, source.ID, failure, []service.AccountGroup{{GroupID: int64(^uint64(0) >> 1), Priority: 1}})
	require.Error(t, err)

	var accountCount, groupCount, failedOutboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts WHERE name = $1", failure.Name).Scan(&accountCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_groups WHERE account_id = $1", failure.ID).Scan(&groupCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id = $1", failure.ID).Scan(&failedOutboxCount))
	require.Zero(t, accountCount)
	require.Zero(t, groupCount)
	require.Zero(t, failedOutboxCount)
}

func Test复制账号测试计划隔离历史并确定恢复模板(t *testing.T) {
	for _, mode := range []string{"源自动计划优先", "普通恢复模板", "无计划默认模板"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			client := testEntClient(t)
			repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
			source, err := client.Account.Create().SetName(fmt.Sprintf("copy-plans-%d", time.Now().UnixNano())).
				SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
				SetCredentials(map[string]any{"api_key": "secret"}).Save(ctx)
			require.NoError(t, err)
			copy := &service.Account{Name: source.Name + "-copy", Platform: source.Platform, Type: source.Type,
				Status: service.StatusActive, Schedulable: false, Credentials: source.Credentials, Extra: map[string]any{}}
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id IN ($1, $2)", source.ID, copy.ID)
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id IN ($1, $2)", source.ID, copy.ID)
			})
			var sourcePlanIDs []int64
			if mode != "无计划默认模板" {
				for i, p := range []struct {
					model, prompt string
					recover, auto bool
				}{{"disabled-recovery", "不恢复", false, false}, {"first-manual", "最早模板", true, false}, {"later-manual", "后建模板", true, false}, {"auto-model", "系统模板", false, true}} {
					if p.auto && mode != "源自动计划优先" {
						continue
					}
					var id int64
					err = integrationDB.QueryRowContext(ctx, `
						INSERT INTO scheduled_test_plans (account_id, model_id, prompt, cron_expression, enabled, max_results,
							auto_recover, auto_managed, last_run_at, next_run_at, created_at, updated_at)
						VALUES ($1, $2, $3, '*/17 * * * *', true, 37, $4, $5, NOW(), NOW(), $6, NOW()) RETURNING id
					`, source.ID, p.model, p.prompt, p.recover, p.auto, time.Now().Add(time.Duration(i-10)*time.Hour)).Scan(&id)
					require.NoError(t, err)
					sourcePlanIDs = append(sourcePlanIDs, id)
					_, err = integrationDB.ExecContext(ctx, `INSERT INTO scheduled_test_results
						(plan_id, status, error_message, latency_ms, started_at, finished_at, created_at)
						VALUES ($1, 'failed', '源故障', 9, NOW(), NOW(), NOW())`, id)
					require.NoError(t, err)
				}
			}
			require.NoError(t, repo.CreateDuplicateWithPlans(ctx, source.ID, copy, nil))
			plans, err := (&scheduledTestPlanRepository{db: integrationDB}).ListByAccountID(ctx, copy.ID)
			require.NoError(t, err)
			wantCount := len(sourcePlanIDs)
			if mode != "源自动计划优先" {
				wantCount++
			}
			require.Len(t, plans, wantCount)
			autoCount := 0
			for _, p := range plans {
				require.NotContains(t, sourcePlanIDs, p.ID)
				require.False(t, p.Enabled)
				require.Nil(t, p.LastRunAt)
				require.Nil(t, p.NextRunAt)
				if !p.AutoManaged {
					require.Equal(t, p.ModelID != "disabled-recovery", p.AutoRecover)
					continue
				}
				autoCount++
				require.True(t, p.AutoRecover)
				switch mode {
				case "源自动计划优先":
					require.Equal(t, "auto-model", p.ModelID)
					require.Equal(t, "系统模板", p.Prompt)
				case "普通恢复模板":
					require.Equal(t, "first-manual", p.ModelID)
					require.Equal(t, "最早模板", p.Prompt)
				case "无计划默认模板":
					require.Empty(t, p.ModelID)
					require.Empty(t, p.Prompt)
				}
				if mode == "无计划默认模板" {
					require.Equal(t, "*/5 * * * *", p.CronExpression)
					require.Equal(t, 20, p.MaxResults)
				} else {
					require.Equal(t, "*/17 * * * *", p.CronExpression)
					require.Equal(t, 37, p.MaxResults)
				}
			}
			require.Equal(t, 1, autoCount)
			var results, enabledSource int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM scheduled_test_results r
				JOIN scheduled_test_plans p ON p.id = r.plan_id WHERE p.account_id = $1`, copy.ID).Scan(&results))
			require.Zero(t, results)
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduled_test_plans WHERE account_id = $1 AND enabled", source.ID).Scan(&enabledSource))
			require.Equal(t, len(sourcePlanIDs), enabledSource)
		})
	}
}

func Test复制计划写入失败回滚账号及分组(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	suffix := time.Now().UnixNano()
	source, err := client.Account.Create().SetName(fmt.Sprintf("copy-failure-%d", suffix)).
		SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "secret"}).Save(ctx)
	require.NoError(t, err)
	target := &service.Account{Name: source.Name + "-copy", Platform: source.Platform, Type: source.Type,
		Status: service.StatusActive, Credentials: source.Credentials, Extra: map[string]any{}}
	// 仅让此副本的计划插入失败，不影响其他并行集成测试。
	functionName := fmt.Sprintf("reject_copy_plan_%d", suffix)
	_, err = integrationDB.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger AS $$
		BEGIN
			IF EXISTS (SELECT 1 FROM accounts WHERE id = NEW.account_id AND name = '%s') THEN
				RAISE EXCEPTION '复制计划故障注入';
			END IF;
			RETURN NEW;
		END; $$ LANGUAGE plpgsql`, functionName, target.Name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON scheduled_test_plans", functionName))
		_, _ = integrationDB.ExecContext(context.Background(), fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", functionName))
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1 OR name = $2", source.ID, target.Name)
	})
	_, err = integrationDB.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON scheduled_test_plans
		FOR EACH ROW EXECUTE FUNCTION %s()`, functionName, functionName))
	require.NoError(t, err)
	require.ErrorContains(t, repo.CreateDuplicateWithPlans(ctx, source.ID, target, nil), "复制计划故障注入")
	for _, query := range []string{
		"SELECT COUNT(*) FROM accounts WHERE id = $1",
		"SELECT COUNT(*) FROM scheduled_test_plans WHERE account_id = $1",
		"SELECT COUNT(*) FROM scheduler_outbox WHERE account_id = $1",
	} {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx, query, target.ID).Scan(&count))
		require.Zero(t, count)
	}
}

func Test人工暂停阻止旧测活恢复且显式启用后可恢复(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	account, err := client.Account.Create().SetName(fmt.Sprintf("pause-recovery-%d", time.Now().UnixNano())).
		SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "secret"}).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id = $1", account.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
	})
	created, err := repo.PersistFailureSchedulingState(ctx, account.ID, map[string]any{"incident_id": "old-probe", "source": "first_token_timeout"}, time.Now())
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, repo.SetAdminSchedulable(ctx, account.ID, false))
	recovered, err := repo.RecoverFailureSchedulingState(ctx, account.ID, "old-probe")
	require.NoError(t, err)
	require.False(t, recovered)
	require.NoError(t, repo.SetSchedulable(ctx, account.ID, true))
	paused, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, paused.Schedulable)
	require.True(t, paused.IsManuallySchedulingPaused())
	plans, err := (&scheduledTestPlanRepository{db: integrationDB}).ListByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.False(t, plans[0].Enabled)
	require.Nil(t, plans[0].NextRunAt)
	require.NoError(t, repo.SetAdminSchedulable(ctx, account.ID, true))
	recovered, err = repo.RecoverFailureSchedulingState(ctx, account.ID, "old-probe")
	require.NoError(t, err)
	require.True(t, recovered)
}
