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
import { z } from 'zod'

import type { AdminPermissionMatrix } from '@/lib/admin-permissions'
import type { EditableInviteLimitMode } from '@/types/invite-limit'

// ============================================================================
// User Schema & Types
// ============================================================================

/** User status: 1 = enabled, 2 = disabled, 3+ = other states */
export const userStatusSchema = z.number()
export type UserStatus = z.infer<typeof userStatusSchema>

/** User role: 1 = common user, 10 = admin, 100 = root */
export const userRoleSchema = z.number()
export type UserRole = z.infer<typeof userRoleSchema>

export const inviteLimitStatusSchema = z.object({
  unlimited: z.boolean(),
  exempt: z.boolean(),
  limit: z.number(),
  used: z.number(),
  remaining: z.number(),
  mode: z.enum(['inherit', 'limited', 'unlimited', 'exempt']),
})

export const userSchema = z.object({
  id: z.number(),
  username: z.string(),
  display_name: z.string(),
  password: z.string().optional(),
  github_id: z.string().optional(),
  discord_id: z.string().optional(),
  oidc_id: z.string().optional(),
  wechat_id: z.string().optional(),
  telegram_id: z.string().optional(),
  email: z.string().optional(),
  quota: z.number(),
  used_quota: z.number(),
  request_count: z.number(),
  group: z.string(),
  aff_code: z.string().optional(),
  aff_count: z.number().optional(),
  aff_quota: z.number().optional(),
  aff_history_quota: z.number().optional(),
  inviter_id: z.number().optional(),
  inviter_source: z.string().optional(),
  aff_skip_reason: z.string().optional(),
  aff_skip_operator_id: z.number().optional(),
  aff_skip_at: z.number().optional(),
  invite_limit: inviteLimitStatusSchema.optional(),
  invite_limit_mode: z.enum(['inherit', 'limited', 'unlimited']).optional(),
  invite_limit_count: z.number().optional(),
  linux_do_id: z.string().optional(),
  status: userStatusSchema,
  role: userRoleSchema,
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
  last_login_at: z.number().optional(),
  DeletedAt: z.any().nullable().optional(),
  remark: z.string().optional(),
  admin_permissions: z
    .record(z.string(), z.record(z.string(), z.boolean()))
    .optional(),
})
export type User = z.infer<typeof userSchema>

export const userListSchema = z.array(userSchema)

// ============================================================================
// API Request/Response Types
// ============================================================================

/** Generic API response */
export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export type UserSortBy =
  | 'id'
  | 'username'
  | 'quota'
  | 'group'
  | 'created_at'
  | 'last_login_at'

export type UserSortOrder = 'asc' | 'desc'

export interface GetUsersParams {
  p?: number
  page_size?: number
  sort_by?: UserSortBy
  sort_order?: UserSortOrder
}

export interface GetUsersResponse {
  success: boolean
  message?: string
  data?: {
    items: User[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchUsersParams {
  keyword?: string
  group?: string
  role?: string
  status?: string
  p?: number
  page_size?: number
  sort_by?: UserSortBy
  sort_order?: UserSortOrder
}

export interface UserFormData {
  username: string
  display_name: string
  password?: string
  aff_code?: string // Only used when creating user
  aff_skip_reason?: string // Only used when creating user
  role?: number // Only used when creating user
  quota?: number // Only used when updating user
  group?: string // Only used when updating user
  remark?: string // Only used when updating user
  invite_limit_mode?: EditableInviteLimitMode // Only used when updating user
  invite_limit_count?: number // Only used when updating user
  admin_permissions?: AdminPermissionMatrix
  email?: string // Only used when updating user
  github_id?: string // Only used when updating user
  discord_id?: string // Only used when updating user
  oidc_id?: string // Only used when updating user
  wechat_id?: string // Only used when updating user
  telegram_id?: string // Only used when updating user
  linux_do_id?: string // Only used when updating user
}

export type ManageUserAction =
  | 'promote'
  | 'promote_root'
  | 'demote'
  | 'demote_root'
  | 'enable'
  | 'disable'
  | 'delete'
  | 'add_quota'

export interface ManageUserPayload {
  ban_reason_public?: string
  ban_reason_internal?: string
}

export type QuotaAdjustMode = 'add' | 'subtract' | 'override'

export interface ManageUserQuotaPayload {
  id: number
  action: 'add_quota'
  mode: QuotaAdjustMode
  value: number
}

export type UserSecurityAuditRange = '7d' | '30d' | '90d' | 'all' | 'custom'

export interface UserSecurityAuditSummary {
  registration_ip: string
  recent_ip: string
  api_request_count: number
  error_count: number
  login_success_count: number
  login_failure_count: number
  logout_count: number
  anomaly_count: number
  related_user_count: number
  last_activity_at: number
}

export interface UserSecurityAuditRelatedUser {
  user_id: number
  username: string
  shared_ip_count: number
  registration_count: number
  api_call_count: number
  authentication_count: number
  anomaly_count: number
  last_seen_at: number
}

export interface UserSecurityAuditTimelineItem {
  created_at: number
  ip: string
  kind: string
  request_id?: string
}

export interface UserSecurityAudit {
  enabled: boolean
  target_user_id: number
  username: string
  projection: 'full' | 'masked'
  is_root: boolean
  proxy_warning?: string
  summary: UserSecurityAuditSummary
  related_users: UserSecurityAuditRelatedUser[]
  related_total: number
  related_page: number
  related_page_size: number
  related_has_more: boolean
  timeline: UserSecurityAuditTimelineItem[]
  total: number
  page: number
  page_size: number
  range: UserSecurityAuditRange
  start_timestamp: number
  end_timestamp: number
}

export interface GetUserSecurityAuditParams {
  range?: UserSecurityAuditRange
  start_timestamp?: number
  end_timestamp?: number
  p?: number
  page_size?: number
  related_page?: number
  related_page_size?: number
}

// ============================================================================
// Dialog Types
// ============================================================================

export type UsersDialogType = 'create' | 'update' | 'delete'
