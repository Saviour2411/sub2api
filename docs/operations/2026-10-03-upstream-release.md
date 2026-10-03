# 2026-10-03 上游同步发布准备

## 范围

- 用户授权提交代码，经CI通过后合入远端主线，再创建新tag并由GitHub Actions自动蓝绿部署。预期下一版本为v0.1.249，尚未创建标签。
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
- 专项提交的完整CI、安全扫描、主线合入、tag及生产部署仍待完成，不沿用PR #26或旧提交的成功结论。完整日志保存在本轮证据目录。

## 验证证据

- 本地证据目录：`output/release-20261003-v0.1.249`，包含Canvas审计、带unit标签后端验证、只读迁移结构与门禁重放结果。
- 不采集Token、私钥、生产配置正文或业务数据；生产只读命令经标准输入执行，未上传远端文件。
