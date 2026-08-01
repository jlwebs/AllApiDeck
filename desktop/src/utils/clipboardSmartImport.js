const URL_PATTERN = /https?:\/\/[^\s<>"'`，。；：！？、）】》』”’;,!?]+/giu;
const KEY_TOKEN_PATTERN = /[A-Za-z0-9][A-Za-z0-9._+/=\-]{15,}/gu;
const KNOWN_KEY_PREFIX_PATTERN = /^(?:sk-|gsk_|xai-|g2a_|hf_|AIza|key[_-]|api[_-]|token[_-])/iu;
const KEY_FIELD_PATTERN = /(?:["'`]?)(api[_-]?key|apikey|key|token|access[_-]?token|auth[_-]?token|authorization|secret)(?:["'`]?)\s*[:=：]\s*["'`<\[\s]*(?:bearer\s+)?([A-Za-z0-9][A-Za-z0-9._+/=\-]{15,})/giu;
const FIELD_NAME_PATTERN = /(?:["'`]?)([A-Za-z][A-Za-z0-9_-]{0,40})(?:["'`]?)\s*[:=：]\s*$/u;
const TRAILING_URL_PUNCTUATION_PATTERN = /[),.;:!?，。；：！？、）】》』”’}\]]+$/gu;
const TRAILING_TITLE_PUNCTUATION_PATTERN = /[,;:，；：]+$/u;
const TRAILING_KEY_PUNCTUATION_PATTERN = /[`"'”’>)}\],;:，。；：!?！？]+$/gu;
const URL_FIELD_NAMES = new Set([
  'url',
  'baseurl',
  'apiurl',
  'apibase',
  'apibaseurl',
  'endpoint',
  'host',
  'server',
  'proxyurl',
  'apiendpoint',
]);
const KEY_FIELD_NAMES = new Set([
  'apikey',
  'key',
  'token',
  'accesstoken',
  'authtoken',
  'authorization',
  'secret',
]);

function normalizeClipboardText(rawText) {
  return String(rawText || '')
    .replace(/\r\n?/gu, '\n')
    .replace(/\uFEFF|\u200B|\u200C|\u200D/gu, '')
    .replace(/\u00A0/gu, ' ')
    .replace(/\\u002f/giu, '/')
    .replace(/\\\//gu, '/');
}

function normalizeFieldName(value) {
  return String(value || '').trim().toLowerCase().replace(/[_-]/gu, '');
}

function fieldNameBefore(text, index) {
  const before = text.slice(Math.max(0, index - 96), index);
  const match = before.match(FIELD_NAME_PATTERN);
  return normalizeFieldName(match?.[1] || '');
}

function normalizeUrlCandidate(value) {
  const candidate = String(value || '').trim().replace(TRAILING_URL_PUNCTUATION_PATTERN, '');
  if (!candidate) return '';
  try {
    const parsed = new URL(candidate);
    if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.hostname) return '';
    return candidate;
  } catch {
    return '';
  }
}

function normalizeKeyCandidate(value) {
  return String(value || '')
    .trim()
    .replace(/^[`"'“‘<({\[]+/gu, '')
    .replace(TRAILING_KEY_PUNCTUATION_PATTERN, '');
}

export function isLikelyClipboardApiKey(value) {
  const candidate = normalizeKeyCandidate(value);
  if (!candidate || candidate.length < 16 || candidate.length > 4096) return false;
  if (/\s/u.test(candidate) || normalizeUrlCandidate(candidate)) return false;
  if (!/^[A-Za-z0-9._+/=\-]+$/u.test(candidate)) return false;
  if (KNOWN_KEY_PREFIX_PATTERN.test(candidate)) return true;
  return candidate.length >= 24
    && /[A-Za-z]/u.test(candidate)
    && /[0-9]/u.test(candidate);
}

function normalizeSiteNameKey(value) {
  return String(value || '').trim().toLocaleLowerCase();
}

export function ensureUniqueClipboardSiteName(baseName, records = [], currentRecord = null) {
  const normalizedBaseName = String(baseName || '').trim() || '未命名站点';
  const usedNames = new Set(
    (Array.isArray(records) ? records : [])
      .filter(record => record && record !== currentRecord)
      .map(record => normalizeSiteNameKey(record.siteName))
      .filter(Boolean)
  );
  if (!usedNames.has(normalizeSiteNameKey(normalizedBaseName))) return normalizedBaseName;

  let suffix = 2;
  let candidate = `${normalizedBaseName} ${suffix}`;
  while (usedNames.has(normalizeSiteNameKey(candidate))) {
    suffix += 1;
    candidate = `${normalizedBaseName} ${suffix}`;
  }
  return candidate;
}

function getLineIndex(text, index) {
  let lineIndex = 0;
  for (let cursor = 0; cursor < index; cursor += 1) {
    if (text[cursor] === '\n') lineIndex += 1;
  }
  return lineIndex;
}

function extractUrlCandidates(text) {
  const entries = [];
  for (const match of text.matchAll(URL_PATTERN)) {
    const rawValue = match[0];
    const siteUrl = normalizeUrlCandidate(rawValue);
    if (!siteUrl) continue;
    const rawOffset = rawValue.indexOf(siteUrl);
    const start = match.index + Math.max(0, rawOffset);
    entries.push({
      siteUrl,
      start,
      end: start + siteUrl.length,
      lineIndex: getLineIndex(text, start),
      fieldName: fieldNameBefore(text, start),
    });
  }
  return entries;
}

function rangesOverlap(leftStart, leftEnd, rightStart, rightEnd) {
  return leftStart < rightEnd && rightStart < leftEnd;
}

function isInsideUrlCandidate(start, end, urlEntries) {
  return urlEntries.some(entry => rangesOverlap(start, end, entry.start, entry.end));
}

function hasKeyFieldContext(text, start) {
  const before = text.slice(Math.max(0, start - 96), start);
  return /(?:api[_-]?key|apikey|key|token|access[_-]?token|auth[_-]?token|authorization|secret)\s*[:=：]\s*(?:bearer\s+)?$/iu.test(before);
}

function isStandaloneToken(text, start, end) {
  const lineStart = text.lastIndexOf('\n', Math.max(0, start - 1)) + 1;
  const lineEndIndex = text.indexOf('\n', end);
  const lineEnd = lineEndIndex === -1 ? text.length : lineEndIndex;
  const before = text.slice(lineStart, start).trim();
  const after = text.slice(end, lineEnd).trim();
  const separatorPattern = /^[`"'“‘”’<({\[\]}>：:;,，。；|=]*$/u;
  return separatorPattern.test(before) && separatorPattern.test(after);
}

function extractKeyCandidates(text, urlEntries) {
  const candidates = new Map();
  const addCandidate = (value, start, fieldName = '') => {
    const apiKey = normalizeKeyCandidate(value);
    if (!isLikelyClipboardApiKey(apiKey)) return;
    const offset = String(value || '').indexOf(apiKey);
    const normalizedStart = start + Math.max(0, offset);
    const identity = `${normalizedStart}:${normalizedStart + apiKey.length}:${apiKey}`;
    const existing = candidates.get(identity);
    candidates.set(identity, {
      apiKey,
      start: normalizedStart,
      end: normalizedStart + apiKey.length,
      lineIndex: getLineIndex(text, normalizedStart),
      fieldName: existing?.fieldName || normalizeFieldName(fieldName),
    });
  };

  for (const match of text.matchAll(KEY_FIELD_PATTERN)) {
    const value = match[2];
    const valueOffset = match[0].lastIndexOf(value);
    addCandidate(value, match.index + Math.max(0, valueOffset), match[1]);
  }

  for (const match of text.matchAll(KEY_TOKEN_PATTERN)) {
    const value = normalizeKeyCandidate(match[0]);
    const start = match.index;
    const end = start + value.length;
    if (isInsideUrlCandidate(start, end, urlEntries)) continue;
    const knownPrefix = KNOWN_KEY_PREFIX_PATTERN.test(value);
    if (!knownPrefix && !hasKeyFieldContext(text, start) && !isStandaloneToken(text, start, end)) continue;
    addCandidate(value, start, knownPrefix || hasKeyFieldContext(text, start) ? fieldNameBefore(text, start) : '');
  }

  return Array.from(candidates.values()).sort((left, right) => left.start - right.start);
}

function buildStructuredSpans(text) {
  const stack = [];
  const spans = [];
  let quote = '';
  let escaped = false;
  for (let index = 0; index < text.length; index += 1) {
    const character = text[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (character === '\\') {
        escaped = true;
      } else if (character === quote) {
        quote = '';
      }
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }
    if (character === '{' || character === '[') {
      stack.push({ character, start: index });
      continue;
    }
    if (character !== '}' && character !== ']') continue;
    const expected = character === '}' ? '{' : '[';
    const stackIndex = stack.map(item => item.character).lastIndexOf(expected);
    if (stackIndex < 0) continue;
    const opening = stack[stackIndex];
    stack.splice(stackIndex, 1);
    spans.push({ start: opening.start, end: index + 1 });
  }
  return spans;
}

function getSmallestContainingSpan(spans, start, end) {
  return spans
    .filter(span => span.start <= start && end <= span.end)
    .sort((left, right) => (left.end - left.start) - (right.end - right.start))[0] || null;
}

function isReasonablePair(urlEntry, keyEntry) {
  const distance = Math.abs(urlEntry.start - keyEntry.start);
  const lineDistance = Math.abs(urlEntry.lineIndex - keyEntry.lineIndex);
  return distance <= 8192 && lineDistance <= 80;
}

function pairScore(urlEntry, keyEntry, text, spans) {
  if (!isReasonablePair(urlEntry, keyEntry)) return Number.POSITIVE_INFINITY;
  let score = Math.abs(urlEntry.start - keyEntry.start);
  if (urlEntry.lineIndex === keyEntry.lineIndex) score -= 500;
  else if (Math.abs(urlEntry.lineIndex - keyEntry.lineIndex) <= 1) score -= 160;
  if (URL_FIELD_NAMES.has(urlEntry.fieldName) && KEY_FIELD_NAMES.has(keyEntry.fieldName)) score -= 180;
  const urlSpan = getSmallestContainingSpan(spans, urlEntry.start, urlEntry.end);
  const keySpan = getSmallestContainingSpan(spans, keyEntry.start, keyEntry.end);
  if (urlSpan && keySpan && urlSpan.start === keySpan.start && urlSpan.end === keySpan.end) score -= 1000;
  if (text.slice(Math.min(urlEntry.end, keyEntry.end), Math.max(urlEntry.start, keyEntry.start)).includes('\n\n')) score += 80;
  return score;
}

function pairClipboardEntries(urlEntries, keyEntries, text) {
  const spans = buildStructuredSpans(text);
  const pairs = [];
  const usedUrls = new Set();
  const usedKeys = new Set();
  const assign = (urlEntry, keyEntry) => {
    if (!urlEntry || !keyEntry || usedUrls.has(urlEntry) || usedKeys.has(keyEntry)) return false;
    if (!isReasonablePair(urlEntry, keyEntry)) return false;
    usedUrls.add(urlEntry);
    usedKeys.add(keyEntry);
    pairs.push({ urlEntry, keyEntry });
    return true;
  };

  for (const urlEntry of urlEntries) {
    const sameObjectCandidates = keyEntries
      .filter(keyEntry => !usedKeys.has(keyEntry))
      .map(keyEntry => ({ keyEntry, score: pairScore(urlEntry, keyEntry, text, spans) }))
      .filter(candidate => {
        const urlSpan = getSmallestContainingSpan(spans, urlEntry.start, urlEntry.end);
        const keySpan = getSmallestContainingSpan(spans, candidate.keyEntry.start, candidate.keyEntry.end);
        return urlSpan && keySpan && urlSpan.start === keySpan.start && urlSpan.end === keySpan.end;
      })
      .sort((left, right) => left.score - right.score);
    if (sameObjectCandidates.length > 0) assign(urlEntry, sameObjectCandidates[0].keyEntry);
  }

  const remainingUrls = urlEntries.filter(entry => !usedUrls.has(entry));
  const remainingKeys = keyEntries.filter(entry => !usedKeys.has(entry));
  if (remainingUrls.length === remainingKeys.length && remainingUrls.length > 0) {
    const orderedPairsAreReasonable = remainingUrls.every((urlEntry, index) => isReasonablePair(urlEntry, remainingKeys[index]));
    if (orderedPairsAreReasonable) {
      remainingUrls.forEach((urlEntry, index) => assign(urlEntry, remainingKeys[index]));
    }
  }

  const candidates = [];
  for (const urlEntry of urlEntries) {
    if (usedUrls.has(urlEntry)) continue;
    for (const keyEntry of keyEntries) {
      if (usedKeys.has(keyEntry)) continue;
      candidates.push({
        urlEntry,
        keyEntry,
        score: pairScore(urlEntry, keyEntry, text, spans),
      });
    }
  }
  candidates
    .filter(candidate => Number.isFinite(candidate.score))
    .sort((left, right) => left.score - right.score || left.urlEntry.start - right.urlEntry.start || left.keyEntry.start - right.keyEntry.start)
    .forEach(candidate => assign(candidate.urlEntry, candidate.keyEntry));

  return pairs.sort((left, right) => left.urlEntry.start - right.urlEntry.start);
}

function extractUrl(line) {
  return extractUrlCandidates(normalizeClipboardText(line))[0]?.siteUrl || '';
}

function inferSiteName(lines, urlIndex, previousUrlIndex, siteUrl) {
  for (let index = urlIndex - 1; index > previousUrlIndex; index -= 1) {
    const candidate = String(lines[index] || '').trim();
    if (!candidate) break;
    if (extractUrl(candidate) || isLikelyClipboardApiKey(candidate)) continue;
    const cleaned = candidate.replace(TRAILING_TITLE_PUNCTUATION_PATTERN, '').trim();
    if (cleaned && !/[{}"]+/u.test(cleaned)) return cleaned.slice(0, 120);
  }
  try {
    return new URL(siteUrl).host;
  } catch {
    return '未命名站点';
  }
}

export function extractSmartClipboardRecords(rawText) {
  const text = normalizeClipboardText(rawText);
  const lines = text.split('\n');
  const urlEntries = extractUrlCandidates(text);
  const keyEntries = extractKeyCandidates(text, urlEntries);
  const pairs = pairClipboardEntries(urlEntries, keyEntries, text);
  const records = [];
  const seen = new Set();

  pairs.forEach(({ urlEntry, keyEntry }) => {
    const dedupeKey = `${urlEntry.siteUrl.replace(/\/+$/u, '').toLowerCase()}::${keyEntry.apiKey}`;
    if (seen.has(dedupeKey)) return;
    seen.add(dedupeKey);
    const previousUrlEntry = urlEntries[urlEntries.indexOf(urlEntry) - 1];
    records.push({
      sourceType: 'auto',
      siteName: inferSiteName(lines, urlEntry.lineIndex, previousUrlEntry?.lineIndex ?? -1, urlEntry.siteUrl),
      tokenName: '',
      siteUrl: urlEntry.siteUrl,
      apiKey: keyEntry.apiKey,
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
