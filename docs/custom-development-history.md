# 二次开发功能台账

本文档是本仓库相对上游 `Wei-Shaw/sub2api` 的二次开发功能事实来源，用于回答以下问题：

- 当前有哪些仍需独立维护的二次开发能力；
- 每项能力的行为边界、默认状态和主要代码入口是什么；
- 上游同步、功能修改、停用或删除时，哪些本地行为必须保留或重新评估；
- 某项二次开发能力在何时、因何发生过变化，以及使用什么方式验证。

上游提交的逐项处置继续记录在 `docs/upstream-sync-history.md`。本文件不重复上游提交清单，只记录本地能力及其演进。

## Kimi 参数兼容重试（待发布）

新增四个独立网关开关，默认关闭，分别兼容上游明确拒绝采样参数、非法思考强度、非法工具选择和思考预算冲突。仅 API Key 所属类型为 `kimi` 的分组及 Chat Completions 出站生效，覆盖 Messages/Responses 转换，不影响综合分组和原生其他协议。

只匹配实际 HTTP 400 的结构化明确错误，并核对出站字段和值；泛化400、引用文案、歧义JSON、额度/审核错误不匹配。每项每请求最多一次，共最多额外四次；普通重试和切号保留修正，既有 pool 次数不变。已提交响应禁止重放，中间失败不泄露到下游。思考兼容开启时，K3 的 Messages 默认强度改为 `low`；现有 Kimi 显式 `max` 保留行为不变。

通过共享 CC 入口管理响应关闭、取消和首 Token 资源，使用现有上游尝试链与 Recovered 展示。删除 `max_completion_tokens` 会解除该输出上限，可能增加输出和费用，须由管理员主动启用。配置、匹配、三个入口、限次、流安全和恢复日志均有回归，Kimi 用例加入流竞态 CI；本地测试不能替代生产第二次实际派发与完整恢复验收。详细说明见 `docs/kimi-parameter-compatibility.md`。本次不自动启用线上开关或部署。

## Claude透传流终止后的请求清理（2026-09-17，待发布）

针对客户端已经收到完整 `message_stop`，HTTP 响应却延迟结束的问题，修复响应体清理时先 `Close`、后取消本轮上游请求的顺序。底层关闭若依赖读取退出，会与尚未取消的请求相互等待；现在先执行取消与定时器清理，再关闭底层响应体，并保证并发关闭仅执行一次、保留原关闭错误。安全重试关闭时也为每次流式派发创建独立取消句柄，不依赖首 Token 策略是否开启。

只在既有终止判定与响应体释放路径中收尾，不修改终止帧识别，不跳过最终用量，不取消客户端上下文或跨尝试共享预算，不重放已经提交的输出。HTTP 错误正文仍先读取再释放。修复不要求客户端主动断开，不修改压缩协商或线上重试配置。

回归先在旧实现复现取消等待，再验证修复；覆盖并发读取/关闭、EOF/读取错误、关闭错误，以及安全重试和首 Token 策略开关组合。真实 handler 与仓库 HTTP 传输测试覆盖下游 HTTP/1.1、HTTP/2，及上游 identity、gzip、zstd：上游发送完整终止帧后保持响应未结束，客户端仍能读到正常 HTTP EOF，最终用量完整且仅记录一次。新用例纳入现有流竞态 CI 门禁。

本地验证：24 组真实 HTTP 组合、流重试及压缩关闭竞态连续 5 轮、首 Token 相关竞态、后端全量 unit/integration、golangci-lint（0 issues）和服务端构建均通过。开发基线为远端 `main` 的 `fb9712bd4`；远端 CI 和发布后验收另行记录。

验收边界：本地回归不能替代生产验收，也尚未取得生产阻塞时的 goroutine 堆栈；发布后仍需以客户路径的 `message_stop → HTTP EOF` 耗时、服务端请求耗时及单次入账记录确认效果。此记录不代表已经发布。

## Claude安全流重试缓存与首有效内容超时（2026-09-16，待发布）

修复调度候选投影遗漏 `anthropic_passthrough`、`pool_mode`、`pool_mode_retry_count`，导致安全流重试误判无可用账号的问题。新写入缓存保留这些资格字段，旧投影缺字段时从完整账号补全，继续遵守既有调度资格，不要求删除共享Redis缓存。

网关配置新增 `anthropic_stream_safe_retry_first_content_timeout_seconds`，默认180秒，范围0—3600秒，0关闭；仅在Claude透传流安全重试总开关开启时生效。每次成功流响应头后独立计时，心跳、空前导和纯空白不续期；文本、思考、有效工具调用或合法终止事件结束等待。已提交未知事件但未识别有效内容时，到期仅终止、不重放。总预算扩为1—3600秒，默认仍300秒，跨尝试共享；第一轮等待180秒后，300秒总预算仅剩约120秒，不自动延长。

新错误分类 `first_content_timeout` 与上游180秒空闲超时、原有分组首Token策略分开记录。保留提交后禁止重放、256KiB前导上限、独立重试次数及最终一次用量计量。旧管理客户端未传新字段时保留现值，旧存储缺字段时使用180秒默认值；请求冻结配置，运行中不受配置更新影响。

新增缓存序列化/Redis集成、旧投影同号与换号、心跳及空白虚拟时钟、共享预算、前端设置与诊断测试；处理器回归验证第一轮持续心跳超时、第二轮成功，实际两次上游派发、恢复标记为真且仅交付一次成功流和计量一次。本次只提交代码并验证CI，不创建发布标签、不修改生产配置或部署；上线验收应核对 `attempt=2` 与恢复成功，而非仅看 `decision=retry`。

## 一小时观察与超时强制退役（2026-09-16，新授权）

关联 `CUST-OPS-001`、`CUST-OPS-003`。用户将发布策略改为观察一小时，超时允许强制退役。Release执行器新增固定旧实例身份绑定、有界Docker停止、按旧实例清理Redis归属、残留socket清理和可恢复的强制结果；Actions部署上限改为120分钟。正常自然排空路径保留；强制退出不得宣称客户无中断或用量完整，不修改正常请求超时与历史账单。

本次旧green v0.1.235已于17:49回收，退出137且非OOM；17:50核验旧归属0、18082空闲、state为stable/active=blue/pending=null，blue未重启。两段观察共304次健康探针零异常，不等于被强制中断的旧请求零错误。首轮归属扫描超时后已优化批量并加入游标恢复。新策略需包含在后续tag对应的GitHub提交中才对未来Actions生效；完整边界、测试与生产证据见 `docs/operations/2026-09-16-hourly-forced-retirement.md`。

## Nginx 连接容量与分配修复（2026-09-16，已生产生效）

关联 `CUST-OPS-003`。经用户授权，15:50:27 在 server1 平滑加载 `worker_connections 4096 → 16384`、`multi_accept on → off`，其他代理参数和应用/数据库上限不变。当前 Nginx 1.26.3 的 epoll/EPOLLEXCLUSIVE 接入路径在批量接入遇到 EAGAIN 时提前返回，绕过 worker 轮转；运行时事件注册、匹配版本源码及现场前后对照共同支持该原因。

变更前双入口各64条独立健康连接分别集中于1个/2个worker；变更后均分布到64个worker。跨重载714次HTTP/1.1与HTTP/2探针全部成功，没有新增连接耗尽告警。master、应用容器启动时间及蓝绿状态不变，旧worker保留未结束工作自然排空，不强杀。短时验证不等于峰值容量或所有业务零错误，完整证据与回退边界见 `docs/operations/2026-09-16-nginx-worker-balance.md`。

## 生命周期与蓝绿实施进度（2026-09-15，未发布）

功能分支 `codex/graceful-blue-green-deploy` 已合入远端主线 `09523d260`，保留主线 Claude 流安全重试、诊断与安全依赖更新，没有文本冲突。实现应用排空、用量停止兜底、多实例并发状态、Unix socket 会话转发、可逆排空与退役凭据；备份记录改为跨实例互斥保存。Release 已改接固定 SHA/digest 的蓝绿执行器，并保留首次迁移、资源与数据库兼容性门禁。

本轮新增 Linux 实际应用 + Nginx/PG/Redis 的 20 轮混合协议切流门禁，与 Python 编排状态机测试分别报告。本机 SSH 多次握手超时，后通过只读 Actions run `35025763779` 于 2026-09-15 21:28:03 UTC 重新读取生产基线：仍为 v0.1.234 单实例，蓝绿 config/state 缺失，应用池 512 / PG 600 与单应用约 80% 内存软上限不支持原参数双开。未创建标签、调整生产配置或执行迁移。用户已追加授权 CI 通过后发布，但不会以此绕过未知的旧实例排空或资源预算。实现、测试覆盖与首次迁移条件见 `docs/operations/graceful-blue-green-deploy.md`。代码候选 `fb885cd1d` 的 CI run `35024366357` 与 Security Scan `35024366346` 均通过；Linux 20 轮双入口切流为 56–188 毫秒（中位数约 123 毫秒），测试 TTL=5 秒下排空为 5.14–8.37 秒，不作为生产 SLA。此段仍为未生产交付的进度记录，不计入已交付能力族统计。

CI 迭代：修复 WS 验收测试在握手与服务端 Hijack 之间的同步竞态，改用 handler 返回屏障并断言 HTTP 已结束而 WS 仍在。新增仅手动触发的生产只读 Actions 预检，固定 server1、共享发布互斥、仅输出容器/资源白名单；不改变生产、不替代首次迁移审定。

## 蓝绿首次自动迁移修正（2026-09-16，v0.1.235 已部署）

按用户追加要求取消“配置上限简单相加”的硬拒绝，保留单实例连接池/内存上限，改按真实资源余量检查；tag Release 自动发现旧版并生成首次 config/state，保存可恢复 journal，保护现有代理/挂载。新增固定 Unix 旧版续接通道、WS 接入租约交接与旧版共存共享任务抑制；旧版无排空证明则保留并阻止槽位复用，不假造成功退役。macOS job 仍只做检查，生产部署全程由 GitHub Actions 执行。实现与验收边界见 `docs/operations/graceful-blue-green-deploy.md` 最新修正章节。

`v0.1.235` 于 2026-09-16 通过 Release run `35050536184` 自动首次迁移：活动实例切到 `green`，切流确认 1.148 秒，API/direct 连续健康探针各 42 次且零错误；旧 v0.1.234 因无生命周期凭据保留为 `drain_pending / legacy_unverifiable`，没有被强停。部署后只读 run `35052231141` 确认新实例 revision 为 `961fd4160a94fdd61328ca99c6b13b9267b9ed38`，两个入口均返回同一新实例和 `green` 槽标识，连接池 512、GOMEMLIMIT 216181080064B 未降低。详见 `docs/operations/2026-09-16-v0.1.235-blue-green-release.md`。

## 旧版退役门禁与固定双槽循环（2026-09-16，v0.1.237 已切流）

为保证下一 tag 仍可直接由 GitHub Actions 自动发布，增加一次性 legacy-retire 门禁和独立手动观察工作流。Release 自身可在无预热凭据时等待最长 1200 秒并验证 900 秒客户路径静默；候选镜像必须先准备，凭据绑定旧容器及当前活动实例，中断恢复不得在旧版停止后重新拉镜像。退役完成后复用 `blue/18080`，后续只在 18080/18082 循环，不新增端口。server1 短采样已确认 TTL、旧 worker、客户连接、legacy socket 和 Redis 归属均满足；正式停止结果、异常日志计数和下一 tag 切换仍待 CI 及生产发布验证。详见 `docs/operations/2026-09-16-legacy-retire-gate.md`。

发布前补强：观察重跑不触碰 access log 时间；无效代理配置恢复原文；退役重跑重新 reload 并确认 legacy Unix listener 消失；日志读取失败按取证不完整判失败，不假报零丢弃。新增恢复/日志回归后，状态机 27 项、发布证据 2 项及预检 3 项测试通过。

CI 测试夹具修复：隔离 Nginx 初次 warmup 曾在 TLS 就绪前执行而失败，现先等待双入口 `/health` 成功再进入生成/切流验收；不重试生成请求，不削减 20 轮协议及用量核对门禁。

`v0.1.236` 首次部署迭代：production Docker CLI 26.1.5 不支持 `stop --timeout`，退役在发送停止信号前安全失败，旧容器与 green 均健康、未切流。修正为兼容的 `stop -t -1`（仍无限等待），并允许后续通过完整预检且镜像已准备的新 tag 接续尚未启动候选的 legacy 退役操作；候选存在或预检失败则保留原发布身份与流量。

生产验证：独立门禁 run `35057663303` 完成 901.6 秒静默；固定提交 `004cc47a2` 的完整 CI/Security 全绿后发布 `v0.1.237`。Release run `35061507913` 自动恢复退役：v0.1.234 退出码 0、用量丢弃 0、强制关停 0，111.84 秒等待后台任务后删除旧容器；blue 复用 18080，双入口切流确认 1.179 秒。green v0.1.235 仍按真实工作与会话自然排空，不能强占；下一发布候选固定为 green/18082。详情及边界见 `docs/operations/2026-09-16-legacy-retire-gate.md`。

验收边界：Release 最终成功，状态为 `drain_pending / active=blue / pending=green`；API/direct 各 977 次健康探针分别有 7/6 次 TLS 或 HTTP 500 异常，集中在切流约 15 分钟后。Nginx 同时记录 `4096 worker_connections are not enough`，切流前亦有同类告警。未宣称整体零错误、未强制回收 green、未擅自更改全局 Nginx 并发参数。

## 当前基线

- 生产部署核验（2026-09-16）：`0.1.237`、revision `004cc47a2090a35b7cc94a6463e97273b86562fb` 在 blue/18080 接收新流量；v0.1.234 已正常退役，v0.1.235 green 按真实工作自然排空。资源上限、PG/Redis 和持久挂载保持不变。详情见 `docs/operations/2026-09-16-legacy-retire-gate.md`。
- 上一版生产记录（2026-09-15）：v0.1.234 原地部署时观察到 HTTP 强制关停及 `usage_record.task_dropped`（`reason=stopped`）告警；本次生命周期与蓝绿修正的目标即消除计划发布中的这类问题。历史证据保留于 `docs/operations/2026-09-15-v0.1.234-release.md`。

- 基线日期：2026-09-18（北京时间，本地同步）
- 本地版本：`0.1.239`（本次上游同步仅在本地执行，未发布；生产事实沿用上方既有核验记录）
- 本地同步前代码基线提交：`23f34132d5e591007bf8ac8472918fec3af871e5`；本轮合并提交与验证结果见 `docs/upstream-sync-history.md` 的 2026-09-18 记录
- 已完整集成的上游提交：`efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`
- 本次同步范围：`bdb42e22f..efe9aab1e`，108 个上游提交；58 项已有能力族编号保留，另保留上方尚未编号的 Kimi 参数兼容与蓝绿生命周期行为；可用验证与未覆盖范围见本轮记录
- 历史识别统计（2026-09-02，`5097b3145..35c28e324`）：919 个差异文件，新增130491行、删除5976行、本地独有512个提交；此为历史快照，不代表本次同步后的统计
- 2026-09-05 历史代码基线差异（`ab99d56e9..289a00a46`）：926个文件，新增130999行、删除6037行；本地独有517个提交（不含最后记录补记）
- 当前能力族：58 项，分布在 9 个功能域；本次只校正既有编号统计，不新增、复用或删除能力编号

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
| `CUST-GW-001` | 首 Token 超时与请求结果归因 | 在首个有效下游语义输出前执行可配置超时，metadata、preamble 和 keepalive 均不解除守卫；compact fallback 内部信号会撤销首次尝试暂存的响应，避免第一次失败提交空 HTTP 200，二次 fallback 失败返回真实错误；区分 pending/final outcome、部分响应、客户端断开和流内失败，避免错误重试或漏结算。WS 透传按轮等待合法终态，过早 close 不误判成功。HTTP client 从真正发起上游请求时开始计时，非流式 CC 不创建守卫。指定分组模式按 `CUST-GW-012` 忽略前导纯空白，保留原始空白直到有效输出一起交付；全局模式保持原有首输出语义。 本次上游适配：本次接入 pending-turn 与取消归因时仍保留逐轮语义输出、合法终态和一次结算规则。 2026-09-15 上游适配：接入原生 Codex Images 和 Gemini 带内错误登记时保留首语义输出、attempt 完结及部分响应归因。 | `backend/internal/service/first_token_timeout.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/handler/openai_upstream_outcome_tracker.go` | 生效中 |
| `CUST-GW-002` | 2xx 语义错误识别 | 按 contains/regex 规则检查有限长度的成功响应体，将命中的业务错误转为 failover、账号测试失败和可审计错误。 | `backend/internal/service/semantic_error.go`、`backend/internal/service/semantic_error_config.go` | 生效中 |
| `CUST-GW-003` | 流式心跳与长请求保活 | 支持响应前心跳、普通 Responses 与 compact 的 SSE 心跳（含透传首输出前保活）和 Images JSON keepalive；首 Token 计时期间按路径暂停会提前提交状态的心跳，写出判定扣除非语义 keepalive 字节，客户端断开后可继续排空上游完成计费。 本次上游适配：HTTP/2 PING 保活纳入上游连接配置；断连排空仍与首输出守卫协同，不恢复已断开的下游写入。 2026-09-15 上游适配：WS 常驻读循环应答 ping；原生图片流仍按交付/断连状态处理，保活不代替语义输出。 2026-09-15 本地修复：Anthropic API Key 透传改为按最后一行上游数据的剩余空闲时间精确计时，消除180秒阈值可能拖到近360秒才检查的偏差；上游心跳/注释/空行继续续期，下游保活不续期，完整终止与断连排空语义不变。安全重试按 `CUST-GW-013` 独立启用；终止判定进一步要求完整事件分隔空行，事件名称行或截断终止不视为成功。 | `backend/internal/service/gateway_anthropic_passthrough.go`、`backend/internal/service/openai_compact_sse_keepalive.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_images_json_keepalive.go`、`backend/internal/service/gateway_upstream_response.go` | 生效中 |
| `CUST-GW-004` | Pool 模式重试与失败策略配置 | 在二开管理页配置默认重试次数、可重试状态码、首 Token 阈值与生效分组、上游错误阈值和自动探测退避；Claude 在首个语义输出前收到 SSE `overloaded_error` 时按 529 进入同账号重试或换号；使用策略 revision/fingerprint 避免运行时读到撕裂配置。首 Token 超时继续跳过当前账号池内重试，换号次数沿用原配置，不新增三账号上限。 | `backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-005` | 自动托管账号恢复探测 | 为可恢复停调度账号创建或恢复唯一的自动测试计划，支持二开配置的阶梯退避、默认提示词和成功后停用。故障期间（含同时人工暂停）普通计划只推进下次 Cron，不发送重复探测、不伪造执行记录、不提前刷新系统计划的退避时间；同实例按账号互斥执行，并复核旧到期快照，重复启用通知保留已有退避。退避计数与时间在账号行锁下原子持久化到现有 extra 的 `auto_managed_probe_state`，不受测试结果保留数限制，不复制到新账号；旧执行不能覆盖新事故或人工修改后的计划。指定分组首 Token 事故使用快照中的模型和阈值，要求有效首输出达速且最终成功；首输出后不限制总流时长，探测不写业务 streak，旧结果不能恢复新事故或人工暂停。 | `backend/internal/service/admin_scheduled.go`、`backend/internal/service/scheduled_test_runner_service.go`、`backend/internal/service/account_test_first_token_recovery.go`、`backend/internal/repository/scheduled_test_repo.go`、`backend/internal/repository/scheduled_test_retry.go`、`backend/migrations/176_gateway_auto_managed_plan_uniqueness.sql` | 生效中 |
| `CUST-GW-006` | 连续失败停调度保护 | 记录账号错误 streak，达到阈值后临时移出调度；普通请求成功、人工清错和明确的状态恢复可清理对应状态。指定分组首 Token 策略只允许受保护请求改变该类 streak，其他分组、非流式或 WS 结果不清零；停调度仍为账号全局生效，未改为分组局部禁用。管理员调度开关通过现有 extra 的 `manual_scheduling_paused` 与系统故障停调度区分，单账号暂停同时停用系统测活；后台成功恢复及条件清除事故不得覆盖该标记，批量调度开关同样写入标记。OpenAI Team 联动熔断先执行并去重，再与本地 strict failure scheduling、自动测活和调度 outbox 协同；自动用卡不视为通用测试成功，不清除 error、overload、临时停调度、模型限流、failure marker/streak、人工停调度或自动测活事故。任一 WS 轮次失败并触发停调度后，同一会话的后续轮次被阻断。失败即释放会话槽；固定账号模型清单的只读资格检查不代替真实推理的停调度门禁。 2026-09-15 上游适配：WS 新执行作用域、先关闭再取消与本地逐轮失败停调度共同执行，不清除人工暂停或绕过下一轮准入。 2026-09-18 上游适配：Ollama 429 用量探测独立注入限流服务，与本地连续失败缓存及自动探测字段共存；不解除人工暂停，也不以旧探测结果覆盖新事故。 | `backend/internal/service/account_failure_streak.go`、`backend/internal/repository/account_failure_streak_cache.go`、`backend/internal/repository/account_manual_scheduling.go`、`backend/internal/service/failure_scheduling_compat.go`、`backend/internal/service/ratelimit_service.go` | 生效中 |
| `CUST-GW-007` | OpenAI 高级调度器 | 支持 sticky weighted、订阅优先、Top-K 和 priority/load/queue/error-rate/TTFT/reset/quota/session 等权重；另有优先最近重置策略。 2026-09-18 上游适配：接入 Codex 规范窗口配额读取、选号耗时及粘性命中计数修复，继续保留本地调度与故障恢复约束。 | `backend/internal/service/openai_account_scheduler.go`、`backend/internal/service/setting_parse.go` | 生效中 |
| `CUST-GW-008` | 增强故障转移与响应结算 | 覆盖非 JSON 2xx、SSE `event:error`、流式/非流式 Responses `error`/`response.failed` 与传输错误、请求级容量错误、图片服务错误、流式部分结果和已提交响应；仅在安全重建上下文时允许 WS 后续轮次换号，复用错误体并避免重复写帧，客户端正常关闭不计为账号故障。WS 每轮重新取得请求模型和渠道映射、记录 payload hash 并独立结算 usage；失败停调度后阻断后续轮。OAuth transport 插件一旦报告 `RequestSent=true`，HTTP、Images、Live 及流式路径均禁止换号、同号重试、compact retry 或响应体读取错误重放；已收到合法终态时，迟到的 close error 不覆盖成功。 接入不可用 continuation、历史委派和心跳初始化恢复。图片仍强制将真实上游 URL 转为内联 data URL/b64；新可选回填开关默认关闭，关闭它不关闭既有 URL 隐藏。两条图片下载链均使用下载专属公网防护，强制内联链仍按原失败/交付规则处理，不改变业务 HTTP/私网 upstream 开关。WS Cyber 失败终态返回错误并保留部分 usage，不伪装成成功。 本次上游适配：WS 每轮白名单、请求定价、Cyber 和停调度校验继续生效；通过策略后只占槽一次。终态成功写入后才触发结算，断连排空同时保留部分 usage 和失败归因；Live 新增账号参数不放宽 RequestSent 禁重放。 2026-09-15 上游适配：新增原生 Codex Images 分支并保留 ImageDelivered、ClientDisconnect、部分 usage 与端点元数据；RequestSent 禁重放及普通图片 URL 内联防护不变。 | `backend/internal/handler/failover_loop.go`、`backend/internal/service/upstream_outcome_error.go`、`backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/service/openai_plugin_transport.go` | 生效中 |
| `CUST-GW-009` | 代理到期与失败回退 | 代理支持到期处理、质量测试、失败回退和定向调度刷新；生产调度不会继续使用已失效代理。 | `backend/internal/service/proxy_expiry_service.go`、`backend/internal/service/proxy_fallback.go`、`backend/internal/repository/proxy_repo.go` | 生效中 |
| `CUST-GW-010` | 高并发请求体驻留治理 | OpenAI Responses 在首个有效语义输出确认后释放 handler、HTTP/WS 请求和解析视图持有的大请求体；此前完整保留请求体，并在 HTTP 与 WS HTTP bridge 统一暂存账号相关响应头和前导帧，用于安全故障转移。超大 passthrough 请求通过 bridge 下发时仍按轮保留到安全释放点，成功会话审计开启时保留审计所需的原始体。 WS 回放改为共享不可变体并记忆跨轮拒绝的加密内容；本地首语义输出前保留、输出后安全释放及审计保留边界不变。 本次上游适配：释放 WS 原始请求体前快照 service_tier、最终及原始 reasoning effort；成功、失败和断连结果共用完整元数据生成路径。 2026-09-15 上游适配：OpenCode 只保存清洗后的短会话 ID 快照，转换后请求不能覆盖原快照；与本地请求体释放/清理一同释放，不因会话标识保留完整大请求。 | `backend/internal/handler/openai_gateway_handler.go`、`backend/internal/service/openai_gateway_forward.go`、`backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_ws_forwarder_v2.go` | 生效中 |
| `CUST-GW-011` | 可配置附加换号状态码 | 在二开网关配置中按全局开关补充进入账号换号流程的 HTTP 状态码，默认关闭且预填 451；配置只扩展内置规则，保留上下文超限、Grok 内容拒绝等请求级语义终止保护。 | `backend/internal/service/gateway_additional_failover_status.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-012` | 指定分组首 Token 超时保护 | 在二开网关配置中选择全部分组或指定分组，默认 `all` 保持升级前行为；指定组至少选一个有效分组，依据请求上下文中的实际分组判断，不依据账号归属组。仅覆盖既有 HTTP 流式文本路径，不扩展 WS、非流式和媒体；前导纯空白不解除守卫，超时直接换号并沿用原换号上限。选中分组共用现有秒数、连续次数阈值，非受保护请求不增减首 Token streak；达到阈值后账号在所有关联分组停调度，按事故模型和时限测活恢复。范围变化纳入策略代次，旧客户端省略范围时不扩大为全局；全局指纹及原首输出行为兼容。与 `CUST-GW-013` 同时启用时由单层前导缓存协作；纯空白仍不解除首 Token 计时，但已交付不可撤销帧后的超时明确报错，不再换号重放。 | `backend/internal/service/first_token_timeout_scope.go`、`backend/internal/service/custom_feature_settings.go`、`backend/internal/service/account_test_first_token_recovery.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-013` | Claude 透传流安全重试 | 仅对 Anthropic API Key HTTP 透传流提供独立开关，默认关闭，额外重试默认2次（0–5）、总预算默认300秒（1–3600）。首次成功流响应头到达后跨尝试累计预算；仅缓存可证明无有效内容的前导事件（含待判定帧，累计上限256 KiB），先原号再换号，无替代时允许仍可调度且允许同号重试的原号使用剩余额度。交付任何不可撤销内容前关闭重放资格；完整终止帧立即收尾，缺终止、半帧、协议错误不伪装成功。保留首 Token 分组语义、取消、账号资格与按请求用量去重，错误详情展示脱敏尝试诊断。可选提前保活见 `CUST-GW-014`，HTTP 响应开始与内容提交独立判断，原有首字统计不变。 | `backend/internal/service/anthropic_stream_safe_retry.go`、`backend/internal/service/anthropic_stream_guard.go`、`backend/internal/handler/anthropic_stream_safe_retry.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中（默认关闭） |
| `CUST-GW-014` | Claude 安全重试提前 JSON 保活 | 在安全重试开启时，可选于首次成功流响应后立即发送标准 JSON `ping`，流读取期间按保活间隔继续发送（间隔未配置时使用10秒）。只发送无尝试身份的保活，暂存 `message_start` 与空前导；交付有效内容或未知不可撤销事件后禁止重放。保活不延长有效内容、空闲、首 Token 或总等待预算，不改变原有上游首事件统计。提前响应后的最终失败以 SSE 错误结束，非重试 HTTP 错误不得混写 JSON；失败尝试的响应头、消息身份与用量不得混入成功输出。 | `backend/internal/service/anthropic_stream_guard.go`、`backend/internal/service/anthropic_stream_safe_retry.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 待发布（默认关闭） |

### 协议与上游兼容

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-PROTO-001` | OpenAI Codex CLI 模拟与客户端策略 | API Key/OAuth 路径支持 Codex CLI 请求模拟、统一出站身份、User-Agent 和版本范围策略、App Server 客户端识别及能力探测一致性；带 `client_version` 的模型请求及 `/backend-api/codex/models` 按实际分组路由生成完整 Codex 清单，合并管理员模型配置与已同步能力元数据，普通 `/models` 仍保持 OpenAI 列表契约。Fast 模型向 Codex 客户端广告 priority 能力，但 OAuth-like 上游返回的 `default` 不覆盖明确请求的 priority。指纹由系统管理的账号随机种子稳定派生，覆盖 HTTP/WS 请求头、`client_metadata` 和默认 `prompt_cache_key`，同一请求头体共用时间戳。有效版本可按 GitHub 最新 release 定期同步，管理员 UA 只贡献非版本指纹。 纳入 Astra、ultrafast、none 推理来源、能力同步和默认关闭的分组固定账号清单。固定清单仅检查组成员、Active、Schedulable 和有效期，读取时不套用限流/临时冷却，不改变实际推理调度；Astra/gpt-6 进入本地精确已知模型规范化，不采用子串宽匹配。 本次上游适配：分组白名单兼容本地精确已知模型规范化；未知 codex 等自定义变体不自动视为基础模型。 | `backend/internal/service/openai_codex_emulation.go`、`backend/internal/service/openai_codex_identity.go`、`backend/internal/service/openai_codex_fingerprint.go`、`backend/internal/service/openai_codex_models_service.go`、`backend/internal/service/openai_codex_version_sync_service.go`、`backend/migrations/225_backfill_codex_fingerprint_seed.sql` | 生效中 |
| `CUST-PROTO-002` | Codex 图片工具策略 | 可完全禁用或按策略处理图片生成工具；覆盖原生 Responses、Chat fallback、namespace 和 WSv2，防止换名、嵌套或重复注入绕过。 2026-09-15 上游适配：原生 Codex Images 接入不改变既有 Responses/Chat/namespace/WS 图片工具策略；两类路径回归断言均保留。 | `backend/internal/service/codex_image_generation_bridge.go`、`backend/internal/service/openai_responses_namespace.go`、`backend/internal/service/setting_service_codex_policy_test.go` | 生效中 |
| `CUST-PROTO-003` | Anthropic Claude Code 上游模拟 | 支持全局开关，将普通下游请求按 Claude Code 客户端规则清洗、补充和转发；账号测试与真实网关使用同一策略。 纳入可选 CLI 版本环境覆盖；强制模拟后的最终出站 UA 与 billing 指纹版本一致，并保留 Unicode 字符索引算法。 2026-09-15 上游适配：Claude 模拟、网关与账号测试继续复用本地清洗策略；本地 system 配置与上游缓存块上限并存。 | `backend/internal/service/gateway_anthropic_mimicry_sanitize.go`、`backend/internal/service/gateway_anthropic_oauth_mimicry.go` | 生效中 |
| `CUST-PROTO-004` | Claude OAuth 提示词与协议清洗 | 可配置 OAuth system prompt/blocks，处理 beta header、thinking block、Bedrock CC 兼容和重试时的协议字段；清理 deferred tool 的 `cache_control`，并把缓存断点保留在最后一个非延迟工具。 2026-09-15 上游适配：保留 stripSystemCacheControl、preserveSystemText、preserveModel 的独立选项和注入关闭时清洗；count_tokens 也执行四块缓存上限。 | `backend/internal/service/gateway_claude_oauth_body.go`、`backend/internal/service/gateway_tool_rewrite.go`、`backend/internal/service/setting_update.go` | 生效中 |
| `CUST-PROTO-005` | API Key 账号请求头覆写 | Anthropic/OpenAI/Kimi/Zhipu/DeepSeek API Key 账号及 Grok API Key/OAuth 账号可以配置请求头覆写；真实转发、模型列表、能力探测、count tokens、Images、Kimi 原生 Responses 和 WS 使用一致的最终请求头。 仅合法 OpenCode 目标在账号覆写后转发调用者会话头；非 OpenCode 目标不泄露该头。 本次上游适配：兼容 MiniMax 平台与 Ollama Anthropic Bearer 鉴权选择，不改变本地最终请求头覆写顺序。 | `backend/internal/service/openai_gateway_forward.go`、`backend/internal/service/upstream_models.go`、`frontend/src/components/account/EditAccountModal.vue` | 生效中 |
| `CUST-PROTO-006` | 请求端点与 compact 信号规范化 | 从原始路径规范化入站端点，区分 Responses compact 和 `/responses/input_tokens`，识别 `compaction_trigger`，继承客户端工具状态；Kimi PayG/Coding 的显式 `responses` 或 `adaptive` 请求使用各自原生 `/v1/responses`，强制 `store=false` 并移除 `previous_response_id`。继续保留本地 HTTP 流转 WSv2、namespace，以及每轮重新取得请求模型和渠道映射、仅在上下文可完整重建时恢复当前轮次换号的决策边界。 2026-09-15 上游适配：接入 OpenCode 三协议端点与单次模型映射，保留 Kimi 原生 Responses/store=false、本地端点归因及 WS 逐轮策略；compact 缺省模型改为 gpt-5.5。 2026-09-18 上游适配：接入 Responses 前导指令合并、中段指令转 user、严格 Chat 目标的 developer 转 system、Lite namespace 及工具媒体处理；Kimi 默认关闭的参数重试与入口请求体保护保留，并补充角色转换和 400 修正同时发生的交叉回归。 | `backend/internal/handler/gateway_handler.go`、`backend/internal/service/openai_gateway_request_body.go`、`backend/internal/service/openai_ws_http_bridge.go`、`backend/internal/service/account_test_service_cn_adaptive.go` | 生效中 |
| `CUST-PROTO-007` | Responses 渠道监控请求格式 | `probe` 和 `quota_probe` 使用结构化 Responses `input` 消息数组，而不是普通字符串；`quota` 只读取额度，不构造 LLM 请求，上游同步不得退回非标准请求体。 | `backend/internal/service/channel_monitor_checker.go`、`frontend/src/components/admin/monitor/MonitorAdvancedRequestConfig.vue` | 兼容覆盖 |
| `CUST-PROTO-008` | Anthropic 采样参数过滤 | 在二开网关配置中按最终上游模型精确匹配或末尾通配符匹配，从 REST Messages 请求体删除已被新模型弃用的 `temperature`、`top_k` 和 `top_p`；覆盖 API Key、OAuth、协议转换、直通和账号测试，默认关闭。 | `backend/internal/service/gateway_anthropic_sampling_filter.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |

### 账号与运维测试

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-ACC-001` | 账号策略默认值与模型白名单 | 创建账号时提供本地 pool mode 默认策略、模型白名单和平台特定控件；API Key 账号落库前可按当前代理、并发、端点和模型映射预览上游模型，创建后再正式同步能力元数据；批量编辑保持这些字段的 API 契约。分组 Fast 与 Free Fast 策略贯穿创建、更新、复制、管理 API 和认证快照，不改变既有账号默认策略。 新上游请求标识头、图片回填开关与本地失败策略、图片尺寸字段组合保存；模型同步成功提示继续显示数量。认证快照升至 v24，同时保留固定账号清单、长上下文与 Free Fast 字段。 本次上游适配：本次将旧 models_list_config 数据迁移为 model_allowlist；获批后同时约束模型展示与实际请求准入，未启用时不限制。认证快照升级到 v24，固定账号清单、长上下文和 Free Fast 字段完整保留。 2026-09-18 上游适配：按批准方案让 DeepSeek 无显式映射时使用上游模型名单，不再任意兜底；显式映射、本地其他 Provider 白名单及默认策略保留。 | `frontend/src/components/account/poolModeDefaults.ts`、`frontend/src/components/account/ModelWhitelistSelector.vue`、`frontend/src/components/account/CreateAccountModal.vue`、`backend/migrations/145_group_models_list_config.sql` | 生效中 |
| `CUST-ACC-002` | 账号数据批量导入与可见性增强 | 前端支持拖拽/批量导入账号数据；账号列表展示账号 ID，并改进 API Key 查看、滚动和查询上下文重置。 | `frontend/src/views/admin/AccountsView.vue`、`frontend/src/components/account/CreateAccountModal.vue` | 生效中 |
| `CUST-ACC-003` | API Key 分组访问约束 | 严格校验 API Key 的专属分组访问，拒绝跨专属分组调度，并支持管理员按用户 API Key 所在分组筛选。 | `backend/internal/service/gateway_channel_restriction_test.go`、`backend/internal/service/admin_user.go`、`frontend/src/views/admin/AccountsView.vue` | 生效中 |
| `CUST-ACC-004` | OpenAI 额度查询与重置 | 账号管理可查询 rate-limit credits，支持人工重置和达到阈值后自动使用重置卡。只有取得幂等执行权的实例会在首次出站调用前捕获并持久化恢复快照；同一额度周期的超时重试和成功结果回放复用该快照，只通过 `rate_limited_at + rate_limit_reset_at` 双时间戳 CAS 清除同一数据库限流代次，运行时还要求实例 ID、generation 和 429 原因匹配。升级前缺少快照的旧失败态或幂等记录使用空快照，只刷新额度，不捕获或清除当前新限流代次；人工重置继续执行广义状态恢复并失效 Token，调度器同步刷新相关快照。 2026-09-18 上游适配：保留 ClearOpenAIRateLimitIfObserved 的 OpenAI OAuth 双时间戳清除 CAS，另保留上游 SetRateLimitedIfUnchanged 的 UpdatedAt 加限流代次写入 CAS；两个方法不合并、不互相替代。 | `backend/internal/service/openai_quota_service.go`、`backend/internal/service/openai_quota_reset_credits.go`、`backend/internal/service/openai_quota_auto_reset.go`、`backend/internal/service/openai_quota_reset_workflow.go`、`backend/internal/service/ratelimit_service.go` | 生效中 |
| `CUST-ACC-005` | 增强账号测试 | 手动和定时测试覆盖 OpenAI Responses/compact/Images、Anthropic 模拟、Gemini 和 Grok 各媒体模式。普通文本默认测试模型独立于业务及计费：OpenAI 为 `gpt-6-astra`，Anthropic/Antigravity 为 `claude-opus-5`，Gemini 为 `gemini-3.8-flash`，Grok 为 `grok-4.6`；Bedrock/旧 Google One 保留既有受支持默认值，显式任务模型不改写。国产供应商按实际协议测试，从配置中优先选择具体文本请求模型，只有通配符时取具体目标，去重并按字典序稳定选择；不重复映射，无候选时报配置错误且不出站。手动预选同步，自定义选项仍保留；首 Token 恢复事故模型优先于上述默认值。保留采样过滤、语义错误、工具调用、媒体与提示词校验，以及 Kimi 原生 Responses/store=false 契约。 本次上游适配：纳入上游实时模型发现、显示名称和 MiniMax；本地默认模型、显式任务模型及恢复事故模型优先级不变，前后端均补充 MiniMax 确定性选择回归。 2026-09-15 上游适配：OpenCode 有映射时按本地确定性规则选择具体文本模型，无映射时使用 glm-5.3；测试副本固定最终模型以免二次映射，Responses/Anthropic 沿用自定义提示词，已有平台默认值不变。 | `backend/internal/service/account_test_models.go`、`backend/internal/service/account_test_service.go`、`backend/internal/service/account_test_service_cn_adaptive.go`、`frontend/src/utils/accountTestModels.ts`、`frontend/src/components/admin/account/AccountTestModal.vue` | 生效中 |
| `CUST-ACC-006` | 本地数据兼容迁移 | 维护注册来源默认授权、平台 quota、订阅到期通知和账号扩展字段的迁移兼容，并与上游原生 compaction、渠道缓存写价、分组 Fast/Free Fast 及 reasoning 超限策略迁移共存，避免本地历史数据在上游同步后丢失约束。 新增上游四份请求 ID、并发索引、max 倍率和固定账号清单迁移；与本地同数字前缀迁移按完整文件名共存，不重命名或改写旧迁移。上次同步时 SQL 真实执行未验证。 本次上游适配：本次新增白名单收敛与代理字段等上游迁移，与本地同数字前缀迁移按完整文件名共存；已在隔离 PostgreSQL 中验证全部迁移及白名单旧数据修复，不涉及生产数据。 2026-09-15 上游适配：新增两份 238 迁移按完整文件名共存；只删除日/周/月均 NULL 的 quota 行，零限额及任一有效限额保留。隔离旧库升级与重复执行均验证通过，本地 237 用户定制/授信数据不变。 | `backend/migrations/142_extend_user_provider_default_grants_check.sql`、`backend/migrations/143_subscription_expiry_notify_enabled.sql`、`backend/migrations/144_user_platform_quotas.sql`、`backend/migrations/231_add_usage_log_native_compaction_v2.sql`、`backend/migrations/232_group_force_openai_fast.sql`、`backend/migrations/233_group_free_openai_fast.sql` | 兼容覆盖 |
| `CUST-ACC-007` | 复制账号测试计划 | 账号、分组优先级、源测试计划与调度通知在同一事务中复制；副本账号和全部计划初始停用，不继承测试结果、执行时间、故障状态或失败次数。唯一系统计划优先沿用源系统计划，没有则使用最早创建且开启自动恢复的普通计划，否则使用默认模板；故障时自动启用并按二开退避恢复，不解除人工暂停。复制 API 保持幂等和凭据脱敏，不追溯改写历史副本。 | `backend/internal/service/admin_account.go`、`backend/internal/repository/account_repo.go`、`backend/internal/repository/account_duplicate_plans.go` | 生效中 |

### 计费、定价与可观测性

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-BILL-001` | 按用户请求模型计费 | 普通分组严格按用户请求模型计费，不使用渠道映射名、最终上游模型或响应模型回退；Composite 分组仅在公开别名存在显式渠道价时按别名计费，否则按实际路由的具体模型计费。Chat、Responses、Images、Messages 与 WS 入口统一记录请求模型、映射链和上游响应模型；上游响应模型只用于诊断和账号成本，不反向改变用户计费来源。Free Fast 的用户 `ActualCost` 按 Standard 计费，账号 `TotalCost` 仍按 priority 上游成本。 映射模型只影响调度和转发，旧 upstream/channel_mapped/response_model 来源继续归一为 requested；负载、粘性、路由均不能绕过原请求模型价格清单，未知 Astra 自定义别名仍须显式价格。 本次上游适配：DeepSeek 历史 PricingAt 的账号成本使用最终上游模型；请求别名仍须满足本地显式价格准入，不回退映射型号。 2026-09-15 上游适配：OpenCode 与图片新分支仍保留请求模型准入、映射链和响应模型诊断；新增 DeepSeek 切换价不替代自定义渠道/分组价。 | `backend/internal/handler/requested_model_pricing.go`、`backend/internal/service/requested_model_pricing.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/migrations/175_force_requested_billing_model_source.sql` | 生效中 |
| `CUST-BILL-002` | 多层定价与严格缺价处理 | 所有 token 入口统一使用同一计费选择路径，定价优先级固定为“分组逐模型定价 → 渠道定价 → 动态远端定价 → 内置/兜底价格”；支持渠道默认定价、Standard/Fast/Flex 服务层级倍率、时段倍率，以及 token 区间输入/输出/缓存读写绝对价或倍率，认证快照完整保留渠道定价。服务层级按上游实际结果只允许降档计费，不得把请求档位升级为更高倍率；Codex OAuth-like 返回 `default` 不具备上调权威性，明确 `flex` 仍可降档。支持 DeepSeek/GLM/Kimi/MiniMax/豆包/Grok 等兜底定价及 thinking、图片、视频、搜索/音频计价补充；DeepSeek 默认价卡按官方工作日峰谷时段计费并纠正远端旧价，但分组/渠道自定义价不被覆盖。xAI/LiteLLM above-200K 绝对价仅对 Grok 转换为内部倍率，完整远端价卡保持优先；已知残缺 Grok 价卡才补内置阶梯，4.5 的已知错误缓存价只按精确签名修正。目录阶梯与本地边际规则按显式优先级互斥，避免重复升档；非简单模式下选定计费模型缺价必须返回错误，不得静默记为零费用；Composite 的具体模型回退仅用于公开别名未配置渠道价的场景，不改变普通分组边界。 接入 fallback/override 文件内容哈希与一致快照热重载；新增 Astra 精确兜底；Fable 5.1 的 max 不再默认乘3，输入、输出及缓存读写按实际 Token 用量与基础价计费，用户倍率照常应用。仅显式配置的渠道 max 倍率按最终转发 reasoning effort 生效；账号成本和公开价格展示不得隐式注入三倍规则。后续上游同步不得恢复该默认加价，不覆盖本地缺价拒绝、渠道优先及其他模型特例。 本次上游适配：账号成本复用统一峰谷/缓存定价管线，继续保留 Fable max 无默认三倍及无渠道按上游型号成本回退。 2026-09-15 上游适配：接入图片缓存输入细分及 DeepSeek Flash 新默认价；Pro 是否按 Flash 默认价结算按 PricingAt 与北京时间 2026-09-14 12:00 比较。严格缺价、定价优先级、Fable 无默认三倍及显式 max 倍率保留。 | `backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/pricing_service.go`、`backend/internal/service/requested_model_pricing.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/migrations/147_channel_default_pricing.sql`、`backend/migrations/221_group_model_pricing.sql`、`backend/migrations/228_channel_pricing_multipliers.sql` | 生效中 |
| `CUST-BILL-003` | 长上下文计费兼容策略 | OpenAI 账号开关与真实分组配置共同约束长上下文阶梯；只有实际上下文命中渠道区间时才禁用内置倍率，区间未命中回退基础价时仍可应用。数据库中的分组开关默认开启且显式关闭有效，API Key 认证缓存快照必须完整保留该开关；未携带真实分组配置的旧调用对象继续按兼容默认开启。Gemini 原生 `/v1beta` 在超过 200K 后仅对超阈值的 input/cache read 应用 2 倍边际价，output/cache write 保持基础价，模型广场标记为 `marginal`；显式边际规则优先于目录整单阶梯。GPT-5.6 静态兜底在严格超过 272K 时输入 2 倍、输出 1.5 倍；Grok 只服从分组开关，不被 OpenAI 账号开关否决，并在严格超过 200K 时对普通输入、缓存输入与输出使用同一阶梯；GPT-5.4 Pro 无目录数据时只使用基础静态价，有目录阶梯时按目录计费。全部所选分组开启时前端隐藏冗余账号开关，否则保留。 新增 Astra 272K 严格大于阈值的输入2倍、输出1.5倍阶梯，继续服从本地分组长上下文开关并保留 GPT-5.6/Gemini/Grok 边界。 本次上游适配：认证快照 v24 继续完整投影长上下文和分组模型定价，原阈值与默认值不变。 2026-09-15 上游适配：合入图片缓存价时继续保留 Grok 严格大于 200K 的本地边界、分组长上下文开关及 Gemini/GPT/Astra 原规则。 | `backend/internal/service/billing_service.go`、`backend/internal/service/billing_token_cost_request.go`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/api_key_auth_cache_impl.go`、`backend/ent/schema/group.go`、`backend/migrations/175_default_openai_long_context_billing.sql`、`backend/migrations/221_group_model_pricing.sql` | 兼容覆盖 |
| `CUST-BILL-004` | Image 分组成功率 | 统计单次和批量图片请求的分组请求数/失败数，支持代次式原子清零、用户展示开关，并排除 keepalive 字节对结果判断的干扰；用户端 Channel Monitor V1/V2 均可展示该统计。 本次上游适配：根路径别名和 Codex 直连新白名单链保留图片分组结果统计中间件，不漏记 Chat/Responses。 | `backend/internal/service/image_group_success_rate.go`、`backend/internal/repository/image_group_success_rate_repo.go`、`frontend/src/views/user/ChannelStatusV1View.vue`、`frontend/src/views/user/ChannelStatusV2View.vue`、`backend/migrations/177_image_group_success_rates.sql` | 生效中 |
| `CUST-BILL-005` | 用量与费用明细增强 | 展示缓存 Token、请求模型、服务层级、余额调整和图片/视频费用信息；推理强度同时记录请求值与策略映射后值，用户只看请求值，管理员可查看两者；错误请求和实际上游端点记录使用本地归因规则。Free Fast 请求的 usage `service_tier` 保持 priority 以反映上游服务，用户实际费用按 Standard、账号成本按 priority 分别记录。 增加默认隐藏的管理员上游请求 ID 列及按账号配置的响应头采集，未配置不采集。配置写入和读取兜底均拒绝 Authorization、Proxy-Authorization、Cookie、Set-Cookie、X-API-Key、Api-Key、X-Auth-Token、X-Access-Token；WS 无响应头时不伪造值。ultrafast 接入不改变 Free Fast 双成本。 本次上游适配：使用记录筛选加载全部分页 API Key，保留本地列设置与筛选组件。共用用量表格新增首字后平均输出速率，具体口径及不适用边界见下方补充规则；用户充值历史和管理员余额历史支持独立的临时授信记录。 2026-09-15 上游适配：原生图片记录交付/断连、部分用量、图片缓存与真实上游端点；Free Fast 双成本和授信明细保留。 | `backend/internal/service/gateway_usage_billing.go`、`frontend/src/components/admin/usage/UsageTable.vue`、`frontend/src/utils/usageOutputRate.ts`、`frontend/src/components/admin/user/UserBalanceHistoryModal.vue` | 生效中 |
| `CUST-BILL-006` | 高并发余额扣费整流 | 单实例内对同一用户的余额扣费事务串行排队，不同用户仍可并行；usage worker 使用 `SubmitKeyed(userID)` 在执行前按用户聚合，同一热用户只占一个 worker，任务真正开始执行时再创建超时 context，默认及生产继续保持 5 秒 usage task 超时。WS 每轮独立结算 usage，余额事务 gate 继续在 `BeginTx` 前提供最终进程内串行。 | `backend/internal/repository/usage_billing_repo.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/usage_record_worker_pool.go` | 生效中 |

| `CUST-BILL-007` | 一次性自动授信 | 默认关闭，按用户配置阈值（默认1000）和固定正向金额，单位与账户余额一致、保留8位精度。独立后台启动即查，此后每60秒分批检查正常用户；仅 `users.balance` 严格小于阈值且本代次未使用时入账。锁定用户和配置，在单事务中写余额、授信流水、`temporary_credit` 充值历史及鉴权缓存失效事件，以用户＋代次唯一约束防止多实例和重试重复加款；余额缓存失败持久重试并延迟二次删除。只通过校验当前代次的独立确认操作恢复机会，普通保存、开关切换、充值和重置链接均不恢复；不自动扣回，不计真实充值累计，不产生返佣和充值赠送。网关扣费路径、按用户串行和5秒用量任务超时不变。 | `backend/internal/service/temporary_credit_worker.go`、`backend/internal/repository/user_customization_repo.go`、`backend/migrations/237_user_customizations.sql` | 生效中 |

`CUST-BILL-005` 使用明细补充：管理员和用户共用 `UsageTable.vue` 在延时列总耗时下显示首字后平均估算输出速率（两位小数、`tk/s`），口径为 `output_tokens × 1000 ÷ (duration_ms − first_token_ms)`。仅有有效时间数据的 HTTP 流式文本和 WS 文本逐轮适用；非流式、媒体、Live、缺失时间、非正输出或非正时间差显示 `-`。不新增用量数据库字段，不改运维看板及导出口径；入口为 `frontend/src/utils/usageOutputRate.ts` 和共用 `UsageTable.vue`。

### 渠道监控与可观测性

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-OBS-001` | 流式渠道监控与随机抖动 | `probe`/`quota_probe` 渠道监控和模板可以选择流式检测，`quota` 模式独立读取额度；探针总超时与响应头超时统一为 600 秒，Gemini 流式与非流式响应均遍历首个候选的全部文本分段并跳过思考分段；检测间隔支持正负随机抖动，避免大量渠道同一时刻探测，并保留 provider/endpoint/请求体的本地扩展。渠道分组按用户访问范围过滤，OpenAI 可见 TTFT 的统计口径由管理员设置统一控制。 2026-09-18 上游适配：吸收监控自动刷新间隔修复；Provider 测试继续按 PROVIDERS.length 验证并保留 MiniMax，避免固定数量断言遗漏本地供应商。 | `backend/internal/service/channel_monitor_checker.go`、`backend/internal/service/channel_monitor_const.go`、`backend/ent/schema/channel_monitor.go`、`backend/migrations/141_channel_monitor_stream_enabled.sql`、`backend/migrations/226_channel_monitor_quota_mode.sql` | 生效中 |
| `CUST-OBS-002` | 独立上游站点同步监控 | 在二开管理页统一维护 Sub2API/New API 上游站点，定时同步余额、分组倍率与实际消耗，保留用量及倍率历史；Sub2API 同步 Token 与费用，New API 仅通过标准统计接口同步站点及分组费用，不再分页扫描日志，Token 指标明确标记为不可用并隐藏，既有 Token 数据保留但不再更新；支持按分组类型筛选、按余额/有效今日 Token 排序、拖拽调整站点顺序，并按平台优先级和倍率整理账号下方分组；上游分组可绑定本地账号并按最后一次有效倍率自动维护全局优先级，不可用分组仍参与排序，从未取得有效倍率的账号保留原优先级；新增 Sub2API 时会探测 Turnstile，自动改用令牌认证并支持从完整登录响应导入 Access/Refresh Token；令牌模式可在加密凭证中保存登录会话 User-Agent，并在验证、刷新与同步请求中保持一致；目标站点返回 `SESSION_BINDING_MISMATCH` 时会以 Chrome TLS/HTTP2 指纹重试，成功后将该传输模式随凭证加密保存，以兼容绑定出口 IP、UA 与 TLS/JA4 指纹的浏览器会话；Sub2API JWT Access Token 会在到期前 15 分钟主动轮换，非 JWT 令牌保持原有按认证失败刷新行为；New API 同时兼容旧版 Cookie 会话和 rc.22 起的短期 Bearer Access Token/刷新 Cookie，认证失败时优先刷新已有会话，刷新接口明确拒绝或旧版不支持时才回退密码登录，冲突及临时错误不再创建新登录会话；New API 还可直接使用个人中心生成的个人访问令牌，令牌模式不创建登录会话、不使用 Cookie/刷新令牌且认证失败时不会回退密码登录，管理页无需导入会话 User-Agent，后端自动使用固定浏览器 User-Agent 与 Chrome TLS/HTTP2 指纹兼容 Cloudflare，首次验证会从 `/api/user/self` 自动取得远端用户 ID；同步时优先复用加密保存的 Token/Cookie，仅在进入主动刷新窗口、凭证被拒绝或需要重新登录时更新认证。 本次上游适配：上游新增分组行锁与本地账号差量分组绑定、倍率绑定清理兼容；未变关系优先级和剩余账号重排仍受回归保护。 | `backend/internal/service/upstream_service.go`、`backend/internal/service/upstream_provider_http.go`、`backend/internal/service/upstream_provider_newapi.go`、`frontend/src/components/admin/upstream/UpstreamManagementPanel.vue`、`backend/migrations/178_upstream_management.sql`、`backend/migrations/181_upstream_management_order_and_platform.sql` | 生效中 |

### 产品、支付与增长

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-PROD-001` | 每日签到转盘 | 支持余额、并发、订阅和空奖品，按万分比配置概率；支持未付费用户奖励衰减、LinuxDo 豁免、记录查询和防重复签到。默认关闭。 | `backend/internal/service/daily_checkin_service.go`、`backend/migrations/136_daily_checkins.sql`、`backend/migrations/146_daily_checkin_spin_rewards.sql`、`frontend/src/views/user/DailyCheckinView.vue` | 生效中 |
| `CUST-PROD-002` | 本地模型市场（`/models`） | 按指定分组公开模型、协议格式、价格和介绍，管理端可配置开关、介绍和分组范围。默认启用，但只返回公开白名单字段；与上游默认关闭的 `/model-plaza` 及面向 Codex 客户端的分组模型清单独立共存，二者均不替代本地入口。 本次上游适配：公开模型列表使用同组 model_allowlist，支持精确/通配筛选，启用后无匹配或遗留空配置不回填禁止模型；原公开分组范围与默认开关保留。 | `backend/internal/handler/model_marketplace_handler.go`、`backend/internal/handler/openai_codex_models_handler.go`、`frontend/src/views/ModelMarketplaceView.vue` | 生效中 |
| `CUST-PROD-003` | 公开模型定价 | 提供独立定价 API/页面，展示充值倍率和模型费用；导航入口可以隐藏，但直达页面和 API 仍受公开设置控制。 | `backend/internal/handler/payment_handler.go`、`frontend/src/views/ModelPricingView.vue` | 生效中 |
| `CUST-PROD-004` | 批量图片任务 | 提供任务提交、队列处理、列表、明细、输出、下载、取消和清理；限制到允许的 Gemini 分组，包含余额预占、失败恢复和有界结算重试。 本次上游适配：批量图片请求准入和可选模型列表同步受分组白名单约束，历史任务处理及预占/结算规则不改。 | `backend/internal/service/batch_image.go`、`backend/internal/service/batch_image_worker.go`、`backend/internal/server/routes/gateway.go` | 生效中 |
| `CUST-PROD-005` | 兑换码多次使用 | 兑换码支持最大使用次数，并以用户维度记录使用明细，防止同一用户重复使用同一码。 2026-09-18 上游适配：新增用户兑换历史分页入口复用本地 redeem_code_usages 查询，保留多用户使用明细、事务和扫描逻辑，不退回只看最后 used_by 的历史；新增跨用户分页隔离回归。 | `backend/internal/repository/redeem_code_repo.go`、`backend/migrations/140_redeem_code_usage_limits.sql` | 生效中 |
| `CUST-PROD-006` | 支付和充值增强 | 支持余额充值赠送阶梯、手续费/倍率、赠送快照、自定义 EasyPay 支付方式、订阅 USD/CNY 换算预览和管理员删除订单；可配置专属倍率用户不参与余额充值赠送。EasyPay 会将 `mapi.php` 返回的站点根相对支付/二维码地址补全为可访问绝对地址；待支付订单在有效期内周期主动查单，通知丢失时复用幂等履约链路自动补发余额，并记录补查来源审计。 通用待支付补查扩展支付宝，同时排除由 EasyPay 独立路径处理的支付宝订单；EasyPay 微信继续走通用补查。独立路径保持30秒门槛、每轮最多10条/10秒预算、幂等履约及来源审计。 2026-09-18 上游适配：金额输入非法文本回退、支付配置并发等待及退款警告修复与本地充值赠送共存；赠送角标和上游输入校验测试同时保留。 | `backend/internal/service/payment_amounts.go`、`backend/internal/service/payment_order_lifecycle.go`、`backend/internal/payment/provider/easypay.go`、`backend/migrations/149_payment_balance_bonus_rules.sql` | 生效中 |
| `CUST-PROD-007` | 站内无限画布 | 基于 `basketikun/infinite-canvas@b66936d891b82c2b51c1ed05e1a6eae3e31d4ca3` 的 React 画布裁剪集成，保留项目、素材、提示词、文本/图片/视频节点、连线、缩放平移和受控画布助手；移除上游 Key 配置、WebDAV、本地 Agent/MCP、远程插件与脚本、推广及外链入口。Vue 通过同源沙箱 iframe 嵌入 `/canvas-app/`，默认开启的功能开关、JWT 父页面消息桥和按用户隔离的浏览器存储共同约束访问；运行时按用户可用分组复用普通 Key 或懒创建唯一托管 Key，切换分组后的重试只使用当前分组真实模型并展示上游错误，图片默认生成 1 张。OpenAI GPT Image 使用官方输出字段，Grok 图片兼容层将 OpenAI 尺寸转换为 xAI 宽高比和 1K/2K 分辨率，多图编辑只发送互斥的 `images`，不支持的遮罩明确拒绝；Gemini/Antigravity 图片及图片编辑使用 `streamGenerateContent?alt=sse`、`generationConfig.imageConfig` 并从流中提取图片，隐藏只能调用 `predict` 的 Imagen 模型。OpenAI 视频使用 `/v1/videos` multipart、单数 `input_reference` 和官方时长枚举；Grok 视频使用 `/v1/videos/generations` JSON，兼容 `request_id`、`done`、`video.url` 及旧 OpenAI 字段转换。生成结果在浏览器本地安全落盘，视频优先从站内内容接口下载；所有媒体请求继续经过站内余额、订阅、分组计费和并发控制，Gemini 首版不提供视频生成。 | `canvas/`、`frontend/src/views/user/CanvasView.vue`、`backend/internal/service/api_key_service.go`、`backend/internal/service/grok_media.go`、`backend/migrations/222_infinite_canvas_api_keys.sql`、`canvas/THIRD_PARTY_NOTICES.md` CI 合入前将间接 YAML 解析依赖 js-yaml 4.x 的漏洞范围锁定到 4.3.2 补丁版，不改变画布默认值、接口和数据格式；不增加安全扫描豁免。 | 生效中 |

| `CUST-PROD-008` | 用户定制公开余额查询 | 管理员按用户生成唯一长期查询链接 `/balance-query#随机码`，默认不生成；32字节安全随机数，仅存摘要及加密原文，可复制、停用、启用、重置，重置立即废止旧链接，独立于授信开关。页面不恢复登录态，兼容后台模式，仅公开用户名（空时用ID）、ID、余额、安全充值历史和全局有序纯文本充值方式（最多20项、键50字、值500字）；记录默认20条、最多50条分页，仅已到账余额兑换/充值、管理员正向加款和临时授信，普通来源固定备注，授信显示实际入账金额。接口不接受用户ID或URL令牌，仅专用头传码；IP每分钟30次、有效链接60次，429附Retry-After，Redis故障503。页面和接口no-store/noindex/no-referrer，无登录凭据、自动轮询及账户写入，停用/删除用户与失效链接统一不可用。 | `backend/internal/handler/balance_query_handler.go`、`backend/internal/service/user_customization.go`、`frontend/src/views/public/BalanceQueryView.vue`、`frontend/src/components/admin/user/UserCustomizationsPanel.vue` | 生效中 |

### 风控与内容审核

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-RISK-001` | 本地人工审核对话 | 内容审核可进入本地人工审计，持久化请求/响应记录；管理端支持列表、详情、下载和删除。统一安全审计协调器接入上游 prompt audit，协调器未配置时继续使用本地内容审核，审计服务过载时按配置执行回退。 | `backend/internal/securityaudit/coordinator.go`、`backend/internal/service/content_moderation_local_audit.go`、`backend/internal/handler/security_audit_helper.go` | 生效中 |
| `CUST-RISK-002` | Cyber 会话阻断 | `cyber_policy` 命中可沿网关、审计和计费链路透传，并按配置对会话做 TTL 阻断；Responses `response.failed` 先解析 usage 再标记会话，WSv2 透传会在 `AfterTurn` 前设置标记，避免错误终态漏记 Token，用量记录允许 `cyber_blocked` 类型。事件只有在全局风险控制、内容审核、非 `off` 模式以及 group/model scope 均允许时才记录；运行时快照读取失败时安全跳过事后副作用，不扩大阻断范围。 WS 各路径标记与同一逻辑轮次的失败转移链去重，两个原子状态分别保存已阻断与失败转移待判定；不扩大风控 scope，保留失败终态的部分 Token。 2026-09-15 上游适配：WS 作用域与抢占更新后仍逐轮执行 Cyber 策略、失败去重与部分 usage 记录，不扩大作用域。 | `backend/internal/service/content_moderation.go`、`backend/internal/service/openai_ws_v2_passthrough_adapter.go`、`backend/internal/handler/security_audit_helper.go`、`backend/migrations/174_allow_cyber_blocked_usage_request_type.sql` | 生效中 |
| `CUST-RISK-003` | 错误请求详情可见性 | 管理端运维详情支持从账号、用户和请求上下文继续导航；可按设置允许用户查看自己的错误请求详情。 | `frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue`、`backend/internal/service/setting_user_error_view_test.go` | 生效中 |

### 前端与管理体验

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-UI-001` | 独立二开管理页 | 将上游同步监控、模型广场、每日签到、网关运行策略和用户定制集中在 `/admin/custom-features`，与上游通用设置隔离；页面使用宽屏管理台布局展示高密度数据；上游管理始终排在第一个并默认选中，其余页签保持相对顺序。用户定制独立页签支持搜索用户、余额展示、全局充值方式增删排序、专属查询链接管理和授信配置；重置链接与恢复授信均需确认，恢复窗口显示用户、阈值和单次金额，沿用管理员鉴权、写幂等及敏感读取审计。 | `frontend/src/views/admin/CustomFeaturesView.vue`、`frontend/src/components/admin/user/UserCustomizationsPanel.vue`、`backend/internal/server/routes/admin_custom_features.go` | 生效中 |
| `CUST-UI-002` | 统一功能开关注册表 | 公共设置驱动的入口统一声明 opt-in/opt-out 语义，避免 SSR 注入缺字段导致刷新闪烁或错误隐藏；渠道 quota 展示和插件管理均为 opt-in，字段缺失或 false 时不暴露对应入口，Canvas、每日签到和本地模型市场继续保持各自既有默认语义。 本次上游适配：新增上游渠道监控排名开关时保留 Canvas、签到、本地市场、quota 和插件的原开关语义。 2026-09-15 上游适配：订阅为 opt-out，字段缺失时启用；关闭订阅的路由/侧栏处理与 Canvas、签到、本地模型市场及公开余额路由共存。 | `frontend/src/utils/featureFlags.ts`、`frontend/src/components/layout/AppSidebar.vue`、`backend/internal/service/setting_public.go` | 生效中 |
| `CUST-UI-003` | 机甲冷蓝主题与浅色默认 | 保留自定义认证背景、扫描线、计数、倾斜等视觉组件，默认使用浅色主题并提供暗色兼容。 2026-09-15 上游适配：新配额卡继续使用本地 CountUp、PulseDot 与动效组件；桌面和 390px 移动端以隔离夹具检查，无横向溢出。 | `frontend/src/components/common/BackgroundFX.vue`、`frontend/src/composables/useTilt.ts`、`frontend/src/stores/app.ts` | 生效中 |
| `CUST-UI-004` | 表格和导航稳定性 | 保留账号列表禁用虚拟化、查询后滚动重置、分页大小持久化、侧边栏滚动位置，以及日期和列设置下拉浮层不被表格 sticky 表头覆盖；用量窗口统一展示，隐藏的缓存 tooltip 不再撑出横向滚动，账号到期时间按本地时区严格解析。 账号列表接入 lite DTO 和编辑时详情回取；本地禁用虚拟化、分页/滚动和列设置 Teleport 浮层保留。请求 ID/上游 ID 默认隐藏，测试卸载浮层实例避免跨用例串扰。 本次上游适配：账号操作菜单、用量密钥分页和清单编辑即时更新纳入，保留 DataTable 非虚拟化及本地浮层与列配置。 2026-09-15 上游适配：账号创建复用本地 upstreamModelSyncPreviewRequest；OpenCode adaptive 协议进入同一预览路径，模型白名单测试对齐本地接口并保留原断言。 2026-09-18 上游适配：API Key 批量编辑与本地列设置入口同时保留，不以新增批量弹窗替换列配置。 | `frontend/src/components/common/DataTable.vue`、`frontend/src/components/common/ColumnSettingsDropdown.vue`、`frontend/src/components/layout/TablePageLayout.vue`、`frontend/src/components/layout/AppSidebar.vue` | 生效中 |
| `CUST-UI-005` | 认证页加载与存储容错 | 注册页等待公共设置后再开放输入和提交，登录协议、Captcha 与 OAuth 开关使用同一设置快照；local/session storage 不可用时安全降级，刷新令牌并发时防止旧会话覆盖换号后的新会话；仅从 auth store 恢复 pending OAuth 时不会复用普通邮箱注册残留的邀请码。 本次上游适配：注册入口可见性和验证码加载状态纳入；本地认证存储容错保留，SDK 夹具显式按 host:port 绕过代理。 2026-09-18 上游适配：确认密码和缓存公开设置接入现有注册加载保护，确认密码输入及显示切换沿用 registrationInputDisabled；加载容错与缓存设置两组 mock 同时保留。 | `frontend/src/views/auth/RegisterView.vue`、`frontend/src/views/auth/EmailVerifyView.vue`、`frontend/src/stores/auth.ts`、`frontend/src/api/tokenRefresh.ts`、`frontend/src/utils/browserStorage.ts` | 生效中 |

### CI、发布与生产约束

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-OPS-001` | 标签发布与生产自动部署 | `v*` 标签构建发布产物、Docker Hub/GHCR 镜像并自动 SSH 部署；完整多架构发布允许最长 120 分钟，包含连接诊断、重试、健康检查和版本文件回写。 2026-09-14 生产部署目标迁至216.152.153.86，后续按用户要求将SSH改为22748，工作流默认值、强制校验与GitHub DEPLOY_PORT同步修改，独立部署密钥经重启后验证；旧机禁止再次部署业务。2026-09-16新增用户授权的一小时观察及超时强制退役策略，生产job为120分钟，固定实例身份、Redis归属清理、游标恢复和强制结果独立审计；已手动回收green，后续tag须包含该工作流变更，不宣称强制请求无损。 | `.github/workflows/release.yml`、`deploy/blue-green/deploy.py`、`deploy/remote-deploy.sh` | 运维约束 |
| `CUST-OPS-002` | 保留生产 Compose | 自动部署默认不上传仓库 Compose，只更新远端 `.env` 镜像标签并使用活动 Compose，防止通用命名卷配置覆盖生产文件。 本次上游适配：本次仅本地更新两份 Compose 的图片主控模型环境变量，不上传文件或改变生产活动配置。 2026-09-15 上游适配：本次仅在本地同步两份生产 Compose 的 compact 默认值为 gpt-5.5；字节一致约束保留，不上传或变更生产配置。 | `.github/workflows/release.yml`、`deploy/remote-deploy.sh` | 运维约束 |
| `CUST-OPS-003` | 生产数据与网络拓扑 | 生产实例必须保持 bind mount 数据目录、仅回环地址暴露、两个活动 Compose 一致，以及业务要求的 HTTP upstream 安全开关；Compose 接入上游健康检查变化并透传实例级数据保留参数，具体保留天数、实例专用路径和连接参数只保存在不跟踪的 `.env` 与本地运维说明中。 2026-09-05 发布门禁在变更版本/重建前强制校验两份Compose字节一致、应用/PostgreSQL/Redis的精确bind mount和旧实例健康；Release配置只允许服务器1及指定用户、端口、目录和活动文件。不满足时失败退出，不修改.env或重建容器。 本次上游适配：两份 Compose 字节一致，增加 SUB2API_IMAGES_MAIN_MODEL（默认 gpt-5.6-luna）；bind mount、回环监听和 HTTP upstream 约束不变。 2026-09-14 已完成 PostgreSQL/Redis 单写迁移、商店 SQLite 一致性复制及10条 A 记录切换；旧机只保留固定新 IP 的 Nginx 转发，停用数据库、Docker 与业务定时任务。新端写入后禁止直接启动旧库回退。后续经授权完成系统包和内核更新，SSH仅22748且保留密码登录，Fail2ban封禁1小时；BBR/CAKE保留用户配置。脱敏主机配置归档于deploy/operations/production-20260914。 2026-09-16 经授权平滑调整Nginx为worker_connections=16384、multi_accept=off，修复当前epoll接入轮转偏斜，保留旧worker自然排空与应用资源上限；实测证据见docs/operations/2026-09-16-nginx-worker-balance.md。 | `deploy/docker-compose.yml`、`deploy/docker-compose.sub2api.yml`、`deploy/remote-deploy.sh`、`.github/workflows/release.yml` | 运维约束 |
| `CUST-OPS-004` | 本地 CI 与安全门禁 | Push/PR 运行 Go 1.27.0 单元和集成测试、Node 20 前端及 Canvas 测试、golangci-lint 2.13、部署脚本检查及依赖安全扫描；gosec G703/G704 不再全局排除，生产代码逐点说明，测试文件按明确路径豁免。发布前以远端 CI 为最终门禁；本地工具版本低于目标 Go 版本时必须如实标记静态检查未验证。 新增12项隔离部署脚本回归并接入macOS shell job，覆盖禁止清单、不一致清单、命名卷/错误目录、重建前健康失败及重建后健康失败；无检查工具不能伪报成功。 本次上游适配：新增上游 i18n 完整性门禁与本地 featureFlags 门禁并存。ZIP 安装前关闭与本地幂等清理共存；PG 备份迁移锁以本地 mock 验证，未执行真实备份/恢复。用户定制管理、公开余额查询客户端与页面、使用记录表格及输出速率的5个前端测试文件纳入固定CI关键测试清单。Claude 透传流安全重试与精确空闲计时纳入 Linux 定向竞态检查，配置 API、流诊断与错误详情组件纳入关键前端测试；PR #22 安全扫描发现的 gRPC 漏洞通过最小修复版本升级解决，不增加豁免。 2026-09-18 上游适配：新增 API Key 批量编辑关键测试清单与既有门禁并存；保留本地 gRPC 1.83.2 安全版本、validator 直接依赖及 Wire/Ent 工具依赖，不增加安全豁免。 | `Makefile`、`.github/workflows/backend-ci.yml`、`.github/workflows/security-scan.yml`、`backend/.golangci.yml` | 运维约束 |
| `CUST-OPS-005` | 生产资源保护与调优 | Compose 透传 Go 内存、用量 worker、数据库连接与并行查询参数，并为应用、PostgreSQL 和 Redis 配置有界 Docker 日志轮转；上游健康检查变化不得移除这些变量入口。实例参数保存在未跟踪的 `.env`、`deploy/data/config.yaml` 与本地运维说明中。服务器1升级至 12 vCPU、约 32 GiB 内存后，仅按实例覆盖 Go 并行度、Go 软内存限制、PostgreSQL 共享缓存和有效缓存估值；保持数据库连接池、用量 worker/队列、按用户串行扣费、5 秒用量任务超时及其他性能参数不变，具体实施与回滚见 `docs/operations/2026-09-07-resource-tuning.md`。服务器1在持久化配置中将通用 `gateway.response_header_timeout` 显式设为 3600 秒，覆盖后端及通用示例的 600 秒默认值；应用升级或容器重建不得丢失此覆盖。该参数只限制等待上游响应头，不是请求总时长，也不是分组专属；OpenAI/Codex、Grok 的独立响应头超时、流式数据间隔超时及 5 秒用量任务超时保持不变。 2026-09-14迁移阶段先保持原参数，后续经用户明确授权按256逻辑CPU设置Go并行度和512连接池，Go软内存上限为总内存80%，PostgreSQL共享缓存25%、有效缓存估值50%、最大连接600；新增POSTGRES_SHM_SIZE入口且默认2g。保留原用量worker、串行扣费与超时，修正Nginx master文件上限并持久化Redis overcommit=1。已明确记录80%+25%预算叠加的OOM风险，不宣称峰值容量安全。2026-09-15按用户要求将 `deploy/config.example.yaml` 的 `gateway.stream_data_interval_timeout` 示例值从500秒调整为300秒；只影响采用该示例配置的部署，后端缺省仍180秒，不修改生产实例配置、响应头超时或5秒用量任务超时。 2026-09-15 上游适配：上游 WS 连接池系数默认 5 与容量计算合入；保留生产资源入口、日志边界、用户串行扣费、5 秒 usage task、3600 秒持久化响应头超时约束及本地 300 秒流间隔示例。 2026-09-15 后续本地修复：结合近三个完整北京时间自然日的Claude通道统计，将此前300秒流间隔示例统一为后端既有默认180秒，并添加一致性测试；只修正透传计时精度，不进一步缩短阈值或变更生产实例配置。 | `deploy/docker-compose.yml`、`deploy/docker-compose.sub2api.yml`、`deploy/.env.example`、`deploy/config.example.yaml`、`backend/internal/config/config.go` | 运维约束 |

## 变更记录

记录按时间倒序追加。功能清单描述“现在是什么”，本节描述“为什么变成这样”。

| 日期 | 版本/提交 | 类型 | 功能编号 | 变更与原因 | 验证 |
| --- | --- | --- | --- | --- | --- |
| 2026-09-15 | 本地提交 | 修改 | `CUST-OPS-005` | 按用户要求提交既有 `gateway.stream_data_interval_timeout` 示例值从500秒调整为300秒的改动，并同步功能清单。后端缺省180秒、响应头超时及5秒用量任务超时不变；不连接或修改生产。 | 示例YAML解析通过，确认字段值为300；差异检查通过。仅配置示例及台账变更，未重跑全量测试或触发远端CI。 |
| 2026-09-15 | 本次提交 / CI待验证 | 提交、门禁 | `CUST-PROD-008`、`CUST-BILL-007`、`CUST-BILL-005`、`CUST-UI-001`、`CUST-OPS-004`、`CUST-OPS-005` | 按用户要求提交用户定制查询、一次性授信和输出速率，并一并提交既有 `stream_data_interval_timeout: 500` 示例修改；新增5个前端测试文件纳入现有CI关键测试清单。只推送代码，不创建发布标签，不运行Release或连接生产。 | 本地实施和收尾验证见下方记录；按CI实际参数（不附加unit标签）的本地golangci-lint复跑为0项问题。远端CI与Security Scan以本次提交的工作流最终结果为准，推送前不宣称通过。 |
| 2026-09-15 | 本地收尾 / 待提交 | 补充验证 | `CUST-PROD-008`、`CUST-BILL-007`、`CUST-BILL-005`、`CUST-UI-001` | 公开查询处理器改为注入现有限流器，不直接依赖 Redis 客户端；重新生成 Wire，功能及生产配置不变。 | 后端全量 unit、真实 Redis 限流及故障关闭、嵌入构建和公开页禁止缓存测试复跑通过；前端类型检查、全量非改写 lint 和 63 项定向测试通过。使用 Go 1.27 本地编译的 golangci-lint 2.13 补跑全量静态检查，报告 65 项既有问题，均位于本次未修改文件，本次修改及新增文件无报告项；没有为消除既有问题修改无关代码。未提交、未发布、未访问生产。 |
| 2026-09-14 | 本地实施 / 待提交 | 新增、修改 | `CUST-PROD-008`、`CUST-BILL-007`、`CUST-BILL-005`、`CUST-UI-001` | 实施首字后输出速率、用户定制免登录余额查询和按代次的一次性授信；公开查询与后台授信独立，敏感随机码仅以片段和专用头传递，安全历史最小投影，Redis限流失败关闭，授信事务、唯一约束和持久缓存失效保证可重试。新增237增量迁移，不修改生产、不自动部署，保留既有未提交部署示例修改。 | 前端类型检查、全量非改写lint、292文件2166项Vitest及构建通过；后端全量unit、嵌入构建和页面缓存/审计测试通过。真实PostgreSQL18、16完整迁移及授信集成回归（16重复3轮）覆盖阈值、32任务竞争、并发余额扣费/加款、锁争用、事务回滚、重启、代次恢复、跨用户安全历史、缓存持久重试及真实充值/返佣隔离；真实Redis双维限流和故障关闭通过。Playwright使用本地模拟接口验证1440px/390px页面、分页/手动刷新、无登录凭据、充值方式排序保存和授信恢复确认。未部署、未访问生产，未做峰值压力或真实支付/上游调用验证。Windows备份单测初次缺sh，补齐Git bin PATH后全量重跑通过。 |
| 2026-09-14 | `0.1.232` / Claude 流收尾修复（当时未部署；后随 `0.1.234` 发布） | 修复 | `CUST-GW-003`、`CUST-GW-008` | Anthropic API Key 透传在完整终止事件的空行边界转发完毕后立即成功返回，不再等待上游响应体 EOF，避免正常结束后误报流间隔超时并追加502错误。保留最终用量、客户端断连后收集用量、`[DONE]` 与既有 EOF 兼容；由原调用方关闭响应体。没有结束事件的真正超时和断流仍报错，不修改生产超时、账号配置或计费规则。 | 新增4个回归场景先确认修复前失败，再验证分段终止事件不会提前返回、完整事件全部转发、用量及断连状态保留、响应体关闭；Anthropic 透传/部分用量定向测试、service与handler全量unit、相关流收尾/超时/断连race回归通过。未推送、未部署或操作生产。 |
| 2026-09-14 | `0.1.232` / 生产资源与系统更新 | 修改 | `CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-005` | 按用户明确比例调整Go、连接池及PostgreSQL缓存，补齐容器2GiB动态共享内存；完成系统127包升级和1个新内核，SSH与GitHub部署端口改22748并继续允许密码登录，Fail2ban改1小时，持久化Nginx master限制和Redis overcommit。保留用户BBR、五个镜像、业务代码、数据挂载、计费串行与原超时；归档脱敏配置及备份清单补丁。 | 12项部署回归、3项目标门禁、候选配置仅7项运行差异、干净计费退出、停写前后核心摘要与数据库标识一致、新内核及运行参数、SSH双密钥、新旧IP入口证书/HTTP、跨域/401、真实流式用量核验通过。维护05:12:43至05:19:26，非零中断；未做峰值压测，80%Go与25%PG预算叠加风险已告知。详见docs/operations/2026-09-14-production-resource-update.md。 |
| 2026-09-14 | `0.1.232` / 运维迁移 | 修改 | `CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-005` | 生产迁至新机216.152.153.86，保持原镜像、参数、密钥、会话与 bind mount；旧机仅作 Nginx 固定 IP 转发，禁用旧应用/数据库自动启动。备份补齐 ACME 配置并排除迁移临时目录；更新 GitHub 部署目标和独立部署密钥。首轮清理未完成后恢复旧机，经用户接受维护窗口后第二轮完成单写切换。 | 物理备份校验、隔离演练、最终 WAL/Redis 位置和核心数据摘要一致；第二轮清理完成且退出码0；新 IP 与旧入口转发、公网健康、流式用量、跨域和401权限门禁通过；10条 A 记录及保留属性核对通过；续期演练成功。没有宣称首轮完全无损，实际登录/支付仍需用户验收。详见 `docs/operations/2026-09-14-server-migration.md`。 |
| 2026-09-07 | `0.1.231` / 本次发布提交 | 修复 | `CUST-BILL-002` | 移除 Fable 5.1 max 内置3倍计费和账号统计加价，删除模型广场/可用渠道的默认倍率注入，渠道表单改为“默认1（不额外加价）”；显式渠道倍率及请求推理等级不变。官方公开价卡输入/输出为 $10/$50，思考已计入输出 Token，不应额外将全部费用乘三。以线上截图的4输入、163输出、3042缓存写入、192039缓存读取复算：原价 $0.09422475，用户倍率1.10后 $0.103647225；历史账单与余额不回写，未部署生产。 | Go service 包全部 unit 测试通过；回归覆盖 fallback/目录/渠道三条定价路径、六种推理等级、5分钟/1小时缓存、零用户倍率、账号统计和公开展示；前端33项定向测试、类型检查、受影响文件 ESLint、后端全包构建、前端生产构建与差异检查通过。首次远端CI发现后台默认定价接口测试仍断言max倍率为3，已修正为未配置，并补跑后端全包unit测试；未执行生产部署或历史账单修正。 |
| 2026-09-07 | `0.1.229` / 实例配置 | 修改 | `CUST-OPS-005` | 经授权，在服务器1升级至 12 vCPU、约 32 GiB 内存后，仅将 `GOMAXPROCS` 从 4 改为 12、`GOMEMLIMIT` 从 3GiB 改为 10GiB、`POSTGRES_SHARED_BUFFERS` 从 512MB 改为 4GB、`POSTGRES_EFFECTIVE_CACHE_SIZE` 从 4GB 改为 16GB。不改仓库通用默认值、不构建或拉取镜像，不调整连接池、用量 worker/队列、串行扣费、5 秒任务超时及 3600 秒响应头覆盖；配置备份仅存于受限权限且被 Git 忽略的本地工作目录。详见 `docs/operations/2026-09-07-resource-tuning.md`。 | 按服务器北京时间 2026-09-07 02:55:01 至 02:55:52 执行应用停止、PostgreSQL/应用重建及健康确认。候选字节和 Compose 解析仅四项差异；容器实际环境及 SQL SHOW 核验通过，双 Compose、持久化应用配置、镜像、精确 bind mount 和回环端口未变，Redis 容器标识与启动时间未变。截至 02:59:55，重建后新增 769 条用量记录，其中 763 条有输出；观察期间三容器持续健康、无重启/OOM，计费异常日志为零；未压测，不承诺 RPM 提升倍数。 |
| 2026-09-06 | `0.1.230` / 发布候选 | 新增、修改 | `CUST-ACC-005`、`CUST-ACC-007`、`CUST-GW-005`、`CUST-GW-006`、`CUST-GW-012` | 按确认方案原子复制源测试计划，副本及计划初始暂停，清空历史并补齐唯一恢复模板。故障期间普通 Cron 让位于自动测活，系统任务按二开退避且同账号不重入，执行前复核旧任务快照；现有 extra 保存独立退避计数和人工暂停标记，不受结果清理影响、不解除人工暂停、不覆盖新事故。测试默认模型更新为 Astra/Opus 5/Gemini 3.8 Flash/Grok 4.6；国产平台从配置中确定性选择具体文本模型并保持单次映射；保留受限接入、显式模型、Grok 媒体模式和事故模型优先级。不改业务/计费/额度探测默认值、生产配置或数据库结构。 | 本地 Go 全量 unit、golangci-lint、后端构建及集成测试编译通过；前端 lint/typecheck、40 项相关 Vitest 和生产构建通过。新增真实数据库回归覆盖模板优先级、事务回滚、人工暂停、仅保留一条结果时的完整退避、重复启用和旧执行保护；本地没有 Docker/C 编译器，真实 PostgreSQL/Redis 集成与 race 未在本机执行。模拟 API 的1440桌面与390手机弹窗确认 Astra 预选、实际提交及成功状态；CI/Security Scan 和生产发布验收须在该提交推送后执行，不提前记为通过。 |
| 2026-09-06 | `0.1.230` / 发布前复核 | 修复 | `CUST-GW-005`、`CUST-GW-006` | 账号同时存在人工暂停和系统故障时，普通 Cron 仍跳过出站测试，系统恢复任务也不自动启用；人工暂停不能成为普通计划绕过故障退避的条件。 | 增加该组合状态的定向回归；最终补丁提交须重新通过 CI 与安全扫描。 |
| 2026-09-07 | 工作区 / 待提交 | 新增、修改 | `CUST-GW-001`、`CUST-GW-004`、`CUST-GW-005`、`CUST-GW-006`、`CUST-GW-012` | 为指定高速响应分组增加首 Token 超时范围选择、严格空白判定与计数隔离；复用连续阈值停调度、现有换号次数和退避计划，不增加并发竞速或总预算。恢复采用事故模型与时限，避免测试默认模型、重复映射或慢速最终成功误恢复；保留账号级全局停调度的既有边界并在配置页提示。未修改全局超时默认值、生产配置或发布版本。 | Go unit 的 service、handler 全部子包及 repository 全量回归、后端构建、前端 lint/typecheck/配置页16项 Vitest/生产构建通过。回归覆盖指定组与协议边界、计数隔离、空白后超时或流内错误仍可换号、超过三账号仍沿用原换号上限、事故模型与达速恢复、Bedrock 流式探测。模拟 API 的1440像素桌面与390像素手机检查通过，无横向溢出。本地无 C 编译器，未执行 race；真实上游验收和生产部署未执行，未连接或重启服务器。 |
| 2026-09-05 | `0.1.229` / 实例配置 | 修改 | `CUST-OPS-005` | 排查 Claude-Max【蒸馏组】非流式请求约 10 分钟后返回 502，确认直接触发条件为本站等待上游响应头达到默认 600 秒。经授权，将服务器1现有数据挂载中的 `gateway.response_header_timeout` 覆盖为 3600 秒，与现有 Nginx 3600 秒代理读取超时对齐；不修改后端通用默认值、不重建镜像，仅重启应用读取配置。保留原有 OpenAI/Codex、Grok 独立超时、数据库连接与按用户串行扣费、5 秒用量任务超时和生产挂载；含凭据的原配置仅备份到本地受限权限且被 Git 忽略的工作目录。 | 候选 YAML 解析、仅目标字段的语义差异、配置指纹与权限检查通过；2026-09-05 21:44:24（北京时间）仅重启应用，容器内配置值 3600、HTTP 健康检查和 Docker healthy 均通过。双 Compose 与 `.env` 指纹、三项生产 bind mount、数据库/Redis 容器 ID 及启动时间均未变化；重启后该分组已新增 28 条有输出的用量记录。未执行真实上游等待满 3600 秒的边界测试。 |
| 2026-09-05 | `0.1.229` 发布候选 | 修复 | `CUST-OPS-003`、`CUST-OPS-004` | 发布前发现服务器1的备用Compose落后于活动文件；先在本地忽略目录备份备用文件，再以活动文件对齐，未更改活动配置或重建容器。将既有运维约束落实为Release目标校验及远程重建前失败关闭门禁：指定Compose、双文件一致、三项精确bind mount和旧实例健康全部通过后才允许更新版本和重建；缺少健康工具不再视为成功。 | 新增12项无真实Docker/网络/生产配置的mock回归，部署脚本及测试语法、diff检查通过；macOS CI夹具仅适配BSD sed参数。提交后重新跑CI及安全扫描，以最新候选SHA为合入门禁；此记录不将尚未开始的发布写成已部署。 |
| 2026-09-05 | `0.1.228` / `289a00a46` | 上游适配 | `CUST-GW-001`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-003`、`CUST-PROTO-005`、`CUST-ACC-001`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-PROD-006`、`CUST-RISK-002`、`CUST-UI-004` | 完整合并固定上游5097b3145..ab99d56e9的82提交，解决25文件/43冲突块；接入Astra、ultrafast、价格热重载、max倍率、上游请求ID、固定账号只读清单、轻量账号列表、续聊/回放/Cyber修复及支付宝补查。保留严格请求模型/旧来源归一化、未知别名显式价格、Free Fast双成本、长上下文特例、WS逐轮结算和插件已发送禁止重放；图片强制隐藏上游URL优先于可选回填，下载专属公网防护不改业务HTTP开关；EasyPay独立预算与查询集合去重。请求ID敏感头在写/读两端拒绝，认证快照v23同时保留本地字段，生产约束未变。53项/9域与既有编号均保留。 | Go默认及unit全包、build、golangci-lint2.13的0 issues、Wire与Ent320文件零差异、integration仅编译、前端lint/typecheck/274文件1994用例/build、Canvas格式/类型/8文件34用例/build、三部署静态/Caddy/双Compose字节一致和差异检查通过；工具与兼容失败均已定位且完整重跑0。真实数据库/迁移/集成执行、启动健康、真实业务和浏览器E2E未验证；未push、未部署、未访问服务器。 |
| 2026-09-02 | `0.1.228` / `736761ba3` | 上游适配 | `CUST-GW-001`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-ACC-001`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-OBS-001`、`CUST-UI-004`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005` | 完整合并上游 `b5827cfd5..5097b3145` 的 137 个提交（42 个 merge、95 个 non-merge），接入分组 Fast/Free Fast、按模型范围限制 reasoning effort、Kimi 原生 Responses、服务层级分离、价格目录阶梯/覆盖文件、原生 compaction 用量、TTFT 管理设置、WS 会话隔离和数据库启动重试。继续保留普通文本严格按请求模型计费、Free Fast 用户 Standard/账号 priority 双成本、上游服务层级只降不升、Gemini 200K 边际价、GPT-5.6 272K 与 Grok 200K 严格边界、Kimi `store=false`、WS 逐轮模型映射/哈希/结算/失败阻断、按用户串行扣费、5 秒 usage task 超时，以及生产 bind mount、回环暴露、HTTP upstream 和资源变量入口；逐项解决 17 个文本冲突，版本保持 `0.1.228`，未新增二开编号。 | Go 默认及 unit 标签全包、全包构建、Wire 生成、integration 标签仅编译、前端 lint/typecheck/271 文件 1962 项 Vitest/生产构建、Canvas format/typecheck/8 文件 34 项 Vitest/生产构建、部署静态脚本、双生产 Compose 字节一致及 Ent 隔离生成比对通过。golangci-lint 2.9.0 因目标 Go 1.27 高于其构建工具链而拒绝运行；Apple fixture 仅因 Windows 不支持 BSD `stat -f '%Lp'` 失败。Docker/Testcontainers、真实 PostgreSQL/Redis/迁移、Compose 启动、健康检查、真实上游凭据和生产部署未验证。 |
| 2026-08-30 | `v0.1.228` 发布修复 / 待提交 | 修复 | `CUST-OPS-001` | `v0.1.228` 首次完整发布在并行构建 Docker Hub/GHCR 的 amd64/arm64 镜像时超过 GoReleaser 默认 60 分钟，自动部署被跳过；将完整发布命令超时提升为 120 分钟。 | Release 工作流 YAML 与部署静态检查；通过 `workflow_dispatch` 对既有 `v0.1.228` 重跑完整发布并验证生产部署。 |
| 2026-08-30 | `0.1.228` / 待提交 | 修改 | `CUST-OBS-002` | New API 上游新增个人访问令牌认证：管理页允许在密码与访问令牌之间选择，令牌模式只提交并加密保存个人访问令牌，不接受或复用用户导入的会话 User-Agent、刷新令牌及 Cookie；后端自动设置固定浏览器 User-Agent 并启用 Chrome TLS/HTTP2 指纹，直接以 Bearer Token 验证和同步余额、分组、定价及统计，首次 `/api/user/self` 成功后记录远端用户 ID，令牌被拒绝时不调用刷新接口或回退密码登录。密码模式原有旧 Cookie 及 rc.22 短期 Access Token/刷新 Cookie 兼容逻辑保持不变；从密码模式切换令牌模式会先清除短期登录会话，避免将缓存会话 Token 误当作个人访问令牌。 | New API 令牌参数校验、密码会话切换隔离、Bearer 请求头/用户 ID 回填、无登录会话同步、令牌拒绝不回退登录，以及管理页密码/令牌模式载荷测试；使用外部 New API 测试站点从服务器 1 网络出口运行 Linux Provider 只读接口探针。 |
| 2026-08-30 | `0.1.226` / `1095fc942` | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-ACC-001`、`CUST-ACC-002`、`CUST-ACC-003`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-005`、`CUST-PROD-002`、`CUST-PROD-006`、`CUST-RISK-002`、`CUST-UI-004`、`CUST-OPS-004` | 完整合并上游 `aa2c4e8d1..b5827cfd5` 的 136 个提交（63 个 merge、73 个 non-merge），接入分组路由 Codex 模型清单及能力同步、用户公开分组限制、请求/映射后推理强度记录、DeepSeek 官方峰谷价、EasyPay 相对地址补全、非流式终态失败换号、WS 正常关闭归因、实际上游端点和统一传输错误处理。继续保留本地首语义输出与响应前心跳、请求体释放、插件请求发出后不可重放、严格请求模型计费及分组/渠道自定义价优先级、API Key 专属分组约束、本地模型市场、充值赠送/到账余额/汇率预览、Cyber 严格门禁、表格浮层和生产约束；逐项解决 20 个文本冲突，移除已被本地严格计费链路替代的两个上游遗留辅助函数，版本保持 `0.1.226`，未新增二开编号。 | Go 默认及 unit 标签全包、golangci-lint v2.13（`0 issues`）、CGO 关闭全包构建、repository integration 标签编译、前端 lint/typecheck/268 文件 1920 项 Vitest/生产构建、Canvas format/typecheck/8 文件 34 项 Vitest/生产构建、Ent/Wire 隔离重复生成一致、Compose 安全/Gateway 环境变量/Docker 资源/Caddy 静态检查及双生产 Compose 字节一致性通过；Apple fixture 仅因 Windows 不支持 BSD `stat -f '%Lp'` 而失败，与同步前基线一致。Docker/Testcontainers、race、govulncheck、真实 PostgreSQL/Redis 与迁移 231、Node 20、真实上游凭据/插件、本地健康检查和生产部署未验证。 |
| 2026-08-25 | `0.1.225` / 本提交 | 上游适配 | `CUST-ACC-004`、`CUST-GW-001`、`CUST-GW-006`、`CUST-GW-008`、`CUST-BILL-002`、`CUST-OPS-004`、`CUST-UI-002` | 完整合并上游 `2bc139ab5..aa2c4e8d1` 的 207 个提交，接入 OAuth outbound transport 插件、OpenAI 重置卡阈值自动使用、Fast service tier、统一 token 计费、OpenAI/Grok/Responses Lite 兼容修复、调度诊断及 Go 1.27.0；继续保留本地首语义输出暂存、插件请求发出后不可重放、严格缺价与分组/渠道定价优先级、Grok 严格 `>200K` 长上下文边界、自动用卡限流代次隔离、公共开关默认语义及生产 Compose 约束。自动用卡恢复快照由幂等 owner 在首次出站前持久化，超时重试和成功回放复用原始快照；逐项解决 42 个文本冲突，版本保持 `0.1.225`，未新增二开编号。 | Go 默认标签与 unit 标签全包测试、golangci-lint 2.13（`0 issues`）、CGO 关闭全包构建、前端 lint/typecheck/264 文件 1881 项 Vitest/生产构建、Canvas format/typecheck/34 项 Vitest/生产构建及受影响定向回归通过；repository integration 仅编译通过，部署静态检查和双生产 Compose 一致性通过。Docker/Testcontainers、真实 PostgreSQL/Redis 与迁移 229/230、真实上游凭据/插件包、本地健康检查和生产部署未验证。 |
| 2026-08-22 | `0.1.224` / 待提交 | 修复 | `CUST-PROD-006` | EasyPay 异步通知未到达时，普通订单轮询只读取本地状态，主动补查又仅覆盖微信支付，导致已支付订单要等到过期扫描才入账。现将仍在有效期、创建至少 30 秒且已完成支付初始化的 EasyPay 非微信待支付订单纳入单 leader 周期补查，每轮最多查询 10 笔且不取消未支付订单；支付确认继续复用订单状态 CAS、履约租约和兑换码事务，避免与迟到通知并发时重复入账，并新增 `PAYMENT_RECONCILED` 来源审计。 | EasyPay 已支付自动补发、未支付不取消、非 EasyPay/过新/过期/未初始化订单过滤、批次上限、重复补查幂等及支付生命周期定向单元测试。 |
| 2026-08-21 | `0.1.223` / 待提交 | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-004`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-004`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-PROTO-007`、`CUST-ACC-005`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-OBS-001`、`CUST-OBS-002`、`CUST-PROD-001`、`CUST-PROD-002`、`CUST-PROD-007`、`CUST-RISK-002`、`CUST-UI-002` | 完整合并上游 `5253bb72b..2bc139ab5` 的 171 个提交，接入渠道监控 `quota`/`quota_probe`、渠道服务层级/时段/token 区间定价、CN 自适应协议、Responses input tokens 与客户端工具恢复、OpenAI WS 后续轮次请求级容量换号、Codex 随机种子指纹及 OpenAI Team 联动熔断；继续保留本地首个语义输出前超时与暂存、非语义心跳扣除、部分输出后的 failover 边界、严格请求模型计费、长上下文双门禁、流式渠道监控与 600 秒超时、Responses 结构化 input、Gemini 全分段、独立上游同步 runner、公共 Canvas/每日签到/本地模型市场开关及 cyber 用量记录边界。逐项解决 30 个文本冲突，版本保持 `0.1.223`，未新增二开编号。 | 同步前后 Go 全量 unit、golangci-lint、CGO 关闭构建、Vue lint/typecheck/257 文件 1802 项 Vitest/生产构建、Canvas format/typecheck/34 项 Vitest/生产构建通过；Ent/Wire 隔离重生成一致，新增迁移静态测试、双生产 Compose 字节一致、部署脚本语法/安全/资源/Caddy 及假 Token 隔离测试通过。Docker/Testcontainers、race、govulncheck、真实 PostgreSQL 迁移、Node 20、外部凭据、本地服务健康检查和生产部署未验证。 |
| 2026-08-18 | `0.1.222` / 待提交 | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`、`CUST-GW-011`、`CUST-PROTO-001`、`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-PROTO-008`、`CUST-ACC-001`、`CUST-ACC-003`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-OBS-002`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-001`、`CUST-UI-004`、`CUST-PROD-007`、`CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005` | 完整合并上游 `fbfdcef81..5253bb72b` 并保留本地二开边界：Responses native v2 与 legacy compact 分流、首输出后请求体释放、部分 usage、成功审计、客户端断连排水和 WS 每轮真实开始时刻继续生效；普通文本严格按用户请求模型计费，响应模型仅作诊断，Composite 只有显式渠道价才按别名计费；CN 供应商的 `claude-*` 候选无显式分组/渠道价时不套 Claude 内置价，成功请求保留零费用 usage/幂等审计并返回 `ErrModelPricingUnavailable`。分组日汇总新增前向迁移，使当前开放日 INSERT 遇回填锁快速放行，历史及跨午夜写入仍串行回退水位。未新增二开编号。 | 受影响 Go 定向单元、WS relay/adapter 生命周期与计时回归、全包仅编译、迁移静态断言；真实 PostgreSQL 迁移重复执行、当前日 5 秒窗口非阻塞、历史/跨午夜串行失效测试通过。 |
| 2026-08-17 | `0.1.222` / 待提交 | 修复 | `CUST-PROD-007` | 修复画布 Grok 视频沿用 OpenAI multipart 请求导致 xAI `/v1/videos/generations` 返回 415、创建响应 `request_id` 无法识别，以及完成响应 `done + video.url` 无法落盘的问题。画布按分组平台选择协议：Grok 直接发送官方 JSON `model/prompt/duration/resolution/aspect_ratio/image`，并把清晰度和比例映射到 xAI 合法枚举；OpenAI 保持 `/v1/videos` multipart、单数 `input_reference`、官方尺寸及 `4/8/12` 秒枚举；后端同时把旧客户端的 `seconds/resolution_name/size/input_reference` 转为 xAI JSON，兼容字符串时长、data URL、任务 ID 别名和失败终态，审核与计费继续读取转换前语义。同步按官方 schema 复核其他生成接口：GPT Image 不再发送不支持的 `response_format`，OpenAI/Grok 单次图片数限制为 10；Grok 图片移除 OpenAI 专属选项并把尺寸映射为 `aspect_ratio/resolution`，多图编辑保证 `image/images` 互斥且明确拒绝官方不支持的 mask；Gemini 使用 `generationConfig.imageConfig` 并过滤仅支持 `predict` 的 Imagen 模型，SSE `inlineData` 解析保持不变。 | OpenAI 官方 Videos/Images API、xAI Images/Videos REST schema 与 Gemini `streamGenerateContent` schema 逐字段核对；Grok 媒体 Go 定向回归及重复运行通过，画布 8 个 Vitest 文件共 34 项、TypeScript、Prettier、生产构建和差异检查通过。Go `internal/service` 全包除既有 `TestGrokQuotaServiceQueryQuotaCustomPaidMonthlyLimitSkipsActiveProbe` 过期账期夹具导致额外主动探针外均执行，该失败与本次媒体文件无依赖；未调用真实计费媒体接口，未连接或部署生产服务器。 |
| 2026-08-16 | `0.1.221` / 待提交 | 修复 | `CUST-OBS-001` | 将渠道探针的请求总超时和响应头超时由 45/30 秒统一提高到与真实网关默认值一致的 600 秒，避免慢响应模型在真实用户请求成功前被探针提前判红；Gemini 流式与非流式解析统一遍历首个 candidate 的全部 part，并跳过 `thought=true` 的思考分段，避免最终答案不在首个 part 时产生 challenge mismatch 假红。 | 探针超时常量及 Gemini 多分段/思考分段定向回归、渠道监控 service 单元测试。 |
| 2026-08-16 | `0.1.220` / 待提交 | 修复 | `CUST-PROD-007` | 修复画布 Gemini/Antigravity 图片及图片编辑使用非流式 `generateContent` 时，特定上游链路长时间不返回完整响应并被前置代理断开的问题：统一改为 `streamGenerateContent?alt=sse`，按 SSE 事件增量解析 `inlineData`、`inline_data` 和文件 URI，兼容事件跨网络分片、前置文本、累计事件去重、响应包装及非 SSE JSON 回退；流内和 HTTP 错误优先显示上游真实原因，仅在流结束仍无图时报告无图片。生产只读核验确认同一账号的两次非流式画布请求分别耗时 141.043 秒和 137.659 秒后取消，而流式账号图片测试 10.728 秒完成，问题在协议链路而非模型实际生成耗时。同步审查其他画布生成链路：Gemini 文本/助手已使用 SSE，视频均创建任务后轮询，Gemini 视频/音频请求前拒绝；OpenAI/Grok 图片和普通音频没有可通用替换的同协议流式返回格式，维持现有站内接口。 | React 8 个 Vitest 文件共 27 项、类型检查、Prettier 和生产构建通过；SSE 回归覆盖跨分片、前置文本、多图去重、图片编辑参考图、字段命名兼容、JSON 回退、真实错误及无图终态；差异检查通过。未执行新的真实计费媒体请求，生产服务器仅进行日志和配置只读核验。 |
| 2026-08-16 | `0.1.219` / 待提交 | 修复 | `CUST-PROD-007` | 修复画布图片接口已成功返回 base64 结果后，前端使用 `fetch(data:)` 触发 CSP 拦截并显示 `Failed to fetch` 的问题：data URL 改为浏览器本地解码 Blob，图片、媒体和导出共用安全转换；画布专属 CSP 补充 `data:`/`blob:` 连接源及 HTTPS 视频兜底，其他页面策略不变。切换分组后重试会重新解析当前分组的同能力模型，当前分组无对应模型时停止请求并明确提示；批量生成节点保留首个真实错误，不再被“全部生成失败”覆盖。OpenAI/Grok 兼容视频完成后优先读取站内内容接口，远程 URL 仅作兜底；Gemini 视频继续在请求前明确拒绝。新建生成配置及旧版默认值均由 3 张调整为 1 张。 | React 8 个 Vitest 文件共 22 项、类型检查和生产构建通过；Vue 主站类型检查和生产构建通过，并按 Vue 后 React 顺序验证最终产物；Go 全量单元测试及画布 CSP 定向测试通过；差异与格式检查通过。未执行真实计费媒体请求。 |
| 2026-08-16 | `0.1.218` / 待提交 | 修复 | `CUST-PROD-007` | 上线前复核修复专属分组替换与画布托管 Key 的两个一致性问题：旧分组托管 Key 在事务内撤销而不迁移，避免目标分组已有托管 Key 时触发唯一索引冲突；事务前后分别采集用户 Key 并去重失效认证缓存，确保已软删除的旧画布凭据不能在缓存 TTL 内继续访问旧分组。用户和管理员均不能修改托管 Key 的分组或用途，仍允许删除后按需重建。补齐站内 OpenAI/Grok、Gemini 图像、视频轮询、Gemini 视频拒绝和运行时 Key 不持久化的请求级回归；本次未连接或部署生产服务器。 | Go 全量 unit 与 integration 标签测试、golangci-lint、嵌入式后端构建；Vue lint/typecheck、16 个关键 Vitest 文件共 182 项、生产构建；React format/typecheck、7 个 Vitest 文件共 13 项、生产构建；迁移 SQL、静态 SPA 回退、CSP/XFO、双前端构建顺序和高危依赖审计检查通过。真实计费媒体调用继续未执行。 |
| 2026-08-15 | `0.1.218` / 待提交 | 新增 | `CUST-PROD-007` | 集成固定版本 Infinite Canvas 为独立 React 子项目，由 Vue 登录页面通过严格校验来源的 `postMessage` 提供用户初始化信息和临时站内 Key；新增画布功能开关、分组/模型发现、OpenAI/Grok 图片与视频、Gemini/Antigravity 图片、按用户 IndexedDB 命名空间、内置及本地提示词、受白名单与双重确认约束的画布助手。新增 API Key `purpose`、每用户每分组托管 Key 部分唯一索引、普通 Key 优先复用和并发创建恢复；静态服务为 `/canvas-app/*` 提供独立 SPA 回退、缓存和同源 iframe 安全头。构建及 CI 固定 Vue 先构建、React 后构建，保留上游 MIT 许可证并记录裁剪范围；本次未连接或部署生产服务器。 | React 类型检查、13 项 Vitest、Vue 类型检查/定向测试/lint、Go 凭据服务定向测试（含权限、复用、自动创建、跨用户隔离和并发唯一性）、静态服务及 CSP/XFO 测试、双前端生产构建；桌面与移动浏览器验收覆盖 Mock 图片生成、Mock 视频请求适配与任务轮询，未执行真实计费媒体调用。 |
| 2026-08-15 | `0.1.218` / 待提交 | 修复 | `CUST-BILL-003` | 修复 API Key 认证缓存快照遗漏分组长上下文开关，导致数据库已开启但认证热路径还原为 `false`、Grok 用户扣费未应用 200K 阶梯的问题。快照补齐开关的双向映射并升级到 v20，旧 v19 Redis 快照自动失效回源；无数据库和 HTTP API 变化。 | 缓存开关开启/关闭往返、旧 v19 快照淘汰，以及截图同口径的 Grok 4.6 `1567 input + 272128 cache read + 252 output` 端到端计费回归；Go 定向与全量 unit、golangci-lint、CGO 关闭构建、govulncheck 通过，远端 CI 与生产新请求核验作为发布门禁。 |
| 2026-08-15 | `0.1.217` / 待提交 | 修复 | `CUST-BILL-002`、`CUST-BILL-003`、`CUST-OPS-004` | 修复 Grok 动态价卡存在但缺少 200K 阶梯时遮蔽正确内置兜底的问题：兼容 xAI/LiteLLM above-200K 绝对价格，仅对 Grok 转换为内部阈值和倍率；对已知残缺 Grok 价卡只补缺失阶梯，并仅在 4.5 命中 `$0.50/MTok` 已知错误签名时修正为官方 `$0.30/MTok`。完整远端价卡继续优先，默认远端 URL、10 分钟哈希检查和自动更新流程不变；内置资源补齐 Grok 价卡，200K 边界按官方“超过”语义处理。首次远端安全扫描同时发现 Go 1.26.5 标准库新公告，构建与 CI 统一升级到已修复的 1.26.6。 | xAI 原生字段/显式字段优先/非 xAI 隔离/异常倍率测试，Grok 4.5/4.6 残缺与完整动态价卡测试，内置资源价卡测试，200K/200001 边界及分组开关测试；外部价表 JSON、生成哈希与二次同步幂等验证；Go 1.26.6 全量 unit、golangci-lint、CGO 关闭构建、govulncheck、内置 JSON、生产 Compose 一致性和 diff 检查通过，远端 CI 与生产部署核验待完成。 |
| 2026-08-13 | `0.1.215` / `a39851432` | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-004`、`CUST-GW-005`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-011`、`CUST-PROTO-001`、`CUST-PROTO-006`、`CUST-ACC-001`、`CUST-ACC-002`、`CUST-ACC-004`、`CUST-ACC-005`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-001`、`CUST-UI-004`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005` | 完整合并上游 `48eb3766d..fbfdcef81` 的 111 个提交，接入 Responses/WS 错误与 TTFT 修复、分卷备份、API Key 校验、Codex 指纹收敛、Grok 订阅档位与 4.6、原生 `x_search`、分组逐模型定价、长上下文开关、渠道缓存刷新、定时备份 leader 锁及相关前端展示。继续保留普通分组严格按用户请求模型计费、响应模型仅观测与 mismatch 诊断、Composite 显式别名价例外、分组定价优先级、真实分组长上下文门禁、Cyber Enabled/Mode/scope 门禁、附加换号与 Pool Mode 自定义状态码、自动托管测试/连续失败依赖、按用户串行扣费和 5 秒 usage task 超时；版本保持本地 `0.1.215`，未新增二开编号。PR #9 首轮安全扫描发现新发布的 `nanoid` 高危公告，使用精确 override 将全部生产依赖链从 `3.3.17` 升级到修复版 `3.3.18`，未新增审计豁免。 | 同步前后本地 Go 全量 unit、golangci-lint、CGO 关闭构建、前端冻结安装/lint/typecheck/全量 Vitest/生产构建通过；Ent 隔离双生成逐文件一致，Wire 重复生成稳定，双生产 Compose 字节一致且本次无 `deploy/` 变更。安全修复后本地审计例外检查和前端全量回归通过；PR #9 对代码提交 `a39851432` 的 push/PR 双触发 CI、Security Scan、Go Unit/Integration、golangci-lint、macOS 部署脚本和 Node 20 前端检查全部通过。未运行 race，未执行生产数据迁移、服务健康检查或部署。 |
| 2026-08-09 | `0.1.214` / `9c63ee6f6` | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-006`、`CUST-GW-007`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-PROTO-007`、`CUST-ACC-001`、`CUST-ACC-004`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-004`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-OBS-001`、`CUST-PROD-002`、`CUST-PROD-006`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-002`、`CUST-UI-004`、`CUST-UI-005`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005` | 完整合并上游 `7e2e9ba05..48eb3766d` 的 223 个提交，接入 Captcha/OAuth 安全修复、Codex 官方版本同步、Channel Monitor V2、Grok 搜索/媒体/语音/实时完整链路、上游响应模型诊断、订阅/退款并发修复、邮箱域名注册额度与依赖安全更新；逐文件解决 49 个文本冲突。继续保留首 Token 与部分输出禁止重试、客户端断连及成功审计、请求体及时释放、WS 逐轮并发/计费/终态错误、严格按用户请求模型计费、Claude/Codex 本地协议策略、图片分组成功率、公开注册页与存储容错、本地模型市场、内容审核/Cyber，以及生产 bind mount、仅回环暴露和实例资源参数。版本保持本地 `0.1.214`，未新增二开编号。 | Go 全量 unit、golangci-lint、CGO 关闭构建、前端 lint/typecheck/全量 Vitest/生产构建及受影响定向回归通过；Ent/Wire 重复生成摘要稳定，双生产 Compose SHA-256 一致。integration、race、govulncheck、真实外部凭据、本地依赖服务启动和生产部署未验证 |
| 2026-08-02 | `0.1.213` / `6884fa682` | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-004`、`CUST-GW-006`、`CUST-GW-007`、`CUST-GW-008`、`CUST-GW-010`、`CUST-GW-011`、`CUST-PROTO-001`、`CUST-PROTO-002`、`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-PROTO-008`、`CUST-ACC-001`、`CUST-ACC-003`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-PROD-002`、`CUST-PROD-006`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-002`、`CUST-UI-004`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-005` | 完整合并上游 `5a6143097..7e2e9ba05` 的 99 个提交，接入 Anthropic classifier/count-tokens、Codex namespace 与工具图片、OpenAI WS/compaction、流式部分 usage、全 API-key 平台倍率探测与写回、分组利润控制、安全审计、内容审核代理、支付设置、Compact 首页、依赖定价及部署安全更新；逐文件解决 22 个文本冲突。继续保留严格 Claude Code 协议头判定、非流式 HTTP 不走 WS、首 Token/usage/failover 结算边界、普通分组按请求模型计费、Composite 例外、媒体实际模型计费、按用户串行扣费与 5 秒 usage task 超时、本地模型市场、内容审核/Cyber、附加换号状态码、表格稳定性以及生产 bind mount、仅回环暴露和实例资源参数；版本保持 `0.1.213`。未新增二开编号。 | 同步前后 Go 全量 unit、golangci-lint、CGO 关闭构建、前端 lint/typecheck/全量 Vitest/生产构建及受影响定向回归通过；Ent/Wire 重复生成稳定，双生产 Compose 字节一致，部署静态脚本通过。`go test -tags=integration ./...` 因本机无 Docker、外部 TLS 探针被拒绝及未缓存依赖下载超时退出 1；Apple 生命周期 fixture 因 Windows Git Bash 不支持 macOS `stat -f '%Lp'` 退出 1，均作为环境限制记录 |
| 2026-07-30 | `0.1.212` / 待提交 | 新增 | `CUST-GW-011` | 二开网关配置新增附加换号状态码开关和列表，默认关闭并预填 451；开启后在 Anthropic、OpenAI/Grok、Gemini、Antigravity、Bedrock 等路径中将命中状态码并入现有换号流程，同时保留上下文超限、cyber_policy 硬阻断和明确内容拒绝等语义终止规则。配置使用现有 settings 存储和运行时缓存，无需数据库迁移或重启。 | 配置默认值/持久化/规范化/非法值测试，Gateway、OpenAI/Grok、Gemini、Antigravity 换号判断与语义保护测试，Anthropic 首账号 451 后下一账号成功回归，管理接口和前端表单兼容/校验测试，Go 定向测试、前端 Vitest 与 typecheck |
| 2026-07-29 | `0.1.211` / 待提交 | 修复 | `CUST-UI-004` | 用户使用记录、API 密钥及管理端使用记录、用户、分组、订阅页的列设置菜单统一改为挂载到 `body` 的 fixed 浮层，并按视口空间自动向上或向下展开；修复原有卡片内 absolute 菜单被 DataTable sticky 表头高层级覆盖的问题。账号页列设置已位于 Teleport 的更多工具浮层中，无需重复改造。 | 组件定位/层级/向上展开回归，用户与管理端使用记录页面回归，六个入口统一实现检查，前端 lint、typecheck、全量 Vitest 和生产构建 |
| 2026-07-29 | `0.1.211` / 待提交 | 上游适配 | `CUST-GW-006`、`CUST-BILL-005`、`CUST-BILL-006` | 完整合并上游 `8fd01c281..5a6143097` 的 8 个提交：OpenAI Live observer 在 store 故障时有限重试并于会话到期兜底 finalize，用量日志改走带日志和同步重试的 best-effort 写入，避免 Redis 抖动导致租约与唯一用量记录静默丢失；继续保留 Live 零计费现状、按用户串行扣费和 5 秒 usage task 超时。同步接入 Claude Sonnet 5 状态别名及 Passkey 禁用时跳过凭据请求、按 `error.reason` 静默处理的修复；版本继续保留本地 `0.1.211`。 | 同步前后 Go 全量 unit、golangci-lint、后端构建、前端离线冻结安装/lint/typecheck/全量 Vitest/生产构建均通过；OpenAI Live、账号状态和 Passkey 定向回归通过；Compose 一致性、祖先关系、冲突标记、意外删除、敏感路径及空白检查通过。Docker/integration/Testcontainers、govulncheck、真实 Redis/OpenAI Live/Passkey 硬件和本地服务健康检查未验证 |
| 2026-07-29 | `0.1.211` / 待提交 | 上游适配 | `CUST-GW-001`、`CUST-GW-002`、`CUST-GW-003`、`CUST-GW-004`、`CUST-GW-006`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-002`、`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-006`、`CUST-PROTO-008`、`CUST-ACC-001`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-PROD-002`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-001`、`CUST-UI-002`、`CUST-UI-004`、`CUST-UI-005`、`CUST-OPS-003`、`CUST-OPS-004` | 完整合并上游 `6d956bdc2..8fd01c281` 的 127 个提交，接入 OpenAI Live、Passkey、上游 Model Plaza、面板限流、Ollama 刷新、邮箱别名去重、Kimi K3 和协议兼容更新；保留逐轮 WS 并发/结算/审计、首 Token 与失败停调度、普通文本严格请求模型计费、Composite 例外、媒体实际模型计费、Claude handler 严格客户端判定、Pool Mode 空响应重试边界、本地 `/models` 模型市场、每日签到、内容审核/Cyber、DataTable 与生产 Compose 约束。为避免同名产品混淆，将 `CUST-PROD-002` 清单名称明确为本地模型市场，并记录上游 `/model-plaza` 默认关闭且独立共存。 | Ent 临时 target 与正式生成结果逐文件一致，Wire 重复生成稳定；Go 全包编译/全量 unit、golangci-lint、CGO 关闭构建、前端 lint/typecheck/全量 Vitest/生产构建、Compose 一致性、冲突路径和空白检查通过；Docker、govulncheck、integration、race、真实服务和外部凭据流程未验证 |
| 2026-07-28 | `0.1.211` / 待提交 | 修复 | `CUST-OBS-002` | 兼容 New API `v1.0.0-rc.22` 将后台认证改为短期 Bearer Access Token 与刷新 Cookie：密码登录提取并加密保存 Access Token，受保护接口统一携带 Bearer Token；旧 Cookie 或失效 Access Token 收到 401 时优先调用 `/api/user/auth/refresh` 恢复原会话，仅在刷新凭证被明确拒绝或旧版本不支持刷新接口时回退密码登录，409 冲突与临时错误不再反复创建会话。修复钟意AI、羊羊中转升级后同步持续 401 并最终触发 `AUTH_SESSION_LIMIT`。 | New API 旧 Cookie、rc.22 登录、已有会话刷新、冲突不重登及整轮同步认证恢复测试，后端相关包测试、全量单元测试、静态检查、构建与生产双站点同步验证 |
| 2026-07-26 | `0.1.210` / 待提交 | 修复 | `CUST-OPS-003` | 发布后核验发现两份仓库生产 Compose 虽保持一致，但数据目录被上游通用模板改为 Docker 命名卷，监听默认值也回退为 `0.0.0.0:8080`，与生产 bind mount 和仅回环暴露约束不符；恢复 `./data`、`./postgres_data`、`./redis_data` bind mount，并将默认监听恢复为 `127.0.0.1:18080`，避免后续开启 Compose 上传时切换生产数据源或扩大网络暴露。 | 双 Compose 字节一致性、命名卷残留检查、Compose 静态解析、生产挂载/镜像 revision/健康检查及远端活动 Compose 对比 |
| 2026-07-25 | `0.1.210` / `0ccedbc0a` | 修复 | `CUST-PROD-004` | 修复上游 `session_id` 与本地 `group_id` 合并后，`batch_image_jobs` 插入语句有 38 个目标列和参数但仅保留 37 个占位符，导致所有批量图片任务创建在 PostgreSQL 上失败；补齐 `$38` 并增加 SQLMock 列/参数绑定回归。 | repository 定向与全包单元测试、golangci-lint、后端构建；GitHub Actions Unit/Integration 与 Security Scan 通过 |
| 2026-07-25 | `0.1.210` / `68f708d11` | 上游适配 | `CUST-GW-005`、`CUST-GW-006`、`CUST-GW-010`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-004`、`CUST-BILL-005`、`CUST-BILL-006`、`CUST-PROD-004`、`CUST-PROD-006`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-UI-004`、`CUST-OPS-003`、`CUST-OPS-005` | 完整合并上游 `60013c5f1..6d956bdc2`，接入 Composite 路由、请求会话 ID、Ollama Cloud 用量、支付宝移动端当面付唤起和 OpenAI 代理流熔断；保留本地逐轮 WS 并发释放与审计、成功会话审计、严格请求模型计费、Composite 显式别名价例外、图片分组结果统计、批量图片字段、充值赠送及专属倍率禁返利、DataTable 非虚拟化和生产资源参数。 | Ent/Wire 重复生成结果稳定；Go 全包编译、全量 unit、golangci-lint、CGO 关闭构建、前端 frozen install/lint/typecheck/全量 Vitest/生产构建、Compose 一致性与源码冲突标记检查通过 |
| 2026-07-25 | `0.1.210` / 待提交 | 新增/移除 | `CUST-PROTO-003`、`CUST-PROTO-008` | 确认分组级 `claude_code_upstream_mimicry` 为已脱离运行链路的历史二开，新增向前 migration 删除数据库列与索引并清理 Ent 字段，保留仍生效的全局 Claude Code 模拟；新增按最终上游模型过滤 Anthropic REST Messages 根级 `temperature`、`top_k`、`top_p` 的网关配置，支持精确 ID 和末尾通配符，并同步用于账号测试。 | 配置读写/校验/缓存测试、清洗器与两类 Messages 构造器测试、账号模型映射测试、前端配置与兼容测试、Ent codegen、Go/前端全量检查 |
| 2026-07-25 | `0.1.210` / 待提交 | 修改 | `CUST-OPS-005` | 生产实例迁移至 4 vCPU、8 GiB 内存主机后，重新评估实例级运行参数：释放 Go 四核并行和更大堆空间，提高用量 worker、HTTP 空闲连接复用及 PostgreSQL 缓存与并行查询能力，同时保持数据库连接总量、计费超时、队列上限和日志轮转边界。具体实例值仅记录在不跟踪的本地运维说明中。 | 新旧主机资源对比、Compose 配置解析、容器实际环境变量、PostgreSQL 实际参数、挂载/健康检查及生产观察 |
| 2026-07-24 | `0.1.210` / 待提交 | 修改 | `CUST-GW-010`、`CUST-BILL-006` | 生产 GC 诊断确认高并发 Responses 请求在长流期间保留多份最大约 52 MB 的请求体，live heap 可增长至约 3.1 GiB 并触发 OOM；改为首个有效输出后释放不再用于故障转移的大对象。usage worker 改为按用户聚合排空，避免热用户的 gate 等待占满全部 worker 并持续产生 5 秒扣费超时，不改变用户入口并发。 | 请求体释放时机/幂等测试、keyed worker 同用户串行/不同用户并行/超时窗口/队列清理测试、Go race/全量测试、生产 30 分钟稳定观察 |
| 2026-07-24 | `0.1.209` / 待提交 | 新增 | `CUST-BILL-006`、`CUST-OPS-005` | 同一用户余额扣费在开启事务前改为进程内串行，避免高并发请求同时占用连接并争抢 `users` 行锁；后台扣费继续隔离请求取消，但不再覆盖 usage worker 的较短 deadline。Compose 新增 worker、PostgreSQL 并行查询与 Docker 日志轮转参数透传，为 2 核小内存生产实例提供可回滚的资源保护。 | gate 并发/取消/事务退出测试、worker deadline 测试、计费幂等集成测试、Go race/全量测试、Compose 一致性与生产健康检查 |
| 2026-07-23 | `0.1.208` / 待提交 | 修改 | `CUST-OPS-003` | 两份生产 Compose 增加 `DASHBOARD_AGGREGATION_RETENTION_USAGE_LOGS_DAYS` 环境变量透传，仓库默认值仍为 90 天，生产实例通过未跟踪的 `.env` 配置为 30 天。修复 `.env` 已设置保留策略但变量未进入应用容器、运行时仍使用代码默认值的问题。 | 两份 Compose 内容一致性、`docker compose config`、生产容器环境变量与健康检查 |
| 2026-07-23 | `0.1.207` / `e957a0a38` | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-006`、`CUST-GW-007`、`CUST-GW-008`、`CUST-GW-009`、`CUST-PROTO-001`、`CUST-PROTO-002`、`CUST-PROTO-005`、`CUST-PROTO-006`、`CUST-ACC-001`、`CUST-ACC-002`、`CUST-ACC-003`、`CUST-ACC-005`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-004`、`CUST-BILL-005`、`CUST-OBS-001`、`CUST-PROD-006`、`CUST-RISK-001`、`CUST-RISK-003`、`CUST-UI-003`、`CUST-UI-004`、`CUST-UI-005`、`CUST-OPS-003`、`CUST-OPS-004` | 完整合并上游 `e625ce3b3..60013c5f1`，接入 Grok compact 与客户端工具回程、OpenAI reasoning effort 分组策略、调度排除原因、hosted image token 计费、Redis ACL、移动端布局和依赖安全更新；保留本地首 Token/body-signal unary compact、逐轮 WS 结算、图片尺寸能力、按请求模型计费、充值赠送、账号列表禁用虚拟化、启动容错及双生产 Compose 约束。 | Go 全量 unit、lint、构建、govulncheck、定向协议/调度/计费回归、前端 lint/typecheck/全量 Vitest/build/audit、Apple fixture、Compose 与生成一致性检查 |
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

1. 网关 handler/service 签名变化：`CUST-GW-001` 至 `CUST-GW-013`；
2. OpenAI Responses、WSv2、namespace、图片工具变化：`CUST-PROTO-001`、`CUST-PROTO-002`、`CUST-PROTO-006`、`CUST-PROTO-007`；
3. Anthropic 请求转换和账号测试变化：`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-008`、`CUST-ACC-005`；
4. 计费和 usage schema 变化：`CUST-BILL-001` 至 `CUST-BILL-005`；
5. account/group/channel/payment schema 变化：`CUST-ACC-001`、`CUST-ACC-006`、`CUST-PROD-005` 至 `CUST-PROD-007`；
6. 前端设置、导航和公共配置变化：`CUST-PROD-001`、`CUST-PROD-002`、`CUST-PROD-007`、`CUST-OBS-002`、`CUST-UI-001` 至 `CUST-UI-005`；
7. release、Compose 或镜像变化：`CUST-OPS-001` 至 `CUST-OPS-004`。

## 已被上游吸收的历史二开

以下能力曾由本地提交引入或扩展，但在当前上游基线中已存在等价实现，本地没有独立行为差异，因此不占用当前功能编号：

| 能力 | 当前结论 | 复核方式 |
| --- | --- | --- |
| API Key 并发统计 | 当前上游已包含等价契约和实现；本地只保留仍有差异的专属分组访问约束，见 `CUST-ACC-003`。 | `git diff upstream/main..main` 不再包含并发统计实现差异 |
| 邀请返利核心流程和订阅返利 | 当前上游已包含等价实现；本地仍有差异的充值赠送、费用展示归入 `CUST-PROD-006` 和 `CUST-BILL-005`。 | `affiliate_service.go`、`payment_fulfillment.go` 相对当前上游无差异 |

如果上游已经完整提供同等能力，应在本文件把对应项记为“上游吸收”变更，确认本地差异和迁移兼容都可删除后，再将状态改为“已移除”；不得仅因代码冲突较多而无记录地放弃本地行为。


### 2026-09-11 上游增量适配

- 时间：2026-09-11T00:49:38+08:00；版本仍为 `0.1.231`，同步范围 `ab99d56e9..98d86915b`。
- 类型：上游适配；编号：`CUST-GW-001`、`CUST-GW-003`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-001`、`CUST-PROTO-005`、`CUST-ACC-001`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-004`、`CUST-BILL-005`、`CUST-OBS-002`、`CUST-PROD-002`、`CUST-PROD-004`、`CUST-UI-002`、`CUST-UI-004`、`CUST-UI-005`、`CUST-OPS-002`、`CUST-OPS-003`、`CUST-OPS-004`。
- 保留全部 55 个稳定编号、功能默认值和生产约束；只有已获批的分组白名单从展示配置升级为请求准入。各项具体适配已更新上方清单。
- 验证：同步前后 22 项 Windows 构建/测试/静态命令最终均为 0；Linux 隔离集成仅有相同的 Windows 挂载权限基线失败，该单项在仓库内 POSIX tmpfs 补测通过；隔离 PG/Redis 初始化及健康检查前后通过。详细命令、重测和逐提交映射见本次同步历史。
- 未验证：真实上游账号/模型、实际插件、支付/回调、浏览器端到端、完整 race、安全漏洞数据库扫描、Apple 专有容器能力及生产环境。未推送、未部署。

### 2026-09-11 CI 安全依赖修复

- 时间：2026-09-11T01:16:14+08:00；版本仍为 `0.1.231`；类型：修复；编号：`CUST-PROD-007`。
- 用户追加授权推送同步分支、通过 CI 后合入远端 main。PR #21 首轮安全扫描发现 Canvas 间接依赖 `js-yaml 4.3.1` 命中高危 `GHSA-2883-xcg3-v3hh`；用 pnpm 定向覆盖至官方补丁 `4.3.2` 并更新锁文件。结构化对比确认其他依赖解析不变，画布功能、默认开关及生产配置不改。
- 修复后 Canvas 格式检查、类型检查、34 个单元测试和构建均退出 0；安全审计高危/严重均为 0，现有审计检查脚本退出 0。原始 JSON 仍列出 6 项中危，pnpm audit 命令退出 1，未隐藏此结果、未增加例外、未降低门禁。
- 修改范围仅 Canvas manifest/锁文件及两份台账；远端最终 CI 与合并结果以 PR 检查及交付总结为准。没有推送版本标签、触发 Release、部署或连接生产服务器。

### 2026-09-15 本地上游同步 bdb42e22f

- 时间：2026-09-15T03:21:15.9841200+08:00；版本保持 `0.1.232`；类型：上游适配；代码与本台账同一 merge 提交，实际代码 SHA 由最后的同步历史提交记录。
- 影响编号：`CUST-GW-001`、`CUST-GW-003`、`CUST-GW-006`、`CUST-GW-008`、`CUST-GW-010`、`CUST-PROTO-002`、`CUST-PROTO-003`、`CUST-PROTO-004`、`CUST-PROTO-006`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-RISK-002`、`CUST-UI-002`、`CUST-UI-003`、`CUST-UI-004`、`CUST-OPS-002`、`CUST-OPS-005`。其余编号未移除或复用，未把纯上游功能新增为二次开发编号。
- 完整合并固定范围的 141 提交，24 个文本冲突逐块组合解决。接入 OpenCode、原生 Codex Images、WS 执行作用域/保活/抢占、新配额卡与订阅开关；保留严格请求模型计费、Free Fast 双成本、Fable 无默认三倍、Grok 严格边界、首 Token 保护、连续失败停调度及插件已发送禁重放。
- 适配中修复选项/函数参数遗漏、重复预览 computed、图片 SSE 错误分支和旧价格夹具；旧图片与签到断言未删除。WS 同线程测试只调整假上游放行顺序，保持请求在飞直到抢占关闭，保留关闭码、原因及恰好一次抢占断言；两种线程关系各重复 50 次并启用 race 检查通过。
- 验证：Go 默认与 unit 全包、全包构建、golangci-lint、同步重点回归、前端 300 文件 2293 用例/类型/lint/构建、Canvas 34 用例/类型/构建、Ent 320 文件无差异和 Wire 通过；Linux 集成的挂载权限基线由仓库内 POSIX tmpfs 补测通过，定向 race、隔离 PG/Redis 启动与健康检查、旧库升级及 238 重入数据断言通过。浏览器检查桌面/移动配额、订阅关闭、兑换刷新失败成功态、签到记录、Canvas 初始化与公开余额凭据隔离。完整命令与重试见同步历史。
- 既有问题与边界：Canvas package.json 格式、Apple BSD stat 夹具及 Windows 挂载 0600 权限表现与同步前相同；同步前 Cyber 异步时序测试曾失败，未改业务代码，重复复测通过。govulncheck 无可达漏洞报告；pnpm audit 原始退出码为 1，前端两项高危沿用有效的既有例外，Canvas 无高危/严重，CI 例外门禁通过，依赖及例外清单均未改。
- 未验证：真实上游账号/模型、实际 transport 插件、支付/回调/短信邮件、完整浏览器业务端到端、全包 race、峰值容量、Apple 专有容器与生产行为。浏览器只使用隔离 mock，图片/视频不实际生成；不据此声称所有真实业务不受影响。未推送、未创建 PR、未访问远程服务器或部署。

### 2026-09-15 追加授权发布 v0.1.233

- 类型：发布验收补记；关联 `CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-005`；没有新增能力编号、修改功能行为、改变资源默认值或降低 CI 门禁。
- 用户在上述本地阶段后明确授权推送、通过 CI 后打 tag 自动部署。候选 `148cd9afa` 的 main 与 tag CI、安全扫描全部通过；Release `34889754841` 的实际部署步骤于 04:07:57 成功，生产镜像 revision 与候选一致。
- 04:10:48 生产配置、挂载、健康及 3 份新增迁移校验通过，数据库/Redis 未重建；公网健康检查通过。3600 秒响应头超时、5 秒用量任务、256 核并行及 512/600 连接基线均保留。
- 旧 HTTP 服务发生强制关停，随后完成全部清理；观察窗口未发现用量任务丢弃告警，不将此视为零中断或账单完整性证明。真实第三方业务和峰值验证边界不变；详细证据见 `docs/operations/2026-09-15-v0.1.233-release.md`。


### 2026-09-15 授权发布 v0.1.234：Claude 透传流收尾修复

- 类型：发布验收补记；关联 `CUST-GW-003`、`CUST-GW-008`、`CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-005`。用户明确授权推送、通过 CI 后更新 tag 自动部署；没有新增业务能力、数据库迁移或生产配置变更。
- 原修复 `66c9ee8a0` 经合并提交 `cfd9afa87` 推送 main；固定候选的 CI `34941090320` 与安全扫描 `34941090352` 全部通过后，新建并推送 annotated tag `v0.1.234`，没有移动旧标签。标签 CI `34942303928`、安全扫描 `34942303892` 与 Release `34942303933` 均成功。
- 修复只处理完整终止事件已经转发、上游却不关闭响应体导致的误报超时；没有终止事件的真实断流和超时仍报错，不补造 `message_stop`。本地定向 unit、race 回归和 12 项部署门禁通过。
- 15:48:34（UTC+8）实际远端部署步骤成功；15:49:42 生产验收确认镜像 `saviour2411/sub2api:0.1.234`、revision 与候选一致，健康、配置、挂载及资源比较通过，PostgreSQL/Redis 容器未重建。自动版本回写提交 `742cdd32c` 已同步到本地。
- 旧容器 15:48:23 出现 HTTP 强制关停与一条用量丢弃告警，原因 `stopped`，当时累计 `dropped_pool_stopped=1`、`dropped_queue_full=0`，15:48:26 完成清理。告警有限频，不能据单条日志确认精确受影响请求数或账单结果；未执行历史账单补写、重算或回滚。生产机访问公网域名健康检查返回 200；本地出口访问该域名返回 403，未据此宣称所有外部路径正常。详情见 `docs/operations/2026-09-15-v0.1.234-release.md`。

### 2026-09-15 Claude 透传流空闲计时与示例一致性修复

- 类型：本地修复；关联 `CUST-GW-003`、`CUST-OPS-005`；版本保持 `0.1.234`，不新增能力编号。
- 用户授权分析近期生产等待情况后修改并提交超时逻辑。只读查询分组41在北京时间2026-09-13至2026-09-15三个完整自然日的1,248,609条流式用量记录；83.95%转发总耗时不超过60秒，但有输出且无可匹配失败记录的样本仍有41,945条超过180秒。现有数据没有逐帧间隔，部分日期错误日志已不保留，不能据总耗时证明降低空闲阈值到60/90/120秒安全。
- 保留后端180秒默认，将Anthropic API Key透传的整周期Ticker改成按最后一行上游数据的剩余空闲时间重新设置Timer，避免180秒空闲阈值因检查粒度额外等待近180秒；示例从300秒统一为180秒。上游心跳、注释、空行继续沿用既有活跃判定，不新增语义停顿或请求总时长限制。
- 保留完整终止事件立即结束、缺终止事件仍失败、客户端断开后排空及部分用量；首Token分组策略、同账号重试/换号、账号停调度和其他协议路径不变。生产3600秒响应头超时、5秒usage任务、资源与持久化目录不变。
- 新增虚拟时钟边界回归和示例/后端默认一致性回归，旧实现先确认失败、修复后通过；新增超时回归重复100次、config/service/handler三个完整unit包及CGO关闭的后端构建通过。本次静态差异检查0告警；全量unit标签lint仍报告58项位于未改文件的问题，未修改规则。Windows缺少GCC导致race编译失败，竞态检测未验证。统计口径、证据、影响范围及完整验证记录见 `docs/operations/2026-09-15-claude-stream-idle-timeout.md`。
- 本次没有推送、发布、部署、重启或修改生产配置，也没有执行真实上游推理请求；当前生产仍运行旧计时逻辑。

### 2026-09-15 Claude 透传流安全重试、诊断与页签调整

- 类型：新增与修改；新增 `CUST-GW-013`，关联 `CUST-GW-001`、`CUST-GW-003`、`CUST-GW-012`、`CUST-UI-001`；基于本地 `93fd4a438`，版本保持 `0.1.234`。
- 用户确认只覆盖 Claude API Key 透传、默认关闭，额外重试2次／总预算300秒，从首次成功流响应头开始，先原账号再换号，无替代账号时有限同号兜底。独立配置持久化到 settings，复用原运行时缓存失效机制；不新增数据库迁移。
- 新增完整 SSE 事件分类、256 KiB 前导缓存、请求级重试计数与可解除的等待预算。上游心跳不延长总预算；后续响应头、选号、排队和退避也计入。内容提交、客户端取消、不可重放标记、协议故障及缓存超限均禁止再生成，完整终止立即收尾。
- 不把真实 HTTP 200 流故障伪造为账号 HTTP 状态错误；保留既有首 Token 与流超时策略。被丢弃的尝试仅留诊断，不重复向客户计费；最终失败采用最近有计量尝试及其账号的一次用量快照。实际 provider 可能对失败尝试收费，本功能不保证上游无重复消耗。
- Ops 事件保存独立的流诊断摘要并在错误详情展示，明确真实 HTTP／逻辑状态、最后事件、终止完整性、是否交付、缓存和剩余预算、重试决定；不记录新正文。上游管理固定为首个页签。
- 验证与限制：四个相关后端完整包通过，新增服务层回归重复100次通过；前端5个文件27项测试、类型检查、前后端构建及变更级只读 lint 通过。本机缺 GCC／CGO 工具链，竞态检测未执行。详见 `docs/operations/2026-09-15-claude-stream-safe-retry.md`。本次只使用本地夹具，不访问生产或真实上游；未推送、发布、部署或开启生产功能。
- 后续交付授权：用户要求推送 Git、通过 CI 并确认合入主线；采用独立分支与 PR，补充 Linux 定向 `-race` 检查及配置 API／诊断／错误详情前端关键测试。不修改生产开关，不打标签、发布或部署；CI 最终结果以相应提交的 Actions 记录为准。
- CI 安全修复：PR #22 的分支安全扫描识别现有 `google.golang.org/grpc v1.82.1` 命中 `GO-2026-6443` 与 `GO-2026-6348`。核对 Go 官方漏洞库后定向升级到同时覆盖两项修复的最小版本 `v1.83.2`，仅接受该版本要求的间接依赖最低版本更新，保留原 Go 版本和安全门禁，不使用扫描豁免。该依赖供插件运行时使用，业务功能和生产配置不变。

### 2026-09-18 本地上游同步 efe9aab1e

- 类型：上游兼容适配；固定范围 bdb42e22f..efe9aab1e，共108个上游提交，完整合并，不挑选或遗漏前置提交。保留本地版本0.1.239；不改生产版本、资源、配置、数据挂载或迁移历史。
- 关联编号：CUST-GW-006、CUST-GW-007、CUST-PROTO-006、CUST-ACC-001、CUST-ACC-004、CUST-OBS-001、CUST-PROD-005、CUST-PROD-006、CUST-UI-004、CUST-UI-005、CUST-OPS-004；各清单项已同步补记处理边界。本轮不新增二开能力编号，现有58个编号保持稳定。
- 已批准的十文件冲突逐块处理：保留两套独立限流CAS、多次兑换明细、连续失败保护、充值赠送角标、MiniMax监控断言、认证加载容错与表格列设置；注入Ollama探测并重新生成Wire，继续保留生命周期与customFeatureHandler。注册确认密码控件补齐本地禁用状态引用，不改变输入与提交的既有不同限制。
- 回归新增：Kimi原生Chat和Responses转Chat各覆盖流式及非流式，验证developer角色转换与400采样修正共存、原始请求体不被修改、成功用量与失败诊断正确；兑换仓储覆盖同码多人使用后的用户分页、总数、归属和使用时间。注册设置尚未加载时确认密码输入与切换控件也保持禁用。
- 测试机械适配：新增Gemini构造与SSE用例补齐本地SettingService和Account参数；105条兑换历史夹具通过既有仓储写入独立使用表，保留20/50/100分页、排序和用户隔离断言；签到奖励响应夹具改为分页对象；Ollama旧回调用例显式改变重置代次，避免Windows时钟精度偶发失败，100轮复测通过。没有删除断言、改变限流CAS或放宽兑换查询。
- 刻意保留：Claude流终止清理、安全重试及首有效内容计时、Kimi四项默认关闭开关、首Token分组策略、失败与人工暂停保护、按用户串行扣费和5秒usage超时、自动授信、用户定制、既有模型/价格/路由覆盖，以及蓝绿生命周期和强制退役的明确风险边界。
- 验证结果、全部实际命令和退出码集中写入本轮同步历史，失败复测不覆盖原始证据。同步前已记录Canvas格式问题、前端依赖告警和macOS专用验证限制；Windows/WSL临时目录与Go 1.27 GOTMPDIR问题通过仓库内隔离POSIX测试目录处理，不修改业务断言。
- 本次只授权本地合并、提交和隔离验证；未推送、打标签、创建PR、访问服务器或部署。真实Provider、支付回调、生产行为、macOS原生功能与峰值负载不以本地测试代替验收。

### 2026-09-18 上游同步发布 v0.1.240

- 类型：既有上游适配的发布验收，不新增、停用或改变二开行为；上条列出的关联编号及现有58个编号均保留。用户在本地同步后追加授权推送、CI通过后合入主线、tag及Actions自动蓝绿部署。
- 发布候选`ef187f2ec9cd9615a3c803dffd20fc3188e33785`；候选、main、tag的7项CI与2项安全扫描均成功。v0.1.240固定指向候选，Release `35268007982`五项任务全部成功，VERSION由Actions回写为0.1.240。
- `CUST-OPS-001`、`CUST-OPS-003`沿用既有无迁移蓝绿门禁及一小时强制退役策略。04:18:38切流green/18082，05:19:47恢复stable且pending=null；09:21最终只读核验通过，批准资源值、3600秒响应头等待、5秒usage超时、数据挂载与PG/Redis容器保持不变。
- 旧blue到期仍有78个会话租约，强制退出137，非OOM；可见用量丢弃0不代表账单完整，clean_exit=false、usage_loss_unknown=true，可能中断旧请求/续接。本次没有修改退役策略、重算账单或以探针零错误声称业务无损。
- 本轮连续健康探针共7018次、异常0，只覆盖API/direct健康路径。真实付费模型、支付/回调、完整账单、峰值负载及所有二开浏览器端到端仍未验证。详细运行ID、镜像、退役证据与配置核验见`docs/operations/2026-09-18-v0.1.240-release.md`。

### 2026-09-19 Claude 安全重试提前 JSON 保活

- 新增 `CUST-GW-014`，关联 `CUST-GW-013`。用户优先要求尽早响应 New API，同时保留空流和仅前导异常结束时的重试保护；用户再次明确要求原有首字统计保持不变。本次不修改 `first_token_ms` 的计算、入库或页面展示，也不回算历史数据。
- 新增 `anthropic_stream_safe_retry_early_keepalive_enabled`，默认关闭，仅安全重试总开关开启且命中 Claude API Key 透传流时生效。管理页位于“网关配置 / Claude 透传流安全重试 / 提前发送 JSON 保活”。旧客户端省略字段时保留现值，旧存储缺字段时关闭，复用既有缓存失效与请求级设置快照。
- 首次成功流响应进入处理后发送 `event: ping` 和 `data: {"type":"ping"}`，不发送失败尝试的 `message_start` 或账号相关响应头。流读取期间按既有保活间隔继续发送，未配置间隔时使用10秒；读取中的上游半帧仍在本地缓存，不能与下游保活交错。此版本不新增跨选号、排队及重试响应头等待的后台写协程，这些阶段仍受原有共享等待预算约束。既有 HTTP 传输层可能先等待 gzip/zstd 压缩头才能交回响应，本次没有改动该共用解压逻辑，不保证在压缩头也未到达时提前发包。
- 已写 HTTP 200 不代表已交付内容：前导 EOF、空流和原有可重试超时仍按原策略有限重试，成功只交付一份开头和一次用量。文本、思考、工具调用和未知不可撤销事件提交后不重放；客户端写失败停止重试，保活不重置任何上游超时或总预算。
- 提前保活后无法再修改 HTTP 状态；最终错误必须通过 SSE 传递并进入逻辑失败统计。补齐重试后 HTTP 400 等非重试错误的处理，保留真实上游结果归因，禁止把普通 JSON 拼入已开始的事件流。诊断新增 `early_keepalive_sent`，与 `output_committed` 分开记录。
- 行为边界：提前保活改善首包到达，并可触发 New API 的首响应统计，不保证 Claude CLI 更早显示正文或改变等待状态；失败尝试仍可能消耗上游额度。新开关和原总开关均不会自动在生产启用。本次不修改生产资源、3600秒响应头配置、5秒 usage 超时、镜像或数据挂载。
- 验证结果：后端 service、handler、admin 三个完整 unit 包通过；Linux Go 1.27 定向 `-race -count=3` 通过。回归覆盖原首字仍为3000ms而正文第59秒到达、空流恢复、内容后禁止重放、各类超时不续期、取消和写失败、半帧隔离、HTTP错误流内终止、配置兼容；真实 HTTP/1.1、HTTP/2 各验证 identity/gzip/zstd 恢复，共6组，首轮无压缩响应在任何上游事件前已收到 JSON ping。
- 前端5个文件33项测试、类型检查、变更级只读 ESLint、前后端构建和 `git diff --check` 通过；构建仍有 Browserslist 数据陈旧和现有分包体积等提示。Playwright 使用本地接口夹具验证开关禁用关系、保存后刷新保留，以及1440×1000和390×844布局无横向溢出或开关重叠，截图位于 `output/playwright/early-keepalive-desktop.png` 和 `output/playwright/early-keepalive-mobile.png`。
- 验证限制：未运行真实 New API/Claude CLI 端到端或生产流量验证；实现阶段未访问生产、推送、提交、打标签或部署。
- 后续交付授权：用户要求提交代码、推送主线并通过 CI；保持原有首字统计及开关默认值，不打标签、发布或部署，也不修改生产开关。主线检查结果以本次提交对应的 GitHub Actions 记录为准。
