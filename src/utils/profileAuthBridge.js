import { isProbablyWailsRuntime } from './runtimeApi.js';

function getAppBridge() {
  return window?.go?.main?.App;
}

export function isChromeProfileAuthBridgeAvailable() {
  const app = getAppBridge();
  return Boolean(
    isProbablyWailsRuntime() &&
      app &&
      typeof app.ExtractChromeProfileTokens === 'function'
  );
}

export async function extractChromeProfileTokens(accounts) {
  const app = getAppBridge();
  if (!app?.ExtractChromeProfileTokens) {
    throw new Error('当前运行环境不支持 Chrome Profile 文件提取，请在桌面端 EXE 中使用');
  }
  return app.ExtractChromeProfileTokens({ accounts });
}
