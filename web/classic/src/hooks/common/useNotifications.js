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

import { useState, useEffect } from 'react';

export const getAnnouncementKey = (announcement) => {
  const payload = JSON.stringify({
    id: announcement?.id ?? null,
    title: announcement?.title ?? '',
    content: announcement?.content ?? '',
    extra: announcement?.extra ?? '',
    imageUrl: announcement?.imageUrl ?? '',
    publishDate: announcement?.publishDate ?? '',
    type: announcement?.type ?? 'default',
    popup: announcement?.popup === true,
  });
  let hash = 0x811c9dc5;
  for (let index = 0; index < payload.length; index += 1) {
    hash ^= payload.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return `v2:${announcement?.id ?? 'anonymous'}:${(hash >>> 0).toString(16).padStart(8, '0')}`;
};

export const useNotifications = (statusState) => {
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);

  const announcements = statusState?.status?.announcements || [];

  // Helper functions
  const calculateUnreadCount = () => {
    if (!announcements.length) return 0;
    let readKeys = [];
    try {
      readKeys = JSON.parse(localStorage.getItem('notice_read_keys')) || [];
    } catch (_) {
      readKeys = [];
    }
    const readSet = new Set(readKeys);
    return announcements.filter((a) => !readSet.has(getAnnouncementKey(a)))
      .length;
  };

  const getUnreadKeys = () => {
    if (!announcements.length) return [];
    let readKeys = [];
    try {
      readKeys = JSON.parse(localStorage.getItem('notice_read_keys')) || [];
    } catch (_) {
      readKeys = [];
    }
    const readSet = new Set(readKeys);
    return announcements
      .filter((a) => !readSet.has(getAnnouncementKey(a)))
      .map(getAnnouncementKey);
  };

  // Effects
  useEffect(() => {
    setUnreadCount(calculateUnreadCount());
  }, [announcements]);

  // Actions
  const handleNoticeOpen = () => {
    setNoticeVisible(true);
  };

  const handleNoticeClose = () => {
    setNoticeVisible(false);
    if (announcements.length) {
      let readKeys = [];
      try {
        readKeys = JSON.parse(localStorage.getItem('notice_read_keys')) || [];
      } catch (_) {
        readKeys = [];
      }
      const mergedKeys = Array.from(
        new Set([...readKeys, ...announcements.map(getAnnouncementKey)]),
      );
      localStorage.setItem('notice_read_keys', JSON.stringify(mergedKeys));
    }
    setUnreadCount(0);
  };

  return {
    noticeVisible,
    unreadCount,
    announcements,
    handleNoticeOpen,
    handleNoticeClose,
    getUnreadKeys,
  };
};
