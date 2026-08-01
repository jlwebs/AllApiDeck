import assert from 'node:assert/strict';
import { createDefaultAdvancedProxyConfig } from '../src/utils/advancedProxyBridge.js';
import {
  ADVANCED_PROXY_MODEL_NAME,
  buildDesktopConfigPreview,
  createDesktopConfigDraft,
} from '../src/utils/desktopConfigTransform.js';
import {
  buildDesktopConfigTakeoverRestorePreview,
  captureDesktopConfigTakeoverBackup,
  clearDesktopConfigTakeoverBackup,
} from '../src/utils/desktopConfigTakeover.js';

const storage = new Map();
globalThis.localStorage = {
  getItem: key => storage.get(key) || null,
  setItem: (key, value) => storage.set(key, String(value)),
};

const advancedProxyConfig = createDefaultAdvancedProxyConfig();
advancedProxyConfig.enabled = true;
for (const appId of ['claude', 'codex', 'grokbuild', 'opencode', 'openclaw']) {
  advancedProxyConfig[appId].enabled = true;
}
advancedProxyConfig.queues.global.providers = [{
  id: 'provider-1',
  name: 'Upstream',
  baseUrl: 'https://upstream.example/v1',
  apiKey: 'upstream-key',
  model: 'upstream-model',
  apiFormat: 'openai_responses',
  enabled: true,
}];
storage.set('batch_api_check_advanced_proxy_config_v1', JSON.stringify(advancedProxyConfig));

const draft = {
  ...createDesktopConfigDraft({
    siteName: 'Upstream',
    siteUrl: 'https://upstream.example/v1',
    apiKey: 'upstream-key',
    selectedModel: 'upstream-model',
  }),
  selectedApps: ['claude', 'codex', 'grokbuild', 'opencode', 'openclaw'],
  claudeUseAdvancedProxy: true,
  codexUseAdvancedProxy: true,
  grokbuildUseAdvancedProxy: true,
  opencodeUseAdvancedProxy: true,
  openclawUseAdvancedProxy: true,
};

const snapshot = {
  files: [
    { appId: 'claude', appName: 'Claude', fileId: 'settings', label: 'settings.json', path: 'claude', exists: true, content: '{"env":{}}' },
    { appId: 'codex', appName: 'Codex', fileId: 'auth', label: 'auth.json', path: 'codex-auth', exists: true, content: '{}' },
    { appId: 'codex', appName: 'Codex', fileId: 'config', label: 'config.toml', path: 'codex-config', exists: true, content: '' },
    { appId: 'grokbuild', appName: 'Grok Build', fileId: 'config', label: 'config.toml', path: 'grokbuild', exists: true, content: '' },
    { appId: 'opencode', appName: 'OpenCode', fileId: 'config', label: 'opencode.json', path: 'opencode', exists: true, content: '{}' },
    { appId: 'openclaw', appName: 'OpenClaw', fileId: 'config', label: 'openclaw.json', path: 'openclaw', exists: true, content: '{}' },
  ],
};

const preview = buildDesktopConfigPreview(draft, snapshot);
assert.deepEqual(preview.errors, []);
assert.equal(preview.writes.length, 6);

const modelWrites = new Map(
  preview.writes
    .filter(file => !(file.appId === 'codex' && file.fileId === 'auth'))
    .map(file => [`${file.appId}/${file.fileId}`, file.content]),
);
assert.match(modelWrites.get('claude/settings'), new RegExp(`ANTHROPIC_MODEL\\": \\"${ADVANCED_PROXY_MODEL_NAME}`));
assert.match(modelWrites.get('codex/config'), new RegExp(`model = \\"${ADVANCED_PROXY_MODEL_NAME}\\"`));
assert.match(modelWrites.get('grokbuild/config'), new RegExp(`default = \\"${ADVANCED_PROXY_MODEL_NAME}\\"`));
assert.equal(JSON.parse(modelWrites.get('opencode/config')).provider.custom.models[ADVANCED_PROXY_MODEL_NAME].name, ADVANCED_PROXY_MODEL_NAME);
assert.equal(JSON.parse(modelWrites.get('openclaw/config')).agents.defaults.model.primary, `custom/${ADVANCED_PROXY_MODEL_NAME}`);

const takeoverSnapshot = {
  files: [
    { appId: 'codex', appName: 'Codex', fileId: 'auth', label: 'auth.json', path: 'codex-auth', exists: true, content: '{"OPENAI_API_KEY":"original"}' },
    { appId: 'codex', appName: 'Codex', fileId: 'config', label: 'config.toml', path: 'codex-config', exists: false, content: '' },
  ],
};
assert.equal(captureDesktopConfigTakeoverBackup('codex', takeoverSnapshot), true);
assert.equal(captureDesktopConfigTakeoverBackup('codex', takeoverSnapshot), false);

const restorePreview = buildDesktopConfigTakeoverRestorePreview('codex', {
  files: takeoverSnapshot.files.map(file => ({ ...file, exists: true, content: 'managed' })),
});
assert.equal(restorePreview.appGroups.length, 1);
assert.equal(restorePreview.writes.length, 2);
assert.equal(restorePreview.writes.find(file => file.fileId === 'config').exists, false);
assert.equal(restorePreview.appGroups[0].files.find(file => file.fileId === 'auth').after, '{"OPENAI_API_KEY":"original"}');

clearDesktopConfigTakeoverBackup('codex');
assert.equal(buildDesktopConfigTakeoverRestorePreview('codex', takeoverSnapshot), null);
