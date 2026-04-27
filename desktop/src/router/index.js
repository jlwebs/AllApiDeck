import { createRouter, createWebHistory } from 'vue-router';
import Home from '../views/Home.vue';
import Batch from '../views/Batch.vue';
import Layout from '../views/Layout.vue';

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
    path: '/desktop-config',
    component: () => import('../views/DesktopConfig.vue'),
  },
  {
    path: '/',
    component: Layout,
    children: [
      {
        path: '',
        component: Batch,
      },
      {
        path: 'single',
        component: Home,
      },
      {
        path: 'batch',
        redirect: '/',
      },
      {
        path: 'keys',
        component: () => import('../views/Keys.vue'),
      },
      {
        path: 'sites',
        component: () => import('../views/Sites.vue'),
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
