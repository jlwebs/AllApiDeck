<template>
  <div class="editor-view">
    <div class="editor-shell">
      <div class="editor-header">
        <div>
          <div class="editor-title">{{ editingRecord ? '编辑密钥' : '手工添加密钥' }}</div>
          <div class="editor-subtitle">保存后会直接写入本地密钥库</div>
        </div>
        <a-button type="text" @click="closeWindow">关闭</a-button>
      </div>

      <a-form layout="vertical" class="editor-form">
        <a-form-item label="网站">
          <a-input v-model:value="draft.siteName" placeholder="例如 Claude Hub" />
        </a-form-item>
        <a-form-item label="Token 名称">
          <a-input v-model:value="draft.tokenName" placeholder="可选" />
        </a-form-item>
        <a-form-item label="接口地址">
          <a-input v-model:value="draft.siteUrl" placeholder="https://example.com" />
        </a-form-item>
        <a-form-item label="API Key">
          <a-input-password v-model:value="draft.apiKey" placeholder="sk-..." />
        </a-form-item>
        <a-form-item label="模型">
          <a-select
            v-model:value="draft.modelsValue"
            :options="modelOptions"
            :loading="modelLoading"
            show-search
            :filter-option="true"
            option-filter-prop="label"
            placeholder="打开这里后自动抓取模型"
            @dropdownVisibleChange="handleModelDropdownVisibleChange"
          />
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="draft.status">
            <a-select-option :value="1">正常</a-select-option>
            <a-select-option :value="2">禁用/异常</a-select-option>
          </a-select>
        </a-form-item>
      </a-form>

      <div class="editor-footer">
        <a-button @click="closeWindow">取消</a-button>
        <a-button type="primary" :loading="saving" @click="submitRecord">保存</a-button>
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
.editor-view{min-height:100vh;padding:20px;background:linear-gradient(180deg,#f8fafc,#eef2ff)}
.editor-shell{max-width:560px;margin:0 auto;padding:22px 22px 18px;border-radius:24px;background:rgba(255,255,255,.96);box-shadow:0 24px 60px rgba(15,23,42,.14)}
.editor-header{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;margin-bottom:18px}
.editor-title{font-size:22px;font-weight:700;color:#0f172a}
.editor-subtitle{margin-top:4px;font-size:12px;color:#64748b}
.editor-form{margin-top:8px}
.editor-footer{display:flex;align-items:center;justify-content:flex-end;gap:10px;margin-top:8px}
</style>
