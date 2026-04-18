<div align="center">

<img src="./assets/appicon.png" alt="All API Deck" width="80">

**中转站的中转站 - 将分散的 AI 中转站聚合为一个统一网关**

<p>
把你在各处注册的 New API / One API / OneHub / DoneHub / Veloera / AnyRouter / Sub2API 等站点，
<br>
汇聚成 <strong>一个 API Key、一个入口</strong>，自动发现模型、智能路由、成本最优。
</p>

<p align="center">
<a href="https://github.com/jlwebs/AllApiDeck/releases">
  <img alt="GitHub Release" src="https://img.shields.io/github/v/release/jlwebs/AllApiDeck?label=Release&logo=github&style=flat">
</a><!--
--><a href="https://github.com/jlwebs/AllApiDeck/stargazers">
  <img alt="GitHub Stars" src="https://img.shields.io/github/stars/jlwebs/AllApiDeck?style=flat&logo=github&label=Stars">
</a><!--
--><a href="https://deepwiki.com/jlwebs/AllApiDeck">
  <img alt="Ask DeepWiki" src="https://deepwiki.com/badge.svg">
</a><!--
--><a href="LICENSE">
  <img alt="License" src="https://img.shields.io/badge/license-MIT-brightgreen?style=flat">
</a><!--
--><img alt="Node.js" src="https://img.shields.io/badge/Node.js-22.15%2B-339933?logo=node.js&style=flat"><!--
--><img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript&style=flat">
</p>

<p align="center">
  <a href="README.md"><strong>中文</strong></a> |
  <a href="README.en.md">English</a>
</p>

</div>
## 界面预览

<img src="./docs/images/all-api-deck-home.png" alt="All API Deck 首页预览" width="72%" />
<img src="./docs/images/all-api-deck-site-management.png" alt="All API Deck 站点管理" width="72%" />
<img src="./docs/images/all-api-deck-sidebar.png" alt="侧边栏" width="24%" />

## 主要功能

### 1. 多种导入方式支持
- 推荐，基于拓展桥自动识别当前浏览器打开的中转站页面并自动完成导入，兼容性最佳
- 也推荐从浏览器扩展ALL-API-HUB数据文件直接导入站点与账号信息，适合已有扩展数据直接一键到位

### 2. 备份 JSON 导入

支持导入ALL-API-HUB插件导出的标准备份文件，例如：

- `accounts-backup.json`
- `accounts-backup-2026-04-01.json`

### 3. 批量模型发现

对导入的多个站点并发拉取模型列表，并支持失败诊断、状态追踪与标签分组。

### 4. 批量可用性检测

支持对选定站点与模型执行批量检测，输出：

- 可用 / 异常状态
- 错误码
- 常见原因说明
- 调研 trace 日志
- fetch 复现片段

### 5. 本地 Profile / CDP 双模式

支持两类登录态读取模式：

- `Profile 文件模式`
- `CDP 重开模式`

设置页可切换，便于在不同站点兼容性之间取舍。

### 6. 侧边面板

支持最小化到托盘后使用侧边面板管理密钥记录，包括：

- 快速刷新余额
- 快速测试
- 选择模型
- 打开专属一键配置窗口
- 监控当前设置的供应队列和实时响应的目标供应商
- 非Win系统可通过密钥管理——miniBar按钮进入该窗体

### 7. 专属一键配置

支持基于当前选中的站点记录，生成桌面客户端配置变更预览，并写入本机配置文件。

当前已覆盖的典型目标应用包括：

- Claude
- Codex
- OpenCode
- OpenClaw

## 项目结构

```text
.
├─ src/                     前端页面与组件
├─ wailsjs/                 Wails 绑定代码
├─ build/                   构建输出
├─ logs/                    运行日志
├─ scripts/                 开发与构建脚本
├─ main.go                  Wails 入口
├─ app.go                   应用生命周期与后端主逻辑
├─ window_sidebar.go        托盘 / 侧边面板窗口逻辑
└─ local_api.go             本地接口与请求处理
```
## 技术栈

- 桌面壳：`Wails`
- 前端界面：`Vue 3 + Ant Design Vue + Vite`
- 本地后端逻辑：`Go`
- 
## 开发环境

建议环境：

- Windows 10/11
- Go 1.24+
- Node.js 24+
- npm 11+
- WebView2 Runtime

## 开发启动

安装依赖：

```bash
npm install
```

桌面开发模式：

```bash
npm run dev
```

仅前端开发：

```bash
npm run dev:web
```

## 构建

桌面构建：

```bash
wails build
```

或：

```bash
npm run build:desktop
```

构建产物默认位于：

```text
build/bin/
```

GitHub Release 当前会附带这些桌面端产物：

- Windows：`batch-api-check-windows-amd64.exe`
- macOS：`batch-api-check-macos-universal.dmg`
- Linux：`batch-api-check-linux-amd64.tar.gz`
- Linux AppImage：`batch-api-check-linux-amd64.AppImage`
- Linux DEB：`batch-api-check-linux-amd64.deb`

Linux 的 `.deb` 和 `AppImage` 目前是在 CI 里基于 Wails 构建产物再额外组装出来的，因为 Wails v2 本身不会直接生成这两类发布包。

## 日志

设置里也可获取，对应日志目录：

```text
logs/
```

其中通常会包含：

- `EXE_BACKEND_DEBUG.log`
- `wails-dev-host.log`
- `wails-dev-runner.log`
- `wails-dev-vite.log`

## GitHub

项目主页：

https://github.com/jlwebs/AllApiDeck

## 致谢

感谢 [Linux.do](https://linux.do/) 社区提供的反馈、测试和传播支持。
