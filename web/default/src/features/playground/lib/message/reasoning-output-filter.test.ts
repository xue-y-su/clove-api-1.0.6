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
import { readFileSync } from 'node:fs'

import { createThinkTagFilter } from './message-reasoning-utils'
import {
  applyChatCompletionResponse,
  applyStreamingChunk,
  completeAssistantMessageWithOptions,
} from './message-streaming-utils'
import {
  createLoadingAssistantMessage,
  getMessageContent,
} from './message-utils'

const reasoningSource = readFileSync(
  new URL('./message-reasoning-utils.ts', import.meta.url),
  'utf8'
)
const handlerSource = readFileSync(
  new URL('../../hooks/use-chat-handler.ts', import.meta.url),
  'utf8'
)

describe('playground reasoning output suppression', () => {
  test('uses a request-scoped filter for split and dangling think tags', () => {
    expect(reasoningSource).toContain('createThinkTagFilter')
    expect(reasoningSource).toContain('insideThink')
    expect(reasoningSource).toContain('pendingDelimiter')
    expect(reasoningSource).toContain("const OPEN_TAG = '<think>'")
    expect(reasoningSource).toContain("const CLOSE_TAG = '</think>'")
  })

  test('drops reasoning before message mutation when reasoning is off', () => {
    expect(handlerSource).toContain("config.reasoning_effort === 'off'")
    expect(handlerSource).toContain("update.type === 'reasoning'")
    expect(handlerSource).toContain('reasoningOutputFilterRef')
    expect(handlerSource).toContain('hideReasoning: true')
  })

  test('removes complete think blocks while preserving visible text', () => {
    const filter = createThinkTagFilter()
    expect(filter.push('before <think>secret</think> after')).toBe(
      'before  after'
    )
    expect(filter.finish()).toBe('')
  })

  test('removes tags and reasoning split across stream chunks', () => {
    const filter = createThinkTagFilter()
    expect(filter.push('answer <thi')).toBe('answer ')
    expect(filter.push('nk>hidden')).toBe('')
    expect(filter.push('</thi')).toBe('')
    expect(filter.push('nk>done')).toBe('done')
  })

  test('drops dangling think content at stream completion', () => {
    const filter = createThinkTagFilter()
    expect(filter.push('visible<think>never closes')).toBe('visible')
    expect(filter.finish()).toBe('')
  })

  test('removes non-stream reasoning before producing the message', () => {
    const message = applyChatCompletionResponse(
      createLoadingAssistantMessage(),
      {
        id: 'response-1',
        object: 'chat.completion',
        created: 1,
        model: 'custom-model',
        choices: [
          {
            index: 0,
            message: {
              role: 'assistant',
              content: '<think>secret</think>Visible answer',
              reasoning_content: 'also secret',
            },
            finish_reason: 'stop',
          },
        ],
      },
      { hideReasoning: true }
    )

    expect(message).not.toBeNull()
    if (!message) throw new Error('expected an assistant message')
    expect(getMessageContent(message)).toBe('Visible answer')
    expect(message?.reasoning).toBeUndefined()
  })

  test('clears any streamed reasoning before final storage state', () => {
    let message = createLoadingAssistantMessage()
    message = applyStreamingChunk(message, 'reasoning', 'secret')
    message = applyStreamingChunk(
      message,
      'content',
      '<think>hidden</think>Visible answer'
    )

    const completed = completeAssistantMessageWithOptions(message, {
      hideReasoning: true,
    })
    expect(getMessageContent(completed)).toBe('Visible answer')
    expect(completed.reasoning).toBeUndefined()
  })

  test('shows a stable fallback when a hidden stream has no visible content', () => {
    let message = createLoadingAssistantMessage()
    message = applyStreamingChunk(
      message,
      'content',
      '<think>hidden only</think>'
    )

    const completed = completeAssistantMessageWithOptions(message, {
      hideReasoning: true,
    })

    expect(getMessageContent(completed)).toBe(
      'The model returned no displayable content'
    )
    expect(completed.reasoning).toBeUndefined()
  })

  test('shows a stable fallback for a hidden non-stream response', () => {
    const message = applyChatCompletionResponse(
      createLoadingAssistantMessage(),
      {
        id: 'response-2',
        object: 'chat.completion',
        created: 1,
        model: 'custom-model',
        choices: [
          {
            index: 0,
            message: {
              role: 'assistant',
              content: '<think>hidden only</think>',
            },
            finish_reason: 'stop',
          },
        ],
      },
      { hideReasoning: true }
    )

    expect(message).not.toBeNull()
    if (!message) throw new Error('expected an assistant message')
    expect(getMessageContent(message)).toBe(
      'The model returned no displayable content'
    )
  })
})
