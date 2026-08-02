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
