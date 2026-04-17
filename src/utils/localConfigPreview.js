function ensureTrailingNewline(text) {
  const normalized = String(text || '');
  return normalized.endsWith('\n') ? normalized : `${normalized}\n`;
}

function normalizePreviewContent(content) {
  if (typeof content === 'string') {
    return ensureTrailingNewline(content);
  }

  return ensureTrailingNewline(JSON.stringify(content ?? {}, null, 2));
}

export function buildSingleFileWritePreview({
  appId,
  appName,
  fileId,
  label,
  path,
  before,
  after,
}) {
  const beforeText = normalizePreviewContent(before);
  const afterText = normalizePreviewContent(after);

  return {
    appGroups: [
      {
        appId,
        appName,
        files: [
          {
            appId,
            appName,
            fileId,
            label,
            path,
            exists: true,
            before: beforeText,
            after: afterText,
          },
        ],
      },
    ],
    writes: [
      {
        appId,
        fileId,
        content: afterText,
      },
    ],
    errors: [],
  };
}
