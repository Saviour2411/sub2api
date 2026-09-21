# 上游同步历史

## 2026-07-26 发布核验补充（同步至 6d956bdc2）

- `v0.1.210` 更新到同步后提交并自动发布时，生产服务器继续使用 bind mount 活动 Compose，镜像 revision 已更新为 `4957c7b5876643037e7744535705e213947312a1` 且健康检查通过。
- 发布后独立对比发现，仓库中的 `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 虽然字节一致，但实际内容已被上游通用模板改为 Docker 命名卷，并将默认监听回退为 `0.0.0.0:8080`；此前同步记录将“文件一致”误判为“生产约束已保留”。
- 本次恢复两份生产 Compose 的 `./data`、`./postgres_data`、`./redis_data` bind mount 和 `127.0.0.1:18080` 默认监听，保留本轮上游新增环境变量及本地批准的连接池、worker、PostgreSQL、Go 内存和日志参数。
- 自动部署继续默认不上传 Compose；生产服务器活动 Compose 在更新前后均未切换到命名卷，生产数据目录未迁移或清理。

## 2026-07-29 同步至 5a6143097

- 执行时间：2026-07-29T01:44:21+08:00
- 执行状态：同步分支完整合并并通过本地验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`ae833c83c283a36943bab17251530c3f3b20e1ec`
- 上游代码合并提交：`565c685f88d698db65d21d160bcb1027252db515`
- 最后一个代码/测试提交：`ee222f0307fe8297c9dfdaa98991906bcf674d63`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`8fd01c2814f42997d79bdb4bafcbcfab2fabeee3`
- `UPSTREAM_NEW_SHA`：`5a6143097db142b72a6fc848c214e97214470bdd`
- merge-base：`8fd01c2814f42997d79bdb4bafcbcfab2fabeee3`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`5a6143097db142b72a6fc848c214e97214470bdd`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA；逐文件解决唯一文本冲突，复核自动合并路径的二开语义，并以独立测试提交补充 Passkey 禁用状态回归
- 备份分支：`backup/pre-upstream-sync-20260729-014421-ae833c83c`
- 同步分支：`sync/upstream-20260729-5a6143097`

### 上游提交处置

本次固定范围共 8 个提交，其中 3 个 merge commit、5 个 non-merge commit。6 个提交为 `Applied`，2 个版本提交因 patch-id 已在本地历史等价存在并继续保留本地较新版本，记为 `Already Applied + Overridden`；无 `Skipped`、`Deferred` 或未解决 `Conflict`。

| 上游提交 | 状态 | 内容与处置 |
| --- | --- | --- |
| `1c26dc7ad` | Applied | OpenAI Live finalize 的唯一用量日志改走 `writeUsageLogBestEffort`；observer 区分 store 故障与控制权接管，有限重试后按 `ExpiresAt` 兜底 finalize，避免租约和用量记录静默丢失 |
| `99c8e4bf7` | Applied | 合并 OpenAI Live store 容错修复；完整保留对应祖先关系 |
| `32618e71e` | Applied | 账号状态显示增加 `claude-sonnet-5` 到 `CSon5` 的别名 |
| `f2d824836` | Applied | 合并 Claude Sonnet 5 状态别名修复；完整保留对应祖先关系 |
| `acad7f1a0` | Applied | Passkey 功能禁用时不再请求凭据列表，并按 API 规范化错误的 `reason` 字段静默处理 `PASSKEY_DISABLED` |
| `6e1cbed42` | Applied | 合并 Passkey 禁用状态提示修复；完整保留对应祖先关系 |
| `b9c7cb8e2` | Already Applied + Overridden | 上游版本同步到 `0.1.167` 的 patch-id 已在本地历史等价存在；完整 merge 保留祖先关系，最终不回退本地版本 |
| `5a6143097` | Already Applied + Overridden | 上游版本同步到 `0.1.168` 的 patch-id 已在本地历史等价存在；完整 merge 保留祖先关系，最终继续使用本地 `0.1.211` |

### 本地提交与文件

- 上游范围整体映射到本地 merge commit `565c685f88d698db65d21d160bcb1027252db515`；其双亲为同步前本地 SHA 与固定上游 SHA。
- 独立回归测试提交 `ee222f0307fe8297c9dfdaa98991906bcf674d63` 新增 `ProfilePasskeyCard.spec.ts`，覆盖功能禁用时不发请求、`PASSKEY_DISABLED` 静默及其他错误仍提示三个场景。
- 写入两份台账前，代码与测试相对 `LOCAL_PRE_SYNC_SHA` 修改 6 个文件：新增 1 个、修改 5 个，共增加 335 行、删除 14 行，无删除文件。
- 主要更新：OpenAI Live observer/store 故障容错与 usage log 兜底、Claude Sonnet 5 状态缩写、Passkey 禁用状态请求和提示抑制。
- `backend/cmd/server/VERSION` 最终保持 `0.1.211`；前端构建未留下锁文件或其他未提交受控差异。

### 冲突与最终解决方案

- 唯一文本冲突位于 `backend/cmd/server/VERSION`；按批准方案保留本地 `0.1.211`，不回退到上游 `0.1.168`。
- `frontend/src/components/account/AccountStatusIndicator.vue` 自动合并后同时保留本地连续失败停调度状态逻辑和上游 `claude-sonnet-5 → CSon5` 别名。
- OpenAI Live 接入上游 store 故障容错和同步 usage log 兜底；本地连续失败停调度、按用户串行扣费、5 秒 usage task 超时及 Live 当前零计费行为均未改变。
- Passkey 组件在功能禁用时清空本地列表并跳过请求；配置变更竞态下仍按 `error.reason` 静默处理 `PASSKEY_DISABLED`，其他错误保持原有提示。
- 最终索引无未解决路径，未整文件采用 `ours` 或 `theirs`。

### 刻意保留的二次开发功能

- 账号连续失败停调度、strict 调度、pending/final outcome、streak 清理和调度 outbox 协同。
- OpenAI Live 当前零计费边界、用量明细归因、按用户串行扣费和 5 秒 usage task 超时；未仅为匹配 worker 数提高数据库连接池。
- 首 Token、WS 逐轮并发/结算/审计、普通文本严格请求模型计费、Composite 例外和媒体实际模型计费。
- 生产 bind mount、仅回环暴露、HTTP upstream 业务开关、实例级性能参数透传和双生产 Compose 一致性约束。

### 验证记录

验证使用 Go 1.26.3、Node 24、corepack 管理的 pnpm 9.15.9 和 golangci-lint 2.9.0；CI 基线为 Go 1.26.5 与 Node 20。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./...` | 0 | Go 全量 unit 通过，耗时 203.1 秒 |
| 同步前 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步前 | `CGO_ENABLED=0 go build -trimpath ./cmd/server` | 0 | 后端构建通过 |
| 同步前 | `corepack pnpm install --offline --frozen-lockfile`、`lint:check`、`typecheck`、`test:run`、`build` | 0 | 前端离线冻结安装、静态检查、类型检查、全量 Vitest 和生产构建均通过 |
| 适配 | `go test -tags=unit -run '^$' ./...` | 0 | Go 全包编译通过 |
| 适配 | OpenAI Live 6 个定向测试 | 0 | observer/store 故障重试、到期 finalize、usage log 兜底和既有生命周期场景通过 |
| 适配 | `AccountStatusIndicator.spec.ts` | 0 | 11 个测试通过，覆盖 Sonnet 5 别名及既有停调度状态 |
| 适配 | `ProfilePasskeyCard.spec.ts` | 0 | 3 个测试通过 |
| 同步后 | `go test -tags=unit ./...` | 0 | Go 全量 unit 通过，耗时 174.8 秒 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath ./cmd/server` | 0 | 后端构建通过 |
| 同步后 | `corepack pnpm install --offline --frozen-lockfile`、`lint:check`、`typecheck` | 0 | 均通过 |
| 同步后 | `corepack pnpm run test:run` | 0 | 前端全量 Vitest 通过，耗时 34.5 秒；仅输出既有 Vue/i18n 警告 |
| 同步后 | `corepack pnpm run build` | 0 | `vue-tsc -b` 与 Vite 生产构建通过；仅有既有 Browserslist、动态/静态导入和大 chunk 非致命警告 |
| 同步后 | Compose 哈希、bind mount/回环/性能变量静态复核、祖先关系、冲突标记、意外删除、敏感路径和 `git diff --check` | 0 | 两份生产 Compose 字节一致且 SHA-256 均为 `7091912E26F962DD1A37CAC5E16876E6489E59E4192DE0989DBCD3BE920B3D1E`；固定上游 SHA 已成为当前分支祖先；无新增异常 |

### 未验证项与残余风险

- 未运行 Docker、Testcontainers、`go test -tags=integration ./...`、真实 PostgreSQL migration 或 `-race`。
- 未运行 `govulncheck`；漏洞可达性需由后续 CI 或具备相应工具的隔离环境复核。
- 未读取 `.env`，未启动依赖 PostgreSQL/Redis 的真实服务，因此实例专用性能值、本地服务启动和健康检查未验证。
- 未注入真实 Redis 故障，未使用真实 OpenAI Live attestation/媒体会话或 Passkey 硬件；相关行为由本地单元和组件回归覆盖。
- 本地 Go 1.26.3、Node 24 与 CI Go 1.26.5、Node 20 存在环境差异，需以后续远端 CI 为发布门禁。
- 未执行 push、PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-07-29 同步至 8fd01c281

- 执行时间：2026-07-29T03:34:27+08:00
- 执行状态：同步分支完整合并并通过本地验证；本记录与 merge commit 同一提交，随后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`b77f979d61cea38f2acfc248480a2a8b9d8ff172`
- 上游代码合并提交：本记录与 merge commit 同一提交，最终 SHA 仅在执行总结中报告
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`6d956bdc20f0d8c38275d4d77b628a8ff776711c`
- `UPSTREAM_NEW_SHA`：`8fd01c2814f42997d79bdb4bafcbcfab2fabeee3`
- merge-base：`6d956bdc20f0d8c38275d4d77b628a8ff776711c`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`8fd01c2814f42997d79bdb4bafcbcfab2fabeee3`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决 29 个文本冲突和关联语义冲突，重新生成 Ent/Wire，并将二开兼容处理及两份历史台账纳入同一 merge commit
- 备份分支：`backup/pre-upstream-sync-20260729-014042-b77f979d6`
- 同步分支：`sync/upstream-20260729-8fd01c281`

### 上游提交处置

本次固定范围共 127 个提交，其中 53 个 merge commit、74 个 non-merge commit。127 个提交均通过完整 merge 保留祖先关系并记为 `Applied`，其中下表列出的 13 个提交同时按本地二次开发边界记为 `Applied + Overridden`；无 `Already Applied`、`Skipped`、`Deferred` 或未解决 `Conflict`。

| 上游提交集合 | 数量 | 状态 | 内容与处置 |
| --- | ---: | --- | --- |
| `6d956bdc2..8fd01c281` | 127 | Applied | 完整接入 OpenAI Live 与 macOS attestation、Passkey、上游 Model Plaza、面板 API 限流、Ollama Cloud 用量刷新、注册邮箱别名去重、Kimi K3、Responses/Anthropic 工具兼容、支付统计、公告样式、Caddy SSE、依赖安全更新及相关测试 |
| `2730c1c43`、`59ce11c78` | 2 | Applied + Overridden | 保留上游版本提交的祖先关系，最终版本继续使用本地较新的 `0.1.211`，不回退到上游 `0.1.165` 或 `0.1.166` |
| `1f45c99de`、`be65c713f` | 2 | Applied + Overridden | 接入映射模型与最终上游模型的用量归因并始终记录 `upstream_model`；普通文本计费继续严格使用用户请求模型，Composite 保留显式别名渠道价例外，图片和视频继续使用专用媒体计费模型 |
| `7ce6e8d65`、`eb6e3d1f1` | 2 | Applied + Overridden | 接入 WS 每轮模型跟踪；每轮只执行一次渠道映射和账号映射，并继续保留逐轮并发释放、用量快照、请求体哈希、失败停调度和审计 |
| `7b3ed2a96`、`b468e428e` | 2 | Applied + Overridden | 保留 OAuth prompt cache 修复提交的祖先关系和缓存断点处理；真实 Claude Code 仍只信任 handler 的严格客户端判定，不采用仅凭 body billing block 的宽松识别 |
| `71d7f8688`、`3ce8efc12`、`d96b6a31f` | 3 | Applied + Overridden | 接入 Antigravity OpenAI 兼容、非流式空响应拒绝及流式转换修复；只有账号启用 Pool Mode 时才允许空响应在同账号重试，避免普通账号绕过既有故障转移边界 |
| `720c405e3`、`8fd01c281` | 2 | Applied + Overridden | 接入上游 `/model-plaza`、分组范围和定价展示，但默认关闭；本地 `/models` 模型市场、默认值、管理配置和公开字段白名单继续独立保留 |

### 本地提交与文件

- 在写入两份台账前，合并树相对 `LOCAL_PRE_SYNC_SHA` 修改 354 个代码、配置、资源和测试文件：新增 66 个、修改 283 个、删除 5 个，共增加 20129 行、删除 1101 行。
- 主要新增：OpenAI Live HTTP/WS 入口与 attestation、Passkey 登录和用户凭据管理、上游 Model Plaza、面板 API 分层限流、分组 `allow_live` 字段及 4 个数据库迁移。
- 主要修复：Ollama Cloud 刷新节流、注册邮箱别名并发去重、设置部分更新、OpenAI reasoning 故障转移、WS 每轮模型归因、Responses 工具配对、Antigravity/Gemini/Grok 兼容、支付多币种统计和 Caddy SSE 压缩缓冲。
- Ent 通过临时 target 完整生成并与正式 `backend/ent` 逐文件 SHA-256 比对一致；临时目录已删除。Wire 重复生成结果稳定，`wire_gen.go` SHA-256 为 `DA4B0969FC7CC3061E653CA7D3E9F9DB511F86DDFED857AB2148038173A11539`。
- 上游 sponsor 更新删除 5 个不再引用的合作方 PNG；未删除本地业务资源。
- `go.sum` 使用官方依赖下载补齐 `github.com/google/subcommands` 校验项；前端测试、lint 和构建后没有锁文件或其他未暂存受控差异。

### 冲突与最终解决方案

- 29 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`，最终索引无未解决路径。
- OpenAI Live、Passkey、上游 Model Plaza 和面板限流完整接入；`/model-plaza` 默认关闭，本地 `/models` 模型市场和每日签到继续保留各自设置及前端入口。
- OpenAI REST/WS 同时接入首响应心跳、首 Token 检测、Responses 工具映射、reasoning 故障转移和每轮模型跟踪；本地逐轮并发释放、用量快照、请求体哈希、失败停调度、审计及每轮只映射一次的边界保持不变。
- `UpstreamModel` 只用于转发日志和账号成本归因；普通文本用户计费继续严格按请求模型，Composite 保留既有例外，图片和视频通过 `BillingModel` 按实际媒体模型计费。
- Claude CLI 版本更新到 `2.1.220`，并接入原始 system cache breakpoint；OAuth 模拟继续只信任 handler 严格判定，不接受 body 特征放宽客户端身份。
- Antigravity 接入非流式空响应与流式兼容修复，但同账号重试继续受 Pool Mode 约束；首响应心跳、失败切换和流内用量语义保持本地边界。
- 修复合并后的注册页未定义 `registrationActionDisabled`、Grok 测试遗漏 `prompt` 参数、语义错误默认值测试依赖全局缓存、Groups Live 能力旧 Mock 和 Profile Passkey 布局旧 Stub。
- 删除冲突解决后不再使用、且会绕过本地严格计费或客户端判定的 3 个 helper，`golangci-lint` 最终为 `0 issues`。
- `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 本轮未改动且保持完全一致，SHA-256 均为 `7091912E26F962DD1A37CAC5E16876E6489E59E4192DE0989DBCD3BE920B3D1E`；生产 bind mount、回环暴露、HTTP upstream 开关和 4 vCPU/8 GiB 参数基线均未改变。

### 刻意保留的二次开发功能

- 首 Token 超时、首响应前心跳边界、upstream/client stream 拆分、WS 逐轮并发释放/结算、成功会话审计与失败调度保护。
- 普通分组严格按用户请求模型计费、Composite 显式别名价例外、严格缺价错误、图片/视频实际媒体模型计费、按用户串行扣费和 5 秒 usage task 超时。
- Claude Code 严格客户端判定、全局上游模拟、Codex 图片工具策略、API Key 请求头覆写、内容审核/Cyber 阻断及渠道监控结构化 Responses `input`。
- 本地每日签到、`/models` 模型市场、充值赠送、专属倍率用户禁返利、图片分组成功率和批量图片结算。
- 账号列表禁用虚拟化、查询后滚动重置、DataTable 稳定性、二开管理入口和生产双 Compose 约束。

### 验证记录

验证使用 Go 1.26.5、corepack 管理的 pnpm、golangci-lint 2.9.0 和既有本地依赖缓存。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | 完整回归基线 | 未保留 | 执行上下文交接时已进入 merge 现场；未伪造同步前命令或退出码，`main` 与备份分支始终保持 `LOCAL_PRE_SYNC_SHA` |
| 生成 | Ent 临时 target 完整生成和逐文件 SHA-256 比对、`go generate ./cmd/server` 重复执行 | 0 | Ent 与正式目录完全一致；Wire 两次结果稳定，无临时目录或锁文件残留 |
| 适配 | `go test -tags=unit -run '^$' ./...` | 0 | Go 全包编译通过 |
| 适配 | OpenAI WS/Live、Gateway 用量、Antigravity、Passkey、设置、server/routes/middleware 定向测试 | 0 | 所有受影响后端定向测试通过 |
| 适配 | 前端冲突点 4 文件定向 Vitest | 0 | 40 个测试通过 |
| 同步后 | `go test -tags=unit ./...` | 1 / 0 | 首次发现语义错误默认值测试依赖全局缓存；显式注入空设置仓库并重置缓存后全包通过 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 1 / 0 | 首次发现 3 个冲突解决后的未使用 helper；按本地严格计费和客户端判定边界删除后 `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath -o bin/server-sync.exe ./cmd/server` | 0 | 后端构建通过，产物位于忽略目录 |
| 同步后 | `corepack pnpm lint:check`、`corepack pnpm typecheck` | 0 | 串行复验均通过；一次与 Vitest 并行的 lint 仅因扫描到 Vitest 瞬时配置文件失败，不属于代码错误 |
| 同步后 | `corepack pnpm test:run` | 1 / 1 / 0 | 两轮分别发现 Groups Live 能力 Mock 和 Profile Passkey 子组件 Stub 过期；修复测试隔离后全量 Vitest 通过 |
| 同步后 | `corepack pnpm build` | 0 | `vue-tsc -b` 与 Vite 生产构建通过；仅有既有 Browserslist、动态导入和大 chunk 非致命警告 |
| 同步后 | `git diff --cached --check`、冲突路径、未暂存文件和 Compose 一致性检查 | 0 | 无空白错误、未解决冲突或未暂存受控差异；两份生产 Compose 字节一致且未被本轮修改 |

### 未验证项与残余风险

- 本机 Docker 与 `govulncheck` 不可用，未运行 Testcontainers、真实 PostgreSQL migration、Compose 启动或 Go 漏洞可达性扫描。
- 未运行 `go test -tags=integration ./...` 或 `-race`；当前验证覆盖全量 unit、静态检查、构建和受影响定向用例。
- 未读取 `.env`，未启动依赖 PostgreSQL/Redis 的真实服务，因此本地启动和健康检查标记为未验证。
- 未使用真实 OpenAI Live attestation、Passkey 硬件、OpenAI、Anthropic、Grok、Ollama、支付、S3 或上游站点凭据；外部业务流程仅由本地单元、契约和前端测试覆盖。
- 未执行真实浏览器端到端交互；Passkey、Model Plaza、面板限流和 Live 管理路径由 API/组件测试、typecheck 和生产构建覆盖。
- 未执行 push、PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-07-12 同步至 e316ebf52

- 执行时间：2026-07-12T21:28:10+08:00
- 执行状态：同步分支验证成功；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`6c8bbc1273453495c9eeee82e0cd1ba447b379b5`
- 上游代码合并提交：`95b9cb9cfe7e770b295a575ecbe26b917a20b24f`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`6c588bb950dafa6db2b4413e896d93b4cb592944`
- `UPSTREAM_NEW_SHA`：`e316ebf52838a89d57fc790981cce7520f819ac8`
- merge-base：`6c588bb950dafa6db2b4413e896d93b4cb592944`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`e316ebf52838a89d57fc790981cce7520f819ac8`
- 集成策略：先将本地 `main` 显式 fast-forward 到 `origin/main`，再从该提交创建同步分支，并使用 `git merge --no-ff --no-commit` 完整合并固定的上游 SHA
- 备份分支：`backup/pre-upstream-sync-20260712-211629-6c8bbc127`
- 同步分支：`sync/upstream-20260712-e316ebf52`

### 上游提交处置

本次固定范围共 16 个提交，全部通过完整 merge 集成；无 `Already Applied`、`Skipped` 或 `Deferred`。

| 上游提交 | 状态 | 内容与处置 |
| --- | --- | --- |
| `75fb3c41c272163e02970d23df6c793f1519acf1` | Applied | Responses 到 Chat 桥支持 custom 工具 |
| `27e29f05621488b9402a373bbb434bda499645e7` | Applied | 增加 tool_search 降级与回程支持 |
| `18e26c127c03187bbedb33d18bb97421330541f3` | Applied | 合并对应工具兼容分支 |
| `79423383287e945a1d953a6f280bf35ea6b7f422` | Applied | namespace 子工具摊平与回程还原 |
| `f1082bb78f788e716c810103101b10b854d2f77d` | Applied | namespace 摊平名冲突时显式拒绝 |
| `0d28f7f90d80bfdbf9d44e3efe2ddbfc5a58f7e0` | Applied | Responses 与 Anthropic 转换保留 cache creation Token |
| `eb4d0050312f33eded6f28abe7cdb3f1731a6869` | Applied | 合并缓存 Token 与工具兼容分支 |
| `83f169e4fa815f7083de23e301d1a1560dc71ca8` | Applied | 流式 Responses 到 Anthropic 路径补齐 cache creation Token |
| `89a551b964076f2e61b71c0b8fa34f9464100cb0` | Applied | 防止 opsCaptureWriter 释放后访问 nil panic |
| `bc3cb290276922074213c5bc8ebc404bc6d083a8` | Applied | 补齐 opsCaptureWriter 委托方法的 nil 防护 |
| `a2cdaa6419e0ab2cb20b38ed64981c6ffd57046a` | Applied | 拒绝内置 tool_search 与同名工具冲突 |
| `e2b68d1f905005f394117643f4e1fed512d1ad3e` | Applied | 只转发实际存在的工具选择 |
| `90e9d03dec4dafc4e9bb354c7c48c1b4cc02c4ef` | Applied | 将强制 tool_search 选择降级为代理 function 选择 |
| `151b9265fca035ea68796b4fa3c3914ecd211455` | Applied | 合并 opsCaptureWriter nil 防护更新 |
| `07fac347137118cc05caa7eddeb0035cdb8066a3` | Applied | 合并 Anthropic usage 缓存 Token 更新 |
| `e316ebf52838a89d57fc790981cce7520f819ac8` | Applied | 合并 Codex MCP/custom/tool_search Chat fallback 更新 |

### 本地提交与文件

- 上游范围整体映射到本地 merge commit：`95b9cb9cfe7e770b295a575ecbe26b917a20b24f`
- 上游合并修改 13 个代码文件：新增 3 个测试文件，修改 10 个实现或测试文件，共增加 1874 行、删除 98 行。
- 主要更新：ops 响应捕获 writer 的生命周期防护、Responses/Anthropic cache creation Token 转换、custom/tool_search/namespace 工具在 Chat fallback 中的请求降级和响应还原。

### 冲突与用户决定

- 用户批准先以 `git merge --ff-only origin/main` 将本地目标更新到 `6c8bbc1273453495c9eeee82e0cd1ba447b379b5`，纳入 0.1.188 的账号连续失败停调度保护。
- 唯一文本冲突位于 `backend/internal/service/openai_gateway_responses_chat_fallback.go`。
- 最终方案同时保留本地首 Token 超时的 `wrapResponse`、`finish` 和请求发送签名，并向流式、非流式转换传递上游新增的 `customTools`、`toolSearch`、`namespaceTools`。
- `backend/internal/service/openai_gateway_messages_chat_fallback.go` 自动合并成功，并将 `ChatCompletionsResponseToResponses` 调整为新签名，同时保留首 Token 超时逻辑。
- 未出现计划外文本冲突或新的业务取舍。

### 刻意保留的二次开发功能

- 账号连续失败停调度保护与失败状态清理。
- 首 Token 超时、流式响应包装和失败调度兼容。
- Codex 图片工具禁用、namespace 图片工具剥离及相关策略优先级。
- 按用户请求模型计费、网关配置、渠道与 Image 分组成功率监控。
- 签到、模型广场、充值返利展示和二次开发功能配置入口。

### 验证记录

验证使用 Go 1.26.5、Node 24 和通过 `corepack pnpm` 固定的 pnpm 9.15.9。首次直接调用系统 `pnpm` 命中了 11.7.0，冻结安装因 overrides 与锁文件配置不匹配而拒绝；改用 `corepack pnpm` 后冻结安装成功，工作树未被该失败修改。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./...` | 0 | 通过 |
| 同步前 | `go test -tags=integration ./...` | 1 | 仅 `internal/pkg/tlsfingerprint` 的 3 个外部联网用例因 `tls.peet.ws:443` 拒绝连接失败，其余通过 |
| 同步前 | `golangci-lint run ./...` | 0 | 0 issues |
| 同步前 | `go build -o <临时目录>/sub2api-baseline-server.exe ./cmd/server` | 0 | 通过，临时产物已删除 |
| 同步前 | `corepack pnpm --dir frontend run lint:check` | 0 | 通过 |
| 同步前 | `corepack pnpm --dir frontend run typecheck` | 0 | 通过 |
| 同步前 | CI 关键 Vitest 集合 | 0 | 6 个文件、97 个测试通过 |
| 同步前 | `corepack pnpm --dir frontend run build` | 0 | 通过，存在既有 chunk 与动态导入警告 |
| 同步后 | apicompat、handler、service 定向回归测试 | 0 | custom/tool_search/namespace/cache creation、opsCaptureWriter、首 Token/失败调度/计费/图片策略相关测试通过 |
| 同步后 | `go test -tags=unit ./...` | 0 | 通过 |
| 同步后 | `go test -tags=integration ./...` | 1 | 与同步前完全相同，仅 3 个外部联网用例失败，无新增失败 |
| 同步后 | `golangci-lint run ./...` | 0 | 0 issues |
| 同步后 | `go build -o <临时目录>/sub2api-post-sync-server.exe ./cmd/server` | 0 | 通过，临时产物已删除 |
| 同步后 | `corepack pnpm --dir frontend run lint:check` | 0 | 通过 |
| 同步后 | `corepack pnpm --dir frontend run typecheck` | 0 | 通过 |
| 同步后 | CI 关键 Vitest 集合 | 0 | 6 个文件、97 个测试通过 |
| 同步后 | `corepack pnpm --dir frontend run build` | 0 | 通过，警告与同步前一致 |
| 同步后 | `git diff --check`、冲突标记、意外删除、敏感路径检查 | 0 | 通过 |

### 未验证项与残余风险

- 本机没有 Node 20 环境，未验证与 CI Node 20 完全等价；本地 Node 24 验证已通过。
- 未读取 `.env`，也没有安全的隔离运行配置，因此未执行本地服务启动和健康检查。
- TLS 指纹集成测试依赖外部站点，本次环境无法访问；该失败在同步前后保持一致。
- 未执行 push、PR、部署、远程服务器访问或生产数据操作。

## 2026-07-14 同步至 da85cc7e4

- 执行时间：2026-07-14T23:31:44+08:00
- 执行状态：同步分支完整合并并更新本地 `main`；首轮远端 CI 暴露批量图片插入占位符冲突，修复后第二轮 CI 与 Security Scan 全部通过
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`ce7ff703925415b61855d5d3b67fcee413fc5e87`
- 上游代码合并提交：`1774fb96e15e69a13956580c15318cc24ac624a0`
- 最后一个代码提交：`9386396d30814e928b68488c2e643ec6e35c3656`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`7d239d62e8f1c6aea79164f88903f4158cbf2f98`
- `UPSTREAM_NEW_SHA`：`da85cc7e47882090b115d664afe8e39b37aa7417`
- merge-base：`7d239d62e8f1c6aea79164f88903f4158cbf2f98`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`da85cc7e47882090b115d664afe8e39b37aa7417`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决文本和语义冲突，重新生成 Ent/Wire，再提交二次开发兼容调整
- 备份分支：`backup/pre-upstream-sync-20260714-230039-ce7ff7039`
- 同步分支：`sync/upstream-20260714-da85cc7e4`

### 上游提交处置

本次固定范围共 68 个提交，其中 26 个 merge commit、42 个 non-merge commit。66 个为 `Applied`，2 个为 `Applied + Overridden`；无 `Already Applied`、`Skipped` 或 `Deferred`。

| 上游提交 | 状态 | 内容与处置 |
| --- | --- | --- |
| `b6bb74b6fa83c3fb16357d3eafdad68182d14c97` | Applied | 防止重复注入 Codex 图片工具 |
| `0dce07ee8b189c1c8ce9f90e636c50e0c638a170` | Applied | API Key 上游支持代理 Codex 模型清单 |
| `92dcfb5ebcf18efe2b360cca547b1aaab76a0c51` | Applied | 按账号控制 OpenAI 长上下文计费并记录 usage 快照 |
| `139f79b85afc7444a98a0df52e3bc550d420f5d0` | Applied | 补齐长上下文计费 API 契约测试 |
| `0d9c140bc22a808bf201eb796f3fae149714cd8e` | Applied | 合并长上下文计费功能分支的上游基线 |
| `54a8606e2afec3c8c5e67a2ebc68c56a79c9aca3` | Applied | 原生 Responses namespace 兼容设计文档 |
| `1d86c1bf81592920353850091720940de0f36842` | Applied | 原生 Responses namespace 实施计划 |
| `317de9c04b610eb853080991c2bd8f4574db8d2f` | Applied | 原生 Responses 支持 namespace 工具摊平与回程恢复 |
| `8d5bc448b5b966f17e6511f1cb31d5a7967411c3` | Applied | 避免重复扫描 OpenAI 请求体 |
| `a0ac5e024041d21f527a345d4a20bf22168c59d3` | Applied | 完成长上下文计费开关在创建、导入和同步路径的接线 |
| `40ec74b9fc84f82afba64949433445d27dc00ce3` | Applied | 保留 Messages 分发的精确模型映射 |
| `3e4d48e01082be4cca86a1bc565c59c39ddeb03b` | Applied | 合并长上下文计费分支的后续上游基线 |
| `f63d168ae0bbae3ddd8aea1f2eadb5d883ec4ed1` | Applied | 校验长上下文开关必须为布尔值 |
| `ed31a52424ff996d9210fbad1644cd9a1a7c8698` | Applied | 稳定 API Key Codex 清单刷新 |
| `c896cacf6d093adc39bdb18c7d3ec8d3cdae5f44` | Applied | 改进 Grok 免费额度探测与用量展示 |
| `3c68b2e3693272e6066cf2cb072e3fb6b41a02aa` | Applied | Codex 清单刷新支持账号故障转移 |
| `a0778e9a42bcce7948922c1f74cf839a7ae331f3` | Applied | 调度延迟改为读取未消费 outbox 事件 |
| `831862b9240b1d56f27f478951bc0fdb90f1b5e2` | Applied | 合并并发全量调度重建请求 |
| `98027cdded50997c416ce7aa389e35993c410968` | Applied | OpenAI HTTP/2 连接启用 keepalive PING |
| `e9fb5983cd0744eaff5cd4486118d7ff190a0a60` | Applied + Overridden | 新账号继续默认关闭长上下文计费；历史缺失字段账号回填为开启，保持同步前有效计费行为 |
| `ad4bf5c60d06e2a75a222dfedfa63cc8459a3c72` | Applied | Grok Web SSO 批量导入并转换 Build OAuth |
| `54d228dda5d2616dbd4b590d8af50c1f81ec5b11` | Applied | 增加默认关闭的管理端 Server-Timing 指标 |
| `966afd1b4b1c0bffea4988f6823690709816e473` | Applied | 保留被监测 HTTP client 的接口契约 |
| `2c2e50ba589ed828a001b5e34295602391c8c663` | Applied | 系统日志增加主机字段、筛选和索引 |
| `0f2ec134b5eb8bcaa67a06aa920b014d18a4e309` | Applied | 限制日志主机索引字段长度 |
| `6c441637b048916058697eeb5efe688510338ff7` | Applied | 移除账号类型页重复的 Grok SSO 卡片入口 |
| `f2ca16577e4ac71d03372347b573fa53fef45543` | Applied | 补齐流式图片生成最终结果状态 |
| `002c0b9fda475344c44037275cbc07783f98f793` | Applied | 非流式 Images 请求支持可选 JSON keepalive |
| `74e78c3de0784747fe079d448f088a61a01ff29f` | Applied | 合并 OpenAI 长上下文计费功能 PR |
| `8f328d4ab3b6bd97bb83d43b2fcb9463044c9716` | Applied | 代理到期改投使用定向调度事件 |
| `9033e14bb7570a01ce12cd08d767683cd89078db` | Applied | 账号到期暂停使用定向调度事件 |
| `8cd848313c92edc3ea8ecc913e2fcfd77924a81e` | Applied | 改进 OpenAI reset credit quota 识别 |
| `029e5ce9f925eaefcda67bc86b912f50939f5e11` | Applied | 合并 Codex 图片工具重复注入修复 PR |
| `7358810659f11ab4f5a01fdf985c77d158c533f1` | Applied | 合并请求体单次扫描优化 PR |
| `1847bdf9fd42e40639660b0df7fc7f04731c2234` | Applied | 合并 Messages 精确模型映射修复 PR |
| `41c71a1528b3eaa9673d11b195e92a4030c0d95d` | Applied | 合并 Ops 日志主机筛选 PR |
| `623a9647c07fd172593574de1471078024d20f12` | Applied | 合并 API Key Codex 模型代理 PR |
| `a8927d8ec7684782c1eac83c7b2dca5cd887b171` | Applied | 合并 Server-Timing 指标 PR |
| `93f2ccf3a5fddc171129237ce07902692586c68e` | Applied | 合并 Grok 免费额度探测 PR |
| `d41a10111dd5347bbf57bd1cc94ed4bfd7a7cfeb` | Applied | Grok SSO 功能分支同步上游主线 |
| `5d1c577cb2c735ca1f1d57533dff1302f6998d91` | Applied | 合并 Grok SSO 设备授权 PR |
| `30d4301bea25a5367d161a0d2e9ac927fa688728` | Applied | Grok 免费额度改用滚动 24 小时估算 |
| `27fcbace8945cd8cc474e61a1a4c3e3fa55d9649` | Applied | 合并 HTTP/2 keepalive PR |
| `87118829186aadabd3ca08fae953b5df53df5c25` | Applied | 合并账号自动暂停调度事件 PR |
| `9c3c560d4958e26445107b6877308898472fa357` | Applied | 合并代理到期调度事件 PR |
| `24d908b257f4ad593cdfd5622052a818c10df5b8` | Applied | 合并调度 outbox 延迟修复 PR |
| `2590b86e3164e577e847e35a8a17e0ca25964a0d` | Applied | 合并调度重建并发合并 PR |
| `97176993677e78efe7b4d31e4506ac87b54bed2a` | Applied | 合并流式图片最终状态修复 PR |
| `527279c95312010009d41835ff66680e6bb0b2db` | Applied | 合并额度重置识别修复 PR |
| `ac7a141a2475d85d2824cff7cf027cb78924fe8e` | Applied | Images keepalive 写入大小用于 OAuth 响应快照 |
| `c361b0606dee7d8de78145c64e97369b2f48910f` | Applied | 合并非流式 Images keepalive PR |
| `69bc6a87dde89e79ba39436467ec46dee6a6b234` | Applied | 合并 Grok 滚动 24 小时额度 PR |
| `a1b5c75ca334c972a6bc62ef99baf35ab1eee716` | Applied | 新导入 Grok OAuth 账号自动探测额度 |
| `d8a07e91a5945882a18de104d389ab23460c0b11` | Applied | 稳定 Grok prompt cache 路由 identity |
| `0a64a6d8ceba7b0429e2efa1c1e8b23162d30011` | Applied | 渠道健康监控支持 Grok provider |
| `d2d3fcf57ba5647d23817c077a49b9d4b3132217` | Applied | 前端展示 Grok 监控与 Free 标识 |
| `16d1fbfd4e2ed219c607afc9a9ac8d0c0ac32c05` | Applied | 探测调度快照保持为测试内状态 |
| `ff639ba757cd28126adea4281550d993fd22f032` | Applied | 清除 Grok reasoning 项的空 content |
| `2f715baf054ba040b4c75c5e82657c1aab24d540` | Applied | Responses Lite 保留客户端图片工具 |
| `2e9b8d9a648ce9c37ab1c89df3db8cad8ef85eea` | Applied | 修复 reasoning 测试的 lint 和格式问题 |
| `03646e943404bf025d4d41cb75f6d953111f52f5` | Applied | 合并 Grok reasoning 空 content 修复 PR |
| `11ed22d052415340dabe4d0be295ebfd3256add0` | Applied | 合并 Responses Lite 图片展示修复 PR |
| `53004e2e90bf061bc92c1189f5b71b383991649b` | Applied | 合并 Grok 监控自动探测 PR |
| `252ef8b73a668d06b74c8c8be4646fed57cac3f5` | Applied | namespace 功能分支同步上游主线 |
| `fa1641f05f1607276b867e20194e12ad5499f4ef` | Applied | WSv2 转发保持 namespace 原样并修复测试断言 |
| `41cec0db059ffb82d0efdcfcf07a24ab51fbfe97` | Applied | 合并原生 Responses namespace PR |
| `7c717365ef728e53cdcf6d639a4dd68226db03b2` | Applied + Overridden | 上游版本更新至 0.1.155；保留本地较新版本 0.1.193 |
| `da85cc7e47882090b115d664afe8e39b37aa7417` | Applied | 更新赞助商文档与图片 |

### 本地提交与文件

- 上游范围整体映射到本地 merge commit：`1774fb96e15e69a13956580c15318cc24ac624a0`。
- 二次开发兼容提交：`9386396d30814e928b68488c2e643ec6e35c3656`。
- 同步记录提交前，代码与配置相对 `LOCAL_PRE_SYNC_SHA` 修改 237 个文件：新增 54 个、修改 183 个、删除 0 个，共增加 14792 行、删除 874 行。
- 主要更新：OpenAI 长上下文计费开关和 usage 快照、Codex namespace 与 Responses Lite 图片工具、Images 非流式 keepalive 和流式结果修复、API Key Codex 模型清单、HTTP/2 PING、Grok SSO/额度/监控、Server-Timing、Ops 日志主机筛选、调度 outbox 与重建优化、额度重置识别及赞助商文档。
- 重新生成 Ent/Wire；生成结果与 schema 和 provider 源一致，未保留 Wire 工具自身写入的无关 `go.sum` 校验和。

### 冲突与最终解决方案

- 15 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`。
- `backend/cmd/server/VERSION` 保留本地 `0.1.193`。
- HTTP upstream 在 `servertiming.Do` 前继续启动本地首 Token 计时，并纳入上游 HTTP/2 PING。
- Images handler 使用排除 keepalive 的有效写入大小判断是否可以 failover，同时保留本地 outcome 失败结算。
- Responses 流先规范图片完成状态，再执行本地语义错误检测，并继续完成 namespace 回程恢复。
- 原生 namespace 根据本地 HTTP 流式转 WSv2 的实际决策决定是否摊平；WSv2 保持原样，`image_gen` 继续受本地 Codex 图片工具策略控制。
- 渠道监控保留本地结构化 Responses `input`，同时引入 Grok adapter 与 Server-Timing wrapper。
- 长上下文计费保留本地“仅实际命中区间时禁用内置倍率”规则，并叠加账号开关；历史缺失开关的 OpenAI 主账号回填为开启，新账号默认关闭。
- Grok SSO、长上下文开关与本地 Codex CLI 控件在账号创建弹窗中并存。
- `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 同步增加默认关闭的 `ENABLE_SERVER_TIMING`，最终内容保持完全一致；生产 bind mount、localhost 暴露和安全开关未改变。
- 上游新增 Images 测试适配本地带账号参数的响应处理签名；除此之外没有计划外业务取舍。

### 刻意保留的二次开发功能

- 账号连续失败停调度、strict 调度、pending/final outcome 与 streak 清理。
- 首 Token 超时、body-signal compact、paused keepalive、WS lease 和流内错误结算。
- 按请求模型计费、部分区间价格回退、图片分组成功率、视频价格、充值返利、签到和模型广场。
- Codex 图片工具策略、Claude 上游模拟、渠道/分组扩展配置和 API 契约。
- Responses 渠道监控的结构化 `input` 请求格式。
- 账号列表禁用虚拟化及查询上下文滚动重置。
- 生产 bind-mounted 数据目录、localhost 暴露、HTTP upstream 安全开关和双 Compose 一致性约束。

### 验证记录

验证实际使用 Go 自动工具链 1.26.5、Node 24.15.0 和 pnpm 9.15.9。本机无 Docker、Node 20 或安全隔离的应用启动配置。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./...` | 1 | 仅 `TestProcessGeminiStream_SemanticErrorFails` 在全量并行运行中偶发未命中断言；随后定向重跑退出码 0 |
| 同步前 | `go test -tags=integration ./...` | 1 | 仅 `internal/pkg/tlsfingerprint` 的 3 个外部联网用例因 `tls.peet.ws:443` 拒绝连接失败，其余通过 |
| 同步前 | `golangci-lint run ./...` | 0 | `0 issues` |
| 同步前 | `go build -o <系统临时文件> ./cmd/server` | 0 | 构建通过，临时产物已删除 |
| 同步前 | 前端 lint、typecheck | 0 | 均通过 |
| 同步前 | `corepack pnpm --dir frontend run test:run` | 0 | 163 个文件、1035 个测试通过 |
| 同步前 | `corepack pnpm --dir frontend run build` | 0 | 通过，存在既有 Browserslist、动态导入和大 chunk 警告 |
| 生成 | `go generate ./ent`、`go generate ./cmd/server` | 0 | Ent/Wire 生成成功 |
| 适配 | `go test ./... -run '^$'` | 1/0 | 首次发现上游新增 Images 测试缺少本地账号参数；适配后全量编译检查通过 |
| 适配 | 长上下文计费与 migration 定向测试 | 0 | 历史回填、新账号默认关闭、区间回退和账号 opt-out 组合通过 |
| 同步后 | `go test -tags=unit ./...` | 0 | 全部通过，包含同步前偶发失败用例 |
| 同步后 | `go test -tags=integration ./...` | 1 | 与同步前完全相同，仅 3 个 `tls.peet.ws` 外网用例失败，无新增失败 |
| 同步后 | `golangci-lint run ./...` | 0 | `0 issues` |
| 同步后 | `go build -o <系统临时文件> ./cmd/server` | 0 | 构建通过，临时产物已删除 |
| 同步后 | `corepack pnpm --dir frontend run typecheck` | 0 | 通过 |
| 同步后 | `corepack pnpm --dir frontend run test:run` | 0 | 170 个文件、1093 个测试全部通过 |
| 同步后 | `corepack pnpm --dir frontend run build` | 0 | 通过，非致命警告与同步前同类 |
| 同步后 | 前端 lint 与 Vitest 并行执行 | 1 | ESLint 扫描到 Vitest 已删除的瞬时时间戳文件；所有前端进程结束后单独重跑通过 |
| 同步后 | `corepack pnpm --dir frontend run lint:check`（独立重跑） | 0 | 通过，瞬时时间戳文件数量为 0 |
| 同步后 | Compose 哈希、`git diff --check` 与冲突标记检查 | 0 | 两个生产 Compose 完全一致，无冲突标记或空白错误 |

### 未验证项与残余风险

- 本机没有 Docker，依赖 testcontainers 的集成路径无法等价覆盖；迁移静态测试和不依赖 Docker 的集成用例已通过。
- 本机没有 Node 20，未验证与 CI Node 20 完全等价；Node 24 验证已通过。
- 未读取 `.env`，也没有安全隔离的 PostgreSQL/Redis 配置，因此未执行本地服务启动、健康检查或真实数据库迁移。
- 未使用真实 Grok SSO、quota、Codex/OpenAI 上游凭据验证外部业务流程；相关请求转换、handler、service 和前端用例已通过本地测试。
- 未验证生产环境应用迁移；本次未访问服务器、修改生产文件、重启容器或操作生产数据。
- 未执行 push、PR 或部署。

## 2026-07-14 同步至 7d239d62e

- 执行时间：2026-07-14T01:25:25+08:00
- 执行状态：同步分支完整合并并验证完成；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`3d6aeed837b25bdf291634a817f8af6843ea05e1`
- 上游代码合并提交：`1bd656838c68b6e688a230e7c158bde6497e3dd0`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`e316ebf52838a89d57fc790981cce7520f819ac8`
- `UPSTREAM_NEW_SHA`：`7d239d62e8f1c6aea79164f88903f4158cbf2f98`
- merge-base：`e316ebf52838a89d57fc790981cce7520f819ac8`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`7d239d62e8f1c6aea79164f88903f4158cbf2f98`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决文本和语义冲突，重新生成 Ent/Wire 后创建 merge commit
- 备份分支：`backup/pre-upstream-sync-20260714-004333-3d6aeed83`
- 同步分支：`sync/upstream-20260714-7d239d62e`

### 上游提交处置

本次固定范围共 85 个提交，其中 35 个 merge commit、50 个 non-merge commit；全部通过完整 merge 集成，无 `Already Applied`、`Skipped` 或 `Deferred`。

| 上游提交 | 状态 | 内容与处置 |
| --- | --- | --- |
| `0464856c4aa5deb613dabff662f6ca6bf98fba13` | Applied | Fast/Flex 策略支持搜索选择用户 |
| `4d4ba64bf7ba110241e0850bee2dd4180a6b3f49` | Applied | 剥离续链 message item 的非法 `item_*` ID |
| `6e2bb312812b214751e7602cf48271ab9efefbcb` | Applied | 防护 compact keepalive writer 委托方法 |
| `84bb7d070974dc9ee12dcca3d263a87cb4a58430` | Applied | 保留 `remote_compaction_v2` 原生 Responses 链路 |
| `51de58b37f662a758dfee4f9cc5aa08c79b88ece` | Applied | 规范 OpenAI OAuth 测试中的 GPT-5.6 别名 |
| `94a22b62f7b963b6b671e3fc292ad0af609e6143` | Applied + Overridden | DataTable 阈值虚拟化和主键行高缓存已集成；账号列表继续按本地策略禁用虚拟化 |
| `80b7a8d4cbc8e17676fa9ac9d751bc39eacd75fe` | Applied | 限定每个 API Key 的最新 IP 查询范围 |
| `1c02158c2a7fcdf97540ccc719df5323667146af` | Applied | 增加最新 API Key IP 查询索引 |
| `c56a64fabdd0bb29416a47029a8fb3ac798b1f82` | Applied | 账号编辑支持手动覆盖 OpenAI OAuth `plan_type` |
| `0478fd36683dbf86e30dbfef0f618a012f7c1daf` | Applied | Grok OAuth 免费账号支持 prompt cache |
| `52071d391b5b2a4e4e0940aea85fc731857c6d07` | Applied | 转发 Codex alpha/search 独立搜索端点 |
| `1dedb2097dcf50845f5169f7ff25425a3857f187` | Applied | 将 Grok quota exhaustion 持久化为限流状态 |
| `d5b47c21429e405c4142c61c5d37620b09a67d4d` | Applied | 恢复 OAuth Messages 的 Codex identity |
| `5015b7a1c174583ce4b31b0deee85f576850146a` | Applied | 修复 `tool_search` 参数对象反序列化 |
| `06af8115f7fda82c70075a675bb581a25c3ed4d7` | Applied | 修复 compact 心跳 writer 生命周期 |
| `1a8401f5f320c4e11fd643f86565f62ac688a7c8` | Applied | 合并 `tool_search` 参数修复 |
| `f57d06d959c25317889d522f0527ac034eae3933` | Applied | 合并 compact writer 修复 |
| `73ffd134301190ffd27c6b6ab5749a21d87be0df` | Applied | 合并上游 issue #3818/#3887/#3961 关联修复 |
| `fe184f8c33e4bc2bccf82e6d15051041edd5c153` | Applied | 修复调度缓存异常时间阻塞 |
| `ff5c21618932328f33b12b83eafc62aff25f1464` | Applied | Chat bridge 支持 Codex additional tools |
| `7050070aa38d88cf71f26990f4ab1732963a5fd2` | Applied | Grok 可缓存 Chat 请求改走 Responses |
| `05865d9b655c30efa19188981f89e5122f5d9d2c` | Applied | 合并 Codex message item ID 修复 PR |
| `d734dbdac44df1a3944ce2cc282245c4ebcf5eb8` | Applied | 合并 OpenAI Fast Policy 用户范围 PR |
| `877bee84a18a37bd7b8b343bf6028a2b851d001a` | Applied | 合并上游复审修复 PR |
| `2f4478fd32a1fc6dc227bb575f95fe375d91a41a` | Applied | 合并 compact keepalive writer nil 防护 PR |
| `841481e051ed7ee8a1c869c2c7d6677df3f39eaa` | Applied | 合并 remote compaction v2 PR |
| `1c214eaca4f473586014eeb1108fa820efdf0b9e` | Applied | 合并 OpenAI Messages Codex identity PR |
| `33b1d772f734d70470269d5696fa2c2e2bd3d884` | Applied | 合并 Codex alpha search PR |
| `8d51364c3dda3085aa1d16b4b522f4e8f416aed6` | Applied | 合并 prompt cache 功能分支中的上游主线 |
| `42f3c22830b8b15650b12faeb38bbadb1641e6b1` | Applied | 合并 Grok prompt cache identity PR |
| `7cbb36f278f50f95e26ca737824d41adf2a8410a` | Applied | Codex alpha/search 网页搜索按次计费 |
| `038b25c0b1cc4c99f4486490f29b5f4d8ed88d76` | Applied | 修复近期 Grok 集成问题 |
| `ad18ee7c4f7d49e38f90b61b59365173f0d47d35` | Applied | OpenCode 使用 Responses adapter |
| `d9e466ad3a65c58d988a574a189a71f1b12e9069` | Applied | Grok 支持 xAI API Key 账号 |
| `3375b4ed2b7d6ac01ce59f0201516e57481eb8b6` | Applied | Grok OAuth subscription 经 CLI proxy 转发 |
| `cbddb57dec088b758728d9c9ff43dbe7c44040d7` | Applied | 展示 Grok 剩余 quota capacity |
| `f187f08ae366a52b2f95d6317e542f9a87fd1559` | Applied | 加固 Grok OAuth 路由和 CLI 版本校验 |
| `ce3f12bbffbf6d4423c6b3f419b2d52726e8c28b` | Applied | 覆盖 transport 边界的 Grok CLI identity |
| `c4ff604e9327c2a06c9b3a5c9549a2128cd06c0d` | Applied | 覆盖 Grok OAuth Chat permission identity |
| `aeb34d2003e3db0ba7126a5878539fa0979786b9` | Applied | 清理 Grok composer reasoning 参数 |
| `8a22dc7347d383b0b8fe3e510dfa246ee721dac2` | Applied | 按平台诊断 Grok 不可用模型 |
| `64a2a31729537c76d628da854c3556b9c2311756` | Applied | 修复 alpha search 按次计费复审问题 |
| `f73031f4362e914058997ee5badf4a1f861aa019` | Applied | 对齐 Grok 调度原因测试 |
| `e5af699d0f6926408e71f7f43164889e3aa0f919` | Applied | API 契约补充 `web_search_price_per_call` |
| `0d318195bf466f041e81a7fe536df69d944b8f0b` | Applied | 合并 alpha search 按次计费 PR |
| `b73d8c3efe01a290eaaa9326b6e40ece02c67a0e` | Applied | 合并近期 Grok 问题修复 PR |
| `83c10133d1615b2e3b71a8e173b5f466f7928de7` | Applied | 增加 Apple container 部署支持 |
| `909b96edd24fc5ee9be1d56a08a51adde2bfe2fa` | Applied | 支持 Grok 视频编辑与扩展 |
| `a1930ea6f29fc5f17ae0020f4e2d38e789c49d73` | Applied + Overridden | 上游版本同步至 0.1.152；最终保留本地较新版本 0.1.191 |
| `1e97e4cee4daccb9af4018aacb5c1a13b4d7fb58` | Applied | 嵌入式静态资源设置长效 Cache-Control |
| `3605a316af6872452ac4f08d484003179a57ad35` | Applied | API 与 dashboard 使用一致的 usage 时间范围 |
| `b0441ca5aafe98f99b6715fa0e5fe31769cc3efe` | Applied | API Key 支持 Grok 上游模型同步 |
| `a5d40c9845b06519c62c5d1518beac0fa3f58353` | Applied | Read 工具参数按流实时发送 |
| `a7ddca8930f41fedcaa6b17848757079edd71147` | Applied | 补齐中文 overview 与 misc 文案 |
| `b0fa2b352f95d470a7a40d0b73e396061e6372ea` | Applied | alpha search 绕过前端静态路由 |
| `b6427d4ec067ff06fbb4b46e543ebd7e8ab2dbd1` | Applied | 对齐 Anthropic 流结束原因和 content filter |
| `c7c933776db3847f60fa65945f389d82071ff5d9` | Applied + Overridden | 账号级池模式重试应用到多条转发路径，同时保留本地 strict 调度和 pending/final outcome 逻辑 |
| `50e5372fed019337297e010ff7a8920a4ea8b1fa` | Applied | 合并 Grok 上游模型同步 PR |
| `d8fa425a275effb97199bcaff8b6b31595d3cf28` | Applied | 合并流式停止原因与 content filter 修复 PR |
| `90bff0ea17674e149df07e2c9659cf70c0de94ba` | Applied | 合并 API Key 最新 IP 查询性能 PR |
| `daf0b99dcbd7561223e5b04aeabbcfd67635543c` | Applied | 合并静态资源缓存头 PR |
| `98cc6410085034a317ba63a6a72b347b3f0cfdea` | Applied | 合并 usage 时间范围与本地日期修复 PR |
| `fbc3f42a22291e7ac878e417ed9c09ae9d9efc7f` | Applied | 合并 Read 工具参数流式修复 PR |
| `b8dcae3bcf40b4e41860d6fcf5fb76aca3ec110f` | Applied | 合并中文 i18n 缺失项修复 PR |
| `a60a282473d1ca48f61e11b91578c6ea1a6af2f6` | Applied | 合并 alpha search 前端绕过 PR |
| `baab0adf7cd1cd2a5a6579f6e415df4f91209c35` | Applied | 合并调度缓存异常时间修复 PR |
| `d774948e09fdb3f2b2e309c94443dc21aa59e8b9` | Applied | 合并 Codex additional tools bridge PR |
| `fc9b4891060f0bf94d3621cde834d63cca8f6919` | Applied | 合并 Apple container 支持 PR |
| `8315defe8e8f3cecc5979dd2e7665647350f97ad` | Applied | 视频编辑功能分支同步上游主线 |
| `551e2570dd5e069e21cb5c9c1bb7ef092f5de5df` | Applied | 合并 Grok 视频编辑与扩展 PR |
| `03ccb2a08e8953eb5166be627897f6a422577b0e` | Applied | 删除泄露内部 AI 渠道配置的废弃支付接口 |
| `bc5d6ecb464e378b09e514dac863fec08f1b929d` | Applied | Grok 支持第三方 API base URL |
| `b0d0de05470df13fbfd1f9051a84635bd562c9e7` | Applied | 合并 GPT-5.6 OAuth 测试修复 PR |
| `0465540195825268588050cfe4939ecbebe35c87` | Applied | 合并 DataTable 滚动抖动修复 PR |
| `b4aa3eb02308642fa4199ef16e0f3b57755c9333` | Applied | 合并池模式账号级 retry count PR |
| `c8cfc936326fd98da046cfc74123fb1bb8985385` | Applied | 限定 OpenAI WS ingress session 生命周期 |
| `664b7be30f3b317762b8201c1ef41dc6cfd913f4` | Applied | 合并账号 `plan_type` 编辑 PR |
| `4bc7486c3b4cf0a0c4b4b551bdb3f5cb5f825ad2` | Applied | 合并删除废弃 payment channels 端点 PR |
| `540e90ca8b1220e95393a4fac6f7e23c6683e76e` | Applied | 合并 Grok 第三方 API 修复 PR |
| `a2bc1337474b68b62391116835e5698ebb5526bd` | Applied | 合并 OpenAI WS ingress 生命周期修复 PR |
| `5aeb03018c1defc8d46e108a4a72fcc2b72ff4fe` | Applied | 按账号冷却 Codex plan-gated 模型 |
| `55ed0ab0da367183d97c15659e33ae9e83f6ff90` | Applied + Overridden | 上游版本同步至 0.1.153；最终保留本地较新版本 0.1.191 |
| `bb734167337d4322f7da8bd0b768dc00e39ce127` | Applied | Grok OAuth 媒体改走官方 API |
| `adb5106c1f383fa0d382b200a9c750d1c66a04ff` | Applied | 合并 OpenAI OAuth 模型能力冷却 PR |
| `7d239d62e8f1c6aea79164f88903f4158cbf2f98` | Applied | 合并 Grok OAuth 媒体路由修复 PR |

### 本地提交与文件

- 上游范围整体映射到本地 merge commit：`1bd656838c68b6e688a230e7c158bde6497e3dd0`。
- merge commit 修改 201 个文件，其中新增 28 个、修改 173 个，共增加 10941 行、删除 661 行，无二进制文件和删除文件。
- 主要更新：OpenAI/Codex alpha search 与按次计费、identity/compact/tool bridge/WS 生命周期；Grok prompt cache、OAuth/API Key、模型同步、视频编辑及媒体路由；API Key 最新 IP 索引；Fast/Flex 用户范围、账号订阅档位和 DataTable；usage 时间范围与分页；Anthropic/Responses 流式兼容；调度缓存、账号级重试与模型冷却；静态资源缓存；Apple container 部署；支付废弃接口移除；i18n、README 与 CI。
- 重新生成 Ent 与 Wire，修复自动合并造成的 group 字段索引错位，并恢复本地 `stream_enabled`、`claude_code_upstream_mimicry`、支付返利字段的生成代码。

### 冲突与用户决定

- 用户批准完整 merge，保留本地 strict 调度、pending/final outcome、连续失败停调度、请求模型计费、首 Token/body-signal/paused keepalive、WS lease、账号列表禁用虚拟化以及生产 bind mount/Compose 约束。
- 25 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`。
- `VERSION` 保留本地 `0.1.191`，未回退到上游 `0.1.153`。
- 池模式重试在 Anthropic、Gemini、通用及 OpenAI 转发路径使用账号配置，同时保留本地 strict 调度和结果结算语义。
- Grok body-signal compact 分离 `upstreamStream` 与 `clientStream`：非流式上游仍保留下游 SSE、首 Token watchdog、暂停心跳、`FirstTokenMs` 和正确的 `Stream` 记录。
- compact keepalive 同时保留本地 paused 状态与上游 writer 恢复，避免请求结束后持有池化 writer。
- 支付端点删除 `/payment/channels`，保留公开定价、结算配置和充值返利；相关本地测试适配新的两参数构造器。
- API 契约同时保留本地视频价格字段和新增 `web_search_price_per_call`。
- 新增 Alpha Search、视频编辑/扩展路由；图片分组成功率白名单同步纳入视频编辑/扩展端点。
- DataTable 完整吸收阈值和主键行高缓存；账号列表继续 `virtualized=false` 并保留筛选/分页滚动重置。
- 部署文档与 `.env.example` 同时保留本地镜像/远程部署项和上游 Apple container 配置；未修改生产 Compose 文件。

### 刻意保留的二次开发功能

- 账号连续失败停调度、strict 调度、pending/final outcome 与 streak 清理。
- 首 Token 超时、body-signal compact、paused keepalive、WS lease 和流内错误结算。
- 按用户请求模型计费、图片分组成功率监控、视频价格、充值返利、签到和模型广场。
- Codex 图片工具策略、Claude 上游模拟、渠道/分组扩展配置和 API 契约。
- 账号列表禁用虚拟化及查询上下文滚动重置。
- 生产 bind-mounted 数据目录、localhost 暴露和 HTTP upstream 安全开关约束；本次未访问或修改生产环境。

### 验证记录

本机环境为 Go 1.26.5、Node 24.15.0、pnpm 9.15.9；没有 Node 20、macOS Apple container 或安全的本地运行配置。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | 完整回归基线 | 未执行 | 执行上下文交接时已经处于 merge 现场，未伪造同步前退出码；`main` 与备份分支始终保持 `LOCAL_PRE_SYNC_SHA` |
| 生成 | `go generate ./ent`（默认 Go proxy） | 1 | `proxy.golang.org` IPv6 连接失败，未生成代码 |
| 生成 | `GOPROXY=https://goproxy.cn,direct go generate ./ent` | 0 | 成功；修复 Ent 字段索引并恢复本地生成字段，去除生成器写入的无关 `go.sum` 副作用 |
| 生成 | `GOPROXY=https://goproxy.cn,direct go generate ./cmd/server` | 0 | Wire 成功生成两次且结果一致 |
| 同步后 | handler/server/repository/migration 定向测试 | 0 | failover、compact、路由、支付、API 契约、调度缓存与迁移测试通过 |
| 同步后 | service 核心定向测试 | 0 | Grok compact、WS bridge、CC/Responses fallback、keepalive 与首 Token 测试通过 |
| 同步后 | `go test -tags=unit ./...` | 0 | 全部通过；最慢 `internal/service` 115.911 秒 |
| 同步后 | `go test -tags=integration ./...` | 1 | 仅 `internal/pkg/tlsfingerprint` 的 3 个外部联网用例因 `tls.peet.ws:443` 拒绝连接失败，其余通过；该包本次无代码变化，失败与上次同步记录一致 |
| 同步后 | `golangci-lint run ./...` | 0 | `0 issues` |
| 同步后 | `go build -o <系统临时文件> ./cmd/server` | 0 | 构建通过，临时产物已安全删除 |
| 同步后 | `corepack pnpm --dir frontend run lint:check` | 0 | 通过 |
| 同步后 | `corepack pnpm --dir frontend run typecheck` | 0 | 通过 |
| 同步后 | `corepack pnpm --dir frontend exec vitest run` | 0 | 163 个文件、1035 个测试全部通过 |
| 同步后 | `corepack pnpm --dir frontend run build` | 0 | 通过；存在 Browserslist、动态/静态导入和大 chunk 非致命警告 |
| 同步后 | `bash -n deploy/apple-container.sh deploy/tests/apple-container-test.sh` | 0 | shell 语法检查通过 |
| 同步后 | `git diff --check`、冲突标记、意外删除、敏感路径与 untracked 检查 | 0 | 通过 |

### 未验证项与残余风险

- 本次未在 merge 前重新运行完整基线；同步后 unit、lint、构建和前端全量测试均通过，integration 唯一失败包本次未修改且失败与已有记录相同。
- 本机没有 Node 20，未验证与 CI Node 20 完全等价；Node 24 验证已通过。
- 本机不是 macOS，Apple container 生命周期测试未执行，仅完成 shell 语法和 fixture 静态审查。
- 当前 `CGO_ENABLED=0`，未运行 `-race`；Go 明确报告 `-race requires cgo`。
- 未读取 `.env`，也没有安全的隔离运行配置，因此未执行本地服务启动和健康检查。
- WS HTTP bridge 使用 detached context 排空上游；若上游永久停滞，缺少流级截止时间可能长期占用响应体和账号并发，这是既有行为。
- HTTP bridge 尚无直接覆盖 `response.failed` 与仅 `[DONE]` 结束分支的定向测试。
- 未执行 push、PR、部署、远程服务器访问或生产数据操作。

## 2026-07-20 同步至 e625ce3b3

- 执行时间：2026-07-20T20:52:43+08:00
- 执行状态：同步分支完整合并并验证完成；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`ae1f867eeb647fde908a01626bdc18ffc90b30d0`
- 上游代码合并提交：`9d0083646b65a86d631afc6ac57a4618b78888e8`
- 二次开发适配提交：`fc53abf1b`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`da85cc7e47882090b115d664afe8e39b37aa7417`
- `UPSTREAM_NEW_SHA`：`e625ce3b3b3b955b7c3afc93221f7c5f0ae55aa8`
- merge-base：`da85cc7e47882090b115d664afe8e39b37aa7417`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`e625ce3b3b3b955b7c3afc93221f7c5f0ae55aa8`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决文本和语义冲突，重新生成 Ent/Wire，再提交兼容性修复
- 备份分支：`backup/pre-upstream-sync-20260720-184738-ae1f867ee`
- 同步分支：`sync/upstream-20260720-e625ce3b3`

### 上游提交处置

本次固定范围共 445 个提交，其中 161 个 merge commit、284 个 non-merge commit。完整 merge 保留了该范围内全部祖先关系；由 `git rev-list --reverse da85cc7e4..e625ce3b3` 产生的每个 SHA 均为 `Applied`，无 `Already Applied`、`Skipped` 或 `Deferred`。

| 上游提交集合 | 数量 | 状态 | 内容与处置 |
| --- | ---: | --- | --- |
| `da85cc7e4..e625ce3b3` | 445 | Applied | 整体映射到本地 merge commit `9d0083646`；固定范围完整进入本地历史 |
| `d515c3045`、`60732a2e8`、`bc2244c83`、`c2c19a7cb`、`57914967c`、`d4b9797ff`、`e625ce3b3` | 7 | Applied + Overridden | 上游版本从 0.1.156 递增至 0.1.162；最终保留本地较新版本 0.1.203 |
| `d11bdb13f` 及其安全审计修复链 | 已包含于 445 | Applied + Overridden | 接入上游 prompt audit、guard、控制台和全提示词持久化，同时保留本地内容审核与 Cyber 会话阻断作为统一安全审计协调器的兼容降级 |
| `90ee85f3e` 及倍率探测链 | 已包含于 445 | Applied + Overridden | 接入上游计费倍率探测、展示和调度成本；计费归因继续按本地设计统一使用用户请求模型，不引入不可达的逐账号 billing source 语义 |
| Agent Identity、WS 生命周期、HTTP bridge 安全修复链 | 已包含于 445 | Applied + Overridden | 接入 Agent Identity、终态事件、任务恢复和安全切号；保留本地首 Token、语义错误、结果归因、图片计费及后续 turn 不重放约束 |

### 本地提交与文件

- 上游范围整体映射到本地 merge commit：`9d0083646b65a86d631afc6ac57a4618b78888e8`。
- 二次开发兼容提交：`fc53abf1b`，包含 11 个文件、41 行新增和 50 行删除。
- 同步记录提交前，代码与配置相对 `LOCAL_PRE_SYNC_SHA` 修改 848 个文件：新增 294 个、修改 553 个、删除 1 个；共新增 102905 行、删除 5406 行，包含 3 个二进制资源/归档。
- 唯一删除文件 `backend/internal/repository/ops_repo_lookup_deleted_key_audit_integration_test.go` 来自上游 `b92bbf029`，其覆盖已由新的入口拒绝和鉴权边界测试替代。
- 主要更新：Agent Identity、OpenAI WS/HTTP bridge 生命周期、异步图片任务、图片输入 Token 定价、上游倍率探测与调度、Grok OAuth/媒体/视频/缓存恢复、prompt audit、安全审计控制台、审计日志、step-up、可信代理与客户端 IP、重复创建幂等、运维入口拒绝聚合、支付币种与充值返利、前端 i18n/品牌和部署参数。
- 已重新生成 Ent 与 Wire；`backend/ent/migrate/schema.go` 和 `backend/cmd/server/wire_gen.go` 与合并后的 schema/provider 源一致。

### 冲突与最终解决方案

- 69 个初始文本冲突均逐文件处理，无整文件采用 `ours` 或 `theirs`，最终索引没有未解决冲突。
- `VERSION` 保留本地 0.1.203；上游新增 SVG logo，同时恢复本地 `frontend/public/logo.png`。
- 安全审计入口只调用一次 `checkSecurityAudit`；协调器不可用时继续执行本地内容审核，避免重复审核，也不丢失 Cyber 会话阻断。
- Gateway/OpenAI 转发保留本地计费预检、首 Token 超时、语义错误、结果归因、心跳和图片计费，并叠加上游 Agent Identity、终态事件、Grok encrypted reasoning 恢复及异步图片接口。
- WS v2 和 HTTP bridge 同时保留逐轮图片计费、终态错误归因、Responses Lite payload 与 turn 生命周期；后续 turn 的传输错误写入错误事件，不再误包装为可切号错误。
- Claude OAuth 模拟继续依赖 handler 的严格客户端判定，非法 metadata 会被规范化；真实 Claude Code context 保持客户端 headers/body，Haiku 兼容路径继续执行完整 system 改写。
- `AdminService` 同时保留本地默认定时测试计划仓库、账号连续失败缓存，以及上游 duplicate repositories、affiliate service；Wire 中合并设置服务与 Agent Identity WS invalidator。
- 渠道 token 定价在缺少基础价格且只配置区间时不再因图片输入价覆盖触发空指针；媒体定价完整性仍按本地请求前校验规则执行。
- DataTable、账号滚动重置、倍率探测、支付返利/返佣设置及相关双方测试均保留。
- 两份生产 Compose 同时保留本地资源参数与上游 Redis 参数，删除重复 PostgreSQL `command`；SHA-256 均为 `89437fa1258bade1251787d53e061deb525d18af89c7e1354719924000f1b493`。

### 刻意保留的二次开发功能

- 账号连续失败停调度、strict 调度、pending/final outcome、streak 清理和自动托管恢复测试。
- 首 Token 超时、2xx 语义错误、body-signal compact、paused keepalive、WS lease、终态归因和流内失败结算。
- 按用户请求模型计费、渠道定价完整性、图片/视频计费、充值返利、签到、模型广场和上游站点同步。
- Claude Code 上游模拟、Codex 图片工具策略、API Key 请求头覆写、渠道监控结构化 Responses `input`。
- 本地内容审核、人工审计、Cyber 会话阻断与上游 prompt audit 的统一协调。
- 账号列表禁用虚拟化、查询上下文滚动重置，以及生产 bind mount、localhost 暴露和双 Compose 一致性约束。

### 验证记录

验证使用仓库内现有 Go 1.26.5、Node 20.19.4、Go/Node 缓存和已安装前端依赖。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | 完整回归基线 | 未保留 | 执行上下文交接时已进入 merge 现场；未伪造同步前命令或退出码，`main` 与备份分支始终保持 `LOCAL_PRE_SYNC_SHA` |
| 生成 | `go generate ./ent`、`go generate ./cmd/server` | 0 | Ent/Wire 生成成功 |
| 适配 | `go test -tags=unit -run '^$' ./...` | 0 | Go 全包编译通过 |
| 适配 | handler/admin/service/repository/server 定向测试 | 1/0 | 首次发现 Claude、WS、媒体定价和模板 SQL 断言问题；修复后各失败组与完整 `internal/service` 均通过 |
| 同步后 | `go test -tags=unit ./...` | 1/0 | 首次仅模板 SQLMock 未包含本地字段；适配后全包通过，后续受影响 handler/service 串行重跑通过 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 1/0 | 首次仅工具链 PATH 缺失；修正后发现并删除 3 个不可达定义，最终 `0 issues` |
| 同步后 | `make build`（backend） | 0 | `CGO_ENABLED=0` 构建 0.1.203 成功 |
| 同步后 | `npm run lint:check`、`npm run typecheck` | 0 | 使用 Node 20，均通过 |
| 同步后 | `npm run test:run` | 1/0 | 首次发现 4 个过期断言和 1 个缺失依赖链接；适配后 196 个测试文件全部通过 |
| 同步后 | `npm run build` | 0 | `vue-tsc -b` 与 Vite 生产构建通过；仅有动态导入和大 chunk 非致命警告 |
| 同步后 | `/bin/bash -n deploy/apple-container.sh` | 0 | shell 语法检查通过 |
| 同步后 | `TMPDIR=<项目缓存> /bin/bash deploy/tests/apple-container-test.sh` | 0 | Apple container 生命周期 fixture 全部通过，未操作真实容器引擎 |
| 同步后 | Compose 哈希、冲突标记、敏感路径、删除来源和 `git diff --check` | 0 | 两份生产 Compose 完全一致；源码无冲突标记、敏感文件路径或空白错误；归档补丁保持原样并从源码空白检查中排除 |

### 未验证项与残余风险

- 未运行 `go test -tags=integration ./...`、Testcontainers、`-race` 或真实数据库迁移；这些操作需要 Docker/CGO 或会扩大当前授权范围。
- 未读取 `.env`，也未启动需要 PostgreSQL/Redis 的真实服务，因此本地健康检查标记为未验证。
- 未使用真实 OpenAI、Anthropic、Grok、Agent Identity、S3 或支付凭据验证外部业务流程；相关请求转换、handler、service、repository 和前端用例已通过本地测试。
- 当前 `node_modules` 来自既有 pnpm 9 环境；新增 message compiler 已存在于本地 store 并完成测试，但未执行 CI 等价的全新 `pnpm install --frozen-lockfile`。
- 上游归档 `openspec/changes/add-openai-compatible-prompt-audit/source-freeze/aicodex-prompt-audit-tracked.patch` 保存另一个仓库的原始空白差异，未格式化或修改。
- 未执行 push、PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-07-25 同步至 6d956bdc2

- 执行时间：2026-07-25T18:46:50+08:00
- 执行状态：同步分支完整合并并验证完成；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`02a8884a350664d132fd57f36b9d12e3591c683d`
- 上游代码合并提交：`68f708d11fc13eaa7dc0f6738fdae2a5c1b8dac0`
- 最后一个代码修复提交：`0ccedbc0a6df98bd4509ef50eed00a3a40a957a4`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`60013c5f100be7b4f2e6caee415883d221d33e32`
- `UPSTREAM_NEW_SHA`：`6d956bdc20f0d8c38275d4d77b628a8ff776711c`
- merge-base：`60013c5f100be7b4f2e6caee415883d221d33e32`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`6d956bdc20f0d8c38275d4d77b628a8ff776711c`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决 23 个文本冲突和相关语义冲突，重新生成 Ent/Wire，并将必要兼容处理纳入 merge commit
- 备份分支：`backup/pre-upstream-sync-20260725-151555-02a8884a3`
- 同步分支：`sync/upstream-20260725-6d956bdc2`

### 上游提交处置

本次固定范围共 61 个提交，其中 18 个 merge commit、43 个 non-merge commit。61 个提交均通过完整 merge 保留祖先关系并记为 `Applied`，其中下表列出的提交同时按本地二次开发边界记为 `Applied + Overridden`；无 `Already Applied`、`Skipped`、`Deferred` 或未解决 `Conflict`。

| 上游提交集合 | 数量 | 状态 | 内容与处置 |
| --- | ---: | --- | --- |
| `60013c5f1..6d956bdc2` | 61 | Applied | 整体映射到本地 merge commit `68f708d11`；引入 Composite 分组路由、Ollama Cloud 用量、客户端 session ID、支付宝移动端当面付唤起、OpenAI 输入 namespace/item ID 清洗、代理流熔断、Claude Opus 5、Grok 与简单模式修复、前端移动端适配及依赖更新 |
| `44093579e`、`ba88cc239` | 2 | Applied + Overridden | 保留渠道模型名规范化和 Composite 按实际路由模型计费的修复意图；普通分组继续严格按用户请求模型计费，Composite 公开别名仅在存在显式渠道价时按别名计费，否则按具体模型计费 |
| `47ad29db3`、`6aeea70ee` | 2 | Applied + Overridden | 接入 OpenAI 代理流断连熔断；两份生产 Compose 使用本地批准的阈值 2、窗口 60 秒、TTL 600 秒，并继续保留本地连接池、worker、数据库和日志参数 |
| `e70fbf320`、`1bdf99109`、`9994eaa70`、`c5d9d5794`、`1891faa68`、`333acde7d` | 6 | Applied + Overridden | 接入 HTTP 输入 namespace 与 API Key item ID 清洗；同步重建本地增量请求视图，避免后续补丁恢复已移除字段，同时保持 upstream/client stream 拆分、首 Token 与 WS 本地语义 |
| `cb24522dd` | 1 | Applied + Overridden | 保留上游版本提交的祖先关系，最终版本继续使用本地较新的 `0.1.210`，不回退到上游 `0.1.164` |

### 本地提交与文件

- 上游范围整体映射到 merge commit `68f708d11fc13eaa7dc0f6738fdae2a5c1b8dac0`，其双亲为同步前本地 SHA 和固定上游 SHA。
- merge commit 相对 `LOCAL_PRE_SYNC_SHA` 修改 246 个文件：新增 53 个、修改 193 个、删除 0 个，共增加 19218 行、删除 547 行。
- 二开台账基线与验证记录更新提交：`0a4763eb4040cc756ebeddf9adc33d345fc12aeb`。
- 首轮 CI 批量图片 PostgreSQL 插入修复提交：`0ccedbc0a6df98bd4509ef50eed00a3a40a957a4`。
- 主要新增：Composite 路由表及管理接口、Ollama Cloud 用量抓取与前端设置、用量 session ID、支付宝移动端深链、OpenAI 代理流熔断、Claude Opus 5 定价与测试。
- Ent/Wire 从合并后的 schema/provider 重新生成；重复生成的受控差异 SHA-256 均为 `faf24976dec5dabde9d3b57542b6a39a8a76b2cb`。
- `frontend/pnpm-lock.yaml` 仅包含上游依赖更新；安装、测试和构建后无额外未暂存锁文件变化。

### 冲突与最终解决方案

- 23 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`，最终索引无未解决路径。
- 支付配置、订单履约与前端支付流程同时保留本地充值赠送、专属倍率用户禁返利和上游支付宝移动端唤起。
- Composite 路由接入 handler、service、repository、Ent 与 Wire；计费和额度平台按实际路由账号归属，同时保留普通分组严格请求模型计费边界。
- OpenAI Responses 同时保留本地 upstream/client stream 拆分、首 Token、WS 逐轮结算和审计，并接入 namespace/item ID 清洗及代理流熔断；修复请求视图未同步导致清洗字段被后续补丁恢复的问题。
- 审计日志继续保留成功会话记录、敏感字段清理和本地响应包装，接入 session ID 但不记录 Ollama 会话明文。
- 图片与批量图片路径保留本地分组成功率、请求质量/尺寸、公开字段和结算语义，并接入上游诊断字段与 Composite 路由。
- 首轮 GitHub Actions 集成测试发现 `batch_image_jobs` 同时合入本地 `group_id` 和上游 `session_id` 后有 38 个目标列，但 `VALUES` 仅到 `$37`；补齐 `$38` 并增加 38 参数 SQLMock 回归，避免仅靠真实 PostgreSQL 才发现列值数量漂移。
- `frontend/src/components/common/DataTable.vue` 保留本地非虚拟化和滚动稳定性，同时接入上游表格行为更新。
- `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 保持完全一致，SHA-256 均为 `2A5CD7204F2B66A49ACBAF914108383B48D5E0F292E49AAEF16508059275A731`；继续使用 bind mount、回环地址、HTTP upstream 业务开关和既定 4 vCPU/8 GiB 参数。

### 刻意保留的二次开发功能

- 首 Token 超时、upstream/client stream 拆分、WS 逐轮并发释放/结算、成功会话审计与失败调度保护。
- 普通分组严格按用户请求模型计费、严格缺价错误、Composite 显式别名渠道价例外、按用户串行扣费和 5 秒 usage task 超时。
- 图片分组成功率、批量图片字段与结算、充值赠送、专属倍率用户禁返利和内容审核/Cyber 阻断。
- 账号列表禁用虚拟化、查询后滚动重置、DataTable 稳定性、二开配置入口和生产双 Compose 约束。
- 生产连接池、worker、PostgreSQL、Go 内存与 Docker 日志参数均保持批准基线，没有为了提高 worker 数量扩大数据库连接池。

### 验证记录

验证使用 Go 1.26.3、Node 24.15.0、corepack pnpm 9.15.9 和 golangci-lint 2.9.0。系统 pnpm 为 11.9.0，仅用于确认失败原因，未用于最终安装或修改锁文件。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | Git SHA、祖先关系与工作树预检 | 0 | `main` 固定在 `02a8884a3`，旧上游 SHA 是本地祖先；创建备份与同步分支后开始合并 |
| 生成 | `go generate ./ent`、`go generate ./cmd/server`，重复执行 | 0 | Ent/Wire 生成成功且两次结果哈希一致 |
| 适配 | `go test -run '^$' ./...` | 1 / 0 | 首次发现请求流拆分和 Composite 计费调用不兼容；修复后全包编译通过 |
| 适配 | 计费、Grok、API 契约和 OpenAI namespace/item ID 定向测试 | 1 / 0 | 首轮定位构造参数、Grok 不可达分支和请求视图恢复 namespace；修复后全部通过 |
| 同步后 | `go test -tags=unit ./...` | 1 / 1 / 0 | 依次发现 `NewAdminService` 新参数缺失及 4 个 OpenAI namespace 回归；修复后全包通过 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 1 / 0 | 首轮发现 5 个问题：无效 stream 赋值、Grok 恒真判断和 2 个未使用通用计费 helper；按本地策略修复后 `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath -o bin/server.exe ./cmd/server` | 0 | 后端构建通过，产物位于忽略目录 |
| 前端安装 | 系统 pnpm 11 frozen install（两次尝试） | 1 / 1 | 分别被新发布 `postcss@8.5.23` 的最短发布时间策略和 pnpm 11 忽略 `package.json#pnpm.overrides` 导致的 frozen mismatch 拒绝；锁文件未变化 |
| 前端安装 | `corepack pnpm install --offline --frozen-lockfile` | 0 | 使用与 CI 对齐的 pnpm 9.15.9 离线冻结安装成功 |
| 同步后 | `corepack pnpm run lint:check`、`corepack pnpm run typecheck` | 0 | 均通过 |
| 同步后 | `corepack pnpm run test:run` | 0 | 前端全量 Vitest 通过；仅输出既有 i18n 测试警告 |
| 同步后 | `corepack pnpm run build` | 0 | TypeScript/Vite 生产构建通过；仅有大 chunk 非致命警告 |
| 同步后 | Compose 哈希、关键环境参数、冲突标记、删除、敏感路径、锁文件和 `git diff --check` | 0 | 两份生产 Compose 完全一致；无源码冲突标记、删除文件、实际凭据路径、未暂存锁文件或空白错误 |
| 远端 CI 首轮 | GitHub Actions `CI` / `Integration tests` | 2 | 其余 CI Job 与 `Security Scan` 通过；9 个批量图片 repository 用例统一因 `INSERT has more target columns than expressions` 失败，定位为合并后的占位符缺失 |
| CI 修复 | `go test -tags=unit ./internal/repository -run '^TestCreateBatchImageJobWithSQLBindsAllColumns$'`、repository 全包 | 0 | 新增回归确认 38 个目标列绑定 `$1..$38` 和 38 个参数 |
| 远端 CI 修复后 | GitHub Actions `CI` 运行 `30158725332`、`Security Scan` 运行 `30158725302` | 0 | shell、frontend、golangci-lint、Unit tests、Integration tests 与安全扫描全部通过 |

### 未验证项与残余风险

- 本地未运行 `go test -tags=integration ./...`、Testcontainers、真实 PostgreSQL migration 或 `-race`；GitHub Actions 已运行完整 Integration tests，修复批量图片 SQL 后全部通过。
- 本机 `docker` 与 `govulncheck` 不在 PATH，未执行 Compose 启动、容器健康检查或 Go 漏洞可达性扫描。
- 未读取 `.env`，未启动依赖 PostgreSQL/Redis 的真实服务，因此本地启动与健康检查标记为未验证。
- 未使用真实 OpenAI、Anthropic、Grok、Ollama、支付、S3 或上游站点凭据；外部业务流程仅由本地单元、契约和前端测试覆盖。
- 未执行真实浏览器端到端交互；移动端支付、Ollama Cloud 设置和 Composite 管理页面由组件测试、typecheck 和生产构建覆盖。
- 已按用户授权推送 `main` 以执行 GitHub Actions；未执行 PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-07-23 同步至 60013c5f1

- 执行时间：2026-07-23T01:40:53+08:00
- 执行状态：同步分支完整合并并验证完成；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`69c680f2f834670c209d70e1210c71d42c7611c5`
- 上游代码合并提交：`e957a0a38f0e969a104789190ad8ab0407fde05e`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`e625ce3b3b3b955b7c3afc93221f7c5f0ae55aa8`
- `UPSTREAM_NEW_SHA`：`60013c5f100be7b4f2e6caee415883d221d33e32`
- merge-base：`e625ce3b3b3b955b7c3afc93221f7c5f0ae55aa8`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`60013c5f100be7b4f2e6caee415883d221d33e32`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决文本与语义冲突，重新生成 Ent/Wire，并将必要兼容处理纳入可编译的 merge commit
- 备份分支：`backup/pre-upstream-sync-20260723-012045-69c680f2f`
- 同步分支：`sync/upstream-20260723-60013c5f1`

### 上游提交处置

本次固定范围共 69 个提交，其中 30 个 merge commit、39 个 non-merge commit。完整 merge 保留全部祖先关系；68 个提交为 `Applied`，1 个版本提交为 `Already Applied + Overridden`，无 `Skipped`、`Deferred` 或未解决 `Conflict`。

| 上游提交集合 | 数量 | 状态 | 内容与处置 |
| --- | ---: | --- | --- |
| `e625ce3b3..d0bdd7e77` | 68 | Applied | 整体映射到本地 merge commit `e957a0a38`；引入 Grok compact/客户端工具/错误隔离、OpenAI reasoning effort、调度可观测性与缓存修复、hosted image token 计费、Redis ACL、移动端适配、用量筛选及依赖安全更新 |
| `60013c5f1` | 1 | Already Applied + Overridden | 与本地 `6f96ebbb5` patch-id 等价；完整 merge 保留上游祖先关系，最终继续使用本地较新版本 `0.1.207` |
| `106043fd9`、`9da816154` | 已包含于 68 | Applied + Overridden | 接受示例镜像修复意图，但生产 Compose 继续使用本地 `SUB2API_IMAGE`/`SUB2API_TAG` 变量，不改为固定上游镜像 |
| `2ae61f3da`、`0b9d44545` | 已包含于 68 | Applied + Overridden | 接入 Grok compact 输入/输出转换；保持本地 body-signal compact 上游 unary、下游按客户端意愿桥接 SSE、首 Token watchdog 与暂停心跳语义 |
| `29bea0a75`、`a31933316`、`1f9eac4fb` | 已包含于 68 | Applied + Overridden | 接入调度排除原因统计；继续按本地用户请求模型执行渠道限制，并保留图片尺寸层级能力检查 |
| `6af622c34`、`6c93f01c9`、`ebfaf2496` | 已包含于 68 | Applied + Overridden | 接入分组 reasoning effort 映射与上限；WS 同时保留本地逐轮 usage、失败阻断、审计哈希和并发槽位生命周期 |

### 本地提交与文件

- 上游范围整体映射到本地 merge commit：`e957a0a38f0e969a104789190ad8ab0407fde05e`。
- 上游 merge commit 相对 `LOCAL_PRE_SYNC_SHA` 修改 168 个文件：新增 24 个、修改 144 个、删除 0 个，共增加 7338 行、删除 531 行，包含 1 个新增移动端截图。
- 主要更新：Grok compact、Codex/custom/tool_search/namespace 工具回程、Grok OAuth 模型与 403 隔离、OpenAI reasoning effort 分组策略、调度排除统计与缓存侧键、hosted image token 计费、Redis ACL、优雅关停、代理双栈探测、套餐有效期、移动端账号/运维布局、用量筛选和 Axios/Go 安全依赖。
- Ent 与 Wire 已从合并后的 schema/provider 重新生成；重复生成后工作树保持干净。
- `docs/custom-development-history.md` 更新到本次代码基线，修正当前能力族数量为实际的 47 项，并追加上游适配记录。

### 冲突与最终解决方案

- 9 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`，最终索引无未解决项。
- `backend/cmd/server/VERSION` 保留本地 `0.1.207`，不回退到上游 `0.1.163`。
- `backend/ent/mutation.go` 及关联生成代码由合并后的 schema 重新生成，同时保留本地 group 字段和上游 reasoning effort 两个新字段。
- `backend/internal/handler/openai_gateway_handler.go` 同时保留 WS 逐轮 usage、失败阻断和请求审计，并接入 reasoning effort 映射/上限。
- `backend/internal/service/openai_account_scheduler.go` 接入排除原因统计，保留本地请求模型渠道限制入口与图片尺寸层级能力；未恢复已被本地计费策略覆盖的逐账号 upstream billing source 分支。
- `backend/internal/service/openai_gateway_grok.go` 同时保留 upstream/client stream 分离、首 Token、客户端断开结算，并接入客户端工具流式回程。
- `deploy/docker-compose.yml` 保留本地镜像变量、网络和数据约束；Redis ACL username 同步到 `deploy/docker-compose.sub2api.yml`，两份生产 Compose 保持完全一致。
- `frontend/pnpm-lock.yaml` 升级 Axios 至 1.18.1，并保留本地 overrides 和其他依赖。
- `frontend/src/main.ts` 同时保留安全存储、启动错误兜底与 iOS viewport 修复；`frontend/src/views/user/PaymentView.vue` 同时保留充值赠送与套餐有效期格式化。
- 首次编译发现 Grok compact helper 与本地 helper 同名、上游测试仍使用旧签名、调度器引用本地已移除的 upstream billing source 方法；按现有本地契约修复后全包编译通过。

### 刻意保留的二次开发功能

- 首 Token 超时、body-signal unary compact、暂停心跳、upstream/client stream 分离、WS 逐轮结算和失败阻断。
- 连续失败停调度、strict 调度、pending/final outcome、图片尺寸能力、请求模型渠道限制及调度 outbox 语义。
- 按用户请求模型计费、渠道定价完整性、图片/视频计费、Image 分组成功率、充值赠送和专属倍率用户限制。
- Codex 图片工具策略、Claude Code 上游模拟、API Key 请求头覆写、内容审核/Cyber 阻断和渠道监控结构化 Responses `input`。
- 账号列表禁用虚拟化、查询上下文滚动重置、主题与启动容错，以及生产 bind mount、localhost 暴露和双 Compose 一致性约束。

### 验证记录

验证使用项目内现有 Go 1.26.5、Node 20.19.4、pnpm 9.15.9、golangci-lint 2.9.0 和项目内缓存。所有构建、审计输出和临时文件均位于仓库 `.cache`、`backend/bin` 或前端目录。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./...` | 0 | 全包通过 |
| 同步前 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步前 | `CGO_ENABLED=0 go build -trimpath ./cmd/server` | 0 | 构建通过 |
| 同步前 | `govulncheck ./...` | 0 | 无可达漏洞；依赖模块中的 8 个已知漏洞均未被当前代码调用 |
| 同步前 | `pnpm install --frozen-lockfile`、lint、typecheck | 0 | 均通过，使用项目内 pnpm store |
| 同步前 | `pnpm run test:run` | 0 | 196 个测试文件、1350 个测试通过 |
| 同步前 | `pnpm run build` | 0 | 生产构建通过；仅有既有 Browserslist、动态导入和大 chunk 警告 |
| 同步前 | `pnpm audit` / 审计例外门禁 | 1 / 0 | 0 critical、2 high、30 moderate、8 low；仓库例外校验通过 |
| 同步前 | Apple 脚本/fixture、4 份 Compose 静态解析、双生产 Compose 比较 | 0 | 全部通过，未启动真实容器或服务 |
| 生成 | `go generate ./ent`、`go generate ./cmd/server` | 0 | 首次及最终重复生成均成功且结果稳定 |
| 适配 | `go test -tags=unit -run '^$' ./...` | 1 / 0 | 首次发现 3 类双方独立演进造成的编译问题；修复 helper、测试签名和本地计费策略后全包编译通过 |
| 适配 | service 高风险定向回归 | 1 / 0 | 首次 2 个 Grok compact 用例发现 `stream:false` 字段破坏本地 unary body 契约；改为删除该字段后 compact 与完整定向集合通过 |
| 适配 | handler、repository、migration、前端 11 文件定向测试 | 0 | 后端相关包通过；前端 66 个测试通过 |
| 同步后 | `go test -tags=unit ./...` | 0 | 全包通过 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath ./cmd/server`、`govulncheck ./...` | 0 | 构建通过；安全结果与同步前一致，无可达漏洞 |
| 同步后 | 前端 frozen install、lint、typecheck、全量 Vitest、build | 0 | 201 个测试文件、1377 个测试通过；生产构建通过 |
| 同步后 | `pnpm audit` / 审计例外门禁 | 1 / 0 | 漏洞计数与同步前完全一致，仓库例外校验通过 |
| 同步后 | Apple 脚本/fixture、4 份 Compose 静态解析、双生产 Compose 比较 | 0 | 全部通过；两份生产 Compose SHA-256 均为 `92769f2b9f18e415b8c88927c6e4655abe8085dcd3518f74b616b64c6b8a2534` |
| 同步后 | `git diff --check`、冲突标记、意外删除、敏感路径、祖先关系和工作树检查 | 0 | 无空白错误、冲突标记、删除文件或敏感文件；固定上游 SHA 已成为本地祖先，代码提交后工作树干净 |

### 未验证项与残余风险

- 未运行 `go test -tags=integration ./...`：所需 `redis:8.4-alpine` 与 `postgres:18.1-alpine3.23` 镜像不存在，自动拉取会写入项目外 Docker 全局缓存，违反本次临时文件约束；真实 PostgreSQL migration 因此未验证。
- 未运行 `-race`；当前验证范围不包含 CGO race 环境。
- 未读取 `.env`，也未启动依赖 PostgreSQL/Redis 的真实服务，本地启动和健康检查标记为未验证。
- 未使用真实 OpenAI、Anthropic、Grok、支付、S3 或上游站点凭据；协议转换、handler、service、repository 和前端组件仅通过本地测试覆盖。
- 未执行真实浏览器端到端交互；移动端和 iOS 变更由组件测试、typecheck 和生产构建覆盖。
- `pnpm audit` 仍报告 2 high、30 moderate、8 low，均为同步前已有且通过仓库例外门禁；发布前仍需远端 CI 复核。
- 未执行 push、PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-08-02 同步至 7e2e9ba05

- 执行时间：2026-08-02T17:36:21+08:00
- 执行状态：同步分支完整合并并通过可在当前本机执行的关键验证；integration 与 Apple 生命周期 fixture 的环境限制已如实记录；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`7b65f71d43e33925dceb6eb6728bdc54cbc81c0e`
- 上游代码合并提交：`6884fa6826883f1cccb1c119d22b7095d77eb3ee`
- 最后一个代码/测试提交：`6884fa6826883f1cccb1c119d22b7095d77eb3ee`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`5a6143097db142b72a6fc848c214e97214470bdd`
- `UPSTREAM_NEW_SHA`：`7e2e9ba05026b7126318aa0754c1afa0ac00bc58`
- merge-base：`5a6143097db142b72a6fc848c214e97214470bdd`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`7e2e9ba05026b7126318aa0754c1afa0ac00bc58`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决 22 个文本冲突，复核自动合并路径的二开语义，补齐合并后测试契约并重复生成 Ent/Wire；上游祖先关系由独立 merge commit 完整保留
- 备份分支：`backup/pre-upstream-sync-20260802-173621-7b65f71d4`
- 同步分支：`sync/upstream-20260802-7e2e9ba05`

### 上游提交处置

本次固定范围共 99 个提交，其中 40 个 merge commit、59 个 non-merge commit。99 个提交均通过完整 merge 保留祖先关系并记为 `Applied`；下表重点提交组同时按本地二次开发边界记为 `Applied + Overridden`。重点组是总范围的子集，数量不可与 99 重复相加。无 `Already Applied`、`Skipped`、`Deferred` 或未解决 `Conflict`。

| 上游提交集合 | 状态 | 内容与处置 |
| --- | --- | --- |
| `5a6143097..7e2e9ba05`（99 个） | Applied | 完整接入 Anthropic classifier/count-tokens、Codex namespace 与工具图片、OpenAI WS/compaction、流式部分 usage、全 API-key 平台倍率探测与写回、分组利润控制、安全审计、内容审核代理、Compact 首页、支付设置、模型定价、部署安全及相关测试 |
| `7ceabb3fd`、`7e2e9ba05` | Applied + Overridden | 保留上游版本提交的祖先关系，最终版本继续使用本地较新的 `0.1.213`，不回退到上游 `0.1.169` 或 `0.1.170` |
| `efa5a2240`、`54b1f8f6b`、`352b21f4e`、`2ef124629`、`53aa5cd24`、`2be08f3f3`、`a8cd33eea`、`8f5caef78` | Applied + Overridden | 接入 count-tokens 的 `max_tokens` 清理和 classifier 多 system/auto 分类兼容；真实 Claude Code 仍要求严格协议头判定，测试补齐 `interleaved-thinking`，不因宽松 body 特征放大客户端信任面 |
| `272735b0a`、`dfdbc2770`、`2bf9c6d56`、`fe2172586`、`21aacde0b` 及对应 merge commit | Applied + Overridden | 接入 Codex namespace、缺失 instructions 默认值、工具输出图片桥接、加密 compaction 恢复和 WS relay 关闭竞态修复；继续保留本地图片工具策略、图片尺寸能力透传、逐轮 WS 结算，以及非流式 HTTP 不转入 WS 的边界 |
| `7d3bf86e5`、`da49ce3f2`、`85a27fae3`、`bd52e5d77` 及对应 merge commit | Applied + Overridden | 接入 pool mode 流式容量重试、代理流熔断、SSE 429 和 Anthropic 中断流已观测 usage 记录；首 Token 缓冲器会先交付已读取 prelude，再向协议层报告上游读错误，已观测 Anthropic usage 后禁止错误 failover，避免漏计或重复计费 |
| `f3a3d8684`、`56f3d3c9b`、`b0f5007f0`、`0b6b4ea95`、`20ad5ec50`、`fad2f215e`、`dec47e8fa`、`d99ee7291`、`11c1e944b`、`c043c2477` | Applied + Overridden | 接入全 API-key 平台倍率探测、受控倍率写回和分组利润控制；CreateAccount 向所有 API-key 平台传递探测开关，调度使用映射后模型限制辅助逻辑，同时继续保留专属分组约束、普通分组按请求模型计费、Composite 显式例外和严格缺价处理 |
| `fc495e087`、`d74e669a2`、`570ea74d1`、`948b63c9c`、`682c4fe0e` | Applied + Overridden | 接入 Qwen3Guard 辅助字段、窄范围阻断审计和内容审核代理；兼容新增仓储参数，继续保留本地人工审核、统一安全审计协调器、Cyber 会话阻断与过载回退边界 |
| `b2d895fb8`、`488d3b09e`、`313121f3f`、`493955f7b`、`f54e9827a`、`698547418` 及对应 merge commit | Applied + Overridden | 接入 GPT-5.6、GLM-5.2、Codex Auto-review 定价与 release fallback 资源；普通文本仍严格按用户请求模型计费，媒体继续使用实际模型，未以映射名或最终上游模型静默改变本地计费来源 |
| `739c0ff9c`、`beeb2f989`、`2980ff385`、`0ee9ea576`、`3deb2f17d`、`8ed9f754c`、`c772d1866`、`132d446ca` | Applied + Overridden | 接入 Compact 首页和支付标题、可见支付方式、选择器布局修复；Home compact 测试补齐本地模型市场所需 app store mock，本地 `/models`、充值赠送及支付增强继续保留 |
| `0010894f9`、`0a45be17d` | Applied + Overridden | 接入容器 `no-new-privileges` 安全配置，同时保持两份生产 Compose 的 bind mount、仅回环暴露、HTTP upstream 业务开关和 4 vCPU/8 GiB 资源参数基线 |

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `6884fa6826883f1cccb1c119d22b7095d77eb3ee`；双亲分别为同步前本地 SHA `7b65f71d43e33925dceb6eb6728bdc54cbc81c0e` 与固定上游 SHA `7e2e9ba05026b7126318aa0754c1afa0ac00bc58`。
- 写入两份台账前，代码、配置、资源和测试相对 `LOCAL_PRE_SYNC_SHA` 修改 295 个文件：新增 57 个、修改 237 个、删除 1 个，共增加 16820 行、删除 1189 行。
- 唯一删除文件为上游不再引用的合作方资源 `assets/partners/logos/666api.jpg`；未删除本地业务文件。
- Ent 与 Wire 重复生成结果稳定；冲突中的 `backend/ent/client.go` 及关联生成文件由最终 schema/Wire 图重新生成，不手工维护生成代码。
- `backend/cmd/server/VERSION` 最终保持 `0.1.213`。
- `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 字节完全一致，SHA-256 均为 `2865001838D1C9E8FEDC798E77742481BA7B6AC09774E417E889945BBB49A05A`。

### 冲突与最终解决方案

22 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`，最终索引无未解决路径：

- 版本与生成代码（2）：`backend/cmd/server/VERSION`、`backend/ent/client.go`。
- 管理、审核与支付服务（4）：`backend/internal/handler/admin/content_moderation_handler.go`、`backend/internal/service/admin_service.go`、`backend/internal/service/content_moderation.go`、`backend/internal/service/payment_config_service.go`。
- 网关、调度、计费与协议测试（12）：`backend/internal/handler/failover_loop.go`、`backend/internal/handler/gateway_handler.go`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/handler/openai_gateway_handler_test.go`、`backend/internal/server/routes/gateway.go`、`backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`、`backend/internal/service/gateway_count_tokens.go`、`backend/internal/service/gateway_scheduling.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/openai_gateway_grok.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_ws_protocol_forward_test.go`。
- 生产 Compose（1）：`deploy/docker-compose.yml`。
- 前端与组件测试（3）：`frontend/src/components/account/CreateAccountModal.vue`、`frontend/src/views/HomeView.vue`、`frontend/src/views/user/__tests__/PaymentView.spec.ts`。

最终兼容决策：

- 版本保持本地 `0.1.213`；Ent/Wire 以双方最终 schema 和依赖注入图重新生成，重复生成稳定。
- Claude Code 继续由严格协议头判定；上游 classifier 多 system/auto 兼容完整接入，测试补齐 `interleaved-thinking`。
- 首 Token 缓冲在读错误前先交付已读取 prelude；Anthropic 已观测 usage 后不再 failover，继续保留 pending/final outcome、部分响应、按用户串行扣费和 5 秒 usage task 超时。
- WSv2 encrypted reasoning/compaction 恢复测试使用 `stream:true`；本地非流式 HTTP 不走 WS、逐轮并发释放、结算与审计边界不变。
- CreateAccount 对全部 API-key 平台传递上游倍率探测开关；渠道调度补齐映射后模型限制，仍执行专属分组访问和本地请求模型计费约束。
- OpenAI 工具输出图片完整接入并补传图片尺寸能力参数；本地 Codex 图片工具禁用/桥接策略、媒体实际模型计费和图片结果统计边界保留。
- 内容审核接入上游代理配置与新增仓储参数，同时保留本地人工审核、Cyber 阻断和协调器回退。
- Home compact 测试补齐模型市场读取所需 app store mock；支付可见方式修复与本地充值赠送、支付增强共存。
- 生产 Compose 接入上游容器安全项，但继续使用 bind mount、`127.0.0.1:18080`、允许 HTTP upstream 的业务开关及既定资源参数；冲突解决后同步到 `docker-compose.sub2api.yml` 并验证字节一致。

### 刻意保留的二次开发功能

- 首 Token 超时、首响应/首有效输出区分、部分 usage 归因、已提交响应保护、增强 failover 和连续失败停调度。
- 普通分组严格按用户请求模型计费、Composite 显式别名价例外、严格缺价错误、媒体实际模型计费、按用户串行扣费和 5 秒 usage task 超时。
- Claude Code 严格客户端判定、全局上游模拟、Anthropic 采样参数过滤、API Key 请求头覆写、Codex namespace/图片工具策略及非流式 HTTP 不走 WS。
- 专属分组访问、账号模型白名单、增强账号测试、本地模型市场、充值赠送、内容人工审核/Cyber、附加换号状态码和表格稳定性。
- 生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性、本地 CI 门禁和实例资源保护参数。

### 验证记录

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./...` | 0 | Go 全量 unit 通过 |
| 同步前 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步前 | `CGO_ENABLED=0 go build -trimpath -o bin/server ./cmd/server` | 0 | 后端无 CGO 构建通过 |
| 同步前 | `corepack pnpm run lint:check`、`corepack pnpm run typecheck`、`corepack pnpm run test:run`、`corepack pnpm run build` | 0 | 前端 lint、类型检查、全量 Vitest 与生产构建均通过 |
| 生成 | Ent/Wire 重复生成及差异复核 | 0 | 重复生成稳定，无意外受控差异 |
| 适配 | 3 类定向后端失败复现与修复 | 1 / 0 | 修复测试辅助类型重名、内容审核新增仓储参数和 Failover API 新参数后，相关定向集合通过 |
| 适配 | `go test -tags=unit ./internal/handler/... ./internal/server/routes/... ./internal/service/...` | 0 | 受影响 handler、routes、service 全部通过 |
| 适配 | `corepack pnpm exec vitest run` 的 6 个受影响测试文件 | 0 | 6 个文件、60 个测试通过 |
| 同步后 | `go test -tags=unit ./...` | 0 | Go 全量 unit 通过 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath -o bin/server ./cmd/server` | 0 | 后端无 CGO 构建通过 |
| 同步后 | `corepack pnpm run test:run` | 0 | 前端全量 Vitest 通过 |
| 同步后 | `corepack pnpm run lint:check`、`corepack pnpm run typecheck` | 0 | lint 与类型检查通过 |
| 同步后 | `corepack pnpm run build` | 0 | 生产构建通过；仅有既有动态导入和大 chunk 非致命警告 |
| 部署静态检查 | `bash -n deploy/apple-container.sh` | 0 | Shell 语法检查通过 |
| 部署静态检查 | `bash deploy/tests/docker-compose-security-test.sh`、`bash deploy/tests/docker-runtime-resources-test.sh`、`bash deploy/test-caddyfile-cache.sh` | 0 | Compose 安全、运行时资源和 Caddy 缓存测试通过 |
| Apple fixture | `bash deploy/tests/apple-container-test.sh` | 1 | Windows Git Bash 的 `stat` 不支持 macOS `stat -f '%Lp'`，fixture 在权限校验阶段退出；未证明 macOS 生命周期失败 |
| Integration | `go test -tags=integration ./...` | 1 | 本机无 Docker 导致 3 个 Testcontainers 包 setup 失败；`tls.peet.ws` 探针访问被拒绝；未缓存 `go.opentelemetry.io/auto/sdk` 且从 `proxy.golang.org` 下载超时；其余可运行 integration 包通过 |
| 最终静态复核 | Compose SHA-256、祖先关系、冲突标记、未跟踪文件、意外删除、敏感路径、`git diff --check` | 0 | 双生产 Compose 字节一致，固定上游 SHA 已成为当前分支祖先，无未解决冲突或未跟踪文件；仅删除上游不再引用的 `666api.jpg` |

### 未验证项与残余风险

- `go test -tags=integration ./...` 未整体通过，Docker/Testcontainers、真实 PostgreSQL migration、TLS 指纹外部探针和需在线补齐的 OpenTelemetry 依赖仍需在具备 Docker 与稳定网络的隔离环境或远端 CI 复核。
- Apple container 生命周期 fixture 未在 macOS 原生 `stat` 环境完成；Windows Git Bash 的命令兼容问题不代表脚本在目标平台通过。
- 未运行 `-race` 或 `govulncheck`；当前结论不覆盖数据竞争和最新漏洞可达性。
- 未读取 `.env`，未启动依赖 PostgreSQL/Redis 的真实服务，未执行本地健康检查或真实数据库迁移。
- 未使用真实 OpenAI、Anthropic、Grok、支付、S3、SMTP 或安全审计服务凭据；相关路径由单元、组件和契约测试覆盖。
- 未执行 push、PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-08-09 同步至 48eb3766d

- 执行时间：2026-08-09T23:21:00+08:00
- 执行状态：同步分支完整合并并通过当前本机可执行的全量 unit、静态检查与构建验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`fbd42f0d5ff1dc39ec60e7bd2ce081b03464f285`
- 上游代码合并提交：`9c63ee6f6917e2c945a9e738077de405f5c4da0b`
- 最后一个代码/测试提交：`9c63ee6f6917e2c945a9e738077de405f5c4da0b`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`7e2e9ba05026b7126318aa0754c1afa0ac00bc58`
- `UPSTREAM_NEW_SHA`：`48eb3766d2da817b171b45bb3036d42575e42b8f`
- merge-base：`7e2e9ba05026b7126318aa0754c1afa0ac00bc58`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`48eb3766d2da817b171b45bb3036d42575e42b8f`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决 49 个文本冲突，复核自动合并路径的二开语义，补齐合并后的测试契约并重复生成 Ent/Wire；上游祖先关系由独立 merge commit 完整保留
- 备份分支：`backup/pre-upstream-sync-20260809-221027-fbd42f0d5`
- 同步分支：`sync/upstream-20260809-48eb3766d`

### 上游提交处置

本次固定范围共 223 个提交，其中 52 个 merge commit、171 个 non-merge commit。223 个提交均通过完整 merge 保留祖先关系并记为 `Applied`；下表重点提交组同时按本地二次开发边界记为 `Applied + Overridden`。重点组是总范围的子集，数量不可与 223 重复相加。无 `Already Applied`、`Skipped`、`Deferred` 或未解决 `Conflict`。

| 上游提交集合 | 状态 | 内容与处置 |
| --- | --- | --- |
| `7e2e9ba05..48eb3766d`（223 个） | Applied | 完整接入支付/订阅并发安全、Codex 身份与版本同步、腾讯和阿里云 Captcha、OAuth pending 安全、Channel Monitor V2、Grok 搜索/视频/语音/实时链路、上游响应模型诊断、Responses/Anthropic 协议修复、邮箱域名注册额度、Gemini 图片计费、依赖安全及相关测试 |
| `aac53afe0`、`68d8f122e`、`48eb3766d` | Applied + Overridden | 保留上游版本提交的祖先关系，最终版本继续使用本地较新的 `0.1.214`，不回退到上游 `0.1.171`、`0.1.172` 或 `0.1.173` |
| `2eb24814f`、`4c4ff3638`、`c899c8cf3`、`2d3e84520`、`dbb42881c` 及对应 merge commit | Applied + Overridden | 接入 Codex 官方 release 版本同步、启动防抖、统一出站版本身份和 `codex-tui` 默认身份；继续保留本地 Codex CLI 模拟、图片工具策略、App Server 识别和管理员 UA 只贡献非版本指纹的边界 |
| `e592c5f9e`、`635a27189`、`26e0a8932`、`02e50cc22`、`899157487` 及对应 merge commit | Applied + Overridden | 接入腾讯/阿里云 Captcha、pending OAuth 创建账号门禁和账号接管防护；继续保留公开注册页 401 不强制跳登录、安全 storage 降级、刷新令牌并发/换号保护，并限制仅从 auth store 恢复的 pending 会话复用普通注册残留邀请码 |
| `a58048ac3` 至 `59b5ac545`、`0d98176c5`、`9f3ee38d4`、`64d1ebe4a`、`800533574`、`3c22aeeb3`、`825f9c78d`、`04d9eeaf0` 及对应 merge commit | Applied + Overridden | 完整接入 Channel Monitor V2 聚合、模式开关、隐私默认、渐进回填、错误去重和用户/管理界面；默认继续使用 V1，可显式切换 V2，同时把本地图片分组成功率接回 V1/V2 用户展示 |
| `74249b8fe` 至 `79df1647d`、`25d2b03e` 至 `245d06960`、`d7c9e7167` 至 `72a56f862`、`d0767eab9` 至 `fb0475656` 及对应 merge commit | Applied + Overridden | 接入 Grok 模型映射、密码/SSO/RT 授权、搜索、图片、视频、Voice TTS/STT、Realtime、free/P2 配额、媒体计费和管理端多模式测试；继续保留严格按用户请求模型计费、实际媒体结果结算、失败关闭安全边界及本地默认测试提示词 |
| `30d2589ef`、`0b9f40e23`、`e2652eb85`、`f3c94d209`、`db0bff82c`、`c33c3208e`、`cbf2be05a`、`6e34fb09c` 及对应 merge commit | Applied + Overridden | 接入 WS lease 丢失终态、未结算 usage 保留、金额量化、Responses 工具 schema、上游响应模型审计、流内降载恢复、图片请求与客户端 context 解耦和响应模型观测优化；继续保留首 Token/部分输出禁止重试、客户端断连和成功审计、请求体及时释放、WS 逐轮并发/计费/终态错误及响应模型仅诊断不改计费来源 |
| `f970bd48c`、`7d38e6712`、`e687ca3e9`、`99b357083`、`38081ef72`、`db725a775` 及对应 merge commit | Applied + Overridden | 接入请求取消停止调度、稀疏流量失败 streak、系统日志落库退避、每日配额午夜重置、刷新令牌竞态和订阅续费串行化；继续保留按用户串行扣费和 5 秒 usage task 超时，不为匹配 worker 数量扩大数据库连接池 |

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `9c63ee6f6917e2c945a9e738077de405f5c4da0b`；双亲分别为同步前本地 SHA `fbd42f0d5ff1dc39ec60e7bd2ce081b03464f285` 与固定上游 SHA `48eb3766d2da817b171b45bb3036d42575e42b8f`。
- 写入两份台账前，代码、配置、资源和测试相对 `LOCAL_PRE_SYNC_SHA` 修改 588 个文件：新增 161 个、修改 424 个、删除 3 个，共增加 50944 行、删除 3840 行。
- 删除文件均为上游已移除的合作方资源：`assets/partners/logos/AICodeMirror.jpg`、`assets/partners/logos/anpin.jpg`、`assets/partners/logos/unity2.png`；未删除本地业务文件。
- `backend/cmd/server/VERSION` 最终保持 `0.1.214`。
- Ent 在隔离恢复模块中连续两次生成的摘要均为 `5CFD9B182E07A04B5AB01925E121B0D517C4660B4232A18B8E584F0309812F5D`；Wire 连续生成摘要均为 `BA7243E941F7541C1DC43AA9B1622656B25B20A4CD31C842A5E4623B1924E88A`。临时恢复目录已删除。
- `go mod tidy` 将测试直接使用的 `github.com/go-playground/validator/v10` 提升为直接依赖，并保留 Wire 生成器需要的 `github.com/google/subcommands v1.2.0` 间接依赖及完整校验和。
- `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 字节完全一致，SHA-256 均为 `2865001838D1C9E8FEDC798E77742481BA7B6AC09774E417E889945BBB49A05A`。

### 冲突与最终解决方案

49 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`，最终索引无未解决路径。冲突集中在版本与 Go 依赖、Ent/Wire 生成代码、设置与账号 schema、OpenAI/Grok 网关和 WS、计费与 usage、认证/Captcha、Channel Monitor、管理端设置及用户页面。

最终兼容决策：

- 版本保持本地 `0.1.214`；Ent/Wire 以双方最终 schema 和依赖注入图重新生成，不手工拼接生成代码。
- 设置系统同时保留本地默认测试提示词、每日签到、本地模型市场、缓存/测活配置，并接入上游 Captcha、Codex 版本同步、账号调度阈值和 Channel Monitor V2 设置。
- OpenAI/Grok/WS 同时保留首 Token 与部分输出禁止重试、客户端断连和成功审计、请求体及时释放、逐轮并发释放/计费/终态错误，以及严格按用户请求模型计费；上游响应模型仅用于诊断。
- Grok 搜索、图片、视频、语音和实时模式完整接入；搜索附加计费、视频按完成结果结算、音频定价和免费额度门禁与本地计费边界共存。
- 认证页接入腾讯/阿里云 Captcha 和 pending OAuth 安全修复；公开注册页 401 仍不强制跳登录，storage 不可用时安全降级，刷新令牌并发换号保护保留，陈旧普通注册邀请码不进入仅由 auth store 恢复的 pending OAuth 创建账号请求。
- Channel Monitor 保留 V1/V2 可切换模式，V2 使用上游隐私默认和渐进回填；本地图片分组成功率继续在用户端展示。
- 生产 Compose 未切换到命名卷，继续保持 bind mount、`127.0.0.1:18080`、允许 HTTP upstream 的业务开关、4 vCPU/8 GiB 实例参数和按用户串行扣费/5 秒 usage task 超时。

### 刻意保留的二次开发功能

- 首 Token 超时、首响应/首有效输出区分、部分输出后禁止重试、客户端断连与成功审计、增强 failover、连续失败停调度和大请求体及时释放。
- 普通分组严格按用户请求模型计费、Composite 显式别名价例外、严格缺价错误、媒体实际模型计费、图片分组成功率、按用户串行扣费和 5 秒 usage task 超时。
- Claude/Codex 严格客户端与协议策略、API Key 请求头覆写、非流式 HTTP 不走 WS、WS 逐轮并发释放/结算/终态错误，以及上游响应模型仅用于诊断。
- 可配置账号测试默认提示词、Grok 多测试模式、本地 `/models` 模型市场、每日签到、内容人工审核/Cyber、附加换号状态码、公开注册页和 storage/刷新令牌容错。
- 生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性、本地 CI 门禁和实例资源保护参数。

### 验证记录

验证使用 Go 1.26.3、Node 24.15.0、corepack pnpm 9.15.9 和 golangci-lint 2.9.0。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | 审批阶段记录的 Go unit/lint/build 与前端 lint/typecheck/Vitest/build 基线 | 0 | 同步前 `main` 基线通过，作为本次新增失败判断基准 |
| 合并编译 | `go test -tags=unit -run '^$' ./...` | 0 | 全包编译通过 |
| 生成 | Ent 隔离恢复模块连续两次生成与摘要复核 | 0 | 两次摘要均为 `5CFD9B182E07A04B5AB01925E121B0D517C4660B4232A18B8E584F0309812F5D` |
| 生成 | Wire 连续生成与摘要复核 | 0 | 两次摘要均为 `BA7243E941F7541C1DC43AA9B1622656B25B20A4CD31C842A5E4623B1924E88A` |
| 适配 | `corepack pnpm exec vitest run src/i18n/__tests__/staticKeys.spec.ts src/views/auth/__tests__/EmailVerifyView.spec.ts` | 0 | 2 个文件、19 个测试通过；修复翻译键层级和 pending OAuth affiliate 来源边界 |
| 同步后 | `go test -tags=unit ./...` | 0 | Go 全量 unit 通过 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath -o bin/server.exe ./cmd/server` | 0 | 后端无 CGO 构建通过；验证产物已删除 |
| 同步后 | `corepack pnpm run test:run` | 0 | 前端全量 Vitest 通过；仅输出既有模拟异常和组件告警 |
| 同步后 | `corepack pnpm run lint:check`、`corepack pnpm run typecheck` | 0 | lint 与类型检查通过 |
| 同步后 | `corepack pnpm run build` | 0 | 生产构建通过；仅有既有动态导入、大 chunk、Browserslist 数据和 Node shell 参数非致命警告 |
| 最终静态复核 | Compose SHA-256、上游祖先关系、冲突标记、未跟踪文件、临时目录、版本、`git diff --check` | 0 | 双生产 Compose 一致，固定上游 SHA 为 merge commit 第二父提交，无未解决冲突、Ent 临时目录或意外文件 |

### 未验证项与残余风险

- 未运行 `go test -tags=integration ./...`、`-race` 或 `govulncheck`；当前结论不覆盖真实 PostgreSQL/Redis 集成、数据竞争和最新漏洞可达性。
- 未启动依赖 PostgreSQL/Redis 的本地服务，未执行真实数据库迁移或本地健康检查。
- 未使用真实 OpenAI、Anthropic、Grok、Captcha、支付、S3、SMTP 或安全审计服务凭据；相关路径由单元、组件和契约测试覆盖。
- 未在 Node 20 / Go 1.26.5 的 CI 同构环境复核；本地 Node 24.15.0 / Go 1.26.3 验证已通过。
- 未执行 push、PR、部署、远程服务器访问、容器重启或生产数据操作。

## 2026-08-13 同步至 fbfdcef81

- 执行时间：2026-08-13T23:12:53+08:00
- 执行状态：同步分支完整合并并通过当前本机可执行的全量 unit、静态检查与构建验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`bb10cbc67d46664debfa4b4e09299845c10d8832`
- 上游代码合并提交：`fc9c1d839436b7e0d358e9ff6facf1cf503e646e`
- 最后一个代码/测试提交：`a39851432c31141416c0a86c656022199d6215d5`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`48eb3766d2da817b171b45bb3036d42575e42b8f`
- `UPSTREAM_NEW_SHA`：`fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`
- merge-base：`48eb3766d2da817b171b45bb3036d42575e42b8f`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`
- 集成策略：在隔离同步分支使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA，逐文件解决 21 个文本冲突，复核自动合并路径的二开语义，补齐响应模型计费与 Teleport 测试契约，并在临时模块中重复生成 Ent、在活动模块中重复生成 Wire；上游祖先关系由独立 merge commit 完整保留
- 备份分支：`backup/pre-upstream-sync-20260813-220543-bb10cbc67`
- 同步分支：`sync/upstream-20260813-fbfdcef81`

### 上游提交处置

本次固定范围共 111 个提交，其中 43 个 merge commit、68 个 non-merge commit。111 个提交均通过完整 merge 保留祖先关系；98 个记为 `Applied`，12 个因本地兼容边界记为 `Applied + Overridden`，1 个纯 CI 重试提交因 patch-id 已等价存在记为 `Already Applied`。无 `Skipped`、`Deferred` 或未解决 `Conflict`。

为保证每个提交都有唯一处置状态，全集按以下规则划分：`48eb3766d..fbfdcef81` 中除下表显式列出的 13 个 SHA 外，其余 98 个 SHA 全部为 `Applied`；下表 SHA 分别为 `Applied + Overridden` 或 `Already Applied`。可使用 `git rev-list --reverse 48eb3766d..fbfdcef81` 与下表做集合差复核。

| 上游提交集合 | 数量 | 状态 | 内容与处置 |
| --- | ---: | --- | --- |
| `48eb3766d..fbfdcef81` 中除下列例外外的全部提交 | 98 | Applied | 完整接入 Responses/WS 错误处理与 TTFT、分卷备份、API Key 输入校验、Codex 指纹收敛、Grok 订阅档位与 4.6、原生 `x_search`、搜索/音频计费、账号用量刷新、渠道缓存失效、定时备份 leader 锁、文档/赞助资源及相关测试 |
| `7045f89de` | 1 | Applied + Overridden | 接入 Pool 认证失败重试；同时保留本地 Pool Mode 自定义重试状态码、附加换号状态码和语义终止保护，避免扩大普通账号重试边界 |
| `9096492b5`、`b689e5b40`、`33351c7bc`、`e5b325e48` | 4 | Applied + Overridden | 保留响应模型观测、使用日志、mismatch 诊断与兼容输入字段；普通分组仍严格按用户请求模型扣费，服务层将历史 `billing_model_source` 归一为 `requested`，管理页不暴露切换计费来源的入口 |
| `6564d376e` | 1 | Applied + Overridden | 接入 Cyber 事件 group/model scope；继续要求全局风险控制与内容审核启用、Mode 非 `off` 且命中 scope，运行时快照读取失败时安全跳过事后记录 |
| `814ecfba7` | 1 | Applied + Overridden | 接入分组平台变化后的渠道缓存失效；AdminService 同时保留本地自动托管测试和连续失败 streak 依赖 |
| `f3d949107`、`b830bc14d`、`fd82dfd52` | 3 | Applied + Overridden | 接入分组逐模型定价、视频计费模式和长上下文开关；本地固定 `Group → Channel → 全局价格` 优先级，数据库真实分组默认开启且显式关闭有效，手工旧对象继续兼容默认开启，Grok 只服从分组开关 |
| `ef4f99f29`、`0e82efe48` | 2 | Applied + Overridden | 保留上游版本提交的祖先关系，最终版本继续使用本地较新的 `0.1.215`，不回退到上游 `0.1.175` 或 `0.1.176` |
| `da283854f` | 1 | Already Applied | 仅重试因 registry timeout 失败的 CI；patch-id 等价内容已存在，完整 merge 仅保留祖先关系，不引入代码变化 |

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `fc9c1d839436b7e0d358e9ff6facf1cf503e646e`；双亲分别为同步前本地 SHA `bb10cbc67d46664debfa4b4e09299845c10d8832` 与固定上游 SHA `fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`。
- 写入两份台账前，代码、资源和测试相对 `LOCAL_PRE_SYNC_SHA` 修改 195 个文件：新增 33 个、修改 161 个、删除 1 个，共增加 10365 行、删除 676 行。
- 唯一删除文件为上游已替换的合作方资源 `assets/partners/logos/haoai.svg`；同时新增 `duckip.png` 与 `swiftprox.png`，未删除本地业务文件。
- `backend/cmd/server/VERSION` 最终保持 `0.1.215`。
- PR #9 首轮 Security Scan 在 2026-08-13 新更新的 `GHSA-2v37-7h3g-55p8` 上失败；依赖修复提交 `a39851432c31141416c0a86c656022199d6215d5` 通过 pnpm override 将 PostCSS/Vue 生产依赖链中的 `nanoid` 从 `3.3.17` 精确升级到修复版 `3.3.18`，未新增审计例外。
- Ent 在两个隔离的完整 backend 模块中各生成 371 个文件，两轮逐文件 SHA-256 完全一致，且与活动 `backend/ent` 树一致；Wire 连续两次生成哈希一致。
- `deploy/` 相对同步前基线无受控差异；`deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 字节一致，SHA-256 均为 `2865001838D1C9E8FEDC798E77742481BA7B6AC09774E417E889945BBB49A05A`。

### 冲突与最终解决方案

21 个文本冲突均逐文件解决，无整文件采用 `ours` 或 `theirs`，最终索引无未解决路径。冲突集中在版本、Ent/Wire、分组/渠道 schema 与缓存、计费和 usage、OpenAI/Grok 转发、内容审核以及管理端定价与用量展示。

最终兼容决策：

- 版本保持本地 `0.1.215`；Ent/Wire 根据最终 schema 和依赖注入图生成。直接在活动 Ent 目录生成时 Windows 两次因 user-mapped section 占用中断，恢复半生成文件后改在两个临时完整模块中生成和比对，活动树未保留半生成结果。
- 普通分组继续严格按用户请求模型计费；Composite 保留显式别名渠道价例外。上游响应模型仍会记录到使用日志并产生 mismatch 诊断，但不反向改变用户扣费。
- 分组逐模型定价优先于渠道和全局价格；渠道缓存失效与本地自动托管测试、连续失败 streak 依赖同时保留。
- 长上下文增加“是否来自真实分组配置”的内部状态：真实数据库分组默认开启且可显式关闭；未携带真实分组的旧手工对象继续兼容开启，Grok 不受 OpenAI 账号开关否决。
- Cyber 事件继续受风险控制、内容审核、Mode、group/model scope 四层门禁；快照加载失败不写入事后副作用。
- 附加换号状态码、Pool Mode 自定义重试状态码、按用户串行扣费和 5 秒 usage task 超时继续保留。
- 管理端用量请求 ID 默认隐藏但可从列设置启用；测试按 `ColumnSettingsDropdown` 的 Teleport-to-body 实际挂载位置取按钮，避免把测试选择器失效误判为产品回归。

### 刻意保留的二次开发功能

- 首 Token/可见输出 TTFT、部分输出后禁止重试、客户端断连与成功审计、增强 failover、连续失败停调度、大请求体及时释放以及附加换号状态码。
- 普通分组严格请求模型计费、Composite 显式别名价例外、响应模型仅诊断、分组定价优先级、严格缺价、媒体实际模型计费、按用户串行扣费和 5 秒 usage task 超时。
- Claude/Codex 本地协议策略、Codex 图片工具策略、非流式 HTTP 不走 WS、WS 逐轮并发/结算/审计，以及本地自动托管账号恢复测试。
- 本地 `/models` 模型市场、每日签到、内容人工审核/Cyber、表格和 Teleport 浮层稳定性。
- 生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性和实例资源保护参数。

### 验证记录

验证使用 Go 1.26.3、Node 24.15.0、corepack pnpm 9.15.9 和 golangci-lint 2.9.0。

| 阶段 | 命令 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./...`、`golangci-lint run --timeout=30m ./...`、`CGO_ENABLED=0 go build` | 0 | Go 全量 unit、静态检查与构建通过 |
| 同步前 | `corepack pnpm install --offline --frozen-lockfile`、`lint:check`、`typecheck`、`test:run`、`build` | 1 / 0 | 离线缓存最初缺少 `nanoid 3.3.17`；联网冻结安装补齐缓存后通过，锁文件未变化；前端检查和构建通过 |
| 合并编译 | `go test -tags=unit -run '^$' ./...` | 0 | Go 全包编译通过 |
| 适配 | 计费、分组定价、长上下文、Cyber、故障转移、Grok、备份、Codex 指纹和调度阈值定向测试 | 0 | 受影响后端契约通过 |
| 生成 | 两个隔离 backend 模块分别执行 `go generate ./ent` 并与活动树逐文件比对 | 0 | 两轮各 371 个文件，轮次间差异 0，与活动树差异 0 |
| 生成 | Wire 连续两次生成与哈希复核 | 0 | 两次结果一致 |
| 同步后 | `go test -tags=unit ./...` | 1 / 0 | 首轮在大量测试日志中一次退出 1；随后使用 `go test -tags=unit -json ./...` 完整复验退出 0，无可复现失败包 |
| 同步后 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步后 | `CGO_ENABLED=0 go build -trimpath -o bin/server-sync.exe ./cmd/server` | 0 | 后端无 CGO 构建通过；验证产物已删除 |
| 同步后 | `corepack pnpm install --offline --frozen-lockfile` | 0 | 冻结安装通过，锁文件未变化 |
| 同步后 | `corepack pnpm run lint:check`、`corepack pnpm run typecheck` | 0 | lint 与类型检查通过 |
| 同步后 | `corepack pnpm run test:run` | 1 / 0 | 首轮仅 `UsageView.spec.ts` 因未从 Teleport 容器查找按钮失败；修正测试后定向 13 项及全量 Vitest 均通过 |
| 同步后 | `corepack pnpm run build` | 0 | 生产构建通过；仅有既有 Browserslist 数据和大 chunk 非致命警告 |
| PR 首轮 | PR #9 的 push/PR 双触发 `Security Scan / frontend-security` | 1 | 两个 job 均发现 `nanoid 3.3.17` 命中高危 `GHSA-2v37-7h3g-55p8`；公告要求升级到 `3.3.18` 或更高版本 |
| 安全修复 | `pnpm` 精确 override、离线冻结安装、`pnpm audit --prod --audit-level=high` 与 `check_pnpm_audit_exceptions.py` | 0 | 所有生产依赖链解析为 `nanoid 3.3.18`，仓库审计例外检查通过，未新增临时豁免 |
| 安全修复 | `corepack pnpm run lint:check`、`typecheck`、`test:run`、`build` | 0 | 前端 lint、类型检查、全量 Vitest 与生产构建通过 |
| PR 复验 | PR #9 对 `a39851432` 的 push/PR 双触发 CI 与 Security Scan | 0 | 两组 backend-security/govulncheck、frontend-security、Node 20 前端、golangci-lint、macOS shell 及 Go 1.26.5 Unit/Integration 全部通过；CLA 两项按设计 skipped，PR 状态为 `clean` |
| 最终静态复核 | `git diff --check`、索引冲突项、真实冲突标记、敏感路径、Compose 哈希、祖先关系和临时产物检查 | 0 | 无未解决冲突、未暂存差异或敏感文件；固定上游 SHA 为 merge commit 第二父提交；双生产 Compose 一致，临时 Ent 目录和验证二进制已删除 |

### 未验证项与残余风险

- 本机无可用 Docker，未在本地运行 Testcontainers、真实 PostgreSQL migration 或 Compose 启动/健康检查；PR #9 的 Ubuntu CI 已通过 `make test-integration`。
- 未运行 `-race`；PR #9 的 Security Scan 已通过 `govulncheck ./...`，但当前结论仍不覆盖数据竞争。
- 未读取 `.env`，因此实例专用资源参数仅能确认本次同步未修改 `deploy/` 和两份生产 Compose，不能声明已在运行实例中复核。
- 未使用真实 OpenAI、Anthropic、Grok、S3、SMTP 或安全审计服务凭据；相关路径由单元、组件和契约测试覆盖。
- PR #9 已在 Node 20 / Go 1.26.5 环境通过 CI；本地验证环境为 Node 24.15.0 / Go 1.26.3。
- 已推送同步分支并创建 PR #9；未执行部署、远程服务器访问、容器重启或生产数据操作。

## 2026-08-18 同步至 5253bb72b

- 执行时间：2026-08-18T04:18:00+08:00
- 执行状态：同步分支完整合并固定上游范围；完成二开兼容提交、验证修复和当前环境可执行的后端、前端、Canvas、迁移及部署静态验证；本记录提交后按 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`832e20ed7fb12b4b70b41f8ad827749fb9b4e5ff`
- 上游代码合并提交：`18268da8bdbc0ce96d1e4d1786a5a961ced9ff01`
- 最后一个代码/测试提交：`cfa0fd3e2bc8dcbb78b7fce5da00656b41a0d6b3`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`
- `UPSTREAM_NEW_SHA`：`5253bb72b08586c619f5b369b2f1dc7547b0e97a`
- merge-base：`fbfdcef8184ae4b2e224d5cfc47cf1d0e3742710`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`5253bb72b08586c619f5b369b2f1dc7547b0e97a`
- 集成策略：在隔离同步分支创建备份分支后，使用 `git merge --no-ff --no-commit` 完整合并固定上游 SHA；逐文件解决 21 个文本冲突、39 个冲突块，复核自动合并路径的二开语义，随后以独立提交补齐迁移兼容、回归测试和同步后静态检查修复；Ent/Wire 在隔离 worktree 中按顺序重复生成并确认零差异，保留上游祖先关系
- 备份分支：`backup/pre-upstream-sync-20260818-022958-832e20ed7`
- 同步分支：`sync/upstream-20260818-5253bb72b`

### 上游提交处置

本次固定范围共 28 个提交，其中 9 个 merge commit、19 个 non-merge commit。28 个提交均通过完整 merge 保留祖先关系；4 个记为 `Applied`，23 个因本地兼容边界记为 `Applied + Overridden`，1 个因 patch-id 等价内容已存在记为 `Already Applied`。无 `Skipped`、`Deferred` 或未解决 `Conflict`。下表集合互不重叠并覆盖固定范围内全部 SHA。

| 上游提交集合 | 数量 | 状态 | 内容与处置 |
| --- | ---: | --- | --- |
| `11e1e2288`、`51016bb03` | 2 | Applied | 接入 Go builder 镜像版本与 Docker 构建链更新，完整保留上游实现；本地部署文件未被本次同步改写。 |
| `22fc0cdbf`、`396a9d113` | 2 | Applied | 接入 OpenAI Fast/Flex 策略文案与对应前端合并，完整保留上游行为。 |
| `baeac1f3d` | 1 | Already Applied | 版本同步提交的内容在本地 `0.1.222` 已等价存在；完整 merge 仅保留祖先关系，不重复改变版本。 |
| `9662cff2e`、`a8b9ea22b`、`1d3b9665c` | 3 | Applied + Overridden | 接入远程 compaction v2 端点及 native/legacy 分流；最终保留本地端点规范化、legacy `/responses/compact` 兼容和请求体生命周期边界。 |
| `cb7b03795`、`89d826be2`、`45dcce0e4`、`c204d33b0` | 4 | Applied + Overridden | 接入分组日用量汇总及测试修复；新增 226 前向迁移，使当前开放日 INSERT 遇回填锁快速放行，历史日期和跨午夜事务仍等待水位锁并回退水位。 |
| `8219dcfc8`、`8ae6d8f67`、`fce41e318`、`4d9fedee2`、`073e92d17` | 5 | Applied + Overridden | 接入 Codex turn-state、session beta 能力、指纹 opt-in 及测试；继续保留本地首输出守卫、请求体释放、WS 逐轮真实开始时刻、结算和断连排水。 |
| `901a0439f`、`4b667ccd4`、`e72854538`、`e330c243a`、`7cdca9e49`、`10c8b7020`、`c38c5beef`、`ab28a2d10`、`6bf335965` | 9 | Applied + Overridden | 接入 Kimi/Zhipu/DeepSeek 一等支持、分组入口、缺陷修复、文案及合并语义；继续严格按用户请求模型计费，CN `claude-*` 无显式价格时不套内置价，成功请求保留零费用 usage/幂等审计并返回 `ErrModelPricingUnavailable`。 |
| `9f24a5530`、`5253bb72b` | 2 | Applied + Overridden | 接入渠道模型分时倍率定价；保留 Composite 仅在显式渠道价存在时按公开别名计费，以及普通分组严格请求模型计费和严格缺价边界。 |

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `18268da8bdbc0ce96d1e4d1786a5a961ced9ff01`；双亲分别为同步前本地 SHA `832e20ed7fb12b4b70b41f8ad827749fb9b4e5ff` 与固定上游 SHA `5253bb72b08586c619f5b369b2f1dc7547b0e97a`。
- 在写入本次两份台账前，相对 `LOCAL_PRE_SYNC_SHA` 共修改 181 个跟踪文件：新增 45 个、修改 136 个、删除 0 个，共增加 13532 行、删除 707 行；`deploy/` 下生产 Compose 未产生受控差异。
- 本地提交映射：`f07e6769d5`（迁移兼容、回归测试与二开台账适配）、`cfa0fd3e2`（同步后静态检查修复）；本记录提交自身的 SHA 在最终总结中报告。
- `backend/cmd/server/VERSION` 保持本地 `0.1.222`。
- Ent/Wire 直接在活动目录生成时受 Windows user-mapped section 锁定；恢复由生成器留下的半成品后，在同一 merge HEAD 的隔离 worktree 中串行生成 Ent 和 Wire，生成后及第二次复核均为零差异，未把临时文件带回活动树。
- `deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 字节完全一致，SHA-256 均为 `2865001838D1C9E8FEDC798E77742481BA7B6AC09774E417E889945BBB49A05A`。

### 冲突与最终解决方案

21 个文本冲突共 39 个冲突块均已逐块处理，无整文件采用 `ours` 或 `theirs`，最终索引无未解决路径。冲突集中在版本/Ent/Wire、分组用量与迁移、计费和定价、OpenAI Responses/compact、HTTP/WS 转发、CN Provider 以及管理端账号和渠道组件。

最终兼容决策：

- native compaction v2 与 legacy `/responses/compact` 保持独立路由；请求模型、端点规范化、首输出后释放请求体、成功审计、部分 usage 和客户端断连排水继续按本地边界执行。
- 普通文本严格按用户请求模型计费；上游响应模型只作日志和诊断。Composite 仅在显式渠道价存在时按公开别名计费，其他场景不改变具体模型计费来源。
- CN Provider 的 `claude-*` 在没有显式分组/渠道价时不套 Claude 内置价；成功请求仍写零费用 usage 与幂等记录，再返回 `ErrModelPricingUnavailable`。
- WS 保留逐轮并发释放、真实 `StartedAt` 计价、terminal payload、断连排水和每轮结算；turn-state 出站守卫只在客户端成功写入后登记。
- 分组日汇总的 226 迁移不修改已发布的 222/223，当前开放日锁竞争快速放行，历史和跨午夜写入继续串行化，避免回填水位丢失。
- 版本、生产 bind mount、仅回环暴露、HTTP upstream 业务开关和实例资源参数约束均未被本次同步放宽或删除。

### 刻意保留的二次开发功能

- 首 Token/可见输出归因、增强 failover、连续失败停调度、大请求体及时释放、客户端断连排水和成功审计。
- 普通分组请求模型计费、Composite 显式别名价例外、严格缺价、媒体实际模型计费、按用户串行扣费和 5 秒 usage task 超时。
- Claude/Codex 本地协议清洗、API Key 请求头覆写、Codex 图片工具策略、非流式 HTTP 不走 WS、WS 逐轮结算与审计。
- 专属分组访问、CN Provider 账号/配额能力、本地模型市场、内容审核/Cyber、渠道监控与上游站点同步。
- 生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性和 4 vCPU/8 GiB 资源保护参数。

### 验证记录

| 阶段 | 命令或检查 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | 后端 `go test -tags=unit ./...`、`golangci-lint run --timeout=30m ./...`、CGO 关闭构建；前端 lint/typecheck、1681 项 Vitest、build；Canvas typecheck、34 项 Vitest、build | 0 | 同步前基线验证通过，作为本次比较基准 |
| 生成 | 隔离 worktree 中 `go generate ./ent`、`go generate ./cmd/server`，各重复生成并比较工作树 | 0 | Ent/Wire 生成稳定，第二轮及与 merge HEAD 比对均零差异 |
| 兼容回归 | `go test -tags=unit ./migrations ./internal/repository -run GroupUsageRollup -count=1` | 0 | 迁移静态断言和 repository 定向测试通过 |
| 兼容回归 | 临时 PostgreSQL 上的 `TestGroupUsageRollupTrigger...` 及完整迁移重复执行验证 | 0 | 当前日 5 秒窗口非阻塞，历史/跨午夜写入等待并正确失效水位；测试容器和临时数据已清理 |
| 同步后后端 | `go test -tags=unit ./...` | 0 | 全包通过，耗时 198.380 秒 |
| 同步后后端 | `golangci-lint run --timeout=30m ./...` | 1 → 0 | 首轮发现 gofmt 和未使用 helper；删除无调用函数并格式化后复跑为 `0 issues` |
| 同步后后端 | `CGO_ENABLED=0 go build ./...`；`go test -tags=unit ./internal/service -count=1` | 0 | 构建和修复后的 service 定向回归通过 |
| 同步后前端 | `corepack pnpm@9.15.9 install --frozen-lockfile`、`lint:check`、`typecheck`、`test:run`、`build` | 1 → 0 | 首轮未使用函数和旧 Grok 源码断言导致失败；修复后 245 个测试文件、1713 项通过，lint/typecheck/build 全部通过 |
| 同步后 Canvas | `corepack pnpm@9.15.9 install --frozen-lockfile`、`typecheck`、`test`、`format:check`、`build` | 0 | 8 个测试文件、34 项通过，类型、格式和生产构建通过 |
| 部署静态 | `bash -n` 五个部署脚本；Compose security/resources、Caddy cache、GitHub token 隔离测试 | 0 | 使用 Git Bash 临时 fixture，未启动 Docker、未访问网络或生产配置；全部通过 |
| Apple fixture | `deploy/tests/apple-container-test.sh` | 1 | Windows Git Bash 的 `stat -f '%Lp'` 不兼容 macOS 权限参数；未据此判断 macOS 生命周期实现失败 |
| Compose 一致性 | 两份生产 Compose SHA-256 与字节比较 | 0 | 两份文件字节一致，哈希均为 `286500...49A05A` |
| 最终静态复核 | `git diff --check`、索引冲突项、冲突标记、意外删除、敏感路径、固定上游祖先关系和工作树检查 | 0 | 无空白错误、未解决冲突、意外删除或敏感文件；固定上游 SHA 已成为同步分支祖先 |

### 未验证项与残余风险

- 未运行完整 `go test -tags=integration ./...`：本机没有可用 Docker/Testcontainers 环境；真实 Redis、全量 PostgreSQL schema、外部 TLS 探针和本地服务健康检查仍需隔离 CI 复核。本次仅在临时 PostgreSQL 上验证了分组汇总迁移和触发器。
- 未运行 `-race`，本机缺少 GCC；未运行 `govulncheck`，本记录不覆盖数据竞争或最新依赖漏洞可达性。
- Apple container 生命周期未在 macOS 原生环境执行；Windows fixture 失败仅反映 `stat` 命令兼容性。
- 未读取 `.env`，未使用真实 OpenAI、Anthropic、Grok、支付、S3、SMTP 或安全审计凭据，未执行真实计费、消息发送、push、PR、部署、远程服务器访问、容器重启或生产数据操作。
- 本机 Go 为 1.26.3、Node 为 24.15.0；仓库 CI 基线分别为 Go 1.26.6 与 Node 20，版本差异仍需 CI 复核。

## 2026-08-21 同步至 2bc139ab5

- 执行时间：2026-08-21T03:24:47+08:00
- 执行状态：同步分支完整合并固定上游范围并通过可执行的本地验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`3bf1f2ee3fb0af82110ae539a2e1382e1ac44fe6`
- 上游代码合并/最后一个代码提交：`e7b2c6db05751accafdc0894baac1faa21d209be`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`5253bb72b08586c619f5b369b2f1dc7547b0e97a`
- `UPSTREAM_NEW_SHA`：`2bc139ab527b4a687546d145dc7bb9063cf14510`
- merge-base：`5253bb72b08586c619f5b369b2f1dc7547b0e97a`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`2bc139ab527b4a687546d145dc7bb9063cf14510`
- 集成策略：在隔离同步分支执行 `git merge --no-ff --no-commit 2bc139ab527b4a687546d145dc7bb9063cf14510`，逐文件解决 30 个文本冲突，复核自动合并路径的二开语义，重生成 Ent/Wire，并将兼容调整和二开台账纳入明确的 merge commit
- 备份分支：`backup/pre-upstream-sync-20260821-020901-3bf1f2ee3`
- 同步分支：`sync/upstream-20260821-2bc139ab5`

### 上游提交处置

固定范围共 171 个提交，其中 62 个 merge commit、109 个 non-merge commit。171 个提交均通过完整 merge 保留祖先关系并记为 `Applied`，其中 15 个非 merge 实现提交同时按本地二次开发边界记为 `Applied + Overridden`；`Already Applied`、`Skipped`、`Deferred` 和未解决 `Conflict` 均为 0。

| 上游提交 | 状态 | 上游主题 |
| --- | --- | --- |
| `5e72deb7d` | Applied | feat: ops 错误详情弹窗支持自定义时间区间 |
| `3bff4b64b` | Applied | fix(ui): localize user role label in app header |
| `7d796f111` | Applied | fix(ui): adapt native form controls to dark mode via color-scheme |
| `a6d868f27` | Applied | fix(dashboard): include cache tokens in token card breakdown |
| `35e8ba2a3` | Applied | fix(announcements): use proper empty-state copy instead of error message |
| `0d5e3ca9b` | Applied | fix(ops): show neutral SLA card when window has no requests |
| `9c36b75a7` | Applied | fix(claude): strip cache control from deferred tools |
| `0b35370a7` | Applied | fix(claude): support top-level deferred tools |
| `2321c0e77` | Applied | chore: retry CI checks |
| `1afb8264e` | Applied | fix(lint): use require.NotNil for staticcheck SA5011 |
| `5ea03c178` | Applied | fix(lint): resolve remaining nil dereference warnings |
| `145b0ac35` | Applied | fix(lint): make remaining pointer assertions explicit |
| `e087f1b73` | Applied | fix(lint): use fatal pointer assertions in usage tests |
| `c3fc4331c` | Applied | fix(lint): make compatibility test assertions fatal |
| `c91fbeb0c` | Applied | chore: remove unrelated test refactors |
| `76a13a5a8` | Applied + Overridden | fix(gateway): handle Anthropic SSE overload errors |
| `a288bab73` | Applied | fix(gateway): align passthrough model discovery |
| `674570ca1` | Applied + Overridden | fix: preserve group pricing in auth snapshots |
| `44ef88f65` | Applied | fix(openai): restore API-key custom tools |
| `d677d67dd` | Applied + Overridden | feat: OpenAI Team 联动熔断 |
| `c3063e01a` | Applied + Overridden | fix(openai): recover message-only capacity failures |
| `539064798` | Applied + Overridden | fix(openai): complete request-scoped capacity recovery |
| `a600dd1c0` | Applied | Merge remote-tracking branch 'origin/main' into agent/openai-capacity-failover |
| `e8ff2017c` | Applied | fix(admin): show category labels in ops error distribution legend |
| `cb7841d85` | Applied | fix(i18n): add missing expired key to account status block used by proxy list |
| `5f1943310` | Applied | fix(ops): avoid single-insert fallback after batch failure |
| `79c2eb502` | Applied | fix: skip expiry reminders without SMTP config |
| `1977810cf` | Applied | fix(frontend): isolate account helper data loading |
| `9aac3b73f` | Applied | add Ollama usage query action |
| `cb5e03a72` | Applied | fix(antigravity): preserve mixed Gemini tool config |
| `8a0a76aeb` | Applied | Merge pull request #1 from xiaobo121388/codex/add-ollama-usage-query |
| `76b70b168` | Applied | fix(openai): validate bulk account settings |
| `b8642ef67` | Applied | fix(auth): make invitation code consumption atomic with user creation |
| `8d82bb069` | Applied | fix(openai): expose bulk account settings |
| `3c3bb2fa1` | Applied | fix(gemini): support includeServerSideToolInvocations in GeminiToolConfig |
| `612436a5a` | Applied | fix(openai-compat): Responses→Chat 桥接按 reasoning item id 缓存回注 reasoning_content |
| `971544570` | Applied | test(antigravity): check tool config assertions |
| `401dd43b4` | Applied | fix(apicompat): 链式工具调用回放本轮 reasoning_content |
| `6793d5ac8` | Applied + Overridden | fix(openai): make Codex convergence identity consistent |
| `c46d07ca0` | Applied | fix: normalize Grok response model audit aliases |
| `7e45634df` | Applied | chore: remove leftover Sora references after platform removal |
| `b2d1c3859` | Applied | Merge pull request #5738 from okbexx/fix/codex-identity-snapshot |
| `7d633f5fc` | Applied | Merge pull request #5742 from heathermhuang/codex/fix-grok-response-model-audit |
| `f66a42c40` | Applied | Merge pull request #5697 from feitianbubu/fix/ops-error-distribution-legend-labels |
| `8869775ed` | Applied | Merge pull request #5636 from feitianbubu/fix/proxy-status-expired-i18n-key |
| `ab0fcd1a0` | Applied | fix(gemini): Skipped 错误策略对齐 OpenAI，上游 4xx 不再硬改 500 |
| `615e6901e` | Applied | feat(channel-monitor): add quota mode schema (migration 226 + ent) |
| `c44711ac9` | Applied + Overridden | feat(channel-monitor): quota mode service layer (fetcher + dispatch + repo) |
| `6a6fd304f` | Applied | feat(settings): channel_monitor_show_quota public setting (default off) |
| `41344c20f` | Applied + Overridden | feat(monitor): wire quota fetcher & expose check_mode in handlers |
| `7ab6d3db6` | Applied | test(channel-monitor): quota mode unit/integration/migration coverage |
| `c51fd7d0b` | Applied | test(channel-monitor): adapt checker body test to 3-arg normalizeMonitorPrimaryModel |
| `bb6c3b4f6` | Applied + Overridden | fix: unify Codex OAuth outbound identity onto the inference resolver |
| `16e4f7ecc` | Applied | 修复 Codex 额度探针模型兼容性 |
| `a20e1c00c` | Applied | feat(monitor-ui): 配额模式表单、用量快照视图与 8 平台支持 |
| `302a10b88` | Applied | test(monitor-ui): 配额视图渲染与开关门控用例 |
| `a7a321232` | Applied | Merge pull request #5567 from wucm667/fix/issue-5563-anthropic-sse-overload |
| `cddb03c0f` | Applied | Merge pull request #5609 from wucm667/fix/issue-5607-auth-pricing-snapshot |
| `853234474` | Applied | Merge pull request #5715 from wucm667/fix/issue-3917-current-main |
| `1a3ecd2b9` | Applied | Merge pull request #5004 from wucm667/fix/issue-4990-deferred-tool-cache-control |
| `3d9a0a71b` | Applied | Merge pull request #4057 from feitianbubu/fix/ops-sla-zero-window |
| `632b4f4fa` | Applied | Merge pull request #4053 from feitianbubu/fix/announcements-empty-state-copy |
| `5dfac8bff` | Applied | Merge pull request #4049 from feitianbubu/fix/dashboard-token-cache-breakdown |
| `f6d6613ec` | Applied | Merge pull request #4006 from feitianbubu/fix/native-controls-dark-mode |
| `5c1296028` | Applied | Merge pull request #4005 from feitianbubu/fix/header-role-i18n |
| `c76ff5599` | Applied | Merge pull request #2148 from feitianbubu/pr/ops-error-detail-support-custom-time |
| `c0325d24f` | Applied | Merge pull request #5759 from o2e/codex/fix-codex-usage-probe-model-clean |
| `baaf59d41` | Applied | Merge pull request #5755 from feeeei/feat/gemini |
| `6259940ef` | Applied | Merge pull request #5669 from feeeei/main |
| `ed2da8239` | Applied | Merge pull request #5711 from wucm667/fix/issue-5709-antigravity-tool-config |
| `1ea4150bf` | Applied | Merge pull request #5581 from wucm667/fix/issue-5574-passthrough-model-discovery |
| `7376f4a48` | Applied | fix(lint): gofmt 对齐 + 移除未使用的 isSupportedProvider |
| `c792b44bb` | Applied | Merge pull request #5712 from xiaobo121388/main |
| `4d1983618` | Applied | Merge pull request #5716 from wucm667/fix/issue-2695-current-main |
| `938f1868a` | Applied | Merge pull request #5714 from wucm667/fix/issue-2733-current-main |
| `a9514a68d` | Applied | perf(usage): aggregate stats in one scan |
| `ebcae03af` | Applied | fix(lint): 补齐 setting_public.go 与配额 fetcher 测试的 gofmt 对齐 |
| `7e579cb28` | Applied | fix(openai): adapt client tools in WS HTTP bridge |
| `269fbcac0` | Applied | feat: Grok 用量条补齐本站 24h/7d/30d 聚合 |
| `1870b58c1` | Applied | Merge pull request #5721 from lyy0709/codex/bulk-openai-settings |
| `fd42d3722` | Applied | fix: hide Grok prepaid and used/limit when they are empty |
| `9617775f9` | Applied | fix(repo): tolerate ErrTxStarted for tx-bound clients and harden test stubs |
| `1ba92449c` | Applied | fix(gemini): wire includeServerSideToolInvocations into the typed transform path |
| `a34123959` | Applied + Overridden | fix(fingerprint): align credential-face identity with the real client and de-drift models version |
| `37732dcd3` | Applied | Merge pull request #5725 from tamseno/fix/gemini-include-server-side-tool-invocations |
| `f211a630c` | Applied | Merge pull request #5720 from tamseno/fix/invitation-code-toctou-race |
| `1ed3b6aef` | Applied | Merge pull request #5760 from spongehah/feature/unify-codex-outbound-identity |
| `685455985` | Applied | Merge pull request #5765 from IanShaw027/feat/grok-from-personal-main |
| `58ea46e89` | Applied | Merge pull request #5661 from wucm667/fix/issue-5659-openai-custom-tools |
| `26cb59df0` | Applied | Merge pull request #5764 from hansnow/fix/ws-http-bridge-custom-tools |
| `c253bd2c7` | Applied | fix(openai): restore client tools in terminal events |
| `58ccea4ea` | Applied | Merge pull request #5767 from hansnow/fix/ws-http-bridge-custom-tools |
| `8f6f45983` | Applied | fix(channels): support kimi/zhipu/deepseek platforms in channel pricing |
| `22df600d0` | Applied | fix(channel-monitor): 配额快照识别值通道失败并加 60s 负缓存与 singleflight |
| `03c3f3b6f` | Applied | feat(ui): Select 组件支持可选远程搜索（remote/loading props + search 事件） |
| `5cbd0c96a` | Applied | fix(monitor-ui): 关联账号选择器改服务端搜索+回填，OpenAI 配额模式加消耗提示 |
| `dd04503e1` | Applied | Merge pull request #5773 from Wei-Shaw/fix/channel-pricing-cn-platforms |
| `e0c48a19e` | Applied | Merge pull request #5761 from Randark-JMT/feat/channel-monitor-quota-mode |
| `49504adc9` | Applied | chore: sync VERSION to 0.1.178 [skip ci] |
| `3d21d6160` | Applied | chore: retrigger CI（上游 flaky 测试 TestApplyCodexFingerprintClientMetadataRaw_MatchesMapVariant 毫秒边界误报，与本 PR 无关） |
| `1128df259` | Applied | fix(monitor): align quota-fetcher credential/balance semantics with scheduler |
| `c41ae19e5` | Applied | fix(monitor): reject unusable quota data sources and invalid mode combos at write time |
| `e2dfb3b8c` | Applied | refactor(monitor): pass loaded account through quota sources (single load per fetch) |
| `c9effc456` | Applied | fix(frontend): monitor form check-mode restore, account unbinding and mode badge |
| `2c250bfd7` | Applied | fix(monitor-ui): localize the "quota" placeholder model across monitor views |
| `ac6208de1` | Applied | fix(accounts): route CN provider chat tests correctly |
| `85cb732cd` | Applied | docs: fix broken star history chart in README |
| `214210e1b` | Applied | Merge pull request #5782 from UnlastingR/fix/cn-provider-account-test-routing |
| `359fd12b2` | Applied | Merge pull request #5749 from Randark-JMT/chore/remove-sora-leftovers |
| `b228b93e9` | Applied + Overridden | fix(openai): 修复 Chat 非流式缓冲读取错误未触发故障转移 |
| `c6f4fbde4` | Applied | Merge pull request #5676 from Perfecto23/agent/openai-capacity-failover |
| `e61595fb3` | Applied | Merge pull request #5780 from Randark-JMT/fix/channel-monitor-p2 |
| `82f7dd14f` | Applied | Merge pull request #5794 from Dessalines39394/fix/star-history-chart |
| `bfac49fef` | Applied | fix(codex): handle responses input token preflight |
| `892787723` | Applied | fix(grok): preserve xhigh effort for grok-4.6 |
| `58e147fba` | Applied | feat(composite): support Codex endpoints |
| `b171bb0e4` | Applied | fix(composite): support CN providers |
| `e943f817b` | Applied | Merge pull request #5729 from lbyxiaolizi/fix/responses-chat-reasoning-content-passback |
| `4d3b300a2` | Applied | test(scheduler): update CN platform expectations |
| `2a5ae2b2d` | Applied | Merge pull request #5816 from kingsleydon/fix/composite-codex-support |
| `7d9c95848` | Applied | Merge pull request #5810 from Pluviobyte/codex/fix-responses-input-tokens |
| `bd1ccd973` | Applied | Merge remote-tracking branch 'origin/main' into fix/issue-5796-composite-new-platforms |
| `aa673062e` | Applied | fix(composite): keep CN rollout on fully supported paths |
| `b94e484e2` | Applied | fix(openai): preserve client tools across WS bridge turns |
| `499a8ee42` | Applied | fix(composite): exempt resolved grok/CN targets from messages dispatch gate |
| `e4896c41d` | Applied | test(apicompat): satisfy client tool type assertions |
| `fefd0d514` | Applied | test(apicompat): avoid unchecked tool history assertions |
| `6a945b3ca` | Applied | Merge pull request #5817 from wucm667/fix/issue-5796-composite-new-platforms |
| `e8b53c919` | Applied | Merge pull request #5801 from MokoYee/fix/openai-chat-buffered-stream-failover |
| `89d28037f` | Applied | Merge pull request #5822 from hansnow/fix/ws-http-bridge-followup-client-tools |
| `ae62854ab` | Applied | Merge pull request #5815 from 771373073/fix/grok46-xhigh |
| `f917d19d3` | Applied | test(frontend): align Grok API key placeholder assertion |
| `b0464a986` | Applied | feat(proxy): allow configurable probe targets |
| `994fbfedd` | Applied | fix(frontend): prevent CN quota labels overlapping bars |
| `63839f193` | Applied | fix(frontend): align admin role selector styling |
| `99a8b8470` | Applied | 修复 Grok 内联图片与 view_image 冲突 |
| `82cbe6aff` | Applied + Overridden | fix(openai): resume later websocket turns after 429 |
| `1b30a2d74` | Applied | feat(accounts): support header overrides for CN providers |
| `b0cdea303` | Applied | 补全 Grok 多入口内联图片工具适配 |
| `68666e1f8` | Applied | Merge pull request #5845 from vincenthcui/fix/openai-ws-later-turn-429-failover |
| `23dc9377b` | Applied | Merge pull request #5844 from lyen1688/fix/grok-inline-image-view-image |
| `d5484866f` | Applied | fix(config): register proxy probe URLs default |
| `b7fe8ebaa` | Applied | Merge pull request #5762 from jaxxjj/codex/perf-usage-stats-grouping-sets |
| `38a5f266e` | Applied | Merge pull request #5839 from xuhaihan/test/fix-grok-api-key-placeholder |
| `32ecd5cc6` | Applied | Merge pull request #5847 from wucm667/feat/issue-5826-cn-header-overrides |
| `32a0d9ba2` | Applied | Merge pull request #5837 from xuhaihan/fix/cn-provider-quota-cell-layout |
| `ec5a34593` | Applied | fix(proxy): validate configurable probe targets |
| `1ab325678` | Applied | fix(proxy): format validated probe targets |
| `fce90ecf8` | Applied | 渠道定价：持久化服务层级与区间倍率 |
| `5b2a386ed` | Applied + Overridden | 计费：应用渠道倍率与上下文区间价格 |
| `7dae055f2` | Applied | 计费：识别并记录 Anthropic Fast 请求 |
| `26be82cc8` | Applied | 前端：配置渠道倍率并精简长上下文开关 |
| `d536795e9` | Applied | 测试：同步长上下文计费断言 |
| `d4d2c746c` | Applied | 前端：修正账号长上下文开关门控 |
| `5b2089c5a` | Applied | fix(grok): lower Codex tool-search discovery outputs |
| `1f2a87adb` | Applied | fix(admin): 补全平台筛选选项 |
| `2d03e40fd` | Applied | Merge pull request #5868 from X-T-E-R/codex/fix-grok-tool-search-output |
| `394b12afd` | Applied | Merge pull request #5875 from hansnow/fix/admin-platform-filter-options |
| `1b5dc676a` | Applied | Merge pull request #5851 from IanShaw027/feat/channel-pricing-tier-multipliers |
| `6b0ec50f2` | Applied | fix(ops): exclude model configuration errors from SLA |
| `85051616f` | Applied | feat(accounts): add adaptive API protocol routing |
| `9ede0f716` | Applied | fix(grok): promote tool-search discoveries into callable tools |
| `06fc0055c` | Applied | Merge remote-tracking branch 'origin/main' into codex/promote-grok-tool-search-discoveries |
| `cb7fef14c` | Applied | Merge pull request #5838 from xuhaihan/fix/admin-user-role-select-styling |
| `fb94fd352` | Applied | Merge pull request #5834 from xuhaihan/feat/configurable-proxy-probe-targets |
| `b3092145d` | Applied + Overridden | fix(accounts): harden adaptive protocol compatibility |
| `752b3d857` | Applied | Merge pull request #5881 from X-T-E-R/codex/promote-grok-tool-search-discoveries |
| `740f58080` | Applied | Merge pull request #5842 from SavitarC/feat/adaptive-api-protocol |
| `75f88be5f` | Applied | Merge pull request #5876 from wucm667/fix/issue-5872-exclude-model-not-found-sla |
| `c0e073a79` | Applied | chore: update sponsors |
| `2bc139ab5` | Applied + Overridden | chore: sync VERSION to 0.1.179 [skip ci] |

`Applied + Overridden` 的覆盖边界如下：

- `76a13a5a8`：接入 Anthropic SSE overload 识别；在首个语义输出前先丢弃暂存头和前导帧，再把未输出的 `overloaded_error` 映射为 529 并进入本地重试/换号流程，已输出后不切号。
- `674570ca1`：接入分组定价认证快照，并将本地 API Key 缓存快照升至 v21，继续完整携带长上下文门禁、服务层级、时段和 token 区间定价字段。
- `d677d67dd`：接入 OpenAI Team 联动熔断和去重；联动处理先执行，再进入本地 strict failure scheduling、连续失败停调度和自动测活。
- `c3063e01a`、`539064798`：接入请求级容量错误恢复；继续以首个语义输出为边界，metadata/preamble/keepalive 不视为已输出。
- `b228b93e9`：接入 Chat 非流式缓冲读取错误故障转移；非流式 HTTP 继续保持 HTTP，并沿用本地暂存与换号边界。
- `82cbe6aff`：接入 WS 后续轮次 429 恢复；只有当前轮次上下文可完整重建时才允许换号，已输出后的轮次不回退重放。
- `6793d5ac8`、`bb6c3b4f6`、`a34123959`：接入 Codex 统一出站身份与 credential-face 一致性；指纹字段取双方并集，HTTP/WS 头与 body 共用时间戳，并从账号随机种子稳定派生。
- `c44711ac9`、`41344c20f`：接入 quota fetch/dispatch/repository 与 handler；继续保留本地流式探针、600 秒超时、Responses 结构化 `input` 和 quota 展示 opt-in 白名单边界。
- `5b2a386ed`：接入渠道服务层级、时段和 token 区间定价；OpenAI 长上下文继续受账号与真实分组双门禁，Grok 只服从分组开关，只有真实上下文命中配置区间时才抑制内置阶梯，未命中区间仍回退基础价。
- `b3092145d`：接入 Adaptive 协议兼容加固；国产供应商账号测试继续覆盖本地自定义提示词、采样参数过滤、语义错误和实际协议路由。
- `2bc139ab5`：保留上游最终版本提交及祖先关系，但不回退到 `0.1.179`，继续使用本地 `0.1.223`；中间版本 `49504adc9` 已被上游自身后续版本自然替换，记为普通 `Applied`。

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `e7b2c6db05751accafdc0894baac1faa21d209be`；双亲为同步前本地 SHA `3bf1f2ee3fb0af82110ae539a2e1382e1ac44fe6` 和固定上游 SHA `2bc139ab527b4a687546d145dc7bb9063cf14510`。
- 写入本记录前，相对 `LOCAL_PRE_SYNC_SHA` 共变更 359 个文件：新增 57 个、修改 300 个、删除 2 个，增加 20528 行、删除 2080 行。上游范围涉及 357 个路径；额外路径为本地首语义输出兼容、定价兼容测试和二开台账。
- 主要新增：渠道监控 `quota`/`quota_probe` 与额度展示、渠道服务层级/时段/token 区间定价、CN 自适应协议、`/responses/input_tokens`、OpenAI WS 后续轮次容量恢复、Codex 指纹随机种子、OpenAI Team 联动熔断，以及迁移 225 至 228。
- 主要修复：Anthropic SSE overload 的 529/failover 处理、非流式 Chat 缓冲读取错误、Responses 客户端工具跨 WS bridge 轮次恢复、Gemini/Antigravity 工具配置、Composite Codex/CN 路由、邀请代码原子消费、用量统计聚合和前端监控/账号交互。
- 删除的 `assets/partners/logos/claudeapi.jpg` 与 `assets/partners/logos/code0.jpg` 均为上游 sponsor 更新已删除且不再引用的资源；未删除本地业务资源。
- `backend/cmd/server/VERSION` 最终保持 `0.1.223`。上游该路径与本地最终内容一致，因此相对同步前无文件差异；`backend/ent/group.go` 的上游结果也已由本地现状等价覆盖。
- Ent/Wire 从合并后的 schema/provider 隔离重生成并验证可复现；前端与 Canvas 构建未留下锁文件或其他未提交受控差异。

### 冲突与最终解决方案

- 用户批准集中审批报告中列明的完整 merge、30 个文本冲突解决方案和本地验证范围；执行中未出现需要新增业务选择的计划外高风险冲突。
- 30 个文本冲突均逐文件解决，未整文件采用 `ours`、`theirs` 或上游版本；最终索引无未解决路径。
- 普通分组继续严格按用户请求模型计费，缺价返回错误；上游响应模型只用于诊断。Composite 仅在显式渠道价存在时按公开别名计费，媒体路径继续使用实际模型。
- OpenAI 长上下文使用账号与真实分组双门禁；Grok 只服从分组开关。只有真实上下文命中配置区间时才抑制模型内置阶梯，避免部分区间未命中时错误选择最低档。
- 首个语义输出前统一暂存账号相关响应头和前导帧，Responses metadata、preamble 与 keepalive 均不解除守卫；SSE error 先丢弃缓冲帧再判断换号，避免 529 被已提交状态误判。
- 流式 HTTP 请求可按批准边界转 WSv2，非流式 HTTP 保持 HTTP；客户端工具状态跨轮次保留，但只有上下文可完整重建时才恢复当前轮次换号。
- Codex 指纹字段取双方并集，HTTP/WS 头与 body 共用时间戳，并从系统管理的账号随机种子稳定派生；Team 联动熔断先于本地 failure scheduling 执行。
- 渠道监控保留本地流式请求、600 秒超时和 Responses 结构化 `input`，接入 `quota`/`quota_probe`、关联账号及额度视图；`quota` 不构造 LLM 请求。
- 公共设置继续保留 Canvas、每日签到和本地模型市场默认语义；渠道 quota 展示为 opt-in，缺失或 false 时服务端剥离。
- CN Adaptive 账号测试覆盖自定义提示词、实际协议路由、采样参数过滤和语义错误；GPT-5.5/Pro 使用官方价格，不被旧 GPT-5.4 映射遮蔽。
- Ent 与 Wire 以合并后的最终 schema/provider 重生成，生成结果与工作树逐文件一致；两份生产 Compose 继续保持字节完全一致。

### 刻意保留的二次开发功能

- `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-004`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`：首语义输出、非语义心跳扣除、529/failover、连续失败停调度、部分输出边界和请求体释放。
- `CUST-PROTO-001`、`CUST-PROTO-004`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-PROTO-007`、`CUST-ACC-005`：Codex 指纹、Claude 工具缓存、请求头覆写、Responses/WS 路由、结构化监控请求和增强账号测试。
- `CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`：严格请求模型计费、多层定价、长上下文双门禁和服务层级费用展示。
- `CUST-OBS-001`、`CUST-OBS-002`、`CUST-PROD-001`、`CUST-PROD-002`、`CUST-PROD-007`、`CUST-RISK-002`、`CUST-UI-002`：流式渠道监控、独立上游同步、每日签到、本地模型市场、Canvas、cyber 用量边界和公共设置白名单。
- 生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性、4 vCPU/8 GiB 性能参数、按用户串行扣费和 5 秒 usage task 超时均未被放宽。

### 验证记录

本机使用 Go 1.26.6、Node 24.15.0、pnpm 9.15.9 和 golangci-lint 2.9.0；CI Node 基线为 20。

| 阶段 | 命令或检查 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前 | `go test -tags=unit ./... -count=1`、`golangci-lint run --timeout=30m ./...`、`CGO_ENABLED=0 go build -trimpath ./...` | 0 | Go 全量 unit、lint 与 CGO 关闭构建通过，作为同步后比较基准 |
| 同步前 | frontend `lint:check`、`typecheck`、`test:run`、`build`；Canvas `format:check`、`typecheck`、`test`、`build` | 0 | Vue 与 Canvas 基线检查、测试和生产构建通过 |
| 兼容回归 | 计费区间/长上下文双门禁、Spark Shadow 父账号门禁、首语义输出、Anthropic SSE overload、WS 后续轮次、CN Adaptive、Codex 指纹和渠道监控定向 Go 测试 | 0 | 相关定向回归通过；测试夹具独立验证分组与父账号开关，已输出场景先发送真实文本增量 |
| 迁移 | `go test ./migrations` 的 225 至 228 静态用例；`go test -tags=unit ./internal/repository` 的非事务迁移执行器用例 | 0 | JSONB/约束/索引 SQL 静态契约及 `CREATE INDEX CONCURRENTLY` 非事务执行路径通过 |
| 同步后后端 | `go test -tags=unit ./... -count=1` | 0 | Go 全量 unit 通过 |
| 同步后后端 | `golangci-lint run --timeout=30m ./...` | 0 | `0 issues` |
| 同步后后端 | `CGO_ENABLED=0 go build -trimpath ./...` | 0 | 全包构建通过 |
| 同步后前端 | `corepack pnpm --dir frontend run lint:check`、`typecheck`、`test:run`、`build` | 0 | lint/typecheck 通过；257 个测试文件、1802 项测试通过；生产构建通过，仅有既存动态导入和 chunk 体积警告 |
| 同步后 Canvas | `corepack pnpm --dir canvas run format:check`、`typecheck`、`test`、`build` | 0 | 格式、类型检查、34 项测试和生产构建通过 |
| 生成一致性 | 在隔离目录重生成 Ent/Wire，并与 merge tree 逐文件比较 | 0 | 两类生成结果均可复现，重生成后无差异 |
| 部署静态 | Git Bash `bash -n`/`sh -n`；Compose security/resources、Caddy cache 和假 GitHub Token 隔离测试 | 0 | 脚本语法、安全配置、资源参数、Caddy 缓存和假 Token 隔离均通过；未读取真实 Token 或 `.env`，未联网部署 |
| Compose 一致性 | `fc /b deploy\docker-compose.yml deploy\docker-compose.sub2api.yml` | 0 | 两份生产 Compose 字节完全一致 |
| 最终静态复核 | `git diff --cached --check`、索引冲突项、冲突标记、意外删除、凭据类路径、祖先关系和工作树检查 | 0 | 无空白错误、未解决冲突、异常删除或凭据文件；固定上游 SHA 已成为同步分支祖先 |

### 未验证项与残余风险

- 本机无 Docker CLI，未运行 Docker/Testcontainers、完整 `go test -tags=integration ./...` 或依赖 PostgreSQL/Redis 的本地完整服务；`/health` 未验证。
- 未在真实 PostgreSQL 上执行 225 至 228 完整迁移；新增 JSONB、约束、backfill 与 `CREATE INDEX CONCURRENTLY` 仍需隔离 CI/临时数据库验证。
- 未运行 `-race`，本机无 GCC；未安装/运行 `govulncheck`，本记录不覆盖数据竞争或最新漏洞可达性。
- 本机 Node 24.15.0 与 CI Node 20 存在版本差异，需以后续 CI 复核 Node 20 结果。
- 未使用外部真实 OpenAI、Anthropic、Grok、CN Provider、SMTP 或其他业务凭据；真实额度、媒体、流式故障转移和消息发送未验证。
- 未读取 `.env`、`D:\project\github_token.sh` 或任何私钥；未执行 push、PR、部署、远程服务器访问、容器重启、生产挂载核验或生产数据操作。
- 生产服务器状态、bind mount 实际挂载、实例性能参数和健康检查均未验证；本次只验证仓库内静态 Compose 约束。

## 2026-08-25 同步至 aa2c4e8d1

- 执行时间：2026-08-25T22:14:09+08:00 至 2026-08-26T00:10:14+08:00
- 执行状态：同步分支已完整合并固定上游范围、完成二次开发适配并通过本地验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`5b452d2289f9c73bccc01ad38dce6cb65af21cd3`
- 上游代码合并提交：`502aa69168f338d4df4792d990602444ae99440c`
- 最后一个代码提交：`a5708ece931f3e0b6688b518f9c9d248f886b604`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`2bc139ab527b4a687546d145dc7bb9063cf14510`
- `UPSTREAM_NEW_SHA`：`aa2c4e8d136b13553ac7bae3d76c25715333a554`
- merge-base：`2bc139ab527b4a687546d145dc7bb9063cf14510`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`aa2c4e8d136b13553ac7bae3d76c25715333a554`
- 集成策略：在隔离同步分支执行 `git merge --no-ff --no-commit aa2c4e8d136b13553ac7bae3d76c25715333a554`，逐文件解决 42 个文本冲突并复核自动合并路径；merge commit 保留完整上游祖先关系，二次开发兼容调整和两份台账使用后续独立提交
- 备份分支：`backup/pre-upstream-sync-20260825-200721-5b452d228`
- 同步分支：`sync/upstream-20260825-aa2c4e8d1`

### 上游提交处置

固定范围共 207 个提交，其中 70 个 merge commit、137 个 non-merge commit。互斥处置结果为：179 个 `Applied`、25 个 `Applied + Overridden`、3 个 `Already Applied + Overridden`；`Skipped`、`Deferred` 和未解决 `Conflict` 均为 0。完整 merge 已保留全部 207 个提交的祖先关系，下表集合互不重叠并覆盖固定范围内全部 SHA。

| 上游提交 | 状态 | 上游主题 |
| --- | --- | --- |
| `953028718` | Applied | 修复 Grok 错误分类与容量重试 |
| `5ade09431` | Applied | 优化 Grok 传输超时与 Realtime 握手 |
| `ed4207a16` | Applied + Overridden | 校正 Grok 模型目录计费与工具出站 |
| `39485f2e2` | Applied + Overridden | 更新 Grok 默认模型与官方计费目录 |
| `ad26172b8` | Applied | 完善 Grok 限流冷却与用量兼容 |
| `611a7c8ed` | Applied | 修复 Grok Realtime 预接入切号 |
| `61c2f5ad2` | Applied | 复用 Grok Realtime 预握手连接 |
| `e85348be8` | Applied | 调整 Grok 媒体超时与重试语义 |
| `0e05c61d3` | Applied | 修复 Grok 容量冷却与用量计费 |
| `f7145c750` | Applied | 迁移 Grok 默认模型到 4.6 |
| `3243983b7` | Applied | 完善 Grok Realtime 与默认映射测试 |
| `726de3010` | Applied | 修复 Grok WebSearch SSE action 兼容 |
| `6c3edc095` | Applied | feat(429): add configurable cooldown and retry strategies |
| `e62ec2c42` | Applied | Revert "feat(429): add configurable cooldown and retry strategies" |
| `8db8791a7` | Applied | 为 Grok 普通 429 增加有限同号重试 |
| `ad87ddee1` | Applied | 补齐 Grok CC 重试与 compaction 恢复 |
| `17c0ee385` | Applied | 支持 Grok compaction 422 重试 |
| `5ae254f77` | Applied | 补齐 Grok CC bridge 同号重试 |
| `ab9cb69e7` | Applied | Revert "修复 Grok WebSearch SSE action 兼容" |
| `2ab24a1e7` | Applied | 修正 Grok 429 边界与 stream idle 重试上限 |
| `0b1f79c83` | Applied | 同步 Grok 429 bridge 回归断言 |
| `f7bc1970e` | Applied | 同步 Grok 默认模型回归断言 |
| `c628b3eea` | Applied | 让 Grok stream idle 重试上限作用于主路径 |
| `cca235365` | Applied | 修正 Grok 默认模型测试断言 |
| `d78e366db` | Applied | 补齐 Grok Realtime 握手失败账号冷却 |
| `39aaf2fea` | Applied | 收紧 Grok 容量重试与兼容性分类 |
| `2e68b10aa` | Applied + Overridden | 完善 Grok 内容拒绝计费与媒体兼容 |
| `1bff06ea5` | Applied + Overridden | 修复 Grok Realtime 关闭检查与 rollup 时区断言 |
| `4033387fd` | Applied | Merge pull request #5925 from IanShaw027/fix/grok-compatibility |
| `354825674` | Applied | chore: update gitignore |
| `cf3577a3c` | Applied + Overridden | fix(openai): harden Responses compatibility |
| `acce29af2` | Applied + Overridden | 补齐 OpenAI 与 Grok 协议兼容处理 |
| `c374ff295` | Applied + Overridden | 完善 OpenAI 网关切换与运维错误语义 |
| `e4f869e0c` | Applied | 完善运维错误详情兼容展示 |
| `d1c6456d0` | Applied | 合并上游最新主分支兼容修复 |
| `ccb20ace8` | Applied | 修复 OpenAI 兼容性 PR 的 CI 回归 |
| `48615d5d1` | Applied | Merge remote-tracking branch 'upstream/main' into fix/openai-responses-compatibility |
| `2ab41b92b` | Applied | 修复 Grok 兼容测试与 errcheck 导致的 CI 失败 |
| `787f875dd` | Applied + Overridden | 修复 Grok 孤儿控件、thinking 断言与流式 failover 的 CI 回归 |
| `7c53a842e` | Applied | 合并上游 main，保留 5925 的 Grok 重试上限与 5888 的协议兼容修复 |
| `7e9af4c10` | Applied + Overridden | 将 compact fallback 的流式重试改为循环，避免递归重置单次重试标志 |
| `16b15e870` | Applied | 修复 5888 与 5925 的同号重试语义冲突 |
| `b2b2adcf8` | Applied + Overridden | 修复 PR 5888 审查发现的兼容性与竞态问题 |
| `1429e8f71` | Applied + Overridden | 修复 PR 5888 剩余审计问题 |
| `9d5171c5d` | Applied | Merge pull request #5888 from IanShaw027/fix/openai-responses-compatibility |
| `f6aa9dc3c` | Applied | fix(securityaudit): log prompt_guard.config_loaded only on change |
| `c2572535d` | Applied | Merge pull request #6016 from YogaSakti/fix/prompt-guard-config-loaded-log-spam |
| `d5824f6a5` | Applied | fix: preserve native max reasoning effort |
| `98d60931c` | Applied | Merge pull request #5954 from StarryKira/codex/fix-5945-upstream |
| `2074fe3ba` | Applied | fix(gateway): 记录国产厂商原生 Anthropic 直通路径的 reasoning_effort |
| `9f74eb57f` | Applied | Merge pull request #5919 from clearmann/fix/cn-native-anthropic-reasoning-effort |
| `011745255` | Applied | fix(test): CN 供应商额度探测 fake 加锁消除并发 append 竞态 |
| `6816c6388` | Applied | Merge pull request #5906 from feeeei/main |
| `21c07e835` | Applied | fix(antigravity): use official daily endpoint |
| `4eb7630ab` | Applied | Merge pull request #5625 from sweetcornna/fix/antigravity-official-daily-endpoint |
| `e7a3c1202` | Applied | fix(antigravity): route paid accounts to daily endpoint |
| `b410c3913` | Applied | fix(security): add nanoid audit exception for GHSA-2v37-7h3g-55p8 |
| `f96035462` | Applied | Merge pull request #5612 from wucm667/fix/issue-5611-antigravity-paid-tier-endpoint |
| `98c7b0e88` | Applied | docs: fix self-referential URLs after file moved into docs/ |
| `b2d5ce039` | Applied | Merge pull request #5662 from yzxcj797/fix/self-referential-paths |
| `b1e60ba45` | Applied + Overridden | fix(gateway): 修复池模式同账号错误重试 |
| `fd24923f6` | Applied | Merge pull request #5685 from Monster-DP/main |
| `bafd2e293` | Applied | fix(apicompat): omit empty tool name on streamed arguments deltas |
| `f646a1f97` | Applied | Merge pull request #5632 from 3219378872/fix/apicompat-streaming-tool-name-empty |
| `3445485eb` | Applied | fix(frontend): prevent token refresh lock loop |
| `5fc977846` | Applied | Merge pull request #6053 from wucm667/fix/issue-5899-token-refresh-lock-cpu |
| `68653fb2c` | Applied | fix: allow messages dispatch for composite groups |
| `afa21336a` | Applied | Merge pull request #6048 from wucm667/fix/issue-5886-composite-messages-dispatch |
| `b0b2734b0` | Applied | fix(deepseek): ignore invalid relay balance payloads |
| `77d8516e8` | Applied | Merge pull request #5911 from xuhaihan/fix/deepseek-relay-balance-validation |
| `a749673de` | Applied | fix(accounts): route CN provider anthropic-protocol tests to the native endpoint |
| `ccfced36a` | Applied | Merge pull request #6011 from HypoxanthineOvO/fix/cn-anthropic-protocol-account-test |
| `2e279c81d` | Applied | fix(frontend): make CN provider quota/balance refresh affordance explicit |
| `beaeaaed0` | Applied | Merge pull request #6009 from HypoxanthineOvO/fix/cn-quota-refresh-affordance |
| `f75c4161f` | Applied | fix(deepseek): make account test links platform-aware |
| `01a008394` | Applied | fix(deepseek): route responses account tests to OpenAI probe |
| `fb56bcbd0` | Applied | Merge pull request #5913 from xuhaihan/fix/deepseek-responses-account-test |
| `1e1798d90` | Applied | fix(gateway): Composite 分组放行视频生成端点 |
| `219368ec6` | Applied | Merge pull request #5654 from zninggo/fix/composite-video-generation |
| `e45490a36` | Applied | fix(openai): stabilize chat sticky hash across dynamic system messages |
| `2ddda6735` | Applied | Merge pull request #6049 from MokoYee/fix/openai-sticky-prefix-system |
| `d9d2854d2` | Applied | Make enabled model plaza discoverable from /home |
| `a53150a7c` | Applied | Merge pull request #5708 from yan9651688/fix/issue-5524-model-plaza-home |
| `40c26f343` | Applied | fix(openai): 空 openai_capabilities 不再排除 OAuth 账号的文本调度（#5530） |
| `67380eafd` | Applied | Merge pull request #5549 from zcxads666/fix/openai-capabilities-empty-set |
| `b30651a0a` | Applied | fix(ollama): 对齐 Cloud Chat Completions 思维字段为 reasoning_content |
| `86470628d` | Applied | feat(ollama): 对 Ollama Cloud 账号 clamp max_tokens 上限 |
| `7c64a48dc` | Applied | Merge pull request #6067 from alfadb/fix/ollama-cloud-cc-reasoning-content |
| `f98a056f7` | Applied | fix(gemini): constrain Google One model catalog |
| `844b11878` | Applied | Merge pull request #5938 from Hakunm/fix/google-one-model-catalog |
| `4d4a0be1a` | Applied | fix(apicompat): chat/completions file part 不再被静默丢弃，转换为 Responses input_file |
| `6244090c1` | Applied | Merge pull request #5487 from an-epiphany/fix/file-part-min |
| `25da02ddd` | Applied | fix(openai): avoid duplicate HTTP bridge replay |
| `66808413d` | Applied | fix(openai): drop orphan replay tool calls |
| `ffc01f9c6` | Applied | Merge pull request #5864 from wucm667/fix/issue-5850-http-bridge-replay |
| `b27cd76a8` | Applied | fix(deepseek): adapt Codex custom tools for Responses |
| `30ae15268` | Applied | Merge remote-tracking branch 'origin2/main' into fix/deepseek-responses-client-tools |
| `cef18b4ad` | Applied + Overridden | fix(deepseek): route client tools through native responses |
| `73f6a590b` | Applied | Merge pull request #5912 from xuhaihan/fix/deepseek-responses-client-tools |
| `e2d9ce0ca` | Applied | fix(apicompat): reject malformed tool-call arguments |
| `fbc9ee626` | Applied | fix(apicompat): narrow malformed tool-call handling |
| `fd6cd474d` | Applied | Merge pull request #5846 from lbyxiaolizi/fix/responses-chat-malformed-tool-arguments |
| `cb8dabc12` | Applied | fix(openai): stabilize oauth image generation |
| `d29d7f8cb` | Applied | Merge pull request #6065 from chinnsenn/fix/image-generation-flows |
| `fa4587041` | Applied + Overridden | fix(openai): keep auto-review on parent account |
| `d45135d87` | Applied | Merge pull request #6068 from okbexx/fix/codex-guardian-parent-affinity |
| `40ea3aeba` | Applied + Overridden | feat: add OAuth outbound transport plugin system |
| `26ac0498f` | Applied | test: update plugin management settings contract |
| `684d9efb1` | Applied | fix: harden plugin runtime and UI bridge |
| `391d69e08` | Applied | fix: preserve initial plugin bridge requests |
| `40aaf7b3a` | Applied | fix: handle plugin route health update errors |
| `f82d32207` | Applied | Merge pull request #6127 from Wei-Shaw/feat/oauth-transport-plugin-system |
| `77e0409f7` | Applied | 新增渠道时间段定价工作日规则 |
| `3e45d4e03` | Applied | Merge pull request #6089 from lyen1688/feat/channel-time-pricing-weekdays |
| `75faedda9` | Applied + Overridden | 计费：fast/priority 按上游响应实际档位只降不升计费 |
| `3b8a148bc` | Applied | Merge pull request #6111 from feeeei/fix/request_billing |
| `616df479e` | Applied | fix(admin): show account priority by default |
| `41f6e6379` | Applied | Merge pull request #6117 from wucm667/feat/issue-6114-account-priority-column |
| `5dfad32b8` | Applied | fix(frontend): accept unlimited (0) user concurrency in the edit dialog |
| `817fd1214` | Applied | Merge pull request #6075 from YogaSakti/fix/user-edit-allow-zero-concurrency |
| `ee62dfbaf` | Applied | fix(proxy): support bracketed IPv6 hosts in batch proxy URL parsing |
| `ba5b861ec` | Applied | Merge pull request #6073 from lbyxiaolizi/fix/proxy-ipv6-batch-parse |
| `cd05772e9` | Applied | fix(ops): avoid mixing cgroup and host memory metrics |
| `a52665d07` | Applied | Merge pull request #6061 from shunwang-crypto/fix/ops-mixing-cgroup-host-memory |
| `3fd66a33b` | Applied + Overridden | fix(scheduler): diagnose load-batch OpenAI exclusions |
| `e00a8abdd` | Applied | Merge pull request #6124 from anguobao123/codex/diagnose-openai-load-batch-exclusions |
| `913ec5d74` | Applied | fix(openai): sync models for OAuth accounts |
| `823895679` | Applied | Merge pull request #6095 from xiaxiaxaia/fix/openai-oauth-upstream-model-sync |
| `9f2f2738f` | Applied | docs(openai): document force HTTP fallback |
| `10081a812` | Applied | fix(deploy): pass force HTTP setting to containers |
| `e2263d256` | Applied | fix(deploy): forward documented gateway settings |
| `6a1efda0c` | Applied | fix(deploy): preserve gateway defaults in compose |
| `fb01f5df2` | Applied | Merge pull request #6060 from anguobao123/codex/document-openai-force-http-fallback |
| `cc894ef57` | Applied | fix(openai): strip empty streamed tool-call id/name |
| `fa42c3d70` | Applied | Merge pull request #6080 from alfadb/fix/cc-stream-empty-tool-call-identity |
| `7a09a2eaf` | Applied | fix(responses): remove orphan deferred tool flags |
| `748b84a15` | Applied | Merge pull request #6081 from wucm667/fix/issue-5942-deferred-tools |
| `31d5b67ba` | Applied + Overridden | fix(openai): restore namespaced custom tool aliases |
| `4eadee107` | Applied | [verified] test(openai): update responses bridge signature |
| `f25f399be` | Applied | Merge pull request #5905 from wucm667/fix/issue-5883-restore-custom-tool-alias |
| `243921dc0` | Applied | fix(openai): rebuild streaming terminal output from the reported items |
| `625f1693c` | Applied | Merge pull request #6118 from akihitohyh/fix/terminal-output-item-preservation |
| `7498d8fdc` | Applied | fix(openai): enforce serial tool calls for Responses Lite |
| `c41646788` | Applied | Merge pull request #6084 from wucm667/fix/issue-6057-responses-lite-parallel-tools |
| `4a1da2950` | Applied | fix(deps): bump dompurify to patch multiple sanitizer-bypass XSS advisories |
| `a177b88e5` | Applied | Merge pull request #6122 from aeonframework/security/bump-dompurify-xss-fixes |
| `cfecc8d11` | Applied | feat: 运维监控错误详情支持返回列表并保留筛选状态 |
| `7075ae0d8` | Applied | Merge pull request #6133 from spongehah/feat-ops-error-detail-back-to-list-pr |
| `695ebede7` | Applied | fix(billing): normalize CN Anthropic usage tokens |
| `b8651947c` | Applied | Merge pull request #6137 from yan9651688/codex/fix-cn-anthropic-usage-billing |
| `6466978d2` | Applied + Overridden | 计费：统一 token 计费路径选择并提供上下文阶梯单价表查询 |
| `377d1230f` | Applied | 模型广场：按计费阶梯单价表展示长上下文档位 |
| `ecce0769c` | Applied | 模型广场：上下文档位统一标签形态并保证升序 |
| `83d4eb6a4` | Applied | 模型广场：增加渠道分时段计价展示 |
| `b07d85c49` | Applied | 模型广场：分时计价同步渠道仅工作日规则 |
| `f19095f96` | Applied | 模型广场：分时时段行明确不含高峰倍率口径并披露叠加 |
| `2f43e72bb` | Applied | Merge pull request #6109 from feeeei/main |
| `cbe258fd1` | Applied + Overridden | build: 升级 Go 1.27.0，同步 CI/Dockerfile 并适配 jsonv2 与 golangci-lint v2.13 |
| `3b8177642` | Applied | fix(test): grok QueryQuota 用例排除后台 /v1/models 同步请求，消除请求计数竞态 |
| `73aabc861` | Applied + Overridden | build: 取消 gosec G703/G704 全局排除，生产代码逐点 nolint、测试文件按路径豁免；DEV_GUIDE 同步 golangci-lint v2.13 |
| `c4ae3550d` | Applied | Merge pull request #6119 from feeeei/feat/go1.27.0 |
| `f06bf181d` | Applied + Overridden | feat(openai): support Fast mode service_tier across responses/chat/WS paths |
| `c0c3e1cb4` | Applied | fix(openai): wire local observer service tier in WS ingress; bound handler tests |
| `e457f0fa2` | Applied | fix(openai): adapt service tier observation to upstream constraints |
| `1591477a3` | Applied | test(apicompat): adapt ChatCompletionsResponseToResponses call to upstream functionTools signature |
| `bd17411d0` | Applied | Merge pull request #6129 from alfadb/feature/openai-fast-service-tier |
| `269a40924` | Applied | fix(openai): harden flaky alloc guard in tool schema sanitize test |
| `4a02d8054` | Applied | Merge pull request #6136 from alfadb/fix/flaky-tool-schema-alloc-guard |
| `6f972145b` | Applied + Overridden | feat: 支持 OpenAI 重置卡按用量阈值自动使用 |
| `96b160d9a` | Applied | fix: 修复重置工作流共享告警码检查 |
| `5f43696a9` | Applied | Merge pull request #6121 from creamtea47/codex/feat-openai-auto-reset-credit |
| `d493ce0bb` | Applied + Overridden | fix(openai): scope codex identity to oauth account |
| `7bb9c0ed7` | Applied | Merge pull request #6079 from okbexx/fix/codex-analytics-account-affinity |
| `847c0c452` | Applied | feat(gateway): configure model list read limit |
| `c40edb407` | Applied | Merge pull request #6139 from xz-dev/fix/configurable-model-list-read-limit |
| `03e8ab413` | Already Applied + Overridden | chore: sync VERSION to 0.1.180 [skip ci] |
| `1563db3f8` | Applied | fix(openai): keep parallel_tool_calls for Responses Lite additional_tools |
| `2307aa5ca` | Applied | Merge pull request #6148 from 759502416/fix/responses-lite-parallel-tool-calls |
| `e440ac48c` | Applied | fix(openai): clear the rejected input status for the whole item type |
| `07931bbb1` | Applied | Merge pull request #6143 from akihitohyh/fix/rejected-status-strip-all |
| `9fb260439` | Applied | fix(grok): use official CLI user agent |
| `7ba3e1ac5` | Applied | Merge pull request #6150 from Wei-Shaw/fix/grok-upstream-user-agent |
| `19da0f240` | Applied | fix(gemini): sanitize unsupported tool schema fields |
| `3af5443b2` | Applied | Merge pull request #6116 from wucm667/fix/issue-6110-gemini-tool-schema |
| `e2d9b823f` | Already Applied + Overridden | chore: sync VERSION to 0.1.181 [skip ci] |
| `3b7753a8e` | Applied | chore: update sponsors |
| `329b92ef0` | Applied | fix(openai): preserve OAuth image prompts verbatim |
| `1dc1b4426` | Applied | Merge pull request #6149 from SipengXie2024/fix/oauth-image-verbatim-prompt |
| `99ec347ea` | Applied | fix(antigravity): migrate legacy Sonnet tests to 4.6 |
| `71aa6e357` | Applied | fix(antigravity): preserve explicit Sonnet 4.5 routing |
| `3dd717ab0` | Applied | Merge pull request #5920 from wucm667/fix/issue-5884-antigravity-sonnet46 |
| `bc4a9ae43` | Applied + Overridden | fix: prevent duplicate Anthropic cache TTL billing |
| `636d7debf` | Applied | Merge pull request #6132 from wucm667/fix/issue-6125-anthropic-cache-ttl |
| `eb594eefc` | Applied | fix(payment): refresh balance after fulfillment |
| `3c714d873` | Applied | Merge pull request #6152 from ranxi2001/fix/payment-result-balance-refresh |
| `a6b11ccce` | Applied | fix(openai): honor OpenCode Go usage reset durations |
| `41d712be0` | Applied | Merge pull request #5658 from william-drakemond/fix/opencode-go-usage-limit-reset |
| `4347e5555` | Applied | fix(composite): route Kimi Code K3 model IDs |
| `810d50a00` | Applied | Merge pull request #6155 from HypoxanthineOvO/fix/composite-kimi-k3-routing |
| `49752060f` | Applied | fix(monitor-v2): resolve composite group error facts to concrete account platform |
| `027d442f9` | Applied | Merge pull request #6101 from jianjianai/fix/composite-group-channel-monitor-v2 |
| `53d76ad80` | Applied | fix(openai): enforce Responses Lite tool call mode |
| `d6012b0b3` | Applied | fix(openai): preserve numeric precision in Lite payloads |
| `d5e43ef7d` | Applied | fix(openai): normalize Lite requests in WS HTTP bridge |
| `095b52536` | Applied | fix(openai): pin Responses Lite parallel tool calls |
| `5a7d46962` | Applied | Merge pull request #6157 from LiPu-jpg/fix/issue-6147-responses-lite-parallel |
| `aa2c4e8d1` | Already Applied + Overridden | chore: sync VERSION to 0.1.182 [skip ci] |

`Applied + Overridden` 的覆盖边界如下：

- `b1e60ba45`、`cf3577a3c`、`acce29af2`、`c374ff295`、`787f875dd`、`7e9af4c10`、`b2b2adcf8`、`1429e8f71`、`cef18b4ad`、`31d5b67ba`：接入 OpenAI Responses、compact fallback、协议转换、自定义工具和 failover 修复；继续以首个有效语义输出为提交边界，compact 首次失败撤销暂存响应，插件已发出请求后不得换号、同号重试或重放。
- `d493ce0bb`、`fa4587041`：Codex 身份只用于 OAuth 账号，继续保留本地稳定指纹和 HTTP/WS 一致性；影子账号审核仍落到父账号。
- `ed4207a16`、`39485f2e2`、`2e68b10aa`、`1bff06ea5`：接入 Grok 默认模型、官方目录、内容拒绝、媒体和 rollup 修复；继续保留严格请求模型计费、分组长上下文门禁以及官方“超过 200K”才进入高价阶梯的边界。
- `3fd66a33b`：接入 OpenAI load-batch 排除诊断，不改变本地连续失败停调度、自动测活和调度状态保护。
- `40ea3aeba`：接入 OAuth outbound transport 插件系统；插件管理纳入公共开关注册表并保持 opt-in，`RequestSent=true` 后各入口禁止任何可能重复发送的恢复操作，合法终态不被迟到的 close error 覆盖。
- `6466978d2`、`75faedda9`、`bc4a9ae43`、`f06bf181d`：接入统一 token 计费、Fast service tier 和 Anthropic cache TTL 修复；服务层级只降不升，继续保留严格缺价、分组/渠道优先级、长上下文门禁和 Grok `>200K` 边界。
- `cbe258fd1`、`73aabc861`：接入 Go 1.27.0、golangci-lint 2.13 与 jsonv2 适配；G703/G704 改为生产代码逐点说明和测试路径豁免，不放宽本地安全门禁。
- `6f972145b`：接入 OpenAI 重置卡阈值自动使用；自动用卡前重读账号并持久化恢复快照，只清除双时间戳及运行时实例 ID、generation、429 原因均匹配的同一限流代次，不把自动用卡当作通用测试成功。

`Already Applied + Overridden` 的覆盖边界如下：

- `03e8ab413`、`e2d9b823f`、`aa2c4e8d1`：上游版本分别推进到 `0.1.180`、`0.1.181` 和 `0.1.182`；本地已有更高版本链，完整 merge 仅保留祖先关系，最终继续使用 `0.1.225`。

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `502aa69168f338d4df4792d990602444ae99440c`；双亲为同步前本地 SHA `5b452d2289f9c73bccc01ad38dce6cb65af21cd3` 和固定上游 SHA `aa2c4e8d136b13553ac7bae3d76c25715333a554`。
- merge commit 单独相对 `LOCAL_PRE_SYNC_SHA` 变更 504 个文件，增加 41630 行、删除 3354 行；上游固定范围自身变更 503 个文件，增加 41063 行、删除 3181 行。
- 二次开发适配提交 `a5708ece931f3e0b6688b518f9c9d248f886b604` 相对 `LOCAL_PRE_SYNC_SHA` 变更 513 个文件：新增 107 个、修改 403 个、删除 2 个、重命名 1 个，增加 42738 行、删除 3436 行；相对固定上游目标变更 912 个文件，增加 128969 行、删除 5794 行，本地独有提交 493 个（不含本同步记录提交）。
- 主要新增：OAuth outbound transport 插件及管理界面、OpenAI 重置卡自动使用、Fast service tier、统一 token 计费与上下文阶梯查询、渠道工作日分时定价、Responses Lite 并行工具约束、模型列表读取上限，以及迁移 229、230。
- 主要修复：OpenAI Responses/compact/WS bridge、Grok 4.6/Realtime/媒体/计费、Codex 身份和父账号审核、DeepSeek 原生 Responses 工具、Antigravity/Gemini 工具 schema、Composite Kimi/监控路由、支付余额刷新、代理 IPv6 和运维错误详情。
- 上游删除 `backend/internal/service/channel_plaza.go`，并将 `channel_plaza_test.go` 重命名为 `model_plaza_service_test.go`；删除的 `docs/screenshots/mobile-account-actions-menu.png` 已无当前引用，未删除本地二次开发业务资源。
- `backend/cmd/server/VERSION` 最终保持 `0.1.225`；两份生产 Compose 保持字节完全一致，SHA-256 均为 `817B0DB0801F240F991331F4B3AB0F21C0F32844CCAD6EB35678CED69FAB66B0`。

### 冲突与最终解决方案

- 用户批准集中审批报告中列明的完整 merge、42 个文本冲突解决方案和本地验证范围；42 个冲突均逐文件解决，未整文件采用 `ours`、`theirs` 或上游版本，最终索引无未解决路径。
- 首 Token 和 compact fallback 继续以有效语义输出为边界；metadata、preamble、keepalive 与内部 fallback 信号不会提交空成功响应，二次 fallback 失败返回真实错误。
- 插件 `RequestSent=true` 后，HTTP、Responses、Chat、Messages、Images、Live 和 WS bridge 路径均禁止换号、同号重试、compact retry 或响应体读取错误重放；合法流式终态之后的 close error 不覆盖成功。
- 自动用卡的幂等 owner 在首次出站前把数据库限流双时间戳与运行时实例/代次/原因快照持久化到账号 `Extra`，并随幂等成功结果保存供回放复用；后处理失败后的回放复用原始快照，升级前缺快照的旧记录仅刷新额度，人工重置仍保留广义恢复和 Token 失效。
- 同一账号的活动 429 与非额度运行时阻断原因按保守规则合并：任一原因不是明确 429 或旧原因缺失时整体按非额度处理，纯 429 才允许自动用卡按实例和 generation 清除；过期旧原因不污染新的 429 阻断。
- OpenAI 429 先安装运行时阻断并成功持久化限流状态，再通知自动用卡；持久化失败时不发送信号，避免消费先于限流代次可见。
- 统一 token 计费路径继续执行严格缺价、分组逐模型/渠道/动态/内置优先级和长上下文门禁；实际服务层级只允许向较低档位修正，不得升级收费，Grok 200K 本身仍在基础阶梯。
- Codex 身份只作用于 OAuth 账号且影子账号审核保持父账号归属；调度诊断只补充排除原因，不清除本地 failure marker/streak 或人工、临时和模型级停调度。
- 插件管理作为 opt-in 公共开关接入统一注册表，Canvas、每日签到、本地模型市场和渠道 quota 原有默认语义不变。
- CI 基线采用 Go 1.27.0 和 golangci-lint 2.13，gosec G703/G704 逐点处置；两份生产 Compose、bind mount、仅回环暴露、HTTP upstream 业务开关和 4 vCPU/8 GiB 参数均未被放宽。

### 刻意保留的二次开发功能

- `CUST-ACC-004`、`CUST-GW-001`、`CUST-GW-006`、`CUST-GW-008`：额度重置代次隔离、首语义输出边界、连续失败状态保护和插件请求不可重放。
- `CUST-PROTO-001`、`CUST-PROTO-004`、`CUST-PROTO-005`、`CUST-PROTO-006`：Codex 稳定身份、Claude 工具清洗、请求头覆写及 Responses/compact/WS 路由边界。
- `CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`：严格请求模型计费、多层定价、长上下文门禁和服务层级费用展示。
- `CUST-OBS-001`、`CUST-OBS-002`、`CUST-PROD-001`、`CUST-PROD-002`、`CUST-PROD-007`、`CUST-RISK-002`、`CUST-UI-002`：流式渠道监控、独立上游同步、每日签到、本地模型市场、Canvas、cyber 用量边界和公共设置白名单。
- `CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005`：生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性、实例性能参数、按用户串行扣费和 5 秒 usage task 超时。

### 验证记录

本机系统 Go 启动器为 1.26.3，backend module 通过 `GOTOOLCHAIN=auto` 实际使用 Go 1.27.0；Node 为 24.15.0，通过 Corepack 使用 pnpm 9.15.9。系统 golangci-lint 为 2.9.0，本次在仓库临时目录隔离安装并验证 CI 对应的 golangci-lint 2.13.0；CI Node 基线为 20。

| 阶段 | 命令或检查 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前后端 | `go test -tags=unit ./...`、`golangci-lint run --timeout=30m ./...`、`CGO_ENABLED=0 go build -trimpath ./cmd/server` | 0 | Go 全量 unit、系统 golangci-lint 2.9.0 和 server 构建通过，作为同步后比较基准 |
| 同步前前端 | `corepack pnpm install --offline --frozen-lockfile`、`lint:check`、`typecheck`、`test:run`、`build` | 0 | frozen install、lint/typecheck 通过；257 个测试文件、1802 项测试通过；生产构建通过 |
| 同步前 Canvas | `corepack pnpm install --offline --frozen-lockfile`、`format:check`、`typecheck`、`test`、`build` | 0 | frozen install、格式/类型检查通过；8 个测试文件、34 项测试通过；生产构建通过 |
| 兼容回归 | `go test -tags=unit ./internal/handler -count=1`、`./internal/service`、`./internal/service/openai_ws_v2` | 0 | OpenAI handler、service 与 WSv2 定向 unit 通过 |
| 同步后后端 | 默认标签 Go 全包测试、`go test -tags=unit ./... -count=1` | 0 | 默认标签与 unit 标签全包测试通过 |
| 同步后后端 | `CGO_ENABLED=0 go build -trimpath ./...` | 0 | 全包构建通过 |
| 同步后后端 | golangci-lint 2.13.0 `run ./...` | 0 | CI 对应版本隔离运行，结果为 `0 issues` |
| 同步后前端 | frontend `lint:check`、`typecheck`、`test:run`、`build` | 0 | lint/typecheck 通过；264 个测试文件、1881 项测试通过；生产构建通过 |
| 同步后 Canvas | canvas `typecheck`、`test`、`format:check`、`build` | 0 | 类型和格式检查通过；8 个测试文件、34 项测试通过；生产构建通过 |
| 部署静态 | Git Bash shell 语法；Compose security/gateway env/resources、Caddy cache 和假 GitHub Token 隔离测试 | 0 | 除单列的 Apple fixture 外均通过；未读取真实 Token 或 `.env`，未联网部署 |
| Apple fixture | `bash deploy/tests/apple-container-test.sh` | 1 | Windows Git Bash 的 `stat -f '%Lp'` 与 macOS 语义不兼容，属于本机平台限制 |
| repository integration 仅编译 | `go test -tags=integration ./internal/repository -run '^$' -count=1` | 0 | integration 标签下 repository 包编译通过，未执行测试用例 |
| Compose 一致性 | 字节比较及 SHA-256 | 0 | `docker-compose.yml` 与 `docker-compose.sub2api.yml` 字节一致，SHA-256 均为 `817B0DB0801F240F991331F4B3AB0F21C0F32844CCAD6EB35678CED69FAB66B0` |
| 最终静态复核 | `git diff --check`、冲突标记、意外删除、凭据类路径和祖先关系 | 0 | 适配提交后无空白错误或未解决 Git 冲突；固定上游目标为同步分支祖先，版本、Compose 和受控文件范围符合批准方案 |
| 远端同步分支与 PR | GitHub Actions `CI` #329/#330、`Security Scan` #185/#186 | 0 | push 与 pull_request 两套 shell、Go unit/integration、Node 20 前端/Canvas、golangci-lint 2.13、govulncheck 和依赖审计全部通过；CLA 两项按仓库条件 skipped |
| 远端 `main` | GitHub Actions `CI` #331、`Security Scan` #187 | 0 | merge commit `742b90442d315ec3dd61d2d9965d9738e6d2c3c0` 的 shell、Go unit/integration、Node 20 前端/Canvas、lint、安全与依赖审计全部通过 |

### 远端交付补记

- 经用户后续明确授权，同步分支 `sync/upstream-20260825-aa2c4e8d1` 已推送，并通过 [PR #15](https://github.com/Saviour2411/sub2api/pull/15) 合入远端 `main`。
- PR head 为 `aabd4f02a3aed47ed6c95c52e4c3218d8140f9b3`，base 为 `5b452d2289f9c73bccc01ad38dce6cb65af21cd3`；全部 push/PR 检查成功后使用 merge 方法合入，远端 merge commit 为 `742b90442d315ec3dd61d2d9965d9738e6d2c3c0`。
- 主线 merge commit 触发的 [CI #331](https://github.com/Saviour2411/sub2api/actions/runs/32872480461) 与 [Security Scan #187](https://github.com/Saviour2411/sub2api/actions/runs/32872480398) 再次全部通过；未通过跳过检查、取消任务或绕过保护规则完成合入。

### 未验证项与残余风险

- 本机未运行 Docker/Testcontainers 或完整 integration；GitHub Actions 的 Docker integration 已在同步分支、PR 和主线三次通过。真实生产数据上的 PostgreSQL/Redis 和迁移 229、230 执行仍未验证。
- 自动用卡快照已在首次出站前持久化；若幂等成功结果由另一实例回放，或原 owner 在清理本地运行时阻断前退出，其他实例不能清除原实例内存中的阻断，只能等待其自然过期。这是保守的短暂可用性延迟，不会误清新的限流或非额度故障代次。
- 未运行 `-race`；`govulncheck` 已在三次 GitHub Security Scan 中通过，golangci-lint 2.13.0 已在本地和三次 GitHub CI 中通过。
- 本机 Node 24.15.0 与 CI Node 20 不同；Node 20 的前端和 Canvas 检查已在同步分支、PR 和主线三次通过。
- 未使用真实 OpenAI、Anthropic、Grok、CN Provider 凭据或真实 OAuth 插件包；真实额度消费、媒体、流式故障转移和插件进程端到端未验证。
- 未执行依赖 PostgreSQL/Redis 的本地完整服务和 `/health` 检查。
- 未读取 `.env`、SSH 私钥或其他业务秘密；指定 GitHub Token 仅在子进程中从 `D:\project\github_token.sh` 加载，用于 GitHub API 认证，未输出、写入 Git 配置或提交。经后续授权仅执行同步分支 push、PR #15 和主线 merge，未执行部署、远程服务器访问、容器重启、生产挂载核验或生产数据操作。
- 生产服务器状态、bind mount 实际挂载、实例性能参数和健康检查均未验证；本次只验证仓库内静态 Compose 约束。

## 2026-08-30 同步至 b5827cfd5

- 执行时间：2026-08-29T23:18:44+08:00 至 2026-08-30T01:23:48+08:00
- 执行状态：同步分支已完整合并固定上游范围、完成二次开发适配并通过本地验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`8e4a0b6456a67b9b08a3670d88190cf3da379a6b`
- 上游代码合并提交：`1095fc9428f6ebd1dd8f0bbc5d2b4c60353555eb`
- 最后一个代码提交：`ffc464dffe86915758d86fc1d3d1da907492c5a4`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`aa2c4e8d136b13553ac7bae3d76c25715333a554`
- `UPSTREAM_NEW_SHA`：`b5827cfd54d58c248a9480b800444d0b40f0c6ea`
- merge-base：`aa2c4e8d136b13553ac7bae3d76c25715333a554`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`b5827cfd54d58c248a9480b800444d0b40f0c6ea`
- 集成策略：在隔离同步分支执行 `git merge --no-ff --no-commit b5827cfd54d58c248a9480b800444d0b40f0c6ea`，逐文件解决 20 个文本冲突并复核自动合并路径；merge commit 保留完整上游祖先关系，二次开发兼容调整与两份历史台账使用后续独立提交
- 备份分支：`backup/pre-upstream-sync-20260829-231844-8e4a0b645`
- 同步分支：`sync/upstream-20260829-b5827cfd5`

### 上游提交处置

固定范围共 136 个提交，其中 63 个 merge commit、73 个 non-merge commit。审批分类为 23 个 `Conflict`、47 个 `Review`、3 个 `Low Risk` 和 63 个依赖合并；最终互斥处置结果为 113 个 `Applied`、23 个 `Applied + Overridden`，`Already Applied`、`Skipped`、`Deferred` 和未解决 `Conflict` 均为 0。完整 merge 已保留全部 136 个提交的祖先关系，下表覆盖固定范围内全部 SHA。

| 上游提交 | 集成状态 | 审批分类 | 内容与处置 |
| --- | --- | --- | --- |
| `901a77cfb` | Applied | Review | Anthropic→Chat 桥回传工具调用的 thinking，修复 DeepSeek 多轮兼容 |
| `3c5553e25` | Applied | Review | 为网关合成的 Responses 对象补齐 created_at |
| `b1737cc84` | Applied | Review | 保留 Antigravity 混合内置 Chat 工具 |
| `22e1b8144` | Applied | Review | 新增按实际分组路由生成的 Codex 模型目录 |
| `e471be730` | Applied | Review | 补齐路由 Codex 模型目录字段与能力 |
| `3e98a5a1a` | Applied | Review | Composite 精确路由账号模型别名 |
| `b16ed03ca` | Applied | Review | 使 Codex 目录与实际路由结果一致 |
| `5a2f542ab` | Applied | Review | 目录生成优先采用管理员配置模型 |
| `e39fce270` | Applied + Overridden | Conflict | 同步路由账号能力元数据，并与本地创建预览及模型市场边界合并 |
| `e0e5e45cd` | Applied | Review | 恢复工具调用项时保持类型化 ID |
| `fc589bce1` | Applied | Review | 修复路由 Codex 目录审查问题 |
| `b7ec3cdad` | Applied + Overridden | Conflict | 识别 raw Chat Completions 缺少终态的截断流，并保留本地首 Token/断开计费边界 |
| `e55727d4c` | Applied | Review | 容量换号时保留 sticky 绑定 |
| `3f1581b2d` | Applied + Overridden | Conflict | 避免上游倍率探测导致账号列表整页刷新，并保留本地账号预览契约 |
| `4ca86c52e` | Applied | Review | 为邮箱换绑增加别名与并发守卫 |
| `1a9898a6a` | Applied | Review | 限制 Antigravity 兼容路径的 Token 上限 |
| `11ada80d5` | Applied + Overridden | Conflict | 记录并展示策略映射前的请求推理强度，保留双值审计 |
| `5705f4a4a` | Applied + Overridden | Conflict | 用户端隐藏映射后推理强度，管理员继续可见 |
| `a8cfe746b` | Applied + Overridden | Conflict | 覆盖用户端与管理端推理强度展示边界 |
| `b20f29d11` | Applied | Review | 修复 Channel Monitor V2 的 Composite 聚合 SQL |
| `32064d39e` | Applied | Review | 规范跨供应商 reasoning 回放 |
| `8e60d5747` | Applied | Review | 使用 Codex 会话 ID 请求头参与粘性路由 |
| `3802268e2` | Applied | Review | 保持 Kimi 并发 403 可恢复 |
| `db01fb98f` | Applied | Review | 账号临时不可调度时仍保持 Codex 目录能力稳定 |
| `5934981e2` | Applied | Review | 覆盖不可调度账号参与目录能力交集的回归 |
| `f1aadd48d` | Applied + Overridden | Conflict | 额度耗尽 429 时暂停 OpenAI OAuth 账号，并合并本地失败调度语义 |
| `77b6c5bb0` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `832cf4df6` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `4ff136cfd` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `804042871` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `fa0685a4a` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `4952b919a` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `49ad7021d` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `7a7bd3729` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `e8cb019fa` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `d8694f03b` | Applied | Review | 覆盖 WSv2 清理过期原生工具 ID 的回归 |
| `7634e3c23` | Applied + Overridden | Conflict | 保留版本提交祖先关系，最终继续使用本地 0.1.226 |
| `d522aed65` | Applied | Review | OAuth 注册继续保留优惠码 |
| `4795650d2` | Applied + Overridden | Conflict | 错误路径记录实际上游端点，并与本地端点归因统一 |
| `66d664ff0` | Applied | Low Risk | 更新赞助商说明与静态资源 |
| `195b21970` | Applied | Review | 隔离 API Key Codex 目录缓存并补充 DeepSeek 默认值 |
| `6ca1e15b0` | Applied | Low Risk | 更新赞助商说明 |
| `2abce6503` | Applied + Overridden | Conflict | 加固路由目录能力同步，并保留本地账号创建预览数据流 |
| `1bf76d26d` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `b9083fc7a` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `50ba14629` | Applied | Review | 保留多模态客户端工具输出 |
| `8ba81615e` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `efb46db0a` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `d881bfc0d` | Applied | Review | 避免 Pool 两跳重复计算 system 提示词 |
| `44003d7f6` | Applied + Overridden | Conflict | 将 Anthropic/Bedrock 传输错误统一转为 failover，同时保留插件已发出请求不可重放 |
| `5f09442fc` | Applied | Review | 额度重置后刷新 OpenAI 用量 |
| `d077002eb` | Applied | Review | 图片工具冷却不再由模型回文字触发 |
| `9192426d2` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `4a02e5417` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `00b7c855c` | Applied | Review | 保留 API Key 已声明的 namespace 调用 |
| `00efee430` | Applied | Review | 为 Grok 4.6 广告 xhigh 推理档位 |
| `9e7aff59d` | Applied | Review | 版本比较时剥离连字符后缀 |
| `e6ea7b9af` | Applied | Review | 图像能力丢失时冷却图片调度 |
| `de6ef7134` | Applied | Review | 清洗 Grok Codex Responses 请求 |
| `f4820c00d` | Applied | Review | 简化非法工具 union 根节点 |
| `fd872550d` | Applied | Review | 处理带类型的非法工具 union |
| `f4e3eb1c5` | Applied + Overridden | Conflict | 在 WSv2 透传中识别 Cyber 策略，并保留本地严格门禁及 AfterTurn 顺序 |
| `b56c61ecc` | Applied | Review | 允许管理员限制用户可访问的公开分组 |
| `60756c0ca` | Applied + Overridden | Conflict | Responses 透传首输出前发送 SSE 保活，并保持首 Token 暂存边界 |
| `5929cdd38` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `acd2f09dd` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `0eddfe1cf` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `96e9ab866` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `1e8745c88` | Applied | Review | 补全 EasyPay 返回的相对支付与二维码地址 |
| `d36f4dd6c` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `ee2c8b97b` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `f83fe6435` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `d24de611f` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `81ac8ccd6` | Applied + Overridden | Conflict | 非流式路径按流式同一规则处理 HTTP 200 终态失败并安全换号 |
| `eca8d6b9a` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `28fa458dc` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `0f6ad105f` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `002aaaa3d` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `5eb8628ff` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `f7dca22ea` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `7eed2d3b7` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `de084cdfc` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `fafc4d288` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `a0b313018` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `4e9c1d7c8` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `b588c0bc4` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `443537daa` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `59bb131df` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `dc332d141` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `add86cc3b` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `9b61c1bdd` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `d674a04f2` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `c83dced4b` | Applied + Overridden | Conflict | fix(openai): 入站 WS 的客户端正常关闭与断开不再计为账号故障 |
| `e866ff6ec` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `1e6926cb9` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `446042e51` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `c8a6e93f3` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `423f89575` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `d4754c211` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `7b693ae42` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `0756c9810` | Applied | Review | 批量编辑显式提交关闭 Codex 指纹收敛 |
| `c4e46c3be` | Applied + Overridden | Conflict | 支持智谱 Team GLM Coding Plan 用量查询，并合并账号创建表单 |
| `e652f6e20` | Applied | Review | 配额抓取在 singleflight 内重查缓存，消除重复查询 |
| `02eee39dd` | Applied + Overridden | Conflict | 充值预览显示所选币种，并保留本地赠送与到账余额口径 |
| `c5ff640df` | Applied | Low Risk | 固定 rollup 触发器集成测试的会话时区 |
| `5345881b1` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `da10822d7` | Applied | Review | 保留 Anthropic 工具参数 |
| `c03776604` | Applied | Review | 保留 Claude attribution 请求头 |
| `32ad1dcdc` | Applied | Review | 对齐订阅周/月窗口展示与实际重置锚点 |
| `88cb79d8b` | Applied | Review | Grok 缓存身份优先采用客户端 prompt_cache_key |
| `5688bcba9` | Applied + Overridden | Conflict | Messages 粘性路由采用 Claude Code 会话，并保留本地缓存滚动边界 |
| `f1d845c63` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `1323d1645` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `6506c0ea6` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `1136e290f` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `25dc5742e` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `604360d1c` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `3045b3ade` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `46598dd49` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `ca319d09f` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `3c2e9a43b` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `fd13c72e9` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `b83284071` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `ac18c588c` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `5d9c7abed` | Applied + Overridden | Conflict | 将 Spark 配额 429 限定到模型范围，并合并本地停调度保护 |
| `3c22e78af` | Applied + Overridden | Conflict | 保留 Spark 配额重置语义 |
| `571d1e1d9` | Applied + Overridden | Conflict | 隔离 WebSocket 语义限流状态 |
| `804679d99` | Applied + Overridden | Conflict | 流式 failover 保留模型范围 |
| `987588eaa` | Applied | 依赖合并 | 完整 merge 保留祖先关系；具体功能与处置由关联的 non-merge 提交说明 |
| `c31fe2ed9` | Applied | Review | SMTP 测试端点保留已保存的 TLS 模式 |
| `706b5676a` | Applied | Review | 分组创建和更新时展示 API 错误信息 |
| `eb4237a2b` | Applied | Review | 带后缀模型的渠道定价不再被官方兜底价覆盖 |
| `ed12ea716` | Applied | Review | Codex API Key 模式改为内联认证 |
| `32ac921f2` | Applied | Review | 避免 Fable OAuth system prompt 被拒绝 |
| `ea4291a92` | Applied | Review | 仅对 Fable 模型应用 Fable 调度阈值 |
| `b5827cfd5` | Applied + Overridden | Conflict | 按 DeepSeek 官方工作日峰谷价格修正默认价卡，同时保留分组/渠道自定义价优先 |


### `Applied + Overridden` 覆盖边界

- `e39fce270`、`3f1581b2d`、`2abce6503`：接入路由账号能力元数据、Codex 模型目录和倍率探测修复；继续保留本地账号创建预览契约、API Key 专属分组约束和本地模型市场边界。
- `11ada80d5`、`5705f4a4a`、`a8cfe746b`：记录策略映射前的 requested reasoning effort 和映射后实际值；管理员可审计双值，用户端不暴露映射后强度。
- `b7ec3cdad`、`60756c0ca`、`81ac8ccd6`：接入 raw Chat 流截断识别、响应前 SSE 保活和非流式 HTTP 200 终态失败换号；继续以首个有效语义输出作为提交边界，保留首 Token 暂存、请求体释放和断开排空计费。
- `44003d7f6`、`f4e3eb1c5`：接入 Anthropic/Bedrock 统一传输错误和 WSv2 Cyber 识别；插件 `RequestSent=true` 后仍禁止换号、同号重试或重放，Cyber 严格门禁和 `AfterTurn` 顺序不变。
- `f1aadd48d`、`5d9c7abed`、`3c22e78af`、`571d1e1d9`、`804679d99`：接入 OpenAI 额度耗尽停用、Spark 模型级限流、重置窗口、WS 状态隔离和流式模型级 failover；继续保留本地失败代次、连续失败停调度和模型范围保护。
- `4795650d2`：错误记录采用实际上游端点，并与本地渠道监控归因口径统一。
- `7634e3c23`：完整 merge 保留版本提交祖先关系，最终继续使用本地版本 `0.1.226`。
- `c83dced4b`：接入客户端正常 WebSocket 关闭归因，正常关闭或断开不再记为账号故障，同时保留本地会话结算边界。
- `c4e46c3be`：接入智谱 Team GLM Coding Plan 用量查询，并与本地账号创建预览表单合并。
- `02eee39dd`：充值预览显示所选币种，同时保留本地赠送额、到账余额和汇率口径。
- `5688bcba9`：Messages 粘性路由采用 Claude Code 会话标识，同时保留本地缓存滚动和稳定身份边界。
- `b5827cfd5`：按 DeepSeek 官方工作日峰谷价修正内置默认价卡；严格缺价、请求模型计费及分组/渠道自定义价格优先级不变。

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `1095fc9428f6ebd1dd8f0bbc5d2b4c60353555eb`；双亲为同步前本地 SHA `8e4a0b6456a67b9b08a3670d88190cf3da379a6b` 和固定上游 SHA `b5827cfd54d58c248a9480b800444d0b40f0c6ea`。
- 二次开发适配提交为 `ffc464dffe86915758d86fc1d3d1da907492c5a4`，包含测试签名适配、raw Chat 原始边界测试修正、移除两处已被严格计费链路替代的遗留辅助函数、迁移注释中文化，以及 `docs/custom-development-history.md` 更新。
- 同步代码与适配相对 `LOCAL_PRE_SYNC_SHA` 共变更 255 个文件：新增 35 个、修改 220 个、删除 0 个，增加 17326 行、删除 908 行；本同步记录自身另修改 `docs/upstream-sync-history.md`。
- 主要新增：按实际分组路由的 Codex 模型目录与能力元数据、requested reasoning effort 审计字段、用户公开分组限制、DeepSeek 工作日峰谷默认价、EasyPay 相对地址补全，以及数据库迁移 231。
- 主要修复：raw Chat 截断、响应前保活、非流式终态失败换号、WebSocket 正常关闭归因、实际上游端点记录、跨 Provider 传输错误、Spark 模型级限流、Anthropic reasoning 回放、Composite 监控聚合和账户用量缓存。
- `backend/cmd/server/VERSION` 最终保持 `0.1.226`；两份生产 Compose 字节完全一致，SHA-256 均为 `817B0DB0801F240F991331F4B3AB0F21C0F32844CCAD6EB35678CED69FAB66B0`。

### 文本冲突与最终解决方案

- 20 个文本冲突为：`backend/cmd/server/VERSION`；`backend/internal/handler/admin/account_handler.go`、`account_handler_available_models_test.go`、`openai_gateway_handler.go`；`backend/internal/service/billing_service.go`、`gateway_bedrock.go`、`openai_compact_sse_keepalive.go`、`openai_gateway_cc_pipeline.go`、`openai_gateway_chat_completions_raw.go`、`openai_gateway_passthrough.go`、`openai_gateway_response_handling.go`、`openai_upstream_transport_error.go`、`openai_ws_forwarder_v2.go`、`usage_log_helpers.go`；`frontend/src/api/admin/accounts.ts`、`frontend/src/components/account/CreateAccountModal.vue`、`frontend/src/components/account/__tests__/ModelWhitelistSelector.spec.ts`、`frontend/src/views/admin/UsageView.vue`、`frontend/src/views/user/PaymentView.vue`、`frontend/src/views/user/UsageView.vue`。
- 版本冲突保留本地 `0.1.226`；账号和模型目录冲突接入上游能力同步、公开分组及智谱用量能力，同时保留本地创建预览、API Key 分组和模型市场契约。
- 网关、流和 WS 冲突组合接入上游截断识别、保活、终态失败、正常关闭和传输错误处理；首语义输出、插件不可重放、断开计费、请求体释放、Cyber 与失败代次边界保持不变。
- 计费冲突接入 DeepSeek 峰谷默认价和媒体实际模型识别；严格请求模型计费、Composite 例外及分组/渠道自定义价优先级保持不变。
- 前端冲突按角色展示 requested reasoning effort，补齐 EasyPay 相对地址和币种预览，并保留管理员/用户可见性、赠送额与到账余额口径。
- 所有冲突均逐文件解决，未整文件采用 `ours`、`theirs` 或上游版本；最终索引无未解决路径。另复核自动合并路径和 23 个语义覆盖提交。

### 刻意保留的二次开发功能

- `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`：首语义输出暂存、响应前心跳、断开排空计费、请求体释放、连续失败状态和插件请求不可重放。
- `CUST-PROTO-001`、`CUST-PROTO-005`、`CUST-PROTO-006`：Codex 稳定身份、请求头覆写及 Responses/compact/WS 路由边界。
- `CUST-ACC-001`、`CUST-ACC-002`、`CUST-ACC-003`、`CUST-ACC-006`：账号状态代次、创建预览、API Key 专属分组和公开分组约束。
- `CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-005`：严格请求模型计费、媒体实际模型、Composite 例外、多层定价优先级和费用展示。
- `CUST-PROD-002`、`CUST-PROD-006`、`CUST-RISK-002`、`CUST-UI-004`：本地模型市场、充值赠送/到账余额、Cyber 严格门禁和表格浮层边界。
- `CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005`：生产 bind mount、仅回环暴露、HTTP upstream 业务开关、双生产 Compose 一致性、4 vCPU/8 GiB 参数、按用户串行扣费和 5 秒 usage task 超时。

### 验证记录

本机后端通过 Go 自动工具链实际使用 Go 1.27.0；前端使用本机 Node 环境和 Corepack pnpm，Canvas 以 pnpm 9.15.9 离线冻结安装。golangci-lint 使用与 CI 对齐的 v2.13.0。

| 阶段 | 命令或检查 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步前基线 | 后端默认标签与 unit 标签测试、CGO 关闭构建、前端 lint/typecheck/Vitest/build、Canvas format/typecheck/Vitest/build | 0 | 核心基线通过；Apple fixture 单列的平台失败与同步后相同 |
| 同步前部署静态 | Compose security、Gateway env、Docker resources、Caddy cache | 混合 | security 通过；Gateway env、Docker resources、Caddy 三项既有基线失败在同步后均修复并通过 |
| 同步后后端 | `go test ./...` | 0 | 默认标签全包测试通过 |
| 同步后后端 | `go test -tags=unit ./...` | 0 | unit 标签全包测试通过；`internal/service` 用时 191.529 秒 |
| 同步后后端 | `CGO_ENABLED=0 go build -trimpath ./...` | 0 | CGO 关闭全包构建通过 |
| repository integration 仅编译 | `go test -tags=integration ./internal/repository -run '^$' -count=1` | 0 | integration 标签下 repository 包编译通过，未执行测试用例 |
| 后端静态检查 | golangci-lint v2.13.0 `run ./...` | 0 | CI 对应版本与 Go 1.27.0 运行，结果为 `0 issues` |
| 同步后前端 | `lint:check`、`typecheck`、`test:run`、`build` | 0 | lint/typecheck 通过；268 个测试文件、1920 项 Vitest 通过；生产构建通过 |
| 同步后 Canvas | 离线 `install --frozen-lockfile`、`format:check`、`typecheck`、`test`、`build` | 0 | pnpm 9.15.9 冻结安装、格式和类型检查通过；8 个测试文件、34 项 Vitest 通过；生产构建通过 |
| Ent 生成稳定性 | 隔离环境重复生成两轮并比较受控输出 | 0 | 两轮一致，SHA-256 `02BFACF08F167CA912DD860DD3B458B4633A1660E1CCAD0C1F065028D45DC86E` |
| Wire 生成稳定性 | 隔离环境重复生成两轮并比较受控输出 | 0 | 两轮一致，SHA-256 `15BB73C16A003BC0DD02333B402E2A707827D504066A6BCE1B1F17824A4B3470` |
| 部署静态 | shell 语法、`docker-compose-security-test.sh`、`docker-compose-gateway-env-test.sh`、`docker-runtime-resources-test.sh`、`test-caddyfile-cache.sh` | 0 | 全部通过；未读取真实 Token、`.env` 或连接外部系统 |
| Apple fixture | `bash deploy/tests/apple-container-test.sh` | 1 | Windows 不支持 BSD `stat -f '%Lp'`，与同步前一致，属于本机平台限制 |
| Compose 一致性 | 字节比较及 SHA-256 | 0 | `docker-compose.yml` 与 `docker-compose.sub2api.yml` 字节一致，哈希均为 `817B0DB0801F240F991331F4B3AB0F21C0F32844CCAD6EB35678CED69FAB66B0` |
| 最终静态复核 | `git diff --check`、冲突标记、意外删除、凭据类候选和祖先关系 | 0 | 适配提交后无空白错误或未解决 Git 冲突；固定上游目标为同步分支祖先，版本、Compose 和受控文件范围符合批准方案 |
| 远端同步分支与 PR | GitHub Actions `CI` #336/#337、`Security Scan` #192/#193 | 0 | push 与 pull_request 两套 shell、Go unit/integration、Node 20 前端/Canvas、golangci-lint 2.13、govulncheck 和依赖审计全部通过；CLA #41/#42 按仓库条件 skipped |
| 远端 `main` | GitHub Actions `CI` #338、`Security Scan` #194 | 0 | merge commit `4e75ac73847dfabcd05b69e9f87be8c1958e4984` 的 shell、Go unit/integration、Node 20 前端/Canvas、lint、安全与依赖审计全部通过 |

### 远端交付补记

- 经用户后续明确授权，同步分支 `sync/upstream-20260829-b5827cfd5` 已推送，并通过 [PR #17](https://github.com/Saviour2411/sub2api/pull/17) 合入远端 `main`。
- PR head 为 `769e5dfd977a3464ba35eb9ec0f989693b27aa5b`，base 为 `8e4a0b6456a67b9b08a3670d88190cf3da379a6b`；全部 push/PR 检查成功后使用 merge 方法合入，远端 merge commit 为 `4e75ac73847dfabcd05b69e9f87be8c1958e4984`。
- 主线 merge commit 触发的 [CI #338](https://github.com/Saviour2411/sub2api/actions/runs/33266733682) 与 [Security Scan #194](https://github.com/Saviour2411/sub2api/actions/runs/33266733687) 再次全部通过；未通过跳过检查、取消任务或绕过保护规则完成合入。

### 未验证项与残余风险

- 本机未运行 Docker/Testcontainers 或完整 integration；GitHub Actions 的 Docker integration 已在同步分支、PR 和主线三次通过。真实生产数据上的 PostgreSQL/Redis 和迁移 231 执行仍未验证。
- 未运行 `-race`；`govulncheck` 已在三次 GitHub Security Scan 中通过，golangci-lint 2.13.0 已在本地和三次 GitHub CI 中通过。
- 本机未使用 CI 的 Node 20 基线；Node 20 的前端和 Canvas 检查已在同步分支、PR 和主线三次通过。
- 未使用真实 OpenAI、Anthropic、DeepSeek、Grok、CN Provider 凭据或真实 OAuth 插件包；真实额度消费、媒体、流式故障转移、第三方支付和插件进程端到端未验证。
- 未启动依赖 PostgreSQL/Redis 的完整本地服务，未执行 `/health` 检查。
- 未读取 `.env`、SSH 私钥或其他业务秘密；指定 GitHub Token 仅在临时进程环境中从 `D:\project\github_token.sh` 加载，用于 GitHub API 认证，未输出、写入 Git 配置或提交。Git HTTPS smart-protocol 连接失败后仅使用仓库既有 SSH 认证推送同步分支；经后续授权完成 PR #17 和主线 merge，未执行部署、远程服务器访问、容器重启、生产挂载核验或生产数据操作。
- 生产服务器状态、bind mount 实际挂载、实例性能参数和健康检查均未验证；本次仅验证仓库内静态 Compose 约束。

## 2026-09-02 同步至 5097b3145

- 执行时间：2026-09-02T23:30:45+08:00
- 执行状态：同步分支已完整合并固定上游范围、完成二次开发适配并通过当前环境可执行的本地验证；本记录提交后使用 `--ff-only` 更新本地 `main`
- 本地目标分支：`main`
- `LOCAL_PRE_SYNC_SHA`：`499114a87384de74b3db987299f29c57217a2ae2`
- 上游代码合并提交：`5be874e91ef37c35065ae42e92956d49cee32dea`
- 最后一个代码/测试提交：`35c28e324b17d9e4dcef877a230d803fc6815249`
- 上游仓库：`https://github.com/Wei-Shaw/sub2api.git`
- 上游分支：`main`
- `UPSTREAM_OLD_SHA`：`b5827cfd54d58c248a9480b800444d0b40f0c6ea`
- `UPSTREAM_NEW_SHA`：`5097b31457e6dc9f49e5f5c9c72b925ce79543b3`
- merge-base：`b5827cfd54d58c248a9480b800444d0b40f0c6ea`
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA`：`5097b31457e6dc9f49e5f5c9c72b925ce79543b3`
- 集成策略：在隔离同步分支执行 `git merge --no-ff --no-commit 5097b31457e6dc9f49e5f5c9c72b925ce79543b3`，逐文件解决 17 个文本冲突并复核自动合并路径；merge commit 保留完整上游祖先关系，二次开发兼容调整与两份历史台账使用后续独立提交
- 备份分支：`backup/pre-upstream-sync-20260902-202638-499114a87`
- 同步分支：`sync/upstream-20260902-5097b31457`

### 上游提交处置

固定范围共 137 个提交，其中 42 个 merge commit、95 个 non-merge commit。最终互斥处置结果为 69 个 `Applied`、68 个 `Applied + Overridden`，`Already Applied`、`Skipped`、`Deferred` 和未解决 `Conflict` 均为 0。42 个 merge commit 均计入 `Applied`，68 个覆盖项均为 non-merge；完整 merge 已保留全部 137 个提交的祖先关系，下表覆盖固定范围内全部 SHA。

| 上游提交 | 集成状态 | 内容与处置 |
| --- | --- | --- |
| `92a550973` | Applied + Overridden | 接入 OpenAI 重新授权、额度冷却原子清理、目录禁用账号过滤、passthrough 调度快照及模型冷却修复；继续保留本地失败代次、连续失败停调度、自动测活和账号默认策略 |
| `6ff771d3d` | Applied | 为 OpenAI 图片工具增加可配置冷却策略 |
| `b4b537164` | Applied | 保留 Grok Responses 视觉工具输出中的图片 |
| `1cc6999ad` | Applied + Overridden | 接入原生 compaction 用量类型与 API 契约；本地继续按请求端点归因，并保留 WS 逐轮独立结算和严格计费模型边界 |
| `1a61eb715` | Applied + Overridden | 接入原生 compaction 用量类型与 API 契约；本地继续按请求端点归因，并保留 WS 逐轮独立结算和严格计费模型边界 |
| `0aef702b6` | Applied + Overridden | 接入用量窗口统一展示和缓存提示横向滚动修复；保留本地 DataTable、列设置浮层、分页与滚动位置稳定性 |
| `7c616db07` | Applied + Overridden | 接入 passthrough WS 会话隔离、超大 bridge、空闲连接回收、容量错误和终态 close 修复；本地继续每轮重取请求模型/渠道映射、记录 payload hash、独立结算 usage、释放并发并在失败停调度后阻断后续轮 |
| `a3bbf33c0` | Applied | 渠道监控分组按用户可访问范围过滤 |
| `624e4eef6` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `8b4b3f4a9` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `2b8cb628b` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `d39fc491e` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `3a9070359` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `f323d8464` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `50ad6e2e5` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `82105f260` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `57c76584a` | Applied + Overridden | 接入出站/观测服务层级分离、fallback/WS 透传及 Codex priority 能力；实际响应只允许降低计费档位，OAuth-like `default` 不得把明确请求上调，Free Fast 用户费用仍按 Standard |
| `6532d5b61` | Applied + Overridden | 接入用量窗口统一展示和缓存提示横向滚动修复；保留本地 DataTable、列设置浮层、分页与滚动位置稳定性 |
| `9f1effd71` | Applied | 分组局部更新时保留额度限制字段 |
| `30b29e51e` | Applied | Ollama Cloud 用量窗口支持挂载在国产三家平台账号下 |
| `d5a012463` | Applied + Overridden | 接入 passthrough WS 会话隔离、超大 bridge、空闲连接回收、容量错误和终态 close 修复；本地继续每轮重取请求模型/渠道映射、记录 payload hash、独立结算 usage、释放并发并在失败停调度后阻断后续轮 |
| `8177f27aa` | Applied | 明确账号到期输入使用本地时区 |
| `d66bc88e6` | Applied | 将到期时间输入修复应用到源代码路径 |
| `9eabd2a5b` | Applied + Overridden | 接入账号成本定价、长上下文目录数据驱动、覆盖文件和 cache 字段哨兵；保留分组/渠道显式价优先、Gemini 200K 边际价、GPT-5.6 272K 与 Grok 200K 严格边界及 GPT-5.4 Pro 无目录基础价 |
| `263605779` | Applied | 补充英文到期时区文案 |
| `ae1bcdc25` | Applied | 补充中文到期时区文案 |
| `b7aca87fd` | Applied | 覆盖跨时区本地日期时间解析 |
| `81e461f65` | Applied | 严格解析账号本地到期时间 |
| `5778739cd` | Applied | 兑换码使用严格本地到期时间解析 |
| `3673702af` | Applied | 删除误放的上传组件副本 |
| `94edcd5d8` | Applied | 删除第二个误放的组件副本 |
| `897faea33` | Applied + Overridden | 接入 OpenAI 重新授权、额度冷却原子清理、目录禁用账号过滤、passthrough 调度快照及模型冷却修复；继续保留本地失败代次、连续失败停调度、自动测活和账号默认策略 |
| `e0ecb55d4` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `a4156eea1` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `50a9dd7a6` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `414489d15` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `fbc69322a` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `2e756b71f` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `03ab68768` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `a2d7d4118` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `f34735f46` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `1dd0f2e5d` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `36ee193e3` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `b369fbca1` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `0678b24d5` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `e7c029875` | Applied + Overridden | 接入账号成本定价、长上下文目录数据驱动、覆盖文件和 cache 字段哨兵；保留分组/渠道显式价优先、Gemini 200K 边际价、GPT-5.6 272K 与 Grok 200K 严格边界及 GPT-5.4 Pro 无目录基础价 |
| `1be69e56a` | Applied | 允许无 call id 的 delegation bootstrap |
| `fdf9751c1` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `c66e700f0` | Applied + Overridden | 接入 TTFT 管理设置、API 契约和可见性测试；保留本地监控超时、用户范围过滤和用量展示边界 |
| `8553c91c1` | Applied + Overridden | 接入 TTFT 管理设置、API 契约和可见性测试；保留本地监控超时、用户范围过滤和用量展示边界 |
| `0fcec63b6` | Applied + Overridden | 接入 TTFT 管理设置、API 契约和可见性测试；保留本地监控超时、用户范围过滤和用量展示边界 |
| `85b593fd2` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `8f5451587` | Applied | 修复 Anthropic→Responses 流式 thinking 前的 item 生命周期和 content_index |
| `863667ce6` | Applied | 数据库启动遇临时错误时重试 |
| `e98ef32eb` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `1dc0a0900` | Applied + Overridden | 接入 passthrough WS 会话隔离、超大 bridge、空闲连接回收、容量错误和终态 close 修复；本地继续每轮重取请求模型/渠道映射、记录 payload hash、独立结算 usage、释放并发并在失败停调度后阻断后续轮 |
| `b6a7b8b7d` | Applied | 数据库仓储启动临时错误最多重试八次 |
| `52374af94` | Applied + Overridden | 完整保留上游版本提交的祖先关系，最终不采用上游 `0.1.184`、`0.1.185` 或 `0.2.0`，继续使用本地 `0.1.228` |
| `7c01ec9be` | Applied + Overridden | 接入按模型 exact/prefix/suffix 限制 reasoning effort 和超限 deny/downgrade；本地保持 exact 优先、较长 affix 次之、全局回退及默认降档边界 |
| `ba345f105` | Applied + Overridden | 接入 OpenAI 重新授权、额度冷却原子清理、目录禁用账号过滤、passthrough 调度快照及模型冷却修复；继续保留本地失败代次、连续失败停调度、自动测活和账号默认策略 |
| `e21b849a9` | Applied | API Key 请求不再合成 instructions |
| `6d5f02784` | Applied + Overridden | 接入 passthrough WS 会话隔离、超大 bridge、空闲连接回收、容量错误和终态 close 修复；本地继续每轮重取请求模型/渠道映射、记录 payload hash、独立结算 usage、释放并发并在失败停调度后阻断后续轮 |
| `cc6a8e517` | Applied | 更新赞助商资源 |
| `e2624fb65` | Applied | 保留 Codex 已知图片输入能力 |
| `200602b41` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `530fb20f2` | Applied + Overridden | 接入账号成本定价、长上下文目录数据驱动、覆盖文件和 cache 字段哨兵；保留分组/渠道显式价优先、Gemini 200K 边际价、GPT-5.6 272K 与 Grok 200K 严格边界及 GPT-5.4 Pro 无目录基础价 |
| `593fc9365` | Applied + Overridden | 接入账号成本定价、长上下文目录数据驱动、覆盖文件和 cache 字段哨兵；保留分组/渠道显式价优先、Gemini 200K 边际价、GPT-5.6 272K 与 Grok 200K 严格边界及 GPT-5.4 Pro 无目录基础价 |
| `e2cfaa46e` | Applied + Overridden | 接入账号成本定价、长上下文目录数据驱动、覆盖文件和 cache 字段哨兵；保留分组/渠道显式价优先、Gemini 200K 边际价、GPT-5.6 272K 与 Grok 200K 严格边界及 GPT-5.4 Pro 无目录基础价 |
| `aa679efe4` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `1cb28fd42` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `563979e91` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `95982a508` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `94bb9354c` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `e9e66da5a` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `7e0681657` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `7f5e915dd` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `9fba8ed43` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `a44a1019f` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `f8eae4504` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `7903716ad` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `1a40e5690` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `3fbce499d` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `17747df84` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `0681aa256` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `c7cf84ad3` | Applied + Overridden | 接入分组 Fast 的迁移、Ent、管理 API、认证快照、前端和网关全链路；与本地严格请求模型计费、服务层级只降不升及 Free Fast 双成本规则共同适配 |
| `eebf4e3ff` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `e7caedc22` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `fb409787c` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `7dcf72846` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `636d6be69` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `08d6c153b` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `fd55f3248` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `8224434d2` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `e619ca386` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `e6722126b` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `498e06c58` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `4df6b0636` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `9908d3ca2` | Applied + Overridden | 接入 Free Fast 的迁移、持久化、管理 API、认证快照、前端和计费；用户 `ActualCost` 按 Standard，账号 `TotalCost` 与 usage `service_tier` 保持 priority |
| `bdedd6c54` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `e87c47c95` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `064d9b74e` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `2d7767eaf` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `9d9ed1cc6` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `1cce2b38e` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `f218c8d40` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `6b39d0b45` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `2ac784c51` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `aa7a811e6` | Applied + Overridden | 接入按模型 exact/prefix/suffix 限制 reasoning effort 和超限 deny/downgrade；本地保持 exact 优先、较长 affix 次之、全局回退及默认降档边界 |
| `a2fb09260` | Applied + Overridden | 完整保留上游版本提交的祖先关系，最终不采用上游 `0.1.184`、`0.1.185` 或 `0.2.0`，继续使用本地 `0.1.228` |
| `421a83282` | Applied | 允许无 call id 的 scheduled automation bootstrap |
| `200b1406d` | Applied | 仅在声明 server-side-fallback beta 时保留 Anthropic fallbacks |
| `e93e6368f` | Applied + Overridden | 接入 OpenAI 重新授权、额度冷却原子清理、目录禁用账号过滤、passthrough 调度快照及模型冷却修复；继续保留本地失败代次、连续失败停调度、自动测活和账号默认策略 |
| `343858021` | Applied + Overridden | 接入 OpenAI 重新授权、额度冷却原子清理、目录禁用账号过滤、passthrough 调度快照及模型冷却修复；继续保留本地失败代次、连续失败停调度、自动测活和账号默认策略 |
| `e377c4358` | Applied + Overridden | 接入 Kimi 原生 Responses；PayG/Coding 使用各自 `/v1/responses`，显式 `responses`/`adaptive` 强制 `store=false` 并移除 `previous_response_id`，账号测试验证同一契约 |
| `cd04848b9` | Applied | 更新赞助商资源 |
| `d596d0844` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `65380be9c` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `0d27f45ea` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `504919a05` | Applied + Overridden | 接入 passthrough WS 会话隔离、超大 bridge、空闲连接回收、容量错误和终态 close 修复；本地继续每轮重取请求模型/渠道映射、记录 payload hash、独立结算 usage、释放并发并在失败停调度后阻断后续轮 |
| `bfe0a5a87` | Applied + Overridden | 接入 passthrough WS 会话隔离、超大 bridge、空闲连接回收、容量错误和终态 close 修复；本地继续每轮重取请求模型/渠道映射、记录 payload hash、独立结算 usage、释放并发并在失败停调度后阻断后续轮 |
| `9e2d97f25` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `52c7d8834` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `34b8bf1a6` | Applied | 支持 Claude Fable 5.1 |
| `2786ae9de` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `8d0b5ede2` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `a668aa8b3` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `a4fb58e42` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `559960865` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `f2804eb2c` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `e50bffb7e` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `6566039bc` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `05ea883e2` | Applied | 修复合并后的 Group Ent 字段索引 |
| `77729e272` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `3510aa22b` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `1a33dc8cc` | Applied | 优化分组模型定价弹窗布局 |
| `aa2364883` | Applied | 依赖合并；完整 merge 保留祖先关系，具体功能与处置由关联的 non-merge 提交说明 |
| `5097b3145` | Applied + Overridden | 完整保留上游版本提交的祖先关系，最终不采用上游 `0.1.184`、`0.1.185` 或 `0.2.0`，继续使用本地 `0.1.228` |

### `Applied + Overridden` 覆盖边界

- 原生 compaction：`1cc6999ad`、`1a61eb715`。接入请求类型和契约，继续按真实端点、请求模型及 WS 轮次独立归因和结算。
- 服务层级：`624e4eef6`、`8b4b3f4a9`、`2b8cb628b`、`d39fc491e`、`3a9070359`、`f323d8464`、`50ad6e2e5`、`82105f260`、`57c76584a`。接入出站/观测层级分离及 Codex priority 能力；响应层级只允许降低计费档位，OAuth-like `default` 不具备上调权威性，明确 `flex` 仍可降档。
- 价格目录与账号成本：`9eabd2a5b`、`e7c029875`、`530fb20f2`、`593fc9365`、`e2cfaa46e`。接入目录阶梯、覆盖补丁、缓存字段哨兵和账号成本计价；分组/渠道显式价格优先，Gemini 原生 `/v1beta` 使用 200K 边际规则，GPT-5.6/Grok 使用严格 `>` 边界，GPT-5.4 Pro 无目录时不合成阶梯。
- 分组 Fast：`aa679efe4`、`1cb28fd42`、`563979e91`、`95982a508`、`94bb9354c`、`e9e66da5a`、`7e0681657`、`7f5e915dd`、`9fba8ed43`、`a44a1019f`、`f8eae4504`、`7903716ad`、`1a40e5690`、`3fbce499d`、`17747df84`、`0681aa256`、`c7cf84ad3`。完整接入迁移、持久化、管理 API、认证快照、前端和网关路径；继续与本地严格请求模型计费及 Free Fast 双成本逻辑组合。
- Free Fast：`eebf4e3ff`、`e7caedc22`、`fb409787c`、`7dcf72846`、`636d6be69`、`08d6c153b`、`fd55f3248`、`8224434d2`、`e619ca386`、`e6722126b`、`498e06c58`、`4df6b0636`、`9908d3ca2`。上游请求和 usage 保持 priority，用户 `ActualCost` 按 Standard，账号 `TotalCost` 按 priority；只对 OpenAI 账号且 OpenAI/Composite 分组生效。
- 版本：`52374af94`、`a2fb09260`、`5097b3145`。保留祖先关系但不回退版本，最终继续使用本地 `0.1.228`。
- Reasoning effort：`7c01ec9be`、`aa7a811e6`。支持 exact/prefix/suffix 模型范围，exact 优先、较长 affix 次之、最后回退全局；超限默认降档，也可配置拒绝。
- Kimi 原生 Responses：`e377c4358`。PayG/Coding 使用对应原生端点，强制 `store=false` 并移除 `previous_response_id`；账号测试验证同一请求契约。
- WebSocket 与缓存身份：`7c616db07`、`d5a012463`、`6d5f02784`、`1dc0a0900`、`bfe0a5a87`、`504919a05`。每轮重新取得请求模型和渠道映射、记录 payload hash、独立结算 usage、释放并发并保留真实计价时刻；某轮失败触发停调度后阻断后续轮，只有安全重建上下文时允许换号。
- TTFT：`c66e700f0`、`8553c91c1`、`0fcec63b6`。接入管理设置和可见性契约，保留本地监控超时、访问范围与费用展示边界。
- 账号状态与调度：`92a550973`、`897faea33`、`ba345f105`、`e93e6368f`、`343858021`。接入重新授权、额度冷却、目录能力和调度快照修复；继续保留失败代次、连续失败停调度、自动测活和账号默认策略。
- 前端稳定性：`0aef702b6`、`6532d5b61`。接入用量窗口和缓存提示修复，继续保留本地表格、浮层、分页与滚动位置行为。

### 本地提交与文件

- 上游固定范围整体映射到 merge commit `5be874e91ef37c35065ae42e92956d49cee32dea`；双亲为同步前本地 SHA `499114a87384de74b3db987299f29c57217a2ae2` 和固定上游 SHA `5097b31457e6dc9f49e5f5c9c72b925ce79543b3`。
- 二次开发适配提交为 `736761ba3b55ed393a9efa94e3ebd89b4f5f5c6f`，修改 8 个文件，增加 290 行、删除 64 行；主要实现 Gemini 边际计价、GPT-5.6/Grok 严格边界、服务层级优先级及对应回归测试，并修正 Ent 生成注释。
- CI 稳定性修复提交为 `35c28e324b17d9e4dcef877a230d803fc6815249`：首轮 PR integration 在上海时间 23:58 启动，使测试夹具的 `now.Add(2*time.Minute)` 跨越自然日，站点 `TodayTokens` 按下一日正确归零但用例仍断言 150；将夹具固定到上海时区正午，避免测试运行时刻改变业务日期。
- 写入本记录前，代码与适配相对 `LOCAL_PRE_SYNC_SHA` 共变更 301 个文件：新增 28 个、修改 273 个、删除 0 个，增加 11433 行、删除 1631 行；本记录另修改两份历史台账。
- 当前代码/测试基线相对固定上游目标变更 919 个文件，增加 130491 行、删除 5976 行；本地独有提交 512 个，当前能力族仍为 53 项、9 个功能域。
- 主要新增：分组 Fast/Free Fast、reasoning effort 模型范围与超限策略、Kimi 原生 Responses、价格目录覆盖文件、原生 compaction 用量、TTFT 管理设置，以及迁移 231、232、233。
- 主要修复：OpenAI 服务层级观测/计费分离、Codex priority 广告、WS passthrough 会话隔离和终态识别、数据库启动重试、账号到期本地时区、渠道监控用户范围、Grok 视觉工具图片输出和分组定价弹窗布局。
- `backend/cmd/server/VERSION` 最终保持 `0.1.228`；`deploy/docker-compose.yml` 与 `deploy/docker-compose.sub2api.yml` 字节完全一致，SHA-256 均为 `608C0978CA699089D9BFB13B56DE00FAADC97E65C7EFC085B073AE7649EAEBE6`。

### 文本冲突与最终解决方案

- 17 个文本冲突为：`backend/cmd/server/VERSION`、`backend/ent/group.go`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/service/account_test_service_cn_adaptive.go`、`backend/internal/service/account_test_service_cn_adaptive_test.go`、`backend/internal/service/api_key_auth_cache_impl.go`、`backend/internal/service/api_key_auth_cache_profit_test.go`、`backend/internal/service/billing_service.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/openai_fast_policy_test.go`、`backend/internal/service/openai_gateway_usage.go`、`backend/internal/service/pricing_service.go`、`backend/internal/service/setting_gateway_runtime.go`、`backend/internal/service/upstream_response_model.go`、`deploy/docker-compose.sub2api.yml`、`frontend/src/views/user/__tests__/UsageView.spec.ts`。
- 版本冲突保留本地 `0.1.228`；未采用上游 `0.2.0`。
- 定价冲突接入上游目录数据驱动、覆盖文件、服务层级和 Free Fast，同时保留 Gemini 200K 边际规则、GPT-5.6 272K 与 Grok 200K 严格边界、GPT-5.4 Pro 无目录基础价、严格请求模型计费及分组/渠道显式定价优先级。
- Kimi 冲突接入原生 Responses，并保留 PayG/Coding 端点、自定义请求头、`store=false`、移除 `previous_response_id` 及账号测试覆盖。
- WS 和 usage 冲突接入上游会话隔离、终态与服务层级处理；继续保留逐轮模型映射、payload hash、独立结算、并发释放、真实计价时刻、失败停调度阻断及安全换号边界。
- Compose 冲突接入上游健康检查变化，继续保留 bind mount、仅回环暴露、HTTP upstream 业务开关和生产资源变量入口；两份生产 Compose 最终字节一致。
- 所有冲突均逐文件解决，未整文件采用 `ours`、`theirs` 或上游版本；最终索引无未解决路径。

### 刻意保留的二次开发功能

- `CUST-GW-001`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`：首语义输出边界、失败代次和停调度、OAuth 插件已发出请求不可重放、WS 逐轮结算/并发释放/失败阻断及请求体安全释放。
- `CUST-PROTO-001`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-ACC-005`：Codex 稳定身份与服务层级、API Key 请求头覆写、Kimi 原生 Responses、compact/WS 路由和账号测试契约。
- `CUST-ACC-001`、`CUST-ACC-006`：账号默认策略、创建预览、分组 Fast/Free Fast 认证快照及本地数据迁移兼容。
- `CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-BILL-006`：严格请求模型计费、Free Fast 双成本、服务层级只降不升、长上下文边际/阶梯边界、费用展示、按用户串行扣费和 5 秒 usage task 超时。
- `CUST-OBS-001`、`CUST-UI-004`：渠道监控超时/访问范围、TTFT 可见口径，以及表格、浮层、分页、滚动和时区稳定性。
- `CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005`：生产 bind mount、回环端口、HTTP upstream 开关、双生产 Compose 一致性、4 vCPU/8 GiB 资源参数及远端 CI 门禁。

### 验证记录

本机实际使用 Go 1.27.0、Node 24.15.0、pnpm 11.19.0；本地 golangci-lint 2.9.0 由 Go 1.26.3 构建，无法检查目标 Go 1.27 项目。未读取 `.env` 或任何业务秘密。

| 阶段 | 命令或检查 | 退出码 | 结果 |
| --- | --- | ---: | --- |
| 同步后后端 | `go test ./... -count=1` | 0 | 默认标签全包测试通过 |
| 同步后后端 | `go test -tags=unit ./... -count=1` | 0 | unit 标签全包测试通过 |
| 同步后后端 | `go build ./...` | 0 | 全包构建通过 |
| Wire | `go generate ./cmd/server` | 0 | 连续两轮成功，生成文件无差异 |
| Ent | 隔离 target 执行 `go generate ./ent` 并比较 320 个文件 | 0 | 仅 `backend/ent/group.go` 存在真实生成注释差异并已提交；临时 target 导入路径排序差异不写入正式目录，隔离目录已删除 |
| integration 仅编译 | 仓库内临时 no-op `-exec` 包装器执行 integration 标签全包 | 0 | 仅完成编译，未执行 TestMain、Docker 或测试用例；包装器已删除 |
| 同步后前端 | `pnpm run lint:check`、`pnpm run typecheck`、`pnpm run test:run`、`pnpm run build` | 0 | lint/typecheck/生产构建通过；271 个测试文件、1962 项 Vitest 通过 |
| 同步后 Canvas | 直接调用本地 Node 模块执行 format、`tsc --noEmit`、Vitest、Vite build | 0 | 格式、类型检查和生产构建通过；8 个测试文件、34 项 Vitest 通过 |
| 部署静态 | `docker-compose-security-test.sh`、`docker-compose-gateway-env-test.sh`、`docker-runtime-resources-test.sh`、`test-caddyfile-cache.sh` | 0 | 全部通过；未读取真实配置或连接外部系统 |
| Apple fixture 语法 | shell 语法检查 | 0 | 脚本语法通过 |
| Apple fixture 执行 | Apple 生命周期 fixture | 1 | Windows Git Bash 不支持 macOS BSD `stat -f '%Lp'`，属于平台限制 |
| golangci-lint | `golangci-lint run ./... --timeout=30m` | 3 | golangci-lint 2.9.0 由 Go 1.26.3 构建，拒绝检查目标 Go 1.27 项目；未将其写为通过 |
| 远端首轮 CI | push CI `33650959674`、PR CI `33651146176` | 0 / 2 | push 触发的完整检查通过；PR 重复运行仅 `TestUpstreamRepositoryCommitSyncIdempotentAndCascadeDelete` 失败，原因是上海时区自然日边界导致测试夹具跨日，非生产逻辑回归 |
| CI 稳定性修复 | `go test ./internal/repository -run '^TestUpstreamRepositoryCommitSyncIdempotentAndCascadeDelete$' -count=100` | 0 | 固定上海时区正午后连续 100 次通过 |
| Compose 一致性 | 字节比较及 SHA-256 | 0 | 两份生产 Compose 字节一致，哈希均为 `608C0978CA699089D9BFB13B56DE00FAADC97E65C7EFC085B073AE7649EAEBE6` |
| 最终静态复核 | `git diff --check`、标准冲突标记、意外删除、未跟踪文件、凭据类路径候选、版本、祖先关系和提交处置计数 | 0 | 无空白错误、未解决冲突、删除文件、未跟踪文件或敏感路径候选；版本为 `0.1.228`，固定上游目标已是当前分支祖先，137 个 SHA 均且仅出现一次 |

### 未验证项与残余风险

- 本机 Docker 不可用，未运行真实 integration、Testcontainers、Compose 启动、PostgreSQL/Redis、迁移 231/232/233 或 `/health` 检查；本地 integration 仅完成标签全包编译。远端首轮 push integration 通过，PR 重复运行暴露跨日测试夹具并已修复，后续 CI 继续作为合入门禁。
- 未运行 `-race`、`govulncheck` 或可用版本的 golangci-lint；发布前仍需由远端 CI 的 Go 1.27、Node 20、golangci-lint 2.13、Docker integration 和 Security Scan 作为最终门禁。
- Ent 在 Windows 活动目录直接生成两次均因 user-mapped section 锁定失败；通过仓库内隔离 target 完整生成并逐文件比较，损坏的中间文件已恢复，未留下临时目录。
- 未使用真实 OpenAI、Anthropic、Kimi、Grok、Gemini、CN Provider 凭据或 OAuth 插件包；真实额度消费、服务层级响应、媒体、流式故障转移和 Kimi 原生端点端到端未验证。
- 未执行真实浏览器端到端交互；前端与 Canvas 由组件测试、类型检查和生产构建覆盖。
- 未读取 `.env`、GitHub Token、密码、Cookie 或 SSH 私钥；未执行 push、PR、部署、远程服务器访问、容器重启、生产挂载核验或生产数据操作。
- 生产服务器状态、bind mount 实际挂载、实例性能参数和健康检查均未验证；本次仅验证仓库内静态 Compose 约束。

## 2026-09-05 同步至 ab99d56e9

### 固定范围与批准边界

- 开始时间：2026-09-05T19:02:49+08:00；状态：本地完整合并，验证及代码提交在本节末尾记录。
- 用户已批准集中方案，只授权本地备份、合并、兼容调整、测试、提交和最终 fast-forward；未授权 push、PR、部署或服务器访问，本次均未执行。
- 本地目标分支：`main`；`LOCAL_PRE_SYNC_SHA=6fed6e108dd7dcdcda2d03251e95a4dedd7aae9b`。
- 上游：`https://github.com/Wei-Shaw/sub2api`；默认分支通过远端 HEAD 核验为 `main`，不是猜测。
- `UPSTREAM_OLD_SHA=5097b31457e6dc9f49e5f5c9c72b925ce79543b3`。
- `UPSTREAM_NEW_SHA=ab99d56e9626e6cd731592dae8553c9758a0efa2`。fetch 后已固定，本次不继续追踪新增上游提交。
- `ACTUAL_MERGE_BASE=5097b31457e6dc9f49e5f5c9c72b925ce79543b3`。旧基线既是本地祖先也是目标上游祖先，实际 merge 范围与审批范围一致。
- `LAST_FULLY_INTEGRATED_UPSTREAM_SHA=ab99d56e9626e6cd731592dae8553c9758a0efa2`。
- 预检：目标分支工作区干净，与已核验的跟踪分支无落后/分叉，stash 0，无活动 submodule/LFS；历史 `REBASE_HEAD` 是无活动 rebase 目录的旧遗留，未擅自清理。
- 范围：82 个上游提交（48 非 merge、34 merge）；上游差异297文件，其中138与本地定制重叠；无完整 patch-id 等价提交。
- 集成策略：`git merge --no-ff --no-commit ab99d56e9626e6cd731592dae8553c9758a0efa2`，保留完整祖先关系，不重组上游历史。
- 备份：`backup/pre-upstream-sync-20260905-190009-6fed6e108`；同步：`sync/upstream-20260905-ab99d56e9`。
- 本地版本继续 `0.1.228`；上游 `0.2.1` 版本提交记作覆盖，不记作跳过。

### 上游提交逐项处置

82 项全部进入本地祖先历史：`Applied` 43 项，`Applied + Overridden` 39 项，`Already Applied / Skipped / Deferred` 均0，未解决 `Conflict` 0。`Applied + Overridden` 表示纳入提交历史、但按批准方案保留本地行为；不是选择性跳过。合并项的覆盖边界引用其功能父提交，不能据此认为 merge 一律为空。

| 上游完整SHA | 处置 | 内容与原因 |
| --- | --- | --- |
| `ca9b4d73f2bc680c243ee7a6da6667bd6e83a710` | Applied + Overridden | 共享不可变 WS 回放体：保留首语义输出前驻留、逐轮 payload hash 和独立结算。 |
| `0e08c399994fc1934bdae20123a044b8dd0ed995` | Applied + Overridden | 跨轮记录被拒绝的加密内容：合并清理与重建；不越过本地安全续聊和插件已发送禁止重放边界。 |
| `9ad386569051b60fae616932eca876a7e4f78538` | Applied + Overridden | 负载感知调度补齐渠道限制：旧 upstream 来源仍归一为 requested；原请求模型缺价在负载、粘性、路由选择前拒绝，新增测试适配该本地约束。 |
| `9be9c0b68c47eb433d886453deecf73c7ed55d68` | Applied + Overridden | 转发失败及时释放会话槽：接入释放修复，同时保留每轮账号槽位和失败停调度保护。 |
| `8249ab37dbc20eff0abfebac58d63dd9a7fd62a7` | Applied + Overridden | Anthropic 推理等级及 max 定价：接受 Fable 5.1 max 默认3倍与渠道覆盖；使用实际转发等级，保留严格请求模型、渠道优先和本地长上下文规则。 |
| `10c8f523c49663d6061eabc1cdce13a2a975063f` | Applied | 保留透传推理强度：接入透传修复；保持请求强度与实际转发强度分别记录。 |
| `b6740c937f007a0e103d182bc6197c31e3c3a14f` | Applied + Overridden | 恢复不可用的 continuation：接入安全恢复；不放宽本地首输出、插件已发送或上下文不可重建时的禁止重放。 |
| `fdc2ec17e6776bef109c5ea614af28218e56299b` | Applied + Overridden | 价格文件按内容哈希热重载：接入 fallback/override 变更检测，同时保留本地动态价优先和严格已知模型匹配。 |
| `5e4958c889e0e8a31591a819eb046562807d8beb` | Applied + Overridden | 使用映射模型调度 OpenAI 账号：映射模型只服务账号选择，用户费用仍按请求模型，WS 不恢复旧计费模型回退函数。 |
| `af90a9bd1fbcee62c5e8a87dbb3bd71ee4216f27` | Applied + Overridden | 兼容历史任务委派续聊：接入历史上下文修复，保留本地逐轮校验、结算和不可重放边界。 |
| `62198286e93a6ecf3c157417c80d136b68bd84c4` | Applied | 区分网关准入拒绝：保留本地结果归因并纳入并发准入来源。 |
| `e9e3c46cb76f564554357507102caf44cc79857d` | Applied | 记录上游错误发生时的代理快照：接入代理ID/名称归因，不记录认证头。 |
| `4c1f920d58d6f8b0e2421a42b76f2661fafe4a61` | Applied | 补齐代理归因并限制排队事件体积：接入队列上限与上下文清理；已有统一错误 helper 的代理字段不重复添加。 |
| `78fc7b171ec1a802a8d267d0dcc159539ddb0094` | Applied | 网络不可用时跳过可选真实 Token 对比：确定性测试照常执行；本次不提供真实凭据或进行真实接口比较。 |
| `abc07bb0799c7c8cc15795898d4acec247c7a59c` | Applied | 代理归因测试的 errcheck 和格式修复：完整纳入静态检查修正。 |
| `432ebc498cb395696d0be9762a33f70a63bf9375` | Applied | 合并#6179：保留完整父子历史；功能处置及覆盖边界同 `abc07bb0799c7c8cc15795898d4acec247c7a59c`。 |
| `b1748c4ea99ce2120401a269142aa071e18a84da` | Applied | 更新赞助内容：只变更三份 README 赞助项，删除对应两张赞助图片。 |
| `de8d756afae96f74d46c08f7633eb7e186c25176` | Applied + Overridden | 补查待支付支付宝订单：扩展通用补查，同时保留 EasyPay 独立30秒门槛、每轮10条/10秒预算及幂等审计；按提供商和支付类型划分查询集合。 |
| `28dde982cb5526d873204a50289928357c10f64f` | Applied | 规范心跳初始化上下文：完整接入请求规范化和确定性回归；不建立任何实际自动化。 |
| `de27905e88aa291acd2a292e2b439bce3de0f118` | Applied + Overridden | 按账号配置上游请求 ID 响应头并落库：默认关闭；新增写入与读取双向敏感头拒绝，保留WS无响应头时空值及本地列设置浮层。 |
| `3e60c3b86b4849bdd283f92cb88b5a7c1a79d456` | Applied | 增加轻量管理员账号 DTO：接入轻量清单与详情回取，保持原账号功能字段可在编辑时取得。 |
| `994192ef4f22baf546c032f557bfb6d14b1dcaf4` | Applied | 补全管理员账号列表类型：纳入前端类型和接口契约验证。 |
| `db15e0090aace48e62f952c6a294f38048cf6288` | Applied + Overridden | 账号表格保持使用轻量列表：保留本地禁用虚拟化、分页、滚动和编辑详情回取。 |
| `a3a6a85b73887e8f7961ae25c828d64854f25b9f` | Applied | 测试轻量列表嵌套凭据脱敏：完整保留嵌套脱敏断言。 |
| `f5ae32e53f6d1b42f4da6c78ff06fbd3d870f156` | Applied | 规范 GLM-5.3 thinking effort：完整接入模型专属推理参数转换。 |
| `3c8be00131e69d432246855d14ff014d9f7c10e2` | Applied + Overridden | 支持 GPT-6 Astra：精确增加 Astra/gpt-6 及已知兼容别名；未知请求别名仍须显式价格，保留本地费用与映射模型分离。 |
| `126ac24c8cb4d98ff2a02f46f0008201cd419c10` | Applied + Overridden | 支持 ultrafast 服务层级：接入能力、UI 和标准化；保留本地 Free Fast 用户/账号双成本语义。 |
| `aa556c372f85f0f5d41398d80d5972e53f708c16` | Applied | 活动子路由下允许折叠侧栏：完整接入并保留本地滚动、列设置浮层。 |
| `6cf645f87cbe3b616fd4a977e472ee4253d8bc85` | Applied | 限定 OpenCode 会话头转发范围：仅匹配合法 OpenCode 目标；测试适配本地扩展签名及非流式无首 Token 守卫契约。 |
| `c7538cd256c2f60ed909402213ef869acef2f8b2` | Applied | 为桥接路由广告搜索能力：完整接入搜索能力发现，原工具策略继续生效。 |
| `51631d032474fe8001d0711b067ba9ac8e65a78f` | Applied | Chat fallback 保留发现的工具：完整接入发现工具的转换及回归断言。 |
| `9ddf69698d41af3b05e4fe4560f5d9d41b81ebbe` | Applied | 保留零值系统指标：零值不再被误当作缺失。 |
| `28f673e4cde06ed68fc56c1718b6ac6776bef311` | Applied | Gemini 尊重自定义原生模型清单：接入清单选择，不改本地 Gemini 计费与流式适配。 |
| `1e28ef59423f121761fd0b7bb95e2fac03ebe823` | Applied + Overridden | OpenAI 分组固定账号 Codex 清单：默认关闭且仅用于只读清单；认证快照升v23并保留本地长上下文/Free Fast字段，不替代推理调度门禁。 |
| `e9bad40b10a8fc8879f65ac38a31beeaa5a64523` | Applied | 可配置 Claude CLI 广告版本：接入可选版本覆盖，未设置时沿用既有默认。 |
| `4aaf5cb73bfe87e6dc8502c7bab4023229d43c5e` | Applied | Astra Messages 启用提示缓存：接入模型能力判定与缓存键。 |
| `7969751f1f0aaa262d7c0a2550d0f12c0fed8a35` | Applied | 推理映射支持 none 来源：完整接入前后端映射字段与范围校验。 |
| `983a3db3afc0a7f08b51e4e3091b0c75801b6a6f` | Applied | 同步并保存 Astra Codex 能力：接入能力持久化；与本地创建预览、正式同步共存。 |
| `a016bd4ba5dd1920c11d0c853f56c3f2f3c94e11` | Applied | 能力同步测试格式修复：完整纳入测试格式修正。 |
| `d8b9a83f8cda66a821b3dcadf8ecbdd2c6c65b90` | Applied + Overridden | 合并#6492：保留完整父子历史；功能处置及覆盖边界同 `9ad386569051b60fae616932eca876a7e4f78538`。 |
| `1cab4d8c22bb79a82970a467a2492e5ff0a763d9` | Applied + Overridden | 保证价格热重载快照一致：合并锁定与哈希一致性，保留本地覆盖和 nil guard。 |
| `708b85a6a4b853f6db3c488c0c84701edf1e7929` | Applied + Overridden | 补齐上游请求 ID 传递：接入全部HTTP结果字段，并延续敏感头禁录与本地请求归因。 |
| `78e1aaedb983a3e30d250e15b1093be3d3ca8018` | Applied + Overridden | 合并#6535：保留完整父子历史；功能处置及覆盖边界同 `1cab4d8c22bb79a82970a467a2492e5ff0a763d9`。 |
| `91cf660377af627c1e48a2f20776982ae6dc3df1` | Applied + Overridden | 合并#6397：保留完整父子历史；功能处置及覆盖边界同 `0e08c399994fc1934bdae20123a044b8dd0ed995`。 |
| `701f844755aa4b9dd1d79ae621820a9b3dcf469d` | Applied + Overridden | 合并#6555：保留完整父子历史；功能处置及覆盖边界同 `708b85a6a4b853f6db3c488c0c84701edf1e7929`。 |
| `c4e6dcfd80abe36e98718c95b98f17e4ba2737df` | Applied + Overridden | 合并#6602：保留完整父子历史；功能处置及覆盖边界同 `1e28ef59423f121761fd0b7bb95e2fac03ebe823`。 |
| `15278fe2f9f145eb1e45ad136901c0da83cd0be0` | Applied | 合并#6580：保留完整父子历史；功能处置及覆盖边界同 `aa556c372f85f0f5d41398d80d5972e53f708c16`。 |
| `570bd084c23429077dec0bede49157f8e4191ea7` | Applied | 合并#6594：保留完整父子历史；功能处置及覆盖边界同 `9ddf69698d41af3b05e4fe4560f5d9d41b81ebbe`。 |
| `2ad6e8fb75dfd2ad75744da1cac7827a96a0fffc` | Applied | 合并#6599：保留完整父子历史；功能处置及覆盖边界同 `28f673e4cde06ed68fc56c1718b6ac6776bef311`。 |
| `eba6ea563991d62d6742d5c1ae1ae8514b708e35` | Applied | 合并#6590：保留完整父子历史；功能处置及覆盖边界同 `c7538cd256c2f60ed909402213ef869acef2f8b2`。 |
| `d7fb9b5d35bf3f19cf68df9c329974ce99f7932a` | Applied | 合并#6593：保留完整父子历史；功能处置及覆盖边界同 `51631d032474fe8001d0711b067ba9ac8e65a78f`。 |
| `85f431a072b391e704df6cc3a12f7db46b62d1ef` | Applied | 合并#6620：保留完整父子历史；功能处置及覆盖边界同 `4aaf5cb73bfe87e6dc8502c7bab4023229d43c5e`。 |
| `8f1d6af3ed1aae36d2b95cfdbb827ba535787562` | Applied + Overridden | 合并#6531：保留完整父子历史；功能处置及覆盖边界同 `b6740c937f007a0e103d182bc6197c31e3c3a14f`。 |
| `63dc24b5e8a736279d60c8270b763c3e733c9688` | Applied | 合并#6626：保留完整父子历史；功能处置及覆盖边界同 `7969751f1f0aaa262d7c0a2550d0f12c0fed8a35`。 |
| `07bf8b92fda067516b7989412f09b75bb39bc113` | Applied + Overridden | 合并#6550：保留完整父子历史；功能处置及覆盖边界同 `de8d756afae96f74d46c08f7633eb7e186c25176`。 |
| `3c53ba01a7747d7f40302bd15e1d92c908fddbad` | Applied + Overridden | 图片 URL 转 base64 可选回填：开关默认关闭；本地强制隐藏真实图片URL、生成 data URL/b64 的策略仍优先，关闭可选回填不关闭既有内联转换。 |
| `2da31290a1bcbf68fbf49c8f13b4c3ddb3d29d1e` | Applied + Overridden | 各 WS 路径记录 Cyber 失败：合并跨尝试去重；本地失败终态继续返回错误及部分usage，保留风险门禁和原子逐轮阻断。 |
| `3a8c9f16de2d06ad425daeddc9b6c405752f7631` | Applied + Overridden | 合并Astra分支与上游：保留完整父子历史；功能处置及覆盖边界同 `07bf8b92fda067516b7989412f09b75bb39bc113`。 |
| `c227863d51468bee3a48f999515bd233bc2d41cf` | Applied + Overridden | 图片下载公网防护和内容嗅探：原回填器保留公网/字节嗅探；本地强制内联下载也增加公网、重定向与超时防护，不修改业务HTTP/私网 upstream开关。 |
| `68f099707e2eb9d7c4e9e7e8176c15a588209345` | Applied | 合并#6553：保留完整父子历史；功能处置及覆盖边界同 `28dde982cb5526d873204a50289928357c10f64f`。 |
| `7b271bbee985e8d62498947811e5886fd93cbc92` | Applied | 合并#6529：保留完整父子历史；功能处置及覆盖边界同 `10c8f523c49663d6061eabc1cdce13a2a975063f`。 |
| `c6ff69fd12b4d684350e5834c4ab859d0a0afa99` | Applied | 合并#6542：保留完整父子历史；功能处置及覆盖边界同 `62198286e93a6ecf3c157417c80d136b68bd84c4`。 |
| `620eb3fd006918772882dbe57e62702a2cdfd663` | Applied | 合并#6581：保留完整父子历史；功能处置及覆盖边界同 `6cf645f87cbe3b616fd4a977e472ee4253d8bc85`。 |
| `40f4d450beda49d2cedee432a51b26f7bfbc4b07` | Applied | 合并#6507：保留完整父子历史；功能处置及覆盖边界同 `f5ae32e53f6d1b42f4da6c78ff06fbd3d870f156`。 |
| `9517f12cdd34b42c499e96d0a97aef668d14edcc` | Applied + Overridden | 合并#6510：保留完整父子历史；功能处置及覆盖边界同 `9be9c0b68c47eb433d886453deecf73c7ed55d68`。 |
| `9b4fcfb897455eebc17812a0af2d365c0c86fc3f` | Applied | 合并#6606：保留完整父子历史；功能处置及覆盖边界同 `e9bad40b10a8fc8879f65ac38a31beeaa5a64523`。 |
| `0d9e7e1530855c93d4d00fc87406960d4a7fefd5` | Applied | 合并#6628：保留完整父子历史；功能处置及覆盖边界同 `a016bd4ba5dd1920c11d0c853f56c3f2f3c94e11`。 |
| `d7f048a2656612059ae764dcf48f1571ff592e73` | Applied + Overridden | 保留已同步 Astra 能力及续聊：接入能力合并并保留严格模型规范化和安全续聊边界。 |
| `cc52c93ad7d60c5364c66bbf8c2efacb253a29d7` | Applied + Overridden | 合并#6539：保留完整父子历史；功能处置及覆盖边界同 `af90a9bd1fbcee62c5e8a87dbb3bd71ee4216f27`。 |
| `aafe93b335763e0586f4f30665751c279dc78c73` | Applied | 合并#6557：保留完整父子历史；功能处置及覆盖边界同 `a3a6a85b73887e8f7961ae25c828d64854f25b9f`。 |
| `83094abf21c5f5752c3e770bc8c70704737a35ec` | Applied + Overridden | 合并#6536：保留完整父子历史；功能处置及覆盖边界同 `78fc7b171ec1a802a8d267d0dcc159539ddb0094`。 |
| `2dc287f2e04210209bebe865ebe6f8b42f81f03c` | Applied + Overridden | 合并#6636：保留完整父子历史；功能处置及覆盖边界同 `2da31290a1bcbf68fbf49c8f13b4c3ddb3d29d1e`。 |
| `c42d78e26c42153dac7641d51ddd6fb0977b0b14` | Applied + Overridden | 合并#6571：保留完整父子历史；功能处置及覆盖边界同 `126ac24c8cb4d98ff2a02f46f0008201cd419c10`。 |
| `fb44e2dd091970dc899f59cbc568bed961035d3f` | Applied + Overridden | 合并#6514：保留完整父子历史；功能处置及覆盖边界同 `8249ab37dbc20eff0abfebac58d63dd9a7fd62a7`。 |
| `dee35a841deb8853d67a914c56b50de48fd7143b` | Applied + Overridden | 合并并调和Astra能力：该merge含额外16文件调整，已纳入能力、缓存与续聊审查，不当作空合并。 |
| `62cd63bfdab33d4bff289be8fe5bfc98eda23294` | Applied | 模型列表不可用时保留已有能力：完整纳入已同步能力保留及部分成功提示。 |
| `d2f87c612e1820f6d4900a71fda1248c5b3544ed` | Applied + Overridden | 合并#6638：保留完整父子历史；功能处置及覆盖边界同 `c227863d51468bee3a48f999515bd233bc2d41cf`。 |
| `994ca26e9270ef5bb631fc6b2dd5f4bfbd5f9e06` | Applied + Overridden | Claude billing 指纹与最终 UA 一致：强制模拟后的最终出站 UA 决定 billing 版本，保留本地 Unicode 字符索引覆盖。 |
| `2e1c7c00a686fbc446f94634522e31724ac23dc5` | Applied | 校验 Claude 测试请求体关闭错误：保留错误检查及本地新增断言。 |
| `ed7c8f2208f478907782d2ad646463941d455379` | Applied | 合并#6572：保留完整父子历史；功能处置及覆盖边界同 `62cd63bfdab33d4bff289be8fe5bfc98eda23294`。 |
| `578785ee7fb35030b094b69624efe25670a36f5f` | Applied | 合并#6640：保留完整父子历史；功能处置及覆盖边界同 `2e1c7c00a686fbc446f94634522e31724ac23dc5`。 |
| `ab99d56e9626e6cd731592dae8553c9758a0efa2` | Applied + Overridden | 上游版本升级到0.2.1：本地版本按批准决定继续0.1.228，不覆盖本地发布约束。 |

### 文本冲突与本地兼容处理

共25文件、43个文本冲突块，按审批逐块组合，未整文件采用 ours/theirs。以下数量是实际冲突块数，自动合并文件另有语义审查和测试适配。

| 文件 | 冲突块 |
| --- | ---: |
| `backend/cmd/server/VERSION` | 1 |
| `backend/internal/service/account_stats_pricing.go` | 2 |
| `backend/internal/service/api_key_auth_cache_impl.go` | 1 |
| `backend/internal/service/api_key_auth_cache_profit_test.go` | 1 |
| `backend/internal/service/billing_service.go` | 2 |
| `backend/internal/service/model_pricing_resolver.go` | 1 |
| `backend/internal/service/gateway_usage_billing.go` | 1 |
| `backend/internal/service/pricing_service.go` | 2 |
| `backend/internal/service/pricing_service_test.go` | 1 |
| `backend/internal/service/gateway_billing_header_test.go` | 1 |
| `backend/internal/repository/http_upstream.go` | 2 |
| `backend/internal/service/gateway_count_tokens.go` | 1 |
| `backend/internal/service/gateway_upstream_request.go` | 1 |
| `backend/internal/service/openai_embeddings.go` | 1 |
| `backend/internal/service/openai_gateway_grok.go` | 1 |
| `backend/internal/service/openai_images.go` | 4 |
| `backend/internal/service/openai_images_responses.go` | 2 |
| `backend/internal/service/openai_model_alias.go` | 1 |
| `backend/internal/service/openai_ws_http_bridge.go` | 1 |
| `backend/internal/service/payment_order_expiry_service.go` | 1 |
| `backend/internal/service/payment_order_lifecycle.go` | 1 |
| `frontend/src/components/account/CreateAccountModal.vue` | 8 |
| `frontend/src/components/account/EditAccountModal.vue` | 1 |
| `backend/internal/handler/openai_gateway_handler_test.go` | 1 |
| `backend/internal/handler/openai_gateway_handler.go` | 4 |

1. **计费与映射**：保留严格请求模型、分组/渠道价优先、未知模型缺价拒绝、Free Fast 双成本、Grok/Gemini/GPT长上下文特例及nil保护。映射模型只参与调度；旧渠道计费来源仍归一为 requested，负载/粘性/模型路由不构成价格清单旁路。接入实际推理等级和 Fable 5.1 max 默认3倍及渠道覆盖。
2. **Astra/能力/认证**：接入 Astra、ultrafast、none来源与固定账号清单；补齐本地精确定价规范化中的 Astra/gpt-6，拒绝未知后缀和自定义别名隐式借价。固定清单默认关闭、仅只读检查组成员/Active/Schedulable/有效期，忽略限流/临时冷却；真实推理保护不变。认证快照 v23 保留新旧全部字段。
3. **WS与HTTP**：保留每轮模型映射/hash/usage/槽位释放及停调度阻断，接入共享不可变回放体和加密内容恢复。HTTP client receiver化仍启动本地首Token尝试；插件 RequestSent=true 后不可重放。Cyber 逻辑轮次采用原子双状态去重；失败终态返回错误并保留部分usage，不能视为成功。
4. **图片**：新回填选项默认关闭；本地原有强制内联 data URL/b64、隐藏真实上游URL的策略优先，不因关闭新开关而停用。原回填器保留字节嗅探、公网及大小限制；原强制内联下载也加图片专属公网/重定向与60秒超时防护，但不改业务 HTTP/私网 upstream开关。原实际交付、客户端断开及部分结果结算不回退。
5. **请求标识与隐私**：写配置和读取兜底均拒绝 Authorization、Proxy-Authorization、Cookie、Set-Cookie、X-API-Key、Api-Key、X-Auth-Token、X-Access-Token。无配置不采集；WS无响应头不伪造；上游ID和请求ID列默认隐藏。
6. **支付**：通用补查扩展支付宝；EasyPay支付宝由独立路径处理，保持30秒门槛、每轮10条/10秒预算和幂等履约；EasyPay微信保留原通用路径，查询集合无重复。
7. **账号和UI**：新账号请求ID头/图片回填与本地失败策略/图片尺寸同时保存；轻量列表和详情回取不恢复虚拟化，不覆盖滚动/分页/Teleport浮层。成功同步提示保留数量；测试显式卸载浮层，避免跨用例误点击。
8. **Claude与OpenCode**：Claude billing使用最终出站UA且保留Unicode字符索引断言；OpenCode新测试适配扩展方法返回值/模型参数，非流式仍无首Token守卫，不关闭任何生产检查。
9. **迁移/生成/版本**：四份新增上游迁移按完整文件名与本地232/233等同数字前缀共存；两个234文件不重命名，不改历史迁移。Ent/Wire生成一致性与版本覆盖分别核验。赞助素材两处删除属已审查上游变化，不是误删本地功能。

### 二开台账与保留项

同一代码提交更新 `docs/custom-development-history.md` 中16项受影响能力并追加变更记录；仍为53项、9域，无新增/停用/删除编号。按用户串行扣费、5秒usage task超时、全部生产性能参数、bind mount、回环暴露和两份活动Compose一致性继续保留；生产文件本次无修改，未读取真实实例配置或执行生产核验。纯上游能力没有单独新增二开编号。

### 验证记录

最终同环境验证完成时间：2026-09-05T20:05:49+08:00。以下均是本地结果，不代表生产验收。

- 后端默认/`unit`全包测试与全包build均退出0；golangci-lint **v2.13.0（Go1.27构建）**退出0，`0 issues`。
- 前端：同步前271文件/1962用例，后274文件/1994用例，完整Vitest均通过；lint/typecheck/build均退出0。Canvas前后均8文件/34用例，format/typecheck/test/build全部0；最终按Vue先构建、Canvas后构建保留产物顺序。
- Ent前后各320生成文件、0差异；Wire生成前后均0且正式生成文件未变。integration标签用`-exec`空执行器**只编译**，退出0不是实际执行通过。
- 三个部署静态脚本前后均0。Caddy脚本实际在`deploy/test-caddyfile-cache.sh`，早期runner误查`deploy/tests`导致同步前没跑；本次补跑退出0，不伪称同步前也已验证。
- 两份生产Compose字节一致且与同步前完全相同，SHA256：`608C0978CA699089D9BFB13B56DE00FAADC97E65C7EFC085B073AE7649EAEBE6`；`go.mod/go.sum`、前端/Canvas锁文件和正式Wire生成文件未混入工具变化。
- 工作树冲突标记0，高置信度秘密扫描命中0，意外删除0；只有已批准的两张赞助图被删。`git diff --check HEAD`和静态差异检查退出0。未跟踪文件0，临时目录均被既有规则忽略。

| 验证项 | 同步前最终退出码 | 同步后最终退出码 | 最后命令ID |
| --- | ---: | ---: | --- |
| backend/build | 0 | 0 | V21 |
| backend/test-default | 0 | 0 | V01 |
| frontend/lint | 0 | 0 | V14 |
| backend/test-unit | 0 | 0 | V18 |
| frontend/typecheck | 0 | 0 | V15 |
| frontend/test | 0 | 0 | V19 |
| frontend/build | 0 | 0 | V17 |
| canvas/format | 0 | 0 | V06 |
| canvas/typecheck | 0 | 0 | V07 |
| canvas/test | 0 | 0 | V26 |
| canvas/build | 0 | 0 | V09 |
| extra/golangci-lint | 0 | 0 | V20 |
| extra/compile-wrapper | 0 | 0 | V23 |
| extra/integration-compile-only | 0 | 0 | V24 |
| extra/wire-generate | 0 | 0 | V25 |
| extra/ent-generate | 0 | 0 | V27 |
| static/docker-compose-security-test | 0 | 0 | V10 |
| static/docker-compose-gateway-env-test | 0 | 0 | V11 |
| static/docker-runtime-resources-test | 0 | 0 | V12 |
| static/test-caddyfile-cache | 未执行（补充检查） | 0 | V28 |
| static/diff-check | 0 | 0 | V13 |
| backend/compat-regressions | 未执行（补充检查） | 0 | V30 |

**中间失败并未隐藏：** 初始pnpm11不兼容旧overrides及依赖自动安装，改用校验过的pnpm9.15.9和冻结锁文件安装；Vitest固定`--maxWorkers=4 --minWorkers=1`，不降低断言或关闭检查。Ent首次因空/过深目标无法推断Go包而失败，使用隔离包占位与tools.mod/tools.sum后通过；正式go.sum曾出现8条工具校验和，已确认原因并在merge前只恢复该文件，未混入提交。后端新OpenCode测试签名、非流式nil守卫、Astra严格规范化/自定义别名显式价格、图片内联结果、Cyber失败部分usage、渠道旧upstream来源预期，以及前端计数提示/Teleport用例串扰均已按保留本地业务契约修复；针对性重跑及最终完整回归均0。

<details>
<summary>全部84次实际验证调用、命令和退出码</summary>

路径宏仅用于缩短记录：`<R>=D:/project/sub2api`；`<V>=D:/project/sub2api/output/upstream-sync-20260905-ab99d56e9`；`<G>=C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin`。命令ID对应同一工作目录、可执行文件和完整参数；未用新命令覆盖旧失败。完整日志、每次起止时间和命令JSON留在验证目录。所有测试清除真实业务/API/数据库环境变量，仅使用mock/隔离夹具；GOMAXPROCS=8、GOFLAGS=-p=4、GOMEMLIMIT=6GiB，TMP和缓存均位于仓库。

| ID | 工作目录 | 实际程序及参数 |
| --- | --- | --- |
| V01 | `<R>/backend` | `<G>/go.exe test ./... -count=1 -timeout=20m` |
| V02 | `<R>/frontend` | `C:/Users/xk/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/fallback/pnpm.cmd run lint:check` |
| V03 | `<R>/frontend` | `C:/Users/xk/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/fallback/pnpm.cmd run typecheck` |
| V04 | `<R>/frontend` | `C:/Users/xk/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/fallback/pnpm.cmd run test:run --maxWorkers=4` |
| V05 | `<R>/frontend` | `C:/Users/xk/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/fallback/pnpm.cmd run build` |
| V06 | `<R>/canvas` | `C:/Program Files/nodejs/node.exe node_modules/prettier/bin/prettier.cjs --check .` |
| V07 | `<R>/canvas` | `C:/Program Files/nodejs/node.exe node_modules/typescript/bin/tsc --noEmit` |
| V08 | `<R>/canvas` | `C:/Program Files/nodejs/node.exe node_modules/vitest/vitest.mjs run --maxWorkers=4` |
| V09 | `<R>/canvas` | `C:/Program Files/nodejs/node.exe node_modules/vite/bin/vite.js build` |
| V10 | `<R>` | `C:/Program Files/Git/bin/bash.exe <R>/deploy/tests/docker-compose-security-test.sh` |
| V11 | `<R>` | `C:/Program Files/Git/bin/bash.exe <R>/deploy/tests/docker-compose-gateway-env-test.sh` |
| V12 | `<R>` | `C:/Program Files/Git/bin/bash.exe <R>/deploy/tests/docker-runtime-resources-test.sh` |
| V13 | `<R>` | `git diff --check` |
| V14 | `<R>/frontend` | `C:/Program Files/nodejs/node.exe <V>/pnpm-9.15.9/package/bin/pnpm.cjs run lint:check` |
| V15 | `<R>/frontend` | `C:/Program Files/nodejs/node.exe <V>/pnpm-9.15.9/package/bin/pnpm.cjs run typecheck` |
| V16 | `<R>/frontend` | `C:/Program Files/nodejs/node.exe <V>/pnpm-9.15.9/package/bin/pnpm.cjs run test:run --maxWorkers=4` |
| V17 | `<R>/frontend` | `C:/Program Files/nodejs/node.exe <V>/pnpm-9.15.9/package/bin/pnpm.cjs run build` |
| V18 | `<R>/backend` | `<G>/go.exe test -tags=unit ./... -count=1 -timeout=20m` |
| V19 | `<R>/frontend` | `C:/Program Files/nodejs/node.exe <V>/pnpm-9.15.9/package/bin/pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` |
| V20 | `<R>/backend` | `<V>/go-tools/bin/golangci-lint.exe run ./... --timeout=30m` |
| V21 | `<R>/backend` | `<G>/go.exe build ./...` |
| V22 | `<R>/backend` | `C:/Program Files/nodejs/node.exe <V>/check-ent.mjs before` |
| V23 | `<R>/backend` | `<G>/go.exe build -o <V>/artifacts/compile-only.exe <V>/compile-only.go` |
| V24 | `<R>/backend` | `<G>/go.exe test -tags=integration -exec <V>/artifacts/compile-only.exe ./... -count=1` |
| V25 | `<R>/backend` | `<G>/go.exe generate ./cmd/server` |
| V26 | `<R>/canvas` | `C:/Program Files/nodejs/node.exe node_modules/vitest/vitest.mjs run --maxWorkers=4 --minWorkers=1` |
| V27 | `<R>/backend` | `C:/Program Files/nodejs/node.exe <V>/check-ent.mjs after` |
| V28 | `<R>` | `C:/Program Files/Git/bin/bash.exe <R>/deploy/test-caddyfile-cache.sh` |
| V29 | `<R>/backend` | `<G>/go.exe test -tags=unit ./internal/service -run Test(AstraForward&#124;GPT6AstraDedicated&#124;NormalizeKnownOpenAI&#124;OpenAIGatewayServiceForwardImages&#124;OpenCodeSession&#124;ForwardOpenAIWSV2_MarksCyberPolicy&#124;SelectAccountWithLoadAwareness_.*Restriction&#124;本地图片) -count=1 -timeout=10m` |
| V30 | `<R>/backend` | `<G>/go.exe test -tags=unit ./internal/service -run Test(AstraForward&#124;GPT6AstraDedicated&#124;NormalizeKnownOpenAI&#124;OpenAIGatewayServiceForwardImages&#124;OpenCodeSession&#124;ForwardOpenAIWSV2_MarksCyberPolicy&#124;SelectAccountWithLoadAwareness_.*(Restriction&#124;LegacyUpstream)&#124;本地图片) -count=1 -timeout=10m` |

| 阶段/项目 | 开始时间（2026-09-05 +08:00） | 命令 | 退出码 |
| --- | --- | --- | ---: |
| 同步前/backend/test-default | 19:03:20 | V01 | 0 |
| 同步前/frontend/lint | 19:03:20 | V02 | 1 |
| 同步前/frontend/typecheck | 19:04:12 | V03 | 1 |
| 同步前/frontend/test | 19:04:13 | V04 | 1 |
| 同步前/frontend/build | 19:04:15 | V05 | 1 |
| 同步前/canvas/format | 19:04:18 | V06 | 0 |
| 同步前/canvas/typecheck | 19:04:30 | V07 | 0 |
| 同步前/canvas/test | 19:04:47 | V08 | 0 |
| 同步前/canvas/build | 19:05:32 | V09 | 0 |
| 同步前/static/docker-compose-security-test | 19:06:39 | V10 | 0 |
| 同步前/static/docker-compose-gateway-env-test | 19:06:40 | V11 | 0 |
| 同步前/static/docker-runtime-resources-test | 19:07:10 | V12 | 0 |
| 同步前/static/diff-check | 19:07:11 | V13 | 0 |
| 同步前/frontend/lint | 19:08:08 | V14 | 0 |
| 同步前/frontend/typecheck | 19:09:16 | V15 | 0 |
| 同步前/frontend/test | 19:09:44 | V16 | 1 |
| 同步前/frontend/build | 19:09:54 | V17 | 0 |
| 同步前/backend/test-unit | 19:09:59 | V18 | 0 |
| 同步前/canvas/format | 19:11:11 | V06 | 0 |
| 同步前/canvas/typecheck | 19:11:16 | V07 | 0 |
| 同步前/canvas/test | 19:11:21 | V08 | 0 |
| 同步前/canvas/build | 19:11:25 | V09 | 0 |
| 同步前/frontend/test | 19:13:32 | V19 | 0 |
| 同步前/extra/golangci-lint | 19:13:56 | V20 | 0 |
| 同步前/backend/build | 19:14:38 | V21 | 0 |
| 同步前/extra/ent-generate | 19:15:15 | V22 | 1 |
| 同步前/extra/compile-wrapper | 19:16:49 | V23 | 0 |
| 同步前/extra/integration-compile-only | 19:16:54 | V24 | 0 |
| 同步前/extra/wire-generate | 19:18:39 | V25 | 0 |
| 同步前/extra/ent-generate | 19:20:00 | V22 | 0 |
| 同步后/backend/build | 19:27:44 | V21 | 0 |
| 同步后/backend/test-default | 19:31:06 | V01 | 1 |
| 同步后/frontend/lint | 19:31:07 | V14 | 0 |
| 同步后/backend/test-unit | 19:32:10 | V18 | 1 |
| 同步后/frontend/typecheck | 19:32:32 | V15 | 0 |
| 同步后/frontend/test | 19:33:21 | V19 | 1 |
| 同步后/backend/build | 19:33:37 | V21 | 0 |
| 同步后/backend/test-default | 19:35:00 | V01 | 1 |
| 同步后/frontend/build | 19:35:47 | V17 | 0 |
| 同步后/canvas/format | 19:37:05 | V06 | 0 |
| 同步后/canvas/typecheck | 19:37:09 | V07 | 0 |
| 同步后/canvas/test | 19:37:16 | V26 | 0 |
| 同步后/canvas/build | 19:37:39 | V09 | 0 |
| 同步后/backend/test-unit | 19:38:20 | V18 | 1 |
| 同步后/backend/build | 19:42:30 | V21 | 0 |
| 同步后/frontend/lint | 19:42:40 | V14 | 0 |
| 同步后/extra/golangci-lint | 19:42:40 | V20 | 0 |
| 同步后/frontend/typecheck | 19:43:07 | V15 | 0 |
| 同步后/frontend/test | 19:43:39 | V19 | 1 |
| 同步后/extra/compile-wrapper | 19:44:50 | V23 | 0 |
| 同步后/extra/integration-compile-only | 19:44:50 | V24 | 0 |
| 同步后/backend/build | 19:45:16 | V21 | 0 |
| 同步后/frontend/build | 19:45:27 | V17 | 0 |
| 同步后/extra/wire-generate | 19:45:42 | V25 | 0 |
| 同步后/extra/ent-generate | 19:45:54 | V27 | 0 |
| 同步后/static/docker-compose-security-test | 19:46:04 | V10 | 0 |
| 同步后/backend/test-default | 19:46:05 | V01 | 1 |
| 同步后/static/docker-compose-gateway-env-test | 19:46:05 | V11 | 0 |
| 同步后/static/docker-runtime-resources-test | 19:46:34 | V12 | 0 |
| 同步后/static/test-caddyfile-cache | 19:46:34 | V28 | 0 |
| 同步后/static/diff-check | 19:46:35 | V13 | 0 |
| 同步后/backend/compat-regressions | 19:52:57 | V29 | 1 |
| 同步后/frontend/lint | 19:52:57 | V14 | 0 |
| 同步后/frontend/typecheck | 19:53:27 | V15 | 0 |
| 同步后/frontend/test | 19:53:52 | V19 | 0 |
| 同步后/frontend/build | 19:55:24 | V17 | 0 |
| 同步后/backend/test-default | 19:55:41 | V01 | 0 |
| 同步后/canvas/format | 19:56:28 | V06 | 0 |
| 同步后/canvas/typecheck | 19:56:31 | V07 | 0 |
| 同步后/canvas/test | 19:56:35 | V26 | 0 |
| 同步后/canvas/build | 19:56:39 | V09 | 0 |
| 同步后/backend/test-unit | 19:58:57 | V18 | 0 |
| 同步后/backend/compat-regressions | 20:02:18 | V30 | 0 |
| 同步后/backend/build | 20:03:06 | V21 | 0 |
| 同步后/extra/golangci-lint | 20:03:14 | V20 | 0 |
| 同步后/extra/compile-wrapper | 20:04:08 | V23 | 0 |
| 同步后/extra/integration-compile-only | 20:04:09 | V24 | 0 |
| 同步后/extra/wire-generate | 20:04:31 | V25 | 0 |
| 同步后/extra/ent-generate | 20:04:41 | V27 | 0 |
| 同步后/static/docker-compose-security-test | 20:05:31 | V10 | 0 |
| 同步后/static/docker-compose-gateway-env-test | 20:05:31 | V11 | 0 |
| 同步后/static/docker-runtime-resources-test | 20:05:49 | V12 | 0 |
| 同步后/static/test-caddyfile-cache | 20:05:49 | V28 | 0 |
| 同步后/static/diff-check | 20:05:49 | V13 | 0 |

</details>

### 未验证项与残余风险

- 本机没有可用 Docker/Podman，integration标签只有编译，未真实运行Testcontainers、PostgreSQL/Redis、SQL迁移、Compose或服务启动/健康检查；不能将编译结果称为集成通过。
- 未执行真实模型/Token对比、图片下载、支付/回调、数据库写入、真实插件、浏览器E2E、race和govulncheck。没有访问两台远程服务器，没有push、PR或部署。
- 本地模拟和静态检查通过不代表所有生产组合安全；新字段和四份迁移在未来另行获批的隔离部署前仍须验证。固定账号清单忽略临时冷却是只读清单策略，不应据此放宽推理调度。
- 本次只更新到固定SHA，不宣称任务完成时互联网上已无更晚提交。
- 临时工具、缓存、完整命令日志和隔离生成文件留在仓库忽略目录 `output/upstream-sync-20260905-ab99d56e9` 与 `backend/.gocache/upstream-sync-ab99d56e9`，不提交、不擅自删除其他本地资料。


### 本地代码提交与完成门禁补记

- 补记时间：2026-09-05T20:11:35.141+08:00。状态：**成功完成本地代码集成及可执行验证**；真实集成/迁移/健康/生产能力仍按上述范围标为未验证。
- 本地合并代码提交：`289a00a46b023c957db16bbc30a1fc15c30d44a0`（同步：合并上游更新至 ab99d56e9 并保留二开功能）。两个父提交依次为`6fed6e108dd7dcdcda2d03251e95a4dedd7aae9b`与`ab99d56e9626e6cd731592dae8553c9758a0efa2`，已核验不是squash或单亲替代。
- 上游到本地映射：本节82个上游SHA全部作为祖先完整进入上述merge，统一映射到该合并代码提交；逐项内容处置仍为43 Applied、39 Applied + Overridden，无跳过或延期。
- 相对同步前，合并提交变更298文件（296个上游及兼容相关文件＋2份台账），新增13090行、删除1057行；版本文件保持本地内容，因此不计入最终变更文件。完整文件清单可由`git diff --name-status 6fed6e108dd7dcdcda2d03251e95a4dedd7aae9b 289a00a46b023c957db16bbc30a1fc15c30d44a0`复核。
- 暂存前检查：25个索引冲突全部清除；冲突标记0、意外文件0、未跟踪文件0；53个稳定二开编号无重复/删改。代码提交后工作区干净，原`main`仍为同步前SHA。最终记录只补充这两份台账，不再修改已验证代码。
- 已通过差异、双亲/祖先和不可变main门禁；本地交付仅允许在最终记录提交后执行`git switch main`及`git merge --ff-only sync/upstream-20260905-ab99d56e9`。记录提交自身SHA与最终main SHA只在最终总结报告，避免文档自引用。
- 未push、未创建PR、未访问服务器、未部署或修改生产数据；未做自动stash、reset --hard、clean或无关文件清理。


### v0.1.229 远端交付授权与候选准备

- 时间：2026-09-05T20:20:28.331+08:00。用户在完成本地同步后追加授权：推送候选代码，CI通过后合入远端主线，创建新tag，由GitHub Actions自动部署服务器1。前述“未推送/未部署”描述的是前一阶段已完成时的事实，不代表本阶段仍禁止交付。
- 实时核验：远端main仍为`6fed6e108dd7dcdcda2d03251e95a4dedd7aae9b`；最新正式发布为`v0.1.228`，目标`v0.1.229`不存在。候选分支为`codex/release-v0.1.229`，从已验证的本地`6ec6ccd092aaed72e34e2010c4b8ea8636f1fc34`开始，仅递增VERSION并补记录，不扩大固定上游范围。
- 交付门禁：候选分支的CI与Security Scan均通过后才允许远端main快进；若失败则在候选分支修复并重新验证，不强推或绕过检查。新建annotated tag，不覆盖旧tag。main与tag指向的代码必须一致，Release工作流正常构建镜像后自动部署服务器1，不手工重启。
- 发布前须只读核验服务器1的活动Compose、应用/PostgreSQL bind mount及本机健康；不访问服务器2、不读取或输出真实.env/私钥，不改生产资源或数据挂载。CI/发布/部署实际结果以相应Actions运行与最终交付总结为准；仅准备候选不写成已部署。


### 发布安全预检与自动门禁修复

- 2026-09-05T20:23:57+08:00 只读核验：服务器1运行`saviour2411/sub2api:0.1.228`，revision为`6fed6e108dd7dcdcda2d03251e95a4dedd7aae9b`，活动Compose哈希与仓库相同；应用、PostgreSQL和Redis均使用预期bind mount，回环18080健康正常。仅备用Compose落后（旧SHA256：`2865001838d1c9e8fedc798e77742481ba7b6ac09774e417e889945bbb49a05a`）。
- 旧备用文件已备份至仓库忽略目录`output/release-20260905/server1/docker-compose.sub2api.before.yml`并校验哈希；随后只把活动文件复制到备用文件，两者现为`608c0978ca699089d9bfb13b56de00faadc97e65c7efc085b073ae7649eaebe6`，活动配置、容器ID、实例.env和数据目录未改变。
- 自动部署脚本原先没有在重建前强制检查上述约束，因此本发布候选补充生产目标、双Compose、精确bind mount、旧实例健康及缺少检查工具时的失败关闭门禁；不改变正常发布版本更新或业务参数。配套更新二开台账CUST-OPS-003/004并新增12项本地隔离测试，已通过，纳入CI的macOS shell任务。
- 第一候选`d5b8a3a1441a627ebd88a3579975e5a334f4816f`已触发CI和Security Scan；新增安全门禁后必须以新的候选SHA重新检查，旧候选通过不能替代新候选的合入门禁。

- 首轮候选CI（运行33965856935）的shell、frontend、golangci-lint与test四项全部success，test中的单元和集成步骤均通过；Security Scan（运行33965856933）两个任务success。新加部署安全门禁不复用该结果，仍重新检查最新提交。


## 2026-09-11：增量同步至 98d86915b

### 固定范围与权限

- 时间：2026-09-11T00:49:38+08:00（Asia/Shanghai）。状态：本地集成与验证通过；合并代码提交 SHA 见本节最后记录补记。
- 原目标：`main`；LOCAL_PRE_SYNC_SHA：`d3c44a97b0a25fddd7fae3cdeb537682f365ebf5`。
- 上游：`https://github.com/Wei-Shaw/sub2api.git`，远端 HEAD 核验默认分支 `main`；本次 fetch 后即固定目标，不自动扩大到之后提交。
- UPSTREAM_OLD_SHA：`ab99d56e9626e6cd731592dae8553c9758a0efa2`；UPSTREAM_NEW_SHA：`98d86915becae9fe9491a91ffc6defd5235c8d2b`。
- ACTUAL_MERGE_BASE：`ab99d56e9626e6cd731592dae8553c9758a0efa2`，等于已记录旧基线，旧基线是双方祖先；实际 merge 范围无额外未审提交。
- LAST_FULLY_INTEGRATED_UPSTREAM_SHA：`98d86915becae9fe9491a91ffc6defd5235c8d2b`。
- 范围共 181 提交（103 普通、78 merge），499 个上游变化文件、189 个本地重叠文件；完整 patch-id 检查未发现重复集成。
- 原 main 与 origin/main 一致且干净，用户集中批准后先建隔离分支、运行基线，再执行 `git merge --no-ff --no-commit 98d86915becae9fe9491a91ffc6defd5235c8d2b`。
- 备份：`backup/pre-upstream-sync-20260910-232555-d3c44a97b`；同步：`sync/upstream-20260910-98d86915b`。版本保留 `0.1.231`。
- 本次授权仅限本地；未 push/PR、未 SSH、未部署或写生产数据，未自动 stash、reset、clean、整文件 ours/theirs。

### 功能变化与冲突处理

- 纳入分组请求白名单与数据修复、固定账号模型发现、简单模式基本分组、MiniMax、媒体及协议兼容、DeepSeek 峰谷账号成本、代理到期回退、渠道缓存广播、HTTP/2 PING、监控排名、日志保留、密钥分页及 i18n 门禁等全部获批变化。
- 实际 24 个文本冲突与预演一致：Makefile、VERSION、Wire、OpenAI handler/测试、API 合约/路由、账号成本/测试/管理/认证缓存、Live、WS forwarder/relay、插件 ZIP、公共设置/依赖及四个前端账号/用量文件。逐冲突处理，未整文件选任一侧。
- 白名单在认证之后、Composite 改写之前；本地公开市场与批量图片模型列表同步过滤。未知模型变体不扩大为已知基础模型；启用空白名单不回填默认列表。
- WS 保留首语义输出、逐轮模型/定价/Cyber、失败停调度与人工暂停隔离；策略验证后只占槽一次，终态写入后结算，断连排空元数据完整且失败不伪报成功。插件 RequestSent=true 后不重放。
- 账号复制测试计划、固定默认测试模型、请求模型严格缺价、Free Fast 双成本、长上下文、Fable 无默认三倍、Canvas/签到/本地市场开关和 DataTable 行为均保留。
- 两份生产 Compose 字节一致，仅共同加入 SUB2API_IMAGES_MAIN_MODEL；bind mount、回环、HTTP upstream、资源参数、生产持久化 3600 秒响应头覆盖、5 秒 usage task 与按用户串行扣费均不变，未读真实实例配置。
- Ent 隔离生成 320 文件无差异，Wire 重新生成接线并验证；不重命名或改写历史迁移。

### 逐提交处置

- Applied：148；Applied + Overridden：33；Already Applied/Skipped/Deferred/Conflict：均为 0。所有 SHA 经完整 merge 保留祖先关系，统一映射到最后补记中的合并代码提交。Overridden 表示已集成且有明确本地覆盖，不是跳过。

| 上游完整 SHA | 处置 | 功能组 | 原因/覆盖 |
| --- | --- | --- | --- |
| `cff3f89850738e3504e35b01c9b4028d62b65a01` | Applied + Overridden | 分组准入与模型目录 | 请求白名单按批准启用；保留本地精确模型别名、WS 逐轮计费/Cyber 校验、图片统计和公开模型市场。 |
| `55c5eed9fc74debfeb774790f88ecfd6c3466e2f` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `f3bbb95310fe01ef29f5f86f7b7eeea991cc443e` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `cb31033972023e596d1f16f6fd944b3ca09a6478` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `f88d62ad282bd6dceffa93924e81543cf481c9ff` | Applied + Overridden | 分组准入与模型目录 | 合入可用模型发现；保留本地固定默认测试模型、配置映射选择和恢复事故模型优先级。 |
| `a3aa9bae6da021bae11f09bcb563704fc9d5794f` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `a3675552bd1af0eadb42b6992f1258ecf8d6faf2` | Applied + Overridden | 简单模式与账号管理 | 基础分组纳入简单模式，同时保留本地账号复制测试计划、失败缓存和人工暂停依赖。 |
| `05ad6b49a1605ca0aa3c5eee597139a1037bd450` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `f46038820bc29197ebf9f39e10138021e26f70f9` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `4a4fd35e0cb82765059d8409fa29736ca30eef74` | Applied + Overridden | 简单模式与账号管理 | 空分组删除锁与本地差量绑定、上游倍率绑定清理共存；SQLite 夹具保留存在性校验。 |
| `b59f3bf46c66a24b6c59260092d66d2d871c7173` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `a7d2c782df93b017c50be117d805d6c6c76aef87` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `8c8fd6e298521f1032a739d42cb0acdf0b8a1522` | Applied | 页面认证与使用记录 | 订阅用量跳转、密钥分页、认证加载、注册可见性、支付安全 Markdown 与自定义页拖动；保留本地列设置和认证容错。 |
| `8f7f2b5d9235435d872ad395f13de21798c02b73` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `7a70de401c01ffdd8c58357451d772f757d4ec2b` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `ce328fb37a55ee8a28d8a5c07c59e3e2250a0e5b` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `0b061c5fa133bf930981816d483d563231a63952` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `e274de45b784fc39bac880a57ec61c4b1e361cd3` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `985428d9d2f5fabb7aa99b10ac702f3e41dfae5d` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `9578ddbddd863185503bb9a4abf811ccf888a348` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `222181efd6bebc28d2258a7ac5b5b4a8044fb649` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `c0420e2b8a45598c39f957dd7e5ce6c88ac1c814` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `5fb5dfb34cd1540a39639983f0e7b17c2a62ce58` | Applied + Overridden | 网关调度与流式生命周期 | 逐轮重新占槽纳入；保留本地策略校验成功后才占槽的顺序，移除合并产生的重复回调。 |
| `959cbe3d87186c80b36a2b8bde44559bf7da17e3` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `6271c517da6ec76905667fb5f2bcd958d5acdfd9` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `8116d235ef62dc1d88e7a2a9603058e87e368cea` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `28167bbf8596dc031e0cea70077611890cb8940a` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `027e3f2d54a1e0fedbb1b1b65c09f77941cdebfa` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `96884dd1aee34c9c76dbe827476dcaa3bb83ad86` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `f09a5b602e57dc712a1ee8df20f569769308e778` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `8363d537e0dd88a749e1faa7a3f47ca3b4f91cfd` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `f8e1e7fed64bd2966199b86f4f34e69b0517a572` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `3cb2381bdc9fafcc895f331212fd25fb74847983` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `ce157b32ed584bd91b1edafcf305ca5fb84c52ed` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `3efeba33a47a9cfc51faac6057806a50cabee12d` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `787a6a33df3c7a7d17563d1e9d61e3d4800e38e7` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `b3ad9b67cfb3bd7d419a924725ef6bbf2a6f2f5a` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `111c41cf3f345c36a40ed7cbf7f279cb7a2627b0` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `a6430bfe20ce5ebb7c8e625d49c5540c5c8b53f0` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `1f716aa18142880a69790c94008b1e3a3374405c` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `32bf3d0f33c39e2ca8a9ca5816daccf620dc3e92` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `96cf8bd76ab0e062e0b8774ef11d189f1db42761` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `c7343d2aa26274fda4d95646ba7315c5f59f4e49` | Applied | 监控与日志 | 监控排名开关、探测角色与系统日志保留边界；保留本地配额隐藏及其他功能开关。 |
| `b8ff74ad701c13cb11403f6b3714c9bd5d5e602d` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `a16070ccf62200674d51ad2fb3ae58ba50b013d3` | Applied | 页面认证与使用记录 | 订阅用量跳转、密钥分页、认证加载、注册可见性、支付安全 Markdown 与自定义页拖动；保留本地列设置和认证容错。 |
| `909b8c7ef44cfc7eeae18e37f629c4e58c9b727b` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `81ff9384cb6bcba37b070acaa1173b8bb09b582f` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 已审查相对自动合并额外产生的解决内容。 |
| `b939fa9d4a2128f06be9a9e86bbdfd70659d1ee9` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `7ae0312091520ce46aea4fe0016f57e14c17d730` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `2cc1e7ef900962735992c42e351430abe3a0603e` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `b8ed24508555406a34b0bbc9c980ff158362d678` | Applied + Overridden | 网关调度与流式生命周期 | 合入后续轮额度恢复；保留失败停调度阻断后续轮、可重建上下文和逐轮结算边界。 |
| `22c16d0517d1511c95678e5cb6fcf24cc257453b` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `58e35a4f327e82fe4f4c9de9e53c0e53a7fd7bad` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `f7c48ba59a81e8a2dee644866e3ea1cae9f96f2c` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `3bdb869fcad8ce2c6881a3f503c2964ef23d5575` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `b9ebc8f20989e5793455d4da64ad336e797b6e35` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `dc6b318c343dfc4e7b5cb2a4c4b4b6455b23237a` | Applied + Overridden | 页面认证与使用记录 | 使用记录密钥分页纳入，继续使用本地筛选控件与列设置，不恢复旧下拉实现。 |
| `c05bc4d3ccb4d52b88e7261df58d03ed5f59aa15` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `2e31d8b702aa4d27e2c71e3066d7c6c612ae2025` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `43491f67a15f2675ea19cf49dd9774a6e1a0b29b` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `2db78bd3d68f58968d73b664c289c79ebee71aee` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `ad4b2f30f591e4aa9bf331cc450e4ffd9b02aaf9` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `561fc1c3e455e405af4d604339e8e9df4f044445` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `6aabbdf547d305bc3d22acfa5529e47087b12892` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `81c0f11d7a8634e2fd2b04deb6dbb5465e59ea66` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `8a469478659b0a87405193afa89df0e0a9f3302f` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `cbb4b7e53e4bd7e94f09a72325f27250b0360e5c` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `c74a4f521c7b71635b20a2f15e9d2758991de05c` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `2b1e5e7f5c65309c57771744754b94ed8d8d0d35` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `a7f1ed8fa49a2f3a7ae9ba81eac45f8d22f426f7` | Applied + Overridden | 网关调度与流式生命周期 | pending-turn 时序与本地 response-ID/模型逐轮绑定共存，合法终态和断连不得双结算。 |
| `4bfcbd0c78b92ff498d437b65fa2499ac4550642` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `a7742306616c9596bb564d790d5a21e5a88dd0b0` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `71e5a4baf79894ddbfcef04af00e15dfde997d38` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `958a21ad6e2eddae4b1ebafa68c24dee8666092a` | Applied + Overridden | 计费与兑换 | 统一峰谷账号成本管线；保留无渠道仍按上游型号计成本、请求模型严格缺价和 Fable 无默认三倍。 |
| `3eb5a84ea132c006509a2d98a7e0a57b6523bdb1` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `95023e7d49cde9f740a4875b3c5d2582ea230628` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `88b697d5dfb3de1b42beca811bec5a3c107ebf98` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `666a5c6f877a306c4743310c04dc5f400656f7a5` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `abf70750fdb697be3e87a59ff19a45f88e0bb570` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `f49a3935667e1a081cd0d2f5b9669cd429c2a597` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `eba85dc47854c7cb0942173a09177170ea7675eb` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `00eabe8ab9dd9de16cf07f62a165b98671c40a97` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `de69bf1e031a08ffb805e1b82902aa106bd132a8` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `b1ce821c497adabcdce6631577134b6ac23040ec` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `65246e69d72159f515b9b9d6c60f92bed8434d9b` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `f62ec2e4a0f7ad7f92359971ef52c80bfb974b48` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `dc46daa690d8a9bfb3718772a7b6b29909ce3a24` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `415a0dc9d0cae3bda18bb1c1573e16de999eb3bf` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `ff758f37d2e751aa26183c6b4473974eb5c69971` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `d03c42d7986cad7170d9e4751852287e84ec3374` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `66cd8a72bc9315396d6b84524ff3c3e84054fa22` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `3aeab296d8ec9a07bdaacdab80cfcaf011f6c15d` | Applied | 页面认证与使用记录 | 订阅用量跳转、密钥分页、认证加载、注册可见性、支付安全 Markdown 与自定义页拖动；保留本地列设置和认证容错。 |
| `10fe591e5477ccce0b7dd8a799f0d6cb0d2a1a0a` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `d106b3b5a067eec3499ce246635ab3bae02e0ab8` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 已审查相对自动合并额外产生的解决内容。 |
| `b491c3271bd0ca33da9131528c6a85cfcb08015f` | Applied | 计费与兑换 | 峰谷成本、缓存价格和支付兑换隔离；继续按请求模型计费、按用户串行扣费且无 Fable 默认三倍加价。 |
| `f6d580200a5442e2da6e4954b60208351cf50ed8` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `8193b80a59ada85d8cdff4f1e47a9be48fca6292` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `2031c1d5ad91cdc4e381fd535b216e11ff770bb7` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 已审查相对自动合并额外产生的解决内容。 |
| `f8351e9e33477287379c37cdd6a2a044feeeb1fc` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `0aaed397cc62cca9d954ba33e1715725cdbeccf7` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `5485f368b29d05adb95a00f71801c7c23d8f48af` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `b7dba62678a834080564966c002fd0ca2b328b7a` | Applied + Overridden | 构建部署与发布元数据 | 保留本地 VERSION 0.1.231，不采用上游 0.2.2。 |
| `e094a3f40b1c84b78c5f6298004145587a3ba2a0` | Applied + Overridden | 分组准入与模型目录 | 保留本地默认测试模型及显式任务模型，兼容实时发现的模型显示名称。 |
| `b439dddfda4ea696480482a8df97b73260b6006b` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `cc91155fe4cb7dcb37228be034749924750465c8` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `2fc24d8879951d571e540718aa011c30ccd7b9a6` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `57387445f3cf204717d9b15b3e8a863c52d1115d` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `14e0a49e17afebf62c5f788f4ef1dc8eef56ac76` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `4a7739acdf4ff12a1683c74a9e99163d15f7d11c` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `8fa67d477d6651a744754392a8982ea589c26ae6` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `772a0382f079676983c06f24b0d41e09139a8462` | Applied + Overridden | 构建部署与发布元数据 | 保留本地 VERSION 0.1.231，不采用上游 0.2.3。 |
| `19382f275e8fdc05a655e3e20fbd688bb9a7ec2a` | Applied + Overridden | MiniMax 平台 | MiniMax 完整接入，本地账号测试和手动预选继续按确定性配置映射规则工作。 |
| `6f2295bfc7a29367e3a300909c75bc79720804f1` | Applied | MiniMax 平台 | 完整接入 MiniMax 平台、配额、监控和调度，补齐本地账号测试平台枚举。 |
| `3495635a52bb0009ad15b4a6990ac3d2b1c43811` | Applied | MiniMax 平台 | 完整接入 MiniMax 平台、配额、监控和调度，补齐本地账号测试平台枚举。 |
| `10940ecf39bf67000c668b07facd6321ec7fb21e` | Applied | MiniMax 平台 | 完整接入 MiniMax 平台、配额、监控和调度，补齐本地账号测试平台枚举。 |
| `dbe92a1c241a03c77e2b218761369ff988b8356b` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `859de4c8fddb1afe3aaebba0a7b0fcf48eaa46eb` | Applied | 代理与日期校验 | 代理到期回退、有向共享备份关系与日期校验。 |
| `28b807a7353154d09f48b084e757ed866824d6f4` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `efc6e4a816df82189f8e866c8a56b4b3f2032378` | Applied | 代理与日期校验 | 代理到期回退、有向共享备份关系与日期校验。 |
| `4892b8f17edcda6a4355ded98c190ac5775982b9` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `941a0487abc30ff9c66af5056244b25087d15b17` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `3ee92b40c5fc34e1254f304219f1da4881b8d00e` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `df64b5f368cce77b5ebc32a86084dd1f8dd40dbf` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `6378fb0cbaff51c968470ef2fe5271e79d59b9fc` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `b6ee9f0a320cbdd980c68e916ff551e1feb573a0` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `979fe247f9026db677598de170f93d6e1930215e` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `35e69af41b13f42c391f36bea0beb405bec26e7e` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `9bcf8a5b801badd8ab39f200ee559b08ec57d2ff` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `be4ab92b2bdcd3e27fbe819546665dd15723912e` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `b6384452347fb6d240fe25dbfca202ca77e6acfb` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `99eba19ffce4b661f4720c139da40434784d374e` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `1923d1c27d411beb5ad8c1fb875900e06c8ec11c` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `38cfd7e2d94decaa2c185169955f996d9990dc37` | Applied | 代理与日期校验 | 代理到期回退、有向共享备份关系与日期校验。 |
| `9e7039ebca5d4cd668ca1868becbd243d14f3c65` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `3fc08745a6a41dc8e77a526e4585d7b6cf045f22` | Applied | 分组准入与模型目录 | 模型白名单、清单编辑、固定账号发现、推理等级拒绝及列修复；适配本地模型市场与严格缺价检查。 |
| `1e6114c27090a8a9b5f55276feb32c28aa228ba9` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `7f0f579bb0a4e71c2041e9ff1fc16de4b1883fac` | Applied | 简单模式与账号管理 | 基础分组边界、账号管理修复与周成本展示；保留本地计划复制、默认测试模型和人工暂停。 |
| `68773aab9862256a274b5a78e76c73d93847cb60` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `dfb3978b9d95c174b3402da0f8f5358ae29a9388` | Applied | 页面认证与使用记录 | 订阅用量跳转、密钥分页、认证加载、注册可见性、支付安全 Markdown 与自定义页拖动；保留本地列设置和认证容错。 |
| `fa556700c28f1a73b5e1cef101afbfcf611a218f` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `6c8ad0bd4a05914065c4cd22800e6624eac32b73` | Applied | 页面认证与使用记录 | 订阅用量跳转、密钥分页、认证加载、注册可见性、支付安全 Markdown 与自定义页拖动；保留本地列设置和认证容错。 |
| `ae4cc14b280e91f407b12a141c768d88d6c535ab` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `188e3a9f90498ce6104ce0158486f34f9b57b9d7` | Applied | 代理与日期校验 | 代理到期回退、有向共享备份关系与日期校验。 |
| `5cbb9191a76394d6c3a95ec8a83b2f8e80265802` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `335fcdc1d657f93833a5fd8183f9acb2ed142fc0` | Applied + Overridden | 构建部署与发布元数据 | ZIP 在提交安装目录前关闭，与本地幂等清理及错误回滚路径合并，避免重复关闭。 |
| `b42157a725fa9079192c1d1badc12a8647f6ff52` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `aa167a25a7ec3bf27acf1814f64e9157f55197fb` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `4e5d67df358cf698134ad71ec317892c25caa9ed` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `4e9b01fd59b6621c4e02e5ab44dc93ce52c8534d` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `5fea83dcaedf49d0b6d866595de7d3c256256d2f` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `e2fd418a964206c374586740025bade1d5493a07` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `1ee929e4b9b1ec9c5aba45363f9322c845d948a6` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `15557e779637f5de073a5c757bcdac2f1f1a009f` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `a10ff1255cbaf2ec61dc9dcbc647cbc5bf5d3535` | Applied + Overridden | 网关调度与流式生命周期 | 取消归因同时检查原始请求和分离后的上游上下文；排空结果保留图片、推理等级和失败归因。 |
| `43569bb44c3eea843cec5fcacf0688961cd97f6b` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `163eb7ffae9c1327735fc5cadb108eda67d3ecf2` | Applied | 监控与日志 | 监控排名开关、探测角色与系统日志保留边界；保留本地配额隐藏及其他功能开关。 |
| `95acbf1f031a291335c60515ee368014139da113` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `0d3dbae710f7106e4b79d970e3a2f495850d88b2` | Applied | 监控与日志 | 监控排名开关、探测角色与系统日志保留边界；保留本地配额隐藏及其他功能开关。 |
| `52f7bcaedddd5294d88eb76d7964e3c2fd8a12c2` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `6d339ec93b35fb1e7b51388519c415c0ed068d54` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `6f0d0ababc64f79f06b6a532b1f0d041b1716d94` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `01bd9b71a26fbb28ef11840b3a7f1a3e3589ef1c` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `29646ba6b4e6b3355ca59cb17141b6ac3e92f206` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `e835f7981981a9be8a09bf072fcc7118655ad685` | Applied | 代理与日期校验 | 代理到期回退、有向共享备份关系与日期校验。 |
| `7137fae2a63d20a95f75f0c6ca8eb89b3feab1b4` | Applied | 代理与日期校验 | 代理到期回退、有向共享备份关系与日期校验。 |
| `c54897a59dfe3faa819f2a33117aa423353e2a1e` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `1859519573deed0a60bd99cc18d10216377aafa8` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `f9d78267112d7a4637643f310ac9093f13b554ef` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `81fd8530056c6480df8e69770a85d8f411340290` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `9dc4c40bff675cadd95e78a106c5325472ba3f8a` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `f5e4da5e72acd4fb026e0e97e7dfc61eae29735c` | Applied | 网关调度与流式生命周期 | 调度、逐轮槽位、取消归因、连接保活和流式修复；保留首语义输出、逐轮结算与插件已发送禁止重放。 |
| `233f3dffe64b0748ea7d610ccca4bdb44544051d` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `c8deeb0b0571fb22f9c1b17b2481800633ece8d0` | Applied | 协议与媒体兼容 | Claude、Astra、Ollama、Grok、Antigravity 与 Image 2.5 兼容；保留本地模拟、采样过滤、图片策略和媒体结果校验。 |
| `d9efbf8229c955e252138d10a279725f6a691fbb` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `76efc52fb6ca8a88b3eb1c7a643426a3e210c60c` | Applied | 页面认证与使用记录 | 订阅用量跳转、密钥分页、认证加载、注册可见性、支付安全 Markdown 与自定义页拖动；保留本地列设置和认证容错。 |
| `775348b402b8612203310fec6456c79ce3503837` | Applied | 合并祖先关系 | 保留完整合并祖先关系及对应功能变化。 |
| `570106ac17cecfbca95a460bd5047d43e8c0aafc` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `270eac6973049fe1b50eb75560a74a029e82884c` | Applied | 构建部署与发布元数据 | i18n 门禁、Redis 依赖、备份锁、插件 ZIP 关闭、部署脚本和资料更新；保留本地版本及双 Compose 约束。 |
| `7ccc8a6f5047a819ad5e57f0feaae4bd6f65840b` | Applied + Overridden | 协议与媒体兼容 | Image 2.5 与主控模型覆盖纳入；保留本地媒体结果校验、统计和双 Compose 约束。 |
| `5de5e2bed035d43591a2e10e51f420ef6a84eb98` | Applied + Overridden | 合并祖先关系 | 保留完整合并祖先关系；本次引入功能中的本地覆盖继承上述逐提交策略。 |
| `98d86915becae9fe9491a91ffc6defd5235c8d2b` | Applied + Overridden | 构建部署与发布元数据 | 保留本地 VERSION 0.1.231，不采用上游 0.2.4。 |

### 验证与重测

- Windows：Go 1.27.0、Node 20.20.2、pnpm 9.15.9、golangci-lint 2.13.0。22 项相同基线命令前后最终退出码均为 0。前端基线 275 文件/2015 用例，同步后 288 文件/2129 用例；Canvas 前后均 34 用例。
- Linux：本地 WSL Ubuntu 24.04/Docker 29.4.1、隔离 PostgreSQL 18.1 与 Redis 8.4；真实执行 integration 标签、全部迁移和仓库/API 用例，不是仅编译。前后仅保留同一权限基线失败 `TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool`：Windows 挂载将期望 0600 报为 0777；该未变单项在工作目录临时 POSIX tmpfs 中补测退出 0，挂载已卸载。
- 带超时的独立数据库/缓存初始化与应用 /health 前后均成功；不使用现有本地业务容器，脚本仅回收自己标记的临时容器和进程。
- 首轮新增失败已定位并修复：新构造签名、删除方法引用、认证快照版本、路由链源码断言、MiniMax 枚举、Pinia 测试依赖、本地精确模型规范化、DeepSeek 请求别名渠道定价夹具、无渠道账号成本断言、WS 原始取消上下文和异步结算回调时序。未删业务断言或关闭门禁。
- 环境失败如实保留：前端嵌套 pnpm 误用系统新版后，固定 pnpm 9 并 frozen/offline 重建依赖后通过，锁文件未变化；备份 mock 测试补足 Git Bash 的 sh PATH；验证码 SDK 需要 NO_PROXY 精确到端口，夹具显式隔离代理并保持端口占用；首次健康构建的 WSL Git ownership 仅通过进程级 safe.directory 修复；同步后首次 Linux 依赖获取/编译运行中断（143）并从本地校验缓存重跑。
- ZIP close 和测试 type assertion 的 errcheck 失败已修复；多次重测均列下表，失败历史未抹去。

<details>
<summary>全部实际验证命令、时间和退出码</summary>

| 编号 | 阶段/组/检查 | 开始时间 | 实际命令 | 退出码 |
| --- | --- | --- | --- | --- |
| V001 | 同步前/backend/test-default | 2026-09-10T23:32:29.6307159+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 0 |
| V002 | 同步前/frontend/lint | 2026-09-10T23:32:29.9447796+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run lint:check` | 0 |
| V003 | 同步前/canvas/format | 2026-09-10T23:32:30.2839262+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/prettier/bin/prettier.cjs --check .` | 0 |
| V004 | 同步前/static/docker-compose-security-test.sh | 2026-09-10T23:32:30.5814436+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\docker-compose-security-test.sh` | 0 |
| V005 | 同步前/static/docker-compose-gateway-env-test.sh | 2026-09-10T23:32:31.3104696+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\docker-compose-gateway-env-test.sh` | 0 |
| V006 | 同步前/canvas/typecheck | 2026-09-10T23:32:41.8675871+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/typescript/bin/tsc --noEmit` | 0 |
| V007 | 同步前/canvas/test | 2026-09-10T23:33:07.0577672+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/vitest/vitest.mjs run --maxWorkers=4 --minWorkers=1` | 0 |
| V008 | 同步前/static/docker-runtime-resources-test.sh | 2026-09-10T23:33:10.1603887+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\docker-runtime-resources-test.sh` | 0 |
| V009 | 同步前/static/remote-deploy-test.sh | 2026-09-10T23:33:10.6698371+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\remote-deploy-test.sh` | 0 |
| V010 | 同步前/extra/golangci-lint | 2026-09-10T23:33:21.0805681+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\golangci-lint.exe run ./... --timeout=30m` | 0 |
| V011 | 同步前/static/caddy-cache | 2026-09-10T23:33:27.2462329+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\test-caddyfile-cache.sh` | 0 |
| V012 | 同步前/static/apple-syntax | 2026-09-10T23:33:28.2602353+08:00 | `C:/Program Files/Git/bin/bash.exe -n D:\project\sub2api\deploy\apple-container.sh` | 0 |
| V013 | 同步前/static/diff-check | 2026-09-10T23:33:28.4050410+08:00 | `git diff --check` | 0 |
| V014 | 同步前/frontend/typecheck | 2026-09-10T23:34:50.6659382+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 0 |
| V015 | 同步前/frontend/test | 2026-09-10T23:35:56.5423376+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` | 0 |
| V016 | 同步前/generate/ent-check | 2026-09-10T23:36:30.4641916+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260910-98d86915b\check-ent.mjs before` | 0 |
| V017 | 同步前/generate/wire | 2026-09-10T23:36:53.4489158+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe generate ./cmd/server` | 0 |
| V018 | 同步前/backend/test-unit | 2026-09-10T23:36:59.2624056+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 0 |
| V019 | 同步前/extra/integration-compile-only | 2026-09-10T23:37:45.6880344+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=integration -exec D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\artifacts\compile-only.exe ./... -count=1` | 0 |
| V020 | 同步前/frontend/build | 2026-09-10T23:40:17.9568966+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V021 | 同步前/integration-linux/ | 2026-09-10T23:40:22+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 1 |
| V022 | 同步前/backend/build | 2026-09-10T23:41:04.4786663+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V023 | 同步前/canvas-build/build | 2026-09-10T23:42:57.0496372+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V024 | 同步前/smoke-linux/ | 2026-09-10T23:51:19+08:00 | `smoke.sh before` | 1 |
| V025 | 同步前/smoke-linux/ | 2026-09-10T23:53:50+08:00 | `smoke.sh before` | 0 |
| V026 | 同步后/frontend/typecheck | 2026-09-10T23:59:47.4611371+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 0 |
| V027 | 同步后/generate/ent-check | 2026-09-10T23:59:47.8139427+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260910-98d86915b\check-ent.mjs after` | 0 |
| V028 | 同步后/backend/build | 2026-09-11T00:00:56.2253155+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 1 |
| V029 | 同步后/frontend/lint | 2026-09-11T00:01:43.4167712+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run lint:check` | 0 |
| V030 | 同步后/generate/wire | 2026-09-11T00:01:43.8194593+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe generate ./cmd/server` | 0 |
| V031 | 同步后/frontend/typecheck | 2026-09-11T00:02:27.1153357+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 0 |
| V032 | 同步后/frontend/test | 2026-09-11T00:02:55.0458600+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` | 1 |
| V033 | 同步后/frontend/build | 2026-09-11T00:05:37.4048208+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 1 |
| V034 | 同步后/backend/test-default | 2026-09-11T00:07:05.4933801+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 1 |
| V035 | 同步后/frontend/install | 2026-09-11T00:08:47.1703437+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs install --frozen-lockfile --offline --ignore-scripts` | 0 |
| V036 | 同步后/backend/test-unit | 2026-09-11T00:10:18.8645738+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 1 |
| V037 | 同步后/backend/build | 2026-09-11T00:11:21.0099177+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V038 | 同步后/frontend/lint | 2026-09-11T00:12:06.6348384+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run lint:check` | 0 |
| V039 | 同步后/frontend/typecheck | 2026-09-11T00:13:13.5341557+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 0 |
| V040 | 同步后/frontend/test | 2026-09-11T00:13:53.4170620+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` | 0 |
| V041 | 同步后/frontend/build | 2026-09-11T00:17:12.2856238+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V042 | 同步后/backend/test-default | 2026-09-11T00:17:51.9506042+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 1 |
| V043 | 同步后/canvas/format | 2026-09-11T00:17:52.7540297+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/prettier/bin/prettier.cjs --check .` | 0 |
| V044 | 同步后/static/docker-compose-security-test.sh | 2026-09-11T00:17:53.1203631+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\docker-compose-security-test.sh` | 0 |
| V045 | 同步后/static/docker-compose-gateway-env-test.sh | 2026-09-11T00:17:53.7949682+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\docker-compose-gateway-env-test.sh` | 0 |
| V046 | 同步后/canvas/typecheck | 2026-09-11T00:18:02.9201715+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/typescript/bin/tsc --noEmit` | 0 |
| V047 | 同步后/integration-linux/ | 2026-09-11T00:18:08+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 143 |
| V048 | 同步后/canvas/test | 2026-09-11T00:18:12.7223806+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/vitest/vitest.mjs run --maxWorkers=4 --minWorkers=1` | 0 |
| V049 | 同步后/static/docker-runtime-resources-test.sh | 2026-09-11T00:18:45.8992425+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\docker-runtime-resources-test.sh` | 0 |
| V050 | 同步后/static/remote-deploy-test.sh | 2026-09-11T00:18:46.5491102+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\tests\remote-deploy-test.sh` | 0 |
| V051 | 同步后/static/caddy-cache | 2026-09-11T00:19:04.0352462+08:00 | `C:/Program Files/Git/bin/bash.exe D:\project\sub2api\deploy\test-caddyfile-cache.sh` | 0 |
| V052 | 同步后/static/apple-syntax | 2026-09-11T00:19:05.3969386+08:00 | `C:/Program Files/Git/bin/bash.exe -n D:\project\sub2api\deploy\apple-container.sh` | 0 |
| V053 | 同步后/static/diff-check | 2026-09-11T00:19:05.5704776+08:00 | `git diff --check` | 0 |
| V054 | 同步后/extra/golangci-lint | 2026-09-11T00:21:15.6368921+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\golangci-lint.exe run ./... --timeout=30m` | 1 |
| V055 | 同步后/canvas-build/build | 2026-09-11T00:21:15.9875648+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V056 | 同步后/backend/test-unit | 2026-09-11T00:21:50.7996042+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 1 |
| V057 | 同步后/extra/integration-compile-only | 2026-09-11T00:24:06.2846780+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=integration -exec D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\artifacts\compile-only.exe ./... -count=1` | 0 |
| V058 | 同步后/backend/regressions | 2026-09-11T00:26:15.0372313+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./internal/service ./internal/handler ./internal/repository -run Test(ModelMarketplaceModelsForGroup_Allowlist\|AliyunCaptchaVerifier_TransportError\|OpenAIGatewayServiceRecordUsage_DeepSeekAccountStatsUsesRequestPricingAtAndUpstreamModel\|ForwardOpenAIWSV2_ClientCancellationDrainsWithoutSyntheticFailure\|PassthroughIngressFollowUpCallsBeforeTurnAfterBeforeRequest\|AccountRepositoryCleansBindingsOnMembershipChangeAndDelete\|APIKeyAuthSnapshotProfitControlRoundtrip\|GroupModelAllowlistAllows)$ -count=5 -timeout=5m` | 1 |
| V059 | 同步后/backend/build | 2026-09-11T00:26:50.2514825+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V060 | 同步后/posix-spool-linux/ | 2026-09-11T00:27:10+08:00 | `TMPDIR=posix-tmp go test -tags=integration ./internal/service -run ^TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool$ -count=1 -timeout=5m` | 0 |
| V061 | 同步后/integration-linux/ | 2026-09-11T00:27:11+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 1 |
| V062 | 同步后/backend/regressions | 2026-09-11T00:28:32.5940549+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./internal/service ./internal/handler ./internal/repository -run Test(ModelMarketplaceModelsForGroup_Allowlist\|AliyunCaptchaVerifier_TransportError\|OpenAIGatewayServiceRecordUsage_DeepSeekAccountStatsUsesRequestPricingAtAndUpstreamModel\|ForwardOpenAIWSV2_ClientCancellationDrainsWithoutSyntheticFailure\|PassthroughIngressFollowUpCallsBeforeTurnAfterBeforeRequest\|AccountRepositoryCleansBindingsOnMembershipChangeAndDelete\|APIKeyAuthSnapshotProfitControlRoundtrip\|GroupModelAllowlistAllows)$ -count=5 -timeout=5m` | 1 |
| V063 | 同步后/backend/test-default | 2026-09-11T00:31:33.8297288+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 0 |
| V064 | 同步后/extra/golangci-lint | 2026-09-11T00:31:34.2206130+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\golangci-lint.exe run ./... --timeout=30m` | 0 |
| V065 | 同步后/smoke-linux/ | 2026-09-11T00:31:34+08:00 | `smoke.sh after` | 0 |
| V066 | 同步后/extra/integration-compile-only | 2026-09-11T00:34:42.9495121+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=integration -exec D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\artifacts\compile-only.exe ./... -count=1` | 0 |
| V067 | 同步后/backend/test-unit | 2026-09-11T00:35:29.3843768+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 1 |
| V068 | 同步后/backend/build | 2026-09-11T00:36:25.6042596+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V069 | 同步后/backend/regressions | 2026-09-11T00:42:32.1540383+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./internal/service ./internal/handler ./internal/repository -run Test(ModelMarketplaceModelsForGroup_Allowlist\|AliyunCaptchaVerifier_TransportError\|OpenAIGatewayServiceRecordUsage_DeepSeekAccountStatsUsesRequestPricingAtAndUpstreamModel\|ForwardOpenAIWSV2_ClientCancellationDrainsWithoutSyntheticFailure\|PassthroughIngressFollowUpCallsBeforeTurnAfterBeforeRequest\|AccountRepositoryCleansBindingsOnMembershipChangeAndDelete\|APIKeyAuthSnapshotProfitControlRoundtrip\|GroupModelAllowlistAllows)$ -count=5 -timeout=5m` | 0 |
| V070 | 同步后/backend/test-unit | 2026-09-11T00:42:32.1825679+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 0 |
| V071 | 同步后/backend/resource-test | 2026-09-11T00:44:52.4049820+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./internal/pkg/openai -count=1 -timeout=5m` | 0 |
| V072 | 同步后/backend/build | 2026-09-11T00:44:54.6038243+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |

</details>

### 未验证与残余风险

- 未访问真实模型/Token/插件、支付/消息/回调或生产数据库；没有真实浏览器 E2E、完整 race、govulncheck/安全漏洞数据库复核和 Apple container/plutil 运行验证。真实上游兼容性及专有平台能力仍为未验证。
- 本地服务只做无外部副作用的隔离健康测试，不代表生产功能组合均通过。后续部署仍需独立授权并遵守生产检查门禁。
- 前端与 Canvas 构建仍有大 chunk 提示，属于基线已有构建警告，不在本次同步中顺便拆包。
- 全部原始日志、工具和缓存保留在仓库忽略目录 `output/upstream-sync-20260910-98d86915b` 及前次缓存目录，不纳入提交，不删除其他本地资料。
- 最终完成还要求记录提交、原 main 未变化、仅 ff-only 更新本地 main 和干净工作树；最终 SHA 在交付总结报告，避免文档自引用。


### 本地提交与最终门禁补记

- 时间：2026-09-11T00:54:59+08:00。合并代码提交：`cf1c01913d01555412224e86f07350cc43da2894`，两个父提交依次为 `d3c44a97b0a25fddd7fae3cdeb537682f365ebf5` 和 `98d86915becae9fe9491a91ffc6defd5235c8d2b`。全部 181 个上游 SHA 均作为祖先纳入此 merge；无需 cherry-pick 或 squash 映射。
- 相对同步前变更 507 个文件；Git 原始统计：`507 files changed, 21049 insertions(+), 2793 deletions(-)`。完整清单可用 `git diff --name-status d3c44a97b0a25fddd7fae3cdeb537682f365ebf5 cf1c01913d01555412224e86f07350cc43da2894` 复核。
- 24 个索引冲突均已清除；55 个稳定编号全部保留，无未知变更、意外删除或秘密检查命中。代码提交后工作树干净，main 仍位于同步前 SHA。
- 此最终记录提交只更新两份台账，不改变已验证代码。完成后仅允许 `git switch main` 和 `git merge --ff-only sync/upstream-20260910-98d86915b`。本记录自身 SHA 与最终 main SHA 仅在最终总结报告。
- 本地代码集成与可用验证通过；全部失败重测、权限基线和未验证范围见前文。未推送、未创建 PR、未连接服务器或部署。
- 最终复核：`node output/upstream-sync-20260910-98d86915b/final-audit.mjs check`、`node output/upstream-sync-20260910-98d86915b/write-records.mjs preview`、`git diff --cached --check`、`git diff --check` 均退出 0；额外断言确认同步历史只追加、181 条完整 SHA 处置与 Git 固定范围一一对应、备份分支正确、合并双亲正确。台账补记后再次审计通过。
- 本地清理核验：本次标签下没有遗留容器，健康检查应用进程不存在，POSIX 临时挂载已卸载。WSL 默认用户首次查询 Docker socket 无权限，改用同一本机 WSL 的 root 只读查询成功；没有连接任何远程服务器。

### 2026-09-11 推送与 CI 修复补记

- 时间：2026-09-11T01:16:14+08:00；用户在本地同步交付后明确授权提交远端、CI 通过后合入主线。已推送 `sync/upstream-20260910-98d86915b` 并创建 `Saviour2411/sub2api` PR #21，目标 `main`；操作前远端 main 仍为 `d3c44a97b0a25fddd7fae3cdeb537682f365ebf5`。
- 首轮候选 `40df432d3e6753fa027ac2340dd1b31b62d9f297` 的 push/PR Security Scan 均在 Canvas 审计失败：`js-yaml 4.3.1` 命中 `GHSA-2883-xcg3-v3hh`；后端 govulncheck、前端构建、lint、macOS shell 检查已通过，后端测试当时仍在执行。本段不把未完成的检查记为成功。
- 新修复只将 Canvas 的受影响 js-yaml 4.x 范围定向覆盖到 `4.3.2`，更新包完整性摘要与 cosmiconfig 解析；YAML 结构化前后比较确认其他包与版本不变。未改 CI、安全例外和任何业务默认值，`CUST-PROD-007` 清单同步更新。上游固定范围和完整集成 SHA 不变。
- 本地实际验证采用既有 Node 20.20.2 与 pnpm 9.15.9：

| 实际命令（工作目录） | 退出码 | 结果 |
| --- | --- | --- |
| `pnpm install --lockfile-only --ignore-scripts --reporter=append-only`（canvas） | 0 | 定向重新解析锁文件；沿用原有 peer 提示 |
| `pnpm install --frozen-lockfile --ignore-scripts --reporter=append-only`（canvas） | 0 | 安装补丁，锁文件冻结验证 |
| `node node_modules/prettier/bin/prettier.cjs --write package.json pnpm-lock.yaml`（canvas） | 0 | 仅恢复两个变更文件的既有格式 |
| `validate.ps1 -Phase after -Suite canvas`（仓库根目录，脚本位于本次 output 目录） | 0 | format、typecheck、34 项 Vitest 全部通过 |
| `validate.ps1 -Phase after -Suite canvas-build`（仓库根目录，脚本位于本次 output 目录） | 0 | 类型检查和生产构建通过；原有大 chunk 提示保留 |
| `pnpm audit --prod --audit-level=high --json`（canvas） | 1 | JSON：高危 0、严重 0、中危 6；js-yaml 告警已消失，原始退出码保留 |
| `python3 tools/check_pnpm_audit_exceptions.py --audit output/upstream-sync-20260910-98d86915b/logs/ci-followup-canvas-audit.json --exceptions .github/audit-exceptions.yml`（本机 WSL，仓库根目录） | 0 | 与 CI 相同的审计门禁通过，未新增例外 |
| `git diff --check`（仓库根目录） | 0 | 无空白错误 |

- 推送本次修复后必须等待最新 PR head 对应 CI 和 Security Scan 成功，再用 merge commit 保留上游祖先关系合入 main；禁止 squash/rebase 或在失败状态绕过检查。最终 PR/合并 SHA 在交付总结记录，避免自引用。
- 本次不推送任何版本标签、不触发 Release、不部署、不连接远程服务器。剩余 6 项中危、真实业务服务和完整 race 等未验证范围仍需后续关注。

## 2026-09-15 完整本地同步 bdb42e22f

### 固定参数与状态

- 记录时间：2026-09-15T03:27:42.2860993+08:00；状态：固定范围代码已完整集成，可用验证完成，相对同步前无未处理新增失败。最后记录提交后按下述门禁仅快进本地 main；最终目标 SHA 在交付总结报告。
- 本地目标：`main`；LOCAL_PRE_SYNC_SHA: `098f3d9e5a14278ada575daae0fd871b50898817`。同步前工作树干净，无未完成 Git 操作；跟踪 `origin/main` 为 `bc6946257c0765d14ae7a2883e3552de3141ba2f`，ahead 1 / behind 0 / 无分叉。
- 上游：`https://github.com/Wei-Shaw/sub2api`，远端 HEAD 确认默认分支 `main`；获取后固定目标，没有扩大到后续更新。
- UPSTREAM_OLD_SHA: `98d86915becae9fe9491a91ffc6defd5235c8d2b`。
- UPSTREAM_NEW_SHA: `bdb42e22f81fcb633ff0a060961211dd2bcb515b`。
- ACTUAL_MERGE_BASE: `98d86915becae9fe9491a91ffc6defd5235c8d2b`，等于旧基线；旧基线属于双方祖先，实际 merge 范围与记录范围相同。
- LAST_FULLY_INTEGRATED_UPSTREAM_SHA: `bdb42e22f81fcb633ff0a060961211dd2bcb515b`。
- 策略：在备份后执行 `git merge --no-ff --no-commit bdb42e22f81fcb633ff0a060961211dd2bcb515b`，逐块解决已批准的 24 个冲突并提交，未使用整文件 ours/theirs。
- 备份分支：`backup/pre-upstream-sync-20260915-014934-098f3d9e5`。同步分支：`sync/upstream-20260915-bdb42e22f`。
- M1（本次代码合并）：`6454c4f68754a0e79ae6cce549ac616999d0ab57`，两个父提交为 `098f3d9e5a14278ada575daae0fd871b50898817` 和 `bdb42e22f81fcb633ff0a060961211dd2bcb515b`。下表全部 SHA 作为 M1 的祖先纳入，无需 cherry-pick/squash 映射。
- 固定范围 141 个提交：84 个普通、57 个 merge。Applied 共 141，其中 Applied + Overridden 16；纯 Applied 125；Already Applied / Skipped / Deferred / Conflict 均 0。Overridden 是 Applied 子集，不重复计数；基线图与 patch-id 未发现完整等价重复提交。
- 上游范围 357 文件（51 新增、306 修改、0 删除），144 与本地覆盖重叠；本地 M1 相对同步前 365 文件，`365 files changed, 15551 insertions(+), 1198 deletions(-)`。最后同步历史提交仅追加本文件，最终统计另含此文件。
- 版本保持 `0.1.232`。未 push、未创建 PR、未触发发布或部署，未连接远程服务器、操作生产数据库或修改实例配置。

### 冲突与本地边界

- 用户已批准完整 merge 和集中冲突处理方案。24 处文件冲突与预演一致，无新业务取舍；处理范围如下。
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/setting_handler.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/account_test_service_openai_image_test.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/gateway_claude_oauth_body.go`
- `backend/internal/service/gateway_count_tokens.go`
- `backend/internal/service/gateway_forward.go`
- `backend/internal/service/gemini_messages_compat_service.go`
- `backend/internal/service/openai_gateway_forward.go`
- `backend/internal/service/openai_gateway_usage.go`
- `backend/internal/service/openai_images.go`
- `backend/internal/service/openai_images_responses.go`
- `backend/internal/service/setting_public.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- `frontend/src/components/user/dashboard/UserDashboardStats.vue`
- `frontend/src/router/__tests__/feature-access.spec.ts`
- `frontend/src/router/index.ts`
- `frontend/src/stores/app.ts`
- `frontend/src/types/index.ts`
- `frontend/src/views/user/__tests__/RedeemView.spec.ts`

- WS 接入沿用本地 prepareFirstWSTurnState、逐轮请求模型/映射/定价/Cyber/hash/停调度；组合上游原始请求作用域、常驻读循环、先关闭帧再取消和按账号池唤醒。新连接系数默认 5，未改为请求入口限流。
- Claude 保留本地可选缓存清洗与 system 文本/模型保留；Gateway、mimicry 与 count_tokens 参数统一，四块缓存上限独立生效。Gemini 带内错误登记与首 Token attempt/finish 组合。
- 原生 Codex Images 与既有 Responses 图片路径共存；保留交付/断连/部分 usage/端点元数据、RequestSent 禁重放和普通 URL 内联/下载公网防护。账号测试先识别 SSE 错误，不删除旧模型或错误断言。
- OpenCode 三协议派发和上游错误处理完整接入；本地测试模型稳定选择、显式模型优先、一次映射、自定义提示词、预览请求接口不变。会话仅驻留清洗后的短 ID，2 MiB 请求体不被会话快照留住。
- 图片缓存计价、DeepSeek 新价与历史 PricingAt 接入；保留严格请求模型计费、缺价拒绝、分组/渠道价格优先、Free Fast 双成本、Fable 无默认三倍、Grok 严格超过 200K 和按用户串行扣费。
- 新配额卡与本地 CountUp/PulseDot/动效组合；订阅默认启用且与 Canvas、市场、签到、公开余额路由共存。兑换刷新失败成功态与签到记录测试同时保留。
- 两份生产 Compose 仅同步 compact 默认 gpt-5.5，并保持字节一致。保留 bind mount/回环地址/HTTP upstream、生产资源变量和日志入口、5 秒 usage task、3600 秒持久化响应头超时约束以及 300 秒流间隔示例；不读取真实 .env。
- 两份新增 238 迁移按完整文件名执行；旧 SQL（含本地 237）未改号、未改写。隔离旧库升级确认全 NULL quota 行清理而零限额/有效限额保留，MiniMax/OpenCode 四个约束与本地余额、授信、链接数据正确；两份迁移再次执行仍通过。
- 57 个本地能力编号全部保留，对应 21 项清单已在 M1 同时更新 `docs/custom-development-history.md`；纯上游功能未另建 CUST 编号。未覆盖真实业务不声称全部正常。

### 逐提交处置

| 上游完整 SHA | 状态 | 原因与最终内容 | 本地映射 |
| --- | --- | --- | --- |
| `1c0932e17302b2ba9b67f6bcc48b913abe552674` | Applied | 按可见目录查询单个模型，复用固定账号与分组准入范围。 按固定目标的最终内容合入。 | M1 |
| `799e9938fb83dbc3f301757640530369b766df1e` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `a4edda36d53a07a363f467c1d1922eba0dc58ed9` | Applied + Overridden | 调整 Claude system 缓存处理。 本地适配：保留本地可选 system 缓存清洗、原文与模型保留选项；四块缓存上限独立执行。 | M1 |
| `4adcef4ff4185e802ec4c1d10a016de807c5dd80` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `21323c31f81d361b3a419181f82c896bd6b81b0b` | Applied | 渠道监控拼接上游 base URL 时保留路径。 按固定目标的最终内容合入。 | M1 |
| `e71d291b39de278b783ac6b31a7acae1209c0c18` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `de76baaa0d24dea5ffff78d57fa10e83ba9f7a0b` | Applied | 调度缓存保存 Anthropic 阈值元数据。 按固定目标的最终内容合入。 | M1 |
| `685e97a3a626fc1c0a253def0c7e466b542e13d2` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `a0babc93dd17becdb66507e92ede18f5bde38d9c` | Applied | 心跳启动解析完整 envelope。 按固定目标的最终内容合入。 | M1 |
| `43f9383d40da666588efae1aceac90b41d89483c` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `f180fbb884b15e2b0483672b7d5a7ede0c31feb5` | Applied | 渠道监控聚合桶使用 UTC 锚点。 按固定目标的最终内容合入。 | M1 |
| `87e01596b705ac25cb3ca2072c96d40697635381` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `8ed48e27a6bc2ec6bfdde30669123c23ae9f7338` | Applied | 安装检查不再硬编码 PostgreSQL 连接参数。 按固定目标的最终内容合入。 | M1 |
| `264cbbec269b17b2e25bd134d4bad3d4847edcf0` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `4e5632c3e32f9a8a5150c44e8a7f46efe7fc2688` | Applied | 解析 Codex 身份前验证 User-Agent。 按固定目标的最终内容合入。 | M1 |
| `2493b0dc3112fc6d03e5429d1615542ad60513a3` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `567ea21dc843a28d5ab603bdec2610e1ba549516` | Applied | 管理员支持删除选中的用户。 按固定目标的最终内容合入。 | M1 |
| `324a47e2b7fa3ea5e4af79e664a987003ae810b2` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `c76db386c066e35ec147645ec05dc4aa9dabaef2` | Applied | 订阅分配搜索排除已删除用户。 按固定目标的最终内容合入。 | M1 |
| `0be30886a5a60afe25a0c5e4a17d33326eeb6024` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `1d4e0436e49749175435fa431b935e738fa6288a` | Applied | 自定义页面支持隐藏打开按钮。 按固定目标的最终内容合入。 | M1 |
| `f4dd88b001a719396bfc3667ee99c9e4716494e3` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `266ac9c11b39b181cc4225ab888a72a4ec8e1c60` | Applied | 错误详情优先展示时间与响应。 按固定目标的最终内容合入。 | M1 |
| `0aac71c6eebe2b89ec6b4e132a72a1ba60031df3` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `fd880839f624366e7d648d7a8de62f6ef69e65a8` | Applied | 费用详情保留八位小数。 按固定目标的最终内容合入。 | M1 |
| `8efe2fd8cf111fa24293be6ffd1076d22192e8f9` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `26798773cb26ad95bd537877efa2cd94aa5cb20c` | Applied | Apple container 安装支持网页更新。 按固定目标的最终内容合入。 | M1 |
| `501cc19d0ee71d4fd63020f2230ee71403b44a5f` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `b9d0728687ba815b44f9297c87285cc163d5a335` | Applied | 前端订阅兑换时长上限对齐后端。 按固定目标的最终内容合入。 | M1 |
| `67d3a896bdd632eeee822fc7496cac74affc12ea` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `58e6e6b325deb09878638c4261539782c454bd82` | Applied | 异步风控处理计数的文案明确统计口径。 按固定目标的最终内容合入。 | M1 |
| `0116c5a1e126c6578ba8a3d47018c1cafb4e7afa` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `5af7e7438a3993f9db7ac359ffcf973cbb3cfe46` | Applied | 安装测试检查延迟关闭数据库的错误。 按固定目标的最终内容合入。 | M1 |
| `bae37a00fb676c2586a85985e52c597ac8b9e063` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `a5b07b296011bc376110a4b914ceaf283424203b` | Applied | WS 池常驻读取循环应答上游 ping。 按固定目标的最终内容合入。 | M1 |
| `bed1e3c3624672ccfee9981e431284687de24271` | Applied | 重试强制使用新连接并放宽轮次预检超时。 按固定目标的最终内容合入。 | M1 |
| `8f9a9a255b0b392fcb5659aa4906eef4d58067b5` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `06b61ba96f1dffdad0e2387669f1df6cf901f7be` | Applied | 按 Codex 线程标识派生 WS 执行作用域。 按固定目标的最终内容合入。 | M1 |
| `9e7c2d713c8510fe4a2147a7db424747092a8aed` | Applied | 会话抢占改用执行作用域键。 按固定目标的最终内容合入。 | M1 |
| `cd1ee1d1ad735888ed63ae72b21ecbe9105ec8a8` | Applied | WS 接入会话状态改用执行作用域键。 按固定目标的最终内容合入。 | M1 |
| `e4c369bd56305c9984c01cf9eb2bfb6f1cb8da1a` | Applied | HTTP 经 WS 池转发时按执行作用域查询状态。 按固定目标的最终内容合入。 | M1 |
| `d0ca057ca41add88bc9e5a496493da969816125f` | Applied + Overridden | 抢占先发送带原因的关闭帧，再取消旧上下文。 本地适配：入口组合本地逐轮准入、模型、定价、Cyber、hash 与 WithClient 注册；同线程测试保持旧请求在飞直到收到关闭帧，全部断言保留。 | M1 |
| `613722eee434c0a62e9397487b3713c16362e18d` | Applied | HTTP 执行作用域从原始请求计算。 按固定目标的最终内容合入。 | M1 |
| `4543ddc5c172b3d5f72d35de1c357a886b55526c` | Applied | 按 request_kind 隔离 WS 执行作用域。 按固定目标的最终内容合入。 | M1 |
| `3fb03cdac1d4878a271a3efc111ec8f0a489f1dd` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `b97a798ebcb2571be2f9e08917f4cf10fe322a27` | Applied | 释放放弃的 Grok 媒体槽并保留视频所有权。 按固定目标的最终内容合入。 | M1 |
| `14029e50aa04e5275de60726037eb9b29ab874b7` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `7e62731863c8f21ca32292e3923860e7f45221e9` | Applied | 减少大型 Responses 图片体的分配，保留本地安全释放时点。 按固定目标的最终内容合入。 | M1 |
| `88011a6a2e7bf221df7b1950c3c0e16a22028002` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `ab9bd9e8779ca2015d398d3d26513c0c6eb4384f` | Applied + Overridden | 接入 DeepSeek Flash 新价及按 PricingAt 的 Pro 转 Flash 切换。 本地适配：更新本地 DeepSeek 旧价测试，不覆盖分组/渠道自定义价、请求模型准入和历史 PricingAt。 | M1 |
| `fd300ab6f37ec901668e593eb6b6a1d8067a7d5c` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `e9f5add97ca9e576ebe2d38df488fd23c9044c9f` | Applied | TTFT 请求详情展示首 Token 延迟。 按固定目标的最终内容合入。 | M1 |
| `b112805ffa8594ab27821b9dcafb057d2c4bf40e` | Applied | 查询匹配测试格式对齐 gofmt。 按固定目标的最终内容合入。 | M1 |
| `7145484fde802a656e652e6f8e0e14f567437e95` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `27bcec76454f81cbb69aa92b6e1c30c424984888` | Applied | Antigravity 支持 Gemini 3.8 Flash。 按固定目标的最终内容合入。 | M1 |
| `8ed57b000ce68c407473d4d0fdc18fe9ff63c70b` | Applied | Antigravity 支持 Gemini 3.7 Flash。 按固定目标的最终内容合入。 | M1 |
| `29a36a10be0eee722c55f753150caff759fe8c66` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `6254aba5e931afbae943baaab5526d128d9d8aaa` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `95a04c8dff6a30227001b48f8dca6bd2ba4a6f99` | Applied | 补充 additional_tools 经 Chat 桥转发的回归测试。 按固定目标的最终内容合入。 | M1 |
| `df8eceba0c150436835f1b02b74b397df81205ac` | Applied | Chat 桥保留 agent_message 任务正文。 按固定目标的最终内容合入。 | M1 |
| `310f8b7fa27c44b12724a3a0f7d9858456379f6d` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `8f2cba0c20bbc2c58c21417e0cd7a60dafe21509` | Applied | 显示新版 ChatGPT 套餐名称。 按固定目标的最终内容合入。 | M1 |
| `623c32e3916aeb138224cab870b2fd4adcb4d82b` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `6feefb7aec553d250c2299b455538c00eb60f4ea` | Applied | 账号模型清单保留 Codex manifest 显示名。 按固定目标的最终内容合入。 | M1 |
| `642d20b8acc2d2dae977bed73a92cae9e1dff9cc` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `ab3b398b5f9a2800303dbdd8928e2da3f5174981` | Applied | 非法峰值倍率配置返回参数错误。 按固定目标的最终内容合入。 | M1 |
| `d3bd614ba7d89cb244a7e54b85c166f113ac65a6` | Applied | 峰值倍率校验测试补齐有效基础倍率。 按固定目标的最终内容合入。 | M1 |
| `75b7dd1e0b019a6782526afea4a9114e1b1ad20d` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `5032d06697930605aaaa5aa8e720c09aeeba112e` | Applied | 模型目录标记 DeepSeek vision 图片输入能力。 按固定目标的最终内容合入。 | M1 |
| `40cf3f50a6cb074361b3d6961b105a240c350b53` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `8c56eabcdc105e56546ef76c1343f04371e577e9` | Applied | 续费套餐列表允许滚动。 按固定目标的最终内容合入。 | M1 |
| `9c30951e4aca86c4590a361a06d599b29c06caad` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `4a4b50d807e594a069a69cdeec4385b22731e79a` | Applied + Overridden | 平台 quota 仅存已配置限额，仪表盘按用量与限额合并展示。 本地适配：组合本地 CountUp、PulseDot 与视觉组件；零限额保留、全 NULL 清理并以完整迁移名共存。 | M1 |
| `2dff7af0f915bbae0cb7871767574a82022e44f3` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `9d475f9edb85edbfa563ddc0802c4e160793ae30` | Applied + Overridden | 新增站点类型与订阅开关，统一用户端入口和文案。 本地适配：订阅缺省启用，与 Canvas、签到、模型市场及公开余额路由的本地规则共存。 | M1 |
| `dfa83fbe93b89c1dca239ee5581e4d6631175de0` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `242907854d831d2c8ccff50ff0ab897e42fba76e` | Applied + Overridden | 新增 OpenCode 平台及 Zen、GO 账号模式。 本地适配：复用本地确定性测试模型与预览请求接口；无映射使用 glm-5.3，既有平台默认值不变。 | M1 |
| `7c008bd8c1fa53b0302672919d6b46726b3eaa85` | Applied + Overridden | OpenCode 账号测试应用模型映射。 本地适配：先选择/映射再固定账号测试副本，保留自定义提示词并防止重复映射。 | M1 |
| `981279c996ada152427fd1a00f44530a8d22718a` | Applied + Overridden | OpenCode 三类协议均写入映射后的上游模型。 本地适配：三协议模型重写与本地单次映射、确定性预选共同执行。 | M1 |
| `efcc2252e59b4f7134f851652b030d3465802f5a` | Applied + Overridden | 完善 OpenCode 派发、计费、会话与 402 错误处理。 本地适配：只快照经过清洗的会话 ID，不驻留原始大请求；在本地释放点清理，保留严格请求模型定价。 | M1 |
| `22dffa1bb95b945a5296711bb4367034bdb20771` | Applied | OpenCode 布尔表达式符合 staticcheck。 按固定目标的最终内容合入。 | M1 |
| `cdb5cfaf6c8cb08612ef552a4458d8d0b5850184` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `c0d51193755c0141fefbbcf4f12e219010d95edd` | Applied + Overridden | OAuth 图片接入原生 Codex Images 转发。 本地适配：保留图片交付标志、客户端断连、部分用量及端点元数据；普通路径 URL 内联与下载防护不变。 | M1 |
| `eacff0a62988d9bbd63f2e844ae7ebd8a96e184c` | Applied + Overridden | 接入图片缓存定价及相关上游配置。 本地适配：接入图片缓存价，不引入与本地 Grok 严格大于 200K 边界冲突的 inclusive 阈值。 | M1 |
| `fa565eafb168e78fed4eab9c231c421c57e2d7d7` | Applied | 价格结构新增图片缓存输入字段。 按固定目标的最终内容合入。 | M1 |
| `e5e04de137a71877c494c499d864aa8c8590faf9` | Applied | 配置与模型测试对齐最终上游能力；本地默认模型不变。 按固定目标的最终内容合入。 | M1 |
| `5ee436d8a2ec561b5e00e40ef4dbe87a3b38db8c` | Applied | 补齐图片缓存兜底单价。 按固定目标的最终内容合入。 | M1 |
| `04c45e9dc2bd2959e9d1a3c1b5e072dff9dc6e2a` | Applied + Overridden | 处理原生图片路径与旧模型测试的兼容。 本地适配：不采用删除本地图片模型断言的处理，组合保留旧路径与 direct 路径覆盖。 | M1 |
| `1067e89fabfd271e03a7c41ab22a7c190cf87934` | Applied + Overridden | 完善图片路由的错误、响应与调用细节。 本地适配：保留直连流式交付/断连归因；账号测试先抽取 SSE 错误再按原生/Responses 路径解析。 | M1 |
| `23eef9ecc3a9dafe7c078653a980f49f609e1a35` | Applied + Overridden | 恢复上游图片模型回归覆盖。 本地适配：恢复上游覆盖的同时继续保留本地模型与错误归因断言。 | M1 |
| `4726bdd08b6201d426a80529b79be123a4008d20` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `44bc47a3e38269c178ac56f667fd152f621bda48` | Applied + Overridden | Gemini 原生转发登记 2xx 内的错误信号。 本地适配：新增错误登记参数与本地 Gemini 首 Token attempt 包装、finish 结算组合。 | M1 |
| `8ea4dc56f0292491af026ce486bd6ed53afc81d3` | Applied | 模型侧 finishReason 不登记为上游失败。 按固定目标的最终内容合入。 | M1 |
| `6206ce940ea9bca3708c8b005c219a7bd1d12a07` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `213de0797e774dec5f028a82ef4edfea3ff85739` | Applied | 避免重复 Accept-Encoding 请求头。 按固定目标的最终内容合入。 | M1 |
| `5948988aae5ce873b17a2446c27e11f84e54d4a6` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `781a02aea7867231727abfb827b71c312715297e` | Applied | 临时服务故障不清除登录会话。 按固定目标的最终内容合入。 | M1 |
| `ba57ea914b01701a8d40fb40793162622bc299de` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `51c5d03888b796bbc0a28ca014f5bfbc18eb4074` | Applied | 订阅分配搜索防抖前清除旧用户选择。 按固定目标的最终内容合入。 | M1 |
| `d2067668d37e9ea97fc852c5a74a1912c1ea59ac` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `c3b3072a6fae7ae6c686e05df78060c74f284a67` | Applied | EasyPay 上游类型允许包含句点。 按固定目标的最终内容合入。 | M1 |
| `e67ffda7aed038c1e678ee835d84cf94e9cc06a5` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `489968fd7a54ffac0939e934e5f0149130519ac7` | Applied + Overridden | compact 默认上游模型切换到 gpt-5.5。 本地适配：同步两份本地生产 Compose 和静态夹具；生产 bind mount、资源入口、300 秒示例与持久化超时约束保留。 | M1 |
| `f78c4b241e4f1fd4fba2129c62892fc34dec635b` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `bb0b50d8bc4e10adfe218a68ab567ee4983d4d11` | Applied | 兑换后刷新失败仍保留成功态，并保留本地签到展示测试。 按固定目标的最终内容合入。 | M1 |
| `4ff3e6dfb92cb3cb4e3ac2528e5f7f22d64d4449` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `29371b081960b1bcc38da45862a096e39d1d811e` | Applied | WS 每账号连接池系数默认值改为 5。 按固定目标的最终内容合入。 | M1 |
| `2a4f3d1a7108f5acf5cd80ccaa7bd83ed6361e82` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `7cf90c79d624eb4b9ccc8279e5facf628bc046e2` | Applied | 连接池变化唤醒该账号等待者重新选择连接。 按固定目标的最终内容合入。 | M1 |
| `3873215908847ead95fe86a7bafcca0adbf79cdf` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `d8326fccfce4ba011f2fc1e148a0181912a9dfd1` | Applied | Claude 会话中保留 output config beta。 按固定目标的最终内容合入。 | M1 |
| `eaa4083e23a94f99bb1442e5c70560b957e3d957` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `5968fd0ed971372714706e3b907157f432022a51` | Applied | 渠道监控允许 MiniMax。 按固定目标的最终内容合入。 | M1 |
| `749cd7c3546187c893513bc21af88c1c50bc2eb9` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `9eb120dd40509d17df17bb61ceef22554d9ca45b` | Applied | 隐私与账号检查使用 Firefox 指纹。 按固定目标的最终内容合入。 | M1 |
| `3a070ec1bf8ce953e228bc54ddc0dfe915f5e193` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `3645ce5f43c6f4c6b5732d05bed1f718fd442810` | Applied | 按身份移除已验证通知邮箱。 按固定目标的最终内容合入。 | M1 |
| `67845665d6eedbaf8503c22bfc49a0803f013cef` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `5bafffed510c1520bde784456119cc3659480574` | Applied | 代理筛选变化时重置分页。 按固定目标的最终内容合入。 | M1 |
| `99b93b298547b12ed303ec6e919e058037ba5cdc` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `429d6f04807e244d636a5955fb552e63d4618c50` | Applied | 跨页导出沿用当前用量筛选。 按固定目标的最终内容合入。 | M1 |
| `f0dd497780acff99b715acb2dfbd52474f0f5778` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `961d9e94f38896c38faf73210a9527d609dc1b5a` | Applied | 代理部分导入成功后刷新列表。 按固定目标的最终内容合入。 | M1 |
| `329641a86fc6c15a4efa31ba5f90eaf1ed848cfa` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `9b6a49b8e70ae65fff231a27aba2307946aaa965` | Applied | 个人资料展示规范化 API 错误。 按固定目标的最终内容合入。 | M1 |
| `f2b51e3b24e075cf81c294cab667441ffadad842` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `a65d476f25ddcd5ef961f18b80fc640cb7341ecd` | Applied | 密钥额度重置后同步状态。 按固定目标的最终内容合入。 | M1 |
| `0a378f3432c4177b5149dd5cf22d14e66d92e92e` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `8e7954438dcae79d5c35b5f910afdb0d5a9d19f8` | Applied | 强制 Fast 策略在省略 service_tier 时同样注入 priority；保留 Free Fast 双成本。 按固定目标的最终内容合入。 | M1 |
| `a59f0c7fac0e653e9135605068c100d75ed5dc2c` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `1035a4daf0cbcc19b5662670cd6be72fb57851db` | Applied | 运维 Token 请求统计扩展到所有平台。 按固定目标的最终内容合入。 | M1 |
| `66a10d4939e959172b49b040edebd8e3583b9057` | Applied | 安装模拟数据库清理检查错误。 按固定目标的最终内容合入。 | M1 |
| `c7ed614240e9f2b2ac14f0bc4a2f48dfa1a86e98` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `195a3313224a7a232370e7aa7a4ed79cf6ad49d7` | Applied | 模型目录保留最大上下文窗口。 按固定目标的最终内容合入。 | M1 |
| `9383e0b154348b224f64e66985f907f336b00c2e` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |
| `956f4672e482bcc42563049741a1b83ce9ad242e` | Applied | 新版 WS ctx_pool 容量计算复用动态开关和类型系数。 按固定目标的最终内容合入。 | M1 |
| `cf3aca6e3b76d7b9d9b680e8ae59a9268a67e6b7` | Applied | 按 OpenAI WS 模式显示链路提示。 按固定目标的最终内容合入。 | M1 |
| `5d5b395d0069ce752922ea65e730f7a2c52a421b` | Applied | 完善 OpenAI WS 模式说明。 按固定目标的最终内容合入。 | M1 |
| `db15ddf07fd70afe5991dc721ce2968d07bb2b97` | Applied | 区分 OpenAI WS 开关与连接模式。 按固定目标的最终内容合入。 | M1 |
| `76b3f3c7cf1d3fd302310b5ae0aaeaf38d3ed107` | Applied | 精简 OpenAI WS 模式提示。 按固定目标的最终内容合入。 | M1 |
| `bdb42e22f81fcb633ff0a060961211dd2bcb515b` | Applied | 保留合并祖先关系；功能处置见所含普通提交，后续本地覆盖不改变该提交已集成的事实。 | M1 |

### 验证环境与结果

- 命令均在本仓库或其子目录执行；正式验证脚本将临时配置、缓存、测试数据、构建产物和日志定向到 `output/upstream-sync-20260915-bdb42e22f`，复用仓库内 20260905/20260910 工具缓存。真实 .env、私钥、GitHub Token 和业务凭据不进入测试。一次手动编译未显式指定临时目录，不能保证该次遵循统一目录约束；随后全部验证改用隔离脚本重跑，未执行工作区外清理。
- Windows Go 1.27.0、Node 20.20.2、pnpm 9.15.9；WSL Ubuntu-24.04 的 Go 1.27.0 和本机 Docker。Linux 集成创建独立 PostgreSQL/Redis 容器；不复用任何现存业务数据库，不处理其他项目容器。
- Windows 套件入口：`./output/upstream-sync-20260915-bdb42e22f/validate.ps1 -Phase before|after -Suite <backend|frontend|canvas|canvas-build|extra|static|generate|security>`，额外筛选为 `-OnlyLabel cyber-flaky|sync-critical`。各实际子命令/退出码全部见下表，失败记录不覆盖。
- Linux 入口：`wsl -d Ubuntu-24.04 -u root -- bash /mnt/d/project/sub2api/output/upstream-sync-20260915-bdb42e22f/integration.sh before|after [race|scope-repeat]`；同目录 `posix-spool.sh before|after`、`smoke.sh before|after` 与 `migration-upgrade.sh` 使用仓库内临时数据与有界清理。

| 检查 | 同步前 | 同步后最终结果与边界 |
| --- | --- | --- |
| Go 默认、unit、构建 | 默认/构建通过；unit 单个 Cyber 1 秒异步等待失败 | 默认与 unit 全包、build 通过；Cyber 不改代码重复复测通过，重点兼容回归通过 |
| golangci-lint、生成 | 通过，Ent 320 文件无差异 | 通过；Wire 无受控代码变化 |
| Vue | 292 文件 2166 用例及 lint/typecheck/build 通过 | 300 文件 2293 用例及冻结安装/lint/typecheck/build 全部通过 |
| Canvas | 34 用例、类型和构建通过；仅 package.json 格式失败 | 同样 34 用例、类型与构建通过；既有格式问题原样保留 |
| Linux integration | 唯一 0600 在 Windows 挂载呈现 0777 的失败 | 最终只剩相同挂载权限失败；前后该单项在仓库内 POSIX tmpfs 均通过，无删除断言/跳过 |
| race | 定向 service/handler 通过 | 同范围通过；WS 两线程关系额外各重复 50 次、race 通过，非全包 race |
| 隔离启动 | PG/Redis 初始化和健康检查通过 | 最新代码重新构建/初始化/health 通过；旧库升级和 238 重入/本地定制数据断言通过 |
| 部署静态 | Compose/资源/远程部署 mock/Caddy/语法通过；Apple BSD stat 失败 | 同样通过/同样环境限制，双生产 Compose 字节一致；未执行真实部署脚本 |
| 依赖安全 | 本轮未做扫描基线 | govulncheck 通过（依赖模块中有 13 项告警，导入包和可达调用均为 0）；前端 audit: 高危 2/严重 0/中危 13/低危 1，Canvas: 高危 0/严重 0/中危 6；原始 audit 均退出 1，现有例外检查均 0，未改依赖/例外 |
| 浏览器 | 未做同步前浏览器基线 | Edge 隔离 profile 与 mock API 验证 1440px 桌面、390px 移动配额、订阅关闭回退、兑换成功态/签到、Canvas 初始化、公开余额不带登录 Authorization/Cookie；无真实媒体/付款 |

### 失败定位与重测

- 同步前 Cyber 异步快照测试的 1 秒等待偶发失败：未修改业务逻辑，单项 `-count=5` 复测通过；同步后完整 unit 通过。Canvas 仅 `package.json` 格式和 Apple `stat -f '%Lp'` 不支持为既有失败。
- 初次后端合并编译发现 OpenCode 测试调用缺少本地提示词参数；后续补齐 Anthropic 可选参数、保留旧调用语义。前端未使用的重复 computed 移除，白名单测试属性对齐本地 previewSyncRequest；全部类型和用例重新通过。
- DeepSeek 新价导致本地旧单价断言不符，只更新该单价，不改定价优先级断言。图片 SSE 错误被 JSON 分支掩盖，改为先抽取上游 SSE 错误；原断言保留后默认/unit 全部通过。
- compact 静态夹具的 gpt-5.4 默认断言同步到获批的新 gpt-5.5；新测试文件 gofmt 后 lint 通过。一次手动 gofmt 使用了错误工作目录，命令找不到目标且未写入；随后在仓库根按具体文件格式化并复核。
- 第二轮 Linux 全量集成偶发命中上游新 SameCodexThreadStillPreempts：测试过早放行 A 的完成帧，与异步关闭通知竞争。只让同线程保持假上游请求在飞直到关闭后再放行，不改运行时；关闭码 1013、原因和恰好一次抢占断言全部保留。两线程用例各 `-count=50 -race` 及完整集成重跑，最终无此失败。
- Windows 挂载权限仅 0600/0777 差异，未把用例关掉；前后 POSIX tmpfs 单项均通过。
- 安全扫描首次缺少本地 govulncheck 可执行文件，随后使用已有缓存可核验的 v1.7.0 安装到仓库工具目录并通过扫描。audit 原始退出 1 不改写为 0，分别记录 CI 例外解析命令退出 0。
- 浏览器最初公告夹具返回对象而非数组，修正隔离 mock 后通过；脚本 URL 构造器在 CLI 沙箱不可用，改用已知路径比较后通过。支付 SDK 外部脚本由隔离 CSP 阻止，兑换后 503 为主动注入；这些不是业务验证成功的外部调用。
- 既有少数单元测试用假 Token 发出 Google 401 请求日志；没有真实凭据或业务写入，但不能声称整个测试过程绝无外部 HTTP 请求。隔离启动禁用外部代理出口，浏览器 CSP 仅允许本地连接。

### 实际验证命令

下表时间保留各记录的原时区（带 Z 为 UTC，其余 +08:00 为北京时间）；所有日期对应本次北京时间 2026-09-15 执行。重测按时间逐次保留。

| 编号 | 阶段/套件/标签 | 开始时间 | 实际命令 | 退出码 |
| --- | --- | --- | --- | --- |
| V001 | 同步前/backend/test-default | 2026-09-15T01:53:58.2303703+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 0 |
| V002 | 同步前/frontend/install | 2026-09-15T01:53:58.5394722+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs install --frozen-lockfile --offline --ignore-scripts` | 0 |
| V003 | 同步前/canvas/install | 2026-09-15T01:53:58.8622846+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs install --frozen-lockfile --offline --ignore-scripts` | 0 |
| V004 | 同步前/static/docker-compose-security-test.sh | 2026-09-15T01:53:59.1875490+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\deploy\tests\docker-compose-security-test.sh` | 0 |
| V005 | 同步前/static/docker-compose-gateway-env-test.sh | 2026-09-15T01:53:59.7399255+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\deploy\tests\docker-compose-gateway-env-test.sh` | 0 |
| V006 | 同步前/frontend/lint | 2026-09-15T01:54:02.1824959+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run lint:check` | 0 |
| V007 | 同步前/canvas/format | 2026-09-15T01:54:02.6051475+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/prettier/bin/prettier.cjs --check .` | 1 |
| V008 | 同步前/canvas/typecheck | 2026-09-15T01:54:13.6880294+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/typescript/bin/tsc --noEmit` | 0 |
| V009 | 同步前/static/docker-runtime-resources-test.sh | 2026-09-15T01:54:32.2996392+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\deploy\tests\docker-runtime-resources-test.sh` | 0 |
| V010 | 同步前/static/remote-deploy-test.sh | 2026-09-15T01:54:32.6827443+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\deploy\tests\remote-deploy-test.sh` | 0 |
| V011 | 同步前/canvas/test | 2026-09-15T01:54:37.5431103+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/vitest/vitest.mjs run --maxWorkers=4 --minWorkers=1` | 0 |
| V012 | 同步前/static/apple-container-test.sh | 2026-09-15T01:54:48.5609324+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\deploy\tests\apple-container-test.sh` | 1 |
| V013 | 同步前/static/caddy-cache | 2026-09-15T01:54:51.7187601+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\deploy\test-caddyfile-cache.sh` | 0 |
| V014 | 同步前/static/apple-syntax | 2026-09-15T01:54:52.9873193+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\apple-container.sh` | 0 |
| V015 | 同步前/static/remote-syntax | 2026-09-15T01:54:53.1275281+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\remote-deploy.sh` | 0 |
| V016 | 同步前/static/diff-check | 2026-09-15T01:54:53.2755118+08:00 | `git diff --check` | 0 |
| V017 | 同步前/frontend/typecheck | 2026-09-15T01:55:24.5897819+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 0 |
| V018 | 同步前/frontend/test | 2026-09-15T01:56:17.0110826+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` | 0 |
| V019 | 同步前/extra/golangci-lint | 2026-09-15T01:56:28.4930658+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\golangci-lint.exe run ./... --timeout=30m` | 0 |
| V020 | 同步前/integration-linux/ | 2026-09-15T01:56:40+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 1 |
| V021 | 同步前/generate/ent-check | 2026-09-15T01:57:00.1453257+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\check-ent.mjs before` | 0 |
| V022 | 同步前/generate/wire | 2026-09-15T01:57:21.0180364+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe generate ./cmd/server` | 0 |
| V023 | 同步前/backend/test-unit | 2026-09-15T01:58:26.2905405+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 1 |
| V024 | 同步前/frontend/build | 2026-09-15T02:00:55.1908981+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V025 | 同步前/smoke-linux/ | 2026-09-15T02:00:57+08:00 | `smoke.sh before` | 0 |
| V026 | 同步前/static/docker-compose-security-test.sh | 2026-09-15T02:00:58.9009743+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-compose-security-test.sh` | 0 |
| V027 | 同步前/static/docker-compose-gateway-env-test.sh | 2026-09-15T02:00:59.9511564+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-compose-gateway-env-test.sh` | 0 |
| V028 | 同步前/static/docker-runtime-resources-test.sh | 2026-09-15T02:01:31.8592109+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-runtime-resources-test.sh` | 0 |
| V029 | 同步前/static/remote-deploy-test.sh | 2026-09-15T02:01:32.4443213+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\remote-deploy-test.sh` | 0 |
| V030 | 同步前/static/apple-container-test.sh | 2026-09-15T02:01:46.4201308+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\apple-container-test.sh` | 1 |
| V031 | 同步前/static/caddy-cache | 2026-09-15T02:01:48.6143352+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\test-caddyfile-cache.sh` | 0 |
| V032 | 同步前/static/apple-syntax | 2026-09-15T02:01:49.5771869+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\apple-container.sh` | 0 |
| V033 | 同步前/static/remote-syntax | 2026-09-15T02:01:49.6855421+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\remote-deploy.sh` | 0 |
| V034 | 同步前/static/diff-check | 2026-09-15T02:01:49.8146132+08:00 | `git diff --check` | 0 |
| V035 | 同步前/backend/build | 2026-09-15T02:04:07.6822287+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V036 | 同步前/canvas-build/build | 2026-09-15T02:07:08.8691293+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V037 | 同步前/backend/cyber-flaky | 2026-09-15T02:09:54.3206091+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./internal/service -run ^TestRecordCyberPolicyEvent_RuntimeSnapshotRefreshFailureKeepsStaleScope$ -count=5 -timeout=2m` | 0 |
| V038 | 同步前/posix-spool-linux/ | 2026-09-15T02:10:01+08:00 | `TMPDIR=posix-tmp go test -tags=integration ./internal/service -run ^TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool$ -count=1 -timeout=5m` | 0 |
| V039 | 同步前/race-linux/ | 2026-09-15T02:10:02+08:00 | `go test -race -tags=unit ./internal/service ./internal/handler -run OpenAIWS\|GrokMedia\|FirstToken\|TemporaryCredit -count=1 -timeout=20m` | 0 |
| V040 | 同步后/backend/test-default | 2026-09-15T02:31:08.2798234+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 1 |
| V041 | 同步后/frontend/install | 2026-09-15T02:31:08.5170472+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs install --frozen-lockfile --offline --ignore-scripts` | 0 |
| V042 | 同步后/canvas/install | 2026-09-15T02:31:08.7395673+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs install --frozen-lockfile --offline --ignore-scripts` | 0 |
| V043 | 同步后/frontend/lint | 2026-09-15T02:31:10.7628656+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run lint:check` | 0 |
| V044 | 同步后/canvas/format | 2026-09-15T02:31:10.8090704+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/prettier/bin/prettier.cjs --check .` | 1 |
| V045 | 同步后/canvas/typecheck | 2026-09-15T02:31:17.2113249+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/typescript/bin/tsc --noEmit` | 0 |
| V046 | 同步后/canvas/test | 2026-09-15T02:31:22.1235345+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe node_modules/vitest/vitest.mjs run --maxWorkers=4 --minWorkers=1` | 0 |
| V047 | 同步后/backend/test-unit | 2026-09-15T02:31:46.6356904+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 1 |
| V048 | 同步后/backend/build | 2026-09-15T02:32:09.9423461+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 1 |
| V049 | 同步后/frontend/typecheck | 2026-09-15T02:32:19.8491752+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 2 |
| V050 | 同步后/frontend/test | 2026-09-15T02:32:44.4442600+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` | 1 |
| V051 | 同步后/backend/sync-critical | 2026-09-15T02:33:39.9571320+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./internal/service ./internal/handler ./internal/server -run OpenCode\|OpenAI.*Images\|CodexDirectImages\|ClaudeOAuth\|SystemCache\|Billing\|Pricing\|OpenAIWS\|APIContracts\|TemporaryCredit\|BalanceQuery -count=1 -timeout=15m` | 1 |
| V052 | 同步后/static/docker-compose-security-test.sh | 2026-09-15T02:33:40.2695877+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-compose-security-test.sh` | 0 |
| V053 | 同步后/generate/ent-check | 2026-09-15T02:33:40.5763607+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\check-ent.mjs after` | 0 |
| V054 | 同步后/static/docker-compose-gateway-env-test.sh | 2026-09-15T02:33:40.6991300+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-compose-gateway-env-test.sh` | 1 |
| V055 | 同步后/static/docker-runtime-resources-test.sh | 2026-09-15T02:33:41.3514115+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-runtime-resources-test.sh` | 0 |
| V056 | 同步后/static/remote-deploy-test.sh | 2026-09-15T02:33:41.8660644+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\remote-deploy-test.sh` | 0 |
| V057 | 同步后/generate/wire | 2026-09-15T02:33:54.5882348+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe generate ./cmd/server` | 0 |
| V058 | 同步后/static/apple-container-test.sh | 2026-09-15T02:33:57.3445437+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\apple-container-test.sh` | 1 |
| V059 | 同步后/static/caddy-cache | 2026-09-15T02:33:58.9279324+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\test-caddyfile-cache.sh` | 0 |
| V060 | 同步后/static/apple-syntax | 2026-09-15T02:34:00.1489985+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\apple-container.sh` | 0 |
| V061 | 同步后/static/remote-syntax | 2026-09-15T02:34:00.2726648+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\remote-deploy.sh` | 0 |
| V062 | 同步后/static/diff-check | 2026-09-15T02:34:00.4071091+08:00 | `git diff --check` | 0 |
| V063 | 同步后/frontend/build | 2026-09-15T02:35:25.5290239+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V064 | 同步后/backend/test-default | 2026-09-15T02:37:44.9778261+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 1 |
| V065 | 同步后/extra/golangci-lint | 2026-09-15T02:37:45.3036076+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\golangci-lint.exe run ./... --timeout=30m` | 1 |
| V066 | 同步后/integration-linux/ | 2026-09-15T02:37:55+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 1 |
| V067 | 同步后/frontend/install | 2026-09-15T02:39:11.1974386+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs install --frozen-lockfile --offline --ignore-scripts` | 0 |
| V068 | 同步后/static/docker-compose-security-test.sh | 2026-09-15T02:39:11.3174215+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-compose-security-test.sh` | 0 |
| V069 | 同步后/static/docker-compose-gateway-env-test.sh | 2026-09-15T02:39:11.9529960+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-compose-gateway-env-test.sh` | 0 |
| V070 | 同步后/frontend/lint | 2026-09-15T02:39:14.1384336+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run lint:check` | 0 |
| V071 | 同步后/static/docker-runtime-resources-test.sh | 2026-09-15T02:39:44.9749447+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\docker-runtime-resources-test.sh` | 0 |
| V072 | 同步后/static/remote-deploy-test.sh | 2026-09-15T02:39:45.4104304+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\remote-deploy-test.sh` | 0 |
| V073 | 同步后/static/apple-container-test.sh | 2026-09-15T02:39:57.6784238+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\tests\apple-container-test.sh` | 1 |
| V074 | 同步后/static/caddy-cache | 2026-09-15T02:39:59.0332008+08:00 | `"C:/Program Files/Git/bin/bash.exe" D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\run-shell.sh D:\project\sub2api\deploy\test-caddyfile-cache.sh` | 0 |
| V075 | 同步后/static/apple-syntax | 2026-09-15T02:39:59.7986853+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\apple-container.sh` | 0 |
| V076 | 同步后/static/remote-syntax | 2026-09-15T02:39:59.8987011+08:00 | `"C:/Program Files/Git/bin/bash.exe" -n D:\project\sub2api\deploy\remote-deploy.sh` | 0 |
| V077 | 同步后/static/diff-check | 2026-09-15T02:39:59.9980452+08:00 | `git diff --check` | 0 |
| V078 | 同步后/frontend/typecheck | 2026-09-15T02:40:20.5551469+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run typecheck` | 0 |
| V079 | 同步后/smoke-linux/ | 2026-09-15T02:40:24+08:00 | `smoke.sh after` | 0 |
| V080 | 同步后/frontend/test | 2026-09-15T02:41:13.0585477+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run test:run --maxWorkers=4 --minWorkers=1` | 0 |
| V081 | 同步后/backend/test-unit | 2026-09-15T02:41:37.0853342+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 1 |
| V082 | 同步后/frontend/build | 2026-09-15T02:45:54.2673130+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V083 | 同步后/backend/build | 2026-09-15T02:47:25.3382843+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V084 | 同步后/canvas-build/build | 2026-09-15T02:48:55.1409892+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs run build` | 0 |
| V085 | 同步后/migration-upgrade-linux/ | 2026-09-15T02:49:03+08:00 | `migration-upgrade.sh` | 0 |
| V086 | 同步后/extra/golangci-lint | 2026-09-15T02:49:28.3979768+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\golangci-lint.exe run ./... --timeout=30m` | 0 |
| V087 | 同步后/backend/test-default | 2026-09-15T02:49:28.4167938+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test ./... -count=1 -timeout=20m` | 0 |
| V088 | 同步后/integration-linux/ | 2026-09-15T02:49:29+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 1 |
| V089 | 同步后/race-linux/ | 2026-09-15T02:49:29+08:00 | `go test -race -tags=unit ./internal/service ./internal/handler -run OpenAIWS\|GrokMedia\|FirstToken\|TemporaryCredit -count=1 -timeout=20m` | 0 |
| V090 | 同步后/backend/test-unit | 2026-09-15T02:52:39.0604351+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./... -count=1 -timeout=20m` | 0 |
| V091 | 同步后/posix-spool-linux/ | 2026-09-15T02:56:42+08:00 | `TMPDIR=posix-tmp go test -tags=integration ./internal/service -run ^TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool$ -count=1 -timeout=5m` | 0 |
| V092 | 同步后/smoke-linux/ | 2026-09-15T02:57:48+08:00 | `smoke.sh after` | 0 |
| V093 | 同步后/backend/build | 2026-09-15T02:58:06.4794653+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe build ./...` | 0 |
| V094 | 同步后/security/govulncheck | 2026-09-15T02:59:06.6292308+08:00 | `D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\go-tools\bin\govulncheck.exe ./...` | 1 |
| V095 | 同步后/security/frontend-audit | 2026-09-15T02:59:06.7240486+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs audit --prod --audit-level=high --json` | 1 |
| V096 | 同步后/security/canvas-audit | 2026-09-15T02:59:11.3837248+08:00 | `D:\project\sub2api\output\upstream-sync-20260910-98d86915b\tools\node-v20.20.2-win-x64\node.exe D:\project\sub2api\output\upstream-sync-20260905-ab99d56e9\pnpm-9.15.9\package\bin\pnpm.cjs audit --prod --audit-level=high --json` | 1 |
| V097 | 同步后/security/install-govulncheck | 2026-09-15T03:03:02.1107968+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe install golang.org/x/vuln/cmd/govulncheck@v1.7.0` | 0 |
| V098 | 同步后/security/govulncheck | 2026-09-15T03:03:23.6425514+08:00 | `D:\project\sub2api\output\upstream-sync-20260915-bdb42e22f\tools\govulncheck.exe ./...` | 0 |
| V099 | 同步后/security/frontend-audit-exceptions | 2026-09-15T03:03:36.0474325+08:00 | `wsl.exe -d Ubuntu-24.04 -u root -- python3 /mnt/d/project/sub2api/tools/check_pnpm_audit_exceptions.py --audit /mnt/d/project/sub2api/output/upstream-sync-20260915-bdb42e22f/logs/after-security-frontend-audit-025906723.log --exceptions /mnt/d/project/sub2api/.github/audit-exceptions.yml` | 0 |
| V100 | 同步后/security/canvas-audit-exceptions | 2026-09-15T03:03:47.9011937+08:00 | `wsl.exe -d Ubuntu-24.04 -u root -- python3 /mnt/d/project/sub2api/tools/check_pnpm_audit_exceptions.py --audit /mnt/d/project/sub2api/output/upstream-sync-20260915-bdb42e22f/logs/after-security-canvas-audit-025911383.log --exceptions /mnt/d/project/sub2api/.github/audit-exceptions.yml` | 0 |
| V101 | 同步后/backend/sync-critical | 2026-09-15T03:05:29.9134338+08:00 | `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe test -tags=unit ./internal/service ./internal/handler ./internal/server -run OpenCode\|OpenAI.*Images\|CodexDirectImages\|ClaudeOAuth\|SystemCache\|Billing\|Pricing\|OpenAIWS\|APIContracts\|TemporaryCredit\|BalanceQuery -count=1 -timeout=15m` | 0 |
| V102 | 同步后/integration-linux/ | 2026-09-15T03:05:43+08:00 | `go test -tags=integration ./... -count=1 -timeout=25m` | 1 |
| V103 | 同步后/scope-repeat-linux/ | 2026-09-15T03:05:43+08:00 | `go test -race -tags=unit ./internal/service -run TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_(CodexThreadsDoNotPreemptEachOther\|SameCodexThreadStillPreempts)$ -count=50 -timeout=10m` | 0 |
| V104 | 同步后/browser-mock/ | 2026-09-14T19:21:15.016Z | `node output/upstream-sync-20260915-bdb42e22f/ui-audit.mjs` | 0 |
| V105 | 同步后/final-audit/staged | 2026-09-14T19:22:47.458Z | `node output/upstream-sync-20260915-bdb42e22f/final-audit.mjs staged` | 0 |
| V106 | 同步后/final-audit/code | 2026-09-14T19:25:05.627Z | `node output/upstream-sync-20260915-bdb42e22f/final-audit.mjs code` | 0 |

### 文件清单与未验证范围

- 完整改动可复核：`git diff --name-status 098f3d9e5a14278ada575daae0fd871b50898817 6454c4f68754a0e79ae6cce549ac616999d0ab57`。下列是 M1 实际文件状态（A 新增，M 修改）；无删除。

```text
M	README_CN.md
M	backend/ent/channelmonitor/channelmonitor.go
M	backend/ent/channelmonitorrequesttemplate/channelmonitorrequesttemplate.go
M	backend/ent/migrate/schema.go
M	backend/ent/schema/channel_monitor.go
M	backend/ent/schema/channel_monitor_request_template.go
M	backend/ent/schema/user_platform_quota.go
M	backend/internal/config/config.go
M	backend/internal/config/config_test.go
M	backend/internal/domain/channel_monitor_quota.go
M	backend/internal/domain/constants.go
M	backend/internal/domain/constants_test.go
M	backend/internal/handler/admin/account_handler.go
M	backend/internal/handler/admin/channel_handler.go
M	backend/internal/handler/admin/channel_monitor_handler.go
M	backend/internal/handler/admin/group_handler.go
M	backend/internal/handler/admin/group_handler_platform_test.go
M	backend/internal/handler/admin/ops_dashboard_handler.go
M	backend/internal/handler/admin/setting_handler.go
M	backend/internal/handler/admin/setting_handler_audit.go
M	backend/internal/handler/admin/setting_handler_partial_payload_test.go
M	backend/internal/handler/admin/setting_handler_update.go
M	backend/internal/handler/admin/user_handler.go
M	backend/internal/handler/admin/user_platform_quota_admin_test.go
M	backend/internal/handler/dto/settings.go
M	backend/internal/handler/endpoint.go
M	backend/internal/handler/endpoint_test.go
M	backend/internal/handler/gateway_handler.go
A	backend/internal/handler/gateway_models_retrieve_test.go
M	backend/internal/handler/gateway_models_test.go
M	backend/internal/handler/grok_media.go
A	backend/internal/handler/grok_media_slots_test.go
M	backend/internal/handler/openai_automation_bootstrap_test.go
M	backend/internal/handler/openai_gateway_cn_dispatch_test.go
M	backend/internal/handler/openai_gateway_handler.go
M	backend/internal/handler/openai_images_failover_test.go
M	backend/internal/handler/openai_models_handler.go
M	backend/internal/handler/ops_error_logger.go
M	backend/internal/handler/ops_error_logger_test.go
M	backend/internal/handler/setting_handler.go
M	backend/internal/model/error_passthrough_rule.go
M	backend/internal/model/error_passthrough_rule_test.go
M	backend/internal/payment/provider/easypay_refund_test.go
M	backend/internal/pkg/antigravity/claude_types.go
M	backend/internal/pkg/antigravity/claude_types_test.go
M	backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go
A	backend/internal/pkg/apicompat/chatcompletions_responses_bridge_additional_tools_test.go
A	backend/internal/pkg/apicompat/chatcompletions_responses_bridge_agent_message_test.go
M	backend/internal/pkg/claude/constants.go
M	backend/internal/pkg/httputil/body.go
A	backend/internal/pkg/httputil/body_memory_test.go
M	backend/internal/pkg/openai/request.go
A	backend/internal/pkg/openai/request_ua_validation_test.go
M	backend/internal/repository/account_repo_integration_test.go
M	backend/internal/repository/channel_monitor_v2_aggregation.go
M	backend/internal/repository/channel_monitor_v2_repo.go
M	backend/internal/repository/channel_monitor_v2_repo_test.go
M	backend/internal/repository/ops_repo_openai_token_stats.go
M	backend/internal/repository/ops_repo_openai_token_stats_test.go
M	backend/internal/repository/ops_repo_request_details.go
A	backend/internal/repository/ops_repo_request_details_test.go
M	backend/internal/repository/req_client_pool.go
M	backend/internal/repository/req_client_pool_test.go
M	backend/internal/repository/scheduler_cache.go
A	backend/internal/repository/scheduler_threshold_cache_unit_test.go
M	backend/internal/repository/user_platform_quota_repo.go
M	backend/internal/repository/user_platform_quota_repo_integration_test.go
M	backend/internal/repository/user_platform_quota_upsert_test.go
M	backend/internal/server/api_contract_test.go
M	backend/internal/server/middleware/admin_auth.go
M	backend/internal/server/middleware/jwt_auth.go
M	backend/internal/server/middleware/jwt_auth_test.go
M	backend/internal/server/routes/gateway.go
M	backend/internal/server/routes/gateway_models_pinned_test.go
M	backend/internal/service/account.go
M	backend/internal/service/account_header_override.go
M	backend/internal/service/account_header_override_test.go
M	backend/internal/service/account_scheduling_threshold_eval.go
M	backend/internal/service/account_service.go
M	backend/internal/service/account_stats_pricing_test.go
M	backend/internal/service/account_test_models.go
M	backend/internal/service/account_test_models_test.go
M	backend/internal/service/account_test_service.go
M	backend/internal/service/account_test_service_cn_adaptive.go
M	backend/internal/service/account_test_service_openai_image_test.go
A	backend/internal/service/account_test_service_opencode_go_test.go
M	backend/internal/service/admin_account.go
M	backend/internal/service/admin_group.go
M	backend/internal/service/admin_service_group_test.go
M	backend/internal/service/auth_email_oauth_auto_test.go
M	backend/internal/service/auth_oauth_email_flow_test.go
M	backend/internal/service/auth_service.go
M	backend/internal/service/auth_service_platform_quota_test.go
M	backend/internal/service/auth_service_register_test.go
M	backend/internal/service/billing_cache_service.go
M	backend/internal/service/billing_context_schedule.go
M	backend/internal/service/billing_service.go
M	backend/internal/service/billing_service_test.go
M	backend/internal/service/channel_monitor_checker.go
M	backend/internal/service/channel_monitor_const.go
A	backend/internal/service/channel_monitor_endpoint_test.go
M	backend/internal/service/channel_monitor_quota_fetcher.go
M	backend/internal/service/channel_monitor_validate.go
M	backend/internal/service/channel_service.go
M	backend/internal/service/channel_service_test.go
M	backend/internal/service/cn_provider_quota_service.go
M	backend/internal/service/cn_providers_test.go
M	backend/internal/service/composite_platform.go
M	backend/internal/service/composite_platform_test.go
M	backend/internal/service/deepseek_pricing_test.go
M	backend/internal/service/domain_constants.go
A	backend/internal/service/gateway_accept_encoding_test.go
M	backend/internal/service/gateway_claude_oauth_body.go
M	backend/internal/service/gateway_context_management_test.go
M	backend/internal/service/gateway_count_tokens.go
M	backend/internal/service/gateway_forward.go
A	backend/internal/service/gateway_mid_conversation_output_config_test.go
M	backend/internal/service/gateway_record_usage_test.go
M	backend/internal/service/gateway_request.go
A	backend/internal/service/gateway_system_cache_control_test.go
M	backend/internal/service/gemini_image_output_accounting_test.go
M	backend/internal/service/gemini_messages_compat_service.go
M	backend/internal/service/gemini_messages_compat_service_test.go
A	backend/internal/service/gemini_native_response_signal_test.go
A	backend/internal/service/gemini_response_signal.go
A	backend/internal/service/gemini_response_signal_test.go
M	backend/internal/service/grok_media.go
A	backend/internal/service/grok_media_selection_test.go
M	backend/internal/service/header_util.go
M	backend/internal/service/image_output_accounting.go
M	backend/internal/service/openai_account_scheduler.go
A	backend/internal/service/openai_codex_context_window_test.go
M	backend/internal/service/openai_codex_identity.go
M	backend/internal/service/openai_codex_model_metadata.go
M	backend/internal/service/openai_codex_models_service.go
M	backend/internal/service/openai_codex_models_service_test.go
M	backend/internal/service/openai_fast_policy_test.go
M	backend/internal/service/openai_fast_policy_ws_test.go
M	backend/internal/service/openai_gateway_cc_pipeline.go
M	backend/internal/service/openai_gateway_chat_completions.go
M	backend/internal/service/openai_gateway_chat_completions_anthropic_native.go
M	backend/internal/service/openai_gateway_cn_fixes_test.go
M	backend/internal/service/openai_gateway_count_tokens.go
M	backend/internal/service/openai_gateway_forward.go
M	backend/internal/service/openai_gateway_messages.go
M	backend/internal/service/openai_gateway_messages_anthropic_native.go
A	backend/internal/service/openai_gateway_opencode_mapping_test.go
M	backend/internal/service/openai_gateway_passthrough.go
M	backend/internal/service/openai_gateway_record_usage_test.go
M	backend/internal/service/openai_gateway_request_body.go
M	backend/internal/service/openai_gateway_response_handling.go
M	backend/internal/service/openai_gateway_responses_anthropic_native.go
M	backend/internal/service/openai_gateway_scheduling.go
M	backend/internal/service/openai_gateway_service.go
M	backend/internal/service/openai_gateway_usage.go
M	backend/internal/service/openai_gpt56_max_test.go
M	backend/internal/service/openai_images.go
M	backend/internal/service/openai_images_actual_size_test.go
A	backend/internal/service/openai_images_direct.go
A	backend/internal/service/openai_images_direct_payload_test.go
A	backend/internal/service/openai_images_direct_test.go
M	backend/internal/service/openai_images_json_keepalive_test.go
M	backend/internal/service/openai_images_responses.go
M	backend/internal/service/openai_images_test.go
A	backend/internal/service/openai_large_request_memory_test.go
A	backend/internal/service/openai_large_request_rewrite_bench_test.go
M	backend/internal/service/openai_messages_dispatch.go
M	backend/internal/service/openai_models_list.go
M	backend/internal/service/openai_models_list_test.go
M	backend/internal/service/openai_opencode_session.go
M	backend/internal/service/openai_opencode_session_test.go
A	backend/internal/service/openai_privacy_cf_challenge_test.go
M	backend/internal/service/openai_privacy_service.go
M	backend/internal/service/openai_request_body_release.go
A	backend/internal/service/openai_request_raw_input.go
M	backend/internal/service/openai_responses_ingress_compat.go
M	backend/internal/service/openai_responses_item_id.go
M	backend/internal/service/openai_setup_token_compat_test.go
A	backend/internal/service/openai_ua_validation_test.go
M	backend/internal/service/openai_ws_client.go
A	backend/internal/service/openai_ws_execution_scope.go
A	backend/internal/service/openai_ws_execution_scope_test.go
M	backend/internal/service/openai_ws_forwarder_ingress.go
A	backend/internal/service/openai_ws_forwarder_ingress_execution_scope_test.go
A	backend/internal/service/openai_ws_forwarder_ingress_retry_test.go
M	backend/internal/service/openai_ws_forwarder_logutil.go
M	backend/internal/service/openai_ws_forwarder_success_test.go
M	backend/internal/service/openai_ws_forwarder_v2.go
A	backend/internal/service/openai_ws_forwarder_v2_execution_scope_test.go
M	backend/internal/service/openai_ws_pool.go
A	backend/internal/service/openai_ws_pool_reader_loop_test.go
M	backend/internal/service/openai_ws_pool_test.go
M	backend/internal/service/openai_ws_session_preemption.go
M	backend/internal/service/openai_ws_session_preemption_test.go
A	backend/internal/service/opencode_go.go
A	backend/internal/service/opencode_go_test.go
M	backend/internal/service/ops_request_details.go
M	backend/internal/service/ops_upstream_context.go
M	backend/internal/service/payment_config_providers.go
M	backend/internal/service/payment_config_providers_test.go
M	backend/internal/service/pricing_service.go
M	backend/internal/service/pricing_service_test.go
M	backend/internal/service/ratelimit_cn_providers.go
M	backend/internal/service/ratelimit_service.go
M	backend/internal/service/ratelimit_service_openai_image_test.go
M	backend/internal/service/scheduler_snapshot_bulk_event_test.go
M	backend/internal/service/scheduler_snapshot_service.go
M	backend/internal/service/setting_features.go
M	backend/internal/service/setting_parse.go
M	backend/internal/service/setting_public.go
M	backend/internal/service/setting_service.go
M	backend/internal/service/setting_service_public_test.go
M	backend/internal/service/setting_update.go
M	backend/internal/service/settings_view.go
M	backend/internal/service/upstream_billing_probe.go
M	backend/internal/service/upstream_billing_probe_multiplatform_test.go
M	backend/internal/service/upstream_models.go
M	backend/internal/service/user_platform_quota_flusher.go
M	backend/internal/service/user_platform_quota_port.go
M	backend/internal/setup/setup.go
M	backend/internal/setup/setup_test.go
A	backend/migrations/238_opencode_go_platform.sql
A	backend/migrations/238_purge_unlimited_user_platform_quotas.sql
A	backend/migrations/opencode_go_platform_migration_test.go
A	backend/migrations/user_platform_quota_purge_unlimited_migration_test.go
M	backend/resources/model-pricing/model_prices_and_context_window.json
M	deploy/.env.example
M	deploy/APPLE_CONTAINER.md
M	deploy/README.md
M	deploy/apple-container.sh
M	deploy/config.example.yaml
M	deploy/docker-compose.dev.yml
M	deploy/docker-compose.local.yml
M	deploy/docker-compose.standalone.yml
M	deploy/docker-compose.sub2api.yml
M	deploy/docker-compose.yml
M	deploy/tests/apple-container-test.sh
M	deploy/tests/docker-compose-gateway-env-test.sh
M	deploy/tests/fixtures/bin/container
M	docs/custom-development-history.md
M	frontend/src/App.vue
M	frontend/src/__tests__/integration/proxy-data-import.spec.ts
M	frontend/src/api/__tests__/client.spec.ts
M	frontend/src/api/admin/accounts.ts
M	frontend/src/api/admin/channelMonitor.ts
M	frontend/src/api/admin/cnProviders.ts
M	frontend/src/api/admin/ops.ts
M	frontend/src/api/admin/settings.ts
M	frontend/src/api/client.ts
M	frontend/src/components/account/AccountUsageCell.vue
M	frontend/src/components/account/BulkEditAccountModal.vue
M	frontend/src/components/account/CNProviderQuotaCell.vue
M	frontend/src/components/account/CreateAccountModal.vue
M	frontend/src/components/account/EditAccountModal.vue
M	frontend/src/components/account/ModelWhitelistSelector.vue
A	frontend/src/components/account/OpenCodeGoProtocolRulesEditor.vue
M	frontend/src/components/account/__tests__/CreateAccountModal.spec.ts
M	frontend/src/components/account/__tests__/EditAccountModal.spec.ts
M	frontend/src/components/account/__tests__/ModelWhitelistSelector.spec.ts
M	frontend/src/components/account/__tests__/credentialsBuilder.spec.ts
M	frontend/src/components/account/credentialsBuilder.ts
M	frontend/src/components/admin/monitor/MonitorFiltersBar.vue
M	frontend/src/components/admin/monitor/MonitorFormDialog.vue
M	frontend/src/components/admin/monitor/MonitorTemplateManagerDialog.vue
M	frontend/src/components/admin/proxy/ImportDataModal.vue
M	frontend/src/components/admin/usage/UsageTable.vue
M	frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts
M	frontend/src/components/admin/user/UserPlatformQuotaModal.vue
M	frontend/src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts
M	frontend/src/components/common/PlatformIcon.vue
M	frontend/src/components/common/PlatformTypeBadge.vue
M	frontend/src/components/common/SubscriptionProgressMini.vue
A	frontend/src/components/common/__tests__/PlatformTypeBadge.openaiPlans.spec.ts
M	frontend/src/components/keys/UseKeyModal.vue
M	frontend/src/components/layout/AppHeader.vue
M	frontend/src/components/layout/AppSidebar.vue
M	frontend/src/components/layout/__tests__/AppSidebar.spec.ts
M	frontend/src/components/payment/PaymentProviderDialog.vue
M	frontend/src/components/payment/__tests__/PaymentProviderDialog.spec.ts
M	frontend/src/components/user/dashboard/UserDashboardStats.vue
A	frontend/src/components/user/dashboard/__tests__/UserDashboardStats.spec.ts
M	frontend/src/components/user/monitor/MonitorCard.vue
M	frontend/src/components/user/monitor/ProviderIcon.vue
M	frontend/src/components/user/profile/ProfileBalanceNotifyCard.vue
M	frontend/src/components/user/profile/ProfileEditForm.vue
M	frontend/src/components/user/profile/ProfilePasswordForm.vue
A	frontend/src/components/user/profile/__tests__/ProfileBalanceNotifyCard.spec.ts
A	frontend/src/components/user/profile/__tests__/ProfileEditForm.spec.ts
M	frontend/src/components/user/profile/__tests__/ProfilePasswordForm.spec.ts
M	frontend/src/composables/useChannelMonitorFormat.ts
M	frontend/src/composables/useModelWhitelist.ts
M	frontend/src/constants/__tests__/platforms.spec.ts
M	frontend/src/constants/channelMonitor.ts
M	frontend/src/constants/platforms.ts
M	frontend/src/i18n/__tests__/openaiFastPolicyLocales.spec.ts
M	frontend/src/i18n/__tests__/wsModeLocaleDesc.spec.ts
M	frontend/src/i18n/locales/en/admin/accounts.ts
M	frontend/src/i18n/locales/en/admin/channels.ts
M	frontend/src/i18n/locales/en/admin/ops.ts
M	frontend/src/i18n/locales/en/admin/overview.ts
M	frontend/src/i18n/locales/en/admin/settings.ts
M	frontend/src/i18n/locales/en/common.ts
M	frontend/src/i18n/locales/en/dashboard.ts
M	frontend/src/i18n/locales/en/landing.ts
M	frontend/src/i18n/locales/en/misc.ts
M	frontend/src/i18n/locales/zh/admin/accounts.ts
M	frontend/src/i18n/locales/zh/admin/channels.ts
M	frontend/src/i18n/locales/zh/admin/ops.ts
M	frontend/src/i18n/locales/zh/admin/overview.ts
M	frontend/src/i18n/locales/zh/admin/settings.ts
M	frontend/src/i18n/locales/zh/common.ts
M	frontend/src/i18n/locales/zh/dashboard.ts
M	frontend/src/i18n/locales/zh/landing.ts
M	frontend/src/i18n/locales/zh/misc.ts
M	frontend/src/router/__tests__/feature-access.spec.ts
M	frontend/src/router/__tests__/title.spec.ts
M	frontend/src/router/index.ts
M	frontend/src/router/meta.d.ts
M	frontend/src/router/title.ts
M	frontend/src/stores/__tests__/app.spec.ts
M	frontend/src/stores/app.ts
M	frontend/src/types/index.ts
M	frontend/src/utils/__tests__/accountTestModels.spec.ts
A	frontend/src/utils/__tests__/featureFlags.spec.ts
M	frontend/src/utils/__tests__/openaiWsMode.spec.ts
A	frontend/src/utils/__tests__/siteBillingMode.spec.ts
M	frontend/src/utils/accountTestModels.ts
M	frontend/src/utils/featureFlags.ts
M	frontend/src/utils/openaiWsMode.ts
A	frontend/src/utils/planType.ts
M	frontend/src/utils/platformColors.ts
A	frontend/src/utils/siteBillingMode.ts
M	frontend/src/views/KeyUsageView.vue
M	frontend/src/views/__tests__/KeyUsageView.spec.ts
M	frontend/src/views/admin/ChannelsView.vue
M	frontend/src/views/admin/GroupsView.vue
M	frontend/src/views/admin/ProxiesView.vue
M	frontend/src/views/admin/RedeemView.vue
M	frontend/src/views/admin/SettingsView.vue
M	frontend/src/views/admin/SubscriptionsView.vue
M	frontend/src/views/admin/UsersView.vue
M	frontend/src/views/admin/__tests__/GroupsView.compositePlatforms.spec.ts
A	frontend/src/views/admin/__tests__/ProxiesView.filters.spec.ts
M	frontend/src/views/admin/__tests__/SettingsView.spec.ts
M	frontend/src/views/admin/__tests__/SubscriptionsView.userUsageLink.spec.ts
M	frontend/src/views/admin/__tests__/UsersView.spec.ts
M	frontend/src/views/admin/__tests__/channelPlatformOptions.spec.ts
M	frontend/src/views/admin/ops/OpsDashboard.vue
M	frontend/src/views/admin/ops/components/OpsDashboardHeader.vue
M	frontend/src/views/admin/ops/components/OpsErrorDetailsModal.vue
M	frontend/src/views/admin/ops/components/OpsErrorLogTable.vue
M	frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue
M	frontend/src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts
M	frontend/src/views/admin/ops/components/__tests__/OpsOpenAITokenStatsCard.spec.ts
A	frontend/src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts
M	frontend/src/views/user/CustomPageView.vue
M	frontend/src/views/user/KeysView.vue
M	frontend/src/views/user/PaymentView.vue
M	frontend/src/views/user/RedeemView.vue
M	frontend/src/views/user/UsageView.vue
M	frontend/src/views/user/__tests__/CustomPageView.spec.ts
M	frontend/src/views/user/__tests__/KeysView.spec.ts
M	frontend/src/views/user/__tests__/PaymentView.spec.ts
M	frontend/src/views/user/__tests__/RedeemView.spec.ts
M	frontend/src/views/user/__tests__/UsageView.spec.ts
```

- 未验证：真实上游 OpenCode/Claude/Gemini/Codex Images/WS、真实插件 transport、支付与第三方回调、邮件/短信、实际媒体生成、完整浏览器业务端到端、全包 race、峰值负载、Apple 专有容器以及生产应用/数据库行为。新默认值与第三方实际价格/模型可用性仅按固定上游代码集成和本地测试，未使用真实账号在线校验。
- 剩余依赖告警、Canvas 既有格式与平台专属验证欠缺继续保留，不新增安全例外或降低 CI 门禁。Go 软内存和数据库资源叠加风险并未经过本次峰值压测。
- 本次临时日志和独立浏览器截图保留在仓库忽略目录，不纳入提交；本次 mock 浏览器/服务、隔离启动进程与容器、POSIX 临时挂载已关闭/清理，不删除其他工作资料。
- 最终门禁：确认两份台账、逐提交处置、版本、旧迁移、57 编号、双 Compose、冲突/秘密特征及工作树；确认 main 仍等于 LOCAL_PRE_SYNC_SHA 后仅执行 `git switch main` 与 `git merge --ff-only sync/upstream-20260915-bdb42e22f`。记录提交自身及最终 main SHA 只在最终总结中报告，不写入本记录产生自引用。
- 收尾只读复核：stash 数量 0；没有实际 submodule 或 Git LFS 文件；未配置自定义 hooksPath，默认 hooks 目录没有启用的非 sample Hook。隔离容器标签查询为空、测试应用进程查询为空，POSIX 临时挂载不存在。

## 2026-09-15 追加授权发布 v0.1.233

- 本条追加于上述本地同步完成之后，不改写当时未推送、未访问生产或部署的历史事实。用户新增授权为：任务未完成则继续，已完成则提交代码，通过 CI 后更新 tag 自动部署。
- 上游固定范围仍为 `98d86915becae9fe9491a91ffc6defd5235c8d2b..bdb42e22f81fcb633ff0a060961211dd2bcb515b`，141 个提交处置不变；LAST_FULLY_INTEGRATED_UPSTREAM_SHA: `bdb42e22f81fcb633ff0a060961211dd2bcb515b`。本条没有新增上游代码或扩大范围。
- 合并提交 `6454c4f68754a0e79ae6cce549ac616999d0ab57`，本地同步记录与发布候选 `148cd9afab8b0bf4b8dbc3ba00c8b4cd8b96cce5`。候选相对同步前共 366 文件变化；先推送 main，等待固定 SHA 的 CI `34888272480` 与 Security Scan `34888272483` 全部成功，再于 03:54:48 创建并推送 `v0.1.233`。
- 新标签对象 `f6d9b40d68e3cb6a12c5fc8aa22fac1ffb4aeae2` 指向该候选；没有复用或改写旧标签。tag CI `34889754413`、安全扫描 `34889754535` 也全部通过；没有增加安全例外或关闭检查。Linux 集成和 macOS 脚本 CI 通过，补充了本地平台验证边界，但不代表真实 Apple 专有容器能力已验收。
- Release `34889754841` 的五项必要任务全部成功，实际生产部署步骤于 04:07:57 成功，未被跳过。镜像 `saviour2411/sub2api:0.1.233` 的 OCI revision 与候选完全一致。自动版本回写提交 `2b9db482350d72ce37c1f4f76e62d8c74df06a52` 仅更新 VERSION，本地通过 fetch 与 ff-only 接收。
- 生产只读验收于 04:10:48 通过：双 Compose 与 config 哈希不变、三项数据 bind mount 正确、仅回环监听、3600 秒通用响应头超时及 5 秒用量任务保留、既有资源值不变。PostgreSQL/Redis 容器 ID 与启动时间未变；3 份相对上一生产标签新增 SQL 的执行记录和按运行器规则计算的校验和一致。公网健康检查为 200/ok。
- 关停观察发现 HTTP 强制关停，随后全部清理完成；观察期间没有清理失败、清理超时或用量任务丢弃告警。不能宣称零中断、全部账单完整或所有真实业务无回归；未执行账单补写、生产迁移重跑或旧库回退。
- 生产机没有 YAML 依赖，验收改为本地既有解析器处理内存管道后脱敏。首次迁移哈希检查因验收脚本未按运行器去除首尾空白而拒绝，修正脚本后全部匹配；部署中一次健康尚未就绪的采样被拒绝，部署完成后重测成功。生产配置和迁移文件未为通过验收而改变。
- 两份台账同步补记；详细运维结果见 `docs/operations/2026-09-15-v0.1.233-release.md`，脱敏证据保留在仓库忽略目录 `output/release-20260915-bdb42e22f/`。备份与同步分支保留原位置；本次追加记录提交自身 SHA 仅在交付总结报告。

## 2026-09-18 本地完整同步 efe9aab1e

### 参数、权限与本地提交

- 时间：从 2026-09-18T02:03:51+08:00 开始，使用北京时间；实际结束时间见本节验证记录。
- 执行状态：成功（本地同步候选已完整集成，并通过相对基线门禁；main快进在记录提交后执行）。门禁结果、限制和最终快进条件如下；记录提交自身 SHA 与最终 main SHA 只在交付总结报告，不在文档内自引用。
- 仓库：`D:/project/sub2api`；本地目标分支：`main`；LOCAL_PRE_SYNC_SHA: `23f34132d5e591007bf8ac8472918fec3af871e5`。初检工作树干净，与 origin/main 同步；stash 为0，无实际 submodule 或 Git LFS 文件，没有启用的非 sample Hook 或自定义 hooksPath。
- 上游：`https://github.com/Wei-Shaw/sub2api`；remote 为 `upstream`，远端 HEAD 对应默认分支 `main`。目标在 fetch 后固定，没有随之后远端变化扩大范围。
- UPSTREAM_OLD_SHA: `bdb42e22f81fcb633ff0a060961211dd2bcb515b`，来自上一条可信完整同步记录；UPSTREAM_NEW_SHA: `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`。
- ACTUAL_MERGE_BASE: `bdb42e22f81fcb633ff0a060961211dd2bcb515b`；旧基线同时是本地同步前 HEAD 和新上游的祖先，记录范围与实际合并范围一致。
- LAST_FULLY_INTEGRATED_UPSTREAM_SHA: `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`。本轮是完整 merge，不是选择性移植；58 个普通提交与50个上游合并提交均保留祖先关系。
- 备份分支：`backup/pre-upstream-sync-20260918-020351-23f34132d`；同步分支：`sync/upstream-20260918-efe9aab1e`。备份一直保留原 SHA。
- 用户批准范围：仅本地备份、完整合并、明确的兼容修改、隔离验证、提交和最后 ff-only 更新 main；没有 push、PR、tag、SSH、远程部署、生产数据库或真实业务写入。未读取真实 .env、Token、密码、私钥或带认证信息的 URL。
- 策略：`git merge --no-ff --no-commit efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`，逐块处理已批准冲突；本地合并提交 M1 为 `9dd767d412f9881ef2ecf7891c99ae2823c10d21`，两个父提交分别为 LOCAL_PRE_SYNC_SHA 和 UPSTREAM_NEW_SHA。提交信息：`sync: 合并上游更新至 efe9aab1e 并保留本地定制`。
- M1 相对同步前变化170文件（52新增、118修改、0删除），包含兼容回归和二开台账。最后一个本地提交只追加本同步历史，因此最终合计171文件（52新增、119修改、0删除）。

### 冲突与定制边界

| 已批准的冲突文件 | 最终处理 |
| --- | --- |
| backend/cmd/server/VERSION | 保留本地0.1.239，上游0.2.5版本提交记为 Applied + Overridden；没有发布或部署。 |
| backend/cmd/server/wire_gen.go | 重新执行 Wire；加入 Ollama 探测依赖，同时保留 lifecycle、customFeatureHandler、accountFailureStreakCache。 |
| backend/go.sum | 逐条合并并执行 tidy，保留本地 grpc 1.83.2 及安全依赖版本；恢复原有代码生成工具校验条目。go.mod 与同步前相同，不批量降级。 |
| backend/internal/repository/account_repo.go | 保留本地 ClearOpenAIRateLimitIfObserved 和上游 SetRateLimitedIfUnchanged 两个独立 CAS，不互相替换。 |
| backend/internal/service/ratelimit_service.go | 本地连续失败保护、自动探测与 Ollama 用量窗口探测分别保留，避免重复字段。 |
| backend/internal/service/redeem_service.go | 保留本地多次兑换事务、使用明细查询和扫描，分页 API 委托既有本地仓储。 |
| frontend/src/components/payment/__tests__/AmountInput.spec.ts | 同时保留赠送角标与非法输入回退测试。 |
| frontend/src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts | 保留 PROVIDERS.length 与 MiniMax 本地覆盖，最终相对同步前无净变化。 |
| frontend/src/views/auth/__tests__/RegisterView.spec.ts | 合并缓存设置、设置加载与容错 mock，保留确认密码和推广码测试。 |
| frontend/src/views/user/KeysView.vue | 批量编辑与本地列设置并存，不删除本地表格偏好。 |

- 自动合并后的机械兼容：确认密码控件引用改为本地 registrationInputDisabled；Gemini 三个构造调用补 SettingService 参数，新 SSE 测试补 Account 参数；签到奖励夹具采用分页对象；105条兑换分页夹具改为通过本地仓储产生独立使用记录；Ollama 旧回调用例显式改变限流代次，消除 Windows 时钟精度问题。没有删除断言、跳过检查或修改生产逻辑让测试通过。
- 新增交叉回归：Kimi Chat/Responses 各覆盖流式与非流式，验证 developer 规范化与采样参数400重试共存、入口请求体不变、用量正确且诊断仅一次；同码多人兑换覆盖分页总数、用户归属与使用时间；注册设置未就绪时确认输入及显示按钮保持禁用。
- 保留58个既有 CUST 编号及本地行为，编号集合前后完全一致；Kimi四项默认关闭开关、Claude流终止清理与安全重试、首有效内容计时、首Token分组策略、连续失败及人工暂停保护、自动授信、多次兑换、赠送角标、MiniMax、模型/价格/路由覆盖、按用户串行扣费、5秒usage任务、蓝绿生命周期及强制退役风险边界均未被删除。
- `docs/custom-development-history.md` 同步更新11个关联能力项（CUST-GW-006/007、CUST-PROTO-006、CUST-ACC-001/004、CUST-OBS-001、CUST-PROD-005/006、CUST-UI-004/005、CUST-OPS-004）并追加变更记录；58项统计只是校正既有编号计数，不新增或复用编号。
- 部署文件、双 Compose、生产资源入口、迁移历史、Ent、VERSION、go.mod、前端/Canvas依赖锁文件与同步前没有净变化。生产状态只引用既有台账，本次没有重新在线核验。

### 逐提交处置与映射

- Applied 共108项，其中普通 Applied 107项、Applied + Overridden 1项（版本号）；Already Applied、Skipped、Deferred、Conflict 均为0。grpc升级内容在本地已等价存在，但本轮仍完整合并其历史并记 Applied，不将其伪称为跳过。
- 下表全部提交都映射到本地合并 M1（`9dd767d412f9881ef2ecf7891c99ae2823c10d21`）；每个上游 merge 连同其父提交图一起集成，最终明确的本地覆盖见上表。批量生图缓存尝试及其上游回退均保留，不把被后续回退的历史标记为漏合并。

| 序号 | 上游完整 SHA | 状态 | 处置原因及映射 |
| --- | --- | --- | --- |
| 1 | `e1eff57c255d5520e1dcb6b205c4f1f05633246e` | Applied | OAuth 注册保留推广码及空值回归。映射 M1。 |
| 2 | `017cc62e41f3acde713b18f71d7c45e7faf7a385` | Applied | 注册增加确认密码；沿用本地输入禁用状态与设置加载容错。映射 M1。 |
| 3 | `58f461b0873e86f6c44ba8235b71a556b04f07a3` | Applied | API Key 批量编辑；保留本地列设置和表格偏好。映射 M1。 |
| 4 | `6c2d2ed0439e08b182d480ad4d185dfeecaebbda` | Applied | 存在客户端工具时移除混合 web_search，保持上游工具约束。映射 M1。 |
| 5 | `c0c4ce1ce6d04b9c1411a9f9982373a631d26469` | Applied | 完整集成上游分支同步与冲突解决历史。映射 M1。 |
| 6 | `1d4b9ead45c6fcfdf1f5c8d584946a0a73f82f14` | Applied | 同步混合工具过滤的测试预期。映射 M1。 |
| 7 | `140e1729512253f3c3076403391621c0e8577b12` | Applied | 保留上游触发 CI 的空提交历史，无新增业务改动。映射 M1。 |
| 8 | `ef891643e976c4f27a30725ce7dbb33afb5d6040` | Applied | 完整集成上游合并 #6689 及父提交关系。映射 M1。 |
| 9 | `1ed36679bd8f2be0ec69c9b2ea3ff7b4e4da49bf` | Applied | 从 done 与终止事件恢复 Responses 文本。映射 M1。 |
| 10 | `c6727ed4c96bb8981041c54ce3a5cd7843363ef8` | Applied | 展示 Antigravity 部分刷新告警。映射 M1。 |
| 11 | `7e152e4279347ea30bde2022979afd40ce85f768` | Applied | 修复选号耗时与粘性命中重复统计。映射 M1。 |
| 12 | `d2c2b46c9e28dae24c60771bf8c107e17a8122e1` | Applied | 修正批量生图账号优先级方向。映射 M1。 |
| 13 | `db76cc4e4ea6f96f9e6a82d1db45c39b32c41d1e` | Applied | 按规范 Codex 配额窗口和重置时间调度。映射 M1。 |
| 14 | `bc8da78150128bd395458252a1f1160f0edb3617` | Applied | Antigravity OAuth 缓存按账号隔离。映射 M1。 |
| 15 | `62635532a662e70a15ba9304f57b7885f75b2e24` | Applied | 显式失效旧项目维度的 Token 缓存。映射 M1。 |
| 16 | `196c15b5cd26436c209e50466ec1b4fe1a98f8c9` | Applied | 清理 OAuth 输入项中的内部元数据。映射 M1。 |
| 17 | `e5272c130c8afea103fbdb9252c09f98db1c7eb5` | Applied | 补充输入项类型检查的回归断言。映射 M1。 |
| 18 | `a1d5968b24bdfa1423f234f1320656c113ddce62` | Applied | Ollama 异步用量重置与写入 CAS；保留本地失败保护和独立清理 CAS，修正时钟夹具。映射 M1。 |
| 19 | `3d6c207723b40c2f727f0fec3e37dbee49217b71` | Applied | 订阅批量操作及部分成功反馈。映射 M1。 |
| 20 | `e3cce574d7254b16f940bd662416f2820428c4c7` | Applied | 订阅延期串行化、事务与幂等协调。映射 M1。 |
| 21 | `52558381c69dba77e9ef5271d8fbc0d5252a8911` | Applied | 合并前导 system 消息并兼容对话中的 system 角色。映射 M1。 |
| 22 | `06af9f871c8081c9c13c6d3ffc6c482ebef80101` | Applied | 监控页面采用用户选择的刷新间隔；MiniMax 覆盖不变。映射 M1。 |
| 23 | `5ecfaa4837530e1e4da497063b675ceb20dd7e6e` | Applied | 保留生图访问缓存用户隔离提交；后续上游明确回退也一并保留。映射 M1。 |
| 24 | `28a4ea914d1f2636406b37a3bbe29964fc1a67a5` | Applied | 允许显式清空代理凭据，不把缺省字段当作清空。映射 M1。 |
| 25 | `4bd7132ba9b51b7d0d0a567d28d6afac23a6baab` | Applied | 保留 Responses Lite 命名空间调用。映射 M1。 |
| 26 | `087987070b5a1413d698dbd9677986ba41a964e7` | Applied | grok-build 输出稳定包含 sequence_number。映射 M1。 |
| 27 | `13c1d4cde291fe107bb3934e43174d43358aea48` | Applied | 覆盖 Lite 命名空间工具回传。映射 M1。 |
| 28 | `e9169901d2ef15d3cf49b6ebc4f75cd7de292137` | Applied | 完整集成上游合并 #7112 及父提交关系。映射 M1。 |
| 29 | `a4c517917beb4c18bd36b584d7fbcdfa757804d1` | Applied | 完整集成上游合并 #7073 及父提交关系。映射 M1。 |
| 30 | `008391f699bdd3b63e93550aefb5c9e1ab631011` | Applied | 完整集成上游合并 #7085 及父提交关系。映射 M1。 |
| 31 | `f7e959ae5f9fccb97ee12ffebd0dc3036c32c987` | Applied | 完整集成上游合并 #7111 及父提交关系。映射 M1。 |
| 32 | `1e1d15cca527e31759ec1f1cc5b5876cc6f8377b` | Applied | 完整集成上游合并 #7082 及父提交关系。映射 M1。 |
| 33 | `d4439129a7011b994e9fd3021bab9651565eb60c` | Applied | 完整集成上游合并 #7094 及父提交关系。映射 M1。 |
| 34 | `dd2e6b36801c7bef5607e0580e5b19263fbbf54c` | Applied | 完整集成上游合并 #7076 及父提交关系。映射 M1。 |
| 35 | `3e4b6a09b2d7b6a1d118f76aef5fa2f759dec912` | Applied | 完整集成上游合并 #6925 及父提交关系。映射 M1。 |
| 36 | `150c52b8de331a985038c5972f993f9623105bf9` | Applied | 完整集成上游合并 #6689 及父提交关系。映射 M1。 |
| 37 | `0bba06d64a2eafdec1b4d37a48e5af40a6d44ce7` | Applied | 完整集成上游合并 #6691 及父提交关系。映射 M1。 |
| 38 | `a444551a6d042f9f0ada68544b653a45b9757532` | Applied | 完整集成上游合并 #6654 及父提交关系。映射 M1。 |
| 39 | `1920a7d6879be25884733f4e8629d0ff3c0e43b6` | Applied | 完整集成上游合并 #7091 及父提交关系。映射 M1。 |
| 40 | `9fade3e8c2ff1e9097364c9e5d1cf8a7e4f36559` | Applied | 完整集成上游合并 #6769 及父提交关系。映射 M1。 |
| 41 | `fb90414c0512537478e3aef4de03c0dcfd70a7aa` | Applied | 完整集成上游合并 #7126 及父提交关系。映射 M1。 |
| 42 | `2061508ad858203aae473af90919e174f14bdbf5` | Applied | 完整集成上游合并 #7072 及父提交关系。映射 M1。 |
| 43 | `0a215c931865fa215b585cd57c308ffbcacbcafe` | Applied | 完整集成上游合并 #7110 及父提交关系。映射 M1。 |
| 44 | `325213ae66adb3fc575fb7c82e4d0ac09732cecd` | Applied | 按上游最终历史回退 5ecfaa483 的缓存隔离尝试，不另行引入本地策略。映射 M1。 |
| 45 | `1be482c313f9a7eb7c54f39a7ece3071e9686719` | Applied | 完整集成上游合并 #7148 及父提交关系。映射 M1。 |
| 46 | `badfad8b7248b8aac0e6b503a06e392aa31cb294` | Applied | 完整集成上游合并 #6257 及父提交关系。映射 M1。 |
| 47 | `a939553c873e4c6ca2ff6c5a5b56e4a37114b1d5` | Applied | DeepSeek 模型名无映射时按上游官方列表校验，不默默兜底。映射 M1。 |
| 48 | `55a95d4c674f0624fad1c31ee4afd0ec03e2a53f` | Applied | API Key 分组按 Provider 筛选并保留本地混合分组能力。映射 M1。 |
| 49 | `32682a4f84c6a29104439050a0be14737552cec2` | Applied | 更新上游赞助文档。映射 M1。 |
| 50 | `8447bdd36821933bdf2620f5ff3016daab74f905` | Applied | Gemini SSE 透传移除额外空行；测试补齐本地账号参数。映射 M1。 |
| 51 | `2d9af8592fc97db2ad4b237b32757de8f3af544d` | Applied | 完整集成上游合并 #7153 及父提交关系。映射 M1。 |
| 52 | `25c21aeddac89eb1b1abd90ae7d90fb5ec14c6e8` | Applied | 完整集成上游合并 #7129 及父提交关系。映射 M1。 |
| 53 | `b8275209e8dc642ca7f974854084280022eba11e` | Applied | 完整集成上游合并 #7162 及父提交关系。映射 M1。 |
| 54 | `30ed40a56a5f4b5ab7b8dd3d685353db3a531c84` | Applied | 完整集成上游合并 #7074 及父提交关系。映射 M1。 |
| 55 | `86f93c28ee34cc74b629dafb748bd5ac5ca8c5ea` | Applied | 完整集成上游合并 #6977 及父提交关系。映射 M1。 |
| 56 | `da74bf13a4099061f71d6816f6e703fb72abf43d` | Applied | 原生 Gemini 模型列表发现 Antigravity 映射；测试补齐本地设置服务参数。映射 M1。 |
| 57 | `881f3202694c6bc932446931a30c27d9675178b9` | Applied + Overridden | 合并版本提交历史，但明确保留本地 VERSION=0.1.239，不采用上游 0.2.5。映射 M1。 |
| 58 | `d7ee1ab6beb618cf413bf7616ecc482251a62157` | Applied | 客户端取消后仍保留 Responses 亲和性绑定，与本地断连排空协调。映射 M1。 |
| 59 | `21532add46331a54d03493871abafc6c9662f309` | Applied | 订单状态筛选变化时重置分页。映射 M1。 |
| 60 | `0ed735d3bae80c6dcf04d329ded055747240ce82` | Applied | 批量代理测试复用进行中请求保护。映射 M1。 |
| 61 | `14636d2f0e674601b4ad77458f7ad8cf658a92d9` | Applied | TOTP 展示归一化 API 错误。映射 M1。 |
| 62 | `6a4938bdfb23606a0d319205ca54cd9a357e5cc0` | Applied | 公告批量已读保留成功项结果。映射 M1。 |
| 63 | `130ba634adbd16aca2303945f0a8fdcbbfef235f` | Applied | 剪贴板降级复制抛错时报告失败。映射 M1。 |
| 64 | `8e34ca5e31ace90f3f9fcf84c207a900bb913158` | Applied | 暂停调度账号仍刷新 OAuth Token，不解除本地人工暂停。映射 M1。 |
| 65 | `18d483c2a10edca7713b52139040007222fc6cc7` | Applied | 分组用量汇总避免全表扫描，保留本地汇总仓储。映射 M1。 |
| 66 | `a9ed2189895fa89c7760701a6d7d63622ab63d3f` | Applied | 兑换历史分页委托本地多次兑换使用明细；105条分页夹具与签到夹具适配，断言保留。映射 M1。 |
| 67 | `2f16e0984ee97ba6c9000745c6613c6d280b7332` | Applied | 避免重复解析 Codex 模型清单。映射 M1。 |
| 68 | `31f3003ff5dfc192a0ba1abb8ac9df6204294950` | Applied | 同时保留模型清单键校验。映射 M1。 |
| 69 | `18bfa4bf20324b36df1885993cd6e16fc85c3c85` | Applied | 严格 Chat 上游的 developer 角色规范化；新增 Kimi 参数兼容交叉回归。映射 M1。 |
| 70 | `1a32b91eb21c022fb8a02bc35bdf707211fcf462` | Applied | 分页跳页输入兼容数值类型。映射 M1。 |
| 71 | `ee9ac3e45fc6226ccd1e824ad2cd5247853553a8` | Applied | TOTP 设置数字输入与状态一致。映射 M1。 |
| 72 | `98321a054ed41c65058966d1d7d705d93dfe902e` | Applied | 非法充值金额输入恢复已接受文本；保留赠送角标。映射 M1。 |
| 73 | `0838e0e6107fc7fb5b64d4ba9a4456120e8031eb` | Applied | 保存前拒绝负数平台配额。映射 M1。 |
| 74 | `f8e5a0d94f811fad399d02bdd97aeefc7dc1a284` | Applied | 空模型输入允许 Tab 离开。映射 M1。 |
| 75 | `fe36f4a9149bedf06fa40b195e144a257bf6c5a3` | Applied | 注册推广码避免加载闪现，保留本地加载容错回归。映射 M1。 |
| 76 | `881ab1b0c6239a928fdccc3c87d2cd011aa337ec` | Applied | DeepSeek Responses 工具输出图片转换。映射 M1。 |
| 77 | `be4a4990ffb960bf797465c01eb448e5a56cf017` | Applied | 清理 Claude 系统提示中的归因元数据，保留本地协议边界。映射 M1。 |
| 78 | `611c30f04fac9081d41bb6f4fa2712d8c7bbe359` | Applied | 合并 grpc 1.83.2 安全升级历史；同步前本地已有该版本，保留更完整安全依赖集。映射 M1。 |
| 79 | `017e9e98d1bb5a752da7d27e8304fdd478a3245b` | Applied | 退款余额告警按请求退款金额计算。映射 M1。 |
| 80 | `be313735dc32f547f220e5592e160f9122f458f2` | Applied | 多实例对话框标题 ID 唯一。映射 M1。 |
| 81 | `b51f0759d4a15bea6071d22387069969f73e3453` | Applied | 支付配置加载等待同一进行中请求。映射 M1。 |
| 82 | `406be7c51888cf8067eb9ceb30dfd340d7eef51b` | Applied | 清空订阅状态时重置加载状态。映射 M1。 |
| 83 | `acc05620cb9ec4e5420c1660f766db7a7d7a1e78` | Applied | DeepSeek 并行工具输出保持相邻，覆盖图片及混合输出。映射 M1。 |
| 84 | `4daabc3d84ddcbaa4df9f74f2f8ac4db6fd157a0` | Applied | 完整集成上游合并 #7234 及父提交关系。映射 M1。 |
| 85 | `131893e449caa529421a08687f48ebf924fdfc44` | Applied | 完整集成上游合并 #7237 及父提交关系。映射 M1。 |
| 86 | `44831bb8549b7ddd327a51329e864d2215fb4748` | Applied | 完整集成上游合并 #7261 及父提交关系。映射 M1。 |
| 87 | `7e4164c1cf6a173ba7faf5c9e5fa595772e0f045` | Applied | 完整集成上游合并 #7207 及父提交关系。映射 M1。 |
| 88 | `d52ab27def5936529d1b59145330446abc4c9373` | Applied | 完整集成上游合并 #7173 及父提交关系。映射 M1。 |
| 89 | `40e7ce7fbf83df9c11b742446aa6d06d5fee04e9` | Applied | 完整集成上游合并 #7265 及父提交关系。映射 M1。 |
| 90 | `ea68c0e82c1c995a8eb797648af72d2088e2955f` | Applied | 完整集成上游合并 #7263 及父提交关系。映射 M1。 |
| 91 | `efd86c63e6724838659cde590a0d4516d045acd0` | Applied | 完整集成上游合并 #7235 及父提交关系。映射 M1。 |
| 92 | `5f75e0d2a7ea7197476be4e77596735c459008f7` | Applied | 完整集成上游合并 #7186 及父提交关系。映射 M1。 |
| 93 | `55bde0857f976f9f1695aa5a0da8bd2531d460b1` | Applied | 完整集成上游合并 #7185 及父提交关系。映射 M1。 |
| 94 | `53fd5f83512efbe95891e16138ecfd57f9bda8b7` | Applied | 完整集成上游合并 #7183 及父提交关系。映射 M1。 |
| 95 | `10fd1ada2801735badec1f04cf1669a784ab7454` | Applied | 完整集成上游合并 #7182 及父提交关系。映射 M1。 |
| 96 | `71c0eb51260fd238b5e485a0127770fe33c8d9d1` | Applied | 完整集成上游合并 #7184 及父提交关系。映射 M1。 |
| 97 | `57e9f6a4b81db65a9bd1c23c9c6cc7965a8e686b` | Applied | 完整集成上游合并 #7238 及父提交关系。映射 M1。 |
| 98 | `ffa43607e50a8f3fbc9f5563f35c2b21805003f3` | Applied | 完整集成上游合并 #7236 及父提交关系。映射 M1。 |
| 99 | `0932e8ce0a6eb0f7aae3c5277ca1c5c1376eec37` | Applied | 完整集成上游合并 #7266 及父提交关系。映射 M1。 |
| 100 | `cd1834e0a948b52b035b7325e6a9efc9e27c967b` | Applied | 完整集成上游合并 #7262 及父提交关系。映射 M1。 |
| 101 | `c84a3d50a3f845fbb826dec00230ade69a59442f` | Applied | 完整集成上游合并 #7215 及父提交关系。映射 M1。 |
| 102 | `50504718228eec88441751f093323496fa1bdf65` | Applied | 完整集成上游合并 #7239 及父提交关系。映射 M1。 |
| 103 | `9efc167482dfe73f3e315b220f38cfd442865827` | Applied | 完整集成上游合并 #7195 及父提交关系。映射 M1。 |
| 104 | `b4e560824a14e9e9093bf974410ecc3825b9e557` | Applied | 完整集成上游合并 #7177 及父提交关系。映射 M1。 |
| 105 | `48a86ec926e2ef8065f1b886de8f3205a3f2029d` | Applied | 完整集成上游合并 #7224 及父提交关系。映射 M1。 |
| 106 | `18328aed05edc7d804d53205470f637216f272bd` | Applied | 完整集成上游合并 #7212 及父提交关系。映射 M1。 |
| 107 | `aeab57b9b21a726d8e55bcc0909bc49523c1b30b` | Applied | 完整集成上游合并 #7256 及父提交关系。映射 M1。 |
| 108 | `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a` | Applied | 完整集成上游合并 #7246 及父提交关系。映射 M1。 |

### 验证结论与失败复测

本地候选相对同步前基线没有未处理的新增失败；这不等于所有检查全绿或真实生产业务已验收。同步前已有的格式、依赖和平台限制继续保留，没有新增豁免或降低门禁。

| 验证项 | 同步前 | 同步后最终结果 |
| --- | --- | --- |
| Go 默认测试、unit、build | 三项退出0 | 三项退出0；早期测试构造不兼容已修复后全量重跑。 |
| WSL 全包 integration | 两次退出1；第二次仅lifecycle受POSIX环境影响，单独补测退出0，其余包通过 | 完整命令退出0，包括兑换历史、CAS、订阅事务、路由和仓储。 |
| golangci-lint | 退出0 | 退出0，0 issues；早期退出7/1保留在逐次表。 |
| 定向竞态 | 流式退出0；lifecycle环境修正后独立全包退出0 | 流式和生命周期两组均退出0，包含指定service/handler/repository及lifecycle用例；不是全仓库race。 |
| 本地交叉回归 | 沿用同步前已有用例 | 后端关键模式、定向编译、前端五文件均退出0；Ollama时钟夹具100轮退出0。 |
| Vue安装、lint、typecheck、build | 全部退出0 | 全部退出0。 |
| Vue全量Vitest | 301文件、2302测试通过 | 321文件、2434测试通过。 |
| Canvas安装、类型检查、测试、build | 全部退出0，8文件/34测试 | 同样全部退出0，8文件/34测试。 |
| Canvas格式 | 退出1，既有package.json格式 | 退出1，同一既有文件；没有顺便格式化。 |
| govulncheck | 退出0 | 退出0，保留grpc 1.83.2安全版本。 |
| frontend生产依赖audit | 退出1：2高危、13中危、1低危 | 退出1，数量及公告集合完全相同；高危为既有xlsx两项。 |
| Canvas生产依赖audit | 退出1：6中危、0高危 | 退出1，数量及公告集合完全相同；不将非零退出码写成通过。 |
| Compose安全/网关/资源、Caddy、脚本语法 | 修正LF归档环境后均退出0 | 全部退出0，配置与生产约束未变。 |
| 远程部署门禁mock | 12项通过 | 12项通过，没有调用真实SSH、Docker或生产配置。 |
| 蓝绿单元与证据/预检 | 48+2+3项通过 | 48+2+3项通过。 |
| Apple专属脚本 | Linux中退出1，不支持BSD stat %Lp | 同样退出1；语法检查通过，macOS原生能力未验证。 |
| 隔离启动及健康 | 退出0 | 退出0，启动重试期间短暂超时/502后健康通过；使用临时PG/Redis，不是生产端点。 |
| 20轮真实应用协议切换、旧版首次迁移、强制退役 | 均退出0 | 均退出0，详情见下方。 |
| POSIX定向环境补验 | 早期两轮退出1 | 修正测试进程GOTMPDIR后退出0。 |
| Wire、依赖整理 | 保持已有基线 | go generate ./cmd/server 与go mod tidy退出0；定向恢复既有生成工具条目，最终go.mod不变；不宣称恢复工具条目后tidy完全无差异。 |

- 环境：Windows及WSL Ubuntu 24.04、Go1.27、Node20.20.2、pnpm9.15.9。Windows禁用CGO，WSL竞态组启用CGO；测试、构建、模块和包管理缓存均位于仓库内忽略目录。工具安装命令、上游变更后的Makefile及脚本先做只读审查；安装使用冻结锁文件、离线和ignore-scripts。无新增迁移或Ent schema，因此未重新生成Ent。
- 环境问题与业务失败分开记录：WSL的DrvFS不满足Unix socket长度和POSIX权限断言；只更改TMPDIR不足以影响Go1.27测试临时目录，最终通过test-exec-v2.sh给测试进程设置仓库内POSIX GOTMPDIR，每次WSL启动检查并创建临时挂载。没有放宽权限断言。
- 同步前早期Linux验证器在运行中被修改，收尾出现EOF错误并误进入forced-retirement参数不足分支（退出2）；之后冻结为linux-final.sh，实际用例与包装器结果分别保留。LF归档最初漏工作流和路径过长，修正归档范围及短路径后原有部署用例通过；没有修改业务脚本以绕过断言。定向PowerShell验证器最初三次因替换字符串转义发生解析失败，修正生成器后正式命令均记录并通过。
- 同步后早期新增失败分别为：注册确认密码禁用变量缺失；Gemini构造/流处理测试参数不匹配；签到分页夹具仍为旧数组；兑换分页夹具未写入本地独立使用表；Windows时钟精度令Ollama新旧重置点相同；一个已修改Go测试的格式问题。均在已批准适配范围内机械修复，原断言保留，相关完整验证和定向复测通过。
- 安全审计按公告ID、模块、严重度及受影响/修复版本区间进行前后集合比较，结果相同。没有用关闭审计、删除锁文件或新增安全例外消除既有告警。

### 蓝绿协议证据

- 同步前报告绑定 `23f34132d5e591007bf8ac8472918fec3af871e5`，同步后报告绑定代码提交 `9dd767d412f9881ef2ecf7891c99ae2823c10d21`。20轮切换前后均为128条mock上游请求对应128条用量记录；普通请求分别16626条与16753条，记录的错误均为0。仅证明本地测试范围，不推断生产账单完整或峰值容量安全。
- 固定旧版二进制来自 `cfd9afa871def9ebd457bb95fb4c2e8b4b02b34f`，前后复用同一隔离旧版。首次迁移检查保持旧实例，迁移后入口租约数为2，已有请求可续接。
- 强制退役独立验证实际Docker退出137、600条旧实例归属清理、活动及其他实例数据保留；告警明确保留“可能中断旧请求，不代表无损退役或用量完整”，未将强制退役包装成无损切换。
- 证据保留于 `output/upstream-sync-20260918-efe9aab1e/artifacts/before-bluegreen-protocol-result.json` 和 `after-bluegreen-protocol-result.json`，完整日志与验证器保留在同级忽略目录，不纳入提交。

### 实际验证命令索引

命令按实际目录及参数去重，下面的每次执行表逐次引用命令编号，保留失败与复测；路径变量仅为缩短记录，可无损展开。

- REPO_WIN = `D:/project/sub2api`，REPO_LINUX = `/mnt/d/project/sub2api`。
- RUN_WIN/ RUN_LINUX 分别为上述根目录下的 `output/upstream-sync-20260918-efe9aab1e`。
- GO_WIN = `C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe`；WSL 的 `go` 来自 `output/upstream-sync-20260910-98d86915b/tools/linux/go/bin`。
- Windows 命令使用程序及独立参数列表的引号表示法；WSL 命令保留执行器记录的 shell 转义。详细原始参数、开始/结束时间、退出码和日志路径仍保存在同目录的 `*-results.jsonl`，不覆盖初次失败。

| 编号 | 实际工作目录 | 实际命令及参数 |
| --- | --- | --- |
| V001 | `$REPO_WIN/backend` | `"$GO_WIN" "test" "./..." "-count=1" "-timeout=20m"` |
| V002 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "install" "--frozen-lockfile" "--offline" "--ignore-scripts"` |
| V003 | `$REPO_WIN/canvas` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "install" "--frozen-lockfile" "--offline" "--ignore-scripts"` |
| V004 | `$REPO_WIN/backend` | `"$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/go-tools/bin/golangci-lint.exe" "run" "./..." "--timeout=30m"` |
| V005 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "run" "lint:check"` |
| V006 | `$REPO_WIN/canvas` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "node_modules/prettier/bin/prettier.cjs" "--check" "."` |
| V007 | `$REPO_WIN/canvas` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "node_modules/typescript/bin/tsc" "--noEmit"` |
| V008 | `$REPO_WIN/canvas` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "node_modules/vitest/vitest.mjs" "run" "--maxWorkers=4" "--minWorkers=1"` |
| V009 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "run" "typecheck"` |
| V010 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "run" "test:run" "--maxWorkers=4" "--minWorkers=1"` |
| V011 | `$REPO_LINUX` | `bash deploy/tests/docker-compose-security-test.sh` |
| V012 | `$REPO_LINUX` | `bash deploy/tests/docker-compose-gateway-env-test.sh` |
| V013 | `$REPO_LINUX/backend` | `go test -tags=integration ./... -count=1 -timeout=25m` |
| V014 | `$REPO_LINUX` | `bash deploy/tests/docker-runtime-resources-test.sh` |
| V015 | `$REPO_LINUX` | `bash deploy/tests/remote-deploy-test.sh` |
| V016 | `$REPO_LINUX` | `bash deploy/tests/apple-container-test.sh` |
| V017 | `$REPO_LINUX` | `bash deploy/test-caddyfile-cache.sh` |
| V018 | `$REPO_LINUX` | `bash -n deploy/apple-container.sh` |
| V019 | `$REPO_LINUX` | `sh -n deploy/remote-deploy.sh` |
| V020 | `$REPO_LINUX` | `python3 deploy/tests/blue-green-test.py` |
| V021 | `$REPO_LINUX` | `python3 deploy/tests/blue-green-evidence-test.py` |
| V022 | `$REPO_LINUX` | `python3 deploy/tests/blue-green-preflight-test.py` |
| V023 | `$REPO_WIN/backend` | `"$GO_WIN" "test" "-tags=unit" "./..." "-count=1" "-timeout=20m"` |
| V024 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "run" "build"` |
| V025 | `$RUN_LINUX/before-static-lf` | `bash deploy/tests/docker-compose-security-test.sh` |
| V026 | `$RUN_LINUX/before-static-lf` | `bash deploy/tests/docker-compose-gateway-env-test.sh` |
| V027 | `$REPO_LINUX/backend` | `go test -race -tags=unit -count=1 -timeout=20m -run TestKimi\\|TestOpsErrorLoggerMiddleware_Kimi\\|TestAnthropicStreamSafeRetry\\|TestFirstTokenCleanup\\|TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeout\\|TestDecompressResponseBodyStreamClose\\|TestDecompressedBodyClose ./internal/service ./internal/handler ./internal/repository` |
| V028 | `$REPO_WIN/canvas` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "run" "build"` |
| V029 | `$RUN_LINUX/before-static-lf` | `bash deploy/tests/docker-runtime-resources-test.sh` |
| V030 | `$RUN_LINUX/before-static-lf` | `bash deploy/tests/remote-deploy-test.sh` |
| V031 | `$RUN_LINUX/before-static-lf` | `bash deploy/tests/apple-container-test.sh` |
| V032 | `$RUN_LINUX/before-static-lf` | `bash deploy/test-caddyfile-cache.sh` |
| V033 | `$RUN_LINUX/before-static-lf` | `bash -n deploy/apple-container.sh` |
| V034 | `$RUN_LINUX/before-static-lf` | `sh -n deploy/remote-deploy.sh` |
| V035 | `$RUN_LINUX/before-static-lf` | `python3 deploy/tests/blue-green-test.py` |
| V036 | `$RUN_LINUX/before-static-lf` | `python3 deploy/tests/blue-green-evidence-test.py` |
| V037 | `$RUN_LINUX/before-static-lf` | `python3 deploy/tests/blue-green-preflight-test.py` |
| V038 | `内部 trap（沿用 smoke 工作目录）` | `smoke.sh before` |
| V039 | `$REPO_LINUX/backend` | `bash $RUN_LINUX/smoke.sh before` |
| V040 | `$REPO_LINUX/backend` | `go build -o $RUN_LINUX/artifacts/before-server-linux ./cmd/server` |
| V041 | `$REPO_WIN/backend` | `"$GO_WIN" "build" "./..."` |
| V042 | `$REPO_LINUX` | `go -C $RUN_LINUX/legacy-source/backend build -o $RUN_LINUX/artifacts/legacy-server-linux ./cmd/server` |
| V043 | `$REPO_WIN/backend` | `"$REPO_WIN/output/upstream-sync-20260915-bdb42e22f/tools/govulncheck.exe" "./..."` |
| V044 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "audit" "--prod" "--audit-level=high" "--json"` |
| V045 | `$REPO_WIN/canvas` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "audit" "--prod" "--audit-level=high" "--json"` |
| V046 | `$REPO_LINUX/backend` | `go test -race -tags=unit -count=1 -timeout=20m -run TestPeer\\|TestAffinity\\|TestPublicPeer\\|TestLongSSE\\|TestHijacked\\|TestDetachedProducer\\|TestControl\\|TestDrain\\|TestNewInstance\\|TestLegacyWaits\\|TestRenewal\\|TestUsageRecordWorkerPool_StopRace\\|TestStoppedKeyed\\|TestOpenAIResponsesWebSocket_Ingress\\|OllamaCloud\\|Idempotency\\|Subscription ./internal/pkg/lifecycle ./internal/service ./internal/repository ./internal/handler` |
| V047 | `$REPO_LINUX` | `timeout 25m python3 deploy/tests/blue-green-integration.py --binary $RUN_LINUX/artifacts/before-server-linux --legacy-binary $RUN_LINUX/artifacts/legacy-server-linux --cycles 20` |
| V048 | `$REPO_LINUX/backend` | `timeout 5m python3 deploy/tests/blue-green-force-integration.py` |
| V049 | `$RUN_LINUX/before-static-lf` | `python3 $REPO_LINUX/deploy/tests/blue-green-test.py` |
| V050 | `$RUN_LINUX/before-static-lf` | `python3 $REPO_LINUX/deploy/tests/blue-green-evidence-test.py` |
| V051 | `$RUN_LINUX/before-static-lf` | `python3 $REPO_LINUX/deploy/tests/blue-green-preflight-test.py` |
| V052 | `$REPO_LINUX` | `timeout 5m python3 deploy/tests/blue-green-force-integration.py` |
| V053 | `$REPO_LINUX/backend` | `go run $RUN_LINUX/envcheck.go` |
| V054 | `$REPO_LINUX/backend` | `go test -tags=integration ./internal/pkg/lifecycle ./internal/service -run \^\(TestPeer\\|TestControl\\|TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool\)\$ -count=1 -timeout=5m` |
| V055 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec.sh -tags=integration ./internal/pkg/lifecycle ./internal/service -run \^\(TestPeer\\|TestControl\\|TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool\)\$ -count=1 -timeout=5m` |
| V056 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec-v2.sh -tags=integration ./internal/pkg/lifecycle -count=1 -timeout=5m` |
| V057 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec-v2.sh -race -tags=unit ./internal/pkg/lifecycle -count=1 -timeout=5m` |
| V058 | `$REPO_WIN/backend` | `"$GO_WIN" "mod" "tidy"` |
| V059 | `$REPO_WIN/backend` | `"$GO_WIN" "generate" "./cmd/server"` |
| V060 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec-v2.sh -tags=integration ./... -count=1 -timeout=25m` |
| V061 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec-v2.sh -race -tags=unit -count=1 -timeout=20m -run TestKimi\\|TestOpsErrorLoggerMiddleware_Kimi\\|TestAnthropicStreamSafeRetry\\|TestFirstTokenCleanup\\|TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeout\\|TestDecompressResponseBodyStreamClose\\|TestDecompressedBodyClose ./internal/service ./internal/handler ./internal/repository` |
| V062 | `$REPO_WIN/backend` | `"$GO_WIN" "test" "-tags=unit" "./internal/service" "./internal/handler" "./internal/server" "-run" "Kimi\|AnthropicStream\|FirstToken\|FailureStreak\|ManualSchedul\|Ollama\|Codex.*Quota\|OpenAI.*Schedul\|Deepseek\|DeepSeek\|Namespace\|StrictChat\|Billing\|Pricing\|Redeem\|Subscription\|Idempotency\|OpenAIWS\|APIContracts\|TemporaryCredit\|BalanceQuery" "-count=1" "-timeout=15m"` |
| V063 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec-v2.sh -race -tags=unit -count=1 -timeout=20m -run TestPeer\\|TestAffinity\\|TestPublicPeer\\|TestLongSSE\\|TestHijacked\\|TestDetachedProducer\\|TestControl\\|TestDrain\\|TestNewInstance\\|TestLegacyWaits\\|TestRenewal\\|TestUsageRecordWorkerPool_StopRace\\|TestStoppedKeyed\\|TestOpenAIResponsesWebSocket_Ingress\\|OllamaCloud\\|Idempotency\\|Subscription ./internal/pkg/lifecycle ./internal/service ./internal/repository ./internal/handler` |
| V064 | `$REPO_WIN/frontend` | `"$REPO_WIN/output/upstream-sync-20260910-98d86915b/tools/node-v20.20.2-win-x64/node.exe" "$REPO_WIN/output/upstream-sync-20260905-ab99d56e9/pnpm-9.15.9/package/bin/pnpm.cjs" "exec" "vitest" "run" "src/views/auth/__tests__/RegisterView.spec.ts" "src/views/user/__tests__/RedeemView.spec.ts" "src/views/user/__tests__/KeysView.spec.ts" "src/components/payment/__tests__/AmountInput.spec.ts" "src/views/admin/__tests__/ChannelMonitorView.grok.spec.ts" "--maxWorkers=4" "--minWorkers=1"` |
| V065 | `$REPO_WIN/backend` | `"$GO_WIN" "test" "-tags=unit" "./internal/service" "./internal/handler" "./internal/server" "-run" "^$" "-count=1"` |
| V066 | `$REPO_WIN/backend` | `"$GO_WIN" "test" "-tags=unit" "./internal/service" "-run" "^TestOllamaProbeCallback_StaleLongDoesNotOverrideNewShort$" "-count=100" "-timeout=5m"` |
| V067 | `内部 trap（沿用 smoke 工作目录）` | `smoke.sh after` |
| V068 | `$REPO_LINUX/backend` | `bash $RUN_LINUX/smoke.sh after` |
| V069 | `$RUN_LINUX/after-static-lf` | `bash deploy/tests/docker-compose-security-test.sh` |
| V070 | `$RUN_LINUX/after-static-lf` | `bash deploy/tests/docker-compose-gateway-env-test.sh` |
| V071 | `$RUN_LINUX/after-static-lf` | `bash deploy/tests/docker-runtime-resources-test.sh` |
| V072 | `$RUN_LINUX/after-static-lf` | `bash deploy/tests/remote-deploy-test.sh` |
| V073 | `$RUN_LINUX/after-static-lf` | `bash deploy/tests/apple-container-test.sh` |
| V074 | `$RUN_LINUX/after-static-lf` | `bash deploy/test-caddyfile-cache.sh` |
| V075 | `$RUN_LINUX/after-static-lf` | `bash -n deploy/apple-container.sh` |
| V076 | `$RUN_LINUX/after-static-lf` | `sh -n deploy/remote-deploy.sh` |
| V077 | `$RUN_LINUX/after-static-lf` | `python3 $REPO_LINUX/deploy/tests/blue-green-test.py` |
| V078 | `$RUN_LINUX/after-static-lf` | `python3 $REPO_LINUX/deploy/tests/blue-green-evidence-test.py` |
| V079 | `$RUN_LINUX/after-static-lf` | `python3 $REPO_LINUX/deploy/tests/blue-green-preflight-test.py` |
| V080 | `$REPO_LINUX/backend` | `go build -o $RUN_LINUX/artifacts/after-server-linux ./cmd/server` |
| V081 | `$REPO_LINUX` | `timeout 25m python3 deploy/tests/blue-green-integration.py --binary $RUN_LINUX/artifacts/after-server-linux --legacy-binary $RUN_LINUX/artifacts/legacy-server-linux --cycles 20` |
| V082 | `$REPO_LINUX/backend` | `go test -exec $RUN_LINUX/test-exec-v2.sh -tags=integration ./internal/pkg/lifecycle ./internal/service -run \^\(TestPeer\\|TestControl\\|TestOpenAIFirstOutputStageOverflowIsAtomicAndCleanupRemovesSpool\)\$ -count=1 -timeout=5m` |

### 每次执行及退出码

下列时间均带时区；日志名相对 RUN 目录。内部 smoke 行是清理 trap 的单独记录，不额外计算为一次健康用例。

| 开始时间 | 阶段及验证项 | 命令编号 | 退出码 | 日志或结果文件 |
| --- | --- | --- | --- | --- |
| 2026-09-18T02:08:28.8282545+08:00 | 同步前/backend/test-default | V001 | 0 | `logs/before-backend-test-default-020828827.log` |
| 2026-09-18T02:08:29.2702256+08:00 | 同步前/frontend/install | V002 | 0 | `logs/before-frontend-install-020829269.log` |
| 2026-09-18T02:08:29.7013651+08:00 | 同步前/canvas/install | V003 | 0 | `logs/before-canvas-install-020829700.log` |
| 2026-09-18T02:08:30.0376158+08:00 | 同步前/extra/golangci-lint | V004 | 0 | `logs/before-extra-golangci-lint-020830037.log` |
| 2026-09-18T02:08:33.8004500+08:00 | 同步前/frontend/lint | V005 | 0 | `logs/before-frontend-lint-020833800.log` |
| 2026-09-18T02:08:33.8720746+08:00 | 同步前/canvas/format | V006 | 1 | `logs/before-canvas-format-020833871.log` |
| 2026-09-18T02:08:44.0717471+08:00 | 同步前/canvas/typecheck | V007 | 0 | `logs/before-canvas-typecheck-020844071.log` |
| 2026-09-18T02:09:05.6819770+08:00 | 同步前/canvas/test | V008 | 0 | `logs/before-canvas-test-020905681.log` |
| 2026-09-18T02:11:11.4188727+08:00 | 同步前/frontend/typecheck | V009 | 0 | `logs/before-frontend-typecheck-021111417.log` |
| 2026-09-18T02:12:22.5223591+08:00 | 同步前/frontend/test | V010 | 0 | `logs/before-frontend-test-021222522.log` |
| 2026-09-18T02:12:32+08:00 | 同步前/static/docker-compose-security-test | V011 | 0 | `logs/before-static-docker-compose-security-test-021232.log` |
| 2026-09-18T02:12:32+08:00 | 同步前/static/docker-compose-gateway-env-test | V012 | 1 | `logs/before-static-docker-compose-gateway-env-test-021232.log` |
| 2026-09-18T02:12:32+08:00 | 同步前/integration/integration | V013 | 1 | `logs/before-integration-integration-021232.log` |
| 2026-09-18T02:12:36+08:00 | 同步前/static/docker-runtime-resources-test | V014 | 1 | `logs/before-static-docker-runtime-resources-test-021236.log` |
| 2026-09-18T02:12:36+08:00 | 同步前/static/remote-deploy-test | V015 | 0 | `logs/before-static-remote-deploy-test-021236.log` |
| 2026-09-18T02:12:38+08:00 | 同步前/static/apple-container-test | V016 | 1 | `logs/before-static-apple-container-test-021238.log` |
| 2026-09-18T02:12:39+08:00 | 同步前/static/caddy-cache | V017 | 1 | `logs/before-static-caddy-cache-021239.log` |
| 2026-09-18T02:12:39+08:00 | 同步前/static/apple-syntax | V018 | 0 | `logs/before-static-apple-syntax-021239.log` |
| 2026-09-18T02:12:39+08:00 | 同步前/static/remote-syntax | V019 | 0 | `logs/before-static-remote-syntax-021239.log` |
| 2026-09-18T02:12:39+08:00 | 同步前/static/blue-green-test | V020 | 1 | `logs/before-static-blue-green-test-021239.log` |
| 2026-09-18T02:12:59+08:00 | 同步前/static/blue-green-evidence-test | V021 | 0 | `logs/before-static-blue-green-evidence-test-021259.log` |
| 2026-09-18T02:13:00+08:00 | 同步前/static/blue-green-preflight-test | V022 | 0 | `logs/before-static-blue-green-preflight-test-021300.log` |
| 2026-09-18T02:14:26.9615480+08:00 | 同步前/backend/test-unit | V023 | 0 | `logs/before-backend-test-unit-021426961.log` |
| 2026-09-18T02:17:21.2447423+08:00 | 同步前/frontend/build | V024 | 0 | `logs/before-frontend-build-021721244.log` |
| 2026-09-18T02:17:42+08:00 | 同步前/static-posix/docker-compose-security-test | V025 | 0 | `logs/before-static-posix-docker-compose-security-test-021742.log` |
| 2026-09-18T02:17:42+08:00 | 同步前/static-posix/docker-compose-gateway-env-test | V026 | 0 | `logs/before-static-posix-docker-compose-gateway-env-test-021742.log` |
| 2026-09-18T02:17:42+08:00 | 同步前/race/stream-race | V027 | 0 | `logs/before-race-stream-race-021742.log` |
| 2026-09-18T02:17:42.6264174+08:00 | 同步前/canvas-build/build | V028 | 0 | `logs/before-canvas-build-build-021742625.log` |
| 2026-09-18T02:17:47+08:00 | 同步前/static-posix/docker-runtime-resources-test | V029 | 0 | `logs/before-static-posix-docker-runtime-resources-test-021747.log` |
| 2026-09-18T02:17:47+08:00 | 同步前/static-posix/remote-deploy-test | V030 | 0 | `logs/before-static-posix-remote-deploy-test-021747.log` |
| 2026-09-18T02:17:51+08:00 | 同步前/static-posix/apple-container-test | V031 | 1 | `logs/before-static-posix-apple-container-test-021751.log` |
| 2026-09-18T02:17:51+08:00 | 同步前/static-posix/caddy-cache | V032 | 0 | `logs/before-static-posix-caddy-cache-021751.log` |
| 2026-09-18T02:17:51+08:00 | 同步前/static-posix/apple-syntax | V033 | 0 | `logs/before-static-posix-apple-syntax-021751.log` |
| 2026-09-18T02:17:51+08:00 | 同步前/static-posix/remote-syntax | V034 | 0 | `logs/before-static-posix-remote-syntax-021751.log` |
| 2026-09-18T02:17:51+08:00 | 同步前/static-posix/blue-green-test | V035 | 1 | `logs/before-static-posix-blue-green-test-021751.log` |
| 2026-09-18T02:18:24+08:00 | 同步前/static-posix/blue-green-evidence-test | V036 | 0 | `logs/before-static-posix-blue-green-evidence-test-021824.log` |
| 2026-09-18T02:18:24+08:00 | 同步前/static-posix/blue-green-preflight-test | V037 | 0 | `logs/before-static-posix-blue-green-preflight-test-021824.log` |
| 2026-09-18T02:19:11+08:00 | 同步前/smoke-linux/内部收尾 | V038 | 0 | `before-smoke-results.jsonl` |
| 2026-09-18T02:19:11+08:00 | 同步前/smoke/health | V039 | 0 | `logs/before-smoke-health-021911.log` |
| 2026-09-18T02:19:12+08:00 | 同步前/protocol/build | V040 | 0 | `logs/before-protocol-build-021912.log` |
| 2026-09-18T02:20:09.0392543+08:00 | 同步前/backend/build | V041 | 0 | `logs/before-backend-build-022009039.log` |
| 2026-09-18T02:21:59+08:00 | 同步前/protocol/legacy-build | V042 | 0 | `logs/before-protocol-legacy-build-022159.log` |
| 2026-09-18T02:23:27+08:00 | 同步前/static-posix/docker-compose-security-test | V025 | 0 | `logs/before-static-posix-docker-compose-security-test-022327.log` |
| 2026-09-18T02:23:27+08:00 | 同步前/static-posix/docker-compose-gateway-env-test | V026 | 0 | `logs/before-static-posix-docker-compose-gateway-env-test-022327.log` |
| 2026-09-18T02:23:27.2462068+08:00 | 同步前/security/govulncheck | V043 | 0 | `logs/before-security-govulncheck-022327245.log` |
| 2026-09-18T02:23:32+08:00 | 同步前/static-posix/docker-runtime-resources-test | V029 | 0 | `logs/before-static-posix-docker-runtime-resources-test-022332.log` |
| 2026-09-18T02:23:32+08:00 | 同步前/static-posix/remote-deploy-test | V030 | 0 | `logs/before-static-posix-remote-deploy-test-022332.log` |
| 2026-09-18T02:23:37+08:00 | 同步前/static-posix/apple-container-test | V031 | 1 | `logs/before-static-posix-apple-container-test-022337.log` |
| 2026-09-18T02:23:38+08:00 | 同步前/static-posix/caddy-cache | V032 | 0 | `logs/before-static-posix-caddy-cache-022338.log` |
| 2026-09-18T02:23:38+08:00 | 同步前/static-posix/apple-syntax | V033 | 0 | `logs/before-static-posix-apple-syntax-022338.log` |
| 2026-09-18T02:23:38+08:00 | 同步前/static-posix/remote-syntax | V034 | 0 | `logs/before-static-posix-remote-syntax-022338.log` |
| 2026-09-18T02:23:38+08:00 | 同步前/static-posix/blue-green-test | V035 | 1 | `logs/before-static-posix-blue-green-test-022338.log` |
| 2026-09-18T02:23:42.4774461+08:00 | 同步前/security/frontend-audit | V044 | 1 | `logs/before-security-frontend-audit-022342477.log` |
| 2026-09-18T02:23:47.0600023+08:00 | 同步前/security/canvas-audit | V045 | 1 | `logs/before-security-canvas-audit-022347059.log` |
| 2026-09-18T02:24:12+08:00 | 同步前/static-posix/blue-green-evidence-test | V036 | 0 | `logs/before-static-posix-blue-green-evidence-test-022412.log` |
| 2026-09-18T02:24:12+08:00 | 同步前/static-posix/blue-green-preflight-test | V037 | 0 | `logs/before-static-posix-blue-green-preflight-test-022412.log` |
| 2026-09-18T02:25:01+08:00 | 同步前/race/lifecycle-race | V046 | 1 | `logs/before-race-lifecycle-race-022501.log` |
| 2026-09-18T02:25:46+08:00 | 同步前/static-posix/docker-compose-security-test | V025 | 0 | `logs/before-static-posix-docker-compose-security-test-022546.log` |
| 2026-09-18T02:25:47+08:00 | 同步前/static-posix/docker-compose-gateway-env-test | V026 | 0 | `logs/before-static-posix-docker-compose-gateway-env-test-022547.log` |
| 2026-09-18T02:25:50+08:00 | 同步前/static-posix/docker-runtime-resources-test | V029 | 0 | `logs/before-static-posix-docker-runtime-resources-test-022550.log` |
| 2026-09-18T02:25:51+08:00 | 同步前/static-posix/remote-deploy-test | V030 | 0 | `logs/before-static-posix-remote-deploy-test-022551.log` |
| 2026-09-18T02:25:54+08:00 | 同步前/static-posix/apple-container-test | V031 | 1 | `logs/before-static-posix-apple-container-test-022554.log` |
| 2026-09-18T02:25:54+08:00 | 同步前/static-posix/caddy-cache | V032 | 0 | `logs/before-static-posix-caddy-cache-022554.log` |
| 2026-09-18T02:25:54+08:00 | 同步前/static-posix/apple-syntax | V033 | 0 | `logs/before-static-posix-apple-syntax-022554.log` |
| 2026-09-18T02:25:55+08:00 | 同步前/static-posix/remote-syntax | V034 | 0 | `logs/before-static-posix-remote-syntax-022555.log` |
| 2026-09-18T02:25:55+08:00 | 同步前/static-posix/blue-green-test | V035 | 1 | `logs/before-static-posix-blue-green-test-022555.log` |
| 2026-09-18T02:25:58+08:00 | 同步前/protocol/twenty-switches | V047 | 0 | `logs/before-protocol-twenty-switches-022558.log` |
| 2026-09-18T02:26:17+08:00 | 同步前/static-posix/blue-green-evidence-test | V036 | 0 | `logs/before-static-posix-blue-green-evidence-test-022617.log` |
| 2026-09-18T02:26:18+08:00 | 同步前/static-posix/blue-green-preflight-test | V037 | 0 | `logs/before-static-posix-blue-green-preflight-test-022618.log` |
| 2026-09-18T02:27:03+08:00 | 同步前/integration/forced-retirement | V048 | 2 | `logs/before-integration-forced-retirement-022703.log` |
| 2026-09-18T02:28:35+08:00 | 同步前/race/stream-race | V027 | 0 | `logs/before-race-stream-race-022835.log` |
| 2026-09-18T02:28:36+08:00 | 同步前/static-posix/docker-compose-security-test | V025 | 0 | `logs/before-static-posix-docker-compose-security-test-022836.log` |
| 2026-09-18T02:28:36+08:00 | 同步前/integration/integration | V013 | 1 | `logs/before-integration-integration-022836.log` |
| 2026-09-18T02:28:37+08:00 | 同步前/static-posix/docker-compose-gateway-env-test | V026 | 0 | `logs/before-static-posix-docker-compose-gateway-env-test-022837.log` |
| 2026-09-18T02:28:40+08:00 | 同步前/static-posix/docker-runtime-resources-test | V029 | 0 | `logs/before-static-posix-docker-runtime-resources-test-022840.log` |
| 2026-09-18T02:28:40+08:00 | 同步前/static-posix/remote-deploy-test | V030 | 0 | `logs/before-static-posix-remote-deploy-test-022840.log` |
| 2026-09-18T02:28:43+08:00 | 同步前/static-posix/apple-container-test | V031 | 1 | `logs/before-static-posix-apple-container-test-022843.log` |
| 2026-09-18T02:28:43+08:00 | 同步前/static-posix/caddy-cache | V032 | 0 | `logs/before-static-posix-caddy-cache-022843.log` |
| 2026-09-18T02:28:43+08:00 | 同步前/static-posix/apple-syntax | V033 | 0 | `logs/before-static-posix-apple-syntax-022843.log` |
| 2026-09-18T02:28:43+08:00 | 同步前/static-posix/remote-syntax | V034 | 0 | `logs/before-static-posix-remote-syntax-022843.log` |
| 2026-09-18T02:28:43+08:00 | 同步前/static-posix/blue-green-test | V049 | 0 | `logs/before-static-posix-blue-green-test-022843.log` |
| 2026-09-18T02:28:44+08:00 | 同步前/static-posix/blue-green-evidence-test | V050 | 0 | `logs/before-static-posix-blue-green-evidence-test-022844.log` |
| 2026-09-18T02:28:44+08:00 | 同步前/static-posix/blue-green-preflight-test | V051 | 0 | `logs/before-static-posix-blue-green-preflight-test-022844.log` |
| 2026-09-18T02:30:40+08:00 | 同步前/race/lifecycle-race | V046 | 1 | `logs/before-race-lifecycle-race-023040.log` |
| 2026-09-18T02:31:04+08:00 | 同步前/protocol/forced-retirement | V052 | 0 | `logs/before-protocol-forced-retirement-023104.log` |
| 2026-09-18T02:35:48+08:00 | 同步前/environment/environment | V053 | 0 | `logs/before-environment-environment-023548.log` |
| 2026-09-18T02:35:53+08:00 | 同步前/environment/posix-targets | V054 | 1 | `logs/before-environment-posix-targets-023553.log` |
| 2026-09-18T02:38:00+08:00 | 同步前/environment/environment | V053 | 0 | `logs/before-environment-environment-023800.log` |
| 2026-09-18T02:38:05+08:00 | 同步前/environment/posix-targets | V055 | 1 | `logs/before-environment-posix-targets-023805.log` |
| 2026-09-18T02:39:00+08:00 | 同步前/posix-lifecycle/integration-lifecycle | V056 | 1 | `logs/before-posix-lifecycle-integration-lifecycle-023900.log` |
| 2026-09-18T02:39:15+08:00 | 同步前/posix-lifecycle/lifecycle-race | V057 | 1 | `logs/before-posix-lifecycle-lifecycle-race-023915.log` |
| 2026-09-18T02:40:31+08:00 | 同步前/posix-lifecycle/integration-lifecycle | V056 | 0 | `logs/before-posix-lifecycle-integration-lifecycle-024031.log` |
| 2026-09-18T02:40:49+08:00 | 同步前/posix-lifecycle/lifecycle-race | V057 | 0 | `logs/before-posix-lifecycle-lifecycle-race-024049.log` |
| 2026-09-18T02:44:49.0850439+08:00 | 同步后/generate/tidy | V058 | 0 | `logs/after-generate-tidy-024449084.log` |
| 2026-09-18T02:44:49.4388334+08:00 | 同步后/frontend/install | V002 | 0 | `logs/after-frontend-install-024449438.log` |
| 2026-09-18T02:44:49.7217594+08:00 | 同步后/canvas/install | V003 | 0 | `logs/after-canvas-install-024449721.log` |
| 2026-09-18T02:44:54.0608087+08:00 | 同步后/frontend/lint | V005 | 0 | `logs/after-frontend-lint-024454059.log` |
| 2026-09-18T02:44:55.1840616+08:00 | 同步后/canvas/format | V006 | 1 | `logs/after-canvas-format-024455183.log` |
| 2026-09-18T02:45:03.2713649+08:00 | 同步后/canvas/typecheck | V007 | 0 | `logs/after-canvas-typecheck-024503271.log` |
| 2026-09-18T02:45:21.3546648+08:00 | 同步后/canvas/test | V008 | 0 | `logs/after-canvas-test-024521354.log` |
| 2026-09-18T02:46:25.1339336+08:00 | 同步后/frontend/typecheck | V009 | 2 | `logs/after-frontend-typecheck-024625133.log` |
| 2026-09-18T02:47:06.9761444+08:00 | 同步后/frontend/test | V010 | 1 | `logs/after-frontend-test-024706975.log` |
| 2026-09-18T02:47:42.3564126+08:00 | 同步后/generate/wire | V059 | 0 | `logs/after-generate-wire-024742355.log` |
| 2026-09-18T02:49:20.4529823+08:00 | 同步后/backend/test-default | V001 | 1 | `logs/after-backend-test-default-024920452.log` |
| 2026-09-18T02:49:21.0802630+08:00 | 同步后/extra/golangci-lint | V004 | 7 | `logs/after-extra-golangci-lint-024921079.log` |
| 2026-09-18T02:49:29+08:00 | 同步后/integration/integration | V060 | 1 | `logs/after-integration-integration-024929.log` |
| 2026-09-18T02:50:53.9415807+08:00 | 同步后/backend/test-unit | V023 | 1 | `logs/after-backend-test-unit-025053941.log` |
| 2026-09-18T02:52:00.4420240+08:00 | 同步后/frontend/build | V024 | 0 | `logs/after-frontend-build-025200441.log` |
| 2026-09-18T02:52:28.1686816+08:00 | 同步后/backend/build | V041 | 0 | `logs/after-backend-build-025228168.log` |
| 2026-09-18T02:55:01+08:00 | 同步后/race/stream-race | V061 | 1 | `logs/after-race-stream-race-025501.log` |
| 2026-09-18T02:55:01.3277983+08:00 | 同步后/backend/test-default | V001 | 1 | `logs/after-backend-test-default-025501327.log` |
| 2026-09-18T02:55:01.3980424+08:00 | 同步后/extra/golangci-lint | V004 | 1 | `logs/after-extra-golangci-lint-025501397.log` |
| 2026-09-18T02:55:42.1872159+08:00 | 同步后/frontend/install | V002 | 0 | `logs/after-frontend-install-025542186.log` |
| 2026-09-18T02:55:42.5986010+08:00 | 同步后/backend/sync-critical | V062 | 1 | `logs/after-backend-sync-critical-025542597.log` |
| 2026-09-18T02:55:42.9585777+08:00 | 同步后/security/govulncheck | V043 | 0 | `logs/after-security-govulncheck-025542957.log` |
| 2026-09-18T02:55:46.5057716+08:00 | 同步后/frontend/lint | V005 | 0 | `logs/after-frontend-lint-025546505.log` |
| 2026-09-18T02:55:59.0019963+08:00 | 同步后/security/frontend-audit | V044 | 1 | `logs/after-security-frontend-audit-025559001.log` |
| 2026-09-18T02:56:03.1630464+08:00 | 同步后/security/canvas-audit | V045 | 1 | `logs/after-security-canvas-audit-025603162.log` |
| 2026-09-18T02:57:48.0796269+08:00 | 同步后/frontend/typecheck | V009 | 0 | `logs/after-frontend-typecheck-025748079.log` |
| 2026-09-18T02:58:31.5878940+08:00 | 同步后/frontend/test | V010 | 0 | `logs/after-frontend-test-025831587.log` |
| 2026-09-18T02:58:34.6484094+08:00 | 同步后/backend/test-unit | V023 | 1 | `logs/after-backend-test-unit-025834648.log` |
| 2026-09-18T03:00:42+08:00 | 同步后/race/lifecycle-race | V063 | 1 | `logs/after-race-lifecycle-race-030042.log` |
| 2026-09-18T03:02:14.7470964+08:00 | 同步后/frontend/build | V024 | 0 | `logs/after-frontend-build-030214746.log` |
| 2026-09-18T03:02:36.0867144+08:00 | 同步后/backend/build | V041 | 0 | `logs/after-backend-build-030236086.log` |
| 2026-09-18T03:07:03.8006560+08:00 | 同步后/frontend/sync-critical | V064 | 0 | `logs/after-frontend-sync-critical-030703800.log` |
| 2026-09-18T03:07:03.8257559+08:00 | 同步后/backend/compile | V065 | 0 | `logs/after-backend-compile-030703825.log` |
| 2026-09-18T03:07:04.1086340+08:00 | 同步后/canvas-build/build | V028 | 0 | `logs/after-canvas-build-build-030704108.log` |
| 2026-09-18T03:07:17+08:00 | 同步后/integration/integration | V060 | 0 | `logs/after-integration-integration-030717.log` |
| 2026-09-18T03:08:21+08:00 | 同步后/race/stream-race | V061 | 0 | `logs/after-race-stream-race-030821.log` |
| 2026-09-18T03:08:21.8016957+08:00 | 同步后/backend/ollama-clock | V066 | 0 | `logs/after-backend-ollama-clock-030821801.log` |
| 2026-09-18T03:08:21.8023162+08:00 | 同步后/extra/golangci-lint | V004 | 0 | `logs/after-extra-golangci-lint-030821801.log` |
| 2026-09-18T03:08:21.8031607+08:00 | 同步后/backend/test-default | V001 | 0 | `logs/after-backend-test-default-030821802.log` |
| 2026-09-18T03:10:29.8506215+08:00 | 同步后/backend/sync-critical | V062 | 0 | `logs/after-backend-sync-critical-031029850.log` |
| 2026-09-18T03:11:06+08:00 | 同步后/race/lifecycle-race | V063 | 0 | `logs/after-race-lifecycle-race-031106.log` |
| 2026-09-18T03:11:30.8834680+08:00 | 同步后/backend/test-unit | V023 | 0 | `logs/after-backend-test-unit-031130883.log` |
| 2026-09-18T03:13:20+08:00 | 同步后/smoke-linux/内部收尾 | V067 | 0 | `after-smoke-results.jsonl` |
| 2026-09-18T03:13:20+08:00 | 同步后/smoke/health | V068 | 0 | `logs/after-smoke-health-031320.log` |
| 2026-09-18T03:13:23+08:00 | 同步后/static-posix/docker-compose-security-test | V069 | 0 | `logs/after-static-posix-docker-compose-security-test-031323.log` |
| 2026-09-18T03:13:23+08:00 | 同步后/static-posix/docker-compose-gateway-env-test | V070 | 0 | `logs/after-static-posix-docker-compose-gateway-env-test-031323.log` |
| 2026-09-18T03:13:27+08:00 | 同步后/static-posix/docker-runtime-resources-test | V071 | 0 | `logs/after-static-posix-docker-runtime-resources-test-031327.log` |
| 2026-09-18T03:13:28+08:00 | 同步后/static-posix/remote-deploy-test | V072 | 0 | `logs/after-static-posix-remote-deploy-test-031328.log` |
| 2026-09-18T03:13:31+08:00 | 同步后/static-posix/apple-container-test | V073 | 1 | `logs/after-static-posix-apple-container-test-031331.log` |
| 2026-09-18T03:13:31+08:00 | 同步后/static-posix/caddy-cache | V074 | 0 | `logs/after-static-posix-caddy-cache-031331.log` |
| 2026-09-18T03:13:31+08:00 | 同步后/static-posix/apple-syntax | V075 | 0 | `logs/after-static-posix-apple-syntax-031331.log` |
| 2026-09-18T03:13:31+08:00 | 同步后/static-posix/remote-syntax | V076 | 0 | `logs/after-static-posix-remote-syntax-031331.log` |
| 2026-09-18T03:13:31+08:00 | 同步后/static-posix/blue-green-test | V077 | 0 | `logs/after-static-posix-blue-green-test-031331.log` |
| 2026-09-18T03:13:32+08:00 | 同步后/static-posix/blue-green-evidence-test | V078 | 0 | `logs/after-static-posix-blue-green-evidence-test-031332.log` |
| 2026-09-18T03:13:33+08:00 | 同步后/static-posix/blue-green-preflight-test | V079 | 0 | `logs/after-static-posix-blue-green-preflight-test-031333.log` |
| 2026-09-18T03:15:46.7239069+08:00 | 同步后/backend/build | V041 | 0 | `logs/after-backend-build-031546723.log` |
| 2026-09-18T03:16:54+08:00 | 同步后/protocol/build | V080 | 0 | `logs/after-protocol-build-031654.log` |
| 2026-09-18T03:17:57+08:00 | 同步后/protocol/twenty-switches | V081 | 0 | `logs/after-protocol-twenty-switches-031757.log` |
| 2026-09-18T03:22:45+08:00 | 同步后/environment/environment | V053 | 0 | `logs/after-environment-environment-032245.log` |
| 2026-09-18T03:22:50+08:00 | 同步后/protocol/forced-retirement | V052 | 0 | `logs/after-protocol-forced-retirement-032250.log` |
| 2026-09-18T03:22:51+08:00 | 同步后/environment/posix-targets | V082 | 0 | `logs/after-environment-posix-targets-032251.log` |

### 修改文件清单

以下为相对 LOCAL_PRE_SYNC_SHA 的最终171个文件（M1的170个文件加最后追加的同步历史）；代码生成、测试和文档均计入文件数，不能直接等同于功能数量。

```text
M	.gitignore
M	backend/cmd/server/wire_gen.go
M	backend/go.sum
M	backend/internal/handler/admin/account_data.go
M	backend/internal/handler/admin/account_handler.go
M	backend/internal/handler/admin/idempotency_helper.go
A	backend/internal/handler/admin/proxy_credentials_update_test.go
M	backend/internal/handler/admin/proxy_data.go
M	backend/internal/handler/admin/proxy_handler.go
A	backend/internal/handler/admin/subscription_bulk_action_test.go
M	backend/internal/handler/admin/subscription_handler.go
M	backend/internal/handler/gateway_model_allowlist_listing_test.go
A	backend/internal/handler/gemini_mixed_models_test.go
M	backend/internal/handler/gemini_v1beta_handler_test.go
M	backend/internal/handler/gemini_v1beta_handler.go
A	backend/internal/handler/redeem_handler_test.go
M	backend/internal/handler/redeem_handler.go
M	backend/internal/handler/stream_error_event_test.go
M	backend/internal/handler/stream_error_event.go
A	backend/internal/pkg/antigravity/attribution_test.go
M	backend/internal/pkg/antigravity/request_transformer_test.go
M	backend/internal/pkg/antigravity/request_transformer.go
M	backend/internal/pkg/apicompat/chatcompletions_responses_bridge_test.go
M	backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go
M	backend/internal/pkg/apicompat/chatcompletions_responses_tool_output_media_test.go
M	backend/internal/pkg/apicompat/responses_stream_event_wire_test.go
A	backend/internal/pkg/apicompat/responses_to_anthropic_text_recovery_test.go
M	backend/internal/pkg/apicompat/responses_to_anthropic.go
A	backend/internal/pkg/apicompat/responses_tool_output_media_test.go
A	backend/internal/pkg/apicompat/responses_tool_output_media.go
M	backend/internal/pkg/apicompat/types.go
M	backend/internal/repository/account_repo_integration_test.go
A	backend/internal/repository/account_repo_ratelimit_cas_integration_test.go
M	backend/internal/repository/account_repo_temp_unsched_test.go
M	backend/internal/repository/account_repo.go
M	backend/internal/repository/custom_group_usage_rollup_repo.go
M	backend/internal/repository/redeem_code_repo_integration_test.go
M	backend/internal/repository/redeem_code_repo_sort_integration_test.go
M	backend/internal/repository/redeem_code_repo.go
M	backend/internal/repository/usage_log_repo_group_summary_test.go
M	backend/internal/server/routes/admin.go
A	backend/internal/server/routes/subscription_bulk_action_routes_test.go
M	backend/internal/service/account_wildcard_test.go
M	backend/internal/service/account.go
A	backend/internal/service/admin_proxy_credentials_update_test.go
M	backend/internal/service/admin_proxy.go
M	backend/internal/service/admin_service.go
M	backend/internal/service/antigravity_gateway_compat_test.go
M	backend/internal/service/antigravity_gateway_compat.go
M	backend/internal/service/antigravity_gateway_gemini.go
M	backend/internal/service/antigravity_gateway_service_test.go
M	backend/internal/service/antigravity_gateway_streaming.go
M	backend/internal/service/antigravity_token_provider.go
M	backend/internal/service/batch_image_public_test.go
M	backend/internal/service/batch_image_public.go
M	backend/internal/service/cn_providers_test.go
M	backend/internal/service/gemini_messages_compat_service.go
A	backend/internal/service/idempotency_detached_test.go
M	backend/internal/service/idempotency.go
A	backend/internal/service/ollama_cloud_usage_rate_limit_probe_test.go
A	backend/internal/service/ollama_cloud_usage_rate_limit_probe.go
A	backend/internal/service/ollama_cloud_usage_reset_test.go
A	backend/internal/service/ollama_cloud_usage_reset.go
M	backend/internal/service/ollama_cloud_usage.go
A	backend/internal/service/openai_account_scheduler_canonical_quota_test.go
A	backend/internal/service/openai_account_scheduler_metrics_test.go
M	backend/internal/service/openai_account_scheduler.go
A	backend/internal/service/openai_chat_roles_test.go
A	backend/internal/service/openai_chat_roles.go
M	backend/internal/service/openai_codex_models_service_test.go
M	backend/internal/service/openai_codex_models_service.go
M	backend/internal/service/openai_codex_transform.go
M	backend/internal/service/openai_compact_stream_bridge_test.go
M	backend/internal/service/openai_compact_stream_bridge.go
M	backend/internal/service/openai_compact_stream_failure_created_at_test.go
M	backend/internal/service/openai_gateway_chat_completions_raw.go
A	backend/internal/service/openai_gateway_kimi_upstream_sync_test.go
M	backend/internal/service/openai_gateway_request_body.go
M	backend/internal/service/openai_gateway_response_handling.go
M	backend/internal/service/openai_gateway_scheduling.go
M	backend/internal/service/openai_gateway_service_test.go
M	backend/internal/service/openai_model_mapping_test.go
M	backend/internal/service/openai_model_mapping.go
A	backend/internal/service/openai_oauth_input_metadata_test.go
M	backend/internal/service/openai_responses_namespace_forward_test.go
M	backend/internal/service/openai_responses_namespace_test.go
M	backend/internal/service/openai_responses_namespace.go
M	backend/internal/service/openai_ws_http_bridge.go
A	backend/internal/service/ratelimit_service_ollama_429_test.go
A	backend/internal/service/ratelimit_service_ollama_429.go
M	backend/internal/service/ratelimit_service.go
M	backend/internal/service/redeem_service.go
A	backend/internal/service/subscription_bulk_action_test.go
A	backend/internal/service/subscription_bulk_action_transaction_test.go
A	backend/internal/service/subscription_bulk_action.go
M	backend/internal/service/subscription_renewal_lock_test.go
M	backend/internal/service/subscription_service.go
M	backend/internal/service/token_cache_invalidator.go
M	backend/internal/service/token_cache_key_test.go
M	backend/internal/service/token_refresh_service_candidates_test.go
M	backend/internal/service/wire.go
A	docs/ANTIGRAVITY_ATTRIBUTION_429.md
M	docs/custom-development-history.md
M	docs/upstream-sync-history.md
A	frontend/src/api/__tests__/admin.subscriptions.bulk.spec.ts
A	frontend/src/api/__tests__/keys.bulkUpdate.spec.ts
A	frontend/src/api/__tests__/redeem.spec.ts
M	frontend/src/api/admin/accounts.ts
M	frontend/src/api/admin/subscriptions.ts
M	frontend/src/api/keys.ts
M	frontend/src/api/redeem.ts
A	frontend/src/components/admin/channel/__tests__/ModelTagInput.keyboard.spec.ts
M	frontend/src/components/admin/channel/ModelTagInput.vue
A	frontend/src/components/admin/payment/__tests__/AdminRefundDialog.balance.spec.ts
M	frontend/src/components/admin/payment/AdminRefundDialog.vue
A	frontend/src/components/admin/subscription/__tests__/BulkSubscriptionActionDialog.spec.ts
A	frontend/src/components/admin/subscription/__tests__/bulkSubscriptionOperation.spec.ts
A	frontend/src/components/admin/subscription/BulkSubscriptionActionDialog.vue
A	frontend/src/components/admin/subscription/bulkSubscriptionOperation.ts
M	frontend/src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts
M	frontend/src/components/admin/user/UserPlatformQuotaModal.vue
M	frontend/src/components/auth/__tests__/EmailOAuthButtons.spec.ts
A	frontend/src/components/common/__tests__/BaseDialog.ids.spec.ts
A	frontend/src/components/common/__tests__/Pagination.jump.spec.ts
A	frontend/src/components/common/__tests__/ProxySelector.testing.spec.ts
M	frontend/src/components/common/BaseDialog.vue
M	frontend/src/components/common/Pagination.vue
M	frontend/src/components/common/ProxySelector.vue
A	frontend/src/components/keys/__tests__/BulkEditKeysModal.spec.ts
A	frontend/src/components/keys/BulkEditKeysModal.vue
M	frontend/src/components/payment/__tests__/AmountInput.spec.ts
M	frontend/src/components/payment/AmountInput.vue
A	frontend/src/components/user/profile/__tests__/Totp.errors.spec.ts
A	frontend/src/components/user/profile/__tests__/TotpSetupModal.inputs.spec.ts
M	frontend/src/components/user/profile/TotpDisableDialog.vue
M	frontend/src/components/user/profile/TotpSetupModal.vue
M	frontend/src/composables/__tests__/useClipboard.spec.ts
M	frontend/src/composables/useAutoRefresh.ts
M	frontend/src/composables/useClipboard.ts
M	frontend/src/i18n/locales/en/admin/channels.ts
M	frontend/src/i18n/locales/en/dashboard.ts
M	frontend/src/i18n/locales/zh/admin/channels.ts
M	frontend/src/i18n/locales/zh/dashboard.ts
A	frontend/src/stores/__tests__/announcements.markAll.spec.ts
A	frontend/src/stores/__tests__/payment.config.spec.ts
A	frontend/src/stores/__tests__/subscriptions.clear.spec.ts
M	frontend/src/stores/announcements.ts
M	frontend/src/stores/payment.ts
M	frontend/src/stores/subscriptions.ts
A	frontend/src/utils/keyGroupProviders.ts
M	frontend/src/views/admin/__tests__/AccountsView.lite.spec.ts
M	frontend/src/views/admin/__tests__/GroupsView.codexManifest.spec.ts
A	frontend/src/views/admin/__tests__/ProxiesView.credentials.spec.ts
A	frontend/src/views/admin/__tests__/SubscriptionsView.bulkActions.spec.ts
M	frontend/src/views/admin/AccountsView.vue
M	frontend/src/views/admin/ProxiesView.vue
M	frontend/src/views/admin/SubscriptionsView.vue
M	frontend/src/views/auth/__tests__/RegisterView.spec.ts
M	frontend/src/views/auth/RegisterView.vue
A	frontend/src/views/user/__tests__/ChannelStatusV1View.refresh.spec.ts
M	frontend/src/views/user/__tests__/KeysView.spec.ts
M	frontend/src/views/user/__tests__/RedeemView.spec.ts
A	frontend/src/views/user/__tests__/UserOrdersView.filters.spec.ts
M	frontend/src/views/user/ChannelStatusV1View.vue
M	frontend/src/views/user/KeysView.vue
M	frontend/src/views/user/RedeemView.vue
M	frontend/src/views/user/UserOrdersView.vue
M	Makefile
M	README_CN.md
M	README_JA.md
M	README.md
```

### 残余风险与最终门禁

- 未验证：真实 Provider 请求、真实支付与第三方回调、邮件/短信、生产应用和数据库、完整浏览器业务端到端、macOS原生容器功能、全包竞态、峰值负载及真实多主机网络。没有凭本地mock/集成用例宣称这些流程全部正常。远端CI没有触发，未以推送获得额外权限。
- 既有xlsx高危及其他依赖告警、Canvas格式和Apple平台限制继续保留。Go软内存与PostgreSQL缓存预算叠加的既有OOM风险不因本轮测试消失；没有修改生产预算、3600秒持久化响应头约束、5秒usage任务或实际部署资源。
- 源码级复核：`git diff --check` 与 `git diff --cached --check` 退出0；未解决索引冲突为空、严格冲突标记扫描为0、意外删除为0；新增差异的私钥头、GitHub/AWS凭据及带认证URL高置信特征命中为0。未包含实际.env、缓存、临时日志或无关文件。这是有限特征检查，不宣称形式化证明无任何秘密。
- 上游新SHA是M1的祖先，两个父SHA已核对；58个功能编号集合与同步前完全一致。版本保持0.1.239，关键部署/迁移/依赖文件无意外净变化。
- 进程及容器收尾：本轮隔离服务和测试容器已结束/自清理；Docker清单仅见既有其他项目容器及原有停止容器，未删除或重启它们。POSIX临时挂载随WSL空闲退出已消失，最终findmnt查询无挂载；不删除仓库内日志、原有文件或其他工作资料。
- 同步记录仅追加，原历史内容保持不变；M1以外最后一个本地提交只包含本文件。原目标main在记录提交前仍为 `23f34132d5e591007bf8ac8472918fec3af871e5`。提交记录后再次检查其未变和工作树干净，仅执行 `git switch main` 与 `git merge --ff-only sync/upstream-20260918-efe9aab1e`；若条件不满足则停止，不强制更新。最终main和记录提交SHA在对话中报告。
- 本轮没有Skipped、Deferred或未解决Conflict；没有push、创建PR、打tag、访问远程服务器、部署或操作生产数据库。备份和同步分支保留，供本地核查。

## 2026-09-18 追加授权后的主线交付与 v0.1.240 发布

- 状态：代码已合入远端main，v0.1.240已通过GitHub Actions自动蓝绿部署；此前本地同步条目中的未推送/未部署仅描述当时边界，本条不改写历史。
- 固定范围仍为bdb42e22f..efe9aab1e，共108个上游提交，无新增Skipped、Deferred或Conflict；`LAST_FULLY_INTEGRATED_UPSTREAM_SHA`继续为`efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`。
- 同步候选`ef187f2ec9cd9615a3c803dffd20fc3188e33785`在同步分支CI `35266469278`、安全扫描`35266469284`全部成功后，于03:57快进推送远端main。main CI `35267957601`/安全`35267957581`、tag CI `35268007948`/安全`35268007916`均全部成功。
- 新附注标签`v0.1.240`固定指向候选；Release `35268007982`的5项任务全部成功，部署05:19:52完成。Actions自动回写VERSION的提交为`50a189519535485643a3ea804b382fdda6bf0381`，本地已ff-only接收，没有移动发布标签。
- 04:18:38切流至green/18082，确认耗时1.192秒；09:21:57最终验收为stable、pending=null，API/direct均200且同一实例。部署窗口两入口各3509次健康探针、异常0，配置、数据挂载和PG/Redis容器身份未变。
- 旧blue在一小时到期后仍有78个会话租约，按既有授权策略强制退役，05:19:47回收完成；退出137、OOMKilled=false、可见用量丢弃0，但clean_exit=false、usage_loss_unknown=true。旧请求/续接可能中断，不能声称自然排空或完整计费。
- 本次仅发布已验证代码，未追加业务变更、生产配置或数据库迁移；真实Provider、支付回调、完整账单及峰值负载仍未验证。详见`docs/operations/2026-09-18-v0.1.240-release.md`；两份台账与发布记录一同走记录分支CI后再合入主线。

## 2026-09-20 本地上游同步 7c700729c

- 记录时间：2026-09-20T23:09:09.441+08:00；执行开始：2026-09-20T21:53:36+08:00，时区Asia/Shanghai。
- 状态：固定范围本地合并与验证成功；目标main的ff-only更新作为记录提交后的最终门禁，最终SHA在对话交付，不自引用记录提交。
- 本地仓库：`D:/project/sub2api`；目标分支：main；LOCAL_PRE_SYNC_SHA：`b28864efb029acb59616b146d1723b0fa3390474`。
- 上游：`https://github.com/Wei-Shaw/sub2api`；remote=upstream；远端HEAD确认默认分支main。
- UPSTREAM_OLD_SHA：`efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`；UPSTREAM_NEW_SHA：`7c700729c23187d31ed320f6b19c790e2f194826`。
- ACTUAL_MERGE_BASE：`efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`，等于记录旧基线；旧基线是本地及新目标的祖先，实际merge范围没有扩大。
- LAST_FULLY_INTEGRATED_UPSTREAM_SHA：`7c700729c23187d31ed320f6b19c790e2f194826`。
- 集成策略：用户集中审批后完整merge，未使用cherry-pick、整文件ours/theirs、stash或历史重写。29普通提交的git cherry结果均为+，17合并节点未发现额外remerge差异。
- 备份分支：`backup/pre-upstream-sync-20260920-215433-b28864efb`；同步分支：`sync/upstream-20260920-7c700729c`。
- M1（上游merge及本地兼容代码提交）：`c40d5917f3f9653db9501c854596309e54419a63`；父提交依次为LOCAL_PRE_SYNC_SHA和UPSTREAM_NEW_SHA。
- M1同时更新二开功能清单和变更记录；M2仅追加本同步历史，是本轮最后一个本地提交。所有46个上游提交均映射到M1的祖先历史，具体处置如下。
- 处置数量：Applied共46，其中6项标注Applied + Overridden，普通Applied为40；Already Applied=0、Skipped=0、Deferred=0、未解决Conflict=0。覆盖标记不表示跳过上游历史。
- 上游净变更135个文件，48个与本地定制重叠；本地M1净变更136个文件，最终含本历史共137个文件。工作树版本保留0.1.242。
- 边界：只授权本地备份、合并、兼容修改、隔离测试、提交和最后快进；没有push、PR、tag、远端CI、发布、部署、远程服务器连接或生产数据库操作。

### 上游逐项处置

| 上游完整SHA | 分组 | 状态 | 处理与本地映射 |
| --- | --- | --- | --- |
| `aba34524f1d52c4f1b9fb59886fa6f55d70ef3ee` | Seedance | Applied + Overridden | M1；原生视频任务的创建、查询、删除及兼容路由；保留本地根路径 Responses 图片统计。 |
| `d099c06945523ab9fe4124a0f1a4ed3f1c83e289` | Seedance | Applied | M1；Seedance lint 与 gRPC 安全检查修复；相关依赖版本已在同步前基线存在，净依赖文件不变化。 |
| `f79b8bf96c8703db4759f9306a7de58d367286e9` | Gemini/传输 | Applied | M1；Go/Python GenAI 客户端禁用原生流 SSE 注释；新增测试按本地签名补充 Account 参数。 |
| `0f4d8acaa8400ddfe9c88efbd791105d8677e071` | Gemini/传输 | Applied | M1；按 thinkingConfig 将 Gemini 裸模型映射为推理变体。 |
| `a9c7c6e8bf2c9576aad024dcb2452e47fc45d703` | 界面 | Applied | M1；移动端竖屏顶栏保留模型广场图标入口。 |
| `0920e06a423abf7b2b87d788c9df55a8a2a4d668` | 协议 | Applied + Overridden | M1；恢复公开响应模型别名，继续保留本地上游型号诊断、映射链、Token 用量与请求模型计费。 |
| `bcc73f8d405892e4748bbcb0fb73af070f3745d4` | 协议 | Applied + Overridden | M1；DeepSeek 缺失历史推理内容时补占位；保留本地 CC 三返回值、首 Token 生命周期及 Kimi 参数重试。 |
| `7b4de8b6ae7652c36fbdc859486f52d937173246` | 审核 | Applied | M1；防止 reminder 标签绕过关键词审核。 |
| `db8692d679d375b53ce157477d8e7b6b4d889140` | 额度/刷新 | Applied | M1；中国产商 Coding Plan 明确额度耗尽 403 进入临时停调度；保留本地人工暂停和故障恢复边界。 |
| `a8a1a3a958a0df8544aaa90729b5e4c178aa4090` | 协议 | Applied | M1；将 Anthropic 工具 schema 根联合类型扁平化，保留既有 namespace 和工具转换链。 |
| `2d37088bd95301300bf5a5360aa546766a1720c3` | 协议 | Applied | M1；非法 User-Agent 不丢弃独立解析的客户端版本。 |
| `c63bd14a021625531f557b1112332273f945808d` | 插件 | Applied | M1；通用宿主服务、KV、账号目录和只读状态桥接；依赖注入与本地构造参数顺序共存。 |
| `6477143533a38cf1803f36aad8970efff2e063ba` | 插件 | Applied | M1；插件类型断言检查补齐，满足 errcheck。 |
| `4dbcdce43c63814c93e215cbf06044b57660341b` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `4275047d164591118be946e964e921e9672347c6` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `d5cea617a5795ba1c159f90a20efeae54df138c1` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `1b45d213288cb8f69e1cd611293f05dae6dc1c7f` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `09f88865e08f938c1e7e6df54df9928fef902f7b` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `0c9e83b59cf9f8647434008a7377ad481b291d3f` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `74431ffad8b9446100a6c1416fe69e908c03ae81` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `aea725f2ea644d5592d0bbb1d63b607efa7e200a` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `1a9d49e16f7a22c432b428fce4af8d731f1fa364` | 版本 | Applied + Overridden | M1；上游 VERSION=0.2.7 的历史纳入；工作树明确保留本地0.1.242，继续使用本地发布版本线。 |
| `b252821c59d399bc067b60f669c242669fa3012b` | 额度/刷新 | Applied | M1；用量查询保留账号刷新错误，不以读取覆盖故障状态。 |
| `783a0a98304a383b333e311dcec49d453533221a` | 审核 | Applied + Overridden | M1；引入 TypeSafe 独立配置与可空 engine_meta；全部 LocalAudit、Cyber 字段、保存、克隆和视图组合保留。 |
| `1bfe0d37284804a94c76c979585195c1f9af06a5` | 审核 | Applied | M1；引擎草稿密钥状态、阈值默认值和界面反馈保持独立。 |
| `29eead917f6b619693d99e668a0ca19c5ccb27d1` | 插件 | Applied | M1；HostService 账号目录增加结构化只读元数据。 |
| `19794bc46afb1147d6cd6c266f3b77193c8c8c42` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `f28adb6ddbfcf6a1b6bc65663d31e983b3eff5a1` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `8bb48622a3893c7a968756bb7d4a5d9cfda5c029` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `53b4bbe736323abac37dcf0a5f1ed7cd7893008f` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `7403a011775c2543cb0e212e8cf1dd52f1a13cc1` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `bbdcfbac0a3c0982a33639fba372d55d25724e2f` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `5090ffe05ef6c9286f425bce4754ca0b9b6f4327` | 积分/邀请 | Applied | M1；Codex 积分展示、兑换与邀请管理；仅运行本地mock，不进行真实邀请或兑换。 |
| `e50993738285b1895195f7ef8a26eb34fca1544d` | 文档 | Applied | M1；上游曾增加积分界面验证文档和截图，随后由95134201b删除；保留两条历史，最终无这些文件。 |
| `e009ea303602d07a11f34e72f0fab3fa8c67d578` | Gemini/传输 | Applied | M1；OpenAI HTTP/2 探测和PING期限恢复为15秒/15秒，其他传输及本地流处理保留。 |
| `fbb9006adef852c46f0c7f18b0a8a740722cfac7` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `8f6bbb59c742e9d3ce56615a8b21bf504b3a56b9` | Gemini/传输 | Applied | M1；稳定 Gemini 注释保活测试，保留断言并与本地账号参数适配。 |
| `fc96132e7231132722f8737257f59a1ca55df347` | 积分/邀请 | Applied | M1；邀请结果处理及独立HTTP传输加固。 |
| `acdf3c54da70ee05c5b939a30ac2d2b28f2a7a22` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `95134201b2529fb8fdc759af625a3431da2d7d1e` | 文档 | Applied | M1；删除误合入的积分文档和截图；与其新增提交一起完整集成。 |
| `0892ef3a9a320dad79ff87930dcd5ae56a9a140a` | 发布 | Applied + Overridden | M1；独立构建矩阵、产物校验和dry-run；保留Canvas、DockerHub凭据回退、固定SHA部署及蓝绿门禁。 |
| `d2e319b2a17006122cd2d53d828c44a0bf21bd9b` | Gemini/传输 | Applied | M1；修复WebSocket同线程抢占测试的关闭时序竞态。 |
| `d2416a9b68cf345a38e5d98c395cfb5d44a22572` | 发布 | Applied | M1；发布工具与跨job缓存键固定，下载的构建产物按同一SHA核验。 |
| `d1556a7a96d99ee2ebd93a624c16deb6fa387e49` | 发布 | Applied | M1；发布辅助测试指向最终目录，本地兼容回归加入相同测试文件。 |
| `87957a608bd3b0fdd1e91852f9afcd9b2af55135` | 合并节点 | Applied | M1；无额外remerge差异；对应功能提交完整纳入，本地覆盖见其功能行。 |
| `7c700729c23187d31ed320f6b19c790e2f194826` | 忽略规则 | Applied | M1；增加上游忽略项，同时保留本地临时、缓存和运维文件规则。 |

### 已批准冲突与兼容处理

- `.github/workflows/backend-ci.yml`：本地流竞态、生命周期及蓝绿任务与上游release-helpers同时保留；没有删除或关闭既有门禁。
- `.github/workflows/release.yml`：保留默认deploy_production和Canvas构建；DockerHub用户名继续secrets/vars回退，登录及描述更新检查token，镜像脚本缺token时只走可用的GHCR路径；矩阵与发布保留120分钟GoReleaser超时。
- 发布的build-frontend、build-binaries、release和deploy-production都绑定prepare.sha；版本回写显式contents:write，dry-run禁止回写；生产job依赖prepare且同时排除dry-run/simple，保留120分钟、3600秒观察及既有强制退役风险提示。仅结构检查和模拟Docker命令，未执行真实发布。
- `.gitignore`组合双方规则；`backend/cmd/server/VERSION`保留0.1.242；`wire_gen.go`通过go generate重新生成，保留本地AccountTest参数次序，注入Referral和PluginKV/目录能力。
- `content_moderation_handler.go`及`content_moderation.go`：Engine/TypeSafe/EngineConfigs与全部LocalAudit、Cyber字段并存，保持原有保存、深拷贝、状态及后台副作用路径；没有用上游结构整体替换本地结构。
- `routes/gateway.go`：增加Seedance路由，根路径Responses及其子路径继续携带trackImageGroupResult。`openai_gateway_cc_pipeline.go`：保留本地三返回值和firstTokenAttempt，在同一出站点加入DeepSeek占位，Kimi参数重试及资源清理不变。
- `frontend/src/types/index.ts`：Seedance端点能力与ImageSizeTier均保留。`RiskControlView.vue`及测试：本地审计控件/列表、Cyber设置和新引擎草稿共存，合并mock而不删断言。以上共12个文本冲突文件，已全部解决。
- 同步后首次默认Go测试发现新增Gemini心跳测试少传本地Account参数；参照既有测试补`&Account{}`，随后默认/unit全量及构建通过。这是批准范围内的机械签名适配，不改变业务策略。
- 同步后首次Linux integration的Claude断连超时用例失败一次，得到缺少终止事件而非超时；相关实现和测试与基线一致。备份b28864efb的独立快照重复20次时复现同样错误一次（命令退出1），合并代码20次全部通过（退出0），确认该失败在同步前代码同样存在。夹具在1秒超时后仅留0.5秒便关闭pipe，推断受调度延迟影响；未修改实现、测试或断言，原全量integration另行复跑，所有结果保留如下。
- 新增2项后端审核交叉测试及2项前端保存切换测试，验证全部LocalAudit和Cyber字段不丢失、TypeSafe及规则快照不共享可变内存；响应别名8组场景补充请求/映射/上游型号及Token用量断言。发布辅助测试新增4项结构化YAML约束，共14项，镜像发布命令使用fake docker。

### 保留的本地行为与接受边界

- 60个CUST编号集合与同步前完全一致，没有停用、删除或复用。清单已更新CUST-GW-001、CUST-GW-003、CUST-PROTO-001、CUST-PROTO-006、CUST-ACC-001、CUST-BILL-001、CUST-BILL-004、CUST-RISK-001、CUST-RISK-002、CUST-OPS-001、CUST-OPS-004；纯上游新功能不重复分配CUST编号。
- Claude安全流重试和提前JSON保活继续默认关闭；首语义输出、终止帧收尾、先取消再关闭、已交付输出禁止重放和按请求用量去重保留。Kimi四项参数兼容开关默认关闭，单项一次及最多额外四次的边界保留。
- 用户按请求模型计费、Composite显式别名价格规则、严格缺价、Fable无默认三倍、Free Fast、人工暂停与故障恢复策略不被上游显示别名覆盖；本轮没有修改实际定价、用户余额或生产账号。
- 按用户串行扣费、usage task 5秒、禁止网页自更新、生产bind mount/回环监听/HTTP upstream、持久化通用响应头3600秒及既有资源预算保持不变。生产Compose、配置和依赖锁文件无净改动。
- Seedance能力默认不启用，创建任务不按最终生成Token立即结算，仅客户端查询成功任务时走既有用量结算；没有新增后台轮询，未查询完成可能不结算是获批保留的上游边界。
- TypeSafe为独立可选审核引擎，默认仍OpenAI；图像跳过、引擎独立密钥/阈值和失败处理保持上游语义，本地规则及Cyber作用域不扩大。新增238b迁移只增加可空engine_meta。
- 插件HostService账号目录仅提供能力授权范围内账号，但其中包含短期访问Token、代理认证与Extra元数据；用户审批已接受这一信任边界。本次不安装或启用插件，不以只读元数据描述为不含敏感信息。

### 验证环境与结果

- 同步前后相同工具环境：Windows Go1.27.0、Node20.20.2、pnpm9.15.9、golangci-lint2.13；Ubuntu24.04/WSL Go1.27.0执行隔离PostgreSQL/Redis集成及CGO竞态。缓存、构建产物和日志均在仓库内，运行记录目录`output/upstream-sync-20260920-7c700729c`。
- Go构建并行参数均为GOFLAGS=-p=4；Windows验证GOMAXPROCS=8/GOMEMLIMIT=6GiB，Linux为4/3GiB，非竞态CGO_ENABLED=0、竞态为1。全包Windows/Linux验证在基线首次争用后不再并行；后补基线仅在独立快照运行一个定向用例。
- 清除实际Provider、数据库及外部测试凭据，TYPESAFE_LIVE_TEST=0；TypeSafe、邀请、插件及Provider验证使用mock/httptest。独立启动使用本机18973和带本轮专有标签的容器，假凭据、临时数据，HTTP代理指向本机拒绝端口以阻断外部业务访问。
- Go默认全量、unit全量、integration全量、构建、golangci-lint、govulncheck、两组流/生命周期race前后最终均退出0；仅定向race，不声称全包竞态。
- 前端：同步前324文件/2441用例，同步后326文件/2481用例，lint、类型检查和构建均0。Canvas前后均8文件/34用例，类型检查、测试和构建均0；格式检查均1且只报告既有package.json。
- 隔离启动前后最终均0；同步后恢复同步前的测试数据库，启动完成迁移、/health通过，校验engine_meta可空并重复DDL成功。启动轮询中的暂时超时不等于最终健康失败，也不作为性能验收。新库完整迁移由integration覆盖。
- Compose安全/网关环境/资源检查、12项模拟远程部署、Caddy策略、shell语法、蓝绿证据2项及预检3项通过；蓝绿48项在项目内POSIX临时目录前后通过。
- 既有依赖审计：前端2高危、13中危、1低危；Canvas6中危，两个pnpm audit命令前后均退出1且数量一致。没有为本次同步扩大升级范围、增加豁免或掩盖这些风险。
- 既有平台限制：Apple测试在Linux使用macOS的stat -f %Lp而失败；原始蓝绿测试把Unix socket建在NTFS路径导致Operation not supported。后者在仓库内tmpfs仅改临时目录、不改断言重跑48项通过；macOS原生行为仍未验证。
- 首次同步前integration因Windows/Linux全包测试同时操作.entc失败；串行复跑退出0。此后两平台全包不并行。首次smoke外层180秒在构建阶段超时124，内部cleanup误报0不能算通过；固定600秒上限后前后启动均成功。
- 同步前曾修改正在运行的Linux验证封装，导致测试结束后外层shell异常；保留首次原始记录，固定封装并复跑集成/race后确认退出0，不把封装异常当业务测试通过。
- 差异检查、索引冲突、严格冲突标记、意外删除、临时/秘密文件路径、高置信私钥/Token/认证URL特征扫描均通过；这些是有限特征检查，不宣称形式化证明没有任何秘密。

### 实际命令索引

下表记录实际执行命令及工作目录。Windows命令以程序和参数逐项JSON引用；Linux命令保留封装记录。V编号在执行表复用，同一命令的首次失败和重跑均保留。内部smoke收尾另列，最终结果以外层timeout命令为准。

| 编号 | 工作目录 | 实际命令 |
| --- | --- | --- |
| V001 | `D:\project\sub2api\backend` | `"C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe" "test" "./..." "-count=1" "-timeout=20m"` |
| V002 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "install" "--frozen-lockfile" "--offline" "--ignore-scripts"` |
| V003 | `D:\project\sub2api\backend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\go-tools\\bin\\golangci-lint.exe" "run" "./..." "--timeout=30m"` |
| V004 | `/mnt/d/project/sub2api/backend` | `go test -exec /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/test-exec-v2.sh -tags=integration ./... -count=1 -timeout=25m ` |
| V005 | `/mnt/d/project/sub2api/backend` | `go test -exec /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/test-exec-v2.sh -race -tags=unit -count=1 -timeout=20m -run TestKimi\\|TestOpsErrorLoggerMiddleware_Kimi\\|TestAnthropicStreamSafeRetry\\|TestFirstTokenCleanup\\|TestGatewayService_AnthropicAPIKeyPassthrough_StreamingIdleTimeout\\|TestDecompressResponseBodyStreamClose\\|TestDecompressedBodyClose ./internal/service ./internal/handler ./internal/repository ` |
| V006 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "lint:check"` |
| V007 | `D:\project\sub2api\canvas` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "node_modules/prettier/bin/prettier.cjs" "--check" "."` |
| V008 | `D:\project\sub2api\canvas` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "node_modules/typescript/bin/tsc" "--noEmit"` |
| V009 | `D:\project\sub2api\canvas` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "node_modules/vitest/vitest.mjs" "run" "--maxWorkers=4" "--minWorkers=1"` |
| V010 | `由封装脚本确定` | `smoke.sh before` |
| V011 | `/mnt/d/project/sub2api/backend` | `timeout 180s bash /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/smoke.sh before ` |
| V012 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash deploy/tests/docker-compose-security-test.sh ` |
| V013 | `D:\project\sub2api\backend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260915-bdb42e22f\\tools\\govulncheck.exe" "./..."` |
| V014 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash deploy/tests/docker-compose-gateway-env-test.sh ` |
| V015 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash deploy/tests/docker-runtime-resources-test.sh ` |
| V016 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash deploy/tests/remote-deploy-test.sh ` |
| V017 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash deploy/tests/apple-container-test.sh ` |
| V018 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash deploy/test-caddyfile-cache.sh ` |
| V019 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `bash -n deploy/apple-container.sh ` |
| V020 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `sh -n deploy/remote-deploy.sh ` |
| V021 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `python3 /mnt/d/project/sub2api/deploy/tests/blue-green-test.py ` |
| V022 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "audit" "--prod" "--audit-level=high" "--json"` |
| V023 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `python3 /mnt/d/project/sub2api/deploy/tests/blue-green-evidence-test.py ` |
| V024 | `/mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/before-static-lf` | `python3 /mnt/d/project/sub2api/deploy/tests/blue-green-preflight-test.py ` |
| V025 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "typecheck"` |
| V026 | `D:\project\sub2api\backend` | `"C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe" "test" "-tags=unit" "./..." "-count=1" "-timeout=20m"` |
| V027 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "test:run" "--maxWorkers=4" "--minWorkers=1"` |
| V028 | `D:\project\sub2api\canvas` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "build" "--outDir" "D:\\project\\sub2api\\output\\upstream-sync-20260920-7c700729c\\artifacts\\before-canvas"` |
| V029 | `/mnt/d/project/sub2api` | `python3 /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/bluegreen-posix.py ` |
| V030 | `/mnt/d/project/sub2api/backend` | `timeout 600s bash /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/smoke.sh before ` |
| V031 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "build" "--outDir" "D:\\project\\sub2api\\output\\upstream-sync-20260920-7c700729c\\artifacts\\before-frontend"` |
| V032 | `/mnt/d/project/sub2api/backend` | `go test -exec /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/test-exec-v2.sh -race -tags=unit -count=1 -timeout=20m -run TestPeer\\|TestAffinity\\|TestPublicPeer\\|TestLongSSE\\|TestHijacked\\|TestDetachedProducer\\|TestControl\\|TestDrain\\|TestNewInstance\\|TestLegacyWaits\\|TestRenewal\\|TestUsageRecordWorkerPool_StopRace\\|TestStoppedKeyed\\|TestOpenAIResponsesWebSocket_Ingress\\|OllamaCloud\\|Idempotency\\|Subscription ./internal/pkg/lifecycle ./internal/service ./internal/repository ./internal/handler ` |
| V033 | `D:\project\sub2api\backend` | `"C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe" "build" "./..."` |
| V034 | `D:\project\sub2api\backend` | `"C:/Users/xk/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.0.windows-amd64/bin/go.exe" "generate" "./cmd/server"` |
| V035 | `/mnt/d/project/sub2api` | `python3 -m unittest discover -s .github/release-tools -p test_release_matrix.py ` |
| V036 | `/mnt/d/project/sub2api` | `bash -n .github/release-tools/release-images.sh ` |
| V037 | `D:\project\sub2api\canvas` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "build" "--outDir" "D:\\project\\sub2api\\output\\upstream-sync-20260920-7c700729c\\artifacts\\after-canvas"` |
| V038 | `D:\project\sub2api\frontend` | `"D:\\project\\sub2api\\output\\upstream-sync-20260910-98d86915b\\tools\\node-v20.20.2-win-x64\\node.exe" "D:\\project\\sub2api\\output\\upstream-sync-20260905-ab99d56e9\\pnpm-9.15.9\\package\\bin\\pnpm.cjs" "run" "build" "--outDir" "D:\\project\\sub2api\\output\\upstream-sync-20260920-7c700729c\\artifacts\\after-frontend"` |
| V039 | `由封装脚本确定` | `smoke.sh after` |
| V040 | `/mnt/d/project/sub2api/backend` | `timeout 600s bash /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/smoke.sh after ` |
| V041 | `D:\project\sub2api` | `node output/upstream-sync-20260920-7c700729c/final-check.mjs` |
| V042 | `/mnt/d/project/sub2api/backend` | `go -C /mnt/d/project/sub2api/backend test -exec /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/test-exec-v2.sh -tags=integration ./internal/service -run \^TestGatewayService_AnthropicAPIKeyPassthrough_StreamingTimeoutAfterClientDisconnect\$ -count=20 -timeout=5m ` |
| V043 | `/mnt/d/project/sub2api/backend` | `go -C /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/baseline-disconnect/backend test -exec /mnt/d/project/sub2api/output/upstream-sync-20260920-7c700729c/test-exec-v2.sh -tags=integration ./internal/service -run \^TestGatewayService_AnthropicAPIKeyPassthrough_StreamingTimeoutAfterClientDisconnect\$ -count=20 -timeout=5m ` |

### 前后执行记录

| 开始时间 | 阶段/套件/检查 | 命令 | 退出码 | 证据 |
| --- | --- | --- | --- | --- |
| 2026-09-20T22:00:28.8544656+08:00 | 同步前/backend/test-default | V001 | 0 | `logs/before-backend-test-default-220028854.log` |
| 2026-09-20T22:00:29.1266450+08:00 | 同步前/frontend/install | V002 | 0 | `logs/before-frontend-install-220029126.log` |
| 2026-09-20T22:00:29.4760490+08:00 | 同步前/canvas/install | V002 | 0 | `logs/before-canvas-install-220029475.log` |
| 2026-09-20T22:00:29.7858307+08:00 | 同步前/extra/golangci-lint | V003 | 0 | `logs/before-extra-golangci-lint-220029785.log` |
| 2026-09-20T22:00:30+08:00 | 同步前/integration/integration | V004 | 1 | `logs/before-integration-integration-220030.log` |
| 2026-09-20T22:00:30+08:00 | 同步前/race/stream-race | V005 | 0 | `logs/before-race-stream-race-220030.log` |
| 2026-09-20T22:00:32.6871653+08:00 | 同步前/frontend/lint | V006 | 0 | `logs/before-frontend-lint-220032686.log` |
| 2026-09-20T22:00:33.0858253+08:00 | 同步前/canvas/format | V007 | 1 | `logs/before-canvas-format-220033085.log` |
| 2026-09-20T22:00:44.1847197+08:00 | 同步前/canvas/typecheck | V008 | 0 | `logs/before-canvas-typecheck-220044184.log` |
| 2026-09-20T22:01:12.9340646+08:00 | 同步前/canvas/test | V009 | 0 | `logs/before-canvas-test-220112933.log` |
| 2026-09-20T22:02:09+08:00 | 同步前/smoke-linux/内部收尾 | V010 | 0 | `before-smoke-results.jsonl` |
| 2026-09-20T22:02:09+08:00 | 同步前/smoke/health | V011 | 124 | `logs/before-smoke-health-220209.log` |
| 2026-09-20T22:02:10+08:00 | 同步前/static-posix/docker-compose-security-test | V012 | 0 | `logs/before-static-posix-docker-compose-security-test-220210.log` |
| 2026-09-20T22:02:10.2783368+08:00 | 同步前/security/govulncheck | V013 | 0 | `logs/before-security-govulncheck-220210276.log` |
| 2026-09-20T22:02:11+08:00 | 同步前/static-posix/docker-compose-gateway-env-test | V014 | 0 | `logs/before-static-posix-docker-compose-gateway-env-test-220211.log` |
| 2026-09-20T22:02:17+08:00 | 同步前/static-posix/docker-runtime-resources-test | V015 | 0 | `logs/before-static-posix-docker-runtime-resources-test-220217.log` |
| 2026-09-20T22:02:17+08:00 | 同步前/static-posix/remote-deploy-test | V016 | 0 | `logs/before-static-posix-remote-deploy-test-220217.log` |
| 2026-09-20T22:02:22+08:00 | 同步前/static-posix/apple-container-test | V017 | 1 | `logs/before-static-posix-apple-container-test-220222.log` |
| 2026-09-20T22:02:23+08:00 | 同步前/static-posix/caddy-cache | V018 | 0 | `logs/before-static-posix-caddy-cache-220223.log` |
| 2026-09-20T22:02:23+08:00 | 同步前/static-posix/apple-syntax | V019 | 0 | `logs/before-static-posix-apple-syntax-220223.log` |
| 2026-09-20T22:02:23+08:00 | 同步前/static-posix/remote-syntax | V020 | 0 | `logs/before-static-posix-remote-syntax-220223.log` |
| 2026-09-20T22:02:23+08:00 | 同步前/static-posix/blue-green-test | V021 | 1 | `logs/before-static-posix-blue-green-test-220223.log` |
| 2026-09-20T22:02:43.4706562+08:00 | 同步前/security/frontend-audit | V022 | 1 | `logs/before-security-frontend-audit-220243470.log` |
| 2026-09-20T22:02:53.2950753+08:00 | 同步前/security/canvas-audit | V022 | 1 | `logs/before-security-canvas-audit-220253294.log` |
| 2026-09-20T22:02:55+08:00 | 同步前/static-posix/blue-green-evidence-test | V023 | 0 | `logs/before-static-posix-blue-green-evidence-test-220255.log` |
| 2026-09-20T22:02:56+08:00 | 同步前/static-posix/blue-green-preflight-test | V024 | 0 | `logs/before-static-posix-blue-green-preflight-test-220256.log` |
| 2026-09-20T22:03:40.3717187+08:00 | 同步前/frontend/typecheck | V025 | 0 | `logs/before-frontend-typecheck-220340371.log` |
| 2026-09-20T22:05:04.9515283+08:00 | 同步前/backend/test-unit | V026 | 0 | `logs/before-backend-test-unit-220504951.log` |
| 2026-09-20T22:05:05.0097363+08:00 | 同步前/frontend/test | V027 | 0 | `logs/before-frontend-test-220505009.log` |
| 2026-09-20T22:05:30.7988256+08:00 | 同步前/canvas-build/build | V028 | 0 | `logs/before-canvas-build-build-220530797.log` |
| 2026-09-20T22:07:32+08:00 | 同步前/bluegreen-posix/blue-green-test | V029 | 0 | `logs/before-bluegreen-posix-blue-green-test-220732.log` |
| 2026-09-20T22:07:33+08:00 | 同步前/smoke-linux/内部收尾 | V010 | 0 | `before-smoke-results.jsonl` |
| 2026-09-20T22:07:33+08:00 | 同步前/smoke/health | V030 | 0 | `logs/before-smoke-health-220733.log` |
| 2026-09-20T22:10:27.4781158+08:00 | 同步前/frontend/build | V031 | 0 | `logs/before-frontend-build-221027478.log` |
| 2026-09-20T22:10:48+08:00 | 同步前/race/lifecycle-race | V032 | 0 | `logs/before-race-lifecycle-race-221048.log` |
| 2026-09-20T22:11:55.1386188+08:00 | 同步前/backend/build | V033 | 0 | `logs/before-backend-build-221155138.log` |
| 2026-09-20T22:13:52+08:00 | 同步前/integration/integration | V004 | 0 | `logs/before-integration-integration-221352.log` |
| 2026-09-20T22:13:52+08:00 | 同步前/race/stream-race | V005 | 0 | `logs/before-race-stream-race-221352.log` |
| 2026-09-20T22:15:57+08:00 | 同步前/race/lifecycle-race | V032 | 0 | `logs/before-race-lifecycle-race-221557.log` |
| 2026-09-20T22:25:37.2148253+08:00 | 同步后/generate/wire | V034 | 0 | `logs/after-generate-wire-222537214.log` |
| 2026-09-20T22:27:28+08:00 | 同步后/release-helpers/tests | V035 | 0 | `logs/after-release-helpers-tests-222728.log` |
| 2026-09-20T22:27:28.3671264+08:00 | 同步后/frontend/install | V002 | 0 | `logs/after-frontend-install-222728366.log` |
| 2026-09-20T22:27:29+08:00 | 同步后/release-helpers/syntax | V036 | 0 | `logs/after-release-helpers-syntax-222729.log` |
| 2026-09-20T22:27:30.8052441+08:00 | 同步后/frontend/lint | V006 | 0 | `logs/after-frontend-lint-222730804.log` |
| 2026-09-20T22:27:54.4492389+08:00 | 同步后/backend/test-default | V001 | 1 | `logs/after-backend-test-default-222754448.log` |
| 2026-09-20T22:27:54.8106743+08:00 | 同步后/canvas/install | V002 | 0 | `logs/after-canvas-install-222754810.log` |
| 2026-09-20T22:27:57.2704721+08:00 | 同步后/canvas/format | V007 | 1 | `logs/after-canvas-format-222757269.log` |
| 2026-09-20T22:28:06.2407560+08:00 | 同步后/canvas/typecheck | V008 | 0 | `logs/after-canvas-typecheck-222806240.log` |
| 2026-09-20T22:28:27.9359706+08:00 | 同步后/canvas/test | V009 | 0 | `logs/after-canvas-test-222827935.log` |
| 2026-09-20T22:29:01.8500030+08:00 | 同步后/frontend/typecheck | V025 | 0 | `logs/after-frontend-typecheck-222901849.log` |
| 2026-09-20T22:29:16.7528550+08:00 | 同步后/canvas-build/build | V037 | 0 | `logs/after-canvas-build-build-222916752.log` |
| 2026-09-20T22:29:17+08:00 | 同步后/race/stream-race | V005 | 0 | `logs/after-race-stream-race-222917.log` |
| 2026-09-20T22:29:20.7757027+08:00 | 同步后/backend/test-unit | V026 | 0 | `logs/after-backend-test-unit-222920775.log` |
| 2026-09-20T22:29:51.0841975+08:00 | 同步后/frontend/test | V027 | 0 | `logs/after-frontend-test-222951083.log` |
| 2026-09-20T22:33:48.1273353+08:00 | 同步后/extra/golangci-lint | V003 | 0 | `logs/after-extra-golangci-lint-223348126.log` |
| 2026-09-20T22:33:48.4656305+08:00 | 同步后/security/govulncheck | V013 | 0 | `logs/after-security-govulncheck-223348464.log` |
| 2026-09-20T22:34:20.7437856+08:00 | 同步后/frontend/build | V038 | 0 | `logs/after-frontend-build-223420743.log` |
| 2026-09-20T22:34:31.2847688+08:00 | 同步后/security/frontend-audit | V022 | 1 | `logs/after-security-frontend-audit-223431284.log` |
| 2026-09-20T22:34:35.5934925+08:00 | 同步后/security/canvas-audit | V022 | 1 | `logs/after-security-canvas-audit-223435593.log` |
| 2026-09-20T22:34:52.8170359+08:00 | 同步后/backend/build | V033 | 0 | `logs/after-backend-build-223452816.log` |
| 2026-09-20T22:35:34+08:00 | 同步后/race/lifecycle-race | V032 | 0 | `logs/after-race-lifecycle-race-223534.log` |
| 2026-09-20T22:36:56.5742455+08:00 | 同步后/backend/test-default | V001 | 0 | `logs/after-backend-test-default-223656573.log` |
| 2026-09-20T22:36:57+08:00 | 同步后/smoke-linux/内部收尾 | V039 | 0 | `after-smoke-results.jsonl` |
| 2026-09-20T22:36:57+08:00 | 同步后/smoke/health | V040 | 0 | `logs/after-smoke-health-223657.log` |
| 2026-09-20T22:38:05+08:00 | 同步后/bluegreen-posix/blue-green-test | V029 | 0 | `logs/after-bluegreen-posix-blue-green-test-223805.log` |
| 2026-09-20T22:38:06+08:00 | 同步后/static-posix/docker-compose-security-test | V012 | 0 | `logs/after-static-posix-docker-compose-security-test-223806.log` |
| 2026-09-20T22:38:06+08:00 | 同步后/static-posix/docker-compose-gateway-env-test | V014 | 0 | `logs/after-static-posix-docker-compose-gateway-env-test-223806.log` |
| 2026-09-20T22:38:10+08:00 | 同步后/static-posix/docker-runtime-resources-test | V015 | 0 | `logs/after-static-posix-docker-runtime-resources-test-223810.log` |
| 2026-09-20T22:38:10+08:00 | 同步后/static-posix/remote-deploy-test | V016 | 0 | `logs/after-static-posix-remote-deploy-test-223810.log` |
| 2026-09-20T22:38:13+08:00 | 同步后/static-posix/apple-container-test | V017 | 1 | `logs/after-static-posix-apple-container-test-223813.log` |
| 2026-09-20T22:38:14+08:00 | 同步后/static-posix/caddy-cache | V018 | 0 | `logs/after-static-posix-caddy-cache-223814.log` |
| 2026-09-20T22:38:14+08:00 | 同步后/static-posix/apple-syntax | V019 | 0 | `logs/after-static-posix-apple-syntax-223814.log` |
| 2026-09-20T22:38:14+08:00 | 同步后/static-posix/remote-syntax | V020 | 0 | `logs/after-static-posix-remote-syntax-223814.log` |
| 2026-09-20T22:38:14+08:00 | 同步后/static-posix/blue-green-test | V021 | 1 | `logs/after-static-posix-blue-green-test-223814.log` |
| 2026-09-20T22:38:34+08:00 | 同步后/static-posix/blue-green-evidence-test | V023 | 0 | `logs/after-static-posix-blue-green-evidence-test-223834.log` |
| 2026-09-20T22:38:34+08:00 | 同步后/static-posix/blue-green-preflight-test | V024 | 0 | `logs/after-static-posix-blue-green-preflight-test-223834.log` |
| 2026-09-20T14:41:20.476Z | 同步后/safety/final-check | V041 | 0 | `logs/after-safety-1789915281314.json` |
| 2026-09-20T22:41:54+08:00 | 同步后/integration/integration | V004 | 1 | `logs/after-integration-integration-224154.log` |
| 2026-09-20T22:53:53+08:00 | 同步后/disconnect-check/repeated-timeout | V042 | 0 | `logs/after-disconnect-check-repeated-timeout-225353.log` |
| 2026-09-20T22:54:08+08:00 | 基线快照复核/disconnect-check/repeated-timeout | V043 | 1 | `logs/before-disconnect-check-repeated-timeout-225408.log` |
| 2026-09-20T22:56:28+08:00 | 同步后/integration/integration | V004 | 0 | `logs/after-integration-integration-225628.log` |
| 2026-09-20T15:05:50.723Z | 同步后/safety/final-check | V041 | 0 | `logs/after-safety-1789916751536.json` |

### 修改文件

以下为M1相对同步前本地基线的136个文件；最后的M2另追加本同步历史，最终137个。

```text
A	.github/release-tools/.gitignore
A	.github/release-tools/README.md
A	.github/release-tools/release-images.sh
A	.github/release-tools/release_matrix.py
A	.github/release-tools/requirements-release.txt
A	.github/release-tools/test_release_matrix.py
M	.github/workflows/backend-ci.yml
M	.github/workflows/release.yml
M	.gitignore
M	Makefile
M	backend/cmd/server/wire_gen.go
A	backend/internal/handler/admin/content_moderation_engine_test.go
M	backend/internal/handler/admin/content_moderation_handler.go
M	backend/internal/handler/admin/openai_oauth_handler.go
M	backend/internal/handler/admin/openai_oauth_handler_reset_quota_test.go
A	backend/internal/handler/admin/openai_referral_handler.go
A	backend/internal/handler/admin/openai_referral_handler_test.go
M	backend/internal/handler/admin/plugin_handler.go
M	backend/internal/handler/endpoint.go
M	backend/internal/handler/grok_media.go
M	backend/internal/handler/grok_media_slots_test.go
A	backend/internal/handler/seedance.go
A	backend/internal/handler/seedance_test.go
M	backend/internal/pkg/apicompat/responses_to_anthropic_request.go
A	backend/internal/pkg/apicompat/responses_to_anthropic_tool_schema.go
M	backend/internal/pkg/apicompat/responses_to_anthropic_tools_test.go
A	backend/internal/pkg/typesafe/client.go
A	backend/internal/pkg/typesafe/client_test.go
M	backend/internal/repository/account_repo.go
A	backend/internal/repository/account_repo_codex_display_snapshot_test.go
M	backend/internal/repository/content_moderation_repo.go
M	backend/internal/repository/content_moderation_repo_test.go
M	backend/internal/repository/http_upstream.go
M	backend/internal/repository/http_upstream_http2_keepalive_test.go
A	backend/internal/repository/http_upstream_http2_ping_test.go
A	backend/internal/repository/openai_referral_client.go
A	backend/internal/repository/openai_referral_client_test.go
A	backend/internal/repository/plugin_kv_store.go
A	backend/internal/repository/plugin_kv_store_test.go
M	backend/internal/repository/wire.go
M	backend/internal/server/routes/admin.go
M	backend/internal/server/routes/gateway.go
M	backend/internal/server/routes/gateway_model_allowlist_test.go
A	backend/internal/server/routes/seedance_test.go
M	backend/internal/service/account.go
M	backend/internal/service/account_usage_service.go
M	backend/internal/service/account_usage_service_batch_test.go
M	backend/internal/service/account_usage_service_spark_shadow_test.go
M	backend/internal/service/antigravity_gateway_gemini.go
M	backend/internal/service/antigravity_gateway_streaming.go
A	backend/internal/service/antigravity_gemini_thinking_variant.go
A	backend/internal/service/antigravity_gemini_thinking_variant_test.go
M	backend/internal/service/content_moderation.go
A	backend/internal/service/content_moderation_engine_custom_test.go
A	backend/internal/service/content_moderation_engines.go
A	backend/internal/service/content_moderation_engines_test.go
M	backend/internal/service/content_moderation_input.go
A	backend/internal/service/content_moderation_reminder_test.go
A	backend/internal/service/content_moderation_typesafe.go
A	backend/internal/service/content_moderation_typesafe_live_test.go
M	backend/internal/service/gateway_forward_as_responses_test.go
A	backend/internal/service/gemini_sse_comment_compat.go
A	backend/internal/service/gemini_sse_comment_compat_test.go
M	backend/internal/service/grok_media.go
M	backend/internal/service/openai_codex_version_sync_service_test.go
M	backend/internal/service/openai_gateway_cc_pipeline.go
M	backend/internal/service/openai_gateway_chat_completions_raw.go
M	backend/internal/service/openai_gateway_chat_completions_raw_test.go
A	backend/internal/service/openai_gateway_deepseek_chat_reasoning_test.go
M	backend/internal/service/openai_gateway_passthrough.go
M	backend/internal/service/openai_gateway_request_body.go
M	backend/internal/service/openai_gateway_response_handling.go
M	backend/internal/service/openai_gateway_responses_chat_fallback.go
M	backend/internal/service/openai_gateway_service_test.go
A	backend/internal/service/openai_plugin_account_directory.go
A	backend/internal/service/openai_plugin_account_directory_test.go
A	backend/internal/service/openai_quota_credits_test.go
M	backend/internal/service/openai_quota_reset_credits_test.go
M	backend/internal/service/openai_quota_service.go
M	backend/internal/service/openai_quota_spark_window_test.go
A	backend/internal/service/openai_referral_client.go
A	backend/internal/service/openai_referral_service.go
A	backend/internal/service/openai_referral_service_test.go
A	backend/internal/service/openai_response_model_rewrite_test.go
M	backend/internal/service/openai_ws_forwarder_ingress_execution_scope_test.go
A	backend/internal/service/plugin_host_services.go
A	backend/internal/service/plugin_host_services_broker_test.go
A	backend/internal/service/plugin_host_services_test.go
M	backend/internal/service/plugin_manager.go
M	backend/internal/service/plugin_manager_routing_test.go
M	backend/internal/service/plugin_runtime.go
M	backend/internal/service/plugin_runtime_integration_test.go
M	backend/internal/service/ratelimit_cn_providers.go
M	backend/internal/service/ratelimit_service.go
M	backend/internal/service/ratelimit_service_401_test.go
A	backend/internal/service/ratelimit_service_cn_quota_403_test.go
A	backend/internal/service/seedance.go
A	backend/internal/service/seedance_test.go
M	backend/internal/service/setting_gateway_runtime.go
M	backend/internal/service/wire.go
A	backend/migrations/238b_content_moderation_engine_meta.sql
A	backend/migrations/content_moderation_engine_meta_test.go
M	backend/pkg/pluginapi/README.md
M	backend/pkg/pluginapi/docs/ui-bridge.md
M	backend/pkg/pluginapi/v1/plugin.pb.go
M	backend/pkg/pluginapi/v1/plugin.proto
M	backend/pkg/pluginapi/v1/plugin_grpc.pb.go
M	backend/pkg/pluginapi/v1/runtime.go
M	docs/custom-development-history.md
A	docs/seedance-api.md
M	frontend/src/api/__tests__/client.spec.ts
M	frontend/src/api/admin/accounts.ts
M	frontend/src/api/admin/plugins.ts
M	frontend/src/api/admin/riskControl.ts
M	frontend/src/api/client.ts
M	frontend/src/components/account/BulkEditAccountModal.vue
M	frontend/src/components/account/CreateAccountModal.vue
M	frontend/src/components/account/EditAccountModal.vue
M	frontend/src/components/account/OpenAIQuotaResetCell.vue
A	frontend/src/components/account/OpenAIReferralCell.vue
M	frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts
M	frontend/src/components/account/__tests__/EditAccountModal.spec.ts
M	frontend/src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts
A	frontend/src/components/account/__tests__/OpenAIReferralCell.spec.ts
A	frontend/src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts
M	frontend/src/components/layout/AppHeader.vue
M	frontend/src/i18n/locales/en/admin/accounts.ts
M	frontend/src/i18n/locales/en/admin/channels.ts
M	frontend/src/i18n/locales/zh/admin/accounts.ts
M	frontend/src/i18n/locales/zh/admin/channels.ts
M	frontend/src/i18n/locales/zh/common.ts
M	frontend/src/types/index.ts
A	frontend/src/types/openaiReferrals.ts
M	frontend/src/views/admin/PluginsView.vue
M	frontend/src/views/admin/RiskControlView.vue
M	frontend/src/views/admin/__tests__/RiskControlView.spec.ts
```

### 未验证与最终门禁

- 未验证：真实Provider、真实TypeSafe、真实邀请/积分兑换、支付/邮件/第三方回调、插件真实部署、完整浏览器业务E2E、生产数据库及服务、原生macOS、多架构真实Release、GitHub Actions在线工作流、全包race与峰值负载。没有由mock结果推断这些链路全部正常。
- 生产Go软内存与PostgreSQL缓存叠加的既有OOM风险继续存在，本轮没有压力验证，也没有调整预算。Seedance查询结算与插件凭据边界按获批方案保留；依赖告警和Canvas格式问题继续作为已知基线。
- Claude断连超时用例的既有计时波动未在本轮修复；基线快照已复现相同失败，不能把重复测试通过描述为已消除该稳定性风险。
- 形成M1前没有未解决Git/语义冲突，相对同步前没有尚未修复的新增测试失败，关键门禁均通过。备份保留原HEAD，main在记录提交前仍为LOCAL_PRE_SYNC_SHA，未提前更新。
- M2提交后再次检查main未变且工作树干净，只执行git switch main和git merge --ff-only同步分支；若不满足则停止，绝不强制更新。最终记录SHA、main SHA及清理核验在对话报告。
- 本轮隔离smoke进程和专有标签容器已清理，18973无监听；Testcontainers容器已回收，Docker最终仅见原有其他项目容器及历史停止容器。POSIX临时挂载初检非空而保留，进一步确认仅余两个无监听者的插件测试socket后解除挂载；未删除原有其他项目容器、缓存、日志或工作资料。
- Git执行摘要：固定SHA的git merge --no-ff --no-commit退出1（预期12个文本冲突）；逐块apply_patch、Wire生成和兼容修正完成后，git add及暂存差异检查退出0，git commit创建M1退出0。M1的双父关系已核对，最后的记录提交及ff-only结果在最终对话交付。

## 2026-09-21 同步后发布兼容门禁

- 上轮完整同步提交 `8b6b56f043def67860b8f1a0a487d0d69844ecae` 已在候选与主线 CI、安全扫描全部成功后快进合入 `main`。`LAST_FULLY_INTEGRATED_UPSTREAM_SHA` 仍为 `7c700729c23187d31ed320f6b19c790e2f194826`，未扩大上游范围。
- 用户继续授权更新标签并经 GitHub Actions 蓝绿部署。预检发现 `238b_content_moderation_engine_meta.sql` 被既有“无迁移”门禁拒绝，尚未推送标签。2026-09-21 用户在了解共享数据库和锁表风险后批准旧版兼容的字段扩展。
- 新增 `CUST-OPS-007` 及兼容审批文档，不修改上游原始 SQL、旧迁移校验和或业务默认值。部署门禁只接纳受限新增可空字段且需绑定旧代码审查；其他变化继续拒绝。
- 候选启动完整待执行清单校验、NOWAIT 和短事务预算、禁用失败候选自动重启纳入本次实现。测试及实际发布结果另行补记，不把本地实现写成生产已完成。
- 本地门禁验证退出0：Python证据6项、状态机51项、预检3项，Go迁移定向单元和隔离PostgreSQL集成测试，golangci-lint无问题。实际生产迁移清单只读比对为唯一238b，原SQL摘要保留；现有一小时强制退役策略、资源参数和数据目录均未修改。
