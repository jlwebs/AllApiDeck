<template>
  <ConfigProvider :theme="configProviderTheme">
    <div class="key-management">
      <AppHeader current-page="keys" :is-dark-mode="isDarkMode" @toggle-theme="handleToggleTheme" @experimental="showExperimentalFeatures = true" @settings="openSettingsHint" />

      <a-card title="批量同步密钥到本地库" class="sync-card">
        <div class="sync-toolbar">
          <div class="sync-meta">
            <span>本地记录：{{ tableData.length }}</span>
            <span>状态正常：{{ healthyKeyCount }}</span>
            <span>上次同步：{{ formatDateTime(syncMeta.lastBatchSyncAt) }}</span>
          </div>
        </div>

        <div v-if="loading" class="sync-loading"><a-spin /><span>正在批量提取 sk key，并写入 localStorage...</span></div>
        <a-alert v-if="!loading && failedSites.length > 0" type="warning" show-icon class="sync-alert" :message="`${failedSites.length} 个站点本次未获取到 key，详见 logs/fetch-keys.log`" :description="failedSiteNames" />
        <a-alert v-if="!loading" type="info" show-icon class="sync-alert" :message="syncSummary" />
      </a-card>

      <a-card title="本地密钥管理" class="inventory-card">
        <template #extra>
          <a-space wrap>
            <a-button @click="openManualRecordModal()">手工添加</a-button>
            <a-popover trigger="hover" placement="bottom">
              <template #content>
                <div class="import-export-menu">
                  <a-button size="small" block @click="exportAllValidKeysPackage">导出全部有效 Key</a-button>
                  <a-button size="small" block @click="importFromClipboardPackage">从剪贴板导入</a-button>
                </div>
              </template>
              <a-button type="primary">导入/导出</a-button>
            </a-popover>
            <a-button :disabled="displayedRows.length === 0" @click="exportCsv">导出 CSV</a-button>
            <a-popconfirm title="确认清空本地密钥库？" ok-text="清空" cancel-text="取消" @confirm="clearLocalRecords">
              <a-button danger :disabled="displayedRows.length === 0">清空本地库</a-button>
            </a-popconfirm>
          </a-space>
        </template>

        <a-empty v-if="displayedRows.length === 0" description="暂无本地密钥记录，可从批量检测自动同步、剪贴板导入或手工添加。" />
        <a-table v-else :columns="columns" :data-source="displayedRows" :row-key="record => record.rowKey" :pagination="{ pageSize: 20, showSizeChanger: true, pageSizeOptions: ['20', '50', '100'] }" :scroll="{ x: 1560 }" size="middle">
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'siteName'">
              <div class="site-cell">
                <div class="site-heading">
                  <a-tag v-if="record.sourceType === 'manual'" color="blue">手工添加</a-tag>
                  <strong>{{ record.siteName }}</strong>
                </div>
                <span class="subtle-text">{{ record.tokenName || '未命名 Token' }}</span>
              </div>
            </template>
            <template v-else-if="column.dataIndex === 'apiKey'">
              <a-typography-text :copyable="{ text: record.apiKey }" :ellipsis="{ tooltip: record.apiKey }" class="cell-copy-text">{{ maskApiKey(record.apiKey) }}</a-typography-text>
            </template>
            <template v-else-if="column.dataIndex === 'siteUrl'">
              <a-typography-text :copyable="{ text: record.siteUrl }" :ellipsis="{ tooltip: record.siteUrl }" class="cell-copy-text">{{ record.siteUrl }}</a-typography-text>
            </template>
            <template v-else-if="column.dataIndex === 'modelsText'">
              <a-tooltip :title="record.modelsText || '未提供模型信息'"><span class="models-text">{{ record.modelsText || '未提供模型信息' }}</span></a-tooltip>
            </template>
            <template v-else-if="column.dataIndex === 'status'">
              <a-tag :color="record.status === 1 ? 'green' : 'red'">{{ record.status === 1 ? '正常' : '禁用/异常' }}</a-tag>
            </template>
            <template v-else-if="column.dataIndex === 'exportActions'">
              <div class="inline-export-actions">
                <a-tooltip title="便捷一键设置">
                  <button type="button" class="export-icon-button export-desktop" @click="openDesktopConfigWizard(record)">
                    <img :src="quickSetupIcon" alt="便捷一键设置" class="export-icon-image" />
                  </button>
                </a-tooltip>
                <a-tooltip title="导出到 Cherry Studio">
                  <button type="button" class="export-icon-button export-cherry" @click="launchCherryStudio(record)">
                    <span class="export-icon-glyph">🍒</span>
                  </button>
                </a-tooltip>
                <a-popover trigger="hover" placement="bottom">
                  <template #content>
                    <div class="switch-app-menu">
                      <button
                        v-for="app in CC_SWITCH_TARGET_APPS"
                        :key="app"
                        type="button"
                        class="switch-app-item"
                        @click="launchCCSwitch(record, app)"
                      >
                        {{ app }}
                      </button>
                    </div>
                  </template>
                  <a-tooltip title="导出到 CC Switch">
                    <button type="button" class="export-icon-button export-switch">
                      <img :src="ccSwitchIcon" alt="CC Switch" class="export-icon-image export-icon-image-switch" />
                    </button>
                  </a-tooltip>
                </a-popover>
                <a-tooltip title="复制为单个 sk:// 导入命令">
                  <button type="button" class="export-icon-button export-copy" @click="copySingleImportCommand(record)">
                    <span class="export-icon-glyph">⧉</span>
                  </button>
                </a-tooltip>
              </div>
            </template>
            <template v-else-if="column.dataIndex === 'quickTest'">
              <div class="quick-test-cell">
                <a-button type="primary" size="small" :loading="record.quickTestLoading" @click="runQuickTest(record)">快速测有效</a-button>
                <a-tooltip :title="getQuickTestTooltip(record)">
                  <a-tag v-if="record.quickTestStatus" :color="getQuickTestColor(record.quickTestStatus)" class="quick-test-tag">{{ record.quickTestLabel || record.quickTestStatus }}</a-tag>
                  <span v-else class="subtle-text">未测速</span>
                </a-tooltip>
              </div>
            </template>
            <template v-else-if="column.dataIndex === 'updatedAt'">
              <div class="time-cell"><span>{{ formatDateTime(record.updatedAt) }}</span><span class="subtle-text">{{ record.quickTestAt ? `快测 ${formatDateTime(record.quickTestAt)}` : '暂无快测记录' }}</span></div>
            </template>
            <template v-else-if="column.dataIndex === 'rowActions'">
              <a-space wrap size="small">
                <a-button size="small" @click="openManualRecordModal(record)">编辑</a-button>
                <a-popconfirm title="确认删除这条记录？" ok-text="删除" cancel-text="取消" @confirm="deleteRecord(record)">
                  <a-button size="small" danger>删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-card>

      <a-modal v-model:open="manualRecordModalOpen" :title="manualRecordEditing ? '编辑记录' : '手工添加记录'" :confirm-loading="manualRecordSaving" ok-text="保存" cancel-text="取消" width="820px" @ok="submitManualRecord">
        <a-form layout="vertical">
          <div class="config-grid">
            <a-form-item label="网站名称">
              <a-input v-model:value="manualRecordDraft.siteName" placeholder="例如 My Site" />
            </a-form-item>
            <a-form-item label="Token 名称">
              <a-input v-model:value="manualRecordDraft.tokenName" placeholder="可选" />
            </a-form-item>
            <a-form-item label="接口地址">
              <a-input v-model:value="manualRecordDraft.siteUrl" placeholder="https://example.com" />
            </a-form-item>
            <a-form-item label="API Key">
              <a-input-password v-model:value="manualRecordDraft.apiKey" placeholder="sk-..." />
            </a-form-item>
            <a-form-item label="模型候选">
              <a-select
                v-model:value="manualRecordDraft.modelsValue"
                mode="tags"
                :options="manualModelOptions"
                :loading="manualModelLoading"
                :filter-option="true"
                :token-separators="[',', '，', ' ']"
                placeholder="切换到这里会自动抓取模型，支持多选"
                @dropdownVisibleChange="handleManualModelDropdownVisibleChange"
                @change="handleManualModelSelectionChange"
              />
            </a-form-item>
            <a-form-item label="状态">
              <a-select v-model:value="manualRecordDraft.status">
                <a-select-option :value="1">正常</a-select-option>
                <a-select-option :value="2">禁用/异常</a-select-option>
              </a-select>
            </a-form-item>
          </div>
        </a-form>
      </a-modal>

      <a-modal v-model:open="desktopConfigModalOpen" title="专属一键配置" :confirm-loading="desktopConfigLoading" ok-text="生成变更预览" cancel-text="取消" width="1120px" @ok="generateDesktopConfigPreview">
        <div v-if="desktopConfigTargetRecord" class="desktop-config-modal">
          <a-alert type="info" show-icon class="desktop-config-alert" :message="`${desktopConfigTargetRecord.siteName} | ${desktopConfigTargetRecord.siteUrl}`" :description="`将读取本机应用配置，生成变更预览，确认后才会真正写入。`" />
          <div class="desktop-config-layout">
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
                  <a-form-item label="Provider 名称"><a-input v-model:value="desktopConfigDraft.providerName" placeholder="例如 My Provider" /></a-form-item>
                  <a-form-item label="Provider Key"><a-input v-model:value="desktopConfigDraft.providerKey" placeholder="例如 my-provider" /></a-form-item>
                  <a-form-item label="API Key"><a-input-password v-model:value="desktopConfigDraft.apiKey" placeholder="sk-..." /></a-form-item>
                  <a-form-item label="默认模型"><a-input v-model:value="desktopConfigDraft.model" placeholder="例如 gpt-4o-mini" /></a-form-item>
                  <a-form-item label="Claude Base URL"><a-input v-model:value="desktopConfigDraft.claudeBaseUrl" /></a-form-item>
                  <a-form-item label="Claude Key 字段"><a-select v-model:value="desktopConfigDraft.claudeApiKeyField"><a-select-option value="ANTHROPIC_AUTH_TOKEN">ANTHROPIC_AUTH_TOKEN</a-select-option><a-select-option value="ANTHROPIC_API_KEY">ANTHROPIC_API_KEY</a-select-option></a-select></a-form-item>
                  <a-form-item label="Codex Base URL"><a-input v-model:value="desktopConfigDraft.codexBaseUrl" /></a-form-item>
                  <a-form-item label="OpenCode Base URL"><a-input v-model:value="desktopConfigDraft.opencodeBaseUrl" /></a-form-item>
                  <a-form-item label="OpenCode Adapter"><a-select v-model:value="desktopConfigDraft.opencodeNpm"><a-select-option value="@ai-sdk/openai-compatible">@ai-sdk/openai-compatible</a-select-option><a-select-option value="@openrouter/ai-sdk-provider">@openrouter/ai-sdk-provider</a-select-option></a-select></a-form-item>
                  <a-form-item label="OpenClaw Base URL"><a-input v-model:value="desktopConfigDraft.openclawBaseUrl" /></a-form-item>
                  <a-form-item label="OpenClaw API 协议"><a-select v-model:value="desktopConfigDraft.openclawApi"><a-select-option value="openai-completions">openai-completions</a-select-option><a-select-option value="anthropic-messages">anthropic-messages</a-select-option></a-select></a-form-item>
                </div>
              </a-form>
            </section>
          </div>
        </div>
      </a-modal>

      <DesktopConfigDiffModal :open="desktopConfigDiffOpen" :preview="desktopConfigPreview" @cancel="desktopConfigDiffOpen = false" @confirm="applyDesktopConfigPreview" />
      <a-modal v-model:open="showExperimentalFeatures" title="实验功能" :footer="null" @cancel="showExperimentalFeatures = false"><div class="experimental-modal"><p>实验功能仍在整理中，后续会继续补充。</p></div></a-modal>
    </div>
  </ConfigProvider>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { ConfigProvider, message, theme } from 'ant-design-vue';
import AppHeader from './AppHeader.vue';
import DesktopConfigDiffModal from './DesktopConfigDiffModal.vue';
import { fetchModelList } from '../utils/api.js';
import { maskApiKey } from '../utils/normal.js';
import { apiFetch } from '../utils/runtimeApi.js';
import { toggleTheme } from '../utils/theme.js';
import { applyManagedAppConfigFiles, isDesktopConfigBridgeAvailable, readManagedAppConfigFiles } from '../utils/desktopConfigBridge.js';
import { buildDesktopConfigPreview, createDesktopConfigDraft, DESKTOP_CONFIG_APPS } from '../utils/desktopConfigTransform.js';
import claudeAppIcon from '../assets/app-icons/claude.svg';
import codexAppIcon from '../assets/app-icons/codex.svg';
import geminiAppIcon from '../assets/app-icons/gemini.svg';
import opencodeAppIcon from '../assets/app-icons/opencode.svg';
import openclawAppIcon from '../assets/app-icons/openclaw-fallback.svg';
import quickSetupIcon from '../assets/action-icons/quick-setup-cute.svg';
import ccSwitchIcon from '../assets/action-icons/cc-switch.png';

const STORAGE_KEY = 'api_check_key_management_records_v1';
const MANUAL_STORAGE_KEY = 'api_check_key_management_manual_records_v1';
const META_STORAGE_KEY = 'api_check_key_management_meta_v1';
const DEFAULT_TEST_TIMEOUT_MS = 20000;
const CC_SWITCH_TARGET_APPS = ['claude', 'codex', 'gemini', 'opencode', 'openclaw'];
const DESKTOP_APP_ICONS = {
  claude: claudeAppIcon,
  codex: codexAppIcon,
  gemini: geminiAppIcon,
  opencode: opencodeAppIcon,
  openclaw: openclawAppIcon,
};

function createManualRecordDraft(record = null) {
  const modelsValue = normalizeModels(record?.modelsList || record?.modelsText);
  return {
    rowKey: record?.rowKey || '',
    sourceType: record?.sourceType || 'manual',
    siteName: record?.siteName || '',
    tokenName: record?.tokenName || '',
    siteUrl: record?.siteUrl || '',
    apiKey: record?.apiKey || '',
    modelsText: modelsValue.join(', '),
    modelsValue,
    status: Number(record?.status || 1),
  };
}

const isDarkMode = ref(false);
const loading = ref(false);
const allResults = ref([]);
const tableData = ref([]);
const showExperimentalFeatures = ref(false);
const syncMeta = ref({ lastBatchSyncAt: null, lastBatchSyncCount: 0, lastBatchFailedCount: 0 });
const desktopConfigModalOpen = ref(false);
const desktopConfigDiffOpen = ref(false);
const desktopConfigLoading = ref(false);
const desktopConfigTargetRecord = ref(null);
const desktopConfigPreview = ref({ appGroups: [], writes: [], errors: [] });
const desktopConfigDraft = reactive(createDesktopConfigDraft({}));
const manualRecordModalOpen = ref(false);
const manualRecordSaving = ref(false);
const manualRecordEditing = ref(false);
const manualRecordDraft = reactive(createManualRecordDraft());
const manualModelOptions = ref([]);
const manualModelLoading = ref(false);
const manualModelFetchKey = ref('');

const configProviderTheme = computed(() => ({
  algorithm: isDarkMode.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}));
const columns = [
  { title: '网站', dataIndex: 'siteName', key: 'siteName', width: 180, fixed: 'left', sorter: (a, b) => String(a.siteName || '').localeCompare(String(b.siteName || '')) },
  { title: 'API Key', dataIndex: 'apiKey', key: 'apiKey', width: 220 },
  { title: '接口地址', dataIndex: 'siteUrl', key: 'siteUrl', width: 260, sorter: (a, b) => String(a.siteUrl || '').localeCompare(String(b.siteUrl || '')) },
  { title: '模型候选', dataIndex: 'modelsText', key: 'modelsText', width: 260 },
  { title: '状态', dataIndex: 'status', key: 'status', width: 110, sorter: (a, b) => Number(a.status || 0) - Number(b.status || 0) },
  { title: '专属导出', dataIndex: 'exportActions', key: 'exportActions', width: 180 },
  { title: '快速测有效', dataIndex: 'quickTest', key: 'quickTest', width: 220 },
  { title: '操作', dataIndex: 'rowActions', key: 'rowActions', width: 160 },
  { title: '最近同步', dataIndex: 'updatedAt', key: 'updatedAt', width: 190, sorter: (a, b) => Number(a.updatedAt || 0) - Number(b.updatedAt || 0), defaultSortOrder: 'descend' },
];
const failedSites = computed(() => allResults.value.filter(result => !Array.isArray(result?.tokens) || result.tokens.length === 0));
const failedSiteNames = computed(() => failedSites.value.map(site => site?.site_name || site?.id || '未命名站点').join('，'));
const displayedRows = computed(() => [...tableData.value].sort((a, b) => Number(b.updatedAt || 0) - Number(a.updatedAt || 0) || String(a.siteName || '').localeCompare(String(b.siteName || ''))));
const healthyKeyCount = computed(() => tableData.value.filter(record => record.status === 1).length);
const syncSummary = computed(() => !syncMeta.value.lastBatchSyncAt ? '导入 accounts-backup JSON 后，会自动把获取到的 sk key 写入 localStorage。' : `最近一次批量同步写入 ${syncMeta.value.lastBatchSyncCount} 条记录，失败站点 ${syncMeta.value.lastBatchFailedCount} 个。`);

onMounted(() => {
  if (!document.body.classList.contains('dark-mode') && !document.body.classList.contains('light-mode')) document.body.classList.add('light-mode');
  isDarkMode.value = document.body.classList.contains('dark-mode');
  tableData.value = loadStoredRecords();
  syncMeta.value = loadStoredMeta();
});

function handleToggleTheme() {
  toggleTheme(isDarkMode);
  document.body.classList.toggle('dark-mode', isDarkMode.value);
  document.body.classList.toggle('light-mode', !isDarkMode.value);
}

function openSettingsHint() {
  message.info('密钥管理页暂时没有独立设置项，可在首页继续使用通用设置。');
}

function beforeUpload(file) {
  const reader = new FileReader();
  reader.onload = async event => {
    try {
      const parsed = JSON.parse(String(event.target?.result || ''));
      const accounts = extractAccountsFromBackup(parsed);
      if (!accounts.length) {
        message.error('备份文件中未找到可用账号数据');
        return;
      }
      message.success(`已加载 ${accounts.length} 个账号，开始同步真实 sk key`);
      await processAccounts(accounts);
    } catch (error) {
      console.error(error);
      message.error(`解析备份文件失败：${error.message || '未知错误'}`);
    }
  };
  reader.readAsText(file);
  return false;
}

async function processAccounts(accounts) {
  const accountsToTarget = accounts.filter(account => !account?.disabled && account?.account_info?.access_token);
  if (accountsToTarget.length === 0) {
    message.warning('没有找到包含 access_token 的可用账号');
    return;
  }

  loading.value = true;
  allResults.value = [];
  try {
    const response = await apiFetch('/api/fetch-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ accounts: accountsToTarget }),
    });
    if (!response.ok) {
      const errorPayload = await safeReadJson(response);
      throw new Error(errorPayload?.message || '批量获取真实 key 失败');
    }

    const data = await response.json();
    const results = Array.isArray(data?.results) ? data.results : [];
    const normalizedRows = normalizeFetchedRows(results);
    const mergedRows = mergeStoredRecords(normalizedRows);
    const failedCount = results.filter(result => !Array.isArray(result?.tokens) || result.tokens.length === 0).length;

    allResults.value = results;
    tableData.value = mergedRows;
    syncMeta.value = { lastBatchSyncAt: Date.now(), lastBatchSyncCount: normalizedRows.length, lastBatchFailedCount: failedCount };
    persistRecords();
    persistMeta();
    message.success(`批量同步完成：本次获取 ${normalizedRows.length} 个 sk key，失败站点 ${failedCount} 个。`);
  } catch (error) {
    console.error(error);
    message.error(`同步失败：${error.message || '未知错误'}`);
  } finally {
    loading.value = false;
  }
}

function normalizeFetchedRows(results) {
  const normalized = [];
  results.forEach(result => {
    const tokens = Array.isArray(result?.tokens) ? result.tokens : [];
    tokens.forEach((token, index) => {
      const apiKey = normalizeApiKey(token?.key);
      const siteUrl = resolveSiteUrl(result);
      if (!apiKey || !siteUrl) return;
      const modelsList = normalizeModels(token?.models);
      normalized.push({
        rowKey: buildRowKey(siteUrl, apiKey),
        sourceType: 'auto',
        siteName: result?.site_name || '未命名站点',
        tokenName: token?.name || `未命名 Token ${index + 1}`,
        siteUrl,
        apiKey,
        modelsList,
        modelsText: modelsList.length ? modelsList.join(', ') : '未提供模型信息',
        status: typeof token?.status === 'number' ? token.status : token?.is_disabled ? 2 : 1,
        remainQuota: token?.remain_quota ?? null,
        usedQuota: token?.used_quota ?? null,
        unlimitedQuota: token?.unlimited_quota === true || token?.remain_quota === undefined || token?.remain_quota < 0,
      });
    });
  });
  return normalized;
}

function mergeStoredRecords(incomingRows) {
  const now = Date.now();
  const mergedMap = new Map();
  loadStoredRecords().forEach(record => mergedMap.set(record.rowKey, { ...record, quickTestLoading: false }));
  incomingRows.forEach(row => {
    const previous = mergedMap.get(row.rowKey);
    mergedMap.set(row.rowKey, {
      ...previous,
      ...row,
      createdAt: previous?.createdAt || now,
      updatedAt: now,
      quickTestStatus: previous?.quickTestStatus || '',
      quickTestLabel: previous?.quickTestLabel || '',
      quickTestModel: previous?.quickTestModel || '',
      quickTestRemark: previous?.quickTestRemark || '',
      quickTestAt: previous?.quickTestAt || null,
      quickTestResponseTime: previous?.quickTestResponseTime || '',
      quickTestResponseContent: previous?.quickTestResponseContent || '',
      quickTestLoading: false,
    });
  });
  return Array.from(mergedMap.values());
}

async function runQuickTest(record) {
  if (record.quickTestLoading) return;
  record.quickTestLoading = true;
  try {
    const model = await resolveQuickTestModel(record);
    const testResult = await executeQuickTest({ apiKey: record.apiKey, siteUrl: record.siteUrl, model });
    record.quickTestStatus = testResult.status;
    record.quickTestLabel = testResult.label;
    record.quickTestModel = model;
    record.quickTestRemark = testResult.remark;
    record.quickTestAt = Date.now();
    record.quickTestResponseTime = testResult.responseTime;
    record.quickTestResponseContent = testResult.responseContent || '';
    persistRecords();
    const messageMethod = testResult.status === 'success' ? 'success' : testResult.status === 'warning' ? 'warning' : 'error';
    message[messageMethod](`快测${testResult.label}：${record.siteName} / ${model}${testResult.responseTime ? ` / ${testResult.responseTime}s` : ''}`);
  } catch (error) {
    console.error(error);
    record.quickTestStatus = 'error';
    record.quickTestLabel = '失败';
    record.quickTestRemark = error.message || '快速测试失败';
    record.quickTestAt = Date.now();
    record.quickTestResponseTime = '';
    record.quickTestResponseContent = '';
    persistRecords();
    message.error(`快速测试失败：${error.message || '未知错误'}`);
  } finally {
    record.quickTestLoading = false;
  }
}

async function resolveQuickTestModel(record) {
  const fromRecord = pickPreferredModel(record.modelsList);
  if (fromRecord) return fromRecord;
  const modelResponse = await fetchModelList(record.siteUrl, record.apiKey);
  const rawCandidates = modelResponse?.data || modelResponse?.models || [];
  const normalizedCandidates = normalizeModels(rawCandidates);
  if (normalizedCandidates.length === 0) throw new Error('没有获取到可测试模型');
  const preferred = pickPreferredModel(normalizedCandidates);
  if (!preferred) throw new Error('没有找到适合快速对话测试的模型');
  record.modelsList = normalizedCandidates;
  record.modelsText = normalizedCandidates.join(', ');
  persistRecords();
  return preferred;
}

async function executeQuickTest({ apiKey, siteUrl, model }) {
  let timeoutMs = DEFAULT_TEST_TIMEOUT_MS;
  if (/^o1-|^o3-/i.test(model)) timeoutMs *= 3;
  const startedAt = Date.now();
  const response = await apiFetch('/api/check-key', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url: normalizeSiteUrl(siteUrl), key: apiKey, model, messages: [{ role: 'user', content: 'hello' }], timeoutMs, _isFirst: false }),
  });
  if (!response.ok) {
    const rawError = await safeReadResponsePayload(response);
    throw new Error(extractReadableError(rawError, response.status));
  }

  let data = await response.json();
  if (data?.htmlSnippet) {
    const snippet = String(data.htmlSnippet).replace(/^data:\s*/, '').trim();
    if (snippet.startsWith('{') || snippet.startsWith('[')) {
      try { data = JSON.parse(snippet); } catch (error) { console.warn('Failed to parse htmlSnippet JSON payload', error); }
    }
  }

  const returnedModel = String(data?.model || 'unknown');
  const messageObj = data?.choices?.[0]?.message;
  const hasContent = Boolean(messageObj?.content || messageObj?.reasoning_content || messageObj?.thinking);
  const responseContent = extractQuickTestResponseContent(messageObj);
  const responseTime = ((Date.now() - startedAt) / 1000).toFixed(2);
  if (returnedModel.toLowerCase().includes(model.toLowerCase()) || returnedModel === 'unknown') {
    if (hasContent) {
      return {
        status: returnedModel === 'unknown' ? 'warning' : 'success',
        label: returnedModel === 'unknown' ? '可用待确认' : '可用',
        remark: returnedModel === 'unknown' ? '接口有正常响应，但未返回模型标识' : '接口返回了有效对话内容',
        responseTime,
        responseContent,
      };
    }
    return { status: 'warning', label: '结构异常', remark: '接口响应成功，但未检测到有效消息内容', responseTime, responseContent };
  }
  return { status: 'warning', label: '模型映射', remark: `平台返回模型 ${returnedModel}，请求模型为 ${model}`, responseTime, responseContent };
}

async function exportAllValidKeysPackage() {
  const validRecords = tableData.value
    .filter(record => record.status === 1 && record.siteUrl && record.apiKey)
    .map(({ quickTestLoading, ...record }) => ({
      ...record,
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: record.modelsText || '未提供模型信息',
    }));

  if (validRecords.length === 0) {
    message.warning('当前没有状态正常的 Key');
    return;
  }

  try {
    const payload = {
      format: 'api-check-key-export-v1',
      compressed: 'gzip',
      exportedAt: Date.now(),
      records: validRecords,
    };
    const compressed = await compressClipboardPackage(JSON.stringify(payload));
    const output = `sk://${compressed}`;
    await navigator.clipboard.writeText(output);
    message.success(`已导出 ${validRecords.length} 条有效 Key 到剪贴板`);
  } catch (error) {
    console.error(error);
    message.error(`导出失败：${error.message || '未知错误'}`);
  }
}

async function importFromClipboardPackage() {
  try {
    const text = String(await navigator.clipboard.readText()).trim();
    if (!text) {
      throw new Error('剪贴板为空');
    }

    if (!text.startsWith('sk://')) {
      throw new Error('剪贴板内容不是 sk:// 导入包');
    }

    const payloadText = await decompressClipboardPackage(text.slice('sk://'.length));
    const payload = JSON.parse(payloadText);
    const importedRecords = Array.isArray(payload?.records) ? payload.records : [];
    if (importedRecords.length === 0) {
      throw new Error('导入包中没有记录');
    }

    const merged = new Map(tableData.value.map(record => [record.rowKey, { ...record }]));
    importedRecords.forEach(rawRecord => {
      const record = {
        ...rawRecord,
        sourceType: rawRecord.sourceType || 'auto',
        siteName: String(rawRecord.siteName || '未命名站点').trim() || '未命名站点',
        tokenName: String(rawRecord.tokenName || '').trim(),
        siteUrl: normalizeSiteUrl(rawRecord.siteUrl),
        apiKey: normalizeApiKey(rawRecord.apiKey),
        modelsList: normalizeModels(rawRecord.modelsList || rawRecord.modelsText),
        modelsText: normalizeModels(rawRecord.modelsList || rawRecord.modelsText).join(', ') || '未提供模型信息',
        status: Number(rawRecord.status || 1),
        quickTestStatus: rawRecord.quickTestStatus || '',
        quickTestLabel: rawRecord.quickTestLabel || '',
        quickTestModel: rawRecord.quickTestModel || '',
        quickTestRemark: rawRecord.quickTestRemark || '',
        quickTestAt: rawRecord.quickTestAt || null,
        quickTestResponseTime: rawRecord.quickTestResponseTime || '',
        quickTestResponseContent: rawRecord.quickTestResponseContent || '',
        quickTestLoading: false,
      };
      record.rowKey = rawRecord.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey));
      if (record.siteUrl && record.apiKey) {
        merged.set(record.rowKey, record);
      }
    });

    tableData.value = Array.from(merged.values());
    syncMeta.value = {
      lastBatchSyncAt: Date.now(),
      lastBatchSyncCount: importedRecords.length,
      lastBatchFailedCount: importedRecords.filter(record => Number(record?.status || 1) !== 1).length,
    };
    persistRecords();
    persistMeta();
    message.success(`已从剪贴板导入 ${importedRecords.length} 条记录`);
  } catch (error) {
    console.error(error);
    message.error(`导入失败：${error.message || '未知错误'}`);
  }
}

async function copySingleImportCommand(record) {
  try {
    const normalizedRecord = {
      ...record,
      sourceType: record.sourceType || 'auto',
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
      siteName: String(record.siteName || '未命名站点').trim() || '未命名站点',
      tokenName: String(record.tokenName || '').trim(),
      siteUrl: normalizeSiteUrl(record.siteUrl),
      apiKey: normalizeApiKey(record.apiKey),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: normalizeModels(record.modelsList || record.modelsText).join(', ') || '未提供模型信息',
      quickTestResponseContent: record.quickTestResponseContent || '',
    };
    const payload = {
      format: 'api-check-key-export-v1',
      compressed: 'gzip',
      exportedAt: Date.now(),
      records: [normalizedRecord],
    };
    const compressed = await compressClipboardPackage(JSON.stringify(payload));
    await navigator.clipboard.writeText(`sk://${compressed}`);
    message.success('已复制单条 sk:// 导入命令；相同记录会覆盖，不会重复追加');
  } catch (error) {
    console.error(error);
    message.error(`复制导入命令失败：${error.message || '未知错误'}`);
  }
}

function openManualRecordModal(record = null) {
  manualRecordEditing.value = Boolean(record);
  overwriteManualRecordDraft(createManualRecordDraft(record));
  manualRecordModalOpen.value = true;
}

async function submitManualRecord() {
  const siteName = String(manualRecordDraft.siteName || '').trim();
  const siteUrl = normalizeSiteUrl(manualRecordDraft.siteUrl);
  const apiKey = normalizeApiKey(manualRecordDraft.apiKey);
  manualRecordDraft.modelsText = normalizeModels(manualRecordDraft.modelsValue).join(', ');
  if (!siteName || !siteUrl || !apiKey) {
    message.warning('请至少填写网站名称、接口地址和 API Key');
    return;
  }

  manualRecordSaving.value = true;
  try {
    const existingRecord = manualRecordDraft.rowKey
      ? tableData.value.find(item => item.rowKey === manualRecordDraft.rowKey)
      : null;
    const nextRecord = createRecordFromDraft(manualRecordDraft, existingRecord);
    tableData.value = [
      ...tableData.value.filter(item => item.rowKey !== manualRecordDraft.rowKey),
      nextRecord,
    ];
    persistRecords();
    manualRecordModalOpen.value = false;
    message.success(manualRecordEditing.value ? '记录已更新' : '手工记录已添加');
  } finally {
    manualRecordSaving.value = false;
  }
}

function deleteRecord(record) {
  tableData.value = tableData.value.filter(item => item.rowKey !== record.rowKey);
  persistRecords();
  message.success('记录已删除');
}

async function handleManualModelDropdownVisibleChange(open) {
  if (!open) return;
  await loadManualModelOptions();
}

function handleManualModelSelectionChange(values) {
  const normalizedValues = normalizeModels(values);
  manualRecordDraft.modelsValue = normalizedValues;
  manualRecordDraft.modelsText = normalizedValues.join(', ');
  mergeManualModelOptions(normalizedValues);
}

async function loadManualModelOptions(force = false) {
  const siteUrl = normalizeSiteUrl(manualRecordDraft.siteUrl);
  const apiKey = normalizeApiKey(manualRecordDraft.apiKey);
  if (!siteUrl || !apiKey) {
    message.warning('请先填写接口地址和 API Key，再获取模型列表');
    return;
  }

  const currentFetchKey = `${siteUrl}::${apiKey}`;
  if (!force && manualModelFetchKey.value === currentFetchKey && manualModelOptions.value.length > 0) {
    return;
  }

  manualModelLoading.value = true;
  try {
    const modelResponse = await fetchModelList(siteUrl, apiKey);
    const rawCandidates = modelResponse?.data || modelResponse?.models || [];
    const normalizedCandidates = normalizeModels(rawCandidates);
    if (!normalizedCandidates.length) {
      throw new Error('没有获取到可用模型');
    }
    manualModelFetchKey.value = currentFetchKey;
    mergeManualModelOptions(normalizedCandidates);
  } catch (error) {
    console.error(error);
    message.error(`获取模型列表失败：${error.message || '未知错误'}`);
  } finally {
    manualModelLoading.value = false;
  }
}

function mergeManualModelOptions(values) {
  const merged = normalizeModels([
    ...manualModelOptions.value.map(option => option.value),
    ...values,
  ]);
  manualModelOptions.value = merged.map(value => ({ label: value, value }));
}

async function compressClipboardPackage(text) {
  if (typeof CompressionStream !== 'function') {
    throw new Error('当前环境不支持压缩导出');
  }
  const source = new Blob([new TextEncoder().encode(String(text || ''))]).stream();
  const compressed = source.pipeThrough(new CompressionStream('gzip'));
  const arrayBuffer = await new Response(compressed).arrayBuffer();
  return bytesToBase64Url(new Uint8Array(arrayBuffer));
}

async function decompressClipboardPackage(text) {
  if (typeof DecompressionStream !== 'function') {
    throw new Error('当前环境不支持压缩导入');
  }
  const bytes = base64UrlToBytes(text);
  const decompressed = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('gzip'));
  return await new Response(decompressed).text();
}

function bytesToBase64Url(bytes) {
  let binary = '';
  const chunkSize = 0x8000;
  for (let index = 0; index < bytes.length; index += chunkSize) {
    const chunk = bytes.subarray(index, index + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function base64UrlToBytes(value) {
  const normalized = String(value || '').replace(/-/g, '+').replace(/_/g, '/');
  const padding = normalized.length % 4 === 0 ? '' : '='.repeat(4 - (normalized.length % 4));
  const binary = atob(`${normalized}${padding}`);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

function exportCsv() {
  if (!displayedRows.value.length) return;
  let csv = '\uFEFF网站,Token名称,API Key,接口地址,模型候选,状态,最近同步,最近快测,快测结果\n';
  displayedRows.value.forEach(record => {
    csv += [`"${record.siteName}"`, `"${record.tokenName || ''}"`, `"${record.apiKey}"`, `"${record.siteUrl}"`, `"${record.modelsText || ''}"`, `"${record.status === 1 ? '正常' : '禁用/异常'}"`, `"${formatDateTime(record.updatedAt)}"`, `"${formatDateTime(record.quickTestAt)}"`, `"${record.quickTestRemark || ''}"`].join(',');
    csv += '\n';
  });
  const anchor = document.createElement('a');
  anchor.href = `data:text/csv;charset=utf-8,${encodeURIComponent(csv)}`;
  anchor.download = `key-management-${Date.now()}.csv`;
  anchor.click();
}

function clearLocalRecords() {
  tableData.value = [];
  allResults.value = [];
  syncMeta.value = { lastBatchSyncAt: null, lastBatchSyncCount: 0, lastBatchFailedCount: 0 };
  localStorage.removeItem(STORAGE_KEY);
  localStorage.removeItem(MANUAL_STORAGE_KEY);
  localStorage.removeItem(META_STORAGE_KEY);
  message.success('本地密钥库已清空');
}

function launchCherryStudio(record) {
  if (!record.apiKey || !record.siteUrl) {
    message.warning('配置不完整，无法导出');
    return;
  }
  const payload = { id: `key-${record.rowKey}`, baseUrl: normalizeSiteUrl(record.siteUrl), apiKey: record.apiKey, name: `${record.siteName}${record.quickTestModel ? ` (${record.quickTestModel})` : ''}` };
  try {
    const encoded = btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(payload))));
    window.open(`cherrystudio://providers/api-keys?v=1&data=${encoded}`, '_blank');
    message.success('正在尝试唤起 Cherry Studio');
  } catch (error) {
    console.error(error);
    message.error(`导出 Cherry Studio 失败：${error.message || '未知错误'}`);
  }
}

function launchCCSwitch(record, targetApp = 'claude') {
  if (!record.apiKey || !record.siteUrl) {
    message.warning('配置不完整，无法导出');
    return;
  }
  const params = new URLSearchParams();
  params.set('resource', 'provider');
  params.set('app', targetApp);
  params.set('name', `${record.siteName}${record.quickTestModel ? ` - ${record.quickTestModel}` : ''}`);
  params.set('homepage', normalizeSiteUrl(record.siteUrl));
  params.set('endpoint', normalizeSiteUrl(record.siteUrl));
  params.set('apiKey', record.apiKey);
  if (record.quickTestModel) params.set('model', record.quickTestModel);
  window.open(`ccswitch://v1/import?${params.toString()}`, '_blank');
  message.success(`正在尝试唤起 CC Switch (${targetApp})`);
}

function openDesktopConfigWizard(record) {
  if (!isDesktopConfigBridgeAvailable()) {
    message.warning('专属一键配置仅支持桌面版 EXE 运行环境');
    return;
  }
  desktopConfigTargetRecord.value = record;
  desktopConfigPreview.value = { appGroups: [], writes: [], errors: [] };
  overwriteDesktopConfigDraft(createDesktopConfigDraft(record));
  desktopConfigModalOpen.value = true;
}

async function generateDesktopConfigPreview() {
  if (!desktopConfigDraft.selectedApps.length) {
    message.warning('请至少选择一个目标应用');
    return;
  }
  desktopConfigLoading.value = true;
  try {
    const snapshot = await readManagedAppConfigFiles(desktopConfigDraft.selectedApps);
    const preview = buildDesktopConfigPreview(desktopConfigDraft, snapshot);
    desktopConfigPreview.value = preview;
    if (!preview.appGroups.length && preview.errors.length) throw new Error(preview.errors.join('；'));
    desktopConfigDiffOpen.value = true;
    if (preview.errors.length) message.warning(`部分应用预览生成失败：${preview.errors.join('；')}`);
    else message.success(`已生成 ${preview.writes.length} 个配置文件的变更预览`);
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
    desktopConfigModalOpen.value = false;
    message.success(`已写入 ${appliedCount} 个本地配置文件，并自动创建备份`);
  } catch (error) {
    console.error(error);
    message.error(`写入本地配置失败：${error.message || '未知错误'}`);
  } finally {
    desktopConfigLoading.value = false;
  }
}

function overwriteDesktopConfigDraft(nextDraft) {
  Object.keys(desktopConfigDraft).forEach(key => delete desktopConfigDraft[key]);
  Object.assign(desktopConfigDraft, nextDraft);
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
}

function overwriteManualRecordDraft(nextDraft) {
  Object.keys(manualRecordDraft).forEach(key => delete manualRecordDraft[key]);
  Object.assign(manualRecordDraft, nextDraft);
  manualModelFetchKey.value = '';
  mergeManualModelOptions(nextDraft.modelsValue || []);
}

function getQuickTestTooltip(record) {
  if (!record.quickTestStatus) return '尚未执行快速对话测试';
  return [
    `结果：${record.quickTestLabel || record.quickTestStatus}`,
    record.quickTestModel ? `模型：${record.quickTestModel}` : '',
    record.quickTestResponseTime ? `耗时：${record.quickTestResponseTime}s` : '',
    record.quickTestRemark ? `说明：${record.quickTestRemark}` : '',
    record.quickTestResponseContent ? `内容：${record.quickTestResponseContent}` : '',
    record.quickTestAt ? `时间：${formatDateTime(record.quickTestAt)}` : '',
  ].filter(Boolean).join('\n');
}

function getQuickTestColor(status) {
  if (status === 'success') return 'green';
  if (status === 'warning') return 'orange';
  if (status === 'error') return 'red';
  return 'default';
}

function extractQuickTestResponseContent(messageObj) {
  const candidates = [
    normalizeQuickTestContent(messageObj?.content),
    normalizeQuickTestContent(messageObj?.reasoning_content),
    normalizeQuickTestContent(messageObj?.thinking),
  ].filter(Boolean);
  return candidates[0] || '';
}

function normalizeQuickTestContent(value) {
  if (!value) return '';
  let text = '';

  if (typeof value === 'string') {
    text = value;
  } else if (Array.isArray(value)) {
    text = value
      .map(item => {
        if (typeof item === 'string') return item;
        if (typeof item?.text === 'string') return item.text;
        if (typeof item?.content === 'string') return item.content;
        if (typeof item?.value === 'string') return item.value;
        return '';
      })
      .filter(Boolean)
      .join('\n');
  } else if (typeof value === 'object') {
    text = String(value?.text || value?.content || value?.value || '');
  }

  text = text.replace(/\s+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
  if (text.length > 500) {
    return `${text.slice(0, 500)}...`;
  }
  return text;
}

const buildRowKey = (siteUrl, apiKey) => `${normalizeSiteUrl(siteUrl)}::${String(apiKey || '').trim()}`;
const buildManualRowKey = () => `manual::${Date.now()}::${Math.random().toString(36).slice(2, 8)}`;

function createRecordFromDraft(draft, existingRecord = null) {
  const modelsList = normalizeModels(draft.modelsText);
  const now = Date.now();
  const sourceType = existingRecord?.sourceType || draft.sourceType || 'manual';
  const isManual = sourceType === 'manual';
  return {
    ...existingRecord,
    rowKey: isManual ? (existingRecord?.rowKey || draft.rowKey || buildManualRowKey()) : buildRowKey(draft.siteUrl, draft.apiKey),
    sourceType,
    siteName: String(draft.siteName || '').trim() || '未命名站点',
    tokenName: String(draft.tokenName || '').trim(),
    siteUrl: normalizeSiteUrl(draft.siteUrl),
    apiKey: normalizeApiKey(draft.apiKey),
    modelsList,
    modelsText: modelsList.length ? modelsList.join(', ') : '未提供模型信息',
    status: Number(draft.status || 1),
    createdAt: existingRecord?.createdAt || now,
    updatedAt: now,
    quickTestStatus: existingRecord?.quickTestStatus || '',
    quickTestLabel: existingRecord?.quickTestLabel || '',
    quickTestModel: existingRecord?.quickTestModel || '',
    quickTestRemark: existingRecord?.quickTestRemark || '',
    quickTestAt: existingRecord?.quickTestAt || null,
    quickTestResponseTime: existingRecord?.quickTestResponseTime || '',
    quickTestResponseContent: existingRecord?.quickTestResponseContent || '',
    quickTestLoading: false,
  };
}
function normalizeApiKey(rawKey) {
  let apiKey = String(rawKey || '').trim();
  if (!apiKey) return '';
  if (!apiKey.startsWith('sk-')) apiKey = `sk-${apiKey}`;
  return apiKey;
}

function normalizeSiteUrl(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function normalizeModels(rawModels) {
  const list = Array.isArray(rawModels) ? rawModels : String(rawModels || '').split(/[\n,，\s]+/).map(item => item.trim());
  return Array.from(new Set(list.map(item => typeof item === 'string' ? item : item?.id || item?.model || '').map(item => String(item || '').trim()).filter(Boolean)));
}

function resolveSiteUrl(result) {
  const explicitSiteUrl = normalizeSiteUrl(result?.site_url);
  if (explicitSiteUrl) return explicitSiteUrl;
  const apiAddress = normalizeSiteUrl(result?.api_url || result?.api_address);
  if (apiAddress) return apiAddress;
  const rawApiKey = String(result?.api_key || '').trim();
  return rawApiKey.startsWith('http://') || rawApiKey.startsWith('https://') ? normalizeSiteUrl(rawApiKey) : '';
}

function extractAccountsFromBackup(parsed) {
  if (Array.isArray(parsed?.accounts?.accounts)) return parsed.accounts.accounts;
  if (Array.isArray(parsed?.accounts)) return parsed.accounts;
  if (Array.isArray(parsed)) return parsed;
  return [];
}

function pickPreferredModel(candidates) {
  const chatCandidates = normalizeModels(candidates).filter(isLikelyChatModel);
  if (!chatCandidates.length) return '';
  const preferredPatterns = [/gpt-5/i, /gpt-4\.1/i, /gpt-4o/i, /^o3/i, /^o1/i, /claude/i, /gemini/i, /deepseek/i, /qwen/i, /grok/i, /kimi/i, /chat/i];
  return chatCandidates.find(model => preferredPatterns.some(pattern => pattern.test(model))) || chatCandidates[0];
}

function isLikelyChatModel(model) {
  return !/(embedding|tts|whisper|speech|audio|image|video|vision|flux|midjourney|mj|rerank|bge|stability|playground|suno|music|ocr|moderation|asr)/i.test(String(model || ''));
}

async function safeReadJson(response) {
  try {
    return await response.json();
  } catch (error) {
    console.warn('Failed to read JSON response', error);
    return null;
  }
}

async function safeReadResponsePayload(response) {
  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) return safeReadJson(response);
  const text = await response.text();
  const htmlTitle = text.match(/<title>(.*?)<\/title>/i)?.[1];
  return { message: htmlTitle || text.slice(0, 300) };
}

function extractReadableError(payload, statusCode) {
  if (!payload) return `HTTP ${statusCode}`;
  return payload?.error?.message || payload?.message || `HTTP ${statusCode}`;
}

function formatDateTime(timestamp) {
  if (!timestamp) return '未同步';
  try {
    return new Date(timestamp).toLocaleString();
  } catch (error) {
    console.warn('Failed to format timestamp', error);
    return '时间异常';
  }
}

function loadStoredRecords() {
  try {
    const autoRaw = localStorage.getItem(STORAGE_KEY);
    const manualRaw = localStorage.getItem(MANUAL_STORAGE_KEY);
    const autoRecords = JSON.parse(autoRaw || '[]');
    const manualRecords = JSON.parse(manualRaw || '[]');
    const parsedRecords = [
      ...(Array.isArray(autoRecords) ? autoRecords : []),
      ...(Array.isArray(manualRecords) ? manualRecords : []),
    ];
    return parsedRecords.map(record => ({
      ...record,
      sourceType: record.sourceType || 'auto',
      siteName: record.siteName || '未命名站点',
      tokenName: record.tokenName || '',
      siteUrl: normalizeSiteUrl(record.siteUrl),
      apiKey: String(record.apiKey || '').trim(),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: record.modelsText || '未提供模型信息',
      quickTestStatus: record.quickTestStatus || '',
      quickTestLabel: record.quickTestLabel || '',
      quickTestModel: record.quickTestModel || '',
      quickTestRemark: record.quickTestRemark || '',
      quickTestAt: record.quickTestAt || null,
      quickTestResponseTime: record.quickTestResponseTime || '',
      quickTestResponseContent: record.quickTestResponseContent || '',
      quickTestLoading: false,
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
    })).filter(record => record.siteUrl && record.apiKey);

    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed = JSON.parse(raw || '[]');
    if (!Array.isArray(parsed)) return [];
    return parsed.map(record => ({
      ...record,
      siteName: record.siteName || '未命名站点',
      tokenName: record.tokenName || '',
      siteUrl: normalizeSiteUrl(record.siteUrl),
      apiKey: String(record.apiKey || '').trim(),
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      modelsText: record.modelsText || '未提供模型信息',
      quickTestStatus: record.quickTestStatus || '',
      quickTestLabel: record.quickTestLabel || '',
      quickTestModel: record.quickTestModel || '',
      quickTestRemark: record.quickTestRemark || '',
      quickTestAt: record.quickTestAt || null,
      quickTestResponseTime: record.quickTestResponseTime || '',
      quickTestResponseContent: record.quickTestResponseContent || '',
      quickTestLoading: false,
      rowKey: record.rowKey || buildRowKey(record.siteUrl, record.apiKey),
    })).filter(record => record.siteUrl && record.apiKey);
  } catch (error) {
    console.error(error);
    return [];
  }
}

function loadStoredMeta() {
  try {
    const raw = localStorage.getItem(META_STORAGE_KEY);
    const parsed = JSON.parse(raw || '{}');
    return {
      lastBatchSyncAt: parsed?.lastBatchSyncAt || null,
      lastBatchSyncCount: parsed?.lastBatchSyncCount || 0,
      lastBatchFailedCount: parsed?.lastBatchFailedCount || 0,
    };
  } catch (error) {
    console.error(error);
    return { lastBatchSyncAt: null, lastBatchSyncCount: 0, lastBatchFailedCount: 0 };
  }
}

function persistRecords() {
  const autoRecords = [];
  const manualRecords = [];
  tableData.value.forEach(({ quickTestLoading, ...record }) => {
    const normalizedRecord = {
      ...record,
      sourceType: record.sourceType || 'auto',
      modelsList: normalizeModels(record.modelsList || record.modelsText),
      quickTestResponseContent: record.quickTestResponseContent || '',
      rowKey: record.rowKey || (record.sourceType === 'manual' ? buildManualRowKey() : buildRowKey(record.siteUrl, record.apiKey)),
    };
    if (normalizedRecord.sourceType === 'manual') manualRecords.push(normalizedRecord);
    else autoRecords.push(normalizedRecord);
  });
  localStorage.setItem(STORAGE_KEY, JSON.stringify(autoRecords));
  localStorage.setItem(MANUAL_STORAGE_KEY, JSON.stringify(manualRecords));
  return;

  const serializable = tableData.value.map(({ quickTestLoading, ...record }) => ({
    ...record,
    modelsList: normalizeModels(record.modelsList || record.modelsText),
    rowKey: record.rowKey || buildRowKey(record.siteUrl, record.apiKey),
  }));
  localStorage.setItem(STORAGE_KEY, JSON.stringify(serializable));
}

function persistMeta() {
  localStorage.setItem(META_STORAGE_KEY, JSON.stringify(syncMeta.value));
}
</script>

<style scoped>
.key-management{width:100%;padding:20px;display:flex;flex-direction:column;gap:20px}
.sync-card,.inventory-card{width:100%}
.sync-toolbar,.sync-meta,.quick-test-cell,.site-cell,.time-cell{display:flex;gap:12px;flex-wrap:wrap}
.sync-toolbar{align-items:center;justify-content:space-between}
.sync-meta,.site-cell,.time-cell,.quick-test-cell{flex-direction:column;gap:4px}
.site-heading{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.sync-meta,.subtle-text{color:#64748b;font-size:12px}
.sync-loading{margin-top:16px;display:flex;align-items:center;gap:10px}
.sync-alert{margin-top:16px}
.cell-copy-text{max-width:260px;display:inline-block}
.models-text{display:block;width:100%;max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.import-export-menu{min-width:170px;display:flex;flex-direction:column;gap:8px}
.inline-export-actions{display:flex;align-items:center;gap:10px}
.export-icon-button{width:34px;height:34px;border:0;border-radius:12px;display:inline-flex;align-items:center;justify-content:center;cursor:pointer;transition:transform .18s ease, box-shadow .18s ease, filter .18s ease;background:linear-gradient(135deg,#f8fafc,#e2e8f0);box-shadow:inset 0 0 0 1px rgba(148,163,184,.28)}
.export-icon-button:hover{transform:translateY(-1px) scale(1.06);filter:saturate(1.08)}
.export-icon-glyph{font-size:16px;line-height:1}
.export-icon-image{width:22px;height:22px;display:block;object-fit:contain}
.export-icon-image-switch{width:20px;height:20px;border-radius:6px}
.export-copy{color:#0f172a}
.export-cherry{background:linear-gradient(135deg,#fff1f2,#ffe4e6);color:#be123c}
.export-switch{background:linear-gradient(135deg,#fff7ed,#ffedd5);color:#1d4ed8}
.export-desktop{background:linear-gradient(135deg,#eff6ff,#dbeafe);color:#fff;box-shadow:0 10px 24px rgba(96,165,250,.22)}
.switch-app-menu{min-width:132px;display:flex;flex-direction:column;gap:6px}
.switch-app-item{border:0;border-radius:10px;background:#f8fafc;color:#0f172a;padding:8px 10px;text-align:left;cursor:pointer;transition:background .18s ease,color .18s ease}
.switch-app-item:hover{background:#e0ecff;color:#1d4ed8}
.quick-test-tag{width:fit-content}
.desktop-config-modal{display:flex;flex-direction:column;gap:16px}
.desktop-config-alert{margin-bottom:4px}
.desktop-config-layout{display:grid;grid-template-columns:280px minmax(0,1fr);gap:20px;align-items:start}
.desktop-app-panel,.desktop-form-panel{border-radius:24px;background:linear-gradient(180deg,#f8fafc,#eef2ff);padding:18px}
.desktop-panel-title{font-size:16px;font-weight:700;color:#0f172a}
.desktop-panel-hint{margin-top:6px;color:#64748b;font-size:12px;line-height:1.5}
.desktop-app-grid{margin-top:16px;display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}
.desktop-app-card{border:0;border-radius:22px;padding:16px 12px;background:#fff;color:#0f172a;box-shadow:0 10px 24px rgba(15,23,42,.08),inset 0 0 0 1px rgba(148,163,184,.16);display:flex;flex-direction:column;align-items:center;gap:10px;cursor:pointer;transition:transform .18s ease,box-shadow .18s ease,background .18s ease}
.desktop-app-card:hover{transform:translateY(-2px)}
.desktop-app-card-active{box-shadow:0 14px 30px rgba(37,99,235,.16),inset 0 0 0 2px rgba(37,99,235,.45);background:linear-gradient(180deg,#ffffff,#eff6ff)}
.desktop-app-logo{width:58px;height:58px;border-radius:18px;display:inline-flex;align-items:center;justify-content:center;background:#f8fafc;padding:10px}
.desktop-app-logo-image{width:100%;height:100%;display:block;object-fit:contain}
.desktop-app-name{font-size:13px;font-weight:600}
.desktop-app-claude .desktop-app-logo{background:linear-gradient(135deg,#fff7ed,#ffedd5)}
.desktop-app-codex .desktop-app-logo{background:linear-gradient(135deg,#ffffff,#f3f4f6)}
.desktop-app-gemini .desktop-app-logo{background:linear-gradient(135deg,#ffffff,#eef4ff)}
.desktop-app-opencode .desktop-app-logo{background:linear-gradient(135deg,#eef2ff,#dbeafe)}
.desktop-app-openclaw .desktop-app-logo{background:linear-gradient(135deg,#fff1f2,#ffe4e6)}
.config-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:0 16px}
@media (max-width:900px){.key-management{padding:16px}.desktop-config-layout{grid-template-columns:1fr}.desktop-app-grid{grid-template-columns:repeat(4,minmax(0,1fr));overflow:auto}.config-grid{grid-template-columns:1fr}}
</style>
