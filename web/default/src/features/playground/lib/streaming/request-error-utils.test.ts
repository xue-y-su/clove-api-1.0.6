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

import { parseRequestErrorDetails } from './request-error-utils'

describe('playground request errors', () => {
  test('prefers the structured provider message and code', () => {
    expect(
      parseRequestErrorDetails({
        message: 'Request failed with status code 502',
        response: {
          status: 502,
          data: {
            error: {
              code: 'bad_response_status_code',
              message: 'bad response status code 502 (request id: request-502)',
            },
          },
        },
      })
    ).toEqual({
      errorCode: 'bad_response_status_code',
      errorMessage: 'bad response status code 502 (request id: request-502)',
    })
  })

  test('replaces an opaque provider message with an actionable HTTP status', () => {
    expect(
      parseRequestErrorDetails({
        message: 'Request failed with status code 502',
        response: {
          status: 502,
          data: {
            error: {
              code: 'bad_response_status_code',
              message: 'openai_error',
            },
          },
        },
      })
    ).toEqual({
      errorCode: 'bad_response_status_code',
      errorMessage: 'HTTP 502: Connection closed',
    })
  })
})
