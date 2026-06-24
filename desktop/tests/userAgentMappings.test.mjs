import assert from 'node:assert/strict';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const modulePath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'utils', 'userAgentMappings.js');
const moduleUrl = pathToFileURL(modulePath).href;
const userAgentMappings = await import(`${moduleUrl}?t=${Date.now()}-${Math.random()}`);

assert.deepStrictEqual(
  userAgentMappings.DEFAULT_USER_AGENT_MAPPINGS.map(item => item.modelContains),
  ['gpt', 'claude']
);

assert.deepStrictEqual(
  userAgentMappings.parseMappedUserAgentHeaders('User-Agent: claude-cli/2.1.129 (external, cli); x-app: cli'),
  {
    'User-Agent': 'claude-cli/2.1.129 (external, cli)',
    'X-App': 'cli',
  }
);

assert.deepStrictEqual(
  userAgentMappings.parseMappedUserAgentHeaders(userAgentMappings.DEFAULT_CLAUDE_TARGET_UA),
  {
    'User-Agent': 'claude-cli/2.1.129 (external, cli)',
    'X-App': 'cli',
    'Anthropic-Version': '2023-06-01',
    'Anthropic-Beta': 'claude-code-20250219,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,context-management-2025-06-27,prompt-caching-scope-2026-01-05,effort-2025-11-24',
    'Anthropic-Dangerous-Direct-Browser-Access': 'true',
    'X-Stainless-Arch': 'x64',
    'X-Stainless-Lang': 'js',
    'X-Stainless-Os': 'Windows',
    'X-Stainless-Package-Version': '0.93.0',
    'X-Stainless-Retry-Count': '0',
    'X-Stainless-Runtime': 'node',
    'X-Stainless-Runtime-Version': 'v24.3.0',
    'X-Stainless-Timeout': '600',
  }
);

assert.deepStrictEqual(
  userAgentMappings.resolveMappedHeadersForModel('claude-3-7-sonnet', userAgentMappings.DEFAULT_USER_AGENT_MAPPINGS),
  {
    match: 'claude',
    headers: {
      'User-Agent': 'claude-cli/2.1.129 (external, cli)',
      'X-App': 'cli',
      'Anthropic-Version': '2023-06-01',
      'Anthropic-Beta': 'claude-code-20250219,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,context-management-2025-06-27,prompt-caching-scope-2026-01-05,effort-2025-11-24',
      'Anthropic-Dangerous-Direct-Browser-Access': 'true',
      'X-Stainless-Arch': 'x64',
      'X-Stainless-Lang': 'js',
      'X-Stainless-Os': 'Windows',
      'X-Stainless-Package-Version': '0.93.0',
      'X-Stainless-Retry-Count': '0',
      'X-Stainless-Runtime': 'node',
      'X-Stainless-Runtime-Version': 'v24.3.0',
      'X-Stainless-Timeout': '600',
    },
  }
);

assert.deepStrictEqual(
  userAgentMappings.normalizeUserAgentMappings(
    [{ modelContains: 'claude', targetUA: userAgentMappings.LEGACY_DEFAULT_CLAUDE_TARGET_UA }],
    { fallbackToDefault: false }
  ),
  [{ modelContains: 'claude', targetUA: userAgentMappings.DEFAULT_CLAUDE_TARGET_UA }]
);

console.log('PASS tests/userAgentMappings.test.mjs');
