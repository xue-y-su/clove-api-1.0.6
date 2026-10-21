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
import { expect, test } from 'bun:test'

import { getServerErrorMessage } from './handle-server-error'

test('shows the public message and request ID without channel metadata', () => {
  const message = getServerErrorMessage(
    {
      response: {
        data: {
          channel: 'rightcode-image2',
          message: 'The submitted amount exceeds the available balance.',
          request_id: 'request-visible',
        },
      },
    },
    'Request failed.',
    'Request ID'
  )

  expect(message).toBe(
    'The submitted amount exceeds the available balance. (Request ID: request-visible)'
  )
  expect(message).not.toContain('rightcode-image2')
})

test('does not append the same request ID twice', () => {
  expect(
    getServerErrorMessage(
      {
        response: {
          data: {
            message: 'Request failed (request id: request-visible)',
            request_id: 'request-visible',
          },
        },
      },
      'Request failed.',
      'Request ID'
    )
  ).toBe('Request failed (request id: request-visible)')
})
