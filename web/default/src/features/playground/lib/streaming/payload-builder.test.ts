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

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../../constants'
import type { Message, PlaygroundConfig } from '../../types'
import { buildChatCompletionPayload } from './payload-builder'

describe('playground payload builder', () => {
  const userMessage: Message = {
    key: 'user-1',
    from: 'user',
    versions: [{ id: 'v1', content: 'What changed today?' }],
  }

  test('adds basic hosted web search fields when playground search is basic', () => {
    const config = {
      ...DEFAULT_CONFIG,
      web_search: 'basic',
    } as PlaygroundConfig

    expect(
      buildChatCompletionPayload(
        [userMessage],
        config,
        DEFAULT_PARAMETER_ENABLED
      )
    ).toMatchObject({
      tools: [
        {
          type: 'web_search_preview',
          search_context_size: 'low',
        },
      ],
      web_search_options: {
        search_context_size: 'low',
      },
    })
  })

  test('adds advanced hosted web search fields when playground search is advanced', () => {
    const payload = buildChatCompletionPayload(
      [userMessage],
      { ...DEFAULT_CONFIG, web_search: 'advanced' },
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload).toMatchObject({
      tools: [
        {
          type: 'web_search_preview',
          search_context_size: 'high',
        },
      ],
      web_search_options: {
        search_context_size: 'high',
      },
    })
  })

  test('does not add web search fields when playground search is off', () => {
    const payload = buildChatCompletionPayload(
      [userMessage],
      { ...DEFAULT_CONFIG, web_search: 'off' },
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload).not.toHaveProperty('tools')
    expect(payload).not.toHaveProperty('web_search_options')
  })

  test('sends an explicit disable value when reasoning is off', () => {
    const payload = buildChatCompletionPayload(
      [userMessage],
      { ...DEFAULT_CONFIG, model: 'gpt-5-mini', reasoning_effort: 'off' },
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload).toMatchObject({ reasoning_effort: 'none' })
  })

  test('sends selected reasoning effort when enabled', () => {
    const payload = buildChatCompletionPayload(
      [userMessage],
      { ...DEFAULT_CONFIG, model: 'gpt-5-mini', reasoning_effort: 'high' },
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload).toMatchObject({ reasoning_effort: 'high' })
  })

  test('sends reasoning effort without model-name gating', () => {
    const payload = buildChatCompletionPayload(
      [userMessage],
      { ...DEFAULT_CONFIG, model: 'gpt-4o', reasoning_effort: 'high' },
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload).toMatchObject({ reasoning_effort: 'high' })
  })
})
