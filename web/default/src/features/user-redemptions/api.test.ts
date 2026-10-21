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

import {
  batchSettleAdminUserRedemptions,
  buildCreateUserRedemptionRequest,
  claimUserRedemption,
  createUserRedemption,
} from './api'

const originalPost = api.post

afterEach(() => {
  api.post = originalPost
})

describe('user redemption mutation API', () => {
  test('builds a subscription redemption purchase request', () => {
    expect(
      buildCreateUserRedemptionRequest({
        rewardType: 'subscription',
        planId: 7,
        units: 3,
        expiresAt: 1_800_000_000,
      })
    ).toEqual({
      reward_type: 'subscription',
      plan_id: 7,
      units: 3,
      expires_at: 1_800_000_000,
    })
  })

  test('builds the backward-compatible balance redemption request', () => {
    expect(
      buildCreateUserRedemptionRequest({
        rewardType: 'balance',
        kind: 'random',
        totalQuota: '1500000',
        maxClaims: 3,
        expiresAt: 1_800_000_000,
      })
    ).toEqual({
      reward_type: 'balance',
      kind: 'random',
      total_quota: '1500000',
      max_claims: 3,
      expires_at: 1_800_000_000,
    })
  })

  test('returns every batch settlement result without discarding partial failures', async () => {
    const post = mock(async () => ({
      data: {
        success: true,
        data: [
          { id: 11, success: true },
          { id: 12, success: false, code: 'not_found' },
        ],
      },
    }))
    api.post = post as typeof api.post

    await expect(
      batchSettleAdminUserRedemptions({
        ids: [11, 12],
        disposition: 'refund',
        reason: 'batch review',
      })
    ).resolves.toEqual([
      { id: 11, success: true },
      { id: 12, success: false, code: 'not_found' },
    ])
    expect(post).toHaveBeenCalledWith(
      '/api/user-redemption/admin/batch-settlement',
      {
        ids: [11, 12],
        disposition: 'refund',
        reason: 'batch review',
      }
    )
  })

  test('surfaces an HTTP 200 business failure to the creation dialog', async () => {
    const post = mock(async () => ({
      data: {
        success: false,
        code: 'REDEMPTION_AMOUNT_EXCEEDS_BALANCE',
        message: 'Amount 10000000000.00 exceeds available wallet balance 12.34',
      },
    }))
    api.post = post as typeof api.post

    await expect(
      createUserRedemption({
        kind: 'single',
        total_quota: '5000000000000000',
        max_claims: 1,
        expires_at: 1_800_000_000,
      })
    ).rejects.toThrow(
      'Amount 10000000000.00 exceeds available wallet balance 12.34'
    )
    expect(post).toHaveBeenCalledWith(
      '/api/user-redemption/',
      {
        kind: 'single',
        total_quota: '5000000000000000',
        max_claims: 1,
        expires_at: 1_800_000_000,
      },
      { skipBusinessError: true, skipErrorHandler: true }
    )
  })

  test('rejects a subscription claim business failure instead of reporting success', async () => {
    const post = mock(async () => ({
      data: {
        success: false,
        code: 'USER_REDEMPTION_PLAN_PURCHASE_LIMIT',
        message: 'Subscription purchase limit reached',
      },
    }))
    api.post = post as typeof api.post

    await expect(
      claimUserRedemption('SUB-CODE', 'subscription')
    ).rejects.toThrow('Subscription purchase limit reached')
    expect(post).toHaveBeenCalledWith(
      '/api/user-redemption/claim',
      {
        code: 'SUB-CODE',
        expected_reward_type: 'subscription',
      },
      { skipBusinessError: true, skipErrorHandler: true }
    )
  })
})
