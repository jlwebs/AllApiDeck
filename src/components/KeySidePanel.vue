<template>
  <div class="key-side-panel" :class="{ 'is-super-mini': superMiniMode }">
    <div ref="panelShellRef" class="panel-shell" :class="{ 'is-super-mini': superMiniMode }">
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

      <section
        v-if="showAdvancedProxyQueueCard"
        class="panel-queue-card"
        :class="[`panel-queue-card-${advancedProxyQueueTone}`]"
        :title="superMiniQueueCardHintText"
        @mouseenter="beginSuperMiniQueueCardHintHover"
        @mouseleave="commitSuperMiniQueueCardHintSeen"
        @pointerdown="beginSuperMiniWindowDrag"
        @pointermove="dragSuperMiniWindow"
        @pointerrawupdate="dragSuperMiniWindow"
        @pointerup="endSuperMiniWindowDrag"
        @pointercancel="endSuperMiniWindowDrag"
        @dblclick.stop.prevent="toggleSuperMiniMode"
      >
        <div class="panel-queue-head">
          <span class="panel-queue-title">{{ advancedProxyQueueTitle }}</span>

          <div class="panel-queue-signals" aria-hidden="true">
            <span class="panel-queue-signal panel-queue-signal-red" :class="{ 'is-active': advancedProxyQueueTone === 'red' }"></span>
            <span class="panel-queue-signal panel-queue-signal-yellow" :class="{ 'is-active': advancedProxyQueueTone === 'yellow' }"></span>
            <span class="panel-queue-signal panel-queue-signal-green" :class="{ 'is-active': advancedProxyQueueTone === 'green' }"></span>
          </div>
        </div>

        <div
          v-if="advancedProxyQueueItems.length > 0"
          ref="panelQueueStripRef"
          class="panel-queue-strip"
          @pointerdown.stop="beginQueueStripDrag"
          @pointermove="dragQueueStrip"
          @pointerup="endQueueStripDrag"
          @pointercancel="endQueueStripDrag"
          @pointerleave="endQueueStripDrag"
        >
          <transition-group
            name="panel-queue-item"
            tag="div"
            class="panel-queue-strip-track"
          >
            <div
              v-for="item in advancedProxyQueueItems"
              :key="item.id"
              class="panel-queue-item"
            >
              <div class="panel-queue-item-avatar-wrap">
                <a-tooltip
                  v-if="item.hasTooltip"
                  placement="top"
                  overlay-class-name="panel-queue-tooltip"
                  :overlay-style="getQueueTooltipOverlayStyle(item)"
                  :destroy-tooltip-on-hide="true"
                >
                  <template #title>
                    <div class="panel-queue-tooltip-content">
                      <strong>{{ item.siteName }}</strong>
                      <div class="panel-queue-tooltip-status">
                        <span
                          class="panel-queue-tooltip-state"
                          :class="{
                            'is-active': item.dispatchState.active,
                            'is-fading': item.dispatchState.fading,
                          }"
                          :style="{ opacity: item.dispatchState.visible ? item.dispatchState.opacity : 1 }"
                        >
                          {{ item.dispatchState.visible ? `⭐ ${item.dispatchState.labelText || '代理调用中'}` : '● Not Hit' }}
                        </span>
                      </div>
                      <div v-if="item.hitSummaryText" class="panel-queue-tooltip-hit-summary">
                        {{ item.hitSummaryText }}
                      </div>
                      <span>{{ item.modelLabel }}</span>
                      <span v-if="item.queueScopeText">{{ item.queueScopeText }}</span>
                      <code v-if="item.endpoint">{{ item.endpoint }}</code>
                      <code v-if="item.apiKey">{{ item.apiKey }}</code>
                    </div>
                  </template>

                  <div class="panel-queue-item-avatar">
                    <span class="panel-record-emoji">{{ item.avatar }}</span>
                  </div>
                </a-tooltip>
                <div v-else class="panel-queue-item-avatar">
                  <span class="panel-record-emoji">{{ item.avatar }}</span>
                </div>
                <span
                  v-if="item.dispatchState.visible"
                  class="panel-queue-item-star"
                  :class="{ 'is-fading': item.dispatchState.fading }"
                  :style="{ opacity: item.dispatchState.opacity }"
                  aria-hidden="true"
                >
                  ★
                </span>
                <span v-if="item.order !== null && item.order !== undefined" class="panel-queue-item-order">No.{{ item.order }}</span>
              </div>

              <div class="panel-queue-item-copy">
                <span class="panel-queue-item-name">{{ item.siteName.slice(0, 3) }}</span>
              </div>
            </div>
          </transition-group>
        </div>

        <div v-else class="panel-queue-empty">
          当前还没有可用 Provider，去高级代理里补一条队列记录就会出现在这里。
        </div>
      </section>

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
          :class="[
            `panel-record-${getQuickTestTone(record.quickTestStatus)}`,
            getAdvancedProxyCardStateClass(record),
          ]"
        >
          <div class="panel-record-top">
            <div class="panel-record-sitebox">
              <a-tooltip
                v-if="getAdvancedProxyTooltipLines(record).length > 0"
                placement="top"
                :mouse-enter-delay="0.08"
              >
                <template #title>
                  <div class="panel-routing-tooltip">
                    <div v-for="line in getAdvancedProxyTooltipLines(record)" :key="line">{{ line }}</div>
                  </div>
                </template>
                <div class="panel-record-avatar" :class="getAdvancedProxyAvatarClass(record)">
                  <span class="panel-record-emoji">{{ getSiteEmoji(record.siteName) }}</span>
                  <span v-if="isAdvancedProxyDispatching(record)" class="panel-record-dispatch-star">
                    ⭐ {{ getAdvancedProxyDispatchLabel(record) || '代理调用中' }}
                  </span>
                  <span v-if="getAdvancedProxyQueueOrder(record)" class="panel-record-order">
                    No.{{ getAdvancedProxyQueueOrder(record) }}
                  </span>
                </div>
              </a-tooltip>
              <div v-else class="panel-record-avatar" :class="getAdvancedProxyAvatarClass(record)">
                <span class="panel-record-emoji">{{ getSiteEmoji(record.siteName) }}</span>
                <span v-if="isAdvancedProxyDispatching(record)" class="panel-record-dispatch-star">
                  ⭐ {{ getAdvancedProxyDispatchLabel(record) || '代理调用中' }}
                </span>
                <span v-if="getAdvancedProxyQueueOrder(record)" class="panel-record-order">
                  No.{{ getAdvancedProxyQueueOrder(record) }}
                </span>
              </div>
              <div class="panel-record-copy">
                <span class="panel-record-site">{{ getSiteShortName(record.siteName) }}</span>
                <span class="panel-record-model" :title="getModelSummary(record)">{{ getModelSummary(record) }}</span>
              </div>
            </div>

            <span
              class="panel-record-status"
              :class="`panel-record-status-${record.status === 1 ? 'ok' : 'bad'}`"
              :title="record.status === 1 ? '可用' : '异常'"
              :aria-label="record.status === 1 ? '可用' : '异常'"
            >
              <span v-if="record.status === 1" class="panel-record-status-dot" aria-hidden="true"></span>
              <span v-else>异常</span>
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
              <a-tooltip v-if="canRefreshBalance(record, contextMap)" title="刷新余额">
                <button
                  type="button"
                  class="panel-refresh-button"
                  :disabled="record.balanceLoading"
                  aria-label="刷新余额"
                  @click="handleRefreshBalance(record)"
                >
                  <ReloadOutlined :class="{ 'panel-spinning': record.balanceLoading }" />
                </button>
              </a-tooltip>
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
  AppendClientLog,
  GetPanelDockState,
  InitPanelWindow,
  GetPanelWindowBounds,
  OpenDesktopConfigWindow,
  OpenKeyEditor,
  RequestMainWindowRestore,
} from '../../wailsjs/go/main/App.js';
import {
  WindowGetPosition,
  WindowGetSize,
  WindowSetPosition,
  WindowSetSize,
} from '../../wailsjs/runtime/runtime.js';
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
import { hydrateLastResultsSnapshotCache, HISTORY_SNAPSHOT_SYNC_EVENT } from '../utils/historySnapshotStore.js';
import {
  ADVANCED_PROXY_SYNC_EVENT,
  ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE,
  getAdvancedProxyRoutingLocalSnapshot,
  getAdvancedProxyRoutingSnapshot,
  getAdvancedProxyLocalSnapshot,
  getAdvancedProxyQueueProviders,
  getAdvancedProxyTakeoverMap,
} from '../utils/advancedProxyBridge.js';
import { logClientDiagnostic } from '../utils/clientDiagnostics.js';
import {
  buildPerformanceTooltipLines,
  extractPerformanceMetrics,
  hasPerformanceMetrics,
} from '../utils/performanceMetrics.js';

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
const ADVANCED_PROXY_ACTIVE_ROUTE_WINDOW_MS = 4500;
const ADVANCED_PROXY_RECENT_ROUTE_WINDOW_MS = 45000;
const ADVANCED_PROXY_DISPATCH_HOLD_MS = 10000;
const ADVANCED_PROXY_DISPATCH_FADE_MS = 2500;
const ADVANCED_PROXY_DISPATCH_TICK_MS = 500;
const SUPER_MINI_SCALE = 1.2;
const SUPER_MINI_QUEUE_CARD_HINT_TEXT = 'double click me!';
const SUPER_MINI_QUEUE_CARD_HINT_LIMIT = 2;
const SUPER_MINI_QUEUE_CARD_HINT_MIN_HOVER_MS = 800;
const SUPER_MINI_QUEUE_CARD_HINT_STORAGE_KEY = 'panel.super-mini.queue-card-hint-seen-count';
const ADVANCED_PROXY_APP_META = {
  claude: { label: 'Claude', className: 'panel-record-avatar-app-claude' },
  codex: { label: 'Codex', className: 'panel-record-avatar-app-codex' },
  opencode: { label: 'OpenCode', className: 'panel-record-avatar-app-opencode' },
  openclaw: { label: 'OpenClaw', className: 'panel-record-avatar-app-openclaw' },
};
const ADVANCED_PROXY_APP_ORDER = ['claude', 'codex', 'opencode', 'openclaw'];

const records = ref([]);
const advancedProxyConfigSnapshot = ref(getAdvancedProxyLocalSnapshot());
const contextMap = ref(new Map());
const advancedProxyTakeoverMap = ref(getAdvancedProxyTakeoverMap());
const advancedProxyRoutingSnapshot = ref({ apps: {}, providers: {} });
const advancedProxyDispatchClock = ref(Date.now());
const activePopoverRowKey = ref('');
const activeModelDropdownRowKey = ref('');
const panelBodyRef = ref(null);
const panelQueueStripRef = ref(null);
const panelShellRef = ref(null);
const panelScrollRatio = ref(0);
const hasScrollableContent = ref(false);
const superMiniMode = ref(false);
const superMiniQueueCardHintSeenCount = ref(0);
const PANEL_PERSIST_DEBOUNCE_MS = 120;

let panelBodyResizeObserver = null;
let panelPersistTimer = null;
let advancedProxyRoutingTimer = null;
let advancedProxyDispatchTimer = null;
let lastSidebarRoutingLogAt = 0;
let superMiniQueueScrollLeft = 0;
let superMiniQueueCardHintHoverStartedAt = 0;
let superMiniWindowBounds = null;
let superMiniRestoreBounds = null;
let superMiniTransitionToken = 0;
let superMiniWindowDragLogAt = 0;
let panelQueueDragLogAt = 0;
const superMiniWindowDragState = {
  active: false,
  pointerId: -1,
  offsetX: 0,
  offsetY: 0,
  startWindowX: 0,
  startWindowY: 0,
  lastScreenX: 0,
  lastScreenY: 0,
  rafId: 0,
  pendingWindowX: 0,
  pendingWindowY: 0,
  moved: false,
};
const panelQueueDragState = {
  active: false,
  armed: false,
  pointerId: -1,
  startX: 0,
  startScrollLeft: 0,
  moved: false,
};

function syncSuperMiniBodyClass() {
  if (typeof document === 'undefined') return;
  document.body.classList.toggle('panel-super-mini-mode', superMiniMode.value);
}

function readSuperMiniQueueCardHintSeenCount() {
  if (typeof window === 'undefined') return 0;
  try {
    const raw = window.localStorage.getItem(SUPER_MINI_QUEUE_CARD_HINT_STORAGE_KEY);
    const parsed = Number.parseInt(String(raw || '0'), 10);
    return Number.isFinite(parsed) && parsed > 0 ? Math.min(parsed, SUPER_MINI_QUEUE_CARD_HINT_LIMIT) : 0;
  } catch {
    return 0;
  }
}

function persistSuperMiniQueueCardHintSeenCount(value) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(
      SUPER_MINI_QUEUE_CARD_HINT_STORAGE_KEY,
      String(Math.min(Math.max(0, Number(value || 0)), SUPER_MINI_QUEUE_CARD_HINT_LIMIT)),
    );
  } catch {}
}

function beginSuperMiniQueueCardHintHover() {
  if (superMiniQueueCardHintSeenCount.value >= SUPER_MINI_QUEUE_CARD_HINT_LIMIT) return;
  superMiniQueueCardHintHoverStartedAt = Date.now();
}

function commitSuperMiniQueueCardHintSeen() {
  if (superMiniQueueCardHintSeenCount.value >= SUPER_MINI_QUEUE_CARD_HINT_LIMIT) return;
  if (!superMiniQueueCardHintHoverStartedAt) return;
  const elapsed = Date.now() - superMiniQueueCardHintHoverStartedAt;
  superMiniQueueCardHintHoverStartedAt = 0;
  if (elapsed < SUPER_MINI_QUEUE_CARD_HINT_MIN_HOVER_MS) return;
  const current = Math.min(
    SUPER_MINI_QUEUE_CARD_HINT_LIMIT,
    Math.max(0, Number(superMiniQueueCardHintSeenCount.value || 0)),
  );
  if (current >= SUPER_MINI_QUEUE_CARD_HINT_LIMIT) return;
  const next = current + 1;
  superMiniQueueCardHintSeenCount.value = next;
  persistSuperMiniQueueCardHintSeenCount(next);
}

function appendPanelClientLog(scope, message) {
  if (typeof window === 'undefined') return;
  if (!AppendClientLog) return;
  try {
    void AppendClientLog(scope, message);
  } catch {}
}

function formatSidebarBounds(bounds) {
  if (!bounds) return 'null';
  return `x=${Math.round(Number(bounds.x || 0))},y=${Math.round(Number(bounds.y || 0))},w=${Math.round(Number(bounds.width || 0))},h=${Math.round(Number(bounds.height || 0))}`;
}

function normalizeSidebarWindowBounds(bounds) {
  if (!bounds) return null;
  return {
    width: Number(bounds?.width ?? bounds?.Width ?? 0),
    height: Number(bounds?.height ?? bounds?.Height ?? 0),
    x: Number(bounds?.x ?? bounds?.X ?? 0),
    y: Number(bounds?.y ?? bounds?.Y ?? 0),
  };
}

function isValidSidebarWindowBounds(bounds) {
  return Boolean(bounds && Number(bounds.width) > 0 && Number(bounds.height) > 0);
}

async function readSidebarWindowBounds() {
  try {
    const backendBounds = await GetPanelWindowBounds().catch(() => null);
    const normalizedBackendBounds = normalizeSidebarWindowBounds(backendBounds);
    if (isValidSidebarWindowBounds(normalizedBackendBounds)) {
      return { bounds: normalizedBackendBounds, source: 'backend' };
    }
  } catch {}

  try {
    const [size, position] = await Promise.all([
      WindowGetSize().catch(() => null),
      WindowGetPosition().catch(() => null),
    ]);
    const fallbackBounds = normalizeSidebarWindowBounds({
      width: size?.w,
      height: size?.h,
      x: position?.x,
      y: position?.y,
    });
    if (isValidSidebarWindowBounds(fallbackBounds)) {
      return { bounds: fallbackBounds, source: 'runtime' };
    }
  } catch {}

  return { bounds: null, source: 'none' };
}

async function logSuperMiniWindowSnapshot(stage, token, extra = '') {
  try {
    const current = await readSidebarWindowBounds();
    const shellRect = panelShellRef.value?.getBoundingClientRect?.();
    const shellSize = shellRect ? `${Math.ceil(Number(shellRect.width || 0))}x${Math.ceil(Number(shellRect.height || 0))}` : 'null';
    const suffix = String(extra || '').trim();
    appendPanelClientLog(
      'panel.super-mini',
      `${stage} token=${token} enabled=${superMiniMode.value} source=${current.source} window=${formatSidebarBounds(current.bounds)} shell=${shellSize}${suffix ? ` ${suffix}` : ''}`,
    );
  } catch (error) {
    appendPanelClientLog(
      'panel.super-mini',
      `${stage} token=${token} snapshot failed=${error?.message || String(error)}`,
    );
  }
}

async function applySuperMiniWindowMode(enabled) {
  const transitionToken = ++superMiniTransitionToken;
  appendPanelClientLog(
    'panel.super-mini',
    `toggle request enabled=${Boolean(enabled)} token=${transitionToken} current=${superMiniMode.value} queueScroll=${panelQueueStripRef.value?.scrollLeft || 0}`,
  );
  if (typeof window === 'undefined') {
    superMiniMode.value = Boolean(enabled);
    syncSuperMiniBodyClass();
    return;
  }

  const queueStrip = panelQueueStripRef.value;
  if (enabled && queueStrip) {
    superMiniQueueScrollLeft = queueStrip.scrollLeft || 0;
  }

  if (enabled) {
    try {
      const current = await readSidebarWindowBounds();
      const restoreBounds = current.bounds;
      if (isValidSidebarWindowBounds(restoreBounds)) {
        superMiniRestoreBounds = restoreBounds;
        superMiniWindowBounds = {
          ...restoreBounds,
          width: Math.max(1, Math.round(restoreBounds.width)),
          height: Math.max(1, Math.round(restoreBounds.height)),
        };
        appendPanelClientLog(
          'panel.super-mini',
          `captured restore bounds token=${transitionToken} source=${current.source} bounds=${formatSidebarBounds(restoreBounds)}`,
        );
      } else {
        appendPanelClientLog(
          'panel.super-mini',
          `capture skipped token=${transitionToken} source=${current.source}`,
        );
      }
    } catch {}
  }

  if (transitionToken !== superMiniTransitionToken) {
    appendPanelClientLog('panel.super-mini', `toggle aborted by newer request token=${transitionToken}`);
    return;
  }

  superMiniMode.value = Boolean(enabled);
  syncSuperMiniBodyClass();
  appendPanelClientLog('panel.super-mini', `mode applied enabled=${Boolean(enabled)} token=${transitionToken}`);
  await logSuperMiniWindowSnapshot('mode applied snapshot', transitionToken, `step=${enabled ? 'enable' : 'disable'}`);

  if (queueStrip) {
    await nextTick();
    if (transitionToken !== superMiniTransitionToken) {
      appendPanelClientLog('panel.super-mini', `queue scroll aborted by newer request token=${transitionToken}`);
      return;
    }
    queueStrip.scrollLeft = enabled ? 0 : superMiniQueueScrollLeft;
    appendPanelClientLog(
      'panel.super-mini',
      `queue scroll applied enabled=${Boolean(enabled)} token=${transitionToken} value=${queueStrip.scrollLeft || 0}`,
    );
  }

  if (enabled) {
    await nextTick();
    if (transitionToken !== superMiniTransitionToken) {
      appendPanelClientLog('panel.super-mini', `enable stage aborted after nextTick token=${transitionToken}`);
      return;
    }
    await new Promise(resolve => window.requestAnimationFrame(() => resolve()));
    if (transitionToken !== superMiniTransitionToken) {
      appendPanelClientLog('panel.super-mini', `enable stage aborted after raf token=${transitionToken}`);
      return;
    }
    const shellRect = panelShellRef.value?.getBoundingClientRect?.();
    const shellWidth = Math.ceil(Number(shellRect?.width || 0));
    const shellHeight = Math.ceil(Number(shellRect?.height || 0));
    const width = Math.round((shellWidth || 100) * SUPER_MINI_SCALE);
    const height = Math.round((shellHeight || 100) * SUPER_MINI_SCALE * 0.9);
    const cardScale = shellWidth > 0 ? width / shellWidth : SUPER_MINI_SCALE;
    appendPanelClientLog(
      'panel.super-mini',
      `enable sizing token=${transitionToken} shell=${shellWidth}x${shellHeight} target=${width}x${height} scale=${cardScale.toFixed(4)}`,
    );
    const enableBounds = superMiniRestoreBounds || superMiniWindowBounds;
    const dockState = String(await GetPanelDockState().catch(() => '')).trim();
    const enableTargetX = isValidSidebarWindowBounds(enableBounds)
      ? (dockState === 'right'
        ? Math.round(enableBounds.x + enableBounds.width - width)
        : Math.round(enableBounds.x))
      : null;
    const enableTargetY = isValidSidebarWindowBounds(enableBounds)
      ? Math.round(enableBounds.y)
      : null;
    if (panelShellRef.value) {
      panelShellRef.value.style.setProperty('--super-mini-card-width', `${width}px`);
      panelShellRef.value.style.setProperty('--super-mini-card-height', `${height}px`);
      panelShellRef.value.style.setProperty('--super-mini-card-scale', `${cardScale}`);
    }
    try {
      if (enableTargetX !== null && enableTargetY !== null) {
        WindowSetPosition(enableTargetX, enableTargetY);
        appendPanelClientLog(
          'panel.super-mini',
          `enable position set token=${transitionToken} dock=${dockState || 'unknown'} pos=${enableTargetX},${enableTargetY}`,
        );
      }
      WindowSetSize(width, height);
      appendPanelClientLog('panel.super-mini', `window size set token=${transitionToken} size=${width}x${height}`);
      await logSuperMiniWindowSnapshot('after resize', transitionToken, `target=${width}x${height}`);
    } catch {}
    return;
  }

  const restoreBounds = isValidSidebarWindowBounds(superMiniRestoreBounds)
    ? superMiniRestoreBounds
    : isValidSidebarWindowBounds(superMiniWindowBounds)
      ? superMiniWindowBounds
      : null;
  appendPanelClientLog(
    'panel.super-mini',
    `restore sizing enabled=${Boolean(enabled)} token=${transitionToken} bounds=${formatSidebarBounds(restoreBounds)}`,
  );
  try {
    if (restoreBounds) {
      WindowSetPosition(
        Math.round(restoreBounds.x),
        Math.round(restoreBounds.y),
      );
      WindowSetSize(
        Math.max(1, Math.round(restoreBounds.width)),
        Math.max(1, Math.round(restoreBounds.height)),
      );
    }
    await logSuperMiniWindowSnapshot('after restore', transitionToken, `restored=${formatSidebarBounds(restoreBounds)}`);
  } catch {}
  if (!enabled) {
    superMiniQueueScrollLeft = 0;
    appendPanelClientLog('panel.super-mini', `queue scroll reset token=${transitionToken}`);
  }
  if (panelShellRef.value) {
    panelShellRef.value.style.removeProperty('--super-mini-card-width');
    panelShellRef.value.style.removeProperty('--super-mini-card-height');
    panelShellRef.value.style.removeProperty('--super-mini-card-scale');
  }
  superMiniWindowBounds = null;
  superMiniRestoreBounds = null;
  appendPanelClientLog('panel.super-mini', `transition cleared token=${transitionToken}`);
}

function toggleSuperMiniMode() {
  appendPanelClientLog(
    'panel.super-mini',
    `toggle clicked next=${!superMiniMode.value} restore=${formatSidebarBounds(superMiniRestoreBounds)} current=${formatSidebarBounds(superMiniWindowBounds)}`,
  );
  void applySuperMiniWindowMode(!superMiniMode.value);
}

async function beginSuperMiniWindowDrag(event) {
  if (!superMiniMode.value || event?.button !== 0) {
    appendPanelClientLog(
      'panel.super-mini.drag',
      `ignored pointerdown mode=${superMiniMode.value} button=${Number(event?.button ?? -1)} pointer=${Number(event?.pointerId ?? -1)}`,
    );
    return;
  }
  const target = event.currentTarget;
  if (!target) return;
  event.preventDefault?.();
  const baseBounds = isValidSidebarWindowBounds(superMiniWindowBounds)
    ? superMiniWindowBounds
    : isValidSidebarWindowBounds(superMiniRestoreBounds)
      ? superMiniRestoreBounds
      : null;
  if (!baseBounds) {
    appendPanelClientLog(
      'panel.super-mini.drag',
      `ignored pointerdown missing bounds pointer=${Number(event?.pointerId ?? -1)}`,
    );
    return;
  }
  const screenX = Number.isFinite(Number(event?.screenX)) ? Number(event.screenX) : Number(event?.clientX || 0) + Number(window?.screenX || 0);
  const screenY = Number.isFinite(Number(event?.screenY)) ? Number(event.screenY) : Number(event?.clientY || 0) + Number(window?.screenY || 0);
  superMiniWindowDragState.active = true;
  superMiniWindowDragState.pointerId = event.pointerId;
  superMiniWindowDragState.startWindowX = Number(baseBounds.x || 0);
  superMiniWindowDragState.startWindowY = Number(baseBounds.y || 0);
  superMiniWindowDragState.offsetX = screenX - superMiniWindowDragState.startWindowX;
  superMiniWindowDragState.offsetY = screenY - superMiniWindowDragState.startWindowY;
  superMiniWindowDragState.lastScreenX = screenX;
  superMiniWindowDragState.lastScreenY = screenY;
  superMiniWindowDragState.pendingWindowX = superMiniWindowDragState.startWindowX;
  superMiniWindowDragState.pendingWindowY = superMiniWindowDragState.startWindowY;
  superMiniWindowDragState.moved = false;
  superMiniWindowDragLogAt = 0;
  appendPanelClientLog(
    'panel.super-mini.drag',
    `start pointer=${event.pointerId} screen=${screenX},${screenY} window=${superMiniWindowDragState.startWindowX},${superMiniWindowDragState.startWindowY} offset=${superMiniWindowDragState.offsetX},${superMiniWindowDragState.offsetY}`,
  );
  try {
    target.setPointerCapture?.(event.pointerId);
  } catch {}
}

function flushSuperMiniWindowDrag(force = false) {
  if (!force && !superMiniWindowDragState.active) return;
  try {
    WindowSetPosition(
      superMiniWindowDragState.pendingWindowX,
      superMiniWindowDragState.pendingWindowY,
    );
    superMiniWindowBounds = {
      width: Math.max(1, Math.round(superMiniWindowBounds?.width ?? superMiniRestoreBounds?.width ?? 1)),
      height: Math.max(1, Math.round(superMiniWindowBounds?.height ?? superMiniRestoreBounds?.height ?? 1)),
      x: superMiniWindowDragState.pendingWindowX,
      y: superMiniWindowDragState.pendingWindowY,
    };
  } catch {}
  superMiniWindowDragState.rafId = 0;
}

function dragSuperMiniWindow(event) {
  if (!superMiniWindowDragState.active || superMiniWindowDragState.pointerId !== event.pointerId) return;
  const screenX = Number.isFinite(Number(event?.screenX)) ? Number(event.screenX) : Number(event?.clientX || 0) + Number(window?.screenX || 0);
  const screenY = Number.isFinite(Number(event?.screenY)) ? Number(event.screenY) : Number(event?.clientY || 0) + Number(window?.screenY || 0);
  superMiniWindowDragState.lastScreenX = screenX;
  superMiniWindowDragState.lastScreenY = screenY;
  const nextX = Math.max(0, Math.round(screenX - superMiniWindowDragState.offsetX));
  const nextY = Math.max(0, Math.round(screenY - superMiniWindowDragState.offsetY));
  if (nextX !== superMiniWindowDragState.startWindowX || nextY !== superMiniWindowDragState.startWindowY) {
    superMiniWindowDragState.moved = true;
  }
  superMiniWindowDragState.pendingWindowX = nextX;
  superMiniWindowDragState.pendingWindowY = nextY;
  const now = Date.now();
  if (!superMiniWindowDragLogAt || now - superMiniWindowDragLogAt >= 180) {
    superMiniWindowDragLogAt = now;
    appendPanelClientLog(
      'panel.super-mini.drag',
      `move pointer=${event.pointerId} screen=${screenX},${screenY} pending=${superMiniWindowDragState.pendingWindowX},${superMiniWindowDragState.pendingWindowY}`,
    );
  }
  if (superMiniWindowDragState.rafId) {
    window.cancelAnimationFrame(superMiniWindowDragState.rafId);
    superMiniWindowDragState.rafId = 0;
  }
  flushSuperMiniWindowDrag(true);
}

function endSuperMiniWindowDrag(event) {
  const target = event?.currentTarget || event?.target;
  const wasActive = superMiniWindowDragState.active && superMiniWindowDragState.pointerId === event?.pointerId;
  if (target && superMiniWindowDragState.pointerId === event?.pointerId) {
    try {
      target.releasePointerCapture?.(event.pointerId);
    } catch {}
  }
  superMiniWindowDragState.active = false;
  superMiniWindowDragState.pointerId = -1;
  superMiniWindowDragState.startX = 0;
  superMiniWindowDragState.startY = 0;
  superMiniWindowDragState.startWindowX = 0;
  superMiniWindowDragState.startWindowY = 0;
  if (superMiniWindowDragState.rafId) {
    window.cancelAnimationFrame(superMiniWindowDragState.rafId);
    superMiniWindowDragState.rafId = 0;
  }
  if (wasActive) {
    if (superMiniWindowBounds) {
      superMiniWindowBounds = {
        ...superMiniWindowBounds,
        x: Math.round(superMiniWindowDragState.pendingWindowX ?? superMiniWindowBounds.x ?? 0),
        y: Math.round(superMiniWindowDragState.pendingWindowY ?? superMiniWindowBounds.y ?? 0),
      };
    }
    flushSuperMiniWindowDrag(true);
    superMiniWindowDragState.pendingWindowX = 0;
    superMiniWindowDragState.pendingWindowY = 0;
  }
  appendPanelClientLog(
    'panel.super-mini.drag',
    `end pointer=${event?.pointerId ?? -1} active=${wasActive} moved=${superMiniWindowDragState.moved} last=${superMiniWindowDragState.lastScreenX},${superMiniWindowDragState.lastScreenY} final=${superMiniWindowBounds ? formatSidebarBounds(superMiniWindowBounds) : 'null'} current=${formatSidebarBounds(superMiniRestoreBounds)}`,
  );
  superMiniWindowDragState.moved = false;
  superMiniWindowDragState.offsetX = 0;
  superMiniWindowDragState.offsetY = 0;
  superMiniWindowDragState.lastScreenX = 0;
  superMiniWindowDragState.lastScreenY = 0;
}

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
  reloadAdvancedProxyConfigState();
  reloadAdvancedProxyTakeoverState();
  void nextTick(syncScrollIndicator);
}

function reloadAdvancedProxyTakeoverState(event = null) {
  const takeoverMap = event?.detail?.takeoverMap;
  advancedProxyTakeoverMap.value = takeoverMap && typeof takeoverMap === 'object'
    ? takeoverMap
    : getAdvancedProxyTakeoverMap();
}

function reloadAdvancedProxyConfigState(event = null) {
  const config = event?.detail?.config;
  advancedProxyConfigSnapshot.value = config && typeof config === 'object'
    ? config
    : getAdvancedProxyLocalSnapshot();
}

async function reloadAdvancedProxyRoutingState() {
  try {
    const snapshot = await getAdvancedProxyRoutingSnapshot();
    advancedProxyRoutingSnapshot.value = snapshot && typeof snapshot === 'object'
      ? snapshot
      : getAdvancedProxyRoutingLocalSnapshot();
    advancedProxyDispatchClock.value = Date.now();
    emitSidebarRoutingDiagnostics();
  } catch {
    advancedProxyRoutingSnapshot.value = getAdvancedProxyRoutingLocalSnapshot();
    advancedProxyDispatchClock.value = Date.now();
    emitSidebarRoutingDiagnostics();
  }
}

function buildAdvancedProxyHitSummaryText(record) {
  const siteName = String(record?.siteName || '').trim() || 'Provider';
  const modelLabel = String(
    record?.selectedModel ||
    record?.quickTestModel ||
    record?.model ||
    '未设置模型'
  ).trim() || '未设置模型';
  const performanceMetrics = extractPerformanceMetrics(record || {});
  const latencyText = performanceMetrics.latencySeconds != null
    ? `${performanceMetrics.latencySeconds.toFixed(2)}s`
    : '-';
  const ttftText = performanceMetrics.ttftMs != null
    ? `${Math.round(performanceMetrics.ttftMs)}ms`
    : '-';
  const tpsText = performanceMetrics.tps != null
    ? performanceMetrics.tps.toFixed(2)
    : '-';
  return `最近命中：${siteName} ${modelLabel} + Latency ${latencyText} + TTFT ${ttftText} + TPS ${tpsText}`;
}

function getAdvancedProxyDispatchVisualState(record) {
  const routeStates = record ? getAdvancedProxyProviderRouteStatesForRecord(record) : [];
  const activeRoute = routeStates.find(item => item.isActive) || null;
  const recentRoute = routeStates.find(item => item.isRecent) || null;
  const now = Number(advancedProxyDispatchClock.value) || Date.now();
  const currentRoute = activeRoute || recentRoute;
  if (!currentRoute) {
    return {
      visible: false,
      active: false,
      fading: false,
      opacity: 0,
      labelText: '',
      summaryText: '',
    };
  }

  const sourceUpdatedAtMs = Number(currentRoute.updatedAtMs || 0);
  const expireAtMs = (Number.isFinite(sourceUpdatedAtMs) && sourceUpdatedAtMs > 0
    ? sourceUpdatedAtMs
    : now) + ADVANCED_PROXY_DISPATCH_HOLD_MS;

  if (!activeRoute && now > expireAtMs) {
    return {
      visible: false,
      active: false,
      fading: false,
      opacity: 0,
      labelText: '',
      summaryText: '',
    };
  }

  const remainingMs = Math.max(0, expireAtMs - now);
  const opacity = activeRoute
    ? 1
    : Math.max(0, Math.min(1, remainingMs / ADVANCED_PROXY_DISPATCH_FADE_MS));

  return {
    visible: true,
    active: Boolean(activeRoute),
    fading: !activeRoute && remainingMs <= ADVANCED_PROXY_DISPATCH_FADE_MS,
    opacity,
    labelText: currentRoute.appTypeLabelText || '',
    summaryText: buildAdvancedProxyHitSummaryText(record),
  };
}

function emitSidebarRoutingDiagnostics() {
  const snapshotApps = advancedProxyRoutingSnapshot.value?.apps || {};
  const now = Date.now();
  if (now - lastSidebarRoutingLogAt < 10000) return;
  lastSidebarRoutingLogAt = now;

  const appSummaries = Object.entries(snapshotApps).map(([appId, routeState]) => ({
    appId,
    providerId: String(routeState?.providerId || '').trim(),
    providerRowKey: String(routeState?.providerRowKey || '').trim(),
    providerName: String(routeState?.providerName || '').trim(),
    targetUrl: String(routeState?.targetUrl || '').trim(),
    status: String(routeState?.status || '').trim(),
    updatedAt: String(routeState?.updatedAt || '').trim(),
  }));

  const recordSummaries = visibleRecords.value.map(record => {
    const siteName = String(record?.siteName || '').trim();
    const siteUrl = String(record?.siteUrl || '').trim();
    const rowKey = String(record?.rowKey || '').trim();
    const matches = Object.entries(snapshotApps).map(([appId, routeState]) => ({
      appId,
      match: doesRouteStateMatchRecord(record, routeState),
      providerId: String(routeState?.providerId || '').trim(),
      providerRowKey: String(routeState?.providerRowKey || '').trim(),
      providerName: String(routeState?.providerName || '').trim(),
      targetUrl: String(routeState?.targetUrl || '').trim(),
    }));
    return {
      siteName,
      siteUrl,
      rowKey,
      takeoverApps: getAdvancedProxyAppsForRecord(record),
      matchedApps: matches.filter(item => item.match).map(item => item.appId),
      matches,
    };
  });

  logClientDiagnostic('sidebar.routing', JSON.stringify({
    snapshotApps: appSummaries,
    visibleRecords: recordSummaries,
  }));
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

function normalizeComparableSiteUrl(value) {
  return String(value || '').trim().replace(/\/+$/, '').toLowerCase();
}

function normalizeComparableName(value) {
  return String(value || '').trim().toLowerCase();
}

function doesRouteStateMatchRecord(record, routeState) {
  const rowKey = String(record?.rowKey || '').trim();
  if (!rowKey || !routeState || typeof routeState !== 'object') return false;

  const matchedProviderKey = String(routeState?.providerRowKey || routeState?.providerId || '').trim();
  if (matchedProviderKey && matchedProviderKey === rowKey) {
    return true;
  }

  const recordSiteUrl = normalizeComparableSiteUrl(record?.siteUrl);
  const targetUrl = normalizeComparableSiteUrl(routeState?.targetUrl);
  if (!recordSiteUrl || !targetUrl) {
    const providerName = normalizeComparableName(routeState?.providerName);
    const siteName = normalizeComparableName(record?.siteName);
    return Boolean(providerName && siteName && providerName === siteName);
  }
  if (targetUrl === recordSiteUrl || targetUrl.startsWith(`${recordSiteUrl}/`)) {
    return true;
  }

  const providerName = normalizeComparableName(routeState?.providerName);
  const siteName = normalizeComparableName(record?.siteName);
  if (providerName && siteName && providerName === siteName) {
    return true;
  }

  return false;
}

function normalizeAdvancedProxyRouteState(record, appId) {
  const snapshot = advancedProxyRoutingSnapshot.value?.apps || {};
  const routeState = snapshot?.[appId];
  if (!routeState || typeof routeState !== 'object') return null;
  if (!doesRouteStateMatchRecord(record, routeState)) return null;

  const updatedAt = String(routeState?.updatedAt || '').trim();
  const updatedAtMs = updatedAt ? Date.parse(updatedAt) : NaN;
  const ageMs = Number.isFinite(updatedAtMs) ? Math.max(0, Date.now() - updatedAtMs) : Number.POSITIVE_INFINITY;
  const status = String(routeState?.status || '').trim().toLowerCase();
  const isActive = ageMs <= ADVANCED_PROXY_ACTIVE_ROUTE_WINDOW_MS || ageMs <= ADVANCED_PROXY_RECENT_ROUTE_WINDOW_MS;
  const isRecent = !isActive && ageMs <= ADVANCED_PROXY_RECENT_ROUTE_WINDOW_MS;
  if (!isActive && !isRecent) return null;

  return {
    appId,
    appLabel: ADVANCED_PROXY_APP_META[appId]?.label || appId,
    status,
    routeKind: String(routeState?.routeKind || '').trim(),
    providerName: String(routeState?.providerName || '').trim(),
    targetUrl: String(routeState?.targetUrl || '').trim(),
    isActive,
    isRecent,
  };
}

function getAdvancedProxyRouteStatesForRecord(record) {
  return ADVANCED_PROXY_APP_ORDER
    .map(appId => normalizeAdvancedProxyRouteState(record, appId))
    .filter(Boolean)
    .sort((left, right) => {
      if (left.isActive !== right.isActive) {
        return left.isActive ? -1 : 1;
      }
      return ADVANCED_PROXY_APP_ORDER.indexOf(left.appId) - ADVANCED_PROXY_APP_ORDER.indexOf(right.appId);
    });
}

function getPrimaryAdvancedProxyVisualApp(record) {
  const currentState = getAdvancedProxyProviderRouteStatesForRecord(record).find(item => item.isActive || item.isRecent);
  if (currentState?.appTypeIds?.[0]) return currentState.appTypeIds[0];
  return getPrimaryAdvancedProxyApp(record);
}

function matchesAdvancedProxyProviderRoute(record, routeState) {
  if (!record || !routeState || typeof routeState !== 'object') return false;
  const rowKey = String(record?.rowKey || '').trim();
  const providerRowKey = String(routeState?.providerRowKey || '').trim();
  if (rowKey && providerRowKey && rowKey === providerRowKey) {
    return true;
  }

  const siteName = String(record?.siteName || '').trim().toLowerCase();
  const providerName = String(routeState?.providerName || '').trim().toLowerCase();
  if (siteName && providerName && siteName === providerName) {
    return true;
  }

  const siteUrl = String(record?.siteUrl || '').trim().replace(/\/+$/, '').toLowerCase();
  const targetUrl = String(routeState?.targetUrl || '').trim().replace(/\/+$/, '').toLowerCase();
  if (siteUrl && targetUrl && (targetUrl === siteUrl || targetUrl.startsWith(`${siteUrl}/`))) {
    return true;
  }

  return false;
}

function normalizeAdvancedProxyProviderRouteState(record, providerKey, routeState) {
  if (!matchesAdvancedProxyProviderRoute(record, routeState)) return null;

  const updatedAt = String(routeState?.updatedAt || '').trim();
  const updatedAtMs = updatedAt ? Date.parse(updatedAt) : NaN;
  const ageMs = Number.isFinite(updatedAtMs) ? Math.max(0, Date.now() - updatedAtMs) : Number.POSITIVE_INFINITY;
  const activeCount = Math.max(0, Number(routeState?.activeCount || 0));
  const appTypeIds = Array.isArray(routeState?.appTypes)
    ? routeState.appTypes.map(appId => String(appId || '').trim()).filter(Boolean)
    : [];
  const appTypeLabels = appTypeIds.map(appId => ADVANCED_PROXY_APP_META[appId]?.label || appId);
  const status = String(routeState?.status || '').trim().toLowerCase();
  const isActive = activeCount > 0 || status === 'dispatching';
  const isRecent = !isActive && ageMs <= ADVANCED_PROXY_RECENT_ROUTE_WINDOW_MS;
  if (!isActive && !isRecent) return null;

  return {
    providerKey: String(providerKey || '').trim(),
    providerRowKey: String(routeState?.providerRowKey || '').trim(),
    providerId: String(routeState?.providerId || '').trim(),
    providerName: String(routeState?.providerName || '').trim(),
    appTypeIds,
    appTypeLabels,
    appTypeLabelText: appTypeLabels.join(' / '),
    status,
    routeKind: String(routeState?.routeKind || '').trim(),
    targetUrl: String(routeState?.targetUrl || '').trim(),
    updatedAt,
    updatedAtMs: Number.isFinite(updatedAtMs) ? updatedAtMs : 0,
    isActive,
    isRecent,
  };
}

function getAdvancedProxyProviderRouteStatesForRecord(record) {
  const snapshotProviders = advancedProxyRoutingSnapshot.value?.providers;
  if (!snapshotProviders || typeof snapshotProviders !== 'object') return [];

  return Object.entries(snapshotProviders)
    .map(([providerKey, routeState]) => normalizeAdvancedProxyProviderRouteState(record, providerKey, routeState))
    .filter(Boolean)
    .sort((left, right) => {
      if (left.isActive !== right.isActive) {
        return left.isActive ? -1 : 1;
      }
      return left.providerKey.localeCompare(right.providerKey);
    });
}

const advancedProxyQueueProviders = computed(() =>
  getAdvancedProxyQueueProviders(advancedProxyConfigSnapshot.value, ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE, {
    effective: false,
    enabledOnly: true,
  })
);

const advancedProxyQueueTitle = computed(() => 'Proxy Providers');

const advancedProxyQueueDescription = computed(() => {
  const enabledAppLabels = Object.entries(advancedProxyConfigSnapshot.value?.queues || {})
    .filter(([scope, section]) => scope !== ADVANCED_PROXY_GLOBAL_QUEUE_SCOPE && section?.inheritGlobal !== true && Array.isArray(section?.providers) && section.providers.length > 0)
    .map(([scope]) => String(ADVANCED_PROXY_APP_META?.[scope]?.label || scope).trim())
    .filter(Boolean);

  if (!advancedProxyConfigSnapshot.value?.enabled) {
    return '高级代理尚未开启。开启总开关后，这里会显示当前可调度的上游 Provider 队列。';
  }

  if (advancedProxyQueueProviders.value.length === 0) {
    return '当前全局队列还没有可用 Provider。点击下面的卡片加入队列后，这里会自动出现。';
  }

  const inheritanceText = enabledAppLabels.length > 0
    ? `，${enabledAppLabels.join(' / ')} 的独立队列会优先按自己的配置调度`
    : '，未覆盖应用默认继承全局';
  return `全局队列启用 ${advancedProxyQueueProviders.value.length} 条${inheritanceText}`;
});

const advancedProxyQueueTone = computed(() => {
  if (!advancedProxyConfigSnapshot.value?.enabled) return 'red';
  if (advancedProxyQueueProviders.value.length === 0) return 'red';

  const hasFailed = advancedProxyQueueItems.value.some(item => item.hasFailedRoute);
  if (hasFailed) return 'red';

  const hasActive = advancedProxyQueueItems.value.some(item => item.hasActiveRoute);
  if (hasActive) return 'green';

  const hasRecent = advancedProxyQueueItems.value.some(item => item.hasRecentRoute);
  if (hasRecent) return 'yellow';

  return 'yellow';
});

const advancedProxyQueueItems = computed(() => {
  const orderMap = advancedProxyQueueOrderByApiKey.value;
  const recordMap = new Map(
    records.value.map(record => [String(record?.apiKey || '').trim(), record])
  );

  return advancedProxyQueueProviders.value.map((provider, index) => {
    const rowKey = String(provider?.rowKey || provider?.id || '').trim();
    const skKey = String(provider?.apiKey || '').trim();
    const record = (skKey && recordMap.get(skKey)) || null;
    const appIds = record ? getAdvancedProxyAppsForRecord(record) : [];
    const routeStates = record ? getAdvancedProxyProviderRouteStatesForRecord(record) : [];
    const activeRoute = routeStates.find(item => item.isActive) || null;
    const recentRoute = routeStates.find(item => item.isRecent) || null;
    const failedRoute = routeStates.some(item => item.status === 'failed');
    const siteName = String(record?.siteName || provider?.name || 'Provider').trim() || 'Provider';
    const modelLabel = String(
      record?.selectedModel ||
      record?.quickTestModel ||
      provider?.model ||
      '未设置模型'
    ).trim() || '未设置模型';
    const apiKey = String(record?.apiKey || provider?.apiKey || '').trim();
    const endpoint = String(record?.siteUrl || provider?.baseUrl || '').trim();
    const queueScopeText = appIds.length > 0
      ? appIds.map(appId => ADVANCED_PROXY_APP_META[appId]?.label || appId).join(' / ')
      : '全局继承';
    const order = skKey ? orderMap.get(skKey) || null : null;
    const dispatchSource = record || {
      rowKey,
      siteName,
      siteUrl: endpoint,
      apiKey: skKey,
      selectedModel: modelLabel,
      model: modelLabel,
    };
    const dispatchState = getAdvancedProxyDispatchVisualState(dispatchSource);
    const dispatchLabel = dispatchState.labelText || activeRoute?.appTypeLabelText || '';
    const tooltipLines = [
      siteName,
      modelLabel,
      queueScopeText !== '全局继承' ? queueScopeText : '',
      endpoint,
      apiKey,
    ].map(text => String(text || '').trim()).filter(Boolean);

    return {
      id: rowKey || `provider-${index}`,
      order,
      siteName,
      modelLabel,
      apiKey,
      skKey,
      endpoint,
      avatar: getSiteEmoji(siteName),
      queueScopeText,
      hasActiveRoute: dispatchState.active,
      hasRecentRoute: Boolean(recentRoute),
      hasFailedRoute: Boolean(failedRoute),
      hasDispatchingRoute: dispatchState.visible,
      dispatchState,
      activeRouteLabel: dispatchLabel,
      recentRouteLabel: recentRoute?.appTypeLabelText || '',
      dispatchLabel,
      hasTooltip: tooltipLines.length > 0,
      hitSummaryText: dispatchState.summaryText,
    };
  });
});

const advancedProxyQueueOrderByApiKey = computed(() => {
  const orderMap = new Map();
  visibleRecords.value.forEach((record, index) => {
    const skKey = String(record?.apiKey || '').trim();
    if (skKey && !orderMap.has(skKey)) {
      orderMap.set(skKey, index + 1);
    }
  });
  return orderMap;
});

const showAdvancedProxyQueueCard = computed(() =>
  advancedProxyConfigSnapshot.value?.enabled === true || advancedProxyQueueProviders.value.length > 0
);

function getAdvancedProxyAvatarClass(record) {
  const appId = getPrimaryAdvancedProxyVisualApp(record);
  if (!appId) return '';
  const className = ADVANCED_PROXY_APP_META[appId]?.className;
  const routeStates = getAdvancedProxyProviderRouteStatesForRecord(record);
  const hasActiveRoute = routeStates.some(item => item.isActive);
  const hasRecentRoute = !hasActiveRoute && routeStates.some(item => item.isRecent);
  return [
    'panel-record-avatar-takeover',
    className,
    hasActiveRoute ? 'panel-record-avatar-routing-active' : '',
    hasRecentRoute ? 'panel-record-avatar-routing-recent' : '',
  ].filter(Boolean);
}

function getAdvancedProxyCardStateClass(record) {
  const routeStates = getAdvancedProxyProviderRouteStatesForRecord(record);
  if (routeStates.some(item => item.isActive)) {
    return 'panel-record-routing-active';
  }
  if (routeStates.some(item => item.isRecent)) {
    return 'panel-record-routing-recent';
  }
  return '';
}

function getAdvancedProxyTooltipLines(record) {
  const routeStates = getAdvancedProxyProviderRouteStatesForRecord(record);
  const routeAppIds = routeStates.flatMap(item => item.appTypeIds);
  const takeoverAppIds = getAdvancedProxyAppsForRecord(record);
  const appIds = ADVANCED_PROXY_APP_ORDER.filter(appId => takeoverAppIds.includes(appId) || routeAppIds.includes(appId));
  if (appIds.length === 0) return [];
  const labels = appIds.map(appId => ADVANCED_PROXY_APP_META[appId]?.label || appId);
  const lines = [`高级代理接管：${labels.join(' / ')}`];
  routeStates.forEach(item => {
    const routeLabel = item.routeKind === 'responses_compact'
      ? 'responses/compact'
      : (item.routeKind === 'responses' ? 'responses' : (item.routeKind === 'chat' ? 'chat/completions' : item.routeKind || '代理路由'));
    const statusLabel = item.appTypeLabelText
      ? `⭐ ${item.appTypeLabelText}`
      : (item.isActive ? '⭐ 代理调用中' : '最近一次命中');
    const extra = item.providerName ? ` · ${item.providerName}` : '';
    lines.push(`${statusLabel} · ${routeLabel}${extra}`);
  });
  return lines;
}

function getQueueItemShortLabel(item) {
  const index = String(item?.order || '').trim();
  const siteName = String(item?.siteName || '').trim();
  const shortName = siteName ? siteName.slice(0, 3) : '---';
  return `${index ? `${index}.` : ''}${shortName}`;
}

function getAdvancedProxyQueueOrder(record, fallbackOrder = '') {
  const skKey = String(record?.apiKey || '').trim();
  if (!skKey) return fallbackOrder;
  return advancedProxyQueueOrderByApiKey.value.get(skKey) ?? fallbackOrder;
}

function isAdvancedProxyDispatching(record) {
  return getAdvancedProxyProviderRouteStatesForRecord(record).some(item => item.isActive);
}

function getAdvancedProxyDispatchLabel(record) {
  const routeStates = getAdvancedProxyProviderRouteStatesForRecord(record);
  const labels = routeStates
    .filter(item => item.isActive)
    .flatMap(item => item.appTypeLabels)
    .filter(Boolean);
  if (labels.length === 0) return '';
  return Array.from(new Set(labels)).join(' / ');
}

function getSiteShortName(siteName) {
  const text = String(siteName || '').trim();
  if (!text) return '未命名';
  return text.length > 6 ? text.slice(0, 6) : text;
}

function getQueueTooltipOverlayStyle(item) {
  const total = Math.max(1, advancedProxyQueueItems.value.length);
  const index = Math.max(0, Number(item?.order || 1) - 1);
  const ratio = total <= 1 ? 0.5 : 0.2 + ((index / Math.max(1, total - 1)) * 0.6);
  return {
    '--panel-queue-tooltip-arrow-left': `${(ratio * 100).toFixed(2)}%`,
  };
}

const superMiniQueueCardHintText = computed(() =>
  superMiniQueueCardHintSeenCount.value < SUPER_MINI_QUEUE_CARD_HINT_LIMIT ? SUPER_MINI_QUEUE_CARD_HINT_TEXT : '',
);

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

function beginQueueStripDrag(event) {
  if (event?.button !== 0) return;
  const target = panelQueueStripRef.value;
  if (!target) return;
  panelQueueDragState.active = false;
  panelQueueDragState.armed = true;
  panelQueueDragState.pointerId = event.pointerId;
  panelQueueDragState.startX = event.clientX;
  panelQueueDragState.startScrollLeft = target.scrollLeft;
  panelQueueDragState.moved = false;
  panelQueueDragLogAt = 0;
  appendPanelClientLog('panel.queue.drag', `arm pointer=${event.pointerId} client=${event.clientX},${event.clientY} scroll=${target.scrollLeft || 0}`);
}

function dragQueueStrip(event) {
  if (panelQueueDragState.pointerId !== event.pointerId) return;
  const target = panelQueueStripRef.value;
  if (!target) return;
  const deltaX = event.clientX - panelQueueDragState.startX;
  if (!panelQueueDragState.active && Math.abs(deltaX) <= 4) {
    return;
  }
  if (!panelQueueDragState.active) {
    panelQueueDragState.active = true;
    panelQueueDragState.moved = true;
    try {
      target.setPointerCapture?.(event.pointerId);
    } catch {}
    appendPanelClientLog('panel.queue.drag', `start pointer=${event.pointerId} client=${event.clientX},${event.clientY} scroll=${target.scrollLeft || 0}`);
  }
  if (Math.abs(deltaX) > 3) {
    panelQueueDragState.moved = true;
  }
  target.scrollLeft = panelQueueDragState.startScrollLeft - deltaX;
  const now = Date.now();
  if (!panelQueueDragLogAt || now - panelQueueDragLogAt >= 180) {
    panelQueueDragLogAt = now;
    appendPanelClientLog('panel.queue.drag', `move pointer=${event.pointerId} client=${event.clientX},${event.clientY} scroll=${target.scrollLeft || 0}`);
  }
}

function endQueueStripDrag(event) {
  const target = panelQueueStripRef.value;
  if (target && panelQueueDragState.pointerId === event?.pointerId) {
    try {
      target.releasePointerCapture?.(event.pointerId);
    } catch {}
  }
  panelQueueDragState.active = false;
  panelQueueDragState.armed = false;
  panelQueueDragState.pointerId = -1;
  panelQueueDragState.startX = 0;
  panelQueueDragState.startScrollLeft = 0;
  appendPanelClientLog('panel.queue.drag', `end pointer=${event?.pointerId ?? -1} moved=${panelQueueDragState.moved} armed=${panelQueueDragState.armed} scroll=${target?.scrollLeft || 0}`);
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
  appendPanelClientLog('panel.actions', 'restore main window requested');
  try {
    await RequestMainWindowRestore();
  } catch (error) {
    appendPanelClientLog('panel.actions', `restore main window failed: ${error?.message || String(error)}`);
    message.error(error?.message || '无法恢复主窗口');
  }
}

async function handleOpenEditor(rowKey) {
  appendPanelClientLog('panel.actions', `open editor requested rowKey=${String(rowKey || '')}`);
  try {
    await OpenKeyEditor(String(rowKey || ''));
  } catch (error) {
    appendPanelClientLog('panel.actions', `open editor failed rowKey=${String(rowKey || '')} err=${error?.message || String(error)}`);
    message.error(error?.message || '无法打开编辑窗口');
  }
}

async function handleQuickSetup(record) {
  appendPanelClientLog('panel.actions', `quick setup requested rowKey=${String(record?.rowKey || '')}`);
  try {
    await OpenDesktopConfigWindow(String(record?.rowKey || ''));
  } catch (error) {
    appendPanelClientLog('panel.actions', `quick setup failed rowKey=${String(record?.rowKey || '')} err=${error?.message || String(error)}`);
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

function patchRecord(rowKey, patch) {
  const targetIndex = records.value.findIndex(item => item.rowKey === rowKey);
  if (targetIndex === -1) return null;
  const currentRecord = records.value[targetIndex];
  const nextPatch = typeof patch === 'function' ? patch(currentRecord) : patch;
  if (!nextPatch || typeof nextPatch !== 'object') {
    return currentRecord;
  }
  const nextRecord = {
    ...currentRecord,
    ...nextPatch,
  };
  records.value[targetIndex] = nextRecord;
  schedulePanelPersist();
  void nextTick(syncScrollIndicator);
  return nextRecord;
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

  patchRecord(record.rowKey, { modelLoading: true });
  try {
    const nextRecord = await loadRecordModelOptions(record, contextMap.value);
    updateRecord({
      ...hydrateRecordModelSelection(nextRecord, contextMap.value),
      modelLoading: false,
    });
  } catch (error) {
    patchRecord(record.rowKey, { modelLoading: false });
    message.error(error?.message || '模型获取失败');
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

  patchRecord(record.rowKey, { quickTestLoading: true });
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

    updateRecord({
      ...nextRecord,
      quickTestLoading: false,
    });
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

  patchRecord(record.rowKey, { balanceLoading: true });
  try {
    const nextRecord = await refreshRecordBalance(record, contextMap.value);
    updateRecord({
      ...nextRecord,
      balanceLoading: false,
    });
  } catch (error) {
    updateRecord({
      ...record,
      balanceError: error?.message || '余额刷新失败',
      balanceLoading: false,
    });
    message.error(error?.message || '余额刷新失败');
  }
}

onMounted(async () => {
  appendPanelClientLog('panel.lifecycle', `mount start visibleRecords=${records.value.length} advancedProxyItems=${advancedProxyQueueItems.value?.length || 0}`);
  await hydrateLastResultsSnapshotCache();
  reloadAdvancedProxyConfigState();
  reloadRecords();
  await reloadAdvancedProxyRoutingState();
  superMiniQueueCardHintSeenCount.value = readSuperMiniQueueCardHintSeenCount();
  syncSuperMiniBodyClass();
  appendPanelClientLog('panel.lifecycle', 'mount bootstrap complete');
  try {
    await InitPanelWindow(window?.screen?.availWidth || 1440, window?.screen?.availHeight || 900);
    appendPanelClientLog(
      'panel.lifecycle',
      `init panel window called screen=${window?.screen?.availWidth || 1440}x${window?.screen?.availHeight || 900}`,
    );
  } catch (error) {
    appendPanelClientLog('panel.lifecycle', `init panel window failed: ${error?.message || String(error)}`);
  }
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
  window.addEventListener(HISTORY_SNAPSHOT_SYNC_EVENT, reloadRecords);
  window.addEventListener('storage', reloadRecords);
  window.addEventListener('storage', reloadAdvancedProxyConfigState);
  window.addEventListener('storage', reloadAdvancedProxyRoutingState);
  window.addEventListener(ADVANCED_PROXY_SYNC_EVENT, reloadAdvancedProxyConfigState);
  window.addEventListener(ADVANCED_PROXY_SYNC_EVENT, reloadAdvancedProxyTakeoverState);
  advancedProxyRoutingTimer = window.setInterval(() => {
    void reloadAdvancedProxyRoutingState();
  }, 1200);
  advancedProxyDispatchTimer = window.setInterval(() => {
    advancedProxyDispatchClock.value = Date.now();
  }, ADVANCED_PROXY_DISPATCH_TICK_MS);
});

watch(superMiniMode, async (enabled, previous) => {
  if (enabled === previous) return;
  appendPanelClientLog('panel.super-mini', `mode watch prev=${previous} next=${enabled}`);
  await nextTick();
  await logSuperMiniWindowSnapshot('mode watch snapshot', superMiniTransitionToken, `prev=${previous} next=${enabled}`);
});

watch(visibleRecords, async () => {
  await nextTick();
  syncScrollIndicator();
}, { flush: 'post' });

onBeforeUnmount(() => {
  appendPanelClientLog('panel.lifecycle', 'before unmount');
  if (typeof document !== 'undefined') {
    document.body.classList.remove('panel-super-mini-mode');
  }
  superMiniQueueCardHintHoverStartedAt = 0;
  flushPanelPersist();
  void setPanelInteractionLocked(false);
  panelBodyResizeObserver?.disconnect?.();
  panelBodyResizeObserver = null;
  if (advancedProxyRoutingTimer) {
    window.clearInterval(advancedProxyRoutingTimer);
    advancedProxyRoutingTimer = null;
  }
  if (advancedProxyDispatchTimer) {
    window.clearInterval(advancedProxyDispatchTimer);
    advancedProxyDispatchTimer = null;
  }
  window.removeEventListener('resize', syncScrollIndicator);
  window.removeEventListener(KEY_MANAGEMENT_SYNC_EVENT, reloadRecords);
  window.removeEventListener(HISTORY_SNAPSHOT_SYNC_EVENT, reloadRecords);
  window.removeEventListener('storage', reloadRecords);
  window.removeEventListener('storage', reloadAdvancedProxyConfigState);
  window.removeEventListener('storage', reloadAdvancedProxyRoutingState);
  window.removeEventListener(ADVANCED_PROXY_SYNC_EVENT, reloadAdvancedProxyConfigState);
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

.key-side-panel.is-super-mini {
  width: fit-content;
  height: fit-content;
  min-height: 0;
  padding: 0;
  overflow: visible;
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
  gap: 8px;
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

.panel-shell.is-super-mini {
  width: fit-content;
  height: fit-content;
  padding: 1px;
  gap: 1px;
  background: transparent;
  box-shadow: none;
  backdrop-filter: none;
  align-items: flex-start;
  --super-mini-card-scale: 1;
}

.panel-shell.is-super-mini::before,
.panel-shell.is-super-mini .panel-ambient,
.panel-shell.is-super-mini .panel-topbar,
.panel-shell.is-super-mini .panel-scroll-guide,
.panel-shell.is-super-mini .panel-body {
  display: none;
}

.panel-shell.is-super-mini .panel-queue-card {
  position: relative;
  inset: auto;
  z-index: 3;
  margin: 0;
  width: var(--super-mini-card-width, 128px);
  min-width: var(--super-mini-card-width, 128px);
  max-width: var(--super-mini-card-width, 128px);
  height: var(--super-mini-card-height, auto);
  border-radius: calc(14px * var(--super-mini-card-scale, 1));
  display: flex;
  flex-direction: column;
  box-shadow:
    0 18px 36px rgba(19, 24, 19, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.45);
  cursor: grab;
  --wails-draggable: no-drag;
}

.panel-shell.is-super-mini .panel-queue-card:active {
  cursor: grabbing;
}

.panel-shell.is-super-mini .panel-queue-head {
  gap: calc(4px * var(--super-mini-card-scale, 1));
  margin-bottom: 0;
}

.panel-shell.is-super-mini .panel-queue-strip,
.panel-shell.is-super-mini .panel-queue-empty {
  flex: 0 0 auto;
  min-height: 0;
}

.panel-shell.is-super-mini .panel-queue-strip {
  display: flex;
  align-items: flex-start;
  width: 100%;
  max-width: 100%;
  margin-top: -1px;
}

.panel-shell.is-super-mini .panel-queue-strip-track {
  width: max-content;
  min-width: 0;
  gap: calc(1px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item {
  padding: 0 calc(1px * var(--super-mini-card-scale, 1)) 0;
}

.panel-shell.is-super-mini .panel-queue-item-avatar-wrap,
.panel-shell.is-super-mini .panel-queue-item-avatar,
.panel-shell.is-super-mini .panel-queue-item-avatar .panel-record-emoji {
  width: calc(22px * var(--super-mini-card-scale, 1));
  height: calc(22px * var(--super-mini-card-scale, 1));
  border-radius: calc(8px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item-avatar .panel-record-emoji {
  font-size: calc(11px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item-name {
  font-size: calc(5px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item-order,
.panel-shell.is-super-mini .panel-queue-item-star {
  font-size: calc(5px * var(--super-mini-card-scale, 1));
  padding: 0 calc(1px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-title {
  font-size: calc(5px * var(--super-mini-card-scale, 1));
  letter-spacing: calc(0.12em * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-signals {
  gap: calc(3px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-signal {
  width: calc(6px * var(--super-mini-card-scale, 1));
  height: calc(6px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item-copy {
  margin-top: calc(-2px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item {
  gap: calc(0px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item-order {
  top: calc(-1px * var(--super-mini-card-scale, 1));
  right: calc(-1px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item-star {
  top: calc(-1px * var(--super-mini-card-scale, 1));
  left: calc(-1px * var(--super-mini-card-scale, 1));
}

.panel-shell.is-super-mini .panel-queue-item:hover {
  transform: translateY(calc(-1px * var(--super-mini-card-scale, 1)));
}

:global(body.panel-super-mini-mode),
:global(body.panel-super-mini-mode #app) {
  background: transparent !important;
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
  top: 126px;
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

.panel-queue-card {
  position: relative;
  z-index: 2;
  margin: 0;
  padding: 4px 6px 2px;
  border-radius: 14px;
  border: 1px solid rgba(150, 185, 151, 0.22);
  background: linear-gradient(180deg, rgba(246, 251, 246, 0.96), rgba(232, 241, 231, 0.92));
  box-shadow:
    0 8px 18px rgba(19, 24, 19, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.45);
  overflow: hidden;
  cursor: grab;
  --wails-draggable: no-drag;
}

.panel-queue-card:active {
  cursor: grabbing;
}

.panel-queue-card-red {
  border-color: rgba(237, 109, 109, 0.24);
  box-shadow:
    0 10px 22px rgba(19, 24, 19, 0.08),
    0 0 0 1px rgba(237, 109, 109, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.45);
}

.panel-queue-card-yellow {
  border-color: rgba(235, 195, 92, 0.24);
  box-shadow:
    0 10px 22px rgba(19, 24, 19, 0.08),
    0 0 0 1px rgba(235, 195, 92, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.45);
}

.panel-queue-card-green {
  border-color: rgba(110, 183, 121, 0.26);
  box-shadow:
    0 10px 22px rgba(19, 24, 19, 0.08),
    0 0 0 1px rgba(110, 183, 121, 0.08),
    inset 0 1px 0 rgba(255, 255, 255, 0.45);
}

.panel-queue-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 2px;
  min-height: 0;
  position: relative;
}

.panel-queue-copy {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.panel-queue-title {
  margin: 0;
  color: rgba(33, 49, 32, 0.4);
  font-size: 6px;
  line-height: 1.1;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.panel-queue-signals {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 0;
  flex: 0 0 auto;
}

.panel-queue-signal {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  opacity: 0.28;
  transform: scale(0.92);
  transition: opacity 0.16s ease, transform 0.16s ease, box-shadow 0.16s ease;
}

.panel-queue-signal.is-active {
  opacity: 1;
  transform: scale(1.12);
}

.panel-queue-signal-red {
  background: #ef6a6a;
  box-shadow: 0 0 12px rgba(239, 106, 106, 0.34);
}

.panel-queue-signal-yellow {
  background: #ebb94f;
  box-shadow: 0 0 12px rgba(235, 185, 79, 0.34);
}

.panel-queue-signal-green {
  background: #57bd75;
  box-shadow: 0 0 12px rgba(87, 189, 117, 0.34);
}

.panel-queue-strip {
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
  -ms-overflow-style: none;
  cursor: grab;
  user-select: none;
  touch-action: pan-y;
}

.panel-queue-strip::-webkit-scrollbar {
  display: none;
}

.panel-queue-strip-track {
  display: flex;
  gap: clamp(4px, 0.9vw, 8px);
  flex-wrap: nowrap;
  width: 100%;
  min-width: 100%;
  justify-content: flex-start;
  padding-top: 0;
}

.panel-queue-item {
  appearance: none;
  -webkit-appearance: none;
  border: 0;
  border-radius: 0;
  background: transparent;
  min-width: 0;
  width: auto;
  min-height: 0;
  padding: 0 2px 0;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  flex: 0 0 auto;
  gap: 0;
  text-align: center;
  box-shadow: none;
  transition: transform 0.18s ease, opacity 0.18s ease;
}

.panel-queue-item:hover {
  transform: translateY(-1px);
}

.panel-queue-item-avatar-wrap {
  position: relative;
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
}

.panel-queue-item-avatar {
  width: 28px;
  height: 28px;
  border-radius: 10px;
  background: rgba(34, 49, 72, 0.06);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.panel-queue-item-avatar .panel-record-emoji {
  width: 28px;
  height: 28px;
  border-radius: 10px;
  background: transparent;
  font-size: 15px;
}

.panel-queue-item-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0;
  margin-top: -5px;
}

.panel-queue-item-name {
  width: 100%;
  color: rgba(34, 49, 28, 0.56);
  font-size: 7px;
  font-weight: 600;
  line-height: 1;
  letter-spacing: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  transform: translateY(-20%);
}

.panel-queue-item-order {
  position: absolute;
  right: -2px;
  top: 0px;
  z-index: 10;
  padding: 0;
  background: transparent;
  box-shadow: none;
  color: rgba(22, 35, 56, 0.72);
  font-size: 6px;
  line-height: 1;
  font-weight: 400;
  letter-spacing: 0.01em;
  white-space: nowrap;
  pointer-events: none;
}

.panel-queue-item-star {
  position: absolute;
  left: -2px;
  top: 0px;
  z-index: 10;
  color: #f5d06d;
  font-size: 7px;
  line-height: 1;
  font-weight: 700;
  text-shadow: 0 0 6px rgba(255, 196, 72, 0.28);
  pointer-events: none;
}

.panel-queue-empty {
  padding: 10px 8px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.64);
  color: #6a7867;
  font-size: 10px;
  line-height: 1.45;
}

.panel-queue-item-enter-active,
.panel-queue-item-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease, filter 0.22s ease;
}

.panel-queue-item-enter-from,
.panel-queue-item-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.92);
  filter: blur(1px);
}

.panel-queue-item-move {
  transition: transform 0.28s ease;
}

:global(.panel-queue-tooltip .ant-tooltip-inner) {
  padding: 8px 10px;
}

.panel-queue-tooltip-content {
  display: grid;
  gap: 4px;
  max-width: 280px;
  line-height: 1.45;
}

.panel-queue-tooltip-content strong,
.panel-queue-tooltip-content span,
.panel-queue-tooltip-content code {
  display: block;
  word-break: break-word;
}

.panel-queue-tooltip-status {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.panel-queue-tooltip-state {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  color: rgba(233, 240, 232, 0.54);
  font-size: 9px;
  line-height: 1.2;
  white-space: nowrap;
}

.panel-queue-tooltip-state.is-active {
  background: rgba(255, 255, 255, 0.12);
  color: #ffffff;
}

.panel-queue-tooltip-state.is-fading {
  background: rgba(255, 255, 255, 0.08);
}

.panel-queue-tooltip-hit-summary {
  color: #f1f5ed;
  font-size: 10px;
  line-height: 1.35;
  white-space: pre-wrap;
}

.panel-queue-tooltip-content strong {
  color: #ffffff;
  font-size: 12px;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.24);
}

.panel-queue-tooltip-content span {
  color: #5f6e5a;
  font-size: 11px;
}

.panel-queue-tooltip-content code {
  color: #4a5c45;
  font-size: 10px;
  white-space: pre-wrap;
}

:global(.panel-queue-tooltip .ant-tooltip-arrow) {
  left: var(--panel-queue-tooltip-arrow-left, 50%) !important;
  inset-inline-start: var(--panel-queue-tooltip-arrow-left, 50%) !important;
  transform: translateX(-50%) !important;
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
  gap: 8px;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 6px 2px 12px 8px;
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
  gap: 6px;
  min-height: 0;
  padding: 10px 11px 8px;
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

.panel-record-routing-active {
  border-color: rgba(255, 207, 92, 0.92);
  box-shadow:
    0 0 0 1px rgba(255, 222, 133, 0.5),
    0 16px 34px rgba(21, 22, 28, 0.16),
    0 0 22px rgba(255, 191, 68, 0.3),
    0 0 44px rgba(255, 170, 0, 0.14);
}

.panel-record-routing-active::before {
  background: rgba(255, 240, 201, 0.68);
  opacity: 0.94;
}

.panel-record-routing-active::after {
  background: rgba(255, 225, 150, 0.26);
  opacity: 0.82;
}

.panel-record-routing-recent {
  border-color: rgba(255, 214, 122, 0.64);
  box-shadow:
    0 12px 24px rgba(21, 22, 28, 0.12),
    0 0 16px rgba(255, 196, 72, 0.16);
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
  gap: 6px;
}

.panel-record-sitebox {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.panel-record-avatar {
  position: relative;
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
  border-radius: 10px;
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

.panel-record-avatar-routing-active::before {
  inset: -9px;
  opacity: 1;
  background:
    conic-gradient(
      from 0deg,
      transparent 0deg 20deg,
      rgba(255, 215, 94, 0.98) 38deg 90deg,
      transparent 116deg 158deg,
      rgba(255, 189, 46, 0.98) 188deg 242deg,
      transparent 268deg 318deg,
      rgba(255, 170, 0, 0.94) 334deg 360deg
    );
  box-shadow:
    0 0 14px rgba(255, 185, 42, 0.54),
    0 0 26px rgba(255, 191, 68, 0.66),
    0 0 42px rgba(255, 170, 0, 0.4);
  animation:
    panel-avatar-orbit 1.6s linear infinite,
    panel-avatar-pulse 1.1s ease-in-out infinite alternate;
}

.panel-record-avatar-routing-active::after {
  inset: -4px;
  opacity: 1;
  border-width: 2px;
  border-color: rgba(255, 196, 72, 0.9);
  box-shadow:
    0 0 0 1px rgba(255, 212, 125, 0.34),
    0 0 18px rgba(255, 196, 72, 0.5),
    0 0 28px rgba(255, 170, 0, 0.26);
}

.panel-record-avatar-routing-active .panel-record-emoji {
  box-shadow:
    0 0 0 2px rgba(255, 207, 102, 0.34),
    0 0 16px rgba(255, 185, 42, 0.34);
}

.panel-record-avatar-routing-active .panel-record-order {
  box-shadow:
    0 4px 12px rgba(13, 24, 20, 0.3),
    0 0 18px rgba(255, 196, 72, 0.28),
    inset 0 1px 0 rgba(223, 255, 239, 0.24);
}

.panel-record-avatar-routing-recent::before {
  opacity: 1;
  box-shadow:
    0 0 10px rgba(255, 196, 72, 0.24),
    0 0 18px rgba(255, 185, 42, 0.18);
}

.panel-record-avatar-routing-recent::after {
  opacity: 1;
  box-shadow:
    0 0 0 1px rgba(255, 212, 125, 0.2),
    0 0 14px rgba(255, 196, 72, 0.18);
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
  width: 28px;
  height: 28px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: rgba(34, 49, 72, 0.07);
  font-size: 14px;
  line-height: 1;
}

.panel-record-order {
  position: absolute;
  right: -7px;
  top: -6px;
  z-index: 2;
  padding: 1px 3px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(61, 110, 88, 0.98), rgba(24, 44, 35, 0.98));
  box-shadow:
    0 4px 10px rgba(13, 24, 20, 0.24),
    inset 0 1px 0 rgba(223, 255, 239, 0.2);
  color: #eef8f1;
  font-size: 7px;
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: 0.01em;
  white-space: nowrap;
}

.panel-record-dispatch-star {
  position: absolute;
  left: -7px;
  top: -6px;
  z-index: 2;
  padding: 1px 3px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(61, 110, 88, 0.98), rgba(24, 44, 35, 0.98));
  box-shadow:
    0 4px 10px rgba(13, 24, 20, 0.24),
    inset 0 1px 0 rgba(223, 255, 239, 0.2);
  color: #eef8f1;
  font-size: 7px;
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: 0.01em;
  pointer-events: none;
}

.panel-record-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.panel-record-site {
  color: #162338;
  font-size: 11px;
  line-height: 1.04;
  font-weight: 700;
  letter-spacing: 0.01em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.panel-record-model {
  color: rgba(35, 49, 72, 0.62);
  font-size: 8px;
  line-height: 1.08;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.panel-record-status {
  width: fit-content;
  padding: 3px 6px;
  border-radius: 999px;
  font-size: 9px;
  line-height: 1;
  font-weight: 700;
}

.panel-record-status-ok {
  min-width: 14px;
  min-height: 14px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: transparent;
  background: transparent;
}

.panel-record-status-bad {
  color: #806553;
  background: rgba(123, 98, 78, 0.12);
}

.panel-record-status-dot {
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: #57bd75;
  box-shadow: 0 0 0 1px rgba(87, 189, 117, 0.2), 0 0 10px rgba(87, 189, 117, 0.24);
}

.panel-record-metrics {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 3px;
}

.panel-record-metrics-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  min-height: 24px;
}

.panel-record-quick-group {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.panel-record-balance {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 28px;
  padding: 5px 8px;
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(255, 244, 201, 0.92), rgba(251, 230, 156, 0.84));
}

.panel-record-balance-empty {
  background: linear-gradient(180deg, rgba(233, 237, 238, 0.92), rgba(220, 226, 228, 0.84));
}

.panel-record-balance-label {
  color: rgba(22, 35, 56, 0.58);
  font-size: 8px;
  line-height: 1;
  font-weight: 600;
}

.panel-record-balance-value {
  min-width: 0;
  color: var(--panel-gold);
  font-size: 10px;
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
  background: rgba(255, 249, 239, 0.92);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.panel-record-meta {
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding-top: 6px;
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

.panel-routing-tooltip {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-width: 240px;
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

@keyframes panel-avatar-pulse {
  0% {
    transform: scale(0.98);
    filter: saturate(1);
  }
  100% {
    transform: scale(1.04);
    filter: saturate(1.16);
  }
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
