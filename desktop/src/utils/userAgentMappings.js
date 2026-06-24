export const DEFAULT_CODEX_TARGET_UA = `originator: Codex Desktop
user-agent: Codex Desktop/0.142.0-alpha.6 (Windows 10.0.19044; x86_64) unknown (Codex Desktop; 26.616.51431)`;

export const DEFAULT_USER_AGENT_MAPPINGS = Object.freeze([
  Object.freeze({
    modelContains: 'gpt',
    targetUA: DEFAULT_CODEX_TARGET_UA,
  }),
  Object.freeze({
    modelContains: 'claude',
    targetUA: 'User-Agent: claude-cli/2.1.129 (external, cli); x-app: cli',
  }),
]);

const HEADER_NAME_SPECIAL_CASES = {
  accept: 'Accept',
  authorization: 'Authorization',
  'content-type': 'Content-Type',
  originator: 'Originator',
  'user-agent': 'User-Agent',
  'x-api-key': 'X-Api-Key',
};

export function cloneDefaultUserAgentMappings() {
  return DEFAULT_USER_AGENT_MAPPINGS.map(entry => ({ ...entry }));
}

export function normalizeUserAgentMappingEntry(value) {
  return {
    modelContains: String(value?.modelContains || value?.model || '').trim(),
    targetUA: String(value?.targetUA || value?.targetUa || value?.target || value?.headers || '').trim(),
  };
}

export function normalizeUserAgentMappings(value, options = {}) {
  const fallbackToDefault = options?.fallbackToDefault === true;
  if (value == null) {
    return fallbackToDefault ? cloneDefaultUserAgentMappings() : [];
  }

  const normalized = (Array.isArray(value) ? value : [])
    .map(normalizeUserAgentMappingEntry);

  if (!normalized.length && fallbackToDefault) {
    return cloneDefaultUserAgentMappings();
  }

  return normalized;
}

function canonicalizeHeaderName(rawName) {
  const normalized = String(rawName || '').trim().toLowerCase();
  if (!normalized) {
    return '';
  }
  if (HEADER_NAME_SPECIAL_CASES[normalized]) {
    return HEADER_NAME_SPECIAL_CASES[normalized];
  }
  return normalized
    .split('-')
    .filter(Boolean)
    .map(segment => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join('-');
}

function isHeaderNameChar(char) {
  return /[A-Za-z0-9-]/.test(char);
}

function looksLikeHeaderStart(text, startIndex) {
  let index = startIndex;
  while (index < text.length && /\s/.test(text[index])) {
    index += 1;
  }
  const nameStart = index;
  while (index < text.length && isHeaderNameChar(text[index])) {
    index += 1;
  }
  return index > nameStart && text[index] === ':';
}

function splitMappedHeaderSegments(text) {
  const segments = [];
  let start = 0;

  for (let index = 0; index < text.length; index += 1) {
    const current = text[index];
    if (current === '\r' || current === '\n') {
      segments.push(text.slice(start, index));
      if (current === '\r' && text[index + 1] === '\n') {
        index += 1;
      }
      start = index + 1;
      continue;
    }

    if (current !== ';') {
      continue;
    }

    if (!looksLikeHeaderStart(text, index + 1)) {
      continue;
    }

    segments.push(text.slice(start, index));
    start = index + 1;
  }

  segments.push(text.slice(start));
  return segments
    .map(segment => segment.trim())
    .filter(Boolean);
}

export function parseMappedUserAgentHeaders(rawValue) {
  const text = String(rawValue || '').trim();
  if (!text) {
    return {};
  }

  const segments = splitMappedHeaderSegments(text);
  const parsedHeaders = {};
  let sawHeaderSyntax = false;

  segments.forEach(segment => {
    const separatorIndex = segment.indexOf(':');
    if (separatorIndex <= 0) {
      return;
    }
    const rawName = segment.slice(0, separatorIndex).trim();
    const rawHeaderValue = segment.slice(separatorIndex + 1).trim();
    if (!/^[A-Za-z0-9-]+$/.test(rawName)) {
      return;
    }
    sawHeaderSyntax = true;
    parsedHeaders[canonicalizeHeaderName(rawName)] = rawHeaderValue;
  });

  if (sawHeaderSyntax) {
    return parsedHeaders;
  }

  return {
    'User-Agent': text,
  };
}

export function resolveMappedHeadersForModel(model, mappings) {
  const normalizedModel = String(model || '').trim().toLowerCase();
  if (!normalizedModel) {
    return null;
  }

  const normalizedMappings = normalizeUserAgentMappings(mappings, { fallbackToDefault: false });
  for (const entry of normalizedMappings) {
    if (!entry.modelContains || !entry.targetUA) {
      continue;
    }
    if (!normalizedModel.includes(entry.modelContains.toLowerCase())) {
      continue;
    }
    const headers = parseMappedUserAgentHeaders(entry.targetUA);
    if (!Object.keys(headers).length) {
      continue;
    }
    return {
      match: entry.modelContains,
      headers,
    };
  }

  return null;
}
