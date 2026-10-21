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
export type {
  UserRedemptionClaim,
  UserRedemptionCode,
  UserRedemptionDetail,
  UserRedemptionList,
  UserRedemptionListParams,
} from './api'

export type UserRedemptionKind = 'single' | 'random' | 'fixed'
export type UserRedemptionRewardType = 'balance' | 'subscription'
export type UserRedemptionStatus =
  | 'active'
  | 'disabled'
  | 'exhausted'
  | 'expired_pending'
  | 'refund_pending'
  | 'refunded'
  | 'forfeited'
  | 'deleted'

export const USER_REDEMPTION_KIND_LABELS: Record<UserRedemptionKind, string> = {
  single: 'Single',
  random: 'Red packet',
  fixed: 'Fixed units',
}

export const USER_REDEMPTION_REWARD_LABELS: Record<
  UserRedemptionRewardType,
  string
> = {
  balance: 'Balance',
  subscription: 'Subscription',
}

export const USER_REDEMPTION_STATUS_LABELS: Record<
  UserRedemptionStatus,
  string
> = {
  active: 'Active',
  disabled: 'Disabled',
  exhausted: 'Exhausted',
  expired_pending: 'Pending settlement',
  refund_pending: 'Refund pending',
  refunded: 'Refunded',
  forfeited: 'Forfeited',
  deleted: 'Deleted',
}

export function canDeleteUserRedemption(item: { deleted: boolean }): boolean {
  return !item.deleted
}

export function canSettleUserRedemption(item: {
  deleted: boolean
  reward_type: UserRedemptionRewardType
  status: UserRedemptionStatus
}): boolean {
  return (
    !item.deleted &&
    item.reward_type === 'balance' &&
    (item.status === 'active' ||
      item.status === 'disabled' ||
      item.status === 'expired_pending')
  )
}
