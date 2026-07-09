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
	RecoverAccountAfterSuccessfulTest(ctx context.Context, accountID int64) (*SuccessfulTestRecoveryResult, error)
}

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo       ScheduledTestPlanRepository
	scheduledSvc   *ScheduledTestService
	accountTestSvc scheduledAccountTester
	rateLimitSvc   scheduledAccountRecovery
	accountRepo    AccountRepository
	cfg            *config.Config

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
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
		s.tryRecoverAccount(ctx, plan.AccountID, plan.ID)
	}

	if result.Status == "success" && plan.AutoManaged {
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
	if s != nil && s.scheduledSvc != nil && plan != nil {
		results, err := s.scheduledSvc.ListResults(ctx, plan.ID, len(scheduledTestAutoManagedBackoffSteps)+1)
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d ListResults for backoff error: %v", plan.ID, err)
		} else {
			consecutiveFailures = countConsecutiveScheduledTestFailures(results)
			if consecutiveFailures <= 0 {
				consecutiveFailures = 1
			}
		}
	}
	return from.Add(scheduledTestAutoManagedBackoffDuration(consecutiveFailures))
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

func scheduledTestAutoManagedBackoffDuration(consecutiveFailures int) time.Duration {
	if consecutiveFailures <= 1 {
		return scheduledTestAutoManagedBackoffSteps[0]
	}
	index := consecutiveFailures - 1
	if index >= len(scheduledTestAutoManagedBackoffSteps) {
		index = len(scheduledTestAutoManagedBackoffSteps) - 1
	}
	return scheduledTestAutoManagedBackoffSteps[index]
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
func (s *ScheduledTestRunnerService) tryRecoverAccount(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, accountID)
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
