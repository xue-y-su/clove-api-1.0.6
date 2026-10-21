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

import {
  buildImageGenerationRequest,
  createImagePreviews,
  getImageGenerationErrorMessage,
  revokeImagePreviews,
} from './image-generation'

describe('playground image generation', () => {
  test('gives every image option select an accessible name', () => {
    const panelSource = readFileSync(
      new URL(
        '../../components/image/playground-image-panel.tsx',
        import.meta.url
      ),
      'utf8'
    )

    for (const label of ['Group', 'Size', 'Quality', 'Style', 'Background']) {
      expect(panelSource).toContain(`aria-label={t('${label}')}`)
    }
    expect(panelSource).toContain("ariaLabel={t('Model')}")
  })

  test('turns base64 images into revocable object URLs', () => {
    const created: Blob[] = []
    const revoked: string[] = []
    const previews = createImagePreviews(
      [
        { b64_json: 'aGVsbG8=', revised_prompt: 'hello' },
        { url: 'https://example.test/image.png' },
      ],
      {
        prompt: 'submitted prompt',
        createObjectURL: (blob) => {
          created.push(blob)
          return 'blob:preview-1'
        },
      }
    )

    expect(previews.map((item) => item.src)).toEqual([
      'blob:preview-1',
      'https://example.test/image.png',
    ])
    expect(previews.map((item) => item.id)).toEqual([
      'blob:preview-1:0',
      'https://example.test/image.png:1',
    ])
    expect(previews.map((item) => item.displayPrompt)).toEqual([
      'hello',
      'submitted prompt',
    ])
    expect(created).toHaveLength(1)

    revokeImagePreviews(previews, (url) => revoked.push(url))
    expect(revoked).toEqual(['blob:preview-1'])
  })

  test('sends only capability-supported optional fields', () => {
    expect(
      buildImageGenerationRequest({
        background: 'transparent',
        count: 2,
        group: 'default',
        model: 'gpt-image-1',
        prompt: 'a snow house',
        quality: 'high',
        size: '1024x1024',
        style: 'vivid',
      })
    ).toEqual({
      background: 'transparent',
      group: 'default',
      model: 'gpt-image-1',
      n: 2,
      prompt: 'a snow house',
      quality: 'high',
      response_format: 'url',
      size: '1024x1024',
    })
  })

  test('omits provider-controlled auto options', () => {
    expect(
      buildImageGenerationRequest({
        background: 'auto',
        count: 1,
        group: 'default',
        model: 'gpt-image-1',
        prompt: 'a quiet lake',
        quality: 'auto',
        size: '1024x1024',
      })
    ).toEqual({
      group: 'default',
      model: 'gpt-image-1',
      n: 1,
      prompt: 'a quiet lake',
      response_format: 'url',
      size: '1024x1024',
    })
  })

  test('prefers the provider error message over the generic HTTP status', () => {
    expect(
      getImageGenerationErrorMessage({
        message: 'Request failed with status code 503',
        response: {
          status: 503,
          data: { error: { message: 'upstream image service unavailable' } },
        },
      })
    ).toBe('upstream image service unavailable')
  })

  test('keeps the HTTP status when the provider gives no message', () => {
    expect(
      getImageGenerationErrorMessage({
        message: 'Request failed with status code 503',
        response: { status: 503, data: {} },
      })
    ).toBe('Request failed with status code 503')
  })

  test('replaces an opaque image-route 404 with actionable model guidance', () => {
    expect(
      getImageGenerationErrorMessage({
        message: 'Request failed with status code 404',
        response: {
          status: 404,
          data: { error: { message: 'openai_error' } },
        },
      })
    ).toBe(
      'The selected model or channel does not support image generation. Please choose another model.'
    )
  })
})
