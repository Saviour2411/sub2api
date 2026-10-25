package service

import "slices"

// CloneSchedulerSnapshot 隔离调度精简投影的可变字段，保留进程内解码缓存。
// 投影的 AccountGroups 仅含标量，不包含 Account、Group 或 Proxy 关联实体。
func (a *Account) CloneSchedulerSnapshot() Account {
	cloned := *a
	cloned.Credentials = cloneSchedulerJSONMap(a.Credentials)
	cloned.Extra = cloneSchedulerJSONMap(a.Extra)
	cloned.AccountGroups = slices.Clone(a.AccountGroups)
	cloned.GroupIDs = slices.Clone(a.GroupIDs)
	cloned.LoadFactor = cloneAccountValuePointer(a.LoadFactor)
	cloned.RateMultiplier = cloneAccountValuePointer(a.RateMultiplier)
	cloned.LastUsedAt = cloneAccountValuePointer(a.LastUsedAt)
	cloned.ExpiresAt = cloneAccountValuePointer(a.ExpiresAt)
	cloned.RateLimitedAt = cloneAccountValuePointer(a.RateLimitedAt)
	cloned.RateLimitResetAt = cloneAccountValuePointer(a.RateLimitResetAt)
	cloned.OverloadUntil = cloneAccountValuePointer(a.OverloadUntil)
	cloned.TempUnschedulableUntil = cloneAccountValuePointer(a.TempUnschedulableUntil)
	cloned.SessionWindowStart = cloneAccountValuePointer(a.SessionWindowStart)
	cloned.SessionWindowEnd = cloneAccountValuePointer(a.SessionWindowEnd)
	cloned.ParentAccountID = cloneAccountValuePointer(a.ParentAccountID)
	// 请求可原地修改映射，不能沿用共享母本的免签名缓存。
	cloned.modelMappingCache = nil
	cloned.modelMappingCacheReady = false
	cloned.modelMappingCacheShared = false
	cloned.headerOverrideCache = nil
	cloned.headerOverrideCacheReady = false
	return cloned
}

func cloneSchedulerJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneSchedulerJSONValue(item)
	}
	return cloned
}

func cloneSchedulerJSONValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		return cloneSchedulerJSONMap(item)
	case []any:
		cloned := make([]any, len(item))
		for i, element := range item {
			cloned[i] = cloneSchedulerJSONValue(element)
		}
		return cloned
	default:
		return value
	}
}
