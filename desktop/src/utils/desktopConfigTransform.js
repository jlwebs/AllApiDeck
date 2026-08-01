import {
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyLocalSnapshot,
  isAdvancedProxyAppReady,
} from './advancedProxyBridge.js';
import { resolveOpenAIExportBaseUrl } from './exportEndpoint.js';

const OPENCLAW_DEFAULT_CONFIG = {
  models: {
    mode: 'merge',
    providers: {},
  },
};

const PROXY_MANAGED_TOKEN = 'PROXY_MANAGED';
const ADVANCED_PROXY_PROVIDER_NAME = 'AllApiDeck Advanced Proxy';
export const ADVANCED_PROXY_MODEL_NAME = 'AllApiDeck-mix';

export const DESKTOP_CONFIG_APPS = [
  { id: 'claude', label: 'Claude' },
  { id: 'codex', label: 'Codex' },
  { id: 'grokbuild', label: 'Grok Build' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'openclaw', label: 'OpenClaw' },
  { id: 'hermes', label: 'Hermes' },
];

export function createDesktopConfigDraft(record) {
  const defaultModel =
    String(record?.selectedModel || '').trim() ||
    String(record?.quickTestModel || '').trim() ||
    pickFallbackModel(record?.modelsList, record?.modelsText);
  const providerName = String(record?.siteName || 'Custom Provider').trim() || 'Custom Provider';
  const endpoint = String(record?.siteUrl || '').trim();
  const smartOpenAIBaseUrl = resolveOpenAIExportBaseUrl(record, endpoint) || endpoint;
  const apiKey = String(record?.apiKey || '').trim();
  return {
    selectedApps: [],
    providerName,
    providerKey: 'custom',
    forceCustomProviderKey: true,
    endpoint,
    apiKey,
    model: defaultModel || 'gpt-4o-mini',
    effort: 'high',
    claudeBaseUrl: endpoint,
    claudeApiKeyField: 'ANTHROPIC_AUTH_TOKEN',
    claudeUseAdvancedProxy: false,
    codexBaseUrl: smartOpenAIBaseUrl,
    codexUseAdvancedProxy: false,
    grokbuildBaseUrl: smartOpenAIBaseUrl,
    grokbuildUseAdvancedProxy: false,
    grokbuildApiBackend: 'responses',
    opencodeBaseUrl: smartOpenAIBaseUrl,
    opencodeUseAdvancedProxy: false,
    opencodeNpm: '@ai-sdk/openai-compatible',
    openclawBaseUrl: smartOpenAIBaseUrl,
    openclawUseAdvancedProxy: false,
    openclawApi: 'openai-completions',
    hermesBaseUrl: smartOpenAIBaseUrl,
    hermesUseAdvancedProxy: false,
    hermesApiMode: 'chat_completions',
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

    if (appId === 'hermes') {
      return resolveProviderKeyForApp(appId, {
        ...draft,
        forceCustomProviderKey: false,
      }, fileContent);
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
    : ['codex', 'opencode', 'openclaw', 'hermes'];
  const uniqueKeys = [];

  preferredApps.forEach(appId => {
    if (!['codex', 'opencode', 'openclaw', 'hermes'].includes(appId)) return;
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
    case 'grokbuild':
      return [buildGrokBuildPreview(appName, draft, findSnapshotFile(snapshotFiles, 'grokbuild', 'config'))];
    case 'opencode':
      return [buildOpenCodePreview(appName, draft, findSnapshotFile(snapshotFiles, 'opencode', 'config'))];
    case 'openclaw':
      return [buildOpenClawPreview(appName, draft, findSnapshotFile(snapshotFiles, 'openclaw', 'config'))];
    case 'hermes':
      return [buildHermesPreview(appName, draft, findSnapshotFile(snapshotFiles, 'hermes', 'config'))];
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
  const model = useAdvancedProxy
    ? ADVANCED_PROXY_MODEL_NAME
    : requireField(draft.model, `${appName} 模型`);
  const effort = requireField(draft.effort, `${appName} Effort`);
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
  next.effortLevel = effort;

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
  const model = useAdvancedProxy
    ? ADVANCED_PROXY_MODEL_NAME
    : requireField(draft.model, `${appName} 模型`);
  const effort = requireField(draft.effort, `${appName} Effort`);

  const next = upsertCodexConfigToml(file.content, {
    providerKey,
    providerName,
    baseUrl,
    model,
    effort,
  });

  return buildPreviewFile(file, next);
}

function buildGrokBuildPreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('grokbuild', appName, draft, advancedProxySnapshot);
  const model = useAdvancedProxy
    ? ADVANCED_PROXY_MODEL_NAME
    : requireField(draft.model, `${appName} model`);
  const effort = requireField(draft.effort, `${appName} Effort`);
  const next = upsertGrokBuildConfigToml(file.content, {
    model,
    effort,
    baseUrl: useAdvancedProxy
      ? getAdvancedProxyAppBaseUrl('grokbuild', advancedProxySnapshot)
      : requireField(draft.grokbuildBaseUrl, `${appName} Base URL`),
    name: useAdvancedProxy
      ? ADVANCED_PROXY_PROVIDER_NAME
      : requireField(draft.providerName, `${appName} Provider Name`),
    apiKey: useAdvancedProxy
      ? PROXY_MANAGED_TOKEN
      : requireField(draft.apiKey, `${appName} API Key`),
    apiBackend: draft.grokbuildApiBackend === 'chat_completions' ? 'chat_completions' : 'responses',
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
  const model = useAdvancedProxy
    ? ADVANCED_PROXY_MODEL_NAME
    : requireField(draft.model, `${appName} 模型`);
  const effort = requireField(draft.effort, `${appName} Effort`);

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
        options: {
          reasoningEffort: effort,
        },
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
  const model = useAdvancedProxy
    ? ADVANCED_PROXY_MODEL_NAME
    : requireField(draft.model, `${appName} 模型`);
  const effort = requireField(draft.effort, `${appName} Effort`);
  const api = useAdvancedProxy ? 'openai-completions' : (draft.openclawApi || 'openai-completions');

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
    api,
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
    params: buildOpenClawEffortParams(api, effort),
  };

  return buildPreviewFile(file, JSON.stringify(next, null, 2));
}

function buildHermesPreview(appName, draft, file) {
  const advancedProxySnapshot = getAdvancedProxyLocalSnapshot();
  const useAdvancedProxy = shouldUseAdvancedProxy('hermes', appName, draft, advancedProxySnapshot);
  const providerKey = resolveProviderKeyForApp('hermes', draft, file.content);
  const baseUrl = useAdvancedProxy
    ? getAdvancedProxyAppBaseUrl('hermes', advancedProxySnapshot)
    : requireField(draft.hermesBaseUrl, `${appName} Base URL`);
  const apiKey = useAdvancedProxy
    ? PROXY_MANAGED_TOKEN
    : requireField(draft.apiKey, `${appName} API Key`);
  const model = requireField(
    useAdvancedProxy ? ADVANCED_PROXY_MODEL_NAME : draft.model,
    `${appName} 模型`,
  );
  const apiMode = useAdvancedProxy
    ? 'chat_completions'
    : normalizeHermesApiMode(draft.hermesApiMode);

  const next = patchHermesConfigYaml(file.content, {
    providerKey,
    baseUrl,
    apiKey,
    apiMode,
    model,
  });

  return buildPreviewFile(file, next);
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
  text = upsertTomlRootField(text, 'model_reasoning_effort', quoteTomlString(options.effort));
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

const HERMES_PROVIDER_KNOWN_FIELDS = new Set([
  'name',
  'base_url',
  'baseUrl',
  'api_key',
  'apiKey',
  'api_mode',
  'apiMode',
  'model',
  'models',
]);

function normalizeHermesApiMode(value) {
  const normalized = String(value || '').trim().toLowerCase();
  return ['chat_completions', 'anthropic_messages', 'codex_responses', 'bedrock_converse'].includes(normalized)
    ? normalized
    : 'chat_completions';
}

function quoteHermesYamlString(value) {
  return JSON.stringify(String(value ?? ''));
}

function isHermesTopLevelKeyLine(line) {
  return /^[^\s#][^:]*:\s*(?:#.*)?$/.test(String(line || ''));
}

function findHermesTopLevelSectionRange(lines, key) {
  const headerPattern = new RegExp(`^${escapeRegExp(key)}\\s*:`);
  const start = lines.findIndex(line => headerPattern.test(String(line || '')) && isHermesTopLevelKeyLine(line));
  if (start < 0) return null;

  let end = lines.length;
  for (let index = start + 1; index < lines.length; index += 1) {
    if (isHermesTopLevelKeyLine(lines[index])) {
      end = index;
      break;
    }
  }
  return { start, end };
}

function replaceHermesTopLevelSection(rawText, key, buildSection) {
  const lines = String(rawText || '').replace(/\r\n/g, '\n').split('\n');
  const range = findHermesTopLevelSectionRange(lines, key);
  const replacement = buildSection(range ? lines.slice(range.start, range.end) : null);
  if (range) {
    return [...lines.slice(0, range.start), ...replacement, ...lines.slice(range.end)].join('\n');
  }

  const trimmed = lines.join('\n').replace(/\n+$/g, '');
  return trimmed ? `${trimmed}\n\n${replacement.join('\n')}` : replacement.join('\n');
}

function parseHermesYamlScalar(value) {
  const raw = String(value || '').trim().replace(/\s+#.*$/, '');
  if (!raw) return '';
  if ((raw.startsWith('"') && raw.endsWith('"')) || (raw.startsWith("'") && raw.endsWith("'"))) {
    return raw.slice(1, -1).replace(/\\"/g, '"').replace(/\\'/g, "'");
  }
  return raw;
}

function getHermesProviderBlockName(block) {
  const inlineName = String(block[0] || '').match(/^\s{2}-\s+name\s*:\s*(.*)$/);
  if (inlineName) return parseHermesYamlScalar(inlineName[1]);
  const nameLine = block.find(line => /^\s{4}name\s*:\s*/.test(line));
  return nameLine ? parseHermesYamlScalar(nameLine.replace(/^\s{4}name\s*:\s*/, '')) : '';
}

function getHermesProviderBlocks(sectionLines) {
  const blocks = [];
  let current = null;
  sectionLines.forEach((line, index) => {
    if (index === 0) return;
    if (/^\s{2}-\s+/.test(line)) {
      if (current) blocks.push(current);
      current = [line];
      return;
    }
    if (current) current.push(line);
  });
  if (current) blocks.push(current);
  return blocks;
}

function extractHermesUnknownProviderChunks(block) {
  const chunks = [];
  for (let index = 1; index < block.length; index += 1) {
    const fieldMatch = block[index].match(/^\s{4}([A-Za-z0-9_.-]+)\s*:/);
    if (!fieldMatch || HERMES_PROVIDER_KNOWN_FIELDS.has(fieldMatch[1])) continue;
    const start = index;
    index += 1;
    while (index < block.length && !/^\s{4}[A-Za-z0-9_.-]+\s*:/.test(block[index])) {
      index += 1;
    }
    chunks.push(block.slice(start, index));
    index -= 1;
  }
  return chunks;
}

function buildHermesProviderBlock(options, existingBlock = null) {
  const providerKey = requireField(options.providerKey, 'Hermes Provider Key');
  const model = requireField(options.model, 'Hermes 模型');
  const lines = [
    `  - name: ${quoteHermesYamlString(providerKey)}`,
    `    base_url: ${quoteHermesYamlString(options.baseUrl)}`,
    `    api_key: ${quoteHermesYamlString(options.apiKey)}`,
    `    api_mode: ${quoteHermesYamlString(normalizeHermesApiMode(options.apiMode))}`,
    `    model: ${quoteHermesYamlString(model)}`,
    '    models:',
    `      ${quoteHermesYamlString(model)}: {}`,
  ];
  if (existingBlock) {
    extractHermesUnknownProviderChunks(existingBlock).forEach(chunk => lines.push(...chunk));
  }
  return lines;
}

function patchHermesCustomProvidersSection(sectionLines, options) {
  const header = sectionLines?.[0] || 'custom_providers:';
  const existingBlocks = sectionLines ? getHermesProviderBlocks(sectionLines) : [];
  const matchingBlock = existingBlocks.find(block => getHermesProviderBlockName(block) === options.providerKey) || null;
  const nextBlocks = existingBlocks.map(block => (
    block === matchingBlock ? buildHermesProviderBlock(options, block) : block
  ));
  if (!matchingBlock) {
    nextBlocks.push(buildHermesProviderBlock(options));
  }
  return [header, ...nextBlocks.flatMap((block, index) => index === 0 ? block : ['', ...block])];
}

function extractHermesUnknownModelChunks(sectionLines) {
  const chunks = [];
  for (let index = 1; index < sectionLines.length; index += 1) {
    const fieldMatch = sectionLines[index].match(/^\s{2}([A-Za-z0-9_.-]+)\s*:/);
    if (!fieldMatch || fieldMatch[1] === 'default' || fieldMatch[1] === 'provider') continue;
    const start = index;
    index += 1;
    while (index < sectionLines.length && !/^\s{2}[A-Za-z0-9_.-]+\s*:/.test(sectionLines[index])) {
      index += 1;
    }
    chunks.push(sectionLines.slice(start, index));
    index -= 1;
  }
  return chunks;
}

function patchHermesModelSection(sectionLines, options) {
  const lines = [
    'model:',
    `  default: ${quoteHermesYamlString(options.model)}`,
    `  provider: ${quoteHermesYamlString(options.providerKey)}`,
  ];
  if (sectionLines) {
    extractHermesUnknownModelChunks(sectionLines).forEach(chunk => lines.push(...chunk));
  }
  return lines;
}

function patchHermesConfigYaml(rawText, options) {
  let next = replaceHermesTopLevelSection(rawText, 'custom_providers', section =>
    patchHermesCustomProvidersSection(section, options));
  next = replaceHermesTopLevelSection(next, 'model', section => patchHermesModelSection(section, options));
  return ensureTrailingNewline(next.replace(/\n{3,}/g, '\n\n').trim());
}

function upsertGrokBuildConfigToml(currentText, options) {
  const model = String(options.model || '').trim();
  const retained = removeGrokBuildSections(currentText, model, extractGrokBuildDefaultModel(currentText));
  const configSections = [
    '[models]',
    `default = ${quoteTomlString(model)}`,
    [
      `[model.${quoteTomlString(model)}]`,
      `model = ${quoteTomlString(model)}`,
      `base_url = ${quoteTomlString(options.baseUrl)}`,
      `name = ${quoteTomlString(options.name)}`,
      `api_key = ${quoteTomlString(options.apiKey)}`,
      `api_backend = ${quoteTomlString(options.apiBackend)}`,
      `reasoning_effort = ${quoteTomlString(options.effort)}`,
      'context_window = 500000',
    ].join('\n'),
  ];

  if (retained) {
    configSections.unshift(retained);
  }
  return ensureTrailingNewline(configSections.join('\n\n').replace(/\n{3,}/g, '\n\n').trim());
}

function removeGrokBuildSections(text, selectedModel, previousDefaultModel = '') {
  const source = String(text || '').replace(/\r\n/g, '\n');
  const keptSections = [];
  let currentHeader = '';
  let currentLines = [];

  const flush = () => {
    if (!currentLines.length) return;
    const sectionText = currentLines.join('\n').trim();
    if (sectionText && !shouldDropGrokBuildSection(currentHeader, selectedModel, previousDefaultModel)) {
      keptSections.push(sectionText);
    }
    currentHeader = '';
    currentLines = [];
  };

  source.split('\n').forEach(line => {
    if (/^\[[^\]]+\]\s*$/.test(line.trim())) {
      flush();
      currentHeader = line.trim();
      currentLines = [line];
      return;
    }
    if (!currentLines.length) {
      currentLines = [line];
      return;
    }
    currentLines.push(line);
  });
  flush();

  return keptSections.join('\n\n').trim();
}

function shouldDropGrokBuildSection(header, selectedModel, previousDefaultModel = '') {
  const normalizedHeader = String(header || '').trim();
  const dropModelHeaders = [
    selectedModel,
    previousDefaultModel,
  ]
    .map(value => String(value || '').trim())
    .filter(Boolean)
    .flatMap(model => [
      `[model.${quoteTomlString(model)}]`,
      `[model.${model}]`,
    ]);

  return normalizedHeader === '[models]' ||
    dropModelHeaders.includes(normalizedHeader);
}

function extractGrokBuildDefaultModel(text) {
  const source = String(text || '').replace(/\r\n/g, '\n');
  const lines = source.split('\n');
  let inModels = false;

  for (const line of lines) {
    const trimmed = line.trim();
    if (/^\[[^\]]+\]\s*$/.test(trimmed)) {
      inModels = trimmed === '[models]';
      continue;
    }
    if (!inModels) continue;

    const match = trimmed.match(/^default\s*=\s*(?:"((?:\\.|[^"\\])*)"|'([^'\n]*)'|([^\s#]+))/);
    if (match) {
      const raw = match[1] ?? match[2] ?? match[3] ?? '';
      return String(raw).trim();
    }
  }

  return '';
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

function buildOpenClawEffortParams(api, effort) {
  if (api === 'anthropic-messages') {
    return { thinking: effort };
  }
  return {
    extra_body: {
      reasoning_effort: effort,
    },
  };
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
  if (appId === 'hermes') {
    return extractHermesProviderKey(source) || fallback;
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

function extractHermesProviderKey(text) {
  const source = String(text || '').replace(/\r\n/g, '\n');
  const modelSection = findHermesTopLevelSectionRange(source.split('\n'), 'model');
  if (modelSection) {
    const lines = source.split('\n').slice(modelSection.start, modelSection.end);
    const providerLine = lines.find(line => /^\s{2}provider\s*:\s*/.test(line));
    const provider = providerLine
      ? parseHermesYamlScalar(providerLine.replace(/^\s{2}provider\s*:\s*/, ''))
      : '';
    if (provider) return sanitizeProviderKey(provider);
  }

  const customSection = findHermesTopLevelSectionRange(source.split('\n'), 'custom_providers');
  if (!customSection) return '';
  const blocks = getHermesProviderBlocks(source.split('\n').slice(customSection.start, customSection.end));
  const firstName = blocks.map(getHermesProviderBlockName).find(Boolean);
  return firstName ? sanitizeProviderKey(firstName) : '';
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
