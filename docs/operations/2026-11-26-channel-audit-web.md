# 2026-11-26 渠道检测网页部署

## 部署范围

按用户要求，将附件中的现签、AWSB结构标记和工具续接检查纳入独立测试包，并在服务器2部署网页工作台。对应能力为`CUST-OBS-004`和`CUST-OBS-005`。本次操作对象仅为服务器2的新检测服务，没有连接服务器1，也没有修改生产网关、渠道或计费配置。

## 入口与文件

| 项目 | 配置 |
|---|---|
| 公网入口 | `https://audit.87-76-198-249.sslip.io:8444` |
| 服务器 | `87.76.198.249`，SSH端口22748 |
| HTTPS端口 | 8444，Nginx监听IPv4和IPv6 |
| 后端监听 | `127.0.0.1:18766` |
| 服务名 / 用户 | `channel-audit.service` / `channelaudit` |
| 工作目录 | `/opt/channel-audit` |
| 发布目录 | `/opt/channel-audit/releases/20261126-1` |
| 当前版本链接 | `/opt/channel-audit/current` |
| 历史任务与报告 | `/opt/channel-audit/runs` |
| 登录哈希配置 | `/opt/channel-audit/auth.json`，root拥有、服务组只读、权限640 |
| Nginx站点 | `/etc/nginx/sites-available/channel-audit` |
| systemd配置 | `/etc/systemd/system/channel-audit.service` |
| 证书目录 | `/etc/letsencrypt/live/audit.87-76-198-249.sslip.io` |

访问口令单独生成，服务器只保存PBKDF2-SHA256盐值、迭代次数和哈希。明文凭据仅写入本地交付目录`outputs/channel-audit-web-20261126/访问说明.md`，不随分享包或Git提交。待测渠道API Key只用于内存任务及子进程环境，任务配置、报告和日志均不保留Key。

## 安装与隔离

先检查服务器2端口、磁盘、内存、现有Nginx和容器状态，确认8444与18766未占用。部署使用独立目录和无登录权限的系统用户。Nginx先添加新域名的HTTP-01验证路由，通过现有Certbot的webroot方式签发证书，再启用HTTPS站点；只平滑reload Nginx。防火墙新增8444/tcp，其余规则保留。

证书首次到期日为2027-02-23。使用现有Certbot定时续期以及`/etc/letsencrypt/renewal-hooks/deploy/reload-nginx`钩子；不替换现有证书或其他站点。

服务设置开机自启，后端端口不对公网开放。systemd限制：非root运行、只允许`runs`目录写入、禁止提权、最多768个任务/线程、4096文件描述符、1GiB内存、200% CPU。客户端最多200并发，实际发送峰值和延迟仍由测试报告记录；服务器2环境与服务器1历史并发指纹不同。

网页所有写操作检查Origin和Host；使用HttpOnly、SameSite=Strict和HTTPS Secure会话Cookie。公网渠道地址同时检查DNS结果与连接后的实际对端，认证头发送前拒绝私网/回环/链路本地目标，不跟随重定向。Nginx限制请求体32KiB、每IP连接及请求速率。

## 行为与判定

CLI默认仍为三轮缓存和三条Token指纹，共6次生成。网页默认增加Opus与Sonnet各两次现签，最多10次生成；其他模型缺少固定指纹时明确显示未评估。伪造签名、工具续接、AWSB附加条件、并发均按页面选择执行。

现签必须在每一轮请求窗口内读取到`f2.f1.f21`且跨轮严格递增。缺少字段、无法解码、非完整SSE、拒答或时钟跳变均无法确认，不算通过。AWSB附加要求所有有效样本`f2.f1.f2=1`，这只是结构特征，不证明实际AWS调用归属。工具续接精确校验六位编号，保留全部原历史，前后原样对照失效则关联结论无法判定。

网页同时只执行1个测试任务，待完成任务最多5个、历史最多200个、每任务最长2小时。取消终止本地进程并保留已有事件，不能撤销上游已接受请求；重启后的未完成任务标为中断，不自动重跑。结果下载和删除均需登录。

## 验证记录

- Windows Python 3.12与服务器2 Python 3.13分别运行完整84项本地模拟回归，最终全部通过；均未调用付费渠道。
- 服务器首次回归发现旧宽松Base64模拟端依赖Python版本行为：Python 3.13对填充符后追加字符报错。修正测试夹具，显式在填充符处截断并严格验证前段，保留实际诊断逻辑和HTTP400可信标准；补齐null类型处理后重跑通过。
- 浏览器从外部通过HTTPS登录，默认计划API返回200及10次生成预算；私网URL返回400；未登录不能读取历史和报告，会话Cookie不能由JavaScript读取。
- 本地浏览器通过合成接口执行两轮现签，得到通过结果，查看签名字段、请求记录和历史，下载Markdown报告成功。桌面1440px与手机390px截图检查无页面横向溢出，JavaScript无报错。
- 公网`/health`返回200；服务处于active且enabled，后端仅监听回环；既有容器维持原运行状态。

交付证据位于本地`outputs/channel-audit-web-20261126/`：版本清单、部署归档、可分享ZIP、浏览器截图和验证JSON。部署归档只包含测试程序、静态资源、测试夹具、说明和部署模板，不包含访问口令、待测Key或历史测试结果。

## 日常维护

~~~bash
systemctl status channel-audit --no-pager
journalctl -u channel-audit -n 80 --no-pager
curl -fsS https://audit.87-76-198-249.sslip.io:8444/health
nginx -t
~~~

更新前从网页确认无运行或排队任务，准备新发布目录并完成本地模拟回归，再切换`current`链接并重启该独立服务。旧版本和`runs`均保留；登录口令哈希配置不随代码覆盖。回退只切换该服务链接，不涉及其他应用或数据库。

修改访问口令时在工作目录生成新的随机盐和PBKDF2-SHA256哈希，使用至少240000次迭代并保持`auth.json`权限；空闲时重启此服务使旧会话失效。不要把API Key或明文访问口令写入命令行、Nginx日志或Git。
