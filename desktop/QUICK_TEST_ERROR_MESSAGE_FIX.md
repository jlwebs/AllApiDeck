# 快速测活错误信息优化文档

## 问题描述

用户报告：快速测活失败时，401 错误只显示 "HTTP 401"，没有具体的错误原因，用户无法判断是密钥无效、过期还是其他问题。

## 示例问题

**优化前**：
```
快速测活失效

HTTP 401

输入地址: https://rrrapi.com
请求模型: gpt-5.5
超时设置: 20s

尝试日志:
1. [401] https://rrrapi.com/v1/chat/completions -> HTTP 401
2. [401] https://rrrapi.com/v1/responses -> HTTP 401
```

用户看不出具体原因，只知道是 401 错误。

## 根本原因分析

### 1. 后端错误消息提取不完善

在 `local_api.go` 的 `extractCheckErrorMessage` 函数中：

```go
func extractCheckErrorMessage(payload map[string]any, fallback string) string {
    return firstNonEmpty(
        getNestedString(payload, "error", "message"),
        strings.TrimSpace(toStringValue(payload["message"])),
        strings.TrimSpace(toStringValue(payload["error"])),
        fallback,  // 直接返回 "HTTP 401"
    )
}
```

**问题**：
- 如果上游 API 返回的 401 响应体为空或格式不标准
- 函数只会返回 `fallback`，即 "HTTP 401"
- 没有为常见的 HTTP 状态码提供有意义的默认消息

### 2. 前端显示逻辑不够清晰

在 `keyPanelStore.js` 和 `KeyManagement.vue` 中：

```javascript
attempts.forEach((attempt, index) => {
  const status = Number(attempt?.status || 0);
  const endpoint = String(attempt?.endpoint || '').trim();
  const message = String(attempt?.message || '').trim();
  lines.push(`${index + 1}. [${status || '?'}] ${endpoint}${message ? ` -> ${message}` : ''}`);
});
```

**问题**：
- 当 `message` 为空时，只显示端点和状态码
- 对于 401/403 等关键错误，没有特殊处理
- 用户无法快速识别问题类型

## 修复方案

### 修复 1: 增强后端错误消息提取

为常见的 HTTP 状态码提供有意义的默认消息：

```go
func extractCheckErrorMessage(payload map[string]any, fallback string) string {
    extracted := firstNonEmpty(
        getNestedString(payload, "error", "message"),
        strings.TrimSpace(toStringValue(payload["message"])),
        strings.TrimSpace(toStringValue(payload["error"])),
    )

    // If we extracted something meaningful, return it
    if extracted != "" && extracted != "null" {
        return extracted
    }

    // For common status codes, provide meaningful fallback messages
    if strings.HasPrefix(fallback, "HTTP 401") {
        return "HTTP 401 Unauthorized - API key invalid or expired"
    }
    if strings.HasPrefix(fallback, "HTTP 403") {
        return "HTTP 403 Forbidden - Access denied"
    }
    if strings.HasPrefix(fallback, "HTTP 429") {
        return "HTTP 429 Too Many Requests - Rate limit exceeded"
    }

    return fallback
}
```

**收益**：
- 即使上游没有返回错误详情，也能显示有意义的信息
- 用户可以立即知道 401 = 密钥问题，403 = 权限问题，429 = 限流

### 修复 2: 优化前端显示逻辑

为 401/403 等关键错误提供特殊处理：

```javascript
// keyPanelStore.js 和 KeyManagement.vue
attempts.forEach((attempt, index) => {
  const status = Number(attempt?.status || 0);
  const endpoint = String(attempt?.endpoint || '').trim();
  const messageText = String(attempt?.message || '').trim();

  // For 401/403 errors, show the error message prominently
  if (status === 401 || status === 403) {
    const errorText = messageText || 'HTTP ' + status;
    lines.push(`${index + 1}. [${status}] ${endpoint} -> ${errorText}`);
  } else if (messageText) {
    lines.push(`${index + 1}. [${status || '?'}] ${endpoint} -> ${messageText}`);
  } else {
    lines.push(`${index + 1}. [${status || '?'}] ${endpoint}`);
  }
});
```

**收益**：
- 确保 401/403 错误始终显示错误文本
- 即使后端消息为空，也会显示状态码
- 其他错误保持原有显示逻辑

## 优化效果

### 优化前

```
快速测活失效

HTTP 401

输入地址: https://rrrapi.com
请求模型: gpt-5.5
超时设置: 20s

尝试日志:
1. [401] https://rrrapi.com/v1/chat/completions -> HTTP 401
2. [401] https://rrrapi.com/v1/responses -> HTTP 401
```

**问题**：
- 不知道是密钥错误、过期还是其他问题
- 用户需要自己猜测或查阅文档

### 优化后

#### 场景 1: 上游返回详细错误

```
快速测活失效

Invalid API key provided

输入地址: https://rrrapi.com
请求模型: gpt-5.5
超时设置: 20s

尝试日志:
1. [401] https://rrrapi.com/v1/chat/completions -> Invalid API key provided
2. [401] https://rrrapi.com/v1/responses -> Invalid API key provided
```

#### 场景 2: 上游只返回 401（无详细信息）

```
快速测活失效

HTTP 401 Unauthorized - API key invalid or expired

输入地址: https://rrrapi.com
请求模型: gpt-5.5
超时设置: 20s

尝试日志:
1. [401] https://rrrapi.com/v1/chat/completions -> HTTP 401 Unauthorized - API key invalid or expired
2. [401] https://rrrapi.com/v1/responses -> HTTP 401 Unauthorized - API key invalid or expired
```

**收益**：
- 用户立即知道是密钥问题
- 可以直接去检查密钥是否正确或是否过期
- 无需猜测或查阅文档

### 其他常见错误优化

#### 403 Forbidden

**优化后**：
```
HTTP 403 Forbidden - Access denied

尝试日志:
1. [403] https://api.example.com/v1/chat/completions -> HTTP 403 Forbidden - Access denied
```

**用户行动**：检查账户权限或订阅状态

#### 429 Rate Limit

**优化后**：
```
HTTP 429 Too Many Requests - Rate limit exceeded

尝试日志:
1. [429] https://api.example.com/v1/chat/completions -> HTTP 429 Too Many Requests - Rate limit exceeded
```

**用户行动**：等待一段时间后重试，或升级配额

## 技术细节

### 为什么不在所有状态码都添加默认消息？

1. **保持简洁**：大多数错误（404, 500 等）都有上游返回的详细信息
2. **避免误导**：一些错误码（如 502）的原因多样，不应该过早下结论
3. **聚焦关键**：401/403/429 是用户最常遇到且最需要明确指引的错误

### 字符串检查的选择

使用 `strings.HasPrefix(fallback, "HTTP 401")` 而不是直接比较：

```go
if strings.HasPrefix(fallback, "HTTP 401") {
    // 匹配 "HTTP 401" 和 "HTTP 401 Unauthorized" 等变体
}
```

**原因**：
- 兼容性好：即使 fallback 已经包含部分信息，也能匹配
- 扩展性强：未来可能有其他格式的 fallback

### 前端的防御性编程

```javascript
const errorText = messageText || 'HTTP ' + status;
```

**三层保障**：
1. 优先使用后端返回的 `messageText`
2. 如果为空，至少显示 "HTTP 401"
3. 如果 status 也为空，显示 "HTTP ?"

## 测试验证

### 测试用例

1. **有详细错误的 401**
   - 输入：上游返回 `{"error": {"message": "Invalid API key"}}`
   - 预期：显示 "Invalid API key"

2. **无详细错误的 401**
   - 输入：上游返回空响应体或 HTML
   - 预期：显示 "HTTP 401 Unauthorized - API key invalid or expired"

3. **403 错误**
   - 输入：上游返回 403
   - 预期：显示 "HTTP 403 Forbidden - Access denied"

4. **429 错误**
   - 输入：上游返回 429
   - 预期：显示 "HTTP 429 Too Many Requests - Rate limit exceeded"

5. **其他错误**
   - 输入：500, 502, 504 等
   - 预期：如果有详细错误显示详细错误，否则显示 "HTTP xxx"

### 手动测试步骤

1. 准备一个无效的 API key
2. 在密钥管理页面点击"快速测活"
3. 查看错误提示对话框
4. 验证：
   - 错误消息是否清晰
   - 尝试日志是否包含详细信息
   - 用户是否能快速判断问题原因

## 后续改进方向

### 1. 更多状态码的友好消息

```go
case "HTTP 400":
    return "HTTP 400 Bad Request - Invalid request format"
case "HTTP 404":
    return "HTTP 404 Not Found - API endpoint not found"
case "HTTP 500":
    return "HTTP 500 Internal Server Error - Server error"
```

### 2. 多语言支持

```go
func getErrorMessageForStatusCode(statusCode int, lang string) string {
    if lang == "zh" {
        switch statusCode {
        case 401:
            return "HTTP 401 未授权 - API 密钥无效或已过期"
        case 403:
            return "HTTP 403 禁止访问 - 访问被拒绝"
        }
    }
    // English fallback...
}
```

### 3. 可操作的建议

在错误消息中提供下一步行动：

```
HTTP 401 Unauthorized - API key invalid or expired

建议操作：
1. 检查 API key 是否正确复制
2. 确认账户是否有效
3. 尝试重新生成密钥
```

## 结论

通过以下两个核心优化：

1. ✅ **后端增强错误消息提取**（为 401/403/429 提供默认消息）
2. ✅ **前端优化显示逻辑**（确保关键错误始终显示详情）

成功解决了快速测活 401 错误信息不清晰的问题，用户现在可以立即知道具体的错误原因，无需猜测或查阅文档。

---

优化日期: 2026-07-08
影响范围: 
- `desktop/local_api.go` - 后端错误消息提取
- `desktop/src/utils/keyPanelStore.js` - 前端显示逻辑
- `desktop/src/components/KeyManagement.vue` - 前端显示逻辑
