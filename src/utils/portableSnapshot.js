import { loadLastResultsSnapshot, saveLastResultsSnapshot } from './historySnapshotStore.js';

export const PORTABLE_HISTORY_SNAPSHOT_STORAGE_KEY = 'batch_api_check_portable_history_snapshot_v1';

export async function snapshotPortableLocalStorage() {
  const snapshot = {};
  try {
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index);
      if (!key) continue;
      snapshot[key] = localStorage.getItem(key);
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
    localStorage.setItem(key, value == null ? '' : String(value));
  });

  if (!historySnapshotRaw) return;

  try {
    const parsedHistorySnapshot = JSON.parse(historySnapshotRaw);
    if (Array.isArray(parsedHistorySnapshot)) {
      await saveLastResultsSnapshot(parsedHistorySnapshot);
    }
  } catch {}
}
