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
  applyModelPricingChange,
  readModelPricingConfig,
  shouldRewriteModelPricing,
  type ModelPricingMaps,
} from './model-pricing-form'

const pricingSettings = {
  ModelPrice: JSON.stringify({ fixed: 0.25, target: 0.5 }),
  ModelRatio: JSON.stringify({ token: 1.5, old: 2, target: 9 }),
  CacheRatio: JSON.stringify({ token: 0 }),
  CompletionRatio: JSON.stringify({ token: 3, old: 4, target: 7 }),
  ImageRatio: '{}',
  AudioRatio: '{}',
  AudioCompletionRatio: '{}',
}

const pricingMaps = (): ModelPricingMaps => ({
  price: { fixed: 0.25, target: 0.5 },
  ratio: { token: 1.5, old: 2, target: 9 },
  cache: { token: 0 },
  completion: { token: 3, old: 4, target: 7 },
  image: {},
  audio: {},
  audioCompletion: {},
})

describe('model pricing form preservation', () => {
  test('loads fixed and zero-valued token pricing without hiding it', () => {
    const fixed = readModelPricingConfig(pricingSettings, 'fixed')
    expect(fixed.mode).toBe('per-request')
    expect(fixed.fields.price).toBe('0.25')

    const token = readModelPricingConfig(pricingSettings, 'token')
    expect(token.mode).toBe('per-token')
    expect(token.fields.ratio).toBe('1.5')
    expect(token.fields.cacheRatio).toBe('0')
    expect(token.promptPrice).toBe('3')
    expect(token.completionPrice).toBe('9')
    expect(token.advancedOpen).toBe(true)
  })

  test('does not treat loaded defaults as an explicit pricing edit', () => {
    expect(
      shouldRewriteModelPricing('per-token', 'per-token', {
        model_name: true,
      })
    ).toBe(false)
    expect(shouldRewriteModelPricing('per-token', 'per-request', {})).toBe(true)
    expect(
      shouldRewriteModelPricing('per-token', 'per-token', { ratio: true })
    ).toBe(true)
  })

  test('preserves existing pricing when creating metadata for the same name', () => {
    const result = applyModelPricingChange(pricingMaps(), {
      isEditing: false,
      oldModelName: '',
      finalModelName: 'target',
      rewritePricing: false,
      pricingMode: 'per-token',
      values: {},
    })

    expect(result.price.target).toBe(0.5)
    expect(result.ratio.target).toBe(9)
    expect(result.completion.target).toBe(7)
  })

  test('removes the old name but preserves target pricing on rename', () => {
    const result = applyModelPricingChange(pricingMaps(), {
      isEditing: true,
      oldModelName: 'old',
      finalModelName: 'target',
      rewritePricing: false,
      pricingMode: 'per-token',
      values: { ratio: '2', completionRatio: '4' },
    })

    expect(result.ratio.old).toBeUndefined()
    expect(result.completion.old).toBeUndefined()
    expect(result.price.target).toBe(0.5)
    expect(result.ratio.target).toBe(9)
    expect(result.completion.target).toBe(7)
  })

  test('moves existing pricing when renaming to an unpriced name', () => {
    const result = applyModelPricingChange(pricingMaps(), {
      isEditing: true,
      oldModelName: 'old',
      finalModelName: 'renamed',
      rewritePricing: false,
      pricingMode: 'per-token',
      values: { ratio: '2', completionRatio: '4' },
    })

    expect(result.ratio.old).toBeUndefined()
    expect(result.completion.old).toBeUndefined()
    expect(result.ratio.renamed).toBe(2)
    expect(result.completion.renamed).toBe(4)
  })

  test('replaces target pricing only after an explicit pricing edit', () => {
    const result = applyModelPricingChange(pricingMaps(), {
      isEditing: true,
      oldModelName: 'old',
      finalModelName: 'target',
      rewritePricing: true,
      pricingMode: 'per-request',
      values: { price: '1.25' },
    })

    expect(result.price.target).toBe(1.25)
    expect(result.ratio.target).toBeUndefined()
    expect(result.completion.target).toBeUndefined()
  })

  test('allows an explicit clear to remove loaded pricing', () => {
    const result = applyModelPricingChange(pricingMaps(), {
      isEditing: true,
      oldModelName: 'target',
      finalModelName: 'target',
      rewritePricing: true,
      pricingMode: 'per-token',
      values: {},
    })

    expect(result.price.target).toBeUndefined()
    expect(result.ratio.target).toBeUndefined()
    expect(result.completion.target).toBeUndefined()
  })
})
