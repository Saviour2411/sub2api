# 正确排空、多实例与蓝绿发布（生产迁移待门禁）

> 2026-09-15，功能分支 `codex/graceful-blue-green-deploy`，已合入主线基线 `09523d260`。
> 用户追加授权在 CI 通过后通过 tag/Actions 部署；授权不等于绕过首次迁移、资源预算或旧连接排空门禁。
> **尚未生产部署，也未创建发布标签。** 本机 SSH 多次在 banner 握手阶段超时；改用只读 Actions 后，在 2026-09-15 21:28:03 UTC 重新读取了 server1 基线。确认首次迁移状态缺失且现有资源参数不能直接双开。本地/CI 隔离测试数据不是生产升级耗时。

## 已落地的实现

### 生命周期

- `starting → standby → active → draining → drained`；只有 `active` 且数据库、Redis、实例租约正常时 `/readyz` 返回 200。`/health` 保持原行为。
- `lifecycle.mode=standby` 的业务入口不接单；激活、排空、退役只通过 `lifecycle.socket` 指定的 Unix socket。
- 管理操作为 `GET /state`、`GET /check`、`GET /synthetic`、`POST /activate`、`POST /drain`、`POST /retire`。
- socket 必须为绝对路径，权限 0600；进程不覆盖另一个进程仍占用的 socket。
- 存活 WS 在连接生命周期内续约其会话归属，防止长轮次超过缓存 TTL 后续接漂移；不改变原有 WS 空闲/读取超时，关闭后按既有 TTL 自然过期。
- 状态分别统计 HTTP、SSE、升级后的 WS 连接、WS 轮次、断连后的上游工作、排队/执行中用量、用量生产者、后台任务与会话归属租约。
- `draining` 不取消现有业务，也不拒绝代理仍送来的残余业务；只有原子封闭接入后才拒绝新工作。排空暂停新共享任务，不销毁调度器；`/activate` 可在未封闭接入前恢复 `active`，用于回滚。
- `/retire` 必须看到全部工作和租约归零；封闭入口与请求登记共用互斥锁。先写入 `<socket>.retired` 凭据，再通知主程序退出，避免控制响应丢失使执行器无法判断是否已退役。
- HTTP Shutdown 不设置发布强退时间，另外等待 hijack 的 WS 和脱离客户端的工作。

### 用量与依赖

- `SubmitKeyed` 的停止分支统一返回 `dropped_stopped`，与显式队列溢出 `dropped` 区分。
- 已入队任务、执行中任务和同步兜底纳入生命周期。正常请求超时与模型计费规则未改。
- 清理先暂停生产者并等待请求/会话，再停用量池，最后关闭连接池及数据库/Redis。
- 定向测试覆盖停止竞争与恰好执行一次；没有通过补造 `message_stop` 或重放生成请求实现“成功”。

### 多实例与内部续接

- 实例使用唯一 ID；Redis 默认 20 秒续约、60 秒存活租约，排空时仍续约。
- 启动不再按“非本进程前缀”删除槽位或等待数；旧字符串等待计数只读兼容，新等待计数按 owner 分字段聚合。
- 本进程只刷新仍持有的槽位及等待字段；槽位释放后不会被续约复活。
- 已改造的共享任务统一经过 PostgreSQL advisory lock，不在 Redis 和 PostgreSQL 两个锁域间故障切换；未确认解锁的数据库连接丢弃，不回池。
- 备份和恢复记录包含可选 owner；未知旧记录、活跃同伴以及租约查询失败都不据此判死。
- OpenAI Responses、Messages、Chat Completions 及 WS 首帧加入归属路由；响应归属与会话 TTL 阻止旧实例提前退役；可恢复的 turn state 存入 Redis，实际连接仍在原实例。
- 仅可转发到 Redis 中登记、租约有效、位于共享私有目录的 `blue.sock`/`green.sock`，禁止任意 TCP 地址、Docker socket 和转发环路。
- 私有通道签名绑定 URI、用户、API Key、分组和原始网络身份；接收方先恢复经签名验证的网络上下文，再执行原有鉴权和 IP 白名单检查，处理前再次核对归属。
- WS 公网入口的连接接入租约仅领取一次；拥有者负责每轮账号/用户槽位、调度及用量，中转侧不执行这些操作。

## 蓝绿执行器的当前范围

`deploy/blue-green/deploy.py` 实现以下状态机与安全检查；生产首次迁移仍需另行通过门禁：

1. 两槽为 blue=18080、green=18082，不占用 18081，不创建第二套数据库。
2. 必须预先存在经过首次迁移审定的 `blue-green/config.json` 与 `state.json`；缺失即拒绝，不接管旧单容器。
3. 核验主机 machine-id、Compose 基线、现有数据 bind mount、共享 JWT/TOTP 密钥、固定镜像摘要/revision、迁移兼容性记录、双实例连接池及内存预算。
4. 候选继承已解析的应用 Compose 环境/数据挂载，共用已有外部网络；日志、socket 独立。
5. 候选 standby → 依赖和无上游调用的合成流检查 → 激活 → 同时供 API/direct 引用的 upstream 原子替换 → `nginx -t` → reload → 新连接实例 ID 核验 → 旧实例排空。
6. 状态文件 fsync 后原子替换；服务器 flock 互斥；Actions 已增加生产部署互斥且不取消进行中的发布。
7. 默认观察 900 秒；超时保存 `drain_pending`、保留旧槽；不同的新发布不得覆盖旧槽。相同发布重跑继续观察。
8. 停容器前还需确认旧 Nginx worker 已退出，并校验实例/容器身份与退役凭据；`docker stop --timeout -1` 不设置强杀倒计时。
9. 候选采用 `on-failure`，避免正常退役退出后被 `unless-stopped` 自动拉起。
10. `--rollback` 仅回到身份匹配、尚未封闭的保留实例，并要求精确版本的数据库回滚兼容记录；回切后排空新实例。回滚中断可用同命令恢复。
11. Release 已改接该执行器，不再调用 `remote-deploy.sh`。上传采用发布唯一目录；不覆盖活动 Compose，不将缺少部署密钥报告为部署成功。
12. `evidence.py` 通过 GitHub Actions API 验证固定 SHA 的 CI/Security Scan 以及未跳过的必要 job，包括真实协议切流 job；镜像固定 digest，运行时再次校验 OCI revision。自动路径只接受 `backend/migrations` 与 `backend/ent/schema` 无变化，出现变化就拒绝，不能把迁移串行锁当作兼容证明。
13. 备份记录数组的读改写使用 PG advisory lock；存储读取失败或历史记录损坏时拒绝覆盖。请求触发的异步备份/恢复与仪表盘回填也计入生命周期。

## 生产仍需满足的门禁和已知边界

- server1 的现有容器、版本、挂载和容量已由本轮只读 Actions 重新查询；Nginx 完整配置、实际生效预算及首次迁移方案仍需单独审定，不能把诊断成功当作发布批准。
- `blue-green/config.json` 和 `state.json` 只能由审定后的首次迁移创建；执行器不会自动把 `v0.1.234` 标为 drained，也不会停止没有退役凭据的旧容器。
- 隔离协议测试使用真实当前应用进程、Nginx、PostgreSQL、Redis 和本地模拟上游；覆盖实际 Responses SSE、WS 跨入口续接、HTTP/2、普通请求、客户端断连与用量表逐笔核对。不是生产数据，也不是所有模型/媒体/实时音视频协议的完整证明。
- 协议测试中的 TTL 在测试库初始化时固定为 5 秒；只为有限时间测试会话到期，不修改生产 TTL、不在排空时缩短有效期。旧实例仍有 WS/会话/生产者时不可退役。
- 执行器的故障/重跑/回滚是独立状态机测试；隔离协议测试不伪装成通过生产主机、Compose 指纹和实际容量检查的远端发布。
- 未完成真实生产双实例容量预检、旧版首次迁移和生产 tag/Actions 验证前，不宣告生产零错误升级验收。

## 已执行的测试类别

- 生命周期、管理接口、SSE 自然终止、handler 返回后仍存活的 WS、断连后用量工作、内部身份/目标/环路约束的定向 race 测试。
- 普通/按用户用量提交与停止竞争，任务执行次数逐条断言。
- 真实 Redis 的并发槽/等待数兼容、重启保护与本实例续约测试（现有 repository integration harness）。
- Python 发布状态机单元测试，包括 20 轮**模拟**状态切换、候选失败、保留旧槽、拒绝复用、退役凭据不符、中断后恢复、幂等重跑，以及旧实例已 draining 时的回滚；证据测试拒绝错误 SHA、跳过 job 和未知迁移。
- `deploy/tests/blue-green-integration.py --cycles 20` 使用实际应用和隔离 Docker 依赖；Linux CI 的 `bluegreen-protocol` 为必要门禁，不使用生产密钥或付费上游。测试结果由 Actions artifact 保存，新连接切流与旧实例排空分别记录。
- 全量 Go unit：最终本地增量通过；全量 integration：本轮通过（包含真实 Redis/PostgreSQL repository harness）。
- golangci-lint v2.13.0：0 issues；当前服务二进制构建成功；Wire 再生成无差异。
- 既有 macOS 部署脚本安全门禁保留，新增状态机、证据和 Linux 真实协议门禁。
- 合并 `09523d260` 后已重跑全量 Go unit/integration、定向 race 和 lint；任何后续补丁以固定提交的 CI/Security Scan 结果为准。
- 本地测试结果不等于固定提交的远端 CI/Security Scan。测试原始日志留在项目内 `.analysis_tmp/bluegreen-*.log`，不是生产测量报告。

### CI 迭代与只读生产预检

- `9165ee1b9` 的功能分支 CI/Security Scan 已全部通过，并已快进到远端 main；生产仍未部署。
- 首轮 CI 的 WS 测试曾在客户端握手返回后、服务端 Hijack 完成前读取计数。测试改为明确等待整个中间件返回，再断言 HTTP 为零、WS 为一并阻止退役；保留活连接收发与自然关闭断言，不增加固定 sleep。修复后本地 1,000 次定向 race 与生命周期包 10 轮 race 均通过。
- 手动工作流 `Blue-green production preflight (read-only)` 使用同一生产部署互斥与既有 SSH 凭据，从 GitHub runner 对固定 server1 执行 `preflight.py`。只查询容器/容量/审批文件是否存在，不写远端文件、不输出完整环境或密钥、不改资源、不切流、不停容器。
- 预检成功仅表示读到了白名单诊断，不会创建 `config.json/state.json` 或批准首次迁移；环境预算缺失时标为未知，不猜测默认值。

## 首次从 v0.1.234 迁移：先通过独立门禁

旧版没有统一生命周期、会话归属和退役凭据，任何工具都不能以 `/health=200` 或等待时间推断它已排空。

另行审批前应只读确认：

- 固定主机、现有 API/direct Nginx 文件、证书、数据挂载、网络与端口占用。
- 两个入口改用同一个 `sub2api_active` upstream 的审定方案，且不存在强制结束旧 worker 的配置。
- 活动镜像摘要/revision、当前数据库迁移集和候选变更的向后兼容结论；迁移锁不等于兼容证明。
- 双实例应用连接池总和加维护预留不超过实际 `max_connections`；双实例内存加数据库/Redis/代理/系统预留不超过整机容量。
- JWT/TOTP、备份加密密钥和持久配置一致；不自动改写现有生产值。
- 如何处理不具备归属能力的旧版长连接及未落库用量。不得自动停止旧版，不承诺首次迁移无中断。
- 首次转换失败的人工回退条件、负责人和观测窗口。

用户已授权发布，但上述门禁尚未通过。不得通过创建 tag 绕过无法验证的首次迁移，也不得重算历史账单。

## 运维命令与恢复原则

```sh
# 固定发布输入由 Release evidence.py 生成，不能手写“CI success”替代验证。
python3 deploy/blue-green/deploy.py --directory "$DEPLOY_DIR" --release release.json --window 900
# 仅在保留实例尚未退役、且该次发布具备精确回滚兼容记录时可用。
python3 deploy/blue-green/deploy.py --directory "$DEPLOY_DIR" --rollback --window 900
```

观察超时为 `drain_pending`，不是退役成功。重跑同一输入继续观察；不同的新发布不得覆盖仍在服务的保留槽。恢复时保留 `state.json`、镜像摘要、实例/容器身份、退役凭据和上次代理配置；不要删除状态后“从头部署”。

第一次迁移需要额外解决两件事：旧版无法证明排空；本轮只读 Actions 再次确认单应用池上限 512 / 数据库容量 600，以及单应用约 80% 整机内存软上限，均不支持原值直接双开。资源变更不得由部署脚本自行执行。

## 2026-09-15 本轮验收记录（非生产升级验收）

应用/预检代码候选为 `fb885cd1d72a7a21544095c58b4c8699e5c0c0e2`，已快进到远端 main：

- CI run `35024366357`：所有 7 个必要 job 成功，包括全量 unit/integration、两类 race、macOS 脚本门禁和 Linux 真实协议门禁。
- Security Scan run `35024366346`：成功。
- Linux 实际应用双入口 20 轮：切流最小 0.055844 秒、中位数 0.123219 秒、最大 0.187618 秒；旧实例排空为 5.139413–8.366780 秒。真实 SSE/WS/断连续收用量与逐笔用量断言通过。
- 以上排空时间使用测试初始化时固定的 5 秒会话 TTL，不是生产排空 SLA，不会为生产部署缩短会话有效期。
- 本机 macOS Docker Desktop 的真实协议测试曾出现 VM 到宿主机链路连接超时；没有把失败包装为通过。Linux CI 在固定候选上通过；生产仍需独立观测。

只读生产预检 run `35025763779` 成功，在 `2026-09-15T21:28:03Z` 采集：

| 项目 | 实际结果 | 发布含义 |
| --- | --- | --- |
| 运行应用 | 单容器 `sub2api`，revision `cfd9afa871def9ebd457bb95fb4c2e8b4b02b34f`，端口 18080 | 仍为 v0.1.234，不具备新生命周期/会话归属能力 |
| 蓝绿状态 | `config.json`、`state.json` 均不存在 | 不能直接调用常规发布执行器或假造审批状态 |
| 数据库 | 应用环境池上限 512，PostgreSQL `max_connections=600` | 两个同配置实例需要 1,024 个连接，尚未计维护预留，拒绝双开 |
| 内存 | 整机 270,226,366,464 字节；单应用 `GOMEMLIMIT=216181080064B` | 每实例约 80%，复制双开超过整机容量；需另行审定预算 |
| 持久目录 | 应用 data、postgres_data、redis_data 的既有 bind mount 均读到 | 仅核对，不调整挂载或数据 |

**预检只读成功不等于部署成功。** 未创建/移动 tag，未启动候选、切换代理、停止容器或调整生产参数。后续先审定旧版连接与用量处理、双实例实际生效预算和首次迁移/回退步骤，再安排首次生产迁移及后续常规蓝绿 tag 验证。

## 2026-09-16 修正：保持上限，首次初始化由 tag/Actions 执行

本节覆盖前文原有“上限相加即拒绝、必须预先人工创建审批文件”的旧行为：

- 保持生产连接池及 GOMEMLIMIT。两个实例配置上限相加超过整机容量仅告警；硬检查改为 PostgreSQL 实际客户端连接余量（扣除保留连接后至少 32）和 MemAvailable（至少 1 GiB），在候选启动前及激活前检查。当前余量不保证未来峰值，不宣称任意负载下零错误。
- Release 在服务器上只读发现现有状态；缺失时识别固定 revision 的 v0.1.234 单容器，先生成与实际旧 revision 绑定的无迁移变化证据，再由服务器发布互斥保护下的初始化流程生成 config/state。不会手写 CI 通过或假造 drained。
- 初始化保存 bootstrap journal 和原代理内容；两入口只替换已识别的应用 upstream，保留证书、超时、监听端口和其他站点。先 reload 到仍指向旧实例的统一 upstream，再启动候选、就绪验证、切流。初始化中断可从 journal 重跑，未知配置/容器身份拒绝覆盖。
- 首次旧会话通过父目录 0700 的固定 legacy Unix socket 回旧版，再由旧版重新鉴权和计费；不接受客户端指定目标、不缓存完整流、不重试生成请求。WS 转交前释放中转侧入口租约，避免旧版不识别新私有协议而重复占位。
- 首次共存期间新实例暂停共享后台任务，继续由旧版负责，避免 Redis/PG 两种历史互斥方式并行执行；请求和每实例本地服务仍运行。
- 首次切流后保留旧容器及合法续接，记录 `drain_pending / legacy_unverifiable`，不自动杀死没有生命周期凭据的旧版。后续若该旧槽仍保留则安全拒绝复用；不能把首次切流完成误报成旧版退役完成。
- GitHub macOS runner 仅执行脚本测试；实际发布仍是 GitHub Ubuntu runner SSH 到固定 server1，不依赖本机在线。Release 采集切流耗时和双入口无付费健康探针错误，作为观测证据而非所有客户请求零错误的证明。
- CI 除原 20 轮协议切换外，增加构建实际 v0.1.234 二进制的首次续接测试，覆盖旧长 SSE、旧 WS、新入口同会话 WS、入口限额不重复、旧 response 续接及逐笔用量。

以上为本轮候选实现说明，具体 tag/生产结果以本轮执行后的交付证据为准。
