import {
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyLocalSnapshot,
  isAdvancedProxyAppReady,
} from './advancedProxyBridge.js';

const OPENCLAW_DEFAULT_CONFIG = {
  models: {
    mode: 'merge',
    providers: {},
  },
};

const PROXY_MANAGED_TOKEN = 'PROXY_MANAGED';
const ADVANCED_PROXY_PROVIDER_NAME = 'AllApiDeck Advanced Proxy';

export const DESKTOP_CONFIG_APPS = [
  { id: 'claude', label: 'Claude' },
  { id: 'codex', label: 'Codex' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'openclaw', label: 'OpenClaw' },
];

export function createDesktopConfigDraft(record) {
  const defaultModel =
    String(record?.selectedModel || '').trim() ||
    String(record?.quickTestModel || '').trim() ||
    pickFallbackModel(record?.modelsList, record?.modelsText);
  const providerName = String(record?.siteName || 'Custom Provider').trim() || 'Custom Provider';
  const endpoint = String(record?.siteUrl || '').trim();
  const apiKey = String(record?.apiKey || '').trim();
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();

  return {
    selectedApps: [],
    providerName,
    providerKey: 'custom',
    forceCustomProviderKey: true,
    endpoint,
    apiKey,
    model: defaultModel || 'gpt-4o-mini',
    claudeBaseUrl: endpoint,
    claudeApiKeyField: 'ANTHROPIC_AUTH_TOKEN',
    claudeUseAdvancedProxy: isAdvancedProxyAppReady('claude', advancedProxySnapshot),
    codexBaseUrl: endpoint,
    codexUseAdvancedProxy: isAdvancedProxyAppReady('codex', advancedProxySnapshot),
    opencodeBaseUrl: endpoint,
    opencodeUseAdvancedProxy: isAdvancedProxyAppReady('opencode', advancedProxySnapshot),
    opencodeNpm: '@ai-sdk/openai-compatible',
    openclawBaseUrl: endpoint,
    openclawUseAdvancedProxy: isAdvancedProxyAppReady('openclaw', advancedProxySnapshot),
    openclawApi: 'openai-completions',
  };
}

export function buildDesktopConfigPreview(draft, snapshot) {
  const selectedApps = Array.isArray(draft?.selectedApps) ? draft.selectedApps : [];
  const selectedSet = new Set(selectedApps);
  const files = Array.isArray(snapshot?.files) ? snapshot.files : [];
  const appGroups = [];
  const writes = [];
  const errors = [];

  for (const app of DESKTOP_CONFIG_APPS) {
    if (!selectedSet.has(app.id)) {
      continue;
    }

    try {
      const appFiles = buildAppFilePreview(app.id, app.label, draft, files);
      if (appFiles.length > 0) {
        appGroups.push({
          appId: app.id,
          appName: app.label,
          files: appFiles,
        });
        for (const file of appFiles) {
          writes.push({
            appId: file.appId,
            fileId: file.fileId,
            content: file.after,
          });
        }
      }
    } catch (error) {
      errors.push(`${app.label}: ${error.message || '生成配置失败'}`);
    }
  }

  return {
    appGroups,
    writes,
    errors,
  };
}

export function detectProviderKeyFromSnapshotFile(appId, draft, fileContent) {
  if (!fileContent || appId === 'claude') return '';

  try {
    if (appId === 'codex') {
      return resolveProviderKeyForApp(appId, {
        ...draft,
        forceCustomProviderKey: false,
      }, fileContent);
    }

    if (appId === 'opencode') {
      return resolveProviderKeyForApp(appId, {
        ...draft,
        forceCustomProviderKey: false,
      }, parseStrictJsonObject(fileContent, 'OpenCode opencode.json', {
        $schema: 'https://opencode.ai/config.json',
      }));
    }

    if (appId === 'openclaw') {
      return resolveProviderKeyForApp(appId, {
        ...draft,
        forceCustomProviderKey: false,
      }, parseStrictJsonObject(fileContent, 'OpenClaw config.json', structuredClone(OPENCLAW_DEFAULT_CONFIG)));
    }
  } catch {
    return '';
  }

  return '';
}

export function inferProviderKeyFromSnapshot(snapshot, draft, selectedApps = []) {
  const files = Array.isArray(snapshot?.files) ? snapshot.files : [];
  const preferredApps = Array.isArray(selectedApps) && selectedApps.length
    ? selectedApps
    : ['codex', 'opencode', 'openclaw'];
  const uniqueKeys = [];

  preferredApps.forEach(appId => {
    if (!['codex', 'opencode', 'openclaw'].includes(appId)) return;
    const fileId = appId === 'codex' ? 'config' : 'config';
    const snapshotFile = findSnapshotFile(files, appId, fileId);
    const providerKey = detectProviderKeyFromSnapshotFile(appId, draft, snapshotFile?.content || '');
    if (providerKey && !uniqueKeys.includes(providerKey)) {
      uniqueKeys.push(providerKey);
    }
  });

  return {
    providerKey: uniqueKeys[0] || '',
    providerKeys: uniqueKeys,
  };
}

function buildAppFilePreview(appId, appName, draft, snapshotFiles) {
  switch (appId) {
    case 'claude':
      return [buildClaudePreview(appName, draft, findSnapshotFile(snapshotFiles, 'claude', 'settings'))];
    case 'codex':
      return [
        buildCodexAuthPreview(appName, draft, findSnapshotFile(snapshotFiles, 'codex', 'auth')),
        buildCodexConfigPreview(appName, draft, findSnapshotFile(snapshotFiles, 'codex', 'config')),
      ];
    case 'opencode':
      return [buildOpenCodePreview(appName, draft, findSnapshotFile(snapshotFiles, 'opencode', 'config'))];
    case 'openclaw':
      return [buildOpenClawPreview(appName, draft, findSnapshotFile(snapshotFiles, 'openclaw', 'config'))];
    default:
      throw new Error(`Unsupported app: ${appId}`);
  }
}

function buildClaudePreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('claude', appName, draft, advancedProxySnapshot);
  const baseUrl = useAdvancedProxy
    ? getAdvancedProxyAppBaseUrl('claude', advancedProxySnapshot)
    : requireField(draft.claudeBaseUrl, `${appName} Base URL`);
  const apiKey = useAdvancedProxy
    ? PROXY_MANAGED_TOKEN
    : requireField(draft.apiKey, `${appName} API Key`);
  const model = requireField(draft.model, `${appName} 模型`);
  const keyField = draft.claudeApiKeyField === 'ANTHROPIC_API_KEY'
    ? 'ANTHROPIC_API_KEY'
    : 'ANTHROPIC_AUTH_TOKEN';

  const current = parseStrictJsonObject(file.content, 'Claude settings.json');
  const next = structuredClone(current);
  if (!isPlainObject(next.env)) {
    next.env = {};
  }

  next.env.ANTHROPIC_BASE_URL = baseUrl;
  next.env[keyField] = apiKey;
  next.env.ANTHROPIC_MODEL = model;
  next.env.ANTHROPIC_DEFAULT_HAIKU_MODEL = model;
  next.env.ANTHROPIC_DEFAULT_SONNET_MODEL = model;
  next.env.ANTHROPIC_DEFAULT_OPUS_MODEL = model;

  if (keyField === 'ANTHROPIC_AUTH_TOKEN') {
    delete next.env.ANTHROPIC_API_KEY;
  } else {
    delete next.env.ANTHROPIC_AUTH_TOKEN;
  }

  return buildPreviewFile(file, JSON.stringify(next, null, 2));
}

function buildCodexAuthPreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('codex', appName, draft, advancedProxySnapshot);
  const apiKey = useAdvancedProxy
    ? PROXY_MANAGED_TOKEN
    : requireField(draft.apiKey, `${appName} API Key`);
  const current = parseStrictJsonObject(file.content, 'Codex auth.json');
  const next = structuredClone(current);
  next.OPENAI_API_KEY = apiKey;
  return buildPreviewFile(file, JSON.stringify(next, null, 2));
}

function buildCodexConfigPreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('codex', appName, draft, advancedProxySnapshot);
  const providerKey = resolveProviderKeyForApp('codex', draft, file.content);
  const providerName = useAdvancedProxy
    ? ADVANCED_PROXY_PROVIDER_NAME
    : requireField(draft.providerName, `${appName} Provider Name`);
  const baseUrl = useAdvancedProxy
    ? getAdvancedProxyAppBaseUrl('codex', advancedProxySnapshot)
    : requireField(draft.codexBaseUrl, `${appName} Base URL`);
  const model = requireField(draft.model, `${appName} 模型`);

  const next = upsertCodexConfigToml(file.content, {
    providerKey,
    providerName,
    baseUrl,
    model,
  });

  return buildPreviewFile(file, next);
}

function buildOpenCodePreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('opencode', appName, draft, advancedProxySnapshot);
  const providerName = useAdvancedProxy
    ? ADVANCED_PROXY_PROVIDER_NAME
    : requireField(draft.providerName, `${appName} Provider Name`);
  const baseUrl = useAdvancedProxy
    ? getAdvancedProxyAppBaseUrl('opencode', advancedProxySnapshot)
    : requireField(draft.opencodeBaseUrl, `${appName} Base URL`);
  const apiKey = useAdvancedProxy
    ? PROXY_MANAGED_TOKEN
    : requireField(draft.apiKey, `${appName} API Key`);
  const model = requireField(draft.model, `${appName} 模型`);

  const current = parseStrictJsonObject(file.content, 'OpenCode opencode.json', {
    $schema: 'https://opencode.ai/config.json',
  });
  const providerKey = resolveProviderKeyForApp('opencode', draft, current);
  const next = structuredClone(current);

  if (!isPlainObject(next.provider)) {
    next.provider = {};
  }

  next.provider = removeMatchingOpenCodeProviders(next.provider, {
    providerKey,
    providerName,
    baseUrl,
  });

  next.provider[providerKey] = {
    npm: useAdvancedProxy ? '@ai-sdk/openai-compatible' : (draft.opencodeNpm || '@ai-sdk/openai-compatible'),
    name: providerName,
    options: {
      baseURL: baseUrl,
      apiKey,
    },
    models: {
      [model]: {
        name: model,
      },
    },
  };

  return buildPreviewFile(file, JSON.stringify(next, null, 2));
}

function buildOpenClawPreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('openclaw', appName, draft, advancedProxySnapshot);
  const providerName = useAdvancedProxy
    ? ADVANCED_PROXY_PROVIDER_NAME
    : requireField(draft.providerName, `${appName} Provider Name`);
  const baseUrl = useAdvancedProxy
    ? getAdvancedProxyAppBaseUrl('openclaw', advancedProxySnapshot)
    : requireField(draft.openclawBaseUrl, `${appName} Base URL`);
  const apiKey = useAdvancedProxy
    ? PROXY_MANAGED_TOKEN
    : requireField(draft.apiKey, `${appName} API Key`);
  const model = requireField(draft.model, `${appName} 模型`);

  const current = parseLooseJsonObject(file.content, 'OpenClaw openclaw.json', OPENCLAW_DEFAULT_CONFIG);
  const providerKey = resolveProviderKeyForApp('openclaw', draft, current);
  const next = structuredClone(current);

  if (!isPlainObject(next.models)) {
    next.models = {};
  }
  if (!isPlainObject(next.models.providers)) {
    next.models.providers = {};
  }
  if (!next.models.mode) {
    next.models.mode = 'merge';
  }

  const removedOpenClawProviderKeys = [];
  next.models.providers = removeMatchingOpenClawProviders(next.models.providers, {
    providerKey,
    baseUrl,
  }, removedOpenClawProviderKeys);

  next.models.providers[providerKey] = {
    baseUrl,
    apiKey,
    api: useAdvancedProxy ? 'openai-completions' : (draft.openclawApi || 'openai-completions'),
    models: [
      {
        id: model,
        name: model,
      },
    ],
  };

  if (!isPlainObject(next.agents)) {
    next.agents = {};
  }
  if (!isPlainObject(next.agents.defaults)) {
    next.agents.defaults = {};
  }
  if (!isPlainObject(next.agents.defaults.models)) {
    next.agents.defaults.models = {};
  }

  if (removedOpenClawProviderKeys.length > 0) {
    Object.keys(next.agents.defaults.models).forEach(modelKey => {
      const normalizedModelKey = String(modelKey || '').trim();
      if (!normalizedModelKey.includes('/')) return;
      const modelProviderKey = sanitizeProviderKey(normalizedModelKey.split('/')[0]);
      if (removedOpenClawProviderKeys.includes(modelProviderKey) && modelProviderKey !== providerKey) {
        delete next.agents.defaults.models[modelKey];
      }
    });
  }

  const fullModelName = `${providerKey}/${model}`;
  next.agents.defaults.model = {
    primary: fullModelName,
  };
  next.agents.defaults.models[fullModelName] = {
    alias: providerName,
  };

  return buildPreviewFile(file, JSON.stringify(next, null, 2));
}

function shouldUseAdvancedProxy(appId, appName, draft, advancedProxySnapshot) {
  const flagKey = `${appId}UseAdvancedProxy`;
  if (draft?.[flagKey] !== true) {
    return false;
  }
  if (!isAdvancedProxyAppReady(appId, advancedProxySnapshot)) {
    throw new Error(`${appName} 高级代理尚未就绪，请先在“高级代理功能”中启用对应接管并准备兼容上游`);
  }
  return true;
}

function buildPreviewFile(file, after) {
  const before = String(file?.content || '');
  return {
    appId: file.appId,
    appName: file.appName,
    fileId: file.fileId,
    label: file.label,
    path: file.path,
    exists: Boolean(file.exists),
    before,
    after: ensureTrailingNewline(after),
  };
}

function parseStrictJsonObject(text, label, fallback = {}) {
  if (!String(text || '').trim()) {
    return structuredClone(fallback);
  }

  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch (error) {
    throw new Error(`${label} 不是合法 JSON，无法自动合并`);
  }

  if (!isPlainObject(parsed)) {
    throw new Error(`${label} 根节点必须是对象`);
  }

  return parsed;
}

function parseLooseJsonObject(text, label, fallback = {}) {
  if (!String(text || '').trim()) {
    return structuredClone(fallback);
  }

  const normalized = normalizeJson5LikeToJson(text);
  let parsed;
  try {
    parsed = JSON.parse(normalized);
  } catch (error) {
    throw new Error(`${label} 不是可解析的 JSON/JSON5，无法自动合并`);
  }

  if (!isPlainObject(parsed)) {
    throw new Error(`${label} 根节点必须是对象`);
  }

  return parsed;
}

function normalizeJson5LikeToJson(input) {
  const withoutComments = stripJsonComments(input);
  const withDoubleQuotes = convertSingleQuotedStrings(withoutComments);
  const quotedKeys = withDoubleQuotes.replace(
    /([{,]\s*)([A-Za-z_$][\w$-]*)(\s*:)/g,
    '$1"$2"$3'
  );
  return quotedKeys.replace(/,(\s*[}\]])/g, '$1');
}

function stripJsonComments(input) {
  let result = '';
  let inSingle = false;
  let inDouble = false;
  let escaping = false;

  for (let index = 0; index < input.length; index += 1) {
    const current = input[index];
    const next = input[index + 1];

    if (!inSingle && !inDouble && current === '/' && next === '/') {
      while (index < input.length && input[index] !== '\n') {
        index += 1;
      }
      if (index < input.length) {
        result += '\n';
      }
      continue;
    }

    if (!inSingle && !inDouble && current === '/' && next === '*') {
      index += 2;
      while (index < input.length && !(input[index] === '*' && input[index + 1] === '/')) {
        index += 1;
      }
      index += 1;
      continue;
    }

    result += current;

    if (escaping) {
      escaping = false;
      continue;
    }

    if ((inSingle || inDouble) && current === '\\') {
      escaping = true;
      continue;
    }

    if (!inDouble && current === '\'') {
      inSingle = !inSingle;
      continue;
    }

    if (!inSingle && current === '"') {
      inDouble = !inDouble;
    }
  }

  return result;
}

function convertSingleQuotedStrings(input) {
  let result = '';
  let inDouble = false;
  let escaping = false;

  for (let index = 0; index < input.length; index += 1) {
    const current = input[index];

    if (inDouble) {
      result += current;
      if (escaping) {
        escaping = false;
      } else if (current === '\\') {
        escaping = true;
      } else if (current === '"') {
        inDouble = false;
      }
      continue;
    }

    if (current === '"') {
      inDouble = true;
      result += current;
      continue;
    }

    if (current !== '\'') {
      result += current;
      continue;
    }

    let buffer = '';
    let innerEscaping = false;
    let closed = false;
    for (index += 1; index < input.length; index += 1) {
      const inner = input[index];
      if (innerEscaping) {
        buffer += inner;
        innerEscaping = false;
        continue;
      }
      if (inner === '\\') {
        innerEscaping = true;
        buffer += inner;
        continue;
      }
      if (inner === '\'') {
        closed = true;
        break;
      }
      buffer += inner;
    }

    if (!closed) {
      throw new Error('Single-quoted string is not closed');
    }

    const decoded = buffer
      .replace(/\\'/g, '\'')
      .replace(/\\"/g, '"');
    result += JSON.stringify(decoded);
  }

  return result;
}

function upsertCodexConfigToml(currentText, options) {
  let text = String(currentText || '').trim();
  if (!text) {
    text = '';
  }

  text = upsertTomlRootField(text, 'model_provider', quoteTomlString(options.providerKey));
  text = upsertTomlRootField(text, 'model', quoteTomlString(options.model));
  text = upsertTomlRootField(text, 'model_reasoning_effort', quoteTomlString('high'));
  text = upsertTomlRootField(text, 'disable_response_storage', 'true');

  const providerSection = [
    `[model_providers.${options.providerKey}]`,
    `name = ${quoteTomlString(options.providerName)}`,
    `base_url = ${quoteTomlString(options.baseUrl)}`,
    'wire_api = "responses"',
    'requires_openai_auth = true',
  ].join('\n');

  text = removeMatchingCodexProviderSections(text, options);
  text = `${text.trim()}\n\n${providerSection}\n`;

  return ensureTrailingNewline(text.replace(/\n{3,}/g, '\n\n').trim());
}

function removeMatchingCodexProviderSections(text, options) {
  const source = String(text || '').replace(/\r\n/g, '\n');
  const lines = source.split('\n');
  const keptSections = [];
  let currentHeader = '';
  let currentLines = [];

  const flush = () => {
    if (!currentLines.length) return;
    const sectionText = currentLines.join('\n').trim();
    if (!sectionText) {
      currentHeader = '';
      currentLines = [];
      return;
    }
    if (!shouldDropCodexProviderSection(currentHeader, sectionText, options)) {
      keptSections.push(sectionText);
    }
    currentHeader = '';
    currentLines = [];
  };

  lines.forEach(line => {
    if (/^\[[^\]]+\]\s*$/.test(line.trim())) {
      flush();
      currentHeader = line.trim();
      currentLines = [line];
      return;
    }

    if (currentLines.length === 0) {
      currentLines = [line];
      return;
    }
    currentLines.push(line);
  });

  flush();
  return keptSections.join('\n\n').trim();
}

function shouldDropCodexProviderSection(header, sectionText, options) {
  const normalizedHeader = String(header || '').trim();
  const providerHeader = `[model_providers.${options.providerKey}]`;
  if (normalizedHeader === providerHeader) {
    return true;
  }
  if (!normalizedHeader.startsWith('[model_providers.')) {
    return false;
  }

  const nameMatch = sectionText.match(/^\s*name\s*=\s*["']([^"'\n]+)["']/m);
  if (nameMatch?.[1] && String(nameMatch[1]).trim() === String(options.providerName || '').trim()) {
    return true;
  }

  const baseUrlMatch = sectionText.match(/^\s*base_url\s*=\s*["']([^"'\n]+)["']/m);
  if (baseUrlMatch?.[1] && normalizeComparableUrl(baseUrlMatch[1]) === normalizeComparableUrl(options.baseUrl)) {
    return true;
  }

  return false;
}

function removeMatchingOpenCodeProviders(providers, options) {
  const source = isPlainObject(providers) ? providers : {};
  const next = {};
  const normalizedProviderKey = sanitizeProviderKey(options.providerKey);
  const normalizedProviderName = String(options.providerName || '').trim();
  const normalizedBaseUrl = normalizeComparableUrl(options.baseUrl);

  Object.entries(source).forEach(([key, value]) => {
    const normalizedKey = sanitizeProviderKey(key);
    const providerName = String(value?.name || '').trim();
    const providerBaseUrl = normalizeComparableUrl(value?.options?.baseURL);
    const shouldDrop =
      normalizedKey === normalizedProviderKey ||
      (normalizedProviderName && providerName === normalizedProviderName) ||
      (normalizedBaseUrl && providerBaseUrl === normalizedBaseUrl);
    if (!shouldDrop) {
      next[key] = value;
    }
  });

  return next;
}

function removeMatchingOpenClawProviders(providers, options, removedKeys = []) {
  const source = isPlainObject(providers) ? providers : {};
  const next = {};
  const normalizedProviderKey = sanitizeProviderKey(options.providerKey);
  const normalizedBaseUrl = normalizeComparableUrl(options.baseUrl);

  Object.entries(source).forEach(([key, value]) => {
    const normalizedKey = sanitizeProviderKey(key);
    const providerBaseUrl = normalizeComparableUrl(value?.baseUrl);
    const shouldDrop =
      normalizedKey === normalizedProviderKey ||
      (normalizedBaseUrl && providerBaseUrl === normalizedBaseUrl);
    if (shouldDrop) {
      if (!removedKeys.includes(normalizedKey)) {
        removedKeys.push(normalizedKey);
      }
      return;
    }
    next[key] = value;
  });

  return next;
}

function upsertTomlRootField(text, field, valueLiteral) {
  const pattern = new RegExp(`^${escapeRegExp(field)}\\s*=.*$`, 'm');
  if (pattern.test(text)) {
    return text.replace(pattern, `${field} = ${valueLiteral}`);
  }

  const trimmed = String(text || '').trim();
  return trimmed ? `${field} = ${valueLiteral}\n${trimmed}` : `${field} = ${valueLiteral}`;
}

function findSnapshotFile(files, appId, fileId) {
  const file = files.find(item => item.appId === appId && item.fileId === fileId);
  if (!file) {
    throw new Error(`未找到 ${appId}/${fileId} 的本地配置文件快照`);
  }
  return file;
}

function requireField(value, label) {
  const normalized = String(value || '').trim();
  if (!normalized) {
    throw new Error(`${label} 不能为空`);
  }
  return normalized;
}

function ensureTrailingNewline(text) {
  const normalized = String(text || '');
  return normalized.endsWith('\n') ? normalized : `${normalized}\n`;
}

function escapeRegExp(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function quoteTomlString(value) {
  return JSON.stringify(String(value || ''));
}

function sanitizeProviderKey(value) {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '');

  return normalized || 'custom_provider';
}

function resolveProviderKeyForApp(appId, draft, source) {
  if (draft?.forceCustomProviderKey !== false) {
    return 'custom';
  }

  const fallback = sanitizeProviderKey(draft?.providerKey || draft?.providerName || 'custom');
  if (appId === 'codex') {
    return extractCodexProviderKey(source) || fallback;
  }
  if (appId === 'opencode') {
    return extractOpenCodeProviderKey(source, draft) || fallback;
  }
  if (appId === 'openclaw') {
    return extractOpenClawProviderKey(source, draft) || fallback;
  }
  return fallback;
}

function extractCodexProviderKey(text) {
  const match = String(text || '').match(/^\s*model_provider\s*=\s*["']([^"'\n]+)["']/m);
  return match?.[1] ? sanitizeProviderKey(match[1]) : '';
}

function extractOpenCodeProviderKey(config, draft) {
  const providers = isPlainObject(config?.provider) ? config.provider : {};
  const keys = Object.keys(providers);
  if (!keys.length) return '';

  const preferred = sanitizeProviderKey(draft?.providerKey || draft?.providerName || '');
  if (preferred && providers[preferred]) return preferred;

  const providerName = String(draft?.providerName || '').trim();
  if (providerName) {
    const nameMatch = keys.find(key => String(providers[key]?.name || '').trim() === providerName);
    if (nameMatch) return sanitizeProviderKey(nameMatch);
  }

  const endpoint = normalizeComparableUrl(draft?.opencodeBaseUrl || draft?.endpoint);
  if (endpoint) {
    const endpointMatch = keys.find(
      key => normalizeComparableUrl(providers[key]?.options?.baseURL) === endpoint
    );
    if (endpointMatch) return sanitizeProviderKey(endpointMatch);
  }

  return sanitizeProviderKey(keys[0]);
}

function extractOpenClawProviderKey(config, draft) {
  const providers = isPlainObject(config?.models?.providers) ? config.models.providers : {};
  const keys = Object.keys(providers);
  if (!keys.length) return '';

  const primary = String(config?.agents?.defaults?.model?.primary || '').trim();
  if (primary.includes('/')) {
    const activeKey = sanitizeProviderKey(primary.split('/')[0]);
    if (providers[activeKey]) return activeKey;
  }

  const preferred = sanitizeProviderKey(draft?.providerKey || draft?.providerName || '');
  if (preferred && providers[preferred]) return preferred;

  const endpoint = normalizeComparableUrl(draft?.openclawBaseUrl || draft?.endpoint);
  if (endpoint) {
    const endpointMatch = keys.find(key => normalizeComparableUrl(providers[key]?.baseUrl) === endpoint);
    if (endpointMatch) return sanitizeProviderKey(endpointMatch);
  }

  return sanitizeProviderKey(keys[0]);
}

function normalizeComparableUrl(value) {
  return String(value || '').trim().replace(/\/+$/, '').toLowerCase();
}

function pickFallbackModel(modelsList, modelsText) {
  const candidates = [];
  if (Array.isArray(modelsList)) {
    candidates.push(...modelsList);
  }
  if (typeof modelsText === 'string') {
    candidates.push(...modelsText.split(/[\n,，\s]+/));
  }

  return (
    candidates
      .map(item => String(item || '').trim())
      .find(Boolean) || ''
  );
}

function isPlainObject(value) {
  return Object.prototype.toString.call(value) === '[object Object]';
}
