# Kimi 参数兼容重试

## 启用范围

“二开功能 → 网关 → Kimi 参数兼容重试”提供四个独立开关，默认关闭。仅 API Key 所属分组的类型为 `kimi` 时生效，不根据分组名称或模型名称启用，也不适用于综合分组。

支持 Chat Completions 出站：原始 CC 请求，以及 Messages、Responses 转换后的请求；流式和非流式均支持。原生 Anthropic、Responses 出站不变。

| 设置字段 | 开启后的行为 |
|---|---|
| `kimi_sampling_parameter_retry_enabled` | 上游明确拒绝采样字段后，删除已传入的 `temperature`、`top_p`、`top_k`、`presence_penalty`、`frequency_penalty`，使用渠道默认值重试；保留 `n`、`stop`。 |
| `kimi_reasoning_effort_retry_enabled` | 上游明确拒绝思考强度后改为 `low`；若明确给出的合法值不含 `low`，不重试。K3 的 Messages→CC 默认强度同时改为 `low`，显式强度原样保留。 |
| `kimi_tool_choice_retry_enabled` | 非法 `tool_choice` 或缺少 `function` 时，仅删除 `tool_choice`；保留 `tools` 和历史，不再强制选择工具。 |
| `kimi_max_completion_tokens_retry_enabled` | 上游明确报告 `max_completion_tokens` 与 `thinking_budget` 大小关系冲突时，仅删除 `max_completion_tokens`。 |

删除输出上限可能增加生成量、耗时和费用，开启预算兼容前应确认业务接受此变化。本站不自动增加 token 额度，也不改动客户的其他预算字段。

旧配置缺少开关字段时按关闭处理；旧管理客户端部分更新不会清空新设置。每个请求冻结首次进入兼容链路时的设置，运行中不随管理员修改而变化。

## 匹配与安全边界

- 只处理实际 HTTP 400 的有效 JSON 错误；校验明确的错误句式、字段名称及实际出站值/结构，支持标准 `error` 对象与百炼顶层 `code/message`。
- 泛化 `Invalid request parameters`、审核/额度错误、HTML、截断或歧义 JSON，以及描述中引用的错误文案均不触发。
- 每项每请求最多修正一次，总计最多额外派发四次。多个不同问题可以依次修正；不消耗或重置原有 pool HTTP 重试次数。
- 修正保留在当前请求内，普通同号重试或切号时重新应用字段级修改，不恢复已确认有问题的字段，也不覆盖新账号的模型映射。
- 客户端取消、原请求截止时间和既有首 Token 守卫仍生效。旧响应体与计时资源在下一次尝试前清理。
- 已提交下游响应后不重放，也不处理 HTTP 200 中的 SSE 错误。失败后不向下游拼接两次生成。

## 思考档位说明

百炼 Chat Completions 文档为阿里云直供 `kimi-k3` 列出 `low/high/max`，官方默认值是 `max`。本兼容功能按本站需求在 Messages→CC 缺省强度时选择 `low`，并非声称官方默认值为 `low`。

现有最终模型归一化已经为 Kimi 保留显式 `max`；本功能不回退该行为。开启思考兼容时，最终映射模型精确为 `kimi-k3` 的 Messages 转换不再使用通用缺省 `medium`。其他显式值先原样请求，明确被拒后才降为 `low`。

原始 CC、Responses 请求没有指定强度时不额外补入。开关关闭及其他模型的转换规则保持原样。

## 验收与排查

系统日志区分 `kimi.parameter_compat_repair`、`kimi.parameter_compat_dispatch` 和 `kimi.parameter_compat_response`，记录账号、规则、字段名、兼容重试序号及上游状态，不记录完整请求体或密钥。

运维上游尝试链使用 `kind=parameter_compat_retry`，`reason` 为兼容规则，`detail` 保存修改字段名和序号；思考修正额外记录有界脱敏的被拒强度与实际 `low`，保留既有原始请求强度计费字段。最终完整成功后通过现有 Recovered 记录展示；若最后仍失败，保留失败分类和尝试链。派发日志或上游响应头 200 本身都不等于完整恢复成功，必须结合最终请求结果检查。

先逐项启用并观察同一请求的第二次实际派发、最终成功和单次入账。不要用这些开关处理渠道余额不足、容量不足、Cloudflare 超时或已输出后的 HTTP/2 断流。
