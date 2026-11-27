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

## 实际发布结果

北京时间2026-11-28 03:34:21完成蓝到绿切流，双入口身份确认耗时约1.159秒。上线代码提交为248f681f2ba90835af52a55a7c93063a339a58a6，版本号保持0.1.272。生产镜像ID为sha256:5dcf19820fec1a72067ebbf9c12cc5bc54e8140b622e1af60a531b0cb155dc89，镜像revision和二进制vcs.revision一致，vcs.modified=false。

本地Docker29与生产Docker26的镜像ID表示不同；本次从归档配置blob计算生产镜像ID，并在docker load后再次核验。最终image.tar的SHA256为7bd37a20711652d3a2b96ede904dd048bca3d0881db4c81afca3ade2523c0a31；image.tar.gz为b3c11b61656d91bc23d94f7c240165552095cb80beee1e0c552d8bf686b724f7。上传前后归档、五类验收日志和固定提交发布器均校验一致。

后端受影响包的Linux完整unit、真实PostgreSQL迁移和任务恢复、专项race、前端测试/类型/构建及发布器回归通过。实际应用HTTP/SSE/WebSocket三次蓝绿切换验收中，1624次普通请求零错误，19笔上游请求对应19笔用量。本次未运行GitHub CI或Security Scan，未创建Git tag；不将本地验收描述为CI成功。

发布器执行120秒排空观察，API和direct入口各119次健康探针均成功。北京时间03:36:54再次核验，绿槽i8938fd547cb340ce8507519597487654为active且ready，数据库253迁移账本摘要正确，旧检测禁用约束已生效，新旧检测均关闭，旧检测未完成任务为0。检测需管理员补齐模型、题目、密钥及判题配置后启用；发布过程未发起付费模型检测。

生产data/postgres_data/redis_data的bind mount正确；.env、data/config.yaml及两份Compose的摘要与切流前完全一致，保留3600秒响应头、5秒usage任务和既有资源参数。双实例仍使用既有超配预算，发布器依据实际资源余量放行，不代表完成峰值压力验证。

截至上述核验时，状态为active=green、phase=drain_pending、pending=blue。旧蓝槽仍有9个HTTP、8个SSE工作计数及3794个会话租约，容器保留继续排空，未强制停止，不可复用蓝槽。健康探针零失败只能说明观测到的入口可用性，不等于逐笔证明所有客户请求无损。

发布清单、镜像、配置摘要、旧检测备份及production-result.json/state-after.json保存在生产/root/proj/sub2api/deploy/blue-green/releases/local-248f681f2-20261128；本地证据保存在tmp/model-quality-v2-20261128。旧检测备份仅存服务器且权限为0600。本地临时数据库和Vite已停止，两个测试tmpfs挂载已卸载。
