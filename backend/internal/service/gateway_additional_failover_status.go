package service

import (
	"context"
	"slices"
)

// isAdditionalFailoverStatus 判断管理员配置的附加状态码是否应进入现有换号流程。
func isAdditionalFailoverStatus(settingService *SettingService, statusCode int) bool {
	if settingService == nil || statusCode < 100 || statusCode > 599 {
		return false
	}
	settings := settingService.GetGatewayRuntime(context.Background())
	return settings.AdditionalFailoverStatusCodesEnabled &&
		slices.Contains(settings.AdditionalFailoverStatusCodes, statusCode)
}
