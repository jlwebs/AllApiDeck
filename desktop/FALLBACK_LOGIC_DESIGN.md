# 高级代理回落（Fallback）逻辑设计文档

## 概述

AllApiDeck 的高级代理实现了一个智能的多协议回落机制，能够在不同的 API 格式之间自动切换，以确保请求的成功率和最佳兼容性。

## 核心概念

### 1. 协议相位（Protocol Phase）

系统使用"相位"（Phase）的概念来管理协议尝试的顺序：

#### Claude 代理相位结构
```go
type claudeProxyAttemptPhase struct {
    apiFormat          string  // "anthropic", "openai_responses", "openai_chat"
    routeKind          string  // "messages", "responses", "chat"
    source             string  // "preference", "fallback", "original"
    preferenceValue    int     // 用户偏好优先级
    preferenceScopeKey string  // 持久化偏好的作用域键
}
```

#### OpenAI 代理相位结构
```go
type openAIProxyAttemptPhase struct {
    outboundRoute      string  // "chat", "responses", "messages"
    requestBody        []byte  // 转换后的请求体
    resolvedModel      string  // 解析后的模型名
    responseTransform  string  // 响应转换标识
    hostedWebSearch    bool    // 是否使用托管搜索
    preferenceValue    int     // 协议偏好值
    preferenceScopeKey string  // 偏好作用域
    source             string  // 相位来源
}
```

### 2. 协议格式

系统支持三种主要协议格式：

1. **Anthropic Messages API** (`anthropic`)
   - 标准的 Claude Messages API 格式
   - 端点: `/v1/messages`

2. **OpenAI Responses API** (`openai_responses`)
   - OpenAI 的 Responses API 格式
   - 端点: `/v1/responses`
   - 支持更复杂的对话结构和工具调用

3. **OpenAI Chat Completions API** (`openai_chat`)
   - 标准的 OpenAI Chat API 格式
   - 端点: `/v1/chat/completions`

## 回落决策逻辑

### 1. Responses → Chat 回落条件

函数: `shouldFallbackResponsesToChat()`

触发条件：
- **HTTP 状态码**: 404 (Not Found), 405 (Method Not Allowed)
- **错误消息匹配**:
  - "unknown api route" - API 路由未知
  - "does not support selected model" - 模型不支持
  - "field messages is required" - 缺少 messages 字段
  - "invalid json" - JSON 格式错误
  - "failed to deserialize" + "tools" - 工具反序列化失败
  - "missing field" + "tools" - 缺少工具字段
  - "(html)" - 返回了 HTML 响应
  - "unsupported" + "route" - 不支持的路由
  - "not implemented" - 未实现

```go
func shouldFallbackResponsesToChat(statusCode int, responseBody []byte) bool {
    if statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed {
        return true
    }
    message := strings.ToLower(strings.TrimSpace(firstNonEmpty(
        summarizeAdvancedProxyBody(responseBody), 
        fmt.Sprintf("http %d", statusCode)
    )))
    // 检查各种错误模式...
}
```

### 2. Chat → Responses 回落条件

函数: `shouldFallbackChatPreferenceBackToResponses()`

触发条件（更保守）：
- **HTTP 状态码**: 404, 405
- **错误消息匹配**:
  - "unknown api route"
  - "unsupported"
  - "not implemented"

这个条件比 Responses → Chat 更严格，因为它是"恢复"操作。

### 3. 成功响应的回落

函数: `shouldFallbackSuccessfulResponsesToChat()`

特殊场景：即使 HTTP 状态码是 2xx，但响应体包含错误信息，仍然触发回落。

```go
func shouldFallbackSuccessfulResponsesToChat(statusCode int, responseBody []byte) bool {
    if statusCode < 200 || statusCode >= 300 || !hasOpenAIErrorEnvelope(responseBody) {
        return false
    }
    return shouldFallbackResponsesToChat(statusCode, responseBody)
}
```

## 相位构建策略

### Claude 代理的相位构建

函数: `buildClaudeProxyAttemptPhases()`

#### 情况 1: 使用 Web Search 工具

当请求包含 Anthropic Web Search 工具时：

```
如果有用户偏好 Responses:
  1. openai_responses (preference)
  2. anthropic (fallback_restore)

否则（默认）:
  1. anthropic (original)
  2. openai_responses (fallback)
```

原因：Web Search 需要特定的协议支持，选项有限。

#### 情况 2: 有用户偏好

根据用户存储的协议偏好构建相位：

**偏好 Responses**:
```
1. openai_responses (preference)
2. anthropic (fallback_restore)
3. openai_chat (fallback_restore)
```

**偏好 Chat**:
```
1. openai_chat (preference)
2. anthropic (fallback_restore)
3. openai_responses (fallback_restore)
```

**偏好 Anthropic** (默认):
```
1. anthropic (preference)
2. openai_responses (fallback_restore)
3. openai_chat (fallback_restore)
```

#### 情况 3: 提供商配置指定

根据 `provider.APIFormat` 配置：

**配置为 openai_responses**:
```
1. openai_responses (provider_config)
2. anthropic (fallback)
3. openai_chat (fallback_secondary)
```

**配置为 openai_chat**:
```
1. openai_chat (provider_config)
2. anthropic (fallback)
3. openai_responses (fallback_secondary)
```

#### 情况 4: 默认策略

没有任何配置时的默认顺序：
```
1. anthropic (original)
2. openai_responses (fallback)
3. openai_chat (fallback_secondary)
```

### OpenAI 代理的相位构建

对于 OpenAI 代理（入站为 Responses API）：

```go
// 主要相位
appendResponsesPhase("original", preferenceValue, scopeKey)

// 如果支持，添加 Chat 回落
if fallbackPlan.SupportsChat {
    appendChatPhase("fallback", preferenceValue, scopeKey)
}

// 如果支持 Messages API（Anthropic），添加 Messages 回落
appendMessagesPhase("fallback_secondary")
```

## 协议转换

### 1. Anthropic → OpenAI Responses

函数: `anthropicRequestToOpenAIResponses()`

主要转换：
- `messages` → `input` (消息数组变为输入项)
- `system` → `instructions` (系统提示变为指令)
- `tools` → `tools` (工具定义转换)
- `tool_choice` → `tool_choice` (工具选择转换，已修复格式)
- `thinking` → `reasoning` (思考模式转换)
- `max_tokens` → `max_output_tokens`

### 2. Anthropic → OpenAI Chat

函数: `anthropicRequestToOpenAIChat()`

主要转换：
- `messages` 保持结构
- `system` 可以作为系统消息
- `tools` 结构相似
- `tool_choice` 转换（已修复格式）
- `max_tokens` 字段名保持

### 3. Responses → Chat

函数: `buildOpenAIChatFallbackPlanFromResponses()`

生成回落计划，评估：
- 是否支持 Chat API
- 存在的阻塞因素（Blockers）
- 是否使用托管 Web Search

主要转换：
- `input` → `messages`
- `instructions` → system message
- `max_output_tokens` → `max_tokens`
- `reasoning` → `reasoning_effort`

**已修复的 tool_choice 转换**:
```go
// 正确处理嵌套格式
functionMap, _ := typed["function"].(map[string]any)
if functionMap != nil {
    name = functionMap["name"]
}
// 同时兼容旧版扁平格式
if name == "" {
    name = typed["name"]  // 向后兼容
}
```

## 协议偏好持久化

### 偏好存储

位置: `~/.allapi-deck/advanced-proxy/claude-protocol-preferences.json`

格式:
```json
{
  "host=api.example.com&key=abc123&model=claude-3-5-sonnet&claude_api_format=": 1,
  "host=api.other.com&key=def456&model=gpt-4&claude_api_format=openai_responses": 2
}
```

值的含义:
- `0` = Anthropic Messages API
- `1` = OpenAI Responses API
- `2` = OpenAI Chat Completions API

### 偏好作用域键（Scope Key）

```go
func resolveAdvancedProxyClaudeProtocolPreferenceScopeKey(
    provider AdvancedProxyProvider, 
    model string
) string {
    // 格式: host={host}&key={key_hash}&model={model}&claude_api_format={format}
}
```

作用域包含：
- 主机（BaseURL 的主机名）
- API 密钥的 SHA256 哈希
- 模型名称
- Claude API 格式配置

### 偏好学习机制

当某个协议成功后，系统会：
1. 记录成功的协议格式
2. 为该作用域更新偏好
3. 下次请求直接使用成功的协议
4. 保持其他协议作为回落选项

## 执行流程

### 主流程

```
1. 构建相位列表 (buildClaudeProxyAttemptPhases / OpenAI phases)
   ↓
2. 对每个相位按顺序尝试:
   ↓
   2.1 转换请求体到目标协议
   ↓
   2.2 发送请求到上游
   ↓
   2.3 检查响应
   ↓
   2.4 如果成功: 返回响应
   ↓
   2.5 如果失败: 
       - 评估是否应该回落 (shouldFallback*)
       - 如果应该回落: 继续下一个相位
       - 如果不应该回落: 返回错误
   ↓
3. 所有相位失败: 返回最后的错误
```

### 日志记录

系统在每个关键点记录日志：

```
[OPENAI_PROXY_FALLBACK_PLAN] - 回落计划生成
[OPENAI_PROXY_FALLBACK] - 触发回落
[OPENAI_PROXY_CHAT_RESTORE] - 恢复到 Chat
[CLAUDE_PROXY_PHASE] - Claude 代理相位执行
[CLAUDE_PROXY_FALLBACK] - Claude 回落
```

## 特殊处理

### 1. 加密内容修复（Encrypted Content Healing）

针对 Anthropic API 的加密内容验证失败：
- 检测 `encrypted_content` 错误
- 记录会话的加密内容计数
- 自动移除历史加密内容
- 重试请求

### 2. 工具调用顺序标准化

OpenAI Responses API 对工具调用顺序有要求：
- 自动重排序工具调用历史
- 确保输入/输出配对正确

### 3. Input Item ID 分配

Responses API 需要每个输入项有唯一 ID：
- 自动为缺失 ID 的项分配
- 使用稳定的生成算法

### 4. 字符串保护（Anti-Poison）

防止上游响应中的恶意内容：
- 临时替换敏感模式
- 处理响应后恢复
- 记录保护操作

## 错误处理

### 可重试错误

- 5xx 服务器错误
- 超时错误
- 网络错误

### 不可重试错误

- 4xx 客户端错误（除了 404, 405）
- 认证失败
- 配额超限

### 回落决策

回落决策基于：
1. HTTP 状态码
2. 响应体的错误消息
3. 响应内容类型（防止 HTML 响应）
4. 协议兼容性

## 性能优化

### 1. 偏好缓存

- 内存中缓存协议偏好
- 避免重复读取文件
- 线程安全的访问

### 2. 请求体预转换

- 在相位构建时预转换所有协议格式
- 避免运行时转换开销

### 3. 短路机制

- 成功后立即返回，不尝试剩余相位
- 不可回落错误立即失败

## 监控与可观测性

### 记录的指标

1. **协议尝试**: 每个相位的尝试次数
2. **回落率**: 触发回落的频率
3. **成功率**: 各协议的成功率
4. **延迟**: 每个协议的响应时间

### 日志追踪

每个请求记录：
- 路由追踪（Route Trace）
- 相位序列
- 回落原因
- 最终结果

## 最佳实践

### 提供商配置

1. **显式配置 APIFormat**: 如果已知提供商支持特定格式，配置 `provider.APIFormat`
2. **设置合理的超时**: 不同协议可能有不同的响应时间
3. **启用调试日志**: 在测试阶段启用详细日志

### 错误诊断

1. 检查日志中的 `[OPENAI_PROXY_FALLBACK]` 或 `[CLAUDE_PROXY_FALLBACK]`
2. 查看 `reason=` 字段了解回落原因
3. 检查 Route Trace 了解完整的尝试路径

### 协议选择建议

- **Claude 模型**: 优先 `anthropic`
- **OpenAI 模型**: 优先 `openai_chat`
- **通用代理**: 使用 `openai_responses` 获得最佳兼容性
- **Web Search**: 必须使用 `anthropic` 或 `openai_responses`

## 未来改进方向

1. **动态协议探测**: 自动探测提供商支持的协议
2. **回落策略学习**: 基于历史数据优化回落顺序
3. **并行尝试**: 对可并行的协议同时发送请求
4. **协议能力协商**: 与上游协商最佳协议版本

---

最后更新: 2026-07-08
