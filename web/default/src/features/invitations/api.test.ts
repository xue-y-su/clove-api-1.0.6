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
  freezeInvitationRewardPayoutRequest,
  getInvitationRewardPayoutRequestDetail,
  getInvitationRewardSummary,
  issueInvitationCode,
  submitInvitationRewardPayout,
  unfreezeInvitationRewardPayoutRequest,
  validateInvitationCode,
} from './api'

const originalPost = api.post
const originalGet = api.get

afterEach(() => {
  api.post = originalPost
  api.get = originalGet
})

describe('invitation mutation API', () => {
  test('validates invitation codes through the public endpoint with cancellation', async () => {
    const controller = new AbortController()
    const post = mock(async () => ({
      data: {
        success: true,
        data: {
          valid: true,
          code: 'invitation_available',
          message: 'This invitation code is available',
        },
      },
    }))
    api.post = post as typeof api.post

    const result = await validateInvitationCode('abcd2345', controller.signal)

    expect(result.valid).toBe(true)
    expect(post).toHaveBeenCalledWith(
      '/api/invitation/validate',
      { code: 'abcd2345' },
      {
        signal: controller.signal,
        skipBusinessError: true,
        skipErrorHandler: true,
      }
    )
  })

  test('owns mutation errors without invoking the global error toast', async () => {
    const post = mock(async () => ({
      data: {
        success: true,
        data: { id: 42, code: 'inv_test' },
      },
    }))
    api.post = post as typeof api.post

    await issueInvitationCode({
      target_user_id: 2882,
      reason: 'QA issue',
      kind: 'single',
      max_uses: 1,
      expires_at: 1_800_000_000,
    })

    expect(post).toHaveBeenCalledWith(
      '/api/invitation/admin/codes',
      {
        target_user_id: 2882,
        reason: 'QA issue',
        kind: 'single',
        max_uses: 1,
        expires_at: 1_800_000_000,
      },
      { skipBusinessError: true, skipErrorHandler: true }
    )
  })

  test('surfaces the server message instead of the raw Axios status text', async () => {
    api.post = mock(async () => {
      throw Object.assign(new Error('Request failed with status code 429'), {
        response: {
          data: { message: 'Too many requests, please try again later.' },
        },
      })
    }) as typeof api.post

    await expect(
      issueInvitationCode({
        target_user_id: 2882,
        reason: 'QA issue',
        kind: 'single',
        max_uses: 1,
        expires_at: 1_800_000_000,
      })
    ).rejects.toThrow('Too many requests, please try again later.')
  })

  test('loads reward totals and submits one payout request', async () => {
    const get = mock(async () => ({
      data: {
        success: true,
        data: {
          invited_count: 3,
          total_earned: 120,
          available: 50,
          pending_payout: 30,
          paid: 40,
          frozen: 0,
        },
      },
    }))
    const post = mock(async () => ({
      data: { success: true, data: { id: 8, amount: 50, status: 'pending' } },
    }))
    api.get = get as typeof api.get
    api.post = post as typeof api.post

    const summary = await getInvitationRewardSummary()
    const payoutRequest = await submitInvitationRewardPayout()

    expect(summary).toEqual({
      invited_count: 3,
      total_earned: 120,
      available: 50,
      pending_payout: 30,
      paid: 40,
      frozen: 0,
    })
    expect(payoutRequest.amount).toBe(50)
    expect(get).toHaveBeenCalledWith('/api/invitation/rewards')
    expect(post).toHaveBeenCalledWith(
      '/api/invitation/rewards/payout-requests',
      undefined,
      { skipBusinessError: true, skipErrorHandler: true }
    )
  })

  test('loads payout details and mutates frozen state through stable routes', async () => {
    const get = mock(async () => ({
      data: {
        success: true,
        data: { request: { id: 8 }, rewards: [], actions: [] },
      },
    }))
    const post = mock(async () => ({ data: { success: true, data: null } }))
    api.get = get as typeof api.get
    api.post = post as typeof api.post

    await getInvitationRewardPayoutRequestDetail(8)
    await freezeInvitationRewardPayoutRequest(8, 'fraud review')
    await unfreezeInvitationRewardPayoutRequest(8, 'review cleared')

    expect(get).toHaveBeenCalledWith(
      '/api/invitation/admin/reward-payout-requests/8'
    )
    expect(post).toHaveBeenNthCalledWith(
      1,
      '/api/invitation/admin/reward-payout-requests/8/freeze',
      { reason: 'fraud review' },
      { skipBusinessError: true, skipErrorHandler: true }
    )
    expect(post).toHaveBeenNthCalledWith(
      2,
      '/api/invitation/admin/reward-payout-requests/8/unfreeze',
      { reason: 'review cleared' },
      { skipBusinessError: true, skipErrorHandler: true }
    )
  })
})
