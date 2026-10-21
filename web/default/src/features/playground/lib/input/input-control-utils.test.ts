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

import { getInputControlState } from './input-control-utils'

const groups = [{ label: 'default', value: 'default', ratio: 1 }]
const models = [{ label: 'gpt-4o', value: 'gpt-4o' }]

describe('playground input control utilities', () => {
  test('allows attachment-only submit when a model is selected', () => {
    expect(
      getInputControlState({
        groups,
        hasAttachments: true,
        hasStopHandler: false,
        models,
        text: '',
      })
    ).toEqual({
      canSubmit: true,
      isSelectorDisabled: false,
      shouldShowStop: false,
      submitDisabledReason: null,
    })
  })

  test('reports a missing model reason when content exists but no models are available', () => {
    expect(
      getInputControlState({
        groups,
        hasAttachments: true,
        hasStopHandler: false,
        models: [],
        text: 'describe this',
      })
    ).toMatchObject({
      canSubmit: false,
      submitDisabledReason: 'noModel',
    })
  })
})
