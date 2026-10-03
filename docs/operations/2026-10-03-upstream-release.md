# 2026-10-03 上游同步与v0.1.249发布

结果：v0.1.249已由Actions完成蓝绿部署，最终blue稳定、旧green已回收、241保护已解除。本轮发生超时强退，用量完整性未知，不能宣称无损发布。

## 范围

- 用户授权提交代码，经CI通过后合入远端主线，再创建新tag并由GitHub Actions自动蓝绿部署。本轮版本为v0.1.249，实际标签、运行和验收证据在后续章节记录。
- 上游固定基线为`b8dece9000c68815a5b867ca5a1e6f236e173905`（v0.2.13），本地同步合并为`3a1f3127484f8b4e959ffdf00696e8bdf3d53139`，PR #26的候选分支为`sync/upstream-20261002-b8dece900`。
- 保留merge祖先关系、现有CI与安全扫描、生产数据挂载、Compose、资源、3600秒响应头等待及5秒usage任务。不得把发布授权解释为允许绕过迁移门禁。

## 首轮门禁与修复

- push CI `37081979583`：shell、frontend、golangci-lint、bluegreen-protocol、release-helpers通过；test、stream-race、lifecycle-race因`unit`标签测试的旧接口调用失败。默认`go test ./...`未包含这些测试，原本地成功不等于完整CI成功。
- 修正支付订单快照测试的重复赠金参数；上游在途预留测试改为校验保守预留估算与本地请求模型准入边界，保留无价别名必须拒绝的断言，不恢复映射型号计费。
- push Security Scan `37081979653`：后端通过，Canvas审计失败。brace-expansion的`GHSA-qhr7-859c-m2p7`和`GHSA-6j4f-fj2g-mc7p`于UTC 2026-09-29发布，axios相关高危公告于UTC 2026-09-30发布；按公告修复版本升级Canvas至brace-expansion 5.0.11、axios 1.20.0。
- 本地冻结安装、Canvas 34项测试、类型检查、生产构建通过。修复后审计为0项high/critical、12项moderate、1项low；原始审计退出1，现有豁免门禁退出0，未增加或延长豁免。
- 以上记录不预写后续CI、主线、tag或部署成功结论。
- 修复后的全量unit测试进一步发现`gpt-6.1-sol`价格表与本地计费名称规范化衔接遗漏，已补齐严格匹配和正反例；其余本机Unix socket/文件权限失败来自WSL的Windows挂载目录，在工作区内tmpfs复验，不修改业务代码或弱化断言来掩盖环境失败。
- 候选`87886068b`的PR CI `37083191255`已确认仅有上述两个定价用例失败，其余7个CI作业成功；PR Security Scan `37083191259`成功。失败日志通过完整运行归档获取，未关闭TLS证书校验。
- 名称规范化修复后，在工作区内tmpfs执行`go test -tags=unit ./internal/service ./internal/pkg/openai -count=1 -timeout=10m -run 'TestNormalizeKnownOpenAIPricingModel|TestNewModelPricing|TestBillingInflight|TestGPT61Sol'`退出0，上游同步元数据校验退出0。全量`go test -tags=unit ./... -count=1 -timeout=20m`及新提交的远端门禁结果仍待完成，不能沿用旧提交的成功检查。

## 后续门禁与静态样式依赖

- 规范化修复提交`7887a6c4d01d3cbe1e4aaab38a2bfeff8e99ae91`的push CI `37087589670`及PR CI `37087594263`各8项作业全部成功，包含单元、集成、竞态与隔离蓝绿协议验证。本地工作区tmpfs全量unit退出0，临时挂载已正常卸载。
- 同提交的push Security Scan `37087589728`及PR Security Scan `37087594216`仍失败，原因是Canvas的braces 3.0.3：`GHSA-vfj7-8cjw-p6xm`。公告API显示UTC 2026-09-18发布、UTC 2026-10-02 22:36:34更新，受影响范围为`<=3.0.3`，未列出修复版本；npm当前发布版本也仍为3.0.3，不能编造升级版本或沿用此前审计通过的结论。
- `pnpm why braces --prod`显示全部路径来自未调用的shadcn CLI，Canvas仅导入`shadcn/tailwind.css`。固定保留4.18.0的静态样式及MIT许可，将导入改为本地文件，再移除CLI及其依赖链；没有移动到开发依赖、覆盖不兼容实现、修改安全检查或新增豁免。
- 修复后冻结安装退出0；8个Vitest文件34项测试、类型检查和Vite构建退出0。修改前后11个构建文件的SHA-256逐一一致，包含HTML、JS、CSS及图标；保留既有大包与动态导入构建警告，未顺便重构。
- 生产依赖审计剩1项moderate、1项low、0项high/critical。原始pnpm审计退出1，既有`check_pnpm_audit_exceptions.py`退出0；完整结果见`canvas-audit-css-vendor.json`和`canvas-css-vendor-artifact-comparison.json`。锁文件不再包含braces、micromatch或fast-glob，新提交仍待远端完整门禁。

## 生产只读核验

- 北京时间2026-10-03 08:24核验，生产状态`stable`、活动槽`green`、`pending=null`，版本仍为0.1.248、固定SHA为`211d3f29e9a8652e7f7323bda6bcada8ee7382c3`。历史强退记录属于上一轮发布，不是本轮结果。
- 08:38对PostgreSQL 18.4进行只读结构查询。`payment_orders.bonus_amount`已为`numeric(20,2) NOT NULL DEFAULT 0`，由既有149迁移建立；新增241赠金迁移仍未登记，不能因字段存在就跳过迁移账本校验。
- TypeSafe迁移将重建`user_platform_quotas`及`composite_model_routes`平台CHECK约束以增加`typesafe`。两表当前均为无继承普通表，含索引总大小分别393216和40960字节；这只是采样，不代替执行时锁内检查及旧版读写兼容性证明。
- 本地重放当前`migration_evidence`明确拒绝本轮Ent schema与SQL变化。现行审批仅覆盖普通可空列及已批准的239/240专项，不能直接套用到本轮241约束迁移。
- 尚未修改兼容审批或执行器、未执行生产DDL、未创建tag、未触发Release。需要用户确认专项兼容适配范围后继续，不通过人工预执行SQL、隐藏差异或改写历史迁移绕过门禁。

## 241 专项审批与实现

- PR #26已于北京时间2026-10-03 19:58:47合入主线`f826de06592046c3c17b09e0c45dab651143430d`；候选`411c5bdaf`的push/PR各8项CI与2项安全检查全部成功，主线CI `37121379877`及安全扫描`37121379937`也全部成功。
- 用户随后明确批准两条241迁移的专项兼容适配、验证后发新tag及Actions自动蓝绿部署。专项分支为`codex/bluegreen-241-v01249`，不放宽通用可空字段扩展规则，不改写原SQL。
- 北京时间22:30只读复核活动基线仍为`211d3f29e9a8652e7f7323bda6bcada8ee7382c3`（v0.1.248），PostgreSQL18.4；三张相关表均无继承、自定义触发器和RLS，大小与早前记录一致。仅采集结构及迁移账本，不采集业务内容。
- `bonus-existing-241-v1`固定赠金SQL摘要；执行前要求149账本准确、既有列为numeric(20,2)、非空且默认0，原SQL只执行IF NOT EXISTS空操作和登记账本，禁止悄悄补列、回填或修正漂移。
- `platform-guarded-241-v1`固定平台SQL及Ent文件修改前后摘要，精确绑定当前旧版本的读写契约。锁内核验238账本及两表原CHECK定义，原SQL扩展后在同一事务安装`sub2api_bg241_quota`和`sub2api_bg241_route`，新旧实例共存时禁止向这两表写入typesafe。
- 专项限PostgreSQL18、无自定义触发器/RLS/继承且至多16MiB的普通表；所有表锁使用NOWAIT，锁等待/单条语句/事务预算为1/5/10秒。冲突立即失败并回滚，不自动重试候选启动。
- 切流或回滚前复核保护、完整账本及实际结构。只有旧槽已退役、无其他旧应用容器、机器和活动容器身份、Compose、数据挂载及双入口健康均符合时，才在短事务中解除241保护；回滚或失败保留保护，解除锁冲突可恢复重试。其他平台约束不删除，普通迁移规则不放宽。
- 本地最终复验：`go test -tags=unit ./internal/repository -run 'TestBlueGreen' -count=1 -timeout=10m`及`go test -tags=integration ./internal/repository -run 'TestBlueGreen' -count=1 -timeout=15m`均退出0，包含真实PostgreSQL18隔离数据库中的原238/241约束、旧新版读写、锁冲突回滚、结构/账本漂移、继承/RLS/触发器/超大表拒绝和退役保护解除恢复。
- 集成首轮失败来自测试夹具将`pg_get_expr`文本重新解析为SQL，改变了PostgreSQL内部类型转换表示；修正为执行真实238迁移，并新增原238/241约束表示精确回归，未放宽生产结构匹配。
- Python发布证据8项、状态机57项、预检3项、元数据5项测试及元数据实际校验全部通过。WSL Windows目录的Unix socket不支持及初次隔离目录过长分别造成环境失败，最终在工作区内短路径tmpfs中逐字复制相关代码复验，保留原断言；临时挂载已卸载。Go格式及`git diff --check`通过。
- 本地复验完成时，专项提交的完整CI、安全扫描、主线合入、tag及生产部署仍待完成，不沿用PR #26或旧提交的成功结论；后续实际结果见下文。完整日志保存在本轮证据目录。

## 专项提交与发布门禁

- 专项提交为`2349a7623468f646c63d8abc80928293fea402e9`，分支`codex/bluegreen-241-v01249`。PR #27的push/PR两轮门禁全部成功后，以merge方式合入主线`6e4161ea9aa804e8ed6dd42dffafa94bde7bcb33`，合并结果与候选树一致，保留上游祖先关系。
- 主线再次通过完整门禁后，于北京时间2026-10-03 23:33新建附注标签`v0.1.249`，对象`3a9bb377d6b0142397b487f65f2072ff511335b1`，指向上述主线提交。未移动旧标签，未通过workflow_dispatch绕过标签发布。

| 阶段 | CI运行 | 安全扫描运行 | 结果 |
| --- | --- | --- | --- |
| 专项分支push | `37132116339` | `37132116346` | 8+2全部成功 |
| PR #27 | `37132152517` | `37132152498` | 8+2全部成功 |
| 合并后的main | `37132887030` | `37132887104` | 8+2全部成功 |
| v0.1.249标签 | `37133683250` | `37133683257` | 8+2全部成功 |

- Release `37133683269`第1次执行已完成五个平台包及Docker Hub/GHCR镜像发布。校验清单自身SHA256与GitHub元数据匹配，五个平台包的清单摘要也逐项匹配；未下载、执行全部平台二进制，不等同于原生跨平台运行验收。
- Actions自动回写VERSION的提交为`9f35f89efe49e93dbaaec0bd344247eeb2d4b553`，只将0.1.248更新为0.1.249，保留既有`[skip ci]`机制；不将该自动提交描述为单独经过CI。镜像来源仍是不可变标签提交`6e4161ea9`。

## 切流与共存期核验

- 发布前23:29只读复核生产仍为v0.1.248/green、stable且无pending；311条历史迁移与上一轮最终验收逐项相同。最近完成备份记录为北京时间2026-10-03 12:00至12:12:01，4139354150字节；本轮未做备份恢复演练。
- Actions于北京时间2026-10-03 23:42:11.588切流至blue，耗时约1.203秒。新实例为`i21f56e78be994bb59def6a89a3dfb3fc`，容器为`5f7e343e59798cf9aeb4785f390be1029fa984659f340148a473c63953a02e8b`；旧green实例仍为`i72b1133a340441c59e4a1e19c1d66f69`，未提前重启或停止。
- 23:43只读核验活动镜像、运行程序、首页注入版本及公开设置接口均为0.1.249，两个入口均指向同一新实例，首页及Canvas静态脚本/样式可读取且类型正确。这不是登录后浏览器E2E或真实Provider调用验收。
- 迁移账本由311增至313，原311条摘要逐项不变，仅新增获批的两条241。共存期实际数据库报告`present=2`、`valid=true`、`migrations=true`、`schema=true`，两个TypeSafe写入保护仍在；Actions上传的保护脚本摘要与固定源文件一致。
- 双Compose、生产bind mount、资源配置、持久配置摘要、3600秒响应头等待及5秒usage任务未变。既有3600秒观察窗口的到期时间为北京时间2026-10-04 00:42:11.588；观察、旧槽退役与保护解除由Actions完成，最终结果另行追加。

## 最终退役与验收

- Release `37133683269`的attempt 1全部10个作业成功，含实际蓝绿部署步骤；未人工执行生产DDL、切流或缩短观察窗口。观察期间一次人工SSH只读采样连接超时，00:32重试成功，公开健康入口返回HTTP 200，未据此重启服务或认定业务中断。
- 满3600秒后，Actions于北京时间2026-10-04 00:42:12.680启动既有超时强退策略，00:43:25.352完成旧green回收。强退前HTTP工作计数3、SSE计数0、会话租约635，background、upstream_drains及三项usage计数均为0。
- 固定旧实例`i72b1133a340441c59e4a1e19c1d66f69`获得60秒停止宽限，最终退出137、非OOM；关停日志摘要记录`usage_drop_count=0`、`forced_shutdown_count=0`，清理626条旧实例归属且游标归零。但`clean_exit=false`、`usage_loss_unknown=true`，不能据此证明请求或用量无损，也不据此断言已确认丢失用量。
- 旧槽回收后重新核验身份、Compose、挂载和双入口健康，Actions于00:43:25.739自动解除两个241兼容保护；最终`platform_guard_result.status=released`且绑定发布SHA。只读数据库核验`present=0`、`migrations=true`、`schema=true`，原平台CHECK及赠金列均保留，没有反向删除迁移。
- 00:44最终状态为`stable`、`active=blue`、`pending=null`、`last_error=null`，仅剩`sub2api-blue`应用容器。活动镜像为`saviour2411/sub2api@sha256:f1c640deb34507230e5eee8710bb5514e80cca405331a0e820b89c45ed9686c6`，版本、OCI提交及实例身份匹配固定标签。
- 前后比较及Actions归档逐项核验通过：新blue切流后容器身份、启动时间及重启次数未变；PostgreSQL/Redis身份、启动时间、重启次数和挂载未变；双Compose及持久配置摘要未变，资源、3600秒响应头等待和5秒usage任务保持原值。迁移账本仍为313条，原311条摘要不变，仅有两条固定241新增。
- 首页注入版本和公开版本接口均为0.1.249，首页/Canvas共8个本机脚本与样式资源复验通过。最终双入口观察结果为api和direct各3507次请求、错误均为0，与Actions归档逐项一致；观察期间未将磁盘上的上一轮统计误当作本轮结果。
- 部署归档为`blue-green-deployment-37133683269-1`，artifact ID `11279201526`，下载SHA256为`2c5575a9dc4ede27b4123dfb4e223df72ae4047db1f5ecdc4ac0378b096f620a`，与GitHub元数据一致；固定SHA、镜像、旧版兼容基线、恰好两条241策略及保护解除结果均与生产匹配。
- 本轮验收记录作为独立文档提交，验证候选及主线门禁，不再打标签或改变部署代码。文档提交自身SHA及最终主线结果在交付总结报告，避免自引用或预写成功。

## 验证边界

- 未执行真实收费Provider、支付、兑换、邮件或完整登录后浏览器E2E，未做生产历史账单回放、备份恢复演练或峰值压测；健康探针无错误不等于业务请求或用量无损。
- Canvas最终已审计为0项high/critical、1项moderate、1项low；前端既有xlsx高危豁免未新增或延长，继续以仓库安全扫描及豁免清单为准，不宣称全部依赖无漏洞。
- 保留按用户串行扣费及5秒usage任务；既有Go软内存与PostgreSQL缓存预算合计超过物理内存的风险未消除，不将配置描述为峰值验证过的硬资源保证。

## 验证证据

- 本地证据目录：`output/release-20261003-v0.1.249`，包含Canvas审计、带unit标签后端验证、只读迁移结构与门禁重放结果。
- 不采集Token、私钥、生产配置正文或业务数据；生产只读命令经标准输入执行，未上传远端文件。
