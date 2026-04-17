function toFiniteNumber(value) {
  const num = Number(value);
  return Number.isFinite(num) ? num : null;
}

function formatQuotaAmount(rawQuota) {
  const quota = toFiniteNumber(rawQuota);
  if (quota == null) return '';
  const isDirectAmount = Math.abs(quota) < 100000;
  const amount = isDirectAmount ? quota : quota / 500000;
  return `$${amount.toFixed(3)}`;
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

export function isDisplayableQuotaLabel(value) {
  const text = String(value || '').trim();
  return /^\$\d/.test(text) || text === '无限';
}

export async function fetchQuotaLabelWithBatchLogic({ apiFetch, site, siteUrl }) {
  const normalizedSiteUrl = String(siteUrl || '').replace(/\/+$/, '').trim();
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

    let quota = null;
    let finalResStatus = 200;

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
        if (quota !== null) break;
      } else if ([401, 403].includes(res.status)) {
        break;
      }
    }

    if (quota !== null) {
      return formatQuotaAmount(quota);
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
