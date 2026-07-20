# Grok Build / xAI Responses 与 OpenAI Responses 兼容性说明

本文用于解释近期 Grok Build / xAI Responses 接入中出现的 `serialization error`、未知事件、缺失字段、工具参数格式错误等问题，并给出基于官方文档的兼容边界判断。

## 结论摘要

xAI 官方提供 `POST /v1/responses`，并在示例中使用 OpenAI SDK 通过 `base_url="https://api.x.ai/v1"` 调用 `client.responses.create(...)`。这说明 xAI 的 Responses 在基础调用层面兼容 OpenAI SDK，但不能等价理解为“xAI/Grok Build 发出的所有事件和字段都能被 OpenAI/Codex 严格客户端完整接受”，也不能等价理解为“OpenAI Responses 的所有工具类型、事件类型和对象变体都能被 xAI/Grok Build 接收”。参考 xAI 官方 Generate Text 文档和 `llms.txt`：  
<https://docs.x.ai/developers/model-capabilities/text/generate-text>  
<https://docs.x.ai/llms.txt>

OpenAI 官方迁移文档明确说明，Responses 流式输出不是 Chat Completions 的 `delta` chunk，而是按 `type` 分派的 typed SSE events；消费者需要按事件 `type` 分支处理。参考 OpenAI 迁移文档：  
<https://developers.openai.com/api/docs/guides/migrate-to-responses#7-update-streaming-consumers>

因此，严格客户端通常会把流式事件反序列化为一个封闭的枚举集合。只要上游返回 `codex.rate_limits`、`response.metadata` 这类不在客户端事件枚举内的事件，就可能直接报 `unknown variant ...`。这不是 HTTP 状态码层面的失败，200 成功响应里也可以包含客户端无法反序列化的 SSE 事件。

OpenAI Responses 对若干对象字段有硬性结构要求。官方 OpenAPI 示例中，`Response` 顶层有 `id`，输出消息 `item` 有 `id`，`output_text` 内容块有 `annotations: []`。缺少这些字段时，使用强类型 SDK 或 Codex 客户端时会触发类似 `missing field id`、`missing field annotations` 的序列化错误。参考 OpenAI Responses 创建接口和流式事件参考：  
<https://developers.openai.com/docs/api-reference/responses/create>  
<https://developers.openai.com/api/reference/resources/responses/streaming-events>

## 错误根因对照

| 报错 | 直接原因 | 官方协议依据 | 处理建议 |
| --- | --- | --- | --- |
| `unknown variant codex.rate_limits` | 上游 SSE 中混入 Codex 私有/旁路事件。它不是 OpenAI Responses 官方流式事件。 | OpenAI 文档要求 Responses stream consumer 按 `type` 分派；官方事件参考列出的是 `response.created`、`response.output_text.delta`、`response.completed`、`error` 等 Responses 事件。 | 对 Grok Build / 严格客户端链路做事件白名单剪枝，丢弃不可识别旁路事件。 |
| `unknown variant response.metadata` | `metadata` 是 OpenAI Response 对象字段，不是官方 Responses stream event type。`event: response.metadata` 会被严格事件枚举拒绝。 | OpenAI 创建响应示例包含 `metadata: {}` 字段；流式事件参考中不把 `response.metadata` 作为事件类型。 | 不要把对象字段拆成同名 SSE event。若上游发出，转发前剪枝或改写为客户端能接受的事件结构。 |
| `missing field id` | 上游或转换层构造的 `response` / `output item` 缺少 `id`。 | OpenAI 流式 `response.created` schema 中 `response.id` 是非 optional；官方 streaming 示例中 `response.output_item.added` 的 `item` 也包含 `id`。 | 从 Chat Completions 或非标准流转换到 Responses 时，必须生成稳定 `resp_...` / `msg_...` / `fc_...` id。只在语义明确时补，避免伪造会话关系。 |
| `missing field annotations` | `output_text` 内容块缺少 `annotations` 数组。 | OpenAI Responses 创建示例和 streaming 示例中 `output_text` 都包含 `annotations: []`；内容块没有引用时也应是空数组。 | 构造 `output_text`、`content_part.added`、`content_part.done`、最终 `message.content[]` 时统一补 `annotations: []`。 |
| `unknown variant namespace, expected one of function, web_search, x_search, ...` | 请求侧把不被上游 Responses `tools[].type` 接受的字段或变体透传给 Grok/xAI/Grok Build。`namespace` 不是 xAI 示例中的工具类型。 | xAI 文档示例中 Responses 工具使用 `web_search`、`x_search`、`mcp` 等；OpenAI Responses 函数工具请求示例使用 `type: "function"`、`name`、`parameters`，不是 `type: "namespace"`。 | 出站到 Grok/xAI/Grok Build 时按目标官方支持工具类型白名单裁剪字段。OpenAI/Codex 内部扩展字段不能原样透传。 |
| `assistant.tool_calls 缺少有效 id、name 或 arguments` | Chat Completions 兼容层输出的工具调用缺少 OpenAI Chat 工具调用所需的基本结构。 | OpenAI Chat/Responses 的函数调用示例都包含调用标识、函数名和参数。Responses 函数输出示例包含 `id`、`call_id`、`name`、`arguments`。 | 从 Responses 转 Chat Completions 时，要生成 `tool_calls[].id`、`function.name`、`function.arguments`，并保持 arguments 形态符合目标端点要求。 |
| `expected JSON object for tool arguments` | 该错误来自具体 Grok 兼容网关/Completion 接口。它说明该上游在该路径要求工具参数是 JSON object，而不是已字符串化 JSON。 | OpenAI Responses 函数输出示例里 `arguments` 是 JSON 字符串；不同“OpenAI-compatible”网关可能对 Chat Completions 兼容层做了不同约束。 | 不要全局改 arguments 类型。按目标 provider/endpoint 做转换：OpenAI 风格保留字符串；该 Grok completion 网关路径按其要求转为 object。 |
| `invalid_encrypted_content` | 跨 provider 或跨渠道复用/裁剪了加密推理内容，导致接收方无法解密或校验。 | xAI 文档说明 `encrypted_content` 是 opaque，不要解析或修改，只能原样传回 xAI API；OpenAI 也会校验加密内容。 | `encrypted_content` 只能在同一 provider/同一兼容上下文中原样回传。跨 Grok/OpenAI 渠道时移除或中断复用链路。 |

## OpenAI Responses 的关键协议点

OpenAI Responses 的 `stream=true` 返回 `text/event-stream`，事件是 typed SSE event。迁移文档建议消费者按事件 `type` 分派，而不是按 Chat Completions 的 `delta` chunk 处理。参考：  
<https://developers.openai.com/api/docs/guides/migrate-to-responses#7-update-streaming-consumers>

OpenAI Responses 创建接口的 200 响应可以是 JSON `Response`，也可以是 `text/event-stream` 的 `ResponseStreamEvent`。官方示例包含：

- `event: response.created`，其中 `response.id` 存在。
- `event: response.output_item.added`，其中 `item.id` 存在。
- `event: response.content_part.added`，其中 `part.type = "output_text"` 且 `part.annotations = []`。
- `event: response.output_text.delta` / `response.output_text.done`。
- `event: response.content_part.done`，其中最终 `part.annotations = []`。
- `event: response.completed`，其中最终 `response.id` 存在。

参考 OpenAI Responses 创建接口和流式事件参考：  
<https://developers.openai.com/docs/api-reference/responses/create>  
<https://developers.openai.com/api/reference/resources/responses/streaming-events>

这解释了为什么客户端在 HTTP 200 的情况下仍然报错：协议层响应成功，但客户端反序列化阶段发现事件类型或必需字段不满足它编译时绑定的 schema。

## xAI / Grok Responses 的关键协议点

xAI 官方 Generate Text 文档展示了以下基础兼容能力：

- OpenAI SDK 可通过 `base_url="https://api.x.ai/v1"` 调 xAI。
- 可调用 `client.responses.create(model="grok-4.5", input=...)`。
- REST endpoint 是 `POST https://api.x.ai/v1/responses`。

参考：  
<https://docs.x.ai/developers/model-capabilities/text/generate-text>  
<https://docs.x.ai/llms.txt>

xAI 也有自己的扩展和约束：

- `usage.cost_in_usd_ticks` 是 xAI 在每个 inference response 的 `usage` 中返回的计费字段。参考 xAI `llms.txt` 中 Cost 章节：  
  <https://docs.x.ai/llms.txt>
- `encrypted_content` 是 opaque blob。xAI 官方要求不要解析、编辑或手工合并，只能原样保存并传回 xAI API；该内容只在传回 xAI API 时有意义。参考：  
  <https://docs.x.ai/developers/model-capabilities/text/reasoning#encrypted-reasoning-content>
- xAI 文档和配置示例把 `api_backend` 分为 `chat_completions`、`responses`、`messages`，说明不同后端协议不是同一个对象模型。参考：  
  <https://docs.x.ai/llms.txt>

## 重叠部分

| 能力 | OpenAI Responses | xAI Responses | 互通判断 |
| --- | --- | --- | --- |
| 基本文本输入输出 | `model` + `input`，返回 `Response` / `output_text`。 | xAI 示例同样使用 `model` + `input` 和 `client.responses.create(...)`。 | 可按重叠子集兼容。转换层仍要补齐 OpenAI 必需字段。 |
| `/v1/responses` endpoint | 官方主端点。 | xAI 官方也提供 `/v1/responses`。 | 端点名兼容，但 schema 细节不能默认全量兼容。 |
| OpenAI SDK 调用方式 | 官方 SDK 原生支持。 | xAI 示例使用 OpenAI SDK + xAI `base_url`。 | SDK 调用入口兼容。严格事件/字段仍要按目标处理。 |
| 文本流式事件 | OpenAI 使用 typed SSE events，如 `response.output_text.delta`。 | xAI/Grok 兼容实现通常复用 Responses 流式概念。 | 基本文本事件可兼容；旁路事件必须剪枝。 |
| 函数/工具调用 | OpenAI Responses 支持 `tools: [{ type: "function", ... }]` 和函数调用输出。 | xAI 文档展示 `web_search`、`x_search`、`mcp` 等工具。 | 只能按目标支持的工具类型白名单转换。 |

## OpenAI 独有或更完整的部分

OpenAI Responses 官方事件集合较大，包含文本、拒绝、函数调用参数、文件搜索、Web 搜索、reasoning summary/text、图像生成、MCP、code interpreter、annotation、custom tool call input、audio 等多类事件。参考 OpenAI 流式事件参考：  
<https://developers.openai.com/api/reference/resources/responses/streaming-events>

这些事件不能默认被 xAI/Grok Build 或第三方 Grok 网关完整接受。特别是以下情况需要裁剪：

- OpenAI/Codex 内部或新版本事件，目标端 SDK/网关枚举未更新。
- OpenAI 内置工具事件，如 MCP、code interpreter、image generation、custom tool input 等，目标端未声明支持。
- 对象内部字段被错误提升成事件类型，例如 `response.metadata`。

## xAI 独有或扩展部分

xAI 官方文档明确存在一些 xAI 自己的扩展字段和行为：

| xAI 特性 | 官方依据 | 对 OpenAI/Codex 严格客户端的兼容判断 |
| --- | --- | --- |
| `usage.cost_in_usd_ticks` | xAI `llms.txt` 说明每个 inference response 的 `usage` 都包含该字段。 | 宽松 JSON 客户端通常可忽略；强类型客户端如果不允许额外字段，可能失败。跨 OpenAI 链路可删除或放入本地统计，不应当作为 OpenAI 官方字段依赖。 |
| `encrypted_content` opaque blob | xAI reasoning 文档说明不要解析或修改，只能原样传回 xAI API。 | 不可移植到 OpenAI 或另一个不共享加密上下文的 provider。跨 provider 应移除，避免 `invalid_encrypted_content`。 |
| `api_backend = responses / chat_completions / messages` | xAI 配置文档区分不同后端协议。 | 转换层必须知道当前目标协议，不能把 Responses item 原样塞入 Chat Completions messages。 |
| `web_search` / `x_search` / xAI 工具形态 | xAI Responses 示例展示这些工具类型。 | OpenAI 未必接受 xAI 私有工具类型；发往 OpenAI 时按 OpenAI 官方工具类型过滤。 |

## 互通性判断

| 数据/事件 | OpenAI -> xAI/Grok | xAI/Grok -> OpenAI/Codex | 处理方式 |
| --- | --- | --- | --- |
| 基本文本 Responses 请求 | 通常可行。 | 通常可行。 | 保留 `model`、`input`、`stream` 等基础字段。 |
| OpenAI 完整 Responses SSE 事件集合 | 不保证。 | 不适用。 | 只发送目标声明支持的事件。 |
| Grok/Codex 旁路事件，如 `codex.rate_limits` | 不适用。 | 不兼容严格 OpenAI/Codex event enum。 | 入站剪枝。 |
| `response.metadata` 作为对象字段 | OpenAI Response 对象支持。 | 对象字段可保留。 | 不得作为 SSE event type 转发。 |
| `response.metadata` 作为 SSE event | 非 OpenAI 官方事件。 | 不兼容严格客户端。 | 剪枝。 |
| `output_text.annotations` | OpenAI 期望存在数组。 | 缺失会导致严格客户端失败。 | 缺失时补 `[]`。 |
| `response.id` / `item.id` | OpenAI 期望存在。 | 缺失会导致严格客户端失败。 | 转换时生成稳定 id。 |
| xAI `cost_in_usd_ticks` | OpenAI 不需要。 | 强类型 OpenAI 客户端可能不接受。 | 面向 OpenAI/Codex 时可删除或仅本地记录。 |
| `encrypted_content` | 不应跨 provider 修改/复用。 | 不应跨 provider 修改/复用。 | 同 provider 原样回传；跨 provider 删除或断链。 |

## AllApiDeck 实现建议

1. 协议转换必须同时区分 `app`、`route`、`provider`、`endpoint`。`responses` 和 `chat/completions` 不能共用一套字段透传策略。
2. 对 `app=grokbuild` 且 `route=responses` 的流式响应做事件白名单，只保留严格客户端支持的 Responses SSE events，丢弃 `codex.rate_limits`、`response.metadata` 等旁路事件。
3. 从 Chat Completions 或第三方非标准响应构造 OpenAI Responses 时，统一补齐：
   - `response.id`
   - `output item.id`
   - `output_text.annotations: []`
   - 必要的 `status`、`role`、`type`
4. 发往 Grok/xAI/Grok Build 的 `tools` 需要按目标官方支持的类型白名单裁剪。不要把 OpenAI/Codex 内部扩展字段，例如 `namespace`，原样传给不支持该字段的上游。
5. `arguments` 的字符串/object 形态要按 endpoint 区分。OpenAI Responses 函数调用输出示例中 `arguments` 是字符串；但已有第三方 Grok completion 网关报错要求 JSON object，因此只能为该 provider/endpoint 做定向转换。
6. `encrypted_content` 不做解析、合并、改写。跨 provider 或跨渠道切换时删除该字段，避免解密校验失败。
7. 对 HTTP 200 + SSE 内部 `response.failed` / `error` 的情况，不能只看 HTTP status。需要解析 SSE 中的失败事件并进入已有“愈合/重试/回退”逻辑。

## 参考链接

- OpenAI Responses 迁移指南，streaming consumer 需要按事件 `type` 分派：<https://developers.openai.com/api/docs/guides/migrate-to-responses#7-update-streaming-consumers>
- OpenAI Responses streaming events reference：<https://developers.openai.com/api/reference/resources/responses/streaming-events>
- OpenAI Responses create API reference：<https://developers.openai.com/docs/api-reference/responses/create>
- OpenAI Responses guide：<https://developers.openai.com/docs/guides/responses>
- xAI Generate Text / Responses API：<https://docs.x.ai/developers/model-capabilities/text/generate-text>
- xAI reasoning / encrypted reasoning content：<https://docs.x.ai/developers/model-capabilities/text/reasoning#encrypted-reasoning-content>
- xAI `llms.txt`，包含 Responses、OpenAI SDK 示例、`cost_in_usd_ticks`、`encrypted_content`、`api_backend` 等官方摘录：<https://docs.x.ai/llms.txt>
