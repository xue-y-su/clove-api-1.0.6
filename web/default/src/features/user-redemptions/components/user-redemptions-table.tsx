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
import { Ban, CircleCheck, Copy, Eye, OctagonX, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { formatQuota } from '@/lib/format'

import type { UserRedemptionCode } from '../api'
import {
  canDeleteUserRedemption,
  canSettleUserRedemption,
  USER_REDEMPTION_KIND_LABELS,
  USER_REDEMPTION_REWARD_LABELS,
  USER_REDEMPTION_STATUS_LABELS,
} from '../types'

type UserRedemptionsTableProps = {
  items: UserRedemptionCode[]
  loading?: boolean
  busy?: boolean
  showCreator?: boolean
  onView?: (id: number) => void
  onCancel?: (id: number) => void
  onDisable?: (id: number) => void
  onEnable?: (id: number) => void
  onTerminate?: (id: number) => void
  onDelete?: (id: number) => void
  selectedIds?: number[]
  onSelectedIdsChange?: (ids: number[]) => void
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function RedemptionQuotaValue({ item }: { item: UserRedemptionCode }) {
  const { t } = useTranslation()
  if (item.reward_type === 'subscription') {
    return (
      <div className='whitespace-nowrap tabular-nums'>
        {item.claim_count}/{item.max_claims} {t('units')}
        {item.unit_paid_quota !== undefined ? (
          <div className='text-muted-foreground text-xs'>
            {t('Per unit')} {formatQuota(item.unit_paid_quota)}
          </div>
        ) : null}
      </div>
    )
  }

  return (
    <div className='whitespace-nowrap tabular-nums'>
      {formatQuota(item.claimed_quota)}/{formatQuota(item.total_quota)}
      {item.refunded_quota > 0 ? (
        <div className='text-muted-foreground text-xs'>
          {t('Refund')} {formatQuota(item.refunded_quota)}
        </div>
      ) : null}
      {item.forfeited_quota > 0 ? (
        <div className='text-muted-foreground text-xs'>
          {t('Forfeited')} {formatQuota(item.forfeited_quota)}
        </div>
      ) : null}
    </div>
  )
}

function RedemptionActionButtons({
  item,
  tableProps,
  hasGovernanceActions,
  onCopy,
}: {
  item: UserRedemptionCode
  tableProps: UserRedemptionsTableProps
  hasGovernanceActions: boolean
  onCopy: () => void
}) {
  const { t } = useTranslation()
  const isLive =
    item.status === 'active' ||
    item.status === 'disabled' ||
    item.status === 'expired_pending'

  return (
    <div className='flex flex-wrap gap-1'>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        title={t('Copy redemption code')}
        aria-label={t('Copy redemption code')}
        onClick={onCopy}
      >
        <Copy aria-hidden='true' />
      </Button>
      {tableProps.onView ? (
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          title={t('View details')}
          aria-label={t('View details')}
          onClick={() => tableProps.onView?.(item.id)}
        >
          <Eye aria-hidden='true' />
        </Button>
      ) : null}
      {tableProps.onCancel &&
      item.status === 'active' &&
      !hasGovernanceActions ? (
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={tableProps.busy}
          title={t('Cancel')}
          aria-label={t('Cancel')}
          onClick={() => tableProps.onCancel?.(item.id)}
        >
          <Ban aria-hidden='true' />
        </Button>
      ) : null}
      {tableProps.onDisable && item.status === 'active' ? (
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={tableProps.busy}
          title={t('Disable')}
          aria-label={t('Disable')}
          onClick={() => tableProps.onDisable?.(item.id)}
        >
          <Ban aria-hidden='true' />
        </Button>
      ) : null}
      {tableProps.onEnable && item.status === 'disabled' ? (
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={tableProps.busy}
          title={t('Enable')}
          aria-label={t('Enable')}
          onClick={() => tableProps.onEnable?.(item.id)}
        >
          <CircleCheck aria-hidden='true' />
        </Button>
      ) : null}
      {tableProps.onTerminate && isLive ? (
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={tableProps.busy}
          title={t('Terminate')}
          aria-label={t('Terminate')}
          onClick={() => tableProps.onTerminate?.(item.id)}
        >
          <OctagonX aria-hidden='true' />
        </Button>
      ) : null}
      {tableProps.onDelete &&
      hasGovernanceActions &&
      canDeleteUserRedemption(item) ? (
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={tableProps.busy}
          title={t('Delete')}
          aria-label={t('Delete')}
          onClick={() => tableProps.onDelete?.(item.id)}
        >
          <Trash2 aria-hidden='true' />
        </Button>
      ) : null}
    </div>
  )
}

export function UserRedemptionsTable(props: UserRedemptionsTableProps) {
  const { t } = useTranslation()
  const clipboard = useCopyToClipboard()
  const hasGovernanceActions = Boolean(
    props.onDisable || props.onEnable || props.onTerminate || props.onDelete
  )
  const selectedIds = props.selectedIds ?? []
  const selectionEnabled = Boolean(props.onSelectedIdsChange)
  const selectableItems = props.items.filter(canSettleUserRedemption)
  const allSelectableItemsSelected =
    selectableItems.length > 0 &&
    selectableItems.every((item) => selectedIds.includes(item.id))

  function toggleItemSelection(item: UserRedemptionCode, checked: boolean) {
    if (!props.onSelectedIdsChange || !canSettleUserRedemption(item)) return
    const next = checked
      ? [...new Set([...selectedIds, item.id])]
      : selectedIds.filter((id) => id !== item.id)
    props.onSelectedIdsChange(next)
  }

  function toggleAllSelection(checked: boolean) {
    if (!props.onSelectedIdsChange) return
    if (checked) {
      props.onSelectedIdsChange([
        ...new Set([...selectedIds, ...selectableItems.map((item) => item.id)]),
      ])
      return
    }
    const selectableIds = new Set(selectableItems.map((item) => item.id))
    props.onSelectedIdsChange(
      selectedIds.filter((id) => !selectableIds.has(id))
    )
  }

  if (props.loading) {
    return (
      <div className='text-muted-foreground text-sm'>{t('Loading...')}</div>
    )
  }

  if (!props.items.length) {
    return (
      <div className='text-muted-foreground text-sm'>{t('No codes yet.')}</div>
    )
  }

  return (
    <>
      <div className='divide-y rounded-md border md:hidden'>
        {props.items.map((item) => (
          <article key={item.id} className='space-y-3 p-3'>
            <div className='flex min-w-0 items-start gap-2'>
              {selectionEnabled ? (
                <Checkbox
                  className='mt-1 rounded-full'
                  checked={selectedIds.includes(item.id)}
                  disabled={!canSettleUserRedemption(item)}
                  aria-label={t('Select redemption code {{code}}', {
                    code: item.code,
                  })}
                  onCheckedChange={(checked) =>
                    toggleItemSelection(item, Boolean(checked))
                  }
                />
              ) : null}
              <div className='min-w-0'>
                <div className='font-mono text-xs leading-5 [overflow-wrap:anywhere] break-all'>
                  {item.code}
                </div>
                <div className='text-muted-foreground mt-1 flex flex-wrap gap-x-2 gap-y-1 text-xs'>
                  <span>
                    {t(USER_REDEMPTION_REWARD_LABELS[item.reward_type])}
                  </span>
                  <span>{t(USER_REDEMPTION_KIND_LABELS[item.kind])}</span>
                  <span
                    className={
                      item.status === 'expired_pending'
                        ? 'font-medium text-amber-700 dark:text-amber-300'
                        : undefined
                    }
                  >
                    {t(USER_REDEMPTION_STATUS_LABELS[item.status])}
                  </span>
                </div>
              </div>
            </div>
            {item.reward_type === 'subscription' && item.plan_title ? (
              <div className='text-xs'>
                <div className='text-muted-foreground'>{t('Plan')}</div>
                <div className='font-medium'>{item.plan_title}</div>
              </div>
            ) : null}

            {props.showCreator ? (
              <div className='text-xs'>
                <div className='text-muted-foreground'>{t('Creator ID')}</div>
                <div className='font-medium tabular-nums'>
                  #{item.creator_user_id}
                  {item.creator_username ? ` · ${item.creator_username}` : ''}
                </div>
              </div>
            ) : null}

            <dl className='grid grid-cols-2 gap-x-4 gap-y-2 text-xs'>
              <div className='min-w-0'>
                <dt className='text-muted-foreground'>{t('Quota')}</dt>
                <dd className='mt-0.5'>
                  <RedemptionQuotaValue item={item} />
                </dd>
              </div>
              <div>
                <dt className='text-muted-foreground'>{t('Claims')}</dt>
                <dd className='mt-0.5 tabular-nums'>
                  {item.claim_count}/{item.max_claims}
                </dd>
              </div>
              <div className='col-span-2'>
                <dt className='text-muted-foreground'>{t('Created')}</dt>
                <dd className='mt-0.5 tabular-nums'>
                  {formatTimestamp(item.created_at)}
                </dd>
              </div>
              <div className='col-span-2'>
                <dt className='text-muted-foreground'>{t('Expires')}</dt>
                <dd className='mt-0.5 tabular-nums'>
                  {formatTimestamp(item.expires_at)}
                </dd>
              </div>
            </dl>

            <div className='flex justify-end border-t pt-2'>
              <RedemptionActionButtons
                item={item}
                tableProps={props}
                hasGovernanceActions={hasGovernanceActions}
                onCopy={() => void clipboard.copyToClipboard(item.code)}
              />
            </div>
          </article>
        ))}
      </div>

      <div className='hidden max-w-full overflow-x-auto rounded-md border md:block'>
        <table className='w-full min-w-[1320px] text-sm'>
          <thead className='bg-muted/40 text-left'>
            <tr>
              {selectionEnabled ? (
                <th className='w-10 p-2 whitespace-nowrap'>
                  <Checkbox
                    className='rounded-full'
                    checked={allSelectableItemsSelected}
                    disabled={selectableItems.length === 0}
                    aria-label={t('Select all eligible redemption codes')}
                    onCheckedChange={(checked) =>
                      toggleAllSelection(Boolean(checked))
                    }
                  />
                </th>
              ) : null}
              <th className='w-56 p-2 whitespace-nowrap'>{t('Code')}</th>
              {props.showCreator ? (
                <th className='w-32 p-2 whitespace-nowrap'>
                  {t('Creator ID')}
                </th>
              ) : null}
              <th className='w-24 p-2 whitespace-nowrap'>{t('Kind')}</th>
              <th className='w-40 p-2 whitespace-nowrap'>
                {t('Reward type')}
              </th>
              <th className='w-28 p-2 whitespace-nowrap'>{t('Status')}</th>
              <th className='w-44 p-2 whitespace-nowrap'>{t('Quota')}</th>
              <th className='w-24 p-2 whitespace-nowrap'>{t('Claims')}</th>
              <th className='w-40 p-2 whitespace-nowrap'>{t('Created')}</th>
              <th className='w-40 p-2 whitespace-nowrap'>{t('Expires')}</th>
              <th className='w-36 p-2 whitespace-nowrap'>{t('Actions')}</th>
            </tr>
          </thead>
          <tbody>
            {props.items.map((item) => (
              <tr key={item.id} className='border-t align-top'>
                {selectionEnabled ? (
                  <td className='p-2'>
                    <Checkbox
                      className='rounded-full'
                      checked={selectedIds.includes(item.id)}
                      disabled={!canSettleUserRedemption(item)}
                      aria-label={t('Select redemption code {{code}}', {
                        code: item.code,
                      })}
                      onCheckedChange={(checked) =>
                        toggleItemSelection(item, Boolean(checked))
                      }
                    />
                  </td>
                ) : null}
                <td className='p-2 font-mono text-xs whitespace-nowrap'>
                  {item.code}
                </td>
                {props.showCreator ? (
                  <td className='p-2'>
                    <div className='font-medium tabular-nums'>
                      #{item.creator_user_id}
                    </div>
                    {item.creator_username ? (
                      <div className='text-muted-foreground max-w-40 truncate text-xs'>
                        {item.creator_username}
                      </div>
                    ) : null}
                  </td>
                ) : null}
                <td className='p-2 whitespace-nowrap'>
                  {t(USER_REDEMPTION_KIND_LABELS[item.kind])}
                </td>
                <td className='p-2 whitespace-nowrap'>
                  <div>
                    {t(USER_REDEMPTION_REWARD_LABELS[item.reward_type])}
                  </div>
                  {item.plan_title ? (
                    <div className='text-muted-foreground max-w-40 truncate text-xs'>
                      {item.plan_title}
                    </div>
                  ) : null}
                </td>
                <td className='p-2 whitespace-nowrap'>
                  <span
                    className={
                      item.status === 'expired_pending'
                        ? 'font-medium text-amber-700 dark:text-amber-300'
                        : undefined
                    }
                  >
                    {t(USER_REDEMPTION_STATUS_LABELS[item.status])}
                  </span>
                </td>
                <td className='p-2 whitespace-nowrap'>
                  <RedemptionQuotaValue item={item} />
                </td>
                <td className='p-2 whitespace-nowrap tabular-nums'>
                  {item.claim_count}/{item.max_claims}
                </td>
                <td className='p-2 text-xs whitespace-nowrap'>
                  {formatTimestamp(item.created_at)}
                </td>
                <td className='p-2 text-xs whitespace-nowrap'>
                  {formatTimestamp(item.expires_at)}
                </td>
                <td className='p-2'>
                  <div className='min-w-32'>
                    <RedemptionActionButtons
                      item={item}
                      tableProps={props}
                      hasGovernanceActions={hasGovernanceActions}
                      onCopy={() => void clipboard.copyToClipboard(item.code)}
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}
