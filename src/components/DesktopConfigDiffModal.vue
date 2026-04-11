<template>
  <a-modal
    :open="open"
    title="确认写入本地配置"
    :width="width"
    ok-text="确认写入"
    cancel-text="取消"
    :ok-button-props="{ disabled: !hasWritableFiles }"
    @ok="$emit('confirm')"
    @cancel="$emit('cancel')"
  >
    <div class="diff-modal">
      <a-alert
        v-if="preview?.errors?.length"
        type="warning"
        show-icon
        class="diff-errors"
        :message="`以下应用未生成成功：${preview.errors.join('；')}`"
      />

      <a-empty v-if="!appGroups.length" description="暂无可预览的配置变更" />

      <template v-else>
        <a-tabs v-model:activeKey="activeAppId" class="diff-app-tabs">
          <a-tab-pane
            v-for="app in appGroups"
            :key="app.appId"
            :tab="`${app.appName} (${app.files.length})`"
          />
        </a-tabs>

        <a-tabs v-model:activeKey="activeFileKey" class="diff-file-tabs" size="small">
          <a-tab-pane
            v-for="file in activeAppFiles"
            :key="buildFileKey(file)"
            :tab="file.label"
          />
        </a-tabs>

        <div v-if="activeFile" class="diff-meta">
          <span>{{ activeFile.path }}</span>
          <a-tag :color="diffData.hasChanges ? 'orange' : 'green'">
            {{ diffData.hasChanges ? `${diffData.chunks.length} 处变更` : '无差异' }}
          </a-tag>
        </div>

        <div v-if="activeFile" class="diff-layout">
          <div class="diff-column">
            <div class="diff-column-header">变动前</div>
            <div ref="beforePaneRef" class="diff-pane" @scroll="handlePaneScroll('before')">
              <div
                v-for="(row, index) in diffData.rows"
                :key="`${row.key}-before`"
                :data-row-index="index"
                class="diff-row"
                :class="rowClassName(row, 'before')"
              >
                <div class="diff-line-number">
                  {{ row.beforeLineNumber ?? '' }}
                </div>
                <div class="diff-line-content">
                  <template v-if="row.beforeParts.length">
                    <span
                      v-for="(part, partIndex) in row.beforeParts"
                      :key="`${row.key}-before-${partIndex}`"
                      :class="{ 'diff-inline-change': part.changed }"
                    >
                      {{ part.text || ' ' }}
                    </span>
                  </template>
                </div>
              </div>
            </div>
          </div>

          <div class="diff-column">
            <div class="diff-column-header">变动后</div>
            <div ref="afterPaneRef" class="diff-pane" @scroll="handlePaneScroll('after')">
              <div
                v-for="(row, index) in diffData.rows"
                :key="`${row.key}-after`"
                :data-row-index="index"
                class="diff-row"
                :class="rowClassName(row, 'after')"
              >
                <div class="diff-line-number">
                  {{ row.afterLineNumber ?? '' }}
                </div>
                <div class="diff-line-content">
                  <template v-if="row.afterParts.length">
                    <span
                      v-for="(part, partIndex) in row.afterParts"
                      :key="`${row.key}-after-${partIndex}`"
                      :class="{ 'diff-inline-change': part.changed }"
                    >
                      {{ part.text || ' ' }}
                    </span>
                  </template>
                </div>
              </div>
            </div>
          </div>

          <div class="diff-minimap">
            <div class="diff-minimap-title">变更导航</div>
            <div class="diff-minimap-track">
              <button
                v-for="chunk in diffData.chunks"
                :key="chunk.id"
                type="button"
                class="diff-minimap-marker"
                :style="markerStyle(chunk)"
                :title="`跳转到第 ${chunk.startIndex + 1} 行附近`"
                @click="scrollToChunk(chunk)"
              ></button>
            </div>
          </div>
        </div>
      </template>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue';
import { buildSideBySideDiff } from '../utils/textDiff.js';

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  preview: {
    type: Object,
    default: () => ({ appGroups: [], writes: [], errors: [] }),
  },
  width: {
    type: Number,
    default: 1500,
  },
});

defineEmits(['cancel', 'confirm']);

const beforePaneRef = ref(null);
const afterPaneRef = ref(null);
const activeAppId = ref('');
const activeFileKey = ref('');
const syncingScroll = ref(false);

const appGroups = computed(() => props.preview?.appGroups || []);

const activeAppFiles = computed(() => {
  const app = appGroups.value.find(item => item.appId === activeAppId.value) || appGroups.value[0];
  return app?.files || [];
});

const activeFile = computed(() => {
  const file = activeAppFiles.value.find(item => buildFileKey(item) === activeFileKey.value);
  return file || activeAppFiles.value[0] || null;
});

const diffData = computed(() => buildSideBySideDiff(activeFile.value?.before, activeFile.value?.after));

const hasWritableFiles = computed(() => (props.preview?.writes || []).length > 0);

watch(
  () => props.open,
  async isOpen => {
    if (!isOpen) {
      return;
    }

    const firstApp = appGroups.value[0];
    activeAppId.value = firstApp?.appId || '';
    activeFileKey.value = firstApp?.files?.[0] ? buildFileKey(firstApp.files[0]) : '';
    await nextTick();
    focusFirstChange();
  }
);

watch(activeAppFiles, files => {
  if (!files.length) {
    activeFileKey.value = '';
    return;
  }

  const currentExists = files.some(file => buildFileKey(file) === activeFileKey.value);
  if (!currentExists) {
    activeFileKey.value = buildFileKey(files[0]);
  }
});

watch(
  () => activeFile.value?.path,
  async () => {
    await nextTick();
    focusFirstChange();
  }
);

function buildFileKey(file) {
  return `${file.appId}:${file.fileId}`;
}

function rowClassName(row, side) {
  if (row.type === 'equal') {
    return 'diff-row-equal';
  }

  if (row.type === 'modify') {
    return 'diff-row-modify';
  }

  if (row.type === 'add') {
    return side === 'after' ? 'diff-row-add' : 'diff-row-empty';
  }

  return side === 'before' ? 'diff-row-remove' : 'diff-row-empty';
}

function handlePaneScroll(source) {
  if (syncingScroll.value) {
    return;
  }

  const sourcePane = source === 'before' ? beforePaneRef.value : afterPaneRef.value;
  const targetPane = source === 'before' ? afterPaneRef.value : beforePaneRef.value;
  if (!sourcePane || !targetPane) {
    return;
  }

  syncingScroll.value = true;
  targetPane.scrollTop = sourcePane.scrollTop;
  requestAnimationFrame(() => {
    syncingScroll.value = false;
  });
}

function markerStyle(chunk) {
  const rowCount = Math.max(diffData.value.rows.length, 1);
  const top = (chunk.startIndex / rowCount) * 100;
  const height = Math.max(((chunk.endIndex - chunk.startIndex + 1) / rowCount) * 100, 1.6);
  return {
    top: `${top}%`,
    height: `${height}%`,
  };
}

function scrollToChunk(chunk) {
  if (!beforePaneRef.value || !afterPaneRef.value) {
    return;
  }

  const targetRow = beforePaneRef.value.querySelector(`[data-row-index="${chunk.startIndex}"]`);
  if (!targetRow) {
    return;
  }

  const targetTop = targetRow.offsetTop - 40;
  beforePaneRef.value.scrollTop = targetTop;
  afterPaneRef.value.scrollTop = targetTop;
}

function focusFirstChange() {
  const firstChunk = diffData.value.chunks[0];
  if (!firstChunk) {
    if (beforePaneRef.value) beforePaneRef.value.scrollTop = 0;
    if (afterPaneRef.value) afterPaneRef.value.scrollTop = 0;
    return;
  }
  scrollToChunk(firstChunk);
}
</script>

<style scoped>
.diff-modal {
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-x: hidden;
  min-width: 0;
}

.diff-errors {
  margin-bottom: 4px;
}

.diff-app-tabs,
.diff-file-tabs {
  margin-bottom: 0;
}

.diff-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: #64748b;
  font-size: 13px;
  word-break: break-all;
}

.diff-layout {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  min-height: 620px;
  min-width: 0;
  width: 100%;
}

.diff-column {
  min-width: 0;
  border: 1px solid #d9d9d9;
  border-radius: 12px;
  overflow: hidden;
  background: #fff;
}

.diff-column-header {
  padding: 10px 14px;
  font-weight: 600;
  border-bottom: 1px solid #f0f0f0;
  background: #fafafa;
}

.diff-pane {
  height: 560px;
  overflow: auto;
  font-family:
    'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
}

.diff-row {
  display: grid;
  grid-template-columns: 60px minmax(0, 1fr);
}

.diff-line-number {
  padding: 0 10px;
  color: #94a3b8;
  text-align: right;
  border-right: 1px solid #f1f5f9;
  user-select: none;
}

.diff-line-content {
  padding: 0 12px;
  white-space: pre-wrap;
  word-break: break-word;
  min-height: 19px;
}

.diff-row-equal {
  background: #fff;
}

.diff-row-modify {
  background: #fff7e6;
}

.diff-row-add {
  background: #f6ffed;
}

.diff-row-remove {
  background: #fff1f0;
}

.diff-row-empty {
  background: #fafafa;
}

.diff-inline-change {
  background: rgba(250, 173, 20, 0.28);
  border-radius: 2px;
}

.diff-row-add .diff-inline-change {
  background: rgba(82, 196, 26, 0.24);
}

.diff-row-remove .diff-inline-change {
  background: rgba(255, 77, 79, 0.2);
}

.diff-minimap {
  display: none;
}

.diff-minimap-title {
  writing-mode: vertical-rl;
  color: #64748b;
  font-size: 12px;
}

.diff-minimap-track {
  position: relative;
  width: 16px;
  flex: 1;
  min-height: 560px;
  border-radius: 999px;
  background: linear-gradient(180deg, #f8fafc 0%, #eef2f7 100%);
}

.diff-minimap-marker {
  position: absolute;
  left: 1px;
  width: 14px;
  border: 0;
  border-radius: 999px;
  background: linear-gradient(180deg, #fa8c16 0%, #f5222d 100%);
  cursor: pointer;
  opacity: 0.9;
}

.diff-minimap-marker:hover {
  opacity: 1;
  transform: scaleX(1.08);
}

</style>
