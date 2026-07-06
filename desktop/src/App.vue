<template>
  <a-config-provider :theme="theme">
    <div class="app-shell" :class="launchModeClass">
      <router-view v-if="appReady && !routeRenderError" />
      <div v-else-if="appReady" class="app-render-error-shell">
        <div class="app-render-error-panel">
          <div class="app-render-error-title">页面渲染异常</div>
          <div class="app-render-error-text">
            {{ routeRenderError?.message || '检测到未捕获错误，已阻断空白页。' }}
          </div>
          <a-button type="primary" size="small" @click="retryRouteRender">重试</a-button>
        </div>
      </div>
    </div>
  </a-config-provider>
</template>

<script>
import { computed, onBeforeUnmount, onErrorCaptured, onMounted, ref } from 'vue';
import { Modal, theme as antTheme } from 'ant-design-vue';
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
import { installSidebarRoutingDiagnostics, logClientDiagnostic } from './utils/clientDiagnostics.js';
import { clearCurrentLaunchMode, setCurrentLaunchMode } from './utils/launchModeState.js';
import {
  applyThemeMode,
  getStoredThemeMode,
  isDarkThemeMode,
  THEME_MODE_CHANGE_EVENT,
} from './utils/theme.js';

export default {
  name: 'App',
  setup() {
    const { t } = useI18n();
const router = useRouter();
const appReady = ref(false);
const launchMode = ref('');
const routeRenderError = ref(null);
const themeMode = ref(getStoredThemeMode());
    const theme = computed(() => ({
      algorithm: isDarkThemeMode(themeMode.value) ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
      token: themeMode.value === 'gaia-dark'
        ? {
          colorPrimary: '#69858e',
          colorInfo: '#69858e',
          colorSuccess: '#5f7b78',
          colorWarning: '#a57d5a',
        }
        : {},
    }));
    const ADVANCED_PROXY_DEFAULT_PROMPT_STORAGE_KEY = 'batch_api_check_advanced_proxy_defaults_prompt_seen_version_v1';
    let themeModeListener = null;

    const syncLaunchModeClasses = mode => {
      if (typeof document === 'undefined') return;
      const normalizedMode = String(mode || '').trim();
      const body = document.body;
      const root = document.documentElement;
      const modeClasses = ['launch-mode-main-window', 'launch-mode-panel-window', 'launch-mode-editor-window', 'launch-mode-ai-image-window', 'launch-mode-desktop-config-window', 'launch-mode-model-probe-window'];
      body?.classList?.remove(...modeClasses);
      root?.classList?.remove(...modeClasses);
      if (!normalizedMode) return;
      const nextClass = `launch-mode-${normalizedMode}-window`;
      body?.classList?.add(nextClass);
      root?.classList?.add(nextClass);
    };

    const clearLaunchModeClasses = () => {
      if (typeof document === 'undefined') return;
      const modeClasses = ['launch-mode-main-window', 'launch-mode-panel-window', 'launch-mode-editor-window', 'launch-mode-ai-image-window', 'launch-mode-desktop-config-window', 'launch-mode-model-probe-window'];
      document.body?.classList?.remove(...modeClasses);
      document.documentElement?.classList?.remove(...modeClasses);
    };

    const launchModeClass = computed(() => {
      const normalizedMode = String(launchMode.value || '').trim();
      return normalizedMode ? `launch-mode-${normalizedMode}-window` : '';
    });

    onErrorCaptured((error, instance, info) => {
      const message = error?.message || String(error || 'unknown error');
      routeRenderError.value = { message, info: String(info || 'render') };
      logClientDiagnostic('vue.error', `${String(info || 'render')} | ${message}`);
      console.error(error);
      return false;
    });

    const retryRouteRender = () => {
      routeRenderError.value = null;
      window.location.reload();
    };

    const consumePendingRouteAfterReload = () => {
      try {
        const pendingPath = String(sessionStorage.getItem('allapideck_pending_route_after_reload_v1') || '').trim();
        sessionStorage.removeItem('allapideck_pending_route_after_reload_v1');
        if (!pendingPath || pendingPath === '/' || !pendingPath.startsWith('/')) return '';
        return pendingPath;
      } catch {
        return '';
      }
    };

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
      themeMode.value = applyThemeMode(getStoredThemeMode(), { persist: false, dispatch: false });
      themeModeListener = event => {
        const nextMode = String(event?.detail?.mode || '').trim();
        if (!nextMode) return;
        themeMode.value = nextMode;
      };
      window.addEventListener(THEME_MODE_CHANGE_EVENT, themeModeListener);

      let mode = '';
      try {
        mode = await GetLaunchMode();
        launchMode.value = String(mode || '').trim();
        setCurrentLaunchMode(launchMode.value);
        syncLaunchModeClasses(launchMode.value);
        if (mode === 'panel' && router.currentRoute.value.path !== '/panel') {
          await router.replace('/panel');
        } else if (mode === 'editor' && router.currentRoute.value.path !== '/editor') {
          await router.replace('/editor');
        } else if (mode === 'ai-image' && router.currentRoute.value.path !== '/ai-image') {
          await router.replace('/ai-image');
        } else if (mode === 'desktop-config' && router.currentRoute.value.path !== '/desktop-config') {
          await router.replace('/desktop-config');
        } else if (mode === 'model-probe' && router.currentRoute.value.path !== '/sites') {
          await router.replace('/sites');
        }
        if (mode === '' || mode === 'main') {
          const pendingPath = consumePendingRouteAfterReload();
          if (pendingPath && router.currentRoute.value.fullPath !== pendingPath) {
            await router.replace(pendingPath);
          }
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

    onBeforeUnmount(() => {
      clearCurrentLaunchMode();
      if (themeModeListener) {
        window.removeEventListener(THEME_MODE_CHANGE_EVENT, themeModeListener);
        themeModeListener = null;
      }
      clearLaunchModeClasses();
    });

    return {
      appReady,
      launchModeClass,
      routeRenderError,
      retryRouteRender,
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
  position: relative;
  isolation: isolate;
}

.app-shell > * {
  position: relative;
  z-index: 1;
}

body.gaia-dark .app-shell::before {
  content: '';
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background:
    radial-gradient(circle at 14% 12%, rgba(104, 134, 146, 0.16), transparent 24%),
    radial-gradient(circle at 88% 10%, rgba(146, 110, 76, 0.12), transparent 18%),
    repeating-linear-gradient(120deg, rgba(255, 255, 255, 0.028) 0 1px, transparent 1px 150px),
    linear-gradient(180deg, rgba(255, 255, 255, 0.018), rgba(255, 255, 255, 0));
}

body.launch-mode-panel-window .app-shell,
body.launch-mode-editor-window .app-shell,
body.launch-mode-ai-image-window .app-shell {
  background: transparent !important;
}

body.launch-mode-panel-window .app-shell::before,
body.launch-mode-editor-window .app-shell::before,
body.launch-mode-ai-image-window .app-shell::before {
  display: none !important;
}

.app-render-error-shell {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.app-render-error-panel {
  width: min(560px, 100%);
  display: grid;
  gap: 10px;
  padding: 20px 18px;
  border: 1px solid rgba(92, 101, 96, 0.16);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.88);
  color: #263b2a;
}

.app-render-error-title {
  font-size: 16px;
  font-weight: 700;
}

.app-render-error-text {
  font-size: 12px;
  line-height: 1.6;
  word-break: break-word;
  color: #5f6f59;
}
</style>
