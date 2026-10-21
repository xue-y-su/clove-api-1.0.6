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
import { expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

test('translates reusable channel control labels', () => {
  const spinnerSource = readFileSync(
    new URL('./numeric-spinner-input.tsx', import.meta.url),
    'utf8'
  )
  const advancedCustomSource = readFileSync(
    new URL('./dialogs/advanced-custom-editor-dialog.tsx', import.meta.url),
    'utf8'
  )
  const codexUsageSource = readFileSync(
    new URL('./dialogs/codex-usage-dialog.tsx', import.meta.url),
    'utf8'
  )

  expect(spinnerSource).toContain("aria-label={t('Decrement')}")
  expect(spinnerSource).toContain("aria-label={t('Increment')}")
  expect(advancedCustomSource).toContain('{t(option.label)}')
  expect(codexUsageSource).toContain('label={t(accountBadge.label)}')
  expect(codexUsageSource).toContain("label={t('User ID')}")
})
