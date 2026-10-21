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
import { afterEach, describe, expect, mock, test } from 'bun:test'

import { api } from '@/lib/api'

import { claimRedemptionCode } from './api'

const originalPost = api.post

afterEach(() => {
  api.post = originalPost
})

describe('wallet redemption API', () => {
  test('submits one server-detected redemption request', async () => {
    const post = mock(async () => ({
      data: {
        success: true,
        data: {
          reward_type: 'subscription',
          source: 'user',
          quota: 500000,
          subscription_id: 42,
        },
      },
    }))
    api.post = post as typeof api.post

    await expect(claimRedemptionCode('wallet-code')).resolves.toEqual({
      success: true,
      data: {
        reward_type: 'subscription',
        source: 'user',
        quota: 500000,
        subscription_id: 42,
      },
    })
    expect(post).toHaveBeenCalledWith(
      '/api/redemption/claim',
      { code: 'wallet-code' },
      { skipBusinessError: true }
    )
  })
})
