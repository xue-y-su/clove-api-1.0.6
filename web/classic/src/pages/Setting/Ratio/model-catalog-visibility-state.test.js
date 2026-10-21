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
  buildCatalogVisibilityDraft,
  clearCatalogRule,
  filterCatalogGroups,
  getCatalogRule,
  setCatalogRuleMode,
  setCatalogRuleUserGroup,
  setCatalogVisibilityEnabled,
  toVisibilityUpdate,
} from './model-catalog-visibility-state';

describe('classic catalog visibility state', () => {
  const state = {
    config: {
      version: 3,
      enabled: true,
      default_mode: 'all',
      rules: {
        vip: { mode: 'selected', user_groups: ['default'] },
        removed: { mode: 'selected', user_groups: ['removed-user'] },
      },
    },
    etag: 'etag-1',
    groups: [
      { name: 'vip', description: 'VIP', exists: true },
      { name: 'default', description: 'Default', exists: true },
      { name: 'removed', description: '', exists: false },
    ],
    user_groups: [
      { name: 'default', description: 'Default', exists: true },
      { name: 'admin', description: 'Admin', exists: true },
      { name: 'removed-user', description: '', exists: false },
    ],
    summary: {
      group_count: 3,
      model_count: 2,
      restricted_group_count: 2,
      pricing_version: 'v3',
    },
  };

  test('matches the versioned backend update contract', () => {
    let draft = buildCatalogVisibilityDraft(state);
    draft = setCatalogRuleUserGroup(draft, 'vip', 'admin', true);
    expect(toVisibilityUpdate(draft)).toEqual({
      config: {
        version: 3,
        enabled: true,
        default_mode: 'all',
        rules: {
          removed: { mode: 'selected', user_groups: ['removed-user'] },
          vip: { mode: 'selected', user_groups: ['admin', 'default'] },
        },
      },
      expected_etag: 'etag-1',
    });
  });

  test('preserves stale groups and resolves independent rules', () => {
    let draft = buildCatalogVisibilityDraft(state);
    expect(draft.groups.at(-1)).toMatchObject({
      name: 'removed',
      exists: false,
    });
    expect(getCatalogRule(draft, 'vip')).toEqual({
      mode: 'selected',
      userGroups: ['default'],
    });
    expect(getCatalogRule(draft, 'default')).toEqual({
      mode: 'all',
      userGroups: [],
    });
    draft = setCatalogRuleMode(draft, 'default', 'selected');
    expect(getCatalogRule(draft, 'default').userGroups).toEqual(['default']);
    draft = clearCatalogRule(draft, 'default');
    expect(getCatalogRule(draft, 'default').mode).toBe('all');
    expect(filterCatalogGroups(draft.groups, 'removed')).toHaveLength(1);
  });

  test('disabling custom visibility preserves saved v3 rules', () => {
    const draft = setCatalogVisibilityEnabled(
      buildCatalogVisibilityDraft(state),
      false,
    );

    expect(toVisibilityUpdate(draft).config).toMatchObject({
      version: 3,
      enabled: false,
      rules: {
        vip: { mode: 'selected', user_groups: ['default'] },
      },
    });
  });
});
