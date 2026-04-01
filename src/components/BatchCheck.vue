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
            <div v-show="isLoadingModels" class="step-container loading-container">
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

              <div class="tree-wrapper">
                <a-tree
                  v-model:checkedKeys="checkedKeys"
                  :tree-data="treeData"
                  checkable
                  defaultExpandAll
                  height="400"
                />
              </div>

              <div class="settings-action-bar">
                <div class="batch-settings">
                  <span style="font-size: 14px; margin-right: 10px;">并发数：</span>
                  <a-input-number v-model:value="batchConcurrency" :min="1" :max="50" />
                  <span style="font-size: 14px; margin-left: 20px; margin-right: 10px;">超时(秒)：</span>
                  <a-input-number v-model:value="modelTimeout" :min="1" />
                </div>
                <div class="actions">
                  <a-button @click="resetStep1" style="margin-right: 10px;">重新导入</a-button>
                  <a-button type="primary" size="large" @click="startBatchCheck">
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
                    <a-tooltip :title="record.fullResponse || '无原始响应数据'" placement="topLeft">
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
import { ConfigProvider, message, theme } from 'ant-design-vue';
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

// 按 siteUrl 缓存余额，确保其为响应式对象
const siteQuotaCache = reactive({});

const batchConcurrency = ref(20);
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

// Search & Filter State (Default no filter, no memory)
const searchQuery = ref('');
const filterOnlySuccess = ref(false);

const testProgress = computed(() => {
  if (totalTasks.value === 0) return 0;
  return Math.floor((completedTasks.value / totalTasks.value) * 100);
});

// --- NEW Core Computed: Organized & Filtered Tree Data ---
const organizedTreeData = computed(() => {
  const results = testResults.value;
  if (results.length === 0) return [];

  const keywords = searchQuery.value.trim().toLowerCase().split(/\s+/).filter(k => k);
  
  // Grouping
  const groups = new Map();
  results.forEach(task => {
    // Keyword match: site name or model name matches ANY of symbols
    const matchSearch = keywords.length === 0 || keywords.some(k => 
      task.siteName.toLowerCase().includes(k) || 
      task.modelName.toLowerCase().includes(k)
    );
    
    // Status match
    const isError = task.status === 'error';
    if (filterOnlySuccess.value && isError) return;
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

    return {
      title: `[${g.siteName}] ${g.apiKey.slice(0, 15)}...${quotaStr}`,
      key: `${g.siteName}|${g.apiKey}`,
      class: titleClass,
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

const currentResultData = computed(() => testResults.value);

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
};

const resetStep2 = () => {
  step.value = 2;
  testResults.value = [];
  completedTasks.value = 0;
  totalTasks.value = 0;
};

// --- Upload and Parse ---
const beforeUpload = (file) => {
  const reader = new FileReader();
  reader.onload = (e) => {
    try {
      const data = JSON.parse(e.target.result);
      if (data && data.accounts && Array.isArray(data.accounts.accounts)) {
        processAccounts(data.accounts.accounts);
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
  
  totalAccountsCount.value = accountsToFetch.length;
  isLoadingModels.value = true;
  step.value = -1; // 显示提取中的中间状态
  loadedSitesCount.value = 0;
  
  // ── 第 1 步：后端并发提取 ──
  let extractedSites = [];
  try {
    const response = await fetch('/api/fetch-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ accounts: accountsToFetch }),
    });

    if (!response.ok) {
      throw new Error((await response.json()).message || '提取过程出错');
    }

    const data = await response.json();
    extractedSites = data.results || [];
    validAccounts.value = extractedSites; // 提前设置，这样 UI 统计分母正常 (已加载/总提取数)
    
    // ── 第 1.5 步：后台提前刷额度 ──
    preloadAllQuotas(extractedSites);
  } catch (err) {
    message.error(`批量提取 Token 失败: ${err.message}`);
    isLoadingModels.value = false;
    step.value = 1;
    return;
  }

  const siteTrees = [];
  const fullCheckedKeys = [];
  const fullAllKeys = [];

  // ── 第 2 步：并发探测模型 ──
  const discoveryLimit = 30;
  let currentIndex = 0;

  const discoverWorker = async () => {
    while (currentIndex < extractedSites.length) {
      const site = extractedSites[currentIndex++];
      if (!site || site.error || !site.tokens || site.tokens.length === 0) {
        loadedSitesCount.value++;
        continue;
      }

      const baseUrl = site.site_url.replace(/\/+$/, '');
      // 使用提取出的第一个可用 Token 进行模型探测
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
          // 经 scripts/verify-uid.cjs 验证通过：模型查验也带上真实的数字 UID
          const rawDiscoveryId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
          const discoveryUid = /^\d+$/.test(String(rawDiscoveryId)) ? String(rawDiscoveryId) : '';
          
          const res = await fetch(`/api/proxy-get?url=${encodeURIComponent(ep.url)}&uid=${discoveryUid}`, {
            method: 'GET',
            headers: { Authorization: `Bearer ${testApiKey}` }
          });
          if (res.ok) {
            const contentType = res.headers.get('content-type') || '';
            if (contentType.includes('application/json')) {
              const result = await res.json();
              if (ep.type === 'newapi_user' && Array.isArray(result.data)) {
                supportedModels = (typeof result.data[0] === 'string' ? result.data : result.data.map(m => m.id)).sort();
              } else if (result.data && Array.isArray(result.data)) {
                supportedModels = result.data.map(m => m.id || m.name || m).filter(m => typeof m === 'string').sort();
              }
              if (supportedModels.length > 0) break;
            }
          }
        } catch (e) {
          console.warn(`Discovery fail: ${site.site_name} ${ep.url}`, e);
        }
      }

      // 如果探测失败，提供基础模型
      if (supportedModels.length === 0) {
        supportedModels = ['gpt-3.5-turbo', 'gpt-4o', 'gpt-4o-mini', 'claude-3-5-sonnet-20240620', 'gemini-1.5-flash-latest'];
      }

      // 按 Site -> Token -> Models 构建树
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
        siteTrees.push({
          title: `[${site.site_name}] ${tName} (${tKey.slice(0, 15)}...)`,
          key: tokenNodeKey,
          children: children,
        });
      });

      loadedSitesCount.value++;
    }
  };

  const discoveryWorkers = [];
  for (let i = 0; i < Math.min(discoveryLimit, extractedSites.length); i++) {
    discoveryWorkers.push(discoverWorker());
  }
  await Promise.all(discoveryWorkers);

  treeData.value = siteTrees;
  allKeys.value = fullAllKeys;
  checkedKeys.value = fullCheckedKeys; // 默认全选
  
  isLoadingModels.value = false;
  step.value = 2; // 进入树形选择器
  
  validAccounts.value = extractedSites; 
};

// --- Tree Actions ---
const selectAllNodes = () => {
  checkedKeys.value = [...allKeys.value];
};

const unselectAllNodes = () => {
  checkedKeys.value = [];
};

const selectChatModelsOnly = () => {
  const notChatPattern = /(bge|stabilityai|dall|mj|stable|flux|video|midjourney|stable-diffusion|playground|swap_face|tts|whisper|text|emb|luma|vidu|pdf|suno|pika|chirp|domo|runway|cogvideo|babbage|davinci|gpt-4o-realtime)/i;
  
  const filteredKeys = [];
  const childKeys = allKeys.value.filter(k => k.includes('|'));
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
  const selectedModelKeys = checkedKeys.value.filter(k => k.includes('|'));
  if (selectedModelKeys.length === 0) {
    message.warning('请至少勾选一个模型进行测试');
    return;
  }

  step.value = 3;
  testing.value = true;
  cancelTokens.value = [];
  testResults.value = [];
  
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
      const isStrictSSE = data.isStreamHack;

      let suffixHtml = '';
      let suffixPlain = '';
      if (isReasoning) {
        suffixHtml = ' <span style="color:#52c41a; font-weight:500; font-size:12px;">(thinking)</span>';
        suffixPlain = ' (thinking)';
      } else if (isStrictSSE) {
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
          errText = rawData.error?.message || rawData.message || '请求失败';
        }
        task.fullResponse = rawData.htmlSnippet ? `HTML 内容摘要: ${rawData.htmlSnippet}\n\n完整响应: ${JSON.stringify(rawData, null, 2)}` : JSON.stringify(rawData, null, 2);
      } catch (e) {
        errText = `HTTP ${response.status}`;
        task.fullResponse = `Error: ${errText}`;
      }
      task.status = 'error';
      task.statusText = '调用失败';
      task.remark = errText;
    }
  } catch (err) {
    task.status = 'error';
    task.statusText = '调用失败';
    if (err.name === 'AbortError') {
      task.remark = '请求超时';
    } else {
      task.remark = err.message;
    }
  } finally {
    clearTimeout(id);
    const cIdx = cancelTokens.value.indexOf(controller);
    if (cIdx > -1) cancelTokens.value.splice(cIdx, 1);
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
