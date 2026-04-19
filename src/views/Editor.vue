<template>
  <div class="editor-view">
    <div class="editor-shell">
      <div class="editor-header">
        <div class="editor-header-copy">
          <div class="editor-kicker">Key Editor</div>
          <div class="editor-title">{{ editingRecord ? '编辑密钥' : '手工添加密钥' }}</div>
          <div class="editor-subtitle">常用字段两列排布，减少上下滚动和来回切换。</div>
        </div>

        <div class="editor-header-actions">
          <a-button type="text" size="small" class="editor-close-button" aria-label="关闭" title="关闭" @click="closeWindow">×</a-button>
        </div>
      </div>

      <a-form layout="vertical" class="editor-form">
        <div class="editor-fields">
          <div class="editor-row">
            <a-form-item label="网站" class="editor-form-item">
              <a-input v-model:value="draft.siteName" size="small" placeholder="例如 Claude Hub" />
            </a-form-item>
            <a-form-item label="Token 名称" class="editor-form-item">
              <a-input v-model:value="draft.tokenName" size="small" placeholder="可选" />
            </a-form-item>
          </div>

          <div class="editor-row">
            <a-form-item label="接口地址" class="editor-form-item">
              <a-input v-model:value="draft.siteUrl" size="small" placeholder="https://example.com" />
            </a-form-item>
            <a-form-item label="API Key" class="editor-form-item">
              <a-input-password v-model:value="draft.apiKey" size="small" placeholder="sk-..." />
            </a-form-item>
          </div>

          <div class="editor-row editor-row-last">
            <a-form-item label="状态" class="editor-form-item editor-form-item-tight">
              <a-select v-model:value="draft.status" size="small">
                <a-select-option :value="1">正常</a-select-option>
                <a-select-option :value="2">禁用/异常</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="模型" class="editor-form-item editor-form-item-tight">
              <a-select
                v-model:value="draft.modelsValue"
                :options="modelOptions"
                :loading="modelLoading"
                size="small"
                show-search
                :filter-option="true"
                option-filter-prop="label"
                placeholder="打开后自动抓取模型"
                @dropdownVisibleChange="handleModelDropdownVisibleChange"
              />
            </a-form-item>
          </div>
        </div>
      </a-form>

      <div class="editor-footer">
        <a-button size="small" @click="closeWindow">取消</a-button>
        <a-button type="primary" size="small" :loading="saving" @click="submitRecord">保存</a-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue';
import { message } from 'ant-design-vue';
import { GetLaunchRecordKey, RequestQuit } from '../../wailsjs/go/main/App.js';
import {
  buildManualRecordFromDraft,
  createManualDraft,
  getRecordModelOptions,
  hydrateRecordModelSelection,
  loadBatchHistoryContextMap,
  loadPanelRecords,
  loadRecordModelOptions,
  persistPanelRecords,
} from '../utils/keyPanelStore.js';
import { hydrateLastResultsSnapshotCache } from '../utils/historySnapshotStore.js';

const records = ref([]);
const contextMap = ref(new Map());
const editingRecord = ref(null);
const draft = reactive(createManualDraft());
const modelOptions = ref([]);
const modelLoading = ref(false);
const saving = ref(false);

function overwriteDraft(nextDraft) {
  Object.keys(draft).forEach(key => delete draft[key]);
  Object.assign(draft, nextDraft);
}

async function closeWindow() {
  await RequestQuit();
}

async function bootstrap() {
  await hydrateLastResultsSnapshotCache();
  const loaded = loadPanelRecords();
  contextMap.value = loaded.contextMap || loadBatchHistoryContextMap();
  records.value = loaded.records;
  const rowKey = await GetLaunchRecordKey().catch(() => '');
  const matched = rowKey ? records.value.find(item => item.rowKey === rowKey) || null : null;
  editingRecord.value = matched;
  overwriteDraft(createManualDraft(matched));
  modelOptions.value = matched ? getRecordModelOptions(matched, contextMap.value) : [];
}

async function handleModelDropdownVisibleChange(open) {
  if (!open || modelLoading.value) return;
  modelLoading.value = true;
  try {
    const nextRecord = await loadRecordModelOptions({
      ...editingRecord.value,
      ...draft,
      selectedModel: draft.modelsValue,
      modelsList: draft.modelsValue ? [draft.modelsValue] : [],
    }, contextMap.value, true);
    modelOptions.value = getRecordModelOptions(nextRecord, contextMap.value);
    if (!draft.modelsValue) {
      draft.modelsValue = nextRecord.selectedModel || '';
    }
  } catch (error) {
    message.error(error?.message || '模型获取失败');
  } finally {
    modelLoading.value = false;
  }
}

async function submitRecord() {
  saving.value = true;
  try {
    const existingRecord = editingRecord.value
      ? records.value.find(item => item.rowKey === editingRecord.value.rowKey) || null
      : null;
    const nextRecord = hydrateRecordModelSelection(buildManualRecordFromDraft(draft, existingRecord), contextMap.value);
    if (!nextRecord.siteName || !nextRecord.siteUrl || !nextRecord.apiKey) {
      throw new Error('网站、接口地址、API Key 不能为空');
    }
    const nextRecords = [
      ...records.value.filter(item => item.rowKey !== nextRecord.rowKey),
      nextRecord,
    ].sort((left, right) => Number(right.updatedAt || 0) - Number(left.updatedAt || 0));
    persistPanelRecords(nextRecords);
    message.success(existingRecord ? '已更新手工配置' : '已添加手工配置');
    setTimeout(() => {
      void closeWindow();
    }, 160);
  } catch (error) {
    message.error(error?.message || '保存失败');
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void bootstrap();
});
</script>

<style scoped>
.editor-view {
  width: 100%;
  height: 100vh;
  min-height: 100vh;
  box-sizing: border-box;
  padding: 8px;
  display: flex;
  overflow: hidden;
  border-radius: 24px;
  background:
    radial-gradient(circle at top left, rgba(191, 219, 254, 0.75), transparent 34%),
    radial-gradient(circle at top right, rgba(167, 243, 208, 0.45), transparent 30%),
    linear-gradient(180deg, #f8fafc, #eef2ff);
}

.editor-shell {
  width: min(100%, 880px);
  max-width: 100%;
  margin: 0 auto;
  padding: 10px 12px 8px;
  border-radius: 21px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.94));
  box-shadow:
    0 18px 38px rgba(15, 23, 42, 0.12),
    inset 0 1px 0 rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(12px) saturate(108%);
  overflow: hidden;
}

.editor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
  -webkit-app-region: drag;
}

.editor-header-copy {
  min-width: 0;
}

.editor-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
  -webkit-app-region: no-drag;
}

.editor-close-button {
  padding-inline: 8px;
  color: #ef4444;
  font-size: 20px;
  font-weight: 800;
  line-height: 1;
  -webkit-app-region: no-drag;
}

.editor-close-button:hover {
  color: #dc2626 !important;
  background: rgba(239, 68, 68, 0.08);
}

.editor-kicker {
  margin-bottom: 2px;
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.editor-title {
  color: #0f172a;
  font-size: 18px;
  font-weight: 800;
  line-height: 1.2;
}

.editor-subtitle {
  max-width: 48ch;
  margin-top: 2px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.35;
}

.editor-form {
  margin-top: 0;
}

.editor-fields {
  display: grid;
  gap: 6px;
}

.editor-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 6px;
  align-items: start;
}

.editor-row-last {
  grid-template-columns: minmax(150px, 0.7fr) minmax(0, 1.3fr);
}

.editor-form-item {
  margin-bottom: 0;
  min-width: 0;
}

.editor-form-item :deep(.ant-form-item-label) {
  padding-bottom: 1px;
}

.editor-form-item :deep(.ant-form-item-control) {
  min-width: 0;
}

.editor-form-item :deep(.ant-form-item-label > label) {
  color: #334155;
  font-size: 10px;
  font-weight: 600;
  line-height: 14px;
  height: 14px;
}

.editor-form-item :deep(.ant-input),
.editor-form-item :deep(.ant-input-password),
.editor-form-item :deep(.ant-select-selector) {
  border-radius: 11px;
}

.editor-form-item :deep(.ant-input),
.editor-form-item :deep(.ant-input-password) {
  padding: 1px 10px;
}

.editor-form-item :deep(.ant-select-selection-item),
.editor-form-item :deep(.ant-select-selection-placeholder) {
  font-size: 12px;
  line-height: 22px;
}

.editor-form-item-tight {
  align-self: end;
}

.editor-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 6px;
  padding-top: 6px;
  border-top: 1px solid rgba(148, 163, 184, 0.18);
}

@media (max-width: 560px) {
  .editor-view {
    padding: 10px;
  }

  .editor-shell {
    padding: 14px;
  }

  .editor-row,
  .editor-row-last {
    grid-template-columns: 1fr;
  }
}
</style>
