package service

import (
	"context"
	"log/slog"
)

func (s *adminServiceImpl) createDefaultScheduledTestPlanAsync(account *Account) {
	if s == nil || s.defaultScheduledTestPlanRepo == nil || account == nil || account.ID <= 0 {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("create_default_scheduled_test_plan_panic", "account_id", account.ID, "recover", r)
			}
		}()
		_, err := s.defaultScheduledTestPlanRepo.Create(context.Background(), &ScheduledTestPlan{
			AccountID:      account.ID,
			CronExpression: "*/5 * * * *",
			Enabled:        false,
			MaxResults:     20,
			AutoRecover:    true,
			AutoManaged:    true,
		})
		if err != nil {
			slog.Warn("create_default_scheduled_test_plan_failed", "account_id", account.ID, "error", err)
		}
	}()
}
