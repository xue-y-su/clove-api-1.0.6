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
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { formatQuota } from '@/lib/format'

import {
  deleteAdminUserRedemption,
  disableAdminUserRedemption,
  enableAdminUserRedemption,
  terminateAdminUserRedemption,
  type UserRedemptionCode,
} from '../api'

export type AdminUserRedemptionAction =
  | 'disable'
  | 'enable'
  | 'terminate'
  | 'delete'

interface AdminUserRedemptionGovernanceDialogProps {
  target: UserRedemptionCode | null
  action: AdminUserRedemptionAction | null
  onOpenChange: (open: boolean) => void
  onCompleted: () => void | Promise<void>
}

export function AdminUserRedemptionGovernanceDialog(
  props: AdminUserRedemptionGovernanceDialogProps
) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')
  const [settlement, setSettlement] = useState<'refund' | 'forfeit' | ''>('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const isLive =
    props.target?.status === 'active' ||
    props.target?.status === 'disabled' ||
    props.target?.status === 'expired_pending'
  const isSubscription = props.target?.reward_type === 'subscription'
  const settlementRequired =
    props.action === 'terminate' || (props.action === 'delete' && isLive)
  const normalizedReason = reason.trim()

  function reset() {
    setReason('')
    setSettlement('')
    setError('')
  }

  function handleOpenChange(open: boolean) {
    if (!open && !busy) reset()
    props.onOpenChange(open)
  }

  async function submit() {
    if (!props.target || !props.action || !normalizedReason) return
    if (settlementRequired && !settlement) return

    setBusy(true)
    setError('')
    try {
      switch (props.action) {
        case 'disable':
          await disableAdminUserRedemption(props.target.id, normalizedReason)
          break
        case 'enable':
          await enableAdminUserRedemption(props.target.id, normalizedReason)
          break
        case 'terminate':
          await terminateAdminUserRedemption(
            props.target.id,
            settlement as 'refund' | 'forfeit',
            normalizedReason
          )
          break
        case 'delete':
          await deleteAdminUserRedemption(
            props.target.id,
            settlementRequired ? settlement : '',
            normalizedReason
          )
          break
      }
      reset()
      props.onOpenChange(false)
      await props.onCompleted()
    } catch (caught: unknown) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t('Request failed')
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={props.target !== null && props.action !== null}
      onOpenChange={handleOpenChange}
      title={getActionTitle(props.action, t)}
      description={t(
        'This administrative action is recorded in the user log and audit log.'
      )}
      contentClassName='sm:max-w-lg'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={busy}
            onClick={() => handleOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            variant={props.action === 'delete' ? 'destructive' : 'default'}
            disabled={
              busy ||
              normalizedReason.length === 0 ||
              normalizedReason.length > 500 ||
              (settlementRequired && !settlement)
            }
            onClick={() => void submit()}
          >
            {busy ? (
              <Loader2 className='animate-spin' aria-hidden='true' />
            ) : null}
            {t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='space-y-5'>
        {props.target ? (
          <div className='bg-muted/35 rounded-md border px-3 py-2 text-sm'>
            <div className='font-mono'>{props.target.code}</div>
            <div className='text-muted-foreground mt-1 text-xs'>
              {t('Remaining')}:{' '}
              {isSubscription
                ? `${Math.max(0, props.target.max_claims - props.target.claim_count)} ${t('units')}`
                : formatQuota(props.target.remaining_quota)}
            </div>
          </div>
        ) : null}

        {settlementRequired ? (
          <div className='space-y-2'>
            <Label>{t('Settlement')}</Label>
            <div
              className='bg-muted grid grid-cols-2 rounded-md p-1'
              role='group'
              aria-label={t('Settlement')}
            >
              <Button
                type='button'
                variant={settlement === 'refund' ? 'secondary' : 'ghost'}
                aria-pressed={settlement === 'refund'}
                onClick={() => setSettlement('refund')}
              >
                {isSubscription
                  ? t('Refund remaining units')
                  : t('Refund remaining balance')}
              </Button>
              <Button
                type='button'
                variant={settlement === 'forfeit' ? 'secondary' : 'ghost'}
                aria-pressed={settlement === 'forfeit'}
                onClick={() => setSettlement('forfeit')}
              >
                {isSubscription
                  ? t('Forfeit remaining units')
                  : t('Forfeit remaining balance')}
              </Button>
            </div>
            <p className='text-muted-foreground text-xs'>
              {settlement === 'refund'
                ? t(
                    isSubscription
                      ? 'The unclaimed subscription units will return to the creator wallet at their reserved cost.'
                      : 'The unclaimed balance will return to the creator wallet.'
                  )
                : t(
                    isSubscription
                      ? 'No wallet refund is issued when the remaining units are forfeited.'
                      : 'No wallet refund is issued when the balance is forfeited.'
                  )}
            </p>
          </div>
        ) : null}

        <div className='space-y-2'>
          <Label htmlFor='user-redemption-governance-reason'>
            {t('Reason')}
          </Label>
          <Textarea
            id='user-redemption-governance-reason'
            value={reason}
            maxLength={500}
            rows={4}
            placeholder={t('Enter a reason for this action')}
            onChange={(event) => setReason(event.target.value)}
          />
          <div className='text-muted-foreground text-right text-xs tabular-nums'>
            {reason.length}/500
          </div>
        </div>

        {error ? (
          <Alert variant='destructive'>
            <AlertTitle>{t('Action failed')}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
      </div>
    </Dialog>
  )
}

function getActionTitle(
  action: AdminUserRedemptionAction | null,
  translate: (key: string) => string
): string {
  switch (action) {
    case 'disable':
      return translate('Disable')
    case 'enable':
      return translate('Enable')
    case 'terminate':
      return translate('Terminate')
    case 'delete':
      return translate('Delete')
    default:
      return translate('Manage')
  }
}
