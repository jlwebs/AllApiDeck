# plugin-bridge-js

这是一个 Tampermonkey / Violentmonkey 油猴桥接脚本的基础雏形目录。

注意：

- 这里不包含自动提取或外送 `cookie`、`localStorage`、`sessionStorage` 等敏感会话数据的逻辑。
- 当前脚本只演示一个最小可运行的本地桥接握手结构，便于后续接入你自己的、显式授权的导入协议。

当前文件：

- `bridge.user.js`
  - 油猴脚本基础模板
  - 页面注入后提供一个最小桥接入口
  - 只发送非敏感元信息与显式确认后的测试 payload

建议后续演进方向：

1. 本地进程提供明确的桥接接口，例如：
   - `GET /bridge/ping`
   - `POST /bridge/import`
2. 前端应用增加“当前浏览器标签直接导入”配对码或一次性会话 token。
3. 油猴脚本仅在用户显式点击并确认后，发送用户允许的字段。
4. 对接收端增加来源校验、时间窗校验、一次性 token 校验和日志记录。

