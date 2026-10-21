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
import { describe, expect, test } from 'bun:test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const localesDirectory = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  'locales',
);

const localeFiles = [
  'en.json',
  'fr.json',
  'ja.json',
  'ru.json',
  'vi.json',
  'zh-CN.json',
  'zh-TW.json',
  'zh.json',
];

const requiredKeys = [
  'Failed to load security settings',
  'Failed to update settings',
  'Security settings saved',
  'Feature Controls',
  'Custom API Keys',
  'Control whether users may choose a custom value when creating an API Key.',
  'Allow custom API Key creation',
  'User profile policy',
  'Configure avatar uploads and display name changes.',
  'Enable avatar uploads',
  'Avatar upload endpoint',
  'Allowed avatar domains',
  'One domain per line',
  'Maximum display name length',
  'Display name cooldown (seconds)',
  'IP Security Audit',
  'Enable IP security event collection',
  'Retention days',
  'Projection interval (seconds)',
  'Projection batch size',
  'Shared registration user threshold',
  'Shared API user threshold',
  'Unauthorized request threshold',
  'Save security settings',
  'Failed to load announcement settings',
  'Failed to update announcement settings',
  'Announcement settings saved',
  'Announcement image exceeds the configured size limit',
  'Enable announcements',
  'Show unread announcements as a popup',
  'Maximum announcement image size (MiB)',
  'Save announcement settings',
  'PNG, JPEG, GIF, or WebP. Maximum {{size}} MiB.',
  'Announcements saved successfully',
  'Failed to save announcements',
  'Announcement deleted. Click "Save Settings" to apply.',
  'Announcement updated. Click "Save Settings" to apply.',
  'Announcement added. Click "Save Settings" to apply.',
  'Please complete all announcement fields',
  'Please select items to delete',
  '{{count}} announcements deleted. Click "Save Settings" to apply.',
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
];

describe('classic production controls locale coverage', () => {
  test('translates every production control in all eight locales', () => {
    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8'),
      );
      const translations = locale.translation || locale;
      const missing = requiredKeys.filter((key) => !translations[key]?.trim());
      const untranslated =
        localeFile === 'en.json'
          ? []
          : requiredKeys.filter((key) => translations[key] === key);

      expect({ localeFile, missing, untranslated }).toEqual({
        localeFile,
        missing: [],
        untranslated: [],
      });
    }
  });
});
