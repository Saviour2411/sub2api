# 2026-09-16 旧版退役门禁与双槽循环验证

本文时间除特别说明外均为北京时间（UTC+8）。本记录先描述代码与只观察验证；实际旧版停止、下一 tag 切换及最终结果在发布后继续补记。

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

## 待完成验证

独立门禁 run `35057663303` 已于 13:13:43 成功，实际连续静默 901.6 秒，全部客户路径计数为 0。`3793f691c` 的 CI `35058399290`、Security `35058399264` 全通过后创建 `v0.1.236`，Release `35059222782` 成功发布镜像，但生产 Docker CLI 26.1.5 拒绝 `docker stop --timeout`；旧版没有收到停止信号，state 保持 `legacy_retiring / active=green / pending=blue`，两个应用健康，API/direct 均为 200。

迭代修正使用兼容短参数 `docker stop -t -1`，不改变无限等待语义。仅在 legacy 退役阶段且候选容器尚未创建时，允许后续 tag 先通过完整预检和固定镜像准备，再接续原容器/活动实例绑定的操作；保留原发布身份用于审计。不移动失败标签，不手工停止应用，不升级生产 Docker。

补强提交的首轮 CI 在隔离 Nginx 初次 warmup 时出现 TLS `SSL_ERROR_SYSCALL`，尚未开始任何切流。测试启动顺序修正为：Docker 返回端口后，先轮询双入口无付费 `/health` 至 TLS 就绪，再发送一次 warmup 生成请求；不重放生成，不放宽切流及用量断言。

- 功能分支 CI 与 Security Scan 全部通过并合入 `main`。
- 创建下一 annotated tag，验证旧版退出码为 0、两个异常计数均为 0。
- 验证 `sub2api` 消失、`sub2api-blue` 在 18080 启动、两入口切到 blue，green 自然排空后删除。
- 最终状态必须为 `stable / active=blue / pending=null`；再确认下一发布目标为 `green/18082`。
