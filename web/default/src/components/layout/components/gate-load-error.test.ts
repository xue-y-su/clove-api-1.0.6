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
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import {
  extractGateRateLimit,
  extractGateRequestId,
} from './gate-load-error-utils'

test('keeps request parsing outside the Fast Refresh component boundary', () => {
  const componentSource = readFileSync(
    fileURLToPath(new URL('./gate-load-error.tsx', import.meta.url)),
    'utf8'
  )

  expect(componentSource).not.toContain('export function extractGateRequestId')
})

test('extracts a request identifier without exposing technical error details', () => {
  expect(
    extractGateRequestId({
      response: {
        data: { request_id: 'request-from-body' },
        headers: { 'x-request-id': 'request-from-header' },
      },
    })
  ).toBe('request-from-body')
  expect(
    extractGateRequestId(
      new Error('upstream unavailable (request id: request-from-message)')
    )
  ).toBe('request-from-message')
  expect(extractGateRequestId(new Error('network unavailable'))).toBeNull()
})

test('extracts structured retry information from a rate limited gate request', () => {
  expect(
    extractGateRateLimit({
      response: {
        status: 429,
        data: { retry_after: 73 },
        headers: { 'retry-after': '91' },
      },
    })
  ).toEqual({ retryAfterSeconds: 73 })

  expect(
    extractGateRateLimit({
      response: {
        status: 429,
        data: {},
        headers: { 'Retry-After': '45' },
      },
    })
  ).toEqual({ retryAfterSeconds: 45 })
  expect(extractGateRateLimit(new Error('network unavailable'))).toBeNull()
})
