<template>
  <div class="key-management" style="padding: 20px;">
    <a-card title="批量获取真实 API 密钥 (Token)" style="margin-bottom: 20px;">
      <a-upload
        :before-upload="beforeUpload"
        :file-list="fileList"
        :max-count="1"
        accept=".json"
        :showUploadList="false"
      >
        <a-button type="primary">
          <upload-outlined /> 导入账号备份 (支持 Linuxdo Connect 等格式)
        </a-button>
      </a-upload>

      <div v-if="loading" style="margin-top: 15px; display: flex; align-items: center; gap: 10px;">
        <a-spin />
        <span>正在并发获取全部站点的 Key，请耐心等待...</span>
      </div>

      <a-alert
        v-if="!loading && failedSites.length > 0"
        type="warning"
        style="margin-top: 15px;"
        :message="`${failedSites.length} 个站点未获取到 Key，详情见 logs/fetch-keys.log`"
        :description="failedSites.map(s => s.site_name).join('、')"
        show-icon
      />
    </a-card>

    <a-card v-if="tableData.length > 0" :title="`获取到的 API 密钥列表（共 ${tableData.length} 条）`">
      <template #extra>
        <a-space>
          <a-button @click="copyAllKeys" type="primary">复制全部有效 Key</a-button>
          <a-button @click="exportCsv">导出 CSV</a-button>
        </a-space>
      </template>

      <a-table
        :columns="columns"
        :data-source="tableData"
        :row-key="r => r.rowKey"
        :pagination="{ pageSize: 20, showSizeChanger: true, pageSizeOptions: ['20','50','100'] }"
        size="middle"
        :scroll="{ x: 1200 }"
      >
        <template #bodyCell="{ column, record }">
          <!-- Key 列：固定宽度 + 省略 + tooltip + 一键复制 -->
          <template v-if="column.dataIndex === 'key'">
            <div class="key-cell">
              <a-typography-text
                :copyable="{ text: record.key }"
                :ellipsis="{ tooltip: record.key }"
                style="max-width: 220px; display: inline-block;"
              >{{ record.key }}</a-typography-text>
            </div>
          </template>

          <template v-else-if="column.dataIndex === 'status'">
            <a-tag :color="record.status === 1 ? 'green' : 'red'">
              {{ record.status === 1 ? '正常' : '禁用/异常' }}
            </a-tag>
          </template>

          <template v-else-if="column.dataIndex === 'remain_quota'">
            <span v-if="record.unlimited_quota" style="color: #1677ff;">无限额度</span>
            <span v-else-if="record.remain_quota > 0" style="color: #52c41a; font-weight: 600;">
              {{ formatQuota(record.remain_quota) }} $
            </span>
            <span v-else style="color: #ff4d4f;">已耗尽</span>
          </template>

          <template v-else-if="column.dataIndex === 'used_quota'">
            <span>{{ formatQuota(record.used_quota) }} $</span>
          </template>

          <template v-else-if="column.dataIndex === 'models'">
            <a-tooltip :title="record.models">
              <span style="max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; vertical-align: middle;">
                {{ record.models }}
              </span>
            </a-tooltip>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { message } from 'ant-design-vue';
import { UploadOutlined } from '@ant-design/icons-vue';
import { apiFetch } from '../utils/runtimeApi.js';

const fileList = ref([]);
const loading = ref(false);
const tableData = ref([]);
const allResults = ref([]); // 原始结果，含失败

const failedSites = computed(() =>
  allResults.value.filter(r => !r.tokens || r.tokens.length === 0)
);

const columns = [
  {
    title: '归属网站',
    dataIndex: 'site_name',
    key: 'site_name',
    width: 130,
    fixed: 'left',
    sorter: (a, b) => a.site_name.localeCompare(b.site_name),
  },
  {
    title: '令牌名称',
    dataIndex: 'name',
    key: 'name',
    width: 130,
    sorter: (a, b) => (a.name || '').localeCompare(b.name || ''),
  },
  {
    title: '真实 API Key',
    dataIndex: 'key',
    key: 'key',
    width: 260,
  },
  {
    title: '剩余额度',
    dataIndex: 'remain_quota',
    key: 'remain_quota',
    width: 110,
    sorter: (a, b) => {
      if (a.unlimited_quota && b.unlimited_quota) return 0;
      if (a.unlimited_quota) return 1;
      if (b.unlimited_quota) return -1;
      return (a.remain_quota || 0) - (b.remain_quota || 0);
    },
  },
  {
    title: '已用额度',
    dataIndex: 'used_quota',
    key: 'used_quota',
    width: 100,
    sorter: (a, b) => (a.used_quota || 0) - (b.used_quota || 0),
  },
  {
    title: '模型限制',
    dataIndex: 'models',
    key: 'models',
    width: 180,
  },
  {
    title: '状态',
    dataIndex: 'status',
    key: 'status',
    width: 90,
    sorter: (a, b) => a.status - b.status,
  },
];

const formatQuota = (quota) => {
  if (quota === undefined || quota === null) return '0.00';
  return (quota / 500000).toFixed(4);
};

const beforeUpload = (file) => {
  const reader = new FileReader();
  reader.onload = (e) => {
    try {
      const data = JSON.parse(e.target.result);
      if (data.accounts && data.accounts.accounts) {
        message.success('成功加载账号数据，开始并发获取真实密钥...');
        processAccounts(data.accounts.accounts);
      } else {
        message.error('无效的格式，找不到 accounts 数组。');
      }
    } catch (error) {
      console.error(error);
      message.error('解析备份 JSON 文件失败！');
    }
  };
  reader.readAsText(file);
  return false;
};

const processAccounts = async (accounts) => {
  const accountsToTarget = accounts.filter(
    (acc) => acc.account_info && acc.account_info.access_token
  );

  loading.value = true;
  tableData.value = [];
  allResults.value = [];

  try {
    const response = await apiFetch('/api/fetch-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ accounts: accountsToTarget }),
    });

    if (!response.ok) {
      const err = await response.json();
      throw new Error(err.message || '后端请求失败');
    }

    const { results } = await response.json();
    allResults.value = results;

    const currentResults = [];
    results.forEach((result) => {
      if (!result.tokens || result.tokens.length === 0) return;

      result.tokens.forEach((k, index) => {
        let theKey = k.key || '';
        if (theKey && !theKey.startsWith('sk-')) {
          theKey = 'sk-' + theKey;
        }
        currentResults.push({
          rowKey: `${result.id}-${k.id || index}`,
          site_name: result.site_name,
          site_url: result.site_url,
          name: k.name || `未命名 Token ${k.id || index + 1}`,
          key: theKey,
          status: typeof k.status === 'number' ? k.status : k.is_disabled ? 2 : 1,
          remain_quota: k.remain_quota,
          used_quota: k.used_quota,
          unlimited_quota:
            k.unlimited_quota === true ||
            k.remain_quota < 0 ||
            k.remain_quota === undefined,
          models: k.models
            ? Array.isArray(k.models)
              ? k.models.join(', ')
              : k.models
            : '不限 (全部模型)',
        });
      });
    });

    tableData.value = currentResults;
    const failed = results.filter((r) => !r.tokens || r.tokens.length === 0).length;
    message.success(
      `成功拉取完毕！获取到 ${currentResults.length} 个密钥，${failed} 个站点失败（见 logs/fetch-keys.log）`
    );
  } catch (error) {
    console.error(error);
    message.error(`获取失败：${error.message}`);
  } finally {
    loading.value = false;
  }
};

const copyAllKeys = () => {
  const validKeys = tableData.value
    .filter((r) => r.status === 1)
    .map((r) => r.key)
    .filter(Boolean);

  if (validKeys.length === 0) {
    message.warning('没有状态正常的 Key');
    return;
  }
  navigator.clipboard
    .writeText(validKeys.join('\n'))
    .then(() => message.success(`已复制 ${validKeys.length} 个 Key 到剪贴板！`))
    .catch(() => message.error('复制失败，请手动复制'));
};

const exportCsv = () => {
  if (!tableData.value.length) return;
  let csv = '\uFEFF归属网站,名称,API Key,状态,剩余额度,已用额度,模型限制\n';
  tableData.value.forEach((r) => {
    csv += [
      `"${r.site_name}"`,
      `"${r.name}"`,
      `"${r.key}"`,
      r.status === 1 ? '正常' : '禁用',
      r.unlimited_quota ? '无限' : formatQuota(r.remain_quota),
      formatQuota(r.used_quota),
      `"${r.models}"`,
    ].join(',') + '\n';
  });
  const a = document.createElement('a');
  a.href = 'data:text/csv;charset=utf-8,' + encodeURIComponent(csv);
  a.download = `api-keys-${Date.now()}.csv`;
  a.click();
};
</script>

<style scoped>
.key-management { width: 100%; }
.key-cell { display: flex; align-items: center; gap: 6px; max-width: 260px; overflow: hidden; }
</style>
