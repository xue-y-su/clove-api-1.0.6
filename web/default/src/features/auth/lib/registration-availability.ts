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
import type { SystemStatus } from '../types'

function statusValue<K extends keyof NonNullable<SystemStatus['data']>>(
  status: SystemStatus | null | undefined,
  key: K
) {
  return status?.[key] ?? status?.data?.[key]
}

export function hasOAuthProvider(status: SystemStatus | null | undefined) {
  const customProviders =
    status?.custom_oauth_providers ?? status?.data?.custom_oauth_providers ?? []
  return Boolean(
    statusValue(status, 'wechat_login') ||
    statusValue(status, 'github_oauth') ||
    statusValue(status, 'discord_oauth') ||
    statusValue(status, 'oidc_enabled') ||
    statusValue(status, 'linuxdo_oauth') ||
    statusValue(status, 'telegram_oauth') ||
    customProviders.length > 0
  )
}

export function isRegistrationEnabled(status: SystemStatus | null | undefined) {
  return (
    statusValue(status, 'self_use_mode_enabled') !== true &&
    statusValue(status, 'register_enabled') !== false
  )
}

export function hasPasswordRegistration(
  status: SystemStatus | null | undefined
) {
  return (
    isRegistrationEnabled(status) &&
    statusValue(status, 'password_register_enabled') !== false
  )
}

export function hasOAuthRegistration(status: SystemStatus | null | undefined) {
  return (
    isRegistrationEnabled(status) &&
    statusValue(status, 'oauth_register_enabled') !== false &&
    hasOAuthProvider(status)
  )
}

export function hasAnyRegistrationMethod(
  status: SystemStatus | null | undefined
) {
  return hasPasswordRegistration(status) || hasOAuthRegistration(status)
}
