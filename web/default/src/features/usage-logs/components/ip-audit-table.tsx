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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import {
  ExternalLink,
  Loader2,
  RefreshCw,
  ShieldCheck,
  ShieldOff,
  TriangleAlert,
} from 'lucide-react'
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import { getIPAudit, getIPAuditDetail } from '../api'
import { buildIPThreatURL, buildUsageLogSearch } from '../lib/ip-audit-links'
import {
  buildIPAuditQuery,
  canShowSensitiveIPAuditDetails,
  formatIPAuditTimestamp,
  getIPAuditErrorMessageKey,
  getIPAuditNetworkScopeLabelKey,
  getIPAuditPageCorrection,
  getIPAuditRiskReasonLabelKeys,
  getIPAuditScopeLabelKey,
  getIPAuditTimelineLabelKey,
  getIPAuditViewState,
  IP_AUDIT_COUNT_METRICS,
  isIPAuditRange,
  isIPAuditView,
  type IPAuditRange,
  type IPAuditView,
} from '../lib/query-params'
import type { IPAuditDetail, IPAuditItem, IPAuditTimelineItem } from '../types'
import {
  IPAuditPagination,
  IPAuditRangeSelector,
  IPAuditViewSelector,
  TrustedProxyWarning,
} from './ip-audit-controls'

const route = getRouteApi('/_authenticated/security-audit/')

function AccountAttributionCell(props: { item: IPAuditItem }) {
  const { t } = useTranslation()

  return (
    <div className='grid max-w-[18rem] min-w-32 gap-0.5 overflow-hidden text-xs tabular-nums'>
      <span className='whitespace-nowrap'>
        {t('Registration accounts')}: {props.item.registration_user_count}
      </span>
      <span className='whitespace-nowrap'>
        {t('API accounts')}: {props.item.api_user_count}
      </span>
      <span className='whitespace-nowrap'>
        {t('Authentication accounts')}: {props.item.authentication_user_count}
      </span>
      <span className='text-muted-foreground whitespace-nowrap'>
        {t('Total accounts')}: {props.item.user_count}
      </span>
    </div>
  )
}

function IPAuditAddress(props: { item: IPAuditItem }) {
  const { t } = useTranslation()
  const scopeLabelKey = getIPAuditNetworkScopeLabelKey(props.item)
  const threatURL = buildIPThreatURL(
    props.item.ip,
    props.item.can_query_external_profile
  )

  return (
    <div className='grid min-w-0 gap-1'>
      <div className='flex min-w-0 items-start gap-1'>
        <span
          title={props.item.ip}
          className='block min-w-0 truncate font-mono font-medium'
        >
          {props.item.ip}
        </span>
        {threatURL ? (
          <a
            href={threatURL}
            target='_blank'
            rel='noreferrer noopener'
            className='text-muted-foreground hover:text-foreground mt-0.5 shrink-0'
            aria-label={t('View external IP profile')}
            title={t('View external IP profile')}
          >
            <ExternalLink className='size-3.5' aria-hidden='true' />
          </a>
        ) : null}
      </div>
      {scopeLabelKey ? (
        <Badge variant='outline' className='w-fit max-w-full whitespace-normal'>
          {t(scopeLabelKey)}
        </Badge>
      ) : null}
    </div>
  )
}

function IPAuditRiskReasons(props: { item: IPAuditItem }) {
  const { t } = useTranslation()
  const reasonLabelKeys = getIPAuditRiskReasonLabelKeys(props.item)

  return (
    <div className='flex min-w-0 flex-wrap gap-1'>
      {reasonLabelKeys.map((labelKey) => {
        let variant: 'destructive' | 'secondary' | 'outline' = 'outline'
        if (
          labelKey === 'Anomalous activity' ||
          labelKey === 'Unauthorized burst'
        ) {
          variant = 'destructive'
        } else if (labelKey === 'Infrastructure IP') {
          variant = 'secondary'
        }
        return (
          <Badge
            key={labelKey}
            variant={variant}
            className='max-w-full whitespace-normal'
          >
            {t(labelKey)}
          </Badge>
        )
      })}
    </div>
  )
}

function IPAuditDetailPanel(props: {
  detail?: IPAuditDetail
  isLoading: boolean
  isError: boolean
  onRetry: () => void
  onUserPageChange: (page: number) => void
  onUserPageSizeChange: (pageSize: number) => void
  onTimelinePageChange: (page: number) => void
  onTimelinePageSizeChange: (pageSize: number) => void
  onRequestIDSearch: (requestID: string, createdAt: number) => void
}) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return (
      <div
        className='text-muted-foreground flex min-h-20 items-center justify-center'
        role='status'
        aria-live='polite'
      >
        <Loader2 className='size-5 animate-spin' aria-hidden='true' />
        <span className='sr-only'>{t('Loading...')}</span>
      </div>
    )
  }
  if (props.isError) {
    return (
      <Alert variant='destructive'>
        <TriangleAlert />
        <AlertTitle>{t('Failed to load IP audit')}</AlertTitle>
        <AlertDescription className='flex items-center justify-between gap-3'>
          <span>{t('Please try again.')}</span>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={props.onRetry}
          >
            <RefreshCw />
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }
  if (!props.detail) {
    return null
  }

  return (
    <div className='space-y-4'>
      <div className='text-muted-foreground flex min-w-0 items-start gap-2 text-xs'>
        <ShieldCheck className='mt-0.5 size-3.5 shrink-0' aria-hidden='true' />
        <span>
          {t(
            'API requests are attributed to the API Key owner. Authentication events are attributed to the acting account.'
          )}
        </span>
      </div>
      <section className='min-w-0 border-t pt-3'>
        <div className='mb-2 flex items-center gap-2 text-sm font-medium'>
          <span>{t('Related accounts')}</span>
          <Badge variant='secondary'>{props.detail.user_total}</Badge>
        </div>
        {props.detail.users.length > 0 ? (
          <div className='flex flex-wrap gap-1.5 pb-2'>
            {props.detail.users.map((user) => (
              <Badge
                key={user.user_id}
                variant='secondary'
                className='max-w-full [overflow-wrap:anywhere] whitespace-normal'
              >
                {user.username || `#${user.user_id}`} · #{user.user_id}
              </Badge>
            ))}
          </div>
        ) : (
          <div className='text-muted-foreground py-3 text-sm'>
            {t('No related accounts')}
          </div>
        )}
        <IPAuditPagination
          page={props.detail.user_page}
          pageSize={props.detail.user_page_size}
          total={props.detail.user_total}
          onPageChange={props.onUserPageChange}
          onPageSizeChange={props.onUserPageSizeChange}
        />
      </section>
      <section className='min-w-0 border-t pt-3'>
        <div className='mb-2 text-sm font-medium'>{t('Security timeline')}</div>
        <TimelineRows
          timeline={props.detail.timeline}
          onRequestIDSearch={props.onRequestIDSearch}
        />
        <IPAuditPagination
          page={props.detail.timeline_page}
          pageSize={props.detail.timeline_page_size}
          total={props.detail.timeline_total}
          onPageChange={props.onTimelinePageChange}
          onPageSizeChange={props.onTimelinePageSizeChange}
        />
      </section>
    </div>
  )
}

function TimelineRows(props: {
  timeline?: IPAuditTimelineItem[]
  onRequestIDSearch: (requestID: string, createdAt: number) => void
}) {
  const { t, i18n } = useTranslation()
  const timeline = props.timeline ?? []
  if (timeline.length === 0) {
    return (
      <div className='text-muted-foreground px-2 py-3 text-sm'>
        {t('No timeline entries')}
      </div>
    )
  }

  return (
    <div className='divide-y rounded-md border'>
      {timeline.map((entry, index) => (
        <div
          key={`${entry.created_at}-${entry.user_id}-${entry.request_id ?? index}`}
          className='grid gap-2 px-3 py-2 text-sm md:grid-cols-[10rem_10rem_9rem_1fr] md:items-center'
        >
          <span className='text-muted-foreground tabular-nums'>
            {formatIPAuditTimestamp(entry.created_at, i18n.resolvedLanguage)}
          </span>
          <span className='font-medium'>
            {entry.username || `#${entry.user_id}`}
          </span>
          <Badge variant='outline' className='w-fit'>
            {t(getIPAuditTimelineLabelKey(entry))}
          </Badge>
          {entry.request_id ? (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='break-anywhere h-auto min-w-0 justify-start px-0 font-mono text-xs'
              onClick={() =>
                props.onRequestIDSearch(
                  entry.request_id ?? '',
                  entry.created_at
                )
              }
            >
              {entry.request_id}
            </Button>
          ) : (
            <span className='text-muted-foreground font-mono text-xs'>-</span>
          )}
        </div>
      ))}
    </div>
  )
}

function CountValue({
  value,
  risk = false,
}: {
  value: number
  risk?: boolean
}) {
  if (risk && value > 0) {
    return <Badge variant='destructive'>{value.toLocaleString()}</Badge>
  }
  return <span className='tabular-nums'>{value.toLocaleString()}</span>
}

function MobileAuditCard(props: {
  item: IPAuditItem
  showSensitive: boolean
  onOpenDetails: () => void
}) {
  const { t, i18n } = useTranslation()
  const metrics = IP_AUDIT_COUNT_METRICS.map((metric) => ({
    ...metric,
    value: props.item[metric.field],
  }))

  return (
    <div className='bg-card rounded-lg border p-3'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <IPAuditAddress item={props.item} />
          <div className='text-muted-foreground mt-1 text-xs tabular-nums'>
            {formatIPAuditTimestamp(
              props.item.first_seen_at,
              i18n.resolvedLanguage
            )}{' '}
            -{' '}
            {formatIPAuditTimestamp(
              props.item.last_seen_at,
              i18n.resolvedLanguage
            )}
          </div>
        </div>
      </div>

      <div className='mt-3'>
        <IPAuditRiskReasons item={props.item} />
      </div>

      <div className='mt-3 grid grid-cols-2 gap-x-3 gap-y-2 sm:grid-cols-3'>
        {metrics.map((metric) => (
          <div key={metric.field}>
            <div className='text-muted-foreground text-xs'>
              {t(metric.labelKey)}
            </div>
            <div className='mt-0.5 text-sm font-medium'>
              <CountValue value={metric.value} risk={metric.risk} />
            </div>
          </div>
        ))}
      </div>

      <div className='mt-3 flex flex-wrap items-center justify-between gap-2 border-t pt-3 text-sm'>
        <AccountAttributionCell item={props.item} />
        <span className='text-muted-foreground'>
          {t('Quota')}: {formatQuota(props.item.quota)}
        </span>
      </div>
      {props.showSensitive ? (
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='mt-3 w-full'
          onClick={props.onOpenDetails}
        >
          {t('View IP details')}
        </Button>
      ) : null}
    </div>
  )
}

export function IPAuditTable() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const searchParams = route.useSearch()
  const [selectedIP, setSelectedIP] = useState<string | null>(null)
  const [detailUserPage, setDetailUserPage] = useState(1)
  const [detailUserPageSize, setDetailUserPageSize] = useState(20)
  const [detailTimelinePage, setDetailTimelinePage] = useState(1)
  const [detailTimelinePageSize, setDetailTimelinePageSize] = useState(20)

  const range: IPAuditRange = isIPAuditRange(searchParams.range)
    ? searchParams.range
    : '30d'
  const auditView: IPAuditView = isIPAuditView(searchParams.view)
    ? searchParams.view
    : 'risk'
  const page = Math.max(1, searchParams.page ?? 1)
  const pageSize = Math.min(100, Math.max(1, searchParams.pageSize ?? 20))
  const customRangeValid =
    range !== 'custom' ||
    Boolean(
      searchParams.startTime &&
      searchParams.endTime &&
      searchParams.startTime <= searchParams.endTime
    )

  const queryParams = useMemo(
    () =>
      buildIPAuditQuery({
        range,
        view: auditView,
        page,
        pageSize,
        startTime: searchParams.startTime,
        endTime: searchParams.endTime,
      }),
    [
      auditView,
      page,
      pageSize,
      range,
      searchParams.endTime,
      searchParams.startTime,
    ]
  )

  const detailQueryParams = useMemo(
    () =>
      buildIPAuditQuery({
        range,
        view: auditView,
        page,
        pageSize,
        startTime: searchParams.startTime,
        endTime: searchParams.endTime,
        userPage: detailUserPage,
        userPageSize: detailUserPageSize,
        timelinePage: detailTimelinePage,
        timelinePageSize: detailTimelinePageSize,
      }),
    [
      auditView,
      detailTimelinePage,
      detailTimelinePageSize,
      detailUserPage,
      detailUserPageSize,
      page,
      pageSize,
      range,
      searchParams.endTime,
      searchParams.startTime,
    ]
  )

  const auditQuery = useQuery({
    queryKey: ['ip-audit', queryParams],
    enabled: customRangeValid,
    queryFn: async () => {
      const result = await getIPAudit(queryParams)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to load IP audit'))
      }
      return result.data
    },
  })

  const data = auditQuery.data
  const items = data?.items ?? []
  const total = data?.total ?? 0
  const showSensitive = canShowSensitiveIPAuditDetails(data)
  const detailQuery = useQuery({
    queryKey: ['ip-audit-detail', selectedIP, detailQueryParams],
    enabled: showSensitive && selectedIP !== null,
    queryFn: async () => {
      if (selectedIP === null) {
        throw new Error(t('Failed to load IP audit'))
      }
      const result = await getIPAuditDetail(selectedIP, detailQueryParams)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to load IP audit'))
      }
      return result.data
    },
  })
  const pageCorrection =
    data?.page === page
      ? getIPAuditPageCorrection({
          page: data.page,
          pageSize: data.page_size,
          total,
          itemCount: items.length,
        })
      : null
  const viewState = getIPAuditViewState({
    isLoading: auditQuery.isLoading || pageCorrection !== null,
    isError: auditQuery.isError || !customRangeValid,
    enabled: data?.enabled,
    itemCount: items.length,
  })
  const desktopColSpan = showSensitive ? 15 : 14

  const updateSearch = useCallback(
    (updates: Record<string, unknown>) => {
      void navigate({
        to: '/security-audit',
        search: { ...searchParams, ...updates },
      })
      setSelectedIP(null)
      setDetailUserPage(1)
      setDetailTimelinePage(1)
    },
    [navigate, searchParams]
  )

  const openIPDetails = useCallback((ip: string) => {
    setDetailUserPage(1)
    setDetailTimelinePage(1)
    setSelectedIP(ip)
  }, [])

  const handleTimelineRequestSearch = useCallback(
    (requestID: string, createdAt: number) => {
      const search = buildUsageLogSearch(requestID, createdAt)
      if (!search.requestId) return
      void navigate({
        to: '/usage-logs/$section',
        params: { section: 'common' },
        search,
      })
    },
    [navigate]
  )

  useEffect(() => {
    if (pageCorrection !== null && page !== pageCorrection) {
      updateSearch({ page: pageCorrection })
    }
  }, [page, pageCorrection, updateSearch])

  const handleRangeChange = useCallback(
    (nextRange: IPAuditRange) => {
      if (nextRange === 'custom') {
        const now = Date.now()
        const hasValidDates =
          searchParams.startTime &&
          searchParams.endTime &&
          searchParams.startTime <= searchParams.endTime
        updateSearch({
          range: nextRange,
          page: 1,
          startTime: hasValidDates
            ? searchParams.startTime
            : now - 30 * 24 * 60 * 60 * 1000,
          endTime: hasValidDates ? searchParams.endTime : now,
        })
        return
      }
      updateSearch({
        range: nextRange,
        page: 1,
        startTime: undefined,
        endTime: undefined,
      })
    },
    [searchParams.endTime, searchParams.startTime, updateSearch]
  )

  const handleCustomRangeChange = useCallback(
    (nextRange: { start?: Date; end?: Date }) => {
      updateSearch({
        range: 'custom',
        page: 1,
        startTime: nextRange.start?.getTime(),
        endTime: nextRange.end?.getTime(),
      })
    },
    [updateSearch]
  )

  let desktopRows: ReactNode
  if (viewState === 'loading') {
    desktopRows = (
      <TableRow>
        <TableCell colSpan={desktopColSpan} className='h-36 text-center'>
          <Loader2 className='text-muted-foreground mx-auto size-5 animate-spin' />
          <span className='sr-only'>{t('Loading...')}</span>
        </TableCell>
      </TableRow>
    )
  } else if (viewState === 'error') {
    desktopRows = (
      <TableRow>
        <TableCell colSpan={desktopColSpan} className='h-40'>
          <div className='mx-auto flex max-w-lg flex-col items-center gap-3 text-center'>
            <TriangleAlert className='text-destructive size-6' />
            <div>
              <div className='font-medium'>{t('Failed to load IP audit')}</div>
              <div className='text-muted-foreground mt-1 text-sm'>
                {customRangeValid
                  ? t(
                      getIPAuditErrorMessageKey(
                        auditQuery.error,
                        'Please try again.'
                      )
                    )
                  : t('Select a complete custom range.')}
              </div>
            </div>
            {customRangeValid ? (
              <Button
                type='button'
                variant='outline'
                onClick={() => auditQuery.refetch()}
              >
                <RefreshCw />
                {t('Retry')}
              </Button>
            ) : null}
          </div>
        </TableCell>
      </TableRow>
    )
  } else if (viewState === 'disabled') {
    desktopRows = (
      <TableRow>
        <TableCell colSpan={desktopColSpan} className='h-40'>
          <div className='text-muted-foreground mx-auto flex max-w-lg flex-col items-center gap-2 text-center'>
            <ShieldOff className='size-6' aria-hidden='true' />
            <div className='font-medium'>{t('IP audit is disabled')}</div>
            <div className='text-sm'>
              {t(
                'IP security event collection is disabled by the administrator.'
              )}
            </div>
          </div>
        </TableCell>
      </TableRow>
    )
  } else if (viewState === 'empty') {
    desktopRows = (
      <TableRow>
        <TableCell
          colSpan={desktopColSpan}
          className='text-muted-foreground h-36 text-center'
        >
          {t('No IP audit records')}
        </TableCell>
      </TableRow>
    )
  } else {
    desktopRows = items.map((item) => (
      <TableRow key={item.ip}>
        <TableCell className='min-w-0 overflow-hidden'>
          <IPAuditAddress item={item} />
        </TableCell>
        <TableCell>
          <CountValue value={item.request_count} />
        </TableCell>
        <TableCell>
          <CountValue value={item.registration_count} />
        </TableCell>
        <TableCell>
          <CountValue value={item.api_request_count} />
        </TableCell>
        <TableCell>
          <CountValue value={item.unauthorized_count} risk />
        </TableCell>
        <TableCell>
          <CountValue value={item.login_success_count} />
        </TableCell>
        <TableCell>
          <CountValue value={item.login_failure_count} risk />
        </TableCell>
        <TableCell>
          <CountValue value={item.logout_count} />
        </TableCell>
        <TableCell>
          <CountValue value={item.anomaly_count} risk />
        </TableCell>
        <TableCell>
          <IPAuditRiskReasons item={item} />
        </TableCell>
        <TableCell>
          <AccountAttributionCell item={item} />
        </TableCell>
        <TableCell className='whitespace-nowrap'>
          {formatQuota(item.quota)}
        </TableCell>
        <TableCell className='whitespace-nowrap tabular-nums'>
          {formatIPAuditTimestamp(item.first_seen_at, i18n.resolvedLanguage)}
        </TableCell>
        <TableCell className='whitespace-nowrap tabular-nums'>
          {formatIPAuditTimestamp(item.last_seen_at, i18n.resolvedLanguage)}
        </TableCell>
        {showSensitive ? (
          <TableCell className='bg-background sticky right-0 z-10'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='whitespace-nowrap'
              onClick={() => openIPDetails(item.ip)}
            >
              {t('View IP details')}
            </Button>
          </TableCell>
        ) : null}
      </TableRow>
    ))
  }

  let mobileRows: ReactNode
  if (viewState === 'ready') {
    mobileRows = items.map((item) => (
      <MobileAuditCard
        key={item.ip}
        item={item}
        showSensitive={showSensitive}
        onOpenDetails={() => openIPDetails(item.ip)}
      />
    ))
  } else if (viewState === 'loading') {
    mobileRows = (
      <div
        className='text-muted-foreground flex h-36 items-center justify-center'
        role='status'
        aria-live='polite'
      >
        <Loader2 className='size-5 animate-spin' aria-hidden='true' />
        <span className='sr-only'>{t('Loading...')}</span>
      </div>
    )
  } else if (viewState === 'error') {
    mobileRows = (
      <Alert variant='destructive'>
        <TriangleAlert />
        <AlertTitle>{t('Failed to load IP audit')}</AlertTitle>
        <AlertDescription>
          {customRangeValid
            ? t(
                getIPAuditErrorMessageKey(auditQuery.error, 'Please try again.')
              )
            : t('Select a complete custom range.')}
        </AlertDescription>
      </Alert>
    )
  } else if (viewState === 'disabled') {
    mobileRows = (
      <div className='text-muted-foreground flex h-36 flex-col items-center justify-center gap-2 text-center text-sm'>
        <ShieldOff className='size-6' aria-hidden='true' />
        <span className='font-medium'>{t('IP audit is disabled')}</span>
        <span>
          {t('IP security event collection is disabled by the administrator.')}
        </span>
      </div>
    )
  } else {
    mobileRows = (
      <div className='text-muted-foreground flex h-36 items-center justify-center text-sm'>
        {t('No IP audit records')}
      </div>
    )
  }

  let auditBadge: ReactNode = null
  if (data?.enabled === false) {
    auditBadge = (
      <Badge variant='secondary' className='gap-1.5'>
        <ShieldOff className='size-3.5' />
        {t('IP audit is disabled')}
      </Badge>
    )
  } else if (data) {
    auditBadge = (
      <Badge
        variant={data.is_root ? 'default' : 'secondary'}
        className='gap-1.5'
      >
        <ShieldCheck className='size-3.5' />
        {t(getIPAuditScopeLabelKey(data))}
      </Badge>
    )
  }

  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <div className='flex flex-col gap-2 rounded-lg border p-2.5 sm:p-3 lg:flex-row lg:items-start'>
        <div className='min-w-0 flex-1 space-y-2'>
          <IPAuditViewSelector
            view={auditView}
            disabled={auditQuery.isFetching || data?.enabled === false}
            onViewChange={(nextView) =>
              updateSearch({ view: nextView, page: 1 })
            }
          />
          <IPAuditRangeSelector
            range={range}
            startTime={searchParams.startTime}
            endTime={searchParams.endTime}
            disabled={auditQuery.isFetching || data?.enabled === false}
            loading={auditQuery.isFetching}
            onRangeChange={handleRangeChange}
            onCustomRangeChange={handleCustomRangeChange}
          />
        </div>
        <div className='flex items-center gap-2 lg:ms-auto'>
          {auditBadge}
          <Button
            type='button'
            variant='outline'
            size='icon'
            disabled={auditQuery.isFetching || !customRangeValid}
            onClick={() => auditQuery.refetch()}
          >
            <RefreshCw
              className={cn(auditQuery.isFetching && 'animate-spin')}
            />
            <span className='sr-only'>{t('Refresh')}</span>
          </Button>
        </div>
      </div>

      {data?.enabled !== false ? (
        <TrustedProxyWarning warning={data?.proxy_warning} />
      ) : null}

      {data?.enabled !== false && data?.sync?.ready === false ? (
        <Alert data-nonblocking-progress='true'>
          {data.sync.status === 'unknown' ? (
            <TriangleAlert className='size-4' aria-hidden='true' />
          ) : (
            <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          )}
          <AlertTitle>
            {data.sync.status === 'unknown'
              ? t('IP audit sync status is temporarily unavailable')
              : t('IP audit data is syncing')}
          </AlertTitle>
          <AlertDescription>
            {data.sync.status === 'unknown' ? (
              t(
                'Available audit results are shown. Refresh later to check sync status.'
              )
            ) : (
              <>
                {t(
                  'Historical IP audit data is syncing. Available results are shown now.'
                )}{' '}
                {t('Processed {{count}} historical log records.', {
                  count: data.sync.processed_logs.toLocaleString(),
                })}
              </>
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className='flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg border'>
        <div className='min-h-0 flex-1 overflow-auto'>
          <div className='hidden md:block'>
            <Table className='min-w-[2200px] table-fixed'>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-60'>{t('IP')}</TableHead>
                  <TableHead className='w-24'>{t('Requests')}</TableHead>
                  <TableHead className='w-24'>{t('Registrations')}</TableHead>
                  <TableHead className='w-24'>{t('API requests')}</TableHead>
                  <TableHead className='w-36 whitespace-normal'>
                    {t('Unauthorized (401)')}
                  </TableHead>
                  <TableHead className='w-28 whitespace-normal'>
                    {t('Login success')}
                  </TableHead>
                  <TableHead className='w-28 whitespace-normal'>
                    {t('Login failure')}
                  </TableHead>
                  <TableHead className='w-20'>{t('Logouts')}</TableHead>
                  <TableHead className='w-20'>{t('Anomalies')}</TableHead>
                  <TableHead className='w-64'>{t('Risk')}</TableHead>
                  <TableHead className='w-56'>
                    {t('Account attribution')}
                  </TableHead>
                  <TableHead className='w-28'>{t('Quota')}</TableHead>
                  <TableHead className='w-48 whitespace-nowrap'>
                    {t('First Seen')}
                  </TableHead>
                  <TableHead className='w-48 whitespace-nowrap'>
                    {t('Last Seen')}
                  </TableHead>
                  {showSensitive ? (
                    <TableHead className='bg-background sticky right-0 z-10 w-44'>
                      {t('Actions')}
                    </TableHead>
                  ) : null}
                </TableRow>
              </TableHeader>
              <TableBody>{desktopRows}</TableBody>
            </Table>
          </div>
          <div className='grid gap-2 p-2 md:hidden'>{mobileRows}</div>
        </div>
        {viewState === 'ready' || viewState === 'empty' ? (
          <IPAuditPagination
            page={data?.page ?? page}
            pageSize={data?.page_size ?? pageSize}
            total={total}
            disabled={auditQuery.isFetching}
            onPageChange={(nextPage) => updateSearch({ page: nextPage })}
            onPageSizeChange={(nextPageSize) =>
              updateSearch({ page: 1, pageSize: nextPageSize })
            }
          />
        ) : null}
      </div>

      <Sheet
        open={selectedIP !== null}
        onOpenChange={(open) => {
          if (!open) setSelectedIP(null)
        }}
      >
        <SheetContent className='w-full sm:max-w-2xl'>
          <SheetHeader className='border-b pe-14'>
            <SheetTitle>{t('IP audit details')}</SheetTitle>
            <SheetDescription className='break-anywhere font-mono'>
              {selectedIP}
            </SheetDescription>
          </SheetHeader>
          <div className='min-h-0 flex-1 overflow-y-auto px-4 pb-4'>
            <IPAuditDetailPanel
              detail={detailQuery.data}
              isLoading={detailQuery.isLoading}
              isError={detailQuery.isError}
              onRetry={() => void detailQuery.refetch()}
              onUserPageChange={setDetailUserPage}
              onUserPageSizeChange={(nextPageSize) => {
                setDetailUserPage(1)
                setDetailUserPageSize(nextPageSize)
              }}
              onTimelinePageChange={setDetailTimelinePage}
              onTimelinePageSizeChange={(nextPageSize) => {
                setDetailTimelinePage(1)
                setDetailTimelinePageSize(nextPageSize)
              }}
              onRequestIDSearch={handleTimelineRequestSearch}
            />
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}
