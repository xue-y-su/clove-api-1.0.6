import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const srcRoot = resolve(__dirname, '..');
const accountManagement = readFileSync(
  resolve(srcRoot, 'components/settings/personal/cards/AccountManagement.jsx'),
  'utf8',
);

assert.match(
  accountManagement,
  /customOAuthItems|customOAuthProviderItems/,
  'classic 个人资料页必须合并启用 provider 和已绑定 provider',
);
assert.match(
  accountManagement,
  /seenProviderIds|bindingMap/,
  'classic 个人资料页必须把禁用但已绑定的自定义 OAuth provider 补入列表',
);
assert.doesNotMatch(
  accountManagement,
  /status\.custom_oauth_providers\s*&&\s*status\.custom_oauth_providers\.map/,
  'classic 个人资料页不能只遍历启用 provider，否则禁用后无法解绑',
);

console.log('classic account binding checks passed');
