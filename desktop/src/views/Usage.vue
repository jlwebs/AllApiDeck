<template>
  <div class="usage-view" :class="{ 'usage-view-dark': isDarkMode, 'usage-view-embedded': props.embedded }">
    <div class="usage-page-shell">
      <AppHeader v-if="!props.embedded" current-page="usage" :is-dark-mode="isDarkMode" />

      <main class="usage-main">
        <section v-if="!props.embedded" class="usage-hero">
          <div>
            <p class="usage-kicker">{{ t('USAGE_NAV') }}</p>
            <h1>{{ t('USAGE_TITLE') }}</h1>
            <p>{{ t('USAGE_DESCRIPTION') }}</p>
          </div>
          <div class="usage-actions">
            <div class="usage-range-tabs" role="tablist" :aria-label="t('USAGE_DATE_RANGE')">
              <button
                v-for="tab in rangeTabs"
                :key="tab.id"
                type="button"
                :class="{ 'is-active': range === tab.id }"
                @click="range = tab.id"
              >
                {{ tab.label }}
              </button>
            </div>
            <button type="button" class="usage-refresh" :disabled="isLoadingUsage" @click="refresh">
              <span :class="{ 'is-spinning': isLoadingUsage }" aria-hidden="true">↻</span>
              {{ isLoadingUsage ? t('USAGE_REFRESHING') : t('USAGE_REFRESH') }}
            </button>
          </div>
        </section>

        <div v-else class="usage-embedded-toolbar">
          <div class="usage-range-tabs" role="tablist" :aria-label="t('USAGE_DATE_RANGE')">
            <button
              v-for="tab in rangeTabs"
              :key="tab.id"
              type="button"
              :class="{ 'is-active': range === tab.id }"
              @click="range = tab.id"
            >
              {{ tab.label }}
            </button>
          </div>
          <button type="button" class="usage-refresh" :disabled="isLoadingUsage" @click="refresh">
            <span :class="{ 'is-spinning': isLoadingUsage }" aria-hidden="true">↻</span>
            {{ isLoadingUsage ? t('USAGE_REFRESHING') : t('USAGE_REFRESH') }}
          </button>
        </div>

        <div v-if="error" class="usage-error" role="alert">
          <strong>{{ t('USAGE_LOAD_FAILED') }}</strong>
          <span>{{ error }}</span>
          <button type="button" @click="refresh">{{ t('USAGE_TRY_AGAIN') }}</button>
        </div>

        <div v-if="isLoadingUsage" class="usage-loading-state" role="status" aria-live="polite">
          <span class="usage-loading-spinner is-spinning" aria-hidden="true">↻</span>
          <div class="usage-loading-copy">
            <strong>{{ t('USAGE_LOADING_DATA') }} {{ usageLoadProgress }}%</strong>
            <small>{{ usageLoadingStage }}</small>
          </div>
          <div class="usage-loading-track" aria-hidden="true"><span :style="{ width: usageLoadProgress + '%' }"></span></div>
        </div>

        <section class="usage-panel usage-filters">
          <div class="usage-panel-heading">
            <div>
              <span class="usage-kicker">{{ t('USAGE_FILTERS') }}</span>
              <h2>{{ t('USAGE_FOCUS_VIEW') }}</h2>
            </div>
            <button v-if="hasActiveFilters" type="button" class="usage-clear" @click="clearFilters">{{ t('USAGE_CLEAR_FILTERS') }}</button>
          </div>
          <div class="usage-filter-grid">
            <label>
              <span>{{ t('USAGE_APP') }}</span>
              <select v-model="appFilter">
                <option value="all">{{ t('USAGE_ALL_APPS') }}</option>
                <option v-for="option in appOptions" :key="option.value" :value="option.value">{{ option.label }} · {{ option.count }}</option>
              </select>
            </label>
            <label>
              <span>{{ t('USAGE_PROVIDER') }}</span>
              <select v-model="providerFilter">
                <option value="all">{{ t('USAGE_ALL_PROVIDERS') }}</option>
                <option v-for="option in providerOptions" :key="option.value" :value="option.value">{{ option.label }} · {{ option.count }}</option>
              </select>
            </label>
            <label>
              <span>{{ t('USAGE_MODEL') }}</span>
              <select v-model="modelFilter">
                <option value="all">{{ t('USAGE_ALL_MODELS') }}</option>
                <option v-for="option in modelOptions" :key="option.value" :value="option.value">{{ option.label }} · {{ option.count }}</option>
              </select>
            </label>
            <div class="usage-filter-status">
              <span class="usage-dot" aria-hidden="true"></span>
              <strong>{{ t('USAGE_VISIBLE_ROWS', { count: filteredRows.length }) }}</strong>
              <small>{{ activeRangeLabel }} · {{ sourceLabel }}</small>
            </div>
          </div>
        </section>

        <section class="usage-metrics" :aria-label="t('USAGE_SUMMARY')">
          <article v-for="card in metricCards" :key="card.id" class="usage-metric" :class="'usage-metric-' + card.id">
            <span>{{ card.label }}</span>
            <strong :title="card.title">{{ card.value }}</strong>
            <small>{{ card.detail }}</small>
          </article>
        </section>

        <section class="usage-panel">
          <div class="usage-panel-heading">
            <div>
              <span class="usage-kicker">{{ t('USAGE_ACTIVITY') }}</span>
              <h2>{{ t('USAGE_TOKEN_TREND') }}</h2>
            </div>
            <div class="usage-peak">
              <strong>{{ formatCompact(trend.maxValue) }}</strong>
              <span>{{ t('USAGE_PEAK_TOKENS_PER_BUCKET') }}</span>
            </div>
          </div>

          <div class="usage-trend-layout">
            <div class="usage-chart-wrap">
              <svg class="usage-chart" viewBox="0 0 760 230" role="img" :aria-label="t('USAGE_TOKEN_TREND_ARIA')">
                <defs>
                  <linearGradient id="usageTrendFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stop-color="#83b97f" stop-opacity="0.32" />
                    <stop offset="100%" stop-color="#83b97f" stop-opacity="0.02" />
                  </linearGradient>
                  <linearGradient id="usageTrendStroke" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0%" stop-color="#6d9b73" />
                    <stop offset="100%" stop-color="#c28b58" />
                  </linearGradient>
                </defs>
                <g class="usage-chart-grid">
                  <line v-for="line in trend.gridLines" :key="line" x1="24" x2="736" :y1="line" :y2="line" />
                </g>
                <path v-if="trend.points.length" class="usage-chart-area" :d="trend.areaPath" />
                <polyline v-if="trend.points.length" class="usage-chart-line" :points="trend.linePoints" />
                <g v-for="point in trend.points" :key="point.key" class="usage-chart-point">
                  <circle :cx="point.x" :cy="point.y" r="4">
                    <title>{{ point.label }} · {{ formatNumber(point.value) }} {{ t('USAGE_TOKENS') }}</title>
                  </circle>
                  <text :x="point.x" y="211" text-anchor="middle">{{ point.label }}</text>
                </g>
              </svg>
              <div v-if="!trend.points.length" class="usage-chart-empty">{{ t('USAGE_NO_TOKEN_ACTIVITY') }}</div>
            </div>
            <aside class="usage-trend-aside">
              <div><span>{{ t('USAGE_LATEST_BUCKET') }}</span><strong>{{ trend.latestLabel || '—' }}</strong></div>
              <div><span>{{ t('USAGE_LATEST_TOKENS') }}</span><strong>{{ formatCompact(trend.latestValue) }}</strong></div>
              <div><span>{{ t('USAGE_RANGE') }}</span><strong>{{ activeRangeLabel }}</strong></div>
              <p>{{ t('USAGE_LOCAL_FALLBACK_NOTE') }}</p>
            </aside>
          </div>
        </section>

        <section class="usage-panel usage-details">
          <div class="usage-panel-heading">
            <div>
              <span class="usage-kicker">{{ t('USAGE_DETAILS') }}</span>
              <h2>{{ t('USAGE_BREAKDOWN') }}</h2>
            </div>
            <div class="usage-details-meta">
              <span>{{ t('USAGE_ROWS', { count: filteredRows.length }) }}</span>
                <span v-if="sourceRows.some(row => row.synthetic)" class="usage-session-badge">{{ t('USAGE_LOCAL_SESSION_SOURCE') }}</span>
            </div>
          </div>

          <div class="usage-data-tabs" role="tablist" :aria-label="t('USAGE_BREAKDOWN_TABS')">
            <button
              v-for="tab in usageTabs"
              :key="tab.id"
              type="button"
              :class="{ 'is-active': activeTab === tab.id }"
              @click="activeTab = tab.id"
            >
              {{ tab.label }}
            </button>
          </div>

          <div v-if="activeTab === 'requests'" class="usage-table-wrap">
            <table class="usage-table">
              <thead>
                <tr><th>{{ t('USAGE_TIME') }}</th><th>{{ t('USAGE_PROVIDER') }}</th><th>{{ t('USAGE_MODEL') }}</th><th>{{ t('USAGE_INPUT') }}</th><th>{{ t('USAGE_OUTPUT') }}</th><th>{{ t('USAGE_COST') }}</th><th>{{ t('USAGE_LATENCY') }}</th><th>{{ t('USAGE_STATUS') }}</th><th>{{ t('USAGE_SOURCE') }}</th></tr>
              </thead>
              <tbody>
                <tr v-for="row in filteredRows" :key="row.id">
                  <td><strong>{{ formatDateTime(row.timestamp) }}</strong><small>{{ formatDateOnly(row.timestamp) }}</small></td>
                  <td><strong>{{ row.provider }}</strong><small>{{ row.appLabel }}</small></td>
                  <td>
                    <span
                      class="usage-model-tooltip"
                      :class="{ 'has-model-costs': modelCostTooltipItems(row).length > 1 }"
                      :title="modelCostTooltipText(row)"
                      :data-tooltip="modelCostTooltipText(row)"
                      :tabindex="modelCostTooltipItems(row).length > 1 ? 0 : undefined"
                      :aria-label="modelCostTooltipText(row) || undefined"
                    >
                      <strong class="usage-model">{{ row.model }}</strong>
                    </span>
                    <small v-if="row.route">{{ row.route }}</small>
                  </td>
                  <td><strong>{{ formatNumber(row.inputTokens) }}</strong><small v-if="row.cacheReadTokens">{{ t('USAGE_CACHE_READ', { value: formatCompact(row.cacheReadTokens) }) }}</small></td>
                  <td><strong>{{ formatNumber(row.outputTokens) }}</strong><small v-if="row.reasoningTokens">{{ t('USAGE_REASONING', { value: formatCompact(row.reasoningTokens) }) }}</small></td>
                  <td><span :class="{ 'is-muted': row.cost == null }">{{ formatCost(row.cost) }}</span></td>
                  <td>{{ formatLatency(row.latencyMs) }}</td>
                  <td><span class="usage-status" :class="statusClass(row)">{{ statusLabel(row) }}</span></td>
                  <td><code>{{ row.source }}</code></td>
                </tr>
                <tr v-if="!filteredRows.length"><td colspan="9" class="usage-empty">{{ isLoadingUsage ? t('USAGE_LOADING_DATA') : t('USAGE_NO_REQUESTS') }}</td></tr>
              </tbody>
            </table>
          </div>

          <div v-else-if="activeTab === 'providers'" class="usage-table-wrap">
            <table class="usage-table usage-summary-table">
              <thead><tr><th>{{ t('USAGE_PROVIDER') }}</th><th>{{ t('USAGE_REQUESTS_HEADER') }}</th><th>{{ t('USAGE_TOTAL_TOKENS') }}</th><th>{{ t('USAGE_INPUT') }}</th><th>{{ t('USAGE_OUTPUT') }}</th><th>{{ t('USAGE_SUCCESS_RATE') }}</th><th>{{ t('USAGE_AVERAGE_LATENCY') }}</th></tr></thead>
              <tbody>
                <tr v-for="item in providerStats" :key="item.key">
                  <td><strong>{{ item.label }}</strong><small>{{ item.appLabels.join(' · ') }}</small></td>
                  <td>{{ formatNumber(item.requests) }}</td><td>{{ formatNumber(item.totalTokens) }}</td><td>{{ formatNumber(item.inputTokens) }}</td><td>{{ formatNumber(item.outputTokens) }}</td><td>{{ formatPercent(item.successRate) }}</td><td>{{ formatLatency(item.averageLatency) }}</td>
                </tr>
                <tr v-if="!providerStats.length"><td colspan="7" class="usage-empty">{{ t('USAGE_NO_PROVIDER_DATA') }}</td></tr>
              </tbody>
            </table>
          </div>

          <div v-else class="usage-table-wrap">
            <table class="usage-table usage-summary-table">
              <thead><tr><th>{{ t('USAGE_MODEL') }}</th><th>{{ t('USAGE_REQUESTS_HEADER') }}</th><th>{{ t('USAGE_TOTAL_TOKENS') }}</th><th>{{ t('USAGE_INPUT') }}</th><th>{{ t('USAGE_OUTPUT') }}</th><th>{{ t('USAGE_SUCCESS_RATE') }}</th><th>{{ t('USAGE_AVERAGE_LATENCY') }}</th></tr></thead>
              <tbody>
                <tr v-for="item in modelStats" :key="item.key">
                  <td>
                    <span
                      class="usage-model-tooltip"
                      :class="{ 'has-model-costs': modelCostTooltipItems(item).length > 1 }"
                      :title="modelCostTooltipText(item)"
                      :data-tooltip="modelCostTooltipText(item)"
                      :tabindex="modelCostTooltipItems(item).length > 1 ? 0 : undefined"
                      :aria-label="modelCostTooltipText(item) || undefined"
                    >
                      <strong class="usage-model">{{ item.label }}</strong>
                    </span>
                    <small>{{ item.providerLabels.join(' · ') }}</small>
                  </td>
                  <td>{{ formatNumber(item.requests) }}</td><td>{{ formatNumber(item.totalTokens) }}</td><td>{{ formatNumber(item.inputTokens) }}</td><td>{{ formatNumber(item.outputTokens) }}</td><td>{{ formatPercent(item.successRate) }}</td><td>{{ formatLatency(item.averageLatency) }}</td>
                </tr>
                <tr v-if="!modelStats.length"><td colspan="7" class="usage-empty">{{ t('USAGE_NO_MODEL_DATA') }}</td></tr>
              </tbody>
            </table>
          </div>

          <p v-if="!summary.costKnown" class="usage-note">{{ t('USAGE_COST_UNAVAILABLE', { dash: '—' }) }}</p>
        </section>
      </main>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import AppHeader from '../components/AppHeader.vue';
import {
  getUsagePreloadSnapshot,
  listAdvancedProxyRequestRecords,
  preloadUsageData,
  USAGE_PRELOAD_EVENT,
} from '../utils/advancedProxyBridge.js';
import { getAppliedThemeMode, isDarkThemeMode, THEME_MODE_CHANGE_EVENT } from '../utils/theme.js';

const props = defineProps({
  embedded: {
    type: Boolean,
    default: false,
  },
});
const { t, locale } = useI18n();
const rangeTabs = computed(() => [
  { id: 'today', label: t('USAGE_TODAY') },
  { id: '7d', label: t('USAGE_7_DAYS') },
  { id: '30d', label: t('USAGE_30_DAYS') },
  { id: 'all', label: t('USAGE_ALL_TIME') },
]);
const usageTabs = computed(() => [
  { id: 'requests', label: t('USAGE_REQUEST_LOGS') },
  { id: 'providers', label: t('USAGE_PROVIDER_STATS') },
  { id: 'models', label: t('USAGE_MODEL_STATS') },
]);
const appLabels = { codex: 'Codex', claude: 'Claude', 'claude-desktop': 'Claude Desktop', gemini: 'Gemini', grok: 'Grok', openclaw: 'OpenClaw', opencode: 'OpenCode' };
const initialUsageSnapshot = getUsagePreloadSnapshot();
const records = ref(Array.isArray(initialUsageSnapshot.records) ? initialUsageSnapshot.records : []);
const analytics = ref(initialUsageSnapshot.analytics && typeof initialUsageSnapshot.analytics === 'object' ? initialUsageSnapshot.analytics : null);
const preloadState = ref(initialUsageSnapshot);
const loading = ref(initialUsageSnapshot.loading === true);
const error = ref('');
const range = ref('today');
const activeTab = ref('requests');
const appFilter = ref('all');
const providerFilter = ref('all');
const modelFilter = ref('all');
const isDarkMode = ref(isDarkThemeMode(getAppliedThemeMode()));
const backgroundLoading = ref(false);
const historyLoaded = ref(false);
const loadProgress = ref(initialUsageSnapshot.loading ? initialUsageSnapshot.progress : 0);
let refreshGeneration = 0;
let historyGeneration = 0;

const isLoadingUsage = computed(() => loading.value || backgroundLoading.value || preloadState.value.loading === true);
const usageLoadProgress = computed(() => Math.max(1, Math.min(100, Math.round(loadProgress.value || preloadState.value.progress || 0))));
const usageLoadingStage = computed(() => {
  if (backgroundLoading.value) return t('USAGE_LOADING_HISTORY');
  if (preloadState.value.loading) {
    return preloadState.value.stage === 'records' ? t('USAGE_LOADING_RECORDS') : t('USAGE_LOADING_ANALYTICS');
  }
  return t('USAGE_LOADING_DATA');
});

function text(value) { return String(value ?? '').trim(); }
function number(value) {
  if (value === null || value === undefined || value === '') return null;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}
function firstNumber() {
  for (const value of arguments) {
    const parsed = number(value);
    if (parsed !== null) return Math.max(0, parsed);
  }
  return 0;
}
function nullableNumber() {
  for (const value of arguments) {
    const parsed = number(value);
    if (parsed !== null) return Math.max(0, parsed);
  }
  return null;
}
function timestamp(value) {
  if (typeof value === 'number' && Number.isFinite(value)) return value < 100000000000 ? value * 1000 : value;
  const parsed = Date.parse(text(value));
  return Number.isFinite(parsed) ? parsed : 0;
}
function appName(value) {
  const key = text(value).toLowerCase().replace(/_/g, '-');
  if (appLabels[key]) return appLabels[key];
  if (!key) return t('USAGE_UNKNOWN_APP');
  return key.split('-').filter(Boolean).map(part => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
}
function usageObject(record) {
  if (record && record.usage && typeof record.usage === 'object') return record.usage;
  if (record && record.tokenUsage && typeof record.tokenUsage === 'object') return record.tokenUsage;
  return {};
}

function normalizeModelCosts(value) {
  let source = [];
  if (Array.isArray(value)) {
    source = value;
  } else if (value && typeof value === 'object') {
    source = Object.entries(value).map(([model, cost]) => ({ model, cost }));
  }
  const map = new Map();
  source.forEach(item => {
    const model = text(item && (item.model || item.modelName || item.name || item.key));
    const cost = nullableNumber(item && (item.cost ?? item.totalCost ?? item.costUsd ?? item.amount));
    const tokens = nullableNumber(item && (item.tokens ?? item.tokenCount ?? item.totalTokens));
    const costKnown = item && typeof item.costKnown === 'boolean' ? item.costKnown : cost !== null;
    if (!model || (cost === null && tokens === null)) return;
    const key = model.toLowerCase();
    const current = map.get(key) || {
      model: model,
      modelName: text(item && (item.modelName || item.name)) || model,
      cost: 0,
      tokens: 0,
      costKnown: false,
    };
    if (cost !== null) current.cost += cost;
    if (tokens !== null) current.tokens += tokens;
    current.costKnown = current.costKnown || costKnown;
    map.set(key, current);
  });
  return Array.from(map.values()).sort((left, right) => {
    if (left.costKnown && right.costKnown && left.cost !== right.cost) return right.cost - left.cost;
    if (left.tokens !== right.tokens) return right.tokens - left.tokens;
    return left.model.localeCompare(right.model);
  });
}

function normalizeRecord(record, index) {
  const usage = usageObject(record);
  const appType = text(record && record.appType).toLowerCase() || 'proxy';
  const appLabel = appName(appType);
  const provider = text(record && record.providerName) || appLabel;
  const model = text(record && (record.model || record.modelName || record.requestModel)) || text(usage.model || usage.modelName) || '—';
  const recordedAt = timestamp(record && (record.recordedAt || record.createdAt || record.updatedAt || record.timestamp || record.time));
  const code = nullableNumber(record && record.statusCode);
  const inputTokens = firstNumber(record && record.inputTokens, record && record.promptTokens, usage.inputTokens, usage.input_tokens, usage.promptTokens, usage.prompt_tokens);
  const outputTokens = firstNumber(record && record.outputTokens, record && record.completionTokens, usage.outputTokens, usage.output_tokens, usage.completionTokens, usage.completion_tokens);
  const reasoningTokens = firstNumber(record && record.reasoningTokens, usage.reasoningTokens, usage.reasoning_tokens, usage.reasoning_output_tokens);
  const totalTokens = firstNumber(record && record.totalTokens, usage.totalTokens, usage.total_tokens, inputTokens + outputTokens + reasoningTokens);
  const cost = nullableNumber(record && record.cost, record && record.totalCost, record && record.costUsd, usage.cost, usage.totalCost, usage.total_cost, usage.costUsd, record && record.pricing && (record.pricing.cost ?? record.pricing.totalCost ?? record.pricing.total_cost));
  const latencyMs = nullableNumber(record && record.latencyMs, record && record.durationMs, record && record.duration);
  const cacheReadTokens = firstNumber(record && record.cacheReadTokens, record && record.cache_read_input_tokens, usage.cacheReadTokens, usage.cachedInputTokens, usage.cached_input_tokens, usage.cache_read_input_tokens, usage.cache_read_tokens);
  const statusText = text(record && record.status).toLowerCase();
  const success = code !== null ? code >= 200 && code < 400 : ['ok', 'success', 'completed', 'complete'].includes(statusText);
  const route = text(record && (record.outboundRoute || record.clientRoute));
  return {
    id: text(record && record.id) || ['request', recordedAt || Date.now(), index].join('-'),
    timestamp: recordedAt,
    provider: provider,
    providerKey: provider,
    appType: appType,
    appLabel: appLabel,
    model: model,
    inputTokens: inputTokens,
    outputTokens: outputTokens,
    reasoningTokens: reasoningTokens,
    totalTokens: totalTokens,
    cacheReadTokens: cacheReadTokens,
    cost: cost,
    latencyMs: latencyMs,
    statusCode: code === null ? 0 : Math.round(code),
    success: success,
    source: text(record && record.source) || route || appType,
    route: route,
    modelCosts: normalizeModelCosts(record && (record.modelCosts || record.model_costs)),
    requestCount: 1,
    synthetic: false,
  };
}

function buildLocalSessionRow(session, index) {
  const source = text(session && session.source);
  const appType = text(session && session.appType).toLowerCase()
    || (source.toLowerCase().startsWith('claude') ? 'claude' : 'codex');
  const appLabel = appName(appType);
  const sourceLabel = text(session && session.sourceLabel) || appLabel;
  const provider = text(session && session.provider) || sourceLabel + ' (Session)';
  const inputTokens = firstNumber(session && session.inputTokens);
  const outputTokens = firstNumber(session && session.outputTokens);
  const reasoningTokens = firstNumber(session && session.reasoningTokens);
  const cacheReadTokens = firstNumber(session && (session.cacheReadTokens ?? session.cache_read_tokens));
  const totalTokens = firstNumber(session && session.totalTokens, inputTokens + outputTokens + reasoningTokens);
  const recordedAt = timestamp(session && (session.timestamp || session.updatedAt || session.createdAt)) || Date.now();
  return {
    id: text(session && session.id) || [appType + '-session', recordedAt, index].join('-'),
    timestamp: recordedAt,
    provider: provider,
    providerKey: provider,
    appType: appType,
    appLabel: appLabel,
    model: text(session && (session.modelName || session.model)) || '—',
    inputTokens: inputTokens,
    outputTokens: outputTokens,
    reasoningTokens: reasoningTokens,
    totalTokens: totalTokens,
    cacheReadTokens: cacheReadTokens,
    cost: nullableNumber(session && (session.cost ?? session.totalCost ?? session.costUsd)),
    latencyMs: null,
    statusCode: 200,
    success: true,
    source: source || appType + '_session',
    route: text(session && session.modelName) && text(session && session.model) && text(session.modelName) !== text(session.model) ? text(session.model) : '',
    modelCosts: normalizeModelCosts(session && (session.modelCosts || session.model_costs)),
    requestCount: Math.max(1, Math.round(firstNumber(session && session.requestCount))),
    synthetic: true,
  };
}

function sessionRows(localAnalytics) {
  const sessions = Array.isArray(localAnalytics && localAnalytics.sessions) ? localAnalytics.sessions : [];
  if (sessions.length) return sessions.map(buildLocalSessionRow);

  const series = Array.isArray(localAnalytics && localAnalytics.series) ? localAnalytics.series : [];
  const rows = series.map((point, index) => {
    const date = text(point && point.date);
    const hour = text(point && point.hour).padStart(2, '0');
    const source = text(point && point.source);
    const appType = text(point && point.appType).toLowerCase()
      || (source.toLowerCase().startsWith('claude') ? 'claude' : 'codex');
    return buildLocalSessionRow({
      id: [appType + '-session', date || 'unknown', hour || '00', index].join('-'),
      timestamp: date ? date + 'T' + hour + ':00:00' : '',
      appType: appType,
      source: source || appType + '_session',
      sourceLabel: text(point && point.sourceLabel),
      model: point && point.model,
      inputTokens: point && point.inputTokens,
      outputTokens: point && point.outputTokens,
      reasoningTokens: point && point.reasoningTokens,
      cacheReadTokens: point && point.cacheReadTokens,
      totalTokens: point && point.totalTokens,
      cost: point && point.cost,
      modelCosts: point && (point.modelCosts || point.model_costs),
      requestCount: point && point.sessionCount,
    }, index);
  });
  if (rows.length || !(localAnalytics && localAnalytics.totalTokens)) return rows;
  return [buildLocalSessionRow({
    id: 'local-session-total',
    timestamp: Date.now(),
    appType: text(localAnalytics && localAnalytics.source) === 'claude' ? 'claude' : 'codex',
    source: text(localAnalytics && localAnalytics.source) || 'local_sessions',
    sourceLabel: text(localAnalytics && localAnalytics.sourceLabel),
    model: localAnalytics.model,
    inputTokens: localAnalytics.inputTokens,
    outputTokens: localAnalytics.outputTokens,
    reasoningTokens: localAnalytics.reasoningTokens,
    cacheReadTokens: localAnalytics.cacheReadTokens,
    totalTokens: localAnalytics.totalTokens,
    cost: localAnalytics.cost,
    requestCount: localAnalytics.sessionCount,
  }, 0)];
}

const requestRows = computed(() => (Array.isArray(records.value) ? records.value : []).map(normalizeRecord));
const localRows = computed(() => sessionRows(analytics.value));
const sourceRows = computed(() => {
  const requests = requestRows.value;
  const sessions = localRows.value;
  if (!sessions.length) return requests;
  if (!requests.length) return sessions;

  // Local session logs are authoritative for the app types they cover.
  // Keep proxy rows for other app types as a live-data supplement.
  const localAppTypes = new Set(sessions.map(row => row.appType).filter(Boolean));
  return [...requests.filter(row => !localAppTypes.has(row.appType)), ...sessions];
});
const activeRangeLabel = computed(() => (rangeTabs.value.find(item => item.id === range.value) || rangeTabs.value[1]).label);
const sourceLabel = computed(() => localRows.value.length ? t('USAGE_LOCAL_SESSIONS') : t('USAGE_PROXY_REQUEST_RECORDS'));

const rangeStart = computed(() => {
  if (range.value === 'all') return 0;
  if (range.value === 'today') {
    const date = new Date();
    date.setHours(0, 0, 0, 0);
    return date.getTime();
  }
  const days = range.value === '30d' ? 30 : 7;
  const date = new Date();
  date.setHours(0, 0, 0, 0);
  date.setDate(date.getDate() - days + 1);
  return date.getTime();
});

const rowsInRange = computed(() => sourceRows.value
  .filter(row => rangeStart.value === 0 || (row.timestamp && row.timestamp >= rangeStart.value))
  .slice()
  .sort((left, right) => right.timestamp - left.timestamp));

function filterOptions(rows, valueKey, labelKey) {
  const map = new Map();
  rows.forEach(row => {
    const value = text(row[valueKey]);
    if (!value) return;
    const item = map.get(value) || { value: value, label: text(row[labelKey]) || value, count: 0 };
    item.count += row.requestCount || 1;
    map.set(value, item);
  });
  return Array.from(map.values()).sort((left, right) => right.count - left.count || left.label.localeCompare(right.label));
}
const appOptions = computed(() => filterOptions(rowsInRange.value, 'appType', 'appLabel'));
const providerOptions = computed(() => filterOptions(rowsInRange.value, 'providerKey', 'provider'));
const modelOptions = computed(() => filterOptions(rowsInRange.value, 'model', 'model'));
const filteredRows = computed(() => rowsInRange.value.filter(row => (
  (appFilter.value === 'all' || row.appType === appFilter.value)
  && (providerFilter.value === 'all' || row.providerKey === providerFilter.value)
  && (modelFilter.value === 'all' || row.model === modelFilter.value)
)));
const hasActiveFilters = computed(() => appFilter.value !== 'all' || providerFilter.value !== 'all' || modelFilter.value !== 'all');

const summary = computed(() => {
  const rows = filteredRows.value;
  const requests = rows.reduce((sum, row) => sum + (row.requestCount || 1), 0);
  const inputTokens = rows.reduce((sum, row) => sum + row.inputTokens, 0);
  const outputTokens = rows.reduce((sum, row) => sum + row.outputTokens, 0);
  const reasoningTokens = rows.reduce((sum, row) => sum + row.reasoningTokens, 0);
  const totalTokens = rows.reduce((sum, row) => sum + row.totalTokens, 0) || inputTokens + outputTokens + reasoningTokens;
  const successful = rows.reduce((sum, row) => sum + (row.success ? row.requestCount || 1 : 0), 0);
  const latencyRows = rows.filter(row => row.latencyMs !== null);
  const pricedRows = rows.filter(row => row.cost !== null);
  return {
    requests: requests,
    rows: rows.length,
    inputTokens: inputTokens,
    outputTokens: outputTokens,
    reasoningTokens: reasoningTokens,
    totalTokens: totalTokens,
    cacheReadTokens: rows.reduce((sum, row) => sum + row.cacheReadTokens, 0),
    successRate: requests ? successful / requests * 100 : null,
    averageLatency: latencyRows.length ? latencyRows.reduce((sum, row) => sum + row.latencyMs, 0) / latencyRows.length : null,
    cost: pricedRows.length ? pricedRows.reduce((sum, row) => sum + row.cost, 0) : null,
    costKnown: pricedRows.length > 0,
  };
});

const metricCards = computed(() => [
  { id: 'requests', label: t('USAGE_REQUESTS'), value: formatCompact(summary.value.requests), title: formatNumber(summary.value.requests), detail: t('USAGE_TRACKED_POINTS', { count: summary.value.rows }) },
  { id: 'tokens', label: t('USAGE_TOTAL_TOKENS_LABEL'), value: formatCompact(summary.value.totalTokens), title: formatNumber(summary.value.totalTokens), detail: t('USAGE_INPUT_OUTPUT_REASONING') },
  { id: 'input', label: t('USAGE_INPUT_TOKENS'), value: formatCompact(summary.value.inputTokens), title: formatNumber(summary.value.inputTokens), detail: summary.value.cacheReadTokens ? t('USAGE_CACHE_READ_DETAIL', { value: formatCompact(summary.value.cacheReadTokens) }) : t('USAGE_PROMPT_TOKENS') },
  { id: 'output', label: t('USAGE_OUTPUT_TOKENS'), value: formatCompact(summary.value.outputTokens), title: formatNumber(summary.value.outputTokens), detail: summary.value.reasoningTokens ? t('USAGE_REASONING_DETAIL', { value: formatCompact(summary.value.reasoningTokens) }) : t('USAGE_COMPLETION_TOKENS') },
  { id: 'success', label: t('USAGE_SUCCESS_RATE'), value: formatPercent(summary.value.successRate), title: '', detail: t('USAGE_AVERAGE_LATENCY_DETAIL', { value: formatLatency(summary.value.averageLatency) }) },
  { id: 'cost', label: t('USAGE_COST'), value: formatCost(summary.value.cost), title: '', detail: summary.value.costKnown ? t('USAGE_BASED_ON_RECORD_PRICING') : t('USAGE_NOT_PRICED') },
]);

function mergeModelCosts(existing, additions) {
  if (!Array.isArray(additions) || !additions.length) return Array.isArray(existing) ? existing : [];
  const map = new Map();
  [...(Array.isArray(existing) ? existing : []), ...additions].forEach(item => {
    const model = text(item && (item.model || item.modelName || item.name));
    const cost = nullableNumber(item && (item.cost ?? item.totalCost ?? item.costUsd ?? item.amount));
    const tokens = nullableNumber(item && (item.tokens ?? item.tokenCount ?? item.totalTokens));
    const costKnown = item && typeof item.costKnown === 'boolean' ? item.costKnown : cost !== null;
    if (!model || (cost === null && tokens === null)) return;
    const key = model.toLowerCase();
    const current = map.get(key) || { model, modelName: text(item && (item.modelName || item.name)) || model, cost: 0, tokens: 0, costKnown: false };
    if (cost !== null) current.cost += cost;
    if (tokens !== null) current.tokens += tokens;
    current.costKnown = current.costKnown || costKnown;
    map.set(key, current);
  });
  return Array.from(map.values()).sort((left, right) => {
    if (left.costKnown && right.costKnown && left.cost !== right.cost) return right.cost - left.cost;
    if (left.tokens !== right.tokens) return right.tokens - left.tokens;
    return left.model.localeCompare(right.model);
  });
}

function modelCostTooltipItems(source) {
  const entries = normalizeModelCosts(source && source.modelCosts);
  if (entries.length < 2) return [];
  const totalCost = entries.reduce((sum, item) => sum + (item.costKnown ? item.cost : 0), 0);
  const useCost = totalCost > 0;
  const totalTokens = entries.reduce((sum, item) => sum + item.tokens, 0);
  if (!useCost && !(totalTokens > 0)) return [];
  return entries.map(item => ({
    ...item,
    label: item.modelName || item.model,
    metric: useCost ? formatCost(item.cost) : `${formatNumber(item.tokens)} ${t('USAGE_TOKENS')}`,
    percent: (useCost ? (item.costKnown ? item.cost : 0) : item.tokens) / (useCost ? totalCost : totalTokens) * 100,
  }));
}

function modelCostTooltipText(source) {
  return modelCostTooltipItems(source)
    .map(item => `${item.label}: ${item.metric} (${formatModelCostPercent(item.percent)})`)
    .join('\n');
}

function formatModelCostPercent(value) {
  const parsed = number(value);
  return parsed === null ? '—' : parsed.toFixed(1) + '%';
}

function groupedStats(rows, keyName, labelName) {
  const map = new Map();
  rows.forEach(row => {
    const key = text(row[keyName]) || 'Unknown';
    const item = map.get(key) || { key: key, label: text(row[labelName]) || key, requests: 0, totalTokens: 0, inputTokens: 0, outputTokens: 0, cost: 0, costKnown: false, modelCosts: [], success: 0, latency: 0, latencyRows: 0, appLabels: new Set(), providerLabels: new Set() };
    const count = row.requestCount || 1;
    item.requests += count;
    item.totalTokens += row.totalTokens;
    item.inputTokens += row.inputTokens;
    item.outputTokens += row.outputTokens;
    if (row.cost !== null) {
      item.cost += row.cost;
      item.costKnown = true;
    }
    item.modelCosts = mergeModelCosts(item.modelCosts, row.modelCosts);
    item.success += row.success ? count : 0;
    if (row.latencyMs !== null) {
      item.latency += row.latencyMs;
      item.latencyRows += 1;
    }
    item.appLabels.add(row.appLabel);
    item.providerLabels.add(row.provider);
    map.set(key, item);
  });
  return Array.from(map.values()).map(item => ({
    key: item.key,
    label: item.label,
    requests: item.requests,
    totalTokens: item.totalTokens,
    inputTokens: item.inputTokens,
    outputTokens: item.outputTokens,
    cost: item.costKnown ? item.cost : null,
    modelCosts: item.modelCosts,
    successRate: item.requests ? item.success / item.requests * 100 : null,
    averageLatency: item.latencyRows ? item.latency / item.latencyRows : null,
    appLabels: Array.from(item.appLabels),
    providerLabels: Array.from(item.providerLabels),
  })).sort((left, right) => right.totalTokens - left.totalTokens || right.requests - left.requests || left.label.localeCompare(right.label));
}
const providerStats = computed(() => groupedStats(filteredRows.value, 'providerKey', 'provider'));
const modelStats = computed(() => groupedStats(filteredRows.value, 'model', 'model'));

const trend = computed(() => {
  const buckets = new Map();
  filteredRows.value.forEach(row => {
    const date = new Date(row.timestamp);
    if (Number.isNaN(date.getTime())) return;
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hour = String(date.getHours()).padStart(2, '0');
    const hourly = range.value === 'today';
    const key = hourly ? [year, month, day, hour].join('-') : [year, month, day].join('-');
    const label = hourly ? hour + ':00' : month + '/' + day;
    const item = buckets.get(key) || { key: key, label: label, value: 0 };
    item.value += row.totalTokens;
    buckets.set(key, item);
  });
  const maxPoints = range.value === 'today' ? 24 : range.value === 'all' ? 30 : 14;
  const items = Array.from(buckets.values()).sort((left, right) => left.key.localeCompare(right.key)).slice(-maxPoints);
  const maxValue = Math.max(0, ...items.map(item => item.value));
  const safeMax = maxValue || 1;
  const points = items.map((item, index) => ({
    key: item.key,
    label: item.label,
    value: item.value,
    x: items.length === 1 ? 380 : 24 + index / (items.length - 1) * 712,
    y: 182 - item.value / safeMax * 150,
  }));
  const linePoints = points.map(point => point.x + ',' + point.y).join(' ');
  const linePath = points.map((point, index) => (index ? 'L ' : 'M ') + point.x + ' ' + point.y).join(' ');
  const areaPath = points.length ? 'M ' + points[0].x + ' 182 L ' + points.map(point => point.x + ' ' + point.y).join(' L ') + ' L ' + points[points.length - 1].x + ' 182 Z' : '';
  const last = points.length ? points[points.length - 1] : null;
  return {
    points: points,
    maxValue: maxValue,
    linePoints: linePoints,
    linePath: linePath,
    areaPath: areaPath,
    latestLabel: last ? last.label : '',
    latestValue: last ? last.value : 0,
    gridLines: [0, 0.25, 0.5, 0.75, 1].map(ratio => 182 - ratio * 150),
  };
});

function formatNumber(value) {
  const parsed = Number(value || 0);
  return Number.isFinite(parsed) ? new Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 0 }).format(parsed) : '0';
}
function formatCompact(value) {
  const parsed = Number(value || 0);
  if (!Number.isFinite(parsed)) return '0';
  if (parsed >= 1000000000) return (parsed / 1000000000).toFixed(parsed >= 10000000000 ? 0 : 1) + 'B';
  if (parsed >= 1000000) return (parsed / 1000000).toFixed(parsed >= 10000000 ? 0 : 1) + 'M';
  if (parsed >= 1000) return (parsed / 1000).toFixed(parsed >= 10000 ? 0 : 1) + 'K';
  return formatNumber(parsed);
}
function formatPercent(value) {
  if (value === null || value === undefined || value === '') return '—';
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed.toFixed(parsed >= 99.95 || parsed < 10 ? 1 : 0) + '%' : '—';
}
function formatCost(value) {
  const parsed = number(value);
  if (parsed === null) return '—';
  const digits = parsed !== 0 && Math.abs(parsed) < 0.01 ? 6 : 4;
  return '$' + parsed.toFixed(digits);
}
function formatLatency(value) {
  const parsed = number(value);
  if (parsed === null) return '—';
  return parsed >= 1000 ? (parsed / 1000).toFixed(parsed >= 10000 ? 0 : 1) + 's' : Math.round(parsed) + 'ms';
}
function formatDateTime(value) {
  const date = new Date(value);
  return !value || Number.isNaN(date.getTime()) ? t('USAGE_UNKNOWN_TIME') : date.toLocaleString(locale.value || undefined, { month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false });
}
function formatDateOnly(value) {
  const date = new Date(value);
  return !value || Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: '2-digit' });
}
function statusClass(row) {
  return row.success ? 'is-success' : row.statusCode >= 400 ? 'is-error' : 'is-neutral';
}
function statusLabel(row) {
  return row.statusCode ? String(row.statusCode) : row.success ? t('USAGE_OK') : '—';
}
function clearFilters() {
  appFilter.value = 'all';
  providerFilter.value = 'all';
  modelFilter.value = 'all';
}
async function loadHistoryData() {
  if (historyLoaded.value || backgroundLoading.value) return;
  const generation = ++historyGeneration;
  backgroundLoading.value = true;
  loadProgress.value = 48;
  try {
    const result = await listAdvancedProxyRequestRecords(400);
    if (generation !== historyGeneration) return;
    records.value = Array.isArray(result) ? result : records.value;
    historyLoaded.value = true;
    loadProgress.value = 100;
    error.value = '';
  } catch (loadError) {
    if (generation === historyGeneration) {
      error.value = loadError && loadError.message ? loadError.message : t('USAGE_LOCAL_BRIDGE_ERROR');
    }
  } finally {
    if (generation === historyGeneration) backgroundLoading.value = false;
  }
}

async function refresh(options = {}) {
  const force = options && typeof options === 'object' && options.force === false ? false : true;
  const generation = ++refreshGeneration;
  const cached = getUsagePreloadSnapshot();
  syncPreload({ detail: cached });
  const hasCachedData = records.value.length > 0 || localRows.value.length > 0;
  loading.value = !hasCachedData || cached.loading || force;
  loadProgress.value = cached.loading ? cached.progress : hasCachedData && !force ? 100 : 8;
  error.value = '';

  try {
    const snapshot = await preloadUsageData({ force });
    if (generation !== refreshGeneration) return;
    syncPreload({ detail: snapshot });
    if (snapshot.error && !records.value.length && !localRows.value.length) error.value = snapshot.error;
  } catch (loadError) {
    if (generation === refreshGeneration) {
      error.value = loadError && loadError.message ? loadError.message : t('USAGE_LOCAL_BRIDGE_ERROR');
    }
  } finally {
    if (generation === refreshGeneration) {
      loading.value = false;
      loadProgress.value = 100;
    }
  }

  if (generation !== refreshGeneration) return;
  await nextTick();
  if (range.value !== 'today') {
    historyLoaded.value = false;
    void loadHistoryData();
  }
}
function syncTheme(event) {
  isDarkMode.value = isDarkThemeMode(event && event.detail && event.detail.mode || getAppliedThemeMode());
}
function syncPreload(event) {
  const snapshot = event?.detail && typeof event.detail === 'object' ? event.detail : getUsagePreloadSnapshot();
  preloadState.value = snapshot;
  // A non-today range may already have fetched the larger history window. Do
  // not let the background 120-row preload replace that view when its final
  // event arrives later.
  if (Array.isArray(snapshot.records) && (range.value === 'today' || !historyLoaded.value)) {
    records.value = snapshot.records;
  }
  if (snapshot.analytics && typeof snapshot.analytics === 'object') analytics.value = snapshot.analytics;
  if (snapshot.loading && !backgroundLoading.value) {
    loading.value = true;
    loadProgress.value = snapshot.progress;
  } else if (snapshot.stage === 'ready' && !backgroundLoading.value) {
    loading.value = false;
    loadProgress.value = 100;
  }
}
watch(range, value => {
  if (value !== 'today') void loadHistoryData();
});
onMounted(() => {
  window.addEventListener(THEME_MODE_CHANGE_EVENT, syncTheme);
  window.addEventListener(USAGE_PRELOAD_EVENT, syncPreload);
  syncPreload();
  void refresh({ force: false });
});
onBeforeUnmount(() => {
  window.removeEventListener(THEME_MODE_CHANGE_EVENT, syncTheme);
  window.removeEventListener(USAGE_PRELOAD_EVENT, syncPreload);
});
</script>

<style scoped>
.usage-view {
  --bg: #f2f6ee;
  --surface: rgba(255, 255, 255, 0.88);
  --strong: #fff;
  --border: rgba(83, 111, 82, 0.14);
  --text: #213629;
  --muted: #718071;
  --faint: #9aa79b;
  --accent: #6d9b73;
  --accent-soft: #e4efda;
  min-height: 100vh;
  color: var(--text);
  background: radial-gradient(circle at 12% 0%, rgba(204, 227, 178, 0.42), transparent 29%), linear-gradient(180deg, #f8fbf4, var(--bg));
}
.usage-view-dark {
  --bg: #0b1518;
  --surface: rgba(19, 31, 34, 0.92);
  --strong: #162428;
  --border: rgba(132, 165, 170, 0.18);
  --text: #edf5f0;
  --muted: #a6b8b2;
  --faint: #7d938e;
  --accent: #8cbd91;
  --accent-soft: rgba(91, 132, 101, 0.24);
  background: radial-gradient(circle at 10% 0%, rgba(64, 104, 108, 0.22), transparent 30%), linear-gradient(180deg, #0a1317, var(--bg));
}
.usage-view-embedded { min-height: 0; background: transparent; }
.usage-view-embedded .usage-page-shell { width: 100%; padding: 8px 0 16px; }
.usage-view-embedded .usage-main { gap: 10px; }
.usage-view-embedded .usage-panel { border-radius: 16px; }
.usage-embedded-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 10px; min-width: 0; }
.usage-page-shell { width: min(1480px, calc(100% - 32px)); margin: 0 auto; padding: 16px 0 42px; }
.usage-main { display: grid; gap: 14px; }
.usage-hero, .usage-panel, .usage-metric { border: 1px solid var(--border); background: var(--surface); box-shadow: 0 16px 34px rgba(52, 74, 56, 0.06), inset 0 1px 0 rgba(255, 255, 255, 0.5); backdrop-filter: blur(16px); }
.usage-view-dark .usage-hero, .usage-view-dark .usage-panel, .usage-view-dark .usage-metric { box-shadow: 0 20px 42px rgba(0, 0, 0, 0.2), inset 0 1px 0 rgba(212, 240, 234, 0.04); }
.usage-hero { display: flex; align-items: flex-end; justify-content: space-between; gap: 22px; padding: 24px 26px; border-radius: 24px; }
.usage-kicker { margin: 0 0 6px; color: var(--accent); font-size: 11px; font-weight: 800; letter-spacing: .16em; text-transform: uppercase; }
.usage-hero h1, .usage-panel h2 { margin: 0; color: var(--text); font-weight: 750; letter-spacing: -.03em; }
.usage-hero h1 { font-size: clamp(25px, 3vw, 38px); }
.usage-hero p:last-child { max-width: 620px; margin: 8px 0 0; color: var(--muted); font-size: 13px; line-height: 1.6; }
.usage-actions { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 9px; }
.usage-range-tabs, .usage-data-tabs { display: inline-flex; gap: 3px; padding: 3px; border: 1px solid var(--border); border-radius: 999px; background: rgba(122, 151, 116, .07); }
.usage-range-tabs button, .usage-data-tabs button, .usage-refresh, .usage-clear, .usage-error button { border: 0; font: inherit; cursor: pointer; }
.usage-range-tabs button, .usage-data-tabs button { padding: 8px 12px; border-radius: 999px; color: var(--muted); background: transparent; font-size: 12px; font-weight: 700; }
.usage-range-tabs button.is-active, .usage-data-tabs button.is-active { color: var(--text); background: var(--strong); box-shadow: 0 5px 12px rgba(66, 89, 68, .1); }
.usage-refresh { display: inline-flex; align-items: center; gap: 7px; min-height: 34px; padding: 0 13px; border: 1px solid var(--border); border-radius: 999px; color: var(--text); background: var(--strong); font-size: 12px; font-weight: 750; }
.usage-refresh:disabled { cursor: wait; opacity: .62; }
.usage-refresh span { display: inline-block; font-size: 17px; line-height: 1; }
.is-spinning { animation: usage-spin .8s linear infinite; }
.usage-error { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 12px 15px; border: 1px solid rgba(190, 92, 71, .25); border-radius: 16px; color: #9a4a38; background: rgba(255, 235, 225, .82); font-size: 12px; }
.usage-error span { opacity: .82; }
.usage-error button { margin-left: auto; padding: 5px 9px; border-radius: 8px; color: inherit; background: rgba(255, 255, 255, .46); font-size: 11px; font-weight: 750; }
.usage-loading-state { display: grid; grid-template-columns: auto minmax(0, auto) minmax(120px, 1fr); align-items: center; gap: 10px; min-height: 48px; padding: 9px 13px; border: 1px solid var(--border); border-radius: 14px; color: var(--muted); background: rgba(123, 157, 117, .07); }
.usage-loading-spinner { display: inline-block; color: var(--accent); font-size: 22px; line-height: 1; }
.usage-loading-copy { display: grid; gap: 2px; min-width: 0; }
.usage-loading-copy strong { color: var(--text); font-size: 12px; }
.usage-loading-copy small { overflow: hidden; color: var(--faint); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.usage-loading-track { height: 6px; overflow: hidden; border-radius: 999px; background: rgba(109, 155, 115, .15); }
.usage-loading-track span { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--accent), #c28b58); transition: width .22s ease; }
.usage-panel { display: grid; gap: 15px; padding: 19px; border-radius: 21px; }
.usage-panel-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 14px; }
.usage-panel h2 { font-size: 18px; }
.usage-clear { padding: 6px 9px; border-radius: 8px; color: var(--accent); background: var(--accent-soft); font-size: 11px; font-weight: 800; }
.usage-filter-grid { display: grid; grid-template-columns: repeat(3, minmax(150px, 1fr)) minmax(210px, 1.2fr); gap: 10px; align-items: end; }
.usage-filter-grid label { display: grid; gap: 6px; min-width: 0; }
.usage-filter-grid label > span { color: var(--muted); font-size: 11px; font-weight: 800; letter-spacing: .04em; text-transform: uppercase; }
.usage-filter-grid select { width: 100%; min-height: 35px; padding: 0 10px; border: 1px solid var(--border); border-radius: 10px; outline: none; color: var(--text); background: var(--strong); font: inherit; font-size: 12px; }
.usage-filter-status { display: grid; grid-template-columns: auto auto; align-items: center; column-gap: 7px; row-gap: 2px; min-height: 35px; padding: 0 11px; border: 1px solid var(--border); border-radius: 10px; background: rgba(123, 157, 117, .06); font-size: 12px; }
.usage-filter-status small { grid-column: 2; color: var(--muted); font-size: 10px; }
.usage-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--accent); box-shadow: 0 0 0 4px rgba(109, 155, 115, .12); }
.usage-metrics { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 10px; }
.usage-metric { display: grid; gap: 7px; min-width: 0; min-height: 112px; padding: 15px 16px; border-radius: 17px; }
.usage-metric > span { color: var(--muted); font-size: 11px; font-weight: 800; letter-spacing: .04em; text-transform: uppercase; }
.usage-metric strong { overflow: hidden; color: var(--text); font-size: clamp(20px, 2.2vw, 28px); font-weight: 800; letter-spacing: -.04em; text-overflow: ellipsis; white-space: nowrap; }
.usage-metric small { overflow: hidden; color: var(--faint); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.usage-metric-requests { background: linear-gradient(145deg, rgba(231, 243, 220, .94), rgba(255, 255, 255, .84)); }
.usage-metric-cost { background: linear-gradient(145deg, rgba(249, 239, 222, .9), rgba(255, 255, 255, .84)); }
.usage-view-dark .usage-metric-requests { background: linear-gradient(145deg, rgba(43, 73, 57, .86), rgba(21, 38, 39, .9)); }
.usage-view-dark .usage-metric-cost { background: linear-gradient(145deg, rgba(77, 61, 44, .7), rgba(27, 38, 38, .9)); }
.usage-peak { display: grid; justify-items: end; gap: 2px; }
.usage-peak strong { color: var(--text); font-size: 18px; }
.usage-peak span { color: var(--muted); font-size: 10px; }
.usage-trend-layout { display: grid; grid-template-columns: minmax(0, 1fr) 220px; gap: 14px; min-width: 0; }
.usage-chart-wrap, .usage-trend-aside { min-width: 0; border: 1px solid var(--border); border-radius: 16px; background: rgba(116, 151, 116, .045); }
.usage-chart-wrap { min-height: 244px; padding: 9px 10px 4px; position: relative; }
.usage-chart { display: block; width: 100%; height: 234px; overflow: visible; }
.usage-chart-grid line { stroke: var(--border); stroke-dasharray: 3 6; }
.usage-chart-area { fill: url(#usageTrendFill); }
.usage-chart-line { fill: none; stroke: url(#usageTrendStroke); stroke-linecap: round; stroke-linejoin: round; stroke-width: 3; }
.usage-chart-point circle { fill: var(--strong); stroke: var(--accent); stroke-width: 2.5; }
.usage-chart-point text { fill: var(--muted); font-size: 10px; }
.usage-chart-empty { position: absolute; inset: 0; display: grid; place-items: center; color: var(--muted); font-size: 12px; }
.usage-trend-aside { display: grid; align-content: start; padding: 4px 13px; }
.usage-trend-aside > div { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; padding: 12px 0; border-bottom: 1px solid var(--border); }
.usage-trend-aside span { color: var(--muted); font-size: 11px; }
.usage-trend-aside strong { color: var(--text); font-size: 13px; text-align: right; }
.usage-trend-aside p { margin: 13px 0 9px; color: var(--faint); font-size: 11px; line-height: 1.55; }
.usage-details { gap: 12px; }
.usage-details-meta { display: inline-flex; align-items: center; flex-wrap: wrap; justify-content: flex-end; gap: 8px; color: var(--muted); font-size: 11px; }
.usage-session-badge { padding: 4px 7px; border: 1px solid rgba(109, 155, 115, .2); border-radius: 999px; color: var(--accent); background: var(--accent-soft); font-size: 10px; font-weight: 800; }
.usage-data-tabs { justify-self: start; }
.usage-table-wrap { width: 100%; overflow: auto; border: 1px solid var(--border); border-radius: 15px; background: rgba(255, 255, 255, .18); }
.usage-table { width: 100%; min-width: 920px; border-collapse: collapse; text-align: left; font-size: 12px; }
.usage-summary-table { min-width: 760px; }
.usage-table th { padding: 11px 12px; border-bottom: 1px solid var(--border); color: var(--muted); background: rgba(116, 151, 116, .055); font-size: 10px; font-weight: 850; letter-spacing: .08em; text-transform: uppercase; white-space: nowrap; }
.usage-table td { padding: 12px; border-bottom: 1px solid rgba(111, 143, 115, .1); color: var(--muted); vertical-align: middle; white-space: nowrap; }
.usage-table tbody tr:last-child td { border-bottom: 0; }
.usage-table tbody tr:hover td { background: rgba(109, 155, 115, .055); }
.usage-model-tooltip { position: relative; display: block; max-width: 220px; }
.usage-model-tooltip.has-model-costs { cursor: help; }
.usage-model-tooltip.has-model-costs::after { position: absolute; z-index: 20; left: 0; bottom: calc(100% + 8px); width: max-content; max-width: min(320px, 70vw); padding: 9px 11px; border: 1px solid rgba(132, 165, 170, .25); border-radius: 10px; color: var(--text); background: var(--strong); box-shadow: 0 12px 28px rgba(0, 0, 0, .2); content: attr(data-tooltip); font-size: 11px; font-weight: 600; line-height: 1.55; opacity: 0; pointer-events: none; transform: translateY(4px); transition: opacity .14s ease, transform .14s ease; white-space: pre-line; }
.usage-model-tooltip.has-model-costs:hover::after, .usage-model-tooltip.has-model-costs:focus-visible::after { opacity: 1; transform: translateY(0); }
.usage-table td strong { display: block; max-width: 220px; overflow: hidden; color: var(--text); font-size: 12px; font-weight: 750; text-overflow: ellipsis; white-space: nowrap; }
.usage-table td small { display: block; max-width: 220px; margin-top: 4px; overflow: hidden; color: var(--faint); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.usage-table td span.is-muted { color: var(--faint); }
.usage-status { display: inline-flex; align-items: center; justify-content: center; min-width: 38px; padding: 4px 7px; border-radius: 999px; font-size: 10px; font-weight: 850; }
.usage-status.is-success { color: #3e7b4c; background: rgba(142, 201, 133, .2); }
.usage-status.is-error { color: #ae513e; background: rgba(221, 133, 106, .18); }
.usage-status.is-neutral { color: var(--muted); background: rgba(129, 145, 135, .14); }
.usage-table code, .usage-note code { padding: 3px 6px; border: 1px solid var(--border); border-radius: 6px; color: var(--muted); background: rgba(126, 151, 132, .07); font: 10px/1.2 'Cascadia Code', 'Consolas', monospace; }
.usage-empty { padding: 34px 14px !important; color: var(--faint) !important; text-align: center; }
.usage-note { margin: 0; color: var(--faint); font-size: 11px; line-height: 1.5; }
@keyframes usage-spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
@media (max-width: 1180px) { .usage-metrics { grid-template-columns: repeat(3, minmax(0, 1fr)); } .usage-filter-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 820px) { .usage-page-shell { width: min(100% - 20px, 720px); padding-top: 10px; } .usage-hero { align-items: flex-start; flex-direction: column; padding: 20px; } .usage-actions { width: 100%; justify-content: flex-start; } .usage-trend-layout { grid-template-columns: minmax(0, 1fr); } .usage-trend-aside { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; padding: 0 12px; } .usage-trend-aside > div { display: grid; gap: 4px; padding: 12px 0; border-bottom: 0; } .usage-trend-aside strong { text-align: left; } .usage-trend-aside p { grid-column: 1 / -1; margin-top: 0; } }
@media (max-width: 560px) { .usage-filter-grid, .usage-metrics { grid-template-columns: minmax(0, 1fr); } .usage-panel { padding: 15px; } .usage-panel-heading { align-items: flex-start; flex-direction: column; } .usage-details-meta { justify-content: flex-start; } .usage-trend-aside { grid-template-columns: minmax(0, 1fr); } .usage-trend-aside p { grid-column: auto; } }
@media (max-width: 560px) { .usage-loading-state { grid-template-columns: auto minmax(0, 1fr); } .usage-loading-track { grid-column: 1 / -1; } }
</style>
