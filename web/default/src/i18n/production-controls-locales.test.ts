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
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const localesDirectory = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  'locales'
)

const localeFiles = [
  'en.json',
  'fr.json',
  'ja.json',
  'ru.json',
  'vi.json',
  'zh-TW.json',
  'zh.json',
]

const requiredKeys = [
  'Allow custom API Key creation',
  'Allow published announcements to appear in the site.',
  'Allowed avatar domains',
  'Announcement image exceeds the configured size limit',
  'Announcement settings saved',
  'Avatar upload endpoint',
  'Configure avatar uploads and display name changes.',
  'Configure event collection, retention, and projection worker limits.',
  'Control whether users may choose a custom value when creating an API Key.',
  'Custom API Keys',
  'Display name cooldown (seconds)',
  'Enable IP security event collection',
  'Enable announcements',
  'Enable avatar uploads',
  'Failed to load announcement settings',
  'Failed to load security settings',
  'Failed to update announcement settings',
  'Feature Controls',
  'IP Security Audit',
  'Maximum announcement image size (MiB)',
  'Maximum display name length',
  'PNG, JPEG, GIF, or WebP. Maximum {{size}} MiB.',
  'Projection batch size',
  'Projection interval (seconds)',
  'Shared registration user threshold',
  'Shared API user threshold',
  'Unauthorized request threshold',
  'Save announcement settings',
  'Save security settings',
  'Security settings saved',
  'Show unread announcements as a popup',
  'User profile policy',
  'Users confirm each published announcement once.',
  'User redemption codes',
  'Control which user-funded redemption codes can be created and limit their lifetime and recipients.',
  'Allow users to create redemption codes',
  'Existing codes remain claimable and manageable when new creation is disabled.',
  'Allow single-recipient codes',
  'Allow red packets',
  'Maximum active codes per user',
  'Set 0 for no active-code limit.',
  'Maximum red packet recipients',
  'Maximum validity (hours)',
  'This redemption mode is currently disabled',
  '1 hour',
  '{{count}} hours',
  '{{count}} days',
  'Expire unused check-in rewards daily',
  'New check-in rewards expire at the next midnight. Wallet spending uses the reward first.',
  'Unused check-in reward expires at midnight',
]

describe('default production controls locale coverage', () => {
  test('translates every production control in all seven locales', () => {
    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8')
      ) as { translation: Record<string, string> }

      const missing = requiredKeys.filter(
        (key) => !locale.translation[key]?.trim()
      )
      const untranslated =
        localeFile === 'en.json'
          ? []
          : requiredKeys.filter((key) => locale.translation[key] === key)

      expect({ localeFile, missing, untranslated }).toEqual({
        localeFile,
        missing: [],
        untranslated: [],
      })
    }
  })
})
