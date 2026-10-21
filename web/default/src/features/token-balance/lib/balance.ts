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
import type {
  QuotaAmount,
  TokenBalanceCalculation,
  TokenBalanceDisplay,
} from '../types'

export type BalanceProgressSegments = {
  available: number
  used: number
  restricted: number
}

export type BalanceMetricValues = {
  available: QuotaAmount
  used: number
  third: {
    kind: 'restricted' | 'unlimited'
    value: number
  }
}

function safeQuota(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0
}

function toPercent(value: number, total: number): number {
  if (total <= 0) return 0
  return Math.min(100, Math.max(0, (value / total) * 100))
}

export function getTokenCallableQuota(
  balance: TokenBalanceCalculation
): QuotaAmount {
  return balance.token_callable_quota ?? balance.current_available
}

export function getBalanceProgressSegments(
  balance: TokenBalanceCalculation
): BalanceProgressSegments {
  const callable = getTokenCallableQuota(balance)
  if (callable.unlimited) {
    return { available: 100, used: 0, restricted: 0 }
  }

  const available = safeQuota(callable.value)
  const used = safeQuota(balance.token_used)
  const restricted = balance.token_unlimited ? 0 : safeQuota(balance.restricted)
  const total = balance.token_unlimited
    ? available + used
    : safeQuota(balance.token_total)

  if (total <= 0) {
    return { available: 0, used: 0, restricted: 0 }
  }

  return {
    available: toPercent(available, total),
    used: toPercent(used, total),
    restricted: toPercent(restricted, total),
  }
}

export function getBalanceMetricValues(
  balance: TokenBalanceCalculation
): BalanceMetricValues {
  return {
    available: getTokenCallableQuota(balance),
    used: safeQuota(balance.token_used),
    third: balance.token_unlimited
      ? { kind: 'unlimited', value: 0 }
      : { kind: 'restricted', value: safeQuota(balance.restricted) },
  }
}

export function formatBalanceQuota(
  amount: QuotaAmount,
  display: TokenBalanceDisplay,
  unlimitedLabel: string,
  locale?: Intl.LocalesArgument
): string {
  if (amount.unlimited) return unlimitedLabel

  const value = safeQuota(amount.value)
  if (display.type === 'TOKENS') {
    return new Intl.NumberFormat(locale, {
      maximumFractionDigits: 0,
    }).format(value)
  }

  const quotaPerUnit =
    Number.isFinite(display.quota_per_unit) && display.quota_per_unit > 0
      ? display.quota_per_unit
      : 1
  const exchangeRate =
    Number.isFinite(display.exchange_rate) && display.exchange_rate > 0
      ? display.exchange_rate
      : 1
  const converted = (value / quotaPerUnit) * exchangeRate
  const digits = Math.abs(converted) < 1 && converted !== 0 ? 4 : 2
  const formatted = new Intl.NumberFormat(locale, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(converted)

  return `${display.symbol}${formatted}`
}
