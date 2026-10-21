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
import { afterEach, describe, expect, mock, test } from 'bun:test'

import { api } from '@/lib/api'

import {
  acknowledgeAnnouncement,
  getAnnouncementPopupState,
  uploadAnnouncementMedia,
} from './api'

const originalGet = api.get
const originalPost = api.post

afterEach(() => {
  api.get = originalGet
  api.post = originalPost
})

describe('announcement API', () => {
  test('loads the server-selected popup state without a global error toast', async () => {
    const get = mock(async () => ({
      data: {
        success: true,
        data: {
          enabled: true,
          announcement: { id: 7, content: 'Notice' },
          signature: 'a'.repeat(64),
          acknowledged: false,
        },
      },
    }))
    api.get = get as typeof api.get

    await expect(getAnnouncementPopupState()).resolves.toMatchObject({
      signature: 'a'.repeat(64),
    })
    expect(get).toHaveBeenCalledWith('/api/announcement/popup-state', {
      skipBusinessError: true,
      skipErrorHandler: true,
    })
  })

  test('posts an acknowledgement and exposes business failures to the dialog', async () => {
    const post = mock(async () => ({
      data: { success: false, message: 'confirmation unavailable' },
    }))
    api.post = post as typeof api.post

    await expect(acknowledgeAnnouncement('b'.repeat(64))).rejects.toThrow(
      'confirmation unavailable'
    )
    expect(post).toHaveBeenCalledWith(
      '/api/announcement/acknowledge',
      { signature: 'b'.repeat(64) },
      { skipBusinessError: true, skipErrorHandler: true }
    )
  })

  test('uploads a file as multipart data and returns only the managed URL', async () => {
    const post = mock(async () => ({
      data: {
        success: true,
        data: { url: `/api/announcement-media/${'c'.repeat(64)}.png` },
      },
    }))
    api.post = post as typeof api.post
    const file = new File(['png'], 'notice.png', { type: 'image/png' })

    await expect(uploadAnnouncementMedia(file)).resolves.toBe(
      `/api/announcement-media/${'c'.repeat(64)}.png`
    )
    const [, body, config] = post.mock.calls[0]
    expect(body).toBeInstanceOf(FormData)
    expect((body as FormData).get('file')).toMatchObject({
      name: 'notice.png',
      type: 'image/png',
    })
    expect(config).toEqual({ skipBusinessError: true, skipErrorHandler: true })
  })
})
