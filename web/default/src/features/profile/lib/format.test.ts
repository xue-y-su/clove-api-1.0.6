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

import { formatProfileRelativeTime } from './format'

describe('profile time formatting', () => {
  test('uses the selected interface locale for relative time', () => {
    const originalNow = Date.now
    Date.now = () => Date.UTC(2026, 6, 28, 0, 0, 0)

    try {
      const lastUsed = '2026-07-03T00:00:00.000Z'

      expect(formatProfileRelativeTime(lastUsed, 'zh-CN')).toBe('25天前')
      expect(formatProfileRelativeTime(lastUsed, 'en')).toBe('25 days ago')
    } finally {
      Date.now = originalNow
    }
  })

  test('returns null when no valid timestamp exists', () => {
    expect(formatProfileRelativeTime(null, 'zh-CN')).toBeNull()
    expect(formatProfileRelativeTime('not-a-date', 'zh-CN')).toBeNull()
  })
})
