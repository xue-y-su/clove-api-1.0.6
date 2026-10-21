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
import type {
  PlaygroundWebSearchCapability,
  SystemStatus,
} from '@/features/auth/types'

const UNAVAILABLE_CAPABILITY: PlaygroundWebSearchCapability = {
  available: false,
  native: false,
  tavily: false,
  reason: 'not_configured',
}

function isCapability(value: unknown): value is PlaygroundWebSearchCapability {
  if (!value || typeof value !== 'object') return false
  const capability = value as Record<string, unknown>
  return (
    typeof capability.available === 'boolean' &&
    typeof capability.native === 'boolean' &&
    typeof capability.tavily === 'boolean' &&
    typeof capability.reason === 'string'
  )
}

export function getPlaygroundWebSearchCapability(
  status: SystemStatus | null
): PlaygroundWebSearchCapability {
  const nested = status?.data?.playground_web_search
  if (isCapability(nested)) return nested

  const direct = status?.playground_web_search
  if (isCapability(direct)) return direct

  return UNAVAILABLE_CAPABILITY
}
