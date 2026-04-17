<template>
  <ConfigProvider :theme="configProviderTheme">
    <div class="wrapper batch-wrapper">
      <div class="batch-shell">
        <div class="batch-forest-scene" aria-hidden="true">
          <div class="forest-mist forest-mist-left"></div>
          <div class="forest-mist forest-mist-right"></div>
          <div class="forest-path-glow"></div>
          <div class="forest-firegrass firegrass-left"></div>
          <div class="forest-firegrass firegrass-right"></div>
          <div class="forest-slime slime-a"></div>
          <div class="forest-slime slime-b"></div>
          <div class="forest-slime slime-c"></div>
        </div>

        <div class="page-content batch-page-content">
          <div class="container batch-page-container">
            <AppHeader
              current-page="sites"
              :is-dark-mode="isDarkMode"
              @experimental="handleExperimental"
              @settings="handleSettings"
              @toggle-theme="handleToggleTheme"
            />

            <section class="batch-hero batch-hero-compact">
              <div class="batch-hero-motion" aria-hidden="true">
                <span class="leaf leaf-a"></span>
                <span class="leaf leaf-b"></span>
                <span class="leaf leaf-c"></span>
                <span class="leaf leaf-d"></span>
                <span class="grass grass-a"></span>
                <span class="grass grass-b"></span>
                <span class="grass grass-c"></span>
              </div>

              <div class="batch-hero-head">
                <div class="batch-hero-copy">
                  <p class="batch-hero-kicker">Site Cache Workspace</p>
                  <div class="page-title-row">
                    <div class="page-title-block">
                      <h1 class="page-title">站点管理</h1>
                      <p class="page-subtitle">
                        用户态 Token 与账号内密钥，树形浏览并按站点就地维护。
                      </p>
                    </div>
                  </div>
                  <div class="batch-hero-meta">
                    <span class="batch-hero-tag">缓存站点 {{ records.length }}</span>
                    <span class="batch-hero-tag">已禁用 {{ disabledCount }}</span>
                    <span class="batch-hero-tag">自定义 SK {{ customTokenCount }}</span>
                  </div>
                </div>
              </div>
            </section>

            <div class="step-container">
              <div class="selection-topbar">
                <div class="selection-header-row">
                  <h3 class="selection-title">请勾选需要测试的网站与模型</h3>
                  <a-space wrap class="selection-action-group">
                    <a-button @click="selectAllNodes" size="small">全部全选</a-button>
                    <a-button @click="unselectAllNodes" size="small">全部反选</a-button>
                    <a-button @click="selectChatModelsOnly" size="small">仅选主流聊天</a-button>
                  </a-space>
                </div>
                <div class="selection-quick-filters">
                  <div class="quick-filter-toolbar">
                    <div class="quick-filter-strip" v-if="quickFilters.length">
                      <a-popover
                        v-for="family in quickFilters"
                        :key="family.key"
                        trigger="hover"
                        placement="bottomLeft"
                        overlayClassName="quick-filter-family-popover"
                      >
                        <template #content>
                          <div class="quick-filter-family-panel">
                            <div class="quick-filter-family-panel-title">{{ family.label }}</div>
                            <div class="quick-filter-option-list">
                              <a-button
                                v-for="option in family.options"
                                :key="option.key"
                                size="small"
                                :type="activeQuickFilters.includes(option.key) ? 'primary' : 'default'"
                                @click="toggleQuickFilter(option.key)"
                              >
                                {{ option.label }}
                              </a-button>
                              <a-button
                                size="small"
                                class="quick-filter-family-select-all"
                                @click="selectQuickFilterFamily(family)"
                              >
                                {{ isQuickFilterFamilyFullySelected(family) ? '取消' : '全选' }}
                              </a-button>
                            </div>
                          </div>
                        </template>
                        <a-button
                          class="quick-filter-family-trigger"
                          :type="isQuickFilterFamilyActive(family) ? 'primary' : 'default'"
                          @click="selectQuickFilterFamily(family)"
                        >
                          {{ family.label }}
                          <span v-if="getQuickFilterFamilyActiveCount(family)" class="quick-filter-family-count">
                            {{ getQuickFilterFamilyActiveCount(family) }}
                          </span>
                        </a-button>
                      </a-popover>
                      <a-button
                        class="quick-filter-clear-trigger"
                        @click="clearQuickFilters"
                        :disabled="!activeQuickFilters.length"
                      >
                        清空
                      </a-button>
                    </div>
                    <div v-else class="quick-filter-empty-inline">暂无可用快捷分组</div>
                    <span v-if="activeQuickFilterSummary" class="quick-filter-summary">{{ activeQuickFilterSummary }}</span>
                  </div>
                </div>
              </div>

              <div class="tree-wrapper">
                <a-empty v-if="!treeData.length" description="当前没有可展示的站点缓存" />
                <a-tree
                  v-else
                  v-model:checkedKeys="checkedKeys"
                  :expanded-keys="expandedKeys"
                  :tree-data="treeData"
                  checkable
                  @expand="handleTreeExpand"
                  @check="handleTreeCheck"
                >
                  <template #title="node">
                    <div class="custom-tree-node-wrapper tree-provider-node-wrapper" style="display: flex; align-items: center;">
                      <div class="provider-tree-label">
                        <button
                          v-if="node.isSiteRoot && node.providerSiteUrl"
                          type="button"
                          :class="['provider-tree-link', { 'is-grey': node.siteDisabled }]"
                          @click.stop="openSiteUrl(node.providerSiteUrl)"
                        >
                          {{ node.providerTitleText }}
                        </button>
                        <span
                          v-if="node.isSiteRoot"
                          :class="['custom-tree-node', node.titleClass]"
                        >
                          {{ node.providerStatusText }}
                        </span>
                        <span
                          v-else
                          :class="['custom-tree-node', node.titleClass]"
                        >
                          {{ node.title }}
                        </span>
                        <span v-if="node.isManualToken" class="site-tree-inline-tag">手动添加</span>
                        <a-popconfirm
                          v-if="node.isManualToken"
                          title="确认删除这个手动添加的 key？"
                          @confirm="removeManualTokenByNode(node)"
                        >
                          <button type="button" class="site-tree-inline-delete-btn" @click.stop>
                            <DeleteOutlined />
                          </button>
                        </a-popconfirm>
                        <span v-if="node.isSiteRoot && node.siteNote" class="site-tree-note-badge">
                          {{ node.siteNote }}
                        </span>
                      </div>
                      <span v-if="node.isModelDiscovering || node.isBrowserPending" class="tree-node-pending-hint">
                        <a-spin size="small" />
                        <span>{{ node.isModelDiscovering ? (node.modelDiscoveringHint || '模型检测中') : node.pendingHint }}</span>
                      </span>
                      <div v-if="node.isSiteRoot" class="site-tree-actions">
                        <a-tooltip title="基于缓存用户态 token 重新读取站点数据">
                          <button type="button" class="site-tree-action-btn" @click.stop="refreshOneByNode(node)">
                            <ReloadOutlined />
                          </button>
                        </a-tooltip>
                        <a-tooltip title="手动追加自定义 sk">
                          <button type="button" class="site-tree-action-btn" @click.stop="appendCustomSkByNode(node)">
                            <LockOutlined />
                          </button>
                        </a-tooltip>
                        <a-tooltip :title="node.siteDisabled ? '激活该站点' : '禁用该站点'">
                          <button type="button" class="site-tree-action-btn" @click.stop="toggleDisabledByNode(node)">
                            <CheckCircleOutlined v-if="node.siteDisabled" />
                            <StopOutlined v-else />
                          </button>
                        </a-tooltip>
                        <a-tooltip title="设置 10 字以内备注">
                          <button type="button" class="site-tree-action-btn" @click.stop="editNoteByNode(node)">
                            <MessageOutlined />
                          </button>
                        </a-tooltip>
                        <a-popconfirm title="确认删除该站点缓存？" @confirm="removeRecordByNode(node)">
                          <button type="button" class="site-tree-action-btn is-danger" @click.stop>
                            <DeleteOutlined />
                          </button>
                        </a-popconfirm>
                      </div>
                      <div v-if="isProviderDiagnosticTreeNode(node)" class="provider-tree-actions">
                        <a-popover trigger="hover" placement="rightTop" overlayClassName="provider-diagnostic-popover">
                          <template #content>
                            <div class="provider-diagnostic-menu">
                              <a-button size="small" @click.stop="copyProviderFetchReplay(node)">复制 fetch 复现</a-button>
                              <a-button size="small" @click.stop="copyProviderTraceLog(node)">复制调研 trace 日志</a-button>
                            </div>
                          </template>
                          <span class="provider-diagnostic-trigger" @click.stop>调试</span>
                        </a-popover>
                      </div>
                    </div>
                  </template>
                </a-tree>
              </div>

              <div class="settings-action-bar">
                <div class="batch-settings">
                  <span class="batch-settings-label" style="margin-right: 10px;">并发数：</span>
                  <a-input-number v-model:value="batchConcurrency" :min="1" :max="100" />
                  <span class="batch-settings-label" style="margin-left: 20px; margin-right: 10px;">超时(秒)：</span>
                  <a-input-number v-model:value="modelTimeout" :min="1" />
                </div>
                <div class="actions">
                  <a-button @click="goBackToImport" style="margin-right: 10px;">重新导入</a-button>
                  <a-button type="primary" size="large" @click="startBatchCheckFromSiteManagement">
                    开始检测
                  </a-button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <SystemSettingsModal
      v-model:open="showAppSettingsModal"
      v-model:tree-expanded="globalTreeExpanded"
      v-model:desktop-token-source-mode="desktopTokenSourceMode"
      :app-name="'All API Deck'"
    />
    <AdvancedProxyModal v-model:open="showExperimentalFeatures" />
  </ConfigProvider>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ConfigProvider, message, theme } from 'ant-design-vue';
import {
  ReloadOutlined,
  LockOutlined,
  StopOutlined,
  CheckCircleOutlined,
  MessageOutlined,
  DeleteOutlined,
} from '@ant-design/icons-vue';
import AppHeader from './AppHeader.vue';
import AdvancedProxyModal from './AdvancedProxyModal.vue';
import SystemSettingsModal from './SystemSettingsModal.vue';
import { toggleTheme } from '../utils/theme.js';
import { loadDesktopTokenSourceMode, loadTreeExpandedSetting } from '../utils/systemSettings.js';
import { refreshCachedSiteTokens } from '../utils/siteTokenRefresh.js';
import {
  SITE_CACHE_SYNC_EVENT,
  appendCustomKeysToSiteCache,
  consumePendingBatchStart,
  deleteSiteCacheRecord,
  loadAllSiteCacheRecords,
  mergeExtractedSitesIntoTempCache,
  mergeExtractedSitesIntoCache,
  removeCustomKeyFromSiteCache,
  setSiteCacheDisabled,
  updateSiteCacheNote,
  writePendingBatchStart,
  writePendingSiteRestore,
} from '../utils/siteCacheStore.js';

const SITE_NOTE_MAX_LENGTH = 10;

const router = useRouter();
const showExperimentalFeatures = ref(false);
const showAppSettingsModal = ref(false);
const globalTreeExpanded = ref(loadTreeExpandedSetting(true));
const desktopTokenSourceMode = ref(loadDesktopTokenSourceMode());
const isDarkMode = ref(false);
const keyword = ref('');
const hideDisabled = ref(false);
const records = ref([]);
const checkedKeys = ref([]);
const expandedKeys = ref([]);
const batchConcurrency = ref(25);
const modelTimeout = ref(15);
const activeQuickFilters = ref([]);
const quickFilterSelectionMode = ref(false);
const stableSiteOrderMap = new Map();

const configProviderTheme = computed(() => ({
  algorithm: isDarkMode.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}));

const disabledCount = computed(() => records.value.filter(item => item.disabled).length);
const customTokenCount = computed(() => records.value.reduce((sum, item) => sum + (item.customTokens?.length || 0), 0));

const cloneNodeList = value => {
  if (!Array.isArray(value)) return [];
  try {
    return JSON.parse(JSON.stringify(value));
  } catch {
    return [];
  }
};

const extractSiteOrderFromText = value => {
  const match = String(value || '').trim().match(/^(\d+)\./);
  return match ? Number(match[1]) : 0;
};

const getCachedSitePreferredOrder = record => {
  const cachedNodes = Array.isArray(record?.cachedTreeNodes) ? record.cachedTreeNodes : [];
  const rootNode = cachedNodes.find(node => node?.isSiteRoot) || cachedNodes[0] || null;
  if (!rootNode) return 0;
  return (
    extractSiteOrderFromText(rootNode?.providerTitleText) ||
    extractSiteOrderFromText(rootNode?.title) ||
    0
  );
};

const ensureStableSiteOrders = nextRecords => {
  const usedOrders = new Set(Array.from(stableSiteOrderMap.values()).filter(value => Number(value) > 0));
  let nextOrder = usedOrders.size ? Math.max(...usedOrders) + 1 : 1;
  (Array.isArray(nextRecords) ? nextRecords : []).forEach(record => {
    const siteCacheKey = String(record?.siteCacheKey || '').trim();
    if (!siteCacheKey || stableSiteOrderMap.has(siteCacheKey)) return;
    const preferredOrder = getCachedSitePreferredOrder(record);
    if (preferredOrder > 0 && !usedOrders.has(preferredOrder)) {
      stableSiteOrderMap.set(siteCacheKey, preferredOrder);
      usedOrders.add(preferredOrder);
      return;
    }
    while (usedOrders.has(nextOrder)) nextOrder += 1;
    stableSiteOrderMap.set(siteCacheKey, nextOrder);
    usedOrders.add(nextOrder);
    nextOrder += 1;
  });
};

const getStableSiteOrder = record => {
  const siteCacheKey = String(record?.siteCacheKey || '').trim();
  if (!siteCacheKey) return 0;
  return Number(stableSiteOrderMap.get(siteCacheKey) || getCachedSitePreferredOrder(record) || 0);
};

const filteredRecords = computed(() => {
  const text = String(keyword.value || '').trim().toLowerCase();
  return records.value.filter(record => {
    if (hideDisabled.value && record.disabled) return false;
    if (!text) return true;
    return [
      record.siteName,
      record.siteUrl,
      record.note,
      record.resolvedUserId,
    ].some(value => String(value || '').toLowerCase().includes(text));
  }).sort((left, right) => {
    const leftOrder = getStableSiteOrder(left);
    const rightOrder = getStableSiteOrder(right);
    if (leftOrder !== rightOrder) return leftOrder - rightOrder;
    return String(left?.siteName || '').localeCompare(String(right?.siteName || ''));
  });
});

const rootSiteKeys = computed(() => treeData.value
  .filter(node => node?.isSiteRoot)
  .map(node => String(node?.key || '').trim())
  .filter(Boolean));

const selectedModelKeys = computed(() => checkedKeys.value
  .map(key => String(key || '').trim())
  .filter(isSelectableModelKey));

const selectedSiteCacheKeys = computed(() => Array.from(new Set(
  selectedModelKeys.value
    .map(key => key.split('|')[0])
    .map(value => String(value || '').trim())
    .filter(Boolean)
)));

const extractTokenKeyFromNode = node => {
  const key = String(node?.key || '').trim();
  if (!key.startsWith('token|')) return '';
  const parts = key.split('|');
  return String(parts[2] || '').trim();
};

const collectLeafModelNames = nodes => {
  const bucket = [];
  const walk = list => {
    (Array.isArray(list) ? list : []).forEach(node => {
      const key = String(node?.key || '').trim();
      if (node?.isLeaf === true && isSelectableModelKey(key)) {
        const modelName = getModelNameFromSelectableKey(key);
        if (modelName) bucket.push(modelName);
        return;
      }
      if (Array.isArray(node?.children) && node.children.length > 0) {
        walk(node.children);
      }
    });
  };
  walk(nodes);
  return Array.from(new Set(bucket));
};

const buildManualTokenNode = (record, displayOrder, token, tokenIndex, siteModels) => {
  const tokenKey = String(token?.key || token?.access_token || `token-${tokenIndex + 1}`).trim();
  if (!tokenKey) return null;
  const tokenName = String(token?.name || `Manual SK ${tokenIndex + 1}`).trim() || `Manual SK ${tokenIndex + 1}`;
  return {
    title: `${displayOrder}. [${record.siteName}] ${tokenName} (${maskValue(tokenKey)})`,
    key: `token|${record.siteCacheKey}|${tokenKey}`,
    siteCacheKey: record.siteCacheKey,
    disableCheckbox: true,
    selectable: false,
    isManualToken: true,
    children: (Array.isArray(siteModels) ? siteModels : [])
      .map(model => String(model || '').trim())
      .filter(Boolean)
      .map(model => ({
        title: model,
        key: `${record.siteCacheKey}|${tokenKey}|${model}`,
        isLeaf: true,
      })),
  };
};

const buildFallbackTree = (record, displayOrder) => {
  const remoteTokens = Array.isArray(record.tokens) ? record.tokens : [];
  const customTokens = Array.isArray(record.customTokens) ? record.customTokens : [];
  const mergedTokens = [
    ...remoteTokens.map(token => ({ ...token, _origin: 'remote' })),
    ...customTokens.map(token => ({ ...token, _origin: 'manual' })),
  ];
  const usableCount = mergedTokens.filter(token => Number(token?.status ?? 1) === 1).length;
  const summaryParts = [
    `${usableCount} 个可用 Key`,
    record.lastSyncedAt || record.updatedAt ? `同步 ${formatTime(record.lastSyncedAt || record.updatedAt)}` : '',
  ].filter(Boolean);

  return [{
    key: `site-root|${record.siteCacheKey}`,
    title: `${displayOrder}. [${record.siteName}]`,
    providerTitleText: `${displayOrder}. [${record.siteName}]`,
    providerStatusText: `- ${summaryParts.join(' / ')}`,
    providerSiteUrl: record.siteUrl,
    siteCacheKey: record.siteCacheKey,
    siteDisabled: record.disabled === true,
    siteNote: String(record.note || '').trim(),
    disableCheckbox: true,
    selectable: false,
    titleClass: record.disabled ? 'tree-site-disabled' : '',
    isSiteRoot: true,
    children: mergedTokens.map((token, tokenIndex) => {
      const tokenKey = String(token?.key || token?.access_token || `token-${tokenIndex + 1}`).trim();
      const tokenName = String(token?.name || `Token ${tokenIndex + 1}`).trim();
      const sourceLabel = token?._origin === 'manual' ? '手工' : '内置';
      const statusLabel = Number(token?.status ?? 1) === 1 ? '可用' : '异常';
      return {
        key: `token|${record.siteCacheKey}|${tokenKey}|${tokenIndex}`,
        siteCacheKey: record.siteCacheKey,
        title: `${tokenName} · ${maskValue(tokenKey)} · ${sourceLabel} · ${statusLabel}`,
        disableCheckbox: true,
        selectable: false,
        isManualToken: token?._origin === 'manual',
        titleClass: Number(token?.status ?? 1) === 1 ? '' : 'tree-site-disabled',
      };
    }),
  }];
};

const treeData = computed(() => filteredRecords.value.flatMap(record => {
  const displayOrder = getStableSiteOrder(record) || 0;
  const cachedNodes = cloneNodeList(record.cachedTreeNodes);
  if (!cachedNodes.length) {
    return buildFallbackTree(record, displayOrder);
  }
  const siteDisplayTitle = `${displayOrder}. [${record.siteName}]`;
  const siteModels = collectLeafModelNames(cachedNodes);
  const customTokens = Array.isArray(record.customTokens) ? record.customTokens : [];
  return cachedNodes.map(node => {
    if (node?.isSiteRoot) {
      const existingChildren = Array.isArray(node?.children) ? node.children : [];
      const existingTokenKeys = new Set(existingChildren.map(extractTokenKeyFromNode).filter(Boolean));
      const manualChildren = customTokens
        .filter(token => !existingTokenKeys.has(String(token?.key || token?.access_token || '').trim()))
        .map((token, index) => buildManualTokenNode(record, displayOrder, token, index, siteModels))
        .filter(Boolean);
      const mergedChildren = [...existingChildren, ...manualChildren];
      const mergedModelNames = collectLeafModelNames(mergedChildren);
      const usableKeyCount = mergedChildren.filter(child => String(child?.key || '').startsWith('token|')).length;
      return {
        ...node,
        title: siteDisplayTitle,
        providerTitleText: siteDisplayTitle,
        providerStatusText: mergedModelNames.length > 0
          ? `- ${usableKeyCount} 个可用 Key / ${mergedModelNames.length} 个模型`
          : (usableKeyCount > 0 ? `- ${usableKeyCount} 个可用 Key` : String(node?.providerStatusText || '').trim()),
        children: mergedChildren,
        disableCheckbox: true,
        selectable: false,
        siteCacheKey: record.siteCacheKey,
        siteDisabled: record.disabled === true,
        siteNote: String(record.note || '').trim(),
        titleClass: record.disabled === true ? 'tree-site-disabled' : (node?.titleClass || ''),
      };
    }
    return node;
  });
}));

const syncExpandedKeys = () => {
  const allowed = new Set(treeData.value.map(node => node.key));
  const next = expandedKeys.value.filter(key => allowed.has(key));
  expandedKeys.value = next.length ? next : treeData.value.map(node => node.key);
};

const syncCheckedKeys = () => {
  const allowed = new Set(collectSelectableModelKeysFromTreeNodes(treeData.value, []));
  checkedKeys.value = checkedKeys.value.filter(key => allowed.has(String(key || '').trim()));
};

const reloadRecords = () => {
  const nextRecords = loadAllSiteCacheRecords();
  ensureStableSiteOrders(nextRecords);
  records.value = nextRecords;
  syncExpandedKeys();
  syncCheckedKeys();
};

const formatTime = value => {
  const timestamp = Number(value || 0);
  if (!timestamp) return '-';
  try {
    return new Date(timestamp).toLocaleString();
  } catch {
    return '-';
  }
};

const maskValue = value => {
  const text = String(value || '').trim();
  if (!text) return '';
  if (text.length <= 16) return text;
  return `${text.slice(0, 8)}...${text.slice(-6)}`;
};

const isSelectableModelKey = (key) => {
  const text = String(key || '');
  if (!text.includes('|')) return false;
  if (text.startsWith('token|')) return false;
  if (text.startsWith('fail-site|')) return false;
  if (text.startsWith('no-model-site|')) return false;
  if (text.startsWith('no-usable-token-site|')) return false;
  if (text.startsWith('discover-loading|')) return false;
  const parts = text.split('|');
  return parts.length >= 3;
};

const collectSelectableModelKeysFromTreeNodes = (nodes, bucket = []) => {
  (Array.isArray(nodes) ? nodes : []).forEach(node => {
    const key = String(node?.key || '');
    if (node?.isLeaf === true && isSelectableModelKey(key)) {
      bucket.push(key);
      return;
    }
    const children = Array.isArray(node?.children) ? node.children : [];
    if (children.length > 0) {
      collectSelectableModelKeysFromTreeNodes(children, bucket);
    }
  });
  return bucket;
};

const normalizeQuickFilterName = (name) => {
  const normalized = String(name || '').trim();
  if (!normalized) return '';
  const withoutVendor = normalized.includes('/') ? normalized.split('/').pop() : normalized;
  return String(withoutVendor || '').trim();
};

const extractQuickFilterCategory = (name) => {
  const normalized = normalizeQuickFilterName(name);
  if (!normalized) return '';
  const match = normalized.match(/gpt|[a-zA-Z]{3,}/i);
  return match ? match[0].toLowerCase() : '';
};

const extractQuickFilterVersion = (name) => {
  const normalized = normalizeQuickFilterName(name);
  if (!normalized) return '';
  const match = normalized.match(/\d+(?:\.\d+)?/);
  return match ? match[0] : '';
};

const buildQuickFilterOptionLabel = (category, version, sampleName) => {
  if (version) return `${category}-${version}`;
  return normalizeQuickFilterName(sampleName || category);
};

const getModelNameFromSelectableKey = (key) => {
  const parts = String(key || '').split('|');
  if (parts.length < 3) return '';
  return String(parts.slice(2).join('|') || '').trim();
};

const quickFilterSourceModels = computed(() => Array.from(new Set(
  collectSelectableModelKeysFromTreeNodes(treeData.value, [])
    .map(getModelNameFromSelectableKey)
    .map(model => String(model || '').trim())
    .filter(Boolean)
)));

const quickFilters = computed(() => {
  const models = quickFilterSourceModels.value;
  const familyMap = new Map();

  models.forEach(model => {
    const category = extractQuickFilterCategory(model);
    if (!category) return;
    const version = extractQuickFilterVersion(model);
    const familyKey = category;
    const optionKey = `${familyKey}:${version || normalizeQuickFilterName(model).toLowerCase()}`;
    if (!familyMap.has(familyKey)) {
      familyMap.set(familyKey, {
        key: familyKey,
        label: familyKey.toUpperCase(),
        category: familyKey,
        optionsMap: new Map(),
      });
    }

    const family = familyMap.get(familyKey);
    if (!family.optionsMap.has(optionKey)) {
      family.optionsMap.set(optionKey, {
        key: optionKey,
        label: buildQuickFilterOptionLabel(familyKey, version, model),
        version,
        models: [],
      });
    }
    family.optionsMap.get(optionKey).models.push(model);
  });

  const regularFamilies = [];
  const rareOptions = [];
  familyMap.forEach(family => {
    const options = Array.from(family.optionsMap.values()).sort((a, b) => {
      const versionDiff = (parseFloat(b.version) || 0) - (parseFloat(a.version) || 0);
      if (versionDiff !== 0) return versionDiff;
      return a.label.localeCompare(b.label);
    });
    const nextFamily = {
      key: family.key,
      label: family.label,
      category: family.category,
      options,
    };
    if (options.length <= 1) {
      rareOptions.push(...options);
      return;
    }
    regularFamilies.push(nextFamily);
  });

  if (rareOptions.length > 0) {
    rareOptions.sort((a, b) => a.label.localeCompare(b.label));
    regularFamilies.push({
      key: 'rare',
      label: '冷门组模型',
      category: 'rare',
      options: rareOptions,
    });
  }

  const priority = ['gpt', 'claude', 'gemini', 'deepseek', 'llama', 'minimax', 'grok', 'kimi', 'glm'];
  regularFamilies.sort((a, b) => {
    const idxA = priority.indexOf(a.category);
    const idxB = priority.indexOf(b.category);
    if (idxA !== -1 && idxB !== -1) return idxA - idxB;
    if (idxA !== -1) return -1;
    if (idxB !== -1) return 1;
    if (a.options.length !== b.options.length) return b.options.length - a.options.length;
    return a.label.localeCompare(b.label);
  });

  return regularFamilies;
});

watch(quickFilters, families => {
  const validOptionKeys = new Set();
  (Array.isArray(families) ? families : []).forEach(family => {
    (Array.isArray(family?.options) ? family.options : []).forEach(option => {
      validOptionKeys.add(option.key);
    });
  });
  if (activeQuickFilters.value.length === 0) return;
  activeQuickFilters.value = activeQuickFilters.value.filter(key => validOptionKeys.has(key));
});

const applyActiveQuickFilters = nextOptionKeys => {
  const normalized = Array.from(new Set((Array.isArray(nextOptionKeys) ? nextOptionKeys : []).filter(Boolean)));
  if (normalized.length > 0 && !quickFilterSelectionMode.value) {
    checkedKeys.value = [];
    quickFilterSelectionMode.value = true;
  }
  activeQuickFilters.value = normalized;
  if (normalized.length === 0 && quickFilterSelectionMode.value) {
    checkedKeys.value = [];
    quickFilterSelectionMode.value = false;
  }
};

const toggleQuickFilter = optionKey => {
  const current = new Set(activeQuickFilters.value);
  if (current.has(optionKey)) current.delete(optionKey);
  else current.add(optionKey);
  applyActiveQuickFilters(Array.from(current));
};

const clearQuickFilters = () => {
  applyActiveQuickFilters([]);
};

const isQuickFilterFamilyFullySelected = family => (
  family.options.length > 0 && family.options.every(option => activeQuickFilters.value.includes(option.key))
);

const selectQuickFilterFamily = family => {
  const current = new Set(activeQuickFilters.value);
  if (isQuickFilterFamilyFullySelected(family)) {
    family.options.forEach(option => current.delete(option.key));
  } else {
    family.options.forEach(option => current.add(option.key));
  }
  applyActiveQuickFilters(Array.from(current));
};

const isQuickFilterFamilyActive = family => family.options.some(option => activeQuickFilters.value.includes(option.key));

const getQuickFilterFamilyActiveCount = family => family.options.filter(option => activeQuickFilters.value.includes(option.key)).length;

const activeQuickFilterModelSet = computed(() => {
  const selectedModels = new Set();
  quickFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (!activeQuickFilters.value.includes(option.key)) return;
      option.models.forEach(model => selectedModels.add(model));
    });
  });
  return selectedModels;
});

watch(activeQuickFilterModelSet, currentModelSet => {
  if (!(currentModelSet instanceof Set) || currentModelSet.size === 0) {
    if (quickFilterSelectionMode.value) checkedKeys.value = [];
    return;
  }
  const selectableKeys = collectSelectableModelKeysFromTreeNodes(treeData.value, []);
  checkedKeys.value = selectableKeys.filter(key => currentModelSet.has(getModelNameFromSelectableKey(key)));
});

const activeQuickFilterSummary = computed(() => {
  const labels = [];
  quickFilters.value.forEach(family => {
    family.options.forEach(option => {
      if (activeQuickFilters.value.includes(option.key)) labels.push(option.label);
    });
  });
  if (labels.length === 0) return '';
  if (labels.length <= 3) return `已选: ${labels.join(' / ')}`;
  return `已选: ${labels.slice(0, 3).join(' / ')} +${labels.length - 3}`;
});

const isProviderDiagnosticTreeNode = node => Boolean(node?.isProviderDiagnostic);

const buildProviderFetchReplayText = node => {
  const meta = node?.providerDiagnostic || {};
  const request = meta?.replayRequest || null;
  const replayCandidates = Array.isArray(meta?.replayCandidates) ? meta.replayCandidates.filter(Boolean) : [];
  if ((!request?.url || !request?.headers?.Authorization) && replayCandidates.length > 0) {
    const candidateUrls = replayCandidates.map(item => JSON.stringify(item.url)).join(',\n  ');
    const headersText = JSON.stringify(replayCandidates[0]?.headers || {}, null, 2);
    return [
      `// ${meta.siteName || 'provider'} Token 列表抓取复现`,
      'const targets = [',
      `  ${candidateUrls}`,
      '];',
      `const headers = ${headersText};`,
      'for (const url of targets) {',
      "  const res = await fetch(url, { method: 'GET', headers, credentials: 'include' });",
      "  console.log('url=', url, 'status=', res.status);",
      '  console.log(await res.text());',
      '}',
    ].join('\n');
  }
  if (!request?.url || !request?.headers?.Authorization) {
    return [
      `// ${meta.siteName || 'provider'} 暂无可复现的探测请求`,
      `// 原因: ${meta.userFacingError || meta.rawError || 'unknown'}`,
    ].join('\n');
  }
  return [
    `// ${meta.siteName || 'provider'} 模型发现复现`,
    `const res = await fetch(${JSON.stringify(request.url)}, {`,
    "  method: 'GET',",
    `  headers: ${JSON.stringify(request.headers, null, 2)}`,
    '});',
    "console.log('status=', res.status);",
    'console.log(await res.text());',
  ].join('\n');
};

const buildProviderTraceLogText = node => {
  const meta = node?.providerDiagnostic || {};
  const storageFields = Array.isArray(meta?.storageFields) ? meta.storageFields.filter(Boolean) : [];
  const traceLines = Array.isArray(meta?.traceLines) && meta.traceLines.length ? meta.traceLines : ['(empty)'];
  return [
    `[Provider] ${meta.siteName || '-'}`,
    `[SiteURL] ${meta.siteUrl || '-'}`,
    `[Stage] ${meta.stage || '-'}`,
    `[ExtractionMode] ${meta.extractionMode || '-'}`,
    `[UID] ${meta.uid || '-'}`,
    `[Tokens] total=${Number(meta.totalTokens || 0)} usable=${Number(meta.usableTokens || 0)}`,
    `[TokenEndpoint] ${meta.tokenEndpoint || '-'}`,
    `[StorageOrigin] ${meta.storageOrigin || '-'}`,
    `[StorageFields] ${storageFields.length ? storageFields.join(', ') : '-'}`,
    `[ReasonRaw] ${meta.rawError || '-'}`,
    `[ReasonDisplay] ${meta.userFacingError || '-'}`,
    '',
    '[Trace]',
    ...traceLines,
  ].join('\n');
};

const writeTextToClipboard = async text => {
  const content = String(text || '');
  if (!content) throw new Error('empty_text');
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(content);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = content;
  textarea.setAttribute('readonly', 'readonly');
  textarea.style.position = 'fixed';
  textarea.style.top = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
};

const copyProviderFetchReplay = async node => {
  try {
    await writeTextToClipboard(buildProviderFetchReplayText(node));
    message.success('已复制 fetch 复现语句');
  } catch (error) {
    message.error(error?.message || '复制 fetch 复现失败');
  }
};

const copyProviderTraceLog = async node => {
  try {
    await writeTextToClipboard(buildProviderTraceLogText(node));
    message.success('已复制调研 trace 日志');
  } catch (error) {
    message.error(error?.message || '复制 trace 日志失败');
  }
};

const getRecordBySiteCacheKey = siteCacheKey => records.value.find(item => item.siteCacheKey === siteCacheKey) || null;

const restoreSelectedToBatch = async () => {
  if (!selectedSiteCacheKeys.value.length) {
    message.warning('请至少勾选一个站点');
    return;
  }
  writePendingSiteRestore(selectedSiteCacheKeys.value);
  await router.push('/');
  message.success(`已将 ${selectedSiteCacheKeys.value.length} 个站点恢复到批量检测`);
};

const selectAllNodes = () => {
  checkedKeys.value = collectSelectableModelKeysFromTreeNodes(treeData.value, []);
};

const unselectAllNodes = () => {
  checkedKeys.value = [];
};

const selectChatModelsOnly = () => {
  const notChatPattern = /(bge|stabilityai|dall|mj|stable|flux|video|midjourney|stable-diffusion|playground|swap_face|tts|whisper|text|emb|luma|vidu|pdf|suno|pika|chirp|domo|runway|cogvideo|babbage|davinci|gpt-4o-realtime)/i;
  const filteredKeys = [];
  const childKeys = collectSelectableModelKeysFromTreeNodes(treeData.value, []);
  childKeys.forEach(k => {
    const parts = k.split('|');
    const model = parts[2];
    if (!notChatPattern.test(model) && !/(image|audio|video|music|pdf|flux|suno|embed)/i.test(model)) {
      filteredKeys.push(k);
    }
  });
  checkedKeys.value = filteredKeys;
};

const refreshOne = async record => {
  try {
    const refreshedSite = await refreshCachedSiteTokens(record);
    mergeExtractedSitesIntoTempCache([refreshedSite], {
      importSource: 'site_cache_refresh',
      refreshedAt: Date.now(),
    });
    mergeExtractedSitesIntoCache([refreshedSite], {
      importSource: 'site_cache_refresh',
      refreshedAt: Date.now(),
    });
    reloadRecords();
    message.success(`已刷新 ${record.siteName}`);
  } catch (error) {
    message.error(error?.message || '刷新失败');
  }
};

const appendCustomSk = record => {
  const raw = window.prompt('请输入一个或多个 sk，支持换行、空格、逗号分隔');
  if (!raw) return;
  appendCustomKeysToSiteCache(record.siteCacheKey, raw);
  reloadRecords();
  message.success('自定义 SK 已追加');
};

const toggleDisabled = record => {
  setSiteCacheDisabled(record.siteCacheKey, !record.disabled);
  reloadRecords();
  message.success(record.disabled ? '站点已激活' : '站点已禁用');
};

const editNote = record => {
  const raw = window.prompt(`请输入 ${SITE_NOTE_MAX_LENGTH} 个字以内备注`, String(record.note || ''));
  if (raw == null) return;
  updateSiteCacheNote(record.siteCacheKey, raw);
  reloadRecords();
  message.success('备注已更新');
};

const removeRecord = record => {
  deleteSiteCacheRecord(record.siteCacheKey);
  reloadRecords();
  message.success('站点缓存已删除');
};

const refreshOneByNode = async node => {
  const record = getRecordBySiteCacheKey(node?.siteCacheKey);
  if (!record) return;
  await refreshOne(record);
};

const appendCustomSkByNode = node => {
  const record = getRecordBySiteCacheKey(node?.siteCacheKey);
  if (!record) return;
  appendCustomSk(record);
};

const removeManualTokenByNode = node => {
  const siteCacheKey = String(node?.siteCacheKey || '').trim();
  const tokenKey = extractTokenKeyFromNode(node);
  if (!siteCacheKey || !tokenKey) return;
  removeCustomKeyFromSiteCache(siteCacheKey, tokenKey);
  reloadRecords();
  message.success('手动添加的 key 已删除');
};

const toggleDisabledByNode = node => {
  const record = getRecordBySiteCacheKey(node?.siteCacheKey);
  if (!record) return;
  toggleDisabled(record);
};

const editNoteByNode = node => {
  const record = getRecordBySiteCacheKey(node?.siteCacheKey);
  if (!record) return;
  editNote(record);
};

const removeRecordByNode = node => {
  const record = getRecordBySiteCacheKey(node?.siteCacheKey);
  if (!record) return;
  removeRecord(record);
};

const goBackToImport = async () => {
  writePendingBatchStart({ autoStart: false });
  await router.push('/');
};

const startBatchCheckFromSiteManagement = async () => {
  const modelKeys = selectedModelKeys.value;
  if (!modelKeys.length) {
    message.warning('请至少勾选一个模型进行测试');
    return;
  }
  if (!selectedSiteCacheKeys.value.length) {
    message.warning('当前没有可恢复的站点缓存');
    return;
  }
  writePendingSiteRestore(selectedSiteCacheKeys.value);
  writePendingBatchStart({
    autoStart: true,
    checkedKeys: modelKeys,
    batchConcurrency: Number(batchConcurrency.value || 25),
    modelTimeout: Number(modelTimeout.value || 15),
  });
  await router.push('/');
};

const openSiteUrl = url => {
  const target = String(url || '').trim();
  if (!target) return;
  window.open(target, '_blank', 'noopener');
};

const selectAllSites = () => {
  checkedKeys.value = [...rootSiteKeys.value];
};

const invertSelectedSites = () => {
  const current = new Set(checkedKeys.value.map(key => String(key || '').trim()));
  checkedKeys.value = rootSiteKeys.value.filter(key => !current.has(key));
};

const handleTreeExpand = keys => {
  expandedKeys.value = Array.isArray(keys) ? [...keys] : [];
};

const handleTreeCheck = keys => {
  checkedKeys.value = Array.isArray(keys) ? [...keys] : [];
};

const handleToggleTheme = () => {
  isDarkMode.value = toggleTheme();
};

const handleSettings = () => {
  showAppSettingsModal.value = true;
};

const handleExperimental = () => {
  showExperimentalFeatures.value = true;
  return;
  message.info('站点管理当前直接复用缓存树浏览视图。');
};

const handleSync = () => {
  reloadRecords();
};

watch(treeData, () => {
  syncExpandedKeys();
  syncCheckedKeys();
});

onMounted(() => {
  isDarkMode.value = document.body.classList.contains('dark-mode');
  reloadRecords();
  const pendingBatchStart = consumePendingBatchStart();
  if (pendingBatchStart) {
    batchConcurrency.value = Number(pendingBatchStart?.batchConcurrency || batchConcurrency.value || 25);
    modelTimeout.value = Number(pendingBatchStart?.modelTimeout || modelTimeout.value || 15);
  }
  window.addEventListener(SITE_CACHE_SYNC_EVENT, handleSync);
});

onBeforeUnmount(() => {
  window.removeEventListener(SITE_CACHE_SYNC_EVENT, handleSync);
});
</script>

<style scoped>
.selection-topbar {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-bottom: 18px;
}

.selection-quick-filters {
  width: 100%;
  min-width: 0;
}

.selection-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.selection-title {
  margin: 0;
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
  color: #ffffff;
}

.selection-action-group {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  margin-left: auto;
}

.quick-filter-toolbar {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 12px;
  min-height: 32px;
  min-width: 0;
  width: 100%;
}

.quick-filter-strip {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  gap: 0;
  width: 100%;
  max-width: 100%;
  border: 1px solid rgba(15, 23, 42, 0.12);
  border-radius: 12px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

.quick-filter-strip > :not(.quick-filter-clear-trigger) {
  min-width: 0;
}

.quick-filter-empty-inline {
  color: #94a3b8;
  font-size: 13px;
  padding: 6px 0;
}

.quick-filter-family-trigger,
.quick-filter-clear-trigger {
  width: 100%;
  border: 0 !important;
  border-right: 1px solid rgba(15, 23, 42, 0.08) !important;
  border-bottom: 1px solid rgba(15, 23, 42, 0.08) !important;
  border-radius: 0 !important;
  box-shadow: none !important;
  height: 40px;
  justify-content: center;
  padding: 0 20px !important;
}

.quick-filter-family-trigger:hover,
.quick-filter-clear-trigger:hover {
  background: rgba(22, 119, 255, 0.06) !important;
}

.quick-filter-clear-trigger.ant-btn[disabled],
.quick-filter-clear-trigger.ant-btn[disabled]:hover {
  background: rgba(148, 163, 184, 0.08) !important;
  color: rgba(148, 163, 184, 0.9) !important;
}

.quick-filter-family-count {
  margin-left: 6px;
  font-size: 11px;
  opacity: 0.75;
}

.quick-filter-summary {
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.quick-filter-family-panel {
  width: min(420px, 56vw);
  max-width: 420px;
}

.quick-filter-family-panel-title {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.quick-filter-option-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.quick-filter-family-select-all {
  border: 2px solid #8b5e3c !important;
  color: #8b5e3c !important;
  background: #fffaf4 !important;
  box-shadow: none !important;
  font-weight: 600;
}

.batch-hero {
  position: relative;
  overflow: hidden;
  margin-bottom: 6px;
  padding: 10px 12px 10px;
  border-radius: 18px;
  border: 1px solid rgba(90, 117, 79, 0.1);
  background:
    radial-gradient(circle at top right, rgba(255, 231, 161, 0.38), transparent 36%),
    radial-gradient(circle at left center, rgba(204, 228, 184, 0.34), transparent 32%),
    linear-gradient(145deg, rgba(255, 252, 244, 0.95), rgba(244, 249, 236, 0.9));
  box-shadow:
    0 36px 90px rgba(98, 119, 84, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.84);
}

.batch-hero-compact {
  padding-bottom: 12px;
}

.batch-hero-head {
  position: relative;
  z-index: 1;
}

.batch-hero-copy {
  max-width: 100%;
}

.batch-hero-kicker {
  margin: 0 0 4px;
  color: #8a936f;
  font-size: 8px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.batch-hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 6px;
}

.batch-hero-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.55);
  border: 1px solid rgba(90, 117, 79, 0.08);
  color: #6e7c64;
  font-size: 9px;
  font-weight: 600;
}

.page-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 0;
  flex-wrap: wrap;
}

.page-title-block {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.page-title {
  margin: 0;
  text-align: left;
  color: #31422f;
  font: 700 clamp(20px, 2.4vw, 30px)/1 Georgia, 'Times New Roman', serif;
  letter-spacing: -0.03em;
}

.page-subtitle {
  margin: 0;
  color: #72806c;
  font-size: 10px;
  line-height: 1.2;
}

.batch-wrapper {
  min-height: calc(var(--vh, 1vh) * 100);
  padding: 0;
  overflow: hidden;
}

.batch-page-container {
  max-width: 100% !important;
  padding: 8px 8px 0 !important;
  margin: 0 auto !important;
}

.batch-shell {
  width: 100%;
  min-height: calc(var(--vh, 1vh) * 100);
  position: relative;
  isolation: isolate;
  overflow: hidden;
}

.batch-forest-scene {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
  background:
    radial-gradient(circle at 16% 18%, rgba(164, 213, 120, 0.14), transparent 24%),
    radial-gradient(circle at 84% 14%, rgba(255, 213, 116, 0.14), transparent 22%),
    linear-gradient(180deg, rgba(8, 18, 12, 0.14) 0%, rgba(8, 20, 13, 0.34) 42%, rgba(6, 16, 10, 0.62) 100%),
    url('/forest-batch-bg-v2.png') center center / cover no-repeat;
  opacity: 0.92;
}

.forest-mist,
.forest-path-glow,
.forest-firegrass,
.forest-slime {
  position: absolute;
}

.forest-mist {
  top: 8%;
  width: 34%;
  height: 44%;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(210, 255, 232, 0.12) 0%, rgba(210, 255, 232, 0.02) 56%, transparent 74%);
  filter: blur(12px);
}

.forest-mist-left {
  left: -10%;
}

.forest-mist-right {
  right: -8%;
  top: 12%;
}

.forest-path-glow {
  left: 50%;
  bottom: -12%;
  width: min(460px, 42vw);
  height: 42%;
  transform: translateX(-50%);
  background:
    radial-gradient(ellipse at center bottom, rgba(255, 214, 126, 0.22) 0%, rgba(212, 255, 182, 0.12) 24%, rgba(30, 58, 33, 0) 72%);
  clip-path: polygon(47% 100%, 53% 100%, 65% 76%, 60% 56%, 67% 33%, 57% 0, 43% 0, 33% 33%, 40% 56%, 35% 76%);
  filter: blur(8px);
  opacity: 0.9;
}

.forest-firegrass {
  bottom: -4px;
  width: 188px;
  height: 122px;
  background: url('/forest-firegrass-sprite-v2.png') left bottom / auto 100% no-repeat;
  filter: drop-shadow(0 6px 12px rgba(18, 38, 22, 0.2));
  opacity: 0.98;
}

.firegrass-left {
  left: 8px;
}

.firegrass-right {
  right: 8px;
  transform: scaleX(-1);
  transform-origin: center bottom;
}

.forest-slime {
  bottom: 26px;
  width: 26px;
  height: 22px;
  border-radius: 58% 58% 46% 46%;
  background:
    radial-gradient(circle at 36% 36%, rgba(255,255,255,0.9) 0 10%, transparent 11%),
    radial-gradient(circle at 64% 36%, rgba(255,255,255,0.9) 0 10%, transparent 11%),
    radial-gradient(circle at 40% 40%, rgba(20,34,21,0.86) 0 3%, transparent 4%),
    radial-gradient(circle at 60% 40%, rgba(20,34,21,0.86) 0 3%, transparent 4%),
    radial-gradient(circle at 50% 72%, rgba(18,72,42,0.44) 0 14%, transparent 15%),
    linear-gradient(180deg, rgba(177, 255, 149, 0.98), rgba(70, 177, 88, 0.94));
  box-shadow:
    inset 0 2px 0 rgba(255,255,255,0.45),
    0 10px 16px rgba(14, 38, 18, 0.24),
    0 0 10px rgba(154, 255, 142, 0.18);
}

.slime-a { left: 44%; }
.slime-b { left: 51%; width: 20px; height: 17px; }
.slime-c { left: 57%; width: 18px; height: 15px; }

.batch-page-content {
  background: transparent;
  border-radius: 0;
  box-shadow: none;
  padding: 2px;
  min-height: calc(var(--vh, 1vh) * 100);
  position: relative;
  z-index: 1;
}

.step-container {
  margin-top: 6px;
}

.tree-wrapper {
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(90, 117, 79, 0.12);
  border-radius: 20px;
  padding: 14px;
  margin-bottom: 20px;
  max-height: 420px;
  overflow-y: auto;
  box-shadow: 0 16px 36px rgba(98, 119, 84, 0.08);
  contain: layout paint;
}

.batch-settings-label {
  font-size: 14px;
  color: #ffffff;
}

.settings-action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  border-top: 1px solid rgba(90, 117, 79, 0.12);
  padding-top: 15px;
}

.batch-hero-motion {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.leaf,
.grass {
  position: absolute;
  opacity: 0.42;
}

.leaf {
  width: 10px;
  height: 20px;
  border-radius: 70% 0 70% 0;
  background: linear-gradient(180deg, rgba(170, 202, 127, 0.7), rgba(96, 131, 75, 0.42));
  filter: blur(0.2px);
  transform-origin: center bottom;
}

.leaf-a { top: 24%; right: 18%; }
.leaf-b { top: 42%; right: 8%; width: 9px; height: 18px; }
.leaf-c { bottom: 28%; left: 9%; width: 10px; height: 20px; }
.leaf-d { bottom: 18%; right: 28%; width: 8px; height: 14px; }

.grass {
  bottom: -10px;
  width: 2px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(121, 157, 96, 0), rgba(121, 157, 96, 0.58));
}

.grass-a { left: 8%; height: 38px; }
.grass-b { left: 11%; height: 30px; }
.grass-c { right: 12%; height: 34px; }

.custom-tree-node {
  font-size: 14px;
}

.tree-node-grey { color: #999; opacity: 0.7; }

.tree-node-pending-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: 10px;
  color: #1677ff;
  font-size: 12px;
}

.custom-tree-node-wrapper {
  display: flex !important;
  align-items: center;
  width: 100%;
}

.tree-provider-node-wrapper {
  gap: 10px;
}

.provider-tree-label {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  flex: 1;
}

.provider-tree-link {
  border: none;
  background: transparent;
  padding: 0;
  margin: 0;
  color: #1677ff;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.4;
}

.provider-tree-link:hover {
  text-decoration: underline;
}

.provider-tree-link.is-grey {
  color: #8c8c8c;
}

.site-tree-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  opacity: 0.12;
  transition: opacity 0.2s ease;
}

.provider-tree-actions {
  margin-left: auto;
  opacity: 0.12;
  transition: opacity 0.2s ease;
}

.tree-provider-node-wrapper:hover .provider-tree-actions {
  opacity: 1;
}

.tree-provider-node-wrapper:hover .site-tree-actions {
  opacity: 1;
}

.site-tree-action-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: rgba(22, 119, 255, 0.08);
  color: #1677ff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.site-tree-action-btn:hover {
  background: rgba(22, 119, 255, 0.16);
}

.site-tree-action-btn.is-danger {
  background: rgba(255, 77, 79, 0.08);
  color: #ff4d4f;
}

.site-tree-note-badge {
  display: inline-flex;
  align-items: center;
  max-width: 120px;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(245, 208, 112, 0.2);
  color: #8a5a00;
  font-size: 11px;
  line-height: 20px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.site-tree-inline-tag {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  border-radius: 999px;
  background: rgba(22, 119, 255, 0.12);
  color: #1677ff;
  font-size: 11px;
  line-height: 20px;
  user-select: none;
}

.site-tree-inline-delete-btn {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: rgba(255, 77, 79, 0.08);
  color: #ff4d4f;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.site-tree-inline-delete-btn:hover {
  background: rgba(255, 77, 79, 0.16);
}

.provider-diagnostic-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 42px;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(140, 140, 140, 0.14);
  color: #8c8c8c;
  font-size: 12px;
  cursor: pointer;
  user-select: none;
}

.provider-diagnostic-menu {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 180px;
}

.tree-site-disabled {
  color: #8c8c8c;
  text-decoration: line-through;
  opacity: 0.76;
}

:deep(.ant-tree-node-content-wrapper) {
  width: 100%;
}

:deep(body.dark-mode) .batch-hero {
  border-color: rgba(160, 189, 144, 0.12);
  background:
    radial-gradient(circle at top right, rgba(179, 147, 67, 0.24), transparent 34%),
    radial-gradient(circle at left center, rgba(104, 149, 88, 0.2), transparent 34%),
    linear-gradient(145deg, rgba(24, 38, 27, 0.95), rgba(35, 53, 39, 0.92));
  box-shadow:
    0 34px 90px rgba(0, 0, 0, 0.28),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:deep(body.dark-mode) .batch-forest-scene {
  background:
    radial-gradient(circle at 18% 18%, rgba(92, 161, 113, 0.14), transparent 24%),
    radial-gradient(circle at 82% 15%, rgba(255, 206, 104, 0.1), transparent 22%),
    linear-gradient(180deg, rgba(4, 10, 7, 0.42) 0%, rgba(4, 10, 7, 0.62) 42%, rgba(2, 6, 4, 0.86) 100%),
    url('/forest-batch-bg-v2.png') center center / cover no-repeat;
}

:deep(body.dark-mode) .page-title,
:deep(body.dark-mode) .selection-title {
  color: #eef5e6;
}

:deep(body.dark-mode) .page-subtitle,
:deep(body.dark-mode) .batch-hero-tag {
  color: #b8c8b2;
}

:deep(body.dark-mode) .batch-hero-tag,
:deep(body.dark-mode) .tree-wrapper {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(160, 189, 144, 0.12);
}

:deep(body.dark-mode) .site-tree-action-btn {
  background: rgba(172, 199, 151, 0.12);
  color: #dfead8;
}

:deep(body.dark-mode) .site-tree-action-btn:hover {
  background: rgba(172, 199, 151, 0.22);
}

:deep(body.dark-mode) .site-tree-action-btn.is-danger {
  background: rgba(255, 77, 79, 0.16);
  color: #ffb6b7;
}

:deep(body.dark-mode) .site-tree-note-badge {
  background: rgba(245, 208, 112, 0.18);
  color: #ffd98b;
}

:deep(body.dark-mode) .site-tree-inline-tag {
  background: rgba(92, 164, 255, 0.18);
  color: #a9d0ff;
}

:deep(body.dark-mode) .site-tree-inline-delete-btn {
  background: rgba(255, 77, 79, 0.16);
  color: #ffb6b7;
}

@media (max-width: 620px) {
  .batch-hero {
    padding: 12px 10px;
  }

  .page-title-row {
    gap: 14px;
  }

  .selection-header-row {
    align-items: flex-start;
  }

  .selection-title,
  .selection-action-group {
    white-space: normal;
  }

  .selection-action-group {
    margin-left: 0;
    justify-content: flex-start;
  }
}
</style>
