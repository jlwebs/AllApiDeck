const TAKEOVER_BACKUP_STORAGE_KEY = 'api_check_console_advanced_proxy_takeover_backups_v1';

function loadTakeoverBackups() {
  try {
    const raw = localStorage.getItem(TAKEOVER_BACKUP_STORAGE_KEY);
    const parsed = JSON.parse(raw || '{}');
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function saveTakeoverBackups(backups) {
  try {
    localStorage.setItem(TAKEOVER_BACKUP_STORAGE_KEY, JSON.stringify(backups || {}));
    return true;
  } catch (error) {
    console.warn('[desktopConfigTakeover] save backups failed:', error?.message || String(error));
    return false;
  }
}

export function getDesktopConfigTakeoverBackup(appId) {
  const normalizedAppId = String(appId || '').trim();
  if (!normalizedAppId) return null;
  const backups = loadTakeoverBackups();
  const backup = backups[normalizedAppId];
  return backup && Array.isArray(backup.files) ? backup : null;
}

export function captureDesktopConfigTakeoverBackup(appId, snapshot) {
  const normalizedAppId = String(appId || '').trim();
  if (!normalizedAppId) return false;
  const backups = loadTakeoverBackups();
  if (backups[normalizedAppId]?.files?.length) return false;

  const files = (Array.isArray(snapshot?.files) ? snapshot.files : [])
    .filter(file => String(file?.appId || '').trim() === normalizedAppId)
    .map(file => ({
      appId: normalizedAppId,
      fileId: String(file?.fileId || '').trim(),
      content: String(file?.content || ''),
      exists: file?.exists === true,
    }))
    .filter(file => file.fileId);
  if (!files.length) return false;

  backups[normalizedAppId] = {
    capturedAt: Date.now(),
    files,
  };
  if (!saveTakeoverBackups(backups)) {
    throw new Error('无法保存客户端原始配置备份，请清理本地存储后重试');
  }
  return true;
}

export function clearDesktopConfigTakeoverBackup(appId) {
  const normalizedAppId = String(appId || '').trim();
  if (!normalizedAppId) return;
  const backups = loadTakeoverBackups();
  if (!Object.prototype.hasOwnProperty.call(backups, normalizedAppId)) return;
  delete backups[normalizedAppId];
  saveTakeoverBackups(backups);
}

export function buildDesktopConfigTakeoverRestorePreview(appId, snapshot) {
  const normalizedAppId = String(appId || '').trim();
  const backup = getDesktopConfigTakeoverBackup(normalizedAppId);
  if (!backup?.files?.length) return null;

  const currentFiles = new Map(
    (Array.isArray(snapshot?.files) ? snapshot.files : [])
      .filter(file => String(file?.appId || '').trim() === normalizedAppId)
      .map(file => [String(file?.fileId || '').trim(), file]),
  );
  const restoreFiles = backup.files.map(file => {
    const current = currentFiles.get(file.fileId);
    return {
      appId: normalizedAppId,
      appName: current?.appName || normalizedAppId,
      fileId: file.fileId,
      label: current?.label || file.fileId,
      path: current?.path || '',
      exists: file.exists === true,
      before: String(current?.content || ''),
      after: file.content,
    };
  });
  const writes = restoreFiles.map(file => ({
    appId: normalizedAppId,
    fileId: file.fileId,
    content: file.after,
    exists: file.exists,
  }));

  return {
    appGroups: restoreFiles.length
      ? [{
          appId: normalizedAppId,
          appName: restoreFiles[0].appName || normalizedAppId,
          files: restoreFiles,
        }]
      : [],
    writes,
    errors: [],
  };
}
