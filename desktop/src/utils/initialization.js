import { appInfo, banner } from './info.js';
import { applyThemeMode, getStoredThemeMode, isDarkThemeMode } from './theme.js';

export function initializeTheme(isDarkMode) {
  const appliedMode = applyThemeMode(getStoredThemeMode(), { persist: false, dispatch: false });
  isDarkMode.value = isDarkThemeMode(appliedMode);
}

export function initializeLanguage(locale, currentLanguage) {
  const savedLocale = localStorage.getItem('locale');
  if (savedLocale) {
    locale.value = savedLocale;
  } else {
    locale.value = 'zh';
  }
}

export function initConsole() {
  const message = 'hello';
  console.log(
    `%c  API CHECK v${appInfo.version} %c  ${appInfo.officialUrl} `,
    'color: #fadfa3; background: #030307; padding:5px 0;',
    'background: #fadfa3; padding:5px 0;',
  );
  console.log(banner);
  console.log(message + location.href);
  console.log(appInfo.author.name + ':' + appInfo.author.url);
  console.log(appInfo.coauthor.name + ':' + appInfo.coauthor.url);
}
