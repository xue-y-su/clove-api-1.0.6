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

describe('model dialog translations', () => {
  test('translates sync results and local fallbacks', () => {
    const source = readFileSync(
      new URL('./sync-wizard-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain("t('Failed to preview upstream diff')")
    expect(source).toContain(
      "t('Found {{count}} conflicts. Please resolve them first.'"
    )
    expect(source).toContain(
      "'Sync completed! Created {{created}} models, updated {{updated}}, and added {{vendors}} vendors.'"
    )
    expect(source).toContain("t('Sync failed')")
    expect(source).toContain("label={t('Default')}")
  })

  test('translates conflict table labels, values, counts, and selection controls', () => {
    const source = readFileSync(
      new URL('./upstream-conflict-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(source).toContain("header: t('Model')")
    expect(source).toContain("header: t('Diff')")
    expect(source).toContain("header: t('Field')")
    expect(source).toContain("header: t('Local Value')")
    expect(source).toContain("header: t('Upstream Value')")
    expect(source).toContain("aria-label={t('Select all')}")
    expect(source).toContain("aria-label={t('Select row')}")
    expect(source).toContain("t('View diff')")
    expect(source).toContain("label={t('Local')}")
    expect(source).toContain("label={t('Upstream')}")
    expect(source).toContain("t('{{count}} model with conflicts'")
    expect(source).toContain("t('{{count}} models with conflicts'")
    expect(source).toContain("t('{{count}} field showing'")
    expect(source).toContain("t('{{count}} fields showing'")
    expect(source).toContain("t('{{count}} selected'")
    expect(source).toContain("t('Showing {{start}}-{{end}} of {{count}} field'")
    expect(source).toContain(
      "t('Showing {{start}}-{{end}} of {{count}} fields'"
    )
  })
})
