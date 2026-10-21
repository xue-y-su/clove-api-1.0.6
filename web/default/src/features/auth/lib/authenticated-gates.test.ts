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

import { getAuthenticatedGateAccess } from './authenticated-gates'

describe('authenticated blocking gate order', () => {
  test('does not load any authenticated gate for guests', () => {
    expect(
      getAuthenticatedGateAccess({
        signedIn: false,
        compliance: 'satisfied',
        sourceAttribution: 'satisfied',
      })
    ).toEqual({
      compliance: false,
      sourceAttribution: false,
      announcement: false,
      content: false,
      failed: null,
    })
  })

  test('loads only compliance until it is satisfied', () => {
    expect(
      getAuthenticatedGateAccess({
        signedIn: true,
        compliance: 'required',
        sourceAttribution: 'satisfied',
      })
    ).toEqual({
      compliance: true,
      sourceAttribution: false,
      announcement: false,
      content: false,
      failed: null,
    })
  })

  test('loads source attribution only after compliance', () => {
    expect(
      getAuthenticatedGateAccess({
        signedIn: true,
        compliance: 'satisfied',
        sourceAttribution: 'required',
      })
    ).toEqual({
      compliance: true,
      sourceAttribution: true,
      announcement: false,
      content: false,
      failed: null,
    })
  })

  test('loads announcements only after both required gates are satisfied', () => {
    expect(
      getAuthenticatedGateAccess({
        signedIn: true,
        compliance: 'satisfied',
        sourceAttribution: 'satisfied',
      })
    ).toEqual({
      compliance: true,
      sourceAttribution: true,
      announcement: true,
      content: true,
      failed: null,
    })
  })

  test('blocks all later gates when compliance loading fails', () => {
    expect(
      getAuthenticatedGateAccess({
        signedIn: true,
        compliance: 'error',
        sourceAttribution: 'satisfied',
      })
    ).toEqual({
      compliance: true,
      sourceAttribution: false,
      announcement: false,
      content: false,
      failed: 'compliance',
    })
  })

  test('shows a source failure only after compliance is satisfied', () => {
    expect(
      getAuthenticatedGateAccess({
        signedIn: true,
        compliance: 'required',
        sourceAttribution: 'error',
      }).failed
    ).toBeNull()
    expect(
      getAuthenticatedGateAccess({
        signedIn: true,
        compliance: 'satisfied',
        sourceAttribution: 'error',
      }).failed
    ).toBe('sourceAttribution')
  })
})
