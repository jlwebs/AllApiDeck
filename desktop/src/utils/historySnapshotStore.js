const HISTORY_DB_NAME = 'api_check_history_snapshot_v1';
const HISTORY_DB_VERSION = 1;
const HISTORY_STORE_NAME = 'snapshots';
const HISTORY_LATEST_KEY = 'latest';
export const HISTORY_SNAPSHOT_SYNC_EVENT = 'batch-api-check:history-snapshot-sync';
export const HISTORY_SNAPSHOT_INDEX_KEY = 'api_check_last_results_index_v1';
export const HISTORY_SNAPSHOT_LEGACY_KEY = 'api_check_last_results';

let cachedSnapshot = [];
let cachedSnapshotRaw = '';
let cachedSnapshotUpdatedAt = 0;
let cacheHydrated = false;
let hydrationPromise = null;

function appendHistorySnapshotLog(scope, message) {
  try {
    const app = window?.go?.main?.App;
    if (typeof app?.AppendClientLog === 'function') {
      app.AppendClientLog(scope, message);
    }
  } catch {
    // ignore logging failures
  }
}

function hasIndexedDB() {
  return typeof indexedDB !== 'undefined' && indexedDB && typeof indexedDB.open === 'function';
}

function cloneSnapshot(snapshot) {
  try {
    return JSON.parse(JSON.stringify(snapshot));
  } catch {
    return [];
  }
}

function getSnapshotIndex(snapshot) {
  const list = Array.isArray(snapshot) ? snapshot : [];
  const firstUpdatedAt = Number(list[0]?.updatedAt || list[0]?.finishedAt || list[0]?.completedAt || list[0]?.timestamp || 0);
  const lastUpdatedAt = Number(list[list.length - 1]?.updatedAt || list[list.length - 1]?.finishedAt || list[list.length - 1]?.completedAt || list[list.length - 1]?.timestamp || 0);
  return {
    updatedAt: cachedSnapshotUpdatedAt || Date.now(),
    count: list.length,
    firstUpdatedAt,
    lastUpdatedAt,
  };
}

function persistSnapshotIndex(snapshot) {
  if (typeof window === 'undefined' || !window.localStorage) return;
  try {
    window.localStorage.setItem(HISTORY_SNAPSHOT_INDEX_KEY, JSON.stringify(getSnapshotIndex(snapshot)));
  } catch {
    // ignore index write failures
  }
}

function readSnapshotIndex() {
  if (typeof window === 'undefined' || !window.localStorage) return null;
  try {
    const raw = window.localStorage.getItem(HISTORY_SNAPSHOT_INDEX_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : null;
  } catch {
    return null;
  }
}

function readLegacySnapshotRaw() {
  if (typeof window === 'undefined' || !window.localStorage) return '';
  try {
    return String(window.localStorage.getItem(HISTORY_SNAPSHOT_LEGACY_KEY) || '').trim();
  } catch {
    return '';
  }
}

function getDb() {
  if (!hasIndexedDB()) {
    return Promise.reject(new Error('indexeddb_unavailable'));
  }

  return new Promise((resolve, reject) => {
    const request = indexedDB.open(HISTORY_DB_NAME, HISTORY_DB_VERSION);

    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(HISTORY_STORE_NAME)) {
        db.createObjectStore(HISTORY_STORE_NAME, { keyPath: 'id' });
      }
    };

    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('indexeddb_open_failed'));
  });
}

async function readLatestSnapshotRecord() {
  const db = await getDb();
  try {
    appendHistorySnapshotLog('history.snapshot', 'read latest snapshot from indexeddb');
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(HISTORY_STORE_NAME, 'readonly');
      const store = tx.objectStore(HISTORY_STORE_NAME);
      const request = store.get(HISTORY_LATEST_KEY);
      request.onsuccess = () => resolve(request.result || null);
      request.onerror = () => reject(request.error || new Error('indexeddb_read_failed'));
    });
  } finally {
    db.close();
  }
}

async function writeLatestSnapshotRecord(rawJson, snapshot) {
  const db = await getDb();
  try {
    appendHistorySnapshotLog('history.snapshot', `write latest snapshot start count=${Array.isArray(snapshot) ? snapshot.length : 0} rawLength=${String(rawJson || '').length}`);
    await new Promise((resolve, reject) => {
      const tx = db.transaction(HISTORY_STORE_NAME, 'readwrite');
      const store = tx.objectStore(HISTORY_STORE_NAME);
      const request = store.put({
        id: HISTORY_LATEST_KEY,
        rawJson,
        count: Array.isArray(snapshot) ? snapshot.length : 0,
        updatedAt: Date.now(),
      });
      request.onsuccess = () => resolve();
      request.onerror = () => reject(request.error || new Error('indexeddb_write_failed'));
      tx.onabort = () => reject(tx.error || new Error('indexeddb_tx_aborted'));
    });
    appendHistorySnapshotLog('history.snapshot', `write latest snapshot ok count=${Array.isArray(snapshot) ? snapshot.length : 0}`);
  } finally {
    db.close();
  }
}

function setCachedSnapshot(snapshot, rawJson = '', updatedAt = Date.now()) {
  cachedSnapshot = cloneSnapshot(snapshot);
  cachedSnapshotRaw = String(rawJson || '');
  cachedSnapshotUpdatedAt = Number(updatedAt || Date.now());
  cacheHydrated = true;
  persistSnapshotIndex(cachedSnapshot);
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(HISTORY_SNAPSHOT_SYNC_EVENT, {
      detail: {
        count: cachedSnapshot.length,
        updatedAt: Date.now(),
      },
    }));
  }
}

export function getCachedLastResultsSnapshot() {
  return cloneSnapshot(cachedSnapshot);
}

export function getCachedLastResultsSnapshotRaw() {
  return String(cachedSnapshotRaw || '');
}

export function hasCachedLastResultsSnapshot() {
  if (Array.isArray(cachedSnapshot) && cachedSnapshot.length > 0) return true;
  const index = readSnapshotIndex();
  return Boolean(index && Number(index.count || 0) > 0);
}

export async function hydrateLastResultsSnapshotCache() {
  appendHistorySnapshotLog('history.snapshot', `hydrate start cacheHydrated=${cacheHydrated} cachedCount=${Array.isArray(cachedSnapshot) ? cachedSnapshot.length : 0}`);
  if (cacheHydrated && Array.isArray(cachedSnapshot)) {
    appendHistorySnapshotLog('history.snapshot', `hydrate cache hit count=${cachedSnapshot.length}`);
    return getCachedLastResultsSnapshot();
  }
  if (hydrationPromise) {
    return hydrationPromise;
  }

  hydrationPromise = (async () => {
    let rawJson = '';
    let recordUpdatedAt = 0;
    try {
      const record = await readLatestSnapshotRecord();
      rawJson = String(record?.rawJson || '').trim();
      recordUpdatedAt = Number(record?.updatedAt || 0);
      appendHistorySnapshotLog('history.snapshot', `hydrate read record updatedAt=${recordUpdatedAt} rawLength=${rawJson.length}`);
    } catch {
      rawJson = '';
      recordUpdatedAt = 0;
      appendHistorySnapshotLog('history.snapshot', 'hydrate read failed, falling back to legacy storage');
    }

    if (!rawJson) {
      rawJson = readLegacySnapshotRaw();
      if (rawJson) {
        try {
          const legacyParsed = JSON.parse(rawJson);
          if (Array.isArray(legacyParsed)) {
            appendHistorySnapshotLog('history.snapshot', `hydrate migrating legacy snapshot count=${legacyParsed.length}`);
            await writeLatestSnapshotRecord(rawJson, legacyParsed);
          }
        } catch {
          // ignore migration failures
        }
      }
    }

    if (rawJson) {
      try {
        const parsed = JSON.parse(rawJson);
        if (Array.isArray(parsed)) {
          if (cachedSnapshotUpdatedAt > 0 && cachedSnapshotUpdatedAt >= recordUpdatedAt) {
            cacheHydrated = true;
            appendHistorySnapshotLog('history.snapshot', `hydrate keep newer cache count=${cachedSnapshot.length}`);
            return getCachedLastResultsSnapshot();
          }
          setCachedSnapshot(parsed, rawJson, recordUpdatedAt || Date.now());
          appendHistorySnapshotLog('history.snapshot', `hydrate success count=${parsed.length}`);
          return getCachedLastResultsSnapshot();
        }
      } catch {
        // fall through to empty snapshot
      }
    }

    if (cachedSnapshotUpdatedAt === 0) {
      setCachedSnapshot([], '', 0);
      appendHistorySnapshotLog('history.snapshot', 'hydrate no snapshot available');
    } else {
      cacheHydrated = true;
    }
    return [];
  })();

  try {
    return await hydrationPromise;
  } finally {
    hydrationPromise = null;
  }
}

export async function saveLastResultsSnapshot(results) {
  const snapshot = Array.isArray(results) ? cloneSnapshot(results) : [];
  let rawJson = '[]';
  const updatedAt = Date.now();
  appendHistorySnapshotLog('history.snapshot', `save start count=${snapshot.length}`);
  try {
    rawJson = JSON.stringify(snapshot);
  } catch {
    snapshot.length = 0;
    rawJson = '[]';
    appendHistorySnapshotLog('history.snapshot', 'save stringify failed, coerced to empty snapshot');
  }

  setCachedSnapshot(snapshot, rawJson, updatedAt);
  appendHistorySnapshotLog('history.snapshot', `save cached count=${snapshot.length} rawLength=${rawJson.length}`);

  try {
    await writeLatestSnapshotRecord(rawJson, snapshot);
  } catch (error) {
    console.warn('[HistorySnapshot] indexeddb save failed:', error?.message || String(error));
    appendHistorySnapshotLog('history.snapshot', `save indexeddb failed ${error?.message || String(error)}`);
  }

  return getCachedLastResultsSnapshot();
}

export async function loadLastResultsSnapshot() {
  const snapshot = await hydrateLastResultsSnapshotCache();
  return Array.isArray(snapshot) ? snapshot : [];
}
