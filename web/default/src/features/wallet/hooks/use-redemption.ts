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
import i18next from 'i18next'
import { useState, useCallback } from 'react'
import { toast } from 'sonner'

import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'

import { claimRedemptionCode } from '../api'
import type { RedemptionClaimResult } from '../types'

// ============================================================================
// Redemption Hook
// ============================================================================

export function useRedemption() {
  const [redeeming, setRedeeming] = useState(false)

  const redeemCode = useCallback(
    async (code: string): Promise<RedemptionClaimResult | null> => {
      if (!code || code.trim() === '') {
        toast.error(i18next.t('Please enter a redemption code'))
        return null
      }

      try {
        setRedeeming(true)
        const response = await claimRedemptionCode(code.trim())

        if (response.success && response.data) {
          const result = response.data
          if (result.reward_type === 'subscription') {
            toast.success(i18next.t('Subscription redeemed successfully'))
          } else {
            toast.success(
              i18next.t('Redemption successful! Added: {{quota}}', {
                quota: formatQuota(result.quota),
              })
            )
          }
          await getSelf()
          return result
        }

        toast.error(response.message || i18next.t('Redemption failed'))
        return null
      } catch (caught: unknown) {
        toast.error(
          caught instanceof Error && caught.message
            ? caught.message
            : i18next.t('Redemption failed')
        )
        return null
      } finally {
        setRedeeming(false)
      }
    },
    []
  )

  return {
    redeeming,
    redeemCode,
  }
}
