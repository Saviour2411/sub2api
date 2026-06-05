package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
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

const (
	DailyCheckinPrizeTypeBalance      = "balance"
	DailyCheckinPrizeTypeConcurrency  = "concurrency"
	DailyCheckinPrizeTypeSubscription = "subscription"
	DailyCheckinPrizeTypeNone         = "none"

	DailyCheckinProbabilityTotal = 10000
	DailyCheckinFactorFull       = 10000
)

var (
	ErrDailyCheckinDisabled = infraerrors.Forbidden("DAILY_CHECKIN_DISABLED", "daily check-in is disabled")
	ErrDailyCheckinRole     = infraerrors.Forbidden("DAILY_CHECKIN_ROLE_FORBIDDEN", "only users and admins can use daily check-in")
	ErrDailyCheckinDone     = infraerrors.Conflict("DAILY_CHECKIN_ALREADY_DONE", "already checked in today")
)

type DailyCheckinPrizeConfig struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	ProbabilityBps int     `json:"probability_bps"`
	Enabled        bool    `json:"enabled"`
	SortOrder      int     `json:"sort_order"`
	BalanceMode    string  `json:"balance_mode,omitempty"`
	Amount         float64 `json:"amount,omitempty"`
	MinAmount      float64 `json:"min_amount,omitempty"`
	MaxAmount      float64 `json:"max_amount,omitempty"`
	Concurrency    int     `json:"concurrency,omitempty"`
	GroupID        int64   `json:"group_id,omitempty"`
	ValidityDays   int     `json:"validity_days,omitempty"`
}

type DailyCheckinDecayRule struct {
	AfterDays int `json:"after_days"`
	FactorBps int `json:"factor_bps"`
}

type DailyCheckinPrizeView struct {
	DailyCheckinPrizeConfig
	EffectiveProbabilityBps int `json:"effective_probability_bps"`
}

type DailyCheckinPublicPrizeView struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	SortOrder    int     `json:"sort_order"`
	BalanceMode  string  `json:"balance_mode,omitempty"`
	Amount       float64 `json:"amount,omitempty"`
	MinAmount    float64 `json:"min_amount,omitempty"`
	MaxAmount    float64 `json:"max_amount,omitempty"`
	Concurrency  int     `json:"concurrency,omitempty"`
	GroupID      int64   `json:"group_id,omitempty"`
	ValidityDays int     `json:"validity_days,omitempty"`
}

type DailyCheckinDecayStatus struct {
	Paid           bool   `json:"paid"`
	Exempt         bool   `json:"exempt"`
	ExemptReason   string `json:"exempt_reason,omitempty"`
	AccountAgeDays int    `json:"account_age_days"`
	FactorBps      int    `json:"factor_bps"`
	FullDays       int    `json:"full_days"`
}

type DailyCheckinRewardView struct {
	PrizeID          string   `json:"prize_id"`
	PrizeName        string   `json:"prize_name"`
	Type             string   `json:"type"`
	Amount           float64  `json:"amount,omitempty"`
	NewBalance       *float64 `json:"new_balance,omitempty"`
	Concurrency      int      `json:"concurrency,omitempty"`
	NewConcurrency   *int     `json:"new_concurrency,omitempty"`
	GroupID          *int64   `json:"group_id,omitempty"`
	GroupName        string   `json:"group_name,omitempty"`
	ValidityDays     int      `json:"validity_days,omitempty"`
	SubscriptionEnds *string  `json:"subscription_expires_at,omitempty"`
	CheckedInAt      string   `json:"checked_in_at"`
}

type DailyCheckinRecord struct {
	ID           int64   `json:"id"`
	PrizeID      string  `json:"prize_id"`
	PrizeName    string  `json:"prize_name"`
	Type         string  `json:"type"`
	Amount       float64 `json:"amount,omitempty"`
	Concurrency  int     `json:"concurrency,omitempty"`
	GroupID      *int64  `json:"group_id,omitempty"`
	ValidityDays int     `json:"validity_days,omitempty"`
	CheckedInAt  string  `json:"checked_in_at"`
}

type DailyCheckinStatus struct {
	Enabled        bool                          `json:"enabled"`
	CheckedInToday bool                          `json:"checked_in_today"`
	RewardMode     string                        `json:"reward_mode"`
	RewardAmount   float64                       `json:"reward_amount"`
	RewardMin      float64                       `json:"reward_min"`
	RewardMax      float64                       `json:"reward_max"`
	TodayReward    *float64                      `json:"today_reward,omitempty"`
	CheckedInAt    *string                       `json:"checked_in_at,omitempty"`
	Prizes         []DailyCheckinPublicPrizeView `json:"prizes"`
	Decay          DailyCheckinDecayStatus       `json:"decay"`
	TodayResult    *DailyCheckinRewardView       `json:"today_result,omitempty"`
	RecentRecords  []DailyCheckinRecord          `json:"recent_records"`
}

type DailyCheckinResult struct {
	RewardAmount float64                       `json:"reward_amount"`
	NewBalance   float64                       `json:"new_balance"`
	CheckedInAt  string                        `json:"checked_in_at"`
	Prize        DailyCheckinRewardView        `json:"prize"`
	Prizes       []DailyCheckinPublicPrizeView `json:"prizes,omitempty"`
	Decay        DailyCheckinDecayStatus       `json:"decay"`
}

type DailyCheckinService struct {
	client               *dbent.Client
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
	subscriptionService  *SubscriptionService
}

func NewDailyCheckinService(
	client *dbent.Client,
	settingService *SettingService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService *BillingCacheService,
	subscriptionService *SubscriptionService,
) *DailyCheckinService {
	return &DailyCheckinService{
		client:               client,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
		subscriptionService:  subscriptionService,
	}
}

func (s *DailyCheckinService) GetStatus(ctx context.Context, userID int64) (*DailyCheckinStatus, error) {
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, err
	}
	status := &DailyCheckinStatus{
		Enabled:      cfg.Enabled,
		RewardMode:   cfg.LegacyMode,
		RewardAmount: cfg.LegacyAmount,
		RewardMin:    cfg.LegacyMin,
		RewardMax:    cfg.LegacyMax,
		Prizes:       publicPrizeViews(buildEffectivePrizeViews(cfg.Prizes, DailyCheckinFactorFull)),
		Decay: DailyCheckinDecayStatus{
			FactorBps: DailyCheckinFactorFull,
			FullDays:  cfg.UnpaidFullDays,
		},
	}
	if s.client == nil || userID <= 0 {
		return status, nil
	}

	user, err := s.getUserCheckinContext(ctx, s.client, userID, false)
	if err != nil {
		return nil, err
	}
	if !canDailyCheckinRole(user.Role) {
		cfg.Enabled = false
	}

	decay, err := s.decayStatus(ctx, s.client, cfg, user)
	if err != nil {
		return nil, err
	}
	prizes := buildEffectivePrizeViews(cfg.Prizes, decay.FactorBps)
	status.Enabled = cfg.Enabled
	status.Prizes = publicPrizeViews(prizes)
	status.Decay = decay

	today, err := s.getTodayRecord(ctx, s.client, userID)
	if err != nil {
		return nil, err
	}
	if today != nil {
		status.CheckedInToday = true
		status.TodayReward = &today.Amount
		status.CheckedInAt = &today.CheckedInAt
		status.TodayResult = &DailyCheckinRewardView{
			PrizeID:      today.PrizeID,
			PrizeName:    today.PrizeName,
			Type:         today.Type,
			Amount:       today.Amount,
			Concurrency:  today.Concurrency,
			GroupID:      today.GroupID,
			ValidityDays: today.ValidityDays,
			CheckedInAt:  today.CheckedInAt,
		}
	}
	records, err := s.recentRecords(ctx, userID, 10)
	if err != nil {
		return nil, err
	}
	status.RecentRecords = records
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

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin daily check-in transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	user, err := s.getUserCheckinContext(ctx, txClient, userID, true)
	if err != nil {
		return nil, err
	}
	if !canDailyCheckinRole(user.Role) {
		return nil, ErrDailyCheckinRole
	}

	decay, err := s.decayStatus(ctx, txClient, cfg, user)
	if err != nil {
		return nil, err
	}
	prizes := buildEffectivePrizeViews(cfg.Prizes, decay.FactorBps)
	selected, err := chooseDailyCheckinPrize(prizes)
	if err != nil {
		return nil, err
	}
	now := timezonepkg.Now()
	grant, err := s.prepareGrant(ctx, txClient, user, selected, now)
	if err != nil {
		return nil, err
	}
	snapshot, err := json.Marshal(selected.DailyCheckinPrizeConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal daily check-in prize snapshot: %w", err)
	}

	var recordID int64
	rows, err := txClient.QueryContext(ctx, `
INSERT INTO daily_checkins (
    user_id, checkin_date, reward_amount, balance_before, balance_after, created_at,
    prize_id, prize_name, reward_type, probability_bps, effective_probability_bps,
    decay_factor_bps, concurrency_before, concurrency_after, subscription_group_id,
    subscription_validity_days, subscription_expires_at, reward_snapshot
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING id`,
		userID, todayDateString(), grant.Amount, user.Balance, grant.BalanceAfter, now,
		selected.ID, selected.Name, selected.Type, selected.ProbabilityBps, selected.EffectiveProbabilityBps,
		decay.FactorBps, nullableIntArg(grant.ConcurrencyBefore), nullableIntArg(grant.ConcurrencyAfter),
		nullableInt64Value(grant.GroupID), grant.ValidityDays, nullableTimeValue(grant.SubscriptionExpiresAt), string(snapshot),
	)
	if err != nil {
		if isDailyCheckinDuplicate(err) {
			return nil, ErrDailyCheckinDone.WithCause(err)
		}
		return nil, fmt.Errorf("create daily check-in record: %w", err)
	}
	if rows.Next() {
		if err := rows.Scan(&recordID); err != nil {
			_ = rows.Close()
			if isDailyCheckinDuplicate(err) {
				return nil, ErrDailyCheckinDone.WithCause(err)
			}
			return nil, fmt.Errorf("scan daily check-in record: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		if isDailyCheckinDuplicate(err) {
			return nil, ErrDailyCheckinDone.WithCause(err)
		}
		return nil, fmt.Errorf("read daily check-in record: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close daily check-in record rows: %w", err)
	}
	if recordID == 0 {
		return nil, fmt.Errorf("create daily check-in record: missing id")
	}

	if err := s.applyGrant(ctx, txClient, userID, grant, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit daily check-in transaction: %w", err)
	}

	s.invalidateGrantCaches(ctx, userID, grant)
	result := grant.toRewardView(selected, now)
	return &DailyCheckinResult{
		RewardAmount: grant.Amount,
		NewBalance:   grant.BalanceAfter,
		CheckedInAt:  now.Format(time.RFC3339),
		Prize:        result,
		Prizes:       publicPrizeViews(prizes),
		Decay:        decay,
	}, nil
}

type dailyCheckinConfig struct {
	Enabled              bool
	LegacyMode           string
	LegacyAmount         float64
	LegacyMin            float64
	LegacyMax            float64
	Prizes               []DailyCheckinPrizeConfig
	UnpaidFullDays       int
	UnpaidDecayRules     []DailyCheckinDecayRule
	LinuxDoExemptEnabled bool
}

func (s *DailyCheckinService) config(ctx context.Context) (dailyCheckinConfig, error) {
	cfg := dailyCheckinConfig{
		Enabled:              false,
		LegacyMode:           "fixed",
		LegacyAmount:         1,
		LegacyMin:            1,
		LegacyMax:            3,
		UnpaidFullDays:       7,
		UnpaidDecayRules:     DefaultDailyCheckinDecayRules(),
		LinuxDoExemptEnabled: false,
	}
	if s.settingService == nil {
		cfg.Prizes = legacyDailyCheckinPrizes(cfg.LegacyMode, cfg.LegacyAmount, cfg.LegacyMin, cfg.LegacyMax)
		return cfg, nil
	}
	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil {
		return cfg, fmt.Errorf("get daily check-in settings: %w", err)
	}
	cfg.Enabled = settings.DailyCheckinEnabled
	cfg.LegacyMode, cfg.LegacyAmount, cfg.LegacyMin, cfg.LegacyMax = normalizeDailyCheckinSettings(
		settings.DailyCheckinRewardMode,
		settings.DailyCheckinRewardAmount,
		settings.DailyCheckinRewardMin,
		settings.DailyCheckinRewardMax,
	)
	cfg.Prizes = normalizeDailyCheckinPrizes(settings.DailyCheckinPrizes, cfg.LegacyMode, cfg.LegacyAmount, cfg.LegacyMin, cfg.LegacyMax)
	cfg.UnpaidFullDays = normalizeDailyCheckinFullDays(settings.DailyCheckinUnpaidFullDays)
	cfg.UnpaidDecayRules = normalizeDailyCheckinDecayRules(settings.DailyCheckinUnpaidDecayRules)
	cfg.LinuxDoExemptEnabled = settings.DailyCheckinLinuxDoExemptEnabled
	return cfg, nil
}

func DefaultDailyCheckinDecayRules() []DailyCheckinDecayRule {
	return []DailyCheckinDecayRule{
		{AfterDays: 7, FactorBps: 5000},
		{AfterDays: 14, FactorBps: 2000},
		{AfterDays: 30, FactorBps: 0},
	}
}

func normalizeDailyCheckinPrizes(raw []DailyCheckinPrizeConfig, legacyMode string, legacyAmount, legacyMin, legacyMax float64) []DailyCheckinPrizeConfig {
	out := make([]DailyCheckinPrizeConfig, 0, len(raw))
	for i, prize := range raw {
		prize.ID = sanitizeDailyPrizeID(prize.ID, i)
		prize.Name = strings.TrimSpace(prize.Name)
		prize.Type = normalizeDailyPrizeType(prize.Type)
		if prize.Name == "" {
			prize.Name = defaultDailyPrizeName(prize.Type)
		}
		if prize.ProbabilityBps < 0 {
			prize.ProbabilityBps = 0
		}
		if prize.ProbabilityBps > DailyCheckinProbabilityTotal {
			prize.ProbabilityBps = DailyCheckinProbabilityTotal
		}
		if prize.BalanceMode != "range" {
			prize.BalanceMode = "fixed"
		}
		prize.Amount = roundCurrency(maxFloat(0, prize.Amount))
		prize.MinAmount = roundCurrency(maxFloat(0, prize.MinAmount))
		prize.MaxAmount = roundCurrency(maxFloat(prize.MinAmount, prize.MaxAmount))
		if prize.Concurrency < 0 {
			prize.Concurrency = 0
		}
		if prize.ValidityDays < 0 {
			prize.ValidityDays = 0
		}
		if prize.ValidityDays > MaxValidityDays {
			prize.ValidityDays = MaxValidityDays
		}
		out = append(out, prize)
	}
	if len(out) == 0 {
		return legacyDailyCheckinPrizes(legacyMode, legacyAmount, legacyMin, legacyMax)
	}
	return out
}

func legacyDailyCheckinPrizes(mode string, amount, minAmount, maxAmount float64) []DailyCheckinPrizeConfig {
	prize := DailyCheckinPrizeConfig{
		ID:             "legacy_balance",
		Name:           "余额奖励",
		Type:           DailyCheckinPrizeTypeBalance,
		ProbabilityBps: DailyCheckinProbabilityTotal,
		Enabled:        true,
		SortOrder:      0,
		BalanceMode:    mode,
		Amount:         roundCurrency(amount),
		MinAmount:      roundCurrency(minAmount),
		MaxAmount:      roundCurrency(maxAmount),
	}
	return []DailyCheckinPrizeConfig{prize}
}

func normalizeDailyCheckinFullDays(days int) int {
	if days < 0 {
		return 0
	}
	if days > 3650 {
		return 3650
	}
	return days
}

func normalizeDailyCheckinDecayRules(raw []DailyCheckinDecayRule) []DailyCheckinDecayRule {
	rules := raw
	if len(rules) == 0 {
		rules = DefaultDailyCheckinDecayRules()
	}
	out := make([]DailyCheckinDecayRule, 0, len(rules))
	for _, rule := range rules {
		if rule.AfterDays < 0 {
			rule.AfterDays = 0
		}
		if rule.FactorBps < 0 {
			rule.FactorBps = 0
		}
		if rule.FactorBps > DailyCheckinFactorFull {
			rule.FactorBps = DailyCheckinFactorFull
		}
		out = append(out, rule)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].AfterDays < out[i].AfterDays {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func buildEffectivePrizeViews(prizes []DailyCheckinPrizeConfig, factorBps int) []DailyCheckinPrizeView {
	if factorBps < 0 {
		factorBps = 0
	}
	if factorBps > DailyCheckinFactorFull {
		factorBps = DailyCheckinFactorFull
	}
	views := make([]DailyCheckinPrizeView, 0, len(prizes)+1)
	absorbed := 0
	noneIndex := -1
	for _, prize := range prizes {
		if !prize.Enabled || prize.ProbabilityBps <= 0 {
			continue
		}
		effective := prize.ProbabilityBps
		if prize.Type != DailyCheckinPrizeTypeNone {
			effective = prize.ProbabilityBps * factorBps / DailyCheckinFactorFull
			absorbed += prize.ProbabilityBps - effective
		}
		views = append(views, DailyCheckinPrizeView{
			DailyCheckinPrizeConfig: prize,
			EffectiveProbabilityBps: effective,
		})
		if prize.Type == DailyCheckinPrizeTypeNone && noneIndex == -1 {
			noneIndex = len(views) - 1
		}
	}
	if absorbed > 0 {
		if noneIndex >= 0 {
			views[noneIndex].EffectiveProbabilityBps += absorbed
		} else {
			views = append(views, DailyCheckinPrizeView{
				DailyCheckinPrizeConfig: DailyCheckinPrizeConfig{
					ID:             "system_thanks",
					Name:           "谢谢参与",
					Type:           DailyCheckinPrizeTypeNone,
					ProbabilityBps: 0,
					Enabled:        true,
					SortOrder:      9999,
				},
				EffectiveProbabilityBps: absorbed,
			})
		}
	}
	return views
}

func publicPrizeViews(prizes []DailyCheckinPrizeView) []DailyCheckinPublicPrizeView {
	out := make([]DailyCheckinPublicPrizeView, 0, len(prizes))
	for _, prize := range prizes {
		if prize.EffectiveProbabilityBps <= 0 {
			continue
		}
		out = append(out, DailyCheckinPublicPrizeView{
			ID:           prize.ID,
			Name:         prize.Name,
			Type:         prize.Type,
			SortOrder:    prize.SortOrder,
			BalanceMode:  prize.BalanceMode,
			Amount:       prize.Amount,
			MinAmount:    prize.MinAmount,
			MaxAmount:    prize.MaxAmount,
			Concurrency:  prize.Concurrency,
			GroupID:      prize.GroupID,
			ValidityDays: prize.ValidityDays,
		})
	}
	return out
}

func chooseDailyCheckinPrize(prizes []DailyCheckinPrizeView) (DailyCheckinPrizeView, error) {
	total := 0
	for _, prize := range prizes {
		if prize.EffectiveProbabilityBps > 0 {
			total += prize.EffectiveProbabilityBps
		}
	}
	if total <= 0 {
		return DailyCheckinPrizeView{}, infraerrors.InternalServer("DAILY_CHECKIN_PRIZES_INVALID", "daily check-in prize pool is invalid")
	}
	draw, err := rand.Int(rand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return DailyCheckinPrizeView{}, fmt.Errorf("draw daily check-in prize: %w", err)
	}
	ticket := int(draw.Int64()) + 1
	acc := 0
	for _, prize := range prizes {
		if prize.EffectiveProbabilityBps <= 0 {
			continue
		}
		acc += prize.EffectiveProbabilityBps
		if ticket <= acc {
			return prize, nil
		}
	}
	return prizes[len(prizes)-1], nil
}

func canDailyCheckinRole(role string) bool {
	switch role {
	case RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}

type dailyCheckinUserContext struct {
	ID             int64
	Role           string
	Balance        float64
	Concurrency    int
	TotalRecharged float64
	SignupSource   string
	CreatedAt      time.Time
}

func (s *DailyCheckinService) getUserCheckinContext(ctx context.Context, client *dbent.Client, userID int64, lock bool) (dailyCheckinUserContext, error) {
	if client == nil || userID <= 0 {
		return dailyCheckinUserContext{}, ErrUserNotFound
	}
	query := `
SELECT id, role, balance::double precision, concurrency, total_recharged::double precision, signup_source, created_at
FROM users
WHERE id = $1 AND deleted_at IS NULL`
	if lock {
		query += `
FOR UPDATE`
	}
	rows, err := client.QueryContext(ctx, query, userID)
	if err != nil {
		return dailyCheckinUserContext{}, fmt.Errorf("get daily check-in user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return dailyCheckinUserContext{}, fmt.Errorf("read daily check-in user: %w", err)
		}
		return dailyCheckinUserContext{}, ErrUserNotFound
	}
	var out dailyCheckinUserContext
	if err := rows.Scan(&out.ID, &out.Role, &out.Balance, &out.Concurrency, &out.TotalRecharged, &out.SignupSource, &out.CreatedAt); err != nil {
		return dailyCheckinUserContext{}, fmt.Errorf("scan daily check-in user: %w", err)
	}
	return out, rows.Err()
}

func (s *DailyCheckinService) decayStatus(ctx context.Context, client *dbent.Client, cfg dailyCheckinConfig, user dailyCheckinUserContext) (DailyCheckinDecayStatus, error) {
	accountAgeDays := int(timezonepkg.Today().Sub(startOfDay(user.CreatedAt)).Hours() / 24)
	if accountAgeDays < 0 {
		accountAgeDays = 0
	}
	status := DailyCheckinDecayStatus{
		AccountAgeDays: accountAgeDays,
		FactorBps:      DailyCheckinFactorFull,
		FullDays:       cfg.UnpaidFullDays,
	}
	paid, err := s.isPaidUser(ctx, client, user)
	if err != nil {
		return status, err
	}
	status.Paid = paid
	if paid {
		return status, nil
	}
	if cfg.LinuxDoExemptEnabled {
		exempt, err := s.isLinuxDoUser(ctx, client, user)
		if err != nil {
			return status, err
		}
		if exempt {
			status.Exempt = true
			status.ExemptReason = "linuxdo"
			return status, nil
		}
	}
	if accountAgeDays <= cfg.UnpaidFullDays {
		return status, nil
	}
	for _, rule := range cfg.UnpaidDecayRules {
		if accountAgeDays > rule.AfterDays {
			status.FactorBps = rule.FactorBps
		}
	}
	return status, nil
}

func (s *DailyCheckinService) isPaidUser(ctx context.Context, client *dbent.Client, user dailyCheckinUserContext) (bool, error) {
	if client == nil {
		return user.TotalRecharged > 0, nil
	}
	rows, err := client.QueryContext(ctx, `
SELECT 1
FROM payment_orders
WHERE user_id = $1
  AND status = $2
  AND pay_amount > 0
  AND order_type IN ('balance', 'subscription')
LIMIT 1`, user.ID, OrderStatusCompleted)
	if err != nil {
		return false, fmt.Errorf("check daily check-in paid status: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return true, rows.Err()
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return user.TotalRecharged > 0, nil
}

func (s *DailyCheckinService) isLinuxDoUser(ctx context.Context, client *dbent.Client, user dailyCheckinUserContext) (bool, error) {
	if strings.EqualFold(strings.TrimSpace(user.SignupSource), "linuxdo") {
		return true, nil
	}
	if client == nil {
		return false, nil
	}
	rows, err := client.QueryContext(ctx, `
SELECT 1
FROM auth_identities
WHERE user_id = $1 AND provider_type = 'linuxdo'
LIMIT 1`, user.ID)
	if err != nil {
		return false, fmt.Errorf("check daily check-in linuxdo exemption: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return true, rows.Err()
	}
	return false, rows.Err()
}

type dailyCheckinGrant struct {
	Type                  string
	Amount                float64
	BalanceAfter          float64
	Concurrency           int
	ConcurrencyBefore     *int
	ConcurrencyAfter      *int
	GroupID               *int64
	GroupName             string
	ValidityDays          int
	SubscriptionExpiresAt *time.Time
}

func (s *DailyCheckinService) prepareGrant(ctx context.Context, client *dbent.Client, user dailyCheckinUserContext, prize DailyCheckinPrizeView, now time.Time) (dailyCheckinGrant, error) {
	grant := dailyCheckinGrant{
		Type:         prize.Type,
		BalanceAfter: user.Balance,
	}
	switch prize.Type {
	case DailyCheckinPrizeTypeBalance:
		amount, err := prize.balanceReward()
		if err != nil {
			return grant, err
		}
		grant.Amount = amount
		grant.BalanceAfter = user.Balance + amount
	case DailyCheckinPrizeTypeConcurrency:
		if prize.Concurrency <= 0 {
			return grant, infraerrors.InternalServer("DAILY_CHECKIN_PRIZE_INVALID", "daily check-in concurrency prize is invalid")
		}
		before := user.Concurrency
		after := user.Concurrency + prize.Concurrency
		grant.Concurrency = prize.Concurrency
		grant.ConcurrencyBefore = &before
		grant.ConcurrencyAfter = &after
	case DailyCheckinPrizeTypeSubscription:
		sub, err := s.prepareSubscriptionGrant(ctx, client, user.ID, prize, now)
		if err != nil {
			return grant, err
		}
		grant.GroupID = &sub.GroupID
		grant.GroupName = sub.GroupName
		grant.ValidityDays = sub.ValidityDays
		grant.SubscriptionExpiresAt = &sub.ExpiresAt
	case DailyCheckinPrizeTypeNone:
		return grant, nil
	default:
		return grant, infraerrors.InternalServer("DAILY_CHECKIN_PRIZE_INVALID", "daily check-in prize type is invalid")
	}
	return grant, nil
}

func (s *DailyCheckinService) applyGrant(ctx context.Context, client *dbent.Client, userID int64, grant dailyCheckinGrant, now time.Time) error {
	switch grant.Type {
	case DailyCheckinPrizeTypeBalance:
		if grant.Amount <= 0 {
			return nil
		}
		_, err := client.ExecContext(ctx, `
UPDATE users
SET balance = balance + $2, updated_at = $3
WHERE id = $1 AND deleted_at IS NULL`, userID, grant.Amount, now)
		if err != nil {
			return fmt.Errorf("update daily check-in balance: %w", err)
		}
	case DailyCheckinPrizeTypeConcurrency:
		if grant.Concurrency <= 0 {
			return nil
		}
		_, err := client.ExecContext(ctx, `
UPDATE users
SET concurrency = concurrency + $2, updated_at = $3
WHERE id = $1 AND deleted_at IS NULL`, userID, grant.Concurrency, now)
		if err != nil {
			return fmt.Errorf("update daily check-in concurrency: %w", err)
		}
	case DailyCheckinPrizeTypeSubscription:
		if grant.GroupID == nil || grant.SubscriptionExpiresAt == nil {
			return infraerrors.InternalServer("DAILY_CHECKIN_SUBSCRIPTION_INVALID", "daily check-in subscription prize is invalid")
		}
		if err := s.applySubscriptionGrant(ctx, client, userID, *grant.GroupID, grant.GroupName, grant.ValidityDays, *grant.SubscriptionExpiresAt, now); err != nil {
			return err
		}
	}
	return nil
}

type dailySubscriptionGrant struct {
	GroupID      int64
	GroupName    string
	ValidityDays int
	ExpiresAt    time.Time
}

func (s *DailyCheckinService) prepareSubscriptionGrant(ctx context.Context, client *dbent.Client, userID int64, prize DailyCheckinPrizeView, now time.Time) (dailySubscriptionGrant, error) {
	if prize.GroupID <= 0 {
		return dailySubscriptionGrant{}, infraerrors.InternalServer("DAILY_CHECKIN_SUBSCRIPTION_INVALID", "daily check-in subscription prize is missing group")
	}
	validityDays := prize.ValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}
	if validityDays > MaxValidityDays {
		validityDays = MaxValidityDays
	}
	var groupName string
	rows, err := client.QueryContext(ctx, `
SELECT name
FROM groups
WHERE id = $1 AND status = $2 AND subscription_type = $3
LIMIT 1`, prize.GroupID, StatusActive, SubscriptionTypeSubscription)
	if err != nil {
		return dailySubscriptionGrant{}, fmt.Errorf("get daily check-in subscription group: %w", err)
	}
	if rows.Next() {
		if err := rows.Scan(&groupName); err != nil {
			_ = rows.Close()
			return dailySubscriptionGrant{}, fmt.Errorf("scan daily check-in subscription group: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return dailySubscriptionGrant{}, err
	}
	_ = rows.Close()
	if strings.TrimSpace(groupName) == "" {
		return dailySubscriptionGrant{}, infraerrors.InternalServer("DAILY_CHECKIN_SUBSCRIPTION_INVALID", "daily check-in subscription group is unavailable")
	}

	expiresAt := now.AddDate(0, 0, validityDays)
	subRows, err := client.QueryContext(ctx, `
SELECT expires_at
FROM user_subscriptions
WHERE user_id = $1 AND group_id = $2 AND deleted_at IS NULL
FOR UPDATE`, userID, prize.GroupID)
	if err != nil {
		return dailySubscriptionGrant{}, fmt.Errorf("lock daily check-in subscription: %w", err)
	}
	if subRows.Next() {
		var currentExpiresAt time.Time
		if err := subRows.Scan(&currentExpiresAt); err != nil {
			_ = subRows.Close()
			return dailySubscriptionGrant{}, fmt.Errorf("scan daily check-in subscription: %w", err)
		}
		if currentExpiresAt.After(now) {
			expiresAt = currentExpiresAt.AddDate(0, 0, validityDays)
		}
	}
	if err := subRows.Err(); err != nil {
		_ = subRows.Close()
		return dailySubscriptionGrant{}, err
	}
	_ = subRows.Close()
	if expiresAt.After(MaxExpiresAt) {
		expiresAt = MaxExpiresAt
	}
	return dailySubscriptionGrant{GroupID: prize.GroupID, GroupName: groupName, ValidityDays: validityDays, ExpiresAt: expiresAt}, nil
}

func (s *DailyCheckinService) applySubscriptionGrant(ctx context.Context, client *dbent.Client, userID, groupID int64, groupName string, validityDays int, expiresAt, now time.Time) error {
	notes := fmt.Sprintf("每日签到奖励：%s %d 天", groupName, validityDays)
	windowStart := startOfDay(now)
	result, err := client.ExecContext(ctx, `
UPDATE user_subscriptions
SET expires_at = $3,
    status = $4,
    updated_at = $5,
    notes = CASE WHEN notes IS NULL OR notes = '' THEN $6 ELSE notes || E'\n' || $6 END,
    daily_window_start = CASE WHEN expires_at <= $5 THEN $7 ELSE daily_window_start END,
    weekly_window_start = CASE WHEN expires_at <= $5 THEN $7 ELSE weekly_window_start END,
    monthly_window_start = CASE WHEN expires_at <= $5 THEN $7 ELSE monthly_window_start END,
    daily_usage_usd = CASE WHEN expires_at <= $5 THEN 0 ELSE daily_usage_usd END,
    weekly_usage_usd = CASE WHEN expires_at <= $5 THEN 0 ELSE weekly_usage_usd END,
    monthly_usage_usd = CASE WHEN expires_at <= $5 THEN 0 ELSE monthly_usage_usd END
WHERE user_id = $1 AND group_id = $2 AND deleted_at IS NULL`, userID, groupID, expiresAt, SubscriptionStatusActive, now, notes, windowStart)
	if err != nil {
		return fmt.Errorf("update daily check-in subscription: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read daily check-in subscription update result: %w", err)
	}
	if affected > 0 {
		return nil
	}
	_, err = client.ExecContext(ctx, `
INSERT INTO user_subscriptions (
    user_id, group_id, starts_at, expires_at, status,
    daily_window_start, weekly_window_start, monthly_window_start,
    daily_usage_usd, weekly_usage_usd, monthly_usage_usd,
    assigned_at, notes, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $6, $6, 0, 0, 0, $3, $7, $3, $3)`,
		userID, groupID, now, expiresAt, SubscriptionStatusActive, windowStart, notes)
	if err != nil {
		return fmt.Errorf("create daily check-in subscription: %w", err)
	}
	return nil
}

func (s *DailyCheckinService) invalidateGrantCaches(ctx context.Context, userID int64, grant dailyCheckinGrant) {
	switch grant.Type {
	case DailyCheckinPrizeTypeBalance, DailyCheckinPrizeTypeConcurrency:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if grant.Type == DailyCheckinPrizeTypeBalance && s.billingCacheService != nil {
			go func() {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
			}()
		}
	case DailyCheckinPrizeTypeSubscription:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if grant.GroupID == nil {
			return
		}
		groupID := *grant.GroupID
		if s.subscriptionService != nil {
			s.subscriptionService.InvalidateSubCache(userID, groupID)
		}
		if s.billingCacheService != nil {
			go func() {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID)
			}()
		}
	}
}

func (g dailyCheckinGrant) toRewardView(prize DailyCheckinPrizeView, checkedInAt time.Time) DailyCheckinRewardView {
	out := DailyCheckinRewardView{
		PrizeID:      prize.ID,
		PrizeName:    prize.Name,
		Type:         prize.Type,
		Amount:       g.Amount,
		Concurrency:  g.Concurrency,
		GroupID:      g.GroupID,
		GroupName:    g.GroupName,
		ValidityDays: g.ValidityDays,
		CheckedInAt:  checkedInAt.Format(time.RFC3339),
	}
	if prize.Type == DailyCheckinPrizeTypeBalance {
		v := g.BalanceAfter
		out.NewBalance = &v
	}
	if prize.Type == DailyCheckinPrizeTypeConcurrency && g.ConcurrencyAfter != nil {
		v := *g.ConcurrencyAfter
		out.NewConcurrency = &v
	}
	if g.SubscriptionExpiresAt != nil {
		v := g.SubscriptionExpiresAt.Format(time.RFC3339)
		out.SubscriptionEnds = &v
	}
	return out
}

func (s *DailyCheckinService) getTodayRecord(ctx context.Context, client *dbent.Client, userID int64) (*DailyCheckinRecord, error) {
	records, err := queryDailyCheckinRecords(ctx, client, `
WHERE user_id = $1 AND checkin_date = $2
ORDER BY created_at DESC, id DESC
LIMIT 1`, userID, todayDateString())
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[0], nil
}

func (s *DailyCheckinService) recentRecords(ctx context.Context, userID int64, limit int) ([]DailyCheckinRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	return queryDailyCheckinRecords(ctx, s.client, `
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`, userID, limit)
}

func queryDailyCheckinRecords(ctx context.Context, client *dbent.Client, where string, args ...any) ([]DailyCheckinRecord, error) {
	rows, err := client.QueryContext(ctx, `
SELECT id, prize_id, prize_name, reward_type, reward_amount::double precision,
       concurrency_before, concurrency_after, subscription_group_id, subscription_validity_days, created_at
FROM daily_checkins
`+where, args...)
	if err != nil {
		return nil, fmt.Errorf("query daily check-in records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]DailyCheckinRecord, 0)
	for rows.Next() {
		var (
			record            DailyCheckinRecord
			concurrencyBefore sql.NullInt64
			concurrencyAfter  sql.NullInt64
			groupID           sql.NullInt64
			validityDays      sql.NullInt64
			checkedInAtTime   time.Time
		)
		if err := rows.Scan(&record.ID, &record.PrizeID, &record.PrizeName, &record.Type, &record.Amount, &concurrencyBefore, &concurrencyAfter, &groupID, &validityDays, &checkedInAtTime); err != nil {
			return nil, fmt.Errorf("scan daily check-in record: %w", err)
		}
		if record.PrizeID == "" {
			record.PrizeID = "legacy_balance"
		}
		if record.PrizeName == "" {
			record.PrizeName = defaultDailyPrizeName(record.Type)
		}
		if record.Type == "" {
			record.Type = DailyCheckinPrizeTypeBalance
		}
		if v, ok := dailyCheckinConcurrencyDelta(concurrencyBefore, concurrencyAfter); ok {
			record.Concurrency = v
		}
		if groupID.Valid {
			v := groupID.Int64
			record.GroupID = &v
		}
		if validityDays.Valid {
			record.ValidityDays = int(validityDays.Int64)
		}
		record.CheckedInAt = checkedInAtTime.Format(time.RFC3339)
		out = append(out, record)
	}
	return out, rows.Err()
}

func dailyCheckinConcurrencyDelta(before, after sql.NullInt64) (int, bool) {
	if before.Valid && after.Valid {
		return int(after.Int64 - before.Int64), true
	}
	if after.Valid {
		return int(after.Int64), true
	}
	return 0, false
}

func (p DailyCheckinPrizeView) balanceReward() (float64, error) {
	if p.BalanceMode != "range" {
		return roundCurrency(p.Amount), nil
	}
	minCents := amountToCents(p.MinAmount)
	maxCents := amountToCents(p.MaxAmount)
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

func ValidateDailyCheckinPrizeSettings(prizes []DailyCheckinPrizeConfig, enabled bool) error {
	normalized := normalizeDailyCheckinPrizes(prizes, "fixed", 1, 1, 3)
	sum := 0
	active := 0
	for _, prize := range normalized {
		if !prize.Enabled {
			continue
		}
		active++
		sum += prize.ProbabilityBps
		switch prize.Type {
		case DailyCheckinPrizeTypeBalance:
			if prize.BalanceMode == "range" {
				if prize.MaxAmount < prize.MinAmount || (enabled && prize.MaxAmount <= 0) {
					return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "invalid balance range prize")
				}
			} else if enabled && prize.Amount <= 0 {
				return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "balance prize amount must be greater than 0")
			}
		case DailyCheckinPrizeTypeConcurrency:
			if enabled && prize.Concurrency <= 0 {
				return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "concurrency prize must be greater than 0")
			}
		case DailyCheckinPrizeTypeSubscription:
			if enabled && (prize.GroupID <= 0 || prize.ValidityDays <= 0) {
				return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "subscription prize requires group_id and validity_days")
			}
		case DailyCheckinPrizeTypeNone:
		default:
			return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "invalid daily check-in prize type")
		}
	}
	if enabled {
		if active == 0 {
			return infraerrors.BadRequest("DAILY_CHECKIN_PRIZES_REQUIRED", "at least one daily check-in prize is required")
		}
		if sum != DailyCheckinProbabilityTotal {
			return infraerrors.BadRequest("DAILY_CHECKIN_PROBABILITY_INVALID", "daily check-in prize probability must add up to 100%")
		}
	}
	return nil
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

func sanitizeDailyPrizeID(id string, index int) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Sprintf("prize_%d", index+1)
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			_, _ = b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("prize_%d", index+1)
	}
	return b.String()
}

func normalizeDailyPrizeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DailyCheckinPrizeTypeConcurrency:
		return DailyCheckinPrizeTypeConcurrency
	case DailyCheckinPrizeTypeSubscription:
		return DailyCheckinPrizeTypeSubscription
	case DailyCheckinPrizeTypeNone:
		return DailyCheckinPrizeTypeNone
	default:
		return DailyCheckinPrizeTypeBalance
	}
}

func defaultDailyPrizeName(prizeType string) string {
	switch prizeType {
	case DailyCheckinPrizeTypeConcurrency:
		return "并发奖励"
	case DailyCheckinPrizeTypeSubscription:
		return "订阅奖励"
	case DailyCheckinPrizeTypeNone:
		return "谢谢参与"
	default:
		return "余额奖励"
	}
}

func maxFloat(a, b float64) float64 {
	if b < a {
		return a
	}
	return b
}

func nullableIntArg(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64Value(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTimeValue(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
