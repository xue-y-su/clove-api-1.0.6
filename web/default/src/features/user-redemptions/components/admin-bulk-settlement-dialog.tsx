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

import {
  batchSettleAdminUserRedemptions,
  type UserRedemptionBatchDispositionResult,
} from '../api'

type AdminBulkSettlementDialogProps = {
  open: boolean
  ids: number[]
  onOpenChange: (open: boolean) => void
  onCompleted: () => void | Promise<void>
}

export function AdminBulkSettlementDialog(
  props: AdminBulkSettlementDialogProps
) {
  const { t } = useTranslation()
  const [reason, setReason] = useState('')
  const [disposition, setDisposition] = useState<'refund' | 'forfeit' | ''>('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [results, setResults] = useState<
    UserRedemptionBatchDispositionResult[]
  >([])
  const normalizedReason = reason.trim()

  function reset() {
    setReason('')
    setDisposition('')
    setError('')
    setResults([])
  }

  function handleOpenChange(open: boolean) {
    if (!open && !busy) reset()
    props.onOpenChange(open)
  }

  async function submit() {
    if (!props.ids.length || !normalizedReason || !disposition) return

    setBusy(true)
    setError('')
    try {
      const response = await batchSettleAdminUserRedemptions({
        ids: props.ids,
        disposition,
        reason: normalizedReason,
      })
      setResults(response)
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
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Batch settlement')}
      description={t(
        'Settle selected active, disabled, or pending redemption codes with one auditable reason.'
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
            {results.length > 0 ? t('Close') : t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={
              busy ||
              results.length > 0 ||
              props.ids.length === 0 ||
              normalizedReason.length === 0 ||
              normalizedReason.length > 500 ||
              !disposition
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
        <p className='text-muted-foreground text-sm'>
          {t('{{count}} eligible codes selected.', { count: props.ids.length })}
        </p>

        <div className='space-y-2'>
          <Label>{t('Settlement')}</Label>
          <div
            className='bg-muted grid grid-cols-2 rounded-md p-1'
            role='group'
            aria-label={t('Settlement')}
          >
            <Button
              type='button'
              variant={disposition === 'refund' ? 'secondary' : 'ghost'}
              aria-pressed={disposition === 'refund'}
              onClick={() => setDisposition('refund')}
            >
              {t('Refund remaining balance')}
            </Button>
            <Button
              type='button'
              variant={disposition === 'forfeit' ? 'secondary' : 'ghost'}
              aria-pressed={disposition === 'forfeit'}
              onClick={() => setDisposition('forfeit')}
            >
              {t('Forfeit remaining balance')}
            </Button>
          </div>
          <p className='text-muted-foreground text-xs'>
            {disposition === 'refund'
              ? t('The unclaimed balance will return to each creator wallet.')
              : t('No wallet refund is issued when balances are forfeited.')}
          </p>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='user-redemption-bulk-settlement-reason'>
            {t('Reason')}
          </Label>
          <Textarea
            id='user-redemption-bulk-settlement-reason'
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

        {results.length > 0 ? (
          <div className='space-y-2'>
            <h3 className='text-sm font-medium'>
              {t('Batch settlement results')}
            </h3>
            <ul className='divide-y rounded-md border text-sm'>
              {results.map((result) => (
                <li
                  key={result.id}
                  className='flex items-start justify-between gap-3 px-3 py-2'
                >
                  <span className='font-mono text-xs'>#{result.id}</span>
                  <span
                    className={
                      result.success
                        ? 'text-emerald-700 dark:text-emerald-300'
                        : 'text-destructive'
                    }
                  >
                    {result.success
                      ? t('Settled')
                      : getBatchSettlementError(result.code, t)}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}

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

function getBatchSettlementError(
  code: UserRedemptionBatchDispositionResult['code'],
  translate: (key: string) => string
): string {
  switch (code) {
    case 'not_found':
      return translate('Redemption code was not found')
    case 'inactive':
      return translate('Redemption code is not eligible for settlement')
    case 'conflict':
      return translate('Redemption code has already been settled differently')
    case 'invalid':
      return translate('Settlement request is invalid')
    default:
      return translate('Unable to settle this redemption code')
  }
}
