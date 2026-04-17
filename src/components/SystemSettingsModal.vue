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
      <a-tab-pane key="1" tab="本地绿色化">
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
      <a-tab-pane key="2" tab="常规设置">
        <div class="settings-tab-content">
          <p><b>桌面端提取方式</b></p>
          <a-space direction="vertical" style="width: 100%; margin-bottom: 16px;">
            <a-radio-group :value="desktopTokenSourceMode" :disabled="!isWailsRuntime" @change="handleDesktopTokenSourceModeChange">
              <a-radio value="profile_file">Profile 文件</a-radio>
              <a-radio value="cdp_restart">CDP 重开模式</a-radio>
            </a-radio-group>
            <div class="settings-muted-text">
              <div>Profile 文件模式：从本机 Chrome 默认 Profile 的本地存储文件读取登录态，例如 auth_token、auth_user、refresh_token，再直接请求站点 Token 列表。不主动拉起受控浏览器。</div>
              <div>CDP 重开模式：检测失败站点后，打开或重启 Chrome/Edge 受控浏览器，附着到 CDP 会话，在真实浏览器上下文里读取登录态并轮询抓取 Token。会使用 shadow / remote debugging 这套流程。</div>
              <div>桌面端会严格按所选模式执行，不自动切换到另一种模式。</div>
              <div v-if="!isWailsRuntime">该设置仅在桌面端 EXE 生效，浏览器模式仍走前端直连。</div>
              <div v-else-if="desktopTokenSourceMode === 'profile_file' && !effectiveChromeProfileAuthAvailable">当前桌面端尚未暴露 Profile 文件提取接口，无法使用该模式。</div>
            </div>
          </a-space>

          <p><b>代理模式</b></p>
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
            <div class="settings-muted-text">
              <div>默认使用系统代理，显式集成到桌面端 Go 后端请求链路。</div>
              <div>自定义格式兼容：`socks5://`、`socks5h://`、`http://`、`https://`，也支持 `user:pass@host:port`。</div>
              <div>浏览器模式下仅保存配置，不会接管浏览器自身网络栈；桌面端 EXE / Wails 才会真正作用于后端请求。</div>
            </div>
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
      <a-tab-pane key="3" tab="日志">
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
                  <a-radio-button v-for="group in desktopLogGroups" :key="group.key" :value="group.key">
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
import { computed, reactive, ref, watch } from 'vue';
import { message } from 'ant-design-vue';
import { isProbablyWailsRuntime } from '../utils/runtimeApi.js';
import { isDesktopLogBridgeAvailable, listDesktopLogFiles, readDesktopLogFile } from '../utils/desktopLogBridge.js';
import { isChromeProfileAuthBridgeAvailable } from '../utils/profileAuthBridge.js';
import {
  getOutboundProxyConfig,
  normalizeDesktopTokenSourceMode,
  normalizeOutboundProxyConfig,
  saveDesktopTokenSourceMode,
  saveTreeExpandedSetting,
  setOutboundProxyConfig,
} from '../utils/systemSettings.js';

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

const isDesktopLogAvailable = computed(() => isDesktopLogBridgeAvailable());

const desktopLogGroups = computed(() => {
  const groupMap = new Map();
  (Array.isArray(desktopLogFiles.value) ? desktopLogFiles.value : []).forEach(file => {
    const key = String(file?.groupKey || 'other').trim() || 'other';
    const label = String(file?.groupLabel || '其他日志').trim() || '其他日志';
    if (!groupMap.has(key)) {
      groupMap.set(key, { key, label, files: [] });
    }
    groupMap.get(key).files.push(file);
  });
  return Array.from(groupMap.values());
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
  void loadProxyDraft();
  if (isWailsRuntime) {
    void loadDesktopLogs();
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

function snapshotPortableLocalStorage() {
  const snapshot = {};
  try {
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index);
      if (!key) continue;
      snapshot[key] = localStorage.getItem(key);
    }
  } catch {}
  return snapshot;
}

function applyPortableLocalStorageSnapshot(snapshot) {
  try {
    localStorage.clear();
    Object.entries(snapshot || {}).forEach(([key, value]) => {
      localStorage.setItem(key, value == null ? '' : String(value));
    });
  } catch {}
}

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
    const result = await packer(JSON.stringify(snapshotPortableLocalStorage()));
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
    applyPortableLocalStorageSnapshot(JSON.parse(String(result?.localStorageJson || '{}')));
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

async function loadDesktopLogContent(path) {
  const targetPath = String(path || '').trim();
  if (!targetPath || !isDesktopLogAvailable.value) {
    selectedDesktopLogContent.value = '';
    return;
  }
  desktopLogContentLoading.value = true;
  try {
    const result = await readDesktopLogFile(targetPath);
    selectedDesktopLogPath.value = targetPath;
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

.settings-switch-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
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

@media (max-width: 720px) {
  .proxy-custom-row {
    grid-template-columns: 1fr;
  }

  .portable-settings-actions {
    flex-wrap: wrap;
  }
}
</style>
