import { logClientDiagnostic } from './clientDiagnostics.js';

const APP_GITHUB_OWNER = String(typeof __APP_GITHUB_OWNER__ !== 'undefined' ? __APP_GITHUB_OWNER__ : 'jlwebs').trim();
const APP_GITHUB_REPO = String(typeof __APP_GITHUB_REPO__ !== 'undefined' ? __APP_GITHUB_REPO__ : 'AllApiDeck').trim();
const APP_GITHUB_URL = String(
  typeof __APP_GITHUB_URL__ !== 'undefined'
    ? __APP_GITHUB_URL__
    : `https://github.com/${APP_GITHUB_OWNER}/${APP_GITHUB_REPO}`,
).trim();
const APP_RELEASE_TAG = String(typeof __APP_RELEASE_TAG__ !== 'undefined' ? __APP_RELEASE_TAG__ : '').trim();
const APP_RELEASE_VERSION = normalizeVersion(
  typeof __APP_RELEASE_VERSION__ !== 'undefined' ? __APP_RELEASE_VERSION__ : '',
);

let startupUpdateCheckPromise = null;
let startupLatestReleasePayload = null;
let startupUpdateStatus = buildUpdateStatus({
  checked: false,
  hasUpdate: false,
  latestTag: '',
  latestVersion: '',
  htmlUrl: '',
  currentTag: APP_RELEASE_TAG,
  currentVersion: APP_RELEASE_VERSION,
});

function normalizeVersion(value) {
  return String(value || '')
    .trim()
    .replace(/^v/i, '')
    .replace(/[^0-9A-Za-z.+-].*$/, '');
}

function parseVersionParts(version) {
  return normalizeVersion(version)
    .split('.')
    .map(part => Number.parseInt(part, 10))
    .filter(part => Number.isFinite(part) && part >= 0);
}

function isNewerVersion(latest, current) {
  const latestParts = parseVersionParts(latest);
  const currentParts = parseVersionParts(current);
  if (!latestParts.length || !currentParts.length) {
    return false;
  }

  const maxLength = Math.max(latestParts.length, currentParts.length);
  for (let index = 0; index < maxLength; index += 1) {
    const latestValue = latestParts[index] || 0;
    const currentValue = currentParts[index] || 0;
    if (latestValue > currentValue) return true;
    if (latestValue < currentValue) return false;
  }
  return false;
}

function buildUpdateStatus(partial = {}) {
  return {
    checked: Boolean(partial.checked),
    hasUpdate: Boolean(partial.hasUpdate),
    latestTag: String(partial.latestTag || '').trim(),
    latestVersion: normalizeVersion(partial.latestVersion),
    htmlUrl: String(partial.htmlUrl || '').trim(),
    currentTag: String(partial.currentTag || APP_RELEASE_TAG).trim(),
    currentVersion: normalizeVersion(partial.currentVersion || APP_RELEASE_VERSION),
    error: String(partial.error || '').trim(),
  };
}

export function getAppGithubUrl() {
  return APP_GITHUB_URL;
}

export function getCurrentAppTag() {
  return String(APP_RELEASE_TAG || '').trim();
}

export function getCurrentAppVersion() {
  return APP_RELEASE_VERSION;
}

export function getStartupUpdateStatus() {
  return startupUpdateStatus;
}

export function getStartupLatestReleasePayload() {
  return startupLatestReleasePayload;
}

export async function ensureStartupUpdateStatus() {
  if (startupUpdateCheckPromise) {
    return startupUpdateCheckPromise;
  }

  startupUpdateCheckPromise = (async () => {
    const canCompareVersion = Boolean(APP_RELEASE_TAG && APP_RELEASE_VERSION && APP_RELEASE_VERSION !== '0.0.0');
    const apiUrl = `https://api.github.com/repos/${APP_GITHUB_OWNER}/${APP_GITHUB_REPO}/releases/latest`;
    logClientDiagnostic(
      'app.update',
      `startup check begin currentTag=${APP_RELEASE_TAG || 'n/a'} currentVersion=${APP_RELEASE_VERSION || 'n/a'} comparable=${canCompareVersion} repo=${APP_GITHUB_OWNER}/${APP_GITHUB_REPO}`,
    );

    try {
      const response = await fetch(apiUrl, {
        headers: {
          Accept: 'application/vnd.github+json',
        },
      });
      if (!response.ok) {
        throw new Error(`github_release_http_${response.status}`);
      }

      const payload = await response.json();
      startupLatestReleasePayload = payload && typeof payload === 'object' ? payload : null;
      const latestTag = String(payload?.tag_name || '').trim();
      const latestVersion = normalizeVersion(latestTag);
      const hasUpdate = canCompareVersion && isNewerVersion(latestVersion, APP_RELEASE_VERSION);

      startupUpdateStatus = buildUpdateStatus({
        checked: true,
        hasUpdate,
        latestTag,
        latestVersion,
        htmlUrl: String(payload?.html_url || APP_GITHUB_URL).trim(),
      });
      logClientDiagnostic(
        'app.update',
        `startup check done current=${startupUpdateStatus.currentVersion || 'n/a'} latest=${startupUpdateStatus.latestVersion || 'n/a'} hasUpdate=${startupUpdateStatus.hasUpdate}`,
      );
      return startupUpdateStatus;
    } catch (error) {
      startupLatestReleasePayload = null;
      startupUpdateStatus = buildUpdateStatus({
        checked: true,
        hasUpdate: false,
        error: error?.message || String(error || 'unknown error'),
      });
      logClientDiagnostic(
        'app.update',
        `startup check failed error=${startupUpdateStatus.error || 'unknown error'}`,
      );
      return startupUpdateStatus;
    }
  })();

  return startupUpdateCheckPromise;
}
