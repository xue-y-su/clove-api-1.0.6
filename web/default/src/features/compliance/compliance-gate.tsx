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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Markdown } from '@/components/ui/markdown'
import type { AuthenticatedGateResolution } from '@/features/auth/lib/authenticated-gates'

import {
  confirmComplianceStatement,
  getComplianceStatement,
  type ComplianceConfirmRequest,
  type ComplianceConfirmationMode,
  type ComplianceSnapshot,
  type ComplianceStatement,
} from './api'

type DisplaySnapshot = ComplianceSnapshot & {
  legacy?: boolean
}

function getPendingSnapshots(
  statement?: ComplianceStatement
): DisplaySnapshot[] {
  if (!statement?.enabled || statement.confirmed) return []
  const snapshots = (statement.snapshots ?? [])
    .filter((snapshot) => !snapshot.confirmed)
    .map((snapshot) => ({ ...snapshot, legacy: false }))
    .sort((a, b) => {
      if (a.sort_order === b.sort_order) return a.id - b.id
      return a.sort_order - b.sort_order
    })

  if (snapshots.length > 0) return snapshots
  if (!statement.content) return []

  return [
    {
      id: 0,
      item_key: 'statement',
      title: 'Compliance Statement',
      source_type: 'manual',
      version: statement.version,
      content: statement.content,
      content_hash: statement.content_hash,
      content_length: statement.content.length,
      sort_order: 1,
      confirmation_mode: 'checkbox',
      confirmed: false,
      legacy: true,
    },
  ]
}

function normalizeConfirmationMode(
  mode: string | undefined
): ComplianceConfirmationMode {
  if (mode === 'phrase' || mode === 'segments') return mode
  return 'checkbox'
}

type ComplianceGateProps = {
  queryEnabled?: boolean
  displayEnabled?: boolean
  onResolutionChange?: (
    resolution: AuthenticatedGateResolution,
    error?: unknown
  ) => void
}

export function ComplianceGate(props: ComplianceGateProps) {
  const { queryEnabled, displayEnabled, onResolutionChange } = props
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [acknowledged, setAcknowledged] = useState(false)
  const [confirmationText, setConfirmationText] = useState('')
  const [segmentTexts, setSegmentTexts] = useState<string[]>([])
  const statementQuery = useQuery({
    queryKey: ['compliance-statement'],
    queryFn: ({ signal }) => getComplianceStatement(signal),
    enabled: queryEnabled !== false,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const confirmMutation = useMutation({
    mutationFn: confirmComplianceStatement,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to confirm compliance'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['compliance-statement'] })
      setAcknowledged(false)
      setConfirmationText('')
      setSegmentTexts([])
      toast.success(t('Compliance statement confirmed'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to confirm compliance'))
    },
  })

  const statement = statementQuery.data?.data
  const pendingSnapshots = useMemo(
    () => getPendingSnapshots(statement),
    [statement]
  )
  const activeSnapshot = pendingSnapshots[0]
  const activeMode = normalizeConfirmationMode(
    activeSnapshot?.confirmation_mode
  )
  const activeSegmentCount = activeSnapshot?.segments?.length ?? 0
  const visibleSnapshots =
    statement?.display_mode === 'single'
      ? pendingSnapshots
      : activeSnapshot
        ? [activeSnapshot]
        : []
  const open = pendingSnapshots.length > 0
  const resolution: AuthenticatedGateResolution =
    queryEnabled === false || statementQuery.isPending
      ? 'loading'
      : statementQuery.isError || !statement
        ? 'error'
        : open
          ? 'required'
          : 'satisfied'
  const formatSnapshotTitle = (snapshot: DisplaySnapshot) =>
    snapshot.title === 'Compliance Statement'
      ? t('Compliance Statement')
      : snapshot.title || t('Compliance Statement')

  useEffect(() => {
    setAcknowledged(false)
    setConfirmationText('')
    setSegmentTexts(Array.from({ length: activeSegmentCount }, () => ''))
  }, [
    activeSnapshot?.legacy,
    activeSnapshot?.id,
    activeSnapshot?.item_key,
    activeSnapshot?.content,
    activeSnapshot?.confirmation_mode,
    activeSegmentCount,
  ])

  useEffect(() => {
    onResolutionChange?.(resolution, statementQuery.error)
  }, [onResolutionChange, resolution, statementQuery.error])

  const confirmDisabled =
    confirmMutation.isPending ||
    !activeSnapshot ||
    (activeMode === 'checkbox' && !acknowledged) ||
    (activeMode === 'phrase' && !confirmationText.trim()) ||
    (activeMode === 'segments' &&
      (segmentTexts.length !== (activeSnapshot.segments?.length ?? 0) ||
        segmentTexts.some((text) => !text.trim())))

  const handleConfirm = () => {
    if (!activeSnapshot) return
    const payload: ComplianceConfirmRequest = {}
    if (!activeSnapshot.legacy && activeSnapshot.id > 0) {
      payload.snapshot_id = activeSnapshot.id
    }
    if (activeMode === 'phrase') {
      payload.confirmation_text = confirmationText.trim()
    }
    if (activeMode === 'segments') {
      payload.segments = segmentTexts.map((text) => text.trim())
    }
    confirmMutation.mutate(payload)
  }

  const updateSegmentText = (index: number, value: string) => {
    setSegmentTexts((current) => {
      const next = [...current]
      next[index] = value
      return next
    })
  }

  return (
    <Dialog open={displayEnabled !== false && open}>
      <DialogContent showCloseButton={false} className='sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Compliance Statement')}</DialogTitle>
          <DialogDescription>
            {t('Please read and confirm the current compliance statement.')}
          </DialogDescription>
        </DialogHeader>
        <div className='max-h-[58vh] space-y-4 overflow-auto pr-1'>
          {visibleSnapshots.map((snapshot, index) => (
            <div
              key={
                snapshot.legacy
                  ? `legacy-${snapshot.item_key}`
                  : `snapshot-${snapshot.id}`
              }
              className='border-border bg-muted/30 rounded-md border p-3'
            >
              <div className='mb-2 flex flex-wrap items-center gap-x-3 gap-y-1'>
                <h3 className='text-sm font-medium'>
                  {statement?.display_mode === 'single'
                    ? `${index + 1}. ${formatSnapshotTitle(snapshot)}`
                    : formatSnapshotTitle(snapshot)}
                </h3>
                {snapshot.version ? (
                  <span className='text-muted-foreground text-xs'>
                    {t('Version number')}: {snapshot.version}
                  </span>
                ) : null}
              </div>
              <Markdown>{snapshot.content || ''}</Markdown>
            </div>
          ))}
        </div>
        {activeSnapshot && (
          <div className='space-y-3'>
            <div className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs'>
              <span>
                {t('Current term')}: {formatSnapshotTitle(activeSnapshot)}
              </span>
              {activeSnapshot.content_hash ? (
                <span className='break-all'>
                  {t('Content hash')}: {activeSnapshot.content_hash}
                </span>
              ) : null}
              <span>
                {pendingSnapshots.length > 1
                  ? t('Pending {{current}} / {{total}}', {
                      current: 1,
                      total: pendingSnapshots.length,
                    })
                  : t('Pending 1 item')}
              </span>
            </div>

            {activeMode === 'checkbox' && (
              <Label className='flex items-center gap-2 text-sm'>
                <Checkbox
                  checked={acknowledged}
                  onCheckedChange={(checked) => setAcknowledged(!!checked)}
                />
                {t('I have read and agree to the current terms')}
              </Label>
            )}

            {activeMode === 'phrase' && (
              <div className='grid gap-2'>
                <Label htmlFor='compliance-confirmation-phrase'>
                  {t('Enter confirmation phrase')}
                </Label>
                <Input
                  id='compliance-confirmation-phrase'
                  value={confirmationText}
                  onChange={(event) => setConfirmationText(event.target.value)}
                  placeholder={activeSnapshot.confirmation_phrase || ''}
                />
              </div>
            )}

            {activeMode === 'segments' && (
              <div className='space-y-3'>
                {(activeSnapshot.segments ?? []).map((segment, index) => (
                  <div
                    key={`${activeSnapshot.id}-segment-${index}`}
                    className='grid gap-2'
                  >
                    <Label htmlFor={`compliance-segment-${index}`}>
                      {t('Segment {{number}}', { number: index + 1 })}
                    </Label>
                    <div className='text-muted-foreground rounded-md border p-2 text-xs'>
                      {segment}
                    </div>
                    <Input
                      id={`compliance-segment-${index}`}
                      value={segmentTexts[index] ?? ''}
                      onChange={(event) =>
                        updateSegmentText(index, event.target.value)
                      }
                      placeholder={t('Enter or paste this segment')}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
        <DialogFooter>
          <Button onClick={handleConfirm} disabled={confirmDisabled}>
            {confirmMutation.isPending
              ? t('Confirming...')
              : t('Confirm current term')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
