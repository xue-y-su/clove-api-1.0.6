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

test('renders playground option labels instead of internal values', () => {
  const source = readFileSync(
    new URL('./playground-input-tools.tsx', import.meta.url),
    'utf8'
  )

  expect(source).toContain('activeWebSearchLabel')
  expect(source).toContain('activeReasoningEffortLabel')
  expect(source).toContain('t(activeWebSearchLabel)')
  expect(source).toContain('t(activeReasoningEffortLabel)')
  expect(source).toContain("t('Selected')")
  expect(source).not.toContain('t(webSearchMode)')
  expect(source).not.toContain('t(reasoningEffort)')
})

test('keeps reasoning controls available for every model', () => {
  const source = readFileSync(
    new URL('./playground-input-tools.tsx', import.meta.url),
    'utf8'
  )

  expect(source).not.toContain('supportsPlaygroundReasoning(config.model)')
  expect(source).toContain('disabled={disabled}')
  expect(source).toContain('Higher reasoning effort may use more tokens')
})
