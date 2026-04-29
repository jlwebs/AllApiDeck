<template>
  <div class="ai-image-view">
    <div class="ai-image-shell">
      <div class="ai-image-header">
        <div class="ai-image-header-copy">
          <div class="ai-image-kicker">AI Image Lab</div>
          <div class="ai-image-title">AI 绘图</div>
          <div class="ai-image-subtitle">
            {{ targetRecord ? `${targetRecord.siteName} · ${maskedRecordKeyLabel}` : '当前窗口需要从密钥管理记录中启动。' }}
          </div>
        </div>

        <div class="ai-image-header-actions">
          <a-button type="text" size="small" class="ai-image-close-button" @click.stop="closeWindow">关闭</a-button>
        </div>
      </div>

      <a-empty
        v-if="!targetRecord"
        description="未找到目标密钥记录，无法发起 AI 绘图。"
      />

      <template v-else>
        <div class="ai-image-layout">
          <section class="ai-image-form-panel">
            <div class="ai-image-panel-head">
              <div>
                <div class="ai-image-panel-title">绘图配置</div>
                <div class="ai-image-panel-hint">沿用当前密钥上下文，可直接微调接口地址、模型、尺寸与提示词。</div>
              </div>
              <div class="ai-image-record-pills">
                <span class="ai-image-record-pill">{{ targetRecord.siteName }}</span>
                <span class="ai-image-record-pill">{{ maskedRecordKeyLabel }}</span>
                <span class="ai-image-record-pill">{{ targetRecord.siteUrl }}</span>
              </div>
            </div>

            <a-form layout="vertical" class="ai-image-form">
              <div class="ai-image-form-grid">
                <a-form-item label="Base URL">
                  <a-input v-model:value="draft.baseUrl" placeholder="https://api.openai.com/v1" />
                </a-form-item>
                <a-form-item label="Model">
                  <a-select
                    v-model:value="draft.model"
                    :options="modelOptions"
                    :loading="modelLoading"
                    show-search
                    :filter-option="true"
                    option-filter-prop="label"
                    placeholder="优先选择 gpt-5.3-codex，没有时请手动选择"
                  />
                </a-form-item>
              </div>

              <div v-if="modelSelectionHint" class="ai-image-model-hint">{{ modelSelectionHint }}</div>

              <a-form-item label="API Key">
                <a-input-password v-model:value="draft.apiKey" placeholder="sk-..." />
              </a-form-item>

              <a-form-item label="图片尺寸">
                <div class="ai-image-size-strip">
                  <button
                    v-for="item in SIZE_OPTIONS"
                    :key="item.value"
                    type="button"
                    class="ai-image-size-option"
                    :class="{ 'ai-image-size-option-active': draft.size === item.value }"
                    @click="draft.size = item.value"
                  >
                    {{ item.label }}
                  </button>
                </div>
                <div class="ai-image-model-hint">当前接口只走 `/v1/responses` + `image_generation`，尺寸/比例不会单独传参，会自动追加到提示词里。</div>
              </a-form-item>

              <a-form-item label="提示词">
                <a-textarea
                  v-model:value="draft.prompt"
                  :auto-size="{ minRows: 5, maxRows: 10 }"
                  placeholder="描述你想生成的图片内容、风格、构图、比例与细节要求。"
                />
              </a-form-item>

              <div class="ai-image-submit-row">
                <a-button
                  type="primary"
                  size="large"
                  class="ai-image-generate-button"
                  :loading="generating"
                  @click="generateImage"
                >
                  发起 AI 绘图
                </a-button>
                <span class="ai-image-submit-hint">调用 `responses` + `image_generation` 工具链，结果仅保存在本地窗口历史中。</span>
              </div>
            </a-form>
          </section>

          <section class="ai-image-result-panel">
            <div class="ai-image-panel-title">当前结果</div>
            <div v-if="generating" class="ai-image-state ai-image-state-loading">
              <a-spin size="large" />
              <div class="ai-image-state-title">正在生成图片</div>
              <div class="ai-image-state-desc">请求已发出，通常需要十几秒到几十秒。</div>
            </div>

            <div v-else-if="errorMessage" class="ai-image-state ai-image-state-error">
              <div class="ai-image-state-title">绘图失败</div>
              <div class="ai-image-error-box">{{ errorMessage }}</div>
            </div>

            <div v-else-if="activeItem" class="ai-image-result-ready">
              <button type="button" class="ai-image-preview-card" @click="openPreview(activeItem)">
                <img :src="toDataUrl(activeItem.base64)" :alt="activeItem.prompt" class="ai-image-preview-image" />
              </button>
              <div class="ai-image-result-meta">
                <div class="ai-image-result-meta-line">模型：{{ activeItem.model || '未设置' }}</div>
                <div class="ai-image-result-meta-line">尺寸：{{ activeItem.size || 'auto' }}</div>
                <div class="ai-image-result-meta-line">时间：{{ formatDateTime(activeItem.timestamp) }}</div>
              </div>
              <div class="ai-image-result-prompt">{{ activeItem.prompt }}</div>
              <div class="ai-image-result-actions">
                <a-button @click="copyPrompt(activeItem)">复制提示词</a-button>
                <a-button @click="downloadImage(activeItem)">下载图片</a-button>
                <a-button type="primary" ghost @click="openPreview(activeItem)">查看大图</a-button>
              </div>
            </div>

            <div v-else class="ai-image-state ai-image-state-empty">
              <div class="ai-image-empty-icon">绘</div>
              <div class="ai-image-state-title">还没有图片结果</div>
              <div class="ai-image-state-desc">填写提示词后发起绘图，首张结果会显示在这里。</div>
            </div>
          </section>
        </div>

        <section class="ai-image-history-panel">
          <div class="ai-image-history-head">
            <div>
              <div class="ai-image-panel-title">本地历史</div>
              <div class="ai-image-panel-hint">仅保留当前 key 的绘图历史，互不串库。</div>
            </div>
            <a-button danger :disabled="historyItems.length === 0" @click="confirmClearHistory">清空历史</a-button>
          </div>

          <a-empty v-if="historyItems.length === 0" description="当前 key 暂无绘图历史。" />

          <div v-else class="ai-image-history-grid">
            <button
              v-for="item in historyItems"
              :key="item.id"
              type="button"
              class="ai-image-history-card"
              :class="{ 'ai-image-history-card-active': activeItem?.id === item.id }"
              @click="selectHistoryItem(item)"
            >
              <img :src="toDataUrl(item.base64)" :alt="item.prompt" class="ai-image-history-thumb" />
              <div class="ai-image-history-overlay">
                <div class="ai-image-history-time">{{ formatCompactDateTime(item.timestamp) }}</div>
                <button
                  type="button"
                  class="ai-image-history-delete"
                  @click.stop="confirmDeleteHistoryItem(item)"
                >
                  删除
                </button>
              </div>
            </button>
          </div>
        </section>
      </template>
    </div>

    <a-modal
      v-model:open="previewOpen"
      title="图片预览"
      :footer="null"
      width="min(92vw, 1200px)"
      wrap-class-name="ai-image-preview-modal"
    >
      <div v-if="previewItem" class="ai-image-preview-modal-body">
        <img :src="toDataUrl(previewItem.base64)" :alt="previewItem.prompt" class="ai-image-preview-modal-image" />
        <div class="ai-image-preview-modal-prompt">{{ previewItem.prompt }}</div>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue';
import { message, Modal } from 'ant-design-vue';
import { useRoute } from 'vue-router';
import { WindowSetTitle } from '../../wailsjs/runtime/runtime.js';
import { fetchModelList } from '../utils/api.js';
import { loadPanelRecords } from '../utils/keyPanelStore.js';
import { hydrateLastResultsSnapshotCache } from '../utils/historySnapshotStore.js';
import { maskApiKey } from '../utils/normal.js';
import { apiFetch, isProbablyWailsRuntime } from '../utils/runtimeApi.js';

function getWailsAppBridge() {
  return window?.go?.main?.App || null;
}

async function requestQuitSafely() {
  const bridge = getWailsAppBridge();
  if (typeof bridge?.RequestQuit !== 'function') {
    return false;
  }
  await bridge.RequestQuit();
  return true;
}

async function getLaunchRecordKeySafely() {
  const bridge = getWailsAppBridge();
  if (typeof bridge?.GetLaunchRecordKey !== 'function') {
    return '';
  }
  return String(await bridge.GetLaunchRecordKey() || '').trim();
}

const SIZE_PROMPT_HINTS = {
  '1024x1024': '请直接生成一张图片，不要输出文字说明。画面比例为 1:1，方图构图。',
  '1024x1536': '请直接生成一张图片，不要输出文字说明。画面比例为 2:3，竖图构图。',
  '1536x1024': '请直接生成一张图片，不要输出文字说明。画面比例为 3:2，横图构图。',
  auto: '请直接生成一张图片，不要输出文字说明。',
};

const SIZE_OPTIONS = [
  { label: '1024×1024', value: '1024x1024' },
  { label: '1024×1536', value: '1024x1536' },
  { label: '1536×1024', value: '1536x1024' },
  { label: 'Auto', value: 'auto' },
];

const IMAGE_HISTORY_DB_NAME = 'AllApiDeckAIImageHistory';
const IMAGE_HISTORY_DB_VERSION = 2;
const IMAGE_HISTORY_STORE = 'images';
const IMAGE_HISTORY_SCOPE_INDEX = 'scopeKey';
const IMAGE_REQUEST_TIMEOUT_MS = 180000;
const IMAGE_WINDOW_SETTINGS_PREFIX = 'all_api_deck_ai_image_window_settings_v1';

const IMAGE_MODEL_PATTERN = /(gpt-image|image|flux|playground|midjourney|mj|dall|sdxl|stability|recraft|ideogram|kolors)/i;

const route = useRoute();
const records = ref([]);
const targetRecord = ref(null);
const scopeKey = ref('');
const generating = ref(false);
const errorMessage = ref('');
const historyItems = ref([]);
const activeItemId = ref(null);
const previewOpen = ref(false);
const previewItem = ref(null);
const modelOptions = ref([]);
const modelLoading = ref(false);
const draft = reactive({
  baseUrl: '',
  apiKey: '',
  model: '',
  prompt: '',
  size: '1024x1536',
});

const maskedRecordKeyLabel = computed(() => maskApiKey(String(targetRecord.value?.apiKey || '').trim()) || '未命名 Key');

const activeItem = computed(() => {
  if (!historyItems.value.length) return null;
  return historyItems.value.find(item => item.id === activeItemId.value) || historyItems.value[0] || null;
});

const hasPreferredModel = computed(() => modelOptions.value.some(item => item.value === 'gpt-5.3-codex'));

const modelSelectionHint = computed(() => {
  if (modelLoading.value) return '正在加载模型列表...';
  if (!modelOptions.value.length) return '暂未拿到模型列表，请确认 Base URL / API Key 是否正确。';
  if (hasPreferredModel.value) return '';
  if (!draft.model) return '当前接口没有 gpt-5.3-codex，请手动选择一个 OpenAI 模型后再发起绘图。';
  return '';
});

function buildImagePrompt(prompt, size) {
  const normalizedPrompt = String(prompt || '').trim();
  const hint = SIZE_PROMPT_HINTS[size] || SIZE_PROMPT_HINTS.auto;
  if (!normalizedPrompt) {
    return hint;
  }
  return `${normalizedPrompt}\n\n${hint}`;
}

const windowTitle = computed(() => {
  if (!targetRecord.value) return 'AI 绘图';
  const siteName = String(targetRecord.value.siteName || '未命名站点').trim();
  const maskedKey = maskedRecordKeyLabel.value;
  return `AI绘图-${maskedKey}-${siteName}`;
});

function normalizeBaseUrl(input) {
  return String(input || '')
    .trim()
    .replace(/\/+$/, '')
    .replace(/\/(chat\/completions|completions|responses|images\/generations|embeddings)$/i, '');
}

function stripKnownApiSuffix(input) {
  const patterns = [
    /\/v\d+\/chat\/completions$/i,
    /\/chat\/completions$/i,
    /\/v\d+\/completions$/i,
    /\/completions$/i,
    /\/v\d+\/responses$/i,
    /\/responses$/i,
    /\/v\d+\/images\/generations$/i,
    /\/images\/generations$/i,
    /\/v\d+\/embeddings$/i,
    /\/embeddings$/i,
    /\/api\/v\d+$/i,
    /\/api$/i,
  ];

  for (const pattern of patterns) {
    if (pattern.test(input)) {
      return input.replace(pattern, '');
    }
  }

  return input;
}

function buildResponsesEndpointCandidates(input) {
  const normalizedInput = normalizeBaseUrl(input);
  if (!normalizedInput) return [];

  if (/\/v\d+$/i.test(normalizedInput)) {
    return [`${normalizedInput}/responses`];
  }

  if (/\/v\d+\/responses$/i.test(normalizedInput)) {
    return [normalizedInput];
  }

  const stripped = stripKnownApiSuffix(normalizedInput);
  if (/\/v\d+$/i.test(stripped)) {
    return [`${stripped}/responses`];
  }

  return [`${stripped}/v1/responses`];
}

function buildScopeSettingsKey() {
  return `${IMAGE_WINDOW_SETTINGS_PREFIX}:${scopeKey.value || 'global'}`;
}

function isOpenAIFamilyModel(model) {
  return /^(gpt|chatgpt|o1|o3|o4|dall)/i.test(String(model || '').trim());
}

function normalizeModelCandidates(record, extraModels = []) {
  const combined = [
    ...(Array.isArray(record?.modelsList) ? record.modelsList : []),
    record?.selectedModel || '',
    record?.quickTestModel || '',
    ...extraModels,
  ];

  return Array.from(new Set(
    combined
      .map(item => typeof item === 'string' ? item : (item?.id || item?.model || item?.name || ''))
      .map(item => String(item || '').trim())
      .filter(Boolean)
  ));
}

function buildModelOptions(record, extraModels = []) {
  const normalized = normalizeModelCandidates(record, extraModels);
  const preferred = normalized.filter(model => model === 'gpt-5.3-codex');
  const openaiModels = normalized.filter(model => model !== 'gpt-5.3-codex' && isOpenAIFamilyModel(model));
  const others = normalized.filter(model => model !== 'gpt-5.3-codex' && !isOpenAIFamilyModel(model));
  return [...preferred, ...openaiModels, ...others].map(model => ({
    label: model,
    value: model,
  }));
}

function resolveInitialSelectedModel(savedModel = '') {
  if (hasPreferredModel.value) return 'gpt-5.3-codex';
  const normalizedSaved = String(savedModel || '').trim();
  if (normalizedSaved && modelOptions.value.some(item => item.value === normalizedSaved) && isOpenAIFamilyModel(normalizedSaved)) {
    return normalizedSaved;
  }
  return '';
}

function buildDefaultPrompt(record) {
  const siteName = String(record?.siteName || '').trim();
  if (!siteName) return '';
  return `围绕“${siteName}”生成一张高质量视觉图，保持清晰主体、完整构图与可直接使用的成片质感。`;
}

function loadPersistedSettings() {
  try {
    return JSON.parse(localStorage.getItem(buildScopeSettingsKey()) || '{}');
  } catch {
    return {};
  }
}

function persistSettings() {
  if (!scopeKey.value) return;
  try {
    localStorage.setItem(buildScopeSettingsKey(), JSON.stringify({
      baseUrl: draft.baseUrl,
      apiKey: draft.apiKey,
      model: draft.model,
      size: draft.size,
      prompt: draft.prompt,
    }));
  } catch {}
}

function buildInitialDraft(record) {
  const saved = loadPersistedSettings();
  draft.baseUrl = String(saved.baseUrl || normalizeBaseUrl(record?.siteUrl || '')).trim();
  draft.apiKey = String(saved.apiKey || record?.apiKey || '').trim();
  draft.model = String(saved.model || '').trim();
  draft.prompt = String(saved.prompt || buildDefaultPrompt(record)).trim();
  draft.size = String(saved.size || '1024x1536').trim() || '1024x1536';
}

async function refreshModelOptions(record = targetRecord.value) {
  if (!record) {
    modelOptions.value = [];
    draft.model = '';
    return;
  }

  modelLoading.value = true;
  try {
    let fetchedModels = [];
    try {
      const response = await fetchModelList(draft.baseUrl || record.siteUrl, draft.apiKey || record.apiKey);
      const candidates = Array.isArray(response?.data) ? response.data : [];
      fetchedModels = candidates.map(item => item?.id || item?.model || item?.name || '').filter(Boolean);
    } catch {}

    modelOptions.value = buildModelOptions(record, fetchedModels);
    draft.model = resolveInitialSelectedModel(draft.model);
  } finally {
    modelLoading.value = false;
  }
}

function formatDateTime(value) {
  const timestamp = Number(value || 0);
  if (!timestamp) return '未知时间';
  return new Date(timestamp).toLocaleString('zh-CN');
}

function formatCompactDateTime(value) {
  const timestamp = Number(value || 0);
  if (!timestamp) return '--';
  return new Date(timestamp).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function toDataUrl(base64) {
  return `data:image/png;base64,${String(base64 || '')}`;
}

async function closeWindow() {
  if (isProbablyWailsRuntime()) {
    try {
      if (await requestQuitSafely()) {
        return;
      }
    } catch (error) {
      message.error(error?.message || '关闭窗口失败');
    }
  }
  window.close();
  if (typeof window !== 'undefined' && !window.closed) {
    window.history.back();
  }
}

function selectHistoryItem(item) {
  activeItemId.value = item?.id ?? null;
  errorMessage.value = '';
}

function openPreview(item) {
  previewItem.value = item || null;
  previewOpen.value = Boolean(item);
}

function updateWindowTitle() {
  const nextTitle = windowTitle.value;
  if (typeof document !== 'undefined') {
    document.title = nextTitle;
  }
  if (isProbablyWailsRuntime()) {
    try {
      WindowSetTitle(nextTitle);
    } catch {}
  }
}

function openHistoryDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(IMAGE_HISTORY_DB_NAME, IMAGE_HISTORY_DB_VERSION);
    request.onupgradeneeded = event => {
      const db = event.target.result;
      const store = db.objectStoreNames.contains(IMAGE_HISTORY_STORE)
        ? request.transaction.objectStore(IMAGE_HISTORY_STORE)
        : db.createObjectStore(IMAGE_HISTORY_STORE, { keyPath: 'id', autoIncrement: true });
      if (!store.indexNames.contains(IMAGE_HISTORY_SCOPE_INDEX)) {
        store.createIndex(IMAGE_HISTORY_SCOPE_INDEX, IMAGE_HISTORY_SCOPE_INDEX, { unique: false });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('open indexeddb failed'));
  });
}

async function loadHistoryList() {
  if (!scopeKey.value) {
    historyItems.value = [];
    activeItemId.value = null;
    return;
  }
  try {
    const db = await openHistoryDB();
    const items = await new Promise((resolve, reject) => {
      const tx = db.transaction(IMAGE_HISTORY_STORE, 'readonly');
      const store = tx.objectStore(IMAGE_HISTORY_STORE);
      const index = store.index(IMAGE_HISTORY_SCOPE_INDEX);
      const request = index.getAll(scopeKey.value);
      request.onsuccess = () => resolve(request.result || []);
      request.onerror = () => reject(request.error || new Error('load history failed'));
    });
    const sorted = (Array.isArray(items) ? items : [])
      .sort((left, right) => Number(right?.timestamp || 0) - Number(left?.timestamp || 0));
    historyItems.value = sorted;
    if (!sorted.length) {
      activeItemId.value = null;
      return;
    }
    const existing = sorted.find(item => item.id === activeItemId.value);
    activeItemId.value = existing?.id ?? sorted[0].id;
  } catch (error) {
    console.warn(error);
  }
}

async function saveHistoryItem(item) {
  const db = await openHistoryDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IMAGE_HISTORY_STORE, 'readwrite');
    const store = tx.objectStore(IMAGE_HISTORY_STORE);
    const request = store.add(item);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('save history failed'));
  });
}

async function deleteHistoryItem(itemId) {
  const db = await openHistoryDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IMAGE_HISTORY_STORE, 'readwrite');
    const store = tx.objectStore(IMAGE_HISTORY_STORE);
    const request = store.delete(itemId);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error || new Error('delete history failed'));
  });
}

async function clearHistoryByScope() {
  const db = await openHistoryDB();
  const items = historyItems.value.map(item => item.id).filter(id => Number.isFinite(id));
  await Promise.all(items.map(id => deleteHistoryItem(id)));
  await loadHistoryList();
}

function buildErrorMessage(status, payloadText) {
  const trimmed = String(payloadText || '').trim();
  if (!trimmed) return `HTTP ${status}`;
  try {
    const parsed = JSON.parse(trimmed);
    return parsed?.error?.message || parsed?.message || trimmed.slice(0, 400);
  } catch {
    return `HTTP ${status}: ${trimmed.slice(0, 400)}`;
  }
}

function extractPayloadErrorDetails(payload) {
  const lines = [];
  const actualModel = String(payload?.model || '').trim();
  const status = String(payload?.status || '').trim();
  const topLevelError = payload?.error?.message || payload?.error || '';
  const outputs = Array.isArray(payload?.output) ? payload.output : [];
  const outputTexts = outputs
    .flatMap(item => Array.isArray(item?.content) ? item.content : [])
    .filter(item => item?.type === 'output_text')
    .map(item => String(item?.text || '').trim())
    .filter(Boolean);

  if (actualModel) lines.push(`实际响应模型：${actualModel}`);
  if (status) lines.push(`响应状态：${status}`);
  if (topLevelError) lines.push(`原始错误：${String(topLevelError).trim()}`);
  if (outputTexts.length) {
    lines.push(`原始输出：${outputTexts.join('\n').slice(0, 1200)}`);
  } else if (outputs.length) {
    lines.push(`原始输出片段：${JSON.stringify(outputs).slice(0, 1200)}`);
  } else {
    lines.push(`原始响应：${JSON.stringify(payload).slice(0, 1200)}`);
  }

  return lines.join('\n');
}

function extractDetailedPayloadErrorDetails(payload) {
  const lines = [];
  const objectType = String(payload?.object || '').trim();
  const actualModel = String(payload?.model || '').trim();
  const status = String(payload?.status || '').trim();
  const topLevelError = payload?.error?.message || payload?.error || '';
  const outputs = Array.isArray(payload?.output) ? payload.output : [];
  const outputTexts = outputs
    .flatMap(item => Array.isArray(item?.content) ? item.content : [])
    .filter(item => item?.type === 'output_text')
    .map(item => String(item?.text || '').trim())
    .filter(Boolean);
  const choiceTexts = Array.isArray(payload?.choices)
    ? payload.choices
      .map(choice => {
        const content = choice?.message?.content;
        if (typeof content === 'string') {
          return content.trim();
        }
        if (Array.isArray(content)) {
          return content
            .map(item => {
              if (typeof item === 'string') return item.trim();
              if (item?.type === 'text' || item?.type === 'output_text') {
                return String(item?.text || '').trim();
              }
              return '';
            })
            .filter(Boolean)
            .join('\n');
        }
        return '';
      })
      .filter(Boolean)
    : [];

  if (objectType) lines.push(`\u5b9e\u9645\u54cd\u5e94\u5bf9\u8c61\uff1a${objectType}`);
  if (actualModel) lines.push(`\u5b9e\u9645\u54cd\u5e94\u6a21\u578b\uff1a${actualModel}`);
  if (status) lines.push(`\u54cd\u5e94\u72b6\u6001\uff1a${status}`);
  if (topLevelError) lines.push(`\u539f\u59cb\u9519\u8bef\uff1a${String(topLevelError).trim()}`);
  if (outputTexts.length) {
    lines.push(`\u539f\u59cb\u8f93\u51fa\uff1a${outputTexts.join('\n').slice(0, 1200)}`);
  } else if (choiceTexts.length) {
    lines.push(`\u539f\u59cb choices \u6587\u672c\uff1a${choiceTexts.join('\n').slice(0, 1200)}`);
  } else if (outputs.length) {
    lines.push(`\u539f\u59cb\u8f93\u51fa\u7247\u6bb5\uff1a${JSON.stringify(outputs).slice(0, 1200)}`);
  } else {
    lines.push(`\u539f\u59cb\u54cd\u5e94\uff1a${JSON.stringify(payload).slice(0, 1200)}`);
  }

  return lines.join('\n');
}

function buildReadablePayloadErrorDetails(payload) {
  const lines = [];
  const objectType = String(payload?.object || '').trim();
  const actualModel = String(payload?.model || '').trim();
  const status = String(payload?.status || '').trim();
  const topLevelError = payload?.error?.message || payload?.error || '';
  const outputs = Array.isArray(payload?.output) ? payload.output : [];
  const outputTexts = outputs
    .flatMap(item => Array.isArray(item?.content) ? item.content : [])
    .filter(item => item?.type === 'output_text')
    .map(item => String(item?.text || '').trim())
    .filter(Boolean);
  const choiceTexts = Array.isArray(payload?.choices)
    ? payload.choices
      .map(choice => {
        const content = choice?.message?.content;
        if (typeof content === 'string') return content.trim();
        if (!Array.isArray(content)) return '';
        return content
          .map(item => {
            if (typeof item === 'string') return item.trim();
            if (item?.type === 'text' || item?.type === 'output_text') {
              return String(item?.text || '').trim();
            }
            return '';
          })
          .filter(Boolean)
          .join('\n');
      })
      .filter(Boolean)
    : [];

  if (objectType) lines.push(`\u5b9e\u9645\u54cd\u5e94\u5bf9\u8c61\uff1a${objectType}`);
  if (actualModel) lines.push(`\u5b9e\u9645\u54cd\u5e94\u6a21\u578b\uff1a${actualModel}`);
  if (status) lines.push(`\u54cd\u5e94\u72b6\u6001\uff1a${status}`);
  if (topLevelError) lines.push(`\u539f\u59cb\u9519\u8bef\uff1a${String(topLevelError).trim()}`);
  if (outputTexts.length) {
    lines.push(`\u539f\u59cb\u8f93\u51fa\uff1a${outputTexts.join('\n').slice(0, 1200)}`);
  } else if (choiceTexts.length) {
    lines.push(`\u539f\u59cb choices \u6587\u672c\uff1a${choiceTexts.join('\n').slice(0, 1200)}`);
  } else if (outputs.length) {
    lines.push(`\u539f\u59cb\u8f93\u51fa\u7247\u6bb5\uff1a${JSON.stringify(outputs).slice(0, 1200)}`);
  } else {
    lines.push(`\u539f\u59cb\u54cd\u5e94\uff1a${JSON.stringify(payload).slice(0, 1200)}`);
  }

  return lines.join('\n');
}

async function generateImage() {
  const baseUrl = normalizeBaseUrl(draft.baseUrl);
  const apiKey = String(draft.apiKey || '').trim();
  const model = String(draft.model || '').trim();
  const prompt = String(draft.prompt || '').trim();
  if (!baseUrl) {
    errorMessage.value = '请输入 Base URL';
    return;
  }
  if (!apiKey) {
    errorMessage.value = '请输入 API Key';
    return;
  }
  if (!prompt) {
    errorMessage.value = '请输入提示词';
    return;
  }
  if (!model) {
    errorMessage.value = '请先从下拉列表选择一个绘图模型，再发起 AI 绘图';
    return;
  }

  generating.value = true;
  errorMessage.value = '';

  try {
    const finalPrompt = buildImagePrompt(prompt, draft.size);
    const requestBody = {
      model,
      input: finalPrompt,
      tools: [
        {
          type: 'image_generation',
        },
      ],
      tool_choice: 'auto',
    };
    const endpoints = buildResponsesEndpointCandidates(baseUrl);
    const errors = [];
    let payload = null;
    let usedEndpoint = '';

    for (const endpoint of endpoints) {
      try {
        const response = await apiFetch(endpoint, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${apiKey}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(requestBody),
          timeoutMs: IMAGE_REQUEST_TIMEOUT_MS,
        });

        const responseText = await response.text().catch(() => '');
        if (!response.ok) {
          const detail = buildErrorMessage(response.status, responseText);
          errors.push(`${endpoint} -> ${detail}`);
          continue;
        }

        try {
          payload = JSON.parse(responseText || 'null');
        } catch {
          errors.push(`${endpoint} -> \u54cd\u5e94\u4e0d\u662f\u5408\u6cd5 JSON\uff1a${responseText.slice(0, 1200)}`);
          continue;
        }

        usedEndpoint = endpoint;
        break;
      } catch (error) {
        errors.push(`${endpoint} -> ${error?.message || '\u8bf7\u6c42\u5931\u8d25'}`);
      }
    }

    if (!payload) {
      throw new Error(errors.join('\n\n') || '\u7ed8\u56fe\u63a5\u53e3\u8bf7\u6c42\u5931\u8d25');
    }

    const outputs = Array.isArray(payload?.output) ? payload.output : [];
    const imageCall = outputs.find(item => item?.type === 'image_generation_call' && String(item?.result || '').trim());
    const base64 = String(imageCall?.result || '').trim();
    if (!base64) {
      const details = buildReadablePayloadErrorDetails(payload);
      const endpointLine = usedEndpoint ? `\u547d\u4e2d\u7aef\u70b9\uff1a${usedEndpoint}\n` : '';
      throw new Error(`Free\u53f7\u6c60\u4e0d\u5177\u5907\u8c03\u7528 image_generation \u5de5\u5177\u80fd\u529b\uff0c\u8bf7\u66f4\u6362\u4e2d\u8f6c\u7ad9\u7ed8\u56fe\uff01\n${endpointLine}\u8bf7\u6c42\u6a21\u578b\uff1a${model}\n${details}`);
    }
    const nextItem = {
      scopeKey: scopeKey.value,
      siteName: String(targetRecord.value?.siteName || '').trim(),
      siteUrl: String(targetRecord.value?.siteUrl || '').trim(),
      model,
      size: draft.size,
      prompt,
      base64,
      timestamp: Date.now(),
    };

    const savedId = await saveHistoryItem(nextItem);
    nextItem.id = savedId;
    await loadHistoryList();
    activeItemId.value = nextItem.id;
    persistSettings();
    message.success('AI 绘图已完成');
  } catch (error) {
    errorMessage.value = error?.message || '绘图请求失败';
  } finally {
    generating.value = false;
  }
}

async function copyPrompt(item) {
  try {
    await navigator.clipboard.writeText(String(item?.prompt || ''));
    message.success('提示词已复制');
  } catch (error) {
    message.error(error?.message || '复制失败');
  }
}

function downloadImage(item) {
  const anchor = document.createElement('a');
  anchor.href = toDataUrl(item?.base64);
  anchor.download = `ai-image-${new Date(Number(item?.timestamp || Date.now())).toISOString().slice(0, 19).replace(/[:T]/g, '-')}.png`;
  anchor.click();
}

function confirmDeleteHistoryItem(item) {
  Modal.confirm({
    title: '确认删除这张图片？',
    okText: '删除',
    cancelText: '取消',
    okButtonProps: { danger: true },
    async onOk() {
      await deleteHistoryItem(item.id);
      await loadHistoryList();
      if (activeItemId.value === item.id) {
        activeItemId.value = historyItems.value[0]?.id ?? null;
      }
    },
  });
}

function confirmClearHistory() {
  Modal.confirm({
    title: '确认清空当前 key 的绘图历史？',
    content: '只会删除当前密钥对应的图片历史，不影响其他 key。',
    okText: '清空',
    cancelText: '取消',
    okButtonProps: { danger: true },
    async onOk() {
      await clearHistoryByScope();
      message.success('当前 key 的绘图历史已清空');
    },
  });
}

async function bootstrap() {
  await hydrateLastResultsSnapshotCache();
  let launchedRowKey = '';
  if (isProbablyWailsRuntime()) {
    try {
      launchedRowKey = await getLaunchRecordKeySafely();
    } catch {
      launchedRowKey = '';
    }
  }
  const fallbackRowKey = String(route.query.rowKey || '').trim();
  scopeKey.value = String(launchedRowKey || fallbackRowKey || '').trim();
  const loaded = loadPanelRecords();
  records.value = loaded.records || [];
  targetRecord.value = records.value.find(item => String(item?.rowKey || '').trim() === scopeKey.value) || null;
  if (targetRecord.value) {
    buildInitialDraft(targetRecord.value);
    await refreshModelOptions(targetRecord.value);
    await loadHistoryList();
  }
  updateWindowTitle();
}

watch(
  () => [draft.baseUrl, draft.apiKey, draft.model, draft.prompt, draft.size],
  () => {
    persistSettings();
  },
  { deep: false }
);

watch(windowTitle, () => {
  updateWindowTitle();
});

onMounted(() => {
  void bootstrap();
});
</script>

<style scoped>
.ai-image-view{
  width:100%;
  min-height:100vh;
  box-sizing:border-box;
  padding:10px;
  display:flex;
  background:
    radial-gradient(circle at 14% 12%,rgba(190,224,183,.78),transparent 24%),
    radial-gradient(circle at 86% 10%,rgba(179,210,242,.58),transparent 22%),
    linear-gradient(180deg,#edf4e7,#dfe8da);
}
.ai-image-shell{
  width:min(100%,1180px);
  min-height:calc(100vh - 20px);
  margin:0 auto;
  padding:14px 16px 16px;
  border-radius:26px;
  background:linear-gradient(180deg,rgba(255,255,255,.98),rgba(247,250,246,.95));
  box-shadow:0 22px 56px rgba(61,83,48,.14),inset 0 1px 0 rgba(255,255,255,.74);
  backdrop-filter:blur(14px) saturate(108%);
  display:flex;
  flex-direction:column;
  gap:14px;
  overflow:hidden;
}
.ai-image-header{
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:14px;
  -webkit-app-region:drag;
}
.ai-image-header-actions{
  -webkit-app-region:no-drag;
}
.ai-image-close-button{
  -webkit-app-region:no-drag;
  border-radius:12px;
  padding-inline:12px;
}
.ai-image-close-button:hover{
  background:rgba(15,23,42,.06) !important;
}
.ai-image-kicker{
  margin-bottom:2px;
  color:#6f8f55;
  font-size:11px;
  font-weight:800;
  letter-spacing:.12em;
  text-transform:uppercase;
}
.ai-image-title{
  color:#243329;
  font:800 29px/1.06 Georgia,'Times New Roman',serif;
}
.ai-image-subtitle{
  margin-top:4px;
  color:#6d7d67;
  font-size:12px;
  line-height:1.45;
}
.ai-image-layout{
  display:grid;
  grid-template-columns:minmax(0,1.05fr) minmax(320px,.95fr);
  gap:14px;
  min-height:0;
}
.ai-image-form-panel,.ai-image-result-panel,.ai-image-history-panel{
  min-width:0;
  border:1px solid rgba(102,130,93,.12);
  border-radius:22px;
  background:linear-gradient(180deg,rgba(252,254,251,.98),rgba(243,248,239,.94));
  box-shadow:inset 0 1px 0 rgba(255,255,255,.72);
}
.ai-image-form-panel,.ai-image-result-panel{
  padding:14px 16px 16px;
}
.ai-image-history-panel{
  padding:14px 16px 16px;
}
.ai-image-panel-head,.ai-image-history-head{
  display:flex;
  align-items:flex-start;
  justify-content:space-between;
  gap:14px;
  margin-bottom:10px;
}
.ai-image-panel-title{
  color:#25342a;
  font:700 17px/1.1 Georgia,'Times New Roman',serif;
}
.ai-image-panel-hint{
  margin-top:4px;
  color:#6f7d6d;
  font-size:12px;
  line-height:1.45;
}
.ai-image-record-pills{
  display:flex;
  flex-wrap:wrap;
  justify-content:flex-end;
  gap:8px;
}
.ai-image-record-pill{
  max-width:240px;
  padding:6px 10px;
  border-radius:999px;
  background:rgba(255,255,255,.78);
  box-shadow:inset 0 0 0 1px rgba(134,156,121,.16);
  color:#42523f;
  font-size:11px;
  line-height:1.2;
  white-space:nowrap;
  overflow:hidden;
  text-overflow:ellipsis;
}
  .ai-image-form-grid{
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:0 12px;
}
.ai-image-model-hint{
  margin:-4px 0 12px;
  color:#9a6700;
  font-size:12px;
  line-height:1.5;
}
.ai-image-size-strip{
  display:flex;
  flex-wrap:wrap;
  gap:8px;
}
.ai-image-size-option{
  border:0;
  border-radius:12px;
  padding:8px 14px;
  background:rgba(255,255,255,.82);
  box-shadow:inset 0 0 0 1px rgba(118,144,108,.18);
  color:#41523f;
  cursor:pointer;
  transition:transform .16s ease,box-shadow .16s ease,background .16s ease,color .16s ease;
}
.ai-image-size-option:hover{
  transform:translateY(-1px);
  box-shadow:0 8px 18px rgba(90,117,79,.12),inset 0 0 0 1px rgba(118,144,108,.22);
}
.ai-image-size-option-active{
  background:linear-gradient(135deg,#476847,#6f8f55);
  color:#fff;
  box-shadow:0 12px 24px rgba(87,118,76,.18);
}
.ai-image-submit-row{
  display:flex;
  align-items:center;
  gap:14px;
  margin-top:4px;
}
.ai-image-generate-button{
  min-width:180px;
  border-radius:14px;
  background:linear-gradient(135deg,#476847,#6f8f55);
  border:0;
  box-shadow:0 14px 28px rgba(87,118,76,.2);
}
.ai-image-submit-hint{
  color:#697968;
  font-size:12px;
  line-height:1.45;
}
.ai-image-result-panel{
  display:flex;
  flex-direction:column;
}
.ai-image-state{
  flex:1 1 auto;
  min-height:420px;
  display:flex;
  flex-direction:column;
  align-items:center;
  justify-content:center;
  text-align:center;
  gap:12px;
}
.ai-image-state-title{
  color:#243329;
  font:700 18px/1.2 Georgia,'Times New Roman',serif;
}
.ai-image-state-desc{
  max-width:34ch;
  color:#6d7d67;
  font-size:12px;
  line-height:1.55;
}
.ai-image-state-empty{
  background:
    radial-gradient(circle at center,rgba(203,224,193,.18),transparent 42%),
    linear-gradient(180deg,rgba(255,255,255,.3),rgba(255,255,255,0));
  border-radius:18px;
}
.ai-image-empty-icon{
  width:64px;
  height:64px;
  border-radius:20px;
  display:inline-flex;
  align-items:center;
  justify-content:center;
  background:linear-gradient(135deg,#edf5df,#d9e9ce);
  color:#53734d;
  font:700 28px/1 Georgia,'Times New Roman',serif;
  box-shadow:0 14px 26px rgba(117,156,90,.14);
}
.ai-image-state-error{
  align-items:stretch;
  justify-content:center;
  text-align:left;
}
.ai-image-error-box{
  padding:14px 16px;
  border-radius:16px;
  background:rgba(255,241,242,.96);
  border:1px solid rgba(244,63,94,.16);
  color:#9f1239;
  font-size:12px;
  line-height:1.6;
  white-space:pre-wrap;
  word-break:break-word;
}
.ai-image-result-ready{
  display:flex;
  flex-direction:column;
  gap:10px;
}
.ai-image-preview-card{
  border:0;
  padding:0;
  border-radius:18px;
  overflow:hidden;
  background:#f8fafc;
  cursor:pointer;
  box-shadow:0 14px 32px rgba(15,23,42,.1);
}
.ai-image-preview-image{
  display:block;
  width:100%;
  max-height:460px;
  object-fit:contain;
  background:linear-gradient(180deg,#f7faf7,#ebf1e8);
}
.ai-image-result-meta{
  display:flex;
  flex-wrap:wrap;
  gap:10px;
}
.ai-image-result-meta-line{
  padding:6px 10px;
  border-radius:999px;
  background:rgba(255,255,255,.8);
  box-shadow:inset 0 0 0 1px rgba(118,144,108,.16);
  color:#4a5b48;
  font-size:11px;
}
.ai-image-result-prompt{
  padding:12px 14px;
  border-radius:16px;
  background:rgba(255,255,255,.72);
  box-shadow:inset 0 0 0 1px rgba(118,144,108,.12);
  color:#354337;
  font-size:12px;
  line-height:1.65;
  white-space:pre-wrap;
  word-break:break-word;
}
.ai-image-result-actions{
  display:flex;
  flex-wrap:wrap;
  gap:10px;
}
.ai-image-history-grid{
  display:grid;
  grid-template-columns:repeat(auto-fill,minmax(152px,1fr));
  gap:12px;
}
.ai-image-history-card{
  position:relative;
  border:0;
  padding:0;
  border-radius:16px;
  overflow:hidden;
  aspect-ratio:1;
  cursor:pointer;
  background:#f8fafc;
  box-shadow:0 12px 24px rgba(15,23,42,.08);
  transition:transform .16s ease,box-shadow .16s ease;
}
.ai-image-history-card:hover{
  transform:translateY(-2px);
  box-shadow:0 16px 28px rgba(15,23,42,.12);
}
.ai-image-history-card-active{
  box-shadow:0 0 0 2px rgba(111,143,85,.42),0 18px 30px rgba(87,118,76,.14);
}
.ai-image-history-thumb{
  display:block;
  width:100%;
  height:100%;
  object-fit:cover;
}
.ai-image-history-overlay{
  position:absolute;
  inset:auto 0 0 0;
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:8px;
  padding:8px 10px;
  background:linear-gradient(180deg,rgba(0,0,0,0),rgba(0,0,0,.74));
}
.ai-image-history-time{
  color:#eff7e9;
  font-size:11px;
}
.ai-image-history-delete{
  border:0;
  padding:0;
  background:transparent;
  color:#fecdd3;
  cursor:pointer;
  font-size:11px;
  line-height:1.2;
}
.ai-image-preview-modal-body{
  display:grid;
  gap:12px;
}
.ai-image-preview-modal-image{
  display:block;
  width:100%;
  max-height:72vh;
  object-fit:contain;
  border-radius:14px;
  background:linear-gradient(180deg,#f7faf7,#ebf1e8);
}
.ai-image-preview-modal-prompt{
  color:#475569;
  font-size:12px;
  line-height:1.65;
  white-space:pre-wrap;
  word-break:break-word;
}
@media (max-width: 980px){
  .ai-image-layout{
    grid-template-columns:minmax(0,1fr);
  }
}
@media (max-width: 720px){
  .ai-image-view{
    padding:6px;
  }
  .ai-image-shell{
    min-height:calc(100vh - 12px);
    padding:12px;
    border-radius:20px;
  }
  .ai-image-header,.ai-image-panel-head,.ai-image-history-head,.ai-image-submit-row{
    flex-direction:column;
    align-items:stretch;
  }
  .ai-image-record-pills{
    justify-content:flex-start;
  }
  .ai-image-form-grid{
    grid-template-columns:minmax(0,1fr);
  }
}
</style>
