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
            <span>当前队列 Provider</span>
            <strong>{{ providerCount }}</strong>
            <small>{{ selectedQueueLabel }}队列启用 {{ enabledProviderCount }} 条</small>
          </article>
          <article class="advanced-proxy-summary-card">
            <span>当前队列 OpenAI 兼容</span>
            <strong>{{ openAIProviderCount }}</strong>
            <small>{{ selectedQueueScope === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE ? '未覆盖应用默认继承全局' : `${selectedQueueAppLabel} 当前有效兼容数` }}</small>
          </article>
          <article class="advanced-proxy-summary-card">
            <span>熔断打开数</span>
            <strong>{{ openCircuitCount }}</strong>
            <small>故障自动转移 {{ unifiedFailoverEnabled ? '已开启' : '未开启' }}</small>
          </article>
        </section>

        <div class="advanced-proxy-layout">
          <section class="advanced-proxy-section">
            <div class="advanced-proxy-section-head">
              <div>
                <h4>{{ queuePanelTitle }}</h4>
                <p>{{ queuePanelDescription }}</p>
              </div>
              <div class="advanced-proxy-queue-toolbar">
                <a-select
                  class="advanced-proxy-queue-select"
                  :value="selectedQueueScope"
                  :options="queueScopeOptions"
                  @change="handleQueueScopeChange"
                />
                <a-tooltip :title="quickSetupTooltipText">
                  <a-button
                    class="advanced-proxy-toolbar-icon-button"
                    :disabled="!validProviderCandidateCards.length"
                    @click="handleQuickSelectValidProviders"
                  >
                    <template #icon>
                      <span class="advanced-proxy-toolbar-emoji-icon" aria-hidden="true">💪</span>
                    </template>
                  </a-button>
                </a-tooltip>
                <a-tooltip
                  v-if="selectedQueueScope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE"
                  :title="selectedQueueInheritGlobal ? '当前已经在跟随全局队列' : '切换为跟随全局队列'"
                >
                  <a-button
                    class="advanced-proxy-toolbar-icon-button"
                    :disabled="selectedQueueInheritGlobal"
                    @click="handleFollowGlobalQueue"
                  >
                    <template #icon>
                      <CloudSyncOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
                <a-tooltip title="刷新记录">
                  <a-button class="advanced-proxy-toolbar-icon-button" @click="reloadContext">
                    <template #icon>
                      <ReloadOutlined />
                    </template>
                  </a-button>
                </a-tooltip>
              </div>
            </div>

            <div v-if="selectedQueueScope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE" class="advanced-proxy-queue-mode">
              <a-tag :color="selectedQueueInheritGlobal ? 'gold' : 'green'">
                {{ selectedQueueInheritGlobal ? '当前继承全局' : '当前使用独立队列' }}
              </a-tag>
              <span>
                {{ selectedQueueInheritGlobal
                  ? '点任意卡片后会先复制全局队列，再切换为当前应用的独立队列。'
                  : '当前应用会优先使用自己的 Provider 队列，不再继承全局。' }}
              </span>
            </div>

            <div class="advanced-proxy-provider-pool">
              <div class="advanced-proxy-provider-panel-grid">
                <button
                  v-for="item in providerCandidateCards"
                  :key="item.id"
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
              </div>

              <div v-if="providerCandidateCards.length && !providerCount" class="advanced-proxy-empty advanced-proxy-empty-compact">
                {{ queuePanelEmptyText }}
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
                  <h4>请求策略与并发</h4>
                  <p>分发策略负责挑选请求路径，RPM 在下方按供应商单独设定。</p>
                </div>
              </div>

              <div class="advanced-proxy-ha-grid">
                <div class="advanced-proxy-radio-card">
                  <label class="advanced-proxy-compact-label">请求分发策略</label>
                  <a-radio-group
                    class="advanced-proxy-radio-group"
                    :value="draft.highAvailability.dispatchMode"
                    @change="handleDispatchModeChange"
                  >
                    <a-radio-button
                      v-for="option in dispatchModeOptions"
                      :key="option.value"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </a-radio-button>
                  </a-radio-group>
                  <p class="advanced-proxy-radio-hint">{{ selectedDispatchModeDescription }}</p>
                </div>
                <div class="advanced-proxy-inline-control advanced-proxy-ha-toggle-card">
                  <div class="advanced-proxy-ha-toggle-copy">
                    <label class="advanced-proxy-compact-label">高可用智能调度</label>
                    <p class="advanced-proxy-radio-hint">启用后按健康度、实时负载和当前 RPM 限制执行请求分发。</p>
                  </div>
                  <a-switch
                    :checked="highAvailabilityEnabled"
                    @change="handleHighAvailabilityToggle"
                  />
                </div>
              </div>

              <div class="advanced-proxy-inline-control advanced-proxy-rpm-row">
                <a-tooltip placement="topLeft">
                  <template #title>
                    <div class="advanced-proxy-tooltip">
                      <span>0 表示不限制。</span>
                      <span>默认从“全局”读取；选择某个 provider 后会优先使用该供应商的值。</span>
                      <span>下拉框包含全局和当前队列中的 provider。</span>
                    </div>
                  </template>
                  <span class="advanced-proxy-inline-label">RPM 设置</span>
                </a-tooltip>
                <div class="advanced-proxy-rpm-controls">
                  <a-select
                    class="advanced-proxy-rpm-select"
                    :value="selectedHighAvailabilityRpmProviderKey"
                    :options="rpmProviderOptions"
                    popupClassName="advanced-proxy-rpm-dropdown"
                    @change="handleHighAvailabilityRpmProviderChange"
                  />
                  <a-input-number
                    class="advanced-proxy-rpm-input"
                    :value="selectedHighAvailabilityRpmValue"
                    :min="0"
                    :precision="0"
                    :step="1"
                    @change="handleHighAvailabilityRpmValueChange"
                  />
                </div>
              </div>
            </section>
            <section class="advanced-proxy-section">
              <div class="advanced-proxy-section-head">
                <div>
                  <h4>故障转移</h4>
                  <p>代理入口会按所属应用挑选自己的有效队列；未单独覆盖的应用会继续继承全局队列。</p>
                </div>
              </div>

              <div class="advanced-proxy-inline-grid">
                <div class="advanced-proxy-inline-control">
                  <span class="advanced-proxy-inline-label">故障自动转移</span>
                  <a-switch
                    :checked="unifiedFailoverEnabled"
                    @change="handleUnifiedFailoverToggle"
                  />
                </div>
                <div class="advanced-proxy-inline-control">
                  <span class="advanced-proxy-inline-label">动态优化队列（仅基于故障率调整队列）</span>
                  <a-switch
                    :checked="draft.highAvailability.dynamicOptimizeQueue"
                    @change="value => handleHighAvailabilityFieldMutation('dynamicOptimizeQueue', value)"
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
                <li><code>codex</code> / <code>opencode</code> / <code>openclaw</code> 入口会直接代理 OpenAI 兼容请求，并按各自的有效队列执行重试与熔断。</li>
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
import { CloudSyncOutlined, ReloadOutlined } from '@ant-design/icons-vue';
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
  ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  ADVANCED_PROXY_QUEUE_SCOPES,
  getAdvancedProxyAppBaseUrl,
  getAdvancedProxyConfig,
  getAdvancedProxyEffectiveProviders,
  getAdvancedProxyQueueProviders,
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
const DISPATCH_MODE_OPTIONS = [
  { value: 'fixed', label: '固定', description: '按当前队列顺序执行，不额外重排。' },
  { value: 'ordered', label: '顺序', description: '按健康度、负载和 RPM 约束动态排队。' },
  { value: 'random', label: '随机', description: '在满足约束的候选中随机分散压力。' },
];

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
const selectedQueueScope = ref(ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
const selectedHighAvailabilityRpmProviderKey = ref(ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE);
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
const queueScopeOptions = ADVANCED_PROXY_QUEUE_SCOPES.map(item => ({
  value: item.id,
  label: item.label,
}));

const enabledAppIds = computed(() => getEnabledAppIds(draft));
const enabledAppCount = computed(() => enabledAppIds.value.length);
const enabledAppLabels = computed(() =>
  ADVANCED_PROXY_APPS
    .filter(app => enabledAppIds.value.includes(app.id))
    .map(app => app.label)
    .join(' / ')
);
const highAvailabilityEnabled = computed(() => draft?.highAvailability?.enabled === true);
const unifiedFailoverEnabled = computed(() =>
  draft?.failover?.enabled === true && draft?.failover?.autoFailoverEnabled === true
);
const dispatchModeOptions = DISPATCH_MODE_OPTIONS;
const selectedDispatchModeDescription = computed(() =>
  DISPATCH_MODE_OPTIONS.find(option => option.value === draft?.highAvailability?.dispatchMode)?.description
  || DISPATCH_MODE_OPTIONS[0].description
);
const selectedHighAvailabilityRpmValue = computed(() => {
  const rpm = draft?.highAvailability?.rpm || {};
  const providerKey = selectedHighAvailabilityRpmProviderKey.value;
  if (providerKey === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return Number.isFinite(Number(rpm.global)) ? Number(rpm.global) : 0;
  }
  const providerValue = rpm?.providers?.[providerKey];
  if (providerValue == null || providerValue === '') {
    return Number.isFinite(Number(rpm.global)) ? Number(rpm.global) : 0;
  }
  return Number.isFinite(Number(providerValue)) ? Number(providerValue) : 0;
});
const proxyMasterEnabled = computed(() => enabledAppIds.value.length > 0);
const selectedQueueLabel = computed(() =>
  ADVANCED_PROXY_QUEUE_SCOPES.find(item => item.id === selectedQueueScope.value)?.label || '全局'
);
const selectedQueueAppLabel = computed(() =>
  ADVANCED_PROXY_APPS.find(app => app.id === selectedQueueScope.value)?.label || '全局'
);
const selectedQueueInheritGlobal = computed(() =>
  selectedQueueScope.value !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE
  && draft?.queues?.[selectedQueueScope.value]?.inheritGlobal === true
);
const displayedQueueProviders = computed(() =>
  getAdvancedProxyQueueProviders(draft, selectedQueueScope.value, {
    effective: selectedQueueScope.value !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  })
);
const rpmProviderOptions = computed(() => {
  const options = [{
    value: ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
    label: '全局',
  }];

  displayedQueueProviders.value.forEach((provider, index) => {
    const key = String(provider?.rowKey || provider?.id || '').trim();
    if (!key) return;
    const name = String(provider?.name || provider?.baseUrl || `Provider ${index + 1}`).trim() || `Provider ${index + 1}`;
    const model = String(provider?.model || '').trim();
    options.push({
      value: key,
      label: `${index + 1}. ${name}${model ? ` · ${model}` : ''}`,
    });
  });

  return options;
});
const providerCount = computed(() => displayedQueueProviders.value.length);
const enabledProviderCount = computed(() => displayedQueueProviders.value.filter(provider => provider?.enabled !== false).length);
const openAIProviderCount = computed(() =>
  displayedQueueProviders.value.filter(
    provider => provider?.enabled !== false && String(provider?.apiFormat || '').trim().toLowerCase() !== 'anthropic',
  ).length
);
const breakerAppIdsForSummary = computed(() => {
  if (selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return enabledAppIds.value;
  }
  return [selectedQueueScope.value];
});
const openCircuitCount = computed(() =>
  displayedQueueProviders.value.filter(provider =>
    breakerAppIdsForSummary.value.some(appId => getBreakerStateLabel(provider.id, appId) === 'open')
  ).length
);
const queuePanelTitle = computed(() => `[${selectedQueueLabel.value}]上游 Provider 队列`);
const queuePanelDescription = computed(() => {
  if (selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    return '点击卡片即可维护默认全局队列。未单独覆盖的应用，都会继承这条全局队列。';
  }
  if (selectedQueueInheritGlobal.value) {
    return `${selectedQueueAppLabel.value} 当前继承全局队列。点击卡片后会自动复制出独立队列，并按点击顺序维护优先级。`;
  }
  return `${selectedQueueAppLabel.value} 当前使用独立队列，优先级按点击顺序自动更新。`;
});
const queuePanelEmptyText = computed(() =>
  selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE
    ? '点击卡片加入全局默认队列，队列优先级按点击顺序自动更新。'
    : (selectedQueueInheritGlobal.value
      ? '当前应用正在继承全局队列。点任意卡片后会自动分叉出独立队列。'
      : '当前独立队列为空，点击卡片即可加入 Provider。')
);
const quickSetupTooltipText = computed(() =>
  validProviderCandidateCards.value.length
    ? '一键勾选有效密钥'
    : '一键勾选有效密钥前，请先完成快速测活'
);
const providerSelectionMap = computed(() => {
  const map = new Map();
  displayedQueueProviders.value.forEach((provider, index) => {
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
      skLabel: duplicate.count > 1
        ? formatProviderSkLabel(duplicate.index, String(record?.apiKey || selectedMeta?.provider?.apiKey || '').trim())
        : '',
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
const validProviderCandidateCards = computed(() =>
  providerCandidateCards.value.filter(item => isValidProviderSourceRecord(item?.sourceRecord))
);

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

watch(
  rpmProviderOptions,
  options => {
    const current = String(selectedHighAvailabilityRpmProviderKey.value || '').trim() || ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    if (options.some(option => option.value === current)) {
      return;
    }
    const fallback = options.find(option => option.value !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE)?.value
      || ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
    selectedHighAvailabilityRpmProviderKey.value = fallback;
  },
  { immediate: true },
);

function toPlainValue(value) {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function maskProviderApiKey(value) {
  const normalized = String(value || '').trim();
  if (!normalized) return '';
  if (normalized.length <= 8) return normalized;
  return `${normalized.slice(0, 4)}****${normalized.slice(-4)}`;
}

function formatProviderSkLabel(index, apiKey) {
  const maskedKey = maskProviderApiKey(apiKey);
  if (!Number(index)) {
    return maskedKey ? `SK | ${maskedKey}` : '';
  }
  return maskedKey ? `SK ${index} | ${maskedKey}` : `SK ${index}`;
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
  selectedHighAvailabilityRpmProviderKey.value = ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

function ensureQueueSection(config, scope) {
  if (!config.queues || typeof config.queues !== 'object') {
    config.queues = {};
  }
  if (!config.queues[scope] || typeof config.queues[scope] !== 'object') {
    config.queues[scope] = {
      inheritGlobal: scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
      providers: [],
    };
  }
  if (!Array.isArray(config.queues[scope].providers)) {
    config.queues[scope].providers = [];
  }
  if (scope === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    config.queues[scope].inheritGlobal = false;
  } else if (typeof config.queues[scope].inheritGlobal !== 'boolean') {
    config.queues[scope].inheritGlobal = true;
  }
  return config.queues[scope];
}

function createPendingConfig(source = draft) {
  const plainDraft = toPlainValue(source);
  ADVANCED_PROXY_QUEUE_SCOPES.forEach(item => {
    const queue = ensureQueueSection(plainDraft, item.id);
    queue.providers = (queue.providers || []).map((provider, index) => ({
      ...provider,
      sortIndex: index + 1,
    }));
  });
  plainDraft.claude.providers = [...(plainDraft.queues?.global?.providers || [])];
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

function isValidProviderSourceRecord(record) {
  if (!record?.rowKey || !record?.siteUrl || !record?.apiKey) return false;
  const quickTestStatus = String(record?.quickTestStatus || '').trim().toLowerCase();
  return quickTestStatus === 'success' || quickTestStatus === 'warning';
}

function getEnabledAppIds(source = draft) {
  return ADVANCED_PROXY_APPS
    .filter(app => source?.[app.id]?.enabled === true)
    .map(app => app.id);
}

function getDisplayedQueueProviders(source, scope = selectedQueueScope.value) {
  return getAdvancedProxyQueueProviders(source, scope, {
    effective: scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  });
}

function isQueueFollowingGlobal(source, scope = selectedQueueScope.value) {
  return scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE
    && source?.queues?.[scope]?.inheritGlobal === true;
}

function replaceQueueProviders(config, scope, providers) {
  const queue = ensureQueueSection(config, scope);
  queue.providers = providers.map((provider, index) => ({
    ...provider,
    enabled: provider?.enabled !== false,
    sortIndex: index + 1,
  }));
  if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
    queue.inheritGlobal = false;
  }
}

function hasConfigChanges(nextConfig) {
  const beforeText = JSON.stringify(normalizeForSave(loadedConfigSnapshot.value));
  const afterText = JSON.stringify(normalizeForSave(nextConfig));
  return beforeText !== afterText;
}

async function syncSavedConfig(savedConfig) {
  loadedConfigSnapshot.value = normalizeAdvancedProxyConfig(savedConfig);
  overwriteDraft(loadedConfigSnapshot.value);
  await reloadBreakerStatsForScope(selectedQueueScope.value, draft);
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
  const providers = getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly });
  return providers[0] || null;
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
  const effectiveProviders = getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly: false });
  if (effectiveProviders.some(item => String(item?.model || '').trim())) {
    return String(effectiveProviders.find(item => String(item?.model || '').trim())?.model || '').trim();
  }
  const globalProviders = getAdvancedProxyQueueProviders(config, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, { effective: false });
  return String(globalProviders.find(item => String(item?.model || '').trim())?.model || '').trim();
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

function handleUnifiedFailoverToggle(value) {
  handleConfigMutation(next => {
    next.failover.enabled = value === true;
    next.failover.autoFailoverEnabled = value === true;
  }, '故障自动转移开关已更新');
}

function handleHighAvailabilityFieldMutation(field, value) {
  handleConfigMutation(next => {
    if (!next.highAvailability || typeof next.highAvailability !== 'object') {
      next.highAvailability = {};
    }
    next.highAvailability[field] = value;
  }, '高可用与并发配置已更新');
}

function handleHighAvailabilityToggle(value) {
  handleHighAvailabilityFieldMutation('enabled', Boolean(value));
}

function normalizeRpmInputValue(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0;
  }
  return Math.floor(parsed);
}

function handleHighAvailabilityRpmProviderChange(providerKey) {
  const normalizedProviderKey = String(providerKey || '').trim();
  selectedHighAvailabilityRpmProviderKey.value = rpmProviderOptions.value.some(option => option.value === normalizedProviderKey)
    ? normalizedProviderKey
    : ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

function handleHighAvailabilityRpmValueChange(value) {
  const rpmValue = normalizeRpmInputValue(value);
  handleConfigMutation(next => {
    if (!next.highAvailability || typeof next.highAvailability !== 'object') {
      next.highAvailability = {};
    }
    if (!next.highAvailability.rpm || typeof next.highAvailability.rpm !== 'object') {
      next.highAvailability.rpm = {
        global: 0,
        providers: {},
      };
    }
    if (!next.highAvailability.rpm.providers || typeof next.highAvailability.rpm.providers !== 'object') {
      next.highAvailability.rpm.providers = {};
    }
    if (selectedHighAvailabilityRpmProviderKey.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) {
      next.highAvailability.rpm.global = rpmValue;
      return;
    }
    next.highAvailability.rpm.providers[selectedHighAvailabilityRpmProviderKey.value] = rpmValue;
  }, '高可用 RPM 设置已更新');
}

function handleDispatchModeChange(event) {
  const value = event?.target?.value;
  if (!value) return;
  handleHighAvailabilityFieldMutation('dispatchMode', value);
}

function handleQueueScopeChange(scope) {
  selectedQueueScope.value = ADVANCED_PROXY_QUEUE_SCOPES.some(item => item.id === scope)
    ? scope
    : ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE;
}

function handleFollowGlobalQueue() {
  if (selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE) return;
  if (selectedQueueInheritGlobal.value) {
    message.info('当前已经在跟随全局队列');
    return;
  }
  handleConfigMutation(next => {
    const queue = ensureQueueSection(next, selectedQueueScope.value);
    queue.inheritGlobal = true;
    queue.providers = [];
  }, `${selectedQueueAppLabel.value} 已改为跟随全局队列`);
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

watch(
  selectedQueueScope,
  async scope => {
    if (props.open) {
      await reloadBreakerStatsForScope(scope, draft);
    }
  }
);

function toggleProviderQueue(item) {
  const providerId = String(item?.id || '').trim();
  if (!providerId) return;

  handleConfigMutation(next => {
    const scope = selectedQueueScope.value;
    const list = getDisplayedQueueProviders(next, scope).map(provider => ({ ...provider }));
    if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE && isQueueFollowingGlobal(next, scope)) {
      ensureQueueSection(next, scope).inheritGlobal = false;
    }
    const existingIndex = list.findIndex(provider => String(provider?.id || provider?.rowKey || '').trim() === providerId);

    if (existingIndex >= 0) {
      list.splice(existingIndex, 1);
    } else if (item?.sourceRecord) {
      list.push(buildProviderFromRecord(item.sourceRecord, list.length + 1));
    }

    replaceQueueProviders(next, scope, list);
  }, item?.selected ? `${selectedQueueLabel.value} 队列已移出 Provider` : `${selectedQueueLabel.value} 队列已加入 Provider`);
}

function handleQuickSelectValidProviders() {
  const validCards = validProviderCandidateCards.value;
  if (!validCards.length) {
    message.warning('暂无可一键勾选的有效密钥，请先完成快速测活');
    return;
  }

  handleConfigMutation(next => {
    const scope = selectedQueueScope.value;
    if (scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE && isQueueFollowingGlobal(next, scope)) {
      ensureQueueSection(next, scope).inheritGlobal = false;
    }

    replaceQueueProviders(
      next,
      scope,
      validCards
        .map(item => item?.sourceRecord)
        .filter(record => isValidProviderSourceRecord(record))
        .map((record, index) => buildProviderFromRecord(record, index + 1))
    );
  }, `${selectedQueueLabel.value} 队列已一键勾选 ${validCards.length} 条有效密钥`);
}

function getBreakerStatsKey(appId, providerId) {
  return `${String(appId || '').trim().toLowerCase()}:${String(providerId || '').trim()}`;
}

function getBreakerStats(providerId, appId = 'claude') {
  return breakerStatsMap.value[getBreakerStatsKey(appId, providerId)] || {};
}

function getBreakerStateLabel(providerId, appId = 'claude') {
  const state = String(getBreakerStats(providerId, appId)?.state || 'closed').trim();
  if (state === 'half_open') return 'half_open';
  if (state === 'open') return 'open';
  return 'closed';
}

function breakerStateColor(providerId, appId = 'claude') {
  const state = getBreakerStateLabel(providerId, appId);
  if (state === 'open') return 'red';
  if (state === 'half_open') return 'orange';
  return 'green';
}

async function reloadProviderStats(providerId, appId = 'claude') {
  if (!providerId || !appId) return;
  try {
    const stats = await getCircuitBreakerStats(appId, providerId);
    breakerStatsMap.value = {
      ...breakerStatsMap.value,
      [getBreakerStatsKey(appId, providerId)]: stats || {},
    };
  } catch (error) {
    console.warn('[AdvancedProxy] reload breaker stats failed:', error);
  }
}

async function reloadBreakerStatsForScope(scope = selectedQueueScope.value, source = draft) {
  const providers = getDisplayedQueueProviders(source, scope);
  const appIds = scope === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE ? getEnabledAppIds(source) : [scope];
  const nextStatsMap = {};

  await Promise.all(
    providers.flatMap(provider =>
      appIds.map(async appId => {
        if (!provider?.id || !appId) return;
        try {
          const stats = await getCircuitBreakerStats(appId, provider.id);
          nextStatsMap[getBreakerStatsKey(appId, provider.id)] = stats || {};
        } catch {}
      })
    )
  );

  breakerStatsMap.value = nextStatsMap;
}

async function resetProviderBreaker(providerId, appId = selectedQueueScope.value === ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE ? 'claude' : selectedQueueScope.value) {
  try {
    await resetCircuitBreaker(appId, providerId);
    await reloadProviderStats(providerId, appId);
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

.advanced-proxy-queue-toolbar,
.advanced-proxy-queue-mode {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.advanced-proxy-queue-toolbar {
  justify-content: flex-end;
}

.advanced-proxy-queue-select {
  min-width: 132px;
}

.advanced-proxy-toolbar-icon-button {
  width: 40px;
  min-width: 40px;
  height: 40px;
  padding: 0;
  border-radius: 12px;
}

.advanced-proxy-toolbar-icon-button :deep(.ant-btn-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
}

.advanced-proxy-toolbar-emoji-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  line-height: 1;
}

.advanced-proxy-queue-mode {
  padding: 8px 10px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(252, 253, 250, 0.84);
  color: #66725f;
  font-size: 11px;
  line-height: 1.45;
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

.advanced-proxy-radio-stack {
  display: grid;
  gap: 8px;
}

.advanced-proxy-radio-card {
  display: grid;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(90, 117, 79, 0.13);
  background: rgba(255, 255, 255, 0.84);
}

.advanced-proxy-ha-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.18fr) minmax(240px, 0.82fr);
  gap: 8px;
  align-items: stretch;
}

.advanced-proxy-ha-toggle-card {
  align-items: flex-start;
}

.advanced-proxy-rpm-row {
  margin-top: 8px;
  gap: 12px;
  flex-wrap: wrap;
}

.advanced-proxy-rpm-controls {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}

.advanced-proxy-rpm-select {
  min-width: 160px;
  font-size: 14px;
}

.advanced-proxy-rpm-dropdown {
  font-size: 14px;
}

.advanced-proxy-rpm-dropdown :deep(.ant-select-item),
.advanced-proxy-rpm-dropdown :deep(.ant-select-item-option-content) {
  font-size: 14px;
  line-height: 1.35;
}

.advanced-proxy-rpm-input {
  width: 132px;
}

.advanced-proxy-ha-toggle-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.advanced-proxy-radio-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.advanced-proxy-radio-hint {
  margin: 0;
  color: #6a7867;
  font-size: 11px;
  line-height: 1.45;
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

.advanced-proxy-section :deep(.ant-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.advanced-proxy-section :deep(.ant-radio-button-wrapper) {
  height: 30px;
  line-height: 28px;
  border-radius: 10px;
  font-size: 11px;
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
  .advanced-proxy-ha-grid,
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
