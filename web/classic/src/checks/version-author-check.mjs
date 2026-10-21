import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const srcRoot = resolve(__dirname, '..');

const otherSetting = readFileSync(
  resolve(srcRoot, 'components/settings/OtherSetting.jsx'),
  'utf8',
);

assert.doesNotMatch(
  otherSetting,
  /鏈煡/,
  'classic version author fallback must not contain mojibake text',
);
assert.match(
  otherSetting,
  /version_author\s*\|\|\s*t\('未知'\)/,
  'classic version author fallback should use the translated Unknown text',
);

console.log('classic version author checks passed');
