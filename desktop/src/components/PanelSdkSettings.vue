<template>
  <div class="panel-sdk">
    <aside class="sdk-api-sidebar" aria-label="API 列表">
      <div class="sdk-api-sidebar-head">
        <span>API 列表</span>
        <span class="sdk-api-count">1</span>
      </div>
      <div class="sdk-api-group">密钥管理</div>
      <button type="button" class="sdk-api-item is-active">
        <span class="sdk-method sdk-method-post">POST</span>
        <span class="sdk-api-item-copy">
          <strong>剪贴板智能导入</strong>
          <small>/clipboard-import</small>
        </span>
      </button>
    </aside>

    <main class="sdk-document">
      <header class="sdk-document-head">
        <div>
          <div class="sdk-document-title-row">
            <span class="sdk-method sdk-method-post">POST</span>
            <h2>剪贴板智能导入</h2>
          </div>
          <p>解析剪贴板文本中的站点地址和 API Key，并写入密钥管理。</p>
        </div>
        <span class="sdk-runtime-badge">本地接口</span>
      </header>

      <div class="sdk-endpoint">
        <code>{{ endpointUrl }}</code>
        <a-tooltip title="复制接口地址">
          <button type="button" class="sdk-icon-button" aria-label="复制接口地址" @click="copyContent(endpointUrl, '接口地址已复制')">
            <CopyOutlined />
          </button>
        </a-tooltip>
      </div>

      <section class="sdk-section">
        <h3>请求规范</h3>
        <div class="sdk-table-wrap">
          <table class="sdk-table">
            <thead>
              <tr>
                <th>字段</th>
                <th>类型</th>
                <th>必填</th>
                <th>说明</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td><code>targetGroupName</code></td>
                <td>string</td>
                <td>否</td>
                <td>目标分组名。省略、空字符串、<code>全部分组</code> 或 <code>全部密钥</code> 时不附加指定分组。</td>
              </tr>
              <tr>
                <td><code>clipboardText</code></td>
                <td>string</td>
                <td>是</td>
                <td>待解析的剪贴板原文，支持“说明 / URL / Key”、URL 与 Key 互换顺序及多组连续文本。</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="sdk-field-aliases">
          字段别名：<code>target_group_name</code>、<code>clipboard_text</code>、<code>目标分组名</code>、<code>剪贴板文本</code>。
        </p>
      </section>

      <section class="sdk-section">
        <div class="sdk-section-head">
          <h3>请求示例</h3>
          <div class="sdk-example-actions">
            <a-segmented v-model:value="exampleMode" size="small" :options="exampleModes" />
            <a-tooltip title="复制请求示例">
              <button type="button" class="sdk-icon-button" aria-label="复制请求示例" @click="copyContent(requestExample, '请求示例已复制')">
                <CopyOutlined />
              </button>
            </a-tooltip>
          </div>
        </div>
        <pre class="sdk-code"><code>{{ requestExample }}</code></pre>
      </section>

      <section class="sdk-section">
        <h3>导入规则</h3>
        <ul class="sdk-rule-list">
          <li>按规范化后的站点 URL 与 API Key 去重；已有记录执行更新，不重复追加。</li>
          <li>目标分组不存在时自动创建。记录始终保留在全部密钥中，并额外分配到目标分组。</li>
          <li>未提供文字名称时使用 URL 的 host 作为站点名，端口会保留。</li>
          <li>优先按现有压缩导入格式解析；格式不匹配时自动进入智能提取模式。</li>
        </ul>
      </section>

      <section class="sdk-section">
        <div class="sdk-section-head">
          <h3>成功响应</h3>
          <a-tooltip title="复制响应示例">
            <button type="button" class="sdk-icon-button" aria-label="复制响应示例" @click="copyContent(successResponseExample, '响应示例已复制')">
              <CopyOutlined />
            </button>
          </a-tooltip>
        </div>
        <pre class="sdk-code"><code>{{ successResponseExample }}</code></pre>
        <div class="sdk-table-wrap">
          <table class="sdk-table">
            <thead>
              <tr>
                <th>字段</th>
                <th>类型</th>
                <th>说明</th>
              </tr>
            </thead>
            <tbody>
              <tr><td><code>mode</code></td><td>string</td><td><code>package</code> 或 <code>smart</code>，表示本次采用的解析模式。</td></tr>
              <tr><td><code>importedCount</code></td><td>number</td><td>本次创建与更新的记录总数。</td></tr>
              <tr><td><code>createdCount</code></td><td>number</td><td>新建记录数。</td></tr>
              <tr><td><code>updatedCount</code></td><td>number</td><td>覆盖更新的已有记录数。</td></tr>
              <tr><td><code>targetGroupName</code></td><td>string</td><td>实际写入的目标分组名称。</td></tr>
              <tr><td><code>groupCreated</code></td><td>boolean</td><td>目标分组是否由本次请求创建。</td></tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="sdk-section">
        <h3>状态码</h3>
        <div class="sdk-status-grid">
          <div><code>200</code><span>导入完成</span></div>
          <div><code>400</code><span>JSON 无效或缺少剪贴板文本</span></div>
          <div><code>405</code><span>请求方法不是 POST</span></div>
          <div><code>413</code><span>请求体超过 4 MiB</span></div>
          <div><code>422</code><span>未识别到有效记录</span></div>
          <div><code>503</code><span>桌面前端未就绪或 15 秒内未响应</span></div>
        </div>
      </section>

      <footer class="sdk-runtime-note">
        接口仅监听本机回环地址。调用时桌面程序需保持运行且主界面已加载完成。
      </footer>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { CopyOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';

const DEFAULT_SERVER_URL = 'http://127.0.0.1:8888';
const API_PATH = '/api/key-management/clipboard-import';

const serverUrl = ref(DEFAULT_SERVER_URL);
const exampleMode = ref('JSON');
const exampleModes = ['JSON', 'cURL'];

const endpointUrl = computed(() => `${serverUrl.value.replace(/\/+$/, '')}${API_PATH}`);

const requestPayload = {
  targetGroupName: 'Grok 福利',
  clipboardText: 'grok 500刀\nhttps://api.example.com/v1\nsk-example1234567890',
};

const requestExample = computed(() => {
  if (exampleMode.value === 'cURL') {
    const serializedPayload = JSON.stringify(requestPayload);
    return [
      `curl --request POST "${endpointUrl.value}"`,
      '  --header "Content-Type: application/json"',
      `  --data '${serializedPayload}'`,
    ].join(' \\\n');
  }
  return JSON.stringify(requestPayload, null, 2);
});

const successResponseExample = JSON.stringify({
  success: true,
  mode: 'smart',
  importedCount: 1,
  createdCount: 1,
  updatedCount: 0,
  targetGroupName: 'Grok 福利',
  groupCreated: true,
}, null, 2);

async function loadServerUrl() {
  const getter = window?.go?.main?.App?.GetBridgeImportSnapshot;
  if (typeof getter !== 'function') return;
  try {
    const snapshot = await getter();
    const resolved = String(snapshot?.serverUrl || '').trim();
    if (/^https?:\/\/127\.0\.0\.1:\d+$/i.test(resolved)) {
      serverUrl.value = resolved;
    }
  } catch {}
}

async function writeClipboardText(content) {
  const text = String(content || '');
  if (navigator?.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  textarea.remove();
}

async function copyContent(content, successMessage) {
  try {
    await writeClipboardText(content);
    message.success(successMessage);
  } catch {
    message.error('复制失败');
  }
}

onMounted(() => {
  void loadServerUrl();
});
</script>

<style scoped>
.panel-sdk {
  display: grid;
  grid-template-columns: 224px minmax(0, 1fr);
  height: min(560px, 68vh);
  border: 1px solid #e3e7eb;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
  color: #20262d;
}

.sdk-api-sidebar {
  min-width: 0;
  min-height: 0;
  padding: 16px 12px;
  border-right: 1px solid #e3e7eb;
  background: #f5f7f8;
}

.sdk-api-sidebar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px 14px;
  color: #30363d;
  font-size: 13px;
  font-weight: 700;
}

.sdk-api-count {
  min-width: 22px;
  height: 20px;
  padding: 0 6px;
  border-radius: 8px;
  background: #e4e9ee;
  color: #5b6570;
  font-size: 11px;
  line-height: 20px;
  text-align: center;
}

.sdk-api-group {
  padding: 8px;
  color: #7a838d;
  font-size: 11px;
  font-weight: 700;
}

.sdk-api-item {
  width: 100%;
  min-height: 58px;
  padding: 10px;
  border: 1px solid transparent;
  border-radius: 6px;
  display: flex;
  align-items: flex-start;
  gap: 9px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.sdk-api-item:hover,
.sdk-api-item.is-active {
  border-color: #cbd7e2;
  background: #fff;
}

.sdk-api-item.is-active {
  box-shadow: inset 3px 0 0 #2877c7;
}

.sdk-api-item-copy {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.sdk-api-item-copy strong {
  overflow-wrap: anywhere;
  font-size: 12px;
}

.sdk-api-item-copy small {
  overflow-wrap: anywhere;
  color: #7a838d;
  font-size: 10px;
}

.sdk-method {
  flex: 0 0 auto;
  padding: 2px 5px;
  border-radius: 4px;
  font: 700 10px/1.4 ui-monospace, SFMono-Regular, Consolas, monospace;
}

.sdk-method-post {
  background: #dcefe4;
  color: #187044;
}

.sdk-document {
  min-width: 0;
  min-height: 0;
  padding: 20px 24px 28px;
  overflow: auto;
}

.sdk-document-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 16px;
}

.sdk-document-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.sdk-document h2,
.sdk-document h3,
.sdk-document p {
  margin: 0;
}

.sdk-document h2 {
  font-size: 20px;
  line-height: 1.35;
}

.sdk-document h3 {
  font-size: 14px;
  line-height: 1.4;
}

.sdk-document-head p {
  margin-top: 7px;
  color: #68727d;
  font-size: 12px;
  line-height: 1.7;
}

.sdk-runtime-badge {
  flex: 0 0 auto;
  padding: 4px 8px;
  border: 1px solid #d9c9a8;
  border-radius: 6px;
  background: #fff7e8;
  color: #8b5b12;
  font-size: 11px;
}

.sdk-endpoint {
  min-height: 42px;
  padding: 8px 8px 8px 12px;
  border: 1px solid #dce2e8;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: #f7f9fb;
}

.sdk-endpoint code {
  min-width: 0;
  overflow-wrap: anywhere;
  color: #1e5f9f;
  font-size: 12px;
}

.sdk-icon-button {
  width: 30px;
  height: 30px;
  padding: 0;
  border: 1px solid #d7dde3;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  color: #4d5965;
  cursor: pointer;
}

.sdk-icon-button:hover {
  border-color: #91b7db;
  color: #1768ad;
}

.sdk-section {
  padding: 20px 0;
  border-bottom: 1px solid #eaedf0;
}

.sdk-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.sdk-section > h3 {
  margin-bottom: 12px;
}

.sdk-example-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sdk-table-wrap {
  width: 100%;
  overflow-x: auto;
}

.sdk-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 12px;
  line-height: 1.6;
}

.sdk-table th,
.sdk-table td {
  padding: 9px 10px;
  border-bottom: 1px solid #e5e9ed;
  vertical-align: top;
  text-align: left;
  overflow-wrap: anywhere;
}

.sdk-table th {
  background: #f5f7f8;
  color: #59636e;
  font-size: 11px;
  font-weight: 700;
}

.sdk-table th:nth-child(1) {
  width: 25%;
}

.sdk-table th:nth-child(2) {
  width: 14%;
}

.sdk-table th:nth-child(3) {
  width: 11%;
}

.sdk-table code,
.sdk-field-aliases code,
.sdk-status-grid code {
  color: #1e5f9f;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
}

.sdk-field-aliases {
  margin-top: 10px !important;
  color: #6b7580;
  font-size: 11px;
  line-height: 1.7;
}

.sdk-code {
  max-width: 100%;
  margin: 0;
  padding: 14px 16px;
  border: 1px solid #303943;
  border-radius: 6px;
  overflow: auto;
  background: #161b22;
  color: #e6edf3;
  font: 12px/1.7 ui-monospace, SFMono-Regular, Consolas, monospace;
  white-space: pre;
}

.sdk-rule-list {
  margin: 0;
  padding-left: 19px;
  color: #4f5964;
  font-size: 12px;
  line-height: 1.8;
}

.sdk-status-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  overflow: hidden;
  border: 1px solid #e1e5e9;
  border-radius: 6px;
  background: #e1e5e9;
}

.sdk-status-grid > div {
  min-width: 0;
  padding: 9px 10px;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 8px;
  background: #fff;
  font-size: 11px;
  line-height: 1.6;
}

.sdk-status-grid span {
  overflow-wrap: anywhere;
  color: #59636e;
}

.sdk-runtime-note {
  padding-top: 16px;
  color: #7a5a26;
  font-size: 11px;
  line-height: 1.7;
}

:global(body.dark-mode) .panel-sdk,
:global(body.gaia-dark) .panel-sdk {
  border-color: #39434b;
  background: #171d21;
  color: #e8edf1;
}

:global(body.dark-mode) .sdk-api-sidebar,
:global(body.gaia-dark) .sdk-api-sidebar {
  border-color: #39434b;
  background: #11171b;
}

:global(body.dark-mode) .sdk-api-item:hover,
:global(body.dark-mode) .sdk-api-item.is-active,
:global(body.gaia-dark) .sdk-api-item:hover,
:global(body.gaia-dark) .sdk-api-item.is-active {
  border-color: #47545f;
  background: #1d252b;
}

:global(body.dark-mode) .sdk-api-sidebar-head,
:global(body.gaia-dark) .sdk-api-sidebar-head {
  color: #e8edf1;
}

:global(body.dark-mode) .sdk-api-count,
:global(body.gaia-dark) .sdk-api-count {
  background: #2a343c;
  color: #b8c3cc;
}

:global(body.dark-mode) .sdk-document-head p,
:global(body.dark-mode) .sdk-field-aliases,
:global(body.dark-mode) .sdk-rule-list,
:global(body.gaia-dark) .sdk-document-head p,
:global(body.gaia-dark) .sdk-field-aliases,
:global(body.gaia-dark) .sdk-rule-list {
  color: #aeb9c1;
}

:global(body.dark-mode) .sdk-endpoint,
:global(body.gaia-dark) .sdk-endpoint {
  border-color: #3d4952;
  background: #11171b;
}

:global(body.dark-mode) .sdk-icon-button,
:global(body.gaia-dark) .sdk-icon-button {
  border-color: #46515a;
  background: #20282e;
  color: #c6d0d7;
}

:global(body.dark-mode) .sdk-section,
:global(body.gaia-dark) .sdk-section {
  border-color: #343e46;
}

:global(body.dark-mode) .sdk-table th,
:global(body.gaia-dark) .sdk-table th {
  background: #20282e;
  color: #b9c4cc;
}

:global(body.dark-mode) .sdk-table th,
:global(body.dark-mode) .sdk-table td,
:global(body.gaia-dark) .sdk-table th,
:global(body.gaia-dark) .sdk-table td {
  border-color: #39434b;
}

:global(body.dark-mode) .sdk-status-grid,
:global(body.gaia-dark) .sdk-status-grid {
  border-color: #39434b;
  background: #39434b;
}

:global(body.dark-mode) .sdk-status-grid > div,
:global(body.gaia-dark) .sdk-status-grid > div {
  background: #1b2328;
}

:global(body.dark-mode) .sdk-status-grid span,
:global(body.gaia-dark) .sdk-status-grid span {
  color: #b4bec6;
}

@media (max-width: 760px) {
  .panel-sdk {
    grid-template-columns: 1fr;
    max-height: 72vh;
  }

  .sdk-api-sidebar {
    padding: 10px;
    border-right: 0;
    border-bottom: 1px solid #e3e7eb;
  }

  .sdk-api-sidebar-head,
  .sdk-api-group {
    display: none;
  }

  .sdk-api-item {
    min-height: 48px;
  }

  .sdk-document {
    padding: 18px 14px 24px;
  }

  .sdk-status-grid {
    grid-template-columns: 1fr;
  }
}
</style>
