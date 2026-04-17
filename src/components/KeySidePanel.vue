<template>
  <div class="key-side-panel">
    <div class="panel-shell">
      <div class="panel-ambient panel-ambient-top"></div>
      <div class="panel-ambient panel-ambient-bottom"></div>
      <div class="panel-scroll-guide" :class="{ 'is-idle': !hasScrollableContent }" aria-hidden="true">
        <span class="panel-scroll-guide-line"></span>
        <span class="panel-scroll-guide-dot" :style="panelScrollDotStyle"></span>
      </div>
      <div class="panel-topbar" title="拖动侧栏位置">
        <header class="panel-header">
          <div class="panel-header-copy">
            <span class="panel-header-kicker">Key Shelf</span>
            <span class="panel-header-title">密钥</span>
          </div>

          <div class="panel-header-actions">
            <button type="button" class="panel-header-button" @click="restoreMainWindow">
              <ExportOutlined />
            </button>

            <button type="button" class="panel-header-button panel-header-button-accent" @click="handleOpenEditor('')">
              <PlusOutlined />
            </button>
          </div>
        </header>
      </div>

      <section ref="panelBodyRef" class="panel-body" @scroll.passive="handlePanelScroll">
        <div v-if="visibleRecords.length === 0" class="panel-empty">
          <div class="panel-empty-badge">SK</div>
          <div class="panel-empty-title">暂无可用密钥</div>
          <div class="panel-empty-text">批量检测拿到的有效配置会在这里展示。</div>
        </div>

        <article
          v-for="(record, index) in visibleRecords"
          :key="record.rowKey"
          class="panel-record"
          :class="[`panel-record-${getQuickTestTone(record.quickTestStatus)}`]"
        >
          <div class="panel-record-top">
            <div class="panel-record-sitebox">
              <a-tooltip
                placement="top"
                :title="getAdvancedProxyTooltip(record) || null"
                :mouse-enter-delay="0.08"
              >
                <div class="panel-record-avatar" :class="getAdvancedProxyAvatarClass(record)">
                  <span class="panel-record-emoji">{{ getSiteEmoji(record.siteName) }}</span>
                  <span class="panel-record-order">No.{{ index + 1 }}</span>
                </div>
              </a-tooltip>
              <div class="panel-record-copy">
                <span class="panel-record-site">{{ getSiteShortName(record.siteName) }}</span>
                <span class="panel-record-model" :title="getModelSummary(record)">{{ getModelSummary(record) }}</span>
              </div>
            </div>

            <span class="panel-record-status" :class="`panel-record-status-${record.status === 1 ? 'ok' : 'bad'}`">
              {{ record.status === 1 ? '可用' : '异常' }}
            </span>
          </div>

          <div class="panel-record-metrics">
            <div class="panel-record-metrics-top">
              <div class="panel-record-quick-inline panel-record-quick-group">
                <span class="panel-record-quick">{{ getQuickStatusSummary(record) }}</span>
                <a-tooltip v-if="hasPerformanceMetrics(record)">
                  <template #title>
                    <div class="performance-tooltip-list">
                      <div v-for="line in getPerformanceTooltipLines(record)" :key="line">{{ line }}</div>
                    </div>
                  </template>
                  <span class="panel-performance-badge" aria-label="性能指标">
                    <ThunderboltOutlined />
                  </span>
                </a-tooltip>
              </div>
              <button
                v-if="canRefreshBalance(record, contextMap)"
                type="button"
                class="panel-refresh-button"
                :disabled="record.balanceLoading"
                @click="handleRefreshBalance(record)"
              >
                <ReloadOutlined :class="{ 'panel-spinning': record.balanceLoading }" />
              </button>
            </div>
            <div
              v-if="getCompactBalanceText(record)"
              class="panel-record-balance"
            >
              <span class="panel-record-balance-label">余额</span>
              <span class="panel-record-balance-value">{{ getCompactBalanceText(record) }}</span>
            </div>
          </div>

          <div class="panel-record-extra">
            <div class="panel-record-actions">
            <a-popover
              trigger="click"
              placement="leftTop"
              overlay-class-name="panel-model-popover"
              :get-popup-container="resolvePopoverContainer"
              :open="activePopoverRowKey === record.rowKey"
              @openChange="open => handlePopoverToggle(record, open)"
            >
              <template #content>
                <div class="panel-popover-content">
                  <div class="panel-popover-title">选择测活模型</div>
                  <a-select
                    size="small"
                    class="panel-model-select"
                    :value="record.selectedModel || undefined"
                    :options="getRecordModelOptions(record, contextMap)"
                    :loading="record.modelLoading"
                    :get-popup-container="resolveSelectContainer"
                    popup-class-name="panel-model-dropdown"
                    :dropdown-match-select-width="214"
                    show-search
                    :filter-option="true"
                    option-filter-prop="label"
                    @dropdownVisibleChange="open => handleModelSelectDropdown(record, open)"
                    @change="value => changeRecordModel(record, value)"
                  />
                </div>
              </template>

              <a-tooltip title="选择模型">
                <button type="button" class="panel-action-button">
                  <DatabaseOutlined />
                </button>
              </a-tooltip>
            </a-popover>

            <a-tooltip title="便捷一键设置">
              <button type="button" class="panel-action-button" @click="handleQuickSetup(record)">
                <img :src="quickSetupIcon" alt="便捷一键设置" class="panel-action-image" />
              </button>
            </a-tooltip>

            <a-tooltip placement="bottom" :arrow="{ pointAtCenter: true }" overlay-class-name="panel-quick-tooltip">
              <template #title>
                <div class="panel-quick-tooltip-content">{{ getTruncatedQuickTestHint(record) }}</div>
              </template>
              <button
                type="button"
                class="panel-action-button panel-action-button-primary"
                :disabled="record.quickTestLoading"
                @click="handleQuickTest(record)"
              >
                <ThunderboltOutlined />
              </button>
            </a-tooltip>

            <a-tooltip title="编辑配置">
              <button type="button" class="panel-action-button" @click="handleOpenEditor(record.rowKey)">
                <EditOutlined />
              </button>
            </a-tooltip>
            </div>
          </div>
        </article>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { message, Modal } from 'ant-design-vue';
import {
  DatabaseOutlined,
  EditOutlined,
  ExportOutlined,
  PlusOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue';
import {
  InitPanelWindow,
  OpenDesktopConfigWindow,
  OpenKeyEditor,
  RequestMainWindowRestore,
} from '../../wailsjs/go/main/App.js';
import quickSetupIcon from '../assets/action-icons/quick-setup-cute.svg';
import {
  KEY_MANAGEMENT_SYNC_EVENT,
  canRefreshBalance,
  getQuickTestTone,
  getRecordBalanceValue,
  getRecordModelOptions,
  hydrateRecordModelSelection,
  loadBatchHistoryContextMap,
  loadPanelRecords,
  loadRecordModelOptions,
  persistPanelRecords,
  refreshRecordBalance,
  runRecordQuickTest,
} from '../utils/keyPanelStore.js';
import {
  ADVANCED_PROXY_SYNC_EVENT,
  getAdvancedProxyTakeoverMap,
} from '../utils/advancedProxyBridge.js';
import { buildPerformanceTooltipLines, hasPerformanceMetrics } from '../utils/performanceMetrics.js';

const SITE_EMOJI_LIST = [
  '🦊', '🦉', '🦋', '🦭', '🦜', '🪿', '🐬', '🦄', '🐿️', '🪼',
  '🧭', '🪄', '🪁', '🎐', '🪵', '🍵', '🌿', '🍯', '🪴', '📮',
];

const COMPAT_SITE_EMOJI_LIST = [
  '🦊', '🐧', '🐬', '🦄', '🐼', '🐳', '🐙', '🐢', '🐝', '🐻',
  '🐱', '🐶', '🌵', '🍀', '🍵', '🍬', '📮', '🎯', '🎈', '🎁',
];

const QUICK_TOOLTIP_MAX_CHARS = 15;
const SIDEBAR_QUICK_TEST_TIMEOUT_MS = 25000;
const SIDEBAR_QUICK_TEST_TIMEOUT_SECONDS = Math.round(SIDEBAR_QUICK_TEST_TIMEOUT_MS / 1000);
const ADVANCED_PROXY_APP_META = {
  claude: { label: 'Claude', className: 'panel-record-avatar-app-claude' },
  codex: { label: 'Codex', className: 'panel-record-avatar-app-codex' },
  opencode: { label: 'OpenCode', className: 'panel-record-avatar-app-opencode' },
  openclaw: { label: 'OpenClaw', className: 'panel-record-avatar-app-openclaw' },
};
const ADVANCED_PROXY_APP_ORDER = ['claude', 'codex', 'opencode', 'openclaw'];

const records = ref([]);
const contextMap = ref(new Map());
const advancedProxyTakeoverMap = ref(getAdvancedProxyTakeoverMap());
const activePopoverRowKey = ref('');
const activeModelDropdownRowKey = ref('');
const panelBodyRef = ref(null);
const panelScrollRatio = ref(0);
const hasScrollableContent = ref(false);
const PANEL_PERSIST_DEBOUNCE_MS = 120;

let panelBodyResizeObserver = null;
let panelPersistTimer = null;

const visibleRecords = computed(() => records.value.filter(record => Number(record?.status || 0) === 1));
const panelScrollDotStyle = computed(() => {
  const ratio = Math.min(1, Math.max(0, Number(panelScrollRatio.value) || 0));
  return {
    top: `calc(${(ratio * 100).toFixed(2)}% - ${(ratio * 10).toFixed(2)}px)`,
    opacity: hasScrollableContent.value ? '1' : '0.34',
  };
});

function reloadRecords() {
  const loaded = loadPanelRecords();
  contextMap.value = loaded.contextMap || loadBatchHistoryContextMap();
  records.value = loaded.records;
  reloadAdvancedProxyTakeoverState();
  void nextTick(syncScrollIndicator);
}

function reloadAdvancedProxyTakeoverState(event = null) {
  const takeoverMap = event?.detail?.takeoverMap;
  advancedProxyTakeoverMap.value = takeoverMap && typeof takeoverMap === 'object'
    ? takeoverMap
    : getAdvancedProxyTakeoverMap();
}

function getAdvancedProxyAppsForRecord(record) {
  const rowKey = String(record?.rowKey || '').trim();
  if (!rowKey) return [];
  const byRowKey = advancedProxyTakeoverMap.value?.byRowKey || {};
  const matched = Array.isArray(byRowKey[rowKey]) ? byRowKey[rowKey] : [];
  return ADVANCED_PROXY_APP_ORDER.filter(appId => matched.includes(appId));
}

function getPrimaryAdvancedProxyApp(record) {
  return getAdvancedProxyAppsForRecord(record)[0] || '';
}

function getAdvancedProxyAvatarClass(record) {
  const appId = getPrimaryAdvancedProxyApp(record);
  if (!appId) return '';
  const className = ADVANCED_PROXY_APP_META[appId]?.className;
  return ['panel-record-avatar-takeover', className].filter(Boolean);
}

function getAdvancedProxyTooltip(record) {
  const appIds = getAdvancedProxyAppsForRecord(record);
  if (appIds.length === 0) return '';
  const labels = appIds.map(appId => ADVANCED_PROXY_APP_META[appId]?.label || appId);
  return `已进入代理接管：${labels.join(' / ')}`;
}

function getSiteShortName(siteName) {
  const text = String(siteName || '').trim();
  if (!text) return '未命名';
  return text.length > 6 ? text.slice(0, 6) : text;
}

function getSiteEmoji(siteName) {
  const text = String(siteName || '').trim().toLowerCase();
  if (!text) return COMPAT_SITE_EMOJI_LIST[0];

  let hash = 2166136261;
  for (let index = 0; index < text.length; index += 1) {
    hash ^= text.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  const listLength = COMPAT_SITE_EMOJI_LIST.length || 1;
  const normalizedHash = hash >>> 0;
  const emojiIndex = ((normalizedHash % listLength) + listLength) % listLength;
  const emoji = COMPAT_SITE_EMOJI_LIST[emojiIndex];
  return typeof emoji === 'string' && emoji.trim() ? emoji : COMPAT_SITE_EMOJI_LIST[0];
}

function getCompactBalanceText(record) {
  const value = String(getRecordBalanceValue(record) || '').trim();
  if (!value) return '';
  if (value === '无限') return value;

  const match = value.match(/-?\d+(?:\.\d+)?/);
  if (!match) return value;
  return `$${match[0]}`;
}

function getModelSummary(record) {
  return String(record?.selectedModel || '').trim() || '待抓取模型';
}

function getQuickStatusSummary(record) {
  if (record.quickTestLoading) return `正在测活 · ${SIDEBAR_QUICK_TEST_TIMEOUT_SECONDS}s 超时`;
  if (record.quickTestLabel) {
    const responseTime = record.quickTestResponseTime ? `${record.quickTestResponseTime}s` : '';
    return [record.quickTestLabel, responseTime].filter(Boolean).join(' · ');
  }
  return '未测活';
}

function buildQuickTestHint(record) {
  if (record.quickTestLoading) return `快速测活中 · 最长 ${SIDEBAR_QUICK_TEST_TIMEOUT_SECONDS}s`;
  if (record.quickTestLabel) {
    return [record.quickTestLabel, record.quickTestModel].filter(Boolean).join(' ');
  }
  return '快速测活';
}

function getTruncatedQuickTestHint(record) {
  const text = String(buildQuickTestHint(record) || '').replace(/\s+/g, ' ').trim();
  if (!text) return '';
  if (text.length <= QUICK_TOOLTIP_MAX_CHARS) return text;
  return `${text.slice(0, QUICK_TOOLTIP_MAX_CHARS)}...`;
}

function getPerformanceTooltipLines(record) {
  return buildPerformanceTooltipLines(record);
}

function resolvePopoverContainer(triggerNode) {
  return triggerNode?.closest?.('.panel-shell') || document.body;
}

function resolveSelectContainer(triggerNode) {
  return triggerNode?.closest?.('.panel-popover-content') || triggerNode?.parentNode || document.body;
}

function syncScrollIndicator() {
  const element = panelBodyRef.value;
  if (!element) {
    panelScrollRatio.value = 0;
    hasScrollableContent.value = false;
    return;
  }

  const maxScrollTop = Math.max(0, element.scrollHeight - element.clientHeight);
  hasScrollableContent.value = maxScrollTop > 2;
  panelScrollRatio.value = maxScrollTop > 0 ? element.scrollTop / maxScrollTop : 0;
}

function handlePanelScroll() {
  syncScrollIndicator();
}

async function setPanelInteractionLocked(locked) {
  try {
    await window?.go?.main?.App?.SetPanelInteractionLocked?.(locked);
  } catch {}
}

async function syncPanelInteractionLock() {
  await setPanelInteractionLocked(Boolean(activePopoverRowKey.value || activeModelDropdownRowKey.value));
}

async function restoreMainWindow() {
  try {
    await RequestMainWindowRestore();
  } catch (error) {
    message.error(error?.message || '无法恢复主窗口');
  }
}

async function handleOpenEditor(rowKey) {
  try {
    await OpenKeyEditor(String(rowKey || ''));
  } catch (error) {
    message.error(error?.message || '无法打开编辑窗口');
  }
}

async function handleQuickSetup(record) {
  try {
    await OpenDesktopConfigWindow(String(record?.rowKey || ''));
  } catch (error) {
    message.error(error?.message || '无法打开一键配置');
  }
}

function updateRecord(nextRecord) {
  const targetIndex = records.value.findIndex(item => item.rowKey === nextRecord.rowKey);
  if (targetIndex === -1) return;
  records.value[targetIndex] = nextRecord;
  schedulePanelPersist();
  void nextTick(syncScrollIndicator);
}

function flushPanelPersist() {
  if (panelPersistTimer) {
    clearTimeout(panelPersistTimer);
    panelPersistTimer = null;
  }
  persistPanelRecords(records.value, { broadcast: false });
}

function schedulePanelPersist() {
  if (panelPersistTimer) {
    clearTimeout(panelPersistTimer);
  }
  panelPersistTimer = setTimeout(() => {
    panelPersistTimer = null;
    flushPanelPersist();
  }, PANEL_PERSIST_DEBOUNCE_MS);
}

async function handlePopoverToggle(record, open) {
  activePopoverRowKey.value = open ? record.rowKey : '';
  await syncPanelInteractionLock();
}

async function handleModelSelectDropdown(record, open) {
  activeModelDropdownRowKey.value = open ? record.rowKey : '';
  await syncPanelInteractionLock();
  if (!open || record.modelLoading) return;

  record.modelLoading = true;
  try {
    const nextRecord = await loadRecordModelOptions(record, contextMap.value);
    updateRecord(hydrateRecordModelSelection(nextRecord, contextMap.value));
  } catch (error) {
    message.error(error?.message || '模型获取失败');
  } finally {
    record.modelLoading = false;
  }
}

function changeRecordModel(record, value) {
  const nextRecord = hydrateRecordModelSelection(
    {
      ...record,
      selectedModel: String(value || '').trim(),
    },
    contextMap.value,
  );
  updateRecord(nextRecord);
}

async function handleQuickTest(record) {
  if (record.quickTestLoading) return;

  record.quickTestLoading = true;
  let timeoutId = null;
  try {
    const timeoutPromise = new Promise((_, reject) => {
      timeoutId = window.setTimeout(() => {
        const timeoutError = new Error(`快速测活超时（${SIDEBAR_QUICK_TEST_TIMEOUT_SECONDS}s）`);
        timeoutError.detail = `快速测活超时（${SIDEBAR_QUICK_TEST_TIMEOUT_SECONDS}s）\n请检查接口稳定性、模型可用性，或稍后重试。`;
        reject(timeoutError);
      }, SIDEBAR_QUICK_TEST_TIMEOUT_MS);
    });

    const nextRecord = await Promise.race([
      runRecordQuickTest(record, contextMap.value),
      timeoutPromise,
    ]);

    updateRecord(nextRecord);
  } catch (error) {
    const detail = String(error?.detail || error?.message || '快速测活失败').trim();
    updateRecord({
      ...record,
      quickTestStatus: 'error',
      quickTestLabel: '失败',
      quickTestRemark: detail,
      quickTestAt: Date.now(),
      quickTestResponseTime: '',
      quickTestTtftMs: '',
      quickTestTps: '',
      quickTestResponseContent: detail,
      quickTestLoading: false,
    });
    message.error(error?.message || '快速测活失败');
    showQuickTestErrorDialog(detail);
  } finally {
    if (timeoutId != null) {
      clearTimeout(timeoutId);
    }
    record.quickTestLoading = false;
  }
}

function showQuickTestErrorDialog(detailText) {
  Modal.error({
    title: '快速测活失败',
    width: 760,
    okText: '关闭',
    content: h('div', {
      style: {
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
        maxHeight: '60vh',
        overflow: 'auto',
        fontSize: '12px',
        lineHeight: '1.6',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
      },
    }, detailText),
  });
}

async function handleRefreshBalance(record) {
  if (record.balanceLoading || !canRefreshBalance(record, contextMap.value)) return;

  record.balanceLoading = true;
  try {
    const nextRecord = await refreshRecordBalance(record, contextMap.value);
    updateRecord(nextRecord);
  } catch (error) {
    updateRecord({
      ...record,
      balanceError: error?.message || '余额刷新失败',
      balanceLoading: false,
    });
    message.error(error?.message || '余额刷新失败');
  } finally {
    record.balanceLoading = false;
  }
}

onMounted(async () => {
  reloadRecords();
  try {
    await InitPanelWindow(window?.screen?.availWidth || 1440, window?.screen?.availHeight || 900);
  } catch {}
  await nextTick();
  syncScrollIndicator();
  if (typeof ResizeObserver === 'function' && panelBodyRef.value) {
    panelBodyResizeObserver = new ResizeObserver(() => {
      syncScrollIndicator();
    });
    panelBodyResizeObserver.observe(panelBodyRef.value);
  }
  await setPanelInteractionLocked(false);

  window.addEventListener('resize', syncScrollIndicator);
  window.addEventListener(KEY_MANAGEMENT_SYNC_EVENT, reloadRecords);
  window.addEventListener('storage', reloadRecords);
  window.addEventListener(ADVANCED_PROXY_SYNC_EVENT, reloadAdvancedProxyTakeoverState);
});

watch(visibleRecords, async () => {
  await nextTick();
  syncScrollIndicator();
}, { flush: 'post' });

onBeforeUnmount(() => {
  flushPanelPersist();
  void setPanelInteractionLocked(false);
  panelBodyResizeObserver?.disconnect?.();
  panelBodyResizeObserver = null;
  window.removeEventListener('resize', syncScrollIndicator);
  window.removeEventListener(KEY_MANAGEMENT_SYNC_EVENT, reloadRecords);
  window.removeEventListener('storage', reloadRecords);
  window.removeEventListener(ADVANCED_PROXY_SYNC_EVENT, reloadAdvancedProxyTakeoverState);
});
</script>

<style scoped>
.key-side-panel {
  --panel-text: #f5f0e8;
  --panel-muted: rgba(245, 240, 232, 0.66);
  --panel-card: rgba(246, 242, 235, 0.92);
  --panel-gold: #b8872c;
  --panel-ink: #243042;
  --panel-shadow: 0 24px 48px rgba(28, 31, 36, 0.22);
  width: 100%;
  min-width: 0;
  height: 100vh;
  padding: 6px 0 8px 6px;
  box-sizing: border-box;
  background: transparent;
  overflow: hidden;
  overflow-x: hidden;
  position: relative;
  font-family: "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans SC", sans-serif;
  user-select: none;
  -webkit-user-select: none;
}

.panel-shell {
  position: relative;
  z-index: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 8px 6px 12px 8px;
  border-radius: 28px 0 0 28px;
  background:
    linear-gradient(180deg, rgba(64, 74, 91, 0.92), rgba(31, 39, 52, 0.9));
  box-shadow:
    0 28px 54px rgba(16, 19, 25, 0.26),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(10px) saturate(108%);
  overflow: hidden;
  overflow-x: hidden;
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.panel-shell::before {
  content: "";
  position: absolute;
  left: 10px;
  right: 0;
  top: 66px;
  bottom: 8px;
  border-radius: 24px 0 24px 24px;
  background:
    linear-gradient(180deg, rgba(89, 98, 116, 0.66), rgba(42, 49, 62, 0.76));
  box-shadow:
    inset 0 16px 26px rgba(19, 24, 33, 0.22),
    inset 0 -12px 20px rgba(255, 255, 255, 0.04);
  pointer-events: none;
}

.panel-ambient {
  position: absolute;
  border-radius: 999px;
  filter: blur(2px);
  pointer-events: none;
}

.panel-ambient-top {
  top: -18px;
  left: -10px;
  width: 110px;
  height: 110px;
  background: radial-gradient(circle, rgba(232, 194, 118, 0.34), rgba(232, 194, 118, 0));
}

.panel-ambient-bottom {
  right: -18px;
  bottom: 56px;
  width: 86px;
  height: 86px;
  background: radial-gradient(circle, rgba(129, 168, 212, 0.2), rgba(129, 168, 212, 0));
}

.panel-scroll-guide {
  position: absolute;
  left: 4px;
  top: 86px;
  bottom: 18px;
  width: 12px;
  z-index: 2;
  pointer-events: none;
}

.panel-scroll-guide-line {
  position: absolute;
  left: 50%;
  top: 0;
  bottom: 0;
  width: 3px;
  transform: translateX(-50%);
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(189, 244, 219, 0.18), rgba(165, 245, 214, 0.86), rgba(112, 189, 158, 0.24));
  box-shadow:
    0 0 10px rgba(144, 224, 187, 0.18),
    0 0 18px rgba(84, 156, 123, 0.12);
  opacity: 0.9;
}

.panel-scroll-guide-dot {
  position: absolute;
  left: 50%;
  width: 10px;
  height: 10px;
  border-radius: 999px;
  transform: translateX(-50%);
  background: radial-gradient(circle, rgba(245, 255, 250, 0.98) 0%, rgba(176, 255, 222, 0.96) 42%, rgba(98, 194, 151, 0.88) 72%, rgba(98, 194, 151, 0) 100%);
  box-shadow:
    0 0 0 1px rgba(226, 255, 240, 0.32),
    0 0 14px rgba(144, 250, 201, 0.44),
    0 0 24px rgba(84, 156, 123, 0.26);
  transition: top 0.16s ease, opacity 0.16s ease, transform 0.16s ease;
}

.panel-scroll-guide.is-idle .panel-scroll-guide-line {
  opacity: 0.38;
}

.panel-scroll-guide.is-idle .panel-scroll-guide-dot {
  transform: translateX(-50%) scale(0.82);
}

.panel-topbar {
  position: relative;
  z-index: 2;
  width: 100%;
  box-sizing: border-box;
  padding: 8px 8px 10px;
  margin: 0;
  cursor: grab;
  --wails-draggable: drag;
}

.panel-topbar::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 26px 20px 18px 18px;
  background: linear-gradient(180deg, rgba(53, 92, 74, 0.98), rgba(26, 43, 35, 0.97));
  box-shadow:
    inset 0 1px 0 rgba(173, 221, 199, 0.24),
    inset 0 -10px 16px rgba(8, 18, 14, 0.28),
    0 16px 28px rgba(8, 14, 12, 0.24);
  pointer-events: none;
}

.panel-topbar::after {
  content: "";
  position: absolute;
  left: 16px;
  right: 16px;
  bottom: -2px;
  height: 8px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(8, 21, 16, 0.44), rgba(8, 21, 16, 0));
  pointer-events: none;
}

.panel-topbar:active {
  cursor: grabbing;
}

.panel-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 44px;
  padding: 2px 2px 0;
}

.panel-header-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
}

.panel-header-kicker {
  color: rgba(205, 231, 220, 0.74);
  font-size: 8px;
  line-height: 1;
  letter-spacing: 0.28em;
  text-transform: uppercase;
}

.panel-header-title {
  color: #eef8f1;
  font-size: 16px;
  line-height: 1.05;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-shadow: 0 2px 10px rgba(8, 18, 14, 0.22);
}

.panel-header-actions {
  display: flex;
  gap: 6px;
  --wails-draggable: no-drag;
}

.panel-header-button,
.panel-action-button,
.panel-refresh-button {
  border: 0;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, background 0.18s ease;
}

.panel-header-button {
  width: 34px;
  height: 34px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--panel-text);
  background: linear-gradient(180deg, rgba(239, 250, 244, 0.18), rgba(239, 250, 244, 0.07));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.16),
    0 10px 20px rgba(8, 18, 14, 0.18);
  --wails-draggable: no-drag;
}

.panel-header-button-accent {
  background: linear-gradient(180deg, rgba(108, 171, 139, 0.3), rgba(59, 103, 82, 0.2));
}

.panel-header-button:hover,
.panel-action-button:hover,
.panel-refresh-button:hover {
  transform: translateY(-1px);
}

.panel-body {
  position: relative;
  z-index: 1;
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 10px 2px 12px 8px;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.panel-body::-webkit-scrollbar {
  width: 0;
}

.panel-empty {
  flex: 0 0 auto;
  min-height: 152px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  text-align: center;
  color: var(--panel-muted);
  border-radius: 24px;
  background: rgba(248, 242, 232, 0.08);
}

.panel-empty-badge {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 249, 241, 0.12);
  color: var(--panel-text);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.14em;
}

.panel-empty-title {
  color: var(--panel-text);
  font-size: 13px;
  font-weight: 700;
}

.panel-empty-text {
  max-width: 126px;
  font-size: 10px;
  line-height: 1.45;
}

.panel-record {
  position: relative;
  flex: 0 0 auto;
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  padding: 13px 12px 14px;
  border-radius: 22px;
  background: var(--panel-card);
  box-shadow: 0 10px 20px rgba(21, 22, 28, 0.1);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.52);
  transition: transform 0.18s ease, box-shadow 0.18s ease, background 0.18s ease;
  contain: layout paint;
}

.panel-record::before {
  content: "";
  position: absolute;
  left: 8px;
  right: 8px;
  top: -8px;
  height: 14px;
  border-radius: 16px;
  background: rgba(255, 249, 239, 0.4);
  transform: scaleY(0.8);
  opacity: 0.72;
  z-index: -1;
}

.panel-record::after {
  content: "";
  position: absolute;
  left: 14px;
  right: 14px;
  top: -14px;
  height: 16px;
  border-radius: 16px;
  background: rgba(240, 235, 227, 0.24);
  transform: scaleY(0.76);
  opacity: 0.58;
  z-index: -2;
}

.panel-record-success {
  background: linear-gradient(180deg, rgba(243, 251, 246, 0.96), rgba(228, 241, 233, 0.94));
}

.panel-record-warning {
  background: linear-gradient(180deg, rgba(249, 244, 232, 0.98), rgba(238, 229, 207, 0.94));
}

.panel-record-error {
  background: linear-gradient(180deg, rgba(244, 241, 237, 0.98), rgba(232, 226, 219, 0.94));
}

.panel-record-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.panel-record-sitebox {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-record-avatar {
  position: relative;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  border-radius: 11px;
}

.panel-record-avatar::before,
.panel-record-avatar::after {
  content: "";
  position: absolute;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.18s ease;
}

.panel-record-avatar-takeover {
  --panel-takeover-color: rgba(255, 255, 255, 0.95);
  --panel-takeover-glow: rgba(255, 255, 255, 0.28);
}

.panel-record-avatar-takeover::before {
  inset: -5px;
  z-index: 0;
  padding: 2px;
  border-radius: 15px;
  opacity: 1;
  background:
    conic-gradient(
      from 0deg,
      transparent 0deg 24deg,
      var(--panel-takeover-color) 42deg 92deg,
      transparent 118deg 162deg,
      var(--panel-takeover-color) 198deg 244deg,
      transparent 270deg 324deg,
      var(--panel-takeover-color) 338deg 360deg
    );
  box-shadow:
    0 0 8px var(--panel-takeover-glow),
    0 0 14px var(--panel-takeover-glow);
  animation: panel-avatar-orbit 2.3s linear infinite;
  -webkit-mask:
    linear-gradient(#000 0 0) content-box,
    linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
}

.panel-record-avatar-takeover::after {
  inset: -2px;
  z-index: 0;
  opacity: 1;
  border-radius: 13px;
  border: 1px solid color-mix(in srgb, var(--panel-takeover-color) 72%, transparent);
  box-shadow:
    0 0 0 1px color-mix(in srgb, var(--panel-takeover-color) 22%, transparent),
    0 0 12px var(--panel-takeover-glow);
}

.panel-record-avatar-app-claude {
  --panel-takeover-color: rgba(255, 255, 255, 0.96);
  --panel-takeover-glow: rgba(255, 255, 255, 0.34);
}

.panel-record-avatar-app-codex {
  --panel-takeover-color: rgba(255, 214, 102, 0.98);
  --panel-takeover-glow: rgba(255, 191, 68, 0.4);
}

.panel-record-avatar-app-opencode {
  --panel-takeover-color: rgba(100, 236, 255, 0.98);
  --panel-takeover-glow: rgba(70, 210, 224, 0.42);
}

.panel-record-avatar-app-openclaw {
  --panel-takeover-color: rgba(196, 128, 255, 0.98);
  --panel-takeover-glow: rgba(164, 94, 235, 0.4);
}

.panel-record-emoji {
  position: relative;
  z-index: 1;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
  background: rgba(34, 49, 72, 0.07);
  font-size: 16px;
  line-height: 1;
}

.panel-record-order {
  position: absolute;
  right: -8px;
  top: -6px;
  z-index: 2;
  padding: 1px 4px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(61, 110, 88, 0.98), rgba(24, 44, 35, 0.98));
  box-shadow:
    0 4px 10px rgba(13, 24, 20, 0.24),
    inset 0 1px 0 rgba(223, 255, 239, 0.2);
  color: #eef8f1;
  font-size: 8px;
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: 0.01em;
  white-space: nowrap;
}

.panel-record-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.panel-record-site {
  color: #162338;
  font-size: 13px;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.panel-record-model {
  color: rgba(35, 49, 72, 0.62);
  font-size: 10px;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.panel-record-status {
  width: fit-content;
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 10px;
  line-height: 1;
  font-weight: 700;
}

.panel-record-status-ok {
  color: #117b53;
  background: rgba(70, 200, 130, 0.16);
}

.panel-record-status-bad {
  color: #806553;
  background: rgba(123, 98, 78, 0.12);
}

.panel-record-metrics {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
}

.panel-record-metrics-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.panel-record-quick-group {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.panel-record-balance {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 34px;
  padding: 7px 10px;
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 244, 201, 0.92), rgba(251, 230, 156, 0.84));
}

.panel-record-balance-empty {
  background: linear-gradient(180deg, rgba(233, 237, 238, 0.92), rgba(220, 226, 228, 0.84));
}

.panel-record-balance-label {
  color: rgba(22, 35, 56, 0.58);
  font-size: 10px;
  line-height: 1;
  font-weight: 600;
}

.panel-record-balance-value {
  min-width: 0;
  color: var(--panel-gold);
  font-size: 12px;
  line-height: 1;
  font-weight: 700;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.panel-record-balance-empty .panel-record-balance-value {
  color: rgba(22, 35, 56, 0.52);
}

.panel-refresh-button {
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 11px;
  color: #7b6113;
  background: rgba(255, 249, 239, 0.9);
}

.panel-record-meta {
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 10px;
}

.panel-record-quick {
  color: rgba(35, 49, 72, 0.7);
  font-size: 10px;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.panel-record-quick-inline {
  flex: 1;
}

.performance-tooltip-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.panel-performance-badge {
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid rgba(217, 119, 6, 0.22);
  background: rgba(255, 247, 237, 0.92);
  color: #d97706;
  font-size: 10px;
  line-height: 1;
  cursor: help;
}

.panel-record-extra {
  max-height: 0;
  opacity: 0;
  transform: translateY(-4px);
  overflow: hidden;
  transition: max-height 0.18s ease, opacity 0.16s ease, transform 0.18s ease;
  pointer-events: none;
}

.panel-record:hover .panel-record-extra,
.panel-record:focus-within .panel-record-extra {
  max-height: 96px;
  opacity: 1;
  transform: translateY(0);
  pointer-events: auto;
}

.panel-record-actions {
  overflow: hidden;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 6px;
  margin-top: 10px;
}

.panel-action-button {
  min-width: 0;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border-radius: 13px;
  color: #1a2740;
  background: rgba(252, 248, 242, 0.94);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.78);
  font-size: 14px;
  --wails-draggable: no-drag;
}

.panel-action-button-primary {
  color: #f8fbff;
  background: linear-gradient(180deg, rgba(126, 194, 255, 0.96), rgba(80, 147, 244, 0.86));
}

.panel-action-image {
  width: 15px;
  height: 15px;
  flex: 0 0 auto;
}

.panel-action-button:disabled,
.panel-refresh-button:disabled {
  opacity: 0.42;
  cursor: default;
}

.panel-popover-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: min(214px, calc(100vw - 40px));
  max-width: calc(100vw - 40px);
  min-width: 0;
  box-sizing: border-box;
}

.panel-popover-title {
  color: #17243a;
  font-size: 13px;
  line-height: 1.2;
  font-weight: 700;
}

.panel-model-select {
  display: block;
  width: 100%;
  max-width: 100%;
  min-width: 0;
}

.panel-model-select :deep(.ant-select-selector) {
  padding-inline: 8px !important;
}

.panel-model-select :deep(.ant-select-selection-search) {
  max-width: 100%;
}

.panel-model-select :deep(.ant-select-selection-search-input) {
  min-width: 0 !important;
}

.panel-model-select :deep(.ant-select-selection-item),
.panel-model-select :deep(.ant-select-selection-placeholder) {
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.panel-model-popover) {
  max-width: calc(100vw - 32px);
}

:deep(.panel-model-popover .ant-popover-inner) {
  max-width: calc(100vw - 32px);
  overflow: hidden;
}

:deep(.panel-model-popover .ant-select-selector),
:deep(.panel-model-popover .ant-select-selection-item),
:deep(.panel-model-popover .ant-select-selection-placeholder) {
  max-width: 100%;
}

:deep(.panel-model-dropdown) {
  max-width: calc(100vw - 40px);
}

:deep(.panel-model-dropdown .ant-select-item-option-content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
}

:deep(.panel-quick-tooltip) {
  pointer-events: none;
}

:deep(.panel-quick-tooltip .ant-tooltip-inner) {
  max-width: 200px;
  overflow: hidden;
  white-space: nowrap;
}

:deep(.panel-quick-tooltip .ant-tooltip-arrow) {
  display: block !important;
  visibility: visible !important;
}

.panel-quick-tooltip-content {
  max-width: 160px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
}

.panel-spinning {
  animation: panel-spin 1s linear infinite;
}

:deep(.panel-model-popover .ant-popover-inner) {
  border-radius: 18px;
}

:deep(.panel-model-popover .ant-popover-inner-content) {
  padding: 10px 10px 10px 8px;
}

@keyframes panel-spin {
  from { transform: rotate(0); }
  to { transform: rotate(360deg); }
}

@keyframes panel-avatar-orbit {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes panel-card-in {
  from {
    opacity: 0;
    transform: translateX(14px) translateY(4px);
  }
  to {
    opacity: 1;
    transform: translateX(0) translateY(0);
  }
}
</style>
