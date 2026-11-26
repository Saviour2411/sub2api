# 2026-11-26 Passkey 配置蓝绿切换

用户授权直接更新服务器1配置，使用现有镜像蓝绿切换，保留正在处理的业务；本次不经过 GitHub Actions 发布，完成后提交并推送配置记录。

## 生效配置

服务器文件为 `/root/proj/sub2api/deploy/data/config.yaml`，挂载到容器的 `/app/data/config.yaml`。只在原文件顶层新增 WebAuthn 段，完整的非敏感配置片段见 [webauthn.yaml](../../deploy/operations/production-passkey-20261126/webauthn.yaml)。保留其余生产配置和文件权限、所有者。

```yaml
webauthn:
  enabled: true
  rp_display_name: "ZeroAPI"
  rp_id: "saviour.cc.cd"
  rp_origins:
    - "https://direct.saviour.cc.cd"
    - "https://api.saviour.cc.cd"
```

同时将数据库 `settings.passkey_enabled` 从 `false` 改为 `true`。这是全站开关；用户需在自己的设备上进入「个人资料 → Passkey → 添加 Passkey」，输入账户密码并完成设备验证。

RP ID 使用两个入口的共同父域。同一 Passkey 可在列出的两个 HTTPS 来源使用；父域无需另外提供网站。上线前生产库中 Passkey 凭据数为 0。以后已有用户绑定时，不能随意改动 RP ID，否则旧凭据不能用于新的 RP ID。

## 镜像与切换

- 应用版本：`0.1.272`，本次没有构建或升级应用。
- 应用提交：`3d5d7724c42bbfd712220b901205f7ce175fdc3d`。
- 镜像：`saviour2411/sub2api@sha256:198b5639e21d1d48e3205d1a05eed78f562380095c418ddb02c4508ccd6a3a3d`。
- green 原实例：`i782a58ac4dd142eda16e919c1576e09d`。
- blue 新实例：`if7cd880840f8470ea78de0986e845e90`。
- 切流时间：北京时间 2026-11-26 21:03:34；双入口切流确认耗时 1.197 秒。

现有发布器在版本和镜像相同时会直接返回，因此本次用一次性操作脚本复用 `deploy/blue-green/deploy.py` 的预检、候选渲染、健康检查、原子切流和排空方法。没有改动常规发布器、伪造新提交或发起 GitHub 工作流。复用已部署固定镜像的既有验收记录，迁移基线为该镜像自身，迁移清单为空。

候选复用了现有 Compose 环境、共享数据库和 Redis、JWT/TOTP 密钥及私有通信目录。启动前比较部署环境与运行中 green 的环境，除槽位、日志和迁移控制字段外无差异。双实例参数上限仍沿用原设置；发布器检查了数据库实际连接和可用内存，资源余量通过检查。

配置和候选验证完成前，green 持续接收原有流量。blue 先以 standby 启动，依赖检查及合成流通过后激活，验证其真实接入端口与 Passkey，再通过共享 Nginx upstream 切换两个入口。

## 验证结果与边界

- blue 的依赖检查、合成流、接入端口检查通过。
- 两个来源的 `/api/v1/settings/public` 均返回 HTTP 200，`passkey_enabled=true`。
- 两个来源的 `/api/v1/auth/passkey/login/begin` 均返回 HTTP 200，登录挑战的 `rpId=saviour.cc.cd`、`userVerification=required`。
- 切换期间两个入口各 33 次持续健康探针均成功，未观察到探针错误。
- 切流后从服务器通过两个公网域名读取公开设置，均返回 HTTP 200 和启用状态。
- 21:04:59 复核时，blue/green 均 healthy、重启次数均为 0、镜像内容相同。blue 为 active；green 为 draining，仍有 40 个 HTTP 工作、28 个 SSE 工作和 7,132 个会话归属，因此继续保留。
- 探针没有调用付费模型，也没有代替用户绑定认证器。实际设备上的 Passkey 注册和完整登录仍由用户完成；健康探针成功不等同于逐条证明所有客户请求无错误。

## 自然排空与回滚

本次禁止强制停止旧实例、强杀旧 Nginx worker、缩短会话归属 TTL 或清理仍在使用的归属数据。切流完成时部署状态为 `drain_pending`，不能把它记作已完成退役。

已在服务器启动现有发布器的自然排空收尾进程：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 \
  /root/proj/sub2api/deploy/operations/passkey-20261126/deploy.py \
  --directory /root/proj/sub2api/deploy \
  --retire-pending --window 86400
```

没有传入 `--force-after-window`。只有旧实例工作及会话归零、旧 Nginx worker 全部退出，且应用写出退役凭据后，执行器才停止并移除 green。24 小时观察期结束时如果仍不满足条件，会保留 green，不强制回收。该进程持有部署互斥锁；旧槽退役前不能复用。

服务器证据和备份保存在 `/root/proj/sub2api/deploy/operations/passkey-20261126/`：

- `config.before.yaml`、`state.before.json`、`upstream.before.conf`、`setting.before.json`：变更前备份，仅在服务器保留，含敏感配置的文件权限为 0600，不提交到仓库。
- `result.json`、`observation.json`：本次切换结果和持续健康检查。
- `retire-process.json`、`retire.log`：自然排空进程身份和日志。
- `release.json`、`deploy.py`、`passkey-config-rollout-20261126.py`：固定镜像证据和本次实际执行脚本。

如果需要回滚，先核对 `retire-process.json` 中 PID 与实际命令，停止收尾进程以释放发布锁，再确认 green 容器仍存在且尚未封闭。将 Passkey 开关恢复为 `false`，按备份恢复配置内容及原文件权限，然后使用同一发布器的 `--rollback` 回切 green；候选 blue 同样只能自然排空。不得用重建容器或重置数据库替代回切。

配置变更前 SHA-256：`8f01b13b6a126e82f7a6e3d6c9eab9fa3ebe8fcaca23a01b7de49783fdf41ed4`。

配置变更后 SHA-256：`8c9bf135cfaab55642d41ca81eecfdef49357e644f2ae4b22351cd7e7b13db0d`。
