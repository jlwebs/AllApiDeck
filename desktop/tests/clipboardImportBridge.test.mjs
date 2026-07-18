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
