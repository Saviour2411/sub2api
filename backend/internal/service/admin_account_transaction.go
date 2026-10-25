package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// 外层事务只清理数据库错误字段；事故 marker 保留到提交后 Redis 重置成功。
// 后续事故清理仍使用独立事务及事故 ID 条件更新，避免缓存失败放行或覆盖新事故。
func (s *adminServiceImpl) deferAccountErrorRecovery(ctx context.Context, account *Account, occurredAt time.Time) bool {
	tx := dbent.TxFromContext(ctx)
	if tx == nil {
		return false
	}
	incidentID, _ := account.FailureStrategyUnscheduledIncident()
	tx.OnCommit(func(next dbent.Committer) dbent.Committer {
		return dbent.CommitFunc(func(commitCtx context.Context, tx *dbent.Tx) error {
			if err := next.Commit(commitCtx, tx); err != nil {
				return err
			}
			baseCtx := dbent.NewTxContext(ctx, nil)
			if err := s.resetAccountFailureStreaks(baseCtx, account.ID, occurredAt); err != nil {
				s.restoreAdminFailureSchedulingRuntimeBlock(account)
				return err
			}
			if account.HasFailureStrategyUnscheduledMarker() {
				if err := s.clearFailureSchedulingStateForAdmin(baseCtx, account, incidentID); err != nil {
					s.restoreAdminFailureSchedulingRuntimeBlock(account)
					return err
				}
			}
			if s.runtimeBlocker != nil {
				s.runtimeBlocker.ClearAccountSchedulingBlock(account.ID)
			}
			updated, err := s.accountRepo.GetByID(baseCtx, account.ID)
			if err != nil {
				s.restoreAdminFailureSchedulingRuntimeBlock(account)
				return err
			}
			s.restoreAdminFailureSchedulingRuntimeBlock(updated)
			*account = *updated
			return nil
		})
	})
	return true
}
