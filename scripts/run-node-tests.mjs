import { readdir } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';
import { pathToFileURL } from 'node:url';

async function collectTestFiles(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...await collectTestFiles(fullPath));
      continue;
    }

    if (entry.isFile() && /\.test\.mjs$/i.test(entry.name)) {
      files.push(fullPath);
    }
  }

  return files;
}

async function main() {
  const testRoot = path.resolve(process.cwd(), 'tests');
  let files = [];

  try {
    files = await collectTestFiles(testRoot);
  } catch {
    files = [];
  }

  files.sort((left, right) => left.localeCompare(right));

  if (!files.length) {
    console.log('No test files found.');
    return;
  }

  for (const file of files) {
    try {
      console.log(`Running ${path.relative(process.cwd(), file)}`);
      await import(`${pathToFileURL(file).href}?t=${Date.now()}-${Math.random()}`);
    } catch (error) {
      console.error(`Test failed: ${path.relative(process.cwd(), file)}`);
      console.error(error);
      process.exitCode = 1;
      return;
    }
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
