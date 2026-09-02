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
