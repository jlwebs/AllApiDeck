import { isProbablyWailsRuntime } from './runtimeApi.js';

const STORAGE_KEY = 'batch_api_check_advanced_proxy_config_v1';
const TAKEOVER_MAP_STORAGE_KEY = 'batch_api_check_advanced_proxy_takeover_map_v1';
export const ADVANCED_PROXY_SYNC_EVENT = 'batch-api-check:advanced-proxy-sync';
export const ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE = 'global';

export const ADVANCED_PROXY_APPS = [
  { id: 'claude', label: 'Claude', defaultBasePath: '/advanced-proxy/claude', mode: 'anthropic' },
  { id: 'codex', label: 'Codex', defaultBasePath: '/advanced-proxy/codex/v1', mode: 'openai' },
  { id: 'opencode', label: 'OpenCode', defaultBasePath: '/advanced-proxy/opencode/v1', mode: 'openai' },
  { id: 'openclaw', label: 'OpenClaw', defaultBasePath: '/advanced-proxy/openclaw/v1', mode: 'openai' },
];

export const ADVANCED_PROXY_QUEUE_SCOPES = [
  { id: ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, label: '全局' },
  ...ADVANCED_PROXY_APPS.map(app => ({ id: app.id, label: app.label })),
];

const DEFAULT_BASE_PATHS = Object.fromEntries(
  ADVANCED_PROXY_APPS.map(app => [app.id, app.defaultBasePath]),
);

function getAppBridge() {
  return window?.go?.main?.App;
}

function getDefaultAppSection(appId) {
  return {
    enabled: false,
    basePath: DEFAULT_BASE_PATHS[appId] || '/',
  };
}

function getDefaultQueueSection(inheritGlobal = false) {
  return {
    inheritGlobal: inheritGlobal === true,
    providers: [],
  };
}

function normalizeQueueScope(scope) {
  const normalized = String(scope || ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE).trim().toLowerCase();
  if (normalized === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
  }
  return ADVANCED_PROXY_APPS.some(app => app.id === normalized)
    ? normalized
    : ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

export function createDefaultAdvancedProxyConfig() {
  return {
    enabled: false,
    listenHost: '127.0.0.1',
    listenPort: 8888,
    queues: {
      global: getDefaultQueueSection(false),
      claude: getDefaultQueueSection(true),
      codex: getDefaultQueueSection(true),
      opencode: getDefaultQueueSection(true),
      openclaw: getDefaultQueueSection(true),
    },
    claude: {
      ...getDefaultAppSection('claude'),
      defaultModel: '',
      providers: [],
    },
    codex: getDefaultAppSection('codex'),
    opencode: getDefaultAppSection('opencode'),
    openclaw: getDefaultAppSection('openclaw'),
    failover: {
      appType: 'claude',
      enabled: false,
      autoFailoverEnabled: false,
      maxRetries: 2,
      streamingFirstByteTimeout: 25,
      streamingIdleTimeout: 60,
      nonStreamingTimeout: 90,
      circuitFailureThreshold: 3,
      circuitSuccessThreshold: 2,
      circuitTimeoutSeconds: 45,
      circuitErrorRateThreshold: 0.6,
      circuitMinRequests: 3,
    },
    rectifier: {
      enabled: true,
      requestThinkingSignature: true,
      requestThinkingBudget: true,
    },
    optimizer: {
      enabled: false,
      thinkingOptimizer: true,
      cacheInjection: true,
      cacheTtl: '1h',
    },
  };
}

function normalizeApiFormat(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === 'openai_chat' || normalized === 'openai_responses') {
    return normalized;
  }
  return 'anthropic';
}

function normalizeApiKeyField(value) {
  return String(value || '').trim() === 'ANTHROPIC_API_KEY'
    ? 'ANTHROPIC_API_KEY'
    : 'ANTHROPIC_AUTH_TOKEN';
}

function sanitizeProviders(providers) {
  const list = Array.isArray(providers) ? providers : [];
  return list
    .map((provider, index) => ({
      id: String(provider?.id || provider?.rowKey || provider?.baseUrl || `provider-${index + 1}`).trim(),
      rowKey: String(provider?.rowKey || '').trim(),
      name: String(provider?.name || provider?.baseUrl || `Provider ${index + 1}`).trim(),
      baseUrl: String(provider?.baseUrl || '').trim().replace(/\/+$/, ''),
      apiKey: String(provider?.apiKey || '').trim(),
      model: String(provider?.model || '').trim(),
      apiFormat: normalizeApiFormat(provider?.apiFormat),
      apiKeyField: normalizeApiKeyField(provider?.apiKeyField),
      enabled: provider?.enabled !== false,
      sortIndex: Number(provider?.sortIndex || index + 1),
      sourceType: String(provider?.sourceType || '').trim(),
    }))
    .filter(provider => provider.baseUrl && provider.apiKey)
    .sort((left, right) => left.sortIndex - right.sortIndex)
    .map((provider, index) => ({ ...provider, sortIndex: index + 1 }));
}

function normalizeAppSection(appId, input, defaults) {
  const next = {
    ...getDefaultAppSection(appId),
    ...(input || {}),
  };
  next.basePath = String(next.basePath || defaults.basePath).trim() || defaults.basePath;
  if (!next.basePath.startsWith('/')) {
    next.basePath = `/${next.basePath}`;
  }
  next.enabled = next.enabled === true;
  return next;
}

function normalizeQueueSection(input, defaults, fallbackProviders = null) {
  const next = {
    ...defaults,
    ...(input || {}),
  };
  const incomingProviders = Array.isArray(next.providers) ? next.providers : [];
  const providers = incomingProviders.length
    ? incomingProviders
    : (Array.isArray(fallbackProviders) ? fallbackProviders : []);
  next.inheritGlobal = next.inheritGlobal === true;
  next.providers = sanitizeProviders(providers);
  return next;
}

function getQueueSection(snapshot, scope) {
  const normalizedScope = normalizeQueueScope(scope);
  const defaults = createDefaultAdvancedProxyConfig();
  return snapshot?.queues?.[normalizedScope]
    || defaults.queues[normalizedScope]
    || defaults.queues.global;
}

export function normalizeAdvancedProxyConfig(input) {
  const defaults = createDefaultAdvancedProxyConfig();
  const legacyGlobalProviders = Array.isArray(input?.claude?.providers) ? input.claude.providers : [];
  const next = {
    ...defaults,
    ...(input || {}),
    queues: {
      ...defaults.queues,
      ...(input?.queues || {}),
    },
    claude: {
      ...defaults.claude,
      ...(input?.claude || {}),
    },
    codex: {
      ...defaults.codex,
      ...(input?.codex || {}),
    },
    opencode: {
      ...defaults.opencode,
      ...(input?.opencode || {}),
    },
    openclaw: {
      ...defaults.openclaw,
      ...(input?.openclaw || {}),
    },
    failover: {
      ...defaults.failover,
      ...(input?.failover || {}),
    },
    rectifier: {
      ...defaults.rectifier,
      ...(input?.rectifier || {}),
    },
    optimizer: {
      ...defaults.optimizer,
      ...(input?.optimizer || {}),
    },
  };

  next.listenHost = String(next.listenHost || defaults.listenHost).trim() || defaults.listenHost;
  next.listenPort = Number(next.listenPort || defaults.listenPort) || defaults.listenPort;

  next.queues.global = normalizeQueueSection(
    next.queues.global,
    defaults.queues.global,
    legacyGlobalProviders,
  );
  ADVANCED_PROXY_APPS.forEach(app => {
    next.queues[app.id] = normalizeQueueSection(next.queues[app.id], defaults.queues[app.id]);
  });

  next.claude = normalizeAppSection('claude', next.claude, defaults.claude);
  next.claude.defaultModel = String(next.claude.defaultModel || '').trim();
  next.claude.providers = [...next.queues.global.providers];

  next.codex = normalizeAppSection('codex', next.codex, defaults.codex);
  next.opencode = normalizeAppSection('opencode', next.opencode, defaults.opencode);
  next.openclaw = normalizeAppSection('openclaw', next.openclaw, defaults.openclaw);

  next.failover.maxRetries = Math.max(0, Math.min(10, Number(next.failover.maxRetries || defaults.failover.maxRetries)));
  next.failover.streamingFirstByteTimeout = Math.max(5, Number(next.failover.streamingFirstByteTimeout || defaults.failover.streamingFirstByteTimeout));
  next.failover.streamingIdleTimeout = Math.max(5, Number(next.failover.streamingIdleTimeout || defaults.failover.streamingIdleTimeout));
  next.failover.nonStreamingTimeout = Math.max(5, Number(next.failover.nonStreamingTimeout || defaults.failover.nonStreamingTimeout));
  next.failover.circuitFailureThreshold = Math.max(1, Number(next.failover.circuitFailureThreshold || defaults.failover.circuitFailureThreshold));
  next.failover.circuitSuccessThreshold = Math.max(1, Number(next.failover.circuitSuccessThreshold || defaults.failover.circuitSuccessThreshold));
  next.failover.circuitTimeoutSeconds = Math.max(5, Number(next.failover.circuitTimeoutSeconds || defaults.failover.circuitTimeoutSeconds));
  next.failover.circuitErrorRateThreshold = Number(next.failover.circuitErrorRateThreshold);
  if (!Number.isFinite(next.failover.circuitErrorRateThreshold) || next.failover.circuitErrorRateThreshold <= 0 || next.failover.circuitErrorRateThreshold > 1) {
    next.failover.circuitErrorRateThreshold = defaults.failover.circuitErrorRateThreshold;
  }
  next.failover.circuitMinRequests = Math.max(1, Number(next.failover.circuitMinRequests || defaults.failover.circuitMinRequests));
  next.failover.appType = String(next.failover.appType || defaults.failover.appType).trim() || defaults.failover.appType;
  next.optimizer.cacheTtl = String(next.optimizer.cacheTtl || defaults.optimizer.cacheTtl).trim() || defaults.optimizer.cacheTtl;

  next.enabled = ADVANCED_PROXY_APPS.some(app => next?.[app.id]?.enabled === true);
  return next;
}

function saveLocalSnapshot(config) {
  const normalizedConfig = normalizeAdvancedProxyConfig(config);
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(normalizedConfig));
  } catch {}
  try {
    localStorage.setItem(TAKEOVER_MAP_STORAGE_KEY, JSON.stringify(buildAdvancedProxyTakeoverMap(normalizedConfig)));
  } catch {}
}

function emitAdvancedProxySync(config) {
  if (typeof window === 'undefined') return;
  const snapshot = normalizeAdvancedProxyConfig(config);
  window.dispatchEvent(new CustomEvent(ADVANCED_PROXY_SYNC_EVENT, {
    detail: {
      config: snapshot,
      takeoverMap: getAdvancedProxyTakeoverMap(snapshot),
    },
  }));
}

export function getAdvancedProxyLocalSnapshot() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return normalizeAdvancedProxyConfig(JSON.parse(raw || '{}'));
  } catch {
    return normalizeAdvancedProxyConfig({});
  }
}

export function isAdvancedProxyBridgeAvailable() {
  const app = getAppBridge();
  return Boolean(
    isProbablyWailsRuntime() &&
      app &&
      typeof app.GetAdvancedProxyConfig === 'function' &&
      typeof app.SetAdvancedProxyConfig === 'function'
  );
}

export async function getAdvancedProxyConfig() {
  const app = getAppBridge();
  if (!app?.GetAdvancedProxyConfig) {
    return getAdvancedProxyLocalSnapshot();
  }
  const config = normalizeAdvancedProxyConfig(await app.GetAdvancedProxyConfig());
  saveLocalSnapshot(config);
  emitAdvancedProxySync(config);
  return config;
}

export async function setAdvancedProxyConfig(config) {
  const nextConfig = normalizeAdvancedProxyConfig(config);
  const app = getAppBridge();
  if (!app?.SetAdvancedProxyConfig) {
    saveLocalSnapshot(nextConfig);
    emitAdvancedProxySync(nextConfig);
    return nextConfig;
  }
  const saved = normalizeAdvancedProxyConfig(await app.SetAdvancedProxyConfig(nextConfig));
  saveLocalSnapshot(saved);
  emitAdvancedProxySync(saved);
  return saved;
}

export async function getAdvancedProxyConfigFilePath() {
  const app = getAppBridge();
  if (!app?.GetAdvancedProxyConfigFilePath) {
    return `localStorage:${STORAGE_KEY}`;
  }

  const resolved = String((await app.GetAdvancedProxyConfigFilePath()) || '').trim();
  return resolved || `localStorage:${STORAGE_KEY}`;
}

export async function getCircuitBreakerStats(appType, providerId) {
  const app = getAppBridge();
  if (!app?.GetCircuitBreakerStats) {
    return {
      state: 'closed',
      consecutiveFailures: 0,
      consecutiveSuccesses: 0,
      totalRequests: 0,
      failedRequests: 0,
    };
  }
  return app.GetCircuitBreakerStats(String(appType || 'claude'), String(providerId || '').trim());
}

export async function resetCircuitBreaker(appType, providerId) {
  const app = getAppBridge();
  if (!app?.ResetCircuitBreaker) return true;
  return app.ResetCircuitBreaker(String(appType || 'claude'), String(providerId || '').trim());
}

export function getAdvancedProxyQueueProviders(config = null, scope = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, options = {}) {
  const snapshot = normalizeAdvancedProxyConfig(config || getAdvancedProxyLocalSnapshot());
  const normalizedScope = normalizeQueueScope(scope);
  const { effective = false, enabledOnly = false } = options || {};

  let providers = getQueueSection(snapshot, normalizedScope).providers || [];
  if (
    effective &&
    normalizedScope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE &&
    getQueueSection(snapshot, normalizedScope).inheritGlobal
  ) {
    providers = getQueueSection(snapshot, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE).providers || [];
  }

  const cloned = [...providers];
  return enabledOnly ? cloned.filter(provider => provider?.enabled !== false) : cloned;
}

export function getAdvancedProxyEffectiveProviders(config = null, appId = 'claude', options = {}) {
  const normalizedAppId = normalizeQueueScope(appId);
  const enabledProviders = getAdvancedProxyQueueProviders(config, normalizedAppId, {
    effective: true,
    enabledOnly: options?.enabledOnly !== false,
  });

  if (normalizedAppId === 'claude') {
    return enabledProviders;
  }

  return enabledProviders.filter(provider => normalizeApiFormat(provider?.apiFormat) !== 'anthropic');
}

export function getAdvancedProxyAppBaseUrl(appId, config = null) {
  const normalizedAppId = String(appId || 'claude').trim().toLowerCase();
  const snapshot = normalizeAdvancedProxyConfig(config || getAdvancedProxyLocalSnapshot());
  const section = snapshot?.[normalizedAppId] || getDefaultAppSection(normalizedAppId);
  const basePath = String(section.basePath || DEFAULT_BASE_PATHS[normalizedAppId] || '/').trim() || '/';
  const normalizedBasePath = basePath.startsWith('/') ? basePath : `/${basePath}`;
  return `http://${snapshot.listenHost}:${snapshot.listenPort}${normalizedBasePath}`;
}

export function getAdvancedProxyClaudeBaseUrl(config = null) {
  return getAdvancedProxyAppBaseUrl('claude', config);
}

function buildAdvancedProxyTakeoverMap(snapshot) {
  const byApp = {};
  const byRowKey = {};

  ADVANCED_PROXY_APPS.forEach(app => {
    const isEnabled = snapshot?.enabled === true && snapshot?.[app.id]?.enabled === true;
    const rowKeys = isEnabled
      ? getAdvancedProxyEffectiveProviders(snapshot, app.id, { enabledOnly: true })
        .map(provider => String(provider?.rowKey || provider?.id || '').trim())
        .filter(Boolean)
      : [];

    byApp[app.id] = [...rowKeys];
    rowKeys.forEach(rowKey => {
      if (!byRowKey[rowKey]) {
        byRowKey[rowKey] = [];
      }
      byRowKey[rowKey].push(app.id);
    });
  });

  return { byApp, byRowKey };
}

export function getAdvancedProxyTakeoverMap(config = null) {
  if (config) {
    return buildAdvancedProxyTakeoverMap(normalizeAdvancedProxyConfig(config));
  }

  try {
    const raw = localStorage.getItem(TAKEOVER_MAP_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        return {
          byApp: parsed.byApp && typeof parsed.byApp === 'object' ? parsed.byApp : {},
          byRowKey: parsed.byRowKey && typeof parsed.byRowKey === 'object' ? parsed.byRowKey : {},
        };
      }
    }
  } catch {}

  return buildAdvancedProxyTakeoverMap(getAdvancedProxyLocalSnapshot());
}

export function countAdvancedProxyEnabledProviders(config = null, scope = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, options = {}) {
  return getAdvancedProxyQueueProviders(config, scope, options).filter(provider => provider?.enabled !== false).length;
}

export function countAdvancedProxyOpenAIProviders(config = null, scope = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, options = {}) {
  return getAdvancedProxyQueueProviders(config, scope, options).filter(
    provider => provider?.enabled !== false && normalizeApiFormat(provider?.apiFormat) !== 'anthropic',
  ).length;
}

export function isAdvancedProxyAppReady(appId, config = null) {
  const normalizedAppId = String(appId || 'claude').trim().toLowerCase();
  const snapshot = normalizeAdvancedProxyConfig(config || getAdvancedProxyLocalSnapshot());
  if (!snapshot.enabled || snapshot?.[normalizedAppId]?.enabled !== true) {
    return false;
  }

  return getAdvancedProxyEffectiveProviders(snapshot, normalizedAppId, { enabledOnly: true }).length > 0;
}

export function isAdvancedProxyClaudeReady(config = null) {
  return isAdvancedProxyAppReady('claude', config);
}
