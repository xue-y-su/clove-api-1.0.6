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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { LoaderCircle } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import type { AnnouncementItem } from '@/features/dashboard/types'
import { formatDateTimeObject } from '@/lib/time'
import { useAuthStore } from '@/stores/auth-store'

import { AdaptiveAnnouncementImage } from './adaptive-announcement-image'
import { AnnouncementImagePreview } from './announcement-image-preview'
import {
  acknowledgeAnnouncement,
  getAnnouncementPopupState,
  type AnnouncementPopupState,
} from './api'
import { normalizeAnnouncementContent } from './content'
import { isSafeAnnouncementImageURL } from './global-announcement'

type GlobalAnnouncementDialogProps = {
  enabled?: boolean
}

export function GlobalAnnouncementDialog(props: GlobalAnnouncementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const popupQueryKey = ['announcement-popup-state', userId] as const
  const [announcement, setAnnouncement] = useState<AnnouncementItem | null>(
    null
  )
  const [signature, setSignature] = useState('')
  const [open, setOpen] = useState(false)
  const [acknowledging, setAcknowledging] = useState(false)
  const [acknowledgementError, setAcknowledgementError] = useState('')
  const [dismissedAnnouncementKey, setDismissedAnnouncementKey] = useState('')
  const [imagePreviewOpen, setImagePreviewOpen] = useState(false)
  const popupQuery = useQuery({
    queryKey: popupQueryKey,
    queryFn: getAnnouncementPopupState,
    enabled: props.enabled !== false && Boolean(userId),
    staleTime: 30_000,
    retry: 2,
  })

  useEffect(() => {
    const state = popupQuery.data
    if (
      !state?.enabled ||
      !state.announcement ||
      !state.signature ||
      state.acknowledged ||
      `${userId}:${state.signature}` === dismissedAnnouncementKey
    ) {
      setAnnouncement(null)
      setSignature('')
      setOpen(false)
      return
    }

    setAnnouncement(state.announcement)
    setSignature(state.signature)
    setAcknowledgementError('')
    setOpen(true)
  }, [dismissedAnnouncementKey, popupQuery.data, userId])

  useEffect(() => {
    if (props.enabled !== false && userId) return
    setAnnouncement(null)
    setSignature('')
    setOpen(false)
  }, [props.enabled, userId])

  const imageURL = useMemo(() => {
    const value = announcement?.imageUrl?.trim()
    return value && isSafeAnnouncementImageURL(value) ? value : null
  }, [announcement])

  const dismiss = async () => {
    if (!announcement || !signature || acknowledging) return

    setAcknowledgementError('')
    if (!userId) return
    const dismissalKey = `${userId}:${signature}`

    setAcknowledging(true)
    try {
      await acknowledgeAnnouncement(signature)
      queryClient.setQueryData<AnnouncementPopupState>(
        popupQueryKey,
        (current) =>
          current?.signature === signature
            ? { ...current, acknowledged: true }
            : current
      )
      setDismissedAnnouncementKey(dismissalKey)
      setOpen(false)
      void queryClient.invalidateQueries({
        queryKey: ['announcement-popup-state'],
      })
    } catch {
      setAcknowledgementError(
        t('Announcement confirmation failed. Please try again.')
      )
    } finally {
      setAcknowledging(false)
    }
  }

  if (props.enabled === false || !userId || !announcement) return null

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (nextOpen) setOpen(true)
      }}
      title={announcement.title?.trim() || t('System Announcement')}
      description={
        announcement.publishDate
          ? `${t('Published:')} ${formatDateTimeObject(new Date(announcement.publishDate))}`
          : undefined
      }
      contentClassName='sm:max-w-xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      showCloseButton={false}
      footer={
        <Button disabled={acknowledging} onClick={() => void dismiss()}>
          {acknowledging ? (
            <>
              <LoaderCircle className='h-4 w-4 animate-spin' />
              {t('Confirming announcement')}
            </>
          ) : (
            t('Got it')
          )}
        </Button>
      }
    >
      {imageURL ? (
        <button
          type='button'
          className='mx-auto flex w-fit max-w-full justify-center'
          aria-label={t('View announcement image')}
          onClick={() => setImagePreviewOpen(true)}
        >
          <AdaptiveAnnouncementImage
            src={imageURL}
            alt={t('Announcement image')}
            className='max-h-[min(52vh,480px)]'
            loading='eager'
            referrerPolicy='no-referrer'
          />
        </button>
      ) : null}
      <RichContent
        breaks
        content={normalizeAnnouncementContent(announcement.content)}
      />
      {announcement.extra ? (
        <RichContent
          breaks
          content={normalizeAnnouncementContent(announcement.extra)}
          className='text-muted-foreground'
        />
      ) : null}
      {acknowledgementError ? (
        <p className='text-destructive text-sm' role='alert'>
          {acknowledgementError}
        </p>
      ) : null}
      <AnnouncementImagePreview
        imageURL={imageURL}
        open={imagePreviewOpen}
        onOpenChange={setImagePreviewOpen}
      />
    </Dialog>
  )
}
