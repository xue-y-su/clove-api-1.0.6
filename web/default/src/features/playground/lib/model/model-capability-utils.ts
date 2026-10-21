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
const VISION_MODEL_PATTERNS = [
  /\bgpt-4(?:\.1|\.5)?(?:o|[-_]?vision)?\b/,
  /\bgpt-5\b/,
  /\bo[34]\b/,
  /\bgemini\b/,
  /\bclaude-3\b/,
  /\bqwen(?:2(?:\.5)?|3)?[-_ ]?vl\b/,
  /\bdeepseek[-_ ]?vl\d*\b/,
  /\bqvq\b/,
  /\bglm-4v\b/,
  /\bcogvlm\b/,
  /\bllava\b/,
  /\bminicpm[-_ ]?v\b/,
  /\bpixtral\b/,
  /\bvision\b/,
  /\bvisual\b/,
  /\bvl\b/,
  /\bomni\b/,
]

const IMAGE_GENERATION_MODEL_PATTERNS = [
  /\bgpt[-_ ]?image(?:[-_ ]?\d+)?\b/,
  /\bdall[-_ ]?e(?:[-_ ]?\d+)?\b/,
  /\bimagen(?:[-_ ]?\d+)?\b/,
  /\bflux(?:[-_ ]?(?:pro|dev|schnell))?\b/,
  /\bideogram\b/,
  /\bmidjourney\b/,
  /\bstable[-_ ]?diffusion\b/,
  /\bsdxl\b/,
]

function normalizeModelName(model: string): string {
  return model.trim().toLowerCase()
}

export function isKnownVisionModel(model: string): boolean {
  const normalized = normalizeModelName(model)
  if (!normalized) return false

  return VISION_MODEL_PATTERNS.some((pattern) => pattern.test(normalized))
}

export function isKnownImageGenerationModel(model: string): boolean {
  const normalized = normalizeModelName(model)
  if (!normalized) return false

  return IMAGE_GENERATION_MODEL_PATTERNS.some((pattern) =>
    pattern.test(normalized)
  )
}

export function supportsPlaygroundReasoning(model: string): boolean {
  const normalized = normalizeModelName(model)
  if (!normalized) return false

  return (
    /^(?:o1|o3|o4)(?:[-_]|$)/.test(normalized) ||
    /^gpt[-_ ]?5(?:[-_ ]|$)/.test(normalized) ||
    /^grok[-_ ]?3[-_ ]?mini(?:[-_ ]|$)/.test(normalized)
  )
}

export type ImageGenerationCapabilities = {
  background: boolean
  quality: string[]
  style: string[]
}

export function getImageGenerationCapabilities(
  model: string
): ImageGenerationCapabilities {
  const normalized = normalizeModelName(model)
  if (/\bgpt[-_ ]?image(?:[-_ ]?\d+)?\b/.test(normalized)) {
    return {
      background: true,
      quality: ['auto', 'low', 'medium', 'high'],
      style: [],
    }
  }
  if (/\bdall[-_ ]?e[-_ ]?3\b/.test(normalized)) {
    return {
      background: false,
      quality: ['standard', 'hd'],
      style: ['vivid', 'natural'],
    }
  }
  return { background: false, quality: [], style: [] }
}

export function isKnownTextOnlyModelForImageInput(model: string): boolean {
  void model
  return false
}

export function shouldBlockImageInputForModel(model: string): boolean {
  // Image attachments are always allowed through. Upstream model capability
  // errors should come from the provider, not from model-name guessing here.
  void model
  return false
}
