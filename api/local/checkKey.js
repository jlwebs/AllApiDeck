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
  return `Request timed out (${Math.round(timeoutMs / 1000)}s)`;
}

function normalizeUrlInput(rawUrl) {
  return String(rawUrl || '').trim().replace(/\/+$/, '');
}

function stripKnownEndpointSuffix(input) {
  const patterns = [
    /\/v\d+\/chat\/completions$/i,
    /\/chat\/completions$/i,
    /\/api\/user\/models$/i,
    /\/api\/models$/i,
    /\/api\/v\d+\/models$/i,
    /\/v\d+\/models$/i,
    /\/models$/i,
    /\/api\/v\d+$/i,
    /\/v\d+$/i,
    /\/api$/i,
  ];

  for (const pattern of patterns) {
    if (pattern.test(input)) {
      return input.replace(pattern, '');
    }
  }

  return input;
}

function buildChatEndpointCandidates(rawUrl) {
  const input = normalizeUrlInput(rawUrl);
  if (!input) {
    return [];
  }

  const candidates = [];
  const addCandidate = candidate => {
    const normalizedCandidate = normalizeUrlInput(candidate);
    if (!normalizedCandidate) return;
    if (!candidates.includes(normalizedCandidate)) {
      candidates.push(normalizedCandidate);
    }
  };

  if (/\/chat\/completions$/i.test(input)) {
    addCandidate(input);
  }

  const bases = new Set([input]);
  const strippedInput = stripKnownEndpointSuffix(input);
  if (strippedInput && strippedInput !== input) {
    bases.add(strippedInput);
  }

  bases.forEach(base => {
    if (!base) return;

    if (/\/chat\/completions$/i.test(base)) {
      addCandidate(base);
      return;
    }

    if (/\/api\/v\d+$/i.test(base) || /\/v\d+$/i.test(base)) {
      addCandidate(`${base}/chat/completions`);
      return;
    }

    if (/\/api$/i.test(base)) {
      addCandidate(`${base}/v1/chat/completions`);
      addCandidate(`${base}/chat/completions`);
      return;
    }

    addCandidate(`${base}/v1/chat/completions`);
    addCandidate(`${base}/chat/completions`);
    addCandidate(`${base}/api/v1/chat/completions`);
  });

  return candidates;
}

function isRetryableEndpointStatus(status) {
  return status === 404 || status === 405;
}

function buildEndpointErrorMessage(message, status, endpoint) {
  const fallback = `HTTP ${status}`;
  const baseMessage = String(message || '').trim() || fallback;
  if (!endpoint) return baseMessage;
  if (baseMessage.includes(endpoint)) return baseMessage;
  return `${baseMessage} @ ${endpoint}`;
}

function buildAttemptRecord({ endpoint, status, message, retryable }) {
  return {
    endpoint: String(endpoint || ''),
    status: Number(status || 0),
    message: String(message || '').trim(),
    retryable: retryable === true,
  };
}

function extractResponseErrorMessage(text, status) {
  let message = `HTTP ${status}`;

  try {
    const errJson = JSON.parse(text);
    return errJson?.error?.message || errJson?.message || message;
  } catch {}

  const titleMatch = String(text || '').match(/<title>(.*?)<\/title>/i);
  if (titleMatch) {
    message = `(HTML) ${titleMatch[1].substring(0, 100)}`;
  }

  return message;
}

async function requestChatCompletion({ endpoint, key, model, messages, uid, timeoutMs }, log = console.log) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  const startTime = Date.now();

  log(
    `[CHECK] trying endpoint=${endpoint} | model=${model} | key=${String(key || '').slice(0, 12)}... | timeout=${timeoutMs}ms`,
  );

  try {
    const response = await fetch(endpoint, {
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

    const duration = ((Date.now() - startTime) / 1000).toFixed(2);
    const status = response.status;
    const contentType = response.headers.get('content-type') || '';

    if (!response.ok) {
      const errText = await response.text();
      const errMsg = buildEndpointErrorMessage(extractResponseErrorMessage(errText, status), status, endpoint);
      log(`[CHECK] failed: endpoint=${endpoint} | ${model} | ${errMsg} | ${duration}s`);
      return {
        ok: false,
        attempt: buildAttemptRecord({
          endpoint,
          status,
          message: errMsg,
          retryable: isRetryableEndpointStatus(status),
        }),
        retryable: isRetryableEndpointStatus(status),
        status,
        message: errMsg,
      };
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
        log(`[CHECK] success(JSON): endpoint=${endpoint} | ${model} | ${duration}s`);
        return {
          ok: true,
          endpoint,
          result: {
            status: 200,
            body: {
              model: data.model || model,
              choices: data.choices,
              usage: data.usage,
              message: 'success',
            },
          },
        };
      }

      const errMsg = buildEndpointErrorMessage(data?.error?.message || data?.message || 'Unknown error', 400, endpoint);
      log(`[CHECK] failed: endpoint=${endpoint} | ${model} | ${errMsg} | ${duration}s`);
      return {
        ok: false,
        attempt: buildAttemptRecord({
          endpoint,
          status: 400,
          message: errMsg,
          retryable: false,
        }),
        retryable: false,
        status: 400,
        message: errMsg,
      };
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
      log(`[CHECK] success(SSE): endpoint=${endpoint} | ${model} | ${duration}s | chunks=${chunkCount}${isThinking ? ' thinking' : ''}`);
      return {
        ok: true,
        endpoint,
        result: {
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
        },
      };
    }

    const streamError = buildEndpointErrorMessage('Stream response did not contain valid chunks (0 chunks)', 502, endpoint);
    log(`[CHECK] failed: endpoint=${endpoint} | ${model} | ${streamError} | ${duration}s`);
    return {
      ok: false,
      attempt: buildAttemptRecord({
        endpoint,
        status: 502,
        message: streamError,
        retryable: false,
      }),
      retryable: false,
      status: 502,
      message: streamError,
    };
  } catch (err) {
    const status = err?.name === 'AbortError' ? 504 : 500;
    const errMsg = buildEndpointErrorMessage(
      err?.name === 'AbortError' ? createAbortErrorMessage(timeoutMs) : err?.message,
      status,
      endpoint,
    );
    log(`[CHECK] exception: endpoint=${endpoint} | ${model} | ${errMsg}`);
    return {
      ok: false,
      attempt: buildAttemptRecord({
        endpoint,
        status,
        message: errMsg,
        retryable: false,
      }),
      retryable: false,
      status,
      message: errMsg,
    };
  } finally {
    clearTimeout(timer);
  }
}

export async function checkKey({ url, key, model, messages, uid, timeoutMs }, log = console.log) {
  const normalizedTimeoutMs = clampTimeoutMs(timeoutMs);
  const inputUrl = normalizeUrlInput(url);
  const endpoints = buildChatEndpointCandidates(inputUrl);
  const attempts = [];

  if (!inputUrl || endpoints.length === 0) {
    return {
      status: 400,
      body: { error: { message: 'API URL is empty or invalid' } },
    };
  }

  log(
    `[CHECK] start ${inputUrl} | model=${model} | endpoints=${endpoints.length} | timeout=${normalizedTimeoutMs}ms`,
  );

  let lastFailure = null;
  for (const endpoint of endpoints) {
    const result = await requestChatCompletion({
      endpoint,
      key,
      model,
      messages,
      uid,
      timeoutMs: normalizedTimeoutMs,
    }, log);

    if (result.ok) {
      result.result.body.diagnostics = {
        inputUrl,
        model: String(model || '').trim(),
        timeoutMs: normalizedTimeoutMs,
        resolvedEndpoint: result.endpoint,
        attempts,
      };
      return result.result;
    }

    lastFailure = result;
    if (result.attempt) {
      attempts.push(result.attempt);
    }
    if (!result.retryable) {
      return {
        status: result.status,
        body: {
          error: {
            message: result.message,
            diagnostics: {
              inputUrl,
              model: String(model || '').trim(),
              timeoutMs: normalizedTimeoutMs,
              attempts,
            },
          },
        },
      };
    }
  }

  const fallbackMessage = lastFailure?.message || 'No compatible chat completion endpoint found';
  return {
    status: lastFailure?.status || 404,
    body: {
      error: {
        message: fallbackMessage,
        diagnostics: {
          inputUrl,
          model: String(model || '').trim(),
          timeoutMs: normalizedTimeoutMs,
          attempts,
        },
      },
    },
  };
}
