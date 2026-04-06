import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { build, createServer, mergeConfig, preview } from 'vite';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '..');
const rawArgs = process.argv.slice(2);

function parseArgs(argv) {
  const command = argv[0] && !argv[0].startsWith('-') ? argv[0] : 'dev';
  const optionArgs = command === 'dev' ? argv : argv.slice(1);
  const options = {};

  for (let i = 0; i < optionArgs.length; i += 1) {
    const arg = optionArgs[i];
    if (!arg.startsWith('--')) continue;

    const key = arg.slice(2);
    const next = optionArgs[i + 1];
    if (!next || next.startsWith('--')) {
      options[key] = true;
      continue;
    }

    options[key] = next;
    i += 1;
  }

  return { command, options };
}

async function loadUserConfig(command, mode) {
  const configUrl = `${pathToFileURL(path.join(projectRoot, 'vite.config.js')).href}?t=${Date.now()}`;
  const configModule = await import(configUrl);
  const exported = configModule.default ?? {};
  if (typeof exported === 'function') {
    return await exported({
      command: command === 'build' ? 'build' : 'serve',
      mode,
      isPreview: command === 'preview',
      isSsrBuild: false,
    });
  }
  return exported;
}

function toNumber(value, fallback) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

async function run() {
  const { command, options } = parseArgs(rawArgs);
  const mode = typeof options.mode === 'string' ? options.mode : undefined;
  const userConfig = await loadUserConfig(command, mode);

  if (command === 'build') {
    const config = mergeConfig(userConfig, {
      configFile: false,
      root: projectRoot,
      mode,
      logLevel: options.logLevel,
      clearScreen: options.clearScreen,
    });
    await build(config);
    return;
  }

  if (command === 'preview') {
    const config = mergeConfig(userConfig, {
      configFile: false,
      root: projectRoot,
      mode,
      logLevel: options.logLevel,
      clearScreen: options.clearScreen,
      preview: {
        host: options.host,
        port: toNumber(options.port, 4173),
        strictPort: Boolean(options.strictPort),
        open: options.open === true ? true : options.open || false,
      },
    });
    const server = await preview(config);
    server.printUrls();
    return;
  }

  const devConfig = mergeConfig(userConfig, {
    configFile: false,
    root: projectRoot,
    mode,
    logLevel: options.logLevel,
    clearScreen: options.clearScreen,
    optimizeDeps: {
      ...(userConfig.optimizeDeps || {}),
      force: true,
      include: Array.from(new Set([
        ...((userConfig.optimizeDeps && Array.isArray(userConfig.optimizeDeps.include))
          ? userConfig.optimizeDeps.include
          : []),
        'vue',
        'vue-router',
        'vue-i18n',
        'ant-design-vue',
        '@ant-design/icons-vue',
        'echarts',
      ])),
    },
    server: {
      ...(userConfig.server || {}),
      host: options.host || userConfig.server?.host,
      port: toNumber(options.port, userConfig.server?.port || 3000),
      strictPort: Boolean(options.strictPort),
      open: options.open === true ? true : options.open || false,
      cors: Boolean(options.cors),
    },
  });

  const server = await createServer(devConfig);
  await server.listen();
  server.printUrls();
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
