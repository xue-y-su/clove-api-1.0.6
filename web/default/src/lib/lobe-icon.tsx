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
import type { ReactNode } from 'react'

import { LobeIcon } from './lobe-icon-component'

export function getLobeIcon(
  iconName: string | undefined | null,
  size: number = 20
): ReactNode {
  if (!iconName || typeof iconName !== 'string') {
    return <LobeIcon iconName='?' size={size} />
  }

  const trimmedName = iconName.trim()
  if (!trimmedName) {
    return <LobeIcon iconName='?' size={size} />
  }

  return <LobeIcon iconName={trimmedName} size={size} />
}
