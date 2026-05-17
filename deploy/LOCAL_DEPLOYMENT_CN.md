# Sub2API 本地部署文档

> 维护要求：后续每次修改部署方式、端口、域名、证书路径、Docker Compose、Nginx、Cloudflare 规则或运维命令时，必须同步更新本文档。

## 当前部署链路

- 外部访问地址：`https://api.saviour.cc.cd`
- Cloudflare：
  - DNS 开启橙云代理。
  - SSL/TLS 模式使用 `Full (strict)`。
  - Origin Rule 将 `api.saviour.cc.cd` 的回源端口改为 `2503`。
- 服务器：
  - Nginx 监听 `2503 ssl`。
  - Nginx 仅允许 TLS SNI 匹配 `api.saviour.cc.cd` 的请求进入 Sub2API，默认拒绝其它域名或 IP 直连请求。
  - Nginx 反代到 `http://127.0.0.1:18080`。
  - Sub2API 容器映射 `127.0.0.1:18080:8080`。

请求路径：

```text
用户 -> Cloudflare:443 -> 源站 Nginx:2503 -> 127.0.0.1:18080 -> Sub2API 容器:8080
```

## 本地文件

- Compose 文件：`deploy/docker-compose.sub2api.yml`
  - 本地构建镜像：`sub2api:local`
  - 使用根目录 `Dockerfile`
  - Dockerfile 固定使用 `pnpm@9`，与 GitHub Actions 保持一致，避免 `pnpm@latest` 的依赖脚本审批策略导致构建失败。
  - 数据持久化目录：
    - `deploy/data`
    - `deploy/postgres_data`
    - `deploy/redis_data`
- 环境文件：`deploy/.env`
  - 该文件包含密钥，不提交 Git。
  - 当前关键配置：
    - `BIND_HOST=127.0.0.1`
    - `SERVER_PORT=18080`
    - `ADMIN_EMAIL=saviour2411@163.com`
    - `POSTGRES_PASSWORD`、`JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 已固定生成。
- Nginx 模板：`deploy/nginx-sub2api-api.saviour.cc.cd.conf`
  - 目标安装路径：`/etc/nginx/sites-available/sub2api-api.saviour.cc.cd`
  - 启用软链：`/etc/nginx/sites-enabled/sub2api-api.saviour.cc.cd`
- 前端主题资产：
  - `frontend/public/theme/staly.png`：原始参考图，约 2.7MB，仅作为兜底资源保留。
  - `frontend/public/theme/staly-login.webp`：桌面端认证页背景，约 123KB。
  - `frontend/public/theme/staly-login-mobile.webp`：移动端认证页背景，约 51KB。
  - 后台页面使用同一套冷蓝机甲视觉语言，但不直接大面积铺图，避免影响数据阅读。

## 前端主题风格

- 当前本地分支已将全站视觉调整为 `staly.png` 参考的机甲冷蓝 HUD 控制台风格。
- 覆盖范围：
  - 认证页：优先使用 WebP 压缩主题图，叠加斜切装甲面板、HUD 网格、扫描线和冷蓝/橙色能量线。
  - 登录后布局：`AppLayout`、顶部栏、侧边栏、表格页容器统一使用冷蓝、钢铁灰、装甲切角、边角发光和数据终端风格。
  - 公共组件：按钮、输入框、卡片、表格、下拉框、开关、弹窗、Toast、空状态等通过全局样式统一。
  - 静态缓存：后端对 `/assets/` 输出 `public, max-age=31536000, immutable`，对 `/theme/` 输出 `public, max-age=2592000`；HTML 仍保持 `no-cache`。
- 运维注意：
  - 修改主题时优先改 `frontend/src/style.css`、`frontend/tailwind.config.js` 和布局组件，避免逐页重复硬编码。
  - 替换认证页大图时必须同时生成桌面和移动端 WebP，避免登录页重新加载 2MB 以上图片。
  - 后台保持高信息密度，避免大面积背景图直接进入表格和设置页内容区。

## Nginx 配置要点

证书路径：

```text
/root/cert/saviour.cc.cd/saviour.cc.cd.pem
/root/cert/saviour.cc.cd/saviour.cc.cd.key
```

Sub2API/Codex 相关请求可能使用带下划线的请求头，因此 Nginx 配置需要保留：

```nginx
underscores_in_headers on;
```

`2503` 端口需要配置默认拒绝站点：

```nginx
server {
    listen 2503 ssl default_server;
    http2 on;
    server_name _;

    ssl_reject_handshake on;
}

```

该配置会拒绝 `https://服务器IP:2503` 或错误域名的 TLS 握手。它不是 Cloudflare 回源白名单；如果客户端直连源站 IP 但主动带正确 SNI `api.saviour.cc.cd`，仍可能命中正式站点。要严格防源站绕过，需要在防火墙或云安全组中限制 `2503` 只允许 Cloudflare IP 段访问。

当前计划使用 `sites-available` 和 `sites-enabled`，不使用 `conf.d`。

## 系统安装步骤

安装 Nginx 站点配置：

```bash
sudo install -m 644 deploy/nginx-sub2api-api.saviour.cc.cd.conf /etc/nginx/sites-available/sub2api-api.saviour.cc.cd
sudo ln -sf /etc/nginx/sites-available/sub2api-api.saviour.cc.cd /etc/nginx/sites-enabled/sub2api-api.saviour.cc.cd
sudo nginx -t
sudo systemctl reload nginx
```

启动 Sub2API：

```bash
cd /root/proj/sub2api
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml up -d --build
```

查看状态和日志：

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml ps
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml logs -f sub2api
```

## 验证命令

验证容器本地健康检查：

```bash
curl http://127.0.0.1:18080/health
```

验证 Nginx 源站 HTTPS：

```bash
curl -k --resolve api.saviour.cc.cd:2503:127.0.0.1 https://api.saviour.cc.cd:2503/health
```

验证 IP/SNI 不匹配被拒绝：

```bash
curl -k https://127.0.0.1:2503/health
```

验证公网入口：

```bash
curl https://api.saviour.cc.cd/health
```

检查端口：

```bash
ss -ltnp
```

如果在受限执行环境中验证，`curl 127.0.0.1:18080` 或 `ss -ltnp` 可能因为沙箱网络/权限限制失败；以宿主机权限执行或以 Docker healthcheck 为准。

预期：

- Nginx 监听 `2503`。
- 只有 SNI 为 `api.saviour.cc.cd` 的请求会进入 Sub2API。
- Sub2API 只暴露在 `127.0.0.1:18080`。
- PostgreSQL 和 Redis 不暴露公网端口。

## 升级与同步上游

同步原作者更新时采用手动流程：

```bash
git fetch upstream
git checkout -b sync-upstream-YYYYMMDD
git merge upstream/main
```

处理冲突后，先测试再发布：

```bash
cd /root/proj/sub2api
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml build
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml up -d
```

后续如果改为 GHCR 或 Docker Hub 镜像发布，需要更新：

- `deploy/docker-compose.sub2api.yml` 的 `image/build` 策略。
- 本文档的启动、升级和回滚命令。

## 数据库迁移说明

应用启动时会自动执行 `backend/migrations/*.sql` 中尚未应用的迁移。当前本地分支新增：

- `136_daily_checkins.sql`
  - 创建 `daily_checkins` 表。
  - 用 `(user_id, checkin_date)` 唯一约束保证用户每天只能签到一次。
  - 记录签到奖励、签到前余额、签到后余额和创建时间。

升级部署前仍建议先备份 PostgreSQL 数据目录或执行数据库 dump。

## 本地定制功能记录

当前本地分支包含以下账号管理定制：

- 新增账号时，默认选中“上游报错后停止调度，测试通过后恢复”策略。
- 编辑 API Key、上游或 Bedrock API Key 模式账号时，页面会显示已保存 API Key 的只读查看区，支持显示/隐藏和复制；替换输入框仍保持“留空不修改”的行为。

### 2xx 响应语义错误识别

当前本地分支在 `系统设置 -> 网关服务 -> 2xx 语义错误识别` 新增独立配置卡片。

用途：

- 处理上游返回 HTTP 2xx，但响应内容实际是错误语义的场景。
- 命中规则后不透传原始上游语义，统一返回管理员配置的自定义错误内容。
- 如果命中的账号已开启“上游报错后停止调度，测试通过后恢复”策略，会自动设置 `schedulable=false`，后续手动测试或定时测试通过后再恢复。

规则配置：

- `semantic_error_detection_enabled`：全局开关，默认关闭。
- `semantic_error_match_max_chars`：匹配阈值，默认 `4096`，范围 `128-65536`。
- `semantic_error_rules`：JSON 规则数组。
  - `enabled`：规则开关。
  - `name`：规则名称。
  - `platforms`：平台过滤，空数组表示全平台；支持 `anthropic`、`openai`、`gemini`、`antigravity`。
  - `match_type`：`contains` 或 `regex`。
  - `pattern`：文本或正则表达式。
  - `custom_message`：命中后返回给客户端的内容。
  - `priority`：数字越小越先匹配。

页面兼容性：

- 设置页加载和保存回填时会对 `semantic_error_rules` 做前端归一化，确保 `platforms`、`match_type`、`priority` 等字段缺失或异常时不会导致页面白屏。

性能策略：

- 非流式响应：仅当响应字符数不超过阈值时匹配；超过阈值直接放行。
- 流式响应：只缓存上游开头不超过阈值的小段内容；如果流在阈值内结束则匹配，命中后返回协议兼容错误事件；如果达到阈值仍未结束，立即放行缓存并停止匹配。
- `contains` 匹配使用大小写不敏感逻辑；`regex` 使用 Go 正则表达式并在保存设置时校验。

协议返回：

- OpenAI/Chat Completions：返回 `{"error":{"type":"upstream_error","message":"自定义内容"}}`，流式返回 SSE error chunk。
- Anthropic/Claude：返回 `{"type":"error","error":{"type":"upstream_error","message":"自定义内容"}}`，流式返回 Anthropic SSE error 事件。

相关文件：

- `backend/internal/service/semantic_error.go`
- `backend/internal/service/semantic_error_test.go`
- `backend/internal/service/setting_service.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_chat_completions_raw.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/handler/dto/settings.go`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

同步上游时注意：该功能未新增数据库迁移，复用 `settings` 表保存配置；若上游改动设置页面、网关响应处理、OpenAI Chat Completions 转换或错误监控上下文，需要优先检查上述文件的冲突。

## Cloudflare 524 与流式心跳

当前本地分支新增“预响应流式心跳”能力，用于 Cloudflare 橙云代理下的长时间上游等待场景。

背景：

- Cloudflare 免费/Pro/Business 橙云代理在源站长时间没有返回响应数据时可能返回 524。
- Nginx 和 Sub2API 的超时已经配置得比 120 秒长，但如果 Sub2API 在等待上游响应头期间没有向 Cloudflare 写出任何字节，Cloudflare 仍可能先断开。

推荐配置：

```env
GATEWAY_PRE_RESPONSE_STREAM_KEEPALIVE_ENABLED=true
GATEWAY_PRE_RESPONSE_STREAM_KEEPALIVE_INITIAL_DELAY=80
GATEWAY_STREAM_KEEPALIVE_INTERVAL=10
```

当前 `deploy/docker-compose.sub2api.yml` 已把 `GATEWAY_PRE_RESPONSE_STREAM_KEEPALIVE_ENABLED` 默认值改为 `true`，新容器启动后默认开启。系统设置页面也提供热更新开关：

- 路径：`系统设置 -> 网关服务 -> 请求转发行为 -> 预响应流式心跳`
- 页面保存后会写入数据库设置并刷新进程内缓存，新请求会按页面配置生效。
- 数据库设置优先于环境变量；环境变量只作为首次初始化或设置缺失时的默认值。

行为说明：

- 仅对 `stream=true` 请求生效。
- OpenAI/Codex `/v1/responses` 和 `/v1/chat/completions` 使用 SSE 注释心跳：

```text
: keepalive

```

- Claude `/v1/messages` 使用 Anthropic 兼容 ping：

```text
event: ping
data: {"type":"ping"}

```

- 如果已经发送过预响应心跳，HTTP 状态码已固定为 `200`；后续上游失败会以流内错误事件结束，不能再改成普通 JSON 错误状态码。
- 该方案不能解决客户端自身超时，也不能让真正卡死的上游恢复；它只用于防止 CDN 因源站长时间无字节而 524。

回滚：

```env
GATEWAY_PRE_RESPONSE_STREAM_KEEPALIVE_ENABLED=false
```

## 本地功能改动记录

### 每日签到领余额

当前本地分支新增普通用户每日签到功能。

管理入口：

- `系统设置 -> 用户默认设置`
- 可配置是否开启每日签到。
- 开启后可选择奖励模式：
  - 固定额度：每次签到发放固定余额。
  - 随机范围：在最小和最大额度之间随机发放，精确到 `0.01`。

用户入口：

- 用户个人资料页的余额卡片会显示“每日签到”按钮。
- 普通用户每天只能签到一次。
- 今日已签到后按钮显示“今日已签到”，并显示今日获得额度。

后端接口：

- `GET /api/v1/user/checkin/status`
- `POST /api/v1/user/checkin`

实现要点：

- 签到记录使用独立 `daily_checkins` 表，不复用兑换码表，避免污染卡密/兑换码管理。
- 签到奖励会增加用户余额，但不会计入 `users.total_recharged`，也不会触发邀请返利。
- 签到成功后会失效用户余额/API Key 鉴权缓存。
- 管理员余额历史会显示 `daily_checkin_balance` 类型记录，便于审计。

相关文件：

- `backend/internal/service/daily_checkin_service.go`
- `backend/internal/handler/daily_checkin_handler.go`
- `backend/migrations/136_daily_checkins.sql`
- `frontend/src/components/user/profile/ProfileInfoCard.vue`
- `frontend/src/views/admin/SettingsView.vue`

### CLIProxyAPI/Codex 账号 JSON 导入

账号管理的“导入”弹窗已增强为自动识别两类 JSON：

- Sub2API 自身导出的账号/代理数据：继续走原有数据导入接口。
- CLIProxyAPI/Codex 单账号或账号数组 JSON：走现有 Codex session 导入接口。

CLIProxyAPI/Codex 导入行为：

- 重复账号默认更新已有账号。
- 默认不绑定默认分组，导入后保持未分组。
- `disabled` 字段会被忽略，不映射到账号状态或调度开关。
- `expired` 会映射到账号凭据过期时间。
- `websockets=true` 会映射为 OpenAI OAuth WebSocket v2 `ctx_pool`；`websockets=false` 会映射为 `off`；字段缺失时不覆盖已有配置。

相关文件：

- `backend/internal/handler/admin/account_codex_import.go`
- `backend/internal/handler/admin/account_codex_import_test.go`
- `frontend/src/components/admin/account/ImportDataModal.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

同步上游时注意：该功能复用原有 `/admin/accounts/import/codex-session` 接口，未新增数据库字段或迁移；若上游改动账号导入弹窗或 Codex 导入接口，需要优先检查上述文件的冲突。

### 账号失败调度策略

当前本地分支新增了账号级可选策略：`上游报错后停调度，测试通过后恢复`。

实现要点：

- 配置保存在 `accounts.extra.failure_scheduling_strategy`。
- 策略值为 `disable_until_test_pass` 时，账号调用上游遇到上游错误会设置 `schedulable=false`，不修改 `status=error`。
- 自动停调度运行标记保存在 `accounts.extra.failure_strategy_unscheduled`。
- 停调度和运行标记写库使用独立短超时上下文，不依赖客户端请求是否仍连接；这修复了长流式请求结束时 `context canceled` 导致账号未被停调度的问题。
- 账号管理里的手动测试成功、或定时测试成功且启用自动恢复时，会清除该运行标记；只有存在该标记时才恢复 `schedulable=true`，避免误恢复管理员手动停调度的账号。
- 默认策略保持原作者原有行为，不影响未开启该策略的账号。

生产补偿记录：

- `2026-05-14`：`any-openai` 账号多次 `/v1/responses` 上游返回 `524`，但旧逻辑在客户端请求上下文已取消时写库失败，日志出现 `failure_strategy_marker_update_failed` 和 `failure_strategy_set_unschedulable_failed`，错误为 `context canceled`。
- 已手动补偿：`accounts.id=8` 设置 `schedulable=false`，并写入 `extra.failure_strategy_unscheduled`，状态码 `524`，原因 `upstream failover error`。

相关文件：

- `backend/internal/service/account.go`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/handler/failover_loop.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_images.go`
- `frontend/src/constants/account.ts`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

同步上游时注意：该功能刻意使用 `extra` 存储，未新增数据库迁移；若上游改动账号表单或 failover 处理，需要优先检查上述文件的冲突。

### 前端机甲沉浸主题

当前本地分支将前端主题调整为暗色优先的强沉浸机甲控制台风格。

实现要点：

- 无本地主题偏好时默认启用暗色主题；用户手动切换到浅色后仍会保存并继续生效。
- 通过全局设计系统覆盖卡片、表格、侧边栏、顶部栏、弹窗、按钮、输入框、徽章、移动端表格卡片等基础 UI。
- 登录页继续使用已优化的 `/theme/staly-login.webp` 与 `/theme/staly-login-mobile.webp`，避免直接加载根目录未跟踪的原始大图 `staly.png`。
- 本地 pnpm 缓存目录 `frontend/.pnpm-store/`、`frontend/.pnpm-home/` 已加入 `.gitignore` 与 `.dockerignore`，不要提交依赖缓存，也不要带入 Docker 构建上下文。

相关文件：

- `frontend/src/style.css`
- `frontend/src/main.ts`
- `frontend/src/components/layout/AppLayout.vue`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/components/layout/AuthLayout.vue`
- `frontend/src/components/layout/TablePageLayout.vue`
- `frontend/src/components/common/DataTable.vue`
- `frontend/src/views/auth/LoginView.vue`

同步上游时注意：本次没有新增后端接口或数据库迁移；若上游改动全局样式、布局壳层、登录页或表格组件，需要优先检查上述文件的冲突。

## 备份与回滚

备份部署数据：

```bash
cd /root/proj/sub2api
tar czf sub2api-deploy-backup-$(date +%Y%m%d-%H%M%S).tar.gz deploy/.env deploy/data deploy/postgres_data deploy/redis_data
```

停止服务：

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml down
```

回滚代码或镜像后重新启动：

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.sub2api.yml up -d --build
```

## 当前未完成事项

- Nginx 配置已安装到 `/etc/nginx/sites-available/sub2api-api.saviour.cc.cd`。
- Nginx 软链已启用：`/etc/nginx/sites-enabled/sub2api-api.saviour.cc.cd`。
- Nginx 已通过 `nginx -t` 并 reload。
- Sub2API、PostgreSQL、Redis 已通过 Docker Compose 启动并处于 healthy 状态。
- 本机健康检查已通过：
  - `http://127.0.0.1:18080/health` 返回 `200 {"status":"ok"}`。
  - `https://api.saviour.cc.cd:2503/health` 通过 `--resolve api.saviour.cc.cd:2503:127.0.0.1` 回源返回 `HTTP/2 200 {"status":"ok"}`。
- Nginx 已配置 `ssl_reject_handshake` 默认拒绝站点，用于拒绝 IP 或错误域名访问 `2503`；`https://127.0.0.1:2503/health` 验证为 TLS 握手失败。
- Cloudflare Origin Rule 需要在控制台确认已配置：`api.saviour.cc.cd` 回源端口 `2503`。
