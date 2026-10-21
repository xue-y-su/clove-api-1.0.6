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

type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type SourceAttributionOption = {
  id: string
  name: string
  requires_description: boolean
  sort_order: number
}

export type SourceAttributionAdminOption = SourceAttributionOption & {
  target_user_id: number
  target_aff_code?: string
}

export type SourceAttributionStatus = {
  enabled: boolean
  confirmed: boolean
  options: SourceAttributionOption[]
}

export type SourceAttributionSettings = {
  enabled: boolean
  token_api_enforcement_enabled: boolean
  options: SourceAttributionAdminOption[]
}

export type SourceAttributionConfirmation = {
  id: number
  user_id: number
  username: string
  option_id: string
  option_name: string
  source_description: string
  target_user_id: number
  target_username: string
  target_aff_code: string
  ip: string
  user_agent: string
  confirmed_at: number
}

export type SourceAttributionConfirmationsPage = {
  page: number
  page_size: number
  total: number
  items: SourceAttributionConfirmation[]
}

export type RegistrationSettings = {
  aff_required: boolean
  invite_limit_unlimited: boolean
  invite_limit_count: number
  invitation_applications_enabled: boolean
  invitation_reapply_cooldown_seconds: number
  invitation_poster_title: string
  invitation_poster_subtitle: string
}

export async function getSourceAttributionStatus(signal?: AbortSignal) {
  const res = await api.get<ApiResponse<SourceAttributionStatus>>(
    '/api/source-attribution/status',
    {
      disableDuplicate: true,
      signal,
      skipErrorHandler: true,
      timeout: 10_000,
    }
  )
  return res.data
}

export async function confirmSourceAttribution(payload: {
  option_id: string
  source_description?: string
}) {
  const res = await api.post<ApiResponse<{ confirmed: boolean }>>(
    '/api/source-attribution/confirm',
    payload
  )
  return res.data
}

export async function getSourceAttributionSettings() {
  const res = await api.get<ApiResponse<SourceAttributionSettings>>(
    '/api/source-attribution/settings'
  )
  return res.data
}

export async function updateSourceAttributionSettings(
  settings: SourceAttributionSettings
) {
  const res = await api.put<ApiResponse<SourceAttributionSettings>>(
    '/api/source-attribution/settings',
    settings
  )
  return res.data
}

export async function getSourceAttributionConfirmations(params: {
  p?: number
  page_size?: number
  username?: string
  option_id?: string
  target_user_id?: number
  ip?: string
}) {
  const res = await api.get<ApiResponse<SourceAttributionConfirmationsPage>>(
    '/api/source-attribution/confirmations',
    { params }
  )
  return res.data
}

export async function getRegistrationSettings() {
  const res = await api.get<ApiResponse<RegistrationSettings>>(
    '/api/registration/settings'
  )
  return res.data
}

export async function updateRegistrationSettings(
  settings: RegistrationSettings
) {
  const res = await api.put<ApiResponse<RegistrationSettings>>(
    '/api/registration/settings',
    settings
  )
  return res.data
}
