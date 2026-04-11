import { fetchModelList } from './api.js';
import { apiFetch } from './runtimeApi.js';
import { fetchQuotaLabelWithBatchLogic, isDisplayableQuotaLabel } from './balance.js';
import { buildQuickTestMessages } from './quickTestPrompts.js';

export const STORAGE_KEY = 'api_check_key_management_records_v1';
export const MANUAL_STORAGE_KEY = 'api_check_key_management_manual_records_v1';
export const META_STORAGE_KEY = 'api_check_key_management_meta_v1';
export const LAST_RESULTS_STORAGE_KEY = 'api_check_last_results';
export const KEY_MANAGEMENT_SYNC_EVENT = 'batch-api-check:key-management-sync';

const DEFAULT_TEST_TIMEOUT_MS = 18000;

export function normalizeApiKey(rawKey) {
  let apiKey = String(rawKey || '').trim();
  if (!apiKey) return '';
  if (!apiKey.startsWith('sk-')) apiKey = `sk-${apiKey}`;
  return apiKey;
}

export function normalizeSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

export function normalizeModels(rawModels) {
  const list = Array.isArray(rawModels)
    ? rawModels
    : String(rawModels || '')
      .split(/[\n,\s]+/)
      .map(item => item.trim());
  return Array.from(
    new Set(
      list
        .map(item => (typeof item === 'string' ? item : item?.id || item?.model || ''))
        .map(item => String(item || '').trim())
        .filter(Boolean)
    )
  );
}

export function buildRowKey(siteUrl, apiKey) {
  return `${normalizeSiteUrl(siteUrl)}::${String(apiKey || '').trim()}`;
}

function buildManualRowKey() {
  return `manual::${Date.now()}::${Math.random().toString(36).slice(2, 8)}`;
}

function getHistoryTaskWeight(task) {
  const status = String(task?.status || '').trim();
  if (status === 'success') return 3;
  if (status === 'warning') return 2;
  if (status === 'pending') return 1;
  return 0;
}

function compareHistoryTasks(left, right) {
  const leftWeight = getHistoryTaskWeight(left);
  const rightWeight = getHistoryTaskWeight(right);
  if (leftWeight !== rightWeight) return rightWeight - leftWeight;
  const leftUpdatedAt = Number(left?.updatedAt || 0);
  const rightUpdatedAt = Number(right?.updatedAt || 0);
  if (leftUpdatedAt !== rightUpdatedAt) return rightUpdatedAt - leftUpdatedAt;
  return String(left?.modelName || '').localeCompare(String(right?.modelName || ''));
}

function isUsableHistoryTask(task) {
  const status = String(task?.status || '').trim();
  return Boolean(task?.modelName) && (status === 'success' || status === 'warning');
}

function getContextModelNames(context) {
  return Array.isArray(context?.tasks) ? context.tasks.map(task => task?.modelName).filter(Boolean) : [];
}

export function buildHistoryTaskSummary(task) {
  if (!task) return '';
  const suffix = String(task?.modelSuffix || '').replace(/[()]/g, '').trim();
  const statusText = String(task?.statusText || '').trim() || (String(task?.status || '').trim() === 'success' ? '一致可用' : '');
  const responseTime = String(task?.responseTime || '').trim();
  return [suffix, statusText, responseTime ? `${responseTime}s` : ''].filter(Boolean).join(' / ');
}

function buildModelOptionLabel(model, task = null) {
  const summary = buildHistoryTaskSummary(task);
  return summary ? `${model} (${summary})` : model;
}

function isLikelyChatModel(model) {
  return !/(embedding|tts|whisper|speech|audio|image|video|vision|flux|midjourney|mj|rerank|bge|stability|playground|suno|music|ocr|moderation|asr)/i.test(String(model || ''));
}

function pickPreferredModel(candidates) {
  const chatCandidates = normalizeModels(candidates).filter(isLikelyChatModel);
  if (!chatCandidates.length) return '';
  const preferredPatterns = [/gpt-5/i, /gpt-4\.1/i, /gpt-4o/i, /^o3/i, /^o1/i, /claude/i, /gemini/i, /deepseek/i, /qwen/i, /grok/i, /kimi/i, /chat/i];
  return chatCandidates.find(model => preferredPatterns.some(pattern => pattern.test(model))) || chatCandidates[0];
}

function loadBatchHistoryBalanceMap() {
  try {
    const raw = localStorage.getItem(LAST_RESULTS_STORAGE_KEY);
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return new Map();

    const balanceMap = new Map();
    parsed.forEach(item => {
      const siteUrl = normalizeSiteUrl(item?.siteUrl);
      const apiKey = String(item?.apiKey || '').trim();
      const balanceLabel = normalizeBalanceLabel(item?.quota);
      if (!siteUrl || !apiKey || !balanceLabel) return;
      const rowKey = buildRowKey(siteUrl, apiKey);
      const updatedAt = Number(item?.updatedAt || item?.finishedAt || item?.completedAt || item?.timestamp || Date.now());
      const current = balanceMap.get(rowKey);
      if (!current || updatedAt >= current.balanceUpdatedAt) {
        balanceMap.set(rowKey, {
          balanceLabel,
          balanceUpdatedAt: updatedAt,
        });
      }
    });
    return balanceMap;
  } catch (error) {
    console.error(error);
    return new Map();
  }
}

export function loadBatchHistoryContextMap() {
  try {
    const raw = localStorage.getItem(LAST_RESULTS_STORAGE_KEY);
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return new Map();

    const groupedContextMap = new Map();
    parsed.forEach(item => {
      const siteUrl = normalizeSiteUrl(item?.siteUrl);
      const apiKey = String(item?.apiKey || '').trim();
      if (!siteUrl || !apiKey) return;
      const rowKey = buildRowKey(siteUrl, apiKey);
      const updatedAt = Number(item?.updatedAt || item?.finishedAt || item?.completedAt || item?.timestamp || Date.now());
      const modelName = String(item?.modelName || '').trim();
      const current = groupedContextMap.get(rowKey) || {
        updatedAt: 0,
        accountData: null,
        tasksByModel: new Map(),
      };
      if (updatedAt >= current.updatedAt && item?.accountData) {
        current.accountData = item.accountData;
      }
      current.updatedAt = Math.max(current.updatedAt, updatedAt);
      if (modelName) {
        const taskSnapshot = {
          modelName,
          status: String(item?.status || '').trim(),
          statusText: String(item?.statusText || '').trim(),
          responseTime: String(item?.responseTime || '').trim(),
          modelSuffix: String(item?.modelSuffix || '').trim(),
          remark: String(item?.remark || '').trim(),
          updatedAt,
        };
        const previousTask = current.tasksByModel.get(modelName);
        if (!previousTask || compareHistoryTasks(taskSnapshot, previousTask) < 0) {
          current.tasksByModel.set(modelName, taskSnapshot);
        }
      }
      groupedContextMap.set(rowKey, current);
    });

    const contextMap = new Map();
    groupedContextMap.forEach((context, rowKey) => {
      const tasks = Array.from(context.tasksByModel.values()).sort(compareHistoryTasks);
      const preferredTask = tasks.find(isUsableHistoryTask) || tasks[0] || null;
      contextMap.set(rowKey, {
        updatedAt: context.updatedAt,
        accountData: context.accountData,
        tasks,
        preferredTask,
        preferredModel: preferredTask?.modelName || '',
      });
    });
    return contextMap;
  } catch (error) {
    console.error(error);
    return new Map();
  }
}

export function getBatchHistoryContextByKeys(contextMap, siteUrl, apiKey) {
  return contextMap.get(buildRowKey(siteUrl, apiKey)) || null;
}

function buildMergedModelList(record, context) {
  return normalizeModels([
    ...getContextModelNames(context),
    ...(Array.isArray(record?.modelsList) ? record.modelsList : []),
    record?.selectedModel || '',
    record?.quickTestModel || '',
  ]);
}

export function hydrateRecordModelSelection(record, contextMap) {
  const context = getBatchHistoryContextByKeys(contextMap, record?.siteUrl, record?.apiKey);
  const modelsList = buildMergedModelList(record, context);
  const selectedModel = String(record?.selectedModel || '').trim();
  const preferredModel = context?.preferredModel || pickPreferredModel(modelsList) || '';
  const nextSelectedModel = modelsList.includes(selectedModel)
    ? selectedModel
    : (modelsList.includes(preferredModel) ? preferredModel : (modelsList[0] || ''));
  return {
    ...record,
    modelsList,
    modelsText: modelsList.join(', ') || '未提供模型信息',
    selectedModel: nextSelectedModel,
    modelLoading: false,
    quickTestLoading: false,
    balanceLoading: false,
  };
}

export function getRecordModelOptions(record, contextMap) {
  const context = getBatchHistoryContextByKeys(contextMap, record?.siteUrl, record?.apiKey);
  const taskMap = new Map((Array.isArray(context?.tasks) ? context.tasks : []).map(task => [task.modelName, task]));
  return buildMergedModelList(record, context).map(model => ({
    label: buildModelOptionLabel(model, taskMap.get(model) || null),
    value: model,
  }));
}

export function getRecordSelectedModelTask(record, contextMap) {
  const context = getBatchHistoryContextByKeys(contextMap, record?.siteUrl, record?.apiKey);
  const selectedModel = String(record?.selectedModel || '').trim();
  if (!selectedModel || !Array.isArray(context?.tasks)) return null;
  return context.tasks.find(task => task.modelName === selectedModel) || null;
}

export function getRecordModelTooltip(record, contextMap) {
  const selectedModel = String(record?.selectedModel || '').trim();
  if (!selectedModel) return record.modelsText || '未提供模型信息';
  const summary = buildHistoryTaskSummary(getRecordSelectedModelTask(record, contextMap));
  return summary ? `${selectedModel} (${summary})` : selectedModel;
}

export function loadPanelRecords() {
  try {
    const contextMap = loadBatchHistoryContextMap();
    const autoRaw = localStorage.getItem(STORAGE_KEY);
    const manualRaw = localStorage.getItem(MANUAL_STORAGE_KEY);
    const autoRecords = JSON.parse(autoRaw || '[]');
    const manualRecords = JSON.parse(manualRaw || '[]');
    const parsedRecords = [
      ...(Array.isArray(autoRecords) ? autoRecords : []),
      ...(Array.isArray(manualRecords) ? manualRecords : []),
    ];

    const balanceMap = loadBatchHistoryBalanceMap();
    const records = parsedRecords
      .map(record => {
        const rowKey = record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey));
        const historyBalance = balanceMap.get(buildRowKey(record.siteUrl, record.apiKey));
        return {
          ...record,
          sourceType: record.sourceType || 'auto',
          rowKey,
          siteName: record.siteName || '未命名站点',
          tokenName: record.tokenName || '',
          siteUrl: normalizeSiteUrl(record.siteUrl),
          apiKey: String(record.apiKey || '').trim(),
          modelsList: normalizeModels(record.modelsList || record.modelsText),
          modelsText: record.modelsText || '未提供模型信息',
          selectedModel: String(record.selectedModel || '').trim(),
          quickTestStatus: record.quickTestStatus || '',
          quickTestLabel: record.quickTestLabel || '',
          quickTestModel: record.quickTestModel || '',
          quickTestRemark: record.quickTestRemark || '',
          quickTestAt: record.quickTestAt || null,
          quickTestResponseTime: record.quickTestResponseTime || '',
          quickTestResponseContent: record.quickTestResponseContent || '',
          balanceLabel: record.balanceLabel || historyBalance?.balanceLabel || '',
          balanceUpdatedAt: record.balanceUpdatedAt || historyBalance?.balanceUpdatedAt || null,
          balanceError: record.balanceError || '',
          remainQuota: record.remainQuota ?? null,
          usedQuota: record.usedQuota ?? null,
          unlimitedQuota: record.unlimitedQuota === true,
        };
      })
      .filter(record => record.siteUrl && record.apiKey)
      .map(record => hydrateRecordModelSelection(record, contextMap));

    return { records, contextMap };
  } catch (error) {
    console.error(error);
    return { records: [], contextMap: new Map() };
  }
}

export function persistPanelRecords(records) {
  const autoRecords = [];
  const manualRecords = [];

  records.forEach(({ quickTestLoading, balanceLoading, modelLoading, modelFetchKey, ...record }) => {
    const normalizedRecord = {
      ...record,
      sourceType: record.sourceType || 'auto',
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: normalizeModels(record.modelsList || record.modelsText).join(', '),
      selectedModel: String(record.selectedModel || '').trim(),
      quickTestResponseContent: record.quickTestResponseContent || '',
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
    };
    if (normalizedRecord.sourceType === 'manual') {
      manualRecords.push(normalizedRecord);
    } else {
      autoRecords.push(normalizedRecord);
    }
  });

  localStorage.setItem(STORAGE_KEY, JSON.stringify(autoRecords));
  localStorage.setItem(MANUAL_STORAGE_KEY, JSON.stringify(manualRecords));
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(KEY_MANAGEMENT_SYNC_EVENT));
  }
}

export function createManualDraft(record = null) {
  return {
    rowKey: record?.rowKey || '',
    sourceType: 'manual',
    siteName: record?.siteName || '',
    tokenName: record?.tokenName || '',
    siteUrl: record?.siteUrl || '',
    apiKey: record?.apiKey || '',
    modelsValue: record?.selectedModel || normalizeModels(record?.modelsList || record?.modelsText)[0] || '',
    status: Number(record?.status || 1),
  };
}

export function buildManualRecordFromDraft(draft, existingRecord = null) {
  const modelsList = normalizeModels([draft.modelsValue]);
  const now = Date.now();
  return {
    ...existingRecord,
    rowKey: existingRecord?.rowKey || draft.rowKey || buildManualRowKey(),
    sourceType: 'manual',
    siteName: String(draft.siteName || '').trim() || '未命名站点',
    tokenName: String(draft.tokenName || '').trim(),
    siteUrl: normalizeSiteUrl(draft.siteUrl),
    apiKey: normalizeApiKey(draft.apiKey),
    modelsList,
    modelsText: modelsList.join(', ') || '未提供模型信息',
    selectedModel: modelsList[0] || '',
    status: Number(draft.status || 1),
    createdAt: existingRecord?.createdAt || now,
    updatedAt: now,
    quickTestStatus: existingRecord?.quickTestStatus || '',
    quickTestLabel: existingRecord?.quickTestLabel || '',
    quickTestModel: existingRecord?.quickTestModel || '',
    quickTestRemark: existingRecord?.quickTestRemark || '',
    quickTestAt: existingRecord?.quickTestAt || null,
    quickTestResponseTime: existingRecord?.quickTestResponseTime || '',
    quickTestResponseContent: existingRecord?.quickTestResponseContent || '',
    balanceLabel: existingRecord?.balanceLabel || '',
    balanceUpdatedAt: existingRecord?.balanceUpdatedAt || null,
    balanceError: existingRecord?.balanceError || '',
    remainQuota: existingRecord?.remainQuota ?? null,
    usedQuota: existingRecord?.usedQuota ?? null,
    unlimitedQuota: existingRecord?.unlimitedQuota === true,
  };
}

export async function loadRecordModelOptions(record, contextMap, force = false) {
  if (!record?.siteUrl || !record?.apiKey) return record;
  const currentFetchKey = `${normalizeSiteUrl(record.siteUrl)}::${normalizeApiKey(record.apiKey)}`;
  if (!force && record.modelFetchKey === currentFetchKey && Array.isArray(record.modelsList) && record.modelsList.length > 0) {
    return record;
  }

  const modelResponse = await fetchModelList(record.siteUrl, record.apiKey);
  const rawCandidates = modelResponse?.data || modelResponse?.models || [];
  const normalizedCandidates = normalizeModels(rawCandidates);
  const context = getBatchHistoryContextByKeys(contextMap, record.siteUrl, record.apiKey);
  const mergedModels = normalizeModels([
    ...getContextModelNames(context),
    ...normalizedCandidates,
    ...(Array.isArray(record.modelsList) ? record.modelsList : []),
  ]);
  if (!mergedModels.length) {
    throw new Error('没有获取到可用模型');
  }
  return {
    ...record,
    modelsList: mergedModels,
    modelsText: mergedModels.join(', '),
    modelFetchKey: currentFetchKey,
    selectedModel: record.selectedModel && mergedModels.includes(record.selectedModel)
      ? record.selectedModel
      : (context?.preferredModel || pickPreferredModel(mergedModels) || mergedModels[0] || ''),
  };
}

function extractQuickTestResponseContent(messageObj) {
  const candidates = [
    normalizeQuickTestContent(messageObj?.content),
    normalizeQuickTestContent(messageObj?.reasoning_content),
    normalizeQuickTestContent(messageObj?.thinking),
  ].filter(Boolean);
  return candidates[0] || '';
}

function normalizeQuickTestContent(value) {
  if (!value) return '';
  let text = '';

  if (typeof value === 'string') {
    text = value;
  } else if (Array.isArray(value)) {
    text = value
      .map(item => {
        if (typeof item === 'string') return item;
        if (typeof item?.text === 'string') return item.text;
        if (typeof item?.content === 'string') return item.content;
        if (typeof item?.value === 'string') return item.value;
        return '';
      })
      .filter(Boolean)
      .join('\n');
  } else if (typeof value === 'object') {
    text = String(value?.text || value?.content || value?.value || '');
  }

  text = text.replace(/\s+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
  if (text.length > 500) {
    return `${text.slice(0, 500)}...`;
  }
  return text;
}

async function safeReadJson(response) {
  try {
    return await response.json();
  } catch (error) {
    console.warn('Failed to read JSON response', error);
    return null;
  }
}

async function safeReadResponsePayload(response) {
  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) return safeReadJson(response);
  const text = await response.text();
  const htmlTitle = text.match(/<title>(.*?)<\/title>/i)?.[1];
  return { message: htmlTitle || text.slice(0, 300) };
}

function extractReadableError(payload, statusCode) {
  if (!payload) return `HTTP ${statusCode}`;
  return payload?.error?.message || payload?.message || `HTTP ${statusCode}`;
}

function buildQuickTestDiagnosticText(payload, statusCode, requestMeta = {}) {
  const diagnostics = payload?.error?.diagnostics || payload?.diagnostics || null;
  const lines = [extractReadableError(payload, statusCode)];

  if (requestMeta?.siteUrl) {
    lines.push(`输入地址: ${requestMeta.siteUrl}`);
  }
  if (requestMeta?.model) {
    lines.push(`请求模型: ${requestMeta.model}`);
  }
  if (Number.isFinite(Number(requestMeta?.timeoutMs)) && Number(requestMeta.timeoutMs) > 0) {
    lines.push(`超时设置: ${Math.round(Number(requestMeta.timeoutMs) / 1000)}s`);
  }
  if (diagnostics?.resolvedEndpoint) {
    lines.push(`命中端点: ${diagnostics.resolvedEndpoint}`);
  }

  const attempts = Array.isArray(diagnostics?.attempts) ? diagnostics.attempts : [];
  if (attempts.length) {
    lines.push('尝试日志:');
    attempts.forEach((attempt, index) => {
      const status = Number(attempt?.status || 0);
      const endpoint = String(attempt?.endpoint || '').trim();
      const message = String(attempt?.message || '').trim();
      lines.push(`${index + 1}. [${status || '?'}] ${endpoint}${message ? ` -> ${message}` : ''}`);
    });
  }

  return lines.filter(Boolean).join('\n');
}

function createQuickTestError(payload, statusCode, requestMeta = {}) {
  const error = new Error(extractReadableError(payload, statusCode));
  error.detail = buildQuickTestDiagnosticText(payload, statusCode, requestMeta);
  return error;
}

export async function runRecordQuickTest(record, contextMap) {
  const nextRecord = { ...record };
  const model = await resolveQuickTestModel(nextRecord, contextMap);
  let timeoutMs = DEFAULT_TEST_TIMEOUT_MS;
  if (/^o1-|^o3-/i.test(model)) timeoutMs *= 3;
  const startedAt = Date.now();

  const response = await apiFetch('/api/check-key', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      url: normalizeSiteUrl(nextRecord.siteUrl),
      key: nextRecord.apiKey,
      model,
      messages: buildQuickTestMessages(),
      timeoutMs,
      _isFirst: false,
    }),
  });

  if (!response.ok) {
    const rawError = await safeReadResponsePayload(response);
    throw createQuickTestError(rawError, response.status, {
      siteUrl: normalizeSiteUrl(nextRecord.siteUrl),
      model,
      timeoutMs,
    });
  }

  let data = await response.json();
  if (data?.htmlSnippet) {
    const snippet = String(data.htmlSnippet).replace(/^data:\s*/, '').trim();
    if (snippet.startsWith('{') || snippet.startsWith('[')) {
      try {
        data = JSON.parse(snippet);
      } catch (error) {
        console.warn('Failed to parse htmlSnippet JSON payload', error);
      }
    }
  }

  const returnedModel = String(data?.model || 'unknown');
  const messageObj = data?.choices?.[0]?.message;
  const hasContent = Boolean(messageObj?.content || messageObj?.reasoning_content || messageObj?.thinking);
  const responseContent = extractQuickTestResponseContent(messageObj);
  const responseTime = ((Date.now() - startedAt) / 1000).toFixed(2);

  let status = 'warning';
  let label = '结构异常';
  let remark = '接口响应成功，但未检测到有效消息内容';
  if (returnedModel.toLowerCase().includes(model.toLowerCase()) || returnedModel === 'unknown') {
    if (hasContent) {
      status = returnedModel === 'unknown' ? 'warning' : 'success';
      label = returnedModel === 'unknown' ? '可用待确认' : '可用';
      remark = returnedModel === 'unknown' ? '接口有正常响应，但未返回模型标识' : '接口返回了有效对话内容';
    }
  } else {
    label = '模型映射';
    remark = `平台返回模型 ${returnedModel}，请求模型为 ${model}`;
  }

  return {
    ...nextRecord,
    quickTestStatus: status,
    quickTestLabel: label,
    quickTestModel: model,
    quickTestRemark: remark,
    quickTestAt: Date.now(),
    quickTestResponseTime: responseTime,
    quickTestResponseContent: responseContent,
  };
}

async function resolveQuickTestModel(record, contextMap) {
  const selectedModel = String(record?.selectedModel || '').trim();
  if (selectedModel) return selectedModel;
  const historyPreferred = getBatchHistoryContextByKeys(contextMap, record?.siteUrl, record?.apiKey)?.preferredModel || '';
  if (historyPreferred) {
    record.selectedModel = historyPreferred;
    return historyPreferred;
  }
  const fromRecord = pickPreferredModel(record.modelsList);
  if (fromRecord) {
    record.selectedModel = fromRecord;
    return fromRecord;
  }
  const nextRecord = await loadRecordModelOptions(record, contextMap, true);
  record.modelsList = nextRecord.modelsList;
  record.modelsText = nextRecord.modelsText;
  record.selectedModel = nextRecord.selectedModel;
  if (!record.selectedModel) {
    throw new Error('没有找到适合快速对话测试的模型');
  }
  return record.selectedModel;
}

export function canRefreshBalance(record, contextMap) {
  return Boolean(getBatchHistoryContextByKeys(contextMap, record?.siteUrl, record?.apiKey)?.accountData);
}

export async function refreshRecordBalance(record, contextMap) {
  const context = getBatchHistoryContextByKeys(contextMap, record?.siteUrl, record?.apiKey);
  if (!context?.accountData) {
    throw new Error('缺少批量检测上下文，无法复用余额刷新逻辑');
  }

  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site: context.accountData,
    siteUrl: normalizeSiteUrl(record.siteUrl),
  });
  if (!isDisplayableQuotaLabel(label)) {
    throw new Error('批量检测同款余额接口未返回可识别字段');
  }
  return {
    ...record,
    balanceLabel: label,
    balanceUpdatedAt: Date.now(),
    unlimitedQuota: /^无限/.test(String(label || '').trim()),
    balanceError: '',
  };
}

function normalizeBalanceLabel(value) {
  const text = String(value || '').trim();
  if (!text) return '';
  if (/^\$?-?\d/.test(text) || /USD$/i.test(text)) return text;
  if (/^无限/.test(text)) return text;
  return '';
}

function formatBalanceDisplay(value) {
  const text = String(value || '').trim();
  if (!text) return '';
  if (/^无限/.test(text)) return '无限';
  if (/USD$/i.test(text)) return text.replace(/\s+/g, ' ');
  if (text.startsWith('$')) return `${text.slice(1)} USD`;
  return text;
}

export function getRecordBalanceValue(record) {
  const directLabel = normalizeBalanceLabel(record?.balanceLabel);
  if (directLabel) return formatBalanceDisplay(directLabel);
  if (record?.unlimitedQuota) return '无限';
  const remainQuota = Number(record?.remainQuota);
  if (Number.isFinite(remainQuota)) {
    return `${remainQuota.toFixed(2)} USD`;
  }
  return '';
}

export function getBalanceRelativeTime(record) {
  if (record?.balanceLoading) return '刷新中';
  const timestamp = Number(record?.balanceUpdatedAt || 0);
  if (!timestamp) return '未刷新';
  const diffMs = Math.max(0, Date.now() - timestamp);
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (diffMs < minute) return '刚刚';
  if (diffMs < hour) return `${Math.floor(diffMs / minute)} 分钟前`;
  if (diffMs < day) return `${Math.floor(diffMs / hour)} 小时前`;
  return `${Math.floor(diffMs / day)} 天前`;
}

export function getQuickTestTone(status) {
  if (status === 'success') return 'success';
  if (status === 'warning') return 'warning';
  if (status === 'error') return 'error';
  return 'idle';
}

export function formatDateTime(timestamp) {
  if (!timestamp) return '未同步';
  try {
    return new Date(timestamp).toLocaleString();
  } catch (error) {
    console.warn('Failed to format timestamp', error);
    return '时间异常';
  }
}
