/*
Copyright (C) 2025 QuantumNous

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
import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const source = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');

const dataSource = source(
  '../../../hooks/model-pricing/useModelPricingData.jsx',
);
const groupsSource = source('./filter/PricingGroups.jsx');
const cardSource = source('./view/card/PricingCardView.jsx');
const columnsSource = source('./view/table/PricingTableColumns.jsx');
const detailsSource = source('./modal/components/ModelPricingTable.jsx');
const mobileFiltersSource = source('./modal/components/FilterModalContent.jsx');

test('normalizes all_group and keeps locked filters selectable', () => {
  expect(dataSource).toContain('all_group');
  expect(dataSource).toContain('normalizePricingGroups');
  expect(groupsSource).toContain('pricingGroups');
  expect(groupsSource).toContain('LockKeyhole');
  expect(groupsSource).toContain("t('当前账户不可用')");
  expect(groupsSource).not.toMatch(/disabled=\{[^}]*locked/);
});

test('marks locked prices in classic cards and tables', () => {
  for (const componentSource of [cardSource, columnsSource]) {
    expect(componentSource).toContain('getCatalogGroupState');
    expect(componentSource).toContain('priceGroup.locked');
    expect(componentSource).toContain('LockKeyhole');
    expect(componentSource).toContain("t('当前账户不可用')");
  }
});

test('renders all model display groups in the detail price table', () => {
  expect(detailsSource).toContain('modelData.display_groups');
  expect(detailsSource).not.toContain('Object.keys(usableGroup');
  expect(detailsSource).toContain('group.usable');
  expect(detailsSource).toContain('LockKeyhole');
});

test('keeps mobile group filtering and price selection in sync', () => {
  expect(mobileFiltersSource).toContain(
    'setFilterGroup={sidebarProps.handleGroupClick}',
  );
});
