# 二次开发功能台账

## 2026-10-29 OpenAI 流传输错误恢复修复

关联 `CUST-GW-008`。上游以裸 SSE `error` 返回 `upstream_http2_stream_error`、`upstream_stream_read_error` 或 `upstream_stream_truncated` 时，首内容输出前进入既有有界重试与切号流程，兼容顶层和嵌套错误码。已输出内容的请求继续返回失败，不重放；未知错误和策略拒绝不因消息提到 HTTP/2 而扩大重试范围。

重试耗尽的错误体及补发的 `response.failed` 保留具体传输错误码，监控按上游故障归类，避免裸 error 回落为本站 internal。回归覆盖 Responses、透传、Chat Completions、Messages 的首内容前恢复与输出后不重放，以及非流式聚合和错误日志归因。本次修复上游错误的处理，不改变 HTTP/2 协议配置或现有重试预算。

同轮安全检查发现 `GHSA-83rm-6c8r-g728`：Vue I18n 9.14.5 的 `escapeParameter` 不能防止译文 `srcdoc` 注入。9.x 无修复版本，升级 Vue I18n 与消息编译器至已包含修复的 11.4.16，沿用项目已使用的 Composition API。依赖要求 Node.js 22 及以上，CI、安全扫描及发布前端构建统一为与 Docker 构建一致的 Node.js 24。

## 2026-10-26 v0.1.260发布兼容适配

- 用户在PR #45通过CI合入后要求更新tag并自动蓝绿部署。生产仍为v0.1.259，本次同时发布已合入的上游同步和渠道/降智页面优化；关联既有`CUST-OPS-007`、`CUST-OPS-001`、`CUST-OBS-001`、`CUST-OBS-003`，不新增功能编号。
- 增加`gemini-scoped-246-247-v1`，仅绑定生产提交`52c45a51b5f221728c3b7d3d39b83c9f86c81c22`及246生图开关、247管理密钥两条原始SQL。旧Gemini入口不使用该开关拒绝生图，分组显式列名读写；旧用户删除使用软删除，兼容新增创建者外键。
- 两条原文与账本在一个事务中执行，保持PostgreSQL18、16MiB既有表、NOWAIT及1/5/10秒预算；普通查询可继续。拒绝同名对象、部分账本、未知分组/账本触发器、DDL触发器和结构漂移。失败完整回滚，候选不切流；重复启动核验新表完整定义，不重新回填已由管理员关闭的开关。
- 回退保留新表、密钥记录和已迁移的开关值，旧版不提供新管理密钥功能。生产只读预检、验证及发布边界见`docs/operations/2026-10-26-v0.1.260-release.md`；普通迁移规则、历史SQL及生产资源配置不变。

## 2026-10-15 v0.1.256降智检测发布适配

- 用户在PR #38通过CI合入后要求更新tag并自动部署。本轮关联`CUST-OBS-003`、`CUST-OPS-007`、`CUST-OPS-001`，能力编号保持65项。
- 新增`model-quality-additive-246-v1`，只绑定生产v0.1.255及原始`246_model_quality.sql`，新增五张检测专用表及默认关闭的设置项；旧版按键读写设置且不访问新表。历史SQL和普通字段门禁保持。
- PostgreSQL18、至多16MiB的设置/账本普通表、无附加触发器/RLS/规则；NOWAIT设置写入锁及1/5/10秒预算。拒绝已有检测对象/开关/账本和DDL触发器，原SQL及账本同事务提交；完整结构摘要覆盖字段、索引、约束、外键、序列和附加行为，重复启动只读重验。
- 版本更新为0.1.256。生产基线、测试、迁移与回退边界见`docs/operations/2026-10-15-v0.1.256-release.md`；发布仍通过固定主线SHA的完整CI、安全扫描及tag Release自动执行，不自动开启检测或消耗测试Key。

## 2026-10-14 v0.1.255发布兼容适配

- 用户在PR #36通过CI合入后要求更新tag并自动蓝绿部署。关联既有`CUST-OPS-007`、`CUST-OPS-001`，能力编号仍为64项，不扩展上游同步范围。
- 增加`upstream216-additive-v1`固定策略，绑定生产v0.1.254、四条242至245上游SQL、分组Ent及旧读写实现摘要。只支持PostgreSQL18与至多16MiB的分组普通表，沿用固定触发器审查、NOWAIT和1/5/10秒预算。
- 四条迁移和账本同事务提交；任何失败整体回滚，不切流或自动重启候选。初次拒绝同名结构与已有/部分账本；重启检查完整账本与实际字段、索引及约束，确认分组默认值扩展未重写表。普通可空字段门禁和历史SQL保持。
- 旧版分组按列名读写，新译文不被旧更新覆盖；新缓存表不被旧版访问。不改余额/价格/生产配置，不增加共存期业务限制。应用回退保留结构和数据，旧版不提供新增缓存管理与译文界面。
- 原同步台账记录的是当时未批准部署的本地阶段；本次生产发布要求及兼容补丁另记于`docs/operations/2026-10-14-v0.1.255-release.md`，不改写此前验收事实。验证与执行结果在该发布记录补充。

本文档是本仓库相对上游 `Wei-Shaw/sub2api` 的二次开发功能事实来源，用于回答以下问题：

- 当前有哪些仍需独立维护的二次开发能力；
- 每项能力的行为边界、默认状态和主要代码入口是什么；
- 上游同步、功能修改、停用或删除时，哪些本地行为必须保留或重新评估；
- 某项二次开发能力在何时、因何发生过变化，以及使用什么方式验证。

上游提交的逐项处置继续记录在 `docs/upstream-sync-history.md`。本文件不重复上游提交清单，只记录本地能力及其演进。

## Kimi 参数兼容重试（思考模式类型兼容计划随 v0.1.244 发布）

提供五个独立网关开关，默认关闭，分别兼容上游明确拒绝采样参数、非法思考强度、非法工具选择、思考预算冲突和非法 `thinking.type`。仅 API Key 所属类型为 `kimi` 的分组及 Chat Completions 出站生效，覆盖 Messages/Responses 转换，不影响综合分组和原生其他协议。

只匹配实际 HTTP 400 的结构化明确错误，并核对出站字段和值；泛化400、引用文案、歧义JSON、额度/审核错误不匹配。每项每请求最多一次，共最多额外五次；普通重试和切号保留修正，既有 pool 次数不变。已提交响应禁止重放，中间失败不泄露到下游。思考兼容开启时，K3 的 Messages 默认强度改为 `low`；现有 Kimi 显式 `max` 保留行为不变。

新增的思考模式类型开关 `kimi_thinking_type_retry_enabled` 只匹配已复现的 enabled/disabled/auto 枚举报错并核对实际出站非法值，剥离整个 `thinking` 后重试一次；合法值、缺失字段、歧义JSON及错误路径不符均不触发。剥离会移除嵌套预算和历史思考保留设置，独立的 `reasoning_effort`、`thinking_budget` 和输出上限保持不变。

通过共享 CC 入口管理响应关闭、取消和首 Token 资源，使用现有上游尝试链与 Recovered 展示。删除 `max_completion_tokens` 会解除该输出上限，可能增加输出和费用，须由管理员主动启用。配置、匹配、三个入口、限次、流安全和恢复日志均有回归，Kimi 用例加入流竞态 CI；本地测试不能替代生产第二次实际派发与完整恢复验收。详细说明见 `docs/kimi-parameter-compatibility.md`。用户于2026-09-22追加授权提交代码，待 CI 与安全扫描通过后创建新标签并经 GitHub Actions 蓝绿部署；本次不自动启用线上开关。

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

- 当前生产核验（2026-09-24 11:44:11）：`0.1.246`、revision `94dac3dcb880dcf96e3677827d3b4af7a55b9c98`在green/18082接流，状态stable、pending=null；239/240已执行且并行期倍率保护在旧槽退役后解除，原配置、资源与数据挂载不变。旧blue一小时后强退，退出137且非OOM、用量收尾完整性未知，详情见`docs/operations/2026-09-24-v0.1.246-release.md`。
- 历史生产核验（2026-09-21 22:25:37）：`0.1.243`、revision `6154c64ffc83f180f7e8f118ccbd5182b8cb0f52` 在blue/18080接流，状态stable、pending=null；兼容字段迁移已完成，生产保护项不变。旧green按一小时策略强退，退出137且非OOM、用量收尾完整性未知，详情见 `docs/operations/2026-09-21-v0.1.243-release.md`。
- 历史生产核验（2026-09-16）：`0.1.237`、revision `004cc47a2090a35b7cc94a6463e97273b86562fb` 在 blue/18080 接收新流量；当时v0.1.234已正常退役，v0.1.235 green按真实工作自然排空。资源上限、PG/Redis和持久挂载保持不变。详情见 `docs/operations/2026-09-16-legacy-retire-gate.md`。
- 上一版生产记录（2026-09-15）：v0.1.234 原地部署时观察到 HTTP 强制关停及 `usage_record.task_dropped`（`reason=stopped`）告警；本次生命周期与蓝绿修正的目标即消除计划发布中的这类问题。历史证据保留于 `docs/operations/2026-09-15-v0.1.234-release.md`。

- 本地代码基线日期：2026-09-24（北京时间，上游同步与后续获批发布；生产事实以上述独立验收为准）
- 本地版本：`0.1.248`（本轮同步继续保留二开版本；上游来源版本单独记录为 `0.2.13`）
- 本地同步前代码基线提交：`fb0319dc793fdd3049e0f1afb9367d316c90ed0d`；本轮合并提交与验证结果见 `docs/upstream-sync-history.md` 的 2026-10-02 记录
- 已完整集成的上游提交：`b8dece9000c68815a5b867ca5a1e6f236e173905`
- 本次同步范围：`a60a29549..b8dece9000`，62个上游提交（43个非合并、19个合并）；63项既有能力族编号全部保留，无跳过或延期提交；本地覆盖、实际验证和未覆盖范围见本轮同步记录
- 历史识别统计（2026-09-02，`5097b3145..35c28e324`）：919 个差异文件，新增130491行、删除5976行、本地独有512个提交；此为历史快照，不代表本次同步后的统计
- 2026-09-05 历史代码基线差异（`ab99d56e9..289a00a46`）：926个文件，新增130999行、删除6037行；本地独有517个提交（不含最后记录补记）
- 当前能力族：66 项，分布在 9 个功能域；2026-09-20核对59项已有清单（含`CUST-GW-014`）并新增`CUST-OPS-006`，2026-09-21新增`CUST-OPS-007`，2026-09-22新增`CUST-GW-015`，2026-09-23新增`CUST-OPS-008`，2026-10-09新增`CUST-GW-016`，2026-10-15新增`CUST-OBS-003`，2026-10-18新增`CUST-UI-006`；全部编号保留，不复用或删除

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
| `CUST-GW-001` | 首 Token 超时与请求结果归因 | 在首个有效下游语义输出前执行可配置超时，metadata、preamble 和 keepalive 均不解除守卫；compact fallback 内部信号会撤销首次尝试暂存的响应，避免第一次失败提交空 HTTP 200，二次 fallback 失败返回真实错误；区分 pending/final outcome、部分响应、客户端断开和流内失败，避免错误重试或漏结算。WS 透传按轮等待合法终态，过早 close 不误判成功。HTTP client 从真正发起上游请求时开始计时，非流式 CC 不创建守卫。指定分组模式按 `CUST-GW-012` 忽略前导纯空白，保留原始空白直到有效输出一起交付；全局模式保持原有首输出语义。 本次上游适配：本次接入 pending-turn 与取消归因时仍保留逐轮语义输出、合法终态和一次结算规则。 2026-09-15 上游适配：接入原生 Codex Images 和 Gemini 带内错误登记时保留首语义输出、attempt 完结及部分响应归因。 2026-09-20 上游适配：共用 CC 出站增加 DeepSeek 历史推理占位时保留本地三返回值、firstTokenAttempt 生命周期及 Kimi 参数重试，不改变首输出计时或已提交响应禁止重放边界。 2026-09-24 上游适配：Gemini 传输错误切号前仍完结首 Token attempt；终态提前收尾、部分 usage 与已输出禁止重放规则保留。 2026-09-30 上游适配：Anthropic 转 CC 接入实际 usage 事件与缓存归一化，保留 firstTokenAttempt 的包装、完结和取消；客户端取消归因为499。 2026-10-09 上游适配：Anthropic 转 CC 的注释心跳沿用首语义输出守卫，Flush 不得提前提交暂存响应或解除超时。 2026-10-13 上游适配：Gemini 缓存绑定接入后仍完结 pending outcomes；Responses 结构错误回退保留首输出 attempt 收尾及插件已发送错误传播。 2026-10-16 上游适配：Gemini 三个入口同时保留首 Token attempt 与每响应一次错误策略检查；思考摘要接入仍沿用首语义输出及取消收尾。Chat Completions 删除读取前必被覆盖的策略清空赋值，另两条缓存兼容路径保留每轮重置。 2026-10-25 上游适配：兼容流排空与异常终态处理继续经过本地首 Token 尝试包装及收尾。 兼容流终止帧结束等待但不计首 Token；已超时、取消或前奏溢出仍由原尝试收尾。 2026-10-30 上游适配：OAuth 非流式聚合及 Gemini 重试适配继续保留首语义输出、attempt 收尾和客户端取消归因。 | `backend/internal/service/first_token_timeout.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/handler/openai_upstream_outcome_tracker.go` | 生效中 |
| `CUST-GW-002` | 2xx 语义错误识别 | 按 contains/regex 规则检查有限长度的成功响应体，将命中的业务错误转为 failover、账号测试失败和可审计错误。 2026-10-25 上游适配：公共 SSE 错误转换保留未输出 overloaded_error 的 529 归因和限流副作用。 | `backend/internal/service/semantic_error.go`、`backend/internal/service/semantic_error_config.go` | 生效中 |
| `CUST-GW-003` | 流式心跳与长请求保活 | 支持响应前心跳、普通 Responses 与 compact 的 SSE 心跳（含透传首输出前保活）和 Images JSON keepalive；首 Token 计时期间按路径暂停会提前提交状态的心跳，写出判定扣除非语义 keepalive 字节，客户端断开后可继续排空上游完成计费。 本次上游适配：HTTP/2 PING 保活纳入上游连接配置；断连排空仍与首输出守卫协同，不恢复已断开的下游写入。 2026-09-15 上游适配：WS 常驻读循环应答 ping；原生图片流仍按交付/断连状态处理，保活不代替语义输出。 2026-09-15 本地修复：Anthropic API Key 透传改为按最后一行上游数据的剩余空闲时间精确计时，消除180秒阈值可能拖到近360秒才检查的偏差；上游心跳/注释/空行继续续期，下游保活不续期，完整终止与断连排空语义不变。安全重试按 `CUST-GW-013` 独立启用；终止判定进一步要求完整事件分隔空行，事件名称行或截断终止不视为成功。 2026-09-20 上游适配：OpenAI HTTP/2 探测及应答期限恢复为15秒/15秒，其他长流参数保留；Gemini 原生流对 Go/Python GenAI 客户端不发送 SSE 注释，普通客户端保活及本地首输出/断连处理保留。 2026-09-30 上游适配：Antigravity 兼容流默认不启用上游新增的15秒首内容前保活和固定两分钟截止，保留首内容前未提交响应的安全换号；空流及 signature-only 修复合入，内容后的保活不变。 2026-10-09 上游适配：接入 Anthropic 转 CC 的 SSE 注释心跳，保留首 Token 期间的缓冲与已提交输出禁止重放边界。 2026-10-13 上游适配：重试耗尽入口传递上游模型时，提前保活仍不误标真实响应已提交。 2026-10-16 上游适配：Responses 增加 created_at 时保留透传写锁；原始透传事件不补写字段，只为合成失败终态学习时间。 2026-10-30 上游适配：图片按张开关接入后保留交付状态和断连排空；流式断连不得以默认张数补记未交付图片。 | `backend/internal/service/gateway_anthropic_passthrough.go`、`backend/internal/service/openai_compact_sse_keepalive.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_images_json_keepalive.go`、`backend/internal/service/gateway_upstream_response.go` | 生效中 |
| `CUST-GW-004` | Pool 模式重试与失败策略配置 | 在二开管理页配置默认重试次数、可重试状态码、首 Token 阈值与生效分组、上游错误阈值和自动探测退避；Claude 在首个语义输出前收到 SSE `overloaded_error` 时按 529 进入同账号重试或换号；使用策略 revision/fingerprint 避免运行时读到撕裂配置。首 Token 超时继续跳过当前账号池内重试，换号次数沿用原配置，不新增三账号上限。 2026-10-13 上游适配：调度投影并入显式缓存资格字段，保留 pool_mode、pool_mode_retry_count；创建账号重置仍继承全局 Pool 默认。 2026-10-16 上游适配：Vertex 共享容量 429 同号退避及服务账号失效换号纳入；错误策略每响应只检查一次，重试时重置检查状态，已有池模式和自定义策略保持。 2026-10-25 上游适配：公共 SSE 错误保持账号 Pool 重试资格与自定义次数；service_account 仅扩展配额，不扩展 Pool。 2026-10-30 上游适配：接入 Gemini 同账号重试配置及绑定缓存的池模式重试，保留本地失败策略和缓存绑定不换号规则。 | `backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-005` | 自动托管账号恢复探测 | 为可恢复停调度账号创建或恢复唯一的自动测试计划，支持二开配置的阶梯退避、默认提示词和成功后停用。故障期间（含同时人工暂停）普通计划只推进下次 Cron，不发送重复探测、不伪造执行记录、不提前刷新系统计划的退避时间；同实例按账号互斥执行，并复核旧到期快照，重复启用通知保留已有退避。退避计数与时间在账号行锁下原子持久化到现有 extra 的 `auto_managed_probe_state`，不受测试结果保留数限制，不复制到新账号；旧执行不能覆盖新事故或人工修改后的计划。指定分组首 Token 事故使用快照中的模型和阈值，要求有效首输出达速且最终成功；首输出后不限制总流时长，探测不写业务 streak，旧结果不能恢复新事故或人工暂停。 2026-10-25 上游适配：人工调度开关与事故清理复用受限管理密钥外层事务，测活计划及 outbox 随事务一起提交或回滚。 | `backend/internal/service/admin_scheduled.go`、`backend/internal/service/scheduled_test_runner_service.go`、`backend/internal/service/account_test_first_token_recovery.go`、`backend/internal/repository/scheduled_test_repo.go`、`backend/internal/repository/scheduled_test_retry.go`、`backend/migrations/176_gateway_auto_managed_plan_uniqueness.sql` | 生效中 |
| `CUST-GW-006` | 连续失败停调度保护 | 记录账号错误 streak，达到阈值后临时移出调度；普通请求成功、人工清错和明确的状态恢复可清理对应状态。指定分组首 Token 策略只允许受保护请求改变该类 streak，其他分组、非流式或 WS 结果不清零；停调度仍为账号全局生效，未改为分组局部禁用。管理员调度开关通过现有 extra 的 `manual_scheduling_paused` 与系统故障停调度区分，单账号暂停同时停用系统测活；后台成功恢复及条件清除事故不得覆盖该标记，批量调度开关同样写入标记。OpenAI Team 联动熔断先执行并去重，再与本地 strict failure scheduling、自动测活和调度 outbox 协同；自动用卡不视为通用测试成功，不清除 error、overload、临时停调度、模型限流、failure marker/streak、人工停调度或自动测活事故。任一 WS 轮次失败并触发停调度后，同一会话的后续轮次被阻断。失败即释放会话槽；固定账号模型清单的只读资格检查不代替真实推理的停调度门禁。 2026-09-15 上游适配：WS 新执行作用域、先关闭再取消与本地逐轮失败停调度共同执行，不清除人工暂停或绕过下一轮准入。 2026-09-18 上游适配：Ollama 429 用量探测独立注入限流服务，与本地连续失败缓存及自动探测字段共存；不解除人工暂停，也不以旧探测结果覆盖新事故。 2026-10-13 上游适配：接入临时停调度作用范围和模型级 401 规则，保留账号连续失败策略选择及既有恢复行为。 2026-10-25 上游适配：受限恢复提交成功后才重置连续失败计数；Redis 失败保留事故和运行态阻断，成功后按事故 ID 清理并复核并发新事故。 | `backend/internal/service/account_failure_streak.go`、`backend/internal/repository/account_failure_streak_cache.go`、`backend/internal/repository/account_manual_scheduling.go`、`backend/internal/service/failure_scheduling_compat.go`、`backend/internal/service/ratelimit_service.go` | 生效中 |
| `CUST-GW-007` | OpenAI 高级调度器 | 支持 sticky weighted、订阅优先、Top-K 和 priority/load/queue/error-rate/TTFT/reset/quota/session 等权重；另有优先最近重置策略。 2026-09-18 上游适配：接入 Codex 规范窗口配额读取、选号耗时及粘性命中计数修复，继续保留本地调度与故障恢复约束。 2026-09-24 上游适配：合入普通选号 previous_response 路由与决策标签补齐，本地高级调度、按请求兼容性筛选和释放规则保留。 2026-10-25 上游适配：接入上游快照解码缓存；返回请求前深复制 JSON 可变字段、集合及时间指针，避免本地原地更新污染共享母本。 | `backend/internal/service/openai_account_scheduler.go`、`backend/internal/service/setting_parse.go` | 生效中 |
| `CUST-GW-008` | 增强故障转移与响应结算 | 覆盖非 JSON 2xx、SSE `event:error`、流式/非流式 Responses `error`/`response.failed` 与传输错误、请求级容量错误、图片服务错误、流式部分结果和已提交响应；仅在安全重建上下文时允许 WS 后续轮次换号，复用错误体并避免重复写帧，客户端正常关闭不计为账号故障。WS 每轮重新取得请求模型和渠道映射、记录 payload hash 并独立结算 usage；失败停调度后阻断后续轮。OAuth transport 插件一旦报告 `RequestSent=true`，HTTP、Images、Live 及流式路径均禁止换号、同号重试、compact retry 或响应体读取错误重放；已收到合法终态时，迟到的 close error 不覆盖成功。 接入不可用 continuation、历史委派和心跳初始化恢复。图片仍强制将真实上游 URL 转为内联 data URL/b64；新可选回填开关默认关闭，关闭它不关闭既有 URL 隐藏。两条图片下载链均使用下载专属公网防护，强制内联链仍按原失败/交付规则处理，不改变业务 HTTP/私网 upstream 开关。WS Cyber 失败终态返回错误并保留部分 usage，不伪装成成功。 本次上游适配：WS 每轮白名单、请求定价、Cyber 和停调度校验继续生效；通过策略后只占槽一次。终态成功写入后才触发结算，断连排空同时保留部分 usage 和失败归因；Live 新增账号参数不放宽 RequestSent 禁重放。 2026-09-15 上游适配：新增原生 Codex Images 分支并保留 ImageDelivered、ClientDisconnect、部分 usage 与端点元数据；RequestSent 禁重放及普通图片 URL 内联防护不变。 2026-09-24 上游适配：合入流读取错误分类与脱敏，但继续发送单一 response.failed 合法终态；错误码保存在 response.error，部分用量及响应提交标记保留。 2026-09-30 上游适配：WS 综合别名路由与账号所有权检查合入；失败结果已有逐轮用量时 Cyber 仅写审计，缺少结果时才兜底记账，防止白名单继续会话时重复用量。 2026-10-13 上游适配：缓存引用固定绑定账号且失败不换号，仍完成终态归因；Responses 新回退限完整明确的400错误、未提交输出且未取消。 2026-10-16 上游适配：合成 Responses 失败终态补齐时间及序号，同时保留后到成功终态优先、实际用量结算和已输出禁止重放。 2026-10-25 上游适配：合入兼容流断连排空和异常终态部分用量结算，成功与错误路径共用一次用量提交；保留已输出禁止重放。 已提交兼容流终止错误时不再追加通用错误帧。 2026-10-30 候选验证补充：透传响应按上游和客户端两套流式标记共同分支，非流式请求聚合 JSON，compact 的独立 SSE 桥接保持。 2026-10-30 上游适配：OAuth 非流式 Responses 支持缺失 Content-Type 的 SSE 聚合及取消终态，保留上游与客户端流式标记、首输出前传输错误恢复及断连结算。 | `backend/internal/handler/failover_loop.go`、`backend/internal/service/upstream_outcome_error.go`、`backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/service/openai_plugin_transport.go` | 生效中 |
| `CUST-GW-009` | 代理到期与失败回退 | 代理支持到期处理、质量测试、失败回退和定向调度刷新；生产调度不会继续使用已失效代理。 | `backend/internal/service/proxy_expiry_service.go`、`backend/internal/service/proxy_fallback.go`、`backend/internal/repository/proxy_repo.go` | 生效中 |
| `CUST-GW-010` | 高并发请求体驻留治理 | OpenAI Responses 在首个有效语义输出确认后释放 handler、HTTP/WS 请求和解析视图持有的大请求体；此前完整保留请求体，并在 HTTP 与 WS HTTP bridge 统一暂存账号相关响应头和前导帧，用于安全故障转移。超大 passthrough 请求通过 bridge 下发时仍按轮保留到安全释放点，成功会话审计开启时保留审计所需的原始体。 WS 回放改为共享不可变体并记忆跨轮拒绝的加密内容；本地首语义输出前保留、输出后安全释放及审计保留边界不变。 本次上游适配：释放 WS 原始请求体前快照 service_tier、最终及原始 reasoning effort；成功、失败和断连结果共用完整元数据生成路径。 2026-09-15 上游适配：OpenCode 只保存清洗后的短会话 ID 快照，转换后请求不能覆盖原快照；与本地请求体释放/清理一同释放，不因会话标识保留完整大请求。 2026-09-30 上游适配：综合路由、上下文滚动与风控白名单接入后继续按轮快照模型、渠道及载荷散列，保留原子状态和请求体释放边界。 | `backend/internal/handler/openai_gateway_handler.go`、`backend/internal/service/openai_gateway_forward.go`、`backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_ws_forwarder_v2.go` | 生效中 |
| `CUST-GW-011` | 可配置附加换号状态码 | 在二开网关配置中按全局开关补充进入账号换号流程的 HTTP 状态码，默认关闭且预填 451；配置只扩展内置规则，保留上下文超限、Grok 内容拒绝等请求级语义终止保护。 | `backend/internal/service/gateway_additional_failover_status.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-012` | 指定分组首 Token 超时保护 | 在二开网关配置中选择全部分组或指定分组，默认 `all` 保持升级前行为；指定组至少选一个有效分组，依据请求上下文中的实际分组判断，不依据账号归属组。仅覆盖既有 HTTP 流式文本路径，不扩展 WS、非流式和媒体；前导纯空白不解除守卫，超时直接换号并沿用原换号上限。选中分组共用现有秒数、连续次数阈值，非受保护请求不增减首 Token streak；达到阈值后账号在所有关联分组停调度，按事故模型和时限测活恢复。范围变化纳入策略代次，旧客户端省略范围时不扩大为全局；全局指纹及原首输出行为兼容。与 `CUST-GW-013` 同时启用时由单层前导缓存协作；纯空白仍不解除首 Token 计时，但已交付不可撤销帧后的超时明确报错，不再换号重放。 2026-10-25 上游适配：OpenAI queued、metadata 与空 delta 重试判断继续优先保留指定分组严格首输出判定。 | `backend/internal/service/first_token_timeout_scope.go`、`backend/internal/service/custom_feature_settings.go`、`backend/internal/service/account_test_first_token_recovery.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |
| `CUST-GW-013` | Claude 透传流安全重试 | 仅对 Anthropic API Key HTTP 透传流提供独立开关，默认关闭，额外重试默认2次（0–5）、总预算默认300秒（1–3600）。首次成功流响应头到达后跨尝试累计预算；仅缓存可证明无有效内容的前导事件（含待判定帧，累计上限256 KiB），先原号再换号，无替代时允许仍可调度且允许同号重试的原号使用剩余额度。交付任何不可撤销内容前关闭重放资格；完整终止帧立即收尾，缺终止、半帧、协议错误不伪装成功。保留首 Token 分组语义、取消、账号资格与按请求用量去重，错误详情展示脱敏尝试诊断。可选提前保活见 `CUST-GW-014`，HTTP 响应开始与内容提交独立判断，原有首字统计不变。 2026-10-13 上游适配：Haiku 5.5 和模型级错误处理合入时，保留 Claude 模拟 beta 过滤、流安全重试与账号候选资格。 | `backend/internal/service/anthropic_stream_safe_retry.go`、`backend/internal/service/anthropic_stream_guard.go`、`backend/internal/handler/anthropic_stream_safe_retry.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中（默认关闭） |
| `CUST-GW-014` | Claude 安全重试提前 JSON 保活 | 在安全重试开启时，可选于首次成功流响应后立即发送标准 JSON `ping`，流读取期间按保活间隔继续发送（间隔未配置时使用10秒）。只发送无尝试身份的保活，暂存 `message_start` 与空前导；交付有效内容或未知不可撤销事件后禁止重放。保活不延长有效内容、空闲、首 Token 或总等待预算，不改变原有上游首事件统计。提前响应后的最终失败以 SSE 错误结束，非重试 HTTP 错误不得混写 JSON；失败尝试的响应头、消息身份与用量不得混入成功输出。 2026-10-13 上游适配：重试耗尽错误新增模型参数，仍区分提前 JSON 保活与实际语义输出提交。 | `backend/internal/service/anthropic_stream_guard.go`、`backend/internal/service/anthropic_stream_safe_retry.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 已发布v0.1.241（默认关闭） |
| `CUST-GW-015` | Kimi 思考模式类型兼容重试 | 在既有 Kimi 参数兼容重试中新增默认关闭的独立开关，仅对 Kimi 类型分组的 Chat Completions 出站、明确400枚举报错及实际非法 `thinking.type` 生效。剥离整个 `thinking`，保留独立思考强度与输出上限，每请求最多一次；嵌套预算/keep 随之移除，已提交响应或取消不重放。五项规则累计最多额外五次，修正沿用至外层重试与换号，不影响其他分组及原生其他协议。 2026-10-13 上游适配：共享错误体读取新增完整性判断，同时保留读取错误返回值和 Kimi 参数重试调用契约。 | `backend/internal/service/kimi_parameter_compat.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 待发布，默认关闭 |
| `CUST-GW-016` | Kimi 动态工具契约与渠道兼容 | 新增默认关闭的独立网关开关，仅 Kimi 类型分组的 Chat Completions 入站生效。在并发排队和协议转换前拒绝错误角色、非空正文、非法定义及重名；接纳 KVV 空正文声明并合并到顶层工具，保留普通消息、工具历史与上游 usage。支持既有 CC/Responses/Anthropic 出站路由，开关请求级冻结，重试不重复合并；不修正 Token 基准，不宣称原生动态工具缓存语义等价。 2026-10-13 上游适配：Responses 转换回退仍传播插件已发送的不可重放错误，保留动态工具与首输出生命周期。 | `backend/internal/service/kimi_dynamic_tools.go`、`backend/internal/handler/openai_chat_completions.go`、`frontend/src/views/admin/CustomFeaturesView.vue`、`docs/kimi-dynamic-tools.md` | 待发布，默认关闭 |

### 协议与上游兼容

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-PROTO-001` | OpenAI Codex CLI 模拟与客户端策略 | API Key/OAuth 路径支持 Codex CLI 请求模拟、统一出站身份、User-Agent 和版本范围策略、App Server 客户端识别及能力探测一致性；带 `client_version` 的模型请求及 `/backend-api/codex/models` 按实际分组路由生成完整 Codex 清单，合并管理员模型配置与已同步能力元数据，普通 `/models` 仍保持 OpenAI 列表契约。Fast 模型向 Codex 客户端广告 priority 能力，但 OAuth-like 上游返回的 `default` 不覆盖明确请求的 priority。指纹由系统管理的账号随机种子稳定派生，覆盖 HTTP/WS 请求头、`client_metadata` 和默认 `prompt_cache_key`，同一请求头体共用时间戳。有效版本可按 GitHub 最新 release 定期同步，管理员 UA 只贡献非版本指纹。 纳入 Astra、ultrafast、none 推理来源、能力同步和默认关闭的分组固定账号清单。固定清单仅检查组成员、Active、Schedulable 和有效期，读取时不套用限流/临时冷却，不改变实际推理调度；Astra/gpt-6 进入本地精确已知模型规范化，不采用子串宽匹配。 本次上游适配：分组白名单兼容本地精确已知模型规范化；未知 codex 等自定义变体不自动视为基础模型。 2026-09-20 上游适配：非法 UA 不再丢弃独立解析的 Codex 版本，管理员非版本指纹与本地统一身份策略继续共存。 2026-10-07 上游适配：Codex 远程模型目录配置增加 api_key_model_discovery；文件目录模式不启用该开关。保留本地身份模拟、鉴权模式、HTTP/WS 和各路由平台配置，并补充 macOS/Linux 与 Windows 配置断言。 2026-10-09 上游适配：接入加密推理签名恢复及 web_search 历史工具声明，保留本地 Codex 模拟开关与严格计费边界。 2026-10-25 上游适配：合入 Codex overload 身份归一及空流重试，保留指纹、版本和模拟开关。 | `backend/internal/service/openai_codex_emulation.go`、`backend/internal/service/openai_codex_identity.go`、`backend/internal/service/openai_codex_fingerprint.go`、`backend/internal/service/openai_codex_models_service.go`、`backend/internal/service/openai_codex_version_sync_service.go`、`backend/migrations/225_backfill_codex_fingerprint_seed.sql`、`frontend/src/components/keys/UseKeyModal.vue` | 生效中 |
| `CUST-PROTO-002` | Codex 图片工具策略 | 可完全禁用或按策略处理图片生成工具；覆盖原生 Responses、Chat fallback、namespace 和 WSv2，防止换名、嵌套或重复注入绕过。 2026-09-15 上游适配：原生 Codex Images 接入不改变既有 Responses/Chat/namespace/WS 图片工具策略；两类路径回归断言均保留。 | `backend/internal/service/codex_image_generation_bridge.go`、`backend/internal/service/openai_responses_namespace.go`、`backend/internal/service/setting_service_codex_policy_test.go` | 生效中 |
| `CUST-PROTO-003` | Anthropic Claude Code 上游模拟 | 支持全局开关，将普通下游请求按 Claude Code 客户端规则清洗、补充和转发；账号测试与真实网关使用同一策略。 纳入可选 CLI 版本环境覆盖；强制模拟后的最终出站 UA 与 billing 指纹版本一致，并保留 Unicode 字符索引算法。 2026-09-15 上游适配：Claude 模拟、网关与账号测试继续复用本地清洗策略；本地 system 配置与上游缓存块上限并存。 2026-09-24 上游适配：接入 Claude Code 动态版本；messages、count_tokens 和账号测试共用单次请求的 UA/billing 版本快照，运行期切换不能造成头体不一致，全局强制模拟继续覆盖 API Key。 2026-09-30 上游适配：显式 structured-output beta 在模拟白名单内保留，策略过滤仍优先；Sonnet 5.5 工具集 beta 过滤在最终模拟头形成后执行，不混入账号覆写的客户端身份。 2026-10-09 上游适配：仅在生成 tools/system 自动缓存断点时检查后续客户端 1h，必要时生成 1h，否则仍显式 5m；不在出口统一升级客户端显式 5m。 2026-10-30 上游适配：指纹按批准方案改为 JavaScript UTF-16 码元采样及 UTF-8 编码，保留模拟开关、UA/版本一致性和中文回归。 | `backend/internal/service/gateway_anthropic_mimicry_sanitize.go`、`backend/internal/service/gateway_anthropic_oauth_mimicry.go` | 生效中 |
| `CUST-PROTO-004` | Claude OAuth 提示词与协议清洗 | 可配置 OAuth system prompt/blocks，处理 beta header、thinking block、Bedrock CC 兼容和重试时的协议字段；清理 deferred tool 的 `cache_control`，并把缓存断点保留在最后一个非延迟工具。 2026-09-15 上游适配：保留 stripSystemCacheControl、preserveSystemText、preserveModel 的独立选项和注入关闭时清洗；count_tokens 也执行四块缓存上限。 2026-10-09 上游适配：默认系统块和布尔 true 自动断点按后续 1h 选择 TTL，显式配置对象及客户端断点保持原值，客户端自身无效顺序不由网关覆盖修复。 | `backend/internal/service/gateway_claude_oauth_body.go`、`backend/internal/service/gateway_tool_rewrite.go`、`backend/internal/service/setting_update.go` | 生效中 |
| `CUST-PROTO-005` | API Key 账号请求头覆写 | Anthropic/OpenAI/Kimi/Zhipu/DeepSeek API Key 账号及 Grok API Key/OAuth 账号可以配置请求头覆写；真实转发、模型列表、能力探测、count tokens、Images、Kimi 原生 Responses 和 WS 使用一致的最终请求头。 仅合法 OpenCode 目标在账号覆写后转发调用者会话头；非 OpenCode 目标不泄露该头。 本次上游适配：兼容 MiniMax 平台与 Ollama Anthropic Bearer 鉴权选择，不改变本地最终请求头覆写顺序。 2026-09-30 上游适配：请求头覆写仍受认证及模拟身份边界约束；Sonnet 5.5 不兼容的细粒度流式 beta 在 messages、count_tokens 最终头上再次过滤。 | `backend/internal/service/openai_gateway_forward.go`、`backend/internal/service/upstream_models.go`、`frontend/src/components/account/EditAccountModal.vue` | 生效中 |
| `CUST-PROTO-006` | 请求端点与 compact 信号规范化 | 从原始路径规范化入站端点，区分 Responses compact 和 `/responses/input_tokens`，识别 `compaction_trigger`，继承客户端工具状态；Kimi PayG/Coding 的显式 `responses` 或 `adaptive` 请求使用各自原生 `/v1/responses`，强制 `store=false` 并移除 `previous_response_id`。继续保留本地 HTTP 流转 WSv2、namespace，以及每轮重新取得请求模型和渠道映射、仅在上下文可完整重建时恢复当前轮次换号的决策边界。 2026-09-15 上游适配：接入 OpenCode 三协议端点与单次模型映射，保留 Kimi 原生 Responses/store=false、本地端点归因及 WS 逐轮策略；compact 缺省模型改为 gpt-5.5。 2026-09-18 上游适配：接入 Responses 前导指令合并、中段指令转 user、严格 Chat 目标的 developer 转 system、Lite namespace 及工具媒体处理；Kimi 默认关闭的参数重试与入口请求体保护保留，并补充角色转换和 400 修正同时发生的交叉回归。 2026-09-20 上游适配：DeepSeek 推理占位及 Anthropic 根联合工具 schema 扁平化进入共用协议链；Kimi 四项默认关闭的参数修正、有限重试、首 Token 清理和既有 namespace 转换保留。 | `backend/internal/handler/gateway_handler.go`、`backend/internal/service/openai_gateway_request_body.go`、`backend/internal/service/openai_ws_http_bridge.go`、`backend/internal/service/account_test_service_cn_adaptive.go` | 生效中 |
| `CUST-PROTO-007` | Responses 渠道监控请求格式 | `probe` 和 `quota_probe` 使用结构化 Responses `input` 消息数组，而不是普通字符串；`quota` 只读取额度，不构造 LLM 请求，上游同步不得退回非标准请求体。 | `backend/internal/service/channel_monitor_checker.go`、`frontend/src/components/admin/monitor/MonitorAdvancedRequestConfig.vue` | 兼容覆盖 |
| `CUST-PROTO-008` | Anthropic 采样参数过滤 | 按最终上游模型精确匹配或末尾通配符匹配，在模型校验前删除根级 `temperature`、`top_k`、`top_p`；覆盖 Messages、令牌计数、API Key/OAuth、协议转换、直通和账号测试。可选择 Anthropic 分组停用整个功能；独立跳过 thinking 和 tool_choice 模型限制的开关默认关闭，受总开关、模型白名单及实际转发分组共同约束。无分组账号测试只保留采样过滤。 | `backend/internal/service/gateway_anthropic_sampling_filter.go`、`backend/internal/service/custom_feature_settings.go`、`frontend/src/views/admin/CustomFeaturesView.vue` | 生效中 |

### 账号与运维测试

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-ACC-001` | 账号策略默认值与模型白名单 | 创建账号时提供本地 pool mode 默认策略、模型白名单和平台特定控件；API Key 账号落库前可按当前代理、并发、端点和模型映射预览上游模型，创建后再正式同步能力元数据；批量编辑保持这些字段的 API 契约。分组 Fast 与 Free Fast 策略贯穿创建、更新、复制、管理 API 和认证快照，不改变既有账号默认策略。 新上游请求标识头、图片回填开关与本地失败策略、图片尺寸字段组合保存；模型同步成功提示继续显示数量。认证快照升至 v24，同时保留固定账号清单、长上下文与 Free Fast 字段。 本次上游适配：本次将旧 models_list_config 数据迁移为 model_allowlist；获批后同时约束模型展示与实际请求准入，未启用时不限制。认证快照升级到 v24，固定账号清单、长上下文和 Free Fast 字段完整保留。 2026-09-18 上游适配：按批准方案让 DeepSeek 无显式映射时使用上游模型名单，不再任意兜底；显式映射、本地其他 Provider 白名单及默认策略保留。 2026-09-20 上游适配：账号端点能力增加默认不启用的 Seedance，继续保留本地图片尺寸类型、模型白名单、pool mode 和表单策略。 2026-09-30 上游适配：拒绝白名单与非同名模型映射冲突，保留保存前凭据预览同步、模型ID复制和原有默认策略；分组白名单支持任意位置通配符。 2026-10-09 上游适配：平台选项和模型同步能力改从上游平台目录派生，保留本地预览请求接口与既有账号策略默认值。 2026-10-16 上游适配：订阅模型限制按实际 OpenAI OAuth/setup-token/shadow 凭据及最终映射目标判定，含透传；API Key 和明确替换模型保持可用，原账号默认策略不变。 2026-10-25 上游适配：创建账号接入 service_account 配额判定时保留本地失败调度策略注入和账号默认值。 2026-10-30 上游适配：合入 Grok 目录快照与差异诊断，预览继续使用本地 previewSyncRequest，并隔离凭据变化后的旧响应。 | `frontend/src/components/account/poolModeDefaults.ts`、`frontend/src/components/account/ModelWhitelistSelector.vue`、`frontend/src/components/account/CreateAccountModal.vue`、`backend/migrations/145_group_models_list_config.sql` | 生效中 |
| `CUST-ACC-002` | 账号数据批量导入与可见性增强 | 前端支持拖拽/批量导入账号数据；账号列表展示账号 ID，并改进 API Key 查看、滚动和查询上下文重置。 2026-10-16 上游适配：Codex 导入按工作区隔离，保留现有批量导入、可见性与账号身份约束。 | `frontend/src/views/admin/AccountsView.vue`、`frontend/src/components/account/CreateAccountModal.vue` | 生效中 |
| `CUST-ACC-003` | API Key 分组访问约束 | 严格校验 API Key 的专属分组访问，拒绝跨专属分组调度，并支持管理员按用户 API Key 所在分组筛选。 | `backend/internal/service/gateway_channel_restriction_test.go`、`backend/internal/service/admin_user.go`、`frontend/src/views/admin/AccountsView.vue` | 生效中 |
| `CUST-ACC-004` | OpenAI 额度查询与重置 | 账号管理可查询 rate-limit credits，支持人工重置和达到阈值后自动使用重置卡。只有取得幂等执行权的实例会在首次出站调用前捕获并持久化恢复快照；同一额度周期的超时重试和成功结果回放复用该快照，只通过 `rate_limited_at + rate_limit_reset_at` 双时间戳 CAS 清除同一数据库限流代次，运行时还要求实例 ID、generation 和 429 原因匹配。升级前缺少快照的旧失败态或幂等记录使用空快照，只刷新额度，不捕获或清除当前新限流代次；人工重置继续执行广义状态恢复并失效 Token，调度器同步刷新相关快照。 2026-09-18 上游适配：保留 ClearOpenAIRateLimitIfObserved 的 OpenAI OAuth 双时间戳清除 CAS，另保留上游 SetRateLimitedIfUnchanged 的 UpdatedAt 加限流代次写入 CAS；两个方法不合并、不互相替代。 | `backend/internal/service/openai_quota_service.go`、`backend/internal/service/openai_quota_reset_credits.go`、`backend/internal/service/openai_quota_auto_reset.go`、`backend/internal/service/openai_quota_reset_workflow.go`、`backend/internal/service/ratelimit_service.go` | 生效中 |
| `CUST-ACC-005` | 增强账号测试 | 手动和定时测试覆盖 OpenAI Responses/compact/Images、Anthropic 模拟、Gemini 和 Grok 各媒体模式。普通文本默认测试模型独立于业务及计费：OpenAI 为 `gpt-6-astra`，Anthropic/Antigravity 为 `claude-opus-5`，Gemini 为 `gemini-3.8-flash`，Grok 为 `grok-4.6`；Bedrock/旧 Google One 保留既有受支持默认值，显式任务模型不改写。国产供应商按实际协议测试，从配置中优先选择具体文本请求模型，只有通配符时取具体目标，去重并按字典序稳定选择；不重复映射，无候选时报配置错误且不出站。手动预选同步，自定义选项仍保留；首 Token 恢复事故模型优先于上述默认值。保留采样过滤、语义错误、工具调用、媒体与提示词校验，以及 Kimi 原生 Responses/store=false 契约。 本次上游适配：纳入上游实时模型发现、显示名称和 MiniMax；本地默认模型、显式任务模型及恢复事故模型优先级不变，前后端均补充 MiniMax 确定性选择回归。 2026-09-15 上游适配：OpenCode 有映射时按本地确定性规则选择具体文本模型，无映射时使用 glm-5.3；测试副本固定最终模型以免二次映射，Responses/Anthropic 沿用自定义提示词，已有平台默认值不变。 2026-09-24 上游适配：模型选择器接入映射过滤与 OAuth 图片别名修复，继续保留本地默认测试模型和确定性选择规则。 2026-10-09 上游适配：接入按模型路由、测试失败日志归属及 Command Code/Cline；保留本地默认模型、可配置提示词和单次映射，Cline 无映射时按订阅与钱包冷却动态选默认值。 2026-10-16 上游适配：Antigravity 测试支持模型目录及图片结果；本地账号测试提示和请求隔离边界继续保留。 2026-10-25 上游适配：账号模型响应提取为共享方法后，继续优先保留多协议 API Key 提供商的本地测试模型目录。 2026-10-30 上游适配：普通 Gemini 默认测试模型保持 gemini-3.8-flash；Google One 使用保守列表首项 gemini-2.5-flash；保留 prepareAccountTestModel 的映射选择。 | `backend/internal/service/account_test_models.go`、`backend/internal/service/account_test_service.go`、`backend/internal/service/account_test_service_cn_adaptive.go`、`frontend/src/utils/accountTestModels.ts`、`frontend/src/components/admin/account/AccountTestModal.vue` | 生效中 |
| `CUST-ACC-006` | 本地数据兼容迁移 | 维护注册来源默认授权、平台 quota、订阅到期通知和账号扩展字段的迁移兼容，并与上游原生 compaction、渠道缓存写价、分组 Fast/Free Fast 及 reasoning 超限策略迁移共存，避免本地历史数据在上游同步后丢失约束。 新增上游四份请求 ID、并发索引、max 倍率和固定账号清单迁移；与本地同数字前缀迁移按完整文件名共存，不重命名或改写旧迁移。上次同步时 SQL 真实执行未验证。 本次上游适配：本次新增白名单收敛与代理字段等上游迁移，与本地同数字前缀迁移按完整文件名共存；已在隔离 PostgreSQL 中验证全部迁移及白名单旧数据修复，不涉及生产数据。 2026-09-15 上游适配：新增两份 238 迁移按完整文件名共存；只删除日/周/月均 NULL 的 quota 行，零限额及任一有效限额保留。隔离旧库升级与重复执行均验证通过，本地 237 用户定制/授信数据不变。 2026-09-24 上游适配：新增上游239通用思考倍率回填及240线下提现幂等索引迁移，原始SQL不改写；仅隔离验证，不因本次代码同步授权生产执行。 | `backend/migrations/142_extend_user_provider_default_grants_check.sql`、`backend/migrations/143_subscription_expiry_notify_enabled.sql`、`backend/migrations/144_user_platform_quotas.sql`、`backend/migrations/231_add_usage_log_native_compaction_v2.sql`、`backend/migrations/232_group_force_openai_fast.sql`、`backend/migrations/233_group_free_openai_fast.sql` | 兼容覆盖 |
| `CUST-ACC-007` | 复制账号测试计划 | 账号、分组优先级、源测试计划与调度通知在同一事务中复制；副本账号和全部计划初始停用，不继承测试结果、执行时间、故障状态或失败次数。唯一系统计划优先沿用源系统计划，没有则使用最早创建且开启自动恢复的普通计划，否则使用默认模板；故障时自动启用并按二开退避恢复，不解除人工暂停。复制 API 保持幂等和凭据脱敏，不追溯改写历史副本。 | `backend/internal/service/admin_account.go`、`backend/internal/repository/account_repo.go`、`backend/internal/repository/account_duplicate_plans.go` | 生效中 |

### 计费、定价与可观测性

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-BILL-001` | 按用户请求模型计费 | 普通分组严格按用户请求模型计费，不使用渠道映射名、最终上游模型或响应模型回退；Composite 分组仅在公开别名存在显式渠道价时按别名计费，否则按实际路由的具体模型计费。Chat、Responses、Images、Messages 与 WS 入口统一记录请求模型、映射链和上游响应模型；上游响应模型只用于诊断和账号成本，不反向改变用户计费来源。Free Fast 的用户 `ActualCost` 按 Standard 计费，账号 `TotalCost` 仍按 priority 上游成本。 映射模型只影响调度和转发，旧 upstream/channel_mapped/response_model 来源继续归一为 requested；负载、粘性、路由均不能绕过原请求模型价格清单，未知 Astra 自定义别名仍须显式价格。 本次上游适配：DeepSeek 历史 PricingAt 的账号成本使用最终上游模型；请求别名仍须满足本地显式价格准入，不回退映射型号。 2026-09-15 上游适配：OpenCode 与图片新分支仍保留请求模型准入、映射链和响应模型诊断；新增 DeepSeek 切换价不替代自定义渠道/分组价。 2026-09-20 上游适配：映射请求的响应 model 可还原为公开别名，真实上游响应型号仍先行采集用于诊断及账号成本；请求模型、映射链、Token 用量和用户按请求模型计费规则不被响应改写覆盖。 2026-09-24 上游适配：通用思考倍率使用最终出站等级，Kimi 兼容重试后的有效等级优先；计费模型继续采用用户请求模型。 2026-09-30 上游适配：WS 综合模型仍按每轮用户请求模型计费，映射后的上游模型仅按既有规则用于账号成本与诊断。 2026-10-03 CI适配：gpt-6.1-sol补入已知请求模型的严格计费规范化，复用已同步价格表；已知后缀和compact别名可归并，未知自定义后缀仍须显式价格，不回退账号或渠道映射。 2026-10-09 上游适配：新增国产平台/OpenCode Claude 计费用例继续核验请求模型和不使用响应模型兜底的本地边界；仅补齐 OpenCode Go 缺价零费用审计分支，不改变严格缺价错误与扣费策略。 2026-10-13 上游适配：Gemini 显式缓存创建、续期报价和入账统一按请求计费模型；禁用响应模型改价与缺价后采用映射模型收费。 2026-10-16 上游适配：新 Gemini/Antigravity 型号、思考档位与工具图片接入后继续按用户请求模型计费，不改用映射模型或响应模型兜底。 2026-10-25 上游适配：配额累加与通知仍使用 accountQuotaBaseCost；新增日周维度参数及 Vertex 配额。Gemini 请求模型定价检查与 countTokens 免费语义保留。Grok 4.7 Fast 使用独立价卡，长上下文继续严格超过 200K 才加价，测试覆盖边界两侧。 2026-10-30 上游适配：Grok 响应模型审计增强仍只影响诊断及账号成本，不恢复按响应模型给用户扣费。 | `backend/internal/handler/requested_model_pricing.go`、`backend/internal/service/requested_model_pricing.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/migrations/175_force_requested_billing_model_source.sql` | 生效中 |
| `CUST-BILL-002` | 多层定价与严格缺价处理 | 所有 token 入口统一使用同一计费选择路径，定价优先级固定为“分组逐模型定价 → 渠道定价 → 动态远端定价 → 内置/兜底价格”；支持渠道默认定价、Standard/Fast/Flex 服务层级倍率、时段倍率，以及 token 区间输入/输出/缓存读写绝对价或倍率，认证快照完整保留渠道定价。服务层级按上游实际结果只允许降档计费，不得把请求档位升级为更高倍率；Codex OAuth-like 返回 `default` 不具备上调权威性，明确 `flex` 仍可降档。支持 DeepSeek/GLM/Kimi/MiniMax/豆包/Grok 等兜底定价及 thinking、图片、视频、搜索/音频计价补充；DeepSeek 默认价卡按官方工作日峰谷时段计费并纠正远端旧价，但分组/渠道自定义价不被覆盖。xAI/LiteLLM above-200K 绝对价仅对 Grok 转换为内部倍率，完整远端价卡保持优先；已知残缺 Grok 价卡才补内置阶梯，4.5 的已知错误缓存价只按精确签名修正。目录阶梯与本地边际规则按显式优先级互斥，避免重复升档；非简单模式下选定计费模型缺价必须返回错误，不得静默记为零费用；Composite 的具体模型回退仅用于公开别名未配置渠道价的场景，不改变普通分组边界。 接入 fallback/override 文件内容哈希与一致快照热重载；新增 Astra 精确兜底；Fable 5.1 的 max 不再默认乘3，输入、输出及缓存读写按实际 Token 用量与基础价计费，用户倍率照常应用。仅显式配置的渠道 max 倍率按最终转发 reasoning effort 生效；账号成本和公开价格展示不得隐式注入三倍规则。后续上游同步不得恢复该默认加价，不覆盖本地缺价拒绝、渠道优先及其他模型特例。 本次上游适配：账号成本复用统一峰谷/缓存定价管线，继续保留 Fable max 无默认三倍及无渠道按上游型号成本回退。 2026-09-15 上游适配：接入图片缓存输入细分及 DeepSeek Flash 新默认价；Pro 是否按 Flash 默认价结算按 PricingAt 与北京时间 2026-09-14 12:00 比较。严格缺价、定价优先级、Fable 无默认三倍及显式 max 倍率保留。 2026-09-24 上游适配：采用 reasoning_effort_multipliers 映射，未配置等级为1倍，旧显式max配置由上游迁移转换；保留严格缺价、渠道默认价、分时价和账号统计成本。 2026-09-30 上游适配：渠道图片输入/输出价留空继承目录价，显式0免费；无目录图片价才回退文本价。保留分组/渠道优先级、区间/Fast/Flex、严格缺价和无渠道账号成本回退；Free Fast 在标准模式缺价仍报错且不写用量，简易模式才按既有规则保留零费用日志。 2026-10-09 上游适配：新增国产平台/OpenCode Claude 缺价测试沿用零费用审计并返回缺价错误供告警的契约；修复 OpenCode Go 候选模型被过滤后未记录审计的问题，保留真实 Token、请求模型和不扣费断言，显式价格正常扣费，不以返回 nil 掩盖缺价。 2026-10-13 上游适配：缓存存储保留明确型号及显式配置单价；未知 Pro/Flash 型号不得按名称猜价，报价缺价必须返回错误；纯续期不重复收 token 或按次费。 2026-10-16 上游适配：纳入上游 Argon 与 Claude 5.5 思考档位价卡；保留显式渠道价、严格缺价和未知缓存型号不猜价，Argon 无内置显式缓存存储价。 2026-10-30 上游适配：按批准方案取消 DeepSeek Pro 按日期转 Flash 默认价；自定义价格优先不变。图片按张开关默认关闭并接入严格路径，渠道计算失败仍沿用本地回退。 | `backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/pricing_service.go`、`backend/internal/service/requested_model_pricing.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/migrations/147_channel_default_pricing.sql`、`backend/migrations/221_group_model_pricing.sql`、`backend/migrations/228_channel_pricing_multipliers.sql` | 生效中 |
| `CUST-BILL-003` | 长上下文计费兼容策略 | OpenAI 账号开关与真实分组配置共同约束长上下文阶梯；只有实际上下文命中渠道区间时才禁用内置倍率，区间未命中回退基础价时仍可应用。数据库中的分组开关默认开启且显式关闭有效，API Key 认证缓存快照必须完整保留该开关；未携带真实分组配置的旧调用对象继续按兼容默认开启。Gemini 原生 `/v1beta` 在超过 200K 后仅对超阈值的 input/cache read 应用 2 倍边际价，output/cache write 保持基础价，模型广场标记为 `marginal`；显式边际规则优先于目录整单阶梯。GPT-5.6 静态兜底在严格超过 272K 时输入 2 倍、输出 1.5 倍；Grok 只服从分组开关，不被 OpenAI 账号开关否决，并在严格超过 200K 时对普通输入、缓存输入与输出使用同一阶梯；GPT-5.4 Pro 无目录数据时只使用基础静态价，有目录阶梯时按目录计费。全部所选分组开启时前端隐藏冗余账号开关，否则保留。 新增 Astra 272K 严格大于阈值的输入2倍、输出1.5倍阶梯，继续服从本地分组长上下文开关并保留 GPT-5.6/Gemini/Grok 边界。 本次上游适配：认证快照 v24 继续完整投影长上下文和分组模型定价，原阈值与默认值不变。 2026-09-15 上游适配：合入图片缓存价时继续保留 Grok 严格大于 200K 的本地边界、分组长上下文开关及 Gemini/GPT/Astra 原规则。 2026-09-24 上游适配：接入 GPT-6 Sol/Luna 与 Opus 5.5 定价及 Fast 规则，同时保留本地 GPT-5.6/Grok 修正；新模型加入严格白名单，不扩大未知型号兜底。 2026-09-30 上游适配：账号成本的目录价格阶梯只看账号开关，并显式记录门禁已配置；客户售价继续要求分组和账号双门禁，不改为上游或门禁。 2026-10-13 上游适配：实际 OAuth 类账号发送 GPT-6.1 Sol 时豁免内置长上下文附加费，API Key 与管理员显式区间价保留；继续使用双门禁及 actualInterval == nil 判断。 2026-10-14 补充适配：新 Gemini 缓存接口把 Gin 中的已鉴权用户标识传入请求上下文，确保真实入口的必记账任务使用正确的用户队列键。 2026-10-16 上游适配：新思考档位继续经过既有计费入口，分组长上下文门禁、显式定价优先和缓存对象按请求模型归因保持。 2026-10-30 上游适配：Terra 同时接入独立定价归一化及缓存写入策略；图片渠道按次价格保留 token 上下文分档，本地长上下文边界保持。 | `backend/internal/service/billing_service.go`、`backend/internal/service/billing_token_cost_request.go`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/api_key_auth_cache_impl.go`、`backend/ent/schema/group.go`、`backend/migrations/175_default_openai_long_context_billing.sql`、`backend/migrations/221_group_model_pricing.sql` | 兼容覆盖 |
| `CUST-BILL-004` | Image 分组成功率 | 统计单次和批量图片请求的分组请求数/失败数，支持代次式原子清零、用户展示开关，并排除 keepalive 字节对结果判断的干扰；用户端 Channel Monitor V1/V2 均可展示该统计。 2026-10-11 按用户确认的业务口径，将403余额/权限拒绝、明确的余额不足、内容审核拒绝及参数/输入错误同时排除出请求数和失败数，不算作成功；HTTP、流式终态与批量提交/逐项结果采用一致规则。保留超时、过载、上游401/429、网络故障和未知原因失败；不改计费、重试、运维错误日志或历史计数。 本次上游适配：根路径别名和 Codex 直连新白名单链保留图片分组结果统计中间件，不漏记 Chat/Responses。 2026-09-20 上游适配：新增 Seedance 任务路由后，根路径 Responses 及其子路径仍安装 trackImageGroupResult；未移除既有图片请求统计。 2026-10-13 上游适配：监控自动刷新更新明细，仍保留生图成功率、取消过期列表请求和清除旧明细；延迟返回的旧明细不得覆盖刷新结果。 | `backend/internal/service/image_group_success_rate.go`、`backend/internal/service/image_group_success_rate_errors.go`、`backend/internal/repository/image_group_success_rate_repo.go`、`frontend/src/views/user/ChannelStatusV1View.vue`、`frontend/src/views/user/ChannelStatusV2View.vue`、`backend/migrations/177_image_group_success_rates.sql` | 生效中 |
| `CUST-BILL-005` | 用量与费用明细增强 | 展示缓存 Token、请求模型、服务层级、余额调整和图片/视频费用信息；推理强度同时记录请求值与策略映射后值，用户只看请求值，管理员可查看两者；错误请求和实际上游端点记录使用本地归因规则。Free Fast 请求的 usage `service_tier` 保持 priority 以反映上游服务，用户实际费用按 Standard、账号成本按 priority 分别记录。 增加默认隐藏的管理员上游请求 ID 列及按账号配置的响应头采集，未配置不采集。配置写入和读取兜底均拒绝 Authorization、Proxy-Authorization、Cookie、Set-Cookie、X-API-Key、Api-Key、X-Auth-Token、X-Access-Token；WS 无响应头时不伪造值。ultrafast 接入不改变 Free Fast 双成本。 本次上游适配：使用记录筛选加载全部分页 API Key，保留本地列设置与筛选组件。共用用量表格新增首字后平均输出速率，具体口径及不适用边界见下方补充规则；用户充值历史和管理员余额历史支持独立的临时授信记录。 2026-09-15 上游适配：原生图片记录交付/断连、部分用量、图片缓存与真实上游端点；Free Fast 双成本和授信明细保留。 2026-09-24 修复大批量导出：新增独立游标查询，保留认证、筛选和脱敏，固定按 ID 倒序且每批最多1000条，不重复 COUNT/OFFSET；前端限速、有界重试、进度与取消，CSV按批编码，XLSX继续在浏览器组装。详见下方导出边界。 2026-09-24 上游适配：缺失思考等级导出为普通横线；保留危险值公式防注入、游标分页、节流、取消和初始筛选快照。 2026-09-30 上游适配：Free Fast 仅在既有允许缺价记录的路径（本次回归覆盖简易模式）保留零费用用量行，标准模式仍拒绝未知定价且不写用量；已有价格时 Standard 客户费用与 priority 账号成本分离不变，WS Cyber 失败不重复记账。 2026-10-09 上游适配：共用用量表继续使用首字后两位小数 tk/s，不采用上游总耗时单请求 TPS；运维看板的上游 TPS 分位数独立接入，不新增用量数据库字段或改变导出。 2026-10-13 上游适配：用户游标导出与列表共同按 Accept-Language 本地化分组并移除完整译文表，保留认证、所有者限定、筛选和管理字段脱敏。 | `backend/internal/repository/usage_export.go`、`frontend/src/utils/usageExport.ts`、`backend/internal/service/gateway_usage_billing.go`、`frontend/src/components/admin/usage/UsageTable.vue`、`frontend/src/utils/usageOutputRate.ts`、`frontend/src/components/admin/user/UserBalanceHistoryModal.vue` | 生效中 |
| `CUST-BILL-006` | 高并发余额扣费整流 | 单实例内对同一用户的余额扣费事务串行排队，不同用户仍可并行；usage worker 使用 `SubmitKeyed(userID)` 在执行前按用户聚合，同一热用户只占一个 worker，任务真正开始执行时再创建超时 context，默认及生产继续保持 5 秒 usage task 超时。WS 每轮独立结算 usage，余额事务 gate 继续在 `BeginTx` 前提供最终进程内串行。 2026-09-24 上游适配：简易模式新增可选API Key时间窗口检查，不改变按用户串行扣费、既有队列和5秒任务超时。 2026-09-30 上游适配：实际流 usage、账号成本门禁和失败去重合入后，按用户串行扣费及5秒任务超时保持不变。 2026-10-13 上游适配：GatewayHandler 必记账任务改用 SubmitKeyed(userID)，同步回退由10秒统一到5秒；队列满或已停止仍执行，计费完成后释放在途预留。 | `backend/internal/repository/usage_billing_repo.go`、`backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/usage_record_worker_pool.go` | 生效中 |

| `CUST-BILL-007` | 一次性自动授信 | 默认关闭，按用户配置阈值（默认1000）和固定正向金额，单位与账户余额一致、保留8位精度。独立后台启动即查，此后每60秒分批检查正常用户；仅 `users.balance` 严格小于阈值且本代次未使用时入账。锁定用户和配置，在单事务中写余额、授信流水、`temporary_credit` 充值历史及鉴权缓存失效事件，以用户＋代次唯一约束防止多实例和重试重复加款；余额缓存失败持久重试并延迟二次删除。只通过校验当前代次的独立确认操作恢复机会，普通保存、开关切换、充值和重置链接均不恢复；不自动扣回，不计真实充值累计，不产生返佣和充值赠送。网关扣费路径、按用户串行和5秒用量任务超时不变。 | `backend/internal/service/temporary_credit_worker.go`、`backend/internal/repository/user_customization_repo.go`、`backend/migrations/237_user_customizations.sql` | 生效中 |

`CUST-BILL-005` 使用明细补充：管理员和用户共用 `UsageTable.vue` 在延时列总耗时下显示首字后平均估算输出速率（两位小数、`tk/s`），口径为 `output_tokens × 1000 ÷ (duration_ms − first_token_ms)`。仅有有效时间数据的 HTTP 流式文本和 WS 文本逐轮适用；非流式、媒体、Live、缺失时间、非正输出或非正时间差显示 `-`。不新增用量数据库字段，不改运维看板及导出口径；入口为 `frontend/src/utils/usageOutputRate.ts` 和共用 `UsageTable.vue`。

`CUST-BILL-005` 导出边界（2026-09-24）：使用记录导出复用原筛选和用户归属校验，但顺序固定为不可变 ID 倒序，不跟随表格临时排序。游标为十进制字符串，避免大 ID 的 JavaScript 精度丢失；新写入的更大 ID 不插入当前遍历，导出不是跨批次数据库事务快照。单批查询20秒超时，批次间隔1250毫秒；429按 Retry-After 等待，网络和部分5xx最多重试5次，不放宽现有限流。用户CSV只累计编码后的分片；管理员XLSX仍受浏览器内存及Excel单表行数限制，未引入持久化后台导出任务。取消或离开页面立即终止后续请求，取消不再提示失败。不改导出费用字段和计费规则，不需要数据库迁移。

### 渠道监控与可观测性

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-OBS-001` | 流式渠道监控与随机抖动 | `probe`/`quota_probe` 渠道监控和模板可以选择流式检测，`quota` 模式独立读取额度；探针总超时与响应头超时统一为 600 秒，Gemini 流式与非流式响应均遍历首个候选的全部文本分段并跳过思考分段；检测间隔支持正负随机抖动，避免大量渠道同一时刻探测，并保留 provider/endpoint/请求体的本地扩展。渠道分组按用户访问范围过滤，OpenAI 可见 TTFT 的统计口径由管理员设置统一控制。2026-09-18 上游适配：吸收监控自动刷新间隔修复；Provider 测试继续按 PROVIDERS.length 验证并保留 MiniMax。2026-09-24 修复启动/待激活/退役及共享执行资格拒绝后未释放 in-flight 导致永久停探；无样本可用率为 null，真实失败仍为 0%；区分过期数据并隐藏过期延迟/额度，同步刷新多窗口缓存，过期阈值包含两倍最大抖动间隔及完整探测预算。2026-10-09 接入智谱端点路径修复与请求样本门槛。2026-10-26：V1 按平台顺序分区、区内保持原排序、未知平台追加，Image 独立；普通卡片约 146px，桌面四列、宽屏六列。列表隐藏最后检测/成功时间、检测次数和重复倒计时，刷新集中顶部；保留历史条、统计周期、分组、配额开关、过期/无样本/真实零值语义，详情仍保留时间和样本信息。2026-10-29：监控和模板的空合并请求体按默认请求处理；覆盖模式仍要求非空对象，提交前校验当前 JSON，清空操作显式移除旧请求体。渠道状态分区的厂商图标与名称复用统一平台配色，适配深浅主题，Image 和未知厂商保持原有中性色。 | `backend/internal/service/channel_monitor_runner.go`、`backend/internal/service/channel_monitor_aggregator.go`、`backend/internal/service/channel_monitor_checker.go`、`backend/internal/service/channel_monitor_const.go`、`backend/internal/service/channel_monitor_template_service.go`、`backend/ent/schema/channel_monitor.go`、`backend/migrations/141_channel_monitor_stream_enabled.sql`、`backend/migrations/226_channel_monitor_quota_mode.sql`、`frontend/src/components/admin/monitor/MonitorAdvancedRequestConfig.vue`、`frontend/src/components/user/monitor/MonitorCardGrid.vue`、`frontend/src/components/user/monitor/MonitorCard.vue` | 生效中 |
| `CUST-OBS-002` | 独立上游站点同步监控 | 在二开管理页统一维护 Sub2API/New API 上游站点，定时同步余额、分组倍率与实际消耗，保留用量及倍率历史；Sub2API 同步 Token 与费用，New API 仅通过标准统计接口同步站点及分组费用，不再分页扫描日志，Token 指标明确标记为不可用并隐藏，既有 Token 数据保留但不再更新；支持按分组类型筛选、按余额/有效今日 Token 排序、拖拽调整站点顺序，并按平台优先级和倍率整理账号下方分组；上游分组可绑定本地账号并按最后一次有效倍率自动维护全局优先级，不可用分组仍参与排序，从未取得有效倍率的账号保留原优先级；新增 Sub2API 时会探测 Turnstile，自动改用令牌认证并支持从完整登录响应导入 Access/Refresh Token；令牌模式可在加密凭证中保存登录会话 User-Agent，并在验证、刷新与同步请求中保持一致；目标站点返回 `SESSION_BINDING_MISMATCH` 时会以 Chrome TLS/HTTP2 指纹重试，成功后将该传输模式随凭证加密保存，以兼容绑定出口 IP、UA 与 TLS/JA4 指纹的浏览器会话；Sub2API JWT Access Token 会在到期前 15 分钟主动轮换，非 JWT 令牌保持原有按认证失败刷新行为；New API 同时兼容旧版 Cookie 会话和 rc.22 起的短期 Bearer Access Token/刷新 Cookie，认证失败时优先刷新已有会话，刷新接口明确拒绝或旧版不支持时才回退密码登录，冲突及临时错误不再创建新登录会话；New API 还可直接使用个人中心生成的个人访问令牌，令牌模式不创建登录会话、不使用 Cookie/刷新令牌且认证失败时不会回退密码登录，管理页无需导入会话 User-Agent，后端自动使用固定浏览器 User-Agent 与 Chrome TLS/HTTP2 指纹兼容 Cloudflare，首次验证会从 `/api/user/self` 自动取得远端用户 ID；同步时优先复用加密保存的 Token/Cookie，仅在进入主动刷新窗口、凭证被拒绝或需要重新登录时更新认证。 本次上游适配：上游新增分组行锁与本地账号差量分组绑定、倍率绑定清理兼容；未变关系优先级和剩余账号重排仍受回归保护。 2026-10-29：New API 个人访问令牌模式支持可选的上游用户 ID，首次认证及后续同步携带 New-Api-User；ID 与凭据一同加密保存，管理响应仅回填 ID，不返回令牌。创建和更新校验正整数，更新省略时保留、空字符串显式清除；更换令牌未提交 ID 时清除旧 ID，显式提交值在会话清理后应用。上游明确要求 ID 时返回专用错误并保留表单重试，其他平台和密码模式不接收该字段。 | `backend/internal/service/upstream_service.go`、`backend/internal/service/upstream_provider_http.go`、`backend/internal/service/upstream_provider_newapi.go`、`frontend/src/components/admin/upstream/UpstreamManagementPanel.vue`、`backend/migrations/178_upstream_management.sql`、`backend/migrations/181_upstream_management_order_and_platform.sql` | 生效中 |
| `CUST-OBS-003` | 分组降智检测与作品画廊 | 默认关闭。管理员选择 OpenAI 分组、绑定自己的对应分组有效密钥，配置多模型、糖果/鹈鹕方式、推理程度与 5～1440 分钟间隔；端点只读系统公网 HTTPS 地址。任务先持久化，再通过满血AI创建与轮询；全局至多两个外部检测、同组不重叠，重启优先恢复查询，提交不明不盲目重试。两项结果独立存储，错误不直接判为降智；登录用户查看最近 24 小时结果。两层 sandbox 与独立 CSP 隔离作品，密钥和第三方任务 ID 不出现在用户接口。关闭后停止新提交并隐藏入口，继续回收已提交结果及清理过期作品；不改变网关路由、分组启停或生产配置。v0.1.256 采用固定 246 迁移适配，应用回退保留检测数据。2026-10-17 适配统一 Responses 协议，已知提交拒绝使用固定中文提示，不透出原始错误或替换所选模型。2026-10-26：紧凑画廊只显示两项判定结果，独立详情 `/model-quality/runs/:id` 展示作品、纯文本答案及执行信息；筛选、分页写入查询参数，返回恢复滚动。共享加载器合并请求，并发上限四；完成记录按 LRU 缓存最多 24 条/约 32MiB，有效期不超过五分钟及记录到期时间。400px 预加载，离屏/后台卸载实例，过期/退出/换号/关闭/权限失效清理内容并阻止旧响应回填。 | `backend/internal/service/model_quality.go`、`backend/internal/service/model_quality_runner.go`、`backend/internal/repository/model_quality_repo.go`、`backend/internal/pkg/manxue/client.go`、`frontend/src/views/user/ModelQualityView.vue`、`frontend/src/views/user/ModelQualityDetailView.vue`、`frontend/src/utils/modelQualityLoader.ts`、`frontend/src/stores/modelQuality.ts`、`frontend/src/components/admin/model-quality/ModelQualitySettings.vue`、`docs/model-quality.md` | 生效中，默认关闭；本次前端优化待发布 |

### 产品、支付与增长

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-PROD-001` | 每日签到转盘 | 支持余额、并发、订阅和空奖品，按万分比配置概率；支持未付费用户奖励衰减、LinuxDo 豁免、记录查询和防重复签到。默认关闭。 | `backend/internal/service/daily_checkin_service.go`、`backend/migrations/136_daily_checkins.sql`、`backend/migrations/146_daily_checkin_spin_rewards.sql`、`frontend/src/views/user/DailyCheckinView.vue` | 生效中 |
| `CUST-PROD-002` | 本地模型市场（`/models`） | 按指定分组公开模型、协议格式、价格和介绍，管理端可配置开关、介绍和分组范围。默认启用，但只返回公开白名单字段；与上游默认关闭的 `/model-plaza` 及面向 Codex 客户端的分组模型清单独立共存，二者均不替代本地入口。 本次上游适配：公开模型列表使用同组 model_allowlist，支持精确/通配筛选，启用后无匹配或遗留空配置不回填禁止模型；原公开分组范围与默认开关保留。 2026-09-24 上游适配：市场展示接入通用倍率并克隆配置映射；Fable 5.1继续无默认三倍，显式倍率仍可展示与配置。 2026-10-30 上游适配：接入模型广场分组模型价格优先级，保留本地模型市场与公开定价的既有组织方式。 | `backend/internal/handler/model_marketplace_handler.go`、`backend/internal/handler/openai_codex_models_handler.go`、`frontend/src/views/ModelMarketplaceView.vue` | 生效中 |
| `CUST-PROD-003` | 公开模型定价 | 提供独立定价 API/页面，展示充值倍率和模型费用；导航入口可以隐藏，但直达页面和 API 仍受公开设置控制。 | `backend/internal/handler/payment_handler.go`、`frontend/src/views/ModelPricingView.vue` | 生效中 |
| `CUST-PROD-004` | 批量图片任务 | 提供任务提交、队列处理、列表、明细、输出、下载、取消和清理；限制到允许的 Gemini 分组，包含余额预占、失败恢复和有界结算重试。 本次上游适配：批量图片请求准入和可选模型列表同步受分组白名单约束，历史任务处理及预占/结算规则不改。 | `backend/internal/service/batch_image.go`、`backend/internal/service/batch_image_worker.go`、`backend/internal/server/routes/gateway.go` | 生效中 |
| `CUST-PROD-005` | 兑换码多次使用 | 兑换码支持最大使用次数，并以用户维度记录使用明细，防止同一用户重复使用同一码。 2026-09-18 上游适配：新增用户兑换历史分页入口复用本地 redeem_code_usages 查询，保留多用户使用明细、事务和扫描逻辑，不退回只看最后 used_by 的历史；新增跨用户分页隔离回归。 | `backend/internal/repository/redeem_code_repo.go`、`backend/migrations/140_redeem_code_usage_limits.sql` | 生效中 |
| `CUST-PROD-006` | 支付和充值增强 | 支持余额充值赠送阶梯、手续费/倍率、赠送快照、自定义 EasyPay 支付方式、订阅 USD/CNY 换算预览和管理员删除订单；可配置专属倍率用户不参与余额充值赠送。EasyPay 会将 `mapi.php` 返回的站点根相对支付/二维码地址补全为可访问绝对地址；待支付订单在有效期内周期主动查单，通知丢失时复用幂等履约链路自动补发余额，并记录补查来源审计。 通用待支付补查扩展支付宝，同时排除由 EasyPay 独立路径处理的支付宝订单；EasyPay 微信继续走通用补查。独立路径保持30秒门槛、每轮最多10条/10秒预算、幂等履约及来源审计。 2026-09-18 上游适配：金额输入非法文本回退、支付配置并发等待及退款警告修复与本地充值赠送共存；赠送角标和上游输入校验测试同时保留。 2026-10-07 安全同步：清除客户端 return_url 查询参数，并在 EasyPay 通知验签前拒绝非标准回调参数，阻断下单签名复用；保留主动查单中 WAITING 优先于 status=1、30秒/10条/10秒预算、赠送快照及幂等履约。本轮不修改历史余额、订单或封禁状态。 2026-10-16 上游适配：EasyPay 成功回调须查单确认，失败码、商户、订单和非有限金额拒绝；保留 WAITING/空状态保守判定、嵌套字段、赠送快照、30秒门槛/10条/10秒补查与幂等履约。退款待确认保留扣款，失败终态原子归还并失效缓存。 | `backend/internal/service/payment_amounts.go`、`backend/internal/service/payment_order_lifecycle.go`、`backend/internal/payment/provider/easypay.go`、`backend/migrations/149_payment_balance_bonus_rules.sql`、`backend/internal/service/payment_resume_service.go` | 生效中 |
| `CUST-PROD-007` | 站内无限画布 | 基于 `basketikun/infinite-canvas@b66936d891b82c2b51c1ed05e1a6eae3e31d4ca3` 的 React 画布裁剪集成，保留项目、素材、提示词、文本/图片/视频节点、连线、缩放平移和受控画布助手；移除上游 Key 配置、WebDAV、本地 Agent/MCP、远程插件与脚本、推广及外链入口。Vue 通过同源沙箱 iframe 嵌入 `/canvas-app/`，默认开启的功能开关、JWT 父页面消息桥和按用户隔离的浏览器存储共同约束访问；运行时按用户可用分组复用普通 Key 或懒创建唯一托管 Key，切换分组后的重试只使用当前分组真实模型并展示上游错误，图片默认生成 1 张。OpenAI GPT Image 使用官方输出字段，Grok 图片兼容层将 OpenAI 尺寸转换为 xAI 宽高比和 1K/2K 分辨率，多图编辑只发送互斥的 `images`，不支持的遮罩明确拒绝；Gemini/Antigravity 图片及图片编辑使用 `streamGenerateContent?alt=sse`、`generationConfig.imageConfig` 并从流中提取图片，隐藏只能调用 `predict` 的 Imagen 模型。OpenAI 视频使用 `/v1/videos` multipart、单数 `input_reference` 和官方时长枚举；Grok 视频使用 `/v1/videos/generations` JSON，兼容 `request_id`、`done`、`video.url` 及旧 OpenAI 字段转换。生成结果在浏览器本地安全落盘，视频优先从站内内容接口下载；所有媒体请求继续经过站内余额、订阅、分组计费和并发控制，Gemini 首版不提供视频生成。 | `canvas/`、`frontend/src/views/user/CanvasView.vue`、`backend/internal/service/api_key_service.go`、`backend/internal/service/grok_media.go`、`backend/migrations/222_infinite_canvas_api_keys.sql`、`canvas/THIRD_PARTY_NOTICES.md` CI 合入前将间接 YAML 解析依赖 js-yaml 4.x 的漏洞范围锁定到 4.3.2 补丁版，不改变画布默认值、接口和数据格式；不增加安全扫描豁免。 2026-10-03 发布门禁适配：仅使用的 shadcn 4.18.0 静态 CSS 连同 MIT 许可固定保留到仓库，移除未使用的 CLI 及 braces 依赖链；画布默认值、交互和构建产物不变。 2026-10-25：将间接 DOMPurify 和 follow-redirects 定向覆盖到 3.4.17 和 1.16.1 安全补丁，画布功能开关、接口和数据格式保持不变。 | 生效中 |

| `CUST-PROD-008` | 用户定制公开余额查询 | 管理员按用户生成唯一长期查询链接 `/balance-query#随机码`，默认不生成；32字节安全随机数，仅存摘要及加密原文，可复制、停用、启用、重置，重置立即废止旧链接，独立于授信开关。页面不恢复登录态，兼容后台模式，仅公开用户名（空时用ID）、ID、余额、安全充值历史和全局有序纯文本充值方式（最多20项、键50字、值500字）；记录默认20条、最多50条分页，仅已到账余额兑换/充值、管理员正向加款和临时授信，普通来源固定备注，授信显示实际入账金额。接口不接受用户ID或URL令牌，仅专用头传码；IP每分钟30次、有效链接60次，429附Retry-After，Redis故障503。页面和接口no-store/noindex/no-referrer，无登录凭据、自动轮询及账户写入，停用/删除用户与失效链接统一不可用。2026-09-20 后台用户定制投影新增邮箱，供管理员搜索和识别；公开 DTO 和余额页仍不包含邮箱，用户名为空仍仅以 ID 兜底，不修改用户资料、余额、授信或链接规则。 | `backend/internal/handler/balance_query_handler.go`、`backend/internal/service/user_customization.go`、`frontend/src/views/public/BalanceQueryView.vue`、`frontend/src/components/admin/user/UserCustomizationsPanel.vue` | 生效中 |

### 风控与内容审核

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-RISK-001` | 本地人工审核对话 | 内容审核可进入本地人工审计，持久化请求/响应记录；管理端支持列表、详情、下载和删除。统一安全审计协调器接入上游 prompt audit，协调器未配置时继续使用本地内容审核，审计服务过载时按配置执行回退。 2026-09-20 上游适配：OpenAI/TypeSafe 独立引擎配置与全部 LocalAudit 字段共存；保存、切换引擎、快照克隆和前端草稿保留审计开关、场景、容量、图片排除、客户端/工具规则及捕获并发。默认引擎仍为 OpenAI，日志新增可空 engine_meta。 2026-09-30 上游适配：风险用户白名单默认空；显式加入后内容审核仍留证据但不处罚或通知，不绕过独立 prompt audit、本地人工审核及既有作用域。 2026-10-25 上游适配：Chat/Responses 共用用量提交闭包后仍单独保留成功且未断连请求的对话审计；失败和断连不写成功审计。 | `backend/internal/securityaudit/coordinator.go`、`backend/internal/service/content_moderation_local_audit.go`、`backend/internal/handler/security_audit_helper.go` | 生效中 |
| `CUST-RISK-002` | Cyber 会话阻断 | `cyber_policy` 命中可沿网关、审计和计费链路透传，并按配置对会话做 TTL 阻断；Responses `response.failed` 先解析 usage 再标记会话，WSv2 透传会在 `AfterTurn` 前设置标记，避免错误终态漏记 Token，用量记录允许 `cyber_blocked` 类型。事件只有在全局风险控制、内容审核、非 `off` 模式以及 group/model scope 均允许时才记录；运行时快照读取失败时安全跳过事后副作用，不扩大阻断范围。 WS 各路径标记与同一逻辑轮次的失败转移链去重，两个原子状态分别保存已阻断与失败转移待判定；不扩大风控 scope，保留失败终态的部分 Token。 2026-09-15 上游适配：WS 作用域与抢占更新后仍逐轮执行 Cyber 策略、失败去重与部分 usage 记录，不扩大作用域。 2026-09-20 上游适配：保留 cyber_policy_exclude_from_ban_count、作用域校验和事件副作用；该配置不随审核引擎切换重置，既有 Cyber 日志不伪造第三方审核引擎来源。 2026-09-30 上游适配：风险白名单用户的 Cyber 在原开关与作用域内只留日志、不新增会话阻断/处罚/通知，不能绕过风控/审核关闭、off模式及group/model scope；非白名单仍遵守原作用域和 EmailOnHit，WS 两个原子状态及逐轮失败去重保留。 | `backend/internal/service/content_moderation.go`、`backend/internal/service/openai_ws_v2_passthrough_adapter.go`、`backend/internal/handler/security_audit_helper.go`、`backend/migrations/174_allow_cyber_blocked_usage_request_type.sql` | 生效中 |
| `CUST-RISK-003` | 错误请求详情可见性 | 管理端运维详情支持从账号、用户和请求上下文继续导航；可按设置允许用户查看自己的错误请求详情。 | `frontend/src/views/admin/ops/components/OpsRequestDetailsModal.vue`、`backend/internal/service/setting_user_error_view_test.go` | 生效中 |

### 前端与管理体验

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-UI-001` | 独立二开管理页 | 将上游同步监控、模型广场、每日签到、网关运行策略和用户定制集中在 `/admin/custom-features`，与上游通用设置隔离；页面使用宽屏管理台布局展示高密度数据；上游管理始终排在第一个并默认选中，其余页签保持相对顺序。用户定制独立页签支持搜索用户、余额展示、全局充值方式增删排序、专属查询链接管理和授信配置；重置链接与恢复授信均需确认，恢复窗口显示用户、阈值和单次金额，沿用管理员鉴权、写幂等及敏感读取审计。2026-09-20 搜索统一为用户名和邮箱不区分大小写的包含匹配、ID 精确匹配，去除首尾空白，提交时回到第一页，列表和总数共用过滤条件并排除软删除用户；不搜索备注或 API Key。列表、编辑和确认窗口优先显示用户名，空时显示邮箱，始终保留 ID；有用户名时附带邮箱。 2026-10-15：在网关配置和每日签到之间新增独立降智检测页签，配置全局开关及分组检测参数。 2026-10-25 上游适配：中英文词典增加管理密钥词条，同时保留 custom 的递归合并。 | `frontend/src/views/admin/CustomFeaturesView.vue`、`frontend/src/components/admin/user/UserCustomizationsPanel.vue`、`backend/internal/server/routes/admin_custom_features.go`、`backend/internal/repository/user_customization_repo.go` | 生效中 |
| `CUST-UI-002` | 统一功能开关注册表 | 公共设置驱动的入口统一声明 opt-in/opt-out 语义，避免 SSR 注入缺字段导致刷新闪烁或错误隐藏；渠道 quota 展示和插件管理均为 opt-in，字段缺失或 false 时不暴露对应入口，Canvas、每日签到和本地模型市场继续保持各自既有默认语义。 本次上游适配：新增上游渠道监控排名开关时保留 Canvas、签到、本地市场、quota 和插件的原开关语义。 2026-09-15 上游适配：订阅为 opt-out，字段缺失时启用；关闭订阅的路由/侧栏处理与 Canvas、签到、本地模型市场及公开余额路由共存。 2026-10-15：降智检测使用默认关闭的 opt-in 标记，同步 SSR、侧栏、登录路由与后端数据访问。 | `frontend/src/utils/featureFlags.ts`、`frontend/src/components/layout/AppSidebar.vue`、`backend/internal/service/setting_public.go` | 生效中 |
| `CUST-UI-003` | 机甲冷蓝主题与浅色默认 | 保留自定义认证背景、扫描线、计数、倾斜等视觉组件，默认使用浅色主题并提供暗色兼容。 2026-09-15 上游适配：新配额卡继续使用本地 CountUp、PulseDot 与动效组件；桌面和 390px 移动端以隔离夹具检查，无横向溢出。 | `frontend/src/components/common/BackgroundFX.vue`、`frontend/src/composables/useTilt.ts`、`frontend/src/stores/app.ts` | 生效中 |
| `CUST-UI-004` | 表格和导航稳定性 | 保留账号列表禁用虚拟化、查询后滚动重置、分页大小持久化、侧边栏滚动位置，以及日期和列设置下拉浮层不被表格 sticky 表头覆盖；用量窗口统一展示，隐藏的缓存 tooltip 不再撑出横向滚动，账号到期时间按本地时区严格解析。 账号列表接入 lite DTO 和编辑时详情回取；本地禁用虚拟化、分页/滚动和列设置 Teleport 浮层保留。请求 ID/上游 ID 默认隐藏，测试卸载浮层实例避免跨用例串扰。 本次上游适配：账号操作菜单、用量密钥分页和清单编辑即时更新纳入，保留 DataTable 非虚拟化及本地浮层与列配置。 2026-09-15 上游适配：账号创建复用本地 upstreamModelSyncPreviewRequest；OpenCode adaptive 协议进入同一预览路径，模型白名单测试对齐本地接口并保留原断言。 2026-09-18 上游适配：API Key 批量编辑与本地列设置入口同时保留，不以新增批量弹窗替换列配置。 2026-09-24 上游适配：日期弹层保留 Teleport、固定定位及窄屏边界，同时接入关闭放弃草稿、应用后保留和跨午夜刷新。 2026-10-09 上游适配：接入选择器、弹窗及异步页面的过期响应隔离，保留用量表自定义列、导出进度和本地主题。 2026-10-11：账号筛选取消折叠，平台、状态、类型、Privacy 和分组默认展示；用量统计缓存明细及账号、分组、用户、余额、接口地址、运维诊断共 33 处提示统一使用页面根节点浮层，公共组件按视口边界换向、限宽和滚动，保留键盘与复制交互；窄屏统计卡片改为单列避免数值截断。 2026-10-13 上游适配：Gemini 配额条件显示、多语言分组及新缓存开关合入，保留根节点 HelpTooltip、账号常显筛选、失败策略与 Pool 继承。 2026-10-16 上游适配：上游恢复账号内联筛选时保留本地常显、窄屏两列和可访问名称，不删除筛选测试。 | `frontend/src/components/admin/account/AccountTableFilters.vue`、`frontend/src/components/common/HelpTooltip.vue`、`frontend/src/components/admin/usage/UsageStatsCards.vue`、`frontend/src/components/common/DataTable.vue`、`frontend/src/components/common/ColumnSettingsDropdown.vue`、`frontend/src/components/layout/TablePageLayout.vue`、`frontend/src/components/layout/AppSidebar.vue` | 生效中 |
| `CUST-UI-005` | 认证页加载与存储容错 | 注册页等待公共设置后再开放输入和提交，登录协议、Captcha 与 OAuth 开关使用同一设置快照；local/session storage 不可用时安全降级，刷新令牌并发时防止旧会话覆盖换号后的新会话；仅从 auth store 恢复 pending OAuth 时不会复用普通邮箱注册残留的邀请码。 本次上游适配：注册入口可见性和验证码加载状态纳入；本地认证存储容错保留，SDK 夹具显式按 host:port 绕过代理。 2026-09-18 上游适配：确认密码和缓存公开设置接入现有注册加载保护，确认密码输入及显示切换沿用 registrationInputDisabled；加载容错与缓存设置两组 mock 同时保留。 | `frontend/src/views/auth/RegisterView.vue`、`frontend/src/views/auth/EmailVerifyView.vue`、`frontend/src/stores/auth.ts`、`frontend/src/api/tokenRefresh.ts`、`frontend/src/utils/browserStorage.ts` | 生效中 |
| `CUST-UI-006` | 自定义页面身份透传保护 | 自定义菜单逐页分别配置 `pass_user_id` 和 `pass_auth_token`，只有显式为 true 才附加当前用户身份；新增菜单及缺少字段的旧配置默认关闭。iframe 与新窗口共用同一构造器，普通用户和管理员专属菜单遵循相同规则；来源 `src_url` 始终移除查询参数及片段，保留目标 URL 自有参数和展示上下文。后端 DTO、管理保存和公开配置保留开关，管理端提示完整登录 Token 可读取 API Key，仅为完全可信页面开启。选择性适配上游未合并 PR #8125 的固定提交，不改变完整同步基线；Markdown、隐藏按钮和拖动行为保留。 | `frontend/src/utils/embedded-url.ts`、`frontend/src/views/user/CustomPageView.vue`、`frontend/src/views/admin/SettingsView.vue`、`backend/internal/handler/dto/settings.go` | 已随 v0.1.259 发布 |

### CI、发布与生产约束

| 编号 | 功能 | 当前行为与边界 | 关键入口 | 状态 |
| --- | --- | --- | --- | --- |
| `CUST-OPS-007` | 蓝绿兼容字段迁移门禁 | 普通规则仅允许绑定旧提交、SQL和旧代码摘要的新增可空字段，238b已随v0.1.243发布。2026-09-24经用户追加批准，239/240采用固定SQL摘要专项，不放宽普通门禁、不改上游SQL。专项限PostgreSQL18、无继承且至多16MiB的普通表；未知用户触发器拒绝，仅按定义及函数摘要接纳184/193既有groups鉴权缓存触发器。NOWAIT表锁及1秒锁等待/5秒语句/10秒事务预算；失败不自动重启候选或切流。239仅允许无显式倍率，事务内安装检查约束，临时禁止新旧实例共存期写入推理倍率，普通价格可写；切流/回滚前核对保护，旧槽退役并重新安全核验后自动解除，回滚或失败保留。240核验可空操作ID及部分唯一索引。已提交迁移不反向删除，不承诺零延迟或无损强制退役。239/240已随v0.1.246发布，旧槽退役后保护已解除，验收见本轮发布记录。 2026-10-03 用户批准241专项：固定旧版、SQL及Ent差异摘要；赠金列仅核验既有定义并登记，TypeSafe约束扩展期间加写入保护，旧槽退役后经身份/挂载/健康复核解除；NOWAIT及1/5/10秒预算不放宽。 241已随v0.1.249由Actions发布，2026-10-04旧槽回收后保护已自动解除；本轮发生超时强退，用量完整性未知，详见本轮验收记录。 2026-10-09 上游适配：原样合入 242 平台 CHECK 移除迁移，但不扩大蓝绿兼容放行规则；该破坏性结构变化仍为未来生产发布阻塞，须另行审批。 2026-10-10 用户批准242专项：固定v0.1.251旧提交、原SQL和两处Ent摘要，按旧版十一平台同事务安装临时CHECK后执行原242；旧槽退役且身份、挂载、配置与双入口复核后解除。失败/回滚保留，普通迁移规则不放宽；PostgreSQL18、16MiB、NOWAIT双表锁及1/5/10秒预算继续生效。 2026-10-13 上游适配：四条242至245缓存/多语言迁移保持上游原文，仅做本地隔离新建和升级验证；不修改通用蓝绿门禁或新增生产审批。 2026-10-14 按用户tag自动蓝绿发布要求补齐0.2.16固定扩展：绑定v0.1.254、四条原SQL及分组Ent摘要，四条迁移与账本在NOWAIT及1/5/10秒预算内原子提交；拒绝同名对象、部分账本及结构漂移，旧分组读写保留。 2026-10-15增加固定246专项，仅绑定v0.1.255和原SQL，新增检测专用表及关闭开关；设置/账本NOWAIT短事务、完整结构指纹及重复启动重验，详见本轮发布记录。 2026-10-26新增固定246生图开关/247管理密钥专项，绑定v0.1.259及两条原文、旧代码摘要；三表NOWAIT短事务将回填、新表和账本原子提交，完整结构重验且不重复回填后续管理员开关，保留16MiB与1/5/10秒预算。 2026-10-30 上游适配：本轮仅合入原始 248_group_image_count_billing.sql；生产蓝绿迁移门禁不放宽，发布兼容适配仍须独立任务处理。 | `deploy/blue-green/evidence.py`、`deploy/blue-green/expand-approvals.json`、`deploy/blue-green/pricing-guard.py`、`backend/internal/repository/migrations_blue_green_special.go`、`docs/operations/blue-green-expand-migrations.md`、`backend/internal/repository/migrations_blue_green_241.go`、`deploy/blue-green/platform-guard.py`、`backend/internal/repository/migrations_blue_green_216.go`、`backend/internal/repository/migrations_blue_green_242.go`、`deploy/blue-green/platform242-guard.py` | 发布门禁 |
| `CUST-OPS-001` | 标签发布与生产自动部署 | `v*` 标签构建发布产物、Docker Hub/GHCR 镜像并自动 SSH 部署；完整多架构发布允许最长 120 分钟，包含连接诊断、重试、健康检查和版本文件回写。 2026-09-14 生产部署目标迁至216.152.153.86，后续按用户要求将SSH改为22748，工作流默认值、强制校验与GitHub DEPLOY_PORT同步修改，独立部署密钥经重启后验证；旧机禁止再次部署业务。2026-09-16新增用户授权的一小时观察及超时强制退役策略，生产job为120分钟，固定实例身份、Redis归属清理、游标恢复和强制结果独立审计；已手动回收green，后续tag须包含该工作流变更，不宣称强制请求无损。 2026-09-20 上游适配：接入独立构建矩阵与 dry-run，构建/镜像/部署绑定 prepare 的固定 SHA；保留 DockerHub secrets/vars 回退、凭据门禁、Canvas 构建、120分钟超时和3600秒蓝绿观察。dry-run 禁止版本回写及部署，simple 模式禁止部署，正常发布默认部署策略不变。本轮仅本地验证，不触发发布。 2026-09-24 上游适配：月度归档与本地备份生命周期组合；记录写入有数据库时沿用 backup:records advisory lock（含共享数据库回退），恢复写入保留实例归属，未知或存活实例不能仅凭超时判失败，Stop仍等待活跃任务。 | `.github/workflows/release.yml`、`deploy/blue-green/deploy.py`、`deploy/remote-deploy.sh` | 运维约束 |
| `CUST-OPS-002` | 保留生产 Compose | 自动部署默认不上传仓库 Compose，只更新远端 `.env` 镜像标签并使用活动 Compose，防止通用命名卷配置覆盖生产文件。 本次上游适配：本次仅本地更新两份 Compose 的图片主控模型环境变量，不上传文件或改变生产活动配置。 2026-09-15 上游适配：本次仅在本地同步两份生产 Compose 的 compact 默认值为 gpt-5.5；字节一致约束保留，不上传或变更生产配置。 2026-09-24 上游适配：两份生产Compose同步增加简易模式分组初始化和Key窗口开关，继续逐字一致，保留绑定目录、回环端口及资源入口。 2026-09-30 上游适配：Redis command 改为参数列表，两份生产Compose同步且字节一致；保留 bind mount、maxclients、回环监听和所有资源入口。 2026-10-07 上游适配：仅本地同步两份生产 Compose 的 ADMIN_EMAIL 空默认值，由首次安装逻辑生成管理员邮箱；绑定目录、回环监听、性能参数、HTTP upstream 许可和双文件一致约束保持，不上传或改动线上配置。 | `.github/workflows/release.yml`、`deploy/remote-deploy.sh`、`deploy/docker-compose.yml`、`deploy/docker-compose.sub2api.yml` | 运维约束 |
| `CUST-OPS-003` | 生产数据与网络拓扑 | 生产实例必须保持 bind mount 数据目录、仅回环地址暴露、两个活动 Compose 一致，以及业务要求的 HTTP upstream 安全开关；Compose 接入上游健康检查变化并透传实例级数据保留参数，具体保留天数、实例专用路径和连接参数只保存在不跟踪的 `.env` 与本地运维说明中。 2026-09-05 发布门禁在变更版本/重建前强制校验两份Compose字节一致、应用/PostgreSQL/Redis的精确bind mount和旧实例健康；Release配置只允许服务器1及指定用户、端口、目录和活动文件。不满足时失败退出，不修改.env或重建容器。 本次上游适配：两份 Compose 字节一致，增加 SUB2API_IMAGES_MAIN_MODEL（默认 gpt-5.6-luna）；bind mount、回环监听和 HTTP upstream 约束不变。 2026-09-14 已完成 PostgreSQL/Redis 单写迁移、商店 SQLite 一致性复制及10条 A 记录切换；旧机只保留固定新 IP 的 Nginx 转发，停用数据库、Docker 与业务定时任务。新端写入后禁止直接启动旧库回退。后续经授权完成系统包和内核更新，SSH仅22748且保留密码登录，Fail2ban封禁1小时；BBR/CAKE保留用户配置。脱敏主机配置归档于deploy/operations/production-20260914。 2026-09-16 经授权平滑调整Nginx为worker_connections=16384、multi_accept=off，修复当前epoll接入轮转偏斜，保留旧worker自然排空与应用资源上限；实测证据见docs/operations/2026-09-16-nginx-worker-balance.md。 | `deploy/docker-compose.yml`、`deploy/docker-compose.sub2api.yml`、`deploy/remote-deploy.sh`、`.github/workflows/release.yml` | 运维约束 |
| `CUST-OPS-004` | 本地 CI 与安全门禁 | `main` 的 Push/所有 PR 运行 Go 1.27.3 单元和集成测试、Node 20 前端及 Canvas 测试、golangci-lint 2.14.0、部署脚本检查及依赖安全扫描；gosec G703/G704 不再全局排除，生产代码逐点说明，测试文件按明确路径豁免。发布前以远端 CI 为最终门禁；本地工具版本低于目标 Go 版本时必须如实标记静态检查未验证。 新增12项隔离部署脚本回归并接入macOS shell job，覆盖禁止清单、不一致清单、命名卷/错误目录、重建前健康失败及重建后健康失败；无检查工具不能伪报成功。 本次上游适配：新增上游 i18n 完整性门禁与本地 featureFlags 门禁并存。ZIP 安装前关闭与本地幂等清理共存；PG 备份迁移锁以本地 mock 验证，未执行真实备份/恢复。用户定制管理、公开余额查询客户端与页面、使用记录表格及输出速率的5个前端测试文件纳入固定CI关键测试清单。Claude 透传流安全重试与精确空闲计时纳入 Linux 定向竞态检查，配置 API、流诊断与错误详情组件纳入关键前端测试；PR #22 安全扫描发现的 gRPC 漏洞通过最小修复版本升级解决，不增加豁免。 2026-09-18 上游适配：新增 API Key 批量编辑关键测试清单与既有门禁并存；保留本地 gRPC 1.83.2 安全版本、validator 直接依赖及 Wire/Ent 工具依赖，不增加安全豁免。 2026-09-20 上游适配：发布辅助脚本14项测试与原有流竞态、生命周期、蓝绿门禁同时保留；新增审核引擎与本地字段的交叉回归、响应别名与用量元数据断言，未删除断言或增加安全豁免。 2026-09-24 上游适配：上游新增用例与本地交叉回归同时保留；动态UA、旧版备份锁、超时存活任务、终态错误、定价白名单、日期与CSV验证纳入本轮记录。 2026-09-30 发布准备：为消除Canvas既有安全门禁失败，将fast-uri的v3受影响版本定向覆盖到3.1.8，保留锁文件和审计门禁，不添加豁免或跨主版本升级。 2026-09-30 后续审计：Canvas的undici 7.29.0新检出两项high，定向覆盖到兼容的7.29.1；保留失败证据，不加豁免，以新标签补充发布。 2026-10-03 发布前适配：Canvas定向升级axios至1.20.0、brace-expansion至5.0.11；保留安全扫描及现有豁免边界，补跑unit标签测试，不用默认go test结果替代CI。 后续braces审计失败通过移除未使用的shadcn CLI依赖链解决，保留等价静态样式；不转移到开发依赖、不新增或延长豁免。 2026-10-07 经用户批准合入 Vue 3.5.43 与 source-map-js 安全修复，保留本地 axios/nanoid 约束；仅将两项既有 XLSX 豁免延至2027-01-06，基于仅导出、不读取外部表格的当前使用边界。这是明确风险接受，不是 XLSX 漏洞修复；其他豁免不延期，安全门禁和原有回归断言继续保留。 2026-10-09 上游适配：工具链与容器构建升级 Go 1.27.2、golangci-lint 2.14.0，保留本地关键前端与竞态用例，不增加审计豁免。 2026-10-10 用量优化：CI 与安全扫描仅接受 main 的 Push，保留全部 PR 和手动运行；每个工作流按 PR 编号或 ref 取消过期检查，安全扫描保留每周定时。完整 8 项 CI、2 项安全检查及 macOS 覆盖不减少；发布与生产运维的触发及取消规则不变。新增触发/并发/覆盖边界回归并复用现有辅助检查作业。 2026-10-13 上游适配：新增模型白名单关键回归合入；保留本地安全审计例外范围，安全侧枝无依赖净升级，不新增或延长豁免。 2026-10-25 上游适配：构建及三份工作流同步 Go 1.27.3；更新 DOMPurify、follow-redirects，保留本地 nanoid 3.3.18 覆盖。 2026-10-25 PR #44 安全门禁修复：Canvas 间接依赖 DOMPurify 3.4.13 定向升级至 3.4.17、follow-redirects 1.16.0 至 1.16.1；保留审计检查和既有豁免范围，未新增或延长豁免。 2026-10-29 PR #47 安全门禁修复：将 x/net 从 0.60.0 升至修复 GO-2026-6860 的 0.62.0，按模块约束联动更新 x/crypto、x/sys、x/term、x/text、x/tools、x/sync，保留 Go 1.27.3 与完整扫描，不增加豁免；上游用户 ID、监控请求体保存和账号选择回归加入前端固定 CI 清单。同日因 GitHub ARM64 容量不足，将脚本检查切至 macos-15-intel，保留 macOS 15 与全部回归及执行器断言。 | `Makefile`、`.github/workflows/backend-ci.yml`、`.github/workflows/security-scan.yml`、`backend/.golangci.yml`、`.github/audit-exceptions.yml`、`frontend/package.json`、`canvas/package.json`、`canvas/pnpm-lock.yaml`、`tools/test_workflow_usage_policy.py` | 运维约束 |
| `CUST-OPS-005` | 生产资源保护与调优 | Compose 透传 Go 内存、用量 worker、数据库连接与并行查询参数，并为应用、PostgreSQL 和 Redis 配置有界 Docker 日志轮转；上游健康检查变化不得移除这些变量入口。实例参数保存在未跟踪的 `.env`、`deploy/data/config.yaml` 与本地运维说明中。服务器1升级至 12 vCPU、约 32 GiB 内存后，仅按实例覆盖 Go 并行度、Go 软内存限制、PostgreSQL 共享缓存和有效缓存估值；保持数据库连接池、用量 worker/队列、按用户串行扣费、5 秒用量任务超时及其他性能参数不变，具体实施与回滚见 `docs/operations/2026-09-07-resource-tuning.md`。服务器1在持久化配置中将通用 `gateway.response_header_timeout` 显式设为 3600 秒，覆盖后端及通用示例的 600 秒默认值；应用升级或容器重建不得丢失此覆盖。该参数只限制等待上游响应头，不是请求总时长，也不是分组专属；OpenAI/Codex、Grok 的独立响应头超时、流式数据间隔超时及 5 秒用量任务超时保持不变。 2026-09-14迁移阶段先保持原参数，后续经用户明确授权按256逻辑CPU设置Go并行度和512连接池，Go软内存上限为总内存80%，PostgreSQL共享缓存25%、有效缓存估值50%、最大连接600；新增POSTGRES_SHM_SIZE入口且默认2g。保留原用量worker、串行扣费与超时，修正Nginx master文件上限并持久化Redis overcommit=1。已明确记录80%+25%预算叠加的OOM风险，不宣称峰值容量安全。2026-09-15按用户要求将 `deploy/config.example.yaml` 的 `gateway.stream_data_interval_timeout` 示例值从500秒调整为300秒；只影响采用该示例配置的部署，后端缺省仍180秒，不修改生产实例配置、响应头超时或5秒用量任务超时。 2026-09-15 上游适配：上游 WS 连接池系数默认 5 与容量计算合入；保留生产资源入口、日志边界、用户串行扣费、5 秒 usage task、3600 秒持久化响应头超时约束及本地 300 秒流间隔示例。 2026-09-15 后续本地修复：结合近三个完整北京时间自然日的Claude通道统计，将此前300秒流间隔示例统一为后端既有默认180秒，并添加一致性测试；只修正透传计时精度，不进一步缩短阈值或变更生产实例配置。 2026-09-30 上游适配：Redis 参数列表迁移保留 REDIS_MAXCLIENTS，生产实例资源、3600秒响应头等待及5秒 usage task 约束未改；本次不连接生产。 | `deploy/docker-compose.yml`、`deploy/docker-compose.sub2api.yml`、`deploy/.env.example`、`deploy/config.example.yaml`、`backend/internal/config/config.go` | 运维约束 |
| `CUST-OPS-006` | 禁止网页自更新 | 禁止网页更新、下载、回滚、重启和自动同步代码。已移除的 `/api/v1/admin/system/` 下 `check-updates`、`rollback-versions`、`update`、`rollback`、`restart` 路由继续返回 404。2026-09-23 允许新增管理员只读 `upstream-version` 查询及发布页提示；`version` 仅返回当前构建和已同步来源，不访问 GitHub。新检查服务只获得最新 Release 查询能力，不恢复旧更新服务、操作锁或执行入口。共享 GitHub 客户端和首次安装使用的重启工具保留；实际升级仍走定制版标签发布和蓝绿部署流程。 | `backend/internal/handler/admin/system_handler.go`、`backend/internal/server/routes/admin.go`、`backend/internal/service/upstream_version.go`、`backend/internal/service/github_release.go` | 运维约束 |
| `CUST-OPS-008` | 构建版本与上游来源提示 | 当前二开版本来自运行程序构建信息，启动时统一注入 HTML、公共设置及管理员接口；旧首屏配置缺版本时仅补查一次公共设置。管理员显示二开版本、已同步原版版本及原版更新状态，普通用户只看二开版本。来源由随二进制嵌入的 `upstream-sync.json` 记录，缺失时显示未记录，不与二开版本比较。最新正式 Release 只读检查采用8秒超时、30分钟成功缓存、60秒失败退避和并发合并；失败保留旧结果并标记缓存，无历史显示暂不可用。可见页定时检查，后台暂停，退出或换用户清理请求和计时器；支持折叠详情、移动端、深浅主题与长版本换行。 2026-09-24 上游适配：随合并更新嵌入来源为0.2.8/a3eb7ef302961cba716dc78b39b93b60c467db0e，二开版本保持0.1.245；只读提示、缓存、身份差异及禁止自更新边界不变。 2026-09-30 上游适配：嵌入来源更新为0.2.10/a60a29549f488a854966aaec9541abbe006cac22，本地二开版本保留0.1.246，不创建发布或开放自更新。 2026-10-07 上游适配：嵌入来源更新为0.2.14/3f1a2ea0a760730e3bc528105c00b4ee4f23e469；本地版本仍为0.1.249。本轮仅本地合并，不创建标签、发布或部署，版本提示的身份隔离、缓存与禁止自更新边界不变。 2026-10-09 上游适配：本地版本保持 0.1.251，上游来源更新为 0.2.15 和固定 SHA 3a6fd1c9db07203ca308aaba69e502bc1f35b307，不改发布标签。 2026-10-13 上游适配：来源更新为0.2.16/4d9ac3e482106d2da3a39fe3e546d8d6801f1cc1，本地版本保持0.1.254，禁止网页自更新边界不变。 2026-10-16 上游适配：二开版本保持0.1.256，构建来源更新为上游0.2.17及固定提交b0a02a330；来源仍由祖先关系校验。 2026-10-25 上游适配：本地版本保持 0.1.259，嵌入来源更新为上游 0.2.18/3f1c8f36ce8c2cd378015830a90e89970b9a9ea7。 2026-10-30 上游适配：来源元数据推进至上游 0.2.19 / a3298b46e，本地产品版本保持 0.1.261。 | `backend/internal/pkg/buildmeta/upstream-sync.json`、`backend/internal/handler/wire.go`、`backend/internal/service/upstream_version.go`、`frontend/src/composables/useVersionInfo.ts`、`frontend/src/components/common/VersionBadge.vue`、`tools/check_upstream_sync_metadata.py` | 生效中 |

## 变更记录

### 2026-10-29 Anthropic 采样过滤前置及分组控制（0.1.261）

- `CUST-PROTO-008`：修复采样字段在 Claude 5.5 模型校验后才被删除，导致命中白名单的请求仍被本地 400 拦截的问题。各入口使用最终上游模型，协议转换前清除三个采样字段，校验和出站共用单次尝试策略。
- 增加 Anthropic 停用分组列表及独立的 thinking/tool_choice 校验跳过开关；默认列表为空、两个开关均关闭，分组停用时恢复原严格行为。既有总开关和白名单保持原值。
- 转换器保留严格默认接口，仅显式选项放宽模型限制；既有格式检查、必要协议转换和上游错误处理不变。每次换号和切组重新判定，不从账号的多个分组推测请求来源。
- 设置使用现有表持久化并深拷贝缓存；旧管理客户端省略字段时保留原配置。界面支持停用中的 Anthropic 分组及历史失效选择，不影响其他功能的活跃分组列表。无数据库结构迁移。
- 新增入口实际出站、模型别名、分组隔离、独立开关、转换器严格默认、设置缓存、部分更新和界面回归测试。CI、合并及蓝绿发布结果以对应 PR、Release 和运维验收记录为准。

### 2026-10-29 PR #47 macOS 执行器容量适配

- 关联 `CUST-OPS-004`，随本次修复提交。CI 作业 `123015827332` 因 GitHub macOS ARM64 容量不足，多次未能分配 hosted runner 后被取消；同一提交其余 7 项 CI 与 2 项安全检查均通过。
- 将脚本检查从 `macos-15` 调整为同版本的标准 Intel 执行器 `macos-15-intel`，同步更新执行器断言。完整脚本集合、macOS/BSD 行为覆盖、8 项 CI 与 2 项安全检查及生产操作边界保持不变。
- 验证：工作流策略回归与完整远端门禁随 PR #47 重验；最终提交须全部通过后才能合入，不把执行器容量失败计作测试通过。

### 2026-10-29 PR #47 后端安全依赖修复

- 关联 `CUST-OPS-004`，随本次修复提交，待发布。首轮 Security Scan 的作业 `123009192235` 检出 `GO-2026-6860`：现有 Ollama 用量解析调用 `golang.org/x/net/html.Parse`，受文本节点合并的二次复杂度问题影响。
- 将 `golang.org/x/net` 从 `v0.60.0` 定向升级至公告修复版本 `v0.62.0`；按依赖约束同步升级 `x/crypto`、`x/sys`、`x/term`、`x/text`、`x/tools` 和 `x/sync`，不修改业务实现、Go 工具链版本或安全豁免。
- 将上游用户 ID、监控/模板请求体保存及监控账号选择三组回归加入前端固定 CI 清单，确保本次新增和调整的用例在每轮门禁中执行。
- 验证：Ollama HTML/用量解析、上游与渠道监控回归以及完整 CI/安全扫描随 PR #47 重验；保留首轮失败证据，修复提交通过后方可合入。生产部署不在本次变更范围。

### 2026-10-29 渠道状态厂商标识配色

- 关联 `CUST-OBS-001`，随本次修复提交，待发布。渠道状态分区标题的厂商图标和名称使用 `platformTextClass` 统一配色，覆盖现有全部厂商及深浅主题；Image 独立分区和未知厂商保持原有中性色。
- 仅调整标题颜色，分隔线、渠道卡片、状态和历史条的颜色及统计口径不变；不新增平台配色映射。
- 验证：渠道卡片与 Image 分组既有 6 项测试、前端类型检查、变更文件 ESLint 和生产构建通过。Playwright 在 1440×1000、390×844 下核对全部 10 个厂商图标与名称的深浅主题实际颜色，确认 Image/未知厂商中性色、分隔线及卡片布局保留，手机端无横向溢出；截图位于 `output/playwright/monitor-provider-color-*.png`。本次仅本地修改，未连接或部署生产。

### 2026-10-29 渠道监控请求体合并保存修复

- 关联 `CUST-OBS-001`，随本次修复提交，待发布。修复页面提示留空不覆盖、但保存接口拒绝空合并请求体的不一致；监控和请求模板均允许 `merge` 模式下的空值或空对象，沿用探针默认请求体及挑战校验。`replace` 模式继续要求非空对象并保留协议字段校验。
- 监控和模板提交前主动解析当前 JSON，防止未失焦时保存旧值、非法 JSON 仍提交或空覆盖到后端才报错；清空时发送空对象，避免更新接口将 `null` 解释为不修改而保留旧配置。表单按合并/覆盖模式分别提示，并说明灰色示例不参与提交。
- 验证：先用回归用例复现空合并保存失败及前端旧值提交问题；修复后 Go 1.27.3 渠道监控定向 unit 测试、前端 8 文件/56 项用例、类型检查、变更文件 ESLint、语言键完整性及生产构建通过。覆盖监控与模板的空合并创建/更新、显式清空、未失焦输入、非法 JSON、空覆盖拦截、有效覆盖及模式切换；确认 Anthropic、OpenAI Chat/Responses、Gemini 空合并请求与默认请求一致。Playwright 使用本地模拟接口验证空合并保存、重新打开及空覆盖中文提示，检查 1440×1000 和 390×844 截图，产物位于 `output/playwright/monitor-body-merge-desktop.png` 和 `output/playwright/monitor-body-replace-mobile.png`。本次无需数据库迁移，未连接或部署生产。

### 2026-10-29 New API 访问令牌支持上游用户 ID

- 关联 `CUST-OBS-002`，随本次修复提交，待发布。修复部分 New API 站点首次调用 `/api/user/self` 必须携带 `New-Api-User`、但现有表单无法提供 ID 的认证失败。
- 新增可选字符串字段 `newapi_user_id`，前后端校验正整数和 64 位有符号整数范围；新增、编辑回填、显式清除及更换令牌行为完整贯通。复用现有加密凭据字段，无数据库迁移，不改变已有无 ID 站点的自动发现行为。
- 缺少 ID 的明确 401 响应映射为 `UPSTREAM_NEWAPI_USER_ID_REQUIRED`，用户可在保留令牌的表单中补填；令牌无效、ID 已填写和其他 HTTP 状态不误判为缺少 ID。
- 验证：Go 1.27.3 执行 `go test -tags=unit ./internal/service -run 'Test(Upstream|NewAPIUpstream|MergeUpstream|SanitizeNewAPI|Sub2APIUpstream)' -count=1` 通过，覆盖创建/更新/同步、格式校验、凭据作用域隔离和错误分类；前端相关 42 项用例、类型检查、变更文件 ESLint 及生产构建通过。Playwright 在 1440×1000 和 390×844 下检查新增表单，并验证模拟保存后的编辑回填；截图位于 `output/playwright/upstream-user-id-desktop.png` 和 `output/playwright/upstream-user-id-mobile.png`。本次修改未连接或部署生产。

### 2026-10-26 渠道状态与降智检测紧凑布局

- 关联既有 `CUST-OBS-001`、`CUST-OBS-003`，按 2026-10-25 已确认的桌面及手机设计实施，不新增功能编号。渠道状态 V1 按平台分区并保持区内排序，未知平台追加、Image 独立；列表移除最后检测/成功时间、检测次数和重复倒计时，保留历史、周期、分组、配额及过期/无样本/真实零值语义。
- 降智画廊桌面四至六列、常见手机两列；列表仅展示检测结果。新增独立详情路由，完整回答按纯文本展示并可复制，作品可全屏。查询参数保存分组、模型、方式、结果和页码，返回恢复筛选及滚动；位置恢复使用立即滚动，避免全站平滑滚动与后台刷新竞争。
- 共享加载器按记录合并请求、最多四个并发，详情和可见作品优先，400px 预加载；完成记录缓存最多 24 条/约 32MiB，期限取五分钟与记录到期时间较早者。状态变化失效，运行中十五秒更新；离屏/后台卸载执行实例，账号、权限、开关和过期变化清理内容并阻止迟到响应回填。原双层 sandbox、CSP、消息来源校验及正文大小限制保留。
- 本轮修复长期到期计时器溢出、详情切换的旧请求状态干扰、同账号资料刷新误清缓存、返回滚动中间值及 scoped 深色选择器失效。既有余额页测试改为在环境退出前清理动画定时器，消除全量测试结束后的未处理异常；新增缓存、会话、平台及配额用例加入 CI 的前端关键清单。
- 验证：全量前端 408 个文件、3204 个用例通过；类型、静态检查及生产构建通过。实际 Chrome 检查 320/390/768/1600/2213px、深浅主题、筛选分页、详情往返、复制、全屏、动画与隔离。相同受控样本三轮测量，同一完成记录完整往返的正文请求从 4 次降至 1 次；首次加载未证明加速。完整条件、耗时、失败修复及命令结果见 `docs/channel-quality-ui-verification-2026-10-26.md`。
- 变更基线为 `0acaf83576ffcd6dd414d6f1d8019b670ea2367d`，本次记录对应本轮实现提交。复用原后端接口与数据库，未调用真实供应商或生产 Key，未连接、部署或修改生产环境。

### 2026-10-18 v0.1.259 身份透传安全修复发布

- 用户在修复 PR #42 经 CI 合入主线后追加授权更新标签并自动蓝绿部署。标签 `v0.1.259` 固定提交 `52c45a51b5f221728c3b7d3d39b83c9f86c81c22`，该提交的 10 项 CI 与安全检查全部通过；Release `39133089139` 已完成前端、五个平台二进制、镜像、正式版本发布及自动部署，工作流结论为成功。
- 北京时间 21:55:40 已从 green 切到 blue，公开设置返回 `0.1.259`，api/direct 两入口均 HTTP 200 且指向同一新实例。`CUST-UI-006` 已上线；完整上游同步基线不变，无数据库迁移。
- 旧 green 在既有一小时窗口后仍未排空，于北京时间 22:56:49 按自动超时策略完成强制退役；退出码 137、用量完整性未知，旧请求可能中断，不能记为无损退役。22:58:12 复核状态 `stable`、无待退役槽，新版双入口健康；工作流双入口各 3507 次健康探测均无错误，该结果不代表业务零损失。
- 最终核验两份 Compose 一致，Compose、持久化配置及排除版本标签的环境文件摘要与发布前相同，资源参数和数据 bind mount 保留，数据库未重启。完整证据及风险见 `docs/operations/2026-10-18-v0.1.259-release.md`。
- 修复不会撤销此前可能已经外传的凭据，本轮未核查历史泄露。

### 2026-10-18 自定义菜单登录凭据外传修复

- 新增 `CUST-UI-006`，当前能力族增至 66 项。本地基线 `00ef6eb85`、版本 `0.1.258`，选择性适配原始上游 PR #8125 固定提交 `14c4d361c72c318b072da454e8936b10989c4582`；截至本次核查该 PR 仍未合并，完整上游基线保持 `b0a02a3304a0a78cdd003ddda1dd23ba02c03084`。
- 修复旧行为在访问外部自定义菜单时自动附带完整登录 Token 和用户 ID 的问题；改为独立开关显式开启、旧配置默认关闭。登录 Token 可进一步用于读取所属账户 API Key，管理员凭据影响更广；本轮确认代码风险，未核查生产是否曾发生泄露。
- `src_url` 无论是否开启身份透传都清理查询参数与片段；HTTP/HTTPS 目标保留原有参数、主题与语言。配置经管理请求、存储、管理响应和用户公开响应往返保留，不需要数据库迁移。
- 补充 URL 构造、普通用户/管理员 iframe 与新窗口、配置加载保存及后端 DTO 往返回归，并更新支付集成说明。验证：5 个相关前端测试文件共 85 项通过；隔离管理页测试夹具后复跑 2 项身份透传测试通过；前端类型检查、修改文件 ESLint、生产构建和 `git diff --check` 通过；WSL Go 1.27.2 执行 `go test ./internal/handler/dto -count=1` 通过。未执行全量后端测试或生产环境验证。
- URL 构造、自定义页面及其竞态回归纳入前端关键测试列表，由 PR CI 持续执行。用户已授权提交、推送，并在仓库 CI 与安全扫描通过后合入 `main`；远端验收以对应 PR 与检查记录为准。本次不打标签、发布、部署或访问生产。显式开启 Token 透传仍交出完整登录身份；修复不会收回此前可能已外传的凭据。

### 2026-10-17 降智检测第三方接口兼容修复

- 关联既有 `CUST-OBS-003`。服务器1排查时运行 v0.1.257，检测记录显示从北京时间 00:21 起持续在第三方任务创建阶段收到拒绝；相同配置最后一次成功检测约为 00:09，故障早于当日 19:41 的当前版本启动。
- 复现确认满血AI已移除 `protocol` 参数，旧请求返回 HTTP 400 与 `Invalid API request parameters`。移除该字段后，使用既有检测绑定及 `gpt-6.1-sol/high` 的单次糖果检测完成并通过，第三方报告耗时 33389 毫秒。
- 第三方同时将鹈鹕模型限定为 `gpt-6-astra`；`gpt-6.1-sol` 的鹈鹕请求即使移除旧参数仍返回 HTTP 400。保留原模型与配置，明确展示该限制，不通过替换模型伪造检测成功。
- 客户端删除废弃字段，拒绝原因仅从最多 4 KiB 响应中匹配已知错误并映射固定提示；未知错误仅保留状态码。增加请求参数、拒绝原因落库、凭据隔离、无效与超长错误响应回归，原有限流退避和不明确提交保护继续保留。
- 验证：Go 1.27.2 下运行 `go test ./internal/pkg/manxue ./internal/service -run '^(TestClient|TestModelQuality)' -count=1`，两个包均通过；`git diff --check` 通过。未执行全量测试或生产发布。
- 排查阶段完成本地补丁，后续按用户要求提交 PR 并在 CI 通过后合入主线；不修改生产配置、历史检测状态或发布版本，不重启生产应用。

### 2026-10-15 降智检测与分组画廊

- 新增 `CUST-OBS-003`，同步 `CUST-UI-001`、`CUST-UI-002`，能力清单增至 65 项。用户已明确通过桌面画廊、作品详情、移动布局和管理员配置设计。
- 新增独立配置与 API、SQL 迁移 246、第三方客户端、持久化状态机和后台调度。数据库行锁保护两个全局槽位，活动轮次唯一索引防止同组重复执行；限流遵循 Retry-After，提交结果不明确时保留异常并等待外部执行期结束，避免重复扣费。
- 画廊按分组切换、筛选和倒序分页，糖果与鹈鹕结果独立展示；详情按需读取，动画进入视口加载、离屏卸载。作品使用双层隔离壳及独立 CSP，用户端不返回凭据、内部端点或第三方任务 ID。历史按开始时间保留 24 小时，关闭功能后仍清理。
- CI 集成独立 PostgreSQL 服务执行降智检测数据库用例，前端关键测试清单纳入画廊、配置、隔离预览、功能开关和管理路由回归。
- 验证包括模拟第三方的配置、权限、恢复、限流、超时和开关回归；独立本地 PostgreSQL 的迁移幂等、并发争抢、结果持久化、间隔和清理集成测试；前端组件、类型与构建检查，以及桌面/手机与恶意 HTML 浏览器验收。真实检测由管理员配置后“立即检测”验证，本轮未调用生产 Key、未部署生产。

### 2026-10-11 生图成功率排除业务拒绝

- 关联既有 `CUST-BILL-004`。按用户确认的生图业务口径，403余额/权限拒绝、明确的余额不足、内容审核拒绝及无效参数/输入不进入成功率分母，也不增加失败数或成功数；既有分组和接口范围保持。
- 单次请求复用有界错误响应捕获与上游最终错误上下文，识别包装成502的原始403及HTTP 200内的流式失败。只看最终请求结果，成功重试不被早先错误污染，请求级流内结果不继承此前上游错误。
- 批量提交使用实际错误原因，覆盖预扣余额不足；混合批次分页检查失败条目，只排除明确的业务拒绝，读取失败交由原有Worker重试。任务结果、退款和结算仍保留原始失败数，仅成功率聚合采用过滤后的计数。
- 部分上游将生图超时也返回为400/434，因此不按全部4xx或通用 `invalid_request_error` 一刀切排除。超时、过载、上游401/429、流中断及未知故障继续计失败。
- 回归覆盖余额/审核/输入错误、包装403、流式终态、重试恢复及最终故障归因、批次混合结果、分页和数据库读取失败。不增加数据库迁移，不自动清零或回写历史计数。

### 2026-10-11 账号筛选常显与提示浮层裁切修复

- 关联既有 `CUST-UI-004`；修复主题容器与表格、弹窗中的提示显示，不新增能力编号，64 个既有编号保持。
- 历史来源：上游提交 `679a7186f` 于北京时间 2026-10-04 08:43:47 加入“更多筛选”；本地同步提交 `85665a068` 为 2026-10-09 23:36:50，PR #32 的主线合并提交 `aa4ec3ea1` 为 2026-10-10 01:36:24。此次按用户要求覆盖折叠交互，保留上游筛选项、选项和事件语义，并移除已无引用的折叠按钮文案。
- 缓存提示原本嵌入带 `clip-path` 的统计卡片，提升 z-index 无法越过裁切。将该处及其他 32 处容器内悬停提示迁入已有 `HelpTooltip`，覆盖账号错误、限流、用量说明、创建/编辑表单、分组设置、用户分组/余额/平台用量、页头余额、接口地址及运维诊断。原生 title 与已使用 Teleport 的独立浮层保持原实现。
- 公共提示使用 fixed 视口坐标，不再叠加页面滚动偏移；左右限宽与上下自动换向，长内容按可用高度滚动。保留移入选中文字、点击模式与关闭操作，补充键盘聚焦、Escape 和禁用状态，嵌套提示按深度提升层级，避免内层帮助被外层诊断覆盖；隐藏时不参与布局。统计卡片在窄屏使用单列，防止大数值被边框裁切。
- 验证：`pnpm exec vitest run --maxWorkers=4 --minWorkers=1 --reporter=dot` 全量通过 391 个测试文件、3046 项测试；`pnpm run lint:check` 无代码错误或警告，`pnpm run build`（含 i18n 完整性检查、vue-tsc 类型检查）与 `git diff --check` 通过。新增/调整回归覆盖筛选常显与重置、缓存浮层挂载、键盘、四角定位、滚动/缩放、长内容与嵌套层级，并保留停调度原因及端点复制断言。
- 浏览器验收：隔离页面复用真实组件和项目样式，使用示例数据；Playwright 在 1600×1000、390×844、320×760 三种视口及深浅主题下完成 60 组提示四角/中心命中、边界和横向溢出检查，全部通过；额外验证分组直接选择、移入提示选中文字的持续显示、键盘开关与嵌套提示层级。生成结果位于 `output/playwright/account-filters-tooltips-20261011/`，检查日志位于 `output/playwright/ui-filters-tooltips-*-final.log`。构建仍有既有的大包与 Browserslist 数据更新提示，不影响通过；初始修复提交 `483f62783` 经 PR #34 的 8 项 CI 与 2 项安全扫描通过。用户随后授权合入主线、更新标签并自动蓝绿部署，版本文件更新为 `0.1.253`，发布范围与生产只读基线见 `docs/operations/2026-10-11-v0.1.253-release.md`；实际发布结果以对应 Actions 运行及部署证据为准。

### 2026-10-10 242专项兼容与v0.1.252发布准备

- 用户已明确批准先完成242专项兼容适配，通过CI后创建新标签并由Actions自动蓝绿部署。关联既有 `CUST-OPS-007`、`CUST-OPS-001`、`CUST-OPS-004`；64项能力编号保持，不扩大上游同步范围。
- 固定生产旧提交 `a335cbcc59c343278174e5382256b0fd641015f5`，原242 SQL及两个Ent文件均不修改，以精确前后摘要审查。迁移在同一短事务内先安装与旧约束等价的十一平台保护，再执行原SQL并记账；共存期间两表禁止command_code、cline及其他新平台写入。旧版平台读写、用量字段和历史数据不变。
- 只允许PostgreSQL18、16MiB以内无自定义触发器/RLS/继承的普通表，严格核对平台列、238/241账本和原CHECK定义；NOWAIT同时获取双表锁，沿用1/5/10秒锁/语句/事务预算。切流和回滚前复核保护，旧槽完全退役后经既有固定身份/挂载/配置/健康核验才解除；失败或回滚保留，解除重试幂等，不反向删除迁移账本。
- 新增固定审批、非法schema/基线拒绝、真实数据库锁冲突与漂移、旧版读写、阻止提前解除及退役重试回归；完整CI及安全扫描仍作为发布门禁。版本文件更新0.1.252，原标签不移动，不修改生产资源、账务、超时或业务开关。
- 本条为实施记录，不预写远端检查或部署成功；实际结果集中记录在 `docs/operations/2026-10-10-v0.1.252-release.md`。

### 2026-10-10 CI 用量优化

- 修改既有 `CUST-OPS-004`，不新增或复用能力编号。CI 和 Security Scan 的 Push 仅监听 main，消除功能分支 Push 与 PR 的成对运行；PR 不增加路径过滤，两个工作流均保留手动运行，安全扫描仍每周执行。
- 按工作流名称与 PR 编号或 ref 分组，新检查取消同组的过期运行；不同 PR 和 CI/安全扫描彼此隔离。Release、生产预检和旧实例退役工作流不改动，不把自动取消应用到发布或部署。
- 8 项 CI 和 2 项安全检查全部保留，包括 macOS 部署脚本、单元/集成、两组竞态、前端/Canvas、lint、蓝绿协议和依赖安全门禁。不减少测试、不扩大漏洞豁免，不以工作流墙钟时长冒充计费分钟或承诺固定节省比例。
- 新增 6 项工作流策略回归，先在旧配置上检出预期问题，再验证修改后全部通过；同时通过 14 项发布辅助回归、5 项上游元数据回归及当前来源校验。测试复用现有 release-helpers 作业及 PyYAML 依赖，不新增 runner。
- 用户授权提交并在远端 CI 与安全扫描通过后以保留历史的 merge commit 合入 PR #32；以最终提交的实际 Actions 结果为合并依据。本条不预写远端成功，不创建标签、不访问服务器、不部署，也不改变生产参数或迁移放行边界。

### 2026-10-09 v0.1.251发布准备

- 用户追加授权更新标签并由既有 GitHub Actions 自动蓝绿部署。发布范围为 PR #30 已合入主线的 `CUST-GW-016` 动态工具契约校验与渠道兼容；Token 基准项不修改，不增加上游同步或数据库迁移。
- 版本文件由0.1.250更新为0.1.251，保持功能默认关闭及请求级配置快照。版本候选和合并后主线必须通过完整 CI 与安全扫描，随后才创建新附注标签 `v0.1.251`，不移动已有标签。
- 生产目标仅为服务器1，保留既有bind mount、双Compose、资源参数、3600秒响应头超时和5秒usage任务超时。沿用一小时排空观察及既有超时强退策略，强退仍可能中断旧请求和用量收尾。
- 本条仅记录发布准备，不预写部署成功；实际CI、发布和验收结果见 `docs/operations/2026-10-09-v0.1.251-release.md`。

### 2026-10-09 Kimi 动态工具契约与渠道兼容

- 新增 `CUST-GW-016`，按用户授权实现“二开功能 → 网关配置”独立开关 `kimi_dynamic_tools_enabled`，默认关闭。旧配置、旧管理客户端部分更新、请求级配置快照和其他平台分组行为保留。
- 依据 KVV 固定提交 `66092cf444c97356c0e11c5078c67116390615d9`，校验动态工具的原始角色、正文、结构、命名和重复声明。合法工具合并到顶层，非法请求在并发排队及 SSE 响应提交前返回400；普通工具执行结果保留正文和调用编号。
- 保留输出额度、tool_choice、思考历史、流式工具事件、用量记录及现有五项参数重试开关。`assistant_hello` Token 基准不修改，不通过合成工具响应或补 Token 数获取测试通过。
- 入口只校验，保留原始请求供审计、计费预估和会话识别使用；转发层在协议转换前合并工具。无需数据库迁移。
- 验证通过：Go 1.27 的 `-tags=unit` 服务层、请求入口和管理接口三个包整包测试；`TestKimiDynamicTools|TestCustomFeatureHandler_Kimi` 三个包的 `-race` 回归；前端定向29项测试、类型检查、改动文件 ESLint 和生产构建。新增 Kimi 用例自动纳入现有 `TestKimi` 流竞态 CI 选择范围，远端 CI 与主线合并结果以对应 PR 的检查及合并记录为准。
- 浏览器验证通过：桌面1440×1000和手机390×844布局检查，开关启用、关闭、保存及刷新回显，控制台0错误、0警告。使用本地模拟管理接口，不连接生产。截图及日志保存在 `output/playwright/kimi-dynamic-tools-20261009/`。
- 环境说明：WSL 挂载盘不提供测试所需的 Unix socket 和文件权限语义，初轮四个既有用例失败；改用工作区内 Linux tmpfs 后服务层整包通过，未修改既有断言。本地模拟上游不代表真实渠道 KVV 或完整认证；未发布、未修改生产配置。
- PR #30 首轮 CI 发现新增错误文案的英文大写首字符不符合 Staticcheck ST1005，改为中文开头后重新验证；不改变错误状态码、契约或兼容行为，不放宽静态检查。
- 合并到顶层会改变历史工具位置及前缀缓存，使用边界与回退方式见 `docs/kimi-dynamic-tools.md`。

### 2026-09-24 v0.1.246发布验收

- 关联`CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-007`，不新增或退役二开编号，63项能力族全部保留。实现提交`94dac3dcb880dcf96e3677827d3b4af7a55b9c98`通过候选、主线、标签各8项CI及2项安全扫描；新附注tag`v0.1.246`与Release `35947647075`的10项作业均成功，没有重打标签或绕过门禁。
- 10:41:53切流green/18082，11:43:01旧blue回收后恢复stable、pending=null，11:43:02自动解除倍率保护；11:44:11只读核验发布版本/摘要、唯一活动槽、双入口和迁移结构均符合预期。309条历史迁移及摘要未变，仅新增239/240，三项临时保护约束已消失；普通字段门禁及原SQL不变。
- 原Compose/config哈希、批准资源参数、HTTP upstream、bind mount、PG/Redis身份及启动时间均不变，保留3600秒响应头等待、5秒usage任务与按用户串行扣费；green切流后未重启。VERSION仅由Actions回写至0.1.246，生产验收记录不另触发部署。
- 旧blue一小时到期时仍有HTTP 5、SSE 3、会话租约832，按既有策略强制退出137、非OOM，clean_exit=false、usage_loss_unknown=true；817条旧归属已清理。7012次健康探针无异常不能证明旧请求无中断或用量完整，不改账单和退役策略。
- 真实付费Provider、支付/回调/提现、完整账单、备份恢复演练、全二开浏览器端到端及峰值压测仍未验证；既有内存预算OOM风险保留。完整时序、CI、镜像与迁移证据见`docs/operations/2026-09-24-v0.1.246-release.md`；本记录通过独立文档分支门禁后再合入主线。

### 2026-09-24 239/240专项蓝绿兼容适配

- 关联`CUST-OPS-001`、`CUST-OPS-007`。用户在主线`304006b24`候选/主线各8项CI与2项安全扫描通过后，明确批准迁移兼容适配、隔离验证及后续tag自动蓝绿发布。
- 只批准固定旧SHA及两个原始SQL摘要；普通可空字段规则、历史迁移校验和、生产Compose、资源参数、3600秒响应头超时、5秒usage任务及按用户串行扣费不变。新增专项表大小/类型/版本/锁超时检查，候选失败不切流。
- 新旧并行期间以数据库约束保护空推理倍率；旧版可继续保存空倍率及其他价格，新增倍率需待退役解除后保存。回滚保留约束；解除失败报告失败而非静默放行。此行为只用于本次受控发布，不改变常规启动和计费默认值。
- 新增旧新版读写、全推理级别计费、前置拒绝、事务回滚、索引、锁冲突、定义漂移、重复启动及解除恢复测试。生产备份恢复及峰值负载不在本地测试证明范围内；本条为实现记录，不代表标签发布已完成。
- 同日10:00生产只读检查识别184/193已有groups触发器；按真实定义和函数摘要接纳其模型定价更新直接返回路径，新增真实193函数及定义漂移拒绝测试。其他用户触发器不放行，不修改线上触发器。

### 2026-09-24 上游0.2.8本地增量同步

- 完整合并固定范围 `7c700729c..a3eb7ef30`。38个文本冲突按批准方案逐块组合，未整文件采用任一侧。保留本地0.1.245版本线、63项编号、禁止网页更新及所有生产保护边界。
- 通用思考倍率与本地请求模型/严格定价组合，保持Fable未配置为1倍；新GPT-6 Sol/Luna接入本地严格规范化，不放宽未知模型识别。流错误保留response.failed终态，同时接入分类、脱敏及单次输出标记。
- 月度归档接入本地实例归属及优雅排空。数据库写锁保留旧键并拒绝错误后改用Redis；恢复开始与结果写入保留owner，只在明确死亡时回收，归档保护和持久写入失败不删对象规则保留。
- 动态Claude版本保持全局API Key强制模拟；messages、count_tokens及账号测试的UA与billing版本来自同一快照。日期弹层继续Teleport，CSV继续游标导出、节流、取消与公式保护。
- 补充旧锁键/共享数据库/异常不降级、存活未知owner超时保护、恢复归属、动态版本翻转及新模型白名单回归；适配上游日期测试的真实Teleport定位与通用倍率表单，不删除断言。前后实际命令、首次失败与修复复跑、未验证项集中记录在本轮同步历史。
- 新迁移239/240只进行隔离验证，均不符合现行蓝绿迁移门禁；未扩展兼容白名单。没有push、PR、标签、SSH或生产操作，也不将本地测试等同于真实Provider/生产验收。

### 2026-09-23 恢复版本来源并新增只读上游提示

- 涉及 `CUST-OPS-006`、新增 `CUST-OPS-008`。修复删除旧更新服务后遗漏 `SettingService.SetVersion`，导致首屏注入版本为空且前端永久缓存空值的问题；不在前端写死当前版本，不主动提升发布版本。
- 嵌入已核验的完整同步基线：`Wei-Shaw/sub2api`、`0.2.7`、`7c700729c23187d31ed320f6b19c790e2f194826`、`2026-09-20T23:09:09+08:00`。这是已有同步事实的结构化记录，不代表本次合入新上游代码。
- 管理员的两个版本接口均须通过管理员鉴权，公共配置不暴露同步来源；同步来源和当前版本不等待 GitHub。最新版本仅与已同步原版基线做语义化比较，排除草稿、预发布及无效标签；发布链接限定原仓库，不暴露 GitHub 凭据或错误详情。
- 界面保留品牌样式，管理员固定三行信息，折叠时可打开详情；检查失败不弹全局错误，不影响导航，不恢复任何更新执行能力。
- 轮询补充浏览器与服务器时钟偏差保护：服务端返回成功缓存但本地计算已到期时，恢复正常30分钟间隔，避免每秒重复请求；包含对应前端回归。
- 回归覆盖版本注入及接口一致性、真实普通用户权限、已删除路由404、缓存/超时/退避/并发/取消与旧结果保留、前端身份差异和计时器清理。关键前端用例、上游检查竞态、实际构建 HTML 注入检查及同步元数据校验纳入 CI；元数据校验要求提交是当前历史祖先且其上游 `VERSION` 与记录相同。
- 本地验证：Linux 后端全量单元/集成测试、上游检查竞态测试、后端 lint、前端 lint/类型检查、41个关键测试文件共479项、Canvas 34项测试及两套前端生产构建通过；真实构建 HTML 注入回归和嵌入式后端构建通过。隔离浏览器完成11个截图场景，覆盖1440×1000桌面、390×844移动端、折叠、深浅主题、长版本、普通用户、网络失败、GitHub不可用与旧缓存；检查前后导航纵坐标均为96px。开发服务启动被本地环境策略拒绝，界面验证直接读取构建产物并使用浏览器内模拟接口，未访问生产。
- 交付边界：经 `codex/` 分支及 PR 验证后合入主线，等待主线 CI 和 Security Scan；无需数据库迁移，不打标签、不部署、不修改生产配置。远端验收结果随交付报告提供，本条不宣称已发布。

### 2026-09-21 v0.1.243 发布验收

- 关联`CUST-OPS-001`、`CUST-OPS-003`、`CUST-OPS-007`。实现提交`6154c64ffc83f180f7e8f118ccbd5182b8cb0f52`通过候选CI及安全扫描后快进主线，再推送新附注标签`v0.1.243`；候选、主线、标签各8项CI和2项安全扫描全部成功。Release `35604350151`的10项作业及实际蓝绿部署步骤全部成功，没有覆盖旧标签。
- 新增`engine_meta`实测为可空JSONB、无默认值，历史迁移账本摘要全部不变且只新增238b；实际活动迁移策略与Actions兼容证据一致。21:23:32切流blue/18080，双入口确认约1.156秒，22:24:41恢复stable、pending=null。
- 旧green按既有一小时策略强退，关停凭据记录HTTP 10、SSE 4、会话租约495，退出137且非OOM、clean_exit=false、usage_loss_unknown=true。7016次健康探针无异常不代表旧请求无中断或账单完整，字段兼容审批不扩大为无损退役承诺。
- 最终只读验收确认新blue未重启，PG/Redis身份、启动时间、重启计数、资源参数、原bind mount、双Compose和持久配置均不变；保留3600秒响应头超时及5秒usage任务超时。完整镜像、归档摘要、时序和未验证边界见`docs/operations/2026-09-21-v0.1.243-release.md`。

### 2026-09-21 蓝绿可空字段扩展审批

- 用户在风险说明后批准旧版兼容的字段迁移，新增 `CUST-OPS-007`，与 `CUST-OPS-001` 发布流程集成；不放开删除、改类型、强约束、数据回填或历史迁移改写。
- 对上游新增 `engine_meta JSONB` 保留原始 SQL 和校验和，以旧版明确列名读写为审查依据，新增固定基线/摘要证据和候选启动保护。版本预备为 `0.1.243`。
- 验证与生产交付结果在发布前后分别补记；尚未以本段声明生产迁移、标签推送或部署完成。
- 本地验证：证据测试6项、部署状态机51项、预检3项通过；迁移定向单元与 PostgreSQL 集成测试通过，golangci-lint 0 issues。NTFS Unix socket 用例首次失败，转至仓库内 POSIX 临时目录后原断言通过。初始化与常规启动统一使用受保护迁移入口。
- 2026-09-21 20:58（UTC+8）生产只读预检：活动旧提交与审批一致，实际仅待执行238b；目标为普通表且无继承，采样无该表锁/长事务。备份记录最近完成于同日12:12:53，本轮未下载或演练恢复，不将记录状态等同于恢复能力证明。

记录按时间倒序追加。功能清单描述“现在是什么”，本节描述“为什么变成这样”。

| 日期 | 版本/提交 | 类型 | 功能编号 | 变更与原因 | 验证 |
| --- | --- | --- | --- | --- | --- |
| 2026-09-24 | 本次修复提交 / 不提升版本 | 修复 | `CUST-OBS-001`、`CUST-BILL-005` | 修复渠道监控在生命周期拒绝执行后永久保留本地in-flight标记的问题，避免无样本被误显示为0%或陈旧状态继续显示正常；修复用户导出第61页触发限流、管理员深分页重复COUNT/OFFSET造成长时间导出失败。新导出接口保留原鉴权、权限过滤及DTO投影，前端按稳定游标限速读取、重试和取消。只提交代码并验证CI，不打标签、不部署或重启生产，不修改历史监控与用量数据。 | Go全量unit与真实PostgreSQL/Redis integration、渠道调度定向race、golangci-lint（0 issues）、CGO关闭构建及实际前端嵌入版本回归通过；前端329文件/2511用例、ESLint、类型检查、生产构建，画布8文件/34用例、类型检查和构建通过。回归覆盖生命周期拒绝后恢复、无样本/真实0%/过期预算、游标筛选/用户隔离/并发插入、61页节流/429重试/取消。Chrome本地构建与模拟API完成1440px/390px、深浅主题、渠道详情、CSV/XLSX下载及行列校验、进度与取消验收；未做生产压测。远端CI与Security Scan以最终提交的工作流结果为准。 |
| 2026-09-22 | `0.1.244` / 发布候选 | 新增与修复 | `CUST-GW-015` | 为 Kimi 参数兼容重试增加非法思考类型独立开关，明确400且出站值非法时删除整个 thinking 重试一次；默认关闭，新增配置读写/缓存、管理页中英文文案与旧客户端部分更新保护，累计规则上限由四次扩为五次。按用户追加授权，CI 与安全扫描通过后推送新标签并自动蓝绿部署；不修改生产开关及退役策略。 | 修复前真实上游三组 HTTP/2 流式对照已复现400，剥离/auto均完整成功。后端 service、admin handler 全量 unit 与 Kimi/审计定向竞态测试通过；前端326个文件、2481项测试全部通过，类型检查、改动文件 ESLint、gofmt 与 diff 检查通过。首次前端全量仅既有 browserStorage 测试失败，禁用 Node 25 原生 Web Storage 后定向及全量重跑通过，未修改无关代码。首轮远端 CI 检出新增分支的 staticcheck QF1003，改为等价 switch 后定向单测和与 CI 同版 golangci-lint 2.13.0 全量检查通过（0 issues）。新增配置、匹配边界、流式/非流式恢复、限次、换号保留与界面回归均通过；本行记录发布前验证，不代表线上部署已完成。 |
| 2026-09-20 | `0.1.242` / 本轮合并 | 上游适配 | `CUST-GW-001`、`CUST-GW-003`、`CUST-PROTO-001`、`CUST-PROTO-006`、`CUST-ACC-001`、`CUST-BILL-001`、`CUST-BILL-004`、`CUST-RISK-001`、`CUST-RISK-002`、`CUST-OPS-001`、`CUST-OPS-004` | 完整合并上游46个提交；逐块组合12个冲突文件，保留本地版本、审核/计费/重试策略与发布约束。纯上游新增能力不重复分配本地编号。 | 同步前后验证与首次失败/复跑证据见本轮同步历史；新增审核切换、元数据及发布门禁交叉回归。不推送、不部署，不以本地模拟验证代替真实Provider、支付和生产验收。 |
| 2026-09-20 | `0.1.241` / 本地未提交 | 新增与修复 | `CUST-OPS-006`、`CUST-UI-001`、`CUST-PROD-008` | 删除不适用于定制版的网页自更新及危险操作接口；修复用户名为空时邮箱关键词无法搜索的问题，后台补充邮箱身份显示，公开页不增加邮箱。无数据库迁移，不推送、不打标签、不自动部署。 | Go 全量 unit、真实 PostgreSQL/Redis repository 集成包、golangci-lint 0 issues、嵌入构建；前端全量 324 文件 / 2441 用例、类型检查、非改写 ESLint、构建及 1440px/390px 本地浏览器夹具检查通过。 |
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
6. 前端设置、导航和公共配置变化：`CUST-PROD-001`、`CUST-PROD-002`、`CUST-PROD-007`、`CUST-OBS-002`、`CUST-UI-001` 至 `CUST-UI-006`；
7. release、Compose、镜像或网页版本管理变化：`CUST-OPS-001` 至 `CUST-OPS-006`；尤其不得恢复网页自更新、回滚或重启入口。

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
- 再次追加发布授权：主线 `eed208250` 的7项CI和2项安全扫描均通过后，用户要求更新tag并通过GitHub Actions自动蓝绿部署。已创建固定指向该提交的附注标签 `v0.1.241`，不改生产开关、资源、挂载或既有一小时退役策略；发布结果与实际验收见 `docs/operations/2026-09-19-v0.1.241-release.md`。
- 发布验收：tag的7项CI、2项安全扫描及Release `35431430291` 的5项任务均成功；16:26:17切流blue/18080，17:27:27旧green回收后恢复stable、pending=null。新实例、数据库、Redis健康，资源、挂载、双Compose及持久化配置哈希保持不变。
- 旧实例为到期强制退役，关停前HTTP19、SSE12、会话租约929，退出137且非OOM；clean_exit=false、usage_loss_unknown=true。7016次健康探针无异常不等于旧请求无中断或账单完整，未改退役策略或补写账单。
- 发布过程未自动开启开关；16:37:56管理端审计129155确认admin/JWT保存将安全重试和提前保活同时设为true，最终保留该管理操作，不自行回退。代码默认关闭及原有首字统计仍保持不变。

### 2026-09-20 删除网页自更新并修复用户定制搜索（北京时间）

- 实现阶段基线 `71213b65c123c6c553e7f3cd8261be1270ea8757`，版本保持 `0.1.241`。新增 `CUST-OPS-006`，修改 `CUST-UI-001`、`CUST-PROD-008`。实现阶段仅本地修改与验证，不提交、推送、打标签或部署，未连接生产服务器，无数据库迁移。
- 网页版本区不再调用上游版本检查、更新、回滚或重启接口；服务端移除对应五条路由及专用更新依赖，仅返回当前构建版本。共享 Codex 客户端版本同步、首次安装重启、定制版标签发布和蓝绿部署均保留。
- 搜索按用户名和邮箱不区分大小写包含匹配，ID 精确匹配，保留空白清理、提交查询、重置页码及软删除过滤。邮箱仅增加在管理员响应和身份显示中，公开余额 DTO、页面、账户数据和授信规则不变。
- 前端全量 324 个文件 / 2441 个用例、类型检查、非改写 ESLint、生产构建通过；构建仍有已有 Browserslist 数据陈旧、动态/静态混合导入和分包体积提示，不更新依赖。新增版本、后台 API、搜索和确认窗口测试加入关键前端回归清单。浏览器复验发现无空格长邮箱会溢出确认消息，已补充自动换行，确认与取消事件保持不变。
- 后端 Go 1.27 全量 unit（58 个有测试的包）、真实 PostgreSQL 18.1 / Redis 8.4 的 repository 集成包、golangci-lint 2.13（0 issues）及带前端嵌入的服务端构建通过。搜索覆盖大小写、首尾空格、中文、邮箱片段、精确 ID、空查询、无结果、软删除、备注排除和分页总数；授信并发、恢复代次、链接失效与公开字段隔离回归保留。新增路由测试验证五个旧接口 404、只读版本响应正常且仍需管理员鉴权；共享 Codex 版本同步及首次安装相关既有测试通过。
- 初次全量回归发现测试库其他用例也含 `box`，搜索用例改为核验相对既有结果的增量并明确排除软删除及备注匹配。WSL 的 Windows 挂载目录不支持 Unix socket 和 POSIX 文件权限，导致既有生命周期及首输出暂存测试失败；仅将测试临时目录切到工作目录内的 Linux tmpfs 后全量通过，随后已卸载，不修改这些业务逻辑。WSL 构建的 Git 所有权检查通过本次命令的 `-buildvcs=false` 避开，不改仓库信任配置或发布脚本。
- Playwright 以本地构建产物和隔离接口夹具检查 1440×1000、390×844：版本区只读且无上游检查请求；`box`、`BOX`、首尾空格及 ID 搜索、中文身份、分页后重新搜索回到第一页均通过；编辑、恢复和重置窗口显示正确身份及金额，长邮箱不溢出；公开页仅保留 ID、用户名和余额，不含邮箱，请求不携带登录凭据。截图位于 `output/playwright/update-search-*.png`，辅助夹具与日志位于 `tmp/web-update-search-20260920/`。
- 实现阶段验证边界：本地 Vite 后台启动被工具策略拒绝，未换壳重试，浏览器通过测试会话加载已构建的静态产物完成检查；当时未执行真实生产流量、实际首次安装重启或发布后验收。生产应用、数据库、Redis、配置、数据挂载和版本均未被该实现阶段修改。
- 后续发布授权：用户追加要求提交并推送代码、合入主线，待该固定提交的全部 CI 和安全扫描成功后，再创建新标签并通过 GitHub Actions 自动蓝绿部署。预计使用尚未占用的 `v0.1.242`，不覆盖旧标签，不修改生产资源、配置、开关或既有一小时观察及到期强制退役策略；本条仅记录授权与待执行流程，不代表发布已经完成，实际结果另行归档。
- 发布结果：功能提交 `ee1c56f75a38c8678ed338a4a1fecad51b08593c` 已推送主线，在7项CI和2项安全扫描通过后创建附注标签 `v0.1.242`。标签的7项CI、2项安全扫描及 Release `35462200005` 的5项任务全部成功；Actions 自动回写版本为 `0.1.242`，未移动旧标签。完整记录见 `docs/operations/2026-09-20-v0.1.242-release.md`。
- 生产验收：03:00:22 切流到 green/18082，双入口确认约1.176秒；04:01:31 旧 blue 回收后恢复 stable、pending=null。实际管理员只读请求中 `box`、`BOX`、` box `、`1054` 均匹配目标用户，五个已移除接口均404，公共设置及只读版本接口返回0.1.242；邮箱仅出现在管理员响应中，未回传生产凭据、明文邮箱或余额。
- 数据保护：切流后、观察中及退役后比对均确认数据库和Redis的容器身份、启动时间、挂载、资源以及双Compose、持久化配置、环境文件哈希未变，安全重试与提前保活继续保留管理员设置的true/true。新green的实例身份和启动时间在观察期间未变。
- 退役边界：旧blue按一小时策略强制退役，关停凭据记录HTTP17、SSE10、会话租约720，退出137且非OOM，clean_exit=false、usage_loss_unknown=true。7016次健康探针无异常不代表旧请求无中断或用量完整；本次未改退役策略、补写账单或重启数据库/Redis。

## 2026-09-30 上游同步 a60a29549（执行阶段，北京时间）

- 在 `sync/upstream-20260930-a60a29549` 完整合并上游0.2.10；原main基线为 `8bef535b1df379fdddac734040b7017d5214e666`。本地版本保持0.1.246，仅更新嵌入上游来源，不发布或部署。
- 当前清单的63项编号保持，更新17项受影响条目。保留首Token/安全重试、Kimi、逐轮请求模型收费、严格缺价、用户串行扣费、5秒usage任务及蓝绿资源/挂载约束。
- WS合入综合路由、账号所有权、风控白名单，保留原子状态、逐轮用量归因和大请求体释放；新增两轮恰好两次记账回归，修复Cyber兜底与完整失败结果的重复用量。
- 图片输入/输出价留空继承目录，显式0免费；未配置独立图片价才回退文本价。账号成本长上下文只受账号门禁控制，客户售价保持分组与账号双门禁；三种渠道历史计费来源仍统一按用户请求模型收费。
- Anthropic模拟身份与账号覆写边界不变，Sonnet 5.5过滤最终beta；显式structured-output可保留，策略过滤优先。实际usage归一化不移除首Token取消和清理。
- 风控白名单默认空，启用后仅留证据，不新增处罚、通知、会话阻断；保留EmailOnHit、作用域和独立审计。Antigravity兼容入口不采用默认15秒提前保活/固定两分钟截止，空流和signature修复正常纳入。
- Redis参数列表在两份生产Compose保持字节一致，保留maxclients和bind mount；未连接生产、未读实例秘密。逐提交103项处置、冲突及最终验证见 `docs/upstream-sync-history.md` 本次记录。此阶段记录不代替最终验证结果。

### 2026-09-30 同步验收中的测试契约适配

- 关联编号：`CUST-BILL-002`、`CUST-BILL-005`、`CUST-RISK-002`；不新增或改变产品行为。首轮全量验证发现上游两个测试夹具与本地已批准保留的规则不一致。
- Cyber 白名单留证测试显式开启内容审核、观察模式、通知和自动封禁，继续断言留证但不计数、不通知、不封禁；补充风控关闭、审核关闭、模式关闭、分组和模型不在作用域时均不留证、不计数的回归。
- Free Fast 缺价用例先验证标准模式返回 `ErrModelPricingUnavailable` 且不写日志，再验证简易模式保留真实 Token、priority 层级和零费用行；没有把标准模式未知模型改为免费。清单中原“缺价保留零费用”表述据此限定适用路径。
- 本地适配提交追加到候选 merge 之后，不改写已有提交。最终验证与实际提交映射追加到同步历史，保留首轮失败证据。

### 2026-09-30 上游同步最终本地验收

- 代码合并 `64e77f8c1432a6045a22cc45e156fb639ce59de2`，测试契约适配 `74aed12274dfea757d8ada83dba11c4ae5b57461`；完整纳入固定上游 `a60a29549f488a854966aaec9541abbe006cac22`，103项处置无跳过/延期。63个二开编号与同步前完全一致，保留上方全部定制边界，不新增默认开启能力。
- 前后端全量测试、类型/静态检查、Linux及Windows embed构建、两组Linux race、Wire一致性、部署静态/状态机/证据、隔离空业务库恢复启动和健康检查最终通过。前端由362文件2729项增至364文件2763项，Canvas8文件34项通过；全部实际命令、首轮失败及修复见同步历史最终验收节。
- 组件级浏览器夹具核验Claude重置次数查询、模型映射冲突拒绝与保存前预览同步，以及桌面/移动端亮暗截图；不代表全应用E2E或真实Provider、兑换、支付、邮件、生产历史数据和峰值负载验收。
- 保留已存在的问题：Apple Container原生macOS未验证，Linux同一平台检查失败；Canvas依赖扫描的两项fast-uri high在固定原main及候选中相同且未获豁免，发布前仍须处理。没有通过新增安全豁免或调整依赖掩盖失败；本次不声称完整CI通过。
- 两份生产Compose一致，版本仍0.1.246，来源元数据为上游0.2.10；未推送、发布、部署或连接业务服务器，未改生产资源、超时、挂载与实例秘密。验收后按授权将本地main快进到最后的记录提交，最终SHA在交付总结报告。

### 2026-09-30 上游同步发布准备 v0.1.247

- 用户追加授权将本轮同步提交至Git，通过CI后合入主线，再创建新标签并由GitHub Actions自动蓝绿部署；该授权不回写或改变上面“仅本地同步”阶段的事实。
- 关联 `CUST-OPS-004`：Canvas将fast-uri从3.1.6定向升级至兼容v3的3.1.8，修复现有安全门禁问题，不新增审计豁免、不跨主版本升级、不改业务默认值。63个二开编号保持。
- 保留生产数据挂载、资源、3600秒响应头等待、用户串行扣费和5秒usage任务；沿用现有1小时观察及到期强制退役风险边界。发布详情见 `docs/operations/2026-09-30-v0.1.247-release.md`，此条不表示部署已完成。
- 补充验证：Canvas类型检查、34项测试、构建及现有审计门禁通过；原始审计仍有9项中危，前端保留既有xlsx的2项高危豁免。没有新增安全豁免，远端CI和生产结果待后续验收。

### 2026-09-30 v0.1.247 发布验收

- 候选、PR #25、合并后的main及标签的8项CI与2项安全门禁全部通过；以merge保留上游祖先关系，发布提交为 `cf49ac9fe7f85858cf48248b87566ceae80be776`。63个二开编号保持，Canvas安全补丁没有扩展豁免。
- 标签 `v0.1.247` 由GitHub Actions构建和发布，于北京时间03:55从green切至blue，04:56完成旧槽回收；最终stable、pending为空。版本、静态资源、数据服务身份、311条迁移账本、挂载、Compose、配置和生产资源保护项经只读核验及归档比较符合预期。
- Release `36621524840` 的10个作业及实际蓝绿步骤全部成功，双入口各3507次健康探针无错误。旧green因102个会话租约未排空而按一小时策略强退；当时工作计数均为0，但退出137、非OOM，`usage_loss_unknown=true`，不宣称无损退役。既有安全豁免和生产内存预算风险保留。
- 关联 `CUST-OPS-001`、`CUST-OPS-002`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-008`；详见 `docs/operations/2026-09-30-v0.1.247-release.md`。不把健康成功描述为全业务或长请求无损。

### 2026-09-30 主线审计变化与v0.1.248准备

- `CUST-OPS-004`：v0.1.247验收文档的候选门禁成功后，主线复跑新检出undici的两项high，原失败保留记录。定向将Canvas间接依赖7.29.0升级到官方修复版7.29.1，不跨主版本、不扩展豁免，不更改生产业务默认值。
- 继续用户原有CI后合入及Actions部署授权，以新标签v0.1.248交付修复，不移动已发布标签。上游a60a29549完整基线、63个正式编号及生产保护项保持，实际验收见 `docs/operations/2026-09-30-v0.1.248-release.md`。

### 2026-09-30 v0.1.248 补充发布验收

- `CUST-OPS-004`的undici最小安全修复以提交`211d3f29e9a8652e7f7323bda6bcada8ee7382c3`交付；候选、main、标签的8项CI和2项安全门禁均成功，不扩展豁免。此前v0.1.247验收文档主线安全失败事实保留；本次不改变上游完整基线或63个正式二开编号。
- GitHub Actions Release `36635585434` 首次Windows依赖下载发生HTTP/2网络错误，发布和部署跳过；同一SHA完整重跑的attempt 2全部10个作业成功，标签`v0.1.248`不移动旧标签。06:04切至green，07:05完成旧blue回收，最终stable、pending为空。
- 一小时到期按既有策略强退旧blue：HTTP/SSE各1、会话租约111，退出137、非OOM，虽然记录的用量丢弃数为0，仍标记`usage_loss_unknown=true`。该证据绑定本次发布SHA，不宣称请求或用量无损。
- 生产和Actions归档逐项比对通过：311条迁移账本、数据服务身份与重启次数、数据挂载、Compose、持久配置、资源、3600秒响应头等待及5秒usage任务不变；保留用户串行扣费。双入口各3506次健康探针无错误，新实例切流后未重建，版本及8个静态资源复验正确。
- 关联`CUST-OPS-001`、`CUST-OPS-002`、`CUST-OPS-003`、`CUST-OPS-004`、`CUST-OPS-008`，不新增默认开启功能。Canvas仍有8项中危，前端既有xlsx高危豁免及原内存预算风险保留；未执行真实收费业务或峰值压测。完整边界见 `docs/operations/2026-09-30-v0.1.248-release.md`。

### 2026-10-02 上游增量适配 b8dece900

- 类型：上游适配；版本继续保持 `0.1.248`；上游来源更新为 `0.2.13 / b8dece9000c68815a5b867ca5a1e6f236e173905`，不新增、复用或删除二次开发编号。
- 影响编号：`CUST-GW-001`、`CUST-GW-003`、`CUST-GW-008`、`CUST-GW-010`、`CUST-ACC-003`、`CUST-ACC-005`、`CUST-ACC-006`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-BILL-005`、`CUST-PROD-006`、`CUST-UI-002`、`CUST-UI-004`、`CUST-UI-005`、`CUST-OPS-008`；其余编号继续保持原行为和默认值。
- 保留边界：本地支付区间返利规则或用户禁用开关优先于上游充值阶梯；未配置本地规则时才启用上游赠金/折扣报价。按用户串行扣费、5 秒 usage task 超时、生产 bind mount、仅回环暴露、HTTP upstream 业务开关、3600 秒响应头等待和实例资源参数不变。
- 上游新增 TypeSafe/System One、API Key 创建限制、Claude reset 幂等、订单赠金阶梯和折扣展示均完成兼容；生成代码只保留唯一 `payment_order.bonus_amount` 字段，未删除本地订单快照和返利字段。
- 验证：同步分支执行冲突标记、未解决路径、`git diff --check`、前端 lint/typecheck/Vitest/build、后端可执行定向测试和构建；Go 1.27 Ent 自动生成受本机 Go 1.26.3 与缺失 `encoding/json/jsontext` 阻断，已如实记录，不宣称生成命令通过。未访问远程服务器、未推送、未部署。

### 2026-10-03 PR #26 发布前CI适配

- 用户追加授权提交、通过CI后合入远端主线、再由tag触发Actions蓝绿部署。原“仅本地同步”记录是此前阶段事实，不代表当前已部署。
- `CUST-OPS-004`：首轮CI发现`unit`标签下的上游测试接口与本地支付、定价契约不一致；修复测试调用，明确保守预留估算不能替代本地请求模型定价准入，不恢复映射模型计费回退。
- Canvas安全扫描检出axios与brace-expansion高危公告，定向升级至1.20.0、5.0.11，锁文件只调整这两项依赖，未增加或延长安全豁免。补充冻结安装、34项测试、类型检查和构建通过；审计为0项high/critical、12项moderate、1项low，既有门禁通过，不宣称依赖全无漏洞。
- `CUST-BILL-001`、`CUST-BILL-002`：支付订单快照测试沿用本地唯一的赠金参数；增加无价别名不因预留估算绕过定价准入的断言，按请求模型计费及串行扣费行为不变。
- `CUST-BILL-001`：带unit标签的价格回归发现`gpt-6.1-sol`已合入价格表但遗漏本地严格计费名称规范化，补齐已知名称、推理后缀及compact别名，保留未知后缀拒绝；不改价格值，不启用映射型号兜底。WSL Windows挂载目录的Unix socket和权限失败单独按本机环境问题记录，在工作区内tmpfs复验，不放宽测试断言。
- `CUST-OPS-007`：当前生产为v0.1.248，新增两条241迁移及Ent schema变化被既有迁移门禁拒绝。仅执行只读结构核验，未增加审批、未执行DDL、未创建发布标签；专项适配须另行确认。后续实际门禁和发布结果见`docs/operations/2026-10-03-upstream-release.md`。
- `CUST-PROD-007`、`CUST-OPS-004`：候选`7887a6c4d`的push/PR各8项CI全部成功，但Canvas新增检出braces 3.0.3高危公告`GHSA-vfj7-8cjw-p6xm`，未将其豁免。项目仅使用shadcn包的静态CSS，固定保留4.18.0样式规则和MIT许可，移除未使用的CLI及其braces/micromatch/fast-glob依赖链，不迁移到开发依赖规避审计。冻结安装、34项测试、类型检查和构建通过；11个构建文件逐字节相同。审计为0项high/critical、1项moderate、1项low，既有门禁通过。后端全量unit在工作区tmpfs复验通过并已卸载临时挂载，新提交仍须独立通过远端门禁后才能合入。
- `CUST-OPS-007`：用户于2026-10-03明确批准两条241迁移专项适配。赠金迁移绑定149账本和既有列精确定义；平台迁移绑定238账本、两表原约束和固定Ent差异，执行原SQL后同事务安装typesafe写入保护，旧槽退役且重新核验身份、挂载与健康后才解除，失败和回滚保留。限PostgreSQL18、16MiB普通无自定义触发器/RLS/继承表、NOWAIT锁和1/5/10秒超时，不改原SQL或放宽通用门禁；发布证据与状态机回归通过，数据库集成和后续CI/部署进度见本轮发布记录。

### 2026-10-04 v0.1.249发布验收

- `CUST-OPS-007`：专项提交`2349a7623`经完整门禁以PR #27合入`6e4161ea9`；主线门禁成功后由新标签触发Actions发布v0.1.249，标签门禁也已全部成功。仅新增两条固定241账本，原311条摘要不变；旧green于00:43回收后，两个TypeSafe兼容保护经固定身份、挂载和双入口健康复核自动解除，原CHECK和赠金列保留。
- `CUST-OPS-001`、`CUST-OPS-003`：Release `37133683269`的attempt 1全部10个作业成功，最终blue稳定且pending为空；数据服务身份、挂载、双Compose、配置、资源、3600秒响应头等待和5秒usage任务未变。首页及公开接口版本0.1.249、8个静态资源正确，双入口各3507次探针无错误。
- 本轮按既有一小时策略强退旧green，HTTP工作计数3、退出137、非OOM，用量丢弃摘要为0但完整性未知，不能称为无损发布。保留用户串行扣费、不扩展安全豁免或上游范围；真实收费业务、完整E2E、备份恢复及峰值验证仍未执行。完整证据见`docs/operations/2026-10-03-upstream-release.md`。

## 2026-10-07 上游增量同步与支付安全适配

- 时间：`10/07/2026 12:23:17`。用户已批准固定范围的本地合并、兼容处理及两项 XLSX 既有豁免续期；本轮不推送、不部署、不访问生产，不做账务或封禁处置。
- 固定上游范围：`b8dece9000c68815a5b867ca5a1e6f236e173905..3f1a2ea0a760730e3bc528105c00b4ee4f23e469`，共8个提交，完整保留祖先关系；二开编号仍为63项，没有新增、退役或删除编号。
- 更新功能项：`CUST-PROTO-001`、`CUST-PROD-006`、`CUST-OPS-002`、`CUST-OPS-004`、`CUST-OPS-008`。支付补丁不替代历史异常余额核对，也不会自动追回既有损失。
- 保留边界：严格计费、用户串行扣费、5秒 usage task 超时、支付赠送和幂等补发、流式安全重试、蓝绿退役保护、生产 bind mount/回环监听/资源配置及允许 HTTP upstream 均不改动。
- 同步前已用 Go 1.27.0、Node 20.20.2、pnpm 9.15.9、golangci-lint 2.13.0 完成默认/unit/真实本地容器集成测试、构建、前端2832项测试、Canvas34项测试、部署静态和模拟测试及隔离启动。Windows/WSL 初始环境失败与修正证据保留在忽略的 `tmp/upstream-sync-20261007`；不通过修改业务代码、删除断言或关闭安全检查消除失败。
- 旧代码负对照中，签名复用、订单 URL 重放和未知参数三项攻击回归按预期失败，合法回调通过；候选补丁的正向复验与最终分支更新结果追加到同步历史，不将当前合并阶段描述为已发布或已部署。

## 2026-10-09 上游增量同步兼容适配

- 固定范围：`3f1a2ea0a760730e3bc528105c00b4ee4f23e469..3a6fd1c9db07203ca308aaba69e502bc1f35b307`，146 个提交；采用完整 merge 保留祖先关系，二开编号保持 64 项，不新增、删除或退役编号。
- 更新上述 15 项清单的上游适配边界。保留 Kimi 动态工具 `CUST-GW-016` 与五项参数兼容、按请求模型严格计费、逐用户串行扣费、5 秒 usage task 超时、支付及赠金、Canvas、蓝绿退役保护和生产数据/网络/资源配置。
- 获批的 15 个文本冲突按功能合并解决；缓存、测试默认模型、提示词、白名单预览与首字后输出速率保留本地语义，对应上游提交记录为 `Applied + Overridden`，不记为跳过。
- 242 迁移仅在本地隔离数据库验证，不修改原 SQL 或蓝绿通用放行门禁。未来生产发布仍需专项审批；本次不推送、不建 PR、不创建标签、不部署、不访问远程服务器。
- 修复蓝绿测试夹具随当前 Ent schema 漂移的问题：241 专项用固定历史文件与原摘要校验，补充 242 删除约束仍被现有门禁拒绝的回归；不增加审批、不修改运行时门禁。
- `CUST-BILL-001`、`CUST-BILL-002`：上游新增国产平台/OpenCode 的 Claude 缺价测试对齐既有本地契约，明确断言缺价错误、保留请求模型、零费用审计与不扣费；修复 OpenCode Go 候选模型被过滤后未进入零费用审计分支的问题，严格缺价错误与不扣费策略不变，不删除落账断言。
- 本条随代码合并提交记录执行事实；完整基线、候选验证、逐提交处置、提交映射及未验证项在最后的同步验收记录补齐，不将合并中状态描述为已发布。

## 2026-10-13 上游增量同步兼容适配

- 固定范围为 `3a6fd1c9db07203ca308aaba69e502bc1f35b307..4d9ac3e482106d2da3a39fe3e546d8d6801f1cc1`，共 79 个提交，采用完整 merge；本地版本保持 `0.1.254`，来源更新为上游 `0.2.16`。64 个二开编号保持，不新增、删除、复用或退役编号。
- 同步更新清单中的 `CUST-GW-001/003/004/006/008/013/014/015/016`、`CUST-BILL-001/002/003/004/006`、`CUST-UI-004`、`CUST-OPS-004/007/008`。Gemini 显式缓存默认关闭、缓存对象绑定账号；缓存报价与入账共同遵循请求模型及显式渠道价，不回退到响应或映射模型，不给未知系列兜底价格。纯存储续期不收 token 或按次费，缺价明确报错。
- Gemini 必记账任务沿用按用户串行队列，队列满或停止时同步回退，超时保持 5 秒；保留在途预留交接。Claude 模拟 beta 过滤、Kimi 动态工具、首次输出控制、插件接管错误、提示审计和请求模型计费语义不变；新增读取完整性判定，仅完整 400 错误且未提交响应、未取消时允许结构输入降级。
- 长上下文附加价同时满足账号和分组门禁，保留显式区间及禁用开关优先级；页面根节点浮层、分组本地化、监控刷新与生图成功率共存，延迟返回的旧明细不能覆盖刷新结果。账号浮层测试分别检查浮层标题与单元格内模拟配额说明，不删除原有断言。
- 4 条新增迁移保持上游原文，仅在本地隔离数据库验证；生产蓝绿迁移门禁、双 Compose、绑定挂载、回环监听、HTTP upstream、3600 秒响应头等待、5 秒 usage task、实例资源参数及既有安全豁免不变。没有推送、PR、标签、部署或远程服务器访问。
- 本条记录合并适配事实；同步前已有时序和内存阈值测试失败、候选复验、逐提交处置与未验证项在 `docs/upstream-sync-history.md` 最后追加验收记录，不将本地验证等同于生产或真实供应商验收。

### 2026-10-13 分组本地化与用量导出补充适配

- `CUST-BILL-005`：上游新增用户 DTO 本地化检查发现本地游标导出遗漏；导出复用列表的 `Localized(Accept-Language)` 转换，返回用户首选语言的分组名称并移除完整译文表。认证、所有者限定、筛选、游标、每批上限和管理字段脱敏保持。
- 现有导出权限测试同时检查英文译名与译文表不外泄，保留原有全部断言；与同步历史同提交记录，累计涉及 19 项既有清单，正式编号仍为 64 项。本次为获批分组多语言兼容范围内的遗漏修复，不调整生产配置或新增业务取舍。

### 2026-10-14 Gemini 缓存记账上下文补充适配

- `CUST-BILL-003`：真实 Google 鉴权入口把用户标识写入 Gin，而必记账队列从请求上下文读取。新增缓存接口的公共前置校验将已鉴权标识传入请求上下文，覆盖陈旧标识并保留请求 ID，使此前接入的按用户队列在真实请求中生效；只作用于本轮新增缓存接口。
- 补充从实际 Gin 鉴权上下文到记账队列键的回归，保留同用户串行、其他用户并行、满队列/停止时必执行及 5 秒回退断言。两份台账同提交维护，19 项适配及 64 个编号的范围不变。

## 2026-10-16 上游增量同步兼容适配

- 用户已批准固定范围 `4d9ac3e482106d2da3a39fe3e546d8d6801f1cc1..b0a02a3304a0a78cdd003ddda1dd23ba02c03084`，48个提交（30个普通、18个合并），通过完整 merge 保留祖先关系。本地版本保持0.1.256，上游来源为0.2.17；65个稳定编号全部保留。
- 同步更新上述13项清单。Gemini 的首 Token 生命周期与单次错误策略并存；Responses 时间戳与本地写锁、后到成功终态优先和用量结算并存。账号筛选保持常显与原测试，支付查询保留嵌套字段及等待状态判定。
- 新退款逻辑保持待确认扣款，失败终态原子归还；EasyPay 成功回调增加上游确认及有限金额校验。本地赠送快照、补查年龄/批量/时间预算及幂等履约保持。OpenAI 订阅型号限制按真实凭据和最终目标生效，API Key 路径保持。
- Kimi 五项兼容重试及动态工具、请求模型计费、严格缺价、按用户串行扣费、5秒 usage task、降智检测默认关闭、Canvas、历史迁移、生产配置及发布门禁均保留。纯上游最小输出配置默认128且最低16，Web 搜索模拟及其 OAuth 排除一并纳入，不新增二开编号。
- 本条随适配代码记录本地合并事实。完整同步前后检查、48项处置、实际提交映射及未验证边界在同步历史的最终验收记录追加；未推送、建PR、打标签、部署或访问服务器。

### 2026-10-16 Gemini 错误策略静态检查修正

- `CUST-GW-001`：删除 Chat Completions 重试循环中必然被当前响应策略覆盖的冗余 `checkedPolicy = nil` 赋值。请求构造或传输失败均直接返回，其余路径在退出循环前必定保存当前响应策略，因此每响应一次检查与首 Token 生命周期不变。
- Messages 和 Native 两条存在缓存响应提前退出的路径仍保留每轮清空策略。此项仅处理合并后的 `ineffassign` 失败，不删除断言、不关闭静态检查；修正后的验证结果在最终同步记录追加。

### 2026-10-25 上游增量适配（本地版本 0.1.259）

类型：上游适配；本次同步涉及 `CUST-GW-001`、`CUST-GW-002`、`CUST-GW-004`、`CUST-GW-005`、`CUST-GW-006`、`CUST-GW-007`、`CUST-GW-008`、`CUST-GW-012`、`CUST-PROTO-001`、`CUST-ACC-001`、`CUST-ACC-005`、`CUST-BILL-001`、`CUST-RISK-001`、`CUST-UI-001`、`CUST-OPS-004`、`CUST-OPS-008`。全部 66 个稳定编号继续保留，无退役或复用。

按用户批准完整合并固定上游 3f1c8f36c。特别保留原子人工暂停、事故 ID 条件恢复、按请求模型计价及成功审计；受限密钥恢复分两阶段：错误字段在授权事务内更新，提交后重置 Redis，再以独立事务条件清除原事故。Redis 失败保留事故阻断并返回错误，不伪装成全部回滚。新增缓存隔离并发测试、事务回滚和恢复失败测试。

本提交记录实现；基线、最终验证命令与结果、尚未验证的真实供应商及生产路径统一记录在本轮 docs/upstream-sync-history.md 追加记录。

2026-10-25 同轮验证修正（`CUST-GW-001`、`CUST-GW-008`）：兼容流自行生成的终止错误帧明确结束首输出等待，但不计为生成首 Token；已超时、客户端已取消或前奏溢出的尝试继续丢弃缓冲并由原逻辑处理。Chat/Responses 已写终止帧时不再重复追加通用错误；首输出前可重试错误继续换号，保持成功审计和单次结算。补充终止帧与首 Token 状态边界测试，并保留上游异常终态断言。

2026-10-25 同轮定价验收适配（`CUST-BILL-001`）：Grok 4.7 Fast 接入上游基础价与长上下文倍率，继续沿用本地严格大于 200K 才加价的边界。新增边界恰好等于和超过一个 Token 的分项费用断言；保留标准型号不被 Fast 遮蔽、缓存费和倍率检查。此项仅适配上游测试对已批准本地规则的预期，不改计费实现或默认值。

### 2026-10-25 PR #44 安全门禁修复

- 关联 `CUST-PROD-007`、`CUST-OPS-004`：首轮远端 Security Scan 在 Canvas 检出 DOMPurify 的 1 项 critical、1 项 high，以及 follow-redirects 的 2 项 high。具体公告和失败作业记录见同步历史追加节。
- Canvas 在现有 pnpm overrides 中将 DOMPurify 从 3.4.13 定向覆盖至 3.4.17、follow-redirects 从 1.16.0 至 1.16.1。锁文件只更新这两个包及其引用，其他依赖版本未变；未新增或延长安全豁免，保留全部原有检查。66 个稳定编号保持。
- 冻结安装、类型检查、8 个文件共 34 项测试和构建通过。原始生产依赖审计退出码为 1，剩余 1 项 low、1 项 moderate，high/critical 均为 0；仓库现有审计门禁退出码为 0。构建仍有已有分包体积提示。
- 本条与依赖补丁同提交；重新推送后的远端 CI 与合并结果以 PR #44 检查记录和交付答复为准。未发布标签、部署或访问生产服务器。

## 2026-10-30 上游 0.2.19 本地兼容集成

- 用户已集中批准完整同步 73 个上游提交，范围为 `3f1c8f36ce8c2cd378015830a90e89970b9a9ea7..a3298b46ebe239fb0720b068a1eb3383955bdac2`。保留全部 66 个能力编号，本地版本保持 `0.1.261`。
- 对应清单项已逐项更新：`CUST-GW-001`、`CUST-GW-003`、`CUST-GW-004`、`CUST-GW-008`、`CUST-PROTO-003`、`CUST-ACC-001`、`CUST-ACC-005`、`CUST-BILL-001`、`CUST-BILL-002`、`CUST-BILL-003`、`CUST-PROD-002`、`CUST-OPS-007`、`CUST-OPS-008`。
- DeepSeek Pro 默认价不再按日期切换 Flash，Claude 指纹改为 UTF-16 码元规则，均按本轮批准方案实施。用户请求模型计费、严格缺价、自定义价格优先及长上下文策略保留。
- 图片按张开关默认关闭；严格计费、渠道上下文分档、图片交付标记和流式断连结算共同保留。新增开关两种状态下的断连回归，OAuth 流式断连不得恢复请求 n 兜底。
- Grok 目录接入诊断及过期结果隔离，沿用本地预览接口；普通 Gemini 测试默认 3.8，Google One 使用保守列表；Terra 接入本地严格定价归一化。
- 迁移仅增加原始 248，历史 SQL、生产 Compose、迁移门禁及资源配置不变。串行扣费和 5 秒 usage task 超时保持。本次仅本地同步，不进行发布或生产操作。
- 本条记录代码适配；候选验证及最终集成状态以 `docs/upstream-sync-history.md` 的本轮验收记录为准。

### 同轮候选验证适配（2026-10-30T04:26:23+08:00）

关联 `CUST-GW-008`。修复合并后的 OAuth 非流式响应分支，使上游强制 stream=true 不再改变客户端响应形态，保留 compact 的客户端标记及 SSE 桥接。上游新增测试按本地账号、设置服务和双流式参数签名调用；前端翻译桩断言保留计数与总数检查。扩展定向测试通过，完整后端验证继续执行。

### 同轮认证快照测试适配（2026-10-30）

关联 `CUST-BILL-001`。全量测试发现本地利润控制快照回归仍硬编码 v24；按照已批准的快照 v25 契约更新预期，增加图片按张计费开关与利润、长上下文、模型价格共同经过 JSON 往返后的保留断言，并同时覆盖 v20、v24 旧缓存淘汰。只修正测试契约，不改变生产逻辑或默认值；首轮失败和复测结果在同步历史最终验收节记录。
