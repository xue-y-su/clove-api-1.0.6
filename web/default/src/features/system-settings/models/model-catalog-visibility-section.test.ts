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
import { expect, test } from 'bun:test'

const section = await Bun.file(
  new URL('./model-catalog-visibility-section.tsx', import.meta.url)
).text()
const api = await Bun.file(new URL('../api.ts', import.meta.url)).text()
const registry = await Bun.file(
  new URL('./section-registry.tsx', import.meta.url)
).text()
const pricingHook = await Bun.file(
  new URL('../../pricing/hooks/use-pricing-data.ts', import.meta.url)
).text()

test('wires the dedicated atomic visibility endpoint', () => {
  expect(api).toContain("'/api/option/model-catalog-visibility'")
  expect(section).toContain('updateModelCatalogVisibility')
  expect(section).toContain('getModelCatalogVisibility')
  expect(section).toContain('status === 409')
  expect(section).toContain('break-words')
  expect(registry).toContain("id: 'catalog-visibility'")
  expect(section).toContain('for (let attempt = 0; attempt < 3; attempt += 1)')
  expect(section).toContain('setTimeout(resolve, 100 * (attempt + 1))')
})

test('uses an explicit user-group by model-group visibility matrix', () => {
  expect(section).toContain('CatalogVisibilityMatrix')
  expect(section).toContain("t('All user groups')")
  expect(section).toContain("t('Inherit default rule')")
  expect(section).toContain('onUserGroupChange')
  expect(section).toContain('setCatalogVisibilityEnabled')
  expect(section).toContain('setCatalogDefaultMode')
  expect(section).toContain("t('User group')")
  expect(section).toContain("t('Model groups visible to each user group')")
  expect(section).toContain(
    'Rows are user groups and columns are model groups.'
  )
  expect(section).toContain("t('Enable custom model catalog visibility rules')")
  expect(section).toContain("t('Using original model catalog rules')")
  expect(section).toContain("t('Use platform original rules')")
  expect(section).toContain("t('Use custom visibility rules')")
  expect(section).toContain("'Visible to {{userGroup}} for {{catalogGroup}}'")
  expect(section).toContain('sticky left-0')
  expect(section).not.toContain('Collapsible')
  expect(section).toContain('description === group.name')
  expect(section).not.toContain('model_name')
})

test('refreshes model catalog data after visibility changes', () => {
  expect(section).toContain("invalidateQueries({ queryKey: ['pricing'] })")
  expect(section).toContain('MODEL_CATALOG_VISIBILITY_REFRESH_KEY')
  expect(pricingHook).toContain("refetchOnWindowFocus: 'always'")
  expect(pricingHook).toContain("window.addEventListener('storage'")
  expect(pricingHook).toContain('MODEL_CATALOG_VISIBILITY_REFRESH_KEY')
})
