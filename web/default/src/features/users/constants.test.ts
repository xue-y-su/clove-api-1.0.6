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

import { getUserRoleOptions, getUserStatusOptions } from './constants'

describe('user table filter options', () => {
  test('keeps status labels as i18n keys for the shared faceted filter', () => {
    expect(getUserStatusOptions().map((option) => option.label)).toEqual([
      'Enabled',
      'Disabled',
      'Deleted',
    ])
  })

  test('keeps role labels as i18n keys for the shared faceted filter', () => {
    expect(getUserRoleOptions().map((option) => option.label)).toEqual([
      'User',
      'Admin',
      'Root',
    ])
  })
})
