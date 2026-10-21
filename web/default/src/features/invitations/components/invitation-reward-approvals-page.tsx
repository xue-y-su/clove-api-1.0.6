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
import { Check, Eye, Search, Snowflake, Sun, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { formatQuota } from '@/lib/format'

import {
  freezeInvitationRewardPayoutRequest,
  getInvitationRewardPayoutRequestDetail,
  listInvitationRewardPayoutRequests,
  reviewInvitationRewardPayoutRequest,
  unfreezeInvitationRewardPayoutRequest,
} from '../api'
import { getInvitationErrorMessage } from '../lib/invitation-state'
import type {
  InvitationListQuery,
  InvitationRewardPayoutRequest,
  InvitationRewardPayoutRequestDetail,
  InvitationRewardPayoutRequestStatus,
} from '../types'

const PAGE_SIZE = 20

type ViewMode = 'pending' | 'history'
type ReviewMode = 'approve' | 'reject'
type MutationMode = 'freeze' | 'unfreeze'

export function InvitationRewardApprovalsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [view, setView] = useState<ViewMode>('pending')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [reviewTarget, setReviewTarget] =
    useState<InvitationRewardPayoutRequest | null>(null)
  const [reviewMode, setReviewMode] = useState<ReviewMode>('approve')
  const [reviewReason, setReviewReason] = useState('')
  const [detailTarget, setDetailTarget] =
    useState<InvitationRewardPayoutRequest | null>(null)
  const [mutationTarget, setMutationTarget] = useState<{
    item: InvitationRewardPayoutRequest
    mode: MutationMode
  } | null>(null)
  const [mutationReason, setMutationReason] = useState('')

  const query: InvitationListQuery = {
    page,
    pageSize: PAGE_SIZE,
    status: view,
    keyword,
  }
  const requestsQuery = useQuery({
    queryKey: ['invitation', 'admin', 'reward-payout-requests', query],
    queryFn: () => listInvitationRewardPayoutRequests(query),
  })
  const detailQuery = useQuery({
    queryKey: [
      'invitation',
      'admin',
      'reward-payout-request-detail',
      detailTarget?.id,
    ],
    queryFn: () => {
      if (!detailTarget) {
        throw new Error('Invitation reward payout request target is missing')
      }
      return getInvitationRewardPayoutRequestDetail(detailTarget.id)
    },
    enabled: detailTarget !== null,
  })
  const reviewMutation = useMutation({
    mutationFn: (value: {
      requestId: number
      approve: boolean
      reviewReason: string
    }) =>
      reviewInvitationRewardPayoutRequest(value.requestId, {
        approve: value.approve,
        review_reason: value.reviewReason,
      }),
    onSuccess: async () => {
      setReviewTarget(null)
      setReviewReason('')
      toast.success(t('Payout request reviewed'))
      await invalidateRewardRequests(queryClient)
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  const stateMutation = useMutation({
    mutationFn: async (value: {
      requestId: number
      mode: MutationMode
      reason: string
    }) => {
      if (value.mode === 'freeze') {
        return freezeInvitationRewardPayoutRequest(
          value.requestId,
          value.reason
        )
      }
      return unfreezeInvitationRewardPayoutRequest(
        value.requestId,
        value.reason
      )
    },
    onSuccess: async (_, value) => {
      setMutationTarget(null)
      setMutationReason('')
      toast.success(
        value.mode === 'freeze'
          ? t('Payout request frozen')
          : t('Payout request unfrozen')
      )
      await invalidateRewardRequests(queryClient)
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })

  function search() {
    setPage(1)
    setKeyword(keywordInput.trim())
  }

  function openReview(item: InvitationRewardPayoutRequest, mode: ReviewMode) {
    setReviewTarget(item)
    setReviewMode(mode)
    setReviewReason('')
  }

  function openMutation(
    item: InvitationRewardPayoutRequest,
    mode: MutationMode
  ) {
    setMutationTarget({ item, mode })
    setMutationReason('')
  }

  function submitReview() {
    if (!reviewTarget) return
    const reason = reviewReason.trim()
    if (reviewMode === 'reject' && !reason) {
      toast.error(t('A rejection reason is required'))
      return
    }
    reviewMutation.mutate({
      requestId: reviewTarget.id,
      approve: reviewMode === 'approve',
      reviewReason: reason,
    })
  }

  function submitMutation() {
    const reason = mutationReason.trim()
    if (!mutationTarget || !reason) {
      toast.error(t('A reason is required'))
      return
    }
    stateMutation.mutate({
      requestId: mutationTarget.item.id,
      mode: mutationTarget.mode,
      reason,
    })
  }

  const items = requestsQuery.data?.items ?? []
  const total = requestsQuery.data?.total ?? 0
  const actions: RewardRequestActions = {
    detail: setDetailTarget,
    review: openReview,
    mutate: openMutation,
  }
  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Invitation reward approvals')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3'>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between'>
              <Tabs
                value={view}
                onValueChange={(value) => {
                  setView(value as ViewMode)
                  setPage(1)
                }}
              >
                <TabsList className='grid w-full grid-cols-2 sm:w-auto'>
                  <TabsTrigger value='pending'>
                    {t('Pending review')}
                  </TabsTrigger>
                  <TabsTrigger value='history'>
                    {t('Approval history')}
                  </TabsTrigger>
                </TabsList>
              </Tabs>
              <div className='flex min-w-0 gap-2 sm:w-80'>
                <Input
                  className='min-w-0'
                  value={keywordInput}
                  onChange={(event) => setKeywordInput(event.target.value)}
                  onKeyDown={(event) => event.key === 'Enter' && search()}
                  placeholder={t('Search users or codes')}
                />
                <Button
                  variant='outline'
                  size='icon'
                  aria-label={t('Search')}
                  onClick={search}
                >
                  <Search aria-hidden='true' />
                </Button>
              </div>
            </div>

            <div className='space-y-3 lg:hidden'>
              {items.map((item) => (
                <RewardRequestCard
                  key={item.id}
                  item={item}
                  actions={actions}
                />
              ))}
              <RewardRequestEmptyState
                loading={requestsQuery.isLoading}
                error={requestsQuery.isError}
                empty={items.length === 0}
              />
            </div>

            <div className='hidden min-h-0 flex-1 overflow-auto rounded-md border lg:block'>
              <RewardRequestTable
                items={items}
                loading={requestsQuery.isLoading}
                error={requestsQuery.isError}
                actions={actions}
              />
            </div>

            <div className='flex flex-wrap items-center justify-end gap-2 sm:justify-between'>
              <span className='text-muted-foreground hidden text-sm sm:inline'>
                {t('Total {{count}}', { count: total })}
              </span>
              <div className='flex max-w-full gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page <= 1}
                  onClick={() => setPage((value) => value - 1)}
                >
                  {t('Previous')}
                </Button>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={page * PAGE_SIZE >= total}
                  onClick={() => setPage((value) => value + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ReviewRewardDialog
        open={reviewTarget !== null}
        mode={reviewMode}
        reason={reviewReason}
        pending={reviewMutation.isPending}
        onOpenChange={(open) => !open && setReviewTarget(null)}
        onReasonChange={setReviewReason}
        onSubmit={submitReview}
      />
      <MutationRewardDialog
        open={mutationTarget !== null}
        mode={mutationTarget?.mode ?? 'freeze'}
        reason={mutationReason}
        pending={stateMutation.isPending}
        onOpenChange={(open) => !open && setMutationTarget(null)}
        onReasonChange={setMutationReason}
        onSubmit={submitMutation}
      />
      <RewardDetailDialog
        open={detailTarget !== null}
        detail={detailQuery.data}
        loading={detailQuery.isLoading}
        error={detailQuery.isError}
        onOpenChange={(open) => !open && setDetailTarget(null)}
      />
    </>
  )
}

type RewardRequestActions = {
  detail: (item: InvitationRewardPayoutRequest) => void
  review: (item: InvitationRewardPayoutRequest, mode: ReviewMode) => void
  mutate: (item: InvitationRewardPayoutRequest, mode: MutationMode) => void
}

function RewardRequestTable(props: {
  items: InvitationRewardPayoutRequest[]
  loading: boolean
  error: boolean
  actions: RewardRequestActions
}) {
  const { t } = useTranslation()
  return (
    <Table className='min-w-[1168px]'>
      <TableHeader>
        <TableRow>
          <TableHead>{t('User')}</TableHead>
          <TableHead>{t('Amount')}</TableHead>
          <TableHead>{t('Reward count')}</TableHead>
          <TableHead>{t('Operation ID')}</TableHead>
          <TableHead>{t('Status')}</TableHead>
          <TableHead>{t('Submitted')}</TableHead>
          <TableHead>{t('Reviewed')}</TableHead>
          <TableHead>{t('Reason')}</TableHead>
          <TableHead className='text-right'>{t('Actions')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.id}>
            <TableCell>
              <div className='min-w-28'>
                <div className='font-medium'>
                  {item.display_name || item.username}
                </div>
                <div className='text-muted-foreground text-xs'>
                  @{item.username} · #{item.user_id}
                </div>
              </div>
            </TableCell>
            <TableCell className='font-mono tabular-nums'>
              {formatQuota(item.amount)}
            </TableCell>
            <TableCell className='tabular-nums'>{item.item_count}</TableCell>
            <TableCell>
              <code
                className='block max-w-52 text-xs break-all whitespace-normal'
                title={item.operation_id}
              >
                {item.operation_id}
              </code>
            </TableCell>
            <TableCell>
              <RewardStatusBadge status={item.status} />
            </TableCell>
            <TableCell>{formatTimestamp(item.created_at)}</TableCell>
            <TableCell>{formatTimestamp(item.reviewed_at)}</TableCell>
            <TableCell className='max-w-52 break-words whitespace-normal'>
              {item.reason || '-'}
              {item.reviewer_username ? (
                <div className='text-muted-foreground mt-1 text-xs'>
                  {t('Reviewer')}: {item.reviewer_username}
                </div>
              ) : null}
            </TableCell>
            <TableCell className='text-right'>
              <RewardActions item={item} actions={props.actions} compact />
            </TableCell>
          </TableRow>
        ))}
        {!props.loading && props.items.length === 0 ? (
          <TableRow>
            <TableCell
              colSpan={9}
              className='text-muted-foreground h-36 text-center text-sm'
            >
              {props.error
                ? t('Unable to load payout requests')
                : t('No payout requests found')}
            </TableCell>
          </TableRow>
        ) : null}
      </TableBody>
    </Table>
  )
}

function RewardRequestCard(props: {
  item: InvitationRewardPayoutRequest
  actions: RewardRequestActions
}) {
  const { t } = useTranslation()
  const item = props.item
  return (
    <article className='space-y-3 rounded-md border p-3'>
      <div className='flex min-w-0 items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='font-medium break-words'>
            {item.display_name || item.username}
          </div>
          <div className='text-muted-foreground text-xs break-all'>
            @{item.username} · #{item.user_id}
          </div>
        </div>
        <RewardStatusBadge status={item.status} />
      </div>
      <dl className='grid grid-cols-2 gap-x-3 gap-y-2 text-sm'>
        <DetailTerm label={t('Amount')} value={formatQuota(item.amount)} />
        <DetailTerm label={t('Reward count')} value={String(item.item_count)} />
        <DetailTerm
          label={t('Submitted')}
          value={formatTimestamp(item.created_at)}
        />
        <DetailTerm
          label={t('Reviewed')}
          value={formatTimestamp(item.reviewed_at)}
        />
      </dl>
      <div className='text-muted-foreground text-xs break-all'>
        {item.operation_id}
      </div>
      {item.reason ? (
        <p className='text-sm break-words'>{item.reason}</p>
      ) : null}
      <RewardActions item={item} actions={props.actions} />
    </article>
  )
}

function RewardRequestEmptyState(props: {
  loading: boolean
  error: boolean
  empty: boolean
}) {
  const { t } = useTranslation()
  if (props.loading || !props.empty) return null
  return (
    <div className='text-muted-foreground py-12 text-center text-sm'>
      {props.error
        ? t('Unable to load payout requests')
        : t('No payout requests found')}
    </div>
  )
}

function RewardActions(props: {
  item: InvitationRewardPayoutRequest
  actions: RewardRequestActions
  compact?: boolean
}) {
  const { t } = useTranslation()
  const common = props.compact
    ? { size: 'icon' as const }
    : { size: 'sm' as const }
  return (
    <div className='flex flex-wrap justify-end gap-1'>
      <Button
        variant='ghost'
        {...common}
        aria-label={t('View payout details')}
        title={t('View payout details')}
        onClick={() => props.actions.detail(props.item)}
      >
        <Eye aria-hidden='true' />
        {props.compact ? null : t('Details')}
      </Button>
      {props.item.status === 'pending' ? (
        <>
          <Button
            variant='ghost'
            {...common}
            aria-label={t('Approve payout request')}
            title={t('Approve payout request')}
            onClick={() => props.actions.review(props.item, 'approve')}
          >
            <Check aria-hidden='true' />
            {props.compact ? null : t('Approve')}
          </Button>
          <Button
            variant='ghost'
            {...common}
            aria-label={t('Reject payout request')}
            title={t('Reject payout request')}
            onClick={() => props.actions.review(props.item, 'reject')}
          >
            <X aria-hidden='true' />
            {props.compact ? null : t('Reject')}
          </Button>
        </>
      ) : null}
      {props.item.status === 'paid' ? (
        <Button
          variant='ghost'
          {...common}
          aria-label={t('Freeze payout request')}
          title={t('Freeze payout request')}
          onClick={() => props.actions.mutate(props.item, 'freeze')}
        >
          <Snowflake aria-hidden='true' />
          {props.compact ? null : t('Freeze')}
        </Button>
      ) : null}
      {props.item.status === 'frozen' ? (
        <Button
          variant='ghost'
          {...common}
          aria-label={t('Unfreeze payout request')}
          title={t('Unfreeze payout request')}
          onClick={() => props.actions.mutate(props.item, 'unfreeze')}
        >
          <Sun aria-hidden='true' />
          {props.compact ? null : t('Unfreeze')}
        </Button>
      ) : null}
    </div>
  )
}

function RewardStatusBadge(props: {
  status: InvitationRewardPayoutRequestStatus
}) {
  const { t } = useTranslation()
  let variant: 'default' | 'destructive' | 'outline' = 'default'
  if (props.status === 'rejected') {
    variant = 'destructive'
  } else if (props.status === 'pending') {
    variant = 'outline'
  }
  return <Badge variant={variant}>{t(rewardStatusKey(props.status))}</Badge>
}

function rewardStatusKey(status: InvitationRewardPayoutRequestStatus): string {
  switch (status) {
    case 'pending':
      return 'Pending review'
    case 'paid':
      return 'Paid'
    case 'rejected':
      return 'Rejected'
    case 'frozen':
      return 'Frozen'
    case 'cancelled':
      return 'Cancelled'
  }
}

function ReviewRewardDialog(props: {
  open: boolean
  mode: ReviewMode
  reason: string
  pending: boolean
  onOpenChange: (open: boolean) => void
  onReasonChange: (value: string) => void
  onSubmit: () => void
}) {
  const { t } = useTranslation()
  const reject = props.mode === 'reject'
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={reject ? t('Reject payout request') : t('Approve payout request')}
      description={
        reject
          ? t(
              'Rejected rewards return to the available amount for resubmission.'
            )
          : t(
              'Approval credits the full request amount to the user balance once.'
            )
      }
      contentClassName='sm:max-w-md'
      footer={
        <Button
          variant={reject ? 'destructive' : 'default'}
          disabled={props.pending}
          onClick={props.onSubmit}
        >
          {reject ? t('Reject payout request') : t('Approve payout request')}
        </Button>
      }
    >
      <Textarea
        value={props.reason}
        onChange={(event) => props.onReasonChange(event.target.value)}
        maxLength={500}
        placeholder={
          reject ? t('Enter a rejection reason') : t('Optional review note')
        }
      />
    </Dialog>
  )
}

function MutationRewardDialog(props: {
  open: boolean
  mode: MutationMode
  reason: string
  pending: boolean
  onOpenChange: (open: boolean) => void
  onReasonChange: (value: string) => void
  onSubmit: () => void
}) {
  const { t } = useTranslation()
  const freeze = props.mode === 'freeze'
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={freeze ? t('Freeze payout request') : t('Unfreeze payout request')}
      description={
        freeze
          ? t(
              'The full approved amount will be deducted. The user balance may become negative.'
            )
          : t('The exact frozen amount will be restored to the user balance.')
      }
      contentClassName='sm:max-w-md'
      footer={
        <Button
          variant={freeze ? 'destructive' : 'default'}
          disabled={props.pending}
          onClick={props.onSubmit}
        >
          {freeze ? t('Freeze payout request') : t('Unfreeze payout request')}
        </Button>
      }
    >
      <Textarea
        value={props.reason}
        onChange={(event) => props.onReasonChange(event.target.value)}
        maxLength={500}
        placeholder={t('Enter a reason for this action')}
      />
    </Dialog>
  )
}

function RewardDetailDialog(props: {
  open: boolean
  detail?: InvitationRewardPayoutRequestDetail
  loading: boolean
  error: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const detail = props.detail
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Payout request details')}
      description={
        detail
          ? `${detail.request.display_name || detail.request.username} · ${formatQuota(detail.request.amount)}`
          : undefined
      }
      contentClassName='sm:max-w-3xl'
    >
      {props.loading ? (
        <p className='text-muted-foreground py-8 text-center text-sm'>
          {t('Loading')}
        </p>
      ) : null}
      {props.error ? (
        <p className='text-destructive py-8 text-center text-sm'>
          {t('Unable to load payout request details')}
        </p>
      ) : null}
      {detail ? <RewardDetailContent detail={detail} /> : null}
    </Dialog>
  )
}

function RewardDetailContent({
  detail,
}: {
  detail: InvitationRewardPayoutRequestDetail
}) {
  const { t } = useTranslation()
  const request = detail.request
  return (
    <div className='space-y-5'>
      <dl className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        <DetailTerm
          label={t('Applicant')}
          value={`${request.display_name || request.username} (@${request.username}, #${request.user_id})`}
        />
        <DetailTerm
          label={t('Status')}
          value={t(rewardStatusKey(request.status))}
        />
        <DetailTerm
          label={t('Original request amount')}
          value={formatQuota(request.amount)}
        />
        <DetailTerm
          label={t('Request reversed amount')}
          value={formatQuota(request.reversed_amount)}
        />
        <DetailTerm
          label={t('Current net payout')}
          value={formatQuota(request.net_amount)}
        />
        <DetailTerm
          label={t('Reviewer')}
          value={request.reviewer_username || '-'}
        />
        <DetailTerm
          label={t('Reviewed')}
          value={formatTimestamp(request.reviewed_at)}
        />
        <DetailTerm
          label={t('Approval reason')}
          value={request.reason || '-'}
        />
        <DetailTerm
          label={t('Payout operation ID')}
          value={request.operation_id}
          breakAll
        />
      </dl>
      <DetailSection
        title={t('Reward sources')}
        empty={detail.rewards.length === 0}
      >
        {detail.rewards.map((reward) => (
          <div
            key={reward.reward_id}
            className='grid gap-2 border-b py-3 last:border-b-0 sm:grid-cols-2'
          >
            <DetailTerm
              label={t('Source account')}
              value={`${reward.source_display_name || reward.source_username} (@${reward.source_username}, #${reward.source_user_id})`}
            />
            <DetailTerm
              label={t('Trigger type')}
              value={t(rewardTriggerTypeKey(reward.trigger_type))}
            />
            <CopyableDetailTerm
              label={t('Trigger ID')}
              value={reward.trigger_id}
            />
            <DetailTerm
              label={t('Eligible quota')}
              value={formatQuota(reward.eligible_quota)}
            />
            <DetailTerm
              label={t('Commission rate')}
              value={formatBasisPoints(reward.rate_basis_points)}
            />
            <DetailTerm
              label={t('Gross reward')}
              value={formatQuota(reward.amount)}
            />
            <DetailTerm
              label={t('Reversed amount')}
              value={formatQuota(reward.reversed_amount)}
            />
            <DetailTerm
              label={t('Net reward')}
              value={formatQuota(reward.net_amount)}
            />
            <DetailTerm
              label={t('Reversal status')}
              value={t(rewardReversalStatusKey(reward.reversal_state))}
            />
            <DetailTerm
              label={t('Registration IP')}
              value={reward.registration_ip || '-'}
              breakAll
            />
            <DetailTerm
              label={t('Registered')}
              value={formatTimestamp(reward.registered_at)}
            />
            <DetailTerm
              label={t('Invitation code')}
              value={reward.invite_code || '-'}
              breakAll
            />
            <DetailTerm
              label={t('Reward status')}
              value={t(rewardDetailStatusKey(reward.status))}
            />
          </div>
        ))}
      </DetailSection>
      <DetailSection
        title={t('Freeze history')}
        empty={detail.actions.length === 0}
      >
        {detail.actions.map((action) => (
          <div
            key={action.id}
            className='grid gap-2 border-b py-3 last:border-b-0 sm:grid-cols-2'
          >
            <DetailTerm
              label={t('Action')}
              value={action.action === 'freeze' ? t('Frozen') : t('Unfrozen')}
            />
            <DetailTerm label={t('Revision')} value={String(action.revision)} />
            <DetailTerm
              label={t('Operator')}
              value={`${action.actor_display_name || action.actor_username} (@${action.actor_username})`}
            />
            <DetailTerm
              label={t('Time')}
              value={formatTimestamp(action.created_at)}
            />
            <DetailTerm label={t('Reason')} value={action.reason} />
            <DetailTerm
              label={t('Wallet operation ID')}
              value={action.wallet_operation_id}
              breakAll
            />
          </div>
        ))}
      </DetailSection>
    </div>
  )
}

function rewardDetailStatusKey(status: string): string {
  if (status === 'frozen') return 'Frozen'
  if (status === 'paid') return 'Paid'
  return status
}

function rewardTriggerTypeKey(triggerType: string): string {
  switch (triggerType) {
    case 'registration_inviter':
      return 'Inviter registration reward'
    case 'cash_topup':
      return 'Cash top-up'
    case 'cash_subscription':
      return 'Cash subscription purchase'
    case 'admin_balance_redemption':
      return 'Administrator balance code'
    case 'admin_subscription_redemption':
      return 'Administrator subscription code'
    default:
      return triggerType
  }
}

function rewardReversalStatusKey(status: string): string {
  if (status === 'none') return 'Not reversed'
  if (status === 'partial') return 'Partially reversed'
  if (status === 'full') return 'Fully reversed'
  return status
}

function formatBasisPoints(value: number): string {
  return `${(value / 100).toLocaleString(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  })}%`
}

function DetailSection(props: {
  title: string
  empty: boolean
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  return (
    <section>
      <h3 className='text-sm font-semibold'>{props.title}</h3>
      <div className='mt-2 rounded-md border px-3'>
        {props.empty ? (
          <p className='text-muted-foreground py-5 text-center text-sm'>
            {t('No records')}
          </p>
        ) : (
          props.children
        )}
      </div>
    </section>
  )
}

function DetailTerm(props: {
  label: string
  value: string
  breakAll?: boolean
}) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-xs'>{props.label}</dt>
      <dd
        className={`mt-0.5 text-sm ${props.breakAll ? 'font-mono text-xs break-all' : 'break-words'}`}
      >
        {props.value}
      </dd>
    </div>
  )
}

function CopyableDetailTerm(props: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-xs'>{props.label}</dt>
      <dd className='mt-0.5 flex min-w-0 items-start gap-1 text-xs'>
        <code className='min-w-0 break-all'>{props.value || '-'}</code>
        {props.value ? (
          <CopyButton
            value={props.value}
            className='size-7'
            iconClassName='size-3.5'
            tooltip={props.label}
          />
        ) : null}
      </dd>
    </div>
  )
}

async function invalidateRewardRequests(
  queryClient: ReturnType<typeof useQueryClient>
) {
  await queryClient.invalidateQueries({
    queryKey: ['invitation', 'admin', 'reward-payout-requests'],
  })
  await queryClient.invalidateQueries({
    queryKey: ['invitation', 'admin', 'reward-payout-request-detail'],
  })
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}
