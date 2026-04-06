export function buildSideBySideDiff(beforeText, afterText) {
  const beforeLines = splitLines(beforeText);
  const afterLines = splitLines(afterText);
  const operations = buildLineOperations(beforeLines, afterLines);
  const rows = [];

  let beforeLineNumber = 1;
  let afterLineNumber = 1;
  let index = 0;

  while (index < operations.length) {
    const operation = operations[index];

    if (operation.type === 'equal') {
      rows.push({
        key: `equal-${beforeLineNumber}-${afterLineNumber}`,
        type: 'equal',
        beforeLineNumber,
        afterLineNumber,
        beforeText: operation.text,
        afterText: operation.text,
        beforeParts: [{ text: operation.text, changed: false }],
        afterParts: [{ text: operation.text, changed: false }],
      });
      beforeLineNumber += 1;
      afterLineNumber += 1;
      index += 1;
      continue;
    }

    const removed = [];
    const added = [];

    while (index < operations.length && operations[index].type !== 'equal') {
      if (operations[index].type === 'remove') {
        removed.push(operations[index].text);
      } else {
        added.push(operations[index].text);
      }
      index += 1;
    }

    const maxLength = Math.max(removed.length, added.length);
    for (let offset = 0; offset < maxLength; offset += 1) {
      const beforeValue = removed[offset];
      const afterValue = added[offset];
      const rowType = beforeValue != null && afterValue != null
        ? 'modify'
        : beforeValue != null
          ? 'remove'
          : 'add';

      const inlineParts = buildInlineParts(beforeValue || '', afterValue || '');

      rows.push({
        key: `${rowType}-${beforeLineNumber + offset}-${afterLineNumber + offset}`,
        type: rowType,
        beforeLineNumber: beforeValue != null ? beforeLineNumber + offset : null,
        afterLineNumber: afterValue != null ? afterLineNumber + offset : null,
        beforeText: beforeValue || '',
        afterText: afterValue || '',
        beforeParts: beforeValue != null
          ? rowType === 'modify'
            ? inlineParts.before
            : [{ text: beforeValue, changed: true }]
          : [],
        afterParts: afterValue != null
          ? rowType === 'modify'
            ? inlineParts.after
            : [{ text: afterValue, changed: true }]
          : [],
      });
    }

    beforeLineNumber += removed.length;
    afterLineNumber += added.length;
  }

  const chunks = buildDiffChunks(rows);
  return {
    rows,
    chunks,
    hasChanges: chunks.length > 0,
  };
}

function splitLines(text) {
  const normalized = String(text || '').replace(/\r\n/g, '\n');
  const lines = normalized.split('\n');
  if (lines.length > 0 && lines[lines.length - 1] === '') {
    lines.pop();
  }
  return lines;
}

function buildLineOperations(beforeLines, afterLines) {
  const beforeLength = beforeLines.length;
  const afterLength = afterLines.length;
  const matrix = Array.from({ length: beforeLength + 1 }, () =>
    new Array(afterLength + 1).fill(0)
  );

  for (let beforeIndex = beforeLength - 1; beforeIndex >= 0; beforeIndex -= 1) {
    for (let afterIndex = afterLength - 1; afterIndex >= 0; afterIndex -= 1) {
      if (beforeLines[beforeIndex] === afterLines[afterIndex]) {
        matrix[beforeIndex][afterIndex] = matrix[beforeIndex + 1][afterIndex + 1] + 1;
      } else {
        matrix[beforeIndex][afterIndex] = Math.max(
          matrix[beforeIndex + 1][afterIndex],
          matrix[beforeIndex][afterIndex + 1]
        );
      }
    }
  }

  const operations = [];
  let beforeIndex = 0;
  let afterIndex = 0;

  while (beforeIndex < beforeLength && afterIndex < afterLength) {
    if (beforeLines[beforeIndex] === afterLines[afterIndex]) {
      operations.push({ type: 'equal', text: beforeLines[beforeIndex] });
      beforeIndex += 1;
      afterIndex += 1;
      continue;
    }

    if (matrix[beforeIndex + 1][afterIndex] >= matrix[beforeIndex][afterIndex + 1]) {
      operations.push({ type: 'remove', text: beforeLines[beforeIndex] });
      beforeIndex += 1;
    } else {
      operations.push({ type: 'add', text: afterLines[afterIndex] });
      afterIndex += 1;
    }
  }

  while (beforeIndex < beforeLength) {
    operations.push({ type: 'remove', text: beforeLines[beforeIndex] });
    beforeIndex += 1;
  }

  while (afterIndex < afterLength) {
    operations.push({ type: 'add', text: afterLines[afterIndex] });
    afterIndex += 1;
  }

  return operations;
}

function buildInlineParts(beforeText, afterText) {
  if (beforeText === afterText) {
    return {
      before: [{ text: beforeText, changed: false }],
      after: [{ text: afterText, changed: false }],
    };
  }

  let prefixLength = 0;
  const maxPrefix = Math.min(beforeText.length, afterText.length);
  while (
    prefixLength < maxPrefix &&
    beforeText[prefixLength] === afterText[prefixLength]
  ) {
    prefixLength += 1;
  }

  let suffixLength = 0;
  const maxSuffix = Math.min(
    beforeText.length - prefixLength,
    afterText.length - prefixLength
  );
  while (
    suffixLength < maxSuffix &&
    beforeText[beforeText.length - 1 - suffixLength] ===
      afterText[afterText.length - 1 - suffixLength]
  ) {
    suffixLength += 1;
  }

  const beforePrefix = beforeText.slice(0, prefixLength);
  const beforeChanged = beforeText.slice(
    prefixLength,
    beforeText.length - suffixLength
  );
  const beforeSuffix = beforeText.slice(beforeText.length - suffixLength);

  const afterPrefix = afterText.slice(0, prefixLength);
  const afterChanged = afterText.slice(
    prefixLength,
    afterText.length - suffixLength
  );
  const afterSuffix = afterText.slice(afterText.length - suffixLength);

  return {
    before: buildParts(beforePrefix, beforeChanged, beforeSuffix),
    after: buildParts(afterPrefix, afterChanged, afterSuffix),
  };
}

function buildParts(prefix, changed, suffix) {
  const parts = [];
  if (prefix) {
    parts.push({ text: prefix, changed: false });
  }
  if (changed) {
    parts.push({ text: changed, changed: true });
  }
  if (suffix) {
    parts.push({ text: suffix, changed: false });
  }
  if (parts.length === 0) {
    parts.push({ text: '', changed: false });
  }
  return parts;
}

function buildDiffChunks(rows) {
  const chunks = [];
  let currentChunk = null;

  rows.forEach((row, index) => {
    if (row.type === 'equal') {
      currentChunk = null;
      return;
    }

    if (!currentChunk) {
      currentChunk = {
        id: `chunk-${chunks.length + 1}`,
        startIndex: index,
        endIndex: index,
      };
      chunks.push(currentChunk);
    } else {
      currentChunk.endIndex = index;
    }
  });

  return chunks;
}
