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

import {
  type PermissionCatalog,
  type AdminPermissionMatrix,
  normalizeAdminPermissions,
} from '@/lib/admin-permissions'
import { quotaUnitsToDollars } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import type { EditableInviteLimitMode } from '@/types/invite-limit'

import { DEFAULT_GROUP } from '../constants'
import type { UserFormData, User } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export const userFormSchema = z.object({
  username: z
    .string()
    .min(1, 'Username is required')
    .max(20, 'Username must be at most 20 characters'),
  display_name: z.string().optional(),
  password: z.string().optional(),
  aff_code: z.string().optional(),
  aff_skip_reason: z.string().optional(),
  role: z.number().optional(),
  quota_dollars: z.number().min(0).optional(),
  group: z.string().optional(),
  remark: z.string().optional(),
  invite_limit_mode: z.enum(['inherit', 'limited', 'unlimited']).optional(),
  invite_limit_count: z.number().min(0).optional(),
  admin_permissions: z
    .record(z.string(), z.record(z.string(), z.boolean()))
    .optional(),
  email: z.string().optional(),
  github_id: z.string().optional(),
  discord_id: z.string().optional(),
  oidc_id: z.string().optional(),
  wechat_id: z.string().optional(),
  telegram_id: z.string().optional(),
  linux_do_id: z.string().optional(),
})

export type UserFormValues = z.infer<typeof userFormSchema>

// ============================================================================
// Form Defaults
// ============================================================================

export const USER_FORM_DEFAULT_VALUES: UserFormValues = {
  username: '',
  display_name: '',
  password: '',
  aff_code: '',
  aff_skip_reason: '',
  role: 1, // Default to common user
  quota_dollars: 0,
  group: DEFAULT_GROUP,
  remark: '',
  invite_limit_mode: 'inherit',
  invite_limit_count: 0,
  email: '',
  github_id: '',
  discord_id: '',
  oidc_id: '',
  wechat_id: '',
  telegram_id: '',
  linux_do_id: '',
  // Filled against the backend catalog at render time; see UsersMutateDrawer.
  admin_permissions: {},
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: UserFormValues,
  userId?: number,
  catalog?: PermissionCatalog
): UserFormData & { id?: number } {
  const cleanOptional = (value?: string) => value?.trim() || undefined
  const payload: UserFormData & { id?: number } = {
    username: data.username.trim(),
    display_name: cleanOptional(data.display_name) || data.username.trim(),
    password: cleanOptional(data.password),
  }

  const role = userId === undefined ? data.role || 1 : (data.role ?? 0)

  // Only send the permission matrix when the target is an admin and the catalog
  // is available; without the catalog we cannot build a full matrix, so we omit
  // the field (the backend then leaves existing permissions untouched).
  if (role >= ROLE.ADMIN && catalog) {
    payload.admin_permissions = normalizeAdminPermissions(
      data.admin_permissions as AdminPermissionMatrix | undefined,
      catalog
    )
  }

  // For create: only send required fields
  if (userId === undefined) {
    payload.role = role
    const affCode = data.aff_code?.trim()
    const affSkipReason = data.aff_skip_reason?.trim()
    if (affCode) {
      payload.aff_code = affCode
    }
    if (affSkipReason) {
      payload.aff_skip_reason = affSkipReason
    }
  } else {
    // For update: quota is adjusted atomically via /api/user/manage, not sent here
    payload.group = data.group
    payload.remark = cleanOptional(data.remark)
    payload.invite_limit_mode = (data.invite_limit_mode ??
      'inherit') as EditableInviteLimitMode
    payload.invite_limit_count = Math.max(0, data.invite_limit_count ?? 0)
    payload.email = data.email?.trim() ?? ''
    payload.github_id = data.github_id?.trim() ?? ''
    payload.discord_id = data.discord_id?.trim() ?? ''
    payload.oidc_id = data.oidc_id?.trim() ?? ''
    payload.wechat_id = data.wechat_id?.trim() ?? ''
    payload.telegram_id = data.telegram_id?.trim() ?? ''
    payload.linux_do_id = data.linux_do_id?.trim() ?? ''
    payload.id = userId
  }

  return payload
}

/**
 * Transform user data to form defaults. The admin permission matrix is passed
 * through as-is (the backend already returns a full matrix); it is filled against
 * the catalog at render time in UsersMutateDrawer.
 */
export function transformUserToFormDefaults(user: User): UserFormValues {
  return {
    username: user.username,
    display_name: user.display_name,
    password: '',
    aff_code: '',
    aff_skip_reason: '',
    role: user.role,
    quota_dollars: quotaUnitsToDollars(user.quota),
    group: user.group || DEFAULT_GROUP,
    remark: user.remark || '',
    invite_limit_mode: user.invite_limit_mode ?? 'inherit',
    invite_limit_count: Math.max(0, user.invite_limit_count ?? 0),
    email: user.email || '',
    github_id: user.github_id || '',
    discord_id: user.discord_id || '',
    oidc_id: user.oidc_id || '',
    wechat_id: user.wechat_id || '',
    telegram_id: user.telegram_id || '',
    linux_do_id: user.linux_do_id || '',
    admin_permissions: user.admin_permissions ?? {},
  }
}
