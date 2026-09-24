# Kimi K3 接口契约评估（2026-09-24）

## 结论与范围

本次调用用户提供的 direct 域名及 `kimi-k3`，不运行 OCRBench、MMMU、AIME、BEAM 或 DeepSWE 能力基准。结果是：**当前链路不能通过整个接口测试集，也不能据此声称与 Kimi 官方 API 完全一致**。

测试器固定为 MoonshotAI/Kimi-Vendor-Verifier 提交 `66092cf444c97356c0e11c5078c67116390615d9`，线上 Sub2API 为0.1.246，镜像修订 `94dac3dcb880dcf96e3677827d3b4af7a55b9c98`。请求实际经过第三方转发账号，不是 Sub2API 直接连接 Moonshot 官方域名；分组名称不能证明实际供应商。

本地修复签名挑战头和响应证明头的透传，保留现有参数兼容策略。本轮没有修改生产配置、重启容器、发布镜像或修改数据库；因此线上结果仍然是修复前的结果。

## 测试逻辑

- `tests/params`：合法参数应接受、非法参数应拒绝。但当前实现把所有异常都作为失败请求处理，负向断言只判断“没有成功”，超时、401、502也可能被计为测试通过，并非严格的HTTP 400校验。
- `tests/k3_features`：动态工具、结构化输出、工具选择与思考设置；大部分分别检查流式和非流式。思考项包含“问模型自己当前effort是什么”和比较推理文字长度的启发式断言，并非服务器配置证明；还包含短算术提示，不能将这一部分计为纯协议认证。
- `tests/prompt_tokens`：固定请求及本地图片，比较服务商返回的 `usage.prompt_tokens` 与固定参考值，容许最多多3个token。结果依赖模型版本、推理配置、聊天模板和实际路由，不能由网关人为修正计数来通过。
- `tests/tool_call_json_schema`：204个schema、流式和非流式各一次，共408项；调用真实模型并检验工具参数JSON，不是数学/视觉能力榜单，但仍受模型生成、解码约束和网络时延影响。
- 仓库不包含本次关注的 `X-Msh-Request-Nonce` 官方签名验证，因此另做独立探针。

## 执行方法与完整结果

先按原超时/重试运行，发现大量上游502叠加网关重试，执行约12分钟后中止，该次部分结果不混入统计。随后保持请求参数、测试断言和原始跳过标记不变，仅用外置pytest插件设置客户端缺省30秒超时、SDK重试0、pytest重试0，并用4个worker运行。测试显式传入的单次超时仍可能覆盖客户端缺省。

第一批记录321项，达到20分钟执行限额后，按nodeid仅补跑未完成290项。合并日志核对共有611个唯一nodeid，无重复，统计如下。这是**有界传输条件下的原始断言结果**，不是未修改运行条件的官方认证。

| 类别 | 通过 | 失败 | 跳过 | 总数 |
| --- | ---: | ---: | ---: | ---: |
| 参数行为 | 15 | 3 | 0 | 18 |
| 动态工具 | 0 | 54 | 0 | 54 |
| 结构化输出 | 16 | 8 | 0 | 24 |
| 思考设置/启发式 | 9 | 3 | 14 | 26 |
| K3额外token groundtruth | 0 | 0 | 2 | 2 |
| 工具选择 | 12 | 6 | 2 | 20 |
| Prompt Token | 42 | 17 | 0 | 59 |
| 工具参数JSON Schema | 372 | 36 | 0 | 408 |
| **总计** | **466** | **127** | **18** | **611** |

127个失败按日志表现分类：72个传输超时、36个非法请求被接受、14个token参考值不匹配、3个effort启发式不匹配、2个动态/全局工具选择不匹配。工具schema的36个失败全部是超时，不能归结为36个schema生成错误。超时放宽后可能改善计数，但不会解决已复现的非法参数接受或签名缺失。

## 官方契约与旧测试的冲突

截至2026-09-24，官方参数文档规定 `kimi-k3`：

| 参数 | 官方要求 |
| --- | --- |
| `temperature` | 固定1.0 |
| `top_p` | 固定0.95 |
| `n` | 固定1 |
| `presence_penalty`、`frequency_penalty` | 固定0 |
| 推理档位 | 顶层 `reasoning_effort`，仅low/high/max，缺省max |
| `thinking` | 文档归类为K2.x参数；K3始终开启推理及历史思考保留 |
| 动态工具 | `system`消息的 `tools`，不得同时带 `content` |

旧参数测试却要求思考模式接受温度0.6、0.0；旧effort测试要求嵌套 `thinking.effort` 覆盖顶层 `reasoning_effort`，与其文件开头说明也不一致；部分动态工具正向测试同时发送 `content: ""`。因此不能同时承诺“严格遵循当前文档”和“旧测试原样全绿”。

模型回答“当前是low”或某次推理变长，不是证明实际effort配置的可靠依据，不应据此改写系统提示让模型报出指定字符串。

## 失败归因

### 1. 非法参数与重试策略

独立官方契约探针中，缺省请求和所有固定参数合法值、low/high/max均返回200；`n=2`、`reasoning_effort=medium/invalid`也返回200，后者某次模型回显为 `k3`。非法温度、top_p及penalty出现90秒读超时。

绕过Sub2API直接请求同一个第三方上游，非法温度0.6与top_p=0.8复现HTTP 502，错误类型为 `smart_route_all_candidates_failed`，不是官方要求的400。服务器1该账号开启pool mode，对429/502/503/504最多重试10次，因此放大了等待时间。不能仅凭这个泛化502强制转换成400，否则会把真实容量、网络或余额问题误判为客户端错误。

生产五个Kimi参数兼容开关均已开启；它们的产品目标是遇到明确400后删除或调整不支持的参数、尽量完成请求，本就不同于“严格保留非法输入并返回400”。本次未擅自关闭这些已启用的功能。

### 2. 动态工具

原测试54项全部失败，但部分用例本身与当前文档不一致。为排除这一点，另按当前文档构造了不带 `content` 的system工具消息：第三方上游仍返回502，经Sub2API请求45秒超时。把同一工具放到顶层 `tools` 后，上游和Sub2API都返回200和 `get_weather` 工具调用。

本地回归确认原生Chat路径保留 `messages[].tools`、`reasoning_effort`、历史 `reasoning_content`、`response_format` 和 `tool_choice`。因此不能把上述现象简单归咎于网关丢字段。把动态工具搬到顶层可以作为降级策略，但会改变工具生效位置、前缀缓存和token计数，不是等价的官方动态工具实现，本次没有这样改写。

### 3. Prompt Token参考值

初次测试有多个视觉请求固定多68个token，关闭思考的旧用例也有差异。追加对照中，同一个旧 `reasoning_effort=none` 文本用例在第三方上游和Sub2API均返回36，与参考值匹配；合法max用例双方均返回99，也匹配。前后模型回显在 `kimi-k3` 和 `k3` 间变化。

这说明参考值偏差不能稳定归因为本项目增加68个token，也不足以单凭偏差断定假模型。不同上游分支、推理处理或模板是待确认因素；需供应商提供固定路由与版本，或用官方基线对照。网关必须保留原始usage，不能减68或覆盖成测试期望值。

### 4. 官方请求签名

官方签名流程是客户端发送随机 `X-Msh-Request-Nonce`，真实Kimi上游返回 `Msh-Request-Timestamp` 和 `Msh-Request-Signature`，再提交nonce、时间戳、实际请求模型和签名到官方 `/v1/signatures/verify`。

本次公网流式/非流式请求都没有两个响应头；绕过Sub2API直连第三方上游仍然没有。因此既有网关白名单会丢头是一个真实缺口，但不是当前链路无法获得签名的唯一原因。

用户域名的 `/v1/signatures/verify` 返回404。本次不新增假验证端点：没有官方证明时返回 `valid:true` 是错误实现。签名验证应在真正的官方接口使用有效官方密钥进行，不能把本站密钥直接当作官方密钥。

有效签名证明官方曾接受指定nonce、时间戳和模型，不证明最终内容完整或请求成功，也不天然防重放。若模型名称在网关被映射，必须按真实上游请求模型验证，不能篡改签名使客户端别名“匹配”。

## 已实现的本地修复

`CUST-PROTO-009`：仅Kimi账号透传 `X-Msh-Request-Nonce`，覆盖共用Chat出站、原生Messages、原生Responses及Responses透传构造器。默认响应白名单允许 `Msh-Request-Timestamp`、`Msh-Request-Signature` 原值返回，保留管理员 `ForceRemove` 优先级。

不会生成签名、不会虚构nonce、不转发客户端认证作为上游认证、不放行其他未知头、不改变参数/路由/usage/模型响应内容。上游没有签名时，下游也没有签名。

验证：新增流式/非流式、签名存在/缺失、跨平台隔离、原生Messages/Responses、认证隔离、显式删除和真实usage保留测试；`internal/service`与`internal/util/responseheaders`完整unit测试通过。完整611项结果仍针对生产旧代码，不能写作补丁上线后的通过率。

## 如何达到可验证的一致性

1. 先以当前官方文档修正测试规范：非法参数必须断言400；鉴权、限流、服务故障与超时单列；移除用回答自报effort和随机长度比较作为硬门禁；不把跳过算通过。
2. 为需要官方等价的调用建立明确的严格策略：固定参数及枚举本地预校验，不把非法请求“修正成功”；它与当前宽容兼容开关存在行为冲突，需决定应用范围后再实现，不宜全站静默切换。
3. 使用真正支持当前K3动态工具和官方签名传递的上游，固定路由/模型版本。当前上游的缺失无法仅靠Sub2API安全补齐。
4. 审核并发布本地签名透传补丁，再验证完整 `nonce → 原始签名 → 官方verify`；当前未部署、未证明验签成功。
5. 在足够超时下重测网络失败项，并分别报告功能错误、基础设施错误、旧测试不适用和成功，不只给一个总通过率。

## 证据与来源

本机项目内 `.analysis_tmp/kimi-k3-contract-20260924/` 保存：

- `reports/bounded-results.jsonl`：611个唯一用例结果，含失败详情。
- `reports/completed-first-batch.jsonl`、`reports/resumed-checks.xml`：两批测试证据。
- `reports/official-contract-probe.json`：独立参数/签名/路由探针。
- `upstream-probe.json`、`upstream-comparison.json`：绕过网关的对照。
- `docs/`：当日官方文档快照。
- `affected-package-unit-tests.log`：完整受影响包unit测试。

服务器2报告目录为 `/opt/kimi-vendor-verifier/reports/kimi-k3-contract-20260924/`。API密钥未写入上述报告或源码。

官方文档与测试源码：

- https://platform.kimi.com/docs/api/models-overview
- https://platform.kimi.com/docs/api/signatures-verify
- https://platform.kimi.com/docs/guide/use-dynamic-tool-loading
- https://platform.kimi.com/docs/guide/use-reasoning-effort
- https://github.com/MoonshotAI/Kimi-Vendor-Verifier/tree/66092cf444c97356c0e11c5078c67116390615d9/tests
