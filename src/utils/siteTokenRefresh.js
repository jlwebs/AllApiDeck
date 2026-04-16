import { apiFetch, isProbablyWailsRuntime } from './runtimeApi.js';
import { extractChromeProfileTokens, isChromeProfileAuthBridgeAvailable } from './profileAuthBridge.js';
import { normalizeSiteUrl } from './siteCacheStore.js';

function attachCompatHeaders(headers, uid) {
  const normalizedUid = String(uid || '').trim();
  if (!/^\d+$/.test(normalizedUid)) return headers;
  return {
    ...headers,
    'one-api-user': normalizedUid,
    'New-API-User': normalizedUid,
    'Veloera-User': normalizedUid,
    'voapi-user': normalizedUid,
    'User-id': normalizedUid,
    'Rix-Api-User': normalizedUid,
    'neo-api-user': normalizedUid,
  };
}

function buildRefreshSeed(siteLike) {
  const siteUrl = normalizeSiteUrl(siteLike?.siteUrl || siteLike?.site_url);
  const accessToken = String(
    siteLike?.resolvedAccessToken ||
    siteLike?.resolved_access_token ||
    siteLike?.accountInfo?.access_token ||
    siteLike?.account_info?.access_token ||
    ''
  ).trim();
  const userId = String(
    siteLike?.resolvedUserId ||
    siteLike?.resolved_user_id ||
    siteLike?.accountInfo?.id ||
    siteLike?.account_info?.id ||
    ''
  ).trim();
  const importSource = String(
    siteLike?._lastImportSource ||
    siteLike?.lastImportSource ||
    siteLike?.importSource ||
    ''
  ).trim();

  return {
    ...siteLike,
    id: String(siteLike?.id || siteLike?.siteCacheKey || siteLike?._siteCacheKey || userId || siteUrl).trim(),
    site_name: String(siteLike?.siteName || siteLike?.site_name || '未命名站点').trim() || '未命名站点',
    site_url: siteUrl,
    site_type: String(siteLike?.siteType || siteLike?.site_type || '').trim(),
    api_key: String(siteLike?.apiBaseUrl || siteLike?.api_key || '').trim(),
    account_info: {
      ...(siteLike?.accountInfo || siteLike?.account_info || {}),
      ...(userId ? { id: userId } : {}),
      ...(accessToken ? { access_token: accessToken } : {}),
    },
    resolved_access_token: accessToken,
    resolved_user_id: userId,
    _siteCacheKey: String(siteLike?.siteCacheKey || siteLike?._siteCacheKey || '').trim(),
    _localDisabled: siteLike?.disabled === true || siteLike?._localDisabled === true,
    _localNote: String(siteLike?.note || siteLike?._localNote || '').trim(),
    _lastImportSource: importSource,
  };
}

async function tryRefreshAccessTokenFromExtension(siteLike) {
  if (!isProbablyWailsRuntime()) return buildRefreshSeed(siteLike);

  const seed = buildRefreshSeed(siteLike);
  if (!/extension_import/i.test(String(seed?._lastImportSource || ''))) {
    return seed;
  }

  const importer = window?.go?.main?.App?.ImportExtensionAccounts;
  if (typeof importer !== 'function') return seed;

  try {
    const result = await importer();
    const accounts = result?.payload?.accounts?.accounts;
    const matched = (Array.isArray(accounts) ? accounts : []).find(account =>
      normalizeSiteUrl(account?.site_url) === normalizeSiteUrl(seed?.site_url)
    );
    if (!matched?.account_info?.access_token) return seed;
    return buildRefreshSeed({
      ...seed,
      ...matched,
      account_info: {
        ...(seed?.account_info || {}),
        ...(matched?.account_info || {}),
      },
      _lastImportSource: 'extension_import_refresh',
    });
  } catch {
    return seed;
  }
}

async function fetchOneSiteTokens(account) {
  const accessToken = String(account?.account_info?.access_token || '').trim();
  const userId = String(account?.account_info?.id || '').trim();
  const response = await apiFetch('/api/fetch-keys', {
    method: 'POST',
    headers: attachCompatHeaders({ 'Content-Type': 'application/json' }, userId),
    body: JSON.stringify({
      accounts: [{
        ...account,
        resolved_access_token: accessToken,
        resolved_user_id: userId,
      }],
    }),
  });

  if (!response.ok) {
    const text = await response.text().catch(() => '');
    throw new Error(text || `刷新失败 (HTTP ${response.status})`);
  }

  const payload = await response.json().catch(() => ({}));
  const result = Array.isArray(payload?.results) ? payload.results[0] : null;
  if (!result) {
    throw new Error('刷新接口未返回任何站点数据');
  }

  return {
    ...account,
    ...result,
    site_name: result?.site_name || account.site_name,
    site_url: result?.site_url || account.site_url,
    site_type: result?.site_type || account.site_type,
    api_key: result?.api_key || account.api_key,
    account_info: {
      ...(account.account_info || {}),
      ...(result?.account_info || {}),
      ...(userId ? { id: userId } : {}),
      ...(accessToken ? { access_token: accessToken } : {}),
    },
    resolved_access_token: result?.resolved_access_token || accessToken,
    resolved_user_id: result?.resolved_user_id || userId,
    _siteCacheKey: String(account?._siteCacheKey || '').trim(),
    _localDisabled: account?._localDisabled === true,
    _localNote: String(account?._localNote || '').trim(),
    _lastImportSource: String(account?._lastImportSource || '').trim(),
  };
}

export async function refreshCachedSiteTokens(siteLike) {
  let account = await tryRefreshAccessTokenFromExtension(siteLike);
  const siteUrl = normalizeSiteUrl(account?.site_url);
  const accessToken = String(account?.account_info?.access_token || '').trim();

  if (!siteUrl || !accessToken) {
    throw new Error('当前站点缓存缺少 site_url 或 access_token，无法刷新');
  }

  let lastError = null;
  try {
    return await fetchOneSiteTokens(account);
  } catch (error) {
    lastError = error;
  }

  if (isProbablyWailsRuntime() && isChromeProfileAuthBridgeAvailable()) {
    try {
      const response = await extractChromeProfileTokens([account]);
      const profileResult = Array.isArray(response?.results) ? response.results[0] : null;
      if (profileResult) {
        account = buildRefreshSeed({
          ...account,
          ...profileResult,
          account_info: {
            ...(account?.account_info || {}),
            ...(profileResult?.account_info || {}),
          },
        });
        return {
          ...account,
          ...profileResult,
          resolved_access_token: profileResult?.resolved_access_token || account?.account_info?.access_token || '',
          resolved_user_id: profileResult?.resolved_user_id || account?.account_info?.id || '',
          _siteCacheKey: String(account?._siteCacheKey || '').trim(),
          _localDisabled: account?._localDisabled === true,
          _localNote: String(account?._localNote || '').trim(),
          _lastImportSource: String(account?._lastImportSource || '').trim(),
        };
      }
    } catch (error) {
      lastError = error;
    }
  }

  throw lastError || new Error('刷新失败');
}
