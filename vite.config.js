import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import fs from 'fs';
import path from 'path';
import { spawn, execFileSync } from 'child_process';
import net from 'net';
import { chromium } from 'playwright-core';
import Components from 'unplugin-vue-components/vite';
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers';

// ─── 配置与辅助函数 ────────────────────────────────────────────────────────
const FETCH_LOG = path.join(process.cwd(), 'logs/fetch-keys.log');
const CHECK_LOG = path.join(process.cwd(), 'logs/check-keys.log');

if (!fs.existsSync('logs')) fs.mkdirSync('logs');

const fetchLog = (msg) => {
  const line = `[${new Date().toLocaleTimeString()}] ${msg}\n`;
  fs.appendFileSync(FETCH_LOG, line);
  console.log(line.trim());
};

const checkLog = (msg) => {
  const line = `[${new Date().toLocaleTimeString()}] ${msg}\n`;
  fs.appendFileSync(CHECK_LOG, line);
};

const ensureSkPrefix = (key) => (key && !key.startsWith('sk-') ? `sk-${key}` : key);
const isMaskedKey = (key) => key && key.includes('*');

function openUrlInSystemBrowser(targetUrl) {
  return new Promise((resolve, reject) => {
    let child;

    if (process.platform === 'win32') {
      child = spawn('cmd', ['/c', 'start', '', targetUrl], {
        detached: true,
        stdio: 'ignore',
      });
    } else if (process.platform === 'darwin') {
      child = spawn('open', [targetUrl], {
        detached: true,
        stdio: 'ignore',
      });
    } else {
      child = spawn('xdg-open', [targetUrl], {
        detached: true,
        stdio: 'ignore',
      });
    }

    child.on('error', reject);
    child.unref();
    resolve();
  });
}

function findLocalBrowserExecutable(preferredBrowser = 'chrome') {
  const envPath = process.env.PLAYWRIGHT_BROWSER_PATH;
  if (envPath && fs.existsSync(envPath)) return envPath;

  const candidateSets = process.platform === 'win32'
    ? {
        chrome: [
          'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
          'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
        ],
        edge: [
          'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
          'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
        ],
      }
    : process.platform === 'darwin'
      ? {
          chrome: ['/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'],
          edge: ['/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge'],
        }
      : {
          chrome: [
            '/usr/bin/google-chrome',
            '/usr/bin/google-chrome-stable',
            '/usr/bin/chromium',
            '/usr/bin/chromium-browser',
          ],
          edge: [
            '/usr/bin/microsoft-edge',
            '/usr/bin/microsoft-edge-stable',
          ],
        };

  const normalizedBrowser = preferredBrowser === 'edge' ? 'edge' : 'chrome';
  const preferredCandidates = candidateSets[normalizedBrowser] || [];
  const fallbackCandidates = normalizedBrowser === 'chrome'
    ? candidateSets.edge || []
    : candidateSets.chrome || [];

  return [...preferredCandidates, ...fallbackCandidates].find(candidate => fs.existsSync(candidate)) || null;
}

function getDefaultBrowserUserDataDir(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  if (process.platform === 'win32') {
    const localAppData = process.env.LOCALAPPDATA || '';
    if (!localAppData) return null;
    return normalizedBrowser === 'edge'
      ? path.join(localAppData, 'Microsoft', 'Edge', 'User Data')
      : path.join(localAppData, 'Google', 'Chrome', 'User Data');
  }
  return null;
}

function getBrowserProfileLaunchMode(browserType = 'chrome') {
  const explicitMode = String(process.env.BROWSER_PROFILE_MODE || '').trim().toLowerCase();
  if (explicitMode === 'managed-copy' || explicitMode === 'linked-default' || explicitMode === 'shadow-copy') {
    return explicitMode;
  }
  // shadow-copy: 大目录 Junction 链接 + 核心文件物理拷贝，Windows 默认
  return process.platform === 'win32' ? 'shadow-copy' : 'managed-copy';
}

function detectInstalledBrowsers() {
  const browsers = [];
  const chromePath = findLocalBrowserExecutable('chrome');
  const edgePath = findLocalBrowserExecutable('edge');

  if (chromePath) browsers.push({ type: 'chrome', path: chromePath });
  if (edgePath) browsers.push({ type: 'edge', path: edgePath });

  let defaultBrowser = null;
  if (chromePath) defaultBrowser = 'chrome';
  else if (edgePath) defaultBrowser = 'edge';

  return { browsers, defaultBrowser };
}

function createBrowserProfileInUseError(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const error = new Error(
    `检测到 ${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 已在普通模式运行，默认 profile 被占用。`
  );
  error.code = 'BROWSER_PROFILE_IN_USE';
  return error;
}

function isBrowserProcessRunning(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  try {
    if (process.platform === 'win32') {
      const imageName = normalizedBrowser === 'edge' ? 'msedge.exe' : 'chrome.exe';
      const output = execFileSync('tasklist.exe', ['/FI', `IMAGENAME eq ${imageName}`], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'ignore'],
      });
      return output.toLowerCase().includes(imageName);
    }

    if (process.platform === 'darwin') {
      const procName = normalizedBrowser === 'edge' ? 'Microsoft Edge' : 'Google Chrome';
      const output = execFileSync('pgrep', ['-f', procName], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'ignore'],
      });
      return Boolean(String(output || '').trim());
    }

    const procName = normalizedBrowser === 'edge' ? 'microsoft-edge' : 'chrome';
    const output = execFileSync('pgrep', ['-f', procName], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    return Boolean(String(output || '').trim());
  } catch {
    return false;
  }
}

function killBrowserProcesses(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  try {
    if (process.platform === 'win32') {
      const imageName = normalizedBrowser === 'edge' ? 'msedge.exe' : 'chrome.exe';
      execFileSync('taskkill.exe', ['/IM', imageName, '/F'], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      return true;
    }

    if (process.platform === 'darwin') {
      const procName = normalizedBrowser === 'edge' ? 'Microsoft Edge' : 'Google Chrome';
      execFileSync('pkill', ['-f', procName], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      return true;
    }

    const procName = normalizedBrowser === 'edge' ? 'microsoft-edge' : 'chrome';
    execFileSync('pkill', ['-f', procName], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    return true;
  } catch (err) {
    const message = String(err?.stderr || err?.stdout || err?.message || '');
    if (/not found|没有运行的任务|no running instance|not matched/i.test(message)) {
      return false;
    }
    throw err;
  }
}

async function waitForBrowserProcessStopped(browserType = 'chrome', timeoutMs = 10000) {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (!isBrowserProcessRunning(normalizedBrowser)) return true;
    await new Promise(resolve => setTimeout(resolve, 300));
  }
  return !isBrowserProcessRunning(normalizedBrowser);
}

let browserFallbackContextPromise = null;
const browserFallbackPages = new Map();
let browserFallbackContextBrowserType = null;
let browserFallbackLaunchPromise = null;
let browserFallbackLaunchBrowserType = null;
const CDP_PORTS = {
  chrome: 9222,
  edge: 9223,
};

function getCdpBaseUrl(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  return `http://127.0.0.1:${CDP_PORTS[normalizedBrowser]}`;
}

function getManagedBrowserProfileDir(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const profileDir = path.join(process.cwd(), '.browser-session-profile', normalizedBrowser);
  fs.mkdirSync(profileDir, { recursive: true });
  return profileDir;
}

function getLinkedDefaultBrowserProfileDir(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const defaultDir = getDefaultBrowserUserDataDir(normalizedBrowser);
  if (!defaultDir || !fs.existsSync(defaultDir)) {
    throw new Error(`${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 默认用户目录不存在`);
  }

  const linkRoot = path.join(process.cwd(), '.browser-session-linked');
  const linkDir = path.join(linkRoot, normalizedBrowser);
  fs.mkdirSync(linkRoot, { recursive: true });

  try {
    const stat = fs.lstatSync(linkDir);
    if (stat.isSymbolicLink()) {
      const real = fs.realpathSync(linkDir);
      if (path.resolve(real) === path.resolve(defaultDir)) {
        return { profileDir: linkDir, targetDir: defaultDir, mode: 'linked-default' };
      }
    }
    fs.rmSync(linkDir, { recursive: true, force: true });
  } catch {}

  fs.symlinkSync(defaultDir, linkDir, 'junction');
  return { profileDir: linkDir, targetDir: defaultDir, mode: 'linked-default' };
}

/**
 * 创建"影子"配置目录：
 *  - 大体积目录（Extensions/ShaderCache 等）使用 Junction 符号链接，避免拷贝耗时
 *  - 核心状态文件（Preferences/Cookies/Login Data 等）物理拷贝，避免与正在运行的浏览器争锁
 */
function createShadowProfileDir(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const defaultUserDataDir = getDefaultBrowserUserDataDir(normalizedBrowser);
  if (!defaultUserDataDir || !fs.existsSync(defaultUserDataDir)) {
    throw new Error(`${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 默认用户目录不存在，无法创建影子配置`);
  }

  const shadowRoot = path.join(process.cwd(), '.browser-session-shadow', normalizedBrowser);
  // 每次启动前重建影子目录，确保核心状态文件是最新拷贝
  try { fs.rmSync(shadowRoot, { recursive: true, force: true }); } catch {}
  fs.mkdirSync(shadowRoot, { recursive: true });

  const defaultProfileSrc = path.join(defaultUserDataDir, 'Default');
  const defaultProfileDst = path.join(shadowRoot, 'Default');
  fs.mkdirSync(defaultProfileDst, { recursive: true });

  // 1. 物理拷贝 User Data 根目录核心文件
  for (const file of ['Local State', 'First Run']) {
    const src = path.join(defaultUserDataDir, file);
    if (fs.existsSync(src)) {
      try { fs.copyFileSync(src, path.join(shadowRoot, file)); } catch (e) {
        fetchLog(`[ShadowProfile] 拷贝根文件失败: ${file} | ${e.message}`);
      }
    }
  }

  // 2. 物理拷贝 Default/ 核心文件（Chrome 启动时会对其加锁）
  const profileFilesToCopy = [
    'Preferences', 'Login Data', 'Login Data-journal',
    'Cookies', 'Cookies-journal',
    'Web Data', 'Web Data-journal',
    'Bookmarks', 'History', 'TransportSecurity',
  ];
  for (const file of profileFilesToCopy) {
    const src = path.join(defaultProfileSrc, file);
    if (fs.existsSync(src)) {
      try { fs.copyFileSync(src, path.join(defaultProfileDst, file)); } catch (e) {
        fetchLog(`[ShadowProfile] 拷贝 Default/${file} 失败: ${e.message}`);
      }
    }
  }

  // 3. 大体积目录用 Junction 符号链接，读取最新但不拷贝
  const profileDirsToLink = [
    'Extensions', 'Dictionaries', 'ShaderCache', 'GrShaderCache',
    'Application Cache', 'Code Cache', 'IndexedDB', 'Local Storage',
    'Cache', 'Cache2', 'GPUCache', 'Service Worker',
  ];
  for (const dir of profileDirsToLink) {
    const src = path.join(defaultProfileSrc, dir);
    const dst = path.join(defaultProfileDst, dir);
    if (fs.existsSync(src)) {
      try { fs.symlinkSync(src, dst, 'junction'); } catch (e) {
        fetchLog(`[ShadowProfile] 链接 Default/${dir} 失败: ${e.message}`);
      }
    }
  }

  // 4. User Data 根目录的大体积目录也做 Junction
  const rootDirsToLink = ['Crashpad', 'hyphen-data', 'WidevineCdm', 'SwReporter'];
  for (const dir of rootDirsToLink) {
    const src = path.join(defaultUserDataDir, dir);
    const dst = path.join(shadowRoot, dir);
    if (fs.existsSync(src)) {
      try { fs.symlinkSync(src, dst, 'junction'); } catch (e) {
        fetchLog(`[ShadowProfile] 链接根目录 ${dir} 失败: ${e.message}`);
      }
    }
  }

  fetchLog(`[ShadowProfile] 影子配置创建完成(${normalizedBrowser}) shadow=${shadowRoot} source=${defaultUserDataDir}`);
  return { profileDir: shadowRoot, targetDir: defaultUserDataDir, mode: 'shadow-copy' };
}

function resolveBrowserProfileDir(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const mode = getBrowserProfileLaunchMode(normalizedBrowser);
  if (mode === 'shadow-copy') {
    return createShadowProfileDir(normalizedBrowser);
  }
  if (mode === 'linked-default') {
    return getLinkedDefaultBrowserProfileDir(normalizedBrowser);
  }

  return {
    profileDir: getManagedBrowserProfileDir(normalizedBrowser),
    targetDir: null,
    mode,
  };
}


function isTcpPortReachable(port, host = '127.0.0.1', timeoutMs = 800) {
  return new Promise((resolve) => {
    const socket = net.createConnection({ port, host });
    const finish = (result) => {
      socket.destroy();
      resolve(result);
    };

    socket.setTimeout(timeoutMs);
    socket.once('connect', () => finish(true));
    socket.once('timeout', () => finish(false));
    socket.once('error', () => finish(false));
  });
}

async function getRemoteDebugVersion(browserType = 'chrome') {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 1200);
  try {
    const response = await fetch(`${getCdpBaseUrl(browserType)}/json/version`, {
      signal: controller.signal,
    });
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}

async function waitForRemoteDebugReady(browserType = 'chrome', timeoutMs = 15000) {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const port = CDP_PORTS[normalizedBrowser];
  const start = Date.now();
  let lastProgressLogAt = 0;
  while (Date.now() - start < timeoutMs) {
    const version = await getRemoteDebugVersion(browserType);
    if (version?.webSocketDebuggerUrl) return version;
    if (Date.now() - lastProgressLogAt >= 5000) {
      const running = isBrowserProcessRunning(normalizedBrowser);
      const portOpen = await isTcpPortReachable(port);
      fetchLog(`[BrowserSession] 等待 CDP(${normalizedBrowser}) elapsed=${Date.now() - start}ms running=${running} port=${portOpen ? 'open' : 'closed'}`);
      lastProgressLogAt = Date.now();
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  const running = isBrowserProcessRunning(normalizedBrowser);
  const portOpen = await isTcpPortReachable(port);
  fetchLog(`[BrowserSession] CDP 等待超时(${normalizedBrowser}) timeout=${timeoutMs}ms running=${running} port=${portOpen ? 'open' : 'closed'}`);
  return null;
}

function hasManagedBrowserLaunch(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  return (
    browserFallbackContextBrowserType === normalizedBrowser ||
    browserFallbackLaunchBrowserType === normalizedBrowser
  );
}

function launchBrowserWithRemoteDebugging(executablePath, browserType = 'chrome', profileSpec = resolveBrowserProfileDir(browserType)) {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const port = CDP_PORTS[normalizedBrowser];
  const { profileDir, targetDir, mode } = profileSpec;
  const args = [
    `--remote-debugging-port=${port}`,
    '--remote-allow-origins=*',
    `--user-data-dir=${profileDir}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-session-crashed-bubble',
  ];
  const child = spawn(executablePath, args, {
    detached: true,
    stdio: 'ignore',
  });
  fetchLog(`[BrowserSession] 启动受控浏览器(${normalizedBrowser}) pid=${child.pid} mode=${mode} profile=${profileDir}${targetDir ? ` -> ${targetDir}` : ''}`);
  child.unref();
}

function launchBrowserUrlWithRemoteDebugging(executablePath, browserType = 'chrome', targetUrl = 'about:blank', profileSpec = resolveBrowserProfileDir(browserType)) {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const port = CDP_PORTS[normalizedBrowser];
  const { profileDir, targetDir, mode } = profileSpec;
  const args = [
    `--remote-debugging-port=${port}`,
    '--remote-allow-origins=*',
    `--user-data-dir=${profileDir}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-session-crashed-bubble',
    '--new-window',
    targetUrl,
  ];
  const child = spawn(executablePath, args, {
    detached: true,
    stdio: 'ignore',
  });
  fetchLog(`[BrowserSession] 启动受控浏览器并打开首站(${normalizedBrowser}) pid=${child.pid} mode=${mode} profile=${profileDir}${targetDir ? ` -> ${targetDir}` : ''} url=${targetUrl}`);
  child.unref();
}

async function openSitesAsTabsViaCdp(sites, browserType = 'chrome') {
  if (!sites.length) return { opened: 0, attached: true };

  fetchLog(`[BrowserSession] 标签页批量打开开始(${browserType}) count=${sites.length}`);
  await getBrowserFallbackContext(browserType);
  let opened = 0;
  let currentIndex = 0;

  const worker = async () => {
    while (currentIndex < sites.length) {
      const site = sites[currentIndex++];
      try {
        await ensureBrowserFallbackPage(site.url, browserType);
        opened += 1;
      } catch (err) {
        fetchLog(`[BrowserSession] 打开标签页失败(${browserType}): [${site.name}] ${site.url} | ${err.message}`);
      }
    }
  };

  const concurrency = Math.min(6, sites.length);
  await Promise.all(Array.from({ length: concurrency }, () => worker()));
  fetchLog(`[BrowserSession] 标签页批量打开完成(${browserType}) opened=${opened}/${sites.length}`);

  return { opened, attached: true };
}

async function startManagedBrowserWithRemoteDebugging(executablePath, browserType = 'chrome', targetUrl = 'about:blank') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';

  if (browserFallbackLaunchPromise && browserFallbackLaunchBrowserType === normalizedBrowser) {
    return await browserFallbackLaunchPromise;
  }

  browserFallbackLaunchBrowserType = normalizedBrowser;
  browserFallbackLaunchPromise = (async () => {
    const profileSpec = resolveBrowserProfileDir(normalizedBrowser);
    fetchLog(`[BrowserSession] 准备启动受控浏览器(${normalizedBrowser}) mode=${profileSpec.mode} 并等待 CDP 端口 ${CDP_PORTS[normalizedBrowser]}`);
    launchBrowserUrlWithRemoteDebugging(executablePath, normalizedBrowser, targetUrl, profileSpec);
    const readyVersion = await waitForRemoteDebugReady(normalizedBrowser, 12000);
    if (!readyVersion?.webSocketDebuggerUrl) {
      fetchLog(`[BrowserSession] CDP 未就绪(${normalizedBrowser}) mode=${profileSpec.mode}`);
      throw new Error(
        `${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 启动后未能建立远程调试连接，请重试`
      );
    }
    fetchLog(`[BrowserSession] CDP 已就绪(${normalizedBrowser}) mode=${profileSpec.mode} ws=${readyVersion.webSocketDebuggerUrl}`);
    return readyVersion;
  })();

  try {
    return await browserFallbackLaunchPromise;
  } finally {
    browserFallbackLaunchPromise = null;
    browserFallbackLaunchBrowserType = null;
  }
}

async function ensureRemoteDebugBrowser(browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const existing = await getRemoteDebugVersion(normalizedBrowser);
  if (existing?.webSocketDebuggerUrl) return existing;

  const executablePath = findLocalBrowserExecutable(normalizedBrowser);
  if (!executablePath) {
    throw new Error(`未找到可用的 ${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 浏览器，请先安装或设置 PLAYWRIGHT_BROWSER_PATH`);
  }

  fetchLog(`[BrowserSession] 尝试启动 ${normalizedBrowser} 并开启远程调试: ${executablePath}`);
  launchBrowserWithRemoteDebugging(executablePath, normalizedBrowser);

  const version = await waitForRemoteDebugReady(normalizedBrowser);
  if (!version?.webSocketDebuggerUrl) {
    throw new Error(
      `${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 未能开启远程调试。请先完全关闭已运行的${normalizedBrowser === 'chrome' ? ' Chrome ' : ' Edge '}窗口后重试，或手动使用 --remote-debugging-port=${CDP_PORTS[normalizedBrowser]} 启动。`
    );
  }

  return version;
}

async function openSitesViaBrowserLaunch(sites, browserType = 'chrome') {
  const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
  const executablePath = findLocalBrowserExecutable(normalizedBrowser);
  if (!executablePath) {
    throw new Error(`未找到可用的 ${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 浏览器，请先安装或设置 PLAYWRIGHT_BROWSER_PATH`);
  }

  const version = await getRemoteDebugVersion(normalizedBrowser);
  if (version?.webSocketDebuggerUrl) {
    fetchLog(`[BrowserSession] 检测到现有 CDP 会话(${normalizedBrowser})，改为标签页打开`);
    return await openSitesAsTabsViaCdp(sites, normalizedBrowser);
  }

  if (browserFallbackLaunchPromise && browserFallbackLaunchBrowserType === normalizedBrowser) {
    fetchLog(`[BrowserSession] ${normalizedBrowser} 已在受控启动中，等待就绪后改为标签页打开`);
    await browserFallbackLaunchPromise;
    return await openSitesAsTabsViaCdp(sites, normalizedBrowser);
  }

  if (sites.length > 0) {
    fetchLog(`[BrowserSession] 首次启动受控浏览器(${normalizedBrowser})，使用独立 profile 打开首个站点，再通过标签页追加其余站点`);
    await startManagedBrowserWithRemoteDebugging(executablePath, normalizedBrowser, sites[0].url);
    await new Promise(resolve => setTimeout(resolve, 800));
    if (sites.length === 1) {
      return { opened: 1, attached: true };
    }
    const remainder = sites.slice(1);
    const tabResult = await openSitesAsTabsViaCdp(remainder, normalizedBrowser);
    return { opened: 1 + tabResult.opened, attached: true };
  } else {
    await startManagedBrowserWithRemoteDebugging(executablePath, normalizedBrowser);
  }

  return { opened: sites.length, attached: true };
}

async function getBrowserFallbackContext(preferredBrowser = 'chrome') {
  const normalizedBrowser = preferredBrowser === 'edge' ? 'edge' : 'chrome';

  if (browserFallbackContextPromise && browserFallbackContextBrowserType === normalizedBrowser) {
    return browserFallbackContextPromise;
  }

  if (browserFallbackContextPromise && browserFallbackContextBrowserType !== normalizedBrowser) {
    browserFallbackContextPromise = null;
    browserFallbackContextBrowserType = null;
    browserFallbackPages.clear();
  }

  browserFallbackContextPromise = (async () => {
    const version = await ensureRemoteDebugBrowser(normalizedBrowser);
    fetchLog(`[BrowserSession] 通过 CDP 附着 ${normalizedBrowser}: ${version.webSocketDebuggerUrl}`);
    const browser = await chromium.connectOverCDP(getCdpBaseUrl(normalizedBrowser));
    browser.on('disconnected', () => {
      browserFallbackContextPromise = null;
      browserFallbackContextBrowserType = null;
      browserFallbackLaunchPromise = null;
      browserFallbackLaunchBrowserType = null;
      browserFallbackPages.clear();
    });

    const context = browser.contexts()[0];
    if (!context) {
      await browser.close().catch(() => {});
      throw new Error('CDP 已连接，但未找到可用浏览器上下文');
    }

    browserFallbackContextBrowserType = normalizedBrowser;
    return context;
  })().catch(err => {
    browserFallbackContextPromise = null;
    browserFallbackContextBrowserType = null;
    throw err;
  });

  return browserFallbackContextPromise;
}

function getCompatHeaders(userId) {
  return {
    'new-api-user': userId,
    'one-api-user': userId,
    'New-API-User': userId,
    'Veloera-User': userId,
    'voapi-user': userId,
    'User-id': userId,
    'Rix-Api-User': userId,
    'neo-api-user': userId,
  };
}

async function ensureBrowserFallbackPage(targetUrl, preferredBrowser = 'chrome', options = {}) {
  const navigateIfDifferentOrigin = options.navigateIfDifferentOrigin !== false;
  const context = await getBrowserFallbackContext(preferredBrowser);
  const origin = new URL(targetUrl).origin;
  let page = browserFallbackPages.get(origin);
  let createdNewPage = false;

  if (!page || page.isClosed()) {
    page = context.pages().find(candidate => {
      try {
        return !candidate.isClosed() && new URL(candidate.url()).origin === origin;
      } catch {
        return false;
      }
    });
  }

  if (!page || page.isClosed()) {
    page = await context.newPage();
    browserFallbackPages.set(origin, page);
    createdNewPage = true;
  }

  try {
    const currentOrigin = page.url() && page.url() !== 'about:blank' ? new URL(page.url()).origin : null;
    const shouldNavigate = createdNewPage || (navigateIfDifferentOrigin && currentOrigin !== origin);
    if (shouldNavigate && currentOrigin !== origin) {
      fetchLog(`[BrowserSession] 标签页导航开始(${preferredBrowser}): ${targetUrl}`);
      await page.goto(targetUrl, { waitUntil: 'domcontentloaded', timeout: 15000 });
      fetchLog(`[BrowserSession] 标签页导航成功(${preferredBrowser}): ${targetUrl}`);
    }
  } catch (err) {
    fetchLog(`[BrowserSession] 打开页面失败: ${targetUrl} | ${err.message}`);
  }

  return page;
}

async function closeBrowserFallbackPage(targetUrl, preferredBrowser = 'chrome') {
  const normalizedBrowser = preferredBrowser === 'edge' ? 'edge' : 'chrome';
  const origin = new URL(targetUrl).origin;
  const page = browserFallbackPages.get(origin);

  if (!page || page.isClosed()) {
    browserFallbackPages.delete(origin);
    return false;
  }

  try {
    await page.close({ runBeforeUnload: false });
    browserFallbackPages.delete(origin);
    fetchLog(`[BrowserSession] 关闭已完成标签页(${normalizedBrowser}): ${targetUrl}`);
    return true;
  } catch (err) {
    fetchLog(`[BrowserSession] 关闭标签页失败(${normalizedBrowser}): ${targetUrl} | ${err.message}`);
    return false;
  }
}

async function fetchTokensForAccountViaBrowserSession(account, preferredBrowser = 'chrome') {
  const { site_url, site_name, account_info, id, api_key } = account;
  const access_token = account.access_token || account_info?.access_token;
  const baseUrl = (site_url || '').replace(/\/+$/, '');
  const userId = String(account_info?.id || '').trim();

  if (!baseUrl) {
    return { ...account, tokens: [], error: 'browser_session_missing_url' };
  }

  const page = await ensureBrowserFallbackPage(baseUrl, preferredBrowser, {
    // Do not force navigation during polling; keep user's manual login flow intact.
    navigateIfDifferentOrigin: false,
  });
  fetchLog(`[BrowserSession] Start site fetch(${preferredBrowser}): [${site_name}] ${baseUrl}`);

  const endpoints = getTokenListEndpoints(account, baseUrl);
  const normalizedAccessToken = String(access_token || '').trim();
  const authValue = normalizedAccessToken
    ? (normalizedAccessToken.startsWith('Bearer ')
      ? normalizedAccessToken
      : `Bearer ${normalizedAccessToken}`)
    : '';
  const compatHeaders = userId ? getCompatHeaders(userId) : {};
  const classifyBrowserSessionFailure = (failures) => {
    const failureList = Array.isArray(failures) ? failures : [];
    if (!failureList.length) return 'browser_session_no_tokens';

    const reasons = new Set(failureList.map(item => String(item?.reason || '').toLowerCase()));
    const only401or404 = failureList.every(item => {
      const status = Number(item?.status);
      return status === 401 || status === 404 || Number.isNaN(status);
    });
    const hasNetworkException = failureList.some(item => {
      if (String(item?.reason || '') !== 'fetch_exception') return false;
      const msg = String(item?.message || '').toLowerCase();
      return msg.includes('fetch failed') || msg.includes('err_connection') || msg.includes('timeout');
    });
    const hasInvalidTokenBusinessError = failureList.some(item => {
      const reason = String(item?.reason || '').toLowerCase();
      if (reason !== 'business_error') return false;
      const msg = String(item?.message || '').toLowerCase();
      return msg.includes('token') && (msg.includes('invalid') || msg.includes('无效') || msg.includes('expired') || msg.includes('过期'));
    });

    if (hasNetworkException) return 'browser_session_network_unreachable';
    if (reasons.has('html_response')) return 'browser_session_html_challenge';
    if (reasons.has('http_403')) return 'browser_session_cf_blocked';
    if (hasInvalidTokenBusinessError) return 'browser_session_token_invalid_or_expired';
    if (only401or404 && reasons.has('http_401')) return 'browser_session_login_or_token_expired';
    if (reasons.has('empty_items')) return 'browser_session_empty_items';
    return 'browser_session_no_tokens';
  };

  let result;
  const evaluateOnce = async () => page.evaluate(async ({ baseUrl, endpoints, authValue, compatHeaders }) => {
    const extractItems = (json) => {
      if (Array.isArray(json)) return json;
      if (json && typeof json === 'object') {
        let d = json.data || json.items;
        if (d && typeof d === 'object' && !Array.isArray(d)) {
          d = d.data || d.items || d;
        }
        if (Array.isArray(d)) return d;
        if (Array.isArray(json.data)) return json.data;
      }
      return [];
    };

    const normalizeBearerToken = (input) => {
      const value = String(input || '').trim();
      if (!value) return '';
      return value.replace(/^Bearer\s+/i, '').trim();
    };

    const getSafeStorage = (name) => {
      try {
        const value = window?.[name];
        if (value && typeof value.getItem === 'function') return value;
      } catch {}
      return null;
    };

    const safeReadStorage = (storage, key) => {
      try {
        if (!storage || typeof storage.getItem !== 'function') return null;
        return storage.getItem(key);
      } catch {
        return null;
      }
    };

    const safeWriteStorage = (storage, key, value) => {
      try {
        if (!storage || typeof storage.setItem !== 'function') return;
        storage.setItem(key, value);
      } catch {
        // ignore
      }
    };

    const localStorageRef = getSafeStorage('localStorage');
    const sessionStorageRef = getSafeStorage('sessionStorage');

    const parseJsonObject = (raw) => {
      try {
        if (!raw || typeof raw !== 'string') return null;
        const parsed = JSON.parse(raw);
        return parsed && typeof parsed === 'object' ? parsed : null;
      } catch {
        return null;
      }
    };

    const readUserIdCandidate = (obj) => {
      if (!obj || typeof obj !== 'object') return '';
      const rawId = obj.id ?? obj.user_id ?? obj.userId ?? obj?.user?.id;
      if (rawId === undefined || rawId === null) return '';
      const value = String(rawId).trim();
      return value && /^\d+$/.test(value) ? value : '';
    };

    const buildCompatHeadersByUserId = (uid) => {
      if (!uid) return {};
      return {
        'New-API-User': uid,
        'Veloera-User': uid,
        'voapi-user': uid,
        'User-id': uid,
        'Rix-Api-User': uid,
        'neo-api-user': uid,
      };
    };

    const localAuthUserRaw = safeReadStorage(localStorageRef, 'auth_user');
    const localUserRaw = safeReadStorage(localStorageRef, 'user');
    const localAuthUserObj = parseJsonObject(localAuthUserRaw);
    const localUserObj = parseJsonObject(localUserRaw);
    const runtimeUserId =
      readUserIdCandidate(localAuthUserObj) ||
      readUserIdCandidate(localUserObj) ||
      '';
    const runtimeCompatHeaders = {
      ...compatHeaders,
      ...buildCompatHeadersByUserId(runtimeUserId),
    };

    const baseHeaders = {
      'Accept': 'application/json, text/plain, */*',
      'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
      'Pragma': 'no-cache',
      'Cache-Control': 'no-cache',
      'X-Requested-With': 'XMLHttpRequest',
      'Referer': `${baseUrl}/`,
      'Origin': baseUrl,
      ...runtimeCompatHeaders,
    };

    const tokenCandidates = [];
    const tokenSources = new Set();
    const pushTokenCandidate = (source, rawToken) => {
      const token = normalizeBearerToken(rawToken);
      if (!token) return;
      if (tokenSources.has(`${source}:${token}`)) return;
      tokenSources.add(`${source}:${token}`);
      tokenCandidates.push({ source, token });
    };

    pushTokenCandidate('provided', authValue);
    pushTokenCandidate('localStorage.auth_token', safeReadStorage(localStorageRef, 'auth_token'));
    pushTokenCandidate('localStorage.access_token', safeReadStorage(localStorageRef, 'access_token'));
    pushTokenCandidate('localStorage.token', safeReadStorage(localStorageRef, 'token'));
    pushTokenCandidate('localStorage.authToken', safeReadStorage(localStorageRef, 'authToken'));
    pushTokenCandidate('sessionStorage.auth_token', safeReadStorage(sessionStorageRef, 'auth_token'));
    pushTokenCandidate('sessionStorage.access_token', safeReadStorage(sessionStorageRef, 'access_token'));
    pushTokenCandidate('sessionStorage.token', safeReadStorage(sessionStorageRef, 'token'));
    pushTokenCandidate('sessionStorage.authToken', safeReadStorage(sessionStorageRef, 'authToken'));
    pushTokenCandidate('localStorage.user.access_token', localUserObj?.access_token);
    pushTokenCandidate('localStorage.auth_user.access_token', localAuthUserObj?.access_token);

    const isSub2ApiLike = endpoints.some((url) => /\/api\/v1\/keys/i.test(url));
    if (isSub2ApiLike) {
      const authToken = normalizeBearerToken(safeReadStorage(localStorageRef, 'auth_token'));
      const refreshToken = String(safeReadStorage(localStorageRef, 'refresh_token') || '').trim();
      const tokenExpiresAtRaw = String(safeReadStorage(localStorageRef, 'token_expires_at') || '').trim();
      const tokenExpiresAt = Number(tokenExpiresAtRaw);
      const shouldRefresh =
        !!refreshToken &&
        Number.isFinite(tokenExpiresAt) &&
        tokenExpiresAt > 0 &&
        tokenExpiresAt - Date.now() <= 120000;

      if (shouldRefresh) {
        try {
          const refreshHeaders = {
            'Content-Type': 'application/json',
          };
          if (authToken) {
            refreshHeaders.Authorization = `Bearer ${authToken}`;
          }

          const refreshRes = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
            method: 'POST',
            headers: refreshHeaders,
            credentials: 'include',
            body: JSON.stringify({ refresh_token: refreshToken }),
          });
          const refreshBody = await refreshRes.json().catch(() => null);
          const refreshedToken = normalizeBearerToken(
            refreshBody?.data?.access_token || refreshBody?.access_token,
          );
          if (refreshRes.ok && refreshedToken) {
            pushTokenCandidate('sub2api.refresh', refreshedToken);
            safeWriteStorage(localStorageRef, 'auth_token', refreshedToken);
            const refreshedRefreshToken = String(
              refreshBody?.data?.refresh_token || refreshBody?.refresh_token || '',
            ).trim();
            if (refreshedRefreshToken) {
              safeWriteStorage(localStorageRef, 'refresh_token', refreshedRefreshToken);
            }
            const refreshedExpiresIn = Number(refreshBody?.data?.expires_in || refreshBody?.expires_in);
            if (Number.isFinite(refreshedExpiresIn) && refreshedExpiresIn > 0) {
              safeWriteStorage(localStorageRef, 'token_expires_at', String(Date.now() + refreshedExpiresIn * 1000));
            }
          }
        } catch {}
      }
    }

    const strategies = [
      { name: 'token-auth', credentials: 'omit', withAuth: true },
      { name: 'cookie-auth', credentials: 'include', withAuth: false },
      { name: 'mixed-auth', credentials: 'include', withAuth: true },
    ];
    const failures = [];

    const readTextPreview = async (response) => {
      try {
        const text = await response.text();
        if (!text) return '';
        return text.slice(0, 180).replace(/\s+/g, ' ').trim();
      } catch {
        return '';
      }
    };

    const extractSecretKey = (payload) => {
      if (!payload) return '';
      if (typeof payload === 'string') return payload.trim();
      if (typeof payload !== 'object') return '';
      const candidates = [
        payload?.key,
        payload?.data?.key,
        payload?.data,
        payload?.result?.key,
        payload?.result?.data?.key,
        payload?.token,
      ];
      for (const candidate of candidates) {
        if (typeof candidate === 'string' && candidate.trim()) {
          return candidate.trim();
        }
      }
      return '';
    };

    for (const strategy of strategies) {
      for (const endpoint of endpoints) {
        const authVariants = strategy.withAuth ? tokenCandidates : [{ source: 'none', token: '' }];
        for (const authVariant of authVariants) {
          try {
            const requestHeaders = strategy.withAuth
              ? { ...baseHeaders, Authorization: `Bearer ${authVariant.token}` }
              : baseHeaders;
            const response = await fetch(endpoint, {
              method: 'GET',
              headers: requestHeaders,
              credentials: strategy.credentials,
            });

            const ct = response.headers.get('content-type') || '';
            if (!response.ok) {
              failures.push({
                endpoint,
                strategy: strategy.name,
                authSource: authVariant.source,
                status: response.status,
                contentType: ct,
                reason: `http_${response.status}`,
                preview: await readTextPreview(response),
              });
              continue;
            }

            if (/html/i.test(ct)) {
              failures.push({
                endpoint,
                strategy: strategy.name,
                authSource: authVariant.source,
                status: response.status,
                contentType: ct,
                reason: 'html_response',
                preview: await readTextPreview(response),
              });
              continue;
            }

            const body = await response.json().catch(() => null);
            if (!body) {
              failures.push({
                endpoint,
                strategy: strategy.name,
                authSource: authVariant.source,
                status: response.status,
                contentType: ct,
                reason: 'json_parse_failed',
              });
              continue;
            }

            if (typeof body === 'object' && body) {
              if (typeof body.code === 'number' && body.code !== 0) {
                failures.push({
                  endpoint,
                  strategy: strategy.name,
                  authSource: authVariant.source,
                  status: response.status,
                  contentType: ct,
                  reason: 'business_error',
                  message: String(body.message || `code_${body.code}`),
                });
                continue;
              }
              if (body.success === false) {
                failures.push({
                  endpoint,
                  strategy: strategy.name,
                  authSource: authVariant.source,
                  status: response.status,
                  contentType: ct,
                  reason: 'business_error',
                  message: String(body.message || 'success_false'),
                });
                continue;
              }
            }

            const items = extractItems(body);
            if (!items.length) {
              failures.push({
                endpoint,
                strategy: strategy.name,
                authSource: authVariant.source,
                status: response.status,
                contentType: ct,
                reason: 'empty_items',
              });
              continue;
            }

            const tokens = [];
            for (const item of items) {
              const rawKey = item?.key || item?.access_token || item?.token || item?.api_key || item?.apikey || (typeof item === 'string' ? item : '');
              if (rawKey && String(rawKey).includes('*') && item?.id) {
                try {
                  let fullKey = '';
                  const secretCredentialOrder = [
                    strategy.credentials,
                    strategy.credentials === 'omit' ? 'include' : 'omit',
                  ];
                  const secretEndpointCandidates = [
                    { url: `${baseUrl}/api/token/${item.id}/key`, method: 'POST' },
                    { url: `${baseUrl}/api/token/${item.id}/key`, method: 'GET' },
                    { url: `${baseUrl}/api/token/${item.id}`, method: 'GET' },
                    { url: `${baseUrl}/api/v1/keys/${item.id}`, method: 'GET' },
                  ];
                  for (const credentialMode of secretCredentialOrder) {
                    for (const secretEndpoint of secretEndpointCandidates) {
                      const secretHeaders = strategy.withAuth
                        ? { ...baseHeaders, Authorization: `Bearer ${authVariant.token}` }
                        : baseHeaders;
                      const secretResponse = await fetch(secretEndpoint.url, {
                        method: secretEndpoint.method,
                        headers: secretHeaders,
                        credentials: credentialMode,
                      });
                      if (!secretResponse.ok) continue;
                      const secretBody = await secretResponse.json().catch(() => null);
                      fullKey = extractSecretKey(secretBody);
                      if (fullKey) break;
                    }
                    if (fullKey) break;
                  }

                  tokens.push({
                    ...item,
                    key: fullKey || rawKey,
                    masked: !fullKey,
                    unresolved: !fullKey,
                  });
                  continue;
                } catch {}
              }

              tokens.push({ ...item, key: rawKey || 'unknown_token_format' });
            }

            if (tokens.length) {
              return {
                success: true,
                endpoint,
                strategy: strategy.name,
                authSource: authVariant.source,
                tokens,
              };
            }
          } catch (error) {
            failures.push({
              endpoint,
              strategy: strategy.name,
              authSource: authVariant.source,
              reason: 'fetch_exception',
              message: error?.message || String(error),
            });
            continue;
          }
        }
      }
    }

    return {
      success: false,
      tokens: [],
      error: 'browser_session_no_tokens',
      failures,
    };
  }, { baseUrl, endpoints, authValue, compatHeaders });

  const shouldRetryEvaluateError = (message) => {
    const m = String(message || '');
    return /Execution context was destroyed|Cannot find context with specified id|Target closed/i.test(m);
  };

  try {
    for (let attempt = 1; attempt <= 3; attempt += 1) {
      try {
        result = await evaluateOnce();
        break;
      } catch (error) {
        const errMsg = error?.message || String(error);
        if (attempt < 3 && shouldRetryEvaluateError(errMsg)) {
          fetchLog(`[BrowserSession] [${site_name}] evaluate retry ${attempt}/3: ${errMsg}`);
          await new Promise((resolve) => setTimeout(resolve, 500 * attempt));
          continue;
        }
        throw error;
      }
    }
  } catch (error) {
    const errMsg = error?.message || String(error);
    fetchLog(`[BrowserSession] [${site_name}] evaluate failed: ${errMsg}`);
    return {
      id,
      site_name,
      site_url,
      api_key,
      tokens: [],
      account_info,
      access_token,
      error: 'browser_session_page_context_lost',
      diagnostics: [{ reason: 'evaluate_exception', message: errMsg }],
    };
  }

  if (!result?.success) {
    const classifiedError = classifyBrowserSessionFailure(result?.failures);
    const failureSummary = Array.isArray(result?.failures)
      ? result.failures
        .slice(0, 4)
        .map(item => `${item.strategy || 'unknown'}@${item.authSource || 'none'}|${item.status || '-'}|${item.reason || 'unknown'}|${item.endpoint || '-'}`)
        .join(' ; ')
      : '';
    fetchLog(`[BrowserSession] [${site_name}] fetch failed reason=${classifiedError} ${failureSummary ? `details=${failureSummary}` : ''}`);
    return {
      id,
      site_name,
      site_url,
      api_key,
      tokens: [],
      account_info,
      access_token,
      error: classifiedError,
      diagnostics: Array.isArray(result?.failures) ? result.failures : [],
    };
  }

  const browserSessionDetailPreview = (Array.isArray(result.tokens) ? result.tokens : [])
    .slice(0, 5)
    .map((token, idx) => {
      const tokenId = token?.id ?? token?.token_id ?? `idx${idx + 1}`;
      const tokenName = String(token?.name || token?.token_name || '').trim();
      const tokenKey = String(token?.key || token?.access_token || '').trim();
      const tokenKeyPreview = tokenKey ? `${tokenKey.slice(0, 12)}...${tokenKey.slice(-4)}` : '(empty-key)';
      return `#${tokenId}${tokenName ? `(${tokenName})` : ''}:${tokenKeyPreview}`;
    })
    .join(' | ');
  fetchLog(`[BrowserSession] [${site_name}] fetch success count=${result.tokens.length} strategy=${result.strategy || 'unknown'} authSource=${result.authSource || 'unknown'} endpoint=${result.endpoint || '-'} details=${browserSessionDetailPreview || '(no-preview)'}`);
  await closeBrowserFallbackPage(baseUrl, preferredBrowser);
  return {
    id,
    site_name,
    site_url,
    api_key,
    tokens: result.tokens,
    account_info,
    access_token,
    endpoint: result.endpoint,
    strategy: result.strategy,
  };
}

const extractItems = (json) => {
  if (Array.isArray(json)) return json;
  if (json && typeof json === 'object') {
    // 处理各种嵌套返回结构 (Data/Items, Data.Data/Data.Items)
    let d = json.data || json.items;
    if (d && typeof d === 'object' && !Array.isArray(d)) {
       d = d.data || d.items || d;
    }
    if (Array.isArray(d)) return d;
    if (Array.isArray(json.data)) return json.data;
  }
  return [];
};

/**
 * 掩码 Key 递归解析
 */
function extractSecretKeyFromPayload(payload) {
  if (!payload) return '';
  if (typeof payload === 'string') return payload.trim();
  if (typeof payload !== 'object') return '';
  const candidates = [
    payload?.key,
    payload?.data?.key,
    payload?.data,
    payload?.result?.key,
    payload?.result?.data?.key,
    payload?.token,
  ];
  for (const candidate of candidates) {
    if (typeof candidate === 'string' && candidate.trim()) {
      return candidate.trim();
    }
  }
  return '';
}

async function resolveFullKey(baseUrl, tokenId, authValue, compatHeaders, siteName, retryCount = 0) {
  const MAX_RETRIES = 5;
  const endpointCandidates = [
    { url: `${baseUrl}/api/token/${tokenId}/key`, method: 'POST' },
    { url: `${baseUrl}/api/token/${tokenId}/key`, method: 'GET' },
    { url: `${baseUrl}/api/token/${tokenId}`, method: 'GET' },
    { url: `${baseUrl}/api/v1/keys/${tokenId}`, method: 'GET' },
  ];
  try {
    const failedStatus = [];
    for (const endpoint of endpointCandidates) {
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), 12000);
      const res = await fetch(endpoint.url, {
        method: endpoint.method,
        headers: {
          ...(authValue ? { 'Authorization': authValue } : {}),
          'Accept': 'application/json, text/plain, */*',
          ...(endpoint.method !== 'GET' ? { 'Content-Type': 'application/json' } : {}),
          'Pragma': 'no-cache',
          'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
          ...compatHeaders,
        },
        signal: ctrl.signal,
      });
      clearTimeout(timer);

      if (res.status === 429 && retryCount < MAX_RETRIES) {
        const delay = Math.min(10000, Math.pow(2, retryCount) * 1000) + Math.random() * 1000;
        fetchLog(`[${siteName}] [Resolve] token#${tokenId} 限流(429)，${(delay/1000).toFixed(1)}s 后重试 (${retryCount + 1}/${MAX_RETRIES})`);
        await new Promise(r => setTimeout(r, delay));
        return resolveFullKey(baseUrl, tokenId, authValue, compatHeaders, siteName, retryCount + 1);
      }

      if (!res.ok) {
        failedStatus.push(`${endpoint.method} ${endpoint.url} => ${res.status}`);
        continue;
      }

      const data = await res.json().catch(() => null);
      const key = extractSecretKeyFromPayload(data);
      if (key) {
        return ensureSkPrefix(key);
      }
      failedStatus.push(`${endpoint.method} ${endpoint.url} => 200 but no key`);
    }
    fetchLog(`[${siteName}] [Resolve] token#${tokenId} 失败: ${failedStatus.join(' ; ')}`);
  } catch (err) {
    fetchLog(`[${siteName}] [Resolve] token#${tokenId} 失败: ${err.message}`);
  }
  return null;
}

function getTokenListEndpoints(account, baseUrl) {
  if (account.site_type === 'sub2api') {
    return [
      `${baseUrl}/api/v1/keys?page=1&page_size=100`,
      `${baseUrl}/api/v1/keys?p=0&size=100`,
      `${baseUrl}/api/token/?p=0&size=100`,
      `${baseUrl}/api/token?p=0&size=100`,
    ];
  }

  return [
    `${baseUrl}/api/token/?p=0&size=100`,
    `${baseUrl}/api/token?p=0&size=100`,
    `${baseUrl}/api/v1/keys?page=1&page_size=100`,
    `${baseUrl}/api/v1/keys?p=0&size=100`,
  ];
}

/**
 * 核心：提取账号的所有令牌 (参考 all-api-hub 分页机制)
 */
async function fetchTokensForAccount(account) {
  const { site_url, site_name, account_info, id, api_key } = account;
  const access_token = account.access_token || account_info?.access_token;
  
  if (!site_url || !access_token) {
    if (!site_url) fetchLog(`[${site_name || '未知'}] 缺失 URL，跳过`);
    else fetchLog(`[${site_name}] 缺失 AccessToken，跳过`);
    return { ...account, tokens: [], error: '账号数据不完整 (缺少 URL 或 Token)' };
  }

  const baseUrl = site_url.replace(/\/+$/, '');
  const userId = String(account_info?.id || '');

  const compatHeaders = {
    'New-API-User': userId,
    'Veloera-User': userId,
    'voapi-user': userId,
    'User-id': userId,
    'Rix-Api-User': userId,
    'neo-api-user': userId,
  };

  const browserHeaders = {
    'Accept': 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
    'Referer': `${baseUrl}/`,
    'X-Requested-With': 'XMLHttpRequest',
  };

  const authValue = String(access_token).startsWith('Bearer ') ? access_token : `Bearer ${access_token}`;
  const allTokens = [];
  const pageSize = 100;
  const maxPages = 10;
  
  const endpoints = getTokenListEndpoints(account, baseUrl);
  
  for (const endpoint of endpoints) {
    if (allTokens.length > 0) break;

    const supportsPagedLoop = /[?&](p|page)=/i.test(endpoint);
    const firstPage = /[?&]page=1/i.test(endpoint) ? 1 : 0;
    const loopCount = supportsPagedLoop ? maxPages : 1;

    for (let pageIndex = 0; pageIndex < loopCount; pageIndex++) {
      const currentPage = firstPage + pageIndex;
      const url = supportsPagedLoop
        ? endpoint
            .replace(/([?&])p=\d+/i, `$1p=${currentPage}`)
            .replace(/([?&])page=\d+/i, `$1page=${currentPage}`)
        : endpoint;

      try {
        const ctrl = new AbortController();
        const timeout = setTimeout(() => ctrl.abort(), 10000);
        const res = await fetch(url, {
          headers: { 'Authorization': authValue, ...browserHeaders, ...compatHeaders },
          signal: ctrl.signal
        });
        clearTimeout(timeout);
        
        if (res.ok) {
          const json = await res.json();
          const items = extractItems(json);
          if (items.length === 0) break;
          
          allTokens.push(...items);
          const detailPreview = items
            .slice(0, 5)
            .map((item, idx) => {
              const tokenId = item?.id ?? item?.token_id ?? `idx${idx + 1}`;
              const keyLike = String(item?.key || item?.access_token || item?.token || item?.api_key || item?.apikey || '').trim();
              const keyPreview = keyLike ? `${keyLike.slice(0, 12)}...${keyLike.slice(-4)}` : '(empty-key)';
              const namePreview = String(item?.name || item?.token_name || '').trim();
              return `#${tokenId}${namePreview ? `(${namePreview})` : ''}:${keyPreview}`;
            })
            .join(' | ');
          fetchLog(`[${site_name}] 第 ${currentPage} 页获取成功，共 ${items.length} 个，明细: ${detailPreview || '(no-preview)'}`);
          if (items.length < pageSize) break;
        } else {
          if (pageIndex === 0) fetchLog(`[${site_name}] 提取失败(${res.status}): ${url}`);
          break;
        }
      } catch (err) {
        if (pageIndex === 0) fetchLog(`[${site_name}] 异常: ${url} | ${err.message}`);
        break;
      }
    }
  }

  if (allTokens.length === 0) {
    return { ...account, tokens: [], error: '未能获取到任何令牌' };
  }

  // 递归解析掩码 Key
  const resolvedTokens = [];
  for (const t of allTokens) {
    const rawKey = t.key || t.access_token || t.token || t.api_key || t.apikey || (typeof t === 'string' ? t : '');
    if (isMaskedKey(rawKey) && t.id) {
       const full = await resolveFullKey(baseUrl, t.id, authValue, compatHeaders, site_name);
       if (full) {
         resolvedTokens.push({ ...t, key: full });
       } else {
         resolvedTokens.push({ ...t, key: rawKey || '未知格式Token', masked: true, unresolved: true });
       }
    } else if (rawKey && rawKey.length > 5) {
       resolvedTokens.push({ ...t, key: ensureSkPrefix(rawKey) });
    } else if (t.id || t.name) {
       // 保留没解析出key的对象，以防因为格式兼容问题全丢了
       resolvedTokens.push({ ...t, key: rawKey || '未知格式Token' });
    }
  }

  const unresolvedCount = resolvedTokens.filter(token => token?.unresolved === true || isMaskedKey(String(token?.key || ''))).length;
  fetchLog(`[${site_name}] 提取汇总: raw=${allTokens.length}, resolved=${resolvedTokens.length}, unresolved=${unresolvedCount}`);

  return { id, site_name, site_url, api_key, tokens: resolvedTokens, account_info, access_token };
}

// ─── Vite 插件：混合代理中间件 ───────────────────────────────────────────────
function proxyMiddlewarePlugin() {
  return {
    name: 'proxy-middleware',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (req.url.startsWith('/api/')) {
          const requestOrigin = req.headers.origin;
          if (requestOrigin) {
            res.setHeader('Access-Control-Allow-Origin', requestOrigin);
            res.setHeader('Vary', 'Origin');
          } else {
            res.setHeader('Access-Control-Allow-Origin', '*');
          }
          res.setHeader('Access-Control-Allow-Methods', 'GET,POST,OPTIONS');
          res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization, X-Requested-With');
          res.setHeader('Access-Control-Max-Age', '86400');

          if (req.method === 'OPTIONS') {
            res.statusCode = 204;
            return res.end();
          }
        }

        // 0. 清空日志接口
        if (req.url.startsWith('/api/clear-logs')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          const params = new URL(req.url, 'http://localhost').searchParams;
          const target = params.get('type') || 'check';
          const file = target === 'fetch' ? FETCH_LOG : CHECK_LOG;
          fs.writeFileSync(file, '', 'utf8');
          return res.end(JSON.stringify({ success: true }));
        }

        // 1. 批量提取 Token 接口
        if (req.url.startsWith('/api/fetch-keys')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
             try {
               const { accounts } = JSON.parse(body);
               fetchLog(`开始批量提取 ${accounts.length} 个站点的令牌...`);
               const results = new Array(accounts.length);
               let currentIndex = 0;
               const CONCURRENCY = 25;
               const worker = async () => {
                 while (currentIndex < accounts.length) {
                   const i = currentIndex++;
                   results[i] = await fetchTokensForAccount(accounts[i]);
                 }
               };
               await Promise.all(Array.from({ length: CONCURRENCY }).map(worker));
              const total = results.reduce((n, r) => n + (r.tokens?.length || 0), 0);
              const successSites = results.filter(item => Array.isArray(item?.tokens) && item.tokens.length > 0).length;
              const unresolvedTokens = results.reduce((n, r) => {
                const tokens = Array.isArray(r?.tokens) ? r.tokens : [];
                return n + tokens.filter(token => token?.unresolved === true || String(token?.key || '').includes('*')).length;
              }, 0);
              const usableTokens = Math.max(0, total - unresolvedTokens);
              fetchLog(`===== 完成提取: successSites=${successSites}/${results.length}, total=${total}, usable=${usableTokens}, unresolved=${unresolvedTokens} =====`);
               res.setHeader('Content-Type', 'application/json');
               res.end(JSON.stringify({ results }));
             } catch (err) {
               res.statusCode = 500;
               res.end(JSON.stringify({ message: err.message }));
             }
          });
          return;
        }

        // 1.5. 打开系统浏览器兜底页面，旧方案保留
        if (req.url.startsWith('/api/open-sites')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const { sites = [] } = JSON.parse(body || '{}');
              const normalized = sites
                .map(site => ({
                  name: String(site?.name || '未知站点'),
                  url: String(site?.url || '').trim(),
                }))
                .filter(site => /^https?:\/\//i.test(site.url));

              for (const site of normalized) {
                fetchLog(`[BrowserFallback] 打开站点: [${site.name}] ${site.url}`);
                await openUrlInSystemBrowser(site.url);
                await new Promise(r => setTimeout(r, 200));
              }

              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({ success: true, opened: normalized.length }));
            } catch (err) {
              res.statusCode = 500;
              res.end(JSON.stringify({ success: false, message: err.message }));
            }
          });
          return;
        }

        // 1.55. 探测本机可用浏览器
        if (req.url.startsWith('/api/browser-session/browsers')) {
          try {
            const result = detectInstalledBrowsers();
            res.setHeader('Content-Type', 'application/json');
            res.end(JSON.stringify({ success: true, ...result }));
          } catch (err) {
            res.statusCode = 500;
            res.end(JSON.stringify({ success: false, message: err.message }));
          }
          return;
        }

        // 1.6. 在受控浏览器会话中打开站点，等待用户手动登录/过盾
        if (req.url.startsWith('/api/browser-session/status')) {
          try {
            const query = new URL(req.url, 'http://localhost').searchParams;
            const browserType = query.get('browserType') === 'edge' ? 'edge' : 'chrome';
            const remoteDebug = await getRemoteDebugVersion(browserType);
            const running = isBrowserProcessRunning(browserType);
            res.setHeader('Content-Type', 'application/json');
            res.end(JSON.stringify({
              success: true,
              browserType,
              running,
              attached: Boolean(remoteDebug?.webSocketDebuggerUrl),
              launching: Boolean(browserFallbackLaunchPromise && browserFallbackLaunchBrowserType === browserType),
              managed: hasManagedBrowserLaunch(browserType),
            }));
          } catch (err) {
            res.statusCode = 500;
            res.end(JSON.stringify({ success: false, message: err.message }));
          }
          return;
        }

        if (req.url.startsWith('/api/browser-session/open')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const { sites = [], browserType = 'chrome' } = JSON.parse(body || '{}');
              const normalized = sites
                .map(site => ({
                  name: String(site?.name || '未知站点'),
                  url: String(site?.url || '').trim(),
                }))
                .filter(site => /^https?:\/\//i.test(site.url));

              const launchResult = await openSitesViaBrowserLaunch(normalized, browserType);
              normalized.forEach(site => {
                fetchLog(`[BrowserSession] 打开受控页面(${browserType}): [${site.name}] ${site.url}`);
              });

              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({ success: true, opened: normalized.length, attached: launchResult.attached }));
            } catch (err) {
              const code = err?.code || 'BROWSER_SESSION_OPEN_FAILED';
              res.statusCode = code === 'BROWSER_PROFILE_IN_USE' ? 409 : 500;
              res.end(JSON.stringify({ success: false, code, message: err.message }));
            }
          });
          return;
        }

        // 1.65. 结束已运行的系统浏览器进程，释放默认 profile
        if (req.url.startsWith('/api/browser-session/kill')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const { browserType = 'chrome' } = JSON.parse(body || '{}');
              const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
              const killed = killBrowserProcesses(normalizedBrowser);
              browserFallbackContextPromise = null;
              browserFallbackContextBrowserType = null;
              browserFallbackLaunchPromise = null;
              browserFallbackLaunchBrowserType = null;
              browserFallbackPages.clear();
              const stopped = await waitForBrowserProcessStopped(normalizedBrowser, 12000);
              fetchLog(`[BrowserSession] 结束浏览器进程(${normalizedBrowser}): ${killed ? 'success' : 'no-process'}`);
              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({ success: true, killed, stopped, browserType: normalizedBrowser }));
            } catch (err) {
              res.statusCode = 500;
              res.end(JSON.stringify({ success: false, code: 'BROWSER_KILL_FAILED', message: err.message }));
            }
          });
          return;
        }

        // 1.66. 结束已运行浏览器进程后，立即重新以受控模式打开目标站点
        if (req.url.startsWith('/api/browser-session/restart-open')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const { browserType = 'chrome', sites = [] } = JSON.parse(body || '{}');
              const normalizedBrowser = browserType === 'edge' ? 'edge' : 'chrome';
              const normalizedSites = sites
                .map(site => ({
                  name: String(site?.name || '未知站点'),
                  url: String(site?.url || '').trim(),
                }))
                .filter(site => /^https?:\/\//i.test(site.url));

              const killed = killBrowserProcesses(normalizedBrowser);
              browserFallbackContextPromise = null;
              browserFallbackContextBrowserType = null;
              browserFallbackLaunchPromise = null;
              browserFallbackLaunchBrowserType = null;
              browserFallbackPages.clear();
              const stopped = await waitForBrowserProcessStopped(normalizedBrowser, 12000);
              fetchLog(`[BrowserSession] 结束浏览器进程(${normalizedBrowser}): ${killed ? 'success' : 'no-process'}`);

              if (!stopped) {
                res.statusCode = 409;
                return res.end(JSON.stringify({
                  success: false,
                  code: 'BROWSER_KILL_NOT_STOPPED',
                  message: `${normalizedBrowser === 'chrome' ? 'Chrome' : 'Edge'} 进程结束后仍未完全退出`,
                }));
              }

              const launchResult = await openSitesViaBrowserLaunch(normalizedSites, normalizedBrowser);
              normalizedSites.forEach(site => {
                fetchLog(`[BrowserSession] 重启后打开受控页面(${normalizedBrowser}): [${site.name}] ${site.url}`);
              });

              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({
                success: true,
                killed,
                stopped,
                opened: normalizedSites.length,
                attached: launchResult.attached,
                browserType: normalizedBrowser,
              }));
            } catch (err) {
              const code = err?.code || 'BROWSER_RESTART_OPEN_FAILED';
              res.statusCode = 500;
              res.end(JSON.stringify({ success: false, code, message: err.message }));
            }
          });
          return;
        }

        // 1.7. 直接在受控浏览器会话中发起 token 抓取，复用用户刚刚完成验证的 cookie/session
        if (req.url.startsWith('/api/browser-session/fetch-keys')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const {
                accounts = [],
                browserType = 'chrome',
                round = null,
                totalRounds = null,
              } = JSON.parse(body || '{}');
              const roundText =
                Number.isFinite(Number(round)) && Number.isFinite(Number(totalRounds))
                  ? ` round=${Number(round)}/${Number(totalRounds)}`
                  : '';
              fetchLog(`[BrowserSession] Start auto fetch(${browserType})${roundText} accounts=${accounts.length}`);
              const results = new Array(accounts.length);
              let currentIndex = 0;
              const CONCURRENCY = 4;

              const worker = async () => {
                while (currentIndex < accounts.length) {
                  const i = currentIndex++;
                  results[i] = await fetchTokensForAccountViaBrowserSession(accounts[i], browserType);
                }
              };

              await Promise.all(Array.from({ length: Math.min(CONCURRENCY, accounts.length || 1) }).map(worker));
              const successCount = results.filter(item => Array.isArray(item?.tokens) && item.tokens.length > 0).length;
              const failedResults = results.filter(item => !Array.isArray(item?.tokens) || item.tokens.length === 0);
              const failedSummary = failedResults
                .slice(0, 5)
                .map(item => {
                  const name = item?.site_name || 'unknown-site';
                  const reason = item?.error || 'unknown-error';
                  const firstDiag = Array.isArray(item?.diagnostics) && item.diagnostics.length > 0
                    ? item.diagnostics[0]
                    : null;
                  const diagText = firstDiag
                    ? `${firstDiag.strategy || 'unknown'}|${firstDiag.status || '-'}|${firstDiag.reason || 'unknown'}`
                    : '-';
                  return `[${name}] ${reason} diag=${diagText}`;
                })
                .join(' ; ');
              fetchLog(`[BrowserSession] Auto fetch done(${browserType})${roundText} successSites=${successCount}/${accounts.length}${failedSummary ? ` failed=${failedSummary}` : ''}`);
              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({ success: true, results }));
            } catch (err) {
              fetchLog(`[BrowserSession] Auto fetch error: ${err.message}`);
              res.statusCode = 500;
              res.end(JSON.stringify({ success: false, message: err.message }));
            }
          });
          return;
        }

        // 2. 模型发现接口代理
        if (req.url.startsWith('/api/proxy-get')) {
          const query = new URL(req.url, 'http://localhost').searchParams;
          const targetUrl = query.get('url');
          const uid = query.get('uid') || '';
          const auth = req.headers['authorization'];
          if (!targetUrl) { res.statusCode = 400; return res.end('URL Required'); }

          const baseUrl = new URL(targetUrl).origin;
          const compatHeaders = {
            'New-API-User': uid, 'Veloera-User': uid, 'voapi-user': uid,
            'User-id': uid, 'Rix-API-User': uid, 'neo-api-user': uid,
          };
          const browserHeaders = {
            'Accept': 'application/json, text/plain, */*',
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
            'Referer': `${baseUrl}/`,
            'X-Requested-With': 'XMLHttpRequest',
          };

          try {
            checkLog(`[PROXY-GET] 正在请求: ${targetUrl} | UID: ${uid}`);
            const ctrl = new AbortController();
            const timeout = setTimeout(() => ctrl.abort(), 10000);
            const apiRes = await fetch(targetUrl, {
              headers: { 'Authorization': auth || '', ...browserHeaders, ...compatHeaders },
              signal: ctrl.signal
            });
            clearTimeout(timeout);
            const data = await apiRes.text();
            res.statusCode = apiRes.status;
            res.setHeader('Content-Type', apiRes.headers.get('content-type') || 'application/json');
            if (!apiRes.ok) {
               checkLog(`[PROXY-GET] 失败(${apiRes.status}): ${targetUrl} | 长度:${data.length} | 预览:${data.slice(0,200)}`);
            } else {
               checkLog(`[PROXY-GET] 成功(${apiRes.status}): ${targetUrl} | 长度:${data.length}`);
            }
            return res.end(data);
          } catch (err) {
            checkLog(`[PROXY-GET] 异常: ${err.message}`);
            res.statusCode = 500; return res.end(JSON.stringify({ message: err.message }));
          }
        }

        // 3. 密钥/模型可用性检测
        if (req.url.startsWith('/api/check-key')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const params = JSON.parse(body || '{}');
              if (params._isFirst) {
                fs.writeFileSync(CHECK_LOG, '', 'utf8');
              }

              // 兼容两种前端请求协议：
              // 1) 旧协议: { site, tokenKey, model }
              // 2) 新协议: { url, key, model, messages }
              let normalized = params;
              if ((!params?.url || !params?.key) && params?.site) {
                const site = params.site;
                const uid = String(site?.account_info?.id || site?.id || '').trim();
                const apiBaseUrl = site.api_key?.startsWith('http')
                  ? String(site.api_key).replace(/\/+$/, '')
                  : String(site.site_url || '').replace(/\/+$/, '');
                normalized = {
                  url: apiBaseUrl,
                  key: String(params.tokenKey || '').trim(),
                  model: params.model,
                  messages: params.messages,
                  uid,
                };
              } else if (!normalized?.uid && params?.uid) {
                normalized = { ...normalized, uid: String(params.uid).trim() };
              }

              const { checkKey } = await import('./api/local/checkKey.js');
              const result = await checkKey(normalized, checkLog);
              res.statusCode = Number(result?.status || 500);
              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify(result?.body || { error: { message: 'check_key_empty_result' } }));
            } catch (err) {
              checkLog(`[CHECK] 异常: ${err.message}`);
              res.statusCode = 500;
              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({ error: { message: err.message } }));
            }
          });
          return;
        }

        next();
      });
    }
  };
}

export default defineConfig({
  plugins: [
    vue(),
    Components({
      resolvers: [AntDesignVueResolver({ importStyle: false, resolveIcons: true })],
    }),
    proxyMiddlewarePlugin(),
  ],
  server: { port: 3000, host: '0.0.0.0', watch: { ignored: ['**/all-api-hub/**'] } },
  resolve: { alias: { '@': '/src' } },
  optimizeDeps: {
    exclude: ['all-api-hub'],
    entries: ['index.html', 'src/**/*.vue']
  },
  css: {
    preprocessorOptions: {
      less: {
        modifyVars: { hack: `true; @import "~ant-design-vue/lib/style/themes/dark.less";` },
        javascriptEnabled: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('ant-design-vue')) return 'ant-design-vue';
            if (id.includes('lodash')) return 'lodash';
            return 'vendor';
          }
        },
      },
    },
  },
});


