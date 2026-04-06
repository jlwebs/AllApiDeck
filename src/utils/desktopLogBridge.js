import { isProbablyWailsRuntime } from './runtimeApi.js';

function getAppBridge() {
  return window?.go?.main?.App;
}

export function isDesktopLogBridgeAvailable() {
  const app = getAppBridge();
  return Boolean(
    isProbablyWailsRuntime() &&
      app &&
      typeof app.ListDesktopLogFiles === 'function' &&
      typeof app.ReadDesktopLogFile === 'function'
  );
}

export async function listDesktopLogFiles() {
  const app = getAppBridge();
  if (!app?.ListDesktopLogFiles) {
    throw new Error('当前运行环境不支持桌面端日志查看，请在 EXE 中使用');
  }
  return app.ListDesktopLogFiles();
}

export async function readDesktopLogFile(path) {
  const app = getAppBridge();
  if (!app?.ReadDesktopLogFile) {
    throw new Error('当前运行环境不支持桌面端日志查看，请在 EXE 中使用');
  }
  return app.ReadDesktopLogFile(path);
}
