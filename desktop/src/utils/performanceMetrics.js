function toFiniteNumber(value) {
  if (value == null || value === '') return null;
  if (typeof value === 'number') return Number.isFinite(value) ? value : null;
  const normalized = String(value).trim();
  if (!normalized) return null;
  const match = normalized.match(/-?\d+(?:\.\d+)?/);
  if (!match) return null;
  const parsed = Number(match[0]);
  return Number.isFinite(parsed) ? parsed : null;
}

function firstFiniteNumber(candidates) {
  for (const candidate of candidates) {
    const parsed = toFiniteNumber(candidate);
    if (parsed != null) return parsed;
  }
  return null;
}

function readUsageCompletionTokens(source) {
  const usage = source?.usage && typeof source.usage === 'object' ? source.usage : null;
  return firstFiniteNumber([
    source?.quickTestCompletionTokens,
    source?.completionTokens,
    source?.performance?.completionTokens,
    usage?.completion_tokens,
    usage?.output_tokens,
    usage?.completionTokens,
    usage?.outputTokens,
  ]);
}

export function extractPerformanceMetrics(source = {}) {
  const latencySeconds = firstFiniteNumber([
    source?.quickTestLatencySeconds,
    source?.latencySeconds,
    source?.performance?.latencySeconds,
    source?.quickTestResponseTime,
    source?.responseTime,
    source?.latency,
  ]);

  const ttftMs = firstFiniteNumber([
    source?.quickTestTtftMs,
    source?.ttftMs,
    source?.performance?.ttftMs,
    source?.ttft,
  ]);

  const completionTokens = readUsageCompletionTokens(source);
  let tps = firstFiniteNumber([
    source?.quickTestTps,
    source?.tps,
    source?.performance?.tps,
  ]);

  if (tps == null && latencySeconds && latencySeconds > 0 && completionTokens && completionTokens > 0) {
    tps = completionTokens / latencySeconds;
  }

  return {
    ttftMs,
    tps,
    latencySeconds,
    completionTokens,
  };
}

export function hasPerformanceMetrics(source = {}) {
  const metrics = extractPerformanceMetrics(source);
  return metrics.ttftMs != null || metrics.tps != null || metrics.latencySeconds != null;
}

export function buildPerformanceTooltipLines(source = {}) {
  const metrics = extractPerformanceMetrics(source);
  const latencyText = metrics.latencySeconds != null ? `${metrics.latencySeconds.toFixed(2)}s` : '-';
  const ttftText = metrics.ttftMs != null ? `${Math.round(metrics.ttftMs)}ms` : '-';
  const tpsText = metrics.tps != null ? `${metrics.tps.toFixed(2)} tok/s` : '-';
  return [
    `TTFT: ${ttftText}`,
    `TPS: ${tpsText}`,
    `Latency: ${latencyText}`,
  ];
}

export function derivePerformanceMetricsFromResponse(payload = {}, latencySeconds = null) {
  const metrics = extractPerformanceMetrics({
    ...payload,
    responseTime: latencySeconds,
  });
  return {
    ttftMs: metrics.ttftMs != null ? String(Math.round(metrics.ttftMs)) : '',
    tps: metrics.tps != null ? metrics.tps.toFixed(2) : '',
    latencySeconds: metrics.latencySeconds != null ? metrics.latencySeconds.toFixed(2) : '',
  };
}
