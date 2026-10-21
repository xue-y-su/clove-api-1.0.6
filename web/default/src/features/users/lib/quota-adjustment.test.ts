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

import { canSubmitQuotaAdjustment } from './quota-adjustment'

describe('quota adjustment validation', () => {
  test('rejects empty, zero, and sub-unit additive adjustments', () => {
    expect(canSubmitQuotaAdjustment('add', '', 0)).toBe(false)
    expect(canSubmitQuotaAdjustment('add', '0', 0)).toBe(false)
    expect(canSubmitQuotaAdjustment('subtract', '0', 0)).toBe(false)
    expect(canSubmitQuotaAdjustment('add', '0.0000001', 0)).toBe(false)
    expect(canSubmitQuotaAdjustment('add', '1', 500000)).toBe(true)
  })

  test('allows an explicit zero override but not a blank override', () => {
    expect(canSubmitQuotaAdjustment('override', '0', 0)).toBe(true)
    expect(canSubmitQuotaAdjustment('override', '', 0)).toBe(false)
  })
})
