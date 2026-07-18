import assert from 'node:assert/strict';
import { gzipSync } from 'node:zlib';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const modulePath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'utils', 'clipboardSmartImport.js');
const moduleUrl = pathToFileURL(modulePath).href;
const { extractSmartClipboardRecords, isLikelyClipboardApiKey, resolveClipboardImportRecords } = await import(`${moduleUrl}?t=${Date.now()}-${Math.random()}`);

const sample = `claude 深夜福利 1500个号
https://cpa.233318.xyz/v1
sk-PU6ECq4etTFYtVpu3

[飞升福利] grok1000刀：
https://pianyitoken.com/v1
sk-jInQO4trlHt5aZ58EvM8eShrOGaKk0vQ9WKP3LvBHRJHw4v6

grok 500刀 深夜福利：
https://ss.1263831.xyz/
sk-06e51dc83fc533d8118ddd9b3af6b5e30a6d76b327c8aa001a965560a8a6b27c

grok免费500刀（并发80，rpm1200）：
https://grok.justnew.net
g2a_85cf1188adbb_jHe09z1BTKswcB18Dr5qwovHmwCp9RJH

深夜福利-继续Grok，
https://newapi.rainflow.foo/v1
c2stdlNQR3J5UEZzN2FBZ3RMS0xVOE1aTHozdzh3dWJ2Q1dLRUtTSnFVbHhHQlN3Q1hD

第八波-公益Grok4.5 1000刀 rpm80
https://sub.yxxb.eu.cc
sk-272593d0e6553692979e73b89e24efcf81e20b5e8d2929237e1de9d2aa4e6898

公益Claude API，限额5小时，200刀左右，
https://api.gogocode.net
sk-0c69d28d6c47641f139f48e861e2fa40dc7118dee544593bc1c0a7a13d6cfa60`;

const records = extractSmartClipboardRecords(sample);
assert.equal(records.length, 7);
assert.deepStrictEqual(
  records.map(record => [record.siteName, record.siteUrl, record.apiKey]),
  [
    ['claude 深夜福利 1500个号', 'https://cpa.233318.xyz/v1', 'sk-PU6ECq4etTFYtVpu3'],
    ['[飞升福利] grok1000刀', 'https://pianyitoken.com/v1', 'sk-jInQO4trlHt5aZ58EvM8eShrOGaKk0vQ9WKP3LvBHRJHw4v6'],
    ['grok 500刀 深夜福利', 'https://ss.1263831.xyz/', 'sk-06e51dc83fc533d8118ddd9b3af6b5e30a6d76b327c8aa001a965560a8a6b27c'],
    ['grok免费500刀（并发80，rpm1200）', 'https://grok.justnew.net', 'g2a_85cf1188adbb_jHe09z1BTKswcB18Dr5qwovHmwCp9RJH'],
    ['深夜福利-继续Grok', 'https://newapi.rainflow.foo/v1', 'c2stdlNQR3J5UEZzN2FBZ3RMS0xVOE1aTHozdzh3dWJ2Q1dLRUtTSnFVbHhHQlN3Q1hD'],
    ['第八波-公益Grok4.5 1000刀 rpm80', 'https://sub.yxxb.eu.cc', 'sk-272593d0e6553692979e73b89e24efcf81e20b5e8d2929237e1de9d2aa4e6898'],
    ['公益Claude API，限额5小时，200刀左右', 'https://api.gogocode.net', 'sk-0c69d28d6c47641f139f48e861e2fa40dc7118dee544593bc1c0a7a13d6cfa60'],
  ]
);

assert.equal(isLikelyClipboardApiKey('ordinary-description-without-digits'), false);
assert.equal(isLikelyClipboardApiKey('g2a_85cf1188adbb_jHe09z1BTKswcB18Dr5qwovHmwCp9RJH'), true);

const noisyRecords = extractSmartClipboardRecords(`
https://api.example.com/v1，
sk-1234567890abcdefghijklmnop
https://api.example.com/v1
sk-1234567890abcdefghijklmnop

https://without-title.example/v1
not a key
`);
assert.equal(noisyRecords.length, 1);
assert.equal(noisyRecords[0].siteName, 'api.example.com');
assert.equal(noisyRecords[0].siteUrl, 'https://api.example.com/v1');

const orderVariants = extractSmartClipboardRecords(`http://216.195.211.206:8317/
sk-8lIaur3S2i5Xpi3yfXbiVVZLoF0mTpBy

gpt
https://ai.hhhl.cc/
sk-67cPBY14bpAgfkaDXDINGGF9eGDMngJz

sk-88c9d04dc94257ce78114042e4ab4b33845feaa367171ca6
https://welfare.0xpsyche.me/`);
assert.deepStrictEqual(
  orderVariants.map(record => [record.siteName, record.siteUrl, record.apiKey]),
  [
    ['216.195.211.206:8317', 'http://216.195.211.206:8317/', 'sk-8lIaur3S2i5Xpi3yfXbiVVZLoF0mTpBy'],
    ['gpt', 'https://ai.hhhl.cc/', 'sk-67cPBY14bpAgfkaDXDINGGF9eGDMngJz'],
    ['welfare.0xpsyche.me', 'https://welfare.0xpsyche.me/', 'sk-88c9d04dc94257ce78114042e4ab4b33845feaa367171ca6'],
  ]
);

const packagePayload = {
  format: 'api-check-key-export-v1',
  records: [{
    siteName: 'package',
    siteUrl: 'https://package.example/v1',
    apiKey: 'sk-package1234567890',
  }],
};
const packageToken = gzipSync(Buffer.from(JSON.stringify(packagePayload)))
  .toString('base64url');
const packageResult = await resolveClipboardImportRecords(`sk://${packageToken}`);
assert.equal(packageResult.mode, 'package');
assert.equal(packageResult.records.length, 1);
assert.equal(packageResult.records[0].siteName, 'package');

console.log('PASS tests/clipboardSmartImport.test.mjs');
