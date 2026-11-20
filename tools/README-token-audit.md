# 渠道Token、缓存、思考签名与并发测试

Python 3.10及以上，无第三方依赖。入口是 `tools/probe_token_audit.py`，实现和内置指纹位于旁边的 `tools/token_audit/`。脚本只向指定API发送测试请求，不读取生产数据库或修改渠道配置。

## 最简用法

在仓库根目录执行，只需要URL和Key：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key"
~~~

Windows也可以使用 `python` 代替 `python3`。URL支持根地址、带路径的 `/v1` 地址，以及完整 `/v1/messages` 地址。默认认证头为 `x-api-key`；需要Bearer的接口加 `--auth bearer`。

默认模型为 `claude-opus-5-5`。默认最多发起6次生成请求，会产生正常上游用量：

1. `conversation`：三轮真实连续对话。首轮带独立随机前缀，后两轮带回上轮真实可见回复并追加虚构业务数据，显式设置5分钟缓存断点。检查命中率、缓存衔接、TTL字段和响应完整性。
2. `fingerprint`：重放三条内置的完整请求快照，比较输入总量。第二、三条快照包含采集时的历史回复，不拿本次回答替换，因此这项不是新的真实连续对话。固定快照可能已被预热，缓存衔接只由第一项判定。

只跑三轮缓存对话：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests conversation
~~~

仅运行这项且没有实时参照时，输入总量显示“未评估”，不会套用不同对话的历史Token数。拒绝、空回复、未正常结束时停止续接，不伪造下一轮历史。所有测试都不自动重试；目标端401/403/429或响应回显认证密钥时停止后续目标请求。

Key也可以通过环境变量提供，避免出现在命令行历史中：

~~~powershell
$env:TOKEN_AUDIT_API_KEY = "你的Key"
python tools/probe_token_audit.py --url https://你的上游域名
~~~

## 并发能力测试

只测并发，在URL和Key之外选择测试项即可；默认一轮200请求、200个工作线程同步发起：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests concurrency
~~~

可修改并发数、模型或输出上限：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --tests concurrency --concurrency 100 --model claude-opus-5-5 --load-max-tokens 2048 --load-origin "我的测试机"
~~~

上例使用环境变量`TOKEN_AUDIT_API_KEY`。并发范围1至200；本项每轮请求数等于并发数，没有额外预热、重试或自动请求可信端点。每个请求使用独立新连接和会话。连接阶段上限为min(30秒, --timeout)，请求总预算由`--timeout`控制，默认180秒。操作系统DNS与连接错误仍可能先于或晚于相应预算返回。

`--load-max-tokens`独立控制本项，默认2048；`--max-tokens`仍只调整三轮真实对话。短正文也可能消耗默认思考预算，因此报告将以下口径分开：

- 接口正常结束率：HTTP200且协议完整、无流内错误，包含`max_tokens`预算停止。
- 完整正文成功率：接口正常结束、正文非空、`end_turn`、返回模型名称一致。
- 首字P50/P90/P95：收到第一个非空白可见文本的时间，不把响应头、ping或思考算首字。
- 完整响应P50/P90/P95：收到完整`message_stop`；JSON响应按完整消息计时并单独标注。
- 分位数仅统计完整成功请求；失败保留在请求总数分母，记录失败阶段、HTTP状态、流内错误、预算停止和请求ID。
- 同时报告客户端在途峰值、已发送在途峰值、发送跨度；客户端200并发不保证200个请求同时到达渠道。

内置可信历史参考为用户指定的`direct.saviour.cc.cd`，2026-11-19 02:21:56至02:22:14（北京时间）从服务器1实测，模型`claude-opus-5-5`、200并发、2048输出上限。完整正文与接口正常结束均200/200；首字P50/P90/P95为12.033/14.374/15.432秒，完整响应为12.698/15.058/16.124秒。

参考文件位于`tools/token_audit/baselines/concurrency-claude-opus-5-5.20261119.json`，保存参数哈希、来源、时间、样本数、统计和原始证据哈希，不包含Key。模型、提示词、并发数、输出预算、认证或超时不一致时，仍完成测试，但不计算性能倍数；相同参数也只作历史描述性对照，不设置固定性能合格线，不据此判断模型真假、注水或持续容量。`--load-origin`只是来源标签，不会连接服务器；未确认相同测试环境时报告会明确提示。

可通过`--load-baseline-file 自定义参考.json`加载同格式参考。文件在发请求前校验。参考是离线文件，`--reference-url`仍仅用于三轮对话，不会因并发项额外调用可信渠道。

与已有项目组合：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --tests conversation,fingerprint,signature,concurrency --dry-run
~~~

默认同模型组合最多306次目标请求。只输入URL和Key仍是原来的6次检查，不会自动跑并发。并发一轮中的401/403/429及密钥回显会阻止后续测试项；已经同时发出的请求会收集完结果，保持分母完整。纯并发轮完整收集到错误或预算停止时退出1，测试未完成或后续项目被跳过时退出3。

## 选择模型与测试项

~~~bash
python3 tools/probe_token_audit.py --list-models
python3 tools/probe_token_audit.py --list-tests
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --model "你的模型名"
python3 tools/probe_token_audit.py --url https://你的上游域名 --dry-run
~~~

`--model`接受目标接口支持的任意模型名，协议仍为Anthropic Messages。`--list-models`列出本地已有指纹的模型，并非上游完整模型列表。未提供模型时，使用所选指纹的模型；未指定指纹文件时使用内置默认模型。

其他模型没有匹配指纹时仍可运行真实对话，但固定指纹项会标记“无法判断”并跳过，不能用Opus的标准去判其他模型。`--dry-run`只展示请求数量及测试计划，不需要Key，不联网、不创建文件。

## 思考签名篡改测试

签名测试是可选项，默认不运行。只测试签名：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests signature
~~~

缓存、Token指纹和签名一起测试：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests conversation,fingerprint,signature
~~~

签名测试默认最多100次目标请求，联合测试默认最多106次，均可能消耗上游额度。请求预算为：场景数 × 每场景样本数 ×（1次获取签名 + 21种变更 + 3次原样对照）。每种场景默认2份独立来源，各25次，共4份来源。每份来源有前、中、后3次原样续接对照；每个负例都从同一原始请求独立修改，不把其他负例的回答接入历史。

两种场景：

- `chat`：先请求多步余数计算，获取真实思考块及签名，再带回完整回答续接核验。
- `tool`：先计算约束问题，再要求调用虚构的 `query_probe_metrics`；完整回传思考、签名及工具调用，提供本地合成的三天请求数据后续接。不会执行模型发出的工具调用。

覆盖21种情况，其中18种计入核心统计：

| 类别 | 情况 | 数量 |
|---|---|---:|
| 签名内容 | 两份等长随机签名；头部、中部、尾部、随机位置单字符修改；解码后翻转一位；拼接另一真实签名片段；解码后追加8字节再编码 | 9 |
| 格式 | 截去末尾4字符、截断至一半、空串、删除字段、非法字符、null | 6 |
| 绑定 | 另一响应的完整签名；保留签名但追加或替换思考文本 | 3 |
| 编码边界 | 签名字符串末尾追加8个字符 | 1 |
| 兼容性对照 | 删除整个思考块、替换为另一响应的完整真实思考块 | 2 |

单字符修改会验证解码后的载荷确实变化，避免只改到Base64填充位。字符串尾随追加可能被解码器忽略，因此单列编码边界；真正追加8字节计入核心统计。格式拒绝只能说明字段校验，不能单独证明密码学验签。兼容性对照的接受或拒绝不作为核心异常。

只有同一样本的前、中、后原样对照全部完整成功，核心结果才计入有效统计。后置对照失败时，此前的负例结果也失效。明确的HTTP400签名无效、签名字段必填或类型错误会单独记录，HTTP200内的SSE签名错误另列“流内明确拒绝”；普通400、其他JSON错误、鉴权、限流、5xx、断流、空回复、模型不一致都不当作成功验签。有效对照下的核心变更被完整接受会提示核验，但不能据此证明模型真假，也可能是网关过滤、重签或协议转换。

每次直接在待测渠道取得新鲜签名，不内置历史签名、不把可信渠道的签名跨渠道回放。原有Token指纹不参与这项判定。缺少有效签名、重复签名、无法无损保存的内容块或无法解码的字节级用例都明确记录为未完成，不自动重试或伪造样本。连续3次传输、断流或服务器错误会停止后续目标请求。

本项当前使用 `thinking={type:adaptive,display:summarized}`；普通对话 `effort=high`，工具场景 `effort=max`。可用 `--model`选择模型，目标需要支持这些参数并返回 `thinking.signature`；不支持时报告无法判断，不自动改参数掩盖差异。

~~~bash
# 仅普通对话，两份独立来源，最多50次请求
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests signature --signature-scenarios chat

# 每种场景增加到3份独立来源，最多150次请求
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests signature --signature-samples 3

# 先看预算，不联网、无需Key
python3 tools/probe_token_audit.py --url https://你的上游域名 --tests signature --dry-run
~~~

`--signature-samples`范围2至5；`--signature-scenarios`可选 `chat`、`tool`或 `chat,tool`。`--signature-max-tokens`控制本项每次请求的输出上限，默认4096，范围1024至16384；原有 `--max-tokens`仍只控制三轮缓存对话。`--reference-url`仍仅用于真实对话的Token对照，不用于签名测试。

## 内置可信指纹

内置文件 `tools/token_audit/fingerprints/claude-opus-5-5.20261104.json` 来自2026年11月4日用户指定的0注入现签参照组的真实响应：

| 轮次 | 普通输入 | 缓存读取 | 缓存创建 | 总输入 |
|---|---:|---:|---:|---:|
| 1 | 4 | 0 | 18,045 | 18,049 |
| 2 | 4 | 18,045 | 1,877 | 19,926 |
| 3 | 4 | 19,922 | 1,890 | 21,816 |

指纹包含模型、版本、采集时间、三条完整请求、原始usage、响应ID以及每条请求的SHA256。测试语料保留原始英文文本，以维持Token计数的可复现性；说明和报告使用中文。

SHA256基于UTF-8、键排序、紧凑分隔符且不转义非ASCII的规范化JSON。它检查请求内容是否被修改，不是来源数字签名，也不证明模型身份。指纹没有API Key和供应商入口，默认客户端保留采集时的非认证协议头。服务端变更、隐藏提示或模型别名变化都可能改变计量；检测到偏差时应结合请求ID核验。

只重放固定指纹，使用严格的零容差：

~~~bash
python3 tools/probe_token_audit.py --url https://你的上游域名 --key "你的Key" --tests fingerprint --tolerance-tokens 0 --tolerance-ratio 0
~~~

默认容差为 `max(32 Token, 参照总输入 × 0.5%)`。超过容差时分别报告偏高和偏低，不把偏高直接定性为恶意注水。

## 实时参照

需要精确比较本次真实对话时，提供可信参照的URL与Key：

~~~bash
python3 tools/probe_token_audit.py --url https://待测上游 --key "待测Key" --reference-url https://可信上游 --reference-key "参照Key"
~~~

默认最多9次请求：目标端真实对话3次、固定指纹3次、参照端3次。所有目标请求结束后才发送实时参照，避免参照提前预热目标对话；参照使用相同模型、完全相同请求体和会话头，哈希必须一致。参照生成的新回复不会被拼入目标对话。

`--tests conversation` 配合实时参照时最多6次请求。参照密钥也支持 `TOKEN_AUDIT_REFERENCE_KEY`，认证方式支持 `--reference-auth bearer`。参照端失败或响应模型不同会标记对照不可用；不会将失败结果作为正常标准。

## 采集其他模型的指纹

先在你信任的接口上采集三轮真实对话：

~~~bash
python3 tools/probe_token_audit.py --url https://可信上游 --key "可信Key" --model "你的模型名" --tests conversation --save-fingerprint output/my-trusted-model.json
~~~

只有三轮都正常完成、输入usage完整且响应模型一致时才保存。该操作保存实测标准，可信性来自你对采集端点的选择；它不会自行证明新模型的Token数量正确。

随后用同一个文件测试其他接口：

~~~bash
python3 tools/probe_token_audit.py --url https://待测上游 --key "待测Key" --fingerprint-file output/my-trusted-model.json
~~~

自定义指纹直接提供对应模型和语料，无需改代码。文件必须保持三轮连续历史、纯文本Messages、5分钟缓存断点、完整usage及有效请求哈希。不要通过修改 `model` 字符串给旧指纹换模型。已有指纹文件和已有输出目录都拒绝覆盖。

## 输出与判定

默认保存到当前工作目录的 `output/token-audit-时间戳-随机编号/`，也可以通过 `--output`指定一个新的目录。所有生成文件必须位于当前工作目录内。

| 文件 | 内容 |
|---|---|
| `plan.json` | 测试计划、模型、请求数量上限及指纹版本 |
| `events.jsonl` | 逐请求记录，包含请求体、最终usage、初始及末尾usage、回复、错误和响应ID；签名项额外保存完整思考/签名/工具块和初步分类，最终对照有效性以summary为准 |
| `summary.json` | 完整结构化结果、参照结果、偏差与缓存衔接 |
| `rounds.csv` | 可供表格软件查看的逐轮输入、缓存、倍数和请求ID |
| `signature-cases.csv` | 选择signature时生成：场景、样本、变更、HTTP、判定、原样对照有效性、是否计入核心统计、变更路径、载荷变化、请求ID和错误 |
| `concurrency.csv` | 选择concurrency时生成：逐请求HTTP、接口/正文成功、失败阶段、首字/完整响应/建连时间、停止原因和请求ID |
| `report.md` | 中文结果报告及需要核验的项目 |

所有输出统一脱敏目标和参照Key。只记录必要响应头，不保存Cookie；API重定向不会被跟随。遇到中断时保留已完成轮次。Python标准库的DNS解析受操作系统控制，可能超过设定时间；建立连接后使用总时间预算关闭超时请求，不把超时部分响应视为成功。

计量公式：

- 总输入 = `input_tokens + cache_read_input_tokens + cache_creation_input_tokens`。缺字段、负数或非整数不能补成0。
- 命中率 = 缓存读取 / 总输入，零分母显示不可用。
- 衔接差 = 本轮缓存读取 - 上轮（缓存读取 + 创建）。相同前缀完整复用时应为0；非0还可能是失效、切换后端或提示改写。
- 两轮开始时间间隔达到5分钟时，缓存衔接标记为无法确定，不按过期后的缓存差值判注水。
- `message_start`和`message_delta`的usage按累计值合并，不重复累加。只有完整 `message_stop`事件才能结束SSE成功检测；没有事件末尾空行、断流、错误事件或仅有`[DONE]`都不能代替有效终态。
- 请求5分钟缓存而响应报告1小时创建，或分项之和不等于创建总量，都会单独列出；不据此断言上游账本已按1小时档扣款。

常用参数：`--timeout 180`设置每请求超时；`--interval 0`设置真实对话间隔；`--max-tokens 2048`只调整真实对话输出额度。固定指纹必须保持采集时参数，不能随该选项改变。

退出码：

| 退出码 | 含义 |
|---:|---|
| 0 | 所选检查完成且未发现超阈值偏差；不代表所有方面都已验证，需看 `input_quantity_evaluated` |
| 1 | 检查完成，存在计量、缓存、签名观察，或并发错误、预算停止、实际到达峰值不足等需核验情况 |
| 2 | 配置、文件或报告保存错误 |
| 3 | 有失败、未完成或无法判断的测试，包括无匹配指纹、缺签名、签名用例跳过或原样对照失败；预期的明确签名拒绝本身不算失败 |
| 130 | 用户中断 |

## 扩展测试项

新增测试函数放在 `token_audit/runner.py`，通过 `@register("测试名", "中文说明", max_requests=请求数量)`注册。函数接收 `RunContext`，使用 `context.call()`发送目标请求，以共用认证、超时、脱敏、流式解析和输出。注册项会自动出现在 `--list-tests` 中，可通过 `--tests`选择；默认测试列表保持显式配置，新增项目不会自动增加用户请求费用。

每条结果的 `observations`记录 `warning`或`incomplete`及具体原因。`max_requests`可以是固定数量或接收CLI选项的预算函数，通过 `TestSpec.request_limit(options)`统一计算计划和完成条件；未取得这些轮次会标记不完整。签名逻辑位于 `signatures.py`，变更生成位于 `signature_cases.py`；通过 `context.call(..., capture_content=True, check_usage=False)`复用传输并自行判定预期拒绝，不套用Token缺usage规则。完整内容解析由专用解析器实现，不修改全局解析器。跨渠道对照放在目标测试完成之后，不能在真实对话中插入共享参照预热请求。

并发逻辑位于`concurrency.py`，经同一注册表及预算接口接入；工作线程只负责独立请求，主线程串行保存结果和事件，避免并发写入日志。协议层共享认证、解析和计时；并发测试自行区分预算停止与请求错误，不套用Token注水规则。后续测试项可以沿用注册表扩展，无需改写现有测试。

协议模块只负责传输和Messages解析，`fixtures.py`只负责指纹验证及采集，报告汇总不依赖任何生产数据库或分组ID。

## 分享给渠道

请一起提供入口、完整的 `tools/token_audit/`模块目录、指纹文件、并发参考和本说明；只发送入口文件无法运行。分享包可移除历史指纹的来源标签和响应ID，但三条请求体、usage、模型及请求SHA256必须保持原样。渠道只需自行提供URL和Key，不需要服务器SSH或数据库权限。

## 本地验证

~~~bash
python3 -B -m unittest discover -s tools -p 'test_probe_*audit.py' -v
~~~

回归使用历史实测指纹和本地HTTP模拟服务，覆盖完整命令行流程、真实续接、精确重放、指纹导出再载入、参照后置、usage累计合并、断流、超时、拒绝、认证失败、未知模型、TTL、重定向及脱敏。签名回归覆盖两种场景、21种变更、严格/宽松模拟端、流式与JSON内容保留、泛化400、流内拒绝、后置对照失效、重复来源、密钥藏在思考/签名/工具参数、限流停测和中断保留。不消耗真实上游额度。

并发回归要求模拟服务收到全部200个请求后才发出任何响应，核验实际发送峰值；另外覆盖首字/思考区分、失败分母、输出预算停止、限流后续停测、密钥回显、CSV公式转义、参考参数绑定、参考文件前置验证及流中超时。
