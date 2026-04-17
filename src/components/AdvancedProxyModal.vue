<template>
  <a-modal :open="open" title="高级代理功能" :width="modalWidth" :footer="null" @cancel="handleCancel">
    <a-spin :spinning="loading || saving">
      <div class="advanced-proxy-shell">
        <section class="advanced-proxy-hero">
          <div class="advanced-proxy-hero-copy">
            <a-tag color="green">本地代理接管</a-tag>
            <h3>兼容 OpenAI vendor，并为 Claude / Codex / OpenCode / OpenClaw 提供统一接管入口</h3>
            <p>涉及本地应用配置文件的变更会先弹出 diff 预览；仅修改高级代理自身配置时，会直接写入并立即生效。</p>
          </div>

          <div class="advanced-proxy-master-row">
            <div class="advanced-proxy-master-copy">
              <strong>代理总开关</strong>
              <small>{{ proxyMasterEnabled ? (enabledAppLabels || '已启用') : '关闭后不会接管任何本地应用' }}</small>
            </div>
            <a-switch
              :checked="proxyMasterEnabled"
              checked-children="开启"
              un-checked-children="关闭"
              @change="handleProxyMasterToggle"
            />
          </div>

          <div class="advanced-proxy-app-strip">
            <a-tooltip v-for="app in appCards" :key="app.id" placement="top">
              <template #title>
                <div class="advanced-proxy-app-tooltip">
                  <div>{{ app.label }} · {{ app.modeLabel }}</div>
                  <div v-if="app.tooltipDetail">{{ app.tooltipDetail }}</div>
                </div>
              </template>
              <button
                type="button"
                class="advanced-proxy-app-token"
                :class="{ 'advanced-proxy-app-token-active': app.enabled }"
                @click="handleAppTakeoverToggle(app.id, !app.enabled)"
              >
                <span class="advanced-proxy-app-icon-shell" :class="`advanced-proxy-app-icon-shell-${app.id}`">
                  <img :src="app.icon" :alt="app.label" class="advanced-proxy-app-icon-image" />
                </span>
              </button>
            </a-tooltip>
          </div>
        </section>

        <section class="advanced-proxy-summary-grid">
          <article class="advanced-proxy-summary-card">
            <span>已启用应用</span>
            <strong>{{ enabledAppCount }}</strong>
            <small>{{ enabledAppLabels || '当前未启用' }}</small>
          </article>
          <article class="advanced-proxy-summary-card">
            <span>Provider 队列</span>
            <strong>{{ providerCount }}</strong>
            <small>启用 {{ enabledProviderCount }} 条</small>
          </article>
          <article class="advanced-proxy-summary-card">
            <span>OpenAI 兼容上游</span>
            <strong>{{ openAIProviderCount }}</strong>
            <small>Codex / OpenCode / OpenClaw 共用</small>
          </article>
          <article class="advanced-proxy-summary-card">
            <span>熔断打开数</span>
            <strong>{{ openCircuitCount }}</strong>
            <small>故障转移 {{ draft.failover.enabled ? '已开启' : '未开启' }}</small>
          </article>
        </section>

        <div class="advanced-proxy-layout">
          <section class="advanced-proxy-section">
            <div class="advanced-proxy-section-head">
              <div>
                <h4>上游 Provider 队列</h4>
                <p>点击卡片即可加入 / 移出队列，队列优先级按点击顺序自动更新。详细的 Key 和端点地址放在 tooltip 中。</p>
              </div>
              <a-button @click="reloadContext">刷新记录</a-button>
            </div>

            <div class="advanced-proxy-provider-pool">
              <div class="advanced-proxy-provider-panel-grid">
                <a-tooltip v-for="item in providerCandidateCards" :key="item.id" placement="topLeft">
                  <template #title>
                    <div class="advanced-proxy-provider-tooltip">
                      <strong>{{ item.siteName }}</strong>
                      <span>{{ item.modelLabel }}</span>
                      <span v-if="item.skLabel">{{ item.skLabel }}</span>
                      <code>{{ item.endpoint || '-' }}</code>
                      <code>{{ item.apiKey || '-' }}</code>
                    </div>
                  </template>

                  <button
                    type="button"
                    class="advanced-proxy-provider-panel"
                    :class="{ 'advanced-proxy-provider-panel-active': item.selected }"
                    @click="toggleProviderQueue(item)"
                  >
                    <div class="advanced-proxy-provider-panel-top">
                      <strong class="advanced-proxy-provider-panel-title">{{ item.siteName }}</strong>
                      <span v-if="item.queueOrder" class="advanced-proxy-provider-order">P{{ item.queueOrder }}</span>
                    </div>
                    <div class="advanced-proxy-provider-panel-model">{{ item.modelLabel }}</div>
                    <div class="advanced-proxy-provider-panel-meta">
                      <span v-if="item.skLabel" class="advanced-proxy-provider-chip">{{ item.skLabel }}</span>
                      <span v-if="item.orphaned" class="advanced-proxy-provider-chip advanced-proxy-provider-chip-muted">已不在密钥管理中</span>
                    </div>
                  </button>
                </a-tooltip>
              </div>

              <div v-if="providerCandidateCards.length && !providerCount" class="advanced-proxy-empty advanced-proxy-empty-compact">
                点击卡片加入队列，队列优先级按点击顺序自动更新。
              </div>
            </div>

            <div v-if="!providerCandidateCards.length" class="advanced-proxy-empty">
              还没有可用 Provider。先从密钥管理中加入至少一条记录，再在这里点击卡片组成队列。
            </div>
          </section>

          <aside class="advanced-proxy-side">
            <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <div>
                  <h4>故障转移</h4>
                  <p>所有接管应用共享同一条上游队列，并按当前顺序执行重试与熔断。</p>
                </div>
              </div>

              <div class="advanced-proxy-inline-grid">
                <div class="advanced-proxy-inline-control">
                  <span class="advanced-proxy-inline-label">启用故障转移</span>
                  <a-switch
                    :checked="draft.failover.enabled"
                    @change="value => handleConfigMutation(next => { next.failover.enabled = value; }, '故障转移开关已更新')"
                  />
                </div>
                <div class="advanced-proxy-inline-control">
                  <span class="advanced-proxy-inline-label">启用自动切换</span>
                  <a-switch
                    :checked="draft.failover.autoFailoverEnabled"
                    @change="value => handleConfigMutation(next => { next.failover.autoFailoverEnabled = value; }, '自动切换开关已更新')"
                  />
                </div>
              </div>

              <div class="advanced-proxy-dense-rows">
                <div class="advanced-proxy-triple-row">
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">最大重试次数</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.maxRetries" :min="0" :max="10" @change="value => handleFailoverFieldMutation('maxRetries', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">流式首字节超时</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.streamingFirstByteTimeout" :min="5" :max="300" @change="value => handleFailoverFieldMutation('streamingFirstByteTimeout', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">流式空闲超时</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.streamingIdleTimeout" :min="5" :max="600" @change="value => handleFailoverFieldMutation('streamingIdleTimeout', value)" />
                  </div>
                </div>
                <div class="advanced-proxy-triple-row">
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">非流式超时</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.nonStreamingTimeout" :min="5" :max="600" @change="value => handleFailoverFieldMutation('nonStreamingTimeout', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">熔断失败阈值</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitFailureThreshold" :min="1" :max="20" @change="value => handleFailoverFieldMutation('circuitFailureThreshold', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">恢复成功阈值</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitSuccessThreshold" :min="1" :max="20" @change="value => handleFailoverFieldMutation('circuitSuccessThreshold', value)" />
                  </div>
                </div>
                <div class="advanced-proxy-triple-row">
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">熔断恢复等待</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitTimeoutSeconds" :min="5" :max="600" @change="value => handleFailoverFieldMutation('circuitTimeoutSeconds', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">错误率阈值</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitErrorRateThreshold" :min="0.1" :max="1" :step="0.05" @change="value => handleFailoverFieldMutation('circuitErrorRateThreshold', value)" />
                  </div>
                  <div class="advanced-proxy-compact-field">
                    <label class="advanced-proxy-compact-label">最小请求数</label>
                    <a-input-number class="advanced-proxy-short-number" :value="draft.failover.circuitMinRequests" :min="1" :max="100" @change="value => handleFailoverFieldMutation('circuitMinRequests', value)" />
                  </div>
                </div>
              </div>
            </section>
            <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <div>
                  <h4>错误修正</h4>
                  <p>仅作用于 Claude 兼容链路，用来最小化修正常见的 thinking 签名和预算错误。</p>
                </div>
              </div>

              <div class="advanced-proxy-toggle-list">
                <div class="advanced-proxy-toggle-row">
                  <span>总开关</span>
                  <a-switch
                    :checked="draft.rectifier.enabled"
                    @change="value => handleConfigMutation(next => { next.rectifier.enabled = value; }, '错误修正总开关已更新')"
                  />
                </div>
                <div class="advanced-proxy-toggle-row">
                  <span>修正 thinking signature</span>
                  <a-switch
                    :checked="draft.rectifier.requestThinkingSignature"
                    @change="value => handleConfigMutation(next => { next.rectifier.requestThinkingSignature = value; }, 'thinking signature 修正开关已更新')"
                  />
                </div>
                <div class="advanced-proxy-toggle-row">
                  <span>修正 thinking budget</span>
                  <a-switch
                    :checked="draft.rectifier.requestThinkingBudget"
                    @change="value => handleConfigMutation(next => { next.rectifier.requestThinkingBudget = value; }, 'thinking budget 修正开关已更新')"
                  />
                </div>
              </div>
            </section>

            <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <h4>使用说明</h4>
              </div>
              <ul class="advanced-proxy-notes">
                <li><code>claude</code> 入口会把 Anthropic Messages 请求转换到上游 Provider 定义的格式。</li>
                <li><code>codex</code> / <code>opencode</code> / <code>openclaw</code> 入口会直接代理 OpenAI 兼容请求，并使用同一份上游追踪配置。</li>
                <li>接管打开后，本地应用配置会写入本地代理地址；真实反代目标只保存在这里的 Provider 列表中。</li>
                <li>如果要接管 Codex，请至少准备一条可用的 OpenAI 兼容上游，最好支持 <code>/v1/responses</code>。</li>
              </ul>
            </section>
          </aside>
        </div>
      </div>
    </a-spin>
  </a-modal>

  <DesktopConfigDiffModal :open="previewOpen" :preview="configPreview" @cancel="cancelPreview" @confirm="applyPreview" />
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { message } from 'ant-design-vue';
import claudeAppIcon from '../assets/app-icons/claude.svg';
import codexAppIcon from '../assets/app-icons/codex.svg';
import opencodeAppIcon from '../assets/app-icons/opencode.svg';
import openclawAppIcon from '../assets/app-icons/openclaw-fallback.svg';
import DesktopConfigDiffModal from './DesktopConfigDiffModal.vue';
import { applyManagedAppConfigFiles, isDesktopConfigBridgeAvailable, readManagedAppConfigFiles } from '../utils/desktopConfigBridge.js';
import { buildDesktopConfigPreview, createDesktopConfigDraft } from '../utils/desktopConfigTransform.js';
import { loadPanelRecords } from '../utils/keyPanelStore.js';
import {
  ADVANCED_PROXY_APPS,
  countAdvancedProxyEnabledProviders,
  countAdvancedProxyOpenAIProviders,
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyConfig,
  getCircuitBreakerStats,
  normalizeAdvancedProxyConfig,
  resetCircuitBreaker,
  setAdvancedProxyConfig,
} from '../utils/advancedProxyBridge.js';

const EMPTY_PREVIEW = { appGroups: [], writes: [], errors: [] };
const modalWidth = 'min(1180px, calc(100vw - 24px))';
const PROXY_MANAGED_TOKEN = 'PROXY_MANAGED';
const ADVANCED_PROXY_PROVIDER_NAME = 'AllApiDeck Advanced Proxy';
const ADVANCED_PROXY_APP_ICONS = {
  claude: claudeAppIcon,
  codex: codexAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
};

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(['update:open']);

const loading = ref(false);
const saving = ref(false);
const previewOpen = ref(false);
const availableRecords = ref([]);
const breakerStatsMap = ref({});
const loadedConfigSnapshot = ref(normalizeAdvancedProxyConfig({}));
const pendingSaveConfig = ref(null);
const pendingManagedWrites = ref([]);
const pendingWriteOrder = ref('config-first');
const pendingSuccessMessage = ref('高级代理配置已更新');
const configPreview = ref(EMPTY_PREVIEW);
const lastEnabledAppIds = ref([]);
const draft = reactive(normalizeAdvancedProxyConfig({}));

const providerCount = computed(() => draft.claude.providers.length);
const enabledProviderCount = computed(() => countAdvancedProxyEnabledProviders(draft));
const openAIProviderCount = computed(() => countAdvancedProxyOpenAIProviders(draft));
const enabledAppIds = computed(() => getEnabledAppIds(draft));
const enabledAppCount = computed(() => enabledAppIds.value.length);
const enabledAppLabels = computed(() =>
  ADVANCED_PROXY_APPS
    .filter(app => enabledAppIds.value.includes(app.id))
    .map(app => app.label)
    .join(' / ')
);
const proxyMasterEnabled = computed(() => enabledAppIds.value.length > 0);
const openCircuitCount = computed(() => draft.claude.providers.filter(provider => getBreakerStateLabel(provider.id) === 'open').length);
const providerSelectionMap = computed(() => {
  const map = new Map();
  (draft.claude.providers || []).forEach((provider, index) => {
    const id = String(provider?.id || provider?.rowKey || '').trim();
    if (!id) return;
    map.set(id, {
      order: index + 1,
      provider,
    });
  });
  return map;
});

const providerCandidateCards = computed(() => {
  const duplicateMeta = buildProviderDuplicateMeta(availableRecords.value);
  const cards = availableRecords.value.map(record => {
    const id = String(record?.rowKey || '').trim();
    const selectedMeta = providerSelectionMap.value.get(id) || null;
    const duplicate = duplicateMeta.get(id) || { index: 0, count: 0 };
    return {
      id,
      siteName: String(record?.siteName || record?.siteUrl || 'Provider').trim() || 'Provider',
      modelLabel: String(record?.selectedModel || record?.quickTestModel || selectedMeta?.provider?.model || '未设置模型').trim() || '未设置模型',
      endpoint: String(record?.siteUrl || selectedMeta?.provider?.baseUrl || '').trim(),
      apiKey: String(record?.apiKey || selectedMeta?.provider?.apiKey || '').trim(),
      skLabel: duplicate.count > 1 ? `SK ${duplicate.index}` : '',
      selected: Boolean(selectedMeta),
      queueOrder: selectedMeta?.order || 0,
      orphaned: false,
      sortTime: Number(record?.updatedAt || 0),
      sourceRecord: record,
    };
  });

  providerSelectionMap.value.forEach((meta, id) => {
    if (cards.some(item => item.id === id)) return;
    cards.push({
      id,
      siteName: String(meta?.provider?.name || meta?.provider?.baseUrl || 'Provider').trim() || 'Provider',
      modelLabel: String(meta?.provider?.model || '未设置模型').trim() || '未设置模型',
      endpoint: String(meta?.provider?.baseUrl || '').trim(),
      apiKey: String(meta?.provider?.apiKey || '').trim(),
      skLabel: '',
      selected: true,
      queueOrder: meta?.order || 0,
      orphaned: true,
      sortTime: 0,
      sourceRecord: null,
    });
  });

  return cards.sort((left, right) => {
    if (left.selected && right.selected) {
      return left.queueOrder - right.queueOrder;
    }
    if (left.selected) return -1;
    if (right.selected) return 1;
    return Number(right.sortTime || 0) - Number(left.sortTime || 0)
      || String(left.siteName || '').localeCompare(String(right.siteName || ''));
  });
});

const appCards = computed(() =>
  ADVANCED_PROXY_APPS.map(app => {
    const enabled = draft?.[app.id]?.enabled === true;
    return {
      ...app,
      enabled,
      icon: ADVANCED_PROXY_APP_ICONS[app.id],
      modeLabel: app.mode === 'anthropic' ? 'Anthropic Messages 入口' : 'OpenAI Compatible 入口',
      tooltipDetail: app.id === 'claude'
        ? '已支持：Claude 客户端 -> 8888 -> OpenAI 上游。Claude 请求会自动转成 OpenAI 上游请求，并把返回结果转回 Claude 格式。'
        : '',
    };
  })
);

function toPlainValue(value) {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function normalizeForSave(config) {
  const next = normalizeAdvancedProxyConfig(toPlainValue(config));
  delete next.updatedAt;
  return next;
}

function overwriteDraft(nextConfig) {
  const normalized = normalizeAdvancedProxyConfig(nextConfig);
  Object.keys(draft).forEach(key => delete draft[key]);
  Object.assign(draft, normalized);
}

function createPendingConfig(source = draft) {
  const plainDraft = toPlainValue(source);
  plainDraft.claude.providers = (plainDraft.claude?.providers || []).map((provider, index) => ({
    ...provider,
    sortIndex: index + 1,
  }));
  return normalizeAdvancedProxyConfig(plainDraft);
}

function buildProviderDuplicateMeta(records) {
  const buckets = new Map();
  (Array.isArray(records) ? records : []).forEach(record => {
    const key = `${String(record?.siteUrl || '').trim().toLowerCase()}|${String(record?.siteName || '').trim().toLowerCase()}`;
    if (!buckets.has(key)) {
      buckets.set(key, []);
    }
    buckets.get(key).push(record);
  });

  const meta = new Map();
  buckets.forEach(group => {
    group.forEach((record, index) => {
      meta.set(String(record?.rowKey || '').trim(), {
        index: index + 1,
        count: group.length,
      });
    });
  });
  return meta;
}

function buildProviderFromRecord(record, sortIndex) {
  return {
    id: record.rowKey,
    rowKey: record.rowKey,
    name: record.siteName || record.siteUrl || 'Provider',
    baseUrl: record.siteUrl,
    apiKey: record.apiKey,
    model: record.selectedModel || record.quickTestModel || '',
    apiFormat: 'openai_chat',
    apiKeyField: 'ANTHROPIC_AUTH_TOKEN',
    enabled: true,
    sortIndex,
    sourceType: record.sourceType || 'auto',
  };
}

function getEnabledAppIds(source = draft) {
  return ADVANCED_PROXY_APPS
    .filter(app => source?.[app.id]?.enabled === true)
    .map(app => app.id);
}

function hasConfigChanges(nextConfig) {
  const beforeText = JSON.stringify(normalizeForSave(loadedConfigSnapshot.value));
  const afterText = JSON.stringify(normalizeForSave(nextConfig));
  return beforeText !== afterText;
}

async function syncSavedConfig(savedConfig) {
  loadedConfigSnapshot.value = normalizeAdvancedProxyConfig(savedConfig);
  overwriteDraft(loadedConfigSnapshot.value);
  breakerStatsMap.value = {};
  await Promise.all((draft.claude.providers || []).map(provider => reloadProviderStats(provider.id)));
}

async function saveConfigImmediately(nextConfig, successMessage = '高级代理配置已更新') {
  if (!hasConfigChanges(nextConfig)) {
    message.info('当前没有需要写入的配置变更');
    return false;
  }

  saving.value = true;
  try {
    const saved = await setAdvancedProxyConfig(createPendingConfig(nextConfig));
    await syncSavedConfig(saved);
    message.success(successMessage);
    return true;
  } catch (error) {
    message.error(error?.message || '写入高级代理配置失败');
    return false;
  } finally {
    saving.value = false;
  }
}

function openPreviewForManagedWrites(nextConfig, desktopPreview, successMessage = '高级代理配置已更新', options = {}) {
  if (!hasConfigChanges(nextConfig)) {
    message.info('当前没有需要写入的配置变更');
    return;
  }

  const managedWrites = Array.isArray(desktopPreview?.writes) ? desktopPreview.writes : [];
  if (!managedWrites.length) {
    saveConfigImmediately(nextConfig, successMessage);
    return;
  }

  pendingSaveConfig.value = createPendingConfig(nextConfig);
  pendingManagedWrites.value = managedWrites;
  pendingWriteOrder.value = options.writeOrder === 'managed-first' ? 'managed-first' : 'config-first';
  pendingSuccessMessage.value = successMessage;
  configPreview.value = desktopPreview || EMPTY_PREVIEW;
  previewOpen.value = true;
}

async function handleConfigMutation(mutator, successMessage) {
  if (saving.value) return;
  const nextConfig = createPendingConfig();
  mutator(nextConfig);
  await saveConfigImmediately(nextConfig, successMessage);
}

function getCompatibleProviderForApp(config, appId, enabledOnly = true) {
  const providers = Array.isArray(config?.claude?.providers) ? config.claude.providers : [];
  const filtered = enabledOnly ? providers.filter(provider => provider?.enabled !== false) : providers;
  if (appId === 'claude') {
    return filtered[0] || null;
  }
  return filtered.find(provider => provider?.apiFormat === 'openai_chat' || provider?.apiFormat === 'openai_responses') || null;
}

function getPreferredModelForApp(config, appId, provider = null) {
  const directModel = String(provider?.model || '').trim();
  if (directModel) {
    return directModel;
  }
  const defaultModel = String(config?.claude?.defaultModel || '').trim();
  if (defaultModel) {
    return defaultModel;
  }
  const providers = Array.isArray(config?.claude?.providers) ? config.claude.providers : [];
  const compatibleProvider = getCompatibleProviderForApp(config, appId, false);
  return String(compatibleProvider?.model || providers.find(item => String(item?.model || '').trim())?.model || '').trim();
}

function createTakeoverDesktopDraft(appId, enabled, config) {
  const sourceProvider = getCompatibleProviderForApp(config, appId, true);
  const model = getPreferredModelForApp(config, appId, sourceProvider);
  if (!model) {
    throw new Error('请先给 Provider 补一个模型，再启用该应用接管');
  }

  if (!enabled && !sourceProvider) {
    throw new Error(appId === 'claude'
      ? '当前没有可回退的 Claude 上游 Provider'
      : '当前没有可回退的 OpenAI 兼容上游 Provider');
  }

  const endpoint = enabled ? getAdvancedProxyAppBaseUrl(appId, config) : String(sourceProvider?.baseUrl || '').trim();
  const apiKey = enabled ? PROXY_MANAGED_TOKEN : String(sourceProvider?.apiKey || '').trim();
  const providerName = enabled ? ADVANCED_PROXY_PROVIDER_NAME : String(sourceProvider?.name || 'Custom Provider').trim();

  if (!endpoint) {
    throw new Error('缺少可写入的目标地址');
  }
  if (!apiKey) {
    throw new Error('缺少可写入的 API Key');
  }

  const nextDraft = createDesktopConfigDraft({
    siteName: providerName,
    siteUrl: endpoint,
    apiKey,
    selectedModel: model,
    quickTestModel: model,
  });

  nextDraft.selectedApps = [appId];
  nextDraft.providerName = providerName;
  nextDraft.providerKey = 'custom';
  nextDraft.forceCustomProviderKey = true;
  nextDraft.endpoint = endpoint;
  nextDraft.apiKey = apiKey;
  nextDraft.model = model;
  nextDraft.claudeBaseUrl = appId === 'claude' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.claudeApiKeyField = enabled ? 'ANTHROPIC_AUTH_TOKEN' : String(sourceProvider?.apiKeyField || 'ANTHROPIC_AUTH_TOKEN').trim();
  nextDraft.codexBaseUrl = appId === 'codex' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.opencodeBaseUrl = appId === 'opencode' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.openclawBaseUrl = appId === 'openclaw' ? endpoint : String(sourceProvider?.baseUrl || endpoint).trim();
  nextDraft.claudeUseAdvancedProxy = false;
  nextDraft.codexUseAdvancedProxy = false;
  nextDraft.opencodeUseAdvancedProxy = false;
  nextDraft.openclawUseAdvancedProxy = false;
  return nextDraft;
}

async function handleAppTakeoverToggle(appId, value) {
  if (saving.value || loading.value) return;
  if (!isDesktopConfigBridgeAvailable()) {
    message.warning('高级代理接管仅支持桌面版 EXE 运行环境');
    return;
  }

  const app = ADVANCED_PROXY_APPS.find(item => item.id === appId);
  const nextConfig = createPendingConfig();
  if (!nextConfig[appId]) {
    nextConfig[appId] = {};
  }
  nextConfig[appId].enabled = value;

  try {
    const desktopDraft = createTakeoverDesktopDraft(appId, value, nextConfig);
    const snapshot = await readManagedAppConfigFiles([appId]);
    const desktopPreview = buildDesktopConfigPreview(desktopDraft, snapshot);
    if (!desktopPreview.appGroups.length && desktopPreview.errors.length) {
      throw new Error(desktopPreview.errors.join('\n'));
    }

    openPreviewForManagedWrites(nextConfig, desktopPreview, `${app?.label || appId} 接管配置已更新`, {
      writeOrder: value ? 'config-first' : 'managed-first',
    });
  } catch (error) {
    message.error(error?.message || `${app?.label || appId} 接管预览生成失败`);
  }
}

function handleProxyMasterToggle(value) {
  if (value) {
    handleConfigMutation(next => {
      const restoreIds = lastEnabledAppIds.value.length ? [...lastEnabledAppIds.value] : ['claude'];
      ADVANCED_PROXY_APPS.forEach(app => {
        if (!next[app.id]) {
          next[app.id] = {};
        }
        next[app.id].enabled = restoreIds.includes(app.id);
      });
    }, '代理总开关已更新');
    return;
  }

  const currentEnabledIds = getEnabledAppIds(draft);
  if (currentEnabledIds.length) {
    lastEnabledAppIds.value = [...currentEnabledIds];
  }
  handleConfigMutation(next => {
    ADVANCED_PROXY_APPS.forEach(app => {
      if (!next[app.id]) {
        next[app.id] = {};
      }
      next[app.id].enabled = false;
    });
  }, '代理总开关已更新');
}

function handleFailoverFieldMutation(field, value) {
  if (value == null || value === '') return;
  handleConfigMutation(next => {
    next.failover[field] = value;
  }, '故障转移配置已更新');
}

async function reloadContext() {
  const { records } = loadPanelRecords();
  availableRecords.value = records;
}

async function loadData() {
  loading.value = true;
  try {
    await reloadContext();
    const config = await getAdvancedProxyConfig();
    await syncSavedConfig(config);
  } catch (error) {
    message.error(error?.message || '加载高级代理配置失败');
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.open,
  async value => {
    if (value) {
      await loadData();
    }
  },
  { immediate: true }
);

watch(
  enabledAppIds,
  ids => {
    if (ids.length) {
      lastEnabledAppIds.value = [...ids];
    }
  },
  { immediate: true }
);

function toggleProviderQueue(item) {
  const providerId = String(item?.id || '').trim();
  if (!providerId) return;

  handleConfigMutation(next => {
    const list = [...(next.claude?.providers || [])];
    const existingIndex = list.findIndex(provider => String(provider?.id || provider?.rowKey || '').trim() === providerId);

    if (existingIndex >= 0) {
      list.splice(existingIndex, 1);
    } else if (item?.sourceRecord) {
      list.push(buildProviderFromRecord(item.sourceRecord, list.length + 1));
    }

    next.claude.providers = list.map((provider, index) => ({
      ...provider,
      enabled: true,
      sortIndex: index + 1,
    }));
  }, item?.selected ? 'Provider 已移出队列' : 'Provider 已加入队列');
}

function getBreakerStats(providerId) {
  return breakerStatsMap.value[providerId] || {};
}

function getBreakerStateLabel(providerId) {
  const state = String(getBreakerStats(providerId)?.state || 'closed').trim();
  if (state === 'half_open') return 'half_open';
  if (state === 'open') return 'open';
  return 'closed';
}

function breakerStateColor(providerId) {
  const state = getBreakerStateLabel(providerId);
  if (state === 'open') return 'red';
  if (state === 'half_open') return 'orange';
  return 'green';
}

async function reloadProviderStats(providerId) {
  if (!providerId) return;
  try {
    const stats = await getCircuitBreakerStats('claude', providerId);
    breakerStatsMap.value = {
      ...breakerStatsMap.value,
      [providerId]: stats || {},
    };
  } catch (error) {
    console.warn('[AdvancedProxy] reload breaker stats failed:', error);
  }
}

async function resetProviderBreaker(providerId) {
  try {
    await resetCircuitBreaker('claude', providerId);
    await reloadProviderStats(providerId);
    message.success('已重置该 Provider 的熔断状态');
  } catch (error) {
    message.error(error?.message || '重置熔断失败');
  }
}

async function applyPreview() {
  if (!pendingSaveConfig.value) {
    message.warning('没有待写入的配置变更');
    return;
  }

  saving.value = true;
  try {
    if (pendingManagedWrites.value.length && pendingWriteOrder.value === 'managed-first') {
      await applyManagedAppConfigFiles(pendingManagedWrites.value);
    }

    const saved = await setAdvancedProxyConfig(pendingSaveConfig.value);

    if (pendingManagedWrites.value.length && pendingWriteOrder.value !== 'managed-first') {
      await applyManagedAppConfigFiles(pendingManagedWrites.value);
    }

    await syncSavedConfig(saved);
    previewOpen.value = false;
    configPreview.value = EMPTY_PREVIEW;
    pendingSaveConfig.value = null;
    pendingManagedWrites.value = [];
    pendingWriteOrder.value = 'config-first';
    message.success(pendingSuccessMessage.value || '高级代理配置已更新');
  } catch (error) {
    message.error(error?.message || '写入高级代理配置失败');
  } finally {
    saving.value = false;
  }
}

function cancelPreview() {
  previewOpen.value = false;
  pendingSaveConfig.value = null;
  pendingManagedWrites.value = [];
  pendingWriteOrder.value = 'config-first';
  configPreview.value = EMPTY_PREVIEW;
}

function handleCancel() {
  cancelPreview();
  emit('update:open', false);
}
</script>

<style scoped>
.advanced-proxy-shell {
  display: grid;
  gap: 10px;
}

.advanced-proxy-hero,
.advanced-proxy-summary-grid,
.advanced-proxy-layout,
.advanced-proxy-provider-grid,
.advanced-proxy-inline-grid,
.advanced-proxy-dense-grid,
.advanced-proxy-toggle-list {
  display: grid;
  gap: 8px;
}

.advanced-proxy-hero {
  grid-template-columns: 1fr;
  padding: 12px 14px;
  border-radius: 18px;
  border: 1px solid rgba(90, 117, 79, 0.14);
  background:
    radial-gradient(circle at top right, rgba(208, 230, 193, 0.3), transparent 36%),
    linear-gradient(135deg, rgba(250, 252, 247, 0.98), rgba(239, 247, 232, 0.94));
}

.advanced-proxy-hero-copy,
.advanced-proxy-side,
.advanced-proxy-provider-list,
.advanced-proxy-section {
  display: grid;
  gap: 8px;
}

.advanced-proxy-hero-copy h3,
.advanced-proxy-section-head h4 {
  margin: 0;
  color: #22311c;
  font-size: 16px;
  line-height: 1.3;
}

.advanced-proxy-hero-copy p,
.advanced-proxy-section-head p,
.advanced-proxy-provider-meta,
.advanced-proxy-provider-stats,
.advanced-proxy-notes,
.advanced-proxy-master-copy small {
  margin: 0;
  color: #6a7867;
  font-size: 11px;
  line-height: 1.45;
}

.advanced-proxy-master-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 10px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.84);
}

.advanced-proxy-master-copy {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.advanced-proxy-master-copy strong,
.advanced-proxy-summary-card strong,
.advanced-proxy-provider-name {
  color: #22311c;
  font-size: 13px;
  font-weight: 700;
}

.advanced-proxy-app-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.advanced-proxy-provider-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-summary-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.advanced-proxy-summary-card,
.advanced-proxy-section,
.advanced-proxy-provider-card,
.advanced-proxy-inline-control,
.advanced-proxy-compact-field,
.advanced-proxy-toggle-row {
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.84);
}

.advanced-proxy-app-token,
.advanced-proxy-provider-head,
.advanced-proxy-provider-title,
.advanced-proxy-provider-actions,
.advanced-proxy-provider-tags,
.advanced-proxy-toggle-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-proxy-provider-head,
.advanced-proxy-toggle-row {
  justify-content: space-between;
}

.advanced-proxy-section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.advanced-proxy-section-head > div {
  min-width: 0;
  flex: 1;
}

.advanced-proxy-provider-pool {
  display: grid;
  gap: 8px;
}

.advanced-proxy-provider-panel-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 10px;
}

.advanced-proxy-provider-panel {
  appearance: none;
  width: 100%;
  min-width: 0;
  min-height: 90px;
  display: grid;
  align-content: start;
  gap: 7px;
  padding: 11px 12px;
  border-radius: 16px;
  border: 1px solid rgba(90, 117, 79, 0.15);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(248, 250, 246, 0.92));
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.advanced-proxy-provider-panel:hover {
  border-color: rgba(88, 125, 66, 0.24);
  box-shadow: 0 12px 24px rgba(74, 104, 58, 0.08);
  transform: translateY(-1px);
}

.advanced-proxy-provider-panel-active {
  border-color: rgba(75, 128, 50, 0.34);
  box-shadow:
    0 0 0 1px rgba(102, 168, 68, 0.12),
    0 0 0 4px rgba(147, 210, 109, 0.12),
    0 14px 28px rgba(74, 104, 58, 0.12);
  background: linear-gradient(180deg, rgba(252, 255, 249, 0.98), rgba(242, 248, 236, 0.96));
}

.advanced-proxy-provider-panel-top,
.advanced-proxy-provider-panel-meta,
.advanced-proxy-provider-tooltip {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-proxy-provider-panel-top {
  justify-content: space-between;
}

.advanced-proxy-provider-panel-title {
  min-width: 0;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.25;
  color: #22311c;
}

.advanced-proxy-provider-panel-model {
  min-width: 0;
  color: #5f6e5a;
  font-size: 11px;
  line-height: 1.35;
  word-break: break-word;
}

.advanced-proxy-provider-chip {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(79, 108, 62, 0.08);
  color: #355029;
  font-size: 10px;
  font-weight: 600;
}

.advanced-proxy-provider-chip-muted {
  background: rgba(108, 122, 101, 0.08);
  color: #66725f;
}

.advanced-proxy-provider-tooltip {
  flex-direction: column;
  align-items: flex-start;
}

.advanced-proxy-app-tooltip {
  display: grid;
  gap: 4px;
  max-width: 320px;
  line-height: 1.45;
}

.advanced-proxy-provider-tooltip code {
  max-width: 340px;
  white-space: pre-wrap;
  word-break: break-all;
}

.advanced-proxy-empty-compact {
  padding: 11px 12px;
}

.advanced-proxy-app-token {
  appearance: none;
  width: 100%;
  min-width: 0;
  min-height: 58px;
  justify-content: center;
  padding: 8px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.88);
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease, background 0.18s ease;
}

.advanced-proxy-app-token:hover {
  border-color: rgba(72, 113, 54, 0.28);
  box-shadow: 0 10px 22px rgba(74, 104, 58, 0.08);
  transform: translateY(-1px);
}

.advanced-proxy-app-token-active {
  border-color: rgba(67, 113, 49, 0.28);
  background: linear-gradient(135deg, rgba(250, 252, 247, 0.98), rgba(236, 246, 228, 0.96));
  box-shadow: 0 12px 24px rgba(74, 104, 58, 0.1);
}

.advanced-proxy-app-token-active .advanced-proxy-app-icon-shell {
  box-shadow: inset 0 0 0 1px rgba(57, 94, 41, 0.1);
}

.advanced-proxy-app-icon-shell {
  width: 36px;
  height: 36px;
  border-radius: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px;
  box-shadow: inset 0 0 0 1px rgba(90, 117, 79, 0.08);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(242, 247, 236, 0.92));
}

.advanced-proxy-app-icon-shell-claude {
  background: linear-gradient(135deg, #fff7ed, #ffedd5);
}

.advanced-proxy-app-icon-shell-codex {
  background: linear-gradient(135deg, #ffffff, #f3f4f6);
}

.advanced-proxy-app-icon-shell-opencode {
  background: linear-gradient(135deg, #eef2ff, #dbeafe);
}

.advanced-proxy-app-icon-shell-openclaw {
  background: linear-gradient(135deg, #fff1f2, #ffe4e6);
}

.advanced-proxy-app-icon-image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
}

.advanced-proxy-summary-card,
.advanced-proxy-provider-card,
.advanced-proxy-section {
  display: grid;
  gap: 6px;
  padding: 9px 10px;
}

.advanced-proxy-summary-card {
  align-content: start;
  min-height: 84px;
}

.advanced-proxy-summary-card span,
.advanced-proxy-summary-card small {
  font-size: 11px;
  line-height: 1.35;
}

.advanced-proxy-layout {
  grid-template-columns: minmax(0, 1.42fr) minmax(420px, 1.08fr);
  align-items: start;
  gap: 10px;
}

.advanced-proxy-empty {
  padding: 14px 12px;
  border-radius: 14px;
  border: 1px dashed rgba(90, 117, 79, 0.28);
  color: #6a7965;
  background: rgba(247, 250, 244, 0.9);
  font-size: 11px;
  line-height: 1.5;
}

.advanced-proxy-provider-order {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 24px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(60, 103, 39, 0.12);
  color: #2c4a1f;
  font-size: 10px;
  font-weight: 700;
}

.advanced-proxy-provider-head {
  align-items: flex-start;
}

.advanced-proxy-provider-actions {
  justify-content: flex-end;
}

.advanced-proxy-provider-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-provider-grid :deep(.ant-form-item) {
  margin-bottom: 4px;
}

.advanced-proxy-provider-stats {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 11px;
}

.advanced-proxy-inline-grid,
.advanced-proxy-toggle-list {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-dense-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.advanced-proxy-dense-rows {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.advanced-proxy-triple-row {
  display: contents;
}

.advanced-proxy-triple-row > .advanced-proxy-compact-field {
  width: auto;
  min-width: 0;
  justify-self: stretch;
}

.advanced-proxy-inline-control,
.advanced-proxy-compact-field,
.advanced-proxy-toggle-row {
  padding: 8px 10px;
}

.advanced-proxy-inline-control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.advanced-proxy-toggle-list {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.advanced-proxy-toggle-row span,
.advanced-proxy-inline-label {
  flex: 1;
  min-width: 0;
}

.advanced-proxy-inline-label,
.advanced-proxy-compact-label {
  color: #22311c;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.3;
}

.advanced-proxy-compact-field {
  display: grid;
  gap: 4px;
  padding: 10px 10px;
}

.advanced-proxy-compact-field :deep(.ant-input-number) {
  border-radius: 9px;
}

.advanced-proxy-short-number {
  width: 100%;
  min-width: 0;
}

.advanced-proxy-notes {
  padding-left: 16px;
  font-size: 11px;
  line-height: 1.45;
}

.advanced-proxy-section :deep(.ant-input),
.advanced-proxy-section :deep(.ant-input-password),
.advanced-proxy-section :deep(.ant-input-affix-wrapper),
.advanced-proxy-section :deep(.ant-select-selector),
.advanced-proxy-section :deep(.ant-input-number),
.advanced-proxy-section :deep(.ant-btn) {
  min-height: 28px;
  font-size: 11px;
}

.advanced-proxy-section :deep(.ant-form-item-label > label) {
  font-size: 10px;
  line-height: 1.2;
}

.advanced-proxy-section :deep(.ant-select-single:not(.ant-select-customize-input) .ant-select-selector) {
  height: 28px;
}

.advanced-proxy-section :deep(.ant-select-single .ant-select-selector .ant-select-selection-item),
.advanced-proxy-section :deep(.ant-select-single .ant-select-selector .ant-select-selection-placeholder) {
  line-height: 26px;
}

.advanced-proxy-section :deep(.ant-input-number-input) {
  height: 26px;
}

@media (max-width: 1180px) {
  .advanced-proxy-layout {
    grid-template-columns: 1fr;
  }

  .advanced-proxy-summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 760px) {
  .advanced-proxy-toggle-list {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .advanced-proxy-dense-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 620px) {
  .advanced-proxy-summary-grid,
  .advanced-proxy-provider-grid,
  .advanced-proxy-inline-grid,
  .advanced-proxy-dense-grid,
  .advanced-proxy-toggle-list {
    grid-template-columns: 1fr;
  }

  .advanced-proxy-dense-rows {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .advanced-proxy-provider-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 560px) {
  .advanced-proxy-provider-panel-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .advanced-proxy-app-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 480px) {
  .advanced-proxy-provider-panel-grid,
  .advanced-proxy-dense-rows {
    grid-template-columns: 1fr;
  }
}
</style>
