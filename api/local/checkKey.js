// api/local/checkKey.js
// 统一的 API 密钥检测核心逻辑
// vite.config.js (开发) 和 server.js/proxy.js (生产) 共用此模块

/**
 * 检测单个 API 密钥 + 模型的可用性
 * 始终使用 stream:true 发送请求，在服务端组装 SSE 流为统一 JSON
 * 
 * @param {Object} params - { url, key, model, messages }
 * @param {Function} log - 日志函数
 * @returns {Object} { status, body } - status 为 HTTP 状态码，body 为 JSON 对象
 */
export async function checkKey({ url, key, model, messages, uid }, log = console.log) {
  const baseUrl = (url || '').replace(/\/+$/, '');
  const targetUrl = `${baseUrl}/v1/chat/completions`;

  log(`[CHECK] 正在测试: ${baseUrl} | Model: ${model} | Key: ${key?.slice(0, 12)}...`);

  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 55000); // 55秒超时（思维链模型需要更长）

    const startTime = Date.now();
    const compatHeaders = /^\d+$/.test(String(uid || ''))
      ? {
        'New-Api-User': String(uid),
        'Veloera-User': String(uid),
        'voapi-user': String(uid),
        'User-id': String(uid),
        'Rix-Api-User': String(uid),
        'neo-api-user': String(uid),
      }
      : {};

    const response = await fetch(targetUrl, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${key}`,
        'Content-Type': 'application/json',
        'User-Agent': 'Mozilla/5.0 ApiChecker/1.0',
        ...compatHeaders,
      },
      body: JSON.stringify({
        model,
        messages: messages || [{ role: 'user', content: 'hi' }],
        stream: true,
      }),
      signal: controller.signal,
    });
    clearTimeout(timer);

    const duration = ((Date.now() - startTime) / 1000).toFixed(2);
    const status = response.status;
    const contentType = response.headers.get('content-type') || '';

    // ── 非 2xx：直接读取错误 ──
    if (!response.ok) {
      const errText = await response.text();
      let errMsg = `HTTP ${status}`;
      try {
        const errJson = JSON.parse(errText);
        errMsg = errJson.error?.message || errJson.message || errMsg;
      } catch {
        const titleMatch = errText.match(/<title>(.*?)<\/title>/i);
        if (titleMatch) errMsg = `(HTML) ${titleMatch[1].substring(0, 100)}`;
      }
      log(`[CHECK] 失败: ${baseUrl} | ${model} | ${errMsg} | ${duration}s`);
      return { status, body: { error: { message: errMsg } } };
    }

    // ── 2xx + JSON (某些服务端即使 stream:true 仍返回完整 JSON) ──
    if (contentType.includes('application/json')) {
      const resText = await response.text();
      let data;
      try { data = JSON.parse(resText); } catch { data = { error: { message: 'Invalid JSON' } }; }
      if (data.choices) {
        log(`[CHECK] 成功(非流式回退): ${baseUrl} | ${model} | ${duration}s`);
        return { status: 200, body: { model: data.model || model, choices: data.choices, usage: data.usage, message: 'success' } };
      }
      const errMsg = data.error?.message || data.message || 'Unknown error';
      log(`[CHECK] 失败: ${baseUrl} | ${model} | ${errMsg} | ${duration}s`);
      return { status: 400, body: { error: { message: errMsg } } };
    }

    // ── 核心：逐行读取 SSE 流，组装结果 ──
    const resText = await response.text();
    const lines = resText.split('\n');

    let returnedModel = '';
    let content = '';
    let reasoningContent = '';
    let usage = null;
    let chunkCount = 0;

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || !trimmed.startsWith('data:')) continue;
      const jsonStr = trimmed.slice(5).trim();
      if (jsonStr === '[DONE]') break;
      try {
        const chunk = JSON.parse(jsonStr);
        chunkCount++;
        if (chunk.model && !returnedModel) returnedModel = chunk.model;
        if (chunk.usage) usage = chunk.usage;
        const delta = chunk.choices?.[0]?.delta;
        if (delta) {
          if (delta.content) content += delta.content;
          if (delta.reasoning_content) reasoningContent += delta.reasoning_content;
          if (delta.thinking) reasoningContent += delta.thinking;
        }
      } catch { /* 跳过无法解析的行 */ }
    }

    if (chunkCount > 0) {
      const isThinking = reasoningContent.length > 0;
      log(`[CHECK] 成功: ${baseUrl} | ${model} | ${duration}s | ${chunkCount} chunks${isThinking ? ' (thinking)' : ''}`);
      return {
        status: 200,
        body: {
          model: returnedModel || model,
          choices: [{ message: { role: 'assistant', content: content || null, reasoning_content: reasoningContent || undefined } }],
          usage,
          isStreamAssembled: true,
          message: 'success',
        },
      };
    } else {
      log(`[CHECK] 失败: ${baseUrl} | ${model} | ${duration}s | 流式响应无有效数据`);
      return { status: 502, body: { error: { message: '流式响应无有效数据 (0 chunks)' } } };
    }
  } catch (err) {
    log(`[CHECK] 异常: ${baseUrl} | ${model} | ${err.message}`);
    if (err.name === 'AbortError') {
      return { status: 504, body: { error: { message: '请求超时 (55s)' } } };
    }
    return { status: 500, body: { error: { message: err.message } } };
  }
}
