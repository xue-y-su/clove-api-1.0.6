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
import { toIntlLocale } from '@/i18n/languages'
import type { AuthUser } from '@/stores/auth-store'

import type { AvatarUploadConfig, UserProfile } from '../types'

export type ProfileIdentityDraft = {
  display_name: string
  avatar_url: string
}

type ProfileIdentityCurrent = Pick<UserProfile, 'display_name' | 'avatar_url'> &
  Partial<Pick<UserProfile, 'username'>>

const displayNameCooldownClockSkewToleranceSeconds = 5 * 60

export function mergeAuthUserProfile(
  authUser: AuthUser,
  profile: UserProfile
): AuthUser {
  return {
    ...authUser,
    username: profile.username,
    display_name: profile.display_name,
    avatar_url: profile.avatar_url,
    group: profile.group,
  }
}

export function buildProfilePatch(
  current: ProfileIdentityCurrent,
  draft: ProfileIdentityDraft
): Partial<ProfileIdentityDraft> {
  const patch: Partial<ProfileIdentityDraft> = {}
  const displayName = draft.display_name.trim()
  const avatarURL = draft.avatar_url.trim()
  const currentDisplayName = (
    current.display_name ||
    current.username ||
    ''
  ).trim()

  if (displayName !== currentDisplayName) {
    patch.display_name = displayName
  }
  if (avatarURL !== (current.avatar_url || '').trim()) {
    patch.avatar_url = avatarURL
  }
  return patch
}

export function isDisplayNameCoolingDown(
  profile: Pick<UserProfile, 'display_name_next_change_at'>,
  nowSeconds = Math.floor(Date.now() / 1000)
): boolean {
  return (
    typeof profile.display_name_next_change_at === 'number' &&
    profile.display_name_next_change_at > nowSeconds
  )
}

export function formatDisplayNameCooldownRemaining(
  nextChangeAt: number | undefined,
  nowSeconds = Math.floor(Date.now() / 1000),
  locale?: string
): string {
  if (typeof nextChangeAt !== 'number' || nextChangeAt <= nowSeconds) {
    return ''
  }
  const remainingSeconds = nextChangeAt - nowSeconds
  let value: number
  let unit: Intl.RelativeTimeFormatUnit
  if (remainingSeconds >= 24 * 60 * 60) {
    value = Math.max(
      1,
      Math.ceil(
        (remainingSeconds - displayNameCooldownClockSkewToleranceSeconds) /
          (24 * 60 * 60)
      )
    )
    unit = 'day'
  } else if (remainingSeconds >= 60 * 60) {
    value = Math.max(
      1,
      Math.ceil(
        (remainingSeconds - displayNameCooldownClockSkewToleranceSeconds) /
          (60 * 60)
      )
    )
    unit = 'hour'
  } else {
    value = Math.max(1, Math.ceil(remainingSeconds / 60))
    unit = 'minute'
  }
  return new Intl.RelativeTimeFormat(toIntlLocale(locale), {
    numeric: 'always',
  }).format(value, unit)
}

export function formatDisplayNameCooldownDate(
  nextChangeAt: number | undefined,
  locale?: string
): string {
  if (typeof nextChangeAt !== 'number' || nextChangeAt <= 0) {
    return ''
  }
  return new Date(nextChangeAt * 1000).toLocaleString(toIntlLocale(locale))
}

const supportedAvatarTypes = new Set([
  'image/jpeg',
  'image/png',
  'image/webp',
  'image/gif',
])

function avatarUploadResponseURL(payload: unknown): string {
  if (typeof payload !== 'object' || payload === null) return ''
  const response = payload as Record<string, unknown>
  if (typeof response.url === 'string') return response.url.trim()
  if (typeof response.data === 'string') return response.data.trim()
  if (typeof response.data === 'object' && response.data !== null) {
    const data = response.data as Record<string, unknown>
    if (typeof data.url === 'string') return data.url.trim()
    if (typeof data.image_url === 'string') return data.image_url.trim()
  }
  return ''
}

function isAllowedAvatarURL(url: string, allowedDomains: string[]): boolean {
  try {
    const parsed = new URL(url)
    if (
      parsed.protocol !== 'https:' ||
      parsed.username ||
      parsed.password ||
      parsed.port ||
      parsed.hash
    ) {
      return false
    }
    const host = parsed.hostname.toLowerCase()
    return allowedDomains.some((domain) => domain.trim().toLowerCase() === host)
  } catch {
    return false
  }
}

export async function uploadAvatarToImageHost(
  file: File,
  config: AvatarUploadConfig,
  fetcher: typeof fetch = fetch,
  options: { signal?: AbortSignal; timeoutMs?: number } = {}
): Promise<string> {
  if (!supportedAvatarTypes.has(file.type.toLowerCase())) {
    throw new Error('Avatar must be a JPEG, PNG, WebP, or GIF image')
  }
  if (!config.enabled || !config.endpoint.trim()) {
    throw new Error('Avatar uploads are unavailable')
  }
  if (options.signal?.aborted) {
    throw new Error('Avatar upload cancelled')
  }
  const controller = new AbortController()
  let timedOut = false
  const abortUpload = () => controller.abort()
  options.signal?.addEventListener('abort', abortUpload, { once: true })
  const timeoutId = globalThis.setTimeout(
    () => {
      timedOut = true
      controller.abort()
    },
    Math.max(1, options.timeoutMs ?? 30_000)
  )
  const formData = new FormData()
  formData.append('image', file, file.name)
  try {
    let response: Response
    try {
      response = await fetcher(config.endpoint, {
        method: 'POST',
        body: formData,
        signal: controller.signal,
      })
    } catch {
      if (timedOut) throw new Error('Avatar upload timed out')
      if (options.signal?.aborted) throw new Error('Avatar upload cancelled')
      throw new Error('Unable to reach avatar host')
    }
    let payload: unknown
    try {
      payload = await response.json()
    } catch {
      if (timedOut) throw new Error('Avatar upload timed out')
      if (options.signal?.aborted) throw new Error('Avatar upload cancelled')
      throw new Error('Avatar host returned an invalid response')
    }
    const result = payload as Record<string, unknown>
    if (!response.ok || result.success === false) {
      let message = 'Avatar upload failed'
      if (typeof result.message === 'string') {
        message = result.message
      } else if (typeof result.error === 'string') {
        message = result.error
      }
      throw new Error(message)
    }
    const url = avatarUploadResponseURL(payload)
    if (!isAllowedAvatarURL(url, config.allowed_domains)) {
      throw new Error('Avatar upload returned an untrusted URL')
    }
    return url
  } finally {
    globalThis.clearTimeout(timeoutId)
    options.signal?.removeEventListener('abort', abortUpload)
  }
}
