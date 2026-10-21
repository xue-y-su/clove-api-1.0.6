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
import { readFileSync } from 'node:fs';

import { normalizeInvitationRequestError } from './invitation-request-error';

const apiSource = readFileSync(new URL('./api.js', import.meta.url), 'utf8');

describe('classic invitation mutation API', () => {
  test('exposes cancellable public invitation validation', () => {
    expect(apiSource).toContain('validateInvitationCode');
    expect(apiSource).toContain('/api/invitation/validate');
    expect(apiSource).toContain('signal');
  });

  test('owns mutation errors without invoking the global error toast', async () => {
    expect(apiSource).toContain('skipErrorHandler: true');
  });

  test('surfaces the server message instead of the raw Axios status text', async () => {
    const normalized = normalizeInvitationRequestError(
      Object.assign(new Error('Request failed with status code 429'), {
        response: {
          data: { message: '请求过于频繁，请稍后重试。' },
        },
      }),
      '请求失败',
    );

    expect(normalized.message).toBe('请求过于频繁，请稍后重试。');
  });
});
