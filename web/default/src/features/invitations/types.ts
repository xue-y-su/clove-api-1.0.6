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
export type InvitationApplicationStatus =
  | 'pending'
  | 'withdrawn'
  | 'approved'
  | 'rejected'

export type InvitationCodeStatus =
  | 'active'
  | 'exhausted'
  | 'expired'
  | 'revoked'

export type InvitationCodeKind = 'single' | 'multi'

export type InvitationRewardPayoutRequestStatus =
  | 'pending'
  | 'paid'
  | 'rejected'
  | 'frozen'
  | 'cancelled'

export interface InvitationApplication {
  id: number
  user_id: number
  reason: string
  status: InvitationApplicationStatus
  reviewer_id: number
  review_reason: string
  reviewed_at: number
  created_at: number
  updated_at: number
}

export interface InvitationCode {
  id: number
  owner_user_id: number
  code: string
  kind: InvitationCodeKind
  max_uses: number
  used_count: number
  status: InvitationCodeStatus
  expires_at: number
  issued_by: number
  application_id?: number
  created_at: number
  updated_at: number
  revoked_at?: number
}

export interface InvitationUse {
  id: number
  invite_code_id: number
  inviter_user_id: number
  registered_user_id: number
  registered_at: number
  source: string
  registration_ip: string
  inviter_reward: number
  invitee_reward: number
  inviter_reward_status: string
  invitee_reward_status: string
  reward_attempt_count: number
  reward_next_attempt_at: number
  rewards_completed_at: number
  invite_code?: string
  inviter_username?: string
  inviter_role?: number
  registered_username?: string
}

export interface InvitationUserState {
  application: InvitationUserApplication | null
  code: InvitationUserCode | null
  uses: InvitationUserUse[]
  can_apply: boolean
  reapply_at?: number
}

export interface InvitationUserApplication {
  id: number
  reason: string
  status: InvitationApplicationStatus
  review_reason: string
  reviewed_at: number
  created_at: number
  updated_at: number
}

export interface InvitationUserCode {
  id: number
  code: string
  kind: InvitationCodeKind
  max_uses: number
  used_count: number
  status: InvitationCodeStatus
  expires_at: number
  created_at: number
  updated_at: number
}

export interface InvitationUserUse {
  id: number
  registered_user_id: number
  registered_at: number
  source: string
  inviter_reward: number
  inviter_reward_status: string
  rewards_completed_at: number
}

export interface InvitationApplicationListItem extends InvitationApplication {
  applicant_username: string
  applicant_role: number
  reviewer_username: string
}

export interface InvitationCodeListItem extends InvitationCode {
  owner_username: string
  owner_role: number
  issuer_username: string
}

export interface InvitationListResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ApiResponse<T> {
  success: boolean
  message: string
  data: T
}

export interface InvitationListQuery {
  page: number
  pageSize: number
  status?: string
  keyword?: string
}

export interface InvitationIssueInput {
  kind: InvitationCodeKind
  max_uses: number
  expires_at: number
}

export interface InvitationCodeValidation {
  valid: boolean
  code: 'invitation_available' | 'invitation_unavailable'
  message: string
}

export interface InvitationRewardSummary {
  invited_count: number
  total_earned: number
  available: number
  pending_payout: number
  paid: number
  frozen: number
  cash_commission_visible: boolean
  cash_commission_rate: string
}

export interface InvitationRewardPayoutRequest {
  id: number
  operation_id: string
  user_id: number
  amount: number
  status: InvitationRewardPayoutRequestStatus
  reason: string
  reviewer_id: number
  freeze_revision: number
  reviewed_at: number
  created_at: number
  updated_at: number
  username: string
  display_name: string
  user_role: number
  reviewer_username: string
  item_count: number
  reversed_amount: number
  net_amount: number
}

export interface InvitationRewardPayoutSource {
  reward_id: number
  invite_code_use_id: number
  trigger_type: string
  trigger_id: string
  eligible_quota: number
  rate_basis_points: number
  amount: number
  reversed_amount: number
  net_amount: number
  reversal_state: string
  status: string
  source_user_id: number
  source_username: string
  source_display_name: string
  registration_ip: string
  registered_at: number
  invite_code: string
}

export interface InvitationRewardPayoutAction {
  id: number
  request_id: number
  action: 'freeze' | 'unfreeze'
  revision: number
  actor_id: number
  reason: string
  amount: number
  wallet_operation_id: string
  created_at: number
  actor_username: string
  actor_display_name: string
}

export interface InvitationRewardPayoutRequestDetail {
  request: InvitationRewardPayoutRequest
  rewards: InvitationRewardPayoutSource[]
  actions: InvitationRewardPayoutAction[]
}
