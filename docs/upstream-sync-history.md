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
