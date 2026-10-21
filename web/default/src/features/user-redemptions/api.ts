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
import { api } from '@/lib/api'

export type UserRedemptionCode = {
  id: number
  code: string
  creator_user_id: number
  creator_username?: string
  reward_type: 'balance' | 'subscription'
  kind: 'single' | 'random' | 'fixed'
  plan_id?: number
  plan_title?: string
  plan_subtitle?: string
  currency?: string
  price_amount?: number
  unit_paid_quota?: number
  total_quota: number
  claimed_quota: number
  refunded_quota: number
  forfeited_quota: number
  remaining_quota: number
  max_claims: number
  claim_count: number
  status:
    | 'active'
    | 'disabled'
    | 'exhausted'
    | 'expired_pending'
    | 'refund_pending'
    | 'refunded'
    | 'forfeited'
    | 'deleted'
  expires_at: number
  disabled_at: number
  disabled_by?: number
  disabled_reason?: string
  terminal_at: number
  terminal_actor_id?: number
  terminal_reason?: string
  deleted: boolean
  deleted_at: number
  deleted_by?: number
  delete_reason?: string
  created_at: number
  updated_at: number
}

export type UserRedemptionClaim = {
  id: number
  reward_type: 'balance' | 'subscription'
  quota: number
  user_subscription_id?: number
  claimed_at: number
  claimant_display?: string
  claimant_user_id?: number
  claimant_username?: string
}

export type UserRedemptionBatchDispositionResult = {
  id: number
  success: boolean
  code?: 'not_found' | 'inactive' | 'conflict' | 'invalid' | 'failed'
  message?: string
}

export type UserRedemptionDetail = {
  code: UserRedemptionCode
  claims: UserRedemptionClaim[]
}

type UserRedemptionDetailPayload = Omit<UserRedemptionDetail, 'claims'> & {
  claims?: UserRedemptionClaim[] | null
}

export function normalizeUserRedemptionDetail(
  detail: UserRedemptionDetailPayload
): UserRedemptionDetail {
  return {
    ...detail,
    claims: Array.isArray(detail.claims) ? detail.claims : [],
  }
}

export type UserRedemptionList = {
  items: UserRedemptionCode[]
  total: number
  page: number
  page_size: number
}

export type UserRedemptionListParams = {
  page?: number
  page_size?: number
  status?: string
  reward_type?: 'balance' | 'subscription' | ''
  creator_user_id?: number
  creator_username?: string
  kind?: 'single' | 'random' | 'fixed' | ''
  deleted?: 'false' | 'true' | 'all'
  created_from?: number
  created_to?: number
  expires_from?: number
  expires_to?: number
}

export function getUserRedemptionListQueryKey(admin: boolean, page: number) {
  return ['user-redemptions', admin ? 'admin' : 'mine', 'list', page] as const
}

export function getUserRedemptionDetailQueryKey(
  admin: boolean,
  id: number | null
) {
  return ['user-redemptions', admin ? 'admin' : 'mine', 'detail', id] as const
}

export async function disableAdminUserRedemption(id: number, reason: string) {
  const { data } = await api.post(`/api/user-redemption/admin/${id}/disable`, {
    reason,
  })
  return data.data as UserRedemptionCode
}

export async function enableAdminUserRedemption(id: number, reason: string) {
  const { data } = await api.post(`/api/user-redemption/admin/${id}/enable`, {
    reason,
  })
  return data.data as UserRedemptionCode
}

export async function terminateAdminUserRedemption(
  id: number,
  settlement: 'refund' | 'forfeit',
  reason: string
) {
  const { data } = await api.post(
    `/api/user-redemption/admin/${id}/terminate`,
    { settlement, reason }
  )
  return data.data as UserRedemptionCode
}

export async function deleteAdminUserRedemption(
  id: number,
  settlement: 'refund' | 'forfeit' | '',
  reason: string
) {
  const { data } = await api.post(`/api/user-redemption/admin/${id}/delete`, {
    settlement,
    reason,
  })
  return data.data as UserRedemptionCode
}

export async function batchSettleAdminUserRedemptions(input: {
  ids: number[]
  disposition: 'refund' | 'forfeit'
  reason: string
}) {
  const { data } = await api.post(
    '/api/user-redemption/admin/batch-settlement',
    input
  )
  return data.data as UserRedemptionBatchDispositionResult[]
}

export async function listUserRedemptions(
  options: UserRedemptionListParams & { admin?: boolean } = {}
) {
  const endpoint = options.admin
    ? '/api/user-redemption/admin/'
    : '/api/user-redemption/'
  const params = {
    page: options.page,
    page_size: options.page_size,
    status: options.status,
    reward_type: options.reward_type,
    creator_user_id: options.creator_user_id,
    creator_username: options.creator_username,
    kind: options.kind,
    deleted: options.deleted,
    created_from: options.created_from,
    created_to: options.created_to,
    expires_from: options.expires_from,
    expires_to: options.expires_to,
  }
  const { data } = await api.get(endpoint, { params })
  return data.data as UserRedemptionList
}

export async function listMyUserRedemptions(params?: UserRedemptionListParams) {
  return listUserRedemptions(params)
}

export type CreateUserRedemptionInput =
  | {
      rewardType: 'balance'
      kind: 'single' | 'random'
      totalQuota: string
      maxClaims: number
      expiresAt: number
    }
  | {
      rewardType: 'subscription'
      planId: number
      units: number
      expiresAt: number
    }

export type CreateUserRedemptionRequest =
  | {
      reward_type: 'balance'
      kind: 'single' | 'random'
      total_quota: string
      max_claims: number
      expires_at: number
    }
  | {
      reward_type: 'subscription'
      plan_id: number
      units: number
      expires_at: number
    }

export function buildCreateUserRedemptionRequest(
  input: CreateUserRedemptionInput
): CreateUserRedemptionRequest {
  if (input.rewardType === 'subscription') {
    return {
      reward_type: 'subscription',
      plan_id: input.planId,
      units: input.units,
      expires_at: input.expiresAt,
    }
  }
  return {
    reward_type: 'balance',
    kind: input.kind,
    total_quota: input.totalQuota,
    max_claims: input.maxClaims,
    expires_at: input.expiresAt,
  }
}

export async function createUserRedemption(body: CreateUserRedemptionRequest) {
  const { data } = await api.post('/api/user-redemption/', body, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  if (!data?.success) {
    throw new Error(data?.message || 'Request failed')
  }
  return data.data as UserRedemptionCode
}

export async function getUserRedemption(id: number, admin = false) {
  const endpoint = admin
    ? `/api/user-redemption/admin/${id}`
    : `/api/user-redemption/${id}`
  const { data } = await api.get(endpoint)
  return normalizeUserRedemptionDetail(data.data as UserRedemptionDetailPayload)
}

export async function claimUserRedemption(
  code: string,
  expectedRewardType?: 'balance' | 'subscription'
) {
  const { data } = await api.post(
    '/api/user-redemption/claim',
    {
      code,
      expected_reward_type: expectedRewardType,
    },
    { skipBusinessError: true, skipErrorHandler: true }
  )
  if (!data?.success) {
    throw new Error(data?.message || 'Request failed')
  }
  return data.data
}

export async function cancelUserRedemption(
  id: number,
  options?: {
    admin?: boolean
    settlement?: 'refund' | 'forfeit'
    reason?: string
  }
) {
  const endpoint = options?.admin
    ? `/api/user-redemption/admin/${id}/cancel`
    : `/api/user-redemption/${id}/cancel`
  if (options?.admin) {
    const reason = options.reason?.trim() || ''
    if (!options.settlement || !reason) {
      throw new Error(
        'Administrator cancellation requires settlement and reason'
      )
    }
    const { data } = await api.post(endpoint, {
      settlement: options.settlement,
      reason,
    })
    return data.data as UserRedemptionCode
  }
  const { data } = await api.post(endpoint)
  return data.data as UserRedemptionCode
}
