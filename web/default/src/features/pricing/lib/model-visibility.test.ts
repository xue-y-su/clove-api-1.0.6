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

import type { PricingGroup, PricingModel } from '../types'
import * as modelHelpers from './model-helpers'

const { getAvailableGroups } = modelHelpers
const visibilityHelpers = modelHelpers as typeof modelHelpers & {
  normalizePricingGroups?: (
    allGroup: Record<string, string>,
    usableGroup: Record<string, string>
  ) => PricingGroup[]
  getModelDisplayGroups?: (
    enableGroups: string[],
    groups: PricingGroup[]
  ) => PricingGroup[]
  resolveCatalogGroup?: (
    model: PricingModel,
    groups: PricingGroup[],
    selectedGroup?: string
  ) => { group: string | null; ratio: number; locked: boolean }
  buildPricingGroupOptions?: (
    groups: PricingGroup[]
  ) => Array<{ value: string; label: string; locked: boolean }>
}

function model(enableGroups: string[]): PricingModel {
  return {
    id: 1,
    model_name: 'catalog-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: enableGroups,
  }
}

describe('model catalog visibility', () => {
  test('does not intersect model groups with the users usable groups', () => {
    expect(
      getAvailableGroups(model(['default', 'enterprise']), {
        default: { desc: 'Default', ratio: 1 },
        enterprise: { desc: 'Enterprise', ratio: 2 },
      })
    ).toEqual(['default', 'enterprise'])
  })

  test('expands the all marker against every catalog group', () => {
    expect(
      getAvailableGroups(model(['all']), {
        default: { desc: 'Default', ratio: 1 },
        vip: { desc: 'VIP', ratio: 0.5 },
      })
    ).toEqual(['default', 'vip'])
  })

  test('normalizes full groups and marks groups outside usable_group as locked', () => {
    const normalize = visibilityHelpers.normalizePricingGroups
    expect(typeof normalize).toBe('function')
    if (!normalize) return

    expect(
      normalize(
        { enterprise: 'Enterprise', default: 'Default' },
        { default: 'Default' }
      )
    ).toEqual([
      { name: 'default', description: 'Default', usable: true },
      { name: 'enterprise', description: 'Enterprise', usable: false },
    ])
  })

  test('never reconstructs a hidden catalog group from usable_group', () => {
    const normalize = visibilityHelpers.normalizePricingGroups
    expect(typeof normalize).toBe('function')
    if (!normalize) return

    expect(
      normalize(
        { default: 'Default' },
        { default: 'Default', hidden: 'Hidden' }
      )
    ).toEqual([{ name: 'default', description: 'Default', usable: true }])
  })

  test('marks the cheapest all-group price as locked when unavailable', () => {
    const normalize = visibilityHelpers.normalizePricingGroups
    const expand = visibilityHelpers.getModelDisplayGroups
    const resolve = visibilityHelpers.resolveCatalogGroup
    expect(typeof normalize).toBe('function')
    expect(typeof expand).toBe('function')
    expect(typeof resolve).toBe('function')
    if (!normalize || !expand || !resolve) return

    const groups = normalize(
      { default: 'Default', vip: 'VIP' },
      { default: 'Default' }
    )
    expect(expand(['all'], groups)).toEqual(groups)
    expect(
      resolve(
        {
          ...model(['all']),
          group_ratio: { default: 1, vip: 0.5 },
        },
        groups
      )
    ).toEqual({ group: 'vip', ratio: 0.5, locked: true })
  })

  test('keeps unavailable groups selectable and marks their filter options as locked', () => {
    const buildOptions = visibilityHelpers.buildPricingGroupOptions
    expect(typeof buildOptions).toBe('function')
    if (!buildOptions) return

    expect(
      buildOptions([
        { name: 'default', description: 'Default', usable: true },
        {
          name: 'enterprise',
          description: 'Enterprise',
          usable: false,
        },
      ])
    ).toEqual([
      { value: 'default', label: 'Default', locked: false },
      { value: 'enterprise', label: 'Enterprise', locked: true },
    ])
  })
})
