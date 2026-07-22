package service

import (
	"context"
	"log/slog"
)

// IsBalanceRechargeBonusDisabled 返回当前用户是否应跳过余额充值赠送。
func (s *PaymentService) IsBalanceRechargeBonusDisabled(ctx context.Context, userID int64) bool {
	if s == nil || s.settingService == nil {
		return false
	}
	settings := s.settingService.GetGatewayRuntime(ctx)
	if !settings.DisableRechargeBonusForCustomRateUsers {
		return false
	}
	if s.userGroupRateRepo == nil {
		slog.Warn("专属倍率仓储未配置，余额充值按不返利处理", "user_id", userID)
		return true
	}
	rates, err := s.userGroupRateRepo.GetByUserID(ctx, userID)
	if err != nil {
		slog.Warn("查询用户专属倍率失败，余额充值按不返利处理", "user_id", userID, "error", err)
		return true
	}
	return len(rates) > 0
}
