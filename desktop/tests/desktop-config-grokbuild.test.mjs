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
    selectedModel: 'grok-4.5',
  }),
  selectedApps: ['grokbuild'],
  grokbuildApiBackend: 'chat_completions',
};

const preview = buildDesktopConfigPreview(draft, {
  files: [
    {
      appId: 'grokbuild',
      appName: 'Grok Build',
      fileId: 'config',
      label: 'config.toml',
      path: 'C:/Users/example/.grok/config.toml',
      exists: true,
      content: `[models]
default = "old-profile"

[model."old-profile"]
model = "old-model"
base_url = "https://old.example.com/v1"
name = "Old Relay"
api_key = "old-key"
api_backend = "responses"
context_window = 128000

[mcp.servers.echo]
command = "echo"

[model."other-profile"]
model = "grok-4"
base_url = "https://other.example.com/v1"
name = "Other"
api_key = "other-key"
api_backend = "responses"
context_window = 500000
`,
    },
  ],
});

assert.deepEqual(preview.errors, []);
assert.equal(preview.writes.length, 1);

const content = preview.writes[0].content;
assert.match(content, /\[models\]\s+default = "grok-4\.5"/);
assert.match(content, /\[model\."grok-4\.5"\]\nmodel = "grok-4\.5"/);
assert.match(content, /base_url = "https:\/\/relay\.example\.com\/v1"/);
assert.match(content, /name = "Relay"/);
assert.match(content, /api_key = "secret-key"/);
assert.match(content, /api_backend = "chat_completions"/);
assert.match(content, /context_window = 500000/);
assert.match(content, /\[mcp\.servers\.echo\]\ncommand = "echo"/);
assert.match(content, /\[model\."other-profile"\]/);
assert.doesNotMatch(content, /\[model\."old-profile"\]/);

const advancedProxyDraft = {
  ...draft,
  grokbuildUseAdvancedProxy: true,
};

localStorage.setItem('batch_api_check_advanced_proxy_config_v1', JSON.stringify({
  enabled: true,
  grokbuild: {
    enabled: true,
    basePath: '/advanced-proxy/grokbuild/v1',
  },
  queues: {
    global: { providers: [] },
    grokbuild: {
      inheritGlobal: false,
      providers: [{
        id: 'grok-provider',
        baseUrl: 'https://relay.example.com/v1',
        apiKey: 'secret-key',
        model: 'grok-4.5',
        apiFormat: 'openai_responses',
        enabled: true,
      }],
    },
  },
}));

const advancedProxyPreview = buildDesktopConfigPreview(advancedProxyDraft, {
  files: [{
    appId: 'grokbuild',
    appName: 'Grok Build',
    fileId: 'config',
    label: 'config.toml',
    path: 'C:/Users/example/.grok/config.toml',
    exists: false,
    content: '',
  }],
});

assert.deepEqual(advancedProxyPreview.errors, []);
assert.match(advancedProxyPreview.writes[0].content, /base_url = "http:\/\/127\.0\.0\.1:8888\/advanced-proxy\/grokbuild\/v1"/);
assert.match(advancedProxyPreview.writes[0].content, /name = "AllApiDeck Advanced Proxy"/);
assert.match(advancedProxyPreview.writes[0].content, /api_key = "PROXY_MANAGED"/);
