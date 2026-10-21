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

import { getDisplayLogQuota, isPerCallBilling } from './utils'

describe('isPerCallBilling', () => {
  test('recognizes a zero-priced per-call model from explicit billing mode', () => {
    expect(isPerCallBilling(0, 'per_call')).toBe(true)
  })

  test('keeps explicit token and tiered modes out of per-call display', () => {
    expect(isPerCallBilling(1, 'per_token')).toBe(false)
    expect(isPerCallBilling(1, 'tiered_expr')).toBe(false)
  })

  test('preserves historical log fallback based on a positive model price', () => {
    expect(isPerCallBilling(1)).toBe(true)
    expect(isPerCallBilling(0)).toBe(false)
  })
})

describe('getDisplayLogQuota', () => {
  test('hides the legacy one-quota floor for a free per-call model', () => {
    expect(
      getDisplayLogQuota(1, {
        billing_mode: 'per_call',
        model_price: 0,
        billing_breakdown: {
          model_quota: 1,
          tool_quota: 0,
          total_quota: 1,
        },
      })
    ).toBe(0)
  })

  test('keeps tool charges for a free per-call model', () => {
    expect(
      getDisplayLogQuota(2500, {
        billing_mode: 'per_call',
        model_price: 0,
        billing_breakdown: {
          model_quota: 0,
          tool_quota: 2500,
          total_quota: 2500,
        },
      })
    ).toBe(2500)
  })
})
