import { createApp } from 'vue';
import App from './App.vue';
import router from './router';
import { ConfigProvider } from 'ant-design-vue';
import 'ant-design-vue/dist/reset.css';
import i18n from './i18n';
import { initializeLanguageRuntime, LANGUAGE_CHANGE_EVENT, toVueI18nLocale } from './i18n/runtime.js';
import { installClientDiagnostics, logClientDiagnostic } from './utils/clientDiagnostics.js';
import { installRuntimeFetchBridge } from './utils/runtimeApi.js';
import { hydrateLastResultsSnapshotCache } from './utils/historySnapshotStore.js';
import { ensureStartupUpdateStatus } from './utils/appUpdateState.js';

installClientDiagnostics();
installRuntimeFetchBridge();
void hydrateLastResultsSnapshotCache();
void ensureStartupUpdateStatus();
initializeLanguageRuntime({ installDom: false });
logClientDiagnostic('bootstrap', 'fetch bridge install called');

const app = createApp(App);
logClientDiagnostic('bootstrap', 'vue app created');

app.use(router);
app.use(ConfigProvider);
app.use(i18n);
window.addEventListener(LANGUAGE_CHANGE_EVENT, event => {
  i18n.global.locale.value = toVueI18nLocale(event?.detail?.language);
});
logClientDiagnostic('bootstrap', 'plugins registered');
app.mount('#app');
initializeLanguageRuntime({ installDom: true, dispatch: true });
logClientDiagnostic('bootstrap', 'app mounted');
