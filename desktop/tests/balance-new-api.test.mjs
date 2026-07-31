import assert from 'node:assert/strict';

import {
  fetchQuotaLabelWithBatchLogic,
  formatNewApiQuotaAmount,
  isDisplayableQuotaLabel,
} from '../src/utils/balance.js';

function response(status, payload) {
  return {
    status,
    ok: status >= 200 && status < 300,
    async json() {
      return payload;
    },
  };
}

function getTargetUrl(proxyUrl) {
  return new URL(`http://local.test${proxyUrl}`).searchParams.get('url');
}

function fetchWithRoutes(routes, calls) {
  return async proxyUrl => {
    const targetUrl = getTargetUrl(proxyUrl);
    calls.push(targetUrl);
    return routes[targetUrl] || response(404, {});
  };
}

assert.equal(formatNewApiQuotaAmount(9992.39), '$0.020');
assert.equal(formatNewApiQuotaAmount(-24856), '$-0.050');
assert.equal(isDisplayableQuotaLabel('$-0.050'), true);
assert.equal(isDisplayableQuotaLabel('$0.000'), true);

{
  const calls = [];
  const apiFetch = await fetchWithRoutes({
    'https://new-api.example.com/api/usage/token/': response(404, {}),
    'https://new-api.example.com/api/usage/token': response(200, {
      data: {
        total_available: 50000,
        total_used: 450000,
        unlimited_quota: false,
      },
    }),
  }, calls);

  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site: { tokens: [{ key: 'sk-new-api' }] },
    siteUrl: 'https://new-api.example.com/api/v1',
  });

  assert.equal(label, '$0.100');
  assert.ok(calls.includes('https://new-api.example.com/api/usage/token/'));
  assert.ok(calls.includes('https://new-api.example.com/api/usage/token'));
}

{
  const apiFetch = await fetchWithRoutes({
    'https://new-api-unlimited.example.com/api/usage/token/': response(200, {
      data: {
        total_available: 0,
        unlimited_quota: true,
      },
    }),
  }, []);

  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site: { tokens: [{ key: 'sk-new-api-unlimited' }] },
    siteUrl: 'https://new-api-unlimited.example.com',
  });

  assert.equal(isDisplayableQuotaLabel(label), true);
  assert.equal(label.startsWith('$'), false);
}

{
  const authorizations = [];
  const apiFetch = async (proxyUrl, options = {}) => {
    authorizations.push(options?.headers?.Authorization || '');
    const targetUrl = getTargetUrl(proxyUrl);
    if (targetUrl === 'https://new-api-negative.example.com/api/usage/token/') {
      return response(200, {
        data: {
          total_available: -24856,
          total_granted: 2500000000,
          total_used: 2500024856,
          unlimited_quota: false,
        },
      });
    }
    return response(404, {});
  };

  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site: {
      account_info: { access_token: 'new-api-login-token' },
      tokens: [{ key: 'sk-other-token' }],
    },
    siteUrl: 'https://new-api-negative.example.com/v1',
    apiKey: 'sk-target-token',
  });

  assert.equal(label, '$-0.050');
  assert.equal(authorizations[0], 'Bearer sk-target-token');
}

{
  const calls = [];
  const apiFetch = await fetchWithRoutes({
    'https://sub2api.example.com/v1/usage': response(200, {
      mode: 'quota_limited',
      remaining: 1.25,
      unit: 'USD',
    }),
  }, calls);

  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site: { site_type: 'sub2api', tokens: [{ key: 'sk-sub2api' }] },
    siteUrl: 'https://sub2api.example.com/v1',
  });

  assert.equal(label, '$1.250');
  assert.deepEqual(calls, [
    'https://sub2api.example.com/api/usage/token/',
    'https://sub2api.example.com/api/usage/token',
    'https://sub2api.example.com/v1/usage',
  ]);
}

{
  const apiFetch = await fetchWithRoutes({}, []);

  const label = await fetchQuotaLabelWithBatchLogic({
    apiFetch,
    site: { tokens: [{ key: 'sk-new-api-low-balance', remain_quota: 9992.39 }] },
    siteUrl: 'https://new-api-low-balance.example.com',
  });

  assert.equal(label, '$0.020');
}
