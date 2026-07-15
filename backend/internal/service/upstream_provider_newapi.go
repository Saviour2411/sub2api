package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	newAPILogPageSize          = 100
	newAPIUpstreamPlatformName = "New API"
)

type newAPIUpstreamProvider struct {
	http *upstreamHTTPClient
}

type newAPIAuthState struct {
	credential   UpstreamCredential
	headers      map[string]string
	self         map[string]any
	quotaPerUnit float64
}

func newNewAPIUpstreamProvider(client *upstreamHTTPClient) UpstreamProvider {
	return &newAPIUpstreamProvider{http: client}
}

func (p *newAPIUpstreamProvider) Platform() string { return UpstreamPlatformNewAPI }

func (p *newAPIUpstreamProvider) Validate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (*UpstreamCredential, error) {
	state, err := p.authenticate(ctx, site, credential)
	if err != nil {
		return nil, err
	}
	return &state.credential, nil
}

func (p *newAPIUpstreamProvider) Sync(ctx context.Context, req UpstreamSyncRequest) (*UpstreamSyncResult, error) {
	state, err := p.authenticate(ctx, req.Site, req.Credential)
	if err != nil {
		return nil, err
	}
	result, err := p.syncAuthenticated(ctx, req, state)
	if !isUpstreamAuthenticationError(err) {
		return result, err
	}

	state, authErr := p.authenticate(ctx, req.Site, state.credential)
	if authErr != nil {
		return nil, fmt.Errorf("重新登录 New API: %w", authErr)
	}
	return p.syncAuthenticated(ctx, req, state)
}

func (p *newAPIUpstreamProvider) syncAuthenticated(ctx context.Context, req UpstreamSyncRequest, state *newAPIAuthState) (*UpstreamSyncResult, error) {
	quota, ok := numberValue(valueByKeys(apiData(state.self), "quota", "balance"))
	var balance *float64
	if ok {
		value := quota / state.quotaPerUnit
		balance = &value
	}

	groups, err := p.fetchGroups(ctx, req.Site, state)
	if err != nil {
		return nil, err
	}
	daily := make([]UpstreamDailySnapshot, 0, len(req.Dates))
	todayUsage := make(map[string]UpstreamGroupSnapshot)
	todayKey := dayKey(time.Now(), req.Location)
	for _, date := range req.Dates {
		snapshot, groupUsage, fetchErr := p.fetchDailyLogs(ctx, req.Site, state, date, req.Location)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if dayKey(date, req.Location) == todayKey {
			snapshot.BalanceUSD = balance
			todayUsage = groupUsage
		}
		daily = append(daily, snapshot)
	}
	for index := range groups {
		if usage, found := todayUsage[groups[index].RemoteID]; found {
			groups[index].TodayTokens = usage.TodayTokens
			groups[index].TodayCostUSD = usage.TodayCostUSD
		} else if usage, found := todayUsage[groups[index].Name]; found {
			groups[index].TodayTokens = usage.TodayTokens
			groups[index].TodayCostUSD = usage.TodayCostUSD
		}
	}
	return &UpstreamSyncResult{BalanceUSD: balance, Groups: groups, Daily: daily, Credential: &state.credential}, nil
}

func (p *newAPIUpstreamProvider) authenticate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (*newAPIAuthState, error) {
	login, cookie, err := p.http.doJSON(ctx, http.MethodPost, site.BaseURL, "/api/user/login", nil, "", map[string]string{
		"username": site.Account,
		"password": credential.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("登录 New API: %w", err)
	}
	if cookie == "" {
		cookie = credential.Cookie
	}
	if cookie == "" {
		return nil, fmt.Errorf("new API 登录未返回 Cookie")
	}
	credential.Cookie = cookie
	userID := stringValue(valueByKeys(apiData(login), "id", "user_id"))
	headers := map[string]string{}
	if userID != "" {
		headers["New-Api-User"] = userID
	}
	self, _, err := p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/user/self", headers, cookie, nil)
	if err != nil {
		return nil, fmt.Errorf("读取 New API 账号信息: %w", err)
	}
	status, _, err := p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/status", nil, "", nil)
	if err != nil {
		return nil, fmt.Errorf("读取 New API 计价单位: %w", err)
	}
	quotaPerUnit, ok := numberValue(valueByKeys(apiData(status), "quota_per_unit"))
	if !ok || quotaPerUnit <= 0 {
		return nil, fmt.Errorf("new API quota_per_unit 无效")
	}
	return &newAPIAuthState{credential: credential, headers: headers, self: self, quotaPerUnit: quotaPerUnit}, nil
}

func (p *newAPIUpstreamProvider) fetchGroups(ctx context.Context, site *UpstreamSite, state *newAPIAuthState) ([]UpstreamGroupSnapshot, error) {
	payload, _, err := p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/user/self/groups", state.headers, state.credential.Cookie, nil)
	if err != nil && isHTTPStatus(err, http.StatusNotFound) {
		payload, _, err = p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/user/groups", state.headers, state.credential.Cookie, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("读取 New API 可用分组: %w", err)
	}
	groups := parseNewAPIGroups(apiData(payload))
	pricing, _, pricingErr := p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/pricing", state.headers, state.credential.Cookie, nil)
	if pricingErr != nil && !isHTTPStatus(pricingErr, http.StatusNotFound) {
		return nil, fmt.Errorf("读取 New API 分组定价: %w", pricingErr)
	}
	pricingRates := collectNewAPIPricingRates(apiData(pricing))
	for index := range groups {
		if rate, ok := pricingRates[groups[index].RemoteID]; ok {
			rateCopy := rate
			groups[index].Multiplier = &rateCopy
		} else if rate, ok := pricingRates[groups[index].Name]; ok {
			rateCopy := rate
			groups[index].Multiplier = &rateCopy
		}
	}
	return groups, nil
}

func (p *newAPIUpstreamProvider) fetchDailyLogs(ctx context.Context, site *UpstreamSite, state *newAPIAuthState, date time.Time, loc *time.Location) (UpstreamDailySnapshot, map[string]UpstreamGroupSnapshot, error) {
	if loc == nil {
		loc = time.Local
	}
	start := time.Date(date.In(loc).Year(), date.In(loc).Month(), date.In(loc).Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	var tokens int64
	var quota float64
	groups := make(map[string]UpstreamGroupSnapshot)
	for page := 1; page <= 10000; page++ {
		query := url.Values{}
		query.Set("p", strconv.Itoa(page))
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(newAPILogPageSize))
		query.Set("type", "0")
		query.Set("start_timestamp", strconv.FormatInt(start.Unix(), 10))
		query.Set("end_timestamp", strconv.FormatInt(end.Unix(), 10))
		payload, _, err := p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/log/self?"+query.Encode(), state.headers, state.credential.Cookie, nil)
		if err != nil {
			return UpstreamDailySnapshot{}, nil, fmt.Errorf("读取 New API %s 第 %d 页日志: %w", start.Format("2006-01-02"), page, err)
		}
		items := extractItems(payload)
		for _, item := range items {
			object := asMap(item)
			if object == nil {
				continue
			}
			itemTokens, hasTotal := int64Value(valueByKeys(object, "total_tokens"))
			if !hasTotal {
				prompt, _ := int64Value(valueByKeys(object, "prompt_tokens", "input_tokens"))
				completion, _ := int64Value(valueByKeys(object, "completion_tokens", "output_tokens"))
				itemTokens = prompt + completion
				if itemTokens == 0 {
					itemTokens, _ = int64Value(valueByKeys(object, "token_used"))
				}
			}
			itemQuota, _ := numberValue(valueByKeys(object, "quota", "use_quota", "quota_used"))
			tokens += itemTokens
			quota += itemQuota
			groupName := stringValue(valueByKeys(object, "group", "group_name"))
			if groupName != "" {
				usage := groups[groupName]
				usage.RemoteID = groupName
				usage.Name = groupName
				usage.TodayTokens += itemTokens
				usage.TodayCostUSD += itemQuota / state.quotaPerUnit
				groups[groupName] = usage
			}
		}
		total, hasTotal := int64Value(valueByKeys(apiData(payload), "total", "total_count"))
		if len(items) == 0 || len(items) < newAPILogPageSize || (hasTotal && int64(page*newAPILogPageSize) >= total) {
			break
		}
		if page == 10000 {
			return UpstreamDailySnapshot{}, nil, fmt.Errorf("new API %s 日志分页超过安全上限", start.Format("2006-01-02"))
		}
	}

	statQuery := url.Values{}
	statQuery.Set("type", "0")
	statQuery.Set("start_timestamp", strconv.FormatInt(start.Unix(), 10))
	statQuery.Set("end_timestamp", strconv.FormatInt(end.Unix(), 10))
	stat, _, statErr := p.http.doJSON(ctx, http.MethodGet, site.BaseURL, "/api/log/self/stat?"+statQuery.Encode(), state.headers, state.credential.Cookie, nil)
	if statErr == nil {
		if statQuota, ok := numberValue(valueByKeys(apiData(stat), "quota", "used_quota", "total_quota")); ok {
			quota = statQuota
		}
	} else if !isHTTPStatus(statErr, http.StatusNotFound) {
		return UpstreamDailySnapshot{}, nil, fmt.Errorf("读取 New API %s 日志统计: %w", start.Format("2006-01-02"), statErr)
	}
	return UpstreamDailySnapshot{Date: start, Tokens: tokens, CostUSD: quota / state.quotaPerUnit}, groups, nil
}

func parseNewAPIGroups(value any) []UpstreamGroupSnapshot {
	groups := make([]UpstreamGroupSnapshot, 0)
	if object := asMap(value); object != nil {
		if nested := object["groups"]; nested != nil {
			if asMap(nested) != nil || asSlice(nested) != nil {
				return parseNewAPIGroups(nested)
			}
		}
		if nested := asMap(object["groups"]); nested != nil {
			object = nested
		}
		for key, raw := range object {
			if key == "total" || key == "items" {
				continue
			}
			name := key
			remoteID := key
			multiplier := floatPointer(raw)
			if item := asMap(raw); item != nil {
				if value := stringValue(valueByKeys(item, "name", "group_name")); value != "" {
					name = value
				}
				if value := stringValue(valueByKeys(item, "id", "key")); value != "" {
					remoteID = value
				}
				multiplier = floatPointer(valueByKeys(item, "ratio", "rate", "multiplier"))
			}
			description := ""
			platform := ""
			if item := asMap(raw); item != nil {
				description = stringValue(valueByKeys(item, "description", "desc"))
				platform = stringValue(valueByKeys(item, "platform", "provider", "provider_type"))
			}
			platform = resolveNewAPIGroupPlatform(platform, name, description)
			groups = append(groups, UpstreamGroupSnapshot{
				RemoteID: remoteID, Name: name, Platform: platform,
				Description: description, Multiplier: multiplier,
			})
		}
	}
	if items := asSlice(value); items != nil {
		for index, raw := range items {
			item := asMap(raw)
			if item == nil {
				continue
			}
			name := stringValue(valueByKeys(item, "name", "group_name"))
			remoteID := stringValue(valueByKeys(item, "id", "key"))
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
			description := stringValue(valueByKeys(item, "description", "desc"))
			platform := resolveNewAPIGroupPlatform(
				stringValue(valueByKeys(item, "platform", "provider", "provider_type")),
				name,
				description,
			)
			groups = append(groups, UpstreamGroupSnapshot{
				RemoteID: remoteID, Name: name, Platform: platform,
				Description: description,
				Multiplier:  floatPointer(valueByKeys(item, "ratio", "rate", "multiplier")),
			})
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
}

func resolveNewAPIGroupPlatform(explicit, name, description string) string {
	if platform := canonicalNewAPIGroupPlatform(explicit); platform != "" {
		return platform
	}
	if platform := inferNewAPIGroupPlatform(name + " " + description); platform != "" {
		return platform
	}
	return newAPIUpstreamPlatformName
}

func canonicalNewAPIGroupPlatform(value string) string {
	trimmed := strings.TrimSpace(value)
	normalized := strings.ToLower(trimmed)
	switch normalized {
	case "openai", "open ai":
		return "OpenAI"
	case "anthropic", "claude":
		return "Anthropic"
	case "gemini", "google", "google ai":
		return "Gemini"
	case "grok", "xai", "x.ai":
		return "Grok"
	case "antigravity", "google antigravity":
		return "Antigravity"
	case "newapi", "new api":
		return ""
	default:
		if strings.Contains(normalized, "newapi") || strings.Contains(normalized, "new api") {
			return ""
		}
		return trimmed
	}
}

func inferNewAPIGroupPlatform(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(normalized, "claude"), strings.Contains(normalized, "anthropic"):
		return "Anthropic"
	case strings.Contains(normalized, "kiro"), strings.Contains(normalized, "sonnet"), strings.Contains(normalized, "opus"), strings.Contains(normalized, "haiku"):
		return "Anthropic"
	case strings.Contains(normalized, "gpt"), strings.Contains(normalized, "openai"):
		return "OpenAI"
	case strings.Contains(normalized, "antigravity"):
		return "Antigravity"
	case strings.Contains(normalized, "gemini"), strings.Contains(normalized, "google ai"):
		return "Gemini"
	case strings.Contains(normalized, "grok"), strings.Contains(normalized, "x.ai"):
		return "Grok"
	default:
		return ""
	}
}

func collectNewAPIPricingRates(value any) map[string]float64 {
	result := collectNamedRates(value)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, raw := range typed {
				if strings.EqualFold(key, "group_ratio") || strings.EqualFold(key, "group_ratios") {
					for name, rate := range collectNamedRates(raw) {
						if _, exists := result[name]; !exists {
							result[name] = rate
						}
					}
				}
				walk(raw)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return result
}
