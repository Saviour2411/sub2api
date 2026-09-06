//go:build unit

package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledTesterFunc func(context.Context, int64, string, ...string) (*ScheduledTestResult, error)

func (f scheduledTesterFunc) RunTestBackground(ctx context.Context, accountID int64, model string, prompt ...string) (*ScheduledTestResult, error) {
	return f(ctx, accountID, model, prompt...)
}

type recoveryPlanStore struct {
	scheduledTestPlanRepoStub
	plans          map[int64]*ScheduledTestPlan
	ensuredEnabled []bool
}

func (r *recoveryPlanStore) GetByID(_ context.Context, id int64) (*ScheduledTestPlan, error) {
	if p := r.plans[id]; p != nil {
		copy := *p
		return &copy, nil
	}
	return nil, nil
}

func (r *recoveryPlanStore) EnsureAutoManaged(_ context.Context, accountID int64, enabled bool, next *time.Time) (*ScheduledTestPlan, error) {
	r.ensuredEnabled = append(r.ensuredEnabled, enabled)
	for _, p := range r.plans {
		if p.AccountID == accountID && p.AutoManaged {
			return p, nil
		}
	}
	p := &ScheduledTestPlan{ID: 99, AccountID: accountID, AutoManaged: true, AutoRecover: true, Enabled: enabled, NextRunAt: next}
	r.plans[p.ID] = p
	return p, nil
}

func (r *recoveryPlanStore) UpdateAfterRun(_ context.Context, id int64, last, next time.Time) error {
	r.plans[id].LastRunAt = &last
	r.plans[id].NextRunAt = &next
	return nil
}

func (r *recoveryPlanStore) DeferOrdinaryPlan(ctx context.Context, id int64, next time.Time) error {
	r.plans[id].NextRunAt = &next
	return r.scheduledTestPlanRepoStub.DeferOrdinaryPlan(ctx, id, next)
}

func dueRecoveryPlan(id int64, auto bool) *ScheduledTestPlan {
	due := time.Now().Add(-time.Minute)
	last := due.Add(-time.Hour)
	return &ScheduledTestPlan{ID: id, AccountID: 42, Enabled: true, AutoManaged: auto,
		AutoRecover: true, CronExpression: "*/5 * * * *", LastRunAt: &last, NextRunAt: &due, MaxResults: 20}
}

func Test故障期间普通计划不绕过退避或伪造测试记录(t *testing.T) {
	ordinary, auto := dueRecoveryPlan(1, false), dueRecoveryPlan(2, true)
	backoff := time.Now().Add(time.Hour)
	auto.NextRunAt = &backoff
	lastOrdinary := *ordinary.LastRunAt
	store := &recoveryPlanStore{plans: map[int64]*ScheduledTestPlan{1: ordinary, 2: auto}}
	results := &firstTokenRecoveryResultRepoStub{}
	recovery := &scheduledAccountRecoveryStub{}
	runner := &ScheduledTestRunnerService{
		planRepo: store, scheduledSvc: NewScheduledTestService(store, results), rateLimitSvc: recovery,
		accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{42: {ID: 42, Status: StatusError}}},
		accountTestSvc: scheduledTesterFunc(func(context.Context, int64, string, ...string) (*ScheduledTestResult, error) {
			t.Fatal("故障期间普通计划不应发送测试请求")
			return nil, nil
		}),
	}
	runner.runDuePlan(context.Background(), ordinary)
	require.Len(t, store.deferred, 1)
	require.True(t, ordinary.Enabled)
	require.Equal(t, lastOrdinary, *ordinary.LastRunAt)
	require.True(t, ordinary.NextRunAt.After(time.Now()))
	require.Equal(t, backoff, *auto.NextRunAt)
	require.Equal(t, []bool{false}, store.ensuredEnabled)
	require.Empty(t, results.results)
	require.Empty(t, recovery.incidentIDs)

	// 健康恢复后仍是原普通计划，按其 Cron 正常执行。
	runner.accountRepo = &mockAccountRepoForGemini{accountsByID: map[int64]*Account{42: {ID: 42, Status: StatusActive, Schedulable: true}}}
	ordinary.AutoRecover = false
	due := time.Now().Add(-time.Minute)
	ordinary.NextRunAt = &due
	runner.accountTestSvc = &scheduledAccountTesterStub{result: &ScheduledTestResult{Status: "success"}}
	runner.runDuePlan(context.Background(), ordinary)
	require.Len(t, results.results, 1)
	require.Equal(t, "success", results.results[0].Status)
	require.True(t, ordinary.LastRunAt.After(lastOrdinary))
	require.Equal(t, backoff, *auto.NextRunAt)
}

func Test定时测活同账号互斥并复核旧到期快照(t *testing.T) {
	auto, ordinary := dueRecoveryPlan(1, true), dueRecoveryPlan(2, false)
	store := &recoveryPlanStore{plans: map[int64]*ScheduledTestPlan{1: auto, 2: ordinary}}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var calls atomic.Int32
	runner := &ScheduledTestRunnerService{
		planRepo: store, scheduledSvc: NewScheduledTestService(store, &firstTokenRecoveryResultRepoStub{}),
		accountTestSvc: scheduledTesterFunc(func(context.Context, int64, string, ...string) (*ScheduledTestResult, error) {
			calls.Add(1)
			close(entered)
			<-release
			return &ScheduledTestResult{Status: "failed"}, nil
		}),
	}
	stale := *auto
	go func() { defer close(done); runner.runDuePlan(context.Background(), &stale) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("测活未开始")
	}
	runner.runDuePlan(context.Background(), ordinary)
	require.Equal(t, int32(1), calls.Load())
	once.Do(func() { close(release) })
	<-done
	require.True(t, auto.NextRunAt.After(time.Now()))
	runner.runDuePlan(context.Background(), &stale)
	ordinary.Enabled = false
	runner.runDuePlan(context.Background(), ordinary)
	require.Equal(t, int32(1), calls.Load())
}

func Test自动测活按配置递进退避且普通计划不能提前重排(t *testing.T) {
	auto, ordinary := dueRecoveryPlan(1, true), dueRecoveryPlan(2, false)
	store := &recoveryPlanStore{plans: map[int64]*ScheduledTestPlan{1: auto, 2: ordinary}}
	settings := &SettingService{}
	runtime := DefaultGatewaySettings()
	runtime.AutoManagedProbeBackoffMinutes = []int{7, 11}
	settings.storeGatewaySettingsCache(runtime, time.Hour)
	runner := &ScheduledTestRunnerService{
		planRepo: store, settingService: settings,
		scheduledSvc:   NewScheduledTestService(store, &firstTokenRecoveryResultRepoStub{}),
		accountRepo:    &mockAccountRepoForGemini{accountsByID: map[int64]*Account{42: {ID: 42, Status: StatusError}}},
		accountTestSvc: &scheduledAccountTesterStub{result: &ScheduledTestResult{Status: "failed"}},
	}
	for _, wait := range []time.Duration{7 * time.Minute, 11 * time.Minute, 11 * time.Minute} {
		due := time.Now().Add(-time.Minute)
		auto.NextRunAt, ordinary.NextRunAt = &due, &due
		before := time.Now()
		runner.runDuePlan(context.Background(), auto)
		require.WithinDuration(t, before.Add(wait), *auto.NextRunAt, 2*time.Second)
		next := *auto.NextRunAt
		runner.runDuePlan(context.Background(), ordinary)
		require.Equal(t, next, *auto.NextRunAt)
	}
}
