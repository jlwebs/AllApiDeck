function getAppBridge() {
  return window?.go?.main?.App;
}

const queue = [];
let flushTimer = null;

function scheduleFlush() {
  if (flushTimer) {
    return;
  }
  flushTimer = setInterval(() => {
    const app = getAppBridge();
    if (!app?.AppendClientLog) {
      return;
    }
    while (queue.length > 0) {
      const item = queue.shift();
      try {
        app.AppendClientLog(item.scope, item.message);
      } catch {}
    }
    clearInterval(flushTimer);
    flushTimer = null;
  }, 400);
}

export function logClientDiagnostic(scope, message) {
  queue.push({
    scope: String(scope || 'client').trim() || 'client',
    message: String(message || '').trim(),
  });
  scheduleFlush();
}

export function installClientDiagnostics() {
  logClientDiagnostic('bootstrap', 'main.js start');

  window.addEventListener('error', (event) => {
    const text = [
      event?.message || 'unknown error',
      event?.filename ? `file=${event.filename}` : '',
      Number.isFinite(event?.lineno) ? `line=${event.lineno}` : '',
      Number.isFinite(event?.colno) ? `col=${event.colno}` : '',
    ].filter(Boolean).join(' | ');
    logClientDiagnostic('window.error', text);
  });

  window.addEventListener('unhandledrejection', (event) => {
    const reason = event?.reason;
    const text = reason?.stack || reason?.message || String(reason || 'unknown rejection');
    logClientDiagnostic('unhandledrejection', text);
  });

  window.addEventListener('load', () => {
    logClientDiagnostic('window.load', 'window load event fired');
  }, { once: true });
}
