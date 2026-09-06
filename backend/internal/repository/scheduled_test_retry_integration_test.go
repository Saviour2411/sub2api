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

func Test自动测活退避不受历史保留数限制且不能覆盖并发变更(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	account, err := client.Account.Create().SetName(fmt.Sprintf("probe-retry-%d", time.Now().UnixNano())).
		SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "test"}).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id = $1", account.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
	})
	repo := &scheduledTestPlanRepository{db: integrationDB}
	resultRepo := &scheduledTestResultRepository{db: integrationDB}
	now := time.Now().UTC().Truncate(time.Microsecond)
	plan, err := repo.Create(ctx, &service.ScheduledTestPlan{AccountID: account.ID, CronExpression: "*/5 * * * *",
		Enabled: true, AutoManaged: true, AutoRecover: true, MaxResults: 1, NextRunAt: &now})
	require.NoError(t, err)
	steps := []time.Duration{7 * time.Minute, 7 * time.Minute, 11 * time.Minute, 13 * time.Minute}
	for _, wait := range []time.Duration{steps[0], steps[1], steps[2], steps[3], steps[3]} {
		_, err = resultRepo.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, Status: "failed", StartedAt: now, FinishedAt: now})
		require.NoError(t, err)
		require.NoError(t, resultRepo.PruneOldResults(ctx, plan.ID, 1))
		require.NoError(t, repo.ScheduleAutoManagedRetry(ctx, plan, now, steps))
		plan, err = repo.GetByID(ctx, plan.ID)
		require.NoError(t, err)
		require.Equal(t, wait, plan.NextRunAt.Sub(*plan.LastRunAt))
		results, err := resultRepo.ListByPlanID(ctx, plan.ID, 10)
		require.NoError(t, err)
		require.Len(t, results, 1)
		// 重复启用通知及旧扫描快照都不能将下一次探测提前。
		ensured, err := repo.EnsureAutoManaged(ctx, account.ID, true, &now)
		require.NoError(t, err)
		require.Equal(t, *plan.NextRunAt, *ensured.NextRunAt)
		require.NoError(t, repo.EnableAutoManaged(ctx, plan.ID, now))
		now = plan.NextRunAt.Add(time.Second)
	}
	// 调整二开退避仍按持久化次数使用末档，不因仅保留一条结果而回退。
	require.NoError(t, repo.RescheduleEnabledAutoManaged(ctx, []time.Duration{time.Minute, 20 * time.Minute}, *plan.LastRunAt))
	plan, err = repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, 20*time.Minute, plan.NextRunAt.Sub(*plan.LastRunAt))
	require.NoError(t, repo.ScheduleAutoManagedRetry(ctx, plan, now, steps))
	plan, err = repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, 13*time.Minute, plan.NextRunAt.Sub(*plan.LastRunAt))

	// 在途旧测试失败不能推迟新事故的立即测活。
	freshTime := now.Add(time.Second)
	require.NoError(t, repo.UpdateAfterRun(ctx, plan.ID, *plan.LastRunAt, freshTime))
	require.NoError(t, repo.ScheduleAutoManagedRetry(ctx, plan, now.Add(time.Minute), steps))
	fresh, err := repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, freshTime, *fresh.NextRunAt)

	accounts := newAccountRepositoryWithSQL(client, integrationDB, nil)
	require.NoError(t, accounts.SetAdminSchedulable(ctx, account.ID, false))
	require.NoError(t, repo.ScheduleAutoManagedRetry(ctx, fresh, now.Add(time.Minute), steps))
	paused, err := repo.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.False(t, paused.Enabled)
	require.Nil(t, paused.NextRunAt)
}
