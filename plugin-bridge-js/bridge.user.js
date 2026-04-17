// ==UserScript==
// @name         All API Deck Local Bridge Import
// @namespace    http://tampermonkey.net/
// @version      0.2.5
// @description  当前标签页桥接导入：显式确认后提取站点登录态候选、账号信息与站内 key 列表，并发送到本地 All API Deck 进程。
// @author       All API Deck
// @match        http://127.0.0.1/*
// @match        https://*/*
// @grant        GM_xmlhttpRequest
// @connect      127.0.0.1
// @run-at       document-start
// ==/UserScript==

(function () {
  'use strict';

  const receiverBase = 'http://127.0.0.1:8888';
  const bridgeVersion = '0.2.4';
  const executionId = `bridge-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
  const phaseLogs = [];
  const nativeFetch = typeof window.fetch === 'function' ? window.fetch.bind(window) : null;
  const observedBridgeState = {
    authCandidates: [],
    userIdCandidates: [],
    tokenSnapshots: [],
    responseTraces: [],
  };

  const TOKEN_HINT_RE = /(access[_-]?token|auth[_-]?token|id[_-]?token|jwt|token|authorization|bearer|session|sess|login)/i;
  const USER_ID_HINT_RE = /(user[_-]?id|userid|uid|account[_-]?id|member[_-]?id)/i;
  const API_KEY_HINT_RE = /(api[_-]?key|sk[_-]?key|secret[_-]?key)/i;
  const CLIENT_ID_HINT_RE = /(client[_-]?id|app[_-]?id|oauth[_-]?client)/i;
  const bridgePanelState = {
    busy: true,
    line1: '检测中...',
    relayText: '检测中',
    submittedText: '未提交',
    tone: 'pending',
    detail: '等待页面稳定后开始分析',
  };
  let bridgePanelRoot = null;
  let bridgePanelMounted = false;
  let bridgePanelSuppressed = true;

  function removeBridgePanel() {
    if (bridgePanelRoot && bridgePanelRoot.parentNode) {
      bridgePanelRoot.parentNode.removeChild(bridgePanelRoot);
    }
    bridgePanelRoot = null;
    bridgePanelMounted = false;
  }

  function escapeHtml(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function ensureBridgePanelMounted() {
    if (bridgePanelMounted && bridgePanelRoot && document.contains(bridgePanelRoot)) {
      return bridgePanelRoot;
    }
    if (!document.body) {
      setTimeout(ensureBridgePanelMounted, 150);
      return null;
    }

    bridgePanelRoot = document.createElement('div');
    bridgePanelRoot.id = 'all-api-deck-bridge-panel';
    bridgePanelRoot.style.cssText = [
      'position:fixed',
      'right:14px',
      'top:50%',
      'transform:translateY(-50%)',
      'z-index:2147483647',
      'pointer-events:none',
      'user-select:none',
      'font-family:Segoe UI, PingFang SC, Microsoft YaHei, sans-serif',
    ].join(';');
    document.body.appendChild(bridgePanelRoot);
    bridgePanelMounted = true;
    renderBridgePanel();
    return bridgePanelRoot;
  }

  function renderBridgePanel() {
    if (bridgePanelSuppressed) {
      removeBridgePanel();
      return;
    }
    const root = ensureBridgePanelMounted();
    if (!root) return;
    const toneMap = {
      pending: {
        glow: 'rgba(250,204,21,.34)',
        border: 'rgba(250,204,21,.36)',
        bg: 'linear-gradient(180deg, rgba(17,24,39,.92), rgba(31,41,55,.9))',
      },
      success: {
        glow: 'rgba(34,197,94,.34)',
        border: 'rgba(34,197,94,.34)',
        bg: 'linear-gradient(180deg, rgba(9,24,17,.94), rgba(20,46,33,.92))',
      },
      danger: {
        glow: 'rgba(248,113,113,.32)',
        border: 'rgba(248,113,113,.3)',
        bg: 'linear-gradient(180deg, rgba(39,18,18,.94), rgba(58,24,24,.9))',
      },
    };
    const tone = toneMap[bridgePanelState.tone] || toneMap.pending;
    const spinnerMarkup = bridgePanelState.busy
      ? '<span class="aad-bridge-spinner"></span>'
      : '<span class="aad-bridge-spinner aad-bridge-spinner-idle"></span>';

    const relaySentence = bridgePanelState.relayText === '检测中'
      ? '当前网站是否为中转站：检测中'
      : `当前网站${bridgePanelState.relayText}中转站`;

    root.innerHTML = `
      <div class="aad-bridge-card">
        <style>
          #all-api-deck-bridge-panel .aad-bridge-card{
            width:224px;
            padding:12px 14px;
            border-radius:16px;
            background:${tone.bg};
            border:1px solid ${tone.border};
            box-shadow:0 12px 32px ${tone.glow}, inset 0 1px 0 rgba(255,255,255,.06);
            color:#f8fafc;
            backdrop-filter:blur(14px);
          }
          #all-api-deck-bridge-panel .aad-bridge-line1{
            display:flex;
            align-items:center;
            gap:8px;
            min-width:0;
            font-size:13px;
            font-weight:700;
            color:#f8fafc;
          }
          #all-api-deck-bridge-panel .aad-bridge-line2{
            margin-top:8px;
            font-size:12px;
            line-height:1.55;
            color:rgba(255,255,255,.84);
            word-break:break-word;
          }
          #all-api-deck-bridge-panel .aad-bridge-detail{
            margin-top:6px;
            font-size:11px;
            line-height:1.45;
            color:rgba(255,255,255,.62);
            word-break:break-word;
          }
          #all-api-deck-bridge-panel .aad-bridge-spinner{
            width:12px;
            height:12px;
            border-radius:999px;
            border:2px solid rgba(255,255,255,.22);
            border-top-color:#facc15;
            display:inline-block;
            animation:aad-bridge-spin .9s linear infinite;
            flex:0 0 auto;
          }
          #all-api-deck-bridge-panel .aad-bridge-spinner-idle{
            border-color:rgba(255,255,255,.18);
            border-top-color:rgba(255,255,255,.72);
            animation:none;
          }
          @keyframes aad-bridge-spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
        </style>
        <div class="aad-bridge-line1">${spinnerMarkup}<span>${escapeHtml(bridgePanelState.line1)}</span></div>
        <div class="aad-bridge-line2">${escapeHtml(relaySentence)}，提交状态：${escapeHtml(bridgePanelState.submittedText)}</div>
        <div class="aad-bridge-detail">${escapeHtml(bridgePanelState.detail)}</div>
      </div>
    `;
  }

  function updateBridgePanel(patch) {
    Object.assign(bridgePanelState, patch || {});
    renderBridgePanel();
  }

  function nowIso() {
    return new Date().toISOString();
  }

  function safeString(value) {
    return String(value == null ? '' : value).trim();
  }

  function previewText(value, limit = 120) {
    const text = safeString(value).replace(/\s+/g, ' ');
    if (!text) return '';
    return text.length > limit ? `${text.slice(0, limit)}...(truncated)` : text;
  }

  function maskSecret(value, head = 10, tail = 4) {
    const text = safeString(value);
    if (!text) return '';
    if (text.length <= head + tail + 3) return `${text.slice(0, 3)}***`;
    return `${text.slice(0, head)}...${text.slice(-tail)}`;
  }

  function logPhase(stage, detail, extra) {
    const entry = {
      at: nowIso(),
      stage,
      detail: safeString(detail),
      extra: extra && typeof extra === 'object' ? extra : {},
    };
    phaseLogs.push(entry);
    if (phaseLogs.length > 36) phaseLogs.shift();
    const preview = Object.keys(entry.extra).length ? ` | ${JSON.stringify(entry.extra)}` : '';
    console.log(`[AllApiDeck Bridge][${stage}] ${entry.detail}${preview}`);
    return entry;
  }

  function request(method, url, payload) {
    return new Promise((resolve, reject) => {
      const rawPayload = payload ? JSON.stringify(payload) : '';
      logPhase('request:start', `${method} ${url}`, { payloadBytes: rawPayload.length });
      GM_xmlhttpRequest({
        method,
        url,
        data: rawPayload || undefined,
        headers: {
          'X-AllApiDeck-Bridge-Client': 'userscript',
          ...(payload ? { 'Content-Type': 'application/json' } : {}),
        },
        timeout: 10000,
        onload: response => {
          logPhase('request:done', `${method} ${url}`, {
            status: response.status,
            responseBytes: safeString(response.responseText).length,
          });
          resolve(response);
        },
        onerror: error => {
          logPhase('request:error', `${method} ${url}`, {
            error: safeString(error?.error || error?.message || 'unknown_error'),
          });
          reject(error);
        },
        ontimeout: () => {
          logPhase('request:timeout', `${method} ${url}`, { timeoutMs: 10000 });
          reject(new Error(`request timeout: ${method} ${url}`));
        },
      });
    });
  }

  function safeJsonParse(text) {
    try {
      return JSON.parse(text);
    } catch {
      return null;
    }
  }

  function decodeBase64UrlSegment(segment) {
    try {
      const normalized = String(segment || '').replace(/-/g, '+').replace(/_/g, '/');
      const padding = normalized.length % 4 === 0 ? '' : '='.repeat(4 - (normalized.length % 4));
      return atob(normalized + padding);
    } catch {
      return '';
    }
  }

  function tryDecodeJwtPayload(token) {
    const text = safeString(token);
    const parts = text.split('.');
    if (parts.length < 2) return null;
    const decoded = decodeBase64UrlSegment(parts[1]);
    if (!decoded) return null;
    return safeJsonParse(decoded);
  }

  function isLikelyJwt(token) {
    const payload = tryDecodeJwtPayload(token);
    return Boolean(payload && typeof payload === 'object');
  }

  function isJwtExpired(token) {
    const payload = tryDecodeJwtPayload(token);
    const exp = Number(payload?.exp || 0);
    if (!Number.isFinite(exp) || exp <= 0) return false;
    return exp * 1000 <= Date.now();
  }

  function isLikelyTokenCandidate(value) {
    const text = safeString(value);
    if (!text || /\s/.test(text)) return false;
    if (text.length < 24) return false;
    if (/^sk-[A-Za-z0-9]/.test(text)) return false;
    if (/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(text)) return true;
    return /^[A-Za-z0-9._~+/=-]{24,}$/.test(text);
  }

  function isLikelyUserIdCandidate(value) {
    const text = safeString(value);
    if (!text) return false;
    if (/^\d{1,18}$/.test(text)) return true;
    if (/^[0-9a-f]{8}-[0-9a-f-]{27,}$/i.test(text)) return true;
    return false;
  }

  function makeSetArray(values) {
    return Array.from(new Set((Array.isArray(values) ? values : []).map(item => safeString(item)).filter(Boolean)));
  }

  function pushLimited(array, item, limit = 60) {
    array.push(item);
    if (array.length > limit) array.splice(0, array.length - limit);
  }

  function parseUrlMaybe(rawUrl) {
    try {
      return new URL(rawUrl, window.location.href);
    } catch {
      return null;
    }
  }

  function isSameOriginUrl(rawUrl) {
    const parsed = parseUrlMaybe(rawUrl);
    return Boolean(parsed && parsed.origin === window.location.origin);
  }

  function normalizeHeadersObject(input) {
    const result = {};
    if (!input) return result;
    if (typeof Headers !== 'undefined' && input instanceof Headers) {
      input.forEach((value, key) => {
        result[key] = value;
      });
      return result;
    }
    if (Array.isArray(input)) {
      input.forEach(entry => {
        if (!Array.isArray(entry) || entry.length < 2) return;
        result[String(entry[0])] = String(entry[1]);
      });
      return result;
    }
    if (typeof input === 'object') {
      Object.entries(input).forEach(([key, value]) => {
        result[String(key)] = String(value);
      });
    }
    return result;
  }

  function extractBearerTokenFromHeaders(headersObject) {
    const auth = safeString(headersObject?.Authorization || headersObject?.authorization);
    if (/^Bearer\s+/i.test(auth)) {
      return auth.replace(/^Bearer\s+/i, '').trim();
    }
    return '';
  }

  function scoreTokenCandidate(entry) {
    const keyName = safeString(entry?.keyName);
    const token = safeString(entry?.value);
    let score = 0;
    if (TOKEN_HINT_RE.test(keyName)) score += 80;
    if (/access[_-]?token|auth[_-]?token|id[_-]?token/i.test(keyName)) score += 40;
    if (API_KEY_HINT_RE.test(keyName)) score -= 40;
    if (CLIENT_ID_HINT_RE.test(keyName)) score -= 160;
    if (isLikelyJwt(token)) score += 60;
    if (/^Bearer\s+/i.test(token)) score += 25;
    if (String(entry?.source || '').includes('observed-fetch-auth')) score += 420;
    if (String(entry?.source || '').includes('observed-xhr-auth')) score += 420;
    if (String(entry?.path || '').includes('/api/')) score += 70;
    if (String(entry?.path || '').includes('/chat/')) score += 18;
    if (String(entry?.source || '').includes('cookie')) score += 10;
    if (String(entry?.path || '').includes('account')) score += 8;
    if (token.length >= 80) score += 6;
    return score;
  }

  function scoreUserIdCandidate(entry) {
    const keyName = safeString(entry?.keyName);
    const value = safeString(entry?.value);
    let score = 0;
    if (USER_ID_HINT_RE.test(keyName)) score += 80;
    if (/^\d+$/.test(value)) score += 15;
    if (String(entry?.source || '').includes('observed-')) score += 160;
    if (String(entry?.path || '').includes('account')) score += 8;
    return score;
  }

  function extractCandidateFromString(value, meta, bucket) {
    const text = safeString(value);
    if (!text) return;
    const keyName = safeString(meta?.keyName || meta?.path || '');
    if (isLikelyTokenCandidate(text)) {
      bucket.tokenCandidates.push({
        source: safeString(meta?.source),
        storage: safeString(meta?.storage),
        keyName,
        path: safeString(meta?.path),
        value: text,
        preview: maskSecret(text),
      });
    }
    if (isLikelyUserIdCandidate(text) && USER_ID_HINT_RE.test(keyName)) {
      bucket.userIdCandidates.push({
        source: safeString(meta?.source),
        storage: safeString(meta?.storage),
        keyName,
        path: safeString(meta?.path),
        value: text,
      });
    }
  }

  function walkCandidateValue(value, meta, bucket, depth = 0) {
    if (depth > 4 || value == null) return;
    if (typeof value === 'string') {
      extractCandidateFromString(value, meta, bucket);
      const parsed = value.length <= 8192 ? safeJsonParse(value) : null;
      if (parsed && typeof parsed === 'object') {
        walkCandidateValue(parsed, meta, bucket, depth + 1);
      }
      return;
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      extractCandidateFromString(String(value), meta, bucket);
      return;
    }
    if (Array.isArray(value)) {
      value.slice(0, 30).forEach((item, index) => {
        walkCandidateValue(item, {
          ...meta,
          path: `${safeString(meta?.path || meta?.keyName || 'value')}[${index}]`,
        }, bucket, depth + 1);
      });
      return;
    }
    if (typeof value === 'object') {
      Object.entries(value).slice(0, 40).forEach(([key, item]) => {
        const nextPath = safeString(meta?.path)
          ? `${safeString(meta.path)}.${key}`
          : key;
        walkCandidateValue(item, {
          ...meta,
          keyName: key,
          path: nextPath,
        }, bucket, depth + 1);
      });
    }
  }

  function collectStorageEntries(storage, storageName) {
    const entries = [];
    try {
      for (let index = 0; index < storage.length; index += 1) {
        const key = storage.key(index);
        if (!key) continue;
        const value = storage.getItem(key);
        entries.push({
          storage: storageName,
          key,
          value,
          preview: previewText(value, 180),
        });
      }
    } catch (error) {
      updateBridgePanel({
        busy: false,
        line1: '提交失败',
        submittedText: '未提交',
        tone: 'danger',
        detail: safeString(error?.message || error?.error || error) || '未知错误',
      });
      updateBridgePanel({
        busy: false,
        line1: '提交失败',
        submittedText: '未提交',
        tone: 'danger',
        detail: safeString(error?.message || error?.error || error) || '未知错误',
      });
      logPhase('storage:error', `读取 ${storageName} 失败`, { error: safeString(error?.message || error) });
    }
    return entries;
  }

  function collectCookieEntries() {
    return safeString(document.cookie)
      .split(';')
      .map(item => item.trim())
      .filter(Boolean)
      .map(raw => {
        const separatorIndex = raw.indexOf('=');
        if (separatorIndex === -1) {
          return { key: raw, value: '', preview: '' };
        }
        const key = raw.slice(0, separatorIndex).trim();
        const value = raw.slice(separatorIndex + 1).trim();
        return {
          key,
          value,
          preview: previewText(value, 120),
        };
      });
  }

  function collectGlobalBootstrapValues(bucket) {
    const globalsToInspect = [
      '__NEXT_DATA__',
      '__NUXT__',
      '__INITIAL_STATE__',
      '__APP_STATE__',
      '__PINIA__',
      '__APOLLO_STATE__',
      '__PRELOADED_STATE__',
    ];
    globalsToInspect.forEach(name => {
      try {
        if (!(name in window)) return;
        walkCandidateValue(window[name], {
          source: 'window-global',
          storage: 'window',
          keyName: name,
          path: name,
        }, bucket, 0);
      } catch (error) {
        logPhase('window:inspect:error', `读取 window.${name} 失败`, {
          error: safeString(error?.message || error),
        });
      }
    });
  }

  function recordObservedUserId(value, meta = {}) {
    const text = safeString(value);
    if (!isLikelyUserIdCandidate(text)) return;
    pushLimited(observedBridgeState.userIdCandidates, {
      source: safeString(meta?.source || 'observed'),
      storage: 'runtime',
      keyName: safeString(meta?.keyName || 'user_id'),
      path: safeString(meta?.path || ''),
      value: text,
    }, 24);
  }

  function recordObservedAuthToken(token, meta = {}) {
    const text = safeString(token);
    if (!isLikelyTokenCandidate(text)) return;
    pushLimited(observedBridgeState.authCandidates, {
      source: safeString(meta?.source || 'observed-auth'),
      storage: 'runtime',
      keyName: safeString(meta?.keyName || 'authorization'),
      path: safeString(meta?.path || ''),
      value: text,
      preview: maskSecret(text),
    }, 24);
  }

  function detectObservedSiteTypeFromPath(pathname) {
    const path = safeString(pathname);
    if (path.startsWith('/api/v1/keys')) return 'sub2api';
    if (path.includes('/api/token')) return '';
    return '';
  }

  function recordObservedTokenSnapshot(meta = {}) {
    const tokens = normalizeBridgeImportedTokensForObservation(meta?.tokens);
    if (!tokens.length) return;
    pushLimited(observedBridgeState.tokenSnapshots, {
      source: safeString(meta?.source || 'observed-response'),
      endpoint: safeString(meta?.endpoint || ''),
      siteType: safeString(meta?.siteType || ''),
      tokenCount: tokens.length,
      tokens,
    }, 10);
  }

  function normalizeBridgeImportedTokensForObservation(tokens) {
    const dedupe = new Map();
    (Array.isArray(tokens) ? tokens : []).forEach((token, index) => {
      const key = safeString(
        token?.key ||
        token?.access_token ||
        token?.token ||
        token?.api_key ||
        token?.apikey ||
        (typeof token === 'string' ? token : '')
      );
      if (!key) return;
      dedupe.set(key, {
        ...token,
        key,
        access_token: key,
        name: safeString(token?.name || token?.token_name || `Observed Token ${index + 1}`) || `Observed Token ${index + 1}`,
        source: safeString(token?.source || 'observed') || 'observed',
        status: token?.status ?? 1,
      });
    });
    return Array.from(dedupe.values());
  }

  function inspectObservedJsonPayload(rawUrl, payload, source) {
    const parsedUrl = parseUrlMaybe(rawUrl);
    const path = safeString(parsedUrl?.pathname || '');
    const traceEntry = `[${source}] ${path || rawUrl}`;
    pushLimited(observedBridgeState.responseTraces, traceEntry, 40);

    const userIds = collectUserIdCandidatesFromPayload(payload);
    userIds.forEach(userId => {
      recordObservedUserId(userId, {
        source: `observed-${source}-response`,
        keyName: 'user_id',
        path,
      });
    });

    const listItems = extractListItems(payload);
    if (listItems.length && (/\/api\/token/.test(path) || /\/api\/v1\/keys/.test(path))) {
      recordObservedTokenSnapshot({
        source: `observed-${source}-response`,
        endpoint: path,
        siteType: detectObservedSiteTypeFromPath(path),
        tokens: listItems,
      });
    }
  }

  function inspectObservedResponse(rawUrl, response, source) {
    const parsedUrl = parseUrlMaybe(rawUrl);
    if (!parsedUrl || parsedUrl.origin !== window.location.origin) return;
    const contentType = safeString(response?.headers?.get && response.headers.get('content-type'));
    if (!/json/i.test(contentType)) return;
    try {
      response.clone().json()
        .then(payload => inspectObservedJsonPayload(parsedUrl.href, payload, source))
        .catch(() => {});
    } catch {}
  }

  function installRuntimeObservers() {
    if (window.__allApiDeckBridgeObserversInstalled) return;
    window.__allApiDeckBridgeObserversInstalled = true;

    if (nativeFetch) {
      window.fetch = async function patchedFetch(input, init) {
        const requestUrl = typeof input === 'string' ? input : (input?.url || '');
        const parsedUrl = parseUrlMaybe(requestUrl);
        const headersObject = normalizeHeadersObject(init?.headers || (typeof Request !== 'undefined' && input instanceof Request ? input.headers : null));
        if (parsedUrl && parsedUrl.origin === window.location.origin) {
          const bearerToken = extractBearerTokenFromHeaders(headersObject);
          if (bearerToken) {
            recordObservedAuthToken(bearerToken, {
              source: 'observed-fetch-auth',
              keyName: 'authorization',
              path: parsedUrl.pathname + parsedUrl.search,
            });
          }
          const compatUid = safeString(
            headersObject['one-api-user'] ||
            headersObject['User-id'] ||
            headersObject['user-id'] ||
            headersObject['New-API-User']
          );
          if (compatUid) {
            recordObservedUserId(compatUid, {
              source: 'observed-fetch-headers',
              keyName: 'user_id',
              path: parsedUrl.pathname + parsedUrl.search,
            });
          }
        }
        const response = await nativeFetch(input, init);
        if (parsedUrl && parsedUrl.origin === window.location.origin) {
          inspectObservedResponse(parsedUrl.href, response, 'fetch');
        }
        return response;
      };
    }

    if (typeof XMLHttpRequest !== 'undefined') {
      const originalOpen = XMLHttpRequest.prototype.open;
      const originalSend = XMLHttpRequest.prototype.send;
      const originalSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;

      XMLHttpRequest.prototype.open = function patchedOpen(method, url, async, user, password) {
        this.__allApiDeckBridgeMeta = {
          method: safeString(method || 'GET').toUpperCase(),
          url: safeString(url),
          headers: {},
        };
        return originalOpen.call(this, method, url, async, user, password);
      };

      XMLHttpRequest.prototype.setRequestHeader = function patchedSetRequestHeader(key, value) {
        try {
          if (this.__allApiDeckBridgeMeta) {
            this.__allApiDeckBridgeMeta.headers[safeString(key)] = safeString(value);
          }
        } catch {}
        return originalSetRequestHeader.call(this, key, value);
      };

      XMLHttpRequest.prototype.send = function patchedSend(body) {
        const meta = this.__allApiDeckBridgeMeta || { method: 'GET', url: '', headers: {} };
        const parsedUrl = parseUrlMaybe(meta.url);
        if (parsedUrl && parsedUrl.origin === window.location.origin) {
          const bearerToken = extractBearerTokenFromHeaders(meta.headers);
          if (bearerToken) {
            recordObservedAuthToken(bearerToken, {
              source: 'observed-xhr-auth',
              keyName: 'authorization',
              path: parsedUrl.pathname + parsedUrl.search,
            });
          }
          const compatUid = safeString(
            meta.headers['one-api-user'] ||
            meta.headers['User-id'] ||
            meta.headers['user-id'] ||
            meta.headers['New-API-User']
          );
          if (compatUid) {
            recordObservedUserId(compatUid, {
              source: 'observed-xhr-headers',
              keyName: 'user_id',
              path: parsedUrl.pathname + parsedUrl.search,
            });
          }

          this.addEventListener('load', () => {
            const contentType = safeString(this.getResponseHeader && this.getResponseHeader('content-type'));
            if (!/json/i.test(contentType)) return;
            const payload = safeJsonParse(this.responseText);
            if (payload) {
              inspectObservedJsonPayload(parsedUrl.href, payload, 'xhr');
            }
          }, { once: true });
        }
        return originalSend.call(this, body);
      };
    }
  }

  function buildCompatHeaders(uid) {
    const normalizedUid = safeString(uid);
    if (!/^\d+$/.test(normalizedUid)) return {};
    return {
      'one-api-user': normalizedUid,
      'New-API-User': normalizedUid,
      'Veloera-User': normalizedUid,
      'voapi-user': normalizedUid,
      'User-id': normalizedUid,
      'Rix-Api-User': normalizedUid,
      'neo-api-user': normalizedUid,
    };
  }

  function buildAuthHeaders(accessToken, uid) {
    const headers = {
      Accept: 'application/json, text/plain, */*',
      'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
      'X-Requested-With': 'XMLHttpRequest',
      'Cache-Control': 'no-cache',
      Pragma: 'no-cache',
    };
    const normalizedToken = safeString(accessToken);
    if (normalizedToken) {
      headers.Authorization = /^Bearer\s+/i.test(normalizedToken) ? normalizedToken : `Bearer ${normalizedToken}`;
    }
    return {
      ...headers,
      ...buildCompatHeaders(uid),
    };
  }

  async function sameOriginFetch(url, options = {}, timeoutMs = 7000) {
    if (!nativeFetch) {
      throw new Error('native fetch unavailable');
    }
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      const response = await nativeFetch(url, {
        method: options.method || 'GET',
        headers: options.headers || {},
        body: options.body,
        credentials: 'include',
        cache: 'no-store',
        mode: 'same-origin',
        signal: controller.signal,
      });
      return response;
    } finally {
      clearTimeout(timer);
    }
  }

  function extractListItems(body) {
    if (Array.isArray(body)) return body;
    if (!body || typeof body !== 'object') return [];
    if (Array.isArray(body.items)) return body.items;
    if (Array.isArray(body.data)) return body.data;
    if (body.data && typeof body.data === 'object') {
      if (Array.isArray(body.data.items)) return body.data.items;
      if (Array.isArray(body.data.data)) return body.data.data;
    }
    return [];
  }

  function extractSecretKeyFromPayload(payload) {
    if (!payload) return '';
    if (typeof payload === 'string') return safeString(payload);
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
      const value = safeString(candidate);
      if (value) return value;
    }
    return '';
  }

  function countUsableResolvedTokens(tokens) {
    return (Array.isArray(tokens) ? tokens : []).filter(item => {
      const key = safeString(item?.key || item?.access_token || item?.token || item?.api_key || item?.apikey || item);
      if (!key) return false;
      if (item?.unresolved === true) return false;
      return !key.includes('*');
    }).length;
  }

  function classifyBridgeFailure(status, payload) {
    const code = safeString(payload?.code || payload?.error || payload?.error_code).toUpperCase();
    const messageText = safeString(payload?.message || payload?.msg || payload?.error_description || payload?.detail).toLowerCase();
    if (code.includes('TOKEN_EXPIRED') || messageText.includes('token has expired') || messageText.includes('token expired')) {
      return 'token_expired';
    }
    if (status === 401 || code.includes('UNAUTHORIZED') || messageText.includes('not login') || messageText.includes('unauthorized') || messageText.includes('please login') || messageText.includes('login required')) {
      return 'not_logged_in';
    }
    return '';
  }

  async function resolveMaskedKey(origin, tokenId, headers, probeTrace) {
    const endpointCandidates = [
      { path: `/api/token/${tokenId}/key`, method: 'POST' },
      { path: `/api/token/${tokenId}/key`, method: 'GET' },
      { path: `/api/token/${tokenId}`, method: 'GET' },
      { path: `/api/v1/keys/${tokenId}`, method: 'GET' },
    ];
    for (const endpoint of endpointCandidates) {
      try {
        const url = `${origin}${endpoint.path}`;
        const response = await sameOriginFetch(url, {
          method: endpoint.method,
          headers: {
            ...headers,
            ...(endpoint.method !== 'GET' ? { 'Content-Type': 'application/json' } : {}),
          },
        }, 6000);
        if (!response.ok) {
          probeTrace.push(`[RESOLVE_KEY_HTTP_${response.status}] ${url}`);
          continue;
        }
        const payload = await response.json().catch(() => null);
        const key = extractSecretKeyFromPayload(payload);
        if (key) {
          probeTrace.push(`[RESOLVE_KEY_OK] ${url}`);
          return key;
        }
      } catch (error) {
        probeTrace.push(`[RESOLVE_KEY_EXCEPTION] ${endpoint.path} ${safeString(error?.message || error)}`);
      }
    }
    return '';
  }

  async function probeTokenEndpoints(origin, accessToken, userId, inferredSiteType) {
    const probeTrace = [];
    let detectedReason = '';
    const endpoints = inferredSiteType === 'anyrouter'
      ? [
        '/api/token/?p=0&size=100',
        '/api/token?p=0&size=100',
      ]
      : inferredSiteType === 'sub2api' || isLikelyJwt(accessToken)
        ? [
          '/api/v1/keys?page=1&page_size=100',
          '/api/v1/keys?p=0&size=100',
          '/api/token/?p=0&size=100',
          '/api/token?p=0&size=100',
        ]
        : [
          '/api/token/?p=0&size=100',
          '/api/token?p=0&size=100',
          '/api/v1/keys?page=1&page_size=100',
          '/api/v1/keys?p=0&size=100',
        ];
    const headers = buildAuthHeaders(accessToken, userId);
    for (const path of endpoints) {
      const url = `${origin}${path}`;
      try {
        probeTrace.push(`[TOKEN_TRY] ${url}`);
        const response = await sameOriginFetch(url, { headers }, 7000);
        if (!response.ok) {
          const payload = await response.clone().json().catch(() => null);
          detectedReason = detectedReason || classifyBridgeFailure(response.status, payload);
          probeTrace.push(`[TOKEN_HTTP_${response.status}] ${url}`);
          continue;
        }
        const contentType = safeString(response.headers.get('content-type'));
        if (/html/i.test(contentType)) {
          probeTrace.push(`[TOKEN_HTML] ${url}`);
          continue;
        }
        const payload = await response.json().catch(() => null);
        if (!payload) {
          probeTrace.push(`[TOKEN_JSON_FAIL] ${url}`);
          continue;
        }
        const items = extractListItems(payload);
        if (!items.length) {
          probeTrace.push(`[TOKEN_EMPTY] ${url}`);
          continue;
        }

        const normalizedTokens = [];
        for (const item of items.slice(0, 200)) {
          const rawKey = safeString(item?.key || item?.access_token || item?.token || item?.api_key || item?.apikey || (typeof item === 'string' ? item : ''));
          let resolvedKey = rawKey;
          if (resolvedKey.includes('*') && item?.id) {
            const fullKey = await resolveMaskedKey(origin, item.id, headers, probeTrace);
            if (fullKey) resolvedKey = fullKey;
          }
          normalizedTokens.push({
            ...item,
            key: resolvedKey || '未知格式Token',
            access_token: resolvedKey || '未知格式Token',
            unresolved: Boolean(resolvedKey.includes('*')),
          });
        }

        probeTrace.push(`[TOKEN_OK] ${url} count=${normalizedTokens.length}`);
        return {
          ok: true,
          endpoint: path,
          tokenCount: normalizedTokens.length,
          siteType: path.startsWith('/api/v1/keys') ? 'sub2api' : inferredSiteType,
          tokens: normalizedTokens,
          trace: probeTrace,
        };
      } catch (error) {
        probeTrace.push(`[TOKEN_EXCEPTION] ${url} ${safeString(error?.message || error)}`);
      }
    }
    return {
      ok: false,
      endpoint: '',
      tokenCount: 0,
      siteType: inferredSiteType,
      tokens: [],
      reason: detectedReason,
      trace: probeTrace,
    };
  }

  function collectUserIdCandidatesFromPayload(payload) {
    const results = [];
    if (!payload || typeof payload !== 'object') return results;
    const candidateValues = [
      payload?.id,
      payload?.uid,
      payload?.user_id,
      payload?.userId,
      payload?.data?.id,
      payload?.data?.uid,
      payload?.data?.user_id,
      payload?.data?.userId,
      payload?.data?.user?.id,
      payload?.data?.user?.uid,
      payload?.user?.id,
      payload?.user?.uid,
    ];
    candidateValues.forEach(value => {
      const text = safeString(value);
      if (isLikelyUserIdCandidate(text)) results.push(text);
    });
    return makeSetArray(results);
  }

  async function probeSelfEndpoints(origin, accessToken, userId) {
    const trace = [];
    let detectedReason = '';
    const endpoints = ['/api/user/self', '/api/v1/auth/me'];
    const headers = buildAuthHeaders(accessToken, userId);
    for (const path of endpoints) {
      const url = `${origin}${path}`;
      try {
        trace.push(`[SELF_TRY] ${url}`);
        const response = await sameOriginFetch(url, { headers }, 6000);
        if (!response.ok) {
          const payload = await response.clone().json().catch(() => null);
          detectedReason = detectedReason || classifyBridgeFailure(response.status, payload);
          trace.push(`[SELF_HTTP_${response.status}] ${url}`);
          continue;
        }
        const contentType = safeString(response.headers.get('content-type'));
        if (/html/i.test(contentType)) {
          trace.push(`[SELF_HTML] ${url}`);
          continue;
        }
        const payload = await response.json().catch(() => null);
        if (!payload) {
          trace.push(`[SELF_JSON_FAIL] ${url}`);
          continue;
        }
        const candidates = collectUserIdCandidatesFromPayload(payload);
        if (candidates.length > 0) {
          trace.push(`[SELF_OK] ${url} userId=${candidates[0]}`);
          return {
            ok: true,
            endpoint: path,
            userId: candidates[0],
            trace,
          };
        }
        trace.push(`[SELF_EMPTY] ${url}`);
      } catch (error) {
        trace.push(`[SELF_EXCEPTION] ${url} ${safeString(error?.message || error)}`);
      }
    }
    return {
      ok: false,
      endpoint: '',
      userId: '',
      reason: detectedReason,
      trace,
    };
  }

  function inferSiteType(origin, accessToken) {
    try {
      const parsed = new URL(origin);
      const host = safeString(parsed.hostname).toLowerCase();
      if (host === 'anyrouter.top' || host.endsWith('.anyrouter.top')) {
        return 'anyrouter';
      }
    } catch {}
    if (isLikelyJwt(accessToken)) {
      return 'sub2api';
    }
    return '';
  }

  function pickBestTokenCandidate(bucket) {
    const candidates = bucket.tokenCandidates
      .map(item => ({ ...item, score: scoreTokenCandidate(item) }))
      .sort((left, right) => right.score - left.score);
    return {
      selected: candidates[0] || null,
      candidates: candidates.slice(0, 12).map(item => ({
        source: item.source,
        storage: item.storage,
        keyName: item.keyName,
        path: item.path,
        score: item.score,
        preview: item.preview,
        kind: isLikelyJwt(item.value) ? 'jwt' : 'token',
      })),
    };
  }

  function mapBridgeIgnoreReasonToDetail(reason, fallbackDetail = '') {
    const normalized = safeString(reason);
    const reasonMap = {
      token_expired: '识别到站点登录态已过期，请重新登录后再试',
      token_expired_local: '识别到本地登录态已过期，请重新登录后再试',
      not_logged_in: '当前页面未登录，请先登录站点',
      weak_access_token: '只捕获到弱登录态，请在站点主界面重新触发',
      missing_access_token_and_tokens: '未发现可复用登录态，当前不会提交',
      no_bridge_signal: '未发现可复用的中转站特征',
      oauth_surface: '当前页是 OAuth 授权页，不是中转站主界面',
      cookie_only_nonrelay: '只抓到 Cookie，未发现可复用登录态',
      bootstrap_page: '当前页面属于桥接安装或引导页，已跳过',
      session_inactive: '桥接会话已关闭，本次不会提交',
    };
    return reasonMap[normalized] || fallbackDetail || normalized || '本次未提交';
  }

  function classifyRelaySite(payload) {
    const extracted = payload?.extracted || {};
    const diagnostics = payload?.diagnostics || {};
    const accessToken = safeString(extracted?.resolved_access_token || extracted?.account_info?.access_token);
    const userId = safeString(extracted?.resolved_user_id || extracted?.account_info?.id);
    const endpoint = safeString(extracted?.endpoint);
    const siteType = safeString(extracted?.site_type);
    const tokens = Array.isArray(extracted?.tokens) ? extracted.tokens : [];
    const extractedError = safeString(extracted?.error);
    const selfProbeOK = diagnostics?.self_probe?.ok === true;
    const tokenProbeOK = diagnostics?.token_probe?.ok === true;
    const observedAuthCount = Array.isArray(diagnostics?.observed_auth_candidates) ? diagnostics.observed_auth_candidates.length : 0;
    const observedSnapshotCount = Array.isArray(diagnostics?.observed_token_snapshots) ? diagnostics.observed_token_snapshots.length : 0;
    const blockedReason = ['token_expired', 'token_expired_local', 'not_logged_in', 'weak_access_token'].includes(extractedError)
      ? extractedError
      : '';

    const isRelay = Boolean(
      tokens.length > 0 ||
      endpoint ||
      siteType ||
      selfProbeOK ||
      tokenProbeOK ||
      (accessToken && (userId || observedAuthCount > 0 || observedSnapshotCount > 0)) ||
      ((extractedError === 'token_expired' || extractedError === 'token_expired_local' || extractedError === 'not_logged_in') && accessToken)
    );
    const shouldSubmit = isRelay && !blockedReason;

    let detail = '未发现可复用的中转站特征';
    if (tokens.length > 0) {
      detail = `已预取到 ${tokens.length} 个 key`;
    } else if (endpoint) {
      detail = `发现接口端点 ${endpoint}`;
    } else if (siteType) {
      detail = `识别站点类型 ${siteType}`;
    } else if (extractedError === 'token_expired' || extractedError === 'token_expired_local') {
      detail = '识别到站点登录态已过期，请重新登录后再试';
    } else if (extractedError === 'not_logged_in') {
      detail = '当前页面未登录，请先登录站点';
    } else if (extractedError === 'weak_access_token') {
      detail = '只捕获到弱登录态，请在站点主界面重新触发';
    } else if (accessToken) {
      detail = '已捕获登录态，等待进一步确认';
    }

    return { isRelay, shouldSubmit, blockedReason, detail };
  }

  function pickBestObservedTokenSnapshot() {
    const snapshots = (Array.isArray(observedBridgeState.tokenSnapshots) ? observedBridgeState.tokenSnapshots : [])
      .filter(item => Array.isArray(item?.tokens) && item.tokens.length > 0);
    return snapshots.length ? snapshots[snapshots.length - 1] : null;
  }

  function pickBestUserIdCandidate(bucket, accessToken) {
    const jwtPayload = tryDecodeJwtPayload(accessToken);
    const jwtCandidates = [
      jwtPayload?.user_id,
      jwtPayload?.uid,
      jwtPayload?.id,
      jwtPayload?.sub,
    ].map(value => safeString(value)).filter(isLikelyUserIdCandidate);

    const storageCandidates = bucket.userIdCandidates
      .map(item => ({ ...item, score: scoreUserIdCandidate(item) }))
      .sort((left, right) => right.score - left.score);

    const selected = jwtCandidates[0] || storageCandidates[0]?.value || '';
    return {
      selected,
      candidates: makeSetArray([
        ...jwtCandidates,
        ...storageCandidates.slice(0, 12).map(item => item.value),
      ]),
    };
  }

  function buildSafeBridgePayload() {
    const localStorageEntries = collectStorageEntries(window.localStorage, 'localStorage');
    const sessionStorageEntries = collectStorageEntries(window.sessionStorage, 'sessionStorage');
    const cookieEntries = collectCookieEntries();
    const bucket = {
      tokenCandidates: [],
      userIdCandidates: [],
    };

    localStorageEntries.forEach(entry => {
      walkCandidateValue(entry.value, {
        source: `storage:${entry.storage}`,
        storage: entry.storage,
        keyName: entry.key,
        path: entry.key,
      }, bucket, 0);
    });
    sessionStorageEntries.forEach(entry => {
      walkCandidateValue(entry.value, {
        source: `storage:${entry.storage}`,
        storage: entry.storage,
        keyName: entry.key,
        path: entry.key,
      }, bucket, 0);
    });
    cookieEntries.forEach(entry => {
      extractCandidateFromString(entry.value, {
        source: 'cookie',
        storage: 'cookie',
        keyName: entry.key,
        path: `cookie.${entry.key}`,
      }, bucket);
    });
    collectGlobalBootstrapValues(bucket);
    observedBridgeState.authCandidates.forEach(item => bucket.tokenCandidates.push(item));
    observedBridgeState.userIdCandidates.forEach(item => bucket.userIdCandidates.push(item));

    const tokenPick = pickBestTokenCandidate(bucket);
    const selectedAccessToken = safeString(tokenPick.selected?.value);
    const userIdPick = pickBestUserIdCandidate(bucket, selectedAccessToken);
    const observedTokenSnapshot = pickBestObservedTokenSnapshot();
    const inferredSiteType = safeString(observedTokenSnapshot?.siteType) || inferSiteType(window.location.origin, selectedAccessToken);

    return {
      bridge_version: bridgeVersion,
      bridge_protocol: 'site_account_prefetch_v1',
      execution_id: executionId,
      type: 'site_account_prefetch',
      source_url: window.location.href,
      source_origin: window.location.origin,
      title: document.title || '',
      user_agent: navigator.userAgent,
      captured_at: nowIso(),
      extracted: {
        site_name: document.title || window.location.hostname || '',
        site_url: window.location.origin,
        site_type: inferredSiteType,
        api_base_url: window.location.origin,
        account_info: {
          id: userIdPick.selected,
          access_token: selectedAccessToken,
        },
        resolved_access_token: selectedAccessToken,
        resolved_user_id: userIdPick.selected,
        tokens: Array.isArray(observedTokenSnapshot?.tokens) ? observedTokenSnapshot.tokens : [],
        endpoint: safeString(observedTokenSnapshot?.endpoint),
        error: '',
        storage_origin: window.location.origin,
        storage_fields: makeSetArray([
          ...localStorageEntries.map(item => item.key),
          ...sessionStorageEntries.map(item => item.key),
        ]),
        cookie_fields: makeSetArray(cookieEntries.map(item => item.key)),
      },
      diagnostics: {
        local_storage_keys: makeSetArray(localStorageEntries.map(item => item.key)),
        session_storage_keys: makeSetArray(sessionStorageEntries.map(item => item.key)),
        cookie_keys: makeSetArray(cookieEntries.map(item => item.key)),
        token_candidates: tokenPick.candidates,
        user_id_candidates: userIdPick.candidates,
        selected_access_token_preview: maskSecret(selectedAccessToken),
        selected_user_id: userIdPick.selected,
        observed_auth_candidates: observedBridgeState.authCandidates.map(item => ({
          source: item.source,
          path: item.path,
          preview: item.preview,
        })),
        observed_token_snapshots: observedBridgeState.tokenSnapshots.map(item => ({
          source: item.source,
          endpoint: item.endpoint,
          site_type: item.siteType,
          token_count: item.tokenCount,
        })),
        observed_response_traces: observedBridgeState.responseTraces.slice(-20),
      },
      client_logs: phaseLogs.slice(),
    };
  }

  async function enrichBridgePayload(payload) {
    const next = {
      ...payload,
      extracted: {
        ...(payload?.extracted || {}),
      },
      diagnostics: {
        ...(payload?.diagnostics || {}),
      },
    };

    const origin = safeString(next?.source_origin);
    const accessToken = safeString(next?.extracted?.resolved_access_token || next?.extracted?.account_info?.access_token);
    const selectedUserId = safeString(next?.extracted?.resolved_user_id || next?.extracted?.account_info?.id);
    const locallyExpiredJwt = accessToken && isJwtExpired(accessToken);

    logPhase('probe:start', '开始同源探测站点账号信息', {
      siteUrl: origin,
      accessToken: maskSecret(accessToken),
      userId: selectedUserId,
      siteType: next?.extracted?.site_type || '',
      localExpiredJwt: locallyExpiredJwt,
    });

    const selfProbe = locallyExpiredJwt
      ? {
          ok: false,
          endpoint: '',
          userId: '',
          reason: 'token_expired_local',
          trace: ['[SELF_SKIP] local jwt expired'],
        }
      : await probeSelfEndpoints(origin, accessToken, selectedUserId);
    if (selfProbe.ok && selfProbe.userId) {
      next.extracted.account_info.id = selfProbe.userId;
      next.extracted.resolved_user_id = selfProbe.userId;
    }

    const observedTokens = Array.isArray(next?.extracted?.tokens) ? next.extracted.tokens : [];
    const observedUsableTokenCount = countUsableResolvedTokens(observedTokens);
    let tokenProbe = {
      ok: observedTokens.length > 0 && observedUsableTokenCount > 0,
      endpoint: safeString(next?.extracted?.endpoint),
      tokenCount: observedTokens.length,
      siteType: safeString(next?.extracted?.site_type),
      tokens: observedTokens,
      trace: observedTokens.length > 0
        ? [`[TOKEN_OBSERVED] count=${observedTokens.length} usable=${observedUsableTokenCount}`]
        : [],
    };
    if (!tokenProbe.ok && !locallyExpiredJwt) {
      tokenProbe = await probeTokenEndpoints(
        origin,
        accessToken,
        safeString(next?.extracted?.resolved_user_id || next?.extracted?.account_info?.id),
        safeString(next?.extracted?.site_type)
      );
    }

    if (tokenProbe.ok) {
      next.extracted.tokens = Array.isArray(tokenProbe.tokens) ? tokenProbe.tokens : [];
      next.extracted.endpoint = safeString(tokenProbe.endpoint);
      if (safeString(tokenProbe.siteType)) {
        next.extracted.site_type = safeString(tokenProbe.siteType);
      }
      next.extracted.error = '';
    } else {
      next.extracted.tokens = [];
      next.extracted.error = safeString(tokenProbe.reason || selfProbe.reason || (locallyExpiredJwt ? 'token_expired_local' : 'bridge_prefetch_failed'));
    }

    next.diagnostics.self_probe = {
      ok: selfProbe.ok,
      endpoint: selfProbe.endpoint,
      user_id: selfProbe.userId,
      reason: safeString(selfProbe.reason),
      trace: selfProbe.trace,
    };
    next.diagnostics.token_probe = {
      ok: tokenProbe.ok,
      endpoint: tokenProbe.endpoint,
      token_count: tokenProbe.tokenCount,
      site_type: tokenProbe.siteType,
      reason: safeString(tokenProbe.reason),
      trace: tokenProbe.trace,
    };

    logPhase('probe:done', '同源探测结束', {
      userId: safeString(next?.extracted?.resolved_user_id),
      tokenCount: Array.isArray(next?.extracted?.tokens) ? next.extracted.tokens.length : 0,
      endpoint: safeString(next?.extracted?.endpoint),
      siteType: safeString(next?.extracted?.site_type),
    });

    next.client_logs = phaseLogs.slice();
    return next;
  }

  async function pingBridge() {
    try {
      const response = await request('GET', `${receiverBase}/bridge/ping`);
      const payload = safeJsonParse(response.responseText || '{}') || {};
      const sessionActive = payload.sessionActive !== false;
      const ok = response.status >= 200 && response.status < 300 && sessionActive;
      logPhase('ping:ok', '本地桥接响应正常', {
        status: response.status,
        mode: payload.mode || '',
        serverUrl: payload.serverUrl || receiverBase,
        version: payload.version || '',
      });
      return {
        ok,
        status: response.status,
        sessionActive,
        payload,
      };
    } catch (error) {
      logPhase('ping:fail', '本地桥接不可达', {
        error: safeString(error?.message || error?.error || error),
      });
      return {
        ok: false,
        status: 0,
        sessionActive: false,
        payload: {},
        error,
      };
    }
  }

  function isIgnoredBootstrapPage() {
    const host = safeString(window.location.hostname).toLowerCase();
    const path = safeString(window.location.pathname).toLowerCase();
    const search = safeString(window.location.search).toLowerCase();
    if (window.location.origin === receiverBase && /^\/bridge(\/|$)/.test(path)) {
      return true;
    }
    if ((host === 'localhost' || host === '127.0.0.1') && /^\/bridge(\/|$)/.test(path)) {
      return true;
    }
    if (/tampermonkey\.net$/i.test(host) && (path === '/script_installation.php' || path === '/userscript.php')) {
      return true;
    }
    if ((path === '/oauth2/authorize' || path === '/authorize') && /(?:^|[?&])client_id=/.test(search)) {
      return true;
    }
    return false;
  }

  async function waitForPageSettle() {
    if (document.readyState === 'complete') {
      await new Promise(resolve => setTimeout(resolve, 1200));
      return;
    }
    await new Promise(resolve => {
      const done = () => {
        window.removeEventListener('load', done);
        setTimeout(resolve, 1200);
      };
      window.addEventListener('load', done, { once: true });
    });
  }

  function getUsablePrefetchedKeyCount(payload) {
    const extracted = payload?.extracted || {};
    return countUsableResolvedTokens(Array.isArray(extracted?.tokens) ? extracted.tokens : []);
  }

  function tryAutoCloseCurrentTab(reason = '') {
    logPhase('tab:close-attempt', 'try close current tab', { reason });
    try {
      window.close();
    } catch (error) {
      logPhase('tab:close-fail', 'window.close failed', {
        reason,
        error: safeString(error?.message || error),
      });
      return false;
    }
    setTimeout(() => {
      if (!document.hidden) {
        logPhase('tab:close-blocked', 'browser blocked auto close', { reason });
      }
    }, 700);
    return true;
  }

  async function analyzeBridgeSubmission() {
    const payload = await enrichBridgePayload(buildSafeBridgePayload());
    const usableKeyCount = getUsablePrefetchedKeyCount(payload);
    let relayDecision = classifyRelaySite(payload);

    if (relayDecision.isRelay && usableKeyCount <= 0) {
      relayDecision = {
        ...relayDecision,
        shouldSubmit: false,
        blockedReason: relayDecision.blockedReason || 'no_usable_key',
        detail: '未获取到可用 key，请在站点主界面重试',
      };
    }

    updateBridgePanel({
      busy: false,
      line1: relayDecision.isRelay
        ? (relayDecision.shouldSubmit ? '检测完成' : '检测到异常状态，未提交')
        : '当前网站不是中转站',
      relayText: relayDecision.isRelay ? '是' : '不是',
      submittedText: '未提交',
      tone: relayDecision.isRelay ? (relayDecision.shouldSubmit ? 'success' : 'pending') : 'pending',
      detail: relayDecision.detail,
    });

    return {
      payload,
      relayDecision,
      usableKeyCount,
    };
  }

  async function sendBridgeImport(analysis) {
    logPhase('snapshot:build', 'build bridge payload', {
      url: window.location.href,
      title: document.title || '',
    });

    const resolvedAnalysis = analysis || await analyzeBridgeSubmission();
    const payload = resolvedAnalysis?.payload || {};
    const relayDecision = resolvedAnalysis?.relayDecision || { isRelay: false, shouldSubmit: false, detail: '未完成站点分析' };
    const usableKeyCount = Number(resolvedAnalysis?.usableKeyCount ?? getUsablePrefetchedKeyCount(payload));
    updateBridgePanel({
      busy: relayDecision.shouldSubmit === true,
      line1: relayDecision.shouldSubmit ? '分析完成，准备提交…' : (relayDecision.isRelay ? '检测到异常状态，未提交' : '当前网站不是中转站'),
      relayText: relayDecision.isRelay ? '是' : '不是',
      submittedText: '未提交',
      tone: relayDecision.shouldSubmit ? 'success' : 'pending',
      detail: relayDecision.detail,
    });
    if (!relayDecision.isRelay || relayDecision.shouldSubmit === false) {
      logPhase('import:skip', 'skip bridge import', {
        detail: relayDecision.detail,
        blockedReason: relayDecision.blockedReason || '',
      });
      return {
        skipped: true,
        relayDecision,
        payload,
      };
    }

    if (usableKeyCount <= 0) {
      const abnormalDetail = relayDecision.detail || '未获取到可用 key，请在站点主界面重试';
      logPhase('import:skip-no-usable-key', 'skip bridge import due to no usable key', {
        detail: abnormalDetail,
      });
      return {
        skipped: true,
        relayDecision: {
          ...relayDecision,
          shouldSubmit: false,
          detail: abnormalDetail,
        },
        payload,
        usableKeyCount,
      };
    }

    const pingState = await pingBridge();
    if (!pingState.ok) {
      bridgePanelSuppressed = true;
      removeBridgePanel();
      updateBridgePanel({
        busy: false,
        line1: '桥接会话已关闭',
        relayText: '是',
        submittedText: '未提交',
        tone: 'danger',
        detail: 'UI 已关闭或本地桥接不可用，本次不会提交',
      });
      return {
        skipped: true,
        relayDecision,
        payload,
        pingState,
      };
    }

    bridgePanelSuppressed = false;
    const response = await request('POST', `${receiverBase}/bridge/import`, payload);
    let result = {};
    try {
      result = JSON.parse(response.responseText || '{}');
    } catch {}
    logPhase('import:ack', 'bridge import acknowledged', {
      status: response.status,
      id: result.id || '',
      storedAt: result.storedAt || '',
      logPath: result.logPath || '',
      ignored: result?.ignored === true,
      reason: result?.reason || '',
    });

    if (result?.ignored || result?.ok === false) {
      if (safeString(result?.reason) === 'session_inactive') {
        bridgePanelSuppressed = true;
        removeBridgePanel();
      }
      return {
        response,
        result,
        payload,
        relayDecision: {
          ...relayDecision,
          shouldSubmit: false,
          blockedReason: result?.reason || relayDecision.blockedReason || '',
          detail: mapBridgeIgnoreReasonToDetail(result?.reason, relayDecision.detail),
        },
        skipped: true,
        usableKeyCount,
      };
    }

    return {
      response,
      result,
      payload,
      relayDecision,
      skipped: false,
      usableKeyCount,
    };
  }

  async function run() {
    if (isIgnoredBootstrapPage()) {
      console.info('[AllApiDeck Bridge] skip self bridge page.');
      return;
    }

    bridgePanelSuppressed = true;
    updateBridgePanel({
      busy: true,
      line1: '检测中...',
      relayText: '检测中',
      submittedText: '未提交',
      tone: 'pending',
      detail: '正在等待页面稳定并检查本地桥接会话',
    });
    installRuntimeObservers();
    await waitForPageSettle();

    logPhase('boot', 'bridge script injected', {
      executionId,
      href: window.location.href,
    });

    const pingState = await pingBridge();
    if (!pingState.ok) {
      bridgePanelSuppressed = true;
      removeBridgePanel();
      updateBridgePanel({
        busy: false,
        line1: pingState.status === 409 ? '桥接会话已关闭' : '本地桥接不可达',
        relayText: '检测中',
        submittedText: '未提交',
        tone: 'danger',
        detail: pingState.status === 409 ? '请先在 All API Deck 内打开当前标签导入面板' : `无法连接 ${receiverBase}`,
      });
      console.warn(`[AllApiDeck Bridge] local receiver unavailable: ${receiverBase}`);
      return;
    }

    bridgePanelSuppressed = false;
    updateBridgePanel({
      busy: true,
      line1: '检测中...',
      relayText: '检测中',
      submittedText: '未提交',
      tone: 'pending',
      detail: '本地桥接握手成功，正在分析当前站点',
    });

    const analysis = await analyzeBridgeSubmission();
    if (!analysis?.relayDecision?.isRelay) {
      return;
    }
    if (analysis?.relayDecision?.shouldSubmit === false) {
      updateBridgePanel({
        busy: false,
        line1: '检测到异常状态，未提交',
        relayText: '是',
        submittedText: '未提交',
        tone: 'pending',
        detail: analysis?.relayDecision?.detail || '当前页面存在异常状态，本次不会提交',
      });
      return;
    }

    updateBridgePanel({
      busy: true,
      line1: '自动提交中...',
      relayText: '是',
      submittedText: '未提交',
      tone: 'success',
      detail: '识别为中转站，正在静默提交到本地 All API Deck',
    });

    try {
      const submitState = await sendBridgeImport(analysis);
      if (submitState?.skipped && submitState?.relayDecision) {
        updateBridgePanel({
          busy: false,
          line1: submitState.relayDecision.isRelay ? '检测到异常状态，未提交' : '当前网站不是中转站',
          relayText: submitState.relayDecision.isRelay ? '是' : '不是',
          submittedText: '未提交',
          tone: 'pending',
          detail: submitState.relayDecision.detail || '本次未提交',
        });
        return;
      }
      const usableKeyCount = Number(submitState?.usableKeyCount || 0);
      updateBridgePanel({
        busy: false,
        line1: '提交完成',
        relayText: submitState?.relayDecision?.isRelay ? '是' : '检测中',
        submittedText: '已提交',
        tone: 'success',
        detail: usableKeyCount > 0
          ? `桥接数据已发送到本地 All API Deck，原因：成功获取 ${usableKeyCount} 个可用 key，准备关闭标签`
          : '桥接数据已发送到本地 All API Deck',
      });
      logPhase('import:auto-done', 'silent bridge submit completed', {
        usableKeyCount,
      });
      if (usableKeyCount > 0) {
        setTimeout(() => {
          tryAutoCloseCurrentTab(`usable_keys=${usableKeyCount}`);
        }, 900);
      }
    } catch (error) {
      updateBridgePanel({
        busy: false,
        line1: '提交失败',
        submittedText: '未提交',
        tone: 'danger',
        detail: safeString(error?.message || error?.error || error) || '未知错误',
      });
      logPhase('import:auto-fail', 'silent bridge submit failed', {
        error: safeString(error?.message || error?.error || error),
      });
    }
  }

  run();
})();
