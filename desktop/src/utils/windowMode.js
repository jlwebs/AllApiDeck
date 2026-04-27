function getAppBridge() {
  return window?.go?.main?.App;
}

export function isSidebarBridgeAvailable() {
  return typeof getAppBridge()?.ExitSidebarMode === 'function';
}

export async function exitSidebarMode() {
  const app = getAppBridge();
  if (typeof app?.ExitSidebarMode === 'function') {
    return app.ExitSidebarMode();
  }
  return false;
}

export async function enterSidebarMode() {
  const app = getAppBridge();
  if (typeof app?.EnterSidebarMode === 'function') {
    return app.EnterSidebarMode();
  }
  return false;
}

export async function toggleSidebarMode() {
  const app = getAppBridge();
  if (typeof app?.ToggleSidebarMode === 'function') {
    return app.ToggleSidebarMode();
  }
  return false;
}

export async function getSidebarMode() {
  const app = getAppBridge();
  if (typeof app?.GetSidebarMode === 'function') {
    return app.GetSidebarMode();
  }
  return false;
}

export function isManualSidebarBridgeAvailable() {
  return typeof getAppBridge()?.OpenManualSidebarPanel === 'function';
}

export async function openManualSidebarPanel() {
  const app = getAppBridge();
  if (typeof app?.OpenManualSidebarPanel === 'function') {
    return app.OpenManualSidebarPanel();
  }
  return false;
}
