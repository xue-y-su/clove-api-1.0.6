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

const EXCLUDED_GROUPS = new Set(['', 'auto']);

export function normalizePricingGroups(allGroup = {}, usableGroup = {}) {
  return Object.keys(allGroup || {})
    .filter((name) => !EXCLUDED_GROUPS.has(name))
    .sort((left, right) => left.localeCompare(right))
    .map((name) => ({
      name,
      description: allGroup[name] || name,
      usable: Object.hasOwn(usableGroup, name),
    }));
}

export function getModelDisplayGroups(enableGroups = [], pricingGroups = []) {
  const enabled = Array.isArray(enableGroups) ? enableGroups : [];
  if (enabled.includes('all')) return pricingGroups;
  const enabledNames = new Set(enabled);
  return pricingGroups.filter((group) => enabledNames.has(group.name));
}

export function getCatalogGroupState(model, group) {
  const matched = model?.display_groups?.find((item) => item.name === group);
  return {
    group: matched?.name ?? group ?? null,
    locked: matched ? !matched.usable : false,
  };
}
