import assert from 'node:assert/strict';

globalThis.window = {
  dispatchEvent() {},
};

const bridge = await import('../src/utils/advancedProxyBridge.js');

const baseConfig = bridge.normalizeAdvancedProxyConfig({
  enabled: true,
  queues: {
    global: {
      inheritGlobal: false,
      providers: [{
        id: 'row-provider-1',
        rowKey: 'row-provider-1',
        name: 'Old Provider',
        baseUrl: 'https://old.example/v1',
        apiKey: 'sk-old',
        model: 'old-model',
        apiFormat: 'openai_responses',
        apiKeyField: 'ANTHROPIC_AUTH_TOKEN',
        enabled: true,
        sortIndex: 1,
      }],
    },
    codex: {
      inheritGlobal: true,
      providers: [],
    },
  },
});

const records = [{
  rowKey: 'row-provider-1',
  siteName: 'New Provider',
  siteUrl: 'https://new.example/v1/',
  apiKey: 'sk-new',
  selectedModel: 'new-model',
  quickTestModel: 'quick-test-model',
  sourceType: 'manual',
}];

const { config: syncedConfig, changed } = bridge.syncAdvancedProxyProvidersFromRecords(baseConfig, records);
const provider = syncedConfig.queues.global.providers[0];

assert.equal(changed, true);
assert.equal(provider.id, 'row-provider-1');
assert.equal(provider.rowKey, 'row-provider-1');
assert.equal(provider.name, 'New Provider');
assert.equal(provider.baseUrl, 'https://new.example/v1');
assert.equal(provider.apiKey, 'sk-new');
assert.equal(provider.model, 'new-model');
assert.equal(provider.apiFormat, 'openai_responses');
assert.equal(syncedConfig.claude.providers[0].model, 'new-model');

const { changed: unchanged } = bridge.syncAdvancedProxyProvidersFromRecords(syncedConfig, records);
assert.equal(unchanged, false);

const { config: scopedConfig } = bridge.syncAdvancedProxyProvidersFromRecords(baseConfig, records, {
  modelResolver: record => record.rowKey === 'row-provider-1' ? 'group-model' : '',
});

assert.equal(scopedConfig.queues.global.providers[0].model, 'group-model');
