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
import type { UserRedemptionKind } from './types'

const USER_REDEMPTION_EXPIRY_OPTIONS = [1, 24, 24 * 7, 24 * 30]

export function getUserRedemptionExpiryOptions(
  maxExpiryHours = 24 * 30
): number[] {
  const ceiling = Number.isFinite(maxExpiryHours)
    ? Math.min(24 * 30, Math.max(1, Math.trunc(maxExpiryHours)))
    : 24 * 30
  return [
    ...new Set([
      ...USER_REDEMPTION_EXPIRY_OPTIONS.filter((hours) => hours <= ceiling),
      ceiling,
    ]),
  ].sort((left, right) => left - right)
}

export function getUserRedemptionExpiryTranslation(hours: number): {
  key: string
  values: { count: number }
} {
  if (hours === 1) return { key: '1 hour', values: { count: 1 } }
  if (hours % 24 === 0) {
    return { key: '{{count}} days', values: { count: hours / 24 } }
  }
  return { key: '{{count}} hours', values: { count: hours } }
}

export function calculateSubscriptionRedemptionCost(
  priceAmount: number,
  units: number,
  quotaPerUnit: number
): { unitQuota: number; totalQuota: number } {
  const normalizedPrice =
    Number.isFinite(priceAmount) && priceAmount > 0 ? priceAmount : 0
  const normalizedUnits =
    Number.isFinite(units) && units > 0 ? Math.trunc(units) : 0
  const normalizedQuotaPerUnit =
    Number.isFinite(quotaPerUnit) && quotaPerUnit > 0 ? quotaPerUnit : 0
  const unitQuota = Math.ceil(normalizedPrice * normalizedQuotaPerUnit)
  return {
    unitQuota,
    totalQuota: unitQuota * normalizedUnits,
  }
}

function getRedemptionClaimCount(
  kind: UserRedemptionKind,
  maxClaims: number
): number {
  if (kind !== 'random') return 1
  if (!Number.isFinite(maxClaims)) return 2
  return Math.min(100, Math.max(2, Math.trunc(maxClaims)))
}

export function getMinimumRedemptionAmount(
  kind: UserRedemptionKind,
  maxClaims: number,
  minimumUnitAmount = 1
): number {
  const unitAmount =
    Number.isFinite(minimumUnitAmount) && minimumUnitAmount > 0
      ? minimumUnitAmount
      : 1
  return sanitizeRedemptionAmountInput(
    unitAmount * getRedemptionClaimCount(kind, maxClaims)
  )
}

export function sanitizeRedemptionAmountInput(amount: number): number {
  if (!Number.isFinite(amount)) return amount
  return Math.round((amount + Number.EPSILON) * 100) / 100
}

export function normalizeRedemptionAmount(
  kind: UserRedemptionKind,
  amount: number,
  maxClaims: number,
  minimumUnitAmount = 1,
  maxClaimsPerCode = 100
): number {
  const claimLimit = Number.isFinite(maxClaimsPerCode)
    ? Math.min(100, Math.max(2, Math.trunc(maxClaimsPerCode)))
    : 100
  const minimum = getMinimumRedemptionAmount(
    kind,
    Math.min(maxClaims, claimLimit),
    minimumUnitAmount
  )
  const sanitized = sanitizeRedemptionAmountInput(amount)
  return Number.isFinite(sanitized) ? Math.max(sanitized, minimum) : minimum
}

export function getRedemptionAmountValidation(
  kind: UserRedemptionKind,
  amount: number,
  maxClaims: number,
  minimumUnitAmount = 1
): { key: string; values: Record<string, number> } | null {
  const minimumAmount = getMinimumRedemptionAmount(
    kind,
    maxClaims,
    minimumUnitAmount
  )
  const sanitized = sanitizeRedemptionAmountInput(amount)
  if (Number.isFinite(sanitized) && sanitized >= minimumAmount) return null
  if (kind === 'random') {
    return {
      key: 'Red packet total must be at least {{amount}} for {{count}} recipients.',
      values: {
        amount: minimumAmount,
        count: getRedemptionClaimCount(kind, maxClaims),
      },
    }
  }
  return {
    key: 'Amount must be at least {{amount}}.',
    values: { amount: minimumAmount },
  }
}
