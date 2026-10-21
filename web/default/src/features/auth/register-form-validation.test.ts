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

import { registerFormSchema } from './constants'

test('registration rejects usernames longer than the backend limit', () => {
  const result = registerFormSchema.safeParse({
    username: 'a'.repeat(21),
    email: '',
    password: 'password123',
    confirmPassword: 'password123',
    aff_code: '',
  })

  expect(result.success).toBe(false)
  if (!result.success) {
    expect(
      result.error.issues.find((issue) => issue.path[0] === 'username')?.message
    ).toBe('Username must be at most 20 characters')
  }
})

test('registration accepts Chinese usernames supported by the backend', () => {
  const result = registerFormSchema.safeParse({
    username: '用户_Alice-123',
    email: '',
    password: 'password123',
    confirmPassword: 'password123',
    aff_code: '',
  })

  expect(result.success).toBe(true)
})

test('registration rejects unsupported username characters before submit', () => {
  const result = registerFormSchema.safeParse({
    username: '用户.name😀',
    email: '',
    password: 'password123',
    confirmPassword: 'password123',
    aff_code: '',
  })

  expect(result.success).toBe(false)
  if (!result.success) {
    expect(
      result.error.issues.find((issue) => issue.path[0] === 'username')?.message
    ).toBe(
      'Username may contain Chinese characters, ASCII letters, numbers, underscores, or hyphens'
    )
  }
})
