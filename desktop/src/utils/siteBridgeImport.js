import { apiFetch } from './runtimeApi.js';
import { buildRowKey, loadPanelRecords, normalizeModels as normalizeKeyPanelModels, persistPanelRecords } from './keyPanelStore.js';
import { buildSiteCacheKey, normalizeSiteUrl } from './siteCacheStore.js';

function looksLikeJwtToken(value) {
  const text = String(value || '').trim();
  if (!text) return false;
  const parts = text.split('.');
  return parts.length >= 3 && parts.every(part => /^[A-Za-z0-9_-]+$/.test(part));
}

function buildTokenKey(token) {
  return String(
    token?.key ||
    token?.access_token ||
    token?.token ||
    token?.api_key ||
    token?.apikey ||
    (typeof token === 'string' ? token : '')
  ).trim();
}

export function normalizeBridgeImportedTokens(tokens) {
  const dedupe = new Map();
  (Array.isArray(tokens) ? tokens : []).forEach((token, index) => {
    const key = buildTokenKey(token);
    if (!key) return;
    dedupe.set(key, {
      ...token,
      key,
      access_token: key,
      name: String(token?.name || token?.token_name || `Bridge Token ${index + 1}`).trim() || `Bridge Token ${index + 1}`,
      status: token?.status ?? 1,
      source: String(token?.source || 'bridge').trim() || 'bridge',
    });
  });
  return Array.from(dedupe.values());
}

export function inferBridgeImportedSiteType({ siteType, siteUrl, endpoint, accessToken }) {
  const explicit = String(siteType || '').trim().toLowerCase();
  if (explicit) return explicit;
  const endpointText = String(endpoint || '').trim();
  if (endpointText.startsWith('/api/v1/keys')) return 'sub2api';
  try {
    const host = String(new URL(siteUrl).hostname || '').toLowerCase();
    if (host === 'anyrouter.top' || host.endsWith('.anyrouter.top')) return 'anyrouter';
  } catch {}
  if (looksLikeJwtToken(accessToken)) return 'sub2api';
  return '';
}

export function buildBridgeImportedPreparedPayload(records) {
  const prefetchedSites = [];
  const accounts = [];
  const skipped = [];
  const blockedReasons = {
    token_expired: '登录态已过期，请重新登录站点后再试',
    token_expired_local: '登录态已过期，请重新登录站点后再试',
    not_logged_in: '当前页面未登录，请先登录站点后再试',
    weak_access_token: '未捕获到可复用的真实登录态，请在站点主界面重新触发',
  };

  (Array.isArray(records) ? records : []).forEach((record, index) => {
    const payload = record?.payload && typeof record.payload === 'object' ? record.payload : {};
    const extracted = payload?.extracted && typeof payload.extracted === 'object' ? payload.extracted : payload;
    const readyReason = String(record?.readyReason || '').trim();
    const extractedError = String(extracted?.error || '').trim();
    const sourceUrl = normalizeSiteUrl(
      extracted?.site_url ||
      extracted?.source_origin ||
      payload?.source_origin ||
      record?.sourceOrigin ||
      record?.sourceUrl ||
      ''
    );
    if (!sourceUrl) {
      skipped.push({
        title: String(record?.title || `桥接记录 ${index + 1}`).trim() || `桥接记录 ${index + 1}`,
        reason: '缺少站点地址',
      });
      return;
    }

    let hostname = '';
    try {
      hostname = new URL(sourceUrl).hostname;
    } catch {}

    const accountInfo = extracted?.account_info && typeof extracted.account_info === 'object'
      ? extracted.account_info
      : {};
    const accessToken = String(
      extracted?.resolved_access_token ||
      extracted?.access_token ||
      accountInfo?.access_token ||
      ''
    ).trim();
    const userId = String(
      extracted?.resolved_user_id ||
      extracted?.user_id ||
      accountInfo?.id ||
      ''
    ).trim();
    const endpoint = String(extracted?.endpoint || payload?.endpoint || '').trim();
    const siteType = inferBridgeImportedSiteType({
      siteType: extracted?.site_type || payload?.site_type,
      siteUrl: sourceUrl,
      endpoint,
      accessToken,
    });
    const siteName = String(
      extracted?.site_name ||
      record?.title ||
      hostname ||
      `桥接站点 ${index + 1}`
    ).trim() || `桥接站点 ${index + 1}`;

    const blockedReason = blockedReasons[readyReason] || blockedReasons[extractedError] || '';
    if (blockedReason) {
      skipped.push({
        title: siteName,
        reason: blockedReason,
      });
      return;
    }

    const prefetchedTokens = normalizeBridgeImportedTokens(extracted?.tokens || payload?.tokens);
    const storageFields = Array.isArray(extracted?.storage_fields)
      ? extracted.storage_fields
      : Array.isArray(payload?.storage_fields)
        ? payload.storage_fields
        : [];
    const storageOrigin = String(
      extracted?.storage_origin ||
      payload?.storage_origin ||
      sourceUrl
    ).trim();
    const baseSite = {
      site_name: siteName,
      site_url: sourceUrl,
      site_type: siteType,
      api_key: normalizeSiteUrl(extracted?.api_base_url || payload?.api_base_url || sourceUrl),
      account_info: {
        ...(accountInfo || {}),
        ...(userId ? { id: userId } : {}),
        ...(accessToken ? { access_token: accessToken } : {}),
      },
      resolved_access_token: accessToken,
      resolved_user_id: userId,
      tokens: prefetchedTokens,
      endpoint,
      error: String(extracted?.error || '').trim(),
      _profileStorageFields: storageFields,
      _profileStorageOrigin: storageOrigin,
      _siteCacheKey: String(extracted?._siteCacheKey || buildSiteCacheKey({
        site_url: sourceUrl,
        site_name: siteName,
        resolved_user_id: userId,
        account_info: { ...(accountInfo || {}), ...(userId ? { id: userId } : {}) },
      })).trim(),
    };

    if (prefetchedTokens.length > 0) {
      prefetchedSites.push(baseSite);
      return;
    }

    if (accessToken) {
      accounts.push({
        id: String(userId || sourceUrl || `bridge-account-${index + 1}`).trim(),
        site_name: siteName,
        site_url: sourceUrl,
        site_type: siteType,
        api_key: normalizeSiteUrl(extracted?.api_base_url || payload?.api_base_url || sourceUrl),
        account_info: {
          ...(accountInfo || {}),
          ...(userId ? { id: userId } : {}),
          access_token: accessToken,
        },
        resolved_access_token: accessToken,
        resolved_user_id: userId,
        _profileStorageFields: storageFields,
        _profileStorageOrigin: storageOrigin,
        _siteCacheKey: baseSite._siteCacheKey,
      });
      return;
    }

    skipped.push({
      title: siteName,
      reason: '未提取到 access_token 且未预取到账户内 key',
    });
  });

  return {
    prefetchedSites,
    accounts,
    skipped,
  };
}

export async function fetchBridgeImportSites(prepared) {
  const prefetchedSites = Array.isArray(prepared?.prefetchedSites) ? prepared.prefetchedSites : [];
  const accounts = Array.isArray(prepared?.accounts) ? prepared.accounts : [];
  const skipped = Array.isArray(prepared?.skipped) ? prepared.skipped : [];
  const importedSites = [...prefetchedSites];
  const failed = [...skipped];

  if (accounts.length > 0) {
    const response = await apiFetch('/api/fetch-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ accounts }),
    });
    if (!response.ok) {
      const text = await response.text().catch(() => '');
      throw new Error(text || `bridge fetch failed (${response.status})`);
    }

    const data = await response.json().catch(() => ({}));
    const results = Array.isArray(data?.results) ? data.results : [];
    results.forEach((site, index) => {
      const tokenList = normalizeBridgeImportedTokens(site?.tokens);
      const siteName = String(site?.site_name || accounts[index]?.site_name || `桥接站点 ${index + 1}`).trim() || `桥接站点 ${index + 1}`;
      if (tokenList.length <= 0) {
        failed.push({
          title: siteName,
          reason: String(site?.error || '未获取到任何 key').trim() || '未获取到任何 key',
        });
        return;
      }
      importedSites.push({
        ...site,
        tokens: tokenList,
      });
    });
  }

  return {
    sites: importedSites,
    skipped: failed,
  };
}

function collectSiteCacheModelsByToken(nodes, bucket = new Map()) {
  (Array.isArray(nodes) ? nodes : []).forEach(node => {
    const key = String(node?.key || '').trim();
    if (key.startsWith('token|')) {
      const parts = key.split('|');
      const tokenKey = String(parts[2] || '').trim();
      if (tokenKey) {
        const models = (Array.isArray(node?.children) ? node.children : [])
          .map(child => {
            const childKey = String(child?.key || '').trim();
            const childTitle = String(child?.title || '').trim();
            if (childTitle) return childTitle;
            if (!childKey.includes('|')) return '';
            return String(childKey.split('|').slice(2).join('|') || '').trim();
          })
          .filter(Boolean);
        if (models.length > 0) {
          bucket.set(tokenKey, normalizeKeyPanelModels([
            ...(bucket.get(tokenKey) || []),
            ...models,
          ]));
        }
      }
    }
    if (Array.isArray(node?.children) && node.children.length > 0) {
      collectSiteCacheModelsByToken(node.children, bucket);
    }
  });
  return bucket;
}

export function syncBridgeSitesToKeyPanel(sites) {
  const targetSites = Array.isArray(sites) ? sites : [];
  if (targetSites.length === 0) return 0;

  const { records: existingRecords } = loadPanelRecords();
  const mergedRecords = new Map(
    existingRecords.map(record => [
      String(record?.rowKey || buildRowKey(record?.siteUrl, record?.apiKey)).trim(),
      { ...record },
    ])
  );

  const now = Date.now();
  let importedCount = 0;

  targetSites.forEach((site, siteIndex) => {
    const siteUrl = normalizeSiteUrl(site?.siteUrl || site?.site_url);
    if (!siteUrl) return;

    const siteName = String(site?.siteName || site?.site_name || `桥接站点 ${siteIndex + 1}`).trim() || `桥接站点 ${siteIndex + 1}`;
    const modelsByToken = collectSiteCacheModelsByToken(site?.cachedTreeNodes || site?._cachedTreeNodes);
    const tokenMap = new Map();

    [...(Array.isArray(site?.tokens) ? site.tokens : []), ...(Array.isArray(site?.customTokens) ? site.customTokens : [])]
      .forEach((token, tokenIndex) => {
        const apiKey = buildTokenKey(token);
        if (!apiKey) return;
        tokenMap.set(apiKey, {
          ...token,
          apiKey,
          tokenName: String(token?.name || `Bridge Token ${tokenIndex + 1}`).trim() || `Bridge Token ${tokenIndex + 1}`,
        });
      });

    tokenMap.forEach(token => {
      const rowKey = buildRowKey(siteUrl, token.apiKey);
      const existing = mergedRecords.get(rowKey) || null;
      const modelsList = normalizeKeyPanelModels([
        ...(Array.isArray(existing?.modelsList) ? existing.modelsList : []),
        ...(Array.isArray(token?.models) ? token.models : []),
        ...(modelsByToken.get(token.apiKey) || []),
        token?.model || '',
        existing?.selectedModel || '',
      ]);
      const statusValue = Number(token?.status ?? existing?.status ?? 1);
      const nextRecord = {
        ...existing,
        rowKey,
        sourceType: 'auto',
        siteName,
        tokenName: token.tokenName || existing?.tokenName || '',
        siteUrl,
        apiKey: token.apiKey,
        modelsList,
        modelsText: modelsList.join(', ') || existing?.modelsText || '未提供模型信息',
        selectedModel: (
          (existing?.selectedModel && modelsList.includes(String(existing.selectedModel).trim()) && String(existing.selectedModel).trim())
          || (String(token?.selectedModel || '').trim() && modelsList.includes(String(token.selectedModel).trim()) && String(token.selectedModel).trim())
          || modelsList[0]
          || ''
        ),
        status: statusValue === 2 ? 2 : 1,
        createdAt: existing?.createdAt || now,
        updatedAt: now,
        quickTestStatus: existing?.quickTestStatus || '',
        quickTestLabel: existing?.quickTestLabel || '',
        quickTestModel: existing?.quickTestModel || '',
        quickTestRemark: existing?.quickTestRemark || '',
        quickTestAt: existing?.quickTestAt || null,
        quickTestResponseTime: existing?.quickTestResponseTime || '',
        quickTestTtftMs: existing?.quickTestTtftMs || '',
        quickTestTps: existing?.quickTestTps || '',
        quickTestResponseContent: existing?.quickTestResponseContent || '',
        balanceLabel: existing?.balanceLabel || '',
        balanceUpdatedAt: existing?.balanceUpdatedAt || null,
        balanceError: existing?.balanceError || '',
        remainQuota: token?.remain_quota ?? existing?.remainQuota ?? null,
        usedQuota: token?.used_quota ?? existing?.usedQuota ?? null,
        unlimitedQuota: token?.unlimited_quota === true || existing?.unlimitedQuota === true,
      };
      mergedRecords.set(rowKey, nextRecord);
      importedCount += 1;
    });
  });

  if (importedCount > 0) {
    persistPanelRecords(Array.from(mergedRecords.values()));
  }

  return importedCount;
}

export function summarizeBridgeImportNotices(beforeRecords, importedSites) {
  const beforeMap = new Map(
    (Array.isArray(beforeRecords) ? beforeRecords : [])
      .map(record => [String(record?.siteCacheKey || '').trim(), record])
      .filter(([key]) => key)
  );

  return (Array.isArray(importedSites) ? importedSites : []).map(site => {
    const siteCacheKey = String(site?._siteCacheKey || site?.siteCacheKey || buildSiteCacheKey(site)).trim();
    const currentSiteName = String(site?.site_name || site?.siteName || '未命名站点').trim() || '未命名站点';
    const beforeRecord = beforeMap.get(siteCacheKey) || null;
    const beforeKeys = new Set((Array.isArray(beforeRecord?.tokens) ? beforeRecord.tokens : []).map(token => buildTokenKey(token)).filter(Boolean));
    const afterKeys = new Set((Array.isArray(site?.tokens) ? site.tokens : []).map(token => buildTokenKey(token)).filter(Boolean));
    let appendedCount = 0;
    afterKeys.forEach(key => {
      if (!beforeKeys.has(key)) appendedCount += 1;
    });

    if (!beforeRecord) {
      return {
        siteCacheKey,
        siteName: currentSiteName,
        tone: 'green',
        kind: 'new',
        text: '新站点',
      };
    }

    if (appendedCount > 0) {
      return {
        siteCacheKey,
        siteName: currentSiteName,
        tone: 'blue',
        kind: 'append',
        text: `追加 ${appendedCount} Key`,
      };
    }

    return {
      siteCacheKey,
      siteName: currentSiteName,
      tone: 'gold',
      kind: 'update',
      text: '已更新',
    };
  });
}
