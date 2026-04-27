const formatBalance = (quota) => (quota / 500000).toFixed(2);

const PROTOCOL_MAP = {
  'sub2api': {
    primary: '/api/v1/auth/me',
    isUsd: true
  },
  'new-api': {
    primary: '/api/user/self',
    isUsd: false
  }
};

function simulateParsing(siteType, endpoint, json) {
  const proto = PROTOCOL_MAP[siteType];
  const data = json?.data ?? json;
  
  let val = data.quota ?? data.balance ?? null;
  let quota = null;
  
  if (val !== null) {
    const needsUsdConversion = (proto?.isUsd || endpoint.includes('/v1/auth/me')) && val < 1000;
    quota = needsUsdConversion ? Math.round(Number(val) * 500000) : Number(val);
  }
  
  return quota;
}

// 测试用例 1: Sub2API 返回美元余额
const test1 = simulateParsing('sub2api', '/api/v1/auth/me', {
  code: 0,
  message: "success",
  data: { balance: 0.5 }
});
console.log(`Test 1 (Sub2API USD 0.5): Expected 250000, Got ${test1} ${test1 === 250000 ? '✅' : '❌'}`);

// 测试用例 2: NewAPI 返回 Quota
const test2 = simulateParsing('new-api', '/api/user/self', {
  success: true,
  data: { quota: 500000 }
});
console.log(`Test 2 (NewAPI Quota 500000): Expected 500000, Got ${test2} ${test2 === 500000 ? '✅' : '❌'}`);

// 测试用例 3: 自动探测回退 (sub2api 类型但在 user/self 端点)
const test3 = simulateParsing('sub2api', '/api/user/self', {
  quota: 1000000
});
console.log(`Test 3 (Sub2API fallback Quota): Expected 1000000, Got ${test3} ${test3 === 1000000 ? '✅' : '❌'}`);

// 测试用例 4: 极小值判定渲染
if (test1 !== null) {
    console.log(`Test 4 (Formatting): Expected $0.50, Got $${formatBalance(test1)} ${formatBalance(test1) === '0.50' ? '✅' : '❌'}`);
}
