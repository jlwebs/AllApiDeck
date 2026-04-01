<template>
  <ConfigProvider :theme="configProviderTheme">
    <div class="wrapper batch-wrapper">
      <a-flex :direction="'vertical'" :justify="'center'" :align="'center'">
        <div class="page-content" style="max-width: 1000px; width: 100%">
          <div class="container">
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
                <a-tooltip :title="'返回常规检测'" placement="bottom">
                  <a @click="$router.push('/')" class="icon-button">
                    <HomeOutlined style="cursor: pointer" />
                  </a>
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
            </div>

            <!-- 加载状态 -->
            <div v-show="isLoadingModels" class="step-container loading-container">
              <a-spin size="large" />
              <p style="margin-top: 20px;">正在并发获取各大站点的模型列表，请稍候... ({{ loadedSitesCount }} / {{ validAccounts.length }})</p>
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
                <h3 style="margin: 0;">批量检测结果</h3>
                <a-space>
                  <a-button type="primary" ghost @click="copyValidConfigs" :disabled="testing">复制可用配置</a-button>
                  <a-button danger v-if="testing" @click="stopTesting">停止检测</a-button>
                  <a-button v-else @click="resetStep2">返回选择面板</a-button>
                </a-space>
              </div>

              <a-progress :percent="testProgress" show-info style="margin-bottom: 15px" />

              <a-table
                :columns="resultColumns"
                :data-source="currentResultData"
                :pagination="{ pageSize: 15 }"
                size="small"
                row-key="id"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.dataIndex === 'status'">
                    <a-tag :color="getStatusColor(record.status)">
                      {{ record.statusText }}
                    </a-tag>
                  </template>
                  <template v-else-if="column.dataIndex === 'remark'">
                    <span :style="{ color: record.status === 'error' ? 'red' : 'inherit' }">
                      {{ record.remark }}
                    </span>
                  </template>
                </template>
              </a-table>
            </div>

          </div>
        </div>
      </a-flex>
    </div>
  </ConfigProvider>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { ConfigProvider, message, theme } from 'ant-design-vue';
import { HomeOutlined, InboxOutlined, PlayCircleOutlined } from '@ant-design/icons-vue';
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

const validAccounts = ref([]);
const treeData = ref([]);
const checkedKeys = ref([]);
const allKeys = ref([]); // Store all keys for easy 'Select All'

const loadedSitesCount = ref(0);

const batchConcurrency = ref(10);
const modelTimeout = ref(15);

const testing = ref(false);
const cancelTokens = ref([]); // to allow stopping
const testResults = ref([]); // all tasks
const totalTasks = ref(0);
const completedTasks = ref(0);

const testProgress = computed(() => {
  if (totalTasks.value === 0) return 0;
  return Math.floor((completedTasks.value / totalTasks.value) * 100);
});

const currentResultData = computed(() => testResults.value);

const resultColumns = [
  { title: '平台名称', dataIndex: 'siteName', width: 120 },
  { title: '模型名称', dataIndex: 'modelName', width: 150 },
  { title: '状态', dataIndex: 'status', width: 100 },
  { title: '响应(s)', dataIndex: 'responseTime', width: 80 },
  { title: '备注信息', dataIndex: 'remark', ellipsis: true },
];

onMounted(() => {
  isDarkMode.value = document.body.classList.contains('dark-mode');
});

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
  // Filter disabled and invalid accounts
  const accountsToTest = accounts.filter(acc => 
    !acc.disabled && 
    acc.site_url && 
    acc.account_info && 
    acc.account_info.access_token
  );
  
  if (accountsToTest.length === 0) {
    message.warning('备份文件中未找到可用账号配置！');
    return;
  }
  
  validAccounts.value = accountsToTest;
  isLoadingModels.value = true;
  step.value = -1; // hide step 1
  loadedSitesCount.value = 0;
  
  const siteTrees = [];
  const fullCheckedKeys = [];
  const fullAllKeys = [];

  // Concurrently fetch /v1/models with simple mapping
  const fetchPromises = accountsToTest.map(async (acc) => {
    try {
      const response = await fetchModelList(acc.site_url, acc.account_info.access_token);
      let supportedModels = [];
      if (response && response.data) {
        supportedModels = [...new Set(response.data.map(m => m.id))].sort();
      }
      return { 
        ...acc, 
        models: supportedModels 
      };
    } catch (error) {
      return { 
        ...acc, 
        models: [], 
        error: error.message 
      };
    } finally {
      loadedSitesCount.value++;
    }
  });

  const parsedResults = await Promise.all(fetchPromises);
  
  parsedResults.forEach((accResult) => {
    if (accResult.models.length > 0) {
      const siteNodeKey = `site_${accResult.id}`;
      fullAllKeys.push(siteNodeKey);
      fullCheckedKeys.push(siteNodeKey);
      
      const children = accResult.models.map(model => {
        const itemKey = `${accResult.id}|${model}`;
        fullAllKeys.push(itemKey);
        fullCheckedKeys.push(itemKey);
        return {
          title: model,
          key: itemKey,
          isLeaf: true
        };
      });
      
      siteTrees.push({
        title: `${accResult.site_name} (${accResult.models.length} 个模型) - ${accResult.site_url}`,
        key: siteNodeKey,
        children: children,
      });
    } else {
      // No models found or error
      const siteNodeKey = `site_${accResult.id}`;
      siteTrees.push({
        title: `${accResult.site_name} (获取模型失败或空) - ${accResult.site_url}`,
        key: siteNodeKey,
        disabled: true,
        children: [],
      });
    }
  });

  treeData.value = siteTrees;
  allKeys.value = fullAllKeys;
  checkedKeys.value = fullCheckedKeys; // Default select all
  
  isLoadingModels.value = false;
  step.value = 2; // Show step 2
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
  
  // Go through all keys
  // site node is added if at least one child is matched (optional, antd handles half-checked)
  const childKeys = allKeys.value.filter(k => k.includes('|'));
  childKeys.forEach(k => {
    const parts = k.split('|');
    const model = parts[1];
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
    const [accId, modelName] = keyStr.split('|');
    const account = validAccounts.value.find(a => a.id === accId);
    if (account) {
      const task = {
        id: `task_${idx}`,
        accId,
        siteName: account.site_name,
        siteUrl: account.site_url,
        apiKey: account.account_info.access_token,
        modelName: modelName,
        status: 'pending',
        statusText: '排队中',
        responseTime: '-',
        remark: '-',
        accountData: account, // keep reference for export later
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
  }
};

const stopTesting = () => {
  testing.value = false;
  // Trigger abort on controllers
  cancelTokens.value.forEach(controller => controller.abort());
  message.info('已停止检测');
};

const runSingleTest = async (task) => {
  const apiUrlValue = task.siteUrl.replace(/\/+$/, '');
  let timeout = modelTimeout.value * 1000;
  if (task.modelName.startsWith('o1-')) {
    timeout *= 6;
  }

  const controller = new AbortController();
  cancelTokens.value.push(controller);
  
  const id = setTimeout(() => controller.abort(), timeout);
  const startTime = Date.now();

  try {
    const requestBody = {
      model: task.modelName,
      messages: [{ role: 'user', content: 'hello' }],
    };
    if (/^(gpt-|chatgpt-)/.test(task.modelName)) {
      requestBody.seed = 331;
    }
    
    // In actual implementation, we'd reuse normal api.js logic, but since we are doing custom
    // tracking for a table row, we fetch directly here.
    const response = await fetch(`${apiUrlValue}/v1/chat/completions`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${task.apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(requestBody),
      signal: controller.signal,
    });

    const endTime = Date.now();
    const responseTime = ((endTime - startTime) / 1000).toFixed(2);
    task.responseTime = responseTime;

    if (response.ok) {
      const data = await response.json();
      const returnedModel = data.model || 'unknown';
      if (returnedModel === task.modelName) {
        task.status = 'success';
        task.statusText = '一致可用';
        task.remark = '通过';
      } else {
        task.status = 'warning';
        task.statusText = '模型重定向';
        task.remark = `映射由平台处理 -> ${returnedModel}`;
      }
    } else {
      let errText = '';
      try {
        const jsonResponse = await response.json();
        errText = jsonResponse.error?.message || '请求失败没返回原因';
      } catch (e) {
        errText = `HTTP ${response.status}`;
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

const copyValidConfigs = () => {
  const validTasks = testResults.value.filter(t => t.status === 'success' || t.status === 'warning');
  if (validTasks.length === 0) {
    message.warning('没有可用的配置组合！');
    return;
  }
  
  // Aggregate by siteUrl + apiKey
  const siteMap = new Map();
  validTasks.forEach(task => {
    const key = `${task.siteUrl}|${task.apiKey}`;
    if (!siteMap.has(key)) {
      siteMap.set(key, { ...task.accountData, supported_tested_models: [] });
    }
    siteMap.get(key).supported_tested_models.push(task.modelName);
  });
  
  const resultsToCopy = Array.from(siteMap.values()).map(acc => {
    return `====================\n平台名称: ${acc.site_name}\n接口地址: ${acc.site_url}\nAPI 密钥: ${acc.account_info.access_token}\n可用模型: ${acc.supported_tested_models.join(',')}\n`;
  });

  const textToCopy = resultsToCopy.join('\n');
  navigator.clipboard.writeText(textToCopy).then(() => {
    message.success(`已成功复制 ${siteMap.size} 个有效站点的配置信息`);
  }).catch(err => {
    console.error('Copy failed: ', err);
    message.error('复制失败');
  });
};

</script>

<style scoped>
.batch-wrapper {
  min-height: 100vh;
  padding: 20px;
}
.page-content {
  background-color: var(--container-bg);
  border-radius: 12px;
  box-shadow: var(--shadow-color);
  padding: 20px;
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
</style>
