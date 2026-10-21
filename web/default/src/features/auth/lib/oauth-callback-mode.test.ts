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
  markOAuthBindPopup,
  resolveOAuthCallbackMode,
  type OAuthModeStorage,
} from './oauth-callback-mode'

function fakeStorage(): OAuthModeStorage {
  const data = new Map<string, string>()
  return {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => void data.set(key, value),
  }
}

describe('resolveOAuthCallbackMode', () => {
  test('accepts a matching bind marker with a live opener', () => {
    const storage = fakeStorage()
    expect(markOAuthBindPopup(storage, 'oidc', 'state-1')).toBe(true)
    expect(
      resolveOAuthCallbackMode('oidc', 'state-1', {
        opener: { closed: false },
        storage,
      })
    ).toBe('bind')
  })

  test('does not mistake a foreign opener for a bind flow', () => {
    expect(
      resolveOAuthCallbackMode('oidc', 'state-1', {
        opener: { closed: false },
        storage: fakeStorage(),
      })
    ).toBe('login')
  })

  test('requires the same provider and state', () => {
    const storage = fakeStorage()
    markOAuthBindPopup(storage, 'github', 'state-1')
    expect(
      resolveOAuthCallbackMode('oidc', 'state-1', {
        opener: { closed: false },
        storage,
      })
    ).toBe('login')
  })
})
