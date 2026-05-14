const URL_IN_TEXT_PATTERN = /https?:\/\/[^\s]+/gi;

function normalizeSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function trimPathname(pathname) {
  const normalized = String(pathname || '').trim().replace(/\/+$/, '');
  return normalized || '/';
}

function hasVersionPrefix(pathname) {
  return /^\/v\d+(?:\/|$)/i.test(trimPathname(pathname));
}

function appendV1ToBaseUrl(rawUrl) {
  const normalized = normalizeSiteUrl(rawUrl);
  if (!normalized) return '';
  try {
    const url = new URL(normalized);
    if (!/^https?:$/i.test(url.protocol)) return normalized;
    const pathname = trimPathname(url.pathname);
    if (hasVersionPrefix(pathname)) return normalized;
    const targetPath = pathname === '/' ? '/v1' : `${pathname}/v1`;
    url.pathname = targetPath;
    const suffix = `${url.search || ''}${url.hash || ''}`;
    const serializedPath = trimPathname(url.pathname);
    return serializedPath === '/'
      ? `${url.origin}${suffix}`
      : `${url.origin}${serializedPath}${suffix}`;
  } catch {
    if (/^https?:\/\/[^/]+$/i.test(normalized)) {
      return `${normalized}/v1`;
    }
    return normalized;
  }
}

function extractResolvedEndpointFromText(text) {
  const source = String(text || '');
  if (!source.trim()) return '';
  const directMatch = source.match(/命中端点:\s*(https?:\/\/[^\s]+)/i);
  if (directMatch?.[1]) {
    return normalizeSiteUrl(directMatch[1]);
  }
  const urls = source.match(URL_IN_TEXT_PATTERN) || [];
  return normalizeSiteUrl(urls[0] || '');
}

export function extractLatestResolvedEndpoint(record) {
  const direct = normalizeSiteUrl(record?.quickTestResolvedEndpoint);
  if (direct) return direct;
  const fromRemark = extractResolvedEndpointFromText(record?.quickTestRemark);
  if (fromRemark) return fromRemark;
  const fromContent = extractResolvedEndpointFromText(record?.quickTestResponseContent);
  return fromContent;
}

function shouldAppendV1ByResolvedEndpoint(baseUrl, resolvedEndpoint) {
  const normalizedBase = normalizeSiteUrl(baseUrl);
  const normalizedResolved = normalizeSiteUrl(resolvedEndpoint);
  if (!normalizedBase || !normalizedResolved) return false;
  try {
    const base = new URL(normalizedBase);
    const resolved = new URL(normalizedResolved);
    if (base.origin.toLowerCase() !== resolved.origin.toLowerCase()) return false;
    const basePath = trimPathname(base.pathname);
    if (hasVersionPrefix(basePath)) return false;
    const resolvedPath = trimPathname(resolved.pathname);
    if (!hasVersionPrefix(resolvedPath)) return false;
    const expectedPrefix = (basePath === '/' ? '/v1' : `${basePath}/v1`).toLowerCase();
    const resolvedLower = resolvedPath.toLowerCase();
    return resolvedLower === expectedPrefix || resolvedLower.startsWith(`${expectedPrefix}/`);
  } catch {
    return false;
  }
}

export function resolveOpenAIExportBaseUrl(record, rawBaseUrl = '') {
  const baseUrl = normalizeSiteUrl(rawBaseUrl || record?.siteUrl);
  if (!baseUrl) return '';
  const resolvedEndpoint = extractLatestResolvedEndpoint(record);
  if (!resolvedEndpoint) return baseUrl;
  if (shouldAppendV1ByResolvedEndpoint(baseUrl, resolvedEndpoint)) {
    return appendV1ToBaseUrl(baseUrl);
  }
  return baseUrl;
}

