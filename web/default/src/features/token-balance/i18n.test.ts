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

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'

const pageSource = await Bun.file(
  new URL('./index.tsx', import.meta.url)
).text()

describe('token balance translations', () => {
  test('accepts custom keys without implying an sk prefix', () => {
    expect(pageSource).toContain("placeholder={t('Enter API Key')}")
    expect(pageSource).not.toContain("placeholder='sk-...'")
  })

  test('translates balance detail labels in every supported locale', () => {
    const dynamicKeys = [
      'Restriction Reason',
      'Key Limit',
      'The API Key is invalid. Check it and try again.',
      'Too Many Requests',
      'Try again in {{seconds}} seconds.',
      'The balance service is temporarily unavailable. Try again later.',
      'This API Key has been disabled.',
      'This API Key has expired.',
      'This API Key has no remaining quota.',
      'The account for this API Key has been disabled.',
    ] as const

    for (const locale of [en, zh, zhTW, fr, ru, ja, vi]) {
      for (const key of dynamicKeys) {
        expect(locale.translation[key]).toBeTruthy()
      }
    }
  })
})
