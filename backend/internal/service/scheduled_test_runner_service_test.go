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
	rescheduledSteps     []time.Duration
	rescheduledAt        time.Time
}

type scheduledTestResultRepoStub struct {
	results []*ScheduledTestResult
	err     error
}

type scheduledAccountTesterStub struct {
	result *ScheduledTestResult
	err    error
}

type scheduledAccountRecoveryStub struct {
	incidentIDs       []string
	recoveryStartedAt []time.Time
	result            *SuccessfulTestRecoveryResult
	err               error
}

type scheduledPlanEnableCall struct {
	id        int64
	nextRunAt time.Time
}

type scheduledPlanDisableCall struct {
	id        int64
	lastRunAt *time.Time
}

func (s *scheduledAccountTesterStub) RunTestBackground(_ context.Context, _ int64, _ string, _ ...string) (*ScheduledTestResult, error) {
	return s.result, s.err
}

func (s *scheduledAccountRecoveryStub) HandleStrictFailureScheduling(_ context.Context, _ *Account, _ int, _ string) bool {
	return false
}

func (s *scheduledAccountRecoveryStub) RecoverAccountAfterSuccessfulTestIncident(
	_ context.Context,
	_ int64,
	incidentID string,
	recoveryStartedAt ...time.Time,
) (*SuccessfulTestRecoveryResult, error) {
	s.incidentIDs = append(s.incidentIDs, incidentID)
	if len(recoveryStartedAt) > 0 {
		s.recoveryStartedAt = append(s.recoveryStartedAt, recoveryStartedAt[0])
	}
	return s.result, s.err
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

func (r *scheduledTestPlanRepoStub) EnsureAutoManaged(_ context.Context, accountID int64, enabled bool, nextRunAt *time.Time) (*ScheduledTestPlan, error) {
	plan := &ScheduledTestPlan{
		ID:             100,
		AccountID:      accountID,
		CronExpression: "*/5 * * * *",
		Enabled:        enabled,
		MaxResults:     20,
		AutoRecover:    true,
		AutoManaged:    true,
		NextRunAt:      nextRunAt,
	}
	r.createdPlan = plan
	if r.createCh != nil {
		r.createCh <- plan
	}
	return plan, nil
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

func (r *scheduledTestPlanRepoStub) RescheduleEnabledAutoManaged(_ context.Context, steps []time.Duration, now time.Time) error {
	r.rescheduledSteps = append([]time.Duration(nil), steps...)
	r.rescheduledAt = now
	return nil
}

func (r *scheduledTestResultRepoStub) Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	return nil, errors.New("unexpected Create call")
}

func (r *scheduledTestResultRepoStub) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error) {
	if r.err != nil {
		return nil, r.err
	}
	if limit > 0 && len(r.results) > limit {
		return r.results[:limit], nil
	}
	return r.results, nil
}

func (r *scheduledTestResultRepoStub) ListLatestFailuresByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*ScheduledTestLatestFailure, error) {
	return nil, errors.New("unexpected ListLatestFailuresByAccountIDs call")
}

func (r *scheduledTestResultRepoStub) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	return errors.New("unexpected PruneOldResults call")
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
		require.Equal(t, "*/5 * * * *", plan.CronExpression)
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
			{ID: 1, AccountID: 11, CronExpression: "*/5 * * * *", AutoManaged: true},
			{ID: 2, AccountID: 12, CronExpression: "*/5 * * * *", AutoManaged: true},
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
	require.Equal(t, now, repo.enabled[0].nextRunAt)
}

func TestScheduledTestRunner_AutoManagedBackoffDuration(t *testing.T) {
	require.Equal(t, 5*time.Minute, scheduledTestAutoManagedBackoffDuration(0))
	require.Equal(t, 5*time.Minute, scheduledTestAutoManagedBackoffDuration(1))
	require.Equal(t, 10*time.Minute, scheduledTestAutoManagedBackoffDuration(2))
	require.Equal(t, 15*time.Minute, scheduledTestAutoManagedBackoffDuration(3))
	require.Equal(t, 30*time.Minute, scheduledTestAutoManagedBackoffDuration(4))
	require.Equal(t, 60*time.Minute, scheduledTestAutoManagedBackoffDuration(5))
	require.Equal(t, 60*time.Minute, scheduledTestAutoManagedBackoffDuration(6))
}

func TestScheduledTestRunner_NextAutoManagedRetryUsesConsecutiveFailures(t *testing.T) {
	now := time.Now()
	resultRepo := &scheduledTestResultRepoStub{results: []*ScheduledTestResult{
		{Status: "failed"},
		{Status: "failed"},
		{Status: "success"},
		{Status: "failed"},
	}}
	runner := &ScheduledTestRunnerService{
		scheduledSvc: NewScheduledTestService(nil, resultRepo),
	}

	nextRun := runner.nextAutoManagedRetry(context.Background(), &ScheduledTestPlan{ID: 1, AutoManaged: true}, now)

	require.Equal(t, now.Add(10*time.Minute), nextRun)
}

func TestScheduledTestRunner_NextAutoManagedRetryUsesGatewayConfiguration(t *testing.T) {
	now := time.Now()
	settingRepo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyGatewayAutoManagedProbeBackoffMinutes: `[7,11]`,
	}}
	runner := &ScheduledTestRunnerService{
		scheduledSvc: NewScheduledTestService(nil, &scheduledTestResultRepoStub{results: []*ScheduledTestResult{
			{Status: "failed"},
			{Status: "failed"},
		}}),
		settingService: NewSettingService(settingRepo, nil),
	}

	nextRun := runner.nextAutoManagedRetry(context.Background(), &ScheduledTestPlan{ID: 1, AutoManaged: true}, now)
	require.Equal(t, now.Add(11*time.Minute), nextRun)
}

func TestScheduledTestRunner_EnsureAutoManagedProbeEnablesImmediately(t *testing.T) {
	repo := &scheduledTestPlanRepoStub{}
	runner := &ScheduledTestRunnerService{planRepo: repo}

	require.NoError(t, runner.EnsureAutoManagedProbe(context.Background(), 42))
	require.NotNil(t, repo.createdPlan)
	require.Equal(t, int64(42), repo.createdPlan.AccountID)
	require.True(t, repo.createdPlan.Enabled)
	require.NotNil(t, repo.createdPlan.NextRunAt)
}

func TestScheduledTestRunner_CapturesIncidentBeforeAutoManagedProbe(t *testing.T) {
	accountRepo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
		42: {
			ID: 42,
			Extra: map[string]any{
				accountFailureStrategyUnscheduledKey: map[string]any{
					accountFailureStrategyUnscheduledSourceKey:     "first_token_timeout",
					accountFailureStrategyUnscheduledIncidentIDKey: "incident-before-probe",
				},
			},
		},
	}}
	runner := &ScheduledTestRunnerService{accountRepo: accountRepo}

	incidentID, ok := runner.captureRecoveryIncident(context.Background(), &ScheduledTestPlan{
		ID:          7,
		AccountID:   42,
		AutoManaged: true,
	})
	require.True(t, ok)
	require.Equal(t, "incident-before-probe", incidentID)
}

func TestScheduledTestRunner_CapturesIncidentBeforeManualAutoRecoverProbe(t *testing.T) {
	accountRepo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
		42: {
			ID: 42,
			Extra: map[string]any{
				accountFailureStrategyUnscheduledKey: map[string]any{
					accountFailureStrategyUnscheduledSourceKey:     "upstream_error",
					accountFailureStrategyUnscheduledIncidentIDKey: "manual-incident",
				},
			},
		},
	}}
	runner := &ScheduledTestRunnerService{accountRepo: accountRepo}

	incidentID, ok := runner.captureRecoveryIncident(context.Background(), &ScheduledTestPlan{
		ID:          8,
		AccountID:   42,
		AutoRecover: true,
		AutoManaged: false,
	})

	require.True(t, ok)
	require.Equal(t, "manual-incident", incidentID)
}

func TestScheduledTestRunner_IncidentRecoveryDoesNotDisablePlanTwice(t *testing.T) {
	planRepo := &scheduledTestPlanRepoStub{}
	accountRepo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{
		42: {
			ID:          42,
			Status:      StatusActive,
			Schedulable: false,
			Extra: map[string]any{
				accountFailureStrategyUnscheduledKey: map[string]any{
					accountFailureStrategyUnscheduledSourceKey:     "first_token_timeout",
					accountFailureStrategyUnscheduledIncidentIDKey: "incident-before-probe",
				},
			},
		},
	}}
	recovery := &scheduledAccountRecoveryStub{result: &SuccessfulTestRecoveryResult{ClearedRateLimit: true}}
	runner := &ScheduledTestRunnerService{
		planRepo:       planRepo,
		scheduledSvc:   NewScheduledTestService(planRepo, &scheduledTestResultRepoStub{}),
		accountTestSvc: &scheduledAccountTesterStub{result: &ScheduledTestResult{Status: "success"}},
		rateLimitSvc:   recovery,
		accountRepo:    accountRepo,
	}

	runner.runOnePlan(context.Background(), &ScheduledTestPlan{
		ID:             7,
		AccountID:      42,
		CronExpression: "*/5 * * * *",
		AutoRecover:    true,
		AutoManaged:    true,
		MaxResults:     20,
	})

	require.Equal(t, []string{"incident-before-probe"}, recovery.incidentIDs)
	require.Len(t, recovery.recoveryStartedAt, 1)
	require.False(t, recovery.recoveryStartedAt[0].IsZero())
	require.Empty(t, planRepo.disabled)
}

func TestScheduledTestRunner_NextAutoManagedRetryFallsBackToFiveMinutes(t *testing.T) {
	now := time.Now()
	runner := &ScheduledTestRunnerService{
		scheduledSvc: NewScheduledTestService(nil, &scheduledTestResultRepoStub{err: errors.New("db unavailable")}),
	}

	nextRun := runner.nextAutoManagedRetry(context.Background(), &ScheduledTestPlan{ID: 1, AutoManaged: true}, now)

	require.Equal(t, now.Add(5*time.Minute), nextRun)
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
