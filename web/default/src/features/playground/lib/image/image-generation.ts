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
import type {
  ImageGenerationData,
  ImageGenerationRequest,
  PlaygroundImageSize,
} from '../../types'
import { getImageGenerationCapabilities } from '../model/model-capability-utils'

export type ImagePreview = ImageGenerationData & {
  displayPrompt: string
  id: string
  objectURL?: string
  src: string
}

type ImagePreviewOptions = {
  createObjectURL?: (blob: Blob) => string
  prompt: string
}

type ImageRequestInput = {
  background?: string
  count: number
  group: string
  model: string
  prompt: string
  quality?: string
  size: PlaygroundImageSize
  style?: string
}

type ImageGenerationErrorLike = {
  message?: unknown
  response?: {
    data?: unknown
    status?: unknown
  }
}

function readErrorMessage(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function getImageGenerationErrorMessage(
  error: unknown,
  fallback = 'Unknown error'
): string {
  const requestError = error as ImageGenerationErrorLike
  const responseData = requestError?.response?.data
  const responseRecord =
    typeof responseData === 'object' && responseData !== null
      ? (responseData as Record<string, unknown>)
      : null
  const providerError = responseRecord?.error
  const providerRecord =
    typeof providerError === 'object' && providerError !== null
      ? (providerError as Record<string, unknown>)
      : null
  const providerMessage =
    readErrorMessage(providerRecord?.message) || readErrorMessage(providerError)

  if (
    Number(requestError?.response?.status) === 404 &&
    providerMessage.toLowerCase() === 'openai_error'
  ) {
    return 'The selected model or channel does not support image generation. Please choose another model.'
  }

  return (
    providerMessage ||
    readErrorMessage(responseRecord?.message) ||
    readErrorMessage(requestError?.message) ||
    fallback
  )
}

export function buildImageGenerationRequest(
  input: ImageRequestInput
): ImageGenerationRequest {
  const capabilities = getImageGenerationCapabilities(input.model)
  const request: ImageGenerationRequest = {
    group: input.group,
    model: input.model,
    n: input.count,
    prompt: input.prompt,
    response_format: 'url',
    size: input.size,
  }
  if (
    input.quality &&
    input.quality !== 'auto' &&
    capabilities.quality.includes(input.quality)
  ) {
    request.quality = input.quality
  }
  if (input.style && capabilities.style.includes(input.style)) {
    request.style = input.style
  }
  if (
    input.background &&
    input.background !== 'auto' &&
    capabilities.background
  ) {
    request.background = input.background
  }
  return request
}

export function createImagePreviews(
  images: ImageGenerationData[],
  options: ImagePreviewOptions
): ImagePreview[] {
  return images.map((image, index) => {
    const displayPrompt = image.revised_prompt?.trim() || options.prompt
    if (!image.b64_json) {
      const src = image.url || ''
      return {
        ...image,
        displayPrompt,
        id: `${src || image.revised_prompt || 'image'}:${index}`,
        src,
      }
    }
    const binary = atob(image.b64_json)
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
    const createObjectURL =
      options.createObjectURL || ((blob: Blob) => URL.createObjectURL(blob))
    const objectURL = createObjectURL(new Blob([bytes], { type: 'image/png' }))
    return {
      ...image,
      displayPrompt,
      id: `${objectURL}:${index}`,
      objectURL,
      src: objectURL,
    }
  })
}

export function revokeImagePreviews(
  images: ImagePreview[],
  revokeObjectURL: (url: string) => void = (url) => URL.revokeObjectURL(url)
): void {
  for (const image of images) {
    if (image.objectURL) revokeObjectURL(image.objectURL)
  }
}
