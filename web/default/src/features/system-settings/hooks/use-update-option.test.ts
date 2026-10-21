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
import { expect, test } from 'bun:test'

import { getGroupRelatedQueryKeys } from './use-update-option'

test('special usable group changes refresh every user group query', () => {
  expect(
    getGroupRelatedQueryKeys('group_ratio_setting.group_special_usable_group')
  ).toEqual([
    ['pricing'],
    ['groups'],
    ['user-groups'],
    ['user-models'],
    ['playground-groups'],
    ['playground-models'],
    ['token-auto-groups'],
  ])
})

test('token Auto group limit refreshes group and token configuration queries', () => {
  expect(getGroupRelatedQueryKeys('MaxTokenAutoGroups')).toContainEqual([
    'token-auto-groups',
  ])
})

test('unrelated options do not refresh user group queries', () => {
  expect(getGroupRelatedQueryKeys('SystemName')).toEqual([])
})
