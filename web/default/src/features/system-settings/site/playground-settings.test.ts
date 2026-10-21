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

import { buildPlaygroundOptionUpdates } from './playground-settings'

const defaults = {
  enabled: false,
  apiKeyConfigured: false,
  defaultDepth: 'basic' as const,
  maxResults: 5,
  maxContextLength: 4000,
  nativeModels: 'gpt-4.1*',
  imageAllowlist: '',
  imageDenylist: '',
  imageDisplayOrder: '',
  imageExtraAllowlist: '',
}

test('saves a new Tavily key before enabling search', () => {
  const updates = buildPlaygroundOptionUpdates(defaults, {
    ...defaults,
    enabled: true,
    apiKey: 'tvly-new-key',
    clearAPIKey: false,
  })

  expect(updates).toEqual([
    { key: 'PlaygroundTavilyAPIKey', value: 'tvly-new-key' },
    { key: 'PlaygroundTavilyEnabled', value: 'true' },
  ])
})

test('disables Tavily before clearing its configured key', () => {
  const configuredDefaults = {
    ...defaults,
    enabled: true,
    apiKeyConfigured: true,
  }
  const updates = buildPlaygroundOptionUpdates(configuredDefaults, {
    ...configuredDefaults,
    enabled: false,
    apiKey: '',
    clearAPIKey: true,
  })

  expect(updates).toEqual([
    { key: 'PlaygroundTavilyEnabled', value: 'false' },
    { key: 'PlaygroundTavilyAPIKey', value: '' },
  ])
})

test('does not overwrite a configured key with a blank replacement', () => {
  const configuredDefaults = { ...defaults, apiKeyConfigured: true }
  const updates = buildPlaygroundOptionUpdates(configuredDefaults, {
    ...configuredDefaults,
    apiKey: '   ',
    clearAPIKey: false,
    maxResults: 7,
  })

  expect(updates).toEqual([{ key: 'PlaygroundTavilyMaxResults', value: '7' }])
})

test('does not expose removed image policy controls', async () => {
  const sectionSource = await Bun.file(
    new URL('./playground-section.tsx', import.meta.url)
  ).text()
  const settingsSource = await Bun.file(
    new URL('./playground-settings.ts', import.meta.url)
  ).text()

  for (const key of [
    'imageAllowlist',
    'imageDenylist',
    'imageDisplayOrder',
    'imageExtraAllowlist',
    'PlaygroundImageModelsAllowlist',
    'PlaygroundImageModelsDenylist',
    'PlaygroundImageModelsDisplayOrder',
    'PlaygroundImageModelsExtraAllowlist',
  ]) {
    expect(sectionSource).not.toContain(key)
    expect(settingsSource).not.toContain(key)
  }
})

describe('invalid Tavily transitions', () => {
  test('rejects enabling without an existing or replacement key', () => {
    expect(() =>
      buildPlaygroundOptionUpdates(defaults, {
        ...defaults,
        enabled: true,
        apiKey: '',
        clearAPIKey: false,
      })
    ).toThrow('API key')
  })
})
