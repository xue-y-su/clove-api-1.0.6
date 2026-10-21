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
import type { ToolSurchargeItem } from '../types'

export function formatCompactToolSurcharges(
  items: ToolSurchargeItem[] | undefined,
  formatQuota: (quota: number) => string
): string {
  if (!Array.isArray(items)) return ''

  return items
    .filter(
      (item) =>
        typeof item?.name === 'string' &&
        item.name.trim() !== '' &&
        Number.isFinite(item.count) &&
        item.count > 0 &&
        Number.isFinite(item.subtotal_quota)
    )
    .map(
      (item) =>
        `${item.name.trim()} x${item.count} (${formatQuota(item.subtotal_quota)})`
    )
    .join(' · ')
}
