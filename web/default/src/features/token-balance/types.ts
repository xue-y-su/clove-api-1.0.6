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
export type QuotaAmount = {
  unlimited: boolean
  value: number
}

export type TokenBalanceFunding = {
  available: QuotaAmount
  used: number
  total: QuotaAmount
}

export type TokenBalanceWalletFunding = {
  available: QuotaAmount
}

export type TokenBalanceCalculation = {
  token_unlimited: boolean
  token_total: number
  token_remaining: number
  token_used: number
  current_available: QuotaAmount
  token_callable_quota?: QuotaAmount
  restricted: number
  restriction_reason?: string
  single_request_limit: QuotaAmount
  participating_sources: Array<'wallet' | 'subscription'>
  wallet?: TokenBalanceWalletFunding
  subscription?: TokenBalanceFunding
}

export type TokenBalanceGroup = {
  mode: 'fixed' | 'user' | 'auto'
  name?: string
  effective_groups: string[]
}

export type TokenBalanceModelLimits = {
  enabled: boolean
  count: number
  models?: string[]
}

export type TokenBalanceDisplay = {
  type: 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM'
  quota_per_unit: number
  exchange_rate: number
  symbol: string
}

export type TokenBalanceData = {
  masked_key: string
  status: 'active' | 'disabled' | 'expired' | 'exhausted' | 'user_disabled'
  expires_at: number
  billing_preference:
    | 'subscription_first'
    | 'wallet_first'
    | 'subscription_only'
    | 'wallet_only'
  group: TokenBalanceGroup
  model_limits: TokenBalanceModelLimits
  balance: TokenBalanceCalculation
  display: TokenBalanceDisplay
}

export type TokenBalanceResponse = {
  success: boolean
  code?: string
  message?: string
  retry_after_seconds?: number
  data?: TokenBalanceData
}
