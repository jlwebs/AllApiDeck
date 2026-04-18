import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const batchCheckPath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'components', 'BatchCheck.vue');

const source = await readFile(batchCheckPath, 'utf8');
const matches = [...source.matchAll(/saveLastResultsSnapshot\(([^)]*)\)/g)];

assert.ok(matches.length >= 4, 'expected the batch history save call sites to be present');
assert.ok(!source.includes('saveLastResultsSnapshot();'), 'no-argument saveLastResultsSnapshot call found');

for (const match of matches) {
  assert.match(match[1], /testResults\.value/, `unexpected saveLastResultsSnapshot argument: ${match[1]}`);
}

console.log('PASS tests/batchCheck-history-regression.test.mjs');
