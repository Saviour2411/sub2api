package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	timezonepkg "github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

var (
	ErrDailyCheckinDisabled = infraerrors.Forbidden("DAILY_CHECKIN_DISABLED", "daily check-in is disabled")
	ErrDailyCheckinRole     = infraerrors.Forbidden("DAILY_CHECKIN_ROLE_FORBIDDEN", "only regular users can use daily check-in")
	ErrDailyCheckinDone     = infraerrors.Conflict("DAILY_CHECKIN_ALREADY_DONE", "already checked in today")
)

type DailyCheckinStatus struct {
	Enabled        bool
	CheckedInToday bool
	RewardMode     string
	RewardAmount   float64
	RewardMin      float64
	RewardMax      float64
	TodayReward    *float64
	CheckedInAt    *time.Time
}

type DailyCheckinResult struct {
	RewardAmount float64
	NewBalance   float64
	CheckedInAt  time.Time
}

type DailyCheckinService struct {
	client               *dbent.Client
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewDailyCheckinService(
	client *dbent.Client,
	settingService *SettingService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService *BillingCacheService,
) *DailyCheckinService {
	return &DailyCheckinService{
		client:               client,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

func (s *DailyCheckinService) GetStatus(ctx context.Context, userID int64) (*DailyCheckinStatus, error) {
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, err
	}

	role, err := s.userRole(ctx, userID)
	if err != nil {
		return nil, err
	}
	if role != RoleUser {
		cfg.Enabled = false
	}

	status := &DailyCheckinStatus{
		Enabled:      cfg.Enabled,
		RewardMode:   cfg.Mode,
		RewardAmount: cfg.Amount,
		RewardMin:    cfg.Min,
		RewardMax:    cfg.Max,
	}
	if s.client == nil || userID <= 0 {
		return status, nil
	}

	rows, err := s.client.QueryContext(ctx, `
SELECT reward_amount::double precision, created_at
FROM daily_checkins
WHERE user_id = $1 AND checkin_date = $2
LIMIT 1`, userID, todayDateString())
	if err != nil {
		return nil, fmt.Errorf("get daily check-in status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if rows.Next() {
		var reward float64
		var checkedInAt time.Time
		if err := rows.Scan(&reward, &checkedInAt); err != nil {
			return nil, fmt.Errorf("scan daily check-in status: %w", err)
		}
		status.CheckedInToday = true
		status.TodayReward = &reward
		status.CheckedInAt = &checkedInAt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read daily check-in status: %w", err)
	}

	return status, nil
}

func (s *DailyCheckinService) Checkin(ctx context.Context, userID int64) (*DailyCheckinResult, error) {
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrDailyCheckinDisabled
	}
	if s.client == nil {
		return nil, infraerrors.InternalServer("DAILY_CHECKIN_UNAVAILABLE", "daily check-in storage is unavailable")
	}

	reward, err := cfg.reward()
	if err != nil {
		return nil, err
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin daily check-in transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	role, balanceBefore, err := lockUserForCheckin(ctx, txClient, userID)
	if err != nil {
		return nil, err
	}
	if role != RoleUser {
		return nil, ErrDailyCheckinRole
	}

	now := timezonepkg.Now()
	balanceAfter := balanceBefore + reward
	_, err = txClient.ExecContext(ctx, `
INSERT INTO daily_checkins (user_id, checkin_date, reward_amount, balance_before, balance_after, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`, userID, todayDateString(), reward, balanceBefore, balanceAfter, now)
	if err != nil {
		if isDailyCheckinDuplicate(err) {
			return nil, ErrDailyCheckinDone.WithCause(err)
		}
		return nil, fmt.Errorf("create daily check-in record: %w", err)
	}

	_, err = txClient.ExecContext(ctx, `
UPDATE users
SET balance = balance + $2, updated_at = $3
WHERE id = $1 AND deleted_at IS NULL`, userID, reward, now)
	if err != nil {
		return nil, fmt.Errorf("update daily check-in balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit daily check-in transaction: %w", err)
	}

	s.invalidateBalanceCaches(ctx, userID)
	return &DailyCheckinResult{
		RewardAmount: reward,
		NewBalance:   balanceAfter,
		CheckedInAt:  now,
	}, nil
}

func (s *DailyCheckinService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService == nil {
		return
	}
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
	}()
}

func (s *DailyCheckinService) userRole(ctx context.Context, userID int64) (string, error) {
	if s.client == nil || userID <= 0 {
		return "", ErrUserNotFound
	}
	rows, err := s.client.QueryContext(ctx, `
SELECT role
FROM users
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1`, userID)
	if err != nil {
		return "", fmt.Errorf("get daily check-in user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("read daily check-in user: %w", err)
		}
		return "", ErrUserNotFound
	}
	var role string
	if err := rows.Scan(&role); err != nil {
		return "", fmt.Errorf("scan daily check-in user: %w", err)
	}
	return role, nil
}

func lockUserForCheckin(ctx context.Context, client *dbent.Client, userID int64) (string, float64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT role, balance::double precision
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`, userID)
	if err != nil {
		return "", 0, fmt.Errorf("lock daily check-in user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", 0, fmt.Errorf("read locked daily check-in user: %w", err)
		}
		return "", 0, ErrUserNotFound
	}
	var role string
	var balance float64
	if err := rows.Scan(&role, &balance); err != nil {
		return "", 0, fmt.Errorf("scan locked daily check-in user: %w", err)
	}
	return role, balance, nil
}

type dailyCheckinConfig struct {
	Enabled bool
	Mode    string
	Amount  float64
	Min     float64
	Max     float64
}

func (s *DailyCheckinService) config(ctx context.Context) (dailyCheckinConfig, error) {
	cfg := dailyCheckinConfig{
		Enabled: false,
		Mode:    "fixed",
		Amount:  1,
		Min:     1,
		Max:     3,
	}
	if s.settingService == nil {
		return cfg, nil
	}
	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil {
		return cfg, fmt.Errorf("get daily check-in settings: %w", err)
	}
	cfg.Enabled = settings.DailyCheckinEnabled
	cfg.Mode, cfg.Amount, cfg.Min, cfg.Max = normalizeDailyCheckinSettings(
		settings.DailyCheckinRewardMode,
		settings.DailyCheckinRewardAmount,
		settings.DailyCheckinRewardMin,
		settings.DailyCheckinRewardMax,
	)
	return cfg, nil
}

func (c dailyCheckinConfig) reward() (float64, error) {
	if c.Mode != "range" {
		return roundCurrency(c.Amount), nil
	}
	minCents := amountToCents(c.Min)
	maxCents := amountToCents(c.Max)
	if maxCents < minCents {
		maxCents = minCents
	}
	if maxCents == minCents {
		return float64(minCents) / 100, nil
	}
	offset, err := rand.Int(rand.Reader, big.NewInt(maxCents-minCents+1))
	if err != nil {
		return 0, fmt.Errorf("generate daily check-in reward: %w", err)
	}
	return float64(minCents+offset.Int64()) / 100, nil
}

func amountToCents(value float64) int64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int64(math.Round(value * 100))
}

func roundCurrency(value float64) float64 {
	return float64(amountToCents(value)) / 100
}

func todayDateString() string {
	return timezonepkg.Today().Format("2006-01-02")
}

func isDailyCheckinDuplicate(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "daily_checkins_user_date_unique") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate entry")
}
