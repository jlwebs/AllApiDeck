function toFiniteNumber(value) {
  const normalized = typeof value === 'string'
    ? value.replace(/[$,\s]/g, '')
    : value;
  const num = Number(normalized);
  return Number.isFinite(num) ? num : null;
}

function formatQuotaAmount(rawQuota) {
  const quota = toFiniteNumber(rawQuota);
  if (quota == null) return '';
  const isDirectAmount = Math.abs(quota) < 100000;
  const amount = isDirectAmount ? quota : quota / 500000;
  return `$${amount.toFixed(3)}`;
}

function normalizeQuotaBaseUrl(rawUrl) {
  return String(rawUrl || '')
    .replace(/\/+$/, '')
    .replace(/\/v1\/(?:chat\/completions|responses|completions)$/i, '/v1')
    .trim();
}

function isMeaningfulQuotaLabel(label) {
  const text = String(label || '').trim();
  if (!text) return false;
  if (text === '无限') return true;
  const normalized = text.replace(/USD$/i, '').replace(/^\$/, '').trim();
  const amount = Number(normalized);
  return Number.isFinite(amount) && Math.abs(amount) > 0;
}

function formatQuotaLabelFromTokens(tokens) {
  const list = Array.isArray(tokens) ? tokens : [];
  if (!list.length) return '';

  let totalRemainQuota = 0;
  let hasFiniteRemainQuota = false;

  for (const token of list) {
    const unlimitedQuota =
      token?.unlimited_quota === true ||
      token?.unlimitedQuota === true;
    const remainQuota = toFiniteNumber(
      token?.remain_quota ??
      token?.remainQuota ??
      token?.quota ??
      token?.balance
    );

    if (unlimitedQuota || (remainQuota != null && remainQuota < 0)) {
      return '无限';
    }
    if (remainQuota != null) {
      totalRemainQuota += remainQuota;
      hasFiniteRemainQuota = true;
    }
  }

  if (!hasFiniteRemainQuota) return '';
  return formatQuotaAmount(totalRemainQuota);
}

function uniqueValues(list) {
  return Array.from(new Set(list.filter(Boolean)));
}

function buildUsageEndpoints(normalizedSiteUrl) {
  const base = normalizeQuotaBaseUrl(normalizedSiteUrl);
  if (!base) return [];
  const withoutV1 = base.replace(/\/v1$/i, '');
  return uniqueValues([
    /\/v1$/i.test(base) ? `${base}/usage` : `${base}/v1/usage`,
    `${withoutV1}/v1/usage`,
    `${base}/usage`,
  ]);
}

function pickFiniteField(source, keys) {
  if (!source || typeof source !== 'object') return null;
  for (const key of keys) {
    if (!Object.prototype.hasOwnProperty.call(source, key)) continue;
    const value = toFiniteNumber(source[key]);
    if (value != null) return value;
  }
  return null;
}

function extractQuotaFromUsagePayload(payload) {
  const objects = [
    payload,
    payload?.data,
    payload?.usage,
    payload?.quota,
    payload?.balance,
    payload?.billing,
    payload?.account,
    payload?.user,
    payload?.subscription,
    payload?.data?.usage,
    payload?.data?.quota,
    payload?.data?.balance,
    payload?.data?.billing,
    payload?.data?.account,
    payload?.data?.user,
    payload?.data?.subscription,
  ].filter(item => item && typeof item === 'object');

  const pairFields = [
    ['total_granted', 'total_used'],
    ['totalGranted', 'totalUsed'],
    ['total_quota', 'used_quota'],
    ['totalQuota', 'usedQuota'],
    ['hard_limit_usd', 'total_usage'],
  ];

  for (const item of objects) {
    for (const [totalKey, usedKey] of pairFields) {
      const total = toFiniteNumber(item?.[totalKey]);
      const used = toFiniteNumber(item?.[usedKey]);
      if (total == null || used == null) continue;
      const normalizedUsed = usedKey === 'total_usage' && Math.abs(used) >= 100000 ? used / 100 : used;
      return total - normalizedUsed;
    }
  }

  const quotaFields = [
    'remaining_balance',
    'remain_balance',
    'available_balance',
    'available_amount',
    'balance',
    'remaining',
    'remaining_amount',
    'remain',
    'remain_amount',
    'available',
    'total_available',
    'totalAvailable',
    'credit',
    'credits',
    'amount',
    'quota',
    'remain_quota',
    'remainQuota',
    'available_quota',
    'availableQuota',
    'hard_limit_usd',
    'total_quota',
    'totalQuota',
  ];

  for (const item of objects) {
    const value = pickFiniteField(item, quotaFields);
    if (value != null) return value;
  }

  return null;
}

async function tryFetchUsageQuotaLabel({ apiFetch, normalizedSiteUrl, auth, signal }) {
  let lastStatus = 0;

  for (const endpoint of buildUsageEndpoints(normalizedSiteUrl)) {
    const proxyUrl = `/api/proxy-get?url=${encodeURIComponent(endpoint)}`;
    const res = await apiFetch(proxyUrl, {
      headers: { Authorization: `Bearer ${auth}` },
      signal,
    });
    lastStatus = res.status;

    if ([401, 403].includes(res.status)) continue;
    if (!res.ok) continue;

    let json = null;
    try {
      json = await res.json();
    } catch {
      continue;
    }
    const quota = extractQuotaFromUsagePayload(json);
    if (quota == null) continue;

    const label = formatQuotaAmount(quota);
    if (isDisplayableQuotaLabel(label)) {
      return { label, status: res.status, forbidden: false };
    }
  }

  return { label: '', status: lastStatus };
}

export function isDisplayableQuotaLabel(value) {
  const text = String(value || '').trim();
  return /^\$\d/.test(text) || text === '无限';
}

export async function fetchQuotaLabelWithBatchLogic({ apiFetch, site, siteUrl }) {
  const normalizedSiteUrl = normalizeQuotaBaseUrl(siteUrl);
  const rawId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
  const userId = /^\d+$/.test(String(rawId)) ? String(rawId) : '';
  const auth = site?.account_info?.access_token || site?.access_token || site?.tokens?.[0]?.key || '';
  const tokenFallbackLabel = formatQuotaLabelFromTokens(site?.tokens);

  if (!normalizedSiteUrl) {
    return tokenFallbackLabel || '无站点地址';
  }

  if (!auth) {
    return tokenFallbackLabel || '无授权';
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 15000);

  try {
    const isSub2Api = site?.site_type === 'sub2api';
    const endpoints = isSub2Api ? ['/api/v1/auth/me', '/api/user/self'] : ['/api/user/self', '/api/v1/auth/me'];
    let lastKnownQuotaLabel = '';
    let finalResStatus = 200;

    const usageResult = await tryFetchUsageQuotaLabel({
      apiFetch,
      normalizedSiteUrl,
      auth,
      signal: controller.signal,
    });
    if (usageResult.label) return usageResult.label;
    if (usageResult.status) finalResStatus = usageResult.status;

    for (let attempt = 0; attempt < 3; attempt += 1) {
      let quota = null;
      let shouldStop = false;

      for (const endpoint of endpoints) {
        const url = `${normalizedSiteUrl}${endpoint}`;
        const proxyUrl = `/api/proxy-get?url=${encodeURIComponent(url)}&uid=${userId}`;

        const res = await apiFetch(proxyUrl, {
          headers: { Authorization: `Bearer ${auth}` },
          signal: controller.signal,
        });

        finalResStatus = res.status;
        if (res.ok) {
          const json = await res.json();
          quota = json?.data?.quota
            ?? json?.quota
            ?? json?.data?.user?.quota
            ?? json?.user?.quota
            ?? json?.data?.balance
            ?? json?.balance
            ?? json?.total_quota
            ?? null;
          if (quota !== null) {
            const label = formatQuotaAmount(quota);
            if (isDisplayableQuotaLabel(label)) {
              lastKnownQuotaLabel = label;
              if (isMeaningfulQuotaLabel(label)) {
                return label;
              }
            }
            break;
          }
        } else if ([401, 403].includes(res.status)) {
          shouldStop = true;
          break;
        }
      }

      if (shouldStop) {
        break;
      }
    }

    if (lastKnownQuotaLabel) {
      return lastKnownQuotaLabel;
    }

    if (tokenFallbackLabel) {
      return tokenFallbackLabel;
    }
    if ([401, 403].includes(finalResStatus)) {
      return '登录态失效';
    }
    return `获取失败(${finalResStatus})`;
  } catch (error) {
    if (tokenFallbackLabel) {
      return tokenFallbackLabel;
    }
    return error?.name === 'AbortError' ? '请求超时' : '网络错误';
  } finally {
    clearTimeout(timer);
  }
}
