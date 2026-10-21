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
import { safeJsonParse } from '@/features/system-settings/utils/json-parser'

export type ModelPricingMode = 'per-token' | 'per-request'

export type ModelPricingFields = {
  price: string
  ratio: string
  cacheRatio: string
  completionRatio: string
  imageRatio: string
  audioRatio: string
  audioCompletionRatio: string
}

type ModelPricingSettings = {
  ModelPrice: string
  ModelRatio: string
  CacheRatio: string
  CompletionRatio: string
  ImageRatio: string
  AudioRatio: string
  AudioCompletionRatio: string
}

export type ModelPricingConfig = {
  mode: ModelPricingMode
  fields: ModelPricingFields
  promptPrice: string
  completionPrice: string
  advancedOpen: boolean
}

export type ModelPricingMaps = {
  price: Record<string, number>
  ratio: Record<string, number>
  cache: Record<string, number>
  completion: Record<string, number>
  image: Record<string, number>
  audio: Record<string, number>
  audioCompletion: Record<string, number>
}

type ApplyModelPricingChangeInput = {
  isEditing: boolean
  oldModelName: string
  finalModelName: string
  rewritePricing: boolean
  pricingMode: ModelPricingMode
  values: Partial<ModelPricingFields>
}

export const MODEL_PRICING_FIELD_NAMES = [
  'price',
  'ratio',
  'cacheRatio',
  'completionRatio',
  'imageRatio',
  'audioRatio',
  'audioCompletionRatio',
] as const

const emptyPricingFields = (): ModelPricingFields => ({
  price: '',
  ratio: '',
  cacheRatio: '',
  completionRatio: '',
  imageRatio: '',
  audioRatio: '',
  audioCompletionRatio: '',
})

const emptyPricingConfig = (): ModelPricingConfig => ({
  mode: 'per-token',
  fields: emptyPricingFields(),
  promptPrice: '',
  completionPrice: '',
  advancedOpen: false,
})

const lookupPricingValue = (
  rawMap: string,
  modelName: string
): number | undefined =>
  safeJsonParse<Record<string, number>>(rawMap, {
    fallback: {},
    silent: true,
  })[modelName]

export function readModelPricingConfig(
  settings: ModelPricingSettings | null,
  modelName: string
): ModelPricingConfig {
  if (!settings || !modelName) return emptyPricingConfig()

  const price = lookupPricingValue(settings.ModelPrice, modelName)
  const ratio = lookupPricingValue(settings.ModelRatio, modelName)
  const cacheRatio = lookupPricingValue(settings.CacheRatio, modelName)
  const completionRatio = lookupPricingValue(
    settings.CompletionRatio,
    modelName
  )
  const imageRatio = lookupPricingValue(settings.ImageRatio, modelName)
  const audioRatio = lookupPricingValue(settings.AudioRatio, modelName)
  const audioCompletionRatio = lookupPricingValue(
    settings.AudioCompletionRatio,
    modelName
  )

  if (price !== undefined && price !== null) {
    return {
      ...emptyPricingConfig(),
      mode: 'per-request',
      fields: { ...emptyPricingFields(), price: price.toString() },
    }
  }

  const promptPrice =
    ratio === undefined || ratio === null ? '' : (ratio * 2).toString()
  const completionPrice =
    promptPrice === '' ||
    completionRatio === undefined ||
    completionRatio === null
      ? ''
      : (Number.parseFloat(promptPrice) * completionRatio).toString()

  return {
    mode: 'per-token',
    fields: {
      ...emptyPricingFields(),
      ratio: ratio?.toString() ?? '',
      cacheRatio: cacheRatio?.toString() ?? '',
      completionRatio: completionRatio?.toString() ?? '',
      imageRatio: imageRatio?.toString() ?? '',
      audioRatio: audioRatio?.toString() ?? '',
      audioCompletionRatio: audioCompletionRatio?.toString() ?? '',
    },
    promptPrice,
    completionPrice,
    advancedOpen: [
      cacheRatio,
      imageRatio,
      audioRatio,
      audioCompletionRatio,
    ].some((value) => value !== undefined && value !== null),
  }
}

export function shouldRewriteModelPricing(
  initialMode: ModelPricingMode,
  currentMode: ModelPricingMode,
  dirtyFields: Record<string, unknown>
): boolean {
  if (initialMode !== currentMode) return true
  return MODEL_PRICING_FIELD_NAMES.some((name) => Boolean(dirtyFields[name]))
}

export function parseModelPricingMaps(
  settings: ModelPricingSettings
): ModelPricingMaps {
  const parse = (value: string) =>
    safeJsonParse<Record<string, number>>(value, {
      fallback: {},
      silent: true,
    })
  return {
    price: parse(settings.ModelPrice),
    ratio: parse(settings.ModelRatio),
    cache: parse(settings.CacheRatio),
    completion: parse(settings.CompletionRatio),
    image: parse(settings.ImageRatio),
    audio: parse(settings.AudioRatio),
    audioCompletion: parse(settings.AudioCompletionRatio),
  }
}

export function applyModelPricingChange(
  source: ModelPricingMaps,
  input: ApplyModelPricingChangeInput
): ModelPricingMaps {
  const result: ModelPricingMaps = {
    price: { ...source.price },
    ratio: { ...source.ratio },
    cache: { ...source.cache },
    completion: { ...source.completion },
    image: { ...source.image },
    audio: { ...source.audio },
    audioCompletion: { ...source.audioCompletion },
  }
  const allMaps = Object.values(result)

  if (
    input.isEditing &&
    input.oldModelName &&
    input.oldModelName !== input.finalModelName
  ) {
    const targetHasPricing = allMaps.some((map) =>
      Object.hasOwn(map, input.finalModelName)
    )
    for (const map of allMaps) {
      if (
        !input.rewritePricing &&
        !targetHasPricing &&
        Object.hasOwn(map, input.oldModelName)
      ) {
        map[input.finalModelName] = map[input.oldModelName]
      }
      delete map[input.oldModelName]
    }
  }
  if (!input.rewritePricing) return result

  for (const map of allMaps) delete map[input.finalModelName]
  if (input.pricingMode === 'per-request') {
    setPricingValue(result.price, input.finalModelName, input.values.price)
    return result
  }

  setPricingValue(result.ratio, input.finalModelName, input.values.ratio)
  setPricingValue(result.cache, input.finalModelName, input.values.cacheRatio)
  setPricingValue(
    result.completion,
    input.finalModelName,
    input.values.completionRatio
  )
  setPricingValue(result.image, input.finalModelName, input.values.imageRatio)
  setPricingValue(result.audio, input.finalModelName, input.values.audioRatio)
  setPricingValue(
    result.audioCompletion,
    input.finalModelName,
    input.values.audioCompletionRatio
  )
  return result
}

function setPricingValue(
  target: Record<string, number>,
  modelName: string,
  rawValue: string | undefined
) {
  if (rawValue === undefined || rawValue === '') return
  target[modelName] = Number.parseFloat(rawValue)
}
