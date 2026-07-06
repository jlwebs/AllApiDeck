import en from '../locales/en.json';
import zh from '../locales/zh.json';
import { tr } from './runtime.js';

function buildDerivedMessages(language) {
  return Object.fromEntries(
    Object.entries(zh).map(([key, value]) => [
      key,
      tr(String(value ?? key), language),
    ])
  );
}

export const legacyMessages = {
  zh,
  en,
  'zh-TW': buildDerivedMessages('zh-TW'),
  ja: buildDerivedMessages('ja'),
  ko: buildDerivedMessages('ko'),
  hi: buildDerivedMessages('hi'),
  ar: buildDerivedMessages('ar'),
};
