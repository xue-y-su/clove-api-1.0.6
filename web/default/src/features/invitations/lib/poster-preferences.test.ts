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
  DEFAULT_POSTER_PREFERENCES,
  POSTER_PREFERENCES_STORAGE_KEY,
  countPosterCharacters,
  limitPosterCharacters,
  readPosterPreferences,
} from './poster-preferences'

function memoryStorage(initialValue?: string) {
  const values = new Map<string, string>()
  if (initialValue !== undefined) {
    values.set(POSTER_PREFERENCES_STORAGE_KEY, initialValue)
  }
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  }
}

describe('invitation poster preferences', () => {
  test('uses the shared v2 default for first-time preferences', () => {
    expect(readPosterPreferences(memoryStorage())).toEqual(
      DEFAULT_POSTER_PREFERENCES
    )
    expect(DEFAULT_POSTER_PREFERENCES).toMatchObject({
      version: 2,
      size: '3:4',
      showNickname: true,
    })
  })

  test('uses the shared v2 normalizer and storage key', () => {
    const storage = memoryStorage(
      JSON.stringify({ version: 1, size: '9:16', showNickname: false })
    )
    expect(readPosterPreferences(storage)).toMatchObject({
      version: 2,
      size: '9:16',
      showNickname: false,
    })
    expect(POSTER_PREFERENCES_STORAGE_KEY).toBe('invitation-poster-preferences')
  })

  test('ignores malformed and future-version preferences', () => {
    expect(readPosterPreferences(memoryStorage('{broken'))).toEqual(
      DEFAULT_POSTER_PREFERENCES
    )
    expect(
      readPosterPreferences(
        memoryStorage(
          JSON.stringify({ version: 3, size: '4:5', showNickname: false })
        )
      )
    ).toEqual(DEFAULT_POSTER_PREFERENCES)
  })

  test('does not define a second size or schema validator', async () => {
    const source = await Bun.file(
      new URL('./poster-preferences.ts', import.meta.url)
    ).text()
    expect(source).toContain(
      "from '../../../../../shared/invitation-poster-design.js'"
    )
    expect(source).not.toContain('VALID_POSTER_SIZES')
  })
})

test('counts and limits poster copy by Unicode code point', () => {
  expect(countPosterCharacters('邀请😜')).toBe(3)
  expect(limitPosterCharacters('邀请😜加入', 4)).toBe('邀请😜加')
})
