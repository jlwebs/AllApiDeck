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
  userAgentMappings.resolveMappedHeadersForModel('claude-3-7-sonnet', userAgentMappings.DEFAULT_USER_AGENT_MAPPINGS),
  {
    match: 'claude',
    headers: {
      'User-Agent': 'claude-cli/2.1.129 (external, cli)',
      'X-App': 'cli',
    },
  }
);

console.log('PASS tests/userAgentMappings.test.mjs');
