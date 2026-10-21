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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getInvitationRewardSummary,
  submitInvitationRewardPayout,
} from '@/features/invitations/api'
import { getInvitationErrorMessage } from '@/features/invitations/lib/invitation-state'
import { formatQuota } from '@/lib/format'

interface AffiliateRewardsCardProps {
  complianceConfirmed?: boolean
  loading?: boolean
}

export function AffiliateRewardsCard(props: AffiliateRewardsCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const rewardsQuery = useQuery({
    queryKey: ['invitation', 'rewards'],
    queryFn: getInvitationRewardSummary,
  })
  const payoutMutation = useMutation({
    mutationFn: submitInvitationRewardPayout,
    onSuccess: async () => {
      toast.success(t('Payout request submitted'))
      await queryClient.invalidateQueries({
        queryKey: ['invitation', 'rewards'],
      })
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  if (props.loading || rewardsQuery.isLoading) {
    return (
      <Card data-card-hover='false' className='bg-muted/20 py-0'>
        <CardContent className='grid gap-4 p-3 sm:p-4'>
          <div>
            <Skeleton className='h-5 w-32' />
            <Skeleton className='mt-2 h-4 w-48' />
          </div>
          <Skeleton className='h-14 rounded-lg' />
          <Skeleton className='h-10 rounded-lg' />
        </CardContent>
      </Card>
    )
  }

  const rewards = rewardsQuery.data
  return (
    <Card data-card-hover='false' className='bg-muted/20 self-start py-0'>
      <CardContent className='grid gap-4 p-4'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <IconBadge tone='chart-3'>
              <Gift />
            </IconBadge>
            <div className='min-w-0'>
              <h3 className='truncate text-sm font-semibold'>
                {t('Invitation rewards')}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Invitation rewards are submitted together for administrator approval.'
                )}
              </p>
              {rewards?.cash_commission_visible ? (
                <p className='text-primary mt-1 text-xs font-medium'>
                  {t('Top-up cashback {{rate}}%', {
                    rate: formatCommissionRate(rewards.cash_commission_rate),
                  })}
                </p>
              ) : null}
            </div>
          </div>
          <Button
            onClick={() => payoutMutation.mutate()}
            disabled={
              !props.complianceConfirmed ||
              payoutMutation.isPending ||
              (rewards?.available ?? 0) <= 0
            }
            className='h-9'
            size='sm'
          >
            {t('Submit payout review')}
          </Button>
        </div>

        <div className='grid grid-cols-2 gap-3 text-center sm:grid-cols-6'>
          {[
            [
              t('Invited users'),
              (rewards?.invited_count ?? 0).toLocaleString(),
            ],
            [t('Total earned'), formatQuota(rewards?.total_earned ?? 0)],
            [t('Available'), formatQuota(rewards?.available ?? 0)],
            [t('Pending payout'), formatQuota(rewards?.pending_payout ?? 0)],
            [t('Paid'), formatQuota(rewards?.paid ?? 0)],
            [t('Frozen'), formatQuota(rewards?.frozen ?? 0)],
          ].map(([label, value]) => (
            <div key={label} className='min-w-0'>
              <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                {label}
              </div>
              <div className='mt-0.5 truncate text-sm font-semibold tabular-nums'>
                {value}
              </div>
            </div>
          ))}
        </div>
        {rewardsQuery.isError ? (
          <p className='text-muted-foreground text-xs'>
            {t('Unable to load invitation rewards. Please try again later.')}
          </p>
        ) : null}
        {!props.complianceConfirmed ? (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Invitation reward payout requests are disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

function formatCommissionRate(rate: string): string {
  const normalized = rate.trim()
  if (!normalized.includes('.')) return normalized
  return normalized.replace(/0+$/, '').replace(/\.$/, '')
}
