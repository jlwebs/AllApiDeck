<template>
  <a-drawer
    :open="open"
    :width="drawerWidth"
    placement="right"
    title="请求记录"
    :class="['advanced-proxy-records-drawer', { 'advanced-proxy-records-drawer-dark': isDarkMode }]"
    @close="handleClose"
  >
    <div class="request-records-scroll-shell">
      <div class="request-records-shell" :class="{ 'request-records-shell-dark': isDarkMode }">
        <header class="request-records-toolbar">
          <div class="request-records-toolbar-meta">
            <span class="request-records-toolbar-pill" :class="{ 'is-loading': loading }">
              <span class="request-records-toolbar-dot"></span>
              <span>{{ loading ? '同步中' : `缓存 ${records.length} 条` }}</span>
            </span>
            <span
              v-for="item in statusSummaryItems"
              :key="item.id"
              class="request-records-toolbar-pill request-records-toolbar-pill-muted"
            >
              {{ item.label }} {{ item.count }}
            </span>
          </div>

          <div class="request-records-toolbar-actions">
            <a-button
              size="small"
              class="request-records-action-button request-records-action-button-refresh"
              :loading="loading"
              @click="refreshRecords"
            >
              <ReloadOutlined />
              刷新
            </a-button>
            <a-button
              size="small"
              class="request-records-action-button request-records-action-button-clear"
              danger
              ghost
              :disabled="records.length === 0"
              @click="handleClear"
            >
              <DeleteOutlined />
              清空
            </a-button>
          </div>
        </header>

        <section class="request-records-overview">
          <article class="request-records-metric">
            <span class="request-records-metric-label">请求数</span>
            <strong class="request-records-metric-value">{{ summary.total }}</strong>
            <small>{{ requestCountSubtext }}</small>
          </article>

          <article class="request-records-metric">
            <span class="request-records-metric-label">成功率</span>
            <strong class="request-records-metric-value">{{ summary.successRate }}</strong>
            <small>{{ summary.successCount }} ok · {{ summary.errorCount }} fail</small>
          </article>

          <article class="request-records-metric">
            <span class="request-records-metric-label">平均耗时</span>
            <strong class="request-records-metric-value">{{ summary.avgDuration }}</strong>
            <small>TTFT {{ summary.avgTtft }}</small>
          </article>

          <article class="request-records-metric">
            <span class="request-records-metric-label">Token</span>
            <strong class="request-records-metric-value">{{ summary.totalTokens }}</strong>
            <small>↑ {{ summary.inputTokens }} · ↓ {{ summary.outputTokens }} · TPS {{ summary.avgTps }}</small>
          </article>
        </section>

        <div v-if="!bridgeAvailable" class="request-records-empty">
          当前环境无法读取请求记录。
        </div>

        <section v-else class="request-records-board">
          <div class="request-records-board-head">
            <div class="request-records-board-title">
              <strong>请求流水</strong>
              <span>{{ filteredRecords.length }} 条</span>
            </div>

            <div class="request-records-board-chips">
              <button
                v-for="item in appSummaryItems"
                :key="`app-${item.id}`"
                type="button"
                class="request-records-board-chip request-records-board-chip-toggle"
                :class="{ 'is-inactive': isAppChipHidden(item.id) }"
                @click="toggleAppFilter(item.id)"
              >
                {{ item.label }} {{ item.count }}
              </button>
              <span
                v-for="item in routeSummaryItems"
                :key="`route-${item.id}`"
                class="request-records-board-chip request-records-board-chip-muted"
              >
                {{ item.label }} {{ item.count }}
              </span>
            </div>
          </div>

          <div class="request-records-table-wrap">
            <a-spin :spinning="loading" class="request-records-table-spin">
              <div
                ref="tableScrollRef"
                class="request-records-table-scroll"
                :class="{
                  'is-draggable': tableDragEnabled,
                  'is-dragging': tableDragging,
                }"
                :style="{ maxHeight: `${tableScrollY}px` }"
                @pointerdown="handleTablePointerDown"
                @click.capture="handleTableClickCapture"
              >
                <div
                  class="request-records-table-stage"
                  :style="{ width: `${tableLayoutWidth}px`, transform: `translateX(-${tableScrollLeft}px)` }"
                >
                  <table
                    ref="tableElementRef"
                    class="request-records-table-native"
                    :style="{ width: `${tableLayoutWidth}px` }"
                  >
                    <colgroup>
                      <col
                        v-for="column in columns"
                        :key="`col-${column.key}`"
                        :style="{ width: resolveColumnWidth(column.width) }"
                      />
                    </colgroup>
                    <thead>
                      <tr>
                        <th
                          v-for="column in columns"
                          :key="`head-${column.key}`"
                          class="request-records-table-head"
                          :class="{
                            'is-center': column.align === 'center',
                            'is-actions': column.key === 'actions',
                          }"
                        >
                          {{ column.title }}
                        </th>
                      </tr>
                    </thead>
                    <tbody v-if="pagedRecords.length > 0">
                      <tr v-for="record in pagedRecords" :key="record.id || record.recordedAt">
                        <td
                          v-for="column in columns"
                          :key="`${record.id || record.recordedAt}-${column.key}`"
                          class="request-records-table-cell"
                          :class="{
                            'is-center': column.align === 'center',
                            'is-actions': column.key === 'actions',
                          }"
                        >
                          <template v-if="column.key === 'time'">
                            <div class="request-records-time">
                              <strong>{{ formatTime(record.recordedAt) }}</strong>
                              <small>{{ formatDate(record.recordedAt) }}</small>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'identity'">
                            <div class="request-records-identity">
                              <div class="request-records-identity-main">
                                <span class="request-records-app-pill">{{ formatAppName(record.appType) }}</span>
                                <strong>{{ record.providerName || '-' }}</strong>
                              </div>
                              <small class="request-records-mono">{{ record.model || '未记录模型' }}</small>
                              <small class="request-records-mono">{{ record.providerKeyPreview || '未记录 key' }}</small>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'link'">
                            <div class="request-records-route">
                              <div class="request-records-route-line">
                                <span class="request-records-route-key">入口</span>
                                <span class="request-records-route-path">{{ summarizeInboundEndpoint(record.inboundEndpoint) }}</span>
                              </div>
                              <div class="request-records-route-line">
                                <span class="request-records-route-key is-meta">协议</span>
                                <span class="request-records-route-pill">{{ summarizeOutboundRoute(record.outboundRoute) }}</span>
                              </div>
                              <div class="request-records-route-line">
                                <span class="request-records-route-key is-out">出口</span>
                                <a-tooltip :title="record.upstreamUrl || record.upstreamEndpoint || '-'">
                                  <span class="request-records-route-path request-records-route-path-out">
                                    {{ summarizeUpstreamTarget(record.upstreamUrl || record.upstreamEndpoint, record.upstreamEndpoint) }}
                                  </span>
                                </a-tooltip>
                              </div>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'route'">
                            <div class="request-records-routing">
                              <div
                                v-for="(step, index) in resolveRouteTraceSteps(record)"
                                :key="`${record.id || record.recordedAt || 'route'}-${index}-${step.route}-${step.status}`"
                                class="request-records-routing-line"
                                :class="resolveRouteTraceLineClass(record, step, index)"
                              >
                                <span class="request-records-routing-icon">{{ resolveRouteTraceIcon(record, step, index) }}</span>
                                <span class="request-records-routing-label">{{ formatRouteTraceLabel(step.route) }}</span>
                                <span v-if="resolveRouteTraceSourceLabel(step.source)" class="request-records-routing-source">
                                  {{ resolveRouteTraceSourceLabel(step.source) }}
                                </span>
                              </div>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'metrics'">
                            <div class="request-records-metrics">
                              <strong>{{ formatDuration(record.durationMs) }}</strong>
                              <small>TTFT {{ formatDuration(record.ttftMs) }} · Gen {{ formatDuration(record.latencyMs) }} · TPS {{ formatTps(record.tps) }}</small>
                              <small>↑ {{ formatTokenValue(record.inputTokens) }} · ↓ {{ formatTokenValue(record.outputTokens) }}</small>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'status'">
                            <div class="request-records-status">
                              <a-tag :color="resolveStatusColor(record.statusCode)">
                                {{ record.statusCode || '-' }}
                              </a-tag>
                              <span
                                class="request-records-source-pill"
                                :class="`is-${resolveSourceTone(record.source)}`"
                              >
                                {{ resolveSourceLabel(record.source) }}
                              </span>
                            </div>
                          </template>

                          <template v-else-if="column.key === 'detail'">
                            <div class="request-records-detail-cell">
                              <a-tooltip :title="resolveDetailText(record)">
                                <div class="request-records-detail-text">
                                  {{ summarizeDetail(record) }}
                                </div>
                              </a-tooltip>
                              <a-button type="text" size="small" class="request-records-detail-button" @click="openRecordDetail(record)">
                                详情
                              </a-button>
                            </div>
                          </template>
                        </td>
                      </tr>
                    </tbody>
                    <tbody v-else>
                      <tr>
                        <td :colspan="columns.length" class="request-records-empty-cell">
                          暂无请求记录
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </a-spin>
            <div
              class="request-records-table-hscroll"
              :class="{ 'is-active': showTableHorizontalScroll }"
            >
              <input
                ref="tableHorizontalScrollRef"
                class="request-records-table-hscroll-range"
                type="range"
                min="0"
                :max="Math.max(tableHorizontalMaxScroll, 1)"
                :value="tableScrollLeft"
                :disabled="tableHorizontalMaxScroll <= 0"
                @input="handleTableHorizontalRangeInput"
              />
            </div>
            <div v-if="filteredRecords.length > REQUEST_RECORD_PAGE_SIZE" class="request-records-pagination">
              <a-pagination
                size="small"
                simple
                :current="currentPage"
                :page-size="REQUEST_RECORD_PAGE_SIZE"
                :total="filteredRecords.length"
                @change="handlePageChange"
              />
            </div>
          </div>
        </section>
      </div>
    </div>

    <a-drawer
      :open="detailOpen"
      :width="detailDrawerWidth"
      placement="right"
      title="请求详情"
      :class="['advanced-proxy-records-detail-drawer', { 'advanced-proxy-records-detail-drawer-dark': isDarkMode }]"
      @close="detailOpen = false"
    >
      <div v-if="selectedRecord" class="request-record-detail-shell" :class="{ 'request-record-detail-shell-dark': isDarkMode }">
        <header class="request-record-detail-hero">
          <div class="request-record-detail-hero-main">
            <span class="request-records-app-pill">{{ formatAppName(selectedRecord.appType) }}</span>
            <strong>{{ selectedRecord.providerName || '-' }}</strong>
            <small>{{ selectedRecord.model || '未记录模型' }}</small>
          </div>

          <div class="request-record-detail-hero-tags">
            <a-tag :color="resolveStatusColor(selectedRecord.statusCode)">
              {{ selectedRecord.statusCode || '-' }}
            </a-tag>
            <span
              class="request-records-source-pill"
              :class="`is-${resolveSourceTone(selectedRecord.source)}`"
            >
              {{ resolveSourceLabel(selectedRecord.source) }}
            </span>
          </div>
        </header>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">标识</div>
          <div class="request-record-detail-grid">
            <div class="request-record-detail-item">
              <span>时间</span>
              <strong>{{ formatDateTime(selectedRecord.recordedAt) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>密钥</span>
              <strong class="request-records-mono">{{ selectedRecord.providerKeyPreview || '-' }}</strong>
            </div>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">链路</div>
          <div class="request-record-detail-grid">
            <div class="request-record-detail-item">
              <span>入口</span>
              <strong>{{ selectedRecord.inboundEndpoint || '-' }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>协议</span>
              <strong>{{ selectedRecord.outboundRoute || '-' }}</strong>
            </div>
            <div class="request-record-detail-item request-record-detail-item-full">
              <span>上游 URL</span>
              <strong>{{ selectedRecord.upstreamUrl || selectedRecord.upstreamEndpoint || '-' }}</strong>
            </div>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">性能</div>
          <div class="request-record-detail-grid">
            <div class="request-record-detail-item">
              <span>耗时</span>
              <strong>{{ formatDuration(selectedRecord.durationMs) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>TTFT</span>
              <strong>{{ formatDuration(selectedRecord.ttftMs) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>Latency</span>
              <strong>{{ formatDuration(selectedRecord.latencyMs) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>Token</span>
              <strong>↑ {{ formatTokenValue(selectedRecord.inputTokens) }} · ↓ {{ formatTokenValue(selectedRecord.outputTokens) }}</strong>
            </div>
            <div class="request-record-detail-item">
              <span>TPS</span>
              <strong>{{ formatTps(selectedRecord.tps) }}</strong>
            </div>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title">返回</div>
          <div class="request-record-detail-item request-record-detail-item-full">
            <span>详情</span>
            <pre>{{ resolveDetailText(selectedRecord) }}</pre>
          </div>
        </section>

        <section class="request-record-detail-section">
          <div class="request-record-detail-section-title request-record-detail-section-title-row">
            <span>请求</span>
            <div class="request-record-detail-section-actions">
              <a-button
                size="small"
                class="request-record-debug-button"
                :loading="requestDebugTesting"
                @click="handleRequestDebugTest"
              >
                测试
              </a-button>
              <a-tooltip v-if="requestDebugState !== 'idle'" :title="requestDebugResponse || '-'">
                <span class="request-record-debug-result" :class="`is-${requestDebugState}`">
                  <LoadingOutlined v-if="requestDebugState === 'loading'" />
                  <CheckCircleFilled v-else-if="requestDebugState === 'success'" />
                  <CloseCircleFilled v-else />
                </span>
              </a-tooltip>
            </div>
          </div>
          <a-textarea
            v-model:value="requestDebugBody"
            class="request-record-debug-textarea"
            :auto-size="{ minRows: 12, maxRows: 22 }"
          />
        </section>
      </div>
    </a-drawer>
  </a-drawer>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { message, Modal } from 'ant-design-vue';
import { CheckCircleFilled, CloseCircleFilled, DeleteOutlined, LoadingOutlined, ReloadOutlined } from '@ant-design/icons-vue';
import {
  clearAdvancedProxyRequestRecords,
  getAdvancedProxyConfig,
  getAdvancedProxyEffectiveProviders,
  isAdvancedProxyRequestRecordBridgeAvailable,
  listAdvancedProxyRequestRecords,
} from '../utils/advancedProxyBridge.js';

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  isDarkMode: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(['update:open']);

const bridgeAvailable = isAdvancedProxyRequestRecordBridgeAvailable();
const loading = ref(false);
const records = ref([]);
const hiddenAppIds = ref([]);
const currentPage = ref(1);
const detailOpen = ref(false);
const selectedRecord = ref(null);
const requestDebugBody = ref('');
const requestDebugState = ref('idle');
const requestDebugResponse = ref('');
const tableScrollRef = ref(null);
const tableElementRef = ref(null);
const tableHorizontalScrollRef = ref(null);
const tableContentWidth = ref(0);
const tableViewportWidth = ref(0);
const tableScrollLeft = ref(0);
const tableVerticalMaxScroll = ref(0);
const tableDragging = ref(false);
const viewportWidth = ref(typeof window === 'undefined' ? 900 : window.innerWidth);
const viewportHeight = ref(typeof window === 'undefined' ? 600 : window.innerHeight);
let tableMetricsFrame = 0;
let tableResizeObserver = null;
let tableDragSession = null;
let tableSuppressClickUntil = 0;
const REQUEST_RECORD_PAGE_SIZE = 50;

const isCompactWindow = computed(() => viewportWidth.value <= 860);
const drawerWidth = computed(() => Math.min(Math.max(viewportWidth.value - 18, 380), 860));
const detailDrawerWidth = computed(() => Math.min(Math.max(Math.floor(viewportWidth.value * 0.42), 320), 420));
const tableScrollY = computed(() => {
  const reservedHeight = isCompactWindow.value ? 360 : 332;
  return Math.max(280, Math.min(620, viewportHeight.value - reservedHeight));
});
const tableLayoutWidth = computed(() => columns.value.reduce((sum, column) => {
  const width = Number(column?.width || 0);
  return sum + (Number.isFinite(width) && width > 0 ? width : 0);
}, 0));
const tableViewportFallbackWidth = computed(() => {
  const numeric = Number(drawerWidth.value || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) return 0;
  return Math.max(260, numeric - 32);
});
const showTableHorizontalScroll = computed(() => tableLayoutWidth.value - tableViewportWidth.value > 2);
const tableHorizontalMaxScroll = computed(() => Math.max(0, tableContentWidth.value - tableViewportWidth.value));
const tableDragEnabled = computed(() => tableHorizontalMaxScroll.value > 0 || tableVerticalMaxScroll.value > 0);
const requestDebugTesting = computed(() => requestDebugState.value === 'loading');

const columns = computed(() => {
  const compact = isCompactWindow.value;
  return [
    { title: '时间', dataIndex: 'recordedAt', key: 'time', width: compact ? 82 : 90 },
    { title: 'Provider', dataIndex: 'providerName', key: 'identity', width: compact ? 168 : 182 },
    { title: '链路', dataIndex: 'outboundRoute', key: 'link', width: compact ? 220 : 250 },
    { title: '路由', dataIndex: 'routeTrace', key: 'route', width: compact ? 138 : 152 },
    { title: '性能', dataIndex: 'durationMs', key: 'metrics', width: compact ? 146 : 158 },
    { title: '状态', dataIndex: 'statusCode', key: 'status', width: compact ? 88 : 96 },
    { title: '摘要', dataIndex: 'errorDetail', key: 'detail', width: compact ? 276 : 346, ellipsis: true },
  ];
});

const filteredRecords = computed(() => {
  const list = Array.isArray(records.value) ? records.value : [];
  if (hiddenAppIds.value.length === 0) {
    return list;
  }
  const hiddenSet = new Set(hiddenAppIds.value);
  return list.filter((record) => !hiddenSet.has(String(record?.appType || '').trim().toLowerCase()));
});

const pagedRecords = computed(() => {
  const start = (currentPage.value - 1) * REQUEST_RECORD_PAGE_SIZE;
  return filteredRecords.value.slice(start, start + REQUEST_RECORD_PAGE_SIZE);
});

const requestCountSubtext = computed(() => {
  if (hiddenAppIds.value.length === 0) {
    return `最近 ${records.value.length} 条`;
  }
  return `显示 ${filteredRecords.value.length} / ${records.value.length} 条`;
});

const summary = computed(() => {
  const list = filteredRecords.value;
  const total = list.length;
  const successCount = list.filter((record) => {
    const code = Number(record?.statusCode || 0);
    return code >= 200 && code < 300;
  }).length;
  const errorCount = Math.max(0, total - successCount);
  const durationValues = list
    .map(record => Number(record?.durationMs || 0))
    .filter(value => Number.isFinite(value) && value > 0);
  const ttftValues = list
    .map(record => Number(record?.ttftMs || 0))
    .filter(value => Number.isFinite(value) && value > 0);
  const inputTokens = list
    .map(record => Number(record?.inputTokens || 0))
    .filter(value => Number.isFinite(value) && value > 0)
    .reduce((sum, value) => sum + value, 0);
  const outputTokens = list
    .map(record => Number(record?.outputTokens || 0))
    .filter(value => Number.isFinite(value) && value > 0)
    .reduce((sum, value) => sum + value, 0);
  const tpsValues = list
    .map(record => Number(record?.tps))
    .filter(value => Number.isFinite(value) && value > 0);

  return {
    total,
    successCount,
    errorCount,
    successRate: total > 0 ? `${Math.round((successCount / total) * 100)}%` : '-',
    avgDuration: durationValues.length > 0
      ? formatDuration(durationValues.reduce((sum, value) => sum + value, 0) / durationValues.length)
      : '-',
    avgTtft: ttftValues.length > 0
      ? formatDuration(ttftValues.reduce((sum, value) => sum + value, 0) / ttftValues.length)
      : '-',
    avgTps: tpsValues.length > 0
      ? formatTps(tpsValues.reduce((sum, value) => sum + value, 0) / tpsValues.length)
      : '-',
    inputTokens: formatCompactNumber(inputTokens),
    outputTokens: formatCompactNumber(outputTokens),
    totalTokens: formatCompactNumber(inputTokens + outputTokens),
  };
});

const statusSummaryItems = computed(() => {
  const buckets = [
    {
      id: '2xx',
      label: '2xx',
      count: filteredRecords.value.filter((record) => {
        const code = Number(record?.statusCode || 0);
        return code >= 200 && code < 300;
      }).length,
    },
    {
      id: '4xx',
      label: '4xx',
      count: filteredRecords.value.filter((record) => {
        const code = Number(record?.statusCode || 0);
        return code >= 400 && code < 500;
      }).length,
    },
    {
      id: '5xx',
      label: '5xx',
      count: filteredRecords.value.filter((record) => {
        const code = Number(record?.statusCode || 0);
        return code >= 500;
      }).length,
    },
  ];
  return buckets.filter(item => item.count > 0);
});

const appSummaryItems = computed(() => {
  const counts = new Map();
  records.value.forEach((record) => {
    const appId = String(record?.appType || '').trim().toLowerCase();
    if (!appId) return;
    counts.set(appId, (counts.get(appId) || 0) + 1);
  });
  return [...counts.entries()]
    .sort((left, right) => right[1] - left[1])
    .slice(0, 4)
    .map(([id, count]) => ({
      id,
      label: formatAppName(id),
      count,
    }));
});

const routeSummaryItems = computed(() => {
  const counts = new Map();
  filteredRecords.value.forEach((record) => {
    const route = summarizeOutboundRoute(record?.outboundRoute);
    if (!route || route === '-') return;
    counts.set(route, (counts.get(route) || 0) + 1);
  });
  return [...counts.entries()]
    .sort((left, right) => right[1] - left[1])
    .slice(0, 3)
    .map(([id, count]) => ({
      id,
      label: id,
      count,
    }));
});

function syncViewport() {
  viewportWidth.value = typeof window === 'undefined' ? 900 : window.innerWidth;
  viewportHeight.value = typeof window === 'undefined' ? 600 : window.innerHeight;
  queueTableMetricsSync();
}

function detachTableResizeObserver() {
  if (tableResizeObserver) {
    tableResizeObserver.disconnect();
    tableResizeObserver = null;
  }
}

function attachTableResizeObserver() {
  detachTableResizeObserver();
  if (typeof ResizeObserver === 'undefined') return;
  const scrollElement = tableScrollRef.value;
  const tableElement = tableElementRef.value;
  if (!scrollElement || !tableElement) return;
  tableResizeObserver = new ResizeObserver(() => {
    queueTableMetricsSync();
  });
  tableResizeObserver.observe(scrollElement);
  tableResizeObserver.observe(tableElement);
}

function syncTableMetrics() {
  tableMetricsFrame = 0;
  const scrollElement = tableScrollRef.value;
  const tableElement = tableElementRef.value;
  if (!scrollElement || !tableElement) {
    tableContentWidth.value = 0;
    tableViewportWidth.value = tableViewportFallbackWidth.value;
    tableVerticalMaxScroll.value = 0;
    tableScrollLeft.value = 0;
    return;
  }
  const measuredViewportWidth = Math.max(scrollElement.clientWidth, 0);
  const fallbackViewportWidth = tableViewportFallbackWidth.value;
  tableContentWidth.value = Math.max(tableLayoutWidth.value, tableElement.offsetWidth, 0);
  tableVerticalMaxScroll.value = Math.max(0, scrollElement.scrollHeight - scrollElement.clientHeight);
  if (fallbackViewportWidth > 0) {
    if (measuredViewportWidth > 0) {
      tableViewportWidth.value = Math.min(measuredViewportWidth, fallbackViewportWidth);
    } else {
      tableViewportWidth.value = fallbackViewportWidth;
    }
  } else {
    tableViewportWidth.value = measuredViewportWidth;
  }
  const maxScrollLeft = Math.max(0, tableContentWidth.value - tableViewportWidth.value);
  if (tableScrollLeft.value > maxScrollLeft) {
    tableScrollLeft.value = maxScrollLeft;
  }
}

function queueTableMetricsSync() {
  if (tableMetricsFrame) {
    window.cancelAnimationFrame(tableMetricsFrame);
  }
  tableMetricsFrame = window.requestAnimationFrame(() => {
    syncTableMetrics();
  });
}

function setTableScrollLeft(nextScrollLeft) {
  const maxScrollLeft = Math.max(0, tableContentWidth.value - tableViewportWidth.value);
  const clamped = Math.max(0, Math.min(maxScrollLeft, Number(nextScrollLeft) || 0));
  tableScrollLeft.value = clamped;
}

function handleTableHorizontalRangeInput(event) {
  const nextValue = Number(event?.target?.value || 0);
  setTableScrollLeft(nextValue);
}

function isTableInteractiveTarget(target) {
  if (!(target instanceof Element)) return false;
  return Boolean(target.closest([
    'button',
    'a',
    'input',
    'textarea',
    'select',
    'label',
    '[role="button"]',
    '.ant-btn',
    '.ant-pagination',
    '.request-records-detail-button',
  ].join(',')));
}

function detachTableDragListeners() {
  window.removeEventListener('pointermove', handleTablePointerMove);
  window.removeEventListener('pointerup', handleTablePointerEnd);
  window.removeEventListener('pointercancel', handleTablePointerEnd);
}

function clearTableDragSession() {
  const session = tableDragSession;
  if (session?.scrollElement?.releasePointerCapture && session.pointerId != null) {
    try {
      session.scrollElement.releasePointerCapture(session.pointerId);
    } catch {}
  }
  detachTableDragListeners();
  tableDragging.value = false;
  tableDragSession = null;
  if (typeof document !== 'undefined') {
    document.body.style.removeProperty('user-select');
  }
}

function handleTablePointerDown(event) {
  const scrollElement = tableScrollRef.value;
  if (!scrollElement || !tableDragEnabled.value) return;
  if (event.pointerType === 'mouse' && event.button !== 0) return;
  if (event.isPrimary === false) return;
  if (isTableInteractiveTarget(event.target)) return;

  clearTableDragSession();
  tableDragSession = {
    pointerId: event.pointerId,
    startClientX: Number(event.clientX || 0),
    startClientY: Number(event.clientY || 0),
    startScrollLeft: tableScrollLeft.value,
    startScrollTop: scrollElement.scrollTop,
    scrollElement,
    dragging: false,
  };

  if (scrollElement.setPointerCapture) {
    try {
      scrollElement.setPointerCapture(event.pointerId);
    } catch {}
  }

  window.addEventListener('pointermove', handleTablePointerMove, { passive: false });
  window.addEventListener('pointerup', handleTablePointerEnd, { passive: false });
  window.addEventListener('pointercancel', handleTablePointerEnd, { passive: false });
}

function handleTablePointerMove(event) {
  const session = tableDragSession;
  if (!session || session.pointerId !== event.pointerId) return;
  const deltaX = Number(event.clientX || 0) - session.startClientX;
  const deltaY = Number(event.clientY || 0) - session.startClientY;
  if (!session.dragging && Math.abs(deltaX) < 3 && Math.abs(deltaY) < 3) {
    return;
  }

  if (!session.dragging) {
    session.dragging = true;
    tableDragging.value = true;
    tableSuppressClickUntil = Date.now() + 250;
    if (typeof document !== 'undefined') {
      document.body.style.userSelect = 'none';
    }
  }

  event.preventDefault();
  setTableScrollLeft(session.startScrollLeft - deltaX);
  const maxScrollTop = Math.max(0, session.scrollElement.scrollHeight - session.scrollElement.clientHeight);
  session.scrollElement.scrollTop = Math.max(0, Math.min(maxScrollTop, session.startScrollTop - deltaY));
}

function handleTablePointerEnd(event) {
  const session = tableDragSession;
  if (!session || session.pointerId !== event.pointerId) return;
  clearTableDragSession();
}

function handleTableClickCapture(event) {
  if (Date.now() > tableSuppressClickUntil) return;
  tableSuppressClickUntil = 0;
  event.preventDefault();
  event.stopPropagation();
}

function normalizeText(value) {
  return String(value || '').trim();
}

function resolveColumnWidth(value) {
  const numeric = Number(value || 0);
  if (!Number.isFinite(numeric) || numeric <= 0) return 'auto';
  return `${numeric}px`;
}

function formatDateTime(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  const date = new Date(text);
  if (Number.isNaN(date.getTime())) return text;
  return date.toLocaleString('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function formatTime(value) {
  const text = formatDateTime(value);
  if (text === '-') return text;
  const parts = text.split(' ');
  return parts[1] || text;
}

function formatDate(value) {
  const text = formatDateTime(value);
  if (text === '-') return text;
  const parts = text.split(' ');
  return parts[0] || text;
}

function formatDuration(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '-';
  if (numeric < 1000) return `${Math.round(numeric)}ms`;
  return `${(numeric / 1000).toFixed(numeric >= 10000 ? 1 : 2)}s`;
}

function formatCompactNumber(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '0';
  if (numeric >= 1000000000) return `${(numeric / 1000000000).toFixed(2)}B`;
  if (numeric >= 1000000) return `${(numeric / 1000000).toFixed(2)}M`;
  if (numeric >= 100000) return `${Math.round(numeric / 1000)}K`;
  if (numeric >= 10000) return `${(numeric / 1000).toFixed(1)}K`;
  if (numeric >= 1000) return `${Math.round(numeric / 100) / 10}K`;
  return String(Math.round(numeric));
}

function formatTokenValue(value) {
  if (value == null || value === '') return '-';
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric < 0) return '-';
  return formatCompactNumber(numeric);
}

function formatTps(value) {
  if (value == null || value === '') return '-';
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '-';
  if (numeric >= 100) return numeric.toFixed(0);
  if (numeric >= 10) return numeric.toFixed(1);
  return numeric.toFixed(2);
}

function formatAppName(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'claude':
      return 'Claude';
    case 'codex':
      return 'Codex';
    case 'opencode':
      return 'OpenCode';
    case 'openclaw':
      return 'OpenClaw';
    default:
      return normalizeText(value) || '-';
  }
}

function resolveSourceLabel(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'original':
      return '原始';
    case 'fallback':
      return '回退';
    case 'preference':
      return '偏好';
    case 'direct':
      return '直连';
    default:
      return normalizeText(value) || '-';
  }
}

function resolveSourceTone(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'fallback':
      return 'fallback';
    case 'preference':
      return 'preference';
    case 'direct':
      return 'direct';
    default:
      return 'default';
  }
}

function resolveStatusColor(statusCode) {
  const code = Number(statusCode || 0);
  if (code >= 200 && code < 300) return 'green';
  if (code >= 400 && code < 500) return 'orange';
  return 'red';
}

function handlePageChange(page) {
  const numeric = Number(page || 1);
  currentPage.value = Number.isFinite(numeric) && numeric > 0 ? numeric : 1;
}

function isAppChipHidden(appId) {
  return hiddenAppIds.value.includes(String(appId || '').trim().toLowerCase());
}

function toggleAppFilter(appId) {
  const normalized = String(appId || '').trim().toLowerCase();
  if (!normalized) return;
  if (isAppChipHidden(normalized)) {
    hiddenAppIds.value = hiddenAppIds.value.filter(id => id !== normalized);
    return;
  }
  hiddenAppIds.value = [...hiddenAppIds.value, normalized];
}

function resolveDetailText(record) {
  const text = normalizeText(record?.errorDetail);
  return text || '请求成功';
}

function normalizeComparableUrl(value) {
  return String(value || '').trim().replace(/\/+$/, '').toLowerCase();
}

function formatRequestDebugResponse(value) {
  const text = String(value || '').trim();
  if (!text) return '(empty)';
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

function resetRequestDebugState() {
  requestDebugState.value = 'idle';
  requestDebugResponse.value = '';
}

function collectRequestDebugProviders(config, appId) {
  const candidates = [];
  const seen = new Set();
  const pushProvider = (provider) => {
    const id = String(provider?.id || '').trim();
    const rowKey = String(provider?.rowKey || '').trim();
    const baseUrl = String(provider?.baseUrl || '').trim();
    const dedupeKey = `${id}::${rowKey}::${baseUrl}`;
    if (!baseUrl || seen.has(dedupeKey)) return;
    seen.add(dedupeKey);
    candidates.push(provider);
  };

  getAdvancedProxyEffectiveProviders(config, appId, { enabledOnly: false }).forEach(pushProvider);
  Object.values(config?.queues || {}).forEach((queueSection) => {
    (Array.isArray(queueSection?.providers) ? queueSection.providers : []).forEach(pushProvider);
  });
  (Array.isArray(config?.claude?.providers) ? config.claude.providers : []).forEach(pushProvider);

  return candidates;
}

function resolveRequestDebugProvider(record, config) {
  const appId = String(record?.appType || '').trim().toLowerCase() || 'claude';
  const candidates = collectRequestDebugProviders(config, appId);
  const providerId = String(record?.providerId || '').trim();
  const providerRowKey = String(record?.providerRowKey || '').trim();
  const providerName = String(record?.providerName || '').trim();
  const providerModel = String(record?.model || '').trim().toLowerCase();
  const normalizedTargetURL = normalizeComparableUrl(record?.upstreamUrl);

  return candidates.find((provider) => {
    if (!provider) return false;
    if (providerId && String(provider?.id || '').trim() === providerId) return true;
    if (providerRowKey && String(provider?.rowKey || '').trim() === providerRowKey) return true;

    const normalizedBaseURL = normalizeComparableUrl(provider?.baseUrl);
    if (normalizedBaseURL && normalizedTargetURL.startsWith(normalizedBaseURL)) {
      if (!providerModel) return true;
      return String(provider?.model || '').trim().toLowerCase() === providerModel;
    }

    if (providerName && String(provider?.name || '').trim() === providerName) {
      if (!providerModel) return true;
      return String(provider?.model || '').trim().toLowerCase() === providerModel;
    }

    return false;
  }) || null;
}

function buildRequestDebugHeaders(record, provider, payload) {
  const stream = payload?.stream === true;
  const route = String(record?.outboundRoute || '').trim().toLowerCase();
  const apiFormat = String(provider?.apiFormat || '').trim().toLowerCase();
  const headers = {
    'Content-Type': 'application/json',
    'Accept': stream ? 'text/event-stream' : 'application/json',
  };

  if (String(record?.appType || '').trim().toLowerCase() === 'claude' && (route === 'messages' || apiFormat === 'anthropic')) {
    headers['x-api-key'] = String(provider?.apiKey || '').trim();
    headers['anthropic-version'] = '2023-06-01';
    return headers;
  }

  headers.Authorization = `Bearer ${String(provider?.apiKey || '').trim()}`;
  return headers;
}

function buildRequestDebugCommand(targetURL, headers, payload) {
  const normalizedURL = String(targetURL || '').trim();
  const normalizedHeaders = headers && typeof headers === 'object' ? headers : {};
  const normalizedPayload = payload && typeof payload === 'object' ? payload : {};
  const headerText = JSON.stringify(normalizedHeaders, null, 2) || '{}';
  const payloadText = JSON.stringify(normalizedPayload, null, 2) || '{}';

  return [
    `fetch(${JSON.stringify(normalizedURL)}, {`,
    `  method: "POST",`,
    `  headers: ${headerText.replace(/\n/g, '\n  ')},`,
    `  body: JSON.stringify(${payloadText.replace(/\n/g, '\n  ')})`,
    `})`,
  ].join('\n');
}

function parseRequestDebugPayload(record) {
  const text = String(record?.requestBody || '').trim();
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return {};
  }
}

async function syncRequestDebugEditor(record) {
  resetRequestDebugState();
  if (!record) {
    requestDebugBody.value = '';
    return;
  }

  const targetURL = String(record?.upstreamUrl || '').trim();
  const payload = parseRequestDebugPayload(record);

  try {
    const config = await getAdvancedProxyConfig();
    const provider = resolveRequestDebugProvider(record, config);
    const headers = buildRequestDebugHeaders(record, provider || {}, payload);
    requestDebugBody.value = buildRequestDebugCommand(targetURL, headers, payload);
  } catch {
    requestDebugBody.value = buildRequestDebugCommand(targetURL, {
      'Content-Type': 'application/json',
      'Accept': payload?.stream === true ? 'text/event-stream' : 'application/json',
    }, payload);
  }
}

async function executeRequestDebugCommand(commandText) {
  const trimmed = String(commandText || '').trim().replace(/;+\s*$/, '');
  if (!trimmed) {
    throw new Error('empty fetch command');
  }
  const runner = new Function('fetch', `"use strict"; return (async () => { return ${trimmed}; })();`);
  return runner(fetch);
}

function isRequestDebugResponseLike(value) {
  return Boolean(value) && typeof value === 'object' && typeof value.text === 'function';
}

async function normalizeRequestDebugExecutionResult(result) {
  if (isRequestDebugResponseLike(result)) {
    return {
      ok: typeof result.ok === 'boolean' ? result.ok : true,
      text: await result.text(),
    };
  }
  if (typeof result === 'string') {
    return { ok: true, text: result };
  }
  if (result == null) {
    return { ok: true, text: '(empty)' };
  }
  if (typeof result === 'object') {
    try {
      return { ok: true, text: JSON.stringify(result, null, 2) };
    } catch {
      return { ok: true, text: String(result) };
    }
  }
  return { ok: true, text: String(result) };
}

async function handleRequestDebugTest() {
  const record = selectedRecord.value;
  if (!record) return;

  requestDebugState.value = 'loading';
  requestDebugResponse.value = '';

  try {
    const result = await executeRequestDebugCommand(requestDebugBody.value);
    const normalizedResult = await normalizeRequestDebugExecutionResult(result);
    requestDebugResponse.value = formatRequestDebugResponse(normalizedResult.text);
    requestDebugState.value = normalizedResult.ok ? 'success' : 'error';
  } catch (error) {
    requestDebugState.value = 'error';
    requestDebugResponse.value = error?.message || 'request failed';
  }
}

function summarizeDetail(record) {
  const text = resolveDetailText(record);
  return text.length > 80 ? `${text.slice(0, 80)}...` : text;
}

function summarizeInboundEndpoint(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  return text
    .replace(/^https?:\/\/[^/]+/i, '')
    .replace(/^\/+/, '')
    .replace(/^advanced-proxy\//i, '');
}

function summarizeOutboundRoute(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  return text.replace(/^\/+/, '');
}

function resolveRouteTraceSteps(record) {
  const rawSteps = Array.isArray(record?.routeTrace) ? record.routeTrace : [];
  const normalized = rawSteps
    .map((step) => ({
      route: normalizeText(step?.route),
      source: normalizeText(step?.source).toLowerCase(),
      status: normalizeText(step?.status).toLowerCase(),
    }))
    .filter(step => step.route);
  if (normalized.length > 0) {
    return normalized.slice(-3);
  }
  const fallbackRoute = summarizeOutboundRoute(record?.outboundRoute);
  if (!fallbackRoute || fallbackRoute === '-') {
    return [];
  }
  return [{
    route: fallbackRoute,
    source: normalizeText(record?.source).toLowerCase(),
    status: Number(record?.statusCode || 0) >= 200 && Number(record?.statusCode || 0) < 300 ? 'success' : 'failed',
  }];
}

function formatRouteTraceLabel(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'responses':
      return 'responses';
    case 'responses_compact':
      return 'resp/compact';
    case 'chat':
      return 'chat';
    case 'messages':
      return 'messages';
    default:
      return summarizeOutboundRoute(value) || '-';
  }
}

function resolveRouteTraceSourceLabel(value) {
  switch (String(value || '').trim().toLowerCase()) {
    case 'fallback':
      return '回退';
    case 'fallback_restore':
      return '恢复';
    case 'preference':
      return '偏好';
    case 'upgrade':
      return '升级';
    case 'rectified':
      return '修正';
    default:
      return '';
  }
}

function isFinalFallbackRouteStep(record, step, index) {
  const steps = resolveRouteTraceSteps(record);
  if (index !== steps.length - 1 || step?.status !== 'success' || steps.length < 2) {
    return false;
  }
  const previousRoutes = steps.slice(0, -1).map(item => item.route);
  if (previousRoutes.some(route => route !== step.route)) {
    return true;
  }
  return ['fallback', 'fallback_restore', 'preference'].includes(String(step?.source || '').trim().toLowerCase());
}

function resolveRouteTraceLineClass(record, step, index) {
  if (isFinalFallbackRouteStep(record, step, index)) {
    return 'is-fallback-final';
  }
  if (step?.status === 'failed') {
    return 'is-failed';
  }
  return 'is-direct';
}

function resolveRouteTraceIcon(record, step, index) {
  if (isFinalFallbackRouteStep(record, step, index)) {
    return '●';
  }
  if (step?.status === 'failed') {
    return '×';
  }
  return '○';
}

function summarizeUpstreamTarget(rawUrl, rawPath) {
  const host = extractHost(rawUrl);
  const path = normalizeText(rawPath);
  if (host === '-' && !path) return '-';
  if (host === '-') return path;
  if (!path) return host;
  return `${host}${path}`;
}

function extractHost(value) {
  const text = normalizeText(value);
  if (!text) return '-';
  try {
    return new URL(text).host || text;
  } catch {
    return text.replace(/^https?:\/\//i, '').split('/')[0] || text;
  }
}

async function refreshRecords() {
  if (!bridgeAvailable) return;
  loading.value = true;
  try {
    records.value = await listAdvancedProxyRequestRecords(400);
    await nextTick();
    queueTableMetricsSync();
  } catch (error) {
    message.error(error?.message || '读取高级代理请求记录失败');
  } finally {
    loading.value = false;
  }
}

function handleClose() {
  emit('update:open', false);
}

function openRecordDetail(record) {
  selectedRecord.value = record || null;
  void syncRequestDebugEditor(record);
  detailOpen.value = true;
}

function handleClear() {
  Modal.confirm({
    title: '清空请求记录？',
    content: '仅清掉本地高级代理请求记录缓存，不影响运行配置。',
    okText: '清空',
    okButtonProps: { danger: true },
    cancelText: '取消',
    async onOk() {
      try {
        await clearAdvancedProxyRequestRecords();
        records.value = [];
        hiddenAppIds.value = [];
        currentPage.value = 1;
        detailOpen.value = false;
        selectedRecord.value = null;
        void syncRequestDebugEditor(null);
        message.success('请求记录已清空');
      } catch (error) {
        message.error(error?.message || '清空请求记录失败');
      }
    },
  });
}

watch(
  () => props.open,
  async (nextOpen) => {
    if (nextOpen) {
      syncViewport();
      await refreshRecords();
      await nextTick();
      attachTableResizeObserver();
      queueTableMetricsSync();
      return;
    }
    detailOpen.value = false;
    selectedRecord.value = null;
    void syncRequestDebugEditor(null);
    clearTableDragSession();
    detachTableResizeObserver();
  },
  { immediate: true },
);

watch(selectedRecord, (record) => {
  void syncRequestDebugEditor(record);
});

watch(appSummaryItems, (items) => {
  const validIds = new Set(items.map(item => item.id));
  hiddenAppIds.value = hiddenAppIds.value.filter(id => validIds.has(id));
}, { immediate: true });

watch(filteredRecords, (list) => {
  const totalPages = Math.max(1, Math.ceil(list.length / REQUEST_RECORD_PAGE_SIZE));
  if (currentPage.value > totalPages) {
    currentPage.value = totalPages;
  }
  if (currentPage.value < 1) {
    currentPage.value = 1;
  }
}, { immediate: true });

watch(
  () => [
    props.open,
    currentPage.value,
    filteredRecords.value.length,
    columns.value.map(column => `${column.key}:${column.width}`).join('|'),
    tableScrollY.value,
  ],
  async ([nextOpen]) => {
    if (!nextOpen) return;
    await nextTick();
    attachTableResizeObserver();
    queueTableMetricsSync();
  },
  { flush: 'post' },
);

onMounted(() => {
  syncViewport();
  nextTick(() => {
    attachTableResizeObserver();
    queueTableMetricsSync();
  });
  window.addEventListener('resize', syncViewport);
});

onBeforeUnmount(() => {
  clearTableDragSession();
  detachTableResizeObserver();
  if (tableMetricsFrame) {
    window.cancelAnimationFrame(tableMetricsFrame);
    tableMetricsFrame = 0;
  }
  window.removeEventListener('resize', syncViewport);
});
</script>

<style scoped>
.advanced-proxy-records-drawer :deep(.ant-drawer-header),
.advanced-proxy-records-detail-drawer :deep(.ant-drawer-header) {
  padding: 12px 14px 10px;
  border-bottom: 1px solid rgba(102, 122, 108, 0.12);
  background:
    linear-gradient(180deg, rgba(252, 251, 248, 0.98), rgba(246, 248, 244, 0.94)),
    rgba(255, 255, 255, 0.96);
}

.advanced-proxy-records-drawer :deep(.ant-drawer-title),
.advanced-proxy-records-detail-drawer :deep(.ant-drawer-title) {
  color: #223128;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.01em;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-content-wrapper),
.advanced-proxy-records-drawer :deep(.ant-drawer-content),
.advanced-proxy-records-drawer :deep(.ant-drawer-wrapper-body),
.advanced-proxy-records-drawer :deep(.ant-drawer-body) {
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-content-wrapper::-webkit-scrollbar),
.advanced-proxy-records-drawer :deep(.ant-drawer-content::-webkit-scrollbar),
.advanced-proxy-records-drawer :deep(.ant-drawer-wrapper-body::-webkit-scrollbar),
.advanced-proxy-records-drawer :deep(.ant-drawer-body::-webkit-scrollbar) {
  width: 0;
  height: 0;
  display: none;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-wrapper-body) {
  overflow: hidden;
}

.advanced-proxy-records-drawer :deep(.ant-drawer-body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px 14px;
  overflow: hidden;
}

.advanced-proxy-records-detail-drawer :deep(.ant-drawer-body) {
  padding: 12px 14px;
  overflow-x: hidden;
  overflow-y: auto;
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.advanced-proxy-records-detail-drawer :deep(.ant-drawer-body::-webkit-scrollbar) {
  width: 0;
  height: 0;
  display: none;
}

.advanced-proxy-records-drawer-dark :deep(.ant-drawer-header),
.advanced-proxy-records-detail-drawer-dark :deep(.ant-drawer-header) {
  border-bottom-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(22, 30, 26, 0.98), rgba(17, 24, 20, 0.96)),
    rgba(17, 24, 20, 0.96);
}

.advanced-proxy-records-drawer-dark :deep(.ant-drawer-title),
.advanced-proxy-records-detail-drawer-dark :deep(.ant-drawer-title) {
  color: #edf6ee;
}

.request-records-scroll-shell {
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  width: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.request-records-scroll-shell::-webkit-scrollbar {
  width: 0;
  height: 0;
  display: none;
}

.request-records-shell {
  display: grid;
  align-content: start;
  gap: 10px;
  min-height: max-content;
  min-width: 0;
  width: 100%;
  padding-bottom: 6px;
}

.request-records-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.request-records-toolbar-meta,
.request-records-toolbar-actions,
.request-records-board-chips,
.request-record-detail-hero-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.request-records-toolbar-meta {
  min-width: 0;
  flex: 1 1 auto;
}

.request-records-toolbar-pill,
.request-records-board-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(110, 133, 118, 0.12);
  background: rgba(255, 255, 255, 0.78);
  color: #536257;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}

.request-records-board-chip-toggle {
  appearance: none;
  -webkit-appearance: none;
  font: inherit;
  cursor: pointer;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease,
    opacity 0.18s ease;
}

.request-records-board-chip-toggle.is-inactive {
  background: rgba(246, 248, 244, 0.92);
  border-color: rgba(110, 133, 118, 0.08);
  color: #8a988d;
  opacity: 0.72;
}

.request-records-toolbar-pill-muted,
.request-records-board-chip-muted {
  background: rgba(246, 248, 244, 0.92);
  color: #69786d;
}

.request-records-toolbar-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #4e7a45;
  box-shadow: 0 0 0 4px rgba(88, 126, 79, 0.12);
}

.request-records-toolbar-pill.is-loading .request-records-toolbar-dot {
  background: #1677ff;
  box-shadow: 0 0 0 4px rgba(22, 119, 255, 0.12);
  animation: request-records-pulse 1.2s ease-in-out infinite;
}

.request-records-action-button {
  height: 30px;
  padding: 0 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 6px 16px rgba(72, 95, 81, 0.08);
}

.request-records-action-button-refresh {
  border-color: rgba(111, 143, 121, 0.18);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(241, 246, 238, 0.94));
  color: #29422f;
}

.request-records-action-button-clear {
  border-color: rgba(191, 106, 98, 0.2);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(251, 241, 238, 0.94));
  color: #a15147;
}

.request-records-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.request-records-metric {
  display: grid;
  gap: 5px;
  min-height: 76px;
  padding: 10px 12px;
  border-radius: 18px;
  border: 1px solid rgba(110, 133, 118, 0.12);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(244, 248, 242, 0.94)),
    rgba(255, 255, 255, 0.94);
  box-shadow:
    0 14px 28px rgba(85, 104, 90, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.request-records-metric-label {
  color: #748076;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.request-records-metric-value {
  color: #213129;
  font-size: 22px;
  line-height: 1.05;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.request-records-metric small {
  color: #6e7d72;
  font-size: 11px;
  line-height: 1.35;
}

.request-records-board {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  flex: none;
  min-height: clamp(420px, 58vh, 680px);
  min-width: 0;
  width: 100%;
  border-radius: 20px;
  border: 1px solid rgba(103, 126, 111, 0.12);
  background:
    linear-gradient(180deg, rgba(254, 254, 253, 0.96), rgba(247, 249, 246, 0.94)),
    rgba(255, 255, 255, 0.94);
  box-shadow:
    0 18px 36px rgba(87, 105, 92, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.82);
  overflow: hidden;
}

.request-records-board-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid rgba(108, 129, 114, 0.1);
  min-width: 0;
}

.request-records-board-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.request-records-board-title strong {
  color: #24342b;
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.request-records-board-title span {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 9px;
  border-radius: 999px;
  background: rgba(232, 239, 230, 0.9);
  color: #536257;
  font-size: 11px;
  font-weight: 700;
}

.request-records-table-wrap {
  display: grid;
  grid-template-rows: minmax(0, 1fr) auto auto;
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  min-width: 0;
  width: 100%;
  overflow: hidden;
}

.request-records-table-spin {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  min-width: 0;
  width: 100%;
  overflow: hidden;
}

.request-records-table-spin :deep(.ant-spin-nested-loading) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  width: 100%;
}

.request-records-table-spin :deep(.ant-spin-container) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  width: 100%;
}

.request-records-table-scroll {
  flex: 1 1 auto;
  min-height: 240px;
  min-width: 0;
  width: 100%;
  max-width: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-gutter: stable;
}

.request-records-table-scroll.is-draggable {
  cursor: grab;
}

.request-records-table-scroll.is-dragging {
  cursor: grabbing;
  user-select: none;
}

.request-records-table-scroll.is-dragging * {
  user-select: none;
}

.request-records-table-stage {
  will-change: transform;
}

.request-records-table-hscroll {
  position: relative;
  z-index: 3;
  min-width: 0;
  width: 60%;
  height: 20px;
  margin: 4px 0 0 12px;
  display: flex;
  align-items: center;
  pointer-events: auto;
}

.request-records-table-hscroll-range {
  appearance: none;
  -webkit-appearance: none;
  width: 100%;
  height: 20px;
  margin: 0;
  background: transparent;
  pointer-events: auto;
}

.request-records-table-hscroll-range::-webkit-slider-runnable-track {
  height: 8px;
  border-radius: 999px;
  background: rgba(228, 235, 230, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.request-records-table-hscroll-range::-webkit-slider-thumb {
  appearance: none;
  -webkit-appearance: none;
  width: 32px;
  height: 12px;
  margin-top: -2px;
  border: 0;
  border-radius: 999px;
  background: rgba(120, 138, 126, 0.38);
  transition: background-color 0.18s ease, opacity 0.18s ease;
}

.request-records-table-hscroll.is-active .request-records-table-hscroll-range::-webkit-slider-thumb {
  background: rgba(96, 120, 104, 0.82);
}

.request-records-table-hscroll-range::-moz-range-track {
  height: 8px;
  border: 0;
  border-radius: 999px;
  background: rgba(228, 235, 230, 0.9);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.request-records-table-hscroll-range::-moz-range-thumb {
  width: 32px;
  height: 12px;
  border: 0;
  border-radius: 999px;
  background: rgba(120, 138, 126, 0.38);
  transition: background-color 0.18s ease, opacity 0.18s ease;
}

.request-records-table-hscroll.is-active .request-records-table-hscroll-range::-moz-range-thumb {
  background: rgba(96, 120, 104, 0.82);
}

.request-records-table-hscroll-range:disabled {
  opacity: 0.6;
  cursor: default;
}

.request-records-table-native {
  width: max-content;
  min-width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  table-layout: fixed;
}

.request-records-table-head {
  position: sticky;
  top: 0;
  z-index: 2;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(110, 132, 118, 0.1);
  background: rgba(248, 250, 247, 0.94);
  color: #718176;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  text-align: left;
  white-space: nowrap;
}

.request-records-table-head.is-center,
.request-records-table-cell.is-center {
  text-align: center;
}

.request-records-table-cell {
  padding: 9px 10px;
  border-bottom: 1px solid rgba(109, 128, 115, 0.08);
  background: transparent;
  vertical-align: top;
}

.request-records-table-native tbody tr:hover > .request-records-table-cell {
  background: rgba(241, 246, 239, 0.76);
}

.request-records-table-cell.is-actions,
.request-records-table-head.is-actions {
  width: 46px;
}

.request-records-empty-cell {
  padding: 56px 20px;
  color: #b6b9b7;
  font-size: 18px;
  text-align: center;
}

.request-records-pagination {
  display: flex;
  justify-content: flex-end;
  padding: 8px 12px 12px;
}

.request-records-table-scroll::-webkit-scrollbar:vertical {
  width: 10px;
}

.request-records-table-scroll::-webkit-scrollbar:horizontal {
  height: 0;
}

.request-records-table-scroll::-webkit-scrollbar-thumb:vertical {
  border-radius: 999px;
  background: rgba(120, 138, 126, 0.62);
}

.request-records-table-scroll::-webkit-scrollbar-track:vertical {
  background: rgba(228, 235, 230, 0.72);
}

.request-records-time,
.request-records-identity,
.request-records-route,
.request-records-metrics,
.request-records-status {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.request-records-time strong,
.request-records-identity strong,
.request-records-metrics strong,
.request-record-detail-item strong,
.request-record-detail-hero-main strong {
  color: #203028;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.request-records-time small,
.request-records-identity small,
.request-records-route small,
.request-records-metrics small,
.request-record-detail-item span,
.request-record-detail-hero-main small {
  color: #738277;
  font-size: 11px;
  line-height: 1.35;
}

.request-records-identity-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.request-records-identity-main strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-app-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(224, 236, 229, 0.94);
  color: #365149;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  flex: 0 0 auto;
}

.request-records-mono {
  font-family: 'Cascadia Code', 'Consolas', monospace;
}

.request-records-route-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.request-records-route-key {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 20px;
  border-radius: 999px;
  background: rgba(228, 234, 226, 0.94);
  color: #5b6b60;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
  flex: 0 0 auto;
}

.request-records-route-key.is-meta {
  background: rgba(241, 236, 223, 0.94);
  color: #8d6b2f;
}

.request-records-route-key.is-out {
  background: rgba(224, 232, 255, 0.92);
  color: #3e5fb9;
}

.request-records-route-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(224, 232, 255, 0.92);
  color: #3e5fb9;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
  flex: 0 0 auto;
}

.request-records-route-inbound,
.request-records-route-host,
.request-records-route-path {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-route-inbound {
  color: #4d5e54;
  font-size: 12px;
  font-weight: 600;
}

.request-records-route-host {
  color: #78867b;
}

.request-records-route-path {
  color: #4d5e54;
  font-size: 12px;
  font-weight: 600;
}

.request-records-route-path-out {
  color: #78867b;
  font-weight: 500;
}

.request-records-routing {
  display: grid;
  gap: 5px;
}

.request-records-routing-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  color: #55655a;
  font-size: 11px;
  line-height: 1.2;
}

.request-records-routing-line.is-direct {
  color: #58695f;
}

.request-records-routing-line.is-failed {
  color: #9b6b5a;
}

.request-records-routing-line.is-fallback-final {
  color: #2f7a45;
  font-weight: 700;
}

.request-records-routing-icon {
  width: 12px;
  text-align: center;
  font-size: 10px;
  flex: 0 0 auto;
}

.request-records-routing-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.request-records-routing-source {
  color: #8b998f;
  font-size: 10px;
  flex: 0 0 auto;
}

.request-records-status {
  justify-items: start;
}

.request-records-source-pill {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(233, 239, 231, 0.94);
  color: #55655a;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.request-records-source-pill.is-fallback {
  background: rgba(255, 236, 212, 0.94);
  color: #a16420;
}

.request-records-source-pill.is-preference {
  background: rgba(232, 226, 255, 0.94);
  color: #7655b5;
}

.request-records-source-pill.is-direct {
  background: rgba(219, 240, 255, 0.94);
  color: #2d72b9;
}

.request-records-detail-text {
  color: #516156;
  font-size: 12px;
  line-height: 1.45;
  white-space: normal;
  word-break: break-word;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.request-records-more {
  width: 28px;
  height: 28px;
  border-radius: 10px;
  color: #59695f;
}

.request-records-more:hover {
  background: rgba(231, 238, 228, 0.86);
  color: #223128;
}

.request-records-empty {
  padding: 18px;
  border-radius: 18px;
  border: 1px dashed rgba(118, 135, 121, 0.22);
  background: rgba(250, 252, 249, 0.92);
  color: #69786d;
  font-size: 12px;
}

.request-record-detail-shell {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.request-record-detail-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 13px;
  border-radius: 18px;
  border: 1px solid rgba(105, 126, 112, 0.12);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(244, 248, 242, 0.94)),
    rgba(255, 255, 255, 0.94);
}

.request-record-detail-hero-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.request-record-detail-section {
  display: grid;
  gap: 8px;
}

.request-record-detail-section-title {
  color: #6f7d73;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.request-record-detail-section-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.request-record-detail-section-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.request-record-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.request-record-detail-item {
  display: grid;
  gap: 5px;
  padding: 12px;
  border-radius: 16px;
  border: 1px solid rgba(107, 127, 114, 0.12);
  background: rgba(248, 251, 247, 0.94);
  min-width: 0;
}

.request-record-detail-item-full {
  grid-column: 1 / -1;
}

.request-record-detail-item pre {
  margin: 0;
  color: #4a5a50;
  font: 12px/1.6 'Cascadia Code', 'Consolas', monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.request-record-debug-button {
  border-radius: 10px;
}

.request-record-debug-result {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  font-size: 14px;
}

.request-record-debug-result.is-loading {
  color: #6f7d73;
}

.request-record-debug-result.is-success {
  color: #2f8f59;
}

.request-record-debug-result.is-error {
  color: #cc4b37;
}

.request-record-debug-textarea :deep(textarea) {
  min-height: 240px;
  border-radius: 16px;
  padding: 12px 13px;
  color: #4a5a50;
  font: 12px/1.6 'Cascadia Code', 'Consolas', monospace;
  background: rgba(248, 251, 247, 0.94);
  border-color: rgba(107, 127, 114, 0.12);
}

.request-records-shell-dark .request-records-toolbar-pill,
.request-records-shell-dark .request-records-board-chip,
.request-records-shell-dark .request-record-detail-item,
.request-records-shell-dark .request-record-detail-hero {
  border-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(28, 37, 32, 0.96), rgba(22, 30, 26, 0.94)),
    rgba(17, 24, 20, 0.92);
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.22);
}

.request-records-shell-dark .request-records-toolbar-pill-muted,
.request-records-shell-dark .request-records-board-chip-muted {
  background: rgba(25, 33, 29, 0.94);
  color: #a9b9af;
}

.request-records-shell-dark .request-records-board-chip-toggle.is-inactive {
  background: rgba(25, 33, 29, 0.94);
  border-color: rgba(133, 162, 145, 0.08);
  color: #8a9990;
  opacity: 0.72;
}

.request-records-shell-dark .request-records-toolbar-dot {
  background: #7fb486;
  box-shadow: 0 0 0 4px rgba(127, 180, 134, 0.12);
}

.request-records-shell-dark .request-records-overview .request-records-metric,
.request-records-shell-dark .request-records-board,
.request-records-shell-dark .request-records-empty {
  border-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(24, 33, 28, 0.96), rgba(18, 25, 21, 0.94)),
    rgba(17, 24, 20, 0.92);
  box-shadow: 0 18px 34px rgba(0, 0, 0, 0.22);
}

.request-records-shell-dark .request-records-table-head {
  border-bottom-color: rgba(129, 155, 140, 0.14);
  background: rgba(24, 33, 28, 0.96);
  color: #aebfb4;
}

.request-records-shell-dark .request-records-table-cell {
  border-bottom-color: rgba(129, 155, 140, 0.1);
}

.request-records-shell-dark .request-records-table-native tbody tr:hover > .request-records-table-cell {
  background: rgba(255, 255, 255, 0.04);
}

.request-records-shell-dark .request-records-empty-cell {
  color: #8ea196;
}

.request-records-shell-dark .request-records-table-scroll::-webkit-scrollbar-thumb:vertical {
  background: rgba(129, 155, 140, 0.52);
}

.request-records-shell-dark .request-records-table-scroll::-webkit-scrollbar-track:vertical {
  background: rgba(23, 31, 27, 0.82);
}

.request-records-shell-dark .request-records-table-hscroll-range::-webkit-slider-runnable-track {
  background: rgba(23, 31, 27, 0.86);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.request-records-shell-dark .request-records-table-hscroll-range::-webkit-slider-thumb {
  background: rgba(129, 155, 140, 0.4);
}

.request-records-shell-dark .request-records-table-hscroll.is-active .request-records-table-hscroll-range::-webkit-slider-thumb {
  background: rgba(129, 155, 140, 0.82);
}

.request-records-shell-dark .request-records-table-hscroll-range::-moz-range-track {
  background: rgba(23, 31, 27, 0.86);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.request-records-shell-dark .request-records-table-hscroll-range::-moz-range-thumb {
  background: rgba(129, 155, 140, 0.4);
}

.request-records-shell-dark .request-records-table-hscroll.is-active .request-records-table-hscroll-range::-moz-range-thumb {
  background: rgba(129, 155, 140, 0.82);
}

.request-records-shell-dark .request-records-metric-label,
.request-records-shell-dark .request-record-detail-section-title,
.request-records-shell-dark .request-record-detail-item span,
.request-records-shell-dark .request-record-detail-hero-main small,
.request-records-shell-dark .request-records-time small,
.request-records-shell-dark .request-records-identity small,
.request-records-shell-dark .request-records-route small,
.request-records-shell-dark .request-records-metrics small {
  color: #aebfb4;
}

.request-records-shell-dark .request-records-metric-value,
.request-records-shell-dark .request-records-board-title strong,
.request-records-shell-dark .request-records-time strong,
.request-records-shell-dark .request-records-identity strong,
.request-records-shell-dark .request-records-metrics strong,
.request-records-shell-dark .request-record-detail-item strong,
.request-records-shell-dark .request-record-detail-hero-main strong {
  color: #edf6ee;
}

.request-records-shell-dark .request-records-metric small,
.request-records-shell-dark .request-records-detail-text,
.request-records-shell-dark .request-records-empty,
.request-records-shell-dark .request-records-route-inbound,
.request-records-shell-dark .request-records-route-host,
.request-records-shell-dark .request-records-route-path,
.request-records-shell-dark .request-records-route-path-out,
.request-records-shell-dark .request-records-routing-line.is-direct,
.request-records-shell-dark .request-records-routing-source,
.request-records-shell-dark .request-record-detail-item pre {
  color: #b8c8be;
}

.request-records-shell-dark .request-record-debug-result.is-loading {
  color: #b8c8be;
}

.request-records-shell-dark .request-record-debug-result.is-success {
  color: #77d69a;
}

.request-records-shell-dark .request-record-debug-result.is-error {
  color: #ff8d7d;
}

.request-records-shell-dark .request-record-debug-textarea :deep(textarea) {
  color: #d3dfd6;
  background: rgba(18, 25, 21, 0.94);
  border-color: rgba(133, 162, 145, 0.16);
}

.request-records-shell-dark .request-records-routing-line.is-failed {
  color: #f0b8a5;
}

.request-records-shell-dark .request-records-routing-line.is-fallback-final {
  color: #87d39c;
}

.request-records-shell-dark .request-records-route-key {
  background: rgba(67, 84, 75, 0.9);
  color: #dbe8df;
}

.request-records-shell-dark .request-records-route-key.is-meta {
  background: rgba(108, 84, 42, 0.34);
  color: #ffe1aa;
}

.request-records-shell-dark .request-records-route-key.is-out {
  background: rgba(64, 86, 146, 0.36);
  color: #dce7ff;
}

.request-records-shell-dark .request-records-app-pill {
  background: rgba(54, 76, 65, 0.94);
  color: #d8e8dd;
}

.request-records-shell-dark .request-records-route-pill {
  background: rgba(64, 86, 146, 0.36);
  color: #dce7ff;
}

.request-records-shell-dark .request-records-source-pill {
  background: rgba(57, 74, 65, 0.92);
  color: #d9e8dc;
}

.request-records-shell-dark .request-records-source-pill.is-fallback {
  background: rgba(113, 78, 43, 0.34);
  color: #ffd39a;
}

.request-records-shell-dark .request-records-source-pill.is-preference {
  background: rgba(88, 64, 136, 0.32);
  color: #e0d6ff;
}

.request-records-shell-dark .request-records-source-pill.is-direct {
  background: rgba(49, 92, 132, 0.34);
  color: #cfe5ff;
}

.request-records-shell-dark .request-records-board-title span {
  background: rgba(56, 74, 65, 0.92);
  color: #d8e8dc;
}

.request-records-shell-dark .request-records-action-button-refresh {
  border-color: rgba(118, 160, 133, 0.22);
  background: linear-gradient(180deg, rgba(43, 63, 51, 0.96), rgba(36, 54, 44, 0.92));
  color: #e0f1e4;
}

.request-records-shell-dark .request-records-action-button-clear {
  border-color: rgba(172, 97, 90, 0.24);
  background: linear-gradient(180deg, rgba(85, 46, 43, 0.94), rgba(68, 37, 35, 0.9));
  color: #ffd8d2;
}

.request-records-shell-dark .request-records-more {
  color: #c8d8cd;
}

.request-records-shell-dark .request-records-more:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #f1f8f2;
}

.request-record-detail-shell-dark .request-record-detail-item,
.request-record-detail-shell-dark .request-record-detail-hero {
  border-color: rgba(133, 162, 145, 0.16);
  background:
    linear-gradient(180deg, rgba(28, 37, 32, 0.96), rgba(22, 30, 26, 0.94)),
    rgba(17, 24, 20, 0.92);
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.22);
}

.request-record-detail-shell-dark .request-record-detail-section-title,
.request-record-detail-shell-dark .request-record-detail-item span,
.request-record-detail-shell-dark .request-record-detail-hero-main small {
  color: #aebfb4;
}

.request-record-detail-shell-dark .request-record-detail-item strong,
.request-record-detail-shell-dark .request-record-detail-hero-main strong {
  color: #edf6ee;
}

.request-record-detail-shell-dark .request-record-detail-item pre {
  color: #b8c8be;
}

.request-record-detail-shell-dark .request-record-debug-result.is-loading {
  color: #b8c8be;
}

.request-record-detail-shell-dark .request-record-debug-result.is-success {
  color: #77d69a;
}

.request-record-detail-shell-dark .request-record-debug-result.is-error {
  color: #ff8d7d;
}

.request-record-detail-shell-dark .request-record-debug-textarea :deep(textarea) {
  color: #d3dfd6;
  background: rgba(18, 25, 21, 0.94);
  border-color: rgba(133, 162, 145, 0.16);
}

@keyframes request-records-pulse {
  0%,
  100% {
    transform: scale(1);
  }

  50% {
    transform: scale(1.15);
  }
}

@media (max-width: 780px) {
  .request-records-overview,
  .request-record-detail-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .request-records-board {
    min-height: 360px;
  }

  .request-records-table-wrap {
    min-height: 280px;
  }

  .request-records-toolbar,
  .request-records-board-head,
  .request-record-detail-hero {
    align-items: flex-start;
    flex-direction: column;
  }

  .request-records-toolbar-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
