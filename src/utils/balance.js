export function isDisplayableQuotaLabel(value) {
  const text = String(value || '').trim();
  return /^\$\d/.test(text) || /^无限/.test(text);
}

export async function fetchQuotaLabelWithBatchLogic({ apiFetch, site, siteUrl }) {
  const normalizedSiteUrl = String(siteUrl || '').replace(/\/+$/, '').trim();
  const rawId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
  const userId = /^\d+$/.test(String(rawId)) ? String(rawId) : '';
  const auth = site?.account_info?.access_token || site?.access_token || site?.tokens?.[0]?.key;

  if (!auth || !normalizedSiteUrl) {
    return '无授权';
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
      const proxyUrl = `/api/proxy-get?url=${encodeURIComponent(url)}&uid=${userId ? String(userId) : ''}`;

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
      const isDirectAmount = quota < 100000;
      const finalAmount = isDirectAmount ? Number(quota).toFixed(3) : (quota / 500000).toFixed(3);
      return `$${finalAmount}`;
    }

    return `获取失败(${finalResStatus})`;
  } catch (error) {
    return error?.name === 'AbortError' ? '请求超时' : '网络错误';
  } finally {
    clearTimeout(timer);
  }
}
