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
