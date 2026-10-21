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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'

import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

import { getPricing } from '../api'
import { MODEL_CATALOG_VISIBILITY_REFRESH_KEY } from '../constants'
import {
  getModelDisplayGroups,
  normalizePricingGroups,
} from '../lib/model-helpers'

export function usePricingData() {
  const { status } = useStatus()
  const queryClient = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id ?? 'anonymous')
  const userGroup = useAuthStore(
    (state) => state.auth.user?.group ?? 'anonymous'
  )

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['pricing', userId, userGroup],
    queryFn: getPricing,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: 'always',
  })

  useEffect(() => {
    const refreshAfterVisibilityChange = (event: StorageEvent) => {
      if (event.key !== MODEL_CATALOG_VISIBILITY_REFRESH_KEY) return
      void queryClient.invalidateQueries({ queryKey: ['pricing'] })
    }

    window.addEventListener('storage', refreshAfterVisibilityChange)
    return () =>
      window.removeEventListener('storage', refreshAfterVisibilityChange)
  }, [queryClient])

  // Ensure rates never reach zero to prevent division errors
  const priceRate = useMemo(
    () => Math.max((status?.price as number) ?? 1, 0.001),
    [status?.price]
  )
  const usdExchangeRate = useMemo(
    () => Math.max((status?.usd_exchange_rate as number) ?? priceRate, 0.001),
    [status?.usd_exchange_rate, priceRate]
  )

  const pricingGroups = useMemo(
    () =>
      normalizePricingGroups(data?.all_group ?? {}, data?.usable_group ?? {}),
    [data?.all_group, data?.usable_group]
  )

  const usableGroup = useMemo(
    () =>
      Object.fromEntries(
        Object.entries(data?.usable_group ?? {}).map(([name, description]) => [
          name,
          {
            desc: description,
            ratio: data?.group_ratio?.[name] ?? 1,
          },
        ])
      ),
    [data?.group_ratio, data?.usable_group]
  )

  const models = useMemo(() => {
    if (!data?.data || !data?.vendors) return []

    const vendorMap = new Map(data.vendors.map((v) => [v.id, v]))

    return data.data.map((model) => {
      const vendor = model.vendor_id
        ? vendorMap.get(model.vendor_id)
        : undefined
      const displayGroups = getModelDisplayGroups(
        model.enable_groups,
        pricingGroups
      )
      return {
        ...model,
        enable_groups: displayGroups.map((group) => group.name),
        display_groups: displayGroups,
        key: model.model_name,
        vendor_name: vendor?.name,
        vendor_icon: vendor?.icon,
        vendor_description: vendor?.description,
        group_ratio: data.group_ratio,
      }
    })
  }, [data, pricingGroups])

  return {
    models,
    vendors: data?.vendors ?? [],
    groupRatio: data?.group_ratio ?? {},
    usableGroup,
    groups: pricingGroups,
    endpointMap: data?.supported_endpoint ?? {},
    autoGroups: data?.auto_groups ?? [],
    pricingVersion: data?.pricing_version ?? '',
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  }
}
