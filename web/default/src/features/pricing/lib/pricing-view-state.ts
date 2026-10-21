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
export type PricingViewState = 'loading' | 'error' | 'empty' | 'content'

interface PricingViewStateInput {
  isLoading: boolean
  hasError: boolean
  modelCount: number
}

export function getPricingViewState(
  input: PricingViewStateInput
): PricingViewState {
  if (input.isLoading) return 'loading'
  if (input.hasError) return 'error'
  if (input.modelCount === 0) return 'empty'
  return 'content'
}

export function reconcilePricingGroup(
  selectedGroup: string,
  groups: Array<{ name: string }>
) {
  if (selectedGroup === 'all') return selectedGroup
  return groups.some((group) => group.name === selectedGroup)
    ? selectedGroup
    : 'all'
}
