import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', 'src');
const globalCss = await readFile(path.join(root, 'styles', 'global.css'), 'utf8');
const routerSource = await readFile(path.join(root, 'router', 'index.js'), 'utf8');

for (const variable of [
  '--surface-1',
  '--surface-2',
  '--text-primary',
  '--text-secondary',
  '--text-muted',
  '--control-bg',
  '--control-border',
]) {
  assert.match(globalCss, new RegExp(`body\\.gaia-dark[\\s\\S]*${variable}`), `missing Gaia-dark variable: ${variable}`);
}

for (const selector of [
  'body.gaia-dark .ant-input',
  'body.gaia-dark .ant-select-selector',
  'body.gaia-dark .ant-modal-content',
  'body.gaia-dark .editor-shell',
  'body.gaia-dark .ai-image-shell',
  'body.gaia-dark .desktop-config-shell',
]) {
  assert.ok(globalCss.includes(selector), `missing dark override: ${selector}`);
}

assert.match(routerSource, /const loadHomeView = \(\) => import\('\.\.\/views\/Home\.vue'\)/);
assert.match(routerSource, /const loadBatchView = \(\) => import\('\.\.\/views\/Batch\.vue'\)/);
assert.ok(!routerSource.includes("import Home from '../views/Home.vue'"));
assert.ok(!routerSource.includes("import Batch from '../views/Batch.vue'"));
assert.match(routerSource, /connection\?\.saveData/);
assert.match(routerSource, /deviceMemory/);
assert.match(routerSource, /scheduleDomTranslation\(document\.querySelector\('\.app-shell'\)/);

console.log('PASS tests/theme-performance-regression.test.mjs');
