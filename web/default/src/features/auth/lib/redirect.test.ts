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

import { sanitizeRedirectTarget } from './redirect'

describe('sanitizeRedirectTarget', () => {
  test('keeps same-site absolute paths', () => {
    expect(sanitizeRedirectTarget('/dashboard?tab=billing')).toBe(
      '/dashboard?tab=billing'
    )
  })

  test('falls back for absolute external URLs', () => {
    expect(sanitizeRedirectTarget('https://evil.example/phish')).toBe(
      '/dashboard'
    )
  })

  test('falls back for protocol-relative URLs', () => {
    expect(sanitizeRedirectTarget('//evil.example/phish')).toBe('/dashboard')
  })

  test('falls back for active URL schemes', () => {
    expect(sanitizeRedirectTarget('javascript:alert(1)')).toBe('/dashboard')
    expect(sanitizeRedirectTarget('data:text/html,<script>1</script>')).toBe(
      '/dashboard'
    )
  })

  test('supports custom fallback', () => {
    expect(sanitizeRedirectTarget(undefined, '/sign-in')).toBe('/sign-in')
    expect(sanitizeRedirectTarget('settings', '/sign-in')).toBe('/sign-in')
  })
})
