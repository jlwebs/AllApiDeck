<template>
  <a-config-provider :theme="theme">
    <div class="app-shell">
      <router-view v-if="appReady" />
    </div>
  </a-config-provider>
</template>

<script>
import { onMounted, ref } from 'vue';
import { Modal } from 'ant-design-vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { GetLaunchMode } from '../wailsjs/go/main/App.js';
import { getCurrentAppVersion } from './utils/appUpdateState.js';
import {
  applyAdvancedProxyVersionedDefaultParameters,
  getAdvancedProxyConfig,
  hasAdvancedProxyVersionedDefaultMismatch,
  setAdvancedProxyConfig,
} from './utils/advancedProxyBridge.js';
import { installSidebarRoutingDiagnostics } from './utils/clientDiagnostics.js';

export default {
  name: 'App',
  setup() {
    const { t } = useI18n();
    const router = useRouter();
    const appReady = ref(false);
    const theme = ref({
      primaryColor: '#1890ff',
    });
    const ADVANCED_PROXY_DEFAULT_PROMPT_STORAGE_KEY = 'batch_api_check_advanced_proxy_defaults_prompt_seen_version_v1';

    const markAdvancedProxyDefaultPromptSeen = (version) => {
      try {
        localStorage.setItem(ADVANCED_PROXY_DEFAULT_PROMPT_STORAGE_KEY, String(version || '').trim());
      } catch {}
    };

    const getAdvancedProxyDefaultPromptSeenVersion = () => {
      try {
        return String(localStorage.getItem(ADVANCED_PROXY_DEFAULT_PROMPT_STORAGE_KEY) || '').trim();
      } catch {
        return '';
      }
    };

    const maybePromptAdvancedProxyVersionDefaults = async () => {
      const currentVersion = String(getCurrentAppVersion() || '').trim();
      if (!currentVersion || currentVersion === '0.0.0') {
        return;
      }
      if (getAdvancedProxyDefaultPromptSeenVersion() === currentVersion) {
        return;
      }

      const config = await getAdvancedProxyConfig();
      if (!hasAdvancedProxyVersionedDefaultMismatch(config)) {
        markAdvancedProxyDefaultPromptSeen(currentVersion);
        return;
      }

      Modal.confirm({
        title: '版本设置参数已更新，是否覆盖最新参数？',
        content: '仅覆盖高级代理的最新故障转移默认参数，provider、队列与接管配置会保留。',
        okText: '覆盖',
        cancelText: '保留',
        async onOk() {
          const nextConfig = applyAdvancedProxyVersionedDefaultParameters(config);
          await setAdvancedProxyConfig(nextConfig);
          markAdvancedProxyDefaultPromptSeen(currentVersion);
        },
        onCancel() {
          markAdvancedProxyDefaultPromptSeen(currentVersion);
        },
      });
    };

    onMounted(async () => {
      let mode = '';
      try {
        mode = await GetLaunchMode();
        if (mode === 'panel' && router.currentRoute.value.path !== '/panel') {
          await router.replace('/panel');
        } else if (mode === 'editor' && router.currentRoute.value.path !== '/editor') {
          await router.replace('/editor');
        } else if (mode === 'desktop-config' && router.currentRoute.value.path !== '/desktop-config') {
          await router.replace('/desktop-config');
        }
        if (mode !== 'panel') {
          installSidebarRoutingDiagnostics(mode || 'main');
        }
      } catch {}
      appReady.value = true;

      if (mode === '' || mode === 'main') {
        void maybePromptAdvancedProxyVersionDefaults();
      }
    });

    return {
      appReady,
      theme,
      t,
    };
  },
};
</script>

<style>
@import './styles/global.css';

.app-shell {
  min-height: 100vh;
  min-width: 0;
}
</style>
