package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userCustomizationRepository struct{ db *sql.DB }

func NewUserCustomizationRepository(db *sql.DB) service.UserCustomizationRepository {
	return &userCustomizationRepository{db: db}
}

const customizationColumns = `u.id, u.username, u.balance, u.status,
    c.link_hash IS NOT NULL, COALESCE(c.link_enabled, FALSE), COALESCE(c.link_version, 0),
    COALESCE(c.auto_credit_enabled, FALSE), COALESCE(c.credit_threshold, 1000),
    COALESCE(c.credit_amount, 0), COALESCE(c.credit_generation, 1), c.credit_used_at`

func scanCustomization(row interface{ Scan(...any) error }) (*service.UserCustomization, error) {
	var item service.UserCustomization
	err := row.Scan(&item.UserID, &item.Username, &item.Balance, &item.Status,
		&item.HasLink, &item.LinkEnabled, &item.LinkVersion, &item.AutoCreditEnabled,
		&item.CreditThreshold, &item.CreditAmount, &item.CreditGeneration, &item.CreditUsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	return &item, err
}

func (r *userCustomizationRepository) List(ctx context.Context, search string, page, size int) ([]service.UserCustomization, int64, error) {
	filter := `u.deleted_at IS NULL AND ($1 = '' OR u.username ILIKE '%' || $1 || '%' OR u.id::text = $1)`
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u WHERE `+filter, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+customizationColumns+`
        FROM users u LEFT JOIN user_customizations c ON c.user_id = u.id
        WHERE `+filter+` ORDER BY u.id DESC LIMIT $2 OFFSET $3`, search, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.UserCustomization{}
	for rows.Next() {
		item, err := scanCustomization(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, rows.Err()
}

func (r *userCustomizationRepository) Get(ctx context.Context, id int64) (*service.UserCustomization, error) {
	return scanCustomization(r.db.QueryRowContext(ctx, `SELECT `+customizationColumns+`
        FROM users u LEFT JOIN user_customizations c ON c.user_id = u.id
        WHERE u.id = $1 AND u.deleted_at IS NULL`, id))
}

func customizationChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrUserCustomizationConflict
	}
	return nil
}

func (r *userCustomizationRepository) Save(ctx context.Context, id int64, in service.UserCustomizationInput) error {
	return customizationChanged(r.db.ExecContext(ctx, `
        INSERT INTO user_customizations (user_id, auto_credit_enabled, credit_threshold, credit_amount)
        SELECT id, $2, $3, $4 FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active'
        ON CONFLICT (user_id) DO UPDATE SET auto_credit_enabled = EXCLUDED.auto_credit_enabled,
            credit_threshold = EXCLUDED.credit_threshold, credit_amount = EXCLUDED.credit_amount, updated_at = NOW()`,
		id, in.AutoCreditEnabled, in.CreditThreshold, in.CreditAmount))
}

func (r *userCustomizationRepository) RotateLink(ctx context.Context, id, version int64, hash, encrypted string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_customizations (user_id)
        SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active'
        ON CONFLICT (user_id) DO NOTHING`, id); err != nil {
		return err
	}
	if err := customizationChanged(tx.ExecContext(ctx, `UPDATE user_customizations
        SET link_hash = $3, link_encrypted = $4, link_enabled = TRUE, link_version = link_version + 1, updated_at = NOW()
        WHERE user_id = $1 AND link_version = $2
          AND EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active')`, id, version, hash, encrypted)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *userCustomizationRepository) SetLinkEnabled(ctx context.Context, id, version int64, enabled bool) error {
	return customizationChanged(r.db.ExecContext(ctx, `UPDATE user_customizations
        SET link_enabled = $3, link_version = link_version + 1, updated_at = NOW()
        WHERE user_id = $1 AND link_version = $2 AND link_hash IS NOT NULL
          AND EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active')`, id, version, enabled))
}

func (r *userCustomizationRepository) GetLinkSecret(ctx context.Context, id int64) (string, error) {
	var secret string
	err := r.db.QueryRowContext(ctx, `SELECT c.link_encrypted FROM user_customizations c
        JOIN users u ON u.id = c.user_id WHERE c.user_id = $1 AND c.link_hash IS NOT NULL
        AND u.deleted_at IS NULL AND u.status = 'active'`, id).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) {
		return "", service.ErrBalanceQueryUnavailable
	}
	return secret, err
}

func (r *userCustomizationRepository) ResolvePublic(ctx context.Context, hash string) (*service.PublicBalanceUser, error) {
	var user service.PublicBalanceUser
	err := r.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.balance FROM user_customizations c
        JOIN users u ON u.id = c.user_id WHERE c.link_hash = $1 AND c.link_enabled
        AND u.deleted_at IS NULL AND u.status = 'active'`, hash).Scan(&user.ID, &user.Username, &user.Balance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrBalanceQueryUnavailable
	}
	return &user, err
}

// 只投影已到账金额和来源，不读取兑换码、管理备注或支付凭据。
const publicRechargeHistory = `WITH history AS (
    SELECT rcu.id AS sort_id, 1 AS source_id, rcu.used_at, rcu.value, rcu.type
    FROM redeem_code_usages rcu
    JOIN redeem_codes rc ON rc.id = rcu.redeem_code_id
    WHERE rcu.user_id = $1 AND rcu.value > 0 AND rcu.type IN ('balance', 'admin_balance', 'temporary_credit')
    UNION ALL
    SELECT rc.id, 0, rc.used_at, rc.value, rc.type FROM redeem_codes rc
    WHERE rc.used_by = $1 AND rc.status = 'used' AND rc.used_at IS NOT NULL AND rc.value > 0
      AND rc.type = 'admin_balance'
      AND NOT EXISTS (SELECT 1 FROM redeem_code_usages rcu WHERE rcu.redeem_code_id = rc.id AND rcu.user_id = $1)
) `

func (r *userCustomizationRepository) PublicHistory(ctx context.Context, id int64, page, size int) ([]service.PublicRechargeRecord, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, publicRechargeHistory+`SELECT COUNT(*) FROM history`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, publicRechargeHistory+`SELECT used_at, value, type FROM history
        ORDER BY used_at DESC, source_id DESC, sort_id DESC LIMIT $2 OFFSET $3`, id, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []service.PublicRechargeRecord{}
	for rows.Next() {
		var item service.PublicRechargeRecord
		if err := rows.Scan(&item.CreatedAt, &item.Amount, &item.Type); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *userCustomizationRepository) RestoreCredit(ctx context.Context, id, generation int64) error {
	return customizationChanged(r.db.ExecContext(ctx, `UPDATE user_customizations
        SET credit_generation = credit_generation + 1, credit_used_at = NULL, auto_credit_enabled = TRUE, updated_at = NOW()
        WHERE user_id = $1 AND credit_generation = $2 AND credit_used_at IS NOT NULL AND credit_amount > 0
          AND EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active')`, id, generation))
}

func (r *userCustomizationRepository) CreditCandidates(ctx context.Context, after int64, limit int) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.user_id FROM user_customizations c JOIN users u ON u.id = c.user_id
        WHERE c.user_id > $1 AND c.auto_credit_enabled AND c.credit_used_at IS NULL
          AND u.deleted_at IS NULL AND u.status = 'active' AND u.balance < c.credit_threshold
        ORDER BY c.user_id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *userCustomizationRepository) TryGrantCredit(ctx context.Context, id int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	// 先锁用户再锁配置；忙用户不占着连接等待，后续周期再处理。
	var lockedID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active' FOR UPDATE SKIP LOCKED`, id).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM user_customizations WHERE user_id = $1 FOR UPDATE SKIP LOCKED`, id).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var grantID int64
	var amount float64
	err = tx.QueryRowContext(ctx, `INSERT INTO temporary_credit_grants
        (user_id, generation, threshold, amount, balance_before, balance_after)
        SELECT u.id, c.credit_generation, c.credit_threshold, c.credit_amount, u.balance, u.balance + c.credit_amount
        FROM users u JOIN user_customizations c ON c.user_id = u.id
        WHERE u.id = $1 AND c.auto_credit_enabled AND c.credit_used_at IS NULL
          AND c.credit_amount > 0 AND u.balance < c.credit_threshold
        ON CONFLICT (user_id, generation) DO NOTHING RETURNING id, amount`, id).Scan(&grantID, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	code, err := service.GenerateRedeemCode()
	if err != nil {
		return false, err
	}
	var redeemID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO redeem_codes
        (code, type, value, status, used_by, used_at, notes, max_uses, used_count)
        SELECT $2, 'temporary_credit', amount, 'used', user_id, created_at, $3, 1, 1
        FROM temporary_credit_grants WHERE id = $1 RETURNING id`, grantID, code, service.TemporaryCreditNote(amount)).Scan(&redeemID)
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO redeem_code_usages (redeem_code_id, user_id, type, value, used_at)
        SELECT $2, user_id, 'temporary_credit', amount, created_at FROM temporary_credit_grants WHERE id = $1`, grantID, redeemID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users u SET balance = u.balance + g.amount, updated_at = NOW()
        FROM temporary_credit_grants g WHERE g.id = $1 AND u.id = g.user_id`, grantID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE user_customizations c SET credit_used_at = g.created_at, updated_at = NOW()
        FROM temporary_credit_grants g WHERE g.id = $1 AND c.user_id = g.user_id`, grantID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE temporary_credit_grants SET redeem_code_id = $2 WHERE id = $1`, grantID, redeemID); err != nil {
		return false, err
	}
	// 复用已有持久化鉴权缓存 outbox；Redis 不可用时也不会丢失失效事件。
	if _, err = tx.ExecContext(ctx, `INSERT INTO auth_cache_invalidation_outbox (cache_key)
        SELECT encode(sha256(convert_to(key, 'UTF8')), 'hex') FROM api_keys
        WHERE user_id = $1 AND deleted_at IS NULL AND key <> ''`, id); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *userCustomizationRepository) PendingCreditInvalidations(ctx context.Context, after int64, limit int) ([]service.CreditCacheInvalidation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id FROM temporary_credit_grants
        WHERE id > $1 AND cache_cleared_at IS NULL AND cache_next_at <= NOW() ORDER BY id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.CreditCacheInvalidation{}
	for rows.Next() {
		var item service.CreditCacheInvalidation
		if err := rows.Scan(&item.ID, &item.UserID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *userCustomizationRepository) CompleteCreditInvalidation(ctx context.Context, id int64) error {
	// 延迟第二次删除，覆盖入账前已经在途的旧余额缓存填充。
	_, err := r.db.ExecContext(ctx, `UPDATE temporary_credit_grants SET
        cache_cleared_at = CASE WHEN cache_pass = 1 THEN NOW() ELSE NULL END,
        cache_pass = 1, cache_next_at = NOW() + INTERVAL '30 seconds'
        WHERE id = $1 AND cache_cleared_at IS NULL AND cache_next_at <= NOW()`, id)
	return err
}
