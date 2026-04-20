import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { ConfigProvider } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';
import i18n from './i18n';
import { installClientDiagnostics, logClientDiagnostic } from './utils/clientDiagnostics.js';
import { installRuntimeFetchBridge } from './utils/runtimeApi.js';
import { hydrateLastResultsSnapshotCache } from './utils/historySnapshotStore.js';
import { ensureStartupUpdateStatus } from './utils/appUpdateState.js';

installClientDiagnostics();
installRuntimeFetchBridge();
void hydrateLastResultsSnapshotCache();
void ensureStartupUpdateStatus();
logClientDiagnostic('bootstrap', 'fetch bridge install called');

const app = createApp(App);
logClientDiagnostic('bootstrap', 'vue app created');

app.use(router);
app.use(ConfigProvider);
app.use(i18n);
logClientDiagnostic('bootstrap', 'plugins registered');
app.mount('#app');
logClientDiagnostic('bootstrap', 'app mounted');
