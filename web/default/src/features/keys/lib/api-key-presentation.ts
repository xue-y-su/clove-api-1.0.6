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
import type { StatusBadgeProps } from '@/components/status-badge'

import { API_KEY_STATUSES, API_KEY_STATUS_REASON } from '../constants'
import type { ApiKey } from '../types'

export type ApiKeyStatusPresentation = {
  label: string
  variant: StatusBadgeProps['variant']
  compromised: boolean
}

export function getApiKeyStatusPresentation(
  apiKey: Pick<ApiKey, 'status' | 'status_reason'>
): ApiKeyStatusPresentation | null {
  if (apiKey.status_reason === API_KEY_STATUS_REASON.COMPROMISED) {
    return {
      label: 'Leaked',
      variant: 'danger',
      compromised: true,
    }
  }

  const status = API_KEY_STATUSES[apiKey.status]
  if (!status) return null

  return {
    label: status.label,
    variant: status.variant,
    compromised: false,
  }
}
