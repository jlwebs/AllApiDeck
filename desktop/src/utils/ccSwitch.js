const OPENAI_COMPATIBLE_TARGET_APPS = new Set(['codex', 'opencode', 'openclaw']);

function normalizeSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function serializeUrl(url) {
  const pathname = String(url?.pathname || '').replace(/\/+$/, '') || '/';
  const suffix = `${url.search || ''}${url.hash || ''}`;
  if (pathname === '/') {
    return `${url.origin}${suffix}`;
  }
  return `${url.origin}${pathname}${suffix}`;
}

function shouldAppendV1Path(pathname) {
  const normalizedPath = String(pathname || '').trim();
  if (!normalizedPath || normalizedPath === '/') return true;
  if (/^\/v\d+(?:\/|$)/i.test(normalizedPath)) return false;
  if (/\/(?:api|openai|backend-api|anthropic|claude|codex|responses)(?:\/|$)/i.test(normalizedPath)) {
    return false;
  }
  return false;
}

export function normalizeCCSwitchEndpoint(rawUrl, targetApp = 'claude') {
  const normalizedUrl = normalizeSiteUrl(rawUrl);
  if (!normalizedUrl) return '';

  const normalizedTargetApp = String(targetApp || '').trim().toLowerCase();
  if (!OPENAI_COMPATIBLE_TARGET_APPS.has(normalizedTargetApp)) {
    return normalizedUrl;
  }

  try {
    const parsedUrl = new URL(normalizedUrl);
    if (!/^https?:$/i.test(parsedUrl.protocol)) {
      return normalizedUrl;
    }
    if (!shouldAppendV1Path(parsedUrl.pathname)) {
      return normalizedUrl;
    }
    parsedUrl.pathname = '/v1';
    return serializeUrl(parsedUrl);
  } catch {
    if (/^https?:\/\/[^/]+$/i.test(normalizedUrl)) {
      return `${normalizedUrl}/v1`;
    }
    return normalizedUrl;
  }
}
