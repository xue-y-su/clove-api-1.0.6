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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import {
  cancelUserRedemption,
  getUserRedemptionListQueryKey,
  listMyUserRedemptions,
  type UserRedemptionCode,
} from '../api'
import { UserRedemptionDetailDialog } from './user-redemption-detail-dialog'
import { UserRedemptionsTable } from './user-redemptions-table'

interface MyUserRedemptionsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onBalanceChange?: () => void | Promise<void>
}

const PAGE_SIZE = 20

export function MyUserRedemptionsDialog(props: MyUserRedemptionsDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [cancelTarget, setCancelTarget] = useState<UserRedemptionCode | null>(
    null
  )
  const listQuery = useQuery({
    queryKey: getUserRedemptionListQueryKey(false, page),
    queryFn: () =>
      listMyUserRedemptions({ page, page_size: PAGE_SIZE, deleted: 'all' }),
    enabled: props.open,
  })
  const cancelMutation = useMutation({
    mutationFn: (id: number) => cancelUserRedemption(id),
    onSuccess: async () => {
      const wasSubscription = cancelTarget?.reward_type === 'subscription'
      setCancelTarget(null)
      toast.success(
        t(
          wasSubscription
            ? 'Unused subscription units were refunded automatically'
            : 'Redemption code submitted for administrator settlement'
        )
      )
      await queryClient.invalidateQueries({ queryKey: ['user-redemptions'] })
      await props.onBalanceChange?.()
    },
    onError: () => toast.error(t('cancel failed')),
  })
  const totalPages = Math.max(
    1,
    Math.ceil((listQuery.data?.total ?? 0) / PAGE_SIZE)
  )

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={(open) => {
          props.onOpenChange(open)
          if (!open) {
            setPage(1)
            setSelectedId(null)
            setCancelTarget(null)
          }
        }}
        title={t('My Redemption Codes')}
        description={t('Review codes you created and their claim history.')}
        contentClassName='sm:max-w-6xl'
        contentHeight='min(68vh, 720px)'
      >
        <div className='space-y-4'>
          {listQuery.isError ? (
            <div className='text-destructive text-sm'>
              {t('failed to load')}
            </div>
          ) : null}
          <UserRedemptionsTable
            items={listQuery.data?.items ?? []}
            loading={listQuery.isLoading}
            busy={cancelMutation.isPending}
            onView={setSelectedId}
            onCancel={(id) => {
              const item = listQuery.data?.items.find(
                (candidate) => candidate.id === id
              )
              setCancelTarget(item ?? null)
            }}
          />
          {totalPages > 1 ? (
            <div className='flex items-center justify-end gap-2'>
              <Button
                type='button'
                variant='outline'
                disabled={listQuery.isFetching || page <= 1}
                onClick={() => setPage((current) => Math.max(1, current - 1))}
              >
                {t('Previous')}
              </Button>
              <span className='text-muted-foreground text-sm tabular-nums'>
                {page} / {totalPages}
              </span>
              <Button
                type='button'
                variant='outline'
                disabled={listQuery.isFetching || page >= totalPages}
                onClick={() => setPage((current) => current + 1)}
              >
                {t('Next')}
              </Button>
            </div>
          ) : null}
        </div>
      </Dialog>

      <ConfirmDialog
        open={cancelTarget !== null}
        onOpenChange={(open) => {
          if (!open) setCancelTarget(null)
        }}
        title={t('Cancel redemption code?')}
        desc={t(
          cancelTarget?.reward_type === 'subscription'
            ? 'Unused subscription units will be refunded to your wallet automatically.'
            : 'The unclaimed balance will be submitted for administrator settlement.'
        )}
        confirmText={t('Cancel redemption code')}
        destructive
        isLoading={cancelMutation.isPending}
        handleConfirm={() => {
          if (cancelTarget) cancelMutation.mutate(cancelTarget.id)
        }}
      />

      <UserRedemptionDetailDialog
        id={selectedId}
        onOpenChange={(open) => {
          if (!open) setSelectedId(null)
        }}
      />
    </>
  )
}
