<template>
  <div class="header">
    <button
      type="button"
      class="nav-item nav-button"
      :aria-label="isDarkMode ? '切换到浅色模式' : '切换到深色模式'"
      @click="$emit('toggle-theme')"
    >
      <span class="nav-icon">
        <svg
          v-if="!isDarkMode"
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="transparent"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="12" cy="12" r="4"></circle>
          <path d="M12 2v2"></path>
          <path d="M12 20v2"></path>
          <path d="m4.93 4.93 1.41 1.41"></path>
          <path d="m17.66 17.66 1.41 1.41"></path>
          <path d="M2 12h2"></path>
          <path d="M20 12h2"></path>
          <path d="m6.34 17.66-1.41 1.41"></path>
          <path d="m19.07 4.93-1.41 1.41"></path>
        </svg>
        <svg
          v-else
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="transparent"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"></path>
        </svg>
      </span>
      <span class="nav-label">主题</span>
    </button>

    <button
      type="button"
      class="nav-item nav-button"
      :class="{ 'nav-item-active': currentPage === 'batch' }"
      @click="navigate('/')"
    >
      <span class="nav-icon"><AppstoreAddOutlined /></span>
      <span class="nav-label">批量检测</span>
    </button>

    <button
      type="button"
      class="nav-item nav-button"
      :class="{ 'nav-item-active': currentPage === 'keys' }"
      @click="navigate('/keys')"
    >
      <span class="nav-icon"><KeyOutlined /></span>
      <span class="nav-label">密钥管理</span>
    </button>

    <button
      v-if="showSettings"
      type="button"
      class="nav-item nav-button"
      @click="$emit('settings')"
    >
      <span class="nav-icon"><SettingOutlined /></span>
      <span class="nav-label">设置</span>
    </button>

    <button
      v-if="showExperimental"
      type="button"
      class="nav-item nav-button"
      @click="$emit('experimental')"
    >
      <span class="nav-icon"><ExperimentOutlined /></span>
      <span class="nav-label">实验功能</span>
    </button>

    <button type="button" class="nav-item nav-button" @click="openGitHub">
      <span class="nav-icon"><GithubOutlined /></span>
      <span class="nav-label">GitHub</span>
    </button>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router';
import {
  AppstoreAddOutlined,
  ExperimentOutlined,
  GithubOutlined,
  KeyOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue';

defineEmits(['experimental', 'settings', 'toggle-theme']);

defineProps({
  currentPage: {
    type: String,
    default: '',
  },
  isDarkMode: {
    type: Boolean,
    default: false,
  },
  showExperimental: {
    type: Boolean,
    default: true,
  },
  showSettings: {
    type: Boolean,
    default: true,
  },
});

const router = useRouter();

const navigate = path => {
  if (router.currentRoute.value.path !== path) {
    router.push(path);
  }
};

const openGitHub = () => {
  window.open('https://github.com/jlwebs/api-check', '_blank', 'noopener');
};
</script>

<style scoped>
.header {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  gap: 18px;
  width: 100%;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.nav-item {
  min-width: 76px;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: #666;
  text-align: center;
  user-select: none;
  cursor: pointer;
  transition: color 0.2s ease, transform 0.2s ease;
}

.nav-button {
  background: none;
  border: 0;
  padding: 0;
  font: inherit;
}

.nav-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  font-size: 24px;
  line-height: 1;
}

.nav-label {
  font-size: 12px;
  line-height: 1;
  color: inherit;
  white-space: nowrap;
}

.nav-item:hover {
  color: #1677ff;
  transform: translateY(-1px);
}

.nav-item-active {
  color: #1677ff;
}

:deep(body.dark-mode) .nav-item {
  color: #aaa;
}

:deep(body.dark-mode) .nav-item:hover,
:deep(body.dark-mode) .nav-item-active {
  color: #40a9ff;
}

@media (max-width: 900px) {
  .header {
    justify-content: center;
    gap: 14px;
  }

  .nav-item {
    min-width: 64px;
  }
}
</style>
