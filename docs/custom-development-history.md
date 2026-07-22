# 二次开发功能台账

本文档是本仓库相对上游 `Wei-Shaw/sub2api` 的二次开发功能事实来源，用于回答以下问题：

- 当前有哪些仍需独立维护的二次开发能力；
- 每项能力的行为边界、默认状态和主要代码入口是什么；
- 上游同步、功能修改、停用或删除时，哪些本地行为必须保留或重新评估；
- 某项二次开发能力在何时、因何发生过变化，以及使用什么方式验证。

上游提交的逐项处置继续记录在 `docs/upstream-sync-history.md`。本文件不重复上游提交清单，只记录本地能力及其演进。

## 当前基线

- 基线日期：2026-07-20
- 本地版本：`0.1.203`
- 本地代码基线提交：`fc53abf1b`
- 已完整集成的上游提交：`e625ce3b3b3b955b7c3afc93221f7c5f0ae55aa8`
- 比较范围：`e625ce3b3..fc53abf1b`
- 基线差异：648 个文件，新增 86210 行、删除 3895 行；本地独有提交 402 个
- 当前能力族：46 项，分布在 9 个功能域

这组数字只用于确认分析边界，不能直接等同于功能数量。生成代码、测试、文案、上游提交的本地适配和同一能力的连续修复均会放大差异规模。

## 识别口径

以下情况纳入本台账：

1. 当前代码相对已完整集成的上游仍存在独立业务行为；
2. 功能来自上游，但本地存在明确覆盖策略，后续同步不能直接采用上游默认行为；
3. 生产部署、数据挂载或发布流程存在必须长期保留的本地约束；
4. 多个实现提交共同组成一项用户或运维人员能够感知的能力。

以下情况不单独列项：

1. 已被上游完整吸收且本地不再有行为差异的功能；
2. 不改变能力边界的普通缺陷修复、格式化、生成文件和测试补充；
3. 仅为一次上游同步服务的临时兼容代码；
4. 版本号提交和重新触发 CI 的提交。

功能清单按“能力族”维护。例如首 Token 超时、失败归因和连续失败停调度分别有独立运行状态，因此拆分编号；同一能力的 handler、service、repository、前端和测试不再拆成多个编号。

## 维护规则

1. 新增二次开发能力时，必须在同一提交中新增稳定功能编号、当前行为、关键入口，并在“变更记录”追加一行。
2. 修改既有能力的行为、默认值、配置、接口、数据结构或生产约束时，必须更新对应清单项并追加变更记录。
3. 停用或删除能力时不得删除其编号；应将状态改为“已停用”或“已移除”，保留最后行为和替代方案，并追加变更记录。
4. 上游同步影响任一编号时，除更新 `docs/upstream-sync-history.md` 外，还必须在本文件追加“上游适配”记录。
5. 纯上游功能且没有本地覆盖时，只写上游同步历史，不在本文件新增编号。
6. 一次提交影响多个能力时可以共用一条变更记录，但必须列出全部功能编号。
7. 变更记录必须包含提交或版本、变更类型、行为说明和验证结果，不使用“优化”“调整”等无法复核的孤立描述。
8. 台账不得写入凭据、令牌、私钥或实例专用敏感信息；生产实例的具体连接参数继续保存在不跟踪的本地运维说明中。

状态含义：

| 状态 | 含义 |
| --- | --- |
| 生效中 | 当前存在本地实现，属于持续维护范围 |
| 兼容覆盖 | 基于上游能力，但本地有不同默认值或处理策略 |
| 运维约束 | 不一定表现为产品功能，但发布或生产操作必须遵守 |
| 已停用 | 代码或入口仍可能存在，但正常配置下不再启用 |
| 已移除 | 当前代码已删除，仅保留历史记录和替代说明 |

## 当前功能清单

### 网关、故障转移与调度

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-GW-001` | 首 Token 超时与请求结果归因 | 在首个有效下游 Token 前执行可配置超时；区分 pending/final outcome、部分响应、客户端断开和流内失败，避免错误重试或漏结算。 | `backend/internal/service/first_token_timeout.go`、`backend/internal/handler/openai_upstream_outcome_tracker.go` | 生效中 |
| `CUST-GW-002` | 2xx 语义错误识别 | 按 contains/regex 规则检查有限长度的成功响应体，将命中的业务错误转为 failover、账号测试失败和可审计错误。 | `backend/internal/service/semantic_error.go`、`backend/internal/service/semantic_error_config.go` | 生效中 |
| `CUST-GW-003` | 流式心跳与长请求保活 | 支持响应前心跳、OpenAI compact SSE 心跳、Images JSON keepalive；首 Token 计时期间按路径暂停会提前提交状态的心跳，客户端断开后可继续排空上游完成计费。 | `backend/internal/service/openai_compact_sse_keepalive.go`、`backend/internal/service/openai_images_json_keepalive.go`、`backend/internal/service/gateway_upstream_response.go` | 生效中 |
| `CUST-GW-004` | Pool 模式重试与失败策略配置 | 在二开管理页配置默认重试次数、可重试状态码、首 Token 阈值、上游错误阈值和自动探测退避；使用策略 revision/fingerprint 避免运行时读到撕裂配置。 | `backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-005` | 自动托管账号恢复探测 | 为可恢复停调度账号创建或恢复唯一的自动测试计划，支持阶梯退避、默认测试提示词、成功后停用和并发重建保护。 | `backend/internal/service/admin_scheduled.go`、`backend/internal/service/scheduled_test_runner_service.go`、`backend/migrations/154_scheduled_test_auto_managed.sql`、`backend/migrations/155_scheduled_test_auto_managed_backoff.sql`、`backend/migrations/176_gateway_auto_managed_plan_uniqueness.sql` | 生效中 |
| `CUST-GW-006` | 连续失败停调度保护 | 记录账号错误 streak，达到阈值后临时移出调度；成功、人工清错和状态恢复时清理 streak，并与 strict 调度和调度 outbox 协同。 | `backend/internal/service/account_failure_streak.go`、`backend/internal/repository/account_failure_streak_cache.go`、`backend/internal/service/failure_scheduling_compat.go` | 生效中 |
| `CUST-GW-007` | OpenAI 高级调度器 | 支持 sticky weighted、订阅优先、Top-K 和 priority/load/queue/error-rate/TTFT/reset/quota/session 等权重；另有优先最近重置策略。 | `backend/internal/service/openai_account_scheduler.go`、`backend/internal/service/setting_parse.go` | 生效中 |
| `CUST-GW-008` | 增强故障转移与响应结算 | 覆盖非 JSON 2xx、SSE `event:error`、Responses 传输错误、图片服务错误、流式部分结果和已提交响应，复用错误体并避免重复写帧。 | `backend/internal/handler/failover_loop.go`、`backend/internal/service/upstream_outcome_error.go`、`backend/internal/service/openai_gateway_response_handling.go` | 生效中 |
| `CUST-GW-009` | 代理到期与失败回退 | 代理支持到期处理、质量测试、失败回退和定向调度刷新；生产调度不会继续使用已失效代理。 | `backend/internal/service/proxy_expiry_service.go`、`backend/internal/service/proxy_fallback.go`、`backend/internal/repository/proxy_repo.go` | 生效中 |

### 协议与上游兼容

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-PROTO-001` | OpenAI Codex CLI 模拟与客户端策略 | API Key/OAuth 路径支持 Codex CLI 请求模拟、User-Agent 和版本范围策略、App Server 客户端识别及能力探测一致性。 | `backend/internal/service/openai_codex_emulation.go`、`backend/internal/service/openai_codex_identity.go` | 生效中 |
| `CUST-PROTO-002` | Codex 图片工具策略 | 可完全禁用或按策略处理图片生成工具；覆盖原生 Responses、Chat fallback、namespace 和 WSv2，防止换名、嵌套或重复注入绕过。 | `backend/internal/service/codex_image_generation_bridge.go`、`backend/internal/service/openai_responses_namespace.go`、`backend/internal/service/setting_service_codex_policy_test.go` | 生效中 |
| `CUST-PROTO-003` | Anthropic Claude Code 上游模拟 | 支持全局开关和分组开关，将普通下游请求按 Claude Code 客户端规则清洗、补充和转发；账号测试与真实网关使用同一策略。 | `backend/internal/service/gateway_anthropic_mimicry_sanitize.go`、`backend/internal/service/gateway_anthropic_oauth_mimicry.go`、`backend/migrations/156_add_group_claude_code_upstream_mimicry.sql` | 生效中 |
| `CUST-PROTO-004` | Claude OAuth 提示词与协议清洗 | 可配置 OAuth system prompt/blocks，处理 beta header、thinking block、Bedrock CC 兼容和重试时的协议字段。 | `backend/internal/service/gateway_claude_oauth_body.go`、`backend/internal/service/gateway_tool_rewrite.go`、`backend/internal/service/setting_update.go` | 生效中 |
| `CUST-PROTO-005` | API Key 账号请求头覆写 | Anthropic/OpenAI API Key 账号可以配置请求头覆写；真实转发、模型列表、能力探测、count tokens、Images 和 WS 使用一致的最终请求头。 | `backend/internal/service/openai_gateway_forward.go`、`backend/internal/service/upstream_models.go`、`frontend/src/components/account/EditAccountModal.vue` | 生效中 |
| `CUST-PROTO-006` | 请求端点与 compact 信号规范化 | 从原始路径规范化入站端点，区分 Responses compact，识别 `compaction_trigger`，并保留本地 HTTP 流转 WSv2 和 namespace 的决策边界。 | `backend/internal/handler/gateway_handler.go`、`backend/internal/service/openai_gateway_request_body.go`、`backend/internal/service/openai_ws_http_bridge.go` | 生效中 |
| `CUST-PROTO-007` | Responses 渠道监控请求格式 | 渠道监控使用结构化 Responses `input` 消息数组，而不是普通字符串；上游同步不得退回非标准请求体。 | `backend/internal/service/channel_monitor_checker.go`、`frontend/src/components/admin/monitor/MonitorAdvancedRequestConfig.vue` | 兼容覆盖 |

### 账号与运维测试

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-ACC-001` | 账号策略默认值与模型白名单 | 创建账号时提供本地 pool mode 默认策略、模型白名单和平台特定控件；批量编辑保持这些字段的 API 契约。 | `frontend/src/components/account/poolModeDefaults.ts`、`frontend/src/components/account/ModelWhitelistSelector.vue`、`backend/migrations/145_group_models_list_config.sql` | 生效中 |
| `CUST-ACC-002` | 账号数据批量导入与可见性增强 | 前端支持拖拽/批量导入账号数据；账号列表展示账号 ID，并改进 API Key 查看、滚动和查询上下文重置。 | `frontend/src/views/admin/AccountsView.vue`、`frontend/src/components/account/CreateAccountModal.vue` | 生效中 |
| `CUST-ACC-003` | API Key 分组访问约束 | 严格校验 API Key 的专属分组访问，拒绝跨专属分组调度，并支持管理员按用户 API Key 所在分组筛选。 | `backend/internal/service/gateway_channel_restriction_test.go`、`backend/internal/service/admin_user.go`、`frontend/src/views/admin/AccountsView.vue` | 生效中 |
| `CUST-ACC-004` | OpenAI 额度查询与重置 | 账号管理可查询 rate-limit credits，并在明确操作时重置额度；调度器同步刷新相关快照。 | `backend/internal/service/openai_quota_service.go`、`backend/internal/service/openai_quota_reset_credits.go` | 生效中 |
| `CUST-ACC-005` | 增强账号测试 | 手动和定时测试覆盖 OpenAI Responses/compact/Images、Anthropic 模拟、Gemini 等平台，并执行语义错误、工具调用和提示词校验。 | `backend/internal/service/account_test_service.go`、`frontend/src/components/admin/account/AccountTestModal.vue` | 生效中 |
| `CUST-ACC-006` | 本地数据兼容迁移 | 维护注册来源默认授权、平台 quota、订阅到期通知和账号扩展字段的迁移兼容，避免本地历史数据在上游同步后丢失约束。 | `backend/migrations/142_extend_user_provider_default_grants_check.sql`、`backend/migrations/143_subscription_expiry_notify_enabled.sql`、`backend/migrations/144_user_platform_quotas.sql` | 兼容覆盖 |

### 计费、定价与可观测性

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-BILL-001` | 按用户请求模型计费 | 计费优先使用用户请求模型而不是上游映射模型，并记录 billing model source；Chat、Responses、Images 等入口保持一致。 | `backend/internal/handler/requested_model_pricing.go`、`backend/internal/service/requested_model_pricing.go`、`backend/migrations/175_force_requested_billing_model_source.sql` | 生效中 |
| `CUST-BILL-002` | 多层定价回退 | 支持部分渠道区间回退、渠道默认定价、DeepSeek/GLM/Kimi/MiniMax/豆包等兜底定价，以及 thinking、图片和视频计价补充。 | `backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/pricing_service.go`、`backend/migrations/147_channel_default_pricing.sql` | 生效中 |
| `CUST-BILL-003` | OpenAI 长上下文计费兼容策略 | 账号开关叠加本地区间计价逻辑；只有实际命中渠道区间时才禁用内置倍率。历史缺失开关的主账号回填为开启，新账号默认关闭。 | `backend/internal/service/billing_service.go`、`backend/migrations/175_default_openai_long_context_billing.sql` | 兼容覆盖 |
| `CUST-BILL-004` | Image 分组成功率 | 统计单次和批量图片请求的分组请求数/失败数，支持代次式原子清零、用户展示开关，并排除 keepalive 字节对结果判断的干扰。 | `backend/internal/service/image_group_success_rate.go`、`backend/internal/repository/image_group_success_rate_repo.go`、`backend/migrations/177_image_group_success_rates.sql` | 生效中 |
| `CUST-BILL-005` | 用量与费用明细增强 | 展示缓存 Token、请求模型、余额调整和图片/视频费用信息；错误请求和上游端点记录使用本地归因规则。 | `backend/internal/service/gateway_usage_billing.go`、`frontend/src/views/admin/UsageView.vue`、`frontend/src/components/admin/user/UserBalanceHistoryModal.vue` | 生效中 |

### 渠道监控与可观测性

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-OBS-001` | 流式渠道监控与随机抖动 | 渠道监控和模板可以选择流式检测；检测间隔支持正负随机抖动，避免大量渠道同一时刻探测，并保留 provider/endpoint/请求体的本地扩展。 | `backend/internal/service/channel_monitor_checker.go`、`backend/ent/schema/channel_monitor.go`、`backend/migrations/141_channel_monitor_stream_enabled.sql` | 生效中 |
| `CUST-OBS-002` | 独立上游站点同步监控 | 在二开管理页统一维护 Sub2API/New API 上游站点，定时同步余额、分组倍率与实际消耗，保留用量及倍率历史；Sub2API 同步 Token 与费用，New API 仅通过标准统计接口同步站点及分组费用，不再分页扫描日志，Token 指标明确标记为不可用并隐藏，既有 Token 数据保留但不再更新；支持按分组类型筛选、按余额/有效今日 Token 排序、拖拽调整站点顺序，并按平台优先级和倍率整理账号下方分组；上游分组可绑定本地账号并按最后一次有效倍率自动维护全局优先级，不可用分组仍参与排序，从未取得有效倍率的账号保留原优先级；新增 Sub2API 时会探测 Turnstile，自动改用令牌认证并支持从完整登录响应导入 Access/Refresh Token；令牌模式可在加密凭证中保存登录会话 User-Agent，并在验证、刷新与同步请求中保持一致；目标站点返回 `SESSION_BINDING_MISMATCH` 时会以 Chrome TLS/HTTP2 指纹重试，成功后将该传输模式随凭证加密保存，以兼容绑定出口 IP、UA 与 TLS/JA4 指纹的浏览器会话；Sub2API JWT Access Token 会在到期前 15 分钟主动轮换，非 JWT 令牌保持原有按认证失败刷新行为；同步时优先复用加密保存的 Token/Cookie，仅在进入主动刷新窗口、凭证被拒绝或需要重新登录时更新认证。 | `backend/internal/service/upstream_service.go`、`backend/internal/service/upstream_provider_http.go`、`frontend/src/components/admin/upstream/UpstreamManagementPanel.vue`、`backend/migrations/178_upstream_management.sql`、`backend/migrations/181_upstream_management_order_and_platform.sql` | 生效中 |

### 产品、支付与增长

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-PROD-001` | 每日签到转盘 | 支持余额、并发、订阅和空奖品，按万分比配置概率；支持未付费用户奖励衰减、LinuxDo 豁免、记录查询和防重复签到。默认关闭。 | `backend/internal/service/daily_checkin_service.go`、`backend/migrations/136_daily_checkins.sql`、`backend/migrations/146_daily_checkin_spin_rewards.sql`、`frontend/src/views/user/DailyCheckinView.vue` | 生效中 |
| `CUST-PROD-002` | 模型广场 | 按指定分组公开模型、协议格式、价格和介绍，管理端可配置开关、介绍和分组范围。默认启用，但只返回公开白名单字段。 | `backend/internal/handler/model_marketplace_handler.go`、`frontend/src/views/ModelMarketplaceView.vue` | 生效中 |
| `CUST-PROD-003` | 公开模型定价 | 提供独立定价 API/页面，展示充值倍率和模型费用；导航入口可以隐藏，但直达页面和 API 仍受公开设置控制。 | `backend/internal/handler/payment_handler.go`、`frontend/src/views/ModelPricingView.vue` | 生效中 |
| `CUST-PROD-004` | 批量图片任务 | 提供任务提交、队列处理、列表、明细、输出、下载、取消和清理；限制到允许的 Gemini 分组，包含余额预占、失败恢复和有界结算重试。 | `backend/internal/service/batch_image.go`、`backend/internal/service/batch_image_worker.go`、`backend/internal/server/routes/gateway.go` | 生效中 |
| `CUST-PROD-005` | 兑换码多次使用 | 兑换码支持最大使用次数，并以用户维度记录使用明细，防止同一用户重复使用同一码。 | `backend/internal/repository/redeem_code_repo.go`、`backend/migrations/140_redeem_code_usage_limits.sql` | 生效中 |
| `CUST-PROD-006` | 支付和充值增强 | 支持余额充值赠送阶梯、手续费/倍率、赠送快照、自定义 EasyPay 支付方式、订阅 USD/CNY 换算预览和管理员删除订单。 | `backend/internal/service/payment_amounts.go`、`backend/internal/payment/provider/easypay.go`、`backend/migrations/149_payment_balance_bonus_rules.sql` | 生效中 |

### 风控与内容审核

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-RISK-001` | 本地人工审核对话 | 内容审核可进入本地人工审计，持久化请求/响应记录；管理端支持列表、详情、下载和删除。统一安全审计协调器接入上游 prompt audit，协调器未配置时继续使用本地内容审核，审计服务过载时按配置执行回退。 | `backend/internal/securityaudit/coordinator.go`、`backend/internal/service/content_moderation_local_audit.go`、`backend/internal/handler/security_audit_helper.go` | 生效中 |
| `CUST-RISK-002` | Cyber 会话阻断 | `cyber_policy` 命中可沿网关、审计和计费链路透传，并按配置对会话做 TTL 阻断；用量记录允许 `cyber_blocked` 类型。 | `backend/internal/service/content_moderation.go`、`backend/migrations/174_allow_cyber_blocked_usage_request_type.sql` | 生效中 |
| `CUST-RISK-003` | 错误请求详情可见性 | 管理端运维详情支持从账号、用户和请求上下文继续导航；可按设置允许用户查看自己的错误请求详情。 | `frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue`、`backend/internal/service/setting_user_error_view_test.go` | 生效中 |

### 前端与管理体验

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-UI-001` | 独立二开管理页 | 将上游同步监控、模型广场、每日签到和网关运行策略集中在 `/admin/custom-features`，与上游通用设置隔离；页面使用宽屏管理台布局展示高密度数据。 | `frontend/src/views/admin/CustomFeaturesView.vue`、`backend/internal/server/routes/admin_custom_features.go` | 生效中 |
| `CUST-UI-002` | 统一功能开关注册表 | 公共设置驱动的入口统一声明 opt-in/opt-out 语义，避免 SSR 注入缺字段导致刷新闪烁或错误隐藏。 | `frontend/src/utils/featureFlags.ts`、`frontend/src/__tests__/featureFlags.spec.ts` | 生效中 |
| `CUST-UI-003` | 机甲冷蓝主题与浅色默认 | 保留自定义认证背景、扫描线、计数、倾斜等视觉组件，默认使用浅色主题并提供暗色兼容。 | `frontend/src/components/common/BackgroundFX.vue`、`frontend/src/composables/useTilt.ts`、`frontend/src/stores/app.ts` | 生效中 |
| `CUST-UI-004` | 表格和导航稳定性 | 保留账号列表禁用虚拟化、查询后滚动重置、分页大小持久化、侧边栏滚动位置和日期下拉层级修复。 | `frontend/src/components/common/DataTable.vue`、`frontend/src/components/layout/TablePageLayout.vue`、`frontend/src/components/layout/AppSidebar.vue` | 生效中 |
| `CUST-UI-005` | 认证页加载与存储容错 | 注册页等待公共设置后再开放输入和提交，登录协议、Turnstile 与 OAuth 开关使用同一设置快照；local/session storage 不可用时安全降级，避免新会话白屏。 | `frontend/src/views/auth/RegisterView.vue`、`frontend/src/stores/auth.ts`、`frontend/src/utils/browserStorage.ts` | 生效中 |

### CI、发布与生产约束

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-OPS-001` | 标签发布与生产自动部署 | `v*` 标签构建发布产物、Docker Hub/GHCR 镜像并自动 SSH 部署；包含连接诊断、重试、健康检查和版本文件回写。 | `.github/workflows/release.yml`、`deploy/remote-deploy.sh` | 运维约束 |
| `CUST-OPS-002` | 保留生产 Compose | 自动部署默认不上传仓库 Compose，只更新远端 `.env` 镜像标签并使用活动 Compose，防止通用命名卷配置覆盖生产文件。 | `.github/workflows/release.yml`、`deploy/remote-deploy.sh` | 运维约束 |
| `CUST-OPS-003` | 生产数据与网络拓扑 | 生产实例必须保持 bind mount 数据目录、仅回环地址暴露、两个活动 Compose 一致，以及业务要求的 HTTP upstream 安全开关；实例专用路径和连接参数只保存在不跟踪的本地运维说明中。 | `deploy/remote-deploy.sh`、`.github/workflows/release.yml` | 运维约束 |
| `CUST-OPS-004` | 本地 CI 与安全门禁 | Push/PR 运行 Go 1.26.5 单元和集成测试、Node 20 前端测试、lint、部署脚本检查及依赖安全扫描；发布前以远端 CI 为最终门禁。 | `.github/workflows/backend-ci.yml`、`.github/workflows/security-scan.yml` | 运维约束 |

## 变更记录

记录按时间倒序追加。功能清单描述“现在是什么”，本节描述“为什么变成这样”。

| 日期 | 版本/提交 | 类型 | 功能编号 | 变更与原因 | 验证 |
| --- | --- | --- | --- | --- | --- |
| 2026-07-22 | `0.1.207` / 待提交 | 修改 | `CUST-OBS-002` | New API 用量同步停止每 5 分钟全量分页扫描当日日志，改为按日期调用一次 `/api/log/self/stat` 获取总费用，并对当天可用分组顺序携带 `group` 查询分组费用；New API 不再维护无法由标准统计接口直接提供的 Token 指标，API 通过 `token_metrics_available=false` 明确能力边界，管理页以“—”隐藏站点、分组和历史 Token，既有数据库值保留且不再覆盖。解决高日志量站点固定在深分页触发 HTTP 429、随后重复从首页扫描的问题。 | New API 统计请求计数与错误测试、仓储旧 Token 保留测试、前端组件测试、类型检查和生产构建 |
| 2026-07-21 | `0.1.206` / 待提交 | 修改 | `CUST-OBS-002` | Sub2API 令牌认证会读取 Access Token JWT 的 `exp` 作为调度提示，并在到期前 15 分钟主动轮换 Access/Refresh Token；JWT 不验签且不参与本地认证，非 JWT 或无 `exp` 的上游令牌保持原有按 401 刷新行为。避免 Access Token 与 Refresh Token 同时到期时，定时同步在访问令牌失效后才刷新而错过 Refresh Token 的有效期。 | Provider 主动刷新窗口、窗口外复用、异常令牌兼容测试及生产构建验证 |
| 2026-07-20 | `0.1.203` / `fc53abf1b` | 上游适配 | `CUST-GW-001`、`CUST-GW-006`、`CUST-GW-008`、`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-006`、`CUST-PROTO-007`、`CUST-BILL-001`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-004`、`CUST-OPS-003` | 完整合并上游 `da85cc7e4..e625ce3b3`，接入 Agent Identity、WS 终态/turn 生命周期、倍率探测、图片输入定价和 prompt audit；保留本地首 Token、连续失败停调度、请求模型计费、Claude 严格模拟、内容审核/Cyber 阻断、DataTable 滚动和生产 Compose 约束，并修复 HTTP bridge 后续轮次切号、定价空指针及双方测试契约。 | Go unit、golangci-lint、后端构建、前端 lint/typecheck/全量 Vitest/build、Apple container fixture、Compose/冲突/空白检查 |
| 2026-07-20 | `0.1.203` / 待提交 | 修改 | `CUST-OBS-002` | 修复不可用上游分组冻结整个本地分组优先级的问题：已有绑定改为使用当前或历史中的最后一次有效倍率参与排序，每次成功同步和保存绑定都会纠正优先级漂移；从未取得有效倍率的账号单独保留原优先级，不再阻断其他账号排序。 | 后端仓储回归测试、前端组件测试、类型检查、lint 和生产构建 |
| 2026-07-20 | `0.1.203` / 待提交 | 修改 | `CUST-OBS-002` | Sub2API 令牌认证新增浏览器 TLS 指纹自适应：普通客户端收到 `SESSION_BINDING_MISMATCH` 后以 Chrome TLS/HTTP2 指纹重试，成功后随加密凭证持久化，并用于后续验证、刷新和定时同步；自定义 Transport 继续复用 DNS Rebinding 防护。解决目标站点同时绑定出口 IP、User-Agent 与 TLS/JA4 指纹时令牌无法接入的问题。 | Chrome 指纹真实上游探针、Provider 自动回退测试、HTTP Client/Service 回归测试、生产环境认证与同步验证 |
| 2026-07-20 | `0.1.202` / 待提交 | 修改 | `CUST-OBS-002` | Sub2API 令牌认证新增会话 User-Agent：导入登录响应时自动记录当前浏览器 UA，与 Access/Refresh Token 一并加密保存，并在登录状态验证、令牌刷新和定时同步请求中持续复用，兼容同时绑定出口 IP 与 User-Agent 的上游会话风控。 | 后端 Provider/凭证生命周期测试、前端组件测试、类型检查、生产构建及生产环境认证验证 |
| 2026-07-16 | `0.1.200` / 待提交 | 修改 | `CUST-OBS-002` | Sub2API 密码模式改为优先复用 Access Token、认证失败后优先刷新并仅在刷新凭证被拒绝或接口不支持时回退密码登录；New API 优先复用 Cookie 和加密保存的远端用户 ID，仅在 Cookie 被拒绝时重新登录；网络、限流和服务端错误不触发重复认证，账号、地址、平台或密码变化时主动清除旧会话凭证。 | Provider 请求计数与认证恢复测试、凭证作用域失效测试、后端相关包测试、`go vet`、`golangci-lint` 及 `git diff --check` |
| 2026-07-16 | `0.1.199` / 待提交 | 修改 | `CUST-OBS-002` | Sub2API 上游新增公开设置能力探测；目标站开启 Cloudflare Turnstile 时自动禁用密码认证、切换令牌模式，并允许粘贴登录接口完整 JSON 导入访问/刷新令牌；未经过探测的创建请求也会返回专用 `UPSTREAM_TURNSTILE_REQUIRED` 错误，避免继续显示笼统的认证失败。 | 后端服务/Provider 测试、前端组件测试、类型检查、生产构建及 Playwright 表单流程检查 |
| 2026-07-16 | `0.1.199` / 待提交 | 修改 | `CUST-OBS-002` | 上游管理新增分组类型筛选、余额/今日 Token 双向排序和站点拖拽排序；New API 分组平台改为优先使用上游显式类型、其次按名称/描述识别 Kiro/Claude/OpenAI/Gemini/Grok/Antigravity，并迁移修复历史错误标签；账号下方分组按 OpenAI、Anthropic、Gemini、Grok、Antigravity 和倍率升序排列。 | 后端服务/仓储测试、前端组件测试、类型检查、生产构建及 Playwright 交互与多视口截图检查 |
| 2026-07-15 | `0.1.199` / 待提交 | 修改 | `CUST-OBS-002`、`CUST-UI-001` | 补录独立上游站点同步监控能力；二开管理页移除与应用顶栏重复的标题，放宽内容区并收紧纵向间距；上游主表固定紧凑列宽，将操作区改为 3×2 图标网格，使累计 Token、实际消耗和最后同步在常见桌面宽度下无需横向拖动即可查看。 | 前端组件测试、类型检查、生产构建及桌面/窄屏 Playwright 截图检查 |
| 2026-07-15 | `0.1.194` / `97e3d92c8` | 基线建立 | 全部当前项 | 以 `da85cc7e4` 为已完整集成上游基线，结合 561 个差异文件、377 个本地独有提交、同步记录、迁移、路由和运行配置建立首版能力族台账。 | 功能编号唯一性、关键路径存在性、`git diff --check` 和 Markdown 结构检查 |

后续记录格式示例：

```text
| YYYY-MM-DD | `版本` / `提交` | 新增/修改/上游适配/停用/移除 | `CUST-XXX-000` | 可复核的行为变化、原因和兼容边界 | 实际执行的测试或检查 |
```

## 上游同步检查清单

每次上游同步至少复核以下高风险能力族：

1. 网关 handler/service 签名变化：`CUST-GW-001` 至 `CUST-GW-009`；
2. OpenAI Responses、WSv2、namespace、图片工具变化：`CUST-PROTO-001`、`CUST-PROTO-002`、`CUST-PROTO-006`、`CUST-PROTO-007`；
3. Anthropic 请求转换和账号测试变化：`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-ACC-005`；
4. 计费和 usage schema 变化：`CUST-BILL-001` 至 `CUST-BILL-005`；
5. account/group/channel/payment schema 变化：`CUST-ACC-001`、`CUST-ACC-006`、`CUST-PROD-005`、`CUST-PROD-006`；
6. 前端设置、导航和公共配置变化：`CUST-PROD-001`、`CUST-PROD-002`、`CUST-OBS-002`、`CUST-UI-001` 至 `CUST-UI-005`；
7. release、Compose 或镜像变化：`CUST-OPS-001` 至 `CUST-OPS-004`。

## 已被上游吸收的历史二开

以下能力曾由本地提交引入或扩展，但在当前上游基线中已存在等价实现，本地没有独立行为差异，因此不占用当前功能编号：

| 能力 | 当前结论 | 复核方式 |
| --- | --- | --- |
| API Key 并发统计 | 当前上游已包含等价契约和实现；本地只保留仍有差异的专属分组访问约束，见 `CUST-ACC-003`。 | `git diff upstream/main..main` 不再包含并发统计实现差异 |
| 邀请返利核心流程和订阅返利 | 当前上游已包含等价实现；本地仍有差异的充值赠送、费用展示归入 `CUST-PROD-006` 和 `CUST-BILL-005`。 | `affiliate_service.go`、`payment_fulfillment.go` 相对当前上游无差异 |

如果上游已经完整提供同等能力，应在本文件把对应项记为“上游吸收”变更，确认本地差异和迁移兼容都可删除后，再将状态改为“已移除”；不得仅因代码冲突较多而无记录地放弃本地行为。
