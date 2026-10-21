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

import type { AxiosAdapter } from 'axios'

import { api } from './api'

test('keeps GET requests with caller-owned abort signals independent', async () => {
  let calls = 0
  const adapter: AxiosAdapter = async (config) => {
    const requestNumber = ++calls
    return {
      config,
      data: { requestNumber },
      headers: {},
      status: 200,
      statusText: 'OK',
    }
  }

  const [first, second] = await Promise.all([
    api.get('/__tests__/signal-deduplication', {
      adapter,
      signal: new AbortController().signal,
      skipErrorHandler: true,
    }),
    api.get('/__tests__/signal-deduplication', {
      adapter,
      signal: new AbortController().signal,
      skipErrorHandler: true,
    }),
  ])

  expect(calls).toBe(2)
  expect([first.data.requestNumber, second.data.requestNumber]).toEqual([1, 2])
})

test('does not report intentional request cancellation as a global error', () => {
  const source = readFileSync(new URL('./api.ts', import.meta.url), 'utf8')
  const cancellationGuard = source.indexOf('if (axios.isCancel(error))')
  const globalErrorHandling = source.indexOf(
    'const skip = error?.config?.skipErrorHandler'
  )

  expect(cancellationGuard).toBeGreaterThan(-1)
  expect(cancellationGuard).toBeLessThan(globalErrorHandling)
})
