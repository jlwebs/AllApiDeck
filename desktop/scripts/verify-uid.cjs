// scripts/verify-uid.js
const fs = require('fs');
const path = require('path');

const BACKUP_FILE = "D:\\GitHub\\api-check\\backup\\accounts-backup-2026-04-01.json";

function extractNumericUid(acc) {
    // 逻辑：严格抓取 account_info.id
    const rawId = acc?.account_info?.id || acc?.id || acc?.uid || acc?.user_id || '';
    const userId = /^\d+$/.test(String(rawId)) ? String(rawId) : '';
    return userId;
}

try {
    const content = fs.readFileSync(BACKUP_FILE, 'utf8');
    const data = JSON.parse(content);
    
    // 按 BatchCheck.vue 的逻辑定位数组
    const accounts = data?.accounts?.accounts || [];
    
    console.log(`[TEST] 查找到账号数量: ${accounts.length}`);
    console.log('---------------------------------------------------------');
    console.log('| 站点名称 | 原始 ID (可能是UUID) | 提取出的数字 UID |');
    console.log('---------------------------------------------------------');

    let successCount = 0;
    accounts.slice(0, 15).forEach(acc => {
        const extracted = extractNumericUid(acc);
        const isNumeric = /^\d+$/.test(extracted);
        if (isNumeric) successCount++;
        
        console.log(`| ${acc.site_name?.padEnd(10)} | ${String(acc.id).slice(0, 18)}... | ${extracted.padEnd(14)} |`);
    });

    console.log('---------------------------------------------------------');
    console.log(`[PASS] 前 15 个条目中，成功提取数字 UID 的数量: ${successCount}`);
    
    if (successCount > 0) {
        console.log('\n[RESULT] 验证通过！数字 ID 提取逻辑已生效。');
    } else {
        console.log('\n[FAIL] 警告：未能提取到任何纯数字 ID。');
    }

} catch (err) {
    console.error('[ERROR]', err.message);
}
