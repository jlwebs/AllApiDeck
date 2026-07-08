<template>
  <div class="monitor-card" :class="{ 'monitor-card-disabled': !monitorEnabled }">
    <!-- 顶部：分组名 + 进度圈 + 自动优选 + 开关 -->
    <div class="monitor-card-header">
      <h3 class="monitor-card-title">{{ groupName }}</h3>
      <div class="monitor-card-actions">
        <!-- 自动优选队列开关 -->
        <a-tooltip title="启用后，每轮监控完成时自动按最近1小时成功率更新 provider 队列（排除成功率为0的渠道）">
          <div class="auto-optimize-switch">
            <span class="auto-optimize-label">自动优选队列</span>
            <a-switch
              :checked="autoOptimizeEnabled"
              size="small"
              @update:checked="handleToggleAutoOptimize"
            />
          </div>
        </a-tooltip>

        <!-- 倒计时进度圈 -->
        <a-tooltip v-if="monitorEnabled && nextCheckTime" :title="countdownTooltip">
          <div class="monitor-countdown-circle">
            <svg width="24" height="24" viewBox="0 0 24 24">
              <circle
                class="countdown-bg"
                cx="12"
                cy="12"
                r="10"
                fill="none"
                stroke="#e8e8e8"
                stroke-width="2"
              />
              <circle
                class="countdown-progress"
                cx="12"
                cy="12"
                r="10"
                fill="none"
                stroke="#1890ff"
                stroke-width="2"
                :stroke-dasharray="circumference"
                :stroke-dashoffset="progressOffset"
                transform="rotate(-90 12 12)"
              />
            </svg>
          </div>
        </a-tooltip>

        <a-tooltip :title="monitorEnabled ? '停止监控' : '启动监控'">
          <a-switch
            :checked="monitorEnabled"
            :loading="loading"
            @change="handleToggleMonitor"
          />
        </a-tooltip>
      </div>
    </div>

    <!-- 内容：渠道列表 + 每个渠道的健康条 -->
    <div class="monitor-card-body">
      <!-- 渠道列表：每行显示 站点/模型 + 健康条 -->
      <div v-if="channels.length > 0" class="monitor-channel-list">
        <!-- 时间轴标签（仅在第一个渠道上方显示） -->
        <div class="monitor-timeline-header">
          <div class="monitor-timeline-spacer"></div>
          <div class="monitor-timeline-labels">
            <span>24h前</span>
            <span>12h前</span>
            <span>现在</span>
          </div>
        </div>

        <div
          v-for="channel in channels"
          :key="channel.channelKey"
          class="monitor-channel-row"
        >
          <div class="monitor-channel-label">
            {{ channel.label }}
          </div>
          <div class="monitor-channel-healthbar">
            <MonitorHealthBar
              :history="history"
              :interval="interval"
              :channel-key="channel.channelKey"
            />
          </div>
        </div>
      </div>

      <!-- 空状态 -->
      <div v-else class="monitor-channel-empty">
        该分组下暂无密钥
      </div>

      <!-- 底部信息 -->
      <div v-if="lastCheckTime" class="monitor-footer">
        <span class="monitor-last-check">
          上次检测: {{ formatTime(lastCheckTime) }}
        </span>
        <span v-if="monitorEnabled && nextCheckTime" class="monitor-next-check">
          下次检测: {{ formatTime(nextCheckTime) }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue';
import MonitorHealthBar from './MonitorHealthBar.vue';

const props = defineProps({
  groupName: {
    type: String,
    required: true,
  },
  monitorEnabled: {
    type: Boolean,
    default: false,
  },
  interval: {
    type: Number,
    default: 10,
  },
  history: {
    type: Array,
    default: () => [],
  },
  channels: {
    type: Array,
    default: () => [],
  },
  lastCheckTime: {
    type: Number,
    default: null,
  },
  nextCheckTime: {
    type: Number,
    default: null,
  },
  loading: {
    type: Boolean,
    default: false,
  },
  autoOptimizeEnabled: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(['toggle', 'optimize-queue', 'toggle-auto-optimize']);

// 当前时间（用于计算倒计时）
const now = ref(Date.now());

// 圆周长
const circumference = 2 * Math.PI * 10; // r=10

// 进度偏移量（0% = 满圈，100% = 空圈）
const progressOffset = computed(() => {
  if (!props.monitorEnabled || !props.nextCheckTime || !props.lastCheckTime) {
    return circumference; // 空圈
  }

  const totalDuration = props.nextCheckTime - props.lastCheckTime;
  const elapsed = now.value - props.lastCheckTime;
  const progress = Math.min(elapsed / totalDuration, 1); // 0-1

  // progress=0 → offset=circumference（空圈）
  // progress=1 → offset=0（满圈）
  return circumference * (1 - progress);
});

// 倒计时文本
const countdownTooltip = computed(() => {
  if (!props.monitorEnabled || !props.nextCheckTime) {
    return '';
  }

  const remaining = Math.max(0, props.nextCheckTime - now.value);
  const seconds = Math.floor(remaining / 1000);

  if (seconds < 60) {
    return `${seconds} 秒后执行`;
  } else {
    const minutes = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${minutes} 分 ${secs} 秒后执行`;
  }
});

let countdownTimer = null;

onMounted(() => {
  startCountdownTimer();
});

onBeforeUnmount(() => {
  stopCountdownTimer();
});

function startCountdownTimer() {
  stopCountdownTimer();

  // 只在监控启用时运行定时器
  if (!props.monitorEnabled) return;

  // 每5秒更新一次倒计时（降低GPU负载）
  countdownTimer = setInterval(() => {
    now.value = Date.now();
  }, 5000);
}

function stopCountdownTimer() {
  if (countdownTimer) {
    clearInterval(countdownTimer);
    countdownTimer = null;
  }
}

// 监听 monitorEnabled 变化，动态启停定时器
watch(() => props.monitorEnabled, (enabled) => {
  if (enabled) {
    startCountdownTimer();
  } else {
    stopCountdownTimer();
  }
});

function handleToggleMonitor(checked) {
  emit('toggle', checked);
}

function handleToggleAutoOptimize(checked) {
  emit('toggle-auto-optimize', {
    groupName: props.groupName,
    enabled: checked,
  });
}

function handleAutoOptimizeQueue() {
  // 计算每个渠道的最近1小时成功率
  const now = Date.now();
  const oneHourAgo = now - 60 * 60 * 1000;

  const channelStats = props.channels.map(channel => {
    let recentTotal = 0;
    let recentSuccess = 0;

    props.history.forEach(entry => {
      if (entry.timestamp < oneHourAgo) return;

      const results = entry.results || [];
      const channelResult = results.find(r => `${r.siteUrl}||${r.model}` === channel.channelKey);

      if (!channelResult) return;

      recentTotal++;
      if (channelResult.status === 'success') {
        recentSuccess++;
      }
    });

    const successRate = recentTotal > 0 ? recentSuccess / recentTotal : 0;

    return {
      siteUrl: channel.siteUrl,
      model: channel.model,
      label: channel.label,
      successRate,
      hasData: recentTotal > 0,
    };
  });

  // 过滤掉成功率为0的，并按成功率降序排序
  const optimizedQueue = channelStats
    .filter(ch => ch.successRate > 0)
    .sort((a, b) => b.successRate - a.successRate);

  emit('optimize-queue', {
    groupName: props.groupName,
    queue: optimizedQueue,
  });
}

function formatTime(timestamp) {
  if (!timestamp) return '-';

  const nowTime = Date.now();
  const diff = nowTime - timestamp;

  if (diff < 60000) {
    return '刚刚';
  } else if (diff < 3600000) {
    return `${Math.floor(diff / 60000)} 分钟前`;
  } else if (diff < 86400000) {
    return `${Math.floor(diff / 3600000)} 小时前`;
  } else {
    const date = new Date(timestamp);
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
}
</script>

<style scoped>
.monitor-card {
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(90, 117, 79, 0.12);
  border-radius: 20px;
  padding: 20px;
  margin-bottom: 20px;
  box-shadow: 0 16px 36px rgba(98, 119, 84, 0.08);
  backdrop-filter: blur(8px);
  transition: all 0.3s ease;
}

.monitor-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 20px 40px rgba(98, 119, 84, 0.12);
}

.monitor-card-disabled {
  opacity: 0.7;
}

.monitor-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.monitor-card-title {
  font-size: 18px;
  font-weight: 600;
  color: #2c3e50;
  margin: 0;
}

.monitor-card-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.auto-optimize-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: rgba(24, 144, 255, 0.05);
  border-radius: 4px;
  border: 1px solid rgba(24, 144, 255, 0.2);
}

.auto-optimize-label {
  font-size: 13px;
  color: #1890ff;
  white-space: nowrap;
}

.monitor-countdown-circle {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.countdown-bg {
  transition: stroke 0.3s ease;
}

.countdown-progress {
  transition: stroke-dashoffset 1s linear;
  stroke-linecap: round;
  will-change: stroke-dashoffset;
}

.monitor-card-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.monitor-channel-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.monitor-timeline-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
}

.monitor-timeline-spacer {
  flex: 0 0 35%;
}

.monitor-timeline-labels {
  flex: 1 1 65%;
  display: flex;
  justify-content: space-between;
  font-size: 10px;
  color: #8c8c8c;
  padding: 0 2px;
}

.monitor-channel-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.monitor-channel-row:last-child {
  border-bottom: none;
}

.monitor-channel-label {
  flex: 0 0 35%;
  font-size: 13px;
  color: #595959;
  font-weight: 500;
  overflow-wrap: break-word;
  word-break: break-all;
  line-height: 1.3;
  padding-right: 8px;
}

.monitor-channel-healthbar {
  flex: 1 1 65%;
  min-width: 0;
}

.monitor-channel-empty {
  padding: 24px;
  text-align: center;
  color: #8c8c8c;
  font-size: 13px;
}

.monitor-footer {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #8c8c8c;
  padding-top: 8px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}

.monitor-last-check,
.monitor-next-check {
  display: flex;
  align-items: center;
}

/* 暗色模式 */
:deep(body.dark-mode) .monitor-card {
  background: rgba(30, 30, 30, 0.8);
  border-color: rgba(255, 255, 255, 0.1);
}

:deep(body.dark-mode) .monitor-card-title {
  color: #e6e6e6;
}

:deep(body.dark-mode) .monitor-stats {
  border-bottom-color: rgba(255, 255, 255, 0.08);
}

:deep(body.dark-mode) .monitor-stats-text {
  color: #e6e6e6;
}

:deep(body.dark-mode) .monitor-site-list {
  color: #bfbfbf;
}

:deep(body.dark-mode) .monitor-channel-label {
  color: #bfbfbf;
}

:deep(body.dark-mode) .monitor-timeline-labels {
  color: #bfbfbf;
}

:deep(body.dark-mode) .monitor-footer {
  border-top-color: rgba(255, 255, 255, 0.08);
}

:deep(body.dark-mode) .countdown-bg {
  stroke: #434343;
}

:deep(body.dark-mode) .countdown-progress {
  stroke: #177ddc;
}
</style>
