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

import {
  formatAuditDateTime,
  fromAuditDateTimeInput,
} from '../lib/audit-date-time'

test('associates each custom audit date input with a persistent label', () => {
  const source = readFileSync(
    new URL('./compact-date-time-range-picker.tsx', import.meta.url),
    'utf8'
  )

  expect(source.match(/htmlFor=/g) ?? []).toHaveLength(2)
  expect(source).toContain("t('Start Time')")
  expect(source).toContain("t('End Time')")
})

test('preserves preset end-of-day seconds when the minute draft is unchanged', () => {
  const original = new Date(2026, 0, 2, 23, 59, 59, 999)

  expect(fromAuditDateTimeInput('2026-01-02T23:59', original)?.getTime()).toBe(
    original.getTime()
  )
  expect(
    fromAuditDateTimeInput('2026-01-02T23:58', original)?.getSeconds()
  ).toBe(0)
  expect(formatAuditDateTime(original, 'fr-FR')).toBe(
    original.toLocaleString('fr-FR', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  )
})

test('formats the compact trigger with the active interface locale', () => {
  const source = readFileSync(
    new URL('./compact-date-time-range-picker.tsx', import.meta.url),
    'utf8'
  )

  expect(source).toContain('formatAuditDateTime')
  expect(source).not.toContain("format('YYYY-MM-DD HH:mm')")
})

test('uses shared date-time controls instead of native date-time inputs', () => {
  const source = readFileSync(
    new URL('./compact-date-time-range-picker.tsx', import.meta.url),
    'utf8'
  )

  expect(source.match(/<DateTimePicker/g) ?? []).toHaveLength(2)
  expect(source).not.toMatch(/type=['"]datetime-local['"]/)
})
