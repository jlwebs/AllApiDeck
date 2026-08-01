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

function assertExtractionCase(name, rawText, expected) {
  assert.deepStrictEqual(
    extractSmartClipboardRecords(rawText).map(record => [record.siteUrl, record.apiKey]),
    expected,
    name
  );
}

const extractionCases = [
  [
    'single-line newapi JSON, key before url',
    '{"_type":"newapi_channel_conn","key":"sk-enTIKoiyiTiXrLzUe43D3eQZnc2Hu8HagwYJ7VP7SOOZZBKB","url":"https://api.123nhh.com"}',
    [['https://api.123nhh.com', 'sk-enTIKoiyiTiXrLzUe43D3eQZnc2Hu8HagwYJ7VP7SOOZZBKB']],
  ],
  [
    'single-line JSON, url before key',
    '{"url":"https://reverse.example/v1","key":"sk-reverse1234567890abcdefghijkl"}',
    [['https://reverse.example/v1', 'sk-reverse1234567890abcdefghijkl']],
  ],
  [
    'JSON aliases api_key and base_url',
    '{ "api_key": "sk-alias1234567890abcdefghijkl", "base_url": "https://alias.example/v1" }',
    [['https://alias.example/v1', 'sk-alias1234567890abcdefghijkl']],
  ],
  [
    'same line key equals baseUrl',
    'key=sk-inline1234567890abcdefghijkl baseUrl=https://inline.example/v1',
    [['https://inline.example/v1', 'sk-inline1234567890abcdefghijkl']],
  ],
  [
    'same line url before apiKey',
    'https://inline-reverse.example/v1 | apiKey: sk-inline-reverse1234567890abcdefghijkl',
    [['https://inline-reverse.example/v1', 'sk-inline-reverse1234567890abcdefghijkl']],
  ],
  [
    'leading spaces and tabs around fields',
    '  base_url  :  https://indented.example/v1\n\t\tkey\t:\t sk-indented1234567890abcdefghijkl  ',
    [['https://indented.example/v1', 'sk-indented1234567890abcdefghijkl']],
  ],
  [
    'blank line between url and key',
    'https://blank-after.example/v1\n\n\n  sk-blank-after1234567890abcdefghijkl',
    [['https://blank-after.example/v1', 'sk-blank-after1234567890abcdefghijkl']],
  ],
  [
    'blank line between key and url',
    'sk-blank-before1234567890abcdefghijkl\n\nendpoint: https://blank-before.example/v1',
    [['https://blank-before.example/v1', 'sk-blank-before1234567890abcdefghijkl']],
  ],
  [
    'two records without newlines',
    'url=https://compact-one.example/v1 key=sk-compact-one1234567890abcdef url=https://compact-two.example/v1 key=sk-compact-two1234567890abcdef',
    [
      ['https://compact-one.example/v1', 'sk-compact-one1234567890abcdef'],
      ['https://compact-two.example/v1', 'sk-compact-two1234567890abcdef'],
    ],
  ],
  [
    'mixed order for adjacent inline records',
    'key=sk-mixed-one1234567890abcdef url=https://mixed-one.example/v1; url=https://mixed-two.example/v1 key=sk-mixed-two1234567890abcdef',
    [
      ['https://mixed-one.example/v1', 'sk-mixed-one1234567890abcdef'],
      ['https://mixed-two.example/v1', 'sk-mixed-two1234567890abcdef'],
    ],
  ],
  [
    'multiple urls and keys on one line',
    'https://same-line-one.example/v1 sk-same-line-one1234567890abcdef https://same-line-two.example/v1 sk-same-line-two1234567890abcdef',
    [
      ['https://same-line-one.example/v1', 'sk-same-line-one1234567890abcdef'],
      ['https://same-line-two.example/v1', 'sk-same-line-two1234567890abcdef'],
    ],
  ],
  [
    'JSON array of connection objects',
    '[{"key":"sk-array-one1234567890abcdef","url":"https://array-one.example"},{"url":"https://array-two.example","key":"sk-array-two1234567890abcdef"}]',
    [
      ['https://array-one.example', 'sk-array-one1234567890abcdef'],
      ['https://array-two.example', 'sk-array-two1234567890abcdef'],
    ],
  ],
  [
    'nested JSON connection object',
    '{"data":{"connection":{"baseUrl":"https://nested.example/v1","apiKey":"sk-nested1234567890abcdefghijkl"},"models":["ignore-me"]}}',
    [['https://nested.example/v1', 'sk-nested1234567890abcdefghijkl']],
  ],
  [
    'JSON escaped slashes',
    '{"url":"https:\\/\\/escaped.example.com\\/v1","key":"sk-escaped1234567890abcdefghijkl"}',
    [['https://escaped.example.com/v1', 'sk-escaped1234567890abcdefghijkl']],
  ],
  [
    'endpoint and token aliases',
    '{ endpoint: "https://endpoint.example/v1", token: "sk-endpoint1234567890abcdefghijkl" }',
    [['https://endpoint.example/v1', 'sk-endpoint1234567890abcdefghijkl']],
  ],
  [
    'api_base_url and access_token aliases',
    'api_base_url=https://access-token.example/v1\naccess_token=sk-access-token1234567890abcdef',
    [['https://access-token.example/v1', 'sk-access-token1234567890abcdef']],
  ],
  [
    'markdown link and labeled key',
    '[接口地址](https://markdown.example/v1)\nAPI Key: sk-markdown1234567890abcdefghijkl',
    [['https://markdown.example/v1', 'sk-markdown1234567890abcdefghijkl']],
  ],
  [
    'fenced code block',
    '```text\n  sk-code-block1234567890abcdefghijkl\n  https://code-block.example/v1\n```',
    [['https://code-block.example/v1', 'sk-code-block1234567890abcdefghijkl']],
  ],
  [
    'Chinese punctuation around values',
    '接口地址（https://punctuation.example/v1）；密钥：sk-punctuation1234567890abcdef。',
    [['https://punctuation.example/v1', 'sk-punctuation1234567890abcdef']],
  ],
  [
    'quoted CSV-like pair',
    '"https://quoted.example/v1", "sk-quoted1234567890abcdefghijkl"',
    [['https://quoted.example/v1', 'sk-quoted1234567890abcdefghijkl']],
  ],
  [
    'Bearer authorization wrapper',
    'Authorization: Bearer sk-bearer1234567890abcdefghijkl\nbaseUrl: https://bearer.example/v1',
    [['https://bearer.example/v1', 'sk-bearer1234567890abcdefghijkl']],
  ],
  [
    'xAI key prefix',
    'baseUrl=https://xai.example/v1 apiKey=xai-key1234567890abcdefghijkl',
    [['https://xai.example/v1', 'xai-key1234567890abcdefghijkl']],
  ],
  [
    'Grok key prefix',
    'gsk_grok1234567890abcdefghijkl https://grok-key.example/v1',
    [['https://grok-key.example/v1', 'gsk_grok1234567890abcdefghijkl']],
  ],
  [
    'long key without a known prefix',
    'key=c2stdlNQR3J5UEZzN2FBZ3RMS0xVOE1aTHozdzh3dWJ2Q1dLRUtTSnFVbHhHQlN3Q1hD\nurl=https://generic-key.example/v1',
    [['https://generic-key.example/v1', 'c2stdlNQR3J5UEZzN2FBZ3RMS0xVOE1aTHozdzh3dWJ2Q1dLRUtTSnFVbHhHQlN3Q1hD']],
  ],
  [
    'base64-like key with padding',
    'url=https://base64-key.example/v1\nkey=YWJjMTIzNDU2Nzg5MGFiY2RlZg==',
    [['https://base64-key.example/v1', 'YWJjMTIzNDU2Nzg5MGFiY2RlZg==']],
  ],
  [
    'case-insensitive fields and protocol',
    'BASE_URL: HTTP://case.example/v1\nKEY: sk-case1234567890abcdefghijkl',
    [['HTTP://case.example/v1', 'sk-case1234567890abcdefghijkl']],
  ],
  [
    'angle brackets and quoted key',
    '<https://bracket.example/v1> key: `sk-bracket1234567890abcdefghijkl`',
    [['https://bracket.example/v1', 'sk-bracket1234567890abcdefghijkl']],
  ],
  [
    'duplicate pair is emitted once',
    '{"url":"https://duplicate.example/v1","key":"sk-duplicate1234567890abcdefghijkl"}\nhttps://duplicate.example/v1\nsk-duplicate1234567890abcdefghijkl',
    [['https://duplicate.example/v1', 'sk-duplicate1234567890abcdefghijkl']],
  ],
  [
    'interleaved labels and values',
    '站点名称\nAPI Base\nhttps://interleaved.example/v1\n认证 Token\nsk-interleaved1234567890abcdef',
    [['https://interleaved.example/v1', 'sk-interleaved1234567890abcdef']],
  ],
  [
    'base-url and apikey aliases',
    'base-url=https://hyphen-alias.example/v1\napikey=sk-hyphen-alias1234567890abcdef',
    [['https://hyphen-alias.example/v1', 'sk-hyphen-alias1234567890abcdef']],
  ],
  [
    'no full data parsing, only scalar key and url extraction',
    '{"url":"https://scalar-only.example/v1","key":"sk-scalar-only1234567890abcdef","data":{"url":"https://should-not-win.example","key":"not-a-real-key"}}',
    [['https://scalar-only.example/v1', 'sk-scalar-only1234567890abcdef']],
  ],
];

for (const [name, rawText, expected] of extractionCases) {
  assertExtractionCase(name, rawText, expected);
}

assert.deepStrictEqual(
  extractSmartClipboardRecords('notes=abcdefghijklmnopqrstuvwx https://noise-only.example/v1').map(record => [record.siteUrl, record.apiKey]),
  [],
  'long prose-like text is not treated as an API key'
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
