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
  '90 Days',
  'All history',
  'Custom range',
  'Audit range',
  'IP audit reliability warning',
  'Trusted proxies are not configured. Client IP addresses may be inaccurate behind a reverse proxy.',
  'Trusted proxy configuration accepts untrusted sources. Review TRUSTED_PROXIES before relying on IP audit data.',
  'Registration',
  'API request',
  'API error',
  'Login success',
  'Login failure',
  'Other activity',
  'API requests',
  'Anomalies',
  'Select a complete custom range.',
  'Login success / failure',
  'Logout',
  'Logouts',
  'Login success / failure / logout',
  'Security & IP Audit',
  'Failed to load security audit',
  'No security activity in this range',
  'Registration IP',
  'Recent IP',
  'Related users',
  'Last activity',
  'Errors',
  'No related users',
  'Please try again.',
  'Security timeline',
  'IP audit data is still being prepared. Please retry shortly.',
  'You do not have permission to view this audit.',
  'IP audit is disabled',
  'IP security event collection is disabled by the administrator.',
  'Related accounts',
  'No related accounts',
  'API requests are attributed to the API Key owner. Authentication events are attributed to the acting account.',
  'Risk overview',
  'Shared API IPs',
  'Shared registration IPs',
  'Authentication activity',
  'All IP activity',
  'Registration accounts',
  'API accounts',
  'Authentication accounts',
  'Total accounts',
  'Account attribution',
  'Shared IPs',
  'API calls',
  'Authentication events',
  'Infrastructure IP',
]

describe('default IP audit locale coverage', () => {
  test('defines every audit key explicitly in all seven locales', () => {
    expect(requiredKeys).toHaveLength(52)

    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8')
      ) as Record<string, unknown>
      const translations =
        (locale.translation as Record<string, unknown> | undefined) ?? locale

      for (const key of requiredKeys) {
        expect(Object.hasOwn(translations, key)).toBe(true)
        expect(typeof translations[key]).toBe('string')
        expect((translations[key] as string).trim()).not.toBe('')
      }
    }
  })

  test('uses a neutral label for the current-month range', () => {
    const simplified = JSON.parse(
      fs.readFileSync(path.join(localesDirectory, 'zh.json'), 'utf8')
    ) as { translation: Record<string, string> }
    const traditional = JSON.parse(
      fs.readFileSync(path.join(localesDirectory, 'zh-TW.json'), 'utf8')
    ) as { translation: Record<string, string> }

    expect(simplified.translation['This month']).toBe('本月')
    expect(traditional.translation['This month']).toBe('本月')
  })
})
