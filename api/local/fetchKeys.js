// api/local/fetchKeys.js
// 后端代理: 替前端向各中转站拉取真实 API Token 列表，绕过 CORS 限制

import express from 'express';
import fs from 'fs';
import path from 'path';

const router = express.Router();

// 简单的文件日志记录
const logDir = 'logs';
if (!fs.existsSync(logDir)) fs.mkdirSync(logDir);
const fetchLogStream = fs.createWriteStream(path.join(logDir, 'fetch-keys.log'), { flags: 'a' });

function fetchLog(msg) {
  const timestamp = new Date().toLocaleString();
  const fullMsg = `[${timestamp}] ${msg}`;
  fetchLogStream.write(fullMsg + '\n');
  console.log(fullMsg);
}

// 不做鉴权检查（本机代理接口，外部无法访问）
// POST /api/fetch-keys
// body: { accounts: [{ id, site_name, site_url, site_type, account_info: { access_token } }] }
router.post('/', async (req, res) => {
  const { accounts } = req.body;
  fetchLog(`[BATCH] 收到提取请求，账号数量: ${accounts?.length || 0}`);

  if (!Array.isArray(accounts) || accounts.length === 0) {
    return res.status(400).json({ message: 'accounts 数组不能为空' });
  }

  const results = await Promise.all(
    accounts.map(acc => fetchTokensForAccount(acc))
  );

  res.json({ results });
});

async function fetchTokensForAccount(acc) {
  const { id, site_name, site_url, site_type, account_info } = acc;
  const apiKey = account_info?.access_token;
  const baseUrl = (site_url || '').replace(/\/+$/, '');

  if (!apiKey || !baseUrl) {
    return { id, site_name, site_url, tokens: [], error: '缺少 access_token 或 site_url' };
  }

  // 按 site_type 决定端点优先级
  let endpoints;
  if (site_type === 'sub2api') {
    endpoints = [
      `/api/v1/keys?page=1&page_size=100`,
      `/api/token/?p=0&size=100`,
    ];
  } else {
    endpoints = [
      `/api/token/?p=0&size=100`,
      `/api/token`,
      `/api/token/?p=1&size=100`,
      `/api/v1/keys?page=1&page_size=100`,
    ];
  }

  for (const endpoint of endpoints) {
    try {
      const url = `${baseUrl}${endpoint}`;
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 10000); // 10s 超时

      const response = await fetch(url, {
        method: 'GET',
        headers: {
          Authorization: `Bearer ${apiKey}`,
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        signal: controller.signal,
        redirect: 'follow',
      });

      clearTimeout(timeout);

      if (!response.ok) {
        // 401/404 继续试下一个
        continue;
      }

      const body = await response.json();

      // 解析不同格式的响应
      let items = extractItems(body);

      if (items && items.length > 0) {
        fetchLog(`[SUCCESS] ${site_name} | 从 ${endpoint} 提取出 ${items.length} 个 Token`);
        return { id, site_name, site_url, tokens: items, endpoint };
      }
    } catch (err) {
      // 超时或网络错误，继续下一个端点
      fetchLog(`[WARN] ${site_name} | ${endpoint} 提取异常: ${err.message || err}`);
    }
  }

  fetchLog(`[FAIL] ${site_name} | 所有端点提取均失败`);
  return { id, site_name, site_url, tokens: [], error: '所有端点均未获取到 Token' };
}

function extractItems(body) {
  // 格式1: { data: [...] } 标准 NewAPI/OneAPI
  if (body && body.data !== undefined) {
    const data = body.data;
    if (Array.isArray(data)) return data;
    // 格式2: { data: { items: [...] } } 分页封装
    if (data && Array.isArray(data.items)) return data.items;
  }
  // 格式3: { items: [...] }
  if (body && Array.isArray(body.items)) return body.items;
  // 格式4: 直接是数组
  if (Array.isArray(body)) return body;
  return [];
}

export default router;
