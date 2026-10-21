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

import { formatCompactToolSurcharges } from './billing-breakdown'

describe('formatCompactToolSurcharges', () => {
  test('renders confirmed tool charges as one compact detail line', () => {
    expect(
      formatCompactToolSurcharges(
        [
          {
            name: 'web_search_preview',
            count: 1,
            unit: 'usd_per_1k_calls',
            unit_price: 10,
            subtotal_quota: 2500,
          },
          {
            name: 'file_search',
            count: 2,
            unit: 'usd_per_1k_calls',
            unit_price: 2.5,
            subtotal_quota: 1250,
          },
        ],
        (quota) => `${quota} quota`
      )
    ).toBe('web_search_preview x1 (2500 quota) · file_search x2 (1250 quota)')
  })

  test('ignores malformed or zero-count historical rows', () => {
    expect(
      formatCompactToolSurcharges(
        [
          {
            name: '',
            count: 1,
            unit: 'usd_per_call',
            unit_price: 1,
            subtotal_quota: 1,
          },
          {
            name: 'web_search',
            count: 0,
            unit: 'usd_per_1k_calls',
            unit_price: 10,
            subtotal_quota: 0,
          },
        ],
        String
      )
    ).toBe('')
  })

  test('translates billing breakdown labels in every locale', async () => {
    const keys = ['Model Charge', 'Tool Charge', 'Tool Surcharges'] as const
    for (const localeFile of [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = (await Bun.file(
        new URL(`../../../i18n/locales/${localeFile}`, import.meta.url)
      ).json()) as { translation: Record<string, string> }
      for (const key of keys) {
        expect(locale.translation[key], `${localeFile}: ${key}`).toBeTruthy()
      }
    }
  })
})
