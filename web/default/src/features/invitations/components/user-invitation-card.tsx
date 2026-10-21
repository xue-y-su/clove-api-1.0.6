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
import { QrCode, RefreshCw, Send, TicketCheck, X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useAuthStore } from '@/stores/auth-store'

import {
  applyForInvitation,
  getInvitationSelf,
  revokeOwnInvitationCode,
  withdrawInvitationApplication,
} from '../api'
import {
  buildInvitationRegistrationUrl,
  getInvitationErrorMessage,
  getInvitationReapplyRefreshDelay,
  getInvitationRemainingUses,
  getInvitationViewState,
} from '../lib/invitation-state'
import { InvitationApplicationDialog } from './invitation-application-dialog'
import { InvitationQRDialog } from './invitation-qr-dialog'
import { InvitationRevokeDialog } from './invitation-revoke-dialog'

const INVITATION_SELF_KEY = ['invitation', 'self'] as const

export function UserInvitationCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { status } = useStatus()
  const { systemName, logo } = useSystemConfig()
  const user = useAuthStore((state) => state.auth.user)
  const [applyOpen, setApplyOpen] = useState(false)
  const [qrOpen, setQrOpen] = useState(false)
  const [revokeOpen, setRevokeOpen] = useState(false)
  const refreshedReapplyAt = useRef(0)
  const invitationQuery = useQuery({
    queryKey: INVITATION_SELF_KEY,
    queryFn: getInvitationSelf,
  })
  const reapplyAt = invitationQuery.data?.reapply_at ?? 0
  const canApply = invitationQuery.data?.can_apply ?? false
  const refetchInvitation = invitationQuery.refetch
  useEffect(() => {
    const delay = getInvitationReapplyRefreshDelay(reapplyAt, canApply)
    if (delay === null) {
      refreshedReapplyAt.current = 0
      return
    }
    if (refreshedReapplyAt.current === reapplyAt) return

    const timer = window.setTimeout(() => {
      if (Date.now() >= reapplyAt * 1000) {
        refreshedReapplyAt.current = reapplyAt
      }
      void refetchInvitation()
    }, delay)
    return () => window.clearTimeout(timer)
  }, [canApply, reapplyAt, refetchInvitation])
  const applyMutation = useMutation({
    mutationFn: applyForInvitation,
    onSuccess: async () => {
      setApplyOpen(false)
      toast.success(t('Invitation application submitted'))
      await queryClient.invalidateQueries({ queryKey: INVITATION_SELF_KEY })
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  const withdrawMutation = useMutation({
    mutationFn: withdrawInvitationApplication,
    onSuccess: async () => {
      toast.success(t('Invitation application withdrawn'))
      await queryClient.invalidateQueries({ queryKey: INVITATION_SELF_KEY })
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  const revokeMutation = useMutation({
    mutationFn: async (reason: string) => {
      const codeId = invitationQuery.data?.code?.id
      if (!codeId) throw new Error(t('Invitation code not found'))
      return revokeOwnInvitationCode(codeId, reason)
    },
    onSuccess: async () => {
      setRevokeOpen(false)
      toast.success(t('Invitation code revoked'))
      await queryClient.invalidateQueries({ queryKey: INVITATION_SELF_KEY })
    },
    onError: (error) => {
      toast.error(getInvitationErrorMessage(error, t('Request failed')))
    },
  })
  const view = getInvitationViewState(
    invitationQuery.data,
    invitationQuery.isLoading,
    invitationQuery.isError
  )
  const registrationUrl = useMemo(() => {
    const code = invitationQuery.data?.code?.code
    if (!code || typeof window === 'undefined') return ''
    return buildInvitationRegistrationUrl(window.location.origin, code)
  }, [invitationQuery.data?.code?.code])
  const posterTitleValue =
    status?.invitation_poster_title ?? status?.data?.invitation_poster_title
  const posterSubtitleValue =
    status?.invitation_poster_subtitle ??
    status?.data?.invitation_poster_subtitle
  const displayName = user?.display_name?.trim() || user?.username || ''

  return (
    <>
      <Card data-card-hover='false'>
        <CardHeader className='gap-1 pb-3'>
          <div className='flex items-center justify-between gap-3'>
            <CardTitle className='flex items-center gap-2 text-base'>
              <TicketCheck className='text-primary size-4' aria-hidden='true' />
              {t('My invitation code')}
            </CardTitle>
            {invitationQuery.data?.code ? (
              <Badge
                variant={
                  invitationQuery.data.code.status === 'active'
                    ? 'default'
                    : 'outline'
                }
              >
                {t(invitationQuery.data.code.status)}
              </Badge>
            ) : null}
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Apply for a code, then share it after an administrator approves it.'
            )}
          </p>
        </CardHeader>
        <CardContent className='space-y-4'>
          {view === 'loading' ? <InvitationSkeleton /> : null}
          {view === 'error' ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Unable to load invitation details')}</AlertTitle>
              <AlertDescription className='flex items-center justify-between gap-3'>
                <span>{t('Check your connection and try again.')}</span>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => invitationQuery.refetch()}
                  className='gap-2'
                >
                  <RefreshCw aria-hidden='true' />
                  {t('Retry')}
                </Button>
              </AlertDescription>
            </Alert>
          ) : null}

          {view === 'active' && invitationQuery.data?.code ? (
            <ActiveInvitation
              code={invitationQuery.data.code}
              uses={invitationQuery.data.uses}
              registrationUrl={registrationUrl}
              onOpenQR={() => setQrOpen(true)}
              onRevoke={() => setRevokeOpen(true)}
              revokePending={revokeMutation.isPending}
            />
          ) : null}

          {view === 'pending' && invitationQuery.data?.application ? (
            <Alert>
              <AlertTitle>{t('Application pending')}</AlertTitle>
              <AlertDescription className='space-y-3'>
                <p>{invitationQuery.data.application.reason}</p>
                <Button
                  variant='outline'
                  size='sm'
                  className='gap-2'
                  disabled={withdrawMutation.isPending}
                  onClick={() =>
                    withdrawMutation.mutate(
                      invitationQuery.data?.application?.id ?? 0
                    )
                  }
                >
                  <X aria-hidden='true' />
                  {t('Withdraw application')}
                </Button>
              </AlertDescription>
            </Alert>
          ) : null}

          {view === 'rejected' && invitationQuery.data?.application ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Application rejected')}</AlertTitle>
              <AlertDescription>
                {invitationQuery.data.application.review_reason ||
                  t('No review reason was provided.')}
                {invitationQuery.data.reapply_at ? (
                  <span className='mt-1 block'>
                    {t('You can apply again after {{time}}', {
                      time: formatTimestamp(invitationQuery.data.reapply_at),
                    })}
                  </span>
                ) : null}
              </AlertDescription>
            </Alert>
          ) : null}

          {view === 'terminal' && invitationQuery.data?.code ? (
            <Alert>
              <AlertTitle>
                {t('Invitation code status: {{status}}', {
                  status: t(invitationQuery.data.code.status),
                })}
              </AlertTitle>
              <AlertDescription>
                {t('Submit a new application when you need another code.')}
              </AlertDescription>
            </Alert>
          ) : null}

          {view !== 'loading' &&
          view !== 'error' &&
          view !== 'active' &&
          view !== 'pending' &&
          invitationQuery.data?.can_apply ? (
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <p className='text-muted-foreground text-sm'>
                {t('Request an invitation code from an administrator.')}
              </p>
              <Button
                onClick={() => setApplyOpen(true)}
                disabled={applyMutation.isPending}
                className='gap-2'
              >
                <Send aria-hidden='true' />
                {t('Apply for invitation code')}
              </Button>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <InvitationQRDialog
        open={qrOpen}
        onOpenChange={setQrOpen}
        value={registrationUrl}
        code={invitationQuery.data?.code?.code ?? ''}
        systemName={systemName}
        logo={logo}
        displayName={displayName}
        title={typeof posterTitleValue === 'string' ? posterTitleValue : ''}
        subtitle={
          typeof posterSubtitleValue === 'string' ? posterSubtitleValue : ''
        }
      />
      <InvitationApplicationDialog
        open={applyOpen}
        pending={applyMutation.isPending}
        onOpenChange={setApplyOpen}
        onSubmit={(reason) => applyMutation.mutate(reason)}
      />
      <InvitationRevokeDialog
        open={revokeOpen}
        onOpenChange={setRevokeOpen}
        pending={revokeMutation.isPending}
        onConfirm={(reason) => revokeMutation.mutate(reason)}
      />
    </>
  )
}

function ActiveInvitation(props: {
  code: NonNullable<Awaited<ReturnType<typeof getInvitationSelf>>['code']>
  uses: Awaited<ReturnType<typeof getInvitationSelf>>['uses']
  registrationUrl: string
  onOpenQR: () => void
  onRevoke: () => void
  revokePending: boolean
}) {
  const { t } = useTranslation()
  const remaining = getInvitationRemainingUses(props.code)
  return (
    <div className='space-y-4'>
      <div className='flex min-w-0 items-center gap-2'>
        <Input
          value={props.code.code}
          readOnly
          aria-label={t('Invitation code')}
          className='min-w-0 flex-1 font-mono'
        />
        <CopyButton
          value={props.code.code}
          tooltip={t('Copy invitation code')}
        />
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                type='button'
                variant='outline'
                size='icon'
                aria-label={t('Revoke invitation code')}
                disabled={props.revokePending}
                onClick={props.onRevoke}
              />
            }
          >
            <X aria-hidden='true' />
          </TooltipTrigger>
          <TooltipContent>{t('Revoke invitation code')}</TooltipContent>
        </Tooltip>
      </div>
      <div className='space-y-1.5'>
        <div className='text-muted-foreground text-xs'>
          {t('Invitation link')}
        </div>
        <div className='flex min-w-0 items-center gap-2'>
          <Input
            value={props.registrationUrl}
            readOnly
            aria-label={t('Invitation link')}
            className='min-w-0 flex-1'
          />
          <CopyButton
            value={props.registrationUrl}
            tooltip={t('Copy invitation link')}
          />
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  type='button'
                  variant='outline'
                  size='icon'
                  aria-label={t('Show invitation QR code')}
                  onClick={props.onOpenQR}
                />
              }
            >
              <QrCode aria-hidden='true' />
            </TooltipTrigger>
            <TooltipContent>{t('Show invitation QR code')}</TooltipContent>
          </Tooltip>
        </div>
      </div>
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
        <Metric label={t('Type')} value={t(props.code.kind)} />
        <Metric label={t('Used')} value={String(props.code.used_count)} />
        <Metric label={t('Remaining')} value={String(remaining)} />
        <Metric
          label={t('Expires')}
          value={formatTimestamp(props.code.expires_at)}
        />
      </div>
      {props.uses.length > 0 ? (
        <div className='space-y-2'>
          <h4 className='text-sm font-medium'>{t('Recent registrations')}</h4>
          <div className='divide-y rounded-md border'>
            {props.uses.map((use) => (
              <div
                key={use.id}
                className='flex items-center justify-between gap-3 px-3 py-2 text-sm'
              >
                <span className='truncate'>
                  {t('User #{{id}}', { id: use.registered_user_id })}
                </span>
                <span className='text-muted-foreground shrink-0 text-xs'>
                  {formatTimestamp(use.registered_at)}
                </span>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t('No one has registered with this code yet.')}
        </p>
      )}
    </div>
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

function InvitationSkeleton() {
  return (
    <div className='space-y-3'>
      <Skeleton className='h-9 w-full' />
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className='h-14 w-full' />
        ))}
      </div>
    </div>
  )
}

function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}
