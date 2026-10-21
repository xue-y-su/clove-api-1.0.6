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

import { formatBalanceQuota, getBalanceMetricValues } from '../lib/balance'
import type { TokenBalanceCalculation, TokenBalanceDisplay } from '../types'

type BalanceMetricsProps = {
  balance: TokenBalanceCalculation
  display: TokenBalanceDisplay
}

type MetricProps = {
  dotClassName: string
  label: string
  value: string
  description: string
  destructive?: boolean
}

function Metric({
  dotClassName,
  label,
  value,
  description,
  destructive = false,
}: MetricProps) {
  return (
    <div className='min-w-0 px-4 py-4 sm:px-5'>
      <div className='text-muted-foreground flex items-center gap-2 text-xs font-medium uppercase'>
        <span className={`size-1.5 shrink-0 rounded-[2px] ${dotClassName}`} />
        <span className='min-w-0 break-words'>{label}</span>
      </div>
      <div
        className={`mt-1.5 font-mono text-xl font-bold break-all tabular-nums sm:text-2xl ${destructive ? 'text-destructive' : 'text-foreground'}`}
        title={value}
      >
        {value}
      </div>
      <div className='text-muted-foreground/65 mt-1 text-xs'>{description}</div>
    </div>
  )
}

export function BalanceMetrics({ balance, display }: BalanceMetricsProps) {
  const { t } = useTranslation()
  const unlimited = t('Unlimited')
  const metrics = getBalanceMetricValues(balance)
  const formatAmount = (value: number) =>
    formatBalanceQuota({ unlimited: false, value }, display, unlimited)

  return (
    <div className='divide-border/60 grid grid-cols-1 divide-y border-b sm:grid-cols-3 sm:divide-x sm:divide-y-0'>
      <Metric
        dotClassName='bg-foreground'
        label={t('Token Callable Quota')}
        value={formatBalanceQuota(metrics.available, display, unlimited)}
        description={t(
          'Available to this API Key now after wallet, subscription, and key limits'
        )}
      />
      <Metric
        dotClassName='bg-muted-foreground/50'
        label={t('Used Quota')}
        value={formatAmount(metrics.used)}
        description={t('Total consumed by this API Key')}
      />
      <Metric
        dotClassName={
          metrics.third.kind === 'restricted'
            ? 'bg-destructive'
            : 'border border-muted-foreground bg-transparent rounded-full'
        }
        label={
          metrics.third.kind === 'restricted'
            ? t('Unavailable Quota')
            : t('Key Limit')
        }
        value={
          metrics.third.kind === 'restricted'
            ? formatAmount(metrics.third.value)
            : unlimited
        }
        description={
          metrics.third.kind === 'restricted'
            ? t('Restricted by available funding')
            : t('No independent quota limit')
        }
        destructive={
          metrics.third.kind === 'restricted' && metrics.third.value > 0
        }
      />
    </div>
  )
}
