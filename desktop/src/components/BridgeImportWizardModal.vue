<template>
  <a-modal
    :open="open"
    title="当前浏览器标签直接导入"
    :footer="null"
    width="860px"
    :destroy-on-close="false"
    wrap-class-name="bridge-import-wizard-modal-wrap"
    @cancel="$emit('cancel')"
  >
    <div class="bridge-wizard">
      <section class="bridge-step-card bridge-step-card-accent">
        <div class="bridge-step-head">
          <span class="bridge-step-index">01</span>
          <div>
            <h3>配置油猴桥接脚本</h3>
            <p>先打开本地发布页安装脚本，安装完成后回到这里，再去目标站点标签页触发采集。</p>
          </div>
        </div>
        <div class="bridge-step-actions">
          <a-button type="primary" :loading="openingInstall" @click="$emit('open-install')">打开脚本发布页</a-button>
          <a-tag
            :color="installOpened ? 'success' : 'default'"
            class="bridge-install-status-tag"
          >
            {{ installOpened ? '发布页已打开' : '等待打开发布页' }}
          </a-tag>
        </div>
      </section>

      <section class="bridge-step-card">
        <div class="bridge-step-head">
          <span class="bridge-step-index">02</span>
          <div>
            <h3>通信测试与桥接接收</h3>
            <p>窗口会持续轮询本地桥接状态。只有脚本先通过握手，状态灯才会变绿；会话关闭后脚本会自动停止提交。</p>
          </div>
        </div>

        <div class="bridge-status-row">
          <a-tag v-if="opening" color="processing">扩展桥开放中....</a-tag>
          <a-tag v-else :color="polling ? 'processing' : 'default'">
            {{ polling ? '正在监听桥接提交' : '监听已暂停' }}
          </a-tag>
          <a-tag :color="sessionStatusColor">
            {{ sessionStatusText }}
          </a-tag>
          <a-tag color="blue">已处理 {{ readyCount }} 条</a-tag>
          <a-tag v-if="serverUrl" color="geekblue">服务 {{ serverUrl }}</a-tag>
          <a-tag v-if="lastReceivedAt" color="cyan">最后接收 {{ lastReceivedAt }}</a-tag>
          <a-tag v-if="lastClientPing" color="purple">脚本握手 {{ lastClientPing }}</a-tag>
        </div>

        <div v-if="logPath" class="bridge-meta-line">
          <span class="bridge-meta-label">日志文件</span>
          <code>{{ logPath }}</code>
        </div>

        <div class="bridge-record-stream">
          <template v-if="records.length">
            <div v-for="record in records" :key="record.id" class="bridge-record-chip">
              <a-tag :color="record.ready ? 'success' : recordStatusColor(record)">
                {{ record.ready ? 'ready' : recordStatusLabel(record) }}
              </a-tag>
              <span class="bridge-record-title">{{ record.title || '未命名页面' }}</span>
              <span class="bridge-record-origin">{{ record.sourceOrigin || record.sourceUrl || '-' }}</span>
              <span class="bridge-record-time">{{ record.receivedAt || '-' }}</span>
              <span class="bridge-record-meta">
                <span v-if="record.siteType">{{ record.siteType }}</span>
                <span v-if="Number(record.tokenCount || 0) > 0">{{ record.tokenCount }} keys</span>
                <span v-if="record.resolvedUser">uid {{ record.resolvedUser }}</span>
                <span v-if="!record.ready">{{ recordReadyReasonText(record) }}</span>
              </span>
            </div>
          </template>
          <div v-else class="bridge-record-empty">
            还没有收到脚本提交。安装完油猴脚本后，在目标站点标签页等待右侧桥接浮层检测完成并确认提交即可。
          </div>
        </div>

        <div class="bridge-log-panel">
          <div class="bridge-log-head">
            <span>最近桥接日志</span>
            <a-tag color="default">{{ lastLogs.length }} 条</a-tag>
          </div>
          <div v-if="lastLogs.length" class="bridge-log-stream">
            <div v-for="(line, index) in lastLogs" :key="`${index}-${line}`" class="bridge-log-line">{{ line }}</div>
          </div>
          <div v-else class="bridge-record-empty">
            还没有桥接日志输出。打开发布页、触发脚本或提交导入后，会实时显示在这里。
          </div>
        </div>
      </section>

      <section class="bridge-step-card bridge-step-card-final">
        <div class="bridge-step-head">
          <span class="bridge-step-index">03</span>
          <div>
            <h3>完成导入</h3>
            <p>本次会话已整理出 {{ readyCount }} 条可导入记录，确认后会直接进入标准导入主链，继续接入 key 与模型树。</p>
          </div>
        </div>
        <div class="bridge-step-actions">
          <a-button
            type="primary"
            size="large"
            :loading="importing"
            :disabled="readyCount === 0"
            @click="$emit('finish-import')"
          >
            全部接收完毕，导入
          </a-button>
        </div>
      </section>
    </div>
  </a-modal>
</template>

<script setup>
import { computed } from 'vue';

defineEmits(['cancel', 'open-install', 'finish-import']);

const props = defineProps({
  open: {
    type: Boolean,
    default: false,
  },
  openingInstall: {
    type: Boolean,
    default: false,
  },
  installOpened: {
    type: Boolean,
    default: false,
  },
  polling: {
    type: Boolean,
    default: false,
  },
  opening: {
    type: Boolean,
    default: false,
  },
  importing: {
    type: Boolean,
    default: false,
  },
  readyCount: {
    type: Number,
    default: 0,
  },
  lastReceivedAt: {
    type: String,
    default: '',
  },
  sessionActive: {
    type: Boolean,
    default: false,
  },
  clientReady: {
    type: Boolean,
    default: false,
  },
  lastClientPing: {
    type: String,
    default: '',
  },
  serverUrl: {
    type: String,
    default: '',
  },
  logPath: {
    type: String,
    default: '',
  },
  lastLogs: {
    type: Array,
    default: () => [],
  },
  records: {
    type: Array,
    default: () => [],
  },
});

const reasonTextMap = {
  prefetched_tokens: '已预取到账号内 key',
  access_token_contextual: '已获取登录态，等待后台补全',
  token_expired: '登录态已过期，请重新登录后重试',
  token_expired_local: '本地解析到登录态已过期，请重新登录后重试',
  not_logged_in: '当前页面未登录，请先登录站点',
  weak_access_token: '只抓到弱登录态，请在站点主界面重试',
  oauth_surface: '当前页是 OAuth 授权页，不是中转站主界面',
  cookie_only_nonrelay: '只抓到 Cookie，未发现中转站登录态',
  no_bridge_signal: '未发现可复用的中转站信号',
  missing_access_token_and_tokens: '缺少 access_token 且未抓到 key 列表',
  missing_site_url: '缺少站点地址',
  bridge_prefetch_failed: '预取失败，请查看日志',
};

const sessionStatusColor = computed(() => {
  if (!props.sessionActive) return 'default';
  return props.clientReady ? 'success' : 'error';
});

const sessionStatusText = computed(() => {
  if (!props.sessionActive) return '桥接会话已关闭';
  if (props.clientReady) return '脚本通信正常';
  return '等待脚本握手';
});

function normalizeReason(record) {
  return String(record?.readyReason || record?.payload?.extracted?.error || '').trim();
}

function recordReadyReasonText(record) {
  const reason = normalizeReason(record);
  return reasonTextMap[reason] || reason || '等待补全';
}

function recordStatusLabel(record) {
  const reason = normalizeReason(record);
  if (!reason) return 'pending';
  if (reason === 'access_token_contextual') return '待补全';
  return 'blocked';
}

function recordStatusColor(record) {
  const reason = normalizeReason(record);
  if (reason === 'access_token_contextual') return 'gold';
  if (reason === 'prefetched_tokens') return 'success';
  return 'red';
}
</script>

<style scoped>
.bridge-wizard{display:grid;gap:14px}
.bridge-step-card{padding:16px 18px;border-radius:18px;border:1px solid rgba(90,117,79,.1);background:rgba(255,255,255,.72);box-shadow:0 8px 20px rgba(98,119,84,.08),inset 0 1px 0 rgba(255,255,255,.7)}
.bridge-step-card-accent{background:linear-gradient(145deg,rgba(243,236,188,.92),rgba(245,226,154,.88))}
.bridge-step-card-final{background:linear-gradient(145deg,rgba(248,251,246,.92),rgba(237,245,226,.88))}
.bridge-step-head{display:flex;align-items:flex-start;gap:14px}
.bridge-step-index{width:34px;height:34px;border-radius:12px;display:inline-flex;align-items:center;justify-content:center;background:rgba(49,66,48,.08);color:#314230;font:700 12px/1 ui-monospace,SFMono-Regular,Menlo,monospace;flex:0 0 auto}
.bridge-step-head h3{margin:0;color:#314230;font:700 16px/1.15 Georgia,'Times New Roman',serif}
.bridge-step-head p{margin:6px 0 0;color:#697766;font-size:13px;line-height:1.5}
.bridge-step-actions{margin-top:14px;display:flex;align-items:center;gap:10px;flex-wrap:wrap}
.bridge-status-row{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-top:14px}
.bridge-meta-line{margin-top:10px;display:flex;align-items:center;gap:10px;min-width:0;font-size:12px;color:#667760}
.bridge-meta-label{padding:2px 8px;border-radius:999px;background:rgba(49,66,48,.08);color:#314230;white-space:nowrap}
.bridge-meta-line code{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.bridge-record-stream{margin-top:14px;max-height:220px;overflow:auto;display:grid;gap:8px;padding-right:4px}
.bridge-record-chip{display:grid;grid-template-columns:max-content minmax(0,1fr) minmax(140px,220px) max-content;align-items:center;gap:10px;padding:10px 12px;border-radius:14px;background:rgba(255,255,255,.72);border:1px solid rgba(90,117,79,.08)}
.bridge-record-title{min-width:0;color:#314230;font-weight:600}
.bridge-record-origin{min-width:0;color:#667760;font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.bridge-record-time{color:#94a38f;font-size:12px;white-space:nowrap}
.bridge-record-meta{display:flex;align-items:center;gap:8px;grid-column:2 / -1;color:#7b8a75;font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.bridge-log-panel{margin-top:14px;padding:12px 14px;border-radius:14px;border:1px solid rgba(90,117,79,.1);background:rgba(255,255,255,.62)}
.bridge-log-head{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:10px;color:#314230;font-weight:700}
.bridge-log-stream{max-height:180px;overflow:auto;display:grid;gap:6px}
.bridge-log-line{font:12px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace;color:#4b5a47;white-space:pre-wrap;word-break:break-word;padding:6px 8px;border-radius:10px;background:rgba(255,255,255,.72);border:1px solid rgba(90,117,79,.08)}
.bridge-record-empty{padding:18px 14px;border-radius:14px;border:1px dashed rgba(90,117,79,.16);color:#7a8675;background:rgba(255,255,255,.55)}
:deep(body.dark-mode) .bridge-step-card{background:rgba(255,255,255,.05);border-color:rgba(160,189,144,.14);box-shadow:0 10px 22px rgba(0,0,0,.16),inset 0 1px 0 rgba(255,255,255,.04)}
:deep(body.dark-mode) .bridge-step-card-accent{background:linear-gradient(145deg,rgba(104,75,12,.9),rgba(137,96,18,.86))}
:deep(body.dark-mode) .bridge-step-card-final{background:linear-gradient(145deg,rgba(255,255,255,.06),rgba(160,189,144,.06))}
:deep(body.dark-mode) .bridge-step-index{background:rgba(255,255,255,.08);color:#eef5e6}
:deep(body.dark-mode) .bridge-step-head h3{color:#eef5e6}
:deep(body.dark-mode) .bridge-step-head p,:deep(body.dark-mode) .bridge-record-origin,:deep(body.dark-mode) .bridge-record-time,:deep(body.dark-mode) .bridge-record-empty,:deep(body.dark-mode) .bridge-meta-line,:deep(body.dark-mode) .bridge-log-line,:deep(body.dark-mode) .bridge-record-meta{color:#b8c8b2}
:deep(body.dark-mode) .bridge-meta-label{background:rgba(255,255,255,.08);color:#eef5e6}
:deep(body.dark-mode) .bridge-log-panel,:deep(body.dark-mode) .bridge-log-line{background:rgba(255,255,255,.05);border-color:rgba(160,189,144,.14)}
:deep(body.dark-mode) .bridge-log-head{color:#eef5e6}
:deep(body.dark-mode) .bridge-record-chip{background:rgba(255,255,255,.05);border-color:rgba(160,189,144,.14)}
:deep(body.dark-mode) .bridge-record-title{color:#eef5e6}
:deep(body.dark-mode) .bridge-import-wizard-modal-wrap .ant-tag.ant-tag-default,
:deep(body.dark-mode) .bridge-import-wizard-modal-wrap .bridge-log-head .ant-tag.ant-tag-default{
  background:rgba(255,255,255,.08);
  border-color:rgba(160,189,144,.18);
  color:#dbe7d7 !important;
}
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn{
  border-color:rgba(154,191,142,.18);
  background:rgba(22,28,22,.94);
  color:#e4f1df;
  box-shadow:0 10px 20px rgba(0,0,0,.18);
}
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn:hover,
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn:focus-visible{
  border-color:rgba(154,191,142,.34);
  background:rgba(30,38,29,.98);
  color:#f1f8ec;
  box-shadow:0 12px 26px rgba(0,0,0,.22);
}
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn.ant-btn-primary{
  border-color:rgba(112,148,96,.88);
  background:linear-gradient(135deg,#58764f,#6a895e);
  color:#f3f8ef;
}
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn.ant-btn-primary:hover,
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn.ant-btn-primary:focus-visible{
  border-color:rgba(135,176,116,.96);
  background:linear-gradient(135deg,#64865a,#759867);
  color:#fbfdf9;
}
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn.ant-btn-primary[disabled],
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn.ant-btn-primary:disabled,
:deep(.bridge-import-wizard-modal-wrap) .bridge-step-actions .ant-btn.ant-btn-primary.ant-btn-disabled{
  border-color:rgba(123,145,113,.18) !important;
  background:rgba(38,46,38,.88) !important;
  color:rgba(228,241,223,.42) !important;
  box-shadow:none !important;
}
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.css-dev-only-do-not-override-1am763r,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.css-dev-only-do-not-override-1am763r.ant-btn-primary{
  border-color:rgba(88,118,128,.34) !important;
  background:linear-gradient(180deg,rgba(21,31,37,.96),rgba(15,23,28,.96)) !important;
  color:#dbe9e7 !important;
  box-shadow:0 12px 24px rgba(0,0,0,.26) !important;
}
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn:hover,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn:focus-visible,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary:hover,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary:focus-visible{
  border-color:rgba(111,151,164,.46) !important;
  background:linear-gradient(180deg,rgba(28,40,47,.98),rgba(19,29,35,.98)) !important;
  color:#edf6f5 !important;
}
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn[disabled],
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn:disabled,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-disabled,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary[disabled],
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary:disabled,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary.ant-btn-disabled,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.css-dev-only-do-not-override-1am763r[disabled],
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.css-dev-only-do-not-override-1am763r:disabled,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.css-dev-only-do-not-override-1am763r.ant-btn-disabled{
  border-color:rgba(78,102,111,.22) !important;
  background:linear-gradient(180deg,rgba(24,32,37,.94),rgba(18,24,28,.94)) !important;
  color:rgba(208,223,220,.38) !important;
  opacity:1 !important;
  box-shadow:none !important;
}
:deep(body.gaia-dark) :where(.bridge-import-wizard-modal-wrap) :where(.css-dev-only-do-not-override-1am763r).ant-btn-primary:disabled{
  border-color:rgba(78,102,111,.22) !important;
  color:rgba(208,223,220,.38) !important;
  background:#1f2a30 !important;
  background-color:#1f2a30 !important;
  background-image:none !important;
  box-shadow:none !important;
}
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .ant-tag.ant-tag-default,
:deep(body.gaia-dark) .bridge-import-wizard-modal-wrap .bridge-log-head .ant-tag.ant-tag-default{
  background:rgba(255,255,255,.08);
  border-color:rgba(95,128,138,.24);
  color:#d7e6e4 !important;
}
@media (max-width: 760px){
  .bridge-record-chip{grid-template-columns:1fr;justify-items:start}
}
</style>

<style>
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary:disabled,
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary.ant-btn-disabled,
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-step-actions .ant-btn.ant-btn-primary[disabled],
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-step-actions .css-dev-only-do-not-override-1am763r.ant-btn-primary:disabled,
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-step-actions .css-dev-only-do-not-override-1am763r.ant-btn-primary.ant-btn-disabled,
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-step-actions .css-dev-only-do-not-override-1am763r.ant-btn-primary[disabled] {
  background: #1f2a30 !important;
  background-color: #1f2a30 !important;
  background-image: none !important;
  border-color: rgba(78, 102, 111, 0.22) !important;
  color: rgba(208, 223, 220, 0.38) !important;
  box-shadow: none !important;
  opacity: 1 !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag.ant-tag-default,
body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-log-head .ant-tag.ant-tag-default {
  background: rgba(24, 32, 37, 0.92) !important;
  background-color: rgba(24, 32, 37, 0.92) !important;
  border-color: rgba(95, 128, 138, 0.24) !important;
  color: #d7e6e4 !important;
  box-shadow: none !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .bridge-install-status-tag.ant-tag.ant-tag-default {
  background: rgba(24, 32, 37, 0.92) !important;
  background-color: rgba(24, 32, 37, 0.92) !important;
  border-color: rgba(95, 128, 138, 0.24) !important;
  color: #d7e6e4 !important;
  box-shadow: none !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-blue,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-geekblue,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-cyan,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-purple,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-processing,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-success,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-error,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-gold,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-red {
  background: rgba(24, 32, 37, 0.92) !important;
  background-color: rgba(24, 32, 37, 0.92) !important;
  box-shadow: none !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-blue,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-processing {
  border-color: rgba(94, 137, 169, 0.42) !important;
  color: #8fc0e6 !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-geekblue,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-purple {
  border-color: rgba(116, 127, 189, 0.42) !important;
  color: #aab6f2 !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-cyan {
  border-color: rgba(92, 155, 155, 0.42) !important;
  color: #8fd9d1 !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-success {
  border-color: rgba(92, 150, 110, 0.4) !important;
  color: #9fd6ad !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-error,
body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-red {
  border-color: rgba(155, 96, 96, 0.4) !important;
  color: #e1a0a0 !important;
}

body.gaia-dark .bridge-import-wizard-modal-wrap .ant-tag-gold {
  border-color: rgba(169, 141, 84, 0.4) !important;
  color: #e3c98e !important;
}
</style>
