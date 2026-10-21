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
import { ListChecks, RefreshCw, RotateCcw, Search } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DateRangePicker,
  type DateRangeValue,
} from '@/components/date-range-picker'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import dayjs from '@/lib/dayjs'

import {
  listUserRedemptions,
  type UserRedemptionCode,
  type UserRedemptionListParams,
} from './api'
import { AdminBulkSettlementDialog } from './components/admin-bulk-settlement-dialog'
import {
  AdminUserRedemptionGovernanceDialog,
  type AdminUserRedemptionAction,
} from './components/admin-governance-dialog'
import { UserRedemptionDetailDialog } from './components/user-redemption-detail-dialog'
import { UserRedemptionsTable } from './components/user-redemptions-table'

const PAGE_SIZE = 20

type AdminFilters = {
  creatorId: string
  creatorUsername: string
  rewardType: '' | 'balance' | 'subscription'
  status: string
  kind: '' | 'single' | 'random' | 'fixed'
  deleted: 'false' | 'true' | 'all'
  createdFrom: string
  createdTo: string
  expiresFrom: string
  expiresTo: string
}

const EMPTY_FILTERS: AdminFilters = {
  creatorId: '',
  creatorUsername: '',
  rewardType: '',
  status: '',
  kind: '',
  deleted: 'all',
  createdFrom: '',
  createdTo: '',
  expiresFrom: '',
  expiresTo: '',
}

const REDEMPTION_STATUS_OPTIONS = [
  { value: 'active', label: 'Active' },
  { value: 'disabled', label: 'Disabled' },
  { value: 'exhausted', label: 'Exhausted' },
  { value: 'expired_pending', label: 'Pending settlement' },
  { value: 'refund_pending', label: 'Refund pending' },
  { value: 'refunded', label: 'Refunded' },
  { value: 'forfeited', label: 'Forfeited' },
  { value: 'deleted', label: 'Deleted' },
] as const

function dateToUnix(value: string, endOfDay = false): number | undefined {
  if (!value) return undefined
  const date = dayjs(value)
  if (!date.isValid()) return undefined
  return (endOfDay ? date.endOf('day') : date.startOf('day')).unix()
}

function toRangeValue(from: string, to: string): DateRangeValue {
  return {
    from: from ? dayjs(from).toDate() : undefined,
    to: to ? dayjs(to).toDate() : undefined,
  }
}

function toFilterDate(value?: Date): string {
  return value ? dayjs(value).format('YYYY-MM-DD') : ''
}

function buildListParams(
  filters: AdminFilters,
  page: number
): UserRedemptionListParams & { admin: true } {
  const creatorId = Number.parseInt(filters.creatorId, 10)
  return {
    admin: true,
    page,
    page_size: PAGE_SIZE,
    creator_user_id: Number.isFinite(creatorId) ? creatorId : undefined,
    creator_username: filters.creatorUsername.trim() || undefined,
    reward_type: filters.rewardType,
    status: filters.status || undefined,
    kind: filters.kind,
    deleted: filters.deleted,
    created_from: dateToUnix(filters.createdFrom),
    created_to: dateToUnix(filters.createdTo, true),
    expires_from: dateToUnix(filters.expiresFrom),
    expires_to: dateToUnix(filters.expiresTo, true),
  }
}

export default function UserRedemptionsPage() {
  const { t } = useTranslation()
  const [items, setItems] = useState<UserRedemptionCode[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [draftFilters, setDraftFilters] = useState<AdminFilters>(EMPTY_FILTERS)
  const [filters, setFilters] = useState<AdminFilters>(EMPTY_FILTERS)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [governanceTarget, setGovernanceTarget] =
    useState<UserRedemptionCode | null>(null)
  const [governanceAction, setGovernanceAction] =
    useState<AdminUserRedemptionAction | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [bulkSettlementOpen, setBulkSettlementOpen] = useState(false)
  const [reloadVersion, setReloadVersion] = useState(0)

  const reload = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await listUserRedemptions(buildListParams(filters, page))
      setItems(result.items || [])
      setTotal(result.total || 0)
      setSelectedIds((current) =>
        current.filter((id) => result.items.some((item) => item.id === id))
      )
    } catch (caught: unknown) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t('failed to load')
      )
    } finally {
      setLoading(false)
    }
  }, [filters, page, t])

  useEffect(() => {
    void reload()
  }, [reload, reloadVersion])

  function openGovernance(id: number, action: AdminUserRedemptionAction) {
    const target = items.find((item) => item.id === id)
    if (!target) return
    setGovernanceTarget(target)
    setGovernanceAction(action)
  }

  function closeGovernance() {
    setGovernanceTarget(null)
    setGovernanceAction(null)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('User Redemption Codes')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <span className='text-muted-foreground text-sm tabular-nums'>
            {t('Total')}: {total}
          </span>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            disabled={loading}
            aria-label={t('Refresh')}
            title={t('Refresh')}
            onClick={() => {
              setPage(1)
              setReloadVersion((current) => current + 1)
            }}
          >
            <RefreshCw aria-hidden='true' />
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={selectedIds.length === 0}
            onClick={() => setBulkSettlementOpen(true)}
          >
            <ListChecks aria-hidden='true' />
            {t('Batch settlement')} ({selectedIds.length})
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Audit and govern wallet-funded redemption codes created by users.'
              )}
            </p>

            <section className='space-y-3 border-y py-4'>
              <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-6'>
                <Input
                  value={draftFilters.creatorUsername}
                  placeholder={t('Creator username')}
                  aria-label={t('Creator username')}
                  onChange={(event) =>
                    setDraftFilters((current) => ({
                      ...current,
                      creatorUsername: event.target.value,
                    }))
                  }
                />
                <Input
                  value={draftFilters.creatorId}
                  type='number'
                  min={1}
                  placeholder={t('Creator ID')}
                  aria-label={t('Creator ID')}
                  onChange={(event) =>
                    setDraftFilters((current) => ({
                      ...current,
                      creatorId: event.target.value,
                    }))
                  }
                />
                <Select
                  items={[
                    { value: 'all', label: t('All reward types') },
                    { value: 'balance', label: t('Balance') },
                    { value: 'subscription', label: t('Subscription') },
                  ]}
                  value={draftFilters.rewardType || 'all'}
                  onValueChange={(value) =>
                    setDraftFilters((current) => ({
                      ...current,
                      rewardType:
                        value === 'all'
                          ? ''
                          : (value as AdminFilters['rewardType']),
                    }))
                  }
                >
                  <SelectTrigger
                    className='w-full'
                    aria-label={t('Reward type')}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='all'>
                        {t('All reward types')}
                      </SelectItem>
                      <SelectItem value='balance'>{t('Balance')}</SelectItem>
                      <SelectItem value='subscription'>
                        {t('Subscription')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <Select
                  items={[
                    { value: 'all', label: t('All modes') },
                    { value: 'single', label: t('Single') },
                    { value: 'random', label: t('Red packet') },
                    { value: 'fixed', label: t('Fixed units') },
                  ]}
                  value={draftFilters.kind || 'all'}
                  onValueChange={(value) =>
                    setDraftFilters((current) => ({
                      ...current,
                      kind:
                        value === 'all' ? '' : (value as AdminFilters['kind']),
                    }))
                  }
                >
                  <SelectTrigger className='w-full' aria-label={t('Kind')}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='all'>{t('All modes')}</SelectItem>
                      <SelectItem value='single'>{t('Single')}</SelectItem>
                      <SelectItem value='random'>{t('Red packet')}</SelectItem>
                      <SelectItem value='fixed'>{t('Fixed units')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <Select
                  items={[
                    { value: 'all', label: t('All statuses') },
                    ...REDEMPTION_STATUS_OPTIONS.map((option) => ({
                      value: option.value,
                      label: t(option.label),
                    })),
                  ]}
                  value={draftFilters.status || 'all'}
                  onValueChange={(value) =>
                    setDraftFilters((current) => ({
                      ...current,
                      status: value === 'all' ? '' : (value ?? ''),
                    }))
                  }
                >
                  <SelectTrigger className='w-full' aria-label={t('Status')}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='all'>{t('All statuses')}</SelectItem>
                      {REDEMPTION_STATUS_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.label)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <Select
                  items={[
                    { value: 'all', label: t('All records') },
                    { value: 'false', label: t('Not deleted') },
                    { value: 'true', label: t('Deleted') },
                  ]}
                  value={draftFilters.deleted}
                  onValueChange={(value) =>
                    setDraftFilters((current) => ({
                      ...current,
                      deleted: (value ?? 'all') as AdminFilters['deleted'],
                    }))
                  }
                >
                  <SelectTrigger
                    className='w-full'
                    aria-label={t('Deletion status')}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='all'>{t('All records')}</SelectItem>
                      <SelectItem value='false'>{t('Not deleted')}</SelectItem>
                      <SelectItem value='true'>{t('Deleted')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>

              <div className='flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end'>
                <label className='min-w-0 space-y-1 text-xs sm:w-72'>
                  <span className='text-muted-foreground'>
                    {t('Created date range')}
                  </span>
                  <DateRangePicker
                    value={toRangeValue(
                      draftFilters.createdFrom,
                      draftFilters.createdTo
                    )}
                    onChange={(value) =>
                      setDraftFilters((current) => ({
                        ...current,
                        createdFrom: toFilterDate(value.from),
                        createdTo: toFilterDate(value.to),
                      }))
                    }
                  />
                </label>
                <label className='min-w-0 space-y-1 text-xs sm:w-72'>
                  <span className='text-muted-foreground'>
                    {t('Expiry date range')}
                  </span>
                  <DateRangePicker
                    value={toRangeValue(
                      draftFilters.expiresFrom,
                      draftFilters.expiresTo
                    )}
                    onChange={(value) =>
                      setDraftFilters((current) => ({
                        ...current,
                        expiresFrom: toFilterDate(value.from),
                        expiresTo: toFilterDate(value.to),
                      }))
                    }
                  />
                </label>
                <div className='flex justify-end gap-2 sm:ml-auto'>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => {
                      setDraftFilters(EMPTY_FILTERS)
                      setFilters(EMPTY_FILTERS)
                      setPage(1)
                      setReloadVersion((current) => current + 1)
                    }}
                  >
                    <RotateCcw aria-hidden='true' />
                    {t('Reset')}
                  </Button>
                  <Button
                    type='button'
                    onClick={() => {
                      setFilters(draftFilters)
                      setPage(1)
                      setReloadVersion((current) => current + 1)
                    }}
                  >
                    <Search aria-hidden='true' />
                    {t('Search')}
                  </Button>
                </div>
              </div>
            </section>

            {error ? (
              <div className='text-destructive text-sm'>{error}</div>
            ) : null}
            <UserRedemptionsTable
              items={items}
              loading={loading}
              showCreator
              onView={setSelectedId}
              onDisable={(id) => openGovernance(id, 'disable')}
              onEnable={(id) => openGovernance(id, 'enable')}
              onTerminate={(id) => openGovernance(id, 'terminate')}
              onDelete={(id) => openGovernance(id, 'delete')}
              selectedIds={selectedIds}
              onSelectedIdsChange={setSelectedIds}
            />

            {total > PAGE_SIZE ? (
              <div className='flex items-center justify-end gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  disabled={loading || page <= 1}
                  onClick={() => setPage((current) => Math.max(1, current - 1))}
                >
                  {t('Previous')}
                </Button>
                <span className='text-muted-foreground text-sm tabular-nums'>
                  {t('Page')} {page} /{' '}
                  {Math.max(1, Math.ceil(total / PAGE_SIZE))}
                </span>
                <Button
                  type='button'
                  variant='outline'
                  disabled={loading || page >= Math.ceil(total / PAGE_SIZE)}
                  onClick={() => setPage((current) => current + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            ) : null}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <UserRedemptionDetailDialog
        id={selectedId}
        admin
        onOpenChange={(open) => {
          if (!open) setSelectedId(null)
        }}
      />
      <AdminUserRedemptionGovernanceDialog
        target={governanceTarget}
        action={governanceAction}
        onOpenChange={(open) => {
          if (!open) closeGovernance()
        }}
        onCompleted={reload}
      />
      <AdminBulkSettlementDialog
        open={bulkSettlementOpen}
        ids={selectedIds}
        onOpenChange={setBulkSettlementOpen}
        onCompleted={async () => {
          setSelectedIds([])
          await reload()
        }}
      />
    </>
  )
}
