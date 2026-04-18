<template>
  <div class="desktop-config-view">
    <div class="desktop-config-shell">
      <div class="desktop-config-window-header">
        <div class="desktop-config-window-copy">
          <div class="desktop-config-window-title">专属一键配置</div>
          <div class="desktop-config-window-subtitle">
            {{ targetRecord ? `${targetRecord.siteName} | ${targetRecord.siteUrl}` : '选择一个密钥配置并生成预览' }}
          </div>
        </div>

        <div class="desktop-config-window-actions">
          <a-button @click="closeWindow">关闭</a-button>
          <a-button
            type="primary"
            :loading="desktopConfigLoading"
            :disabled="!targetRecord"
            @click="generateDesktopConfigPreview"
          >
            生成变更预览
          </a-button>
        </div>
      </div>

      <a-alert
        v-if="targetRecord"
        type="info"
        show-icon
        class="desktop-config-alert"
        :message="`${targetRecord.siteName} | ${targetRecord.siteUrl}`"
        description="将读取本机应用配置，生成变更预览，确认后才会真正写入。"
      />

      <a-empty
        v-if="!targetRecord"
        description="未找到目标密钥记录，无法打开专属配置窗口。"
      />

      <div v-else class="desktop-config-layout">
        <section class="desktop-app-panel">
          <div class="desktop-panel-title">目标应用</div>
          <div class="desktop-panel-hint">默认不勾选，按需点选后再生成变更预览。</div>
          <div class="desktop-app-grid">
            <button
              v-for="app in DESKTOP_CONFIG_APPS"
              :key="app.id"
              type="button"
              class="desktop-app-card"
              :class="[`desktop-app-${app.id}`, { 'desktop-app-card-active': isDesktopAppSelected(app.id) }]"
              @click="toggleDesktopAppSelection(app.id)"
            >
              <span class="desktop-app-logo">
                <img :src="DESKTOP_APP_ICONS[app.id]" :alt="app.label" class="desktop-app-logo-image" />
              </span>
              <span class="desktop-app-name">{{ app.label }}</span>
            </button>
          </div>
        </section>

        <section class="desktop-form-panel">
          <a-form layout="vertical">
            <div class="config-grid">
              <a-form-item label="Provider 名称">
                <a-input v-model:value="desktopConfigDraft.providerName" placeholder="例如 My Provider" />
              </a-form-item>

              <a-form-item label="Provider Key">
                <a-input
                  v-model:value="desktopConfigDraft.providerKey"
                  :readonly="desktopConfigDraft.forceCustomProviderKey !== false"
                  :placeholder="desktopConfigDraft.forceCustomProviderKey !== false ? 'custom' : '请输入 provider key'"
                />
                <a-checkbox
                  :checked="desktopConfigDraft.forceCustomProviderKey !== false"
                  class="desktop-provider-checkbox"
                  @change="handleDesktopProviderKeyModeChange"
                >
                  custom:统一化保证历史会话可见
                </a-checkbox>
                <div class="desktop-field-hint">
                  默认勾选会统一写入 `custom`；取消后保持各应用修改前的当前 provider key。
                </div>
              </a-form-item>

              <a-form-item label="API Key">
                <a-input-password v-model:value="desktopConfigDraft.apiKey" placeholder="sk-..." />
              </a-form-item>

              <a-form-item label="默认模型">
                <a-select
                  v-model:value="desktopConfigDraft.model"
                  :options="desktopConfigModelOptions"
                  :loading="modelLoading"
                  show-search
                  :filter-option="true"
                  option-filter-prop="label"
                  placeholder="请选择当前记录模型"
                  @dropdownVisibleChange="handleModelDropdownVisibleChange"
                  @change="handleDesktopModelChange"
                />
              </a-form-item>

              <a-form-item label="Claude Base URL">
                <a-input v-model:value="desktopConfigDraft.claudeBaseUrl" />
              </a-form-item>

              <a-form-item label="Claude Key 字段">
                <a-select v-model:value="desktopConfigDraft.claudeApiKeyField">
                  <a-select-option value="ANTHROPIC_AUTH_TOKEN">ANTHROPIC_AUTH_TOKEN</a-select-option>
                  <a-select-option value="ANTHROPIC_API_KEY">ANTHROPIC_API_KEY</a-select-option>
                </a-select>
              </a-form-item>

              <a-form-item label="Claude 高级代理">
                <a-switch v-model:checked="desktopConfigDraft.claudeUseAdvancedProxy" />
                <div class="desktop-field-hint">开启后会把 Claude Base URL 改写到本机高级代理地址，并由 All API Deck 负责兼容 OpenAI vendor、故障转移和错误修正。</div>
              </a-form-item>

              <a-form-item label="Codex Base URL">
                <a-input v-model:value="desktopConfigDraft.codexBaseUrl" />
              </a-form-item>

              <a-form-item label="Codex 高级代理">
                <a-switch v-model:checked="desktopConfigDraft.codexUseAdvancedProxy" />
                <div class="desktop-field-hint">开启后会把 Codex 的 `base_url` 改写到本地代理，并使用占位 Key。</div>
              </a-form-item>

              <a-form-item label="OpenCode Base URL">
                <a-input v-model:value="desktopConfigDraft.opencodeBaseUrl" />
              </a-form-item>

              <a-form-item label="OpenCode Adapter">
                <a-select v-model:value="desktopConfigDraft.opencodeNpm">
                  <a-select-option value="@ai-sdk/openai-compatible">@ai-sdk/openai-compatible</a-select-option>
                  <a-select-option value="@openrouter/ai-sdk-provider">@openrouter/ai-sdk-provider</a-select-option>
                </a-select>
              </a-form-item>

              <a-form-item label="OpenCode 高级代理">
                <a-switch v-model:checked="desktopConfigDraft.opencodeUseAdvancedProxy" />
                <div class="desktop-field-hint">开启后会改写到本地 OpenAI 兼容代理入口，并固定使用 openai-compatible 适配器。</div>
              </a-form-item>

              <a-form-item label="OpenClaw Base URL">
                <a-input v-model:value="desktopConfigDraft.openclawBaseUrl" />
              </a-form-item>

              <a-form-item label="OpenClaw API 协议">
                <a-select v-model:value="desktopConfigDraft.openclawApi">
                  <a-select-option value="openai-completions">openai-completions</a-select-option>
                  <a-select-option value="anthropic-messages">anthropic-messages</a-select-option>
                </a-select>
              </a-form-item>

              <a-form-item label="OpenClaw 高级代理">
                <a-switch v-model:checked="desktopConfigDraft.openclawUseAdvancedProxy" />
                <div class="desktop-field-hint">开启后会改写到本地 OpenClaw 代理入口，并切到 openai-completions 协议。</div>
              </a-form-item>
            </div>
          </a-form>
        </section>
      </div>
    </div>

    <DesktopConfigDiffModal
      :open="desktopConfigDiffOpen"
      :preview="desktopConfigPreview"
      :width="1500"
      @cancel="desktopConfigDiffOpen = false"
      @confirm="applyDesktopConfigPreview"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { message } from 'ant-design-vue';
import { GetLaunchRecordKey, RequestQuit } from '../../wailsjs/go/main/App.js';
import DesktopConfigDiffModal from '../components/DesktopConfigDiffModal.vue';
import { applyManagedAppConfigFiles, isDesktopConfigBridgeAvailable, readManagedAppConfigFiles } from '../utils/desktopConfigBridge.js';
import { buildDesktopConfigPreview, createDesktopConfigDraft, DESKTOP_CONFIG_APPS, inferProviderKeyFromSnapshot } from '../utils/desktopConfigTransform.js';
import { getRecordModelOptions, loadPanelRecords, loadRecordModelOptions, persistPanelRecords } from '../utils/keyPanelStore.js';
import { hydrateLastResultsSnapshotCache } from '../utils/historySnapshotStore.js';
import claudeAppIcon from '../assets/app-icons/claude.svg';
import codexAppIcon from '../assets/app-icons/codex.svg';
import opencodeAppIcon from '../assets/app-icons/opencode.svg';
import openclawAppIcon from '../assets/app-icons/openclaw-fallback.svg';

const DESKTOP_APP_ICONS = {
  claude: claudeAppIcon,
  codex: codexAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
};

const records = ref([]);
const contextMap = ref(new Map());
const targetRecord = ref(null);
const desktopConfigLoading = ref(false);
const desktopConfigDiffOpen = ref(false);
const desktopConfigPreview = ref({ appGroups: [], writes: [], errors: [] });
const desktopConfigDraft = reactive(createDesktopConfigDraft({}));
const modelLoading = ref(false);
const desktopProviderKeyManualValue = ref('');

const desktopConfigModelOptions = computed(() => {
  const record = targetRecord.value;
  if (!record) return [];
  const options = getRecordModelOptions(record, contextMap.value);
  const currentValue = String(desktopConfigDraft.model || '').trim();
  if (!currentValue) return options;
  return options.some(option => option.value === currentValue)
    ? options
    : [{ label: currentValue, value: currentValue }, ...options];
});

function overwriteDesktopConfigDraft(nextDraft) {
  Object.keys(desktopConfigDraft).forEach(key => delete desktopConfigDraft[key]);
  Object.assign(desktopConfigDraft, nextDraft);
  desktopProviderKeyManualValue.value = String(nextDraft?.providerKey || '').trim();
}

function replaceTargetRecord(nextRecord, persist = false) {
  targetRecord.value = nextRecord;
  if (!nextRecord) return;
  records.value = records.value.map(item => (item.rowKey === nextRecord.rowKey ? nextRecord : item));
  if (persist) {
    persistPanelRecords(records.value);
  }
}

async function closeWindow() {
  await RequestQuit();
}

function isDesktopAppSelected(appId) {
  return Array.isArray(desktopConfigDraft.selectedApps) && desktopConfigDraft.selectedApps.includes(appId);
}

function toggleDesktopAppSelection(appId) {
  const current = Array.isArray(desktopConfigDraft.selectedApps) ? [...desktopConfigDraft.selectedApps] : [];
  if (current.includes(appId)) {
    desktopConfigDraft.selectedApps = current.filter(item => item !== appId);
  } else {
    desktopConfigDraft.selectedApps = [...current, appId];
  }
  if (desktopConfigDraft.forceCustomProviderKey === false) {
    void syncDesktopProviderKeyFromSnapshot();
  }
}

function handleDesktopProviderKeyModeChange(event) {
  const checked = Boolean(event?.target?.checked);
  if (!checked && desktopConfigDraft.forceCustomProviderKey === false) {
    desktopProviderKeyManualValue.value = String(desktopConfigDraft.providerKey || '').trim();
  }
  desktopConfigDraft.forceCustomProviderKey = checked;
  const fallbackManualValue = String(desktopProviderKeyManualValue.value || '').trim();
  desktopConfigDraft.providerKey = checked
    ? 'custom'
    : (fallbackManualValue && fallbackManualValue !== 'custom' ? fallbackManualValue : '');
  if (!checked && !String(desktopConfigDraft.providerKey || '').trim()) {
    void syncDesktopProviderKeyFromSnapshot();
  }
}

async function syncDesktopProviderKeyFromSnapshot() {
  if (!isDesktopConfigBridgeAvailable()) return;
  try {
    const selectedApps = Array.isArray(desktopConfigDraft.selectedApps) && desktopConfigDraft.selectedApps.length
      ? desktopConfigDraft.selectedApps
      : DESKTOP_CONFIG_APPS.map(app => app.id);
    const snapshot = await readManagedAppConfigFiles(selectedApps);
    const inferred = inferProviderKeyFromSnapshot(snapshot, desktopConfigDraft, selectedApps);
    const detected = String(inferred?.providerKey || '').trim();
    if (!detected) return;
    desktopProviderKeyManualValue.value = detected;
    if (desktopConfigDraft.forceCustomProviderKey === false) {
      desktopConfigDraft.providerKey = detected;
    }
  } catch {}
}

function handleDesktopModelChange(value) {
  const selectedModel = String(value || '').trim();
  desktopConfigDraft.model = selectedModel;
  if (!targetRecord.value) return;
  replaceTargetRecord({
    ...targetRecord.value,
    selectedModel,
  }, true);
}

async function handleModelDropdownVisibleChange(open) {
  if (!open || !targetRecord.value || modelLoading.value) return;

  modelLoading.value = true;
  try {
    const nextRecord = await loadRecordModelOptions(targetRecord.value, contextMap.value, true);
    replaceTargetRecord(nextRecord, true);
    const currentModel = String(desktopConfigDraft.model || '').trim();
    if (!currentModel || !nextRecord.modelsList?.includes(currentModel)) {
      desktopConfigDraft.model = String(nextRecord.selectedModel || '').trim();
    }
  } catch (error) {
    message.error(error?.message || '获取模型列表失败');
  } finally {
    modelLoading.value = false;
  }
}

async function generateDesktopConfigPreview() {
  if (!targetRecord.value) {
    message.warning('未找到可用密钥记录');
    return;
  }
  if (!desktopConfigDraft.selectedApps.length) {
    message.warning('请至少选择一个目标应用');
    return;
  }
  if (!isDesktopConfigBridgeAvailable()) {
    message.warning('当前环境不支持桌面配置读写');
    return;
  }

  desktopConfigLoading.value = true;
  try {
    const snapshot = await readManagedAppConfigFiles(desktopConfigDraft.selectedApps);
    const preview = buildDesktopConfigPreview(desktopConfigDraft, snapshot);
    desktopConfigPreview.value = preview;
    if (!preview.appGroups.length && preview.errors.length) {
      throw new Error(preview.errors.join('；'));
    }
    desktopConfigDiffOpen.value = true;
    if (preview.errors.length) {
      message.warning(`部分应用预览生成失败：${preview.errors.join('；')}`);
    } else {
      message.success(`已生成 ${preview.writes.length} 个配置文件的变更预览`);
    }
  } catch (error) {
    console.error(error);
    message.error(`生成配置预览失败：${error.message || '未知错误'}`);
  } finally {
    desktopConfigLoading.value = false;
  }
}

async function applyDesktopConfigPreview() {
  if (!desktopConfigPreview.value.writes.length) {
    message.warning('没有可写入的配置变更');
    return;
  }

  desktopConfigLoading.value = true;
  try {
    const result = await applyManagedAppConfigFiles(desktopConfigPreview.value.writes);
    const appliedCount = Array.isArray(result?.applied) ? result.applied.length : 0;
    desktopConfigDiffOpen.value = false;
    message.success(`已写入 ${appliedCount} 个本地配置文件，并自动创建备份`);
    window.setTimeout(() => {
      void closeWindow();
    }, 160);
  } catch (error) {
    console.error(error);
    message.error(`写入本地配置失败：${error.message || '未知错误'}`);
  } finally {
    desktopConfigLoading.value = false;
  }
}

async function bootstrap() {
  await hydrateLastResultsSnapshotCache();
  const loaded = loadPanelRecords();
  contextMap.value = loaded.contextMap || new Map();
  records.value = loaded.records || [];

  const rowKey = await GetLaunchRecordKey().catch(() => '');
  const matched = rowKey
    ? records.value.find(item => item.rowKey === rowKey) || null
    : (records.value[0] || null);

  if (!matched) {
    message.error('未找到对应的密钥记录');
    return;
  }

  replaceTargetRecord(matched, false);
  overwriteDesktopConfigDraft(createDesktopConfigDraft(matched));
  void syncDesktopProviderKeyFromSnapshot();
}

onMounted(() => {
  void bootstrap();
});
</script>

<style scoped>
.desktop-config-view {
  min-height: 100vh;
  padding: 10px;
  box-sizing: border-box;
  background: linear-gradient(180deg, #eef4ea 0%, #e5efe0 100%);
  overflow: hidden;
}

.desktop-config-shell {
  max-width: 100%;
  margin: 0 auto;
  min-height: calc(100vh - 20px);
  padding: 12px 14px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.12);
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow: hidden;
}

.desktop-config-window-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 0;
}

.desktop-config-window-copy {
  min-width: 0;
}

.desktop-config-window-title {
  font-size: 22px;
  line-height: 1.1;
  font-weight: 800;
  color: #1f2937;
}

.desktop-config-window-subtitle {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  word-break: break-all;
  line-height: 1.4;
}

.desktop-config-window-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}

.desktop-config-alert {
  margin-bottom: 0;
}

.desktop-config-layout {
  display: grid;
  grid-template-columns: 228px minmax(0, 1fr);
  gap: 12px;
  align-items: stretch;
  min-height: 0;
  flex: 1 1 auto;
}

.desktop-app-panel,
.desktop-form-panel {
  min-height: 0;
  border-radius: 18px;
  background: linear-gradient(180deg, #f8fafc, #eef2ff);
  padding: 12px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  overflow: auto;
}

.desktop-panel-title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.desktop-panel-hint,
.desktop-field-hint {
  margin-top: 4px;
  color: #64748b;
  font-size: 11px;
  line-height: 1.35;
}

.desktop-app-grid {
  margin-top: 10px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.desktop-app-card {
  border: 0;
  border-radius: 16px;
  padding: 10px 8px;
  background: #fff;
  color: #0f172a;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08), inset 0 0 0 1px rgba(148, 163, 184, 0.16);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, background 0.18s ease;
}

.desktop-app-card:hover {
  transform: translateY(-2px);
}

.desktop-app-card-active {
  box-shadow: 0 14px 30px rgba(37, 99, 235, 0.16), inset 0 0 0 2px rgba(37, 99, 235, 0.45);
  background: linear-gradient(180deg, #ffffff, #eff6ff);
}

.desktop-app-logo {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #f8fafc;
  padding: 8px;
}

.desktop-app-logo-image {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.desktop-app-name {
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
  text-align: center;
}

.desktop-app-claude .desktop-app-logo {
  background: linear-gradient(135deg, #fff7ed, #ffedd5);
}

.desktop-app-codex .desktop-app-logo {
  background: linear-gradient(135deg, #ffffff, #f3f4f6);
}

.desktop-app-opencode .desktop-app-logo {
  background: linear-gradient(135deg, #eef2ff, #dbeafe);
}

.desktop-app-openclaw .desktop-app-logo {
  background: linear-gradient(135deg, #fff1f2, #ffe4e6);
}

.desktop-provider-checkbox {
  margin-top: 6px;
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}

 :deep(.desktop-form-panel .ant-form-item) {
  margin-bottom: 10px;
}

:deep(.desktop-form-panel .ant-form-item-label > label) {
  font-size: 12px;
  line-height: 1.2;
  height: auto;
}

:deep(.desktop-form-panel .ant-input),
:deep(.desktop-form-panel .ant-input-password),
:deep(.desktop-form-panel .ant-select-selector),
:deep(.desktop-form-panel .ant-btn) {
  min-height: 32px;
  font-size: 12px;
}

:deep(.desktop-form-panel .ant-select-single:not(.ant-select-customize-input) .ant-select-selector) {
  height: 32px;
}

:deep(.desktop-form-panel .ant-select-single .ant-select-selector .ant-select-selection-item),
:deep(.desktop-form-panel .ant-select-single .ant-select-selector .ant-select-selection-placeholder) {
  line-height: 30px;
}

@media (max-width: 720px) {
  .desktop-config-view {
    padding: 12px;
    overflow: auto;
  }

  .desktop-config-shell {
    padding: 16px;
    min-height: auto;
    overflow: visible;
  }

  .desktop-config-window-header {
    flex-direction: column;
    align-items: stretch;
  }

  .desktop-config-window-actions {
    width: 100%;
    justify-content: flex-end;
  }

  .desktop-config-layout {
    grid-template-columns: 1fr;
    min-height: auto;
  }

  .config-grid {
    grid-template-columns: 1fr;
  }
}
</style>
