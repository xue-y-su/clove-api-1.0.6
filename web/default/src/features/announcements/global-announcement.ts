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
import type { AnnouncementItem } from '@/features/dashboard/types'

const DISMISSAL_STORAGE_PREFIX = 'cloveapi:announcement-dismissals'
const MANAGED_ANNOUNCEMENT_IMAGE_PATTERN =
  /^\/api\/announcement-media\/[a-f0-9]{64}\.(?:jpg|png|webp|gif)$/

export function getAnnouncementSignature(
  announcement: AnnouncementItem
): string {
  const payload = JSON.stringify({
    id: announcement.id ?? null,
    title: announcement.title ?? '',
    content: announcement.content,
    extra: announcement.extra ?? '',
    imageUrl: announcement.imageUrl ?? '',
    publishDate: announcement.publishDate ?? '',
    type: announcement.type ?? 'default',
    popup: announcement.popup === true,
    published: announcement.published !== false,
  })
  let hash = 0x811c9dc5
  for (let index = 0; index < payload.length; index += 1) {
    hash ^= payload.charCodeAt(index)
    hash = Math.imul(hash, 0x01000193)
  }

  return `v1:${announcement.id ?? 'anonymous'}:${payload.length}:${(hash >>> 0).toString(16).padStart(8, '0')}`
}

export function getAnnouncementDismissalStorageKey(userId?: number): string {
  return `${DISMISSAL_STORAGE_PREFIX}:${userId ?? 'guest'}`
}

export function isSafeAnnouncementImageURL(imageURL: string): boolean {
  if (isManagedAnnouncementImageURL(imageURL)) return true

  try {
    const parsed = new URL(imageURL)
    const hostname = parsed.hostname.toLowerCase().replace(/\.$/, '')
    const isIPAddress = /^[\d.]+$/.test(hostname) || hostname.includes(':')

    return (
      parsed.protocol === 'https:' &&
      parsed.username === '' &&
      parsed.password === '' &&
      hostname.includes('.') &&
      hostname !== 'localhost' &&
      !hostname.endsWith('.localhost') &&
      !isIPAddress
    )
  } catch {
    return false
  }
}

export function isManagedAnnouncementImageURL(imageURL: string): boolean {
  return MANAGED_ANNOUNCEMENT_IMAGE_PATTERN.test(imageURL.trim())
}

export function selectGlobalAnnouncement(
  announcements: AnnouncementItem[],
  dismissed: ReadonlySet<string>,
  now = new Date()
): AnnouncementItem | null {
  const nowTimestamp = now.getTime()

  return (
    announcements
      .map((announcement, index) => {
        const publishTimestamp = announcement.publishDate
          ? new Date(announcement.publishDate).getTime()
          : 0
        return { announcement, index, publishTimestamp }
      })
      .filter(
        ({ announcement, publishTimestamp }) =>
          announcement.published !== false &&
          Number.isFinite(publishTimestamp) &&
          publishTimestamp <= nowTimestamp &&
          !dismissed.has(getAnnouncementSignature(announcement))
      )
      .sort(
        (left, right) =>
          right.publishTimestamp - left.publishTimestamp ||
          right.index - left.index
      )[0]?.announcement ?? null
  )
}
