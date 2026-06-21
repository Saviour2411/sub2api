//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledTestPlanRepoStub struct {
	createdPlan          *ScheduledTestPlan
	createCh             chan *ScheduledTestPlan
	activationCandidates []*ScheduledTestPlan
	enabled              []scheduledPlanEnableCall
	disabled             []scheduledPlanDisableCall
}

type scheduledPlanEnableCall struct {
	id        int64
	nextRunAt time.Time
}

type scheduledPlanDisableCall struct {
	id        int64
	lastRunAt *time.Time
}

func (r *scheduledTestPlanRepoStub) Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	cp := *plan
	cp.ID = 100
	r.createdPlan = &cp
	if r.createCh != nil {
		r.createCh <- &cp
	}
	return &cp, nil
}

func (r *scheduledTestPlanRepoStub) GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	return nil, errors.New("unexpected GetByID call")
}

func (r *scheduledTestPlanRepoStub) ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	return nil, errors.New("unexpected ListByAccountID call")
}

func (r *scheduledTestPlanRepoStub) ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error) {
	return nil, errors.New("unexpected ListDue call")
}

func (r *scheduledTestPlanRepoStub) ListAutoManagedActivationCandidates(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error) {
	return r.activationCandidates, nil
}

func (r *scheduledTestPlanRepoStub) Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return nil, errors.New("unexpected Update call")
}

func (r *scheduledTestPlanRepoStub) Delete(ctx context.Context, id int64) error {
	return errors.New("unexpected Delete call")
}

func (r *scheduledTestPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	return errors.New("unexpected UpdateAfterRun call")
}

func (r *scheduledTestPlanRepoStub) EnableAutoManaged(ctx context.Context, id int64, nextRunAt time.Time) error {
	r.enabled = append(r.enabled, scheduledPlanEnableCall{id: id, nextRunAt: nextRunAt})
	return nil
}

func (r *scheduledTestPlanRepoStub) DisableAutoManaged(ctx context.Context, id int64, lastRunAt *time.Time) error {
	r.disabled = append(r.disabled, scheduledPlanDisableCall{id: id, lastRunAt: lastRunAt})
	return nil
}

func TestCreateDefaultScheduledTestPlanAsync_CreatesDisabledAutoManagedPlan(t *testing.T) {
	repo := &scheduledTestPlanRepoStub{createCh: make(chan *ScheduledTestPlan, 1)}
	svc := &adminServiceImpl{defaultScheduledTestPlanRepo: repo}

	svc.createDefaultScheduledTestPlanAsync(&Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	})

	select {
	case plan := <-repo.createCh:
		require.Equal(t, int64(42), plan.AccountID)
		require.Equal(t, "0 * * * *", plan.CronExpression)
		require.False(t, plan.Enabled)
		require.True(t, plan.AutoRecover)
		require.True(t, plan.AutoManaged)
		require.Nil(t, plan.NextRunAt)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for default scheduled test plan creation")
	}
}

func TestIsAutoManagedProbeNeeded(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	require.True(t, isAutoManagedProbeNeeded(&Account{Status: StatusError}, now))
	require.True(t, isAutoManagedProbeNeeded(&Account{Status: StatusActive, Extra: map[string]any{
		accountFailureStrategyUnscheduledKey: map[string]any{"reason": "upstream"},
	}}, now))
	require.True(t, isAutoManagedProbeNeeded(&Account{Status: StatusActive, RateLimitResetAt: &future}, now))
	require.True(t, isAutoManagedProbeNeeded(&Account{Status: StatusActive, OverloadUntil: &future}, now))
	require.True(t, isAutoManagedProbeNeeded(&Account{Status: StatusActive, TempUnschedulableUntil: &future}, now))
	require.False(t, isAutoManagedProbeNeeded(&Account{Status: StatusActive, Schedulable: false}, now))
	require.False(t, isAutoManagedProbeNeeded(&Account{Status: StatusActive, RateLimitResetAt: &past}, now))
}

func TestScheduledTestRunner_ActivateAutoManagedPlansOnlyForRecoverableState(t *testing.T) {
	now := time.Now()
	repo := &scheduledTestPlanRepoStub{
		activationCandidates: []*ScheduledTestPlan{
			{ID: 1, AccountID: 11, CronExpression: "0 * * * *", AutoManaged: true},
			{ID: 2, AccountID: 12, CronExpression: "0 * * * *", AutoManaged: true},
		},
	}
	accountRepo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
		11: {ID: 11, Status: StatusError, Schedulable: false},
		12: {ID: 12, Status: StatusActive, Schedulable: false},
	}}
	runner := &ScheduledTestRunnerService{planRepo: repo, accountRepo: accountRepo}

	runner.activateAutoManagedPlans(context.Background(), now)

	require.Len(t, repo.enabled, 1)
	require.Equal(t, int64(1), repo.enabled[0].id)
	require.True(t, repo.enabled[0].nextRunAt.After(now))
}

func TestScheduledTestRunner_DisableAutoManagedAfterSuccessfulRecovery(t *testing.T) {
	repo := &scheduledTestPlanRepoStub{}
	accountRepo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
		11: {ID: 11, Status: StatusActive, Schedulable: true},
	}}
	runner := &ScheduledTestRunnerService{planRepo: repo, accountRepo: accountRepo}

	disabled := runner.disableAutoManagedAfterSuccess(context.Background(), &ScheduledTestPlan{
		ID:          1,
		AccountID:   11,
		AutoManaged: true,
	}, time.Now())

	require.True(t, disabled)
	require.Len(t, repo.disabled, 1)
	require.Equal(t, int64(1), repo.disabled[0].id)
	require.NotNil(t, repo.disabled[0].lastRunAt)
}

func TestScheduledTestRunner_KeepsAutoManagedEnabledWhenRecoveryStateRemains(t *testing.T) {
	repo := &scheduledTestPlanRepoStub{}
	accountRepo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
		11: {ID: 11, Status: StatusError, Schedulable: false},
	}}
	runner := &ScheduledTestRunnerService{planRepo: repo, accountRepo: accountRepo}

	disabled := runner.disableAutoManagedAfterSuccess(context.Background(), &ScheduledTestPlan{
		ID:          1,
		AccountID:   11,
		AutoManaged: true,
	}, time.Now())

	require.False(t, disabled)
	require.Empty(t, repo.disabled)
}
