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
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

import type { AnnouncementItem } from '@/features/dashboard/types'

import {
  getAnnouncementDismissalStorageKey,
  getAnnouncementSignature,
  isSafeAnnouncementImageURL,
  selectGlobalAnnouncement,
} from './global-announcement'

describe('global announcement selection', () => {
  const announcements: AnnouncementItem[] = [
    {
      id: 1,
      content: 'Old',
      publishDate: '2026-07-18T00:00:00.000Z',
    },
    {
      id: 2,
      content: 'Latest',
      imageUrl: 'https://img.cdn1.vip/i/latest.png',
      publishDate: '2026-07-19T00:00:00.000Z',
    },
    {
      id: 3,
      content: 'Future',
      publishDate: '2026-07-21T00:00:00.000Z',
    },
  ]

  test('selects the latest published unread announcement', () => {
    const selected = selectGlobalAnnouncement(
      announcements,
      new Set(),
      new Date('2026-07-20T00:00:00.000Z')
    )
    expect(selected?.id).toBe(2)
  })

  test('does not reopen a dismissed announcement', () => {
    const dismissed = new Set([getAnnouncementSignature(announcements[1])])
    expect(
      selectGlobalAnnouncement(
        announcements,
        dismissed,
        new Date('2026-07-20T00:00:00.000Z')
      )?.id
    ).toBe(1)
  })

  test('treats an image update as a new announcement version', () => {
    const original = getAnnouncementSignature(announcements[1])
    const updated = getAnnouncementSignature({
      ...announcements[1],
      imageUrl: 'https://img.cdn1.vip/i/revised.png',
    })
    expect(updated).not.toBe(original)
  })

  test('treats title and publishing updates as a new announcement version', () => {
    const original = getAnnouncementSignature(announcements[1])
    expect(
      getAnnouncementSignature({ ...announcements[1], title: 'Release' })
    ).not.toBe(original)
    expect(
      getAnnouncementSignature({ ...announcements[1], published: false })
    ).not.toBe(original)
  })

  test('does not select explicitly unpublished announcements', () => {
    expect(
      selectGlobalAnnouncement(
        [{ ...announcements[1], published: false }],
        new Set(),
        new Date('2026-07-20T00:00:00.000Z')
      )
    ).toBeNull()
  })

  test('does not persist announcement content in the dismissal signature', () => {
    const signature = getAnnouncementSignature(announcements[1])
    expect(signature).not.toContain('Latest')
    expect(signature).not.toContain('img.cdn1.vip')
  })

  test('accepts only public HTTPS announcement images', () => {
    expect(
      isSafeAnnouncementImageURL('https://img.cdn1.vip/i/notice.png')
    ).toBe(true)
    expect(isSafeAnnouncementImageURL('http://example.com/notice.png')).toBe(
      false
    )
    expect(isSafeAnnouncementImageURL('https://localhost/notice.png')).toBe(
      false
    )
    expect(isSafeAnnouncementImageURL('https://127.0.0.1/notice.png')).toBe(
      false
    )
    expect(
      isSafeAnnouncementImageURL(
        `/api/announcement-media/${'a'.repeat(64)}.png`
      )
    ).toBe(true)
    expect(isSafeAnnouncementImageURL('/uploads/unsafe.png')).toBe(false)
  })

  test('isolates dismissal state by signed-in user', () => {
    expect(getAnnouncementDismissalStorageKey(1)).not.toBe(
      getAnnouncementDismissalStorageKey(2)
    )
    expect(getAnnouncementDismissalStorageKey()).not.toBe(
      getAnnouncementDismissalStorageKey(1)
    )
  })

  test('uses the server state and acknowledgement APIs for the popup', () => {
    const dialogSource = readFileSync(
      new URL('./global-announcement-dialog.tsx', import.meta.url),
      'utf8'
    )

    expect(dialogSource).not.toContain('useAnnouncements')
    expect(dialogSource).toContain('getAnnouncementPopupState')
    expect(dialogSource).toContain('acknowledgeAnnouncement')
    expect(dialogSource).toContain('setOpen(false)')
    expect(dialogSource).toContain('acknowledgementError')
    expect(dialogSource).toContain(
      'enabled: props.enabled !== false && Boolean(userId)'
    )
    expect(dialogSource).toContain('queryClient.setQueryData')
    expect(dialogSource).toContain('queryClient.invalidateQueries')
    expect(dialogSource).toContain('if (nextOpen) setOpen(true)')
    expect(dialogSource).not.toContain('else void dismiss()')
    expect(dialogSource).not.toContain('localStorage')
    expect(dialogSource).not.toContain('content_format')
    expect(dialogSource).not.toContain('mode={')
  })

  test('renders every announcement through the unified markdown sanitizer', () => {
    for (const sourceURL of [
      new URL('./global-announcement-dialog.tsx', import.meta.url),
      new URL('../../components/notification-popover.tsx', import.meta.url),
      new URL(
        '../dashboard/components/overview/announcement-detail-dialog.tsx',
        import.meta.url
      ),
    ]) {
      const source = readFileSync(sourceURL, 'utf8')
      expect(source).not.toContain('content_format')
      expect(source).not.toContain('mode={')
      expect(source).toContain('RichContent')
    }
  })

  test('mounts the popup only inside the authenticated gate stack', () => {
    const rootSource = readFileSync(
      new URL('../../routes/__root.tsx', import.meta.url),
      'utf8'
    )
    const layoutSource = readFileSync(
      new URL(
        '../../components/layout/components/authenticated-layout.tsx',
        import.meta.url
      ),
      'utf8'
    )

    expect(rootSource).not.toContain('GlobalAnnouncementDialog')
    expect(layoutSource).toContain('GlobalAnnouncementDialog')
    expect(layoutSource).toContain('getAuthenticatedGateAccess')
  })

  test('adapts announcement images without forcing a landscape frame', () => {
    for (const sourceURL of [
      new URL('./global-announcement-dialog.tsx', import.meta.url),
      new URL('../../components/notification-popover.tsx', import.meta.url),
      new URL(
        '../dashboard/components/overview/announcement-detail-dialog.tsx',
        import.meta.url
      ),
    ]) {
      const source = readFileSync(sourceURL, 'utf8')
      expect(source).toContain('AdaptiveAnnouncementImage')
      expect(source).toContain(
        "className='mx-auto flex w-fit max-w-full justify-center'"
      )
      expect(source).not.toContain('aspect-video')
      expect(source).not.toContain('bg-muted/30')
    }

    const adaptiveImageSource = readFileSync(
      new URL('./adaptive-announcement-image.tsx', import.meta.url),
      'utf8'
    )
    expect(adaptiveImageSource).toContain('h-auto')
    expect(adaptiveImageSource).toContain('w-auto')
    expect(adaptiveImageSource).toContain('max-w-full')
    expect(adaptiveImageSource).toContain('object-contain')
    expect(adaptiveImageSource).not.toContain('object-cover')
  })

  test('lets users open announcement images in an accessible preview', () => {
    for (const sourceURL of [
      new URL('./global-announcement-dialog.tsx', import.meta.url),
      new URL('../../components/notification-popover.tsx', import.meta.url),
      new URL(
        '../dashboard/components/overview/announcement-detail-dialog.tsx',
        import.meta.url
      ),
    ]) {
      const source = readFileSync(sourceURL, 'utf8')
      expect(source).toContain('AnnouncementImagePreview')
      expect(source).toContain("t('View announcement image')")
    }

    const previewSource = readFileSync(
      new URL('./announcement-image-preview.tsx', import.meta.url),
      'utf8'
    )
    expect(previewSource).toContain('max-h-[85vh] max-w-[95vw] object-contain')
  })
})
