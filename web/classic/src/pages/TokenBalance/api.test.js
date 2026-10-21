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

import { afterEach, describe, expect, mock, test } from 'bun:test';

import { queryTokenBalance } from './api';

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe('classic token balance API', () => {
  test('posts the key without browser credentials', async () => {
    const fetchMock = mock(
      async () =>
        new Response(
          JSON.stringify({ success: false, code: 'invalid_api_key' }),
          {
            status: 401,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
    );
    globalThis.fetch = fetchMock;

    await queryTokenBalance('sk-test');

    const [url, options] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/token-balance');
    expect(options.credentials).toBe('omit');
    expect(JSON.parse(String(options.body))).toEqual({ key: 'sk-test' });
  });

  test('returns retry metadata from a rate limit response', async () => {
    globalThis.fetch = mock(
      async () =>
        new Response(JSON.stringify({ success: false, code: 'rate_limited' }), {
          status: 429,
          headers: {
            'Content-Type': 'application/json',
            'Retry-After': '60',
          },
        }),
    );

    const response = await queryTokenBalance('sk-test');

    expect(response.retry_after_seconds).toBe(60);
  });

  test('maps malformed error responses to a generic query failure', async () => {
    globalThis.fetch = mock(
      async () =>
        new Response('upstream unavailable', {
          status: 502,
          headers: { 'Content-Type': 'text/plain' },
        }),
    );

    await expect(queryTokenBalance('sk-test')).resolves.toEqual({
      success: false,
      code: 'query_failed',
    });
  });
});
