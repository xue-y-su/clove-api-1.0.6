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
  createTokenRevealMemory,
  normalizeApiKeyCredential,
} from './token-reveal-memory'

describe('token reveal memory', () => {
  test('clears all keys exactly when the server window expires', () => {
    let now = 1_000_000
    const memory = createTokenRevealMemory(() => now)
    memory.setVerifiedUntil(1_030)
    memory.setKey(7, 'secret-key-value')

    now = 1_029_999
    expect(memory.getKey(7)).toBe('secret-key-value')

    now = 1_030_000
    expect(memory.getKey(7)).toBeUndefined()
    expect(memory.snapshot()).toEqual({})
  })

  test('preserves the complete credential without adding a prefix', () => {
    expect(normalizeApiKeyCredential('  plain-key-value  ')).toBe(
      'plain-key-value'
    )
    expect(normalizeApiKeyCredential('sk-custom-key-value')).toBe(
      'sk-custom-key-value'
    )
  })
})
