<template>
  <div class="ai-image-view">
    <div class="ai-image-shell">
      <div class="ai-image-header">
        <div class="ai-image-header-copy">
          <div class="ai-image-kicker">AI Image Lab</div>
          <div class="ai-image-title">AI 绘图</div>
          <div class="ai-image-subtitle">
            {{ targetRecord ? `${targetRecord.siteName} · ${maskedRecordKeyLabel}` : '当前窗口需要从密钥管理中启动。' }}
          </div>
        </div>

        <button type="button" class="ai-image-close-button" aria-label="关闭" @click.stop="closeWindow">×</button>
      </div>

      <a-empty v-if="!targetRecord" description="未找到目标密钥记录，无法发起 AI 绘图。" />

      <template v-else>
        <div class="ai-image-layout">
          <section class="ai-image-form-panel">
            <div class="ai-image-panel-head">
              <div>
                <div class="ai-image-panel-title">绘图配置</div>
                <div class="ai-image-panel-hint">沿用当前密钥上下文，支持文生图、图生图和涂抹式局部重绘。</div>
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
                    :filter-option="filterModelOption"
                    option-filter-prop="label"
                    placeholder="优选 gpt-5.3-codex，没有时请手动选择"
                  />
                </a-form-item>
              </div>

              <div class="ai-image-inline-actions">
                <a-button size="small" :loading="modelLoading" @click="refreshModelOptions(true)">刷新模型</a-button>
                <span class="ai-image-model-hint">{{ modelSelectionHint }}</span>
              </div>

              <a-form-item label="API Key">
                <a-input-password v-model:value="draft.apiKey" placeholder="输入当前站点对应的 API Key" />
              </a-form-item>

              <div class="ai-image-mode-block">
                <div class="ai-image-mode-label">工作模式</div>
                <div class="ai-image-mode-strip">
                  <button
                    v-for="item in WORKFLOW_MODES"
                    :key="item.value"
                    type="button"
                    class="ai-image-mode-option"
                    :class="{ 'ai-image-mode-option-active': workflowMode === item.value }"
                    @click="workflowMode = item.value"
                  >
                    {{ item.label }}
                  </button>
                </div>
                <div class="ai-image-mode-hint">{{ workflowModeHint }}</div>
              </div>

              <div v-if="workflowMode === 'reference'" class="ai-image-media-block">
                <div class="ai-image-media-head">
                  <div>
                    <div class="ai-image-media-title">参考图</div>
                    <div class="ai-image-media-hint">最多添加 4 张。上传后会自动转为 base64 data URL，通过 Responses 对话链路发送。</div>
                  </div>
                  <div class="ai-image-media-actions">
                    <a-button size="small" @click="triggerReferencePicker">添加参考图</a-button>
                    <a-button size="small" :disabled="!activeItem" @click="useActiveResultAsReference">使用当前结果</a-button>
                  </div>
                </div>

                <div v-if="referenceImages.length" class="ai-image-media-grid">
                  <div v-for="item in referenceImages" :key="item.id" class="ai-image-media-card">
                    <img :src="item.dataUrl" :alt="item.name" class="ai-image-media-thumb" />
                    <div class="ai-image-media-card-foot">
                      <span class="ai-image-media-name">{{ item.name }}</span>
                      <button type="button" class="ai-image-media-remove" @click="removeReferenceImage(item.id)">移除</button>
                    </div>
                  </div>
                </div>
                <a-empty v-else description="还没有添加参考图" />
              </div>

              <div v-if="workflowMode === 'inpaint'" class="ai-image-media-block">
                <div class="ai-image-media-head">
                  <div>
                    <div class="ai-image-media-title">底图</div>
                    <div class="ai-image-media-hint">选择底图后，在右侧画布上直接涂抹需要重绘的区域。</div>
                  </div>
                  <div class="ai-image-media-actions">
                    <a-button size="small" @click="triggerInpaintPicker">上传底图</a-button>
                    <a-button size="small" :disabled="!activeItem" @click="useActiveResultAsInpaintSource">使用当前结果</a-button>
                    <a-button size="small" danger :disabled="!inpaintSourceImage" @click="clearInpaintSource">清空底图</a-button>
                  </div>
                </div>

                <div v-if="inpaintSourceImage" class="ai-image-source-card">
                  <img :src="inpaintSourceImage.dataUrl" :alt="inpaintSourceImage.name" class="ai-image-source-image" />
                  <div class="ai-image-source-meta">
                    <div>{{ inpaintSourceImage.name }}</div>
                    <div>{{ inpaintSourceImage.width }} × {{ inpaintSourceImage.height }}</div>
                  </div>
                </div>
                <a-empty v-else description="还没有选择底图" />
              </div>

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
                <div class="ai-image-model-hint">请求仍然统一走 `/v1/responses`，局部重绘会额外附带蒙版图片。</div>
              </a-form-item>

              <a-form-item label="提示词">
                <a-textarea
                  v-model:value="draft.prompt"
                  :auto-size="{ minRows: 5, maxRows: 10 }"
                  placeholder="描述主体、风格、构图和希望修改的内容。局部重绘时可直接描述被涂抹区域要变成什么。"
                />
              </a-form-item>

              <div class="ai-image-submit-row">
                <a-button type="primary" size="large" class="ai-image-generate-button" :loading="generating" @click="generateImage">
                  发起 AI 绘图
                </a-button>
                <span class="ai-image-submit-hint">返回图片后会写入当前 key 的本地历史，不会串到别的 key。</span>
              </div>
            </a-form>
          </section>

          <section class="ai-image-result-panel">
            <div class="ai-image-result-head">
              <div>
                <div class="ai-image-panel-title">{{ resultPanelTitle }}</div>
                <div class="ai-image-panel-hint">{{ resultPanelHint }}</div>
              </div>

              <div v-if="workflowMode === 'inpaint' && inpaintSourceImage" class="ai-image-mask-toolbar">
                <button
                  type="button"
                  class="ai-image-mask-tool"
                  :class="{ 'ai-image-mask-tool-active': maskTool === 'brush' }"
                  @click="maskTool = 'brush'"
                >
                  涂抹
                </button>
                <button
                  type="button"
                  class="ai-image-mask-tool"
                  :class="{ 'ai-image-mask-tool-active': maskTool === 'eraser' }"
                  @click="maskTool = 'eraser'"
                >
                  擦除
                </button>
                <input v-model="maskBrushSize" class="ai-image-mask-range" type="range" min="8" max="72" step="2" />
                <span class="ai-image-mask-size">{{ maskBrushSize }} px</span>
                <a-button size="small" danger @click="resetMaskCanvas">清空涂抹</a-button>
              </div>
            </div>

            <div v-if="workflowMode === 'inpaint' && inpaintSourceImage" class="ai-image-inpaint-stage">
              <div class="ai-image-stage-frame" :style="inpaintStageStyle">
                <img :src="inpaintSourceImage.dataUrl" :alt="inpaintSourceImage.name" class="ai-image-stage-image" />
                <canvas
                  ref="maskCanvasRef"
                  class="ai-image-mask-canvas"
                  @pointerdown="handleMaskPointerDown"
                  @pointermove="handleMaskPointerMove"
                  @pointerup="stopMaskStroke"
                  @pointercancel="stopMaskStroke"
                />
              </div>
              <div class="ai-image-stage-note">白色涂抹区域会参与重绘。未涂抹区域会尽量保持原图不变。</div>
            </div>

            <div v-if="generating" class="ai-image-state ai-image-state-loading">
              <a-spin size="large" />
              <div class="ai-image-state-title">正在生成图片</div>
              <div class="ai-image-state-desc">请求已发出。图生图和局部重绘通常会比纯文生图稍慢一些。</div>
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
                <div class="ai-image-result-meta-line">模式：{{ WORKFLOW_MODE_LABEL_MAP[activeItem.mode] || '文生图' }}</div>
                <div class="ai-image-result-meta-line">模型：{{ activeItem.model || '未记录' }}</div>
                <div class="ai-image-result-meta-line">尺寸：{{ activeItem.size || 'auto' }}</div>
                <div class="ai-image-result-meta-line">时间：{{ formatDateTime(activeItem.timestamp) }}</div>
              </div>
              <div class="ai-image-result-prompt">{{ activeItem.prompt }}</div>
              <div class="ai-image-result-actions">
                <a-button @click="copyPrompt(activeItem)">复制提示词</a-button>
                <a-button @click="downloadImage(activeItem)">下载图片</a-button>
                <a-button :disabled="referenceImages.length >= 4" @click="useActiveResultAsReference">设为参考图</a-button>
                <a-button @click="useActiveResultAsInpaintSource">设为底图</a-button>
                <a-button type="primary" ghost @click="openPreview(activeItem)">查看大图</a-button>
              </div>
            </div>

            <div v-else class="ai-image-state ai-image-state-empty">
              <div class="ai-image-empty-icon">绘</div>
              <div class="ai-image-state-title">还没有图片结果</div>
              <div class="ai-image-state-desc">先配置模型和提示词，再发起绘图。图生图和局部重绘会自动把图片转成 base64 一并发送。</div>
            </div>
          </section>
        </div>

        <section class="ai-image-history-panel">
          <div class="ai-image-history-head">
            <div>
              <div class="ai-image-panel-title">本地历史</div>
              <div class="ai-image-panel-hint">仅保留当前 key 的绘图记录，互不串库。</div>
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
                <button type="button" class="ai-image-history-delete" @click.stop="confirmDeleteHistoryItem(item)">删除</button>
              </div>
            </button>
          </div>
        </section>
      </template>
    </div>

    <input ref="referenceFileInput" type="file" accept="image/*" multiple class="ai-image-hidden-input" @change="handleReferenceFileChange" />
    <input ref="inpaintFileInput" type="file" accept="image/*" class="ai-image-hidden-input" @change="handleInpaintFileChange" />

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
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue';
import { message, Modal } from 'ant-design-vue';
import { useRoute } from 'vue-router';
import { WindowSetTitle } from '../../wailsjs/runtime/runtime.js';
import { fetchModelList } from '../utils/api.js';
import { hydrateLastResultsSnapshotCache } from '../utils/historySnapshotStore.js';
import { loadPanelRecords } from '../utils/keyPanelStore.js';
import { maskApiKey } from '../utils/normal.js';
import { apiFetch, isProbablyWailsRuntime } from '../utils/runtimeApi.js';

const SIZE_OPTIONS = [
  { label: '1024×1024', value: '1024x1024' },
  { label: '1024×1536', value: '1024x1536' },
  { label: '1536×1024', value: '1536x1024' },
  { label: 'Auto', value: 'auto' },
];

const WORKFLOW_MODES = [
  { label: '文生图', value: 'generate' },
  { label: '参考图生成', value: 'reference' },
  { label: '局部重绘', value: 'inpaint' },
];

const WORKFLOW_MODE_LABEL_MAP = {
  generate: '文生图',
  reference: '参考图生成',
  inpaint: '局部重绘',
};

const IMAGE_HISTORY_DB_NAME = 'AllApiDeckAIImageHistory';
const IMAGE_HISTORY_DB_VERSION = 2;
const IMAGE_HISTORY_STORE = 'images';
const IMAGE_HISTORY_SCOPE_INDEX = 'scopeKey';
const IMAGE_REQUEST_TIMEOUT_MS = 180000;
const IMAGE_WINDOW_SETTINGS_PREFIX = 'all_api_deck_ai_image_window_settings_v2';
const MAX_REFERENCE_IMAGES = 4;

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
const workflowMode = ref('generate');
const referenceImages = ref([]);
const inpaintSourceImage = ref(null);
const referenceFileInput = ref(null);
const inpaintFileInput = ref(null);
const maskCanvasRef = ref(null);
const maskTool = ref('brush');
const maskBrushSize = ref(28);
const maskStrokeActive = ref(false);
const maskStrokePointerId = ref(null);
const maskLastPoint = reactive({ x: 0, y: 0, brushPx: 0 });

const draft = reactive({
  baseUrl: '',
  apiKey: '',
  model: '',
  prompt: '',
  size: '1024x1536',
});

const maskedRecordKeyLabel = computed(() => maskApiKey(String(targetRecord.value?.apiKey || '').trim()) || '未命名 Key');
const activeItem = computed(() => historyItems.value.find(item => item.id === activeItemId.value) || historyItems.value[0] || null);
const hasPreferredModel = computed(() => modelOptions.value.some(item => item.value === 'gpt-5.3-codex'));
const inpaintStageStyle = computed(() => {
  const width = Number(inpaintSourceImage.value?.width || 0);
  const height = Number(inpaintSourceImage.value?.height || 0);
  return width > 0 && height > 0 ? { aspectRatio: `${width} / ${height}` } : {};
});
const resultPanelTitle = computed(() => workflowMode.value === 'inpaint' ? '局部重绘画布' : '当前结果');
const resultPanelHint = computed(() => {
  if (workflowMode.value === 'reference') return '会把参考图作为 input_image 一起发给 Responses。';
  if (workflowMode.value === 'inpaint') return '在底图上直接涂抹，蒙版会作为 input_image_mask 发送。';
  return '成功返回后会在这里展示最新结果。';
});
const modelSelectionHint = computed(() => {
  if (modelLoading.value) return '正在加载模型列表...';
  if (!modelOptions.value.length) return '暂未拿到模型列表，请确认 Base URL / API Key 是否正确。';
  if (hasPreferredModel.value) return '已优先命中 gpt-5.3-codex。';
  if (!draft.model) return '当前列表没有 gpt-5.3-codex，请手动选择支持 Responses 生图的 OpenAI 模型。';
  return '如返回纯文本而不是图片，通常代表当前号池不支持 image_generation 工具。';
});
const workflowModeHint = computed(() => {
  if (workflowMode.value === 'reference') return '上传参考图后，会把提示词和参考图一起送入 Responses 进行图生图。';
  if (workflowMode.value === 'inpaint') return '先选底图，再在右侧画布涂抹要修改的区域，最后描述想改成什么。';
  return '纯文本生成图片。';
});
const windowTitle = computed(() => {
  if (!targetRecord.value) return 'AI 绘图';
  const siteName = String(targetRecord.value.siteName || '未命名站点').trim();
  return `AI绘图-${maskedRecordKeyLabel.value}-${siteName}`;
});

function filterModelOption(input, option) {
  const keyword = String(input || '').toLowerCase();
  const label = String(option?.label || option?.value || '').toLowerCase();
  return label.includes(keyword);
}

function getWailsAppBridge() {
  return window?.go?.main?.App || null;
}

async function requestQuitSafely() {
  const bridge = getWailsAppBridge();
  if (typeof bridge?.RequestQuit !== 'function') return false;
  await bridge.RequestQuit();
  return true;
}

async function getLaunchRecordKeySafely() {
  const bridge = getWailsAppBridge();
  if (typeof bridge?.GetLaunchRecordKey !== 'function') return '';
  return String(await bridge.GetLaunchRecordKey() || '').trim();
}

function normalizeBaseUrl(input) {
  return String(input || '').trim().replace(/\/+$/, '');
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
  if (/\/v\d+\/responses$/i.test(normalizedInput)) return [normalizedInput];
  if (/\/v\d+$/i.test(normalizedInput)) return [`${normalizedInput}/responses`];
  const stripped = stripKnownApiSuffix(normalizedInput);
  if (/\/v\d+$/i.test(stripped)) return [`${stripped}/responses`];
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
  return Array.from(
    new Set(
      combined
        .map(item => (typeof item === 'string' ? item : (item?.id || item?.model || item?.name || '')))
        .map(item => String(item || '').trim())
        .filter(Boolean)
    )
  );
}

function getModelPriority(model) {
  if (model === 'gpt-5.3-codex') return 0;
  if (/gpt-image|chatgpt-image/i.test(model)) return 1;
  if (isOpenAIFamilyModel(model)) return 2;
  return 3;
}

function buildModelOptions(record, extraModels = []) {
  return normalizeModelCandidates(record, extraModels)
    .sort((left, right) => {
      const leftPriority = getModelPriority(left);
      const rightPriority = getModelPriority(right);
      if (leftPriority !== rightPriority) return leftPriority - rightPriority;
      return left.localeCompare(right);
    })
    .map(model => ({ label: model, value: model }));
}

function pickPreferredModel(fallback = '') {
  if (modelOptions.value.some(item => item.value === 'gpt-5.3-codex')) return 'gpt-5.3-codex';
  const normalizedFallback = String(fallback || '').trim();
  if (normalizedFallback && modelOptions.value.some(item => item.value === normalizedFallback)) return normalizedFallback;
  return modelOptions.value.find(item => isOpenAIFamilyModel(item.value))?.value || modelOptions.value[0]?.value || '';
}

function buildDefaultPrompt(record) {
  const siteName = String(record?.siteName || '').trim();
  return siteName ? `围绕“${siteName}”生成一张可直接使用的高质量图片，主体明确、构图完整、细节干净。` : '';
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
      prompt: draft.prompt,
      size: draft.size,
      workflowMode: workflowMode.value,
    }));
  } catch {}
}

function buildInitialDraft(record) {
  const saved = loadPersistedSettings();
  draft.baseUrl = String(saved.baseUrl || normalizeBaseUrl(record?.siteUrl || '')).trim();
  draft.apiKey = String(saved.apiKey || record?.apiKey || '').trim();
  draft.model = String(saved.model || record?.selectedModel || '').trim();
  draft.prompt = String(saved.prompt || buildDefaultPrompt(record)).trim();
  draft.size = String(saved.size || '1024x1536').trim() || '1024x1536';
  workflowMode.value = WORKFLOW_MODES.some(item => item.value === saved.workflowMode) ? saved.workflowMode : 'generate';
}

function buildImagePrompt(prompt, size, mode = workflowMode.value) {
  const normalizedPrompt = String(prompt || '').trim();
  const sizeHint = size === 'auto' ? '尺寸要求：自动。' : `尺寸要求：${size}。`;
  const modeHintMap = {
    generate: '请调用 image_generation 工具直接输出图片，不要输出说明文字。',
    reference: '请参考随附参考图的主体、色彩、构图或质感重新生成图片，并直接输出图片结果。',
    inpaint: '请只修改被蒙版选中的区域，未选中的区域尽量保持原图一致，并直接输出图片结果。',
  };
  return [
    normalizedPrompt || '请生成一张图片。',
    sizeHint,
    modeHintMap[mode] || modeHintMap.generate,
  ].filter(Boolean).join('\n\n');
}

function buildResponsesInput(finalPrompt) {
  const content = [{ type: 'input_text', text: finalPrompt }];
  if (workflowMode.value === 'reference') {
    referenceImages.value.forEach(item => {
      content.push({ type: 'input_image', image_url: item.dataUrl });
    });
  }
  if (workflowMode.value === 'inpaint' && inpaintSourceImage.value?.dataUrl) {
    content.push({ type: 'input_image', image_url: inpaintSourceImage.value.dataUrl });
  }
  return [{ role: 'user', content }];
}

function buildImageGenerationTool() {
  const tool = { type: 'image_generation' };
  if (draft.size && draft.size !== 'auto') {
    tool.size = draft.size;
  }
  if (workflowMode.value === 'inpaint') {
    tool.input_image_mask = { image_url: buildMaskDataUrl() };
  }
  return tool;
}

function buildErrorMessage(status, responseText) {
  const rawText = String(responseText || '').trim();
  if (!rawText) return `HTTP ${status}`;
  try {
    const payload = JSON.parse(rawText);
    return payload?.error?.message || payload?.message || rawText;
  } catch {
    return rawText;
  }
}

function collectOutputTexts(payload) {
  const outputs = Array.isArray(payload?.output) ? payload.output : [];
  const contentTexts = outputs
    .flatMap(item => Array.isArray(item?.content) ? item.content : [])
    .filter(item => item?.type === 'output_text')
    .map(item => String(item?.text || '').trim())
    .filter(Boolean);
  const choiceTexts = Array.isArray(payload?.choices)
    ? payload.choices
      .map(choice => String(choice?.message?.content || '').trim())
      .filter(Boolean)
    : [];
  return [...contentTexts, ...choiceTexts];
}

function buildReadablePayloadErrorDetails(payload) {
  const lines = [];
  const objectType = String(payload?.object || '').trim();
  const actualModel = String(payload?.model || '').trim();
  const status = String(payload?.status || '').trim();
  const outputs = Array.isArray(payload?.output) ? payload.output : [];
  const outputTexts = collectOutputTexts(payload);
  const topLevelError = payload?.error?.message || payload?.error || '';

  if (objectType) lines.push(`实际响应对象：${objectType}`);
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

function formatDateTime(timestamp) {
  if (!timestamp) return '未知时间';
  try {
    return new Date(timestamp).toLocaleString();
  } catch {
    return '时间异常';
  }
}

function formatCompactDateTime(timestamp) {
  if (!timestamp) return '--';
  const date = new Date(timestamp);
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${month}-${day} ${hours}:${minutes}`;
}

function toDataUrl(base64) {
  const value = String(base64 || '').trim();
  if (!value) return '';
  if (/^data:/i.test(value)) return value;
  return `data:image/png;base64,${value}`;
}

function updateWindowTitle() {
  try {
    WindowSetTitle(windowTitle.value);
  } catch {}
}

function buildLocalImageId() {
  return `img_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function loadImageElement(src) {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error('图片读取失败'));
    image.src = src;
  });
}

function readFileAsDataUrl(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(new Error('文件读取失败'));
    reader.readAsDataURL(file);
  });
}

async function createLocalImageAsset({ dataUrl, name, source = 'local' }) {
  const image = await loadImageElement(dataUrl);
  return {
    id: buildLocalImageId(),
    name: String(name || 'image').trim() || 'image',
    dataUrl,
    width: Number(image.naturalWidth || image.width || 0),
    height: Number(image.naturalHeight || image.height || 0),
    source,
  };
}

async function createLocalImageAssetFromFile(file, source = 'local') {
  const dataUrl = await readFileAsDataUrl(file);
  return createLocalImageAsset({
    dataUrl,
    name: String(file?.name || 'upload').trim() || 'upload',
    source,
  });
}

async function createLocalImageAssetFromHistory(item, source = 'history') {
  return createLocalImageAsset({
    dataUrl: toDataUrl(item?.base64),
    name: `${WORKFLOW_MODE_LABEL_MAP[item?.mode] || 'result'}-${formatCompactDateTime(item?.timestamp).replace(/\s+/g, '-')}.png`,
    source,
  });
}

function triggerReferencePicker() {
  referenceFileInput.value?.click();
}

function triggerInpaintPicker() {
  inpaintFileInput.value?.click();
}

function appendReferenceAssets(assets) {
  const next = [...referenceImages.value];
  assets.forEach(asset => {
    if (!next.some(item => item.dataUrl === asset.dataUrl)) {
      next.push(asset);
    }
  });
  if (next.length > MAX_REFERENCE_IMAGES) {
    next.splice(MAX_REFERENCE_IMAGES);
    message.warning(`参考图最多保留 ${MAX_REFERENCE_IMAGES} 张。`);
  }
  referenceImages.value = next;
}

async function handleReferenceFileChange(event) {
  const files = Array.from(event?.target?.files || []);
  if (!files.length) return;
  try {
    const assets = await Promise.all(files.map(file => createLocalImageAssetFromFile(file, 'reference-upload')));
    appendReferenceAssets(assets);
  } catch (error) {
    message.error(error?.message || '参考图读取失败');
  } finally {
    event.target.value = '';
  }
}

function removeReferenceImage(id) {
  referenceImages.value = referenceImages.value.filter(item => item.id !== id);
}

async function useActiveResultAsReference() {
  if (!activeItem.value) return;
  try {
    const asset = await createLocalImageAssetFromHistory(activeItem.value, 'reference-history');
    appendReferenceAssets([asset]);
    workflowMode.value = 'reference';
    message.success('已把当前结果加入参考图');
  } catch (error) {
    message.error(error?.message || '参考图转换失败');
  }
}

async function setInpaintSourceAsset(asset) {
  inpaintSourceImage.value = asset;
  workflowMode.value = 'inpaint';
  await nextTick();
  resetMaskCanvas();
}

async function handleInpaintFileChange(event) {
  const files = Array.from(event?.target?.files || []);
  const file = files[0];
  if (!file) return;
  try {
    const asset = await createLocalImageAssetFromFile(file, 'inpaint-upload');
    await setInpaintSourceAsset(asset);
  } catch (error) {
    message.error(error?.message || '底图读取失败');
  } finally {
    event.target.value = '';
  }
}

async function useActiveResultAsInpaintSource() {
  if (!activeItem.value) return;
  try {
    const asset = await createLocalImageAssetFromHistory(activeItem.value, 'inpaint-history');
    await setInpaintSourceAsset(asset);
    message.success('已把当前结果设为底图');
  } catch (error) {
    message.error(error?.message || '底图转换失败');
  }
}

function clearInpaintSource() {
  inpaintSourceImage.value = null;
  resetMaskCanvas();
}

function resetMaskCanvas() {
  const canvas = maskCanvasRef.value;
  const source = inpaintSourceImage.value;
  if (!canvas || !source) return;
  canvas.width = Number(source.width || 0);
  canvas.height = Number(source.height || 0);
  const context = canvas.getContext('2d');
  if (!context) return;
  context.clearRect(0, 0, canvas.width, canvas.height);
  maskStrokeActive.value = false;
  maskStrokePointerId.value = null;
}

function getMaskCanvasPoint(event) {
  const canvas = maskCanvasRef.value;
  if (!canvas) return null;
  const rect = canvas.getBoundingClientRect();
  if (!rect.width || !rect.height) return null;
  const scaleX = canvas.width / rect.width;
  const scaleY = canvas.height / rect.height;
  return {
    x: (event.clientX - rect.left) * scaleX,
    y: (event.clientY - rect.top) * scaleY,
    brushPx: Math.max(6, maskBrushSize.value * ((scaleX + scaleY) / 2)),
  };
}

function paintMaskLine(startPoint, endPoint, brushPx, erase) {
  const canvas = maskCanvasRef.value;
  const context = canvas?.getContext('2d');
  if (!canvas || !context) return;
  context.save();
  context.lineCap = 'round';
  context.lineJoin = 'round';
  context.lineWidth = brushPx;
  context.globalCompositeOperation = erase ? 'destination-out' : 'source-over';
  context.strokeStyle = erase ? 'rgba(0,0,0,1)' : 'rgba(255,255,255,0.96)';
  context.fillStyle = erase ? 'rgba(0,0,0,1)' : 'rgba(255,255,255,0.96)';
  context.beginPath();
  context.moveTo(startPoint.x, startPoint.y);
  context.lineTo(endPoint.x, endPoint.y);
  context.stroke();
  context.beginPath();
  context.arc(endPoint.x, endPoint.y, brushPx / 2, 0, Math.PI * 2);
  context.fill();
  context.restore();
}

function handleMaskPointerDown(event) {
  if (!inpaintSourceImage.value) return;
  const point = getMaskCanvasPoint(event);
  if (!point) return;
  maskStrokeActive.value = true;
  maskStrokePointerId.value = event.pointerId;
  maskLastPoint.x = point.x;
  maskLastPoint.y = point.y;
  maskLastPoint.brushPx = point.brushPx;
  event.target?.setPointerCapture?.(event.pointerId);
  paintMaskLine(point, point, point.brushPx, maskTool.value === 'eraser');
}

function handleMaskPointerMove(event) {
  if (!maskStrokeActive.value || maskStrokePointerId.value !== event.pointerId) return;
  const point = getMaskCanvasPoint(event);
  if (!point) return;
  paintMaskLine(
    { x: maskLastPoint.x, y: maskLastPoint.y },
    point,
    point.brushPx,
    maskTool.value === 'eraser'
  );
  maskLastPoint.x = point.x;
  maskLastPoint.y = point.y;
  maskLastPoint.brushPx = point.brushPx;
}

function stopMaskStroke(event) {
  if (event?.pointerId != null && maskStrokePointerId.value != null && event.pointerId !== maskStrokePointerId.value) return;
  maskStrokeActive.value = false;
  maskStrokePointerId.value = null;
}

function maskHasPaintedPixels() {
  const canvas = maskCanvasRef.value;
  const context = canvas?.getContext('2d');
  if (!canvas || !context) return false;
  const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data;
  for (let index = 3; index < pixels.length; index += 4) {
    if (pixels[index] > 0) return true;
  }
  return false;
}

function buildMaskDataUrl() {
  const canvas = maskCanvasRef.value;
  return canvas ? canvas.toDataURL('image/png') : '';
}

function openHistoryDb() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(IMAGE_HISTORY_DB_NAME, IMAGE_HISTORY_DB_VERSION);
    request.onupgradeneeded = () => {
      const database = request.result;
      const store = database.objectStoreNames.contains(IMAGE_HISTORY_STORE)
        ? request.transaction.objectStore(IMAGE_HISTORY_STORE)
        : database.createObjectStore(IMAGE_HISTORY_STORE, { keyPath: 'id', autoIncrement: true });
      if (!store.indexNames.contains(IMAGE_HISTORY_SCOPE_INDEX)) {
        store.createIndex(IMAGE_HISTORY_SCOPE_INDEX, 'scopeKey', { unique: false });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('IndexedDB 打开失败'));
  });
}

async function saveHistoryItem(item) {
  const database = await openHistoryDb();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(IMAGE_HISTORY_STORE, 'readwrite');
    const store = transaction.objectStore(IMAGE_HISTORY_STORE);
    const request = store.add(item);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('历史写入失败'));
    transaction.oncomplete = () => database.close();
    transaction.onerror = () => reject(transaction.error || new Error('历史写入失败'));
  });
}

async function loadHistoryList() {
  if (!scopeKey.value) {
    historyItems.value = [];
    activeItemId.value = null;
    return;
  }
  const database = await openHistoryDb();
  const previousActiveId = activeItemId.value;
  const items = await new Promise((resolve, reject) => {
    const transaction = database.transaction(IMAGE_HISTORY_STORE, 'readonly');
    const store = transaction.objectStore(IMAGE_HISTORY_STORE);
    const index = store.index(IMAGE_HISTORY_SCOPE_INDEX);
    const request = index.getAll(scopeKey.value);
    request.onsuccess = () => resolve(Array.isArray(request.result) ? request.result : []);
    request.onerror = () => reject(request.error || new Error('历史读取失败'));
    transaction.oncomplete = () => database.close();
    transaction.onerror = () => reject(transaction.error || new Error('历史读取失败'));
  });
  historyItems.value = items.sort((left, right) => Number(right.timestamp || 0) - Number(left.timestamp || 0));
  if (historyItems.value.some(item => item.id === previousActiveId)) {
    activeItemId.value = previousActiveId;
  } else {
    activeItemId.value = historyItems.value[0]?.id ?? null;
  }
}

async function deleteHistoryItem(id) {
  const database = await openHistoryDb();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(IMAGE_HISTORY_STORE, 'readwrite');
    const store = transaction.objectStore(IMAGE_HISTORY_STORE);
    const request = store.delete(id);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error || new Error('历史删除失败'));
    transaction.oncomplete = () => database.close();
    transaction.onerror = () => reject(transaction.error || new Error('历史删除失败'));
  });
}

async function clearHistoryByScope() {
  if (!scopeKey.value) return;
  const database = await openHistoryDb();
  await new Promise((resolve, reject) => {
    const transaction = database.transaction(IMAGE_HISTORY_STORE, 'readwrite');
    const store = transaction.objectStore(IMAGE_HISTORY_STORE);
    const index = store.index(IMAGE_HISTORY_SCOPE_INDEX);
    const request = index.openCursor(IDBKeyRange.only(scopeKey.value));
    request.onsuccess = () => {
      const cursor = request.result;
      if (!cursor) {
        resolve();
        return;
      }
      store.delete(cursor.primaryKey);
      cursor.continue();
    };
    request.onerror = () => reject(request.error || new Error('历史清空失败'));
    transaction.oncomplete = () => database.close();
    transaction.onerror = () => reject(transaction.error || new Error('历史清空失败'));
  });
  historyItems.value = [];
  activeItemId.value = null;
}

async function refreshModelOptions(showFeedback = false) {
  if (!targetRecord.value) return;
  const record = targetRecord.value;
  const baseUrl = normalizeBaseUrl(draft.baseUrl || record.siteUrl || '');
  const apiKey = String(draft.apiKey || record.apiKey || '').trim();
  modelLoading.value = true;
  let extraModels = [];

  try {
    if (baseUrl && apiKey) {
      const payload = await fetchModelList(baseUrl, apiKey);
      extraModels = Array.isArray(payload?.data) ? payload.data : (Array.isArray(payload?.models) ? payload.models : []);
    }
  } catch (error) {
    if (showFeedback) {
      message.warning(error?.message || '模型列表刷新失败');
    }
  } finally {
    modelOptions.value = buildModelOptions(record, extraModels);
    draft.model = pickPreferredModel(draft.model || record.selectedModel || '');
    modelLoading.value = false;
  }
}

function buildRequestBody(finalPrompt) {
  return {
    model: String(draft.model || '').trim(),
    input: buildResponsesInput(finalPrompt),
    tools: [buildImageGenerationTool()],
    tool_choice: 'auto',
  };
}

async function requestImageGeneration(requestBody) {
  const endpoints = buildResponsesEndpointCandidates(draft.baseUrl);
  const errors = [];

  for (const endpoint of endpoints) {
    try {
      const response = await apiFetch(endpoint, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${String(draft.apiKey || '').trim()}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody),
        timeoutMs: IMAGE_REQUEST_TIMEOUT_MS,
      });
      const responseText = await response.text().catch(() => '');
      if (!response.ok) {
        errors.push(`${endpoint} -> ${buildErrorMessage(response.status, responseText)}`);
        continue;
      }
      const payload = JSON.parse(responseText || 'null');
      return { endpoint, payload };
    } catch (error) {
      errors.push(`${endpoint} -> ${error?.message || '请求失败'}`);
    }
  }

  throw new Error(errors.join('\n\n') || '绘图接口请求失败');
}

function validateGenerateRequest() {
  if (!normalizeBaseUrl(draft.baseUrl)) {
    return '请输入 Base URL';
  }
  if (!String(draft.apiKey || '').trim()) {
    return '请输入 API Key';
  }
  if (!String(draft.model || '').trim()) {
    return '请先选择一个支持 Responses 生图的模型';
  }
  if (!String(draft.prompt || '').trim()) {
    return '请输入提示词';
  }
  if (workflowMode.value === 'reference' && referenceImages.value.length === 0) {
    return '请至少添加一张参考图';
  }
  if (workflowMode.value === 'inpaint' && !inpaintSourceImage.value) {
    return '请先选择底图';
  }
  if (workflowMode.value === 'inpaint' && !maskHasPaintedPixels()) {
    return '请先在右侧底图上涂抹需要重绘的区域';
  }
  return '';
}

async function generateImage() {
  const validationError = validateGenerateRequest();
  if (validationError) {
    errorMessage.value = validationError;
    return;
  }

  generating.value = true;
  errorMessage.value = '';

  try {
    const finalPrompt = buildImagePrompt(draft.prompt, draft.size, workflowMode.value);
    const requestBody = buildRequestBody(finalPrompt);
    const { endpoint, payload } = await requestImageGeneration(requestBody);
    const outputs = Array.isArray(payload?.output) ? payload.output : [];
    const imageCall = outputs.find(item => item?.type === 'image_generation_call' && String(item?.result || '').trim());
    const base64 = String(imageCall?.result || '').trim();

    if (!base64) {
      throw new Error(
        [
          '当前号池/中转站不具备调用 image_generation 工具能力，请更换支持 Responses 生图工具的中转站。',
          `命中端点：${endpoint}`,
          `请求模型：${String(draft.model || '').trim()}`,
          buildReadablePayloadErrorDetails(payload),
        ].filter(Boolean).join('\n')
      );
    }

    const nextItem = {
      scopeKey: scopeKey.value,
      siteName: String(targetRecord.value?.siteName || '').trim(),
      siteUrl: String(targetRecord.value?.siteUrl || '').trim(),
      model: String(draft.model || '').trim(),
      size: draft.size,
      prompt: String(draft.prompt || '').trim(),
      mode: workflowMode.value,
      base64,
      timestamp: Date.now(),
    };

    nextItem.id = await saveHistoryItem(nextItem);
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

function selectHistoryItem(item) {
  activeItemId.value = item?.id ?? null;
}

function openPreview(item) {
  previewItem.value = item || null;
  previewOpen.value = Boolean(item);
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
    },
  });
}

function confirmClearHistory() {
  Modal.confirm({
    title: '确认清空当前 key 的绘图历史？',
    content: '只会删除当前密钥对应的本地绘图历史，不影响其他 key。',
    okText: '清空',
    cancelText: '取消',
    okButtonProps: { danger: true },
    async onOk() {
      await clearHistoryByScope();
      message.success('当前 key 的绘图历史已清空');
    },
  });
}

async function closeWindow() {
  try {
    if (await requestQuitSafely()) return;
  } catch {}
  window.close();
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

  if (!targetRecord.value) {
    updateWindowTitle();
    return;
  }

  buildInitialDraft(targetRecord.value);
  modelOptions.value = buildModelOptions(targetRecord.value);
  draft.model = pickPreferredModel(draft.model || targetRecord.value.selectedModel || '');
  await refreshModelOptions(false);
  await loadHistoryList();
  updateWindowTitle();
}

watch(
  () => [draft.baseUrl, draft.apiKey, draft.model, draft.prompt, draft.size, workflowMode.value],
  () => {
    persistSettings();
  }
);

watch(windowTitle, () => {
  updateWindowTitle();
});

watch(
  () => workflowMode.value,
  async mode => {
    if (mode === 'inpaint' && inpaintSourceImage.value) {
      await nextTick();
      resetMaskCanvas();
    }
  }
);

watch(
  () => inpaintSourceImage.value?.id,
  async () => {
    if (inpaintSourceImage.value) {
      await nextTick();
      resetMaskCanvas();
    }
  }
);

onMounted(() => {
  void bootstrap();
});
</script>

<style scoped>
.ai-image-view {
  width: 100%;
  min-height: 100vh;
  box-sizing: border-box;
  padding: 10px;
  display: flex;
  background:
    radial-gradient(circle at 14% 12%, rgba(190, 224, 183, 0.78), transparent 24%),
    radial-gradient(circle at 86% 10%, rgba(179, 210, 242, 0.58), transparent 22%),
    linear-gradient(180deg, #edf4e7, #dfe8da);
}

.ai-image-shell {
  width: min(100%, 1180px);
  min-height: calc(100vh - 20px);
  margin: 0 auto;
  padding: 14px 16px 16px;
  border-radius: 26px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(247, 250, 246, 0.95));
  box-shadow: 0 22px 56px rgba(61, 83, 48, 0.14), inset 0 1px 0 rgba(255, 255, 255, 0.74);
  backdrop-filter: blur(14px) saturate(108%);
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.ai-image-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  -webkit-app-region: drag;
}

.ai-image-header-copy {
  min-width: 0;
}

.ai-image-kicker {
  margin-bottom: 2px;
  color: #6f8f55;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.ai-image-title {
  color: #243329;
  font: 800 29px/1.06 Georgia, 'Times New Roman', serif;
}

.ai-image-subtitle {
  margin-top: 4px;
  color: #6d7d67;
  font-size: 12px;
  line-height: 1.45;
}

.ai-image-close-button {
  -webkit-app-region: no-drag;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 999px;
  background: rgba(185, 28, 28, 0.1);
  color: #c62828;
  font: 700 22px/1 Georgia, serif;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease, transform 0.18s ease;
}

.ai-image-close-button:hover {
  background: rgba(185, 28, 28, 0.18);
  color: #a31212;
  transform: scale(1.04);
}

.ai-image-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.06fr) minmax(320px, 0.94fr);
  gap: 14px;
  min-height: 0;
}

.ai-image-form-panel,
.ai-image-result-panel,
.ai-image-history-panel {
  min-width: 0;
  border: 1px solid rgba(102, 130, 93, 0.12);
  border-radius: 22px;
  background: linear-gradient(180deg, rgba(252, 254, 251, 0.98), rgba(243, 248, 239, 0.94));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.ai-image-form-panel,
.ai-image-result-panel,
.ai-image-history-panel {
  padding: 14px 16px 16px;
}

.ai-image-panel-head,
.ai-image-history-head,
.ai-image-result-head,
.ai-image-media-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.ai-image-panel-head,
.ai-image-history-head {
  margin-bottom: 10px;
}

.ai-image-panel-title,
.ai-image-media-title {
  color: #25342a;
  font: 700 17px/1.1 Georgia, 'Times New Roman', serif;
}

.ai-image-panel-hint,
.ai-image-media-hint {
  margin-top: 4px;
  color: #6f7d6d;
  font-size: 12px;
  line-height: 1.45;
}

.ai-image-record-pills {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.ai-image-record-pill,
.ai-image-result-meta-line {
  max-width: 240px;
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.8);
  box-shadow: inset 0 0 0 1px rgba(134, 156, 121, 0.16);
  color: #42523f;
  font-size: 11px;
  line-height: 1.2;
}

.ai-image-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}

.ai-image-inline-actions {
  margin: -4px 0 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.ai-image-model-hint,
.ai-image-mode-hint {
  color: #9a6700;
  font-size: 12px;
  line-height: 1.5;
}

.ai-image-mode-block,
.ai-image-media-block {
  margin-bottom: 18px;
  padding: 12px 14px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.6);
  box-shadow: inset 0 0 0 1px rgba(118, 144, 108, 0.12);
}

.ai-image-mode-label {
  margin-bottom: 10px;
  color: #344235;
  font-size: 12px;
  font-weight: 700;
}

.ai-image-mode-strip,
.ai-image-size-strip,
.ai-image-result-actions,
.ai-image-media-actions,
.ai-image-mask-toolbar,
.ai-image-result-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ai-image-mode-option,
.ai-image-size-option,
.ai-image-mask-tool {
  border: 0;
  border-radius: 12px;
  padding: 8px 14px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: inset 0 0 0 1px rgba(118, 144, 108, 0.18);
  color: #41523f;
  cursor: pointer;
  transition: transform 0.16s ease, box-shadow 0.16s ease, background 0.16s ease, color 0.16s ease;
}

.ai-image-mode-option:hover,
.ai-image-size-option:hover,
.ai-image-mask-tool:hover {
  transform: translateY(-1px);
  box-shadow: 0 8px 18px rgba(90, 117, 79, 0.12), inset 0 0 0 1px rgba(118, 144, 108, 0.22);
}

.ai-image-mode-option-active,
.ai-image-size-option-active,
.ai-image-mask-tool-active {
  background: linear-gradient(135deg, #476847, #6f8f55);
  color: #fff;
  box-shadow: 0 12px 24px rgba(87, 118, 76, 0.18);
}

.ai-image-media-head {
  margin-bottom: 12px;
}

.ai-image-media-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
  gap: 10px;
}

.ai-image-media-card,
.ai-image-source-card {
  border-radius: 16px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: inset 0 0 0 1px rgba(118, 144, 108, 0.14);
}

.ai-image-media-thumb,
.ai-image-source-image,
.ai-image-history-thumb {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.ai-image-media-card {
  aspect-ratio: 1;
  position: relative;
}

.ai-image-media-card-foot {
  position: absolute;
  inset: auto 0 0 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0), rgba(0, 0, 0, 0.74));
}

.ai-image-media-name {
  color: #eff7e9;
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-image-media-remove,
.ai-image-history-delete {
  border: 0;
  padding: 0;
  background: transparent;
  color: #fecdd3;
  cursor: pointer;
  font-size: 11px;
}

.ai-image-source-card {
  display: grid;
  grid-template-columns: 128px minmax(0, 1fr);
  gap: 12px;
  padding: 10px;
}

.ai-image-source-image {
  aspect-ratio: 1;
  border-radius: 12px;
}

.ai-image-source-meta {
  display: grid;
  align-content: center;
  gap: 6px;
  color: #41523f;
  font-size: 12px;
  line-height: 1.5;
}

.ai-image-submit-row {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 4px;
}

.ai-image-generate-button {
  min-width: 180px;
  border-radius: 14px;
  background: linear-gradient(135deg, #476847, #6f8f55);
  border: 0;
  box-shadow: 0 14px 28px rgba(87, 118, 76, 0.2);
}

.ai-image-submit-hint {
  color: #697968;
  font-size: 12px;
  line-height: 1.45;
}

.ai-image-result-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ai-image-mask-toolbar {
  align-items: center;
}

.ai-image-mask-range {
  width: 120px;
}

.ai-image-mask-size {
  color: #5c6d5a;
  font-size: 11px;
}

.ai-image-inpaint-stage {
  display: grid;
  gap: 10px;
}

.ai-image-stage-frame {
  position: relative;
  width: 100%;
  border-radius: 18px;
  overflow: hidden;
  background: linear-gradient(180deg, #f7faf7, #ebf1e8);
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.08);
}

.ai-image-stage-image,
.ai-image-mask-canvas,
.ai-image-preview-image {
  display: block;
  width: 100%;
  height: 100%;
}

.ai-image-stage-image,
.ai-image-preview-image {
  object-fit: contain;
}

.ai-image-mask-canvas {
  position: absolute;
  inset: 0;
  cursor: crosshair;
  touch-action: none;
}

.ai-image-stage-note,
.ai-image-preview-modal-prompt,
.ai-image-result-prompt {
  color: #475569;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-image-state {
  flex: 1 1 auto;
  min-height: 260px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  gap: 12px;
}

.ai-image-state-title {
  color: #243329;
  font: 700 18px/1.2 Georgia, 'Times New Roman', serif;
}

.ai-image-state-desc {
  max-width: 34ch;
  color: #6d7d67;
  font-size: 12px;
  line-height: 1.55;
}

.ai-image-state-empty {
  background:
    radial-gradient(circle at center, rgba(203, 224, 193, 0.18), transparent 42%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.3), rgba(255, 255, 255, 0));
  border-radius: 18px;
}

.ai-image-empty-icon {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #edf5df, #d9e9ce);
  color: #53734d;
  font: 700 28px/1 Georgia, 'Times New Roman', serif;
  box-shadow: 0 14px 26px rgba(117, 156, 90, 0.14);
}

.ai-image-state-error {
  align-items: stretch;
  text-align: left;
}

.ai-image-error-box {
  padding: 14px 16px;
  border-radius: 16px;
  background: rgba(255, 241, 242, 0.96);
  border: 1px solid rgba(244, 63, 94, 0.16);
  color: #9f1239;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.ai-image-result-ready {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ai-image-preview-card {
  border: 0;
  padding: 0;
  border-radius: 18px;
  overflow: hidden;
  background: #f8fafc;
  cursor: pointer;
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.1);
}

.ai-image-preview-image {
  max-height: 460px;
  background: linear-gradient(180deg, #f7faf7, #ebf1e8);
}

.ai-image-result-prompt {
  padding: 12px 14px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
  box-shadow: inset 0 0 0 1px rgba(118, 144, 108, 0.12);
  color: #354337;
}

.ai-image-history-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(152px, 1fr));
  gap: 12px;
}

.ai-image-history-card {
  position: relative;
  border: 0;
  padding: 0;
  border-radius: 16px;
  overflow: hidden;
  aspect-ratio: 1;
  cursor: pointer;
  background: #f8fafc;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.08);
  transition: transform 0.16s ease, box-shadow 0.16s ease;
}

.ai-image-history-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 16px 28px rgba(15, 23, 42, 0.12);
}

.ai-image-history-card-active {
  box-shadow: 0 0 0 2px rgba(111, 143, 85, 0.42), 0 18px 30px rgba(87, 118, 76, 0.14);
}

.ai-image-history-overlay {
  position: absolute;
  inset: auto 0 0 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0), rgba(0, 0, 0, 0.74));
}

.ai-image-history-time {
  color: #eff7e9;
  font-size: 11px;
}

.ai-image-preview-modal-body {
  display: grid;
  gap: 12px;
}

.ai-image-preview-modal-image {
  display: block;
  width: 100%;
  max-height: 72vh;
  object-fit: contain;
  border-radius: 14px;
  background: linear-gradient(180deg, #f7faf7, #ebf1e8);
}

.ai-image-hidden-input {
  display: none;
}

@media (max-width: 980px) {
  .ai-image-layout {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 720px) {
  .ai-image-view {
    padding: 6px;
  }

  .ai-image-shell {
    min-height: calc(100vh - 12px);
    padding: 12px;
    border-radius: 20px;
  }

  .ai-image-header,
  .ai-image-panel-head,
  .ai-image-history-head,
  .ai-image-result-head,
  .ai-image-media-head,
  .ai-image-submit-row,
  .ai-image-inline-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .ai-image-form-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .ai-image-record-pills {
    justify-content: flex-start;
  }

  .ai-image-source-card {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
