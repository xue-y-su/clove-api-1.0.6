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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  transformFormDataToPayload,
  transformUserToFormDefaults,
  userFormSchema,
  type UserFormValues,
} from './user-form'

const baseCreateFormValues: UserFormValues = {
  username: 'new-user',
  display_name: '',
  password: 'Password123',
  role: 1,
  quota_dollars: 0,
  group: 'default',
  remark: '',
  invite_limit_mode: 'inherit',
  invite_limit_count: 0,
  email: '',
  github_id: '',
  discord_id: '',
  oidc_id: '',
  wechat_id: '',
  telegram_id: '',
  linux_do_id: '',
  admin_permissions: {},
}

describe('user form payload transformation', () => {
  test('rejects usernames longer than the backend limit before submission', () => {
    const result = userFormSchema.safeParse({
      ...baseCreateFormValues,
      username: 'a'.repeat(21),
    })

    assert.equal(result.success, false)
    if (!result.success) {
      assert.equal(
        result.error.issues.find((issue) => issue.path[0] === 'username')
          ?.message,
        'Username must be at most 20 characters'
      )
    }
  })

  test('includes invitation code and skip reason only when creating a user', () => {
    const payload = transformFormDataToPayload({
      ...baseCreateFormValues,
      aff_code: '  abc123  ',
      aff_skip_reason: '  manually approved by admin  ',
    })

    assert.equal(payload.aff_code, 'abc123')
    assert.equal(payload.aff_skip_reason, 'manually approved by admin')
  })

  test('omits empty invitation fields when creating a user', () => {
    const payload = transformFormDataToPayload({
      ...baseCreateFormValues,
      aff_code: '  ',
      aff_skip_reason: '',
    })

    assert.equal('aff_code' in payload, false)
    assert.equal('aff_skip_reason' in payload, false)
  })

  test('does not send create-only invitation fields when updating a user', () => {
    const payload = transformFormDataToPayload(
      {
        ...baseCreateFormValues,
        aff_code: 'abc123',
        aff_skip_reason: 'manually approved by admin',
      },
      42
    )

    assert.equal('aff_code' in payload, false)
    assert.equal('aff_skip_reason' in payload, false)
  })

  test('includes trimmed built-in binding fields when updating a user', () => {
    const payload = transformFormDataToPayload(
      {
        ...baseCreateFormValues,
        username: '  renamed-user  ',
        display_name: '  Renamed User  ',
        email: '  Admin@Example.com  ',
        github_id: '  github-42  ',
        discord_id: '  discord-42  ',
        oidc_id: '  oidc-42  ',
        wechat_id: '  wechat-42  ',
        telegram_id: '  telegram-42  ',
        linux_do_id: '  linuxdo-42  ',
      },
      42
    )

    assert.equal(payload.username, 'renamed-user')
    assert.equal(payload.display_name, 'Renamed User')
    assert.equal(payload.email, 'Admin@Example.com')
    assert.equal(payload.github_id, 'github-42')
    assert.equal(payload.discord_id, 'discord-42')
    assert.equal(payload.oidc_id, 'oidc-42')
    assert.equal(payload.wechat_id, 'wechat-42')
    assert.equal(payload.telegram_id, 'telegram-42')
    assert.equal(payload.linux_do_id, 'linuxdo-42')
  })

  test('omits built-in binding fields when creating a user', () => {
    const payload = transformFormDataToPayload({
      ...baseCreateFormValues,
      email: 'admin@example.com',
      github_id: 'github-42',
      linux_do_id: 'linuxdo-42',
    })

    assert.equal('email' in payload, false)
    assert.equal('github_id' in payload, false)
    assert.equal('linux_do_id' in payload, false)
  })

  test('loads built-in binding fields into update defaults', () => {
    const defaults = transformUserToFormDefaults({
      id: 42,
      username: 'admin',
      display_name: 'Admin',
      email: 'admin@example.com',
      github_id: 'github-42',
      discord_id: 'discord-42',
      oidc_id: 'oidc-42',
      wechat_id: 'wechat-42',
      telegram_id: 'telegram-42',
      linux_do_id: 'linuxdo-42',
      quota: 0,
      used_quota: 0,
      request_count: 0,
      group: 'default',
      status: 1,
      role: 1,
    })

    assert.equal(defaults.email, 'admin@example.com')
    assert.equal(defaults.github_id, 'github-42')
    assert.equal(defaults.linux_do_id, 'linuxdo-42')
  })
})
