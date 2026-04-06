const EXPLICIT_API_BASE_URL = String(import.meta.env.VITE_API_BASE_URL || '')
  .trim()
  .replace(/\/+$/, '');

export function isProbablyWailsRuntime() {
  return typeof window !== 'undefined' && (
    window.location.protocol === 'wails:' ||
    window.location.protocol === 'asset:' ||
    typeof window.go === 'object' ||
    typeof window.runtime === 'object'
  );
}

export function getRuntimeApiBaseUrl() {
  if (EXPLICIT_API_BASE_URL) {
    return EXPLICIT_API_BASE_URL;
  }

  if (isProbablyWailsRuntime()) {
    return 'http://127.0.0.1:3000';
  }

  return '';
}

export function resolveRuntimeApiUrl(input) {
  if (typeof input !== 'string') {
    return input;
  }

  if (!input.startsWith('/api/')) {
    return input;
  }

  const baseUrl = getRuntimeApiBaseUrl();
  return baseUrl ? `${baseUrl}${input}` : input;
}

export function apiFetch(input, init) {
  return fetch(resolveRuntimeApiUrl(input), init);
}
