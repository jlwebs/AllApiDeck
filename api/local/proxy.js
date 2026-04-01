// api/local/proxy.js
// 代理逻辑: 提供 GET 代理(探测模型)与 POST 代理(检测 API 密钥)
// 检测核心逻辑统一在 checkKey.js，开发/生产共用

import express from 'express';
import fs from 'fs';
import path from 'path';
import { checkKey } from './checkKey.js';

const router = express.Router();

const logDir = 'logs';
if (!fs.existsSync(logDir)) fs.mkdirSync(logDir);
const checkLogStream = fs.createWriteStream(path.join(logDir, 'check-keys.log'), { flags: 'a' });

function checkLog(msg) {
  const timestamp = new Date().toLocaleString();
  const fullMsg = `[${timestamp}] ${msg}`;
  checkLogStream.write(fullMsg + '\n');
  console.log(fullMsg);
}

// GET /api/proxy-get?url=...
// 用于探测模型列表，支持 Authorization 透传
router.get('/proxy-get', async (req, res) => {
  const targetUrl = req.query.url;
  const auth = req.headers.authorization;

  if (!targetUrl) return res.status(400).json({ message: '缺少 url 参数' });

  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);

    const response = await fetch(targetUrl, {
      method: 'GET',
      headers: {
        'Authorization': auth,
        'Accept': 'application/json'
      },
      signal: controller.signal
    });
    
    clearTimeout(timeout);

    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('application/json')) {
      const data = await response.json();
      res.status(response.status).json(data);
    } else {
      const text = await response.text();
      const titleMatch = text.match(/<title>(.*?)<\/title>/i);
      const title = (titleMatch ? titleMatch[1] : 'HTML Payload').substring(0, 100);
      res.status(response.status).json({ 
        message: 'Invalid JSON Response', 
        htmlTitle: title,
        htmlSnippet: text.substring(0, 500).replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
      });
    }
  } catch (err) {
    if (err.name === 'AbortError') {
      return res.status(504).json({ message: '代理请求超时' });
    }
    res.status(500).json({ message: '代理请求失败', error: err.message });
  }
});

// POST /api/check-key — 调用统一的 checkKey 模块
router.post('/check-key', async (req, res) => {
  try {
    const result = await checkKey(req.body, checkLog);
    res.status(result.status).json(result.body);
  } catch (err) {
    checkLog(`[CHECK] 异常: ${err.message}`);
    res.status(500).json({ error: { message: err.message } });
  }
});

export default router;
