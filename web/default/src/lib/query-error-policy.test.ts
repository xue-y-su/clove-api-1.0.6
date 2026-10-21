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

import { createAuthStore } from '../stores/auth-store'
import {
  getQueryErrorAction,
  isRecoverableAssetError,
  shouldReloadForAssetError,
} from './query-error-policy'

const refreshedUser = {
  id: 1,
  username: 'clove',
  role: 100,
}

function createMemoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value)
    },
    removeItem: (key: string) => {
      values.delete(key)
    },
  }
}

describe('global query error policy', () => {
  test('expires the session for unauthorized responses', () => {
    expect(getQueryErrorAction(401)).toBe('session-expired')
  })

  test('reports server errors without replacing the current page', () => {
    expect(getQueryErrorAction(500)).toBe('report-server-error')
  })

  test('leaves other errors to their local query consumers', () => {
    expect(getQueryErrorAction(403)).toBe('local')
    expect(getQueryErrorAction(undefined)).toBe('local')
  })
})

describe('client asset error recovery', () => {
  test('recognizes stale route chunk failures without matching API 500 errors', () => {
    expect(
      isRecoverableAssetError(
        new TypeError('Failed to fetch dynamically imported module')
      )
    ).toBeTrue()

    const chunkError = new Error('Loading chunk 42 failed')
    chunkError.name = 'ChunkLoadError'
    expect(isRecoverableAssetError(chunkError)).toBeTrue()
    expect(
      isRecoverableAssetError(new Error('Request failed with status code 500'))
    ).toBeFalse()
  })

  test('allows one automatic reload per recovery window', () => {
    const storage = createMemoryStorage()
    const error = new Error('Loading chunk 42 failed')

    expect(shouldReloadForAssetError(error, storage, 100_000)).toBeTrue()
    expect(shouldReloadForAssetError(error, storage, 110_000)).toBeFalse()
    expect(shouldReloadForAssetError(error, storage, 131_000)).toBeTrue()
  })
})

describe('auth session persistence failures', () => {
  test('repairs a stale request uid from the persisted authenticated user', () => {
    const storage = createMemoryStorage()
    storage.setItem('user', JSON.stringify(refreshedUser))
    storage.setItem('uid', '9400044')

    const store = createAuthStore(storage)

    expect(store.getState().auth.user).toEqual(refreshedUser)
    expect(storage.getItem('uid')).toBe('1')
  })

  test('keeps the request uid atomic with auth changes and session reset', () => {
    const storage = createMemoryStorage()
    const store = createAuthStore(storage)

    store.getState().auth.setUser(refreshedUser)
    expect(storage.getItem('uid')).toBe('1')

    store.getState().auth.reset()
    expect(storage.getItem('user')).toBeNull()
    expect(storage.getItem('uid')).toBeNull()
  })

  test('keeps a refreshed session usable when browser storage rejects writes', () => {
    const storage = {
      getItem: () => null,
      setItem: () => {
        throw new Error('storage unavailable')
      },
      removeItem: () => {
        throw new Error('storage unavailable')
      },
    }
    const store = createAuthStore(storage)

    expect(() => store.getState().auth.setUser(refreshedUser)).not.toThrow()
    expect(store.getState().auth.user).toEqual(refreshedUser)
    expect(() => store.getState().auth.reset()).not.toThrow()
    expect(store.getState().auth.user).toBeNull()
  })

  test('starts signed out when persisted session data is malformed', () => {
    const storage = {
      getItem: () => '{broken-json',
      setItem: () => undefined,
      removeItem: () => {
        throw new Error('storage cleanup unavailable')
      },
    }

    expect(() => createAuthStore(storage)).not.toThrow()
    expect(createAuthStore(storage).getState().auth.user).toBeNull()
  })
})
