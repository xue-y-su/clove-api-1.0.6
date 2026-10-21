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

import { CHANNEL_TYPES, MODEL_FETCHABLE_TYPES } from '../constants'
import {
  createUpstreamDiffSelection,
  deriveRetainedModels,
} from './upstream-update-utils'

describe('Sub2API publication preview', () => {
  test('registers channel type 59 as fetchable', () => {
    expect(CHANNEL_TYPES[59]).toBe('Sub2API')
    expect(MODEL_FETCHABLE_TYPES.has(59)).toBe(true)
  })

  test('selects additions but keeps missing removals off by default', () => {
    expect(
      createUpstreamDiffSelection(['new-a', 'new-b'], ['missing-a'])
    ).toEqual({
      selectedAdd: ['new-a', 'new-b'],
      selectedRemove: [],
    })
  })

  test('derives retained models from current models and missing entries', () => {
    expect(deriveRetainedModels('kept,missing, kept ', ['missing'])).toEqual([
      'kept',
    ])
  })

  test('does not offer unsupported automatic publication for Sub2API', async () => {
    const source = await Bun.file(
      new URL(
        '../components/drawers/channel-mutate-drawer.tsx',
        import.meta.url
      )
    ).text()
    expect(source).toContain('currentType === 59 ||')
  })

  test('keeps publication copy translated in every Default locale', async () => {
    const keys = [
      'Review added, retained, and missing upstream models before confirming.',
      'Approved additions remain hidden from the model catalog until they have explicit pricing and at least one real enabled group.',
      'Added',
      'Retained',
      'Missing',
      'No missing models',
      'Sub2API gateway base URL',
      'Sub2API API key',
      'Models discovered from upstream /v1/models',
      'Enter the Sub2API gateway API key',
    ]

    for (const locale of ['en', 'fr', 'ja', 'ru', 'vi', 'zh-TW', 'zh']) {
      const messages = (await Bun.file(
        new URL(`../../../i18n/locales/${locale}.json`, import.meta.url)
      ).json()) as { translation: Record<string, string> }
      for (const key of keys) {
        expect(messages.translation[key]).toBeTruthy()
      }
    }
  })
})
