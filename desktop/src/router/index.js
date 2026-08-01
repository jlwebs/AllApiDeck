import { createRouter, createWebHistory } from 'vue-router';
import Layout from '../views/Layout.vue';
import { applyLanguage, getStoredLanguage, scheduleDomTranslation } from '../i18n/runtime.js';

const loadKeysView = () => import('../views/Keys.vue');
const loadSitesView = () => import('../views/Sites.vue');
const loadHomeView = () => import('../views/Home.vue');
const loadBatchView = () => import('../views/Batch.vue');
const loadUsageView = () => import('../views/Usage.vue');

const routes = [
  {
    path: '/panel',
    component: () => import('../views/Panel.vue'),
  },
  {
    path: '/editor',
    component: () => import('../views/Editor.vue'),
  },
  {
    path: '/ai-image',
    component: () => import('../views/AIImage.vue'),
  },
  {
    path: '/desktop-config',
    component: () => import('../views/DesktopConfig.vue'),
  },
  {
    path: '/',
    component: Layout,
    children: [
      {
        path: '',
        redirect: '/keys',
      },
      {
        path: 'single',
        component: loadHomeView,
      },
      {
        path: 'batch',
        component: loadBatchView,
      },
      {
        path: 'keys',
        name: 'Keys',
        component: loadKeysView,
        meta: { keepAlive: true },
      },
      {
        path: 'sites',
        name: 'Sites',
        component: loadSitesView,
        meta: { keepAlive: true },
      },
      {
        path: 'usage',
        name: 'Usage',
        component: () => import('../views/Usage.vue'),
        meta: { keepAlive: true },
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

const runWhenIdle = callback => {
  if (typeof window === 'undefined') return;
  const connection = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
  if (connection?.saveData) return;
  if (Number(navigator.deviceMemory || 8) <= 4 || Number(navigator.hardwareConcurrency || 8) <= 4) return;
  if (typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(callback, { timeout: 4000 });
    return;
  }
  window.setTimeout(callback, 1600);
};

router.afterEach(to => {
  const language = getStoredLanguage();
  applyLanguage(language, { persist: false, dispatch: true, translateDom: false });
  scheduleDomTranslation(document.querySelector('.app-shell') || document.body || document.documentElement);
  const name = String(to?.name || '').trim();
  if (name === 'Sites') {
    runWhenIdle(() => { void loadKeysView(); });
  } else if (name === 'Keys') {
    runWhenIdle(() => { void loadSitesView(); });
  }
});

export default router;
