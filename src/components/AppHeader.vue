<template>
  <header class="spring-header">
    <button type="button" class="spring-brand" @click="navigate('/')">
      <span class="spring-brand-mark">
        <img :src="appLogo" alt="" class="spring-brand-icon" />
      </span>
      <span class="spring-brand-title">All API Deck</span>
    </button>

    <nav class="spring-toolbar">
      <button
        type="button"
        class="spring-pill"
        :class="{ 'spring-pill-active': currentPage === 'batch' }"
        @click="navigate('/')"
      >
        <AppstoreAddOutlined />
        <span>批量检测</span>
      </button>

      <button
        type="button"
        class="spring-pill"
        :class="{ 'spring-pill-active': currentPage === 'keys' }"
        @click="navigate('/keys')"
      >
        <KeyOutlined />
        <span>密钥管理</span>
      </button>

      <button
        v-if="showSettings"
        type="button"
        class="spring-pill"
        @click="$emit('settings')"
      >
        <SettingOutlined />
        <span>设置</span>
      </button>

      <button
        v-if="showExperimental"
        type="button"
        class="spring-pill spring-pill-icon-only"
        title="实验功能"
        aria-label="实验功能"
        @click="$emit('experimental')"
      >
        <ExperimentOutlined />
      </button>

      <button
        type="button"
        class="spring-pill spring-pill-ghost spring-pill-icon-only"
        :title="isDarkMode ? '切换到浅色模式' : '切换到深色模式'"
        :aria-label="isDarkMode ? '切换到浅色模式' : '切换到深色模式'"
        @click="$emit('toggle-theme')"
      >
        <span class="spring-theme-icon">
          <svg
            v-if="!isDarkMode"
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
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
            width="14"
            height="14"
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
      </button>

      <button type="button" class="spring-pill spring-pill-ghost" @click="openGitHub">
        <GithubOutlined />
        <span>GitHub</span>
      </button>
    </nav>
  </header>
</template>

<script setup>
import { useRouter } from 'vue-router';
import appLogo from '../assets/logo.png';
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

const navigate = async path => {
  if (router.currentRoute.value.path !== path) {
    router.push(path);
  }
};

const openGitHub = () => {
  window.open('https://github.com/jlwebs/AllApiDeck', '_blank', 'noopener');
};
</script>

<style scoped>
.spring-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  margin-bottom: 8px;
  padding: 8px 10px;
  border-radius: 18px;
  border: 1px solid rgba(77, 104, 73, 0.12);
  background:
    linear-gradient(135deg, rgba(255, 251, 242, 0.94), rgba(239, 246, 228, 0.84)),
    rgba(255, 255, 255, 0.76);
  box-shadow:
    0 10px 24px rgba(87, 107, 73, 0.07),
    inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.spring-brand {
  border: 0;
  background: transparent;
  padding: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  min-width: 0;
  text-align: left;
  flex: 0 0 auto;
}

.spring-brand-mark {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(160deg, #edf5d7, #bfd39a);
  overflow: hidden;
  box-shadow: 0 6px 14px rgba(86, 118, 76, 0.14);
}

.spring-brand-icon {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.spring-brand-title {
  color: #29412d;
  font: 700 14px/1.05 Georgia, 'Times New Roman', serif;
  white-space: nowrap;
}

.spring-toolbar {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.spring-pill {
  border: 1px solid rgba(77, 104, 73, 0.08);
  background: rgba(255, 255, 255, 0.62);
  color: #5e6f59;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border-radius: 999px;
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  white-space: nowrap;
  transition:
    transform 0.2s ease,
    background-color 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease;
}

.spring-pill-active {
  color: #28412c;
  background: linear-gradient(135deg, rgba(234, 243, 213, 0.98), rgba(214, 230, 188, 0.92));
  box-shadow: 0 8px 18px rgba(96, 122, 77, 0.12);
}

.spring-pill-ghost {
  background: rgba(255, 255, 255, 0.5);
}

.spring-pill-icon-only {
  width: 32px;
  min-width: 32px;
  padding: 0;
  gap: 0;
}

.spring-pill :deep(.anticon),
.spring-theme-icon {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  line-height: 1;
}

.spring-pill:hover {
  color: #28412c;
  background: rgba(239, 246, 226, 0.84);
  transform: translateY(-1px);
}

:deep(body.dark-mode) .spring-header {
  border-color: rgba(151, 184, 136, 0.14);
  background:
    linear-gradient(135deg, rgba(25, 38, 28, 0.94), rgba(40, 59, 43, 0.88)),
    rgba(21, 28, 22, 0.82);
  box-shadow:
    0 12px 28px rgba(0, 0, 0, 0.22),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:deep(body.dark-mode) .spring-brand-mark {
  background: linear-gradient(160deg, #486a4d, #314834);
  color: #edf7df;
}

:deep(body.dark-mode) .spring-brand-title,
:deep(body.dark-mode) .spring-pill {
  color: #eef6e6;
}

:deep(body.dark-mode) .spring-pill {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(168, 201, 147, 0.12);
}

:deep(body.dark-mode) .spring-pill-active {
  background: linear-gradient(135deg, rgba(96, 127, 88, 0.5), rgba(71, 97, 66, 0.44));
  color: #f7fcf1;
}

:deep(body.dark-mode) .spring-pill:hover {
  background: rgba(172, 199, 151, 0.12);
  color: #f7fcf1;
}

@media (max-width: 620px) {
  .spring-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .spring-toolbar {
    justify-content: flex-start;
  }
}
</style>
