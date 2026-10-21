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
import { Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampToDate } from '@/lib/format'

import { formatBalanceQuota, getTokenCallableQuota } from '../lib/balance'
import type { QuotaAmount, TokenBalanceData } from '../types'

type BalanceRestrictionsProps = {
  data: TokenBalanceData
}

const preferenceKeys = {
  subscription_first: 'Subscription First',
  wallet_first: 'Wallet First',
  subscription_only: 'Subscription Only',
  wallet_only: 'Wallet Only',
} as const

function RestrictionRow({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className='grid min-w-0 grid-cols-[minmax(7rem,0.7fr)_minmax(0,1.3fr)] gap-3 border-b py-3 last:border-b-0'>
      <dt className='text-muted-foreground text-sm'>{label}</dt>
      <dd className='min-w-0 text-right text-sm font-medium break-words'>
        {children}
      </dd>
    </div>
  )
}

export function BalanceRestrictions({ data }: BalanceRestrictionsProps) {
  const { t } = useTranslation()
  const unlimited = t('Unlimited')
  const format = (value: QuotaAmount) =>
    formatBalanceQuota(value, data.display, unlimited)
  const tokenCallableQuota = getTokenCallableQuota(data.balance)

  const groupValue =
    data.group.mode === 'auto'
      ? `${t('Automatic Group')}: ${data.group.effective_groups.join(', ') || '-'}`
      : data.group.name || data.group.effective_groups[0] || '-'

  const modelValue = data.model_limits.enabled
    ? t('{{count}} models only', { count: data.model_limits.count })
    : t('No Model Restriction')

  const restrictionKeys: Record<string, string> = {
    subscription_quota: 'Restricted by Subscription Quota',
    wallet_quota: 'Restricted by Account Balance',
    funding_quota: 'Restricted by Available Funding',
    key_unavailable: 'Key Unavailable',
  }

  return (
    <div className='grid grid-cols-1 divide-y sm:grid-cols-2 sm:divide-x sm:divide-y-0'>
      <section className='min-w-0 px-4 py-4 sm:px-5'>
        <h2 className='text-sm font-semibold'>{t('Quota Breakdown')}</h2>
        <dl className='mt-2'>
          {data.balance.token_unlimited ? (
            <RestrictionRow label={t('Key Limit')}>{unlimited}</RestrictionRow>
          ) : (
            <RestrictionRow label={t('Remaining quota')}>
              {format({
                unlimited: false,
                value: data.balance.token_remaining,
              })}
            </RestrictionRow>
          )}
          {data.balance.wallet ? (
            <RestrictionRow label={t('Wallet Spendable Balance')}>
              {format(data.balance.wallet.available)}
            </RestrictionRow>
          ) : null}
          {data.balance.subscription ? (
            <RestrictionRow label={t('Subscription Remaining Quota')}>
              {format(data.balance.subscription.available)}
            </RestrictionRow>
          ) : null}
          <RestrictionRow label={t('Token Callable Quota')}>
            {format(tokenCallableQuota)}
          </RestrictionRow>
          {data.balance.restriction_reason ? (
            <RestrictionRow label={t('Restriction Reason')}>
              <span className='text-destructive'>
                {t(
                  restrictionKeys[data.balance.restriction_reason] ||
                    'Restricted Quota'
                )}
              </span>
            </RestrictionRow>
          ) : null}
        </dl>
      </section>

      <section className='min-w-0 px-4 py-4 sm:px-5'>
        <h2 className='text-sm font-semibold'>{t('Key Configuration')}</h2>
        <dl className='mt-2'>
          <RestrictionRow label={t('Billing Source')}>
            {t(preferenceKeys[data.billing_preference])}
          </RestrictionRow>
          <RestrictionRow label={t('Group Restriction')}>
            {groupValue}
          </RestrictionRow>
          <RestrictionRow label={t('Model Restriction')}>
            <span className='inline-flex max-w-full items-center justify-end gap-1.5'>
              <span>{modelValue}</span>
              {data.model_limits.enabled &&
              (data.model_limits.models?.length ?? 0) > 0 ? (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <button
                        type='button'
                        className='text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex size-5 shrink-0 items-center justify-center rounded-sm focus-visible:ring-2 focus-visible:outline-none'
                        aria-label={t('View Model Restrictions')}
                      />
                    }
                  >
                    <Info className='size-3.5' />
                  </TooltipTrigger>
                  <TooltipContent className='max-w-72 break-words'>
                    {data.model_limits.models?.join(', ')}
                  </TooltipContent>
                </Tooltip>
              ) : null}
            </span>
          </RestrictionRow>
          <RestrictionRow label={t('Single Request Limit')}>
            {format(data.balance.single_request_limit)}
          </RestrictionRow>
          <RestrictionRow label={t('Expiration Time')}>
            {data.expires_at > 0
              ? formatTimestampToDate(data.expires_at)
              : t('Never expires')}
          </RestrictionRow>
        </dl>
      </section>
    </div>
  )
}
