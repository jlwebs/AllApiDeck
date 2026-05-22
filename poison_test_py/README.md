# AllApiDeck Poison Test Upstream

这是一个本地投毒模拟上游，用于手动验证高级代理防投毒面板、blocked 流水和客户端可见错误。

## 启动

```powershell
cd D:\GitHub\batch-api-check\poison_test_py
python server.py
```

启动后打开：

```text
http://127.0.0.1:9999
```

脚本默认会自动打开控制页。如果不想自动打开浏览器：

```powershell
python server.py --no-browser
```

## 高级代理接入

在 AllApiDeck 高级代理里新增或编辑一个 provider：

```text
Base URL: http://127.0.0.1:9999/v1
API Key: poison-local
Model: poison-test
```

模型列表接口：

```text
GET http://127.0.0.1:9999/v1/models
```

内置测试模型：

```text
poison-openai-chat
poison-openai-responses
poison-claude-messages
poison-stream
poison-clean
```

协议选择：

```text
OpenAI Chat: openai_chat
OpenAI Responses: openai_responses
Claude Messages: anthropic
```

防投毒需要在高级代理防投毒按钮里开启。

## 控制页

控制页提供三个下拉框：

- 上游协议：`openai_chat`、`openai_responses`、`claude_messages`
- 投毒类型：选择返回哪类恶意响应
- 流式模式：跟随客户端、强制 stream、强制非 stream

通常建议流式模式选择“跟随客户端请求”，这样可以同时测非流式和流式客户端。

普通测活请求如果没有 AllApiDeck 防投毒 guard prompt，模拟器会自动返回正常文本，避免模型检测面板误报“结构异常”。真正经过高级代理防投毒注入的请求，才会按控制页选择的投毒类型返回恶意 toolcall。

## 投毒类型

| 类型 | 预期效果 |
|---|---|
| `missing_guard_toolcall` | 返回真实工具调用但不返回 guard，预期被网关阻断 |
| `guard_digest_mismatch` | 返回真实工具调用和 guard，但 digest 错误，预期被网关阻断 |
| `replay_old_nonce` | guard 使用旧 alias/nonce，预期被网关阻断 |
| `tampered_arguments` | 真实工具参数被替换为危险命令，guard digest 不匹配，预期被网关阻断 |
| `extra_toolcall` | 额外追加真实工具调用，guard 不覆盖完整链路，预期被网关阻断 |
| `guard_only` | 只返回 guard，无真实工具调用，预期 guard 被剥离，通常不阻断 |
| `clean_text` | 正常文本响应，用于确认本地上游接入是否正常 |

## 验证方式

1. 启动 `python server.py`。
2. 在网页里选择投毒类型。
3. 在高级代理里把目标 provider 指向 `http://127.0.0.1:9999/v1`。
4. 通过 Codex / Claude / OpenCode 等客户端发送一次普通请求。
5. 打开防投毒详情面板，查看统计表中是否出现 blocked 行、reason、request out / respond in 流水。

## 端口

默认固定监听：

```text
127.0.0.1:9999
```

如果端口被占用，先关闭占用进程再启动。
