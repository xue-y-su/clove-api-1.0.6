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
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'

type AnnouncementImagePreviewProps = {
  imageURL: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function AnnouncementImagePreview(props: AnnouncementImagePreviewProps) {
  const { t } = useTranslation()

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Announcement image')}
      contentClassName='sm:max-w-5xl'
      contentHeight='auto'
      bodyClassName='flex items-center justify-center'
    >
      {props.imageURL ? (
        <img
          src={props.imageURL}
          alt={t('Announcement image')}
          width={1600}
          height={1200}
          className='max-h-[85vh] max-w-[95vw] object-contain'
          referrerPolicy='no-referrer'
        />
      ) : null}
    </Dialog>
  )
}
