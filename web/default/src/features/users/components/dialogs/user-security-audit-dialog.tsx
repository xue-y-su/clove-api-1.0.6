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
import {
  AlertCircle,
  Clock3,
  Loader2,
  RefreshCw,
  ShieldCheck,
  ShieldOff,
  Users,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import {
  IPAuditPagination,
  IPAuditRangeSelector,
  TrustedProxyWarning,
} from '@/features/usage-logs/components/ip-audit-controls'
import {
  buildIPAuditQuery,
  formatIPAuditTimestamp,
  getIPAuditErrorMessageKey,
  getIPAuditPageCorrection,
  getIPAuditTimelineLabelKey,
  getIPAuditViewState,
  type IPAuditRange,
} from '@/features/usage-logs/lib/query-params'

import { getUserSecurityAudit } from '../../api'
import {
  canShowSensitiveUserSecurityDetails,
  getUserSecurityAuditPageCorrection,
  hasUserSecurityActivity,
  USER_SECURITY_AUDIT_COUNT_METRICS,
} from '../../lib/security-audit'
import type { User, UserSecurityAuditSummary } from '../../types'

type UserSecurityAuditDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: Pick<User, 'id' | 'username'>
}

function SummaryGrid({ summary }: { summary: UserSecurityAuditSummary }) {
  const { t, i18n } = useTranslation()
  const items = [
    { label: 'Registration IP', value: summary.registration_ip || '-' },
    { label: 'Recent IP', value: summary.recent_ip || '-' },
    ...USER_SECURITY_AUDIT_COUNT_METRICS.map((metric) => ({
      label: metric.labelKey,
      value: summary[metric.field].toLocaleString(),
      risk: metric.risk,
    })),
    {
      label: 'Related users',
      value: summary.related_user_count.toLocaleString(),
      risk: summary.related_user_count > 1,
    },
    {
      label: 'Last activity',
      value: formatIPAuditTimestamp(
        summary.last_activity_at,
        i18n.resolvedLanguage
      ),
    },
  ]

  return (
    <div className='grid overflow-hidden rounded-lg border sm:grid-cols-2 lg:grid-cols-4'>
      {items.map((item) => (
        <div
          key={item.label}
          className='border-b p-3 last:border-b-0 sm:border-r sm:nth-[2n]:border-r-0 lg:nth-[2n]:border-r lg:nth-[4n]:border-r-0 lg:nth-last-[-n+4]:border-b-0'
        >
          <div className='text-muted-foreground text-xs'>{t(item.label)}</div>
          <div
            className={
              item.risk
                ? 'text-destructive mt-1 font-medium break-all'
                : 'mt-1 font-medium break-all'
            }
          >
            {item.value}
          </div>
        </div>
      ))}
    </div>
  )
}

function LoadingState() {
  return (
    <div className='space-y-4'>
      <div className='grid gap-px overflow-hidden rounded-lg border sm:grid-cols-2 lg:grid-cols-4'>
        {Array.from({ length: 8 }, (_, index) => (
          <div key={index} className='space-y-2 border-b p-3'>
            <Skeleton className='h-3 w-20' />
            <Skeleton className='h-5 w-28' />
          </div>
        ))}
      </div>
      <Skeleton className='h-36 w-full' />
    </div>
  )
}

export function UserSecurityAuditDialog({
  open,
  onOpenChange,
  user,
}: UserSecurityAuditDialogProps) {
  const { t, i18n } = useTranslation()
  const [range, setRange] = useState<IPAuditRange>('30d')
  const [startTime, setStartTime] = useState<number>()
  const [endTime, setEndTime] = useState<number>()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [relatedPage, setRelatedPage] = useState(1)
  const [relatedPageSize, setRelatedPageSize] = useState(20)
  const customRangeValid =
    range !== 'custom' || Boolean(startTime && endTime && startTime <= endTime)

  const queryParams = useMemo(
    () =>
      buildIPAuditQuery({
        range,
        startTime,
        endTime,
        page,
        pageSize,
        relatedPage,
        relatedPageSize,
      }),
    [endTime, page, pageSize, range, relatedPage, relatedPageSize, startTime]
  )
  const auditQuery = useQuery({
    queryKey: ['user-security-audit', user.id, queryParams],
    enabled: open && customRangeValid,
    queryFn: async () => {
      const result = await getUserSecurityAudit(user.id, queryParams)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to load security audit'))
      }
      return result.data
    },
  })

  const data = auditQuery.data
  const pageCorrection = getUserSecurityAuditPageCorrection(data, page)
  const relatedPageCorrection =
    data && data.related_page === relatedPage
      ? getIPAuditPageCorrection({
          page: data.related_page,
          pageSize: data.related_page_size,
          total: data.related_total,
          itemCount: data.related_users.length,
        })
      : null
  useEffect(() => {
    if (pageCorrection !== null) setPage(pageCorrection)
  }, [pageCorrection])
  useEffect(() => {
    if (relatedPageCorrection !== null) setRelatedPage(relatedPageCorrection)
  }, [relatedPageCorrection])
  const showSensitive = canShowSensitiveUserSecurityDetails(data)
  const viewState = getIPAuditViewState({
    isLoading:
      auditQuery.isLoading ||
      pageCorrection !== null ||
      relatedPageCorrection !== null,
    isError: auditQuery.isError || !customRangeValid,
    enabled: data?.enabled,
    itemCount: hasUserSecurityActivity(data) ? 1 : 0,
  })

  const handleRangeChange = (nextRange: IPAuditRange) => {
    setPage(1)
    setRelatedPage(1)
    setRange(nextRange)
    if (
      nextRange === 'custom' &&
      !(startTime && endTime && startTime <= endTime)
    ) {
      const now = Date.now()
      setStartTime(now - 30 * 24 * 60 * 60 * 1000)
      setEndTime(now)
    }
    if (nextRange !== 'custom') {
      setStartTime(undefined)
      setEndTime(undefined)
    }
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
        variant={showSensitive ? 'default' : 'secondary'}
        className='gap-1.5'
      >
        <ShieldCheck className='size-3.5' />
        {showSensitive ? t('Root view') : t('Masked summary')}
      </Badge>
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='grid max-h-[calc(100vh-2rem)] grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden sm:max-w-5xl'>
        <DialogHeader>
          <div className='flex flex-wrap items-center gap-2 pe-8'>
            <DialogTitle>{t('Security & IP Audit')}</DialogTitle>
            {auditBadge}
          </div>
          <DialogDescription>
            {user.username} · #{user.id}
          </DialogDescription>
        </DialogHeader>

        <IPAuditRangeSelector
          range={range}
          startTime={startTime}
          endTime={endTime}
          disabled={auditQuery.isFetching || data?.enabled === false}
          loading={auditQuery.isFetching}
          onRangeChange={handleRangeChange}
          onCustomRangeChange={(nextRange) => {
            setPage(1)
            setRelatedPage(1)
            setStartTime(nextRange.start?.getTime())
            setEndTime(nextRange.end?.getTime())
          }}
        />

        <div className='min-h-0 space-y-4 overflow-y-auto pe-1'>
          {data?.enabled !== false ? (
            <TrustedProxyWarning warning={data?.proxy_warning} />
          ) : null}

          {viewState === 'loading' ? <LoadingState /> : null}

          {viewState === 'error' ? (
            <Alert variant='destructive'>
              <AlertCircle />
              <AlertTitle>{t('Failed to load security audit')}</AlertTitle>
              <AlertDescription className='flex flex-col items-start gap-3'>
                <span>
                  {customRangeValid
                    ? t(
                        getIPAuditErrorMessageKey(
                          auditQuery.error,
                          'Please try again.'
                        )
                      )
                    : t('Select a complete custom range.')}
                </span>
                {customRangeValid ? (
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => auditQuery.refetch()}
                  >
                    <RefreshCw />
                    {t('Retry')}
                  </Button>
                ) : null}
              </AlertDescription>
            </Alert>
          ) : null}

          {viewState === 'disabled' ? (
            <div className='text-muted-foreground flex min-h-44 flex-col items-center justify-center gap-2 rounded-lg border text-center text-sm'>
              <ShieldOff className='size-6' aria-hidden='true' />
              <div className='font-medium'>{t('IP audit is disabled')}</div>
              <div>
                {t(
                  'IP security event collection is disabled by the administrator.'
                )}
              </div>
            </div>
          ) : null}

          {viewState === 'empty' ? (
            <div className='text-muted-foreground flex min-h-44 flex-col items-center justify-center gap-2 rounded-lg border text-center text-sm'>
              <Clock3 className='size-6' />
              {t('No security activity in this range')}
            </div>
          ) : null}

          {viewState === 'ready' && data ? (
            <>
              <SummaryGrid summary={data.summary} />
              <div className='text-muted-foreground flex min-w-0 items-start gap-2 text-xs'>
                <ShieldCheck
                  className='mt-0.5 size-3.5 shrink-0'
                  aria-hidden='true'
                />
                <span>
                  {t(
                    'API requests are attributed to the API Key owner. Authentication events are attributed to the acting account.'
                  )}
                </span>
              </div>

              {showSensitive ? (
                <section className='space-y-2'>
                  <div className='flex items-center gap-2 text-sm font-medium'>
                    <Users className='size-4' />
                    {t('Related accounts')}
                    <Badge variant='secondary'>{data.related_total}</Badge>
                  </div>
                  {data.related_users.length > 0 ? (
                    <div className='divide-y overflow-hidden rounded-lg border'>
                      {data.related_users.map((relatedUser) => (
                        <div
                          key={relatedUser.user_id}
                          className='grid gap-2 px-3 py-2.5 text-sm md:grid-cols-[minmax(9rem,1fr)_minmax(0,2fr)] md:items-center'
                        >
                          <div className='min-w-0'>
                            <div className='font-medium [overflow-wrap:anywhere]'>
                              {relatedUser.username ||
                                `#${relatedUser.user_id}`}
                            </div>
                            <div className='text-muted-foreground text-xs'>
                              #{relatedUser.user_id} ·{' '}
                              {formatIPAuditTimestamp(
                                relatedUser.last_seen_at,
                                i18n.resolvedLanguage
                              )}
                            </div>
                          </div>
                          <div className='flex min-w-0 flex-wrap gap-1.5'>
                            <Badge variant='secondary'>
                              {t('Shared IPs')}: {relatedUser.shared_ip_count}
                            </Badge>
                            {relatedUser.registration_count > 0 ? (
                              <Badge variant='destructive'>
                                {t('Registrations')}:{' '}
                                {relatedUser.registration_count}
                              </Badge>
                            ) : null}
                            {relatedUser.api_call_count > 0 ? (
                              <Badge variant='outline'>
                                {t('API calls')}: {relatedUser.api_call_count}
                              </Badge>
                            ) : null}
                            {relatedUser.authentication_count > 0 ? (
                              <Badge variant='outline'>
                                {t('Authentication events')}:{' '}
                                {relatedUser.authentication_count}
                              </Badge>
                            ) : null}
                            {relatedUser.anomaly_count > 0 ? (
                              <Badge variant='destructive'>
                                {t('Anomalies')}: {relatedUser.anomaly_count}
                              </Badge>
                            ) : null}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className='text-muted-foreground rounded-lg border px-3 py-4 text-sm'>
                      {t('No related accounts')}
                    </div>
                  )}
                  <IPAuditPagination
                    page={data.related_page}
                    pageSize={data.related_page_size}
                    total={data.related_total}
                    disabled={auditQuery.isFetching}
                    onPageChange={setRelatedPage}
                    onPageSizeChange={(nextPageSize) => {
                      setRelatedPage(1)
                      setRelatedPageSize(nextPageSize)
                    }}
                  />
                </section>
              ) : null}

              {showSensitive ? (
                <section className='overflow-hidden rounded-lg border'>
                  <div className='border-b px-3 py-2 text-sm font-medium'>
                    {t('Security timeline')}
                  </div>
                  {data.timeline.length > 0 ? (
                    <div className='divide-y'>
                      {data.timeline.map((item, index) => (
                        <div
                          key={`${item.created_at}-${item.kind}-${item.request_id ?? index}`}
                          className='grid gap-2 px-3 py-2.5 text-sm md:grid-cols-[10rem_9rem_1fr] md:items-center'
                        >
                          <span className='text-muted-foreground tabular-nums'>
                            {formatIPAuditTimestamp(
                              item.created_at,
                              i18n.resolvedLanguage
                            )}
                          </span>
                          <Badge variant='outline' className='w-fit'>
                            {t(getIPAuditTimelineLabelKey(item))}
                          </Badge>
                          <div className='min-w-0'>
                            <div className='font-mono break-all'>
                              {item.ip || '-'}
                            </div>
                            {item.request_id ? (
                              <div className='text-muted-foreground truncate font-mono text-xs'>
                                {item.request_id}
                              </div>
                            ) : null}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className='text-muted-foreground px-3 py-8 text-center text-sm'>
                      {t('No timeline entries')}
                    </div>
                  )}
                  <IPAuditPagination
                    page={data.page}
                    pageSize={data.page_size}
                    total={data.total}
                    disabled={auditQuery.isFetching}
                    onPageChange={setPage}
                    onPageSizeChange={(nextPageSize) => {
                      setPage(1)
                      setPageSize(nextPageSize)
                    }}
                  />
                </section>
              ) : null}
            </>
          ) : null}

          {auditQuery.isFetching && viewState === 'ready' ? (
            <div className='text-muted-foreground flex items-center justify-center gap-2 py-1 text-xs'>
              <Loader2 className='size-3.5 animate-spin' />
              {t('Loading...')}
            </div>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  )
}
