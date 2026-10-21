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
import type { UpdateOptionRequest } from '../types'

export type PlaygroundSettingsDefaults = {
  enabled: boolean
  apiKeyConfigured: boolean
  defaultDepth: 'basic' | 'advanced'
  maxResults: number
  maxContextLength: number
  nativeModels: string
}

export type PlaygroundSettingsValues = PlaygroundSettingsDefaults & {
  apiKey: string
  clearAPIKey: boolean
}

export function buildPlaygroundOptionUpdates(
  defaults: PlaygroundSettingsDefaults,
  values: PlaygroundSettingsValues
): UpdateOptionRequest[] {
  const replacementKey = values.apiKey.trim()
  const keepsConfiguredKey = defaults.apiKeyConfigured && !values.clearAPIKey
  if (values.enabled && !keepsConfiguredKey && !replacementKey) {
    throw new Error('A Tavily API key is required before enabling search.')
  }
  if (values.clearAPIKey && values.enabled) {
    throw new Error('Disable Tavily before clearing the API key.')
  }

  const updates: UpdateOptionRequest[] = []
  if (values.clearAPIKey) {
    if (defaults.enabled) {
      updates.push({ key: 'PlaygroundTavilyEnabled', value: 'false' })
    }
    updates.push({ key: 'PlaygroundTavilyAPIKey', value: '' })
  } else if (replacementKey) {
    updates.push({ key: 'PlaygroundTavilyAPIKey', value: replacementKey })
  }

  const fields = [
    ['defaultDepth', 'PlaygroundTavilyDefaultDepth'],
    ['maxResults', 'PlaygroundTavilyMaxResults'],
    ['maxContextLength', 'PlaygroundTavilyMaxContextLength'],
    ['nativeModels', 'PlaygroundNativeWebSearchModels'],
  ] as const

  for (const [field, key] of fields) {
    if (values[field] !== defaults[field]) {
      updates.push({ key, value: String(values[field]) })
    }
  }
  if (!values.clearAPIKey && values.enabled !== defaults.enabled) {
    updates.push({
      key: 'PlaygroundTavilyEnabled',
      value: String(values.enabled),
    })
  }
  return updates
}
