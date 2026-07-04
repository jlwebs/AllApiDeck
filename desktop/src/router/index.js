import { createRouter, createWebHistory } from 'vue-router';
import Home from '../views/Home.vue';
import Batch from '../views/Batch.vue';
import Layout from '../views/Layout.vue';

const loadKeysView = () => import('../views/Keys.vue');
const loadSitesView = () => import('../views/Sites.vue');

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
        component: Home,
      },
      {
        path: 'batch',
        component: Batch,
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
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

const runWhenIdle = callback => {
  if (typeof window === 'undefined') return;
  if (typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(callback, { timeout: 1200 });
    return;
  }
  window.setTimeout(callback, 120);
};

router.afterEach(to => {
  const name = String(to?.name || '').trim();
  if (name === 'Sites') {
    runWhenIdle(() => { void loadKeysView(); });
  } else if (name === 'Keys') {
    runWhenIdle(() => { void loadSitesView(); });
  }
});

export default router;
