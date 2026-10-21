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

import {
  getBalanceQueryErrorPresentation,
  getBalanceStatusExplanationKey,
} from './presentation'

describe('token balance query error presentation', () => {
  test('keeps invalid keys generic', () => {
    expect(getBalanceQueryErrorPresentation('invalid_api_key')).toEqual({
      titleKey: 'Balance Query Failed',
      descriptionKey: 'The API Key is invalid. Check it and try again.',
      tone: 'destructive',
    })
  })

  test('uses retry metadata for rate limits', () => {
    expect(getBalanceQueryErrorPresentation('rate_limited', 60)).toEqual({
      titleKey: 'Too Many Requests',
      descriptionKey: 'Try again in {{seconds}} seconds.',
      tone: 'warning',
      values: { seconds: 60 },
    })
  })

  test('falls back to a generic service error', () => {
    expect(getBalanceQueryErrorPresentation('unknown')).toEqual({
      titleKey: 'Unable to Query Balance',
      descriptionKey:
        'The balance service is temporarily unavailable. Try again later.',
      tone: 'default',
    })
  })
})

describe('token balance unavailable status presentation', () => {
  test('explains each recognized unavailable status', () => {
    expect(getBalanceStatusExplanationKey('disabled')).toBe(
      'This API Key has been disabled.'
    )
    expect(getBalanceStatusExplanationKey('expired')).toBe(
      'This API Key has expired.'
    )
    expect(getBalanceStatusExplanationKey('exhausted')).toBe(
      'This API Key has no remaining quota.'
    )
    expect(getBalanceStatusExplanationKey('user_disabled')).toBe(
      'The account for this API Key has been disabled.'
    )
    expect(getBalanceStatusExplanationKey('active')).toBeNull()
  })
})
