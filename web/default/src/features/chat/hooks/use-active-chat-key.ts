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
import { useEffect, useState } from 'react'

import { getApiKeys } from '@/features/keys/api'
import { useTokenReveal } from '@/features/keys/components/token-reveal-provider'
import { API_KEY_STATUS } from '@/features/keys/constants'
import { useAuthStore } from '@/stores/auth-store'

export async function fetchActiveChatKey(
  resolveKey: (id: number) => Promise<string>
) {
  const result = await getApiKeys({ p: 1, size: 50 })
  if (!result.success) {
    throw new Error(result.message || 'Failed to load API keys')
  }

  const items = result.data?.items ?? []
  const active = items.find((item) => item.status === API_KEY_STATUS.ENABLED)
  if (!active) {
    throw new Error('No enabled API keys found. Create or enable one first.')
  }

  return resolveKey(active.id)
}

/**
 * Get the currently active API key for chat links
 */
export function useActiveChatKey(enabled: boolean) {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const { resolveKey } = useTokenReveal()
  const [data, setData] = useState<string>()
  const [error, setError] = useState<Error | null>(null)
  const [isPending, setIsPending] = useState(false)

  useEffect(() => {
    let active = true
    setData(undefined)
    setError(null)
    if (!enabled || !userId) {
      setIsPending(false)
      return () => {
        active = false
      }
    }
    setIsPending(true)
    void fetchActiveChatKey(resolveKey)
      .then((key) => {
        if (active) setData(key)
      })
      .catch((reason: unknown) => {
        if (active) {
          setError(reason instanceof Error ? reason : new Error(String(reason)))
        }
      })
      .finally(() => {
        if (active) setIsPending(false)
      })
    return () => {
      active = false
      setData(undefined)
    }
  }, [enabled, resolveKey, userId])

  return { data, error, isPending, isError: error !== null }
}
