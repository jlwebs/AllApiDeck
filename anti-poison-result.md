# 防投毒功能测试报告

测试范围：高级代理防投毒策略、字符串保护、阻断反馈、统计流水、上游协议兼容性、流式响应处理和桌面构建。

## 结论摘要

防投毒开启后，系统可以在模型返回真实 toolcall 时要求前置 guard JSON 文本块，并由网关校验 `name` / `tool_name` 与真实工具调用的最小绑定关系。若检测到缺失、错配或响应结构异常，会按配置阻断或告警，并在请求详情中记录 blocked 流水。

字符串保护功能可在 request out 阶段把 JSON 密钥字段值、token/secret 类文本、私钥块、敏感工具结果和用户主动 `<<...>>` 标记内容替换为占位符，并在 respond in 阶段还原。普通 `.env`、`settings.json` 等文件名 mention 保持原文，避免工具说明和系统提示被无意义替换。

## 能力对比

| 场景 | 未开启防投毒 | 开启防投毒后 |
|---|---|---|
| 上游响应被注入额外 toolcall | 客户端可能直接接收并继续执行 | 网关要求每个真实 toolcall 有可匹配 guard JSON |
| 攻击者只返回真实工具调用 | 难以区分是否为模型正常输出 | 缺少前置 guard JSON 会触发 `missing_guard_toolcall` |
| 攻击者伪造 guard | 可能混入响应文本 | `name` / `tool_name` 必须匹配本轮命名规则和真实工具名 |
| Hosted web search 输出 | 容易被误判为普通 function call | `web_search_call` 归类为 hosted tool call，不强制普通 function guard |
| 密钥值进入上游 | 可能被模型或注入文本读取并外泄 | request out 替换占位符，respond in 再还原 |
| 用户主动标记机密 | 没有手动保护通道 | `<<passw0rd>>` 这类 `user_text:` 内容会被保护并还原 |
| 被阻断后的可观测性 | 只能看到请求失败 | 记录 before/after、context、channel、route、reason 和工具调用归类 |

## 测试覆盖

| 测试项 | 覆盖内容 | 结果 |
|---|---|---|
| 本地单元测试 | guard prompt、toolcall 校验、字符串保护规则、还原逻辑 | 通过 |
| 字符串保护策略 | JSON key value、token、敏感工具结果、用户 `<<...>>` 标记、普通文件名 mention 不保护 | 通过 |
| 流式处理 | split guard JSON 剥离、Responses function call 生命周期聚合、hosted tool call 归类 | 通过 |
| Hosted Web Search | OpenAI Responses `web_search_call` 不误杀 | 通过 |
| 工具参数规范化 | `pages:""` 等可选空字段清理 | 通过 |
| 阻断链路 | 防投毒 blocked 不走普通 fallback | 通过 |
| UI 构建 | 防投毒面板、详情弹窗、规则配置、请求记录 | 通过 |
| Release Desktop | Windows EXE/MSI、macOS DMG、Linux tar.gz/deb/AppImage | 通过 |

## 攻击/投毒手段覆盖

| 攻击/投毒类别 | 典型手段 | 主要风险 | 防投毒检测点 | 测试结果 |
|---|---|---|---|---|
| 缺失 guard 的真实 toolcall 注入 | 上游响应中直接插入 `shell_command`、文件读取、HTTP 请求等真实 toolcall | 客户端可能继续执行未授权工具调用 | 真实 toolcall 数量大于 0，但 guard JSON 不足 | 阻断，返回客户端可见错误 |
| guard 绑定错误 | guard JSON 的 `tool_name` 与真实工具名不一致 | 攻击者伪造看似合法的 guard 文本 | `name` / `tool_name` 最小绑定校验失败 | 阻断或告警 |
| 额外真实 toolcall | 在正常响应后追加一条恶意工具调用 | 网关若只看文本可能放行链路追加攻击 | 每个真实 toolcall 都需要可匹配 guard JSON | 阻断 |
| 协议形态混淆 | 在 OpenAI Chat、OpenAI Responses、Claude Messages 等不同响应结构里塞入 toolcall | 多协议解析不一致导致漏检 | 分协议解析 toolcall 和 guard JSON，再统一校验 | 已覆盖 |
| Hosted tool 误杀 | OpenAI Responses `web_search_call` 被当成普通 function call | 正常 Web Search 响应被误拦 | hosted tool call 分类跳过普通 function guard 要求 | 已修复并验证 |
| JSON 密钥字段泄露 | JSON 中包含 `api_key`、`secret`、`token`、`authorization`、`password` 等字段值 | 上游模型看到明文密钥后可能泄露或被注入利用 | key/path/text 规则替换为 `__AAD_STR_...__` 占位符 | 已替换，respond in 还原 |
| 用户主动标记机密 | 用户输入 `<<passw0rd>>`、`<<my-token>>` | 用户已明确知道该片段应保护 | `user_text:` 规则只在用户输入文本中保护 | 已替换，respond in 还原 |
| 敏感工具结果泄露 | Read 工具返回 `.env` 文件内容或私钥块 | 文件内容包含真实 token 或私密配置 | 工具结果看起来像真实敏感内容时整体保护 | 已替换并记录上下文 |
| 响应阶段污染回流 | 上游把占位符、guard JSON 或异常内容带回客户端 | 客户端看到内部防护细节或错误内容 | respond in 阶段剥离 guard、还原字符串、记录 blocked | 正常还原或阻断 |
| 阻断后 fallback 绕过 | 检测到投毒后继续 fallback 到其他 provider | 安全失败被当成普通上游失败处理 | anti-poison blocked 标记硬终止本次请求 | 已验证不 fallback |

## 协议矩阵

| 上游/客户端协议形态 | 覆盖重点 | 结果 |
|---|---|---|
| OpenAI Chat Completions | `choices[].message.tool_calls`、兼容 `function_call`、assistant 文本 guard JSON | 通过 |
| OpenAI Responses | `output[].type=function_call`、`instructions` 注入、hosted `web_search_call` 分类 | 通过 |
| OpenAI Responses Stream | split guard JSON、function call 生命周期聚合、guard 剥离 | 通过 |
| Claude Messages | `content[].type=tool_use`、system 注入、text block guard JSON | 通过 |
| Claude 兼容 OpenAI Chat | Claude 客户端请求经 OpenAI Chat 上游转换 | 通过 |
| Claude 兼容 OpenAI Responses | Claude 客户端请求经 Responses 上游转换 | 通过 |

阻断后的行为要求包括：返回客户端可见错误，错误信息包含防投毒校验失败原因，不继续 fallback 到其他 provider，统计流水中写入 blocked=true 记录，并在详情面板中展示上下文。

## 真实链路和构建验证

| 验证项 | 结果 |
|---|---|
| targeted Go tests | 通过 |
| 本地 `npm run build:desktop` | 通过 |
| GitHub Actions `Release Desktop` | 通过 |
| Windows EXE/MSI assets | 已上传 |
| macOS DMG asset | 已上传 |
| Linux tar.gz/deb/AppImage assets | 已上传 |

最近验证命令：

```powershell
cd desktop
go test . -run 'TestAntiPoisonStringProtection|TestParseToolInputMapDropsOptionalEmptyPaginationFields|TestSanitizeAntiPoisonOpenAIResponsesStreamBodyStripsSplitGuardJSON|TestApplyAntiPoisonPromptTo.*' -count=1
npm run build:desktop
```

## 历史评估说明

v0.5.6 beta 期间保留过本地投毒 demo 半小时评估，用于验证多协议、流式、多轮和防泄漏链路。v0.6.0 已将正式说明收敛到当前 guard JSON 和字符串保护实现；历史流水可作为 beta 研发过程参考，但当前 release 判断以本文档和当前单测/构建结果为准。

## 风险边界

该方案不能解决“模型本身已恶意且客户端绕过网关”这种根本性问题，也不能只依赖 Prompt 作为最终裁判。真正的安全边界在网关层：模型只负责按动态规则生成可校验材料，最终是否放行必须由网关校验真实 toolcall、guard JSON、字符串保护记录和响应结构。

当前测试结果表明：对非定向大规模投毒、响应链路 toolcall 伪造、guard 缺失/错配、敏感字符串暴露等场景，防投毒机制已经具备可验证的阻断、还原和审计能力。
