import assert from 'node:assert/strict';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const modulePath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'utils', 'clipboardImportBridge.js');
const moduleUrl = pathToFileURL(modulePath).href;
const { mergeClipboardImportState } = await import(`${moduleUrl}?t=${Date.now()}-${Math.random()}`);

const first = mergeClipboardImportState({
  existingRecords: [],
  existingGroups: [],
  importedRecords: [{
    siteName: 'welfare.0xpsyche.me',
    siteUrl: 'https://welfare.0xpsyche.me/',
    apiKey: 'sk-88c9d04dc94257ce78114042e4ab4b33845feaa367171ca6',
  }],
  now: 1000,
});
assert.equal(first.importedCount, 1);
assert.equal(first.createdCount, 1);
assert.equal(first.groups.length, 0);
assert.deepStrictEqual(first.records[0].groupIds, []);

const grouped = mergeClipboardImportState({
  existingRecords: first.records,
  existingGroups: first.groups,
  importedRecords: [{
    siteName: 'renamed',
    siteUrl: 'https://welfare.0xpsyche.me',
    apiKey: 'sk-88c9d04dc94257ce78114042e4ab4b33845feaa367171ca6',
  }],
  targetGroupName: 'Grok 福利',
  now: 2000,
  groupIdFactory: () => 'group::test',
});
assert.equal(grouped.importedCount, 1);
assert.equal(grouped.createdCount, 0);
assert.equal(grouped.updatedCount, 1);
assert.equal(grouped.groupCreated, true);
assert.deepStrictEqual(grouped.groups, [{ id: 'group::test', name: 'Grok 福利', createdAt: 2000 }]);
assert.deepStrictEqual(grouped.records[0].groupIds, ['group::test']);

const sameGroup = mergeClipboardImportState({
  existingRecords: grouped.records,
  existingGroups: grouped.groups,
  importedRecords: [{
    siteName: 'renamed',
    siteUrl: 'https://welfare.0xpsyche.me',
    apiKey: 'sk-88c9d04dc94257ce78114042e4ab4b33845feaa367171ca6',
  }],
  targetGroupName: 'Grok 福利',
  now: 3000,
});
assert.equal(sameGroup.groupCreated, false);
assert.equal(sameGroup.groups.length, 1);
assert.deepStrictEqual(sameGroup.records[0].groupIds, ['group::test']);

const sameNameBatch = mergeClipboardImportState({
  existingRecords: [{
    rowKey: 'same-name-1',
    siteName: '共享站点',
    siteUrl: 'https://same-name-one.example',
    apiKey: 'sk-same-name-one1234567890',
  }],
  importedRecords: [
    {
      siteName: '共享站点',
      siteUrl: 'https://same-name-two.example',
      apiKey: 'sk-same-name-two1234567890',
    },
    {
      siteName: '共享站点',
      siteUrl: 'https://same-name-three.example',
      apiKey: 'sk-same-name-three1234567890',
    },
  ],
});
assert.deepStrictEqual(
  sameNameBatch.records.map(record => record.siteName),
  ['共享站点', '共享站点 2', '共享站点 3']
);

const sameNameWithExistingSuffix = mergeClipboardImportState({
  existingRecords: [
    {
      rowKey: 'same-name-root',
      siteName: '共享站点',
      siteUrl: 'https://same-name-root.example',
      apiKey: 'sk-same-name-root1234567890',
    },
    {
      rowKey: 'same-name-suffix',
      siteName: '共享站点 2',
      siteUrl: 'https://same-name-suffix.example',
      apiKey: 'sk-same-name-suffix1234567890',
    },
  ],
  importedRecords: [{
    siteName: '共享站点',
    siteUrl: 'https://same-name-next.example',
    apiKey: 'sk-same-name-next1234567890',
  }],
});
assert.equal(sameNameWithExistingSuffix.records[2].siteName, '共享站点 3');

const explicitAll = mergeClipboardImportState({
  existingRecords: [],
  existingGroups: [],
  importedRecords: [{
    siteName: 'api.example.com',
    siteUrl: 'https://api.example.com',
    apiKey: 'sk-example1234567890',
  }],
  targetGroupName: '全部分组',
});
assert.equal(explicitAll.groups.length, 0);
assert.deepStrictEqual(explicitAll.records[0].groupIds, []);

console.log('PASS tests/clipboardImportBridge.test.mjs');
