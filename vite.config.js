import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import Components from 'unplugin-vue-components/vite';
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers';
import { visualizer } from 'rollup-plugin-visualizer';
import fs from 'node:fs';
import path from 'node:path';

// ─── 日志工具 ───────────────────────────────────────────────────────────────
const LOG_DIR = path.resolve('./logs');
const FETCH_LOG = path.join(LOG_DIR, 'fetch-keys.log');
const CHECK_LOG = path.join(LOG_DIR, 'check-keys.log');

function writeLog(file, msg) {
  if (!fs.existsSync(LOG_DIR)) fs.mkdirSync(LOG_DIR, { recursive: true });
  const line = `[${new Date().toISOString()}] ${msg}\n`;
  fs.appendFileSync(file, line, 'utf8');
  console.log(`[${path.basename(file)}]`, msg);
}

// 快捷方法
const fetchLog = (msg) => writeLog(FETCH_LOG, msg);
const checkLog = (msg) => writeLog(CHECK_LOG, msg);

// ─── 响应解析 ────────────────────────────────────────────────────────────────
function extractItems(body) {
  if (body === null || body === undefined) return [];
  if (Array.isArray(body)) return body;
  if (body.data !== undefined) {
    if (Array.isArray(body.data)) return body.data;
    if (body.data && Array.isArray(body.data.items)) return body.data.items;
    if (body.data && body.data.data && Array.isArray(body.data.data)) return body.data.data;
  }
  if (Array.isArray(body.items)) return body.items;
  if (Array.isArray(body.list)) return body.list;
  if (Array.isArray(body.keys)) return body.keys;
  if (Array.isArray(body.tokens)) return body.tokens;
  return [];
}

/** 与插件 isMaskedApiTokenKey 完全一致 - 含星号则为掩码 key */
function isMaskedKey(key) {
  return typeof key === 'string' && key.includes('*');
}

/** 确保 sk- 前缀 */
function ensureSkPrefix(key) {
  if (!key) return key;
  const t = key.trim();
  return /^sk-/i.test(t) ? t : `sk-${t}`;
}

/**
 * 对掩码 key，调用 POST /api/token/{id}/key 获取完整密钥。
 */
async function resolveFullKey(baseUrl, tokenId, authValue, compatHeaders, siteName) {
  const url = `${baseUrl}/api/token/${tokenId}/key`;
  try {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 8000);
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        'Authorization': authValue,
        'Accept': 'application/json',
        'Content-Type': 'application/json',
        'Pragma': 'no-cache',
        ...compatHeaders,
      },
      signal: ctrl.signal,
    });
    clearTimeout(timer);
    
    const status = res.status;
    const rawText = await res.text();
    
    if (!res.ok) {
      fetchLog(`[${siteName}] [Resolve] token#${tokenId} 失败: HTTP ${status}, 响应: ${rawText.slice(0, 100)}`);
      return null;
    }

    let json;
    try {
      json = JSON.parse(rawText);
    } catch {
      fetchLog(`[${siteName}] [Resolve] token#${tokenId} 响应非JSON: ${rawText.slice(0, 100)}`);
      return null;
    }

    const key = json?.data?.key ?? json?.data ?? json?.key ?? null;
    if (key && typeof key === 'string') {
      const finalKey = ensureSkPrefix(key);
      if (isMaskedKey(finalKey)) {
        fetchLog(`[${siteName}] [Resolve] token#${tokenId} 警告: 获取到的依然是掩码 key!`);
        return null;
      }
      return finalKey;
    }
    return null;
  } catch (err) {
    fetchLog(`[${siteName}] [Resolve] token#${tokenId} 异常: ${err.message}`);
    return null;
  }
}

function summarizeShape(obj) {
  if (Array.isArray(obj))
    return `Array[${obj.length}] keys=${JSON.stringify(Object.keys(obj[0] || {}))}`;
  if (obj && typeof obj === 'object')
    return Object.fromEntries(
      Object.entries(obj).map(([k, v]) => [k, Array.isArray(v) ? `Array[${v.length}]` : typeof v])
    );
  return String(obj);
}


// ─── 核心：服务端代抓取 Token ──────────────────────────────────────────────
async function fetchTokensForAccount(acc) {
  const { id, site_name, site_url, site_type, account_info } = acc;
  const apiKey = account_info?.access_token;
  const userId = account_info?.id;
  const baseUrl = (site_url || '').replace(/\/+$/, '');

  fetchLog(`[${site_name}] >>> 开始处理 (UID: ${userId})`);

  if (!apiKey || !baseUrl) return { id, site_name, site_url, tokens: [], error: '缺少 access_token 或 site_url' };

  const isSub2Api = site_type === 'sub2api';
  const endpoints = isSub2Api
    ? ['/api/v1/keys?page=1&page_size=500', '/api/token/?p=0&size=500']
    : ['/api/token/?p=0&size=500', '/api/token?p=0&size=500', '/api/token/', '/api/token', '/api/v1/keys?page=1&page_size=500'];

  const userIdStr = userId ? String(userId) : null;
  const compatUserHeaders = userIdStr ? {
    'New-API-User': userIdStr, 'Veloera-User': userIdStr, 'voapi-user': userIdStr,
    'User-id': userIdStr, 'Rix-Api-User': userIdStr, 'neo-api-user': userIdStr,
  } : {};

  const authValues = isSub2Api ? [`Bearer ${apiKey}`] : [`Bearer ${apiKey}`, apiKey];

  // 竞速模型：所有端点组合一起跑，哪个先拿到且有Token，就保留哪个退出
  const fetchTasks = [];

  for (const endpoint of endpoints) {
    for (const authValue of authValues) {
      const url = `${baseUrl}${endpoint}`;
      
      const task = (async () => {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), 8000); // 8s 超时即可
        
        let response;
        try {
          response = await fetch(url, {
            method: 'GET',
            headers: {
              'Authorization': authValue,
              'Accept': 'application/json',
              'Content-Type': 'application/json',
              'Pragma': 'no-cache',
              'User-Agent': 'Mozilla/5.0 ApiChecker/1.0',
              ...compatUserHeaders,
            },
            signal: controller.signal,
            redirect: 'follow',
          });
          clearTimeout(timer);
        } catch (err) {
          clearTimeout(timer);
          throw new Error(`请求失败: ${err.message}`);
        }

        if (!response.ok) throw new Error(`HTTP ${response.status}`);

        const rawText = await response.text();
        let bodyJson;
        try { bodyJson = JSON.parse(rawText); } catch { throw new Error('非JSON响应'); }

        const rawItems = extractItems(bodyJson);
        if (rawItems.length === 0) throw new Error('没有可用数据项');

        const resolvedItems = [];
        for (const token of rawItems) {
          const rawKey = token.key || '';
          const tokenId = token.id;
          if (!isMaskedKey(rawKey) && rawKey.length > 10) {
            resolvedItems.push({ ...token, key: ensureSkPrefix(rawKey) });
          } else if (tokenId) {
            const fullKey = await resolveFullKey(baseUrl, tokenId, authValue, compatUserHeaders, site_name);
            if (fullKey) resolvedItems.push({ ...token, key: fullKey });
          }
        }
        
        if (resolvedItems.length > 0) {
          // 核心修复：必须带回原始的数字 userId，而不是随机生成的 UUID (id)
          return { id, site_name, site_url, access_token: apiKey, tokens: resolvedItems, endpoint, account_info: { id: userId, access_token: apiKey } };
        } else {
          throw new Error('有效解析后为0');
        }
      })();
      
      fetchTasks.push(task);
    }
  }

  try {
    const result = await Promise.any(fetchTasks);
    fetchLog(`[${site_name}] --- 竞速夺魁：从 ${result.endpoint} 获取到 ${result.tokens.length} 个可用 Token ---`);
    return result;
  } catch (err) {
    // 所有 promise 都抛出错误时
    fetchLog(`[${site_name}] 所有探测组合均失败`);
    return { id, site_name, site_url, tokens: [], error: '未能获取到有效 Token', account_info: { id: userId } };
  }
}

// ─── Vite 插件：混合代理中间件 ───────────────────────────────────────────────
function proxyMiddlewarePlugin() {
  return {
    name: 'proxy-middleware',
    configureServer(server) {
      // 1. 批量提取 Token 接口
      server.middlewares.use('/api/fetch-keys', (req, res) => {
        if (req.method !== 'POST') { res.statusCode = 405; res.end(); return; }
        let body = '';
        req.on('data', chunk => { body += chunk; });
        req.on('end', async () => {
          try {
            const { accounts } = JSON.parse(body);
            const results = await Promise.all(accounts.map(fetchTokensForAccount));
            const total = results.reduce((n, r) => n + r.tokens.length, 0);
            fetchLog(`===== 完成提取，共 ${total} 个 Token =====`);
            res.setHeader('Content-Type', 'application/json');
            res.end(JSON.stringify({ results }));
          } catch (err) {
            res.statusCode = 400;
            res.end(JSON.stringify({ message: err.message }));
          }
        });
      });

      // 2. 批量检测代理接口 — 逻辑统一在 api/local/checkKey.js
      server.middlewares.use('/api/check-key', (req, res) => {
        if (req.method !== 'POST') { res.statusCode = 405; res.end(); return; }
        let body = '';
        req.on('data', chunk => { body += chunk; });
        req.on('end', async () => {
          try {
            const params = JSON.parse(body);
            if (params._isFirst) fs.writeFileSync(CHECK_LOG, '', 'utf8');
            const { checkKey } = await import('./api/local/checkKey.js');
            const result = await checkKey(params, checkLog);
            res.statusCode = result.status;
            res.setHeader('Content-Type', 'application/json');
            res.end(JSON.stringify(result.body));
          } catch (err) {
            checkLog(`[CHECK] 异常: ${err.message}`);
            res.statusCode = 500;
            res.setHeader('Content-Type', 'application/json');
            res.end(JSON.stringify({ error: { message: err.message } }));
          }
        });
      });

      // 3. 通用 GET 代理 (获取模型列表、额度等)
      server.middlewares.use('/api/proxy-get', (req, res) => {
        if (req.method !== 'GET') { res.statusCode = 405; res.end(); return; }
        
        const params = new URL(req.url, 'http://localhost').searchParams;
        const targetUrl = params.get('url');
        const queryUid = params.get('uid');
        const authHeader = req.headers['authorization'];

        if (!targetUrl) { res.statusCode = 400; res.end('Missing url'); return; }

        (async () => {
          try {
            checkLog(`[PROXY-GET] 正在请求: ${targetUrl} | UID: ${queryUid || '无'}`);
            const ctrl = new AbortController();
            const timer = setTimeout(() => ctrl.abort(), 15000);

            const finalHeaders = {
              'Accept': 'application/json',
              'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
              'Authorization': authHeader || '',
              'Pragma': 'no-cache',
              'Cache-Control': 'no-cache'
            };

            // 核心修复：只有 UID 存在且为纯数字时才发送兼容头。UUID 会导致 401 格式错误。
            if (queryUid && /^\d+$/.test(queryUid)) {
              const uid = String(queryUid);
              finalHeaders['New-Api-User'] = uid;
              finalHeaders['Veloera-User'] = uid;
              finalHeaders['voapi-user'] = uid;
              finalHeaders['User-id'] = uid;
              finalHeaders['Rix-Api-User'] = uid;
              finalHeaders['neo-api-user'] = uid;
            }

            const response = await fetch(targetUrl, {
              method: 'GET',
              headers: finalHeaders,
              signal: ctrl.signal,
            });
            clearTimeout(timer);

            const status = response.status;
            const resText = await response.text();
            const contentType = response.headers.get('content-type') || '';
            
            if (status === 401 || status === 403) {
              checkLog(`[PROXY-GET] 鉴权失败(${status}): ${targetUrl} | 头: ${JSON.stringify(finalHeaders)} | 响应: ${resText.slice(0, 200)}`);
            } else {
              checkLog(`[PROXY-GET] 响应(${status}): ${targetUrl} | 长度: ${resText.length}`);
            }

            res.statusCode = status;
            res.setHeader('Content-Type', 'application/json');
            
            if (contentType.includes('application/json')) {
              res.end(resText);
            } else {
              const titleMatch = resText.match(/<title>(.*?)<\/title>/i);
              const title = (titleMatch ? titleMatch[1] : 'HTML Payload').substring(0, 100);
              res.end(JSON.stringify({ message: 'Invalid JSON Response', htmlTitle: title, htmlSnippet: resText.slice(0, 500) }));
            }
          } catch (err) {
            checkLog(`[PROXY-GET] 异常: ${targetUrl} | ${err.message}`);
            res.statusCode = 500;
            res.end(JSON.stringify({ error: err.message }));
          }
        })();
      });
    },
  };
}

// ─── Vite 配置 ───────────────────────────────────────────────────────────────
export default defineConfig({
  plugins: [
    vue(),
    visualizer({ open: false }),
    Components({
      resolvers: [
        AntDesignVueResolver({
          importStyle: false,
          resolveIcons: true,
        }),
      ],
    }),
    proxyMiddlewarePlugin(),
  ],

  server: {
    port: 3000,
    host: '0.0.0.0',
  },
  resolve: {
    alias: { '@': '/src' },
  },
  css: {
    preprocessorOptions: {
      less: {
        modifyVars: {
          hack: `true; @import "~ant-design-vue/lib/style/themes/dark.less";`,
        },
        javascriptEnabled: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('ant-design-vue')) return 'ant-design-vue';
            if (id.includes('lodash')) return 'lodash';
            return 'vendor';
          }
        },
      },
    },
  },
});
