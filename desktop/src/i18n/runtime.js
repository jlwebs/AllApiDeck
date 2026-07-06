import {
  DEFAULT_LANGUAGE,
  I18N_DYNAMIC_PATTERNS,
  I18N_TEXT_MAP,
  LANGUAGE_CHANGE_EVENT,
  LANGUAGE_OPTIONS,
  LANGUAGE_STORAGE_KEY,
  LEGACY_LOCALE_STORAGE_KEY,
} from './catalog.js';
import {
  I18N_SUPPLEMENTAL_DYNAMIC_PATTERNS,
  I18N_SUPPLEMENTAL_TEXT_MAP,
} from './supplemental.js';
import enMessages from '../locales/en.json';
import zhMessages from '../locales/zh.json';

export { LANGUAGE_CHANGE_EVENT };

const TEXT_ATTRS = ['title', 'placeholder', 'aria-label', 'alt'];
const SKIP_TEXT_NODE_PARENT_TAGS = new Set(['SCRIPT', 'STYLE', 'TEXTAREA', 'INPUT', 'CODE', 'PRE']);
const HAN_PATTERN = /[\u4e00-\u9fff]/;
const LOCALE_TEXT_MAP = buildLocaleTextMap();
const MERGED_TEXT_MAP = { ...LOCALE_TEXT_MAP, ...I18N_SUPPLEMENTAL_TEXT_MAP, ...I18N_TEXT_MAP };
const SORTED_TEXT_KEYS = Object.keys(MERGED_TEXT_MAP)
  .filter(key => HAN_PATTERN.test(key))
  .sort((a, b) => b.length - a.length);
const DYNAMIC_TEXT_PATTERNS = [
  ...I18N_DYNAMIC_PATTERNS,
  ...I18N_SUPPLEMENTAL_DYNAMIC_PATTERNS,
].map(item => ({
  regex: new RegExp(item.pattern),
  replacements: item.replacements || {},
}));
const FALLBACK_LANGUAGE = 'en';
let currentLanguage = DEFAULT_LANGUAGE;
let observer = null;
let pendingDomPass = 0;
let domTranslatorInstalled = false;

function buildLocaleTextMap() {
  const map = {};
  Object.entries(zhMessages).forEach(([key, value]) => {
    const source = String(value ?? '').trim();
    if (!source || !HAN_PATTERN.test(source)) return;
    const english = String(enMessages[key] ?? '').trim();
    if (!english || english === source) return;
    map[source] = {
      ...(map[source] || {}),
      en: english,
    };
  });
  return map;
}

function normalizeLegacyLanguage(value) {
  const normalized = String(value || '').trim().toLowerCase();
  if (normalized === 'zh' || normalized === 'zh-cn' || normalized === 'cn' || normalized === 'simplified') return 'zh-CN';
  if (normalized === 'zh-tw' || normalized === 'zh-hk' || normalized === 'tw' || normalized === 'traditional') return 'zh-TW';
  if (normalized === 'en' || normalized === 'en-us' || normalized === 'english') return 'en';
  if (normalized === 'ja' || normalized === 'jp' || normalized === 'japanese') return 'ja';
  if (normalized === 'ko' || normalized === 'kr' || normalized === 'korean') return 'ko';
  if (normalized === 'hi' || normalized === 'in' || normalized === 'india' || normalized === 'hindi') return 'hi';
  if (normalized === 'ar' || normalized === 'arabic') return 'ar';
  return '';
}

export function normalizeLanguage(value) {
  const normalized = normalizeLegacyLanguage(value);
  if (normalized) return normalized;
  return FALLBACK_LANGUAGE;
}

function getNavigatorLanguages() {
  if (typeof navigator === 'undefined') return [];
  const values = [];
  if (Array.isArray(navigator.languages)) {
    values.push(...navigator.languages);
  }
  values.push(navigator.language, navigator.userLanguage, navigator.browserLanguage, navigator.systemLanguage);
  return values.map(item => String(item || '').trim()).filter(Boolean);
}

function detectSystemLanguage() {
  for (const item of getNavigatorLanguages()) {
    const normalized = String(item || '').trim().toLowerCase().replace(/_/g, '-');
    if (!normalized) continue;
    if (normalized.startsWith('zh')) {
      return /(?:^|-)hant(?:-|$)|(?:^|-)(tw|hk|mo)(?:-|$)/.test(normalized) ? 'zh-TW' : 'zh-CN';
    }
    if (normalized.startsWith('en')) return 'en';
    if (normalized.startsWith('ja') || normalized.startsWith('jp')) return 'ja';
    if (normalized.startsWith('ko') || normalized.startsWith('kr')) return 'ko';
    if (normalized.startsWith('hi')) return 'hi';
    if (normalized.startsWith('ar')) return 'ar';
  }
  return FALLBACK_LANGUAGE;
}

export function toVueI18nLocale(language) {
  const normalized = normalizeLanguage(language);
  if (normalized === 'zh-CN') return 'zh';
  if (normalized === 'zh-TW') return 'zh-TW';
  return normalized;
}

export function toLegacyLocale(language) {
  const normalized = normalizeLanguage(language);
  if (normalized === 'zh-CN') return 'zh';
  return normalized;
}

export function isDefaultLanguage(language = currentLanguage) {
  return normalizeLanguage(language) === DEFAULT_LANGUAGE;
}

function getStoredRawLanguage() {
  try {
    return localStorage.getItem(LANGUAGE_STORAGE_KEY)
      || localStorage.getItem(LEGACY_LOCALE_STORAGE_KEY)
      || detectSystemLanguage();
  } catch {
    return detectSystemLanguage();
  }
}

export function getStoredLanguage() {
  return normalizeLanguage(getStoredRawLanguage());
}

function persistLanguage(language) {
  const normalized = normalizeLanguage(language);
  try {
    localStorage.setItem(LANGUAGE_STORAGE_KEY, normalized);
    localStorage.setItem(LEGACY_LOCALE_STORAGE_KEY, toLegacyLocale(normalized));
  } catch {}
  return normalized;
}

function setDocumentLanguage(language) {
  if (typeof document === 'undefined') return;
  const normalized = normalizeLanguage(language);
  if (document.documentElement) {
    document.documentElement.lang = normalized === 'zh-CN' ? 'zh-CN' : normalized;
    document.documentElement.dir = normalized === 'ar' ? 'rtl' : 'ltr';
    document.documentElement.dataset.language = normalized;
  }
  if (document.body) {
    document.body.dataset.language = normalized;
  }
}

export function getCurrentLanguage() {
  return currentLanguage;
}

export function getLanguageOptions() {
  return LANGUAGE_OPTIONS.map(option => ({ ...option }));
}

export function translateText(text, language = currentLanguage) {
  const source = String(text ?? '');
  if (!source || isDefaultLanguage(language)) return source;
  const normalized = normalizeLanguage(language);
  const exact = getMappedTranslation(MERGED_TEXT_MAP[source], normalized);
  if (exact) return exact;
  const dynamicExact = translateDynamicText(source, normalized);
  if (dynamicExact !== source) return dynamicExact;
  const trimmed = source.trim();
  if (!trimmed) return source;
  const trimmedTranslation = getMappedTranslation(MERGED_TEXT_MAP[trimmed], normalized);
  if (trimmedTranslation) return source.replace(trimmed, trimmedTranslation);
  const dynamicTrimmed = translateDynamicText(trimmed, normalized);
  if (dynamicTrimmed !== trimmed) return source.replace(trimmed, dynamicTrimmed);
  return source;
}

export function tr(text, language = currentLanguage) {
  return translateText(text, language) || text;
}

function splitInterpolatedSegments(source) {
  return String(source || '').split(/(\s+|[：:，,。；;（）()[\]{}<>《》“”"']+)/g).filter(Boolean);
}

function applyMappedPhrases(source, language) {
  let translated = String(source ?? '');
  let changed = true;
  let guard = 0;
  while (changed && guard < 200) {
    changed = false;
    guard += 1;
    for (const key of SORTED_TEXT_KEYS) {
      if (!translated.includes(key) || !canReplacePhrase(translated, key)) continue;
      const replacement = getMappedTranslation(MERGED_TEXT_MAP[key], language);
      if (!replacement) continue;
      const next = replaceFirstPhrase(translated, key, replacement);
      if (next !== translated) {
        translated = next;
        changed = true;
        break;
      }
    }
  }
  return translated;
}

function isAsciiWordChar(char) {
  return /[A-Za-z0-9_]/.test(char || '');
}

function canReplaceAt(text, index, phrase) {
  const before = text[index - 1] || '';
  const after = text[index + phrase.length] || '';
  const startsAscii = isAsciiWordChar(phrase[0]);
  const endsAscii = isAsciiWordChar(phrase[phrase.length - 1]);
  if (startsAscii && isAsciiWordChar(before)) return false;
  if (endsAscii && isAsciiWordChar(after)) return false;
  return true;
}

function canReplacePhrase(text, phrase) {
  let index = text.indexOf(phrase);
  while (index !== -1) {
    if (canReplaceAt(text, index, phrase)) return true;
    index = text.indexOf(phrase, index + phrase.length);
  }
  return false;
}

function replacePhrase(text, phrase, replacement) {
  let result = '';
  let cursor = 0;
  let index = text.indexOf(phrase);
  while (index !== -1) {
    if (canReplaceAt(text, index, phrase)) {
      result += text.slice(cursor, index) + replacement;
      cursor = index + phrase.length;
    }
    index = text.indexOf(phrase, index + phrase.length);
  }
  return result + text.slice(cursor);
}

function replaceFirstPhrase(text, phrase, replacement) {
  let index = text.indexOf(phrase);
  while (index !== -1) {
    if (canReplaceAt(text, index, phrase)) {
      return text.slice(0, index) + replacement + text.slice(index + phrase.length);
    }
    index = text.indexOf(phrase, index + phrase.length);
  }
  return text;
}

function translateDynamicText(source, language) {
  const original = String(source ?? '');
  if (!original) return original;
  for (const item of DYNAMIC_TEXT_PATTERNS) {
    const replacement = getMappedTranslation(item.replacements, language);
    if (!replacement || !item.regex.test(original)) continue;
    return original.replace(item.regex, replacement);
  }
  return original;
}

function getMappedTranslation(entry, language) {
  if (!entry || typeof entry !== 'object') return '';
  const normalized = normalizeLanguage(language);
  return entry[normalized] || entry.en || '';
}

function translateMixedText(source, language = currentLanguage) {
  const original = String(source ?? '');
  if (!original || isDefaultLanguage(language) || !HAN_PATTERN.test(original)) return original;
  const direct = translateText(original, language);
  if (direct !== original) return direct;
  const phraseTranslated = applyMappedPhrases(original, normalizeLanguage(language));
  if (phraseTranslated !== original) return phraseTranslated;
  if (shouldKeepUnmappedLongText(original)) return original;
  const segments = splitInterpolatedSegments(original);
  if (segments.length <= 1) return original;
  let changed = false;
  const translated = segments.map(segment => {
    const next = translateText(segment, language);
    if (next !== segment) changed = true;
    return next;
  }).join('');
  return changed ? translated : original;
}

function shouldKeepUnmappedLongText(text) {
  const source = String(text || '').trim();
  if (source.length < 18) return false;
  if (/^[\u4e00-\u9fffA-Za-z0-9\s`"'“”‘’：:，,。；;、/().（）~\-]+$/.test(source)) {
    return true;
  }
  return false;
}

function resolveSourceValue(target, cacheName, value) {
  if (!target || typeof target !== 'object') return String(value ?? '');
  const current = target[cacheName];
  const source = String(value ?? '');
  if (typeof current === 'string' && current) {
    if (isDefaultLanguage(currentLanguage) && !HAN_PATTERN.test(source) && HAN_PATTERN.test(current)) {
      return current;
    }
    const translatedCurrent = translateMixedText(current, currentLanguage);
    if (!HAN_PATTERN.test(source) && source !== translatedCurrent) {
      target[cacheName] = source;
      return source;
    }
    if (
      HAN_PATTERN.test(source)
      && source !== current
      && source !== translatedCurrent
      && translateMixedText(source, currentLanguage) !== translatedCurrent
    ) {
      target[cacheName] = source;
      return source;
    }
    return current;
  }
  target[cacheName] = source;
  return source;
}

function translateTextNode(node, language) {
  const parent = node?.parentElement;
  if (!parent || SKIP_TEXT_NODE_PARENT_TAGS.has(parent.tagName)) return;
  const source = resolveSourceValue(node, '__aadI18nSourceText', node.nodeValue);
  if (!HAN_PATTERN.test(source)) return;
  const translated = isDefaultLanguage(language) ? source : translateMixedText(source, language);
  if (node.nodeValue !== translated) {
    node.nodeValue = translated;
  }
}

function translateElementAttrs(element, language) {
  if (!element || element.nodeType !== 1) return;
  const tagName = element.tagName;
  if (tagName === 'SCRIPT' || tagName === 'STYLE') return;
  TEXT_ATTRS.forEach(attr => {
    if (!element.hasAttribute(attr)) return;
    const value = element.getAttribute(attr);
    const source = resolveSourceValue(element, `__aadI18nAttr_${attr}`, value);
    if (!HAN_PATTERN.test(source)) return;
    const translated = isDefaultLanguage(language) ? source : translateMixedText(source, language);
    if (value !== translated) {
      element.setAttribute(attr, translated);
    }
  });
}

function translateNodeTree(root, language = currentLanguage) {
  if (typeof document === 'undefined' || !root) return;
  if (root.nodeType === 3) {
    translateTextNode(root, language);
    return;
  }
  if (root.nodeType !== 1 && root.nodeType !== 9 && root.nodeType !== 11) return;
  if (root.nodeType === 1) {
    translateElementAttrs(root, language);
  }
  const walker = document.createTreeWalker(
    root,
    NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT,
    {
      acceptNode(node) {
        if (node.nodeType === 1) {
          const tagName = node.tagName;
          if (tagName === 'SCRIPT' || tagName === 'STYLE') return NodeFilter.FILTER_REJECT;
          return NodeFilter.FILTER_ACCEPT;
        }
        if (node.nodeType === 3) {
          const parent = node.parentElement;
          if (!parent || SKIP_TEXT_NODE_PARENT_TAGS.has(parent.tagName)) return NodeFilter.FILTER_REJECT;
          return HAN_PATTERN.test(node.nodeValue || '') ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP;
        }
        return NodeFilter.FILTER_SKIP;
      },
    }
  );
  let node = walker.nextNode();
  while (node) {
    if (node.nodeType === 1) {
      translateElementAttrs(node, language);
    } else if (node.nodeType === 3) {
      translateTextNode(node, language);
    }
    node = walker.nextNode();
  }
}

export function scheduleDomTranslation(root = null) {
  if (typeof window === 'undefined' || typeof document === 'undefined') return;
  const target = root || document.body || document.documentElement;
  if (!target) return;
  if (pendingDomPass) {
    window.cancelAnimationFrame(pendingDomPass);
  }
  pendingDomPass = window.requestAnimationFrame(() => {
    pendingDomPass = 0;
    translateNodeTree(target, currentLanguage);
  });
}

export function installDomTranslator() {
  if (domTranslatorInstalled || typeof window === 'undefined' || typeof document === 'undefined') return;
  domTranslatorInstalled = true;
  setDocumentLanguage(currentLanguage);
  scheduleDomTranslation(document.body || document.documentElement);

  observer = new MutationObserver(mutations => {
    for (const mutation of mutations) {
      if (mutation.type === 'childList' && mutation.addedNodes?.length) {
        scheduleDomTranslation(mutation.target);
        return;
      }
      if (mutation.type === 'characterData' || mutation.type === 'attributes') {
        scheduleDomTranslation(mutation.target?.parentElement || mutation.target);
        return;
      }
    }
  });
  const target = document.body || document.documentElement;
  if (target) {
    observer.observe(target, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true,
      attributeFilter: TEXT_ATTRS,
    });
  }
}

export function applyLanguage(language, options = {}) {
  const { persist = true, dispatch = true, translateDom = true } = options;
  const normalized = normalizeLanguage(language);
  currentLanguage = normalized;
  if (persist) persistLanguage(normalized);
  setDocumentLanguage(normalized);
  if (translateDom) scheduleDomTranslation(document.body || document.documentElement);
  if (dispatch && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(LANGUAGE_CHANGE_EVENT, {
      detail: { language: normalized, locale: toVueI18nLocale(normalized) },
    }));
  }
  return normalized;
}

export function initializeLanguageRuntime(options = {}) {
  const normalized = applyLanguage(getStoredLanguage(), {
    persist: false,
    dispatch: false,
    translateDom: false,
    ...options,
  });
  if (options.installDom !== false) {
    installDomTranslator();
  }
  return normalized;
}
