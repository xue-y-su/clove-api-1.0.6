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

import { formatCompactToolSurcharges } from './billing-breakdown.js';

describe('formatCompactToolSurcharges', () => {
  test('keeps confirmed tool charges on one detail line', () => {
    expect(
      formatCompactToolSurcharges(
        [
          { name: 'web_search', count: 1, subtotal_quota: 2500 },
          { name: 'file_search', count: 2, subtotal_quota: 1250 },
        ],
        (quota) => `${quota} quota`,
      ),
    ).toBe('web_search x1 (2500 quota) · file_search x2 (1250 quota)');
  });

  test('drops malformed historical rows', () => {
    expect(
      formatCompactToolSurcharges(
        [
          { name: '', count: 1, subtotal_quota: 1 },
          { name: 'web_search', count: 0, subtotal_quota: 0 },
        ],
        String,
      ),
    ).toBe('');
  });

  test('translates billing breakdown labels in every locale', async () => {
    const keys = ['扣费构成', '模型扣费', '工具扣费', '合计', '工具附加计价'];
    for (const localeFile of [
      'en.json',
      'fr.json',
      'ja.json',
      'ru.json',
      'vi.json',
      'zh-CN.json',
      'zh-TW.json',
      'zh.json',
    ]) {
      const locale = await Bun.file(
        new URL(`../../i18n/locales/${localeFile}`, import.meta.url),
      ).json();
      for (const key of keys) {
        expect(locale.translation[key], `${localeFile}: ${key}`).toBeTruthy();
      }
    }
  });
});
