const GLOBAL_KEY = '__BATCH_API_CHECK_LAUNCH_MODE__';
const MODE_CLASS_PREFIX = 'launch-mode-';
const MODE_CLASS_SUFFIX = '-window';

function normalizeLaunchMode(mode) {
  return String(mode || '').trim();
}

function readLaunchModeFromClasses() {
  if (typeof document === 'undefined') return '';
  const nodes = [document.body, document.documentElement];
  for (const node of nodes) {
    const classList = node?.classList;
    if (!classList) continue;
    for (const className of classList) {
      if (!className.startsWith(MODE_CLASS_PREFIX) || !className.endsWith(MODE_CLASS_SUFFIX)) continue;
      return className.slice(MODE_CLASS_PREFIX.length, -MODE_CLASS_SUFFIX.length);
    }
  }
  return '';
}

export function setCurrentLaunchMode(mode) {
  const normalized = normalizeLaunchMode(mode);
  if (typeof window !== 'undefined') {
    window[GLOBAL_KEY] = normalized;
  }
  return normalized;
}

export function clearCurrentLaunchMode() {
  if (typeof window !== 'undefined') {
    delete window[GLOBAL_KEY];
  }
}

export function getCurrentLaunchMode() {
  if (typeof window !== 'undefined') {
    const stored = normalizeLaunchMode(window[GLOBAL_KEY]);
    if (stored) return stored;
  }
  return normalizeLaunchMode(readLaunchModeFromClasses());
}

export function isCurrentLaunchMode(mode) {
  return getCurrentLaunchMode() === normalizeLaunchMode(mode);
}
