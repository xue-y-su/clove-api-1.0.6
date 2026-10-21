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
import { expect, test } from 'bun:test';
import { getAnnouncementKey } from './useNotifications';

test('announcement unread keys change when the title or image changes', () => {
  const announcement = {
    id: 7,
    title: 'Release',
    content: 'Important update',
    imageUrl: '/api/announcement-media/old.png',
    publishDate: '2026-07-20T00:00:00Z',
  };
  const original = getAnnouncementKey(announcement);

  expect(getAnnouncementKey({ ...announcement, title: 'Revised' })).not.toBe(
    original,
  );
  expect(
    getAnnouncementKey({
      ...announcement,
      imageUrl: '/api/announcement-media/new.png',
    }),
  ).not.toBe(original);
});
