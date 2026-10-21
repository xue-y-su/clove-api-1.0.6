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
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn, getPageNumbers } from '@/lib/utils'

import {
  getIPAuditPageCount,
  getTrustedProxyWarningKey,
  IP_AUDIT_RANGE_OPTIONS,
  IP_AUDIT_VIEW_OPTIONS,
  isIPAuditView,
  type IPAuditRange,
  type IPAuditView,
} from '../lib/query-params'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'

type IPAuditRangeSelectorProps = {
  range: IPAuditRange
  startTime?: number
  endTime?: number
  disabled?: boolean
  loading?: boolean
  onRangeChange: (range: IPAuditRange) => void
  onCustomRangeChange: (range: { start?: Date; end?: Date }) => void
  className?: string
}

export function IPAuditRangeSelector({
  range,
  startTime,
  endTime,
  disabled,
  loading,
  onRangeChange,
  onCustomRangeChange,
  className,
}: IPAuditRangeSelectorProps) {
  const { t } = useTranslation()
  const selectedOption =
    IP_AUDIT_RANGE_OPTIONS.find((option) => option.value === range) ??
    IP_AUDIT_RANGE_OPTIONS[0]

  return (
    <div className={cn('flex min-w-0 flex-col gap-2', className)}>
      <div
        className='bg-muted flex w-fit max-w-full flex-wrap items-center gap-0.5 rounded-lg p-0.5'
        role='group'
        aria-label={t('Audit range')}
      >
        {IP_AUDIT_RANGE_OPTIONS.map((option) => (
          <Button
            key={option.value}
            type='button'
            size='sm'
            variant={range === option.value ? 'default' : 'ghost'}
            className='h-7 rounded-md px-2 text-xs shadow-none'
            disabled={disabled}
            aria-pressed={range === option.value}
            onClick={() => onRangeChange(option.value)}
          >
            {t(option.labelKey)}
          </Button>
        ))}
      </div>
      <div
        className='flex min-h-5 flex-wrap items-center gap-x-3 gap-y-1 text-xs'
        aria-live='polite'
      >
        <span className='text-muted-foreground'>
          {t('Audit range')}:{' '}
          <span className='text-foreground font-medium'>
            {t(selectedOption.labelKey)}
          </span>
        </span>
        {loading ? (
          <span className='text-muted-foreground inline-flex items-center gap-1'>
            <Loader2 className='size-3 animate-spin' aria-hidden='true' />
            {t('Loading...')}
          </span>
        ) : null}
      </div>
      {range === 'custom' ? (
        <CompactDateTimeRangePicker
          start={startTime ? new Date(startTime) : undefined}
          end={endTime ? new Date(endTime) : undefined}
          onChange={onCustomRangeChange}
          className='w-full sm:w-[24rem]'
        />
      ) : null}
    </div>
  )
}

export function IPAuditViewSelector(props: {
  view: IPAuditView
  disabled?: boolean
  onViewChange: (view: IPAuditView) => void
}) {
  const { t } = useTranslation()

  return (
    <Tabs
      value={props.view}
      onValueChange={(value) => {
        if (isIPAuditView(value)) props.onViewChange(value)
      }}
    >
      <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
        {IP_AUDIT_VIEW_OPTIONS.map((option) => (
          <TabsTrigger
            key={option.value}
            value={option.value}
            disabled={props.disabled}
            className='min-h-7 flex-none px-2 text-xs'
          >
            {t(option.labelKey)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const
const PAGE_SIZE_ITEMS = PAGE_SIZE_OPTIONS.map((pageSize) => ({
  value: String(pageSize),
  label: String(pageSize),
}))

type IPAuditPaginationProps = {
  page: number
  pageSize: number
  total: number
  disabled?: boolean
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}

export function IPAuditPagination({
  page,
  pageSize,
  total,
  disabled,
  onPageChange,
  onPageSizeChange,
}: IPAuditPaginationProps) {
  const { t } = useTranslation()
  const totalPages = getIPAuditPageCount(total, pageSize)
  const currentPage = Math.min(Math.max(1, page), totalPages)
  const pageNumbers = getPageNumbers(currentPage, totalPages)
  let ellipsisCount = 0
  const pageEntries = pageNumbers.map((value) => ({
    value,
    key:
      value === '...'
        ? `ellipsis-${(ellipsisCount += 1)}`
        : `page-${String(value)}`,
  }))

  return (
    <div className='flex min-w-0 flex-col gap-2 border-t px-2.5 py-2 sm:flex-row sm:items-center sm:justify-between sm:px-3'>
      <div className='flex items-center gap-3 text-sm'>
        <span className='whitespace-nowrap'>
          <span className='text-muted-foreground'>{t('Total:')}</span>{' '}
          <span className='font-medium tabular-nums'>
            {total.toLocaleString()}
          </span>
        </span>
        <div className='flex items-center gap-1.5'>
          <span className='text-muted-foreground hidden whitespace-nowrap sm:inline'>
            {t('Rows per page')}
          </span>
          <Select
            items={PAGE_SIZE_ITEMS}
            value={String(pageSize)}
            onValueChange={(value) => {
              if (value) onPageSizeChange(Number(value))
            }}
            disabled={disabled}
          >
            <SelectTrigger
              size='sm'
              className='w-[4.5rem] tabular-nums'
              aria-label={t('Rows per page')}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent side='top' alignItemWithTrigger={false}>
              <SelectGroup>
                {PAGE_SIZE_OPTIONS.map((option) => (
                  <SelectItem key={option} value={String(option)}>
                    {option}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className='flex items-center justify-end gap-1'>
        <Button
          type='button'
          variant='outline'
          size='icon-sm'
          disabled={disabled || currentPage <= 1}
          onClick={() => onPageChange(1)}
        >
          <ChevronsLeft />
          <span className='sr-only'>{t('Go to first page')}</span>
        </Button>
        <Button
          type='button'
          variant='outline'
          size='icon-sm'
          disabled={disabled || currentPage <= 1}
          onClick={() => onPageChange(currentPage - 1)}
        >
          <ChevronLeft />
          <span className='sr-only'>{t('Go to previous page')}</span>
        </Button>
        {pageEntries.map((entry) =>
          entry.value === '...' ? (
            <span
              key={entry.key}
              className='text-muted-foreground px-1 text-sm'
            >
              ...
            </span>
          ) : (
            <Button
              key={entry.key}
              type='button'
              variant={entry.value === currentPage ? 'default' : 'outline'}
              size='icon-sm'
              className='tabular-nums'
              disabled={disabled}
              onClick={() => onPageChange(Number(entry.value))}
            >
              {entry.value}
              <span className='sr-only'>
                {t('Go to page {{page}}', { page: entry.value })}
              </span>
            </Button>
          )
        )}
        <Button
          type='button'
          variant='outline'
          size='icon-sm'
          disabled={disabled || currentPage >= totalPages}
          onClick={() => onPageChange(currentPage + 1)}
        >
          <ChevronRight />
          <span className='sr-only'>{t('Go to next page')}</span>
        </Button>
        <Button
          type='button'
          variant='outline'
          size='icon-sm'
          disabled={disabled || currentPage >= totalPages}
          onClick={() => onPageChange(totalPages)}
        >
          <ChevronsRight />
          <span className='sr-only'>{t('Go to last page')}</span>
        </Button>
      </div>
    </div>
  )
}

export function TrustedProxyWarning({ warning }: { warning?: string }) {
  const { t } = useTranslation()
  const warningKey = getTrustedProxyWarningKey(warning)
  if (!warningKey) return null

  return (
    <Alert variant='destructive'>
      <AlertTriangle />
      <AlertTitle>{t('IP audit reliability warning')}</AlertTitle>
      <AlertDescription>{t(warningKey)}</AlertDescription>
    </Alert>
  )
}
