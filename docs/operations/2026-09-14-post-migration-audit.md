# 迁移后的资源配置与安全检查

检查日期：2026-09-14，北京时间 04:14 起。

本文是调整前的历史审计，不能作为当前运行值。后续按用户明确授权实施的参数、SSH 22748及保留密码登录等设置，见 `2026-09-14-production-resource-update.md`。

本次仅执行只读核验，没有修改生产配置、重启服务、升级软件、进行压力测试或发起付费请求。新机为 `216.152.153.86:22`，旧机为 `152.53.115.95:22748`；没有连接服务器2。本文建议均未执行。

## 结论与优先级

1. 旧 PostgreSQL 的 POSIX 动态共享内存空间不足问题没有随硬件迁移消失。新旧容器 `/dev/shm` 均为 64 MiB，建议先通过生产 Compose 的 `shm_size` 修正。
2. 新机 SSH 允许 root 密码登录，旧机禁止。新机 root 密码状态为已设置而非锁定，22 端口对公网开放，建议优先恢复仅密钥认证。
3. Nginx 主进程文件描述符软上限为 1024。256 逻辑 CPU 配合 `worker_processes auto` 后，重载已出现创建 worker 失败，应先处理实际限制，而不是继续增加并发值。
4. 新机资源远大于旧机，但当前业务采样没有显示数据库连接池耗尽。不应按 CPU 数量同比例增加连接池、计费 worker 或并行 SQL。
5. `vps_backup.sh` 是备份脚本，不是自动系统更新或加固脚本。迁移完成了基础防护，没有完整复制执行旧机的 SSH、网络优化及第三方内核工具。

## 硬件与实际运行状态

| 项目 | 核验值 |
| --- | --- |
| CPU | 双路 AMD EPYC 7702，128 物理核心、256 逻辑 CPU |
| 内存 | 约 251 GiB；采样时可用约 240 GiB |
| Swap | 8 GiB；采样时使用量为 0 |
| 系统与内核 | Debian 13，`6.12.48+deb13-amd64` |
| 生产数据文件系统 | NVMe RAID1，ext4；约 871 GiB，剩余约 737 GiB |
| 数据库大小 | 约 40 GB，包括表和索引 |
| 应用版本 | `0.1.232`，未升级 |
| PostgreSQL | `18.4`，沿用迁移导入镜像 |
| Docker 资源约束 | 应用与 PostgreSQL 没有 CPU 配额或内存硬限制，CPU 可用范围 `0-255` |
| 应用入口 | `127.0.0.1:18080`，健康检查正常 |
| 生产 Compose | 两份文件字节一致；三个生产 bind mount 未变 |

瞬时容器采样：应用约使用 1 个 CPU 核心和 747 MiB 内存，PostgreSQL 约 4.46 GiB 内存。Docker 的 100% CPU 表示约一个逻辑 CPU，不是整台 256 线程主机全部耗尽。

03:21 至 04:19 的 59 条全局一分钟监控样本：连接池在用连接采样最大值为 5，空闲连接最大值为 32，一分钟窗口 QPS 最大值为 25。不能把分钟采样最大值当作瞬时并发峰值或容量上限。`db_conn_waiting` 均为空，不能声称连接池等待次数为零。源码的 `dbPoolStats` 只返回 `sql.DB.Stats()` 的 `InUse` 和 `Idle`。

## PostgreSQL 共享内存

### 已确认的证据

- 旧机保留的 PostgreSQL 容器日志中找到 87 条相关错误，形式为 `could not resize shared memory segment ... No space left on device`。可见记录从 2026-09-07 开始，最后可见相关错误为 2026-09-13 21:30:04。日志有保留范围限制，87 不是服务全生命周期总数。
- 旧容器和新容器的 Docker `ShmSize` 都是 `67108864` 字节，即 64 MiB；生产 Compose 没有 `shm_size`。
- 新容器 `/dev/shm` 采样时已使用约 8.2 MiB，而宿主机 `/dev/shm` 上限约 126 GiB。两者不是同一限额。
- 新 PostgreSQL 实际配置：`shared_memory_type=mmap`、`dynamic_shared_memory_type=posix`、`min_dynamic_shared_memory=0`、`shared_buffers=4GB`。
- 本轮检索中，03:19:55 切换后的新库日志没有共享内存空间不足、连接槽耗尽或 OOM 记录；唯一检索到的 PostgreSQL ERROR 是用户请求取消 SQL。这只说明观测窗口内没有复现，不等于高并发风险已消除。

### 原因与建议

本次错误对应 POSIX 动态共享内存空间不足，不能将其理解为磁盘容量不足或 `shared_buffers` 只有 64 MiB。并行查询可以申请额外动态共享内存；当前主共享内存使用匿名 mmap，与容器 `/dev/shm` 的 POSIX 动态共享内存限额有区别。

建议为生产 PostgreSQL 服务设置 `shm_size: 2g`，同时保持两份活动 Compose 完全一致。这是本机容量条件下的初始建议，不是经过峰值压力验证的永久充分上限。扩容后仍需监控 `/dev/shm` 峰值和错误。

通过 Docker/Compose 持久化该变化需要重建 PostgreSQL 容器；单纯 `docker compose restart` 不会应用新的容器配置。实施前应校验挂载和健康、备份配置、妥善停止应用写入与在途计费，安排短维护窗口。不得拉取新镜像、改动生产数据挂载或启动旧库。

仅增加 `shared_buffers`、宿主内存或 `kernel.shmmax/shmall` 不能修正当前 64 MiB 容器限额。修复前不应提高并行 SQL 数量或全局 `work_mem`。不建议为了绕开问题改用磁盘模拟的 `dynamic_shared_memory_type=mmap`。

### 配置与监控补充

- `work_mem=4MB`，`hash_mem_multiplier=2`，`maintenance_work_mem=128MB`。
- `max_worker_processes=8`、`max_parallel_workers=4`、`max_parallel_workers_per_gather=2`。
- 数据库统计中有 52 个临时文件、约 2540 MB 临时写入。统计窗口并非经过明确重置的对照实验，不能仅凭这一项全局提高 `work_mem`。
- 采样无死锁、无内核 OOM。`pg_stat_statements` 未安装，缺少按 SQL 聚合的峰值耗时和临时文件归因。
- `shared_buffers` 实际值是命令行的 4GB，但文件仍写着初始化默认值 128MB，`pending_restart=true`。这是需在后续配置整理中处理的来源冲突，不能据文件误判当前只生效 128MB，也不应仅为清除标志而直接重启生产库。

## 备份脚本与加固覆盖

`/root/vps_backup.sh` 的 `make_system_config_tar` 收集 Nginx、完整证书续期状态、备份 cron、Docker、Fail2ban、SSH、BBR 模块加载、指定 sysctl、UFW 等配置；另收集运维脚本和站点数据。元数据步骤读取服务、定时器、UFW、iptables 和 nftables 状态。

这些是归档与诊断步骤，不会自动执行系统全量升级，也不会应用所归档的 SSH、UFW、Fail2ban 或 sysctl 配置。生成的恢复说明是说明文本，不代表备份时已执行恢复。

随归档保存的 `/root/tcpx.sh` 包含安装和删除内核、更新引导、BBR/BBRplus/XanMod/Cloud 等选项、TCP 缓冲与拥塞控制、系统资源限制、IPv6 开关以及轻量防 CC 参数等功能。迁移没有运行这套一键优化工具。`/root/global_conf.sh` 是凭据配置文件，不是系统加固程序，本文不记录其内容。

| 项目 | 旧机 | 新机及本次迁移状态 |
| --- | --- | --- |
| 软件安装和更新 | 原运行环境 | 安装 Docker、Compose、Nginx、证书及迁移工具；安装依赖时更新了 OpenSSL、curl、SQLite 等组件，没有全系统升级或更换内核记录 |
| 自动安全更新 | 未配置 unattended-upgrades | 同样未配置；apt 定时器存在，不等于自动安装安全补丁已启用 |
| UFW | 启用 | 已启用默认拒绝入站，放行 SSH 和业务端口；8080 仅允许旧机来源。判断依据是 `ufw status` 的实际规则，不能仅看 oneshot systemd 服务的 active 状态 |
| Fail2ban | sshd，5 次失败、10 分钟窗口、封禁 1 小时 | sshd 已运行且已有封禁，但使用 Debian 默认 nftables 动作和 10 分钟封禁，没有沿用旧自定义 jail |
| SSH | 22748，禁止密码登录，root 仅密钥，关闭 X11 | 22，root 密码登录仍允许且 root 密码已设置，X11 仍允许；这是加固缺口，保留 22 端口本身不是缺口 |
| 网络拥塞控制 | BBR + CAKE | CUBIC + fq_codel；未沿用旧机 TCP 缓冲及 BBR 配置 |
| IPv6/sysctl | 含旧网卡名称和旧 sysctl 文件 | 保留供应商配置，没有覆盖旧网卡相关参数；不能原样套用旧硬件配置 |
| 证书和备份 | 旧机相关任务停用 | 新机 certbot.timer 和每日 02:00 备份任务启用 |

SSH 修复应保留已验证的 22 端口和管理公钥，先验证独立新连接及回退途径，再校验并重载 SSH 配置。不能直接复制旧机 22748 配置覆盖新机。BBR 是网络拥塞控制选择，不是 PostgreSQL 共享内存问题的修复。

## 新机并发配置建议

以下是分阶段候选值，不是立即执行的变更清单，也不是未经压测的性能承诺。

| 配置 | 当前实际值 | 判断与候选方向 |
| --- | --- | --- |
| PostgreSQL `shm_size` | 64 MiB | 优先修正为 2 GiB |
| `GOMAXPROCS` | 12 | 沿用旧 CPU 参数；当前没有 CPU 饱和证据。扩容阶段可先试 32，而不是直接设 256 |
| `GOMEMLIMIT` | 10GiB | 当前没有触及限制的证据。扩容阶段可评估 24GiB；它是 Go 软内存限制，不是提前预留内存 |
| `POSTGRES_SHARED_BUFFERS` | 4GB | 对 40GB 数据库和充裕内存较保守；后续可从 16GB 起试，评估 WAL/checkpoint 行为 |
| `POSTGRES_EFFECTIVE_CACHE_SIZE` | 16GB | 后续可评估 128GB；只是查询规划器的缓存容量估值，不会实际分配该大小内存 |
| 应用数据库连接池 | 最大 80，空闲 32 | 暂时保持；样本未显示池耗尽，不能跟随 256 逻辑 CPU 同比放大 |
| PostgreSQL 最大连接 | 100 | 暂时保持，保留管理及后台连接余量 |
| 并行 worker / 每查询 | 4 / 2 | 先保持，修复 shm 后按报表 SQL 测量；需要提高总 worker 时还要核对 `max_worker_processes` |
| `work_mem` / 维护内存 | 4MB / 128MB | 不按主机内存同比扩张；按具体 SQL 评估，注意并发会话、执行节点和并行 worker 的乘数效应 |
| 用量 worker / 队列 | 64 / 16384 | 保持，自动扩容关闭，继续按用户串行扣费 |
| 用量任务 / 响应头超时 | 5 秒 / 3600 秒 | 保持，不用延长计费超时掩盖连接等待 |
| HTTP 上游连接池 | 单 host 最大 2048，总空闲 2048，每 host 空闲 256 | 当前没有耗尽证据，暂时保持 |

`GOMAXPROCS=12` 限制同时执行 Go 代码的并行度，不代表最多处理 12 个 HTTP 请求；大量网络等待中的流式连接可以同时存在。硬件更多并不意味着扩大所有队列和连接池就会改善吞吐。

## 已发现的额外配置问题

### Nginx 主进程文件描述符

- 配置为 `worker_processes auto`、`worker_connections 4096`、`worker_rlimit_nofile 65535`。
- 实际 worker 数为 244；master 的 `RLIMIT_NOFILE` 为软 1024、硬 524288，worker 是 65535/65535。
- 新机 03:21:23 重载出现 `socketpair() failed while spawning "worker process" (24: Too many open files)`；当前日志共找到 24 条同类文件数错误。
- `worker_rlimit_nofile` 只作用于 worker，并未提高 master 的软上限。建议同时修正 master 实际限制与 systemd 持久化设置，并在校验后验证重载。是否将 worker 固定为 16 至 32，应另依据真实吞吐和连接数测试，不能直接认定 256 worker 最优。
- 本轮主机 `ListenOverflows`、`ListenDrops`、TCP 内存压力计数均为 0，但不能因此忽略已经发生的重载错误。

### Redis 内存策略

`vm.overcommit_memory=0`，启动日志已警告后台保存或复制可能失败。建议评估并持久化为 1；这是 Redis 官方部署建议，不需要为此升级内核。旧机同样为 0，迁移未修复这一既有问题。

Redis 本轮占用约 38 MiB，约 343 个客户端，`maxclients=50000`、阻塞客户端 0、拒绝连接 0，没有客户端容量饱和证据。`maxmemory=0` 是无限额，不应在没有容量规划时随意添加可能导致会话写入失败或被淘汰的限制。

## 建议实施顺序与验证

1. 优先补齐 root 仅密钥认证与 Nginx 主进程文件上限；保留管理连接、配置备份与回退路径。
2. 安排一次短维护修正 PostgreSQL `shm_size`，维持镜像、三个 bind mount、双 Compose 和单写拓扑，核验实际 `/dev/shm` 而不是只看文件。
3. 补齐 Redis 内核参数及安全更新策略。不要直接运行第三方一键内核优化脚本。
4. 独立进行 Go 并行度、数据库缓存的分阶段扩容，使用实际峰值期间的 CPU、GC、RSS、连接池等待、SQL 耗时、临时文件、checkpoint、共享内存峰值和计费完成率作判断。
5. 不启动旧库，不通过旧机回退写入，不在同一次操作中叠加镜像升级、全量系统升级和不相关业务修改。

## 核验依据

现场证据来自 `docker inspect/stats/logs`、`pg_settings/pg_file_settings/pg_stat_*`、应用运维监控表、`/proc/*/limits`、`sshd -T`、`ufw status`、Fail2ban、APT 历史与备份脚本文本。没有导出用户数据、数据库密码、SSH 私钥或 API 令牌。

官方资料用于解释配置语义，不用于证明本机状态：

- PostgreSQL 18 Resource Consumption、Managing Kernel Resources、Query Planning。
- Docker Compose services 的 `shm_size`，以及 `compose up` 与 `compose restart` 行为。
- Nginx Core functionality 中的 `worker_rlimit_nofile`。
- Go runtime 的 `GOMAXPROCS`、`GOMEMLIMIT`。
- Redis administration 的 Linux overcommit 建议。
