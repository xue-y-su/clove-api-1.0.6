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
import { EXCLUDED_GROUPS, FILTER_ALL, QUOTA_TYPE_VALUES } from '../constants'
import type { PricingGroup, PricingModel } from '../types'

// ----------------------------------------------------------------------------
// Model Helper Utilities
// ----------------------------------------------------------------------------

/**
 * Get available groups for a model
 */
export function getAvailableGroups(
  model: PricingModel,
  catalogGroup: Record<string, unknown>
): string[] {
  const modelEnableGroups = Array.isArray(model.enable_groups)
    ? model.enable_groups
    : []

  if (modelEnableGroups.includes('all')) {
    return Object.keys(catalogGroup).filter(
      (group) => !EXCLUDED_GROUPS.includes(group)
    )
  }

  return modelEnableGroups.filter((group) => !EXCLUDED_GROUPS.includes(group))
}

export function isPricingGroupUsable(
  group: string,
  usableGroup: Record<string, string>
): boolean {
  return Object.hasOwn(usableGroup, group)
}

export function normalizePricingGroups(
  allGroup: Record<string, string>,
  usableGroup: Record<string, string>
): PricingGroup[] {
  return Object.keys(allGroup)
    .filter((name) => !EXCLUDED_GROUPS.includes(name))
    .sort((left, right) => left.localeCompare(right))
    .map((name) => ({
      name,
      description: allGroup[name] || name,
      usable: isPricingGroupUsable(name, usableGroup),
    }))
}

export function buildPricingGroupOptions(
  groups: PricingGroup[]
): Array<{ value: string; label: string; locked: boolean }> {
  return groups.map((group) => ({
    value: group.name,
    label: group.description || group.name,
    locked: !group.usable,
  }))
}

export function getModelDisplayGroups(
  enableGroups: string[],
  groups: PricingGroup[]
): PricingGroup[] {
  const normalizedEnableGroups = Array.isArray(enableGroups) ? enableGroups : []
  if (normalizedEnableGroups.includes('all')) return groups
  const enabled = new Set(normalizedEnableGroups)
  return groups.filter((group) => enabled.has(group.name))
}

export function resolveCatalogGroup(
  model: PricingModel,
  groups: PricingGroup[],
  selectedGroup: string = FILTER_ALL
): { group: string | null; ratio: number; locked: boolean } {
  const displayGroups = getModelDisplayGroups(model.enable_groups, groups)
  if (selectedGroup !== FILTER_ALL) {
    const selected = displayGroups.find((group) => group.name === selectedGroup)
    if (selected) {
      return {
        group: selected.name,
        ratio: getConfiguredGroupRatio(model.group_ratio || {}, selected.name),
        locked: !selected.usable,
      }
    }
  }

  let resolved: PricingGroup | null = null
  let minRatio = Number.POSITIVE_INFINITY
  for (const group of displayGroups) {
    const ratio = getConfiguredGroupRatio(model.group_ratio || {}, group.name)
    if (ratio < minRatio) {
      resolved = group
      minRatio = ratio
    }
  }

  return resolved
    ? { group: resolved.name, ratio: minRatio, locked: !resolved.usable }
    : { group: null, ratio: 1, locked: false }
}

/**
 * Read a configured group ratio while preserving valid zero ratios.
 */
export function getConfiguredGroupRatio(
  groupRatio: Record<string, number>,
  group: string
): number {
  const ratio = groupRatio[group]
  return typeof ratio === 'number' && Number.isFinite(ratio) ? ratio : 1
}

/**
 * Resolve the group ratio used by model square summary prices.
 *
 * When no specific group is selected, the model square shows the best price
 * available to the viewer. When a group filter is active, it mirrors classic
 * and shows that group's price.
 */
export function getDisplayGroupRatio(
  model: PricingModel,
  selectedGroup?: string
): number {
  const groupRatio = model.group_ratio || {}
  const displayGroups =
    model.display_groups ??
    (model.enable_groups.includes('all')
      ? Object.keys(groupRatio)
      : model.enable_groups
    ).map((name) => ({ name, description: name, usable: true }))
  return resolveCatalogGroup(model, displayGroups, selectedGroup).ratio
}

/**
 * Replace model placeholder in endpoint path
 */
export function replaceModelInPath(path: string, modelName: string): string {
  return path.replaceAll('{model}', modelName)
}

/**
 * Check if model is token-based pricing
 */
export function isTokenBasedModel(model: PricingModel): boolean {
  return model.quota_type === QUOTA_TYPE_VALUES.TOKEN
}
