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
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import type { AuthenticatedGateFailure } from '@/features/auth/lib/authenticated-gates'

import {
  extractGateRateLimit,
  extractGateRequestId,
} from './gate-load-error-utils'

type GateLoadErrorProps = {
  gate: AuthenticatedGateFailure
  error?: unknown
  onRetry: () => void
}

export function GateLoadError(props: GateLoadErrorProps) {
  const { t } = useTranslation()
  const requestId = extractGateRequestId(props.error)
  const rateLimit = extractGateRateLimit(props.error)
  const [retryAfterSeconds, setRetryAfterSeconds] = useState(
    rateLimit?.retryAfterSeconds ?? 0
  )
  const gateName =
    props.gate === 'compliance'
      ? t('Compliance Statement')
      : t('Source Attribution')

  useEffect(() => {
    setRetryAfterSeconds(rateLimit?.retryAfterSeconds ?? 0)
  }, [props.error, rateLimit?.retryAfterSeconds])

  useEffect(() => {
    if (!rateLimit || retryAfterSeconds <= 0) return
    const timer = window.setTimeout(() => {
      setRetryAfterSeconds((current) => Math.max(0, current - 1))
    }, 1000)
    return () => window.clearTimeout(timer)
  }, [rateLimit, retryAfterSeconds])

  return (
    <div
      className='flex min-h-svh w-full flex-col items-center justify-center gap-4 px-6 text-center'
      role='alert'
    >
      <AlertTriangle className='text-destructive size-8' aria-hidden='true' />
      <div className='space-y-1'>
        <h1 className='text-lg font-semibold'>
          {rateLimit ? t('Too many requests') : t('Loading failed')}
        </h1>
        <p className='text-muted-foreground text-sm'>
          {rateLimit
            ? t('Please retry after {{seconds}} seconds.', {
                seconds: retryAfterSeconds,
              })
            : gateName}
        </p>
        {requestId ? (
          <p className='text-muted-foreground font-mono text-xs break-all'>
            {t('Request ID')}: {requestId}
          </p>
        ) : null}
      </div>
      <Button
        type='button'
        onClick={props.onRetry}
        disabled={Boolean(rateLimit && retryAfterSeconds > 0)}
      >
        <RefreshCw className='size-4' aria-hidden='true' />
        {t('Retry')}
      </Button>
    </div>
  )
}
