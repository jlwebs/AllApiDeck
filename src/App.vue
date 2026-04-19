<template>
  <a-config-provider :theme="theme">
    <div class="app-shell">
      <router-view v-if="appReady" />
    </div>
  </a-config-provider>
</template>

<script>
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { GetLaunchMode } from '../wailsjs/go/main/App.js';
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

    onMounted(async () => {
      try {
        const mode = await GetLaunchMode();
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
