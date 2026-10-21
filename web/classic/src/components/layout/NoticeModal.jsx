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

import React, { useEffect, useState, useContext, useMemo } from 'react';
import {
  Button,
  Modal,
  Empty,
  Tabs,
  TabPane,
  Timeline,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, getRelativeTime } from '../../helpers';
import {
  IllustrationNoContent,
  IllustrationNoContentDark,
} from '@douyinfe/semi-illustrations';
import { StatusContext } from '../../context/Status';
import { Bell, Megaphone } from 'lucide-react';
import { sanitizeMarkdown } from '../../helpers/sanitize';
import AnnouncementImage from '../announcement/AnnouncementImage';
import { normalizeAnnouncementContent } from '../announcement/content';
import { getAnnouncementKey } from '../../hooks/common/useNotifications';

const NoticeModal = ({
  visible,
  onClose,
  isMobile,
  defaultTab = 'inApp',
  unreadKeys = [],
}) => {
  const { t } = useTranslation();
  const [noticeContent, setNoticeContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState(defaultTab);

  const [statusState] = useContext(StatusContext);

  const announcements = statusState?.status?.announcements || [];

  const unreadSet = useMemo(() => new Set(unreadKeys), [unreadKeys]);

  const processedAnnouncements = useMemo(() => {
    return (announcements || []).slice(0, 20).map((item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      return {
        key: getAnnouncementKey(item),
        type: item.type || 'default',
        title: item.title,
        time: absoluteTime,
        content: item.content,
        extra: item.extra,
        imageUrl: item.imageUrl,
        relative: getRelativeTime(item.publishDate),
        isUnread: unreadSet.has(getAnnouncementKey(item)),
      };
    });
  }, [announcements, unreadSet]);

  const handleCloseTodayNotice = () => {
    const today = new Date().toDateString();
    localStorage.setItem('notice_close_date', today);
    onClose();
  };

  const displayNotice = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/notice');
      const { success, message, data } = res.data;
      if (success) {
        if (data !== '') {
          const htmlNotice = sanitizeMarkdown(data);
          setNoticeContent(htmlNotice);
        } else {
          setNoticeContent('');
        }
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      displayNotice();
    }
  }, [visible]);

  useEffect(() => {
    if (visible) {
      setActiveTab(defaultTab);
    }
  }, [defaultTab, visible]);

  const renderMarkdownNotice = () => {
    if (loading) {
      return (
        <div className='py-12'>
          <Empty description={t('加载中...')} />
        </div>
      );
    }

    if (!noticeContent) {
      return (
        <div className='py-12'>
          <Empty
            image={
              <IllustrationNoContent style={{ width: 150, height: 150 }} />
            }
            darkModeImage={
              <IllustrationNoContentDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无公告')}
          />
        </div>
      );
    }

    return (
      <div
        dangerouslySetInnerHTML={{ __html: noticeContent }}
        className='notice-content-scroll max-h-[55vh] overflow-y-auto pr-2'
      />
    );
  };

  const renderAnnouncementTimeline = () => {
    if (processedAnnouncements.length === 0) {
      return (
        <div className='py-12'>
          <Empty
            image={
              <IllustrationNoContent style={{ width: 150, height: 150 }} />
            }
            darkModeImage={
              <IllustrationNoContentDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无系统公告')}
          />
        </div>
      );
    }

    return (
      <div className='max-h-[55vh] overflow-y-auto pr-2 card-content-scroll'>
        <Timeline mode='left'>
          {processedAnnouncements.map((item, idx) => {
            const htmlContent = sanitizeMarkdown(
              normalizeAnnouncementContent(item.content || ''),
            );
            const htmlExtra = item.extra
              ? sanitizeMarkdown(normalizeAnnouncementContent(item.extra))
              : '';
            return (
              <Timeline.Item
                key={idx}
                type={item.type}
                time={`${item.relative ? item.relative + ' ' : ''}${item.time}`}
                extra={
                  item.extra ? (
                    <div
                      className='text-xs text-gray-500'
                      dangerouslySetInnerHTML={{ __html: htmlExtra }}
                    />
                  ) : null
                }
                className={item.isUnread ? '' : ''}
              >
                <div>
                  {item.title ? (
                    <div className='mb-2 break-words font-semibold'>
                      {item.title}
                    </div>
                  ) : null}
                  <div
                    className={item.isUnread ? 'shine-text' : ''}
                    dangerouslySetInnerHTML={{ __html: htmlContent }}
                  />
                  <AnnouncementImage
                    imageUrl={item.imageUrl}
                    className='mt-3'
                  />
                </div>
              </Timeline.Item>
            );
          })}
        </Timeline>
      </div>
    );
  };

  return (
    <Modal
      title={
        <div className='flex items-center justify-between w-full'>
          <span>{t('系统公告')}</span>
          <Tabs activeKey={activeTab} onChange={setActiveTab} type='button'>
            <TabPane
              tab={
                <span className='flex items-center gap-1'>
                  <Bell size={14} /> {t('通知')}
                </span>
              }
              itemKey='inApp'
            />
            <TabPane
              tab={
                <span className='flex items-center gap-1'>
                  <Megaphone size={14} /> {t('系统公告')}
                </span>
              }
              itemKey='system'
            />
          </Tabs>
        </div>
      }
      visible={visible}
      onCancel={onClose}
      footer={
        <div className='flex justify-end'>
          <Button type='secondary' onClick={handleCloseTodayNotice}>
            {t('今日关闭')}
          </Button>
          <Button type='primary' onClick={onClose}>
            {t('关闭公告')}
          </Button>
        </div>
      }
      size={isMobile ? 'full-width' : 'large'}
    >
      <div className='min-h-[55vh]'>
        <section
          hidden={activeTab !== 'inApp'}
          aria-hidden={activeTab !== 'inApp'}
        >
          {renderMarkdownNotice()}
        </section>
        <section
          hidden={activeTab !== 'system'}
          aria-hidden={activeTab !== 'system'}
        >
          {renderAnnouncementTimeline()}
        </section>
      </div>
    </Modal>
  );
};

export default NoticeModal;
