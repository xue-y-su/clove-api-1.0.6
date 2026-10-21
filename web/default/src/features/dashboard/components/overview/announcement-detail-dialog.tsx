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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { ScrollArea } from '@/components/ui/scroll-area'
import { AdaptiveAnnouncementImage } from '@/features/announcements/adaptive-announcement-image'
import { AnnouncementImagePreview } from '@/features/announcements/announcement-image-preview'
import { normalizeAnnouncementContent } from '@/features/announcements/content'
import { isSafeAnnouncementImageURL } from '@/features/announcements/global-announcement'
import { formatDateTimeObject } from '@/lib/time'

interface AnnouncementDetailModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  announcement: {
    title?: string
    content?: string
    tag?: string
    publishDate?: string
    extra?: string
    imageUrl?: string
  } | null
}

export function AnnouncementDetailModal({
  open,
  onOpenChange,
  announcement,
}: AnnouncementDetailModalProps) {
  const { t } = useTranslation()
  const [imagePreviewOpen, setImagePreviewOpen] = useState(false)
  const imageURL =
    announcement?.imageUrl && isSafeAnnouncementImageURL(announcement.imageUrl)
      ? announcement.imageUrl
      : null
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={announcement?.title || t('Announcement Details')}
      description={
        announcement?.publishDate
          ? `${t('Published:')} ${formatDateTimeObject(new Date(announcement.publishDate))}`
          : undefined
      }
      contentClassName='sm:max-w-lg'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      <ScrollArea className='max-h-[min(58vh,520px)] pr-4'>
        <div className='space-y-4'>
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
                className='max-h-[min(46vh,400px)]'
                loading='lazy'
                referrerPolicy='no-referrer'
              />
            </button>
          ) : null}
          {announcement?.content && (
            <div>
              <h4 className='mb-2 font-medium'>{t('Content')}</h4>
              <RichContent
                breaks
                content={normalizeAnnouncementContent(announcement.content)}
              />
            </div>
          )}
          {announcement?.extra && (
            <div>
              <h4 className='mb-2 font-medium'>
                {t('Additional Information')}
              </h4>
              <RichContent
                breaks
                content={normalizeAnnouncementContent(announcement.extra)}
                className='text-muted-foreground'
              />
            </div>
          )}
        </div>
      </ScrollArea>
      <AnnouncementImagePreview
        imageURL={imageURL}
        open={imagePreviewOpen}
        onOpenChange={setImagePreviewOpen}
      />
    </Dialog>
  )
}
