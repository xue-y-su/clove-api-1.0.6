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
import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Button, Modal, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { API } from '../../helpers/api';
import { formatDateTimeString } from '../../helpers/utils';
import { sanitizeMarkdown } from '../../helpers/sanitize';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { normalizeAnnouncementContent } from '../announcement/content';
import {
  acknowledgeGlobalAnnouncement,
  getGlobalAnnouncementState,
  isSafeAnnouncementImageURL,
} from './global-announcement-api';

const { Text } = Typography;
const GlobalAnnouncementModal = ({ enabled = true }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [userState] = useContext(UserContext);
  const userId = userState?.user?.id;
  const [announcement, setAnnouncement] = useState(null);
  const [signature, setSignature] = useState('');
  const [visible, setVisible] = useState(false);
  const [acknowledging, setAcknowledging] = useState(false);
  const [acknowledgementError, setAcknowledgementError] = useState('');
  const [previewImageURL, setPreviewImageURL] = useState('');

  useEffect(() => {
    if (!enabled || !userId) {
      setAnnouncement(null);
      setSignature('');
      setVisible(false);
      return undefined;
    }
    let active = true;

    const loadPopup = async () => {
      try {
        const state = await getGlobalAnnouncementState(API);
        if (!active) return;

        if (
          !state.enabled ||
          !state.announcement ||
          !state.signature ||
          state.acknowledged
        ) {
          setAnnouncement(null);
          setSignature('');
          setVisible(false);
          return;
        }

        setAnnouncement(state.announcement);
        setSignature(state.signature);
        setAcknowledgementError('');
        setVisible(true);
      } catch {
        if (!active) return;
        setAnnouncement(null);
        setSignature('');
        setVisible(false);
      }
    };

    void loadPopup();
    return () => {
      active = false;
    };
  }, [enabled, userId]);

  const imageURL = useMemo(() => {
    const value = announcement?.imageUrl?.trim();
    return value && isSafeAnnouncementImageURL(value) ? value : '';
  }, [announcement]);

  const announcementContent = useMemo(() => {
    if (!announcement) return '';
    return sanitizeMarkdown(
      normalizeAnnouncementContent(announcement.content || ''),
    );
  }, [announcement]);

  const dismiss = async () => {
    if (!announcement || !signature || acknowledging) return;

    setAcknowledgementError('');
    if (!userId) return;

    setAcknowledging(true);
    try {
      await acknowledgeGlobalAnnouncement(API, signature);
      setVisible(false);
      setAnnouncement(null);
    } catch {
      setAcknowledgementError(t('公告确认失败，请重试'));
    } finally {
      setAcknowledging(false);
    }
  };

  if (!enabled || !userId || !announcement) return null;

  return (
    <Modal
      title={announcement.title || t('系统公告')}
      visible={visible}
      onCancel={() => void dismiss()}
      closable={false}
      maskClosable={false}
      keepDOM={false}
      width={isMobile ? 'calc(100vw - 24px)' : 600}
      footer={
        <Button
          type='primary'
          loading={acknowledging}
          disabled={acknowledging}
          onClick={() => void dismiss()}
        >
          {acknowledging ? t('正在确认公告') : t('知道了')}
        </Button>
      }
    >
      <div className='space-y-4'>
        {announcement.publishDate ? (
          <Text type='tertiary'>
            {t('发布时间')}:{' '}
            {formatDateTimeString(new Date(announcement.publishDate))}
          </Text>
        ) : null}
        {imageURL ? (
          <button
            type='button'
            className='w-full'
            aria-label={t('查看公告图片')}
            onClick={() => setPreviewImageURL(imageURL)}
          >
            <img
              src={imageURL}
              alt={t('公告图片')}
              width={1200}
              height={675}
              className='aspect-video max-h-[45vh] w-full rounded-md border border-semi-color-border object-contain bg-semi-color-fill-0'
              loading='eager'
              referrerPolicy='no-referrer'
            />
          </button>
        ) : null}
        <div
          className='break-words'
          dangerouslySetInnerHTML={{
            __html: announcementContent,
          }}
        />
        {announcement.extra ? (
          <div
            className='break-words text-semi-color-text-2'
            dangerouslySetInnerHTML={{
              __html: sanitizeMarkdown(
                normalizeAnnouncementContent(announcement.extra),
              ),
            }}
          />
        ) : null}
        {acknowledgementError ? (
          <Text type='danger' role='alert'>
            {acknowledgementError}
          </Text>
        ) : null}
        <Modal
          title={t('公告图片')}
          visible={Boolean(previewImageURL)}
          footer={null}
          onCancel={() => setPreviewImageURL('')}
          width={isMobile ? 'calc(100vw - 24px)' : 960}
        >
          {previewImageURL ? (
            <img
              src={previewImageURL}
              alt={t('公告图片')}
              width={1600}
              height={1200}
              className='max-h-[85vh] max-w-[95vw] object-contain'
              referrerPolicy='no-referrer'
            />
          ) : null}
        </Modal>
      </div>
    </Modal>
  );
};

export default GlobalAnnouncementModal;
