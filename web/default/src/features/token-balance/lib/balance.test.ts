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

import type { TokenBalanceCalculation } from '../types'
import {
  formatBalanceQuota,
  getBalanceMetricValues,
  getBalanceProgressSegments,
} from './balance'

function calculation(
  overrides: Partial<TokenBalanceCalculation> = {}
): TokenBalanceCalculation {
  return {
    token_unlimited: false,
    token_total: 150,
    token_remaining: 100,
    token_used: 50,
    token_callable_quota: { unlimited: false, value: 60 },
    current_available: { unlimited: false, value: 1000 },
    restricted: 40,
    single_request_limit: { unlimited: false, value: 60 },
    participating_sources: ['subscription'],
    restriction_reason: 'subscription_quota',
    subscription: {
      available: { unlimited: false, value: 60 },
      used: 40,
      total: { unlimited: false, value: 100 },
    },
    ...overrides,
  }
}

describe('token balance progress segments', () => {
  test('uses the finite key total for available, used, and restricted segments', () => {
    const result = getBalanceProgressSegments(calculation())

    expect(result.available).toBeCloseTo(40)
    expect(result.used).toBeCloseTo(100 / 3)
    expect(result.restricted).toBeCloseTo(80 / 3)
    expect(result.available + result.used + result.restricted).toBeCloseTo(100)
  })

  test('uses token callable quota plus key usage for an unlimited key snapshot', () => {
    const result = getBalanceProgressSegments(
      calculation({
        token_unlimited: true,
        token_total: 0,
        token_remaining: 0,
        token_used: 40,
        token_callable_quota: { unlimited: false, value: 60 },
        current_available: { unlimited: false, value: 60 },
        restricted: 0,
        restriction_reason: '',
      })
    )

    expect(result.available).toBeCloseTo(60)
    expect(result.used).toBeCloseTo(40)
    expect(result.restricted).toBe(0)
  })

  test('represents unlimited funding without a numeric sentinel', () => {
    const result = getBalanceProgressSegments(
      calculation({
        token_unlimited: true,
        token_total: 0,
        token_remaining: 0,
        token_used: 500,
        token_callable_quota: { unlimited: true, value: 0 },
        current_available: { unlimited: true, value: 0 },
        restricted: 0,
      })
    )

    expect(result).toEqual({ available: 100, used: 0, restricted: 0 })
  })

  test('returns an empty track when there is no quota or usage', () => {
    const result = getBalanceProgressSegments(
      calculation({
        token_total: 0,
        token_remaining: 0,
        token_used: 0,
        token_callable_quota: { unlimited: false, value: 0 },
        current_available: { unlimited: false, value: 0 },
        restricted: 0,
      })
    )

    expect(result).toEqual({ available: 0, used: 0, restricted: 0 })
  })
})

describe('token balance quota formatting', () => {
  test('converts quota units using the response display configuration', () => {
    expect(
      formatBalanceQuota(
        { unlimited: false, value: 30_000_000 },
        { type: 'USD', quota_per_unit: 500_000, exchange_rate: 1, symbol: '$' },
        'Unlimited'
      )
    ).toBe('$60.00')

    expect(
      formatBalanceQuota(
        { unlimited: false, value: 500_000 },
        {
          type: 'CNY',
          quota_per_unit: 500_000,
          exchange_rate: 7.3,
          symbol: '¥',
        },
        'Unlimited'
      )
    ).toBe('¥7.30')
  })

  test('keeps token displays in raw quota units', () => {
    expect(
      formatBalanceQuota(
        { unlimited: false, value: 12_345 },
        {
          type: 'TOKENS',
          quota_per_unit: 500_000,
          exchange_rate: 1,
          symbol: '',
        },
        'Unlimited'
      )
    ).toBe('12,345')
  })

  test('uses the explicit unlimited label instead of a numeric value', () => {
    expect(
      formatBalanceQuota(
        { unlimited: true, value: 0 },
        { type: 'USD', quota_per_unit: 500_000, exchange_rate: 1, symbol: '$' },
        'Unlimited'
      )
    ).toBe('Unlimited')
  })
})

describe('token balance metric values', () => {
  test('uses restricted quota as the third metric for a finite key', () => {
    expect(getBalanceMetricValues(calculation())).toEqual({
      available: { unlimited: false, value: 60 },
      used: 50,
      third: { kind: 'restricted', value: 40 },
    })
  })

  test('uses an unlimited marker as the third metric for an unlimited key', () => {
    expect(
      getBalanceMetricValues(
        calculation({
          token_unlimited: true,
          token_total: 0,
          token_remaining: 0,
          restricted: 0,
          restriction_reason: '',
        })
      )
    ).toEqual({
      available: { unlimited: false, value: 60 },
      used: 50,
      third: { kind: 'unlimited', value: 0 },
    })
  })
})
