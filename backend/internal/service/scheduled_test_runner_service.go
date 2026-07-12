package service

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const scheduledTestDefaultMaxWorkers = 10

var scheduledTestAutoManagedBackoffSteps = []time.Duration{
	5 * time.Minute,
	10 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	60 * time.Minute,
}

type scheduledAccountTester interface {
	RunTestBackground(ctx context.Context, accountID int64, modelID string, prompt ...string) (*ScheduledTestResult, error)
}

type scheduledAccountRecovery interface {
	HandleStrictFailureScheduling(ctx context.Context, account *Account, statusCode int, reason string) bool
	RecoverAccountAfterSuccessfulTestIncident(ctx context.Context, accountID int64, incidentID string, recoveryStartedAt ...time.Time) (*SuccessfulTestRecoveryResult, error)
}

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo       ScheduledTestPlanRepository
	scheduledSvc   *ScheduledTestService
	accountTestSvc scheduledAccountTester
	rateLimitSvc   scheduledAccountRecovery
	accountRepo    AccountRepository
	cfg            *config.Config
	settingService *SettingService

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
}

// SetSettingService 注入网关配置读取服务。
func (s *ScheduledTestRunnerService) SetSettingService(settingService *SettingService) {
	if s != nil {
		s.settingService = settingService
	}
}

// EnsureAutoManagedProbe 确保账号存在唯一的自动测活计划，并立即启用。
func (s *ScheduledTestRunnerService) EnsureAutoManagedProbe(ctx context.Context, accountID int64) error {
	if s == nil || s.planRepo == nil || accountID <= 0 {
		return nil
	}
	now := time.Now()
	_, err := s.planRepo.EnsureAutoManaged(ctx, accountID, true, &now)
	return err
}

// NewScheduledTestRunnerService creates a new runner.
func NewScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	accountRepo AccountRepository,
	cfg *config.Config,
) *ScheduledTestRunnerService {
	return &ScheduledTestRunnerService{
		planRepo:       planRepo,
		scheduledSvc:   scheduledSvc,
		accountTestSvc: accountTestSvc,
		rateLimitSvc:   rateLimitSvc,
		accountRepo:    accountRepo,
		cfg:            cfg,
	}
}

// Start begins the cron ticker (every minute).
func (s *ScheduledTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *ScheduledTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledTestRunnerService) runScheduled() {
	// Delay 10s so execution lands at ~:10 of each minute instead of :00.
	time.Sleep(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	now := time.Now()
	s.activateAutoManagedPlans(ctx, now)

	plans, err := s.planRepo.ListDue(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] found %d due plans", len(plans))

	sem := make(chan struct{}, scheduledTestDefaultMaxWorkers)
	var wg sync.WaitGroup

	for _, plan := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *ScheduledTestPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlan(ctx, p)
		}(plan)
	}

	wg.Wait()
}

func (s *ScheduledTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledTestPlan) {
	recoveryStartedAt := time.Now().UTC()
	incidentID, recoverySnapshotOK := s.captureRecoveryIncident(ctx, plan)
	result, err := s.accountTestSvc.RunTestBackground(ctx, plan.AccountID, plan.ModelID, plan.Prompt)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d RunTestBackground error: %v", plan.ID, err)
		return
	}

	if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
	}

	if result.Status == "failed" {
		s.handleFailedTest(ctx, plan, result)
		if plan.AutoManaged {
			now := time.Now()
			nextRun := s.nextAutoManagedRetry(ctx, plan, now)
			if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, now, nextRun); err != nil {
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
			}
			return
		}
	} else if result.Status == "success" && plan.AutoRecover {
		if recoverySnapshotOK {
			s.tryRecoverAccount(ctx, plan.AccountID, plan.ID, incidentID, recoveryStartedAt)
		}
	}

	if result.Status == "success" && plan.AutoManaged {
		if plan.AutoRecover && recoverySnapshotOK && incidentID != "" {
			// 带事故 ID 的恢复事务已原子停用计划；CAS 未命中或恢复失败时
			// 必须保留计划，不能让旧测活结果随后停用并发新事故的计划。
			return
		}
		finishedAt := time.Now()
		if s.disableAutoManagedAfterSuccess(ctx, plan, finishedAt) {
			return
		}
	}

	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}

	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

func (s *ScheduledTestRunnerService) captureRecoveryIncident(ctx context.Context, plan *ScheduledTestPlan) (string, bool) {
	if plan == nil || (!plan.AutoManaged && !plan.AutoRecover) {
		return "", true
	}
	if s == nil || s.accountRepo == nil {
		return "", false
	}
	account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d 捕获恢复事故失败: %v", plan.ID, err)
		return "", false
	}
	incidentID, _ := account.FailureStrategyUnscheduledIncident()
	return incidentID, true
}

func (s *ScheduledTestRunnerService) activateAutoManagedPlans(ctx context.Context, now time.Time) {
	if s == nil || s.planRepo == nil || s.accountRepo == nil {
		return
	}
	plans, err := s.planRepo.ListAutoManagedActivationCandidates(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListAutoManagedActivationCandidates error: %v", err)
		return
	}
	for _, plan := range plans {
		if plan == nil || !plan.AutoManaged || plan.Enabled {
			continue
		}
		account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-managed account read failed: %v", plan.ID, err)
			continue
		}
		if !isAutoManagedProbeNeeded(account, now) {
			continue
		}
		nextRun := now
		if err := s.planRepo.EnableAutoManaged(ctx, plan.ID, nextRun); err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-managed enable error: %v", plan.ID, err)
			continue
		}
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-managed enabled for account=%d next_run_at=%s", plan.ID, plan.AccountID, nextRun.Format(time.RFC3339))
	}
}

func (s *ScheduledTestRunnerService) disableAutoManagedAfterSuccess(ctx context.Context, plan *ScheduledTestPlan, finishedAt time.Time) bool {
	if s == nil || s.planRepo == nil || s.accountRepo == nil || plan == nil || !plan.AutoManaged {
		return false
	}
	if disabler, ok := s.planRepo.(interface {
		DisableAutoManagedIfAccountHealthy(context.Context, int64, int64, *time.Time, time.Time) (bool, error)
	}); ok {
		disabled, err := disabler.DisableAutoManagedIfAccountHealthy(
			ctx,
			plan.ID,
			plan.AccountID,
			&finishedAt,
			time.Now(),
		)
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d 条件停用自动测活计划失败: %v", plan.ID, err)
			return false
		}
		if disabled {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d 账号恢复成功后已停用自动测活计划", plan.ID)
		}
		return disabled
	}
	account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-managed recovery account read failed: %v", plan.ID, err)
		return false
	}
	if isAutoManagedProbeNeeded(account, time.Now()) {
		return false
	}
	if err := s.planRepo.DisableAutoManaged(ctx, plan.ID, &finishedAt); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-managed disable error: %v", plan.ID, err)
		return false
	}
	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-managed disabled after successful recovery", plan.ID)
	return true
}

func (s *ScheduledTestRunnerService) nextAutoManagedRetry(ctx context.Context, plan *ScheduledTestPlan, from time.Time) time.Time {
	if from.IsZero() {
		from = time.Now()
	}
	consecutiveFailures := 1
	backoffSteps := s.autoManagedBackoffSteps(ctx)
	if s != nil && s.scheduledSvc != nil && plan != nil {
		results, err := s.scheduledSvc.ListResults(ctx, plan.ID, len(backoffSteps)+1)
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d ListResults for backoff error: %v", plan.ID, err)
		} else {
			consecutiveFailures = countConsecutiveScheduledTestFailures(results)
			if consecutiveFailures <= 0 {
				consecutiveFailures = 1
			}
		}
	}
	return from.Add(scheduledTestAutoManagedBackoffDuration(consecutiveFailures, backoffSteps))
}

func (s *ScheduledTestRunnerService) autoManagedBackoffSteps(ctx context.Context) []time.Duration {
	if s != nil && s.settingService != nil {
		steps := s.settingService.GetGatewayRuntime(ctx).AutoManagedProbeBackoffDurations()
		if len(steps) > 0 {
			return steps
		}
	}
	return append([]time.Duration(nil), scheduledTestAutoManagedBackoffSteps...)
}

func countConsecutiveScheduledTestFailures(results []*ScheduledTestResult) int {
	count := 0
	for _, result := range results {
		if result == nil || result.Status != "failed" {
			break
		}
		count++
	}
	return count
}

func scheduledTestAutoManagedBackoffDuration(consecutiveFailures int, configured ...[]time.Duration) time.Duration {
	steps := scheduledTestAutoManagedBackoffSteps
	if len(configured) > 0 && len(configured[0]) > 0 {
		steps = configured[0]
	}
	if consecutiveFailures <= 1 {
		return steps[0]
	}
	index := consecutiveFailures - 1
	if index >= len(steps) {
		index = len(steps) - 1
	}
	return steps[index]
}

func isAutoManagedProbeNeeded(account *Account, now time.Time) bool {
	if account == nil {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if account.Status == StatusError || account.HasFailureStrategyUnscheduledMarker() {
		return true
	}
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now) {
		return true
	}
	if account.OverloadUntil != nil && account.OverloadUntil.After(now) {
		return true
	}
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(now) {
		return true
	}
	return false
}

func (s *ScheduledTestRunnerService) handleFailedTest(ctx context.Context, plan *ScheduledTestPlan, result *ScheduledTestResult) {
	if s.rateLimitSvc == nil || plan == nil || result == nil {
		return
	}
	reason := strings.TrimSpace(result.ErrorMessage)
	if reason == "" {
		reason = "scheduled test failed"
	}
	account := &Account{ID: plan.AccountID}
	if s.rateLimitSvc.HandleStrictFailureScheduling(ctx, account, scheduledTestFailureStatusCode(reason), reason) {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d marked account=%d unschedulable after failed test", plan.ID, plan.AccountID)
	}
}

func scheduledTestFailureStatusCode(errorMessage string) int {
	const prefix = "API returned "
	idx := strings.Index(errorMessage, prefix)
	if idx < 0 {
		return http.StatusBadGateway
	}
	rest := strings.TrimSpace(errorMessage[idx+len(prefix):])
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return http.StatusBadGateway
	}
	statusCode, err := strconv.Atoi(rest[:end])
	if err != nil || statusCode <= 0 {
		return http.StatusBadGateway
	}
	return statusCode
}

// tryRecoverAccount attempts to recover an account from recoverable runtime state.
func (s *ScheduledTestRunnerService) tryRecoverAccount(
	ctx context.Context,
	accountID int64,
	planID int64,
	incidentID string,
	recoveryStartedAt time.Time,
) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTestIncident(ctx, accountID, incidentID, recoveryStartedAt)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover failed: %v", planID, err)
		return
	}
	if recovery == nil {
		return
	}

	if recovery.ClearedError {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d recovered from error status", planID, accountID)
	}
	if recovery.ClearedRateLimit {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d cleared rate-limit/runtime state", planID, accountID)
	}
}
