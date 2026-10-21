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
import { describe, expect, mock, test } from 'bun:test';
import {
  acknowledgeGlobalAnnouncement,
  getGlobalAnnouncementState,
  uploadAnnouncementMedia,
} from './global-announcement-api';

describe('classic announcement API', () => {
  test('loads popup state without invoking the global error toast', async () => {
    const get = mock(async () => ({
      data: {
        success: true,
        data: {
          enabled: true,
          announcement: { id: 3, content: 'Notice' },
          signature: 'a'.repeat(64),
          acknowledged: false,
        },
      },
    }));
    const apiClient = { get };

    await expect(getGlobalAnnouncementState(apiClient)).resolves.toMatchObject({
      signature: 'a'.repeat(64),
    });
    expect(get).toHaveBeenCalledWith('/api/announcement/popup-state', {
      skipErrorHandler: true,
    });
  });

  test('keeps acknowledgement failures available to the popup', async () => {
    const post = mock(async () => ({
      data: { success: false, message: 'confirmation unavailable' },
    }));
    const apiClient = { post };

    await expect(
      acknowledgeGlobalAnnouncement(apiClient, 'b'.repeat(64)),
    ).rejects.toThrow('confirmation unavailable');
    expect(post).toHaveBeenCalledWith(
      '/api/announcement/acknowledge',
      { signature: 'b'.repeat(64) },
      { skipErrorHandler: true },
    );
  });

  test('uploads announcement media as a multipart file', async () => {
    const post = mock(async () => ({
      data: {
        success: true,
        data: { url: `/api/announcement-media/${'c'.repeat(64)}.png` },
      },
    }));
    const apiClient = { post };
    const file = new File(['png'], 'notice.png', { type: 'image/png' });

    await expect(uploadAnnouncementMedia(apiClient, file)).resolves.toContain(
      '/api/announcement-media/',
    );
    const [, body] = post.mock.calls[0];
    expect(body).toBeInstanceOf(FormData);
    expect(body.get('file')).toMatchObject({
      name: 'notice.png',
      type: 'image/png',
    });
  });
});
