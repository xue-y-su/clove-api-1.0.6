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
import { readFileSync } from 'node:fs'

function source(path: string): string {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const pageSource = source('../index.tsx')
const sidebarSource = source('./pricing-sidebar.tsx')
const cardSource = source('./model-card.tsx')
const columnsSource = source('./pricing-columns.tsx')
const priceLockSource = source('./price-lock-indicator.tsx')
const detailsSource = source('./model-details.tsx')

test('passes the complete pricing group catalog into desktop and mobile filters', () => {
  expect(pageSource).toContain('groups,')
  expect(pageSource).not.toContain('Object.keys(usableGroup')
  expect(sidebarSource).toContain('groups: PricingGroup[]')
  expect(sidebarSource).toContain('buildPricingGroupOptions')
  expect(sidebarSource).toContain('LockKeyhole')
  expect(sidebarSource).toContain("t('Unavailable to your account')")
  expect(sidebarSource).not.toMatch(/disabled=\{[^}]*locked/)
})

test('labels summary prices when their resolved group is unavailable', () => {
  for (const componentSource of [cardSource, columnsSource]) {
    expect(componentSource).toContain('resolveCatalogGroup')
    expect(componentSource).toContain('priceGroup.locked')
    expect(componentSource).toContain("t('Unavailable to your account')")
  }
  expect(cardSource).toContain('LockKeyhole')
  expect(columnsSource).toContain('PriceLockIndicator')
  expect(priceLockSource).toContain('LockKeyhole')
})

test('shows every model group and lock state in model details', () => {
  expect(detailsSource).toContain('model.display_groups')
  expect(detailsSource).toContain('group.usable')
  expect(detailsSource).toContain('LockKeyhole')
  expect(detailsSource).toContain("t('Unavailable to your account')")
})
