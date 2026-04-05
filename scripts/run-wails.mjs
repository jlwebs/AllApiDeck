import { execFileSync, spawn } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '..');
const rawArgs = process.argv.slice(2);
const command = rawArgs[0] || 'dev';
const isWindows = process.platform === 'win32';

function safeGoEnv(name) {
  try {
    return execFileSync('go', ['env', name], {
      cwd: projectRoot,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
  } catch {
    return '';
  }
}

function resolveWailsExecutable() {
  const explicit = process.env.WAILS_BIN;
  if (explicit && fs.existsSync(explicit)) return explicit;

  const gobin = process.env.GOBIN || safeGoEnv('GOBIN');
  if (gobin) {
    const fromGobin = path.join(gobin, isWindows ? 'wails.exe' : 'wails');
    if (fs.existsSync(fromGobin)) return fromGobin;
  }

  const gopath = process.env.GOPATH || safeGoEnv('GOPATH');
  if (gopath) {
    const fromGopath = path.join(gopath, 'bin', isWindows ? 'wails.exe' : 'wails');
    if (fs.existsSync(fromGopath)) return fromGopath;
  }

  return isWindows ? 'wails.exe' : 'wails';
}

function buildEnv() {
  const env = { ...process.env };
  const noisyKeys = [
    'npm_config_electron_mirror',
    'NPM_CONFIG_ELECTRON_MIRROR',
    'npm_config_home',
    'NPM_CONFIG_HOME',
    'ELECTRON_MIRROR',
    'electron_mirror',
    'HOME',
  ];

  noisyKeys.forEach((key) => {
    delete env[key];
  });

  env.GOPROXY = env.GOPROXY || 'https://goproxy.cn,direct';

  const npmrcPath = path.join(projectRoot, '.npmrc.wails');
  if (!fs.existsSync(npmrcPath)) {
    fs.writeFileSync(npmrcPath, '', 'utf8');
  }
  env.npm_config_userconfig = npmrcPath;
  env.NPM_CONFIG_USERCONFIG = npmrcPath;

  return env;
}

function isPortOpen(port, host = '127.0.0.1') {
  return new Promise((resolve) => {
    const socket = net.createConnection({ port, host });
    socket.once('connect', () => {
      socket.destroy();
      resolve(true);
    });
    socket.once('error', () => {
      resolve(false);
    });
  });
}

async function findAvailablePort(startPort = 3000, attempts = 20) {
  for (let port = startPort; port < startPort + attempts; port += 1) {
    const open = await isPortOpen(port);
    if (!open) return port;
  }
  throw new Error(`No available port found in range ${startPort}-${startPort + attempts - 1}`);
}

async function waitForPort(port, timeoutMs = 30000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (await isPortOpen(port)) return true;
    await new Promise((resolve) => setTimeout(resolve, 300));
  }
  return false;
}

async function waitForHttpReady(url, timeoutMs = 30000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(url);
      if (response.ok || response.status < 500) {
        return true;
      }
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 300));
  }
  return false;
}

function killTree(pid) {
  if (!pid) return;
  try {
    if (isWindows) {
      execFileSync('taskkill.exe', ['/PID', String(pid), '/T', '/F'], {
        stdio: ['ignore', 'ignore', 'ignore'],
      });
      return;
    }
    process.kill(pid, 'SIGTERM');
  } catch {}
}

function listProjectDevProcesses() {
  if (!isWindows) return [];

  const escapedRoot = projectRoot.replace(/'/g, "''");
  const script = [
    `$project = '${escapedRoot}'`,
    "$names = @('node.exe','wails.exe','batch-api-check-dev.exe','batch-api-check.exe','cmd.exe')",
    'Get-CimInstance Win32_Process |',
    '  Where-Object {',
    '    $cmd = [string]$_.CommandLine',
    '    $name = [string]$_.Name',
    '    $cmd -and $cmd.Contains($project) -and $names.Contains($name)',
    '  } |',
    '  Select-Object ProcessId, Name, CommandLine |',
    '  ConvertTo-Json -Compress',
  ].join('\n');

  try {
    const output = execFileSync('powershell.exe', ['-NoProfile', '-Command', script], {
      cwd: projectRoot,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    if (!output) return [];
    const parsed = JSON.parse(output);
    return Array.isArray(parsed) ? parsed : [parsed];
  } catch {
    return [];
  }
}

async function cleanupStaleDevProcesses() {
  const staleProcesses = listProjectDevProcesses()
    .filter((proc) => Number(proc?.ProcessId) && Number(proc.ProcessId) !== process.pid);

  if (!staleProcesses.length) return;

  const summary = staleProcesses
    .map((proc) => `${proc.Name}:${proc.ProcessId}`)
    .join(', ');
  console.log(`[dev] Cleaning stale dev processes: ${summary}`);

  for (const proc of staleProcesses) {
    killTree(proc.ProcessId);
  }

  await new Promise((resolve) => setTimeout(resolve, 1200));
}

function attachExit(child, cleanup = () => {}) {
  child.on('error', (error) => {
    cleanup();
    console.error(`[wails] Failed to start: ${error.message}`);
    console.error('[wails] Ensure `go install github.com/wailsapp/wails/v2/cmd/wails@latest` has been run.');
    process.exit(1);
  });

  child.on('exit', (code, signal) => {
    cleanup();
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 0);
  });
}

async function runDevMode() {
  const env = buildEnv();
  const wailsExecutable = resolveWailsExecutable();
  await cleanupStaleDevProcesses();

  const sidecarPort = await findAvailablePort(13000, 20);
  const vitePort = await findAvailablePort(3000, 20);
  const viteExecutable = path.join(projectRoot, 'node_modules', 'vite', 'bin', 'vite.js');
  const frontendUrl = `http://localhost:${vitePort}`;
  const sidecarUrl = `http://127.0.0.1:${sidecarPort}`;

  if (!fs.existsSync(viteExecutable)) {
    throw new Error(`Missing Vite executable: ${viteExecutable}`);
  }

  console.log(`[dev] Starting sidecar: ${sidecarUrl}`);
  const sidecarChild = spawn(
    process.execPath,
    [path.join(projectRoot, 'server.js')],
    {
      cwd: projectRoot,
      stdio: ['ignore', 'inherit', 'inherit'],
      env: { ...env, PORT: String(sidecarPort) },
    },
  );

  const sidecarReady = await waitForPort(sidecarPort, 30000);
  if (!sidecarReady) {
    killTree(sidecarChild.pid);
    throw new Error(`Sidecar did not start within 30s: ${sidecarUrl}`);
  }

  console.log(`[dev] Starting Vite: ${frontendUrl}`);
  const viteChild = spawn(
    process.execPath,
    [viteExecutable, '--host', '0.0.0.0', '--port', String(vitePort), '--strictPort'],
    {
      cwd: projectRoot,
      stdio: ['ignore', 'inherit', 'inherit'],
      env: {
        ...env,
        VITE_API_BASE_URL: frontendUrl,
        VITE_FORCE_SERVER_FETCH: '1',
      },
    },
  );

  const viteReady = await waitForPort(vitePort, 30000);
  if (!viteReady) {
    killTree(sidecarChild.pid);
    killTree(viteChild.pid);
    throw new Error(`Vite dev server did not start within 30s: ${frontendUrl}`);
  }

  const apiReady = await waitForHttpReady(`${frontendUrl}/api/browser-session/browsers`, 30000);
  if (!apiReady) {
    killTree(sidecarChild.pid);
    killTree(viteChild.pid);
    throw new Error(`Vite API middleware did not become ready within 30s: ${frontendUrl}/api/browser-session/browsers`);
  }

  const wailsArgs = ['dev', '-m', '-s', `-frontenddevserverurl=${frontendUrl}`];
  for (const extraArg of rawArgs.slice(1)) {
    if (!wailsArgs.includes(extraArg)) {
      wailsArgs.push(extraArg);
    }
  }

  console.log(`[dev] Starting Wails: ${wailsArgs.join(' ')}`);
  const wailsChild = spawn(wailsExecutable, wailsArgs, {
    cwd: projectRoot,
    stdio: ['ignore', 'inherit', 'inherit'],
    env,
  });

  const cleanup = () => {
    killTree(sidecarChild.pid);
    killTree(viteChild.pid);
  };

  sidecarChild.on('exit', (code) => {
    if (code !== 0) {
      killTree(viteChild.pid);
      killTree(wailsChild.pid);
      process.exit(code ?? 1);
    }
  });

  viteChild.on('exit', (code) => {
    if (code !== 0) {
      killTree(sidecarChild.pid);
      killTree(wailsChild.pid);
      process.exit(code ?? 1);
    }
  });

  const shutdown = () => {
    cleanup();
    killTree(wailsChild.pid);
  };

  process.once('SIGINT', shutdown);
  process.once('SIGTERM', shutdown);

  attachExit(wailsChild, cleanup);
}

function buildPassthroughArgs() {
  const args = rawArgs.length ? [...rawArgs] : [command];
  if (args[0] === 'dev' && !args.includes('-m')) args.push('-m');
  return args;
}

async function main() {
  if (command === 'dev') {
    await runDevMode();
    return;
  }

  const child = spawn(resolveWailsExecutable(), buildPassthroughArgs(), {
    cwd: projectRoot,
    stdio: ['ignore', 'inherit', 'inherit'],
    env: buildEnv(),
  });

  attachExit(child);
}

main().catch((error) => {
  console.error(`[wails] ${error.message}`);
  process.exit(1);
});
