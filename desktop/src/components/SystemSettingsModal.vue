<template>
  <a-modal
    :open="open"
    title="系统设置"
    :footer="null"
    :width="600"
    :centered="true"
    :destroyOnClose="true"
    @cancel="emit('update:open', false)"
  >
    <a-tabs>
      <a-tab-pane key="general" tab="常规设置">
        <div class="settings-tab-content">
          <p class="settings-section-title-row">
            <b>界面主题</b>
            <a-tooltip trigger="hover" placement="topLeft" overlayClassName="settings-info-tooltip">
              <template #title>
                <div class="settings-info-tooltip-copy">
                  <div>同一套主题会同步应用到批量检测、站点管理、密钥管理。</div>
                  <div>`盖亚暗黑` 在现有深色底座上进一步压低明度，并把高光收敛到岩层青苔色系。</div>
                </div>
              </template>
              <button type="button" class="settings-info-icon" aria-label="界面主题说明">i</button>
            </a-tooltip>
          </p>
          <div class="theme-mode-grid">
            <button
              v-for="option in themeModeOptions"
              :key="option.value"
              type="button"
              class="theme-mode-card"
              :class="[`theme-mode-card-${option.value}`, { 'is-active': themeMode === option.value }]"
              @click="handleThemeModeSelection(option.value)"
            >
              <span class="theme-mode-swatches" aria-hidden="true">
                <span class="theme-mode-swatch theme-mode-swatch-a"></span>
                <span class="theme-mode-swatch theme-mode-swatch-b"></span>
                <span class="theme-mode-swatch theme-mode-swatch-c"></span>
              </span>
              <span class="theme-mode-copy">
                <strong>{{ option.label }}</strong>
                <small>{{ option.description }}</small>
              </span>
            </button>
          </div>
          <p class="settings-section-title-row">
            <b>{{ tr('界面语言') }}</b>
            <a-tooltip trigger="hover" placement="topLeft" overlayClassName="settings-info-tooltip">
              <template #title>
                <div class="settings-info-tooltip-copy">
                  <div>{{ tr('中文为默认原文；切换后会按映射文件翻译，未映射文本会自动保留原文。') }}</div>
                </div>
              </template>
              <button type="button" class="settings-info-icon" :aria-label="tr('界面语言说明')">i</button>
            </a-tooltip>
          </p>
          <a-select
            class="settings-language-select"
            :value="languageMode"
            :options="languageOptions"
            @change="handleLanguageSelection"
          />
          <p class="settings-section-title-row">
            <b>代理模式</b>
            <a-tooltip trigger="hover" placement="topLeft" overlayClassName="settings-info-tooltip">
              <template #title>
                <div class="settings-info-tooltip-copy">
                  <div>默认使用系统代理，显式集成到桌面端 Go 后端请求链路。</div>
                  <div>自定义格式兼容：`socks5://`、`socks5h://`、`http://`、`https://`，也支持 `user:pass@host:port`。</div>
                  <div>浏览器模式下仅保存配置，不会接管浏览器自身网络栈；桌面端 EXE / Wails 才会真正作用于后端请求。</div>
                </div>
              </template>
              <button type="button" class="settings-info-icon" aria-label="代理模式说明">i</button>
            </a-tooltip>
          </p>
          <a-space direction="vertical" style="width: 100%; margin-bottom: 16px;">
            <a-radio-group :value="proxyDraft.mode" @change="handleProxyModeChange">
              <a-radio value="system">系统代理</a-radio>
              <a-radio value="direct">无代理</a-radio>
              <a-radio value="custom">自定义 socks5 / http / https 代理</a-radio>
            </a-radio-group>
            <div v-if="proxyDraft.mode === 'custom'" class="proxy-custom-row">
              <a-input
                v-model:value="proxyDraft.customUrl"
                placeholder="例如 socks5://127.0.0.1:7890 或 http://127.0.0.1:7890"
                @pressEnter="applyProxySettings"
              />
              <a-button type="primary" :loading="proxySaving" @click="applyProxySettings">应用代理</a-button>
            </div>
          </a-space>

          <p class="settings-section-title-row">
            <b>桌面端提取方式</b>
            <a-tooltip trigger="hover" placement="topLeft" overlayClassName="settings-info-tooltip">
              <template #title>
                <div class="settings-info-tooltip-copy">
                  <div>Profile 文件模式：从本机 Chrome 默认 Profile 的本地存储文件读取登录态，例如 auth_token、auth_user、refresh_token，再直接请求站点 Token 列表。不主动拉起受控浏览器。</div>
                  <div>CDP 重开模式：检测失败站点后，打开或重启 Chrome/Edge 受控浏览器，附着到 CDP 会话，在真实浏览器上下文里读取登录态并轮询抓取 Token。会使用 shadow / remote debugging 这套流程。</div>
                  <div>桌面端会严格按所选模式执行，不自动切换到另一种模式。</div>
                </div>
              </template>
              <button type="button" class="settings-info-icon" aria-label="桌面端提取方式说明">i</button>
            </a-tooltip>
          </p>
          <a-space direction="vertical" style="width: 100%; margin-bottom: 16px;">
            <a-radio-group :value="desktopTokenSourceMode" :disabled="!isWailsRuntime" @change="handleDesktopTokenSourceModeChange">
              <a-radio value="profile_file">Profile 文件</a-radio>
              <a-radio value="cdp_restart">CDP 重开模式</a-radio>
            </a-radio-group>
            <div v-if="!isWailsRuntime" class="settings-muted-text">该设置仅在桌面端 EXE 生效，浏览器模式仍走前端直连。</div>
            <div v-else-if="desktopTokenSourceMode === 'profile_file' && !effectiveChromeProfileAuthAvailable" class="settings-muted-text">当前桌面端尚未暴露 Profile 文件提取接口，无法使用该模式。</div>
          </a-space>

          <p><b>界面选项</b></p>
          <a-space direction="vertical" style="width: 100%;">
            <div class="settings-switch-row">
              <span>自动展开/折叠树形结果</span>
              <a-switch :checked="treeExpanded" @update:checked="handleTreeExpandedChange" />
            </div>
          </a-space>

          <a-divider />
          <div class="settings-version-text">
            {{ appLabel }}
          </div>
        </div>
      </a-tab-pane>
      <a-tab-pane key="gateway" tab="网关配置">
        <div class="settings-tab-content">
          <a-space direction="vertical" style="width: 100%; margin-bottom: 16px;">
            <div class="context-compression-row">
              <div class="context-compression-copy">
                <span>上下文自动压缩：{{ contextAutoCompressionDraft.thresholdK }}（K）</span>
                <small>强制要求多少上下文即触发压缩</small>
              </div>
              <a-switch
                :checked="contextAutoCompressionDraft.enabled"
                @update:checked="handleContextAutoCompressionEnabledChange"
              />
            </div>
            <a-input-number
              v-model:value="contextAutoCompressionDraft.thresholdK"
              :disabled="!contextAutoCompressionDraft.enabled"
              :min="1"
              :max="4096"
              :step="1"
              addon-after="K"
              style="width: 180px;"
              @change="handleContextAutoCompressionThresholdChange"
            />
          </a-space>

          <p class="settings-section-title settings-section-title-spaced settings-section-title-row">
            <b>User-Agent 映射</b>
            <a-tooltip trigger="hover" placement="topLeft" overlayClassName="settings-info-tooltip">
              <template #title>
                <div class="settings-info-tooltip-copy">
                  <div>支持纯 UA 文本，也支持多行或分号分隔的 `Header: Value`。</div>
                  <div>默认规则：`gpt` 会注入 Codex Desktop 的 `Originator` 和 `User-Agent`；`claude` 会注入 `claude-cli`、`X-App`、`anthropic-*` 与 `X-Stainless-*` 头。</div>
                </div>
              </template>
              <button type="button" class="settings-info-icon" aria-label="User-Agent 映射说明">i</button>
            </a-tooltip>
          </p>
          <div class="ua-mapping-card">
            <div class="ua-mapping-head">
              <div class="ua-mapping-caption">按模型名包含匹配，命中后把右侧内容作为请求头块应用到快测请求。</div>
              <a-button size="small" @click="addUserAgentMappingRow">新增一行</a-button>
            </div>
            <div class="ua-mapping-grid ua-mapping-grid-head">
              <div>Model包含</div>
              <div>目标UA</div>
              <div></div>
            </div>
            <div
              v-for="(mapping, index) in userAgentMappings"
              :key="`ua-mapping-${index}`"
              class="ua-mapping-grid"
            >
              <a-input
                v-model:value="mapping.modelContains"
                placeholder="例如 gpt"
                @blur="void saveUserAgentMappingsDraft()"
                @pressEnter="void saveUserAgentMappingsDraft()"
              />
              <a-textarea
                v-model:value="mapping.targetUA"
                :rows="3"
                placeholder="可填单个 UA，或多行 Header: Value"
                @blur="void saveUserAgentMappingsDraft()"
              />
              <a-button
                danger
                size="small"
                :disabled="userAgentMappings.length <= 1"
                @click="removeUserAgentMappingRow(index)"
              >
                删除
              </a-button>
            </div>
          </div>
        </div>
      </a-tab-pane>
      <a-tab-pane key="portable" tab="本地绿色化">
        <div class="portable-settings-card">
          <div class="portable-settings-copy">
            <div class="portable-settings-title">本地绿色化</div>
            <div class="portable-settings-desc">封包是将本应用数据绿色化到程序目录 `backup`，解包是从 `backup` 解包恢复本程序所有数据。</div>
            <div class="portable-settings-hint">当前会处理运行时目录数据与前端 localStorage 快照。为保证当前窗口状态一致，解包完成后会自动刷新页面。</div>
            <div v-if="portableSettingsMeta" class="portable-settings-meta">{{ portableSettingsMeta }}</div>
            <div v-if="!isWailsRuntime" class="portable-settings-warning">该功能仅在桌面端 EXE / Wails 环境可用。</div>
          </div>
          <div class="portable-settings-actions">
            <a-button type="primary" size="large" :loading="portablePacking" :disabled="!isWailsRuntime || portableUnpacking" @click="packagePortableData">
              封包
            </a-button>
            <a-button size="large" :loading="portableUnpacking" :disabled="!isWailsRuntime || portablePacking" @click="unpackPortableData">
              解包
            </a-button>
          </div>
        </div>
      </a-tab-pane>
      <a-tab-pane key="logs" tab="日志">
        <div class="settings-tab-content">
          <div class="settings-log-head">
            <div class="settings-log-title">运行日志</div>
            <a-button size="small" @click="loadDesktopLogs" :loading="desktopLogsLoading" :disabled="!isDesktopLogAvailable">
              刷新
            </a-button>
          </div>
          <div v-if="!isWailsRuntime || !isDesktopLogAvailable" class="settings-muted-text">
            当前环境不支持桌面端日志查看，请在 EXE 中使用。
          </div>
          <template v-else>
            <a-space direction="vertical" style="width: 100%;">
              <div>
                <div class="settings-field-caption">分组</div>
                <a-radio-group v-model:value="selectedDesktopLogGroup" size="small">
                  <a-radio-button
                    v-for="group in desktopLogGroups"
                    :key="group.key"
                    :value="group.key"
                    :class="{ 'settings-log-group-error': group.key === ERROR_SUMMARY_GROUP_KEY }"
                  >
                    {{ group.label }}
                  </a-radio-button>
                </a-radio-group>
              </div>
              <div>
                <div class="settings-field-caption">文件</div>
                <a-select
                  v-model:value="selectedDesktopLogPath"
                  style="width: 100%;"
                  placeholder="请选择日志文件"
                  :options="currentDesktopLogGroupFiles.map(file => ({
                    label: `${file.name} · ${file.sourceLabel} · ${formatLogSize(file.size)} · ${formatLogTimestamp(file.updatedAt)}`,
                    value: file.path,
                  }))"
                  @change="loadDesktopLogContent"
                />
              </div>
              <div v-if="currentDesktopLogFileMeta" class="settings-file-meta">
                <div>来源：{{ currentDesktopLogFileMeta.sourceLabel }}</div>
                <div>大小：{{ formatLogSize(currentDesktopLogFileMeta.size) }}</div>
                <div>更新时间：{{ formatLogTimestamp(currentDesktopLogFileMeta.updatedAt) }}</div>
              </div>
              <a-spin :spinning="desktopLogsLoading || desktopLogContentLoading">
                <a-textarea
                  :value="selectedDesktopLogContent"
                  :rows="18"
                  readonly
                  style="font-family: Consolas, 'Courier New', monospace;"
                  placeholder="当前分组下暂无日志内容"
                />
              </a-spin>
            </a-space>
          </template>
        </div>
      </a-tab-pane>
    </a-tabs>
  </a-modal>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { message } from 'ant-design-vue';
import { isProbablyWailsRuntime } from '../utils/runtimeApi.js';
import { isDesktopLogBridgeAvailable, listDesktopLogFiles, readDesktopLogFile } from '../utils/desktopLogBridge.js';
import { isChromeProfileAuthBridgeAvailable } from '../utils/profileAuthBridge.js';
import { getAdvancedProxyConfig, setAdvancedProxyConfig } from '../utils/advancedProxyBridge.js';
import {
  getOutboundProxyConfig,
  loadContextAutoCompressionConfig,
  loadUserAgentMappings,
  normalizeContextAutoCompressionConfig,
  normalizeDesktopTokenSourceMode,
  normalizeOutboundProxyConfig,
  saveContextAutoCompressionConfig,
  saveDesktopTokenSourceMode,
  saveTreeExpandedSetting,
  saveUserAgentMappings,
  setOutboundProxyConfig,
} from '../utils/systemSettings.js';
import {
  applyPortableLocalStorageSnapshot,
  snapshotPortableLocalStorage,
} from '../utils/portableSnapshot.js';
import {
  applyThemeMode,
  getStoredThemeMode,
  THEME_MODE_OPTIONS,
} from '../utils/theme.js';
import {
  applyLanguage,
  getLanguageOptions,
  getStoredLanguage,
  LANGUAGE_CHANGE_EVENT,
  tr,
} from '../i18n/runtime.js';

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  treeExpanded: {
    type: Boolean,
    default: true,
  },
  desktopTokenSourceMode: {
    type: String,
    default: 'profile_file',
  },
  isChromeProfileAuthAvailable: {
    type: Boolean,
    default: false,
  },
  appName: {
    type: String,
    default: 'All API Deck',
  },
  appVersion: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['update:open', 'update:treeExpanded', 'update:desktopTokenSourceMode']);

const isWailsRuntime = isProbablyWailsRuntime();
const effectiveChromeProfileAuthAvailable = computed(() => (
  Boolean(props.isChromeProfileAuthAvailable) || isChromeProfileAuthBridgeAvailable()
));
const portablePacking = ref(false);
const portableUnpacking = ref(false);
const portableSettingsMeta = ref('');
const proxySaving = ref(false);
const proxyDraft = reactive(normalizeOutboundProxyConfig({}));
const desktopLogsLoading = ref(false);
const desktopLogContentLoading = ref(false);
const desktopLogFiles = ref([]);
const selectedDesktopLogGroup = ref('');
const selectedDesktopLogPath = ref('');
const selectedDesktopLogContent = ref('');
const userAgentMappings = ref(loadUserAgentMappings());
const contextAutoCompressionDraft = reactive(loadContextAutoCompressionConfig());
const themeMode = ref(getStoredThemeMode());
const themeModeOptions = THEME_MODE_OPTIONS;
const languageMode = ref(getStoredLanguage());
const languageOptions = computed(() =>
  getLanguageOptions().map(option => ({
    value: option.value,
    label: `${tr(option.label, languageMode.value)} · ${option.nativeLabel}`,
  }))
);
const ERROR_SUMMARY_GROUP_KEY = '__error_summary__';
const ERROR_SUMMARY_FILE_PATH = '__error_summary_all__';
const ERROR_SUMMARY_GROUP_LABEL = '错误汇总';
const desktopLogErrorSummaryCache = ref({ fingerprint: '', content: '' });

const isDesktopLogAvailable = computed(() => isDesktopLogBridgeAvailable());

const desktopLogGroups = computed(() => {
  const files = Array.isArray(desktopLogFiles.value) ? desktopLogFiles.value : [];
  const groupMap = new Map();
  files.forEach(file => {
    const key = String(file?.groupKey || 'other').trim() || 'other';
    const label = String(file?.groupLabel || '其他日志').trim() || '其他日志';
    if (!groupMap.has(key)) {
      groupMap.set(key, { key, label, files: [] });
    }
    groupMap.get(key).files.push(file);
  });
  const groups = Array.from(groupMap.values());
  if (files.length) {
    groups.unshift({
      key: ERROR_SUMMARY_GROUP_KEY,
      label: ERROR_SUMMARY_GROUP_LABEL,
      files: [{
        path: ERROR_SUMMARY_FILE_PATH,
        name: '全部错误',
        sourceLabel: '聚合',
        size: 0,
        updatedAt: Date.now(),
      }],
    });
  }
  return groups;
});

const currentDesktopLogGroupFiles = computed(() => {
  const group = desktopLogGroups.value.find(item => item.key === String(selectedDesktopLogGroup.value || '').trim());
  return Array.isArray(group?.files) ? group.files : [];
});

const currentDesktopLogFileMeta = computed(() => {
  const targetPath = String(selectedDesktopLogPath.value || '').trim();
  return currentDesktopLogGroupFiles.value.find(file => String(file?.path || '').trim() === targetPath) || null;
});

const appLabel = computed(() => props.appVersion
  ? `${props.appName} v${props.appVersion}`
  : props.appName);

watch(() => props.open, open => {
  if (!open) return;
  themeMode.value = getStoredThemeMode();
  languageMode.value = getStoredLanguage();
  userAgentMappings.value = loadUserAgentMappings();
  Object.assign(contextAutoCompressionDraft, loadContextAutoCompressionConfig());
  void loadProxyDraft();
  if (isWailsRuntime) {
    void loadDesktopLogs();
  }
});

function syncLanguageMode(event) {
  languageMode.value = getStoredLanguage() || event?.detail?.language || languageMode.value;
}

if (typeof window !== 'undefined') {
  window.addEventListener(LANGUAGE_CHANGE_EVENT, syncLanguageMode);
}

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener(LANGUAGE_CHANGE_EVENT, syncLanguageMode);
  }
});

watch(selectedDesktopLogGroup, groupKey => {
  const files = desktopLogGroups.value.find(group => group.key === groupKey)?.files || [];
  const nextPath = files.find(file => String(file?.path || '') === selectedDesktopLogPath.value)?.path
    || files[0]?.path
    || '';
  selectedDesktopLogPath.value = nextPath;
  if (nextPath) {
    void loadDesktopLogContent(nextPath);
  } else {
    selectedDesktopLogContent.value = '';
  }
});

function getPortableErrorMessage(error, fallback) {
  if (!error) return fallback;
  if (typeof error === 'string') return error.trim() || fallback;
  const direct = String(error?.message || error?.error || '').trim();
  if (direct) return direct;
  try {
    const serialized = JSON.stringify(error);
    if (serialized && serialized !== '{}') return serialized;
  } catch {}
  return String(error).trim() || fallback;
}

async function packagePortableData() {
  const packer = window?.go?.main?.App?.PackagePortableData;
  if (typeof packer !== 'function') {
    message.error('当前环境不支持本地绿色化封包');
    return;
  }
  portablePacking.value = true;
  try {
    const result = await packer(JSON.stringify(await snapshotPortableLocalStorage()));
    portableSettingsMeta.value = `封包完成：${result?.backupDir || 'backup'}，localStorage ${Number(result?.localStorageKeyCount || 0)} 项`;
    message.success('已完成本地绿色化封包');
  } catch (error) {
    message.error(`封包失败：${getPortableErrorMessage(error, '未知错误，请查看 logs/portable-data.log')}`);
  } finally {
    portablePacking.value = false;
  }
}

async function unpackPortableData() {
  const unpacker = window?.go?.main?.App?.UnpackPortableData;
  if (typeof unpacker !== 'function') {
    message.error('当前环境不支持本地绿色化解包');
    return;
  }
  portableUnpacking.value = true;
  try {
    const result = await unpacker();
    await applyPortableLocalStorageSnapshot(JSON.parse(String(result?.localStorageJson || '{}')));
    portableSettingsMeta.value = `解包完成：${result?.backupDir || 'backup'}，已恢复 ${Number(result?.localStorageKeyCount || 0)} 项本地数据`;
    message.success('已从 backup 解包恢复本程序数据，页面即将刷新');
    setTimeout(() => window.location.reload(), 600);
  } catch (error) {
    message.error(`解包失败：${getPortableErrorMessage(error, '未知错误，请查看 logs/portable-data.log')}`);
  } finally {
    portableUnpacking.value = false;
  }
}

function handleDesktopTokenSourceModeChange(event) {
  const nextValue = saveDesktopTokenSourceMode(normalizeDesktopTokenSourceMode(event?.target?.value));
  emit('update:desktopTokenSourceMode', nextValue);
}

function handleTreeExpandedChange(checked) {
  const nextValue = saveTreeExpandedSetting(Boolean(checked));
  emit('update:treeExpanded', nextValue);
}

function handleThemeModeSelection(nextMode) {
  themeMode.value = applyThemeMode(nextMode);
  message.success('界面主题已切换');
}

function handleLanguageSelection(nextLanguage) {
  languageMode.value = applyLanguage(nextLanguage, { translateDom: false });
  message.success(tr('界面语言已切换，正在刷新界面'));
  window.setTimeout(() => {
    try {
      const currentPath = `${window.location.pathname || '/'}${window.location.search || ''}${window.location.hash || ''}`;
      if (currentPath && currentPath !== '/') {
        sessionStorage.setItem('allapideck_pending_route_after_reload_v1', currentPath);
      }
    } catch {}
    const origin = String(window.location.origin || '').trim();
    window.location.assign(origin && origin !== 'null' ? `${origin}/` : '/');
  }, 350);
}

async function saveUserAgentMappingsDraft() {
  userAgentMappings.value = saveUserAgentMappings(userAgentMappings.value);
  try {
    const config = await getAdvancedProxyConfig();
    await setAdvancedProxyConfig(config);
  } catch {}
}

async function saveContextAutoCompressionDraft() {
  const normalized = saveContextAutoCompressionConfig(contextAutoCompressionDraft);
  Object.assign(contextAutoCompressionDraft, normalized);
  try {
    const config = await getAdvancedProxyConfig();
    await setAdvancedProxyConfig({
      ...config,
      contextAutoCompression: normalized,
    });
  } catch {}
}

function handleContextAutoCompressionEnabledChange(checked) {
  contextAutoCompressionDraft.enabled = Boolean(checked);
  if (!Number(contextAutoCompressionDraft.thresholdK)) {
    Object.assign(contextAutoCompressionDraft, normalizeContextAutoCompressionConfig(contextAutoCompressionDraft));
  }
  void saveContextAutoCompressionDraft();
}

function handleContextAutoCompressionThresholdChange(value) {
  contextAutoCompressionDraft.thresholdK = value;
  void saveContextAutoCompressionDraft();
}

function addUserAgentMappingRow() {
  userAgentMappings.value = [...userAgentMappings.value, { modelContains: '', targetUA: '' }];
  void saveUserAgentMappingsDraft();
}

function removeUserAgentMappingRow(index) {
  userAgentMappings.value = userAgentMappings.value.filter((_, itemIndex) => itemIndex !== index);
  void saveUserAgentMappingsDraft();
}

async function loadProxyDraft() {
  try {
    Object.assign(proxyDraft, await getOutboundProxyConfig());
  } catch (error) {
    Object.assign(proxyDraft, normalizeOutboundProxyConfig({}));
    message.error(error?.message || '加载代理设置失败');
  }
}

async function applyProxySettings() {
  proxySaving.value = true;
  try {
    const saved = await setOutboundProxyConfig(proxyDraft);
    Object.assign(proxyDraft, saved);
    message.success('代理设置已更新');
  } catch (error) {
    message.error(error?.message || '保存代理设置失败');
  } finally {
    proxySaving.value = false;
  }
}

function handleProxyModeChange(event) {
  proxyDraft.mode = normalizeOutboundProxyConfig({ mode: event?.target?.value, customUrl: proxyDraft.customUrl }).mode;
  if (proxyDraft.mode !== 'custom') {
    void applyProxySettings();
  }
}

function formatLogTimestamp(ts) {
  const num = Number(ts || 0);
  if (!num) return '-';
  const date = new Date(num);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString();
}

function formatLogSize(size) {
  const value = Number(size || 0);
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(2)} MB`;
  if (value >= 1024) return `${(value / 1024).toFixed(2)} KB`;
  return `${value} B`;
}

function buildDesktopLogFingerprint(files) {
  return (Array.isArray(files) ? files : [])
    .map(file => `${String(file?.path || '').trim()}|${Number(file?.updatedAt || 0)}|${Number(file?.size || 0)}`)
    .join('||');
}

function isDesktopLogSummaryNoiseLine(line) {
  const text = String(line || '').trim();
  if (!text) return true;
  const noisePatterns = [
    /\[(?:[^\]]*TRY|[^\]]*OK|sidebar\.routing)\]/i,
    /"snapshotApps"\s*:/i,
    /"visibleRecords"\s*:/i,
    /\btimeout=\d+s\b/i,
  ];
  return noisePatterns.some(pattern => pattern.test(text));
}

function isDesktopLogErrorSummaryLine(line) {
  const text = String(line || '').trim();
  if (!text || isDesktopLogSummaryNoiseLine(text)) return false;

  const taggedSeverityPattern = /\[(?:[A-Z0-9_.-]*?(?:ERROR|FAIL|FATAL|PANIC|EXCEPTION|MISS))\]/i;
  if (taggedSeverityPattern.test(text)) {
    return true;
  }

  const genericSeverityPattern = /\b(error|fatal|panic|exception|traceback|failed|failure|timed out|denied|refused|invalid|abort)\b|错误|异常|失败|拒绝|崩溃|中断|致命/i;
  return genericSeverityPattern.test(text);
}

function extractErrorSummaryLines(content) {
  const source = String(content || '');
  if (!source) return [];
  const lines = source.split(/\r?\n/);
  const result = [];
  lines.forEach((line, index) => {
    if (!isDesktopLogErrorSummaryLine(line)) return;
    result.push(`[L${index + 1}] ${line}`);
  });
  return result;
}

async function buildDesktopLogErrorSummaryContent() {
  const files = (Array.isArray(desktopLogFiles.value) ? desktopLogFiles.value : [])
    .filter(file => String(file?.path || '').trim());
  if (!files.length) {
    return '暂无日志文件可汇总。';
  }
  const fingerprint = buildDesktopLogFingerprint(files);
  if (desktopLogErrorSummaryCache.value.fingerprint === fingerprint) {
    return desktopLogErrorSummaryCache.value.content;
  }

  const chunks = [];
  for (const file of files) {
    const path = String(file?.path || '').trim();
    if (!path) continue;
    try {
      const result = await readDesktopLogFile(path);
      const errors = extractErrorSummaryLines(result?.content || '');
      if (!errors.length) continue;
      const title = `${String(file?.name || '未知文件')} · ${String(file?.sourceLabel || '未知来源')}`;
      chunks.push(`===== ${title} =====`);
      chunks.push(...errors.slice(0, 500));
      chunks.push('');
    } catch {}
  }

  const content = chunks.length
    ? chunks.join('\n').trim()
    : '未检测到错误级日志（error/fatal/failed/timeout/错误/异常 等）。';
  desktopLogErrorSummaryCache.value = { fingerprint, content };
  return content;
}

async function loadDesktopLogContent(path) {
  const targetPath = String(path || '').trim();
  if (!targetPath || !isDesktopLogAvailable.value) {
    selectedDesktopLogContent.value = '';
    return;
  }
  desktopLogContentLoading.value = true;
  try {
    selectedDesktopLogPath.value = targetPath;
    if (targetPath === ERROR_SUMMARY_FILE_PATH) {
      selectedDesktopLogContent.value = await buildDesktopLogErrorSummaryContent();
      return;
    }
    const result = await readDesktopLogFile(targetPath);
    selectedDesktopLogContent.value = String(result?.content || '');
  } catch (error) {
    selectedDesktopLogContent.value = '';
    message.error(error?.message || '读取日志失败');
  } finally {
    desktopLogContentLoading.value = false;
  }
}

async function loadDesktopLogs() {
  if (!isDesktopLogAvailable.value) {
    desktopLogFiles.value = [];
    selectedDesktopLogGroup.value = '';
    selectedDesktopLogPath.value = '';
    selectedDesktopLogContent.value = '';
    return;
  }
  desktopLogsLoading.value = true;
  try {
    const snapshot = await listDesktopLogFiles();
    desktopLogFiles.value = Array.isArray(snapshot?.files) ? snapshot.files : [];
    desktopLogErrorSummaryCache.value = { fingerprint: '', content: '' };
    selectedDesktopLogGroup.value = desktopLogGroups.value.find(group => group.key === selectedDesktopLogGroup.value)?.key
      || desktopLogGroups.value[0]?.key
      || '';
    const nextPath = currentDesktopLogGroupFiles.value.find(file => String(file?.path || '') === selectedDesktopLogPath.value)?.path
      || currentDesktopLogGroupFiles.value[0]?.path
      || '';
    selectedDesktopLogPath.value = nextPath;
    if (nextPath) {
      await loadDesktopLogContent(nextPath);
    } else {
      selectedDesktopLogContent.value = '';
    }
  } catch (error) {
    desktopLogFiles.value = [];
    desktopLogErrorSummaryCache.value = { fingerprint: '', content: '' };
    selectedDesktopLogGroup.value = '';
    selectedDesktopLogPath.value = '';
    selectedDesktopLogContent.value = '';
    message.error(error?.message || '加载日志列表失败');
  } finally {
    desktopLogsLoading.value = false;
  }
}
</script>

<style scoped>
.settings-tab-content {
  padding: 10px;
}

.settings-muted-text,
.settings-file-meta {
  color: #8c8c8c;
  font-size: 12px;
  line-height: 1.7;
}

.settings-field-caption {
  font-size: 12px;
  color: #8c8c8c;
  margin-bottom: 6px;
}

.settings-section-title {
  margin: 0 0 12px;
}

.settings-section-title-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 12px;
}

.settings-section-title-spaced {
  margin-top: 24px;
}

.settings-info-icon {
  appearance: none;
  width: 17px;
  height: 17px;
  padding: 0;
  border: 1px solid rgba(54, 126, 224, 0.32);
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(54, 126, 224, 0.1);
  color: #2f72d9;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  cursor: help;
}

.settings-info-icon:hover {
  border-color: rgba(54, 126, 224, 0.52);
  background: rgba(54, 126, 224, 0.16);
  color: #1f5fc1;
}

:global(.settings-info-tooltip-copy) {
  display: grid;
  gap: 6px;
  max-width: 420px;
  line-height: 1.55;
}

.settings-switch-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.context-compression-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  width: 100%;
}

.context-compression-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.context-compression-copy span {
  font-weight: 600;
  color: #243229;
}

.context-compression-copy small {
  color: #8c8c8c;
  font-size: 12px;
  line-height: 1.6;
}

.settings-log-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.settings-log-title {
  font-weight: 600;
}

.settings-tab-content :deep(.ant-radio-button-wrapper.settings-log-group-error) {
  color: rgba(196, 40, 40, 0.95);
  border-color: rgba(196, 40, 40, 0.4);
}

.settings-tab-content :deep(.ant-radio-button-wrapper-checked.settings-log-group-error) {
  color: #fff;
  background: rgba(196, 40, 40, 0.92);
  border-color: rgba(196, 40, 40, 0.92);
  box-shadow: -1px 0 0 0 rgba(196, 40, 40, 0.92);
}

.settings-version-text {
  text-align: center;
  color: #999;
}

.proxy-custom-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  width: 100%;
}

.theme-mode-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.settings-language-select {
  width: 100%;
  margin-bottom: 16px;
}

.theme-mode-card {
  border: 1px solid rgba(88, 112, 84, 0.12);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(244, 248, 240, 0.92));
  border-radius: 16px;
  padding: 12px;
  display: grid;
  gap: 10px;
  text-align: left;
  cursor: pointer;
  transition:
    transform 0.2s ease,
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    background-color 0.2s ease;
}

.theme-mode-card:hover {
  transform: translateY(-1px);
  border-color: rgba(88, 112, 84, 0.2);
  box-shadow: 0 10px 20px rgba(38, 51, 35, 0.08);
}

.theme-mode-card.is-active {
  border-color: rgba(86, 122, 104, 0.45);
  box-shadow:
    0 12px 24px rgba(33, 48, 40, 0.1),
    inset 0 0 0 1px rgba(126, 171, 148, 0.18);
}

.theme-mode-swatches {
  display: flex;
  gap: 6px;
}

.theme-mode-swatch {
  flex: 1 1 0;
  height: 22px;
  border-radius: 999px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.5);
}

.theme-mode-card-light .theme-mode-swatch-a { background: #f7ead1; }
.theme-mode-card-light .theme-mode-swatch-b { background: #dce8c4; }
.theme-mode-card-light .theme-mode-swatch-c { background: #ffffff; }
.theme-mode-card-gaia-dark .theme-mode-swatch-a { background: #0c1419; }
.theme-mode-card-gaia-dark .theme-mode-swatch-b { background: #39525e; }
.theme-mode-card-gaia-dark .theme-mode-swatch-c { background: #7b614b; }

.theme-mode-copy {
  display: grid;
  gap: 4px;
}

.theme-mode-copy strong {
  font-size: 13px;
  color: #243229;
}

.theme-mode-copy small {
  font-size: 12px;
  line-height: 1.6;
  color: #627064;
}

.ua-mapping-card {
  display: grid;
  gap: 10px;
  margin-top: 10px;
}

.ua-mapping-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.ua-mapping-caption {
  font-size: 12px;
  line-height: 1.6;
  color: #627064;
}

.ua-mapping-grid {
  display: grid;
  grid-template-columns: minmax(140px, 0.7fr) minmax(0, 1.3fr) auto;
  gap: 10px;
  align-items: start;
}

.ua-mapping-grid-head {
  font-size: 12px;
  color: #8c8c8c;
}

.portable-settings-card {
  display: grid;
  gap: 18px;
  padding: 18px;
  border-radius: 18px;
  border: 1px solid rgba(116, 144, 104, 0.16);
  background: rgba(248, 251, 246, 0.96);
}

.portable-settings-copy {
  display: grid;
  gap: 8px;
}

.portable-settings-title {
  font-size: 18px;
  font-weight: 700;
  color: #20301b;
}

.portable-settings-desc,
.portable-settings-hint,
.portable-settings-meta,
.portable-settings-warning {
  line-height: 1.7;
  color: #5f6f59;
}

.portable-settings-warning {
  color: #b25f00;
}

.portable-settings-actions {
  display: flex;
  gap: 12px;
}

:deep(body.dark-mode) .portable-settings-card {
  border-color: rgba(154, 191, 142, 0.18);
  background: rgba(24, 32, 25, 0.92);
}

:deep(body.dark-mode) .portable-settings-title {
  color: #ecf8e7;
}

:deep(body.dark-mode) .portable-settings-desc,
:deep(body.dark-mode) .portable-settings-hint,
:deep(body.dark-mode) .portable-settings-meta {
  color: #b8cbb1;
}

:deep(body.dark-mode) .portable-settings-warning {
  color: #ffcb8a;
}

:deep(body.dark-mode) .theme-mode-card {
  border-color: rgba(154, 191, 142, 0.14);
  background: linear-gradient(180deg, rgba(30, 39, 31, 0.96), rgba(20, 27, 22, 0.92));
}

:deep(body.dark-mode) .theme-mode-card:hover {
  border-color: rgba(154, 191, 142, 0.24);
  box-shadow: 0 12px 24px rgba(0, 0, 0, 0.2);
}

:deep(body.dark-mode) .theme-mode-card.is-active {
  border-color: rgba(160, 198, 149, 0.34);
  box-shadow:
    0 14px 28px rgba(0, 0, 0, 0.24),
    inset 0 0 0 1px rgba(174, 212, 163, 0.12);
}

:deep(body.dark-mode) .theme-mode-copy strong {
  color: #ebf5e5;
}

:deep(body.dark-mode) .theme-mode-copy small {
  color: #b7c7b1;
}

:deep(body.dark-mode) .settings-info-icon {
  border-color: rgba(96, 165, 250, 0.34);
  background: rgba(96, 165, 250, 0.14);
  color: #93c5fd;
}

:deep(body.dark-mode) .ua-mapping-caption,
:deep(body.dark-mode) .ua-mapping-grid-head {
  color: #b7c7b1;
}

:deep(body.dark-mode) .context-compression-copy span {
  color: #ebf5e5;
}

:deep(body.dark-mode) .context-compression-copy small {
  color: #b7c7b1;
}

:deep(body.gaia-dark) .theme-mode-card {
  border-color: rgba(101, 129, 138, 0.18);
  background:
    radial-gradient(circle at 88% 12%, rgba(138, 108, 76, 0.14), transparent 22%),
    linear-gradient(180deg, rgba(12, 21, 26, 0.98), rgba(8, 14, 18, 0.94));
}

:deep(body.gaia-dark) .theme-mode-card:hover {
  border-color: rgba(118, 151, 162, 0.28);
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.28);
}

:deep(body.gaia-dark) .theme-mode-card.is-active {
  border-color: rgba(118, 151, 162, 0.42);
  box-shadow:
    0 14px 28px rgba(0, 0, 0, 0.3),
    inset 0 0 0 1px rgba(126, 164, 176, 0.12);
}

:deep(body.gaia-dark) .theme-mode-copy strong {
  color: #e6f1ef;
}

:deep(body.gaia-dark) .theme-mode-copy small {
  color: #9eb2b3;
}

:deep(body.gaia-dark) .settings-info-icon {
  border-color: rgba(109, 172, 190, 0.34);
  background: rgba(109, 172, 190, 0.14);
  color: #8bc6d4;
}

:deep(body.gaia-dark) .portable-settings-card {
  border-color: rgba(101, 129, 138, 0.18);
  background:
    radial-gradient(circle at 84% 14%, rgba(133, 103, 73, 0.12), transparent 24%),
    rgba(10, 18, 22, 0.92);
}

:deep(body.gaia-dark) .portable-settings-title {
  color: #e6f1ef;
}

:deep(body.gaia-dark) .portable-settings-desc,
:deep(body.gaia-dark) .portable-settings-hint,
:deep(body.gaia-dark) .portable-settings-meta {
  color: #a7bbbc;
}

:deep(body.gaia-dark) .portable-settings-warning {
  color: #d7b088;
}

:deep(body.gaia-dark) .ua-mapping-caption,
:deep(body.gaia-dark) .ua-mapping-grid-head {
  color: #9eb2b3;
}

:deep(body.gaia-dark) .context-compression-copy span {
  color: #e6f1ef;
}

:deep(body.gaia-dark) .context-compression-copy small {
  color: #9eb2b3;
}

@media (max-width: 720px) {
  .proxy-custom-row {
    grid-template-columns: 1fr;
  }

  .theme-mode-grid {
    grid-template-columns: 1fr;
  }

  .ua-mapping-grid {
    grid-template-columns: 1fr;
  }

  .context-compression-row {
    align-items: center;
  }

  .portable-settings-actions {
    flex-wrap: wrap;
  }
}
</style>
