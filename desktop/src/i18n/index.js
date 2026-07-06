import { createI18n } from 'vue-i18n';
import { legacyMessages } from './legacyMessages.js';
import { getStoredLanguage, toVueI18nLocale } from './runtime.js';

const i18n = createI18n({
  legacy: false,
  fallbackLocale: 'zh',
  missingWarn: false,
  fallbackWarn: false,
  locale: toVueI18nLocale(getStoredLanguage()),
  messages: legacyMessages,
  missing: (_, key) => key,
});

export default i18n;
