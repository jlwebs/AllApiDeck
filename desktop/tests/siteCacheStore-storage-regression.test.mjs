import assert from 'node:assert/strict';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const storePath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'utils', 'siteCacheStore.js');
const storeUrl = pathToFileURL(storePath).href;

const originalWindow = globalThis.window;
const originalCustomEvent = globalThis.CustomEvent;

function createStorageMock(options = {}) {
  const values = new Map();
  let remainingFailures = Number(options.failSetCount || 0);
  return {
    getItem(key) {
      return values.has(String(key)) ? values.get(String(key)) : null;
    },
    setItem(key, value) {
      if (remainingFailures > 0) {
        remainingFailures -= 1;
        const error = new Error('Quota exceeded');
        error.name = 'QuotaExceededError';
        throw error;
      }
      values.set(String(key), String(value));
    },
    removeItem(key) {
      values.delete(String(key));
    },
  };
}

try {
  globalThis.CustomEvent = class CustomEvent extends Event {
    constructor(type, init = {}) {
      super(type, init);
      this.detail = init.detail;
    }
  };

  const localStorage = createStorageMock();
  globalThis.window = {
    localStorage,
    sessionStorage: createStorageMock(),
    dispatchEvent() {
      return true;
    },
  };

  const store = await import(`${storeUrl}?t=${Date.now()}-${Math.random()}`);
  const tokenKey = 'sk-storage-regression';
  const modelNodes = Array.from({ length: 4200 }, (_, index) => ({
    key: `model|token|model-${index}`,
    title: `model-${index}-${'x'.repeat(36)}`,
  }));
  const largeTree = [{
    key: `token|site-cache-key|${tokenKey}`,
    title: 'Token',
    children: modelNodes,
  }];

  const persisted = store.persistSiteCacheRecords([{
    siteCacheKey: 'site-cache-key',
    siteName: 'Storage Regression',
    siteUrl: 'https://example.com',
    tokens: [{ key: tokenKey, name: 'Primary Token' }],
    cachedTreeNodes: largeTree,
  }], { broadcast: false });

  assert.equal(persisted.length, 1);
  assert.deepStrictEqual(persisted[0].cachedTreeNodes, []);
  assert.equal(persisted[0].tokens[0].models.length, modelNodes.length);
  assert.ok(
    localStorage.getItem(store.SITE_CACHE_STORAGE_KEY).length < JSON.stringify(largeTree).length,
    'compacted cache should be smaller than the original tree'
  );

  const quotaStorage = createStorageMock({ failSetCount: 1 });
  globalThis.window.localStorage = quotaStorage;
  const quotaPersisted = store.persistSiteCacheRecords([{
    siteCacheKey: 'quota-site',
    siteName: 'Quota Site',
    siteUrl: 'https://quota.example.com',
    tokens: [{ key: 'sk-quota' }],
    cachedTreeNodes: [{
      key: 'token|quota-site|sk-quota',
      children: [{ key: 'model|quota|gpt-test', title: 'gpt-test' }],
    }],
  }], { broadcast: false });

  assert.deepStrictEqual(quotaPersisted[0].cachedTreeNodes, []);
  assert.deepStrictEqual(
    JSON.parse(quotaStorage.getItem(store.SITE_CACHE_STORAGE_KEY))[0].cachedTreeNodes,
    []
  );

  console.log('PASS tests/siteCacheStore-storage-regression.test.mjs');
} finally {
  globalThis.window = originalWindow;
  globalThis.CustomEvent = originalCustomEvent;
}
