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
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { formatQuota } from '@/lib/format'

import { getUserRedemption, getUserRedemptionDetailQueryKey } from '../api'
import {
  USER_REDEMPTION_REWARD_LABELS,
  USER_REDEMPTION_STATUS_LABELS,
} from '../types'

interface UserRedemptionDetailDialogProps {
  id: number | null
  admin?: boolean
  onOpenChange: (open: boolean) => void
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

export function UserRedemptionDetailDialog(
  props: UserRedemptionDetailDialogProps
) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const detailQuery = useQuery({
    queryKey: getUserRedemptionDetailQueryKey(Boolean(props.admin), props.id),
    queryFn: () => getUserRedemption(props.id as number, props.admin),
    enabled: props.id !== null,
  })
  const detail = detailQuery.data

  return (
    <Dialog
      open={props.id !== null}
      onOpenChange={props.onOpenChange}
      title={t('Redemption Code Details')}
      description={t('Review settlement, expiry, and claim history.')}
      contentClassName='sm:max-w-3xl'
    >
      {detailQuery.isLoading ? (
        <div className='space-y-3'>
          <Skeleton className='h-9 w-full' />
          <Skeleton className='h-24 w-full' />
        </div>
      ) : null}
      {detailQuery.isError ? (
        <Alert variant='destructive'>
          <AlertTitle>{t('Unable to load redemption code')}</AlertTitle>
          <AlertDescription>{t('Request failed')}</AlertDescription>
        </Alert>
      ) : null}
      {detail ? (
        <div className='space-y-5'>
          <div className='flex min-w-0 gap-2'>
            <Input
              value={detail.code.code}
              readOnly
              aria-label={t('Redemption code')}
              className='min-w-0 flex-1 font-mono'
            />
            <Button
              type='button'
              variant='outline'
              size='icon'
              aria-label={t('Copy redemption code')}
              onClick={() => void copyToClipboard(detail.code.code)}
            >
              <Copy aria-hidden='true' />
            </Button>
          </div>

          <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
            <Metric
              label={t('Reward type')}
              value={t(USER_REDEMPTION_REWARD_LABELS[detail.code.reward_type])}
            />
            <Metric
              label={t('Status')}
              value={t(USER_REDEMPTION_STATUS_LABELS[detail.code.status])}
            />
            <Metric
              label={t('Total')}
              value={
                detail.code.reward_type === 'subscription'
                  ? `${detail.code.max_claims} ${t('units')}`
                  : formatQuota(detail.code.total_quota)
              }
            />
            <Metric
              label={t('Claimed')}
              value={
                detail.code.reward_type === 'subscription'
                  ? `${detail.code.claim_count} ${t('units')}`
                  : formatQuota(detail.code.claimed_quota)
              }
            />
            <Metric
              label={t('Remaining')}
              value={
                detail.code.reward_type === 'subscription'
                  ? `${Math.max(0, detail.code.max_claims - detail.code.claim_count)} ${t('units')}`
                  : formatQuota(detail.code.remaining_quota)
              }
            />
            <Metric
              label={t('Refunded')}
              value={formatQuota(detail.code.refunded_quota)}
            />
            <Metric
              label={t('Forfeited')}
              value={formatQuota(detail.code.forfeited_quota)}
            />
            <Metric
              label={t('Claims')}
              value={`${detail.code.claim_count}/${detail.code.max_claims}`}
            />
            <Metric
              label={t('Expires')}
              value={formatTimestamp(detail.code.expires_at)}
            />
          </div>

          {detail.code.reward_type === 'subscription' ? (
            <div className='grid gap-2 text-sm sm:grid-cols-2'>
              <AuditLine
                label={t('Plan')}
                value={detail.code.plan_title || '-'}
              />
              <AuditLine
                label={t('Per unit')}
                value={formatQuota(detail.code.unit_paid_quota ?? 0)}
              />
            </div>
          ) : null}

          {props.admin ? (
            <div className='grid gap-2 text-sm sm:grid-cols-2'>
              <AuditLine
                label={t('Creator')}
                value={`${detail.code.creator_username || '-'} (#${detail.code.creator_user_id})`}
              />
              <AuditLine
                label={t('Created')}
                value={formatTimestamp(detail.code.created_at)}
              />
              <AuditLine
                label={t('Disabled reason')}
                value={detail.code.disabled_reason || '-'}
              />
              <AuditLine
                label={t('Terminal reason')}
                value={detail.code.terminal_reason || '-'}
              />
              <AuditLine
                label={t('Delete reason')}
                value={detail.code.delete_reason || '-'}
              />
              <AuditLine
                label={t('Deleted')}
                value={
                  detail.code.deleted
                    ? formatTimestamp(detail.code.deleted_at)
                    : '-'
                }
              />
            </div>
          ) : null}

          <section className='space-y-2'>
            <h3 className='text-sm font-semibold'>{t('Claim history')}</h3>
            {detail.claims.length ? (
              <div className='divide-y rounded-md border'>
                {detail.claims.map((claim) => (
                  <div
                    key={claim.id}
                    className='flex items-center justify-between gap-3 px-3 py-2 text-sm'
                  >
                    <div className='min-w-0'>
                      <div className='truncate font-medium'>
                        {claim.claimant_username ||
                          claim.claimant_display ||
                          t('Unknown user')}
                      </div>
                      {claim.claimant_user_id ? (
                        <div className='text-muted-foreground text-xs'>
                          #{claim.claimant_user_id}
                        </div>
                      ) : null}
                    </div>
                    <div className='shrink-0 text-right'>
                      <div className='font-medium tabular-nums'>
                        {claim.reward_type === 'subscription'
                          ? t('1 unit')
                          : formatQuota(claim.quota)}
                      </div>
                      {claim.user_subscription_id ? (
                        <div className='text-muted-foreground text-xs'>
                          {t('Subscription')} #{claim.user_subscription_id}
                        </div>
                      ) : null}
                      <div className='text-muted-foreground text-xs'>
                        {formatTimestamp(claim.claimed_at)}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t('No claims yet.')}
              </p>
            )}
          </section>
        </div>
      ) : null}
    </Dialog>
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/35 min-w-0 rounded-md border px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 truncate text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}

function AuditLine(props: { label: string; value: string }) {
  return (
    <div className='flex min-w-0 justify-between gap-3 border-b py-2'>
      <span className='text-muted-foreground shrink-0'>{props.label}</span>
      <span className='min-w-0 text-right break-words'>{props.value}</span>
    </div>
  )
}
