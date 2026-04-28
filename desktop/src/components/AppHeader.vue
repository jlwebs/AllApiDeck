<template>
  <header class="spring-header" :class="{ 'spring-header-gaia': isDarkMode }">
    <button type="button" class="spring-brand" @click="navigate('/')">
      <span class="spring-brand-mark">
        <img :src="appLogo" alt="" class="spring-brand-icon" />
      </span>
      <span class="spring-brand-title">All API Deck</span>
    </button>

    <nav class="spring-toolbar">
      <button
        type="button"
        class="spring-pill"
        :class="{ 'spring-pill-active': currentPage === 'batch' }"
        @click="navigate('/')"
      >
        <span class="spring-pill-icon-svg spring-pill-icon-chrome" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle cx="12" cy="12" r="3.5" />
            <path d="M12 2.75a9.25 9.25 0 0 1 8.01 4.63H11.2" />
            <path d="M4.18 7.38A9.25 9.25 0 0 0 12 21.25l4.41-7.64" />
            <path d="M20.01 6.95A9.25 9.25 0 0 1 12 21.25L7.59 13.6" />
          </svg>
        </span>
        <span>批量检测</span>
      </button>

      <span class="spring-flow-arrow" aria-hidden="true">
        <svg viewBox="0 0 34 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M3 10H24M24 10L17 3M24 10L17 17" />
        </svg>
      </span>

      <button
        type="button"
        class="spring-pill"
        :class="{ 'spring-pill-active': currentPage === 'sites' }"
        @click="navigate('/sites')"
      >
        <DatabaseOutlined />
        <span>站点管理</span>
      </button>

      <span class="spring-flow-arrow" aria-hidden="true">
        <svg viewBox="0 0 34 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M3 10H24M24 10L17 3M24 10L17 17" />
        </svg>
      </span>

      <button
        type="button"
        class="spring-pill"
        :class="{ 'spring-pill-active': currentPage === 'keys' }"
        @click="navigate('/keys')"
      >
        <KeyOutlined />
        <span>密钥管理</span>
      </button>

      <button
        v-if="showSettings"
        type="button"
        class="spring-pill spring-pill-icon-only"
        title="设置"
        aria-label="设置"
        @click="$emit('settings')"
      >
        <SettingOutlined />
      </button>

      <a-tooltip v-if="showExperimental" :title="advancedProxyTooltip">
        <button
          type="button"
          class="spring-pill spring-pill-icon-only"
          :aria-label="advancedProxyLabel"
          @click="$emit('experimental')"
        >
          <ApiOutlined />
        </button>
      </a-tooltip>

      <button type="button" class="spring-pill spring-pill-ghost spring-pill-github" @click="openUpdateModal">
        <span v-if="hasAppUpdate" class="spring-pill-update-dot" aria-hidden="true"></span>
        <GithubOutlined />
        <span>{{ versionButtonLabel }}</span>
      </button>
    </nav>
  </header>

  <a-modal
    :open="updateModalOpen"
    title="版本更新"
    :footer="null"
    :width="620"
    centered
    wrap-class-name="spring-update-modal-wrap"
    @cancel="closeUpdateModal"
  >
    <div class="spring-update-modal">
      <a-spin :spinning="loadingUpdateInfo">
        <div class="spring-update-hero">
          <div class="spring-update-hero-top">
            <div class="spring-update-hero-copy">
              <span class="spring-update-kicker">当前版本</span>
              <strong>{{ currentTagLabel }}</strong>
              <small v-if="updateInfo?.latestTag">最新版本 {{ updateInfo.latestTag }}</small>
              <small v-else-if="loadingUpdateInfo">正在获取最新版本信息…</small>
              <small v-else>暂未获取到最新版本信息</small>
            </div>

            <div class="spring-update-hero-side">
              <a-tag :color="hasUpdate ? 'red' : 'green'">
                {{ hasUpdate ? '发现新版本' : '已是最新版本' }}
              </a-tag>
              <a-button type="link" size="small" @click="openReleasePage">查看 GitHub Release</a-button>
            </div>
          </div>

          <div class="spring-update-summary">
            <div class="spring-update-summary-head">更新说明</div>
            <pre class="spring-update-summary-body">{{ releaseNotesText }}</pre>
          </div>
        </div>

        <div v-if="updateInfoError" class="spring-update-error">
          {{ updateInfoError }}
        </div>

        <div class="spring-update-progress-shell" :class="{ 'is-visible': showDownloadProgress }">
          <div class="spring-update-progress-head">
            <span>{{ downloadSnapshot.message || (hasUpdate ? '可更新到最新版本' : '可重新下载安装包') }}</span>
            <span v-if="showDownloadProgress">{{ downloadPercentText }}</span>
          </div>
          <a-progress
            :percent="downloadPercent"
            :status="downloadProgressStatus"
            size="small"
            :show-info="false"
          />
          <div class="spring-update-progress-meta">
            <span>{{ downloadBytesText }}</span>
            <span v-if="downloadSnapshot.savedPath">{{ downloadSnapshot.savedPath }}</span>
          </div>
          <div v-if="downloadSnapshot.error" class="spring-update-error spring-update-error-inline">
            {{ downloadSnapshot.error }}
          </div>
        </div>

        <div class="spring-update-actions">
          <a-button @click="closeUpdateModal">稍后再说</a-button>
          <a-button
            v-if="showOpenDownloadedButton"
            type="primary"
            @click="openDownloadedUpdatePackage"
          >
            打开安装包
          </a-button>
          <a-button
            v-else
            type="primary"
            :loading="updateActionLoading"
            :disabled="!hasCompatibleAsset || isDownloading"
            @click="startUpdateDownload"
          >
            {{ hasUpdate ? '更新到最新版本' : '下载当前安装包' }}
          </a-button>
        </div>
      </a-spin>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { message } from 'ant-design-vue';
import appLogo from '../assets/logo.png';
import {
  ApiOutlined,
  DatabaseOutlined,
  GithubOutlined,
  KeyOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue';
import * as WailsApp from '../../wailsjs/go/main/App.js';
import { EventsOff, EventsOn } from '../../wailsjs/runtime/runtime.js';
import {
  ensureStartupUpdateStatus,
  getAppGithubUrl,
  getCurrentAppTag,
  getCurrentAppVersion,
  getStartupLatestReleasePayload,
  getStartupUpdateStatus,
} from '../utils/appUpdateState.js';
import { openUrlInSystemBrowser } from '../utils/runtimeApi.js';

defineEmits(['experimental', 'settings']);

defineProps({
  currentPage: {
    type: String,
    default: '',
  },
  isDarkMode: {
    type: Boolean,
    default: false,
  },
  showExperimental: {
    type: Boolean,
    default: true,
  },
  showSettings: {
    type: Boolean,
    default: true,
  },
});

const appUpdateDownloadEventName = 'app:update-download-progress';
const router = useRouter();
const hasAppUpdate = ref(false);
const updateModalOpen = ref(false);
const loadingUpdateInfo = ref(false);
const updateActionLoading = ref(false);
const updateInfo = ref(null);
const updateInfoError = ref('');
const downloadSnapshot = ref(buildEmptyDownloadSnapshot());
const advancedProxyLabel = '高级代理';
const advancedProxyTooltip = '开启兼容 OpenAI vendor、Claude、故障转移、错误修正的高级代理功能';

function buildReleaseInfoFallback(status) {
  if (!status || typeof status !== 'object') {
    return null;
  }
  const latestTag = String(status.latestTag || '').trim();
  const latestVersion = normalizeVersion(status.latestVersion || latestTag);
  const htmlUrl = String(status.htmlUrl || getAppGithubUrl()).trim();
  if (!latestTag && !latestVersion && !htmlUrl) {
    return null;
  }
  return {
    latestTag,
    latestVersion,
    htmlUrl,
    body: '',
    targetOs: '',
    targetArch: '',
    asset: null,
  };
}

function buildReleaseInfoFromStartup(status, payload) {
  const latestTag = String(payload?.tag_name || status?.latestTag || '').trim();
  const latestVersion = normalizeVersion(payload?.tag_name || status?.latestVersion || latestTag);
  const htmlUrl = String(payload?.html_url || status?.htmlUrl || getAppGithubUrl()).trim();
  const body = String(payload?.body || '').trim();
  if (!latestTag && !latestVersion && !htmlUrl && !body) {
    return null;
  }
  return {
    latestTag,
    latestVersion,
    htmlUrl,
    body,
    targetOs: '',
    targetArch: '',
    asset: null,
  };
}

function normalizeVersion(value) {
  return String(value || '')
    .trim()
    .replace(/^v/i, '')
    .replace(/[^0-9A-Za-z.+-].*$/, '');
}

function parseVersionParts(version) {
  return normalizeVersion(version)
    .split('.')
    .map(part => Number.parseInt(part, 10))
    .filter(part => Number.isFinite(part) && part >= 0);
}

function isNewerVersion(latest, current) {
  const latestParts = parseVersionParts(latest);
  const currentParts = parseVersionParts(current);
  if (!latestParts.length || !currentParts.length) {
    return false;
  }
  const maxLength = Math.max(latestParts.length, currentParts.length);
  for (let index = 0; index < maxLength; index += 1) {
    const latestValue = latestParts[index] || 0;
    const currentValue = currentParts[index] || 0;
    if (latestValue > currentValue) return true;
    if (latestValue < currentValue) return false;
  }
  return false;
}

function buildEmptyDownloadSnapshot() {
  return {
    active: false,
    stage: 'idle',
    latestTag: '',
    fileName: '',
    downloadUrl: '',
    savedPath: '',
    totalBytes: 0,
    receivedBytes: 0,
    percent: 0,
    message: '',
    error: '',
    startedAt: 0,
    updatedAt: 0,
  };
}

function normalizeDownloadSnapshot(payload) {
  const source = payload && typeof payload === 'object' && payload.data && typeof payload.data === 'object'
    ? payload.data
    : payload;
  return {
    ...buildEmptyDownloadSnapshot(),
    ...(source || {}),
  };
}

function formatBytes(bytes) {
  const value = Number(bytes || 0);
  if (!Number.isFinite(value) || value <= 0) {
    return '0 B';
  }
  const units = ['B', 'KB', 'MB', 'GB'];
  let unitIndex = 0;
  let currentValue = value;
  while (currentValue >= 1024 && unitIndex < units.length - 1) {
    currentValue /= 1024;
    unitIndex += 1;
  }
  const digits = currentValue >= 100 || unitIndex === 0 ? 0 : currentValue >= 10 ? 1 : 2;
  return `${currentValue.toFixed(digits)} ${units[unitIndex]}`;
}

const currentTagLabel = computed(() => {
  const currentTag = String(getCurrentAppTag() || '').trim();
  if (currentTag) return currentTag;
  const currentVersion = String(getCurrentAppVersion() || '').trim();
  return currentVersion ? `v${currentVersion}` : 'dev';
});

const versionButtonLabel = computed(() => currentTagLabel.value);

const hasCompatibleAsset = computed(() => Boolean(updateInfo.value?.latestTag || updateInfo.value?.latestVersion));

const hasUpdate = computed(() => {
  const startupStatus = getStartupUpdateStatus();
  if (startupStatus?.checked) {
    return Boolean(startupStatus.hasUpdate);
  }
  const latestVersion = normalizeVersion(updateInfo.value?.latestVersion || updateInfo.value?.latestTag || '');
  const currentVersion = normalizeVersion(getCurrentAppVersion());
  return Boolean(latestVersion && currentVersion && isNewerVersion(latestVersion, currentVersion));
});

const isDownloading = computed(() => (
  downloadSnapshot.value.active &&
  (downloadSnapshot.value.stage === 'preparing' || downloadSnapshot.value.stage === 'downloading')
));

const showDownloadProgress = computed(() => {
  const stage = String(downloadSnapshot.value.stage || '');
  return stage === 'preparing' || stage === 'downloading' || stage === 'completed' || stage === 'error';
});

const showOpenDownloadedButton = computed(() => (
  downloadSnapshot.value.stage === 'completed' &&
  Boolean(downloadSnapshot.value.savedPath)
));

const releasePageUrl = computed(() => String(updateInfo.value?.htmlUrl || getAppGithubUrl()).trim());

const releaseNotesText = computed(() => {
  const body = String(updateInfo.value?.body || '').trim();
  if (body) return body;
  if (loadingUpdateInfo.value) return '正在加载更新说明…';
  if (updateInfoError.value) return '最新版本信息获取失败，可以直接打开 GitHub Release 页面查看。';
  return '当前版本暂无额外更新说明。';
});

const downloadPercent = computed(() => {
  const percent = Number(downloadSnapshot.value.percent || 0);
  if (Number.isFinite(percent) && percent > 0) {
    return Math.max(0, Math.min(100, Number(percent.toFixed(1))));
  }
  const totalBytes = Number(downloadSnapshot.value.totalBytes || 0);
  const receivedBytes = Number(downloadSnapshot.value.receivedBytes || 0);
  if (totalBytes > 0 && receivedBytes > 0) {
    return Math.max(0, Math.min(100, Number(((receivedBytes / totalBytes) * 100).toFixed(1))));
  }
  return downloadSnapshot.value.stage === 'completed' ? 100 : 0;
});

const downloadProgressStatus = computed(() => {
  if (downloadSnapshot.value.stage === 'error') return 'exception';
  if (downloadSnapshot.value.stage === 'completed') return 'success';
  return 'active';
});

const downloadPercentText = computed(() => `${downloadPercent.value.toFixed(1)}%`);

const downloadBytesText = computed(() => {
  const received = formatBytes(downloadSnapshot.value.receivedBytes);
  const total = Number(downloadSnapshot.value.totalBytes || 0);
  return total > 0 ? `${received} / ${formatBytes(total)}` : received;
});

const navigate = path => {
  if (router.currentRoute.value.path !== path) {
    router.push(path);
  }
};

const openReleasePage = () => {
  openUrlInSystemBrowser(releasePageUrl.value || getAppGithubUrl());
};

const closeUpdateModal = () => {
  updateModalOpen.value = false;
};

const handleUpdateDownloadEvent = payload => {
  downloadSnapshot.value = normalizeDownloadSnapshot(payload);
};

async function loadUpdateModalState() {
  loadingUpdateInfo.value = true;
  updateInfoError.value = '';
  try {
    const startupStatus = await ensureStartupUpdateStatus();
    const startupPayload = getStartupLatestReleasePayload();
    const startupInfo = buildReleaseInfoFromStartup(startupStatus, startupPayload)
      || buildReleaseInfoFallback(startupStatus);
    const snapshot = await (
      typeof WailsApp.GetAppUpdateDownloadSnapshot === 'function'
        ? WailsApp.GetAppUpdateDownloadSnapshot()
        : Promise.resolve(buildEmptyDownloadSnapshot())
    ).catch(() => buildEmptyDownloadSnapshot());
    updateInfo.value = startupInfo;
    downloadSnapshot.value = normalizeDownloadSnapshot(snapshot);
    hasAppUpdate.value = Boolean(startupStatus?.hasUpdate);
    updateInfoError.value = startupStatus?.error || (!startupInfo ? '获取最新版本信息失败' : '');
  } catch (error) {
    const startupStatus = getStartupUpdateStatus();
    const startupPayload = getStartupLatestReleasePayload();
    const startupInfo = buildReleaseInfoFromStartup(startupStatus, startupPayload)
      || buildReleaseInfoFallback(startupStatus);
    if (startupInfo) {
      updateInfo.value = startupInfo;
      hasAppUpdate.value = Boolean(startupStatus?.hasUpdate);
    }
    updateInfoError.value = startupStatus?.error || error?.message || '获取最新版本信息失败';
    try {
      if (typeof WailsApp.GetAppUpdateDownloadSnapshot === 'function') {
        const snapshot = await WailsApp.GetAppUpdateDownloadSnapshot();
        downloadSnapshot.value = normalizeDownloadSnapshot(snapshot);
      }
    } catch {}
  } finally {
    loadingUpdateInfo.value = false;
  }
}

async function openUpdateModal() {
  updateModalOpen.value = true;
  await loadUpdateModalState();
}

async function startUpdateDownload() {
  if (!hasCompatibleAsset.value) {
    message.warning('当前系统没有可用的版本信息');
    return;
  }
  if (typeof WailsApp.StartLatestAppReleaseDownload !== 'function') {
    message.error('当前运行中的桌面后端尚未加载更新下载接口，请重启后再试');
    return;
  }
  updateActionLoading.value = true;
  try {
    const snapshot = await WailsApp.StartLatestAppReleaseDownload();
    downloadSnapshot.value = normalizeDownloadSnapshot(snapshot);
  } catch (error) {
    try {
      if (typeof WailsApp.GetAppUpdateDownloadSnapshot === 'function') {
        const snapshot = await WailsApp.GetAppUpdateDownloadSnapshot();
        downloadSnapshot.value = normalizeDownloadSnapshot(snapshot);
      }
    } catch {}
    message.error(error?.message || '启动更新下载失败');
  } finally {
    updateActionLoading.value = false;
  }
}

async function openDownloadedUpdatePackage() {
  try {
    if (typeof WailsApp.OpenDownloadedAppUpdate !== 'function') {
      throw new Error('当前运行中的桌面后端尚未加载安装包打开接口，请重启后再试');
    }
    await WailsApp.OpenDownloadedAppUpdate();
  } catch (error) {
    message.error(error?.message || '打开安装包失败');
  }
}

onMounted(async () => {
  try {
    EventsOn(appUpdateDownloadEventName, handleUpdateDownloadEvent);
  } catch {}

  const status = await ensureStartupUpdateStatus();
  hasAppUpdate.value = Boolean(status?.hasUpdate);
});

onBeforeUnmount(() => {
  try {
    EventsOff(appUpdateDownloadEventName);
  } catch {}
});
</script>

<style scoped>
.spring-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  margin-bottom: 8px;
  padding: 8px 10px;
  border-radius: 18px;
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(77, 104, 73, 0.12);
  background:
    linear-gradient(135deg, rgba(255, 251, 242, 0.94), rgba(239, 246, 228, 0.84)),
    rgba(255, 255, 255, 0.76);
  box-shadow:
    0 10px 24px rgba(87, 107, 73, 0.07),
    inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.spring-brand {
  border: 0;
  background: transparent;
  padding: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  min-width: 0;
  text-align: left;
  flex: 0 0 auto;
}

.spring-brand-mark {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(160deg, #edf5d7, #bfd39a);
  overflow: hidden;
  box-shadow: 0 6px 14px rgba(86, 118, 76, 0.14);
}

.spring-brand-icon {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.spring-brand-title {
  color: #29412d;
  font: 700 14px/1.05 Georgia, 'Times New Roman', serif;
  white-space: nowrap;
}

.spring-toolbar {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}

.spring-pill {
  border: 1px solid rgba(77, 104, 73, 0.08);
  background: rgba(255, 255, 255, 0.62);
  color: #5e6f59;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border-radius: 999px;
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  white-space: nowrap;
  transition:
    transform 0.2s ease,
    background-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease;
}

.spring-flow-arrow {
  width: 24px;
  height: 32px;
  flex: 0 0 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgba(105, 123, 95, 0.82);
  margin: 0 -1px;
  pointer-events: none;
}

.spring-flow-arrow svg {
  width: 24px;
  height: 16px;
  overflow: visible;
  filter: drop-shadow(0 1px 0 rgba(255, 255, 255, 0.72));
}

.spring-flow-arrow path {
  stroke: currentColor;
  stroke-width: 3.2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.spring-pill-active {
  color: #28412c;
  background: linear-gradient(135deg, rgba(234, 243, 213, 0.98), rgba(214, 230, 188, 0.92));
  box-shadow: 0 8px 18px rgba(96, 122, 77, 0.12);
}

.spring-pill-ghost {
  background: rgba(255, 255, 255, 0.5);
}

.spring-pill-github {
  position: relative;
  min-width: 86px;
}

.spring-pill-update-dot {
  position: absolute;
  top: 4px;
  right: 7px;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #c9473f;
  box-shadow:
    0 0 0 2px rgba(255, 252, 247, 0.95),
    0 2px 6px rgba(156, 44, 36, 0.22);
}

.spring-pill-icon-only {
  width: 32px;
  min-width: 32px;
  padding: 0;
  gap: 0;
}

.spring-pill :deep(.anticon) {
  font-size: 14px;
}

.spring-pill-icon-svg {
  width: 14px;
  height: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
}

.spring-pill-icon-svg svg {
  width: 14px;
  height: 14px;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.spring-pill:hover {
  color: #28412c;
  background: rgba(239, 246, 226, 0.84);
  transform: translateY(-1px);
}

.spring-update-modal {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.spring-update-hero {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 16px;
  border: 1px solid rgba(121, 145, 102, 0.14);
  background:
    linear-gradient(135deg, rgba(251, 247, 236, 0.96), rgba(237, 245, 224, 0.94)),
    rgba(255, 255, 255, 0.88);
}

.spring-update-hero-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.spring-update-hero-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.spring-update-kicker {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #6e7f62;
}

.spring-update-hero-copy strong {
  color: #243c28;
  font-size: 22px;
  line-height: 1.05;
}

.spring-update-hero-copy small {
  color: #6d7d68;
  font-size: 12px;
}

.spring-update-hero-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
  flex: 0 0 auto;
}

.spring-update-summary {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.spring-update-summary-head {
  color: #334737;
  font-size: 12px;
  font-weight: 700;
}

.spring-update-summary-body {
  margin: 0;
  max-height: 132px;
  overflow: auto;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(114, 141, 97, 0.1);
  background: rgba(255, 255, 255, 0.58);
  color: #435547;
  font: 12px/1.5 'Cascadia Code', 'Consolas', monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.spring-update-progress-shell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid rgba(114, 141, 97, 0.1);
  background: rgba(255, 255, 255, 0.72);
}

.spring-update-progress-shell.is-visible {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.6);
}

.spring-update-progress-head,
.spring-update-progress-meta,
.spring-update-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.spring-update-progress-head {
  color: #304935;
  font-size: 12px;
  font-weight: 600;
}

.spring-update-progress-meta {
  align-items: flex-start;
  color: #71816c;
  font-size: 11px;
}

.spring-update-progress-meta span:last-child {
  text-align: right;
  word-break: break-all;
}

.spring-update-actions {
  justify-content: flex-end;
}

.spring-update-error {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(204, 87, 74, 0.2);
  background: rgba(255, 240, 237, 0.86);
  color: #9e3d33;
  font-size: 12px;
  line-height: 1.45;
}

.spring-update-error-inline {
  padding: 8px 10px;
}

:deep(body.dark-mode) .spring-header {
  border-color: rgba(151, 184, 136, 0.14);
  background:
    linear-gradient(135deg, rgba(25, 38, 28, 0.94), rgba(40, 59, 43, 0.88)),
    rgba(21, 28, 22, 0.82);
  box-shadow:
    0 12px 28px rgba(0, 0, 0, 0.22),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:deep(body.dark-mode) .spring-brand-mark {
  background: linear-gradient(160deg, #486a4d, #314834);
  color: #edf7df;
}

:deep(body.dark-mode) .spring-brand-title,
:deep(body.dark-mode) .spring-pill {
  color: #eef6e6;
}

:deep(body.dark-mode) .spring-pill {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(168, 201, 147, 0.12);
}

:deep(body.dark-mode) .spring-pill-active {
  background: linear-gradient(135deg, rgba(96, 127, 88, 0.5), rgba(71, 97, 66, 0.44));
  color: #f7fcf1;
}

:deep(body.dark-mode) .spring-pill-update-dot {
  box-shadow:
    0 0 0 2px rgba(28, 38, 31, 0.96),
    0 2px 6px rgba(0, 0, 0, 0.28);
}

:deep(body.dark-mode) .spring-pill:hover {
  background: rgba(172, 199, 151, 0.12);
  color: #f7fcf1;
}

:deep(body.dark-mode) .spring-flow-arrow {
  color: rgba(198, 218, 187, 0.72);
}

:deep(body.dark-mode) .spring-update-hero,
:deep(body.dark-mode) .spring-update-progress-shell,
:deep(body.dark-mode) .spring-update-summary-body {
  border-color: rgba(138, 169, 125, 0.12);
  background: rgba(22, 32, 25, 0.88);
}

:deep(body.dark-mode) .spring-update-kicker,
:deep(body.dark-mode) .spring-update-progress-meta {
  color: #a9bba0;
}

:deep(body.dark-mode) .spring-update-hero-copy strong,
:deep(body.dark-mode) .spring-update-progress-head,
:deep(body.dark-mode) .spring-update-summary-head {
  color: #eef6e6;
}

:deep(body.dark-mode) .spring-update-hero-copy small,
:deep(body.dark-mode) .spring-update-summary-body {
  color: #c8d6c2;
}

:deep(body.dark-mode) .spring-update-error {
  background: rgba(75, 34, 30, 0.88);
  border-color: rgba(180, 85, 74, 0.24);
  color: #f3c0ba;
}

:deep(body.gaia-dark) .spring-header {
  border-color: rgba(101, 129, 138, 0.2);
  background:
    linear-gradient(135deg, rgba(8, 15, 19, 0.98), rgba(16, 26, 32, 0.94)),
    rgba(8, 14, 18, 0.92);
  box-shadow:
    0 16px 36px rgba(0, 0, 0, 0.3),
    inset 0 1px 0 rgba(180, 214, 226, 0.04);
}

:deep(body.gaia-dark) .spring-header::after {
  content: '';
  position: absolute;
  inset: auto 14px 0 14px;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(161, 190, 198, 0.28), rgba(164, 125, 88, 0.2), transparent);
  pointer-events: none;
}

:deep(body.gaia-dark) .spring-brand-mark {
  background: linear-gradient(160deg, #243841, #15242b);
  color: #eef6f4;
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.28);
}

:deep(body.gaia-dark) .spring-brand-title,
:deep(body.gaia-dark) .spring-pill {
  color: #dde9e7;
}

:deep(body.gaia-dark) .spring-pill {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0.015));
  border-color: rgba(101, 129, 138, 0.16);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
}

:deep(body.gaia-dark) .spring-pill-active {
  background: linear-gradient(135deg, rgba(58, 83, 93, 0.88), rgba(36, 53, 61, 0.78));
  border-color: rgba(127, 160, 171, 0.28);
  color: #f4faf8;
  box-shadow:
    0 10px 22px rgba(0, 0, 0, 0.24),
    inset 0 1px 0 rgba(180, 214, 226, 0.06);
}

:deep(body.gaia-dark) .spring-pill:hover {
  background: rgba(88, 116, 126, 0.18);
  color: #f4faf8;
}

:deep(body.gaia-dark) .spring-flow-arrow {
  color: rgba(137, 159, 168, 0.72);
}

:deep(body.gaia-dark) .spring-pill-update-dot {
  box-shadow:
    0 0 0 2px rgba(11, 18, 23, 0.96),
    0 3px 8px rgba(0, 0, 0, 0.3);
}

:deep(body.gaia-dark) .spring-update-hero,
:deep(body.gaia-dark) .spring-update-progress-shell,
:deep(body.gaia-dark) .spring-update-summary-body {
  border-color: rgba(95, 121, 129, 0.22);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.03), rgba(255, 255, 255, 0.015)),
    rgba(10, 18, 23, 0.92);
}

:deep(body.gaia-dark) .spring-update-kicker,
:deep(body.gaia-dark) .spring-update-progress-meta {
  color: #93a8ad;
}

:deep(body.gaia-dark) .spring-update-hero-copy strong,
:deep(body.gaia-dark) .spring-update-progress-head,
:deep(body.gaia-dark) .spring-update-summary-head {
  color: #eef6f4;
}

:deep(body.gaia-dark) .spring-update-hero-copy small,
:deep(body.gaia-dark) .spring-update-summary-body {
  color: #bfd0d3;
}

:deep(body.gaia-dark) .spring-update-error {
  background: rgba(74, 28, 27, 0.88);
  border-color: rgba(180, 88, 80, 0.3);
  color: #efc1bc;
}

@media (max-width: 680px) {
  .spring-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .spring-toolbar {
    justify-content: flex-start;
  }

  .spring-update-hero-top,
  .spring-update-progress-head,
  .spring-update-progress-meta,
  .spring-update-actions {
    flex-direction: column;
    align-items: flex-start;
  }

  .spring-update-hero-side,
  .spring-update-actions {
    width: 100%;
  }

  .spring-update-hero-side {
    align-items: flex-start;
  }

}

.spring-header-gaia {
  border-color: rgba(101, 129, 138, 0.2);
  background:
    linear-gradient(135deg, rgba(8, 15, 19, 0.98), rgba(16, 26, 32, 0.94)),
    rgba(8, 14, 18, 0.92);
  box-shadow:
    0 16px 36px rgba(0, 0, 0, 0.3),
    inset 0 1px 0 rgba(180, 214, 226, 0.04);
}

.spring-header-gaia::after {
  content: '';
  position: absolute;
  inset: auto 14px 0 14px;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(161, 190, 198, 0.28), rgba(164, 125, 88, 0.2), transparent);
  pointer-events: none;
}

.spring-header-gaia .spring-brand-mark {
  background: linear-gradient(160deg, #243841, #15242b);
  color: #eef6f4;
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.28);
}

.spring-header-gaia .spring-brand-title,
.spring-header-gaia .spring-pill {
  color: #dde9e7;
}

.spring-header-gaia .spring-pill {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0.015));
  border-color: rgba(101, 129, 138, 0.16);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
}

.spring-header-gaia .spring-pill-active {
  background: linear-gradient(135deg, rgba(58, 83, 93, 0.88), rgba(36, 53, 61, 0.78));
  border-color: rgba(127, 160, 171, 0.28);
  color: #f4faf8;
  box-shadow:
    0 10px 22px rgba(0, 0, 0, 0.24),
    inset 0 1px 0 rgba(180, 214, 226, 0.06);
}

.spring-header-gaia .spring-pill:hover {
  background: rgba(88, 116, 126, 0.18);
  color: #f4faf8;
}

.spring-header-gaia .spring-flow-arrow {
  color: rgba(137, 159, 168, 0.72);
}

.spring-header-gaia .spring-pill-update-dot {
  box-shadow:
    0 0 0 2px rgba(11, 18, 23, 0.96),
    0 3px 8px rgba(0, 0, 0, 0.3);
}
</style>
