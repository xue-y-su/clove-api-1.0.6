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
  formatGroupRatio,
  shouldFollowUserGroup,
} from './api-key-group-combobox-state'

describe('API key group combobox labels', () => {
  test('renders numeric ratios with the ratio suffix', () => {
    expect(formatGroupRatio(0.5, '倍率')).toBe('0.5x 倍率')
  })

  test('renders the Auto ratio label without a numeric suffix', () => {
    expect(formatGroupRatio('自动', '倍率')).toBe('自动')
  })

  test('only treats an empty regular selector as following the user group', () => {
    expect(shouldFollowUserGroup(undefined, false)).toBe(true)
    expect(shouldFollowUserGroup(undefined, true)).toBe(false)
    expect(shouldFollowUserGroup('default', false)).toBe(false)
  })
})
