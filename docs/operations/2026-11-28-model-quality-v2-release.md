# 2026-11-28 多平台自定义降智检测发布

## 授权与范围

用户批准开发及生产更新，跳过CI、Git tag和版本号更新，必须蓝绿发布，不中断正在处理的业务。功能编号CUST-OBS-003，详见docs/model-quality.md。

## 遗留槽处理

此前绿槽来自2026-11-26 Passkey配置切换，强退选项未启用，24小时观察结束后保留；并非未走CI导致漏触发强退。按本次用户明确授权，核验所有工作计数为零后执行既有发布器的retire-pending和force-after-window。旧绿退出137、非OOM，31条会话归属清除，clean_exit=false、usage_loss_unknown=true。双入口各83次健康探针通过，状态恢复stable，active=blue、pending=null。

## 当前发布基线

活动生产SHA为3d5d7724c42bbfd712220b901205f7ce175fdc3d，版本0.1.272，镜像仓库摘要sha256:198b5639e21d1d48e3205d1a05eed78f562380095c418ddb02c4508ccd6a3a3d。本次保留版本号，通过镜像revision区分代码。

采用现有生产运行时作为固定基础镜像，本地构建并嵌入新前端及Go二进制，替换资源目录。deploy/Dockerfile.local-verified记录构建方法。生产仅docker load导入不可变image ID，不依赖可变tag或pull。

## 验收和切换

本地验收包括后端受影响模块、真实PostgreSQL18并发与完整迁移重放、竞态、前端类型/交互/构建、发布器回归。报告绑定固定SHA及日志摘要；evidence.py --mode local校验干净提交与全部日志，保留ci=not_run、security=not_run。

253固定保护仅允许本次迁移及当前生产基线。先暂停旧检测调度并等待所有旧任务终结，再由绿槽standby启动执行迁移；结构漂移、同名对象、锁冲突、未停止的旧任务均拒绝升级。旧检测启用约束永久保留，应用回退不重新启用第三方检测。

生产数据继续使用deploy/data、deploy/postgres_data、deploy/redis_data。两份活动Compose保持一致，响应头3600秒、usage任务5秒及既有资源参数不变。绿槽通过健康与生命周期验证后，由既有发布器原子切换API/direct共用upstream；本次不传force-after-window，旧蓝槽自然排空，观察超时仍保留。

实际执行状态、镜像ID、健康探针及验收日志保存在生产blue-green/releases对应目录和本地tmp/model-quality-v2-20261128，发布完成后追加结果。
