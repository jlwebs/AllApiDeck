import assert from 'node:assert/strict';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const apiModulePath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'utils', 'api.js');
const apiModuleUrl = pathToFileURL(apiModulePath).href;

const requests = [];
globalThis.fetch = async (input, init = {}) => {
  requests.push({
    input: String(input),
    headers: init?.headers || {},
  });
  return {
    ok: true,
    status: 200,
    async json() {
      return {
        data: ['gpt-5-nano', 'claude-sonnet-4-6'],
      };
    },
  };
};
globalThis.window = {
  fetch: globalThis.fetch,
  location: {
    protocol: 'http:',
    hostname: 'localhost',
  },
};

const api = await import(`${apiModuleUrl}?t=${Date.now()}-${Math.random()}`);

assert.equal(api.__modelListTestUtils.normalizeModelListUid('5004'), '5004');
assert.equal(api.__modelListTestUtils.normalizeModelListUid('abc'), '');

requests.length = 0;
const payload = await api.fetchModelList('https://jiuuij.de5.net', 'sk-test', { uid: '5004' });
assert.deepStrictEqual(payload.data, ['gpt-5-nano', 'claude-sonnet-4-6']);
assert.equal(requests.length > 0, true);
assert.match(requests[0].input, /\/api\/proxy-get\?url=.*&uid=5004$/);
assert.equal(requests[0].headers.Authorization, 'Bearer sk-test');

requests.length = 0;
await api.fetchModelList('https://jiuuij.de5.net', 'sk-test', { uid: 'not-a-number' });
assert.equal(requests.length > 0, true);
assert.doesNotMatch(requests[0].input, /[?&]uid=/);

requests.length = 0;
globalThis.fetch = async (input, init = {}) => {
  requests.push({
    input: String(input),
    headers: init?.headers || {},
  });
  return {
    ok: false,
    status: 401,
    async text() {
      return JSON.stringify({ error: 'unauthorized' });
    },
  };
};
globalThis.window.fetch = globalThis.fetch;

await assert.rejects(
  api.fetchModelList('https://jiuuij.de5.net', 'sk-test', { uid: '5004' }),
  error => {
    assert.match(error.message, /unauthorized|401/);
    assert.equal(Array.isArray(error.modelListDiagnostics?.attempts), true);
    assert.equal(error.modelListDiagnostics.attempts.some(attempt => attempt.status === 401), true);
    assert.match(error.modelListDiagnostics.replayRequest.proxyUrl, /\/api\/proxy-get\?url=/);
    assert.equal(error.modelListDiagnostics.replayRequest.headers.Authorization, 'Bearer sk-test');
    assert.equal(error.modelListDiagnostics.traceLines.some(line => line.includes('HTTP_401')), true);
    return true;
  }
);

console.log('PASS tests/fetchModelList.test.mjs');
