import { isProbablyWailsRuntime } from './runtimeApi.js';

function getAppBridge() {
  return window?.go?.main?.App;
}

export function isDesktopConfigBridgeAvailable() {
  const app = getAppBridge();
  return Boolean(
    isProbablyWailsRuntime() &&
      app &&
      typeof app.ReadManagedAppConfigFiles === 'function' &&
      typeof app.ApplyManagedAppConfigFiles === 'function'
  );
}

export async function readManagedAppConfigFiles(appIds) {
  const app = getAppBridge();
  if (!app?.ReadManagedAppConfigFiles) {
    throw new Error('当前运行环境不支持本地配置读写，请在桌面版 EXE 中使用');
  }
  return app.ReadManagedAppConfigFiles(appIds);
}

export async function applyManagedAppConfigFiles(files) {
  const app = getAppBridge();
  if (!app?.ApplyManagedAppConfigFiles) {
    throw new Error('当前运行环境不支持本地配置写入，请在桌面版 EXE 中使用');
  }
  return app.ApplyManagedAppConfigFiles({ files });
}
