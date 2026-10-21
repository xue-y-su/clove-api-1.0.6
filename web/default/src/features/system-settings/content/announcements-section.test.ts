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
import { expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'

import { formatAnnouncementRelativeTime } from './announcement-relative-time'

test('formats announcement age with the active interface locale', () => {
  const now = new Date('2026-07-18T00:00:00.000Z')
  expect(
    formatAnnouncementRelativeTime('2026-07-08T00:00:00.000Z', 'zhCN', now)
  ).toBe('10天前')
  expect(
    formatAnnouncementRelativeTime('2026-07-08T00:00:00.000Z', 'en', now)
  ).toBe('10 days ago')
})

test('translates audited option labels before rendering them', () => {
  const announcementsSource = readFileSync(
    new URL('./announcements-section.tsx', import.meta.url),
    'utf8'
  )
  const apiInfoSource = readFileSync(
    new URL('./api-info-section.tsx', import.meta.url),
    'utf8'
  )
  const modelRatioSource = readFileSync(
    new URL('../models/model-ratio-visual-editor.tsx', import.meta.url),
    'utf8'
  )
  const playgroundSource = readFileSync(
    new URL('../site/playground-section.tsx', import.meta.url),
    'utf8'
  )
  const tieredPricingSource = readFileSync(
    new URL('../models/tiered-pricing-editor.tsx', import.meta.url),
    'utf8'
  )

  expect(announcementsSource).toContain('t(option.label)')
  expect(announcementsSource).toContain('ImagePlus')
  expect(announcementsSource).toContain('uploadAnnouncementMedia')
  expect(announcementsSource).toContain('aspect-square')
  expect(announcementsSource).toContain('announcementSettingsQuery.isError')
  expect(announcementsSource).toContain('announcementSettingsQuery.refetch()')
  expect(announcementsSource).toContain('width={112}')
  expect(announcementsSource).toContain('height={112}')
  expect(announcementsSource).not.toContain("type='url'")
  expect(announcementsSource).not.toContain(
    "t('Announcement image URL (optional)')"
  )
  expect(announcementsSource).toContain("t('Content is required')")
  expect(announcementsSource).not.toContain(".min(1, 'Content is required')")
  expect(announcementsSource).not.toContain('.max(500')
  expect(announcementsSource).toContain('new TextEncoder().encode(value)')
  expect(announcementsSource).toContain('64 * 1024')
  expect(announcementsSource).not.toContain('content_format')
  expect(announcementsSource).not.toContain('contentFormat')
  expect(announcementsSource).not.toContain('popup_enabled')
  expect(announcementsSource).toContain(
    "t('Show this announcement as a popup')"
  )
  expect(announcementsSource).toContain("name='title'")
  expect(announcementsSource).toContain("name='published'")
  expect(announcementsSource).toContain("t('Publish this announcement')")
  expect(announcementsSource).not.toContain(
    "label={t('Show unread announcements as a popup')}"
  )
  expect(announcementsSource).not.toContain("<SelectItem value='html'>")
  expect(announcementsSource).not.toContain('`${diffDays}d ago`')
  expect(apiInfoSource).toContain('t(option.label)')
  expect(modelRatioSource).toContain("label: t('Per-token')")
  expect(modelRatioSource).toContain("label: t('Per-request')")
  expect(modelRatioSource).toContain("label: t('Expression')")
  expect(playgroundSource).toContain(
    "saveLabel={t('Save playground settings')}"
  )
  expect(playgroundSource).toContain("label: t('Basic')")
  expect(playgroundSource).toContain("label: t('Advanced')")
  expect(tieredPricingSource).toContain('{t(preset.label)}')
})

test('defines translated global announcement text in every locale', () => {
  const keys = [
    'System Announcement',
    'Announcement image',
    'Add announcement image',
    'Replace announcement image',
    'Remove announcement image',
    'Upload announcement image failed',
    'Announcement image must be PNG, JPEG, GIF, or WebP',
    'Confirming announcement',
    'Announcement confirmation failed. Please try again.',
    'Failed to load announcement settings',
    'Failed to update announcement settings',
    'Got it',
    'Content is required',
    'Announcement content must not exceed 64 KiB',
    'Supports Markdown and HTML. Each announcement is limited to 64 KiB.',
    'Publish date is required',
    'Announcement title',
    'Publish this announcement',
    'Unpublished announcements remain visible only to administrators.',
    'Extra must be less than 100 characters',
    'Image URL must be less than 2000 characters',
    'Image URL must be a public HTTPS URL',
  ]

  for (const localeFile of [
    'en.json',
    'fr.json',
    'ja.json',
    'ru.json',
    'vi.json',
    'zh-TW.json',
    'zh.json',
  ]) {
    const locale = JSON.parse(
      readFileSync(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url),
        'utf8'
      )
    ) as { translation: Record<string, string> }

    for (const key of keys) {
      expect(Object.hasOwn(locale.translation, key)).toBe(true)
      if (localeFile !== 'en.json') {
        expect(locale.translation[key]).not.toBe(key)
      }
    }
  }
})

test('shows announcement titles in every public announcement surface', () => {
  const surfaces = [
    [
      '../../announcements/global-announcement-dialog.tsx',
      'announcement.title',
    ],
    ['../../../components/notification-popover.tsx', 'item.title'],
    [
      '../../dashboard/components/overview/announcements-panel.tsx',
      'item.title',
    ],
    [
      '../../dashboard/components/overview/announcement-detail-dialog.tsx',
      'announcement?.title',
    ],
  ]
  for (const [path, token] of surfaces) {
    expect(readFileSync(new URL(path, import.meta.url), 'utf8')).toContain(
      token
    )
  }
})
