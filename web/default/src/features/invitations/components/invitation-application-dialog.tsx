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
import { Loader2, Send } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

interface InvitationApplicationDialogProps {
  open: boolean
  pending: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (reason: string) => void
}

export function InvitationApplicationDialog(
  props: InvitationApplicationDialogProps
) {
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
      title={t('Apply for invitation code')}
      description={t(
        'Tell the administrator who you plan to invite and why you need a code.'
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
            disabled={
              props.pending ||
              normalizedReason.length < 5 ||
              normalizedReason.length > 500
            }
            onClick={() => props.onSubmit(normalizedReason)}
          >
            {props.pending ? (
              <Loader2 className='animate-spin' aria-hidden='true' />
            ) : (
              <Send aria-hidden='true' />
            )}
            {t('Submit application')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label htmlFor='invitation-application-reason'>
          {t('Application reason')}
        </Label>
        <Textarea
          id='invitation-application-reason'
          value={reason}
          minLength={5}
          maxLength={500}
          rows={5}
          placeholder={t('Describe who you plan to invite and why.')}
          onChange={(event) => setReason(event.target.value)}
        />
        <div className='text-muted-foreground text-right text-xs tabular-nums'>
          {reason.length}/500
        </div>
      </div>
    </Dialog>
  )
}
