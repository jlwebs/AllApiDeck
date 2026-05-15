<div align="center">

<img src="./desktop/assets/appicon.png" alt="All API Deck" width="80">

**All API Deck：面向海量中转站 / 密钥的导入、扫描、测试、管理与客户端接管工具**

<p>
支持站点账号便携导入、批量模型发现、快速测活、密钥分组管理、桌面客户端一键接管，以及 Claude / Codex / OpenCode / OpenClaw 的本地高级代理。
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
--><img alt="Golang" src="https://img.shields.io/badge/Golang-1.24%2B-00ADD8?logo=go&style=flat"><!--
--><img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript&style=flat">
</p>

<p align="center">
  <a href="README.md"><strong>中文</strong></a> |
  <a href="desktop/docs/readme/README.en.md">English</a>
</p>

</div>

## 这是什么

All API Deck 不是单纯的“测一个接口能不能用”的小工具，而是围绕“海量站点 / 海量 key / 多模型切换”做的一套桌面工作流：

- 把分散在浏览器扩展、备份文件、目录里的站点 / key / 账号便携迁移到桌面端
- 对大量站点并发做模型发现、快速测活、性能判断和异常筛选
- 在一个界面里完成分组、筛选、状态查看、调度观察和日常维护
- 需要接 Claude / Codex / OpenCode / OpenClaw 时，再用本地高级代理把体验补齐
- 遇到协议不一致时自动尝试 `messages` / `responses` / `chat/completions`，并记录真实路径方便排障

如果你经常在多个公益站、自建站、聚合站之间切换，还要反复导入、筛选、测试、整理和接管客户端，这个项目就是为这类使用方式设计的。高级代理很重要，但它是并列能力，也是舒适对接的最后一环。

## 当前能力

- 支持从浏览器扩展桥接、扩展备份文件、目录扫描等方式便携导入站点与账号
- 支持批量拉取模型列表、批量快速测活、批量检测模型可用性与性能指标
- 支持按分组、状态、模型等维度管理大量密钥记录
- 支持本地侧栏 / miniBar / 悬窗视图，方便观察调度状态、组织调用集群与快速操作
- 支持 Claude / Codex / OpenCode / OpenClaw 一键生成并写入本机配置
- 内置高级代理，支持 provider 队列、故障转移、协议 fallback、错误修正、请求记录
- 支持请求记录详情调试：对最近请求自动生成完整 `fetch(...)` 命令，便于改 header / body 直接复测

## 界面预览

<img src="./desktop/docs/images/all-api-deck-home.png" alt="All API Deck 首页预览" width="72%" />
<img src="./desktop/docs/images/all-api-deck-site-management.png" alt="All API Deck 站点管理" width="72%" />
<img src="./desktop/docs/images/all-api-deck-sidebar.png" alt="侧边栏" width="24%" />

## 高级代理流转图

<img src="./desktop/docs/images/advanced-proxy-architecture-light.svg" alt="All API Deck 高级代理流转图" width="86%" />

## 核心功能

### 1. 站点 / 账号导入与迁移

支持多种导入方式：

- 浏览器扩展桥接导入
- ALL-API-HUB 备份 JSON 导入
- 扩展目录 / 数据目录扫描导入

### 2. 批量模型发现与扫描

对多站点并发拉取模型列表，并保留：

- 成功 / 失败状态
- 失败原因
- 发现到的模型集合
- 后续筛选和批量管理所需的结构化结果

适合快速从大量站点里定位“哪些站点真的有目标模型”。

### 3. 批量快速测活 / 可用性检测

支持对目标站点和模型执行批量检测，输出：

- 可用 / 异常状态
- 状态码和错误原因
- TTFT / TPS / Latency
- 协议探测与 fallback 结果
- 复现所需的请求信息

这里不是只测一个固定协议，而是会结合站点能力，自动尝试可行的 OpenAI / Anthropic 兼容入口。

### 4. 密钥管理、分组与侧栏 / 悬窗

支持：

- 给记录分组
- 从剪贴板批量导入密钥
- 在 miniBar / 侧栏 / 悬窗里快速查看记录状态
- 针对单个记录快速刷新、快速测活、切换模型
- 查看当前 provider 队列和实时调度命中项
- 观察调度与组织调用集群

Windows 下的侧栏 / 悬窗体验最完整；非 Windows 环境可通过 miniBar / 独立窗体使用类似能力。

### 5. 桌面客户端一键接管

当前已覆盖的典型目标应用包括：

- Claude
- Codex
- OpenCode
- OpenClaw

支持基于当前选中的站点记录，生成配置预览并写入本机配置文件，减少手动编辑 base URL、token、模型和协议参数的重复劳动。

### 6. 高级代理

支持：

- provider 优先级队列
- 自动故障转移
- `messages` / `responses` / `chat/completions` 多协议 fallback
- 针对不同 host / key / model 的协议偏好记忆
- 请求整流修正
- `invalid_encrypted_content` 自动愈合
- 调度状态可视化
- 请求记录与路由追踪

典型例子：

- 某个上游只支持 `chat/completions`，但客户端默认走 `responses`
- 某个 Claude 兼容上游只接受 `/v1/messages`
- 同一 host 上不同模型支持的协议不一致

### 7. 请求记录与调试

请求记录面板会保存高级代理近期请求的关键信息：

- 入口 / 出口
- 实际上游 URL
- 路由回退轨迹
- 状态码
- 耗时、TTFT、Latency、TPS
- 输入 / 输出 Token
- 错误摘要

此外，最近 50 条请求还会在内存中附带完整 request body。打开详情后可以：

- 查看格式化后的请求内容
- 自动生成完整 `fetch(...)` 调试命令
- 直接改 headers / body / URL
- 立即在前端本地发起复测

## 适合谁

这个项目更适合下面这些用户：

- 有大量中转站 / key / 模型组合，需要集中管理
- 已经在浏览器扩展或备份文件里积累了很多记录，想便携迁移到桌面端
- 需要高频做模型发现、批量测试、快速筛选和分组维护
- 需要给 Claude / Codex / OpenCode / OpenClaw 接入本地代理
- 经常遇到协议不兼容、模型错配、错误复现困难
- 希望把“导入、扫描、测试、管理、接管客户端、排查失败”放在一个桌面工具里完成

## 快速开始

### 1. 下载桌面版

从 Releases 下载对应平台版本：

https://github.com/jlwebs/AllApiDeck/releases

当前 GitHub Release 会附带这些产物：

- Windows：`allapideck-windows-amd64.exe`
- Windows：`allapideck-windows-amd64.msi`
- macOS：`allapideck-macos-universal.dmg`
- Linux：`allapideck-linux-amd64.tar.gz`
- Linux：`allapideck-linux-amd64.deb`
- Linux：`allapideck-linux-amd64.AppImage`

Windows 自动更新当前优先选择并拉起 `.msi` 安装包，`.exe` 作为兼容兜底资产保留。

### 2. 导入站点记录

推荐优先使用：

- 浏览器扩展桥接导入
- ALL-API-HUB 备份 JSON 导入

常见备份文件名例如：

- `accounts-backup.json`
- `accounts-backup-2026-04-01.json`

### 3. 批量拉模型 / 快速测活

导入后通常先做两件事：

1. 批量拉取模型列表
2. 对目标模型做快速测活

这样你能很快知道：

- 哪些站点真的有这个模型
- 哪些 key 当前可用
- 哪些站点需要切协议或不适合接入桌面客户端

### 4. 按需开启高级代理接管

如果你要让 Claude / Codex / OpenCode / OpenClaw 走本地高级代理：

1. 在“高级代理功能”里配置 provider 队列
2. 为目标应用开启接管
3. 在配置预览里确认 base URL、token、协议类型
4. 写入本机配置

## 项目结构

```text
.
├─ desktop/                          桌面端项目主目录
│  ├─ src/                           Vue 前端页面与组件
│  ├─ wailsjs/                       Wails 绑定代码
│  ├─ scripts/                       开发、打包、安装脚本
│  ├─ docs/                          文档与截图
│  ├─ build/                         桌面构建输出
│  ├─ release-assets/                CI 产物暂存目录
│  ├─ main.go                        Wails 入口
│  ├─ app.go                         应用生命周期与后端主逻辑
│  ├─ advanced_proxy_*.go            高级代理相关逻辑
│  ├─ local_api.go                   本地测活 / 协议探测逻辑
│  └─ window_sidebar.go              托盘 / 侧边栏窗口逻辑
└─ .github/workflows/                发布与 CI 配置
```

## 技术栈

- 桌面壳：`Wails`
- 前端界面：`Vue 3 + Ant Design Vue + Vite`
- 本地后端逻辑：`Go`
- 打包与发布：`GitHub Actions + Wails + 平台补充脚本`

## 开发环境

建议环境：

- Windows 10/11
- Go 1.24+
- Node.js 24+
- npm 11+
- WebView2 Runtime

> 目前 Windows 是主要开发和验证环境，部分功能（尤其侧栏 / miniBar / 某些桌面客户端接管体验）在 Windows 下最完整。

## 本地开发

安装依赖：

```bash
cd desktop
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

标准桌面构建：

```bash
npm run build:desktop
```

调试版桌面构建：

```bash
npm run build:desktop-debug
```

仅前端构建：

```bash
npm run build:web
```

构建产物默认位于：

```text
desktop/build/bin/
```

## 日志与运行时目录

程序运行时目录不是固定写在仓库里，而是落到系统运行时目录中。

典型位置：

- Windows：`%LOCALAPPDATA%\\BatchApiCheck\\runtime`
- macOS：`~/Library/Application Support/BatchApiCheck/runtime`
- Linux：`$XDG_STATE_HOME` / `$XDG_CACHE_HOME` 下的 `batch-api-check/runtime`

日志通常位于：

```text
runtime/logs/
```

常见日志文件包括：

- `advanced-proxy.log`
- `EXE_BACKEND_DEBUG.log`
- `client-runtime.log`
- `wails-dev-host.log`
- `wails-dev-runner.log`
- `wails-dev-vite.log`

## 发布方式

仓库使用 GitHub Actions 自动构建桌面版 release 资产。

当前发布工作流会在打 tag 后自动：

- 构建 Windows / macOS / Linux 产物
- 为 Windows 额外生成 `.msi`
- 为 Linux 额外组装 `.deb` 与 `.AppImage`
- 上传到对应 GitHub Release

## 项目主页

https://github.com/jlwebs/AllApiDeck

## 致谢

感谢 [Linux.do](https://linux.do/) 社区提供的反馈、测试和传播支持。
