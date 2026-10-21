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
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { isSnowHomePage } from './home-page-mode'

const homeSource = readFileSync(
  resolve(import.meta.dir, '..', 'index.tsx'),
  'utf8'
)

describe('home page mode', () => {
  test('embeds Snow only for the explicit snow mode', () => {
    expect(isSnowHomePage('snow')).toBe(true)
    expect(isSnowHomePage('default')).toBe(false)
    expect(isSnowHomePage(undefined)).toBe(false)
  })

  test('keeps the default homepage content inside the main landmark', () => {
    expect(homeSource).toContain('<main>\n        <Hero')
    expect(homeSource).toContain(
      '<CTA isAuthenticated={isAuthenticated} />\n      </main>\n      <Footer />'
    )
  })
})
