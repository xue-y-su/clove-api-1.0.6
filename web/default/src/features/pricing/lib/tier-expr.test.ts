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

import { evalExprLocally, type ExtraTokenValues } from './tier-expr'

const noExtraTokens: ExtraTokenValues = {
  cacheReadTokens: 0,
  cacheCreateTokens: 0,
  cacheCreate1hTokens: 0,
  imageTokens: 0,
  imageOutputTokens: 0,
  audioInputTokens: 0,
  audioOutputTokens: 0,
}

describe('tiered billing expression preview', () => {
  test('evaluates supported tier, conditional, and math expressions', () => {
    const result = evalExprLocally(
      'len <= 300 ? tier("small", ceil(p * 1.5) + max(c, 10)) : tier("large", p * 3)',
      100,
      20,
      {
        ...noExtraTokens,
        cacheReadTokens: 50,
        cacheCreateTokens: 25,
      }
    )

    expect(result).toEqual({ cost: 170, matchedTier: 'small', error: null })
  })

  test('rejects unknown variables without evaluating them', () => {
    const result = evalExprLocally(
      'tier("base", p + unknown_variable)',
      100,
      0,
      noExtraTokens
    )

    expect(result.cost).toBe(0)
    expect(result.matchedTier).toBe('')
    expect(result.error).toContain('unknown_variable')
  })

  test('does not allow expressions to execute arbitrary JavaScript', () => {
    const runtime = globalThis as typeof globalThis & {
      __billingExprExecuted?: boolean
    }
    delete runtime.__billingExprExecuted

    const result = evalExprLocally(
      'tier.constructor("globalThis.__billingExprExecuted = true")() || tier("base", p)',
      100,
      0,
      noExtraTokens
    )

    expect(runtime.__billingExprExecuted).not.toBe(true)
    expect(result.cost).toBe(0)
    expect(result.matchedTier).toBe('')
    expect(result.error).not.toBeNull()
    delete runtime.__billingExprExecuted
  })
})
