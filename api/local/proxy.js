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
// 核心策略：始终使用 stream:true 发送请求，在 proxy 层组装 SSE 流为统一 JSON
// 原因：上游 API 网关 (one-api/new-api 等 Go 服务) 对某些思维链模型在 stream:false 下
//       无法正确将流式响应转为非流式 JSON，报 "invalid character 'd'" 错误。
//       而 Cherry Studio 等客户端默认使用 stream:true 则完全正常。
router.post('/check-key', async (req, res) => {
  const { url, key, model, messages } = req.body;
  const baseUrl = (url || '').replace(/\/+$/, '');
  const targetUrl = `${baseUrl}/v1/chat/completions`;

  checkLog(`[CHECK] 开始测试: ${baseUrl} | ${model}`);

  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 55000); // 55秒超时（思维链模型需要更长时间）

    const response = await fetch(targetUrl, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${key}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        model: model,
        messages: messages || [{ role: 'user', content: 'hi' }],
        stream: true
      }),
      signal: controller.signal
    });
    
    clearTimeout(timeout);

    // ── 非 2xx 响应：直接读取错误信息 ──
    if (!response.ok) {
      let errMsg = `HTTP ${response.status}`;
      let errorData = null;
      try {
        const text = await response.text();
        // 尝试解析为 JSON 错误
        try {
          errorData = JSON.parse(text);
          errMsg = errorData.error?.message || errorData.message || errMsg;
        } catch {
          // 可能是 HTML 错误页
          const titleMatch = text.match(/<title>(.*?)<\/title>/i);
          if (titleMatch) {
            errMsg = `(HTML) ${titleMatch[1].substring(0, 100)}`;
          }
        }
      } catch {}
      checkLog(`[FAIL] ${baseUrl} | ${model} | ${errMsg}`);
      return res.status(response.status).json({ message: errMsg, error: errorData?.error });
    }

    // ── 2xx 响应：解析内容 ──
    const contentType = response.headers.get('content-type') || '';
    
    // 某些服务端可能仍然返回完整 JSON (即使我们请求了 stream:true)
    if (contentType.includes('application/json')) {
      const data = await response.json();
      if (data.choices) {
        checkLog(`[SUCCESS] ${baseUrl} | ${model} | 响应成功 (非流式回退)`);
        return res.json({
          model: data.model || model,
          choices: data.choices,
          usage: data.usage,
          message: 'success'
        });
      }
      // 如果是错误 JSON
      const errMsg = data.error?.message || data.message || 'Unknown error';
      checkLog(`[FAIL] ${baseUrl} | ${model} | ${errMsg}`);
      return res.status(400).json({ message: errMsg, error: data.error });
    }

    // ── 核心：逐行读取 SSE 流，组装结果 ──
    const text = await response.text();
    const lines = text.split('\n');
    
    let returnedModel = '';
    let content = '';
    let reasoningContent = '';
    let usage = null;
    let chunkCount = 0;

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || !trimmed.startsWith('data:')) continue;
      
      const jsonStr = trimmed.slice(5).trim(); // 去掉 "data:" 前缀
      if (jsonStr === '[DONE]') break;
      
      try {
        const chunk = JSON.parse(jsonStr);
        chunkCount++;
        
        // 提取模型名（通常在第一个 chunk 中）
        if (chunk.model && !returnedModel) {
          returnedModel = chunk.model;
        }
        
        // 提取 usage（通常在最后一个 chunk 中）
        if (chunk.usage) {
          usage = chunk.usage;
        }
        
        // 提取 delta 内容
        const delta = chunk.choices?.[0]?.delta;
        if (delta) {
          if (delta.content) content += delta.content;
          if (delta.reasoning_content) reasoningContent += delta.reasoning_content;
          if (delta.thinking) reasoningContent += delta.thinking;
        }
      } catch {
        // 跳过无法解析的行
      }
    }

    // ── 组装统一结果返回前端 ──
    if (chunkCount > 0) {
      const isThinkingModel = reasoningContent.length > 0;
      checkLog(`[SUCCESS] ${baseUrl} | ${model} | 流式响应成功 (${chunkCount} chunks${isThinkingModel ? ', thinking' : ''})`);
      res.json({
        model: returnedModel || model,
        choices: [{
          message: {
            role: 'assistant',
            content: content || null,
            reasoning_content: reasoningContent || undefined
          }
        }],
        usage: usage,
        isStreamAssembled: true,
        message: 'success'
      });
    } else {
      // 没有有效 chunk，可能是空响应或格式完全异常
      checkLog(`[FAIL] ${baseUrl} | ${model} | 流式响应无有效数据块`);
      res.status(502).json({ message: '流式响应无有效数据块 (0 chunks)', error: { message: text.substring(0, 200) } });
    }
  } catch (err) {
    checkLog(`[ERROR] ${baseUrl} | ${model} | ${err.message}`);
    if (err.name === 'AbortError') {
      res.status(504).json({ message: '请求超时 (55s)', error: err.message });
    } else {
      res.status(500).json({ message: '请求异常', error: err.message });
    }
  }
});

export default router;
