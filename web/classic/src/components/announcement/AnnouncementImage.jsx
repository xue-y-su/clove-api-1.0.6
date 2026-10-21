/*
Copyright (C) 2025 QuantumNous

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
import React, { useState } from 'react';
import { Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { isSafeAnnouncementImageURL } from '../layout/global-announcement-api';

const AnnouncementImage = ({ imageUrl, className = '' }) => {
  const { t } = useTranslation();
  const [previewVisible, setPreviewVisible] = useState(false);
  const safeImageURL =
    typeof imageUrl === 'string' && isSafeAnnouncementImageURL(imageUrl)
      ? imageUrl.trim()
      : '';

  if (!safeImageURL) return null;

  return (
    <>
      <button
        type='button'
        className={`w-full ${className}`.trim()}
        aria-label={t('查看公告图片')}
        onClick={() => setPreviewVisible(true)}
      >
        <img
          src={safeImageURL}
          alt={t('公告图片')}
          width={1200}
          height={675}
          className='aspect-video max-h-72 w-full rounded-md border border-semi-color-border bg-semi-color-fill-0 object-contain'
          loading='lazy'
          referrerPolicy='no-referrer'
        />
      </button>
      <Modal
        title={t('公告图片')}
        visible={previewVisible}
        footer={null}
        onCancel={() => setPreviewVisible(false)}
        width='min(960px, calc(100vw - 24px))'
      >
        <img
          src={safeImageURL}
          alt={t('公告图片')}
          width={1600}
          height={1200}
          className='max-h-[85vh] max-w-[95vw] object-contain'
          referrerPolicy='no-referrer'
        />
      </Modal>
    </>
  );
};

export default AnnouncementImage;
