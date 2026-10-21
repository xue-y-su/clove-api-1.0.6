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

import {
  decodeAuthUserGroup,
  hasAuthQueryScopeChanged,
} from './auth-query-scope'

const user = { id: 1, username: 'clove', role: 1, group: 'default' }

test('changes account query scope when identity or user group changes', () => {
  expect(hasAuthQueryScopeChanged(user, user)).toBeFalse()
  expect(hasAuthQueryScopeChanged({ ...user, group: 'grok' }, user)).toBeTrue()
  expect(hasAuthQueryScopeChanged({ ...user, id: 2 }, user)).toBeTrue()
  expect(hasAuthQueryScopeChanged(null, user)).toBeTrue()
})

test('decodes authenticated user groups from response-safe headers', () => {
  expect(decodeAuthUserGroup('%E5%9B%BD%E6%A8%A1')).toBe('国模')
  expect(decodeAuthUserGroup('%broken')).toBeNull()
  expect(decodeAuthUserGroup(undefined)).toBeNull()
})
