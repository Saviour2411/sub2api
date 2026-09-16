# 2026-09-16 Nginx 连接容量与 worker 分配修复

本文时间均为北京时间（UTC+8）。关联 `CUST-OPS-003`，仅操作 server1，不改变应用镜像、计费、数据库、蓝绿活动槽或客户会话有效期。

## 结论与授权范围

经用户授权，于 2026-09-16 15:50:27 平滑加载两项生产 Nginx 配置变更：

| 参数 | 原值 | 新值 |
| --- | --- | --- |
| `worker_connections` | 4096 | 16384 |
| `multi_accept` | on | off |

`worker_processes auto`、`worker_cpu_affinity auto`、`worker_rlimit_nofile 65535` 及 systemd 的 `LimitNOFILE=65535:524288` 保持不变。没有增加 `reuseport`、强制关停超时或修改监听端口。应用连接池 512、内存软上限、PostgreSQL 600 连接上限不变。

## 分配不均的具体原因

现场 Nginx 为 Debian `1.26.3-3+deb13u8`。读取实际 worker 的 `/proc/<pid>/fdinfo`，确认监听事件注册使用 `EPOLLEXCLUSIVE`（事件掩码包含 `0x10000000`）；没有配置 `reuseport` 或显式启用 `accept_mutex`。

当前 Debian 版本的 `src/event/ngx_event_accept.c` 与官方 `release-1.26.3` 文件逐字节一致，接入路径如下：

1. Linux 的 `EPOLLEXCLUSIVE` 监听唤醒可能偏向最先注册的 worker；Nginx 源码自身对此有说明，并用 `ngx_reorder_accept_events()` 定期重新注册监听事件，让其他 worker 有机会接入。
2. 在 Linux epoll 路径，`multi_accept on` 把 `ev->available` 设为真，`accept` 循环持续接收等待中的新连接。
3. 队列接空时 `accept()` 返回 `EAGAIN`，该版本直接从 `ngx_event_accept()` 返回，绕过位于循环末尾的 `ngx_reorder_accept_events()`。分配轮转因而不能正常发挥作用。
4. 少数 worker 不断接到连接，长 SSE、WS 和同一 HTTP/2 连接上的并发流进一步保留在原 worker；它们的连接槽耗尽，并不要求其他 worker 或整台机器同时满载。
5. 改为 `multi_accept off` 后，每次接收一个连接，执行路径可以到达轮转逻辑。提高连接槽是增加余量，关闭批量接入才是本轮针对分配偏斜的修复。

源码依据：

- `https://sources.debian.org/data/main/n/nginx/1.26.3-3%2Bdeb13u8/src/event/ngx_event_accept.c`
- `https://github.com/nginx/nginx/blob/release-1.26.3/src/event/ngx_event_accept.c`
- `https://nginx.org/en/docs/ngx_core_module.html#worker_connections`
- `https://nginx.org/en/docs/ngx_core_module.html#multi_accept`

这是对当前版本、当前事件模型及现场对照结果的结论，不泛化为所有 Nginx 版本或所有操作系统的行为。已有 HTTP/2、SSE、WS 连接不会迁移到新 worker，业务工作量也不保证平均到每个 CPU。

## 修改前证据

- 15:47:51：256 个 worker 中，仅 2 个持有本次统计的业务 TCP 连接，分别为 547 和 56 条（包含客户端及到应用的连接）。
- 对 API/direct 各建立 64 条无付费 `/health` TLS 连接，验证 HTTP 200 后保持连接，按本地源端口与 `ss -p` 的服务端 worker 匹配：API 的 64 条全在 worker `913781`；direct 为该 worker 63 条、另一 worker 1 条。
- 机器可用内存约 215.68 GiB，负载约 0.87；运行时每个 worker 的文件描述符上限为 65535。不能把当前采样当成事故时的历史 CPU/内存曲线，但配置槽位耗尽已有明确日志证据，不需要据此先升级机器。
- 原先 13 次发布探针异常中，4 次 HTTP 500 与同秒 `/health` 的 `worker_connections are not enough while connecting to upstream` 日志逐条匹配，访问日志为 `upstream_status="-"`。9 次 TLS 失败与接入侧告警高度吻合，但缺少逐连接取证，不能逐条断言。
- 14:27:30–14:27:46 的同一窗口还有 94 条非探针请求返回 500；13 次仅是探针失败数，不是客户影响总数。

## 操作保护

- 获取与蓝绿发布相同的 `deploy/blue-green/deploy.lock`，拒绝与发布并发操作。
- 校验原配置 SHA-256，候选必须恰好只修改上述两行；原配置变化时拒绝覆盖。
- 候选及安装后均通过 `nginx -t`；同文件系统原子替换，配置和重载失败时可恢复备份并再次平滑加载。
- 只执行 `systemctl reload nginx`，不 restart、不发送强杀信号，不配置 `worker_shutdown_timeout`。
- Nginx master 在前后均为 `5679`；blue/green 的容器 ID 与启动时间不变，蓝绿 `state.json` 哈希不变。
- 原配置备份位于服务器项目内：`/root/proj/sub2api/deploy/blue-green/nginx.conf.before-balance-20260916T155021`。

## 修改后验证

| 验证项 | 结果 |
| --- | --- |
| 新 worker 启动 | 256 个，master 未重启 |
| API 64 条独立健康连接 | 64 个 worker，各 1 条；全部 HTTP 200 |
| direct 64 条独立健康连接 | 64 个 worker，各 1 条；全部 HTTP 200 |
| 跨重载连续探针 | 15:50:21–15:53:38，714 次，失败 0 |
| 探针协议 | 每入口 HTTP/2 179 次、HTTP/1.1 178 次 |
| 连接耗尽日志增量 | 三份 Nginx error log 均为 0 |
| 应用重启、蓝绿切流 | 均未发生 |

15:55:19 的后续自然业务采样：37 个新 worker 持有业务连接，共 281 条，单 worker 最多 39 条；其中客户端连接为 86 条。旧 worker `913781` 仍有 21 条业务连接，继续自然排空，没有为了改善统计而强制断开。

15:59:36 再次复核：`nginx -t` 通过，三份错误日志自重载后连接槽耗尽告警仍为 0；保留 1 个排空中的旧 worker，blue/green/PostgreSQL/Redis 全部健康。

新上限 16384 是本次带余量的初始配置，不是峰值容量或 SLA 证明。旧 worker 在排空期间仍沿用旧配置，不应通过强停它们消除残余风险。应继续观察高峰期连接分布、每 worker 占用、5xx、CPU、内存及文件描述符。

本轮验证只证明新连接分配改善、探针跨重载成功和连接耗尽告警未再出现，不宣称整个业务零错误：15:50:27–15:54:26 仍有记录 `upstream_status=502/503` 的业务响应，而该窗口 Nginx error log 没有 error/crit/alert/emerg。此前窗口也有上游错误，业务量和请求构成不同，不能根据前后短窗口直接比较错误率或将全部上游错误归因于本次调整。

## 证据与后续发布

- 本机原始证据在项目 `.cache/nginx-balance-before-20260916.json` 与 `.cache/nginx-balance-result-20260916T155021.json`，包含探针逐次结果、配置测试、进程身份、日志增量及端口到 worker 的匹配计数。
- 服务器完整报告在 `/root/proj/sub2api/deploy/blue-green/nginx-balance-result-20260916T155021.json`；诊断脚本及备份也在同一项目目录，没有写入全局临时目录。
- 生效配置是 `/etc/nginx/nginx.conf` 的 `events` 块。普通 tag 蓝绿发布只维护对应站点及活动 upstream，不需要为本次代理参数调整发布应用镜像或新增标签。
- 回退只在发现变更新问题时执行：先取得相同发布锁、核对当前配置和备份，把备份恢复到 `/etc/nginx/nginx.conf`，通过 `nginx -t` 后 `systemctl reload nginx`。不得 restart 或杀掉排空 worker；回退会重新引入 4096 和批量接入，不应作为例行操作。
