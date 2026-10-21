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
import { create } from 'zustand'

import type { AdminCapabilities } from '@/lib/admin-permissions'
import type { InviteLimitStatus } from '@/types/invite-limit'

export type UserPermissions = {
  sidebar_settings?: boolean
  sidebar_modules?: Record<string, unknown>
  admin_permissions?: AdminCapabilities
}

export interface AuthUser {
  id: number
  username: string
  display_name?: string
  avatar_url?: string
  email?: string
  role: number
  status?: number
  group?: string
  quota?: number
  used_quota?: number
  request_count?: number
  aff_code?: string
  aff_count?: number
  aff_quota?: number
  aff_history_quota?: number
  inviter_id?: number
  invite_limit?: InviteLimitStatus
  invite_limit_mode?: string
  invite_limit_count?: number
  github_id?: string
  discord_id?: string
  oidc_id?: string
  wechat_id?: string
  telegram_id?: string
  linux_do_id?: string
  setting?: Record<string, unknown> | string
  stripe_customer?: string
  sidebar_modules?: string
  permissions?: UserPermissions
}

interface AuthState {
  auth: {
    user: AuthUser | null
    setUser: (user: AuthUser | null) => void
    reset: () => void
  }
}

type AuthStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

function getBrowserAuthStorage(): AuthStorage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}

function readStoredUser(storage: AuthStorage | null): AuthUser | null {
  if (!storage) return null

  try {
    const saved = storage.getItem('user')
    return saved ? (JSON.parse(saved) as AuthUser) : null
  } catch {
    try {
      storage.removeItem('user')
    } catch {
      // Storage cleanup is best-effort; session recovery must still continue.
    }
    return null
  }
}

function persistStoredUser(
  storage: AuthStorage | null,
  user: AuthUser | null
): void {
  if (!storage) return

  try {
    if (user) {
      storage.setItem('user', JSON.stringify(user))
      storage.setItem('uid', String(user.id))
    } else {
      storage.removeItem('user')
      storage.removeItem('uid')
    }
  } catch {
    // Keep the in-memory session usable when browser storage is unavailable.
  }
}

export function createAuthStore(
  storage: AuthStorage | null = getBrowserAuthStorage()
) {
  const initUser = readStoredUser(storage)
  persistStoredUser(storage, initUser)

  return create<AuthState>()((set) => ({
    auth: {
      user: initUser,
      setUser: (user) =>
        set((state) => {
          persistStoredUser(storage, user)
          return { ...state, auth: { ...state.auth, user } }
        }),
      reset: () =>
        set((state) => {
          persistStoredUser(storage, null)
          return {
            ...state,
            auth: { ...state.auth, user: null },
          }
        }),
    },
  }))
}

export const useAuthStore = createAuthStore()
