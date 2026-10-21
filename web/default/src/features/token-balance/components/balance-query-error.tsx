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
import { CircleAlert, Clock3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { cn } from '@/lib/utils'

import { getBalanceQueryErrorPresentation } from '../lib/presentation'

type BalanceQueryErrorProps = {
  code: string
  retryAfterSeconds?: number
}

export function BalanceQueryError({
  code,
  retryAfterSeconds,
}: BalanceQueryErrorProps) {
  const { t } = useTranslation()
  const presentation = getBalanceQueryErrorPresentation(code, retryAfterSeconds)
  const warning = presentation.tone === 'warning'
  const destructive = presentation.tone === 'destructive'
  const Icon = warning ? Clock3 : CircleAlert

  return (
    <Alert
      variant={destructive ? 'destructive' : 'default'}
      className={cn(
        warning &&
          'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-700/50 dark:bg-amber-950/30 dark:text-amber-300'
      )}
      aria-live='polite'
    >
      <Icon />
      <AlertTitle>{t(presentation.titleKey)}</AlertTitle>
      <AlertDescription>
        {t(presentation.descriptionKey, presentation.values)}
      </AlertDescription>
    </Alert>
  )
}
