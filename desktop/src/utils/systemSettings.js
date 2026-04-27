import { isProbablyWailsRuntime } from './runtimeApi.js';

const DESKTOP_TOKEN_SOURCE_MODE_STORAGE_KEY = 'batch_api_check_desktop_token_source_mode';
const TREE_EXPANDED_STORAGE_KEY = 'batch_api_check_tree_expanded_v1';
const OUTBOUND_PROXY_STORAGE_KEY = 'batch_api_check_outbound_proxy_v1';

export const OUTBOUND_PROXY_MODE_OPTIONS = [
  { label: '系统代理', value: 'system' },
  { label: '无代理', value: 'direct' },
  { label: '自定义代理', value: 'custom' },
];

function getAppBridge() {
  return window?.go?.main?.App;
}

export function normalizeDesktopTokenSourceMode(value) {
  const normalized = String(value || '').trim();
  if (normalized === 'cdp_restart') return 'cdp_restart';
  if (normalized === 'server_proxy') return 'cdp_restart';
  return 'profile_file';
}

export function loadDesktopTokenSourceMode() {
  try {
    return normalizeDesktopTokenSourceMode(localStorage.getItem(DESKTOP_TOKEN_SOURCE_MODE_STORAGE_KEY));
  } catch {
    return 'profile_file';
  }
}

export function saveDesktopTokenSourceMode(value) {
  const normalized = normalizeDesktopTokenSourceMode(value);
  try {
    localStorage.setItem(DESKTOP_TOKEN_SOURCE_MODE_STORAGE_KEY, normalized);
  } catch {}
  return normalized;
}

export function loadTreeExpandedSetting(defaultValue = true) {
  try {
    const raw = localStorage.getItem(TREE_EXPANDED_STORAGE_KEY);
    if (raw == null) return Boolean(defaultValue);
    return raw === '1' || raw === 'true';
  } catch {
    return Boolean(defaultValue);
  }
}

export function saveTreeExpandedSetting(value) {
  const nextValue = Boolean(value);
  try {
    localStorage.setItem(TREE_EXPANDED_STORAGE_KEY, nextValue ? '1' : '0');
  } catch {}
  return nextValue;
}

export function normalizeOutboundProxyConfig(input) {
  const rawMode = String(input?.mode || '').trim().toLowerCase();
  const mode = ['system', 'direct', 'custom'].includes(rawMode) ? rawMode : 'system';
  const customUrl = String(input?.customUrl || input?.customURL || '').trim();
  return {
    mode,
    customUrl: mode === 'custom' ? customUrl : '',
  };
}

function cacheOutboundProxyConfig(config) {
  try {
    localStorage.setItem(OUTBOUND_PROXY_STORAGE_KEY, JSON.stringify(normalizeOutboundProxyConfig(config)));
  } catch {}
}

export function loadCachedOutboundProxyConfig() {
  try {
    return normalizeOutboundProxyConfig(JSON.parse(localStorage.getItem(OUTBOUND_PROXY_STORAGE_KEY) || '{}'));
  } catch {
    return normalizeOutboundProxyConfig({});
  }
}

export async function getOutboundProxyConfig() {
  const app = getAppBridge();
  if (isProbablyWailsRuntime() && typeof app?.GetOutboundProxyConfig === 'function') {
    const result = normalizeOutboundProxyConfig(await app.GetOutboundProxyConfig());
    cacheOutboundProxyConfig(result);
    return result;
  }
  return loadCachedOutboundProxyConfig();
}

export async function setOutboundProxyConfig(config) {
  const nextConfig = normalizeOutboundProxyConfig(config);
  const app = getAppBridge();
  if (isProbablyWailsRuntime() && typeof app?.SetOutboundProxyConfig === 'function') {
    const saved = normalizeOutboundProxyConfig(await app.SetOutboundProxyConfig(nextConfig));
    cacheOutboundProxyConfig(saved);
    return saved;
  }
  cacheOutboundProxyConfig(nextConfig);
  return nextConfig;
}
