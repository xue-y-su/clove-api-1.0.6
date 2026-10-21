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
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

import {
  formatDateRangeLabel,
  getDateRangePreset,
  normalizeDateRange,
} from './date-range-picker-state'

describe('DateRangePicker', () => {
  test('formats empty, partial, and complete controlled values', () => {
    const from = new Date(2026, 6, 2)
    const to = new Date(2026, 6, 8)

    expect(formatDateRangeLabel({}, 'Choose range')).toBe('Choose range')
    expect(formatDateRangeLabel({ from }, 'Choose range')).toBe(
      '2026-07-02 ~ -'
    )
    expect(formatDateRangeLabel({ from, to }, 'Choose range')).toBe(
      '2026-07-02 ~ 2026-07-08'
    )
  })

  test('normalizes range boundaries in local time', () => {
    const range = normalizeDateRange({
      from: new Date(2026, 6, 2, 14, 25, 30, 123),
      to: new Date(2026, 6, 8, 7, 10, 4, 5),
    })

    expect(range.from).toEqual(new Date(2026, 6, 2, 0, 0, 0, 0))
    expect(range.to).toEqual(new Date(2026, 6, 8, 23, 59, 59, 999))
  })

  test('builds today, seven-day, and month presets', () => {
    const now = new Date(2026, 6, 18, 12, 30)

    expect(getDateRangePreset('today', now)).toEqual({
      from: new Date(2026, 6, 18, 0, 0, 0, 0),
      to: new Date(2026, 6, 18, 23, 59, 59, 999),
    })
    expect(getDateRangePreset('7d', now)).toEqual({
      from: new Date(2026, 6, 12, 0, 0, 0, 0),
      to: new Date(2026, 6, 18, 23, 59, 59, 999),
    })
    expect(getDateRangePreset('month', now)).toEqual({
      from: new Date(2026, 6, 1, 0, 0, 0, 0),
      to: new Date(2026, 6, 31, 23, 59, 59, 999),
    })
  })

  test('uses the project calendar instead of native date inputs', () => {
    const source = readFileSync(
      new URL('./date-range-picker.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain("mode='range'")
    expect(source).toContain('<Calendar')
    expect(source).not.toMatch(/type=['"]date['"]/)
    expect(source).toContain('zhCN')
    expect(source).toContain('zhTW')
  })
})
