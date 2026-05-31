# 防投毒当前验证摘要

本文档只保留当前实现的验证结论，过期评测流水已删除，避免和现有机制混淆。

## 当前机制

| 能力 | 当前行为 |
|---|---|
| Guard 校验 | 真实 toolcall 前必须出现 `<aad_guard_json>{...}</aad_guard_json>` 文本块 |
| Guard 字段 | 只校验 `name` 和 `tool_name`，绑定本轮随机前缀和紧随其后的真实工具名 |
| Guard 输出 | 回客户端前剥离，不展示给用户 |
| 字符串保护 | request out 占位，respond in 还原 |
| 默认保护重点 | JSON 密钥字段值、密钥形态文本、敏感工具结果、用户主动 `<...>` 标记 |
| 默认不保护 | 普通 `.env`、`settings.json`、`.claude/settings.json` 文件名 mention |

## 已验证场景

| 场景 | 预期 | 结果 |
|---|---|---|
| 无真实 toolcall | 不要求 guard JSON | 通过 |
| 真实 toolcall 缺 guard JSON | 阻断或告警，记录原因 | 通过 |
| guard JSON 字段缺失或工具名不匹配 | 识别为覆盖不匹配 | 通过 |
| guard JSON 混入响应文本 | 回客户端前剥离 | 通过 |
| 流式 guard JSON 被分片 | 仍可剥离 | 通过 |
| JSON key value 包含密钥 | 替换为占位符并还原 | 通过 |
| 工具结果包含真实敏感配置内容 | 整体保护并还原 | 通过 |
| 用户输入 `<passw0rd>` | 使用 `user_text:` 规则保护并还原 | 通过 |
| 用户或工具说明只提到 `.env` / `settings.json` | 保持原文，不生成保护记录 | 通过 |

## 最近验证命令

```powershell
cd desktop
go test . -run 'TestAntiPoisonStringProtection|TestParseToolInputMapDropsOptionalEmptyPaginationFields|TestSanitizeAntiPoisonOpenAIResponsesStreamBodyStripsSplitGuardJSON|TestApplyAntiPoisonPromptTo.*' -count=1
```

结果：通过。

```powershell
cd desktop
npm run build:desktop
```

结果：通过，产物为 `desktop/build/bin/all-api-deck.exe`。Wails 仍会输出现有大 chunk 和 reserved keyword `int` 警告，但未阻断构建。

## 观察重点

排查防投毒问题时优先看请求详情中的：

| 字段 | 用途 |
|---|---|
| `antiPoisonOps[].rule` | 判断命中的是 guard 校验、字符串保护还是还原 |
| `antiPoisonOps[].before` | request out 为原文，respond in 为占位符 |
| `antiPoisonOps[].after` | request out 为占位符，respond in 为还原原文 |
| `antiPoisonOps[].context` | 查看命中字符串在 payload 中的上下文 |
| 工具调用归类 | 判断上游是否实际返回 toolcall |
| 上游观察 | 判断模型文本、toolcall 和 guard 是否符合预期 |
