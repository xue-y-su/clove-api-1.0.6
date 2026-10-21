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

import {
  buildCatalogVisibilityDraft,
  clearCatalogRule,
  filterCatalogGroups,
  getCatalogRule,
  getEffectiveUserGroupNames,
  getMissingUserGroups,
  getRestrictedCatalogGroupCount,
  getVisibleUserGroupCount,
  isCatalogRuleExplicit,
  setCatalogDefaultMode,
  setCatalogVisibilityEnabled,
  setCatalogRuleMode,
  setCatalogRuleUserGroup,
  toVisibilityUpdate,
} from './model-catalog-visibility-state'

describe('model catalog visibility editor state', () => {
  const state = {
    config: {
      version: 3 as const,
      enabled: true,
      default_mode: 'all' as const,
      rules: {
        removed: { mode: 'selected' as const, user_groups: ['ghost'] },
        vip: { mode: 'selected' as const, user_groups: ['Admin', 'test'] },
      },
    },
    etag: 'etag-1',
    groups: [
      { name: 'vip', description: 'VIP', exists: true },
      { name: 'test', description: 'Test', exists: true },
      { name: 'removed', description: '', exists: false },
    ],
    user_groups: [
      { name: 'test', description: 'Test users', exists: true },
      { name: 'Admin', description: 'Admin users', exists: true },
      { name: 'ghost', description: '', exists: false },
    ],
    summary: {
      group_count: 3,
      model_count: 2,
      restricted_group_count: 2,
      pricing_version: 'v1',
    },
  }

  test('keeps independent per-catalog rules and stale entries visible', () => {
    const draft = buildCatalogVisibilityDraft(state)
    expect(draft.enabled).toBe(true)
    expect(getCatalogRule(draft, 'vip')).toEqual({
      mode: 'selected',
      userGroups: ['Admin', 'test'],
    })
    expect(getCatalogRule(draft, 'test')).toEqual({
      mode: 'all',
      userGroups: [],
    })
    expect(draft.groups.at(-1)).toMatchObject({
      name: 'removed',
      exists: false,
    })
    expect(getMissingUserGroups(draft, 'removed')).toEqual(['ghost'])
  })

  test('defaults selected mode to the same-named user group when it exists', () => {
    let draft = buildCatalogVisibilityDraft(state)
    draft = setCatalogRuleMode(draft, 'test', 'selected')
    expect(getCatalogRule(draft, 'test')).toEqual({
      mode: 'selected',
      userGroups: ['test'],
    })
  })

  test('updates one catalog group without changing another rule', () => {
    let draft = buildCatalogVisibilityDraft(state)
    draft = setCatalogRuleUserGroup(draft, 'vip', 'Admin', false)
    draft = setCatalogRuleUserGroup(draft, 'vip', 'ghost', true)
    expect(getCatalogRule(draft, 'vip').userGroups).toEqual(['ghost', 'test'])
    expect(getCatalogRule(draft, 'removed').userGroups).toEqual(['ghost'])
    expect(getMissingUserGroups(draft, 'vip')).toEqual(['ghost'])
  })

  test('searches catalog identifiers and emits the v3 atomic request', () => {
    let draft = buildCatalogVisibilityDraft(state)
    draft = clearCatalogRule(draft, 'removed')
    expect(isCatalogRuleExplicit(draft, 'removed')).toBe(false)
    expect(filterCatalogGroups(draft.groups, 'vip')).toHaveLength(1)
    expect(filterCatalogGroups(draft.groups, 'Test')).toHaveLength(1)
    expect(toVisibilityUpdate(draft)).toEqual({
      config: {
        version: 3,
        enabled: true,
        default_mode: 'all',
        rules: {
          vip: { mode: 'selected', user_groups: ['Admin', 'test'] },
        },
      },
      expected_etag: 'etag-1',
    })
  })

  test('keeps rules in the request while visibility control is disabled', () => {
    const draft = setCatalogVisibilityEnabled(
      buildCatalogVisibilityDraft(state),
      false
    )
    expect(getRestrictedCatalogGroupCount(draft)).toBe(0)
    expect(toVisibilityUpdate(draft).config).toMatchObject({
      version: 3,
      enabled: false,
      rules: {
        removed: { mode: 'selected', user_groups: ['ghost'] },
        vip: { mode: 'selected', user_groups: ['Admin', 'test'] },
      },
    })
  })

  test('derives the restricted count from the current unsaved draft', () => {
    let draft = buildCatalogVisibilityDraft({
      ...state,
      summary: { ...state.summary, restricted_group_count: 0 },
    })
    expect(getRestrictedCatalogGroupCount(draft)).toBe(2)
    draft = clearCatalogRule(draft, 'removed')
    expect(getRestrictedCatalogGroupCount(draft)).toBe(1)
  })

  test('unchecking one cell from all keeps every other current user group', () => {
    let draft = buildCatalogVisibilityDraft(state)
    draft = setCatalogRuleMode(draft, 'vip', 'all')
    draft = setCatalogRuleUserGroup(draft, 'vip', 'Admin', false)

    expect(getCatalogRule(draft, 'vip')).toEqual({
      mode: 'selected',
      userGroups: ['test'],
    })
    expect(getEffectiveUserGroupNames(draft, 'vip')).toEqual(['test'])
  })

  test('counts only existing user groups represented by the matrix', () => {
    const draft = buildCatalogVisibilityDraft(state)

    expect(getVisibleUserGroupCount(draft, 'vip')).toBe(2)
    expect(getVisibleUserGroupCount(draft, 'removed')).toBe(0)
    expect(getVisibleUserGroupCount(draft, 'test')).toBe(2)
  })

  test('changes the default rule without replacing explicit row rules', () => {
    const draft = setCatalogDefaultMode(
      buildCatalogVisibilityDraft(state),
      'hidden'
    )

    expect(draft.defaultMode).toBe('hidden')
    expect(getCatalogRule(draft, 'vip')).toEqual({
      mode: 'selected',
      userGroups: ['Admin', 'test'],
    })
    expect(getCatalogRule(draft, 'test')).toEqual({
      mode: 'selected',
      userGroups: [],
    })
  })
})
