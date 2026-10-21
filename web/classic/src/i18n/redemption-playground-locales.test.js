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
  '高级设置',
  '设置自定义 Key 和访问限制',
  '自定义 API Key',
  '可选自定义 Key',
  '仅在单个创建时可用',
  '模型未返回可显示内容',
  '终止',
];

describe('classic redemption and playground locale coverage', () => {
  test('defines the new controls in every shipped locale', () => {
    for (const localeFile of localeFiles) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8'),
      );
      const translations = locale.translation || locale;

      for (const key of requiredKeys) {
        expect(Object.hasOwn(translations, key)).toBe(true);
        expect(typeof translations[key]).toBe('string');
        expect(translations[key].trim()).not.toBe('');
      }
    }
  });

  test('does not ship Simplified Chinese fallbacks in other languages', () => {
    for (const localeFile of localeFiles.filter(
      (file) => file !== 'zh.json' && file !== 'zh-CN.json',
    )) {
      const locale = JSON.parse(
        fs.readFileSync(path.join(localesDirectory, localeFile), 'utf8'),
      );
      const translations = locale.translation || locale;

      for (const key of requiredKeys) {
        expect(translations[key]).not.toBe(key);
      }
    }
  });
});
