// scripts/test-pipeline.cjs
const fs = require('fs');

const BACKUP_FILE = "D:\\GitHub\\api-check\\backup\\accounts-backup-2026-04-01.json";

// --- 阶段 2: 模拟后端代理逻辑 (fetchKeys.js) ---
function mockBackendProxy(accFromFrontend) {
    // 模拟成功提取后的返回
    // 关键：必须带回 account_info
    const items = [{ key: "sk-test-key-123" }]; 
    const endpoint = "/api/token";
    
    return {
        id: accFromFrontend.id,
        site_name: accFromFrontend.site_name,
        site_url: accFromFrontend.site_url,
        tokens: items,
        endpoint: endpoint,
        account_info: accFromFrontend.account_info // <--- 核心修复点：原样带回
    };
}

// --- 阶段 3: 模拟前端提取逻辑 (BatchCheck.vue) ---
function mockFrontendHoverQuota(siteFromBackend) {
    const site = siteFromBackend; // 在 hoverQuota 里，record.accountData 就是从后端拿回的这个对象
    
    // 提取逻辑
    const rawId = site?.account_info?.id || site?.id || site?.uid || site?.user_id || '';
    const userId = /^\d+$/.test(String(rawId)) ? String(rawId) : '';
    return userId;
}

async function runFullPipelineTest() {
    try {
        console.log("=== 开始全链路模拟测试 ===");
        
        // 1. 加载真实文件
        const content = fs.readFileSync(BACKUP_FILE, 'utf8');
        const data = JSON.parse(content);
        
        // --- 阶段 1: 模拟前端上传解析 ---
        const rawAccounts = data?.accounts?.accounts || [];
        console.log(`[PASS] 成功解析账号数组，数量: ${rawAccounts.length}`);
        
        const firstRaw = rawAccounts[0];
        console.log(`[DATA] 原始 JSON ID: ${firstRaw.id}`);
        console.log(`[DATA] 原始 AccountInfo ID: ${firstRaw.account_info?.id}`);

        // 2. 模拟发送给后端并接收回包
        const backendResult = mockBackendProxy(firstRaw);
        console.log(`[PROXY] 后端返回对象 Key 列表: ${Object.keys(backendResult).join(', ')}`);
        
        if (!backendResult.account_info) {
            console.log("[FAIL] 致命错误：后端返回结果中丢失了 account_info！");
            process.exit(1);
        }

        // 3. 模拟前端最终提取
        const finalUid = mockFrontendHoverQuota(backendResult);
        console.log(`[FINAL] 最终提取出的 New-Api-User UID: "${finalUid}"`);

        if (/^\d+$/.test(finalUid) && finalUid !== "") {
            console.log("\n[SUCCESS] 全链路打通！数字 ID 在经过后端代理后成功保留并被前端识别。");
        } else {
            console.log("\n[FAIL] 链路中断：最终提取出的 ID 格式不正确或为空。");
            process.exit(1);
        }

    } catch (err) {
        console.error("[ERROR]", err.message);
        process.exit(1);
    }
}

runFullPipelineTest();
