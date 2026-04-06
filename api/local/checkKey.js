// Shared API key check logic for both Vite middleware and local proxy routes.

function clampTimeoutMs(value, fallback = 55000) {
  if (!Number.isFinite(Number(value))) {
    return fallback;
  }
  return Math.max(5000, Math.min(180000, Number(value)));
}

function buildCompatHeaders(uid) {
  const normalizedUid = String(uid || '').trim();
  if (!/^\d+$/.test(normalizedUid)) {
    return {};
  }

  return {
    'New-Api-User': normalizedUid,
    'Veloera-User': normalizedUid,
    'voapi-user': normalizedUid,
    'User-id': normalizedUid,
    'Rix-Api-User': normalizedUid,
    'neo-api-user': normalizedUid,
  };
}

function createAbortErrorMessage(timeoutMs) {
  return `请求超时 (${Math.round(timeoutMs / 1000)}s)`;
}

export async function checkKey({ url, key, model, messages, uid, timeoutMs }, log = console.log) {
  const baseUrl = String(url || '').replace(/\/+$/, '');
  const targetUrl = `${baseUrl}/v1/chat/completions`;
  const normalizedTimeoutMs = clampTimeoutMs(timeoutMs);

  log(
    `[CHECK] 开始: ${baseUrl} | model=${model} | key=${String(key || '').slice(0, 12)}... | timeout=${normalizedTimeoutMs}ms`,
  );

  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), normalizedTimeoutMs);
    const startTime = Date.now();

    const response = await fetch(targetUrl, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${key}`,
        'Content-Type': 'application/json',
        'User-Agent': 'Mozilla/5.0 ApiChecker/1.0',
        ...buildCompatHeaders(uid),
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

    if (!response.ok) {
      const errText = await response.text();
      let errMsg = `HTTP ${status}`;
      try {
        const errJson = JSON.parse(errText);
        errMsg = errJson.error?.message || errJson.message || errMsg;
      } catch {
        const titleMatch = errText.match(/<title>(.*?)<\/title>/i);
        if (titleMatch) {
          errMsg = `(HTML) ${titleMatch[1].substring(0, 100)}`;
        }
      }
      log(`[CHECK] 失败: ${baseUrl} | ${model} | ${errMsg} | ${duration}s`);
      return { status, body: { error: { message: errMsg } } };
    }

    if (contentType.includes('application/json')) {
      const resText = await response.text();
      let data;
      try {
        data = JSON.parse(resText);
      } catch {
        data = { error: { message: 'Invalid JSON' } };
      }

      if (data.choices) {
        log(`[CHECK] 成功(JSON): ${baseUrl} | ${model} | ${duration}s`);
        return {
          status: 200,
          body: {
            model: data.model || model,
            choices: data.choices,
            usage: data.usage,
            message: 'success',
          },
        };
      }

      const errMsg = data.error?.message || data.message || 'Unknown error';
      log(`[CHECK] 失败: ${baseUrl} | ${model} | ${errMsg} | ${duration}s`);
      return { status: 400, body: { error: { message: errMsg } } };
    }

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
        chunkCount += 1;
        if (chunk.model && !returnedModel) returnedModel = chunk.model;
        if (chunk.usage) usage = chunk.usage;

        const delta = chunk.choices?.[0]?.delta;
        if (!delta) continue;
        if (delta.content) content += delta.content;
        if (delta.reasoning_content) reasoningContent += delta.reasoning_content;
        if (delta.thinking) reasoningContent += delta.thinking;
      } catch {
        // Ignore malformed SSE chunks.
      }
    }

    if (chunkCount > 0) {
      const isThinking = reasoningContent.length > 0;
      log(`[CHECK] 成功(SSE): ${baseUrl} | ${model} | ${duration}s | chunks=${chunkCount}${isThinking ? ' thinking' : ''}`);
      return {
        status: 200,
        body: {
          model: returnedModel || model,
          choices: [
            {
              message: {
                role: 'assistant',
                content: content || null,
                reasoning_content: reasoningContent || undefined,
              },
            },
          ],
          usage,
          isStreamAssembled: true,
          message: 'success',
        },
      };
    }

    log(`[CHECK] 失败: ${baseUrl} | ${model} | ${duration}s | no valid SSE chunks`);
    return { status: 502, body: { error: { message: '流式响应无有效数据 (0 chunks)' } } };
  } catch (err) {
    log(`[CHECK] 异常: ${baseUrl} | ${model} | ${err.message}`);
    if (err.name === 'AbortError') {
      return { status: 504, body: { error: { message: createAbortErrorMessage(normalizedTimeoutMs) } } };
    }
    return { status: 500, body: { error: { message: err.message } } };
  }
}
