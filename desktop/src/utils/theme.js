export const THEME_MODE_STORAGE_KEY = 'api_check_theme';
export const THEME_MODE_CHANGE_EVENT = 'batch-api-check:theme-mode-change';
export const THEME_MODE_OPTIONS = [
  {
    value: 'light',
    label: '浅色默认',
    description: '保留当前明亮工作台风格。',
  },
  {
    value: 'gaia-dark',
    label: '盖亚暗黑',
    description: '黑曜岩底色、冷青矿脉高光、去森林化的深色工作区。',
  },
];

export function normalizeThemeMode(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === 'gaia-dark' || normalized === 'dark') return 'gaia-dark';
  return 'light';
}

export function isDarkThemeMode(mode) {
  return normalizeThemeMode(mode) === 'gaia-dark';
}

export function getStoredThemeMode() {
  try {
    return normalizeThemeMode(localStorage.getItem(THEME_MODE_STORAGE_KEY));
  } catch {
    return 'light';
  }
}

export function getAppliedThemeMode() {
  if (typeof document !== 'undefined' && document.body) {
    if (document.body.classList.contains('gaia-dark')) return 'gaia-dark';
    if (document.body.classList.contains('dark-mode')) return 'gaia-dark';
    if (document.body.classList.contains('light-mode')) return 'light';
  }
  return getStoredThemeMode();
}

export function applyThemeMode(mode, options = {}) {
  const {
    persist = true,
    dispatch = true,
  } = options;
  const normalized = normalizeThemeMode(mode);

  if (persist) {
    try {
      localStorage.setItem(THEME_MODE_STORAGE_KEY, normalized);
    } catch {}
  }

  if (typeof document !== 'undefined' && document.body) {
    document.body.classList.toggle('dark-mode', normalized === 'gaia-dark');
    document.body.classList.toggle('light-mode', normalized === 'light');
    document.body.classList.toggle('gaia-dark', normalized === 'gaia-dark');
    document.body.dataset.themeMode = normalized;
  }

  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.dataset.themeMode = normalized;
  }

  if (dispatch && typeof window !== 'undefined' && typeof window.dispatchEvent === 'function') {
    window.dispatchEvent(new CustomEvent(THEME_MODE_CHANGE_EVENT, {
      detail: {
        mode: normalized,
        isDark: isDarkThemeMode(normalized),
      },
    }));
  }

  return normalized;
}

export function toggleTheme(isDarkMode) {
  const nextMode = isDarkMode?.value ? 'light' : 'gaia-dark';
  const appliedMode = applyThemeMode(nextMode);
  if (isDarkMode && typeof isDarkMode === 'object') {
    isDarkMode.value = isDarkThemeMode(appliedMode);
  }
  return appliedMode;
}
