import {
  HISTORY_SNAPSHOT_PORTABLE_KEY,
  loadLastResultsSnapshot,
  saveLastResultsSnapshot,
} from './historySnapshotStore.js';
import {
  SITE_CACHE_STORAGE_KEY,
  normalizeSiteCacheRecord,
} from './siteCacheStore.js';

export const PORTABLE_HISTORY_SNAPSHOT_STORAGE_KEY = HISTORY_SNAPSHOT_PORTABLE_KEY;

const KEY_RECORD_STORAGE_KEYS = new Set([
  'api_check_key_management_records_v1',
  'api_check_key_management_manual_records_v1',
]);

function compactPortableStorageValue(key, value) {
  const raw = value == null ? '' : String(value);
  try {
    if (key === SITE_CACHE_STORAGE_KEY) {
      const records = JSON.parse(raw || '[]');
      if (!Array.isArray(records)) return raw;
      return JSON.stringify(records.map(record => normalizeSiteCacheRecord(record)).filter(Boolean));
    }
    if (KEY_RECORD_STORAGE_KEYS.has(key)) {
      const records = JSON.parse(raw || '[]');
      if (!Array.isArray(records)) return raw;
      return JSON.stringify(records.map(record => ({
        ...record,
        modelsText: undefined,
        quickTestRemark: String(record?.quickTestRemark || '').slice(0, 2048),
        quickTestResponseContent: String(record?.quickTestResponseContent || '').slice(0, 16384),
      })));
    }
  } catch {}
  return raw;
}

export async function snapshotPortableLocalStorage() {
  const snapshot = {};
  try {
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index);
      if (!key) continue;
      if (key === PORTABLE_HISTORY_SNAPSHOT_STORAGE_KEY) continue;
      snapshot[key] = compactPortableStorageValue(key, localStorage.getItem(key));
    }
  } catch {}

  try {
    const historySnapshot = await loadLastResultsSnapshot();
    if (Array.isArray(historySnapshot) && historySnapshot.length > 0) {
      snapshot[PORTABLE_HISTORY_SNAPSHOT_STORAGE_KEY] = JSON.stringify(historySnapshot);
    }
  } catch {}

  return snapshot;
}

export async function applyPortableLocalStorageSnapshot(snapshot) {
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) {
    throw new Error('invalid_localstorage_snapshot');
  }

  const historySnapshotRaw = String(snapshot[PORTABLE_HISTORY_SNAPSHOT_STORAGE_KEY] || '').trim();

  localStorage.clear();
  Object.entries(snapshot).forEach(([key, value]) => {
    if (key === PORTABLE_HISTORY_SNAPSHOT_STORAGE_KEY) return;
    localStorage.setItem(key, compactPortableStorageValue(key, value));
  });

  if (!historySnapshotRaw) return;

  try {
    const parsedHistorySnapshot = JSON.parse(historySnapshotRaw);
    if (Array.isArray(parsedHistorySnapshot)) {
      await saveLastResultsSnapshot(parsedHistorySnapshot);
    }
  } catch {}
}
