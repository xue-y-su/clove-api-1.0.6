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
import { getIPAuditPageCorrection } from '@/features/usage-logs/lib/query-params'

type UserSecurityAuditProjection = {
  enabled?: boolean
  is_root?: boolean
  projection?: string
}

type UserSecurityAuditActivity = {
  enabled?: boolean
  total?: number
  timeline?: unknown[]
  related_users?: unknown[]
  summary?: {
    registration_ip?: string
    recent_ip?: string
    api_request_count?: number
    error_count?: number
    login_success_count?: number
    login_failure_count?: number
    anomaly_count?: number
    logout_count?: number
    related_user_count?: number
    last_activity_at?: number
  }
}

export const USER_SECURITY_AUDIT_COUNT_METRICS = [
  { field: 'api_request_count', labelKey: 'API requests', risk: false },
  { field: 'error_count', labelKey: 'Errors', risk: true },
  { field: 'anomaly_count', labelKey: 'Anomalies', risk: true },
  { field: 'login_success_count', labelKey: 'Login success', risk: false },
  { field: 'login_failure_count', labelKey: 'Login failure', risk: true },
  { field: 'logout_count', labelKey: 'Logouts', risk: false },
] as const

export function canShowSensitiveUserSecurityDetails(
  data?: UserSecurityAuditProjection
): boolean {
  return (
    data?.enabled !== false &&
    data?.is_root === true &&
    data.projection === 'full'
  )
}

export function hasUserSecurityActivity(
  data?: UserSecurityAuditActivity
): boolean {
  if (data?.enabled === false) return false
  if (data?.total || data?.timeline?.length || data?.related_users?.length) {
    return true
  }
  const summary = data?.summary
  if (!summary) return false
  return Boolean(
    summary.registration_ip ||
    summary.recent_ip ||
    summary.api_request_count ||
    summary.error_count ||
    summary.login_success_count ||
    summary.login_failure_count ||
    summary.anomaly_count ||
    summary.logout_count ||
    summary.related_user_count ||
    summary.last_activity_at
  )
}

export function getUserSecurityAuditPageCorrection(
  data:
    | {
        page: number
        page_size: number
        total: number
        timeline: unknown[]
      }
    | undefined,
  currentPage: number
): number | null {
  if (!data || data.page !== currentPage) return null
  return getIPAuditPageCorrection({
    page: data.page,
    pageSize: data.page_size,
    total: data.total,
    itemCount: data.timeline.length,
  })
}
