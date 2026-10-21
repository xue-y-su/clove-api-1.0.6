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
import { describe, expect, test } from 'bun:test';

import {
  getCatalogGroupState,
  getModelDisplayGroups,
  normalizePricingGroups,
} from './model-visibility';

describe('classic model catalog visibility', () => {
  test('marks groups outside usable_group as locked', () => {
    expect(
      normalizePricingGroups(
        { default: 'Default', enterprise: 'Enterprise' },
        { default: 'Default' },
      ),
    ).toEqual([
      { name: 'default', description: 'Default', usable: true },
      { name: 'enterprise', description: 'Enterprise', usable: false },
    ]);
  });

  test('never reconstructs a hidden catalog group from usable_group', () => {
    expect(
      normalizePricingGroups(
        { default: 'Default' },
        { default: 'Default', hidden: 'Hidden' },
      ),
    ).toEqual([{ name: 'default', description: 'Default', usable: true }]);
  });

  test('expands all without intersecting the model groups with usable groups', () => {
    const groups = normalizePricingGroups(
      { default: 'Default', enterprise: 'Enterprise' },
      { default: 'Default' },
    );

    expect(getModelDisplayGroups(['all'], groups)).toEqual(groups);
    expect(getModelDisplayGroups(['default', 'enterprise'], groups)).toEqual(
      groups,
    );
  });

  test('reports the minimum-price group as locked when it is unavailable', () => {
    const model = {
      enable_groups: ['default', 'enterprise'],
      display_groups: [
        { name: 'default', description: 'Default', usable: true },
        {
          name: 'enterprise',
          description: 'Enterprise',
          usable: false,
        },
      ],
    };

    expect(getCatalogGroupState(model, 'enterprise')).toEqual({
      group: 'enterprise',
      locked: true,
    });
  });
});
