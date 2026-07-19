package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const upstreamSyncInterval = 5 * time.Minute

const maxUpstreamGroupBindings = 1000

// UpstreamService 提供独立上游管理的 CRUD、脱敏和同步编排。
type UpstreamService struct {
	repo       UpstreamRepository
	encryptor  SecretEncryptor
	http       *upstreamHTTPClient
	providers  map[string]UpstreamProvider
	location   *time.Location
	scheduler  UpstreamSyncScheduler
	schedulerM sync.RWMutex
}

func NewUpstreamService(repo UpstreamRepository, encryptor SecretEncryptor, cfg *config.Config) (*UpstreamService, error) {
	httpClient, err := newUpstreamHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	svc := &UpstreamService{
		repo: repo, encryptor: encryptor, http: httpClient, location: loc,
		providers: make(map[string]UpstreamProvider, 2),
	}
	for _, provider := range []UpstreamProvider{
		newSub2APIUpstreamProvider(httpClient),
		newNewAPIUpstreamProvider(httpClient),
	} {
		svc.providers[provider.Platform()] = provider
	}
	return svc, nil
}

func (s *UpstreamService) SetScheduler(scheduler UpstreamSyncScheduler) {
	s.schedulerM.Lock()
	s.scheduler = scheduler
	s.schedulerM.Unlock()
}

func (s *UpstreamService) List(ctx context.Context, params UpstreamListParams) ([]UpstreamSiteView, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}
	if err := validateUpstreamListParams(&params); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	items := make([]UpstreamSiteView, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.toView(row))
	}
	return items, total, nil
}

func (s *UpstreamService) ListAll(ctx context.Context, params UpstreamListParams) ([]UpstreamSiteView, error) {
	if err := validateUpstreamListParams(&params); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListAll(ctx, params)
	if err != nil {
		return nil, err
	}
	items := make([]UpstreamSiteView, 0, len(rows))
	for _, row := range rows {
		items = append(items, s.toView(row))
	}
	return items, nil
}

func (s *UpstreamService) UpdateSortOrder(ctx context.Context, updates []UpstreamSortOrderUpdate) error {
	if len(updates) == 0 || len(updates) > 1000 {
		return ErrUpstreamInvalidInput
	}
	seen := make(map[int64]struct{}, len(updates))
	for _, update := range updates {
		if update.ID <= 0 || update.SortOrder < 0 {
			return ErrUpstreamInvalidInput
		}
		if _, exists := seen[update.ID]; exists {
			return ErrUpstreamInvalidInput
		}
		seen[update.ID] = struct{}{}
	}
	return s.repo.UpdateSortOrder(ctx, updates)
}

func (s *UpstreamService) Get(ctx context.Context, id int64) (*UpstreamSiteView, error) {
	site, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view := s.toView(site)
	return &view, nil
}

func (s *UpstreamService) ProbeCapabilities(ctx context.Context, input UpstreamProbeInput) (*UpstreamCapabilities, error) {
	platform := strings.ToLower(strings.TrimSpace(input.Platform))
	if platform != UpstreamPlatformSub2API && platform != UpstreamPlatformNewAPI {
		return nil, ErrUpstreamInvalidInput.WithCause(fmt.Errorf("platform 仅支持 sub2api 或 newapi"))
	}
	baseURL, err := s.http.normalizeBaseURL(strings.TrimSpace(input.BaseURL))
	if err != nil {
		return nil, ErrUpstreamInvalidInput.WithCause(err)
	}
	capabilities := &UpstreamCapabilities{BaseURL: baseURL, Platform: platform}
	if platform != UpstreamPlatformSub2API {
		return capabilities, nil
	}
	payload, _, err := s.http.doJSON(ctx, "GET", baseURL, "/api/v1/settings/public", nil, "", nil)
	if err != nil {
		return nil, ErrUpstreamConnectionFailed.WithCause(fmt.Errorf("读取 Sub2API 公开设置: %w", err))
	}
	settings := asMap(apiData(payload))
	if settings == nil {
		return nil, ErrUpstreamConnectionFailed.WithCause(errors.New("Sub2API 公开设置响应无效"))
	}
	turnstileEnabled, _ := settings["turnstile_enabled"].(bool)
	capabilities.TurnstileEnabled = turnstileEnabled
	capabilities.TokenAuthRecommended = turnstileEnabled
	return capabilities, nil
}

func (s *UpstreamService) Create(ctx context.Context, input UpstreamCreateInput) (*UpstreamSiteView, error) {
	site, credential, err := s.prepareCreate(input)
	if err != nil {
		return nil, err
	}
	provider := s.providers[site.Platform]
	updatedCredential, err := provider.Validate(ctx, site, credential)
	if err != nil {
		return nil, upstreamValidationError(err)
	}
	if updatedCredential != nil {
		credential = *updatedCredential
	}
	encrypted, err := s.encryptCredential(credential)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	site.CredentialEncrypted = encrypted
	site.Status = UpstreamStatusPending
	site.TrackingStartedAt = now
	site.CreatedBy = input.CreatedBy
	if site.Enabled {
		site.NextSyncAt = &now
	}
	if err := s.repo.Create(ctx, site); err != nil {
		return nil, err
	}
	s.enqueue(site.ID)
	view := s.toView(site)
	return &view, nil
}

func (s *UpstreamService) Update(ctx context.Context, id int64, input UpstreamUpdateInput) (*UpstreamSiteView, error) {
	site, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	credential, err := s.decryptCredential(site.CredentialEncrypted)
	if err != nil {
		return nil, ErrUpstreamCredentialDecrypt.WithCause(err)
	}
	mergeUpstreamUpdate(site, &credential, input)
	if err := s.validateSite(site, credential); err != nil {
		return nil, err
	}
	normalizedURL, err := s.http.normalizeBaseURL(site.BaseURL)
	if err != nil {
		return nil, ErrUpstreamInvalidInput.WithCause(err)
	}
	site.BaseURL = normalizedURL
	updatedCredential, err := s.providers[site.Platform].Validate(ctx, site, credential)
	if err != nil {
		return nil, upstreamValidationError(err)
	}
	if updatedCredential != nil {
		credential = *updatedCredential
	}
	site.CredentialEncrypted, err = s.encryptCredential(credential)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	site.Status = UpstreamStatusPending
	if site.Enabled {
		site.NextSyncAt = &now
	} else {
		site.NextSyncAt = nil
	}
	if err := s.repo.Update(ctx, site); err != nil {
		return nil, err
	}
	// 配置更新完成后执行一次同步；禁用只影响后续自动调度，不阻止本次刷新。
	s.enqueue(site.ID)
	view := s.toView(site)
	return &view, nil
}

func upstreamValidationError(err error) error {
	if errors.Is(err, ErrUpstreamTurnstileRequired) {
		return ErrUpstreamTurnstileRequired.WithCause(err)
	}
	return ErrUpstreamConnectionFailed.WithCause(err)
}

func (s *UpstreamService) SetEnabled(ctx context.Context, id int64, enabled bool) (*UpstreamSiteView, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	var next *time.Time
	if enabled {
		now := time.Now()
		next = &now
	}
	if err := s.repo.SetEnabled(ctx, id, enabled, next); err != nil {
		return nil, err
	}
	if enabled {
		if err := s.repo.MarkPending(ctx, id, next); err != nil {
			return nil, err
		}
		s.enqueue(id)
	}
	site, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	view := s.toView(site)
	return &view, nil
}

func (s *UpstreamService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// QueueSync 手动同步不检查 enabled，因此禁用站点也可执行。
func (s *UpstreamService) QueueSync(ctx context.Context, id int64) error {
	site, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	var next *time.Time
	if site.Enabled {
		now := time.Now()
		next = &now
	}
	if err := s.repo.MarkPending(ctx, id, next); err != nil {
		return err
	}
	s.enqueue(id)
	return nil
}

func (s *UpstreamService) QueueAll(ctx context.Context) (int, error) {
	ids, err := s.repo.ListIDs(ctx, false)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.QueueSync(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (s *UpstreamService) ListGroups(ctx context.Context, id int64) ([]UpstreamGroup, error) {
	groups, err := s.repo.ListGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		normalizeUpstreamGroupBindings(&groups[index])
	}
	return groups, nil
}

func (s *UpstreamService) SetGroupDisplayed(ctx context.Context, id int64, input UpstreamGroupDisplayInput) (*UpstreamGroupDisplayResult, error) {
	remoteID := strings.TrimSpace(input.RemoteID)
	if id <= 0 || remoteID == "" || len(remoteID) > 100 || input.Displayed == nil {
		return nil, ErrUpstreamInvalidInput
	}
	return s.repo.SetGroupDisplayed(ctx, id, remoteID, *input.Displayed)
}

// ReplaceGroupBindings 原子替换指定上游分组的全部账号绑定；空数组表示全部解绑。
func (s *UpstreamService) ReplaceGroupBindings(
	ctx context.Context,
	siteID, upstreamGroupID int64,
	inputs []UpstreamGroupAccountBindingInput,
) (*UpstreamGroup, error) {
	if siteID <= 0 || upstreamGroupID <= 0 || len(inputs) > maxUpstreamGroupBindings {
		return nil, ErrUpstreamInvalidInput
	}
	seenAccounts := make(map[int64]struct{}, len(inputs))
	for _, input := range inputs {
		if input.LocalGroupID <= 0 || input.AccountID <= 0 {
			return nil, ErrUpstreamInvalidInput
		}
		if _, exists := seenAccounts[input.AccountID]; exists {
			return nil, ErrUpstreamInvalidInput.WithCause(fmt.Errorf("请求内存在重复账号 ID: %d", input.AccountID))
		}
		seenAccounts[input.AccountID] = struct{}{}
	}
	group, err := s.repo.ReplaceGroupBindings(ctx, siteID, upstreamGroupID, inputs)
	if err != nil {
		return nil, err
	}
	normalizeUpstreamGroupBindings(group)
	return group, nil
}

func normalizeUpstreamGroupBindings(group *UpstreamGroup) {
	if group != nil && group.Bindings == nil {
		group.Bindings = make([]UpstreamGroupAccountBinding, 0)
	}
}

func (s *UpstreamService) ListHistory(ctx context.Context, id int64, from, through time.Time) ([]UpstreamDailyStat, error) {
	from = dayStartInLocation(from, s.location)
	through = dayStartInLocation(through, s.location)
	if through.Before(from) || through.After(from.AddDate(0, 0, 365)) {
		return nil, ErrUpstreamInvalidInput.WithCause(fmt.Errorf("历史日期范围必须在 1 到 366 天内"))
	}
	return s.repo.ListHistory(ctx, id, from, through)
}

func (s *UpstreamService) ListMultiplierHistory(ctx context.Context, id int64, from, through time.Time) ([]UpstreamGroupMultiplierHistory, error) {
	from = dayStartInLocation(from, s.location)
	through = dayStartInLocation(through, s.location)
	if through.Before(from) || through.After(from.AddDate(0, 0, 365)) {
		return nil, ErrUpstreamInvalidInput.WithCause(fmt.Errorf("历史日期范围必须在 1 到 366 天内"))
	}
	through = through.AddDate(0, 0, 1).Add(-time.Nanosecond)
	return s.repo.ListMultiplierHistory(ctx, id, from, through)
}

func (s *UpstreamService) ListDue(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	return s.repo.ListDue(ctx, now, limit)
}

// RunSync 在内存获得完整快照后才调用 Repository 的单事务提交。
func (s *UpstreamService) RunSync(ctx context.Context, id int64) error {
	site, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	credential, err := s.decryptCredential(site.CredentialEncrypted)
	if err != nil {
		_ = s.repo.MarkSyncFailed(context.Background(), id, "凭证解密失败", nextSyncTime(site.Enabled))
		return ErrUpstreamCredentialDecrypt.WithCause(err)
	}
	provider := s.providers[site.Platform]
	if provider == nil {
		return ErrUpstreamInvalidInput
	}
	if err := s.repo.MarkSyncing(ctx, id); err != nil {
		return err
	}
	now := time.Now()
	dates, err := s.repo.MissingDates(ctx, id, site.TrackingStartedAt, now, s.location)
	if err != nil {
		s.markSyncFailed(id, site.Enabled, err)
		return err
	}
	result, err := provider.Sync(ctx, UpstreamSyncRequest{Site: site, Credential: credential, Dates: dates, Location: s.location})
	if err != nil {
		if credentialErr := s.persistFailedSyncCredential(id, result); credentialErr != nil {
			err = fmt.Errorf("%w；保存刷新后的上游凭证失败: %v", err, credentialErr)
		}
		s.markSyncFailed(id, site.Enabled, err)
		return err
	}
	encrypted := ""
	if result.Credential != nil {
		encrypted, err = s.encryptCredential(*result.Credential)
		if err != nil {
			s.markSyncFailed(id, site.Enabled, err)
			return err
		}
	}
	if err := s.repo.CommitSync(ctx, id, result, encrypted, now, nextSyncTime(site.Enabled)); err != nil {
		s.markSyncFailed(id, site.Enabled, err)
		return err
	}
	return nil
}

func (s *UpstreamService) persistFailedSyncCredential(id int64, result *UpstreamSyncResult) error {
	if result == nil || result.Credential == nil {
		return nil
	}
	encrypted, err := s.encryptCredential(*result.Credential)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.repo.UpdateCredential(ctx, id, encrypted)
}

func (s *UpstreamService) prepareCreate(input UpstreamCreateInput) (*UpstreamSite, UpstreamCredential, error) {
	site := &UpstreamSite{
		Name: strings.TrimSpace(input.Name), BaseURL: strings.TrimSpace(input.BaseURL),
		Platform: strings.ToLower(strings.TrimSpace(input.Platform)), AuthMode: strings.ToLower(strings.TrimSpace(input.AuthMode)),
		Account: strings.TrimSpace(input.Account), Enabled: input.Enabled,
	}
	credential := UpstreamCredential{
		Password: strings.TrimSpace(input.Password), AccessToken: strings.TrimSpace(input.AccessToken),
		RefreshToken: strings.TrimSpace(input.RefreshToken), UserAgent: strings.TrimSpace(input.UserAgent),
	}
	if err := s.validateSite(site, credential); err != nil {
		return nil, UpstreamCredential{}, err
	}
	normalized, err := s.http.normalizeBaseURL(site.BaseURL)
	if err != nil {
		return nil, UpstreamCredential{}, ErrUpstreamInvalidInput.WithCause(err)
	}
	site.BaseURL = normalized
	return site, credential, nil
}

func (s *UpstreamService) validateSite(site *UpstreamSite, credential UpstreamCredential) error {
	if site == nil || site.Name == "" || len(site.Name) > 100 || site.BaseURL == "" {
		return ErrUpstreamInvalidInput
	}
	if site.Platform != UpstreamPlatformSub2API && site.Platform != UpstreamPlatformNewAPI {
		return ErrUpstreamInvalidInput.WithCause(fmt.Errorf("platform 仅支持 sub2api 或 newapi"))
	}
	if site.AuthMode != UpstreamAuthPassword && site.AuthMode != UpstreamAuthToken {
		return ErrUpstreamInvalidInput.WithCause(fmt.Errorf("auth_mode 仅支持 password 或 token"))
	}
	if site.Platform == UpstreamPlatformNewAPI && site.AuthMode != UpstreamAuthPassword {
		return ErrUpstreamInvalidInput.WithCause(fmt.Errorf("new API 仅支持密码认证"))
	}
	if site.AuthMode == UpstreamAuthPassword {
		if site.Account == "" || credential.Password == "" {
			return ErrUpstreamInvalidInput.WithCause(fmt.Errorf("密码认证必须填写账号和密码"))
		}
	} else if credential.AccessToken == "" && credential.RefreshToken == "" {
		return ErrUpstreamInvalidInput.WithCause(fmt.Errorf("令牌认证必须填写访问令牌或刷新令牌"))
	} else if len(credential.UserAgent) > 512 {
		return ErrUpstreamInvalidInput.WithCause(fmt.Errorf("会话 User-Agent 不能超过 512 个字符"))
	}
	return nil
}

func mergeUpstreamUpdate(site *UpstreamSite, credential *UpstreamCredential, input UpstreamUpdateInput) bool {
	changed := false
	previousBaseURL := site.BaseURL
	previousPlatform := site.Platform
	previousAuthMode := site.AuthMode
	previousAccount := site.Account
	apply := func(target *string, value *string, normalize func(string) string) {
		if value == nil {
			return
		}
		next := normalize(*value)
		if *target != next {
			*target = next
			changed = true
		}
	}
	trim := strings.TrimSpace
	lower := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	apply(&site.Name, input.Name, trim)
	apply(&site.BaseURL, input.BaseURL, trim)
	apply(&site.Platform, input.Platform, lower)
	apply(&site.AuthMode, input.AuthMode, lower)
	apply(&site.Account, input.Account, trim)
	if previousBaseURL != site.BaseURL || previousPlatform != site.Platform || previousAccount != site.Account {
		clearUpstreamSessionCredential(credential)
	}
	if previousAuthMode != site.AuthMode {
		if site.AuthMode == UpstreamAuthToken {
			credential.Password = ""
			credential.Cookie = ""
			credential.NewAPIUserID = ""
		} else {
			clearUpstreamSessionCredential(credential)
		}
	}
	if input.Enabled != nil {
		site.Enabled = *input.Enabled
	}
	if input.Password != nil && strings.TrimSpace(*input.Password) != "" {
		clearUpstreamSessionCredential(credential)
		credential.Password = strings.TrimSpace(*input.Password)
		changed = true
	}
	if input.AccessToken != nil && strings.TrimSpace(*input.AccessToken) != "" {
		credential.AccessToken = strings.TrimSpace(*input.AccessToken)
		changed = true
	}
	if input.RefreshToken != nil && strings.TrimSpace(*input.RefreshToken) != "" {
		credential.RefreshToken = strings.TrimSpace(*input.RefreshToken)
		changed = true
	}
	if input.UserAgent != nil && strings.TrimSpace(*input.UserAgent) != "" {
		credential.UserAgent = strings.TrimSpace(*input.UserAgent)
		changed = true
	}
	return changed
}

func clearUpstreamSessionCredential(credential *UpstreamCredential) {
	credential.AccessToken = ""
	credential.RefreshToken = ""
	credential.UserAgent = ""
	credential.Cookie = ""
	credential.NewAPIUserID = ""
}

func (s *UpstreamService) encryptCredential(credential UpstreamCredential) (string, error) {
	raw, err := json.Marshal(credential)
	if err != nil {
		return "", fmt.Errorf("编码上游凭证: %w", err)
	}
	encrypted, err := s.encryptor.Encrypt(string(raw))
	if err != nil {
		return "", fmt.Errorf("加密上游凭证: %w", err)
	}
	return encrypted, nil
}

func (s *UpstreamService) decryptCredential(encrypted string) (UpstreamCredential, error) {
	raw, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return UpstreamCredential{}, err
	}
	var credential UpstreamCredential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return UpstreamCredential{}, err
	}
	return credential, nil
}

func (s *UpstreamService) toView(site *UpstreamSite) UpstreamSiteView {
	view := UpstreamSiteView{
		ID: site.ID, SortOrder: site.SortOrder, Name: site.Name, BaseURL: site.BaseURL, Platform: site.Platform, AuthMode: site.AuthMode,
		Account: site.Account, Enabled: site.Enabled, Status: site.Status, ErrorMessage: site.ErrorMessage,
		BalanceUSD: site.BalanceUSD, TodayTokens: site.TodayTokens, TodayCostUSD: site.TodayCostUSD,
		TotalTokens: site.TotalTokens, TotalCostUSD: site.TotalCostUSD, TrackingStartedAt: site.TrackingStartedAt,
		LastSyncedAt: site.LastSyncedAt, CreatedAt: site.CreatedAt, UpdatedAt: site.UpdatedAt,
		DisplayedGroupCount: site.DisplayedGroupCount,
		BindingCount:        site.BindingCount,
	}
	credential, err := s.decryptCredential(site.CredentialEncrypted)
	if err == nil {
		view.HasPassword = site.AuthMode == UpstreamAuthPassword && credential.Password != ""
		view.HasToken = site.AuthMode == UpstreamAuthToken && (credential.AccessToken != "" || credential.RefreshToken != "")
	}
	return view
}

func validateUpstreamListParams(params *UpstreamListParams) error {
	if params == nil {
		return ErrUpstreamInvalidInput
	}
	params.GroupPlatform = strings.TrimSpace(params.GroupPlatform)
	if len(params.GroupPlatform) > 50 {
		return ErrUpstreamInvalidInput
	}
	params.Platform = strings.ToLower(strings.TrimSpace(params.Platform))
	if params.Platform != "" && params.Platform != UpstreamPlatformSub2API && params.Platform != UpstreamPlatformNewAPI {
		return ErrUpstreamInvalidInput
	}
	params.SortBy = strings.ToLower(strings.TrimSpace(params.SortBy))
	if params.SortBy != "" && params.SortBy != "balance_usd" && params.SortBy != "today_tokens" {
		return ErrUpstreamInvalidInput
	}
	params.SortOrder = strings.ToLower(strings.TrimSpace(params.SortOrder))
	if params.SortOrder == "" {
		params.SortOrder = "asc"
	}
	if params.SortOrder != "asc" && params.SortOrder != "desc" {
		return ErrUpstreamInvalidInput
	}
	return nil
}

func (s *UpstreamService) enqueue(id int64) {
	s.schedulerM.RLock()
	scheduler := s.scheduler
	s.schedulerM.RUnlock()
	if scheduler != nil {
		scheduler.Enqueue(id)
	}
}

func (s *UpstreamService) markSyncFailed(id int64, enabled bool, cause error) {
	message := "同步失败"
	if cause != nil {
		message = cause.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.repo.MarkSyncFailed(ctx, id, message, nextSyncTime(enabled))
}

func nextSyncTime(enabled bool) *time.Time {
	if !enabled {
		return nil
	}
	next := time.Now().Add(upstreamSyncInterval)
	return &next
}

func dayStartInLocation(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc)
}
