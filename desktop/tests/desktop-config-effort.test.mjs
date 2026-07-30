import assert from 'node:assert/strict';
import { buildDesktopConfigPreview, createDesktopConfigDraft } from '../src/utils/desktopConfigTransform.js';

const storage = new Map();
globalThis.localStorage = {
  getItem: key => storage.get(key) || null,
  setItem: (key, value) => storage.set(key, String(value)),
};

const draft = {
  ...createDesktopConfigDraft({
    siteName: 'Relay',
    siteUrl: 'https://relay.example.com/v1',
    apiKey: 'secret-key',
    selectedModel: 'gpt-5.6-luna',
  }),
  selectedApps: ['claude', 'codex', 'grokbuild', 'opencode', 'openclaw'],
  effort: 'max',
};

const preview = buildDesktopConfigPreview(draft, {
  files: [
    {
      appId: 'claude',
      appName: 'Claude',
      fileId: 'settings',
      label: 'settings.json',
      path: 'C:/Users/example/.claude/settings.json',
      exists: true,
      content: '{"env":{}}',
    },
    {
      appId: 'codex',
      appName: 'Codex',
      fileId: 'auth',
      label: 'auth.json',
      path: 'C:/Users/example/.codex/auth.json',
      exists: true,
      content: '{}',
    },
    {
      appId: 'codex',
      appName: 'Codex',
      fileId: 'config',
      label: 'config.toml',
      path: 'C:/Users/example/.codex/config.toml',
      exists: true,
      content: '',
    },
    {
      appId: 'grokbuild',
      appName: 'Grok Build',
      fileId: 'config',
      label: 'config.toml',
      path: 'C:/Users/example/.grok/config.toml',
      exists: true,
      content: '',
    },
    {
      appId: 'opencode',
      appName: 'OpenCode',
      fileId: 'config',
      label: 'opencode.json',
      path: 'C:/Users/example/.config/opencode/opencode.json',
      exists: true,
      content: '{}',
    },
    {
      appId: 'openclaw',
      appName: 'OpenClaw',
      fileId: 'config',
      label: 'openclaw.json',
      path: 'C:/Users/example/.openclaw/openclaw.json',
      exists: true,
      content: '{}',
    },
  ],
});

assert.deepEqual(preview.errors, []);
assert.equal(preview.writes.length, 6);

const writes = new Map(preview.writes.map(file => [`${file.appId}/${file.fileId}`, file.content]));
assert.match(writes.get('claude/settings'), /"effortLevel": "max"/);
assert.match(writes.get('codex/config'), /model_reasoning_effort = "max"/);
assert.match(writes.get('grokbuild/config'), /reasoning_effort = "max"/);

const openCodeConfig = JSON.parse(writes.get('opencode/config'));
assert.equal(openCodeConfig.provider.custom.models['gpt-5.6-luna'].options.reasoningEffort, 'max');

const openClawConfig = JSON.parse(writes.get('openclaw/config'));
assert.equal(
  openClawConfig.agents.defaults.models['custom/gpt-5.6-luna'].params.extra_body.reasoning_effort,
  'max',
);

const anthropicOpenClawDraft = {
  ...draft,
  selectedApps: ['openclaw'],
  openclawApi: 'anthropic-messages',
};
const anthropicOpenClawPreview = buildDesktopConfigPreview(anthropicOpenClawDraft, {
  files: [{
    appId: 'openclaw',
    appName: 'OpenClaw',
    fileId: 'config',
    label: 'openclaw.json',
    path: 'C:/Users/example/.openclaw/openclaw.json',
    exists: true,
    content: '{}',
  }],
});
const anthropicOpenClawConfig = JSON.parse(anthropicOpenClawPreview.writes[0].content);
assert.equal(
  anthropicOpenClawConfig.agents.defaults.models['custom/gpt-5.6-luna'].params.thinking,
  'max',
);
