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
import { FileTextIcon, ImageIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import {
  formatAttachmentSize,
  hasAttachmentPayload,
  type MessageAlignment,
} from '../../lib'
import type { PlaygroundAttachment } from '../../types'

type MessageAttachmentsProps = {
  alignment: MessageAlignment
  attachments?: PlaygroundAttachment[]
}

export function MessageAttachments({
  alignment,
  attachments,
}: MessageAttachmentsProps) {
  const { t } = useTranslation()

  if (!attachments?.length) {
    return null
  }

  return (
    <div
      className={cn(
        'mt-2 flex max-w-[min(34rem,100%)] flex-wrap gap-2',
        alignment === 'right' && 'justify-end'
      )}
    >
      {attachments.map((attachment) => {
        const isImage = attachment.kind === 'image'
        const Icon = isImage ? ImageIcon : FileTextIcon
        const hasPreview = isImage && Boolean(attachment.dataUrl)
        const hasPayload = hasAttachmentPayload(attachment)

        return (
          <div
            className='bg-background border-border/70 flex max-w-full items-center gap-2 rounded-md border px-2 py-1.5 text-xs shadow-xs'
            key={attachment.id}
          >
            <div className='bg-muted/50 flex size-9 shrink-0 items-center justify-center overflow-hidden rounded border'>
              {hasPreview ? (
                <img
                  alt={attachment.name}
                  className='size-full object-cover'
                  src={attachment.dataUrl}
                />
              ) : (
                <Icon className='text-muted-foreground size-4' />
              )}
            </div>
            <div className='min-w-0'>
              <div className='max-w-48 truncate font-medium'>
                {attachment.name}
              </div>
              <div className='text-muted-foreground truncate'>
                {hasPreview || hasPayload
                  ? formatAttachmentSize(attachment.size)
                  : t('{{size}} · saved without payload', {
                      size: formatAttachmentSize(attachment.size),
                    })}
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
