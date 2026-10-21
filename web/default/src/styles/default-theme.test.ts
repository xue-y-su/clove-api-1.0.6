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

import { THEME_PRESETS } from '@/lib/theme-customization'

const themeCss = await Bun.file(new URL('./theme.css', import.meta.url)).text()

function themeBlock(selector: string): string {
  const escapedSelector = selector.replaceAll(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = themeCss.match(
    new RegExp(`${escapedSelector}\\s*\\{([\\s\\S]*?)\\n\\}`)
  )
  if (!match) throw new Error(`Missing ${selector} theme block`)
  return match[1]
}

describe('default theme palette', () => {
  test('keeps primary controls neutral in light and dark mode', () => {
    expect(themeBlock(':root')).toContain('--primary: oklch(0.13 0 0);')
    expect(themeBlock(':root')).toContain(
      '--primary-foreground: oklch(0.985 0 0);'
    )
    expect(themeBlock('.dark')).toContain('--primary: oklch(0.965 0 0);')
    expect(themeBlock('.dark')).toContain(
      '--primary-foreground: oklch(0.155 0 0);'
    )
  })

  test('previews the default preset as a neutral palette', () => {
    const defaultPreset = THEME_PRESETS.find(
      (preset) => preset.value === 'default'
    )

    expect(defaultPreset?.swatches).toEqual([
      'oklch(0.13 0 0)',
      'oklch(0.95 0 0)',
    ])
  })
})
