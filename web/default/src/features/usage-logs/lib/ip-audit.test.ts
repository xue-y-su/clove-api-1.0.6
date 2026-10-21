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

import * as ipAuditLib from './query-params'

test('keeps the desktop table horizontally navigable and bounds user labels', () => {
  expect(tableSource).toContain('table-fixed')
  expect(tableSource).toContain("<div className='hidden md:block'>")
  expect(tableSource).toContain(
    "<Table className='min-w-[2200px] table-fixed'>"
  )
  expect(tableSource).not.toContain(
    "<div className='hidden min-w-[2200px] md:block'>"
  )
  expect(tableSource).toContain('overflow-auto')
  expect(tableSource).toMatch(
    /<TableHead className='w-48 whitespace-nowrap'>\s*\{t\('First Seen'\)\}\s*<\/TableHead>/
  )
  expect(tableSource).toMatch(
    /<TableHead className='w-48 whitespace-nowrap'>\s*\{t\('Last Seen'\)\}\s*<\/TableHead>/
  )
  expect(tableSource).toContain('max-w-[18rem]')
  expect(tableSource).toContain('overflow-hidden')
  expect(tableSource).toContain('[overflow-wrap:anywhere]')
  expect(tableSource).toContain('function IPAuditAddress')
  expect(tableSource).toContain('title={props.item.ip}')
  expect(tableSource).toContain('truncate')
  expect(tableSource).toContain('getIPAuditNetworkScopeLabelKey')
  expect(tableSource).toContain("t('Related accounts')")
  expect(tableSource).toContain(
    'API requests are attributed to the API Key owner. Authentication events are attributed to the acting account.'
  )
})

type IPAuditHelpers = {
  IP_AUDIT_RANGE_OPTIONS?: readonly { value: string; labelKey: string }[]
  IP_AUDIT_VIEW_OPTIONS?: readonly { value: string; labelKey: string }[]
  IP_AUDIT_COUNT_METRICS?: readonly { field: string; labelKey: string }[]
  buildIPAuditQuery?: (search: {
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
  }) => Record<string, unknown>
  getIPAuditViewState?: (input: {
    isLoading: boolean
    isError: boolean
    enabled?: boolean
    itemCount: number
  }) => string
  getIPAuditPageCount?: (total: number, pageSize: number) => number
  getIPAuditPageCorrection?: (input: {
    page: number
    pageSize: number
    total: number
    itemCount: number
  }) => number | null
  getIPAuditErrorMessageKey?: (error: unknown, fallbackKey: string) => string
  canShowSensitiveIPAuditDetails?: (data?: {
    enabled?: boolean
    is_root?: boolean
    can_view_details?: boolean
  }) => boolean
  getIPAuditScopeLabelKey?: (data?: {
    is_root?: boolean
    can_view_details?: boolean
  }) => string
  getTrustedProxyWarningKey?: (warning?: string) => string | null
  getIPAuditNetworkScopeLabelKey?: (item: {
    proxy_artifact?: boolean
  }) => string | null
  getIPAuditRiskReasonLabelKeys?: (item: {
    risk_reasons?: string[]
  }) => string[]
  getIPAuditTimelineLabelKey?: (item: {
    kind?: string
    content?: string
  }) => string
  formatIPAuditTimestamp?: (timestamp: number, locale?: string) => string
}

const helpers = ipAuditLib as IPAuditHelpers
const usageAPI = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const usersAPI = readFileSync(
  new URL('../../users/api.ts', import.meta.url),
  'utf8'
)
const controlsSource = readFileSync(
  new URL('../components/ip-audit-controls.tsx', import.meta.url),
  'utf8'
)
const tableSource = readFileSync(
  new URL('../components/ip-audit-table.tsx', import.meta.url),
  'utf8'
)
const commonLogsColumnsSource = readFileSync(
  new URL('../components/columns/common-logs-columns.tsx', import.meta.url),
  'utf8'
)
const detailsDialogSource = readFileSync(
  new URL('../components/dialogs/details-dialog.tsx', import.meta.url),
  'utf8'
)
const typesSource = readFileSync(
  new URL('../types.ts', import.meta.url),
  'utf8'
)

describe('IP audit list contract', () => {
  test('includes registration-only rows in the displayed count metrics', () => {
    expect(
      helpers.IP_AUDIT_COUNT_METRICS?.map((metric) => metric.field)
    ).toContain('registration_count')
    expect(
      helpers.IP_AUDIT_COUNT_METRICS?.map((metric) => metric.field)
    ).toContain('logout_count')
  })

  test('shows unauthorized responses without promoting ordinary upstream errors', () => {
    const fields = helpers.IP_AUDIT_COUNT_METRICS?.map((metric) => metric.field)

    expect(fields).toContain('unauthorized_count')
    expect(fields).not.toContain('error_count')
    expect(tableSource).not.toContain("t('Errors')")
  })

  test('offers the exact supported range presets', () => {
    expect(
      helpers.IP_AUDIT_RANGE_OPTIONS?.map((option) => option.value)
    ).toEqual(['7d', '30d', '90d', 'all', 'custom'])
  })

  test('offers separate risk, API, registration, authentication, and all views', () => {
    expect(
      helpers.IP_AUDIT_VIEW_OPTIONS?.map((option) => option.value)
    ).toEqual([
      'risk',
      'shared_api',
      'shared_registration',
      'authentication',
      'all',
    ])
  })

  test('keeps audit HTTP failures inside the redacted inline error state', () => {
    expect(usageAPI).toMatch(
      /\/api\/log\/ip-audit[\s\S]{0,400}skipBusinessError:\s*true[\s\S]{0,100}skipErrorHandler:\s*true/
    )
    expect(usersAPI).toMatch(
      /security-audit[\s\S]{0,400}skipBusinessError:\s*true[\s\S]{0,100}skipErrorHandler:\s*true/
    )
  })

  test('loads permission-scoped IP details after an IP is selected', () => {
    expect(usageAPI).toContain('/api/log/ip-audit/detail')
    expect(tableSource).toContain("queryKey: ['ip-audit-detail'")
    expect(tableSource).toMatch(
      /enabled:\s*showSensitive\s*&&\s*selectedIP\s*!==\s*null/
    )
  })

  test('names page-size and mobile loading controls for assistive technology', () => {
    expect(controlsSource).toMatch(/aria-label=\{t\(['"]Rows per page['"]\)\}/)
    expect(tableSource).toMatch(/role=['"]status['"]/)
    expect(tableSource).toMatch(/aria-live=['"]polite['"]/)
    expect(tableSource).toMatch(/t\(['"]Loading\.\.\.['"]\)/)
  })

  test('bounds IPv6 addresses in common log rows and safely wraps details', () => {
    expect(commonLogsColumnsSource).toContain(
      "className='max-w-[140px] overflow-hidden font-mono [&>span]:truncate'"
    )
    expect(commonLogsColumnsSource).toContain(
      'title={sensitiveVisible ? ip : undefined}'
    )
    expect(detailsDialogSource).toContain("className='min-w-0 break-all'")
  })

  test('lets wrapped mobile view tabs grow instead of covering range controls', () => {
    expect(controlsSource).toContain('group-data-horizontal/tabs:h-auto')
  })

  test('makes the selected audit range and loading state explicit', () => {
    expect(controlsSource).toContain(
      "variant={range === option.value ? 'default' : 'ghost'}"
    )
    expect(controlsSource).toContain("aria-live='polite'")
    expect(controlsSource).toContain("t('Audit range')")
    expect(controlsSource).toContain("t('Loading...')")
    expect(tableSource).toContain('loading={auditQuery.isFetching}')
  })

  test('keeps partial audit rows usable while historical data is syncing', () => {
    expect(typesSource).toContain('sync: IPAuditSyncStatus')
    expect(tableSource).toContain('data?.sync?.ready === false')
    expect(tableSource).toContain("data-nonblocking-progress='true'")
    expect(tableSource).toContain(
      "'Historical IP audit data is syncing. Available results are shown now.'"
    )
    expect(tableSource).toContain('data.sync.processed_logs.toLocaleString()')
    expect(typesSource).toContain("| 'unknown'")
    expect(tableSource).toContain(
      "'IP audit sync status is temporarily unavailable'"
    )
  })

  test('formats audit timestamps with the active locale', () => {
    const timestamp = 1_700_000_000
    expect(helpers.formatIPAuditTimestamp?.(timestamp, 'fr-FR')).toBe(
      new Date(timestamp * 1000).toLocaleString('fr-FR')
    )
    expect(helpers.formatIPAuditTimestamp?.(0, 'fr-FR')).toBe('-')
  })

  test('formats i18next Chinese locale identifiers with valid Intl tags', () => {
    const timestamp = 1_700_000_000

    expect(helpers.formatIPAuditTimestamp?.(timestamp, 'zhCN')).toBe(
      new Date(timestamp * 1000).toLocaleString('zh-CN')
    )
    expect(helpers.formatIPAuditTimestamp?.(timestamp, 'zhTW')).toBe(
      new Date(timestamp * 1000).toLocaleString('zh-TW')
    )
  })

  test('defaults to the server-backed thirty-day first page', () => {
    expect(helpers.buildIPAuditQuery?.({})).toEqual({
      range: '30d',
      view: 'risk',
      p: 1,
      page_size: 20,
    })
  })

  test('sends custom timestamps in seconds and preserves pagination', () => {
    expect(
      helpers.buildIPAuditQuery?.({
        range: 'custom',
        view: 'shared_registration',
        page: 3,
        pageSize: 50,
        startTime: 1_720_000_000_999,
        endTime: 1_720_086_400_001,
      })
    ).toEqual({
      range: 'custom',
      view: 'shared_registration',
      p: 3,
      page_size: 50,
      start_timestamp: 1_720_000_000,
      end_timestamp: 1_720_086_400,
    })
  })

  test('does not leak stale custom dates into preset queries', () => {
    expect(
      helpers.buildIPAuditQuery?.({
        range: '7d',
        startTime: 1_720_000_000_000,
        endTime: 1_720_086_400_000,
      })
    ).toEqual({ range: '7d', view: 'risk', p: 1, page_size: 20 })
  })

  test('encodes independent IP detail pagination', () => {
    expect(
      helpers.buildIPAuditQuery?.({
        range: 'all',
        view: 'shared_api',
        userPage: 2,
        userPageSize: 50,
        timelinePage: 3,
        timelinePageSize: 10,
        relatedPage: 4,
        relatedPageSize: 20,
      })
    ).toEqual({
      range: 'all',
      view: 'shared_api',
      p: 1,
      page_size: 20,
      user_page: 2,
      user_page_size: 50,
      timeline_page: 3,
      timeline_page_size: 10,
      related_page: 4,
      related_page_size: 20,
    })
  })

  test.each([
    [{ isLoading: true, isError: false, itemCount: 0 }, 'loading'],
    [{ isLoading: false, isError: true, itemCount: 0 }, 'error'],
    [
      { isLoading: false, isError: false, enabled: false, itemCount: 0 },
      'disabled',
    ],
    [{ isLoading: false, isError: false, itemCount: 0 }, 'empty'],
    [{ isLoading: false, isError: false, itemCount: 2 }, 'ready'],
  ])('derives the %s view state', (input, expected) => {
    expect(helpers.getIPAuditViewState?.(input)).toBe(expected)
  })

  test('derives pages from the true server total', () => {
    expect(helpers.getIPAuditPageCount?.(101, 20)).toBe(6)
    expect(helpers.getIPAuditPageCount?.(0, 20)).toBe(1)
  })

  test('corrects an empty out-of-range page to the true last page', () => {
    expect(
      helpers.getIPAuditPageCorrection?.({
        page: 99,
        pageSize: 20,
        total: 45,
        itemCount: 0,
      })
    ).toBe(3)
    expect(
      helpers.getIPAuditPageCorrection?.({
        page: 3,
        pageSize: 20,
        total: 45,
        itemCount: 0,
      })
    ).toBeNull()
    expect(
      helpers.getIPAuditPageCorrection?.({
        page: 99,
        pageSize: 20,
        total: 0,
        itemCount: 0,
      })
    ).toBeNull()
    expect(
      helpers.getIPAuditPageCorrection?.({
        page: 99,
        pageSize: 20,
        total: 45,
        itemCount: 1,
      })
    ).toBeNull()
  })

  test('maps stable audit errors and hides unknown backend details', () => {
    expect(
      helpers.getIPAuditErrorMessageKey?.(
        new Error('ip_audit_projection_not_ready'),
        'Failed to load IP audit'
      )
    ).toBe('IP audit data is still being prepared. Please retry shortly.')
    expect(
      helpers.getIPAuditErrorMessageKey?.(
        new Error('ip_audit_forbidden'),
        'Failed to load IP audit'
      )
    ).toBe('You do not have permission to view this audit.')
    expect(
      helpers.getIPAuditErrorMessageKey?.(
        new Error('ip_audit_invalid_range'),
        'Failed to load IP audit'
      )
    ).toBe('Select a complete custom range.')
    expect(
      helpers.getIPAuditErrorMessageKey?.(
        new Error('SELECT users.password FROM users'),
        'Failed to load IP audit'
      )
    ).toBe('Failed to load IP audit')
  })

  test('uses the server capability to gate IP audit details', () => {
    expect(
      helpers.canShowSensitiveIPAuditDetails?.({
        is_root: false,
        can_view_details: true,
      })
    ).toBe(true)
    expect(
      helpers.canShowSensitiveIPAuditDetails?.({
        is_root: true,
        can_view_details: false,
      })
    ).toBe(false)
    expect(helpers.canShowSensitiveIPAuditDetails?.()).toBe(false)
  })

  test('labels the data scope independently from detail permission', () => {
    expect(
      helpers.getIPAuditScopeLabelKey?.({
        is_root: true,
        can_view_details: true,
      })
    ).toBe('Root view')
    expect(
      helpers.getIPAuditScopeLabelKey?.({
        is_root: false,
        can_view_details: true,
      })
    ).toBe('Admin masked view')
  })

  test('maps trusted-proxy risks to stable translation keys', () => {
    expect(
      helpers.getTrustedProxyWarningKey?.('trusted_proxies_not_configured')
    ).toBe(
      'Trusted proxies are not configured. Client IP addresses may be inaccurate behind a reverse proxy.'
    )
    expect(helpers.getTrustedProxyWarningKey?.('trusted_proxies_unsafe')).toBe(
      'Trusted proxy configuration accepts untrusted sources. Review TRUSTED_PROXIES before relying on IP audit data.'
    )
    expect(helpers.getTrustedProxyWarningKey?.('')).toBeNull()
  })

  test('labels infrastructure addresses without marking public client IPs', () => {
    expect(
      helpers.getIPAuditNetworkScopeLabelKey?.({ proxy_artifact: true })
    ).toBe('Infrastructure IP')
    expect(
      helpers.getIPAuditNetworkScopeLabelKey?.({ proxy_artifact: false })
    ).toBeNull()
  })

  test('maps stable risk reasons and exposes explicit detail actions', () => {
    expect(
      helpers.getIPAuditRiskReasonLabelKeys?.({
        risk_reasons: [
          'shared_registration_ip',
          'shared_api_ip',
          'unauthorized_burst',
          'unknown_reason',
        ],
      })
    ).toEqual(['Shared registration IP', 'Shared API IP', 'Unauthorized burst'])
    expect(typesSource).toContain('risk_reasons: string[]')
    expect(typesSource).toContain('can_query_external_profile: boolean')
    expect(typesSource).toContain('unauthorized_count: number')
    expect(tableSource).toContain('function IPAuditRiskReasons')
    expect(tableSource).toContain("t('View IP details')")
    expect(tableSource).toContain("from '@/components/ui/sheet'")
    expect(tableSource.match(/<IPAuditDetailPanel/g)).toHaveLength(1)
    expect(tableSource).toContain('buildIPThreatURL')
    expect(tableSource).toContain('buildUsageLogSearch')
    expect(tableSource).toContain("labelKey === 'Unauthorized burst'")
  })

  test('labels timeline events from stable kind rather than raw content', () => {
    expect(
      helpers.getIPAuditTimelineLabelKey?.({
        kind: 'login_failure',
        content: 'sensitive upstream failure detail',
      })
    ).toBe('Login failure')
    expect(
      helpers.getIPAuditTimelineLabelKey?.({
        kind: 'api_request',
        content: 'must not be rendered',
      })
    ).toBe('API request')
    expect(helpers.getIPAuditTimelineLabelKey?.({ kind: 'logout' })).toBe(
      'Logout'
    )
  })
})
