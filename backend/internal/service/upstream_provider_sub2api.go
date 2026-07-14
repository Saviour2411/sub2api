package service

import (
	"context"
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
	if _, _, err = p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/v1/auth/me", headers, "", nil); err != nil {
		return nil, fmt.Errorf("验证 Sub2API 登录状态: %w", err)
	}
	return &updated, nil
}

func (p *sub2APIUpstreamProvider) Sync(ctx context.Context, req UpstreamSyncRequest) (*UpstreamSyncResult, error) {
	credential, headers, err := p.authenticate(ctx, req.Site, req.Credential)
	if err != nil {
		return nil, err
	}
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

	daily := make([]UpstreamDailySnapshot, 0, len(req.Dates))
	usageByGroup := make(map[string]UpstreamGroupSnapshot)
	todayKey := dayKey(time.Now(), req.Location)
	for _, date := range req.Dates {
		query := url.Values{}
		query.Set("start_date", date.In(req.Location).Format("2006-01-02"))
		query.Set("end_date", date.In(req.Location).Format("2006-01-02"))
		payload, _, requestErr := p.http.doJSON(ctx, http.MethodGet, req.Site.BaseURL, "/api/v1/usage/stats?"+query.Encode(), headers, "", nil)
		if requestErr != nil {
			return nil, fmt.Errorf("读取 Sub2API %s 用量: %w", date.Format("2006-01-02"), requestErr)
		}
		snapshot, groupUsage := parseSub2APIUsage(payload, date)
		if dayKey(date, req.Location) == todayKey {
			snapshot.BalanceUSD = balance
			usageByGroup = groupUsage
		}
		daily = append(daily, snapshot)
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

func (p *sub2APIUpstreamProvider) authenticate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (UpstreamCredential, map[string]string, error) {
	if site.AuthMode == UpstreamAuthPassword {
		payload, _, err := p.http.doJSON(ctx, http.MethodPost, site.BaseURL, "/api/v1/auth/login", nil, "", map[string]any{
			"email": site.Account, "username": site.Account, "password": credential.Password,
		})
		if err != nil {
			return credential, nil, fmt.Errorf("登录 Sub2API: %w", err)
		}
		credential.AccessToken = stringValue(valueByKeys(apiData(payload), "access_token", "token"))
		if refresh := stringValue(valueByKeys(apiData(payload), "refresh_token")); refresh != "" {
			credential.RefreshToken = refresh
		}
	}
	if credential.AccessToken == "" && credential.RefreshToken != "" {
		payload, _, err := p.http.doJSON(ctx, http.MethodPost, site.BaseURL, "/api/v1/auth/refresh", nil, "", map[string]string{"refresh_token": credential.RefreshToken})
		if err != nil {
			return credential, nil, fmt.Errorf("刷新 Sub2API 令牌: %w", err)
		}
		credential.AccessToken = stringValue(valueByKeys(apiData(payload), "access_token", "token"))
		if refresh := stringValue(valueByKeys(apiData(payload), "refresh_token")); refresh != "" {
			credential.RefreshToken = refresh
		}
	}
	if credential.AccessToken == "" {
		return credential, nil, fmt.Errorf("Sub2API 未返回访问令牌")
	}
	return credential, map[string]string{"Authorization": "Bearer " + credential.AccessToken}, nil
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
		multiplier := floatPointer(valueByKeys(object, "multiplier", "rate", "ratio"))
		if multiplier == nil {
			if value, ok := rates[remoteID]; ok {
				valueCopy := value
				multiplier = &valueCopy
			} else if value, ok := rates[name]; ok {
				valueCopy := value
				multiplier = &valueCopy
			}
		}
		groups = append(groups, UpstreamGroupSnapshot{
			RemoteID: remoteID, Name: name,
			Platform: stringValue(valueByKeys(object, "platform")), Multiplier: multiplier,
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
			if number, ok := numberValue(valueByKeys(nested, "multiplier", "rate", "ratio")); ok {
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

func parseSub2APIUsage(payload map[string]any, date time.Time) (UpstreamDailySnapshot, map[string]UpstreamGroupSnapshot) {
	data := apiData(payload)
	tokens, _ := int64Value(valueByKeys(data, "total_tokens", "tokens", "token_used"))
	if tokens == 0 {
		prompt, _ := int64Value(valueByKeys(data, "prompt_tokens", "input_tokens"))
		completion, _ := int64Value(valueByKeys(data, "completion_tokens", "output_tokens"))
		tokens = prompt + completion
	}
	cost, _ := numberValue(valueByKeys(data, "total_cost_usd", "total_cost", "cost", "amount"))
	snapshot := UpstreamDailySnapshot{Date: date, Tokens: tokens, CostUSD: cost}
	groups := make(map[string]UpstreamGroupSnapshot)
	for _, key := range []string{"groups", "by_group", "group_usage"} {
		raw := valueByKeys(data, key)
		if items := asSlice(raw); items != nil {
			for _, item := range items {
				addSub2APIGroupUsage(groups, asMap(item), "")
			}
		} else if object := asMap(raw); object != nil {
			for name, item := range object {
				addSub2APIGroupUsage(groups, asMap(item), name)
			}
		}
	}
	return snapshot, groups
}

func addSub2APIGroupUsage(target map[string]UpstreamGroupSnapshot, object map[string]any, fallback string) {
	if object == nil {
		return
	}
	id := stringValue(valueByKeys(object, "id", "group_id", "name", "group_name"))
	if id == "" {
		id = fallback
	}
	if id == "" {
		return
	}
	tokens, _ := int64Value(valueByKeys(object, "total_tokens", "tokens", "token_used"))
	cost, _ := numberValue(valueByKeys(object, "total_cost_usd", "total_cost", "cost", "amount"))
	target[id] = UpstreamGroupSnapshot{RemoteID: id, Name: strings.TrimSpace(fallback), TodayTokens: tokens, TodayCostUSD: cost}
}

func dayKey(value time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	return value.In(loc).Format("2006-01-02")
}
