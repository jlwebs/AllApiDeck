<template>
  <div class="usage-view" :class="{ 'usage-view-dark': isDarkMode }">
    <div class="usage-page-shell">
      <AppHeader current-page="usage" :is-dark-mode="isDarkMode" />

      <main class="usage-main">
        <section class="usage-hero">
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
            <button type="button" class="usage-refresh" :disabled="loading" @click="refresh">
              <span :class="{ 'is-spinning': loading }" aria-hidden="true">↻</span>
              {{ loading ? t('USAGE_REFRESHING') : t('USAGE_REFRESH') }}
            </button>
          </div>
        </section>

        <div v-if="error" class="usage-error" role="alert">
          <strong>{{ t('USAGE_LOAD_FAILED') }}</strong>
          <span>{{ error }}</span>
          <button type="button" @click="refresh">{{ t('USAGE_TRY_AGAIN') }}</button>
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
              <p>{{ t('USAGE_FALLBACK_NOTE') }}</p>
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
              <span v-if="sourceRows.some(row => row.synthetic)" class="usage-session-badge">{{ t('USAGE_CODEX_SESSION_FALLBACK') }}</span>
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
                  <td><strong class="usage-model">{{ row.model }}</strong><small v-if="row.route">{{ row.route }}</small></td>
                  <td><strong>{{ formatNumber(row.inputTokens) }}</strong><small v-if="row.cacheReadTokens">{{ t('USAGE_CACHE_READ', { value: formatCompact(row.cacheReadTokens) }) }}</small></td>
                  <td><strong>{{ formatNumber(row.outputTokens) }}</strong><small v-if="row.reasoningTokens">{{ t('USAGE_REASONING', { value: formatCompact(row.reasoningTokens) }) }}</small></td>
                  <td><span :class="{ 'is-muted': row.cost == null }">{{ formatCost(row.cost) }}</span></td>
                  <td>{{ formatLatency(row.latencyMs) }}</td>
                  <td><span class="usage-status" :class="statusClass(row)">{{ statusLabel(row) }}</span></td>
                  <td><code>{{ row.source }}</code></td>
                </tr>
                <tr v-if="!filteredRows.length"><td colspan="9" class="usage-empty">{{ loading ? t('USAGE_LOADING_DATA') : t('USAGE_NO_REQUESTS') }}</td></tr>
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
                  <td><strong class="usage-model">{{ item.label }}</strong><small>{{ item.providerLabels.join(' · ') }}</small></td>
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import AppHeader from '../components/AppHeader.vue';
import { getLocalTokenUsageAnalytics, listAdvancedProxyRequestRecords } from '../utils/advancedProxyBridge.js';
import { getAppliedThemeMode, isDarkThemeMode, THEME_MODE_CHANGE_EVENT } from '../utils/theme.js';

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
const records = ref([]);
const analytics = ref(null);
const loading = ref(false);
const error = ref('');
const range = ref('7d');
const activeTab = ref('requests');
const appFilter = ref('all');
const providerFilter = ref('all');
const modelFilter = ref('all');
const isDarkMode = ref(isDarkThemeMode(getAppliedThemeMode()));

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

function normalizeRecord(record, index) {
  const usage = usageObject(record);
  const appType = text(record && record.appType).toLowerCase() || 'proxy';
  const appLabel = appName(appType);
  const provider = text(record && record.providerName) || appLabel;
  const model = text(record && record.model) || '—';
  const recordedAt = timestamp(record && (record.recordedAt || record.createdAt || record.updatedAt || record.timestamp || record.time));
  const code = nullableNumber(record && record.statusCode);
  const inputTokens = firstNumber(record && record.inputTokens, record && record.promptTokens, usage.inputTokens, usage.promptTokens, usage.prompt_tokens);
  const outputTokens = firstNumber(record && record.outputTokens, record && record.completionTokens, usage.outputTokens, usage.completionTokens, usage.completion_tokens);
  const reasoningTokens = firstNumber(record && record.reasoningTokens, usage.reasoningTokens, usage.reasoning_tokens);
  const totalTokens = firstNumber(record && record.totalTokens, usage.totalTokens, usage.total_tokens, inputTokens + outputTokens + reasoningTokens);
  const cost = nullableNumber(record && record.cost, record && record.totalCost, record && record.costUsd, usage.cost, usage.totalCost, usage.costUsd, record && record.pricing && record.pricing.cost);
  const latencyMs = nullableNumber(record && record.latencyMs, record && record.durationMs, record && record.duration);
  const cacheReadTokens = firstNumber(record && record.cacheReadTokens, record && record.cache_read_input_tokens, usage.cacheReadTokens, usage.cache_read_input_tokens, usage.cache_read_tokens);
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
    requestCount: 1,
    synthetic: false,
  };
}

function sessionRows(localAnalytics) {
  const series = Array.isArray(localAnalytics && localAnalytics.series) ? localAnalytics.series : [];
  const rows = series.map((point, index) => {
    const inputTokens = firstNumber(point && point.inputTokens);
    const outputTokens = firstNumber(point && point.outputTokens);
    const reasoningTokens = firstNumber(point && point.reasoningTokens);
    const totalTokens = firstNumber(point && point.totalTokens, inputTokens + outputTokens + reasoningTokens);
    const date = text(point && point.date);
    const hour = text(point && point.hour).padStart(2, '0');
    const recordedAt = timestamp(date ? date + 'T' + hour + ':00:00' : '') || Date.now();
    return {
      id: ['codex-session', date || 'unknown', hour || '00', index].join('-'),
      timestamp: recordedAt,
      provider: 'Codex (Session)',
      providerKey: 'Codex (Session)',
      appType: 'codex',
      appLabel: 'Codex',
      model: '—',
      inputTokens: inputTokens,
      outputTokens: outputTokens,
      reasoningTokens: reasoningTokens,
      totalTokens: totalTokens,
      cacheReadTokens: 0,
      cost: null,
      latencyMs: null,
      statusCode: 200,
      success: true,
      source: 'codex_session',
      route: '',
      requestCount: Math.max(1, Math.round(firstNumber(point && point.sessionCount))),
      synthetic: true,
    };
  });
  if (rows.length || !(localAnalytics && localAnalytics.totalTokens)) return rows;
  return [{
    id: 'codex-session-total',
    timestamp: Date.now(),
    provider: 'Codex (Session)',
    providerKey: 'Codex (Session)',
    appType: 'codex',
    appLabel: 'Codex',
    model: '—',
    inputTokens: firstNumber(localAnalytics.inputTokens),
    outputTokens: firstNumber(localAnalytics.outputTokens),
    reasoningTokens: firstNumber(localAnalytics.reasoningTokens),
    totalTokens: firstNumber(localAnalytics.totalTokens),
    cacheReadTokens: 0,
    cost: null,
    latencyMs: null,
    statusCode: 200,
    success: true,
    source: 'codex_session',
    route: '',
    requestCount: Math.max(1, Math.round(firstNumber(localAnalytics.sessionCount))),
    synthetic: true,
  }];
}

const requestRows = computed(() => (Array.isArray(records.value) ? records.value : []).map(normalizeRecord));
const localRows = computed(() => sessionRows(analytics.value));
const sourceRows = computed(() => requestRows.value.length ? requestRows.value : localRows.value);
const activeRangeLabel = computed(() => (rangeTabs.value.find(item => item.id === range.value) || rangeTabs.value[1]).label);
const sourceLabel = computed(() => requestRows.value.length ? t('USAGE_PROXY_REQUEST_RECORDS') : t('USAGE_CODEX_LOCAL_SESSIONS'));

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

function groupedStats(rows, keyName, labelName) {
  const map = new Map();
  rows.forEach(row => {
    const key = text(row[keyName]) || 'Unknown';
    const item = map.get(key) || { key: key, label: text(row[labelName]) || key, requests: 0, totalTokens: 0, inputTokens: 0, outputTokens: 0, success: 0, latency: 0, latencyRows: 0, appLabels: new Set(), providerLabels: new Set() };
    const count = row.requestCount || 1;
    item.requests += count;
    item.totalTokens += row.totalTokens;
    item.inputTokens += row.inputTokens;
    item.outputTokens += row.outputTokens;
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
  return parsed === null ? '—' : '$' + parsed.toFixed(4);
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
async function refresh() {
  loading.value = true;
  error.value = '';
  try {
    const result = await Promise.all([listAdvancedProxyRequestRecords(400), getLocalTokenUsageAnalytics()]);
    records.value = Array.isArray(result[0]) ? result[0] : [];
    analytics.value = result[1] && typeof result[1] === 'object' ? result[1] : null;
  } catch (loadError) {
    error.value = loadError && loadError.message ? loadError.message : t('USAGE_LOCAL_BRIDGE_ERROR');
  } finally {
    loading.value = false;
  }
}
function syncTheme(event) {
  isDarkMode.value = isDarkThemeMode(event && event.detail && event.detail.mode || getAppliedThemeMode());
}
onMounted(() => {
  window.addEventListener(THEME_MODE_CHANGE_EVENT, syncTheme);
  void refresh();
});
onBeforeUnmount(() => window.removeEventListener(THEME_MODE_CHANGE_EVENT, syncTheme));
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
</style>
