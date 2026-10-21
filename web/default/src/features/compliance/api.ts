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

export type ComplianceDisplayMode = 'single' | 'multiple'
export type ComplianceConfirmationMode = 'checkbox' | 'phrase' | 'segments'
export type ComplianceSourceType =
  | 'manual'
  | 'user_agreement'
  | 'privacy_policy'

export type ComplianceSnapshot = {
  id: number
  item_key: string
  title: string
  source_type: ComplianceSourceType | string
  version: string
  content: string
  content_hash: string
  content_length: number
  sort_order: number
  confirmation_mode: ComplianceConfirmationMode | string
  confirmation_phrase?: string
  segments?: string[]
  confirmed: boolean
}

export type ComplianceStatement = {
  enabled: boolean
  version: string
  content: string
  content_hash: string
  confirmed: boolean
  display_mode: ComplianceDisplayMode | string
  snapshots: ComplianceSnapshot[]
}

export type ComplianceStatementItem = {
  item_key: string
  title: string
  source_type: ComplianceSourceType | string
  version: string
  content: string
  sort_order: number
  confirmation_mode: ComplianceConfirmationMode | string
  confirmation_phrase?: string
  segments?: string[]
}

export type ComplianceSettingsSnapshot = {
  id: number
  item_key: string
  version: string
  content_hash: string
  content_length: number
  sort_order: number
  created_at: number
}

export type ComplianceSettings = {
  statement_enabled: boolean
  statement_version: string
  statement_content: string
  statement_items: ComplianceStatementItem[]
  statement_display_mode: ComplianceDisplayMode | string
  token_api_enforcement_enabled: boolean
  show_version_to_user: boolean
  show_content_hash_to_user: boolean
  content_hash: string
  snapshots?: ComplianceSettingsSnapshot[]
}

export type ComplianceConfirmation = {
  id: number
  user_id: number
  username: string
  snapshot_id?: number
  version: string
  content_hash: string
  ip: string
  user_agent: string
  confirmed_at: number
}

export type ComplianceConfirmationsPage = {
  page: number
  page_size: number
  total: number
  items: ComplianceConfirmation[]
}

type ApiResponse<T> = {
  success: boolean
  message: string
  data: T
}

export type ComplianceConfirmRequest = {
  snapshot_id?: number
  confirmation_text?: string
  confirmation_texts?: string[]
  segments?: string[]
}

export async function getComplianceStatement(signal?: AbortSignal) {
  const res = await api.get<ApiResponse<ComplianceStatement>>(
    '/api/compliance/statement',
    {
      disableDuplicate: true,
      signal,
      skipErrorHandler: true,
      timeout: 10_000,
    }
  )
  return res.data
}

export async function confirmComplianceStatement(
  payload: ComplianceConfirmRequest = {}
) {
  const res = await api.post<
    ApiResponse<{ confirmed: boolean; snapshots?: ComplianceSnapshot[] }>
  >('/api/compliance/confirm', payload)
  return res.data
}

export async function getComplianceSettings() {
  const res = await api.get<ApiResponse<ComplianceSettings>>(
    '/api/compliance/settings'
  )
  return res.data
}

export async function updateComplianceSettings(settings: ComplianceSettings) {
  const res = await api.put<ApiResponse<ComplianceSettings>>(
    '/api/compliance/settings',
    settings
  )
  return res.data
}

export async function getComplianceConfirmations(params: {
  p?: number
  page_size?: number
  username?: string
  version?: string
  ip?: string
}) {
  const res = await api.get<ApiResponse<ComplianceConfirmationsPage>>(
    '/api/compliance/confirmations',
    { params }
  )
  return res.data
}
