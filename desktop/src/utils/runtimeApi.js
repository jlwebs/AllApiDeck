const EXPLICIT_API_BASE_URL = String(import.meta.env.VITE_API_BASE_URL || '')
  .trim()
  .replace(/\/+$/, '');

function getAppBridge() {
  return window?.go?.main?.App;
}

export function isProbablyWailsRuntime() {
  return typeof window !== 'undefined' && (
    window.location.protocol === 'wails:' ||
    window.location.protocol === 'asset:' ||
    window.location.hostname === 'wails.localhost' ||
    typeof window.go === 'object' ||
    typeof window.runtime === 'object'
  );
}

export function isWailsHttpBridgeAvailable() {
  const app = getAppBridge();
  return Boolean(
    isProbablyWailsRuntime() &&
      app &&
      (
        typeof app.PerformHttpRequestRaw === 'function' ||
        typeof app.PerformHttpRequest === 'function'
      )
  );
}

export function getRuntimeApiBaseUrl() {
  if (isProbablyWailsRuntime() && isWailsHttpBridgeAvailable()) {
    return '';
  }

  if (EXPLICIT_API_BASE_URL) {
    return EXPLICIT_API_BASE_URL;
  }

  if (isProbablyWailsRuntime()) {
    return isWailsHttpBridgeAvailable() ? '' : 'http://127.0.0.1:3000';
  }

  return '';
}

export function resolveRuntimeApiUrl(input) {
  if (typeof input !== 'string') {
    return input;
  }

  if (!input.startsWith('/api/')) {
    return input;
  }

  const baseUrl = getRuntimeApiBaseUrl();
  return baseUrl ? `${baseUrl}${input}` : input;
}

function shouldBridgeRequest(url) {
  if (!isWailsHttpBridgeAvailable() || typeof url !== 'string') {
    return false;
  }

  if (url.startsWith('/api/')) {
    return true;
  }

  return /^https?:\/\//i.test(url);
}

function normalizeHeaders(rawHeaders) {
  const pairs = [];
  if (!rawHeaders) {
    return {};
  }

  if (rawHeaders instanceof Headers) {
    rawHeaders.forEach((value, key) => pairs.push([key, value]));
  } else if (Array.isArray(rawHeaders)) {
    rawHeaders.forEach(([key, value]) => pairs.push([key, value]));
  } else if (typeof rawHeaders === 'object') {
    Object.entries(rawHeaders).forEach(([key, value]) => pairs.push([key, value]));
  }

  return pairs.reduce((acc, [key, value]) => {
    if (typeof key === 'string' && value != null) {
      acc[key] = String(value);
    }
    return acc;
  }, {});
}

class BridgeHeaders {
  constructor(rawHeaders) {
    this.map = new Map();
    Object.entries(rawHeaders || {}).forEach(([key, value]) => {
      this.map.set(String(key).toLowerCase(), String(value));
    });
  }

  get(name) {
    return this.map.get(String(name || '').toLowerCase()) || null;
  }

  has(name) {
    return this.map.has(String(name || '').toLowerCase());
  }
}

class BridgeResponse {
  constructor(payload) {
    this.status = Number(payload?.status || 0);
    this.ok = this.status >= 200 && this.status < 300;
    this.headers = new BridgeHeaders(payload?.headers || {});
    this._body = String(payload?.body || '');
  }

  async json() {
    return JSON.parse(this._body || 'null');
  }

  async text() {
    return this._body;
  }
}

async function buildBridgeRequest(input, init = {}) {
  const requestLike = typeof Request !== 'undefined' && input instanceof Request ? input : null;
  const url = requestLike
    ? requestLike.url
    : input instanceof URL
      ? input.toString()
      : String(input);

  const method = String(init?.method || requestLike?.method || 'GET').toUpperCase();
  const headers = {
    ...normalizeHeaders(requestLike?.headers),
    ...normalizeHeaders(init?.headers),
  };

  let body = '';
  if (typeof init?.body === 'string') {
    body = init.body;
  } else if (init?.body != null) {
    body = String(init.body);
  } else if (requestLike && method !== 'GET' && method !== 'HEAD') {
    body = await requestLike.clone().text().catch(() => '');
  }

  return {
    url,
    method,
    headers,
    body,
    timeoutMs: Number(init?.timeoutMs || 0),
  };
}

function createAbortError() {
  return new DOMException('The operation was aborted.', 'AbortError');
}

async function bridgeFetch(nativeFetch, input, init = {}) {
  const requestLike = typeof Request !== 'undefined' && input instanceof Request ? input : null;
  const rawUrl = requestLike
    ? requestLike.url
    : input instanceof URL
      ? input.toString()
      : String(input);

  if (!shouldBridgeRequest(rawUrl)) {
    return nativeFetch(input, init);
  }

  const app = getAppBridge();
  if (!app?.PerformHttpRequest) {
    return nativeFetch(input, init);
  }

  const signal = init?.signal || requestLike?.signal;
  if (signal?.aborted) {
    throw createAbortError();
  }

  const payload = await buildBridgeRequest(input, init);
  const invokeBridge = typeof app.PerformHttpRequestRaw === 'function'
    ? () => app.PerformHttpRequestRaw(JSON.stringify(payload))
    : () => app.PerformHttpRequest(
      payload.method,
      payload.url,
      JSON.stringify(payload.headers || {}),
      payload.body || '',
      Number(payload.timeoutMs || 0),
    );

  const bridgePromise = Promise.resolve(invokeBridge())
    .then(result => new BridgeResponse(JSON.parse(String(result || '{}'))));

  if (!signal) {
    return bridgePromise;
  }

  const abortPromise = new Promise((_, reject) => {
    signal.addEventListener('abort', () => reject(createAbortError()), { once: true });
  });

  return Promise.race([bridgePromise, abortPromise]);
}

function getNativeFetch() {
  if (typeof window !== 'undefined' && typeof window.__batchApiCheckNativeFetch === 'function') {
    return window.__batchApiCheckNativeFetch;
  }
  if (typeof window !== 'undefined' && typeof window.fetch === 'function') {
    return window.fetch.bind(window);
  }
  if (typeof globalThis !== 'undefined' && typeof globalThis.fetch === 'function') {
    return globalThis.fetch.bind(globalThis);
  }
  throw new Error('Native fetch is unavailable');
}

export function runtimeFetch(input, init) {
  return bridgeFetch(getNativeFetch(), input, init);
}

export function openUrlInSystemBrowser(url, target = '_blank') {
  const normalizedUrl = typeof url === 'string' ? url.trim() : '';
  if (!normalizedUrl) return;
  if (isProbablyWailsRuntime() && typeof window !== 'undefined' && typeof window.runtime?.BrowserOpenURL === 'function') {
    try {
      window.runtime.BrowserOpenURL(normalizedUrl);
      return;
    } catch {}
  }
  window.open(normalizedUrl, target, 'noopener');
}

export function installRuntimeFetchBridge() {
  if (typeof window === 'undefined') {
    return;
  }
  if (!window.__batchApiCheckNativeFetch && typeof window.fetch === 'function') {
    window.__batchApiCheckNativeFetch = window.fetch.bind(window);
  }
  const nativeFetch = window.__batchApiCheckNativeFetch;
  if (typeof nativeFetch !== 'function') {
    return;
  }

  const bridgedFetch = (input, init) => bridgeFetch(nativeFetch, input, init);
  const applyBridge = () => {
    try { window.fetch = bridgedFetch; } catch {}
    try { globalThis.fetch = bridgedFetch; } catch {}
    try {
      if (typeof self !== 'undefined') {
        self.fetch = bridgedFetch;
      }
    } catch {}
    window.__batchApiCheckFetchBridgeInstalled = true;
  };

  applyBridge();
  setTimeout(applyBridge, 0);
  setTimeout(applyBridge, 300);
  setTimeout(applyBridge, 1500);
  if (typeof window.addEventListener === 'function') {
    window.addEventListener('load', applyBridge, { once: true });
  }
}

export function apiFetch(input, init) {
  return runtimeFetch(resolveRuntimeApiUrl(input), init);
}
