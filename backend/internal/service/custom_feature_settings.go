package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrCustomFeatureGroupInvalid = infraerrors.BadRequest(
		"CUSTOM_FEATURE_GROUP_INVALID",
		"custom feature group must exist and be active",
	)
	ErrDailyCheckinDecayInvalid = infraerrors.BadRequest(
		"DAILY_CHECKIN_DECAY_INVALID",
		"daily check-in decay settings are invalid",
	)
)

// ModelMarketplaceSettings 是模型广场的独立管理配置。
type ModelMarketplaceSettings struct {
	Enabled  bool    `json:"enabled"`
	Intro    string  `json:"intro"`
	GroupIDs []int64 `json:"group_ids"`
}

// DailyCheckinSettings 是每日签到的独立管理配置。
type DailyCheckinSettings struct {
	Enabled              bool                      `json:"enabled"`
	Prizes               []DailyCheckinPrizeConfig `json:"prizes"`
	UnpaidFullDays       int                       `json:"unpaid_full_days"`
	UnpaidDecayRules     []DailyCheckinDecayRule   `json:"unpaid_decay_rules"`
	LinuxDoExemptEnabled bool                      `json:"linuxdo_exempt_enabled"`

	// 兼容旧版单一余额奖励，管理接口不直接暴露这些字段。
	LegacyMode   string  `json:"-"`
	LegacyAmount float64 `json:"-"`
	LegacyMin    float64 `json:"-"`
	LegacyMax    float64 `json:"-"`
}

// CustomFeatureSettings 聚合二开功能的独立管理配置。
type CustomFeatureSettings struct {
	ModelMarketplace ModelMarketplaceSettings `json:"model_marketplace"`
	DailyCheckin     DailyCheckinSettings     `json:"daily_checkin"`
}

var customFeatureSettingKeys = []string{
	SettingKeyModelMarketplaceEnabled,
	SettingKeyModelMarketplaceIntro,
	SettingKeyModelMarketplaceGroupIDs,
	SettingKeyDailyCheckinEnabled,
	SettingKeyDailyCheckinMode,
	SettingKeyDailyCheckinAmount,
	SettingKeyDailyCheckinMin,
	SettingKeyDailyCheckinMax,
	SettingKeyDailyCheckinPrizes,
	SettingKeyDailyCheckinUnpaidFullDays,
	SettingKeyDailyCheckinUnpaidDecayRules,
	SettingKeyDailyCheckinLinuxDoExemptEnabled,
}

// GetCustomFeatureSettings 读取模型广场和每日签到配置。
func (s *SettingService) GetCustomFeatureSettings(ctx context.Context) (*CustomFeatureSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, customFeatureSettingKeys)
	if err != nil {
		return nil, fmt.Errorf("get custom feature settings: %w", err)
	}

	marketplace := ModelMarketplaceSettings{
		Enabled:  !isFalseSettingValue(values[SettingKeyModelMarketplaceEnabled]),
		Intro:    strings.TrimSpace(values[SettingKeyModelMarketplaceIntro]),
		GroupIDs: normalizeCustomFeatureGroupIDs(parseCustomFeatureGroupIDs(values[SettingKeyModelMarketplaceGroupIDs])),
	}
	if marketplace.GroupIDs == nil {
		marketplace.GroupIDs = []int64{}
	}

	mode, amount, minAmount, maxAmount := normalizeDailyCheckinSettings(
		values[SettingKeyDailyCheckinMode],
		parseCustomFeatureFloat(values[SettingKeyDailyCheckinAmount], 1),
		parseCustomFeatureFloat(values[SettingKeyDailyCheckinMin], 1),
		parseCustomFeatureFloat(values[SettingKeyDailyCheckinMax], 3),
	)
	daily := DailyCheckinSettings{
		Enabled:              values[SettingKeyDailyCheckinEnabled] == "true",
		Prizes:               parseCustomFeaturePrizes(values[SettingKeyDailyCheckinPrizes], mode, amount, minAmount, maxAmount),
		UnpaidFullDays:       normalizeDailyCheckinFullDays(parseCustomFeatureInt(values[SettingKeyDailyCheckinUnpaidFullDays], 7)),
		UnpaidDecayRules:     parseCustomFeatureDecayRules(values[SettingKeyDailyCheckinUnpaidDecayRules]),
		LinuxDoExemptEnabled: values[SettingKeyDailyCheckinLinuxDoExemptEnabled] == "true",
		LegacyMode:           mode,
		LegacyAmount:         amount,
		LegacyMin:            minAmount,
		LegacyMax:            maxAmount,
	}

	return &CustomFeatureSettings{
		ModelMarketplace: marketplace,
		DailyCheckin:     daily,
	}, nil
}

// GetDailyCheckinSettings 读取签到运行时配置，确保与管理接口使用同一解析逻辑。
func (s *SettingService) GetDailyCheckinSettings(ctx context.Context) (*DailyCheckinSettings, error) {
	settings, err := s.GetCustomFeatureSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &settings.DailyCheckin, nil
}

// UpdateModelMarketplaceSettings 只更新模型广场相关设置键。
func (s *SettingService) UpdateModelMarketplaceSettings(ctx context.Context, input ModelMarketplaceSettings) (*ModelMarketplaceSettings, error) {
	input.Intro = strings.TrimSpace(input.Intro)
	input.GroupIDs = normalizeCustomFeatureGroupIDs(input.GroupIDs)
	if err := s.validateCustomFeatureGroups(ctx, input.GroupIDs, false); err != nil {
		return nil, err
	}
	groupIDsJSON, err := json.Marshal(input.GroupIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal model marketplace group ids: %w", err)
	}

	updates := map[string]string{
		SettingKeyModelMarketplaceEnabled:  strconv.FormatBool(input.Enabled),
		SettingKeyModelMarketplaceIntro:    input.Intro,
		SettingKeyModelMarketplaceGroupIDs: string(groupIDsJSON),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, fmt.Errorf("update model marketplace settings: %w", err)
	}
	s.notifyCustomFeatureSettingsUpdated()
	return &input, nil
}

// UpdateDailyCheckinSettings 只更新每日签到相关设置键。
func (s *SettingService) UpdateDailyCheckinSettings(ctx context.Context, input DailyCheckinSettings) (*DailyCheckinSettings, error) {
	if input.Enabled && len(input.Prizes) == 0 {
		return nil, infraerrors.BadRequest("DAILY_CHECKIN_PRIZES_REQUIRED", "at least one daily check-in prize is required")
	}
	if err := validateDailyCheckinDecaySettings(input.UnpaidFullDays, input.UnpaidDecayRules); err != nil {
		return nil, err
	}
	if err := validateDailyCheckinPrizeInput(input.Prizes); err != nil {
		return nil, err
	}
	if err := ValidateDailyCheckinPrizeSettings(input.Prizes, input.Enabled); err != nil {
		return nil, err
	}

	input.Prizes = normalizeDailyCheckinPrizes(input.Prizes, "fixed", 1, 1, 3)
	input.UnpaidFullDays = normalizeDailyCheckinFullDays(input.UnpaidFullDays)
	input.UnpaidDecayRules = normalizeDailyCheckinDecayRules(input.UnpaidDecayRules)
	if err := s.validateDailyCheckinSubscriptionPrizes(ctx, input.Prizes); err != nil {
		return nil, err
	}

	prizesJSON, err := json.Marshal(input.Prizes)
	if err != nil {
		return nil, fmt.Errorf("marshal daily check-in prizes: %w", err)
	}
	decayRulesJSON, err := json.Marshal(input.UnpaidDecayRules)
	if err != nil {
		return nil, fmt.Errorf("marshal daily check-in decay rules: %w", err)
	}

	updates := map[string]string{
		SettingKeyDailyCheckinEnabled:              strconv.FormatBool(input.Enabled),
		SettingKeyDailyCheckinPrizes:               string(prizesJSON),
		SettingKeyDailyCheckinUnpaidFullDays:       strconv.Itoa(input.UnpaidFullDays),
		SettingKeyDailyCheckinUnpaidDecayRules:     string(decayRulesJSON),
		SettingKeyDailyCheckinLinuxDoExemptEnabled: strconv.FormatBool(input.LinuxDoExemptEnabled),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, fmt.Errorf("update daily check-in settings: %w", err)
	}
	s.notifyCustomFeatureSettingsUpdated()
	return &input, nil
}

func (s *SettingService) validateCustomFeatureGroups(ctx context.Context, groupIDs []int64, subscriptionOnly bool) error {
	if s.defaultSubGroupReader == nil {
		return nil
	}
	for _, groupID := range groupIDs {
		group, err := s.defaultSubGroupReader.GetByID(ctx, groupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				return ErrCustomFeatureGroupInvalid.WithMetadata(map[string]string{"group_id": strconv.FormatInt(groupID, 10)})
			}
			return fmt.Errorf("get custom feature group %d: %w", groupID, err)
		}
		if group == nil || !group.IsActive() || (subscriptionOnly && !group.IsSubscriptionType()) {
			return ErrCustomFeatureGroupInvalid.WithMetadata(map[string]string{"group_id": strconv.FormatInt(groupID, 10)})
		}
	}
	return nil
}

func (s *SettingService) validateDailyCheckinSubscriptionPrizes(ctx context.Context, prizes []DailyCheckinPrizeConfig) error {
	for _, prize := range prizes {
		if !prize.Enabled || prize.Type != DailyCheckinPrizeTypeSubscription {
			continue
		}
		if err := s.validateCustomFeatureGroups(ctx, []int64{prize.GroupID}, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *SettingService) notifyCustomFeatureSettingsUpdated() {
	if s.onUpdate != nil {
		s.onUpdate()
	}
}

func validateDailyCheckinDecaySettings(fullDays int, rules []DailyCheckinDecayRule) error {
	if fullDays < 0 || fullDays > 3650 {
		return ErrDailyCheckinDecayInvalid
	}
	for _, rule := range rules {
		if rule.AfterDays < 0 || rule.AfterDays > 3650 || rule.FactorBps < 0 || rule.FactorBps > DailyCheckinFactorFull {
			return ErrDailyCheckinDecayInvalid
		}
	}
	return nil
}

func validateDailyCheckinPrizeInput(prizes []DailyCheckinPrizeConfig) error {
	for _, prize := range prizes {
		if !prize.Enabled {
			continue
		}
		if strings.TrimSpace(prize.Name) == "" {
			return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "enabled daily check-in prize name is required")
		}
		if prize.ProbabilityBps < 0 || prize.ProbabilityBps > DailyCheckinProbabilityTotal {
			return infraerrors.BadRequest("DAILY_CHECKIN_PRIZE_INVALID", "daily check-in prize probability is out of range")
		}
	}
	return nil
}

func normalizeCustomFeatureGroupIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseCustomFeatureGroupIDs(raw string) []int64 {
	var ids []int64
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &ids)
	}
	return ids
}

func parseCustomFeaturePrizes(raw, legacyMode string, legacyAmount, legacyMin, legacyMax float64) []DailyCheckinPrizeConfig {
	var prizes []DailyCheckinPrizeConfig
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &prizes)
	}
	return normalizeDailyCheckinPrizes(prizes, legacyMode, legacyAmount, legacyMin, legacyMax)
}

func parseCustomFeatureDecayRules(raw string) []DailyCheckinDecayRule {
	var rules []DailyCheckinDecayRule
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &rules)
	}
	return normalizeDailyCheckinDecayRules(rules)
}

func parseCustomFeatureFloat(raw string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return value
}

func parseCustomFeatureInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}
