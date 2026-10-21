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
  getImageGenerationCapabilities,
  isKnownImageGenerationModel,
  isKnownTextOnlyModelForImageInput,
  isKnownVisionModel,
  shouldBlockImageInputForModel,
  supportsPlaygroundReasoning,
} from './model-capability-utils'

describe('playground model capability utilities', () => {
  test('does not block image input from model-name guesses', () => {
    expect(shouldBlockImageInputForModel('deepseek-r1')).toBe(false)
    expect(shouldBlockImageInputForModel('deepseek-chat')).toBe(false)
    expect(shouldBlockImageInputForModel('text-embedding-3-small')).toBe(false)
    expect(isKnownTextOnlyModelForImageInput('claude-3-5-haiku')).toBe(false)
  })

  test('allows known vision model families', () => {
    expect(isKnownVisionModel('gpt-4o')).toBe(true)
    expect(isKnownVisionModel('gemini-2.5-pro')).toBe(true)
    expect(isKnownVisionModel('claude-3-5-sonnet')).toBe(true)
    expect(isKnownVisionModel('qwen2.5-vl-72b')).toBe(true)
    expect(shouldBlockImageInputForModel('deepseek-vl2')).toBe(false)
  })

  test('does not block unknown custom models', () => {
    expect(shouldBlockImageInputForModel('my-company-vision-preview')).toBe(
      false
    )
    expect(shouldBlockImageInputForModel('custom-model')).toBe(false)
  })

  test('recognizes common image generation model families', () => {
    expect(isKnownImageGenerationModel('gpt-image-1')).toBe(true)
    expect(isKnownImageGenerationModel('dall-e-3')).toBe(true)
    expect(isKnownImageGenerationModel('imagen-4')).toBe(true)
    expect(isKnownImageGenerationModel('flux-pro')).toBe(true)
    expect(isKnownImageGenerationModel('gpt-4o')).toBe(false)
  })

  test('allows reasoning effort only for supported model families', () => {
    expect(supportsPlaygroundReasoning('gpt-5-mini')).toBe(true)
    expect(supportsPlaygroundReasoning('o3-mini')).toBe(true)
    expect(supportsPlaygroundReasoning('grok-3-mini')).toBe(true)
    expect(supportsPlaygroundReasoning('gpt-4o')).toBe(false)
  })

  test('describes model-specific image controls', () => {
    expect(getImageGenerationCapabilities('gpt-image-1')).toEqual({
      background: true,
      quality: ['auto', 'low', 'medium', 'high'],
      style: [],
    })
    expect(getImageGenerationCapabilities('dall-e-3')).toEqual({
      background: false,
      quality: ['standard', 'hd'],
      style: ['vivid', 'natural'],
    })
  })
})
