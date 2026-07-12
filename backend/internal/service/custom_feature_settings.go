package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

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
	ErrGatewaySettingsInvalid = infraerrors.BadRequest(
		"GATEWAY_SETTINGS_INVALID",
		"网关配置无效",
	)
)

const (
	DefaultGatewayPoolModeRetryCount                    = 1
	DefaultGatewayFirstTokenTimeout                     = 60
	DefaultGatewayFirstTokenTimeoutConsecutiveThreshold = 3
	DefaultGatewayUpstreamErrorConsecutiveThreshold     = 10
	DefaultGatewayFailurePolicyRevision                 = int64(1)
	MaxGatewayPoolModeRetryCount                        = 10
	MaxGatewayFirstTokenTimeoutSeconds                  = 600
	MaxGatewayFailureConsecutiveThreshold               = 100
	gatewaySettingsCacheTTL                             = 60 * time.Second
	gatewaySettingsErrorCacheTTL                        = 5 * time.Second
	gatewaySettingsDBTimeout                            = 5 * time.Second
)

var (
	defaultGatewayPoolModeRetryStatusCodes = []int{401, 403, 429, 502, 503, 504}
	defaultGatewayUpstreamErrorStatusCodes = []int{502, 503, 504}
	defaultGatewayProbeBackoffMinutes      = []int{5, 10, 15, 30, 60}
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

// GatewaySettings 是二开功能中的网关运行配置。
type GatewaySettings struct {
	DefaultPoolModeRetryCount             int   `json:"default_pool_mode_retry_count"`
	DefaultPoolModeRetryStatusCodes       []int `json:"default_pool_mode_retry_status_codes"`
	AutoManagedProbeBackoffMinutes        []int `json:"auto_managed_probe_backoff_minutes"`
	FirstTokenTimeoutSeconds              int   `json:"first_token_timeout_seconds"`
	FirstTokenTimeoutConsecutiveThreshold int   `json:"first_token_timeout_consecutive_threshold"`
	UpstreamErrorStatusCodes              []int `json:"upstream_error_status_codes"`
	UpstreamErrorConsecutiveThreshold     int   `json:"upstream_error_consecutive_threshold"`
	ImageGroupSuccessRateVisible          bool  `json:"image_group_success_rate_visible"`
	FailurePolicyRevision                 int64 `json:"-"`
}

type gatewaySettingsRevisionWriter interface {
	SetMultipleWithMonotonicRevision(
		ctx context.Context,
		settings map[string]string,
		revisionKey string,
		fingerprintKey string,
		initialRevision int64,
		currentFingerprintFallback string,
		desiredFingerprint string,
	) (int64, error)
}

type cachedGatewaySettings struct {
	settings  GatewaySettings
	expiresAt int64
}

// CustomFeatureSettings 聚合二开功能的独立管理配置。
type CustomFeatureSettings struct {
	ModelMarketplace ModelMarketplaceSettings `json:"model_marketplace"`
	DailyCheckin     DailyCheckinSettings     `json:"daily_checkin"`
	Gateway          GatewaySettings          `json:"gateway"`
}

var gatewaySettingKeys = []string{
	SettingKeyGatewayDefaultPoolModeRetryCount,
	SettingKeyGatewayDefaultPoolModeRetryStatusCodes,
	SettingKeyGatewayAutoManagedProbeBackoffMinutes,
	SettingKeyGatewayFirstTokenTimeoutSeconds,
	SettingKeyGatewayFirstTokenTimeoutConsecutiveThreshold,
	SettingKeyGatewayUpstreamErrorStatusCodes,
	SettingKeyGatewayUpstreamErrorConsecutiveThreshold,
	SettingKeyGatewayImageGroupSuccessRateVisible,
	SettingKeyGatewayFailurePolicyRevision,
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
	SettingKeyGatewayDefaultPoolModeRetryCount,
	SettingKeyGatewayDefaultPoolModeRetryStatusCodes,
	SettingKeyGatewayAutoManagedProbeBackoffMinutes,
	SettingKeyGatewayFirstTokenTimeoutSeconds,
	SettingKeyGatewayFirstTokenTimeoutConsecutiveThreshold,
	SettingKeyGatewayUpstreamErrorStatusCodes,
	SettingKeyGatewayUpstreamErrorConsecutiveThreshold,
	SettingKeyGatewayImageGroupSuccessRateVisible,
	SettingKeyGatewayFailurePolicyRevision,
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
	gateway := parseGatewaySettings(values)
	s.storeGatewaySettingsCache(gateway, gatewaySettingsCacheTTL)

	return &CustomFeatureSettings{
		ModelMarketplace: marketplace,
		DailyCheckin:     daily,
		Gateway:          gateway,
	}, nil
}

// DefaultGatewaySettings 返回未持久化配置时使用的默认值。
func DefaultGatewaySettings() GatewaySettings {
	return GatewaySettings{
		DefaultPoolModeRetryCount:             DefaultGatewayPoolModeRetryCount,
		DefaultPoolModeRetryStatusCodes:       append([]int(nil), defaultGatewayPoolModeRetryStatusCodes...),
		AutoManagedProbeBackoffMinutes:        append([]int(nil), defaultGatewayProbeBackoffMinutes...),
		FirstTokenTimeoutSeconds:              DefaultGatewayFirstTokenTimeout,
		FirstTokenTimeoutConsecutiveThreshold: DefaultGatewayFirstTokenTimeoutConsecutiveThreshold,
		UpstreamErrorStatusCodes:              append([]int(nil), defaultGatewayUpstreamErrorStatusCodes...),
		UpstreamErrorConsecutiveThreshold:     DefaultGatewayUpstreamErrorConsecutiveThreshold,
		ImageGroupSuccessRateVisible:          true,
		FailurePolicyRevision:                 DefaultGatewayFailurePolicyRevision,
	}
}

// GetGatewaySettings 读取持久化网关配置并应用安全默认值。
func (s *SettingService) GetGatewaySettings(ctx context.Context) (*GatewaySettings, error) {
	if s == nil || s.settingRepo == nil {
		settings := DefaultGatewaySettings()
		return &settings, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, gatewaySettingKeys)
	if err != nil {
		return nil, fmt.Errorf("读取网关配置: %w", err)
	}
	settings := parseGatewaySettings(values)
	s.storeGatewaySettingsCache(settings, gatewaySettingsCacheTTL)
	return &settings, nil
}

// GetGatewayRuntime 为网关热路径提供带进程内缓存的配置快照。
func (s *SettingService) GetGatewayRuntime(ctx context.Context) GatewaySettings {
	if s == nil {
		return DefaultGatewaySettings()
	}
	if cached, ok := s.gatewaySettingsCache.Load().(*cachedGatewaySettings); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
		return cloneGatewaySettings(cached.settings)
	}

	result, _, _ := s.gatewaySettingsSF.Do("gateway_settings", func() (any, error) {
		if cached, ok := s.gatewaySettingsCache.Load().(*cachedGatewaySettings); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
			return cloneGatewaySettings(cached.settings), nil
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewaySettingsDBTimeout)
		defer cancel()
		settings, err := s.GetGatewaySettings(dbCtx)
		if err != nil {
			slog.Warn("读取网关运行配置失败，使用缓存或默认值", "error", err)
			fallback := DefaultGatewaySettings()
			if cached, ok := s.gatewaySettingsCache.Load().(*cachedGatewaySettings); ok && cached != nil {
				fallback = cloneGatewaySettings(cached.settings)
			}
			s.storeGatewaySettingsCache(fallback, gatewaySettingsErrorCacheTTL)
			return fallback, nil
		}
		return cloneGatewaySettings(*settings), nil
	})
	if settings, ok := result.(GatewaySettings); ok {
		return cloneGatewaySettings(settings)
	}
	return DefaultGatewaySettings()
}

// UpdateGatewaySettings 校验、规范化并保存网关配置。
func (s *SettingService) UpdateGatewaySettings(ctx context.Context, input GatewaySettings) (*GatewaySettings, error) {
	if err := validateGatewaySettings(&input); err != nil {
		return nil, err
	}
	current, err := s.GetGatewaySettings(ctx)
	if err != nil {
		return nil, err
	}
	currentFingerprint := BuildGatewayFailurePolicyFingerprint(*current)
	desiredFingerprint := BuildGatewayFailurePolicyFingerprint(input)
	statusCodesJSON, err := json.Marshal(input.DefaultPoolModeRetryStatusCodes)
	if err != nil {
		return nil, fmt.Errorf("序列化默认重试状态码: %w", err)
	}
	backoffJSON, err := json.Marshal(input.AutoManagedProbeBackoffMinutes)
	if err != nil {
		return nil, fmt.Errorf("序列化自动测活退避配置: %w", err)
	}
	upstreamErrorStatusCodesJSON, err := json.Marshal(input.UpstreamErrorStatusCodes)
	if err != nil {
		return nil, fmt.Errorf("序列化上游错误状态码: %w", err)
	}
	updates := map[string]string{
		SettingKeyGatewayDefaultPoolModeRetryCount:             strconv.Itoa(input.DefaultPoolModeRetryCount),
		SettingKeyGatewayDefaultPoolModeRetryStatusCodes:       string(statusCodesJSON),
		SettingKeyGatewayAutoManagedProbeBackoffMinutes:        string(backoffJSON),
		SettingKeyGatewayFirstTokenTimeoutSeconds:              strconv.Itoa(input.FirstTokenTimeoutSeconds),
		SettingKeyGatewayFirstTokenTimeoutConsecutiveThreshold: strconv.Itoa(input.FirstTokenTimeoutConsecutiveThreshold),
		SettingKeyGatewayUpstreamErrorStatusCodes:              string(upstreamErrorStatusCodesJSON),
		SettingKeyGatewayUpstreamErrorConsecutiveThreshold:     strconv.Itoa(input.UpstreamErrorConsecutiveThreshold),
		SettingKeyGatewayImageGroupSuccessRateVisible:          strconv.FormatBool(input.ImageGroupSuccessRateVisible),
	}
	var revision int64
	if writer, ok := s.settingRepo.(gatewaySettingsRevisionWriter); ok {
		revision, err = writer.SetMultipleWithMonotonicRevision(
			ctx,
			updates,
			SettingKeyGatewayFailurePolicyRevision,
			SettingKeyGatewayFailurePolicyFingerprint,
			DefaultGatewayFailurePolicyRevision,
			currentFingerprint,
			desiredFingerprint,
		)
	} else {
		revision = current.FailurePolicyRevision
		if revision <= 0 {
			revision = DefaultGatewayFailurePolicyRevision
		}
		if currentFingerprint != desiredFingerprint {
			if revision == int64(^uint64(0)>>1) {
				err = fmt.Errorf("网关失败策略代次已耗尽")
			} else {
				revision++
			}
		}
		if err == nil {
			updates[SettingKeyGatewayFailurePolicyRevision] = strconv.FormatInt(revision, 10)
			updates[SettingKeyGatewayFailurePolicyFingerprint] = desiredFingerprint
			err = s.settingRepo.SetMultiple(ctx, updates)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("更新网关配置: %w", err)
	}
	input.FailurePolicyRevision = revision
	s.storeGatewaySettingsCache(input, gatewaySettingsCacheTTL)
	if s.scheduledTestPlanRepo != nil {
		if err := s.scheduledTestPlanRepo.RescheduleEnabledAutoManaged(ctx, input.AutoManagedProbeBackoffDurations(), time.Now()); err != nil {
			return nil, fmt.Errorf("重排自动测活计划: %w", err)
		}
	}
	s.notifyCustomFeatureSettingsUpdated()
	result := cloneGatewaySettings(input)
	return &result, nil
}

// SetScheduledTestPlanRepository 注入自动测活计划仓储。
func (s *SettingService) SetScheduledTestPlanRepository(repo ScheduledTestPlanRepository) {
	if s != nil {
		s.scheduledTestPlanRepo = repo
	}
}

// AutoManagedProbeBackoffDurations 返回可直接用于调度器的退避时长。
func (s GatewaySettings) AutoManagedProbeBackoffDurations() []time.Duration {
	minutes := s.AutoManagedProbeBackoffMinutes
	if len(minutes) == 0 {
		minutes = defaultGatewayProbeBackoffMinutes
	}
	result := make([]time.Duration, len(minutes))
	for i, minute := range minutes {
		result[i] = time.Duration(minute) * time.Minute
	}
	return result
}

// ApplyGatewayPoolModeDefaults 仅为新建的 apikey/bedrock 账号补齐池模式默认值。
func ApplyGatewayPoolModeDefaults(accountType string, credentials map[string]any, settings GatewaySettings) map[string]any {
	accountType = strings.ToLower(strings.TrimSpace(accountType))
	if accountType != AccountTypeAPIKey && accountType != AccountTypeBedrock {
		return credentials
	}
	if credentials == nil {
		credentials = make(map[string]any)
	}
	if poolMode, ok := credentials["pool_mode"].(bool); ok && !poolMode {
		return credentials
	}
	if _, ok := credentials["pool_mode"]; !ok {
		credentials["pool_mode"] = true
	}
	if _, ok := credentials["pool_mode_retry_count"]; !ok {
		credentials["pool_mode_retry_count"] = settings.DefaultPoolModeRetryCount
	}
	if _, ok := credentials["pool_mode_retry_status_codes"]; !ok {
		credentials["pool_mode_retry_status_codes"] = append([]int(nil), settings.DefaultPoolModeRetryStatusCodes...)
	}
	return credentials
}

func parseGatewaySettings(values map[string]string) GatewaySettings {
	settings := DefaultGatewaySettings()
	if value, err := strconv.Atoi(strings.TrimSpace(values[SettingKeyGatewayDefaultPoolModeRetryCount])); err == nil && value >= 0 && value <= MaxGatewayPoolModeRetryCount {
		settings.DefaultPoolModeRetryCount = value
	}
	if raw, ok := values[SettingKeyGatewayDefaultPoolModeRetryStatusCodes]; ok && strings.TrimSpace(raw) != "" {
		var codes []int
		if err := json.Unmarshal([]byte(raw), &codes); err == nil && validateRetryStatusCodes(codes) == nil {
			settings.DefaultPoolModeRetryStatusCodes = normalizeRetryStatusCodes(codes)
		}
	}
	if raw, ok := values[SettingKeyGatewayAutoManagedProbeBackoffMinutes]; ok && strings.TrimSpace(raw) != "" {
		var minutes []int
		if err := json.Unmarshal([]byte(raw), &minutes); err == nil && validateProbeBackoffMinutes(minutes) == nil {
			settings.AutoManagedProbeBackoffMinutes = append([]int(nil), minutes...)
		}
	}
	if value, err := strconv.Atoi(strings.TrimSpace(values[SettingKeyGatewayFirstTokenTimeoutSeconds])); err == nil && value >= 0 && value <= MaxGatewayFirstTokenTimeoutSeconds {
		settings.FirstTokenTimeoutSeconds = value
	}
	if value, err := strconv.Atoi(strings.TrimSpace(values[SettingKeyGatewayFirstTokenTimeoutConsecutiveThreshold])); err == nil && value >= 1 && value <= MaxGatewayFailureConsecutiveThreshold {
		settings.FirstTokenTimeoutConsecutiveThreshold = value
	}
	if raw, ok := values[SettingKeyGatewayUpstreamErrorStatusCodes]; ok && strings.TrimSpace(raw) != "" {
		var codes []int
		if err := json.Unmarshal([]byte(raw), &codes); err == nil && validateRetryStatusCodes(codes) == nil {
			settings.UpstreamErrorStatusCodes = normalizeRetryStatusCodes(codes)
		}
	}
	if value, err := strconv.Atoi(strings.TrimSpace(values[SettingKeyGatewayUpstreamErrorConsecutiveThreshold])); err == nil && value >= 1 && value <= MaxGatewayFailureConsecutiveThreshold {
		settings.UpstreamErrorConsecutiveThreshold = value
	}
	if raw, ok := values[SettingKeyGatewayImageGroupSuccessRateVisible]; ok {
		settings.ImageGroupSuccessRateVisible = !isFalseSettingValue(raw)
	}
	if value, err := strconv.ParseInt(strings.TrimSpace(values[SettingKeyGatewayFailurePolicyRevision]), 10, 64); err == nil && value >= DefaultGatewayFailurePolicyRevision {
		settings.FailurePolicyRevision = value
	}
	return settings
}

func validateGatewaySettings(settings *GatewaySettings) error {
	if settings == nil || settings.DefaultPoolModeRetryCount < 0 || settings.DefaultPoolModeRetryCount > MaxGatewayPoolModeRetryCount {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "default_pool_mode_retry_count"})
	}
	if settings.FirstTokenTimeoutSeconds < 0 || settings.FirstTokenTimeoutSeconds > MaxGatewayFirstTokenTimeoutSeconds {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "first_token_timeout_seconds"})
	}
	if settings.FirstTokenTimeoutConsecutiveThreshold < 1 || settings.FirstTokenTimeoutConsecutiveThreshold > MaxGatewayFailureConsecutiveThreshold {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "first_token_timeout_consecutive_threshold"})
	}
	if err := validateRetryStatusCodes(settings.UpstreamErrorStatusCodes); err != nil {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "upstream_error_status_codes"})
	}
	if settings.UpstreamErrorConsecutiveThreshold < 1 || settings.UpstreamErrorConsecutiveThreshold > MaxGatewayFailureConsecutiveThreshold {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "upstream_error_consecutive_threshold"})
	}
	if err := validateRetryStatusCodes(settings.DefaultPoolModeRetryStatusCodes); err != nil {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "default_pool_mode_retry_status_codes"})
	}
	if err := validateProbeBackoffMinutes(settings.AutoManagedProbeBackoffMinutes); err != nil {
		return ErrGatewaySettingsInvalid.WithMetadata(map[string]string{"field": "auto_managed_probe_backoff_minutes"})
	}
	settings.DefaultPoolModeRetryStatusCodes = normalizeRetryStatusCodes(settings.DefaultPoolModeRetryStatusCodes)
	settings.UpstreamErrorStatusCodes = normalizeRetryStatusCodes(settings.UpstreamErrorStatusCodes)
	settings.AutoManagedProbeBackoffMinutes = append([]int(nil), settings.AutoManagedProbeBackoffMinutes...)
	return nil
}

func validateRetryStatusCodes(codes []int) error {
	for _, code := range codes {
		if code < 100 || code > 599 {
			return errors.New("invalid HTTP status code")
		}
	}
	return nil
}

func validateProbeBackoffMinutes(minutes []int) error {
	if len(minutes) < 1 || len(minutes) > 10 {
		return errors.New("invalid backoff step count")
	}
	for i, minute := range minutes {
		if minute < 1 || minute > 1440 || (i > 0 && minute < minutes[i-1]) {
			return errors.New("invalid backoff minutes")
		}
	}
	return nil
}

func normalizeRetryStatusCodes(codes []int) []int {
	if len(codes) == 0 {
		return []int{}
	}
	result := append([]int(nil), codes...)
	sort.Ints(result)
	write := 0
	for _, code := range result {
		if write > 0 && result[write-1] == code {
			continue
		}
		result[write] = code
		write++
	}
	return result[:write]
}

func cloneGatewaySettings(settings GatewaySettings) GatewaySettings {
	settings.DefaultPoolModeRetryStatusCodes = append([]int{}, settings.DefaultPoolModeRetryStatusCodes...)
	settings.UpstreamErrorStatusCodes = append([]int{}, settings.UpstreamErrorStatusCodes...)
	settings.AutoManagedProbeBackoffMinutes = append([]int(nil), settings.AutoManagedProbeBackoffMinutes...)
	return settings
}

func (s *SettingService) storeGatewaySettingsCache(settings GatewaySettings, ttl time.Duration) {
	if s == nil {
		return
	}
	s.gatewaySettingsCache.Store(&cachedGatewaySettings{
		settings:  cloneGatewaySettings(settings),
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
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
