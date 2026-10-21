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

const systemInfoKeys = [
  'Online',
  'Stale',
  'Master',
  'Worker',
  'Pending',
  'Running',
  'Succeeded',
  'Failed',
  'Log cleanup',
  'Batch channel test',
  'Batch upstream model update',
  'Drawing task polling',
  'Async task polling',
  'Master instances run scheduled background tasks.',
  'Worker instances do not run master-only background tasks.',
]

const instancesPanel = readFileSync(
  resolve(
    srcRoot,
    'features/system-info/components/system-instances-panel.tsx'
  ),
  'utf8'
)
const tasksPanel = readFileSync(
  resolve(srcRoot, 'features/system-info/components/system-tasks-panel.tsx'),
  'utf8'
)

assert.doesNotMatch(
  instancesPanel,
  /\{roleLabel\(instance\)\}\s*<\/Badge>/,
  'system-info 实例角色徽标不能裸显示 master/worker，必须经过 t(...) 翻译'
)
assert.match(
  instancesPanel,
  /INSTANCE_STATUS_LABEL/,
  'system-info 实例状态应该通过稳定的 i18n key 映射显示'
)
assert.match(
  tasksPanel,
  /TASK_STATUS_LABEL/,
  'system-info 任务状态应该通过稳定的 i18n key 映射显示'
)

for (const localeFile of readdirSync(localesDir).filter((file) =>
  file.endsWith('.json')
)) {
  const locale = JSON.parse(
    readFileSync(resolve(localesDir, localeFile), 'utf8')
  )
  const translations = locale.translation ?? {}
  const missing = systemInfoKeys.filter((key) => !(key in translations))

  assert.deepEqual(
    missing,
    [],
    `${localeFile} 缺少 system-info 翻译 key: ${missing.join(', ')}`
  )
}

process.stdout.write('system-info i18n checks passed\n')
