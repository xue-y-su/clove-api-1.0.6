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
import { ERROR_MESSAGES } from '../../constants'
import type { ChatCompletionChunk, PlaygroundSearchMetadata } from '../../types'

const STREAM_DONE_MESSAGE = '[DONE]'
const STREAM_CLOSED_READY_STATE = 2

export type StreamMessageUpdate =
  | { type: 'reasoning' | 'content'; chunk: string }
  | { type: 'metadata'; metadata: PlaygroundSearchMetadata }

type StreamErrorPayload = {
  error?: {
    code?: string
    message?: string
  }
}

export type StreamErrorDetails = {
  errorCode?: string
  errorMessage: string
}

function normalizeSearchSources(
  value: unknown
): PlaygroundSearchMetadata['sources'] {
  if (!Array.isArray(value)) return []

  return value.flatMap((source) => {
    if (typeof source !== 'object' || source === null) return []
    const record = source as Record<string, unknown>
    if (typeof record.url !== 'string' || !record.url.trim()) return []
    return [
      {
        title: typeof record.title === 'string' ? record.title : '',
        url: record.url,
        domain: typeof record.domain === 'string' ? record.domain : '',
        snippet: typeof record.snippet === 'string' ? record.snippet : '',
      },
    ]
  })
}

function findStreamErrorPayload(data: string): StreamErrorPayload | null {
  const candidates = [data]
  for (const block of data.split(/\r?\n\r?\n/)) {
    const trimmedBlock = block.trim()
    if (!trimmedBlock) continue

    candidates.push(trimmedBlock)
    const dataLines = trimmedBlock.split(/\r?\n/).map((line) => line.trim())
    if (dataLines.every((line) => line.startsWith('data:'))) {
      candidates.push(
        dataLines.map((line) => line.slice('data:'.length).trim()).join('\n')
      )
    }
  }

  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(candidate) as StreamErrorPayload
      if (parsed?.error) return parsed
    } catch {
      continue
    }
  }
  return null
}

export function parseStreamErrorDetails(
  data?: string,
  status?: number
): StreamErrorDetails {
  if (!data) {
    return { errorMessage: ERROR_MESSAGES.API_REQUEST_ERROR }
  }

  const parsed = findStreamErrorPayload(data)
  if (!parsed?.error) {
    return {
      errorMessage: data.trimStart().startsWith('data:')
        ? ERROR_MESSAGES.API_REQUEST_ERROR
        : data,
    }
  }

  const errorCode = parsed.error.code || undefined
  const providerMessage = parsed.error.message?.trim() || ''
  if (
    (!providerMessage || providerMessage.toLowerCase() === 'openai_error') &&
    Number.isInteger(status) &&
    Number(status) >= 400
  ) {
    return {
      errorCode,
      errorMessage: `HTTP ${status}: ${ERROR_MESSAGES.CONNECTION_CLOSED}`,
    }
  }

  return {
    errorCode,
    errorMessage: providerMessage || ERROR_MESSAGES.API_REQUEST_ERROR,
  }
}

export function parseStreamMessageUpdates(data: string): StreamMessageUpdate[] {
  const chunk = JSON.parse(data) as ChatCompletionChunk & {
    type?: string
    search?: PlaygroundSearchMetadata
  }
  if (chunk.type === 'playground.metadata' && chunk.search) {
    return [
      {
        type: 'metadata',
        metadata: {
          ...chunk.search,
          sources: normalizeSearchSources(chunk.search.sources),
        },
      },
    ]
  }
  const delta = chunk.choices?.[0]?.delta

  if (!delta) {
    return []
  }

  const updates: StreamMessageUpdate[] = []

  if (delta.reasoning_content) {
    updates.push({ type: 'reasoning', chunk: delta.reasoning_content })
  }

  if (delta.content) {
    updates.push({ type: 'content', chunk: delta.content })
  }

  return updates
}

export function isStreamDoneMessage(data: string): boolean {
  return data === STREAM_DONE_MESSAGE
}

export function isStreamClosedReadyState(readyState?: number): boolean {
  return readyState === STREAM_CLOSED_READY_STATE
}

export function getStreamReadyStateError(
  eventReadyState: number | undefined,
  source: unknown
): string | null {
  const status = (source as { status?: number }).status

  if (
    eventReadyState !== undefined &&
    eventReadyState >= STREAM_CLOSED_READY_STATE &&
    status !== undefined &&
    status !== 200
  ) {
    return `HTTP ${status}: ${ERROR_MESSAGES.CONNECTION_CLOSED}`
  }

  return null
}
