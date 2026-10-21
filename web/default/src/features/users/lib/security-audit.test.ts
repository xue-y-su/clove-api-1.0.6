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
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

import * as userLib from './index'
import { USER_SECURITY_AUDIT_COUNT_METRICS } from './security-audit'

type SecurityAuditHelpers = {
  canShowSensitiveUserSecurityDetails?: (data?: {
    is_root?: boolean
    projection?: string
  }) => boolean
  hasUserSecurityActivity?: (data?: {
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
      related_user_count?: number
      last_activity_at?: number
    }
  }) => boolean
  getUserSecurityAuditPageCorrection?: (
    data: {
      page: number
      page_size: number
      total: number
      timeline: unknown[]
    },
    currentPage: number
  ) => number | null
}

const helpers = userLib as SecurityAuditHelpers
const dialogSource = readFileSync(
  new URL(
    '../components/dialogs/user-security-audit-dialog.tsx',
    import.meta.url
  ),
  'utf8'
)

describe('user security audit projection', () => {
  test('shows a distinct disabled state with account terminology', () => {
    expect(dialogSource).toContain("viewState === 'disabled'")
    expect(dialogSource).toContain("t('Related accounts')")
    expect(dialogSource).toContain("t('No related accounts')")
  })

  test('includes the backend anomaly count in the summary metrics', () => {
    expect(
      USER_SECURITY_AUDIT_COUNT_METRICS.map((metric) => metric.field)
    ).toContain('anomaly_count')
  })

  test('paginates related accounts and labels each relationship source', () => {
    expect(dialogSource).toContain('relatedPage')
    expect(dialogSource).toContain('related_page')
    expect(dialogSource).toContain('shared_ip_count')
    expect(dialogSource).toContain('registration_count')
    expect(dialogSource).toContain('api_call_count')
    expect(dialogSource).toContain('authentication_count')
  })

  test('shows associations and timeline only for a full Root projection', () => {
    expect(
      helpers.canShowSensitiveUserSecurityDetails?.({
        is_root: true,
        projection: 'full',
      })
    ).toBe(true)
    expect(
      helpers.canShowSensitiveUserSecurityDetails?.({
        is_root: false,
        projection: 'masked',
      })
    ).toBe(false)
    expect(
      helpers.canShowSensitiveUserSecurityDetails?.({
        is_root: true,
        projection: 'masked',
      })
    ).toBe(false)
  })

  test('distinguishes an empty audit from a masked but populated summary', () => {
    expect(
      helpers.hasUserSecurityActivity?.({
        summary: {
          registration_ip: '',
          recent_ip: '',
          api_request_count: 0,
          error_count: 0,
          login_success_count: 0,
          login_failure_count: 0,
          related_user_count: 0,
          last_activity_at: 0,
        },
      })
    ).toBe(false)
    expect(
      helpers.hasUserSecurityActivity?.({
        summary: {
          registration_ip: '203.0.113.*',
          recent_ip: '',
          api_request_count: 0,
          error_count: 0,
          login_success_count: 0,
          login_failure_count: 0,
          related_user_count: 1,
          last_activity_at: 0,
        },
      })
    ).toBe(true)
  })

  test('treats non-summary Root timeline events as security activity', () => {
    expect(
      helpers.hasUserSecurityActivity?.({
        total: 1,
        timeline: [{ kind: 'activity_3' }],
        related_users: [],
        summary: {
          registration_ip: '',
          recent_ip: '',
          api_request_count: 0,
          error_count: 0,
          login_success_count: 0,
          login_failure_count: 0,
          related_user_count: 0,
          last_activity_at: 0,
        },
      })
    ).toBe(true)
  })

  test('corrects an empty out-of-range timeline without acting on stale data', () => {
    const emptyPage = {
      page: 99,
      page_size: 20,
      total: 45,
      timeline: [],
    }

    expect(helpers.getUserSecurityAuditPageCorrection?.(emptyPage, 99)).toBe(3)
    expect(
      helpers.getUserSecurityAuditPageCorrection?.(emptyPage, 2)
    ).toBeNull()
    expect(
      helpers.getUserSecurityAuditPageCorrection?.(
        { ...emptyPage, timeline: [{ kind: 'registration' }] },
        99
      )
    ).toBeNull()
  })

  test('treats non-summary Root timeline events as security activity', () => {
    expect(
      helpers.hasUserSecurityActivity?.({
        total: 1,
        timeline: [{ kind: 'activity_3' }],
        related_users: [],
        summary: {
          registration_ip: '',
          recent_ip: '',
          api_request_count: 0,
          error_count: 0,
          login_success_count: 0,
          login_failure_count: 0,
          related_user_count: 0,
          last_activity_at: 0,
        },
      })
    ).toBe(true)
  })

  test('corrects an empty out-of-range timeline without acting on stale data', () => {
    const emptyPage = {
      page: 99,
      page_size: 20,
      total: 45,
      timeline: [],
    }

    expect(helpers.getUserSecurityAuditPageCorrection?.(emptyPage, 99)).toBe(3)
    expect(
      helpers.getUserSecurityAuditPageCorrection?.(emptyPage, 2)
    ).toBeNull()
    expect(
      helpers.getUserSecurityAuditPageCorrection?.(
        { ...emptyPage, timeline: [{ kind: 'registration' }] },
        99
      )
    ).toBeNull()
  })
})
