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

const apiSource = await Bun.file(new URL('./api.ts', import.meta.url)).text()
const editorSource = await Bun.file(
  new URL(
    '../system-settings/site/source-attribution-options-editor.tsx',
    import.meta.url
  )
).text()
const confirmationsSource = await Bun.file(
  new URL('./source-attribution-confirmations-table.tsx', import.meta.url)
).text()
const sectionSource = await Bun.file(
  new URL(
    '../system-settings/site/source-attribution-section.tsx',
    import.meta.url
  )
).text()

describe('source attribution target user contract', () => {
  test('edits a numeric target user ID instead of an invitation code', () => {
    expect(apiSource).toMatch(
      /SourceAttributionAdminOption[\s\S]{0,160}target_user_id:\s*number/
    )
    expect(editorSource).toContain('target_user_id: 0')
    expect(editorSource).toContain("t('Target user ID')")
    expect(editorSource).toMatch(
      /type='number'[\s\S]{0,180}value=\{option\.target_user_id\}/
    )
    expect(editorSource).not.toContain('option.target_aff_code')
  })

  test('filters and displays confirmation records by target user ID', () => {
    expect(apiSource).toContain('target_user_id?: number')
    expect(confirmationsSource).toContain('target_user_id:')
    expect(confirmationsSource).toContain('item.target_user_id')
    expect(confirmationsSource).not.toContain('filters.target_aff_code')
  })

  test('describes the JSON contract with a target user ID', () => {
    expect(sectionSource).toContain(
      'Each option includes option ID, name, target user ID, whether source description is required, and sort order.'
    )
    expect(sectionSource).not.toContain('target invitation code')
  })

  test('translates target user labels in every supported locale', async () => {
    const localeFiles = [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-TW.json',
      'zh.json',
    ]
    for (const localeFile of localeFiles) {
      const locale = (await Bun.file(
        new URL(`../../i18n/locales/${localeFile}`, import.meta.url)
      ).json()) as { translation: Record<string, string> }
      expect(locale.translation['Target user']).toBeTruthy()
      expect(locale.translation['Target user ID']).toBeTruthy()
      expect(
        locale.translation[
          'Each option includes option ID, name, target user ID, whether source description is required, and sort order. After the JSON parses successfully, it is synced to the visual editor.'
        ]
      ).toBeTruthy()
      if (localeFile !== 'en.json') {
        expect(locale.translation['Target user']).not.toBe('Target user')
        expect(locale.translation['Target user ID']).not.toBe('Target user ID')
      }
    }
  })
})
