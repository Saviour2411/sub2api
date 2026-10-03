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

## 生产只读核验

- 北京时间2026-10-03 08:24核验，生产状态`stable`、活动槽`green`、`pending=null`，版本仍为0.1.248、固定SHA为`211d3f29e9a8652e7f7323bda6bcada8ee7382c3`。历史强退记录属于上一轮发布，不是本轮结果。
- 08:38对PostgreSQL 18.4进行只读结构查询。`payment_orders.bonus_amount`已为`numeric(20,2) NOT NULL DEFAULT 0`，由既有149迁移建立；新增241赠金迁移仍未登记，不能因字段存在就跳过迁移账本校验。
- TypeSafe迁移将重建`user_platform_quotas`及`composite_model_routes`平台CHECK约束以增加`typesafe`。两表当前均为无继承普通表，含索引总大小分别393216和40960字节；这只是采样，不代替执行时锁内检查及旧版读写兼容性证明。
- 本地重放当前`migration_evidence`明确拒绝本轮Ent schema与SQL变化。现行审批仅覆盖普通可空列及已批准的239/240专项，不能直接套用到本轮241约束迁移。
- 尚未修改兼容审批或执行器、未执行生产DDL、未创建tag、未触发Release。需要用户确认专项兼容适配范围后继续，不通过人工预执行SQL、隐藏差异或改写历史迁移绕过门禁。

## 证据

- 本地证据目录：`output/release-20261003-v0.1.249`，包含Canvas审计、带unit标签后端验证、只读迁移结构与门禁重放结果。
- 不采集Token、私钥、生产配置正文或业务数据；生产只读命令经标准输入执行，未上传远端文件。
