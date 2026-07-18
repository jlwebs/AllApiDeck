const URL_PATTERN = /https?:\/\/[^\s<>"']+/i;
const KNOWN_KEY_PREFIX_PATTERN = /^(?:sk-|g2a_|xai-|key-|api[_-])/i;
const TRAILING_URL_PUNCTUATION_PATTERN = /[),.;:!?，。；：！？）】]+$/u;
const TRAILING_TITLE_PUNCTUATION_PATTERN = /[,;:，；：]+$/u;

function extractUrl(line) {
  const match = String(line || '').match(URL_PATTERN);
  if (!match) return '';
  const candidate = match[0].replace(TRAILING_URL_PUNCTUATION_PATTERN, '');
  try {
    const parsed = new URL(candidate);
    if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.hostname) return '';
    return candidate;
  } catch {
    return '';
  }
}

function normalizeKeyCandidate(line) {
  return String(line || '')
    .trim()
    .replace(/^[`"'“‘]+|[`"'”’]+$/gu, '');
}

export function isLikelyClipboardApiKey(line) {
  const candidate = normalizeKeyCandidate(line);
  if (!candidate || candidate.length < 16 || candidate.length > 4096) return false;
  if (/\s/u.test(candidate) || extractUrl(candidate)) return false;
  if (!/^[A-Za-z0-9._+/=-]+$/u.test(candidate)) return false;
  if (KNOWN_KEY_PREFIX_PATTERN.test(candidate)) return true;
  return candidate.length >= 24
    && /[A-Za-z]/u.test(candidate)
    && /[0-9]/u.test(candidate);
}

function inferSiteName(lines, urlIndex, previousUrlIndex, siteUrl) {
  for (let index = urlIndex - 1; index > previousUrlIndex; index -= 1) {
    const candidate = String(lines[index] || '').trim();
    if (!candidate) break;
    if (extractUrl(candidate) || isLikelyClipboardApiKey(candidate)) continue;
    const cleaned = candidate.replace(TRAILING_TITLE_PUNCTUATION_PATTERN, '').trim();
    if (cleaned) return cleaned.slice(0, 120);
  }
  try {
    return new URL(siteUrl).host;
  } catch {
    return '未命名站点';
  }
}

export function extractSmartClipboardRecords(rawText) {
  const lines = String(rawText || '')
    .replace(/\r\n?/gu, '\n')
    .split('\n');
  const urlEntries = [];

  lines.forEach((line, index) => {
    const siteUrl = extractUrl(line);
    if (siteUrl) urlEntries.push({ index, siteUrl });
  });

  const records = [];
  const seen = new Set();
  const usedKeyIndexes = new Set();
  urlEntries.forEach((entry, entryIndex) => {
    const nextUrlIndex = urlEntries[entryIndex + 1]?.index ?? lines.length;
    const previousUrlIndex = urlEntries[entryIndex - 1]?.index ?? -1;
    let apiKey = '';
    let apiKeyIndex = -1;
    for (let index = entry.index + 1; index < nextUrlIndex; index += 1) {
      if (!String(lines[index] || '').trim()) break;
      if (!isLikelyClipboardApiKey(lines[index])) continue;
      apiKey = normalizeKeyCandidate(lines[index]);
      apiKeyIndex = index;
      break;
    }
    if (!apiKey) {
      for (let index = entry.index - 1; index > previousUrlIndex; index -= 1) {
        if (!String(lines[index] || '').trim()) break;
        if (usedKeyIndexes.has(index) || !isLikelyClipboardApiKey(lines[index])) continue;
        apiKey = normalizeKeyCandidate(lines[index]);
        apiKeyIndex = index;
        break;
      }
    }
    if (!apiKey) return;
    usedKeyIndexes.add(apiKeyIndex);

    const dedupeKey = `${entry.siteUrl.replace(/\/+$/u, '').toLowerCase()}::${apiKey}`;
    if (seen.has(dedupeKey)) return;
    seen.add(dedupeKey);

    records.push({
      sourceType: 'auto',
      siteName: inferSiteName(lines, entry.index, previousUrlIndex, entry.siteUrl),
      tokenName: '',
      siteUrl: entry.siteUrl,
      apiKey,
      modelsList: [],
      modelsText: '未提供模型信息',
      selectedModel: '',
      groupIds: [],
      status: 1,
    });
  });

  return records;
}

function base64UrlToBytes(value) {
  const normalized = String(value || '').replace(/-/g, '+').replace(/_/g, '/');
  const padding = normalized.length % 4 === 0 ? '' : '='.repeat(4 - (normalized.length % 4));
  const binary = atob(`${normalized}${padding}`);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

function remapClipboardPackageToken(value) {
  return String(value || '').replace(/[A-Za-z]/g, letter => {
    const code = letter.charCodeAt(0);
    if (code >= 65 && code <= 90) {
      return String.fromCharCode(90 - (code - 65));
    }
    if (code >= 97 && code <= 122) {
      return String.fromCharCode(122 - (code - 97));
    }
    return letter;
  });
}

async function readClipboardPackagePayload(encoded) {
  if (typeof DecompressionStream !== 'function') {
    throw new Error('当前环境不支持压缩导入');
  }
  const bytes = base64UrlToBytes(encoded);
  const decompressed = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('gzip'));
  return JSON.parse(await new Response(decompressed).text());
}

export async function resolveClipboardPackagePayload(value) {
  const encoded = String(value || '').trim();
  try {
    return await readClipboardPackagePayload(encoded);
  } catch (primaryError) {
    const fallbackEncoded = remapClipboardPackageToken(encoded);
    if (!fallbackEncoded || fallbackEncoded === encoded) throw primaryError;
    try {
      return await readClipboardPackagePayload(fallbackEncoded);
    } catch {
      throw primaryError;
    }
  }
}

export async function resolveClipboardImportRecords(rawText) {
  const text = String(rawText || '').trim();
  if (!text) throw new Error('剪贴板文本为空');

  if (text.startsWith('sk://')) {
    try {
      const payload = await resolveClipboardPackagePayload(text.slice('sk://'.length));
      const records = Array.isArray(payload?.records) ? payload.records : [];
      if (records.length === 0) throw new Error('导入包中没有记录');
      return { mode: 'package', records };
    } catch (packageError) {
      const records = extractSmartClipboardRecords(text);
      if (records.length > 0) return { mode: 'smart', records };
      throw new Error(`sk:// 导入包解析失败：${packageError.message || '格式无效'}`);
    }
  }

  const records = extractSmartClipboardRecords(text);
  if (records.length === 0) throw new Error('未识别到 URL 与 API Key 组合');
  return { mode: 'smart', records };
}
