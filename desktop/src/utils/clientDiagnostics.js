import { getAdvancedProxyRoutingSnapshot } from './advancedProxyBridge.js';
import { loadPanelRecords } from './keyPanelStore.js';

function getAppBridge() {
  return window?.go?.main?.App;
}

const queue = [];
let flushTimer = null;
let sidebarRoutingDiagnosticsTimer = null;
let sidebarRoutingDiagnosticsLastPayload = '';
let bridgeUnavailableWarned = false;

function scheduleFlush() {
  if (flushTimer) {
    return;
  }
  flushTimer = setInterval(() => {
    const app = getAppBridge();
    if (!app?.AppendClientLog) {
      if (!bridgeUnavailableWarned) {
        bridgeUnavailableWarned = true;
        console.warn('[clientDiagnostics] AppendClientLog unavailable; buffering client logs until the Wails bridge is ready.');
      }
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
  logClientDiagnostic('bootstrap', `main.js start bridge=${String(Boolean(getAppBridge()?.AppendClientLog))}`);

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

function normalizeComparableSiteUrl(value) {
  return String(value || '').trim().replace(/\/+$/, '').toLowerCase();
}

function normalizeComparableName(value) {
  return String(value || '').trim().toLowerCase();
}

function doesRouteStateMatchRecord(record, routeState) {
  const rowKey = String(record?.rowKey || '').trim();
  if (!rowKey || !routeState || typeof routeState !== 'object') return false;

  const matchedProviderKey = String(routeState?.providerRowKey || routeState?.providerId || '').trim();
  if (matchedProviderKey && matchedProviderKey === rowKey) {
    return true;
  }

  const recordSiteUrl = normalizeComparableSiteUrl(record?.siteUrl);
  const targetUrl = normalizeComparableSiteUrl(routeState?.targetUrl);
  if (recordSiteUrl && targetUrl && (targetUrl === recordSiteUrl || targetUrl.startsWith(`${recordSiteUrl}/`))) {
    return true;
  }

  const providerName = normalizeComparableName(routeState?.providerName);
  const siteName = normalizeComparableName(record?.siteName);
  return Boolean(providerName && siteName && providerName === siteName);
}

async function emitSidebarRoutingDiagnostics(source = 'main') {
  try {
    const snapshot = await getAdvancedProxyRoutingSnapshot();
    const snapshotApps = snapshot?.apps && typeof snapshot.apps === 'object' ? snapshot.apps : {};
    const loaded = loadPanelRecords();
    const records = Array.isArray(loaded?.records) ? loaded.records : [];

    const appSummaries = Object.entries(snapshotApps).map(([appId, routeState]) => ({
      appId,
      providerId: String(routeState?.providerId || '').trim(),
      providerRowKey: String(routeState?.providerRowKey || '').trim(),
      providerName: String(routeState?.providerName || '').trim(),
      targetUrl: String(routeState?.targetUrl || '').trim(),
      status: String(routeState?.status || '').trim(),
      updatedAt: String(routeState?.updatedAt || '').trim(),
    }));

    const recordSummaries = records
      .filter(record => Number(record?.status || 0) === 1)
      .map(record => {
        const matches = Object.entries(snapshotApps).map(([appId, routeState]) => ({
          appId,
          match: doesRouteStateMatchRecord(record, routeState),
          providerId: String(routeState?.providerId || '').trim(),
          providerRowKey: String(routeState?.providerRowKey || '').trim(),
          providerName: String(routeState?.providerName || '').trim(),
          targetUrl: String(routeState?.targetUrl || '').trim(),
        }));
        return {
          siteName: String(record?.siteName || '').trim(),
          siteUrl: String(record?.siteUrl || '').trim(),
          rowKey: String(record?.rowKey || '').trim(),
          matchedApps: matches.filter(item => item.match).map(item => item.appId),
          matches,
        };
      });

    const payload = JSON.stringify({
      source,
      snapshotApps: appSummaries,
      visibleRecords: recordSummaries,
    });
    if (payload === sidebarRoutingDiagnosticsLastPayload) {
      return;
    }
    sidebarRoutingDiagnosticsLastPayload = payload;
    logClientDiagnostic('sidebar.routing', payload);
  } catch (error) {
    logClientDiagnostic('sidebar.routing.error', error?.stack || error?.message || String(error || 'unknown error'));
  }
}

export function installSidebarRoutingDiagnostics(source = 'main') {
  if (sidebarRoutingDiagnosticsTimer) {
    return;
  }
  void emitSidebarRoutingDiagnostics(source);
  sidebarRoutingDiagnosticsTimer = window.setInterval(() => {
    void emitSidebarRoutingDiagnostics(source);
  }, 2000);
}
