<template>
  <div class="monitor-health-bar">
    <div class="health-bar-grid">
      <div
        v-for="slot in healthSlots"
        :key="slot.id"
        :class="['health-slot', `health-slot-${slot.status}`]"
        :title="slot.tooltip"
        @click="handleSlotClick(slot)"
      ></div>
    </div>

    <!-- 失败详情弹框 -->
    <a-modal
      v-model:open="errorModalVisible"
      title="检测失败详情"
      :footer="null"
      width="600px"
    >
      <div v-if="selectedSlotError" class="error-detail-content">
        <div class="error-detail-time">
          {{ selectedSlotError.time }}
        </div>
        <a-divider style="margin: 12px 0" />
        <div class="error-detail-list">
          <div
            v-for="(err, idx) in selectedSlotError.errors"
            :key="idx"
            class="error-detail-item"
          >
            <div class="error-detail-channel">
              <span class="error-detail-label">渠道:</span>
              {{ err.channel }}
            </div>
            <div class="error-detail-message">
              <span class="error-detail-label">错误:</span>
              <pre>{{ err.message }}</pre>
            </div>
          </div>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { generateHealthSlots } from '../utils/monitorStore.js';

const props = defineProps({
  history: {
    type: Array,
    default: () => [],
  },
  interval: {
    type: Number,
    default: 10,
  },
  channelKey: {
    type: String,
    default: null,
  },
});

const healthSlots = computed(() => {
  return generateHealthSlots(props.history, props.interval, props.channelKey);
});

const errorModalVisible = ref(false);
const selectedSlotError = ref(null);

function handleSlotClick(slot) {
  // 只有错误状态才显示详情
  if (slot.status !== 'error') {
    return;
  }

  // 从 history 中找到该时间段的记录
  const slotStart = slot.timestamp;
  const slotDuration = props.interval * 60 * 1000;
  const slotEnd = slotStart + slotDuration;

  const records = props.history.filter(h =>
    h.timestamp >= slotStart && h.timestamp < slotEnd
  );

  if (records.length === 0) {
    return;
  }

  // 使用最新的记录
  const latest = records[records.length - 1];
  let results = latest.results || [];

  // 如果指定了渠道，只显示该渠道的错误
  if (props.channelKey) {
    results = results.filter(r => `${r.siteUrl}||${r.model}` === props.channelKey);
  }

  // 提取失败的结果
  const errors = results
    .filter(r => r.status === 'error')
    .map(r => {
      const siteName = r.siteUrl.replace(/^https?:\/\//, '').split('/')[0];
      return {
        channel: `${siteName} / ${r.model}`,
        message: r.errorDetail || r.errorMessage || r.error || '未知错误',
      };
    });

  if (errors.length === 0) {
    return;
  }

  // 格式化时间
  const timeStr = new Date(latest.timestamp).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });

  selectedSlotError.value = {
    time: timeStr,
    errors,
  };

  errorModalVisible.value = true;
}
</script>

<style scoped>
.monitor-health-bar {
  width: 100%;
}

.health-bar-grid {
  display: grid;
  grid-template-columns: repeat(72, 1fr);
  gap: 1px;
  height: 20px;
  contain: layout style paint;
  will-change: auto;
}

.health-slot {
  border-radius: 2px;
  cursor: pointer;
  contain: layout style paint;
}

.health-slot-success {
  background: #52c41a;
}

.health-slot-warning {
  background: #faad14;
}

.health-slot-error {
  background: #ff4d4f;
  cursor: pointer;
}

.health-slot-empty {
  background: #d9d9d9;
  opacity: 0.3;
}

.health-slot:hover {
  transform: scaleY(1.5);
  z-index: 1;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.health-slot-error:hover {
  background: #cf1322;
}

/* 错误详情弹框样式 */
.error-detail-content {
  max-height: 500px;
  overflow-y: auto;
}

.error-detail-time {
  font-size: 14px;
  color: #8c8c8c;
  text-align: center;
}

.error-detail-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.error-detail-item {
  padding: 12px;
  background: #fafafa;
  border-radius: 4px;
  border: 1px solid #f0f0f0;
}

.error-detail-channel {
  font-size: 14px;
  font-weight: 600;
  color: #262626;
  margin-bottom: 8px;
}

.error-detail-label {
  color: #8c8c8c;
  margin-right: 8px;
}

.error-detail-message {
  font-size: 13px;
  color: #595959;
}

.error-detail-message pre {
  margin: 4px 0 0 0;
  padding: 8px;
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 4px;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-wrap: break-word;
  color: #d32f2f;
}

/* 暗色模式 */
:deep(body.dark-mode) .health-slot-empty {
  background: #434343;
}

:deep(body.dark-mode) .error-detail-item {
  background: #262626;
  border-color: #434343;
}

:deep(body.dark-mode) .error-detail-channel {
  color: #e8e8e8;
}

:deep(body.dark-mode) .error-detail-message {
  color: #bfbfbf;
}

:deep(body.dark-mode) .error-detail-message pre {
  background: #1f1f1f;
  border-color: #434343;
  color: #ff6b6b;
}
</style>
