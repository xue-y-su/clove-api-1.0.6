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
import { useQuery } from '@tanstack/react-query'

import { getUserGroups } from '@/lib/api'

import type { ApiKeyGroupOption } from '../components/api-key-group-combobox'

const USER_REFERENCE_STALE_TIME_MS = 10_000

export function useApiKeyGroups(): { options: ApiKeyGroupOption[] } {
  const { data } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: USER_REFERENCE_STALE_TIME_MS,
    select: (res) => {
      if (!res.success || !res.data) {
        return { options: [] }
      }
      const options: ApiKeyGroupOption[] = []
      for (const [group, info] of Object.entries(res.data)) {
        options.push({
          value: group,
          label: group,
          desc: info.desc,
          ratio: info.ratio,
        })
      }
      return { options }
    },
  })

  return data ?? { options: [] }
}
