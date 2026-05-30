import fs from 'node:fs/promises';
import path from 'node:path';

const TARGET_URL = process.env.GUARD_PROBE_URL || 'https://cpa.zuiniu.de/v1/responses';
const API_TOKEN = (process.env.GUARD_PROBE_TOKEN || '').trim();
const ATTEMPTS = Math.max(1, Number.parseInt(process.env.GUARD_PROBE_ATTEMPTS || '2', 10) || 2);
const MAX_HTTP_RETRIES = Math.max(0, Number.parseInt(process.env.GUARD_PROBE_HTTP_RETRIES || '2', 10) || 2);
const OUT_DIR = process.env.GUARD_PROBE_OUT_DIR || path.join(process.cwd(), 'build', 'bin', 'guard-prompt-probe');

const ALIAS = 'APTX_GUARD_MIN_01';
const PREFIX = 'aad_guard_probe71';
const NONCE = '7f3c2190d4ab6621';

if (!API_TOKEN) {
  console.error('Missing GUARD_PROBE_TOKEN');
  process.exit(1);
}

const webSearchTool = {
  type: 'function',
  name: 'WebSearch',
  description: [
    'Search the web and return up-to-date information.',
    'Use this tool for current events and recent data.',
    'After the answer, include a Sources section.',
  ].join(' '),
  parameters: {
    type: 'object',
    additionalProperties: false,
    properties: {
      allowed_domains: { type: 'array', items: { type: 'string' } },
      blocked_domains: { type: 'array', items: { type: 'string' } },
      query: { type: 'string', minLength: 2 },
    },
    required: ['query'],
  },
};

const baseInstructions = [
  'You are an interactive coding agent.',
  'For requests about current news, use the available WebSearch tool immediately.',
  'Do not ask clarifying questions for broad headline requests.',
  'Do not explain your plan before acting.',
].join('\n');

const baseInput = [
  {
    role: 'system',
    content: [
      {
        type: 'input_text',
        text: '<system-reminder>\nToday\'s date is 2026/05/27.\n</system-reminder>',
      },
      {
        type: 'input_text',
        text: '<system-reminder>\nAvailable skills exist, but do not matter unless explicitly invoked.\n</system-reminder>',
      },
    ],
  },
  {
    role: 'user',
    content: [
      { type: 'input_text', text: '请立即联网搜索 2026-05-27 今日热点，覆盖世界、中国、科技，不要先问问题，直接调用工具。' },
    ],
  },
];

const variants = [
  {
    name: 'system_min_guard_name',
    placement: 'input_system',
    prompt: [
      '<important_gateway_rules>',
      'IMPORTANT: highest priority.',
      'You must use WebSearch now.',
      'Before the WebSearch toolcall, emit exactly one <aad_guard_json>...</aad_guard_json> block for that next toolcall.',
      `Inside that JSON, name must be exactly ${PREFIX}_WebSearch.`,
      'tool_name must be exactly WebSearch.',
      `algorithm must be exactly ${ALIAS}.`,
      `nonce must be exactly ${NONCE}.`,
      'Include these JSON fields: name, tool_name, tool_type, algorithm, nonce, digest, chain, cover.',
      'digest must be 16 lowercase hex characters.',
      'Never emit the real toolcall before its guard block.',
      'If you cannot do this correctly, emit no toolcall and output plain text: guard generation failed for pending toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
  {
    name: 'system_min_guard_name_ultra_short',
    placement: 'input_system',
    prompt: [
      '<important_gateway_rules>',
      'Use WebSearch now.',
      `Immediately before that WebSearch toolcall, emit exactly one <aad_guard_json>...</aad_guard_json> block with name=${PREFIX}_WebSearch and tool_name=WebSearch.`,
      `Set algorithm=${ALIAS} and nonce=${NONCE}.`,
      'If you cannot do that first, emit no toolcall and output guard generation failed for pending toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
  {
    name: 'system_min_guard_name_with_shape',
    placement: 'input_system',
    prompt: [
      '<important_gateway_rules>',
      'IMPORTANT: highest priority.',
      'You must use WebSearch now.',
      'Before that WebSearch toolcall, emit one valid <aad_guard_json>...</aad_guard_json> block.',
      `name must be exactly ${PREFIX}_WebSearch.`,
      'tool_name must be exactly WebSearch.',
      'A malformed or approximate guard counts as no guard.',
      'Shape example only:',
      `<aad_guard_json>{"name":"${PREFIX}_WebSearch","tool_name":"WebSearch","tool_type":"network","algorithm":"${ALIAS}","nonce":"${NONCE}","digest":"1a2b3c4d5e6f7890","chain":"0|network|WebSearch","cover":"{\\"query\\":\\"...\\"}"}</aad_guard_json>`,
      'If you cannot do this, do not emit any real toolcall; output plain text: guard generation failed for pending toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
  {
    name: 'combined_min_guard_name_with_shape',
    placement: 'both',
    prompt: [
      '<important_gateway_rules>',
      'IMPORTANT: highest priority.',
      'You must use WebSearch now.',
      'Emit one guard block immediately before that WebSearch toolcall.',
      `name must be exactly ${PREFIX}_WebSearch and tool_name must be exactly WebSearch.`,
      `algorithm must be ${ALIAS} and nonce must be ${NONCE}.`,
      'If the guard is not valid, do not emit any toolcall.',
      `<aad_guard_json>{"name":"${PREFIX}_WebSearch","tool_name":"WebSearch","tool_type":"network","algorithm":"${ALIAS}","nonce":"${NONCE}","digest":"1a2b3c4d5e6f7890","chain":"0|network|WebSearch","cover":"{\\"query\\":\\"...\\"}"}</aad_guard_json>`,
      'If you cannot satisfy this, output plain text: guard generation failed for pending toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
  {
    name: 'system_fail_closed_short',
    placement: 'input_system',
    prompt: [
      '<important_gateway_rules>',
      'You must use WebSearch now.',
      'Before the WebSearch toolcall, emit one valid <aad_guard_json>...</aad_guard_json> block.',
      `name must be ${PREFIX}_WebSearch and tool_name must be WebSearch.`,
      'If this is not possible, emit no real toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
  {
    name: 'system_one_guard_one_toolcall',
    placement: 'input_system',
    prompt: [
      '<important_gateway_rules>',
      'Use exactly one real WebSearch toolcall in this turn.',
      'Immediately before that one WebSearch toolcall, emit exactly one <aad_guard_json>...</aad_guard_json> block.',
      `Inside that JSON, name must be exactly ${PREFIX}_WebSearch and tool_name must be exactly WebSearch.`,
      'Do not emit a second toolcall in this turn.',
      'If you cannot satisfy this exactly, emit no toolcall and output guard generation failed for pending toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
  {
    name: 'system_core_fields_no_digest',
    placement: 'input_system',
    prompt: [
      '<important_gateway_rules>',
      'Use exactly one real WebSearch toolcall in this turn.',
      'Immediately before that one WebSearch toolcall, emit exactly one <aad_guard_json>...</aad_guard_json> block.',
      `Inside that JSON, name must be exactly ${PREFIX}_WebSearch.`,
      'tool_name must be exactly WebSearch.',
      'tool_type must be exactly network.',
      `algorithm must be exactly ${ALIAS}.`,
      `nonce must be exactly ${NONCE}.`,
      'Do not emit a second toolcall in this turn.',
      'If you cannot satisfy this exactly, emit no toolcall and output guard generation failed for pending toolcall.',
      '</important_gateway_rules>',
    ].join('\n'),
  },
];

await fs.mkdir(OUT_DIR, { recursive: true });

const summaries = [];
for (const variant of variants) {
  for (let attempt = 1; attempt <= ATTEMPTS; attempt += 1) {
    const started = Date.now();
    const requestBody = buildRequest(variant);
    const raw = await runOne(requestBody);
    const analysis = analyzeSSE(raw);
    const summary = {
      variant: variant.name,
      attempt,
      durationMs: Date.now() - started,
      guardBeforeTool: analysis.guardBeforeTool,
      firstGuardEvent: analysis.firstGuardEvent,
      firstToolEvent: analysis.firstToolEvent,
      guardCount: analysis.guardCount,
      canonicalGuardCount: analysis.canonicalGuardCount,
      firstToolType: analysis.firstToolType,
      firstToolName: analysis.firstToolName,
      guardName: analysis.guardName,
      guardToolName: analysis.guardToolName,
      guardNameMatchesNextTool: analysis.guardNameMatchesNextTool,
      guardToolNameMatches: analysis.guardToolNameMatches,
      malformedGuardLikeCount: analysis.malformedGuardLikeCount,
      failClosedText: analysis.failClosedText,
    };
    summaries.push(summary);

    const base = `${String(summaries.length).padStart(2, '0')}_${variant.name}_a${attempt}`;
    await fs.writeFile(path.join(OUT_DIR, `${base}_request.json`), JSON.stringify(requestBody, null, 2));
    await fs.writeFile(path.join(OUT_DIR, `${base}_response.sse`), raw);
    await fs.writeFile(path.join(OUT_DIR, `${base}_summary.json`), JSON.stringify(summary, null, 2));

    console.log(JSON.stringify(summary));
  }
}

await fs.writeFile(path.join(OUT_DIR, 'summary.json'), JSON.stringify(summaries, null, 2));

function buildRequest(variant) {
  const body = {
    model: 'gpt-5.4',
    stream: true,
    max_output_tokens: 4096,
    instructions: baseInstructions,
    input: structuredClone(baseInput),
    tools: [webSearchTool],
    temperature: 0,
    parallel_tool_calls: false,
  };

  if (variant.placement === 'instructions_prepend') {
    body.instructions = `${variant.prompt}\n\n${body.instructions}`;
  } else if (variant.placement === 'input_system') {
    body.input = [
      {
        role: 'system',
        content: [{ type: 'input_text', text: variant.prompt }],
      },
      ...body.input,
    ];
  } else if (variant.placement === 'both') {
    body.instructions = `${variant.prompt}\n\n${body.instructions}`;
    body.input = [
      {
        role: 'system',
        content: [{ type: 'input_text', text: variant.prompt }],
      },
      ...body.input,
    ];
  }

  return body;
}

async function runOne(body) {
  for (let attempt = 0; attempt <= MAX_HTTP_RETRIES; attempt += 1) {
    const res = await fetch(TARGET_URL, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        accept: 'text/event-stream',
        authorization: `Bearer ${API_TOKEN}`,
      },
      body: JSON.stringify(body),
    });

    const raw = await res.text();
    if (res.ok) {
      return raw;
    }
    if (res.status === 524 && attempt < MAX_HTTP_RETRIES) {
      continue;
    }
    throw new Error(`HTTP ${res.status}: ${raw.slice(0, 800)}`);
  }
  throw new Error('unreachable');
}

function analyzeSSE(raw) {
  const events = parseSSE(raw);
  let firstGuardEvent = null;
  let firstToolEvent = null;
  let firstToolType = '';
  let firstToolName = '';
  let guardName = '';
  let guardToolName = '';
  let canonicalGuardCount = 0;

  for (let i = 0; i < events.length; i += 1) {
    const seq = i + 1;
    const event = events[i];
    if (firstGuardEvent == null && event.outputText.includes('<aad_guard_json>')) {
      firstGuardEvent = seq;
      const match = event.outputText.match(/<aad_guard_json>\s*(\{[\s\S]*?\})\s*<\/aad_guard_json>/i);
      if (match) {
        canonicalGuardCount += 1;
        try {
          const payload = JSON.parse(match[1]);
          guardName = String(payload.name || '');
          guardToolName = String(payload.tool_name || '');
        } catch {}
      }
    } else if (event.outputText.includes('<aad_guard_json>')) {
      canonicalGuardCount += 1;
    }

    if (firstToolEvent == null && event.tool) {
      firstToolEvent = seq;
      firstToolType = event.tool.type;
      firstToolName = event.tool.name;
    }
  }

  const outputJoined = events.map(event => event.outputText).filter(Boolean).join('\n');
  const malformedGuardLikeCount = countMalformedGuardLike(outputJoined);
  const guardCount = (outputJoined.match(/<aad_guard_json>/g) || []).length;
  const guardNameMatchesNextTool = Boolean(firstToolName && guardName === `${PREFIX}_${firstToolName}`);
  const guardToolNameMatches = Boolean(firstToolName && guardToolName === firstToolName);
  const failClosedText = outputJoined.toLowerCase().includes('guard generation failed for pending toolcall');

  return {
    guardBeforeTool: firstGuardEvent != null && firstToolEvent != null && firstGuardEvent < firstToolEvent,
    firstGuardEvent,
    firstToolEvent,
    guardCount,
    canonicalGuardCount,
    firstToolType,
    firstToolName,
    guardName,
    guardToolName,
    guardNameMatchesNextTool,
    guardToolNameMatches,
    malformedGuardLikeCount,
    failClosedText,
  };
}

function parseSSE(raw) {
  const blocks = raw.split(/\r?\n\r?\n/).filter(Boolean);
  const events = [];
  for (const block of blocks) {
    const lines = block.split(/\r?\n/);
    let eventName = '';
    const dataLines = [];
    for (const line of lines) {
      if (line.startsWith('event:')) eventName = line.slice(6).trim();
      if (line.startsWith('data:')) dataLines.push(line.slice(5).trim());
    }
    const joined = dataLines.join('\n');
    let outputText = '';
    let tool = null;
    try {
      const payload = JSON.parse(joined);
      tool = extractTool(payload);
      const maybeText = extractText(payload);
      if (maybeText) outputText = maybeText;
    } catch {}
    events.push({ eventName, outputText, tool });
  }
  return events;
}

function extractTool(payload) {
  const item = payload?.item;
  if (item?.type === 'function_call') {
    return { type: 'function_call', name: String(item.name || '') };
  }
  if (item?.type === 'web_search_call') {
    return { type: 'web_search_call', name: 'web_search_call' };
  }
  const responseOutput = payload?.response?.output;
  if (Array.isArray(responseOutput)) {
    for (const outputItem of responseOutput) {
      if (outputItem?.type === 'function_call') return { type: 'function_call', name: String(outputItem.name || '') };
      if (outputItem?.type === 'web_search_call') return { type: 'web_search_call', name: 'web_search_call' };
    }
  }
  return null;
}

function extractText(payload) {
  if (typeof payload?.delta === 'string') return payload.delta;
  if (typeof payload?.text === 'string') return payload.text;
  const item = payload?.item;
  if (item?.type === 'message' && Array.isArray(item.content)) {
    return item.content.map(part => part?.text || part?.content || '').join('\n');
  }
  return '';
}

function countMalformedGuardLike(raw) {
  const matches = raw.match(/<\s*a\s*a?\s*d[\s_]*g[\s_]*u[\s_]*a[\s_]*r[\s_]*d[\s_]*j[\s_]*s[\s_]*o[\s_]*n/gi) || [];
  return matches.length;
}
