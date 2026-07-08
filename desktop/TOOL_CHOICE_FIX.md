# Tool Choice 协议转换修复文档

## 问题描述

在高级代理的 `responses → message` 协议转换过程中，出现以下报错：

```
app=codex | route=responses | provider=123nhh | endpoint=https://api.123nhh.com/v1/chat/completions | tool_choice: Input should be an object
```

## 根本原因

根据 OpenAI API 官方规范，当 `tool_choice` 指定具体函数时，应该使用以下标准格式：

```json
{
  "type": "function",
  "function": {
    "name": "function_name"
  }
}
```

但代码中存在两处不符合规范的实现：

### 1. `anthropicToolChoiceToResponses` 函数问题

**位置**: `desktop/advanced_proxy_runtime.go:1752-1755`

**错误代码**:
```go
return map[string]any{
    "type": "function",
    "name": strings.TrimSpace(toStringValue(choiceMap["name"])),  // 错误：name 在顶层
}
```

**正确代码**:
```go
return map[string]any{
    "type": "function",
    "function": map[string]any{
        "name": strings.TrimSpace(toStringValue(choiceMap["name"])),
    },
}
```

### 2. `convertResponsesRequestToolChoiceToChat` 函数问题

**位置**: `desktop/advanced_proxy_openai_fallback.go:970`

**问题**: 该函数只从对象的顶层读取 `name` 字段，无法正确处理嵌套在 `function` 对象内的标准格式。

**修复**: 优先从嵌套的 `function` 对象中读取 `name`，同时保留对非标准扁平格式的兼容性。

## 修复内容

### 修改 1: advanced_proxy_runtime.go

修复了 `anthropicToolChoiceToResponses` 函数，使其输出符合 OpenAI API 规范的嵌套格式。

### 修改 2: advanced_proxy_openai_fallback.go

改进了 `convertResponsesRequestToolChoiceToChat` 函数：
- 优先从标准的 `{"type": "function", "function": {"name": "..."}}` 格式读取
- 保留对旧版扁平格式 `{"type": "function", "name": "..."}` 的向后兼容
- 添加了详细注释说明预期格式

### 修改 3: 添加测试用例

在 `advanced_proxy_runtime_test.go` 中添加了 `TestAnthropicToolChoiceToResponsesReturnsCorrectFormat` 测试。

在 `advanced_proxy_openai_fallback_test.go` 中添加了 `TestConvertResponsesRequestToolChoiceToChat` 测试，覆盖以下场景：
- 字符串值（"required", "auto", "none"）的直接传递
- 标准嵌套格式的正确解析
- 旧版扁平格式的兼容处理
- 边界条件（缺失 name、错误 type 等）

## 测试验证

所有相关测试均通过：

```
✓ TestAnthropicToolChoiceToResponsesReturnsCorrectFormat
✓ TestConvertResponsesRequestToolChoiceToChat
✓ TestAnthropicRequestToOpenAIChatMapsImagesStopAndCleansSchema
✓ TestAnthropicRequestToOpenAIResponsesUsesInstructionsAndMapsImages
✓ TestOpenAIResponsesToAnthropicMapsWebSearchBlocksAndUsage
... (所有相关测试通过)
```

## 影响范围

此修复影响以下协议转换场景：
1. Anthropic Messages API → OpenAI Responses API
2. OpenAI Responses API → OpenAI Chat Completions API

特别是当客户端使用 `tool_choice` 参数指定具体函数调用时，现在会生成符合 OpenAI 规范的请求格式。

## 参考文档

- [OpenAI API - Tool Choice Format](https://platform.openai.com/docs/api-reference/chat/create#chat-create-tool_choice)
- [OpenAI API - Responses](https://platform.openai.com/docs/api-reference/responses)

## 修复日期

2026-07-08
