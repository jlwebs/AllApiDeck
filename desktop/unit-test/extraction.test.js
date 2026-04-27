import test from 'node:test';
import assert from 'node:assert';
import fs from 'node:fs';

// ── 模拟逻辑 (提取自 vite.config.js) ──

const ensureSkPrefix = (key) => (key && !key.startsWith('sk-') ? `sk-${key}` : key);
const isMaskedKey = (key) => key && key.includes('*');

const extractItems = (json) => {
  if (Array.isArray(json)) return json;
  if (json && typeof json === 'object') {
    // 处理各种嵌套返回结构 (Data/Items, Data.Data/Data.Items)
    let d = json.data || json.items;
    if (d && typeof d === 'object' && !Array.isArray(d)) {
       d = d.data || d.items || d;
    }
    if (Array.isArray(d)) return d;
    if (Array.isArray(json.data)) return json.data;
  }
  return [];
};

// ── 单元测试：JSON 数据解坑 ──

test('extractItems should handle flat arrays', () => {
    const input = [1, 2, 3];
    assert.deepStrictEqual(extractItems(input), [1, 2, 3]);
});

test('extractItems should handle {data: [...]}', () => {
    const input = { data: [1, 2] };
    assert.deepStrictEqual(extractItems(input), [1, 2]);
});

test('extractItems should handle {data: {data: [...]}} (New-API spec)', () => {
    const input = { data: { data: [1, 2] } };
    assert.deepStrictEqual(extractItems(input), [1, 2]);
});

test('extractItems should handle {items: [...]}', () => {
    const input = { items: [1, 2] };
    assert.deepStrictEqual(extractItems(input), [1, 2]);
});

// ── 现实数据验证：验证用户备份文件结构 ──

test('Extract access_token from real backup structure', () => {
    const BACKUP_PATH = "d:/GitHub/api-check/backup/accounts-backup-2026-04-01.json";
    if (!fs.existsSync(BACKUP_PATH)) {
        console.warn('Backup file missing, skip real file test.');
        return;
    }
    
    const data = JSON.parse(fs.readFileSync(BACKUP_PATH, 'utf8'));
    const accounts = data.accounts?.accounts || [];
    assert.ok(accounts.length > 0, "Should have at least one account");
    
    for (const acc of accounts) {
        if (acc.disabled) {
            console.log(`[SKIP] Account ${acc.site_name} is disabled.`);
            continue;
        }
        
        const token = acc.access_token || acc.account_info?.access_token;
        if (acc.authType === 'cookie') {
            console.log(`[SKIP] Account ${acc.site_name} uses 'cookie' auth, no token expected.`);
            continue;
        }
        
        if (!token) {
            console.error(`[DEBUG] Missing token for site:`, acc.site_name, JSON.stringify(acc, null, 2));
        }
        assert.ok(token, `Account ${acc.site_name} is missing access_token!`);
        assert.ok(acc.site_url, `Account ${acc.site_name} is missing site_url!`);
    }
});

test('ensureSkPrefix should work', () => {
    assert.strictEqual(ensureSkPrefix('test'), 'sk-test');
    assert.strictEqual(ensureSkPrefix('sk-test'), 'sk-test');
});

test('isMaskedKey should work', () => {
    assert.ok(isMaskedKey('sk-***test'));
    assert.ok(!isMaskedKey('sk-real-key'));
});
