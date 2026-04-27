import assert from 'node:assert/strict';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const storePath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'utils', 'historySnapshotStore.js');
const storeUrl = pathToFileURL(storePath).href;

const originalWindow = globalThis.window;
const originalIndexedDB = globalThis.indexedDB;
const originalCustomEvent = globalThis.CustomEvent;

function createLocalStorageMock() {
  const storage = new Map();
  return {
    getItem(key) {
      return storage.has(String(key)) ? storage.get(String(key)) : null;
    },
    setItem(key, value) {
      storage.set(String(key), String(value));
    },
    removeItem(key) {
      storage.delete(String(key));
    },
    clear() {
      storage.clear();
    },
  };
}

function cloneValue(value) {
  return JSON.parse(JSON.stringify(value));
}

function createIndexedDBMock() {
  const records = new Map();

  const store = {
    get(key) {
      const request = {
        result: null,
        error: null,
        onsuccess: null,
        onerror: null,
      };

      queueMicrotask(() => {
        request.result = records.has(key) ? cloneValue(records.get(key)) : null;
        if (typeof request.onsuccess === 'function') request.onsuccess();
      });

      return request;
    },
    put(value) {
      const request = {
        result: null,
        error: null,
        onsuccess: null,
        onerror: null,
      };

      queueMicrotask(() => {
        records.set(String(value.id), cloneValue(value));
        if (typeof request.onsuccess === 'function') request.onsuccess();
      });

      return request;
    },
  };

  const db = {
    objectStoreNames: {
      contains() {
        return true;
      },
    },
    createObjectStore() {
      return store;
    },
    transaction() {
      return {
        objectStore() {
          return store;
        },
        onabort: null,
        error: null,
      };
    },
    close() {},
  };

  return {
    open() {
      const request = {
        result: db,
        error: null,
        onupgradeneeded: null,
        onsuccess: null,
        onerror: null,
      };

      queueMicrotask(() => {
        if (typeof request.onupgradeneeded === 'function') request.onupgradeneeded();
        if (typeof request.onsuccess === 'function') request.onsuccess();
      });

      return request;
    },
  };
}

function installEnvironment() {
  const localStorage = createLocalStorageMock();
  const indexedDB = createIndexedDBMock();
  const logs = [];

  globalThis.CustomEvent = class CustomEvent extends Event {
    constructor(type, init = {}) {
      super(type, init);
      this.detail = init.detail;
    }
  };

  globalThis.window = {
    localStorage,
    indexedDB,
    go: {
      main: {
        App: {
          AppendClientLog(scope, message) {
            logs.push({ scope, message });
          },
        },
      },
    },
    dispatchEvent() {
      return true;
    },
  };

  globalThis.indexedDB = indexedDB;

  return { localStorage, logs };
}

async function loadFreshStoreModule() {
  return import(`${storeUrl}?t=${Date.now()}-${Math.random()}`);
}

try {
  const { localStorage } = installEnvironment();
  const store = await loadFreshStoreModule();
  const snapshot = [
    {
      id: 'site-1',
      siteUrl: 'https://example.com',
      apiKey: 'key-1',
      status: 'success',
      nested: {
        answer: 42,
        flags: ['a', 'b'],
      },
      tokens: [
        { model: 'gpt-4o', status: 'success' },
      ],
      updatedAt: 123456789,
    },
  ];
  const expected = cloneValue(snapshot);

  const saved = await store.saveLastResultsSnapshot(snapshot);

  snapshot[0].nested.answer = 0;
  snapshot[0].tokens.push({ model: 'mutated', status: 'broken' });

  assert.deepStrictEqual(saved, expected);
  assert.deepStrictEqual(store.getCachedLastResultsSnapshot(), expected);
  assert.deepStrictEqual(await store.loadLastResultsSnapshot(), expected);
  assert.equal(store.hasCachedLastResultsSnapshot(), true);
  assert.equal(JSON.parse(localStorage.getItem(store.HISTORY_SNAPSHOT_INDEX_KEY)).count, 1);
  assert.equal(JSON.parse(localStorage.getItem(store.HISTORY_SNAPSHOT_INDEX_KEY)).firstUpdatedAt, 123456789);

  const emptySaved = await store.saveLastResultsSnapshot(undefined);
  assert.deepStrictEqual(emptySaved, []);
  assert.deepStrictEqual(store.getCachedLastResultsSnapshot(), []);
  assert.equal(JSON.parse(localStorage.getItem(store.HISTORY_SNAPSHOT_INDEX_KEY)).count, 0);

  console.log('PASS tests/historySnapshotStore.test.mjs');
} finally {
  globalThis.window = originalWindow;
  globalThis.indexedDB = originalIndexedDB;
  globalThis.CustomEvent = originalCustomEvent;
}
