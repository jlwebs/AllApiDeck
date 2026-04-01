// api/local/proxy.js
// 代理逻辑:提供 GET 代理(探测模型)与 POST 代理(检测 API 密钥)

import express from 'express';
import fs from 'fs';
import path from 'path';

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
    const timeout = setTimeout(() => controller.abort(), 10000); // 10秒超时防卡死

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

// POST /api/check-key
// 用于批量检测 API 密钥及其模型可用性
router.post('/check-key', async (req, res) => {
  const { url, key, model, messages } = req.body;
  const baseUrl = (url || '').replace(/\/+$/, '');
  const targetUrl = `${baseUrl}/v1/chat/completions`;

  checkLog(`[CHECK] 开始测试: ${baseUrl} | ${model}`);

  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 30000); // 30秒超时限制，防止批量测活卡死挂起

    const response = await fetch(targetUrl, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${key}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        model: model,
        messages: messages || [{ role: 'user', content: 'hi' }],
        stream: false
      }),
      signal: controller.signal
    });
    
    clearTimeout(timeout);

    const contentType = response.headers.get('content-type') || '';
    let data;
    if (contentType.includes('application/json')) {
      data = await response.json();
    } else {
      const text = await response.text();
      const titleMatch = text.match(/<title>(.*?)<\/title>/i);
      const title = (titleMatch ? titleMatch[1] : 'HTML Payload').substring(0, 100);
      data = { 
        message: 'Invalid JSON Response', 
        htmlTitle: title,
        htmlSnippet: text.substring(0, 50).replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
      };
    }

    if (response.ok && contentType.includes('application/json')) {
      checkLog(`[SUCCESS] ${baseUrl} | ${model} | 响应成功`);
      res.json({
        model: data.model || model,
        choices: data.choices,
        usage: data.usage,
        message: 'success'
      });
    } else {
      const errMsg = data.htmlTitle ? `[HTML] ${data.htmlTitle}` : (data.error?.message || data.message || `HTTP ${response.status}`);
      checkLog(`[FAIL] ${baseUrl} | ${model} | ${errMsg}`);
      res.status(response.status).json({ 
        message: errMsg,
        error: data.error,
        htmlTitle: data.htmlTitle,
        htmlSnippet: data.htmlSnippet,
        raw: data
      });
    }
  } catch (err) {
    checkLog(`[ERROR] ${baseUrl} | ${model} | ${err.message}`);
    res.status(500).json({ message: '请求异常', error: err.message });
  }
});

export default router;
