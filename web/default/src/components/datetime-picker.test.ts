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

import { clampDateTime } from './datetime-picker-state'

const source = readFileSync(
  new URL('./datetime-picker.tsx', import.meta.url),
  'utf8'
)

test('supports controlled bounds and an accessible trigger', () => {
  expect(source).toContain('minDate?: Date')
  expect(source).toContain('maxDate?: Date')
  expect(source).toContain("'aria-label'?: string")
  expect(source).toContain('disabled={getDisabledDates(minDate, maxDate)}')
  expect(source).toContain('aria-label={ariaLabel}')
})

test('keeps clear behavior and avoids native date-time inputs', () => {
  expect(source).toContain('onChange?.(undefined)')
  expect(source).not.toMatch(/type=['"](?:date|datetime-local)['"]/)
})

test('clamps the time on boundary days', () => {
  const min = new Date(2026, 7, 19, 14, 30)
  const max = new Date(2026, 7, 20, 18, 0)

  expect(clampDateTime(new Date(2026, 7, 19, 8, 0), min, max)).toEqual(min)
  expect(clampDateTime(new Date(2026, 7, 20, 20, 0), min, max)).toEqual(max)
  expect(clampDateTime(new Date(2026, 7, 20, 12, 0), min, max)).toEqual(
    new Date(2026, 7, 20, 12, 0)
  )
})
