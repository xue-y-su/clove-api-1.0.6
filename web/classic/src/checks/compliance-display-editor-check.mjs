/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const checkFile = fileURLToPath(import.meta.url);
const repoRoot = path.resolve(path.dirname(checkFile), '../../../..');
const failures = [];

function readText(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

function assert(condition, message) {
  if (!condition) failures.push(message);
}

function assertIncludes(relativePath, needles) {
  const text = readText(relativePath);
  for (const needle of needles) {
    assert(text.includes(needle), `${relativePath} must include ${needle}`);
  }
}

function assertNotIncludes(relativePath, needles) {
  const text = readText(relativePath);
  for (const needle of needles) {
    assert(
      !text.includes(needle),
      `${relativePath} must not include ${needle}`,
    );
  }
}

assertIncludes('controller/compliance.go', [
  'applyComplianceDisplayVisibility',
  'applyComplianceSnapshotDisplayVisibility',
  'ShowVersionToUser',
  'ShowContentHashToUser',
]);

assertIncludes('controller/compliance_test.go', [
  'TestGetComplianceStatementHidesVersionAndHashByDefault',
  'TestHiddenComplianceStatementStillConfirmsWithRealHash',
  'TestGetComplianceSettingsAlwaysReturnsRealVersionAndHash',
  'TestUpdateComplianceSettingsPersistsUserDisplaySwitches',
]);

assertIncludes('web/default/src/features/compliance/api.ts', [
  'show_version_to_user',
  'show_content_hash_to_user',
]);

assertIncludes('web/default/src/features/source-attribution/api.ts', [
  'target_username',
]);

assertNotIncludes('web/default/src/features/source-attribution/api.ts', [
  'target_inviter_name',
]);

assertIncludes(
  'web/default/src/features/system-settings/site/compliance-section.tsx',
  [
    'Show version to users',
    'Show content hash to users',
    'Visual editor',
    'JSON Editor',
    'ComplianceStatementItemsEditor',
  ],
);

assertIncludes(
  'web/default/src/features/system-settings/site/compliance-section.tsx',
  ['parseStatementItemsOrThrow', 'throw new Error', 'form.setError'],
);

assertNotIncludes(
  'web/default/src/features/system-settings/site/compliance-section.tsx',
  [
    'if (!value.statement_enabled) return',
    'return Array.isArray(parsed) ? (parsed as ComplianceStatementItem[]) : []',
  ],
);

assertIncludes(
  'web/default/src/features/system-settings/site/source-attribution-section.tsx',
  ['Visual editor', 'JSON Editor', 'SourceAttributionOptionsEditor'],
);

assertIncludes('web/classic/src/components/settings/ComplianceSetting.jsx', [
  '向用户展示版本号',
  '向用户展示内容哈希',
  '可视化编辑',
  'JSON 编辑',
  'ComplianceStatementItemsEditor',
]);

assertIncludes(
  'web/classic/src/components/settings/SourceAttributionSetting.jsx',
  ['可视化编辑', 'JSON 编辑', 'SourceAttributionOptionsEditor'],
);

assertNotIncludes('web/default/src/features/compliance/compliance-gate.tsx', [
  'activeSnapshot?.content_hash',
  "`${snapshot.legacy ? 'legacy' : snapshot.id}-${snapshot.content_hash}`",
  "t('Version')",
  "content_hash || '-'",
  'activeSnapshot?.content_length',
]);

assertNotIncludes('web/classic/src/components/compliance/ComplianceGate.jsx', [
  'statement?.content_hash, activeSnapshots',
  "snapshot.version || '-'",
  "statement?.content_hash || '-'",
  'statement?.content?.length',
]);

assertIncludes(
  'web/default/src/features/source-attribution/source-attribution-confirmations-table.tsx',
  ['item.target_username ||'],
);

assertIncludes(
  'web/classic/src/components/source-attribution/SourceAttributionConfirmations.jsx',
  ["dataIndex: 'target_username'"],
);

assertNotIncludes(
  'web/default/src/features/source-attribution/source-attribution-confirmations-table.tsx',
  ['target_inviter_name'],
);

assertNotIncludes(
  'web/classic/src/components/source-attribution/SourceAttributionConfirmations.jsx',
  ['target_inviter_name'],
);

for (const relativePath of [
  'web/default/src/hooks/use-sidebar-data.ts',
  'web/default/src/features/compliance/compliance-confirmations-table.tsx',
  'web/default/src/features/source-attribution/source-attribution-confirmations-table.tsx',
  'web/classic/src/components/layout/SiderBar.jsx',
  'web/classic/src/components/compliance/ComplianceConfirmations.jsx',
  'web/classic/src/components/source-attribution/SourceAttributionConfirmations.jsx',
]) {
  assertNotIncludes(relativePath, [
    'Compliance Confirmations',
    'Source attribution confirmations',
    'Source Attribution Confirmations',
    "t('Version')",
    "t('Username')",
    "t('Invitation code')",
    "t('Confirmed at')",
    "t('Refresh')",
    "t('User Agent')",
  ]);
}

if (failures.length > 0) {
  console.error(failures.map((failure) => `- ${failure}`).join('\n'));
  process.exit(1);
}

console.log('compliance display/editor checks passed');
