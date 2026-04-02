<template>
  <ConfigProvider :theme="configProviderTheme">
    <div class="wrapper batch-wrapper">
      <div style="width: 100%;">
        <div class="page-content" style="width: 100%">
          <div class="container" style="max-width: 100% !important; margin: 0 !important; padding: 20px !important;">
            <!-- Header section, similar to Check.vue for consistency -->
            <div class="header">
              <button
                id="themeToggle"
                :aria-label="t('SWITCH_THEME') || '切换主题'"
                @click="handleToggleTheme"
              >
                <!-- Sun/Moon Icon SVG -->
                <svg
                  v-if="!isDarkMode"
                  xmlns="http://www.w3.org/2000/svg"
                  width="24"
                  height="24"
                  viewBox="0 0 24 24"
                  fill="transparent"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <circle cx="12" cy="12" r="4"></circle>
                  <path d="M12 2v2"></path>
                  <path d="M12 20v2"></path>
                  <path d="m4.93 4.93 1.41 1.41"></path>
                  <path d="m17.66 17.66 1.41 1.41"></path>
                  <path d="M2 12h2"></path>
                  <path d="M20 12h2"></path>
                  <path d="m6.34 17.66-1.41 1.41"></path>
                  <path d="m19.07 4.93-1.41 1.41"></path>
                </svg>
                <svg
                  v-else
                  xmlns="http://www.w3.org/2000/svg"
                  width="24"
                  height="24"
                  viewBox="0 0 24 24"
                  fill="transparent"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                >
                  <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"></path>
                </svg>
              </button>

              <div class="right-icons">
                <a-tooltip :title="'实验性功能'" placement="bottom">
                  <a @click="showExperimentalFeatures = true" class="icon-button">
                    <ExperimentOutlined style="cursor: pointer" />
                  </a>
                </a-tooltip>
                <a-tooltip :title="'设置'" placement="bottom">
                  <a @click="openSettingsModal" class="icon-button">
                    <SettingOutlined style="cursor: pointer" />
                  </a>
                </a-tooltip>
                <a-tooltip :title="'单次检测'" placement="bottom">
                  <a @click="$router.push('/single')" class="icon-button">
                    <CheckCircleOutlined style="cursor: pointer" />
                  </a>
                </a-tooltip>
                <a-tooltip :title="'密钥提取'" placement="bottom">
                  <a @click="$router.push('/keys')" class="icon-button">
                    <KeyOutlined style="cursor: pointer" />
                  </a>
                </a-tooltip>
                <a-tooltip title="GitHub" placement="bottom">
                  <div @click="openGitHub()" class="icon-button">
                    <GithubOutlined style="cursor: pointer" />
                  </div>
                </a-tooltip>
              </div>
            </div>

            <h1 style="text-align: center; margin-bottom: 20px;">
              批量并发检测
            </h1>
            <h3 style="text-align: center; color: #666; margin-bottom: 30px;">
              （支持导入 accounts-backup JSON 进行批量筛查）
            </h3>

            <!-- 步骤 1：上传备份文件 -->
            <div v-show="step === 1" class="step-container">
              <a-upload-dragger
                name="file"
                :multiple="false"
                :before-upload="beforeUpload"
                :show-upload-list="false"
                accept=".json"
              >
                <p class="ant-upload-drag-icon">
                  <InboxOutlined />
                </p>
                <p class="ant-upload-text">点击或将 accounts-backup.json 拖拽到此处</p>
                <p class="ant-upload-hint">解析后将自动并发获取每个网站的模型列表</p>
              </a-upload-dragger>
              
              <div v-if="hasHistory" style="margin-top: 20px; text-align: center;">
                <a-button @click="loadHistory" type="dashed">
                  <HistoryOutlined /> 查看上一次检测结果
                </a-button>
              </div>
            </div>

            <!-- 加载状态 -->
            <div v-show="isLoadingModels && step === -1" class="step-container loading-container">
              <a-spin size="large" />
              <p style="margin-top: 20px;">正在并发获取各大站点的模型列表，请稍候... ({{ loadedSitesCount }} / {{ totalAccountsCount }})</p>
            </div>

            <!-- 步骤 2：树形选择器选择想要检查的模型 -->
            <div v-show="step === 2" class="step-container">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                <h3 style="margin: 0;">请勾选需要测试的网站与模型</h3>
                <a-space>
                  <a-button @click="selectAllNodes" size="small">全部全选</a-button>
                  <a-button @click="unselectAllNodes" size="small">全部反选</a-button>
                  <a-button @click="selectChatModelsOnly" size="small">仅选主流聊天</a-button>
                </a-space>
              </div>

              <div
                v-if="isDiscoveringModels || browserSessionPolling.active"
                style="display:flex; align-items:center; gap:8px; margin-bottom: 12px; color:#1677ff;"
              >
                <a-spin size="small" />
                <span v-if="isDiscoveringModels">模型发现进行中（{{ loadedSitesCount }} / {{ totalAccountsCount }}）</span>
                <span v-if="isDiscoveringModels && browserSessionPolling.active">，</span>
                <span v-if="browserSessionPolling.active">受控浏览器后台检测中（{{ browserSessionPolling.round }} / {{ browserSessionPolling.totalRounds }}），剩余 {{ browserSessionPolling.pending }} 个站点</span>
              </div>

              <div class="tree-wrapper">
                <a-tree
                  v-model:checkedKeys="checkedKeys"
                  :tree-data="treeData"
                  checkable
                  defaultExpandAll
                  height="400"
                >
                  <template #title="node">
                    <div class="custom-tree-node-wrapper" style="display: flex; align-items: center;">
                      <span class="custom-tree-node">{{ node.title }}</span>
                      <span v-if="node.isModelDiscovering || node.isBrowserPending" class="tree-node-pending-hint">
                        <a-spin size="small" />
                        <span>{{ node.isModelDiscovering ? (node.modelDiscoveringHint || '模型检测中') : node.pendingHint }}</span>
                      </span>
                    </div>
                  </template>
                </a-tree>
              </div>

              <div class="settings-action-bar">
                <div class="batch-settings">
                  <span style="font-size: 14px; margin-right: 10px;">并发数：</span>
                  <a-input-number v-model:value="batchConcurrency" :min="1" :max="100" />
                  <span style="font-size: 14px; margin-left: 20px; margin-right: 10px;">超时(秒)：</span>
                  <a-input-number v-model:value="modelTimeout" :min="1" />
                </div>
                <div class="actions">
                  <a-button @click="resetStep1" style="margin-right: 10px;">重新导入</a-button>
                  <a-button type="primary" size="large" @click="startBatchCheck" :disabled="isDiscoveringModels">
                    <PlayCircleOutlined /> 开始检测
                  </a-button>
                </div>
              </div>
            </div>

            <!-- 步骤 3：显示检测结果 -->
            <div v-show="step === 3" class="step-container result-container">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
                <h3 style="margin: 0; cursor: pointer; user-select: none;" @click="isTableExpanded = !isTableExpanded">
                  <DownOutlined v-if="isTableExpanded" style="margin-right: 8px;" />
                  <RightOutlined v-else style="margin-right: 8px;" />
                  批量检测结果
                </h3>
                <a-space>
                  <a-dropdown-button @click="copyOrganizedResults" :disabled="testing || !testResults.length">
                    <CopyOutlined /> 整理有效配置
                    <template #overlay>
                      <a-menu>
                        <a-menu-item key="2" @click="copyAllConfigs">
                          <CopyOutlined /> 复制全表配置
                        </a-menu-item>
                      </a-menu>
                    </template>
                  </a-dropdown-button>
                  <a-button danger v-if="testing" @click="stopTesting">停止检测</a-button>
                  <a-button v-else @click="resetStep2">返回选择面板</a-button>
                </a-space>
              </div>
              <div
                v-if="browserSessionPolling.active"
                style="display:flex; align-items:center; gap:8px; margin-bottom: 10px; color:#1677ff;"
              >
                <a-spin size="small" />
                <span>受控浏览器后台检测中（{{ browserSessionPolling.round }} / {{ browserSessionPolling.totalRounds }}），剩余 {{ browserSessionPolling.pending }} 个站点...</span>
              </div>
              <div style="display:flex; justify-content:flex-end; margin-bottom: 10px;">
                <a-input-search
                  v-model:value="resultModelFilter"
                  placeholder="模型过滤：空格分隔关键字（如 gpt-5.2 codex）"
                  style="width: 420px"
                  allow-clear
                >
                  <template #prefix><SearchOutlined /></template>
                </a-input-search>
              </div>

              <div v-show="isTableExpanded">
                <a-progress :percent="testProgress" show-info style="margin-bottom: 15px" />

                <a-table
                  :columns="resultColumns"
                  :data-source="currentResultData"
                  :pagination="tablePagination"
                  @change="handleTableChange"
                  :row-class-name="record => record.id === highlightedTaskId ? 'highlighted-row' : ''"
                  size="small"
                  row-key="id"
                >
                  <!-- ... table slots ... -->
                  <template #bodyCell="{ column, record }">
                  <template v-if="column.dataIndex === 'siteName'">
                    <a-tooltip :title="record.quota" placement="top">
                      <a :href="record.siteUrl" target="_blank" @mouseenter="hoverQuota(record)">
                        {{ record.siteName }}
                      </a>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.dataIndex === 'payload'">
                    <a-tooltip placement="top">
                      <template #title>
                        <pre style="max-width:300px; white-space:pre-wrap; margin:0; font-size:12px;">{{ getPayloadJson(record) }}</pre>
                      </template>
                      <div style="cursor: pointer; user-select: none;" @dblclick="openPayloadEditor(record)">
                        {{ getMaskedKey(record.apiKey) }}
                      </div>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.dataIndex === 'status'">
                    <a-tooltip placement="topLeft">
                      <template #title>
                        <pre style="max-width:560px; max-height:420px; overflow:auto; white-space:pre-wrap; margin:0; font-size:12px;">{{ getStatusTooltip(record) }}</pre>
                      </template>
                      <a-tag :color="getStatusColor(record.status)" style="cursor: pointer;">
                        {{ record.statusText }}
                      </a-tag>
                    </a-tooltip>
                  </template>
                  <template v-else-if="column.dataIndex === 'remark'">
                    <a-tooltip :title="record.remark">
                      <span :style="{ color: record.status === 'error' ? '#ff4d4f' : 'inherit', fontWeight: record.status === 'error' ? 'bold' : 'normal' }">
                        {{ record.remark }}
                      </span>
                    </a-tooltip>
                  </template>
                </template>
              </a-table>
            </div>

              <!-- NEW ORGANIZED AREA -->
              <div v-if="testResults.length > 0" class="organized-section" style="margin-top: 25px; padding-top: 15px; border-top: 2px dashed var(--border-color);">
                <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 15px;">
                  <h3 style="margin: 0; cursor: pointer; user-select: none;" @click="isTreeExpanded = !isTreeExpanded">
                    <DownOutlined v-if="isTreeExpanded" style="margin-right: 8px;" />
                  <RightOutlined v-else style="margin-right: 8px;" />
                    <ShareAltOutlined /> 整理与概览
                  </h3>
                  <a-space>
                    <a-button 
                      size="small" 
                      type="link"
                      @click="toggleExpandAll"
                      style="margin-right: 2px"
                    >
                      <template v-if="expandedKeys.length > 0">
                        <MenuFoldOutlined /> 全部折叠
                      </template>
                      <template v-else>
                        <MenuUnfoldOutlined /> 全部展开
                      </template>
                    </a-button>
                    <a-button 
                      size="small" 
                      type="link"
                      :loading="isRefreshingBalances" 
                      @click="refreshAllBalances"
                      style="margin-right: 5px; color: #1677ff;"
                    >
                      <ReloadOutlined v-if="!isRefreshingBalances" /> 更新余额
                    </a-button>
                    <a-checkbox v-model:checked="filterOnlySuccess" style="margin-right: 15px;">
                      仅有效(过滤红色/失败)
                    </a-checkbox>
                    <a-input-search
                      v-model:value="searchQuery"
                      placeholder="关键字过滤 (空格分隔多词，如 gpt4 claude)"
                      style="width: 400px"
                      allow-clear
                    >
                      <template #prefix><SearchOutlined /></template>
                    </a-input-search>
                  </a-space>
                </div>

                <div v-show="isTreeExpanded" class="organized-tree-wrapper">
                  <div v-if="organizedTreeData.length === 0" style="text-align: center; padding: 40px; color: #999;">
                    没有匹配当前过滤条件的配置
                  </div>
                  <a-tree
                    v-else
                    :tree-data="organizedTreeData"
                    v-model:expanded-keys="expandedKeys"
                    @select="onTreeSelect"
                    class="result-summary-tree"
                    block-node
                  >
                    <template #title="node">
                       <div class="custom-tree-node-wrapper" style="display: flex; align-items: center;">
                         <span :class="['custom-tree-node', node.class]">{{ node.title }}</span>
                         <span v-if="node.isBrowserPending" class="tree-node-pending-hint">
                           <a-spin size="small" />
                           <span>{{ node.pendingHint }}</span>
                         </span>
                         
                         <!-- 仅在叶子节点（模型项）显示快捷拉起图标，空两格紧跟 -->
                         <div v-if="node.isLeaf" class="shortcut-actions" style="margin-left: 12px; display: flex; gap: 8px;">
                           <a-tooltip title="一键添加到 Cherry Studio">
                             <span class="app-icon cherry-icon" @click.stop="launchCherryStudio(node)">
                               🍒
                             </span>
                           </a-tooltip>
                           <a-tooltip title="一键添加到 CC-Switch">
                             <span class="app-icon switch-icon" @click.stop="launchCCSwitch(node)">
                               🔄
                             </span>
                           </a-tooltip>
                         </div>
                       </div>
                    </template>
                  </a-tree>
                </div>
              </div>
            </div>

            <!-- Payload Editor Modal -->
            <a-modal
              v-model:open="isEditorOpen"
              title="修改并重发请求 Payload"
              @ok="resendPayload"
              ok-text="重发"
              cancel-text="取消"
              destroy-on-close
              width="600px"
            >
              <div style="margin-bottom: 10px; color: #666;">
                在此处修改您想重新测试的 JSON Payload (请确保格式准确)。点击重新发送将直接用此 Payload 请求后端。
              </div>
              <a-textarea v-model:value="editingPayload" :rows="12" style="font-family: monospace;" />
            </a-modal>

            <!-- Experimental Features Modal -->
            <a-modal
              v-model:open="showExperimentalFeatures"
              title="实验性功能"
              :footer="null"
              @cancel="showExperimentalFeatures = false"
            >
              <div style="padding: 20px; text-align: center;">
                <SmileOutlined style="font-size: 48px; color: #1677ff; margin-bottom: 20px;" />
                <p>实验性功能正在开发中，敬请期待！</p>
              </div>
            </a-modal>

            <!-- Settings Modal (Aligned with Check.vue console style) -->
            <a-modal
              v-model:open="showAppSettingsModal"
              title="系统设置"
              :footer="null"
              :width="600"
              @cancel="closeSettingsModal"
              :centered="true"
              :destroyOnClose="true"
            >
              <a-tabs>
                <a-tab-pane key="1" tab="本地缓存">
                  <a-form @submit.prevent>
                    <a-row :gutter="16" type="flex" align="middle">
                      <a-col :span="16">
                        <a-form-item label="API URL">
                          <a-input v-model:value="settingsApiUrl" placeholder="请输入 API URL">
                            <template #prefix><UserOutlined /></template>
                          </a-input>
                        </a-form-item>
                        <a-form-item label="API Key">
                          <a-input-password v-model:value="settingsApiKey" placeholder="请输入 API Key">
                            <template #prefix><LockOutlined /></template>
                          </a-input-password>
                        </a-form-item>
                      </a-col>
                      <a-col :span="8">
                        <a-button type="primary" @click="saveToLocal" block style="height: 104px;">
                          保存到缓存
                        </a-button>
                      </a-col>
                    </a-row>
                  </a-form>
                  <a-list :data-source="localCacheList" bordered style="margin-top: 15px; max-height: 300px; overflow-y: auto;">
                    <template #renderItem="{ item }">
                      <a-list-item>
                        <div style="max-width: 70%;">
                          <div style="font-weight: bold;">{{ item.name }}</div>
                          <div style="font-size: 12px; color: #999; overflow: hidden; text-overflow: ellipsis;">{{ item.url }}</div>
                        </div>
                        <template #actions>
                          <a @click="loadLocalRecord(item.id)">填充</a>
                          <a-popconfirm title="确定删除此缓存？" @confirm="deleteLocalRecord(item.id)">
                            <a style="color: #ff4d4f;">删除</a>
                          </a-popconfirm>
                        </template>
                      </a-list-item>
                    </template>
                  </a-list>
                </a-tab-pane>
                <a-tab-pane key="2" tab="常规设置">
                  <div style="padding: 10px;">
                    <p><b>界面选项</b></p>
                    <a-space direction="vertical" style="width: 100%;">
                      <div style="display: flex; justify-content: space-between;">
                        <span>自动展开/折叠树形结果</span>
                        <a-switch v-model:checked="isTreeExpanded" />
                      </div>
                    </a-space>
                    <a-divider />
                    <div style="text-align: center; color: #999;">
                      {{ appInfo.name }} v{{ appInfo.version }}
                    </div>
                  </div>
                </a-tab-pane>
              </a-tabs>
            </a-modal>
          </div>
        </div>
      </div>
    </div>
  </ConfigProvider>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { ConfigProvider, message, theme, Modal } from 'ant-design-vue';
import { HomeOutlined, ReloadOutlined, MenuUnfoldOutlined, MenuFoldOutlined, InboxOutlined, PlayCircleOutlined, SearchOutlined, CopyOutlined, FilterOutlined, HistoryOutlined, ShareAltOutlined, DownOutlined, RightOutlined, CheckCircleOutlined, SettingOutlined, GithubOutlined, KeyOutlined, ExperimentOutlined, UserOutlined, LockOutlined, MessageOutlined, CopyFilled, SmileOutlined } from '@ant-design/icons-vue';
import { fetchModelList } from '../utils/api.js';
import { toggleTheme } from '../utils/theme.js';

const { t } = useI18n();
const isDarkMode = ref(false);
const configProviderTheme = computed(() => ({
  algorithm: isDarkMode.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}));

// State logic
const step = ref(1); // 1: upload, 2: select tree, 3: result table
const isLoadingModels = ref(false);
const isDiscoveringModels = ref(false);
const totalAccountsCount = ref(0);
const showExperimentalFeatures = ref(false);
const showAppSettingsModal = ref(false);
const settingsApiUrl = ref('');
const settingsApiKey = ref('');
const localCacheList = ref([]);
const isCloudLoggedIn = ref(false);
const cloudUrl = ref('');
const cloudPassword = ref('');
const cloudDataList = ref([]);

const appInfo = reactive({
  name: 'API Checker',
  subtitle: '批量 API 检测工具',
  version: '2.5.0',
  author: { url: 'https://github.com/jlwebs' }
});
const appDescription = ref(['支持 OpenAI / Claude / Gemini / NewAPI 等多种格式接口的批量并发检测与账号管理。']);

const openSettingsModal = () => {
  showAppSettingsModal.value = true;
  loadLocalCache();
};

const closeSettingsModal = () => {
  showAppSettingsModal.value = false;
};

const openGitHub = () => {
  window.open('https://github.com/jlwebs/api-check', '_blank');
};

const loadLocalCache = () => {
  const cache = localStorage.getItem('api_check_local_cache');
  if (cache) {
    try {
      localCacheList.value = JSON.parse(cache);
    } catch (e) {
      localCacheList.value = [];
    }
  }
};

const saveToLocal = () => {
  if (!settingsApiUrl.value || !settingsApiKey.value) {
    message.warning('请输入完整的 API URL 和 Key');
    return;
  }
  const newRecord = {
    id: Date.now(),
    name: new URL(settingsApiUrl.value).hostname,
    url: settingsApiUrl.value,
    apiKey: settingsApiKey.value
  };
  localCacheList.value.push(newRecord);
  localStorage.setItem('api_check_local_cache', JSON.stringify(localCacheList.value));
  message.success('保存成功');
};

const deleteLocalRecord = (id) => {
  localCacheList.value = localCacheList.value.filter(r => r.id !== id);
  localStorage.setItem('api_check_local_cache', JSON.stringify(localCacheList.value));
};

const loadLocalRecord = (id) => {
  const record = localCacheList.value.find(r => r.id === id);
  if (record) {
    // 批量模式通常是通过文件导入，这里加载到设置仅做展示或备用
    settingsApiUrl.value = record.url;
    settingsApiKey.value = record.apiKey;
    message.success('已加载到配置表单');
  }
};

const maskApiKey = (key) => {
  if (!key) return '';
  return key.slice(0, 8) + '***' + key.slice(-4);
};

const isTableExpanded = ref(true);
const isTreeExpanded = ref(true);
const highlightedTaskId = ref(null);
const tablePagination = ref({
  current: 1,
  pageSize: 15,
  showSizeChanger: true,
  pageSizeOptions: ['15', '30', '50', '100', '300', '500'],
});

const handleTableChange = (pagination) => {
  tablePagination.value = pagination;
};

const onTreeSelect = (selectedKeys, e) => {
  if (e.node.isLeaf) {
    const taskId = e.node.key;
    const idx = currentResultData.value.findIndex(item => item.id === taskId);
    if (idx !== -1) {
      isTableExpanded.value = true;
      highlightedTaskId.value = taskId;
      const targetPage = Math.floor(idx / tablePagination.value.pageSize) + 1;
      tablePagination.value.current = targetPage;
      setTimeout(() => {
        const row = document.querySelector(`[data-row-key="${taskId}"]`);
        if (row) {
          row.scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
      }, 100);
      
      setTimeout(() => {
        if (highlightedTaskId.value === taskId) {
          highlightedTaskId.value = null;
        }
      }, 3000);
    }
  }
};

const validAccounts = ref([]);
const treeData = ref([]);
const checkedKeys = ref([]);
const allKeys = ref([]); // Store all keys for easy 'Select All'

const loadedSitesCount = ref(0);
const browserSessionPolling = reactive({
  active: false,
  round: 0,
  totalRounds: 0,
  pending: 0,
});
const browserSessionPendingSiteNames = ref([]);

// 按 siteUrl 缓存余额，确保其为响应式对象
const siteQuotaCache = reactive({});

const batchConcurrency = ref(25);
const modelTimeout = ref(15);

const testing = ref(false);
const isRefreshingBalances = ref(false); // NEW: 刷新余额状态
const expandedKeys = ref([]); // NEW: 受控展开状态
const cancelTokens = ref([]); // to allow stopping

// ── NEW: 提取树形数据中所有的 Key 并展开/折叠 ──
const toggleExpandAll = () => {
  if (expandedKeys.value.length > 0) {
    // 当前有展开的，则执行“全部折叠”
    expandedKeys.value = [];
  } else {
    // 当前全部折叠，提取所有节点的 Key 执行“全部展开”
    const allKeys = [];
    const collectKeys = (nodes) => {
      nodes.forEach(node => {
        allKeys.push(node.key);
        if (node.children && node.children.length > 0) {
          collectKeys(node.children);
        }
      });
    };
    collectKeys(organizedTreeData.value);
    expandedKeys.value = allKeys;
  }
};
const testResults = ref([]); // all tasks
const totalTasks = ref(0);
const completedTasks = ref(0);
const resultModelFilter = ref('');

const ORGANIZED_REFRESH_INTERVAL_MS = 220;
const organizedSourceResults = ref([]);
let organizedRefreshTimer = null;

const refreshOrganizedSourceNow = () => {
  organizedSourceResults.value = [...testResults.value];
};

const scheduleOrganizedSourceRefresh = (force = false) => {
  if (force) {
    if (organizedRefreshTimer) {
      clearTimeout(organizedRefreshTimer);
      organizedRefreshTimer = null;
    }
    refreshOrganizedSourceNow();
    return;
  }
  if (organizedRefreshTimer) return;
  organizedRefreshTimer = setTimeout(() => {
    organizedRefreshTimer = null;
    refreshOrganizedSourceNow();
  }, ORGANIZED_REFRESH_INTERVAL_MS);
};

// Search & Filter State (Default no filter, no memory)
const searchQuery = ref('');
const filterOnlySuccess = ref(false);

const testProgress = computed(() => {
  if (totalTasks.value === 0) return 0;
  return Math.floor((completedTasks.value / totalTasks.value) * 100);
});
const browserSessionPendingSiteNameSet = computed(() => new Set(browserSessionPendingSiteNames.value));

// --- NEW Core Computed: Organized & Filtered Tree Data ---
const organizedTreeData = computed(() => {
  const results = organizedSourceResults.value;
  if (results.length === 0) return [];

  const keywords = searchQuery.value.trim().toLowerCase().split(/\s+/).filter(k => k);
  const modelKeywords = resultModelFilter.value.trim().toLowerCase().split(/\s+/).filter(k => k);
  
  // Grouping
  const groups = new Map();
  results.forEach(task => {
    const matchModel = modelKeywords.length === 0 || modelKeywords.some(k => task.modelName.toLowerCase().includes(k));
    // Keyword match: site name or model name matches ANY of symbols
    const matchSearch = keywords.length === 0 || keywords.some(k => 
      task.siteName.toLowerCase().includes(k) || 
      task.modelName.toLowerCase().includes(k)
    );
    
    // Status match
    const isError = task.status === 'error';
    if (filterOnlySuccess.value && isError) return;
    if (!matchModel) return;
    if (!matchSearch) return;

    const groupKey = `${task.siteName}|${task.apiKey}`;
    if (!groups.has(groupKey)) {
      groups.set(groupKey, {
        siteName: task.siteName,
        apiKey: task.apiKey,
        siteUrl: task.siteUrl,
        tasks: [],
        hasSuccess: false,
        hasWarning: false,
      });
    }
    const g = groups.get(groupKey);
    g.tasks.push(task);
    if (task.status === 'success') g.hasSuccess = true;
    if (task.status === 'warning') g.hasWarning = true;
  });

  // Convert to tree data & Sort
  const treeItems = Array.from(groups.values()).map(g => {
    const sortedTasks = [...g.tasks].sort((a, b) => {
      const order = { 'success': 0, 'warning': 1, 'error': 2, 'testing': 3, 'pending': 4 };
      return order[a.status] - order[b.status];
    });

    const siteKey = g.siteUrl?.replace(/\/+$/, '') || '';
    const quota = siteQuotaCache[siteKey];
    const quotaStr = (quota && !['获取中...', '无授权', '请求超时', '网络错误'].includes(quota)) 
      ? ` (剩余: ${quota.replace('$', '')} $)` 
      : '';

    // Determine node color/style
    let titleClass = 'tree-node-grey';
    if (g.hasSuccess) titleClass = 'tree-node-green';
    else if (g.hasWarning) titleClass = 'tree-node-orange';
    const isBrowserPending = browserSessionPolling.active && browserSessionPendingSiteNameSet.value.has(g.siteName);
    const pendingHint = isBrowserPending
      ? `后台检测中（第 ${Math.max(browserSessionPolling.round, 1)}/${Math.max(browserSessionPolling.totalRounds, 1)} 轮）`
      : '';

    return {
      title: `[${g.siteName}] ${g.apiKey.slice(0, 15)}...${quotaStr}`,
      key: `${g.siteName}|${g.apiKey}`,
      class: titleClass,
      isBrowserPending,
      pendingHint,
      children: sortedTasks.map(t => ({
        title: `${t.modelName}${t.modelSuffix || ''} - ${t.statusText} (${t.responseTime}s)`,
        displayTitle: t.displaySuffixHtml ? `${t.modelName}${t.displaySuffixHtml} - ${t.statusText} (${t.responseTime}s)` : null,
        key: t.id,
        isLeaf: true,
        class: `status-${t.status}`,
        // 核心修复：透传导出及拉起应用必备字段
        siteName: t.siteName,
        siteUrl: t.siteUrl,
        apiKey: t.apiKey,
        model: t.modelName
      })),
      hasSuccess: g.hasSuccess,
      hasWarning: g.hasWarning
    };
  });

  // Global Sort: Green > Orange > Grey
  return treeItems.sort((a, b) => {
    if (a.hasSuccess && !b.hasSuccess) return -1;
    if (!a.hasSuccess && b.hasSuccess) return 1;
    if (a.hasWarning && !b.hasWarning) return -1;
    if (!a.hasWarning && b.hasWarning) return 1;
    return 0;
  });
});

const currentResultData = computed(() => {
  const keywords = resultModelFilter.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (!keywords.length) return testResults.value;
  return testResults.value.filter(item => {
    const model = String(item?.modelName || '').toLowerCase();
    return keywords.some(k => model.includes(k));
  });
});

const resultColumns = [
  { title: '平台名称', dataIndex: 'siteName', width: 120 },
  { title: '请求Payload', dataIndex: 'payload', width: 150 },
  { title: '模型名称', dataIndex: 'modelName', width: 150 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '响应(s)', dataIndex: 'responseTime', width: 80 },
  { title: '备注信息', dataIndex: 'remark', ellipsis: true },
];

const hasHistory = ref(false);

const isEditorOpen = ref(false);
const editingRecord = ref(null);
const editingPayload = ref('');

const getMaskedKey = (key) => {
  if (!key) return '';
  if (key.length <= 10) return key;
  return key.slice(0, 5) + '...' + key.slice(-4);
};

const getPayloadJson = (record) => {
  return JSON.stringify({
    url: record.siteUrl ? record.siteUrl.replace(/\/+$/, '') : '',
    key: record.apiKey,
    model: record.modelName,
    messages: [{ role: 'user', content: 'hello' }]
  }, null, 2);
};

const truncateText = (input, max = 1200) => {
  const text = String(input || '');
  if (text.length <= max) return text;
  return `${text.slice(0, max)}\n...(truncated ${text.length - max} chars)`;
};

const tryParseJson = (input) => {
  try {
    return JSON.parse(String(input || ''));
  } catch {
    return null;
  }
};

const normalizeNestedErrorText = (raw) => {
  let cursor = raw;
  for (let i = 0; i < 2; i += 1) {
    if (!cursor || typeof cursor !== 'string') break;
    const trimmed = cursor.trim();
    if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) break;
    const parsed = tryParseJson(trimmed);
    if (!parsed || typeof parsed !== 'object') break;
    const next = parsed?.error?.message || parsed?.message || parsed?.error;
    if (typeof next === 'string' && next.trim()) {
      cursor = next;
      continue;
    }
    return trimmed;
  }
  return String(cursor || '');
};

const toReadableError = (rawData, fallback = '请求失败') => {
  if (!rawData) return fallback;
  const candidate = rawData?.error?.message || rawData?.message || rawData?.error || fallback;
  const normalized = normalizeNestedErrorText(candidate);
  const parsed = tryParseJson(normalized);
  if (parsed && typeof parsed === 'object') {
    return parsed?.error?.message || parsed?.message || fallback;
  }
  return normalized || fallback;
};

const toStatusTextByError = (messageText) => {
  const msg = String(messageText || '').toLowerCase();
  if (!msg) return '调用失败';
  if (msg.includes('html') || msg.includes('cloudflare')) return '静态页/风控';
  if (msg.includes('overloaded') || msg.includes('繁忙')) return '系统繁忙';
  if (msg.includes('余额不足') || msg.includes('insufficient')) return '余额不足';
  if (msg.includes('unauthorized') || msg.includes('401') || msg.includes('forbidden') || msg.includes('403')) return '鉴权失败';
  if (msg.includes('timeout') || msg.includes('超时')) return '请求超时';
  return '调用失败';
};

const getStatusTooltip = (record) => {
  const raw = String(record?.fullResponse || '').trim();
  if (!raw) return '无原始响应数据';
  const parsed = tryParseJson(raw);
  if (parsed && typeof parsed === 'object') {
    return truncateText(JSON.stringify(parsed, null, 2), 20000);
  }
  return truncateText(raw, 20000);
};

const formatBalance = (amount) => {
  if (amount == null) return '0.000';
  return (amount / 500000).toFixed(3);
};

const hoverQuota = (record) => {
  // 已有有效的缓存直接跳过
  if (record.quota !== undefined) return;

  const siteKey = record.siteUrl?.replace(/\/+$/, '') || '';

  // 命中缓存：同一 siteUrl 已算过
  if (siteQuotaCache[siteKey] !== undefined) {
    record.quota = siteQuotaCache[siteKey];
    return;
  }

  record.quota = '获取中...';

  // 非阻塞异步：用 access_token 请求 /api/user/self，取 quota 字段（插件同款）
  (async () => {
    const site = record.accountData;
    const siteUrl = siteKey;
    
    // 核心修复：经 scripts/verify-uid.cjs 验证通过，只允许纯数字 UID。UUID 会导致 401 格式错误。
    const rawId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
    const userId = /^\d+$/.test(String(rawId)) ? String(rawId) : '';
    
    // 优先 access_token（后台登录 token），其次用首个 sk key
    const auth = site?.account_info?.access_token || site?.access_token || site?.tokens?.[0]?.key;

    if (!auth || !siteUrl) {
      const label = '无授权';
      siteQuotaCache[siteKey] = label;
      testResults.value.forEach(r => { if (r.siteUrl?.replace(/\/+$/, '') === siteKey) r.quota = label; });
      return;
    }

    try {
      // 增加超时宽容度到 15s
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), 15000);
      
      // 核心修复：根据 site_type 动态决定端点。sub2api 类型通常是 /api/v1/auth/me
      const isSub2Api = site?.site_type === 'sub2api';
      const endpoints = isSub2Api 
        ? ['/api/v1/auth/me', '/api/user/self'] 
        : ['/api/user/self', '/api/v1/auth/me'];
      
      let quota = null;
      let finalResStatus = 200;

      for (const endpoint of endpoints) {
        const url = `${siteUrl}${endpoint}`;
        const uid = userId ? String(userId) : '';
        const proxyUrl = `/api/proxy-get?url=${encodeURIComponent(url)}&uid=${uid}`;

        const res = await fetch(proxyUrl, {
          headers: { 'Authorization': `Bearer ${auth}` },
          signal: controller.signal,
        });
        
        finalResStatus = res.status;
        if (res.ok) {
          const json = await res.json();
          // 深度兼容多种返回格式: NewAPI 的 data.quota, sub2api 的 data.user.quota 或 rix-api 的 balance/quota
          quota = json?.data?.quota ?? 
                  json?.quota ?? 
                  json?.data?.user?.quota ?? 
                  json?.user?.quota ?? 
                  json?.data?.balance ?? 
                  json?.balance ?? 
                  json?.total_quota ?? 
                  null;
          if (quota !== null) break;
        } else if ([401, 403].includes(res.status)) {
          // 只有鉴权明确失败(401)或者权限受限(403)时，才立刻中断循环
          break;
        }
      }

      clearTimeout(timer);

      if (quota !== null) {
        // 智能判定：如果是 balance 字段或者数值较小，通常是直观金额，不除以 500000
        const isDirectAmount = quota < 100000;
        const finalAmount = isDirectAmount ? Number(quota).toFixed(3) : (quota / 500000).toFixed(3);
        const label = `$${finalAmount}`;
        siteQuotaCache[siteKey] = label;
        testResults.value.forEach(r => { if (r.siteUrl?.replace(/\/+$/, '') === siteKey) r.quota = label; });
      } else {
        const label = `获取失败(${finalResStatus})`;
        siteQuotaCache[siteKey] = label;
        testResults.value.forEach(r => { if (r.siteUrl?.replace(/\/+$/, '') === siteKey) r.quota = label; });
      }
    } catch (e) {
      const label = e.name === 'AbortError' ? '请求超时' : '网络错误';
      siteQuotaCache[siteKey] = label;
      testResults.value.forEach(r => { if (r.siteUrl?.replace(/\/+$/, '') === siteKey) r.quota = label; });
    }
  })();
};

// ── NEW: 导入文件后直接预取所有额度 ──
const preloadAllQuotas = (extractedSites) => {
  if (!extractedSites || extractedSites.length === 0) return;
  
  extractedSites.forEach(site => {
    if (!site.site_url || site.error) return;
    
    // 模拟一个 record 结构调用 hoverQuota
    const mockRecord = {
      siteUrl: site.site_url,
      accountData: site
    };
    hoverQuota(mockRecord);
  });
};

// ── NEW: 批量异步强制刷新所有已选站点的余额 ──
const refreshAllBalances = async () => {
  if (isRefreshingBalances.value) return;
  
  const results = testResults.value;
  if (results.length === 0) {
    message.warning('当前暂无检测结果，无法刷新余额');
    return;
  }

  isRefreshingBalances.value = true;
  
  // 1. 清空所有 siteQuotaCache 缓存
  Object.keys(siteQuotaCache).forEach(key => delete siteQuotaCache[key]);
  
  // 2. 找到所有唯一的站点 URL
  const uniqueSites = new Map();
  results.forEach(r => {
    const siteKey = r.siteUrl?.replace(/\/+$/, '') || '';
    if (siteKey && !uniqueSites.has(siteKey)) {
      uniqueSites.set(siteKey, r);
    }
  });

  // 3. 异步并发刷新
  const promises = Array.from(uniqueSites.values()).map(record => {
    // 强制重置当前记录的 quota 状态，触发 hoverQuota 的重新获取
    delete record.quota; 
    return hoverQuota(record);
  });

  await Promise.allSettled(promises);
  isRefreshingBalances.value = false;
  message.success('余额刷新请求已全部发出');
};

// ── NEW: 一键拉起 Cherry Studio ──
const launchCherryStudio = (node) => {
  if (!node.apiKey || !node.siteUrl) {
    message.warning('配置信息不完整，无法导出');
    return;
  }
  
  const payload = {
    id: `batch-${node.key}`,
    baseUrl: node.siteUrl.replace(/\/+$/, ''),
    apiKey: node.apiKey,
    name: `${node.siteName} (${node.model})`
  };
  
  try {
    const jsonString = JSON.stringify(payload);
    // 使用 TextEncoder 处理 UTF-8 字符，确保中文字符名不乱码
    const bytes = new TextEncoder().encode(jsonString);
    const base64String = btoa(String.fromCharCode(...bytes));
    const url = `cherrystudio://providers/api-keys?v=1&data=${base64String}`;
    window.open(url, '_blank');
    message.success('正在尝试唤起 Cherry Studio...');
  } catch (err) {
    message.error('生成配置失败: ' + err.message);
  }
};

// ── NEW: 一键拉起 CC-Switch ──
const launchCCSwitch = (node) => {
  if (!node.apiKey || !node.siteUrl) {
    message.warning('配置信息不完整，无法导出');
    return;
  }

  const params = new URLSearchParams();
  params.set('resource', 'provider');
  params.set('app', 'claude'); // 默认映射为 claude 类型
  params.set('name', `${node.siteName} - ${node.model}`);
  params.set('homepage', node.siteUrl);
  params.set('endpoint', node.siteUrl);
  params.set('apiKey', node.apiKey);
  params.set('model', node.model);

  const url = `ccswitch://v1/import?${params.toString()}`;
  window.open(url, '_blank');
  message.success('正在尝试唤起 CC-Switch...');
};

const openPayloadEditor = (record) => {
  editingRecord.value = record;
  editingPayload.value = getPayloadJson(record);
  isEditorOpen.value = true;
};

const resendPayload = async () => {
  let custom;
  try {
    custom = JSON.parse(editingPayload.value);
  } catch(e) {
    message.error('JSON格式不正确，请检查！');
    return;
  }
  isEditorOpen.value = false;
  
    // Update task temporarily
  editingRecord.value.status = 'testing';
  editingRecord.value.statusText = '重测中';
  // If user changed the model or key in payload, do NOT change the table's display fields immediately unless we want to, but running with custom payload is fine.
  
  await runSingleTest(editingRecord.value, custom);
  
  // Also update history immediately
  localStorage.setItem('api_check_last_results', JSON.stringify(testResults.value));
};

onMounted(() => {
  isDarkMode.value = document.body.classList.contains('dark-mode');
  const hist = localStorage.getItem('api_check_last_results');
  if (hist) {
    try {
      const parsed = JSON.parse(hist);
      if (Array.isArray(parsed) && parsed.length > 0) {
        hasHistory.value = true;
      }
    } catch(e) {}
  }
});

const loadHistory = () => {
  const hist = localStorage.getItem('api_check_last_results');
  if (hist) {
    try {
      testResults.value = JSON.parse(hist);
      organizedSourceResults.value = [...testResults.value];
      step.value = 3;
      message.success('历史检测结果已恢复');
    } catch (e) {
      message.error('解析历史数据失败');
    }
  }
};

const handleToggleTheme = () => {
  toggleTheme(isDarkMode);
  document.body.classList.toggle('dark-mode', isDarkMode.value);
  document.body.classList.toggle('light-mode', !isDarkMode.value);
};

const resetStep1 = () => {
  step.value = 1;
  treeData.value = [];
  checkedKeys.value = [];
  validAccounts.value = [];
  testResults.value = [];
  organizedSourceResults.value = [];
};

const resetStep2 = () => {
  step.value = 2;
  testResults.value = [];
  organizedSourceResults.value = [];
  completedTasks.value = 0;
  totalTasks.value = 0;
};

const FALLBACK_BROWSER_STORAGE_KEY = 'batch_api_check_fallback_browser';

const isUsableToken = (token) => {
  const key = String(token?.key || token?.access_token || '').trim();
  if (!key) return false;
  if (token?.unresolved === true) return false;
  return !key.includes('*');
};

const countUsableTokensForSite = (site) => {
  const tokens = Array.isArray(site?.tokens) ? site.tokens : [];
  return tokens.filter(isUsableToken).length;
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

const mergeExtractedSiteResults = (baseResults, retryResults) => {
  const merged = Array.isArray(baseResults) ? baseResults : [];
  const stats = {
    mergedSites: 0,
    recoveredSites: 0,
    gainedTokens: 0,
    gainedUsableTokens: 0,
    changedSiteIds: [],
  };
  const changedSiteIdSet = new Set();

  retryResults.forEach(retryResult => {
    const idx = merged.findIndex(item => item?.id === retryResult?.id);
    if (idx === -1) return;
    const prev = merged[idx];
    const prevTokenCount = Array.isArray(prev?.tokens) ? prev.tokens.length : 0;
    const prevUsableCount = countUsableTokensForSite(prev);
    const prevInvalid = !prev || prev.error || !Array.isArray(prev.tokens) || prev.tokens.length === 0;
    const nextTokenCount = Array.isArray(retryResult?.tokens) ? retryResult.tokens.length : 0;
    const shouldReplace = nextTokenCount > 0 || prevInvalid;
    if (!shouldReplace) return;

    merged[idx] = retryResult;
    const nextUsableCount = countUsableTokensForSite(retryResult);
    stats.mergedSites += 1;
    if (prevInvalid && nextTokenCount > 0) {
      stats.recoveredSites += 1;
    }
    stats.gainedTokens += Math.max(0, nextTokenCount - prevTokenCount);
    stats.gainedUsableTokens += Math.max(0, nextUsableCount - prevUsableCount);
    const changedId = String(retryResult?.id ?? '').trim();
    if (changedId) changedSiteIdSet.add(changedId);
  });

  stats.changedSiteIds = Array.from(changedSiteIdSet);
  return stats;
};

const normalizeFallbackBrowserType = (value) => {
  return value === 'edge' ? 'edge' : 'chrome';
};

const getDetectedFallbackBrowser = async () => {
  const res = await fetch('/api/browser-session/browsers');
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(text || `探测系统浏览器失败(${res.status})`);
  }

  const data = await res.json().catch(() => ({}));
  const browsers = Array.isArray(data?.browsers) ? data.browsers : [];
  const availableTypes = browsers
    .map(item => item?.type)
    .filter(type => type === 'chrome' || type === 'edge');

  if (!availableTypes.length) {
    throw new Error('系统未检测到可用的 Chrome 或 Edge');
  }

  const saved = normalizeFallbackBrowserType(localStorage.getItem(FALLBACK_BROWSER_STORAGE_KEY) || '');
  const browserType = availableTypes.includes(saved)
    ? saved
    : (availableTypes.includes(data?.defaultBrowser) ? data.defaultBrowser : availableTypes[0]);

  localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, browserType);
  return {
    browserType,
    availableTypes,
  };
};

const getFallbackBrowserStatus = async (browserType = 'chrome') => {
  const normalizedType = normalizeFallbackBrowserType(browserType);
  const res = await fetch(`/api/browser-session/status?browserType=${normalizedType}`);
  if (!res.ok) {
    return { running: false, attached: false, browserType: normalizedType };
  }

  const data = await res.json().catch(() => ({}));
  return {
    running: Boolean(data?.running),
    attached: Boolean(data?.attached),
    launching: Boolean(data?.launching),
    managed: Boolean(data?.managed),
    browserType: data?.browserType || normalizedType,
  };
};

const chooseDetectedFallbackBrowserType = ({ browserType, availableTypes }) => {
  return new Promise(resolve => {
    if (!Array.isArray(availableTypes) || availableTypes.length <= 1) {
      localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, browserType);
      resolve(browserType);
      return;
    }

    Modal.confirm({
      title: '选择兜底浏览器',
      content: `检测到多个浏览器可用：${availableTypes.map(type => type === 'edge' ? 'Edge' : 'Chrome').join(' / ')}。请选择要用于兜底抓取的浏览器。`,
      okText: availableTypes.includes('chrome') ? '使用 Chrome' : '继续',
      cancelText: availableTypes.includes('edge') ? '使用 Edge' : '取消',
      closable: false,
      maskClosable: false,
      onOk: () => {
        const finalBrowserType = availableTypes.includes('chrome') ? 'chrome' : browserType;
        localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, finalBrowserType);
        resolve(finalBrowserType);
      },
      onCancel: () => {
        if (availableTypes.includes('edge')) {
          localStorage.setItem(FALLBACK_BROWSER_STORAGE_KEY, 'edge');
          resolve('edge');
          return;
        }
        resolve(null);
      },
    });
  });
};

const confirmWithModal = ({ title, content, okText = '确定', cancelText = '取消', okType = 'primary' }) => {
  return new Promise(resolve => {
    Modal.confirm({
      title,
      content,
      okText,
      cancelText,
      okType,
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
};

const openSitesInBrowserSession = async (sites, browserType = 'chrome') => {
  const payload = sites
    .map(site => ({
      name: site?.site_name || '未知站点',
      url: String(site?.site_url || '').replace(/\/+$/, ''),
    }))
    .filter(site => /^https?:\/\//i.test(site.url));

  if (!payload.length) return 0;

  const res = await fetch('/api/browser-session/open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sites: payload, browserType }),
  });

  if (!res.ok) {
    const data = await res.json().catch(() => null);
    const error = new Error(data?.message || `打开受控浏览器失败(${res.status})`);
    error.code = data?.code || `HTTP_${res.status}`;
    throw error;
  }

  const data = await res.json().catch(() => ({}));
  return Number(data?.opened || payload.length);
};

const restartBrowserSessionProcessAndOpen = async (sites, browserType = 'chrome') => {
  const payload = sites
    .map(site => ({
      name: site?.site_name || '未知站点',
      url: String(site?.site_url || '').replace(/\/+$/, ''),
    }))
    .filter(site => /^https?:\/\//i.test(site.url));

  const res = await fetch('/api/browser-session/restart-open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ browserType, sites: payload }),
  });

  if (!res.ok) {
    const data = await res.json().catch(() => null);
    const error = new Error(data?.message || `重启浏览器并打开站点失败(${res.status})`);
    error.code = data?.code || `HTTP_${res.status}`;
    throw error;
  }

  return await res.json().catch(() => ({}));
};

const browserSessionFetchForAccounts = async (accounts, browserType = 'chrome', round = 1, totalRounds = 1) => {
  if (!accounts.length) return [];

  const res = await fetch('/api/browser-session/fetch-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ accounts, browserType, round, totalRounds }),
  });

  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(text || `浏览器会话抓取失败(${res.status})`);
  }

  const data = await res.json().catch(() => ({}));
  return Array.isArray(data?.results) ? data.results : [];
};

const sleep = (ms) => new Promise(resolve => setTimeout(resolve, ms));

// --- 浏览器端直接提取Token（绕过Cloudflare WAF服务端拦截）---
// 核心原理：Cloudflare Bot Protection会拦截无TLS指纹的服务器请求，
// 但放行真实浏览器发出的请求（有JA3 TLS指纹+clearance cookie）
const fetchTokensForAccountFromBrowser = async (acc) => {
  const { id, site_name, site_url, site_type, account_info } = acc;
  const apiKey = account_info?.access_token;
  const baseUrl = (site_url || '').replace(/\/+$/, '');
  const uid = account_info?.id;

  if (!apiKey || !baseUrl) {
    return { id, site_name, site_url, tokens: [], error: '缺少 access_token 或 site_url', account_info };
  }

  // 优先级端点列表：参考all-api-hub的实现策略
  let endpoints;
  if (site_type === 'sub2api') {
    // sub2api使用JWT token，对应不同的API路径
    endpoints = [
      `/api/v1/keys?page=1&page_size=100`,
      `/api/v1/keys?p=0&size=100`,
      `/api/token/?p=0&size=100`,
    ];
  } else {
    // oneAPI / newAPI / anyrouter 等
    endpoints = [
      `/api/token/?p=0&size=100`,
      `/api/token?p=0&size=100`,
      `/api/v1/keys?page=1&page_size=100`,
    ];
  }

  const headers = {
    'Authorization': `Bearer ${apiKey}`,
    'Accept': 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'X-Requested-With': 'XMLHttpRequest',
  };
  // 如果uid是纯数字，加入兼容头（参考all-api-hub的compat headers）
  if (uid && /^\d+$/.test(String(uid))) {
    headers['new-api-user'] = String(uid);
    headers['one-api-user'] = String(uid);
    headers['New-API-User'] = String(uid);
    headers['Veloera-User'] = String(uid);
    headers['voapi-user'] = String(uid);
    headers['User-id'] = String(uid);
    headers['Rix-Api-User'] = String(uid);
    headers['neo-api-user'] = String(uid);
  }

  const isMaskedKey = (value) => {
    const key = String(value || '').trim();
    if (!key) return false;
    return key.includes('*') || key.includes('***');
  };

  const extractSecretKeyFromPayload = (payload) => {
    if (!payload) return '';
    if (typeof payload === 'string') return payload.trim();
    if (typeof payload !== 'object') return '';
    const candidates = [
      payload?.key,
      payload?.data?.key,
      payload?.data,
      payload?.result?.key,
      payload?.result?.data?.key,
      payload?.token,
    ];
    for (const candidate of candidates) {
      if (typeof candidate === 'string' && candidate.trim()) {
        return candidate.trim();
      }
    }
    return '';
  };

  for (const endpoint of endpoints) {
    try {
      const url = `${baseUrl}${endpoint}`;
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 10000);

      const response = await fetch(url, {
        method: 'GET',
        headers,
        signal: controller.signal,
        credentials: 'include',
        mode: 'cors',
        referrer: `${baseUrl}/`,
      });
      clearTimeout(timeout);

      if (!response.ok) {
        // 403被CF拦截：检查是否返回了HTML（CF页面）
        if (response.status === 403) {
          const ct = response.headers.get('content-type') || '';
          if (/html/i.test(ct)) {
            // CF Bot Protection，浏览器也无法直接绕（需要challenge）
            // 继续试其他端点
            continue;
          }
        }
        continue;
      }

      // 检查Content-Type，CF挡截页也可能是200但返回HTML
      const ct = response.headers.get('content-type') || '';
      if (/html/i.test(ct)) continue;

      let body;
      try {
        body = await response.json();
      } catch (e) {
        continue; // 非JSON，跳过
      }

      // 解析不同格式的响应
      let items = [];
      if (body && body.data !== undefined) {
        const data = body.data;
        if (Array.isArray(data)) items = data;
        else if (data && Array.isArray(data.items)) items = data.items;
      } else if (body && Array.isArray(body.items)) {
        items = body.items;
      } else if (Array.isArray(body)) {
        items = body;
      }

      const resolvedItems = [];
      for (const t of items) {
        const rawKey = t.key || t.access_token || t.token || t.api_key || t.apikey || (typeof t === 'string' ? t : '');
        resolvedItems.push({ ...t, key: rawKey || '未知格式Token' });
      }

      // 二次处理：掩码 key 尝试补全，避免“提取数量很多但最终可用很少”
      if (resolvedItems.length > 0) {
        const normalizedResolvedItems = [];
        for (const t of items) {
          const rawKey = t.key || t.access_token || t.token || t.api_key || t.apikey || (typeof t === 'string' ? t : '');
          let resolvedKey = rawKey || '';
          let unresolved = false;

          if (isMaskedKey(rawKey) && t?.id) {
            const secretEndpointCandidates = [
              { path: `/api/token/${t.id}/key`, method: 'POST' },
              { path: `/api/token/${t.id}/key`, method: 'GET' },
              { path: `/api/token/${t.id}`, method: 'GET' },
              { path: `/api/v1/keys/${t.id}`, method: 'GET' },
            ];
            for (const secretEp of secretEndpointCandidates) {
              try {
                const secretRes = await fetch(`${baseUrl}${secretEp.path}`, {
                  method: secretEp.method,
                  headers: {
                    ...headers,
                    ...(secretEp.method !== 'GET' ? { 'Content-Type': 'application/json' } : {}),
                  },
                  credentials: 'include',
                  mode: 'cors',
                  referrer: `${baseUrl}/`,
                });
                if (!secretRes.ok) continue;
                const secretBody = await secretRes.json().catch(() => null);
                const fullKey = extractSecretKeyFromPayload(secretBody);
                if (fullKey) {
                  resolvedKey = fullKey;
                  break;
                }
              } catch {}
            }
            unresolved = isMaskedKey(resolvedKey);
          }

          normalizedResolvedItems.push({
            ...t,
            key: resolvedKey || '未知格式Token',
            unresolved,
          });
        }
        resolvedItems.length = 0;
        resolvedItems.push(...normalizedResolvedItems);
      }

      if (resolvedItems && resolvedItems.length > 0) {
        console.log(`[BrowserFetch] ${site_name} | ${endpoint} => ${resolvedItems.length}个token`);
        return { id, site_name, site_url, tokens: resolvedItems, endpoint, account_info, _browserFetched: true };
      }
    } catch (err) {
      if (err.name === 'AbortError') continue;
      // CORS错误或网络错误，继续
      console.debug(`[BrowserFetch] ${site_name} | ${endpoint} CORS/网络错误:`, err.message);
      continue;
    }
  }

  // 所有浏览器端端点均失败，返回失败标记（由processAccounts fallback到服务端）
  return {
    id,
    site_name,
    site_url,
    tokens: [],
    error: '浏览器端所有端点均失败，将尝试服务端代理',
    account_info,
    _needServerFallback: true,
    _browserFetchFailed: true,
  };
};

const extractBrowserListItems = (body) => {
  if (Array.isArray(body)) return body;
  if (!body || typeof body !== 'object') return [];
  if (Array.isArray(body.items)) return body.items;
  if (Array.isArray(body.data)) return body.data;
  if (body.data && typeof body.data === 'object') {
    if (Array.isArray(body.data.items)) return body.data.items;
    if (Array.isArray(body.data.data)) return body.data.data;
  }
  return [];
};

const extractSecretKeyFromPayloadForBrowser = (payload) => {
  if (!payload) return '';
  if (typeof payload === 'string') return payload.trim();
  if (typeof payload !== 'object') return '';
  const candidates = [
    payload?.key,
    payload?.data?.key,
    payload?.data,
    payload?.result?.key,
    payload?.result?.data?.key,
    payload?.token,
  ];
  for (const candidate of candidates) {
    if (typeof candidate === 'string' && candidate.trim()) {
      return candidate.trim();
    }
  }
  return '';
};

const fetchTokensForAccountFromBrowserV2 = async (acc) => {
  const { id, site_name, site_url, site_type, account_info } = acc;
  const apiKey = String(account_info?.access_token || '').trim();
  const baseUrl = String(site_url || '').replace(/\/+$/, '');
  const uid = String(account_info?.id || '').trim();

  if (!apiKey || !baseUrl) {
    return { id, site_name, site_url, tokens: [], error: '缺少 access_token 或 site_url', account_info };
  }

  const endpoints = site_type === 'sub2api'
    ? [
      '/api/v1/keys?page=1&page_size=100',
      '/api/v1/keys?p=0&size=100',
      '/api/token/?p=0&size=100',
      '/api/token?p=0&size=100',
    ]
    : [
      '/api/token/?p=0&size=100',
      '/api/token?p=0&size=100',
      '/api/v1/keys?page=1&page_size=100',
      '/api/v1/keys?p=0&size=100',
    ];

  const headers = {
    Authorization: `Bearer ${apiKey}`,
    Accept: 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'X-Requested-With': 'XMLHttpRequest',
  };

  if (/^\d+$/.test(uid)) {
    headers['new-api-user'] = uid;
    headers['one-api-user'] = uid;
    headers['New-API-User'] = uid;
    headers['Veloera-User'] = uid;
    headers['voapi-user'] = uid;
    headers['User-id'] = uid;
    headers['Rix-Api-User'] = uid;
    headers['neo-api-user'] = uid;
  }

  const isMaskedKey = (value) => {
    const key = String(value || '').trim();
    return Boolean(key) && key.includes('*');
  };

  const resolveMaskedKey = async (tokenId) => {
    const endpointCandidates = [
      { path: `/api/token/${tokenId}/key`, method: 'POST' },
      { path: `/api/token/${tokenId}/key`, method: 'GET' },
      { path: `/api/token/${tokenId}`, method: 'GET' },
      { path: `/api/v1/keys/${tokenId}`, method: 'GET' },
    ];

    for (const endpoint of endpointCandidates) {
      try {
        const res = await fetch(`${baseUrl}${endpoint.path}`, {
          method: endpoint.method,
          headers: {
            ...headers,
            ...(endpoint.method !== 'GET' ? { 'Content-Type': 'application/json' } : {}),
          },
          credentials: 'include',
          mode: 'cors',
          referrer: `${baseUrl}/`,
        });
        if (!res.ok) continue;
        const payload = await res.json().catch(() => null);
        const key = extractSecretKeyFromPayloadForBrowser(payload);
        if (key) return key;
      } catch {}
    }
    return '';
  };

  for (const endpoint of endpoints) {
    try {
      const url = `${baseUrl}${endpoint}`;
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 10000);

      const response = await fetch(url, {
        method: 'GET',
        headers,
        signal: controller.signal,
        credentials: 'include',
        mode: 'cors',
        referrer: `${baseUrl}/`,
      });
      clearTimeout(timeout);

      if (!response.ok) {
        if (response.status === 403) {
          const ct = response.headers.get('content-type') || '';
          if (/html/i.test(ct)) continue;
        }
        continue;
      }

      const ct = response.headers.get('content-type') || '';
      if (/html/i.test(ct)) continue;

      const body = await response.json().catch(() => null);
      if (!body) continue;

      const items = extractBrowserListItems(body);
      if (!items.length) continue;

      const resolvedItems = [];
      for (const item of items) {
        const rawKey = item?.key || item?.access_token || item?.token || item?.api_key || item?.apikey || (typeof item === 'string' ? item : '');
        let key = String(rawKey || '').trim();
        if (isMaskedKey(key) && item?.id) {
          const fullKey = await resolveMaskedKey(item.id);
          if (fullKey) key = fullKey;
        }
        resolvedItems.push({
          ...item,
          key: key || '未知格式Token',
          unresolved: isMaskedKey(key),
        });
      }

      if (resolvedItems.length > 0) {
        const usableCount = resolvedItems.filter(isUsableToken).length;
        const unresolvedCount = resolvedItems.length - usableCount;
        const detailPreview = resolvedItems
          .slice(0, 5)
          .map((token, idx) => {
            const tokenId = token?.id ?? token?.token_id ?? `idx${idx + 1}`;
            const tokenKey = String(token?.key || '').trim();
            const keyPreview = tokenKey ? `${tokenKey.slice(0, 12)}...${tokenKey.slice(-4)}` : '(empty-key)';
            const tokenName = String(token?.name || token?.token_name || '').trim();
            return `#${tokenId}${tokenName ? `(${tokenName})` : ''}:${keyPreview}`;
          })
          .join(' | ');
        console.log(`[BrowserFetch] [${site_name}] ${endpoint} 获取成功: count=${resolvedItems.length}, usable=${usableCount}, unresolved=${unresolvedCount}, 明细=${detailPreview || '(no-preview)'}`);
        return { id, site_name, site_url, tokens: resolvedItems, endpoint, account_info, _browserFetched: true };
      }
    } catch (err) {
      if (err?.name === 'AbortError') continue;
      console.debug(`[BrowserFetch] ${site_name} | ${endpoint} CORS/网络错误:`, err?.message || String(err));
      continue;
    }
  }

  return {
    id,
    site_name,
    site_url,
    tokens: [],
    error: '浏览器端所有端点均失败，将尝试服务端代理',
    account_info,
    _needServerFallback: true,
    _browserFetchFailed: true,
  };
};

// --- Upload and Parse ---
const beforeUpload = (file) => {
  const reader = new FileReader();
  reader.onload = (e) => {
    try {
      const data = JSON.parse(e.target.result);
      if (data && data.accounts && Array.isArray(data.accounts.accounts)) {
        processAccountsV2(data.accounts.accounts);
      } else {
        message.error('无效的文件格式: 缺少 accounts 数组');
      }
    } catch (err) {
      message.error('解析 JSON 文件出错');
    }
  };
  reader.readAsText(file);
  return false; // prevent automatic upload
};

const updateBrowserSessionPendingSites = (sites) => {
  browserSessionPendingSiteNames.value = (Array.isArray(sites) ? sites : [])
    .map(site => String(site?.site_name || '').trim())
    .filter(Boolean);
};

const processAccounts = async (accounts) => {
  const accountsToFetch = accounts.filter(acc => 
    !acc.disabled && 
    acc.site_url && 
    acc.account_info && 
    acc.account_info.access_token
  );
  
  if (accountsToFetch.length === 0) {
    message.warning('备份文件中未找到可用账号配置！');
    return;
  }
  
  // ── 第 0 步：清空后端日志 ──
  try {
    await fetch('/api/clear-logs?type=fetch', { method: 'POST' });
    await fetch('/api/clear-logs?type=check', { method: 'POST' });
  } catch (e) {
    console.warn('Clear logs fail, ignoring...', e);
  }

  totalAccountsCount.value = accountsToFetch.length;
  isLoadingModels.value = true;
  step.value = -1; // 显示提取中的中间状态
  loadedSitesCount.value = 0;
  browserSessionPolling.active = false;
  browserSessionPolling.round = 0;
  browserSessionPolling.totalRounds = 0;
  browserSessionPolling.pending = 0;
  browserSessionPendingSiteNames.value = [];
  
  // ── 第 1 步：先用浏览器端直接并发提取（绕过Cloudflare WAF服务端拦截）──
  let extractedSites = [];
  try {
    const BROWSER_FETCH_CONCURRENCY = 25;
    const browserResults = new Array(accountsToFetch.length);
    let currentIdx = 0;

    const browserFetchWorker = async () => {
      while (currentIdx < accountsToFetch.length) {
        const idx = currentIdx++;
        browserResults[idx] = await fetchTokensForAccountFromBrowserV2(accountsToFetch[idx]);
      }
    };

    const browserWorkers = Array.from(
      { length: Math.min(BROWSER_FETCH_CONCURRENCY, accountsToFetch.length) },
      () => browserFetchWorker()
    );
    await Promise.all(browserWorkers);

    // 将浏览器端成功的结果同步
    extractedSites = browserResults;

    // 对于浏览器端提取失败的（_needServerFallback=true），Fallback到服务端代理
    const failedAccounts = accountsToFetch.filter((acc, i) => 
      browserResults[i]?._needServerFallback === true
    );

    if (failedAccounts.length > 0) {
      console.log(`[FetchKeys] 浏览器端失败 ${failedAccounts.length} 个，尝试服务端代理墙跑...`);
      try {
        const serverResponse = await fetch('/api/fetch-keys', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ accounts: failedAccounts }),
        });
        if (serverResponse.ok) {
          const serverData = await serverResponse.json();
          const serverResults = serverData.results || [];
          // 将服务端成功的结果写回 extractedSites
          serverResults.forEach(serverResult => {
            const idx = accountsToFetch.findIndex(a => a.id === serverResult.id);
            if (idx !== -1) {
              // 强制将服务端获取到的结果覆盖浏览器的初始错误态，不管服务端有没有取到token
              extractedSites[idx] = serverResult;
            }
          });
        }
      } catch (e) {
        console.warn('[FetchKeys] 服务端墙跑失败:', e.message);
      }
    }

    let stillFailedAccounts = extractedSites.filter(site =>
      !site || site.error || !site.tokens || site.tokens.length === 0
    );
    updateBrowserSessionPendingSites(stillFailedAccounts);

    // 先展示当前可得结果，不阻塞后续流程
    validAccounts.value = extractedSites;
    preloadAllQuotas(extractedSites);
    const nowSuccessSites = extractedSites.filter(site => site && !site.error && Array.isArray(site.tokens) && site.tokens.length > 0).length;
    const nowTokenCount = extractedSites.reduce((sum, site) => sum + (Array.isArray(site?.tokens) ? site.tokens.length : 0), 0);
    console.log(`[FetchKeys] 当前阶段完成: successSites=${nowSuccessSites}/${extractedSites.length}, totalTokens=${nowTokenCount}, pendingSites=${stillFailedAccounts.length}`);

    if (stillFailedAccounts.length > 0) {
      void (async () => {
        try {
          const detected = await getDetectedFallbackBrowser();
          const browserType = await chooseDetectedFallbackBrowserType(detected);
          if (!browserType) {
            message.warning('你取消了浏览器选择，当前保留已有结果并继续后续流程。');
            return;
          }

          let openedCount = 0;
          try {
            openedCount = await openSitesInBrowserSession(stillFailedAccounts, browserType);
          } catch (openErr) {
            const fallbackStatus = await getFallbackBrowserStatus(browserType).catch(() => ({
              running: false,
              attached: false,
              launching: false,
              managed: false,
              browserType,
            }));
            const shouldHandleAsProfileInUse =
              openErr?.code === 'BROWSER_PROFILE_IN_USE' ||
              (fallbackStatus.running && !fallbackStatus.attached && !fallbackStatus.launching && !fallbackStatus.managed);

            if (shouldHandleAsProfileInUse) {
              const shouldKill = await confirmWithModal({
                title: '浏览器已占用',
                content: `${browserType === 'edge' ? 'Edge' : 'Chrome'} 当前已在普通模式运行，默认 profile 被占用。结束后会关闭该浏览器的所有窗口。是否结束并立即以受控模式重新打开目标站点？`,
                okText: '结束并继续',
                cancelText: '取消',
                okType: 'danger',
              });
              if (!shouldKill) {
                message.warning('你取消了结束浏览器进程，当前保留已有结果并继续后续流程。');
                return;
              }

              const restartResult = await restartBrowserSessionProcessAndOpen(stillFailedAccounts, browserType);
              if (!restartResult?.stopped) {
                message.error(`${browserType === 'edge' ? 'Edge' : 'Chrome'} 进程结束后仍未完全退出，请手动关闭后再重试。`);
                return;
              }
              openedCount = Number(restartResult?.opened || stillFailedAccounts.length);
            } else {
              throw openErr;
            }
          }

          if (openedCount <= 0) return;

          const availableText = detected.availableTypes.map(type => type === 'edge' ? 'Edge' : 'Chrome').join(' / ');
          message.info(`已智能探测到 ${availableText}，当前使用 ${browserType === 'edge' ? 'Edge' : 'Chrome'} 打开 ${openedCount} 个失败站点，后台自动轮询抓取中。`, 6);

          const maxRetryRounds = 3;
          const retryIntervalMs = 15000;
          browserSessionPolling.active = true;
          browserSessionPolling.totalRounds = maxRetryRounds;
          browserSessionPolling.pending = stillFailedAccounts.length;
          updateBrowserSessionPendingSites(stillFailedAccounts);

          try {
            for (let round = 1; round <= maxRetryRounds && stillFailedAccounts.length > 0; round += 1) {
              browserSessionPolling.round = round;
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              console.log(`[FetchKeys] 受控浏览器(${browserType})自动抓取，第 ${round}/${maxRetryRounds} 轮，当前失败站点 ${stillFailedAccounts.length} 个`);

              const browserSessionResults = await browserSessionFetchForAccounts(stillFailedAccounts, browserType, round, maxRetryRounds);
              extractedSites = mergeExtractedSiteResults(extractedSites, browserSessionResults);
              validAccounts.value = extractedSites;
              preloadAllQuotas(extractedSites);

              stillFailedAccounts = extractedSites.filter(site =>
                !site || site.error || !site.tokens || site.tokens.length === 0
              );
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              const roundSuccessSites = extractedSites.filter(site => site && !site.error && Array.isArray(site.tokens) && site.tokens.length > 0).length;
              const roundTokenCount = extractedSites.reduce((sum, site) => sum + (Array.isArray(site?.tokens) ? site.tokens.length : 0), 0);
              console.log(`[FetchKeys] 受控浏览器(${browserType})第 ${round}/${maxRetryRounds} 轮结束: successSites=${roundSuccessSites}/${extractedSites.length}, totalTokens=${roundTokenCount}, pendingSites=${stillFailedAccounts.length}`);

              if (stillFailedAccounts.length === 0) break;
              if (round < maxRetryRounds) {
                await sleep(retryIntervalMs);
              }
            }
          } finally {
            browserSessionPolling.active = false;
            browserSessionPolling.round = 0;
            browserSessionPolling.totalRounds = 0;
            browserSessionPolling.pending = 0;
            browserSessionPendingSiteNames.value = [];
          }

          if (stillFailedAccounts.length > 0) {
            message.warning(`受控浏览器自动轮询完成，仍有 ${stillFailedAccounts.length} 个站点未抓取成功。`);
          }
        } catch (e) {
          console.warn('[FetchKeys] 受控浏览器兜底失败:', e.message);
          message.warning(`失败站点受控浏览器兜底未执行成功: ${e.message}`);
        }
      })();
    }
  } catch (err) {
    message.error(`批量提取 Token 失败: ${err.message}`);
    isLoadingModels.value = false;
    step.value = 1;
    return;
  }

    const discoverySites = [...extractedSites];
    const siteNodes = new Array(discoverySites.length);
    const fullCheckedKeys = [];
    const fullAllKeys = [];

    // ── 第 2 步：探测模型 (采用分流多进程) ──
    const discoveryLimit = 25; 
    let currentIndex = 0;

    const discoverWorker = async () => {
      while (currentIndex < discoverySites.length) {
        const globalIdx = currentIndex++;
        const site = discoverySites[globalIdx];
        
        const siteIdx = globalIdx + 1;
        const siteDisplayTitle = `${siteIdx}. [${site.site_name}]`;
        const currentSiteNodes = [];

        // ── 情况 A: 令牌提取报错 ──
        if (!site || site.error || !site.tokens || site.tokens.length === 0) {
          const errorMsg = site.error || '获取令牌失败';
          currentSiteNodes.push({
            title: `${siteDisplayTitle} - ❌ ${errorMsg}`,
            key: `fail-site|${site.id || globalIdx}`,
            disabled: true,
            selectable: false,
            children: []
          });
          siteNodes[globalIdx] = currentSiteNodes;
          loadedSitesCount.value++;
          continue;
        }

        // 探测模型
        let effectiveBaseUrl = site.site_url.replace(/\/+$/, '');
        const rawApiKey = String(site.api_key || '').trim();
        if (rawApiKey.startsWith('http')) effectiveBaseUrl = rawApiKey.replace(/\/+$/, '');
        
        const baseUrl = effectiveBaseUrl;
        const firstToken = site.tokens[0];
        const testApiKey = firstToken.key || firstToken.access_token;
        
        let supportedModels = [];
        const endpointsToTry = [
          { url: `${baseUrl}/v1/models`, type: 'openai' },
          { url: `${baseUrl}/api/models`, type: 'newapi_public' },
          { url: `${baseUrl}/api/user/models`, type: 'newapi_user' }
        ];

        for (const ep of endpointsToTry) {
          try {
            const rawDiscoveryId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
            const discoveryUid = /^\d+$/.test(String(rawDiscoveryId)) ? String(rawDiscoveryId) : '';
            const res = await fetch(`/api/proxy-get?url=${encodeURIComponent(ep.url)}&uid=${discoveryUid}`, {
              headers: { Authorization: `Bearer ${testApiKey}` }
            });
            if (res.ok) {
              const result = await res.json();
              let rawData = Array.isArray(result) ? result : (result.data?.data || result.data?.items || result.data || []);
              if (rawData.length > 0) {
                supportedModels = rawData.map(m => (typeof m === 'string' ? m : (m.id || m.name || m))).filter(m => typeof m === 'string').sort();
                if (supportedModels.length > 0) break;
              }
            }
          } catch (e) {}
        }

        // ── 情况 B: 探测不到模型 ──
        if (supportedModels.length === 0) {
          console.log(`[FetchKeys] 模型发现失败: [${site.site_name}] tokenCount=${site.tokens?.length || 0}, firstToken=${String(testApiKey || '').slice(0, 12)}...`);
          currentSiteNodes.push({
            title: `${siteDisplayTitle} - ⚠️ 未能探测到可用模型列表`,
            key: `no-model-site|${site.id}`,
            disabled: true,
            selectable: false,
            children: []
          });
        } else {
          // ── 情况 C: 正常 ──
          site.tokens.forEach((token, idx) => {
            const tKey = token.key || token.access_token;
            const tName = token.name || `Token ${idx + 1}`;
            const tokenNodeKey = `token|${site.id}|${tKey}`;
            const children = supportedModels.map(model => {
              const itemKey = `${site.id}|${tKey}|${model}`;
              fullAllKeys.push(itemKey);
              fullCheckedKeys.push(itemKey);
              return { title: model, key: itemKey, isLeaf: true };
            });
            fullAllKeys.push(tokenNodeKey);
            fullCheckedKeys.push(tokenNodeKey);
            currentSiteNodes.push({
              title: `${siteDisplayTitle} ${tName} (${tKey.slice(0, 15)}...)`,
              key: tokenNodeKey,
              children: children,
            });
          });
        }

        siteNodes[globalIdx] = currentSiteNodes;
        loadedSitesCount.value++;
      }
    };

    const discoveryWorkers = Array.from({ length: Math.min(discoveryLimit, discoverySites.length) }, () => discoverWorker());
    await Promise.all(discoveryWorkers);

    const discoveredSiteCount = discoverySites.filter(site => site && !site.error && Array.isArray(site.tokens) && site.tokens.length > 0).length;
    const failedSiteCount = discoverySites.length - discoveredSiteCount;
    const selectableModelCount = fullAllKeys.filter(key => key.includes('|')).length;
    console.log(`[FetchKeys] 模型发现阶段完成: tokenSites=${discoveredSiteCount}, failedSites=${failedSiteCount}, selectableModels=${selectableModelCount}`);

    treeData.value = siteNodes.flat().filter(Boolean);
    allKeys.value = fullAllKeys;
    checkedKeys.value = fullCheckedKeys;
    isLoadingModels.value = false;
    step.value = 2; // 进入树形选择器
  };
  
const processAccountsV2 = async (accounts) => {
  const accountsToFetch = (Array.isArray(accounts) ? accounts : []).filter(acc =>
    !acc?.disabled &&
    acc?.site_url &&
    acc?.account_info &&
    acc?.account_info?.access_token
  );

  if (accountsToFetch.length === 0) {
    message.warning('备份文件中未找到可用账号配置');
    return;
  }

  try {
    await fetch('/api/clear-logs?type=fetch', { method: 'POST' });
    await fetch('/api/clear-logs?type=check', { method: 'POST' });
  } catch (e) {
    console.warn('Clear logs fail, ignoring...', e);
  }

  totalAccountsCount.value = accountsToFetch.length;
  loadedSitesCount.value = 0;
  isLoadingModels.value = true;
  isDiscoveringModels.value = false;
  step.value = -1;
  treeData.value = [];
  checkedKeys.value = [];
  allKeys.value = [];

  browserSessionPolling.active = false;
  browserSessionPolling.round = 0;
  browserSessionPolling.totalRounds = 0;
  browserSessionPolling.pending = 0;
  browserSessionPendingSiteNames.value = [];

  const isSiteFailed = (site) => !site || site.error || !Array.isArray(site.tokens) || site.tokens.length === 0;
  const getPendingHint = () => `后台检测中（第 ${Math.max(browserSessionPolling.round, 1)}/${Math.max(browserSessionPolling.totalRounds, 1)} 轮）`;
  const withPendingMeta = (siteName, node) => {
    const normalizedSiteName = String(siteName || '').trim();
    const pending = browserSessionPolling.active && browserSessionPendingSiteNameSet.value.has(normalizedSiteName);
    return {
      ...node,
      siteName: normalizedSiteName,
      isBrowserPending: pending,
      pendingHint: pending ? getPendingHint() : '',
    };
  };
  const summarizeStage = (tag, sites, pendingSites = []) => {
    const safeSites = Array.isArray(sites) ? sites : [];
    const successSites = safeSites.filter(site => !isSiteFailed(site)).length;
    const totalTokens = safeSites.reduce((sum, site) => sum + (Array.isArray(site?.tokens) ? site.tokens.length : 0), 0);
    const usableTokens = safeSites.reduce((sum, site) => sum + countUsableTokensForSite(site), 0);
    const unresolvedTokens = Math.max(0, totalTokens - usableTokens);
    const pendingCount = Array.isArray(pendingSites) ? pendingSites.length : 0;
    console.log(
      `[FetchKeys] ${tag}: successSites=${successSites}/${safeSites.length}, totalTokens=${totalTokens}, usableTokens=${usableTokens}, unresolvedTokens=${unresolvedTokens}, pendingSites=${pendingCount}`
    );
  };
  const refreshTreePendingHints = () => {
    if (!Array.isArray(treeData.value) || treeData.value.length === 0) return;
    treeData.value = treeData.value.map(node => withPendingMeta(node?.siteName || '', node));
  };

  let extractedSites = [];
  let initialDiscoveryCompleted = false;
  let discoveryInFlight = false;
  let discoveryQueued = false;
  let discoveryQueuedReason = '';
  let discoveryVersion = 0;

  const runModelDiscoveryOnce = async (reason = 'initial') => {
    const runVersion = ++discoveryVersion;
    const snapshot = [...extractedSites];
    const siteNodes = new Array(snapshot.length);
    const fullAllKeys = [];
    const prevSelectableKeys = allKeys.value.filter(isSelectableModelKey);
    const prevSelectableSet = new Set(prevSelectableKeys);
    const prevCheckedSelectableSet = new Set(
      checkedKeys.value.filter(key => prevSelectableSet.has(String(key)))
    );
    const prevAllSelected = prevSelectableKeys.length > 0 && prevCheckedSelectableSet.size === prevSelectableKeys.length;
    const discoveryLimit = 20;
    let currentIndex = 0;
    let noModelSiteCount = 0;
    const isInitialDiscovery = reason === 'initial' || !initialDiscoveryCompleted;
    const existingNodesBySiteName = new Map();
    if (!isInitialDiscovery && Array.isArray(treeData.value)) {
      treeData.value.forEach(node => {
        const name = String(node?.siteName || '').trim();
        if (!name) return;
        if (!existingNodesBySiteName.has(name)) existingNodesBySiteName.set(name, []);
        existingNodesBySiteName.get(name).push(node);
      });
    }

    isDiscoveringModels.value = true;
    loadedSitesCount.value = 0;

    snapshot.forEach((site, idx) => {
      const siteName = String(site?.site_name || `站点${idx + 1}`);
      if (isInitialDiscovery) {
        siteNodes[idx] = [
          withPendingMeta(siteName, {
            title: `${idx + 1}. [${siteName}] - 模型检测中...`,
            key: `discover-loading|${site?.id || idx}|${runVersion}`,
            disabled: true,
            selectable: false,
            isModelDiscovering: true,
            modelDiscoveringHint: '模型检测中',
            children: [],
          }),
        ];
      } else {
        const existing = existingNodesBySiteName.get(siteName);
        siteNodes[idx] = Array.isArray(existing) && existing.length
          ? existing.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }))
          : [];
      }
    });
    if (isInitialDiscovery) {
      treeData.value = siteNodes.flat().filter(Boolean);
    }

    const discoverOne = async (globalIdx) => {
      const site = extractedSites[globalIdx] || snapshot[globalIdx];
      const siteName = String(site?.site_name || `站点${globalIdx + 1}`);
      const siteDisplayTitle = `${globalIdx + 1}. [${siteName}]`;
      const currentSiteNodes = [];
      const existingSiteNodes = Array.isArray(siteNodes[globalIdx]) ? siteNodes[globalIdx] : [];
      const hasExistingModelNodes = existingSiteNodes.some(node => String(node?.key || '').startsWith('token|'));

      if (isSiteFailed(site)) {
        if (!isInitialDiscovery && hasExistingModelNodes) {
          console.log(`[FetchKeys] 模型刷新跳过: [${siteName}] 当前提取失败，保留上次成功模型节点`);
          return existingSiteNodes.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }));
        }
        const errorMsg = site?.error || '获取令牌失败';
        currentSiteNodes.push(withPendingMeta(siteName, {
          title: `${siteDisplayTitle} - ❌ ${errorMsg}`,
          key: `fail-site|${site?.id || globalIdx}`,
          disabled: true,
          selectable: false,
          isModelDiscovering: false,
          children: [],
        }));
        return currentSiteNodes;
      }

      const usableTokens = (site.tokens || []).filter(isUsableToken);
      if (usableTokens.length === 0) {
        if (!isInitialDiscovery && hasExistingModelNodes) {
          console.log(`[FetchKeys] 模型刷新跳过: [${siteName}] usableTokens=0，保留上次成功模型节点`);
          return existingSiteNodes.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }));
        }
        noModelSiteCount += 1;
        currentSiteNodes.push(withPendingMeta(siteName, {
          title: `${siteDisplayTitle} - ⏳ Token 已取到，但可用 Key 为 0（等待后台补全）`,
          key: `no-usable-token-site|${site.id || globalIdx}`,
          disabled: true,
          selectable: false,
          isModelDiscovering: false,
          children: [],
        }));
        return currentSiteNodes;
      }

      let effectiveBaseUrl = String(site.site_url || '').replace(/\/+$/, '');
      const rawApiKey = String(site.api_key || '').trim();
      if (rawApiKey.startsWith('http')) {
        effectiveBaseUrl = rawApiKey.replace(/\/+$/, '');
      }
      const endpointsToTry = [
        { url: `${effectiveBaseUrl}/v1/models`, type: 'openai' },
        { url: `${effectiveBaseUrl}/api/models`, type: 'newapi_public' },
        { url: `${effectiveBaseUrl}/api/user/models`, type: 'newapi_user' },
      ];

      let supportedModels = [];
      let discoveryReason = 'unknown';
      let tokenUsed = '';
      for (const token of usableTokens) {
        const tokenKey = String(token?.key || token?.access_token || '').trim();
        if (!tokenKey) continue;
        for (const ep of endpointsToTry) {
          try {
            const rawDiscoveryId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
            const discoveryUid = /^\d+$/.test(String(rawDiscoveryId)) ? String(rawDiscoveryId) : '';
            const res = await fetch(`/api/proxy-get?url=${encodeURIComponent(ep.url)}&uid=${discoveryUid}`, {
              headers: { Authorization: `Bearer ${tokenKey}` },
            });
            if (!res.ok) {
              discoveryReason = `http_${res.status}`;
              continue;
            }
            const result = await res.json();
            const rawData = Array.isArray(result)
              ? result
              : (result.data?.data || result.data?.items || result.data || []);
            if (Array.isArray(rawData) && rawData.length > 0) {
              supportedModels = rawData
                .map(m => (typeof m === 'string' ? m : (m.id || m.name || m)))
                .filter(m => typeof m === 'string')
                .sort();
              if (supportedModels.length > 0) {
                tokenUsed = tokenKey;
                discoveryReason = `ok_${ep.type}`;
                break;
              }
            } else {
              discoveryReason = 'empty_models';
            }
          } catch (e) {
            discoveryReason = `exception_${e?.message || 'unknown'}`;
          }
        }
        if (supportedModels.length > 0) break;
      }

      if (supportedModels.length === 0) {
        if (!isInitialDiscovery && hasExistingModelNodes) {
          console.log(`[FetchKeys] 模型刷新跳过: [${siteName}] 本轮模型探测为空，保留上次成功模型节点`);
          return existingSiteNodes.map(node => withPendingMeta(siteName, { ...node, isModelDiscovering: false }));
        }
        noModelSiteCount += 1;
        console.log(`[FetchKeys] 模型发现失败: [${siteName}] usableTokens=${usableTokens.length}, reason=${discoveryReason}`);
        currentSiteNodes.push(withPendingMeta(siteName, {
          title: `${siteDisplayTitle} - ⚠️ 未能探测到可用模型列表（usable=${usableTokens.length}, reason=${discoveryReason}）`,
          key: `no-model-site|${site.id || globalIdx}`,
          disabled: true,
          selectable: false,
          isModelDiscovering: false,
          children: [],
        }));
        return currentSiteNodes;
      }

      console.log(`[FetchKeys] 模型发现成功: [${siteName}] models=${supportedModels.length}, usableTokens=${usableTokens.length}, token=${tokenUsed.slice(0, 12)}...`);
      usableTokens.forEach((token, idx) => {
        const tokenKey = String(token.key || token.access_token || '').trim();
        if (!tokenKey) return;
        const tokenName = String(token.name || `Token ${idx + 1}`).trim();
        const tokenNodeKey = `token|${site.id}|${tokenKey}`;
        const children = supportedModels.map(model => {
          const itemKey = `${site.id}|${tokenKey}|${model}`;
          fullAllKeys.push(itemKey);
          return { title: model, key: itemKey, isLeaf: true };
        });
        fullAllKeys.push(tokenNodeKey);
        currentSiteNodes.push(withPendingMeta(siteName, {
          title: `${siteDisplayTitle} ${tokenName} (${tokenKey.slice(0, 15)}...)`,
          key: tokenNodeKey,
          isModelDiscovering: false,
          children,
        }));
      });

      return currentSiteNodes;
    };

    const worker = async () => {
      while (currentIndex < snapshot.length) {
        const idx = currentIndex++;
        if (runVersion !== discoveryVersion) return;
        const nodes = await discoverOne(idx);
        if (runVersion !== discoveryVersion) return;
        siteNodes[idx] = nodes;
        loadedSitesCount.value += 1;
        treeData.value = siteNodes.flat().filter(Boolean);
      }
    };

    await Promise.all(
      Array.from({ length: Math.min(discoveryLimit, Math.max(snapshot.length, 1)) }, () => worker())
    );

    if (runVersion !== discoveryVersion) return;

    treeData.value = siteNodes.flat().filter(Boolean);
    const nextSelectableKeys = fullAllKeys.filter(isSelectableModelKey);
    let nextCheckedKeys = [];
    if (!initialDiscoveryCompleted || prevAllSelected) {
      // 首次默认全选；若上次是“全选”，增量刷新后继续保持全选（避免新模型漏选）
      nextCheckedKeys = [...nextSelectableKeys];
    } else {
      // 保留用户已勾选项，同时清理不存在的脏 key（避免勾选状态残留）
      nextCheckedKeys = nextSelectableKeys.filter(key => prevCheckedSelectableSet.has(key));
    }
    allKeys.value = [...nextSelectableKeys];
    checkedKeys.value = [...new Set(nextCheckedKeys)];
    initialDiscoveryCompleted = true;
    isDiscoveringModels.value = false;

    const tokenSites = extractedSites.filter(site => !isSiteFailed(site)).length;
    const usableTokenSites = extractedSites.filter(site => countUsableTokensForSite(site) > 0).length;
    const selectableModelCount = fullAllKeys.filter(key => key.includes('|') && !key.startsWith('token|')).length;
    console.log(`[FetchKeys] 模型发现阶段完成(${reason}): tokenSites=${tokenSites}, usableTokenSites=${usableTokenSites}, noModelSites=${noModelSiteCount}, selectableModels=${selectableModelCount}`);
  };

  const requestDiscoveryRefresh = async (reason = 'unknown') => {
    if (discoveryInFlight) {
      discoveryQueued = true;
      discoveryQueuedReason = reason;
      return;
    }
    discoveryInFlight = true;
    try {
      let currentReason = reason;
      do {
        discoveryQueued = false;
        try {
          await runModelDiscoveryOnce(currentReason);
        } catch (err) {
          console.warn(`[FetchKeys] 模型发现异常(${currentReason}):`, err?.message || String(err));
          isDiscoveringModels.value = false;
          break;
        }
        currentReason = discoveryQueuedReason || 'queued-refresh';
      } while (discoveryQueued);
    } finally {
      discoveryInFlight = false;
    }
  };

  try {
    const BROWSER_FETCH_CONCURRENCY = 25;
    const browserResults = new Array(accountsToFetch.length);
    let currentIdx = 0;

    const browserFetchWorker = async () => {
      while (currentIdx < accountsToFetch.length) {
        const idx = currentIdx++;
        browserResults[idx] = await fetchTokensForAccountFromBrowserV2(accountsToFetch[idx]);
      }
    };

    await Promise.all(
      Array.from(
        { length: Math.min(BROWSER_FETCH_CONCURRENCY, Math.max(accountsToFetch.length, 1)) },
        () => browserFetchWorker()
      )
    );

    extractedSites = browserResults;

    const failedAccounts = accountsToFetch.filter((acc, i) => browserResults[i]?._needServerFallback === true);
    if (failedAccounts.length > 0) {
      console.log(`[FetchKeys] 浏览器端失败 ${failedAccounts.length} 个，尝试服务端代理兜底...`);
      try {
        const serverResponse = await fetch('/api/fetch-keys', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ accounts: failedAccounts }),
        });
        if (serverResponse.ok) {
          const serverData = await serverResponse.json();
          const serverResults = Array.isArray(serverData?.results) ? serverData.results : [];
          const mergeStats = mergeExtractedSiteResults(extractedSites, serverResults);
          console.log(`[FetchKeys] 服务端兜底合并: mergedSites=${mergeStats.mergedSites}, recoveredSites=${mergeStats.recoveredSites}, gainedTokens=${mergeStats.gainedTokens}, gainedUsableTokens=${mergeStats.gainedUsableTokens}`);
        }
      } catch (e) {
        console.warn('[FetchKeys] 服务端兜底失败:', e?.message || String(e));
      }
    }

    let stillFailedAccounts = extractedSites.filter(isSiteFailed);
    updateBrowserSessionPendingSites(stillFailedAccounts);
    validAccounts.value = extractedSites;
    preloadAllQuotas(extractedSites);

    summarizeStage('提取阶段完成', extractedSites, stillFailedAccounts);

    // 先展示结果页，再后台异步更新
    step.value = 2;
    isLoadingModels.value = false;
    void requestDiscoveryRefresh('initial');

    if (stillFailedAccounts.length > 0) {
      void (async () => {
        try {
          const detected = await getDetectedFallbackBrowser();
          const browserType = await chooseDetectedFallbackBrowserType(detected);
          if (!browserType) {
            message.warning('你取消了浏览器选择，当前保留已提取结果并继续后续流程。');
            return;
          }

          let openedCount = 0;
          try {
            openedCount = await openSitesInBrowserSession(stillFailedAccounts, browserType);
          } catch (openErr) {
            const fallbackStatus = await getFallbackBrowserStatus(browserType).catch(() => ({
              running: false,
              attached: false,
              launching: false,
              managed: false,
              browserType,
            }));
            const shouldHandleAsProfileInUse =
              openErr?.code === 'BROWSER_PROFILE_IN_USE' ||
              (fallbackStatus.running && !fallbackStatus.attached && !fallbackStatus.launching && !fallbackStatus.managed);

            if (shouldHandleAsProfileInUse) {
              const shouldKill = await confirmWithModal({
                title: '浏览器已占用',
                content: `${browserType === 'edge' ? 'Edge' : 'Chrome'} 当前已在普通模式运行，默认 profile 被占用。结束后会关闭该浏览器所有窗口，是否继续？`,
                okText: '结束并继续',
                cancelText: '取消',
                okType: 'danger',
              });
              if (!shouldKill) {
                message.warning('你取消了结束浏览器进程，当前保留已有结果并继续后续流程。');
                return;
              }

              const restartResult = await restartBrowserSessionProcessAndOpen(stillFailedAccounts, browserType);
              if (!restartResult?.stopped) {
                message.error(`${browserType === 'edge' ? 'Edge' : 'Chrome'} 进程结束后仍未完全退出，请手动关闭后重试。`);
                return;
              }
              openedCount = Number(restartResult?.opened || stillFailedAccounts.length);
            } else {
              throw openErr;
            }
          }

          if (openedCount <= 0) return;

          const availableText = detected.availableTypes.map(type => (type === 'edge' ? 'Edge' : 'Chrome')).join(' / ');
          message.info(`已探测到 ${availableText}，当前使用 ${browserType === 'edge' ? 'Edge' : 'Chrome'} 打开 ${openedCount} 个失败站点，后台自动轮询抓取中。`, 6);

          const maxRetryRounds = 3;
          const retryIntervalMs = 15000;
          browserSessionPolling.active = true;
          browserSessionPolling.totalRounds = maxRetryRounds;
          browserSessionPolling.pending = stillFailedAccounts.length;
          updateBrowserSessionPendingSites(stillFailedAccounts);
          refreshTreePendingHints();

          try {
            for (let round = 1; round <= maxRetryRounds && stillFailedAccounts.length > 0; round += 1) {
              browserSessionPolling.round = round;
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              refreshTreePendingHints();
              console.log(`[FetchKeys] 受控浏览器自动抓取: ${browserType} round=${round}/${maxRetryRounds}, pendingSites=${stillFailedAccounts.length}`);

              const browserSessionResults = await browserSessionFetchForAccounts(stillFailedAccounts, browserType, round, maxRetryRounds);
              const mergeStats = mergeExtractedSiteResults(extractedSites, browserSessionResults);
              validAccounts.value = extractedSites;
              preloadAllQuotas(extractedSites);

              stillFailedAccounts = extractedSites.filter(isSiteFailed);
              browserSessionPolling.pending = stillFailedAccounts.length;
              updateBrowserSessionPendingSites(stillFailedAccounts);
              refreshTreePendingHints();
              summarizeStage(`受控浏览器第 ${round} 轮`, extractedSites, stillFailedAccounts);
              console.log(`[FetchKeys] 受控浏览器第 ${round} 轮合并: mergedSites=${mergeStats.mergedSites}, recoveredSites=${mergeStats.recoveredSites}, gainedTokens=${mergeStats.gainedTokens}, gainedUsableTokens=${mergeStats.gainedUsableTokens}`);

              if (mergeStats.gainedUsableTokens > 0) {
                void requestDiscoveryRefresh(`browser-round-${round}`);
              }

              if (stillFailedAccounts.length === 0) break;
              if (round < maxRetryRounds) {
                await sleep(retryIntervalMs);
              }
            }
          } finally {
            browserSessionPolling.active = false;
            browserSessionPolling.round = 0;
            browserSessionPolling.totalRounds = 0;
            browserSessionPolling.pending = 0;
            browserSessionPendingSiteNames.value = [];
            refreshTreePendingHints();
          }

          if (stillFailedAccounts.length > 0) {
            message.warning(`受控浏览器自动轮询完成，仍有 ${stillFailedAccounts.length} 个站点未抓取成功。`);
          }
          summarizeStage('受控浏览器轮询结束', extractedSites, stillFailedAccounts);
          void requestDiscoveryRefresh('browser-polling-finished');
        } catch (e) {
          console.warn('[FetchKeys] 受控浏览器兜底失败:', e?.message || String(e));
          message.warning(`失败站点受控浏览器兜底未执行成功: ${e?.message || String(e)}`);
        }
      })();
    }
  } catch (err) {
    message.error(`批量提取 Token 失败: ${err?.message || String(err)}`);
    isLoadingModels.value = false;
    isDiscoveringModels.value = false;
    step.value = 1;
  }
};

// --- Tree Actions ---
const selectAllNodes = () => {
  checkedKeys.value = allKeys.value.filter(isSelectableModelKey);
};

const unselectAllNodes = () => {
  checkedKeys.value = [];
};

const selectChatModelsOnly = () => {
  const notChatPattern = /(bge|stabilityai|dall|mj|stable|flux|video|midjourney|stable-diffusion|playground|swap_face|tts|whisper|text|emb|luma|vidu|pdf|suno|pika|chirp|domo|runway|cogvideo|babbage|davinci|gpt-4o-realtime)/i;
  
  const filteredKeys = [];
  const childKeys = allKeys.value.filter(isSelectableModelKey);
  childKeys.forEach(k => {
    const parts = k.split('|');
    const model = parts[2]; 
    if (!notChatPattern.test(model) && !/(image|audio|video|music|pdf|flux|suno|embed)/i.test(model)) {
      filteredKeys.push(k);
    }
  });
  
  checkedKeys.value = filteredKeys;
};

// --- Testing Logic ---
const startBatchCheck = async () => {
  // Extract selected tasks
  const selectedModelKeys = checkedKeys.value.filter(k =>
    k.includes('|') &&
    !k.startsWith('token|') &&
    !k.startsWith('fail-site|') &&
    !k.startsWith('no-model-site|') &&
    !k.startsWith('no-usable-token-site|') &&
    !k.startsWith('discover-loading|')
  );
  if (selectedModelKeys.length === 0) {
    message.warning('请至少勾选一个模型进行测试');
    return;
  }

  step.value = 3;
  testing.value = true;
  cancelTokens.value = [];
  testResults.value = [];
  organizedSourceResults.value = [];
  
  // Build task queue
  const tasksQueue = [];
  selectedModelKeys.forEach((keyStr, idx) => {
    // 格式: siteId|tokenKey|modelName
    const parts = keyStr.split('|');
    if (parts.length < 3) return; // 忽略不符合新格式的
    
    const [siteId, tokenKey, modelName] = parts;
    const site = validAccounts.value.find(s => s.id === siteId);
    
    if (site) {
      // 增强逻辑：对 api_key 进行清洗，优先从中提取 API 基址
      let effectiveUrl = site.site_url;
      const rawApiKey = String(site.api_key || '').trim();
      if (rawApiKey.startsWith('http')) {
        effectiveUrl = rawApiKey;
      }
      
      const task = {
        id: `task_${idx}`,
        siteId,
        siteName: site.site_name,
        siteUrl: effectiveUrl,
        apiKey: tokenKey, // <--- 使用真正的 sk- 密钥!
        modelName: modelName,
        status: 'pending',
        statusText: '排队中',
        responseTime: '-',
        remark: '-',
        accountData: site, // 仅做记录
      };
      tasksQueue.push(task);
      testResults.value.push(task);
    }
  });


  totalTasks.value = tasksQueue.length;
  completedTasks.value = 0;
  scheduleOrganizedSourceRefresh(true);
  console.log(`[BatchCheck] 开始检测: selectedModelKeys=${selectedModelKeys.length}, queuedTasks=${tasksQueue.length}`);

  // Concurrency executor
  let currentIndex = 0;
  
  const worker = async () => {
    while (currentIndex < tasksQueue.length && testing.value) {
      const taskIndex = currentIndex++;
      const task = tasksQueue[taskIndex];
      task.status = 'testing';
      task.statusText = '测试中';
      
      await runSingleTest(task);
      
      completedTasks.value++;
      scheduleOrganizedSourceRefresh();
    }
  };

  const workers = [];
  const actualConcurrency = Math.min(batchConcurrency.value, tasksQueue.length);
  for (let i = 0; i < actualConcurrency; i++) {
    workers.push(worker());
  }

  await Promise.all(workers);
  
  if (testing.value) {
    testing.value = false;
    scheduleOrganizedSourceRefresh(true);
    message.success('批量检测完成！');
    // Save to history
    localStorage.setItem('api_check_last_results', JSON.stringify(testResults.value));
    hasHistory.value = true;
  }
};

const stopTesting = () => {
  testing.value = false;
  // Trigger abort on controllers
  cancelTokens.value.forEach(controller => controller.abort());
  message.info('已停止检测');
};

const runSingleTest = async (task, customPayload = null) => {
  const apiUrlValue = customPayload ? customPayload.url.replace(/\/+$/, '') : task.siteUrl.replace(/\/+$/, '');
  const modelToTest = customPayload ? customPayload.model : task.modelName;
  const keyToUse = customPayload ? customPayload.key : task.apiKey;
  const messagesToUse = customPayload ? customPayload.messages : [{ role: 'user', content: 'hello' }];

  let timeout = modelTimeout.value * 1000;
  if (modelToTest.startsWith('o1-')) {
    timeout *= 6;
  }

  const controller = new AbortController();
  cancelTokens.value.push(controller);
  
  const id = setTimeout(() => controller.abort(), timeout + 2000); // 宽延2秒
  const startTime = Date.now();

  try {
    const isFirst = task.id === 'task_0';
    const payloadBody = {
      url: apiUrlValue,
      key: keyToUse,
      model: modelToTest,
      messages: messagesToUse,
      _isFirst: isFirst
    };
    
    // 如果是编辑模式重试，同步更新一下任务的属性以便UI显示最新值 (可选，看是否需要覆盖原来的)
    if (customPayload) {
      task.modelName = modelToTest;
      task.apiKey = keyToUse;
      task.siteUrl = customPayload.url;
    }

    const response = await fetch('/api/check-key', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payloadBody),
      signal: controller.signal,
    });

    const endTime = Date.now();
    const responseTime = ((endTime - startTime) / 1000).toFixed(2);
    task.responseTime = responseTime;

    if (response.ok) {
      let data = await response.json();
      
      // 有些接口返回不是标准的 JSON 格式，可能带有 htmlSnippet。
      // 我们尝试从中深度提取 JSON，增强解析鲁棒性 (处理 SSE 格式的 data: 前缀)
      if (data && data.htmlSnippet) {
        let snippet = String(data.htmlSnippet).trim();
        if (snippet.startsWith('data:')) {
          snippet = snippet.replace(/^data:\s*/, '').trim();
        }
        if (snippet.startsWith('{') || snippet.startsWith('[')) {
          try { data = JSON.parse(snippet); } catch (e) {}
        }
      }

      const returnedModel = data.model || 'unknown';
      const msgObj = data.choices && data.choices[0]?.message;
      
      // 增强兼容性判定：思维链模型可能使用 reasoning_content
      const hasContent = msgObj && (msgObj.content || msgObj.reasoning_content || msgObj.thinking);
      const isReasoning = msgObj && (msgObj.reasoning_content || msgObj.thinking);
      const isStreamAssembled = data.isStreamAssembled;

      let suffixHtml = '';
      let suffixPlain = '';
      if (isReasoning) {
        suffixHtml = ' <span style="color:#52c41a; font-weight:500; font-size:12px;">(thinking)</span>';
        suffixPlain = ' (thinking)';
      } else if (isStreamAssembled) {
        suffixHtml = ' <span style="color:#52c41a; font-weight:500; font-size:12px;">(strict SSE)</span>';
        suffixPlain = ' (strict SSE)';
      }
      
      task.modelSuffix = suffixPlain;
      task.displaySuffixHtml = suffixHtml;
      
      // 保存原始响应
      task.fullResponse = JSON.stringify(data, null, 2);

      if (returnedModel.toLowerCase().includes(task.modelName.toLowerCase()) || task.modelName === 'unknown') {
        task.status = 'success';
        task.statusText = '一致可用';
        task.remark = hasContent ? (msgObj?.content ? '通过' : '思维链模型通过') : '响应成功结构异常';
        if (!hasContent) {
           task.status = 'warning';
        }
      } else {
        task.status = 'warning';
        if (returnedModel === 'unknown') {
          task.statusText = '模型未知';
          task.remark = hasContent ? '✅ 响应成功但未返回模型标识' : '❌ 响应为空且模型未知';
          if (!hasContent) task.status = 'error';
        } else {
          task.statusText = '模型重定向';
          task.remark = `映射由平台处理 -> ${returnedModel}`;
        }
      }
    } else {
      let errText = '';
      let rawData = null;
      try {
        const contentType = response.headers.get('content-type') || '';
        if (contentType.includes('application/json')) {
           rawData = await response.json();
        } else {
           const text = await response.text();
           const titleMatch = text.match(/<title>(.*?)<\/title>/i);
           rawData = { 
             htmlTitle: titleMatch ? titleMatch[1] : 'HTML Payload',
             htmlSnippet: text.substring(0, 500).replace(/<[^>]*>/g, ' ').trim()
           };
        }

        if (rawData.htmlTitle) {
          errText = `(HTML) ${rawData.htmlTitle}`;
        } else {
          errText = toReadableError(rawData, '请求失败');
        }
        task.fullResponse = rawData.htmlSnippet
          ? `HTML 内容摘要: ${rawData.htmlSnippet}\n\n完整响应: ${JSON.stringify(rawData, null, 2)}`
          : JSON.stringify(rawData, null, 2);
      } catch (e) {
        errText = `HTTP ${response.status}`;
        task.fullResponse = `Error: ${errText}`;
      }
      task.status = 'error';
      task.statusText = toStatusTextByError(errText);
      task.remark = truncateText(errText, 200);
    }
  } catch (err) {
    task.status = 'error';
    task.statusText = toStatusTextByError(err?.message || '');
    if (err.name === 'AbortError') {
      task.remark = '请求超时';
    } else {
      task.remark = truncateText(err.message, 200);
    }
  } finally {
    clearTimeout(id);
    const cIdx = cancelTokens.value.indexOf(controller);
    if (cIdx > -1) cancelTokens.value.splice(cIdx, 1);
    scheduleOrganizedSourceRefresh();
  }
};


const getStatusColor = (status) => {
  switch (status) {
    case 'success': return 'green';
    case 'warning': return 'orange';
    case 'error': return 'red';
    case 'testing': return 'blue';
    case 'pending': return 'default';
    default: return 'default';
  }
};

const copyAllConfigs = () => {
  const validTasks = testResults.value.filter(t => t.status === 'success' || t.status === 'warning');
  if (validTasks.length === 0) {
    message.warning('没有可用的配置组合！');
    return;
  }
  
  const siteMap = new Map();
  validTasks.forEach(task => {
    const key = `${task.siteUrl}|${task.apiKey}`;
    if (!siteMap.has(key)) {
      siteMap.set(key, { name: task.siteName, url: task.siteUrl, key: task.apiKey, models: [] });
    }
    siteMap.get(key).models.push(task.modelName);
  });
  
  const text = Array.from(siteMap.values()).map(s => 
    `====================\n平台名称: ${s.name}\n接口地址: ${s.url}\nAPI 密钥: ${s.key}\n可用模型: ${s.models.join(',')}\n`
  ).join('\n');

  navigator.clipboard.writeText(text).then(() => {
    message.success(`已复制全表 ${siteMap.size} 个站点的有效配置`);
  });
};

const copyOrganizedResults = () => {
  const tree = organizedTreeData.value;
  if (tree.length === 0) {
    message.warning('当前视图没有可复制的配置');
    return;
  }

  const text = tree.map(group => {
    const validModels = group.children
      .filter(c => c.class === 'status-success' || c.class === 'status-warning')
      .map(c => c.title.split(' - ')[0]);
    
    if (validModels.length === 0) return null;

    const [siteName, apiKeyTail] = group.key.split('|'); 
    // Find the original full task to get the correct site URL
    const originalTask = testResults.value.find(t => t.siteName === siteName && t.apiKey === apiKeyTail);
    const url = originalTask ? originalTask.siteUrl : 'unknown';

    return `====================\n平台名称: ${siteName}\n接口地址: ${url}\nAPI 密钥: ${apiKeyTail}\n可用模型: ${validModels.join(',')}\n`;
  }).filter(t => t).join('\n');

  if (!text) {
    message.warning('当前筛选出的站点中没有有效的模型配置');
    return;
  }

  navigator.clipboard.writeText(text).then(() => {
    message.success(`已按当前过滤视图复制配置信息`);
  });
};

</script>

<style scoped>
/* Header & Navigation Style */
.header {
  display: flex !important;
  flex-direction: row !important;
  flex-wrap: nowrap !important;
  justify-content: flex-end !important;
  align-items: center;
  margin-bottom: 30px;
  padding: 10px 20px;
}

#themeToggle {
  margin-right: 25px; /* 增加与右侧图标组的间距 */
  flex-shrink: 0;
}

.right-icons {
  display: flex;
  flex-wrap: nowrap !important;
  gap: 22px; /* 进一步增加图标间距 */
  align-items: center;
  flex-shrink: 0;
}

.icon-button {
  font-size: 24px; /* 进一步放大图标至 24px */
  color: #666;
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 8px;
  cursor: pointer;
}

.dark-mode .icon-button {
  color: #aaa;
}

.icon-button:hover {
  color: #1677ff;
  transform: scale(1.15);
}

.dark-mode .icon-button:hover {
  color: #40a9ff;
}

.batch-wrapper {
  min-height: 100vh;
  padding: 0;
}
/* 覆盖 global.css 里 .container 的 max-width: 800px 限制 */
.container {
  max-width: 100% !important;
  padding: 20px !important;
  margin: 0 !important;
}
.page-content {
  background-color: var(--container-bg);
  border-radius: 0;
  box-shadow: var(--shadow-color);
  padding: 20px;
  min-height: 100vh;
}

.step-container {
  margin-top: 20px;
}
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 50px 0;
}
.tree-wrapper {
  background: var(--input-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 10px;
  margin-bottom: 20px;
  overflow-y: auto;
}
.settings-action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  border-top: 1px solid var(--border-color);
  padding-top: 15px;
}
.result-container {
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 15px;
  background-color: var(--input-bg);
}

/* Organized Tree Styles */
.organized-tree-wrapper {
  background: var(--container-bg);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 10px;
  max-height: 500px;
  overflow-y: auto;
}

.custom-tree-node {
  font-size: 14px;
}

.tree-node-green { color: #52c41a; font-weight: bold; }
.tree-node-orange { color: #faad14; font-weight: bold; }
.tree-node-grey { color: #999; opacity: 0.7; }
.tree-node-pending-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: 10px;
  color: #1677ff;
  font-size: 12px;
}

.status-success { color: #52c41a; }
.status-warning { color: #faad14; }
.status-error { color: #ff4d4f; }

:deep(.result-summary-tree .ant-tree-node-content-wrapper) {
  width: 100%;
}

:deep(.highlighted-row) {
  background-color: rgba(24, 144, 255, 0.15) !important;
  transition: background-color 0.5s;
}

:deep(.dark-mode .highlighted-row) {
  background-color: rgba(24, 144, 255, 0.3) !important;
}
.custom-tree-node-wrapper {
  display: flex !important;
  align-items: center;
  width: 100%;
}

.shortcut-actions {
  opacity: 0.1;
  transition: opacity 0.3s ease;
}

.custom-tree-node-wrapper:hover .shortcut-actions {
  opacity: 1;
}

.app-icon {
  cursor: pointer;
  font-size: 14px;
  filter: grayscale(0.8);
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.app-icon:hover {
  filter: grayscale(0);
  transform: scale(1.3);
}

.cherry-icon:hover {
  text-shadow: 0 0 8px rgba(255, 0, 0, 0.4);
}

.switch-icon:hover {
  text-shadow: 0 0 8px rgba(0, 123, 255, 0.4);
}
</style>
