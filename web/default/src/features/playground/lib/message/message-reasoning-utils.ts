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
interface ParsedThinkTags {
  visibleContent: string
  reasoning: string
  hasUnclosedTag: boolean
}

const OPEN_TAG = '<think>'
const CLOSE_TAG = '</think>'

export interface ThinkTagFilter {
  push: (chunk: string) => string
  finish: () => string
}

function longestDelimiterPrefixSuffix(
  value: string,
  delimiter: string
): string {
  const maxLength = Math.min(value.length, delimiter.length - 1)
  for (let length = maxLength; length > 0; length--) {
    const suffix = value.slice(-length)
    if (delimiter.startsWith(suffix)) return suffix
  }
  return ''
}

export function createThinkTagFilter(): ThinkTagFilter {
  let insideThink = false
  let pendingDelimiter = ''

  return {
    push(chunk: string): string {
      let input = pendingDelimiter + chunk
      let visible = ''
      pendingDelimiter = ''

      while (input) {
        const delimiter = insideThink ? CLOSE_TAG : OPEN_TAG
        const delimiterIndex = input.indexOf(delimiter)

        if (delimiterIndex >= 0) {
          if (!insideThink) visible += input.slice(0, delimiterIndex)
          input = input.slice(delimiterIndex + delimiter.length)
          insideThink = !insideThink
          continue
        }

        const pending = longestDelimiterPrefixSuffix(input, delimiter)
        const stableLength = input.length - pending.length
        if (!insideThink) visible += input.slice(0, stableLength)
        pendingDelimiter = pending
        break
      }

      return visible
    },
    finish(): string {
      insideThink = false
      pendingDelimiter = ''
      return ''
    },
  }
}

/**
 * Parse content to separate thinking from visible text.
 * Handles both complete and incomplete <think> tags.
 */
export function parseThinkTags(content: string): ParsedThinkTags {
  if (!content.includes('<think>')) {
    return { visibleContent: content, reasoning: '', hasUnclosedTag: false }
  }

  const visibleParts: string[] = []
  const reasoningParts: string[] = []
  let currentPos = 0
  let hasUnclosedTag = false

  while (true) {
    const openPos = content.indexOf('<think>', currentPos)

    if (openPos === -1) {
      if (currentPos < content.length) {
        visibleParts.push(content.slice(currentPos))
      }
      break
    }

    if (openPos > currentPos) {
      visibleParts.push(content.slice(currentPos, openPos))
    }

    const closePos = content.indexOf('</think>', openPos + 7)

    if (closePos === -1) {
      reasoningParts.push(content.slice(openPos + 7))
      hasUnclosedTag = true
      break
    }

    reasoningParts.push(content.slice(openPos + 7, closePos))
    currentPos = closePos + 8
  }

  return {
    visibleContent: visibleParts.join('').trim(),
    reasoning: reasoningParts.join('\n\n').trim(),
    hasUnclosedTag,
  }
}
