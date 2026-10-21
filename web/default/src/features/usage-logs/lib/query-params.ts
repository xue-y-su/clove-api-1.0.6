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

export function buildQueryParams(
  params: Record<string, unknown>
): URLSearchParams {
  const queryParams = new URLSearchParams()

  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      queryParams.append(key, String(value))
    }
  })

  return queryParams
}

export type IPAuditRange = '7d' | '30d' | '90d' | 'all' | 'custom'
export type IPAuditView =
  | 'risk'
  | 'shared_api'
  | 'shared_registration'
  | 'authentication'
  | 'all'

export const IP_AUDIT_RANGE_OPTIONS = [
  { value: '7d', labelKey: '7 Days' },
  { value: '30d', labelKey: '30 Days' },
  { value: '90d', labelKey: '90 Days' },
  { value: 'all', labelKey: 'All history' },
  { value: 'custom', labelKey: 'Custom range' },
] as const satisfies readonly {
  value: IPAuditRange
  labelKey: string
}[]

export const IP_AUDIT_VIEW_OPTIONS = [
  { value: 'risk', labelKey: 'Risk overview' },
  { value: 'shared_api', labelKey: 'Shared API IPs' },
  { value: 'shared_registration', labelKey: 'Shared registration IPs' },
  { value: 'authentication', labelKey: 'Authentication activity' },
  { value: 'all', labelKey: 'All IP activity' },
] as const satisfies readonly {
  value: IPAuditView
  labelKey: string
}[]

export const IP_AUDIT_COUNT_METRICS = [
  { field: 'request_count', labelKey: 'Requests', risk: false },
  { field: 'registration_count', labelKey: 'Registrations', risk: false },
  { field: 'api_request_count', labelKey: 'API requests', risk: false },
  { field: 'unauthorized_count', labelKey: 'Unauthorized (401)', risk: true },
  { field: 'login_success_count', labelKey: 'Login success', risk: false },
  { field: 'login_failure_count', labelKey: 'Login failure', risk: false },
  { field: 'logout_count', labelKey: 'Logouts', risk: false },
  { field: 'anomaly_count', labelKey: 'Anomalies', risk: true },
] as const

export type IPAuditSearch = {
  range?: string
  view?: string
  page?: number
  pageSize?: number
  startTime?: number
  endTime?: number
  userPage?: number
  userPageSize?: number
  timelinePage?: number
  timelinePageSize?: number
  relatedPage?: number
  relatedPageSize?: number
}

export function isIPAuditRange(value: unknown): value is IPAuditRange {
  return IP_AUDIT_RANGE_OPTIONS.some((option) => option.value === value)
}

export function isIPAuditView(value: unknown): value is IPAuditView {
  return IP_AUDIT_VIEW_OPTIONS.some((option) => option.value === value)
}

export function buildIPAuditQuery(search: IPAuditSearch) {
  const range = isIPAuditRange(search.range) ? search.range : '30d'
  const view = isIPAuditView(search.view) ? search.view : 'risk'
  const query: {
    range: IPAuditRange
    view: IPAuditView
    p: number
    page_size: number
    start_timestamp?: number
    end_timestamp?: number
    user_page?: number
    user_page_size?: number
    timeline_page?: number
    timeline_page_size?: number
    related_page?: number
    related_page_size?: number
  } = {
    range,
    view,
    p: Math.max(1, search.page ?? 1),
    page_size: Math.min(100, Math.max(1, search.pageSize ?? 20)),
  }

  if (search.userPage !== undefined) {
    query.user_page = Math.max(1, search.userPage)
  }
  if (search.userPageSize !== undefined) {
    query.user_page_size = Math.min(100, Math.max(1, search.userPageSize))
  }
  if (search.timelinePage !== undefined) {
    query.timeline_page = Math.max(1, search.timelinePage)
  }
  if (search.timelinePageSize !== undefined) {
    query.timeline_page_size = Math.min(
      100,
      Math.max(1, search.timelinePageSize)
    )
  }
  if (search.relatedPage !== undefined) {
    query.related_page = Math.max(1, search.relatedPage)
  }
  if (search.relatedPageSize !== undefined) {
    query.related_page_size = Math.min(100, Math.max(1, search.relatedPageSize))
  }

  if (range === 'custom') {
    if (search.startTime) {
      query.start_timestamp = Math.floor(search.startTime / 1000)
    }
    if (search.endTime) {
      query.end_timestamp = Math.floor(search.endTime / 1000)
    }
  }

  return query
}

export function formatIPAuditTimestamp(
  timestamp: number,
  locale?: string
): string {
  return timestamp
    ? new Date(timestamp * 1000).toLocaleString(toIntlLocale(locale))
    : '-'
}

export type IPAuditViewState =
  | 'loading'
  | 'error'
  | 'disabled'
  | 'empty'
  | 'ready'

export function getIPAuditViewState(input: {
  isLoading: boolean
  isError: boolean
  enabled?: boolean
  itemCount: number
}): IPAuditViewState {
  if (input.isLoading) return 'loading'
  if (input.isError) return 'error'
  if (input.enabled === false) return 'disabled'
  if (input.itemCount === 0) return 'empty'
  return 'ready'
}

export function getIPAuditPageCount(total: number, pageSize: number): number {
  return Math.max(1, Math.ceil(Math.max(0, total) / Math.max(1, pageSize)))
}

export function getIPAuditPageCorrection(input: {
  page: number
  pageSize: number
  total: number
  itemCount: number
}): number | null {
  if (input.total <= 0 || input.itemCount > 0) return null
  const lastPage = getIPAuditPageCount(input.total, input.pageSize)
  return input.page > lastPage ? lastPage : null
}

const IP_AUDIT_ERROR_MESSAGE_KEYS: Record<string, string> = {
  ip_audit_projection_not_ready:
    'IP audit data is still being prepared. Please retry shortly.',
  ip_audit_forbidden: 'You do not have permission to view this audit.',
  ip_audit_invalid_range: 'Select a complete custom range.',
}

export function getIPAuditErrorMessageKey(
  error: unknown,
  fallbackKey: string
): string {
  let code = ''
  if (error instanceof Error) {
    code = error.message.trim()
  } else if (typeof error === 'string') {
    code = error.trim()
  }
  return IP_AUDIT_ERROR_MESSAGE_KEYS[code] ?? fallbackKey
}

export function canShowSensitiveIPAuditDetails(data?: {
  enabled?: boolean
  can_view_details?: boolean
}): boolean {
  return data?.enabled !== false && data?.can_view_details === true
}

export function getIPAuditScopeLabelKey(data?: { is_root?: boolean }): string {
  return data?.is_root === true ? 'Root view' : 'Admin masked view'
}

const TRUSTED_PROXY_WARNING_KEYS: Record<string, string> = {
  trusted_proxies_not_configured:
    'Trusted proxies are not configured. Client IP addresses may be inaccurate behind a reverse proxy.',
  trusted_proxies_unsafe:
    'Trusted proxy configuration accepts untrusted sources. Review TRUSTED_PROXIES before relying on IP audit data.',
}

export function getTrustedProxyWarningKey(warning?: string): string | null {
  return warning ? (TRUSTED_PROXY_WARNING_KEYS[warning] ?? warning) : null
}

export function getIPAuditNetworkScopeLabelKey(item: {
  proxy_artifact?: boolean
}): string | null {
  return item.proxy_artifact ? 'Infrastructure IP' : null
}

const IP_AUDIT_RISK_REASON_LABEL_KEYS: Record<string, string> = {
  shared_registration_ip: 'Shared registration IP',
  shared_api_ip: 'Shared API IP',
  unauthorized_burst: 'Unauthorized burst',
}

export function getIPAuditRiskReasonLabelKeys(item: {
  risk_reasons?: string[]
}): string[] {
  if (!Array.isArray(item.risk_reasons)) return []
  return item.risk_reasons
    .map((reason) => IP_AUDIT_RISK_REASON_LABEL_KEYS[reason])
    .filter((labelKey): labelKey is string => Boolean(labelKey))
}

const IP_AUDIT_TIMELINE_LABEL_KEYS: Record<string, string> = {
  registration: 'Registration',
  api_request: 'API request',
  api_error: 'API error',
  login_success: 'Login success',
  login_failure: 'Login failure',
  logout: 'Logout',
}

export function getIPAuditTimelineLabelKey(item: { kind?: string }): string {
  return IP_AUDIT_TIMELINE_LABEL_KEYS[item.kind ?? ''] ?? 'Other activity'
}
