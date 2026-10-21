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

import {
  parseStreamErrorDetails,
  parseStreamMessageUpdates,
} from './stream-utils'

describe('playground stream errors', () => {
  test('extracts the actual provider error without leaking preceding metadata', () => {
    const responseText = [
      'data: {"type":"playground.metadata","search":{"status":"unavailable","path":"unavailable"}}',
      '',
      '{"error":{"message":"provider temporarily unavailable","code":"upstream_unavailable"}}',
    ].join('\n')

    expect(parseStreamErrorDetails(responseText, 502)).toEqual({
      errorCode: 'upstream_unavailable',
      errorMessage: 'provider temporarily unavailable',
    })
  })

  test('replaces an opaque upstream error with an actionable HTTP status', () => {
    const responseText = [
      'data: {"type":"playground.metadata","search":{"status":"unavailable","path":"unavailable"}}',
      '',
      '{"error":{"message":"openai_error","code":"bad_response_status_code"}}',
    ].join('\n')

    expect(parseStreamErrorDetails(responseText, 502)).toEqual({
      errorCode: 'bad_response_status_code',
      errorMessage: 'HTTP 502: Connection closed',
    })
  })
})

describe('playground stream metadata', () => {
  test('parses search metadata without rendering it as model content', () => {
    expect(
      parseStreamMessageUpdates(
        JSON.stringify({
          type: 'playground.metadata',
          search: {
            status: 'ok',
            path: 'tavily',
            sources: [
              {
                title: 'Source',
                url: 'https://example.test/a',
                domain: 'example.test',
                snippet: 'Evidence',
              },
            ],
          },
        })
      )
    ).toEqual([
      {
        type: 'metadata',
        metadata: {
          status: 'ok',
          path: 'tavily',
          sources: [
            {
              title: 'Source',
              url: 'https://example.test/a',
              domain: 'example.test',
              snippet: 'Evidence',
            },
          ],
        },
      },
    ])
  })

  test('keeps normal content chunks unchanged', () => {
    expect(
      parseStreamMessageUpdates(
        JSON.stringify({ choices: [{ delta: { content: 'hello' } }] })
      )
    ).toEqual([{ type: 'content', chunk: 'hello' }])
  })

  test('normalizes missing search sources to an empty list', () => {
    expect(
      parseStreamMessageUpdates(
        JSON.stringify({
          type: 'playground.metadata',
          search: {
            status: 'ok',
            path: 'builtin',
          },
        })
      )
    ).toEqual([
      {
        type: 'metadata',
        metadata: {
          status: 'ok',
          path: 'builtin',
          sources: [],
        },
      },
    ])
  })

  test('drops malformed search sources before the UI consumes them', () => {
    expect(
      parseStreamMessageUpdates(
        JSON.stringify({
          type: 'playground.metadata',
          search: {
            status: 'ok',
            path: 'builtin',
            sources: [
              null,
              { title: 'Missing URL' },
              {
                title: 'Valid source',
                url: 'https://example.test/source',
                domain: 'example.test',
                snippet: 'Evidence',
              },
            ],
          },
        })
      )
    ).toEqual([
      {
        type: 'metadata',
        metadata: {
          status: 'ok',
          path: 'builtin',
          sources: [
            {
              title: 'Valid source',
              url: 'https://example.test/source',
              domain: 'example.test',
              snippet: 'Evidence',
            },
          ],
        },
      },
    ])
  })
})
