import { EventsEmit, EventsOn } from '../../wailsjs/runtime/runtime.js';
import {
  buildRowKey,
  loadPanelRecords,
  META_STORAGE_KEY,
  normalizeApiKey,
  normalizeModels,
  normalizeSiteUrl,
  persistPanelRecords,
} from './keyPanelStore.js';
import { resolveClipboardImportRecords } from './clipboardSmartImport.js';

export const CLIPBOARD_IMPORT_REQUEST_EVENT = 'batch-api-check:clipboard-import-request';
export const CLIPBOARD_IMPORT_RESULT_EVENT = 'batch-api-check:clipboard-import-result';
export const KEY_GROUPS_STORAGE_KEY = 'api_check_key_management_groups_v1';

let clipboardImportBridgeInstalled = false;

function normalizeStringList(value) {
  if (!Array.isArray(value)) return [];
  return Array.from(new Set(value.map(item => String(item || '').trim()).filter(Boolean)));
}

function normalizeTargetGroupName(value) {
  const name = String(value || '').trim();
  if (name === '全部分组' || name === '全部密钥') return '';
  return name;
}

function normalizeGroups(value) {
  if (!Array.isArray(value)) return [];
  return value
    .map(group => ({
      id: String(group?.id || '').trim(),
      name: String(group?.name || '').trim(),
      createdAt: Number(group?.createdAt || Date.now()),
    }))
    .filter(group => group.id && group.name);
}

function buildGroupId(now = Date.now()) {
  return `group::${now}::${Math.random().toString(36).slice(2, 7)}`;
}

export function mergeClipboardImportState({
  existingRecords = [],
  existingGroups = [],
  importedRecords = [],
  targetGroupName = '',
  now = Date.now(),
  groupIdFactory = () => buildGroupId(now),
} = {}) {
  const groups = normalizeGroups(existingGroups);
  const resolvedTargetGroupName = normalizeTargetGroupName(targetGroupName);
  let targetGroup = resolvedTargetGroupName
    ? groups.find(group => group.name === resolvedTargetGroupName) || null
    : null;
  let groupCreated = false;
  if (resolvedTargetGroupName && !targetGroup) {
    targetGroup = {
      id: String(groupIdFactory() || '').trim() || buildGroupId(now),
      name: resolvedTargetGroupName,
      createdAt: now,
    };
    groups.push(targetGroup);
    groupCreated = true;
  }

  const merged = new Map();
  existingRecords.forEach(record => {
    const siteUrl = normalizeSiteUrl(record?.siteUrl);
    const apiKey = normalizeApiKey(record?.apiKey);
    if (!siteUrl || !apiKey) return;
    merged.set(buildRowKey(siteUrl, apiKey), { ...record, siteUrl, apiKey });
  });

  let createdCount = 0;
  let updatedCount = 0;
  importedRecords.forEach(rawRecord => {
    const siteUrl = normalizeSiteUrl(rawRecord?.siteUrl);
    const apiKey = normalizeApiKey(rawRecord?.apiKey);
    if (!siteUrl || !apiKey) return;
    const canonicalRowKey = buildRowKey(siteUrl, apiKey);
    const existing = merged.get(canonicalRowKey) || null;
    const modelsList = normalizeModels(rawRecord?.modelsList || rawRecord?.modelsText || existing?.modelsList || existing?.modelsText);
    const groupIds = normalizeStringList([
      ...normalizeStringList(existing?.groupIds),
      ...normalizeStringList(rawRecord?.groupIds),
      ...(targetGroup ? [targetGroup.id] : []),
    ]);
    const record = {
      ...existing,
      ...rawRecord,
      rowKey: existing?.rowKey || rawRecord?.rowKey || canonicalRowKey,
      sourceType: existing?.sourceType || rawRecord?.sourceType || 'auto',
      siteName: String(rawRecord?.siteName || existing?.siteName || '未命名站点').trim() || '未命名站点',
      tokenName: String(rawRecord?.tokenName || existing?.tokenName || '').trim(),
      siteUrl,
      apiKey,
      modelsList,
      modelsText: modelsList.join(', ') || '未提供模型信息',
      selectedModel: String(rawRecord?.selectedModel || existing?.selectedModel || '').trim(),
      groupIds,
      groupSelectedModels: {
        ...(existing?.groupSelectedModels && typeof existing.groupSelectedModels === 'object' ? existing.groupSelectedModels : {}),
        ...(rawRecord?.groupSelectedModels && typeof rawRecord.groupSelectedModels === 'object' ? rawRecord.groupSelectedModels : {}),
      },
      status: Number(rawRecord?.status || existing?.status || 1),
      createdAt: Number(existing?.createdAt || rawRecord?.createdAt || now),
      updatedAt: now,
      quickTestLoading: false,
      balanceLoading: false,
      modelLoading: false,
    };
    merged.set(canonicalRowKey, record);
    if (existing) updatedCount += 1;
    else createdCount += 1;
  });

  return {
    records: Array.from(merged.values()),
    groups,
    targetGroup,
    groupCreated,
    importedCount: createdCount + updatedCount,
    createdCount,
    updatedCount,
  };
}

function loadStoredGroups() {
  try {
    return normalizeGroups(JSON.parse(localStorage.getItem(KEY_GROUPS_STORAGE_KEY) || '[]'));
  } catch {
    return [];
  }
}

export async function importClipboardTextIntoKeyStore({ clipboardText, targetGroupName = '' } = {}) {
  const resolved = await resolveClipboardImportRecords(clipboardText);
  const { records: existingRecords } = loadPanelRecords();
  const merged = mergeClipboardImportState({
    existingRecords,
    existingGroups: loadStoredGroups(),
    importedRecords: resolved.records,
    targetGroupName,
  });
  if (merged.importedCount === 0) throw new Error('未识别到可导入的有效记录');

  localStorage.setItem(KEY_GROUPS_STORAGE_KEY, JSON.stringify(merged.groups));
  localStorage.setItem(META_STORAGE_KEY, JSON.stringify({
    lastBatchSyncAt: Date.now(),
    lastBatchSyncCount: merged.importedCount,
    lastBatchFailedCount: 0,
    lastBatchSyncStrategy: 'clipboard-api',
  }));
  persistPanelRecords(merged.records);

  return {
    mode: resolved.mode,
    importedCount: merged.importedCount,
    createdCount: merged.createdCount,
    updatedCount: merged.updatedCount,
    targetGroupName: merged.targetGroup?.name || '全部分组',
    groupCreated: merged.groupCreated,
  };
}

export function installClipboardImportBridge() {
  if (clipboardImportBridgeInstalled) return true;
  if (typeof window === 'undefined' || typeof window.runtime?.EventsOn !== 'function') return false;

  EventsOn(CLIPBOARD_IMPORT_REQUEST_EVENT, async payload => {
    const requestId = String(payload?.requestId || '').trim();
    if (!requestId) return;
    try {
      const result = await importClipboardTextIntoKeyStore(payload);
      EventsEmit(CLIPBOARD_IMPORT_RESULT_EVENT, {
        requestId,
        success: true,
        ...result,
      });
    } catch (error) {
      EventsEmit(CLIPBOARD_IMPORT_RESULT_EVENT, {
        requestId,
        success: false,
        error: error?.message || String(error || '导入失败'),
      });
    }
  });
  clipboardImportBridgeInstalled = true;
  return true;
}
