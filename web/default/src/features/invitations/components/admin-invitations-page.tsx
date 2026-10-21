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
import { Check, Loader2, Plus, Search, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  issueInvitationCode,
  listInvitationApplications,
  listInvitationCodes,
  listInvitationUses,
  reviewInvitationApplication,
  revokeInvitationCode,
} from '../api'
import {
  formatInvitationRewardStatus,
  getInvitationErrorMessage,
} from '../lib/invitation-state'
import type {
  InvitationApplicationListItem,
  InvitationCodeListItem,
  InvitationListQuery,
  InvitationUse,
} from '../types'
import {
  IssueInvitationDialog,
  type IssueSubmission,
  ReviewInvitationDialog,
  type ReviewSubmission,
  RevokeInvitationDialog,
} from './admin-invitation-dialogs'

type InvitationTab = 'applications' | 'codes' | 'uses'

export function AdminInvitationsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<InvitationTab>('applications')
  const [page, setPage] = useState(1)
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState('')
  const [issueOpen, setIssueOpen] = useState(false)
  const [reviewTarget, setReviewTarget] =
    useState<InvitationApplicationListItem | null>(null)
  const [reviewApprove, setReviewApprove] = useState(true)
  const [revokeTarget, setRevokeTarget] =
    useState<InvitationCodeListItem | null>(null)
  const listQuery: InvitationListQuery = {
    page,
    pageSize: 20,
    keyword,
    status,
  }
  const applications = useQuery({
    queryKey: ['invitation', 'admin', 'applications', listQuery],
    queryFn: () => listInvitationApplications(listQuery),
    enabled: tab === 'applications',
  })
  const codes = useQuery({
    queryKey: ['invitation', 'admin', 'codes', listQuery],
    queryFn: () => listInvitationCodes(listQuery),
    enabled: tab === 'codes',
  })
  const uses = useQuery({
    queryKey: ['invitation', 'admin', 'uses', listQuery],
    queryFn: () => listInvitationUses(listQuery),
    enabled: tab === 'uses',
  })
  const reviewMutation = useMutation({
    mutationFn: (value: { applicationId: number; input: ReviewSubmission }) =>
      reviewInvitationApplication(value.applicationId, value.input),
    onSuccess: async () => {
      setReviewTarget(null)
      toast.success(t('Application reviewed'))
      await invalidateInvitationQueries(queryClient)
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  const issueMutation = useMutation({
    mutationFn: issueInvitationCode,
    onSuccess: async () => {
      setIssueOpen(false)
      toast.success(t('Invitation code issued'))
      await invalidateInvitationQueries(queryClient)
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  const revokeMutation = useMutation({
    mutationFn: (value: { id: number; reason: string }) =>
      revokeInvitationCode(value.id, value.reason),
    onSuccess: async () => {
      setRevokeTarget(null)
      toast.success(t('Invitation code revoked'))
      await invalidateInvitationQueries(queryClient)
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })

  function changeTab(value: string) {
    setTab(value as InvitationTab)
    setPage(1)
    setStatus('')
  }

  function openReview(item: InvitationApplicationListItem, approve: boolean) {
    setReviewApprove(approve)
    setReviewTarget(item)
  }

  function submitSearch(event: React.FormEvent) {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
  }

  let total = uses.data?.total ?? 0
  if (tab === 'applications') {
    total = applications.data?.total ?? 0
  } else if (tab === 'codes') {
    total = codes.data?.total ?? 0
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Invitation codes')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button onClick={() => setIssueOpen(true)} className='gap-2'>
            <Plus aria-hidden='true' />
            {t('Issue code')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3'>
            <Tabs
              value={tab}
              onValueChange={changeTab}
              className='min-h-0 flex-1'
            >
              <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                <TabsList>
                  <TabsTrigger value='applications'>
                    {t('Applications')}
                  </TabsTrigger>
                  <TabsTrigger value='codes'>{t('Issued codes')}</TabsTrigger>
                  <TabsTrigger value='uses'>{t('Registrations')}</TabsTrigger>
                </TabsList>
                <form
                  onSubmit={submitSearch}
                  className='flex min-w-0 flex-1 items-center justify-end gap-2'
                >
                  <NativeSelect
                    value={status}
                    onChange={(event) => {
                      setStatus(event.target.value)
                      setPage(1)
                    }}
                    aria-label={t('Filter by status')}
                  >
                    <NativeSelectOption value=''>
                      {t('All statuses')}
                    </NativeSelectOption>
                    {statusOptions(tab).map((option) => (
                      <NativeSelectOption key={option} value={option}>
                        {t(option)}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                  <Input
                    value={keywordInput}
                    onChange={(event) => setKeywordInput(event.target.value)}
                    placeholder={t('Search users or codes')}
                    className='max-w-xs'
                  />
                  <Button
                    type='submit'
                    variant='outline'
                    size='icon'
                    aria-label={t('Search')}
                  >
                    <Search aria-hidden='true' />
                  </Button>
                </form>
              </div>

              <TabsContent
                value='applications'
                className='min-h-0 overflow-auto'
              >
                <ApplicationsTable
                  items={applications.data?.items ?? []}
                  loading={applications.isLoading}
                  failed={applications.isError}
                  onRetry={() => applications.refetch()}
                  onReview={openReview}
                />
              </TabsContent>
              <TabsContent value='codes' className='min-h-0 overflow-auto'>
                <CodesTable
                  items={codes.data?.items ?? []}
                  loading={codes.isLoading}
                  failed={codes.isError}
                  onRetry={() => codes.refetch()}
                  onRevoke={setRevokeTarget}
                />
              </TabsContent>
              <TabsContent value='uses' className='min-h-0 overflow-auto'>
                <UsesTable
                  items={uses.data?.items ?? []}
                  loading={uses.isLoading}
                  failed={uses.isError}
                  onRetry={() => uses.refetch()}
                />
              </TabsContent>
            </Tabs>
            <Pagination
              page={page}
              pageSize={20}
              total={total}
              onPageChange={setPage}
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ReviewInvitationDialog
        open={reviewTarget !== null}
        onOpenChange={(open) => !open && setReviewTarget(null)}
        application={reviewTarget}
        approve={reviewApprove}
        pending={reviewMutation.isPending}
        onSubmit={(input) => {
          if (reviewTarget) {
            reviewMutation.mutate({ applicationId: reviewTarget.id, input })
          }
        }}
      />
      <IssueInvitationDialog
        open={issueOpen}
        onOpenChange={setIssueOpen}
        pending={issueMutation.isPending}
        onSubmit={(input: IssueSubmission) => issueMutation.mutate(input)}
      />
      <RevokeInvitationDialog
        open={revokeTarget !== null}
        onOpenChange={(open) => !open && setRevokeTarget(null)}
        code={revokeTarget}
        pending={revokeMutation.isPending}
        onSubmit={(reason) => {
          if (revokeTarget) {
            revokeMutation.mutate({ id: revokeTarget.id, reason })
          }
        }}
      />
    </>
  )
}

function ApplicationsTable(props: {
  items: InvitationApplicationListItem[]
  loading: boolean
  failed: boolean
  onRetry: () => void
  onReview: (item: InvitationApplicationListItem, approve: boolean) => void
}) {
  const { t } = useTranslation()
  return (
    <InvitationTableFrame
      loading={props.loading}
      failed={props.failed}
      empty={props.items.length === 0}
      onRetry={props.onRetry}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Applicant')}</TableHead>
            <TableHead>{t('Reason')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Submitted')}</TableHead>
            <TableHead className='text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>{item.applicant_username}</TableCell>
              <TableCell className='max-w-80 truncate'>{item.reason}</TableCell>
              <TableCell>
                <StatusBadge value={item.status} />
              </TableCell>
              <TableCell>{formatTimestamp(item.created_at)}</TableCell>
              <TableCell>
                <div className='flex justify-end gap-1'>
                  <Button
                    variant='ghost'
                    size='icon'
                    disabled={item.status !== 'pending'}
                    aria-label={t('Approve')}
                    onClick={() => props.onReview(item, true)}
                  >
                    <Check aria-hidden='true' />
                  </Button>
                  <Button
                    variant='ghost'
                    size='icon'
                    disabled={item.status !== 'pending'}
                    aria-label={t('Reject')}
                    onClick={() => props.onReview(item, false)}
                  >
                    <X aria-hidden='true' />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </InvitationTableFrame>
  )
}

function CodesTable(props: {
  items: InvitationCodeListItem[]
  loading: boolean
  failed: boolean
  onRetry: () => void
  onRevoke: (item: InvitationCodeListItem) => void
}) {
  const { t } = useTranslation()
  return (
    <InvitationTableFrame
      loading={props.loading}
      failed={props.failed}
      empty={props.items.length === 0}
      onRetry={props.onRetry}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Owner')}</TableHead>
            <TableHead>{t('Code')}</TableHead>
            <TableHead>{t('Usage')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Expires')}</TableHead>
            <TableHead className='text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>{item.owner_username}</TableCell>
              <TableCell>
                <div className='flex items-center gap-1'>
                  <code className='max-w-52 truncate'>{item.code}</code>
                  <CopyButton
                    value={item.code}
                    tooltip={t('Copy invitation code')}
                  />
                </div>
              </TableCell>
              <TableCell>
                {item.used_count}/{item.max_uses}
              </TableCell>
              <TableCell>
                <StatusBadge value={item.status} />
              </TableCell>
              <TableCell>{formatTimestamp(item.expires_at)}</TableCell>
              <TableCell className='text-right'>
                <Button
                  variant='outline'
                  size='sm'
                  disabled={item.status !== 'active'}
                  onClick={() => props.onRevoke(item)}
                >
                  {t('Revoke')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </InvitationTableFrame>
  )
}

function UsesTable(props: {
  items: InvitationUse[]
  loading: boolean
  failed: boolean
  onRetry: () => void
}) {
  const { t } = useTranslation()
  return (
    <InvitationTableFrame
      loading={props.loading}
      failed={props.failed}
      empty={props.items.length === 0}
      onRetry={props.onRetry}
    >
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Inviter')}</TableHead>
            <TableHead>{t('Registered user')}</TableHead>
            <TableHead>{t('Code')}</TableHead>
            <TableHead>{t('Source')}</TableHead>
            <TableHead>{t('IP address')}</TableHead>
            <TableHead>{t('Registered')}</TableHead>
            <TableHead>{t('Reward status')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>{item.inviter_username}</TableCell>
              <TableCell>{item.registered_username}</TableCell>
              <TableCell>
                <code>{item.invite_code}</code>
              </TableCell>
              <TableCell>{item.source || '-'}</TableCell>
              <TableCell className='font-mono'>
                {item.registration_ip || '-'}
              </TableCell>
              <TableCell>{formatTimestamp(item.registered_at)}</TableCell>
              <TableCell>
                {formatInvitationRewardStatus(
                  item.inviter_reward_status,
                  item.invitee_reward_status,
                  t
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </InvitationTableFrame>
  )
}

function InvitationTableFrame(props: {
  loading: boolean
  failed: boolean
  empty: boolean
  onRetry: () => void
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='flex h-48 items-center justify-center'>
        <Loader2 className='size-5 animate-spin' aria-label={t('Loading')} />
      </div>
    )
  }
  if (props.failed) {
    return (
      <div className='flex h-48 flex-col items-center justify-center gap-3 text-sm'>
        <span>{t('Unable to load invitation records')}</span>
        <Button variant='outline' onClick={props.onRetry}>
          {t('Retry')}
        </Button>
      </div>
    )
  }
  if (props.empty) {
    return (
      <div className='text-muted-foreground flex h-48 items-center justify-center text-sm'>
        {t('No invitation records found.')}
      </div>
    )
  }
  return <div className='rounded-md border'>{props.children}</div>
}

function StatusBadge(props: { value: string }) {
  const { t } = useTranslation()
  let variant: 'default' | 'destructive' | 'outline' = 'outline'
  if (props.value === 'active' || props.value === 'approved') {
    variant = 'default'
  } else if (props.value === 'rejected' || props.value === 'revoked') {
    variant = 'destructive'
  }
  return <Badge variant={variant}>{t(props.value)}</Badge>
}

function Pagination(props: {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const pages = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='flex items-center justify-between gap-3 text-sm'>
      <span className='text-muted-foreground'>
        {t('{{count}} records', { count: props.total })}
      </span>
      <div className='flex items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={props.page <= 1}
          onClick={() => props.onPageChange(props.page - 1)}
        >
          {t('Previous')}
        </Button>
        <span className='tabular-nums'>
          {props.page}/{pages}
        </span>
        <Button
          variant='outline'
          size='sm'
          disabled={props.page >= pages}
          onClick={() => props.onPageChange(props.page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}

function statusOptions(tab: InvitationTab): string[] {
  if (tab === 'applications') {
    return ['pending', 'approved', 'rejected', 'withdrawn']
  }
  if (tab === 'codes') {
    return ['active', 'exhausted', 'expired', 'revoked']
  }
  return ['pending', 'completed', 'skipped', 'failed']
}

function formatTimestamp(timestamp: number): string {
  return timestamp ? new Date(timestamp * 1000).toLocaleString() : '-'
}

async function invalidateInvitationQueries(
  queryClient: ReturnType<typeof useQueryClient>
) {
  await queryClient.invalidateQueries({ queryKey: ['invitation'] })
}
