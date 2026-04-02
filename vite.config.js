import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import fs from 'fs';
import path from 'path';
import Components from 'unplugin-vue-components/vite';
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers';

// ─── 配置与辅助函数 ────────────────────────────────────────────────────────
const FETCH_LOG = path.join(process.cwd(), 'logs/fetch-keys.log');
const CHECK_LOG = path.join(process.cwd(), 'logs/check-keys.log');

if (!fs.existsSync('logs')) fs.mkdirSync('logs');

const fetchLog = (msg) => {
  const line = `[${new Date().toLocaleTimeString()}] ${msg}\n`;
  fs.appendFileSync(FETCH_LOG, line);
  console.log(line.trim());
};

const checkLog = (msg) => {
  const line = `[${new Date().toLocaleTimeString()}] ${msg}\n`;
  fs.appendFileSync(CHECK_LOG, line);
};

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

/**
 * 掩码 Key 递归解析
 */
async function resolveFullKey(baseUrl, tokenId, authValue, compatHeaders, siteName, retryCount = 0) {
  const MAX_RETRIES = 5;
  const url = `${baseUrl}/api/token/${tokenId}/key`;
  try {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 12000);
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        'Authorization': authValue,
        'Accept': 'application/json',
        'Content-Type': 'application/json',
        'Pragma': 'no-cache',
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
        ...compatHeaders,
      },
      signal: ctrl.signal,
    });
    clearTimeout(timer);
    
    if (res.status === 429 && retryCount < MAX_RETRIES) {
      const delay = Math.min(10000, Math.pow(2, retryCount) * 1000) + Math.random() * 1000;
      fetchLog(`[${siteName}] [Resolve] token#${tokenId} 限流(429)，${(delay/1000).toFixed(1)}s 后重试 (${retryCount + 1}/${MAX_RETRIES})`);
      await new Promise(r => setTimeout(r, delay));
      return resolveFullKey(baseUrl, tokenId, authValue, compatHeaders, siteName, retryCount + 1);
    }

    if (res.ok) {
      const data = await res.json();
      const key = data.data || data.key || (typeof data === 'string' ? data : '');
      if (key && typeof key === 'string' && key.trim()) {
        return ensureSkPrefix(key.trim());
      }
    }
  } catch (err) {
    fetchLog(`[${siteName}] [Resolve] token#${tokenId} 失败: ${err.message}`);
  }
  return null;
}

/**
 * 核心：提取账号的所有令牌 (参考 all-api-hub 分页机制)
 */
async function fetchTokensForAccount(account) {
  const { site_url, site_name, account_info, id, api_key } = account;
  const access_token = account.access_token || account_info?.access_token;
  
  if (!site_url || !access_token) {
    if (!site_url) fetchLog(`[${site_name || '未知'}] 缺失 URL，跳过`);
    else fetchLog(`[${site_name}] 缺失 AccessToken，跳过`);
    return { ...account, tokens: [], error: '账号数据不完整 (缺少 URL 或 Token)' };
  }

  const baseUrl = site_url.replace(/\/+$/, '');
  const userId = String(account_info?.id || '');

  const compatHeaders = {
    'New-API-User': userId,
    'Veloera-User': userId,
    'voapi-user': userId,
    'User-id': userId,
    'Rix-Api-User': userId,
    'neo-api-user': userId,
  };

  const browserHeaders = {
    'Accept': 'application/json, text/plain, */*',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
    'Referer': `${baseUrl}/`,
    'X-Requested-With': 'XMLHttpRequest',
  };

  const authValue = String(access_token).startsWith('Bearer ') ? access_token : `Bearer ${access_token}`;
  const allTokens = [];
  const pageSize = 100;
  const maxPages = 10;
  
  // 试探路径：包含 New-API/One-API 标准路径以及 Sub2API 专用路径 (/api/v1/keys)
  const baseEndpoints = [
    `${baseUrl}/api/token/`, 
    `${baseUrl}/api/token`,
    `${baseUrl}/api/v1/keys`
  ].sort((a,b) => b.length - a.length);
  
  for (const baseEndpoint of baseEndpoints) {
    if (allTokens.length > 0) break;
    
    for (let p = 0; p < maxPages; p++) {
      const url = `${baseEndpoint}?p=${p}&size=${pageSize}`;
      try {
        const ctrl = new AbortController();
        const timeout = setTimeout(() => ctrl.abort(), 10000);
        const res = await fetch(url, {
          headers: { 'Authorization': authValue, ...browserHeaders, ...compatHeaders },
          signal: ctrl.signal
        });
        clearTimeout(timeout);
        
        if (res.ok) {
          const json = await res.json();
          const items = extractItems(json);
          if (items.length === 0) break; 
          
          allTokens.push(...items);
          fetchLog(`[${site_name}] 第 ${p} 页获取成功，共 ${items.length} 个`);
          if (items.length < pageSize) break; 
        } else {
          if (p === 0) fetchLog(`[${site_name}] 提取失败(${res.status}): ${url}`);
          break;
        }
      } catch (err) {
        if (p === 0) fetchLog(`[${site_name}] 异常: ${url} | ${err.message}`);
        break;
      }
    }
  }

  if (allTokens.length === 0) {
    return { ...account, tokens: [], error: '未能获取到任何令牌' };
  }

  // 递归解析掩码 Key
  const resolvedTokens = [];
  for (const t of allTokens) {
    const rawKey = t.key || t.access_token || '';
    if (isMaskedKey(rawKey) && t.id) {
       const full = await resolveFullKey(baseUrl, t.id, authValue, compatHeaders, site_name);
       if (full) resolvedTokens.push({ ...t, key: full });
    } else if (rawKey && rawKey.length > 5) {
       resolvedTokens.push({ ...t, key: ensureSkPrefix(rawKey) });
    }
  }

  return { id, site_name, site_url, api_key, tokens: resolvedTokens, account_info, access_token };
}

// ─── Vite 插件：混合代理中间件 ───────────────────────────────────────────────
function proxyMiddlewarePlugin() {
  return {
    name: 'proxy-middleware',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        // 0. 清空日志接口
        if (req.url.startsWith('/api/clear-logs')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          const params = new URL(req.url, 'http://localhost').searchParams;
          const target = params.get('type') || 'check';
          const file = target === 'fetch' ? FETCH_LOG : CHECK_LOG;
          fs.writeFileSync(file, '', 'utf8');
          return res.end(JSON.stringify({ success: true }));
        }

        // 1. 批量提取 Token 接口
        if (req.url.startsWith('/api/fetch-keys')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
             try {
               const { accounts } = JSON.parse(body);
               fetchLog(`开始批量提取 ${accounts.length} 个站点的令牌...`);
               const results = new Array(accounts.length);
               let currentIndex = 0;
               const CONCURRENCY = 25;
               const worker = async () => {
                 while (currentIndex < accounts.length) {
                   const i = currentIndex++;
                   results[i] = await fetchTokensForAccount(accounts[i]);
                 }
               };
               await Promise.all(Array.from({ length: CONCURRENCY }).map(worker));
               const total = results.reduce((n, r) => n + (r.tokens?.length || 0), 0);
               fetchLog(`===== 完成提取，共 ${total} 个令牌 =====`);
               res.setHeader('Content-Type', 'application/json');
               res.end(JSON.stringify({ results }));
             } catch (err) {
               res.statusCode = 500;
               res.end(JSON.stringify({ message: err.message }));
             }
          });
          return;
        }

        // 2. 模型发现接口代理
        if (req.url.startsWith('/api/proxy-get')) {
          const query = new URL(req.url, 'http://localhost').searchParams;
          const targetUrl = query.get('url');
          const uid = query.get('uid') || '';
          const auth = req.headers['authorization'];
          if (!targetUrl) { res.statusCode = 400; return res.end('URL Required'); }

          const baseUrl = new URL(targetUrl).origin;
          const compatHeaders = {
            'New-API-User': uid, 'Veloera-User': uid, 'voapi-user': uid,
            'User-id': uid, 'Rix-API-User': uid, 'neo-api-user': uid,
          };
          const browserHeaders = {
            'Accept': 'application/json, text/plain, */*',
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
            'Referer': `${baseUrl}/`,
            'X-Requested-With': 'XMLHttpRequest',
          };

          try {
            checkLog(`[PROXY-GET] 正在请求: ${targetUrl} | UID: ${uid}`);
            const ctrl = new AbortController();
            const timeout = setTimeout(() => ctrl.abort(), 10000);
            const apiRes = await fetch(targetUrl, {
              headers: { 'Authorization': auth || '', ...browserHeaders, ...compatHeaders },
              signal: ctrl.signal
            });
            clearTimeout(timeout);
            const data = await apiRes.text();
            res.statusCode = apiRes.status;
            res.setHeader('Content-Type', apiRes.headers.get('content-type') || 'application/json');
            if (!apiRes.ok) {
               checkLog(`[PROXY-GET] 失败(${apiRes.status}): ${targetUrl} | 长度:${data.length} | 预览:${data.slice(0,200)}`);
            } else {
               checkLog(`[PROXY-GET] 成功(${apiRes.status}): ${targetUrl} | 长度:${data.length}`);
            }
            return res.end(data);
          } catch (err) {
            checkLog(`[PROXY-GET] 异常: ${err.message}`);
            res.statusCode = 500; return res.end(JSON.stringify({ message: err.message }));
          }
        }

        // 3. 密钥/模型可用性检测
        if (req.url.startsWith('/api/check-key')) {
          if (req.method !== 'POST') { res.statusCode = 405; return res.end(); }
          let body = '';
          req.on('data', chunk => { body += chunk; });
          req.on('end', async () => {
            try {
              const { site, tokenKey, model } = JSON.parse(body);
              const apiBaseUrl = site.api_key?.startsWith('http') 
                ? site.api_key.replace(/\/+$/, '') 
                : site.site_url.replace(/\/+$/, '');
              
              const uid = String(site?.account_info?.id || site?.id || '');
              const authValue = ensureSkPrefix(tokenKey);
              const checkUrl = `${apiBaseUrl}/v1/chat/completions`;
              
              const compatHeaders = {
                'New-API-User': uid, 'Veloera-User': uid, 'voapi-user': uid,
                'User-id': uid, 'Rix-API-User': uid, 'neo-api-user': uid,
              };

              const start = Date.now();
              const ctrl = new AbortController();
              const timeout = setTimeout(() => ctrl.abort(), 30000);
              const resApi = await fetch(checkUrl, {
                method: 'POST',
                headers: {
                  'Authorization': `Bearer ${authValue}`,
                  'Content-Type': 'application/json',
                  'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36',
                  ...compatHeaders
                },
                body: JSON.stringify({
                  model: model,
                  messages: [{ role: 'user', content: 'Ping' }],
                  max_tokens: 1
                }),
                signal: ctrl.signal
              });
              clearTimeout(timeout);

              const duration = ((Date.now() - start) / 1000).toFixed(2);
              const status = resApi.status;
              if (resApi.ok) {
                checkLog(`[CHECK] 成功: ${site.site_name} | ${model} | ${duration}s`);
                res.end(JSON.stringify({ success: true, duration, status }));
              } else {
                const errBody = await resApi.text();
                checkLog(`[CHECK] 失败: ${site.site_name} | ${model} | HTTP ${status} | ${duration}s | ${errBody.slice(0,300)}`);
                res.end(JSON.stringify({ success: false, duration, status, message: errBody }));
              }
            } catch (err) {
              checkLog(`[CHECK] 异常: ${err.message}`);
              res.statusCode = 500; res.end(JSON.stringify({ success: false, message: err.message }));
            }
          });
          return;
        }

        next();
      });
    }
  };
}

export default defineConfig({
  plugins: [
    vue(),
    Components({
      resolvers: [AntDesignVueResolver({ importStyle: false, resolveIcons: true })],
    }),
    proxyMiddlewarePlugin(),
  ],
  server: { port: 3000, host: '0.0.0.0', watch: { ignored: ['**/all-api-hub/**'] } },
  resolve: { alias: { '@': '/src' } },
  optimizeDeps: {
    exclude: ['all-api-hub'],
    entries: ['index.html', 'src/**/*.vue']
  },
  css: {
    preprocessorOptions: {
      less: {
        modifyVars: { hack: `true; @import "~ant-design-vue/lib/style/themes/dark.less";` },
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
