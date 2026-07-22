# 上游同步历史

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
- 执行状态：同步分支完整合并并验证完成；本记录提交后使用 `--ff-only` 更新本地 `main`
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
