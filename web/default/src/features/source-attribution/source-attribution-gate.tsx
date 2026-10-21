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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { AuthenticatedGateResolution } from '@/features/auth/lib/authenticated-gates'

import { confirmSourceAttribution, getSourceAttributionStatus } from './api'

type SourceAttributionGateProps = {
  queryEnabled?: boolean
  displayEnabled?: boolean
  onResolutionChange?: (
    resolution: AuthenticatedGateResolution,
    error?: unknown
  ) => void
}

export function SourceAttributionGate(props: SourceAttributionGateProps) {
  const { queryEnabled, displayEnabled, onResolutionChange } = props
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [optionId, setOptionId] = useState('')
  const [sourceDescription, setSourceDescription] = useState('')
  const statusQuery = useQuery({
    queryKey: ['source-attribution-status'],
    queryFn: ({ signal }) => getSourceAttributionStatus(signal),
    enabled: queryEnabled !== false,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const confirmMutation = useMutation({
    mutationFn: confirmSourceAttribution,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to confirm source attribution'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['source-attribution-status'] })
      toast.success(t('Source attribution confirmed'))
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to confirm source attribution'))
    },
  })

  const status = statusQuery.data?.data
  const options = useMemo(
    () =>
      [...(status?.options ?? [])].sort((a, b) => {
        if (a.sort_order === b.sort_order) return a.name.localeCompare(b.name)
        return a.sort_order - b.sort_order
      }),
    [status?.options]
  )
  const selectedOption = options.find((option) => option.id === optionId)
  const open = Boolean(status?.enabled && !status.confirmed)
  const resolution: AuthenticatedGateResolution =
    queryEnabled === false || statusQuery.isPending
      ? 'loading'
      : statusQuery.isError || !status
        ? 'error'
        : open
          ? 'required'
          : 'satisfied'

  useEffect(() => {
    if (!open) {
      setOptionId('')
      setSourceDescription('')
    }
  }, [open])

  useEffect(() => {
    onResolutionChange?.(resolution, statusQuery.error)
  }, [onResolutionChange, resolution, statusQuery.error])

  const confirmDisabled =
    confirmMutation.isPending ||
    !optionId ||
    (selectedOption?.requires_description && !sourceDescription.trim())

  const handleConfirm = () => {
    if (!optionId) return
    confirmMutation.mutate({
      option_id: optionId,
      source_description: sourceDescription.trim() || undefined,
    })
  }

  return (
    <Dialog open={displayEnabled !== false && open}>
      <DialogContent showCloseButton={false} className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Source Attribution')}</DialogTitle>
          <DialogDescription>
            {t('Please confirm your invitation source first.')}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-4'>
          <div className='grid gap-2'>
            <Label htmlFor='source-attribution-option'>
              {t('Attribution option')}
            </Label>
            <Select
              items={options.map((option) => ({
                value: option.id,
                label: option.name,
              }))}
              value={optionId}
              onValueChange={(value) => setOptionId(value ?? '')}
            >
              <SelectTrigger id='source-attribution-option' className='w-full'>
                <SelectValue placeholder={t('Select an attribution option')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {options.map((option) => (
                    <SelectItem key={option.id} value={option.id}>
                      {option.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          {selectedOption?.requires_description && (
            <div className='grid gap-2'>
              <Label htmlFor='source-attribution-description'>
                {t('Source description')}
              </Label>
              <Textarea
                id='source-attribution-description'
                value={sourceDescription}
                onChange={(event) => setSourceDescription(event.target.value)}
                placeholder={t('Enter source description')}
                rows={3}
              />
            </div>
          )}
          {options.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t(
                'No attribution options are configured. Please contact an administrator.'
              )}
            </p>
          )}
        </div>
        <DialogFooter>
          <Button onClick={handleConfirm} disabled={confirmDisabled}>
            {confirmMutation.isPending ? t('Confirming...') : t('Confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
