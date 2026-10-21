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
  getPricingViewState,
  reconcilePricingGroup,
} from './pricing-view-state'

describe('pricing request state', () => {
  test('keeps server errors distinct from an empty model list', () => {
    expect(
      getPricingViewState({ isLoading: false, hasError: true, modelCount: 0 })
    ).toBe('error')
    expect(
      getPricingViewState({ isLoading: false, hasError: false, modelCount: 0 })
    ).toBe('empty')
  })

  test('prioritizes loading before content and empty states', () => {
    expect(
      getPricingViewState({ isLoading: true, hasError: false, modelCount: 1 })
    ).toBe('loading')
    expect(
      getPricingViewState({ isLoading: false, hasError: false, modelCount: 1 })
    ).toBe('content')
  })
})

test('resets a group removed by a new catalog projection', () => {
  expect(reconcilePricingGroup('secret', [{ name: 'default' }])).toBe('all')
  expect(reconcilePricingGroup('default', [{ name: 'default' }])).toBe(
    'default'
  )
  expect(reconcilePricingGroup('all', [])).toBe('all')
})
