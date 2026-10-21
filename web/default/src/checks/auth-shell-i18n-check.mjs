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

const sources = [
  'features/auth/components/terms-footer.tsx',
  'features/auth/components/legal-consent.tsx',
  'components/ui/command.tsx',
  'components/ui/sidebar.tsx',
].map((path) => [path, readFileSync(resolve(srcRoot, path), 'utf8')])

for (const [path, source] of sources) {
  assert.match(
    source,
    /useTranslation/,
    `${path} 必须通过 i18n 渲染可见或无障碍文案`
  )
}

assert.doesNotMatch(
  sources[1][1],
  /&& ' and the '/,
  '认证同意勾选项不能硬编码英文连接词'
)
assert.match(sources[0][1], /t\(text\)/, '认证页脚说明必须翻译')
assert.match(sources[0][1], /t\(firstLink\.label\)/, '认证页脚协议名称必须翻译')

const requiredKeys = [
  'By clicking sign in, you agree to our',
  'By creating an account, you agree to our',
  'Command Palette',
  'Search for a command to run...',
  'Toggle Sidebar',
]
const baseTranslations = JSON.parse(
  readFileSync(resolve(localesDir, 'en.json'), 'utf8')
).translation

for (const localeFile of readdirSync(localesDir).filter((file) =>
  file.endsWith('.json')
)) {
  const translations = JSON.parse(
    readFileSync(resolve(localesDir, localeFile), 'utf8')
  ).translation
  const missing = requiredKeys.filter((key) => !(key in translations))
  assert.deepEqual(missing, [], `${localeFile} 缺少认证与导航翻译: ${missing}`)

  if (localeFile === 'en.json') continue
  const untranslated = requiredKeys.filter(
    (key) => translations[key] === baseTranslations[key]
  )
  assert.deepEqual(
    untranslated,
    [],
    `${localeFile} 认证与导航翻译仍为英文原文: ${untranslated}`
  )
}

process.stdout.write('auth shell i18n checks passed\n')
