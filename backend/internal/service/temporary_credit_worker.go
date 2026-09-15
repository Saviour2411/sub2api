package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type TemporaryCreditWorker struct {
	repo         UserCustomizationRepository
	balanceCache creditBalanceCache
	ctx          context.Context
	cancel       context.CancelFunc
	start        sync.Once
	stop         sync.Once
	wg           sync.WaitGroup
}

func NewTemporaryCreditWorker(repo UserCustomizationRepository, balanceCache *BillingCacheService) *TemporaryCreditWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &TemporaryCreditWorker{repo: repo, ctx: ctx, cancel: cancel}
	if balanceCache != nil {
		w.balanceCache = balanceCache
	}
	return w
}

func (w *TemporaryCreditWorker) Start() {
	w.start.Do(func() {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				w.runCycle(w.ctx)
				select {
				case <-w.ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}

func (w *TemporaryCreditWorker) Stop() {
	if w == nil {
		return
	}
	w.stop.Do(func() { w.cancel(); w.wg.Wait() })
}

// 代次唯一约束保障一次性；有界扫描不占用网关用量 worker。
func (w *TemporaryCreditWorker) runCycle(parent context.Context) {
	release, accepted := trySharedBackgroundJob(context.Background(), "temporary-credit")
	if !accepted {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	for after := int64(0); ctx.Err() == nil; {
		ids, err := w.repo.CreditCandidates(ctx, after, 100)
		if err != nil {
			slog.Error("查询待授信用户失败", "error", err)
			break
		}
		if len(ids) == 0 {
			break
		}
		for _, id := range ids {
			one, done := context.WithTimeout(ctx, 3*time.Second)
			granted, err := w.repo.TryGrantCredit(one, id)
			done()
			if err != nil {
				slog.Error("临时授信失败，后续检查将重试", "user_id", id, "error", err)
			} else if granted {
				slog.Info("临时授信已入账", "user_id", id)
			}
			after = id
		}
	}
	// 缓存重试有独立预算，不因本轮授信扫描超时而饿死。
	cacheCtx, cacheCancel := context.WithTimeout(parent, 10*time.Second)
	defer cacheCancel()
	w.retryInvalidations(cacheCtx)
}

func (w *TemporaryCreditWorker) retryInvalidations(ctx context.Context) {
	if w.balanceCache == nil {
		return
	}
	for after := int64(0); ctx.Err() == nil; {
		pending, err := w.repo.PendingCreditInvalidations(ctx, after, 100)
		if err != nil {
			slog.Error("读取授信缓存失效任务失败", "error", err)
			return
		}
		if len(pending) == 0 {
			return
		}
		for _, item := range pending {
			one, done := context.WithTimeout(ctx, 2*time.Second)
			err := w.balanceCache.InvalidateUserBalance(one, item.UserID)
			if err == nil {
				err = w.repo.CompleteCreditInvalidation(one, item.ID)
			}
			done()
			if err != nil {
				slog.Warn("授信余额缓存失效待重试", "user_id", item.UserID, "error", err)
			}
			after = item.ID
		}
	}
}
