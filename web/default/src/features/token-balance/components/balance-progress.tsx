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
import { useTranslation } from 'react-i18next'

import {
  formatBalanceQuota,
  getBalanceProgressSegments,
  getTokenCallableQuota,
} from '../lib/balance'
import type {
  QuotaAmount,
  TokenBalanceCalculation,
  TokenBalanceDisplay,
} from '../types'

type BalanceProgressProps = {
  balance: TokenBalanceCalculation
  display: TokenBalanceDisplay
}

export function BalanceProgress({ balance, display }: BalanceProgressProps) {
  const { t } = useTranslation()
  const segments = getBalanceProgressSegments(balance)
  const callable = getTokenCallableQuota(balance)
  const unlimited = t('Unlimited')
  const format = (amount: QuotaAmount) =>
    formatBalanceQuota(amount, display, unlimited)
  const formatValue = (value: number) => format({ unlimited: false, value })
  const restrictionLabels: Record<string, string> = {
    subscription_quota: 'Restricted by Subscription Quota',
    wallet_quota: 'Restricted by Account Balance',
    funding_quota: 'Restricted by Available Funding',
    key_unavailable: 'Key Unavailable',
  }
  let rightLabel = `${t('Unavailable Quota')} ${formatValue(0)}`
  if (balance.token_unlimited) {
    rightLabel = `${t('Used Quota')} ${formatValue(balance.token_used)}`
  } else if (balance.restricted > 0) {
    rightLabel = `${formatValue(balance.restricted)} ${t(
      restrictionLabels[balance.restriction_reason || ''] || 'Restricted Quota'
    )}`
  }

  return (
    <section className='border-b px-4 py-4 sm:px-5'>
      <div
        className='bg-muted ring-border flex h-2.5 w-full overflow-hidden rounded-sm ring-1'
        role='img'
        aria-label={t('Quota usage progress')}
      >
        {segments.available > 0 ? (
          <div
            className='bg-foreground h-full'
            style={{ width: `${segments.available}%` }}
          />
        ) : null}
        {segments.used > 0 ? (
          <div
            className='bg-muted-foreground/45 border-background h-full border-l'
            style={{ width: `${segments.used}%` }}
          />
        ) : null}
        {segments.restricted > 0 ? (
          <div
            className='bg-destructive border-background h-full border-l'
            style={{ width: `${segments.restricted}%` }}
          />
        ) : null}
      </div>

      <div className='text-muted-foreground mt-2 flex flex-col justify-between gap-1 text-xs sm:flex-row sm:items-start'>
        <span>
          {balance.token_unlimited
            ? `${t('Token Callable Quota')} ${format(callable)}`
            : `${t('Key Total Limit')} ${formatValue(balance.token_total)}`}
        </span>
        <span
          className={
            balance.restricted > 0 ? 'text-destructive font-medium' : ''
          }
        >
          {rightLabel}
        </span>
      </div>
    </section>
  )
}
