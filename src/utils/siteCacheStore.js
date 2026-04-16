export const SITE_CACHE_STORAGE_KEY = 'api_check_site_cache_records_v1';
export const SITE_CACHE_TEMP_STORAGE_KEY = 'api_check_site_cache_temp_records_v1';
export const SITE_CACHE_PENDING_RESTORE_KEY = 'api_check_site_cache_pending_restore_v1';
export const SITE_CACHE_PENDING_BATCH_START_KEY = 'api_check_site_cache_pending_batch_start_v1';
export const SITE_CACHE_SYNC_EVENT = 'batch-api-check:site-cache-sync';

function safeJsonParse(raw, fallback) {
  try {
    const parsed = JSON.parse(raw);
    return parsed == null ? fallback : parsed;
  } catch {
    return fallback;
  }
}

export function normalizeSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function normalizeNote(value) {
  return String(value || '').trim().slice(0, 10);
}

function clonePlainArray(value) {
  if (!Array.isArray(value)) return [];
  try {
    return JSON.parse(JSON.stringify(value));
  } catch {
    return [];
  }
}

function getStorage(storageType) {
  if (typeof window === 'undefined') return null;
  return storageType === 'temp' ? window.sessionStorage : window.localStorage;
}

function buildTokenKey(token) {
  return String(
    token?.key ||
    token?.access_token ||
    token?.apiKey ||
    token?.token ||
    ''
  ).trim();
}

function normalizeToken(token, index = 0, source = 'remote') {
  const key = buildTokenKey(token);
  if (!key) return null;
  return {
    ...token,
    key,
    access_token: key,
    name: String(token?.name || token?.token_name || `Token ${index + 1}`).trim() || `Token ${index + 1}`,
    source: String(token?.source || source).trim() || source,
    status: token?.status ?? 1,
    updatedAt: Number(token?.updatedAt || Date.now()),
  };
}

export function normalizeTokenList(tokens, source = 'remote') {
  const dedupe = new Map();
  (Array.isArray(tokens) ? tokens : []).forEach((token, index) => {
    const normalized = normalizeToken(token, index, source);
    if (!normalized) return;
    dedupe.set(normalized.key, normalized);
  });
  return Array.from(dedupe.values());
}

function mergeTokenLists(primaryTokens, secondaryTokens) {
  const dedupe = new Map();
  normalizeTokenList(secondaryTokens).forEach(token => {
    dedupe.set(token.key, token);
  });
  normalizeTokenList(primaryTokens).forEach(token => {
    dedupe.set(token.key, token);
  });
  return Array.from(dedupe.values());
}

function sanitizeAccountInfo(accountInfo) {
  const source = accountInfo && typeof accountInfo === 'object' ? accountInfo : {};
  return {
    ...source,
    id: String(source?.id || '').trim(),
    access_token: String(source?.access_token || '').trim(),
  };
}

function resolveSiteUserId(site) {
  return String(
    site?.resolved_user_id ||
    site?.resolvedUserId ||
    site?.account_info?.id ||
    site?.accountInfo?.id ||
    ''
  ).trim();
}

export function buildSiteCacheKey(siteLike) {
  const siteUrl = normalizeSiteUrl(siteLike?.site_url || siteLike?.siteUrl);
  const siteName = String(siteLike?.site_name || siteLike?.siteName || 'site').trim() || 'site';
  const userId = resolveSiteUserId(siteLike) || String(siteLike?.account_info?.id || siteLike?.accountInfo?.id || '').trim() || 'anonymous';
  return `${siteUrl}::${userId}::${siteName}`;
}

export function normalizeSiteCacheRecord(record) {
  if (!record) return null;
  const siteUrl = normalizeSiteUrl(record.siteUrl || record.site_url);
  if (!siteUrl) return null;

  const accountInfo = sanitizeAccountInfo(record.accountInfo || record.account_info);
  const resolvedAccessToken = String(
    record.resolvedAccessToken ||
    record.resolved_access_token ||
    accountInfo.access_token ||
    ''
  ).trim();
  const resolvedUserId = String(
    record.resolvedUserId ||
    record.resolved_user_id ||
    accountInfo.id ||
    ''
  ).trim();
  const siteName = String(record.siteName || record.site_name || '未命名站点').trim() || '未命名站点';
  const siteCacheKey = String(record.siteCacheKey || buildSiteCacheKey({
    site_url: siteUrl,
    site_name: siteName,
    resolved_user_id: resolvedUserId,
    account_info: accountInfo,
  })).trim();
  const remoteTokens = normalizeTokenList(record.tokens, 'remote');
  const customTokens = normalizeTokenList(record.customTokens, 'manual');
  const now = Date.now();

  return {
    siteCacheKey,
    siteName,
    siteUrl,
    siteType: String(record.siteType || record.site_type || '').trim(),
    apiBaseUrl: String(record.apiBaseUrl || record.api_key || '').trim(),
    accountInfo: {
      ...accountInfo,
      ...(resolvedUserId ? { id: resolvedUserId } : {}),
      ...(resolvedAccessToken ? { access_token: resolvedAccessToken } : {}),
    },
    resolvedAccessToken,
    resolvedUserId,
    tokens: remoteTokens,
    customTokens,
    endpoint: String(record.endpoint || '').trim(),
    error: String(record.error || '').trim(),
    profileStorageFields: Array.isArray(record.profileStorageFields || record._profileStorageFields)
      ? [...(record.profileStorageFields || record._profileStorageFields)]
      : [],
    profileStorageOrigin: String(record.profileStorageOrigin || record._profileStorageOrigin || '').trim(),
    cachedTreeNodes: clonePlainArray(record.cachedTreeNodes || record._cachedTreeNodes),
    disabled: record.disabled === true,
    note: normalizeNote(record.note),
    createdAt: Number(record.createdAt || now),
    updatedAt: Number(record.updatedAt || now),
    lastSyncedAt: Number(record.lastSyncedAt || now),
    lastImportSource: String(record.lastImportSource || record._lastImportSource || '').trim(),
    lastRefreshAt: Number(record.lastRefreshAt || 0),
  };
}

function loadRecordsFromStorage(storageKey, storageType = 'persistent') {
  const storage = getStorage(storageType);
  if (!storage) return [];
  const parsed = safeJsonParse(storage.getItem(storageKey) || '[]', []);
  if (!Array.isArray(parsed)) return [];
  return parsed
    .map(item => normalizeSiteCacheRecord(item))
    .filter(Boolean)
    .sort((left, right) => Number(right.updatedAt || 0) - Number(left.updatedAt || 0));
}

function persistRecordsToStorage(storageKey, records, storageType = 'persistent', options = {}) {
  const { broadcast = true } = options;
  const storage = getStorage(storageType);
  if (!storage) return [];
  const normalized = (Array.isArray(records) ? records : [])
    .map(item => normalizeSiteCacheRecord(item))
    .filter(Boolean)
    .sort((left, right) => Number(right.updatedAt || 0) - Number(left.updatedAt || 0));
  storage.setItem(storageKey, JSON.stringify(normalized));
  if (broadcast && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(SITE_CACHE_SYNC_EVENT, {
      detail: {
        count: normalized.length,
        storageType,
        updatedAt: Date.now(),
      },
    }));
  }
  return normalized;
}

function buildCacheCandidate(site, importSource, refreshedAt, now) {
  return normalizeSiteCacheRecord({
    siteCacheKey: site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site),
    siteName: site?.site_name || site?.siteName,
    siteUrl: site?.site_url || site?.siteUrl,
    siteType: site?.site_type || site?.siteType,
    apiBaseUrl: site?.api_key || site?.apiBaseUrl,
    accountInfo: site?.account_info || site?.accountInfo,
    resolvedAccessToken: site?.resolved_access_token || site?.resolvedAccessToken,
    resolvedUserId: site?.resolved_user_id || site?.resolvedUserId,
    tokens: site?.tokens,
    customTokens: site?.customTokens,
    endpoint: site?.endpoint,
    error: site?.error,
    profileStorageFields: site?._profileStorageFields,
    profileStorageOrigin: site?._profileStorageOrigin,
    cachedTreeNodes: site?.cachedTreeNodes || site?._cachedTreeNodes,
    note: site?._localNote ?? site?.note,
    disabled: site?._localDisabled ?? site?.disabled,
    lastImportSource: importSource || site?._lastImportSource || site?.lastImportSource || '',
    lastRefreshAt: refreshedAt,
    updatedAt: now,
    lastSyncedAt: now,
  });
}

function mergeSiteRecords(existing, candidate, importSource, refreshedAt, now) {
  return normalizeSiteCacheRecord({
    ...existing,
    ...candidate,
    accountInfo: {
      ...(existing?.accountInfo || {}),
      ...(candidate?.accountInfo || {}),
    },
    resolvedAccessToken: candidate?.resolvedAccessToken || existing?.resolvedAccessToken || '',
    resolvedUserId: candidate?.resolvedUserId || existing?.resolvedUserId || '',
    tokens: candidate?.tokens?.length > 0 ? candidate.tokens : normalizeTokenList(existing?.tokens, 'remote'),
    customTokens: mergeTokenLists(candidate?.customTokens, existing?.customTokens),
    note: normalizeNote(candidate?.note || existing?.note),
    disabled: candidate?.disabled === true || existing?.disabled === true,
    createdAt: Number(existing?.createdAt || candidate?.createdAt || now),
    updatedAt: now,
    lastSyncedAt: now,
    lastImportSource: importSource || candidate?.lastImportSource || existing?.lastImportSource || '',
    lastRefreshAt: Number(refreshedAt || candidate?.lastRefreshAt || existing?.lastRefreshAt || 0),
  });
}

function mergeExtractedSitesInternal(currentRecords, sites, options = {}) {
  const { importSource = '', refreshedAt = 0 } = options;
  const recordMap = new Map((Array.isArray(currentRecords) ? currentRecords : []).map(item => [item.siteCacheKey, item]));
  const now = Date.now();

  (Array.isArray(sites) ? sites : []).forEach(site => {
    const candidate = buildCacheCandidate(site, importSource, refreshedAt, now);
    if (!candidate) return;
    const existing = recordMap.get(candidate.siteCacheKey);
    recordMap.set(candidate.siteCacheKey, mergeSiteRecords(existing, candidate, importSource, refreshedAt, now));
  });

  return Array.from(recordMap.values());
}

function mergeRecordCollections(baseRecords, overlayRecords) {
  const map = new Map();
  (Array.isArray(baseRecords) ? baseRecords : []).forEach(record => {
    const normalized = normalizeSiteCacheRecord(record);
    if (!normalized) return;
    map.set(normalized.siteCacheKey, normalized);
  });
  (Array.isArray(overlayRecords) ? overlayRecords : []).forEach(record => {
    const normalized = normalizeSiteCacheRecord(record);
    if (!normalized) return;
    const existing = map.get(normalized.siteCacheKey);
    map.set(normalized.siteCacheKey, mergeSiteRecords(existing, normalized, normalized.lastImportSource, normalized.lastRefreshAt, Date.now()));
  });
  return Array.from(map.values()).sort((left, right) => Number(right.updatedAt || 0) - Number(left.updatedAt || 0));
}

export function loadSiteCacheRecords() {
  return loadRecordsFromStorage(SITE_CACHE_STORAGE_KEY, 'persistent');
}

export function loadTempSiteCacheRecords() {
  return loadRecordsFromStorage(SITE_CACHE_TEMP_STORAGE_KEY, 'temp');
}

export function loadAllSiteCacheRecords() {
  return mergeRecordCollections(loadSiteCacheRecords(), loadTempSiteCacheRecords());
}

export function persistSiteCacheRecords(records, options = {}) {
  return persistRecordsToStorage(SITE_CACHE_STORAGE_KEY, records, 'persistent', options);
}

export function persistTempSiteCacheRecords(records, options = {}) {
  return persistRecordsToStorage(SITE_CACHE_TEMP_STORAGE_KEY, records, 'temp', options);
}

export function mergeExtractedSitesIntoCache(sites, options = {}) {
  const next = mergeExtractedSitesInternal(loadSiteCacheRecords(), sites, options);
  return persistSiteCacheRecords(next, options);
}

export function mergeExtractedSitesIntoTempCache(sites, options = {}) {
  const next = mergeExtractedSitesInternal(loadTempSiteCacheRecords(), sites, options);
  return persistTempSiteCacheRecords(next, options);
}

export function setSiteCacheDisabled(siteCacheKey, disabled) {
  const updateOne = records => records.map(record => {
    if (record.siteCacheKey !== siteCacheKey) return record;
    return {
      ...record,
      disabled: disabled === true,
      updatedAt: Date.now(),
    };
  });
  persistTempSiteCacheRecords(updateOne(loadTempSiteCacheRecords()));
  return persistSiteCacheRecords(updateOne(loadSiteCacheRecords()));
}

export function updateSiteCacheNote(siteCacheKey, note) {
  const normalizedNote = normalizeNote(note);
  const updateOne = records => records.map(record => {
    if (record.siteCacheKey !== siteCacheKey) return record;
    return {
      ...record,
      note: normalizedNote,
      updatedAt: Date.now(),
    };
  });
  persistTempSiteCacheRecords(updateOne(loadTempSiteCacheRecords()));
  return persistSiteCacheRecords(updateOne(loadSiteCacheRecords()));
}

export function appendCustomKeysToSiteCache(siteCacheKey, rawKeys) {
  const values = Array.isArray(rawKeys)
    ? rawKeys
    : String(rawKeys || '')
      .split(/[\n,\s，]+/)
      .map(item => item.trim())
      .filter(Boolean);
  const normalizedValues = values
    .flatMap(item => String(item || '').split(/[，；;]+/))
    .map(item => item.trim())
    .filter(Boolean);
  const tokens = normalizedValues
    .map((key, index) => normalizeToken({
      key,
      access_token: key,
      name: `Manual SK ${index + 1}`,
      source: 'manual',
      status: 1,
    }, index, 'manual'))
    .filter(Boolean);

  const updateOne = records => records.map(record => {
    if (record.siteCacheKey !== siteCacheKey) return record;
    return {
      ...record,
      customTokens: mergeTokenLists(tokens, record.customTokens),
      updatedAt: Date.now(),
    };
  });
  persistTempSiteCacheRecords(updateOne(loadTempSiteCacheRecords()));
  return persistSiteCacheRecords(updateOne(loadSiteCacheRecords()));
}

export function removeCustomKeyFromSiteCache(siteCacheKey, tokenKey) {
  const normalizedTokenKey = String(tokenKey || '').trim();
  if (!siteCacheKey || !normalizedTokenKey) return [];
  const pruneCachedTreeNodes = (nodes) => (Array.isArray(nodes) ? nodes : [])
    .map(node => {
      const nextNode = {
        ...node,
        children: pruneCachedTreeNodes(node?.children),
      };
      return nextNode;
    })
    .filter(node => {
      const key = String(node?.key || '').trim();
      if (!key.startsWith('token|')) return true;
      const parts = key.split('|');
      return String(parts[2] || '').trim() !== normalizedTokenKey;
    });
  const updateOne = records => records.map(record => {
    if (record.siteCacheKey !== siteCacheKey) return record;
    return {
      ...record,
      customTokens: normalizeTokenList(record.customTokens, 'manual').filter(token => token.key !== normalizedTokenKey),
      cachedTreeNodes: pruneCachedTreeNodes(record.cachedTreeNodes),
      updatedAt: Date.now(),
    };
  });
  persistTempSiteCacheRecords(updateOne(loadTempSiteCacheRecords()));
  return persistSiteCacheRecords(updateOne(loadSiteCacheRecords()));
}

export function updateSiteCacheTreeNodes(siteCacheKey, cachedTreeNodes) {
  const normalizedNodes = clonePlainArray(cachedTreeNodes);
  const updateOne = records => records.map(record => {
    if (record.siteCacheKey !== siteCacheKey) return record;
    return {
      ...record,
      cachedTreeNodes: normalizedNodes,
      updatedAt: Date.now(),
    };
  });
  persistTempSiteCacheRecords(updateOne(loadTempSiteCacheRecords()));
  return persistSiteCacheRecords(updateOne(loadSiteCacheRecords()));
}

export function deleteSiteCacheRecord(siteCacheKey) {
  persistTempSiteCacheRecords(loadTempSiteCacheRecords().filter(record => record.siteCacheKey !== siteCacheKey));
  return persistSiteCacheRecords(loadSiteCacheRecords().filter(record => record.siteCacheKey !== siteCacheKey));
}

export function findSiteCacheRecord(siteCacheKey) {
  return loadSiteCacheRecords().find(record => record.siteCacheKey === siteCacheKey) || null;
}

export function findAnySiteCacheRecord(siteCacheKey) {
  return loadAllSiteCacheRecords().find(record => record.siteCacheKey === siteCacheKey) || null;
}

export function buildBatchSiteFromCache(record) {
  const normalized = normalizeSiteCacheRecord(record);
  if (!normalized) return null;
  const mergedTokens = mergeTokenLists(normalized.customTokens, normalized.tokens);
  return {
    id: normalized.siteCacheKey,
    site_name: normalized.siteName,
    site_url: normalized.siteUrl,
    site_type: normalized.siteType,
    api_key: normalized.apiBaseUrl,
    account_info: {
      ...(normalized.accountInfo || {}),
      ...(normalized.resolvedUserId ? { id: normalized.resolvedUserId } : {}),
      ...(normalized.resolvedAccessToken ? { access_token: normalized.resolvedAccessToken } : {}),
    },
    resolved_access_token: normalized.resolvedAccessToken,
    resolved_user_id: normalized.resolvedUserId,
    tokens: mergedTokens,
    endpoint: normalized.endpoint,
    error: normalized.error,
    _profileStorageFields: normalized.profileStorageFields,
    _profileStorageOrigin: normalized.profileStorageOrigin,
    _cachedTreeNodes: clonePlainArray(normalized.cachedTreeNodes),
    _siteCacheKey: normalized.siteCacheKey,
    _localDisabled: normalized.disabled,
    _localNote: normalized.note,
    _lastImportSource: normalized.lastImportSource,
  };
}

export function buildBatchSitesFromCache(records, options = {}) {
  const { siteCacheKeys = null, includeDisabled = true } = options;
  const allowedKeySet = Array.isArray(siteCacheKeys) && siteCacheKeys.length > 0
    ? new Set(siteCacheKeys.map(item => String(item || '').trim()).filter(Boolean))
    : null;

  return (Array.isArray(records) ? records : [])
    .filter(record => !allowedKeySet || allowedKeySet.has(String(record?.siteCacheKey || '').trim()))
    .filter(record => includeDisabled || record?.disabled !== true)
    .map(record => buildBatchSiteFromCache(record))
    .filter(Boolean);
}

export function writePendingSiteRestore(siteCacheKeys) {
  const keys = Array.isArray(siteCacheKeys)
    ? siteCacheKeys.map(item => String(item || '').trim()).filter(Boolean)
    : [];
  const storage = getStorage('persistent');
  if (!storage) return;
  storage.setItem(SITE_CACHE_PENDING_RESTORE_KEY, JSON.stringify(keys));
}

export function consumePendingSiteRestore() {
  const storage = getStorage('persistent');
  if (!storage) return [];
  const parsed = safeJsonParse(storage.getItem(SITE_CACHE_PENDING_RESTORE_KEY) || '[]', []);
  storage.removeItem(SITE_CACHE_PENDING_RESTORE_KEY);
  return Array.isArray(parsed) ? parsed.map(item => String(item || '').trim()).filter(Boolean) : [];
}

export function writePendingBatchStart(payload) {
  const storage = getStorage('persistent');
  if (!storage) return;
  storage.setItem(SITE_CACHE_PENDING_BATCH_START_KEY, JSON.stringify(payload || {}));
}

export function consumePendingBatchStart() {
  const storage = getStorage('persistent');
  if (!storage) return null;
  const parsed = safeJsonParse(storage.getItem(SITE_CACHE_PENDING_BATCH_START_KEY) || 'null', null);
  storage.removeItem(SITE_CACHE_PENDING_BATCH_START_KEY);
  return parsed && typeof parsed === 'object' ? parsed : null;
}
