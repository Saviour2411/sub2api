.PHONY: build build-backend build-frontend build-canvas test test-backend test-frontend test-canvas test-frontend-critical

FRONTEND_CRITICAL_VITEST := \
	src/__tests__/featureFlags.spec.ts \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts \
	src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts \
	src/router/__tests__/feature-access.spec.ts \
	src/components/admin/account/__tests__/AccountTestModal.spec.ts \
	src/utils/__tests__/accountTestModels.spec.ts \
	src/views/admin/__tests__/CustomFeaturesView.spec.ts \
	src/api/__tests__/admin.customFeatures.gateway.spec.ts \
	src/utils/__tests__/anthropicStreamDiagnostic.spec.ts \
	src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts \
	src/utils/__tests__/usageOutputRate.spec.ts \
	src/utils/__tests__/usageExport.spec.ts \
	src/views/user/__tests__/UsageView.spec.ts \
	src/views/admin/__tests__/UsageView.spec.ts \
	src/views/user/__tests__/ChannelStatusV1View.refresh.spec.ts \
	src/components/user/monitor/__tests__/MonitorCard.freshness.spec.ts \
	src/components/admin/monitor/__tests__/MonitorPrimaryModelCell.spec.ts \
	src/components/admin/usage/__tests__/UsageTable.spec.ts \
	src/api/__tests__/balanceQuery.spec.ts \
	src/api/__tests__/admin.userCustomizations.spec.ts \
	src/api/__tests__/admin.system.spec.ts \
	src/components/common/__tests__/VersionBadge.spec.ts \
	src/composables/__tests__/useVersionInfo.spec.ts \
	src/stores/__tests__/app.spec.ts \
	src/components/layout/__tests__/AppSidebar.spec.ts \
	src/components/common/__tests__/ConfirmDialog.spec.ts \
	src/components/admin/user/__tests__/UserCustomizationsPanel.spec.ts \
	src/views/public/__tests__/BalanceQueryView.spec.ts \
	src/utils/__tests__/canvasBridge.spec.ts

# 一键编译前后端
build: build-frontend build-backend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build
	@$(MAKE) build-canvas

build-canvas:
	@pnpm --dir canvas run build

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical
	@$(MAKE) test-canvas

test-canvas:
	@pnpm --dir canvas run typecheck
	@pnpm --dir canvas run test

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)
