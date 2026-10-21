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
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

interface InvitationRevokeDialogProps {
  open: boolean
  pending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: (reason: string) => void
}

export function InvitationRevokeDialog(props: InvitationRevokeDialogProps) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')
  const normalizedReason = reason.trim()

  function handleOpenChange(open: boolean) {
    if (!open) setReason('')
    props.onOpenChange(open)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Revoke invitation code?')}
      description={t(
        'This code will stop accepting new registrations. Existing registrations remain unchanged.'
      )}
      contentClassName='sm:max-w-lg'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={props.pending}
            onClick={() => handleOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            disabled={
              props.pending ||
              normalizedReason.length < 1 ||
              normalizedReason.length > 500
            }
            onClick={() => props.onConfirm(normalizedReason)}
          >
            {props.pending ? (
              <Loader2 className='animate-spin' aria-hidden='true' />
            ) : null}
            {t('Revoke invitation code')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label htmlFor='invitation-revoke-reason'>{t('Reason')}</Label>
        <Textarea
          id='invitation-revoke-reason'
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
    </Dialog>
  )
}
