# 生产配置归档

对应 `216.152.153.86` 于北京时间2026-09-14实施的资源与系统配置调整。执行记录见 `docs/operations/2026-09-14-production-resource-update.md`。

| 文件 | 实际部署位置或用途 |
| --- | --- |
| `ssh.conf` | `/etc/ssh/sshd_config.d/00-sub2api-production.conf` |
| `fail2ban.local` | `/etc/fail2ban/jail.d/99-sub2api-production.local` |
| `nginx-limits.conf` | `/etc/systemd/system/nginx.service.d/limits.conf` |
| `redis-sysctl.conf` | `/etc/sysctl.d/99-sub2api-redis.conf` |
| `resources.env.example` | 实例资源键的脱敏快照，不能替换完整生产 `.env` |
| `vps-backup-coverage.patch` | `/root/vps_backup.sh` 新增配置覆盖项 |

这是归档，不提供自动覆盖或重启命令。恢复前必须核对硬件、现有配置、访问路径与生产挂载，保留管理连接，并单独评估应用排空和计费清理。SSH密码登录按用户明确要求保留，不代表通用安全默认值。

Go 80%软内存上限与 PostgreSQL 25%缓存预算合计超过物理内存，极端负载仍有 OOM 风险。这里没有包含应用密钥、数据库密码、Cloudflare/GitHub令牌或SSH私钥。
