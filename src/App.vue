<template>
  <a-config-provider :theme="theme">
    <router-view />
  </a-config-provider>
</template>

<script>
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { GetLaunchMode } from '../wailsjs/go/main/App.js';

export default {
  name: 'App',
  setup() {
    const { t } = useI18n();
    const router = useRouter();
    const theme = ref({
      primaryColor: '#1890ff',
    });

    onMounted(async () => {
      try {
        const mode = await GetLaunchMode();
        if (mode === 'panel' && router.currentRoute.value.path !== '/panel') {
          router.replace('/panel');
        } else if (mode === 'editor' && router.currentRoute.value.path !== '/editor') {
          router.replace('/editor');
        } else if (mode === 'desktop-config' && router.currentRoute.value.path !== '/desktop-config') {
          router.replace('/desktop-config');
        }
      } catch {}
    });

    return {
      theme,
      t,
    };
  },
};
</script>

<style>
@import './styles/global.css';
</style>
