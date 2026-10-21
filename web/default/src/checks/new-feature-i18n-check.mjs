/*
Copyright (C) 2023-2026 QuantumNous

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
import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const srcRoot = resolve(__dirname, '..')
const localesDir = resolve(srcRoot, 'i18n/locales')

const filesUsingNewFeatureI18n = [
  'features/compliance/compliance-confirmations-table.tsx',
  'features/compliance/compliance-gate.tsx',
  'features/source-attribution/source-attribution-confirmations-table.tsx',
  'features/source-attribution/source-attribution-gate.tsx',
  'features/token-balance/components/balance-metrics.tsx',
  'features/token-balance/components/balance-progress.tsx',
  'features/token-balance/components/balance-restrictions.tsx',
  'features/token-balance/components/balance-result.tsx',
  'features/system-settings/site/compliance-section.tsx',
  'features/system-settings/site/compliance-statement-items-editor.tsx',
  'features/system-settings/site/registration-section.tsx',
  'features/system-settings/site/source-attribution-options-editor.tsx',
  'features/system-settings/site/source-attribution-section.tsx',
  'features/users/components/users-columns.tsx',
  'features/users/components/users-mutate-drawer.tsx',
  'features/wallet/components/affiliate-rewards-card.tsx',
]

const requiredKeys = [
  'Add option',
  'Add term',
  'Account Balance',
  'Available to this API Key now',
  'A value of 0 means common users cannot invite anyone.',
  'Administrators and root users are exempt when their own invite code is used unless a custom invite limit is set.',
  'Admins are exempt from invite limits',
  'Attribution option',
  'Attribution options JSON',
  'Body',
  'Key Configuration',
  'Checkbox confirmation',
  'Compliance Statement',
  'Compliance terms must be a JSON array',
  'Compliance terms must be valid JSON',
  'Common users can invite unlimited people by default.',
  'Confirm',
  'Confirmation method',
  'Confirmation phrase',
  'Confirmation time',
  'Confirming...',
  'Content comes from the current user agreement or privacy policy. Backend snapshots are used when saving.',
  'Content hash',
  'Content is required when compliance statement is enabled',
  'Custom invite limit',
  'Custom limit',
  'Current term',
  'No independent quota limit',
  'Description required',
  'Default invitation limit',
  'Display mode',
  'Do not limit invitations by default',
  'Enable compliance statement',
  'Enable source attribution',
  'Enforce confirmation for Token API',
  'Enforce source attribution for Token API',
  'Enter source description',
  'Enter confirmation phrase',
  'Explain why this user can be created without an invitation code.',
  'Failed to confirm source attribution',
  'Follow system',
  'Confirm current term',
  'Invite Limit',
  'Invite limit mode',
  'Invited by user ID',
  'Inviter',
  'JSON Editor',
  'Leave empty only when creating this user without an invitation code.',
  'Manage compliance terms visually. JSON editing still preserves all advanced fields.',
  'Manage source attribution options visually. Saving still writes through the dedicated API.',
  'Manual statement',
  'Multiple dialogs',
  'No source attribution options yet',
  'No terms configured yet. The system will use the legacy single statement above.',
  'One segment per line for users to copy or type for confirmation',
  'Pending {{current}} / {{total}}',
  'Quota Breakdown',
  'Pending 1 item',
  'Please read and confirm the current compliance statement.',
  'Please confirm your invitation source first.',
  'Privacy Policy',
  'Referral reward transfer is disabled until the administrator confirms compliance terms.',
  'Reject API token calls until the user confirms.',
  'Reject API token calls until the user selects a source attribution.',
  'Require console users to confirm the compliance statement.',
  'Require invitation code for registration',
  'Require source description',
  'Required when invitation registration is enforced and no invitation code is provided.',
  'Save registration settings',
  'Segment content',
  'Segment copy or input',
  'Select an attribution option',
  'Select invite limit mode',
  'Target invitation code',
  'Refresh records',
  'Show content hash to users',
  'Show version to users',
  'Single dialog',
  'Skip invitation reason',
  'Sort order',
  'Source Attribution',
  'Source attribution confirmed',
  'Source attribution options must be a JSON array',
  'Source description',
  'Source type',
  'Restricted by available funding',
  'Statement content',
  'Term',
  'Term ID',
  'Terms JSON',
  'Total consumed by this API Key',
  'Title',
  'User Agreement',
  'User agent',
  'Unavailable Quota',
  'Compliance statement confirmed',
  'Enter or paste this segment',
  'I have read and agree to the current terms',
  'Segment {{number}}',
  'Users without an inviter must select a source attribution before continuing.',
  'Version is required when compliance statement is enabled',
  'Version number',
  'Visual editor',
  'When custom limit is selected, 0 means no one can be invited.',
  'When disabled, the number below limits successful invitation-code registrations for common users. Administrators and root users are exempt unless a custom invite limit is set.',
  'When disabled, regular user compliance dialogs and statement APIs will not show the content hash.',
  'When disabled, regular user compliance dialogs and statement APIs will not show the version.',
]

const cjkPattern = /[\u3400-\u9fff]/
const stringLiteralPattern =
  /(["'`])((?:\\.|(?!\1)[\s\S])*?[\u3400-\u9fff][\s\S]*?)\1/g

const cjkFindings = []
for (const relativePath of filesUsingNewFeatureI18n) {
  const source = readFileSync(resolve(srcRoot, relativePath), 'utf8')
  for (const match of source.matchAll(stringLiteralPattern)) {
    const literal = match[2].split('\n')[0]
    if (cjkPattern.test(literal)) {
      cjkFindings.push(`${relativePath}: ${literal}`)
    }
  }
}

assert.deepEqual(
  cjkFindings,
  [],
  `新增功能 UI 文案必须使用英文 i18n source key，不能在源码里硬编码中文:\n${cjkFindings.join(
    '\n'
  )}`
)

const baseTranslations = JSON.parse(
  readFileSync(resolve(localesDir, 'en.json'), 'utf8')
).translation

for (const localeFile of readdirSync(localesDir).filter((file) =>
  file.endsWith('.json')
)) {
  const locale = JSON.parse(
    readFileSync(resolve(localesDir, localeFile), 'utf8')
  )
  const translations = locale.translation ?? {}
  const missing = requiredKeys.filter((key) => !(key in translations))

  assert.deepEqual(
    missing,
    [],
    `${localeFile} 缺少新增功能翻译 key: ${missing.join(', ')}`
  )

  if (localeFile === 'en.json') continue

  const untranslated = requiredKeys.filter(
    (key) => translations[key] === baseTranslations[key]
  )
  assert.deepEqual(
    untranslated,
    [],
    `${localeFile} 新增功能翻译仍为英文原文: ${untranslated.join(', ')}`
  )
}

process.stdout.write('new feature i18n checks passed\n')
