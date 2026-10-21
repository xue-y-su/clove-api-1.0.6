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

describe('prefill group translations', () => {
  test('translates form labels, options, badges, and notifications', () => {
    const source = readFileSync(
      new URL('./drawers/prefill-group-form-drawer.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain("<FormLabel>{t('Group Type')}</FormLabel>")
    expect(source).toContain('t(type.label)')
    expect(source).toContain('t(type.description)')
    expect(source).toContain('label={t(meta.label)}')
    expect(source).toContain("t('Prefill group updated')")
    expect(source).toContain("t('Prefill group created')")
    expect(source).toContain("t('Operation failed')")
  })

  test('keeps translated group type descriptions readable', () => {
    const source = readFileSync(
      new URL('./drawers/prefill-group-form-drawer.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain('w-[420px] max-w-[calc(100vw-2rem)]')
    expect(source).toContain(
      '[&_[data-slot=select-item-text]]:whitespace-normal'
    )
    expect(source).toContain("align='start'")
    expect(source).toContain('className={groupTypeSelectContentClass}')
    expect(source).toContain('className={groupTypeSelectItemClass}')
  })

  test('translates management states and localized counts', () => {
    const source = readFileSync(
      new URL('./dialogs/prefill-group-management-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain('t(meta.label)')
    expect(source).toContain("t('No description provided')")
    expect(source).toContain("t('+{{count}} more'")
    expect(source).toContain("t('{{count}} item'")
    expect(source).toContain("t('{{count}} items'")
    expect(source).toContain("t('{{count}} group'")
    expect(source).toContain("t('{{count}} groups'")
    expect(source).toContain("t('No endpoint mappings configured.')")
    expect(source).toContain("t('No items configured yet.')")
    expect(source).toContain("t('Please retry or refresh the page.')")
  })
})
