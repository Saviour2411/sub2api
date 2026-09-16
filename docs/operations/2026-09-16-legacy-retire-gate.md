# 2026-09-16 旧版退役门禁与双槽循环验证

本文时间除特别说明外均为北京时间（UTC+8）。本记录包含独立门禁、失败发布的安全恢复，以及 v0.1.237 通过 Actions 退役 v0.1.234 并复用 18080 的生产证据。

## 目标

- 下一 tag 仍由 GitHub Actions 自动发布，不要求本机参与，也不要求人工先停止旧版。
- 旧 v0.1.234 只有在会话 TTL、旧 Nginx worker、18080 客户连接、legacy Unix 连接、Redis 归属和专属访问日志静默全部满足时才可退役。
- 退役后立即复用固定 `blue=18080`；以后只在 `18080 ↔ 18082` 两个槽位循环，不创建 18081 或持续增加端口。
- 不降低生产 `DATABASE_MAX_OPEN_CONNS=512` 或 `GOMEMLIMIT=216181080064B`，仍按实际 PostgreSQL 连接余量和可用内存门禁。

## 自动发布与独立门禁

- Release 在固定候选镜像已拉取并核验 revision 后，自动执行最长 1200 秒的旧版退役观察，要求客户路径连续静默 900 秒；没有预热凭据时也能在同一次 tag 发布内完成，不依赖人工工作流。
- 独立 `Legacy retirement production gate` 工作流只允许从 `main` 手动触发，与 Release 共用生产互斥。它可以提前形成 900 秒静默凭据，但不停止容器；Release 仍会重新核对容器、切流时间、活动槽和活动实例身份。
- 门禁凭据绑定旧容器 ID、首次切流时间、活动槽和活动实例 ID。访问日志 marker 变化、实例变化、连接或归属重新出现都会清零静默计时。
- 观察重跑不修改日志时间，保留独立门禁的静默凭据；代理配置失败时恢复原配置。封闭 legacy 通道必须重新加载 Nginx 并确认 Unix listener 已消失，不能以磁盘配置相同推断运行配置已生效。
- 旧版封闭前再次核验候选镜像已存在于本机。退役中断恢复使用已持久化的发布身份和本地镜像，不在旧版停止后重新联网拉取镜像。
- 退役记录包含执行退役的 release SHA、退出码、`usage_record.task_dropped` 和 `Server forced to shutdown` 次数。服务恢复优先，候选继续启动；但任一计数非零会把执行退役的本次 Release 证据判为不合格，不会永久误伤后续无关发布。
- 关停日志读取失败或缺少起始时间时 `logs_verified=false / clean=false`，不能把缺失取证等同于零异常。

## server1 只观察验证

2026-09-16 12:21:27，在 server1 使用当前候选脚本执行 `--legacy-retire-check --legacy-window 180 --legacy-quiet 0`。该操作只生成门禁证据，没有停止或重启应用容器。

- `active=green`，活动实例 `i6793ed3db4cb482aab4aa0d5626f9869`；旧容器 ID 与 `state.json` 一致。
- 首次切流后 3602 秒，已超过生产会话/response TTL 3600 秒。
- 迁移前 Nginx worker：0；18080 established：0；legacy Unix socket connected：0。
- Redis 中 owner=`legacy-v0.1.234` 的归属记录：0，TTL 为 -1/-1。
- 旧版仍有 9 条外部连接，来源包含共享后台工作；该指标只记录观测，不作为客户路径退役硬门禁。
- 专属 legacy access log marker 已建立，短采样 `blockers=[]`、`eligible=true`。正式退役仍要求 Release 或独立工作流完成连续 900 秒静默。

首次试运行发现 `sub2api_gateway` 日志格式定义在后加载的站点文件，`conf.d` 引用会使 `nginx -t` 报 unknown log format。运行中的 Nginx 未 reload，客户流量未受影响；磁盘配置立即改为内置 `combined`，`nginx -t`、reload 和 API/direct 健康检查均成功。代码已同步使用 `combined` 并增加回归断言。

## CI 与发布迭代

独立门禁 run `35057663303` 已于 13:13:43 成功，实际连续静默 901.6 秒，全部客户路径计数为 0。`3793f691c` 的 CI `35058399290`、Security `35058399264` 全通过后创建 `v0.1.236`，Release `35059222782` 成功发布镜像，但生产 Docker CLI 26.1.5 拒绝 `docker stop --timeout`；旧版没有收到停止信号，state 保持 `legacy_retiring / active=green / pending=blue`，两个应用健康，API/direct 均为 200。

迭代修正使用兼容短参数 `docker stop -t -1`，不改变无限等待语义。仅在 legacy 退役阶段且候选容器尚未创建时，允许后续 tag 先通过完整预检和固定镜像准备，再接续原容器/活动实例绑定的操作；保留原发布身份用于审计。不移动失败标签，不手工停止应用，不升级生产 Docker。

补强提交的首轮 CI 在隔离 Nginx 初次 warmup 时出现 TLS `SSL_ERROR_SYSCALL`，尚未开始任何切流。测试启动顺序修正为：Docker 返回端口后，先轮询双入口无付费 `/health` 至 TLS 就绪，再发送一次 warmup 生成请求；不重放生成，不放宽切流及用量断言。

最终候选 `004cc47a2090a35b7cc94a6463e97273b86562fb` 的 CI run `35060652262`、Security run `35060652452` 均通过后快进主线并创建 annotated tag `v0.1.237`；标签 CI run `35061508019`、Security run `35061507906` 也通过。状态机 30 项、发布证据 2 项、预检 3 项通过，Linux 真实应用 20 轮混合协议门禁保留。

## v0.1.237 生产结果

Release run `35061507913` 由 tag 自动触发，复用之前未完成的退役操作，没有本机执行停容器或切流命令。

该 run 于 14:27:52 完成，`deploy-production` 为 14:10:29–14:27:51（17 分 22 秒）且全部步骤成功；这证明发布执行及 legacy 退役门禁通过，不等于客户流量整体零错误。

| 事件 | 北京时间／耗时 | 结果 |
| --- | --- | --- |
| 旧 v0.1.234 收到正常停止信号 | 14:10:52 | 客户路径静默门禁已通过；无强杀倒计时 |
| 旧容器完成退役并删除 | 14:12:43.841；111.84 秒 | 等待后台监控任务自然收尾，exit code 0 |
| 候选接流确认 | 14:12:47.022 | 新版 blue 复用 18080；旧版退役至确认新流量 3.18 秒 |
| Nginx 切流确认 | 1.179 秒 | API/direct 同时返回同一 blue 实例，HTTP/2 200 |

- `legacy_retire_result`：`exit_code=0`、`usage_drop_count=0`、`forced_shutdown_count=0`、`logs_verified=true`、`clean=true`。旧 `sub2api` 容器已不存在，legacy Unix 入口已移除。
- 当前镜像固定为 `saviour2411/sub2api@sha256:a7f5df1d863247d54aed3c658b2bc666a48870353f5bb0f66fa3b0e8d3f0657c`，运行 revision 与候选 SHA 一致。
- blue 的 `LIFECYCLE_LEGACY=false`，共享后台任务已恢复；green 仍为旧版共存模式，不与 blue 重复执行共享任务。
- blue/green 的连接池均保持 512，GOMEMLIMIT 均保持 216181080064B。PostgreSQL、Redis、持久挂载及计费规则未修改。
- 旧版关停的 111.84 秒不是客户停机时间：此期间 green 持续接单；新 blue 完成就绪后才切流。

## 正常双槽排空边界

首次 legacy 阻塞已解除，但 green v0.1.235 的真实 SSE、断连排空工作和会话租约仍需自然结束。不能把 v0.1.234 成功退役等同于所有保留实例都已退役。

15 分钟观察结束后的实际状态为 `phase=drain_pending / active=blue / pending=green / last_error=null`，不是 `stable`。green 的请求、SSE 和会话租约尚未归零，已按要求保留，没有停止容器或缩短 TTL。

下一次候选固定使用 `green/18082`，没有 18081 或第三槽。执行新发布前先检查 green：已排空则自动回收、复用；尚有工作则安全拒绝，不改变 blue 流量。不为发布缩短会话 TTL，也不强杀连接。因此“每个 tag 自动执行部署流程”不等于“旧会话永不结束时仍能无限次立即切换两个槽位”。

## 健康观测异常：零错误验收未通过

- 整个服务器执行窗口内 API/direct 各探测 977 次，API 7 次异常（5 次 TLS、2 次 HTTP 500），direct 6 次异常（4 次 TLS、2 次 HTTP 500）。没有删除或重试这些失败样本。
- 异常集中在 14:27:33–14:27:43，距 14:12:47 切流约 15 分钟。对应 Nginx 日志为 worker `913781` 的 `4096 worker_connections are not enough`，包含 `while connecting to upstream`。
- 同类告警在本次切流前已存在。只读统计当天仍保留的 error.log、API error.log、direct error.log：切流前分别 67/72/2332 条同类告警；该计数是日志行而非唯一失败请求数，前后窗口长度不同，不据此比较错误率。
- 当时生产 Nginx 为 `worker_processes auto`、`worker_connections 4096`、`worker_rlimit_nofile 65535`。应用两个本地 `/health` 为 200，内存和磁盘有余量；随后双公网入口回环探针也恢复为 200。证据指向既有代理连接容量瓶颈，但不足以承诺所有客户请求都未受影响。
- 本轮不擅自改变全局 Nginx 并发参数。legacy 退役验收通过、发布成功与整体零错误目标分开记录；全局代理容量及连接分配需独立审定和验证。
