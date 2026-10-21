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
import type { TokenBalanceData } from '../types'

export type BalanceQueryErrorTone = 'destructive' | 'warning' | 'default'

export type BalanceQueryErrorPresentation = {
  titleKey: string
  descriptionKey: string
  tone: BalanceQueryErrorTone
  values?: Record<string, number>
}

export function getBalanceQueryErrorPresentation(
  code: string,
  retryAfterSeconds?: number
): BalanceQueryErrorPresentation {
  if (code === 'invalid_request') {
    return {
      titleKey: 'Enter API Key',
      descriptionKey: 'Enter the complete API Key to query its balance.',
      tone: 'destructive',
    }
  }
  if (code === 'invalid_api_key') {
    return {
      titleKey: 'Balance Query Failed',
      descriptionKey: 'The API Key is invalid. Check it and try again.',
      tone: 'destructive',
    }
  }
  if (code === 'rate_limited') {
    if (Number.isFinite(retryAfterSeconds) && Number(retryAfterSeconds) > 0) {
      return {
        titleKey: 'Too Many Requests',
        descriptionKey: 'Try again in {{seconds}} seconds.',
        tone: 'warning',
        values: { seconds: Number(retryAfterSeconds) },
      }
    }
    return {
      titleKey: 'Too Many Requests',
      descriptionKey: 'Too many requests, please try again later.',
      tone: 'warning',
    }
  }
  return {
    titleKey: 'Unable to Query Balance',
    descriptionKey:
      'The balance service is temporarily unavailable. Try again later.',
    tone: 'default',
  }
}

const statusExplanationKeys: Partial<
  Record<TokenBalanceData['status'], string>
> = {
  disabled: 'This API Key has been disabled.',
  expired: 'This API Key has expired.',
  exhausted: 'This API Key has no remaining quota.',
  user_disabled: 'The account for this API Key has been disabled.',
}

export function getBalanceStatusExplanationKey(
  status: TokenBalanceData['status']
): string | null {
  return statusExplanationKeys[status] ?? null
}
