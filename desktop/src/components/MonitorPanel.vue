<template>
  <div class="monitor-panel">
    <!-- 监控卡片列表 -->
    <div v-if="monitorGroups.length > 0" class="monitor-card-grid">
      <MonitorCard
        v-for="group in monitorGroups"
        :key="group.id"
        :group-name="group.name"
        :monitor-enabled="group.monitorEnabled"
        :interval="group.interval"
        :history="group.history"
        :channels="group.channels"
        :last-check-time="group.lastCheckTime"
        :next-check-time="group.nextCheckTime"
        :loading="group.loading"
        :auto-optimize-enabled="group.autoOptimizeEnabled"
        @toggle="checked => handleToggleMonitor(group, checked)"
        @optimize-queue="handleOptimizeQueue"
        @toggle-auto-optimize="handleToggleAutoOptimize"
      />
    </div>

    <!-- 空状态 -->
    <a-empty
      v-else
      class="monitor-empty"
      description="暂无自定义分组"
    >
      <template #image>
        <FundProjectionScreenOutlined style="font-size: 64px; color: #bfbfbf;" />
      </template>
      <p style="margin-top: 16px; color: #8c8c8c;">
        请先在密钥管理中创建自定义分组
      </p>
    </a-empty>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { message } from 'ant-design-vue';
import { ReloadOutlined, DeleteOutlined, FundProjectionScreenOutlined } from '@ant-design/icons-vue';
import MonitorCard from './MonitorCard.vue';
import monitorScheduler from '../utils/monitorScheduler.js';
import {
  loadMonitorConfigs,
  setMonitorConfig,
  getMonitorConfig,
  loadMonitorHistory,
  clearAllMonitorHistory,
} from '../utils/monitorStore.js';

const props = defineProps({
  keyGroups: {
    type: Array,
    default: () => [],
  },
  getGroupRecords: {
    type: Function,
    required: true,
  },
  globalInterval: {
    type: Number,
    default: 10,
  },
  refreshSignal: {
    type: Number,
    default: 0,
  },
});

const emit = defineEmits(['optimize-queue']);

// 内部刷新令牌，用于强制重新计算
const refreshToken = ref(0);

function triggerRefresh() {
  refreshToken.value++;
}

// 获取记录在特定分组下的选择模型
function getRecordScopedModel(record, groupId) {
  if (!record || !groupId) {
    return record?.selectedModel || record?.model || '';
  }

  const groupSelectedModels = record.groupSelectedModels || {};
  const scopedModel = groupSelectedModels[groupId];

  if (scopedModel) {
    return String(scopedModel).trim();
  }

  // fallback 到全局 selectedModel
  return String(record.selectedModel || record.model || '').trim();
}

// 按可用性排序渠道
// 优先级：最近一次可用 > 最近1小时可用率 > 总可用率
function sortChannelsByAvailability(channels, history) {
  const now = Date.now();
  const oneHourAgo = now - 60 * 60 * 1000;

  return channels.slice().sort((a, b) => {
    const statsA = calculateChannelStats(a.channelKey, history, oneHourAgo);
    const statsB = calculateChannelStats(b.channelKey, history, oneHourAgo);

    // 1. 最近一次可用优先
    if (statsA.lastSuccess !== statsB.lastSuccess) {
      return statsB.lastSuccess - statsA.lastSuccess;
    }

    // 2. 最近1小时可用率
    if (statsA.recentRate !== statsB.recentRate) {
      return statsB.recentRate - statsA.recentRate;
    }

    // 3. 总可用率
    return statsB.totalRate - statsA.totalRate;
  });
}

// 计算渠道统计数据
function calculateChannelStats(channelKey, history, oneHourAgo) {
  let lastSuccess = 0;
  let recentTotal = 0;
  let recentSuccess = 0;
  let totalCount = 0;
  let totalSuccess = 0;

  history.forEach(entry => {
    const results = entry.results || [];
    const channelResult = results.find(r => `${r.siteUrl}||${r.model}` === channelKey);

    if (!channelResult) return;

    const isSuccess = channelResult.status === 'success';
    const timestamp = entry.timestamp;

    // 总统计
    totalCount++;
    if (isSuccess) {
      totalSuccess++;
      if (timestamp > lastSuccess) {
        lastSuccess = timestamp;
      }
    }

    // 最近1小时统计
    if (timestamp >= oneHourAgo) {
      recentTotal++;
      if (isSuccess) {
        recentSuccess++;
      }
    }
  });

  return {
    lastSuccess: lastSuccess ? 1 : 0, // 转为0/1便于排序
    recentRate: recentTotal > 0 ? recentSuccess / recentTotal : 0,
    totalRate: totalCount > 0 ? totalSuccess / totalCount : 0,
  };
}

// 监控分组数据（依赖 refreshToken 和 refreshSignal 实现响应式）
const monitorGroups = computed(() => {
  // 显式依赖，确保变化时重新计算
  void refreshToken.value;
  void props.refreshSignal;

  const configs = loadMonitorConfigs();

  return props.keyGroups.map(group => {
    const groupName = group.name;
    const groupId = group.id;
    const config = configs[groupName] || {
      enabled: false,
      interval: props.globalInterval,
      lastCheck: 0,
      nextCheck: 0,
    };

    const history = loadMonitorHistory(groupName);
    const records = props.getGroupRecords(groupName);

    // 生成站点/模型渠道列表（每个渠道有独立的健康条）
    // 使用分组范围的 selectedModel
    const channels = records.map(record => {
      const siteUrl = record.siteUrl || record.site_url || record.site || '';
      const model = getRecordScopedModel(record, groupId);
      const siteName = siteUrl.replace(/^https?:\/\//, '').split('/')[0];
      const channelKey = `${siteUrl}||${model}`;

      return {
        siteUrl,
        model,
        channelKey,
        label: `${siteName} / ${model}`,
      };
    });

    // 按可用性排序渠道
    const sortedChannels = sortChannelsByAvailability(channels, history);

    return {
      id: group.id,
      name: groupName,
      monitorEnabled: config.enabled,
      interval: config.interval,
      lastCheckTime: config.lastCheck,
      nextCheckTime: config.nextCheck,
      history,
      channels: sortedChannels,
      loading: false, // 不再显示 loading 状态
      autoOptimizeEnabled: config.autoOptimizeEnabled || false,
    };
  });
});

function handleToggleMonitor(group, checked) {
  const config = getMonitorConfig(group.name) || {
    enabled: checked,
    interval: props.globalInterval,
    lastCheck: 0,
    nextCheck: 0,
  };

  if (checked) {
    // 启动监控
    monitorScheduler.start(group.name, {
      ...config,
      interval: props.globalInterval,
    });
    message.success(`已启动监控: ${group.name}`);
  } else {
    // 停止监控
    monitorScheduler.stop(group.name);
    message.info(`已停止监控: ${group.name}`);
  }

  // 立即触发刷新
  triggerRefresh();
}

function handleOptimizeQueue(payload) {
  // 向 KeyManagement 传递优选队列数据
  emit('optimize-queue', payload);
}

function handleToggleAutoOptimize(payload) {
  const { groupName, enabled } = payload;

  const config = getMonitorConfig(groupName) || {
    enabled: false,
    interval: props.globalInterval,
    lastCheck: 0,
    nextCheck: 0,
  };

  // 更新配置
  setMonitorConfig(groupName, {
    ...config,
    autoOptimizeEnabled: enabled,
  });

  if (enabled) {
    message.success(`已启用 ${groupName} 的自动优选队列`);

    // 立即执行一次优选队列（首次启用时）
    autoOptimizeQueueForGroup(groupName, false); // 传入 false 表示手动触发，显示消息
  } else {
    message.info(`已禁用 ${groupName} 的自动优选队列`);
  }

  // 刷新UI
  triggerRefresh();
}

// 自动优选队列（可选静默执行）
function autoOptimizeQueueForGroup(groupName, auto = true) {
  const group = props.keyGroups.find(g => g.name === groupName);
  if (!group) return;

  const history = loadMonitorHistory(groupName);
  const records = props.getGroupRecords(groupName);

  // 计算每个渠道的最近1小时成功率
  const now = Date.now();
  const oneHourAgo = now - 60 * 60 * 1000;

  const channels = records.map(record => {
    const siteUrl = record.siteUrl || record.site_url || record.site || '';
    const model = getRecordScopedModel(record, group.id);
    const channelKey = `${siteUrl}||${model}`;

    let recentTotal = 0;
    let recentSuccess = 0;

    history.forEach(entry => {
      if (entry.timestamp < oneHourAgo) return;

      const results = entry.results || [];
      const channelResult = results.find(r => `${r.siteUrl}||${r.model}` === channelKey);

      if (!channelResult) return;

      recentTotal++;
      if (channelResult.status === 'success') {
        recentSuccess++;
      }
    });

    const successRate = recentTotal > 0 ? recentSuccess / recentTotal : 0;

    return {
      siteUrl,
      model,
      label: `${siteUrl.replace(/^https?:\/\//, '').split('/')[0]} / ${model}`,
      successRate,
      hasData: recentTotal > 0,
    };
  });

  // 过滤掉成功率为0的，并按成功率降序排序
  const optimizedQueue = channels
    .filter(ch => ch.successRate > 0)
    .sort((a, b) => b.successRate - a.successRate);

  if (optimizedQueue.length > 0) {
    // 触发优选队列
    emit('optimize-queue', {
      groupName,
      queue: optimizedQueue,
      auto, // 传递 auto 标记，控制是否显示消息和弹出面板
    });
  }
}

let uiRefreshTimer = null;

onMounted(() => {
  // 设置获取分组记录的回调
  monitorScheduler.setGetGroupRecordsFn(props.getGroupRecords);

  // 设置历史更新回调，检测完成后立即刷新UI
  monitorScheduler.setOnHistoryUpdate(() => {
    triggerRefresh();
  });

  // 设置检测完成回调，用于自动优选队列
  monitorScheduler.setOnCheckComplete((groupName) => {
    const config = getMonitorConfig(groupName);
    if (config?.autoOptimizeEnabled) {
      // 自动触发优选队列
      autoOptimizeQueueForGroup(groupName);
    }
  });

  // 注意：不在这里调用 resetAllMonitors
  // 应该在应用启动时清空，而不是每次切换标签时清空

  // 定时刷新UI（每5秒，更新倒计时）
  uiRefreshTimer = setInterval(() => {
    triggerRefresh();
  }, 5000);
});

onBeforeUnmount(() => {
  // 注意：不停止监控调度器，让它在后台继续运行
  // 只清理UI刷新定时器
  if (uiRefreshTimer) {
    clearInterval(uiRefreshTimer);
    uiRefreshTimer = null;
  }
});
</script>

<style scoped>
.monitor-panel {
  width: 100%;
  padding: 0;
}

.monitor-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(480px, 1fr));
  gap: 20px;
}

.monitor-empty {
  margin-top: 80px;
}

/* 暗色模式 */
:deep(body.dark-mode) .monitor-panel-title {
  color: #e6e6e6;
}

:deep(body.dark-mode) .monitor-panel-subtitle {
  color: #bfbfbf;
}
</style>
