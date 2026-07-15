package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type sub2APIUpstreamProvider struct {
	http *upstreamHTTPClient
}

func newSub2APIUpstreamProvider(client *upstreamHTTPClient) UpstreamProvider {
	return &sub2APIUpstreamProvider{http: client}
}

func (p *sub2APIUpstreamProvider) Platform() string { return UpstreamPlatformSub2API }

func (p *sub2APIUpstreamProvider) Validate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (*UpstreamCredential, error) {
	updated, headers, err := p.authenticate(ctx, site, credential)
	if err != nil {
		return nil, err
	}
	if _, _, err = p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/v1/auth/me", headers, "", nil); isUpstreamAuthenticationError(err) {
		updated, headers, err = p.reauthenticate(ctx, site, updated)
		if err == nil {
			_, _, err = p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/v1/auth/me", headers, "", nil)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("验证 Sub2API 登录状态: %w", err)
	}
	return &updated, nil
}

func (p *sub2APIUpstreamProvider) Sync(ctx context.Context, req UpstreamSyncRequest) (*UpstreamSyncResult, error) {
	credential, headers, err := p.authenticate(ctx, req.Site, req.Credential)
	if err != nil {
		return nil, err
	}
	result, err := p.syncAuthenticated(ctx, req, credential, headers)
	if err == nil {
		return result, nil
	}
	if !isUpstreamAuthenticationError(err) {
		return upstreamResultWithCredential(result, credential), err
	}

	credential, headers, authErr := p.reauthenticate(ctx, req.Site, credential)
	if authErr != nil {
		return upstreamResultWithCredential(result, credential), authErr
	}
	result, err = p.syncAuthenticated(ctx, req, credential, headers)
	if err != nil {
		return upstreamResultWithCredential(result, credential), err
	}
	return result, nil
}

func upstreamResultWithCredential(result *UpstreamSyncResult, credential UpstreamCredential) *UpstreamSyncResult {
	if result == nil {
		result = &UpstreamSyncResult{}
	}
	result.Credential = &credential
	return result
}

func (p *sub2APIUpstreamProvider) syncAuthenticated(
	ctx context.Context,
	req UpstreamSyncRequest,
	credential UpstreamCredential,
	headers map[string]string,
) (*UpstreamSyncResult, error) {
	me, _, err := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/auth/me", headers, "", nil)
	if err != nil {
		return nil, fmt.Errorf("读取 Sub2API 账号信息: %w", err)
	}
	balance := floatPointer(valueByKeys(apiData(me), "balance_usd", "balance", "remaining_balance"))

	groupsPayload, _, err := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/groups/available", headers, "", nil)
	if err != nil {
		return nil, fmt.Errorf("读取 Sub2API 可用分组: %w", err)
	}
	ratesPayload, _, err := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/groups/rates", headers, "", nil)
	if err != nil && !isHTTPStatus(err, http.StatusNotFound) {
		return nil, fmt.Errorf("读取 Sub2API 分组倍率: %w", err)
	}
	groups := parseSub2APIGroups(groupsPayload, ratesPayload)

	daily, usageByGroup, err := p.fetchUsage(ctx, req, headers, groups)
	if err != nil {
		return nil, err
	}
	todayKey := dayKey(time.Now(), req.Location)
	for index := range daily {
		if dayKey(daily[index].Date, req.Location) == todayKey {
			daily[index].BalanceUSD = balance
		}
	}
	for index := range groups {
		if usage, ok := usageByGroup[groups[index].RemoteID]; ok {
			groups[index].TodayTokens = usage.TodayTokens
			groups[index].TodayCostUSD = usage.TodayCostUSD
		} else if usage, ok := usageByGroup[groups[index].Name]; ok {
			groups[index].TodayTokens = usage.TodayTokens
			groups[index].TodayCostUSD = usage.TodayCostUSD
		}
	}
	return &UpstreamSyncResult{BalanceUSD: balance, Groups: groups, Daily: daily, Credential: &credential}, nil
}

func (p *sub2APIUpstreamProvider) fetchUsage(
	ctx context.Context,
	req UpstreamSyncRequest,
	headers map[string]string,
	groups []UpstreamGroupSnapshot,
) ([]UpstreamDailySnapshot, map[string]UpstreamGroupSnapshot, error) {
	daily, groupUsage, err := p.fetchSnapshotUsage(ctx, req, headers)
	if err == nil {
		return daily, groupUsage, nil
	}
	if !isHTTPStatus(err, http.StatusNotFound) {
		return nil, nil, fmt.Errorf("读取 Sub2API 用量快照: %w", err)
	}
	return p.fetchLegacyUsage(ctx, req, headers, groups)
}

func (p *sub2APIUpstreamProvider) fetchSnapshotUsage(
	ctx context.Context,
	req UpstreamSyncRequest,
	headers map[string]string,
) ([]UpstreamDailySnapshot, map[string]UpstreamGroupSnapshot, error) {
	if len(req.Dates) == 0 {
		return []UpstreamDailySnapshot{}, map[string]UpstreamGroupSnapshot{}, nil
	}
	loc := req.Location
	if loc == nil {
		loc = time.Local
	}
	first, last := req.Dates[0].In(loc), req.Dates[0].In(loc)
	for _, date := range req.Dates[1:] {
		localized := date.In(loc)
		if localized.Before(first) {
			first = localized
		}
		if localized.After(last) {
			last = localized
		}
	}

	query := sub2APISnapshotQuery(first, last, loc, true, false)
	payload, _, err := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/usage/dashboard/snapshot-v2?"+query.Encode(), headers, "", nil)
	if err != nil {
		return nil, nil, err
	}
	daily, err := parseSub2APISnapshotTrend(payload, req.Dates, loc)
	if err != nil {
		return nil, nil, err
	}

	groupUsage := make(map[string]UpstreamGroupSnapshot)
	today := time.Now().In(loc)
	if !containsDay(req.Dates, today, loc) {
		return daily, groupUsage, nil
	}
	query = sub2APISnapshotQuery(today, today, loc, false, true)
	payload, _, err = p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/usage/dashboard/snapshot-v2?"+query.Encode(), headers, "", nil)
	if err != nil {
		return nil, nil, err
	}
	groupUsage, err = parseSub2APISnapshotGroups(payload)
	if err != nil {
		return nil, nil, err
	}
	return daily, groupUsage, nil
}

func sub2APISnapshotQuery(first, last time.Time, loc *time.Location, includeTrend, includeGroups bool) url.Values {
	query := url.Values{}
	query.Set("start_date", first.In(loc).Format("2006-01-02"))
	query.Set("end_date", last.In(loc).Format("2006-01-02"))
	query.Set("granularity", "day")
	query.Set("include_trend", fmt.Sprintf("%t", includeTrend))
	query.Set("include_model_stats", "false")
	query.Set("include_group_stats", fmt.Sprintf("%t", includeGroups))
	query.Set("timezone", loc.String())
	return query
}

func (p *sub2APIUpstreamProvider) fetchLegacyUsage(
	ctx context.Context,
	req UpstreamSyncRequest,
	headers map[string]string,
	groups []UpstreamGroupSnapshot,
) ([]UpstreamDailySnapshot, map[string]UpstreamGroupSnapshot, error) {
	loc := req.Location
	if loc == nil {
		loc = time.Local
	}
	daily := make([]UpstreamDailySnapshot, 0, len(req.Dates))
	usageByGroup := make(map[string]UpstreamGroupSnapshot)
	todayKey := dayKey(time.Now(), loc)
	for _, date := range req.Dates {
		query := sub2APIStatsQuery(date, loc)
		payload, _, requestErr := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/usage/stats?"+query.Encode(), headers, "", nil)
		if requestErr != nil {
			return nil, nil, fmt.Errorf("读取 Sub2API %s 用量: %w", dayKey(date, loc), requestErr)
		}
		snapshot, parseErr := parseSub2APIUsage(payload, date)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("解析 Sub2API %s 用量: %w", dayKey(date, loc), parseErr)
		}
		daily = append(daily, snapshot)
		if dayKey(date, loc) != todayKey {
			continue
		}
		for _, group := range groups {
			groupQuery := sub2APIStatsQuery(date, loc)
			groupQuery.Set("group_id", group.RemoteID)
			groupPayload, _, groupErr := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/usage/stats?"+groupQuery.Encode(), headers, "", nil)
			if groupErr != nil {
				return nil, nil, fmt.Errorf("读取 Sub2API 分组 %s 当日用量: %w", group.Name, groupErr)
			}
			groupSnapshot, parseGroupErr := parseSub2APIUsage(groupPayload, date)
			if parseGroupErr != nil {
				return nil, nil, fmt.Errorf("解析 Sub2API 分组 %s 当日用量: %w", group.Name, parseGroupErr)
			}
			usageByGroup[group.RemoteID] = UpstreamGroupSnapshot{
				RemoteID: group.RemoteID, Name: group.Name,
				TodayTokens: groupSnapshot.Tokens, TodayCostUSD: groupSnapshot.CostUSD,
			}
		}
	}
	return daily, usageByGroup, nil
}

func sub2APIStatsQuery(date time.Time, loc *time.Location) url.Values {
	query := url.Values{}
	query.Set("start_date", date.In(loc).Format("2006-01-02"))
	query.Set("end_date", date.In(loc).Format("2006-01-02"))
	query.Set("timezone", loc.String())
	return query
}

func (p *sub2APIUpstreamProvider) authenticate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (UpstreamCredential, map[string]string, error) {
	if credential.AccessToken != "" {
		return credential, map[string]string{"Authorization": "Bearer " + credential.AccessToken}, nil
	}
	if credential.RefreshToken != "" {
		updated, headers, err := p.refresh(ctx, site, credential)
		if err == nil {
			return updated, headers, nil
		}
		if site.AuthMode != UpstreamAuthPassword || !canFallbackToPasswordLogin(err) {
			return credential, nil, err
		}
	}
	if site.AuthMode == UpstreamAuthPassword {
		return p.login(ctx, site, credential)
	}
	return credential, nil, fmt.Errorf("Sub2API 未返回访问令牌")
}

func (p *sub2APIUpstreamProvider) reauthenticate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (UpstreamCredential, map[string]string, error) {
	if credential.RefreshToken != "" {
		updated, headers, err := p.refresh(ctx, site, credential)
		if err == nil {
			return updated, headers, nil
		}
		if site.AuthMode != UpstreamAuthPassword || !canFallbackToPasswordLogin(err) {
			return credential, nil, err
		}
	}
	if site.AuthMode == UpstreamAuthPassword {
		return p.login(ctx, site, credential)
	}
	return credential, nil, fmt.Errorf("Sub2API 访问令牌已过期且没有刷新令牌")
}

func canFallbackToPasswordLogin(err error) bool {
	return isHTTPStatus(err, http.StatusBadRequest) ||
		isHTTPStatus(err, http.StatusUnauthorized) ||
		isHTTPStatus(err, http.StatusForbidden) ||
		isHTTPStatus(err, http.StatusNotFound) ||
		isHTTPStatus(err, http.StatusMethodNotAllowed)
}

func (p *sub2APIUpstreamProvider) login(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (UpstreamCredential, map[string]string, error) {
	payload, _, err := p.http.doJSON(ctx, http.MethodPost, site.BaseURL, "/api/v1/auth/login", nil, "", map[string]any{
		"email": site.Account, "username": site.Account, "password": credential.Password,
	})
	if err != nil {
		if isSub2APITurnstileError(err) {
			return credential, nil, ErrUpstreamTurnstileRequired.WithCause(err)
		}
		return credential, nil, fmt.Errorf("登录 Sub2API: %w", err)
	}
	credential.AccessToken = stringValue(valueByKeys(apiData(payload), "access_token", "token"))
	credential.RefreshToken = stringValue(valueByKeys(apiData(payload), "refresh_token"))
	if credential.AccessToken == "" {
		return credential, nil, fmt.Errorf("Sub2API 未返回访问令牌")
	}
	return credential, map[string]string{"Authorization": "Bearer " + credential.AccessToken}, nil
}

func isSub2APITurnstileError(err error) bool {
	var statusErr *upstreamHTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadRequest {
		return false
	}
	message := strings.ToLower(statusErr.Message)
	return strings.Contains(message, "turnstile") || strings.Contains(message, "turnstile_verification_failed")
}

func (p *sub2APIUpstreamProvider) refresh(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (UpstreamCredential, map[string]string, error) {
	payload, _, err := p.http.doJSON(ctx, http.MethodPost, site.BaseURL, "/api/v1/auth/refresh", nil, "", map[string]string{"refresh_token": credential.RefreshToken})
	if err != nil {
		return credential, nil, fmt.Errorf("刷新 Sub2API 令牌: %w", err)
	}
	credential.AccessToken = stringValue(valueByKeys(apiData(payload), "access_token", "token"))
	if refresh := stringValue(valueByKeys(apiData(payload), "refresh_token")); refresh != "" {
		credential.RefreshToken = refresh
	}
	if credential.AccessToken == "" {
		return credential, nil, fmt.Errorf("Sub2API 未返回访问令牌")
	}
	return credential, map[string]string{"Authorization": "Bearer " + credential.AccessToken}, nil
}

func isUpstreamAuthenticationError(err error) bool {
	return isHTTPStatus(err, http.StatusUnauthorized) || isHTTPStatus(err, http.StatusForbidden)
}

func parseSub2APIGroups(groupsPayload, ratesPayload map[string]any) []UpstreamGroupSnapshot {
	rates := collectNamedRates(apiData(ratesPayload))
	items := extractItems(groupsPayload)
	groups := make([]UpstreamGroupSnapshot, 0, len(items))
	for index, item := range items {
		object := asMap(item)
		if object == nil {
			continue
		}
		remoteID := stringValue(valueByKeys(object, "id", "group_id", "key"))
		name := stringValue(valueByKeys(object, "name", "group_name", "label"))
		if remoteID == "" {
			remoteID = name
		}
		if name == "" {
			name = remoteID
		}
		if remoteID == "" {
			remoteID = fmt.Sprintf("group-%d", index+1)
			name = remoteID
		}
		multiplier := floatPointer(valueByKeys(object, "rate_multiplier", "multiplier", "rate", "ratio"))
		if value, ok := rates[remoteID]; ok {
			valueCopy := value
			multiplier = &valueCopy
		} else if value, ok := rates[name]; ok {
			valueCopy := value
			multiplier = &valueCopy
		}
		groups = append(groups, UpstreamGroupSnapshot{
			RemoteID: remoteID, Name: name,
			Platform:    stringValue(valueByKeys(object, "platform")),
			Description: stringValue(valueByKeys(object, "description", "desc")), Multiplier: multiplier,
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func collectNamedRates(value any) map[string]float64 {
	result := make(map[string]float64)
	object := asMap(value)
	if object == nil {
		return result
	}
	for key, raw := range object {
		if number, ok := numberValue(raw); ok {
			result[key] = number
			continue
		}
		if nested := asMap(raw); nested != nil {
			if number, ok := numberValue(valueByKeys(nested, "rate_multiplier", "multiplier", "rate", "ratio")); ok {
				result[key] = number
			}
		}
	}
	for _, nestedKey := range []string{"data", "rates", "groups", "items"} {
		for key, value := range collectNamedRates(object[nestedKey]) {
			result[key] = value
		}
	}
	return result
}

func parseSub2APIUsage(payload map[string]any, date time.Time) (UpstreamDailySnapshot, error) {
	data := apiData(payload)
	tokens, _ := int64Value(valueByKeys(data, "total_tokens", "tokens", "token_used"))
	if tokens == 0 {
		prompt, _ := int64Value(valueByKeys(data, "prompt_tokens", "input_tokens"))
		completion, _ := int64Value(valueByKeys(data, "completion_tokens", "output_tokens"))
		tokens = prompt + completion
	}
	cost, ok := numberValue(valueByKeys(data, "total_actual_cost", "actual_cost"))
	if !ok {
		return UpstreamDailySnapshot{}, fmt.Errorf("响应缺少实际扣费字段 total_actual_cost/actual_cost")
	}
	return UpstreamDailySnapshot{Date: date, Tokens: tokens, CostUSD: cost}, nil
}

func parseSub2APISnapshotTrend(payload map[string]any, dates []time.Time, loc *time.Location) ([]UpstreamDailySnapshot, error) {
	data := asMap(apiData(payload))
	if data == nil {
		return nil, fmt.Errorf("快照响应 data 无效")
	}
	raw, exists := data["trend"]
	if !exists {
		return nil, fmt.Errorf("快照响应缺少 trend")
	}
	items := asSlice(raw)
	if raw != nil && items == nil {
		return nil, fmt.Errorf("快照响应 trend 无效")
	}
	byDate := make(map[string]UpstreamDailySnapshot, len(items))
	for _, item := range items {
		object := asMap(item)
		if object == nil {
			return nil, fmt.Errorf("快照趋势条目无效")
		}
		key := normalizedSnapshotDate(stringValue(valueByKeys(object, "date", "bucket_date")))
		if key == "" {
			return nil, fmt.Errorf("快照趋势条目缺少日期")
		}
		cost, ok := numberValue(valueByKeys(object, "actual_cost", "total_actual_cost"))
		if !ok {
			return nil, fmt.Errorf("快照趋势 %s 缺少实际扣费字段 actual_cost", key)
		}
		tokens, _ := int64Value(valueByKeys(object, "total_tokens", "tokens"))
		byDate[key] = UpstreamDailySnapshot{Tokens: tokens, CostUSD: cost}
	}
	result := make([]UpstreamDailySnapshot, 0, len(dates))
	for _, date := range dates {
		key := dayKey(date, loc)
		// snapshot-v2 只聚合存在日志的日期；请求成功但缺少日期代表当日零用量。
		snapshot := byDate[key]
		snapshot.Date = date
		result = append(result, snapshot)
	}
	return result, nil
}

func parseSub2APISnapshotGroups(payload map[string]any) (map[string]UpstreamGroupSnapshot, error) {
	data := asMap(apiData(payload))
	if data == nil {
		return nil, fmt.Errorf("快照响应 data 无效")
	}
	raw, exists := data["groups"]
	if !exists {
		return nil, fmt.Errorf("快照响应缺少 groups")
	}
	result := make(map[string]UpstreamGroupSnapshot)
	if items := asSlice(raw); items != nil {
		for _, item := range items {
			if err := addSub2APIGroupUsage(result, asMap(item), ""); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	if object := asMap(raw); object != nil {
		for name, item := range object {
			if err := addSub2APIGroupUsage(result, asMap(item), name); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	if raw == nil {
		return result, nil
	}
	return nil, fmt.Errorf("快照响应 groups 无效")
}

func addSub2APIGroupUsage(target map[string]UpstreamGroupSnapshot, object map[string]any, fallback string) error {
	if object == nil {
		return fmt.Errorf("快照分组条目无效")
	}
	id := stringValue(valueByKeys(object, "id", "group_id", "name", "group_name"))
	if id == "" {
		id = fallback
	}
	if id == "" {
		return fmt.Errorf("快照分组条目缺少分组标识")
	}
	tokens, _ := int64Value(valueByKeys(object, "total_tokens", "tokens", "token_used"))
	cost, ok := numberValue(valueByKeys(object, "actual_cost", "total_actual_cost"))
	if !ok {
		return fmt.Errorf("快照分组 %s 缺少实际扣费字段 actual_cost", id)
	}
	target[id] = UpstreamGroupSnapshot{RemoteID: id, Name: strings.TrimSpace(fallback), TodayTokens: tokens, TodayCostUSD: cost}
	return nil
}

func normalizedSnapshotDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < len("2006-01-02") {
		return ""
	}
	candidate := value[:len("2006-01-02")]
	if _, err := time.Parse("2006-01-02", candidate); err != nil {
		return ""
	}
	return candidate
}

func containsDay(dates []time.Time, target time.Time, loc *time.Location) bool {
	targetKey := dayKey(target, loc)
	for _, date := range dates {
		if dayKey(date, loc) == targetKey {
			return true
		}
	}
	return false
}

func dayKey(value time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	return value.In(loc).Format("2006-01-02")
}
