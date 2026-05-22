# 防投毒功能测试报告

测试日期：2026-05-22  
测试范围：高级代理防投毒策略、字符串保护、阻断反馈、统计流水、上游协议兼容性、live soak 稳定性。

## 结论摘要

防投毒开启后，系统可以在模型返回真实 toolcall 时要求伴随动态 guard fake toolcall，并由网关校验 nonce、算法别名、调用链 digest。若检测到缺失、错配或伪造链路，会立即终止本次响应，返回客户端可见错误，并在统计表中记录 blocked 流水。

字符串保护功能可在 request out 阶段把 `.env`、`.npmrc`、配置 key、token/secret 类字段替换为占位符，并在 respond in 阶段还原，降低读取配置文件或 JSON 密钥字段时被注入内容诱导泄露的风险。

## 能力对比

| 场景 | 未开启防投毒 | 开启防投毒后 |
|---|---|---|
| 上游响应被注入额外 toolcall | 客户端可能直接接收并继续执行 | 网关校验 toolcall 链路，缺 guard 或 digest 不匹配则阻断 |
| 攻击者只改 tool 参数 | 难以发现参数被替换 | digest 覆盖 canonical arguments，参数变化会导致校验失败 |
| 攻击者伪造一条工具调用 | 可能混入真实链路 | 调用链摘要包含顺序、工具类型、call_id 摘要、参数 hash、nonce |
| 非定向大规模投毒 | 容易复用固定注入模板 | nonce、算法别名、策略句式、插入点随机变化，提升泛化攻击成本 |
| 配置/密钥类字符串进入上游 | 可能被模型或注入文本读取并外泄 | request out 替换占位符，respond in 再还原 |
| 被阻断后的可观测性 | 只能看到请求失败 | 记录 before/after、channel、route、reason，表格爆红提醒 |

## 测试覆盖

| 测试项 | 覆盖内容 | 结果 |
|---|---|---|
| 本地单元测试 | nonce/hash、guard toolcall 校验、字符串保护规则、还原逻辑 | 通过 |
| 本地 E2E 矩阵 | 5 种协议形态 x 5 类投毒注入场景 | 通过，均可阻断并返回客户端可见错误 |
| 阻断链路测试 | 检测到投毒后不 fallback 到其他 provider | 通过 |
| UI 构建 | 防投毒面板、统计表、规则配置、blocked 高亮 | 通过 |
| Live soak：OpenAI Responses | `gpt-5.5`，防投毒开启，30 分钟 | 50/50 成功，0 transient failure |
| Live soak：Claude Messages | `claude-sonnet-4-6`，防投毒开启，30 分钟 | 56/56 成功，0 transient failure |
| Live drill：OpenAI Responses Stream | `gpt-5.5`，防投毒开启，15 分钟 | 29/29 成功，0 transient failure |
| Live drill：Claude Messages Stream | `claude-sonnet-4-6`（`openai_chat` 上游），防投毒开启，15 分钟 | 35/35 成功，0 transient failure |

## 攻击/投毒手段覆盖

本地 E2E fake-upstream 矩阵覆盖了不同上游响应协议下的典型投毒方式。测试目标不是证明模型一定会执行恶意内容，而是证明当上游或响应链路出现恶意 toolcall、参数篡改、伪造 guard、配置字符串诱导等情况时，网关能够识别、阻断、记录并把错误反馈给客户端。

| 攻击/投毒类别 | 典型手段 | 主要风险 | 防投毒检测点 | 测试结果 |
|---|---|---|---|---|
| 缺失 guard 的真实 toolcall 注入 | 上游响应中直接插入 `shell_command`、文件读取、HTTP 请求等真实 toolcall，但不生成对应 guard fake toolcall | 客户端可能继续执行未授权工具调用 | 真实 toolcall 数量大于 0，但 guard fake toolcall 数量不足 | 阻断，返回客户端可见错误 |
| tool 参数定向篡改 | 保留工具名和 call_id，但把 `command`、`path`、`url`、`arguments` 等参数替换为恶意内容 | 只看工具名会漏检，执行目标被悄悄替换 | digest 覆盖 canonical arguments，参数变化导致 digest 不匹配 | 阻断，记录 `guard_digest_mismatch` |
| 伪造额外真实 toolcall | 在正常响应后追加一条恶意工具调用，尝试混入真实调用链 | 网关若只看单条调用，可能放行链路追加攻击 | 调用链 digest 包含顺序、工具类型、工具名、call_id 摘要、参数 hash | 阻断，调用链不一致 |
| 复制/重放旧 guard | 复用上一轮或其他请求中的 nonce、digest、guard fake toolcall | 非定向投毒可尝试模板化重放 | nonce、算法别名、guard tool name 每轮变化 | 阻断，nonce/digest 不匹配 |
| 伪造 guard 但摘要不完整 | 攻击者生成看似合法的 guard fake toolcall，但 chain、cover、digest 与真实 toolcall 不一致 | 欺骗只检查 guard 是否存在的实现 | 网关重新计算真实 toolcall 链路 digest 并对比 | 阻断 |
| 协议形态混淆 | 在 OpenAI Chat、OpenAI Responses、Claude Messages 等不同响应结构里塞入不同格式的 toolcall | 多协议解析不一致导致漏检 | 分协议解析 toolcall，再统一进入 guard 校验 | 5 种协议形态均通过 |
| 读取配置文件诱导 | 用户或注入内容诱导读取 `.env`、`.npmrc`、`.netrc`、`.gitconfig` 等文件 | 文件内容可能包含 token 或私密配置 | request out 阶段按规则替换敏感路径/字符串 | 已替换并记录 before/after |
| JSON 密钥字段泄露 | JSON 中包含 `api_key`、`secret`、`token`、`authorization`、`password` 等字段 | 上游模型看到明文密钥后可能泄露或被注入利用 | key/path 规则命中后替换为 `__AAD_STR_...__` 占位符 | 已替换，respond in 还原 |
| 响应阶段污染回流 | 上游把占位符、guard fake toolcall 或异常内容带回客户端 | 客户端看到内部防护细节或错误内容 | respond in 阶段 strip guard、还原字符串、blocked 记录 | 正常还原或阻断 |
| 阻断后 fallback 绕过 | 检测到投毒后继续 fallback 到其他 provider，间接绕过阻断 | 安全失败被当成普通上游失败处理 | anti-poison blocked 标记会硬终止本次请求 | 已验证不 fallback |

## E2E 协议矩阵

| 上游/客户端协议形态 | 覆盖重点 | 投毒样本数 | 结果 |
|---|---|---:|---|
| OpenAI Chat Completions | `choices[].message.tool_calls`、`function_call` 类注入 | 5 | 全部阻断 |
| OpenAI Responses | `output[].type=function_call` 类注入 | 5 | 全部阻断 |
| Claude Messages | `content[].type=tool_use` 类注入 | 5 | 全部阻断 |
| Claude 兼容 OpenAI Chat | Claude 客户端请求经 OpenAI Chat 上游转换 | 5 | 全部阻断 |
| Claude 兼容 OpenAI Responses | Claude 客户端请求经 Responses 上游转换 | 5 | 全部阻断 |

阻断后的行为要求包括：返回客户端可见 `502`，错误信息包含防投毒校验失败原因，不继续 fallback 到其他 provider，统计流水中写入 blocked=true 记录，并在 UI 表格中高亮。

## Live 稳定性结果

| 模型/协议 | 运行时长 | 成功次数 | 瞬时失败 | 结论 |
|---|---:|---:|---:|---|
| `gpt-5.5` / OpenAI Responses | 30 分钟 | 50 | 0 | 正常链路无回归 |
| `claude-sonnet-4-6` / Claude Messages | 30 分钟 | 56 | 0 | 兼容性通过 |
| `gpt-5.5` / OpenAI Responses Stream | 15 分钟 | 29 | 0 | 流式防投毒与响应剥离稳定 |
| `claude-sonnet-4-6` / Claude Messages Stream (`openai_chat`) | 15 分钟 | 35 | 0 | 流式转换链路稳定 |

Live soak 主要验证防投毒开启后的正常请求兼容性和稳定性。OpenAI Responses 组使用 `gpt-5.5` 连续运行 30 分钟，50 次请求全部成功，说明防投毒 prompt 注入、guard tool schema 注入、响应校验和字符串保护不会破坏正常 Responses 链路。

Claude Messages 组使用 `claude-sonnet-4-6` 连续运行 30 分钟，56 次请求全部成功。该链路覆盖 Claude 客户端协议到 OpenAI Chat 兼容上游的转换，包含 system prompt 兼容修正、guard tool schema 转换、响应转换和防投毒校验，结果显示兼容链路无稳定性回归。

新增 15 分钟流式真实演练：OpenAI Responses Stream 组 (`gpt-5.5`) 连续运行 29 次请求全部成功，说明流式场景下的 toolcall 增量聚合、guard 校验、guard 剥离与字符串保护还原未引入稳定性问题。Claude Messages Stream 组 (`claude-sonnet-4-6`，上游协议 `openai_chat`) 连续运行 35 次请求全部成功，说明 Claude 流式兼容链路在开启防投毒后依然稳定。

恶意注入阻断能力由本地 E2E fake-upstream 矩阵验证；live soak 不主动向真实上游注入恶意 toolcall，避免污染真实服务调用。

## 风险边界

该方案不能解决“模型本身已恶意”这种根本性问题，也不能只依赖 prompt 作为最终裁判。真正的安全边界在网关层：模型只负责按动态策略生成可校验材料，最终是否放行必须由网关校验 nonce、digest、调用链和字符串保护记录决定。

当前测试结果表明：对非定向大规模投毒、响应链路 toolcall 伪造、参数替换、配置字符串暴露等场景，防投毒机制已经具备可验证的阻断、回滚和审计能力。
