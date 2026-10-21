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

import type {
  ApiResponse,
  InvitationApplication,
  InvitationApplicationListItem,
  InvitationCode,
  InvitationCodeValidation,
  InvitationCodeListItem,
  InvitationIssueInput,
  InvitationListQuery,
  InvitationListResult,
  InvitationRewardPayoutRequest,
  InvitationRewardPayoutRequestDetail,
  InvitationRewardSummary,
  InvitationUse,
  InvitationUserState,
} from './types'

export async function validateInvitationCode(
  code: string,
  signal?: AbortSignal
): Promise<InvitationCodeValidation> {
  return mutateInvitation('/api/invitation/validate', { code }, signal)
}

export async function getInvitationSelf(): Promise<InvitationUserState> {
  const response = await api.get('/api/invitation/self')
  return responseData(response.data)
}

export async function getInvitationRewardSummary(): Promise<InvitationRewardSummary> {
  const response = await api.get('/api/invitation/rewards')
  return responseData(response.data)
}

export async function submitInvitationRewardPayout(): Promise<InvitationRewardPayoutRequest> {
  return mutateInvitation('/api/invitation/rewards/payout-requests', undefined)
}

export async function applyForInvitation(
  reason: string
): Promise<InvitationApplication> {
  return mutateInvitation('/api/invitation/applications', { reason })
}

export async function withdrawInvitationApplication(
  applicationId: number
): Promise<null> {
  return mutateInvitation(
    `/api/invitation/applications/${applicationId}/withdraw`,
    undefined
  )
}

export async function listInvitationApplications(
  query: InvitationListQuery
): Promise<InvitationListResult<InvitationApplicationListItem>> {
  const response = await api.get('/api/invitation/admin/applications', {
    params: invitationListParams(query),
  })
  return responseData(response.data)
}

export async function reviewInvitationApplication(
  applicationId: number,
  input: InvitationIssueInput & {
    approve: boolean
    review_reason: string
  }
): Promise<InvitationCode | null> {
  return mutateInvitation(
    `/api/invitation/admin/applications/${applicationId}/review`,
    input
  )
}

export async function listInvitationCodes(
  query: InvitationListQuery
): Promise<InvitationListResult<InvitationCodeListItem>> {
  const response = await api.get('/api/invitation/admin/codes', {
    params: invitationListParams(query),
  })
  return responseData(response.data)
}

export async function issueInvitationCode(
  input: InvitationIssueInput & { target_user_id: number; reason: string }
): Promise<InvitationCode> {
  return mutateInvitation('/api/invitation/admin/codes', input)
}

export async function revokeInvitationCode(
  codeId: number,
  reason: string
): Promise<null> {
  return mutateInvitation(`/api/invitation/admin/codes/${codeId}/revoke`, {
    reason,
  })
}

export async function revokeOwnInvitationCode(
  codeId: number,
  reason: string
): Promise<null> {
  return mutateInvitation(`/api/invitation/codes/${codeId}/revoke`, { reason })
}

export async function listInvitationUses(
  query: InvitationListQuery
): Promise<InvitationListResult<InvitationUse>> {
  const response = await api.get('/api/invitation/admin/uses', {
    params: invitationListParams(query),
  })
  return responseData(response.data)
}

export async function listInvitationRewardPayoutRequests(
  query: InvitationListQuery
): Promise<InvitationListResult<InvitationRewardPayoutRequest>> {
  const response = await api.get(
    '/api/invitation/admin/reward-payout-requests',
    {
      params: invitationListParams(query),
    }
  )
  return responseData(response.data)
}

export async function reviewInvitationRewardPayoutRequest(
  requestId: number,
  input: { approve: boolean; review_reason: string }
): Promise<null> {
  return mutateInvitation(
    `/api/invitation/admin/reward-payout-requests/${requestId}/review`,
    input
  )
}

export async function getInvitationRewardPayoutRequestDetail(
  requestId: number
): Promise<InvitationRewardPayoutRequestDetail> {
  const response = await api.get(
    `/api/invitation/admin/reward-payout-requests/${requestId}`
  )
  return responseData(response.data)
}

export async function freezeInvitationRewardPayoutRequest(
  requestId: number,
  reason: string
): Promise<null> {
  return mutateInvitation(
    `/api/invitation/admin/reward-payout-requests/${requestId}/freeze`,
    { reason }
  )
}

export async function unfreezeInvitationRewardPayoutRequest(
  requestId: number,
  reason: string
): Promise<null> {
  return mutateInvitation(
    `/api/invitation/admin/reward-payout-requests/${requestId}/unfreeze`,
    { reason }
  )
}

function responseData<T>(response: ApiResponse<T>): T {
  if (!response.success) {
    throw new Error(response.message || 'Request failed')
  }
  return response.data
}

async function mutateInvitation<T>(
  url: string,
  data: unknown,
  signal?: AbortSignal
): Promise<T> {
  try {
    const response = await api.post<ApiResponse<T>>(url, data, {
      ...(signal ? { signal } : {}),
      skipBusinessError: true,
      skipErrorHandler: true,
    })
    return responseData(response.data)
  } catch (error) {
    const response = (error as { response?: { data?: { message?: unknown } } })
      ?.response?.data
    if (typeof response?.message === 'string' && response.message.trim()) {
      throw new Error(response.message)
    }
    throw error
  }
}

function invitationListParams(query: InvitationListQuery) {
  return {
    page: query.page,
    page_size: query.pageSize,
    status: query.status || undefined,
    keyword: query.keyword?.trim() || undefined,
  }
}
