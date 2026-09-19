//go:build integration

package repository

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func customizationUser(t *testing.T, balance float64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users (email, username, password_hash, balance)
        VALUES ($1, '查询测试用户', 'hash', $2) RETURNING id`, fmt.Sprintf("custom-%d@example.com", time.Now().UnixNano()), balance).Scan(&id))
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM redeem_codes WHERE used_by = $1`, id)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`DELETE FROM api_keys WHERE user_id = $1`, id)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, id)
		require.NoError(t, err)
	})
	return id
}

func enabledCredit(amount float64) service.UserCustomizationInput {
	return service.UserCustomizationInput{AutoCreditEnabled: true, CreditThreshold: 1000, CreditAmount: amount}
}

func TestUserCustomizationSearchUsernameEmailAndID(t *testing.T) {
	ctx := context.Background()
	repo := NewUserCustomizationRepository(integrationDB)
	svc := service.NewUserCustomizationService(repo, nil, nil)
	prefix := fmt.Sprintf("custom-search-%d", time.Now().UnixNano())
	_, initialTotal, err := repo.List(ctx, "", 1, 20)
	require.NoError(t, err)
	_, existingBoxTotal, err := svc.List(ctx, "box", 1, 100)
	require.NoError(t, err)
	emailID := customizationUser(t, 10)
	usernameID := customizationUser(t, 20)
	deletedID := customizationUser(t, 30)
	notesID := customizationUser(t, 40)
	email := "boxinsmart-" + prefix + "@example.test"
	_, err = integrationDB.Exec(`UPDATE users SET username = '', email = $2 WHERE id = $1`, emailID, email)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE users SET username = $2, email = $3 WHERE id = $1`, usernameID, "中文查询-MixedUser-"+prefix, "other-"+prefix+"@example.test")
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE users SET username = $2, email = $3, deleted_at = NOW() WHERE id = $1`, deletedID, "中文查询-"+prefix, "boxinsmart-deleted-"+prefix+"@example.test")
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE users SET username = '普通用户', email = $2, notes = $3 WHERE id = $1`, notesID, "plain-"+prefix+"@example.test", "boxinsmart 中文查询")
	require.NoError(t, err)

	for _, tc := range []struct {
		name, search string
		wantID       int64
		existing     int64
	}{
		{"邮箱前缀", "box", emailID, existingBoxTotal},
		{"忽略大小写", "BOX", emailID, existingBoxTotal},
		{"首尾空白", " \tbox\n", emailID, existingBoxTotal},
		{"完整邮箱", email, emailID, 0},
		{"邮箱中段", "smart-" + prefix, emailID, 0},
		{"中文用户名", "中文查询-MixedUser-" + prefix, usernameID, 0},
		{"用户名中段", "查询-MixedUser-" + prefix, usernameID, 0},
		{"用户名忽略大小写", "mixeduser-" + prefix, usernameID, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, total, listErr := svc.List(ctx, tc.search, 1, 100)
			require.NoError(t, listErr)
			require.Equal(t, tc.existing+1, total)
			require.Len(t, items, int(total))
			var matched *service.UserCustomization
			for i := range items {
				require.NotEqual(t, deletedID, items[i].UserID)
				require.NotEqual(t, notesID, items[i].UserID)
				if items[i].UserID == tc.wantID {
					matched = &items[i]
				}
			}
			require.NotNil(t, matched)
			if tc.wantID == emailID {
				require.Empty(t, matched.Username)
				require.Equal(t, email, matched.Email)
			}
		})
	}

	items, _, err := svc.List(ctx, " "+strconv.FormatInt(emailID, 10)+" ", 1, 100)
	require.NoError(t, err)
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].UserID
	}
	require.Contains(t, ids, emailID)

	items, total, err := svc.List(ctx, prefix+"-absent", 1, 20)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	_, total, err = svc.List(ctx, " \t ", 1, 20)
	require.NoError(t, err)
	require.Equal(t, initialTotal+3, total)

	var pagedIDs []int64
	for page := 1; page <= 4; page++ {
		items, total, err = svc.List(ctx, prefix, page, 1)
		require.NoError(t, err)
		require.EqualValues(t, 3, total)
		for _, item := range items {
			pagedIDs = append(pagedIDs, item.UserID)
		}
	}
	require.Equal(t, []int64{notesID, usernameID, emailID}, pagedIDs)
	item, err := repo.Get(ctx, emailID)
	require.NoError(t, err)
	require.Equal(t, email, item.Email)
}

func TestUserCustomizationSearchIDRequiresExactMatch(t *testing.T) {
	ctx := context.Background()
	repo := NewUserCustomizationRepository(integrationDB)
	const exactID, longerID int64 = 81234567890123456, 812345678901234567
	_, err := integrationDB.Exec(`INSERT INTO users (id, email, username, password_hash) VALUES
		($1, 'exact-id@example.test', '', 'hash'), ($2, 'longer-id@example.test', '', 'hash')`, exactID, longerID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM users WHERE id IN ($1, $2)`, exactID, longerID)
		require.NoError(t, err)
	})
	items, total, err := repo.List(ctx, strconv.FormatInt(exactID, 10), 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, exactID, items[0].UserID)
	items, total, err = repo.List(ctx, "8123456789012345", 1, 20)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
}

func TestUserCustomizationThresholdAndEligibility(t *testing.T) {
	ctx := context.Background()
	repo := NewUserCustomizationRepository(integrationDB)
	for _, tc := range []struct {
		name          string
		balance       float64
		status        string
		enabled, want bool
	}{
		{"低于阈值", 999.99999999, "active", true, true},
		{"等于阈值", 1000, "active", true, false},
		{"高于阈值", 1000.00000001, "active", true, false},
		{"负余额", -2, "active", true, true},
		{"关闭开关", 1, "active", false, false},
		{"停用用户", 1, "disabled", true, false},
		{"删除用户", 1, "deleted", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := customizationUser(t, tc.balance)
			before, err := repo.Get(ctx, id)
			require.NoError(t, err)
			require.False(t, before.HasLink)
			require.False(t, before.AutoCreditEnabled)
			require.Equal(t, float64(1000), before.CreditThreshold)
			config := enabledCredit(0.12345678)
			config.AutoCreditEnabled = tc.enabled
			require.NoError(t, repo.Save(ctx, id, config))
			if tc.status == "deleted" {
				_, err = integrationDB.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, id)
			} else {
				_, err = integrationDB.Exec(`UPDATE users SET status = $2 WHERE id = $1`, id, tc.status)
			}
			require.NoError(t, err)
			granted, err := repo.TryGrantCredit(ctx, id)
			require.NoError(t, err)
			require.Equal(t, tc.want, granted)
			var balance float64
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id = $1`, id).Scan(&balance))
			want := tc.balance
			if tc.want {
				want += config.CreditAmount
			}
			require.InDelta(t, want, balance, 1e-8)
		})
	}
}

func TestUserCustomizationConcurrentGrantBillingAndRecharge(t *testing.T) {
	ctx := context.Background()
	id := customizationUser(t, 100)
	repo := NewUserCustomizationRepository(integrationDB)
	require.NoError(t, repo.Save(ctx, id, enabledCredit(50)))
	userRepo := NewUserRepository(integrationEntClient, integrationDB)
	var wg sync.WaitGroup
	var grants atomic.Int32
	start := make(chan struct{})
	errs := make(chan error, 34)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			granted, err := repo.TryGrantCredit(ctx, id)
			if granted {
				grants.Add(1)
			}
			errs <- err
		}()
	}
	wg.Add(2)
	go func() { defer wg.Done(); <-start; errs <- userRepo.DeductBalance(ctx, id, 3) }()
	go func() { defer wg.Done(); <-start; _, err := userRepo.AdjustBalance(ctx, id, 7); errs <- err }()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	// 如果本次所有任务都遇到忙行，下一个周期仍可完成入账。
	granted, err := NewUserCustomizationRepository(integrationDB).TryGrantCredit(ctx, id)
	require.NoError(t, err)
	if granted {
		grants.Add(1)
	}
	require.Equal(t, int32(1), grants.Load())
	item, err := repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, float64(154), item.Balance)
	require.NotNil(t, item.CreditUsedAt)
	require.False(t, item.HasLink)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM temporary_credit_grants WHERE user_id = $1`, id).Scan(&count))
	require.Equal(t, 1, count)
	historyRepo := NewRedeemCodeRepository(integrationEntClient)
	total, err := historyRepo.SumPositiveBalanceByUser(ctx, id)
	require.NoError(t, err)
	require.Zero(t, total, "临时授信不计真实充值累计")
	records, paginationResult, err := historyRepo.ListByUserPaginated(ctx, id, pagination.PaginationParams{Page: 1, PageSize: 20}, service.TemporaryCreditType)
	require.NoError(t, err)
	require.EqualValues(t, 1, paginationResult.Total)
	require.Equal(t, "临时授信50", records[0].Notes)
	var trueRecharge float64
	require.NoError(t, integrationDB.QueryRow(`SELECT total_recharged FROM users WHERE id = $1`, id).Scan(&trueRecharge))
	require.Zero(t, trueRecharge)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 OR source_user_id = $1`, id).Scan(&count))
	require.Zero(t, count, "临时授信不产生返佣流水")
}

func TestUserCustomizationPersistentCacheInvalidation(t *testing.T) {
	ctx := context.Background()
	id := customizationUser(t, 1)
	repo := NewUserCustomizationRepository(integrationDB)
	require.NoError(t, repo.Save(ctx, id, enabledCredit(20)))
	key := fmt.Sprintf("sk-credit-cache-%d", id)
	_, err := integrationDB.Exec(`INSERT INTO api_keys (user_id, key, name) VALUES ($1, $2, '测试')`, id, key)
	require.NoError(t, err)
	keyHash := service.BalanceQueryTokenHash(key)
	_, err = integrationDB.Exec(`DELETE FROM auth_cache_invalidation_outbox WHERE cache_key = $1`, keyHash)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM auth_cache_invalidation_outbox WHERE cache_key = $1`, keyHash)
	})
	granted, err := repo.TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.True(t, granted)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key = $1`, keyHash).Scan(&count))
	require.Equal(t, 1, count, "加款事务写入持久化鉴权失效事件")
	var grantID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM temporary_credit_grants WHERE user_id = $1`, id).Scan(&grantID))
	redisClient := testRedis(t)
	cache := NewBillingCache(redisClient)
	require.NoError(t, cache.SetUserBalance(ctx, id, 1))
	require.NoError(t, cache.InvalidateUserBalance(ctx, id))
	require.NoError(t, repo.CompleteCreditInvalidation(ctx, grantID))
	var complete bool
	require.NoError(t, integrationDB.QueryRow(`SELECT cache_cleared_at IS NOT NULL FROM temporary_credit_grants WHERE id = $1`, grantID).Scan(&complete))
	require.False(t, complete, "第一次删除不结束延迟重试")
	// 模拟入账前的在途读取重新填入旧值，以及后台进程重启。
	require.NoError(t, cache.SetUserBalance(ctx, id, 1))
	_, err = integrationDB.Exec(`UPDATE temporary_credit_grants SET cache_next_at = NOW() WHERE id = $1`, grantID)
	require.NoError(t, err)
	restarted := NewUserCustomizationRepository(integrationDB)
	pending, err := restarted.PendingCreditInvalidations(ctx, grantID-1, 1)
	require.NoError(t, err)
	require.Equal(t, []service.CreditCacheInvalidation{{ID: grantID, UserID: id}}, pending)
	require.NoError(t, cache.InvalidateUserBalance(ctx, id))
	require.NoError(t, restarted.CompleteCreditInvalidation(ctx, grantID))
	_, err = cache.GetUserBalance(ctx, id)
	require.Error(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT cache_cleared_at IS NOT NULL FROM temporary_credit_grants WHERE id = $1`, grantID).Scan(&complete))
	require.True(t, complete)
	granted, err = restarted.TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.False(t, granted)
}

func TestUserCustomizationBusyRowRechecksBalance(t *testing.T) {
	ctx := context.Background()
	repo := NewUserCustomizationRepository(integrationDB)
	for _, tc := range []struct {
		balance, updated float64
		want             bool
	}{{10, 1000, false}, {1000, 999, true}} {
		id := customizationUser(t, tc.balance)
		require.NoError(t, repo.Save(ctx, id, enabledCredit(10)))
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.Exec(`UPDATE users SET balance = $2 WHERE id = $1`, id, tc.updated)
		require.NoError(t, err)
		granted, err := repo.TryGrantCredit(ctx, id)
		require.NoError(t, err)
		require.False(t, granted)
		require.NoError(t, tx.Commit())
		granted, err = repo.TryGrantCredit(ctx, id)
		require.NoError(t, err)
		require.Equal(t, tc.want, granted)
	}
}

func TestUserCustomizationRestoreCASAndIndependentControls(t *testing.T) {
	ctx := context.Background()
	id := customizationUser(t, 1)
	repo := NewUserCustomizationRepository(integrationDB)
	require.NoError(t, repo.Save(ctx, id, enabledCredit(10)))
	require.ErrorIs(t, repo.RestoreCredit(ctx, id, 1), service.ErrUserCustomizationConflict)
	granted, err := repo.TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.True(t, granted)
	config := enabledCredit(10)
	config.AutoCreditEnabled = false
	require.NoError(t, repo.Save(ctx, id, config))
	require.NoError(t, repo.Save(ctx, id, enabledCredit(10)))
	require.NoError(t, repo.RotateLink(ctx, id, 0, service.BalanceQueryTokenHash(fmt.Sprint(id)), "密文"))
	require.NoError(t, repo.SetLinkEnabled(ctx, id, 1, false))
	granted, err = repo.TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.False(t, granted)
	require.NoError(t, repo.RestoreCredit(ctx, id, 1))
	granted, err = repo.TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.True(t, granted)
	require.ErrorIs(t, repo.RestoreCredit(ctx, id, 1), service.ErrUserCustomizationConflict, "旧代次重放不得恢复下一轮")
	item, err := repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, int64(2), item.CreditGeneration)
	require.Equal(t, float64(21), item.Balance)
	require.False(t, item.LinkEnabled)
}

func TestUserCustomizationRollbackAndRestart(t *testing.T) {
	ctx := context.Background()
	id := customizationUser(t, 1)
	repo := NewUserCustomizationRepository(integrationDB)
	require.NoError(t, repo.Save(ctx, id, enabledCredit(20)))
	name := fmt.Sprintf("test_credit_failure_%d", id)
	_, err := integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS TRIGGER LANGUAGE plpgsql AS $$
        BEGIN RAISE EXCEPTION '测试入账中途失败'; END $$;
        CREATE TRIGGER %s BEFORE UPDATE OF redeem_code_id ON temporary_credit_grants
        FOR EACH ROW WHEN (NEW.user_id = %d) EXECUTE FUNCTION %s()`, name, name, id, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(fmt.Sprintf(`DROP FUNCTION IF EXISTS %s() CASCADE`, name))
		require.NoError(t, err)
	})
	granted, err := repo.TryGrantCredit(ctx, id)
	require.Error(t, err)
	require.False(t, granted)
	item, err := repo.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, float64(1), item.Balance)
	require.Nil(t, item.CreditUsedAt)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM temporary_credit_grants WHERE user_id = $1`, id).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM redeem_codes WHERE used_by = $1`, id).Scan(&count))
	require.Zero(t, count)
	_, err = integrationDB.Exec(fmt.Sprintf(`DROP FUNCTION %s() CASCADE`, name))
	require.NoError(t, err)
	granted, err = NewUserCustomizationRepository(integrationDB).TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.True(t, granted)
	granted, err = NewUserCustomizationRepository(integrationDB).TryGrantCredit(ctx, id)
	require.NoError(t, err)
	require.False(t, granted)
}

func TestUserCustomizationLinkIsolationAndHistory(t *testing.T) {
	ctx := context.Background()
	repo := NewUserCustomizationRepository(integrationDB)
	first, second := customizationUser(t, 1), customizationUser(t, 2)
	hash := service.BalanceQueryTokenHash(fmt.Sprint(first))
	require.NoError(t, repo.RotateLink(ctx, first, 0, hash, "不可公开的密文"))
	user, err := repo.ResolvePublic(ctx, hash)
	require.NoError(t, err)
	require.Equal(t, first, user.ID)
	require.ErrorIs(t, repo.RotateLink(ctx, first, 0, "旧版本", "密文"), service.ErrUserCustomizationConflict)
	require.NoError(t, repo.SetLinkEnabled(ctx, first, 1, false))
	_, err = repo.ResolvePublic(ctx, hash)
	require.ErrorIs(t, err, service.ErrBalanceQueryUnavailable)
	require.NoError(t, repo.SetLinkEnabled(ctx, first, 2, true))
	require.NoError(t, repo.RotateLink(ctx, first, 3, service.BalanceQueryTokenHash("新"+fmt.Sprint(first)), "新密文"))
	_, err = repo.ResolvePublic(ctx, hash)
	require.ErrorIs(t, err, service.ErrBalanceQueryUnavailable)
	for _, uid := range []int64{first, second} {
		for _, kind := range []string{"balance", "admin_balance", "subscription", "concurrency"} {
			code, err := service.GenerateRedeemCode()
			require.NoError(t, err)
			var rid int64
			require.NoError(t, integrationDB.QueryRow(`INSERT INTO redeem_codes (code, type, value, status, used_by, used_at, notes)
				VALUES ($1, $2, $3, 'used', $4, NOW(), '内部敏感备注') RETURNING id`, code, kind, uid, uid).Scan(&rid))
			_, err = integrationDB.Exec(`INSERT INTO redeem_code_usages (redeem_code_id, user_id, type, value) VALUES ($1, $2, $3, $4)`, rid, uid, kind, uid)
			require.NoError(t, err)
		}
	}
	history, total, err := repo.PublicHistory(ctx, first, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, history, 1)
	require.EqualValues(t, first, history[0].Amount)
	require.Empty(t, history[0].Note)
	history, _, err = repo.PublicHistory(ctx, first, 2, 1)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.EqualValues(t, first, history[0].Amount)
	_, err = integrationDB.Exec(`UPDATE users SET status = 'disabled' WHERE id = $1`, first)
	require.NoError(t, err)
	_, err = repo.ResolvePublic(ctx, service.BalanceQueryTokenHash("新"+fmt.Sprint(first)))
	require.ErrorIs(t, err, service.ErrBalanceQueryUnavailable)
}
