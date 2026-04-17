<template>
  <a-modal :open="open" title="高级代理功能" :width="1260" :footer="null" @cancel="handleCancel">
    <a-spin :spinning="loading || saving">
      <div class="advanced-proxy-shell">
        <section class="advanced-proxy-hero">
          <div class="advanced-proxy-hero-copy">
            <a-tag color="green">本地代理接管</a-tag>
            <h3>兼容 OpenAI vendor，并为 Claude / Codex / OpenCode / OpenClaw 提供统一接管入口</h3>
            <p>这里没有“保存配置”按钮。点任何开关、字段或 Provider 操作时，都会先生成 diff 预览；确认后立即写入，而且弹窗不会自动关闭。</p>
          </div>

          <div class="advanced-proxy-app-grid">
            <article
              v-for="app in appCards"
              :key="app.id"
              class="advanced-proxy-app-card"
              :class="{ 'advanced-proxy-app-card-active': app.enabled }"
            >
              <div class="advanced-proxy-app-head">
                <div class="advanced-proxy-app-copy">
                  <strong>{{ app.label }}</strong>
                  <small>{{ app.modeLabel }}</small>
                </div>
                <a-switch
                  :checked="app.enabled"
                  checked-children="开启"
                  un-checked-children="关闭"
                  @change="value => handleAppToggleMutation(app.id, value)"
                />
              </div>
              <code>{{ app.baseUrl }}</code>
              <div class="advanced-proxy-app-meta">
                <span>{{ app.readyText }}</span>
                <a-button size="small" @click="copyText(app.baseUrl, `${app.label} 代理地址已复制`)">复制</a-button>
              </div>
            </article>
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

        <a-alert type="info" show-icon>
          <template #message>本地配置写入全程可见</template>
          <template #description>
            当前保存目标：<code>{{ configFilePath }}</code>。任何修改都会先弹出 diff 窗口，确认后立刻生效。
          </template>
        </a-alert>

        <div class="advanced-proxy-layout">
          <section class="advanced-proxy-section">
            <div class="advanced-proxy-section-head">
              <div>
                <h4>上游 Provider 队列</h4>
                <p>这里记录真实上游地址和密钥，便于回溯。Claude 兼容转换与其他 OpenAI 兼容应用接管都复用这组 Provider。</p>
              </div>
            </div>

            <div class="advanced-proxy-adder">
              <a-select
                v-model:value="pendingRecordKey"
                show-search
                :options="recordOptions"
                placeholder="从密钥管理中选择一条记录加入队列"
                option-filter-prop="label"
              />
              <a-button type="primary" @click="appendProviderFromSelection">加入队列</a-button>
              <a-button @click="reloadContext">刷新记录</a-button>
            </div>

            <div v-if="!providerCount" class="advanced-proxy-empty">
              还没有可用 Provider。先从密钥管理中加入至少一条记录，再启用任意应用的本地接管。
            </div>

            <div v-else class="advanced-proxy-provider-list">
              <article
                v-for="(provider, index) in draft.claude.providers"
                :key="provider.id"
                class="advanced-proxy-provider-card"
              >
                <div class="advanced-proxy-provider-head">
                  <div class="advanced-proxy-provider-title">
                    <span class="advanced-proxy-provider-order">P{{ index + 1 }}</span>
                    <div>
                      <div class="advanced-proxy-provider-name">{{ provider.name || `Provider ${index + 1}` }}</div>
                      <div class="advanced-proxy-provider-meta">{{ provider.baseUrl || '-' }}</div>
                    </div>
                  </div>

                  <div class="advanced-proxy-provider-actions">
                    <a-switch
                      :checked="provider.enabled"
                      checked-children="启用"
                      un-checked-children="停用"
                      @change="value => handleProviderFieldMutation(index, 'enabled', value, 'Provider 状态已更新')"
                    />
                    <a-button size="small" :disabled="index === 0" @click="moveProvider(index, -1)">上移</a-button>
                    <a-button size="small" :disabled="index === draft.claude.providers.length - 1" @click="moveProvider(index, 1)">下移</a-button>
                    <a-button size="small" @click="reloadProviderStats(provider.id)">刷新熔断</a-button>
                    <a-button size="small" danger @click="resetProviderBreaker(provider.id)">重置熔断</a-button>
                    <a-button size="small" danger @click="removeProvider(index)">移除</a-button>
                  </div>
                </div>

                <div class="advanced-proxy-provider-tags">
                  <a-tag :color="provider.enabled ? 'green' : 'default'">{{ provider.enabled ? '队列启用' : '队列停用' }}</a-tag>
                  <a-tag :color="breakerStateColor(provider.id)">熔断 {{ getBreakerStateLabel(provider.id) }}</a-tag>
                  <a-tag color="blue">{{ provider.apiFormat }}</a-tag>
                  <a-tag>{{ provider.apiKeyField }}</a-tag>
                </div>

                <div class="advanced-proxy-provider-grid">
                  <a-form-item label="名称">
                    <a-input
                      :value="provider.name"
                      @change="event => handleProviderFieldMutation(index, 'name', event?.target?.value ?? '', 'Provider 名称已更新')"
                    />
                  </a-form-item>
                  <a-form-item label="Base URL">
                    <a-input
                      :value="provider.baseUrl"
                      @change="event => handleProviderFieldMutation(index, 'baseUrl', event?.target?.value ?? '', 'Provider 地址已更新')"
                    />
                  </a-form-item>
                  <a-form-item label="API Key">
                    <a-input-password
                      :value="provider.apiKey"
                      @change="event => handleProviderFieldMutation(index, 'apiKey', event?.target?.value ?? '', 'Provider API Key 已更新')"
                    />
                  </a-form-item>
                  <a-form-item label="上游模型">
                    <a-input
                      :value="provider.model"
                      placeholder="留空则沿用请求里的模型"
                      @change="event => handleProviderFieldMutation(index, 'model', event?.target?.value ?? '', 'Provider 模型已更新')"
                    />
                  </a-form-item>
                  <a-form-item label="API 格式">
                    <a-select
                      :value="provider.apiFormat"
                      @change="value => handleProviderFieldMutation(index, 'apiFormat', value, 'Provider API 格式已更新')"
                    >
                      <a-select-option value="anthropic">anthropic</a-select-option>
                      <a-select-option value="openai_chat">openai_chat</a-select-option>
                      <a-select-option value="openai_responses">openai_responses</a-select-option>
                    </a-select>
                  </a-form-item>
                  <a-form-item label="Claude Key 字段">
                    <a-select
                      :value="provider.apiKeyField"
                      @change="value => handleProviderFieldMutation(index, 'apiKeyField', value, 'Provider Key 字段已更新')"
                    >
                      <a-select-option value="ANTHROPIC_AUTH_TOKEN">ANTHROPIC_AUTH_TOKEN</a-select-option>
                      <a-select-option value="ANTHROPIC_API_KEY">ANTHROPIC_API_KEY</a-select-option>
                    </a-select>
                  </a-form-item>
                </div>

                <div class="advanced-proxy-provider-stats">
                  <span>连续失败 {{ getBreakerStats(provider.id).consecutiveFailures || 0 }}</span>
                  <span>连续成功 {{ getBreakerStats(provider.id).consecutiveSuccesses || 0 }}</span>
                  <span>总请求 {{ getBreakerStats(provider.id).totalRequests || 0 }}</span>
                  <span>失败请求 {{ getBreakerStats(provider.id).failedRequests || 0 }}</span>
                </div>
              </article>
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

              <div class="advanced-proxy-dense-grid">
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">最大重试次数</label>
                  <a-input-number :value="draft.failover.maxRetries" :min="0" :max="10" style="width:100%" @change="value => handleFailoverFieldMutation('maxRetries', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">流式首字节超时</label>
                  <a-input-number :value="draft.failover.streamingFirstByteTimeout" :min="5" :max="300" style="width:100%" @change="value => handleFailoverFieldMutation('streamingFirstByteTimeout', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">流式空闲超时</label>
                  <a-input-number :value="draft.failover.streamingIdleTimeout" :min="5" :max="600" style="width:100%" @change="value => handleFailoverFieldMutation('streamingIdleTimeout', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">非流式超时</label>
                  <a-input-number :value="draft.failover.nonStreamingTimeout" :min="5" :max="600" style="width:100%" @change="value => handleFailoverFieldMutation('nonStreamingTimeout', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">熔断失败阈值</label>
                  <a-input-number :value="draft.failover.circuitFailureThreshold" :min="1" :max="20" style="width:100%" @change="value => handleFailoverFieldMutation('circuitFailureThreshold', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">恢复成功阈值</label>
                  <a-input-number :value="draft.failover.circuitSuccessThreshold" :min="1" :max="20" style="width:100%" @change="value => handleFailoverFieldMutation('circuitSuccessThreshold', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">熔断恢复等待</label>
                  <a-input-number :value="draft.failover.circuitTimeoutSeconds" :min="5" :max="600" style="width:100%" @change="value => handleFailoverFieldMutation('circuitTimeoutSeconds', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">错误率阈值</label>
                  <a-input-number :value="draft.failover.circuitErrorRateThreshold" :min="0.1" :max="1" :step="0.05" style="width:100%" @change="value => handleFailoverFieldMutation('circuitErrorRateThreshold', value)" />
                </div>
                <div class="advanced-proxy-compact-field">
                  <label class="advanced-proxy-compact-label">最小请求数</label>
                  <a-input-number :value="draft.failover.circuitMinRequests" :min="1" :max="100" style="width:100%" @change="value => handleFailoverFieldMutation('circuitMinRequests', value)" />
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
import DesktopConfigDiffModal from './DesktopConfigDiffModal.vue';
import { loadPanelRecords } from '../utils/keyPanelStore.js';
import {
  ADVANCED_PROXY_APPS,
  countAdvancedProxyEnabledProviders,
  countAdvancedProxyOpenAIProviders,
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyConfig,
  getAdvancedProxyConfigFilePath,
  getCircuitBreakerStats,
  normalizeAdvancedProxyConfig,
  resetCircuitBreaker,
  setAdvancedProxyConfig,
} from '../utils/advancedProxyBridge.js';
import { buildSingleFileWritePreview } from '../utils/localConfigPreview.js';

const EMPTY_PREVIEW = { appGroups: [], writes: [], errors: [] };

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
const pendingRecordKey = ref('');
const availableRecords = ref([]);
const breakerStatsMap = ref({});
const configFilePath = ref('localStorage:batch_api_check_advanced_proxy_config_v1');
const loadedConfigSnapshot = ref(normalizeAdvancedProxyConfig({}));
const pendingSaveConfig = ref(null);
const pendingSuccessMessage = ref('高级代理配置已更新');
const configPreview = ref(EMPTY_PREVIEW);
const draft = reactive(normalizeAdvancedProxyConfig({}));

const providerCount = computed(() => draft.claude.providers.length);
const enabledProviderCount = computed(() => countAdvancedProxyEnabledProviders(draft));
const openAIProviderCount = computed(() => countAdvancedProxyOpenAIProviders(draft));
const enabledAppCount = computed(() => ADVANCED_PROXY_APPS.filter(app => draft?.[app.id]?.enabled === true).length);
const enabledAppLabels = computed(() => ADVANCED_PROXY_APPS.filter(app => draft?.[app.id]?.enabled === true).map(app => app.label).join(' / '));
const openCircuitCount = computed(() => draft.claude.providers.filter(provider => getBreakerStateLabel(provider.id) === 'open').length);
const recordOptions = computed(() =>
  availableRecords.value.map(record => ({
    value: record.rowKey,
    label: `${record.siteName || '未命名'} | ${record.siteUrl || '-'} | ${record.selectedModel || record.quickTestModel || '未选模型'}`,
  }))
);

const appCards = computed(() =>
  ADVANCED_PROXY_APPS.map(app => {
    const enabled = draft?.[app.id]?.enabled === true;
    const baseUrl = getAdvancedProxyAppBaseUrl(app.id, draft);
    const readyText = app.mode === 'anthropic'
      ? (enabledProviderCount.value > 0 ? '已接入 Claude 兼容链路' : '缺少可用 Provider')
      : (openAIProviderCount.value > 0 ? '已接入 OpenAI 兼容链路' : '缺少 OpenAI 兼容上游');
    return {
      ...app,
      enabled,
      baseUrl,
      modeLabel: app.mode === 'anthropic' ? 'Anthropic Messages 入口' : 'OpenAI Compatible 入口',
      readyText,
    };
  })
);

function toPlainValue(value) {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function normalizeForPreview(config) {
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

function openPreviewForConfig(nextConfig, successMessage = '高级代理配置已更新') {
  const beforeText = JSON.stringify(normalizeForPreview(loadedConfigSnapshot.value), null, 2);
  const afterText = JSON.stringify(normalizeForPreview(nextConfig), null, 2);
  if (beforeText === afterText) {
    message.info('当前没有需要写入的配置变更');
    return;
  }
  pendingSaveConfig.value = createPendingConfig(nextConfig);
  pendingSuccessMessage.value = successMessage;
  configPreview.value = buildSingleFileWritePreview({
    appId: 'advanced-proxy',
    appName: '高级代理',
    fileId: 'config',
    label: 'config.json',
    path: configFilePath.value,
    before: beforeText,
    after: afterText,
  });
  previewOpen.value = true;
}

function handleConfigMutation(mutator, successMessage) {
  if (saving.value) return;
  const nextConfig = createPendingConfig();
  mutator(nextConfig);
  openPreviewForConfig(nextConfig, successMessage);
}

function handleAppToggleMutation(appId, value) {
  const app = ADVANCED_PROXY_APPS.find(item => item.id === appId);
  handleConfigMutation(next => {
    if (!next[appId]) {
      next[appId] = {};
    }
    next[appId].enabled = value;
  }, `${app?.label || appId} 接管开关已更新`);
}

function handleProviderFieldMutation(index, field, value, successMessage) {
  handleConfigMutation(next => {
    const provider = next.claude?.providers?.[index];
    if (provider) provider[field] = value;
  }, successMessage);
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
    loadedConfigSnapshot.value = normalizeAdvancedProxyConfig(config);
    overwriteDraft(loadedConfigSnapshot.value);
    configFilePath.value = await getAdvancedProxyConfigFilePath();
    breakerStatsMap.value = {};
    await Promise.all((draft.claude.providers || []).map(provider => reloadProviderStats(provider.id)));
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

function appendProviderFromSelection() {
  const selected = availableRecords.value.find(record => record.rowKey === pendingRecordKey.value);
  if (!selected) {
    message.warning('请选择要加入队列的记录');
    return;
  }
  if (draft.claude.providers.some(item => item.id === selected.rowKey)) {
    message.info('这条记录已经在队列里了');
    return;
  }

  handleConfigMutation(next => {
    next.claude.providers.push({
      id: selected.rowKey,
      rowKey: selected.rowKey,
      name: selected.siteName || selected.siteUrl || 'Provider',
      baseUrl: selected.siteUrl,
      apiKey: selected.apiKey,
      model: selected.selectedModel || selected.quickTestModel || '',
      apiFormat: 'openai_chat',
      apiKeyField: 'ANTHROPIC_AUTH_TOKEN',
      enabled: true,
      sortIndex: (next.claude.providers?.length || 0) + 1,
      sourceType: selected.sourceType || 'auto',
    });
  }, 'Provider 已加入队列');
  pendingRecordKey.value = '';
}

function removeProvider(index) {
  handleConfigMutation(next => {
    next.claude.providers.splice(index, 1);
  }, 'Provider 已移除');
}

function moveProvider(index, delta) {
  const targetIndex = index + delta;
  if (targetIndex < 0 || targetIndex >= draft.claude.providers.length) return;
  handleConfigMutation(next => {
    const list = [...(next.claude.providers || [])];
    const [current] = list.splice(index, 1);
    list.splice(targetIndex, 0, current);
    next.claude.providers = list;
  }, 'Provider 顺序已更新');
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

async function copyText(text, successMessage) {
  try {
    await navigator.clipboard.writeText(String(text || ''));
    message.success(successMessage);
  } catch (error) {
    message.error(error?.message || '复制失败，请手动复制');
  }
}

async function applyPreview() {
  if (!pendingSaveConfig.value) {
    message.warning('没有待写入的配置变更');
    return;
  }
  saving.value = true;
  try {
    const saved = await setAdvancedProxyConfig(pendingSaveConfig.value);
    loadedConfigSnapshot.value = normalizeAdvancedProxyConfig(saved);
    overwriteDraft(loadedConfigSnapshot.value);
    await Promise.all((draft.claude.providers || []).map(provider => reloadProviderStats(provider.id)));
    previewOpen.value = false;
    configPreview.value = EMPTY_PREVIEW;
    pendingSaveConfig.value = null;
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
  gap: 14px;
}

.advanced-proxy-hero,
.advanced-proxy-summary-grid,
.advanced-proxy-layout,
.advanced-proxy-provider-grid,
.advanced-proxy-inline-grid,
.advanced-proxy-dense-grid {
  display: grid;
  gap: 10px;
}

.advanced-proxy-hero {
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 1.25fr);
  padding: 16px 18px;
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
  gap: 10px;
}

.advanced-proxy-hero-copy h3,
.advanced-proxy-section-head h4 {
  margin: 0;
  color: #22311c;
  font-size: 18px;
  line-height: 1.3;
}

.advanced-proxy-hero-copy p,
.advanced-proxy-section-head p,
.advanced-proxy-provider-meta,
.advanced-proxy-provider-stats,
.advanced-proxy-notes,
.advanced-proxy-app-copy small {
  margin: 0;
  color: #6a7867;
  font-size: 12px;
  line-height: 1.55;
}

.advanced-proxy-app-grid,
.advanced-proxy-summary-grid,
.advanced-proxy-provider-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-app-card,
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

.advanced-proxy-app-card {
  display: grid;
  gap: 8px;
  padding: 12px;
}

.advanced-proxy-app-card-active {
  border-color: rgba(67, 113, 49, 0.24);
  box-shadow: 0 10px 24px rgba(74, 104, 58, 0.08);
}

.advanced-proxy-app-head,
.advanced-proxy-app-meta,
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

.advanced-proxy-app-head,
.advanced-proxy-app-meta,
.advanced-proxy-provider-head,
.advanced-proxy-toggle-row {
  justify-content: space-between;
}

.advanced-proxy-app-copy,
.advanced-proxy-section-head > div {
  min-width: 0;
}

.advanced-proxy-app-copy strong,
.advanced-proxy-summary-card strong,
.advanced-proxy-provider-name {
  color: #22311c;
  font-size: 14px;
  font-weight: 700;
}

.advanced-proxy-app-card code {
  padding: 6px 8px;
  border-radius: 10px;
  background: rgba(79, 108, 62, 0.08);
  color: #2f4a28;
  font-size: 11px;
  word-break: break-all;
}

.advanced-proxy-summary-card,
.advanced-proxy-provider-card,
.advanced-proxy-section {
  display: grid;
  gap: 8px;
  padding: 12px 14px;
}

.advanced-proxy-layout {
  grid-template-columns: minmax(0, 1.55fr) minmax(320px, 0.95fr);
  align-items: start;
}

.advanced-proxy-adder {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 10px;
}

.advanced-proxy-empty {
  padding: 16px 14px;
  border-radius: 14px;
  border: 1px dashed rgba(90, 117, 79, 0.28);
  color: #6a7965;
  background: rgba(247, 250, 244, 0.9);
  font-size: 12px;
  line-height: 1.6;
}

.advanced-proxy-provider-order {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 26px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(60, 103, 39, 0.12);
  color: #2c4a1f;
  font-size: 11px;
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
  margin-bottom: 6px;
}

.advanced-proxy-provider-stats {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.advanced-proxy-inline-grid,
.advanced-proxy-dense-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.advanced-proxy-inline-control,
.advanced-proxy-compact-field,
.advanced-proxy-toggle-row {
  padding: 10px 12px;
}

.advanced-proxy-inline-label,
.advanced-proxy-compact-label {
  color: #22311c;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.advanced-proxy-compact-field {
  display: grid;
  gap: 6px;
}

.advanced-proxy-compact-field :deep(.ant-input-number) {
  border-radius: 10px;
}

.advanced-proxy-notes {
  padding-left: 18px;
}

@media (max-width: 1180px) {
  .advanced-proxy-hero,
  .advanced-proxy-layout,
  .advanced-proxy-summary-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 860px) {
  .advanced-proxy-app-grid,
  .advanced-proxy-provider-grid,
  .advanced-proxy-inline-grid,
  .advanced-proxy-dense-grid,
  .advanced-proxy-adder {
    grid-template-columns: 1fr;
  }

  .advanced-proxy-provider-actions {
    justify-content: flex-start;
  }
}
</style>
