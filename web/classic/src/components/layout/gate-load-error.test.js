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
import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

import { extractGateRateLimit, extractGateRequestId } from './gate-load-error';

test('extracts the classic gate request identifier from safe locations', () => {
  expect(
    extractGateRequestId({
      response: {
        data: { request_id: 'request-from-body' },
        headers: { 'x-request-id': 'request-from-header' },
      },
    }),
  ).toBe('request-from-body');
  expect(
    extractGateRequestId(
      new Error('upstream unavailable (request id: request-from-message)'),
    ),
  ).toBe('request-from-message');
  expect(extractGateRequestId(new Error('network unavailable'))).toBeNull();
});

test('extracts the classic gate retry delay from structured 429 responses', () => {
  expect(
    extractGateRateLimit({
      response: {
        status: 429,
        data: { retry_after: 68 },
        headers: { 'retry-after': '90' },
      },
    }),
  ).toEqual({ retryAfterSeconds: 68 });
  expect(
    extractGateRateLimit({
      response: {
        status: 429,
        data: {},
        headers: { 'Retry-After': '31' },
      },
    }),
  ).toEqual({ retryAfterSeconds: 31 });
  expect(extractGateRateLimit(new Error('network unavailable'))).toBeNull();
});

test('localizes the classic rate-limit recovery message in every active locale', () => {
  const localeNames = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW', 'zh'];
  const keys = ['请求过于频繁', '{{seconds}} 秒后可重新查询。'];

  for (const localeName of localeNames) {
    const locale = JSON.parse(
      readFileSync(
        new URL(`../../i18n/locales/${localeName}.json`, import.meta.url),
        'utf8',
      ),
    ).translation;
    for (const key of keys) {
      expect(typeof locale[key], `${localeName}: ${key}`).toBe('string');
      expect(locale[key].trim(), `${localeName}: ${key}`).not.toBe('');
    }
  }
});
