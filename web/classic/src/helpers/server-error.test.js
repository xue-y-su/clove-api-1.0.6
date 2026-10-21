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

import { getServerErrorMessage } from './server-error';

test('formats workflow errors without exposing channel metadata', () => {
  const message = getServerErrorMessage(
    {
      response: {
        data: {
          channel: 'private-channel',
          message: 'Please check the submitted values.',
          request_id: 'request-visible',
        },
      },
    },
    'Request failed.',
    'Request ID',
  );

  expect(message).toBe(
    'Please check the submitted values. (Request ID: request-visible)',
  );
  expect(message).not.toContain('private-channel');
});

test('uses OpenAI-style public messages when present', () => {
  expect(
    getServerErrorMessage(
      {
        response: {
          data: {
            error: { message: 'Image generation is temporarily unavailable.' },
            request_id: 'image-request',
          },
        },
      },
      'Request failed.',
      'Request ID',
    ),
  ).toBe(
    'Image generation is temporarily unavailable. (Request ID: image-request)',
  );
});
