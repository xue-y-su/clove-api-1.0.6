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
import { CircleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'

import { getBalanceStatusExplanationKey } from '../lib/presentation'
import type { TokenBalanceData } from '../types'
import { BalanceMetrics } from './balance-metrics'
import { BalanceProgress } from './balance-progress'
import { BalanceRestrictions } from './balance-restrictions'

const statusKeys = {
  active: 'Active',
  disabled: 'Disabled',
  expired: 'Expired',
  exhausted: 'Exhausted',
  user_disabled: 'User Disabled',
} as const

const groupModeKeys = {
  fixed: 'Key Assigned',
  user: 'User Group',
  auto: 'Automatic',
} as const

export function BalanceResult({ data }: { data: TokenBalanceData }) {
  const { t } = useTranslation()
  const active = data.status === 'active'
  const groupTitle =
    data.group.mode === 'auto'
      ? t('Automatic Group')
      : data.group.name || data.group.effective_groups[0] || '-'
  const statusExplanationKey = getBalanceStatusExplanationKey(data.status)

  return (
    <Card aria-live='polite' data-card-hover='false' className='gap-0 py-0'>
      <header className='flex min-w-0 flex-wrap items-center gap-2 border-b px-4 py-4'>
        <span className='text-muted-foreground min-w-0 truncate font-mono text-xs'>
          {data.masked_key}
        </span>
        <Badge variant='outline' className='max-w-full break-all'>
          {groupTitle}
        </Badge>
        <Badge variant='outline'>{t(groupModeKeys[data.group.mode])}</Badge>
        <Badge
          variant={active ? 'outline' : 'destructive'}
          className={
            active
              ? 'ml-auto border-emerald-600/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
              : 'ml-auto'
          }
        >
          {t(statusKeys[data.status])}
        </Badge>
      </header>

      <BalanceMetrics balance={data.balance} display={data.display} />
      <BalanceProgress balance={data.balance} display={data.display} />
      <BalanceRestrictions data={data} />

      {statusExplanationKey ? (
        <div className='border-t p-4'>
          <Alert variant='destructive'>
            <CircleAlert />
            <AlertTitle>{t(statusKeys[data.status])}</AlertTitle>
            <AlertDescription>{t(statusExplanationKey)}</AlertDescription>
          </Alert>
        </div>
      ) : null}
    </Card>
  )
}
