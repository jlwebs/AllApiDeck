<div align="center">

<img src="../../assets/appicon.png" alt="All API Deck" width="86">

# All API Deck

**A desktop console for large API key fleets, local AI clients, and advanced proxy routing**

Import, sync, test, group, observe sessions, manage MCP / Skills, take over local clients, guard tool calls, and debug upstream routing from one desktop workflow.

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
--><a href="../../../LICENSE">
  <img alt="License" src="https://img.shields.io/badge/license-MIT-brightgreen?style=flat">
</a><!--
--><img alt="Golang" src="https://img.shields.io/badge/Golang-1.24%2B-00ADD8?logo=go&style=flat"><!--
--><img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-5.x-3178C6?logo=typescript&style=flat">
</p>

<p align="center">
  <a href="../../../README.md">中文</a> |
  <a href="./README.en.md"><strong>English</strong></a>
</p>

</div>

## Overview

All API Deck is not a one-off endpoint tester. It is a desktop workbench for people who maintain many relay sites, API keys, models, and local AI clients over time.

It is built for workflows where:

- You switch between public relay sites, self-hosted sites, and aggregators, and need one place to manage API keys.
- You need to know which sites really support a target model, which keys are alive, and how they perform.
- You want Claude, Codex, OpenCode, or OpenClaw to use a stable local proxy endpoint.
- You need visibility into provider queues, route fallback, real upstream exits, and recent connections.
- You want a local gateway-level guard against polluted tool calls and poisoned upstream responses.

## Workflow

| Stage | What you do | What All API Deck provides |
|---|---|---|
| Import | Bring scattered sites, accounts, and keys into the desktop app | Browser-extension bridge, ALL-API-HUB backup JSON, directory scanning, clipboard batch import |
| Organize | Turn many records into maintainable assets | Key vault, groups, status filters, site search, dedicated export actions |
| Decide | Find usable models and healthy keys | Batch model discovery, quick checks, TTFT / TPS / Latency, protocol probing |
| Take over | Route local AI clients through a stable entrypoint | Claude / Codex / OpenCode / OpenClaw config preview and writeback |
| Route | Hide upstream protocol differences from clients | Provider queues, failover, `messages` / `responses` / `chat/completions` fallback |
| Debug | Reproduce failed or slow requests | Request records, route traces, connection info, editable `fetch(...)` replay |
| Guard | Reduce toolchain poisoning risk | Dynamic guard prompts, string protection, toolcall return validation, strict mode |

## Interface Preview

<table>
  <tr>
    <td width="50%">
      <img src="../images/keyvault.jpg" alt="Synced key vault">
      <br><strong>Synced key vault</strong><br>
      Local records, healthy counts, sync history, group filters, site search, quick checks, and dedicated exports.
    </td>
    <td width="50%">
      <img src="../images/sessions.jpg" alt="Sessions, MCP, and Skill">
      <br><strong>Sessions / MCP / Skill</strong><br>
      Project sessions, message history, MCP entries, Skill state, and multi-client context in one panel.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <img src="../images/proxy.jpg" alt="Advanced proxy connection records">
      <br><strong>Advanced proxy connections</strong><br>
      Provider queue, model, entry, exit, route trace, elapsed time, HTTP status, and recent connections.
    </td>
    <td width="50%">
      <img src="../images/antipoison.jpg" alt="Prompt-injection guard">
      <br><strong>Prompt-injection guard</strong><br>
      Dynamic toolchain watermarking, randomized prompts, return-flow validation stats, and strict mode.
    </td>
  </tr>
</table>

## Advanced Proxy Flow

<p align="center">
  <img src="../images/advanced-proxy-architecture-light.svg" alt="All API Deck Advanced Proxy Flow" width="86%">
</p>

## Feature Modules

### 1. Synced key vault

The new main workspace is organized around the key vault:

- Shows local record count, healthy count, last sync time, and the latest batch sync result.
- Filters by all keys, custom groups, quick groups, and site names.
- Supports copying key / base URL, selecting models, quick checks, and status inspection per record.
- Dedicated export actions send usable records into client configuration flows.

### 2. Import and migration

Supported import paths:

- Browser-extension bridge import.
- ALL-API-HUB backup JSON import.
- Extension directory / local data directory scanning.
- Clipboard batch API key import.

Common backup filenames:

- `accounts-backup.json`
- `accounts-backup-2026-04-01.json`

### 3. Batch model discovery and quick checks

After import, the usual first steps are:

1. Fetch model lists in batch to confirm what each site really supports.
2. Run quick checks against target models to confirm which keys are usable now.

Results keep:

- Success / failure state and failure reason.
- Status code, TTFT, TPS, and latency.
- Protocol probing and fallback result.
- Request details required for reproduction.

### 4. Sessions, MCP, and Skill panel

All API Deck now covers more than API key management. It also improves local AI toolchain visibility:

- Find historical sessions by project path and time.
- Inspect user and assistant messages to recover task context.
- Watch MCP services, Skill state, and multi-entry clients.
- Reduce switching between clients, config files, and logs.

### 5. Sidebar, miniBar, and floating windows

Sidebar / miniBar / floating windows are for continued observation and quick action outside the main window.

<p align="center">
  <img src="../images/all-api-deck-sidebar.png" alt="All API Deck Sidebar" width="26%">
  <img src="../images/minifloating.jpg" alt="All API Deck mini floating" width="34%">
</p>

They support:

- Inspecting record state from miniBar, sidebar, or floating windows.
- Refreshing one record, running quick checks, and switching models.
- Watching the current provider queue and live scheduling hits.
- Observing dispatch state, organizing call clusters, and locating abnormal requests.

> Windows currently has the most complete sidebar and floating-window experience. Other platforms can use miniBar or standalone windows for similar workflows.

### 6. One-click desktop client takeover

Supported targets:

- Claude
- Codex
- OpenCode
- OpenClaw

From the selected site record, All API Deck can generate a config preview and write local client configuration files, reducing repeated manual edits of base URL, token, model, and protocol settings.

### 7. Advanced proxy

Advanced Proxy lets clients keep using a stable local endpoint while protocol differences, retries, and routing decisions stay inside the proxy layer.

Supports:

- Provider priority queues.
- Automatic failover.
- `messages` / `responses` / `chat/completions` protocol fallback.
- Protocol preference memory by host / key / model.
- Request normalization and healing.
- `invalid_encrypted_content` auto-healing.
- Scheduling visualization.
- Request records and route tracing.

Typical cases:

- An upstream only supports `chat/completions`, but the client defaults to `responses`.
- A Claude-compatible upstream only accepts `/v1/messages`.
- Different models on the same host support different protocols.

### 8. Request records and debugging

The request records panel keeps key data from recent advanced-proxy requests:

- Entry / exit route.
- Real upstream URL.
- Route fallback trace.
- Status code.
- Elapsed time, TTFT, latency, and TPS.
- Input / output tokens.
- Error summary.

The latest 50 requests also keep full request bodies in memory. Open details to inspect formatted request content, generate a complete `fetch(...)` replay command, and edit headers / body / URL before retesting locally.

### 9. Prompt-injection guard

The guard does not ask the model to judge its own safety. It builds a verifiable return-flow validation layer in the local Advanced Proxy gateway.

Core mechanism:

- Inject a dynamic guard prompt before forwarding requests upstream.
- If the model intends to emit a real toolcall, it must first emit `<aad_guard_json>...</aad_guard_json>`.
- Guard JSON uses minimal binding fields: `name` and `tool_name`.
- The gateway extracts real toolcalls and guard JSON on the response path and validates the binding.
- Failed validation is blocked or warned according to config; passed responses have guard JSON stripped before returning to the client.
- Keys, secrets, sensitive tool results, and user-marked `<<...>>` content can be protected and restored through string placeholders.

More detail:

- [Anti-poison design wiki](../../../anti-poison-wiki.md)
- [Anti-poison test result](../../../anti-poison-result.md)
- [Local poison demo evaluation report](../../../anti-poison-demo-eval-report.md)

## Who This Is For

- Users managing many relay sites, keys, and model combinations.
- Users migrating extension or backup data into a desktop workspace.
- Users doing repeated discovery, testing, filtering, and grouping.
- Users connecting Claude, Codex, OpenCode, or OpenClaw through a local compatibility layer.
- Users who often hit protocol mismatch, model mismatch, and hard-to-reproduce failures.
- Users who want import, checks, grouping, client takeover, routing visibility, failure debugging, and toolcall guardrails in one desktop app.

## Quick Start

### 1. Download the desktop build

Releases:

https://github.com/jlwebs/AllApiDeck/releases

Current release assets include:

- Windows: `allapideck-windows-amd64.exe`
- Windows: `allapideck-windows-amd64.msi`
- macOS: `allapideck-macos-universal.dmg`
- Linux: `allapideck-linux-amd64.tar.gz`
- Linux: `allapideck-linux-amd64.deb`
- Linux: `allapideck-linux-amd64.AppImage`

On Windows, auto-update prefers the `.msi` installer while `.exe` remains available as a compatibility fallback.

### 2. Import site records

Recommended first:

- Browser-extension bridge import.
- ALL-API-HUB backup JSON import.

### 3. Discover models and run quick checks

After import, fetch model lists in batch, then run quick checks against target models. This quickly shows:

- Which sites really have the model.
- Which keys are currently usable.
- Which sites need protocol changes or should not be connected to desktop clients.

### 4. Enable advanced proxy takeover when needed

To route Claude / Codex / OpenCode / OpenClaw through the local advanced proxy:

1. Configure the provider queue.
2. Enable takeover for the target app.
3. Verify base URL, token, protocol type, and model in the preview.
4. Write the local config.
5. Inspect real routing through request records and connection info.

## Project Structure

```text
.
├─ desktop/                          Main desktop app directory
│  ├─ src/                           Vue frontend pages and components
│  ├─ wailsjs/                       Wails binding code
│  ├─ scripts/                       Dev, build, and packaging scripts
│  ├─ docs/                          Docs and screenshots
│  ├─ build/                         Desktop build output
│  ├─ release-assets/                CI artifact staging
│  ├─ main.go                        Wails entry
│  ├─ app.go                         App lifecycle and backend core logic
│  ├─ advanced_proxy_*.go            Advanced proxy logic
│  ├─ local_api.go                   Local checks and protocol probing
│  └─ window_sidebar.go              Tray / sidebar window logic
├─ anti-poison-wiki.md               Prompt-injection guard design
├─ anti-poison-result.md             Guard test results
└─ .github/workflows/                Release and CI workflows
```

## Tech Stack

- Desktop shell: `Wails`
- Frontend UI: `Vue 3 + Ant Design Vue + Vite`
- Local backend logic: `Go`
- Packaging and release: `GitHub Actions + Wails + platform scripts`

## Development Environment

- Windows 10/11
- Go 1.24+
- Node.js 24+
- npm 11+
- WebView2 Runtime

> Windows is currently the primary development and validation environment. Some features, especially sidebar / miniBar and client takeover flows, are most complete on Windows.

## Development

Install dependencies:

```bash
cd desktop
npm install
```

Desktop dev mode:

```bash
npm run dev
```

Frontend-only dev mode:

```bash
npm run dev:web
```

## Build

Desktop build:

```bash
npm run build:desktop
```

Desktop debug build:

```bash
npm run build:desktop-debug
```

Frontend-only build:

```bash
npm run build:web
```

Build output:

```text
desktop/build/bin/
```

## Logs and Runtime Directory

Runtime data is written to the system runtime directory, not directly into the repository.

Typical locations:

- Windows: `%LOCALAPPDATA%\BatchApiCheck\runtime`
- macOS: `~/Library/Application Support/BatchApiCheck/runtime`
- Linux: `batch-api-check/runtime` under `$XDG_STATE_HOME` or `$XDG_CACHE_HOME`

Logs usually live under:

```text
runtime/logs/
```

Typical files include:

- `advanced-proxy.log`
- `EXE_BACKEND_DEBUG.log`
- `client-runtime.log`
- `wails-dev-host.log`
- `wails-dev-runner.log`
- `wails-dev-vite.log`

## Release

This repository uses GitHub Actions to build desktop release assets.

On tags, the release workflow:

- Builds Windows / macOS / Linux artifacts.
- Generates an extra `.msi` for Windows.
- Assembles `.deb` and `.AppImage` for Linux.
- Uploads artifacts to the matching GitHub Release.

## GitHub

Project homepage:

https://github.com/jlwebs/AllApiDeck

## Acknowledgements

Thanks to the [Linux.do](https://linux.do/) community for feedback, testing, and word-of-mouth support.
